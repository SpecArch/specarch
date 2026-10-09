package genuits

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/ownership"
	"github.com/SpecArch/specarch/internal/wirename"
)

// The page sizes a list offers besides its default, up to its maximum.
var pageSizes = []int{10, 20, 50, 100}

// session is what the target's settings say of the person's session: the
// operation that reads it, the properties of its answer, and the page that
// signs in.
type session struct {
	path, signedIn, permissions, signIn string
}

// readSession reads settings.session, which a page that needs a permission
// cannot do without; nil when the settings name none.
func (g *gen) readSession() *session {
	s, ok := obj(g.settings["session"])
	if !ok {
		return nil
	}
	at := "/"
	opID := text(s["operation"])
	op, path, method := g.operation(opID)
	switch {
	case op == nil || method != "get":
		g.problem("error", at, "the ui target's settings.session names %q, which is not a GET operation of the specification; name the operation that answers who is signed in", opID)
		return nil
	case pathParam.MatchString(path):
		g.problem("error", at, "the ui target's settings.session names %s, whose path takes a parameter; the screens read the session at one path", opID)
		return nil
	}
	props := obj0(obj0(obj0(obj0(obj0(obj0(obj0(op["responses"])["200"])["content"])["application/json"])["schema"]))["properties"])
	out := &session{path: path}
	for _, k := range []struct{ key, typ string }{{"signedIn", "boolean"}, {"permissions", "array"}} {
		prop := text(s[k.key])
		if prop == "" {
			g.problem("error", at, "the ui target's settings.session names no %s, the property of %s's answer that holds it", k.key, opID)
			return nil
		}
		if text(obj0(props[prop])["type"]) != k.typ {
			g.problem("error", at, "the ui target's settings.session says %s holds %s, and the 200 answer of %s has no property %s of type %s", prop, k.key, opID, prop, k.typ)
			return nil
		}
		wire := wirename.Of(g.wireNames, prop)
		if k.key == "signedIn" {
			out.signedIn = wire
		} else {
			out.permissions = wire
		}
	}
	if signIn := text(s["signIn"]); signIn != "" {
		pg, ok := obj(obj0(g.spec["pages"])[signIn])
		if !ok || text(pg["kind"]) != "task" {
			g.problem("error", at, "the ui target's settings.session names %q as the page that signs in, which is not a task page of the specification", signIn)
			return nil
		}
		out.signIn = text(pg["route"])
	}
	return out
}

// listNames are the paginated-list idiom's names on the wire.
func (g *gen) listNames() map[string]string {
	out := map[string]string{}
	for _, i := range g.impl.Idioms {
		if i.Name != "paginated-list" || i.As == "excluded" {
			continue
		}
		for _, src := range []map[string]any{i.Override, i.Content} {
			for _, part := range []string{"parameters", "envelope"} {
				for k, v := range obj0(obj0(obj0(obj0(obj0(src["parts"])[part])["stack"])["any"])["names"]) {
					if _, ok := out[k]; !ok {
						out[k] = text(v)
					}
				}
			}
		}
	}
	return out
}

// refusalsOf are the failed states of a page an operation answers, by
// status, and the page's default with no status.
func (g *gen) refusalsOf(pageName string, op map[string]any, failed map[string]any) []value {
	var out []value
	for _, problem := range sortedKeys(failed) {
		st := obj0(failed[problem])
		if problem == "default" {
			continue
		}
		status := g.problemStatus(op, problem)
		if status == "" {
			continue // another operation of the page answers it
		}
		out = append(out, object([]member{{"status", raw(status)}, {"problem", str(problem)}, {"message", str(g.say(pageName+".failed."+problem, text(st["message"])))}}))
	}
	if d, ok := obj(failed["default"]); ok {
		out = append(out, object([]member{{"problem", str("default")}, {"message", str(g.say(pageName+".failed.default", text(d["message"])))}}))
	}
	return out
}

// listPage writes the schema, the page and the refusal test of a list.
func (g *gen) listPage(pageName string, pg map[string]any, sess *session) (string, string, string, bool) {
	at := "/pages/" + pageName
	for _, k := range []string{"onSubmitted", "pickers"} {
		if _, ok := pg[k]; ok {
			g.problem("warning", at+"/"+k, "%s gives %s, which this version of %s does not read on a list; the list is drawn without it", pageName, k, name)
		}
	}
	before := len(g.diags)
	entityName := text(pg["entity"])
	entity := obj0(obj0(g.spec["entities"])[entityName])
	props := obj0(entity["properties"])
	wire := func(f string) string { return wirename.Of(g.wireNames, f) }
	sourceID := text(pg["source"])
	src, srcPath, srcMethod := g.operation(sourceID)
	if src == nil || srcMethod != "get" {
		g.problem("error", at+"/source", "%s reads %s, which is not a GET operation of the specification; a list is read by GET", pageName, sourceID)
		return "", "", "", false
	}
	if pathParam.MatchString(srcPath) {
		g.problem("error", at+"/source", "%s reads %s at %s, whose path takes a parameter, and this version of %s reads a list whose path takes none", pageName, sourceID, srcPath, name)
	}
	listOf, ok := obj(src["listOf"])
	if pg, named := g.pagedByServer(sourceID); named && !ok {
		listOf, ok = map[string]any{"pageSize": map[string]any{"default": pg.pageSize, "maximum": pg.maximum}}, true
	}
	size := obj0(listOf["pageSize"])
	defaultSize, maxSize := text(size["default"]), text(size["maximum"])
	if defaultSize == "" {
		defaultSize = maxSize
	}
	wireNames := g.listNames()
	switch {
	case !ok:
		g.problem("error", at+"/source", "%s pages through its list, and %s has no listOf, so it does not page; give it listOf, or name it under the ui target's settings.server.pages for a server route to page it", pageName, sourceID)
	case defaultSize == "":
		g.problem("error", at+"/source", "%s pages through its list, and the listOf of %s gives no page size; give pageSize a default", pageName, sourceID)
	case wireNames["page"] == "" || wireNames["items"] == "":
		g.problem("error", at+"/source", "%s pages through the paginated-list idiom, which the implementation file excludes; keep the idiom", pageName)
	}
	if text(pg["enabledBy"]) != "" {
		g.problem("error", at+"/enabledBy", "%s is switched on by %s, and this version of %s does not read configuration, so the page would be served while it is off", pageName, text(pg["enabledBy"]), name)
	}
	permission := text(pg["permission"])
	if permission != "public" && sess == nil {
		g.problem("error", at+"/permission", "%s needs %s, and the ui target's settings name no session to read who holds it; add settings.session with the operation that answers it", pageName, permission)
	}
	if g.failedSince(before) {
		return "", "", "", false
	}
	sortable := map[string]bool{}
	for _, f := range list(listOf["sortable"]) {
		sortable[text(f)] = true
	}
	compact := map[string]bool{}
	for _, c := range list(pg["compactColumns"]) {
		compact[text(c)] = true
	}
	var columns []value
	for i, c := range list(pg["columns"]) {
		f := text(c)
		if schemaName, _ := valueOf(obj0(props[f])); schemaName != "" {
			g.problem("error", fmt.Sprintf("%s/columns/%d", at, i), "%s shows %s in a column, which holds a value of %s, and a cell shows one text; show the value on a view", pageName, f, schemaName)
			continue
		}
		columns = append(columns, object([]member{
			{"field", str(wire(f))},
			{"title", str(g.say(pageName+".columns."+f, orText(text(obj0(props[f])["title"]), f)))},
			{"sortable", raw(strconv.FormatBool(sortable[f]))},
			{"compact", raw(strconv.FormatBool(len(compact) == 0 || compact[f]))},
		}))
	}
	params := map[string]bool{}
	for _, prm := range list(src["parameters"]) {
		if text(obj0(prm)["in"]) == "query" {
			params[text(obj0(prm)["name"])] = true
		}
	}
	filterable := map[string]bool{}
	for _, f := range list(listOf["filterable"]) {
		filterable[text(f)] = true
	}
	var filters []value
	for _, fv := range list(pg["filters"]) {
		f := text(fv)
		var query string
		switch {
		case params[f]:
			query = wire(f)
		case filterable[f] && wireNames["filter"] != "":
			query = wireNames["filter"] + "[" + wire(f) + "]"
		default:
			g.problem("error", at+"/filters", "%s filters by %s, which is neither a query parameter of %s nor filterable in its listOf, so the screen cannot ask for it; add it to the operation", pageName, f, sourceID)
			continue
		}
		entry := []member{{"field", str(wire(f))}, {"query", str(query)}, {"title", str(g.say(pageName+".filters."+f, orText(text(obj0(props[f])["title"]), f)))}}
		if values := g.enumValues(obj0(props[f])); values != nil {
			var opts []value
			for _, v := range values {
				opts = append(opts, str(v))
			}
			entry = append(entry, member{"options", array(opts)})
		} else if text(obj0(props[f])["format"]) == "date" {
			entry = append(entry, member{"date", raw("true")})
		}
		filters = append(filters, object(entry))
	}
	var sizes []value
	def, _ := strconv.Atoi(defaultSize)
	max, err := strconv.Atoi(maxSize)
	if err != nil {
		max = def
	}
	offered := map[int]bool{def: true}
	for _, s := range pageSizes {
		if s <= max {
			offered[s] = true
		}
	}
	var ordered []int
	for s := range offered {
		ordered = append(ordered, s)
	}
	sort.Ints(ordered)
	for _, s := range ordered {
		sizes = append(sizes, raw(strconv.Itoa(s)))
	}
	states := obj0(pg["states"])
	failed := obj0(states["failed"])
	pk := list(entity["primaryKey"])
	var pageActions, rowActions []value
	routes := map[string]string{}
	pages := obj0(g.spec["pages"])
	for i, a := range list(pg["actions"]) {
		am := obj0(a)
		ptr := fmt.Sprintf("%s/actions/%d", at, i)
		target := text(am["target"])
		actionPermission := orText(text(am["permission"]), permission)
		labelKey := pageName + ".actions." + target
		if text(am["kind"]) == "navigate" {
			tp, ok := obj(pages[target])
			if !ok {
				g.problem("error", ptr, "the action %s opens %s, which is not a page of the specification", text(am["label"]), target)
				continue
			}
			routes[target] = text(tp["route"])
			if pathParam.MatchString(text(tp["route"])) {
				if row, ok := g.rowLink(ptr, am, labelKey, actionPermission, entity, text(tp["route"])); ok {
					rowActions = append(rowActions, row)
				}
				continue
			}
			pageActions = append(pageActions, object([]member{{"label", str(g.say(labelKey, text(am["label"])))}, {"navigate", str(target)}, {"permission", str(actionPermission)}}))
			continue
		}
		row, ok := g.rowAction(pageName, ptr, am, labelKey, actionPermission, entity, pk, failed)
		if ok {
			rowActions = append(rowActions, row)
		}
	}
	schemaMembers := []member{}
	part := "list-page"
	add := func(role string, v value) { schemaMembers = append(schemaMembers, member{g.name(part, role), v}) }
	add("title", str(g.say(pageName+".title", text(pg["title"]))))
	add("permission", str(permission))
	add("source", object([]member{{"operation", str(sourceID)}, {"path", str(srcPath)}}))
	add("columns", array(columns))
	add("filters", array(filters))
	add("search", raw(strconv.FormatBool(len(list(listOf["searchable"])) > 0)))
	add("pageSize", raw(strconv.Itoa(def)))
	add("pageSizes", array(sizes))
	add("actions", array(pageActions))
	add("rowActions", array(rowActions))
	if sel, ok := obj(pg["onSelect"]); ok {
		target := text(sel["navigate"])
		if target == "" || sel["message"] != nil {
			g.problem("error", at+"/onSelect", "%s says a message when a row is selected, and this version of %s only opens a page from a row", pageName, name)
		} else {
			routes[target] = text(obj0(pages[target])["route"])
			var ws []member
			with := obj0(sel["with"])
			for _, k := range sortedKeys(with) {
				ws = append(ws, member{k, str(wire(text(with[k])))})
			}
			add("select", object([]member{{"navigate", str(target)}, {"with", object(ws)}}))
		}
	}
	add("empty", str(g.say(pageName+".empty", orText(text(obj0(states["empty"])["message"]), "There is nothing here yet."))))
	add("filteredEmpty", str(g.say(pageName+".filteredEmpty", orText(text(obj0(states["filteredEmpty"])["message"]), "Nothing matches these filters."))))
	add("failed", array(g.refusalsOf(pageName, src, failed)))
	add("wire", object([]member{
		{"page", str(wireNames["page"])}, {"pageSize", str(wireNames["pageSize"])}, {"sort", str(wireNames["sort"])}, {"search", str(wireNames["search"])},
		{"items", str(wireNames["items"])}, {"totalItems", str(wireNames["totalItems"])}, {"totalPages", str(wireNames["totalPages"])},
	}))
	if g.failedSince(before) {
		return "", "", "", false
	}
	importFrom := g.importOf(part)
	schemaType := g.name(part, "schemaType")
	component := g.name(part, "component")
	var schema strings.Builder
	fmt.Fprintf(&schema, "import type { %s } from %q;\n\n", schemaType, importFrom)
	schema.WriteString(statement("export const schema = ", object(schemaMembers), " satisfies "+schemaType+";"))
	components := g.name("application", "components")
	var page strings.Builder
	page.WriteString("import type { Metadata } from \"next\";\n")
	if importFrom == components {
		fmt.Fprintf(&page, "import { Guard, %s } from %q;\n", component, components)
	} else {
		fmt.Fprintf(&page, "import { %s } from %q;\nimport { Guard } from %q;\n", component, importFrom, components)
	}
	page.WriteString("import { chosenLanguage } from \"@/language\";\nimport { stringOf, texts } from \"@/strings\";\nimport { schema } from \"./page.schema\";\n\n")
	page.WriteString(metadataOf(pageName))
	g.writeRoutes(&page, routes)
	page.WriteString("export default async function Page() {\n  const t = texts(schema, await chosenLanguage());\n  return (\n")
	fmt.Fprintf(&page, "    <Guard permission={schema.%s} texts={t}>\n", g.name(part, "permission"))
	fmt.Fprintf(&page, "      <%s schema={schema} texts={t} routes={routes} />\n", component)
	page.WriteString("    </Guard>\n  );\n}\n")
	return schema.String(), page.String(), g.refusalTest(pageName, text(pg["route"]), permission, g.name(part, "permission")), true
}

// rowAction reads an action of kind operation on a list's row, or on the
// record a view shows.
func (g *gen) rowAction(pageName, ptr string, am map[string]any, labelKey, permission string, entity map[string]any, pk []any, failed map[string]any) (value, bool) {
	target := text(am["target"])
	op, path, method := g.operation(target)
	if op == nil || method == "get" {
		g.problem("error", ptr, "the action %s runs %s, which is not an operation of the specification that changes something", text(am["label"]), target)
		return nil, false
	}
	props := obj0(entity["properties"])
	parameters := []member{}
	for _, p := range pathParam.FindAllString(path, -1) {
		param := p[1 : len(p)-1]
		field := ""
		switch {
		case props[param] != nil:
			field = param
		case len(pk) == 1:
			field = text(pk[0])
		default:
			g.problem("error", ptr, "%s takes %s in its path, which is neither a field of the row nor its primary key of one field", target, param)
			return nil, false
		}
		parameters = append(parameters, member{param, str(wirename.Of(g.wireNames, field))})
	}
	entry := []member{
		{"label", str(g.say(labelKey+".label", text(am["label"])))},
		{"operation", str(target)},
		{"method", str(strings.ToUpper(method))},
		{"path", str(path)},
		{"parameters", object(parameters)},
		{"permission", str(permission)},
	}
	entry, ok := g.whenOf(entry, ptr, am, props)
	if !ok {
		return nil, false
	}
	if c := text(am["confirm"]); c != "" {
		entry = append(entry, member{"confirm", str(g.say(labelKey+".confirm", c))})
	}
	body := g.bodySchema(op)
	reason := text(am["reason"])
	if body != nil {
		required := list(body["required"])
		if reason == "" || len(required) > 1 || (len(required) == 1 && text(required[0]) != reason) {
			g.problem("error", ptr, "the action %s runs %s, which takes a request body, and a row action sends only the reason its confirmation asks for; name it under reason", text(am["label"]), target)
			return nil, false
		}
		prop := obj0(obj0(body["properties"])[reason])
		r := []member{{"property", str(wirename.Of(g.wireNames, reason))}, {"label", str(g.say(labelKey+".reason", orText(text(prop["title"]), "Reason")))}}
		if m := text(prop["maxLength"]); m != "" {
			r = append(r, member{"maxLength", raw(m)})
		}
		entry = append(entry, member{"reason", object(r)})
	}
	then := obj0(am["then"])
	if text(then["navigate"]) != "" {
		g.problem("error", ptr+"/then", "the action %s leads to %s once it succeeds, and this version of %s stays on the page and shows the message only", text(am["label"]), text(then["navigate"]), name)
		return nil, false
	}
	if m := text(then["message"]); m != "" {
		entry = append(entry, member{"message", str(g.say(labelKey+".message", m))})
	}
	entry = append(entry, member{"failed", array(g.refusalsOf(pageName, op, failed))})
	return object(entry), true
}

// rowLink is a list's action that opens a page whose route takes a
// parameter: offered on each row while its when holds, the parameters
// filled from the row's fields its with names.
func (g *gen) rowLink(ptr string, am map[string]any, labelKey, permission string, entity map[string]any, route string) (value, bool) {
	with := obj0(am["with"])
	for _, p := range pathParam.FindAllString(route, -1) {
		if with[p[1:len(p)-1]] == nil {
			g.problem("error", ptr, "the action %s opens %s at %s, and its with does not give %s from the row", text(am["label"]), text(am["target"]), route, p)
			return nil, false
		}
	}
	if text(am["confirm"]) != "" {
		g.problem("error", ptr+"/confirm", "the action %s opens a page, and this version of %s asks no confirmation before it does", text(am["label"]), name)
		return nil, false
	}
	var ws []member
	for _, k := range sortedKeys(with) {
		ws = append(ws, member{k, str(wirename.Of(g.wireNames, text(with[k])))})
	}
	entry := []member{
		{"label", str(g.say(labelKey+".label", text(am["label"])))},
		{"navigate", str(text(am["target"]))},
		{"with", object(ws)},
		{"permission", str(permission)},
	}
	entry, ok := g.whenOf(entry, ptr, am, obj0(entity["properties"]))
	if !ok {
		return nil, false
	}
	return object(entry), true
}

// whenOf adds an action's when to its entry, as the rule the components
// evaluate on the record, its fields by their wire names.
func (g *gen) whenOf(entry []member, ptr string, am map[string]any, props map[string]any) ([]member, bool) {
	when := text(am["when"])
	if when == "" {
		return entry, true
	}
	node, errs := expr.Parse(when)
	if len(errs) > 0 {
		g.problem("error", ptr+"/when", "the action's when does not parse: %s", errs[0].Message)
		return nil, false
	}
	fields := map[string]string{}
	for f := range props {
		fields[f] = wirename.Of(g.wireNames, f)
	}
	rule, ok := g.rule(node, fields, ptr+"/when")
	if !ok {
		return nil, false
	}
	return append(entry, member{"when", rule}), true
}

// enumValues are the values a field may take, when they are a list.
func (g *gen) enumValues(prop map[string]any) []string {
	if ref, ok := strings.CutPrefix(text(prop["$ref"]), "#/enums/"); ok {
		prop = obj0(obj0(g.spec["enums"])[ref])
	}
	var out []string
	for _, v := range list(prop["enum"]) {
		out = append(out, text(v))
	}
	return out
}

// importOf is where a part's component and schema type come from: its own
// import, or the application's components.
func (g *gen) importOf(part string) string {
	if i := g.names[part]["import"]; i != "" {
		return i
	}
	return g.name("application", "components")
}

// writeRoutes writes the routes a page's events lead to.
func (g *gen) writeRoutes(b *strings.Builder, routes map[string]string) {
	b.WriteString("const routes = {\n")
	for _, target := range sortedKeys(anyMap(routes)) {
		fmt.Fprintf(b, "  %s: %s,\n", key(target), quote(routes[target]))
	}
	b.WriteString("};\n\n")
}

// refusalTest is the derived test of a page that needs a permission: it
// opens to someone who holds it, refuses someone signed in without it,
// and sends someone signed out to sign in. A public page has none.
func (g *gen) refusalTest(pageName, route, permission, permissionKey string) string {
	if permission == "public" {
		return ""
	}
	var b strings.Builder
	b.WriteString("import assert from \"node:assert/strict\";\nimport { test } from \"node:test\";\nimport { decide } from \"../screens/access.ts\";\n")
	fmt.Fprintf(&b, "import { schema } from \"../app%s/page.schema.ts\";\n\n", routeFolder(route))
	fmt.Fprintf(&b, "test(%q, () => {\n  assert.equal(decide(schema.%s, { signedIn: true, permissions: [] }), \"refused\");\n});\n\n", pageName+" is refused to someone signed in without "+permission, permissionKey)
	fmt.Fprintf(&b, "test(%q, () => {\n  assert.equal(decide(schema.%s, { signedIn: false, permissions: [] }), \"sign-in\");\n});\n\n", pageName+" sends someone signed out to sign in", permissionKey)
	fmt.Fprintf(&b, "test(%q, () => {\n  assert.equal(decide(schema.%s, { signedIn: true, permissions: [%s] }), \"allowed\");\n});\n", pageName+" opens to someone who holds "+permission, permissionKey, quote(permission))
	return b.String()
}

// application is application.ts: how the screens read the session, the
// page that signs in, and the menu, each entry with the permission of the
// page it opens; an entry whose page this run does not write is left out,
// unless another stakeholder writes it.
func (g *gen) application(sess *session, drawn map[string]bool) (string, []string) {
	var b strings.Builder
	fmt.Fprintf(&b, "import type { Application } from %q;\n\n", g.name("application", "components"))
	b.WriteString("/**\n * Where the screens call the service, how they read the person's session,\n * where they sign in, and the menu.\n */\n")
	sessionValue := value(raw("null"))
	signIn := value(raw("null"))
	if sess != nil {
		sessionValue = object([]member{{"path", str(sess.path)}, {"signedIn", str(sess.signedIn)}, {"permissions", str(sess.permissions)}})
		if sess.signIn != "" {
			signIn = str(sess.signIn)
		}
	}
	owned := ownership.Of(g.impl.Content)
	pages := obj0(g.spec["pages"])
	menus := obj0(g.spec["menus"])
	var groups []value
	var testsOf []string
	for _, m := range sortedKeys(menus) {
		mp := obj0(menus[m])
		ptr := "#/menus/" + ownership.Escape(m)
		if owned.Covers(ptr) {
			continue
		}
		items := obj0(mp["items"])
		var entries []value
		for _, i := range sortedKeys(items) {
			if owned.Covers(ptr + "/items/" + ownership.Escape(i)) {
				continue
			}
			it := obj0(items[i])
			at := "/menus/" + ownership.Escape(m) + "/items/" + ownership.Escape(i)
			if _, group := it["items"]; group {
				g.problem("error", at, "%s is a group inside the menu %s, and the side navigation draws one level of groups; move its entries into a menu of their own", i, m)
				continue
			}
			pg, ok := obj(pages[text(it["page"])])
			if !ok {
				g.problem("error", at+"/page", "the menu entry %s opens %s, which is not a page of the specification", i, text(it["page"]))
				continue
			}
			if pathParam.MatchString(text(pg["route"])) {
				g.problem("error", at+"/page", "the menu entry %s opens %s at %s, whose route takes a parameter a menu cannot give; open a page whose route takes none", i, text(it["page"]), text(pg["route"]))
				continue
			}
			if !drawn[text(it["page"])] && !owned.Covers(ownership.Entity("pages", text(it["page"]))) {
				if !g.failed() { // a page refused by an error writes nothing at all
					g.problem("warning", at, "the menu entry %s opens %s, which this version of %s does not write; the entry is left out with its page", i, text(it["page"]), name)
				}
				continue
			}
			entries = append(entries, object([]member{
				{"title", str(g.say("menu."+m+".items."+i, text(it["title"])))},
				{"route", str(text(pg["route"]))},
				{"permission", str(text(pg["permission"]))},
			}))
			testsOf = append(testsOf, text(pg["permission"]))
		}
		if len(entries) == 0 {
			continue
		}
		groups = append(groups, object([]member{{"title", str(g.say("menu."+m+".title", text(mp["title"])))}, {"items", array(entries)}}))
	}
	b.WriteString(statement("export const application: Application = ", object([]member{{"service", g.serviceValue()}, {"session", sessionValue}, {"signIn", signIn}, {"menu", array(groups)}}), ";"))
	return b.String(), testsOf
}

// menuTest is the derived test of the menu: an entry is not shown to
// someone without the permission of the page it opens.
func menuTest() string {
	return `import assert from "node:assert/strict";
import { test } from "node:test";
import { application } from "../application.ts";
import { visibleMenu } from "../screens/access.ts";

test("the menu shows no entry to someone signed in without its page's permission", () => {
  for (const group of application.menu) {
    for (const item of group.items) {
      if (item.permission === "public") {
        continue;
      }
      const held = application.menu.flatMap((g) => g.items.map((i) => i.permission)).filter((p) => p !== item.permission);
      const shown = visibleMenu(application.menu, { signedIn: true, permissions: held }).flatMap((g) => g.items);
      assert.ok(!shown.some((entry) => entry.route === item.route && entry.permission === item.permission), item.route);
    }
  }
});

test("the menu shows every entry to someone who holds every page's permission", () => {
  const held = application.menu.flatMap((g) => g.items.map((i) => i.permission));
  const shown = visibleMenu(application.menu, { signedIn: true, permissions: held }).flatMap((g) => g.items);
  assert.equal(shown.length, application.menu.flatMap((g) => g.items).length);
});
`
}
