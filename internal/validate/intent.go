package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/metamodel"
	"github.com/SpecArch/specarch/internal/source"
)

// checkIntent checks the intent of the requirements stage (ADR-244): the
// references of the problem, the goals, the out-of-scope entries and the
// quality goals, and the six warnings of ADR-242. Nothing here refuses a
// specification for missing intent, and need_without_goal stays silent
// until a goal is written, so a specification without intent reads as it
// did, with problem_missing as its one new warning.
func (c *checker) checkIntent(d *design) {
	goals := topMap(d.root, "goals")
	qualityGoals := topMap(d.root, "qualityGoals")
	problem := source.Child(d.root, "problem")
	for i, sh := range source.Items(source.Child(problem, "stakeholders")) {
		if sh.Value != "" && d.stakeholders[sh.Value] == nil {
			c.add(sh, source.Pointer("problem", "stakeholders", fmt.Sprint(i)), RuleStakeholder,
				"%s is not a stakeholder of the specification%s", sh.Value, suggest(sh.Value, d.stakeholders))
		}
	}
	for name, g := range goals {
		for i, m := range source.Items(source.Child(g, "measuredBy")) {
			if m.Value != "" && d.monitors[m.Value] == nil {
				c.add(m, source.Pointer("goals", name, "measuredBy", fmt.Sprint(i)), RuleMonitor,
					"%s is not a monitor of the specification%s", m.Value, suggest(m.Value, d.monitors))
			}
		}
	}
	for name, out := range topMap(d.root, "outOfScope") {
		for i, n := range source.Items(source.Child(out, "needs")) {
			if n.Value != "" && d.needs[n.Value] == nil {
				c.add(n, source.Pointer("outOfScope", name, "needs", fmt.Sprint(i)), RuleNeed,
					"%s is not a need of the specification%s", n.Value, suggest(n.Value, d.needs))
			}
		}
	}
	for name, q := range qualityGoals {
		for i, r := range source.Items(source.Child(q, "requirements")) {
			if r.Value != "" && d.requirements[r.Value] == nil {
				c.add(r, source.Pointer("qualityGoals", name, "requirements", fmt.Sprint(i)), RuleRequirement,
					"%s is not a requirement of the specification%s", r.Value, suggest(r.Value, d.requirements))
			}
		}
	}
	for name, need := range d.needs {
		for i, g := range source.Items(source.Child(need, "goals")) {
			if g.Value != "" && goals[g.Value] == nil {
				c.add(g, source.Pointer("needs", name, "goals", fmt.Sprint(i)), RuleGoal,
					"%s is not a goal of the specification%s", g.Value, suggest(g.Value, goals))
			}
		}
	}
	for name, dec := range d.decisions {
		for i, q := range source.Items(source.Child(dec, "weighs")) {
			if q.Value != "" && qualityGoals[q.Value] == nil {
				c.add(q, source.Pointer("decisions", name, "weighs", fmt.Sprint(i)), RuleQualityGoal,
					"%s is not a quality goal of the specification%s", q.Value, suggest(q.Value, qualityGoals))
			}
		}
	}

	if ProblemMissing(d.root) && !d.lean() {
		c.warn(source.Child(d.root, "requirements"), "/problem", RuleProblemMissing, "%s", problemMissing)
	}
	firstGoal := ""
	if gs := source.Pairs(source.Child(d.root, "goals")); len(gs) > 0 {
		firstGoal = gs[0].Key.Value
	}
	served := map[string]bool{}
	for _, p := range source.Pairs(source.Child(d.root, "needs")) {
		if source.Str(source.Child(p.Value, "status")) != "accepted" {
			continue
		}
		named := source.Items(source.Child(p.Value, "goals"))
		for _, g := range named {
			served[g.Value] = true
		}
		if len(goals) > 0 && len(named) == 0 {
			c.warn(p.Key, source.Pointer("needs", p.Key.Value), RuleNeedWithoutGoal,
				"need %s names no goal it serves, while the specification has goals; add goals with the IDs of the goals it serves, such as goals: [%s], or set the need's status to rejected when it serves none", p.Key.Value, firstGoal)
		}
	}
	for _, p := range source.Pairs(source.Child(d.root, "goals")) {
		if source.Str(source.Child(p.Value, "status")) != "accepted" {
			continue
		}
		id := p.Key.Value
		ptr := source.Pointer("goals", id)
		var missing []string
		for _, k := range []string{"measure", "target"} {
			if source.Child(p.Value, k) == nil {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			c.warn(p.Key, ptr, RuleGoalUnmeasured,
				"goal %s is accepted with no %s, so it cannot be met or missed; add measure, what is counted, and target, the value wanted, each a plain sentence with a number and its unit", id, strings.Join(missing, " and no "))
		}
		if !served[id] {
			c.warn(p.Key, ptr, RuleGoalUnserved,
				"no accepted need serves goal %s, so nothing the stakeholders asked for leads to it; add goals: [%s] to the need it answers, or set the goal's status to dropped", id, id)
		}
	}
	ruledOut := map[string][]string{}
	for _, p := range source.Pairs(source.Child(d.root, "outOfScope")) {
		for _, n := range source.Items(source.Child(p.Value, "needs")) {
			ruledOut[n.Value] = append(ruledOut[n.Value], p.Key.Value)
		}
	}
	for _, p := range source.Pairs(source.Child(d.root, "requirements")) {
		if source.Str(source.Child(p.Value, "status")) != "accepted" {
			continue
		}
		for i, n := range source.Items(source.Child(p.Value, "needs")) {
			for _, out := range ruledOut[n.Value] {
				c.warn(n, source.Pointer("requirements", p.Key.Value, "needs", fmt.Sprint(i)), RuleScopeConflict,
					"requirement %s is accepted and refines need %s, which out-of-scope entry %s rules out; set the requirement's status to rejected, or, when the boundary has moved, take %s out of the needs of %s", p.Key.Value, n.Value, out, n.Value, out)
			}
		}
	}
	byRank := map[string][]string{}
	for _, p := range source.Pairs(source.Child(d.root, "qualityGoals")) {
		if r := source.Child(p.Value, "rank"); r != nil && r.Value != "" {
			byRank[r.Value] = append(byRank[r.Value], p.Key.Value)
		}
	}
	for _, p := range source.Pairs(source.Child(d.root, "qualityGoals")) {
		r := source.Child(p.Value, "rank")
		if r == nil || len(byRank[r.Value]) < 2 {
			continue
		}
		c.warn(r, source.Pointer("qualityGoals", p.Key.Value, "rank"), RuleQualityGoalRank,
			"quality goals %s share rank %s, so the order does not decide a trade-off between them; give each its own rank", joinAnd(byRank[r.Value]), r.Value)
	}
}

// ProblemMissing reports whether a specification has requirements and no
// problem (ADR-244), on a meta-model whose schema has the key, with no
// open must question blocking the problem. The validator warns of it on
// the full profile; specarch gaps --next asks for it first on every
// profile.
func ProblemMissing(root *yaml.Node) bool {
	d := newDesign(root)
	return len(d.requirements) > 0 && source.Child(root, "problem") == nil &&
		!d.mustBlocked("problem") && !metamodel.Design.IsEarlier(d.metaModelVersion())
}

// problemMissing is the message of problem_missing, the same in both
// validators.
const problemMissing = "the specification has requirements and no problem, so nothing says what the system is for, and no goal or requirement can be judged against it; add problem with statement, the problem the system solves in plain words, and, when they are known, the stakeholders it hurts, how things stand today and why"
