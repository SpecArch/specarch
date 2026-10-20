package generate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Manual writes the user manual (ADR-160): who uses the system and the
// pages each opens, the menus, each flow as numbered instructions, and
// each page with what a person fills in, sees and does there, and what to
// do when it refuses them. A section the specification has nothing for is
// one sentence naming the key that fills it.
func Manual(root *yaml.Node, relRoot string, impls []Implementation, state *State) string {
	d := newDoc("manual", root, relRoot, impls, state)
	info := get(root, "info")
	pages := pairs(root, "pages")
	flows := pairs(root, "flows")
	d.line("# %s: user manual", str(info, "title"))
	d.blank()
	d.para(fmt.Sprintf("Version %s of the specification: %s and %s. For the people who use the system: who may open which page, how to reach it, the tasks done across pages step by step, and what each page asks for and answers.",
		str(info, "version"), countText(len(flows), "task", "tasks"), countText(len(pages), "page", "pages")))
	d.draftNotice("manual")
	d.para(str(info, "description"))

	d.section("Who uses it")
	people := peopleRoles(root)
	if len(people) == 0 {
		d.para("The specification has no role a person holds; the roles under roles, with the permissions the pages need, say who may open each page.")
	} else {
		d.para("Each role a person holds, and the pages it opens. A person holding several roles opens the pages of each.")
		d.line("| Role | Who they are | Pages they open |")
		d.line("|---|---|---|")
		for _, r := range people {
			var opens []string
			held := roleHolds(root, r.Value)
			for _, p := range pages {
				if perm := str(p.Value, "permission"); perm != "" && perm != "public" && held[perm] {
					opens = append(opens, pageTitle(p.Key.Value, p.Value))
				}
			}
			if len(opens) == 0 {
				opens = []string{"none"}
			}
			d.line("| %s | %s | %s |", markInternal(r.Key.Value, r.Value), cell(str(r.Value, "description")), cell(strings.Join(opens, ", ")))
		}
		d.blank()
		d.explainRows(rowsOf(people))
	}
	var open []string
	for _, p := range pages {
		if str(p.Value, "permission") == "public" {
			open = append(open, pageTitle(p.Key.Value, p.Value))
		}
	}
	if len(open) > 0 {
		d.para("Anyone opens these pages, signed in or not: " + strings.Join(open, ", ") + ".")
	}

	d.section("Finding your way")
	if menus := pairs(root, "menus"); len(menus) == 0 {
		d.para("The specification has no menus; the entries a person picks a page from are written under menus.")
	} else {
		d.para("The menus, and the page each entry opens. An entry is shown only to who may open its page.")
		for _, m := range menus {
			menuLines(d, root, m.Value, 0)
		}
		d.blank()
	}

	d.section("Tasks")
	if len(flows) == 0 {
		d.para("The specification has no flows; a task a person does across pages, with who does it and each page on the way, is written under flows.")
	}
	for _, f := range flows {
		d.heading(3, f.Key.Value)
		d.para(str(f.Value, "description"))
		actor := str(f.Value, "actor")
		if desc := str(get(get(root, "roles"), actor), "description"); desc != "" {
			actor += ": " + strings.TrimSuffix(strings.TrimSpace(desc), ".")
		}
		d.para("Done by " + actor + ".")
		d.explain(f.Value)
		n := 0
		steps := items(f.Value, "steps")
		if len(steps) > 0 {
			if path := menuPath(root, str(steps[0], "page")); path != "" {
				n++
				d.line("%d. Open %s from the menu.", n, path)
			}
		}
		for _, s := range steps {
			n++
			d.line("%d. %s", n, flowInstruction(root, s))
		}
		d.blank()
	}

	d.section("Pages")
	if len(pages) == 0 {
		d.para("The specification has no pages; each screen a person opens, with its fields and actions, is written under pages.")
	}
	for _, p := range pages {
		manualPage(d, root, p.Key.Value, p.Value)
	}
	d.sourcesIndex("Sources")
	return d.String()
}

// peopleRoles are the roles a person holds: every role but one a job acts
// as or a client holds, unless a flow names it as its actor.
func peopleRoles(root *yaml.Node) []sourcePair {
	program := map[string]bool{}
	for _, j := range pairs(root, "jobs") {
		program[str(j.Value, "role")] = true
	}
	for _, c := range pairs(root, "clients") {
		for _, r := range strs(c.Value, "roles") {
			program[r] = true
		}
	}
	for _, f := range pairs(root, "flows") {
		delete(program, str(f.Value, "actor"))
	}
	var out []sourcePair
	for _, r := range pairs(root, "roles") {
		if !program[r.Key.Value] {
			out = append(out, r)
		}
	}
	return out
}

// pageTitle is the title a person reads for a page, or its name.
func pageTitle(name string, page *yaml.Node) string {
	if t := str(page, "title"); t != "" {
		return t
	}
	return name
}

// pageTitleOf is the title of a page of the specification by name.
func pageTitleOf(root *yaml.Node, name string) string {
	return pageTitle(name, get(get(root, "pages"), name))
}

// menuLines writes one menu entry and the entries under it as a nested
// list.
func menuLines(d *doc, root, item *yaml.Node, depth int) {
	indent := strings.Repeat("  ", depth)
	if page := str(item, "page"); page != "" {
		d.line("%s- %s: opens the %s page", indent, str(item, "title"), pageTitleOf(root, page))
		return
	}
	d.line("%s- %s", indent, str(item, "title"))
	for _, sub := range pairs(item, "items") {
		menuLines(d, root, sub.Value, depth+1)
	}
}

// menuPath is the way to a page through the menus, as "Members › All
// members", or "" when no menu entry opens it.
func menuPath(root *yaml.Node, page string) string {
	var find func(item *yaml.Node, path []string) string
	find = func(item *yaml.Node, path []string) string {
		path = append(path, str(item, "title"))
		if str(item, "page") == page {
			return strings.Join(path, " › ")
		}
		for _, sub := range pairs(item, "items") {
			if found := find(sub.Value, path); found != "" {
				return found
			}
		}
		return ""
	}
	for _, m := range pairs(root, "menus") {
		if found := find(m.Value, nil); found != "" {
			return found
		}
	}
	return ""
}

// flowInstruction is one step of a flow as an instruction: on which page,
// and what the person does there.
func flowInstruction(root, step *yaml.Node) string {
	name := str(step, "page")
	page := get(get(root, "pages"), name)
	on := "On the " + pageTitle(name, page) + " page, "
	switch str(step, "event") {
	case "select":
		return on + "select the row you want."
	case "submitted":
		var labels []string
		for _, f := range pageFields(root, page) {
			labels = append(labels, f.label)
		}
		if len(labels) == 0 {
			return on + "send the form."
		}
		return on + "fill in " + joinAnd(labels) + ", and send it."
	case "action":
		return on + "choose " + str(step, "action") + "."
	}
	return on + "go on."
}

// pageField is one field a page shows or asks for, as a person reads it.
type pageField struct {
	label, description string
	required           bool
}

// pageSchema is the schema of what a page shows or sends: its entity or
// view, or for a task the body of the operation it submits to.
func pageSchema(root, page *yaml.Node) *yaml.Node {
	if e := str(page, "entity"); e != "" {
		if n := get(get(root, "entities"), e); n != nil {
			return n
		}
		return get(get(root, "views"), e)
	}
	if o := findOperation(root, str(page, "submit")); o != nil {
		schema := get(get(get(get(o.node, "requestBody"), "content"), "application/json"), "schema")
		if ref := str(schema, "$ref"); strings.HasPrefix(ref, "#/") {
			parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
			if len(parts) == 2 {
				return get(get(root, parts[0]), parts[1])
			}
		}
		return schema
	}
	return nil
}

// fieldsOf is how a person reads the named fields of a schema.
func fieldsOf(schema *yaml.Node, names []string) []pageField {
	required := map[string]bool{}
	for _, r := range strs(schema, "required") {
		required[r] = true
	}
	var out []pageField
	for _, n := range names {
		f := get(get(schema, "properties"), n)
		label := str(f, "title")
		if label == "" {
			label = n
		}
		out = append(out, pageField{label: label, description: oneParagraph(str(f, "description")), required: required[n]})
	}
	return out
}

// pageFields are the fields of a page, its sections' in order when it has
// sections.
func pageFields(root, page *yaml.Node) []pageField {
	names := strs(page, "fields")
	for _, s := range items(page, "sections") {
		names = append(names, strs(s, "fields")...)
	}
	return fieldsOf(pageSchema(root, page), names)
}

// manualPage writes one page: what it is for, how a person reaches it and
// who may open it, what it shows or asks for, what can be done on it, and
// what to do when it refuses.
func manualPage(d *doc, root *yaml.Node, name string, page *yaml.Node) {
	d.heading(3, pageTitle(name, page))
	d.para(str(page, "description"))
	kind := str(page, "kind")
	var reached []string
	if path := menuPath(root, name); path != "" {
		reached = append(reached, "the menu "+path)
	}
	for _, other := range pairs(root, "pages") {
		title := pageTitle(other.Key.Value, other.Value)
		if str(get(other.Value, "onSelect"), "navigate") == name {
			reached = append(reached, "a row selected on the "+title+" page")
		}
		for _, a := range items(other.Value, "actions") {
			if str(a, "kind") == "navigate" && str(a, "target") == name {
				reached = append(reached, "the action "+str(a, "label")+" on the "+title+" page")
			}
		}
	}
	if len(reached) > 0 {
		d.para("Reached from " + joinAnd(reached) + ".")
	}
	d.para(whoOpens(root, str(page, "permission")))
	d.explain(page)

	fields := pageFields(root, page)
	switch {
	case kind == "list":
		cols := fieldsOf(pageSchema(root, page), strs(page, "columns"))
		if len(cols) > 0 {
			d.line("| Column | What it shows |")
			d.line("|---|---|")
			for _, c := range cols {
				d.line("| %s | %s |", cell(c.label), cell(c.description))
			}
			d.blank()
		}
		if filters := fieldsOf(pageSchema(root, page), strs(page, "filters")); len(filters) > 0 {
			var labels []string
			for _, f := range filters {
				labels = append(labels, f.label)
			}
			d.para("The list can be narrowed by " + joinAnd(labels) + ".")
		}
	case kind == "view":
		if len(fields) > 0 {
			d.line("| Field | What it shows |")
			d.line("|---|---|")
			for _, f := range fields {
				d.line("| %s | %s |", cell(f.label), cell(f.description))
			}
			d.blank()
		}
	case len(fields) > 0:
		d.line("| Field | What to enter | Required |")
		d.line("|---|---|---|")
		for _, f := range fields {
			req := ""
			if f.required {
				req = "yes"
			}
			d.line("| %s | %s | %s |", cell(f.label), cell(f.description), req)
		}
		d.blank()
	}
	for _, c := range items(page, "childRows") {
		var labels []string
		for _, f := range fieldsOf(get(get(root, "entities"), str(get(get(pageSchema(root, page), "relations"), str(c, "relation")), "target")), strs(c, "fields")) {
			labels = append(labels, f.label)
		}
		title := str(c, "title")
		if title == "" {
			title = str(c, "relation")
		}
		line := fmt.Sprintf("%s are added as rows, each with %s", title, joinAnd(labels))
		if m := str(c, "maximum"); m != "" {
			line += ", up to " + m
		}
		d.para(line + ".")
	}
	if checks := pairs(page, "checks"); len(checks) > 0 {
		var msgs []string
		for _, c := range checks {
			msgs = append(msgs, strings.TrimSuffix(strings.TrimSpace(str(c.Value, "message")), "."))
		}
		d.para("Checked before it is sent: " + strings.Join(msgs, "; ") + ".")
	}

	if actions := items(page, "actions"); len(actions) > 0 {
		d.para("What can be done here:")
		for _, a := range actions {
			d.line("- %s", actionText(root, page, a))
		}
		d.blank()
	}
	if after := afterSubmit(root, get(page, "onSubmitted")); after != "" {
		d.para("Once it is sent: " + after)
	}

	failed := pairs(get(page, "states"), "failed")
	if len(failed) > 0 {
		d.para("When the system refuses:")
		d.line("| You see | Why | What to do |")
		d.line("|---|---|---|")
		for _, f := range failed {
			problem := get(get(root, "errors"), f.Key.Value)
			why := str(problem, "condition")
			if f.Key.Value == "default" {
				// default is every problem type the page does not name.
				why = "Any other problem the system reports."
			}
			d.line("| %s | %s | %s |", cell(str(f.Value, "message")), cell(why), cell(str(problem, "remedy")))
		}
		d.blank()
	}
}

// whoOpens says who may open a page with a permission.
func whoOpens(root *yaml.Node, perm string) string {
	switch perm {
	case "":
		return ""
	case "public":
		return "Anyone opens it, signed in or not."
	}
	var roles []string
	for _, r := range peopleRoles(root) {
		if roleHolds(root, r.Value)[perm] {
			roles = append(roles, r.Key.Value)
		}
	}
	if len(roles) == 0 {
		return "Opened by who holds the permission " + perm + "; no role a person holds in this specification grants it."
	}
	return "Opened by " + joinAnd(roles) + " (the permission " + perm + ")."
}

// actionText is one action of a page in plain words.
func actionText(root, page, a *yaml.Node) string {
	text := "**" + str(a, "label") + "**: "
	switch str(a, "kind") {
	case "navigate":
		text += "opens the " + pageTitleOf(root, str(a, "target")) + " page."
	default:
		what := str(a, "target")
		if o := findOperation(root, what); o != nil && str(o.node, "summary") != "" {
			what = strings.TrimSuffix(str(o.node, "summary"), ".")
		}
		text += what + "."
	}
	if c := str(a, "confirm"); c != "" {
		text += " It asks first: \"" + c + "\""
		if str(a, "reason") != "" {
			text += ", and for the reason"
		}
		text += "."
	}
	if w := str(a, "when"); w != "" {
		text += " Offered only while " + w + "."
	}
	if p := str(a, "permission"); p != "" && p != str(page, "permission") {
		text += " " + whoOpens(root, p)
		text = strings.Replace(text, "Opened by", "Shown to", 1)
	}
	if m := str(get(a, "then"), "message"); m != "" {
		text += " Then: " + m
	}
	return text
}

// afterSubmit says what a page shows and opens once it is sent: one
// event, or one per answer of a task.
func afterSubmit(root, on *yaml.Node) string {
	if on == nil {
		return ""
	}
	event := func(e *yaml.Node) string {
		var parts []string
		if m := strings.TrimSpace(str(e, "message")); m != "" {
			parts = append(parts, m)
		}
		if p := str(e, "navigate"); p != "" {
			parts = append(parts, "The "+pageTitleOf(root, p)+" page opens.")
		}
		return strings.Join(parts, " ")
	}
	if get(on, "navigate") != nil || get(on, "message") != nil {
		return event(on)
	}
	var parts []string
	for _, e := range pairsOf(on) {
		if t := event(e.Value); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " Or: ")
}
