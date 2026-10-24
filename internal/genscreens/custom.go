package genscreens

import "strconv"

// CustomPage reads a custom page: a screen the project builds by hand
// (ADR-183). Its schema holds its title, its permission and the actions it
// leads by, each label a string key of the page, and its routes are those
// of the pages its actions open; the screen itself is the project's.
func (g *Gen) CustomPage(pageName string, pg map[string]any) (*Page, bool) {
	part := "custom-page"
	pages := obj0(g.Spec["pages"])
	routes := map[string]string{}
	var actions []Value
	for i, a := range list(pg["actions"]) {
		action := obj0(a)
		target := text(action["target"])
		if text(action["kind"]) != "navigate" || pages[target] == nil {
			continue // the validator refuses an operation and a page that is not there
		}
		routes[target] = text(obj0(pages[target])["route"])
		label := g.Say(pageName+".actions."+strconv.Itoa(i)+".label", text(action["label"]))
		actions = append(actions, Object([]Member{{"label", Str(label)}, {"page", Str(target)}}))
	}
	permission := text(pg["permission"])
	schema := []Member{
		{g.Name(part, "title"), Str(g.Say(pageName+".title", text(pg["title"])))},
		{g.Name(part, "permission"), Str(permission)},
		{g.Name(part, "actions"), Array(actions)},
	}
	return &Page{Name: pageName, Kind: "custom", Part: part, Route: text(pg["route"]), Permission: permission, Schema: schema, Routes: routes}, true
}
