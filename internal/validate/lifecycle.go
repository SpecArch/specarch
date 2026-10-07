package validate

import (
	"fmt"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// checkRequirementsStage checks the links inside the requirements stage:
// a requirement's needs exist, a need's stakeholders exist.
func (c *checker) checkRequirementsStage(d *design) {
	for name, need := range d.needs {
		for i, sh := range source.Items(source.Child(need, "stakeholders")) {
			if sh.Value != "" && d.stakeholders[sh.Value] == nil {
				c.add(sh, source.Pointer("needs", name, "stakeholders", fmt.Sprint(i)), RuleStakeholder,
					"%s is not a stakeholder of the specification%s", sh.Value, suggest(sh.Value, d.stakeholders))
			}
		}
	}
	for name, req := range d.requirements {
		for i, n := range source.Items(source.Child(req, "needs")) {
			if n.Value != "" && d.needs[n.Value] == nil {
				c.add(n, source.Pointer("requirements", name, "needs", fmt.Sprint(i)), RuleNeed,
					"%s is not a need of the specification%s", n.Value, suggest(n.Value, d.needs))
			}
		}
	}
}

// checkDeploymentStage checks the links inside the deployment and
// commissioning stages, and that no secret carries a value.
func (c *checker) checkDeploymentStage(d *design) {
	for name, env := range d.environments {
		next := source.Child(env, "promotesTo")
		if v := source.Str(next); v != "" && d.environments[v] == nil {
			c.add(next, source.Pointer("environments", name, "promotesTo"), RuleEnvironment,
				"%s is not an environment of the specification%s", v, suggest(v, d.environments))
		}
	}
	for name, chk := range d.checks {
		env := source.Child(chk, "environment")
		if v := source.Str(env); v != "" && d.environments[v] == nil {
			c.add(env, source.Pointer("checks", name, "environment"), RuleEnvironment,
				"%s is not an environment of the specification%s", v, suggest(v, d.environments))
		}
	}
	for name, setting := range d.settings {
		if source.Str(source.Child(setting, "secret")) != "true" {
			continue
		}
		if def := source.Child(source.Child(setting, "schema"), "default"); def != nil {
			c.add(def, source.Pointer("configuration", name, "schema", "default"), RuleSecretValue,
				"%s is a secret, and a secret's value is never written in a specification; remove the default and say in the description where the value comes from", name)
		}
	}
}

// checkTraceability reports what the stages a specification covers leave
// open: a need no requirement refines, a requirement without acceptance
// criteria, a requirement nothing satisfies once there is a design, and a
// requirement nothing verifies once there are tests or checks. All are
// warnings.
func (c *checker) checkTraceability(d *design) {
	satisfied := map[string]bool{}
	verified := map[string]bool{}
	walk(d.root, nil, func(n *yaml.Node, path []string) {
		for _, item := range source.Items(source.Child(n, "satisfies")) {
			satisfied[item.Value] = true
		}
		for _, item := range source.Items(source.Child(n, "verifies")) {
			verified[item.Value] = true
		}
	})
	if d.spec != nil {
		for _, impl := range d.spec.Implementations {
			doc := source.Parse(impl.Data)
			walk(doc.Root, nil, func(n *yaml.Node, path []string) {
				for _, item := range source.Items(source.Child(n, "satisfies")) {
					satisfied[item.Value] = true
				}
			})
		}
	}
	refined := map[string]bool{}
	for _, req := range d.requirements {
		for _, n := range source.Items(source.Child(req, "needs")) {
			refined[n.Value] = true
		}
	}
	hasDesign := d.covers("design")
	hasTests := len(source.Pairs(source.Child(d.root, "tests"))) > 0 || len(d.checks) > 0
	for _, p := range source.Pairs(source.Child(d.root, "needs")) {
		if source.Str(source.Child(p.Value, "status")) == "rejected" {
			continue // a rejected need will not be met, so no requirement refines it
		}
		if !refined[p.Key.Value] {
			c.warn(p.Key, source.Pointer("needs", p.Key.Value), RuleNeedUnrefined,
				"no requirement refines need %s; add a requirement with needs: [%s], or set the need's status to rejected", p.Key.Value, p.Key.Value)
		}
	}
	for _, p := range source.Pairs(source.Child(d.root, "requirements")) {
		id := p.Key.Value
		ptr := source.Pointer("requirements", id)
		status := source.Str(source.Child(p.Value, "status"))
		if status == "rejected" || status == "retired" {
			continue
		}
		if source.Child(p.Value, "acceptance") == nil {
			c.warn(p.Key, ptr, RuleAcceptanceMissing, "requirement %s has no acceptance criteria, so no test can show it is met; add acceptance with one verifiable sentence per criterion", id)
		}
		if hasDesign && !satisfied[id] {
			c.warn(p.Key, ptr, RuleRequirementUnsatisfied, "no design element satisfies requirement %s; add satisfies: [%s] to the entity, operation, command, page, algorithm or decision that meets it", id, id)
		}
		if hasTests && !verified[id] {
			c.warn(p.Key, ptr, RuleRequirementUnverified, "no test or commissioning check verifies requirement %s; add verifies: [%s] to the test that shows it is met", id, id)
		}
	}
}

// covers reports whether the specification has any section of a stage.
func (d *design) covers(stage string) bool {
	if d.spec != nil {
		return d.spec.Covers(stage)
	}
	for _, name := range spec.SectionsOf(stage) {
		if source.Child(d.root, name) != nil {
			return true
		}
	}
	return false
}
