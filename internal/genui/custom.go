package genui

import (
	"fmt"
	"html"
	"strings"
)

// customPage is a custom page: a screen the project builds by hand
// (ADR-183). Its HTML holds the menu and the title, and a notice of the
// module that did not load; its module imports custom/<page>.js, which
// the project writes, and hands it the page's main element, the route's
// parameters and the pages its actions open.
type customPage struct {
	name, title, language string
	actions               []link
}

// custom reads a custom page. specarch-gen-ui writes no route guard for
// any page, so a custom page that needs a permission is warned of: the
// project's module checks the session before it shows anything.
func (g *gen) custom(name string, pg map[string]any) *customPage {
	at := "/pages/" + name
	if p := text(pg["permission"]); p != "public" {
		g.problem("warning", at+"/permission", "%s is a custom page that needs %s, and specarch-gen-ui writes no route guard for any page, so custom/%s.js, which the project writes, checks the session before it shows anything; the service still refuses each call of someone without %s", name, p, name, p)
	}
	c := &customPage{name: name, title: text(pg["title"]), language: text(g.settings["language"])}
	for i, a := range list(pg["actions"]) {
		action := obj0(a)
		if text(action["kind"]) != "navigate" {
			continue // the validator refuses an operation on a custom page
		}
		c.actions = append(c.actions, link{label: text(action["label"]), page: text(action["target"]), at: fmt.Sprintf("%s/actions/%d/target", at, i)})
	}
	return c
}

// module is the file the project writes for the page, as pages.js names
// it and the page's module imports it.
func (c *customPage) module() string { return "custom/" + c.name + ".js" }

func (c *customPage) events() []event { return nil }

// html writes the page: the menu, the title and the notice that stays
// until the project's module shows the screen, so a module that is
// missing or fails leaves a page that says so, never a blank one.
func (c *customPage) html(g *gen) string {
	out := g.headOf(c.name, c.title, c.language, true)
	out = append(out, `<p id="screen" role="status">`+html.EscapeString("This screen did not load: "+c.module()+", which the project writes, is missing or failed; the browser's console says why.")+"</p>")
	out = append(out, "</main>", "</body>", "</html>")
	return strings.Join(out, "\n") + "\n"
}

// js imports the project's module and shows its screen in place of the
// notice, with the route's parameters and the pages its actions open.
func (c *customPage) js() string {
	var b strings.Builder
	fmt.Fprintf(&b, "// The %s page is a screen the project builds by hand: %s exports\n", c.name, c.module())
	b.WriteString("// show(main, parameters, actions), which draws it in the page's main\n")
	b.WriteString("// element. A missing module fails this import, and the notice stays.\n")
	fmt.Fprintf(&b, "import { show } from %s;\nimport { parameters } from \"./pages.js\";\n\n", jsString("./"+c.module()))
	b.WriteString("// The pages the screen's actions open, each with its label.\nconst ACTIONS = [\n")
	for _, a := range c.actions {
		fmt.Fprintf(&b, "  { label: %s, page: %s },\n", jsString(a.label), jsString(a.page))
	}
	b.WriteString("];\n\n")
	b.WriteString("document.getElementById(\"screen\").remove();\nshow(document.querySelector(\"main\"), parameters(), ACTIONS);\n")
	return b.String()
}
