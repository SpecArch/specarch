package validate

import (
	"fmt"
	"strings"
	"unicode"

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
// fields of the page's entity, or, on a task page, from properties of the
// body of the response the event follows.
func (c *checker) checkPageEvents(d *design) {
	for _, p := range source.Pairs(source.Child(d.root, "pages")) {
		name, pg := p.Key.Value, p.Value
		kind := source.Str(source.Child(pg, "kind"))
		fields := fieldsOf(d.entities[source.Str(source.Child(pg, "entity"))])
		for _, e := range []struct{ key, kind, what string }{{"onSubmitted", "form", "a form or a task is submitted"}, {"onSelect", "list", "a row of a list is selected"}} {
			n := source.Child(pg, e.key)
			if n == nil {
				continue
			}
			switch {
			case e.key == "onSubmitted" && kind == "task":
				c.checkStatusEvents(d, name, pg, n)
				continue
			case kind != e.kind:
				c.add(source.Key(pg, e.key), source.Pointer("pages", name, e.key), RuleFlow, "%s is a %s page, and %s is raised when %s; leave it out", name, kind, e.key, e.what)
				continue
			}
			c.checkEvent(d, n, []string{"pages", name, e.key}, fields, "a field of the page's entity")
		}
		for i, a := range source.Items(source.Child(pg, "actions")) {
			if source.Str(source.Child(a, "kind")) == "navigate" {
				c.checkActionWith(d, name, kind, i, a, fields)
			}
			n := source.Child(a, "then")
			if n == nil {
				continue
			}
			if k := source.Str(source.Child(a, "kind")); k != "operation" {
				c.add(source.Key(a, "then"), source.Pointer("pages", name, "actions", fmt.Sprint(i), "then"), RuleFlow, "the action %s navigates already, and then follows an operation; leave it out", source.Str(source.Child(a, "label")))
				continue
			}
			c.checkEvent(d, n, []string{"pages", name, "actions", fmt.Sprint(i), "then"}, fields, "a field of the page's entity")
		}
	}
}

// checkStatusEvents checks a task page's onSubmitted: each status is one
// its submit operation answers, and each event leads where the body of
// that response can take it.
func (c *checker) checkStatusEvents(d *design, name string, pg, events *yaml.Node) {
	op := d.operations[source.Str(source.Child(pg, "submit"))].node
	for _, kv := range source.Pairs(events) {
		base := []string{"pages", name, "onSubmitted", kv.Key.Value}
		if op == nil {
			continue // the page check reports the operation
		}
		r := source.Child(source.Child(op, "responses"), kv.Key.Value)
		if r == nil {
			var declared []string
			for _, rp := range source.Pairs(source.Child(op, "responses")) {
				if strings.HasPrefix(rp.Key.Value, "2") {
					declared = append(declared, rp.Key.Value)
				}
			}
			answers := "it declares no success"
			if len(declared) > 0 {
				answers = "it answers " + strings.Join(declared, " and ")
			}
			c.add(kv.Key, source.Pointer(base...), RulePage, "%s does not answer %s, so %s cannot act on it; %s", source.Str(source.Child(pg, "submit")), kv.Key.Value, name, answers)
			continue
		}
		c.checkEvent(d, kv.Value, base, d.responseBodyFields(r), "a property of the body of the "+kv.Key.Value+" response")
	}
}

// responseBodyFields are the properties of one response's body: an
// entity's fields when it returns one record of it, or the properties of
// an object it declares inline. A list has none an event can take.
func (d *design) responseBodyFields(r *yaml.Node) map[string]*yaml.Node {
	fields := map[string]*yaml.Node{}
	for _, ct := range source.Pairs(source.Child(r, "content")) {
		schema := source.Child(ct.Value, "schema")
		if ref := source.Str(source.Child(schema, "$ref")); strings.HasPrefix(ref, "#/entities/") {
			for k, v := range fieldsOf(d.entities[strings.TrimPrefix(ref, "#/entities/")]) {
				fields[k] = v
			}
			continue
		}
		for _, p := range source.Pairs(source.Child(schema, "properties")) {
			fields[p.Key.Value] = p.Value
		}
	}
	return fields
}

// checkTaskPage checks what a task page holds: no entity and no source,
// since it loads no record; no columns or filters, which belong to a list;
// fields that are properties of the submit operation's request body; and
// every property the body requires among them.
func (c *checker) checkTaskPage(d *design, name string, pg *yaml.Node) {
	base := []string{"pages", name}
	for _, k := range []struct{ key, why string }{
		{"entity", "it submits to an operation and shows no record of an entity"},
		{"source", "it submits to an operation without loading a record"},
		{"columns", "a list shows columns, and a task shows fields"},
		{"filters", "a list is filtered, and a task shows fields"},
	} {
		if source.Child(pg, k.key) != nil {
			c.add(source.Key(pg, k.key), source.Pointer(append(base, k.key)...), RulePage, "%s is a task page, and %s; leave %s out", name, k.why, k.key)
		}
	}
	submit := source.Str(source.Child(pg, "submit"))
	op := d.operations[submit].node
	if op == nil {
		return // the schema asks for submit, and the reference check reports one that does not exist
	}
	if len(source.Pairs(source.Child(source.Child(op, "requestBody"), "content"))) == 0 {
		c.add(source.Child(pg, "submit"), source.Pointer(append(base, "submit")...), RulePage, "%s takes no request body, and a task page's fields are properties of the body it sends; submit to an operation that takes one", submit)
		return
	}
	props, required := d.requestFields(op)
	// A task that uploads may show no field (checkSections), but not when
	// the upload requires parameters (ADR-162).
	upload := source.Str(source.Child(source.Child(op, "stream"), "direction")) == "upload"
	of, holds := "the request body of "+submit, "whose request body requires"
	if upload {
		of, holds = "the parameters "+submit+" takes beside the file it uploads", "whose upload requires the parameter"
	}
	shown := map[string]bool{}
	for i, f := range source.Items(source.Child(pg, "fields")) {
		shown[f.Value] = true
		if props[f.Value] == nil {
			c.add(f, source.Pointer(append(base, "fields", fmt.Sprint(i))...), RulePage, "%s is not a property of %s, which %s submits to%s", f.Value, of, name, suggest(f.Value, props))
		}
	}
	for i, sec := range source.Items(source.Child(pg, "sections")) {
		for j, f := range source.Items(source.Child(sec, "fields")) {
			shown[f.Value] = true
			if props[f.Value] == nil {
				c.add(f, source.Pointer(append(base, "sections", fmt.Sprint(i), "fields", fmt.Sprint(j))...), RulePage, "%s is not a property of %s, which %s submits to%s", f.Value, of, name, suggest(f.Value, props))
			}
		}
	}
	var missing []string
	for _, r := range required {
		if !shown[r] {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 && (len(shown) > 0 || upload) {
		at := source.Key(pg, "fields")
		if at == nil {
			at = source.Key(pg, "sections")
		}
		if at == nil {
			at = source.Child(pg, "submit")
		}
		c.add(at, source.Pointer(base...), RulePage, "%s submits to %s, %s %s; show %s on the page, since the request cannot succeed without %s", name, submit, holds, strings.Join(missing, " and "), map[bool]string{true: "it", false: "them"}[len(missing) == 1], map[bool]string{true: "it", false: "them"}[len(missing) == 1])
	}
}

// checkContentPage checks what a content page holds: blocks only, no
// entity, source or submit, since it loads and sends nothing; no fields,
// sections, columns or filters, which show a record's fields; no actions,
// since its links are link blocks; a route without parameters, since no
// record gives them; each link to a page of the specification whose route
// takes none; and one label for one place. The other keys of a form, a
// list or a task are refused by the checks of those keys.
func (c *checker) checkContentPage(d *design, name string, pg *yaml.Node) {
	base := []string{"pages", name}
	for _, k := range []struct{ key, why string }{
		{"entity", "shows blocks and no record of an entity"},
		{"source", "loads nothing"},
		{"submit", "sends nothing"},
		{"fields", "shows blocks, not the fields of a form, a view or a task"},
		{"sections", "shows blocks, not the fields of a form or a view in groups"},
		{"columns", "shows blocks, not the columns of a list"},
		{"filters", "shows blocks, not the filters of a list"},
		{"actions", "links to other pages with link blocks"},
	} {
		if source.Child(pg, k.key) != nil {
			c.add(source.Key(pg, k.key), source.Pointer(append(base, k.key)...), RulePage, "%s is a content page, which %s; leave %s out", name, k.why, k.key)
		}
	}
	if ps := routeParams(source.Str(source.Child(pg, "route"))); len(ps) > 0 {
		c.add(source.Child(pg, "route"), source.Pointer(append(base, "route")...), RulePage, "%s is a content page, which has no record to give {%s}; take the parameter out of its route", name, strings.Join(ps, "} and {"))
	}
	// A label names one place: two links of one label that lead apart
	// cannot be told apart by a reader, nor by a flow step, which follows
	// the first.
	type labelled struct {
		block  int
		target string
	}
	labels := map[string]labelled{}
	for i, b := range source.Items(source.Child(pg, "blocks")) {
		link := source.Child(b, "link")
		if label := source.Child(link, "label"); label != nil {
			target := source.Str(source.Child(link, "page")) + " " + source.Str(source.Child(link, "href"))
			if first, ok := labels[label.Value]; !ok {
				labels[label.Value] = labelled{i, target}
			} else if first.target != target {
				c.add(label, source.Pointer(append(base, "blocks", fmt.Sprint(i), "link", "label")...), RulePage, "%s has links labelled %s at blocks %d and %d that lead to different places, so a reader cannot tell them apart (WCAG 2.2, 2.4.4) and a flow step follows only the first; give each its own label", name, label.Value, first.block, i)
			}
		}
		nav := source.Child(link, "page")
		if nav == nil {
			continue
		}
		at := append(base, "blocks", fmt.Sprint(i), "link", "page")
		target := d.pages[nav.Value]
		if target == nil {
			c.add(nav, source.Pointer(at...), RulePage, "%s is not a page of the specification%s", nav.Value, suggest(nav.Value, d.pages))
			continue
		}
		if ps := routeParams(source.Str(source.Child(target, "route"))); len(ps) > 0 {
			c.add(nav, source.Pointer(at...), RuleFlow, "page %s needs the route parameter %s, and a link of a content page gives none; link to a page whose route takes none", nav.Value, strings.Join(ps, " and "))
		}
	}
}

// checkEvent checks where one event leads, and that its message is a full
// sentence; the route parameters it gives come from fields, which from
// names.
func (c *checker) checkEvent(d *design, ev *yaml.Node, base []string, fields map[string]*yaml.Node, from string) {
	if m := source.Child(ev, "message"); m != nil && !sentence(m.Value) {
		c.add(m, source.Pointer(append(base, "message")...), RuleFlow, "the message is not a full sentence; start it with a capital and end it with a full stop, so a screen reader reads it as one")
	}
	nav := source.Child(ev, "navigate")
	if nav == nil {
		return // the schema asks for navigate beside with
	}
	if d.pages[nav.Value] == nil {
		c.add(nav, source.Pointer(append(base, "navigate")...), RuleFlow, "%s is not a page of the specification%s", nav.Value, suggest(nav.Value, d.pages))
		return
	}
	c.checkWith(d, nav, "navigate", source.Child(ev, "with"), base, fields, from)
}

// checkActionWith checks the route parameters an action of kind navigate
// gives the page it opens. On a list they come from the row, under with,
// and a page whose route takes none is opened from the toolbar; on any
// other page with is left out, since a view takes them from its record's
// fields of their names or its own route.
func (c *checker) checkActionWith(d *design, name, kind string, i int, a *yaml.Node, fields map[string]*yaml.Node) {
	base := []string{"pages", name, "actions", fmt.Sprint(i)}
	with := source.Child(a, "with")
	if kind != "list" {
		if with != nil {
			c.add(source.Key(a, "with"), source.Pointer(append(base, "with")...), RuleFlow, "%s is a %s page, and with gives a list's row the route parameters of the page it opens; leave it out", name, kind)
		}
		return
	}
	target := source.Child(a, "target")
	if target == nil || d.pages[target.Value] == nil {
		return // the schema asks for target, and the page check reports one that does not exist
	}
	c.checkWith(d, target, "target", with, base, fields, "a field of the page's entity")
}

// checkWith checks that with gives exactly the route parameters of the
// page nav names, each from one of fields; key is where nav is, under
// base.
func (c *checker) checkWith(d *design, nav *yaml.Node, key string, with *yaml.Node, base []string, fields map[string]*yaml.Node, from string) {
	target := d.pages[nav.Value]
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
			c.add(kv.Value, source.Pointer(append(base, "with", kv.Key.Value)...), RuleFlow, "%s is not %s%s", kv.Value.Value, from, suggest(kv.Value.Value, fields))
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
		c.add(at, source.Pointer(append(base, key)...), RuleFlow, "page %s needs the route parameter %s; give it under with, from %s", nav.Value, strings.Join(missing, " and "), from)
	}
}

// eventTargets are the pages an event of a page leads to: one, or on a
// task page one per status it acts on, and none when it leads nowhere or
// the page does not raise it; raised says whether it does.
func eventTargets(pg *yaml.Node, event, action string) (targets []string, raised bool) {
	one := func(t string) []string {
		if t == "" {
			return nil
		}
		return []string{t}
	}
	switch event {
	case "select":
		n := source.Child(pg, "onSelect")
		return one(source.Str(source.Child(n, "navigate"))), n != nil
	case "submitted":
		switch source.Str(source.Child(pg, "kind")) {
		case "form":
			return one(source.Str(source.Child(source.Child(pg, "onSubmitted"), "navigate"))), true
		case "task":
			for _, kv := range source.Pairs(source.Child(pg, "onSubmitted")) {
				if t := source.Str(source.Child(kv.Value, "navigate")); t != "" && !contains(targets, t) {
					targets = append(targets, t)
				}
			}
			return targets, true
		}
	case "action":
		for _, a := range source.Items(source.Child(pg, "actions")) {
			if source.Str(source.Child(a, "label")) != action {
				continue
			}
			if source.Str(source.Child(a, "kind")) == "navigate" {
				return one(source.Str(source.Child(a, "target"))), true
			}
			return one(source.Str(source.Child(source.Child(a, "then"), "navigate"))), true
		}
		for _, b := range source.Items(source.Child(pg, "blocks")) {
			if l := source.Child(b, "link"); l != nil && source.Str(source.Child(l, "label")) == action {
				return one(source.Str(source.Child(l, "page"))), true
			}
		}
	}
	return nil, false
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
		granted := d.roleHolds(role)
		granted["public"] = true
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
			if event == "action" && role != nil {
				for _, a := range source.Items(source.Child(pg, "actions")) {
					if source.Str(source.Child(a, "label")) != source.Str(source.Child(step, "action")) {
						continue
					}
					if perm := source.Str(source.Child(a, "permission")); perm != "" && !granted[perm] {
						c.add(source.Child(step, "action"), source.Pointer(append(at, "action")...), RuleFlow, "%s cannot take the action %s, which needs %s; grant it to the role, or give the flow another actor", actor, source.Str(source.Child(step, "action")), perm)
					}
					break
				}
			}
			targets, raised := eventTargets(pg, event, source.Str(source.Child(step, "action")))
			if !raised {
				what := map[string]string{"select": "has no onSelect", "submitted": "is not a form or a task", "action": "has no action labelled " + source.Str(source.Child(step, "action"))}[event]
				if event == "action" && source.Str(source.Child(pg, "kind")) == "content" {
					what = "has no link labelled " + source.Str(source.Child(step, "action"))
				}
				c.add(source.Child(step, "event"), source.Pointer(append(at, "event")...), RuleFlow, "page %s %s, so it does not raise %s", pageNode.Value, what, stepEvent(step))
				continue
			}
			if i+1 < len(steps) {
				next := source.Str(source.Child(steps[i+1], "page"))
				if !contains(targets, next) && d.pages[next] != nil {
					leads := "leads nowhere"
					if len(targets) > 0 {
						leads = "leads to " + strings.Join(targets, " or ")
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

// pageProblems are the problem types a page can meet, by name, with the
// operation that answers each first: those of the operation it reads or
// submits and of every operation its actions run.
func (d *design) pageProblems(pg *yaml.Node) (names []string, by map[string]string) {
	by = map[string]string{}
	ids := []string{source.Str(source.Child(pg, "source")), source.Str(source.Child(pg, "submit"))}
	for _, a := range source.Items(source.Child(pg, "actions")) {
		if source.Str(source.Child(a, "kind")) == "operation" {
			ids = append(ids, source.Str(source.Child(a, "target")))
		}
	}
	for _, id := range ids {
		o, ok := d.operations[id]
		if id == "" || !ok {
			continue
		}
		for _, r := range source.Pairs(source.Child(o.node, "responses")) {
			if p := source.Str(source.Child(r.Value, "problem")); p != "" && by[p] == "" {
				by[p] = id
				names = append(names, p)
			}
		}
	}
	return names, by
}

// sentence reports whether a message reads as a full sentence: a capital,
// a letter of a script without case, or a digit first, and a full stop,
// question mark or exclamation mark last, in Latin or CJK form. Only the
// ASCII spaces and a byte order mark around it are trimmed, the same in
// both builds, since one YAML reader drops a byte order mark and the other
// keeps it.
func sentence(s string) bool {
	s = strings.Trim(s, " \t\n\r\v\f\ufeff")
	if s == "" {
		return false
	}
	rs := []rune(s)
	first, last := rs[0], rs[len(rs)-1]
	return unicode.In(first, unicode.Lu, unicode.Lt, unicode.Lo, unicode.Nd) && strings.ContainsRune(".?!。？！", last)
}

// checkPageStates checks a page's states: a list has empty, a list with
// filters has filteredEmpty and a page without them has neither, every
// problem type the page can meet is named under failed or covered by its
// default and no other is named, a field a problem is about is a field the
// form shows, and every message is a full sentence. A page without states
// is not checked: its stack shows its own.
func (c *checker) checkPageStates(d *design) {
	for _, p := range source.Pairs(source.Child(d.root, "pages")) {
		name, pg := p.Key.Value, p.Value
		states := source.Child(pg, "states")
		if states == nil {
			continue
		}
		base := []string{"pages", name, "states"}
		kind := source.Str(source.Child(pg, "kind"))
		filters := len(source.Items(source.Child(pg, "filters"))) > 0
		message := func(st *yaml.Node, at []string) {
			if m := source.Child(st, "message"); m != nil && !sentence(m.Value) {
				c.add(m, source.Pointer(append(at, "message")...), RuleState, "the message is not a full sentence; start it with a capital and end it with a full stop, so a screen reader reads it as one")
			}
		}
		for _, k := range []string{"empty", "filteredEmpty"} {
			if st := source.Child(states, k); st != nil {
				message(st, append(base, k))
			}
		}
		switch {
		case kind == "list" && source.Child(states, "empty") == nil:
			c.add(source.Key(pg, "states"), source.Pointer(base...), RuleState, "%s is a list, so it shows an empty state when there are no records; add empty with its message", name)
		case kind != "list" && source.Child(states, "empty") != nil:
			c.add(source.Key(states, "empty"), source.Pointer(append(base, "empty")...), RuleState, "%s is a %s, which is never empty; leave empty out", name, kind)
		}
		switch {
		case filters && source.Child(states, "filteredEmpty") == nil:
			c.add(source.Key(pg, "states"), source.Pointer(base...), RuleState, "%s has filters, so it shows a state when no record matches them; add filteredEmpty with its message", name)
		case !filters && source.Child(states, "filteredEmpty") != nil:
			c.add(source.Key(states, "filteredEmpty"), source.Pointer(append(base, "filteredEmpty")...), RuleState, "%s has no filters, so nothing can filter it empty; leave filteredEmpty out", name)
		}
		problems, by := d.pageProblems(pg)
		failed := source.Child(states, "failed")
		fields := map[string]bool{}
		for _, f := range pageFields(pg) {
			fields[f.Value] = true
		}
		named := map[string]*yaml.Node{}
		for _, kv := range source.Pairs(failed) {
			at := append(base, "failed", kv.Key.Value)
			named[kv.Key.Value] = kv.Value
			if kv.Key.Value != "default" && by[kv.Key.Value] == "" {
				known := map[string]*yaml.Node{}
				for _, pr := range problems {
					known[pr] = kv.Key
				}
				c.add(kv.Key, source.Pointer(at...), RuleState, "%s is not a problem type an operation of %s answers%s", kv.Key.Value, name, suggest(kv.Key.Value, known))
			}
			message(kv.Value, at)
			if f := source.Child(kv.Value, "field"); f != nil {
				switch {
				case kind != "form" && kind != "task":
					c.add(f, source.Pointer(append(at, "field")...), RuleState, "field is for a form or a task, which shows a problem beside the field it is about; %s is a %s", name, kind)
				case !fields[f.Value]:
					c.add(f, source.Pointer(append(at, "field")...), RuleState, "%s is not a field the %s %s shows", f.Value, kind, name)
				}
			}
		}
		if named["default"] != nil {
			continue
		}
		var missing []string
		for _, pr := range problems {
			if named[pr] == nil {
				missing = append(missing, pr+" (from "+by[pr]+")")
			}
		}
		if len(missing) > 0 {
			at := source.Key(states, "failed")
			if at == nil {
				at = source.Key(pg, "states")
			}
			c.add(at, source.Pointer(append(base, "failed")...), RuleState, "%s can fail with %s; give each a message under failed, or a default for the rest", name, strings.Join(missing, ", "))
		}
	}
}

// checkCompactColumns checks that a page's compact columns are a list's
// own columns: what a compact screen keeps of them.
func (c *checker) checkCompactColumns(d *design) {
	for _, p := range source.Pairs(source.Child(d.root, "pages")) {
		name, pg := p.Key.Value, p.Value
		cc := source.Child(pg, "compactColumns")
		if cc == nil {
			continue
		}
		if kind := source.Str(source.Child(pg, "kind")); kind != "list" {
			c.add(source.Key(pg, "compactColumns"), source.Pointer("pages", name, "compactColumns"), RulePage, "%s is a %s, and compactColumns is what a list keeps of its columns on a compact screen; leave it out", name, kind)
			continue
		}
		columns := map[string]*yaml.Node{}
		for _, col := range source.Items(source.Child(pg, "columns")) {
			columns[col.Value] = col
		}
		for i, col := range source.Items(cc) {
			if columns[col.Value] == nil {
				c.add(col, source.Pointer("pages", name, "compactColumns", fmt.Sprint(i)), RulePage, "%s is not a column of %s, and a compact screen keeps only columns the list has%s", col.Value, name, suggest(col.Value, columns))
			}
		}
	}
}

// checkAccessibility checks, once the specification names its target,
// what the design decides of it: every field a page shows or filters by has a title, its
// label (WCAG 2.2, 3.3.2 and 2.4.6), and no two actions of a page share a
// label, so each has a name of its own (4.1.2).
func (c *checker) checkAccessibility(d *design) {
	if source.Child(d.root, "accessibility") == nil {
		return
	}
	shownOn := map[string][]string{} // "Entity.field" to the pages that show it
	var order []string
	for _, p := range source.Pairs(source.Child(d.root, "pages")) {
		name, pg := p.Key.Value, p.Value
		ent := source.Str(source.Child(pg, "entity"))
		for _, list := range [][]*yaml.Node{source.Items(source.Child(pg, "columns")), pageFields(pg), source.Items(source.Child(pg, "filters"))} {
			for _, f := range list {
				k := ent + "." + f.Value
				if len(shownOn[k]) == 0 {
					order = append(order, k)
				}
				if l := shownOn[k]; len(l) == 0 || l[len(l)-1] != name {
					shownOn[k] = append(l, name)
				}
			}
		}
		for _, row := range source.Items(source.Child(pg, "childRows")) {
			target := d.childRowTarget(pg, row)
			for _, f := range source.Items(source.Child(row, "fields")) {
				k := target + "." + f.Value
				if target == "" || shownOn[k] != nil && shownOn[k][len(shownOn[k])-1] == name {
					continue
				}
				if len(shownOn[k]) == 0 {
					order = append(order, k)
				}
				shownOn[k] = append(shownOn[k], name)
			}
		}
		seen := map[string]bool{}
		for i, a := range source.Items(source.Child(pg, "actions")) {
			l := source.Child(a, "label")
			if l == nil {
				continue
			}
			if seen[l.Value] {
				c.add(l, source.Pointer("pages", name, "actions", fmt.Sprint(i), "label"), RuleAccessibility, "%s has two actions labelled %s, which a screen reader cannot tell apart (WCAG 2.2, 4.1.2); give each its own label", name, l.Value)
			}
			seen[l.Value] = true
		}
		c.checkBlocksAccessible(name, pg)
	}
	for _, k := range order {
		ent, field, _ := strings.Cut(k, ".")
		f := fieldsOf(d.entities[ent])[field]
		if f == nil || source.Child(f, "title") != nil {
			continue
		}
		key := source.Key(source.Child(d.entities[ent], "properties"), field)
		c.add(key, source.Pointer("entities", ent, "properties", field), RuleAccessibility, "%s.%s is shown on %s and has no title, the label a person reads beside it (WCAG 2.2, 3.3.2 and 2.4.6); give it a title", ent, field, strings.Join(shownOn[k], ", "))
	}
}

// checkBlocksAccessible checks the blocks of a content page: every image
// has its alternative text (WCAG 2.2, 1.1.1), every link its label (2.4.4),
// and no heading skips a level below the one before it, the page's title
// being level 1 (1.3.1).
func (c *checker) checkBlocksAccessible(name string, pg *yaml.Node) {
	level := int64(1)
	for i, b := range source.Items(source.Child(pg, "blocks")) {
		at := []string{"pages", name, "blocks", fmt.Sprint(i)}
		if img := source.Child(b, "image"); img != nil && source.Child(img, "alternativeText") == nil {
			c.add(source.Key(b, "image"), source.Pointer(append(at, "image")...), RuleAccessibility, "%s shows the image %s with no alternativeText, the text read in its place to who cannot see it (WCAG 2.2, 1.1.1); give it its alternativeText", name, source.Str(source.Child(img, "path")))
		}
		if l := source.Child(b, "link"); l != nil && source.Child(l, "label") == nil {
			to := source.Str(source.Child(l, "page"))
			if to == "" {
				to = source.Str(source.Child(l, "href"))
			}
			c.add(source.Key(b, "link"), source.Pointer(append(at, "link")...), RuleAccessibility, "the link of %s to %s has no label, the text that says where it leads (WCAG 2.2, 2.4.4); give it a label", name, to)
		}
		h := source.Child(source.Child(b, "heading"), "level")
		n, ok := source.Int(h)
		if !ok {
			continue // the schema asks for a level of 1 to 6
		}
		if n > level+1 {
			c.add(h, source.Pointer(append(at, "heading", "level")...), RuleAccessibility, "the heading %s of %s is level %d after level %d, which skips a level (WCAG 2.2, 1.3.1); make it level %d at most", source.Str(source.Child(source.Child(b, "heading"), "text")), name, n, level, level+1)
		}
		level = n
	}
}

// pageFields are the fields a form or a view shows, in order: its fields,
// or the fields of its sections one after another.
func pageFields(pg *yaml.Node) []*yaml.Node {
	if f := source.Child(pg, "fields"); f != nil {
		return source.Items(f)
	}
	var out []*yaml.Node
	for _, sec := range source.Items(source.Child(pg, "sections")) {
		out = append(out, source.Items(source.Child(sec, "fields"))...)
	}
	return out
}

// checkSections checks that a form or a view gives its fields once, in
// fields or in sections, that no field is in two sections, and that a list,
// which shows columns, has no sections. A task that submits to an upload
// stream may show no field: the file is what it takes (ADR-146).
func (c *checker) checkSections(d *design) {
	for _, p := range source.Pairs(source.Child(d.root, "pages")) {
		name, pg := p.Key.Value, p.Value
		secs := source.Child(pg, "sections")
		kind := source.Str(source.Child(pg, "kind"))
		if kind == "form" || kind == "view" || kind == "task" {
			switch fields := source.Child(pg, "fields"); {
			case fields == nil && secs == nil && !(kind == "task" && source.Str(source.Child(source.Child(d.operations[source.Str(source.Child(pg, "submit"))].node, "stream"), "direction")) == "upload"):
				c.add(p.Key, source.Pointer("pages", name), RulePage, "%s is a %s and shows no field; give its fields, or its sections", name, kind)
			case fields != nil && secs != nil:
				c.add(source.Key(pg, "sections"), source.Pointer("pages", name, "sections"), RulePage, "%s gives both fields and sections; name its fields once, in fields or in sections", name)
				continue
			}
		}
		if secs == nil || kind == "content" {
			continue // a content page's sections are reported with its other keys
		}
		if kind == "list" {
			c.add(source.Key(pg, "sections"), source.Pointer("pages", name, "sections"), RulePage, "%s is a list, which shows columns, and sections group the fields of a form or a view; leave them out", name)
			continue
		}
		in := map[string]string{}
		for i, sec := range source.Items(secs) {
			title := source.Str(source.Child(sec, "title"))
			for j, f := range source.Items(source.Child(sec, "fields")) {
				if other, ok := in[f.Value]; ok {
					c.add(f, source.Pointer("pages", name, "sections", fmt.Sprint(i), "fields", fmt.Sprint(j)), RulePage, "%s is in the section %s already, and a field is shown once; leave it out of %s", f.Value, other, title)
					continue
				}
				in[f.Value] = title
			}
		}
	}
}

// checkChildRows checks the child rows of a form: only a form has them,
// each names a one-to-many relation of the form's entity once, and its
// fields are fields of the entity the relation reaches.
func (c *checker) checkChildRows(d *design) {
	for _, p := range source.Pairs(source.Child(d.root, "pages")) {
		name, pg := p.Key.Value, p.Value
		rows := source.Child(pg, "childRows")
		if rows == nil {
			continue
		}
		if kind := source.Str(source.Child(pg, "kind")); kind != "form" {
			c.add(source.Key(pg, "childRows"), source.Pointer("pages", name, "childRows"), RulePage, "%s is a %s, and childRows are the records of a relation edited under a form; leave them out", name, kind)
			continue
		}
		ent := source.Str(source.Child(pg, "entity"))
		e := d.entities[ent]
		if e == nil {
			continue // the entity check reports it
		}
		relations := map[string]*yaml.Node{}
		for _, r := range source.Pairs(source.Child(e, "relations")) {
			relations[r.Key.Value] = r.Value
		}
		seen := map[string]bool{}
		for i, row := range source.Items(rows) {
			at := []string{"pages", name, "childRows", fmt.Sprint(i)}
			rn := source.Child(row, "relation")
			rel := source.Str(rn)
			if rn == nil {
				continue // the schema asks for it
			}
			if seen[rel] {
				c.add(rn, source.Pointer(append(at, "relation")...), RulePage, "%s has child rows of %s already; edit the rows of a relation once", name, rel)
				continue
			}
			seen[rel] = true
			r := relations[rel]
			if r == nil {
				c.add(rn, source.Pointer(append(at, "relation")...), RulePage, "%s is not a relation of %s, the entity of %s%s", rel, ent, name, suggest(rel, relations))
				continue
			}
			if k := source.Str(source.Child(r, "kind")); k != "one-to-many" {
				c.add(rn, source.Pointer(append(at, "relation")...), RulePage, "%s is a %s relation of %s, and child rows are the records of a one-to-many relation", rel, k, ent)
				continue
			}
			target := source.Str(source.Child(r, "target"))
			if d.entities[target] == nil {
				continue // the relation check reports it
			}
			c.checkFieldList(source.Child(row, "fields"), append(at, "fields"), fieldsOf(d.entities[target]), target, "shown in the rows of "+rel)
		}
	}
}

// childRowTarget is the entity the child rows of a form reach through
// their relation, or "" when the relation does not resolve.
func (d *design) childRowTarget(pg, row *yaml.Node) string {
	r := source.Child(source.Child(d.entities[source.Str(source.Child(pg, "entity"))], "relations"), source.Str(source.Child(row, "relation")))
	return source.Str(source.Child(r, "target"))
}
