package extract

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/wirename"
)

// isDartModel says whether a class is a json_serializable or freezed
// model.
func isDartModel(c *dartFact) bool {
	return c.annotation("JsonSerializable", "freezed", "Freezed", "unfreezed") != nil
}

// dartModelFields is a model's fields: a json_serializable class's
// instance fields, or the parameters of a freezed class's factory.
func (dr *dartReader) dartModelFields(c *dartFact) []dartParam {
	freezed := c.annotation("freezed", "Freezed", "unfreezed") != nil
	var out []dartParam
	for _, m := range dr.members[c.Name] {
		switch {
		case freezed && m.Kind == "constructor" && m.Factory && m.Redirect != "":
			if len(out) == 0 {
				for _, p := range m.Parameters {
					p := p
					out = append(out, p)
				}
				dr.fieldAt[c.Name] = m
			}
		case !freezed && m.Kind == "field" && !m.Static:
			p := dartParam{Name: m.Name, Type: m.Type, Annotations: m.Annotations}
			if m.Value != nil {
				p.Default = m.Value
			}
			out = append(out, p)
			dr.memberAt[c.Name+"."+m.Name] = m
		}
	}
	return out
}

// dartField writes a Dart type as a field.
func (dr *dartReader) dartField(t, at string) swField {
	t = strings.TrimSpace(t)
	if strings.HasSuffix(t, "?") {
		f := dr.dartField(t[:len(t)-1], at)
		f.nullable = true
		return f
	}
	if (strings.HasPrefix(t, "List<") || strings.HasPrefix(t, "Iterable<")) && strings.HasSuffix(t, ">") {
		inner := t[strings.Index(t, "<")+1 : len(t)-1]
		item := dr.dartField(inner, at+"/items")
		if item.held != "" {
			return item
		}
		if item.nullable || item.array {
			return swField{held: "a list whose items are nullable or lists themselves, which a field's items do not hold"}
		}
		f := swField{node: flow(mapping("type", "array", "items", item.node)), model: item.model, array: true}
		for _, a := range item.asks {
			a.key = "/items" + a.key
			f.asks = append(f.asks, a)
		}
		return f
	}
	switch {
	case strings.HasPrefix(t, "Map<") || strings.HasPrefix(t, "Set<") || t == "Map" || t == "Set":
		return swField{held: "the type " + t + ", a map or a set, which a field does not hold"}
	}
	switch t {
	case "":
		return swField{node: flow(mapping()), asks: []swAsk{{"must", "", "the source declares no type for it. Which type is it?", "A field has a type, and the source writes none."}}}
	case "String":
		return swField{node: fieldOf([]any{"type", "string"})}
	case "bool":
		return swField{node: fieldOf([]any{"type", "boolean"})}
	case "double":
		return swField{node: fieldOf([]any{"type", "number", "format", "double"})}
	case "int":
		dr.widths = append(dr.widths, at+"/format")
		return swField{node: fieldOf([]any{"type", "integer"})}
	case "num":
		return swField{node: fieldOf([]any{"type", "number"}), asks: []swAsk{{"must", "/format", "it is a num, an int or a double. Which is it, and how wide?", "A num is either kind of number, and a field has one."}}}
	case "DateTime":
		return swField{node: fieldOf([]any{"type", "string", "format", "date-time"})}
	case "Uri":
		return swField{node: fieldOf([]any{"type", "string", "format", "uri"})}
	case "dynamic", "Object", "Object?":
		return swField{node: flow(mapping()), asks: []swAsk{{"must", "", "the source declares it " + t + ", which says no type. Which type is it?", "A field has a type, and dynamic and Object say none."}}}
	}
	if e := dr.enums[t]; e != nil {
		dr.wantEnum(e)
		return swField{node: flow(mapping("$ref", "#/enums/"+t)), model: t}
	}
	if c := dr.classes[t]; c != nil && isDartModel(c) {
		return swField{node: flow(mapping("$ref", "#/schemas/"+t)), model: t}
	}
	return swField{node: flow(mapping()), asks: []swAsk{{"must", "", fmt.Sprintf("the source declares the type %s, which is not a model or an enum of the files read, such as a type of a package. Which type is it?", t), "Only tracked files are read, and a type of a package is not known."}}}
}

func (dr *dartReader) wantEnum(e *dartFact) {
	if dr.enumWanted[e.Name] {
		return
	}
	dr.enumWanted[e.Name] = true
	dr.enumOrder = append(dr.enumOrder, e)
}

// readModels writes each json_serializable and freezed class as a schema,
// and each enum they hold as an enum.
func (dr *dartReader) readModels() {
	dr.fieldAt, dr.memberAt, dr.enumWanted = map[string]*dartFact{}, map[string]*dartFact{}, map[string]bool{}
	var names []string
	for n, c := range dr.classes {
		if isDartModel(c) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return
	}
	snakeAt := ""
	for _, n := range names {
		c := dr.classes[n]
		if a := c.annotation("JsonSerializable"); a != nil {
			if v := dartArgument(a.Arguments, "fieldRename"); v != nil && v.Name != nil && *v.Name == "FieldRename.snake" {
				dr.wireSnake = true
				snakeAt = dr.clause(c)
			}
		}
	}
	type camelNote struct{ clause, schema, prop string }
	var camel []camelNote
	fields := 0
	for _, n := range names {
		c := dr.classes[n]
		clause := dr.clause(c)
		if !pascalWord.MatchString(n) {
			dr.gap(clause, []string{"schemas"}, "", "the model %s: its name is not a PascalCase name a schema can take; left out", n)
			continue
		}
		at := "#/schemas/" + n
		classSnake := false
		if a := c.annotation("JsonSerializable"); a != nil {
			if v := dartArgument(a.Arguments, "fieldRename"); v != nil && v.Name != nil {
				switch *v.Name {
				case "FieldRename.snake":
					classSnake = true
				case "FieldRename.none":
				default:
					dr.gap(clause, []string{at}, "", "%s renames its fields with %s, which info.wireNames cannot give; written by their Dart names", n, *v.Name)
				}
			}
		}
		props := mapping()
		var required []string
		for _, p := range dr.dartModelFields(c) {
			fc := clause
			if m := dr.memberAt[n+"."+p.Name]; m != nil {
				fc = dr.clause(m)
			} else if m := dr.fieldAt[n]; m != nil {
				fc = dr.clause(m)
			}
			if !memberNameWord.MatchString(p.Name) {
				dr.gap(fc, []string{at}, "", "%s.%s: the field's name is not a camelCase name a field can take; left out", n, p.Name)
				continue
			}
			keyed := false
			skip := false
			def := p.Default
			for _, a := range p.Annotations {
				switch a.Name {
				case "JsonKey":
					if v := dartArgument(a.Arguments, "name"); v != nil {
						if w, ok := v.stringLiteral(); ok {
							keyed = true
							switch {
							case w == p.Name:
							case w == wirename.Snake(p.Name):
								dr.wireSnake = true
							default:
								dr.gap(fc, []string{at + "/properties/" + p.Name}, "", "%s.%s goes on the wire as %s, which info.wireNames cannot give; written by its Dart name", n, p.Name, w)
							}
						}
					}
					for _, k := range []string{"includeFromJson", "includeToJson", "ignore"} {
						if v := dartArgument(a.Arguments, k); v != nil && v.Boolean != nil && *v.Boolean == (k == "ignore") {
							skip = true
						}
					}
					if v := dartArgument(a.Arguments, "defaultValue"); v != nil {
						def = v
					}
				case "Default":
					if len(a.Arguments) > 0 {
						def = &a.Arguments[0].Value
					}
				}
			}
			if skip {
				dr.gap(fc, []string{at}, "", "%s.%s is left out of its JSON by its @JsonKey; left out", n, p.Name)
				continue
			}
			if !keyed && !classSnake && wirename.Snake(p.Name) != p.Name {
				camel = append(camel, camelNote{fc, n, p.Name})
			}
			fat := at + "/properties/" + p.Name
			f := dr.dartField(p.Type, fat)
			if f.held != "" {
				dr.gap(fc, []string{at}, "", "%s.%s: %s; left out", n, p.Name, f.held)
				continue
			}
			node := f.node
			if f.nullable {
				node = nullable(node)
			}
			if def != nil {
				if lit := dartLiteralNode(def); lit != nil {
					node = flow(node)
					setKey(node, "default", lit)
				}
			}
			set(props, p.Name, node)
			fields++
			if !f.nullable && def == nil {
				required = append(required, p.Name)
			}
			for _, a := range f.asks {
				dr.question(a.priority, fmt.Sprintf("%s.%s at %s: %s", n, p.Name, fc, a.text), []string{fat + a.key}, a.why, dr.at(fc, "Declares "+p.Name+"."))
			}
		}
		if len(props.Content) == 0 {
			dr.gap(clause, []string{"schemas"}, "", "%s declares no field a schema can hold; left out", n)
			continue
		}
		el := mapping("type", "object", "properties", props)
		if len(required) > 0 {
			set(el, "required", required)
		}
		kind := "@JsonSerializable"
		if c.annotation("freezed", "Freezed", "unfreezed") != nil {
			kind = "@freezed"
		}
		set(el, "origin", "stated")
		set(el, "cites", []*yaml.Node{dr.at(clause, fmt.Sprintf("class %s is %s.", n, kind))})
		if dr.schemas == nil {
			dr.schemas = mapping()
		}
		set(dr.schemas, n, el)
	}
	if dr.wireSnake {
		for _, w := range camel {
			dr.gap(w.clause, []string{"#/schemas/" + w.schema + "/properties/" + w.prop}, "", "%s.%s goes on the wire as %s, since neither its class nor a @JsonKey renames it, and info.wireNames snake_case, which another model gives, would write it %s", w.schema, w.prop, w.prop, wirename.Snake(w.prop))
		}
	}
	_ = snakeAt
	for _, e := range dr.enumOrder {
		dr.writeEnum(e)
	}
	dr.res.say("models: counted %s of %s and %s: every field of a json_serializable class and every parameter of a freezed factory, and every enum they hold", plural(fields, "field"), plural(count(dr.schemas), "model"), plural(count(dr.enumsOut), "enum"))
}

func (dr *dartReader) writeEnum(e *dartFact) {
	clause := dr.clause(e)
	var values []string
	for _, v := range e.Values {
		w := v.Name
		for _, a := range v.Annotations {
			if a.Name == "JsonValue" && len(a.Arguments) > 0 {
				if s, ok := a.Arguments[0].Value.stringLiteral(); ok {
					w = s
				}
			}
		}
		if !snakeWord.MatchString(w) {
			dr.gap(fmt.Sprintf("%s:%d", e.File, v.Line), []string{"#/enums/" + e.Name}, "", "%s.%s is %q on the wire, and an enum's values are snake_case words; left out", e.Name, v.Name, w)
			continue
		}
		values = append(values, w)
	}
	if len(values) == 0 {
		dr.gap(clause, []string{"enums"}, "", "the enum %s has no value an enum can hold; left out", e.Name)
		return
	}
	if dr.enumsOut == nil {
		dr.enumsOut = mapping()
	}
	set(dr.enumsOut, e.Name, mapping("type", "string", "enum", values, "origin", "stated",
		"cites", []*yaml.Node{dr.at(clause, fmt.Sprintf("enum %s has the values %s.", e.Name, strings.Join(values, ", ")))}))
}

func dartLiteralNode(v *dartValue) *yaml.Node {
	n := &yaml.Node{}
	switch {
	case v.String != nil:
		return str(*v.String)
	case v.Integer != nil:
		_ = n.Encode(yamlNumber(*v.Integer))
	case v.Double != nil:
		_ = n.Encode(yamlNumber(*v.Double))
	case v.Boolean != nil:
		_ = n.Encode(*v.Boolean)
	default:
		return nil
	}
	return n
}

// dartScreen is a screen the routes or Navigator open.
type dartScreen struct {
	widget, page, route, clause string
	guard                       string // why a redirect guards it, or ""
}

// ownerWidget is the widget a class is the State of, or the class itself.
func (dr *dartReader) ownerWidget(class string) string {
	if c := dr.classes[class]; c != nil && strings.HasPrefix(c.Extends, "State<") {
		return strings.TrimSuffix(strings.TrimPrefix(c.Extends, "State<"), ">")
	}
	return class
}

// builtWidget is the widget a builder returns: MaterialPage(child: X())
// gives X.
func builtWidget(fn *dartValue) (string, string) {
	if fn == nil || fn.Function == nil || len(fn.Function.Returns) != 1 {
		return "", "a builder whose result is not one widget"
	}
	v := &fn.Function.Returns[0]
	for i := 0; i < 4 && v != nil && v.Call != nil; i++ {
		name := v.Call.Name
		switch name {
		case "MaterialPage", "CupertinoPage", "NoTransitionPage", "CustomTransitionPage":
			v = dartArgument(v.Call.Arguments, "child")
			continue
		}
		if v.Call.Target == nil && name != "" && name[0] >= 'A' && name[0] <= 'Z' {
			return name, ""
		}
		break
	}
	return "", "a builder that returns " + v.describe()
}

// readScreens reads go_router's routes, MaterialApp's routes and the
// screens Navigator.push opens as pages, with their titles, the fields
// their forms save and the ways they open each other.
func (dr *dartReader) readScreens() {
	var screens []*dartScreen
	declared := 0
	var menus []struct {
		page, clause string
	}
	var walk func(list *dartValue, parent, clause, guard string, branch bool)
	walk = func(list *dartValue, parent, clause, guard string, branch bool) {
		if list == nil || list.List == nil {
			return
		}
		for i := range list.List {
			v := &list.List[i]
			if v.Call == nil {
				dr.question("must", fmt.Sprintf("A route at %s is %s, which is not a literal route. Which routes does it declare?", clause, v.describe()),
					[]string{"pages"}, "Routes are read from literal GoRoute calls.", dr.at(clause, "Declares routes."))
				continue
			}
			c := v.Call
			if c.Line > 0 {
				clause = fmt.Sprintf("%s:%d", strings.SplitN(clause, ":", 2)[0], c.Line)
			}
			name := c.Name
			if c.Target != nil && c.Target.Name != nil {
				name = *c.Target.Name + "." + c.Name
			}
			switch name {
			case "ShellRoute":
				walk(dartArgument(c.Arguments, "routes"), parent, clause, guard, false)
				continue
			case "StatefulShellRoute", "StatefulShellRoute.indexedStack":
				if b := dartArgument(c.Arguments, "branches"); b != nil {
					for j := range b.List {
						if bc := b.List[j].Call; bc != nil && bc.Name == "StatefulShellBranch" {
							walk(dartArgument(bc.Arguments, "routes"), parent, clause, guard, true)
						}
					}
				}
				continue
			case "GoRoute":
			default:
				continue
			}
			declared++
			pv := dartArgument(c.Arguments, "path")
			p, ok := pv.stringLiteral()
			if !ok {
				dr.question("must", fmt.Sprintf("A GoRoute at %s has the path %s, which is not a literal. Which path does it serve?", clause, pv.describe()),
					[]string{"pages"}, "A path computed at run time is not known by syntax.", dr.at(clause, "Declares a route."))
				continue
			}
			full := p
			if !strings.HasPrefix(p, "/") {
				full = strings.TrimSuffix(parent, "/") + "/" + p
			}
			g := guard
			if dartArgument(c.Arguments, "redirect") != nil {
				g = "the GoRoute of " + full + " has a redirect, which is code"
			}
			builder := dartArgument(c.Arguments, "builder")
			if builder == nil {
				builder = dartArgument(c.Arguments, "pageBuilder")
			}
			if builder != nil {
				widget, why := builtWidget(builder)
				if widget == "" {
					dr.question("must", fmt.Sprintf("The route %s at %s shows %s. Which screen is it?", full, clause, why),
						[]string{"pages"}, "A screen is the widget its route's builder returns, written as one constructor.", dr.at(clause, "Declares the route "+full+"."))
				} else {
					screens = append(screens, &dartScreen{widget: widget, route: full, clause: clause, guard: g})
					if branch {
						menus = append(menus, struct{ page, clause string }{widget, clause})
						branch = false
					}
				}
			}
			walk(dartArgument(c.Arguments, "routes"), full, clause, g, false)
		}
	}
	for _, f := range dr.calls {
		if f.Name != "GoRouter" || f.Target != nil {
			continue
		}
		guard := ""
		if dartArgument(f.Arguments, "redirect") != nil {
			guard = "GoRouter's redirect at " + dr.clause(f) + " runs before every route, and it is code"
		}
		walk(dartArgument(f.Arguments, "routes"), "", dr.clause(f), guard, false)
	}
	// MaterialApp's home and its literal routes.
	for _, f := range dr.calls {
		if f.Name != "MaterialApp" && f.Name != "CupertinoApp" && f.Name != "MaterialApp.router" {
			continue
		}
		if h := dartArgument(f.Arguments, "home"); h != nil && h.Call != nil {
			declared++
			screens = append(screens, &dartScreen{widget: h.Call.Name, route: "/", clause: dr.clause(f)})
		}
		if rs := dartArgument(f.Arguments, "routes"); rs != nil {
			for _, e := range rs.Map {
				declared++
				p, ok := e.Key.stringLiteral()
				if !ok {
					continue
				}
				if w, _ := builtWidget(e.Value); w != "" {
					screens = append(screens, &dartScreen{widget: w, route: p, clause: dr.clause(f)})
				}
			}
		}
	}
	// What Navigator.push opens with a MaterialPageRoute.
	type edge struct {
		from, to, clause, says string
		route                  string // a literal route, or ""
	}
	var edges []edge
	for _, f := range dr.calls {
		from := dr.ownerWidget(strings.SplitN(f.Within, ".", 2)[0])
		switch f.Name {
		case "push", "pushReplacement":
			if f.Target == nil || f.Target.Name == nil || *f.Target.Name != "Navigator" {
				if a := dartPositional(f.Arguments, 0); a != nil && a.String != nil {
					edges = append(edges, edge{from: from, route: *a.String, clause: dr.clause(f), says: fmt.Sprintf("%s(%q) opens %s.", f.Name, *a.String, *a.String)})
				}
				continue
			}
			r := dartPositional(f.Arguments, 1)
			if r == nil || r.Call == nil {
				continue
			}
			w, _ := builtWidget(dartArgument(r.Call.Arguments, "builder"))
			if w == "" {
				dr.question("should", fmt.Sprintf("%s opens a screen with Navigator.%s whose route's builder does not return one widget. Which screen does it open?", dr.clause(f), f.Name),
					[]string{"pages"}, "A destination chosen at run time is not known by syntax.", dr.at(dr.clause(f), "Opens a screen."))
				continue
			}
			edges = append(edges, edge{from: from, to: w, clause: dr.clause(f), says: fmt.Sprintf("Navigator.%s opens %s.", f.Name, w)})
		case "go", "pushNamed", "goNamed":
			if a := dartPositional(f.Arguments, 0); a != nil && a.String != nil {
				edges = append(edges, edge{from: from, route: *a.String, clause: dr.clause(f), says: fmt.Sprintf("%s(%q) opens %s.", f.Name, *a.String, *a.String)})
			} else if a != nil {
				dr.question("should", fmt.Sprintf("%s calls %s with %s, which is not a literal. Which screen does it open?", dr.clause(f), f.Name, a.describe()),
					[]string{"pages"}, "A destination chosen at run time is not known by syntax.", dr.at(dr.clause(f), "Navigates."))
			}
		}
	}
	if declared == 0 && len(edges) == 0 {
		return
	}
	dr.res.say("screens: counted %s and %s: every GoRoute of a GoRouter, MaterialApp's home and routes, and every push, go and Navigator.push", plural(declared, "route"), plural(len(edges), "navigation call"))
	byWidget := map[string]*dartScreen{}
	byRoute := map[string]*dartScreen{}
	var held []*dartScreen
	add := func(s *dartScreen) {
		if dr.classes[s.widget] == nil {
			dr.question("must", fmt.Sprintf("The route %s at %s shows %s, which is not a class of the files read. Which screen is it?", s.route, s.clause, s.widget),
				[]string{"pages"}, "A screen is read from its widget's own class, and only tracked files are read.", dr.at(s.clause, "Shows "+s.widget+"."))
			return
		}
		if s.route != "" {
			route, why := dartRoutePath(splitSegments(s.route))
			if why != "" {
				dr.gap(s.clause, []string{"pages"}, "", "the route %s: %s; left out", s.route, why)
				return
			}
			if prev := byRoute[route]; prev != nil {
				dr.question("must", fmt.Sprintf("The route %s is declared at %s and at %s. Which screen does the app show there?", route, prev.clause, s.clause),
					[]string{"pages"}, "A router shows one screen for a path.", dr.at(s.clause, "Declares "+s.route+" again."))
				return
			}
			s.route = route
			byRoute[route] = s
		}
		s.page = widgetPage(s.widget)
		if prev := byWidget[s.widget]; prev != nil {
			s.page += "-" + strings.Trim(kebab(strings.NewReplacer("/", "-", "{", "by-", "}", "").Replace(s.route)), "-")
		}
		if !pageNameWord.MatchString(s.page) {
			dr.gap(s.clause, []string{"pages"}, "", "the screen %s: no page name can be made from it; left out", s.widget)
			return
		}
		if _, dup := byWidget[s.widget]; !dup {
			byWidget[s.widget] = s
		}
		held = append(held, s)
	}
	for _, s := range screens {
		add(s)
	}
	for _, e := range edges {
		if e.to != "" && byWidget[e.to] == nil && dr.classes[e.to] != nil {
			add(&dartScreen{widget: e.to, clause: e.clause})
		}
	}
	if len(held) == 0 {
		return
	}
	sort.SliceStable(held, func(i, j int) bool { return held[i].page < held[j].page })
	dr.pages = mapping()
	byPage := map[string]*dartScreen{}
	for _, s := range held {
		byPage[s.page] = s
	}
	routeMatch := func(link string) *dartScreen {
		for _, s := range held {
			if s.route == link {
				return s
			}
		}
		for _, s := range held {
			if s.route != "" && (routeMatches(s.route, link) || routeMatches(s.route, strings.NewReplacer(":", "").Replace(link))) {
				return s
			}
		}
		for _, s := range held {
			if s.route == "" {
				continue
			}
		}
		return nil
	}
	for _, s := range held {
		at := "#/pages/" + s.page
		p := mapping()
		if s.route != "" {
			set(p, "route", s.route)
		}
		cites := []*yaml.Node{dr.at(dr.clause(dr.classes[s.widget]), "class "+s.widget+".")}
		if s.route != "" {
			cites = append([]*yaml.Node{dr.at(s.clause, fmt.Sprintf("The route %s shows %s.", s.route, s.widget))}, cites...)
		}
		title, fields, fcites := dr.screenContent(s.widget, at)
		if title != "" {
			set(p, "title", title)
		}
		if len(fields) > 0 {
			set(p, "fields", fields)
		}
		cites = append(cites, fcites...)
		var actions []*yaml.Node
		var labels []string
		for _, e := range edges {
			if e.from != s.widget {
				continue
			}
			var target *dartScreen
			if e.to != "" {
				target = byWidget[e.to]
			} else if strings.HasPrefix(e.route, "/") {
				target = routeMatch(e.route)
			} else {
				for _, o := range held {
					if o.route != "" && strings.HasSuffix(o.route, "/"+e.route) {
						target = o
					}
				}
			}
			if target == nil {
				dr.question("should", fmt.Sprintf("%s opens %s, which is not a screen the routes read serve. Which screen does it open?", e.clause, e.route+e.to),
					[]string{at + "/actions"}, "A navigate action's target is a page, and no screen read matches it.", dr.at(e.clause, e.says))
				continue
			}
			labels = append(labels, fmt.Sprintf("%s/actions/%d/label", at, len(actions)))
			actions = append(actions, flow(mapping("kind", "navigate", "target", target.page)))
			cites = append(cites, dr.at(e.clause, e.says))
		}
		if len(actions) > 0 {
			set(p, "actions", actions)
		}
		set(p, "origin", "stated")
		set(p, "cites", cites)
		set(dr.pages, s.page, p)
		asks := []string{"kind"}
		blocks := []string{at + "/kind"}
		if s.route == "" {
			asks = append(asks, "route")
			blocks = append(blocks, at+"/route")
		}
		if title == "" {
			asks = append(asks, "title")
			blocks = append(blocks, at+"/title")
		}
		asks = append(asks, "the entity it shows", "the operation it reads", "the operation it submits to", "the columns it lists")
		blocks = append(blocks, at+"/entity", at+"/source", at+"/submit", at+"/columns")
		why := "A Flutter widget says what it draws, not which of the four kinds of page it is or the operations behind it."
		if s.route == "" {
			why += " Navigator opens it with no route of its own."
		}
		dr.question("must", fmt.Sprintf("For the screen %s, what are its %s?", s.widget, joinAnd(asks)), blocks, why, dr.at(dr.clause(dr.classes[s.widget]), "class "+s.widget+"."))
		if len(labels) > 0 {
			dr.question("must", fmt.Sprintf("What does a person tap on the screen %s to follow each of its ways to another screen?", s.widget),
				labels, "A navigate action has a label, and a navigation call gives none.", dr.at(dr.clause(dr.classes[s.widget]), "class "+s.widget+"."))
		}
		text := fmt.Sprintf("Which permission does the screen %s check, or is it open to everyone (public)?", s.widget)
		if s.guard != "" {
			text = fmt.Sprintf("%s; which permission does the screen %s check, or is it open to everyone (public)?", upperFirst(s.guard), s.widget)
		}
		dr.question("must", text, []string{at + "/permission"}, "An app's screen checks nothing for the server, and a guard written as code says nothing by syntax, so it is asked, never assumed.")
	}
	if len(menus) > 0 {
		dr.menus = mapping()
		for _, m := range menus {
			s := byWidget[m.page]
			if s == nil {
				continue
			}
			key := camel(strings.ReplaceAll(s.page, "-", "_"))
			set(dr.menus, key, mapping("page", s.page))
			dr.cite(m.clause)
			dr.question("must", fmt.Sprintf("What is the title of the menu entry of %s, a branch of StatefulShellRoute at %s?", s.widget, m.clause),
				[]string{"#/menus/" + key + "/title"}, "A menu entry has a title, and a branch gives none by syntax.", dr.at(m.clause, "A branch shows "+s.widget+"."))
		}
		if count(dr.menus) == 0 {
			dr.menus = nil
		}
	}
	_ = byPage
}

// screenContent reads a screen's title from its AppBar's literal Text,
// and its fields from the form fields of its class and its State.
func (dr *dartReader) screenContent(widget, at string) (string, []string, []*yaml.Node) {
	var titles []string
	var fields []string
	var cites []*yaml.Node
	for _, f := range dr.calls {
		class := strings.SplitN(f.Within, ".", 2)[0]
		if class == "" || dr.ownerWidget(class) != widget {
			continue
		}
		clause := dr.clause(f)
		switch {
		case f.Name == "AppBar":
			t := dartArgument(f.Arguments, "title")
			if t != nil && t.Call != nil && t.Call.Name == "Text" {
				if s, ok := dartPositional(t.Call.Arguments, 0).stringLiteral(); ok && !contains(titles, s) {
					titles = append(titles, s)
					cites = append(cites, dr.at(clause, fmt.Sprintf("Its AppBar's title is %q.", s)))
				}
			}
		case f.Name == "TextFormField" || f.Name == "DropdownButtonFormField" || f.Name == "CheckboxListTile" || strings.HasPrefix(f.Name, "FormBuilder"):
			name := ""
			if v := dartArgument(f.Arguments, "name"); v != nil {
				name, _ = v.stringLiteral()
			}
			if name == "" {
				for _, k := range []string{"onSaved", "onChanged"} {
					if v := dartArgument(f.Arguments, k); v != nil && v.Function != nil {
						for _, r := range v.Function.Returns {
							if r.Assign != nil {
								parts := strings.Split(r.Assign.Target, ".")
								name = parts[len(parts)-1]
							}
						}
					}
				}
			}
			if name == "" {
				what := "no onSaved that assigns a property and no name"
				if dartArgument(f.Arguments, "controller") != nil {
					what = "a controller, which names no property of a model"
				}
				dr.question("should", fmt.Sprintf("%s has a %s with %s. Which field of the screen %s is it?", clause, f.Name, what, widget),
					[]string{at + "/fields"}, "A field is read from the property a form field saves to, or its name.", dr.at(clause, "Declares a "+f.Name+"."))
				continue
			}
			if !memberNameWord.MatchString(name) {
				dr.gap(clause, []string{at + "/fields"}, "", "the field %s is not a camelCase name a field can take; left out", name)
				continue
			}
			if !contains(fields, name) {
				fields = append(fields, name)
				cites = append(cites, dr.at(clause, fmt.Sprintf("A %s saves the field %s.", f.Name, name)))
			}
			if v := dartArgument(f.Arguments, "validator"); v != nil {
				body := ""
				if v.Function != nil {
					for _, r := range v.Function.Returns {
						body += r.describe() + " "
					}
				}
				vt := v.describe()
				if strings.TrimSpace(body) != "" {
					vt = strings.TrimSpace(body)
				}
				switch {
				case strings.Contains(body, "isEmpty") || strings.Contains(vt, "required"):
					dr.gap(clause, []string{at + "/fields"}, "", "the field %s must not be empty, as its validator checks; a page's field holds no rule, which its entity's field does", name)
				case strings.Contains(body, "length"):
					dr.gap(clause, []string{at + "/fields"}, "", "the field %s has its length checked by its validator; a page's field holds no rule, which its entity's field does", name)
				default:
					dr.question("should", fmt.Sprintf("The field %s of the screen %s at %s is checked by a validator the reader does not read: %s. What does it allow?", name, widget, clause, vt),
						[]string{at + "/fields"}, "A rule written as code is not one a field says by syntax.", dr.at(clause, "Checks "+name+"."))
				}
			}
		}
	}
	title := ""
	if len(titles) == 1 {
		title = titles[0]
	}
	return title, fields, cites
}

// dartRoute is one route a server registers.
type dartRoute struct {
	method, path, clause, says string
	handler                    string // the handler's function, "" for a closure
	handlerAt                  string
	permission, unguarded      string
	params                     []string
	id, at                     string
}

// readServer reads shelf_router's routes and dart_frog's routes/ folder.
func (dr *dartReader) readServer() {
	var routes []*dartRoute
	methods := map[string]string{"get": "GET", "post": "POST", "put": "PUT", "patch": "PATCH", "delete": "DELETE", "head": "HEAD", "options": "OPTIONS", "all": "ALL"}
	// shelf_router: routers by the variable they are given to.
	routers := map[string]string{} // variable -> file
	for _, v := range dr.variables {
		if v.Value != nil && v.Value.Call != nil && v.Value.Call.Name == "Router" && v.Value.Call.Target == nil && dr.imports(v.File, "shelf_router") {
			routers[v.Name] = v.File
		}
		if v.Value != nil && v.Value.Text != nil && strings.HasPrefix(*v.Value.Text, "Router()") && dr.imports(v.File, "shelf_router") {
			routers[v.Name] = v.File
		}
	}
	routerOf := func(f *dartFact) string {
		if f.CascadeOf != "" && routers[f.CascadeOf] != "" {
			return f.CascadeOf
		}
		if f.Target != nil && f.Target.Name != nil && routers[*f.Target.Name] != "" {
			return *f.Target.Name
		}
		return ""
	}
	type mount struct{ prefix, parent string }
	prefix := map[string][]mount{} // a router -> where it is mounted
	for _, f := range dr.calls {
		if f.Name != "mount" {
			continue
		}
		parent := routerOf(f)
		p, ok := dartPositional(f.Arguments, 0).stringLiteral()
		h := dartPositional(f.Arguments, 1)
		if parent == "" || !ok || h == nil || h.Name == nil {
			continue
		}
		child := strings.TrimSuffix(*h.Name, ".call")
		if routers[child] != "" {
			prefix[child] = append(prefix[child], mount{p, parent})
		}
	}
	mountsOf := func(r string, depth int) []string {
		var out []string
		var rec func(r string, base string, d int)
		rec = func(r, base string, d int) {
			ps := prefix[r]
			if len(ps) == 0 || d > 8 {
				out = append(out, base)
				return
			}
			for _, m := range ps {
				rec(m.parent, strings.TrimSuffix(m.prefix, "/")+base, d+1)
			}
		}
		rec(r, "", depth)
		return out
	}
	registrations := 0
	for _, f := range dr.calls {
		method, ok := methods[f.Name]
		if !ok || !dr.imports(f.File, "shelf_router") {
			continue
		}
		r := routerOf(f)
		raw := dartPositional(f.Arguments, 0)
		handler := dartPositional(f.Arguments, 1)
		if r == "" || raw == nil || handler == nil {
			continue
		}
		registrations++
		clause := dr.clause(f)
		if f.InLoop || f.InCondition {
			where := "in a loop"
			if !f.InLoop {
				where = "behind a condition"
			}
			dr.question("must", fmt.Sprintf("%s registers a route %s, so the reader cannot tell how many routes it registers, or whether it registers one. Which routes does the running system register here?", clause, where),
				[]string{"paths"}, "Syntax does not run the code; only the running system's own list of routes says what a loop or a condition registers.", dr.at(clause, "Registers a route "+where+"."))
			continue
		}
		p, ok := raw.stringLiteral()
		if !ok {
			dr.question("must", fmt.Sprintf("%s registers a route whose path is %s, which is not a literal. Which path does the running system serve here?", clause, raw.describe()),
				[]string{"paths"}, "A path computed at run time is not known by syntax.", dr.at(clause, "Registers a route with a path that is not a literal."))
			continue
		}
		for _, base := range mountsOf(r, 0) {
			rt := &dartRoute{method: method, clause: clause, says: fmt.Sprintf("Registers %s %s on %s.", method, base+p, r)}
			full, why := dartRoutePath(splitSegments(base + "/" + p))
			if why != "" {
				dr.gap(clause, []string{"paths"}, "", "the route %s %s: %s; left out", method, base+p, why)
				continue
			}
			rt.path = full
			rt.params, _ = pathParameters(full)
			dr.shelfPermission(rt, f, handler)
			routes = append(routes, rt)
		}
	}
	if registrations > 0 {
		dr.res.say("routes: counted %s: every call of get, post, put, patch, delete, head, options or all with a path and a handler on a shelf_router Router", plural(registrations, "registration"))
	}
	routes = append(routes, dr.readDartFrog()...)
	dr.writeRoutes(routes)
}

// shelfPermission reads a shelf route's permission: a check that wraps its
// handler, or one the handler calls.
func (dr *dartReader) shelfPermission(rt *dartRoute, f *dartFact, handler *dartValue) {
	var found []string
	var at string
	use := func(c *permissionCheck, args []dartArg, clause string) {
		v := dartPositional(args, c.argument-1)
		s, ok := v.stringLiteral()
		if !ok || !permissionWord.MatchString(s) {
			found = append(found, "")
			return
		}
		found = append(found, s)
		if at == "" {
			at = clause
		}
	}
	h := handler
	if h.Call != nil {
		if c := dr.checkOf(h.Call.Name, f.File); c != nil {
			use(c, h.Call.Arguments, dr.clause(f))
		}
		for i := range h.Call.Arguments {
			a := &h.Call.Arguments[i].Value
			if a.Name != nil || a.Function != nil {
				h = a
			}
		}
	}
	switch {
	case h.Name != nil:
		rt.handler = *h.Name
		if fn := dr.functions[*h.Name]; fn != nil {
			rt.handlerAt = dr.clause(fn)
			for _, c := range dr.calls {
				if c.File == fn.File && strings.SplitN(c.Within, ".", 2)[0] == fn.Name {
					if chk := dr.checkOf(c.Name, c.File); chk != nil && c.Target == nil {
						use(chk, c.Arguments, dr.clause(c))
					}
				}
			}
		}
	case h.Function != nil:
		for _, c := range dr.calls {
			if c.File == f.File && c.Function == h.Function.Line {
				if chk := dr.checkOf(c.Name, c.File); chk != nil && c.Target == nil {
					use(chk, c.Arguments, dr.clause(c))
				}
			}
		}
	}
	dr.settlePermission(rt, found, at)
}

func (dr *dartReader) settlePermission(rt *dartRoute, found []string, at string) {
	label := rt.method + " " + rt.path
	switch {
	case len(found) == 1 && found[0] != "":
		rt.permission = found[0]
		rt.says += fmt.Sprintf(" Its check at %s names %s.", at, found[0])
		dr.cite(at)
	case len(found) > 1:
		rt.unguarded = fmt.Sprintf("%s is guarded by %d checks the implementation file names, and an operation checks one permission.", label, len(found))
	case len(found) == 1:
		rt.unguarded = fmt.Sprintf("%s is guarded by a check the implementation file names whose permission is not a literal permission name.", label)
	case dr.checkFile == "":
		rt.unguarded = fmt.Sprintf("%s checks no permission the reader knows, since no implementation file names the project's checks.", label)
	default:
		rt.unguarded = fmt.Sprintf("%s checks no permission through a check the implementation file names.", label)
	}
}

// readDartFrog reads dart_frog's routes/ folder: each file's onRequest is
// a route at the file's place, its methods the HttpMethod its request is
// compared with, and _middleware.dart the check before the folder.
func (dr *dartReader) readDartFrog() []*dartRoute {
	var out []*dartRoute
	type frogFile struct{ file, rel string }
	var files []frogFile
	for _, p := range dr.order {
		rel := dr.fromFolder(p)
		i := strings.Index("/"+rel, "/routes/")
		if i < 0 || !dr.imports(p, "dart_frog") {
			continue
		}
		files = append(files, frogFile{p, ("/" + rel)[i+len("/routes/"):]})
	}
	if len(files) == 0 {
		return nil
	}
	middleware := map[string][]string{} // folder -> the permissions its middleware names, "" for none
	mwAt := map[string]string{}
	for _, f := range files {
		if !strings.HasSuffix(f.rel, "_middleware.dart") {
			continue
		}
		dir := strings.TrimSuffix(strings.TrimSuffix(f.rel, "_middleware.dart"), "/")
		mwAt[dir] = f.file
		for _, c := range dr.calls {
			if c.File == f.file {
				if chk := dr.checkOf(c.Name, c.File); chk != nil && c.Target == nil {
					s, _ := dartPositional(c.Arguments, chk.argument-1).stringLiteral()
					middleware[dir] = append(middleware[dir], s)
					mwAt[dir] = dr.clause(c)
				}
			}
		}
	}
	n := 0
	for _, f := range files {
		if strings.HasSuffix(f.rel, "_middleware.dart") {
			continue
		}
		fn := dr.functions["onRequest"]
		var handler *dartFact
		for _, m := range dr.dump.Facts {
			if m.Kind == "method" && m.File == f.file && m.Within == "" && m.Name == "onRequest" {
				handler = m
			}
		}
		_ = fn
		if handler == nil {
			continue
		}
		n++
		path, ok := dartFrogPath(f.rel)
		clause := dr.clause(handler)
		if !ok {
			dr.gap(clause, []string{"paths"}, "", "the dart_frog route %s: its place gives no path a path template holds; left out", f.rel)
			continue
		}
		var ms []string
		others := ""
		for _, c := range dr.compares {
			if c.File != f.file || !strings.HasSuffix(c.On, "request.method") {
				continue
			}
			if c.Default {
				others = fmt.Sprintf("its switch on the method at %s has a default branch", dr.clause(c))
				continue
			}
			v := ""
			if c.Value != nil && c.Value.Name != nil {
				v = *c.Value.Name
			} else if c.Value != nil && c.Value.Text != nil {
				v = *c.Value.Text
			}
			if strings.HasPrefix(v, "HttpMethod.") {
				m := strings.ToUpper(strings.TrimPrefix(v, "HttpMethod."))
				if !contains(ms, m) {
					ms = append(ms, m)
				}
			}
		}
		if len(ms) == 0 {
			dr.question("must", fmt.Sprintf("Which methods does %s serve at %s, and for each, what does it do, what does it answer and which permission does it check? Its onRequest compares the request's method with no HttpMethod.", f.file, path),
				[]string{"paths"}, "dart_frog serves every method at a route file's place, and the handler decides which it answers.", dr.at(clause, "onRequest serves "+path+"."))
			continue
		}
		if others != "" {
			dr.question("must", fmt.Sprintf("The handler of %s at %s serves %s, and %s. Which other methods does it serve?", path, clause, joinAnd(ms), others),
				[]string{"paths"}, "A handler serves every method a request comes with, and the reader reads the ones it names.", dr.at(clause, "onRequest serves "+path+"."))
		}
		// The checks of onRequest and of the middleware of the folders above.
		type use struct{ permission, at, branch string }
		var uses []use
		for _, c := range dr.calls {
			if c.File == f.file && strings.HasPrefix(c.Within, "onRequest") {
				if chk := dr.checkOf(c.Name, c.File); chk != nil && c.Target == nil {
					s, _ := dartPositional(c.Arguments, chk.argument-1).stringLiteral()
					uses = append(uses, use{s, dr.clause(c), c.Branch})
				}
			}
		}
		var found []string
		at := ""
		mwUnread := ""
		for dir, file := range mwAt {
			if dir == "" || strings.HasPrefix(f.rel, dir+"/") {
				if perms, ok := middleware[dir]; ok {
					found = append(found, perms...)
					if at == "" {
						at = mwAt[dir]
					}
				} else {
					mwUnread = file
				}
			}
		}
		for _, m := range ms {
			rt := &dartRoute{method: m, path: path, clause: clause, says: fmt.Sprintf("dart_frog serves %s %s from %s.", m, path, f.file)}
			rt.params, _ = pathParameters(path)
			mine := append([]string{}, found...)
			mineAt := at
			for _, u := range uses {
				if u.branch == "" || strings.EqualFold(u.branch, "HttpMethod."+strings.ToLower(m)) {
					mine = append(mine, u.permission)
					if mineAt == "" {
						mineAt = u.at
					}
				}
			}
			dr.settlePermission(rt, mine, mineAt)
			if mwUnread != "" && rt.permission == "" {
				rt.unguarded = fmt.Sprintf("%s runs the middleware %s, which calls no check the implementation file names.", m+" "+path, mwUnread)
				dr.cite(mwUnread)
			}
			out = append(out, rt)
		}
	}
	if n > 0 {
		dr.res.say("routes: counted %s under routes/: every dart_frog file with an onRequest", plural(n, "route file"))
	}
	return out
}

// writeRoutes writes the routes as operations with their questions.
func (dr *dartReader) writeRoutes(routes []*dartRoute) {
	seen := map[string]*dartRoute{}
	var held []*dartRoute
	for _, rt := range routes {
		pair := rt.method + " " + rt.path
		if prev := seen[pair]; prev != nil {
			dr.question("must", fmt.Sprintf("%s is registered at %s and at %s. Which handler does the running system run?", pair, prev.clause, rt.clause),
				[]string{"#/paths/" + escapeToken(rt.path) + "/" + strings.ToLower(rt.method)}, "A router serves a method and path pair with one handler.", dr.at(rt.clause, "Registers "+pair+" again."))
			continue
		}
		seen[pair] = rt
		if !heldMethods[rt.method] {
			dr.gap(rt.clause, []string{"paths"}, "", "the route %s: the method %s has no operation in the meta-model, which holds GET, POST, PUT, PATCH and DELETE; left out", pair, rt.method)
			continue
		}
		held = append(held, rt)
	}
	if len(held) == 0 {
		return
	}
	uses := map[string]int{}
	for _, rt := range held {
		if rt.handler != "" {
			uses[rt.handler]++
		}
	}
	for _, rt := range held {
		key := strings.ToLower(rt.method)
		name := strings.TrimPrefix(rt.handler, "_")
		if name != "" && uses[rt.handler] == 1 && handlerName.MatchString(name) {
			rt.id = strings.ToLower(name[:1]) + name[1:]
		} else {
			rt.id = methodPathName(key, rt.path)
		}
		rt.at = "#/paths/" + escapeToken(rt.path) + "/" + key
	}
	sort.SliceStable(held, func(i, j int) bool {
		if held[i].path != held[j].path {
			return held[i].path < held[j].path
		}
		return methodRank(strings.ToLower(held[i].method)) < methodRank(strings.ToLower(held[j].method))
	})
	dr.paths = mapping()
	usedBy := map[string][]*dartRoute{}
	var last string
	var item *yaml.Node
	for _, rt := range held {
		label := rt.method + " " + rt.path
		if rt.path != last {
			last = rt.path
			item = mapping()
			set(dr.paths, rt.path, item)
			at := "#/paths/" + escapeToken(rt.path)
			if len(rt.params) > 0 {
				var list []*yaml.Node
				var blocks []string
				for i, n := range rt.params {
					list = append(list, flow(mapping("name", n, "in", "path", "required", true)))
					blocks = append(blocks, fmt.Sprintf("%s/parameters/%d/schema", at, i))
				}
				set(item, "parameters", list)
				dr.question("must", fmt.Sprintf("What values does each path parameter of %s take: %s?", rt.path, strings.Join(rt.params, ", ")),
					blocks, "A route names a path's parameters and not the values they take.")
			}
		}
		op := mapping("operationId", rt.id)
		if rt.permission != "" {
			set(op, "permission", rt.permission)
			usedBy[rt.permission] = append(usedBy[rt.permission], rt)
		}
		cites := []*yaml.Node{dr.at(rt.clause, rt.says)}
		if rt.handlerAt != "" {
			cites = append(cites, dr.at(rt.handlerAt, "The handler "+rt.handler+"."))
		}
		set(op, "origin", "stated")
		set(op, "cites", cites)
		set(item, strings.ToLower(rt.method), op)
		dr.counts["operations"]++
		dr.question("must", fmt.Sprintf("What does %s do, and what does it answer?", label), []string{rt.at + "/summary", rt.at + "/responses"},
			"A registration names the handler and not what it does or the responses it gives.")
		if rt.permission == "" {
			dr.question("must", rt.unguarded+" Is it meant to be open to everyone (public), or which permission should it check?",
				[]string{rt.at + "/permission"}, "An operation open to everyone is how an open endpoint is usually found, so it is asked, never assumed.")
		}
		dr.question("must", fmt.Sprintf("Does the running system register %s? It is declared at %s, and no list the running system printed was read with it.", label, rt.clause),
			[]string{rt.at}, "Syntax shows a declaration and not what runs; a route table the running system prints says what it registers, and merging it answers this.", dr.at(rt.clause, "Registers "+label+"."))
	}
	if len(usedBy) > 0 {
		var names []string
		for n := range usedBy {
			names = append(names, n)
		}
		sort.Strings(names)
		dr.permissions = mapping()
		var blocks, grants []string
		for _, n := range names {
			var says []string
			for _, rt := range usedBy[n] {
				says = append(says, rt.method+" "+rt.path)
			}
			set(dr.permissions, n, mapping("origin", "stated", "cites", []*yaml.Node{dr.at(usedBy[n][0].clause, fmt.Sprintf("%s %s %s.", joinAnd(says), checkOrChecks(len(says)), n))}))
			blocks = append(blocks, "#/permissions/"+escapeToken(n)+"/description")
			grants = append(grants, "#/permissions/"+escapeToken(n))
		}
		dr.question("must", fmt.Sprintf("What does each permission allow: %s?", strings.Join(names, ", ")), blocks, "The source names the permission a route checks and not what it is for.")
		dr.question("must", fmt.Sprintf("Which role grants each permission: %s?", strings.Join(names, ", ")), grants, "The source names the permission a route checks and not who holds it.")
	}
}

// dartClient is one call to another system.
type dartClient struct{ clause, method, url, dependency, says string }

// dartURL is the URL a client call is given: a literal, Uri.parse of
// one, Uri.https(host, path), or an interpolation of one value per
// segment.
func dartURL(v *dartValue) (string, []string, bool) {
	if v == nil {
		return "", nil, false
	}
	if v.Call != nil && v.Call.Target != nil && v.Call.Target.Name != nil && *v.Call.Target.Name == "Uri" {
		switch v.Call.Name {
		case "parse", "tryParse":
			return dartURL(dartPositional(v.Call.Arguments, 0))
		case "https", "http":
			host, ok1 := dartPositional(v.Call.Arguments, 0).stringLiteral()
			p, params, ok2 := dartURL(dartPositional(v.Call.Arguments, 1))
			if !ok1 || !ok2 {
				return "", nil, false
			}
			return v.Call.Name + "://" + host + "/" + strings.TrimPrefix(p, "/"), params, true
		}
	}
	if v.Call != nil && v.Call.Name == "Uri.https" {
		return "", nil, false
	}
	if s, ok := v.stringLiteral(); ok {
		return s, nil, true
	}
	if v.Interpolated == nil {
		return "", nil, false
	}
	var b strings.Builder
	var params []string
	for i, p := range v.Interpolated {
		if p.Text != nil {
			b.WriteString(*p.Text)
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(*p.Expression, "{"), "}")
		if j := strings.LastIndex(name, "."); j >= 0 {
			name = name[j+1:]
		}
		before := b.String()
		next := ""
		if i+1 < len(v.Interpolated) && v.Interpolated[i+1].Text != nil {
			next = *v.Interpolated[i+1].Text
		}
		if !memberNameWord.MatchString(name) || !strings.HasSuffix(before, "/") || (next != "" && !strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "?")) {
			return "", nil, false
		}
		params = append(params, name)
		b.WriteString("{" + name + "}")
	}
	return b.String(), params, true
}

// readClients reads retrofit's annotated APIs and dio's and http's calls.
func (dr *dartReader) readClients() {
	var clients []dartClient
	n := 0
	add := func(clause, method, how string, u *dartValue, base string) {
		n++
		text, params, ok := dartURL(u)
		if !ok {
			dr.question("should", fmt.Sprintf("%s calls another system with %s at %s, built from parts the reader cannot place. Which system and operation does it call?", clause, how, u.describe()),
				[]string{"dependencies"}, "A URL is read when its host is a literal and each value fills one whole path segment.", dr.at(clause, "Calls "+how+"."))
			return
		}
		if base != "" {
			text = strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(text, "/")
		}
		parsed, err := url.Parse(strings.NewReplacer("{", "", "}", "").Replace(text))
		if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
			dr.question("should", fmt.Sprintf("%s calls %s %s, which names no host. Which system does it call, and is it an operation of this system?", clause, method, text),
				[]string{"dependencies"}, "A dependency is named by the system it calls, and the URL does not name one.", dr.at(clause, fmt.Sprintf("Calls %s %s.", method, text)))
			return
		}
		dep := camel(strings.ToLower(hostWord.ReplaceAllString(parsed.Hostname(), "_")))
		says := fmt.Sprintf("Calls %s %s with %s.", method, text, how)
		clients = append(clients, dartClient{clause, method, text, dep, says})
		if len(params) > 0 {
			var holes []string
			for _, p := range params {
				holes = append(holes, "{"+p+"}")
			}
			dr.question("should", fmt.Sprintf("%s calls %s %s, filling %s from values of the code. What does each hold?", clause, method, text, joinAnd(holes)),
				[]string{"#/dependencies/" + dep}, "A part of a URL interpolated from a value is a parameter, and the code says only which variable fills it.", dr.at(clause, says))
		}
	}
	verbs := map[string]string{"GET": "GET", "POST": "POST", "PUT": "PUT", "PATCH": "PATCH", "DELETE": "DELETE"}
	for _, c := range dr.classes {
		a := c.annotation("RestApi")
		if a == nil {
			continue
		}
		base := ""
		if v := dartArgument(a.Arguments, "baseUrl"); v != nil {
			base, _ = v.stringLiteral()
		}
		for _, m := range dr.members[c.Name] {
			if m.Kind != "method" {
				continue
			}
			for _, an := range m.Annotations {
				if method, ok := verbs[an.Name]; ok && len(an.Arguments) > 0 {
					p := an.Arguments[0].Value
					add(dr.clause(m), method, "retrofit's @"+an.Name, &p, base)
				}
			}
		}
	}
	for _, f := range dr.calls {
		method, ok := map[string]string{"get": "GET", "post": "POST", "put": "PUT", "patch": "PATCH", "delete": "DELETE"}[f.Name]
		if !ok || f.Target == nil || dr.imports(f.File, "shelf_router") {
			continue
		}
		how := ""
		switch {
		case dr.imports(f.File, "http") && f.Target.Name != nil && *f.Target.Name == "http":
			how = "http." + f.Name
		case dr.imports(f.File, "dio"):
			how = "dio's " + f.Name
		default:
			continue
		}
		add(dr.clause(f), method, how, dartPositional(f.Arguments, 0), "")
	}
	if n > 0 {
		dr.res.say("clients: counted %s: every method of a retrofit @RestApi, and every get, post, put, patch and delete of http and dio", plural(n, "call"))
	}
	if len(clients) == 0 {
		return
	}
	byName := map[string][]dartClient{}
	var names []string
	for _, c := range clients {
		if byName[c.dependency] == nil {
			names = append(names, c.dependency)
		}
		byName[c.dependency] = append(byName[c.dependency], c)
	}
	sort.Strings(names)
	dr.deps = mapping()
	for _, name := range names {
		var cites []*yaml.Node
		for _, c := range byName[name] {
			cites = append(cites, dr.at(c.clause, c.says))
		}
		at := "#/dependencies/" + name
		set(dr.deps, name, mapping("origin", "stated", "cites", cites))
		dr.question("must", fmt.Sprintf("What is the system %s the source calls, and how long may one call to it take before the caller gives up?", name),
			[]string{at + "/description", at + "/timeout"}, "The source names the URL it calls, and not what the system is or a time limit, which every dependency has.")
	}
}

// readSettings reads String, int and bool.fromEnvironment and
// Platform.environment by a literal name.
func (dr *dartReader) readSettings() {
	settings := map[string][]jsSetting{}
	n := 0
	for _, f := range dr.calls {
		kind := ""
		switch {
		case f.Name == "fromEnvironment" && f.Target != nil && f.Target.Name != nil:
			kind = *f.Target.Name
		case strings.HasSuffix(f.Name, ".fromEnvironment"):
			kind = strings.TrimSuffix(f.Name, ".fromEnvironment")
		default:
			continue
		}
		clause := dr.clause(f)
		n++
		key, ok := dartPositional(f.Arguments, 0).stringLiteral()
		if !ok {
			dr.question("should", fmt.Sprintf("%s reads %s.fromEnvironment by a name that is not a literal. Which setting does it read?", clause, kind),
				[]string{"configuration"}, "A name computed at build time is not known by syntax.", dr.at(clause, "Reads a setting by a computed name."))
			continue
		}
		s := jsSetting{clause: clause, says: fmt.Sprintf("Reads %s.fromEnvironment(%q), given with --dart-define.", kind, key)}
		switch kind {
		case "String":
			s.kind = "string"
		case "bool":
			s.kind = "bool"
		case "int":
			s.kind = "int"
		}
		if d := dartArgument(f.Arguments, "defaultValue"); d != nil {
			s.value = dartLiteralNode(d)
		}
		settings[key] = append(settings[key], s)
	}
	for _, f := range dr.indexes {
		clause := dr.clause(f)
		n++
		key, ok := f.Key.stringLiteral()
		if !ok {
			dr.question("should", fmt.Sprintf("%s reads %s by a name that is not a literal. Which setting does it read?", clause, f.On),
				[]string{"configuration"}, "A name computed at run time is not known by syntax.", dr.at(clause, "Reads the environment by a computed name."))
			continue
		}
		settings[key] = append(settings[key], jsSetting{clause: clause, kind: "string", says: fmt.Sprintf("Reads %s[%q].", f.On, key)})
	}
	if n > 0 {
		dr.res.say("settings: counted %s: every fromEnvironment and every read of Platform.environment", plural(n, "read"))
	}
	// The settings are written as extract javascript writes them.
	js := &jsReader{key: dr.key, res: dr.res, questions: dr.questions, files: map[string]*jsFile{}}
	js.counter = &dr.nextID
	js.writeSettings(settings)
	dr.notHeld = append(dr.notHeld, js.notHeld...)
	for _, s := range settings {
		for _, r := range s {
			dr.cite(r.clause)
		}
	}
	if js.deployQs != nil {
		if dr.deployQs == nil {
			dr.deployQs = js.deployQs
		} else {
			dr.deployQs.Content = append(dr.deployQs.Content, js.deployQs.Content...)
		}
	}
	dr.config = js.config
}
