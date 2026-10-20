package validate

import (
	"fmt"
	"strconv"
	"strings"

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

// checkInterface warns of a request body that takes any object, because
// its schema is an object that names no property and does not say
// additionalProperties, and of responses with no success among them: in
// either, what the operation takes or answers is not written, and a
// reader that did not find it must not leave that silent (ADR-130).
func (c *checker) checkInterface(o operation) {
	for _, mt := range source.Pairs(source.Child(source.Child(o.node, "requestBody"), "content")) {
		s := source.Child(mt.Value, "schema")
		if s == nil || !objectOnly(source.Child(s, "type")) {
			continue
		}
		if source.Child(s, "properties") != nil || source.Child(s, "additionalProperties") != nil {
			continue
		}
		c.warn(mt.Key, o.pointer("requestBody", "content", mt.Key.Value, "schema"), RuleRequestBodyOpen, "the %s body of %s is an object that names no property, so what it takes is not written; list its fields under properties, or say additionalProperties: true if it takes any", mt.Key.Value, o.id)
	}
	responses := source.Pairs(source.Child(o.node, "responses"))
	if len(responses) == 0 {
		return
	}
	var codes []string
	for _, r := range responses {
		code := r.Key.Value
		if code == "default" {
			return
		}
		if len(code) != 3 {
			// Not a status code: the schema reports the key.
			continue
		}
		if code[0] >= '1' && code[0] <= '3' {
			return
		}
		codes = append(codes, code)
	}
	// A route closed on purpose never succeeds: its refusal is its answer.
	if len(codes) == 0 || source.Str(source.Child(o.node, "permission")) == permissionNobody {
		return
	}
	c.warn(source.Key(o.node, "responses"), o.pointer("responses"), RuleSuccessResponseMissing, "%s answers %s and no success, so what it answers when it succeeds is not written; add its success response", o.id, strings.Join(codes, ", "))
}

// checkMiddlewares checks each middleware's refusals against the catalogue
// of problem types, as a response's are checked (ADR-131).
func (c *checker) checkMiddlewares(d *design) {
	errors := source.Child(d.root, "errors")
	for _, m := range source.Pairs(source.Child(d.root, "middlewares")) {
		for i, r := range source.Items(source.Child(m.Value, "refusals")) {
			status := source.Str(source.Child(r, "status"))
			at := source.Pointer("middlewares", m.Key.Value, "refusals", fmt.Sprint(i))
			p := source.Child(r, "problem")
			if p == nil {
				if len(source.Pairs(errors)) > 0 {
					c.add(r, at, RuleProblem, "the specification declares its problem types under errors, so the %s refusal of middleware %s names one under problem", status, m.Key.Value)
				}
				continue
			}
			t := source.Child(errors, p.Value)
			if t == nil {
				c.add(p, at+"/problem", RuleProblem, "%s is not a problem type under errors%s", p.Value, suggestPairs(p.Value, errors))
				continue
			}
			if s := source.Str(source.Child(t, "status")); s != status {
				c.add(p, at+"/problem", RuleProblem, "%s is a %s problem, and this is a %s refusal; name a problem type of status %s", p.Value, s, status, status)
			}
		}
	}
}

// checkOperationMiddlewares checks that an operation's middlewares are
// declared, and warns of an operation that names none once the
// specification declares its middlewares, as a refusal names its problem
// once problem types are declared (ADR-131).
func (c *checker) checkOperationMiddlewares(d *design, o operation) {
	declared := map[string]*yaml.Node{}
	for _, p := range source.Pairs(source.Child(d.root, "middlewares")) {
		declared[p.Key.Value] = p.Value
	}
	list := source.Child(o.node, "middlewares")
	if list == nil {
		if len(declared) > 0 {
			c.warn(source.Key(o.pathItem, o.method), o.pointer(), RuleMiddlewareMissing, "the specification declares its middlewares, and %s names none; list the ones it runs, in order, under middlewares, or write middlewares: [] if it runs none", o.id)
		}
		return
	}
	for i, item := range source.Items(list) {
		if declared[item.Value] == nil {
			c.add(item, o.pointer("middlewares", fmt.Sprint(i)), RuleMiddleware, "%s is not a middleware of the specification%s", item.Value, suggest(item.Value, declared))
		}
	}
}

// objectOnly says whether a type is object, alone or with null.
func objectOnly(t *yaml.Node) bool {
	if t == nil {
		return false
	}
	if t.Kind == yaml.ScalarNode {
		return t.Value == "object"
	}
	object := false
	for _, n := range t.Content {
		switch n.Value {
		case "object":
			object = true
		case "null":
		default:
			return false
		}
	}
	return object
}

func suggestPairs(name string, n *yaml.Node) string {
	all := map[string]*yaml.Node{}
	for _, p := range source.Pairs(n) {
		all[p.Key.Value] = p.Value
	}
	return suggest(name, all)
}
