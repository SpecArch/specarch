// Package genvuehtm writes the screens of a specification for the web with
// no build step, on Vue's runtime-only build and htm: the logic of the
// specarch-gen-ui-vue-htm plug-in. The menus, pages and actions become
// plain data files under config/, each page one data file under pages/ in
// one fixed shape, the components that read them are written once under
// screens/, every call goes through one api.call, and index.html loads the
// vendored files the implementation file names with their integrity, with
// nothing inline. The pages are read through internal/genscreens, which
// the TypeScript and SwiftUI generators read them through too; what draws
// each page kind and field type, and the keys of its data, are the names
// of the ui-components idiom for vue-htm.
package genvuehtm

import (
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/genscreens"
	"github.com/SpecArch/specarch/internal/genui"
	"github.com/SpecArch/specarch/internal/ownership"
	"github.com/SpecArch/specarch/internal/wirename"
	"github.com/SpecArch/specarch/internal/xref"
)

// Request is what specarch writes on a plug-in's standard input.
type Request = genscreens.Request

// Decode reads a request, keeping numbers as their text.
func Decode(data []byte) (*Request, error) { return genscreens.Decode(data) }

// The plug-in's name, as its header and its messages give it.
const name = "specarch-gen-ui-vue-htm"

// The framework this generator writes, the stack the idiom renders.
const framework = "vue-htm"

//go:embed screens
var screens embed.FS

// The screens' own texts beyond the shared ones, the generator's words
// where a page has none.
var vueStrings = map[string]string{
	"screens.pageSize":      "Rows per page",
	"screens.page":          "Page {count}",
	"screens.previous":      "Previous",
	"screens.missingPage":   "There is no page at {pattern}.",
	"screens.missingModule": "The screen {pattern} did not load; the module beside the pages is missing or failed.",
	"screens.file":          "File",
	"screens.remove":        "Remove",
	"screens.noScript":      "These screens need JavaScript.",
}

type (
	value  = genscreens.Value
	member = genscreens.Member
)

var (
	str        = genscreens.Str
	raw        = genscreens.Raw
	object     = genscreens.Object
	array      = genscreens.Array
	obj0       = genscreens.Obj0
	text       = genscreens.Text
	list       = genscreens.List
	sortedKeys = genscreens.SortedKeys
	pathParam  = genscreens.PathParam
	flat       = genscreens.Flat
	quote      = genscreens.Quote
	key        = genscreens.Key
	statement  = genscreens.Statement
)

func anyMap(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

type gen struct {
	*genscreens.Gen
	// actions are the operations the pages call, by operationId, each with
	// how it travels, gathered from the pages' calls (config/actions.js).
	actions map[string]*action
	// customPages are the custom pages written, whose screens app.js
	// imports from custom/<page>.js; hooked the forms with hooks, whose
	// hooks it imports from hooks/<page>.js.
	customPages []string
	hooked      []string
	// hooksOf are the fields of each hooked form whose starting value a
	// hook gives, by page.
	hooksOf map[string][]string
}

// Generate writes the screens of a request.
func Generate(r *Request) genui.Response {
	g := &gen{actions: map[string]*action{}, hooksOf: map[string][]string{}}
	g.Gen = genscreens.New(r, genscreens.Profile{Name: name, Platform: "web", Framework: framework, Language: "JavaScript", Where: "the browser", OtherMode: "has no media query of its own on the web", Seals: true, Streams: true})
	if g.Failed() {
		return g.Response(nil)
	}
	g.Reach = g.reach
	if _, named := g.Settings["server"]; named {
		g.Problem("error", "/", "the ui target's settings.server names server routes, which %s does not write: these screens are files a server hands out as they are, and they call the service directly; take settings.server out", name)
	}
	vendored := g.readVendor(r.Existing)
	header := "// " + g.Header(r) + "\n"
	var files []genui.File
	pageOf := map[string]string{} // the page each page's file is written for, by its path
	add := func(path, content string) {
		files = append(files, genui.File{Path: path, Content: header + content})
	}
	sess := g.ReadSession()
	pages := obj0(g.Spec["pages"])
	owned := g.Owned()
	drawn := map[string]bool{}
	var written []*genscreens.Page
	hooks, layouts := g.FormSettings()
	for _, pageName := range sortedKeys(pages) {
		pg := obj0(pages[pageName])
		if owned.Covers(ownership.Entity("pages", pageName)) {
			continue // another stakeholder's page; a link to it stays
		}
		var page *genscreens.Page
		var ok bool
		switch text(pg["kind"]) {
		case "task":
			page, ok = g.TaskPage(pageName, pg)
		case "list":
			page, ok = g.ListPage(pageName, pg, sess)
		case "form":
			page, ok = g.FormPage(pageName, pg, sess, hooks[pageName], layouts[pageName])
		case "view":
			page, ok = g.ViewPage(pageName, pg, sess)
		case "content":
			page, ok = g.ContentPage(pageName, pg)
		case "custom":
			page, ok = g.CustomPage(pageName, pg)
		default:
			g.Problem("warning", "/pages/"+pageName, "%s is a %s, which this version of %s does not write; it is left out", pageName, text(pg["kind"]), name)
			continue
		}
		if !ok {
			continue
		}
		files = append(files, genui.File{Path: "pages/" + pageName + ".js", Content: header + g.pageFile(page), Comparison: "object-literal"})
		pageOf["pages/"+pageName+".js"] = pageName
		drawn[pageName] = true
		written = append(written, page)
		if page.Kind == "custom" {
			g.customPages = append(g.customPages, pageName)
		}
		if len(page.Hooks) > 0 {
			g.hooked = append(g.hooked, pageName)
			g.hooksOf[pageName] = page.Hooks
		}
		if test := g.refusalTest(page); test != "" {
			add("tests/"+pageName+".test.js", test)
			pageOf["tests/"+pageName+".test.js"] = pageName
		}
	}
	if sess != nil {
		g.sessionAction(sess)
	}
	homes := g.Homes(drawn, sess)
	groups, menuPermissions := g.Menu(drawn)
	add("config/menus.js", g.menusFile(groups))
	add("config/pages.js", g.pagesFile(written))
	add("config/actions.js", g.actionsFile())
	add("config/application.js", g.applicationFile(sess, homes))
	add("app.js", g.appFile(written))
	add("api.js", apiFile)
	add("transport.js", genui.TransportFile())
	if callback, ok := g.signInCallbackPath(); ok {
		add("identity-provider.js", genui.IdentityProviderFile(callback))
	}
	if len(menuPermissions) > 0 {
		add("tests/menu.test.js", menuTest)
	}
	strs, _ := g.stringsFile(r.Existing)
	add("strings.js", strs)
	entries, _ := screens.ReadDir("screens")
	for _, e := range entries {
		data, _ := screens.ReadFile("screens/" + e.Name())
		if strings.HasSuffix(e.Name(), ".css") {
			files = append(files, genui.File{Path: "screens/" + e.Name(), Content: "/* " + g.Header(r) + " */\n" + string(data)})
			continue
		}
		add("screens/"+e.Name(), string(data))
	}
	add("screens/table.js", g.tableFile())
	files = append(files, genui.File{Path: "theme.css", Content: "/* " + g.Header(r) + " */\n" + genui.ThemeCSS(g.Spec, g.Settings, name, g.Problem)})
	files = append(files, genui.File{Path: "index.html", Content: "<!-- " + g.Header(r) + " -->\n" + g.indexFile(vendored)})
	add("vendor/manifest.js", g.manifestFile(vendored))
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if !r.Check && g.handWritten(files, r.Existing, r) {
		return g.Response(nil)
	}
	g.mark(r, files, header, pageOf, drawn)
	return g.Response(files)
}

// handWritten refuses to write over a file in the output folder this
// plug-in did not write, each kind of file by the header line its comment
// syntax starts with.
func (g *gen) handWritten(files, existing []genui.File, r *Request) bool {
	var js, css, html []genui.File
	for _, f := range files {
		switch {
		case strings.HasSuffix(f.Path, ".css"):
			css = append(css, f)
		case strings.HasSuffix(f.Path, ".html"):
			html = append(html, f)
		default:
			js = append(js, f)
		}
	}
	found := g.HandWritten(js, existing, "// Generated by "+name+" ")
	found = g.HandWritten(css, existing, "/* Generated by "+name+" ") || found
	return g.HandWritten(html, existing, "<!-- Generated by "+name+" ") || found
}

// mark writes the draft notice and the problem marks under the header of
// the JavaScript files, and the draft notice alone under the header of
// theme.css and index.html, in their own comment syntax.
func (g *gen) mark(r *Request, files []genui.File, header string, pageOf map[string]string, drawn map[string]bool) {
	var js []genui.File
	index := map[string]int{}
	for i, f := range files {
		if strings.HasSuffix(f.Path, ".css") || strings.HasSuffix(f.Path, ".html") {
			if r.Draft != "" {
				open, close := "/* ", " */"
				if strings.HasSuffix(f.Path, ".html") {
					open, close = "<!-- ", " -->"
				}
				first := strings.Index(f.Content, "\n") + 1
				files[i].Content = f.Content[:first] + open + r.Draft + close + "\n" + f.Content[first:]
			}
			continue
		}
		index[f.Path] = i
		js = append(js, f)
	}
	g.Mark(r, js, header, "// ", pageOf, drawn, "app.js")
	for _, f := range js {
		files[index[f.Path]] = f
	}
}

// pageFile is pages/<page>.js, one page as data in one shape for every
// kind: its name, kind, route and permission, its schema in the idiom's
// names with each call reduced to its operation's name, and the routes
// its events lead to.
func (g *gen) pageFile(p *genscreens.Page) string {
	members := []member{
		{Key: "name", Val: str(p.Name)},
		{Key: "kind", Val: str(p.Kind)},
		{Key: "route", Val: str(p.Route)},
		{Key: "permission", Val: str(p.Permission)},
		{Key: "schema", Val: g.reduce(object(p.Schema))},
	}
	var routes []member
	for _, target := range sortedKeys(anyMap(p.Routes)) {
		routes = append(routes, member{Key: target, Val: str(p.Routes[target])})
	}
	members = append(members, member{Key: "routes", Val: genscreens.Dictionary(routes)})
	return statement("export const page = ", object(members), ";")
}

// menusFile is config/menus.js: the menu as data, each group with its
// entries, each entry with the title, route and permission of the page
// it opens.
func (g *gen) menusFile(groups []value) string {
	var b strings.Builder
	b.WriteString("/** The application's menu: each group with its entries, each entry the page it opens by its route, shown to who holds the page's permission. */\n")
	b.WriteString(statement("export const menus = ", array(groups), ";"))
	return b.String()
}

// pagesFile is config/pages.js: every page written, by name, with its
// kind, its title's key, its route, its permission and its data file.
func (g *gen) pagesFile(pages []*genscreens.Page) string {
	var members []member
	for _, p := range pages {
		members = append(members, member{Key: p.Name, Val: object([]member{
			{Key: "kind", Val: str(p.Kind)},
			{Key: "title", Val: str(p.Name + ".title")},
			{Key: "route", Val: str(p.Route)},
			{Key: "permission", Val: str(p.Permission)},
			{Key: "file", Val: str("pages/" + p.Name + ".js")},
		})})
	}
	var b strings.Builder
	b.WriteString("/** The pages, by name: each one's kind, the key of its title, its route with its {parameters}, its permission and the data file app.js draws it from. */\n")
	b.WriteString(statement("export const pages = ", genscreens.Dictionary(members), ";"))
	return b.String()
}

// applicationFile is config/application.js: where the screens call the
// service, its dialect and handshakes, how the session is read, the page
// that signs in, the roles' homes, how the routes are written, and the
// languages.
func (g *gen) applicationFile(sess *genscreens.Session, homes []genscreens.Home) string {
	sessionValue := raw("null")
	signIn := raw("null")
	if sess != nil {
		fields := []member{{Key: "operation", Val: str(text(obj0(g.Settings["session"])["operation"]))}, {Key: "signedIn", Val: str(sess.SignedIn)}, {Key: "permissions", Val: str(sess.Permissions)}}
		if sess.Roles != "" {
			fields = append(fields, member{Key: "roles", Val: str(sess.Roles)})
		}
		sessionValue = object(fields)
		if sess.SignIn != "" {
			signIn = str(sess.SignIn)
		}
	}
	members := []member{{Key: "title", Val: str(text(obj0(g.Spec["info"])["title"]))}, {Key: "service", Val: str("")}}
	if g.Dialect == "dxlib" {
		members = append(members, member{Key: "dialect", Val: str("dxlib")})
		var hs []member
		for _, h := range sortedKeys(anyMap(g.Handshakes)) {
			hs = append(hs, member{Key: h, Val: object([]member{{Key: "operation", Val: str(g.Handshakes[h])}})})
		}
		if len(hs) > 0 {
			members = append(members, member{Key: "handshakes", Val: genscreens.Dictionary(hs)})
		}
	} else {
		members = append(members, member{Key: "dialect", Val: str("standard")})
	}
	members = append(members, member{Key: "session", Val: sessionValue}, member{Key: "signIn", Val: signIn})
	if len(homes) > 0 {
		var hs []member
		for _, h := range homes {
			hs = append(hs, member{Key: h.Role, Val: str(h.Route)})
		}
		members = append(members, member{Key: "homes", Val: genscreens.Dictionary(hs)})
	}
	members = append(members, member{Key: "routing", Val: g.routing()})
	languages := []value{str(text(g.Settings["language"]))}
	for _, l := range list(g.Settings["translations"]) {
		languages = append(languages, str(text(l)))
	}
	members = append(members, member{Key: "languages", Val: array(languages)})
	var b strings.Builder
	b.WriteString("/**\n * Where the screens call the service and in which dialect, how they read\n * the person's session, where they sign in, each role's home, how a page's\n * route is written in the address bar, and the languages.\n */\n")
	b.WriteString(statement("export const application = ", object(members), ";"))
	return b.String()
}

// routing is how a page's route is written in the address bar: a path
// under the design's application route when it declares one, which the
// server answers the document at every page route (ADR-216), and a
// fragment after # otherwise, so a folder served as it is still works.
func (g *gen) routing() value {
	if base, ok := g.applicationBase(); ok {
		return object([]member{{Key: "mode", Val: str("path")}, {Key: "base", Val: str(base)}})
	}
	return object([]member{{Key: "mode", Val: str("hash")}})
}

// applicationBase is the path of the design's application route without
// its trailing slash, and whether the design declares one.
func (g *gen) applicationBase() (string, bool) {
	routes := obj0(g.Spec["routes"])
	for _, r := range sortedKeys(routes) {
		route := obj0(routes[r])
		if text(route["serves"]) == "application" {
			return strings.TrimSuffix(text(route["path"]), "/"), true
		}
	}
	return "", false
}

// appFile is app.js: the pages' data imported once, the project's screens
// and hooks imported when their page opens, and the shell mounted.
func (g *gen) appFile(pages []*genscreens.Page) string {
	var b strings.Builder
	b.WriteString("import { mount } from \"./screens/app.js\";\n")
	for _, p := range pages {
		fmt.Fprintf(&b, "import { page as %s } from %q;\n", identifier(p.Name), "./pages/"+p.Name+".js")
	}
	b.WriteString("\n/** The pages, by name, each read from its data file. */\nconst pages = {\n")
	for _, p := range pages {
		fmt.Fprintf(&b, "  %s: %s,\n", key(p.Name), identifier(p.Name))
	}
	b.WriteString("};\n\n")
	b.WriteString("/** The screens the project writes for its custom pages, each imported when its page opens, so one that is missing is said on that page alone. */\nconst custom = {\n")
	for _, p := range g.customPages {
		fmt.Fprintf(&b, "  %s: () => import(%q),\n", key(p), "./custom/"+p+".js")
	}
	b.WriteString("};\n\n")
	b.WriteString("/** The hooks the project writes for its forms, each module imported when its form opens, one named export per field the settings name, keyed here by the field's name on the wire. */\nconst hooks = {\n")
	for _, p := range g.hooked {
		var hs []string
		for _, h := range g.hooksOf[p] {
			hs = append(hs, key(wirename.Of(g.WireNames, h))+": module."+h)
		}
		fmt.Fprintf(&b, "  %s: () => import(%q).then((module) => ({ %s })),\n", key(p), "./hooks/"+p+".js", strings.Join(hs, ", "))
	}
	b.WriteString("};\n\n")
	b.WriteString("mount(document.getElementById(\"app\"), { pages, custom, hooks });\n")
	return b.String()
}

// identifier is a page's name as a JavaScript identifier: its words run
// together, the first lower-case and the rest capitalised.
func identifier(pageName string) string {
	var b strings.Builder
	up := false
	for _, r := range pageName {
		switch {
		case r == '-' || r == '_' || r == '.':
			up = true
		case up:
			b.WriteString(strings.ToUpper(string(r)))
			up = false
		default:
			b.WriteRune(r)
		}
	}
	id := b.String()
	if id == "" || (id[0] >= '0' && id[0] <= '9') || reserved[id] {
		return "page" + strings.ToUpper(id[:1]) + id[1:]
	}
	return id
}

// reserved are the words a JavaScript identifier cannot be.
var reserved = map[string]bool{"default": true, "delete": true, "export": true, "import": true, "new": true, "class": true, "function": true, "return": true, "switch": true, "case": true, "break": true, "continue": true, "const": true, "let": true, "var": true, "if": true, "else": true, "for": true, "while": true, "do": true, "in": true, "of": true, "this": true, "null": true, "true": true, "false": true, "void": true, "typeof": true, "with": true, "yield": true, "await": true, "async": true, "static": true, "super": true, "extends": true, "enum": true, "try": true, "catch": true, "finally": true, "throw": true, "debugger": true, "instanceof": true, "package": true, "private": true, "protected": true, "public": true, "interface": true, "implements": true}

// tableFile is screens/table.js: the table component the list page draws
// its rows through, imported from the module the idiom's list-page part
// names, so a project's table replaces the shipped one by that part alone.
func (g *gen) tableFile() string {
	component := g.Name("list-page", "table")
	module := g.Name("list-page", "tableModule")
	return fmt.Sprintf("/** The table the list page draws its rows through: the ui-components idiom's list-page part names it and its module, and an override of that part replaces it. */\nexport { %s as Table } from %s;\n", component, quote(module))
}

// signInCallbackPath is the path of the operation the specification's
// signIn names as its callback, which identity-provider.js sends the
// person back to; false when it declares no signIn or the callback is not
// an operation, which genscreens reports.
func (g *gen) signInCallbackPath() (string, bool) {
	callback := text(obj0(g.Spec["signIn"])["callback"])
	if callback == "" {
		return "", false
	}
	if _, path, _ := g.Operation(callback); path != "" {
		return path, true
	}
	return "", false
}

// reach is where the screens call another module's operation: the address
// the target's settings.services gives its dependency, a URL or a path of
// the screens' own origin, in the standard dialect (ADR-184).
func (g *gen) reach(dep string, d xref.Dependency) (string, string) {
	base := text(obj0(g.Settings["services"])[dep])
	switch {
	case base == "":
		return "", fmt.Sprintf("the ui target's settings.services gives %s no address; add one, a URL or a path of the screens' own origin, such as %s: https://%s.example.org", dep, dep, d.Specification)
	case !strings.HasPrefix(base, "https://") && !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "/"):
		return "", fmt.Sprintf("the ui target's settings.services gives %s %q, which is neither a URL nor a path of the screens' own origin starting with /", dep, base)
	}
	return strings.TrimRight(base, "/"), ""
}

// refusalTest is the derived test of a page that needs a permission, under
// node:test: it opens to someone who holds it, refuses someone signed in
// without it, and sends someone signed out to sign in, read from the
// page's data and the screens' decision alone. A public page and a task
// page have none.
func (g *gen) refusalTest(p *genscreens.Page) string {
	if p.Kind == "task" || p.Permission == "public" {
		return ""
	}
	var b strings.Builder
	b.WriteString("import assert from \"node:assert/strict\";\nimport { test } from \"node:test\";\nimport { decide } from \"../screens/access.js\";\n")
	fmt.Fprintf(&b, "import { page } from \"../pages/%s.js\";\n\n", p.Name)
	if p.Permission == "signedIn" {
		fmt.Fprintf(&b, "test(%s, () => {\n  assert.equal(decide(page.permission, { signedIn: true, permissions: [] }), \"allowed\");\n});\n\n", quote(p.Name+" opens to everyone signed in, whatever they hold"))
		fmt.Fprintf(&b, "test(%s, () => {\n  assert.equal(decide(page.permission, { signedIn: false, permissions: [] }), \"sign-in\");\n});\n", quote(p.Name+" sends someone signed out to sign in"))
		return b.String()
	}
	fmt.Fprintf(&b, "test(%s, () => {\n  assert.equal(decide(page.permission, { signedIn: true, permissions: [] }), \"refused\");\n});\n\n", quote(p.Name+" is refused to someone signed in without "+p.Permission))
	fmt.Fprintf(&b, "test(%s, () => {\n  assert.equal(decide(page.permission, { signedIn: false, permissions: [] }), \"sign-in\");\n});\n\n", quote(p.Name+" sends someone signed out to sign in"))
	fmt.Fprintf(&b, "test(%s, () => {\n  assert.equal(decide(page.permission, { signedIn: true, permissions: [%s] }), \"allowed\");\n});\n", quote(p.Name+" opens to someone who holds "+p.Permission), quote(p.Permission))
	return b.String()
}

// menuTest is the derived test of the menu: an entry is not shown to
// someone without the permission of the page it opens.
const menuTest = `import assert from "node:assert/strict";
import { test } from "node:test";
import { menus } from "../config/menus.js";
import { visibleMenu } from "../screens/access.js";

test("the menu shows no entry to someone signed in without its page's permission", () => {
  for (const group of menus) {
    for (const item of group.items) {
      if (item.permission === "public" || item.permission === "signedIn") {
        continue;
      }
      const held = menus.flatMap((g) => g.items.map((i) => i.permission)).filter((p) => p !== item.permission);
      const shown = visibleMenu(menus, { signedIn: true, permissions: held }).flatMap((g) => g.items);
      assert.ok(!shown.some((entry) => entry.route === item.route && entry.permission === item.permission), item.route);
    }
  }
});

test("the menu shows every entry to someone who holds every page's permission", () => {
  const held = menus.flatMap((g) => g.items.map((i) => i.permission));
  const shown = visibleMenu(menus, { signedIn: true, permissions: held }).flatMap((g) => g.items);
  assert.equal(shown.length, menus.flatMap((g) => g.items).length);
});
`

// apiFile is api.js: one api.call over the transport, each call named by
// its action, an operation of config/actions.js, which says how it
// travels; in the dxlib dialect every call is a POST at /<operationId>
// with its parameters in one body, and in the standard dialect the
// operation's method at its path.
const apiFile = `import { actions } from "./config/actions.js";
import { application } from "./config/application.js";
import { createTransport, property, save } from "./transport.js";

const transport = createTransport({ service: application.service, dialect: application.dialect, handshakes: application.handshakes ?? {} });

/** The action of a name, or an Error naming the actions there are. */
function target(action) {
  const found = actions[action];
  if (found === undefined) {
    throw new Error(` + "`api.call(${JSON.stringify(action)}): no such action in config/actions.js; the actions are ${Object.keys(actions).join(\", \")}`" + `);
  }
  return found;
}

/** The JSON body of an answer, or undefined when it has none or cannot be read. */
async function body(response) {
  try {
    return await response.json();
  } catch {
    return undefined;
  }
}

/**
 * Every call of the screens: call(action, payload) sends the action's
 * operation as the service's dialect serves it, with payload's parameters
 * (the path's), query, body and headers, establishing the handshake its
 * envelope needs and renewing it when the service asks; upload streams a
 * file as the request's body; save keeps a downloaded file under its name;
 * body and property read an answer. pathOf is the action's path with its
 * {parameters}, for a page that fills one from a record; streamOf and
 * mediaTypeOf the file it carries and an upload's media type.
 */
export const api = {
  call: (action, payload = {}) => transport.call(target(action), payload),
  upload: (action, file, payload = {}) => transport.upload(target(action), file, payload),
  pathOf: (action) => target(action).path,
  streamOf: (action) => target(action).stream,
  mediaTypeOf: (action) => target(action).mediaType,
  body,
  property,
  save,
};
`
