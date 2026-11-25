package validate

import (
	"fmt"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// checkScenarios checks each scenario (ADR-251): that its stakeholder and
// each step's actor are stakeholders, that it has steps and each step
// names what it exercises, that every operation, requirement, entity and
// test a step names exists, and that a test's proves names a scenario.
// A scenario with a problem is still read whole: every problem is listed
// at its entry, none stops the rest.
//
// A requirement of priority must that no step names is warned of
// (scenario_uncovered) once the specification has a scenario, so a
// specification without scenarios reads as it did; info.scenarioCoverage
// error makes it an error, scenarios or none.
func (c *checker) checkScenarios(d *design) {
	tests := topMap(d.root, "tests")
	scenarios := topMap(d.root, "scenarios")
	operations := map[string]*yaml.Node{}
	for id, o := range d.operations {
		operations[id] = o.node
	}
	prefixes := d.requirementSetPrefixes()
	for _, p := range source.Pairs(source.Child(d.root, "scenarios")) {
		id := p.Key.Value
		if sh := source.Child(p.Value, "stakeholder"); sh != nil && sh.Value != "" && d.stakeholders[sh.Value] == nil {
			c.add(sh, source.Pointer("scenarios", id, "stakeholder"), RuleStakeholder,
				"%s is not a stakeholder of the specification%s", sh.Value, suggest(sh.Value, d.stakeholders))
		}
		steps := source.Items(source.Child(p.Value, "steps"))
		if len(steps) == 0 {
			at := p.Key
			if s := source.Child(p.Value, "steps"); s != nil {
				at = s
			}
			c.add(at, source.Pointer("scenarios", id), RuleScenario,
				"scenario %s has no step, so it shows no case; add steps, each naming the operation, requirements, entity or tests it exercises", id)
			continue
		}
		for i, st := range steps {
			at := []string{"scenarios", id, "steps", fmt.Sprint(i)}
			if a := source.Child(st, "actor"); a != nil && a.Value != "" && d.stakeholders[a.Value] == nil {
				c.add(a, source.Pointer(append(at, "actor")...), RuleStakeholder,
					"%s is not a stakeholder of the specification%s", a.Value, suggest(a.Value, d.stakeholders))
			}
			named := false
			if op := source.Child(st, "operation"); op != nil {
				named = true
				if op.Value != "" && operations[op.Value] == nil {
					c.add(op, source.Pointer(append(at, "operation")...), RuleOperation,
						"%s is not an operationId of the specification%s", op.Value, suggest(op.Value, operations))
				}
			}
			for j, r := range source.Items(source.Child(st, "requirements")) {
				named = true
				c.checkRequirementLink(d, prefixes, r, source.Pointer(append(at, "requirements", fmt.Sprint(j))...))
			}
			if e := source.Child(st, "entity"); e != nil {
				named = true
				if e.Value != "" && d.entities[e.Value] == nil {
					c.add(e, source.Pointer(append(at, "entity")...), RuleScenario,
						"%s is not an entity of the specification%s", e.Value, suggest(e.Value, d.entities))
				}
			}
			for j, t := range source.Items(source.Child(st, "tests")) {
				named = true
				if t.Value != "" && tests[t.Value] == nil {
					c.add(t, source.Pointer(append(at, "tests", fmt.Sprint(j))...), RuleScenario,
						"%s is not a test of the specification%s", t.Value, suggest(t.Value, tests))
				}
			}
			if !named {
				c.add(st, source.Pointer(at...), RuleScenario,
					"step %d of scenario %s names no operation, requirements, entity or tests, so nothing ties it to the design; name what the step exercises", i+1, id)
			}
		}
	}
	for _, t := range source.Pairs(source.Child(d.root, "tests")) {
		for i, s := range source.Items(source.Child(t.Value, "proves")) {
			if s.Value != "" && scenarios[s.Value] == nil {
				c.add(s, source.Pointer("tests", t.Key.Value, "proves", fmt.Sprint(i)), RuleScenario,
					"%s is not a scenario of the specification%s", s.Value, suggest(s.Value, scenarios))
			}
		}
	}
	strict := scenarioCoverageError(d.root)
	uncovered := map[string]bool{}
	for _, id := range UncoveredMust(d.root) {
		uncovered[id] = true
	}
	for _, p := range source.Pairs(source.Child(d.root, "requirements")) {
		id := p.Key.Value
		if !uncovered[id] {
			continue
		}
		key, ptr := p.Key, source.Pointer("requirements", id)
		if strict {
			c.add(key, ptr, RuleScenarioUncovered, "%s %s", fmt.Sprintf(uncoveredMessage, id, id), uncoveredStrict)
		} else {
			c.warn(key, ptr, RuleScenarioUncovered, "%s", fmt.Sprintf(uncoveredMessage, id, id))
		}
	}
}

// uncoveredMessage is the message of scenario_uncovered, the same in both
// validators; uncoveredStrict is what it adds when info.scenarioCoverage
// makes it an error.
const (
	uncoveredMessage = "requirement %s has priority must and no step of any scenario names it, so no case shows it in sequence with the operations it runs through; add %s to the requirements of the scenario step that exercises it, or write a scenario for it"
	uncoveredStrict  = "(info.scenarioCoverage is error)"
)

// scenarioCoverageError reports whether info.scenarioCoverage makes an
// uncovered must requirement an error.
func scenarioCoverageError(root *yaml.Node) bool {
	return source.Str(source.Child(source.Child(root, "info"), "scenarioCoverage")) == "error"
}

// UncoveredMust lists, in document order, the requirements of priority
// must, neither rejected nor retired nor held elsewhere, that no step of
// any scenario names (ADR-251). It lists none while the specification has
// no scenario, unless info.scenarioCoverage is error. The validator and
// the traceability matrix read it, so one gap gets one mark.
func UncoveredMust(root *yaml.Node) []string {
	if source.Child(root, "scenarios") == nil && !scenarioCoverageError(root) {
		return nil
	}
	named := ScenarioRequirements(root)
	var out []string
	for _, p := range source.Pairs(source.Child(root, "requirements")) {
		if source.Str(source.Child(p.Value, "priority")) != "must" || source.Str(source.Child(p.Value, "ownedBy")) != "" {
			continue
		}
		if s := source.Str(source.Child(p.Value, "status")); s == "rejected" || s == "retired" {
			continue
		}
		if len(named[p.Key.Value]) == 0 {
			out = append(out, p.Key.Value)
		}
	}
	return out
}

// ScenarioRequirements gives, for each requirement a step names, the
// scenarios whose steps name it, each once, in document order.
func ScenarioRequirements(root *yaml.Node) map[string][]string {
	out := map[string][]string{}
	for _, p := range source.Pairs(source.Child(root, "scenarios")) {
		seen := map[string]bool{}
		for _, st := range source.Items(source.Child(p.Value, "steps")) {
			for _, r := range source.Items(source.Child(st, "requirements")) {
				if r.Value != "" && !seen[r.Value] {
					seen[r.Value] = true
					out[r.Value] = append(out[r.Value], p.Key.Value)
				}
			}
		}
	}
	return out
}
