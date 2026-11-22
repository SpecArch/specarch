// Package docx writes a book of Markdown documents as one Word document,
// an Office Open XML package (ISO/IEC 29500) on A4 pages (ADR-249): front
// matter, a table of contents, the documents, and the list of the
// problems and gaps they mark, with real heading styles, tables,
// bookmarks, links and the same bytes for the same input. It reads the
// book through internal/mdbook, as the PDF writer does, so the two carry
// the same text, anchors, links and marks. It knows Markdown and Word,
// nothing of a specification: what links, what is a mark and the words
// it writes come from the caller.
package docx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SpecArch/specarch/internal/mdbook"
)

// Texts are the words a book writes of its own, in the document's
// language.
type Texts struct {
	Contents      string // the table of contents' heading
	FrontMatter   string // the front matter's heading in the problems and gaps list
	Problems      string // the problems and gaps list's heading
	ProblemsIntro string // the sentence under it, with %d for the count
	NoProblems    string // the sentence when there is none
	PageOf        string // "Page %d of %d", the footer; each %d becomes a field
	Mermaid       string // the label above a Mermaid block
}

// Options are what a book is made of.
type Options struct {
	// Title is the book's title: the document's title property.
	Title string
	// Header is the line at the top of every page; Title when empty.
	Header string
	// Lang is the document's language tag, en or id.
	Lang string
	// FrontMatter is the Markdown of the front matter, before the table of
	// contents; Parts are the documents after it.
	FrontMatter string
	Parts       []mdbook.Part
	// IDs are the element ids that link to the heading or table row that
	// describes them.
	IDs map[string]bool
	// Slug is the anchor a heading gets in its document, as the documents'
	// own links name it.
	Slug func(string) string
	// IsMark says whether a paragraph whose first bold text is lead marks
	// a problem or a gap; text is the whole paragraph.
	IsMark func(lead, text string) bool
	Texts  Texts
}

// The namespaces of the parts.
const (
	nsW   = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsR   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	nsRel = "http://schemas.openxmlformats.org/package/2006/relationships"
	nsCT  = "http://schemas.openxmlformats.org/package/2006/content-types"
	relT  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
)

// A4 in twentieths of a point, with 20 mm margins; width is what is
// between the margins.
const (
	pageW  = 11906
	pageH  = 16838
	margin = 1134
	width  = pageW - 2*margin
)

// zipTime is the time of every entry of the package: the earliest an MS-DOS
// date holds, so nothing in the file comes from the clock.
var zipTime = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

// Write makes the book.
func Write(opt Options) ([]byte, error) {
	w := &writer{opt: opt, mb: mdbook.Prepare(mdbook.Options{FrontMatter: opt.FrontMatter, Parts: opt.Parts, IDs: opt.IDs, Slug: opt.Slug, IsMark: opt.IsMark}), names: map[string]string{}, used: map[string]bool{}}
	w.nameBookmarks()
	w.body()
	return w.pack()
}

type writer struct {
	opt Options
	mb  *mdbook.Book
	// names are the bookmark of each destination, by its name in mb.
	names map[string]string
	used  map[string]bool
	b     strings.Builder // the body of document.xml
	// rels are the external links, in the order they are first used,
	// each with its relationship id.
	rels    []string
	relID   map[string]string
	nextBm  int
	nums    []int // the start of each numbered list, numId 2 on
	pending bool  // the next paragraph starts a page
	// afterTable says the last block was a table, so the next paragraph
	// keeps a space from it.
	afterTable bool
}

// nameBookmarks gives each destination a bookmark name Word accepts: at
// most 40 letters, digits and underscores, starting with a letter, in the
// order the destinations were made, so the same book names them the same.
// A document's name shares its first heading's bookmark.
func (w *writer) nameBookmarks() {
	for _, n := range w.mb.Names {
		if target := w.mb.Dests[n]; target != n {
			continue
		}
		base := n
		if id, ok := strings.CutPrefix(n, "row/"); ok {
			base = "id_" + id
		} else {
			for id, d := range w.mb.IDDest {
				if d == n {
					base = "id_" + id
				}
			}
		}
		w.names[n] = w.unique(sanitize(base))
	}
}

// sanitize keeps the letters and digits of s, writing every other run of
// characters as one underscore, and cuts it to 36 characters, leaving
// room for a number that keeps two names apart.
func sanitize(s string) string {
	var b strings.Builder
	under := false
	for _, r := range s {
		if r < utf8.RuneSelf && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			b.WriteRune(r)
			under = false
			continue
		}
		if !under && b.Len() > 0 {
			b.WriteByte('_')
			under = true
		}
	}
	out := strings.TrimRight(b.String(), "_")
	if out == "" || !(out[0] >= 'a' && out[0] <= 'z' || out[0] >= 'A' && out[0] <= 'Z') {
		out = "d_" + out
	}
	if len(out) > 36 {
		out = strings.TrimRight(out[:36], "_")
	}
	return out
}

func (w *writer) unique(name string) string {
	out := name
	for n := 2; w.used[out]; n++ {
		out = fmt.Sprintf("%s_%d", name, n)
	}
	w.used[out] = true
	return out
}

// bookmark is the bookmark a destination name links to, through a
// document's name to its first heading, or "".
func (w *writer) bookmark(dest string) string {
	if t, ok := w.mb.Dests[dest]; ok {
		return w.names[t]
	}
	return ""
}

func (w *writer) body() {
	front := w.mb.Parts[0]
	for i := range front.Blocks {
		w.block("front", &front.Blocks[i], 0, i == 0)
	}
	w.contents()
	for pi := 1; pi < len(w.mb.Parts); pi++ {
		w.pending = true
		p := &w.mb.Parts[pi]
		for i := range p.Blocks {
			w.block(p.Name, &p.Blocks[i], 0, false)
		}
	}
	w.problems()
}

// contents writes the table of contents as a TOC field whose result holds
// every entry already, each a link to its heading: Word keeps the entries
// and adds the page numbers when the reader updates the field.
func (w *writer) contents() {
	w.pending = true
	w.para(pPr{style: "TOCHeading"}, func() { w.bookmarked(mdbook.ContentsDest, func() { w.run(w.opt.Texts.Contents, runStyle{}) }) })
	entries := append([]mdbook.TOCEntry{}, w.mb.TOC...)
	entries = append(entries, mdbook.TOCEntry{Level: 1, Text: w.opt.Texts.Problems, Dest: mdbook.ProblemsDest})
	for i, e := range entries {
		w.para(pPr{style: fmt.Sprintf("TOC%d", e.Level)}, func() {
			if i == 0 {
				w.b.WriteString(`<w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText xml:space="preserve"> TOC \o "1-3" \h \z \u </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r>`)
			}
			w.link(w.bookmark(e.Dest), "", func() { w.run(e.Text, runStyle{}) })
		})
	}
	w.para(pPr{}, func() { w.b.WriteString(`<w:r><w:fldChar w:fldCharType="end"/></w:r>`) })
}

// problems writes the list of the problems and gaps, grouped by document,
// each number linking to its mark and the mark's label linking back.
func (w *writer) problems() {
	tx := w.opt.Texts
	w.pending = true
	w.para(pPr{style: "Heading1"}, func() { w.bookmarked(mdbook.ProblemsDest, func() { w.run(tx.Problems, runStyle{}) }) })
	intro := tx.NoProblems
	if len(w.mb.Marks) > 0 {
		intro = fmt.Sprintf(tx.ProblemsIntro, len(w.mb.Marks))
	}
	w.para(pPr{}, func() { w.run(intro, runStyle{}) })
	part := ""
	for i, mk := range w.mb.Marks {
		if i == 0 || mk.Part != part {
			part = mk.Part
			title := w.mb.Titles[part]
			if part == "front" {
				title = tx.FrontMatter
			}
			w.para(pPr{style: "Heading2"}, func() { w.run(title, runStyle{}) })
		}
		w.para(pPr{style: "ProblemEntry"}, func() {
			w.bookmarked(mk.Back, func() {
				w.link(w.bookmark(mk.Dest), "", func() { w.run(fmt.Sprintf("%d.", mk.N), runStyle{bold: true}) })
			})
			w.b.WriteString(`<w:r><w:tab/></w:r>`)
			w.run(mk.Text, runStyle{})
		})
	}
}

// block writes one block of a part at list depth depth; title marks the
// front matter's first heading.
func (w *writer) block(part string, bl *mdbook.Block, depth int, title bool) {
	indent := pPr{}
	if depth > 0 {
		indent = pPr{style: "ListParagraph", left: 360 * (depth + 1)}
	}
	switch bl.Kind {
	case mdbook.HeadingBlock:
		style := fmt.Sprintf("Heading%d", bl.Level)
		if title {
			style = "Title"
		}
		w.para(pPr{style: style}, func() {
			w.bookmarked(bl.Dest, func() { w.spans(part, bl.Text, runStyle{}, bl.Dest) })
		})
	case mdbook.ParagraphBlock:
		if bl.Mark != nil {
			w.para(pPr{style: "Mark"}, func() {
				w.bookmarked(bl.Mark.Dest, func() {
					lead := 0
					for lead < len(bl.Text) && bl.Text[lead].Style&mdbook.Strong != 0 {
						lead++
					}
					w.link(w.bookmark(bl.Mark.Back), "", func() { w.spans(part, bl.Text[:lead], runStyle{}, "") })
					w.spans(part, bl.Text[lead:], runStyle{}, "")
				})
			})
			return
		}
		w.para(indent, func() { w.spans(part, bl.Text, runStyle{}, "") })
	case mdbook.QuoteBlock:
		w.para(pPr{style: "Quote"}, func() { w.spans(part, bl.Text, runStyle{}, "") })
	case mdbook.RuleBlock:
		w.para(pPr{style: "Rule"}, func() {})
	case mdbook.CodeBlock:
		if bl.Lang == "mermaid" {
			w.para(pPr{style: "Caption"}, func() { w.run(w.opt.Texts.Mermaid, runStyle{}) })
		}
		for _, l := range strings.Split(strings.ReplaceAll(bl.Code, "\t", "    "), "\n") {
			w.para(pPr{style: "CodeBlock", left: indent.left}, func() { w.run(l, runStyle{}) })
		}
	case mdbook.ListBlock:
		num := 1
		if bl.Ordered {
			start := 1
			if len(bl.Items) > 0 {
				if n, err := strconv.Atoi(strings.TrimRight(bl.Items[0].Label, ".)")); err == nil {
					start = n
				}
			}
			w.nums = append(w.nums, start)
			num = len(w.nums) + 1
		}
		for i := range bl.Items {
			it := &bl.Items[i]
			w.para(pPr{style: "ListParagraph", numID: num, level: min(depth, 8)}, func() { w.spans(part, it.Text, runStyle{}, "") })
			for j := range it.Sub {
				w.block(part, &it.Sub[j], depth+1, false)
			}
		}
	case mdbook.TableBlock:
		w.table(part, bl)
	}
}

// table writes a pipe table: the header row repeated on every page the
// table crosses, the columns as wide as their longest words want.
func (w *writer) table(part string, bl *mdbook.Block) {
	if len(bl.Rows) == 0 {
		return
	}
	if w.pending || w.afterTable {
		// A table cannot start a page itself, and two tables with nothing
		// between them would be read as one: an empty paragraph comes
		// first.
		w.para(pPr{}, func() {})
	}
	cols := 0
	for _, r := range bl.Rows {
		cols = max(cols, len(r))
	}
	widths := columnWidths(bl.Rows, cols)
	w.b.WriteString(`<w:tbl><w:tblPr><w:tblStyle w:val="SpecTable"/><w:tblW w:w="5000" w:type="pct"/><w:tblLayout w:type="fixed"/><w:tblLook w:val="0420" w:firstRow="1" w:lastRow="0" w:firstColumn="0" w:lastColumn="0" w:noHBand="1" w:noVBand="1"/></w:tblPr><w:tblGrid>`)
	for _, wd := range widths {
		fmt.Fprintf(&w.b, `<w:gridCol w:w="%d"/>`, wd)
	}
	w.b.WriteString(`</w:tblGrid>`)
	for r, row := range bl.Rows {
		w.b.WriteString(`<w:tr>`)
		if r == 0 {
			w.b.WriteString(`<w:trPr><w:tblHeader/></w:trPr>`)
		}
		for c := 0; c < cols; c++ {
			fmt.Fprintf(&w.b, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/>`, widths[c])
			if r == 0 {
				w.b.WriteString(`<w:shd w:val="clear" w:color="auto" w:fill="E8EBEF"/>`)
			}
			w.b.WriteString(`</w:tcPr>`)
			var cell []mdbook.Span
			if c < len(row) {
				cell = row[c]
			}
			self := ""
			if c == 0 && bl.RowDest != nil {
				self = bl.RowDest[r]
			}
			st := runStyle{bold: r == 0}
			w.para(pPr{style: "TableText"}, func() {
				if self != "" {
					w.bookmarked(self, func() { w.spans(part, cell, st, self) })
					return
				}
				w.spans(part, cell, st, "")
			})
			w.b.WriteString(`</w:tc>`)
		}
		w.b.WriteString(`</w:tr>`)
	}
	w.b.WriteString(`</w:tbl>`)
	w.afterTable = true
}

// columnWidths shares the width between the columns by the length of the
// longest word and of the whole text each holds, so a column of short ids
// stays narrow; a word longer than 24 characters, such as a path, counts
// as 24, since Word breaks it where it must.
func columnWidths(rows [][][]mdbook.Span, cols int) []int {
	want := make([]float64, cols)
	for _, row := range rows {
		for c := 0; c < cols && c < len(row); c++ {
			text := mdbook.PlainText(row[c])
			longest := 0
			for _, word := range strings.Fields(text) {
				longest = max(longest, utf8.RuneCountInString(word))
			}
			want[c] = max(want[c], float64(min(longest, 24)), min(float64(utf8.RuneCountInString(text)), 60)/2)
		}
	}
	total := 0.0
	for c := range want {
		want[c] = max(want[c], 3)
		total += want[c]
	}
	out := make([]int, cols)
	used := 0
	for c := range want {
		out[c] = int(float64(width) * want[c] / total)
		used += out[c]
	}
	out[cols-1] += width - used
	return out
}

// pPr is what a paragraph's properties say.
type pPr struct {
	style string
	numID int // a list item's numbering, with level
	level int
	left  int // the left indent, in twentieths of a point
}

// para writes a paragraph, its properties in the order the schema gives
// them, and its content from fill.
func (w *writer) para(p pPr, fill func()) {
	w.b.WriteString(`<w:p>`)
	// A heading keeps its own space before it.
	gap := w.afterTable && !strings.HasPrefix(p.style, "Heading") && p.style != "Title"
	w.afterTable = false
	if p.style != "" || p.numID > 0 || p.left > 0 || w.pending || gap {
		w.b.WriteString(`<w:pPr>`)
		if p.style != "" {
			fmt.Fprintf(&w.b, `<w:pStyle w:val="%s"/>`, p.style)
		}
		if w.pending {
			w.b.WriteString(`<w:pageBreakBefore/>`)
			w.pending = false
		}
		if p.numID > 0 {
			fmt.Fprintf(&w.b, `<w:numPr><w:ilvl w:val="%d"/><w:numId w:val="%d"/></w:numPr>`, p.level, p.numID)
		}
		if gap {
			w.b.WriteString(`<w:spacing w:before="160"/>`)
		}
		if p.left > 0 {
			fmt.Fprintf(&w.b, `<w:ind w:left="%d"/>`, p.left)
		}
		w.b.WriteString(`</w:pPr>`)
	}
	fill()
	w.b.WriteString(`</w:p>`)
}

// bookmarked writes fill inside the bookmark of a destination, when it
// has one.
func (w *writer) bookmarked(dest string, fill func()) {
	name := w.names[dest]
	if name == "" {
		fill()
		return
	}
	id := w.nextBm
	w.nextBm++
	fmt.Fprintf(&w.b, `<w:bookmarkStart w:id="%d" w:name="%s"/>`, id, name)
	fill()
	fmt.Fprintf(&w.b, `<w:bookmarkEnd w:id="%d"/>`, id)
}

// link writes fill as a link to a bookmark or a URI, or as it is when it
// has neither.
func (w *writer) link(bookmark, uri string, fill func()) {
	switch {
	case bookmark != "":
		fmt.Fprintf(&w.b, `<w:hyperlink w:anchor="%s" w:history="1">`, bookmark)
	case uri != "":
		fmt.Fprintf(&w.b, `<w:hyperlink r:id="%s" w:history="1">`, w.rel(uri))
	default:
		fill()
		return
	}
	fill()
	w.b.WriteString(`</w:hyperlink>`)
}

func (w *writer) rel(uri string) string {
	if w.relID == nil {
		w.relID = map[string]string{}
	}
	if id, ok := w.relID[uri]; ok {
		return id
	}
	w.rels = append(w.rels, uri)
	id := fmt.Sprintf("rId%d", firstLinkRel+len(w.rels)-1)
	w.relID[uri] = id
	return id
}

// runStyle is what a run's properties say.
type runStyle struct {
	bold, italic, code, link, grey bool
}

// spans writes the inline text of a block: its styles, its links
// resolved, and each element id with a destination other than self made
// a link to it. A link that leads nowhere in the book is written with its
// target in brackets.
func (w *writer) spans(part string, spans []mdbook.Span, base runStyle, self string) {
	for i := 0; i < len(spans); i++ {
		s := spans[i]
		st := base
		st.bold = st.bold || s.Style&mdbook.Strong != 0
		st.italic = st.italic || s.Style&mdbook.Emphasis != 0
		st.code = s.Style&mdbook.Mono != 0
		if s.Link != "" {
			// The spans of one link go into one hyperlink.
			j := i
			for j+1 < len(spans) && spans[j+1].Link == s.Link {
				j++
			}
			group := spans[i : j+1]
			i = j
			dest, uri := w.mb.Resolve(part, s.Link)
			bm := w.bookmark(dest)
			if bm == "" && uri == "" {
				w.plain(group, base)
				w.run(" ("+s.Link+")", runStyle{grey: true})
				continue
			}
			w.link(bm, uri, func() {
				for _, g := range group {
					gs := base
					gs.bold = gs.bold || g.Style&mdbook.Strong != 0
					gs.italic = gs.italic || g.Style&mdbook.Emphasis != 0
					gs.code = g.Style&mdbook.Mono != 0
					gs.link = true
					w.run(g.Text, gs)
				}
			})
			continue
		}
		if st.code || len(w.mb.IDDest) == 0 {
			w.run(s.Text, st)
			continue
		}
		for _, seg := range w.mb.IDLinks(s.Text, self) {
			if seg.Dest == "" {
				w.run(seg.Text, st)
				continue
			}
			ls := st
			ls.link = true
			w.link(w.bookmark(seg.Dest), "", func() { w.run(seg.Text, ls) })
		}
	}
}

func (w *writer) plain(spans []mdbook.Span, base runStyle) {
	for _, s := range spans {
		st := base
		st.bold = st.bold || s.Style&mdbook.Strong != 0
		st.italic = st.italic || s.Style&mdbook.Emphasis != 0
		st.code = s.Style&mdbook.Mono != 0
		w.run(s.Text, st)
	}
}

// run writes one run of text with its properties in the schema's order.
func (w *writer) run(text string, st runStyle) {
	if text == "" {
		return
	}
	w.b.WriteString(`<w:r>`)
	if st != (runStyle{}) {
		w.b.WriteString(`<w:rPr>`)
		switch {
		case st.link:
			w.b.WriteString(`<w:rStyle w:val="Hyperlink"/>`)
		case st.code:
			w.b.WriteString(`<w:rStyle w:val="CodeChar"/>`)
		}
		if st.link && st.code {
			w.b.WriteString(`<w:rFonts w:ascii="Courier New" w:hAnsi="Courier New" w:cs="Courier New"/>`)
		}
		if st.bold {
			w.b.WriteString(`<w:b/>`)
		}
		if st.italic {
			w.b.WriteString(`<w:i/>`)
		}
		if st.grey {
			w.b.WriteString(`<w:color w:val="666666"/>`)
		}
		w.b.WriteString(`</w:rPr>`)
	}
	fmt.Fprintf(&w.b, `<w:t xml:space="preserve">%s</w:t></w:r>`, esc(text))
}

// esc escapes a text for XML. A character XML 1.0 cannot hold is written
// as its code point in brackets, never dropped, as the PDF writer does
// with a character its fonts lack.
func esc(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '&':
			b.WriteString("&amp;")
		case r == '<':
			b.WriteString("&lt;")
		case r == '>':
			b.WriteString("&gt;")
		case r == '"':
			b.WriteString("&quot;")
		case r == '\t':
			b.WriteString("    ")
		case r < 0x20 || r == 0xFFFE || r == 0xFFFF || r >= 0xD800 && r <= 0xDFFF || r == utf8.RuneError:
			fmt.Fprintf(&b, "[U+%04X]", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// The relationships of document.xml: the parts first, then one per
// external link from firstLinkRel on.
const firstLinkRel = 7

// pack writes the package: its parts in a fixed order, the content types
// first, each entry dated zipTime.
func (w *writer) pack() ([]byte, error) {
	lang := w.opt.Lang
	if lang == "" {
		lang = "en"
	}
	header := w.opt.Header
	if header == "" {
		header = w.opt.Title
	}
	var doc strings.Builder
	doc.WriteString(xmlHead + `<w:document xmlns:w="` + nsW + `" xmlns:r="` + nsR + `"><w:body>`)
	doc.WriteString(w.b.String())
	fmt.Fprintf(&doc, `<w:sectPr><w:headerReference w:type="default" r:id="rId4"/><w:footerReference w:type="default" r:id="rId5"/><w:pgSz w:w="%d" w:h="%d"/><w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="567" w:footer="567" w:gutter="0"/><w:cols w:space="708"/></w:sectPr>`, pageW, pageH, margin, margin, margin, margin)
	doc.WriteString(`</w:body></w:document>`)

	var rels strings.Builder
	rels.WriteString(xmlHead + `<Relationships xmlns="` + nsRel + `">`)
	for i, p := range []struct{ typ, target string }{{"styles", "styles.xml"}, {"numbering", "numbering.xml"}, {"settings", "settings.xml"}, {"header", "header1.xml"}, {"footer", "footer1.xml"}, {"fontTable", "fontTable.xml"}} {
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="%s%s" Target="%s"/>`, i+1, relT, p.typ, p.target)
	}
	for i, uri := range w.rels {
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="%shyperlink" Target="%s" TargetMode="External"/>`, firstLinkRel+i, relT, esc(uri))
	}
	rels.WriteString(`</Relationships>`)

	parts := []struct{ name, data string }{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", packageRels},
		{"docProps/core.xml", fmt.Sprintf(coreXML, esc(w.opt.Title), esc(lang))},
		{"docProps/app.xml", appXML},
		{"word/document.xml", doc.String()},
		{"word/_rels/document.xml.rels", rels.String()},
		{"word/styles.xml", fmt.Sprintf(stylesXML, esc(lang))},
		{"word/numbering.xml", w.numbering()},
		{"word/settings.xml", settingsXML},
		{"word/header1.xml", fmt.Sprintf(headerXML, esc(header))},
		{"word/footer1.xml", w.footer()},
		{"word/fontTable.xml", fontTableXML},
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for _, p := range parts {
		f, err := z.CreateHeader(&zip.FileHeader{Name: p.name, Method: zip.Deflate, Modified: zipTime})
		if err != nil {
			return nil, fmt.Errorf("%s: %v", p.name, err)
		}
		if _, err := f.Write([]byte(p.data)); err != nil {
			return nil, fmt.Errorf("%s: %v", p.name, err)
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// footer is the page's footer: PageOf with each %d a field, the page
// number and the number of pages, which Word fills in as it lays out.
func (w *writer) footer() string {
	var b strings.Builder
	b.WriteString(xmlHead + `<w:ftr xmlns:w="` + nsW + `" xmlns:r="` + nsR + `"><w:p><w:pPr><w:pStyle w:val="Footer"/><w:jc w:val="center"/></w:pPr>`)
	pieces := strings.Split(w.opt.Texts.PageOf, "%d")
	fields := []string{"PAGE", "NUMPAGES"}
	for i, piece := range pieces {
		if piece != "" {
			fmt.Fprintf(&b, `<w:r><w:t xml:space="preserve">%s</w:t></w:r>`, esc(piece))
		}
		if i < len(pieces)-1 && i < len(fields) {
			fmt.Fprintf(&b, `<w:fldSimple w:instr=" %s "><w:r><w:t>1</w:t></w:r></w:fldSimple>`, fields[i])
		}
	}
	b.WriteString(`</w:p></w:ftr>`)
	return b.String()
}

// numbering is numbering.xml: one bullet list (numId 1) and one numbered
// list per numbered block, so each starts at its own first number.
func (w *writer) numbering() string {
	var b strings.Builder
	b.WriteString(xmlHead + `<w:numbering xmlns:w="` + nsW + `">`)
	bullets := []string{"•", "◦", "▪"}
	for a, kind := range []string{"bullet", "decimal"} {
		fmt.Fprintf(&b, `<w:abstractNum w:abstractNumId="%d"><w:multiLevelType w:val="hybridMultilevel"/>`, a)
		for l := 0; l < 9; l++ {
			text := fmt.Sprintf("%%%d.", l+1)
			if kind == "bullet" {
				text = bullets[l%len(bullets)]
			}
			fmt.Fprintf(&b, `<w:lvl w:ilvl="%d"><w:start w:val="1"/><w:numFmt w:val="%s"/><w:lvlText w:val="%s"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="%d" w:hanging="360"/></w:pPr></w:lvl>`, l, kind, text, 360*(l+2))
		}
		b.WriteString(`</w:abstractNum>`)
	}
	b.WriteString(`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`)
	for i, start := range w.nums {
		fmt.Fprintf(&b, `<w:num w:numId="%d"><w:abstractNumId w:val="1"/>`, i+2)
		for l := 0; l < 9; l++ {
			fmt.Fprintf(&b, `<w:lvlOverride w:ilvl="%d"><w:startOverride w:val="%d"/></w:lvlOverride>`, l, start)
		}
		b.WriteString(`</w:num>`)
	}
	b.WriteString(`</w:numbering>`)
	return b.String()
}

const xmlHead = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

const contentTypes = xmlHead + `<Types xmlns="` + nsCT + `">` +
	`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
	`<Default Extension="xml" ContentType="application/xml"/>` +
	`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
	`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
	`<Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>` +
	`<Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>` +
	`<Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>` +
	`<Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/>` +
	`<Override PartName="/word/fontTable.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml"/>` +
	`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
	`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>` +
	`</Types>`

const packageRels = xmlHead + `<Relationships xmlns="` + nsRel + `">` +
	`<Relationship Id="rId1" Type="` + relT + `officeDocument" Target="word/document.xml"/>` +
	`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
	`<Relationship Id="rId3" Type="` + relT + `extended-properties" Target="docProps/app.xml"/>` +
	`</Relationships>`

// coreXML holds the title and the language, and no date: a date would
// make every run differ.
const coreXML = xmlHead + `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><dc:title>%s</dc:title><dc:language>%s</dc:language></cp:coreProperties>`

const appXML = xmlHead + `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"><Application>SpecArch</Application></Properties>`

const settingsXML = xmlHead + `<w:settings xmlns:w="` + nsW + `"><w:defaultTabStop w:val="720"/><w:characterSpacingControl w:val="doNotCompress"/><w:compat><w:compatSetting w:name="compatibilityMode" w:uri="http://schemas.microsoft.com/office/word" w:val="15"/></w:compat></w:settings>`

// fontTableXML describes the two fonts the styles name: their family and
// pitch, by which a reader without one picks its replacement.
const fontTableXML = xmlHead + `<w:fonts xmlns:w="` + nsW + `">` +
	`<w:font w:name="Calibri"><w:panose1 w:val="020F0502020204030204"/><w:charset w:val="00"/><w:family w:val="swiss"/><w:pitch w:val="variable"/></w:font>` +
	`<w:font w:name="Courier New"><w:panose1 w:val="02070309020205020404"/><w:charset w:val="00"/><w:family w:val="modern"/><w:pitch w:val="fixed"/></w:font>` +
	`</w:fonts>`

const headerXML = xmlHead + `<w:hdr xmlns:w="` + nsW + `" xmlns:r="` + nsR + `"><w:p><w:pPr><w:pStyle w:val="Header"/></w:pPr><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p></w:hdr>`
