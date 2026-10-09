package validate

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// Placer gives diagnostics their column and their id (ADR-065). Root and
// Files are the merged specification and the file of each of its nodes;
// a pointer that does not lead to a node of the diagnostic's file there is
// followed in that file parsed on its own. Dir is the folder ids name
// files from.
type Placer struct {
	Root     *yaml.Node
	Files    map[*yaml.Node]string
	RootFile string
	Dir      string
	texts    map[string][]string
	trees    map[string]*yaml.Node
}

// Place sets the column of each diagnostic that has none and the id of
// every one, telling apart the ids one rule reports at one pointer of one
// file.
func (p *Placer) Place(ds []Diagnostic) {
	for i := range ds {
		d := &ds[i]
		if d.Column < 1 {
			tokens := Tokens(d.Path)
			d.Column = p.column(d.File, d.Line, p.TreeOf(d.File, tokens), tokens)
		}
		d.ID = fmt.Sprintf("%s@%s#%s", d.Rule, relSlash(p.Dir, d.File), d.Path)
	}
	numberIDs(ds)
}

// TreeOf is the tree a pointer into file is into: the merged specification
// when the pointer leads there to a node of that file, and otherwise the
// file parsed on its own (an implementation file, a record, or a fragment
// the specification could not merge).
func (p *Placer) TreeOf(file string, tokens []string) *yaml.Node {
	if p.Root != nil {
		if n, ok := source.Resolve(p.Root, tokens); ok && p.Files[n] == file {
			return p.Root
		}
		if len(tokens) == 0 && file == p.RootFile {
			return p.Root
		}
	}
	if p.trees == nil {
		p.trees = map[string]*yaml.Node{}
	}
	if t, seen := p.trees[file]; seen {
		return t
	}
	var t *yaml.Node
	if data, err := os.ReadFile(file); err == nil {
		t = source.Parse(data).Root
	}
	p.trees[file] = t
	return t
}

// column is the column of the node the pointer names when the problem is on
// its line (its value, or else its key), and otherwise that of the first
// character on the line that is not a space.
func (p *Placer) column(file string, line int, root *yaml.Node, tokens []string) int {
	if root != nil {
		start, value, ok := Locate(root, tokens)
		if ok {
			if value != nil && value.Line == line && value.Column > 0 {
				return value.Column
			}
			if start != nil && start.Line == line && start.Column > 0 {
				return start.Column
			}
		}
	}
	return p.indent(file, line) + 1
}

// indent counts the spaces and tabs a line of a file starts with.
func (p *Placer) indent(file string, line int) int {
	n := 0
	for _, r := range p.line(file, line) {
		if r != ' ' && r != '\t' {
			return n
		}
		n++
	}
	return 0
}

// line is the text of a line of a file, from 1; "" past its end.
func (p *Placer) line(file string, line int) string {
	if p.texts == nil {
		p.texts = map[string][]string{}
	}
	lines, seen := p.texts[file]
	if !seen {
		if data, err := os.ReadFile(file); err == nil {
			lines = strings.Split(string(data), "\n")
		}
		p.texts[file] = lines
	}
	if line < 1 || line > len(lines) {
		return ""
	}
	return lines[line-1]
}

// Locate follows a pointer from root. It returns where the deepest entry
// reached starts (its key, or its item in a list), its value, and whether
// the whole pointer was reached. For the empty pointer both are the root.
func Locate(root *yaml.Node, tokens []string) (start, value *yaml.Node, whole bool) {
	start, value = root, root
	for _, t := range tokens {
		parent := source.Deref(value)
		next := source.Child(parent, t)
		if next == nil {
			return start, value, false
		}
		if parent.Kind == yaml.MappingNode {
			start = source.Key(parent, t)
		} else {
			start = next
		}
		value = next
	}
	return start, value, true
}

// Tokens splits a JSON pointer into its unescaped tokens; none for "/".
func Tokens(ptr string) []string {
	if ptr == "" || ptr == "/" {
		return nil
	}
	var out []string
	for _, t := range strings.Split(strings.TrimPrefix(ptr, "/"), "/") {
		out = append(out, source.UnescapeToken(t))
	}
	return out
}

// relSlash names a file relative to a folder, with slashes.
func relSlash(from, to string) string {
	af, err1 := filepath.Abs(from)
	at, err2 := filepath.Abs(to)
	if err1 == nil && err2 == nil {
		if r, err := filepath.Rel(af, at); err == nil {
			return filepath.ToSlash(r)
		}
	}
	return filepath.ToSlash(to)
}

// numberIDs tells apart the diagnostics of one rule at one pointer of one
// file: each gets .<tag> after the rule, eight hex digits of the FNV-1a
// hash of its message, so that fixing one leaves the others' ids as they
// are. Diagnostics with the same message as well are numbered -2, -3 and
// on.
func numberIDs(ds []Diagnostic) {
	groups := map[string][]int{}
	var order []string
	for i, d := range ds {
		if _, seen := groups[d.ID]; !seen {
			order = append(order, d.ID)
		}
		groups[d.ID] = append(groups[d.ID], i)
	}
	for _, id := range order {
		idx := groups[id]
		if len(idx) < 2 {
			continue
		}
		seen := map[string]int{}
		for _, i := range idx {
			d := &ds[i]
			h := fnv.New32a()
			h.Write([]byte(d.Message))
			tag := fmt.Sprintf("%08x", h.Sum32())
			seen[tag]++
			if seen[tag] > 1 {
				tag = fmt.Sprintf("%s-%d", tag, seen[tag])
			}
			d.ID = strings.Replace(d.ID, string(d.Rule)+"@", string(d.Rule)+"."+tag+"@", 1)
		}
	}
}
