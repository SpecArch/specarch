package problems

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// Fragment is one YAML file of a specification with its marks written:
// its path, its text, and whether that differs from the text on disk.
type Fragment struct {
	Path    string
	Content string
	Changed bool
}

// MarkLine is the mark of a problem in a YAML file, without its
// indentation: the problem's line from the severity on, without the
// pointer, which the place of the mark and the id already give.
func MarkLine(p Problem) string {
	return source.MarkPrefix + " " + string(p.Severity) + ": " + p.Rule + ": " + oneParagraph(p.Message) + " [" + p.ID + "]"
}

// Files are the YAML files of a specification's folder that it read: its
// root file, its fragments, its implementation files, and any file in the
// folder a problem is in, such as a fragment that does not parse. A file
// it did not read, such as an input of a conformance case under tests/, is
// never marked.
func Files(s *spec.Spec, ps []Problem) []string {
	seen := map[string]bool{}
	var out []string
	add := func(f string) {
		if f == "" {
			return
		}
		f = filepath.Clean(f)
		ext := filepath.Ext(f)
		if seen[f] || !inside(s.Dir, f) || (ext != ".yaml" && ext != ".yml") {
			return
		}
		seen[f] = true
		out = append(out, f)
	}
	add(s.RootFile)
	for _, f := range s.Files {
		add(f)
	}
	for _, i := range s.Implementations {
		add(i.Path)
	}
	for _, p := range ps {
		add(p.disk)
	}
	sort.Strings(out)
	return out
}

// inside tells whether a file is in a folder or below it, whichever of
// them is given as a relative path, such as the folder ".".
func inside(dir, file string) bool {
	d, err1 := filepath.Abs(dir)
	f, err2 := filepath.Abs(file)
	if err1 != nil || err2 != nil {
		return false
	}
	r, err := filepath.Rel(d, f)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) && r != "."
}

// Mark writes the marks of the problems into the files given (ADR-065,
// ADR-066): every line that is a mark is taken out, and each problem in one
// of those files gets its mark on the line above its entry, with the
// entry's indentation, in the order of ps. No other line changes. It
// returns each file's text, and where each line of those files has moved
// to in it.
func Mark(s *spec.Spec, ps []Problem, files []string) ([]Fragment, Moved) {
	placer := validate.SpecPlacer(s)
	marked := map[string]bool{}
	for _, f := range files {
		marked[filepath.Clean(f)] = true
	}
	texts := map[string][]string{}
	read := func(f string) []string {
		if lines, ok := texts[f]; ok {
			return lines
		}
		var lines []string
		if data, err := os.ReadFile(f); err == nil {
			lines = strings.SplitAfter(string(data), "\n")
		}
		texts[f] = lines
		return lines
	}
	above := map[string]map[int][]string{} // file, line of the entry: its marks
	for _, p := range ps {
		f := filepath.Clean(p.disk)
		if !marked[f] {
			continue
		}
		lines := read(f)
		at := anchor(s, placer, f, lines, validate.Tokens(p.Path))
		if above[f] == nil {
			above[f] = map[int][]string{}
		}
		above[f][at] = append(above[f][at], MarkLine(p))
	}
	moved := Moved{}
	var out []Fragment
	for _, f := range files {
		f = filepath.Clean(f)
		lines := read(f)
		if lines == nil {
			continue
		}
		text, newLine := rewrite(lines, above[f])
		moved[f] = newLine
		out = append(out, Fragment{Path: f, Content: text, Changed: text != strings.Join(lines, "")})
	}
	return out, moved
}

// Moved is where each line of the files a run marks has moved to: by file,
// the new line of each old line.
type Moved map[string][]int

// Line is where a line of a file is after the marks; a line of a file not
// marked stays.
func (m Moved) Line(file string, line int) int {
	if newLine, ok := m[filepath.Clean(file)]; ok && line >= 1 && line < len(newLine) && newLine[line] > 0 {
		return newLine[line]
	}
	return line
}

// Problems are the problems with their lines, and their notes' lines, as
// they are after the marks.
func (m Moved) Problems(ps []Problem) []Problem {
	out := make([]Problem, len(ps))
	for i, p := range ps {
		p.Line = m.Line(p.disk, p.Line)
		notes := make([]Note, len(p.Notes))
		for j, n := range p.Notes {
			n.Line = m.Line(n.disk, n.Line)
			notes[j] = n
		}
		p.Notes = notes
		out[i] = p
	}
	return out
}

// rewrite takes the marks out of a file's lines and writes the given marks
// above the lines they belong to; 0 is the top of the file. newLine[i] is
// where old line i is now (0 for a mark that is gone).
func rewrite(lines []string, above map[int][]string) (string, []int) {
	var b strings.Builder
	newLine := make([]int, len(lines)+1)
	n := 0
	crlf := len(lines) > 0 && strings.HasSuffix(lines[0], "\r\n")
	write := func(marks []string, indent string) {
		for _, m := range marks {
			b.WriteString(indent + m)
			if crlf {
				b.WriteString("\r")
			}
			b.WriteString("\n")
			n++
		}
	}
	top := firstEntry(lines)
	for i, l := range lines {
		if i+1 == top {
			write(above[0], "")
		}
		if source.IsMark(l) {
			continue
		}
		write(above[i+1], indentOf(l))
		b.WriteString(l)
		n++
		newLine[i+1] = n
	}
	if top == 0 {
		if len(above[0]) > 0 && b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteString("\n")
		}
		write(above[0], "")
	}
	return b.String(), newLine
}

// firstEntry is the first line that is not blank, a comment or a mark, where
// the marks of the file as a whole go, so that a schema line or other
// comment at the top stays first; 0 when there is none.
func firstEntry(lines []string) int {
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if t != "" && !strings.HasPrefix(t, "#") {
			return i + 1
		}
	}
	return 0
}

func indentOf(l string) string {
	return l[:len(l)-len(strings.TrimLeft(l, " \t"))]
}

// anchor is the line a problem's mark goes above: where the deepest entry
// on its pointer starts in block style, the key of its pair or the - of its
// item, in the file the problem is in. An entry inside a flow collection
// is held by the block-style entry above it; a pointer that leads to no
// entry of the file is about the file as a whole, and its mark goes at the
// top (0).
func anchor(s *spec.Spec, placer *validate.Placer, file string, lines []string, tokens []string) int {
	root := placer.TreeOf(file, tokens)
	if root == nil {
		return 0
	}
	own := root != s.Root
	belongs := func(n *yaml.Node) bool {
		return n != nil && n.Line > 0 && (own || filepath.Clean(s.Files[n]) == file)
	}
	at := 0
	value := root
	for _, t := range tokens {
		parent := source.Deref(value)
		next := source.Child(parent, t)
		if next == nil || parent.Style&yaml.FlowStyle != 0 {
			break
		}
		var entry *yaml.Node
		item := false
		if parent.Kind == yaml.MappingNode {
			entry = source.Key(parent, t)
		} else {
			for i, c := range parent.Content {
				if source.Deref(c) == next {
					entry = parent.Content[i]
					break
				}
			}
			item = true
		}
		if belongs(entry) && entry.Line <= len(lines) {
			at = entry.Line
			if item {
				at = dashLine(lines, entry)
			}
		}
		value = next
	}
	return at
}

// dashLine is the line of the - that starts a list item: the item's own
// line when the - is on it, or the line above it that holds only the -.
func dashLine(lines []string, item *yaml.Node) int {
	for l := item.Line; l >= 1; l-- {
		text := strings.TrimRight(lines[l-1], "\r\n")
		indent := len(indentOf(text))
		t := strings.TrimSpace(text)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasPrefix(t, "-") && indent < item.Column-1 {
			return l
		}
		if l != item.Line {
			break
		}
	}
	return item.Line
}
