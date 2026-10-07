package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// subjectKey identifies a subject the way a test names it.
func subjectKeyOf(s *subject) string {
	return s.yamlKey
}

// testSubjectKey reads the subject a test names, in the same form as
// subject.yamlKey, or "" when the test names none.
func testSubjectKey(t *yaml.Node) string {
	if v := source.Str(source.Child(t, "operation")); v != "" {
		return "operation: " + v
	}
	if v := source.Str(source.Child(t, "command")); v != "" {
		return "command: " + v
	}
	if v := source.Str(source.Child(t, "page")); v != "" {
		return "page: " + v
	}
	ent := source.Str(source.Child(t, "entity"))
	if ent == "" {
		return ""
	}
	if c := source.Str(source.Child(t, "constraint")); c != "" {
		return "entity: " + ent + ", constraint: " + c
	}
	if tr := source.Child(t, "transition"); tr != nil {
		return fmt.Sprintf("entity: %s, transition: { from: %s, to: %s }", ent, source.Str(source.Child(tr, "from")), source.Str(source.Child(tr, "to")))
	}
	return ""
}

func (c *checker) checkTests(d *design) {
	subjects := d.subjects()
	byKey := map[string]*subject{}
	for _, s := range subjects {
		byKey[subjectKeyOf(s)] = s
	}
	type coverage struct {
		golden, red bool
		covered     map[string]bool
	}
	cov := map[*subject]*coverage{}
	for _, s := range subjects {
		cov[s] = &coverage{covered: map[string]bool{}}
	}

	for _, p := range source.Pairs(source.Child(d.root, "tests")) {
		name, t := p.Key.Value, p.Value
		base := []string{"tests", name}
		key := testSubjectKey(t)
		if key == "" {
			continue // the schema reports a test without a subject
		}
		s := byKey[key]
		if s == nil {
			c.add(p.Key, source.Pointer(base...), RuleTestSubject, "test %s is about %s, which is not in the specification; name an operationId, command, page, or an entity's constraint or transition that exists", name, strings.ReplaceAll(key, ": ", " "))
			continue
		}
		scenario := source.Str(source.Child(t, "scenario"))
		cv := cov[s]
		if scenario == "golden" {
			cv.golden = true
		} else if scenario == "red" {
			cv.red = true
		}
		known := map[string]derivedCase{}
		for _, dc := range s.cases {
			known[dc.name] = dc
		}
		for i, item := range source.Items(source.Child(t, "covers")) {
			ptr := source.Pointer(append(base, "covers", fmt.Sprint(i))...)
			dc, ok := known[item.Value]
			if !ok {
				c.add(item, ptr, RuleTestCase, "%q is not a case of %s; %s", item.Value, s.label, listCases(s))
				continue
			}
			if scenario != "" && dc.scenario != scenario && source.Child(t, "notApplicable") == nil {
				c.add(item, ptr, RuleTestCase, "%q is a %s case, but this test is %s; move it to a %s test", item.Value, dc.scenario, scenario, dc.scenario)
				continue
			}
			cv.covered[item.Value] = true
		}
	}

	for _, s := range subjects {
		cv := cov[s]
		if !cv.golden {
			c.warn(s.node, s.path, RuleTestGoldenMissing, "%s has no golden scenario; add one under tests, for example %s", s.label,
				skeleton(s, derivedCase{scenario: "golden", given: "...", when: "...", then: "it succeeds"}, s.name+"-succeeds"))
		}
		hasRedCase := false
		seen := map[string]bool{}
		for _, dc := range s.cases {
			if dc.scenario == "red" {
				hasRedCase = true
			}
			if cv.covered[dc.name] || seen[dc.name] {
				continue
			}
			seen[dc.name] = true
			c.warn(s.node, s.path, RuleTestCaseMissing, "%s has no %s scenario for %q; add under tests %s", s.label, dc.scenario, dc.name,
				skeleton(s, dc, testName(s, dc)))
		}
		if !cv.red && !hasRedCase {
			c.warn(s.node, s.path, RuleTestRedMissing, "%s has no red scenario; add one under tests, for example %s", s.label,
				skeleton(s, derivedCase{scenario: "red", given: "...", when: "...", then: "it is refused"}, s.name+"-refused"))
		}
	}
}

func listCases(s *subject) string {
	if len(s.cases) == 0 {
		return "it has no derived cases, so leave covers out"
	}
	names := make([]string, 0, len(s.cases))
	seen := map[string]bool{}
	for _, dc := range s.cases {
		if !seen[dc.name] {
			seen[dc.name] = true
			names = append(names, dc.name)
		}
	}
	return "its cases are: " + strings.Join(names, ", ")
}

// skeleton is a test in flow style the author can paste under tests.
func skeleton(s *subject, dc derivedCase, name string) string {
	covers := ""
	if dc.name != "" {
		covers = ", covers: [" + dc.name + "]"
	}
	return fmt.Sprintf("%s: { %s, scenario: %s%s, given: %q, when: %q, then: %q }", name, s.yamlKey, dc.scenario, covers, dc.given, dc.when, dc.then)
}

// testName suggests a test name: the subject, then the case, without saying
// the subject's own name twice.
func testName(s *subject, dc derivedCase) string {
	c := dc.name
	if s.raw != "" {
		c = strings.TrimSpace(strings.Replace(c, s.raw, "", 1))
	}
	c = strings.ToLower(strings.NewReplacer(" ", "-", "_", "-", ".", "-").Replace(kebab(c)))
	for strings.Contains(c, "--") {
		c = strings.ReplaceAll(c, "--", "-")
	}
	return s.name + "-" + strings.Trim(c, "-")
}
