// Package refine answers an open question: it writes the decision record
// that closes it and takes the question's entry out of its file as text,
// so that the author's other lines stay as they were (ADR-155).
package refine

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// NextID is the ID the next decision gets: ADR- and the largest number any
// decision carries plus one, at least three digits; ADR-001 for the first.
func NextID(decisions *yaml.Node) string {
	largest := 0
	for _, p := range source.Pairs(decisions) {
		rest, ok := strings.CutPrefix(p.Key.Value, "ADR-")
		if !ok {
			continue
		}
		if n, err := strconv.Atoi(rest); err == nil && n > largest {
			largest = n
		}
	}
	return fmt.Sprintf("ADR-%03d", largest+1)
}

// Record is a decision written for a question.
type Record struct {
	ID           string
	Title        string
	Date         string
	Context      string
	Decision     string
	Says         string // the answer as one paragraph, for the citation
	Consequences string
	Answers      string // the question's ID
	DecidedBy    string
	Source       string // the source cited, or "" for none
}

// Encode is the record as the text of a fragment file holding one accepted
// decision, under a comment saying what wrote it.
func (r Record) Encode() ([]byte, error) {
	entry := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	add := func(key string, value *yaml.Node) {
		entry.Content = append(entry.Content, scalar(key), value)
	}
	add("title", text(r.Title))
	add("status", scalar("accepted"))
	add("date", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: r.Date, Style: yaml.DoubleQuotedStyle})
	add("context", text(r.Context))
	add("decision", text(r.Decision))
	add("consequences", text(r.Consequences))
	add("answers", &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle, Content: []*yaml.Node{scalar(r.Answers)}})
	add("decidedBy", scalar(r.DecidedBy))
	if r.Source != "" {
		add("origin", scalar("stated"))
		cite := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Style: yaml.FlowStyle, Content: []*yaml.Node{scalar("source"), scalar(r.Source), scalar("says"), scalar(r.Says)}}
		add("cites", &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{cite}})
	}
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		scalar("decisions"),
		{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{scalar(r.ID), entry}},
	}}
	var b bytes.Buffer
	fmt.Fprintf(&b, "# Written by specarch decide; answers %s. Edit it as any decision.\n", r.Answers)
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func scalar(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}

// text is a string as YAML writes it: a literal block when it holds more
// than one line, so that prose reads as prose.
func text(s string) *yaml.Node {
	n := scalar(s)
	if strings.Contains(strings.TrimSpace(s), "\n") {
		n.Style = yaml.LiteralStyle
	}
	return n
}

// Wrap breaks a text into lines of at most width characters, at spaces,
// keeping the paragraph breaks it has.
func Wrap(s string, width int) string {
	var paragraphs []string
	for _, p := range strings.Split(strings.TrimSpace(s), "\n\n") {
		var lines []string
		line := ""
		for _, w := range strings.Fields(p) {
			if line != "" && len(line)+1+len(w) > width {
				lines = append(lines, line)
				line = ""
			}
			if line != "" {
				line += " "
			}
			line += w
		}
		if line != "" {
			lines = append(lines, line)
		}
		paragraphs = append(paragraphs, strings.Join(lines, "\n"))
	}
	return strings.Join(paragraphs, "\n\n")
}

// Remove takes the entry of the question id out of a file's text: the
// lines from the problem marks above its key to the last line of its
// value, found by indentation, and the questions key with them when the
// entry was its only one. empty says the file holds nothing but blank and
// comment lines now. The error says why the text cannot be edited: the
// entry is not written in block style, or the edited text does not hold
// every other entry the file held.
func Remove(data []byte, id string) (out []byte, empty bool, err error) {
	doc := source.Parse(data)
	if doc.Root == nil {
		return nil, false, fmt.Errorf("the file does not parse")
	}
	questions := source.Child(doc.Root, "questions")
	questionsKey := source.Key(doc.Root, "questions")
	key := source.Key(questions, id)
	if key == nil || questionsKey == nil {
		return nil, false, fmt.Errorf("the file holds no question %s", id)
	}
	lines := strings.SplitAfter(string(data), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	only := len(source.Pairs(questions)) == 1
	first, last, err := entryLines(lines, key)
	if err != nil {
		return nil, false, err
	}
	if only {
		if first, last, err = entryLines(lines, questionsKey); err != nil {
			return nil, false, err
		}
	}
	var b strings.Builder
	for i, l := range lines {
		if i+1 < first || i+1 > last {
			b.WriteString(l)
		}
	}
	out = []byte(b.String())
	if blank(out) {
		return out, true, nil
	}
	if err := sameButQuestion(doc.Root, source.Parse(out).Root, id, only); err != nil {
		return nil, false, fmt.Errorf("taking out lines %d to %d would leave a file that %s", first, last, err)
	}
	return out, false, nil
}

// entryLines is the first and the last line of a mapping entry, from 1:
// the problem marks above its key, the key's line, and every line below it
// that is blank or indented past the key, without the blank lines at the
// end. An entry whose key does not start its line is in flow style, and
// is refused.
func entryLines(lines []string, key *yaml.Node) (first, last int, err error) {
	if key.Line < 1 || key.Line > len(lines) {
		return 0, 0, fmt.Errorf("the entry %s is at line %d, past the end of the file", key.Value, key.Line)
	}
	indent := key.Column - 1
	if strings.TrimSpace(lines[key.Line-1][:min(indent, len(lines[key.Line-1]))]) != "" || indent < 0 {
		return 0, 0, fmt.Errorf("the entry %s on line %d is not written in block style, with its key starting the line; take it out by hand", key.Value, key.Line)
	}
	first = key.Line
	for first > 1 && source.IsMark(lines[first-2]) && leading(lines[first-2]) == indent {
		first--
	}
	last = key.Line
	for i := key.Line; i < len(lines); i++ {
		l := lines[i]
		if strings.TrimSpace(l) == "" {
			continue
		}
		if leading(l) <= indent {
			break
		}
		last = i + 1
	}
	return first, last, nil
}

// leading counts the spaces a line starts with.
func leading(l string) int {
	return len(l) - len(strings.TrimLeft(l, " "))
}

// blank tells whether a text holds nothing but blank and comment lines.
func blank(data []byte) bool {
	for _, l := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(l)
		if t != "" && !strings.HasPrefix(t, "#") {
			return false
		}
	}
	return true
}

// sameButQuestion checks that the edited file holds every top-level entry
// the file held, and under questions every question but id; with only,
// the questions key is gone too.
func sameButQuestion(before, after *yaml.Node, id string, only bool) error {
	if after == nil {
		return fmt.Errorf("does not parse")
	}
	want := shape(before)
	if only {
		delete(want, "questions")
	} else {
		var kept []string
		for _, q := range want["questions"] {
			if q != id {
				kept = append(kept, q)
			}
		}
		want["questions"] = kept
	}
	got := shape(after)
	for key, children := range want {
		g, ok := got[key]
		if !ok {
			return fmt.Errorf("no longer holds %s", key)
		}
		if strings.Join(g, "\x00") != strings.Join(children, "\x00") {
			return fmt.Errorf("no longer holds every entry of %s", key)
		}
	}
	for key := range got {
		if _, ok := want[key]; !ok {
			return fmt.Errorf("holds %s, which it did not", key)
		}
	}
	return nil
}

// shape is, per top-level key, the sorted names of the entries under it.
func shape(root *yaml.Node) map[string][]string {
	out := map[string][]string{}
	for _, p := range source.Pairs(root) {
		var names []string
		for _, c := range source.Pairs(p.Value) {
			names = append(names, c.Key.Value)
		}
		sort.Strings(names)
		out[p.Key.Value] = names
	}
	return out
}
