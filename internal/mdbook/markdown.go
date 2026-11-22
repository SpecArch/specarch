package mdbook

import (
	"strings"
	"unicode/utf8"
)

// The Markdown the documents of SpecArch write, read into blocks: ATX
// headings, paragraphs, pipe tables, bullet and numbered lists, fenced
// blocks, block quotes and thematic breaks, with bold, italic, code
// spans, links and autolinks inside. An HTML comment is left out; a line
// of any other kind is a paragraph, printed as it is.

type BlockKind int

const (
	ParagraphBlock BlockKind = iota
	HeadingBlock
	TableBlock
	ListBlock
	CodeBlock
	QuoteBlock
	RuleBlock
)

type Block struct {
	Kind  BlockKind
	Level int    // a heading's level
	Text  []Span // a heading's, a paragraph's or a quote's text
	// Rows are a table's Rows, the header first; align is unused, every
	// cell is set left as the documents write them.
	Rows [][][]Span
	// Items are a list's Items; ordered says it is numbered.
	Items   []Item
	Ordered bool
	Code    string // a fenced block's text
	Lang    string // its info string

	// Set while the book is prepared: a heading's destination, the mark a
	// paragraph is, and the destination of each table row an id starts.
	Dest    string
	Mark    *Mark
	RowDest []string
}

type Item struct {
	Label string // "1." or the bullet
	Text  []Span
	Sub   []Block // the blocks nested in the item: a list, mostly
}

type Style int

const (
	Plain  Style = 0
	Strong Style = 1 << iota
	Emphasis
	Mono
)

type Span struct {
	Text  string
	Style Style
	Link  string // the link's target, as written
}

// Parse reads a document into blocks.
func Parse(text string) []Block {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	p := &mdParser{lines: lines}
	return p.blocks(0, len(lines))
}

type mdParser struct {
	lines []string
}

func indentOf(l string) int {
	n := 0
	for _, r := range l {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

func (p *mdParser) blocks(from, to int) []Block {
	var out []Block
	i := from
	for i < to {
		raw := p.lines[i]
		l := strings.TrimSpace(raw)
		switch {
		case l == "":
			i++
		case strings.HasPrefix(l, "<!--"):
			for i < to && !strings.Contains(p.lines[i], "-->") {
				i++
			}
			i++
		case strings.HasPrefix(l, "```") || strings.HasPrefix(l, "~~~"):
			fence := l[:3]
			lang := strings.TrimSpace(strings.TrimLeft(l, fence[:1]))
			var body []string
			i++
			for i < to && !strings.HasPrefix(strings.TrimSpace(p.lines[i]), fence) {
				line := p.lines[i]
				// The fence's indentation is taken off each line.
				cut := min(indentOf(line), indentOf(raw))
				body = append(body, trimIndent(line, cut))
				i++
			}
			i++
			out = append(out, Block{Kind: CodeBlock, Code: strings.Join(body, "\n"), Lang: lang})
		case headingLevel(l) > 0:
			lvl := headingLevel(l)
			t := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(l[lvl:]), "#"))
			out = append(out, Block{Kind: HeadingBlock, Level: lvl, Text: ParseInline(t)})
			i++
		case isRule(l):
			out = append(out, Block{Kind: RuleBlock})
			i++
		case strings.HasPrefix(l, "|"):
			j := i
			for j < to && strings.HasPrefix(strings.TrimSpace(p.lines[j]), "|") {
				j++
			}
			out = append(out, p.table(i, j))
			i = j
		case listMarker(l) != "":
			b, next := p.list(i, to)
			out = append(out, b)
			i = next
		case strings.HasPrefix(l, ">"):
			var q []string
			for i < to && strings.HasPrefix(strings.TrimSpace(p.lines[i]), ">") {
				q = append(q, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(p.lines[i]), ">")))
				i++
			}
			out = append(out, Block{Kind: QuoteBlock, Text: ParseInline(strings.Join(q, " "))})
		default:
			var para []string
			for i < to {
				t := strings.TrimSpace(p.lines[i])
				if t == "" || headingLevel(t) > 0 || strings.HasPrefix(t, "|") || strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") || strings.HasPrefix(t, "<!--") || (len(para) > 0 && listMarker(t) != "") || isRule(t) {
					break
				}
				para = append(para, t)
				i++
			}
			out = append(out, Block{Kind: ParagraphBlock, Text: ParseInline(strings.Join(para, " "))})
		}
	}
	return out
}

func trimIndent(l string, n int) string {
	for n > 0 && len(l) > 0 && l[0] == ' ' {
		l = l[1:]
		n--
	}
	return l
}

func headingLevel(l string) int {
	n := 0
	for n < len(l) && l[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || (n < len(l) && l[n] != ' ') {
		return 0
	}
	return n
}

func isRule(l string) bool {
	if len(l) < 3 {
		return false
	}
	c := l[0]
	if c != '-' && c != '*' && c != '_' {
		return false
	}
	for i := 0; i < len(l); i++ {
		if l[i] != c && l[i] != ' ' {
			return false
		}
	}
	return true
}

// listMarker is the marker a list item starts with, "-", "*", "+" or a
// number and a dot, or "" for a line that starts none.
func listMarker(l string) string {
	if len(l) >= 2 && (l[0] == '-' || l[0] == '*' || l[0] == '+') && l[1] == ' ' {
		return l[:1]
	}
	n := 0
	for n < len(l) && n < 9 && l[n] >= '0' && l[n] <= '9' {
		n++
	}
	if n > 0 && n+1 < len(l) && (l[n] == '.' || l[n] == ')') && l[n+1] == ' ' {
		return l[:n+1]
	}
	return ""
}

func (p *mdParser) list(from, to int) (Block, int) {
	base := indentOf(p.lines[from])
	first := listMarker(strings.TrimSpace(p.lines[from]))
	b := Block{Kind: ListBlock, Ordered: first[0] >= '0' && first[0] <= '9'}
	i := from
	for i < to {
		l := p.lines[i]
		t := strings.TrimSpace(l)
		m := listMarker(t)
		if t == "" {
			// A blank line ends the list unless an item of it follows.
			j := i + 1
			for j < to && strings.TrimSpace(p.lines[j]) == "" {
				j++
			}
			if j < to && indentOf(p.lines[j]) == base && sameKind(listMarker(strings.TrimSpace(p.lines[j])), first) {
				i = j
				continue
			}
			if j < to && indentOf(p.lines[j]) > base && len(b.Items) > 0 {
				i = j
				continue
			}
			break
		}
		ind := indentOf(l)
		if ind < base || (ind == base && !sameKind(m, first)) {
			break
		}
		if ind == base {
			text := []string{strings.TrimSpace(t[len(m):])}
			i++
			// Lines indented under the item continue it, until a nested
			// list or a blank line.
			for i < to {
				n := p.lines[i]
				nt := strings.TrimSpace(n)
				if nt == "" || indentOf(n) <= base || listMarker(nt) != "" || strings.HasPrefix(nt, "```") {
					break
				}
				text = append(text, nt)
				i++
			}
			label := "•"
			if b.Ordered {
				label = m
			}
			b.Items = append(b.Items, Item{Label: label, Text: ParseInline(strings.Join(text, " "))})
			continue
		}
		// Indented deeper: blocks nested in the last item.
		j := i
		for j < to {
			n := p.lines[j]
			if strings.TrimSpace(n) != "" && indentOf(n) <= base {
				break
			}
			if strings.TrimSpace(n) == "" {
				k := j + 1
				for k < to && strings.TrimSpace(p.lines[k]) == "" {
					k++
				}
				if k >= to || indentOf(p.lines[k]) <= base {
					break
				}
			}
			j++
		}
		last := &b.Items[len(b.Items)-1]
		last.Sub = append(last.Sub, p.blocks(i, j)...)
		i = j
	}
	return b, i
}

// sameKind says whether a marker continues a list that started with
// first: a number after a number, a bullet after a bullet.
func sameKind(m, first string) bool {
	if m == "" {
		return false
	}
	isNum := func(s string) bool { return s[0] >= '0' && s[0] <= '9' }
	return isNum(m) == isNum(first)
}

func (p *mdParser) table(from, to int) Block {
	b := Block{Kind: TableBlock}
	for i := from; i < to; i++ {
		cells := splitRow(strings.TrimSpace(p.lines[i]))
		if i == from+1 && isSeparator(cells) {
			continue
		}
		row := make([][]Span, len(cells))
		for c, cell := range cells {
			row[c] = ParseInline(strings.TrimSpace(cell))
		}
		b.Rows = append(b.Rows, row)
	}
	return b
}

// splitRow splits a table row at the pipes that are not escaped or in a
// code span.
func splitRow(l string) []string {
	l = strings.TrimPrefix(l, "|")
	if strings.HasSuffix(l, "|") && !strings.HasSuffix(l, `\|`) {
		l = l[:len(l)-1]
	}
	var cells []string
	var cur strings.Builder
	inCode := false
	for i := 0; i < len(l); i++ {
		c := l[i]
		switch {
		case c == '\\' && i+1 < len(l) && l[i+1] == '|':
			cur.WriteByte('|')
			i++
		case c == '`':
			inCode = !inCode
			cur.WriteByte(c)
		case c == '|' && !inCode:
			cells = append(cells, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	return append(cells, cur.String())
}

func isSeparator(cells []string) bool {
	for _, c := range cells {
		c = strings.TrimSpace(c)
		if c == "" {
			return false
		}
		for _, r := range c {
			if r != '-' && r != ':' {
				return false
			}
		}
	}
	return len(cells) > 0
}

// ParseInline reads the inline text of a block: **bold**, *italic* and
// _italic_, `code`, [text](target), <scheme:target>, and a backslash that
// escapes the next punctuation.
func ParseInline(s string) []Span {
	var out []Span
	var cur strings.Builder
	st := Plain
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, Span{Text: cur.String(), Style: st})
			cur.Reset()
		}
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s) && isPunct(s[i+1]):
			cur.WriteByte(s[i+1])
			i += 2
		case c == '`':
			n := 1
			for i+n < len(s) && s[i+n] == '`' {
				n++
			}
			fence := strings.Repeat("`", n)
			end := strings.Index(s[i+n:], fence)
			if end < 0 {
				cur.WriteString(fence)
				i += n
				continue
			}
			flush()
			text := s[i+n : i+n+end]
			if len(text) > 1 && text[0] == ' ' && text[len(text)-1] == ' ' {
				text = text[1 : len(text)-1]
			}
			out = append(out, Span{Text: text, Style: st | Mono})
			i += n + end + n
		case c == '*' && i+1 < len(s) && s[i+1] == '*':
			flush()
			st ^= Strong
			i += 2
		case (c == '*' || c == '_') && emphasisToggle(s, i, st&Emphasis != 0):
			flush()
			st ^= Emphasis
			i++
		case c == '[':
			text, target, n := linkAt(s[i:])
			if n == 0 {
				cur.WriteByte(c)
				i++
				continue
			}
			flush()
			for _, sp := range ParseInline(text) {
				sp.Style |= st
				sp.Link = target
				out = append(out, sp)
			}
			i += n
		case c == '<':
			end := strings.IndexByte(s[i:], '>')
			inner := ""
			if end > 0 {
				inner = s[i+1 : i+end]
			}
			if strings.HasPrefix(inner, "http://") || strings.HasPrefix(inner, "https://") || strings.HasPrefix(inner, "mailto:") {
				flush()
				out = append(out, Span{Text: inner, Style: st, Link: inner})
				i += end + 1
				continue
			}
			cur.WriteByte(c)
			i++
		default:
			_, size := utf8.DecodeRuneInString(s[i:])
			cur.WriteString(s[i : i+size])
			i += size
		}
	}
	flush()
	return out
}

func isPunct(c byte) bool {
	return strings.IndexByte("\\`*_{}[]()#+-.!|<>~\"'", c) >= 0
}

// emphasisToggle says whether a single * or _ at i opens or closes
// emphasis: one that opens is followed by a non-space and has a closing
// one later; one that closes follows a non-space. An underscore inside a
// word, as in snake_case, is a letter.
func emphasisToggle(s string, i int, open bool) bool {
	c := s[i]
	prevWord := i > 0 && isWordByte(s[i-1])
	nextWord := i+1 < len(s) && isWordByte(s[i+1])
	if c == '_' && prevWord && nextWord {
		return false
	}
	if open {
		return i > 0 && s[i-1] != ' ' && (c == '*' || !nextWord)
	}
	if i+1 >= len(s) || s[i+1] == ' ' {
		return false
	}
	if c == '_' && prevWord {
		return false
	}
	return strings.IndexByte(s[i+1:], c) > 0
}

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c >= 0x80
}

// linkAt reads [text](target) at the start of s; n is 0 when s does not
// start one.
func linkAt(s string) (text, target string, n int) {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				if i+1 >= len(s) || s[i+1] != '(' {
					return "", "", 0
				}
				end := strings.IndexByte(s[i+2:], ')')
				if end < 0 {
					return "", "", 0
				}
				return s[1:i], strings.TrimSpace(s[i+2 : i+2+end]), i + 3 + end
			}
		}
	}
	return "", "", 0
}

// PlainText is the text of spans, without their styles.
func PlainText(spans []Span) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Text)
	}
	return b.String()
}
