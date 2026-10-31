package extract

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
	"golang.org/x/text/encoding/htmlindex"

	"github.com/SpecArch/specarch/internal/ooxml"
	"github.com/SpecArch/specarch/internal/xlsx"
)

// docInput is a document read into blocks by the reader of its format.
type docInput struct {
	title   string // the title the file gives, or "" for the first top heading
	blocks  []docBlock
	says    []string // lines printed after the commit line
	clauses string   // what a clause is, in the description of the tree
	counted string   // what the counts count
}

// docFormat is one format extract documents reads.
type docFormat struct {
	name  string // as messages name it
	exts  []string
	media []string // the media types an email attachment of the format is sent as
	text  bool     // a text file, which a generated mark can be read from
	read  func(name string, data []byte, depth int) (*docInput, error)
}

func documentFormats() []docFormat {
	return []docFormat{
		{"a Markdown document", []string{".md", ".markdown"}, []string{"text/markdown"}, true, readMarkdown},
		{"a PDF document", []string{".pdf"}, []string{"application/pdf"}, false, readPDF},
		{"a Word document", []string{".docx"}, []string{"application/vnd.openxmlformats-officedocument.wordprocessingml.document"}, false, readDOCX},
		{"a PowerPoint presentation", []string{".pptx"}, []string{"application/vnd.openxmlformats-officedocument.presentationml.presentation"}, false, readPPTX},
		{"an Excel workbook", []string{".xlsx"}, []string{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"}, false, readXLSX},
		{"an email message", []string{".eml"}, []string{"message/rfc822"}, true, readEML},
	}
}

// documentFormat is the format a file's extension names.
func documentFormat(name string) (docFormat, bool) {
	ext := strings.ToLower(filepath.Ext(name))
	for _, f := range documentFormats() {
		for _, e := range f.exts {
			if e == ext {
				return f, true
			}
		}
	}
	return docFormat{}, false
}

// attachmentFormat is the format of an email attachment: the one its file
// name's extension names, or else the one its media type names, since a
// forwarded message and many a mail client's attachment carry no file name
// or one with no extension.
func attachmentFormat(name, media string) (docFormat, bool) {
	if f, ok := documentFormat(name); ok {
		return f, true
	}
	for _, f := range documentFormats() {
		for _, m := range f.media {
			if strings.EqualFold(m, media) {
				return f, true
			}
		}
	}
	return docFormat{}, false
}

// documentFormatList names the formats and their extensions.
func documentFormatList() string {
	return "Markdown (.md, .markdown), PDF (.pdf), Word (.docx), PowerPoint (.pptx), Excel (.xlsx) and email messages (.eml)"
}

const (
	clausesByHeading = "every heading and numbered section is a clause"
	countedMarkdown  = "every ATX heading, every paragraph that starts with a section number such as 3.1, every sentence of a paragraph or list item, and every table, outside fenced code"
)

func readMarkdown(_ string, data []byte, _ int) (*docInput, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	return &docInput{blocks: markdownBlocks(lines), clauses: clausesByHeading, counted: countedMarkdown}, nil
}

// --- Word (.docx, ISO/IEC 29500 WordprocessingML) ---

const (
	wordML              = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	markupCompatibility = "http://schemas.openxmlformats.org/markup-compatibility/2006"
)

type wordStyle struct {
	name, basedOn string
	outline       int // -1: none
	numID, ilvl   string
}

type wordLevel struct {
	start        int
	format, text string
	legal        bool
}

type wordNumbering struct {
	abstract map[string]map[int]wordLevel // abstractNumId -> level
	nums     map[string]string            // numId -> abstractNumId
	starts   map[string]map[int]int       // numId -> level -> startOverride
	counters map[string][]int             // list -> counter per level, 0 for not started
}

func readDOCX(_ string, data []byte, _ int) (*docInput, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("the file is not a zip package (%v)", err)
	}
	files := ooxml.Files(z)
	if _, ok := files["word/document.xml"]; !ok {
		return nil, fmt.Errorf("the package holds no word/document.xml")
	}
	styles, err := readWordStyles(files)
	if err != nil {
		return nil, err
	}
	numbering, err := wordNumberingOf(files)
	if err != nil {
		return nil, err
	}
	in := &docInput{clauses: clausesByHeading,
		counted: "every heading (a paragraph whose style or own setting gives it an outline level), with the number the document's numbering gives it, every paragraph that starts with a section number such as 3.1, every sentence of a paragraph or list item, and every table"}
	core := ooxml.CoreTitle(files)
	f, err := files["word/document.xml"].Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := xml.NewDecoder(f)

	type para struct {
		style, numID, ilvl string
		outline            int
		text               strings.Builder
		drawings, boxes    int // drawings, and text boxes among them
		inserted, note     bool
	}
	var p *para
	var inText bool
	tableDepth := 0
	var table [][]string
	var row []string
	var cell strings.Builder
	tablePlace := ""
	n := 0 // paragraphs, those in tables too
	firstHeading, styleTitle := "", ""
	endParagraph := func() {
		if p == nil {
			return
		}
		cur := p
		p = nil
		place := fmt.Sprintf("paragraph %d", n)
		text := strings.Join(strings.Fields(cur.text.String()), " ")
		if tableDepth > 0 {
			if cell.Len() > 0 && text != "" {
				cell.WriteString(" ")
			}
			cell.WriteString(text)
		}
		if cur.drawings > cur.boxes {
			in.blocks = append(in.blocks, docBlock{kind: blockNotHeld, place: place, text: fmt.Sprintf("a picture at %s: a picture is not read; what it shows becomes a requirement only once a sentence says it", place)})
		}
		if cur.boxes > 0 {
			in.blocks = append(in.blocks, docBlock{kind: blockNotHeld, place: place, text: fmt.Sprintf("a text box at %s: the text inside a text box is not read, and the paragraph's own text around it is; what the box says becomes a requirement only once the body says it", place)})
		}
		if cur.inserted {
			in.blocks = append(in.blocks, docBlock{kind: blockNotHeld, place: place, text: fmt.Sprintf("%s holds tracked changes not yet accepted: the inserted text is read and the deleted text is not; accept or reject them in the document to settle which is meant", place)})
		}
		if cur.note {
			in.blocks = append(in.blocks, docBlock{kind: blockNotHeld, place: place, text: fmt.Sprintf("a footnote or an endnote at %s: notes are not read", place)})
		}
		if tableDepth > 0 || text == "" {
			return
		}
		st := styles.resolve(cur.style)
		outline := cur.outline
		if outline < 0 {
			outline = st.outline
		}
		numID, ilvl := cur.numID, cur.ilvl
		if numID == "" {
			numID, ilvl = st.numID, st.ilvl
		}
		if strings.EqualFold(st.name, "title") {
			if styleTitle == "" {
				styleTitle = text
			}
			return
		}
		number, format := numbering.next(numID, ilvl)
		if outline >= 0 && outline <= 8 {
			level := outline + 1
			if firstHeading == "" && level == 1 {
				firstHeading = strings.TrimSpace(number + " " + text)
			}
			switch {
			case format == "" || format == "bullet" || format == "none":
				in.blocks = append(in.blocks, docBlock{kind: blockHeading, level: level, text: text, place: place})
			case number == "":
				in.blocks = append(in.blocks, docBlock{kind: blockSection, level: level, id: text, title: text, place: place},
					docBlock{kind: blockNotHeld, place: place, text: fmt.Sprintf("the heading %q at %s is numbered in the format %s, which this reader does not compute; it is cited by its title", text, place, format)})
			default:
				id := strings.TrimRight(number, ".)")
				in.blocks = append(in.blocks, docBlock{kind: blockSection, level: level, id: id, title: text, place: place})
			}
			return
		}
		if dottedNumber.MatchString(number) {
			text = strings.TrimRight(number, ".") + " " + text
		}
		in.blocks = append(in.blocks, docBlock{kind: blockParagraph, text: text, place: place})
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("word/document.xml does not parse as XML: %v", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Space != wordML {
				// mc:Fallback repeats mc:Choice for older readers, a text box
				// or a drawing in VML: read only the choice.
				if t.Name.Space == markupCompatibility && t.Name.Local == "Fallback" {
					if err := dec.Skip(); err != nil {
						return nil, fmt.Errorf("word/document.xml does not parse as XML: %v", err)
					}
				}
				continue
			}
			switch t.Name.Local {
			case "tbl":
				if tableDepth == 0 {
					table, tablePlace = nil, fmt.Sprintf("paragraph %d", n+1)
				} else {
					in.blocks = append(in.blocks, docBlock{kind: blockNotHeld, place: fmt.Sprintf("paragraph %d", n+1), text: fmt.Sprintf("a table inside a table at paragraph %d: its cells are read as the text of the outer cell", n+1)})
				}
				tableDepth++
			case "tr":
				if tableDepth == 1 {
					row = nil
				}
			case "tc":
				if tableDepth == 1 {
					cell.Reset()
				}
			case "p":
				n++
				p = &para{outline: -1}
			case "pStyle":
				if p != nil {
					p.style = attr(t, "val")
				}
			case "numId":
				if p != nil {
					p.numID = attr(t, "val")
				}
			case "ilvl":
				if p != nil {
					p.ilvl = attr(t, "val")
				}
			case "outlineLvl":
				if p != nil {
					p.outline, _ = strconv.Atoi(attr(t, "val"))
				}
			case "t":
				inText = true
			case "tab", "br", "cr":
				if p != nil {
					p.text.WriteString(" ")
				}
			case "drawing", "pict", "object":
				if p != nil {
					p.drawings++
				}
			case "txbxContent":
				// A text box's paragraphs sit inside the paragraph that
				// anchors it; reading them would end that paragraph early.
				if p != nil {
					p.boxes++
				}
				if err := dec.Skip(); err != nil {
					return nil, fmt.Errorf("word/document.xml does not parse as XML: %v", err)
				}
			case "ins":
				if p != nil {
					p.inserted = true
				}
			case "del":
				if p != nil {
					p.inserted = true
				}
			case "footnoteReference", "endnoteReference":
				if p != nil {
					p.note = true
				}
			}
		case xml.EndElement:
			if t.Name.Space != wordML {
				continue
			}
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				endParagraph()
			case "tc":
				if tableDepth == 1 {
					row = append(row, strings.TrimSpace(cell.String()))
				}
			case "tr":
				if tableDepth == 1 {
					table = append(table, row)
				}
			case "tbl":
				tableDepth--
				if tableDepth == 0 && len(table) > 0 {
					in.blocks = append(in.blocks, docBlock{kind: blockTable, place: tablePlace, rows: table})
				}
			}
		case xml.CharData:
			if inText && p != nil {
				p.text.Write(t)
			}
		}
	}
	switch {
	case styleTitle != "":
		in.title = styleTitle
	case core != "":
		in.title = core
	default:
		in.title = firstHeading
	}
	return in, nil
}

var dottedNumber = regexp.MustCompile(`^[0-9]+(\.[0-9]+)+\.?$`)

func attr(t xml.StartElement, local string) string {
	for _, a := range t.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

type wordStyles map[string]wordStyle

func readWordStyles(files map[string]*zip.File) (wordStyles, error) {
	out := wordStyles{}
	if _, ok := files["word/styles.xml"]; !ok {
		return out, nil
	}
	var doc struct {
		Styles []struct {
			ID   string `xml:"styleId,attr"`
			Name struct {
				Val string `xml:"val,attr"`
			} `xml:"name"`
			BasedOn struct {
				Val string `xml:"val,attr"`
			} `xml:"basedOn"`
			PPr struct {
				Outline *struct {
					Val string `xml:"val,attr"`
				} `xml:"outlineLvl"`
				NumPr struct {
					NumID struct {
						Val string `xml:"val,attr"`
					} `xml:"numId"`
					Ilvl struct {
						Val string `xml:"val,attr"`
					} `xml:"ilvl"`
				} `xml:"numPr"`
			} `xml:"pPr"`
		} `xml:"style"`
	}
	if err := ooxml.Decode(files, "word/styles.xml", &doc); err != nil {
		return nil, err
	}
	for _, s := range doc.Styles {
		st := wordStyle{name: s.Name.Val, basedOn: s.BasedOn.Val, outline: -1, numID: s.PPr.NumPr.NumID.Val, ilvl: s.PPr.NumPr.Ilvl.Val}
		if s.PPr.Outline != nil {
			st.outline, _ = strconv.Atoi(s.PPr.Outline.Val)
		} else if m := headingStyleName.FindStringSubmatch(s.Name.Val); m != nil {
			l, _ := strconv.Atoi(m[1])
			st.outline = l - 1
		}
		out[s.ID] = st
	}
	return out, nil
}

var headingStyleName = regexp.MustCompile(`(?i)^heading ([1-9])$`)

// resolve is a style with what it leaves out taken from the styles it is
// based on.
func (ws wordStyles) resolve(id string) wordStyle {
	out := wordStyle{outline: -1}
	seen := map[string]bool{}
	first := true
	for id != "" && !seen[id] {
		seen[id] = true
		s, ok := ws[id]
		if !ok {
			break
		}
		if first {
			out.name = s.name
			first = false
		}
		if out.outline < 0 {
			out.outline = s.outline
		}
		if out.numID == "" {
			out.numID, out.ilvl = s.numID, s.ilvl
		}
		id = s.basedOn
	}
	return out
}

func wordNumberingOf(files map[string]*zip.File) (*wordNumbering, error) {
	out := &wordNumbering{abstract: map[string]map[int]wordLevel{}, nums: map[string]string{}, starts: map[string]map[int]int{}, counters: map[string][]int{}}
	if _, ok := files["word/numbering.xml"]; !ok {
		return out, nil
	}
	type lvl struct {
		Ilvl  int `xml:"ilvl,attr"`
		Start struct {
			Val string `xml:"val,attr"`
		} `xml:"start"`
		Fmt struct {
			Val string `xml:"val,attr"`
		} `xml:"numFmt"`
		Text struct {
			Val string `xml:"val,attr"`
		} `xml:"lvlText"`
		Legal *struct{} `xml:"isLgl"`
	}
	var doc struct {
		Abstract []struct {
			ID     string `xml:"abstractNumId,attr"`
			Levels []lvl  `xml:"lvl"`
		} `xml:"abstractNum"`
		Nums []struct {
			ID       string `xml:"numId,attr"`
			Abstract struct {
				Val string `xml:"val,attr"`
			} `xml:"abstractNumId"`
			Override []struct {
				Ilvl  int `xml:"ilvl,attr"`
				Start *struct {
					Val string `xml:"val,attr"`
				} `xml:"startOverride"`
			} `xml:"lvlOverride"`
		} `xml:"num"`
	}
	if err := ooxml.Decode(files, "word/numbering.xml", &doc); err != nil {
		return nil, err
	}
	for _, a := range doc.Abstract {
		levels := map[int]wordLevel{}
		for _, l := range a.Levels {
			start := 1
			if l.Start.Val != "" {
				start, _ = strconv.Atoi(l.Start.Val)
			}
			f := l.Fmt.Val
			if f == "" {
				f = "decimal"
			}
			levels[l.Ilvl] = wordLevel{start: start, format: f, text: l.Text.Val, legal: l.Legal != nil}
		}
		out.abstract[a.ID] = levels
	}
	for _, n := range doc.Nums {
		out.nums[n.ID] = n.Abstract.Val
		for _, o := range n.Override {
			if o.Start != nil {
				if out.starts[n.ID] == nil {
					out.starts[n.ID] = map[int]int{}
				}
				out.starts[n.ID][o.Ilvl], _ = strconv.Atoi(o.Start.Val)
			}
		}
	}
	return out, nil
}

// next counts one paragraph of a list and gives its number as the level's
// text makes it, with the level's format; "" with the format when the
// format is not one this reader computes. Lists of one abstract numbering
// count on together unless one restarts with a start override, as word
// processors count them.
func (wn *wordNumbering) next(numID, ilvlText string) (string, string) {
	if numID == "" || numID == "0" {
		return "", ""
	}
	abs, ok := wn.nums[numID]
	if !ok {
		return "", ""
	}
	levels := wn.abstract[abs]
	ilvl, _ := strconv.Atoi(ilvlText)
	l, ok := levels[ilvl]
	if !ok {
		return "", ""
	}
	key := "abstract " + abs
	if wn.starts[numID] != nil {
		key = "num " + numID
	}
	start := func(i int) int {
		if s, ok := wn.starts[numID][i]; ok {
			return s
		}
		return levels[i].start
	}
	c := wn.counters[key]
	if c == nil {
		c = make([]int, 9)
		wn.counters[key] = c
	}
	if ilvl < 0 || ilvl > 8 {
		return "", l.format
	}
	if c[ilvl] == 0 {
		c[ilvl] = start(ilvl)
	} else {
		c[ilvl]++
	}
	for i := ilvl + 1; i < 9; i++ {
		c[i] = 0
	}
	if l.format == "bullet" || l.format == "none" {
		return "", l.format
	}
	text := l.text
	for i := 0; i <= ilvl; i++ {
		v := c[i]
		if v == 0 {
			v = start(i)
		}
		f := levels[i].format
		if l.legal {
			f = "decimal"
		}
		s, ok := numberIn(v, f)
		if !ok && strings.Contains(text, "%"+strconv.Itoa(i+1)) {
			return "", f
		}
		text = strings.ReplaceAll(text, "%"+strconv.Itoa(i+1), s)
	}
	return strings.TrimSpace(text), l.format
}

// numberIn writes n in a numbering format.
func numberIn(n int, format string) (string, bool) {
	switch format {
	case "decimal":
		return strconv.Itoa(n), true
	case "decimalZero":
		return fmt.Sprintf("%02d", n), true
	case "lowerLetter", "upperLetter":
		s := xlsx.ColumnName(n)
		if format == "lowerLetter" {
			s = strings.ToLower(s)
		}
		return s, true
	case "lowerRoman", "upperRoman":
		s := roman(n)
		if format == "lowerRoman" {
			s = strings.ToLower(s)
		}
		return s, s != ""
	}
	return "", false
}

func roman(n int) string {
	if n <= 0 || n >= 4000 {
		return ""
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var b strings.Builder
	for i, v := range vals {
		for n >= v {
			b.WriteString(syms[i])
			n -= v
		}
	}
	return b.String()
}

// --- PowerPoint (.pptx, ISO/IEC 29500 PresentationML) ---

const (
	presentationML = "http://schemas.openxmlformats.org/presentationml/2006/main"
	drawingML      = "http://schemas.openxmlformats.org/drawingml/2006/main"
)

func readPPTX(_ string, data []byte, _ int) (*docInput, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("the file is not a zip package (%v)", err)
	}
	files := ooxml.Files(z)
	var pres struct {
		Slides []struct {
			ID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sldIdLst>sldId"`
	}
	if err := ooxml.Decode(files, "ppt/presentation.xml", &pres); err != nil {
		return nil, err
	}
	rels, err := ooxml.Relationships(files, "ppt/presentation.xml")
	if err != nil {
		return nil, err
	}
	in := &docInput{title: ooxml.CoreTitle(files),
		clauses: "every slide is a clause, as slide 3, and so are its speaker notes, as slide 3 notes",
		counted: "every slide and its speaker notes as a heading, every sentence of a title, a text box or the notes, and every table"}
	for i, s := range pres.Slides {
		n := i + 1
		rel, ok := rels[s.ID]
		if !ok {
			return nil, fmt.Errorf("slide %d names relationship %s, which ppt/_rels/presentation.xml.rels does not hold", n, s.ID)
		}
		slide, err := readSlide(files, rel.Target, false)
		if err != nil {
			return nil, fmt.Errorf("slide %d (%s): %v", n, rel.Target, err)
		}
		place := fmt.Sprintf("slide %d", n)
		title := slide.title
		if title == "" {
			title = place
		}
		if in.title == "" && slide.title != "" {
			in.title = slide.title
		}
		in.blocks = append(in.blocks, docBlock{kind: blockSection, level: 1, id: place, title: title, place: place})
		in.blocks = append(in.blocks, slide.blocks(place)...)
		srels, err := ooxml.Relationships(files, rel.Target)
		if err != nil {
			return nil, err
		}
		for _, r := range sortedRels(srels) {
			if !strings.HasSuffix(r.Type, "/notesSlide") {
				continue
			}
			notes, err := readSlide(files, r.Target, true)
			if err != nil {
				return nil, fmt.Errorf("the notes of slide %d (%s): %v", n, r.Target, err)
			}
			if len(notes.items) == 0 {
				continue
			}
			np := place + " notes"
			in.blocks = append(in.blocks, docBlock{kind: blockSection, level: 2, id: np, title: title, place: np})
			in.blocks = append(in.blocks, notes.blocks(np)...)
		}
	}
	return in, nil
}

func sortedRels(m map[string]ooxml.Relationship) []ooxml.Relationship {
	var ids []string
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var out []ooxml.Relationship
	for _, id := range ids {
		out = append(out, m[id])
	}
	return out
}

type slideItem struct {
	text    string
	table   [][]string
	notHeld string
}

type slideText struct {
	title string
	items []slideItem
}

func (s slideText) blocks(place string) []docBlock {
	var out []docBlock
	for _, it := range s.items {
		switch {
		case it.notHeld != "":
			out = append(out, docBlock{kind: blockNotHeld, place: place, text: fmt.Sprintf("%s on %s: it is not read", it.notHeld, place)})
		case it.table != nil:
			out = append(out, docBlock{kind: blockTable, place: place, rows: it.table})
		default:
			out = append(out, docBlock{kind: blockParagraph, text: it.text, place: place, plain: true})
		}
	}
	return out
}

// readSlide reads a slide's shapes in order: the title placeholder's text
// as the title, every other paragraph of text and every table; of notes,
// only the body placeholder, not the slide image, the number or a footer.
func readSlide(files map[string]*zip.File, name string, notes bool) (slideText, error) {
	var out slideText
	f, ok := files[name]
	if !ok {
		return out, fmt.Errorf("the package holds no %s", name)
	}
	rc, err := f.Open()
	if err != nil {
		return out, err
	}
	defer rc.Close()
	dec := xml.NewDecoder(rc)
	var (
		inShape, inText        bool
		phType                 string
		hasPh                  bool
		shapeParas             []string
		para                   strings.Builder
		inTable                bool
		table                  [][]string
		row                    []string
		cell                   strings.Builder
		frameHasTable, inFrame bool
	)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, fmt.Errorf("does not parse as XML: %v", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Space == markupCompatibility && t.Name.Local == "Fallback":
				// mc:Fallback repeats mc:Choice for older readers (math,
				// SmartArt, media): read only the choice, as in Word.
				if err := dec.Skip(); err != nil {
					return out, fmt.Errorf("does not parse as XML: %v", err)
				}
			case t.Name.Space == presentationML && t.Name.Local == "sp":
				inShape, phType, hasPh, shapeParas = true, "", false, nil
			case t.Name.Space == presentationML && t.Name.Local == "ph":
				hasPh, phType = true, attr(t, "type")
			case t.Name.Space == presentationML && t.Name.Local == "pic" && !notes:
				out.items = append(out.items, slideItem{notHeld: "a picture"})
			case t.Name.Space == presentationML && t.Name.Local == "graphicFrame":
				inFrame, frameHasTable = true, false
			case t.Name.Space == drawingML && t.Name.Local == "tbl":
				inTable, table, frameHasTable = true, nil, true
			case t.Name.Space == drawingML && t.Name.Local == "tr" && inTable:
				row = nil
			case t.Name.Space == drawingML && t.Name.Local == "tc" && inTable:
				cell.Reset()
			case t.Name.Space == drawingML && t.Name.Local == "p":
				para.Reset()
			case t.Name.Space == drawingML && t.Name.Local == "t":
				inText = true
			case t.Name.Space == drawingML && t.Name.Local == "br":
				para.WriteString(" ")
			}
		case xml.EndElement:
			switch {
			case t.Name.Space == drawingML && t.Name.Local == "t":
				inText = false
			case t.Name.Space == drawingML && t.Name.Local == "p":
				text := strings.Join(strings.Fields(para.String()), " ")
				if inTable {
					if text != "" {
						if cell.Len() > 0 {
							cell.WriteString(" ")
						}
						cell.WriteString(text)
					}
				} else if inShape && text != "" {
					shapeParas = append(shapeParas, text)
				}
			case t.Name.Space == drawingML && t.Name.Local == "tc" && inTable:
				row = append(row, strings.TrimSpace(cell.String()))
			case t.Name.Space == drawingML && t.Name.Local == "tr" && inTable:
				table = append(table, row)
			case t.Name.Space == drawingML && t.Name.Local == "tbl":
				inTable = false
				if len(table) > 0 && !notes {
					out.items = append(out.items, slideItem{table: table})
				}
			case t.Name.Space == presentationML && t.Name.Local == "graphicFrame":
				if inFrame && !frameHasTable && !notes {
					out.items = append(out.items, slideItem{notHeld: "a chart, a diagram or an embedded object"})
				}
				inFrame = false
			case t.Name.Space == presentationML && t.Name.Local == "sp":
				inShape = false
				switch {
				case notes && (!hasPh || phType != "body"):
				case !notes && (phType == "title" || phType == "ctrTitle"):
					if out.title == "" {
						out.title = strings.Join(shapeParas, " ")
					}
				case !notes && (phType == "sldNum" || phType == "dt" || phType == "ftr" || phType == "hdr"):
				default:
					for _, p := range shapeParas {
						out.items = append(out.items, slideItem{text: p})
					}
				}
			}
		case xml.CharData:
			if inText {
				para.Write(t)
			}
		}
	}
	return out, nil
}

// --- Excel (.xlsx) ---

func readXLSX(_ string, data []byte, _ int) (*docInput, error) {
	wb, err := xlsx.Read(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	in := &docInput{title: wb.Title,
		clauses: "every sheet and every table of cells is a clause, as Sheet1!A1:C4, and so is every cell of text a citation names, as Sheet1!B4",
		counted: "every sheet as a heading, every region of touching cells of two rows and two columns or more as a table, its first row the header, and every sentence of a cell of text"}
	for _, name := range wb.NotRead {
		in.blocks = append(in.blocks, docBlock{kind: blockNotHeld, place: "sheet " + name, text: fmt.Sprintf("the sheet %s is a chart or a dialog sheet, which holds no cells; it is not read", name)})
	}
	for _, s := range wb.Sheets {
		in.blocks = append(in.blocks, XLSXSheetBlocks(s)...)
	}
	return in, nil
}

// XLSXSheetBlocks is one sheet as document blocks: the sheet as a section,
// each region of two rows and two columns or more as a table under a
// section named by its range, and each cell of text as a paragraph under a
// section named by its reference, listed only when cited.
func XLSXSheetBlocks(s xlsx.Sheet) []docBlock {
	ref := xlsx.SheetRef(s.Name)
	out := []docBlock{{kind: blockSection, level: 1, id: s.Name, title: s.Name, place: "sheet " + s.Name}}
	if s.Hidden {
		out = append(out, docBlock{kind: blockNotHeld, place: "sheet " + s.Name, text: fmt.Sprintf("the sheet %s is hidden in the workbook; it is read as the others are", s.Name)})
	}
	text := map[string]bool{}
	for _, c := range s.Cells {
		if c.Kind == xlsx.Text && strings.TrimSpace(c.Value) != "" {
			text[c.Ref] = true
		}
	}
	for _, r := range s.Regions() {
		level := 2
		firstData := r.FirstRow
		header := map[int]string{}
		if len(r.Rows) >= 2 && r.LastColumn > r.FirstColumn {
			out = append(out,
				docBlock{kind: blockSection, level: 2, id: r.Range(), title: s.Name, place: r.Range()},
				docBlock{kind: blockTable, place: r.Range(), rows: r.Rows})
			level = 3
			firstData = r.FirstRow + 1
			for i, h := range r.Rows[0] {
				header[r.FirstColumn+i] = h
			}
		}
		for row := firstData; row <= r.LastRow; row++ {
			for col := r.FirstColumn; col <= r.LastColumn; col++ {
				cell := xlsx.Ref(row, col)
				if !text[cell] {
					continue
				}
				v := r.Rows[row-r.FirstRow][col-r.FirstColumn]
				title := s.Name
				if h := header[col]; h != "" {
					title = h
				}
				out = append(out,
					docBlock{kind: blockSection, level: level, id: ref + "!" + cell, title: title, place: ref + "!" + cell, cited: true},
					docBlock{kind: blockParagraph, text: strings.Join(strings.Fields(v), " "), place: ref + "!" + cell, plain: true})
			}
		}
	}
	return out
}

// --- PDF ---

type pdfLine struct {
	page    int
	text    string
	size, y float64
	bold    bool
}

func readPDF(_ string, data []byte, _ int) (in *docInput, err error) {
	defer func() {
		if r := recover(); r != nil {
			in, err = nil, fmt.Errorf("the PDF does not parse: %v", r)
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		if err == pdf.ErrInvalidPassword {
			return nil, fmt.Errorf("the PDF is encrypted with a password; save a copy without one and read that")
		}
		if strings.Contains(err.Error(), "encrypt") {
			return nil, fmt.Errorf("the PDF is encrypted in a way this build does not open (%v); save a copy without encryption and read that", err)
		}
		return nil, fmt.Errorf("the PDF does not parse: %v", err)
	}
	in = &docInput{
		clauses: "every heading and numbered section is a clause, and so is every page before the first heading, as page 2",
		counted: "every line set larger than the body text, or in bold with a section number, as a heading, every page before the first heading, every paragraph that starts with a section number such as 3.1, and every sentence of a paragraph; a table is read as its lines, since a PDF does not mark one"}
	if info := r.Trailer().Key("Info"); !info.IsNull() {
		in.title = strings.TrimSpace(info.Key("Title").Text())
	}
	var lines []pdfLine
	empty := map[int]bool{}
	for n := 1; n <= r.NumPage(); n++ {
		p := r.Page(n)
		if p.V.IsNull() {
			continue
		}
		got := pdfPageLines(n, p.Content().Text)
		if len(got) == 0 {
			empty[n] = true
		}
		lines = append(lines, got...)
	}
	// A line that repeats on three pages or more, its digits aside, is a
	// running header or footer.
	repeated := map[string]map[int]bool{}
	digits := regexp.MustCompile(`[0-9]+`)
	for _, l := range lines {
		k := digits.ReplaceAllString(l.text, "#")
		if repeated[k] == nil {
			repeated[k] = map[int]bool{}
		}
		repeated[k][l.page] = true
	}
	running := map[string]bool{}
	if r.NumPage() >= 3 {
		for k, pages := range repeated {
			if len(pages) >= 3 {
				running[k] = true
			}
		}
	}
	var body []pdfLine
	said := map[string]bool{}
	for _, l := range lines {
		k := digits.ReplaceAllString(l.text, "#")
		if running[k] {
			if !said[k] {
				said[k] = true
				in.blocks = append(in.blocks, docBlock{kind: blockNotHeld, place: fmt.Sprintf("page %d", l.page), text: fmt.Sprintf("the line %q repeats on %d pages, a running header or footer; it is not read", l.text, len(repeated[k]))})
			}
			continue
		}
		body = append(body, l)
	}
	// The body size is the size most characters are set in.
	weight := map[float64]int{}
	for _, l := range body {
		weight[l.size] += len([]rune(l.text))
	}
	bodySize, most := 0.0, -1
	for s, w := range weight {
		if w > most || (w == most && s < bodySize) {
			bodySize, most = s, w
		}
	}
	isHeading := func(l pdfLine) bool {
		short := len([]rune(l.text)) <= 120
		if l.size >= bodySize*1.15 && short {
			return true
		}
		return l.bold && short && headingNumber.MatchString(l.text) && !strings.HasSuffix(l.text, ".")
	}
	var sizes []float64
	seen := map[float64]bool{}
	for _, l := range body {
		if isHeading(l) && !seen[l.size] {
			seen[l.size] = true
			sizes = append(sizes, l.size)
		}
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(sizes)))
	levelOf := func(l pdfLine) int {
		for i, s := range sizes {
			if l.size == s {
				return i + 1
			}
		}
		return 1
	}
	headingSeen := false
	page := 0
	var para []string
	var last pdfLine
	flush := func() {
		if len(para) > 0 {
			in.blocks = append(in.blocks, docBlock{kind: blockParagraph, text: strings.Join(para, " "), place: fmt.Sprintf("page %d", last.page)})
			para = nil
		}
	}
	for n := 1; n <= r.NumPage(); n++ {
		if empty[n] {
			in.blocks = append(in.blocks, docBlock{kind: blockNotHeld, place: fmt.Sprintf("page %d", n), text: fmt.Sprintf("page %d holds no text: a scanned page is a picture of text, which is not read; read the page's text from a copy that holds it", n)})
		}
	}
	for _, l := range body {
		if l.page != page {
			flush()
			page = l.page
			if !headingSeen {
				id := fmt.Sprintf("page %d", page)
				in.blocks = append(in.blocks, docBlock{kind: blockSection, level: 1, id: id, title: id, place: id})
			}
		}
		if isHeading(l) {
			flush()
			headingSeen = true
			in.blocks = append(in.blocks, docBlock{kind: blockHeading, level: levelOf(l), text: l.text, place: fmt.Sprintf("page %d", l.page)})
			last = l
			continue
		}
		breaks := len(para) == 0 || last.y-l.y > 1.6*math.Max(l.size, 1) || paragraphNumber.MatchString(l.text) || listItem.MatchString(l.text) || strings.HasPrefix(l.text, "•")
		if breaks {
			flush()
		}
		text := strings.TrimSpace(strings.TrimPrefix(l.text, "•"))
		if m := listItem.FindStringSubmatch(text); m != nil && !paragraphNumber.MatchString(text) {
			text = m[1]
		}
		if n := len(para); n > 0 && strings.HasSuffix(para[n-1], "-") && text != "" && strings.ToLower(text[:1]) == text[:1] {
			para[n-1] = strings.TrimSuffix(para[n-1], "-") + text
		} else {
			para = append(para, text)
		}
		last = l
	}
	flush()
	return in, nil
}

// ligatures are the typographic ligatures a PDF sets one glyph for, read
// as their letters so that a word is found as it is spelled.
var ligatures = strings.NewReplacer("\ufb00", "ff", "\ufb01", "fi", "\ufb02", "fl", "\ufb03", "ffi", "\ufb04", "ffl", "\ufb05", "st", "\ufb06", "st")

// pdfPageLines groups a page's glyphs into lines: by height, top to
// bottom, and in a line left to right, with a space where a gap is wider
// than a fifth of the size.
func pdfPageLines(page int, texts []pdf.Text) []pdfLine {
	ts := append([]pdf.Text(nil), texts...)
	sort.SliceStable(ts, func(i, j int) bool {
		if math.Abs(ts[i].Y-ts[j].Y) > 0.5 {
			return ts[i].Y > ts[j].Y
		}
		return ts[i].X < ts[j].X
	})
	var out []pdfLine
	var cur []pdf.Text
	end := func() {
		if len(cur) == 0 {
			return
		}
		sort.SliceStable(cur, func(i, j int) bool { return cur[i].X < cur[j].X })
		var b strings.Builder
		size, bold := 0.0, false
		chars := 0
		for i, t := range cur {
			if i > 0 {
				prev := cur[i-1]
				// A glyph's width, when the file gives none, is taken as
				// up to one em: a wider step to the next glyph is a space.
				wide := t.X-(prev.X+prev.W) > 0.2*math.Max(t.FontSize, 1)
				if prev.W == 0 {
					wide = t.X-prev.X > float64(len([]rune(prev.S)))*math.Max(prev.FontSize, 1)
				}
				if wide && !strings.HasSuffix(b.String(), " ") && !strings.HasPrefix(t.S, " ") {
					b.WriteString(" ")
				}
			}
			b.WriteString(t.S)
			if strings.TrimSpace(t.S) != "" {
				size = math.Max(size, math.Round(t.FontSize*2)/2)
				if strings.Contains(strings.ToLower(t.Font), "bold") {
					chars++
				} else {
					chars--
				}
			}
		}
		bold = chars > 0
		text := strings.Join(strings.Fields(ligatures.Replace(b.String())), " ")
		if text != "" {
			out = append(out, pdfLine{page: page, text: text, size: size, y: cur[0].Y, bold: bold})
		}
		cur = nil
	}
	for _, t := range ts {
		if len(cur) > 0 && math.Abs(cur[0].Y-t.Y) > 0.5*math.Max(cur[0].FontSize, 1) {
			end()
		}
		cur = append(cur, t)
	}
	end()
	return out
}

// --- Email messages (.eml, RFC 5322 with MIME, RFC 2045 to 2049) ---

const maxAttachmentDepth = 3

type emailAttachment struct {
	name, mediaType string
	data            []byte
	place           string // the name, numbered from 2 when another attachment has it
}

func readEML(_ string, data []byte, depth int) (*docInput, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("the message does not parse: %v", err)
	}
	dec := &mime.WordDecoder{CharsetReader: charsetReader}
	header := func(k string) string {
		v := msg.Header.Get(k)
		if d, err := dec.DecodeHeader(v); err == nil {
			v = d
		}
		return strings.Join(strings.Fields(v), " ")
	}
	subject, from, to := header("Subject"), header("From"), header("To")
	sent := header("Date")
	if t, err := mail.ParseDate(sent); err == nil {
		sent = t.Format(time.RFC3339)
	} else if sent == "" {
		sent = "with no date"
	}
	var text, htmlText []string
	var attachments []emailAttachment
	var notRead []string
	var walk func(h map[string][]string, body io.Reader) error
	walk = func(h map[string][]string, body io.Reader) error {
		get := func(k string) string {
			if v := h[k]; len(v) > 0 {
				return v[0]
			}
			return ""
		}
		ct := get("Content-Type")
		if ct == "" {
			ct = "text/plain"
		}
		media, params, err := mime.ParseMediaType(ct)
		if err != nil {
			media, params = "application/octet-stream", map[string]string{}
		}
		switch strings.ToLower(get("Content-Transfer-Encoding")) {
		case "base64":
			body = base64.NewDecoder(base64.StdEncoding, body)
		case "quoted-printable":
			body = quotedprintable.NewReader(body)
		}
		disposition, dparams, _ := mime.ParseMediaType(get("Content-Disposition"))
		name := dparams["filename"]
		if name == "" {
			name = params["name"]
		}
		if d, err := dec.DecodeHeader(name); err == nil {
			name = d
		}
		if strings.HasPrefix(media, "multipart/") {
			mr := multipart.NewReader(body, params["boundary"])
			for {
				part, err := mr.NextRawPart()
				if err == io.EOF {
					return nil
				}
				if err != nil {
					return fmt.Errorf("a %s part does not parse: %v", media, err)
				}
				if err := walk(part.Header, part); err != nil {
					return err
				}
			}
		}
		content, err := io.ReadAll(body)
		if err != nil {
			return fmt.Errorf("a %s part does not decode: %v", media, err)
		}
		forwarded := strings.EqualFold(media, "message/rfc822")
		if name != "" || disposition == "attachment" || forwarded {
			if name == "" && forwarded {
				// A forwarded message is sent with no file name: it is
				// named by its subject, quoted, since a subject may hold
				// the ": " that ends a name in a clause.
				if m, err := mail.ReadMessage(bytes.NewReader(content)); err == nil {
					if s, err := dec.DecodeHeader(m.Header.Get("Subject")); err == nil && strings.TrimSpace(s) != "" {
						name = "message " + strconv.Quote(strings.Join(strings.Fields(s), " "))
					}
				}
				if name == "" {
					name = "message " + strconv.Itoa(len(attachments)+1)
				}
			}
			if name == "" {
				name = "attachment " + strconv.Itoa(len(attachments)+1)
			}
			// Two attachments of one name (a subject repeated in a thread,
			// a file sent twice) would be one clause: the second is
			// numbered, as "Loan limits" 2.
			place := name
			for k := 2; ; k++ {
				taken := false
				for _, a := range attachments {
					if a.place == place {
						taken = true
						break
					}
				}
				if !taken {
					break
				}
				place = name + " " + strconv.Itoa(k)
			}
			attachments = append(attachments, emailAttachment{name: name, mediaType: media, data: content, place: place})
			return nil
		}
		s, err := decodeCharset(content, params["charset"])
		if err != nil {
			return err
		}
		switch media {
		case "text/plain":
			text = append(text, s)
		case "text/html":
			htmlText = append(htmlText, htmlToText(s))
		default:
			notRead = append(notRead, media)
		}
		return nil
	}
	if err := walk(msg.Header, msg.Body); err != nil {
		return nil, err
	}
	bodyText := strings.Join(text, "\n\n")
	if strings.TrimSpace(bodyText) == "" {
		bodyText = strings.Join(htmlText, "\n\n")
	}
	title := subject
	if title == "" {
		title = "the message"
	}
	in := &docInput{title: subject,
		clauses: fmt.Sprintf("the message %q from %s to %s, sent %s: its body is a clause, as body, and so is every attachment, as its file name, whose own clauses follow its name, as rules.docx: 2.1", subject, orNone(from), orNone(to), sent),
		counted: "the body and every attachment as a heading, and every heading, paragraph, sentence and table of an attachment in a format this build reads, as that format's reader counts them",
		says:    []string{fmt.Sprintf("message: from %s to %s, sent %s, subject %q, with %s", orNone(from), orNone(to), sent, subject, plural(len(attachments), "attachment"))}}
	in.blocks = append(in.blocks, docBlock{kind: blockSection, level: 1, id: "body", title: title, place: "body"})
	quoted, signature := false, false
	var para []string
	flush := func() {
		if len(para) > 0 {
			in.blocks = append(in.blocks, docBlock{kind: blockParagraph, text: strings.Join(para, " "), place: "body"})
			para = nil
		}
	}
	for _, line := range strings.Split(strings.ReplaceAll(bodyText, "\r\n", "\n"), "\n") {
		if line == "-- " {
			signature = true
			break
		}
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, ">"):
			flush()
			quoted = true
		case t == "":
			flush()
		default:
			para = append(para, t)
		}
	}
	flush()
	if quoted {
		in.blocks = append(in.blocks, docBlock{kind: blockNotHeld, place: "body", text: "the lines of the body that start with >, quoted from an earlier message: they are not read; read that message itself"})
	}
	if signature {
		in.blocks = append(in.blocks, docBlock{kind: blockNotHeld, place: "body", text: "the signature after the line \"-- \": it is not read"})
	}
	for _, media := range notRead {
		in.blocks = append(in.blocks, docBlock{kind: blockNotHeld, place: "body", text: fmt.Sprintf("a part of the body of type %s: only text/plain and text/html are read", media)})
	}
	for _, a := range attachments {
		place := a.place
		in.blocks = append(in.blocks, docBlock{kind: blockSection, level: 1, id: place, title: place, place: place})
		f, ok := attachmentFormat(a.name, a.mediaType)
		if !ok || depth+1 > maxAttachmentDepth {
			why := fmt.Sprintf("this build reads attachments in %s", documentFormatList())
			if ok {
				why = fmt.Sprintf("it is nested %d messages deep, and attachments are read %d deep", depth+1, maxAttachmentDepth)
			}
			in.blocks = append(in.blocks, docBlock{kind: blockNotHeld, place: place, text: fmt.Sprintf("the attachment %s, of type %s: %s; it is not read", place, a.mediaType, why)})
			continue
		}
		inner, err := f.read(a.name, a.data, depth+1)
		if err != nil {
			in.blocks = append(in.blocks, docBlock{kind: blockNotHeld, place: place, text: fmt.Sprintf("the attachment %s cannot be read as %s: %v", place, f.name, err)})
			continue
		}
		for _, s := range inner.says {
			in.says = append(in.says, "attachment "+place+": "+s)
		}
		for _, b := range inner.blocks {
			b.level++
			b.prefix = place + ": " + b.prefix
			if b.place == "" {
				b.place = place
			} else {
				b.place = place + ", " + b.place
			}
			if b.kind == blockNotHeld {
				b.text = place + ": " + b.text
			}
			in.blocks = append(in.blocks, b)
		}
	}
	return in, nil
}

func orNone(s string) string {
	if s == "" {
		return "no one named"
	}
	return s
}

func charsetReader(charset string, r io.Reader) (io.Reader, error) {
	e, err := htmlindex.Get(charset)
	if err != nil {
		return nil, fmt.Errorf("the character set %s is not one this build decodes: %v", charset, err)
	}
	return e.NewDecoder().Reader(r), nil
}

func decodeCharset(b []byte, charset string) (string, error) {
	cs := strings.ToLower(charset)
	if cs == "" || cs == "utf-8" || cs == "us-ascii" || cs == "utf8" {
		return string(b), nil
	}
	r, err := charsetReader(charset, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("the text in %s does not decode: %v", charset, err)
	}
	return string(out), nil
}

var (
	htmlDrop  = regexp.MustCompile(`(?is)<(script|style|head)\b.*?</(script|style|head)\s*>`)
	htmlBreak = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlBlock = regexp.MustCompile(`(?i)</?(p|div|li|ul|ol|h[1-6]|tr|table|blockquote)\b[^>]*>`)
	htmlTag   = regexp.MustCompile(`<[^>]*>`)
	htmlQuote = regexp.MustCompile(`(?is)<blockquote\b.*?</blockquote\s*>`)
)

// htmlToText is an HTML body's text: a paragraph, a heading, a list item
// or a row is a paragraph, a line break a new line, and a quotation of an
// earlier message is marked as quoted.
func htmlToText(s string) string {
	s = htmlDrop.ReplaceAllString(s, "")
	s = htmlQuote.ReplaceAllStringFunc(s, func(q string) string { return "\n\n> quoted\n\n" })
	s = htmlBreak.ReplaceAllString(s, "\n")
	s = htmlBlock.ReplaceAllString(s, "\n\n")
	s = htmlTag.ReplaceAllString(s, "")
	return html.UnescapeString(s)
}
