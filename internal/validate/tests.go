package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/metamodel"
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
	if v := source.Str(source.Child(t, "job")); v != "" {
		return "job: " + v
	}
	if v := source.Str(source.Child(t, "gateway")); v != "" {
		return "gateway: " + v
	}
	if v := source.Str(source.Child(t, "flow")); v != "" {
		return "flow: " + v
	}
	if v := source.Str(source.Child(t, "workflow")); v != "" {
		return "workflow: " + v
	}
	if v := source.Str(source.Child(t, "requirement")); v != "" {
		return "requirement: " + v
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
	return "entity: " + ent // the entity's state machine
}

func (c *checker) checkTests(d *design) {
	subjects := d.withIdentity(c.given).subjects()
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
		if s == nil && strings.HasPrefix(key, "entity: ") && !strings.Contains(key, ", ") && d.entities[strings.TrimPrefix(key, "entity: ")] != nil {
			c.add(p.Key, source.Pointer(base...), RuleTestSubject, "test %s is about the state machine of %s, which has no path from an initial to a terminal state; give it transitions, or name one of its constraints or transitions", name, strings.TrimPrefix(key, "entity: "))
			continue
		}
		if s == nil {
			c.add(p.Key, source.Pointer(base...), RuleTestSubject, "test %s is about %s, which is not in the specification; name an operationId, command, page, job, gateway, flow, workflow, requirement with acceptance criteria, entity with a state machine, or an entity's constraint or transition that exists", name, strings.ReplaceAll(key, ": ", " "))
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
			if !ok && d.identityCaseWaits(item.Value) {
				continue
			}
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

	// In a 0.2 or later file a test specarch derive writes and nobody has
	// is an error, except for a subject an open must or should question
	// holds up, whose test derive leaves out too (ADR-055), and in a
	// specification on the lean profile, where it is a warning (ADR-210).
	strict := metamodel.AtLeast(d.reads(), 0, 2) && !d.lean()
	blocking := blockingQuestions(d.root)
	for _, s := range subjects {
		cv := cov[s]
		missing := c.warn
		if strict && len(blockedBy(blocking, s.path)) == 0 {
			missing = c.add
		}
		// A requirement's or a state machine's cases are all golden and
		// each is asked for on its own, so neither needs a scenario of each
		// kind.
		whole := s.kind == "requirement" || s.kind == "flow"
		if !cv.golden && !whole && !s.closed {
			success := s.success
			report := c.warn
			if success.scenario != "" {
				report = missing // derive writes the success the design gives
			} else {
				success = derivedCase{scenario: "golden", given: "...", when: "...", then: "it succeeds"}
			}
			success.name = "" // any golden test of the subject is its success case
			report(s.node, s.path, RuleTestGoldenMissing, "%s has no golden scenario; add one under tests, for example %s", s.label,
				skeleton(s, success, s.name+"-succeeds"))
		}
		hasRedCase := false
		seen := map[string]bool{}
		for _, dc := range s.cases {
			if dc.scenario == "red" {
				hasRedCase = true
			}
			if cv.covered[dc.name] || seen[dc.name] || !s.chosen(dc.name) {
				continue
			}
			seen[dc.name] = true
			missing(s.node, s.path, RuleTestCaseMissing, "%s has no %s scenario for %q; add under tests %s", s.label, dc.scenario, dc.name,
				skeleton(s, dc, testName(s, dc)))
		}
		if !cv.red && !hasRedCase && !whole {
			c.warn(s.node, s.path, RuleTestRedMissing, "%s has no red scenario; add one under tests, for example %s", s.label,
				skeleton(s, derivedCase{scenario: "red", given: "...", when: "...", then: "it is refused"}, s.name+"-refused"))
		}
	}
}

// checkMetaModelVersion warns once in a file written against an earlier
// meta-model, 0.1, 0.2 or 0.3, each read beside 1.0.0 (ADR-186), and
// reports a version line this program does not read as an error at the
// line; the file is then read as 1.0.0, so every other problem is still
// listed.
func (c *checker) checkMetaModelVersion(d *design) {
	v := source.Child(d.root, "specarch")
	upgrade := "run specarch upgrade --dry-run on the specification to see what moving it to " + metamodel.Current + " changes, and specarch upgrade to move it"
	switch source.Str(v) {
	case "0.1":
		c.warn(v, "/specarch", RuleMetaModelVersion, "the specification is written against meta-model 0.1, an earlier version read beside %s with this warning, and its missing tests are warnings; run specarch derive to write the tests 0.2 and later ask for, then %s", metamodel.Current, upgrade)
	case "0.2":
		c.warn(v, "/specarch", RuleMetaModelVersion, "the specification is written against meta-model 0.2, an earlier version read beside %s with this warning; 0.3 and later reserve the name caller for the caller record, so rename any field, property or input called caller, then %s", metamodel.Current, upgrade)
	case "0.3":
		c.warn(v, "/specarch", RuleMetaModelVersion, "the specification is written against meta-model 0.3, an earlier version read beside %s with this warning; %s", metamodel.Current, upgrade)
	default:
		if v != nil && v.Tag == "!!str" {
			if problem := metamodel.Design.Problem(v.Value); problem != "" {
				c.add(v, "/specarch", RuleMetaModelVersion, "the meta-model version %s", problem)
			}
		}
	}
}

// lean reports whether the specification is on the lean profile, which a
// file on 1.0 sets under info.profile; an earlier version has no profile.
func (d *design) lean() bool {
	return !metamodel.Design.IsEarlier(d.metaModelVersion()) &&
		source.Str(source.Child(source.Child(d.root, "info"), "profile")) == "lean"
}

// checkProfile warns once at info.profile in a specification on the lean
// profile, naming the demands it makes warnings and how to tighten it, so a
// missing test printed as a warning says why it is not an error (ADR-210).
func (c *checker) checkProfile(d *design) {
	if !d.lean() {
		return
	}
	c.warn(source.Child(source.Child(d.root, "info"), "profile"), "/info/profile", RuleProfile,
		"the specification is on the lean profile, so a test specarch derive would write and nobody has is a warning, not an error (test_golden_missing, test_case_missing); every other rule is as on full. Write profile: full, or remove the key, to make them errors, and run specarch derive to write the tests")
}

// checkDeprecated warns at each key a file on 1.0 gives under the spelling
// 1.0.0 deprecated, which is read as its replacement until 2.0.0, and
// reports a key given under both spellings, since one thing is said once.
// A file on an earlier version uses the old spelling, which its own
// schema knows, and is told so by the version warning.
func (c *checker) checkDeprecated(d *design) {
	if metamodel.Design.IsEarlier(d.metaModelVersion()) {
		return
	}
	check := func(obj *yaml.Node, base []string, key, old, what string) {
		n := source.Key(obj, old)
		if n == nil {
			return
		}
		ptr := source.Pointer(append(base, old)...)
		if source.Child(obj, key) != nil {
			c.add(n, ptr, RuleMetaModelDeprecated, "%s gives both %s and %s, the spelling of %s before meta-model 1.0.0; move what %s holds into %s and remove %s", what, key, old, key, old, key, old)
			return
		}
		c.warn(n, ptr, RuleMetaModelDeprecated, "%s is the spelling of %s before meta-model 1.0.0, deprecated and read as %s until 2.0.0; run specarch upgrade on the specification, or rename it to %s here", old, key, key, key)
	}
	for _, r := range source.Pairs(source.Child(d.root, "roles")) {
		check(r.Value, []string{"roles", r.Key.Value}, metamodel.RowPoliciesKey, metamodel.RowPoliciesOldKey, "role "+r.Key.Value)
	}
	if h := source.Child(d.root, "handover"); h != nil {
		check(h, []string{"handover"}, metamodel.SourceDeliveryKey, metamodel.SourceDeliveryOldKey, "the handover")
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
		covers = ", covers: [" + flowScalar(dc.name) + "]"
	}
	return fmt.Sprintf("%s: { %s, scenario: %s%s, given: %q, when: %q, then: %q }", name, s.yamlKey, dc.scenario, covers, dc.given, dc.when, dc.then)
}

// flowScalar writes a case name as YAML takes it inside a flow sequence:
// plain, or quoted when it holds a character that would end or change it.
func flowScalar(s string) string {
	if strings.ContainsAny(s, ":,[]{}#\"'") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// testName suggests a test name: the subject, then the case, without saying
// the subject's own name twice.
func testName(s *subject, dc derivedCase) string {
	c := dc.name
	if s.raw != "" {
		c = strings.TrimSpace(strings.Replace(c, s.raw, "", 1))
	}
	c = strings.ToLower(strings.NewReplacer(" ", "-", "_", "-", ".", "-").Replace(kebab(c)))
	// A decision-table case names clauses of an expression; a test name
	// keeps only their words and numbers.
	c = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, c)
	for strings.Contains(c, "--") {
		c = strings.ReplaceAll(c, "--", "-")
	}
	return s.name + "-" + strings.Trim(c, "-")
}

// StatePaths lists, per entity with a state machine, its paths from an
// initial to a terminal state, named as a test covers them.
func StatePaths(root *yaml.Node) map[string][]string {
	d := newDesign(root)
	out := map[string][]string{}
	for _, e := range source.Pairs(source.Child(d.root, "entities")) {
		if s := d.flowSubject(e); s != nil {
			for _, dc := range s.cases {
				out[e.Key.Value] = append(out[e.Key.Value], dc.name)
			}
		}
	}
	return out
}

// LeftOut is a derived case of rank other that no test covers: the test
// plan lists it with the reason it was left out.
type LeftOut struct {
	Subject  string // "operation createItem"
	Case     string // "name longer than 40 characters"
	Scenario string // golden or red
	Reason   string
}

// LeftOutCases lists the derived cases of a valid specification that are
// not chosen and that no test covers, by subject in document order.
func LeftOutCases(root *yaml.Node, given Given) []LeftOut {
	d := newDesign(root).withIdentity(given)
	subjects := d.subjects()
	covered := map[string]map[string]bool{}
	for _, p := range source.Pairs(source.Child(d.root, "tests")) {
		key := testSubjectKey(p.Value)
		if covered[key] == nil {
			covered[key] = map[string]bool{}
		}
		for _, item := range source.Items(source.Child(p.Value, "covers")) {
			covered[key][item.Value] = true
		}
	}
	var out []LeftOut
	for _, s := range subjects {
		seen := map[string]bool{}
		for _, dc := range s.cases {
			if covered[s.yamlKey][dc.name] || seen[dc.name] || s.chosen(dc.name) {
				continue
			}
			seen[dc.name] = true
			out = append(out, LeftOut{Subject: s.label, Case: dc.name, Scenario: dc.scenario, Reason: s.leftOutReason(dc)})
		}
	}
	return out
}
