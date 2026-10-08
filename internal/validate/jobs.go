package validate

import (
	"fmt"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// The keywords of the dxlib study for what runs on its own and how a user
// finds a page (docs/dxlib-lessons.md, section 2): jobs and menus.

// checkJobs checks that a job acts as a role, reads and writes entities,
// consumes and publishes messages, and calls dependencies that exist.
func (c *checker) checkJobs(d *design) {
	for _, j := range source.Pairs(source.Child(d.root, "jobs")) {
		name := j.Key.Value
		if r := source.Child(j.Value, "role"); r != nil && d.roles[r.Value] == nil {
			c.add(r, source.Pointer("jobs", name, "role"), RuleJob, "%s is not a role of the specification%s", r.Value, suggest(r.Value, d.roles))
		}
		for _, list := range []string{"reads", "writes"} {
			for i, e := range source.Items(source.Child(j.Value, list)) {
				if d.entities[e.Value] == nil {
					c.add(e, source.Pointer("jobs", name, list, fmt.Sprint(i)), RuleJob, "%s is not an entity of the specification%s", e.Value, suggest(e.Value, d.entities))
				}
			}
		}
		if m := source.Child(source.Child(j.Value, "trigger"), "consumes"); m != nil && d.message(m.Value) == nil {
			c.add(m, source.Pointer("jobs", name, "trigger", "consumes"), RuleJob, "%s does not name a channel and one of its messages; write channel/Message for a message declared under channels", m.Value)
		}
		for i, e := range source.Items(source.Child(j.Value, "emits")) {
			if d.message(e.Value) == nil {
				c.add(e, source.Pointer("jobs", name, "emits", fmt.Sprint(i)), RuleEmits, "%s does not name a channel and one of its messages; write channel/Message for a message declared under channels", e.Value)
			}
		}
		for i, e := range source.Items(source.Child(j.Value, "calls")) {
			if d.dependencies[e.Value] == nil {
				c.add(e, source.Pointer("jobs", name, "calls", fmt.Sprint(i)), RuleDependency, "%s is not a dependency of the specification; declare it under dependencies with its timeout%s", e.Value, suggest(e.Value, d.dependencies))
			}
		}
	}
}

// checkMenus checks that every menu entry that opens a page names one.
func (c *checker) checkMenus(d *design) {
	var visit func(items *yaml.Node, path []string)
	visit = func(items *yaml.Node, path []string) {
		for _, m := range source.Pairs(items) {
			p := append(append([]string{}, path...), m.Key.Value)
			if page := source.Child(m.Value, "page"); page != nil && d.pages[page.Value] == nil {
				c.add(page, source.Pointer(append(p, "page")...), RuleMenu, "%s is not a page of the specification%s", page.Value, suggest(page.Value, d.pages))
			}
			visit(source.Child(m.Value, "items"), append(p, "items"))
		}
	}
	visit(source.Child(d.root, "menus"), []string{"menus"})
}

// jobSubject is a job's tests: it runs, it runs twice over the same
// records without a second effect, its dependencies fail, and an item
// fails every time.
func (d *design) jobSubject(p source.Pair) *subject {
	name := p.Key.Value
	s := &subject{kind: "job", label: "job " + name, node: p.Key, path: source.Pointer("jobs", name),
		yamlKey: "job: " + name, name: kebab(name)}
	run := "the job " + name + " runs"
	again := "it runs again over the same records"
	if m := source.Str(source.Child(source.Child(p.Value, "trigger"), "consumes")); m != "" {
		run = "a " + m + " arrives for the job " + name
		again = "the same " + m + " arrives again"
	}
	s.success = golden("...", run, "it completes")
	s.cases = append(s.cases, derivedCase{name: "runs twice", scenario: "golden", given: "the job " + name + " has run", when: again,
		then: "nothing changes a second time", frequency: rare, critical: true})
	for _, e := range source.Items(source.Child(p.Value, "calls")) {
		dep := d.dependencies[e.Value]
		if dep == nil {
			continue
		}
		s.byNature("dependency fails "+e.Value, e.Value+" answers with an error", run, "...")
		s.byNature("dependency times out "+e.Value, e.Value+" does not answer within "+source.Str(source.Child(dep, "timeout")), run, "...")
	}
	if r := source.Child(p.Value, "retries"); r != nil {
		limit := source.Str(source.Child(r, "limit"))
		then := "after " + limit + " tries the item is set aside for a person"
		if source.Str(source.Child(r, "then")) == "discard" {
			then = "after " + limit + " tries the item is dropped"
		}
		s.red("an item fails every try", occasional, "an item that fails every time it is tried", run, then)
	}
	return s
}
