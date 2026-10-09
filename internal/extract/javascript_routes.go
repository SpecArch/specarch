package extract

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The methods Express's and Fastify's route calls name.
var jsRouteMethods = map[string]string{"get": "GET", "post": "POST", "put": "PUT", "patch": "PATCH", "delete": "DELETE", "del": "DELETE", "options": "OPTIONS", "head": "HEAD", "all": "ALL"}

// jsMount is a place routes are registered under: the literal segments
// joined so far, the middleware the mounts add, outermost first, and
// where each mount is.
type jsMount struct {
	segments []string
	chain    []jsValue
	at       []string
}

// jsRoute is one route a router registers.
type jsRoute struct {
	fact       *jsFact
	lib        string
	method     string
	path       string
	params     []string
	handler    *jsFact // the handler's function, nil when the reader cannot find it
	name       string  // the handler's name, "" for one written in place
	middleware []jsValue
	opts       *jsValue // Fastify's route options
	permission string
	checks     []string
	mounts     []string // where the router it is registered on is mounted, outermost first
	id, at     string
}

// routerKind is what a variable's value makes: an Express application,
// an Express router or a Fastify instance, or "".
func (js *jsReader) routerKind(vr *jsFact) string {
	if vr == nil || vr.Value == nil || vr.Value.Call == nil {
		return ""
	}
	callee := &vr.Value.Call.Callee
	if callee.Name != nil {
		imported, ok := js.fromLibrary(callee, "express", "fastify")
		if !ok {
			return ""
		}
		m, _ := js.moduleOf(callee)
		switch {
		case jsLibrary(m) == "fastify" && (imported == "default" || imported == "*" || imported == "fastify" || imported == "Fastify"):
			return "fastify"
		case imported == "default" || imported == "*":
			return "app"
		case imported == "Router":
			return "router"
		}
		return ""
	}
	if callee.Member != nil && callee.Member.Name == "Router" {
		if _, ok := js.fromLibrary(&callee.Member.Object, "express"); ok {
			return "router"
		}
	}
	return ""
}

// readRoutes reads the routes Express's and Fastify's routers register
// with literal paths.
func (js *jsReader) readRoutes() {
	express, fastify := false, false
	for _, p := range js.order {
		for m := range js.files[p].imports {
			switch jsLibrary(m) {
			case "express":
				express = true
			case "fastify":
				fastify = true
			}
		}
	}
	if !express && !fastify {
		return
	}
	cache := map[string][]jsMount{}
	why := map[string]string{}
	resolving := map[string]bool{}
	var mountsOf func(v *jsValue) ([]jsMount, string)
	// mountsOfRouter is where an Express router is mounted: under each use
	// of the files read that is given it.
	mountsOfRouter := func(key string, name string) ([]jsMount, string) {
		var mounts []jsMount
		for _, f := range js.calls {
			if f.Callee == nil || f.Callee.Member == nil || f.Callee.Member.Name != "use" {
				continue
			}
			idx := -1
			for i := range f.Arguments {
				if f.Arguments[i].Declaration.key() == key {
					idx = i
				}
			}
			if idx < 0 {
				continue
			}
			var segs []string
			var chain []jsValue
			for i := 0; i < idx; i++ {
				a := f.Arguments[i]
				if i == 0 {
					if s, ok := js.constString(&a); ok {
						segs = splitSegments(s)
						continue
					}
					if a.String == nil && a.Template != nil {
						return nil, "a router mounted at " + js.clause(f) + " under a path that is not a literal"
					}
				}
				chain = append(chain, a)
			}
			base, reason := mountsOf(&f.Callee.Member.Object)
			if reason != "" {
				return nil, reason
			}
			for _, b := range base {
				mounts = append(mounts, jsMount{segments: append(append([]string{}, b.segments...), segs...), chain: append(append([]jsValue{}, b.chain...), chain...), at: append(append([]string{}, b.at...), js.clause(f))})
			}
		}
		if len(mounts) == 0 {
			return nil, "the router " + name + ", which no use() of the files read mounts"
		}
		return mounts, ""
	}
	// pluginMounts is where a Fastify plugin is registered: under each
	// register of the files read that is given its function, with the
	// literal prefix its options give.
	pluginMounts := func(fn *jsFact) ([]jsMount, string) {
		key := (&jsPos{File: fn.File, Line: fn.Line, Column: fn.Column}).key()
		var mounts []jsMount
		for _, f := range js.calls {
			if f.Callee == nil || f.Callee.Member == nil || f.Callee.Member.Name != "register" || len(f.Arguments) == 0 {
				continue
			}
			a := &f.Arguments[0]
			if a.Function.key() != key && a.Declaration.key() != key {
				continue
			}
			var segs []string
			if len(f.Arguments) > 1 {
				if p := f.Arguments[1].prop("prefix"); p != nil {
					s, ok := js.constString(p)
					if !ok {
						return nil, "a plugin registered at " + js.clause(f) + " with a prefix that is not a literal"
					}
					segs = splitSegments(s)
				}
			}
			base, reason := mountsOf(&f.Callee.Member.Object)
			if reason != "" {
				return nil, reason
			}
			for _, b := range base {
				mounts = append(mounts, jsMount{segments: append(append([]string{}, b.segments...), segs...), chain: b.chain, at: append(append([]string{}, b.at...), js.clause(f))})
			}
		}
		if len(mounts) == 0 {
			name := fn.Name
			if name == "" {
				name = "at " + js.clause(fn)
			}
			return nil, "the instance the plugin " + name + " is given, which no register() of the files read registers"
		}
		return mounts, ""
	}
	mountsOf = func(v *jsValue) ([]jsMount, string) {
		switch {
		case v == nil:
			return nil, "nothing"
		case v.Name != nil && v.Parameter != nil:
			fn := js.functions[v.Parameter.Function.key()]
			if fn != nil && v.Parameter.Index == 0 {
				if m, reason := pluginMounts(fn); reason == "" {
					return m, ""
				}
			}
			name := "a function"
			if fn != nil && fn.Name != "" {
				name = "the function " + fn.Name
			}
			return nil, fmt.Sprintf("%s, the router %s is given by a caller the reader does not follow", *v.Name, name)
		case v.Name != nil:
			vr := js.variableOf(v)
			if vr == nil {
				if v.Import != nil && v.Declaration == nil {
					return nil, fmt.Sprintf("%s, imported from %s, which is not a file of the folder read", *v.Name, v.Import.Module)
				}
				return nil, *v.Name + ", a router the reader does not follow"
			}
			key := v.Declaration.key()
			if m, ok := cache[key]; ok {
				return m, why[key]
			}
			if resolving[key] {
				return nil, "a router mounted on itself"
			}
			resolving[key] = true
			var m []jsMount
			var reason string
			switch js.routerKind(vr) {
			case "app", "fastify":
				m = []jsMount{{}}
			case "router":
				m, reason = mountsOfRouter(key, vr.Name)
			default:
				reason = *v.Name + ", a value the reader does not know as a router"
			}
			cache[key], why[key] = m, reason
			return m, reason
		case v.Call != nil && v.Call.Callee.Member != nil && v.Call.Callee.Member.Name == "route" && len(v.Call.Arguments) == 1:
			base, reason := mountsOf(&v.Call.Callee.Member.Object)
			if reason != "" {
				return nil, reason
			}
			s, ok := js.constString(&v.Call.Arguments[0])
			if !ok {
				return nil, "a route() whose path is not a literal"
			}
			var out []jsMount
			for _, b := range base {
				out = append(out, jsMount{segments: append(append([]string{}, b.segments...), splitSegments(s)...), chain: b.chain, at: b.at})
			}
			return out, ""
		}
		return nil, v.describe() + ", which the reader does not follow"
	}

	var routes []*jsRoute
	registrations := 0
	for _, f := range js.calls {
		if f.Callee == nil || f.Callee.Member == nil {
			continue
		}
		name := f.Callee.Member.Name
		object := &f.Callee.Member.Object
		method, isMethod := jsRouteMethods[name]
		fastifyRoute := name == "route" && len(f.Arguments) == 1 && f.Arguments[0].Object != nil && f.Arguments[0].prop("method") != nil
		if !isMethod && !fastifyRoute {
			continue
		}
		// A registration is a call on a router the reader knows, or one
		// with a path and a handler on a value it does not.
		mounts, reason := mountsOf(object)
		if reason != "" {
			last := len(f.Arguments) - 1
			if fastifyRoute || len(f.Arguments) < 2 || f.Arguments[0].String == nil || !strings.HasPrefix(*f.Arguments[0].String, "/") || (f.Arguments[last].Function == nil && js.functionOf(&f.Arguments[last]) == nil) {
				continue
			}
		}
		lib := "express"
		if object.Name != nil {
			if vr := js.variableOf(object); js.routerKind(vr) == "fastify" {
				lib = "fastify"
			}
			if object.Parameter != nil {
				lib = "fastify"
			}
		}
		registrations++
		clause := js.clause(f)
		rt := &jsRoute{fact: f, lib: lib, method: method}
		var pathArg *jsValue
		var rest []jsValue
		if fastifyRoute {
			o := &f.Arguments[0]
			rt.opts = o
			m, ok := js.constString(o.prop("method"))
			if !ok {
				js.question("must", fmt.Sprintf("%s registers a route with route(), by a method that is not a literal. Which method and path does the running system serve here?", clause),
					[]string{"paths"}, "A method computed at run time is not known by syntax.", js.at(clause, "Registers a route with route()."))
				continue
			}
			rt.method = strings.ToUpper(m)
			pathArg = o.prop("url")
			if pathArg == nil {
				pathArg = o.prop("path")
			}
			if h := o.prop("handler"); h != nil {
				rest = []jsValue{*h}
			}
		} else if object.Call != nil && object.Call.Callee.Member != nil && object.Call.Callee.Member.Name == "route" {
			// router.route('/path').get(handler): the path is route()'s.
			empty := ""
			pathArg = &jsValue{String: &empty}
			rest = f.Arguments
		} else {
			if len(f.Arguments) < 2 {
				registrations--
				continue
			}
			pathArg = &f.Arguments[0]
			rest = f.Arguments[1:]
			if lib == "fastify" && len(rest) == 2 && rest[0].Object != nil {
				rt.opts = &rest[0]
				rest = rest[1:]
			}
		}
		if f.InLoop || f.InCondition {
			where := "in a loop"
			if !f.InLoop {
				where = "behind a condition"
			}
			js.question("must", fmt.Sprintf("%s registers a route %s, so the reader cannot tell how many routes it registers, or whether it registers one. Which routes does the running system register here?", clause, where),
				[]string{"paths"}, "Syntax does not run the code; only the running system's own list of routes, such as a route table, says what a loop or a condition registers.", js.at(clause, "Registers a route "+where+"."))
			continue
		}
		raw, ok := js.constString(pathArg)
		if !ok {
			js.question("must", fmt.Sprintf("%s registers a route whose path is %s, which is not a literal. Which path does the running system serve here?", clause, pathArg.describe()),
				[]string{"paths"}, "A path computed at run time is not known by syntax.", js.at(clause, "Registers a route with a path that is not a literal."))
			continue
		}
		segs := splitSegments(raw)
		if reason != "" {
			js.question("must", fmt.Sprintf("%s registers %s %s on %s. Under which path does the running system serve it, and does it?", clause, rt.method, "/"+strings.Join(segs, "/"), reason),
				[]string{"paths"}, "Syntax follows a router from where it is made, and not one handed over in a way it does not read.", js.at(clause, "Registers a route."))
			continue
		}
		if len(rest) == 0 {
			js.question("must", fmt.Sprintf("%s registers %s %s with no handler the reader finds. Which handler serves it?", clause, rt.method, raw),
				[]string{"paths"}, "A route's handler is its last argument, or route()'s handler.", js.at(clause, "Registers a route."))
			continue
		}
		handler := rest[len(rest)-1]
		for _, mw := range rest[:len(rest)-1] {
			if mw.Array != nil {
				rt.middleware = append(rt.middleware, mw.Array...)
			} else {
				rt.middleware = append(rt.middleware, mw)
			}
		}
		if rt.opts != nil {
			for _, k := range []string{"onRequest", "preValidation", "preHandler"} {
				if h := rt.opts.prop(k); h != nil {
					if h.Array != nil {
						rt.middleware = append(rt.middleware, h.Array...)
					} else {
						rt.middleware = append(rt.middleware, *h)
					}
				}
			}
		}
		rt.handler = js.functionOf(&handler)
		if handler.Name != nil {
			rt.name = *handler.Name
		}
		// Middleware a router's use() adds to every route registered
		// after it.
		var used []jsValue
		router := object
		if object.Call != nil && object.Call.Callee.Member != nil && object.Call.Callee.Member.Name == "route" {
			router = &object.Call.Callee.Member.Object
		}
		if router.Declaration != nil {
			for _, u := range js.calls {
				if u.Callee == nil || u.Callee.Member == nil || u.Callee.Member.Name != "use" || u.File != f.File || u.Line >= f.Line {
					continue
				}
				if u.Callee.Member.Object.Declaration.key() != router.Declaration.key() || len(u.Arguments) == 0 || u.Arguments[0].String != nil {
					continue
				}
				for _, a := range u.Arguments {
					if js.routerKind(js.variableOf(&a)) == "" {
						used = append(used, a)
					}
				}
			}
		}
		for _, m := range mounts {
			copyRt := *rt
			r := &copyRt
			all := append(append([]string{}, m.segments...), segs...)
			path, held := jsPath(all, lib)
			if held != "" {
				js.gap(clause, []string{"paths"}, "", "the route %s /%s: %s; left out", r.method, strings.Join(all, "/"), held)
				continue
			}
			r.path = path
			r.params, _ = pathParameters(path)
			r.mounts = m.at
			chain := append(append(append([]jsValue{}, m.chain...), used...), r.middleware...)
			js.routePermission(r, chain, m.at)
			routes = append(routes, r)
		}
	}
	if registrations > 0 {
		js.res.say("routes: counted %s: every call of get, post, put, patch, delete, options, head, all or route with a path and a handler on an Express or Fastify router the reader follows, or on another value", plural(registrations, "registration"))
	}
	js.writeRoutes(routes)
}

// routePermission reads a route's permission through the checks the
// implementation file names: middleware a check makes, outermost first,
// or a check the handler calls.
func (js *jsReader) routePermission(rt *jsRoute, chain []jsValue, at []string) {
	var found []string
	var where []string
	use := func(c *permissionCheck, args []jsValue, clause string) {
		if c.argument > len(args) {
			found = append(found, "")
			return
		}
		s, ok := js.constString(&args[c.argument-1])
		if !ok || !permissionWord.MatchString(s) {
			found = append(found, "")
			return
		}
		found = append(found, s)
		where = append(where, clause)
	}
	citeCheck := func(callee *jsValue) {
		if callee.Declaration != nil {
			js.cite(callee.Declaration.clause())
		}
	}
	for _, mw := range chain {
		v := &mw
		if v.Name != nil {
			if vr := js.variableOf(v); vr != nil && vr.Value != nil && vr.Value.Call != nil {
				v = vr.Value
			}
		}
		if v.Call == nil {
			continue
		}
		if c := js.checkOf(&v.Call.Callee); c != nil {
			use(c, v.Call.Arguments, js.clause(rt.fact))
			citeCheck(&v.Call.Callee)
		}
	}
	if rt.handler != nil {
		key := (&jsPos{File: rt.handler.File, Line: rt.handler.Line, Column: rt.handler.Column}).key()
		for _, f := range js.byWithin[key] {
			if f.Kind == "call" {
				if c := js.checkOf(f.Callee); c != nil {
					use(c, f.Arguments, js.clause(f))
					citeCheck(f.Callee)
				}
			}
		}
	}
	switch {
	case len(found) == 1 && found[0] != "":
		rt.permission = found[0]
		rt.checks = where
	case len(found) > 1:
		rt.checks = []string{fmt.Sprintf("%d checks the implementation file names, and an operation checks one permission", len(found))}
	case len(found) == 1:
		rt.checks = []string{"a check the implementation file names whose permission is not a literal permission name"}
	}
}

// jsPath writes Express's or Fastify's segments as a path template,
// :name a parameter {name}, or says why the meta-model cannot hold them.
func jsPath(segs []string, lib string) (string, string) {
	var parts []string
	for _, s := range segs {
		switch {
		case s == "*" || strings.HasSuffix(s, "*"):
			return "", "the segment " + s + " matches anything after it, and a path parameter holds one segment"
		case strings.HasPrefix(s, ":") && strings.HasSuffix(s, "?"):
			return "", "the parameter " + s + " is optional, and a path parameter is always there"
		case strings.ContainsAny(s, "()[]+?"):
			return "", "the segment " + s + " is a pattern, which a path template cannot say"
		case strings.HasPrefix(s, ":"):
			parts = append(parts, "{"+s[1:]+"}")
		default:
			parts = append(parts, s)
		}
	}
	p := "/" + strings.Join(parts, "/")
	if _, why := pathParameters(p); why != "" {
		return "", why
	}
	return p, ""
}

// writeRoutes writes the routes as operations with their questions.
func (js *jsReader) writeRoutes(routes []*jsRoute) {
	if len(routes) == 0 {
		return
	}
	seen := map[string]*jsRoute{}
	var held []*jsRoute
	for _, rt := range routes {
		pair := rt.method + " " + rt.path
		if prev := seen[pair]; prev != nil {
			js.question("must", fmt.Sprintf("%s %s is registered at %s and at %s. Which handler does the running system run?", rt.method, rt.path, js.clause(prev.fact), js.clause(rt.fact)),
				[]string{"#/paths/" + escapeToken(rt.path) + "/" + strings.ToLower(rt.method)}, "A router serves a method and path pair with its first handler.", js.at(js.clause(rt.fact), "Registers "+pair+" again."))
			continue
		}
		seen[pair] = rt
		if !heldMethods[rt.method] {
			js.gap(js.clause(rt.fact), []string{"paths"}, "", "the route %s: the method %s has no operation in the meta-model, which holds GET, POST, PUT, PATCH and DELETE; left out", pair, rt.method)
			continue
		}
		held = append(held, rt)
	}
	uses := map[string]int{}
	for _, rt := range held {
		if rt.name != "" {
			uses[rt.name]++
		}
	}
	for _, rt := range held {
		key := strings.ToLower(rt.method)
		if rt.name != "" && uses[rt.name] == 1 && handlerName.MatchString(rt.name) {
			rt.id = strings.ToLower(rt.name[:1]) + rt.name[1:]
		} else {
			rt.id = methodPathName(key, rt.path)
			if rt.name != "" {
				js.gap(js.clause(rt.fact), []string{"#/paths/" + escapeToken(rt.path) + "/" + key}, "", "the route %s %s: its handler %s serves more than one route; the operation is named %s", rt.method, rt.path, rt.name, rt.id)
			}
		}
		rt.at = "#/paths/" + escapeToken(rt.path) + "/" + key
	}
	sort.SliceStable(held, func(i, j int) bool {
		if held[i].path != held[j].path {
			return held[i].path < held[j].path
		}
		return methodRank(strings.ToLower(held[i].method)) < methodRank(strings.ToLower(held[j].method))
	})
	js.paths = mapping()
	usedBy := map[string][]*jsRoute{}
	var lastPath string
	var item *yaml.Node
	for _, rt := range held {
		clause := js.clause(rt.fact)
		label := rt.method + " " + rt.path
		if rt.path != lastPath {
			lastPath = rt.path
			item = mapping()
			set(js.paths, rt.path, item)
			at := "#/paths/" + escapeToken(rt.path)
			if len(rt.params) > 0 {
				var list []*yaml.Node
				var blocks []string
				for i, name := range rt.params {
					list = append(list, flow(mapping("name", name, "in", "path", "required", true)))
					blocks = append(blocks, fmt.Sprintf("%s/parameters/%d/schema", at, i))
				}
				set(item, "parameters", list)
				js.question("must", fmt.Sprintf("What values does each path parameter of %s take: %s?", rt.path, strings.Join(rt.params, ", ")),
					blocks, fmt.Sprintf("%s names a path's parameters and not the values they take.", jsLibName(rt.lib)))
			}
		}
		op := mapping("operationId", rt.id)
		if rt.permission != "" {
			set(op, "permission", rt.permission)
			usedBy[rt.permission] = append(usedBy[rt.permission], rt)
		}
		says := "Registers " + label
		if rt.name != "" {
			says += "; its handler is " + rt.name
		}
		if rt.permission != "" {
			says += "; its middleware checks " + rt.permission
		}
		cites := []*yaml.Node{js.at(clause, says+".")}
		for _, m := range rt.mounts {
			if rt.lib == "fastify" {
				cites = append(cites, js.at(m, "Registers the plugin that registers "+label+"."))
			} else {
				cites = append(cites, js.at(m, "Mounts the router that registers "+label+"."))
			}
		}
		if rt.handler != nil && js.clause(rt.handler) != clause {
			what := "The handler."
			if rt.name != "" {
				what = "The handler " + rt.name + "."
			}
			cites = append(cites, js.at(js.clause(rt.handler), what))
		}
		reply := js.readHandler(rt, op)
		set(op, "origin", "stated")
		set(op, "cites", cites)
		set(item, strings.ToLower(rt.method), op)
		js.counts["operations"]++
		answer := fmt.Sprintf("What does %s do, and what does it answer?", label)
		why := "A registration names the handler and not what it does or the responses it gives."
		if reply != "" {
			answer = fmt.Sprintf("What does %s do, and what does it answer with each status code? Its handler declares that it answers %s.", label, reply)
			why = "A declared reply type names the body and not the status codes, nor when each is given."
		}
		js.question("must", answer, []string{rt.at + "/summary", rt.at + "/responses"}, why)
		if rt.permission == "" {
			asked := fmt.Sprintf("%s checks no permission through a check the implementation file names.", label)
			if len(rt.checks) > 0 {
				asked = fmt.Sprintf("%s is guarded by %s.", label, rt.checks[0])
			} else if js.checkFile == "" {
				asked = fmt.Sprintf("%s checks no permission the reader knows, since no implementation file names the project's checks.", label)
			}
			js.question("must", asked+" Is it meant to be open to everyone (public), or which permission should it check?",
				[]string{rt.at + "/permission"}, "An operation open to everyone is how an open endpoint is usually found, so it is asked, never assumed.")
		}
		js.question("must", fmt.Sprintf("Does the running system register %s? It is declared at %s, and no list the running system printed was read with it.", label, clause),
			[]string{rt.at}, "Syntax shows a declaration and not what runs; a route table the running system prints says what it registers, and merging it answers this.", js.at(clause, "Registers "+label+"."))
	}
	if len(usedBy) > 0 {
		var names []string
		for n := range usedBy {
			names = append(names, n)
		}
		sort.Strings(names)
		js.permissions = mapping()
		var blocks, grants []string
		for _, n := range names {
			var says []string
			for _, rt := range usedBy[n] {
				says = append(says, rt.method+" "+rt.path)
			}
			set(js.permissions, n, mapping("origin", "stated", "cites", []*yaml.Node{js.at(js.clause(usedBy[n][0].fact), fmt.Sprintf("%s %s %s.", joinAnd(says), checkOrChecks(len(says)), n))}))
			blocks = append(blocks, "#/permissions/"+escapeToken(n)+"/description")
			grants = append(grants, "#/permissions/"+escapeToken(n))
		}
		js.question("must", fmt.Sprintf("What does each permission allow: %s?", strings.Join(names, ", ")),
			blocks, "The source names the permission a route checks and not what it is for.")
		js.question("must", fmt.Sprintf("Which role grants each permission: %s?", strings.Join(names, ", ")),
			grants, "The source names the permission a route checks and not who holds it.")
	}
}

func jsLibName(lib string) string {
	if lib == "fastify" {
		return "Fastify"
	}
	return "Express"
}

// declaredTypes is what a handler declares of its request: the type of
// its body and of its reply, from Express's Request and Response, or
// Fastify's generic. jsdoc is the comment a type comes from when it is
// JSDoc that the compiler does not check, which is asked instead.
func (js *jsReader) declaredTypes(rt *jsRoute) (body, reply *jsType, jsdoc string) {
	if rt.lib == "fastify" {
		var generic *jsType
		if len(rt.fact.TypeArguments) > 0 {
			generic = rt.fact.TypeArguments[0]
		} else if rt.handler != nil && len(rt.handler.Parameters) > 0 && rt.handler.Parameters[0].Type != nil && len(rt.handler.Parameters[0].Type.Arguments) > 0 {
			generic = rt.handler.Parameters[0].Type.Arguments[0]
		}
		if generic != nil {
			for _, m := range generic.Object {
				switch m.Name {
				case "Body":
					body = m.Type
				case "Reply":
					reply = m.Type
				}
			}
		}
		return body, reply, ""
	}
	if rt.handler == nil {
		return nil, nil, ""
	}
	ps := rt.handler.Parameters
	if len(ps) > 0 && ps[0].Type != nil {
		if ps[0].TypeFrom == "jsdoc" {
			if on, _ := js.checkJs(rt.handler.File); !on {
				return nil, nil, rt.handler.Jsdoc
			}
		}
		t := ps[0].Type
		if t.Reference != nil && strings.HasSuffix(*t.Reference, "Request") {
			if len(t.Arguments) > 2 {
				body = t.Arguments[2]
			}
			if len(t.Arguments) > 1 {
				reply = t.Arguments[1]
			}
		}
	}
	if reply == nil && len(ps) > 1 && ps[1].Type != nil && ps[1].Type.Reference != nil && strings.HasSuffix(*ps[1].Type.Reference, "Response") && len(ps[1].Type.Arguments) > 0 {
		if ps[1].TypeFrom != "jsdoc" {
			reply = ps[1].Type.Arguments[0]
		} else if on, _ := js.checkJs(rt.handler.File); on {
			reply = ps[1].Type.Arguments[0]
		}
	}
	return body, reply, ""
}

// isRequestBody says whether a value is the request's body: the body
// member of the handler's first parameter.
func isRequestBody(v *jsValue, handler *jsPos) bool {
	return v != nil && v.Member != nil && v.Member.Name == "body" && v.Member.Object.Parameter != nil &&
		v.Member.Object.Parameter.Index == 0 && v.Member.Object.Parameter.Function.key() == handler.key()
}

// readHandler reads what a handler says of its operation: the path
// parameters it reads and its body, from a validation schema it applies,
// Fastify's JSON Schema, the type it declares, or the fields it reads.
// It returns the reply type the handler declares, as the question on its
// responses names it, or "".
func (js *jsReader) readHandler(rt *jsRoute, op *yaml.Node) string {
	label := rt.method + " " + rt.path
	var reads []*jsFact
	var hpos *jsPos
	if rt.handler != nil {
		hpos = &jsPos{File: rt.handler.File, Line: rt.handler.Line, Column: rt.handler.Column}
		for _, f := range js.byWithin[hpos.key()] {
			if f.Kind == "access" && f.Parameter != nil && f.Parameter.Index == 0 && len(f.Chain) > 2 {
				reads = append(reads, f)
			}
		}
	}
	for _, f := range reads {
		if f.Chain[1] != "params" {
			continue
		}
		name := f.Chain[2]
		if name != "[computed]" && !contains(rt.params, name) {
			js.question("must", fmt.Sprintf("The handler of %s reads the path parameter %s at %s, which the path does not have, so it is always undefined. Which parameter is meant?", label, name, js.clause(f)),
				[]string{"#/paths/" + escapeToken(rt.path) + "/parameters"}, jsLibName(rt.lib)+" gives no value for a parameter the route's path does not name, and the handler reads one.", js.at(js.clause(f), "Reads the path parameter "+name+"."))
		}
	}
	bodyType, replyType, jsdoc := js.declaredTypes(rt)
	if jsdoc != "" {
		_, file := js.checkJs(rt.handler.File)
		where := "no tsconfig.json or jsconfig.json of the folder read turns checkJs on"
		if file != "" {
			where = file + " does not turn checkJs on"
		}
		js.question("should", fmt.Sprintf("The handler of %s at %s states its types in a JSDoc comment, and %s, so the compiler does not hold the code to them: %s. Are they what the handler takes and answers?", label, js.clause(rt.handler), where, strings.Join(strings.Fields(jsdoc), " ")),
			[]string{rt.at + "/requestBody", rt.at + "/responses"}, "A JSDoc type is read as stated only where the compiler checks it, since an unchecked comment may say what the code no longer does.", js.at(js.clause(rt.handler), "States its types in JSDoc."))
	}
	reply := ""
	if replyType != nil {
		if name, section, ok := js.typeName(replyType); ok {
			reply = "#/" + section + "/" + name
		} else if replyType.Array != nil {
			if name, section, ok := js.typeName(replyType.Array); ok {
				reply = "a list of #/" + section + "/" + name
			}
		}
	}
	if rt.method == "GET" || rt.method == "DELETE" {
		return reply
	}
	body := func(node *yaml.Node) {
		set(op, "requestBody", mapping("required", true, "content", mapping("application/json", flow(mapping("schema", node)))))
	}
	// A validation schema the handler applies to its body.
	if hpos != nil {
		for _, f := range js.byWithin[hpos.key()] {
			if f.Kind != "call" || f.Callee == nil || f.Callee.Member == nil || len(f.Arguments) == 0 || !isRequestBody(&f.Arguments[0], hpos) {
				continue
			}
			switch f.Callee.Member.Name {
			case "parse", "safeParse", "parseAsync", "safeParseAsync", "validate", "validateSync", "validateAsync", "cast":
			default:
				continue
			}
			obj := &f.Callee.Member.Object
			if vr := js.variableOf(obj); vr != nil {
				if name := js.validators[obj.Declaration.key()]; name != "" {
					body(flow(mapping("$ref", "#/schemas/"+name)))
					return reply
				}
			}
			js.question("should", fmt.Sprintf("The handler of %s validates its body at %s with %s, which is not a validation schema the reader writes. What is the body?", label, js.clause(f), obj.describe()),
				[]string{rt.at + "/requestBody"}, "A body is written by the schema of a validation the reader knows.", js.at(js.clause(f), "Validates the body."))
			return reply
		}
	}
	// Fastify's JSON Schema of the body.
	if rt.opts != nil {
		if s := rt.opts.prop("schema"); s != nil {
			if b := s.prop("body"); b != nil {
				if b.Name != nil {
					if vr := js.variableOf(b); vr != nil && vr.Value != nil && vr.Value.Object != nil {
						name := validatorName(vr.Name)
						if pascalWord.MatchString(name) && !js.schemaFrom[name] {
							if el := js.jsonSchemaObject(vr.Value, "#/schemas/"+name, js.clause(vr), "the JSON Schema "+vr.Name); el != nil {
								js.schemaFrom[name] = true
								set(el, "origin", "stated")
								set(el, "cites", []*yaml.Node{js.at(js.clause(vr), vr.Name+" is a JSON Schema of an object.")})
								if js.schemas == nil {
									js.schemas = mapping()
								}
								set(js.schemas, name, el)
							}
						}
						if child(js.schemas, name) != nil {
							body(flow(mapping("$ref", "#/schemas/"+name)))
							return reply
						}
					}
				} else if b.Object != nil {
					name := strings.ToUpper(rt.id[:1]) + rt.id[1:] + "Body"
					if el := js.jsonSchemaObject(b, "#/schemas/"+name, js.clause(rt.fact), "the JSON Schema of the body of "+label); el != nil && !js.schemaFrom[name] {
						js.schemaFrom[name] = true
						set(el, "origin", "stated")
						set(el, "cites", []*yaml.Node{js.at(js.clause(rt.fact), "The route's schema gives the body of "+label+".")})
						if js.schemas == nil {
							js.schemas = mapping()
						}
						set(js.schemas, name, el)
						body(flow(mapping("$ref", "#/schemas/"+name)))
						return reply
					}
				}
				js.question("should", fmt.Sprintf("The route %s at %s gives its body's schema as %s, which the reader cannot read as a JSON Schema object. What is the body?", label, js.clause(rt.fact), b.describe()),
					[]string{rt.at + "/requestBody"}, "A body is written from a literal JSON Schema object.", js.at(js.clause(rt.fact), "Gives a body schema."))
				return reply
			}
		}
	}
	// The type the handler declares.
	if bodyType != nil {
		if name, section, ok := js.typeName(bodyType); ok && section == "schemas" {
			body(flow(mapping("$ref", "#/schemas/"+name)))
			return reply
		}
		if bodyType.Object != nil {
			js.question("should", fmt.Sprintf("The handler of %s declares its body as an object type written in place. Which schema is it?", label),
				[]string{rt.at + "/requestBody"}, "A body is written by a named schema.", js.at(js.clause(rt.fact), "Declares its body in place."))
			return reply
		}
		if bodyType.Keyword == "" || (bodyType.Keyword != "any" && bodyType.Keyword != "unknown") {
			ref := bodyType.Text
			if bodyType.Reference != nil {
				ref = *bodyType.Reference
			}
			js.question("must", fmt.Sprintf("The handler of %s declares its body as %s, which is not a type of the files read. What is the body?", label, ref),
				[]string{rt.at + "/requestBody"}, "Only tracked files are read, so a package's types are not known.", js.at(js.clause(rt.fact), "Declares its body."))
			return reply
		}
	}
	// The fields the handler reads of its body.
	var fields []string
	var cites []*yaml.Node
	whole := false
	for _, f := range reads {
		if f.Chain[1] != "body" {
			continue
		}
		name := f.Chain[2]
		if name == "[computed]" || !memberNameWord.MatchString(name) {
			whole = true
			continue
		}
		if !contains(fields, name) {
			fields = append(fields, name)
			cites = append(cites, js.at(js.clause(f), "Reads "+strings.Join(f.Chain, ".")+"."))
		}
	}
	if hpos != nil {
		for _, f := range js.accesses {
			if f.Within.key() == hpos.key() && f.Parameter != nil && len(f.Chain) == 2 && f.Chain[1] == "body" && f.WrappedBy == "" {
				whole = true
			}
		}
	}
	if len(fields) == 0 {
		if whole {
			js.question("must", fmt.Sprintf("The handler of %s uses its body whole, with no validation schema or type the reader reads. What is the body?", label),
				[]string{rt.at + "/requestBody"}, "A body's fields are read from a schema, a declared type or the fields the handler reads, and none is there.", js.at(js.clause(rt.fact), "Registers "+label+"."))
		}
		return reply
	}
	name := strings.ToUpper(rt.id[:1]) + rt.id[1:] + "Body"
	if js.schemaFrom[name] {
		return reply
	}
	js.schemaFrom[name] = true
	at := "#/schemas/" + name
	props := mapping()
	var blocks []string
	for _, n := range fields {
		set(props, n, flow(mapping()))
		blocks = append(blocks, at+"/properties/"+n)
	}
	el := mapping("type", "object", "properties", props, "origin", "stated", "cites", cites)
	if js.schemas == nil {
		js.schemas = mapping()
	}
	set(js.schemas, name, el)
	body(flow(mapping("$ref", at)))
	js.question("must", fmt.Sprintf("The handler of %s reads %s of its body, with no validation schema or declared type. Which type is each, and which are required?", label, joinAnd(fields)),
		append(blocks, at+"/required"), "The code names the fields it reads and not their types, which plain JavaScript does not declare.", cites[0])
	return reply
}
