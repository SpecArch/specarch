package genui

import (
	_ "embed"
	"fmt"
	"html"
	"strings"
)

// View pages (ADR-147). A view's module is the page as data and one call
// to view.js, which draws the record its operation reads by sections, its
// values with parts as lists of them, and its actions.

//go:embed runtime/view.js
var viewRuntime string

//go:embed runtime/ask.js
var askRuntime string

// viewPage is what one view page needs.
type viewPage struct {
	name, title, upper, language, source, path, key string
	sections                                        []viewSection
	actions                                         []action
	failed                                          string
	sourceRefusals                                  [][2]string
}

type viewSection struct {
	title  string
	fields []viewField
}

// viewField is a field a view shows, or a part of a value it holds.
type viewField struct {
	name, label, hiddenWhen string
	list                    bool
	parts                   []viewField
}

// sectionsOf are a page's sections, each with its title and fields;
// fields given as a list are one section with no title.
func sectionsOf(pg map[string]any) []viewSection {
	if pg["sections"] == nil {
		var fields []viewField
		for _, f := range list(pg["fields"]) {
			fields = append(fields, viewField{name: text(f)})
		}
		return []viewSection{{fields: fields}}
	}
	var out []viewSection
	for _, s := range list(pg["sections"]) {
		var fields []viewField
		for _, f := range list(obj0(s)["fields"]) {
			fields = append(fields, viewField{name: text(f)})
		}
		out = append(out, viewSection{title: text(obj0(s)["title"]), fields: fields})
	}
	return out
}

// fieldAt is the pointer of a page's field: under its sections, or under
// fields when it lists them.
func fieldAt(at string, pg map[string]any, section, field int) string {
	if pg["sections"] != nil {
		return fmt.Sprintf("%s/sections/%d/fields/%d", at, section, field)
	}
	return fmt.Sprintf("%s/fields/%d", at, field)
}

// load reads the operation a form or a view loads its record from: a GET
// whose path's parameters are each a parameter of the page's route. path
// is the JavaScript that writes its path from the route's parameters.
func (g *gen) load(name, at string, pg map[string]any) (op map[string]any, path string, ok bool) {
	id := text(pg["source"])
	op, p, method := g.operation(id)
	if op == nil || method != "get" {
		g.problem("error", at+"/source", "%s loads its record from %s, which is not a GET operation of the specification", name, id)
		return nil, "", false
	}
	if !g.plainOnly(at+"/source", name, "loads its record from", id, op) {
		return nil, "", false
	}
	route := map[string]bool{}
	for _, r := range routeParameters(text(pg["route"])) {
		route[r] = true
	}
	parts := pathParam.Split(p, -1)
	path = jsTemplate(parts[0])
	for i, param := range routeParameters(p) {
		if !route[param] {
			g.problem("error", at+"/source", "%s loads %s at %s, and its route %s has no parameter {%s} to fill it from", name, id, p, text(pg["route"]), param)
			return nil, "", false
		}
		path += "${encodeURIComponent(parameters." + param + ")}" + jsTemplate(parts[i+1])
	}
	return op, path, true
}

// valueParts are the parts of a field that holds a value: a schema's
// properties in the order of their names, and whether it holds a list of
// them; nil parts for a field that holds no value. A list of plain values
// has no parts. within are the schemas being read, to end a cycle.
func (g *gen) valueParts(prop map[string]any, within map[string]bool) (string, map[string]any, bool) {
	many := false
	if text(prop["type"]) == "array" {
		many = true
		prop = obj0(prop["items"])
	}
	ref, ok := strings.CutPrefix(text(prop["$ref"]), "#/schemas/")
	if !ok || within[ref] {
		return "", nil, many
	}
	return ref, obj0(obj0(obj0(g.spec["schemas"])[ref])["properties"]), many
}

// viewField reads a field a view shows, with the parts of a value it holds.
func (g *gen) viewField(name string, prop map[string]any, within map[string]bool) viewField {
	f := viewField{name: name, label: orText(text(prop["title"]), name)}
	ref, parts, many := g.valueParts(prop, within)
	f.list = many
	if ref == "" {
		return f
	}
	inner := map[string]bool{ref: true}
	for k := range within {
		inner[k] = true
	}
	for _, p := range sortedKeys(parts) {
		f.parts = append(f.parts, g.viewField(p, obj0(parts[p]), inner))
	}
	if f.parts == nil {
		f.parts = []viewField{}
	}
	return f
}

// view reads a view page, or reports why it cannot be written.
func (g *gen) view(name string, pg map[string]any) *viewPage {
	at := "/pages/" + name
	entity := obj0(obj0(g.spec["entities"])[text(pg["entity"])])
	props := obj0(entity["properties"])
	op, path, ok := g.load(name, at, pg)
	if !ok {
		return nil
	}
	v := &viewPage{name: name, title: text(pg["title"]), upper: upperSnake(name), language: text(g.settings["language"]),
		source: text(pg["source"]), path: path}
	if pk := list(entity["primaryKey"]); len(pk) == 1 {
		v.key = text(pk[0])
	}
	states := obj0(pg["states"])
	failed := obj0(states["failed"])
	v.failed = orText(text(obj0(failed["default"])["message"]), defaultFailed)
	v.sourceRefusals = refusals(op, failed)
	conditions := obj0(pg["fieldConditions"])
	for i, s := range sectionsOf(pg) {
		section := viewSection{title: s.title}
		for j, f := range s.fields {
			prop, ok := obj(props[f.name])
			if !ok {
				g.problem("error", fieldAt(at, pg, i, j), "%s shows %s, which is not a field of %s; it is left out", name, f.name, text(pg["entity"]))
				continue
			}
			field := g.viewField(f.name, prop, nil)
			if e := text(obj0(conditions[f.name])["hiddenWhen"]); e != "" {
				rule, ok := g.rule(e, at+"/fieldConditions/"+f.name+"/hiddenWhen", "the field's hiddenWhen", props)
				if !ok {
					continue
				}
				field.hiddenWhen = rule
			}
			section.fields = append(section.fields, field)
		}
		v.sections = append(v.sections, section)
	}
	own := routeParameters(text(pg["route"]))
	if own == nil {
		own = []string{}
	}
	for i, a := range list(pg["actions"]) {
		am := obj0(a)
		ptr := fmt.Sprintf("%s/actions/%d", at, i)
		if text(am["kind"]) != "navigate" {
			if act, ok := g.operationAction(ptr, am, props, v.key, failed, own, "record"); ok {
				v.actions = append(v.actions, act)
			}
			continue
		}
		label, target := text(am["label"]), text(am["target"])
		if _, ok := obj(obj0(g.spec["pages"])[target]); !ok {
			g.problem("error", ptr, "the action %s opens %s, which is not a page of the specification", label, target)
			continue
		}
		with, ok := g.withJS(ptr, "the action "+label, target, obj0(am["with"]), props, own, "record")
		if !ok {
			continue
		}
		l := link{label: label, page: target, with: with, at: ptr}
		if w := text(am["when"]); w != "" {
			if l.when, ok = g.rule(w, ptr+"/when", "the action's when", props); !ok {
				continue
			}
		}
		v.actions = append(v.actions, action{label: label, link: &l, when: l.when})
	}
	return v
}

// lowerTitle is the page's title in lower case, safe inside a comment.
func (v *viewPage) lowerTitle() string {
	return strings.ReplaceAll(strings.ToLower(v.title), "*/", "* /")
}

func (v *viewPage) events() []event {
	return append([]event{
		{v.upper + "_LOADED", "The " + v.lowerTitle() + " arrived: { record }"},
		{v.upper + "_FAILED", "The " + v.lowerTitle() + " could not be read: { status }"},
	}, actionEvents(v.actions)...)
}

// html writes the view page's HTML: its sections, each field's label and
// the place of its value, which view.js fills.
func (v *viewPage) html(g *gen) string {
	out := g.head(v.name, v.title, v.language)
	out = append(out, `<p id="status" role="status" aria-live="polite"></p>`, `<div id="record">`)
	for i, s := range v.sections {
		if s.title != "" {
			id := fmt.Sprintf("section-%d", i)
			out = append(out, `<section aria-labelledby="`+id+`">`, `<h2 id="`+id+`">`+html.EscapeString(s.title)+"</h2>")
		} else {
			out = append(out, "<section>")
		}
		out = append(out, placesHTML(s.fields)...)
		out = append(out, "</section>")
	}
	out = append(out, "</div>", `<div id="actions"></div>`, "</main>", "</body>", "</html>")
	return strings.Join(out, "\n") + "\n"
}

// placesHTML writes fields as a description list: each label, and the
// place of its value; a value's parts inside its place, a list's item as
// the template of its entries.
func placesHTML(fields []viewField) []string {
	out := []string{"<dl>"}
	for _, f := range fields {
		start := "<div><dt>" + html.EscapeString(f.label) + "</dt><dd data-field=" + quoted(f.name)
		switch {
		case f.list:
			out = append(out, start+" data-list>", "<ul></ul>", "<template>")
			if f.parts == nil {
				out = append(out, "<li data-item></li>")
			} else {
				out = append(out, "<li data-item>")
				out = append(out, placesHTML(f.parts)...)
				out = append(out, "</li>")
			}
			out = append(out, "</template>", "</dd></div>")
		case f.parts != nil:
			out = append(out, start+">")
			out = append(out, placesHTML(f.parts)...)
			out = append(out, "</dd></div>")
		default:
			out = append(out, start+"></dd></div>")
		}
	}
	return append(out, "</dl>")
}

// article puts a or an before a noun, by its first letter.
func article(noun string) string {
	if noun != "" && strings.ContainsRune("aeiou", rune(noun[0])) {
		return "an " + noun
	}
	return "a " + noun
}

// sentence joins clauses as a sentence does: a, b, and c.
func sentence(clauses []string) string {
	switch len(clauses) {
	case 0:
		return ""
	case 1:
		return clauses[0]
	case 2:
		return clauses[0] + " and " + clauses[1]
	}
	return strings.Join(clauses[:len(clauses)-1], ", ") + ", and " + clauses[len(clauses)-1]
}

// js writes the view page's module.
func (v *viewPage) js() string {
	var b strings.Builder
	w := func(s string) { b.WriteString(s + "\n") }
	clauses := []string{"shows " + article(v.lowerTitle()) + " read through " + v.source}
	var opens, runs []string
	for _, a := range v.actions {
		if a.link != nil {
			opens = append(opens, a.link.page)
		} else {
			runs = append(runs, a.operation)
		}
	}
	if len(opens) > 0 {
		clauses = append(clauses, "opens "+joinAnd(opens))
	}
	if len(runs) > 0 {
		clauses = append(clauses, "runs "+joinAnd(runs)+" on it")
	}
	for _, l := range wrap("The page "+v.name+": "+v.title+". It "+sentence(clauses)+".", 72-3) {
		w("// " + l)
	}
	w("")
	w("import {")
	w("  " + v.upper + "_LOADED, " + v.upper + "_FAILED,")
	ops := operations(v.actions)
	for _, a := range ops {
		u := upperSnake(a.operation)
		w("  " + u + "_SUCCEEDED, " + u + "_FAILED,")
	}
	w(`} from "./events.js";`)
	w(`import { view } from "./view.js";`)
	w("")
	w("view({")
	if v.key != "" {
		w("  key: " + jsString(v.key) + ",")
	}
	w("  source: { operation: " + jsString(v.source) + ", path: (parameters) => `" + v.path + "` },")
	var hidden []viewField
	for _, sec := range v.sections {
		for _, f := range sec.fields {
			if f.hiddenWhen != "" {
				hidden = append(hidden, f)
			}
		}
	}
	if len(hidden) == 0 {
		w("  hidden: {},")
	} else {
		w("  // The fields hidden while their condition holds.")
		w("  hidden: {")
		for _, f := range hidden {
			w("    " + f.name + ": " + f.hiddenWhen + ",")
		}
		w("  },")
	}
	w("  actions: [")
	for _, a := range v.actions {
		actionJS(w, a, "    ", "record")
	}
	w("  ],")
	w("  messages: {")
	w("    failed: " + jsString(v.failed) + ",")
	w("  },")
	w("  // What each operation's refusals say, by status: the page's failed states.")
	w("  refusals: {")
	indented := func(s string) { w("  " + s) }
	writeRefusals(indented, v.source, v.sourceRefusals)
	seen := map[string]bool{v.source: true}
	for _, a := range ops {
		if !seen[a.operation] {
			seen[a.operation] = true
			writeRefusals(indented, a.operation, a.refusals)
		}
	}
	w("  },")
	w("  loaded: " + v.upper + "_LOADED,")
	w("  failed: " + v.upper + "_FAILED,")
	w("});")
	return b.String()
}
