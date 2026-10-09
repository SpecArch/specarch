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
	disk    string // the file as read, for its marks
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
	disk                      string // the file as read, for its marks
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
	c := &collector{s: s, folder: folder, placer: validate.SpecPlacer(s)}
	var out []Problem
	for _, d := range diags {
		out = append(out, c.diagnostic(d))
	}
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
	placer *validate.Placer // the trees of the files read
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
		ID:       d.ID,
		Severity: Severity(d.Severity),
		File:     c.rel(d.File),
		Line:     d.Line,
		Column:   d.Column,
		Path:     d.Path,
		Rule:     string(d.Rule),
		Message:  d.Message,
		disk:     d.File,
	}
	tokens := validate.Tokens(d.Path)
	if root := c.placer.TreeOf(d.File, tokens); root != nil {
		p.Notes = c.citations(root, tokens)
	}
	return p
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
		note := Note{File: c.rel(c.fileOf(cite)), Line: cite.Line, Column: cite.Column, Message: text, disk: c.fileOf(cite)}
		if file, line, ok := c.citedLine(key, clause); ok {
			note.File, note.Line, note.Column, note.disk = c.rel(file), line, 1, file
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
		disk:      c.fileOf(q.Key),
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
		start, _, reached := validate.Locate(c.s.Root, b.Tokens)
		text := "blocks " + block
		if !reached {
			text += ", which is not given yet"
		} else if keys := missing[source.Pointer(b.Tokens...)]; len(keys) > 0 {
			text += ", which leaves out " + strings.Join(keys, ", ")
		}
		p.Notes = append(p.Notes, Note{File: c.rel(c.fileOf(start)), Line: start.Line, Column: start.Column, Message: text, disk: c.fileOf(start)})
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
