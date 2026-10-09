package extract

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The methods a Next.js route handler exports, in the order an operation
// is written.
var nextMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

// nextHandler is one method a route file serves, as its code says.
type nextHandler struct {
	method string   // in capitals, or "" when the handler takes every method
	fn     *jsFact  // the handler's function, or nil
	wrap   *jsValue // a call the handler is given to, such as a check
	at     string   // where the handler is exported
	others string   // why the handler may serve methods besides those read, or ""
}

// nextMiddleware is what a middleware file's code says: the patterns its
// matcher selects and the permission its check names.
type nextMiddleware struct {
	file       string
	patterns   []string // nil when it runs before every route
	unknown    string   // why the matcher cannot be read, or ""
	permission string   // the one literal permission a check names, or ""
	checks     int      // the calls of a check the implementation file names
	at         string   // where the check is called
}

// loadFacts reads the code-facts dump of the folder that holds the router,
// so the router's files are read for what their code says (ADR-090).
func (rd *pageReader) loadFacts(facts, implementation string) error {
	js, dumpName, err := openJS(facts, rd.key)
	if err != nil {
		return err
	}
	if js.r.Repository.Root != rd.r.Repository.Root {
		return refuse("%s is a dump of %s, and the router's folder %s is in another repository", dumpName, js.dump.Path, rd.base)
	}
	if !(js.dump.Path == "." || rd.base == js.dump.Path || strings.HasPrefix(rd.base, js.dump.Path+"/")) {
		return refuse("%s is a dump of %s, which does not hold the router's folder %s; dump the folder that holds it", dumpName, js.dump.Path, rd.base)
	}
	js.res = rd.res
	rd.js = js
	rd.res.say("facts: the code-facts dump %s of %s, made at commit %s, gives what the router's files say", dumpName, js.dump.Path, js.r.Commit)
	return js.loadChecks(implementation)
}

// adopt lets the dump's reader ask its questions among the pages
// reader's, under one numbering.
func (rd *pageReader) adopt() {
	if rd.js == nil {
		return
	}
	rd.js.questions = rd.questions
	rd.js.counter = &rd.nextID
}

// takeHeld moves what the dump's reader could not hold into the pages
// reader's list.
func (rd *pageReader) takeHeld() {
	if rd.js == nil {
		return
	}
	for _, h := range rd.js.notHeld {
		rd.notHeld = append(rd.notHeld, pageNotHeld{notHeld: h})
	}
	rd.js.notHeld = nil
}

// schemaFromFacts is a page schema as the compiler read it: the object
// its file exports, with the consts it names, imported or its own,
// resolved where the files read declare them.
func (rd *pageReader) schemaFromFacts(pg *page) (*object, string) {
	js := rd.js
	if js.files[pg.schema] == nil {
		return nil, "the code-facts dump does not hold it"
	}
	var v *jsValue
	for _, f := range js.named {
		if f.File == pg.schema && f.Name == "default" {
			v = f.Value
		}
	}
	if v == nil {
		for _, f := range js.dump.Facts {
			if f.Kind == "variable" && f.File == pg.schema && f.Exported != "" && f.Within == nil && f.Value != nil {
				v = f.Value
				break
			}
		}
	}
	if v != nil && v.Name != nil {
		if vr := js.variableOf(v); vr != nil {
			v = vr.Value
		}
	}
	if v == nil || v.Object == nil {
		return nil, "it exports no object literal"
	}
	lit, why := js.literal(v, 0)
	if why != "" {
		return nil, why
	}
	return lit.(*object), ""
}

// literal is a value as the page schema's subset holds it: a name a const
// of the files read gives is that const's value; any other name is a
// reference, asked for instead.
func (js *jsReader) literal(v *jsValue, depth int) (any, string) {
	if depth > 16 {
		return reference{v.describe()}, ""
	}
	switch {
	case v.String != nil:
		return *v.String, ""
	case v.Number != nil:
		return numeral{*v.Number}, ""
	case v.Boolean != nil:
		return *v.Boolean, ""
	case v.Null != nil:
		return nil, ""
	case v.Array != nil:
		var items []any
		for i := range v.Array {
			if v.Array[i].Spread != nil {
				return nil, "a list spreads " + v.Array[i].Spread.describe()
			}
			x, why := js.literal(&v.Array[i], depth+1)
			if why != "" {
				return nil, why
			}
			items = append(items, x)
		}
		return items, ""
	case v.Object != nil:
		o := &object{values: map[string]any{}, lines: map[string]int{}}
		for _, p := range v.Object {
			if p.Key == nil {
				what := "a computed key"
				if p.Spread != nil {
					what = "a spread of " + p.Spread.describe()
				}
				return nil, "an object holds " + what
			}
			if _, dup := o.values[*p.Key]; dup {
				return nil, "the key " + *p.Key + " is given twice"
			}
			x, why := js.literal(p.Value, depth+1)
			if why != "" {
				return nil, why
			}
			o.keys = append(o.keys, *p.Key)
			o.values[*p.Key] = x
			o.lines[*p.Key] = p.Line
		}
		return o, ""
	case v.Name != nil:
		if vr := js.variableOf(v); vr != nil && vr.Declaration == "const" && vr.Value != nil {
			return js.literal(vr.Value, depth+1)
		}
		return reference{*v.Name}, ""
	}
	return reference{v.describe()}, ""
}

// nextHandlers is each method a route file's code serves: the functions
// an App Router route file exports by a method's name, or the methods a
// Pages Router API route's default export compares req.method with.
func (rd *pageReader) nextHandlers(rf *routeFile) []nextHandler {
	js := rd.js
	if js == nil || js.files[rf.file] == nil {
		return nil
	}
	clause := func(f *jsFact) string { return js.clause(f) }
	handlerOf := func(v *jsValue) (*jsFact, *jsValue) {
		if v == nil {
			return nil, nil
		}
		if fn := js.functionOf(v); fn != nil {
			return fn, nil
		}
		if v.Call != nil {
			for i := range v.Call.Arguments {
				if fn := js.functionOf(&v.Call.Arguments[i]); fn != nil {
					return fn, v
				}
			}
			return nil, v
		}
		return nil, nil
	}
	var out []nextHandler
	if rd.router == routerApp {
		for _, m := range nextMethods {
			for _, k := range sortedFnKeys(js.functions) {
				f := js.functions[k]
				if f.File == rf.file && f.Name == m && f.Exported != "" && f.Within == nil {
					out = append(out, nextHandler{method: m, fn: f, at: clause(f)})
				}
			}
			for _, f := range js.dump.Facts {
				if f.Kind == "variable" && f.File == rf.file && f.Name == m && f.Exported != "" && f.Within == nil {
					fn, wrap := handlerOf(f.Value)
					out = append(out, nextHandler{method: m, fn: fn, wrap: wrap, at: clause(f)})
				}
				if f.Kind == "export" && f.File == rf.file && f.Name == m {
					fn, wrap := handlerOf(f.Value)
					out = append(out, nextHandler{method: m, fn: fn, wrap: wrap, at: clause(f)})
				}
			}
		}
		return out
	}
	// The Pages Router: the default export, and the methods it tells
	// apart by req.method.
	var fn *jsFact
	var wrap *jsValue
	at := ""
	for _, k := range sortedFnKeys(js.functions) {
		f := js.functions[k]
		if f.File == rf.file && f.Exported == "default" {
			fn, at = f, clause(f)
		}
	}
	if fn == nil {
		if e := js.exports[rf.file]; e != nil {
			fn, wrap = handlerOf(e.Value)
			at = clause(e)
		}
	}
	if fn == nil {
		return nil
	}
	pos := (&jsPos{File: fn.File, Line: fn.Line, Column: fn.Column}).key()
	var methods []string
	others := ""
	for _, f := range js.byWithin[pos] {
		if f.Kind != "access" || len(f.Chain) != 2 || f.Chain[1] != "method" || f.Parameter == nil || f.Parameter.Index != 0 {
			continue
		}
		switch {
		case f.Compared != nil:
			others = fmt.Sprintf("it compares req.method with a value at %s, and what follows the comparison runs for every other method", js.clause(f))
		case f.HasDefault:
			others = fmt.Sprintf("its switch on req.method at %s has a default branch, which runs for every other method", js.clause(f))
		}
		values := f.Cases
		if f.Compared != nil {
			values = append(values, *f.Compared)
		}
		for i := range values {
			if s, ok := values[i].stringLiteral(); ok && !contains(methods, strings.ToUpper(s)) {
				methods = append(methods, strings.ToUpper(s))
			}
		}
	}
	if len(methods) == 0 {
		return []nextHandler{{fn: fn, wrap: wrap, at: at}}
	}
	sort.SliceStable(methods, func(i, j int) bool { return indexOf(nextMethods, methods[i]) < indexOf(nextMethods, methods[j]) })
	for _, m := range methods {
		out = append(out, nextHandler{method: m, fn: fn, wrap: wrap, at: at, others: others})
	}
	return out
}

func indexOf(list []string, s string) int {
	for i, x := range list {
		if x == s {
			return i
		}
	}
	return len(list)
}

func sortedFnKeys(m map[string]*jsFact) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// handlerPermission is the permission the checks the implementation file
// names give a handler: a check its export is wrapped in, or one it
// calls; why says why there is none.
func (rd *pageReader) handlerPermission(h nextHandler, label string) (string, string) {
	js := rd.js
	rt := &jsRoute{fact: &jsFact{File: strings.SplitN(h.at, ":", 2)[0]}, handler: h.fn, branch: h.method}
	if h.fn != nil {
		rt.fact.Line = h.fn.Line
	}
	var chain []jsValue
	if h.wrap != nil {
		chain = append(chain, *h.wrap)
	}
	js.routePermission(rt, chain, nil)
	if rt.permission != "" {
		return rt.permission, ""
	}
	if len(rt.checks) > 0 {
		return "", fmt.Sprintf("%s is guarded by %s.", label, rt.checks[0])
	}
	if js.checkFile == "" {
		return "", fmt.Sprintf("%s checks no permission the reader knows, since no implementation file names the project's checks.", label)
	}
	return "", fmt.Sprintf("%s checks no permission through a check the implementation file names.", label)
}

// handlerBody is the validation schema a handler applies to its body: to
// await request.json() in a route handler, or to req.body in an API
// route.
func (rd *pageReader) handlerBody(fn *jsFact) string {
	js := rd.js
	if fn == nil {
		return ""
	}
	pos := &jsPos{File: fn.File, Line: fn.Line, Column: fn.Column}
	isBody := func(v *jsValue) bool {
		if isRequestBody(v, pos) {
			return true
		}
		if v != nil && v.Name != nil {
			if vr := js.variableOf(v); vr != nil && vr.Value != nil {
				v = vr.Value
			}
		}
		if isRequestBody(v, pos) {
			return true
		}
		return v != nil && v.Call != nil && v.Call.Callee.Member != nil && (v.Call.Callee.Member.Name == "json" || v.Call.Callee.Member.Name == "formData") &&
			v.Call.Callee.Member.Object.Parameter != nil && v.Call.Callee.Member.Object.Parameter.Index == 0 && v.Call.Callee.Member.Object.Parameter.Function.key() == pos.key()
	}
	for _, f := range js.byWithin[pos.key()] {
		if f.Kind != "call" || f.Callee == nil || f.Callee.Member == nil || len(f.Arguments) == 0 || !isBody(&f.Arguments[0]) {
			continue
		}
		switch f.Callee.Member.Name {
		case "parse", "safeParse", "parseAsync", "safeParseAsync", "validate", "validateSync", "validateAsync", "cast":
			obj := &f.Callee.Member.Object
			if js.variableOf(obj) != nil {
				if name := js.validators[obj.Declaration.key()]; name != "" {
					return name
				}
			}
		}
	}
	return ""
}

// staticParams is what a file's generateStaticParams returns as literal
// values, by parameter; why says why it gives none.
func (rd *pageReader) staticParams(file string) (map[string][]string, string, string) {
	js := rd.js
	if js == nil {
		return nil, "", ""
	}
	for _, k := range sortedFnKeys(js.functions) {
		f := js.functions[k]
		if f.File != file || f.Name != "generateStaticParams" || f.Exported == "" {
			continue
		}
		at := js.clause(f)
		if len(f.ReturnValues) != 1 || f.ReturnValues[0].Array == nil {
			return nil, at, "it builds the list it returns at run time"
		}
		out := map[string][]string{}
		for _, item := range f.ReturnValues[0].Array {
			if item.Object == nil {
				return nil, at, "an item it returns is " + item.describe()
			}
			for _, p := range item.Object {
				s, ok := js.constString(p.Value)
				if p.Key == nil || !ok {
					return nil, at, "a value it returns is not a literal"
				}
				if !contains(out[*p.Key], s) {
					out[*p.Key] = append(out[*p.Key], s)
				}
			}
		}
		return out, at, ""
	}
	return nil, "", ""
}

// describeParams writes the values generateStaticParams gives each
// parameter.
func describeParams(values map[string][]string) string {
	var names []string
	for n := range values {
		names = append(names, n)
	}
	sort.Strings(names)
	var parts []string
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%s the values %s", n, joinAnd(quoteAll(values[n]))))
	}
	return joinAnd(parts)
}

// readStaticParams writes a line for each page whose generateStaticParams
// gives its parameters' values, since a page holds no parameter.
func (rd *pageReader) readStaticParams() {
	if rd.js == nil {
		return
	}
	for _, pg := range rd.pages {
		values, at, why := rd.staticParams(pg.file)
		switch {
		case at == "":
		case why != "":
			rd.js.question("should", fmt.Sprintf("generateStaticParams at %s lists the values the page at %s is built for, and %s. Which values does it give?", at, pg.route, why),
				[]string{"#/pages/" + pg.name + "/route"}, "The values a route is built for are known by syntax only when they are literals.", rd.js.at(at, "Lists the values of the page's parameters."))
		default:
			rd.pageGap(pg, "route", false, at, "%s: generateStaticParams gives %s, and a page holds no values of its route's parameters", at, describeParams(values))
		}
	}
}

// serverActions writes an operation for each server action a page's form
// submits to: a function with 'use server', or exported by a file that
// starts with it, given to a form's action.
func (rd *pageReader) serverActions(pages map[string]*page) []*routeFile {
	js := rd.js
	if js == nil {
		return nil
	}
	isAction := map[string]*jsFact{}
	for k, f := range js.functions {
		if contains(f.Directives, "use server") || (contains(js.directives[f.File], "use server") && f.Exported != "" && f.Within == nil) {
			isAction[k] = f
		}
	}
	if len(isAction) == 0 {
		return nil
	}
	byFile := map[string]*page{}
	for _, pg := range pages {
		byFile[pg.file] = pg
	}
	var out []*routeFile
	used := map[string]bool{}
	for _, f := range js.elements {
		if f.Tag == nil || f.Tag.Intrinsic == nil || *f.Tag.Intrinsic != "form" {
			continue
		}
		var action *jsValue
		for i := range f.Attributes {
			if f.Attributes[i].Name == "action" {
				action = f.Attributes[i].Value
			}
		}
		fn := js.functionOf(action)
		if fn == nil {
			continue
		}
		key := (&jsPos{File: fn.File, Line: fn.Line, Column: fn.Column}).key()
		if isAction[key] == nil {
			continue
		}
		used[key] = true
		clause := js.clause(f)
		pg := byFile[f.File]
		name := fn.Name
		if pg == nil {
			js.question("must", fmt.Sprintf("The form at %s submits to the server action %s, in a file that is no page's, so the reader cannot tell which page shows it. Which page is it, and which path does the action answer at?", clause, name),
				[]string{"paths"}, "Next.js posts a server action to the page that shows its form, and the page is known only where its file holds the form.", js.at(clause, "Submits to "+name+"."))
			continue
		}
		if name == "" || !handlerName.MatchString(name) {
			js.question("must", fmt.Sprintf("The form at %s submits to a server action with no name an operation can take. Which operation is it?", clause),
				[]string{"paths"}, "An operation is named after its server action.", js.at(clause, "Submits to a server action."))
			continue
		}
		rf := &routeFile{file: pg.file, path: pg.route, method: "POST", key: "post",
			says: fmt.Sprintf("The page at %s shows a form whose action is the server action %s.", pg.route, name), action: fn, actionAt: clause}
		rf.params, _ = pathParameters(pg.route)
		out = append(out, rf)
	}
	n := 0
	for k, f := range isAction {
		if !used[k] {
			n++
			_ = f
		}
	}
	rd.res.say("actions: counted %s with 'use server', %d of them given to a form's action in a file read", plural(len(isAction), "server action"), len(isAction)-n)
	return out
}

// readMiddleware reads what each middleware file's code says: its
// matcher's patterns and the permission its check names.
func (rd *pageReader) readMiddleware() {
	js := rd.js
	if js == nil {
		return
	}
	for _, file := range rd.middleware {
		if js.files[file] == nil {
			continue
		}
		mw := &nextMiddleware{file: file}
		for _, f := range js.dump.Facts {
			if f.Kind != "variable" || f.File != file || f.Name != "config" || f.Exported == "" || f.Within != nil {
				continue
			}
			m := f.Value.prop("matcher")
			if m == nil {
				break
			}
			var list []jsValue
			if m.Array != nil {
				list = m.Array
			} else {
				list = []jsValue{*m}
			}
			mw.patterns = []string{}
			for i := range list {
				s, ok := js.constString(&list[i])
				if !ok {
					mw.unknown = fmt.Sprintf("its matcher holds %s, which is not a literal pattern", list[i].describe())
					break
				}
				if _, ok := matcherSegments(s); !ok {
					mw.unknown = fmt.Sprintf("its matcher holds the pattern %q, a regular expression the reader does not read", s)
					break
				}
				mw.patterns = append(mw.patterns, s)
			}
		}
		var fn *jsFact
		for _, k := range sortedFnKeys(js.functions) {
			f := js.functions[k]
			if f.File == file && f.Within == nil && (f.Exported == "default" || f.Exported != "" && (f.Name == "middleware" || f.Name == "proxy")) {
				fn = f
			}
		}
		if fn == nil {
			if e := js.exports[file]; e != nil {
				fn = js.functionOf(e.Value)
			}
		}
		if fn != nil {
			label := "the middleware " + file
			h := nextHandler{fn: fn, at: js.clause(fn)}
			p, _ := rd.handlerPermission(h, label)
			pos := (&jsPos{File: fn.File, Line: fn.Line, Column: fn.Column}).key()
			for _, f := range js.byWithin[pos] {
				if f.Kind == "call" && js.checkOf(f.Callee) != nil {
					mw.checks++
					if mw.at == "" {
						mw.at = js.clause(f)
					}
				}
			}
			mw.permission = p
		}
		rd.mw = append(rd.mw, mw)
		what := "every route"
		if mw.patterns != nil {
			what = "the routes " + joinAnd(quoteAll(mw.patterns)) + " match"
		}
		if mw.unknown != "" {
			what = "routes the reader cannot tell, since " + mw.unknown
		}
		check := "no check the implementation file names"
		if js.checkFile == "" {
			check = "no check the reader knows, since no implementation file names one"
		}
		if mw.permission != "" {
			check = "a check of the permission " + mw.permission + " at " + mw.at
		} else if mw.checks > 0 {
			check = fmt.Sprintf("%s the implementation file names, with no one literal permission", plural(mw.checks, "call of a check"))
		}
		rd.res.say("middleware: %s runs before %s, and calls %s", file, what, check)
	}
}

// matcherSegments splits a matcher's pattern into its segments, or says
// it is a regular expression the reader does not read.
func matcherSegments(p string) ([]string, bool) {
	if !strings.HasPrefix(p, "/") {
		return nil, false
	}
	segs := splitSegments(p)
	for i, s := range segs {
		switch {
		case strings.HasPrefix(s, ":") && (strings.HasSuffix(s, "*") || strings.HasSuffix(s, "+")):
			if i != len(segs)-1 {
				return nil, false
			}
		case strings.HasPrefix(s, ":"):
			if strings.ContainsAny(s[1:], "()[]?*+") {
				return nil, false
			}
		case s == "(.*)":
			if i != len(segs)-1 {
				return nil, false
			}
		case strings.ContainsAny(s, "()[]?*+\\^$|"):
			return nil, false
		}
	}
	return segs, true
}

// covers says whether a middleware's matcher selects a route surely, and
// whether it may: a route parameter against a literal segment may match.
func (mw *nextMiddleware) covers(route string) (sure, may bool) {
	if mw.unknown != "" {
		return false, true
	}
	if mw.patterns == nil {
		return true, true
	}
	rs := splitSegments(route)
	for _, p := range mw.patterns {
		ps, _ := matcherSegments(p)
		s, m := matchSegments(ps, rs)
		sure, may = sure || s, may || m
	}
	return sure, may
}

func matchSegments(ps, rs []string) (sure, may bool) {
	sure, may = true, true
	for i, p := range ps {
		last := i == len(ps)-1
		switch {
		case strings.HasPrefix(p, ":") && strings.HasSuffix(p, "*") || p == "(.*)":
			return sure, may
		case strings.HasPrefix(p, ":") && strings.HasSuffix(p, "+"):
			if len(rs) <= i {
				return false, false
			}
			return sure, may
		}
		if len(rs) <= i {
			return false, false
		}
		r := rs[i]
		switch {
		case strings.HasPrefix(p, ":"):
		case strings.HasPrefix(r, "{"):
			sure = false
		case p != r:
			return false, false
		}
		if last && len(rs) != len(ps) {
			return false, false
		}
	}
	if len(rs) != len(ps) {
		return false, false
	}
	return sure, may
}

// middlewarePermission is the permission a middleware gives a route it
// surely covers, and where.
func (rd *pageReader) middlewarePermission(route string) (string, *nextMiddleware) {
	var found []*nextMiddleware
	for _, mw := range rd.mw {
		if sure, _ := mw.covers(route); sure && mw.permission != "" {
			found = append(found, mw)
		}
	}
	if len(found) == 1 {
		return found[0].permission, found[0]
	}
	return "", nil
}

// writeNextPermissions declares each permission an operation's handler,
// its wrapper or a middleware gives, beside those the page schemas name.
func (rd *pageReader) writeNextPermissions(used map[string][]string, cites map[string]*yaml.Node) {
	if len(used) == 0 {
		return
	}
	if rd.permissions == nil {
		rd.permissions = mapping()
	}
	var names []string
	for n := range used {
		if child(rd.permissions, n) == nil {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return
	}
	var blocks, grants []string
	for _, n := range names {
		set(rd.permissions, n, mapping("origin", "stated", "cites", []*yaml.Node{cites[n]}))
		blocks = append(blocks, "#/permissions/"+escapeToken(n)+"/description")
		grants = append(grants, "#/permissions/"+escapeToken(n))
	}
	rd.question(fmt.Sprintf("What does each permission allow: %s?", strings.Join(names, ", ")), blocks, "The code names the permission an operation checks and not what it is for.")
	rd.question(fmt.Sprintf("Which role grants each permission: %s?", strings.Join(names, ", ")), grants, "The code names the permission an operation checks and not who holds it.")
}
