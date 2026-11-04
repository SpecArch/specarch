package extract

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

// A tagged PDF holds a structure tree (ISO 32000-2, 14.7): elements such
// as Table, TR, TH and TD whose content is the page content marked with
// a marked-content identifier, an MCID, unique on its page (14.7.5).
// The reader walks the tree for its tables and matches each cell to the
// glyphs of its marked content, so a table is read as its rows and cells
// and not as its lines (ADR-212). An untagged PDF has no tree and is
// read by its lines alone.

// pdfMark names a piece of marked content: its page and its MCID.
type pdfMark struct {
	page string // pdfPageKey of the page
	mcid int64
}

// pdfStructCell is a TH or TD element: the marked content it holds and
// the rows and columns it spans (14.8.5, standard structure attributes).
type pdfStructCell struct {
	marks            []pdfMark
	rowSpan, colSpan int
}

// pdfStructTable is a Table element: its rows, each a TR's cells, in the
// order of the structure tree, with those of THead, TBody and TFoot.
type pdfStructTable struct {
	rows [][]pdfStructCell
}

// pdfPageKey names a page object so that a structure element's Pg entry
// finds it: by the page's StructParents key, which a tagged PDF gives
// every page holding marked content (14.7.5.4), or else by its
// dictionary as the file writes it, references and all.
func pdfPageKey(page pdf.Value) string {
	return pdfPageKeyOf(page, page.Key("StructParents"))
}

// pdfPageKeyOf is pdfPageKey with the page's StructParents already read,
// so that the structure-tree walk can count what reading it cost.
func pdfPageKeyOf(page, structParents pdf.Value) string {
	if structParents.Kind() == pdf.Integer {
		return fmt.Sprintf("StructParents %d", structParents.Int64())
	}
	return page.String()
}

// pdfStructBudget bounds the work a walk of the structure tree does, in
// units of one object resolved plus one for every pdfStructUnit bytes of
// it. The depth limit alone does not bound it: an element that lists the
// same child twice in K, at every level, makes the walk branch at each
// one, and one that lists it thousands of times makes every visit resolve
// thousands of entries. The library reads an object again from the file
// each time a reference to it is resolved, so a resolve costs the size of
// the object, and the budget counts that size. A malformed or crafted
// tree would otherwise hold the read for hours. A tagged document's tree
// holds a few small elements per paragraph and per cell, its whole tree
// far below this.
const (
	pdfStructBudget = 1000000
	pdfStructUnit   = 64
)

// pdfStructTables walks the structure tree of a tagged PDF and gives its
// tables. A PDF with no structure tree gives none. A tree whose walk
// costs more than pdfStructBudget gives none, and over is true: every
// page is then read by its lines, and the reader says so.
func pdfStructTables(r *pdf.Reader) (tables []pdfStructTable, over bool) {
	root := r.Trailer().Key("Root").Key("StructTreeRoot")
	if root.Kind() != pdf.Dict {
		return nil, false
	}
	return pdfStructTablesFrom(root, pdfStructBudget)
}

// pdfStructTablesFrom walks the tree under root, doing at most budget
// units of work.
func pdfStructTablesFrom(root pdf.Value, budget int) (tables []pdfStructTable, over bool) {
	spent := 0
	// spend counts n units of work, and is false once the budget is
	// spent: every walk then stops where it is.
	spend := func(n int) bool {
		spent += n
		if spent > budget {
			over = true
		}
		return !over
	}
	// resolved is what a value just resolved costs: one, and one for each
	// pdfStructUnit bytes it is written in, its references written and not
	// followed, as the library parsed it.
	resolved := func(v pdf.Value) int {
		return 1 + len(v.String())/pdfStructUnit
	}
	// key reads one entry of a dictionary and spends what resolving it
	// cost. The library resolves a reference each time an entry is read,
	// so every entry the walk reads, its Type, S, RoleMap, A, Stm and
	// MCID as much as its K and Pg, goes through key; an entry left
	// uncounted is one a crafted file points at a large object. Once the
	// budget is spent it gives null, and every walk stops.
	key := func(v pdf.Value, name string) pdf.Value {
		if over {
			return pdf.Value{}
		}
		e := v.Key(name)
		if !spend(resolved(e)) {
			return pdf.Value{}
		}
		return e
	}
	// index reads one entry of an array as key reads one of a
	// dictionary.
	index := func(a pdf.Value, i int) pdf.Value {
		if over {
			return pdf.Value{}
		}
		e := a.Index(i)
		if !spend(resolved(e)) {
			return pdf.Value{}
		}
		return e
	}
	// page gives the key of the page an element names in Pg, or inherited
	// when it names none.
	page := func(v pdf.Value, inherited string) string {
		if pg := key(v, "Pg"); pg.Kind() == pdf.Dict {
			return pdfPageKeyOf(pg, key(pg, "StructParents"))
		}
		return inherited
	}
	roles := key(root, "RoleMap")
	// role resolves a structure type through the role map to the
	// standard type it stands for (14.7.3), a chain followed up to eight
	// steps, so that a custom name a word processor writes is read. Each
	// name is resolved once.
	known := map[string]string{}
	role := func(s string) string {
		if r, ok := known[s]; ok {
			return r
		}
		r := s
		for i := 0; i < 8; i++ {
			m := key(roles, r)
			if m.Kind() != pdf.Name || m.Name() == r {
				break
			}
			r = m.Name()
		}
		known[s] = r
		return r
	}
	// marks collects the marked content under an element, its own and its
	// descendants', with the page inherited from the nearest Pg.
	var marks func(v pdf.Value, on string, depth int, out *[]pdfMark)
	marks = func(v pdf.Value, on string, depth int, out *[]pdfMark) {
		if !spend(resolved(v)) || depth > 64 {
			return
		}
		switch v.Kind() {
		case pdf.Integer:
			*out = append(*out, pdfMark{page: on, mcid: v.Int64()})
		case pdf.Array:
			for i := 0; i < v.Len() && !over; i++ {
				marks(v.Index(i), on, depth+1, out)
			}
		case pdf.Dict:
			on = page(v, on)
			switch key(v, "Type").Name() {
			case "MCR":
				// An MCID in a form XObject's stream (Stm) is that
				// stream's, not the page's, and the form's glyphs are not
				// among the page's text: it is not matched.
				if key(v, "Stm").IsNull() && !over {
					if id := key(v, "MCID"); id.Kind() == pdf.Integer {
						*out = append(*out, pdfMark{page: on, mcid: id.Int64()})
					}
				}
			case "OBJR":
				// An object reference, such as an annotation, holds no
				// page content.
			default:
				marks(v.Key("K"), on, depth+1, out)
			}
		}
	}
	// children gives an element's child elements with the page each
	// inherits and its structure type through the role map, read once.
	// Each entry of K resolved is spent from the budget, so that an
	// element listing the same child thousands of times costs what was
	// resolved; once the budget is spent it gives what it has.
	type child struct {
		v    pdf.Value
		page string
		role string
	}
	children := func(v pdf.Value, inherited string) []child {
		on := page(v, inherited)
		k := key(v, "K")
		if over {
			return nil
		}
		var out []child
		add := func(c pdf.Value) {
			if !spend(resolved(c)) || c.Kind() != pdf.Dict {
				return
			}
			if t := key(c, "Type").Name(); t == "MCR" || t == "OBJR" || over {
				return
			}
			out = append(out, child{c, on, role(key(c, "S").Name())})
		}
		if k.Kind() == pdf.Array {
			for i := 0; i < k.Len() && !over; i++ {
				add(k.Index(i))
			}
		} else {
			add(k)
		}
		return out
	}
	var rows func(v pdf.Value, page string, depth int, t *pdfStructTable)
	rows = func(v pdf.Value, page string, depth int, t *pdfStructTable) {
		if depth > 64 || !spend(1) {
			return
		}
		for _, c := range children(v, page) {
			switch c.role {
			case "THead", "TBody", "TFoot":
				rows(c.v, c.page, depth+1, t)
			case "TR":
				var row []pdfStructCell
				for _, cell := range children(c.v, c.page) {
					if !spend(1) {
						return
					}
					if cell.role != "TH" && cell.role != "TD" {
						continue
					}
					a := key(cell.v, "A")
					sc := pdfStructCell{rowSpan: pdfTableAttribute(a, "RowSpan", key, index), colSpan: pdfTableAttribute(a, "ColSpan", key, index)}
					marks(cell.v, cell.page, 0, &sc.marks)
					row = append(row, sc)
				}
				if len(row) > 0 {
					t.rows = append(t.rows, row)
				}
			}
		}
	}
	var walk func(v pdf.Value, page string, depth int)
	walk = func(v pdf.Value, page string, depth int) {
		if depth > 64 || !spend(1) {
			return
		}
		for _, c := range children(v, page) {
			if c.role == "Table" {
				var t pdfStructTable
				rows(c.v, c.page, 0, &t)
				if len(t.rows) > 0 {
					tables = append(tables, t)
				}
				continue
			}
			walk(c.v, c.page, depth+1)
		}
	}
	walk(root, "", 0)
	if over {
		return nil, true
	}
	return tables, false
}

// pdfTableAttribute reads RowSpan or ColSpan from an element's
// attributes a, a dictionary or an array of them with revision numbers
// between (14.7.6); one when not given. Every entry is read through
// key and index, which count what resolving it cost and give null once
// the walk's budget is spent.
func pdfTableAttribute(a pdf.Value, name string, key func(pdf.Value, string) pdf.Value, index func(pdf.Value, int) pdf.Value) int {
	var dicts []pdf.Value
	if a.Kind() == pdf.Array {
		for i := 0; i < a.Len(); i++ {
			dicts = append(dicts, index(a, i))
		}
	} else {
		dicts = append(dicts, a)
	}
	for _, d := range dicts {
		if n := key(d, name); n.Kind() == pdf.Integer && n.Int64() >= 1 && n.Int64() <= 1000 {
			return int(n.Int64())
		}
	}
	return 1
}

// pdfGlyphMarks gives the MCID of every glyph that the library's
// Content gives for the page, in the same order, or -1 for a glyph in no
// marked content with an MCID. It reads the content stream the way
// Content does: a font's encoding is set by Tf alone, and every string a
// text operator shows gives one glyph per character it decodes to, the
// end of a TJ array a newline decoded as well. The innermost marked
// content that names an MCID holds a glyph, so a Span nested inside for
// its actual text keeps its parent's MCID (14.6).
func pdfGlyphMarks(p pdf.Page) []int64 {
	strm := p.V.Key("Contents")
	if p.V.IsNull() || strm.Kind() == pdf.Null {
		return nil
	}
	var enc pdf.TextEncoding
	decode := func(s string) int {
		if enc == nil {
			return utf8.RuneCountInString(s)
		}
		return utf8.RuneCountInString(enc.Decode(s))
	}
	var out []int64
	var stack []int64
	current := func() int64 {
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i] >= 0 {
				return stack[i]
			}
		}
		return -1
	}
	show := func(n int) {
		id := current()
		for i := 0; i < n; i++ {
			out = append(out, id)
		}
	}
	pdf.Interpret(strm, func(stk *pdf.Stack, op string) {
		n := stk.Len()
		args := make([]pdf.Value, n)
		for i := n - 1; i >= 0; i-- {
			args[i] = stk.Pop()
		}
		switch op {
		case "BMC":
			stack = append(stack, -1)
		case "BDC":
			id := int64(-1)
			if len(args) == 2 {
				props := args[1]
				if props.Kind() == pdf.Name {
					props = p.Resources().Key("Properties").Key(props.Name())
				}
				if m := props.Key("MCID"); m.Kind() == pdf.Integer {
					id = m.Int64()
				}
			}
			stack = append(stack, id)
		case "EMC":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case "Tf":
			if len(args) == 2 {
				enc = p.Font(args[0].Name()).Encoder()
			}
		case "\"":
			if len(args) == 3 {
				show(decode(args[2].RawString()))
			}
		case "'", "Tj":
			if len(args) == 1 {
				show(decode(args[0].RawString()))
			}
		case "TJ":
			if len(args) == 1 {
				v := args[0]
				for i := 0; i < v.Len(); i++ {
					if x := v.Index(i); x.Kind() == pdf.String {
						show(decode(x.RawString()))
					}
				}
				show(decode("\n"))
			}
		}
	})
	return out
}

// pdfTableRows lays a table's cells on a grid, a cell spanning rows or
// columns leaving the places it covers empty, so that every column keeps
// its header; each cell's text is its lines joined. Rows are as wide as
// the widest.
func pdfTableRows(t pdfStructTable, text func(row, cell int) string) [][]string {
	taken := map[[2]int]bool{}
	var grid [][]string
	width := 0
	set := func(r, c int, s string) {
		for len(grid) <= r {
			grid = append(grid, nil)
		}
		for len(grid[r]) <= c {
			grid[r] = append(grid[r], "")
		}
		grid[r][c] = s
		if c+1 > width {
			width = c + 1
		}
	}
	for r, row := range t.rows {
		c := 0
		for i, cell := range row {
			for taken[[2]int{r, c}] {
				c++
			}
			set(r, c, text(r, i))
			for dr := 0; dr < cell.rowSpan && r+dr < len(t.rows); dr++ {
				for dc := 0; dc < cell.colSpan; dc++ {
					taken[[2]int{r + dr, c + dc}] = true
					if dr > 0 || dc > 0 {
						set(r+dr, c+dc, "")
					}
				}
			}
			c += cell.colSpan
		}
	}
	for r := range grid {
		for len(grid[r]) < width {
			grid[r] = append(grid[r], "")
		}
	}
	return grid
}

// pdfCellText joins a cell's lines into one text.
func pdfCellText(lines []pdfLine) string {
	var parts []string
	for _, l := range lines {
		parts = append(parts, l.text)
	}
	return strings.Join(parts, " ")
}
