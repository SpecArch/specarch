// Package problems gathers every problem of a specification, the errors and
// warnings validate finds and the open questions, into one list, and writes
// it as the problems file and as a SARIF 2.1.0 log (ADR-065).
package problems

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// Severity is error, warning or question.
type Severity string

const (
	Error    Severity = "error"
	Warning  Severity = "warning"
	Question Severity = "question"
)

// RuleOpenQuestion is the rule of every question.
const RuleOpenQuestion = "open_question"

// Note is a place that helps with a problem: a source it cites, or an entry
// a question blocks.
type Note struct {
	File    string
	Line    int
	Column  int
	Message string
}

// Problem is one error, warning or open question.
type Problem struct {
	ID       string
	Severity Severity
	File     string
	Line     int
	Column   int
	Path     string
	Rule     string
	Message  string
	Notes    []Note
	// A question's own fields, for the SARIF properties.
	Priority, Kind, DecidedBy string
	Options, Blocks           []string
}

// String is the problem line: file:line:column: severity: pointer: rule:
// message [id].
func (p Problem) String() string {
	return fmt.Sprintf("%s:%d:%d: %s: %s: %s: %s [%s]", p.File, p.Line, p.Column, p.Severity, p.Path, p.Rule, p.Message, p.ID)
}

// String is the note line: file:line:column: note: text.
func (n Note) String() string {
	return fmt.Sprintf("%s:%d:%d: note: %s", n.File, n.Line, n.Column, n.Message)
}

// Collect gathers the problems of a specification: diags are the
// diagnostics validate keeps, covered those an open question covers. Files
// are named relative to folder, where the problems file is written.
func Collect(s *spec.Spec, diags, covered []validate.Diagnostic, folder string) []Problem {
	c := &collector{s: s, folder: folder, texts: map[string][]string{}, trees: map[string]*yaml.Node{}}
	var out []Problem
	for _, d := range diags {
		out = append(out, c.diagnostic(d))
	}
	numberIDs(out)
	missing := map[string][]string{}
	for _, d := range covered {
		if d.Rule == validate.RuleSchema && strings.HasSuffix(d.Message, " is missing; add it here") {
			missing[d.Path] = append(missing[d.Path], strings.TrimSuffix(d.Message, " is missing; add it here"))
		}
	}
	if s.Root != nil {
		for _, q := range source.Pairs(source.Child(s.Root, "questions")) {
			out = append(out, c.question(q, missing))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.ID < b.ID
	})
	return out
}

type collector struct {
	s      *spec.Spec
	folder string
	texts  map[string][]string   // the lines of each file read
	trees  map[string]*yaml.Node // each file parsed on its own
}

// rel names a file relative to the problems file's folder.
func (c *collector) rel(file string) string {
	return relSlash(c.folder, file)
}

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

func (c *collector) diagnostic(d validate.Diagnostic) Problem {
	p := Problem{
		ID:       fmt.Sprintf("%s@%s#%s", d.Rule, relSlash(c.s.Dir, d.File), d.Path),
		Severity: Severity(d.Severity),
		File:     c.rel(d.File),
		Line:     d.Line,
		Path:     d.Path,
		Rule:     string(d.Rule),
		Message:  d.Message,
	}
	tokens := tokensOf(d.Path)
	root := c.treeOf(d.File, tokens)
	p.Column = c.column(d.File, d.Line, root, tokens)
	if root != nil {
		p.Notes = c.citations(root, tokens)
	}
	return p
}

// treeOf is the tree a diagnostic's pointer is into: the merged
// specification when the pointer leads there to a node of that file, and
// otherwise the file parsed on its own (an implementation file, a record,
// or a fragment the specification could not merge).
func (c *collector) treeOf(file string, tokens []string) *yaml.Node {
	if c.s.Root != nil {
		if n, ok := source.Resolve(c.s.Root, tokens); ok && c.s.Files[n] == file {
			return c.s.Root
		}
		if len(tokens) == 0 && file == c.s.RootFile {
			return c.s.Root
		}
	}
	if t, seen := c.trees[file]; seen {
		return t
	}
	var t *yaml.Node
	if data, err := os.ReadFile(file); err == nil {
		t = source.Parse(data).Root
	}
	c.trees[file] = t
	return t
}

// column is the column of the node the pointer names when the problem is on
// its line (its value, or else its key), and otherwise that of the first
// character on the line that is not a space.
func (c *collector) column(file string, line int, root *yaml.Node, tokens []string) int {
	if root != nil {
		start, value, ok := locate(root, tokens)
		if ok {
			if value != nil && value.Line == line && value.Column > 0 {
				return value.Column
			}
			if start != nil && start.Line == line && start.Column > 0 {
				return start.Column
			}
		}
	}
	lines := c.lines(file)
	if line >= 1 && line <= len(lines) {
		text := []rune(lines[line-1])
		for i, r := range text {
			if r != ' ' && r != '\t' {
				return i + 1
			}
		}
	}
	return 1
}

func (c *collector) lines(file string) []string {
	if l, seen := c.texts[file]; seen {
		return l
	}
	var l []string
	if data, err := os.ReadFile(file); err == nil {
		l = strings.Split(string(data), "\n")
	}
	c.texts[file] = l
	return l
}

// locate follows a pointer from root. It returns where the deepest entry
// reached starts (its key, or its item in a list), its value, and whether
// the whole pointer was reached. For the empty pointer both are the root.
func locate(root *yaml.Node, tokens []string) (start, value *yaml.Node, whole bool) {
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

func tokensOf(ptr string) []string {
	if ptr == "" || ptr == "/" {
		return nil
	}
	var out []string
	for _, t := range strings.Split(strings.TrimPrefix(ptr, "/"), "/") {
		out = append(out, source.UnescapeToken(t))
	}
	return out
}

// numberIDs gives the second and later problems of one rule at one pointer
// of one file .2, .3 and on after the rule, in the order of their messages.
func numberIDs(ps []Problem) {
	groups := map[string][]int{}
	var order []string
	for i, p := range ps {
		if _, seen := groups[p.ID]; !seen {
			order = append(order, p.ID)
		}
		groups[p.ID] = append(groups[p.ID], i)
	}
	for _, id := range order {
		idx := groups[id]
		if len(idx) < 2 {
			continue
		}
		sort.SliceStable(idx, func(a, b int) bool { return ps[idx[a]].Message < ps[idx[b]].Message })
		for n, i := range idx[1:] {
			p := &ps[i]
			p.ID = strings.Replace(p.ID, p.Rule+"@", fmt.Sprintf("%s.%d@", p.Rule, n+2), 1)
		}
	}
}

// citations are the notes of the nearest element at or above the pointer
// that cites.
func (c *collector) citations(root *yaml.Node, tokens []string) []Note {
	for n := len(tokens); n >= 0; n-- {
		node, ok := source.Resolve(root, tokens[:n])
		if !ok {
			continue
		}
		if cites := source.Child(node, "cites"); cites != nil {
			return c.citeNotes(cites)
		}
	}
	return nil
}

var clauseLine = regexp.MustCompile(`^(.+):([0-9]+)$`)

// citeNotes are one note per citation: at the cited file and line when the
// source's url is a folder beside the specification and the clause is
// path:line, and otherwise at the citation.
func (c *collector) citeNotes(cites *yaml.Node) []Note {
	var out []Note
	for _, cite := range source.Items(cites) {
		key := source.Str(source.Child(cite, "source"))
		clause := source.Str(source.Child(cite, "clause"))
		says := source.Str(source.Child(cite, "says"))
		text := "source " + key
		if clause != "" {
			text += ", clause " + clause
		}
		if says != "" {
			text += ": " + oneParagraph(says)
		}
		note := Note{File: c.rel(c.fileOf(cite)), Line: cite.Line, Column: cite.Column, Message: text}
		if file, line, ok := c.citedLine(key, clause); ok {
			note.File, note.Line, note.Column = c.rel(file), line, 1
		}
		out = append(out, note)
	}
	return out
}

// citedLine is the file and line a clause names, when the source's url is a
// folder beside the specification and the clause is path:line naming a file
// in it.
func (c *collector) citedLine(key, clause string) (string, int, bool) {
	m := clauseLine.FindStringSubmatch(clause)
	if m == nil || c.s.Root == nil {
		return "", 0, false
	}
	url := source.Str(source.Child(source.Child(source.Child(c.s.Root, "sources"), key), "url"))
	if url == "" || strings.Contains(url, "://") {
		return "", 0, false
	}
	base := filepath.Join(c.s.Dir, filepath.FromSlash(url))
	if info, err := os.Stat(base); err != nil || !info.IsDir() {
		return "", 0, false
	}
	file := filepath.Join(base, filepath.FromSlash(m[1]))
	if info, err := os.Stat(file); err != nil || info.IsDir() {
		return "", 0, false
	}
	line, _ := strconv.Atoi(m[2])
	if line < 1 {
		return "", 0, false
	}
	return file, line, true
}

func (c *collector) fileOf(n *yaml.Node) string {
	if f, ok := c.s.Files[n]; ok && f != "" {
		return f
	}
	return c.s.RootFile
}

// question makes the problem of one open question, at its entry in the
// questions file.
func (c *collector) question(q source.Pair, missing map[string][]string) Problem {
	id := q.Key.Value
	n := q.Value
	str := func(key string) string { return source.Str(source.Child(n, key)) }
	p := Problem{
		ID:        id,
		Severity:  Question,
		File:      c.rel(c.fileOf(q.Key)),
		Line:      q.Key.Line,
		Column:    q.Key.Column,
		Path:      source.Pointer("questions", id),
		Rule:      RuleOpenQuestion,
		Priority:  str("priority"),
		Kind:      str("kind"),
		DecidedBy: str("decidedBy"),
	}
	for _, o := range source.Items(source.Child(n, "options")) {
		p.Options = append(p.Options, source.Str(o))
	}
	var whole []string
	for _, item := range source.Items(source.Child(n, "blocks")) {
		block := source.Str(item)
		p.Blocks = append(p.Blocks, block)
		b, ok := spec.ParseBlock(block)
		if !ok || !b.IsPointer() || c.s.Root == nil {
			whole = append(whole, block)
			continue
		}
		start, _, reached := locate(c.s.Root, b.Tokens)
		text := "blocks " + block
		if !reached {
			text += ", which is not given yet"
		} else if keys := missing[source.Pointer(b.Tokens...)]; len(keys) > 0 {
			text += ", which leaves out " + strings.Join(keys, ", ")
		}
		p.Notes = append(p.Notes, Note{File: c.rel(c.fileOf(start)), Line: start.Line, Column: start.Column, Message: text})
	}
	var b strings.Builder
	fmt.Fprintf(&b, "(%s, %s) %s", p.Priority, p.Kind, oneParagraph(str("question")))
	if len(whole) > 0 {
		fmt.Fprintf(&b, " It blocks %s.", strings.Join(whole, ", "))
	}
	if p.DecidedBy != "" {
		fmt.Fprintf(&b, " Decided by %s.", p.DecidedBy)
	}
	fmt.Fprintf(&b, " Answer with a decision record that names %s under answers.", id)
	p.Message = b.String()
	p.Notes = append(p.Notes, c.citeNotes(source.Child(n, "cites"))...)
	return p
}

func oneParagraph(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
