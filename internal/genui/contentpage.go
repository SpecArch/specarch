package genui

import (
	"fmt"
	"html"
	"strings"
)

// contentPage is a content page: its blocks under its title, written as
// HTML alone, with no module, since it loads and sends nothing.
type contentPage struct {
	name, title, language string
	blocks                []Block
}

// content reads a content page, or reports why it cannot be written. A
// page that needs a permission is refused: the screens are files a server
// hands to anyone who asks, with no guard of their own, and the text of a
// content page is in its file, so nothing would keep it from who lacks
// the permission (ADR-181).
func (g *gen) content(name string, pg map[string]any) *contentPage {
	at := "/pages/" + name
	if p := text(pg["permission"]); p != "public" {
		g.problem("error", at+"/permission", "%s is a content page that needs %s, and specarch-gen-ui writes files a server hands to anyone, with the page's text inside and no route guard, so nothing would keep it from who lacks %s; make the page public, and keep what only holders of %s may read behind an operation", name, p, p, p)
		return nil
	}
	blocks, problems := ContentBlocks(name, pg, obj0(g.spec["pages"]))
	for _, p := range problems {
		g.problem("error", p.At, "%s", p.Message)
	}
	if len(problems) > 0 {
		return nil
	}
	return &contentPage{name: name, title: text(pg["title"]), language: text(g.settings["language"]), blocks: blocks}
}

func (c *contentPage) events() []event { return nil }

func (c *contentPage) js() string { return "" }

// links are the pages the content page's links open, for the links kept
// to pages that are written.
func (c *contentPage) links() []link {
	var out []link
	for _, b := range c.blocks {
		if b.Kind == "link" && b.Page != "" {
			out = append(out, link{label: b.Text, page: b.Page, at: b.At + "/link/page"})
		}
	}
	return out
}

// html writes the content page: its blocks in order under its title, a
// heading as its level, a paragraph, an image with its text and a link.
func (c *contentPage) html(g *gen) string {
	out := g.headOf(c.name, c.title, c.language, false)
	for _, b := range c.blocks {
		switch b.Kind {
		case "heading":
			out = append(out, fmt.Sprintf("<h%d>%s</h%d>", b.Level, html.EscapeString(b.Text), b.Level))
		case "text":
			out = append(out, "<p>"+html.EscapeString(b.Text)+"</p>")
		case "image":
			out = append(out, "<p><img src="+quoted(b.Path)+" alt="+quoted(b.Text)+"></p>")
		case "link":
			href := b.Href
			if b.Page != "" {
				href = b.Page + ".html"
			}
			out = append(out, "<p><a href="+quoted(href)+">"+html.EscapeString(b.Text)+"</a></p>")
		}
	}
	out = append(out, "</main>", "</body>", "</html>")
	return strings.Join(out, "\n") + "\n"
}
