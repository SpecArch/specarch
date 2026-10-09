package extract

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// uiComponent is a component of the project's UI library that the
// implementation file maps under bindings.ui.components.
type uiComponent struct {
	pkg, name, binds, nameProperty string
}

// The HTML controls whose v-model binds a field.
var vueControls = map[string]bool{"input": true, "select": true, "textarea": true}

// loadComponents reads the components the implementation file maps.
func (js *jsReader) loadComponents(file string, doc *yaml.Node) {
	list := child(child(child(doc, "bindings"), "ui"), "components")
	if list == nil {
		return
	}
	var names []string
	for i, item := range list.Content {
		c := uiComponent{pkg: scalar(child(item, "package")), name: scalar(child(item, "component")), binds: scalar(child(item, "binds")), nameProperty: scalar(child(item, "nameProperty"))}
		if c.pkg == "" || c.name == "" || (c.binds != "field" && c.binds != "column") {
			js.question("must", fmt.Sprintf("The implementation file %s maps a component (item %d of bindings.ui.components) without its package, its name and whether it binds a field or a column. Which component is it?", file, i+1),
				[]string{"pages"}, "A component mapped in part cannot be found in a template, and the fields it binds would be read as none.")
			continue
		}
		js.components = append(js.components, c)
		names = append(names, c.name)
	}
	if len(names) > 0 {
		js.res.say("components: %s maps %s", file, joinAnd(names))
	}
}

// componentOf is the mapped component a template element is: by its
// tag, and by the module its file imports it from, when it does; a
// component used with no import is matched by its name, since the
// framework or a global registration provides it.
func (js *jsReader) componentOf(f *jsFact) *uiComponent {
	tag := f.Element
	for i := range js.components {
		c := &js.components[i]
		if c.name != tag && kebab(c.name) != tag {
			continue
		}
		module := ""
		for _, im := range js.dump.Facts {
			if im.Kind != "import" || im.File != f.File {
				continue
			}
			for _, n := range im.Names {
				if n.Local == tag || kebab(n.Local) == tag {
					module = im.Module
				}
			}
		}
		if module == "" || module == c.pkg {
			return c
		}
	}
	return nil
}

func (a *jsAttr) literal() (string, bool) {
	if a.Value == nil {
		return "", false
	}
	return a.Value.stringLiteral()
}

// boundName is the field a v-model binds: form.title and title are both
// the field title.
func boundName(v *jsValue) string {
	switch {
	case v == nil:
		return ""
	case v.Member != nil:
		return v.Member.Name
	case v.Name != nil:
		return *v.Name
	}
	return ""
}

// vueScreen is what a Vue page's template and script say: its fields and
// columns in the order of the source, its heading, and their citations.
type vueScreen struct {
	fields, columns []string
	cites           []*yaml.Node
	title, titleAt  string
	titleWhy        string
}

// readVueScreen reads a .vue file's template: the fields v-model binds on
// a control or a mapped component, the fields a mapped component's name
// property names, the columns a mapped column component names, and its
// one heading.
func (js *jsReader) readVueScreen(file, at string, cat *jsCatalogue) vueScreen {
	var s vueScreen
	var titles []string
	add := func(list *[]string, name, clause, says string) {
		if !memberNameWord.MatchString(name) {
			js.gap(clause, []string{at}, "", "%s: %s is not a camelCase name a field can take; left out", clause, name)
			return
		}
		if !contains(*list, name) {
			*list = append(*list, name)
			s.cites = append(s.cites, js.at(clause, says))
		}
	}
	unmapped := map[string]string{}
	for _, f := range js.templates {
		if f.File != file {
			continue
		}
		clause := js.clause(f)
		if f.Element == "h1" {
			if len(f.Children) == 1 {
				v := f.Children[0].Expression
				if f.Children[0].Text != nil {
					v = &jsValue{String: f.Children[0].Text}
				}
				if text, why, ok := js.message(v, cat); ok {
					if !contains(titles, text) {
						titles = append(titles, text)
						s.titleAt = clause
					}
				} else {
					s.titleWhy = why
				}
			} else if len(f.Children) > 1 {
				s.titleWhy = "a heading of several parts"
			}
			continue
		}
		var model *jsAttr
		for i := range f.Attributes {
			if f.Attributes[i].Directive == "model" {
				model = &f.Attributes[i]
			}
		}
		c := js.componentOf(f)
		switch {
		case c != nil && c.nameProperty != "":
			for i := range f.Attributes {
				a := &f.Attributes[i]
				if a.Name != c.nameProperty || a.Directive != "" {
					continue
				}
				if n, ok := a.literal(); ok {
					if c.binds == "column" {
						add(&s.columns, n, clause, fmt.Sprintf("<%s %s=%q> is the column %s.", f.Element, c.nameProperty, n, n))
					} else {
						add(&s.fields, n, clause, fmt.Sprintf("<%s %s=%q> binds the field %s.", f.Element, c.nameProperty, n, n))
					}
				}
			}
			for i := range f.Attributes {
				a := &f.Attributes[i]
				if a.Name == c.nameProperty && a.Directive == "bind" {
					js.question("should", fmt.Sprintf("%s gives <%s> its %s as %s, which is not a literal. Which field is it?", clause, f.Element, c.nameProperty, a.Value.describe()),
						[]string{at}, "A name computed at run time is not known by syntax.", js.at(clause, "Binds a field by a computed name."))
				}
			}
		case model != nil && (c != nil || vueControls[f.Element]):
			n := boundName(model.Value)
			if n == "" {
				js.question("should", fmt.Sprintf("%s binds <%s> with v-model to %s, which names no property. Which field is it?", clause, f.Element, model.Value.describe()),
					[]string{at}, "A field is the property v-model binds, and this binding names none.", js.at(clause, "Binds with v-model."))
				continue
			}
			list := &s.fields
			if c != nil && c.binds == "column" {
				list = &s.columns
			}
			add(list, n, clause, fmt.Sprintf("<%s v-model=%q> binds the field %s.", f.Element, model.Value.describe(), n))
		case model != nil:
			if _, seen := unmapped[f.Element]; !seen {
				unmapped[f.Element] = clause
			}
		}
	}
	var tags []string
	for t := range unmapped {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	for _, t := range tags {
		js.question("should", fmt.Sprintf("%s binds the component <%s> with v-model, and the implementation file maps no component of that name. Which field does it bind, if any?", unmapped[t], t),
			[]string{at + "/fields"}, "A component's v-model binds a field only where the implementation file says the component edits one; a component of a library is otherwise not known.", js.at(unmapped[t], "Binds <"+t+"> with v-model."))
	}
	if len(titles) == 1 && s.titleWhy == "" {
		s.title = titles[0]
		s.cites = append(s.cites, js.at(s.titleAt, fmt.Sprintf("Its heading is %q.", s.title)))
	} else if len(titles) > 1 {
		s.titleWhy = "one of several headings: " + joinAnd(quoteAll(titles))
	}
	return s
}

// vueComponentFile is the .vue file a route's component value names: an
// import of a .vue file the dump read, or a function that imports one.
func (js *jsReader) vueComponentFile(v *jsValue, from string) string {
	if v == nil {
		return ""
	}
	if v.Name != nil && v.Import != nil {
		for _, im := range js.dump.Facts {
			if im.Kind == "import" && im.File == from && im.Module == v.Import.Module && im.Resolved != "" && strings.HasSuffix(im.Resolved, ".vue") {
				return im.Resolved
			}
		}
	}
	if v.Function != nil {
		key := v.Function.key()
		for _, im := range js.dump.Facts {
			// A dynamic import inside the function itself.
			if im.Kind == "import" && im.Dynamic && im.Within.key() == key && strings.HasSuffix(im.Resolved, ".vue") {
				return im.Resolved
			}
		}
	}
	return ""
}

// vueRoute is a route vue-router's createRouter is given.
type vueRoute struct {
	path, route, clause, file, page string
	guards                          []jsValue
}

// readVueRoutes reads the routes createRouter is given as pages: each
// route with a path whose component is a .vue file the dump read, with
// the guards beforeEach and the route's beforeEnter give.
func (js *jsReader) readVueRoutes(cat *jsCatalogue) {
	var routes []*vueRoute
	var global []jsValue
	var globalAt []string
	for _, f := range js.calls {
		if f.Callee == nil || f.Callee.Member == nil || f.Callee.Member.Name != "beforeEach" || len(f.Arguments) == 0 {
			continue
		}
		global = append(global, f.Arguments[0])
		globalAt = append(globalAt, js.clause(f))
	}
	declared := 0
	for _, f := range js.calls {
		if f.Callee == nil || f.Callee.Name == nil || *f.Callee.Name != "createRouter" || len(f.Arguments) == 0 {
			continue
		}
		if _, ok := js.fromLibrary(f.Callee, "vue-router"); !ok {
			continue
		}
		list := f.Arguments[0].prop("routes")
		if list == nil || list.Array == nil {
			js.question("must", fmt.Sprintf("%s gives createRouter routes that are not a literal list. Which routes does it declare?", js.clause(f)),
				[]string{"pages"}, "Routes are read from a literal list of route objects.", js.at(js.clause(f), "Declares routes."))
			continue
		}
		var walk func(v *jsValue, parent string, guards []jsValue)
		walk = func(v *jsValue, parent string, guards []jsValue) {
			for i := range v.Array {
				o := &v.Array[i]
				clause := js.clause(f)
				if len(o.Object) > 0 && o.Object[0].Line > 0 {
					clause = fmt.Sprintf("%s:%d", f.File, o.Object[0].Line)
				}
				if o.Object == nil {
					js.question("must", fmt.Sprintf("A route at %s is %s, which is not a literal route object. Which routes does it declare?", clause, o.describe()),
						[]string{"pages"}, "Routes are read from literal objects.", js.at(clause, "Declares routes."))
					continue
				}
				p := parent
				if pv := o.prop("path"); pv != nil {
					s, ok := js.constString(pv)
					if !ok {
						js.question("must", fmt.Sprintf("A route at %s has the path %s, which is not a literal. Which path does it serve?", clause, pv.describe()),
							[]string{"pages"}, "A path computed at run time is not known by syntax.", js.at(clause, "Declares a route."))
						continue
					}
					if strings.HasPrefix(s, "/") || parent == "" {
						p = "/" + strings.TrimPrefix(s, "/")
					} else {
						p = strings.TrimSuffix(parent, "/") + "/" + s
					}
				}
				g := guards
				if be := o.prop("beforeEnter"); be != nil {
					g = append(append([]jsValue{}, guards...), *be)
				}
				if children := o.prop("children"); children != nil && children.Array != nil {
					walk(children, p, g)
					continue
				}
				comp := o.prop("component")
				if comp == nil {
					continue
				}
				declared++
				file := js.vueComponentFile(comp, f.File)
				if file == "" {
					js.question("must", fmt.Sprintf("The route %s at %s shows %s, which is not a .vue file of the files read. Which screen does it show?", p, clause, comp.describe()),
						[]string{"pages"}, "A screen is read from its component's own file, and only tracked files are read.", js.at(clause, "Declares the route "+p+"."))
					continue
				}
				routes = append(routes, &vueRoute{path: p, clause: clause, file: file, guards: g})
			}
		}
		walk(list, "", nil)
	}
	if declared == 0 {
		return
	}
	js.res.say("screens: counted %s: every route with a path and a component that vue-router's createRouter is given in a literal list", plural(declared, "route"))
	if cat == nil {
		cat = js.readCatalogues()
	}
	byRoute := map[string]*vueRoute{}
	byName := map[string]*vueRoute{}
	var held []*vueRoute
	for _, rt := range routes {
		route, why := jsPath(splitSegments(rt.path), "vue-router")
		if why != "" {
			js.gap(rt.clause, []string{"pages"}, "", "the route %s: %s; left out", rt.path, why)
			continue
		}
		rt.route = route
		if prev := byRoute[route]; prev != nil {
			js.question("must", fmt.Sprintf("The route %s is declared at %s and at %s. Which screen does the app show there?", route, prev.clause, rt.clause),
				[]string{"pages"}, "A router shows one screen for a path.", js.at(rt.clause, "Declares the route "+rt.path+" again."))
			continue
		}
		byRoute[route] = rt
		base := strings.TrimSuffix(path.Base(rt.file), ".vue")
		for _, suffix := range []string{"Page", "Screen", "View"} {
			if strings.HasSuffix(base, suffix) && len(base) > len(suffix) {
				base = strings.TrimSuffix(base, suffix)
				break
			}
		}
		rt.page = kebab(base)
		if prev := byName[rt.page]; prev != nil || !pageNameWord.MatchString(rt.page) {
			js.question("must", fmt.Sprintf("The screens at %s would both be named %s. Which name does each page take?", joinAnd([]string{routeOf(prev), rt.route}), rt.page),
				[]string{"pages"}, "A page has one name, and two would become one.", js.at(rt.clause, "Declares the route "+rt.path+"."))
			continue
		}
		byName[rt.page] = rt
		held = append(held, rt)
	}
	if len(held) == 0 {
		return
	}
	sort.SliceStable(held, func(i, j int) bool { return held[i].page < held[j].page })
	if js.pages == nil {
		js.pages = mapping()
	}
	for _, rt := range held {
		at := "#/pages/" + rt.page
		vs := js.readVueScreen(rt.file, at, cat)
		p := mapping("route", rt.route)
		if vs.title != "" {
			set(p, "title", vs.title)
		}
		if len(vs.columns) > 0 {
			set(p, "columns", vs.columns)
		}
		if len(vs.fields) > 0 {
			set(p, "fields", vs.fields)
		}
		cites := []*yaml.Node{js.at(rt.clause, fmt.Sprintf("The route %s shows %s.", rt.path, rt.file)), js.at(rt.file, "The component of "+rt.route+".")}
		cites = append(cites, vs.cites...)
		guards := append(append([]jsValue{}, global...), rt.guards...)
		permission, why := js.guardPermission(guards, globalAt, rt.route)
		if permission != "" {
			set(p, "permission", permission)
			js.pagePermission(permission, rt.route, at)
		}
		set(p, "origin", "stated")
		set(p, "cites", cites)
		set(js.pages, rt.page, p)
		asks := []string{"kind"}
		blocks := []string{at + "/kind"}
		if vs.title == "" {
			asks = append(asks, "title")
			blocks = append(blocks, at+"/title")
		}
		asks = append(asks, "the entity it shows", "the operation it reads", "the operation it submits to")
		blocks = append(blocks, at+"/entity", at+"/source", at+"/submit")
		if len(vs.columns) == 0 {
			asks = append(asks, "the columns it lists")
			blocks = append(blocks, at+"/columns")
		}
		whyContent := "A Vue component says what it draws, not which of the four kinds of page it is or the operations behind it."
		if vs.titleWhy != "" {
			whyContent += " Its heading is " + vs.titleWhy + "."
		}
		js.question("must", fmt.Sprintf("For the screen %s at %s, what are its %s?", rt.file, rt.route, joinAnd(asks)), blocks, whyContent, js.at(rt.file, "The component of "+rt.route+"."))
		if permission == "" {
			js.question("must", why+" Is it meant to be open to everyone (public), or which permission should it check?",
				[]string{at + "/permission"}, "A screen open to everyone is how an open screen is usually found, so it is asked, never assumed.")
		}
	}
	js.writePagePermissions()
}

func routeOf(rt *vueRoute) string {
	if rt == nil {
		return "one route"
	}
	return rt.route
}

// guardPermission is the permission the guards of a route give through
// the checks the implementation file names: one literal permission among
// the checks every guard function calls; why says why there is none.
func (js *jsReader) guardPermission(guards []jsValue, globalAt []string, route string) (string, string) {
	if len(guards) == 0 {
		return "", fmt.Sprintf("The screen at %s has no guard: vue-router's beforeEach and the route's beforeEnter give none.", route)
	}
	var found []string
	for i := range guards {
		fn := js.functionOf(&guards[i])
		if fn == nil {
			continue
		}
		rt := &jsRoute{fact: &jsFact{File: fn.File, Line: fn.Line}, handler: fn}
		js.routePermission(rt, nil, nil)
		if rt.permission != "" {
			found = append(found, rt.permission)
			if js.permAt == nil {
				js.permAt = map[string]string{}
			}
			if js.permAt[rt.permission] == "" && len(rt.checks) > 0 {
				js.permAt[rt.permission] = rt.checks[0]
			}
		}
	}
	switch {
	case len(found) == 1:
		return found[0], ""
	case len(found) > 1:
		return "", fmt.Sprintf("The screen at %s is guarded by %d checks the implementation file names, and a page checks one permission.", route, len(found))
	case js.checkFile == "":
		return "", fmt.Sprintf("The screen at %s is guarded, and no implementation file names the project's checks, so the reader knows none its guards call.", route)
	}
	return "", fmt.Sprintf("The screen at %s is guarded by a function that calls no check the implementation file names with a literal permission.", route)
}

// pagePermission records a permission a screen checks, to declare it.
func (js *jsReader) pagePermission(permission, route, at string) {
	if js.pagePerms == nil {
		js.pagePerms = map[string][]string{}
	}
	js.pagePerms[permission] = append(js.pagePerms[permission], route)
}

// writePagePermissions declares each permission a screen's guard gives
// that no operation declared.
func (js *jsReader) writePagePermissions() {
	if len(js.pagePerms) == 0 {
		return
	}
	if js.permissions == nil {
		js.permissions = mapping()
	}
	var names []string
	for n := range js.pagePerms {
		if child(js.permissions, n) == nil {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	var blocks, grants []string
	for _, n := range names {
		set(js.permissions, n, mapping("origin", "stated", "cites", []*yaml.Node{js.at(js.permAt[n], fmt.Sprintf("The screens at %s %s %s.", joinAnd(js.pagePerms[n]), checkOrChecks(len(js.pagePerms[n])), n))}))
		blocks = append(blocks, "#/permissions/"+escapeToken(n)+"/description")
		grants = append(grants, "#/permissions/"+escapeToken(n))
	}
	if len(names) == 0 {
		return
	}
	js.question("must", fmt.Sprintf("What does each permission allow: %s?", strings.Join(names, ", ")), blocks, "The source names the permission a screen's guard checks and not what it is for.")
	js.question("must", fmt.Sprintf("Which role grants each permission: %s?", strings.Join(names, ", ")), grants, "The source names the permission a screen's guard checks and not who holds it.")
}
