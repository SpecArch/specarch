package genscreens

import (
	"strconv"

	"github.com/SpecArch/specarch/internal/genui"
)

// ContentPage reads a content page: its blocks under its title, each text
// a string key of the page, and the routes of the pages its links open
// (ADR-181). A page that needs a permission is drawn behind the route
// guard, and warned of: its texts travel to the browser with the page, so
// the guard keeps the page from showing, not its text from who reads the
// files.
func (g *Gen) ContentPage(pageName string, pg map[string]any) (*Page, bool) {
	at := "/pages/" + pageName
	blocks, problems := genui.ContentBlocks(pageName, pg, obj0(g.Spec["pages"]))
	for _, p := range problems {
		g.Problem("error", p.At, "%s", p.Message)
	}
	if len(problems) > 0 {
		return nil, false
	}
	permission := text(pg["permission"])
	if permission != "public" {
		g.Problem("warning", at+"/permission", "%s is a content page that needs %s, and its text travels to the browser with the page, so the route guard keeps the page from showing but not its text from who reads the files; keep what only holders of %s may read behind an operation", pageName, permission, permission)
	}
	routes := map[string]string{}
	pages := obj0(g.Spec["pages"])
	var values []Value
	for i, b := range blocks {
		key := pageName + ".blocks." + strconv.Itoa(i)
		var entry []Member
		switch b.Kind {
		case "heading":
			entry = []Member{{"kind", Str("heading")}, {"level", Raw(strconv.Itoa(b.Level))}, {"text", Str(g.Say(key+".text", b.Text))}}
		case "text":
			entry = []Member{{"kind", Str("text")}, {"text", Str(g.Say(key+".text", b.Text))}}
		case "image":
			entry = []Member{{"kind", Str("image")}, {"path", Str(b.Path)}, {"alternativeText", Str(g.Say(key+".alternativeText", b.Text))}}
		case "link":
			entry = []Member{{"kind", Str("link")}}
			if b.Page != "" {
				routes[b.Page] = text(obj0(pages[b.Page])["route"])
				entry = append(entry, Member{"page", Str(b.Page)})
			} else {
				entry = append(entry, Member{"href", Str(b.Href)})
			}
			entry = append(entry, Member{"label", Str(g.Say(key+".label", b.Text))})
		}
		values = append(values, Object(entry))
	}
	part := "content-page"
	schema := []Member{
		{g.Name(part, "title"), Str(g.Say(pageName+".title", text(pg["title"])))},
		{g.Name(part, "permission"), Str(permission)},
		{g.Name(part, "blocks"), Array(values)},
	}
	return &Page{Name: pageName, Kind: "content", Part: part, Route: text(pg["route"]), Permission: permission, Schema: schema, Routes: routes}, true
}
