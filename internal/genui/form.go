package genui

import (
	_ "embed"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
)

// Forms and task pages (ADR-153). A form's fields are controls in its
// HTML, with the constraints the browser checks from the JSON Schema
// keywords; its module hands form.js what HTML cannot say: the checks
// across fields, the fields hidden or read-only by the record, the
// pickers, the record it loads, what it sends and where each success
// leads.

//go:embed runtime/form.js
var formRuntime string

// The generator's own words on a form.
const (
	choose         = "Choose one"
	patternMessage = "This is not in the form it takes."
	twiceMessage   = "The two entries are not the same."
)

// formPage is what one form or task page needs.
type formPage struct {
	name, title, upper, language string
	task                         bool
	submit, method, path         string // path is the JavaScript that writes it
	idempotencyKey               string
	sent                         []string
	controls                     []string // the form's HTML inside its element
	checks                       []formCheck
	conditions                   map[string][2]string // hiddenWhen and readOnlyWhen, by field
	pickers                      map[string]string    // a picker's data, by field
	next                         map[string]formNext  // by status, or success for a form
	source, sourcePath           string
	rows                         map[string]string // the loaded record's property of each child row's relation
	failed                       string
	refusals                     [][3]string // status, message and field
	sourceRefusals               [][2]string
}

type formCheck struct{ field, rule, message string }

// formNext is what follows a success: the page it opens, with its route's
// parameters from the answer's or the record's fields, and its message.
type formNext struct {
	page, message string
	with          map[string]string
}

// formField is one field drawn on a form: its name and the property that
// gives its keywords, and whether it is required, read-only and entered
// twice.
type formField struct {
	at, name, label string
	prop            map[string]any
	required        bool
	readOnly        bool
	twice           bool
	// picked is set for a field a picker fills from a list it reads, and
	// search when that list searches.
	picked, search bool
}

// enumValues are the values a property may take, through an enum it
// refers to.
func (g *gen) enumValues(prop map[string]any) []string {
	values := list(prop["enum"])
	if ref, ok := strings.CutPrefix(text(prop["$ref"]), "#/enums/"); ok {
		values = list(obj0(obj0(g.spec["enums"])[ref])["enum"])
	}
	var out []string
	for _, v := range values {
		out = append(out, text(v))
	}
	return out
}

// propertyType is a property's JSON Schema type, null left out.
func propertyType(prop map[string]any) string {
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

// attribute writes a control's attribute from a keyword, when it has it.
func attribute(prop map[string]any, keyword, name string) string {
	if v := text(prop[keyword]); v != "" {
		return " " + name + "=" + quoted(v)
	}
	return ""
}

// control writes one field's control, or reports why it cannot. optional
// is set inside a value that may be left out, where a required part is
// asked for only once any part is given.
func (g *gen) fieldControl(f formField, optional bool) ([]string, bool) {
	prop := f.prop
	label := html.EscapeString(f.label)
	required := ""
	switch {
	case f.required && optional:
		required = " data-required"
	case f.required:
		required = " required"
	}
	if ref, many := valueRef(prop); ref != "" {
		return g.valueControl(f, ref, many, optional)
	}
	readOnly := ""
	if f.readOnly {
		readOnly = " readonly"
	}
	name := " name=" + quoted(f.name)
	var input string
	t := propertyType(prop)
	switch format := text(prop["format"]); {
	case f.picked || g.enumValues(prop) != nil:
		if f.readOnly {
			readOnly = " disabled"
		}
		out := []string{}
		if f.search {
			out = append(out, "<input type=\"search\" data-search="+quoted(f.name)+" aria-label="+quoted("Find "+f.label)+">")
		}
		out = append(out, "<label>"+label, "<select"+name+required+readOnly+">", `<option value="">`+choose+"</option>")
		for _, v := range g.enumValues(prop) {
			if !f.picked {
				out = append(out, "<option value="+quoted(v)+">"+html.EscapeString(v)+"</option>")
			}
		}
		return append(out, "</select>", "</label>"), true
	case t == "boolean":
		if f.readOnly {
			readOnly = " disabled"
		}
		input = "<input" + name + ` type="checkbox"` + readOnly + ">"
	case t == "integer" || t == "number":
		step := ` step="any"`
		if t == "integer" {
			step = ` step="1"`
		}
		input = "<input" + name + ` type="number"` + required + attribute(prop, "minimum", "min") + attribute(prop, "maximum", "max") + step + readOnly + ">"
	case t != "string":
		g.problem("error", f.at, "%s is of type %s, and specarch-gen-ui writes a form's fields of type string, integer, number or boolean, or holding a schema; it is left out", f.name, orText(t, "none"))
		return nil, false
	case format == "date" || format == "email" || format == "password":
		input = "<input" + name + " type=" + quoted(format) + required + attribute(prop, "minLength", "minlength") + attribute(prop, "maxLength", "maxlength") + pattern(prop) + readOnly + ">"
	case format == "decimal":
		input = "<input" + name + ` type="text" inputmode="decimal"` + required + readOnly + ">"
	case format == "" || format == "uuid" || format == "uri":
		lengths := attribute(prop, "minLength", "minlength") + attribute(prop, "maxLength", "maxlength")
		if m, err := strconv.Atoi(text(prop["maxLength"])); format == "" && (err != nil || m > 255) {
			return []string{"<label>" + label, "<textarea" + name + required + lengths + pattern(prop) + readOnly + "></textarea>", "</label>"}, true
		}
		input = "<input" + name + ` type="text"` + required + lengths + pattern(prop) + readOnly + ">"
	default:
		g.problem("error", f.at, "%s is of format %s, which specarch-gen-ui does not write on a form; show it on a view. It is left out", f.name, format)
		return nil, false
	}
	out := []string{"<label>" + label, input, "</label>"}
	if f.twice {
		again := strings.Replace(strings.Replace(input, name, " data-again="+quoted(f.name), 1), " readonly", "", 1)
		if !strings.Contains(again, " required") {
			again = strings.Replace(again, " data-again=", " required data-again=", 1)
		}
		out = append(out, "<label>"+html.EscapeString(f.label+" again"), again, "</label>")
	}
	return out, true
}

// pattern writes a property's pattern for form.js, which reads it as
// JSON Schema does: unanchored, with the u flag. HTML's own pattern
// attribute anchors it and reads it with the v flag, so it is not used.
func pattern(prop map[string]any) string {
	if p := text(prop["pattern"]); p != "" {
		return " data-pattern=" + quoted(p)
	}
	return ""
}

// valueRef is the schema a property holds, and whether it holds a list of
// them; "" when it holds none.
func valueRef(prop map[string]any) (string, bool) {
	if ref, ok := strings.CutPrefix(text(prop["$ref"]), "#/schemas/"); ok {
		return ref, false
	}
	if propertyType(prop) == "array" {
		if ref, ok := strings.CutPrefix(text(obj0(prop["items"])["$ref"]), "#/schemas/"); ok {
			return ref, true
		}
	}
	return "", false
}

// valueControl writes a field that holds a value as a fieldset of its
// parts, in the order of their names, and one that holds a list of them
// as rows a person adds and removes, up to its maxItems.
func (g *gen) valueControl(f formField, ref string, many, optional bool) ([]string, bool) {
	schema := obj0(obj0(g.spec["schemas"])[ref])
	if f.twice || f.readOnly {
		g.problem("error", f.at, "%s holds %s, and a value is neither entered twice nor shown read-only on a form; show it on a view. It is left out", f.name, ref)
		return nil, false
	}
	required := map[string]bool{}
	for _, r := range list(schema["required"]) {
		required[text(r)] = true
	}
	props := obj0(schema["properties"])
	var parts []string
	for _, p := range sortedKeys(props) {
		prop := obj0(props[p])
		label := text(prop["title"])
		if label == "" {
			g.problem("warning", f.at, "the part %s of %s has no title, so its label is its name; give the property a title", p, ref)
			label = p
		}
		lines, ok := g.fieldControl(formField{at: f.at, name: p, label: label, prop: prop, required: required[p]}, optional && !many || !f.required && !many)
		if !ok {
			return nil, false
		}
		parts = append(parts, lines...)
	}
	if !many {
		out := []string{"<fieldset data-value=" + quoted(f.name) + ">", "<legend>" + html.EscapeString(f.label) + "</legend>"}
		return append(append(out, parts...), "</fieldset>"), true
	}
	maximum := ""
	if m := text(f.prop["maxItems"]); m != "" {
		maximum = " data-maximum=" + quoted(m)
	}
	out := []string{"<fieldset data-list=" + quoted(f.name) + maximum + ">", "<legend>" + html.EscapeString(f.label) + "</legend>", "<div data-rows></div>", "<template>", "<fieldset data-row>", "<legend></legend>"}
	out = append(out, parts...)
	return append(out, `<button type="button" data-remove>Remove</button>`, "</fieldset>", "</template>",
		"<button type=\"button\" data-add>"+html.EscapeString("Add to "+f.label)+"</button>", "</fieldset>"), true
}

// form reads a form or a task page, or reports why it cannot be written.
func (g *gen) form(name string, pg map[string]any) *formPage {
	at := "/pages/" + name
	task := text(pg["kind"]) == "task"
	fp := &formPage{name: name, title: text(pg["title"]), upper: upperSnake(name), language: text(g.settings["language"]), task: task,
		conditions: map[string][2]string{}, pickers: map[string]string{}, next: map[string]formNext{}, rows: map[string]string{}}
	fp.submit = text(pg["submit"])
	op, path, method := g.operation(fp.submit)
	if op == nil || method == "get" {
		g.problem("error", at+"/submit", "%s submits to %s, which is not an operation of the specification that takes a request body", name, fp.submit)
		return nil
	}
	if !g.plainOnly(at+"/submit", name, "submits to", fp.submit, op) {
		return nil
	}
	fp.method = strings.ToUpper(method)
	body := g.bodySchema(op)
	bodyProps := obj0(body["properties"])
	required := map[string]bool{}
	for _, r := range list(body["required"]) {
		required[text(r)] = true
	}
	entity := obj0(obj0(g.spec["entities"])[text(pg["entity"])])
	props := obj0(entity["properties"])
	if task {
		props = bodyProps
	}
	route := map[string]bool{}
	for _, p := range routeParameters(text(pg["route"])) {
		route[p] = true
	}
	parts := pathParam.Split(path, -1)
	js := jsTemplate(parts[0])
	for i, p := range routeParameters(path) {
		if !route[p] {
			g.problem("error", at+"/submit", "%s submits to %s at %s, and its route %s gives no {%s}", name, fp.submit, path, text(pg["route"]), p)
			return nil
		}
		js += "${encodeURIComponent(parameters." + p + ")}" + jsTemplate(parts[i+1])
	}
	if len(parts) > 1 {
		fp.path = "(parameters) => `" + js + "`"
	} else {
		fp.path = "() => " + jsString(path)
	}
	for _, prm := range list(op["parameters"]) {
		pm := obj0(prm)
		if in := text(pm["in"]); in != "path" && pm["required"] == true && text(pm["name"]) != text(op["idempotencyKey"]) {
			g.problem("error", at+"/submit", "%s requires the %s parameter %s, which a form does not send", fp.submit, in, text(pm["name"]))
			return nil
		}
	}
	fp.idempotencyKey = text(op["idempotencyKey"])
	if pg["source"] != nil {
		if task {
			g.problem("error", at+"/source", "%s is a task, which loads no record; make it a form", name)
			return nil
		}
		src, srcPath, ok := g.load(name, at, pg)
		if !ok {
			return nil
		}
		fp.source, fp.sourcePath = text(pg["source"]), srcPath
		fp.sourceRefusals = refusals(src, obj0(obj0(pg["states"])["failed"]))
	}
	shown := map[string]bool{}
	conditions := obj0(pg["fieldConditions"])
	pickers := obj0(pg["pickers"])
	twice := map[string]bool{}
	for _, f := range list(pg["enteredTwice"]) {
		twice[text(f)] = true
	}
	for i, s := range sectionsOf(pg) {
		var lines []string
		for j, f := range s.fields {
			ptr := fieldAt(at, pg, i, j)
			shown[f.name] = true
			prop, ok := obj(props[f.name])
			if !ok {
				g.problem("error", ptr, "%s shows %s, which is not a field of %s; it is left out", name, f.name, orText(text(pg["entity"]), "the request body"))
				continue
			}
			cond := obj0(conditions[f.name])
			readOnly := prop["readOnly"] == true || cond["readOnly"] == true
			if bodyProps[f.name] == nil && !readOnly {
				g.problem("error", ptr, "%s shows %s, which the request body of %s does not take; add it to the body, or make it read-only under fieldConditions. It is left out", name, f.name, fp.submit)
				continue
			}
			label := text(prop["title"])
			if label == "" {
				g.problem("warning", ptr, "%s has no title, so its label is its name; give the property a title", f.name)
				label = f.name
			}
			field := formField{at: ptr, name: f.name, label: label, prop: prop, required: required[f.name] && !readOnly, readOnly: readOnly, twice: twice[f.name]}
			if pk, ok := obj(pickers[f.name]); ok {
				data, ok := g.picker(name, f.name, pk, entity)
				if !ok {
					continue
				}
				fp.pickers[f.name] = data
				field.picked, field.search = true, strings.Contains(data, "search: ")
			}
			control, ok := g.fieldControl(field, false)
			if !ok {
				continue
			}
			if c, ok := g.fieldCondition(ptr, at, f.name, cond, props); ok {
				if c != ([2]string{}) {
					if _, value := valueRef(prop); value || strings.HasPrefix(text(prop["$ref"]), "#/schemas/") {
						g.problem("error", at+"/fieldConditions/"+f.name, "%s holds a value, and specarch-gen-ui hides or locks a field that holds none; it is left out", f.name)
						continue
					}
					fp.conditions[f.name] = c
				}
			} else {
				continue
			}
			lines = append(lines, control...)
			if !readOnly {
				fp.sent = append(fp.sent, f.name)
			}
		}
		if s.title != "" {
			fp.controls = append(fp.controls, "<fieldset>", "<legend>"+html.EscapeString(s.title)+"</legend>")
			fp.controls = append(fp.controls, lines...)
			fp.controls = append(fp.controls, "</fieldset>")
		} else {
			fp.controls = append(fp.controls, lines...)
		}
	}
	for i, r := range list(pg["childRows"]) {
		lines, ok := g.childRows(fmt.Sprintf("%s/childRows/%d", at, i), name, pg, obj0(r), entity, bodyProps, fp)
		if ok {
			shown[text(obj0(r)["relation"])] = true
			fp.controls = append(fp.controls, lines...)
		}
	}
	for _, r := range sortedSet(required) {
		if !shown[r] {
			g.problem("error", at, "%s submits to %s, whose request body requires %s, which the page does not show", name, fp.submit, r)
			return nil
		}
	}
	checks := obj0(pg["checks"])
	for _, key := range sortedKeys(checks) {
		c := obj0(checks[key])
		ptr := at + "/checks/" + key
		if text(c["field"]) == "" {
			g.problem("error", ptr, "the check %s names no field, and specarch-gen-ui shows a check's message beside its field; name it under field. It is left out", key)
			continue
		}
		rule, ok := g.rule(text(c["expression"]), ptr+"/expression", "the check "+key, props)
		if ok {
			fp.checks = append(fp.checks, formCheck{field: text(c["field"]), rule: rule, message: text(c["message"])})
		}
	}
	states := obj0(pg["states"])
	failed := obj0(states["failed"])
	fp.failed = orText(text(obj0(failed["default"])["message"]), defaultFailed)
	for _, problem := range sortedKeys(failed) {
		if problem == "default" {
			continue
		}
		st := obj0(failed[problem])
		status := ""
		rs := obj0(op["responses"])
		for _, code := range sortedKeys(rs) {
			if text(obj0(rs[code])["problem"]) == problem {
				status = code
			}
		}
		if status == "" {
			if fp.source == "" {
				g.problem("warning", at+"/states/failed/"+problem, "%s shows a message for %s, which %s does not answer with; it is left out", name, problem, fp.submit)
			}
			continue
		}
		fp.refusals = append(fp.refusals, [3]string{status, text(st["message"]), text(st["field"])})
	}
	sort.SliceStable(fp.refusals, func(i, j int) bool { return fp.refusals[i][0] < fp.refusals[j][0] })
	g.formNext(fp, at, pg, op, props)
	return fp
}

// fieldCondition reads a field's hiddenWhen and readOnlyWhen.
func (g *gen) fieldCondition(ptr, at, field string, cond, props map[string]any) ([2]string, bool) {
	var out [2]string
	for i, k := range []string{"hiddenWhen", "readOnlyWhen"} {
		e := text(cond[k])
		if e == "" {
			continue
		}
		rule, ok := g.rule(e, at+"/fieldConditions/"+field+"/"+k, "the field's "+k, props)
		if !ok {
			return out, false
		}
		out[i] = rule
	}
	return out, true
}

// formNext reads what follows each success: a form's one onSubmitted for
// every success its operation answers, 202 for a workflow's trigger among
// them, a task's by status, each answered status needing one.
func (g *gen) formNext(fp *formPage, at string, pg, op, props map[string]any) {
	submitted := obj0(pg["onSubmitted"])
	own := map[string]bool{}
	for _, p := range routeParameters(text(pg["route"])) {
		own[p] = true
	}
	read := func(ptr string, ev map[string]any) (formNext, bool) {
		n := formNext{message: text(ev["message"]), with: map[string]string{}}
		target := text(ev["navigate"])
		if target == "" {
			return n, true
		}
		tp, ok := obj(obj0(g.spec["pages"])[target])
		if !ok {
			g.problem("error", ptr+"/navigate", "%s leads to %s, which is not a page of the specification", fp.name, target)
			return n, false
		}
		with := obj0(ev["with"])
		for _, p := range routeParameters(text(tp["route"])) {
			switch {
			case text(with[p]) != "":
				n.with[p] = text(with[p])
			case props[p] != nil || own[p]:
				n.with[p] = p
			default:
				g.problem("error", ptr+"/navigate", "%s leads to %s at %s, and nothing gives {%s}: name the field of the answer that does under with", fp.name, target, text(tp["route"]), p)
				return n, false
			}
		}
		n.page = target
		return n, true
	}
	if !fp.task {
		if submitted == nil {
			g.problem("error", at+"/onSubmitted", "%s says nothing of what follows a success, so the page could not tell the person what happened; add onSubmitted", fp.name)
			return
		}
		if n, ok := read(at+"/onSubmitted", submitted); ok {
			fp.next["success"] = n
		}
		return
	}
	for _, status := range sortedKeys(obj0(op["responses"])) {
		if _, ok := submitted[status]; !ok && strings.HasPrefix(status, "2") {
			g.problem("error", at+"/onSubmitted", "%s answers %s, and %s says nothing of it, so the page could not tell the person what happened; add onSubmitted %q", fp.submit, status, fp.name, status)
		}
	}
	for _, status := range sortedKeys(submitted) {
		if n, ok := read(at+"/onSubmitted/"+status, obj0(submitted[status])); ok {
			fp.next[status] = n
		}
	}
}

// picker reads a field's picker: the list operation it reads, the key of
// the record picked, the fields shown to choose by and those it fills, as
// the data form.js takes.
func (g *gen) picker(pageName, field string, pk, entity map[string]any) (string, bool) {
	at := "/pages/" + pageName + "/pickers/" + field
	var target string
	for _, r := range obj0(entity["relations"]) {
		rel := obj0(r)
		if text(rel["kind"]) == "many-to-one" && text(rel["via"]) == field {
			target = text(rel["target"])
		}
	}
	key := list(obj0(obj0(g.spec["entities"])[target])["primaryKey"])
	if target == "" || len(key) != 1 {
		g.problem("error", at, "%s picks %s through no many-to-one relation to a record with a primary key of one field, so the picked record has no one value to hold; it is left out", pageName, field)
		return "", false
	}
	id := text(pk["source"])
	op, path, method := g.operation(id)
	listOf := obj0(op["listOf"])
	size := orText(text(obj0(listOf["pageSize"])["maximum"]), text(obj0(listOf["pageSize"])["default"]))
	switch {
	case op == nil || method != "get":
		g.problem("error", at+"/source", "the picker of %s reads %s, which is not a GET operation of the specification; it is left out", field, id)
		return "", false
	case pathParam.MatchString(path):
		g.problem("error", at+"/source", "the picker of %s reads %s at %s, whose path takes a parameter a picker cannot give; it is left out", field, id, path)
		return "", false
	case listOf == nil || size == "":
		g.problem("error", at+"/source", "the picker of %s reads %s, which does not page; give it listOf with a pageSize. It is left out", field, id)
		return "", false
	case g.list["items"] == "" || g.list["pageSize"] == "":
		g.problem("error", at+"/source", "the picker of %s reads a page through the paginated-list idiom, which the implementation file excludes; keep the idiom", field)
		return "", false
	case !g.plainOnly(at+"/source", "the picker of "+field, "reads", id, op):
		return "", false
	}
	parts := []string{"path: " + jsString(path), "items: " + jsString(g.list["items"]), "pageSize: [" + jsString(g.list["pageSize"]) + ", " + size + "]"}
	if len(list(listOf["searchable"])) > 0 && g.list["search"] != "" {
		parts = append(parts, "search: "+jsString(g.list["search"]))
	} else {
		g.problem("warning", at+"/source", "the picker of %s reads %s, which searches no field, so it offers the first %s records and finds among them only", field, id, size)
	}
	var shows []string
	for _, s := range list(pk["shows"]) {
		shows = append(shows, jsString(text(s)))
	}
	parts = append(parts, "value: "+jsString(text(key[0])), "shows: ["+strings.Join(shows, ", ")+"]")
	var fills []string
	fm := obj0(pk["fills"])
	for _, into := range sortedKeys(fm) {
		fills = append(fills, into+": "+jsString(text(fm[into])))
	}
	if len(fills) == 0 {
		parts = append(parts, "fills: {}")
	} else {
		parts = append(parts, "fills: { "+strings.Join(fills, ", ")+" }")
	}
	return "{ " + strings.Join(parts, ", ") + " }", true
}

// childRows writes a form's rows of a relation: the body's array of that
// name sends the rows added, each with the target entity's fields; a field
// the array's items do not take is shown and never sent, and loaded rows,
// read from the property of the source's answer that carries them, are
// shown locked and never sent.
func (g *gen) childRows(ptr, pageName string, pg, row, entity, bodyProps map[string]any, fp *formPage) ([]string, bool) {
	rel := text(row["relation"])
	target := text(obj0(obj0(entity["relations"])[rel])["target"])
	targetProps := obj0(obj0(obj0(g.spec["entities"])[target])["properties"])
	arr := obj0(bodyProps[rel])
	items := g.resolve(obj0(arr["items"]))
	if propertyType(arr) != "array" || propertyType(items) != "object" && items["properties"] == nil {
		g.problem("error", ptr+"/relation", "%s edits the rows of %s, and the request body of %s has no array of objects named %s to send them in; add it to the body. They are left out", pageName, rel, fp.submit, rel)
		return nil, false
	}
	itemProps := obj0(items["properties"])
	itemRequired := map[string]bool{}
	for _, q := range list(items["required"]) {
		itemRequired[text(q)] = true
	}
	if pg["source"] != nil {
		if row["lockLoadedRows"] != true {
			g.problem("error", ptr, "%s loads the rows of %s and may change them, and specarch-gen-ui sends only the rows added; lock the loaded rows with lockLoadedRows. They are left out", pageName, rel)
			return nil, false
		}
		loaded := g.loadedRows(text(pg["source"]), rel)
		if loaded == "" {
			g.problem("error", ptr+"/relation", "%s loads its record from %s, whose answer carries no rows of %s, so the loaded rows cannot be shown; answer a view that adds rows: %s. They are left out", pageName, text(pg["source"]), rel, rel)
			return nil, false
		}
		fp.rows[rel] = loaded
	}
	shownRow := map[string]bool{}
	var parts []string
	for j, fv := range list(row["fields"]) {
		f := text(fv)
		shownRow[f] = true
		prop := obj0(targetProps[f])
		if prop == nil {
			continue // the validator reports a field the target lacks
		}
		label := orText(text(prop["title"]), f)
		readOnly := prop["readOnly"] == true || itemProps[f] == nil
		lines, ok := g.fieldControl(formField{at: fmt.Sprintf("%s/fields/%d", ptr, j), name: f, label: label, prop: prop, required: itemRequired[f] && !readOnly, readOnly: readOnly}, false)
		if !ok {
			return nil, false
		}
		parts = append(parts, lines...)
	}
	for _, q := range sortedSet(itemRequired) {
		if !shownRow[q] {
			g.problem("error", ptr+"/fields", "the rows of %s are sent to %s, whose items require %s, which the rows do not show; they are left out", rel, fp.submit, q)
			return nil, false
		}
	}
	fp.sent = append(fp.sent, rel)
	maximum := orText(text(row["maximum"]), text(arr["maxItems"]))
	if maximum != "" {
		maximum = " data-maximum=" + quoted(maximum)
	}
	title := orText(text(row["title"]), rel)
	out := []string{"<fieldset data-list=" + quoted(rel) + maximum + ">", "<legend>" + html.EscapeString(title) + "</legend>", "<div data-rows></div>", "<template>", "<fieldset data-row>", "<legend></legend>"}
	out = append(out, parts...)
	return append(out, `<button type="button" data-remove>Remove</button>`, "</fieldset>", "</template>",
		"<button type=\"button\" data-add>"+html.EscapeString("Add to "+title)+"</button>", "</fieldset>"), true
}

// loadedRows is the property of an operation's answer that carries the
// rows of a relation: a property of the view it answers that adds rows of
// it; "" when it carries none.
func (g *gen) loadedRows(sourceID, relation string) string {
	op, _, _ := g.operation(sourceID)
	schema := obj0(obj0(obj0(obj0(obj0(op["responses"])["200"])["content"])["application/json"])["schema"])
	view, ok := strings.CutPrefix(text(schema["$ref"]), "#/views/")
	if !ok {
		return ""
	}
	props := obj0(obj0(obj0(g.spec["views"])[view])["properties"])
	for _, p := range sortedKeys(props) {
		if text(obj0(props[p])["rows"]) == relation {
			return p
		}
	}
	return ""
}

// pages are the pages a form's successes open, in order.
func (fp *formPage) pages() []string {
	seen := map[string]bool{}
	var out []string
	for _, k := range sortedNext(fp.next) {
		if p := fp.next[k].page; p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func sortedNext(m map[string]formNext) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (fp *formPage) events() []event {
	return []event{
		{fp.upper + "_SUBMITTED", strings.ReplaceAll(fp.title, "*/", "* /") + " was sent and succeeded: { status, answer }"},
		{fp.upper + "_REFUSED", strings.ReplaceAll(fp.title, "*/", "* /") + " was refused or failed: { status }"},
	}
}

// html writes the form's HTML: its fields as controls, in sections when
// it has them, its rows and the button that sends it.
func (fp *formPage) html(g *gen) string {
	out := g.head(fp.name, fp.title, fp.language)
	out = append(out, `<p id="status" role="status" aria-live="polite"></p>`, `<form id="record" aria-labelledby="title">`)
	out = append(out, fp.controls...)
	out = append(out, `<button type="submit">`+html.EscapeString(fp.title)+"</button>", "</form>", "</main>", "</body>", "</html>")
	return strings.Join(out, "\n") + "\n"
}

// js writes the form's module.
func (fp *formPage) js() string {
	var b strings.Builder
	w := func(s string) { b.WriteString(s + "\n") }
	intro := "The page " + fp.name + ": " + fp.title + ". It "
	if fp.source != "" {
		intro += "loads its record through " + fp.source + ", "
	}
	intro += "sends its fields to " + fp.submit
	if ps := fp.pages(); len(ps) > 0 {
		intro += " and opens " + strings.Join(ps, " or ") + " once it succeeds"
	}
	for _, l := range wrap(intro+".", 72-3) {
		w("// " + l)
	}
	w("")
	w("import { " + fp.upper + "_SUBMITTED, " + fp.upper + "_REFUSED } from \"./events.js\";")
	w(`import { form } from "./form.js";`)
	w("")
	w("form({")
	w("  submit: {")
	w("    operation: " + jsString(fp.submit) + ",")
	w("    method: " + jsString(fp.method) + ",")
	w("    path: " + fp.path + ",")
	var sent []string
	for _, s := range fp.sent {
		sent = append(sent, jsString(s))
	}
	w("    sent: [" + strings.Join(sent, ", ") + "],")
	if fp.idempotencyKey != "" {
		w("    idempotencyKey: " + jsString(fp.idempotencyKey) + ",")
	}
	w("  },")
	if fp.source != "" {
		src := "  source: { operation: " + jsString(fp.source) + ", path: (parameters) => `" + fp.sourcePath + "`"
		if len(fp.rows) > 0 {
			var rs []string
			for _, k := range sortedKeys(anyStrings(fp.rows)) {
				rs = append(rs, k+": "+jsString(fp.rows[k]))
			}
			src += ", rows: { " + strings.Join(rs, ", ") + " }"
		}
		w(src + " },")
	}
	if len(fp.checks) == 0 {
		w("  checks: [],")
	} else {
		w("  checks: [")
		for _, c := range fp.checks {
			w("    { field: " + jsString(c.field) + ", rule: " + c.rule + ", message: " + jsString(c.message) + " },")
		}
		w("  ],")
	}
	if len(fp.conditions) == 0 {
		w("  conditions: {},")
	} else {
		w("  conditions: {")
		for _, k := range sortedKeys(anyConds(fp.conditions)) {
			c := fp.conditions[k]
			var parts []string
			if c[0] != "" {
				parts = append(parts, "hiddenWhen: "+c[0])
			}
			if c[1] != "" {
				parts = append(parts, "readOnlyWhen: "+c[1])
			}
			w("    " + k + ": { " + strings.Join(parts, ", ") + " },")
		}
		w("  },")
	}
	if len(fp.pickers) == 0 {
		w("  pickers: {},")
	} else {
		w("  pickers: {")
		for _, k := range sortedKeys(anyStrings(fp.pickers)) {
			w("    " + k + ": " + fp.pickers[k] + ",")
		}
		w("  },")
	}
	w("  next: {")
	for _, k := range sortedNext(fp.next) {
		n := fp.next[k]
		var parts []string
		if n.page != "" {
			var ws []string
			for _, p := range sortedKeys(anyStrings(n.with)) {
				ws = append(ws, p+": "+jsString(n.with[p]))
			}
			with := "{}"
			if len(ws) > 0 {
				with = "{ " + strings.Join(ws, ", ") + " }"
			}
			parts = append(parts, "page: "+jsString(n.page), "with: "+with)
		}
		if n.message != "" {
			parts = append(parts, "message: "+jsString(n.message))
		}
		label := k
		if k != "success" {
			label = jsString(k)
		}
		w("    " + label + ": { " + strings.Join(parts, ", ") + " },")
	}
	w("  },")
	w("  messages: {")
	w("    failed: " + jsString(fp.failed) + ",")
	w("    pattern: " + jsString(patternMessage) + ",")
	w("    twice: " + jsString(twiceMessage) + ",")
	w("  },")
	w("  // What the operations' refusals say, by status, and the field each is about.")
	w("  refusals: {")
	if len(fp.refusals) == 0 {
		w("    submit: {},")
	} else {
		w("    submit: {")
		for _, r := range fp.refusals {
			entry := "message: " + jsString(r[1])
			if r[2] != "" {
				entry += ", field: " + jsString(r[2])
			}
			w("      " + jsString(r[0]) + ": { " + entry + " },")
		}
		w("    },")
	}
	if fp.source != "" {
		if len(fp.sourceRefusals) == 0 {
			w("    source: {},")
		} else {
			w("    source: {")
			for _, r := range fp.sourceRefusals {
				w("      " + jsString(r[0]) + ": " + jsString(r[1]) + ",")
			}
			w("    },")
		}
	}
	w("  },")
	w("  submitted: " + fp.upper + "_SUBMITTED,")
	w("  refused: " + fp.upper + "_REFUSED,")
	w("});")
	return b.String()
}

func anyStrings(m map[string]string) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

func anyConds(m map[string][2]string) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	return out
}
