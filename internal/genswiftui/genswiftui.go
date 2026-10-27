// Package genswiftui writes the screens of a specification for the iPhone
// in SwiftUI: the logic of the specarch-gen-ui-swiftui plug-in. Each page
// becomes a schema file that holds data only, one Swift literal typed by
// its component's schema type, and a thin view; the components that read
// the schemas are written once under Screens/, the routes once as one
// enum, the application once, and every text once in Strings.swift, into
// a Swift package the project writes the Package.swift of. The pages are
// read through internal/genscreens, as the TypeScript generator reads
// them; what draws each page kind and field type, and the labels of its
// schema, are the names of the ui-components idiom for swiftui.
package genswiftui

import (
	"embed"
	"fmt"
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
const name = "specarch-gen-ui-swiftui"

// The framework this generator writes, the stack the idiom renders.
const framework = "swiftui"

// The module the screens are written into when the settings name none.
const defaultModule = "Screens"

//go:embed screens
var screens embed.FS

type value = genscreens.Value

var (
	obj0       = genscreens.Obj0
	text       = genscreens.Text
	sortedKeys = genscreens.SortedKeys
)

type gen struct {
	*genscreens.Gen
	// module is the Swift module the screens are, settings.module.
	module string
	// routes are the pages the routes enum holds: every page written, and
	// every page an event or an action leads to, by name.
	routes map[string]*route
}

// route is one case of the routes enum: a page, with the parameters of
// its route, and whether this run writes it.
type route struct {
	page       string
	route      string
	parameters []string
	written    bool
}

// Generate writes the screens of a request.
func Generate(r *Request) genui.Response {
	g := &gen{routes: map[string]*route{}}
	g.Gen = genscreens.New(r, genscreens.Profile{Name: name, Platform: "iphone", Framework: framework, Language: "Swift", Where: "the app", ComparesDecimals: true, OtherMode: "is not a colour scheme the system asks for"})
	if g.Failed() {
		return g.Response(nil)
	}
	g.module = text(g.Settings["module"])
	if g.module == "" {
		g.module = defaultModule
	} else if !identifier(g.module) {
		g.Problem("error", "/", "the ui target's settings.module is %q, and a Swift module is one word of letters, digits and underscores starting with a letter, such as Screens", g.module)
		g.module = defaultModule
	}
	if _, named := g.Settings["server"]; named {
		g.Problem("error", "/", "the ui target's settings name a server, and %s writes an app, which calls the service directly at the URL the app sets; take settings.server out", name)
	}
	header := "// " + g.Header(r) + "\n"
	sources := "Sources/" + g.module + "/"
	tests := "Tests/" + g.module + "Tests/"
	var files []genui.File
	pageOf := map[string]string{} // the page each page's file is written for, by its path
	add := func(path, content string) {
		files = append(files, genui.File{Path: path, Content: header + content})
	}
	sess := g.ReadSession()
	pages := obj0(g.Spec["pages"])
	owned := g.Owned()
	drawn := map[string]bool{}
	hooks, layouts := g.FormSettings()
	var written []*genscreens.Page
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
		case "custom":
			page, ok = g.CustomPage(pageName, pg)
		case "content":
			g.Problem("error", "/pages/"+pageName, "%s is a content page, which this version of %s does not write, so nothing is written for it; write the page in the app by hand, or leave it to a web target", pageName, name)
			continue
		default:
			g.Problem("warning", "/pages/"+pageName, "%s is a %s, which this version of %s does not write; it is left out", pageName, text(pg["kind"]), name)
			continue
		}
		if !ok {
			continue
		}
		written = append(written, page)
		drawn[pageName] = true
		g.routeOf(pageName, page.Route, true)
		for target, targetRoute := range page.Routes {
			g.routeOf(target, targetRoute, false)
		}
	}
	if sess != nil && sess.SignInPage != "" {
		g.routeOf(sess.SignInPage, sess.SignIn, drawn[sess.SignInPage])
	}
	groups, permissions := g.Menu(drawn)
	for _, p := range written {
		pascal := pascalCase(p.Name)
		add(sources+"Pages/"+pascal+"Schema.swift", g.schemaFile(p))
		add(sources+"Pages/"+pascal+"View.swift", g.viewFile(p))
		pageOf[sources+"Pages/"+pascal+"Schema.swift"], pageOf[sources+"Pages/"+pascal+"View.swift"] = p.Name, p.Name
		if test := g.refusalTest(p); test != "" {
			add(tests+pascal+"Tests.swift", test)
			pageOf[tests+pascal+"Tests.swift"] = p.Name
		}
	}
	// The menu adds the routes of the pages only it reaches, so the
	// application is written before the routes.
	application := g.applicationFile(sess, groups, g.Homes(drawn, sess))
	add(sources+"Routes.swift", g.routesFile())
	add(sources+"Application.swift", application)
	if len(permissions) > 0 {
		add(tests+"MenuTests.swift", g.menuTest())
	}
	strs, _ := g.stringsFile(r.Existing)
	add(sources+"Strings.swift", strs)
	if theme, themed := g.themeFile(); themed {
		add(sources+"Theme.swift", theme)
	}
	entries, _ := screens.ReadDir("screens")
	for _, e := range entries {
		data, _ := screens.ReadFile("screens/" + e.Name())
		add(sources+"Screens/"+e.Name(), string(data))
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if !r.Check && g.HandWritten(files, r.Existing, "// Generated by "+name+" ") {
		return g.Response(nil)
	}
	g.Mark(r, files, header, "// ", pageOf, drawn, sources+"Application.swift")
	return g.Response(files)
}

// routeOf records a page among the routes, written when this run writes
// it.
func (g *gen) routeOf(pageName, pageRoute string, written bool) {
	r := g.routes[pageName]
	if r == nil {
		r = &route{page: pageName, route: pageRoute}
		for _, p := range genscreens.PathParam.FindAllString(pageRoute, -1) {
			r.parameters = append(r.parameters, p[1:len(p)-1])
		}
		g.routes[pageName] = r
	}
	r.written = r.written || written
}

// schemaFile is Pages/<Page>Schema.swift: the page as data, one literal
// of the part's schema type.
func (g *gen) schemaFile(p *genscreens.Page) string {
	schemaType := g.Name(p.Part, "schemaType")
	var b strings.Builder
	fmt.Fprintf(&b, "import Foundation\n\n/// The %s page as data, read by %s.\n", p.Name, g.component(p))
	b.WriteString(statement("let "+camelCase(p.Name)+"Schema = ", genscreens.Object(p.Schema), schemaType, ""))
	return b.String()
}

// viewFile is Pages/<Page>View.swift: the view that hands the schema to
// the part's component, behind the guard for a page that needs one, with
// the routes its events lead to and the route's parameters.
func (g *gen) viewFile(p *genscreens.Page) string {
	pascal := pascalCase(p.Name)
	component := g.component(p)
	schema := camelCase(p.Name) + "Schema"
	params := p.Parameters()
	var b strings.Builder
	b.WriteString("import SwiftUI\n\n")
	fmt.Fprintf(&b, "/// The %s page.\npublic struct %sView: View {\n", p.Name, pascal)
	for _, prm := range params {
		fmt.Fprintf(&b, "    let %s: String\n", camelCase(prm))
	}
	if len(params) > 0 {
		b.WriteString("\n")
	}
	var args []string
	for _, prm := range params {
		args = append(args, camelCase(prm)+": String")
	}
	if len(params) == 0 {
		b.WriteString("    public init() {}\n\n")
	} else {
		fmt.Fprintf(&b, "    public init(%s) {\n", strings.Join(args, ", "))
		for _, prm := range params {
			fmt.Fprintf(&b, "        self.%s = %s\n", camelCase(prm), camelCase(prm))
		}
		b.WriteString("    }\n\n")
	}
	b.WriteString("    public var body: some View {\n")
	call := g.componentCall(p, component, schema, params)
	if p.Kind == "task" {
		b.WriteString(indent(call, "        ") + "\n")
	} else {
		fmt.Fprintf(&b, "        Guard(permission: %s.%s, route: %s) {\n", schema, g.Name(p.Part, "permission"), g.routeCase(p.Name, selfParameters(params)))
		b.WriteString(indent(call, "            ") + "\n        }\n")
	}
	b.WriteString("    }\n}\n")
	return b.String()
}

// component is the view that draws a page: its part's component, or for
// a custom page <Page>Screen, a view the project writes in the screens
// module beside Pages/, so a
// missing one fails the build (ADR-183).
func (g *gen) component(p *genscreens.Page) string {
	if p.Kind == "custom" {
		return pascalCase(p.Name) + "Screen"
	}
	return g.Name(p.Part, "component")
}

// componentCall is the component drawn with the schema, the route's
// parameters, the hooks a form takes, and the routes its events lead to.
func (g *gen) componentCall(p *genscreens.Page, component, schema string, params []string) string {
	var args []string
	args = append(args, "schema: "+schema)
	if p.Kind != "task" {
		var ps []string
		for _, prm := range params {
			ps = append(ps, quote(prm)+": "+camelCase(prm))
		}
		if len(ps) == 0 {
			args = append(args, "parameters: [:]")
		} else {
			args = append(args, "parameters: ["+strings.Join(ps, ", ")+"]")
		}
	}
	if len(p.Hooks) > 0 {
		// One member per hooked field, so a hook the project has not
		// written is a compile error, not a field that starts empty.
		var hs []string
		for _, f := range p.Hooks {
			hs = append(hs, quote(f)+": "+pascalCase(p.Name)+"Hooks."+camelCase(f))
		}
		args = append(args, "hooks: ["+strings.Join(hs, ", ")+"]")
	}
	targets := sortedKeys(anyMap(p.Routes))
	if len(targets) == 0 {
		args = append(args, "routes: [:]")
		return component + "(" + strings.Join(args, ", ") + ")"
	}
	var b strings.Builder
	b.WriteString(component + "(" + strings.Join(args, ", ") + ", routes: [\n")
	for i, target := range targets {
		r := g.routes[target]
		if len(r.parameters) == 0 {
			fmt.Fprintf(&b, "    %s: { _ in %s }", quote(target), g.routeCase(target, nil))
		} else {
			var ps []string
			for _, prm := range r.parameters {
				ps = append(ps, camelCase(prm)+": parameters["+quote(prm)+"] ?? \"\"")
			}
			fmt.Fprintf(&b, "    %s: { parameters in .%s(%s) }", quote(target), camelCase(target), strings.Join(ps, ", "))
		}
		if i < len(targets)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("])")
	return b.String()
}

// selfParameters are a view's own parameters as the arguments of its route.
func selfParameters(params []string) []string {
	var out []string
	for _, prm := range params {
		out = append(out, camelCase(prm)+": "+camelCase(prm))
	}
	return out
}

// routeCase is a route's case with its arguments, or without when it
// takes none.
func (g *gen) routeCase(pageName string, args []string) string {
	if len(args) == 0 {
		return "." + camelCase(pageName)
	}
	return "." + camelCase(pageName) + "(" + strings.Join(args, ", ") + ")"
}

// routesFile is Routes.swift: one enum of every page written and every
// page an event leads to, with the parameters of its route, and the view
// each opens.
func (g *gen) routesFile() string {
	names := make([]string, 0, len(g.routes))
	for n := range g.routes {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("import SwiftUI\n\n/// The pages, one case each with the parameters of its route; the app's\n/// NavigationStack opens each through destination.\npublic enum Route: Hashable, Sendable {\n")
	for _, n := range names {
		r := g.routes[n]
		if len(r.parameters) == 0 {
			fmt.Fprintf(&b, "    case %s\n", camelCase(n))
			continue
		}
		var ps []string
		for _, prm := range r.parameters {
			ps = append(ps, camelCase(prm)+": String")
		}
		fmt.Fprintf(&b, "    case %s(%s)\n", camelCase(n), strings.Join(ps, ", "))
	}
	b.WriteString("}\n\nextension Route {\n    /// The view the route opens.\n    @MainActor @ViewBuilder public var destination: some View {\n        switch self {\n")
	for _, n := range names {
		r := g.routes[n]
		pattern := "." + camelCase(n)
		var args []string
		if len(r.parameters) > 0 {
			var binds []string
			for _, prm := range r.parameters {
				binds = append(binds, "let "+camelCase(prm))
				args = append(args, camelCase(prm)+": "+camelCase(prm))
			}
			pattern += "(" + strings.Join(binds, ", ") + ")"
		}
		if r.written {
			fmt.Fprintf(&b, "        case %s: %sView(%s)\n", pattern, pascalCase(n), strings.Join(args, ", "))
		} else {
			fmt.Fprintf(&b, "        case %s: MissingPage(name: %s)\n", pattern, quote(n))
		}
	}
	b.WriteString("        }\n    }\n}\n")
	return b.String()
}

// applicationFile is Application.swift: how the screens read the session,
// the page that signs in, and the menu, each entry with the permission of
// the page it opens.
func (g *gen) applicationFile(sess *genscreens.Session, groups []value, homes []genscreens.Home) string {
	var b strings.Builder
	b.WriteString("/// How the screens read the person's session, where they sign in, and\n/// the menu; where they call the service is Service.baseURL, which the app\n/// sets at launch.\npublic enum Application {\n")
	if sess == nil {
		b.WriteString("    public static let session: SessionSchema? = nil\n")
	} else {
		roles := ""
		if sess.Roles != "" {
			roles = ", roles: " + quote(sess.Roles)
		}
		fmt.Fprintf(&b, "    public static let session: SessionSchema? = .init(path: %s, signedIn: %s, permissions: %s%s)\n", quote(sess.Path), quote(sess.SignedIn), quote(sess.Permissions), roles)
	}
	if sess == nil || sess.SignInPage == "" {
		b.WriteString("    public static let signIn: Route? = nil\n    public static let signInTitle = \"\"\n")
	} else {
		fmt.Fprintf(&b, "    public static let signIn: Route? = %s\n    public static let signInTitle = %s\n", g.routeCase(sess.SignInPage, nil), quote(sess.SignInPage+".title"))
	}
	// A menu entry's route is a case of the enum, not a path.
	var menu []value
	for _, grp := range groups {
		o := grp.(genscreens.ObjectValue)
		var members []genscreens.Member
		for _, m := range o.Members {
			if m.Key != "items" {
				members = append(members, m)
				continue
			}
			var items []value
			for _, it := range m.Val.(genscreens.ArrayValue).Items {
				var entry []genscreens.Member
				for _, em := range it.(genscreens.ObjectValue).Members {
					if em.Key == "route" {
						entry = append(entry, genscreens.Member{Key: "route", Val: genscreens.Raw(g.routeCase(g.pageAt(genscreens.Text(em.Val.(genscreens.Literal).Text)), nil))})
						continue
					}
					entry = append(entry, em)
				}
				items = append(items, genscreens.Object(entry))
			}
			members = append(members, genscreens.Member{Key: "items", Val: genscreens.Array(items)})
		}
		menu = append(menu, genscreens.Object(members))
	}
	b.WriteString(indent(statement("public static let menu: [MenuGroup] = ", genscreens.Array(menu), "", ""), "    ") + "\n")
	// The page each role is shown first, by role (ADR-197).
	if len(homes) == 0 {
		b.WriteString("    public static let homes: [String: Route] = [:]\n")
	} else {
		b.WriteString("    public static let homes: [String: Route] = [\n")
		for _, h := range homes {
			fmt.Fprintf(&b, "        %s: %s,\n", quote(h.Role), g.routeCase(h.Page, nil))
		}
		b.WriteString("    ]\n")
	}
	b.WriteString("}\n")
	return b.String()
}

// pageAt is the name of the page at a route that takes no parameter.
func (g *gen) pageAt(pageRoute string) string {
	pages := obj0(g.Spec["pages"])
	for _, n := range sortedKeys(pages) {
		if text(obj0(pages[n])["route"]) == pageRoute {
			g.routeOf(n, pageRoute, false)
			return n
		}
	}
	return pageRoute
}

// refusalTest is the derived test of a page that needs a permission: it
// opens to someone who holds it, refuses someone signed in without it,
// and sends someone signed out to sign in. A public page and a task page
// have none.
func (g *gen) refusalTest(p *genscreens.Page) string {
	if p.Kind == "task" || p.Permission == "public" {
		return ""
	}
	schema := camelCase(p.Name) + "Schema." + g.Name(p.Part, "permission")
	var b strings.Builder
	fmt.Fprintf(&b, "import Testing\n@testable import %s\n\nstruct %sTests {\n", g.module, pascalCase(p.Name))
	if p.Permission == "signedIn" {
		fmt.Fprintf(&b, "    @Test(%s)\n    func allowed() {\n        #expect(Access.decide(permission: %s, session: Session(signedIn: true, permissions: [])) == .allowed)\n    }\n\n", quote(p.Name+" opens to everyone signed in, whatever they hold"), schema)
		fmt.Fprintf(&b, "    @Test(%s)\n    func signIn() {\n        #expect(Access.decide(permission: %s, session: Session(signedIn: false, permissions: [])) == .signIn)\n    }\n}\n", quote(p.Name+" sends someone signed out to sign in"), schema)
		return b.String()
	}
	fmt.Fprintf(&b, "    @Test(%s)\n    func refused() {\n        #expect(Access.decide(permission: %s, session: Session(signedIn: true, permissions: [])) == .refused)\n    }\n\n", quote(p.Name+" is refused to someone signed in without "+p.Permission), schema)
	fmt.Fprintf(&b, "    @Test(%s)\n    func signIn() {\n        #expect(Access.decide(permission: %s, session: Session(signedIn: false, permissions: [])) == .signIn)\n    }\n\n", quote(p.Name+" sends someone signed out to sign in"), schema)
	fmt.Fprintf(&b, "    @Test(%s)\n    func allowed() {\n        #expect(Access.decide(permission: %s, session: Session(signedIn: true, permissions: [%s])) == .allowed)\n    }\n}\n", quote(p.Name+" opens to someone who holds "+p.Permission), schema, quote(p.Permission))
	return b.String()
}

// menuTest is the derived test of the menu: an entry is not shown to
// someone without the permission of the page it opens.
func (g *gen) menuTest() string {
	return fmt.Sprintf(`import Testing
@testable import %s

struct MenuTests {
    @Test("the menu shows no entry to someone signed in without its page's permission")
    func hidden() {
        for group in Application.menu {
            for item in group.items where item.permission != "public" && item.permission != "signedIn" {
                let held = Application.menu.flatMap { $0.items.map(\.permission) }.filter { $0 != item.permission }
                let shown = Access.visibleMenu(Application.menu, session: Session(signedIn: true, permissions: held)).flatMap(\.items)
                #expect(!shown.contains { $0.route == item.route && $0.permission == item.permission }, "\(item.route)")
            }
        }
    }

    @Test("the menu shows every entry to someone who holds every page's permission")
    func shown() {
        let held = Application.menu.flatMap { $0.items.map(\.permission) }
        let shown = Access.visibleMenu(Application.menu, session: Session(signedIn: true, permissions: held)).flatMap(\.items)
        #expect(shown.count == Application.menu.flatMap(\.items).count)
    }
}
`, g.module)
}

// indent indents every line of a text.
func indent(s, by string) string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = by + l
		}
	}
	return strings.Join(lines, "\n")
}

func anyMap(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// pascalCase is a page or parameter name as a Swift type name: sign-in
// is SignIn, members_list is MembersList.
func pascalCase(s string) string {
	var b strings.Builder
	up := true
	for _, r := range s {
		switch {
		case r == '-' || r == '_' || r == ' ' || r == '.':
			up = true
		case up:
			b.WriteString(strings.ToUpper(string(r)))
			up = false
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// camelCase is a page or parameter name as a Swift identifier: sign-in
// is signIn; a word that is a keyword is written in backticks.
func camelCase(s string) string {
	p := pascalCase(s)
	if p == "" {
		return p
	}
	out := strings.ToLower(p[:1]) + p[1:]
	if swiftKeywords[out] {
		return "`" + out + "`"
	}
	return out
}

// identifier says whether a word is a Swift identifier of letters, digits
// and underscores starting with a letter.
func identifier(s string) bool {
	for i, r := range s {
		switch {
		case r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return s != "" && s[0] != '_'
}

// wire is a field's name on the wire.
func (g *gen) wire(f string) string {
	return wirename.Of(g.WireNames, f)
}
