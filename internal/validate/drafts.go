package validate

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// Draft is one test specarch derive writes: a derived case no test covers,
// as the text of its test.yaml.
type Draft struct {
	Name      string   // the test folder's name, the one the validator suggests
	Subject   string   // how a message names the subject: "operation createLoan"
	Text      string   // the test.yaml
	BlockedBy []string // the must or should questions that hold up the subject
	// Collision says why the draft cannot be written under its name: another
	// draft has the same name, or a test of another subject already has it.
	Collision string
	// Renamed says why the name has a suffix, when another subject's name
	// gives the same test name once both are valid names.
	Renamed string
}

// Drafts lists the tests a valid specification implies and has not got:
// the success test of every subject without a golden test, and every
// chosen derived case no test covers, in the order the warnings name them.
// The cases read the identity module among the specifications given.
func Drafts(root *yaml.Node, given Given) []Draft {
	d := newDesign(root).withIdentity(given)
	subjects := d.subjects()
	golden := map[string]bool{}
	covered := map[string]map[string]bool{}
	existing := map[string]string{} // test name to its subject key
	for _, p := range source.Pairs(source.Child(root, "tests")) {
		key := testSubjectKey(p.Value)
		existing[p.Key.Value] = key
		if source.Str(source.Child(p.Value, "scenario")) == "golden" {
			golden[key] = true
		}
		if covered[key] == nil {
			covered[key] = map[string]bool{}
		}
		for _, item := range source.Items(source.Child(p.Value, "covers")) {
			covered[key][item.Value] = true
		}
	}
	blocking := blockingQuestions(root)
	var out []Draft
	var keys []string // the subject key of each draft
	for _, s := range subjects {
		whole := s.kind == "requirement" || s.kind == "flow"
		blocked := blockedBy(blocking, s.path)
		if !golden[s.yamlKey] && !whole && !s.closed && s.success.scenario != "" {
			success := s.success
			success.name = ""
			out = append(out, Draft{Name: s.name + "-succeeds", Subject: s.label, BlockedBy: blocked, Renamed: s.renamed,
				Text: d.draftText(root, s, success, "every subject needs a golden path, and this is the success the design gives")})
			keys = append(keys, s.yamlKey)
		}
		seen := map[string]bool{}
		for _, dc := range s.cases {
			if covered[s.yamlKey][dc.name] || seen[dc.name] || !s.chosen(dc.name) {
				continue
			}
			seen[dc.name] = true
			out = append(out, Draft{Name: testName(s, dc), Subject: s.label, BlockedBy: blocked, Renamed: s.renamed, Text: d.draftText(root, s, dc, s.chosenReason(dc))})
			keys = append(keys, s.yamlKey)
		}
	}
	markCollisions(out, keys, existing)
	return out
}

// DeriveWrites says whether a diagnostic is a missing test that specarch
// derive writes, an error in a 0.2 file that derive is run to settle.
func DeriveWrites(d Diagnostic) bool {
	return d.Rule == RuleTestCaseMissing || d.Rule == RuleTestGoldenMissing
}

// markCollisions marks every draft that cannot be written under its name:
// one whose name another draft has too, whichever comes first, and one
// whose name is a test of another subject. Writing one of two drafts of a
// name would leave the other unwritten with no word said.
func markCollisions(drafts []Draft, keys []string, existing map[string]string) {
	byName := map[string][]int{}
	for i, dr := range drafts {
		byName[dr.Name] = append(byName[dr.Name], i)
	}
	for i := range drafts {
		dr := &drafts[i]
		if same := byName[dr.Name]; len(same) > 1 {
			var others []string
			for _, j := range same {
				if j != i {
					others = append(others, drafts[j].Subject)
				}
			}
			dr.Collision = "the draft of " + strings.Join(others, " and ") + " has the same name"
		} else if key, ok := existing[dr.Name]; ok && key != keys[i] {
			dr.Collision = "the test there is about " + key + ", not " + keys[i]
		}
	}
}

// chosenReason says why a chosen case is written.
func (s *subject) chosenReason(dc derivedCase) string {
	switch {
	case strings.HasPrefix(dc.name, "dependency fails ") || strings.HasPrefix(dc.name, "dependency times out ") ||
		strings.HasPrefix(dc.name, "dependency retries exhausted ") || strings.HasPrefix(dc.name, "dependency answers unmapped ") ||
		strings.HasPrefix(dc.name, "dependency rate exceeded "):
		return "a failing dependency is always written"
	case dc.name == "concurrent write":
		return "two writers on one record is a case nobody exercises by hand, so it is always written"
	case s.kind == "process" && (dc.name == "restarted after a crash" || dc.name == "crash loop given up"):
		return "a crash and a crash loop are cases nobody exercises by hand, so they are always written"
	case s.kind == "process" && dc.name == "signal SIGTERM":
		return "every stop and every deploy sends SIGTERM, so the case is written"
	case s.kind == "process" && (strings.HasPrefix(dc.name, "setting invalid ") || strings.HasPrefix(dc.name, "secret missing ")):
		return "a setting missing or wrong is the commonest reason a process does not start, so the case is written"
	case s.kind == "route" && dc.name == "path outside the folder":
		return "a path that climbs out of the folder is a case nobody exercises by hand, so it is written"
	case s.kind == "route" && dc.name == "unhealthy":
		return "a health check that never says unhealthy checks nothing, so the case is written"
	case s.kind == "workflow" && workflowReason(dc.name) != "":
		return workflowReason(dc.name)
	case s.critical || dc.critical:
		return s.label + " is at stake in a requirement that names a harm, so its cases are written"
	}
	return "users get this wrong often, so the case is written"
}

// draftText writes a draft test in the layout of a hand-written test.yaml.
func (d *design) draftText(root *yaml.Node, s *subject, dc derivedCase, why string) string {
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	key := s.yamlKey
	if head, tr, ok := strings.Cut(key, ", transition: "); ok {
		line("%s", head)
		line("transition: %s", tr)
	} else {
		for _, part := range strings.Split(key, ", ") {
			line("%s", part)
		}
	}
	level := "system"
	if s.kind == "requirement" {
		level = "acceptance"
	}
	line("level: %s", level)
	line("scenario: %s", dc.scenario)
	if dc.name != "" {
		line("covers: [%s]", flowScalar(dc.name))
	}
	line("given: %q", dc.given)
	line("when: %q", dc.when)
	line("then: %q", dc.then)
	if v := d.draftVerifies(root, s); len(v) > 0 {
		line("verifies: [%s]", strings.Join(v, ", "))
	}
	line("origin: inferred")
	if s.renamed != "" {
		why += "; " + s.renamed
	}
	line("why: %q", "Written by specarch derive: "+why+". A draft: complete the given, when and then, and remove what does not hold.")
	for _, l := range d.draftData(s, dc).lines() {
		line("%s", l)
	}
	return b.String()
}

// draftVerifies is what a draft test verifies: the requirement itself, or
// what the subject's element satisfies.
func (d *design) draftVerifies(root *yaml.Node, s *subject) []string {
	if s.kind == "requirement" {
		return []string{strings.TrimPrefix(s.yamlKey, "requirement: ")}
	}
	var tokens []string
	for _, t := range strings.Split(strings.TrimPrefix(s.path, "/"), "/") {
		tokens = append(tokens, source.UnescapeToken(t))
	}
	n, ok := source.Resolve(root, tokens)
	if !ok {
		return nil
	}
	var out []string
	for _, item := range source.Items(source.Child(n, "satisfies")) {
		out = append(out, item.Value)
	}
	return out
}

// draftData fills what the design says for certain: the status or exit
// the case expects, and for an operation the call itself where the design
// fixes it: a caller who may or, for a denied case, one who may not;
// valid values of every required parameter and body field, with the one
// the case is about left out or changed; and the records those values
// name, or not the one a not-found case looks for.
func (d *design) draftData(s *subject, dc derivedCase) draftCall {
	exit := func(code string) draftCall { return draftCall{expect: []keyText{{"exit", code}}} }
	switch {
	case s.kind == "command" && dc.name == "":
		return exit("0")
	case (s.kind == "command" || s.kind == "process") && strings.HasPrefix(dc.name, "exit "):
		return exit(strings.TrimPrefix(dc.name, "exit "))
	case s.kind == "route":
		if status := routeCaseStatus(dc.name); status != "" {
			return draftCall{expect: []keyText{{"status", status}}}
		}
		return draftCall{}
	case s.kind != "operation":
		return draftCall{}
	}
	o, ok := d.operations[strings.TrimPrefix(s.yamlKey, "operation: ")]
	if !ok {
		return draftCall{}
	}
	return d.operationCall(d.operationDraft(o), dc)
}

// blockingQuestions lists the must and should questions with what they
// block.
func blockingQuestions(root *yaml.Node) map[string][]spec.Block {
	out := map[string][]spec.Block{}
	for _, q := range source.Pairs(source.Child(root, "questions")) {
		if p := source.Str(source.Child(q.Value, "priority")); p != "must" && p != "should" {
			continue
		}
		for _, item := range source.Items(source.Child(q.Value, "blocks")) {
			if b, ok := spec.ParseBlock(item.Value); ok {
				out[q.Key.Value] = append(out[q.Key.Value], b)
			}
		}
	}
	return out
}

// blockedBy lists the questions that hold up the element at a pointer: one
// that blocks its stage, its section, or the element or a part of it.
func blockedBy(qs map[string][]spec.Block, ptr string) []string {
	tokens := strings.Split(strings.TrimPrefix(ptr, "/"), "/")
	section := source.UnescapeToken(tokens[0])
	var out []string
	for id, blocks := range qs {
		for _, b := range blocks {
			hit := false
			switch {
			case b.IsPointer():
				e := b.Holds()
				hit = ptr == e || strings.HasPrefix(ptr, e+"/") || strings.HasPrefix(e, ptr+"/")
			case b.Section != "":
				hit = b.Section == section
			default:
				hit = spec.Sections[section] == b.Stage
			}
			if hit {
				out = append(out, id)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}
