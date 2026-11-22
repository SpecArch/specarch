package pdf

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// emitter writes the objects of the file, each once, by number.
type emitter struct {
	b       *book
	n       int
	objects map[int][]byte
	used    [faces]map[uint16]rune // the glyphs each face draws, and a character of each
	streams map[int]bool           // the objects that are streams, which no object stream holds
	short   map[string]string      // each destination's name in the file
}

func (e *emitter) next() int {
	e.n++
	return e.n
}

func (e *emitter) obj(n int, format string, args ...any) {
	e.objects[n] = []byte(fmt.Sprintf(format, args...))
}

func (e *emitter) stream(n int, dict string, data []byte) {
	z := bytes.NewBuffer(deflate(data))
	extra := ""
	if dict != "" {
		extra = " " + dict
	}
	e.objects[n] = []byte(fmt.Sprintf("<< /Length %d /Filter /FlateDecode%s >>\nstream\n%s\nendstream", z.Len(), extra, z.Bytes()))
	e.streams[n] = true
}

func deflate(data []byte) []byte {
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	_, _ = w.Write(data)
	_ = w.Close()
	return z.Bytes()
}

// dest is a destination's name in the file: d and its rank among the
// names, since a name may be no longer than 127 bytes and a heading's
// anchor can be longer.
func (e *emitter) dest(n string) string { return e.short[n] }

const (
	catalogObj = iota + 1
	pagesObj
	structObj
	parentTreeObj
	outlinesObj
	infoObj
)

func (b *book) emit() ([]byte, error) {
	e := &emitter{b: b, n: infoObj, objects: map[int][]byte{}, streams: map[int]bool{}, short: map[string]string{}}
	for i := range e.used {
		e.used[i] = map[uint16]rune{}
	}
	for _, p := range b.pages {
		p.obj, p.cobj = e.next(), e.next()
	}
	var names []string
	for n, d := range b.dests {
		if d.page != nil {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for i, n := range names {
		e.short[n] = fmt.Sprintf("d%d", i+1)
	}
	key := len(b.pages)
	var annots []*annot
	for _, p := range b.pages {
		for _, a := range p.annots {
			a.obj, a.key = e.next(), key
			key++
			annots = append(annots, a)
		}
	}
	// The content streams first: they say which glyphs each face draws.
	contents := make([][]byte, len(b.pages))
	for i, p := range b.pages {
		contents[i] = e.content(p)
	}
	fontObj := map[face]int{}
	var fontRes []string
	for f := regular; f < faces; f++ {
		if len(e.used[f]) == 0 {
			continue
		}
		n, err := e.font(f)
		if err != nil {
			return nil, err
		}
		fontObj[f] = n
		fontRes = append(fontRes, fmt.Sprintf("/F%d %d 0 R", f, n))
	}
	resources := fmt.Sprintf("<< /Font << %s >> >>", strings.Join(fontRes, " "))

	// The structure tree.
	var elems []*elem
	var number func(x *elem)
	number = func(x *elem) {
		x.obj = e.next()
		elems = append(elems, x)
		for _, k := range x.kids {
			if k.elem != nil {
				number(k.elem)
			}
		}
	}
	number(b.root)
	b.root.parent = nil
	for _, x := range elems {
		parent := structObj
		if x.parent != nil {
			parent = x.parent.obj
		}
		var kids []string
		var pg *page
		for _, k := range x.kids {
			switch {
			case k.elem != nil:
				kids = append(kids, fmt.Sprintf("%d 0 R", k.elem.obj))
			case k.mc != nil:
				kids = append(kids, fmt.Sprintf("<< /Type /MCR /Pg %d 0 R /MCID %d >>", k.mc.page.obj, k.mc.mcid))
				if pg == nil {
					pg = k.mc.page
				}
			case k.annot != nil:
				kids = append(kids, fmt.Sprintf("<< /Type /OBJR /Pg %d 0 R /Obj %d 0 R >>", k.annot.page.obj, k.annot.obj))
			}
		}
		extra := ""
		if pg != nil {
			extra = fmt.Sprintf(" /Pg %d 0 R", pg.obj)
		}
		e.obj(x.obj, "<< /Type /StructElem /S /%s /P %d 0 R%s /K [%s] >>", x.typ, parent, extra, strings.Join(kids, " "))
	}
	var nums []string
	for i, p := range b.pages {
		var refs []string
		for _, mc := range p.mcs {
			refs = append(refs, fmt.Sprintf("%d 0 R", mc.elem.obj))
		}
		nums = append(nums, fmt.Sprintf("%d [%s]", i, strings.Join(refs, " ")))
	}
	for _, a := range annots {
		nums = append(nums, fmt.Sprintf("%d %d 0 R", a.key, a.elem.obj))
	}
	e.obj(parentTreeObj, "<< /Nums [%s] >>", strings.Join(nums, " "))
	e.obj(structObj, "<< /Type /StructTreeRoot /K [%d 0 R] /ParentTree %d 0 R /ParentTreeNextKey %d >>", b.root.obj, parentTreeObj, key)

	// The pages, their contents and their links.
	var kids []string
	for i, p := range b.pages {
		e.stream(p.cobj, "", contents[i])
		var ann []string
		for _, a := range p.annots {
			ann = append(ann, fmt.Sprintf("%d 0 R", a.obj))
		}
		annText := ""
		if len(ann) > 0 {
			annText = fmt.Sprintf(" /Annots [%s]", strings.Join(ann, " "))
		}
		e.obj(p.obj, "<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %s %s] /Resources %s /Contents %d 0 R%s /StructParents %d /Tabs /S >>",
			pagesObj, num(pageW), num(pageH), resources, p.cobj, annText, i)
		kids = append(kids, fmt.Sprintf("%d 0 R", p.obj))
	}
	e.obj(pagesObj, "<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(b.pages))
	for _, a := range annots {
		target := ""
		switch {
		case a.uri != "":
			target = fmt.Sprintf(" /A << /S /URI /URI %s >>", literal(a.uri))
		case b.dests[a.dest] != nil && b.dests[a.dest].page != nil:
			target = " /Dest /" + e.dest(a.dest)
		}
		e.obj(a.obj, "<< /Type /Annot /Subtype /Link /Rect [%s %s %s %s] /Border [0 0 0] /F 4 /StructParent %d /Contents %s%s >>",
			num(a.x), num(pageH-a.y-a.h), num(a.x+a.w), num(pageH-a.y), a.key, textString(a.contents), target)
	}

	// The named destinations, the outline, the catalogue and the document
	// information.
	var dests []string
	for _, n := range names {
		d := b.dests[n]
		dests = append(dests, fmt.Sprintf("/%s [%d 0 R /XYZ %s %s 0]", e.dest(n), d.page.obj, num(left-10), num(pageH-d.y+4)))
	}
	e.outline()
	lang := b.opt.Lang
	if lang == "" {
		lang = "en"
	}
	e.obj(catalogObj, "<< /Type /Catalog /Pages %d 0 R /Outlines %d 0 R /PageMode /UseOutlines /Dests << %s >> /StructTreeRoot %d 0 R /MarkInfo << /Marked true >> /Lang %s /ViewerPreferences << /DisplayDocTitle true >> >>",
		pagesObj, outlinesObj, strings.Join(dests, " "), structObj, literal(lang))
	e.obj(infoObj, "<< /Title %s /Producer (SpecArch) /Creator (specarch document) >>", textString(b.opt.Title))

	return e.file(), nil
}

// file lays the objects out: each stream on its own, every other object
// in object streams of up to 200, then a cross-reference stream.
func (e *emitter) file() []byte {
	type place struct{ stream, index int }
	packed := map[int]place{}
	var loose []int
	for i := 1; i <= e.n; i++ {
		if e.streams[i] {
			loose = append(loose, i)
		}
	}
	var batch []int
	pack := func() {
		if len(batch) == 0 {
			return
		}
		n := e.next()
		var head, body bytes.Buffer
		for k, o := range batch {
			fmt.Fprintf(&head, "%d %d ", o, body.Len())
			body.Write(e.objects[o])
			body.WriteByte('\n')
			packed[o] = place{n, k}
		}
		data := append(head.Bytes(), body.Bytes()...)
		z := deflate(data)
		e.objects[n] = []byte(fmt.Sprintf("<< /Type /ObjStm /N %d /First %d /Length %d /Filter /FlateDecode >>\nstream\n%s\nendstream", len(batch), head.Len(), len(z), z))
		loose = append(loose, n)
		batch = nil
	}
	// The object streams are numbered after the objects they hold.
	last := e.n
	for i := 1; i <= last; i++ {
		if !e.streams[i] {
			batch = append(batch, i)
			if len(batch) == 200 {
				pack()
			}
		}
	}
	pack()
	sort.Ints(loose)
	var out bytes.Buffer
	out.WriteString("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	offsets := map[int]int{}
	for _, i := range loose {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", i)
		out.Write(e.objects[i])
		out.WriteString("\nendobj\n")
	}
	id := sha256.Sum256(out.Bytes())
	xref := e.next()
	offsets[xref] = out.Len()
	var rows bytes.Buffer
	row := func(t byte, a uint32, b uint16) {
		rows.WriteByte(t)
		rows.Write([]byte{byte(a >> 24), byte(a >> 16), byte(a >> 8), byte(a)})
		rows.Write([]byte{byte(b >> 8), byte(b)})
	}
	row(0, 0, 65535)
	for i := 1; i <= e.n; i++ {
		if p, ok := packed[i]; ok {
			row(2, uint32(p.stream), uint16(p.index))
		} else {
			row(1, uint32(offsets[i]), 0)
		}
	}
	z := deflate(rows.Bytes())
	fmt.Fprintf(&out, "%d 0 obj\n<< /Type /XRef /Size %d /W [1 4 2] /Root %d 0 R /Info %d 0 R /ID [<%x> <%x>] /Length %d /Filter /FlateDecode >>\nstream\n", xref, e.n+1, catalogObj, infoObj, id[:16], id[:16], len(z))
	out.Write(z)
	fmt.Fprintf(&out, "\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", offsets[xref])
	return out.Bytes()
}

// content writes the content stream of a page: the header and the footer
// as artifacts, then its operations, each run of an element's content in
// one marked-content sequence.
func (e *emitter) content(p *page) []byte {
	b := e.b
	var s bytes.Buffer
	s.WriteString("/Artifact BMC\n")
	header := b.opt.Header
	if header == "" {
		header = b.opt.Title
	}
	e.text(&s, regular, 7.5, grey, left, headerY, header)
	fmt.Fprintf(&s, "%s RG 0.4 w %s %s m %s %s l S\n", colour(ruleGrey), num(left), num(pageH-headerY-5), num(right), num(pageH-headerY-5))
	footer := fmt.Sprintf(b.opt.Texts.PageOf, p.n, len(b.pages))
	e.text(&s, regular, 7.5, grey, (pageW-b.m[regular].width(footer, 7.5))/2, footerY, footer)
	s.WriteString("EMC\n")
	var open *mcSeq
	artifact := false
	closeAll := func() {
		if open != nil || artifact {
			s.WriteString("EMC\n")
		}
		open, artifact = nil, false
	}
	for _, o := range p.ops {
		if o.mc == nil {
			if !artifact {
				closeAll()
				s.WriteString("/Artifact BMC\n")
				artifact = true
			}
		} else if o.mc != open {
			closeAll()
			fmt.Fprintf(&s, "/%s << /MCID %d >> BDC\n", o.mc.elem.typ, o.mc.mcid)
			open = o.mc
		}
		switch o.kind {
		case textOp:
			text, x := o.text, o.x
			if o.deferred != nil {
				text = o.deferred()
			}
			if o.alignRight {
				x -= b.m[o.face].width(text, o.size)
			}
			e.text(&s, o.face, o.size, o.color, x, o.y, text)
		case rectOp:
			fmt.Fprintf(&s, "%s rg %s %s %s %s re f\n", colour(o.color), num(o.x), num(pageH-o.y-o.h), num(o.w), num(o.h))
		case strokeOp:
			fmt.Fprintf(&s, "%s RG 0.4 w %s %s %s %s re S\n", colour(o.color), num(o.x), num(pageH-o.y-o.h), num(o.w), num(o.h))
		}
	}
	closeAll()
	return s.Bytes()
}

// text writes a text at a baseline, as the glyphs of a face, a character
// the face lacks as its code point.
func (e *emitter) text(s *bytes.Buffer, f face, size float64, c rgb, x, baseline float64, text string) {
	if text == "" {
		return
	}
	m := e.b.m[f]
	var hex strings.Builder
	put := func(r rune) bool {
		g, ok := m.glyph(r)
		if !ok {
			return false
		}
		if _, seen := e.used[f][g]; !seen {
			e.used[f][g] = r
		}
		fmt.Fprintf(&hex, "%04X", g)
		return true
	}
	for _, r := range text {
		if !put(r) {
			for _, c := range missing(r) {
				put(c)
			}
		}
	}
	fmt.Fprintf(s, "BT %s rg /F%d %s Tf %s %s Td <%s> Tj ET\n", colour(c), f, num(size), num(x), num(pageH-baseline), hex.String())
}

// font writes a face as a Type 0 font over a CID font of the TrueType
// subset of the glyphs the book draws, with their widths and a ToUnicode
// map, and returns the Type 0 font's object.
func (e *emitter) font(f face) (int, error) {
	m := e.b.m[f]
	used := e.used[f]
	gids := make([]int, 0, len(used))
	keep := map[uint16]bool{}
	for g := range used {
		gids = append(gids, int(g))
		keep[g] = true
	}
	sort.Ints(gids)
	data, err := subset(fontData[f].ttf, keep)
	if err != nil {
		return 0, fmt.Errorf("the %s font cannot be cut to the glyphs the book uses: %v", fontData[f].name, err)
	}
	h := sha256.New()
	h.Write([]byte(fontData[f].name))
	for _, g := range gids {
		fmt.Fprintf(h, ",%d", g)
	}
	sum := h.Sum(nil)
	tag := make([]byte, 6)
	for i := range tag {
		tag[i] = 'A' + sum[i]%26
	}
	base := string(tag) + "+" + fontData[f].name
	t0, cid, fd, file, tu := e.next(), e.next(), e.next(), e.next(), e.next()
	scale := func(v float64) string { return strconv.Itoa(int(v*1000/m.upem + 0.5*sign(v))) }
	var w strings.Builder
	for i := 0; i < len(gids); {
		j := i
		for j+1 < len(gids) && gids[j+1] == gids[j]+1 {
			j++
		}
		fmt.Fprintf(&w, "%d [", gids[i])
		for k := i; k <= j; k++ {
			if k > i {
				w.WriteByte(' ')
			}
			w.WriteString(scale(float64(m.advance(uint16(gids[k])))))
		}
		w.WriteString("] ")
		i = j + 1
	}
	flags := 32
	angle := "0"
	switch f {
	case mono:
		flags |= 1
	case italic, boldItalic:
		flags |= 64
		angle = "-11"
	}
	stem := "80"
	if f == bold || f == boldItalic {
		stem = "140"
	}
	e.obj(t0, "<< /Type /Font /Subtype /Type0 /BaseFont /%s /Encoding /Identity-H /DescendantFonts [%d 0 R] /ToUnicode %d 0 R >>", base, cid, tu)
	e.obj(cid, "<< /Type /Font /Subtype /CIDFontType2 /BaseFont /%s /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /FontDescriptor %d 0 R /CIDToGIDMap /Identity /DW 1000 /W [%s] >>", base, fd, strings.TrimSpace(w.String()))
	bb := m.bbox
	e.obj(fd, "<< /Type /FontDescriptor /FontName /%s /Flags %d /FontBBox [%s %s %s %s] /ItalicAngle %s /Ascent %s /Descent %s /CapHeight %s /StemV %s /FontFile2 %d 0 R >>",
		base, flags, scale(float64(bb[0])), scale(float64(bb[1])), scale(float64(bb[2])), scale(float64(bb[3])), angle, scale(m.ascent), scale(m.descent), scale(m.ascent*0.75), stem, file)
	e.stream(file, fmt.Sprintf("/Length1 %d", len(data)), data)
	var cm strings.Builder
	cm.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")
	for i := 0; i < len(gids); i += 100 {
		chunk := gids[i:min(i+100, len(gids))]
		fmt.Fprintf(&cm, "%d beginbfchar\n", len(chunk))
		for _, g := range chunk {
			fmt.Fprintf(&cm, "<%04X> <", g)
			for _, u := range utf16.Encode([]rune{used[uint16(g)]}) {
				fmt.Fprintf(&cm, "%04X", u)
			}
			cm.WriteString(">\n")
		}
		cm.WriteString("endbfchar\n")
	}
	cm.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n")
	e.stream(tu, "", []byte(cm.String()))
	return t0, nil
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// outline writes the bookmarks: the headings to the fourth level, nested
// by level, each closed but the first level.
func (e *emitter) outline() {
	type node struct {
		entry    outlineEntry
		obj      int
		parent   *node
		children []*node
	}
	root := &node{obj: outlinesObj}
	stack := []*node{root}
	for _, o := range e.b.outline {
		for len(stack) > 1 && stack[len(stack)-1].entry.level >= o.level {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]
		n := &node{entry: o, parent: parent}
		parent.children = append(parent.children, n)
		stack = append(stack, n)
	}
	var count func(n *node) int
	count = func(n *node) int {
		c := len(n.children)
		for _, k := range n.children {
			c += count(k)
		}
		return c
	}
	var number func(n *node)
	number = func(n *node) {
		for _, k := range n.children {
			k.obj = e.next()
			number(k)
		}
	}
	number(root)
	var write func(n *node)
	write = func(n *node) {
		for i, k := range n.children {
			var parts []string
			parts = append(parts, "/Title "+textString(k.entry.title), fmt.Sprintf("/Parent %d 0 R", n.obj))
			if i > 0 {
				parts = append(parts, fmt.Sprintf("/Prev %d 0 R", n.children[i-1].obj))
			}
			if i+1 < len(n.children) {
				parts = append(parts, fmt.Sprintf("/Next %d 0 R", n.children[i+1].obj))
			}
			if len(k.children) > 0 {
				parts = append(parts, fmt.Sprintf("/First %d 0 R /Last %d 0 R /Count -%d", k.children[0].obj, k.children[len(k.children)-1].obj, count(k)))
			}
			if d := e.b.dests[k.entry.dest]; d != nil && d.page != nil {
				parts = append(parts, "/Dest /"+e.dest(k.entry.dest))
			}
			e.obj(k.obj, "<< %s >>", strings.Join(parts, " "))
			write(k)
		}
	}
	write(root)
	if len(root.children) == 0 {
		e.obj(outlinesObj, "<< /Type /Outlines /Count 0 >>")
		return
	}
	e.obj(outlinesObj, "<< /Type /Outlines /First %d 0 R /Last %d 0 R /Count %d >>", root.children[0].obj, root.children[len(root.children)-1].obj, len(root.children))
}

// num writes a number with at most two decimals.
func num(f float64) string {
	s := strconv.FormatFloat(f, 'f', 2, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}

func colour(c rgb) string { return num(c[0]) + " " + num(c[1]) + " " + num(c[2]) }

// textString is a PDF text string: UTF-16BE with its byte order mark, in
// hexadecimal.
func textString(s string) string {
	var b strings.Builder
	b.WriteString("<FEFF")
	for _, u := range utf16.Encode([]rune(s)) {
		fmt.Fprintf(&b, "%04X", u)
	}
	b.WriteString(">")
	return b.String()
}

// literal is a PDF literal string of ASCII text.
func literal(s string) string {
	r := strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`)
	return "(" + r.Replace(s) + ")"
}

// name is a PDF name, every byte outside the regular characters written
// as #XX.
func name(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '!' || c > '~' || strings.IndexByte("#()<>[]{}/%", c) >= 0 {
			fmt.Fprintf(&b, "#%02X", c)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
