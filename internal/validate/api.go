package validate

import (
	"fmt"
	"strconv"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// The keywords about an operation's interface from the dxlib study
// (docs/dxlib-lessons.md, section 2): a list's whitelists and page size,
// the limits a client keeps to, and the catalogue of problem types.

// checkListOf checks that a list names an entity and its fields, that no
// encrypted field is searched or sorted, or filtered without a hash, and
// that the default page fits the maximum.
func (c *checker) checkListOf(d *design, o operation) {
	l := source.Child(o.node, "listOf")
	if l == nil {
		return
	}
	entNode := source.Child(l, "entity")
	var fields map[string]*yaml.Node
	if viewNode := source.Child(l, "view"); viewNode != nil {
		v := d.views[viewNode.Value]
		if v == nil {
			c.add(viewNode, o.pointer("listOf", "view"), RuleListOf, "%s is not a view of the specification%s", viewNode.Value, suggest(viewNode.Value, d.views))
			return
		}
		for _, p := range source.Pairs(source.Child(v, "properties")) {
			if source.Child(p.Value, "rows") != nil {
				c.add(viewNode, o.pointer("listOf", "view"), RuleListOf, "%s carries the rows of %s in %s, and a list holds one row per record; list a view without rows, and read the rows by the record's identifier", viewNode.Value, source.Str(source.Child(p.Value, "rows")), p.Key.Value)
				return
			}
		}
		entNode, fields = viewNode, d.viewFields(v)
	} else {
		e := d.entities[source.Str(entNode)]
		if e == nil {
			if entNode != nil {
				c.add(entNode, o.pointer("listOf", "entity"), RuleListOf, "%s is not an entity of the specification%s", entNode.Value, suggest(entNode.Value, d.entities))
			}
			return
		}
		fields = fieldsOf(e)
	}
	for _, list := range []string{"searchable", "filterable", "sortable"} {
		for i, item := range source.Items(source.Child(l, list)) {
			ptr := o.pointer("listOf", list, fmt.Sprint(i))
			f := fields[item.Value]
			if f == nil {
				c.add(item, ptr, RuleListOf, "%s is not a field of %s, so it cannot be %s%s", item.Value, entNode.Value, list, suggest(item.Value, fields))
				continue
			}
			if source.Str(source.Child(f, "atRest")) != "encrypted" {
				continue
			}
			switch {
			case list != "filterable":
				c.add(item, ptr, RuleListOf, "%s is encrypted at rest, so it cannot be %s: storage cannot read it; leave it out of %s", item.Value, list, list)
			case source.Str(source.Child(f, "lookup")) != "hash":
				c.add(item, ptr, RuleListOf, "%s is encrypted at rest, so it can be filtered by only through a hash of it; add lookup: hash to the field, or leave it out of filterable", item.Value)
			}
		}
	}
	size := source.Child(l, "pageSize")
	def, max := source.Child(size, "default"), source.Child(size, "maximum")
	if def != nil && max != nil {
		dv, _ := strconv.Atoi(def.Value)
		mv, _ := strconv.Atoi(max.Value)
		if dv > mv {
			c.add(def, o.pointer("listOf", "pageSize", "default"), RuleListOf, "the default page of %d is above the maximum of %d; lower the default, or raise the maximum", dv, mv)
		}
	}
}

// checkLimits refuses a rate over no time, and a burst below the rate.
func (c *checker) checkLimits(o operation) {
	rate := source.Child(source.Child(o.node, "limits"), "rate")
	if rate == nil {
		return
	}
	c.checkTimeout(source.Child(rate, "per"), o.pointer("limits", "rate", "per"), RuleLimits, "the rate")
	burst, requests := source.Child(rate, "burst"), source.Child(rate, "requests")
	if burst != nil && requests != nil {
		b, _ := strconv.Atoi(burst.Value)
		r, _ := strconv.Atoi(requests.Value)
		if b < r {
			c.add(burst, o.pointer("limits", "rate", "burst"), RuleLimits, "a burst of %d is below the %d requests the rate allows, so it would lower the limit; raise it to at least %d, or remove it", b, r, r)
		}
	}
}

// checkProblems checks every response's problem type against the
// catalogue, and that a specification with a catalogue names a type on
// every 4xx and 5xx response.
func (c *checker) checkProblems(d *design, o operation) {
	errors := source.Child(d.root, "errors")
	for _, r := range source.Pairs(source.Child(o.node, "responses")) {
		code := r.Key.Value
		p := source.Child(r.Value, "problem")
		if p == nil {
			if len(source.Pairs(errors)) > 0 && len(code) == 3 && (code[0] == '4' || code[0] == '5') {
				c.add(r.Key, o.pointer("responses", code), RuleProblem, "the specification declares its problem types under errors, so the %s response of %s names one under problem", code, o.id)
			}
			continue
		}
		ptr := o.pointer("responses", code, "problem")
		t := source.Child(errors, p.Value)
		if t == nil {
			c.add(p, ptr, RuleProblem, "%s is not a problem type under errors%s", p.Value, suggestPairs(p.Value, errors))
			continue
		}
		if status := source.Str(source.Child(t, "status")); status != code {
			c.add(p, ptr, RuleProblem, "%s is a %s problem, and this is the %s response; name a problem type of status %s", p.Value, status, code, code)
		}
	}
}

func suggestPairs(name string, n *yaml.Node) string {
	all := map[string]*yaml.Node{}
	for _, p := range source.Pairs(n) {
		all[p.Key.Value] = p.Value
	}
	return suggest(name, all)
}
