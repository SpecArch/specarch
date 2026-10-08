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
