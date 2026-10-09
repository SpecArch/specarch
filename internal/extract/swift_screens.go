package extract

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// swEdge is one way a screen opens another.
type swEdge struct {
	from, to string // the view types
	fact     *swFact
	how      string // the call that opens it, as the says of a citation names it
	label    string // the action's label, "" when the source gives none as a literal
}

// The SwiftUI controls whose binding names a field of a screen, and the
// argument that carries it.
var swiftControls = map[string][]string{
	"TextField": {"text"}, "SecureField": {"text"}, "TextEditor": {"text"},
	"Toggle": {"isOn"}, "Picker": {"selection"}, "DatePicker": {"selection"},
	"Stepper": {"value"}, "Slider": {"value"},
}

// isView says whether a type of the files read is a SwiftUI view.
func (sw *swiftReader) isView(name string) bool {
	t := sw.types[name]
	return t != nil && t.decl != nil && t.conforms("View")
}

// destination is the view a closure or a value makes, when it makes one
// view of the files read and nothing else.
func (sw *swiftReader) destination(v *swValue) (string, string) {
	if v == nil {
		return "", "nothing"
	}
	if v.Closure != nil {
		if len(v.Closure.Statements) != 1 {
			return "", fmt.Sprintf("a closure of %s", plural(len(v.Closure.Statements), "statement"))
		}
		return sw.destination(&v.Closure.Statements[0])
	}
	if v.Branches != nil {
		return "", "a view chosen by a branch"
	}
	name, _ := v.construct()
	switch {
	case name == "":
		return "", v.describe()
	case !sw.isView(name):
		return "", name + ", which is not a view declared in the files read"
	}
	return name, ""
}

// label is the literal text a label closure shows: Text("...") or
// Label("...", ...).
func label(c *swClosure) string {
	if c == nil || len(c.Statements) != 1 {
		return ""
	}
	name, call := c.Statements[0].construct()
	if (name == "Text" || name == "Label") && call != nil && len(call.Arguments) > 0 && call.Arguments[0].Label == "" {
		if s, ok := call.Arguments[0].Value.stringLiteral(); ok {
			return s
		}
	}
	return ""
}

// readScreens writes each SwiftUI view a navigation call opens as a page,
// each way one opens another as a navigate action of the page, and each
// TabView's tabs as menu entries.
func (sw *swiftReader) readScreens() {
	var roots []*swFact
	var rootViews []string
	var edges []swEdge
	type tab struct {
		view, title string
		fact        *swFact
	}
	var tabs []tab
	typed := 0
	screens := map[string]*swFact{} // view -> the first fact that makes it a screen
	reach := func(view string, f *swFact) {
		if screens[view] == nil {
			screens[view] = f
		}
	}
	runtime := func(f *swFact, what string) {
		sw.question("should", fmt.Sprintf("%s at %s opens %s with %s. Which screen does it open?", f.Within, sw.clause(f), what, f.Name),
			[]string{"pages"}, "A destination chosen at run time, or a view outside the files read, is not known by syntax.", sw.at(sw.clause(f), "Opens "+what+" with "+f.Name+"."))
	}
	for _, f := range sw.calls {
		if !sw.imports(f, "SwiftUI") {
			continue
		}
		switch f.Name {
		case "WindowGroup":
			if f.Base != nil || len(f.Closures) == 0 {
				continue
			}
			view, what := sw.destination(&f.Closures[0].Value)
			if view == "" {
				runtime(f, what)
				continue
			}
			roots = append(roots, f)
			rootViews = append(rootViews, view)
			reach(view, f)
		case "TabView":
			if f.Base != nil || len(f.Closures) == 0 || f.Closures[0].Value.Closure == nil {
				continue
			}
			for _, st := range f.Closures[0].Value.Closure.Statements {
				st := st
				t := tab{fact: f}
				name, call := st.construct()
				if name == "Tab" && call != nil {
					if len(call.Arguments) > 0 && call.Arguments[0].Label == "" {
						t.title, _ = call.Arguments[0].Value.stringLiteral()
					}
					if len(call.Closures) > 0 {
						name, _ = sw.destination(&call.Closures[0].Value)
					} else {
						name = ""
					}
				} else {
					for _, m := range st.modifiers() {
						if m.Name == "tabItem" && len(m.Closures) > 0 {
							t.title = label(m.Closures[0].Value.Closure)
						}
					}
				}
				if name == "" || !sw.isView(name) {
					runtime(f, "a tab of "+st.describe())
					continue
				}
				t.view = name
				tabs = append(tabs, t)
				reach(name, f)
			}
		case "NavigationLink":
			if f.Within == "" {
				continue
			}
			e := swEdge{from: f.Within, fact: f, how: "NavigationLink"}
			var dest *swValue
			switch {
			case f.argument("destination") != nil:
				dest = f.argument("destination")
				if len(f.Arguments) > 0 && f.Arguments[0].Label == "" {
					e.label, _ = f.Arguments[0].Value.stringLiteral()
				} else if len(f.Closures) > 0 && f.Closures[0].Label == "" {
					e.label = label(f.Closures[0].Value.Closure)
				}
			case f.argument("value") != nil:
				typed++
				continue
			default:
				if len(f.Arguments) > 0 && f.Arguments[0].Label == "" {
					e.label, _ = f.Arguments[0].Value.stringLiteral()
				} else {
					e.label = label(f.closure("label"))
				}
				if len(f.Closures) > 0 && f.Closures[0].Label == "" {
					dest = &f.Closures[0].Value
				} else if c := f.closure("destination"); c != nil {
					dest = &swValue{Closure: c}
				}
			}
			view, what := sw.destination(dest)
			if view == "" {
				runtime(f, what)
				continue
			}
			e.to = view
			edges = append(edges, e)
			reach(view, f)
		case "navigationDestination", "sheet", "fullScreenCover", "popover":
			if f.Within == "" || f.Base == nil && f.Name != "navigationDestination" {
				continue
			}
			var dest *swValue
			if len(f.Closures) > 0 && f.Closures[0].Label == "" {
				dest = &f.Closures[0].Value
			} else if c := f.closure("destination"); c != nil {
				dest = &swValue{Closure: c}
			} else if c := f.closure("content"); c != nil {
				dest = &swValue{Closure: c}
			}
			how := f.Name
			if v := f.argument("for"); v != nil && v.TypeName != nil {
				how = fmt.Sprintf("navigationDestination(for: %s.self)", *v.TypeName)
			}
			view, what := sw.destination(dest)
			if view == "" {
				runtime(f, what)
				continue
			}
			edges = append(edges, swEdge{from: f.Within, to: view, fact: f, how: how})
			reach(view, f)
		}
	}
	views := 0
	for _, name := range sw.typeOrder {
		if sw.isView(name) {
			views++
		}
	}
	if views > 0 {
		sw.res.say("screens: counted %s, %d of them screens, and %s whose destination navigationDestination gives: every type of the files read that conforms to View, a screen when WindowGroup, a TabView, NavigationLink, navigationDestination, sheet, fullScreenCover or popover opens it", plural(views, "view"), len(screens), plural(typed, "NavigationLink with a value"))
	}
	if len(screens) == 0 {
		return
	}
	var order []string
	for v := range screens {
		order = append(order, v)
	}
	sort.Strings(order)
	pageOf := map[string]string{}
	taken := map[string]string{}
	for _, v := range order {
		name := kebab(strings.TrimSuffix(strings.TrimSuffix(v, "View"), "Screen"))
		if name == "" || taken[name] != "" {
			name = kebab(v)
		}
		if taken[name] != "" {
			sw.gap(sw.clause(sw.types[v].decl), []string{"pages"}, "", "the view %s would be the page %s, which %s is already; left out", v, name, taken[name])
			continue
		}
		taken[name] = v
		pageOf[v] = name
	}
	sw.pages = mapping()
	actions := sw.navigateActions(edges, pageOf)
	for _, v := range order {
		if pageOf[v] != "" {
			sw.writePage(v, pageOf[v], screens[v], actions[v])
		}
	}
	if len(tabs) > 0 {
		sw.menus = mapping()
		for _, t := range tabs {
			page := pageOf[t.view]
			if page == "" {
				continue
			}
			key := camel(strings.ReplaceAll(page, "-", "_"))
			for i := 2; child(sw.menus, key) != nil; i++ {
				key = fmt.Sprintf("%s%d", camel(strings.ReplaceAll(page, "-", "_")), i)
			}
			clause := sw.clause(t.fact)
			item := mapping()
			if t.title != "" {
				set(item, "title", t.title)
			}
			set(item, "page", page)
			sw.cite(clause)
			if t.title == "" {
				sw.question("must", fmt.Sprintf("What is the title of the tab of %s at %s? Its tab item shows no literal text.", t.view, clause),
					[]string{"#/menus/" + key + "/title"}, "A menu entry has a title, and the tab's label is not a literal.", sw.at(clause, "Shows "+t.view+" as a tab."))
			}
			set(sw.menus, key, item)
		}
		if count(sw.menus) == 0 {
			sw.menus = nil
		}
	}
}

// writePage writes one screen: its title from navigationTitle, its fields
// from the controls bound to a property, and its entity where the bound
// properties are of one SwiftData model.
func (sw *swiftReader) writePage(view, page string, opened *swFact, actions []swAction) {
	t := sw.types[view]
	decl := sw.clause(t.decl)
	at := "#/pages/" + page
	cites := []*yaml.Node{sw.at(decl, fmt.Sprintf("struct %s: View, which %s at %s opens.", view, opened.Name, sw.clause(opened)))}
	sw.cite(sw.clause(opened))
	var titles []string
	titleKnown := true
	var fields []string
	seenField := map[string]bool{}
	bases := map[string]bool{}
	var baseTypes []string
	propType := func(name string) string {
		for _, p := range t.props {
			if p.Name == name {
				typ := strings.TrimRight(p.Type, "?!")
				if typ == "" {
					typ = literalType(p.Value)
				}
				return typ
			}
		}
		return ""
	}
	for _, f := range sw.calls {
		if f.Within != view {
			continue
		}
		clause := sw.clause(f)
		if f.Name == "navigationTitle" && f.Base != nil && len(f.Arguments) > 0 {
			if s, ok := f.Arguments[0].Value.stringLiteral(); ok {
				if !contains(titles, s) {
					titles = append(titles, s)
					cites = append(cites, sw.at(clause, fmt.Sprintf("Its navigation title is %q.", s)))
				}
			} else {
				titleKnown = false
			}
			continue
		}
		labels, ok := swiftControls[f.Name]
		if !ok || f.Base != nil {
			continue
		}
		var bound *swValue
		for _, l := range labels {
			if v := f.argument(l); v != nil && v.Binding != nil {
				bound = v
			}
		}
		if bound == nil {
			continue
		}
		parts := strings.Split(*bound.Binding, ".")
		var field, base string
		switch len(parts) {
		case 1:
			field = parts[0]
		case 2:
			base, field = parts[0], parts[1]
		default:
			sw.gap(clause, []string{at + "/fields"}, "", "%s binds %s to $%s, a property of a property, and a page's field is one of its entity's; left out", view, f.Name, *bound.Binding)
			continue
		}
		if !seenField[field] {
			seenField[field] = true
			fields = append(fields, field)
			cites = append(cites, sw.at(clause, fmt.Sprintf("%s is bound to $%s.", f.Name, *bound.Binding)))
		}
		if !bases[base] {
			bases[base] = true
			typ := ""
			if base != "" {
				typ = propType(base)
			}
			baseTypes = append(baseTypes, typ)
		}
	}
	entity := ""
	if len(baseTypes) == 1 && baseTypes[0] != "" && sw.entityNames[baseTypes[0]] {
		entity = baseTypes[0]
	}
	if len(baseTypes) == 0 {
		var queried []string
		for _, p := range t.props {
			if _, ok := p.attribute("Query"); ok {
				typ := strings.TrimSuffix(strings.TrimPrefix(p.Type, "["), "]")
				if sw.entityNames[typ] && !contains(queried, typ) {
					queried = append(queried, typ)
					cites = append(cites, sw.at(sw.clause(p), fmt.Sprintf("It queries %s with @Query.", typ)))
				}
			}
		}
		if len(queried) == 1 {
			entity = queried[0]
		}
	}
	p := mapping()
	if len(titles) == 1 && titleKnown {
		set(p, "title", titles[0])
	}
	if entity != "" {
		set(p, "entity", entity)
	}
	if len(fields) > 0 {
		set(p, "fields", fields)
	}
	var labels []string
	if len(actions) > 0 {
		var list []*yaml.Node
		for i, a := range actions {
			item := mapping()
			if a.label != "" {
				set(item, "label", a.label)
			} else {
				labels = append(labels, fmt.Sprintf("%s/actions/%d/label", at, i))
			}
			set(item, "kind", "navigate")
			set(item, "target", a.target)
			list = append(list, flow(item))
			cites = append(cites, sw.at(a.clause, a.says))
		}
		set(p, "actions", list)
	}
	set(p, "origin", "stated")
	set(p, "cites", cites)
	set(sw.pages, page, p)
	asks := []string{"kind", "route"}
	blocks := []string{at + "/kind", at + "/route"}
	if child(p, "title") == nil {
		asks = append(asks, "title")
		blocks = append(blocks, at+"/title")
	}
	if entity == "" {
		asks = append(asks, "the entity it shows")
		blocks = append(blocks, at+"/entity")
	}
	asks = append(asks, "the operation it reads", "the operation it submits to", "the columns it lists")
	blocks = append(blocks, at+"/source", at+"/submit", at+"/columns")
	why := "A SwiftUI view says what it draws and how it is opened, not which of the four kinds of page it is, and an app's screen has no route of its own unless it answers a link."
	if len(titles) > 1 {
		why += fmt.Sprintf(" Its navigation titles are %s.", joinAnd(quoteAll(titles)))
	}
	sw.question("must", fmt.Sprintf("For the screen %s at %s, what are its %s?", view, decl, joinAnd(asks)), blocks, why, sw.at(decl, "struct "+view+": View."))
	if len(labels) > 0 {
		sw.question("must", fmt.Sprintf("What does a person tap on the screen %s to open %s? The source opens %s with no literal label.", view, joinAnd(labelTargets(actions)), itOrThem(len(labels))),
			labels, "A navigate action has a label, and a sheet, a navigationDestination or a link with a computed label gives none as a literal.", sw.at(decl, "struct "+view+": View."))
	}
	sw.question("must", fmt.Sprintf("Which permission does the screen %s check, or is it open to everyone (public)?", view),
		[]string{at + "/permission"}, "An app's screen checks nothing for the server, and a screen open to everyone is how an open screen is usually found, so it is asked, never assumed.")
}

// swAction is a navigate action a screen has: a way it opens another.
type swAction struct {
	label, target, clause, says, view string
}

// navigateActions is each screen's navigate actions, one per way it opens
// another screen, in the order of the source.
func (sw *swiftReader) navigateActions(edges []swEdge, pageOf map[string]string) map[string][]swAction {
	out := map[string][]swAction{}
	for _, e := range edges {
		clause := sw.clause(e.fact)
		to := pageOf[e.to]
		if to == "" {
			continue
		}
		if pageOf[e.from] == "" {
			sw.question("must", fmt.Sprintf("%s at %s opens the screen %s with %s, and %s is a part of a screen the reader did not find opened. Which screen holds %s?", e.from, clause, e.to, e.how, e.from, e.from),
				[]string{"pages"}, "A view is a screen when a navigation call opens it, and nothing read opens this one.", sw.at(clause, "Opens "+e.to+" with "+e.how+"."))
			continue
		}
		says := fmt.Sprintf("%s opens %s with %s.", e.from, e.to, e.how)
		if e.label != "" {
			says = fmt.Sprintf("%s opens %s with %s, labelled %q.", e.from, e.to, e.how, e.label)
		}
		out[e.from] = append(out[e.from], swAction{label: e.label, target: to, clause: clause, says: says, view: e.to})
	}
	return out
}

func labelTargets(actions []swAction) []string {
	var out []string
	for _, a := range actions {
		if a.label == "" && !contains(out, a.view) {
			out = append(out, a.view)
		}
	}
	return out
}

func quoteAll(items []string) []string {
	var out []string
	for _, s := range items {
		out = append(out, fmt.Sprintf("%q", s))
	}
	return out
}
