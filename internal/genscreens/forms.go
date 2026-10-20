package genscreens

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/wirename"
)

// The layouts a form's sections may take, the ui target's settings.layouts.
var layouts = map[string]bool{"page": true, "tabs": true, "steps": true}

// FormSettings reads what the ui target's settings say of forms: the
// fields whose starting value a hook gives, and how each form's sections
// sit, by page. Each names a form or a view of the specification, and each
// hook a field it shows; a mistake is an error under settings.
func (g *Gen) FormSettings() (map[string][]string, map[string]string) {
	pages := obj0(g.Spec["pages"])
	hooks := map[string][]string{}
	hookSettings := obj0(g.Settings["hooks"])
	for _, pageName := range sortedKeys(hookSettings) {
		pg, ok := obj(pages[pageName])
		if !ok || text(pg["kind"]) != "form" {
			g.Problem("error", "/", "the ui target's settings.hooks name %s, which is not a form of the specification; a hook gives the starting value of a form's field", pageName)
			continue
		}
		shown := map[string]bool{}
		for _, f := range shownFields(pg) {
			shown[f] = true
		}
		for _, f := range list(hookSettings[pageName]) {
			if !shown[text(f)] {
				g.Problem("error", "/", "the ui target's settings.hooks give %s a hook for %s, which it does not show", pageName, text(f))
				continue
			}
			hooks[pageName] = append(hooks[pageName], text(f))
		}
	}
	chosen := map[string]string{}
	layoutSettings := obj0(g.Settings["layouts"])
	for _, pageName := range sortedKeys(layoutSettings) {
		pg, ok := obj(pages[pageName])
		layout := text(layoutSettings[pageName])
		switch {
		case !ok || text(pg["kind"]) != "form":
			g.Problem("error", "/", "the ui target's settings.layouts name %s, which is not a form of the specification", pageName)
		case !layouts[layout]:
			g.Problem("error", "/", "the ui target's settings.layouts give %s the layout %q; a form's sections sit as page, tabs or steps", pageName, layout)
		case layout != "page" && len(list(pg["sections"])) < 2:
			g.Problem("error", "/", "the ui target's settings.layouts give %s %s, and it has fewer than two sections to lay out so; give it sections, or leave the layout out", pageName, layout)
		default:
			chosen[pageName] = layout
		}
	}
	return hooks, chosen
}

// shownFields are the fields a page shows, from fields or its sections.
func shownFields(pg map[string]any) []string {
	var out []string
	for _, f := range list(pg["fields"]) {
		out = append(out, text(f))
	}
	for _, s := range list(pg["sections"]) {
		for _, f := range list(obj0(s)["fields"]) {
			out = append(out, text(f))
		}
	}
	return out
}

// sectionsOf are a page's sections, each with its title and fields; fields
// given as a list are one section with no title.
func sectionsOf(pg map[string]any) []struct {
	title  string
	fields []string
} {
	type section = struct {
		title  string
		fields []string
	}
	if pg["sections"] == nil {
		var fields []string
		for _, f := range list(pg["fields"]) {
			fields = append(fields, text(f))
		}
		return []section{{fields: fields}}
	}
	var out []section
	for _, s := range list(pg["sections"]) {
		var fields []string
		for _, f := range list(obj0(s)["fields"]) {
			fields = append(fields, text(f))
		}
		out = append(out, section{title: text(obj0(s)["title"]), fields: fields})
	}
	return out
}

// propertyType is the JSON Schema type of a property, its null left out,
// read through an enum it refers to.
func (g *Gen) propertyType(prop map[string]any) string {
	if ref, ok := strings.CutPrefix(text(prop["$ref"]), "#/enums/"); ok {
		prop = obj0(obj0(g.Spec["enums"])[ref])
	}
	if types := list(prop["type"]); types != nil {
		for _, t := range types {
			if text(t) != "null" {
				return text(t)
			}
		}
		return ""
	}
	return text(prop["type"])
}

// load reads the operation a form or a view loads its record from: a GET
// whose path's parameters are each a parameter of the page's route.
func (g *Gen) load(pageName, at string, pg map[string]any) (Value, bool) {
	sourceID := text(pg["source"])
	op, path, method := g.Operation(sourceID)
	if op == nil || method != "get" {
		g.Problem("error", at+"/source", "%s loads its record from %s, which is not a GET operation of the specification", pageName, sourceID)
		return nil, false
	}
	if !g.PlainOnly(at+"/source", pageName, "loads its record from", sourceID, op) {
		return nil, false
	}
	route := map[string]bool{}
	for _, p := range PathParam.FindAllString(text(pg["route"]), -1) {
		route[p[1:len(p)-1]] = true
	}
	for _, p := range PathParam.FindAllString(path, -1) {
		if !route[p[1:len(p)-1]] {
			g.Problem("error", at+"/source", "%s loads %s at %s, and its route %s has no parameter %s to fill it from", pageName, sourceID, path, text(pg["route"]), p)
			return nil, false
		}
	}
	failed := obj0(obj0(pg["states"])["failed"])
	return Object(append(append([]Member{{"operation", Str(sourceID)}, {"path", Str(path)}}, g.Transport(op, path)...), Member{"failed", Array(g.refusalsOf(pageName, op, failed))})), true
}

// ordersDecimal names a field of format decimal that an expression orders
// with <, <=, > or >=, which a browser cannot do exactly; "" when none.
func (g *Gen) ordersDecimal(n *expr.Node, props map[string]any) string {
	if n == nil {
		return ""
	}
	switch n.Op {
	case "<", "<=", ">", ">=":
		for _, a := range n.Args {
			if a.Op == expr.OpName && text(obj0(props[a.Text])["format"]) == "decimal" {
				return a.Text
			}
		}
	}
	for _, a := range n.Args {
		if found := g.ordersDecimal(a, props); found != "" {
			return found
		}
	}
	return ""
}

// condition turns a page's expression into a rule over the record's
// fields, refusing what the browser cannot evaluate.
func (g *Gen) condition(expression, at, what string, props map[string]any, wires map[string]string) (Value, bool) {
	node, errs := expr.Parse(expression)
	if len(errs) > 0 {
		g.Problem("error", at, "%s does not parse: %s", what, errs[0].Message)
		return nil, false
	}
	if f := g.ordersDecimal(node, props); f != "" && !g.Profile.ComparesDecimals {
		g.Problem("error", at, "%s orders %s, a decimal, which %s cannot compare exactly; check it on the server", what, f, g.Profile.Where)
		return nil, false
	}
	return g.Rule(node, wires, at)
}

// formField is one field of a form, drawn by the part its property's type
// and format take, or by the lookup part when the page picks it.
type formFieldInput struct {
	pageName, at, name string
	prop               map[string]any
	required, readOnly bool
	conditions         map[string]any
	picker             map[string]any
	hooked, twice      bool
	props              map[string]any
	wires              map[string]string
	entity             map[string]any
	// labelAt is the string key of the label, when it is not the page's
	// field of the name, as a child row's field is not.
	labelAt string
	// path names a part of a value by the path to it, address.street, in
	// messages; the field's name when it is no part.
	path string
	// inSections is set for a field of a form's sections or a part of a
	// value among them, where a value may be drawn.
	inSections bool
	// within are the schemas whose parts hold this field, to end a cycle.
	within map[string]bool
}

func (g *Gen) formField(in formFieldInput) (Value, bool) {
	prop := in.prop
	shown := orText(in.path, in.name)
	label := text(prop["title"])
	if label == "" {
		g.Problem("warning", in.at, "%s has no title, so its label is its name; give the property a title", shown)
		label = in.name
	}
	if schemaName, many := ValueOf(prop); schemaName != "" {
		part, data, ok := g.valueField(in, schemaName, many, label)
		if !ok {
			return nil, false
		}
		return g.fieldRules(in, part, data, label)
	}
	t := g.propertyType(prop)
	format := text(prop["format"])
	var part string
	switch {
	case in.picker != nil:
		part = "lookup-field"
	case g.EnumValues(prop) != nil:
		part = "select-field"
	case t == "boolean":
		part = "checkbox-field"
	case t == "integer" || t == "number":
		part = "number-field"
	case t != "string":
		g.Problem("error", in.at, "%s shows %s, of type %s, and this version of %s writes a form's fields of type string, integer, number or boolean, or holding a schema", in.pageName, shown, orText(t, "none"), g.Profile.Name)
		return nil, false
	case format == "date":
		part = "date-field"
	case format == "email":
		part = "email-field"
	case format == "password":
		part = "password-field"
	case format == "" || format == "uuid" || format == "decimal" || format == "uri":
		part = "text-field"
		if m, err := strconv.Atoi(text(prop["maxLength"])); format == "" && (err != nil || m > 255) {
			part = "text-area-field"
		}
	default:
		g.Problem("error", in.at, "%s shows %s, of format %s, which this version of %s does not write on a form; show it on a view", in.pageName, shown, format, g.Profile.Name)
		return nil, false
	}
	wire := wirename.Of(g.WireNames, in.name)
	labelAt := g.labelKey(in)
	data := []Member{
		{"type", Str(g.Name(part, "type"))},
		{g.Name(part, "name"), Str(wire)},
		{g.Name(part, "label"), Str(g.Say(labelAt, label))},
		{g.Name(part, "required"), Raw(strconv.FormatBool(in.required))},
	}
	switch part {
	case "text-field", "password-field":
		var rules []Value
		for _, kw := range []string{"minLength", "maxLength", "pattern"} {
			v, ok := prop[kw]
			if !ok {
				continue
			}
			if kw == "pattern" {
				data = append(data, Member{g.Name(part, kw), Str(text(v))})
				rules = append(rules, Object([]Member{{"text", Str("screens.rule.pattern")}, {"pattern", Str(text(v))}}))
				continue
			}
			data = append(data, Member{g.Name(part, kw), Raw(text(v))})
			rules = append(rules, Object([]Member{{"text", Str("screens." + kw)}, {"count", Raw(text(v))}}))
		}
		if part == "password-field" {
			data = append(data, Member{g.Name(part, "rules"), Array(rules)})
		}
	case "text-area-field", "email-field":
		if v, ok := prop["maxLength"]; ok {
			data = append(data, Member{g.Name(part, "maxLength"), Raw(text(v))})
		}
	case "number-field":
		for _, kw := range []string{"minimum", "maximum"} {
			if v, ok := prop[kw]; ok {
				data = append(data, Member{g.Name(part, kw), Raw(text(v))})
			}
		}
		if t == "integer" {
			data = append(data, Member{g.Name(part, "step"), Raw("1")})
		}
	case "select-field":
		var opts []Value
		for _, v := range g.EnumValues(prop) {
			opts = append(opts, Str(v))
		}
		data = append(data, Member{g.Name(part, "options"), Array(opts)})
	case "lookup-field":
		lookup, ok := g.lookup(in)
		if !ok {
			return nil, false
		}
		for _, m := range lookup {
			data = append(data, Member{g.Name(part, m.Key), m.Val})
		}
	}
	return g.fieldRules(in, part, data, label)
}

// labelKey is the string key of a field's label: the page's field of its
// name, or the key the caller gives, as a child row's field and a value's
// part have.
func (g *Gen) labelKey(in formFieldInput) string {
	return orText(in.labelAt, in.pageName+".fields."+in.name)
}

// fieldRules adds what a field's conditions, hook and second entry say to
// the data its part takes.
func (g *Gen) fieldRules(in formFieldInput, part string, data []Member, label string) (Value, bool) {
	if in.readOnly {
		data = append(data, Member{g.Name(part, "readOnly"), Raw("true")})
	}
	for _, k := range []string{"readOnlyWhen", "hiddenWhen"} {
		e := text(in.conditions[k])
		if e == "" {
			continue
		}
		rule, ok := g.condition(e, in.at+"/"+k, "the field's "+k, in.props, in.wires)
		if !ok {
			return nil, false
		}
		data = append(data, Member{g.Name(part, k), rule})
	}
	if in.hooked {
		data = append(data, Member{g.Name(part, "hook"), Raw("true")})
	}
	if in.twice {
		data = append(data, Member{g.Name(part, "twice"), Str(g.Say(in.pageName+".fields."+in.name+".again", label+" again"))})
	}
	return Object(data), true
}

// lookup reads a field's picker: the list operation it reads, the key of
// the record picked, the fields shown to choose by and those it fills.
func (g *Gen) lookup(in formFieldInput) ([]Member, bool) {
	at := "/pages/" + in.pageName + "/pickers/" + in.name
	var target string
	for _, r := range obj0(in.entity["relations"]) {
		rel := obj0(r)
		if text(rel["kind"]) == "many-to-one" && text(rel["via"]) == in.name {
			target = text(rel["target"])
		}
	}
	targetEntity := obj0(obj0(g.Spec["entities"])[target])
	pk := list(targetEntity["primaryKey"])
	if target == "" || len(pk) != 1 {
		g.Problem("error", at, "%s picks %s through no many-to-one relation to a record with a primary key of one field, so the picked record has no one value to hold", in.pageName, in.name)
		return nil, false
	}
	sourceID := text(in.picker["source"])
	op, path, method := g.Operation(sourceID)
	listOf := obj0(op["listOf"])
	size := obj0(listOf["pageSize"])
	pageSize := orText(text(size["maximum"]), text(size["default"]))
	wireNames := g.ListNames()
	switch {
	case op == nil || method != "get":
		g.Problem("error", at+"/source", "the picker of %s reads %s, which is not a GET operation of the specification", in.name, sourceID)
		return nil, false
	case PathParam.MatchString(path):
		g.Problem("error", at+"/source", "the picker of %s reads %s at %s, whose path takes a parameter a picker cannot give", in.name, sourceID, path)
		return nil, false
	case listOf == nil || pageSize == "":
		g.Problem("error", at+"/source", "the picker of %s reads %s, which does not page; give it listOf with a pageSize", in.name, sourceID)
		return nil, false
	case wireNames["items"] == "" || wireNames["pageSize"] == "":
		g.Problem("error", at+"/source", "the picker of %s reads a page through the paginated-list idiom, which the implementation file excludes; keep the idiom", in.name)
		return nil, false
	case !g.PlainOnly(at+"/source", "the picker of "+in.name, "reads", sourceID, op):
		return nil, false
	}
	source := append([]Member{{"operation", Str(sourceID)}, {"path", Str(path)}}, g.Transport(op, path)...)
	source = append(source, Member{"items", Str(wireNames["items"])},
		Member{"pageSize", Object([]Member{{"name", Str(wireNames["pageSize"])}, {"value", Raw(pageSize)}})})
	if g.Dialect == "dxlib" && wireNames["page"] != "" {
		source = append(source, Member{"page", Str(wireNames["page"])})
	}
	if len(list(listOf["searchable"])) > 0 && wireNames["search"] != "" {
		source = append(source, Member{"search", Str(wireNames["search"])})
	} else {
		g.Problem("warning", at+"/source", "the picker of %s reads %s, which searches no field, so it offers the first %s records and finds among them only", in.name, sourceID, pageSize)
	}
	var shows []Value
	for _, s := range list(in.picker["shows"]) {
		shows = append(shows, Str(wirename.Of(g.WireNames, text(s))))
	}
	fills := []Member{}
	fillMap := obj0(in.picker["fills"])
	for _, into := range sortedKeys(fillMap) {
		fills = append(fills, Member{wirename.Of(g.WireNames, into), Str(wirename.Of(g.WireNames, text(fillMap[into])))})
	}
	return []Member{
		{"source", Object(source)},
		{"value", Str(wirename.Of(g.WireNames, text(pk[0])))},
		{"text", Array(shows)},
		{"fills", Dictionary(fills)},
	}, true
}

// FormPage reads a form.
func (g *Gen) FormPage(pageName string, pg map[string]any, sess *Session, hooks []string, layout string) (*Page, bool) {
	at := "/pages/" + pageName
	submit := text(pg["submit"])
	before := len(g.Diags)
	if text(pg["enabledBy"]) != "" {
		g.Problem("error", at+"/enabledBy", "%s is switched on by %s, and this version of %s does not read configuration, so the page would be served while it is off", pageName, text(pg["enabledBy"]), g.Profile.Name)
	}
	if pg["actions"] != nil {
		g.Problem("error", at+"/actions", "%s has actions, and this version of %s does not write a form's actions yet", pageName, g.Profile.Name)
	}
	permission := text(pg["permission"])
	if permission != "public" && sess == nil {
		g.Problem("error", at+"/permission", "%s needs %s, and the ui target's settings name no session to read who holds it; add settings.session with the operation that answers it", pageName, permission)
	}
	entity := obj0(obj0(g.Spec["entities"])[text(pg["entity"])])
	props := obj0(entity["properties"])
	wires := map[string]string{}
	for f := range props {
		wires[f] = wirename.Of(g.WireNames, f)
	}
	op, path, method := g.Operation(submit)
	if op == nil || method == "get" {
		g.Problem("error", at+"/submit", "%s submits to %s, which is not an operation of the specification that takes a request body", pageName, submit)
		return nil, false
	}
	if !g.PlainOnly(at+"/submit", pageName, "submits to", submit, op) {
		return nil, false
	}
	var source Value
	if pg["source"] != nil {
		var ok bool
		if source, ok = g.load(pageName, at, pg); !ok {
			return nil, false
		}
	}
	route := map[string]bool{}
	for _, p := range PathParam.FindAllString(text(pg["route"]), -1) {
		route[p[1:len(p)-1]] = true
	}
	for _, p := range PathParam.FindAllString(path, -1) {
		param := p[1 : len(p)-1]
		if !route[param] && (pg["source"] == nil || props[param] == nil) {
			g.Problem("error", at+"/submit", "%s submits to %s at %s, and neither its route nor a record it loads gives %s", pageName, submit, path, p)
		}
	}
	submitMembers := append([]Member{{"operation", Str(submit)}, {"method", Str(strings.ToUpper(method))}, {"path", Str(path)}}, g.Transport(op, path)...)
	for _, prm := range list(op["parameters"]) {
		pm := obj0(prm)
		if in := text(pm["in"]); in != "path" && pm["required"] == true && text(pm["name"]) != text(op["idempotencyKey"]) {
			g.Problem("error", at+"/submit", "%s requires the %s parameter %s, which a form does not send", submit, in, text(pm["name"]))
		}
	}
	body := g.BodySchema(op)
	bodyProps := obj0(body["properties"])
	required := map[string]bool{}
	for _, r := range list(body["required"]) {
		required[text(r)] = true
	}
	shown := map[string]bool{}
	for _, f := range shownFields(pg) {
		shown[f] = true
	}
	var sent []Value
	for _, f := range shownFields(pg) {
		if bodyProps[f] != nil {
			sent = append(sent, Str(wirename.Of(g.WireNames, f)))
		}
	}
	submitMembers = append(submitMembers, Member{"fields", Array(sent)})
	rows := g.childRows(pageName, pg, entity, bodyProps, required, shown)
	if key := text(op["idempotencyKey"]); key != "" {
		submitMembers = append(submitMembers, Member{"idempotencyKey", Str(key)})
	}
	for _, r := range sortedKeys(anyMapBool(required)) {
		if !shown[r] {
			g.Problem("error", at, "%s submits to %s, whose request body requires %s, which the form does not show", pageName, submit, r)
		}
	}
	conditions := obj0(pg["fieldConditions"])
	pickers := obj0(pg["pickers"])
	hooked := map[string]bool{}
	for _, f := range hooks {
		hooked[f] = true
	}
	twice := map[string]bool{}
	for _, f := range list(pg["enteredTwice"]) {
		twice[text(f)] = true
	}
	var sections []Value
	for i, s := range sectionsOf(pg) {
		var fields []Value
		for j, f := range s.fields {
			ptr := fmt.Sprintf("%s/fields/%d", at, j)
			if pg["sections"] != nil {
				ptr = fmt.Sprintf("%s/sections/%d/fields/%d", at, i, j)
			}
			prop := obj0(props[f])
			if prop == nil {
				g.Problem("error", ptr, "%s shows %s, which is not a field of %s", pageName, f, text(pg["entity"]))
				continue
			}
			cond := obj0(conditions[f])
			readOnly := prop["readOnly"] == true || cond["readOnly"] == true
			if bodyProps[f] == nil && !readOnly {
				g.Problem("error", ptr, "%s shows %s, which the request body of %s does not take; add it to the body, or make it read-only under fieldConditions", pageName, f, submit)
				continue
			}
			fv, ok := g.formField(formFieldInput{pageName: pageName, at: ptr, name: f, prop: prop, required: required[f] && !readOnly, readOnly: readOnly,
				conditions: cond, picker: obj0(pickers[f]), hooked: hooked[f], twice: twice[f], props: props, wires: wires, entity: entity, inSections: true})
			if ok {
				fields = append(fields, fv)
			}
		}
		section := []Member{}
		if s.title != "" {
			section = append(section, Member{"title", Str(g.Say(fmt.Sprintf("%s.sections.%d", pageName, i), s.title))})
		}
		section = append(section, Member{"fields", Array(fields)})
		sections = append(sections, Object(section))
	}
	var checks []Value
	checksMap := obj0(pg["checks"])
	for _, key := range sortedKeys(checksMap) {
		c := obj0(checksMap[key])
		ptr := at + "/checks/" + key
		rule, ok := g.condition(text(c["expression"]), ptr+"/expression", "the check "+key, props, wires)
		if !ok {
			continue
		}
		entry := []Member{{"name", Str(key)}, {"rule", rule}, {"message", Str(g.Say(pageName+".checks."+key, text(c["message"])))}}
		if f := text(c["field"]); f != "" {
			entry = append(entry, Member{"field", Str(wireOf(wires, f))})
		}
		checks = append(checks, Object(entry))
	}
	routes := map[string]string{}
	events := []Value{}
	ev, ok := obj(pg["onSubmitted"])
	if !ok {
		g.Problem("error", at+"/onSubmitted", "%s says nothing of what follows a success, so the page could not tell the person what happened; add onSubmitted", pageName)
	} else {
		entry := []Member{}
		// A request that waits for approval is answered 202, and the event
		// is the page's pending state: what it says and where it leads.
		for _, w := range obj0(g.Spec["workflows"]) {
			if text(obj0(w)["trigger"]) == submit {
				entry = append(entry, Member{"status", Raw("202")})
				break
			}
		}
		if target := text(ev["navigate"]); target != "" {
			tp, found := obj(obj0(g.Spec["pages"])[target])
			if !found {
				g.Problem("error", at+"/onSubmitted/navigate", "%s leads to %s, which is not a page of the specification", pageName, target)
			}
			routes[target] = text(tp["route"])
			entry = append(entry, Member{"navigate", Str(target)})
			if with := obj0(ev["with"]); len(with) > 0 {
				var ws []Member
				for _, k := range sortedKeys(with) {
					ws = append(ws, Member{k, Str(wirename.Of(g.WireNames, text(with[k])))})
				}
				entry = append(entry, Member{"with", Dictionary(ws)})
			}
		}
		if m := text(ev["message"]); m != "" {
			entry = append(entry, Member{"message", Str(g.Say(pageName+".onSubmitted", m))})
		}
		events = append(events, Object(entry))
	}
	if g.FailedSince(before) {
		return nil, false
	}
	part := "form-page"
	schemaMembers := []Member{}
	add := func(role string, v Value) { schemaMembers = append(schemaMembers, Member{g.Name(part, role), v}) }
	add("title", Str(g.Say(pageName+".title", text(pg["title"]))))
	add("permission", Str(permission))
	if source != nil {
		add("source", source)
	}
	add("submit", Object(submitMembers))
	add("layout", Str(orText(layout, "page")))
	add("sections", Array(sections))
	add("rows", Array(rows))
	add("checks", Array(checks))
	add("failed", Array(g.refusalsOf(pageName, op, obj0(obj0(pg["states"])["failed"]))))
	add("events", Array(events))
	if g.FailedSince(before) {
		return nil, false
	}
	return &Page{Name: pageName, Kind: "form", Part: part, Route: text(pg["route"]), Permission: permission, Schema: schemaMembers, Routes: routes, Hooks: hooks}, true
}

// childRows reads a form's child rows: the body's array property of the
// relation's name that takes them, the property of the loaded record that
// holds the loaded ones, and each row's fields, checked by the target
// entity's keywords; a field the array's items do not take is shown and
// never sent. The relation's name is taken off required and marked shown,
// since the rows send it.
func (g *Gen) childRows(pageName string, pg, entity, bodyProps map[string]any, required, shown map[string]bool) []Value {
	at := "/pages/" + pageName + "/childRows"
	out := []Value{}
	entities := obj0(g.Spec["entities"])
	for i, r := range list(pg["childRows"]) {
		row := obj0(r)
		ptr := fmt.Sprintf("%s/%d", at, i)
		rel := text(row["relation"])
		target := text(obj0(obj0(entity["relations"])[rel])["target"])
		targetEntity := obj0(entities[target])
		targetProps := obj0(targetEntity["properties"])
		wire := wirename.Of(g.WireNames, rel)
		arr := obj0(bodyProps[rel])
		items := obj0(arr["items"])
		if ref, ok := strings.CutPrefix(text(items["$ref"]), "#/entities/"); ok {
			items = obj0(entities[ref])
		}
		if g.propertyType(arr) != "array" || g.propertyType(items) != "object" {
			g.Problem("error", ptr+"/relation", "%s edits the rows of %s, and the request body of %s has no array of objects named %s to send them in; add it to the body", pageName, rel, text(pg["submit"]), rel)
			continue
		}
		shown[rel] = true
		itemProps := obj0(items["properties"])
		itemRequired := map[string]bool{}
		for _, q := range list(items["required"]) {
			itemRequired[text(q)] = true
		}
		var loaded string
		if pg["source"] != nil {
			if row["lockLoadedRows"] != true {
				g.Problem("error", ptr, "%s loads the rows of %s and may change them, and this version of %s sends only the rows added; lock the loaded rows with lockLoadedRows", pageName, rel, g.Profile.Name)
				continue
			}
			if loaded = g.loadedRows(text(pg["source"]), rel); loaded == "" {
				g.Problem("error", ptr+"/relation", "%s loads its record from %s, whose answer carries no rows of %s, so the loaded rows cannot be shown; answer a view that adds rows: %s", pageName, text(pg["source"]), rel, rel)
				continue
			}
		}
		shownRow := map[string]bool{}
		var fields, sent []Value
		for j, fv := range list(row["fields"]) {
			f := text(fv)
			shownRow[f] = true
			prop := obj0(targetProps[f])
			if prop == nil {
				continue // the validator reports a field the target lacks
			}
			readOnly := prop["readOnly"] == true || itemProps[f] == nil
			field, ok := g.formField(formFieldInput{pageName: pageName, at: fmt.Sprintf("%s/fields/%d", ptr, j), name: f, prop: prop,
				required: itemRequired[f] && !readOnly, readOnly: readOnly, entity: targetEntity, labelAt: pageName + ".rows." + rel + ".fields." + f})
			if !ok {
				continue
			}
			fields = append(fields, field)
			if !readOnly {
				sent = append(sent, Str(wirename.Of(g.WireNames, f)))
			}
		}
		for _, q := range sortedKeys(anyMapBool(itemRequired)) {
			if !shownRow[q] {
				g.Problem("error", ptr+"/fields", "the rows of %s are sent to %s, whose items require %s, which the rows do not show", rel, text(pg["submit"]), q)
			}
		}
		entry := []Member{{"name", Str(wire)}}
		if loaded != "" {
			entry = append(entry, Member{"loaded", Str(loaded)})
		}
		entry = append(entry,
			Member{"title", Str(g.Say(pageName+".rows."+rel, text(row["title"])))},
			Member{"fields", Array(fields)},
			Member{"sent", Array(sent)})
		if m := text(row["maximum"]); m != "" {
			entry = append(entry, Member{"maximum", Raw(m)})
		}
		if m := text(arr["minItems"]); m != "" && m != "0" {
			entry = append(entry, Member{"minimum", Raw(m)})
		}
		entry = append(entry, Member{"lockLoaded", Raw(strconv.FormatBool(row["lockLoadedRows"] == true))})
		out = append(out, Object(entry))
	}
	return out
}

// loadedRows is the property of an operation's answer that carries the
// rows of a relation: a property of the view it answers that adds rows of
// it, on the wire; "" when it carries none.
func (g *Gen) loadedRows(sourceID, relation string) string {
	op, _, _ := g.Operation(sourceID)
	schema := obj0(obj0(obj0(obj0(obj0(op["responses"])["200"])["content"])["application/json"])["schema"])
	view, ok := strings.CutPrefix(text(schema["$ref"]), "#/views/")
	if !ok {
		return ""
	}
	props := obj0(obj0(obj0(g.Spec["views"])[view])["properties"])
	for _, p := range sortedKeys(props) {
		if text(obj0(props[p])["rows"]) == relation {
			return wirename.Of(g.WireNames, p)
		}
	}
	return ""
}

// ViewPage reads a view.
func (g *Gen) ViewPage(pageName string, pg map[string]any, sess *Session) (*Page, bool) {
	at := "/pages/" + pageName
	before := len(g.Diags)
	if text(pg["enabledBy"]) != "" {
		g.Problem("error", at+"/enabledBy", "%s is switched on by %s, and this version of %s does not read configuration, so the page would be served while it is off", pageName, text(pg["enabledBy"]), g.Profile.Name)
	}
	permission := text(pg["permission"])
	if permission != "public" && sess == nil {
		g.Problem("error", at+"/permission", "%s needs %s, and the ui target's settings name no session to read who holds it; add settings.session with the operation that answers it", pageName, permission)
	}
	source, ok := g.load(pageName, at, pg)
	if !ok {
		return nil, false
	}
	entity := obj0(obj0(g.Spec["entities"])[text(pg["entity"])])
	props := obj0(entity["properties"])
	wires := map[string]string{}
	for f := range props {
		wires[f] = wirename.Of(g.WireNames, f)
	}
	conditions := obj0(pg["fieldConditions"])
	var sections []Value
	for i, s := range sectionsOf(pg) {
		var fields []Value
		for j, f := range s.fields {
			label := orText(text(obj0(props[f])["title"]), f)
			entry := []Member{{"name", Str(wirename.Of(g.WireNames, f))}, {"label", Str(g.Say(pageName+".fields."+f, label))}}
			ptr := fmt.Sprintf("%s/fields/%d", at, j)
			if pg["sections"] != nil {
				ptr = fmt.Sprintf("%s/sections/%d/fields/%d", at, i, j)
			}
			parts, many, ok := g.viewParts(pageName, ptr, f, pageName+".fields."+f, obj0(props[f]), nil)
			if !ok {
				continue
			}
			if parts != nil {
				entry = append(entry, Member{"parts", Array(parts)})
				if many {
					entry = append(entry, Member{"list", Raw("true")})
				}
			}
			if e := text(obj0(conditions[f])["hiddenWhen"]); e != "" {
				rule, ok := g.condition(e, at+"/fieldConditions/"+f+"/hiddenWhen", "the field's hiddenWhen", props, wires)
				if !ok {
					continue
				}
				entry = append(entry, Member{"hiddenWhen", rule})
			}
			fields = append(fields, Object(entry))
		}
		section := []Member{}
		if s.title != "" {
			section = append(section, Member{"title", Str(g.Say(fmt.Sprintf("%s.sections.%d", pageName, i), s.title))})
		}
		section = append(section, Member{"fields", Array(fields)})
		sections = append(sections, Object(section))
	}
	routes := map[string]string{}
	var actions, operations []Value
	pages := obj0(g.Spec["pages"])
	own := map[string]bool{}
	for _, p := range PathParam.FindAllString(text(pg["route"]), -1) {
		own[p[1:len(p)-1]] = true
	}
	failed := obj0(obj0(pg["states"])["failed"])
	for i, a := range list(pg["actions"]) {
		am := obj0(a)
		ptr := fmt.Sprintf("%s/actions/%d", at, i)
		target := text(am["target"])
		if text(am["kind"]) != "navigate" {
			op, ok := g.rowAction(pageName, ptr, am, pageName+".actions."+target, orText(text(am["permission"]), permission), entity, list(entity["primaryKey"]), failed)
			if ok {
				operations = append(operations, op)
			}
			continue
		}
		tp, ok := obj(pages[target])
		if !ok {
			g.Problem("error", ptr, "the action %s opens %s, which is not a page of the specification", text(am["label"]), target)
			continue
		}
		for _, p := range PathParam.FindAllString(text(tp["route"]), -1) {
			if param := p[1 : len(p)-1]; props[param] == nil && !own[param] {
				g.Problem("error", ptr, "the action %s opens %s at %s, and neither the record nor the view's route gives %s", text(am["label"]), target, text(tp["route"]), p)
			}
		}
		routes[target] = text(tp["route"])
		actions = append(actions, Object([]Member{
			{"label", Str(g.Say(pageName+".actions."+target, text(am["label"])))},
			{"navigate", Str(target)},
			{"permission", Str(orText(text(am["permission"]), permission))},
		}))
	}
	if g.FailedSince(before) {
		return nil, false
	}
	part := "view-page"
	schemaMembers := []Member{}
	add := func(role string, v Value) { schemaMembers = append(schemaMembers, Member{g.Name(part, role), v}) }
	add("title", Str(g.Say(pageName+".title", text(pg["title"]))))
	add("permission", Str(permission))
	add("source", source)
	add("sections", Array(sections))
	add("actions", Array(actions))
	add("operations", Array(operations))
	if g.FailedSince(before) {
		return nil, false
	}
	return &Page{Name: pageName, Kind: "view", Part: part, Route: text(pg["route"]), Permission: permission, Schema: schemaMembers, Routes: routes}, true
}

func anyMapBool(m map[string]bool) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
