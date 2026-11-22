// Package mdbook reads a book of Markdown documents into the one model
// every writer of a book renders (ADR-248): the blocks of each document,
// the destination of every heading, element id, mark and list entry, the
// table of contents and the marks of the problems and gaps. The PDF and
// the Word writer both take it, so the two carry the same text, the same
// links and the same marks. It knows Markdown, nothing of a
// specification: what links and what is a mark come from the caller.
package mdbook

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

// Options are what a book is read from.
type Options struct {
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
}

// DocPart is a part read into blocks; the front matter's name is "front".
type DocPart struct {
	Name   string
	Blocks []Block
}

// TOCEntry is one line of the table of contents.
type TOCEntry struct {
	Level int
	Text  string
	Dest  string
}

// Mark is a paragraph that marks a problem or a gap, numbered in book
// order.
type Mark struct {
	N    int
	Part string
	Text string
	Dest string // the mark's, "mark/<n>"
	Back string // the list entry's, "problem/<n>"
}

// The destinations every book has: its table of contents and its list of
// the problems and gaps.
const (
	ContentsDest = "contents"
	ProblemsDest = "problems"
)

// Book is a book read and given its destinations.
type Book struct {
	Parts []DocPart // the front matter first
	// Dests are the destinations a link can name, each with the one it
	// stands for: itself, or for a document's name its first heading.
	// Names lists them in the order they were made.
	Dests map[string]string
	Names []string
	// IDDest is the destination of each element id that has one.
	IDDest map[string]string
	TOC    []TOCEntry
	Marks  []*Mark
	// Titles are each part's first heading, by the part's name.
	Titles map[string]string
}

// Prepare reads the parts and gives every heading, mark and id its
// destination, so that a link written before what it names finds it.
func Prepare(opt Options) *Book {
	if opt.Slug == nil {
		opt.Slug = func(s string) string { return s }
	}
	if opt.IsMark == nil {
		opt.IsMark = func(string, string) bool { return false }
	}
	b := &Book{Dests: map[string]string{}, IDDest: map[string]string{}, Titles: map[string]string{}}
	b.Parts = append(b.Parts, DocPart{Name: "front", Blocks: Parse(opt.FrontMatter)})
	for _, p := range opt.Parts {
		b.Parts = append(b.Parts, DocPart{Name: p.Name, Blocks: Parse(p.Markdown)})
	}
	b.add(ContentsDest, ContentsDest)
	b.add(ProblemsDest, ProblemsDest)
	for pi := range b.Parts {
		p := &b.Parts[pi]
		slugs := map[string]int{}
		first := true
		Walk(p.Blocks, func(bl *Block) {
			switch bl.Kind {
			case HeadingBlock:
				text := PlainText(bl.Text)
				anchor := opt.Slug(text)
				if n := slugs[anchor]; n > 0 {
					slugs[anchor] = n + 1
					anchor = fmt.Sprintf("%s-%d", anchor, n)
				} else {
					slugs[anchor] = 1
				}
				name := p.Name + "/" + anchor
				bl.Dest = name
				b.add(name, name)
				if first {
					// A link to the document itself goes to its first heading.
					b.add(p.Name, name)
					b.Titles[p.Name] = text
					first = false
				}
				if opt.IDs[text] && b.IDDest[text] == "" {
					b.IDDest[text] = name
				}
				if pi > 0 && bl.Level <= 3 {
					b.TOC = append(b.TOC, TOCEntry{Level: bl.Level, Text: text, Dest: name})
				}
			case ParagraphBlock:
				if len(bl.Text) > 0 && bl.Text[0].Style&Strong != 0 && opt.IsMark(bl.Text[0].Text, PlainText(bl.Text)) {
					n := len(b.Marks) + 1
					mk := &Mark{N: n, Part: p.Name, Text: PlainText(bl.Text), Dest: fmt.Sprintf("mark/%d", n), Back: fmt.Sprintf("problem/%d", n)}
					b.Marks = append(b.Marks, mk)
					bl.Mark = mk
					b.add(mk.Dest, mk.Dest)
					b.add(mk.Back, mk.Back)
				}
			}
		})
	}
	// A table row is an id's destination when no heading is.
	for pi := range b.Parts {
		Walk(b.Parts[pi].Blocks, func(bl *Block) {
			if bl.Kind != TableBlock {
				return
			}
			bl.RowDest = make([]string, len(bl.Rows))
			for r, row := range bl.Rows {
				if r == 0 || len(row) == 0 {
					continue
				}
				id := strings.TrimSpace(PlainText(row[0]))
				if opt.IDs[id] && b.IDDest[id] == "" {
					name := "row/" + id
					b.IDDest[id] = name
					b.add(name, name)
					bl.RowDest[r] = name
				}
			}
		})
	}
	return b
}

// add names a destination; a name made twice keeps the last one, as a
// map would.
func (b *Book) add(name, target string) {
	if _, ok := b.Dests[name]; !ok {
		b.Names = append(b.Names, name)
	}
	b.Dests[name] = target
}

// Walk visits every block, the ones nested in list items too.
func Walk(blocks []Block, f func(*Block)) {
	for i := range blocks {
		f(&blocks[i])
		for j := range blocks[i].Items {
			Walk(blocks[i].Items[j].Sub, f)
		}
	}
}

// Resolve is where a link written in a part goes: a destination in the
// book, a URI, or nowhere.
func (b *Book) Resolve(part, target string) (dest, uri string) {
	switch {
	case strings.HasPrefix(target, "http://"), strings.HasPrefix(target, "https://"), strings.HasPrefix(target, "mailto:"):
		return "", target
	case strings.HasPrefix(target, "#"):
		name := part + "/" + target[1:]
		if _, ok := b.Dests[name]; ok {
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
	if _, ok := b.Dests[name]; ok {
		return name, ""
	}
	return "", ""
}

// Segment is a run of text, with the destination of the element id it is,
// or "" for text around them.
type Segment struct {
	Text string
	Dest string
}

// IDLinks splits a text at the element ids that have a destination other
// than self, each id a segment of its own with its destination.
func (b *Book) IDLinks(text, self string) []Segment {
	var out []Segment
	start := 0
	for i := 0; i < len(text); {
		if !isIDByte(text[i]) || (i > 0 && isIDByte(text[i-1])) {
			i++
			continue
		}
		j := i
		for j < len(text) && isIDByte(text[j]) {
			j++
		}
		word := strings.TrimRight(text[i:j], ".")
		if d := b.IDDest[word]; d != "" && d != self {
			if start < i {
				out = append(out, Segment{Text: text[start:i]})
			}
			out = append(out, Segment{Text: word, Dest: d})
			start = i + len(word)
		}
		i = j
	}
	if start < len(text) {
		out = append(out, Segment{Text: text[start:]})
	}
	return out
}

func isIDByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.'
}
