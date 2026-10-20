package genscreens

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

// Session is what the target's settings say of the person's session: the
// operation that reads it, the properties of its answer, and the page that
// signs in.
type Session struct {
	// Path is the path of the operation that answers who is signed in.
	Path string
	// SignedIn and Permissions are the properties of its answer, on the
	// wire.
	SignedIn, Permissions string
	// SignIn is the route of the page that signs in, or "".
	SignIn string
	// SignInPage is the name of that page, or "".
	SignInPage string
}

// ReadSession reads settings.session, which a page that needs a permission
// cannot do without; nil when the settings name none.
func (g *Gen) ReadSession() *Session {
	s, ok := obj(g.Settings["session"])
	if !ok {
		return nil
	}
	at := "/"
	opID := text(s["operation"])
	op, path, method := g.Operation(opID)
	switch {
	case op == nil || method != "get":
		g.Problem("error", at, "the ui target's settings.session names %q, which is not a GET operation of the specification; name the operation that answers who is signed in", opID)
		return nil
	case PathParam.MatchString(path):
		g.Problem("error", at, "the ui target's settings.session names %s, whose path takes a parameter; the screens read the session at one path", opID)
		return nil
	}
	props := obj0(obj0(obj0(obj0(obj0(obj0(obj0(op["responses"])["200"])["content"])["application/json"])["schema"]))["properties"])
	out := &Session{Path: path}
	for _, k := range []struct{ key, typ string }{{"signedIn", "boolean"}, {"permissions", "array"}} {
		prop := text(s[k.key])
		if prop == "" {
			g.Problem("error", at, "the ui target's settings.session names no %s, the property of %s's answer that holds it", k.key, opID)
			return nil
		}
		if text(obj0(props[prop])["type"]) != k.typ {
			g.Problem("error", at, "the ui target's settings.session says %s holds %s, and the 200 answer of %s has no property %s of type %s", prop, k.key, opID, prop, k.typ)
			return nil
		}
		wire := wirename.Of(g.WireNames, prop)
		if k.key == "signedIn" {
			out.SignedIn = wire
		} else {
			out.Permissions = wire
		}
	}
	if signIn := text(s["signIn"]); signIn != "" {
		pg, ok := obj(obj0(g.Spec["pages"])[signIn])
		if !ok || text(pg["kind"]) != "task" {
			g.Problem("error", at, "the ui target's settings.session names %q as the page that signs in, which is not a task page of the specification", signIn)
			return nil
		}
		out.SignIn = text(pg["route"])
		out.SignInPage = signIn
	}
	return out
}

// ListNames are the paginated-list idiom's names on the wire: its dxlib
// names when the service serves the dxlib dialect, its names for any stack
// otherwise.
func (g *Gen) ListNames() map[string]string {
	stacks := []string{"any"}
	if g.Dialect == "dxlib" {
		stacks = []string{"dxlib"}
	}
	out := map[string]string{}
	for _, stack := range stacks {
		for _, i := range g.Impl.Idioms {
			if i.Name != "paginated-list" || i.As == "excluded" {
				continue
			}
			for _, src := range []map[string]any{i.Override, i.Content} {
				for _, part := range []string{"parameters", "envelope"} {
					for k, v := range obj0(obj0(obj0(obj0(obj0(src["parts"])[part])["stack"])[stack])["names"]) {
						if _, ok := out[k]; !ok {
							out[k] = text(v)
						}
					}
				}
			}
		}
	}
	return out
}

// sortNames are a list's sort field and direction names in the dxlib
// dialect, whose sort is a list of them; none in the standard dialect.
func sortNames(dialect string, wireNames map[string]string) []Member {
	if dialect != "dxlib" {
		return nil
	}
	var out []Member
	for _, k := range []string{"sortField", "sortDirection"} {
		if wireNames[k] != "" {
			out = append(out, Member{k, Str(wireNames[k])})
		}
	}
	return out
}

// refusalsOf are the failed states of a page an operation answers, by
// status, and the page's default with no status.
func (g *Gen) refusalsOf(pageName string, op map[string]any, failed map[string]any) []Value {
	var out []Value
	for _, problem := range sortedKeys(failed) {
		st := obj0(failed[problem])
		if problem == "default" {
			continue
		}
		status := g.ProblemStatus(op, problem)
		if status == "" {
			continue // another operation of the page answers it
		}
		out = append(out, Object([]Member{{"status", Raw(status)}, {"problem", Str(problem)}, {"message", Str(g.Say(pageName+".failed."+problem, text(st["message"])))}}))
	}
	if d, ok := obj(failed["default"]); ok {
		out = append(out, Object([]Member{{"problem", Str("default")}, {"message", Str(g.Say(pageName+".failed.default", text(d["message"])))}}))
	}
	return out
}

// ListPage reads a list page.
func (g *Gen) ListPage(pageName string, pg map[string]any, sess *Session) (*Page, bool) {
	at := "/pages/" + pageName
	name := g.Profile.Name
	for _, k := range []string{"onSubmitted", "pickers"} {
		if _, ok := pg[k]; ok {
			g.Problem("warning", at+"/"+k, "%s gives %s, which this version of %s does not read on a list; the list is drawn without it", pageName, k, name)
		}
	}
	before := len(g.Diags)
	entityName := text(pg["entity"])
	entity := obj0(obj0(g.Spec["entities"])[entityName])
	props := obj0(entity["properties"])
	wire := func(f string) string { return wirename.Of(g.WireNames, f) }
	sourceID := text(pg["source"])
	src, srcPath, srcMethod := g.Operation(sourceID)
	if src == nil || srcMethod != "get" {
		g.Problem("error", at+"/source", "%s reads %s, which is not a GET operation of the specification; a list is read by GET", pageName, sourceID)
		return nil, false
	}
	if !g.PlainOnly(at+"/source", pageName, "reads", sourceID, src, "") {
		return nil, false
	}
	if PathParam.MatchString(srcPath) {
		g.Problem("error", at+"/source", "%s reads %s at %s, whose path takes a parameter, and this version of %s reads a list whose path takes none", pageName, sourceID, srcPath, name)
	}
	listOf, ok := obj(src["listOf"])
	if g.Profile.PagedByServer != nil {
		if pageSize, maximum, named := g.Profile.PagedByServer(sourceID); named && !ok {
			listOf, ok = map[string]any{"pageSize": map[string]any{"default": pageSize, "maximum": maximum}}, true
		}
	}
	size := obj0(listOf["pageSize"])
	defaultSize, maxSize := text(size["default"]), text(size["maximum"])
	if defaultSize == "" {
		defaultSize = maxSize
	}
	wireNames := g.ListNames()
	switch {
	case !ok && g.Profile.PagedByServer != nil:
		g.Problem("error", at+"/source", "%s pages through its list, and %s has no listOf, so it does not page; give it listOf, or name it under the ui target's settings.server.pages for a server route to page it", pageName, sourceID)
	case !ok:
		g.Problem("error", at+"/source", "%s pages through its list, and %s has no listOf, so it does not page; give it listOf", pageName, sourceID)
	case defaultSize == "":
		g.Problem("error", at+"/source", "%s pages through its list, and the listOf of %s gives no page size; give pageSize a default", pageName, sourceID)
	case wireNames["page"] == "" || wireNames["items"] == "":
		g.Problem("error", at+"/source", "%s pages through the paginated-list idiom, which the implementation file excludes; keep the idiom", pageName)
	}
	if text(pg["enabledBy"]) != "" {
		g.Problem("error", at+"/enabledBy", "%s is switched on by %s, and this version of %s does not read configuration, so the page would be served while it is off", pageName, text(pg["enabledBy"]), name)
	}
	permission := text(pg["permission"])
	if permission != "public" && sess == nil {
		g.Problem("error", at+"/permission", "%s needs %s, and the ui target's settings name no session to read who holds it; add settings.session with the operation that answers it", pageName, permission)
	}
	if g.FailedSince(before) {
		return nil, false
	}
	sortable := map[string]bool{}
	for _, f := range list(listOf["sortable"]) {
		sortable[text(f)] = true
	}
	compact := map[string]bool{}
	for _, c := range list(pg["compactColumns"]) {
		compact[text(c)] = true
	}
	var columns []Value
	for i, c := range list(pg["columns"]) {
		f := text(c)
		if schemaName, _ := ValueOf(obj0(props[f])); schemaName != "" {
			g.Problem("error", fmt.Sprintf("%s/columns/%d", at, i), "%s shows %s in a column, which holds a value of %s, and a cell shows one text; show the value on a view", pageName, f, schemaName)
			continue
		}
		columns = append(columns, Object([]Member{
			{"field", Str(wire(f))},
			{"title", Str(g.Say(pageName+".columns."+f, orText(text(obj0(props[f])["title"]), f)))},
			{"sortable", Raw(strconv.FormatBool(sortable[f]))},
			{"compact", Raw(strconv.FormatBool(len(compact) == 0 || compact[f]))},
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
	var filters []Value
	for _, fv := range list(pg["filters"]) {
		f := text(fv)
		var query string
		switch {
		case params[f]:
			query = wire(f)
		case filterable[f] && wireNames["filter"] != "":
			query = wireNames["filter"] + "[" + wire(f) + "]"
		default:
			g.Problem("error", at+"/filters", "%s filters by %s, which is neither a query parameter of %s nor filterable in its listOf, so the screen cannot ask for it; add it to the operation", pageName, f, sourceID)
			continue
		}
		entry := []Member{{"field", Str(wire(f))}, {"query", Str(query)}, {"title", Str(g.Say(pageName+".filters."+f, orText(text(obj0(props[f])["title"]), f)))}}
		if values := g.EnumValues(obj0(props[f])); values != nil {
			var opts []Value
			for _, v := range values {
				opts = append(opts, Str(v))
			}
			entry = append(entry, Member{"options", Array(opts)})
		} else if text(obj0(props[f])["format"]) == "date" {
			entry = append(entry, Member{"date", Raw("true")})
		}
		filters = append(filters, Object(entry))
	}
	var sizes []Value
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
		sizes = append(sizes, Raw(strconv.Itoa(s)))
	}
	states := obj0(pg["states"])
	failed := obj0(states["failed"])
	pk := list(entity["primaryKey"])
	var pageActions, rowActions []Value
	routes := map[string]string{}
	pages := obj0(g.Spec["pages"])
	for i, a := range list(pg["actions"]) {
		am := obj0(a)
		ptr := fmt.Sprintf("%s/actions/%d", at, i)
		target := text(am["target"])
		actionPermission := orText(text(am["permission"]), permission)
		labelKey := pageName + ".actions." + target
		if text(am["kind"]) == "navigate" {
			tp, ok := obj(pages[target])
			if !ok {
				g.Problem("error", ptr, "the action %s opens %s, which is not a page of the specification", text(am["label"]), target)
				continue
			}
			routes[target] = text(tp["route"])
			if PathParam.MatchString(text(tp["route"])) {
				if row, ok := g.rowLink(ptr, am, labelKey, actionPermission, entity, text(tp["route"])); ok {
					rowActions = append(rowActions, row)
				}
				continue
			}
			pageActions = append(pageActions, Object([]Member{{"label", Str(g.Say(labelKey, text(am["label"])))}, {"navigate", Str(target)}, {"permission", Str(actionPermission)}}))
			continue
		}
		row, ok := g.rowAction(pageName, ptr, am, labelKey, actionPermission, entity, pk, failed)
		if ok {
			rowActions = append(rowActions, row)
		}
	}
	schema := []Member{}
	part := "list-page"
	add := func(role string, v Value) { schema = append(schema, Member{g.Name(part, role), v}) }
	add("title", Str(g.Say(pageName+".title", text(pg["title"]))))
	add("permission", Str(permission))
	add("source", Object(append([]Member{{"operation", Str(sourceID)}, {"path", Str(srcPath)}}, g.Transport(src, srcPath)...)))
	add("columns", Array(columns))
	add("filters", Array(filters))
	add("search", Raw(strconv.FormatBool(len(list(listOf["searchable"])) > 0)))
	add("pageSize", Raw(strconv.Itoa(def)))
	add("pageSizes", Array(sizes))
	add("actions", Array(pageActions))
	add("rowActions", Array(rowActions))
	if sel, ok := obj(pg["onSelect"]); ok {
		target := text(sel["navigate"])
		if target == "" || sel["message"] != nil {
			g.Problem("error", at+"/onSelect", "%s says a message when a row is selected, and this version of %s only opens a page from a row", pageName, name)
		} else {
			routes[target] = text(obj0(pages[target])["route"])
			var ws []Member
			with := obj0(sel["with"])
			for _, k := range sortedKeys(with) {
				ws = append(ws, Member{k, Str(wire(text(with[k])))})
			}
			add("select", Object([]Member{{"navigate", Str(target)}, {"with", Dictionary(ws)}}))
		}
	}
	add("empty", Str(g.Say(pageName+".empty", orText(text(obj0(states["empty"])["message"]), "There is nothing here yet."))))
	add("filteredEmpty", Str(g.Say(pageName+".filteredEmpty", orText(text(obj0(states["filteredEmpty"])["message"]), "Nothing matches these filters."))))
	add("failed", Array(g.refusalsOf(pageName, src, failed)))
	add("wire", Object(append([]Member{
		{"page", Str(wireNames["page"])}, {"pageSize", Str(wireNames["pageSize"])}, {"sort", Str(wireNames["sort"])}, {"search", Str(wireNames["search"])},
		{"items", Str(wireNames["items"])}, {"totalItems", Str(wireNames["totalItems"])}, {"totalPages", Str(wireNames["totalPages"])},
	}, sortNames(g.Dialect, wireNames)...)))
	if g.FailedSince(before) {
		return nil, false
	}
	return &Page{Name: pageName, Kind: "list", Part: part, Route: text(pg["route"]), Permission: permission, Schema: schema, Routes: routes}, true
}

// rowAction reads an action of kind operation on a list's row, or on the
// record a view shows.
func (g *Gen) rowAction(pageName, ptr string, am map[string]any, labelKey, permission string, entity map[string]any, pk []any, failed map[string]any) (Value, bool) {
	target := text(am["target"])
	op, path, method := g.Operation(target)
	if op == nil || method == "get" {
		g.Problem("error", ptr, "the action %s runs %s, which is not an operation of the specification that changes something", text(am["label"]), target)
		return nil, false
	}
	if !g.PlainOnly(ptr, "the action "+text(am["label"]), "runs", target, op, "download") {
		return nil, false
	}
	props := obj0(entity["properties"])
	parameters := []Member{}
	for _, p := range PathParam.FindAllString(path, -1) {
		param := p[1 : len(p)-1]
		field := ""
		switch {
		case props[param] != nil:
			field = param
		case len(pk) == 1:
			field = text(pk[0])
		default:
			g.Problem("error", ptr, "%s takes %s in its path, which is neither a field of the row nor its primary key of one field", target, param)
			return nil, false
		}
		parameters = append(parameters, Member{param, Str(wirename.Of(g.WireNames, field))})
	}
	entry := []Member{
		{"label", Str(g.Say(labelKey+".label", text(am["label"])))},
		{"operation", Str(target)},
		{"method", Str(strings.ToUpper(method))},
		{"path", Str(path)},
	}
	entry = append(entry, g.Transport(op, path)...)
	entry = append(entry, Member{"parameters", Dictionary(parameters)}, Member{"permission", Str(permission)})
	entry, ok := g.whenOf(entry, ptr, am, props)
	if !ok {
		return nil, false
	}
	if c := text(am["confirm"]); c != "" {
		entry = append(entry, Member{"confirm", Str(g.Say(labelKey+".confirm", c))})
	}
	body := g.BodySchema(op)
	reason := text(am["reason"])
	if body != nil {
		required := list(body["required"])
		if reason == "" || len(required) > 1 || (len(required) == 1 && text(required[0]) != reason) {
			g.Problem("error", ptr, "the action %s runs %s, which takes a request body, and a row action sends only the reason its confirmation asks for; name it under reason", text(am["label"]), target)
			return nil, false
		}
		prop := obj0(obj0(body["properties"])[reason])
		r := []Member{{"property", Str(wirename.Of(g.WireNames, reason))}, {"label", Str(g.Say(labelKey+".reason", orText(text(prop["title"]), "Reason")))}}
		if m := text(prop["maxLength"]); m != "" {
			r = append(r, Member{"maxLength", Raw(m)})
		}
		entry = append(entry, Member{"reason", Object(r)})
	}
	then := obj0(am["then"])
	if text(then["navigate"]) != "" {
		g.Problem("error", ptr+"/then", "the action %s leads to %s once it succeeds, and this version of %s stays on the page and shows the message only", text(am["label"]), text(then["navigate"]), g.Profile.Name)
		return nil, false
	}
	if m := text(then["message"]); m != "" {
		entry = append(entry, Member{"message", Str(g.Say(labelKey+".message", m))})
	}
	entry = append(entry, Member{"failed", Array(g.refusalsOf(pageName, op, failed))})
	return Object(entry), true
}

// rowLink is a list's action that opens a page whose route takes a
// parameter: offered on each row while its when holds, the parameters
// filled from the row's fields its with names.
func (g *Gen) rowLink(ptr string, am map[string]any, labelKey, permission string, entity map[string]any, route string) (Value, bool) {
	with := obj0(am["with"])
	for _, p := range PathParam.FindAllString(route, -1) {
		if with[p[1:len(p)-1]] == nil {
			g.Problem("error", ptr, "the action %s opens %s at %s, and its with does not give %s from the row", text(am["label"]), text(am["target"]), route, p)
			return nil, false
		}
	}
	if text(am["confirm"]) != "" {
		g.Problem("error", ptr+"/confirm", "the action %s opens a page, and this version of %s asks no confirmation before it does", text(am["label"]), g.Profile.Name)
		return nil, false
	}
	var ws []Member
	for _, k := range sortedKeys(with) {
		ws = append(ws, Member{k, Str(wirename.Of(g.WireNames, text(with[k])))})
	}
	entry := []Member{
		{"label", Str(g.Say(labelKey+".label", text(am["label"])))},
		{"navigate", Str(text(am["target"]))},
		{"with", Dictionary(ws)},
		{"permission", Str(permission)},
	}
	entry, ok := g.whenOf(entry, ptr, am, obj0(entity["properties"]))
	if !ok {
		return nil, false
	}
	return Object(entry), true
}

// whenOf adds an action's when to its entry, as the rule the components
// evaluate on the record, its fields by their wire names.
func (g *Gen) whenOf(entry []Member, ptr string, am map[string]any, props map[string]any) ([]Member, bool) {
	when := text(am["when"])
	if when == "" {
		return entry, true
	}
	node, errs := expr.Parse(when)
	if len(errs) > 0 {
		g.Problem("error", ptr+"/when", "the action's when does not parse: %s", errs[0].Message)
		return nil, false
	}
	fields := map[string]string{}
	for f := range props {
		fields[f] = wirename.Of(g.WireNames, f)
	}
	rule, ok := g.Rule(node, fields, ptr+"/when")
	if !ok {
		return nil, false
	}
	return append(entry, Member{"when", rule}), true
}

// EnumValues are the values a field may take, when they are a list.
func (g *Gen) EnumValues(prop map[string]any) []string {
	if ref, ok := strings.CutPrefix(text(prop["$ref"]), "#/enums/"); ok {
		prop = obj0(obj0(g.Spec["enums"])[ref])
	}
	var out []string
	for _, v := range list(prop["enum"]) {
		out = append(out, text(v))
	}
	return out
}

// Menu is the menu: each group with its entries, each entry with the
// title, route and permission of the page it opens, and the permissions
// the entries need, in order; an entry whose page this run does not write
// is left out, unless another stakeholder writes it. drawn names the pages
// written.
func (g *Gen) Menu(drawn map[string]bool) ([]Value, []string) {
	owned := ownership.Of(g.Impl.Content)
	pages := obj0(g.Spec["pages"])
	menus := obj0(g.Spec["menus"])
	var groups []Value
	var permissions []string
	for _, m := range sortedKeys(menus) {
		mp := obj0(menus[m])
		ptr := "#/menus/" + ownership.Escape(m)
		if owned.Covers(ptr) {
			continue
		}
		items := obj0(mp["items"])
		var entries []Value
		for _, i := range sortedKeys(items) {
			if owned.Covers(ptr + "/items/" + ownership.Escape(i)) {
				continue
			}
			it := obj0(items[i])
			at := "/menus/" + ownership.Escape(m) + "/items/" + ownership.Escape(i)
			if _, group := it["items"]; group {
				g.Problem("error", at, "%s is a group inside the menu %s, and the side navigation draws one level of groups; move its entries into a menu of their own", i, m)
				continue
			}
			pg, ok := obj(pages[text(it["page"])])
			if !ok {
				g.Problem("error", at+"/page", "the menu entry %s opens %s, which is not a page of the specification", i, text(it["page"]))
				continue
			}
			if PathParam.MatchString(text(pg["route"])) {
				g.Problem("error", at+"/page", "the menu entry %s opens %s at %s, whose route takes a parameter a menu cannot give; open a page whose route takes none", i, text(it["page"]), text(pg["route"]))
				continue
			}
			if !drawn[text(it["page"])] && !owned.Covers(ownership.Entity("pages", text(it["page"]))) {
				if !g.Failed() { // a page refused by an error writes nothing at all
					g.Problem("warning", at, "the menu entry %s opens %s, which this version of %s does not write; the entry is left out with its page", i, text(it["page"]), g.Profile.Name)
				}
				continue
			}
			entries = append(entries, Object([]Member{
				{"title", Str(g.Say("menu."+m+".items."+i, text(it["title"])))},
				{"route", Str(text(pg["route"]))},
				{"permission", Str(text(pg["permission"]))},
			}))
			permissions = append(permissions, text(pg["permission"]))
		}
		if len(entries) == 0 {
			continue
		}
		groups = append(groups, Object([]Member{{"title", Str(g.Say("menu."+m+".title", text(mp["title"])))}, {"items", Array(entries)}}))
	}
	return groups, permissions
}

// Owned are the ownership marks of the implementation file.
func (g *Gen) Owned() ownership.Owned {
	return ownership.Of(g.Impl.Content)
}
