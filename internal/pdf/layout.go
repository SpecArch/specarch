package pdf

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// elem is one element of the structure tree.
type elem struct {
	typ    string
	parent *elem
	kids   []kid
	obj    int
}

// kid is what an element holds, in reading order: an element, a sequence
// of marked content on a page, or a link annotation.
type kid struct {
	elem  *elem
	mc    *mcSeq
	annot *annot
}

// mcSeq is one sequence of marked content, by its page and number there.
type mcSeq struct {
	elem *elem
	page *page
	mcid int
}

type opKind int

const (
	textOp opKind = iota
	rectOp
	strokeOp
)

// op is one thing drawn on a page; y grows downward from the top edge: a
// text's baseline, a rectangle's top.
type op struct {
	kind       opKind
	x, y, w, h float64
	face       face
	size       float64
	color      rgb
	text       string
	deferred   func() string // a text known only once every page is laid out
	alignRight bool          // x is the right edge of a deferred text
	mc         *mcSeq        // nil for an artifact
}

type annot struct {
	page       *page
	x, y, w, h float64
	dest, uri  string
	contents   string
	elem       *elem
	obj, key   int
}

type page struct {
	n      int
	ops    []op
	annots []*annot
	mcs    []*mcSeq
	last   *mcSeq
	obj    int
	cobj   int
}

// piece is a run of text of one look and one link.
type piece struct {
	text      string
	face      face
	size      float64
	color     rgb
	dest, uri string
	group     int // the link it belongs to, 0 for none
}

type frag struct {
	p    *piece
	text string
	x, w float64
}

type line struct {
	frags []frag
	w     float64
}

func (b *book) newPage() {
	b.cur = &page{n: len(b.pages) + 1}
	b.pages = append(b.pages, b.cur)
	b.y = top
}

// ensure starts a new page when h does not fit on this one.
func (b *book) ensure(h float64) {
	if b.y+h > bottom && b.y > top {
		b.newPage()
	}
}

func (b *book) add(parent *elem, typ string) *elem {
	e := &elem{typ: typ, parent: parent}
	parent.kids = append(parent.kids, kid{elem: e})
	return e
}

func (b *book) setDest(name string) {
	if d := b.dests[name]; d != nil {
		d.page, d.y = b.cur, b.y
	}
}

// draw puts a text on the page as content of e, or as an artifact when e
// is nil.
func (b *book) draw(e *elem, x, baseline float64, f face, size float64, c rgb, s string) {
	o := op{kind: textOp, x: x, y: baseline, face: f, size: size, color: c, text: s}
	b.mark(e, &o)
	b.cur.ops = append(b.cur.ops, o)
}

func (b *book) mark(e *elem, o *op) {
	p := b.cur
	if e == nil {
		p.last = nil
		return
	}
	if p.last == nil || p.last.elem != e {
		mc := &mcSeq{elem: e, page: p, mcid: len(p.mcs)}
		p.mcs = append(p.mcs, mc)
		e.kids = append(e.kids, kid{mc: mc})
		p.last = mc
	}
	o.mc = p.last
}

func (b *book) rect(x, y, w, h float64, c rgb) {
	b.cur.last = nil
	b.cur.ops = append(b.cur.ops, op{kind: rectOp, x: x, y: y, w: w, h: h, color: c})
}

func (b *book) stroke(x, y, w, h float64, c rgb) {
	b.cur.last = nil
	b.cur.ops = append(b.cur.ops, op{kind: strokeOp, x: x, y: y, w: w, h: h, color: c})
}

func (b *book) link(e *elem, x, y, w, h float64, dest, uri, contents string) {
	a := &annot{page: b.cur, x: x, y: y, w: w, h: h, dest: dest, uri: uri, contents: contents, elem: e}
	b.cur.annots = append(b.cur.annots, a)
	e.kids = append(e.kids, kid{annot: a})
}

// pieces turns the spans of a block into pieces: the face of each style,
// links resolved, and each element id that has a destination other than
// self made a link to it.
func (b *book) pieces(part string, spans []span, size float64, base face, self string) []piece {
	var out []piece
	group := 0
	for i, s := range spans {
		f := base
		if s.Style&strong != 0 {
			f = bold
		}
		if s.Style&emphasis != 0 {
			if f == bold {
				f = boldItalic
			} else {
				f = italic
			}
		}
		sz := size
		if s.Style&monoStyle != 0 {
			f, sz = mono, size*0.92
		}
		if s.Link != "" {
			d, u := b.mb.Resolve(part, s.Link)
			if d == "" && u == "" {
				out = append(out, piece{text: s.Text, face: f, size: sz, color: black})
				if i+1 >= len(spans) || spans[i+1].Link != s.Link {
					out = append(out, piece{text: " (" + s.Link + ")", face: regular, size: size * 0.9, color: grey})
				}
				continue
			}
			if i == 0 || spans[i-1].Link != s.Link {
				group++
			}
			out = append(out, piece{text: s.Text, face: f, size: sz, color: linkBlue, dest: d, uri: u, group: group})
			continue
		}
		if s.Style&monoStyle != 0 || len(b.mb.IDDest) == 0 {
			out = append(out, piece{text: s.Text, face: f, size: sz, color: black})
			continue
		}
		// Element ids inside the text.
		for _, seg := range b.mb.IDLinks(s.Text, self) {
			if seg.Dest == "" {
				out = append(out, piece{text: seg.Text, face: f, size: sz, color: black})
				continue
			}
			group++
			out = append(out, piece{text: seg.Text, face: f, size: sz, color: linkBlue, dest: seg.Dest, group: group})
		}
	}
	return out
}

// lines breaks pieces into lines no wider than max: at spaces, and inside
// a word only when the word alone is wider.
func (b *book) lines(pieces []piece, max float64) []line {
	var out []line
	var cur line
	flush := func() {
		// Spaces at the end of a line take no width.
		for len(cur.frags) > 0 {
			last := &cur.frags[len(cur.frags)-1]
			t := strings.TrimRight(last.text, " ")
			if t == last.text {
				break
			}
			cur.w -= last.w - b.m[last.p.face].width(t, last.p.size)
			last.w = b.m[last.p.face].width(t, last.p.size)
			last.text = t
			if t != "" {
				break
			}
			cur.frags = cur.frags[:len(cur.frags)-1]
		}
		out = append(out, cur)
		cur = line{}
	}
	push := func(p *piece, t string, w float64) {
		if n := len(cur.frags); n > 0 && cur.frags[n-1].p == p {
			cur.frags[n-1].text += t
			cur.frags[n-1].w += w
		} else {
			cur.frags = append(cur.frags, frag{p: p, text: t, x: cur.w, w: w})
		}
		cur.w += w
	}
	for i := range pieces {
		p := &pieces[i]
		m := b.m[p.face]
		for _, tok := range tokens(p.text) {
			if len(cur.frags) == 0 {
				tok = strings.TrimLeft(tok, " ")
				if tok == "" {
					continue
				}
			}
			word := strings.TrimRight(tok, " ")
			ww := m.width(word, p.size)
			tw := m.width(tok, p.size)
			if cur.w+ww <= max || (len(cur.frags) == 0 && ww <= max) {
				push(p, tok, tw)
				continue
			}
			if ww <= max {
				flush()
				push(p, tok, tw)
				continue
			}
			// A word wider than the line is cut where it fills it.
			rest := tok
			for rest != "" {
				room := max - cur.w
				cut := 0
				w := 0.0
				for cut < len(rest) {
					_, size := utf8.DecodeRuneInString(rest[cut:])
					cw := m.width(rest[cut:cut+size], p.size)
					if w+cw > room && (cut > 0 || len(cur.frags) > 0) {
						break
					}
					w += cw
					cut += size
				}
				if cut == 0 {
					flush()
					continue
				}
				push(p, rest[:cut], w)
				rest = rest[cut:]
				if rest != "" {
					flush()
				}
			}
		}
	}
	if len(cur.frags) > 0 || len(out) == 0 {
		flush()
	}
	return out
}

// tokens splits a text into words, each with the spaces after it.
func tokens(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' && (i+1 == len(s) || s[i+1] != ' ') {
			out = append(out, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// box is what is drawn behind the lines of a block: a fill, and a bar on
// its left.
type box struct {
	fill *rgb
	bar  *rgb
	pad  float64
}

// place puts lines on the pages from x, one per leading, as content of e;
// a link's text goes into a Link element of e, one per link, with an
// annotation over each line of it.
func (b *book) place(e *elem, lines []line, x, leading float64, bx *box) {
	links := map[int]*elem{}
	segPage, segIdx, segY := b.cur, len(b.cur.ops), b.y
	closeSeg := func() {
		if bx == nil || segPage == nil {
			return
		}
		h := b.y - segY
		if segPage != b.cur {
			h = bottom - segY
		}
		var ops []op
		if bx.fill != nil {
			ops = append(ops, op{kind: rectOp, x: x - bx.pad, y: segY - bx.pad/2, w: width - (x - left) + bx.pad, h: h + bx.pad, color: *bx.fill})
		}
		if bx.bar != nil {
			ops = append(ops, op{kind: rectOp, x: x - bx.pad, y: segY - bx.pad/2, w: 2.2, h: h + bx.pad, color: *bx.bar})
		}
		segPage.ops = slices.Insert(segPage.ops, segIdx, ops...)
	}
	for _, ln := range lines {
		before := b.cur
		b.ensure(leading)
		if b.cur != before {
			closeSeg()
			segPage, segIdx, segY = b.cur, len(b.cur.ops), b.y
		}
		baseline := b.y + leading*0.76
		for _, f := range ln.frags {
			owner := e
			if f.p.group > 0 {
				le := links[f.p.group]
				if le == nil {
					le = b.add(e, "Link")
					links[f.p.group] = le
				}
				owner = le
			}
			b.draw(owner, x+f.x, baseline, f.p.face, f.p.size, f.p.color, f.text)
			if f.p.group > 0 && (f.p.dest != "" || f.p.uri != "") {
				b.link(owner, x+f.x, b.y, f.w, leading, f.p.dest, f.p.uri, f.text)
			}
		}
		b.y += leading
	}
	if bx != nil {
		closeSeg()
	}
}

// markGroup is the link group of a mark's lead, apart from the groups
// pieces numbers from 1.
const markGroup = 1 << 30

var headingSize = [7]float64{0, 17, 14, 12, 10.5, 10, 10}

const bodySize = 9.5

func (b *book) layout() {
	b.newPage()
	for i, bl := range b.mb.Parts[0].Blocks {
		if i == 0 && bl.Kind == headingBlock {
			b.y += 60
		}
		b.block("front", &b.mb.Parts[0].Blocks[i], b.root, left, i == 0)
	}

	b.newPage()
	b.setDest("contents")
	b.outline = append(b.outline, outlineEntry{level: 1, title: b.opt.Texts.Contents, dest: "contents"})
	h := b.add(b.root, "H1")
	b.place(h, b.lines([]piece{{text: b.opt.Texts.Contents, face: bold, size: headingSize[1], color: black}}, width), left, headingSize[1]*1.4, nil)
	b.y += 8
	toc := b.add(b.root, "TOC")
	for _, t := range b.mb.TOC {
		b.tocEntry(toc, t)
	}
	// The problems and gaps list ends the book; its page is known once the
	// layout is done, as every other entry's is.
	b.tocEntry(toc, tocEntry{Level: 1, Text: b.opt.Texts.Problems, Dest: "problems"})

	for pi := 1; pi < len(b.mb.Parts); pi++ {
		b.newPage()
		p := &b.mb.Parts[pi]
		for i := range p.Blocks {
			b.block(p.Name, &p.Blocks[i], b.root, left, false)
		}
	}

	b.problems()
}

func (b *book) tocEntry(toc *elem, t tocEntry) {
	size := 9.5
	f := regular
	if t.Level == 1 {
		f, size = bold, 10
	}
	leading := size * 1.45
	indent := float64(t.Level-1) * 14
	ls := b.lines([]piece{{text: t.Text, face: f, size: size, color: black}}, width-indent-40)
	if t.Level == 1 {
		b.y += 4
	}
	b.ensure(leading * float64(len(ls)))
	item := b.add(toc, "TOCI")
	le := b.add(item, "Link")
	y0 := b.y
	b.place(le, ls, left+indent, leading, nil)
	name := t.Dest
	o := op{kind: textOp, x: right, y: b.y - leading*0.24, face: f, size: size, color: black, alignRight: true, deferred: func() string {
		if d := b.dests[name]; d != nil && d.page != nil {
			return strconv.Itoa(d.page.n)
		}
		return ""
	}}
	b.mark(le, &o)
	b.cur.ops = append(b.cur.ops, o)
	// ensure kept the entry on one page.
	b.link(le, left+indent, y0, width-indent, b.y-y0, name, "", t.Text)
}

// block lays out one block of a part under parent, from x.
func (b *book) block(part string, bl *block, parent *elem, x float64, title bool) {
	avail := right - x
	switch bl.Kind {
	case headingBlock:
		lvl := bl.Level
		size := headingSize[lvl]
		if title {
			size = 22
		}
		leading := size * 1.3
		ls := b.lines(b.pieces(part, bl.Text, size, bold, bl.Dest), avail)
		if b.y > top {
			b.y += []float64{0, 14, 12, 9, 7, 6, 6}[lvl]
		}
		b.ensure(leading*float64(len(ls)) + 3*bodySize*1.4)
		b.setDest(bl.Dest)
		if lvl <= 4 {
			b.outline = append(b.outline, outlineEntry{level: lvl, title: plainText(bl.Text), dest: bl.Dest})
		}
		h := b.add(parent, fmt.Sprintf("H%d", lvl))
		b.place(h, ls, x, leading, nil)
		b.y += 3
		if title {
			b.y += 18
		}
	case paragraphBlock:
		e := b.add(parent, "P")
		ps := b.pieces(part, bl.Text, bodySize, regular, "")
		leading := bodySize * 1.38
		var bx *box
		if bl.Mark != nil {
			// The mark's lead links to its entry in the list.
			for i := range ps {
				if i > 0 && ps[i].face != bold {
					break
				}
				ps[i].dest, ps[i].uri, ps[i].group, ps[i].color = bl.Mark.Back, "", markGroup, linkBlue
			}
			bx = &box{fill: &markFill, bar: &markBar, pad: 5}
			x += 6
			b.y += 3
			b.ensure(leading * 2)
			b.setDest(bl.Mark.Dest)
			ls := b.lines(ps, right-x-4)
			b.place(e, ls, x, leading, bx)
			b.y += 8
			return
		}
		b.place(e, b.lines(ps, avail), x, leading, nil)
		b.y += 5
	case quoteBlock:
		e := b.add(parent, "BlockQuote")
		leading := bodySize * 1.38
		ls := b.lines(b.pieces(part, bl.Text, bodySize, italic, ""), avail-12)
		b.place(e, ls, x+12, leading, &box{bar: &quoteGrey, pad: 6})
		b.y += 5
	case ruleBlock:
		b.ensure(10)
		b.rect(x, b.y+4, avail, 0.6, ruleGrey)
		b.y += 10
	case codeBlock:
		if bl.Lang == "mermaid" {
			p := b.add(parent, "P")
			b.place(p, b.lines([]piece{{text: b.opt.Texts.Mermaid, face: italic, size: 8.5, color: grey}}, avail), x, 8.5*1.4, nil)
		}
		e := b.add(parent, "Code")
		size := 7.5
		var ls []line
		for _, l := range strings.Split(strings.ReplaceAll(bl.Code, "\t", "    "), "\n") {
			ps := []piece{{text: l, face: mono, size: size, color: black}}
			if l == "" {
				ps[0].text = " "
			}
			ls = append(ls, b.codeLines(&ps[0], avail-8)...)
		}
		b.y += 3
		b.place(e, ls, x+4, size*1.35, &box{fill: &codeFill, pad: 4})
		b.y += 7
	case listBlock:
		b.list(part, bl, parent, x)
	case tableBlock:
		b.table(part, bl, parent, x)
	}
}

// codeLines cuts a line of a fenced block where it fills the width,
// keeping its spaces.
func (b *book) codeLines(p *piece, max float64) []line {
	m := b.m[p.face]
	var out []line
	rest := p.text
	for {
		cut, w := 0, 0.0
		for cut < len(rest) {
			_, size := utf8.DecodeRuneInString(rest[cut:])
			cw := m.width(rest[cut:cut+size], p.size)
			if w+cw > max && cut > 0 {
				break
			}
			w += cw
			cut += size
		}
		out = append(out, line{frags: []frag{{p: p, text: rest[:cut], w: w}}, w: w})
		rest = rest[cut:]
		if rest == "" {
			return out
		}
	}
}

func (b *book) list(part string, bl *block, parent *elem, x float64) {
	l := b.add(parent, "L")
	leading := bodySize * 1.38
	indent := 14.0
	if bl.Ordered {
		indent = 20
	}
	for i := range bl.Items {
		it := &bl.Items[i]
		li := b.add(l, "LI")
		ls := b.lines(b.pieces(part, it.Text, bodySize, regular, ""), right-x-indent)
		b.ensure(leading)
		lbl := b.add(li, "Lbl")
		b.draw(lbl, x+2, b.y+leading*0.76, regular, bodySize, black, it.Label)
		body := b.add(li, "LBody")
		b.place(body, ls, x+indent, leading, nil)
		b.y += 1.5
		for j := range it.Sub {
			b.block(part, &it.Sub[j], body, x+indent, false)
		}
	}
	b.y += 4
}

// table lays out a pipe table: column widths from the cells' words, the
// header row repeated on each page the table crosses (as an artifact
// there), and a row taller than a page cut between its lines.
func (b *book) table(part string, bl *block, parent *elem, x float64) {
	if len(bl.Rows) == 0 {
		return
	}
	cols := 0
	for _, r := range bl.Rows {
		cols = max(cols, len(r))
	}
	size := 8.0
	if cols >= 7 {
		size = 7.0
	}
	const pad = 3.0
	leading := size * 1.3
	avail := right - x
	cells := make([][][]piece, len(bl.Rows))
	minW := make([]float64, cols)
	natW := make([]float64, cols)
	for r, row := range bl.Rows {
		cells[r] = make([][]piece, cols)
		for c := 0; c < cols; c++ {
			var spans []span
			if c < len(row) {
				spans = row[c]
			}
			base := regular
			if r == 0 {
				base = bold
			}
			self := ""
			if bl.RowDest != nil && c == 0 {
				self = bl.RowDest[r]
			}
			ps := b.pieces(part, spans, size, base, self)
			cells[r][c] = ps
			nat := 0.0
			for _, p := range ps {
				m := b.m[p.face]
				for _, tok := range tokens(p.text) {
					minW[c] = max(minW[c], m.width(strings.TrimRight(tok, " "), p.size))
				}
				nat += m.width(p.text, p.size)
			}
			natW[c] = max(natW[c], nat)
		}
	}
	widths := make([]float64, cols)
	sumNat, sumMin := 0.0, 0.0
	for c := range widths {
		minW[c] = min(minW[c], avail*0.3) + 2*pad
		natW[c] = max(natW[c]+2*pad, minW[c])
		sumNat += natW[c]
		sumMin += minW[c]
	}
	switch {
	case sumNat <= avail:
		copy(widths, natW)
	case sumMin >= avail:
		for c := range widths {
			widths[c] = minW[c] * avail / sumMin
		}
	default:
		extra := avail - sumMin
		for c := range widths {
			widths[c] = minW[c] + extra*(natW[c]-minW[c])/(sumNat-sumMin)
		}
	}
	rowLines := make([][][]line, len(bl.Rows))
	for r := range bl.Rows {
		rowLines[r] = make([][]line, cols)
		for c := 0; c < cols; c++ {
			rowLines[r][c] = b.lines(cells[r][c], widths[c]-2*pad)
		}
	}
	height := func(r, from int) float64 {
		n := 0
		for c := 0; c < cols; c++ {
			n = max(n, len(rowLines[r][c])-from)
		}
		return float64(n)*leading + 2*pad
	}
	t := b.add(parent, "Table")
	b.y += 2
	// drawRow draws the lines of row r from line from on, as many as fit,
	// into the cells given, or as an artifact when cells is nil (the
	// header repeated on a later page), and says how many it drew.
	drawRow := func(r, from int, cellElems []*elem) int {
		n := 0
		for c := 0; c < cols; c++ {
			n = max(n, len(rowLines[r][c])-from)
		}
		n = max(1, min(n, int((bottom-b.y-2*pad)/leading)))
		h := float64(n)*leading + 2*pad
		if r == 0 {
			b.rect(x, b.y, sumOf(widths), h, headFill)
		}
		cx := x
		for c := 0; c < cols; c++ {
			ls := rowLines[r][c]
			end := min(len(ls), from+n)
			if from < end {
				y := b.y
				b.y += pad
				if cellElems != nil {
					b.place(cellElems[c], ls[from:end], cx+pad, leading, nil)
				} else {
					for _, ln := range ls[from:end] {
						for _, f := range ln.frags {
							b.draw(nil, cx+pad+f.x, b.y+leading*0.76, f.p.face, f.p.size, f.p.color, f.text)
						}
						b.y += leading
					}
				}
				b.y = y
			}
			cx += widths[c]
		}
		cx = x
		for c := 0; c < cols; c++ {
			b.stroke(cx, b.y, widths[c], h, ruleGrey)
			cx += widths[c]
		}
		b.y += h
		return n
	}
	var group *elem
	for r := range bl.Rows {
		switch r {
		case 0:
			group = b.add(t, "THead")
			need := height(0, 0)
			if len(bl.Rows) > 1 {
				need += min(height(1, 0), 3*leading+2*pad)
			}
			b.ensure(need)
		case 1:
			group = b.add(t, "TBody")
		}
		tr := b.add(group, "TR")
		cellElems := make([]*elem, cols)
		for c := range cellElems {
			typ := "TD"
			if r == 0 {
				typ = "TH"
			}
			cellElems[c] = b.add(tr, typ)
		}
		headerH := 0.0
		if r > 0 {
			headerH = height(0, 0)
		}
		for from := 0; from < maxLines(rowLines[r]) || from == 0; {
			fresh := b.y <= top+headerH+0.5
			if b.y+height(r, from) > bottom && !fresh {
				b.newPage()
				if r > 0 {
					drawRow(0, 0, nil)
				}
			}
			if from == 0 && bl.RowDest != nil && bl.RowDest[r] != "" {
				b.setDest(bl.RowDest[r])
			}
			from += drawRow(r, from, cellElems)
		}
	}
	b.y += 7
}

func maxLines(cells [][]line) int {
	n := 0
	for _, c := range cells {
		n = max(n, len(c))
	}
	return n
}

func sumOf(v []float64) float64 {
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s
}

// problems lays out the list of the problems and gaps the book marks,
// each linking to its mark and back.
func (b *book) problems() {
	b.newPage()
	b.setDest("problems")
	tx := b.opt.Texts
	b.outline = append(b.outline, outlineEntry{level: 1, title: tx.Problems, dest: "problems"})
	h := b.add(b.root, "H1")
	b.place(h, b.lines([]piece{{text: tx.Problems, face: bold, size: headingSize[1], color: black}}, width), left, headingSize[1]*1.3, nil)
	b.y += 6
	p := b.add(b.root, "P")
	intro := tx.NoProblems
	if len(b.mb.Marks) > 0 {
		intro = fmt.Sprintf(tx.ProblemsIntro, len(b.mb.Marks))
	}
	b.place(p, b.lines([]piece{{text: intro, face: regular, size: bodySize, color: black}}, width), left, bodySize*1.38, nil)
	b.y += 6
	if len(b.mb.Marks) == 0 {
		return
	}
	leading := 8.5 * 1.38
	var l *elem
	part := ""
	for _, mk := range b.mb.Marks {
		if l == nil || mk.Part != part {
			// The marks of each document under its title.
			part = mk.Part
			title := b.mb.Titles[part]
			if part == "front" {
				// The front matter's first heading is the book's title,
				// which a document's own heading may repeat.
				title = tx.FrontMatter
			}
			b.y += 6
			b.ensure(12 * 1.3 * 3)
			h := b.add(b.root, "H2")
			b.place(h, b.lines([]piece{{text: title, face: bold, size: 12, color: black}}, width), left, 12*1.3, nil)
			b.y += 3
			l = b.add(b.root, "L")
		}
		li := b.add(l, "LI")
		label := fmt.Sprintf("%d.", mk.N)
		at := ""
		if d := b.dests[mk.Dest]; d != nil && d.page != nil {
			at = fmt.Sprintf(tx.OnPage, d.page.n)
		}
		ps := []piece{
			{text: mk.Text + " ", face: regular, size: 8.5, color: black},
			{text: at, face: regular, size: 8.5, color: linkBlue, dest: mk.Dest, group: 1},
		}
		ls := b.lines(ps, width-26)
		b.ensure(leading)
		b.setDest(mk.Back)
		lbl := b.add(li, "Lbl")
		le := b.add(lbl, "Link")
		b.draw(le, left, b.y+leading*0.76, bold, 8.5, linkBlue, label)
		b.link(le, left, b.y, 22, leading, mk.Dest, "", label)
		body := b.add(li, "LBody")
		b.place(body, ls, left+26, leading, nil)
		b.y += 3
	}
}
