// Package pdf writes a book of Markdown documents as one tagged PDF on A4
// pages (ADR-248): front matter, a table of contents, the documents, and
// the list of the problems and gaps they mark, with bookmarks, links and
// the same bytes for the same input. It knows Markdown and PDF, nothing
// of a specification: what links, what is a mark and the words it writes
// come from the caller.
package pdf

import (
	"fmt"
	"strings"
)

// Part is one document of a book, by the name its links use ("techspec"
// for a link to techspec.md) and its Markdown.
type Part struct {
	Name     string
	Markdown string
}

// Texts are the words a book writes of its own, in the document's
// language.
type Texts struct {
	Contents      string // the table of contents' heading
	FrontMatter   string // the front matter's heading in the problems and gaps list
	Problems      string // the problems and gaps list's heading
	ProblemsIntro string // the sentence under it, with %d for the count
	NoProblems    string // the sentence when there is none
	OnPage        string // "page %d", after a list entry
	PageOf        string // "Page %d of %d", the footer
	Mermaid       string // the label above a Mermaid block
}

// Options are what a book is made of.
type Options struct {
	// Title is the book's title: the header of every page, the document
	// title and the first bookmark.
	Title string
	// Lang is the document's language tag, en or id.
	Lang string
	// FrontMatter is the Markdown of the front matter, before the table of
	// contents; Parts are the documents after it.
	FrontMatter string
	Parts       []Part
	// IDs are the element ids that link to the heading or table row that
	// describes them.
	IDs map[string]bool
	// Slug is the anchor a heading gets in its document, as the documents'
	// own links name it; a repeated one gets -1, -2 and so on.
	Slug func(string) string
	// IsMark says whether a paragraph whose first bold text is lead marks
	// a problem or a gap; text is the whole paragraph.
	IsMark func(lead, text string) bool
	Texts  Texts
}

// Write makes the book.
func Write(opt Options) ([]byte, error) {
	m, err := fonts()
	if err != nil {
		return nil, err
	}
	b := newBook(opt, m)
	b.prepare()
	b.layout()
	return b.emit()
}

// A4 in points, and the margins.
const (
	pageW   = 595.28
	pageH   = 841.89
	margin  = 56.69 // 20 mm
	headerY = 36.0
	footerY = pageH - 30.0
	top     = margin
	bottom  = pageH - margin
	left    = margin
	right   = pageW - margin
	width   = right - left
)

type rgb [3]float64

var (
	black     = rgb{0.1, 0.1, 0.1}
	grey      = rgb{0.4, 0.4, 0.4}
	linkBlue  = rgb{0.05, 0.25, 0.6}
	markFill  = rgb{1, 0.95, 0.86}
	markBar   = rgb{0.8, 0.4, 0}
	headFill  = rgb{0.91, 0.92, 0.94}
	codeFill  = rgb{0.95, 0.95, 0.95}
	ruleGrey  = rgb{0.7, 0.7, 0.7}
	quoteGrey = rgb{0.75, 0.75, 0.75}
)

// docPart is a part read into blocks, with the anchors of its headings.
type docPart struct {
	name   string
	blocks []block
}

type book struct {
	opt   Options
	m     [faces]*metrics
	parts []docPart // the front matter first
	// dests are the destinations a link can name, and where each is once
	// laid out.
	dests map[string]*dest
	// idDest is the destination of each element id that has one.
	idDest map[string]string
	toc    []tocEntry
	marks  []*mark
	pages  []*page
	cur    *page
	y      float64
	root   *elem
	// outline holds the headings to the fourth level, in book order.
	outline []outlineEntry
	// titles are each part's first heading, by the part's name.
	titles map[string]string
}

type dest struct {
	page *page
	y    float64
}

type tocEntry struct {
	level int
	text  string
	dest  string
}

type outlineEntry struct {
	level int
	title string
	dest  string
}

type mark struct {
	n    int
	part string
	text string
	dest string // the mark's, "mark/<n>"
	back string // the list entry's, "problem/<n>"
}

func newBook(opt Options, m [faces]*metrics) *book {
	if opt.Slug == nil {
		opt.Slug = func(s string) string { return s }
	}
	if opt.IsMark == nil {
		opt.IsMark = func(string, string) bool { return false }
	}
	return &book{opt: opt, m: m, dests: map[string]*dest{}, idDest: map[string]string{}, titles: map[string]string{}, root: &elem{typ: "Document"}}
}

// prepare reads the parts and gives every heading, mark and id its
// destination, so that a link laid out before what it names finds it.
func (b *book) prepare() {
	b.parts = append(b.parts, docPart{name: "front", blocks: parseMarkdown(b.opt.FrontMatter)})
	for _, p := range b.opt.Parts {
		b.parts = append(b.parts, docPart{name: p.Name, blocks: parseMarkdown(p.Markdown)})
	}
	b.dests["contents"] = &dest{}
	b.dests["problems"] = &dest{}
	for pi := range b.parts {
		p := &b.parts[pi]
		slugs := map[string]int{}
		first := true
		walk(p.blocks, func(bl *block) {
			switch bl.kind {
			case headingBlock:
				text := plainText(bl.text)
				anchor := b.opt.Slug(text)
				if n := slugs[anchor]; n > 0 {
					slugs[anchor] = n + 1
					anchor = fmt.Sprintf("%s-%d", anchor, n)
				} else {
					slugs[anchor] = 1
				}
				name := p.name + "/" + anchor
				bl.dest = name
				b.dests[name] = &dest{}
				if first {
					// A link to the document itself goes to its first heading.
					b.dests[p.name] = b.dests[name]
					b.titles[p.name] = text
					first = false
				}
				if b.opt.IDs[text] && b.idDest[text] == "" {
					b.idDest[text] = name
				}
				if pi > 0 && bl.level <= 3 {
					b.toc = append(b.toc, tocEntry{level: bl.level, text: text, dest: name})
				}
			case paragraphBlock:
				if len(bl.text) > 0 && bl.text[0].style&strong != 0 && b.opt.IsMark(bl.text[0].text, plainText(bl.text)) {
					n := len(b.marks) + 1
					mk := &mark{n: n, part: p.name, text: plainText(bl.text), dest: fmt.Sprintf("mark/%d", n), back: fmt.Sprintf("problem/%d", n)}
					b.marks = append(b.marks, mk)
					bl.mark = mk
					b.dests[mk.dest] = &dest{}
					b.dests[mk.back] = &dest{}
				}
			}
		})
	}
	// A table row is an id's destination when no heading is.
	for pi := range b.parts {
		walk(b.parts[pi].blocks, func(bl *block) {
			if bl.kind != tableBlock {
				return
			}
			bl.rowDest = make([]string, len(bl.rows))
			for r, row := range bl.rows {
				if r == 0 || len(row) == 0 {
					continue
				}
				id := strings.TrimSpace(plainText(row[0]))
				if b.opt.IDs[id] && b.idDest[id] == "" {
					name := "row/" + id
					b.idDest[id] = name
					b.dests[name] = &dest{}
					bl.rowDest[r] = name
				}
			}
		})
	}
}

// walk visits every block, the ones nested in list items too.
func walk(blocks []block, f func(*block)) {
	for i := range blocks {
		f(&blocks[i])
		for j := range blocks[i].items {
			walk(blocks[i].items[j].sub, f)
		}
	}
}

// resolve is where a link written in a part goes: a destination in the
// book, a URI, or nowhere.
func (b *book) resolve(part, target string) (dest, uri string) {
	switch {
	case strings.HasPrefix(target, "http://"), strings.HasPrefix(target, "https://"), strings.HasPrefix(target, "mailto:"):
		return "", target
	case strings.HasPrefix(target, "#"):
		name := part + "/" + target[1:]
		if b.dests[name] != nil {
			return name, ""
		}
		return "", ""
	}
	file, anchor, _ := strings.Cut(target, "#")
	if strings.Contains(file, "/") || !strings.HasSuffix(file, ".md") {
		return "", ""
	}
	name := strings.TrimSuffix(file, ".md")
	if anchor != "" {
		name += "/" + anchor
	}
	if b.dests[name] != nil {
		return name, ""
	}
	return "", ""
}
