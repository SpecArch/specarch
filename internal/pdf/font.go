package pdf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// face is one of the fonts a book is written in.
type face int

const (
	regular face = iota
	bold
	italic
	boldItalic
	mono
	faces
)

// fontData are the TrueType files of the faces, and their names in the
// PDF, a subset tag being put before each when it is embedded.
var fontData = [faces]struct {
	name string
	ttf  []byte
}{
	regular:    {"GoRegular", goregular.TTF},
	bold:       {"GoBold", gobold.TTF},
	italic:     {"GoItalic", goitalic.TTF},
	boldItalic: {"GoBoldItalic", gobolditalic.TTF},
	mono:       {"GoMono", gomono.TTF},
}

// metrics are what layout needs of a face, read once per program: the
// glyph and advance of each character, in units of the font's em.
type metrics struct {
	f       *sfnt.Font
	upem    float64
	ascent  float64
	descent float64 // negative, below the baseline
	bbox    [4]int
	mu      sync.Mutex
	glyphs  map[rune]uint16
	widths  map[uint16]int
	buf     sfnt.Buffer
}

var (
	loadOnce sync.Once
	loaded   [faces]*metrics
	loadErr  error
)

func fonts() ([faces]*metrics, error) {
	loadOnce.Do(func() {
		for i, d := range fontData {
			f, err := sfnt.Parse(d.ttf)
			if err != nil {
				loadErr = fmt.Errorf("the %s font cannot be read: %v", d.name, err)
				return
			}
			m := &metrics{f: f, upem: float64(f.UnitsPerEm()), glyphs: map[rune]uint16{}, widths: map[uint16]int{}}
			ppem := fixed.Int26_6(f.UnitsPerEm()) << 6
			fm, err := f.Metrics(&m.buf, ppem, font.HintingNone)
			if err != nil {
				loadErr = fmt.Errorf("the metrics of the %s font cannot be read: %v", d.name, err)
				return
			}
			m.ascent, m.descent = float64(fm.Ascent)/64, -float64(fm.Descent)/64
			b, err := f.Bounds(&m.buf, ppem, font.HintingNone)
			if err != nil {
				loadErr = fmt.Errorf("the bounds of the %s font cannot be read: %v", d.name, err)
				return
			}
			// sfnt's bounds grow downward; the PDF's grow upward.
			m.bbox = [4]int{int(b.Min.X) / 64, -int(b.Max.Y) / 64, int(b.Max.X) / 64, -int(b.Min.Y) / 64}
			loaded[i] = m
		}
	})
	return loaded, loadErr
}

// glyph is the glyph of r in the face, and false when the face has none.
func (m *metrics) glyph(r rune) (uint16, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.glyphs[r]; ok {
		return g, g != 0
	}
	g, err := m.f.GlyphIndex(&m.buf, r)
	if err != nil {
		g = 0
	}
	m.glyphs[r] = uint16(g)
	return uint16(g), g != 0
}

// advance is the advance of a glyph in units of the em.
func (m *metrics) advance(g uint16) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w, ok := m.widths[g]; ok {
		return w
	}
	a, err := m.f.GlyphAdvance(&m.buf, sfnt.GlyphIndex(g), fixed.Int26_6(m.f.UnitsPerEm())<<6, font.HintingNone)
	w := 0
	if err == nil {
		w = int(a) / 64
	}
	m.widths[g] = w
	return w
}

// width is the width of a text in the face at a size, in points, each
// character the face lacks measured as the text that stands for it.
func (m *metrics) width(s string, size float64) float64 {
	total := 0
	for _, r := range s {
		g, ok := m.glyph(r)
		if !ok {
			for _, c := range missing(r) {
				g2, _ := m.glyph(c)
				total += m.advance(g2)
			}
			continue
		}
		total += m.advance(g)
	}
	return float64(total) * size / m.upem
}

// missing is the text a character no face holds is written as: its code
// point in brackets, so nothing is dropped.
func missing(r rune) string { return fmt.Sprintf("[U+%04X]", r) }

// subset is a TrueType font holding only the glyphs a book uses, with
// every glyph keeping its index, so the codes written in the content
// streams are the glyph indexes of the original font. Unused glyphs are
// left empty; their advances stay in hmtx.
func subset(ttf []byte, used map[uint16]bool) ([]byte, error) {
	tables, err := readTables(ttf)
	if err != nil {
		return nil, err
	}
	head, maxp, loca, glyf := tables["head"], tables["maxp"], tables["loca"], tables["glyf"]
	if head == nil || maxp == nil || loca == nil || glyf == nil {
		return nil, fmt.Errorf("the font has no head, maxp, loca or glyf table")
	}
	numGlyphs := int(binary.BigEndian.Uint16(maxp[4:]))
	long := binary.BigEndian.Uint16(head[50:]) == 1
	offset := func(i int) int {
		if long {
			return int(binary.BigEndian.Uint32(loca[4*i:]))
		}
		return 2 * int(binary.BigEndian.Uint16(loca[2*i:]))
	}
	keep := map[uint16]bool{}
	var add func(g uint16)
	add = func(g uint16) {
		if keep[g] || int(g) >= numGlyphs {
			return
		}
		keep[g] = true
		data := glyf[offset(int(g)):offset(int(g)+1)]
		if len(data) < 10 || int16(binary.BigEndian.Uint16(data)) >= 0 {
			return
		}
		// A composite glyph: keep each of its components.
		p := 10
		for p+4 <= len(data) {
			flags := binary.BigEndian.Uint16(data[p:])
			add(binary.BigEndian.Uint16(data[p+2:]))
			p += 4
			if flags&0x1 != 0 {
				p += 4
			} else {
				p += 2
			}
			switch {
			case flags&0x8 != 0:
				p += 2
			case flags&0x40 != 0:
				p += 4
			case flags&0x80 != 0:
				p += 8
			}
			if flags&0x20 == 0 {
				break
			}
		}
	}
	gids := make([]int, 0, len(used))
	for g := range used {
		gids = append(gids, int(g))
	}
	sort.Ints(gids)
	add(0)
	for _, g := range gids {
		add(uint16(g))
	}
	var newGlyf bytes.Buffer
	newLoca := make([]byte, 4*(numGlyphs+1))
	for i := 0; i < numGlyphs; i++ {
		binary.BigEndian.PutUint32(newLoca[4*i:], uint32(newGlyf.Len()))
		if keep[uint16(i)] {
			newGlyf.Write(glyf[offset(i):offset(i+1)])
			for newGlyf.Len()%4 != 0 {
				newGlyf.WriteByte(0)
			}
		}
	}
	binary.BigEndian.PutUint32(newLoca[4*numGlyphs:], uint32(newGlyf.Len()))
	newHead := append([]byte(nil), head...)
	binary.BigEndian.PutUint16(newHead[50:], 1)
	binary.BigEndian.PutUint32(newHead[8:], 0)
	out := map[string][]byte{"head": newHead, "loca": newLoca, "glyf": newGlyf.Bytes()}
	for _, t := range []string{"hhea", "maxp", "hmtx", "cvt ", "fpgm", "prep"} {
		if tables[t] != nil {
			out[t] = tables[t]
		}
	}
	font := writeTables(out)
	// The whole font sums to 0xB1B0AFBA once head's adjustment is set.
	sum := 0xB1B0AFBA - checksum(font)
	headAt := tableOffset(font, "head")
	binary.BigEndian.PutUint32(font[headAt+8:], sum)
	return font, nil
}

func readTables(ttf []byte) (map[string][]byte, error) {
	if len(ttf) < 12 {
		return nil, fmt.Errorf("the font is too short")
	}
	n := int(binary.BigEndian.Uint16(ttf[4:]))
	tables := map[string][]byte{}
	for i := 0; i < n; i++ {
		rec := ttf[12+16*i:]
		tag := string(rec[:4])
		off, length := int(binary.BigEndian.Uint32(rec[8:])), int(binary.BigEndian.Uint32(rec[12:]))
		if off+length > len(ttf) {
			return nil, fmt.Errorf("the font's %s table runs past its end", tag)
		}
		tables[tag] = ttf[off : off+length]
	}
	return tables, nil
}

func writeTables(tables map[string][]byte) []byte {
	tags := make([]string, 0, len(tables))
	for t := range tables {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	n := len(tags)
	entry := 1
	for entry*2 <= n {
		entry *= 2
	}
	selector := 0
	for 1<<(selector+1) <= entry {
		selector++
	}
	var b bytes.Buffer
	be := func(v any) { _ = binary.Write(&b, binary.BigEndian, v) }
	be(uint32(0x00010000))
	be(uint16(n))
	be(uint16(entry * 16))
	be(uint16(selector))
	be(uint16(n*16 - entry*16))
	off := 12 + 16*n
	for _, t := range tags {
		data := tables[t]
		b.WriteString(t)
		be(checksum(data))
		be(uint32(off))
		be(uint32(len(data)))
		off += (len(data) + 3) &^ 3
	}
	for _, t := range tags {
		b.Write(tables[t])
		for b.Len()%4 != 0 {
			b.WriteByte(0)
		}
	}
	return b.Bytes()
}

func checksum(data []byte) uint32 {
	var sum uint32
	for i := 0; i < len(data); i += 4 {
		var w [4]byte
		copy(w[:], data[i:])
		sum += binary.BigEndian.Uint32(w[:])
	}
	return sum
}

func tableOffset(font []byte, tag string) int {
	n := int(binary.BigEndian.Uint16(font[4:]))
	for i := 0; i < n; i++ {
		rec := font[12+16*i:]
		if string(rec[:4]) == tag {
			return int(binary.BigEndian.Uint32(rec[8:]))
		}
	}
	return 0
}
