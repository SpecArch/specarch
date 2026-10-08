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
}

// Drafts lists the tests a valid specification implies and has not got:
// the success test of every subject without a golden test, and every
// chosen derived case no test covers, in the order the warnings name them.
func Drafts(root *yaml.Node) []Draft {
	d := newDesign(root)
	subjects := d.subjects()
	golden := map[string]bool{}
	covered := map[string]map[string]bool{}
	for _, p := range source.Pairs(source.Child(root, "tests")) {
		key := testSubjectKey(p.Value)
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
	for _, s := range subjects {
		whole := s.kind == "requirement" || s.kind == "flow"
		blocked := blockedBy(blocking, s.path)
		if !golden[s.yamlKey] && !whole && s.success.scenario != "" {
			success := s.success
			success.name = ""
			out = append(out, Draft{Name: s.name + "-succeeds", Subject: s.label, BlockedBy: blocked,
				Text: d.draftText(root, s, success, "every subject needs a golden path, and this is the success the design gives")})
		}
		seen := map[string]bool{}
		for _, dc := range s.cases {
			if covered[s.yamlKey][dc.name] || seen[dc.name] || !s.chosen(dc.name) {
				continue
			}
			seen[dc.name] = true
			out = append(out, Draft{Name: testName(s, dc), Subject: s.label, BlockedBy: blocked, Text: d.draftText(root, s, dc, s.chosenReason(dc))})
		}
	}
	return out
}

// chosenReason says why a chosen case is written.
func (s *subject) chosenReason(dc derivedCase) string {
	switch {
	case strings.HasPrefix(dc.name, "dependency fails ") || strings.HasPrefix(dc.name, "dependency times out "):
		return "a failing dependency is always written"
	case dc.name == "concurrent write":
		return "two writers on one record is a case nobody exercises by hand, so it is always written"
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
	line("why: %q", "Written by specarch derive: "+why+". A draft: complete the given, when and then, and remove what does not hold.")
	if caller, expect := d.draftData(root, s, dc); caller != "" || expect != "" {
		if caller != "" {
			line("fixture: { caller: %s }", caller)
		}
		if expect != "" {
			line("expect: { %s }", expect)
		}
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
// the case expects, and for a denied case a role without the permission.
func (d *design) draftData(root *yaml.Node, s *subject, dc derivedCase) (caller, expect string) {
	var op *yaml.Node
	if s.kind == "operation" {
		if o, ok := d.operations[strings.TrimPrefix(s.yamlKey, "operation: ")]; ok {
			op = o.node
		}
	}
	has := func(status string) bool { return source.Child(source.Child(op, "responses"), status) != nil }
	switch {
	case s.kind == "command" && dc.name == "":
		return "", "exit: 0"
	case s.kind == "command" && strings.HasPrefix(dc.name, "exit "):
		return "", "exit: " + strings.TrimPrefix(dc.name, "exit ")
	case op == nil:
		return "", ""
	case dc.name == "":
		for _, r := range source.Pairs(source.Child(op, "responses")) {
			if strings.HasPrefix(r.Key.Value, "2") {
				return "", "status: " + r.Key.Value
			}
		}
	case strings.HasPrefix(dc.name, "response "):
		return "", "status: " + strings.TrimPrefix(dc.name, "response ")
	case strings.HasPrefix(dc.name, "repeated with the same "):
		for _, r := range source.Pairs(source.Child(op, "responses")) {
			if strings.HasPrefix(r.Key.Value, "2") {
				return "", "status: " + r.Key.Value
			}
		}
	case dc.name == "denied with expired session" && has("401"):
		return "", "status: 401"
	case strings.HasPrefix(dc.name, "not found ") && has("404"):
		return "", "status: 404"
	case strings.HasPrefix(dc.name, "denied without "):
		perm := strings.TrimPrefix(dc.name, "denied without ")
		var roles []string
		for name, role := range d.roles {
			holds := false
			for _, p := range source.Items(source.Child(role, "permissions")) {
				holds = holds || p.Value == perm
			}
			if !holds {
				roles = append(roles, name)
			}
		}
		sort.Strings(roles)
		if len(roles) > 0 {
			caller = roles[0]
		}
		if has("403") {
			expect = "status: 403"
		}
		return caller, expect
	}
	return "", ""
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
				e := b.Element()
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
