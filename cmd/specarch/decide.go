package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/refine"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"

	"go.yaml.in/yaml/v3"
)

// runDecide answers an open question (ADR-155): it writes the decision
// record under the next ID as design/decisions/<ID>.yaml, takes the
// question out of the file that holds it by editing the text, and prints
// the entries the question blocked, so that the agent writes them from the
// answer. It works on a specification with errors too, after printing
// them (ADR-065), and exits 1 then.
func runDecide(args []string, stdout, stderr io.Writer) int {
	by, answer, src, date := "", "", "", ""
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, value, ok := "", "", false
		switch {
		case a == "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
			continue
		case a == "--by" || a == "--answer" || a == "--source" || a == "--date":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "specarch decide: %s needs a value\n\n%s", a, usage)
				return 2
			}
			name, value, ok = a, args[i+1], true
			i++
		case strings.HasPrefix(a, "--by=") || strings.HasPrefix(a, "--answer=") || strings.HasPrefix(a, "--source=") || strings.HasPrefix(a, "--date="):
			name, value, _ = strings.Cut(a, "=")
			ok = true
		case strings.HasPrefix(a, "-") && len(a) > 1:
			fmt.Fprintf(stderr, "specarch decide has no option %s; its options are --by, --answer, --source and --date\n\n%s", a, usage)
			return 2
		default:
			rest = append(rest, a)
			continue
		}
		if ok {
			switch name {
			case "--by":
				by = value
			case "--answer":
				answer = value
			case "--source":
				src = value
			case "--date":
				date = value
			}
		}
	}
	switch {
	case by == "":
		fmt.Fprintf(stderr, "specarch decide needs --by <stakeholder>, the role that decided\n\n%s", usage)
		return 2
	case strings.TrimSpace(answer) == "":
		fmt.Fprintf(stderr, "specarch decide needs --answer <text>, the answer the stakeholder gave\n\n%s", usage)
		return 2
	case len(rest) != 2:
		fmt.Fprintf(stderr, "specarch decide needs the question's ID and one folder, in that order\n\n%s", usage)
		return 2
	}
	id, folder := rest[0], rest[1]
	if date == "" {
		date = time.Now().Format("2006-01-02")
	} else if _, err := time.Parse("2006-01-02", date); err != nil {
		fmt.Fprintf(stderr, "specarch decide: the date is written as YYYY-MM-DD, not %q\n", date)
		return 2
	}
	specs, invalid, status := readSpecs([]string{folder}, "decide", stdout, stderr)
	if status != 0 {
		return status
	}
	if len(specs) != 1 {
		fmt.Fprintf(stderr, "specarch decide: %s holds %d specifications; name the folder of the one the question is in\n", folder, len(specs))
		return 2
	}
	l := specs[0]
	root := l.spec.Root
	if root == nil {
		fmt.Fprintf(stderr, "specarch decide: nothing of %s could be read, so no question can be found; fix the errors above first\n", l.spec.RootFile)
		return 1
	}
	questions := source.Child(root, "questions")
	q := source.Child(questions, id)
	if q == nil {
		var ids []string
		for _, p := range source.Pairs(questions) {
			ids = append(ids, p.Key.Value)
		}
		sort.Strings(ids)
		open := "it has no open question"
		if len(ids) > 0 {
			open = "its open questions are " + strings.Join(ids, ", ")
		}
		fmt.Fprintf(stderr, "specarch decide: %s has no open question %s; %s\n", l.spec.Dir, id, open)
		return 1
	}
	if source.Child(source.Child(root, "stakeholders"), by) == nil {
		fmt.Fprintf(stderr, "specarch decide: %s is not a stakeholder of %s; decide as one of its stakeholders, by key\n", by, l.spec.Dir)
		return 1
	}
	tracks := source.Str(source.Child(source.Child(root, "info"), "tracksOrigin")) == "true"
	sources := source.Child(root, "sources")
	switch {
	case src != "" && source.Child(sources, src) == nil:
		var keys []string
		for _, p := range source.Pairs(sources) {
			keys = append(keys, p.Key.Value)
		}
		sort.Strings(keys)
		declared := "declares no source"
		if len(keys) > 0 {
			declared = "declares " + strings.Join(keys, ", ")
		}
		fmt.Fprintf(stderr, "specarch decide: %s is not a source of %s, which %s; declare it under sources in %s, as an interview with its date for an answer given in words\n", src, l.spec.Dir, declared, spec.RootFile)
		return 1
	case src == "" && tracks:
		fmt.Fprintf(stderr, "specarch decide: %s tracks origin, so the decision must cite where the answer came from; declare the answer under sources in %s, as an interview with its date, and give it with --source\n", l.spec.Dir, spec.RootFile)
		return 1
	}
	hasDesign := false
	for _, s := range l.spec.Stages {
		hasDesign = hasDesign || s == "design"
	}
	if !hasDesign {
		fmt.Fprintf(stderr, "specarch decide: stages in %s does not list design, where decisions live; list it and make a design/ folder, then decide again\n", l.spec.RootFile)
		return 1
	}
	decisionID := refine.NextID(source.Child(root, "decisions"))
	recordPath := filepath.Join(l.spec.Dir, "design", "decisions", decisionID+".yaml")
	if _, err := os.Stat(recordPath); err == nil {
		fmt.Fprintf(stderr, "specarch decide: %s exists although no decision carries %s; read it, and name it after the decision it holds or remove it\n", recordPath, decisionID)
		return 2
	}
	questionFile := l.spec.Files[source.Key(questions, id)]
	if questionFile == "" {
		questionFile = l.spec.RootFile
	}
	data, err := os.ReadFile(questionFile)
	if err != nil {
		fmt.Fprintf(stderr, "specarch decide: cannot read %s: %v\n", questionFile, err)
		return 2
	}
	edited, empty, err := refine.Remove(data, id)
	if err != nil {
		fmt.Fprintf(stderr, "specarch decide: %s cannot be taken out of %s: %v; nothing was written\n", id, questionFile, err)
		return 1
	}

	// The entries the question blocked, for the agent to write.
	st := l.state()
	entries := blockedEntries(l, q, decisionID)
	for _, e := range entries {
		fmt.Fprintln(stdout, e.printed(st))
	}

	rec := refine.Record{
		ID:           decisionID,
		Title:        oneParagraph(source.Str(source.Child(q, "question"))),
		Date:         date,
		Context:      questionContext(id, q),
		Decision:     refine.Wrap(answer, 72),
		Says:         oneParagraph(answer),
		Consequences: refine.Wrap(consequences(entries, decisionID), 72),
		Answers:      id,
		DecidedBy:    by,
		Source:       src,
	}
	text, err := rec.Encode()
	if err != nil {
		fmt.Fprintf(stderr, "specarch decide: cannot write the decision: %v\n", err)
		return 2
	}
	if err := os.MkdirAll(filepath.Dir(recordPath), 0o755); err != nil {
		fmt.Fprintf(stderr, "specarch decide: cannot make %s: %v\n", filepath.Dir(recordPath), err)
		return 2
	}
	if err := os.WriteFile(recordPath, text, 0o644); err != nil {
		fmt.Fprintf(stderr, "specarch decide: cannot write %s: %v\n", recordPath, err)
		return 2
	}
	left := fmt.Sprintf("%s taken out of %s", id, relSlash(l.spec.Dir, questionFile))
	if empty {
		if err := os.Remove(questionFile); err != nil {
			fmt.Fprintf(stderr, "specarch decide: %s written, but %s cannot be removed: %v\n", recordPath, questionFile, err)
			return 2
		}
		left = fmt.Sprintf("%s removed, which held nothing but %s", relSlash(l.spec.Dir, questionFile), id)
	} else if err := os.WriteFile(questionFile, edited, 0o644); err != nil {
		fmt.Fprintf(stderr, "specarch decide: %s written, but %s cannot be written: %v\n", recordPath, questionFile, err)
		return 2
	}
	next := "run specarch validate"
	switch len(entries) {
	case 0:
	case 1:
		next = "write the entry above, then run specarch validate"
	default:
		next = fmt.Sprintf("write the %d entries above, then run specarch validate", len(entries))
	}
	fmt.Fprintf(stderr, "specarch decide: %s answered by %s in %s, decided by %s; %s; %s\n", id, decisionID, relSlash(l.spec.Dir, recordPath), by, left, next)
	if invalid {
		fmt.Fprintln(stderr, "specarch decide: the input has errors; they are printed above")
		return 1
	}
	return 0
}

// blockedEntry is one entry a question blocked: a pointer, with where the
// deepest entry that exists starts, or a stage or a section.
type blockedEntry struct {
	block   spec.Block
	text    string // the blocks entry as written
	file    string // relative to the specification's folder; "" for a stage or a section
	line    int
	reached bool // the whole pointer resolves
	id      string
}

func blockedEntries(l loaded, q *yaml.Node, decisionID string) []blockedEntry {
	var out []blockedEntry
	for _, item := range source.Items(source.Child(q, "blocks")) {
		e := blockedEntry{text: source.Str(item), id: decisionID}
		b, ok := spec.ParseBlock(e.text)
		if !ok {
			continue
		}
		e.block = b
		if b.IsPointer() {
			start, _, whole := validate.Locate(l.spec.Root, b.Tokens)
			e.reached = whole
			file := l.spec.Files[start]
			if file == "" {
				file = l.spec.RootFile
			}
			e.file, e.line = relSlash(l.spec.Dir, file), start.Line
		}
		out = append(out, e)
	}
	return out
}

// printed is the entry as decide prints it: file:line: pointer: what to
// write, and what the element is to carry.
func (e blockedEntry) printed(st *generate.State) string {
	b := e.block
	if !b.IsPointer() {
		what := "stage"
		if b.Section != "" {
			what = "section"
		}
		return fmt.Sprintf("%s %s: write what the question asked from %s, each entry with origin: decided and decidedIn: %s", e.text, what, e.id, e.id)
	}
	element := "#" + b.Element()
	mark := fmt.Sprintf("mark %s with origin: decided and decidedIn: %s", element, e.id)
	if element == e.text {
		mark = fmt.Sprintf("mark it with origin: decided and decidedIn: %s", e.id)
	}
	var write string
	switch {
	case !e.reached:
		write = fmt.Sprintf("write %s from %s", b.Tokens[len(b.Tokens)-1], e.id)
	case len(st.Missing[source.Pointer(b.Tokens...)]) > 0:
		write = fmt.Sprintf("write %s from %s", joinAnd(st.Missing[source.Pointer(b.Tokens...)]), e.id)
	default:
		write = fmt.Sprintf("write it from %s", e.id)
	}
	return fmt.Sprintf("%s:%d: %s: %s; %s", e.file, e.line, e.text, write, mark)
}

// questionContext is the record's context: the question, its options, what
// it blocked and its Insight, as one text.
func questionContext(id string, q *yaml.Node) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s asked: %s", id, oneParagraph(source.Str(source.Child(q, "question"))))
	var options []string
	for _, o := range source.Items(source.Child(q, "options")) {
		options = append(options, oneParagraph(source.Str(o)))
	}
	if len(options) > 0 {
		fmt.Fprintf(&b, " The options were: %s.", strings.Join(options, "; "))
	}
	var blocks []string
	for _, item := range source.Items(source.Child(q, "blocks")) {
		blocks = append(blocks, source.Str(item))
	}
	if len(blocks) > 0 {
		fmt.Fprintf(&b, " It blocked %s.", joinAnd(blocks))
	}
	if why := oneParagraph(source.Str(source.Child(q, "why"))); why != "" {
		fmt.Fprintf(&b, " %s", why)
	}
	return refine.Wrap(b.String(), 72)
}

// consequences names the entries that are to be written from the answer.
func consequences(entries []blockedEntry, decisionID string) string {
	if len(entries) == 0 {
		return "Nothing waited on the answer; the specification says what it says."
	}
	var names []string
	for _, e := range entries {
		if e.block.IsPointer() {
			names = append(names, e.text)
		} else if e.block.Section != "" {
			names = append(names, "the "+e.text+" section")
		} else {
			names = append(names, "the "+e.text+" stage")
		}
	}
	if len(names) == 1 {
		return fmt.Sprintf("%s is written from this answer, carrying origin decided and decidedIn %s.", names[0], decisionID)
	}
	return fmt.Sprintf("%s are written from this answer, each entry carrying origin decided and decidedIn %s.", joinAnd(names), decisionID)
}

func oneParagraph(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func joinAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
