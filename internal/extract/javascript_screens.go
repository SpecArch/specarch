package extract

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// A locale's name, as a catalogue's file or folder gives it.
var localeWord = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// The folders message catalogues are kept in.
var catalogueFolders = map[string]bool{"messages": true, "locales": true, "lang": true, "langs": true, "i18n": true, "translations": true}

// jsCatalogue is the message catalogues of the folder read, by locale.
type jsCatalogue struct {
	locales  map[string]map[string]string // locale -> key -> message
	files    map[string][]string          // locale -> the files it is read from
	fallback string                       // the locale a page's text is read in
	asked    bool                         // the default locale was asked
}

// readCatalogues reads the message catalogues under the folder as data:
// messages/<locale>.json, locales/<locale>.json and
// locales/<locale>/<namespace>.json, nested keys joined by dots.
func (js *jsReader) readCatalogues() *jsCatalogue {
	c := &jsCatalogue{locales: map[string]map[string]string{}, files: map[string][]string{}}
	for _, f := range js.r.Files {
		if !js.under(f) || !strings.HasSuffix(f, ".json") || strings.Contains(f, "node_modules/") {
			continue
		}
		dir, base := path.Split(f)
		dir = strings.TrimSuffix(dir, "/")
		stem := strings.TrimSuffix(base, ".json")
		locale := ""
		switch {
		case catalogueFolders[path.Base(dir)] && localeWord.MatchString(stem):
			locale = stem
		case localeWord.MatchString(path.Base(dir)) && catalogueFolders[path.Base(path.Dir(dir))]:
			locale = path.Base(dir)
		default:
			continue
		}
		data := js.readData(f, "A message catalogue, read as data")
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			js.gap(f, []string{"pages"}, "", "the message catalogue %s does not parse as a JSON object; left out", f)
			continue
		}
		if c.locales[locale] == nil {
			c.locales[locale] = map[string]string{}
		}
		c.files[locale] = append(c.files[locale], f)
		flattenMessages("", doc, c.locales[locale])
	}
	if len(c.locales) == 0 {
		return c
	}
	// The default locale: one an i18next init() or next-intl's routing
	// names by a literal, or the one locale there is.
	var named []string
	var at string
	for _, f := range js.calls {
		for _, a := range f.Arguments {
			for _, k := range []string{"lng", "fallbackLng", "defaultLocale"} {
				if s, ok := a.prop(k).stringLiteral(); ok && c.locales[s] != nil && !contains(named, s) {
					if k != "fallbackLng" || len(named) == 0 {
						named = append(named, s)
						at = js.clause(f)
					}
				}
			}
		}
	}
	var locales []string
	for l := range c.locales {
		locales = append(locales, l)
	}
	sort.Strings(locales)
	switch {
	case len(named) >= 1:
		c.fallback = named[0]
		js.res.say("catalogues: read the message catalogues of %s; a page's text is read in %s, as %s names it", joinAnd(locales), c.fallback, at)
	case len(locales) == 1:
		c.fallback = locales[0]
		js.res.say("catalogues: read the message catalogues of %s, the one locale there is", c.fallback)
	default:
		c.asked = true
		js.question("should", fmt.Sprintf("The message catalogues hold the locales %s, and no literal in the code names the default. In which locale is the specification written?", joinAnd(locales)),
			[]string{"pages"}, "The meta-model holds one language, and a page's title is read from the catalogue of the default locale.")
	}
	for _, l := range locales {
		if l == c.fallback || c.fallback == "" {
			continue
		}
		for _, f := range c.files[l] {
			js.gap(f, []string{"pages"}, "", "the message catalogue %s is of the locale %s, and the meta-model holds one language, %s; left out", f, l, c.fallback)
		}
	}
	return c
}

func flattenMessages(prefix string, doc map[string]any, into map[string]string) {
	for k, v := range doc {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch x := v.(type) {
		case string:
			into[key] = x
		case map[string]any:
			flattenMessages(key, x, into)
		}
	}
}

// message is the text a value gives in the default locale: a literal, or
// t() with a literal key the catalogue holds. ok is false when it gives
// none; why says what was missing.
func (js *jsReader) message(v *jsValue, c *jsCatalogue) (text string, why string, ok bool) {
	if v == nil {
		return "", "nothing", false
	}
	if s, ok := v.stringLiteral(); ok {
		return s, "", true
	}
	if v.Call == nil || v.Call.Callee.memberName() != "t" {
		return "", v.describe() + ", which is not a literal", false
	}
	if len(v.Call.Arguments) == 0 {
		return "", "t() with no key", false
	}
	key, ok := v.Call.Arguments[0].stringLiteral()
	if !ok {
		return "", "t() with a key that is not a literal", false
	}
	if c.fallback == "" {
		return "", fmt.Sprintf("t(%q), and the catalogue's default locale is not known", key), false
	}
	m, found := c.locales[c.fallback][key]
	if !found {
		return "", fmt.Sprintf("t(%q), a key the %s catalogue does not hold", key, c.fallback), false
	}
	return m, "", true
}

// askComputedKey asks about a t() whose key is not a literal: which text
// it shows.
func (js *jsReader) askComputedKey(v *jsValue, clause, block string) {
	if v == nil || v.Call == nil || v.Call.Callee.memberName() != "t" || len(v.Call.Arguments) == 0 {
		return
	}
	if _, ok := v.Call.Arguments[0].stringLiteral(); ok {
		return
	}
	js.question("should", fmt.Sprintf("%s shows t(%s), a message whose key is not a literal. Which text does it show?", clause, v.Call.Arguments[0].describe()),
		[]string{block}, "A key computed at run time is not known by syntax, so the catalogue cannot give its text.", js.at(clause, "Shows a message by a computed key."))
}

// jsScreen is a route of React Router that shows a component.
type jsScreen struct {
	path      string
	route     string // the path as a page's route, {name} for :name
	component *jsFact
	name      string // the component's name
	clause    string // where the route is declared
	page      string
}

// readScreens reads React Router's routes as pages: the routes
// createBrowserRouter and the like are given, and the <Route> elements,
// with literal paths.
func (js *jsReader) readScreens() {
	var screens []*jsScreen
	declared := 0
	var walk func(v *jsValue, parent, clause string)
	add := func(p string, element *jsValue, component *jsValue, clause string) {
		declared++
		if component == nil && element != nil && element.Jsx != nil {
			component = &element.Jsx.Tag
		}
		if component == nil {
			js.question("must", fmt.Sprintf("The route %s at %s shows no component the reader finds. Which screen does it show?", p, clause),
				[]string{"pages"}, "A route is a screen through the component it shows, given as element or Component.", js.at(clause, "Declares the route "+p+"."))
			return
		}
		fn := js.functionOf(component)
		if fn == nil {
			js.question("must", fmt.Sprintf("The route %s at %s shows %s, which is not a component of the files read. Which screen is it?", p, clause, component.describe()),
				[]string{"pages"}, "A screen is read from its component's own code, and only tracked files are read.", js.at(clause, "Declares the route "+p+"."))
			return
		}
		screens = append(screens, &jsScreen{path: p, component: fn, name: component.describe(), clause: clause})
	}
	join := func(parent, p string) string {
		if strings.HasPrefix(p, "/") {
			return p
		}
		return strings.TrimSuffix(parent, "/") + "/" + p
	}
	walk = func(v *jsValue, parent, clause string) {
		if v == nil || v.Array == nil {
			return
		}
		for i := range v.Array {
			o := &v.Array[i]
			if o.Object == nil {
				js.question("must", fmt.Sprintf("A route at %s is %s, which is not a literal route object. Which routes does it declare?", clause, o.describe()),
					[]string{"pages"}, "Routes are read from literal objects.", js.at(clause, "Declares routes."))
				continue
			}
			p := parent
			clause := clause
			if len(o.Object) > 0 && o.Object[0].Line > 0 {
				clause = fmt.Sprintf("%s:%d", strings.SplitN(clause, ":", 2)[0], o.Object[0].Line)
			}
			if pv := o.prop("path"); pv != nil {
				s, ok := js.constString(pv)
				if !ok {
					js.question("must", fmt.Sprintf("A route at %s has the path %s, which is not a literal. Which path does it serve?", clause, pv.describe()),
						[]string{"pages"}, "A path computed at run time is not known by syntax.", js.at(clause, "Declares a route."))
					continue
				}
				p = join(parent, s)
			} else if idx := o.prop("index"); idx == nil || idx.Boolean == nil || !*idx.Boolean {
				p = ""
			}
			if lz := o.prop("lazy"); lz != nil {
				js.question("should", fmt.Sprintf("The route %s at %s loads its component lazily. Which screen does it show?", p, clause),
					[]string{"pages"}, "A component loaded by a function at run time is not followed.", js.at(clause, "Declares a lazy route."))
				continue
			}
			children := o.prop("children")
			if children != nil {
				walk(children, p, clause)
				continue
			}
			if p == "" {
				continue
			}
			add(p, o.prop("element"), o.prop("Component"), clause)
		}
	}
	for _, f := range js.calls {
		if f.Callee == nil || f.Callee.Name == nil || len(f.Arguments) == 0 {
			continue
		}
		switch *f.Callee.Name {
		case "createBrowserRouter", "createHashRouter", "createMemoryRouter", "useRoutes":
		default:
			continue
		}
		if _, ok := js.fromLibrary(f.Callee, "react-router", "react-router-dom"); !ok {
			continue
		}
		if f.Arguments[0].Array == nil {
			js.question("must", fmt.Sprintf("%s gives %s routes that are not a literal list: %s. Which routes does it declare?", js.clause(f), *f.Callee.Name, f.Arguments[0].describe()),
				[]string{"pages"}, "Routes are read from a literal list of route objects.", js.at(js.clause(f), "Declares routes."))
			continue
		}
		walk(&f.Arguments[0], "", js.clause(f))
	}
	// <Route> elements, nested under their parents.
	routeAt := map[string]*jsFact{}
	isRoute := func(f *jsFact) bool {
		if f.Tag == nil || f.Tag.Name == nil || *f.Tag.Name != "Route" {
			return false
		}
		_, ok := js.fromLibrary(f.Tag, "react-router", "react-router-dom")
		return ok
	}
	for _, f := range js.elements {
		if isRoute(f) {
			routeAt[fmt.Sprintf("%s:%d:%d", f.File, f.Line, f.Column)] = f
		}
	}
	attr := func(f *jsFact, name string) *jsValue {
		for i := range f.Attributes {
			if f.Attributes[i].Name == name {
				return f.Attributes[i].Value
			}
		}
		return nil
	}
	hasChildren := map[string]bool{}
	for _, f := range js.elements {
		if isRoute(f) && f.Parent != nil {
			hasChildren[fmt.Sprintf("%s:%d:%d", f.File, f.Parent.Line, f.Parent.Column)] = true
		}
	}
	var pathOf func(f *jsFact, depth int) (string, bool)
	pathOf = func(f *jsFact, depth int) (string, bool) {
		parent := ""
		if f.Parent != nil && depth < 32 {
			if pf := routeAt[fmt.Sprintf("%s:%d:%d", f.File, f.Parent.Line, f.Parent.Column)]; pf != nil {
				p, ok := pathOf(pf, depth+1)
				if !ok {
					return "", false
				}
				parent = p
			}
		}
		if pv := attr(f, "path"); pv != nil {
			s, ok := js.constString(pv)
			if !ok {
				return "", false
			}
			return join(parent, s), true
		}
		if idx := attr(f, "index"); idx != nil {
			return parent, true
		}
		return parent, true
	}
	for _, f := range js.elements {
		if !isRoute(f) {
			continue
		}
		clause := js.clause(f)
		if f.InLoop || f.InCondition {
			where := "in a loop"
			if !f.InLoop {
				where = "behind a condition"
			}
			js.question("must", fmt.Sprintf("%s declares a route %s. Which routes does the running app declare here?", clause, where),
				[]string{"pages"}, "Syntax does not run the code, so it cannot tell which routes a loop or a condition declares.", js.at(clause, "Declares a route "+where+"."))
			continue
		}
		if hasChildren[fmt.Sprintf("%s:%d:%d", f.File, f.Line, f.Column)] {
			continue
		}
		p, ok := pathOf(f, 0)
		if !ok {
			js.question("must", fmt.Sprintf("The route at %s, or one it is nested in, has a path that is not a literal. Which path does it serve?", clause),
				[]string{"pages"}, "A path computed at run time is not known by syntax.", js.at(clause, "Declares a route."))
			continue
		}
		if p == "" {
			continue
		}
		add(p, attr(f, "element"), attr(f, "Component"), clause)
	}
	if declared == 0 {
		return
	}
	js.res.say("screens: counted %s: every route with a path that a router of React Router declares, in a list or as a <Route>", plural(declared, "route"))
	cat := js.readCatalogues()
	byRoute := map[string]*jsScreen{}
	var held []*jsScreen
	for _, s := range screens {
		route, why := jsPath(splitSegments(s.path), "react-router")
		if why != "" {
			js.gap(s.clause, []string{"pages"}, "", "the route %s: %s; left out", s.path, why)
			continue
		}
		s.route = route
		if prev := byRoute[route]; prev != nil {
			js.question("must", fmt.Sprintf("The route %s is declared at %s and at %s. Which screen does the app show there?", route, prev.clause, s.clause),
				[]string{"pages"}, "A router shows one screen for a path.", js.at(s.clause, "Declares the route "+s.path+" again."))
			continue
		}
		byRoute[route] = s
		held = append(held, s)
	}
	// A page is named after its component; a component several routes
	// show names each of its pages by its route too.
	shows := map[string]int{}
	for _, s := range held {
		shows[s.component.File+":"+fmt.Sprint(s.component.Line)]++
	}
	byName := map[string]*jsScreen{}
	var named []*jsScreen
	for _, s := range held {
		base := s.name
		for _, suffix := range []string{"Page", "Screen", "View", "Route"} {
			if strings.HasSuffix(base, suffix) && len(base) > len(suffix) {
				base = strings.TrimSuffix(base, suffix)
				break
			}
		}
		s.page = kebab(base)
		if shows[s.component.File+":"+fmt.Sprint(s.component.Line)] > 1 {
			var words []string
			for _, seg := range splitSegments(s.route) {
				if strings.HasPrefix(seg, "{") {
					seg = "by-" + kebab(strings.Trim(seg, "{}"))
				}
				words = append(words, kebab(seg))
			}
			if len(words) == 0 {
				words = []string{"root"}
			}
			s.page += "-" + strings.Join(words, "-")
		}
		if prev := byName[s.page]; prev != nil || !pageNameWord.MatchString(s.page) {
			js.question("must", fmt.Sprintf("The screens at %s would both be named %s. Which name does each page take?", joinAnd([]string{prevRoute(prev), s.route}), s.page),
				[]string{"pages"}, "A page has one name, and two would become one.", js.at(s.clause, "Declares the route "+s.path+"."))
			continue
		}
		byName[s.page] = s
		named = append(named, s)
	}
	held = named
	if len(held) == 0 {
		return
	}
	sort.SliceStable(held, func(i, j int) bool { return held[i].page < held[j].page })
	js.pages = mapping()
	for _, s := range held {
		js.writeScreen(s, byRoute, cat)
	}
}

func prevRoute(s *jsScreen) string {
	if s == nil {
		return "one route"
	}
	return s.route
}

// A page's name, as the design schema takes it.
var pageNameWord = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// routeMatches says whether a link's path is a page's route, a {name}
// matching any one segment.
func routeMatches(route, link string) bool {
	rs, ls := strings.Split(route, "/"), strings.Split(strings.TrimSuffix(link, "/"), "/")
	if link == "/" {
		ls = []string{"", ""}
	}
	if len(rs) != len(ls) {
		return false
	}
	for i := range rs {
		if strings.HasPrefix(rs[i], "{") || strings.HasPrefix(ls[i], "{") {
			continue
		}
		if rs[i] != ls[i] {
			return false
		}
	}
	return true
}

// writeScreen writes one page: its route, its title from its heading,
// its fields from a form library and its navigate actions from its
// links.
func (js *jsReader) writeScreen(s *jsScreen, byRoute map[string]*jsScreen, cat *jsCatalogue) {
	fn := s.component
	decl := js.clause(fn)
	at := "#/pages/" + s.page
	key := (&jsPos{File: fn.File, Line: fn.Line, Column: fn.Column}).key()
	cites := []*yaml.Node{js.at(s.clause, fmt.Sprintf("The route %s shows %s.", s.path, s.name)), js.at(decl, "The component "+s.name+".")}
	var titles []string
	titleWhy := ""
	var fields []string
	type action struct {
		label, target, clause, says, why string
	}
	var actions []action
	registers := map[string]bool{}
	navigates := map[string]bool{}
	for _, f := range js.byWithin[key] {
		if f.Kind != "variable" || f.Value == nil || f.Value.Call == nil {
			continue
		}
		callee := &f.Value.Call.Callee
		switch callee.memberName() {
		case "useForm":
			if _, ok := js.fromLibrary(callee, "react-hook-form"); ok {
				for _, n := range f.Names {
					if n.Property == "register" || (n.Property == "" && n.Local == "register") {
						registers[n.Local] = true
					}
				}
			}
		case "useNavigate":
			if _, ok := js.fromLibrary(callee, "react-router", "react-router-dom"); ok && f.Name != "" {
				navigates[f.Name] = true
			}
		}
	}
	addField := func(name, clause, says string) {
		if !memberNameWord.MatchString(name) {
			js.gap(clause, []string{at + "/fields"}, "", "the field %s of %s is not a camelCase name a field can take; left out", name, s.name)
			return
		}
		if !contains(fields, name) {
			fields = append(fields, name)
			cites = append(cites, js.at(clause, says))
		}
	}
	for _, f := range js.byWithin[key] {
		clause := js.clause(f)
		switch f.Kind {
		case "call":
			name := f.Callee.memberName()
			switch {
			case f.Callee.Name != nil && registers[*f.Callee.Name] && len(f.Arguments) > 0:
				if n, ok := f.Arguments[0].stringLiteral(); ok {
					addField(n, clause, fmt.Sprintf("register(%q) binds the field %s.", n, n))
				} else {
					js.question("should", fmt.Sprintf("%s registers a form field by a name that is not a literal: %s. Which field is it?", clause, f.Arguments[0].describe()),
						[]string{at + "/fields"}, "A name computed at run time is not known by syntax.", js.at(clause, "Registers a field."))
				}
			case (name == "getFieldProps" || name == "useField") && len(f.Arguments) > 0:
				if n, ok := f.Arguments[0].stringLiteral(); ok {
					addField(n, clause, fmt.Sprintf("%s(%q) binds the field %s.", name, n, n))
				}
			case f.Callee.Name != nil && navigates[*f.Callee.Name] && len(f.Arguments) > 0:
				link, _, ok := js.urlTemplate(&f.Arguments[0])
				if !ok {
					js.question("should", fmt.Sprintf("%s navigates to %s, which is not a literal. Which screen does it open?", clause, f.Arguments[0].describe()),
						[]string{at + "/actions"}, "A destination chosen at run time is not known by syntax.", js.at(clause, "Navigates."))
					continue
				}
				actions = append(actions, action{target: link, clause: clause, says: fmt.Sprintf("navigate(%q) opens %s.", link, link), why: "navigate() is given no label"})
			}
		case "jsx":
			if f.Tag == nil {
				continue
			}
			tag := f.Tag.memberName()
			if f.Tag.Intrinsic != nil {
				tag = *f.Tag.Intrinsic
			}
			switch tag {
			case "h1":
				if len(f.Children) == 1 {
					var v *jsValue
					if f.Children[0].Text != nil {
						v = &jsValue{String: f.Children[0].Text}
					} else {
						v = f.Children[0].Expression
					}
					text, why, ok := js.message(v, cat)
					js.askComputedKey(v, clause, at+"/title")
					if ok {
						if !contains(titles, text) {
							titles = append(titles, text)
							cites = append(cites, js.at(clause, fmt.Sprintf("Its heading is %q.", text)))
						}
					} else {
						titleWhy = why
					}
				} else if len(f.Children) > 1 {
					titleWhy = "a heading of several parts"
				}
			case "Field":
				if _, ok := js.fromLibrary(f.Tag, "formik"); !ok {
					continue
				}
				for _, a := range f.Attributes {
					if a.Name == "name" {
						if n, ok := a.Value.stringLiteral(); ok {
							addField(n, clause, fmt.Sprintf("<Field name=%q> binds the field %s.", n, n))
						}
					}
				}
			case "Link", "NavLink":
				if _, ok := js.fromLibrary(f.Tag, "react-router", "react-router-dom"); !ok {
					continue
				}
				var to *jsValue
				for _, a := range f.Attributes {
					if a.Name == "to" {
						to = a.Value
					}
				}
				link, _, ok := js.urlTemplate(to)
				if !ok {
					js.question("should", fmt.Sprintf("%s links to %s, which is not a literal. Which screen does it open?", clause, to.describe()),
						[]string{at + "/actions"}, "A destination chosen at run time is not known by syntax.", js.at(clause, "Links."))
					continue
				}
				a := action{target: link, clause: clause}
				if len(f.Children) == 1 {
					var v *jsValue
					if f.Children[0].Text != nil {
						v = &jsValue{String: f.Children[0].Text}
					} else {
						v = f.Children[0].Expression
					}
					js.askComputedKey(v, clause, at+"/actions")
					if text, why, ok := js.message(v, cat); ok {
						a.label = text
					} else {
						a.why = "its text is " + why
					}
				} else {
					a.why = "its text is not one literal"
				}
				a.says = fmt.Sprintf("<%s to=%q> opens %s.", tag, link, link)
				actions = append(actions, a)
			}
		}
	}
	p := mapping()
	set(p, "route", s.route)
	if len(titles) == 1 && titleWhy == "" {
		set(p, "title", titles[0])
	}
	if len(fields) > 0 {
		set(p, "fields", fields)
	}
	var labels []string
	var list []*yaml.Node
	for _, a := range actions {
		var target *jsScreen
		for _, o := range byRoute {
			if routeMatches(o.route, a.target) && (target == nil || o.page < target.page) {
				target = o
			}
		}
		if target == nil {
			js.question("should", fmt.Sprintf("%s opens %s, which is not a route of the screens read. Which screen does it open?", a.clause, a.target),
				[]string{at + "/actions"}, "A navigate action's target is a page, and no route read matches the link.", js.at(a.clause, a.says))
			continue
		}
		item := mapping()
		if a.label != "" {
			set(item, "label", a.label)
		} else {
			labels = append(labels, fmt.Sprintf("%s/actions/%d/label", at, len(list)))
		}
		set(item, "kind", "navigate")
		set(item, "target", target.page)
		list = append(list, flow(item))
		cites = append(cites, js.at(a.clause, a.says))
	}
	if len(list) > 0 {
		set(p, "actions", list)
	}
	set(p, "origin", "stated")
	set(p, "cites", cites)
	set(js.pages, s.page, p)
	asks := []string{"kind"}
	blocks := []string{at + "/kind"}
	if child(p, "title") == nil {
		asks = append(asks, "title")
		blocks = append(blocks, at+"/title")
	}
	asks = append(asks, "the entity it shows", "the operation it reads", "the operation it submits to", "the columns it lists")
	blocks = append(blocks, at+"/entity", at+"/source", at+"/submit", at+"/columns")
	why := "A React component says what it draws, not which of the four kinds of page it is or the operations behind it."
	if titleWhy != "" {
		why += " Its heading is " + titleWhy + "."
	} else if len(titles) > 1 {
		why += fmt.Sprintf(" Its headings are %s.", joinAnd(quoteAll(titles)))
	}
	js.question("must", fmt.Sprintf("For the screen %s at %s, what are its %s?", s.name, s.route, joinAnd(asks)), blocks, why, js.at(decl, "The component "+s.name+"."))
	if len(labels) > 0 {
		js.question("must", fmt.Sprintf("What does a person click on the screen %s to follow each link that has no literal text?", s.name),
			labels, "A navigate action has a label, and the link's text is computed or not one literal.", js.at(decl, "The component "+s.name+"."))
	}
	js.question("must", fmt.Sprintf("Which permission does the screen %s check, or is it open to everyone (public)?", s.name),
		[]string{at + "/permission"}, "A browser's screen checks nothing for the server, and a screen open to everyone is how an open screen is usually found, so it is asked, never assumed.")
}
