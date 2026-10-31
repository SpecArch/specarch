package generate

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"go.yaml.in/yaml/v3"
)

// Next is the one open question of a specification to put to a person
// next, as gaps --next prints it and --json gives it (ADR-209).
type Next struct {
	// Specification is the root file, as the run names it.
	Specification string `json:"specification"`
	// Title is the specification's info.title.
	Title string `json:"title"`
	// OpenQuestions is how many questions are open.
	OpenQuestions int `json:"openQuestions"`
	// Question is the one ranked first, nil when none is open.
	Question *NextQuestion `json:"question"`
}

// NextQuestion is the question ranked first, with what the person needs
// to answer it and the command that records the answer.
type NextQuestion struct {
	ID        string   `json:"id"`
	Question  string   `json:"question"`
	Kind      string   `json:"kind"`
	Priority  string   `json:"priority"`
	DecidedBy string   `json:"decidedBy"`
	Stage     string   `json:"stage"`
	Blocks    []string `json:"blocks"`
	// Missing are, per blocked pointer, the keys missing there.
	Missing map[string][]string `json:"missing"`
	// HoldsUp are the outputs the question holds up: documents and code.
	HoldsUp   []string `json:"holdsUp"`
	Options   []string `json:"options"`
	Insight   string   `json:"insight,omitempty"`
	Certainty string   `json:"certainty,omitempty"`
	Names     string   `json:"names,omitempty"`
	// Decide is the command that records the answer, as arguments; the
	// answer is <answer>, and --source is left out when the source is
	// not settled (SourceNote says why).
	Decide     []string `json:"decide"`
	SourceNote string   `json:"sourceNote,omitempty"`
	// Recommendation is always null from the program: the agent fills it
	// with the answer it recommends and its reason before it puts the
	// question.
	Recommendation *Recommendation `json:"recommendation"`
}

// Recommendation is the answer an agent recommends, with its reason.
type Recommendation struct {
	Answer string `json:"answer"`
	Reason string `json:"reason"`
}

// NextOf ranks the open questions of a specification and returns the
// first: by priority, then by how many outputs it holds up, then by how
// many entries it blocks, then by stage in life-cycle order, then by ID.
func NextOf(root *yaml.Node, relRoot string, impls []Implementation, state *State) Next {
	qs := questionsOf(root)
	n := Next{Specification: relRoot, Title: str(get(root, "info"), "title"), OpenQuestions: len(qs)}
	if len(qs) == 0 {
		return n
	}
	outputs := outputReads(root, impls)
	held := map[string][]string{}
	for _, q := range qs {
		for _, o := range outputs {
			if q.holds(o.reads) {
				held[q.id] = append(held[q.id], o.name)
			}
		}
	}
	stageRank := func(s string) int {
		for i, st := range spec.Stages {
			if st == s {
				return i
			}
		}
		return len(spec.Stages)
	}
	sort.SliceStable(qs, func(i, j int) bool {
		a, b := qs[i], qs[j]
		if pa, pb := priorityRank(a.priority), priorityRank(b.priority); pa != pb {
			return pa < pb
		}
		if ha, hb := len(held[a.id]), len(held[b.id]); ha != hb {
			return ha > hb
		}
		if ba, bb := len(items(a.node, "blocks")), len(items(b.node, "blocks")); ba != bb {
			return ba > bb
		}
		if sa, sb := stageRank(a.stage), stageRank(b.stage); sa != sb {
			return sa < sb
		}
		return idLess(a.id, b.id)
	})
	q := qs[0]
	nq := &NextQuestion{
		ID:        q.id,
		Question:  oneParagraph(str(q.node, "question")),
		Kind:      str(q.node, "kind"),
		Priority:  q.priority,
		DecidedBy: str(q.node, "decidedBy"),
		Stage:     q.stage,
		Blocks:    []string{},
		Missing:   map[string][]string{},
		HoldsUp:   append([]string{}, held[q.id]...),
		Options:   []string{},
		Insight:   oneParagraph(strings.TrimSpace(str(q.node, "why"))),
		Certainty: str(q.node, "certainty"),
		Names:     str(q.node, "names"),
	}
	for _, item := range items(q.node, "blocks") {
		nq.Blocks = append(nq.Blocks, item.Value)
		if b, ok := spec.ParseBlock(item.Value); ok && b.IsPointer() && state != nil {
			if missing := state.Missing[source.Pointer(b.Tokens...)]; len(missing) > 0 {
				nq.Missing[item.Value] = append([]string{}, missing...)
			}
		}
	}
	for _, o := range strs(q.node, "options") {
		nq.Options = append(nq.Options, oneParagraph(o))
	}
	nq.Decide = []string{"specarch", "decide", q.id, "--by", nq.DecidedBy, "--answer", "<answer>"}
	want, kindText := "interview", "an interview"
	if q.material() {
		want, kindText = "document", "a document"
	}
	var keys []string
	for _, p := range pairs(root, "sources") {
		if str(p.Value, "kind") == want {
			keys = append(keys, p.Key.Value)
		}
	}
	switch len(keys) {
	case 1:
		nq.Decide = append(nq.Decide, "--source", keys[0])
	case 0:
		nq.SourceNote = fmt.Sprintf("the root file declares no source of kind %s; declare %s under sources for the answer to cite, and add --source with its key", want, kindText)
	default:
		nq.SourceNote = fmt.Sprintf("the root file declares %d sources of kind %s (%s); add --source with the one the answer came from", len(keys), want, joinAnd(keys))
	}
	nq.Decide = append(nq.Decide, path.Dir(relRoot))
	n.Question = nq
	return n
}

// output is one row of the open questions document's outputs table and
// the sections it reads.
type output struct {
	name     string
	reads    []string
	document bool
}

// outputReads lists the outputs the open questions document lists, each
// with the sections it reads, in the table's order.
func outputReads(root *yaml.Node, impls []Implementation) []output {
	var out []output
	for _, t := range DocumentTargets {
		if !BuiltDocuments[t] || RefinementDocuments[t] || recordDocuments[t] {
			continue
		}
		if t == "handover" && get(root, "handover") == nil {
			continue // a specification with no handover has no handover document (ADR-129)
		}
		out = append(out, output{name: t + " document", reads: documentReads[t], document: true})
	}
	codeTargets := map[string][]string{}
	var codeNames []string
	for _, i := range impls {
		for _, p := range pairs(i.Node, "targets") {
			name := p.Key.Value
			if IsDocumentTarget(name) {
				continue
			}
			if _, seen := codeTargets[name]; !seen {
				codeNames = append(codeNames, name)
			}
			reads := strs(p.Value, "reads")
			if len(reads) == 0 {
				reads = []string{"*"}
			}
			codeTargets[name] = append(codeTargets[name], reads...)
		}
	}
	sort.Strings(codeNames)
	if len(codeNames) == 0 {
		return append(out, output{name: "code generation", reads: []string{"*"}})
	}
	for _, name := range codeNames {
		reads := codeTargets[name]
		for _, r := range reads {
			if r == "*" {
				reads = []string{"*"}
				break
			}
		}
		out = append(out, output{name: "code target " + name, reads: reads})
	}
	return out
}

// NextText is what gaps --next prints for one specification.
func NextText(n Next) string {
	var b strings.Builder
	q := n.Question
	if q == nil {
		fmt.Fprintf(&b, "%s (%s) has no open question.\n", n.Title, n.Specification)
		return b.String()
	}
	label := fmt.Sprintf("%s (%s, %s", q.ID, q.Priority, q.Kind)
	if q.Certainty != "" {
		label += ", " + q.Certainty
	}
	label += ")"
	fmt.Fprintf(&b, "Next question in %s (%s), ranked first of %s: %s\n\n", n.Title, n.Specification, countText(n.OpenQuestions, "open question", "open questions"), label)
	fmt.Fprintf(&b, "%s\n\n", q.Question)
	if q.Kind == "material" {
		fmt.Fprintf(&b, "- Needed from: %s\n", q.DecidedBy)
	} else {
		fmt.Fprintf(&b, "- Decided by: %s\n", q.DecidedBy)
	}
	var blocks []string
	for _, bl := range q.Blocks {
		if missing := q.Missing[bl]; len(missing) > 0 {
			bl += fmt.Sprintf(" (%s missing)", joinAnd(missing))
		}
		blocks = append(blocks, bl)
	}
	fmt.Fprintf(&b, "- Blocks: %s\n", strings.Join(blocks, ", "))
	if q.Names != "" {
		fmt.Fprintf(&b, "- Names: %s, the name the source gives for the key left out\n", q.Names)
	}
	if len(q.HoldsUp) > 0 {
		fmt.Fprintf(&b, "- Holds up: %s\n", joinAnd(q.HoldsUp))
	} else {
		fmt.Fprintf(&b, "- Holds up: nothing; no output reads what it blocks\n")
	}
	if len(q.Options) > 0 {
		fmt.Fprintf(&b, "- Options:\n")
		for i, o := range q.Options {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, o)
		}
	} else {
		fmt.Fprintf(&b, "- Options: none written; offer two to five\n")
	}
	if q.Insight != "" {
		fmt.Fprintf(&b, "- Insight: %s\n", q.Insight)
	}
	words := make([]string, len(q.Decide))
	for i, w := range q.Decide {
		words[i] = shellWord(w)
	}
	fmt.Fprintf(&b, "- Record the answer: %s\n", strings.Join(words, " "))
	if q.SourceNote != "" {
		fmt.Fprintf(&b, "- Source: %s\n", q.SourceNote)
	}
	fmt.Fprintf(&b, "\nPut it to %s alone, with one answer recommended and why; specarch help asking.\n", q.DecidedBy)
	return b.String()
}

// shellWord quotes a word for a POSIX shell when it needs it; <answer> is
// written in double quotes, as the place of the answer.
func shellWord(w string) string {
	if w == "<answer>" {
		return `"<answer>"`
	}
	safe := w != ""
	for _, r := range w {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_./:#=+@%", r)) {
			safe = false
			break
		}
	}
	if safe {
		return w
	}
	return "'" + strings.ReplaceAll(w, "'", `'\''`) + "'"
}
