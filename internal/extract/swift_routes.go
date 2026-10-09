package extract

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// swMount is a place routes are registered under: the literal segments
// joined so far and the middleware the groups add, outermost first.
type swMount struct {
	segments []string
	chain    []*swCall
	at       []string // where each group was made, outermost first
}

// The methods Vapor's route calls name.
var vaporMethods = map[string]string{"get": "GET", "post": "POST", "put": "PUT", "patch": "PATCH", "delete": "DELETE"}

// swRoute is one route a Vapor builder registers.
type swRoute struct {
	fact       *swFact
	method     string
	path       string
	params     []string
	handler    string // the function's name, "" for a closure
	handlerFn  *swFact
	permission string
	checks     []string // where a named check was found, for the citation
	id         string
	at         string
}

// vaporParamType is the type of a function's parameter as Vapor names a
// router: an Application, or a RoutesBuilder.
func vaporParamType(t string) string {
	t = strings.TrimSpace(t)
	for _, p := range []string{"some ", "any ", "inout "} {
		t = strings.TrimPrefix(t, p)
	}
	return t
}

// readRoutes reads the routes Vapor's builders register with literal
// segments: from an Application a function is given, the groups made from
// it, and the RouteCollections registered on them.
func (sw *swiftReader) readRoutes() {
	vapor := false
	for _, p := range sw.fileOrder {
		vapor = vapor || sw.files[p].imports["Vapor"]
	}
	if !vapor {
		return
	}
	collections := map[string][]swMount{}
	resolving := map[string]bool{}
	var mountsOf func(scope string, v *swValue) ([]swMount, string)
	var scopeVars func(scope string) map[string][]swMount
	varsCache := map[string]map[string][]swMount{}
	unknownVars := map[string]map[string]string{}
	var collectionMounts func(typ string) ([]swMount, string)
	collectionMounts = func(typ string) ([]swMount, string) {
		if m, ok := collections[typ]; ok {
			return m, ""
		}
		if resolving[typ] {
			return nil, "a collection that registers itself"
		}
		resolving[typ] = true
		var mounts []swMount
		for _, f := range sw.calls {
			if f.Name != "register" || !sw.imports(f, "Vapor") {
				continue
			}
			v := f.argument("collection")
			if v == nil {
				continue
			}
			if name, _ := v.construct(); name != typ {
				continue
			}
			base, why := mountsOf(f.scope(), f.Base)
			if why != "" {
				continue
			}
			for _, b := range base {
				b.at = append(append([]string{}, b.at...), sw.clause(f))
				mounts = append(mounts, b)
			}
		}
		collections[typ] = mounts
		if len(mounts) == 0 {
			return nil, "a RouteCollection that no register(collection:) of the files read registers on a builder the reader follows"
		}
		return mounts, ""
	}
	// The builders a function is given.
	roots := func(scope string) (map[string][]swMount, map[string]string) {
		vars, unknown := map[string][]swMount{}, map[string]string{}
		parts := strings.SplitN(scope, "|", 2)
		t := sw.types[parts[0]]
		if t == nil {
			return vars, unknown
		}
		fn := t.funcs[parts[1]]
		if fn == nil {
			return vars, unknown
		}
		for _, p := range fn.Parameters {
			switch vaporParamType(p.Type) {
			case "Application":
				vars[p.Name] = []swMount{{}}
			case "RoutesBuilder":
				if fn.Name == "boot" && t.conforms("RouteCollection") {
					m, why := collectionMounts(t.name)
					if why != "" {
						unknown[p.Name] = why
					}
					vars[p.Name] = m
				} else {
					unknown[p.Name] = "the RoutesBuilder the function " + fn.Name + " is given by a caller the reader does not follow"
				}
			}
		}
		return vars, unknown
	}
	scopeVars = func(scope string) map[string][]swMount {
		if v, ok := varsCache[scope]; ok {
			return v
		}
		vars, unknown := roots(scope)
		varsCache[scope], unknownVars[scope] = vars, unknown
		for _, f := range sw.byScope[scope] {
			if f.Kind != "call" {
				continue
			}
			switch {
			case f.Name == "grouped" && f.AssignedTo != "":
				m, why := mountsOf(scope, &swValue{Call: &swCall{Name: "grouped", Base: f.Base, Arguments: f.Arguments}})
				if why != "" {
					unknown[f.AssignedTo] = why
					continue
				}
				for i := range m {
					m[i].at = append(append([]string{}, m[i].at...), sw.clause(f))
				}
				vars[f.AssignedTo] = m
			case f.Name == "group" && len(f.Closures) > 0 && f.Closures[0].Value.Closure != nil && len(f.Closures[0].Value.Closure.Parameters) == 1:
				p := f.Closures[0].Value.Closure.Parameters[0]
				m, why := mountsOf(scope, &swValue{Call: &swCall{Name: "grouped", Base: f.Base, Arguments: f.Arguments}})
				if why != "" {
					unknown[p] = why
					continue
				}
				for i := range m {
					m[i].at = append(append([]string{}, m[i].at...), sw.clause(f))
				}
				vars[p] = m
			}
		}
		return vars
	}
	mountsOf = func(scope string, v *swValue) ([]swMount, string) {
		switch {
		case v == nil:
			return nil, "nothing"
		case v.Name != nil:
			vars := scopeVars(scope)
			if m, ok := vars[*v.Name]; ok && len(m) > 0 {
				return m, ""
			}
			if why := unknownVars[scope][*v.Name]; why != "" {
				return nil, why
			}
			return nil, *v.Name + ", a builder the reader does not follow"
		case v.Call != nil && (v.Call.Name == "grouped" || v.Call.Name == "group") && len(v.Call.Closures) == 0:
			base, why := mountsOf(scope, v.Call.Base)
			if why != "" {
				return nil, why
			}
			var segs []string
			var chain []*swCall
			for _, a := range v.Call.Arguments {
				if a.Label != "" {
					return nil, "a group made with the argument " + a.Label
				}
				if s, ok := a.Value.stringLiteral(); ok {
					segs = append(segs, splitSegments(s)...)
					continue
				}
				if name, c := a.Value.construct(); name != "" && c == a.Value.Call {
					chain = append(chain, c)
					continue
				}
				return nil, "a group whose path is " + a.Value.describe() + ", which is not a literal"
			}
			var out []swMount
			for _, b := range base {
				out = append(out, swMount{segments: append(append([]string{}, b.segments...), segs...), chain: append(append([]*swCall{}, b.chain...), chain...), at: b.at})
			}
			return out, ""
		}
		return nil, v.describe() + ", which the reader does not follow"
	}

	var routes []*swRoute
	registrations := 0
	for _, f := range sw.calls {
		if !sw.imports(f, "Vapor") || f.Base == nil {
			continue
		}
		method, isRoute := vaporMethods[f.Name]
		if f.Name == "on" {
			isRoute = true
		}
		if !isRoute || f.argument("use") == nil && len(f.Closures) == 0 {
			continue
		}
		registrations++
		clause := sw.clause(f)
		args := f.Arguments
		if f.Name == "on" {
			if len(args) == 0 || args[0].Value.Member == nil {
				sw.question("must", fmt.Sprintf("%s registers a route with on, by a method that is not a literal. Which method and path does the running system serve here?", clause),
					[]string{"paths"}, "A method computed at run time is not known by syntax.", sw.at(clause, "Registers a route with on."))
				continue
			}
			method = strings.ToUpper(strings.TrimPrefix(*args[0].Value.Member, "."))
			args = args[1:]
		}
		if f.InLoop || f.InCondition {
			where := "in a loop"
			if !f.InLoop {
				where = "behind a condition"
			}
			sw.question("must", fmt.Sprintf("%s registers a route %s, so the reader cannot tell how many routes it registers, or whether it registers one. Which routes does the running system register here?", clause, where),
				[]string{"paths"}, "Syntax does not run the code; only the running system's own list of routes, such as a route table, says what a loop or a condition registers.", sw.at(clause, "Registers a route "+where+"."))
			continue
		}
		var segs []string
		computed := ""
		for _, a := range args {
			if a.Label == "use" {
				continue
			}
			if a.Label != "" {
				continue
			}
			if s, ok := a.Value.stringLiteral(); ok {
				segs = append(segs, splitSegments(s)...)
				continue
			}
			computed = a.Value.describe()
		}
		if computed != "" {
			sw.question("must", fmt.Sprintf("%s registers a route whose path has %s, which is not a literal. Which path does the running system serve here?", clause, computed),
				[]string{"paths"}, "A path computed at run time is not known by syntax.", sw.at(clause, "Registers a route with a path that is not a literal."))
			continue
		}
		mounts, why := mountsOf(f.scope(), f.Base)
		if why != "" {
			sw.question("must", fmt.Sprintf("%s registers %s %s on %s. Under which path does the running system serve it, and does it?", clause, method, "/"+strings.Join(segs, "/"), why),
				[]string{"paths"}, "Syntax follows a builder from where it is made, and not one handed over in a way it does not read.", sw.at(clause, "Registers a route."))
			continue
		}
		for _, m := range mounts {
			rt := &swRoute{fact: f, method: method}
			all := append(append([]string{}, m.segments...), segs...)
			path, held := vaporPath(all)
			if held != "" {
				sw.gap(clause, []string{"paths"}, "", "the route %s /%s: %s; left out", method, strings.Join(all, "/"), held)
				continue
			}
			rt.path = path
			params, _ := pathParameters(path)
			rt.params = params
			if h := f.argument("use"); h != nil && h.Name != nil {
				rt.handler = strings.TrimPrefix(*h.Name, "self.")
				if t := sw.types[f.Within]; t != nil && t.funcs[rt.handler] != nil {
					rt.handlerFn = t.funcs[rt.handler]
				} else if t := sw.types[""]; t != nil && t.funcs[rt.handler] != nil {
					rt.handlerFn = t.funcs[rt.handler]
				}
			}
			var found []string
			for _, mw := range m.chain {
				for _, c := range sw.checks {
					if c.function != mw.Name {
						continue
					}
					if c.argument > len(mw.Arguments) {
						found = append(found, "")
						continue
					}
					s, ok := mw.Arguments[c.argument-1].Value.stringLiteral()
					if !ok || !permissionWord.MatchString(s) {
						found = append(found, "")
						continue
					}
					found = append(found, s)
					if t := sw.types[c.function]; t != nil && t.decl != nil {
						sw.cite(t.decl.File)
					}
				}
			}
			switch {
			case len(found) == 1 && found[0] != "":
				rt.permission = found[0]
				rt.checks = m.at
			case len(found) > 1:
				rt.checks = []string{fmt.Sprintf("%d checks the implementation file names, one in each group it is under, and an operation checks one permission", len(found))}
			case len(found) == 1:
				rt.checks = []string{"a check the implementation file names whose permission is not a literal permission name"}
			}
			routes = append(routes, rt)
		}
	}
	if registrations > 0 {
		sw.res.say("routes: counted %s: every call of get, post, put, patch, delete or on with a handler on a value of a file that imports Vapor", plural(registrations, "registration"))
	}
	sw.writeRoutes(routes)
}

// splitSegments splits a literal path argument at its slashes, as Vapor
// does.
func splitSegments(s string) []string {
	var out []string
	for _, p := range strings.Split(s, "/") {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// vaporPath writes Vapor's segments as a path template, :name a parameter
// {name}, or says why the meta-model cannot hold them.
func vaporPath(segs []string) (string, string) {
	var parts []string
	for _, s := range segs {
		switch {
		case s == "**":
			return "", "it ends in the catch-all **, which matches any number of segments, and a path parameter holds one"
		case s == "*":
			return "", "the segment * matches any one segment and names no parameter, which a path template cannot say"
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
func (sw *swiftReader) writeRoutes(routes []*swRoute) {
	if len(routes) == 0 {
		return
	}
	seen := map[string]*swRoute{}
	var held []*swRoute
	for _, rt := range routes {
		pair := rt.method + " " + rt.path
		if prev := seen[pair]; prev != nil {
			sw.question("must", fmt.Sprintf("%s %s is registered at %s and at %s. Which handler does the running system run?", rt.method, rt.path, sw.clause(prev.fact), sw.clause(rt.fact)),
				[]string{"#/paths/" + escapeToken(rt.path) + "/" + strings.ToLower(rt.method)}, "A router serves a method and path pair once.", sw.at(sw.clause(rt.fact), "Registers "+pair+" again."))
			continue
		}
		seen[pair] = rt
		if !heldMethods[rt.method] {
			sw.gap(sw.clause(rt.fact), []string{"paths"}, "", "the route %s: the method %s has no operation in the meta-model, which holds GET, POST, PUT, PATCH and DELETE; left out", pair, rt.method)
			continue
		}
		held = append(held, rt)
	}
	count := map[string]int{}
	for _, rt := range held {
		if rt.handler != "" {
			count[rt.handler]++
		}
	}
	for _, rt := range held {
		key := strings.ToLower(rt.method)
		if rt.handler != "" && count[rt.handler] == 1 && handlerName.MatchString(rt.handler) {
			rt.id = strings.ToLower(rt.handler[:1]) + rt.handler[1:]
		} else {
			rt.id = methodPathName(key, rt.path)
			if rt.handler != "" {
				sw.gap(sw.clause(rt.fact), []string{"#/paths/" + escapeToken(rt.path) + "/" + key}, "", "the route %s %s: its handler %s serves more than one route; the operation is named %s", rt.method, rt.path, rt.handler, rt.id)
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
	sw.paths = mapping()
	usedBy := map[string][]*swRoute{}
	var lastPath string
	var item *yaml.Node
	for _, rt := range held {
		clause := sw.clause(rt.fact)
		label := rt.method + " " + rt.path
		if rt.path != lastPath {
			lastPath = rt.path
			item = mapping()
			set(sw.paths, rt.path, item)
			at := "#/paths/" + escapeToken(rt.path)
			if len(rt.params) > 0 {
				var list []*yaml.Node
				var blocks []string
				for i, name := range rt.params {
					list = append(list, flow(mapping("name", name, "in", "path", "required", true)))
					blocks = append(blocks, fmt.Sprintf("%s/parameters/%d/schema", at, i))
				}
				set(item, "parameters", list)
				sw.question("must", fmt.Sprintf("What values does each path parameter of %s take: %s?", rt.path, strings.Join(rt.params, ", ")),
					blocks, "Vapor names a path's parameters and not the values they take.")
			}
		}
		op := mapping("operationId", rt.id)
		if rt.permission != "" {
			set(op, "permission", rt.permission)
			usedBy[rt.permission] = append(usedBy[rt.permission], rt)
		}
		says := fmt.Sprintf("Registers %s", label)
		if rt.handler != "" {
			says += "; its handler is " + rt.handler
		}
		if rt.permission != "" {
			says += fmt.Sprintf("; the group made at %s checks %s", rt.checks[len(rt.checks)-1], rt.permission)
		}
		cites := []*yaml.Node{sw.at(clause, says+".")}
		if rt.handlerFn != nil {
			cites = append(cites, sw.at(sw.clause(rt.handlerFn), fmt.Sprintf("The handler %s.", rt.handler)))
			sw.readHandler(rt, op)
		}
		set(op, "origin", "stated")
		set(op, "cites", cites)
		set(item, strings.ToLower(rt.method), op)
		sw.counts["operations"]++
		sw.question("must", fmt.Sprintf("What does %s do, and what does it answer?", label),
			[]string{rt.at + "/summary", rt.at + "/responses"}, "A registration names the handler and not what it does or the responses it gives.")
		if rt.permission == "" {
			asked := fmt.Sprintf("%s checks no permission through a check the implementation file names.", label)
			if len(rt.checks) > 0 {
				asked = fmt.Sprintf("%s is guarded by %s.", label, rt.checks[0])
			} else if sw.checkFile == "" {
				asked = fmt.Sprintf("%s checks no permission the reader knows, since no implementation file names the project's checks.", label)
			}
			sw.question("must", asked+" Is it meant to be open to everyone (public), or which permission should it check?",
				[]string{rt.at + "/permission"}, "An operation open to everyone is how an open endpoint is usually found, so it is asked, never assumed.")
		}
		sw.question("must", fmt.Sprintf("Does the running system register %s? It is declared at %s, and no list the running system printed was read with it.", label, clause),
			[]string{rt.at}, "Syntax shows a declaration and not what runs; a route table the running system prints says what it registers, and merging it answers this.", sw.at(clause, "Registers "+label+"."))
	}
	if len(usedBy) > 0 {
		var names []string
		for n := range usedBy {
			names = append(names, n)
		}
		sort.Strings(names)
		sw.permissions = mapping()
		var blocks, grants []string
		for _, n := range names {
			var says []string
			for _, rt := range usedBy[n] {
				says = append(says, rt.method+" "+rt.path)
			}
			set(sw.permissions, n, mapping("origin", "stated", "cites", []*yaml.Node{sw.at(sw.clause(usedBy[n][0].fact), fmt.Sprintf("%s %s %s.", joinAnd(says), checkOrChecks(len(says)), n))}))
			blocks = append(blocks, "#/permissions/"+escapeToken(n)+"/description")
			grants = append(grants, "#/permissions/"+escapeToken(n))
		}
		sw.question("must", fmt.Sprintf("What does each permission allow: %s?", strings.Join(names, ", ")),
			blocks, "The source names the permission a route checks and not what it is for.")
		sw.question("must", fmt.Sprintf("Which role grants each permission: %s?", strings.Join(names, ", ")),
			grants, "The source names the permission a route checks and not who holds it.")
	}
}

// readHandler reads what a handler function says of its operation: the
// path parameters it reads and the body it decodes.
func (sw *swiftReader) readHandler(rt *swRoute, op *yaml.Node) {
	label := rt.method + " " + rt.path
	scope := rt.handlerFn.Within + "|" + rt.handlerFn.Name
	for _, f := range sw.byScope[scope] {
		if f.Kind != "call" {
			continue
		}
		clause := sw.clause(f)
		base := f.baseName()
		switch {
		case f.Name == "get" && strings.HasSuffix(base, ".parameters") && len(f.Arguments) > 0:
			name, ok := f.Arguments[0].Value.stringLiteral()
			if !ok {
				sw.question("should", fmt.Sprintf("The handler of %s reads a path parameter at %s by a name that is not a literal. Which does it read?", label, clause),
					[]string{"#/paths/" + escapeToken(rt.path) + "/parameters"}, "A name computed at run time is not known by syntax.", sw.at(clause, "Reads a path parameter."))
				continue
			}
			if !contains(rt.params, name) {
				sw.question("must", fmt.Sprintf("The handler of %s reads the path parameter %s at %s, which the path does not have, so it is always nil. Which parameter is meant?", label, name, clause),
					[]string{"#/paths/" + escapeToken(rt.path) + "/parameters"}, "Vapor gives nil for a parameter the route's path does not name, and the handler reads one.", sw.at(clause, "Reads the path parameter "+name+"."))
			}
		case f.Name == "decode" && strings.HasSuffix(base, ".content") && len(f.Arguments) > 0 && f.Arguments[0].Value.TypeName != nil:
			typ := *f.Arguments[0].Value.TypeName
			field := sw.field(typ, true)
			if field.model == "" || field.held != "" || child(op, "requestBody") != nil {
				sw.question("should", fmt.Sprintf("The handler of %s decodes its body as %s at %s, which is not a model of the files read the reader writes. What is the body?", label, typ, clause),
					[]string{rt.at + "/requestBody"}, "A body is written by the schema of a type the reader writes.", sw.at(clause, "Decodes the body as "+typ+"."))
				continue
			}
			set(op, "requestBody", mapping("required", true, "content", mapping("application/json", flow(mapping("schema", field.node)))))
		}
	}
}

// swClient is one call to another system a client makes.
type swClient struct {
	clause, method, url, dependency, says string
}

// readClients reads the calls to other systems URLSession makes, with a
// URLRequest or a URL built from literal parts.
func (sw *swiftReader) readClients() {
	var clients []swClient
	n := 0
	urlVars := map[string]map[string]*swValue{} // scope -> name -> the URL value
	requests := map[string]map[string]bool{}
	for scope, facts := range sw.byScope {
		for _, f := range facts {
			if f.Kind != "call" || f.AssignedTo == "" {
				continue
			}
			switch f.Name {
			case "URL":
				if v := f.argument("string"); v != nil && f.Base == nil {
					if urlVars[scope] == nil {
						urlVars[scope] = map[string]*swValue{}
					}
					urlVars[scope][f.AssignedTo] = v
				}
			case "URLRequest":
				if requests[scope] == nil {
					requests[scope] = map[string]bool{}
				}
				requests[scope][f.AssignedTo] = true
			}
		}
	}
	// urlOf is the URL a value names: a URL(string:) call, or a name given
	// one in the same function or as a property of the same type.
	urlOf := func(f *swFact, v *swValue) *swValue {
		if v == nil {
			return nil
		}
		if v.Call != nil && v.Call.Name == "URL" && v.Call.Base == nil {
			return v.Call.argument("string")
		}
		if v.Name != nil {
			name := strings.TrimPrefix(*v.Name, "self.")
			if u := urlVars[f.scope()][name]; u != nil {
				return u
			}
			if u := urlVars[f.Within+"|"+name][name]; u != nil {
				return u
			}
		}
		return nil
	}
	add := func(f *swFact, method string, u *swValue, how string) {
		clause := sw.clause(f)
		if u == nil {
			sw.question("should", fmt.Sprintf("%s calls another system with %s at a URL that is not a literal. Which system and operation does it call?", clause, how),
				[]string{"dependencies"}, "A URL computed at run time is not known by syntax, and a dependency is named by the system it calls.", sw.at(clause, "Calls "+how+" with a URL that is not a literal."))
			return
		}
		text, params, ok := urlTemplate(u)
		if !ok {
			sw.question("should", fmt.Sprintf("%s calls another system with %s at %s, built from parts the reader cannot place. Which system and operation does it call?", clause, how, u.describe()),
				[]string{"dependencies"}, "A URL is read when its host is a literal and each value fills one whole path segment.", sw.at(clause, "Calls "+how+" at "+u.describe()+"."))
			return
		}
		parsed, err := url.Parse(strings.NewReplacer("{", "", "}", "").Replace(text))
		if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
			sw.question("should", fmt.Sprintf("%s calls %s %s, which names no host. Which system does it call?", clause, method, text),
				[]string{"dependencies"}, "A dependency is named by the system it calls, and the URL does not name one.", sw.at(clause, fmt.Sprintf("Calls %s %s.", method, text)))
			return
		}
		dep := camel(strings.ToLower(hostWord.ReplaceAllString(parsed.Hostname(), "_")))
		says := fmt.Sprintf("Calls %s %s with %s.", method, text, how)
		clients = append(clients, swClient{clause: clause, method: method, url: text, dependency: dep, says: says})
		if len(params) > 0 {
			var holes []string
			for _, p := range params {
				holes = append(holes, "{"+p+"}")
			}
			sw.question("should", fmt.Sprintf("%s calls %s %s, filling %s from values of the code. What does each hold?", clause, method, text, joinAnd(holes)),
				[]string{"#/dependencies/" + dep}, "A part of a URL interpolated from a value is a parameter, and the code says only which variable fills it.", sw.at(clause, says))
		}
	}
	for _, f := range sw.calls {
		switch f.Name {
		case "URLRequest":
			if f.Base != nil {
				continue
			}
			n++
			method := "GET"
			for _, a := range sw.byScope[f.scope()] {
				if a.Kind == "assignment" && a.Target == f.AssignedTo+".httpMethod" && f.AssignedTo != "" {
					if s, ok := a.Value.stringLiteral(); ok {
						method = strings.ToUpper(s)
					} else {
						method = ""
						sw.question("should", fmt.Sprintf("%s sets the method of a request to %s, which is not a literal. Which method does it call?", sw.clause(a), a.Value.describe()),
							[]string{"dependencies"}, "A method computed at run time is not known by syntax.", sw.at(sw.clause(a), "Sets httpMethod."))
					}
				}
			}
			if method == "" {
				continue
			}
			add(f, method, urlOf(f, f.argument("url")), "URLRequest")
		case "data", "dataTask", "download", "downloadTask", "upload", "uploadTask":
			if f.Base == nil || len(f.Arguments) == 0 {
				continue
			}
			v := &f.Arguments[0].Value
			if l := f.Arguments[0].Label; l != "from" && l != "with" && l != "for" && l != "" {
				continue
			}
			if v.Name != nil && requests[f.scope()][*v.Name] {
				continue
			}
			if v.Call != nil && v.Call.Name == "URLRequest" {
				continue
			}
			u := urlOf(f, v)
			if u == nil && !(v.Call != nil && v.Call.Name == "URL") {
				continue
			}
			n++
			add(f, "GET", u, "URLSession's "+f.Name)
		}
	}
	if n > 0 {
		sw.res.say("clients: counted %s: every URLRequest, and every call of URLSession's data, download and upload methods with a URL", plural(n, "call"))
	}
	if len(clients) == 0 {
		return
	}
	byName := map[string][]swClient{}
	var names []string
	for _, c := range clients {
		if byName[c.dependency] == nil {
			names = append(names, c.dependency)
		}
		byName[c.dependency] = append(byName[c.dependency], c)
	}
	sort.Strings(names)
	sw.deps = mapping()
	for _, name := range names {
		var cites []*yaml.Node
		for _, c := range byName[name] {
			cites = append(cites, sw.at(c.clause, c.says))
		}
		at := "#/dependencies/" + name
		set(sw.deps, name, mapping("origin", "stated", "cites", cites))
		sw.question("must", fmt.Sprintf("What is the system %s the source calls, and how long may one call to it take before the caller gives up?", name),
			[]string{at + "/description", at + "/timeout"}, "The source names the URL it calls, and not what the system is or a time limit, which every dependency has.")
	}
}

// urlTemplate writes a URL string as a template, each interpolated name
// that fills a whole path segment a parameter {name}; ok is false when a
// value fills anything else.
func urlTemplate(v *swValue) (string, []string, bool) {
	if s, ok := v.stringLiteral(); ok {
		return s, nil, true
	}
	if v.Interpolated == nil {
		return "", nil, false
	}
	var b strings.Builder
	var params []string
	parts := v.Interpolated
	for i, p := range parts {
		if p.Text != nil {
			b.WriteString(*p.Text)
			continue
		}
		name := *p.Expression
		before := b.String()
		next := ""
		if i+1 < len(parts) && parts[i+1].Text != nil {
			next = *parts[i+1].Text
		}
		if !strings.Contains(before, "://") || !strings.HasSuffix(before, "/") || strings.Count(before, "/") < 3 ||
			(next != "" && !strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "?")) || !memberNameWord.MatchString(name) {
			return "", nil, false
		}
		params = append(params, name)
		b.WriteString("{" + name + "}")
	}
	return b.String(), params, true
}
