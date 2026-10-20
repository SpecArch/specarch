// Package genuits writes the screens of a specification for the web in
// TypeScript, on Next.js's app router and IBM's Carbon design system: the
// logic of the specarch-gen-ui-typescript plug-in. Each page becomes a
// schema file that holds data only and a thin page.tsx; the components
// that read the schemas are written once under screens/, and every text
// once in strings.ts. What draws each page kind and field type, and the
// keys of its schema, are the names of the ui-components idiom. The pages
// are read through internal/genscreens, which the SwiftUI generator reads
// them through too; this package writes the TypeScript.
package genuits

import (
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/genscreens"
	"github.com/SpecArch/specarch/internal/genui"
	"github.com/SpecArch/specarch/internal/ownership"
	"github.com/SpecArch/specarch/internal/wirename"
)

// Request is what specarch writes on a plug-in's standard input.
type Request = genscreens.Request

// Decode reads a request, keeping numbers as their text.
func Decode(data []byte) (*Request, error) { return genscreens.Decode(data) }

// The plug-in's name, as its header and its messages give it.
const name = "specarch-gen-ui-typescript"

// The framework this generator writes, the stack the idiom renders.
const framework = "nextjs-carbon"

//go:embed screens
var screens embed.FS

// The screens' own texts, the generator's words where a page has none.
var screenStrings = genscreens.ScreenStrings

type gen struct {
	*genscreens.Gen
	server *server

	schemaFile string // the name of each page's schema file: bindings.ui.schemaFile, or page.schema.ts
}

type (
	value  = genscreens.Value
	member = genscreens.Member
)

var (
	str    = genscreens.Str
	raw    = genscreens.Raw
	object = genscreens.Object
	array  = genscreens.Array
)

var (
	obj0       = genscreens.Obj0
	text       = genscreens.Text
	sortedKeys = genscreens.SortedKeys
	pathParam  = genscreens.PathParam
)

func anyMap(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Generate writes the screens of a request.
func Generate(r *Request) genui.Response {
	g := &gen{}
	g.Gen = genscreens.New(r, genscreens.Profile{Name: name, Platform: "web", Framework: framework, Language: "TypeScript", Where: "the browser", OtherMode: "has no media query of its own on the web", PagedByServer: g.pagedByServer})
	g.schemaFile = text(obj0(obj0(obj0(g.Impl.Content["bindings"])["ui"]))["schemaFile"])
	if g.schemaFile == "" {
		g.schemaFile = "page.schema.ts"
	}
	if g.Failed() {
		return g.Response(nil)
	}
	g.server = g.readServer()
	header := "// " + g.Header(r) + "\n"
	var files []genui.File
	pageOf := map[string]string{} // the page each page's file is written for, by its path
	sess := g.ReadSession()
	pages := obj0(g.Spec["pages"])
	owned := g.Owned()
	drawn := map[string]bool{}
	hooks, layouts := g.FormSettings()
	add := func(path, content string) {
		files = append(files, genui.File{Path: path, Content: header + content})
	}
	for _, pageName := range sortedKeys(pages) {
		pg := obj0(pages[pageName])
		if owned.Covers(ownership.Entity("pages", pageName)) {
			continue // another stakeholder's page; a link to it stays
		}
		folder := "app" + routeFolder(text(pg["route"]))
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
		default:
			g.Problem("warning", "/pages/"+pageName, "%s is a %s, which this version of %s does not write; it is left out", pageName, text(pg["kind"]), name)
			continue
		}
		if !ok {
			continue
		}
		var schema, tsx, client string
		if page.Kind == "task" {
			schema, tsx = g.taskFiles(page)
		} else {
			schema, tsx, client = g.pageFiles(page)
		}
		files = append(files, genui.File{Path: folder + "/" + g.schemaFile, Content: header + schema, Comparison: "object-literal"})
		add(folder+"/page.tsx", tsx)
		pageOf[folder+"/"+g.schemaFile], pageOf[folder+"/page.tsx"] = pageName, pageName
		if client != "" {
			add(folder+"/page.client.tsx", client)
			pageOf[folder+"/page.client.tsx"] = pageName
		}
		drawn[pageName] = true
		if test := g.refusalTest(page); test != "" {
			add("tests/"+pageName+".test.ts", test)
			pageOf["tests/"+pageName+".test.ts"] = pageName
		}
	}
	if g.server != nil {
		listed := map[string]bool{}
		for _, pageName := range sortedKeys(pages) {
			if pg := obj0(pages[pageName]); text(pg["kind"]) == "list" && drawn[pageName] {
				listed[text(pg["source"])] = true
			}
		}
		g.checkPaged(g.server, listed)
		files = append(files, g.serverFiles(g.server, header)...)
	}
	app, menuEntries := g.application(sess, drawn)
	add("application.ts", app)
	if len(menuEntries) > 0 {
		add("tests/menu.test.ts", menuTest())
	}
	strs, _ := g.stringsFile(r.Existing)
	theme, themed := g.themeFile()
	entries, _ := screens.ReadDir("screens")
	for _, e := range entries {
		data, _ := screens.ReadFile("screens/" + e.Name())
		add("screens/"+e.Name(), string(data))
	}
	add("strings.ts", strs)
	add("language.ts", g.languageFile())
	if themed {
		add("theme.scss", theme)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if !r.Check && g.HandWritten(files, r.Existing, "// Generated by "+name+" ") {
		return g.Response(nil)
	}
	g.Mark(r, files, header, "// ", pageOf, drawn, "application.ts")
	return g.Response(files)
}

// taskFiles writes the schema and the page.tsx of a task page.
func (g *gen) taskFiles(p *genscreens.Page) (string, string) {
	components := g.Name("application", "components")
	schemaType := g.Name(p.Part, "schemaType")
	component := g.Name(p.Part, "component")
	var schema strings.Builder
	fmt.Fprintf(&schema, "import type { %s } from %q;\n\n", schemaType, components)
	schema.WriteString(statement("export const schema = ", object(p.Schema), " satisfies "+schemaType+";"))
	var page strings.Builder
	fmt.Fprintf(&page, "import type { Metadata } from \"next\";\nimport { %s } from %q;\nimport { chosenLanguage } from \"@/language\";\nimport { stringOf, texts } from \"@/strings\";\nimport { schema } from %q;\n\n", component, components, g.schemaModule())
	page.WriteString(metadataOf(p.Name))
	g.writeRoutes(&page, p.Routes)
	page.WriteString("export default async function Page({ searchParams }: { readonly searchParams: Promise<Record<string, string | string[] | undefined>> }) {\n")
	page.WriteString("  const { returnTo } = await searchParams;\n  const t = texts(schema, await chosenLanguage());\n")
	fmt.Fprintf(&page, "  return <%s schema={schema} texts={t} routes={routes} returnTo={typeof returnTo === \"string\" ? returnTo : undefined} />;\n}\n", component)
	return schema.String(), page.String()
}

// pageFiles writes a guarded page's schema and its page.tsx, the route's
// parameters handed on. A form whose fields take hooks also gets
// page.client.tsx, which imports them from page.hooks.ts, a file the
// generator never writes, so a missing hook fails tsc; the hooks are
// functions, which run in the browser and cannot be handed from page.tsx,
// a server component.
func (g *gen) pageFiles(p *genscreens.Page) (string, string, string) {
	importFrom := g.ImportOf(p.Part)
	schemaType := g.Name(p.Part, "schemaType")
	component := g.Name(p.Part, "component")
	var schema strings.Builder
	fmt.Fprintf(&schema, "import type { %s } from %q;\n\n", schemaType, importFrom)
	schema.WriteString(statement("export const schema = ", object(p.Schema), " satisfies "+schemaType+";"))
	components := g.Name("application", "components")
	drawn := component
	var client strings.Builder
	if len(p.Hooks) > 0 {
		drawn = "HookedForm"
		var hs []string
		for _, h := range p.Hooks {
			if w := wirename.Of(g.WireNames, h); w != h {
				hs = append(hs, key(w)+": "+h)
			} else {
				hs = append(hs, h)
			}
		}
		client.WriteString("\"use client\";\n\n")
		fmt.Fprintf(&client, "import type { ComponentProps } from \"react\";\nimport { %s } from %q;\nimport { %s } from \"./page.hooks\";\n\n", component, importFrom, strings.Join(p.Hooks, ", "))
		client.WriteString("const hooks = { " + strings.Join(hs, ", ") + " };\n\n")
		client.WriteString("/** The form with the hooks of its fields, which run in the browser. */\n")
		fmt.Fprintf(&client, "export function HookedForm(props: Omit<ComponentProps<typeof %s>, \"hooks\">) {\n  return <%s {...props} hooks={hooks} />;\n}\n", component, component)
	}
	var page strings.Builder
	page.WriteString("import type { Metadata } from \"next\";\n")
	switch {
	case len(p.Hooks) > 0:
		fmt.Fprintf(&page, "import { Guard } from %q;\n", components)
	case importFrom == components:
		both := []string{"Guard", component}
		sort.Strings(both)
		fmt.Fprintf(&page, "import { %s } from %q;\n", strings.Join(both, ", "), components)
	default:
		fmt.Fprintf(&page, "import { %s } from %q;\nimport { Guard } from %q;\n", component, importFrom, components)
	}
	page.WriteString("import { chosenLanguage } from \"@/language\";\nimport { stringOf, texts } from \"@/strings\";\n")
	if len(p.Hooks) > 0 {
		page.WriteString("import { HookedForm } from \"./page.client\";\n")
	}
	fmt.Fprintf(&page, "import { schema } from %q;\n\n", g.schemaModule())
	page.WriteString(metadataOf(p.Name))
	g.writeRoutes(&page, p.Routes)
	props := "schema={schema} texts={t} routes={routes}"
	if pathParam.MatchString(p.Route) {
		page.WriteString("export default async function Page({ params }: { readonly params: Promise<Record<string, string>> }) {\n")
		page.WriteString("  const parameters = await params;\n")
		props += " parameters={parameters}"
	} else {
		page.WriteString("export default async function Page() {\n")
	}
	page.WriteString("  const t = texts(schema, await chosenLanguage());\n  return (\n")
	fmt.Fprintf(&page, "    <Guard permission={schema.%s} texts={t}>\n", g.Name(p.Part, "permission"))
	fmt.Fprintf(&page, "      <%s %s />\n", drawn, props)
	page.WriteString("    </Guard>\n  );\n}\n")
	return schema.String(), page.String(), client.String()
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
// and sends someone signed out to sign in. A public page and a task page
// have none.
func (g *gen) refusalTest(p *genscreens.Page) string {
	if p.Kind == "task" || p.Permission == "public" {
		return ""
	}
	permissionKey := g.Name(p.Part, "permission")
	var b strings.Builder
	b.WriteString("import assert from \"node:assert/strict\";\nimport { test } from \"node:test\";\nimport { decide } from \"../screens/access.ts\";\n")
	fmt.Fprintf(&b, "import { schema } from \"../app%s/%s\";\n\n", routeFolder(p.Route), g.schemaFile)
	fmt.Fprintf(&b, "test(%q, () => {\n  assert.equal(decide(schema.%s, { signedIn: true, permissions: [] }), \"refused\");\n});\n\n", p.Name+" is refused to someone signed in without "+p.Permission, permissionKey)
	fmt.Fprintf(&b, "test(%q, () => {\n  assert.equal(decide(schema.%s, { signedIn: false, permissions: [] }), \"sign-in\");\n});\n\n", p.Name+" sends someone signed out to sign in", permissionKey)
	fmt.Fprintf(&b, "test(%q, () => {\n  assert.equal(decide(schema.%s, { signedIn: true, permissions: [%s] }), \"allowed\");\n});\n", p.Name+" opens to someone who holds "+p.Permission, permissionKey, quote(p.Permission))
	return b.String()
}

// application is application.ts: how the screens read the session, the
// page that signs in, and the menu, each entry with the permission of the
// page it opens; an entry whose page this run does not write is left out,
// unless another stakeholder writes it.
func (g *gen) application(sess *genscreens.Session, drawn map[string]bool) (string, []string) {
	var b strings.Builder
	fmt.Fprintf(&b, "import type { Application } from %q;\n\n", g.Name("application", "components"))
	b.WriteString("/**\n * Where the screens call the service, how they read the person's session,\n * where they sign in, and the menu.\n */\n")
	sessionValue := raw("null")
	signIn := raw("null")
	if sess != nil {
		sessionValue = object([]member{{Key: "path", Val: str(sess.Path)}, {Key: "signedIn", Val: str(sess.SignedIn)}, {Key: "permissions", Val: str(sess.Permissions)}})
		if sess.SignIn != "" {
			signIn = str(sess.SignIn)
		}
	}
	groups, permissions := g.Menu(drawn)
	b.WriteString(statement("export const application: Application = ", object([]member{{Key: "service", Val: g.serviceValue()}, {Key: "session", Val: sessionValue}, {Key: "signIn", Val: signIn}, {Key: "menu", Val: array(groups)}}), ";"))
	return b.String(), permissions
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

// metadataOf is a page's metadata: its title in the chosen language.
func metadataOf(pageName string) string {
	return fmt.Sprintf("export async function generateMetadata(): Promise<Metadata> {\n  return { title: stringOf(%q, await chosenLanguage()) };\n}\n\n", pageName+".title")
}

// routeFolder is the app router's folder of a route: {param} becomes
// [param].
func routeFolder(route string) string {
	return pathParam.ReplaceAllStringFunc(route, func(p string) string { return "[" + p[1:len(p)-1] + "]" })
}

// schemaModule is the module a page's file imports its schema from: the
// schema file beside it, named without its extension.
func (g *gen) schemaModule() string {
	return "./" + strings.TrimSuffix(g.schemaFile, filepath.Ext(g.schemaFile))
}
