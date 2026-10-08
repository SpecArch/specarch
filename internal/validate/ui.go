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
	shown := map[string]bool{}
	for i, f := range source.Items(source.Child(pg, "fields")) {
		shown[f.Value] = true
		if props[f.Value] == nil {
			c.add(f, source.Pointer(append(base, "fields", fmt.Sprint(i))...), RulePage, "%s is not a property of the request body of %s, which %s submits to%s", f.Value, submit, name, suggest(f.Value, props))
		}
	}
	for i, sec := range source.Items(source.Child(pg, "sections")) {
		for j, f := range source.Items(source.Child(sec, "fields")) {
			shown[f.Value] = true
			if props[f.Value] == nil {
				c.add(f, source.Pointer(append(base, "sections", fmt.Sprint(i), "fields", fmt.Sprint(j))...), RulePage, "%s is not a property of the request body of %s, which %s submits to%s", f.Value, submit, name, suggest(f.Value, props))
			}
		}
	}
	var missing []string
	for _, r := range required {
		if !shown[r] {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 && len(shown) > 0 {
		at := source.Key(pg, "fields")
		if at == nil {
			at = source.Key(pg, "sections")
		}
		c.add(at, source.Pointer(base...), RulePage, "%s submits to %s, whose request body requires %s; show %s on the page, since the request cannot succeed without %s", name, submit, strings.Join(missing, " and "), map[bool]string{true: "it", false: "them"}[len(missing) == 1], map[bool]string{true: "it", false: "them"}[len(missing) == 1])
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
		c.add(at, source.Pointer(append(base, "navigate")...), RuleFlow, "page %s needs the route parameter %s; give it under with, from %s", nav.Value, strings.Join(missing, " and "), from)
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
// which shows columns, has no sections.
func (c *checker) checkSections(d *design) {
	for _, p := range source.Pairs(source.Child(d.root, "pages")) {
		name, pg := p.Key.Value, p.Value
		secs := source.Child(pg, "sections")
		kind := source.Str(source.Child(pg, "kind"))
		if kind == "form" || kind == "view" || kind == "task" {
			switch fields := source.Child(pg, "fields"); {
			case fields == nil && secs == nil:
				c.add(p.Key, source.Pointer("pages", name), RulePage, "%s is a %s and shows no field; give its fields, or its sections", name, kind)
			case fields != nil && secs != nil:
				c.add(source.Key(pg, "sections"), source.Pointer("pages", name, "sections"), RulePage, "%s gives both fields and sections; name its fields once, in fields or in sections", name)
				continue
			}
		}
		if secs == nil {
			continue
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
