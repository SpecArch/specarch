package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// The user interface of docs/ui-design.md: a page's events and where each
// leads.

// routeParams are the parameters a route names, in order.
func routeParams(route string) []string {
	var out []string
	for _, m := range pathParam.FindAllStringSubmatch(route, -1) {
		out = append(out, m[1])
	}
	return out
}

// checkPageEvents checks each event of a page: that it is raised by a page
// or an action of the kind that has it, that the page it leads to exists,
// and that with gives exactly the route parameters of that page from
// fields of the page's entity.
func (c *checker) checkPageEvents(d *design) {
	for _, p := range source.Pairs(source.Child(d.root, "pages")) {
		name, pg := p.Key.Value, p.Value
		kind := source.Str(source.Child(pg, "kind"))
		fields := fieldsOf(d.entities[source.Str(source.Child(pg, "entity"))])
		for _, e := range []struct{ key, kind, what string }{{"onSubmitted", "form", "a form is submitted"}, {"onSelect", "list", "a row of a list is selected"}} {
			n := source.Child(pg, e.key)
			if n == nil {
				continue
			}
			if kind != e.kind {
				c.add(source.Key(pg, e.key), source.Pointer("pages", name, e.key), RuleFlow, "%s is a %s page, and %s is raised when %s; leave it out", name, kind, e.key, e.what)
				continue
			}
			c.checkEvent(d, n, []string{"pages", name, e.key}, fields)
		}
		for i, a := range source.Items(source.Child(pg, "actions")) {
			n := source.Child(a, "then")
			if n == nil {
				continue
			}
			if k := source.Str(source.Child(a, "kind")); k != "operation" {
				c.add(source.Key(a, "then"), source.Pointer("pages", name, "actions", fmt.Sprint(i), "then"), RuleFlow, "the action %s navigates already, and then follows an operation; leave it out", source.Str(source.Child(a, "label")))
				continue
			}
			c.checkEvent(d, n, []string{"pages", name, "actions", fmt.Sprint(i), "then"}, fields)
		}
	}
}

// checkEvent checks where one event leads.
func (c *checker) checkEvent(d *design, ev *yaml.Node, base []string, fields map[string]*yaml.Node) {
	nav := source.Child(ev, "navigate")
	with := source.Child(ev, "with")
	if nav == nil {
		return // the schema asks for navigate beside with
	}
	target := d.pages[nav.Value]
	if target == nil {
		c.add(nav, source.Pointer(append(base, "navigate")...), RuleFlow, "%s is not a page of the specification%s", nav.Value, suggest(nav.Value, d.pages))
		return
	}
	params := routeParams(source.Str(source.Child(target, "route")))
	want := map[string]bool{}
	for _, p := range params {
		want[p] = true
	}
	given := map[string]bool{}
	for _, kv := range source.Pairs(with) {
		given[kv.Key.Value] = true
		if !want[kv.Key.Value] {
			c.add(kv.Key, source.Pointer(append(base, "with", kv.Key.Value)...), RuleFlow, "%s is not a route parameter of page %s, whose route is %s", kv.Key.Value, nav.Value, source.Str(source.Child(target, "route")))
			continue
		}
		if fields[kv.Value.Value] == nil {
			c.add(kv.Value, source.Pointer(append(base, "with", kv.Key.Value)...), RuleFlow, "%s is not a field of the page's entity%s", kv.Value.Value, suggest(kv.Value.Value, fields))
		}
	}
	var missing []string
	for _, p := range params {
		if !given[p] {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		at := with
		if at == nil {
			at = nav
		}
		c.add(at, source.Pointer(append(base, "navigate")...), RuleFlow, "page %s needs the route parameter %s; give it under with, from a field of the page's entity", nav.Value, strings.Join(missing, " and "))
	}
}

// eventTarget is the page an event of a page leads to, or "" when it
// leads nowhere or the page does not raise it; raised says whether it does.
func eventTarget(pg *yaml.Node, event, action string) (target string, raised bool) {
	switch event {
	case "select":
		n := source.Child(pg, "onSelect")
		return source.Str(source.Child(n, "navigate")), n != nil
	case "submitted":
		return source.Str(source.Child(source.Child(pg, "onSubmitted"), "navigate")), source.Str(source.Child(pg, "kind")) == "form"
	case "action":
		for _, a := range source.Items(source.Child(pg, "actions")) {
			if source.Str(source.Child(a, "label")) != action {
				continue
			}
			if source.Str(source.Child(a, "kind")) == "navigate" {
				return source.Str(source.Child(a, "target")), true
			}
			return source.Str(source.Child(source.Child(a, "then"), "navigate")), true
		}
	}
	return "", false
}

// stepEvent names the event of a step as a sentence does.
func stepEvent(step *yaml.Node) string {
	if e := source.Str(source.Child(step, "event")); e != "action" {
		return e
	}
	return "the action " + source.Str(source.Child(step, "action"))
}

// checkFlows checks each flow: that its actor is a role that may open
// every page on the way, that each page exists and raises the step's
// event, and that each event leads to the next step's page.
func (c *checker) checkFlows(d *design) {
	for _, f := range source.Pairs(source.Child(d.root, "flows")) {
		name := f.Key.Value
		actorNode := source.Child(f.Value, "actor")
		actor := source.Str(actorNode)
		role := d.roles[actor]
		if actorNode != nil && role == nil {
			c.add(actorNode, source.Pointer("flows", name, "actor"), RuleFlow, "%s is not a role of the specification%s", actor, suggest(actor, d.roles))
		}
		granted := map[string]bool{"public": true}
		for _, p := range source.Items(source.Child(role, "permissions")) {
			granted[p.Value] = true
		}
		steps := source.Items(source.Child(f.Value, "steps"))
		for i, step := range steps {
			at := []string{"flows", name, "steps", fmt.Sprint(i)}
			pageNode := source.Child(step, "page")
			pg := d.pages[source.Str(pageNode)]
			if pg == nil {
				c.add(pageNode, source.Pointer(append(at, "page")...), RuleFlow, "%s is not a page of the specification%s", source.Str(pageNode), suggest(source.Str(pageNode), d.pages))
				continue
			}
			if perm := source.Str(source.Child(pg, "permission")); role != nil && !granted[perm] {
				c.add(pageNode, source.Pointer(append(at, "page")...), RuleFlow, "%s cannot open %s, which needs %s; grant it to the role, or give the flow another actor", actor, pageNode.Value, perm)
			}
			event := source.Str(source.Child(step, "event"))
			target, raised := eventTarget(pg, event, source.Str(source.Child(step, "action")))
			if !raised {
				what := map[string]string{"select": "has no onSelect", "submitted": "is not a form", "action": "has no action labelled " + source.Str(source.Child(step, "action"))}[event]
				c.add(source.Child(step, "event"), source.Pointer(append(at, "event")...), RuleFlow, "page %s %s, so it does not raise %s", pageNode.Value, what, stepEvent(step))
				continue
			}
			if i+1 < len(steps) {
				next := source.Str(source.Child(steps[i+1], "page"))
				if target != next {
					leads := "leads nowhere"
					if target != "" {
						leads = "leads to " + target
					}
					c.add(source.Child(step, "event"), source.Pointer(append(at, "event")...), RuleFlow, "%s on page %s %s, and the next step is on %s; make the event lead there, or correct the steps", stepEvent(step), pageNode.Value, leads, next)
				}
			}
		}
	}
}

// screenFlowSubject is a flow as a test subject: its golden case walks the
// steps.
func (d *design) screenFlowSubject(p source.Pair) *subject {
	name := p.Key.Value
	s := &subject{kind: "screenFlow", label: "flow " + name, node: p.Key, path: source.Pointer("flows", name),
		yamlKey: "flow: " + name, name: name}
	var pages []string
	for _, step := range source.Items(source.Child(p.Value, "steps")) {
		pages = append(pages, source.Str(source.Child(step, "page")))
	}
	s.success = golden("a "+source.Str(source.Child(p.Value, "actor")), "they go through "+strings.Join(pages, ", "), "each step leads to the next, and the last completes")
	return s
}
