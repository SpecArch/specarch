// Package pdf writes a book of Markdown documents as one tagged PDF on A4
// pages (ADR-248): front matter, a table of contents, the documents, and
// the list of the problems and gaps they mark, with bookmarks, links and
// the same bytes for the same input. It knows Markdown and PDF, nothing
// of a specification: what links, what is a mark and the words it writes
// come from the caller.
package pdf

import (
	"github.com/SpecArch/specarch/internal/mdbook"
)

// Part is one document of a book, by the name its links use ("techspec"
// for a link to techspec.md) and its Markdown.
type Part = mdbook.Part

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

type book struct {
	opt Options
	m   [faces]*metrics
	// mb is the book read into blocks, with its destinations, table of
	// contents and marks.
	mb *mdbook.Book
	// dests are the destinations a link can name, and where each is once
	// laid out; a document's name shares its first heading's.
	dests map[string]*dest
	pages []*page
	cur   *page
	y     float64
	root  *elem
	// outline holds the headings to the fourth level, in book order.
	outline []outlineEntry
}

type dest struct {
	page *page
	y    float64
}

type outlineEntry struct {
	level int
	title string
	dest  string
}

func newBook(opt Options, m [faces]*metrics) *book {
	return &book{opt: opt, m: m, dests: map[string]*dest{}, root: &elem{typ: "Document"}}
}

// prepare reads the parts and gives every heading, mark and id its
// destination, so that a link laid out before what it names finds it.
func (b *book) prepare() {
	b.mb = mdbook.Prepare(mdbook.Options{FrontMatter: b.opt.FrontMatter, Parts: b.opt.Parts, IDs: b.opt.IDs, Slug: b.opt.Slug, IsMark: b.opt.IsMark})
	for _, name := range b.mb.Names {
		if target := b.mb.Dests[name]; target == name {
			b.dests[name] = &dest{}
		}
	}
	for _, name := range b.mb.Names {
		if target := b.mb.Dests[name]; target != name {
			b.dests[name] = b.dests[target]
		}
	}
}
