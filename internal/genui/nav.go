package genui

import (
	_ "embed"
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/mark"
)

// Navigation between pages (ADR-152). The screens are files with no server
// to read a path, so a page is <name>.html and its route's parameters
// travel in the query; pages.js declares every page's file and parameters
// once, as events.js declares the events. A page opens another with a
// link, or with go once an operation has succeeded, and a message for the
// page it opens is carried in the browser's session storage for that one
// load.

//go:embed runtime/pages.js
var pagesRuntime string

// link is an action, a selected row or an event that opens another page,
// with the JavaScript that gives the parameters of the page it opens.
type link struct {
	label, page, with, when, at string
}

// pagesJS writes every page of the screens, its file and the parameters
// its address takes, then the functions that open them.
func (g *gen) pagesJS(names []string) string {
	var b strings.Builder
	b.WriteString(`// Where each page is and the parameters its address takes, every page of
// the screens declared once. The screens are files with no server to read
// a path, so a route's parameters travel in the query:
`)
	example := ""
	for _, n := range names {
		route := text(obj0(obj0(g.spec["pages"])[n])["route"])
		if ps := routeParameters(route); len(ps) > 0 && example == "" {
			example = fmt.Sprintf("// %s is %s.html?%s=...\n", route, n, ps[0])
		}
	}
	if example == "" {
		example = "// a page's address is its name and .html.\n"
	}
	b.WriteString(example + "\nexport const PAGES = {\n")
	for _, n := range names {
		var ps []string
		for _, p := range routeParameters(text(obj0(obj0(g.spec["pages"])[n])["route"])) {
			ps = append(ps, jsString(p))
		}
		module := ""
		if text(obj0(obj0(g.spec["pages"])[n])["kind"]) == "custom" {
			// The screen the project writes for a custom page (ADR-183).
			module = ", module: " + jsString("custom/"+n+".js")
		}
		fmt.Fprintf(&b, "  %s: { file: %s, parameters: [%s]%s },\n", jsString(n), jsString(n+".html"), strings.Join(ps, ", "), module)
	}
	b.WriteString("};\n")
	b.WriteString(pagesRuntime)
	return b.String()
}

// opened are the pages a link may open: those written, and those another
// stakeholder owns, whose file is theirs.
func (g *gen) opens(target string) bool { return g.opened[target] }

// withJS writes the parameters a link gives the page it opens: each of
// its route's parameters from the field its with names, else from the
// record's field of that name, else, on a view, from the view's own
// route. ok is false, with an error at the link, when one is not given.
func (g *gen) withJS(at, label, target string, with map[string]any, props map[string]any, own []string, record string) (string, bool) {
	route := text(obj0(obj0(g.spec["pages"])[target])["route"])
	owned := map[string]bool{}
	for _, p := range own {
		owned[p] = true
	}
	var parts []string
	usesOwn := false
	for _, p := range routeParameters(route) {
		switch {
		case text(with[p]) != "":
			parts = append(parts, p+": "+record+"."+g.wire(text(with[p])))
		case props[p] != nil:
			parts = append(parts, p+": "+record+"."+g.wire(p))
		case owned[p]:
			parts = append(parts, p+": parameters."+p)
			usesOwn = true
		default:
			g.problem("error", at, "%s opens %s at %s, and nothing gives {%s}: name the field that does under with", label, target, route, p)
			return "", false
		}
	}
	if len(parts) == 0 {
		return "() => ({})", true
	}
	args := "(" + record + ")"
	if own != nil || usesOwn {
		args = "(" + record + ", parameters)"
	}
	return args + " => ({ " + strings.Join(parts, ", ") + " })", true
}

// menuHTML writes the menu every page carries, from the specification's
// menus; the entry of the page itself is marked as the current one.
func (g *gen) menuHTML(current string) []string {
	menus := obj0(g.spec["menus"])
	if len(menus) == 0 {
		return nil
	}
	out := []string{`<nav id="menu" aria-label="Menu">`, "<ul>"}
	for _, m := range sortedKeys(menus) {
		menu := obj0(menus[m])
		items := obj0(menu["items"])
		var entries []string
		for _, i := range sortedKeys(items) {
			item := obj0(items[i])
			page := text(item["page"])
			if !g.menuEntry[m+"/"+i] {
				continue
			}
			attr := ""
			if page == current {
				attr = ` aria-current="page"`
			}
			li := "<li>"
			if g.session != nil {
				li = "<li data-permission=" + quoted(text(obj0(obj0(g.spec["pages"])[page])["permission"])) + ">"
			}
			entries = append(entries, li+"<a href="+quoted(page+".html")+attr+">"+html.EscapeString(text(item["title"]))+"</a></li>")
		}
		if len(entries) == 0 {
			continue
		}
		out = append(out, "<li>"+html.EscapeString(text(menu["title"])), "<ul>")
		out = append(out, entries...)
		out = append(out, "</ul>", "</li>")
	}
	return append(out, "</ul>", "</nav>")
}

// checkMenus decides which menu entries are written: one whose page is
// written or another stakeholder's, and whose route takes no parameter a
// menu could give. Any other is reported at the entry and left out.
func (g *gen) checkMenus() {
	g.menuEntry = map[string]bool{}
	menus := obj0(g.spec["menus"])
	for _, m := range sortedKeys(menus) {
		items := obj0(obj0(menus[m])["items"])
		for _, i := range sortedKeys(items) {
			item := obj0(items[i])
			page := text(item["page"])
			at := "/menus/" + mark.Escape(m) + "/items/" + mark.Escape(i)
			route := text(obj0(obj0(g.spec["pages"])[page])["route"])
			switch {
			case !g.opens(page):
				g.problem("warning", at, "the menu entry %s opens %s, which specarch-gen-ui does not write, so the entry is left out", text(item["title"]), page)
			case len(routeParameters(route)) > 0:
				g.problem("warning", at, "the menu entry %s opens %s at %s, and a menu gives no parameter, so the entry is left out", text(item["title"]), page, route)
			default:
				g.menuEntry[m+"/"+i] = true
			}
		}
	}
}

// keep leaves out the links to a page that is not written, each reported
// at its link.
func (g *gen) keep(links []link) []link {
	var out []link
	for _, l := range links {
		if !g.opens(l.page) {
			g.problem("warning", l.at, "%s opens %s, which specarch-gen-ui does not write, so it is left out", l.label, l.page)
			continue
		}
		out = append(out, l)
	}
	return out
}

func sortedSet(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
