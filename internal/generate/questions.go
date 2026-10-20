package generate

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// DocumentTargets are the document targets of the design, in the order
// they are listed; BuiltDocuments says which this program has.
var (
	DocumentTargets = []string{"techspec", "requirements", "testplan", "traceability", "deployment", "commissioning", "handover", "questions", "problems", "comparison", "changes", "releases", "html", "manual", "operations"}
	BuiltDocuments  = map[string]bool{"techspec": true, "requirements": true, "testplan": true, "traceability": true, "deployment": true, "commissioning": true, "handover": true, "questions": true, "problems": true, "comparison": true, "changes": true, "releases": true, "html": true, "manual": true, "operations": true}
)

// documentReads says which sections each document reads, so that a
// question blocking one of them makes the document a draft. "*" is every
// section.
var documentReads = map[string][]string{
	"techspec":      {"*"},
	"requirements":  {"stakeholders", "needs", "requirements", "glossary", "assumptions", "constraints"},
	"testplan":      {"tests", "requirements"},
	"traceability":  {"needs", "requirements", "tests", "checks", "monitors", "enums", "entities", "views", "permissions", "roles", "separationOfDuties", "session", "middlewares", "handshakes", "paths", "commands", "channels", "dependencies", "jobs", "workflows", "errors", "pages", "menus", "algorithms", "decisions"},
	"deployment":    {"environments", "configuration", "release", "rollback", "migrations", "handover", "monitors", "backup", "restore"},
	"commissioning": {"checks", "signoff"},
	"handover":      {"environments", "configuration", "release", "rollback", "migrations", "handover", "checks", "signoff", "monitors", "backup", "restore", "tests"},
	"html":          {"*"},
	"manual":        {"flows", "menus", "pages", "roles", "permissions", "errors"},
	"operations":    {"environments", "monitors", "jobs", "channels", "dependencies", "paths", "configuration", "backup", "restore", "rollback"},
}

// RefinementDocuments are the documents about the refinement itself, read
// while questions are open: never a draft, never in the outputs table, and
// not among the documents an approval is of (ADR-157).
var RefinementDocuments = map[string]bool{"questions": true, "problems": true, "comparison": true}

// IsDocumentTarget reports whether a target name is a document's.
func IsDocumentTarget(t string) bool {
	for _, d := range DocumentTargets {
		if d == t {
			return true
		}
	}
	return false
}

// State is what the questions document needs beyond the specification:
// the keys the open questions cover, and where the approval stands.
type State struct {
	Missing  map[string][]string // element pointer -> the keys missing there, as the validator found them
	Approval string              // one sentence: approved on, not approved because
	Approved bool
	LeftOut  []LeftOut // the derived cases of rank other no test covers, for the test plan
	// Records are the record files beside the specification, parsed, in
	// path order, and RecordsRel their folder as the document names it.
	Records    []*yaml.Node
	RecordsRel string
	// StatePaths are, per entity with a state machine, its paths from an
	// initial to a terminal state, for the test plan.
	StatePaths map[string][]string
	// Places are, per block of a question that names what the source
	// gives, the file and line of the element whose key it leaves out.
	Places map[string]string
	// Marks are the errors and warnings of the specification, which the
	// documents mark at their elements.
	Marks   []Mark
	placing *placing
	// Site is what the html target shows that other generators make, and
	// the problems it lists.
	Site *SiteParts
	// References are the references into other specifications that the
	// run could not resolve, in document order.
	References []Reference
	// Withheld is, for a document for everyone, the internal elements it
	// left out (ADR-119); nil for a document that holds every element.
	Withheld *Withheld
	// Chains are the delegation chains that reach the operations with a
	// via, as the induction through calls finds them across the modules
	// given in the run; nil when none is given (ADR-111).
	Chains []Chain
	// Grants are the roles and clients of the identity module that reach
	// this specification's permissions, columns of its matrix (ADR-107).
	Grants []RoleColumn
}

// Chain is one way a call reaches an operation, from the edge on.
type Chain struct {
	Operation string
	Hops      []Hop
}

// Hop is one operation of a chain: its module, the token it accepts, and
// the proof its module presents to the next hop.
type Hop struct {
	Module, Operation, Token, Proof string
}

// RoleColumn is one holder of the specification's permissions held in
// another module, with its cell per permission.
type RoleColumn struct {
	Name  string
	Cells map[string]string
}

// Reference is one reference into another specification that waits: the
// specification is not given, or the element is blocked there.
type Reference struct {
	Path, Text, Specification, Version, Waits string
}

// LeftOut is one derived case the test plan lists as left out.
type LeftOut struct {
	Subject, Case, Scenario, Reason string
}

// question is one open question, parsed.
type question struct {
	id       string
	node     *yaml.Node
	priority string
	blocks   []spec.Block
	stage    string
}

func questionsOf(root *yaml.Node) []question {
	var out []question
	for _, p := range pairs(root, "questions") {
		q := question{id: p.Key.Value, node: p.Value, priority: str(p.Value, "priority")}
		for _, item := range items(p.Value, "blocks") {
			if b, ok := spec.ParseBlock(item.Value); ok {
				q.blocks = append(q.blocks, b)
				if q.stage == "" {
					q.stage = b.Stage
				}
			}
		}
		out = append(out, q)
	}
	return out
}

// holds reports whether the question holds up work that reads the given
// sections ("*" for all). Only must and should questions hold anything.
func (q question) holds(sections []string) bool {
	if q.priority != "must" && q.priority != "should" {
		return false
	}
	all := len(sections) == 1 && sections[0] == "*"
	for _, b := range q.blocks {
		if all {
			return true
		}
		for _, s := range b.Sections() {
			for _, want := range sections {
				if s == want {
					return true
				}
			}
		}
	}
	return false
}

// about reports whether the question blocks the element at a pointer, or a
// key of it.
func (q question) about(ptr string) bool {
	for _, b := range q.blocks {
		if e := b.Element(); e != "" && e == ptr {
			return true
		}
	}
	return false
}

// known is a key's value or, while a question holds the key open, what
// the key is and the question that holds it, with the name the source
// gives when the question names one, so every output marks the gap where
// it is. tokens point at the element the key is of.
func (d *doc) known(n *yaml.Node, key, what string, tokens ...string) string {
	if v := str(n, key); v != "" {
		return v
	}
	ptr := source.Pointer(append(append([]string{}, tokens...), key)...)
	for _, q := range d.questions {
		for _, b := range q.blocks {
			if !b.IsPointer() || source.Pointer(b.Tokens...) != ptr {
				continue
			}
			if names := str(q.node, "names"); names != "" {
				return fmt.Sprintf("%s (open in %s, which names %s)", what, q.id, names)
			}
			return fmt.Sprintf("%s (open in %s)", what, q.id)
		}
	}
	return what + " (not given)"
}

func (q question) label() string {
	if c := str(q.node, "certainty"); c != "" {
		return fmt.Sprintf("%s (%s, %s, %s)", q.id, q.priority, str(q.node, "kind"), c)
	}
	return fmt.Sprintf("%s (%s, %s)", q.id, q.priority, str(q.node, "kind"))
}

// holding lists the IDs of the questions that hold up work reading the
// sections.
func holding(qs []question, sections []string) []string {
	var ids []string
	for _, q := range qs {
		if q.holds(sections) {
			ids = append(ids, q.id)
		}
	}
	return ids
}

// draftNotice writes, under a document's summary, that open questions
// concern what it covers, and then the problems notice.
func (d *doc) draftNotice(target string) {
	defer d.problemsNotice(target)
	d.para(d.withheld.audienceNotice())
	ids := holding(d.questions, documentReads[target])
	if len(ids) == 0 {
		return
	}
	verb := "concern"
	if len(ids) == 1 {
		verb = "concerns"
	}
	d.para(fmt.Sprintf("**Draft:** %s %s this document (%s); see the open questions document, or run specarch gaps.", countText(len(ids), "open question", "open questions"), verb, strings.Join(ids, ", ")))
}

// pointers maps every mapping node of the root to its pointer, so that an
// element's Origin and Open paragraphs can be found from its node.
func pointers(root *yaml.Node) map[*yaml.Node]string {
	out := map[*yaml.Node]string{}
	var walk func(n *yaml.Node, path []string)
	walk = func(n *yaml.Node, path []string) {
		n = source.Deref(n)
		if n == nil {
			return
		}
		switch n.Kind {
		case yaml.MappingNode:
			out[n] = source.Pointer(path...)
			for _, p := range source.Pairs(n) {
				walk(p.Value, append(append([]string{}, path...), p.Key.Value))
			}
		case yaml.SequenceNode:
			for i, item := range n.Content {
				walk(item, append(append([]string{}, path...), fmt.Sprint(i)))
			}
		}
	}
	walk(root, nil)
	return out
}

// origin is the Origin line of an element, or "".
func (d *doc) origin(n *yaml.Node) string {
	switch str(n, "origin") {
	case "stated":
		var in []string
		for _, c := range items(n, "cites") {
			src := get(d.sources, str(c, "source"))
			title := strings.TrimSpace(str(src, "title"))
			if title == "" {
				title = str(c, "source")
			}
			if cl := str(c, "clause"); cl != "" {
				title += ", clause " + cl
			}
			in = append(in, title)
		}
		if len(in) == 0 {
			return "**Origin:** stated."
		}
		return "**Origin:** stated in " + strings.Join(in, "; ") + "."
	case "inferred":
		return "**Origin:** inferred."
	case "decided":
		id := str(n, "decidedIn")
		if title := str(get(d.decisions, id), "title"); title != "" {
			return fmt.Sprintf("**Origin:** decided in %s, %s.", id, title)
		}
		return fmt.Sprintf("**Origin:** decided in %s.", id)
	}
	return ""
}

// open writes the open questions about an element.
func (d *doc) open(label string, n *yaml.Node) {
	ptr, ok := d.pointerOf[source.Deref(n)]
	if !ok {
		return
	}
	on := ""
	if label != "" {
		on = " on " + label
	}
	for _, q := range d.questions {
		if q.about(ptr) {
			d.openParagraph(q, on)
		}
	}
}

// openParagraph writes one open question: its ID, priority and kind, what
// it is on, its text and who decides it.
func (d *doc) openParagraph(q question, on string) {
	d.para(fmt.Sprintf("**Open question %s%s:** %s Decided by %s.", q.label(), on, oneParagraph(str(q.node, "question")), str(q.node, "decidedBy")))
}

// Questions writes the open questions document, which specarch gaps also
// prints: the counts, the questions by stage, and which outputs are ready,
// drafts or waiting.
func Questions(root *yaml.Node, relRoot string, impls []Implementation, state *State) string {
	d := newDoc("questions", root, relRoot, impls, state)
	info := get(root, "info")
	qs := d.questions
	must, should, could := 0, 0, 0
	for _, q := range qs {
		switch q.priority {
		case "must":
			must++
		case "should":
			should++
		case "could":
			could++
		}
	}
	d.line("# %s: open questions", str(info, "title"))
	d.blank()
	if len(qs) == 0 {
		d.para(fmt.Sprintf("Version %s of the specification has no open question: nothing it says waits on a decision or on material from a stakeholder.", str(info, "version")))
	} else {
		d.para(fmt.Sprintf("Version %s of the specification: %s (%d must, %d should, %d could). A must question holds up everything that reads what it blocks; a should question holds up code generation and approval until the stakeholder confirms what was inferred; a could question holds up nothing. Each is answered by the stakeholder named, with a decision record that names the question under answers; the question is then removed.",
			str(info, "version"), countText(len(qs), "open question", "open questions"), must, should, could))
	}
	if len(qs) > 0 {
		d.para(takers(qs))
	}
	d.para(d.withheld.audienceNotice())
	if str(info, "tracksOrigin") == "true" {
		stated, inferred, decided, none := originCounts(root)
		d.para(fmt.Sprintf("Elements by origin: %d stated, %d inferred, %d decided, %d without origin.", stated, inferred, decided, none))
	}
	d.errorsSection()

	opened := openedBy(root)
	write := func(q question) {
		d.heading(3, q.label())
		d.para(oneParagraph(str(q.node, "question")))
		if q.material() {
			d.line("- Needed from: %s", str(q.node, "decidedBy"))
		} else {
			d.line("- Decided by: %s", str(q.node, "decidedBy"))
		}
		var blocks []string
		for _, item := range items(q.node, "blocks") {
			text := item.Value
			if b, ok := spec.ParseBlock(text); ok && b.IsPointer() && state != nil {
				// The keys the validator found missing at the blocked
				// element, at any depth; a pointer to a key that is
				// not there names it already.
				if missing := state.Missing[source.Pointer(b.Tokens...)]; len(missing) > 0 {
					text += fmt.Sprintf(" (%s missing)", joinAnd(missing))
				}
			}
			blocks = append(blocks, text)
		}
		d.line("- Blocks: %s", strings.Join(blocks, ", "))
		if names := str(q.node, "names"); names != "" {
			where := ""
			if items := items(q.node, "blocks"); len(items) == 1 && state != nil && state.Places[items[0].Value] != "" {
				where = " at " + state.Places[items[0].Value]
			}
			text := fmt.Sprintf("- Names: %s, the name the source gives for the key left out%s; it is written there once the specification declares an element of that name", names, where)
			d.line("%s", text)
		}
		if opts := strs(q.node, "options"); len(opts) > 0 {
			d.line("- Options:")
			for i, o := range opts {
				d.line("  %d. %s", i+1, oneParagraph(o))
			}
		}
		if chain := d.chain(q.id, opened); chain != "" {
			d.line("- Opened by: %s", chain)
		}
		d.blank()
		d.explain(q.node)
	}
	// The decisions to take by stage, then the material to provide, which
	// is answered by sending it, not by deciding (ADR-120).
	for _, stage := range spec.Stages {
		var inStage []question
		for _, q := range qs {
			if q.stage == stage && !q.material() {
				inStage = append(inStage, q)
			}
		}
		if len(inStage) == 0 {
			continue
		}
		sortQuestions(inStage)
		d.section(strings.ToUpper(stage[:1]) + stage[1:])
		for _, q := range inStage {
			write(q)
		}
	}
	var material []question
	for _, q := range qs {
		if q.material() {
			material = append(material, q)
		}
	}
	if len(material) > 0 {
		sortQuestions(material)
		d.section("Material to provide")
		d.para("What a stakeholder must send rather than decide: a document, a file, a screenshot or a record. Each is answered by providing it, declared as a source the specification cites, and a decision record that names the question under answers.")
		for _, q := range material {
			write(q)
		}
	}

	if state != nil && len(state.References) > 0 {
		d.section("References to other specifications")
		says := "1 reference names an element of another specification and is not resolved in this run"
		if n := len(state.References); n != 1 {
			says = fmt.Sprintf("%d references name elements of other specifications and are not resolved in this run", n)
		}
		d.para(says + ": the specification named is not given with this one, or the element is blocked there by a must question. Give the other specification in the same run to check them; until then they are gaps, not errors.")
		d.line("| At | Reference | Specification | Waits on |")
		d.line("|---|---|---|---|")
		for _, r := range state.References {
			d.line("| %s | %s | %s %s | %s |", cell(r.Path), cell(r.Text), r.Specification, r.Version, cell(r.Waits))
		}
		d.blank()
	}

	d.coverage(root, impls)

	d.section("Outputs")
	d.para("What can be made from the specification now. A document is a draft while a must or should question blocks what it reads, or an error is in it; code generation waits for those questions, for every error to be fixed, and for the approval.")
	d.line("| Output | State | Waits on |")
	d.line("|---|---|---|")
	for _, t := range DocumentTargets {
		if !BuiltDocuments[t] || RefinementDocuments[t] || recordDocuments[t] {
			continue
		}
		if t == "handover" && get(root, "handover") == nil {
			continue // a specification with no handover has no handover document (ADR-129)
		}
		var waits []string
		if ids := holding(qs, documentReads[t]); len(ids) > 0 {
			waits = append(waits, byStage(qs, ids))
		}
		if errors := d.errorsConcerning(documentReads[t]); errors > 0 {
			waits = append(waits, countText(errors, "error", "errors"))
		}
		if len(waits) == 0 {
			d.line("| %s document | ready | |", t)
		} else {
			d.line("| %s document | draft | %s |", t, strings.Join(waits, ", "))
		}
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
	approval := "the approval"
	if state != nil {
		approval = state.Approval
	}
	if len(codeNames) == 0 {
		codeNames = []string{"code generation"}
		codeTargets["code generation"] = []string{"*"}
	}
	for _, name := range codeNames {
		reads := codeTargets[name]
		for _, r := range reads {
			if r == "*" {
				reads = []string{"*"}
				break
			}
		}
		ids := holding(qs, reads)
		var waits []string
		if errors := d.errorCount(); errors > 0 {
			waits = append(waits, countText(errors, "error", "errors"))
		}
		if len(ids) > 0 {
			waits = append(waits, byStage(qs, ids))
		}
		if state == nil || !state.Approved {
			waits = append(waits, approval)
		}
		label := name
		if name != "code generation" {
			label = "code target " + name
		}
		if len(waits) == 0 {
			d.line("| %s | ready | |", label)
		} else {
			d.line("| %s | waits | %s |", label, cell(strings.Join(waits, "; ")))
		}
	}
	d.blank()
	d.sourcesIndex("Sources")
	return d.String()
}

// material reports whether the question asks for something to read rather
// than a decision.
func (q question) material() bool { return str(q.node, "kind") == "material" }

// sortQuestions orders questions by priority, then by ID.
func sortQuestions(qs []question) {
	sort.SliceStable(qs, func(i, j int) bool {
		pi, pj := priorityRank(qs[i].priority), priorityRank(qs[j].priority)
		if pi != pj {
			return pi < pj
		}
		return idLess(qs[i].id, qs[j].id)
	})
}

// takers says how many decisions each stakeholder must take and how much
// material each must provide, a material question counted only as the
// second.
func takers(qs []question) string {
	decide, provide := map[string]int{}, map[string]int{}
	for _, q := range qs {
		if q.material() {
			provide[str(q.node, "decidedBy")]++
		} else {
			decide[str(q.node, "decidedBy")]++
		}
	}
	list := func(counts map[string]int) string {
		var names []string
		for n := range counts {
			names = append(names, n)
		}
		sort.Strings(names)
		var parts []string
		for _, n := range names {
			parts = append(parts, fmt.Sprintf("%s %d", n, counts[n]))
		}
		return strings.Join(parts, ", ")
	}
	var says []string
	if len(decide) > 0 {
		says = append(says, "Decisions to take: "+list(decide)+".")
	}
	if len(provide) > 0 {
		says = append(says, "Material to provide: "+list(provide)+".")
	}
	return strings.Join(says, " ")
}

// shownPerStage is how many identifiers a cell of the outputs table names
// for one stage; the rest are counted, and listed in the stage's section.
const shownPerStage = 3

// byStage writes the questions holding an output up per stage, in the
// order of the stages: how many, and the first of them by priority.
func byStage(qs []question, ids []string) string {
	holds := map[string]bool{}
	for _, id := range ids {
		holds[id] = true
	}
	var parts []string
	for _, stage := range spec.Stages {
		var in []question
		for _, q := range qs {
			if holds[q.id] && q.stage == stage {
				in = append(in, q)
			}
		}
		if len(in) == 0 {
			continue
		}
		sortQuestions(in)
		var shown []string
		for i, q := range in {
			if i == shownPerStage {
				shown = append(shown, fmt.Sprintf("%d more", len(in)-shownPerStage))
				break
			}
			shown = append(shown, q.id)
		}
		parts = append(parts, fmt.Sprintf("%d in %s (%s)", len(in), stage, joinAnd(shown)))
	}
	return strings.Join(parts, ", ")
}

// openedBy maps each question a decision opens to the decisions that
// opened it, in the order of the decisions.
func openedBy(root *yaml.Node) map[string][]string {
	out := map[string][]string{}
	for _, p := range pairs(root, "decisions") {
		for _, o := range strs(p.Value, "opens") {
			out[o] = append(out[o], p.Key.Value)
		}
	}
	return out
}

// chain says which decisions opened a question and what each answered, and
// so on back for every question answered that a decision opened in turn;
// "" when no decision opened it.
func (d *doc) chain(id string, opened map[string][]string) string {
	var parts []string
	seen := map[string]bool{id: true}
	queue := []string{id}
	for len(queue) > 0 {
		q := queue[0]
		queue = queue[1:]
		decs := opened[q]
		if len(decs) == 0 {
			continue
		}
		var by []string
		for _, dec := range decs {
			text := dec
			if title := str(get(d.decisions, dec), "title"); title != "" {
				text += " (" + title + ")"
			}
			if answers := strs(get(d.decisions, dec), "answers"); len(answers) > 0 {
				text += ", which answered " + joinAnd(answers)
				for _, a := range answers {
					if !seen[a] {
						seen[a] = true
						queue = append(queue, a)
					}
				}
			}
			by = append(by, text)
		}
		if q == id {
			parts = append(parts, joinAnd(by))
		} else {
			parts = append(parts, q+" was opened by "+joinAnd(by))
		}
	}
	return strings.Join(parts, "; ")
}

func priorityRank(p string) int {
	switch p {
	case "must":
		return 0
	case "should":
		return 1
	}
	return 2
}

// idLess orders IDs by prefix, then by number when both have one.
func idLess(a, b string) bool {
	pa, na, _ := strings.Cut(a, "-")
	pb, nb, _ := strings.Cut(b, "-")
	if pa != pb {
		return pa < pb
	}
	var x, y int
	if _, err := fmt.Sscanf(na, "%d", &x); err == nil {
		if _, err := fmt.Sscanf(nb, "%d", &y); err == nil {
			return x < y
		}
	}
	return na < nb
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

// originCounts counts the named elements of every section by origin.
func originCounts(root *yaml.Node) (stated, inferred, decided, none int) {
	count := func(n *yaml.Node) {
		switch str(n, "origin") {
		case "stated":
			stated++
		case "inferred":
			inferred++
		case "decided":
			decided++
		default:
			none++
		}
	}
	for _, p := range source.Pairs(root) {
		name := p.Key.Value
		if _, isSection := spec.Sections[name]; !isSection {
			continue
		}
		switch name {
		case "release", "rollback", "signoff":
			count(p.Value)
		case "paths":
			for _, o := range operations(root) {
				count(o.node)
			}
		default:
			for _, e := range source.Pairs(p.Value) {
				if source.Deref(e.Value).Kind == yaml.MappingNode {
					count(e.Value)
				}
			}
		}
	}
	return
}

// Holding lists the open must and should questions that block what a code
// target reads: the sections its implementation files declare under
// targets.<name>.reads, or every section when none does.
func Holding(root *yaml.Node, impls []Implementation, target string) []string {
	var reads []string
	for _, i := range impls {
		r := strs(get(get(i.Node, "targets"), target), "reads")
		if len(r) == 0 && get(get(i.Node, "targets"), target) != nil {
			return holding(questionsOf(root), []string{"*"})
		}
		reads = append(reads, r...)
	}
	if len(reads) == 0 {
		reads = []string{"*"}
	}
	return holding(questionsOf(root), reads)
}
