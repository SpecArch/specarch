package genuits

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/wirename"
)

// The layouts a form's sections may take, the ui target's settings.layouts.
var layouts = map[string]bool{"page": true, "tabs": true, "steps": true}

// formSettings reads what the ui target's settings say of forms: the
// fields whose starting value a hook gives, and how each form's sections
// sit, by page. Each names a form or a view of the specification, and each
// hook a field it shows; a mistake is an error under settings.
func (g *gen) formSettings() (map[string][]string, map[string]string) {
	pages := obj0(g.spec["pages"])
	hooks := map[string][]string{}
	hookSettings := obj0(g.settings["hooks"])
	for _, pageName := range sortedKeys(hookSettings) {
		pg, ok := obj(pages[pageName])
		if !ok || text(pg["kind"]) != "form" {
			g.problem("error", "/", "the ui target's settings.hooks name %s, which is not a form of the specification; a hook gives the starting value of a form's field", pageName)
			continue
		}
		shown := map[string]bool{}
		for _, f := range shownFields(pg) {
			shown[f] = true
		}
		for _, f := range list(hookSettings[pageName]) {
			if !shown[text(f)] {
				g.problem("error", "/", "the ui target's settings.hooks give %s a hook for %s, which it does not show", pageName, text(f))
				continue
			}
			hooks[pageName] = append(hooks[pageName], text(f))
		}
	}
	chosen := map[string]string{}
	layoutSettings := obj0(g.settings["layouts"])
	for _, pageName := range sortedKeys(layoutSettings) {
		pg, ok := obj(pages[pageName])
		layout := text(layoutSettings[pageName])
		switch {
		case !ok || text(pg["kind"]) != "form":
			g.problem("error", "/", "the ui target's settings.layouts name %s, which is not a form of the specification", pageName)
		case !layouts[layout]:
			g.problem("error", "/", "the ui target's settings.layouts give %s the layout %q; a form's sections sit as page, tabs or steps", pageName, layout)
		case layout != "page" && len(list(pg["sections"])) < 2:
			g.problem("error", "/", "the ui target's settings.layouts give %s %s, and it has fewer than two sections to lay out so; give it sections, or leave the layout out", pageName, layout)
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
func (g *gen) propertyType(prop map[string]any) string {
	if ref, ok := strings.CutPrefix(text(prop["$ref"]), "#/enums/"); ok {
		prop = obj0(obj0(g.spec["enums"])[ref])
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
func (g *gen) load(pageName, at string, pg map[string]any) (value, bool) {
	sourceID := text(pg["source"])
	op, path, method := g.operation(sourceID)
	if op == nil || method != "get" {
		g.problem("error", at+"/source", "%s loads its record from %s, which is not a GET operation of the specification", pageName, sourceID)
		return nil, false
	}
	route := map[string]bool{}
	for _, p := range pathParam.FindAllString(text(pg["route"]), -1) {
		route[p[1:len(p)-1]] = true
	}
	for _, p := range pathParam.FindAllString(path, -1) {
		if !route[p[1:len(p)-1]] {
			g.problem("error", at+"/source", "%s loads %s at %s, and its route %s has no parameter %s to fill it from", pageName, sourceID, path, text(pg["route"]), p)
			return nil, false
		}
	}
	failed := obj0(obj0(pg["states"])["failed"])
	return object([]member{{"operation", str(sourceID)}, {"path", str(path)}, {"failed", array(g.refusalsOf(pageName, op, failed))}}), true
}

// ordersDecimal names a field of format decimal that an expression orders
// with <, <=, > or >=, which the browser cannot do exactly; "" when none.
func (g *gen) ordersDecimal(n *expr.Node, props map[string]any) string {
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
func (g *gen) condition(expression, at, what string, props map[string]any, wires map[string]string) (value, bool) {
	node, errs := expr.Parse(expression)
	if len(errs) > 0 {
		g.problem("error", at, "%s does not parse: %s", what, errs[0].Message)
		return nil, false
	}
	if f := g.ordersDecimal(node, props); f != "" {
		g.problem("error", at, "%s orders %s, a decimal, which the browser cannot compare exactly; check it on the server", what, f)
		return nil, false
	}
	return g.rule(node, wires, at)
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
}

func (g *gen) formField(in formFieldInput) (value, bool) {
	prop := in.prop
	label := text(prop["title"])
	if label == "" {
		g.problem("warning", in.at, "%s has no title, so its label is its name; give the property a title", in.name)
		label = in.name
	}
	t := g.propertyType(prop)
	format := text(prop["format"])
	var part string
	switch {
	case in.picker != nil:
		part = "lookup-field"
	case g.enumValues(prop) != nil:
		part = "select-field"
	case t == "boolean":
		part = "checkbox-field"
	case t == "integer" || t == "number":
		part = "number-field"
	case t != "string":
		g.problem("error", in.at, "%s shows %s, of type %s, and this version of %s writes a form's fields of type string, integer, number or boolean", in.pageName, in.name, orText(t, "none"), name)
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
		g.problem("error", in.at, "%s shows %s, of format %s, which this version of %s does not write on a form; show it on a view", in.pageName, in.name, format, name)
		return nil, false
	}
	wire := wirename.Of(g.wireNames, in.name)
	data := []member{
		{"type", str(g.name(part, "type"))},
		{g.name(part, "name"), str(wire)},
		{g.name(part, "label"), str(g.say(in.pageName+".fields."+in.name, label))},
		{g.name(part, "required"), raw(strconv.FormatBool(in.required))},
	}
	switch part {
	case "text-field", "password-field":
		var rules []value
		for _, kw := range []string{"minLength", "maxLength", "pattern"} {
			v, ok := prop[kw]
			if !ok {
				continue
			}
			if kw == "pattern" {
				data = append(data, member{g.name(part, kw), str(text(v))})
				rules = append(rules, object([]member{{"text", str("screens.rule.pattern")}, {"pattern", str(text(v))}}))
				continue
			}
			data = append(data, member{g.name(part, kw), raw(text(v))})
			rules = append(rules, object([]member{{"text", str("screens." + kw)}, {"count", raw(text(v))}}))
		}
		if part == "password-field" {
			data = append(data, member{g.name(part, "rules"), array(rules)})
		}
	case "text-area-field", "email-field":
		if v, ok := prop["maxLength"]; ok {
			data = append(data, member{g.name(part, "maxLength"), raw(text(v))})
		}
	case "number-field":
		for _, kw := range []string{"minimum", "maximum"} {
			if v, ok := prop[kw]; ok {
				data = append(data, member{g.name(part, kw), raw(text(v))})
			}
		}
		if t == "integer" {
			data = append(data, member{g.name(part, "step"), raw("1")})
		}
	case "select-field":
		var opts []value
		for _, v := range g.enumValues(prop) {
			opts = append(opts, str(v))
		}
		data = append(data, member{g.name(part, "options"), array(opts)})
	case "lookup-field":
		lookup, ok := g.lookup(in)
		if !ok {
			return nil, false
		}
		for _, m := range lookup {
			data = append(data, member{g.name(part, m.key), m.val})
		}
	}
	if in.readOnly {
		data = append(data, member{g.name(part, "readOnly"), raw("true")})
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
		data = append(data, member{g.name(part, k), rule})
	}
	if in.hooked {
		data = append(data, member{g.name(part, "hook"), raw("true")})
	}
	if in.twice {
		data = append(data, member{g.name(part, "twice"), str(g.say(in.pageName+".fields."+in.name+".again", label+" again"))})
	}
	return object(data), true
}

// lookup reads a field's picker: the list operation it reads, the key of
// the record picked, the fields shown to choose by and those it fills.
func (g *gen) lookup(in formFieldInput) ([]member, bool) {
	at := "/pages/" + in.pageName + "/pickers/" + in.name
	var target string
	for _, r := range obj0(in.entity["relations"]) {
		rel := obj0(r)
		if text(rel["kind"]) == "many-to-one" && text(rel["via"]) == in.name {
			target = text(rel["target"])
		}
	}
	targetEntity := obj0(obj0(g.spec["entities"])[target])
	pk := list(targetEntity["primaryKey"])
	if target == "" || len(pk) != 1 {
		g.problem("error", at, "%s picks %s through no many-to-one relation to a record with a primary key of one field, so the picked record has no one value to hold", in.pageName, in.name)
		return nil, false
	}
	sourceID := text(in.picker["source"])
	op, path, method := g.operation(sourceID)
	listOf := obj0(op["listOf"])
	size := obj0(listOf["pageSize"])
	pageSize := orText(text(size["maximum"]), text(size["default"]))
	wireNames := g.listNames()
	switch {
	case op == nil || method != "get":
		g.problem("error", at+"/source", "the picker of %s reads %s, which is not a GET operation of the specification", in.name, sourceID)
		return nil, false
	case pathParam.MatchString(path):
		g.problem("error", at+"/source", "the picker of %s reads %s at %s, whose path takes a parameter a picker cannot give", in.name, sourceID, path)
		return nil, false
	case listOf == nil || pageSize == "":
		g.problem("error", at+"/source", "the picker of %s reads %s, which does not page; give it listOf with a pageSize", in.name, sourceID)
		return nil, false
	case wireNames["items"] == "" || wireNames["pageSize"] == "":
		g.problem("error", at+"/source", "the picker of %s reads a page through the paginated-list idiom, which the implementation file excludes; keep the idiom", in.name)
		return nil, false
	}
	source := []member{{"operation", str(sourceID)}, {"path", str(path)}, {"items", str(wireNames["items"])},
		{"pageSize", object([]member{{"name", str(wireNames["pageSize"])}, {"value", raw(pageSize)}})}}
	if len(list(listOf["searchable"])) > 0 && wireNames["search"] != "" {
		source = append(source, member{"search", str(wireNames["search"])})
	} else {
		g.problem("warning", at+"/source", "the picker of %s reads %s, which searches no field, so it offers the first %s records and finds among them only", in.name, sourceID, pageSize)
	}
	var shows []value
	for _, s := range list(in.picker["shows"]) {
		shows = append(shows, str(wirename.Of(g.wireNames, text(s))))
	}
	fills := []member{}
	fillMap := obj0(in.picker["fills"])
	for _, into := range sortedKeys(fillMap) {
		fills = append(fills, member{wirename.Of(g.wireNames, into), str(wirename.Of(g.wireNames, text(fillMap[into])))})
	}
	return []member{
		{"source", object(source)},
		{"value", str(wirename.Of(g.wireNames, text(pk[0])))},
		{"text", array(shows)},
		{"fills", object(fills)},
	}, true
}

// pageOut are the files of a page: its schema, its page.tsx, the client
// file that hands its hooks on when it has any, and its refusal test.
type pageOut struct {
	schema, page, client, test string
}

// formPage writes the files of a form.
func (g *gen) formPage(pageName string, pg map[string]any, sess *session, hooks []string, layout string) (pageOut, bool) {
	at := "/pages/" + pageName
	if _, ok := pg["childRows"]; ok {
		g.problem("warning", at+"/childRows", "%s edits child rows, and this version of %s does not draw them; it is left out", pageName, name)
		return pageOut{}, false
	}
	submit := text(pg["submit"])
	for _, w := range obj0(g.spec["workflows"]) {
		if text(obj0(w)["trigger"]) == submit && submit != "" {
			g.problem("warning", at+"/submit", "%s starts an approval through %s, and this version of %s does not draw a request that waits; it is left out", pageName, submit, name)
			return pageOut{}, false
		}
	}
	before := len(g.diags)
	if text(pg["enabledBy"]) != "" {
		g.problem("error", at+"/enabledBy", "%s is switched on by %s, and this version of %s does not read configuration, so the page would be served while it is off", pageName, text(pg["enabledBy"]), name)
	}
	if pg["actions"] != nil {
		g.problem("error", at+"/actions", "%s has actions, and this version of %s does not write a form's actions yet", pageName, name)
	}
	permission := text(pg["permission"])
	if permission != "public" && sess == nil {
		g.problem("error", at+"/permission", "%s needs %s, and the ui target's settings name no session to read who holds it; add settings.session with the operation that answers it", pageName, permission)
	}
	entity := obj0(obj0(g.spec["entities"])[text(pg["entity"])])
	props := obj0(entity["properties"])
	wires := map[string]string{}
	for f := range props {
		wires[f] = wirename.Of(g.wireNames, f)
	}
	op, path, method := g.operation(submit)
	if op == nil || method == "get" {
		g.problem("error", at+"/submit", "%s submits to %s, which is not an operation of the specification that takes a request body", pageName, submit)
		return pageOut{}, false
	}
	var source value
	if pg["source"] != nil {
		var ok bool
		if source, ok = g.load(pageName, at, pg); !ok {
			return pageOut{}, false
		}
	}
	route := map[string]bool{}
	for _, p := range pathParam.FindAllString(text(pg["route"]), -1) {
		route[p[1:len(p)-1]] = true
	}
	for _, p := range pathParam.FindAllString(path, -1) {
		param := p[1 : len(p)-1]
		if !route[param] && (pg["source"] == nil || props[param] == nil) {
			g.problem("error", at+"/submit", "%s submits to %s at %s, and neither its route nor a record it loads gives %s", pageName, submit, path, p)
		}
	}
	submitMembers := []member{{"operation", str(submit)}, {"method", str(strings.ToUpper(method))}, {"path", str(path)}}
	for _, prm := range list(op["parameters"]) {
		pm := obj0(prm)
		if in := text(pm["in"]); in != "path" && pm["required"] == true && text(pm["name"]) != text(op["idempotencyKey"]) {
			g.problem("error", at+"/submit", "%s requires the %s parameter %s, which a form does not send", submit, in, text(pm["name"]))
		}
	}
	body := g.bodySchema(op)
	bodyProps := obj0(body["properties"])
	required := map[string]bool{}
	for _, r := range list(body["required"]) {
		required[text(r)] = true
	}
	shown := map[string]bool{}
	for _, f := range shownFields(pg) {
		shown[f] = true
	}
	var sent []value
	for _, f := range shownFields(pg) {
		if bodyProps[f] != nil {
			sent = append(sent, str(wirename.Of(g.wireNames, f)))
		}
	}
	submitMembers = append(submitMembers, member{"fields", array(sent)})
	if key := text(op["idempotencyKey"]); key != "" {
		submitMembers = append(submitMembers, member{"idempotencyKey", str(key)})
	}
	for _, r := range sortedKeys(anyMapBool(required)) {
		if !shown[r] {
			g.problem("error", at, "%s submits to %s, whose request body requires %s, which the form does not show", pageName, submit, r)
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
	var sections []value
	for i, s := range sectionsOf(pg) {
		var fields []value
		for j, f := range s.fields {
			ptr := fmt.Sprintf("%s/fields/%d", at, j)
			if pg["sections"] != nil {
				ptr = fmt.Sprintf("%s/sections/%d/fields/%d", at, i, j)
			}
			prop := obj0(props[f])
			if prop == nil {
				g.problem("error", ptr, "%s shows %s, which is not a field of %s", pageName, f, text(pg["entity"]))
				continue
			}
			cond := obj0(conditions[f])
			readOnly := prop["readOnly"] == true || cond["readOnly"] == true
			if bodyProps[f] == nil && !readOnly {
				g.problem("error", ptr, "%s shows %s, which the request body of %s does not take; add it to the body, or make it read-only under fieldConditions", pageName, f, submit)
				continue
			}
			fv, ok := g.formField(formFieldInput{pageName: pageName, at: ptr, name: f, prop: prop, required: required[f] && !readOnly, readOnly: readOnly,
				conditions: cond, picker: obj0(pickers[f]), hooked: hooked[f], twice: twice[f], props: props, wires: wires, entity: entity})
			if ok {
				fields = append(fields, fv)
			}
		}
		section := []member{}
		if s.title != "" {
			section = append(section, member{"title", str(g.say(fmt.Sprintf("%s.sections.%d", pageName, i), s.title))})
		}
		section = append(section, member{"fields", array(fields)})
		sections = append(sections, object(section))
	}
	var checks []value
	checksMap := obj0(pg["checks"])
	for _, key := range sortedKeys(checksMap) {
		c := obj0(checksMap[key])
		ptr := at + "/checks/" + key
		rule, ok := g.condition(text(c["expression"]), ptr+"/expression", "the check "+key, props, wires)
		if !ok {
			continue
		}
		entry := []member{{"name", str(key)}, {"rule", rule}, {"message", str(g.say(pageName+".checks."+key, text(c["message"])))}}
		if f := text(c["field"]); f != "" {
			entry = append(entry, member{"field", str(wireOf(wires, f))})
		}
		checks = append(checks, object(entry))
	}
	routes := map[string]string{}
	events := []value{}
	ev, ok := obj(pg["onSubmitted"])
	if !ok {
		g.problem("error", at+"/onSubmitted", "%s says nothing of what follows a success, so the page could not tell the person what happened; add onSubmitted", pageName)
	} else {
		entry := []member{}
		if target := text(ev["navigate"]); target != "" {
			tp, found := obj(obj0(g.spec["pages"])[target])
			if !found {
				g.problem("error", at+"/onSubmitted/navigate", "%s leads to %s, which is not a page of the specification", pageName, target)
			}
			routes[target] = text(tp["route"])
			entry = append(entry, member{"navigate", str(target)})
			if with := obj0(ev["with"]); len(with) > 0 {
				var ws []member
				for _, k := range sortedKeys(with) {
					ws = append(ws, member{k, str(wirename.Of(g.wireNames, text(with[k])))})
				}
				entry = append(entry, member{"with", object(ws)})
			}
		}
		if m := text(ev["message"]); m != "" {
			entry = append(entry, member{"message", str(g.say(pageName+".onSubmitted", m))})
		}
		events = append(events, object(entry))
	}
	if g.failedSince(before) {
		return pageOut{}, false
	}
	part := "form-page"
	schemaMembers := []member{}
	add := func(role string, v value) { schemaMembers = append(schemaMembers, member{g.name(part, role), v}) }
	add("title", str(g.say(pageName+".title", text(pg["title"]))))
	add("permission", str(permission))
	if source != nil {
		add("source", source)
	}
	add("submit", object(submitMembers))
	add("layout", str(orText(layout, "page")))
	add("sections", array(sections))
	add("checks", array(checks))
	add("failed", array(g.refusalsOf(pageName, op, obj0(obj0(pg["states"])["failed"]))))
	add("events", array(events))
	if g.failedSince(before) {
		return pageOut{}, false
	}
	schema, page, client := g.pageFiles(pageName, pg, part, schemaMembers, routes, hooks)
	return pageOut{schema, page, client, g.refusalTest(pageName, text(pg["route"]), permission, g.name(part, "permission"))}, true
}

// viewPage writes the files of a view.
func (g *gen) viewPage(pageName string, pg map[string]any, sess *session) (pageOut, bool) {
	at := "/pages/" + pageName
	before := len(g.diags)
	if text(pg["enabledBy"]) != "" {
		g.problem("error", at+"/enabledBy", "%s is switched on by %s, and this version of %s does not read configuration, so the page would be served while it is off", pageName, text(pg["enabledBy"]), name)
	}
	permission := text(pg["permission"])
	if permission != "public" && sess == nil {
		g.problem("error", at+"/permission", "%s needs %s, and the ui target's settings name no session to read who holds it; add settings.session with the operation that answers it", pageName, permission)
	}
	source, ok := g.load(pageName, at, pg)
	if !ok {
		return pageOut{}, false
	}
	entity := obj0(obj0(g.spec["entities"])[text(pg["entity"])])
	props := obj0(entity["properties"])
	wires := map[string]string{}
	for f := range props {
		wires[f] = wirename.Of(g.wireNames, f)
	}
	conditions := obj0(pg["fieldConditions"])
	var sections []value
	for i, s := range sectionsOf(pg) {
		var fields []value
		for _, f := range s.fields {
			label := orText(text(obj0(props[f])["title"]), f)
			entry := []member{{"name", str(wirename.Of(g.wireNames, f))}, {"label", str(g.say(pageName+".fields."+f, label))}}
			if e := text(obj0(conditions[f])["hiddenWhen"]); e != "" {
				rule, ok := g.condition(e, at+"/fieldConditions/"+f+"/hiddenWhen", "the field's hiddenWhen", props, wires)
				if !ok {
					continue
				}
				entry = append(entry, member{"hiddenWhen", rule})
			}
			fields = append(fields, object(entry))
		}
		section := []member{}
		if s.title != "" {
			section = append(section, member{"title", str(g.say(fmt.Sprintf("%s.sections.%d", pageName, i), s.title))})
		}
		section = append(section, member{"fields", array(fields)})
		sections = append(sections, object(section))
	}
	routes := map[string]string{}
	var actions []value
	pages := obj0(g.spec["pages"])
	own := map[string]bool{}
	for _, p := range pathParam.FindAllString(text(pg["route"]), -1) {
		own[p[1:len(p)-1]] = true
	}
	for i, a := range list(pg["actions"]) {
		am := obj0(a)
		ptr := fmt.Sprintf("%s/actions/%d", at, i)
		target := text(am["target"])
		if text(am["kind"]) != "navigate" {
			g.problem("error", ptr, "the action %s runs %s, and this version of %s writes a view's actions that open a page only", text(am["label"]), target, name)
			continue
		}
		tp, ok := obj(pages[target])
		if !ok {
			g.problem("error", ptr, "the action %s opens %s, which is not a page of the specification", text(am["label"]), target)
			continue
		}
		for _, p := range pathParam.FindAllString(text(tp["route"]), -1) {
			if param := p[1 : len(p)-1]; props[param] == nil && !own[param] {
				g.problem("error", ptr, "the action %s opens %s at %s, and neither the record nor the view's route gives %s", text(am["label"]), target, text(tp["route"]), p)
			}
		}
		routes[target] = text(tp["route"])
		actions = append(actions, object([]member{
			{"label", str(g.say(pageName+".actions."+target, text(am["label"])))},
			{"navigate", str(target)},
			{"permission", str(orText(text(am["permission"]), permission))},
		}))
	}
	if g.failedSince(before) {
		return pageOut{}, false
	}
	part := "view-page"
	schemaMembers := []member{}
	add := func(role string, v value) { schemaMembers = append(schemaMembers, member{g.name(part, role), v}) }
	add("title", str(g.say(pageName+".title", text(pg["title"]))))
	add("permission", str(permission))
	add("source", source)
	add("sections", array(sections))
	add("actions", array(actions))
	if g.failedSince(before) {
		return pageOut{}, false
	}
	schema, page, _ := g.pageFiles(pageName, pg, part, schemaMembers, routes, nil)
	return pageOut{schema, page, "", g.refusalTest(pageName, text(pg["route"]), permission, g.name(part, "permission"))}, true
}

// pageFiles writes a guarded page's schema and its page.tsx, the route's
// parameters handed on. A form whose fields take hooks also gets
// page.client.tsx, which imports them from page.hooks.ts, a file the
// generator never writes, so a missing hook fails tsc; the hooks are
// functions, which run in the browser and cannot be handed from page.tsx,
// a server component.
func (g *gen) pageFiles(pageName string, pg map[string]any, part string, schemaMembers []member, routes map[string]string, hooks []string) (string, string, string) {
	importFrom := g.importOf(part)
	schemaType := g.name(part, "schemaType")
	component := g.name(part, "component")
	var schema strings.Builder
	fmt.Fprintf(&schema, "import type { %s } from %q;\n\n", schemaType, importFrom)
	schema.WriteString(statement("export const schema = ", object(schemaMembers), " satisfies "+schemaType+";"))
	components := g.name("application", "components")
	drawn := component
	var client strings.Builder
	if len(hooks) > 0 {
		drawn = "HookedForm"
		var hs []string
		for _, h := range hooks {
			if w := wirename.Of(g.wireNames, h); w != h {
				hs = append(hs, key(w)+": "+h)
			} else {
				hs = append(hs, h)
			}
		}
		client.WriteString("\"use client\";\n\n")
		fmt.Fprintf(&client, "import type { ComponentProps } from \"react\";\nimport { %s } from %q;\nimport { %s } from \"./page.hooks\";\n\n", component, importFrom, strings.Join(hooks, ", "))
		client.WriteString("const hooks = { " + strings.Join(hs, ", ") + " };\n\n")
		client.WriteString("/** The form with the hooks of its fields, which run in the browser. */\n")
		fmt.Fprintf(&client, "export function HookedForm(props: Omit<ComponentProps<typeof %s>, \"hooks\">) {\n  return <%s {...props} hooks={hooks} />;\n}\n", component, component)
	}
	var page strings.Builder
	page.WriteString("import type { Metadata } from \"next\";\n")
	switch {
	case len(hooks) > 0:
		fmt.Fprintf(&page, "import { Guard } from %q;\n", components)
	case importFrom == components:
		both := []string{"Guard", component}
		sort.Strings(both)
		fmt.Fprintf(&page, "import { %s } from %q;\n", strings.Join(both, ", "), components)
	default:
		fmt.Fprintf(&page, "import { %s } from %q;\nimport { Guard } from %q;\n", component, importFrom, components)
	}
	page.WriteString("import { strings, texts } from \"@/strings\";\n")
	if len(hooks) > 0 {
		page.WriteString("import { HookedForm } from \"./page.client\";\n")
	}
	page.WriteString("import { schema } from \"./page.schema\";\n\n")
	fmt.Fprintf(&page, "export const metadata: Metadata = { title: strings[%q] };\n\n", pageName+".title")
	g.writeRoutes(&page, routes)
	props := "schema={schema} texts={t} routes={routes}"
	if pathParam.MatchString(text(pg["route"])) {
		page.WriteString("export default async function Page({ params }: { readonly params: Promise<Record<string, string>> }) {\n")
		page.WriteString("  const parameters = await params;\n")
		props += " parameters={parameters}"
	} else {
		page.WriteString("export default function Page() {\n")
	}
	page.WriteString("  const t = texts(schema);\n  return (\n")
	fmt.Fprintf(&page, "    <Guard permission={schema.%s} texts={t}>\n", g.name(part, "permission"))
	fmt.Fprintf(&page, "      <%s %s />\n", drawn, props)
	page.WriteString("    </Guard>\n  );\n}\n")
	return schema.String(), page.String(), client.String()
}

func anyMapBool(m map[string]bool) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
