package extract

import (
	"fmt"
	"go/ast"
	gotoken "go/token"
	"net/url"
	"os"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/wirename"
)

// The routers the Go reader knows, by import path, and the name its lines
// give each (docs/reading-code.md, Go on net/http and the common routers).
var routerLibraries = map[string]string{
	"net/http":                    "net/http",
	"github.com/go-chi/chi/v5":    "chi",
	"github.com/go-chi/chi":       "chi",
	"github.com/gin-gonic/gin":    "gin",
	"github.com/labstack/echo/v4": "echo",
	"github.com/labstack/echo":    "echo",
	"github.com/gorilla/mux":      "gorilla/mux",
}

// routerMakers are the calls that make a router, by library.
var routerMakers = map[string]map[string]bool{
	"net/http":    {"NewServeMux": true},
	"chi":         {"NewRouter": true, "NewMux": true},
	"gin":         {"New": true, "Default": true},
	"echo":        {"New": true},
	"gorilla/mux": {"NewRouter": true},
}

// routerTypes are the types whose values are routers, by library.
var routerTypes = map[string]map[string]bool{
	"net/http":    {"ServeMux": true},
	"chi":         {"Router": true, "Mux": true},
	"gin":         {"Engine": true, "RouterGroup": true, "IRouter": true, "IRoutes": true},
	"echo":        {"Echo": true, "Group": true},
	"gorilla/mux": {"Router": true},
}

// chi's registration calls named after a method.
var chiMethods = map[string]string{
	"Get": "GET", "Post": "POST", "Put": "PUT", "Patch": "PATCH", "Delete": "DELETE",
	"Head": "HEAD", "Options": "OPTIONS", "Connect": "CONNECT", "Trace": "TRACE",
}

// gin's and echo's registration calls named after a method.
var capitalMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
	"HEAD": true, "OPTIONS": true, "CONNECT": true, "TRACE": true,
}

// The validators whose struct tags the reader reads, by the tag's key and
// the import path that must be in the module for it to be read.
var validatorTags = []struct{ key, library string }{
	{"validate", "github.com/go-playground/validator/v10"},
	{"validate", "github.com/go-playground/validator"},
	{"binding", "github.com/gin-gonic/gin"},
}

// The patterns go-playground/validator checks its alpha, alphanum and
// numeric rules with, as its regexes.go writes them.
var validatorPatterns = map[string]string{
	"alpha":    `^[a-zA-Z]+$`,
	"alphanum": `^[a-zA-Z0-9]+$`,
	"numeric":  `^[-+]?[0-9]+(?:\.[0-9]+)?$`,
}

// The formats go-playground/validator's rules name, as the meta-model
// writes them.
var validatorFormats = map[string]string{
	"email": "email", "uuid": "uuid", "uri": "uri", "url": "uri", "hostname": "hostname", "ipv4": "ipv4", "ipv6": "ipv6",
}

// The net/http calls a client makes, and where each takes its method and
// URL: a method of -1 is the one the call is named after.
var clientCalls = map[string]struct {
	method, url int
	named       string
}{
	"Get": {-1, 0, "GET"}, "Head": {-1, 0, "HEAD"}, "Post": {-1, 0, "POST"}, "PostForm": {-1, 0, "POST"},
	"NewRequest": {0, 1, ""}, "NewRequestWithContext": {1, 2, ""},
}

var (
	routeWildcard = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)(:[^}]*)?\}`)
	colonParam    = regexp.MustCompile(`^:([A-Za-z_][A-Za-z0-9_]*)$`)
	hostWord      = regexp.MustCompile(`[^A-Za-z0-9]+`)
)

// routerNode is one router value: made by a library's call, a group or a
// subrouter of another, or given to a function.
type routerNode struct {
	lib     string
	parent  *routerNode
	prefix  string     // the path it adds to its parent's
	unknown string     // why its place is not known, "" when it is
	checks  []checkUse // the checks every route on it passes, as far as read
	mounted string     // where it is mounted, "" when it is not
	methods []string   // the methods a gorilla/mux subrouter matches, nil for any
}

// place is the path every route on the router starts with, or why it is
// not known.
func (n *routerNode) place() (string, string) {
	prefix := ""
	for x := n; x != nil; x = x.parent {
		if x.unknown != "" {
			return "", x.unknown
		}
		prefix = joinPath(x.prefix, prefix)
	}
	return prefix, ""
}

// chainChecks is every check the router and the routers it is in apply.
func (n *routerNode) chainChecks() []checkUse {
	var out []checkUse
	for x := n; x != nil; x = x.parent {
		out = append(append([]checkUse{}, x.checks...), out...)
	}
	return out
}

// checkUse is one call of a permission check the implementation file names.
type checkUse struct {
	clause     string
	check      string // as the code names it
	permission string // "" when the argument is not a literal
	argument   string // the argument as the code writes it
}

// permissionCheck is one check the implementation file names.
type permissionCheck struct {
	pkg, function string // the function, or Type.Method
	argument      int    // the permission's argument, from 1
}

// goRoute is one route a router registers.
type goRoute struct {
	file       *goFile
	clause     string
	lib        string
	node       *routerNode
	method     string
	path       string // as registered, before the router's prefix, in {name} form
	handler    string // as the code writes it
	name       string // the handler's name, "" when it has none
	middleware []string
	checks     []checkUse
	handlerAt  string
	reads      []paramRead
	body       *bodyRead
	bodies     int // calls that decode a body
	from, to   gotoken.Pos
	hFile      *goFile
	calls      []string
	key, id    string
	at         string
	full       string
	params     []string
}

type paramRead struct {
	name   string
	clause string
}

type bodyRead struct {
	clause string
	file   *goFile
	typ    ast.Expr // the type the body is decoded into, nil when not found
	text   string   // the variable as the code writes it
}

// clientCall is one call of another system the source makes.
type clientCall struct {
	clause     string
	file       *goFile
	pos        gotoken.Pos
	method     string
	dependency string // "" when the URL is not read
	says       string
}

// routeState is what readRouters keeps while it walks the module.
type routeState struct {
	methods   map[string]*ast.FuncDecl // folder|Type.Method -> declaration
	methodOf  map[*ast.FuncDecl]*goFile
	types     map[string]*ast.TypeSpec // folder|Type -> declaration
	typeFile  map[*ast.TypeSpec]*goFile
	walked    map[string]*routerNode // a function and its bound routers -> what it returns
	seen      map[*ast.FuncDecl]bool
	active    map[*ast.FuncDecl]bool
	fields    map[string]*routerNode
	deflt     *routerNode
	routes    []*goRoute
	count     int
	checks    []permissionCheck
	checkFile string // the implementation file, "" when none was given
	clients   []clientCall
	validates map[string]bool // the tag keys of the validators the module imports
}

// readRouters reads the routes the module registers on net/http's
// ServeMux, chi, gin, echo and gorilla/mux, by syntax.
func (g *goReader) readRouters() {
	st := g.rt
	st.methods, st.methodOf = map[string]*ast.FuncDecl{}, map[*ast.FuncDecl]*goFile{}
	st.types, st.typeFile = map[string]*ast.TypeSpec{}, map[*ast.TypeSpec]*goFile{}
	st.walked, st.seen, st.active, st.fields = map[string]*routerNode{}, map[*ast.FuncDecl]bool{}, map[*ast.FuncDecl]bool{}, map[string]*routerNode{}
	st.validates = map[string]bool{}
	routerFiles := false
	called := map[string]bool{}
	for _, f := range g.files {
		dir := path.Dir(f.path)
		for _, ip := range f.imports {
			if routerLibraries[ip] != "" {
				routerFiles = true
			}
			for _, v := range validatorTags {
				if ip == v.library {
					st.validates[v.key] = true
				}
			}
		}
		for _, d := range f.ast.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if t := recvType(d); t != "" {
					st.methods[dir+"|"+t+"."+d.Name.Name] = d
					st.methodOf[d] = f
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					if ts, ok := s.(*ast.TypeSpec); ok {
						st.types[dir+"|"+ts.Name.Name] = ts
						st.typeFile[ts] = f
					}
				}
			}
		}
		ast.Inspect(f.ast, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				switch fn := call.Fun.(type) {
				case *ast.Ident:
					called[fn.Name] = true
				case *ast.SelectorExpr:
					called[fn.Sel.Name] = true
				}
			}
			return true
		})
	}
	g.readClients()
	if !routerFiles {
		return
	}
	// Every function no call in the module names is where the walk starts;
	// then every function not walked yet, a router it is given unknown.
	var later []*ast.FuncDecl
	for _, f := range g.files {
		for _, d := range f.ast.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			if called[fd.Name.Name] {
				later = append(later, fd)
				continue
			}
			g.walkFunc(f, fd, nil, true)
		}
	}
	for _, fd := range later {
		if !st.seen[fd] {
			g.walkFunc(g.fileOf(fd), fd, nil, true)
		}
	}
	for _, c := range st.checks {
		if fd, f := g.checkDecl(c); fd != nil {
			g.gatesInCheck(f, fd)
		}
	}
	libs := map[string]bool{}
	for _, r := range st.routes {
		libs[r.lib] = true
	}
	if st.count > 0 {
		g.res.say("routers: counted %s: every call that registers a route on a router of net/http, chi, gin, echo or gorilla/mux that the reader follows from where it is made, with its groups, mounts and subrouters", plural(st.count, "registration"))
	}
	for _, f := range g.files {
		for _, ip := range f.imports {
			if lib := routerLibraries[ip]; lib != "" && lib != "net/http" && !f.cited {
				g.res.say("nothing read: %s imports %s, and the reader found no route or handler in it", f.path, lib)
				break
			}
		}
	}
}

func (g *goReader) fileOf(fd *ast.FuncDecl) *goFile {
	if f := g.funcFile[fd]; f != nil {
		return f
	}
	return g.rt.methodOf[fd]
}

// recvType is the name of a method's receiver type, or "".
func recvType(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) != 1 {
		return ""
	}
	t := fd.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	switch t := t.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	case *ast.IndexListExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

// pkgPath is the import path of a folder of the module, or the folder
// itself when no go.mod was read.
func (g *goReader) pkgPath(dir string) string {
	if g.modPath == "" {
		return dir
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(dir, g.modRoot), "/")
	if g.modRoot == "." {
		rel = dir
		if dir == "." {
			rel = ""
		}
	}
	if rel == "" {
		return g.modPath
	}
	return g.modPath + "/" + rel
}

// dirOf is the folder of a package of the module, or "" when the import
// path is not the module's.
func (g *goReader) dirOf(ip string) string {
	if g.modPath == "" {
		return ""
	}
	if ip != g.modPath && !strings.HasPrefix(ip, g.modPath+"/") {
		return ""
	}
	return path.Join(g.modRoot, strings.TrimPrefix(strings.TrimPrefix(ip, g.modPath), "/"))
}

// typeKey names a type as its import path and name, a pointer taken off:
// "net/http.ServeMux", or "" when the expression names no type by syntax.
func (g *goReader) typeKey(f *goFile, e ast.Expr) string {
	if s, ok := e.(*ast.StarExpr); ok {
		e = s.X
	}
	switch e := e.(type) {
	case *ast.Ident:
		if g.rt.types[path.Dir(f.path)+"|"+e.Name] != nil {
			return g.pkgPath(path.Dir(f.path)) + "." + e.Name
		}
	case *ast.SelectorExpr:
		if x, ok := e.X.(*ast.Ident); ok && f.imports[x.Name] != "" {
			return f.imports[x.Name] + "." + e.Sel.Name
		}
	}
	return ""
}

// splitKey splits a type key into its import path and its name.
func splitKey(key string) (string, string) {
	i := strings.LastIndex(key, ".")
	if i < 0 {
		return "", key
	}
	return key[:i], key[i+1:]
}

// routerKind is the library whose router a type is, or "": a router type of
// a library the reader knows, or a type of the module that has ServeMux's
// Handle or HandleFunc method, which is read as a ServeMux.
func (g *goReader) routerKind(key string) string {
	if key == "" {
		return ""
	}
	ip, name := splitKey(key)
	if lib := routerLibraries[ip]; lib != "" && routerTypes[lib][name] {
		return lib
	}
	dir := g.dirOf(ip)
	if dir == "" && g.modPath == "" {
		dir = ip
	}
	ts := g.rt.types[dir+"|"+name]
	if ts == nil {
		return ""
	}
	if it, ok := ts.Type.(*ast.InterfaceType); ok {
		for _, m := range it.Methods.List {
			if ft, ok := m.Type.(*ast.FuncType); ok && len(m.Names) == 1 && muxMethod(m.Names[0].Name, ft) {
				return "net/http"
			}
		}
		return ""
	}
	for _, m := range []string{"Handle", "HandleFunc"} {
		if fd := g.rt.methods[dir+"|"+name+"."+m]; fd != nil && muxMethod(m, fd.Type) {
			return "net/http"
		}
	}
	return ""
}

// muxMethod says whether a method is ServeMux's Handle or HandleFunc: that
// name, two parameters, the first a string.
func muxMethod(name string, ft *ast.FuncType) bool {
	if name != "Handle" && name != "HandleFunc" || ft.Params == nil {
		return false
	}
	var types []ast.Expr
	for _, p := range ft.Params.List {
		n := len(p.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			types = append(types, p.Type)
		}
	}
	return len(types) == 2 && exprText(types[0]) == "string"
}

// routeScope is one function being walked: the routers and the types its
// names hold.
type routeScope struct {
	g     *goReader
	file  *goFile
	env   map[string]*routerNode
	types map[string]string
	nodes map[ast.Expr]*routerNode
	skip  map[ast.Node]bool
	stack []ast.Node
	ret   *routerNode
}

// walkFunc walks one function with the routers its parameters are bound
// to, and gives the router it returns, if any. given makes a router
// parameter with nothing bound one whose place is not known.
func (g *goReader) walkFunc(f *goFile, fd *ast.FuncDecl, bound map[int]*routerNode, given bool) *routerNode {
	st := g.rt
	key := fmt.Sprintf("%p", fd)
	var idx []int
	for i := range bound {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	for _, i := range idx {
		key += fmt.Sprintf("|%d=%p", i, bound[i])
	}
	if r, ok := st.walked[key]; ok {
		return r
	}
	if st.active[fd] || fd.Body == nil {
		return nil
	}
	st.active[fd] = true
	defer delete(st.active, fd)
	st.seen[fd] = true
	sc := g.newScope(f)
	if fd.Recv != nil && len(fd.Recv.List) == 1 && len(fd.Recv.List[0].Names) == 1 {
		sc.types[fd.Recv.List[0].Names[0].Name] = g.typeKey(f, fd.Recv.List[0].Type)
	}
	i := 0
	for _, p := range fd.Type.Params.List {
		t := g.typeKey(f, p.Type)
		if len(p.Names) == 0 {
			i++
			continue
		}
		for _, n := range p.Names {
			if t != "" {
				sc.types[n.Name] = t
			}
			switch {
			case bound[i] != nil:
				sc.env[n.Name] = bound[i]
			case given && g.routerKind(t) != "":
				sc.env[n.Name] = &routerNode{lib: g.routerKind(t), unknown: fmt.Sprintf("%s is given the router %s, and the reader found no call that passes it one whose path it knows", g.clause(f, fd.Pos()), n.Name)}
			}
			i++
		}
	}
	sc.walk(fd.Body)
	st.walked[key] = sc.ret
	return sc.ret
}

func (g *goReader) newScope(f *goFile) *routeScope {
	return &routeScope{g: g, file: f, env: map[string]*routerNode{}, types: map[string]string{}, nodes: map[ast.Expr]*routerNode{}, skip: map[ast.Node]bool{}}
}

// hasRouterParam says whether a function takes a router.
func (g *goReader) hasRouterParam(f *goFile, fd *ast.FuncDecl) bool {
	for _, p := range fd.Type.Params.List {
		if g.routerKind(g.typeKey(f, p.Type)) != "" {
			return true
		}
	}
	return false
}

// walk visits a function's statements in order.
func (sc *routeScope) walk(body ast.Node) {
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			sc.stack = sc.stack[:len(sc.stack)-1]
			return true
		}
		descend := sc.visit(n)
		if descend {
			sc.stack = append(sc.stack, n)
		}
		return descend
	})
}

func (sc *routeScope) visit(n ast.Node) bool {
	switch n := n.(type) {
	case *ast.AssignStmt:
		for i, lhs := range n.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok || len(n.Rhs) != len(n.Lhs) {
				continue
			}
			sc.assign(id.Name, n.Rhs[i])
		}
	case *ast.ValueSpec:
		for i, id := range n.Names {
			if n.Type != nil {
				if t := sc.g.typeKey(sc.file, n.Type); t != "" {
					sc.types[id.Name] = t
				}
			}
			if i < len(n.Values) {
				sc.assign(id.Name, n.Values[i])
			}
		}
	case *ast.ReturnStmt:
		if len(n.Results) == 1 && sc.ret == nil {
			sc.ret = sc.eval(n.Results[0])
		}
	case *ast.CallExpr:
		if sc.skip[n] {
			return true
		}
		return sc.call(n)
	}
	return true
}

func (sc *routeScope) assign(name string, rhs ast.Expr) {
	if node := sc.eval(rhs); node != nil {
		sc.env[name] = node
	}
	if t := sc.g.valueType(sc.file, rhs); t != "" {
		sc.types[name] = t
	}
}

// valueType is the type of the module's a value is made as: T{}, &T{} or
// new(T).
func (g *goReader) valueType(f *goFile, e ast.Expr) string {
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == gotoken.AND {
		e = u.X
	}
	switch e := e.(type) {
	case *ast.CompositeLit:
		if e.Type != nil {
			return g.typeKey(f, e.Type)
		}
	case *ast.CallExpr:
		if id, ok := e.Fun.(*ast.Ident); ok && id.Name == "new" && len(e.Args) == 1 {
			return g.typeKey(f, e.Args[0])
		}
	}
	return ""
}

// eval is the router an expression gives, or nil.
func (sc *routeScope) eval(e ast.Expr) *routerNode {
	g := sc.g
	switch e := e.(type) {
	case *ast.ParenExpr:
		return sc.eval(e.X)
	case *ast.Ident:
		return sc.env[e.Name]
	case *ast.UnaryExpr, *ast.CompositeLit:
		if n, ok := sc.nodes[e]; ok {
			return n
		}
		var n *routerNode
		if lib := g.routerKind(g.valueType(sc.file, e)); lib != "" {
			n = &routerNode{lib: lib}
		}
		sc.nodes[e] = n
		return n
	case *ast.SelectorExpr:
		x, ok := e.X.(*ast.Ident)
		if !ok || sc.types[x.Name] == "" {
			return nil
		}
		ip, name := splitKey(sc.types[x.Name])
		dir := g.dirOf(ip)
		if dir == "" && g.modPath == "" {
			dir = ip
		}
		ts := g.rt.types[dir+"|"+name]
		if ts == nil {
			return nil
		}
		stt, ok := ts.Type.(*ast.StructType)
		if !ok {
			return nil
		}
		for _, fl := range stt.Fields.List {
			for _, fn := range fl.Names {
				if fn.Name != e.Sel.Name {
					continue
				}
				lib := g.routerKind(g.typeKey(g.rt.typeFile[ts], fl.Type))
				if lib == "" {
					return nil
				}
				k := dir + "|" + name + "." + fn.Name
				if g.rt.fields[k] == nil {
					g.rt.fields[k] = &routerNode{lib: lib}
				}
				return g.rt.fields[k]
			}
		}
	case *ast.CallExpr:
		if n, ok := sc.nodes[e]; ok {
			return n
		}
		sc.nodes[e] = nil
		n := sc.evalCall(e)
		sc.nodes[e] = n
		return n
	}
	return nil
}

// evalCall is the router a call gives: a library's maker, a group or
// subrouter, or what a function of the module returns.
func (sc *routeScope) evalCall(call *ast.CallExpr) *routerNode {
	g := sc.g
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if isSel {
		if x, ok := sel.X.(*ast.Ident); ok && sc.env[x.Name] == nil {
			if lib := routerLibraries[sc.file.imports[x.Name]]; lib != "" && routerMakers[lib][sel.Sel.Name] {
				return &routerNode{lib: lib}
			}
		}
		if recv := sc.eval(sel.X); recv != nil {
			return sc.derive(recv, call, sel.Sel.Name)
		}
		if sel.Sel.Name == "Subrouter" {
			return sc.subrouter(call)
		}
	}
	if fd, f := sc.callee(call); fd != nil {
		bound := sc.bind(fd, f, call)
		if len(bound) == 0 && g.hasRouterParam(f, fd) {
			return nil
		}
		return g.walkFunc(f, fd, bound, false)
	}
	return nil
}

// derive is the router a call on a router gives: a group with a prefix, a
// router with middleware, or nil.
func (sc *routeScope) derive(recv *routerNode, call *ast.CallExpr, name string) *routerNode {
	switch {
	case (recv.lib == "gin" || recv.lib == "echo") && name == "Group" && len(call.Args) >= 1:
		child := &routerNode{lib: recv.lib, parent: recv}
		sc.prefix(child, call.Args[0], "Group")
		child.checks = sc.checksIn(call.Args[1:])
		return child
	case recv.lib == "chi" && name == "With":
		return &routerNode{lib: recv.lib, parent: recv, checks: sc.checksIn(call.Args)}
	case recv.lib == "chi" && (name == "Route" || name == "Group"):
		return sc.chiScope(recv, call, name)
	}
	return nil
}

// prefix sets a group's prefix from its argument.
func (sc *routeScope) prefix(child *routerNode, e ast.Expr, how string) {
	p, ok := sc.g.stringValue(sc.file, e)
	if !ok {
		child.unknown = fmt.Sprintf("%s: %s is given the prefix %s, which is not a literal", sc.g.clause(sc.file, e.Pos()), how, exprText(e))
		return
	}
	conv, why := sc.g.routePath(child.lib, p)
	if why != "" {
		child.unknown = fmt.Sprintf("%s: %s is given the prefix %s: %s", sc.g.clause(sc.file, e.Pos()), how, p, why)
		return
	}
	child.prefix = conv
}

// chiScope reads chi's Route(prefix, fn) and Group(fn): a router whose
// routes fn registers.
func (sc *routeScope) chiScope(recv *routerNode, call *ast.CallExpr, name string) *routerNode {
	child := &routerNode{lib: recv.lib, parent: recv}
	args := call.Args
	if name == "Route" {
		if len(args) != 2 {
			return nil
		}
		sc.prefix(child, args[0], "Route")
		args = args[1:]
	} else if len(args) != 1 {
		return nil
	}
	sc.within(child, args[0])
	return child
}

// within walks a function that is handed a router, as chi's Route and
// Group hand theirs.
func (sc *routeScope) within(child *routerNode, fn ast.Expr) {
	switch fn := fn.(type) {
	case *ast.FuncLit:
		if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 || len(fn.Type.Params.List[0].Names) == 0 {
			return
		}
		name := fn.Type.Params.List[0].Names[0].Name
		old, had := sc.env[name]
		sc.env[name] = child
		sc.skip[fn] = true
		sc.walk(fn.Body)
		if had {
			sc.env[name] = old
		} else {
			delete(sc.env, name)
		}
	default:
		if fd, f := sc.funcValue(fn); fd != nil {
			sc.g.walkFunc(f, fd, map[int]*routerNode{0: child}, false)
		}
	}
}

// subrouter reads gorilla/mux's PathPrefix(...).Subrouter() and the like.
func (sc *routeScope) subrouter(call *ast.CallExpr) *routerNode {
	sel := call.Fun.(*ast.SelectorExpr)
	chain, recv := sc.chain(sel.X)
	if recv == nil || recv.lib != "gorilla/mux" {
		return nil
	}
	child := &routerNode{lib: recv.lib, parent: recv}
	for _, c := range chain {
		s := c.Fun.(*ast.SelectorExpr)
		switch s.Sel.Name {
		case "PathPrefix":
			if len(c.Args) == 1 {
				sc.prefix(child, c.Args[0], "PathPrefix")
			}
		case "Methods":
			child.methods = nil
			for _, a := range c.Args {
				m, ok := sc.g.methodValue(sc.file, a)
				if !ok {
					child.unknown = fmt.Sprintf("%s: Methods is given %s, which is not a literal method", sc.g.clause(sc.file, a.Pos()), exprText(a))
					break
				}
				child.methods = append(child.methods, strings.ToUpper(m))
			}
		default:
			child.unknown = fmt.Sprintf("%s: the subrouter matches by %s as well as by method and path", sc.g.clause(sc.file, c.Pos()), s.Sel.Name)
		}
	}
	return child
}

// chain is the calls between a gorilla/mux router and a route's end, from
// the router outward, and the router.
func (sc *routeScope) chain(e ast.Expr) ([]*ast.CallExpr, *routerNode) {
	var calls []*ast.CallExpr
	for {
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return nil, nil
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil, nil
		}
		calls = append([]*ast.CallExpr{call}, calls...)
		sc.skip[call] = true
		if recv := sc.eval(sel.X); recv != nil {
			return calls, recv
		}
		e = sel.X
	}
}

// callee is the module's function a call names, found by syntax.
func (sc *routeScope) callee(call *ast.CallExpr) (*ast.FuncDecl, *goFile) {
	return sc.funcValue(call.Fun)
}

// funcValue is the module's function or method an expression names.
func (sc *routeScope) funcValue(e ast.Expr) (*ast.FuncDecl, *goFile) {
	g := sc.g
	switch e := e.(type) {
	case *ast.Ident:
		if fd := g.funcs[path.Dir(sc.file.path)][e.Name]; fd != nil {
			return fd, g.funcFile[fd]
		}
	case *ast.SelectorExpr:
		x, ok := e.X.(*ast.Ident)
		if !ok {
			return nil, nil
		}
		if t := sc.types[x.Name]; t != "" {
			ip, name := splitKey(t)
			dir := g.dirOf(ip)
			if dir == "" && g.modPath == "" {
				dir = ip
			}
			if fd := g.rt.methods[dir+"|"+name+"."+e.Sel.Name]; fd != nil {
				return fd, g.rt.methodOf[fd]
			}
			return nil, nil
		}
		if dir := g.dirOf(sc.file.imports[x.Name]); dir != "" {
			if fd := g.funcs[dir][e.Sel.Name]; fd != nil {
				return fd, g.funcFile[fd]
			}
		}
	}
	return nil, nil
}

// bind is the routers a call passes to a function's parameters.
func (sc *routeScope) bind(fd *ast.FuncDecl, f *goFile, call *ast.CallExpr) map[int]*routerNode {
	bound := map[int]*routerNode{}
	for i, a := range call.Args {
		if n := sc.eval(a); n != nil {
			bound[i] = n
		}
	}
	return bound
}

// call reads one call: a registration, a mount, a check applied to a
// router, or a call of the module's function that is passed a router.
func (sc *routeScope) call(call *ast.CallExpr) bool {
	g := sc.g
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if ok {
		if x, ok := sel.X.(*ast.Ident); ok && sc.file.imports[x.Name] == "net/http" && sc.env[x.Name] == nil && (sel.Sel.Name == "Handle" || sel.Sel.Name == "HandleFunc") {
			if g.rt.deflt == nil {
				g.rt.deflt = &routerNode{lib: "net/http"}
			}
			sc.register(g.rt.deflt, call, sel.Sel.Name)
			return false
		}
		if recv := sc.eval(sel.X); recv != nil {
			switch {
			case sel.Sel.Name == "Use":
				recv.checks = append(recv.checks, sc.checksIn(call.Args)...)
				return true
			case recv.lib == "chi" && (sel.Sel.Name == "Route" || sel.Sel.Name == "Group"):
				sc.eval(call)
				return false
			case recv.lib == "chi" && sel.Sel.Name == "Mount" && len(call.Args) == 2:
				sc.mount(recv, call, call.Args[0], call.Args[1], "Mount")
				return true
			}
			if sc.register(recv, call, sel.Sel.Name) {
				return false
			}
		}
		if g.gorillaEnd(sel.Sel.Name) {
			if chain, recv := sc.chain(call); recv != nil && recv.lib == "gorilla/mux" {
				sc.gorilla(recv, chain)
				return false
			}
		}
	}
	if fd, f := sc.callee(call); fd != nil {
		bound := sc.bind(fd, f, call)
		if len(bound) > 0 || !g.hasRouterParam(f, fd) {
			g.walkFunc(f, fd, bound, false)
		}
	}
	return true
}

// mount places a router under another's prefix: chi's Mount, or a
// ServeMux pattern that ends in a slash and hands the request to a router.
func (sc *routeScope) mount(recv *routerNode, call *ast.CallExpr, prefix, h ast.Expr, how string) {
	g := sc.g
	clause := g.clause(sc.file, call.Pos())
	strip := ""
	if c, ok := h.(*ast.CallExpr); ok && len(c.Args) == 2 {
		if s, ok := c.Fun.(*ast.SelectorExpr); ok && exprText(s) == sc.netHTTP()+".StripPrefix" {
			if p, ok := g.stringValue(sc.file, c.Args[0]); ok {
				strip, h = p, c.Args[1]
			}
		}
	}
	target := sc.eval(h)
	under := exprText(prefix)
	if p, ok := g.stringValue(sc.file, prefix); ok {
		under = p
	}
	if target == nil {
		g.question("must", fmt.Sprintf("%s: %s hands every request under %s to %s, which the reader does not read as a router. Which routes does it serve?", clause, how, under, exprText(h)),
			[]string{"paths"}, "Syntax follows a router the module makes; a handler of another kind may serve any path below the prefix.", g.at(clause, how+" hands requests to "+exprText(h)+"."))
		sc.g.citeFile(sc.file)
		return
	}
	sc.g.citeFile(sc.file)
	if target.mounted != "" {
		g.question("must", fmt.Sprintf("%s: %s mounts a router that %s mounts too. Under which path does the running system serve its routes?", clause, how, target.mounted),
			[]string{"paths"}, "A router mounted twice serves its routes under both prefixes, and the reader writes one.", g.at(clause, how+" mounts "+exprText(h)+" a second time."))
		return
	}
	target.mounted = clause
	target.parent = recv
	if how == "Handle" {
		target.prefix = strip
		if strip != "" {
			if p, why := g.routePath(recv.lib, strip); why == "" {
				target.prefix = p
			} else {
				target.unknown = fmt.Sprintf("%s: StripPrefix is given %s: %s", clause, strip, why)
			}
		}
		return
	}
	sc.prefix(target, prefix, how)
}

func (sc *routeScope) netHTTP() string {
	if n := sc.file.importName("net/http"); n != "" {
		return n
	}
	return "http"
}

func (g *goReader) gorillaEnd(name string) bool {
	switch name {
	case "Methods", "HandlerFunc", "Handler", "HandleFunc", "Handle", "Name", "Queries", "Headers", "Schemes", "Host", "Path":
		return true
	}
	return false
}

// gorilla reads one gorilla/mux route: HandleFunc(path, h).Methods(...), or
// Path(path).Methods(...).HandlerFunc(h), with any of them in any order.
func (sc *routeScope) gorilla(recv *routerNode, chain []*ast.CallExpr) {
	g := sc.g
	clause := g.clause(sc.file, chain[0].Pos())
	g.rt.count++
	sc.g.citeFile(sc.file)
	var pathExpr, handler ast.Expr
	var methods []ast.Expr
	methodsSet := false
	other := ""
	for _, c := range chain {
		s := c.Fun.(*ast.SelectorExpr)
		switch s.Sel.Name {
		case "HandleFunc", "Handle":
			if len(c.Args) == 2 {
				pathExpr, handler = c.Args[0], c.Args[1]
			}
		case "Path":
			if len(c.Args) == 1 {
				pathExpr = c.Args[0]
			}
		case "HandlerFunc", "Handler":
			if len(c.Args) == 1 {
				handler = c.Args[0]
			}
		case "Methods":
			methods, methodsSet = c.Args, true
		case "Name":
		default:
			other = s.Sel.Name
		}
	}
	if pathExpr == nil || handler == nil {
		return
	}
	if other != "" {
		p := exprText(pathExpr)
		if v, ok := g.stringValue(sc.file, pathExpr); ok {
			p = v
		}
		g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: the route %s matches by %s as well as by method and path, which the meta-model does not hold; left out", clause, p, other), clause: clause, blocks: []string{"paths"}})
		return
	}
	if !methodsSet && recv.methods != nil {
		for _, m := range recv.methods {
			methods = append(methods, &ast.BasicLit{Kind: gotoken.STRING, Value: strconv.Quote(m), ValuePos: chain[0].Pos()})
		}
		methodsSet = true
	}
	if !methodsSet {
		sc.anyMethod(clause, pathExpr)
		return
	}
	sc.record(recv, chain[0], clause, methods, pathExpr, handler, nil)
}

// register reads a registration call on a router of a library the reader
// knows, and says whether it was one.
func (sc *routeScope) register(recv *routerNode, call *ast.CallExpr, name string) bool {
	g := sc.g
	clause := g.clause(sc.file, call.Pos())
	args := call.Args
	var methods []ast.Expr
	var pathExpr, handler ast.Expr
	var middleware []ast.Expr
	switch recv.lib {
	case "net/http":
		if name != "Handle" && name != "HandleFunc" || len(args) != 2 {
			return false
		}
		g.rt.count++
		g.citeFile(sc.file)
		return sc.servemux(recv, call, clause, args[0], args[1])
	case "chi":
		switch {
		case chiMethods[name] != "" && len(args) == 2:
			methods = []ast.Expr{&ast.BasicLit{Kind: gotoken.STRING, Value: strconv.Quote(chiMethods[name]), ValuePos: call.Pos()}}
			pathExpr, handler = args[0], args[1]
		case (name == "Method" || name == "MethodFunc") && len(args) == 3:
			methods, pathExpr, handler = args[:1], args[1], args[2]
		case (name == "Handle" || name == "HandleFunc") && len(args) == 2:
			g.rt.count++
			g.citeFile(sc.file)
			sc.anyMethod(clause, args[0])
			return true
		default:
			return false
		}
	case "gin", "echo":
		switch {
		case capitalMethods[name] && len(args) >= 2:
			methods = []ast.Expr{&ast.BasicLit{Kind: gotoken.STRING, Value: strconv.Quote(name), ValuePos: call.Pos()}}
			pathExpr = args[0]
		case (name == "Handle" && recv.lib == "gin" || name == "Add" && recv.lib == "echo") && len(args) >= 3:
			methods, pathExpr, args = args[:1], args[1], args[1:]
		case name == "Match" && len(args) >= 3:
			list, ok := args[0].(*ast.CompositeLit)
			if !ok {
				g.rt.count++
				g.citeFile(sc.file)
				g.question("must", fmt.Sprintf("%s registers a route with Match on methods %s, which is not a literal list. Which methods and path does it register?", clause, exprText(args[0])),
					[]string{"paths"}, "A method computed at run time is a guess for syntax; an operation is never written from a guess.", g.at(clause, "Calls Match with methods that are not a literal list."))
				return true
			}
			methods, pathExpr, args = list.Elts, args[1], args[1:]
		case name == "Any" && len(args) >= 2:
			g.rt.count++
			g.citeFile(sc.file)
			sc.anyMethod(clause, args[0])
			return true
		default:
			return false
		}
		rest := args[1:]
		if recv.lib == "gin" {
			handler, middleware = rest[len(rest)-1], rest[:len(rest)-1]
		} else {
			handler, middleware = rest[0], rest[1:]
		}
	default:
		return false
	}
	g.rt.count++
	g.citeFile(sc.file)
	sc.record(recv, call, clause, methods, pathExpr, handler, middleware)
	return true
}

// servemux reads a ServeMux pattern, "[METHOD ][HOST]/[PATH]".
func (sc *routeScope) servemux(recv *routerNode, call *ast.CallExpr, clause string, patternExpr, handler ast.Expr) bool {
	g := sc.g
	if sc.branched(clause) {
		return true
	}
	pattern, ok := g.stringValue(sc.file, patternExpr)
	if !ok {
		sc.unread(clause, "pattern "+exprText(patternExpr))
		return true
	}
	method, rest := "", strings.TrimSpace(pattern)
	if i := strings.IndexAny(rest, " \t"); i > 0 && !strings.Contains(rest[:i], "/") {
		method, rest = rest[:i], strings.TrimLeft(rest[i:], " \t")
	}
	slash := strings.Index(rest, "/")
	if slash < 0 {
		sc.unread(clause, "pattern "+strconv.Quote(pattern))
		return true
	}
	if host := rest[:slash]; host != "" {
		g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: the pattern %s matches the host %s as well as the path, which the meta-model does not hold; left out", clause, pattern, host), clause: clause, blocks: []string{"paths"}})
		return true
	}
	p := rest[slash:]
	if method == "" && strings.HasSuffix(p, "/") && p != "/" {
		sc.mount(recv, call, patternExpr, handler, "Handle")
		return true
	}
	if method == "" {
		sc.anyMethod(clause, patternExpr)
		return true
	}
	lit := &ast.BasicLit{Kind: gotoken.STRING, Value: strconv.Quote(p), ValuePos: patternExpr.Pos()}
	sc.record(recv, call, clause, []ast.Expr{&ast.BasicLit{Kind: gotoken.STRING, Value: strconv.Quote(method), ValuePos: call.Pos()}}, lit, handler, nil)
	return true
}

// unread asks about a registration whose path the reader does not read.
func (sc *routeScope) unread(clause, what string) {
	g := sc.g
	g.question("must", fmt.Sprintf("%s registers a route whose %s the reader does not read as a literal. Which method and path does it register?", clause, what),
		[]string{"paths"}, "A path or a method computed at run time is a guess for syntax; an operation is never written from a guess.", g.at(clause, "Registers a route with the "+what+"."))
}

// anyMethod asks about a route registered for every method.
func (sc *routeScope) anyMethod(clause string, pathExpr ast.Expr) {
	g := sc.g
	if sc.branched(clause) {
		return
	}
	p := exprText(pathExpr)
	if s, ok := g.stringValue(sc.file, pathExpr); ok {
		p = s
	}
	g.question("must", fmt.Sprintf("%s registers %s for every method. Which methods does the running system answer there, and what does each do?", clause, p),
		[]string{"paths"}, "A route registered for every method answers any method, and an operation has one; writing one per method would write operations the code may refuse.", g.at(clause, "Registers "+p+" for every method."))
}

// record writes one route per method a registration names.
// branched asks about a registration in a loop or behind a condition, and
// says whether it is one.
func (sc *routeScope) branched(clause string) bool {
	g := sc.g
	b := branch(sc.stack)
	if b == nil {
		return false
	}
	g.question("must", fmt.Sprintf("%s registers a route %s at %s, so the reader cannot tell how many routes it registers, or whether it registers one. Which routes does the running system register here?", clause, branchWord(b), g.clause(sc.file, b.Pos())),
		[]string{"paths"}, "Syntax does not run the code; only the running system's own list of routes, such as a route table, says what a loop or a condition registers.", g.at(clause, "Registers a route "+branchWord(b)+"."))
	return true
}

func (sc *routeScope) record(recv *routerNode, call ast.Node, clause string, methods []ast.Expr, pathExpr, handler ast.Expr, middleware []ast.Expr) {
	g := sc.g
	if sc.branched(clause) {
		return
	}
	p, ok := g.stringValue(sc.file, pathExpr)
	if !ok {
		sc.unread(clause, "path "+exprText(pathExpr))
		return
	}
	var list []string
	for _, m := range methods {
		v, ok := g.methodValue(sc.file, m)
		if !ok {
			sc.unread(clause, "method "+exprText(m))
			return
		}
		list = append(list, strings.ToUpper(v))
	}
	conv, why, notes := g.routePathNotes(recv.lib, p)
	if why != "" {
		g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: the route %s: %s; left out", clause, p, why), clause: clause, blocks: []string{"paths"}})
		return
	}
	for _, n := range notes {
		g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: the route %s: %s", clause, p, n), clause: clause, blocks: []string{"paths"}})
	}
	checks := recv.chainChecks()
	var mw []string
	checks = append(checks, sc.checksIn(middleware)...)
	for _, m := range middleware {
		mw = append(mw, exprText(m))
	}
	h, wrapped := sc.unwrap(handler)
	checks = append(checks, wrapped...)
	for _, m := range list {
		r := &goRoute{file: sc.file, clause: clause, lib: recv.lib, node: recv, method: m, path: conv, handler: exprText(h), middleware: mw, checks: append([]checkUse{}, checks...)}
		switch h := h.(type) {
		case *ast.Ident:
			r.name = h.Name
		case *ast.SelectorExpr:
			r.name = h.Sel.Name
		case *ast.FuncLit:
			r.handler = "a function literal"
		}
		sc.handlerBody(r, h)
		g.rt.routes = append(g.rt.routes, r)
	}
}

// unwrap takes the named checks and net/http's HandlerFunc off a handler,
// and gives the handler they wrap and the checks.
func (sc *routeScope) unwrap(h ast.Expr) (ast.Expr, []checkUse) {
	var checks []checkUse
	for {
		call, ok := h.(*ast.CallExpr)
		if !ok {
			return h, checks
		}
		if c, ok := sc.checkCall(call); ok {
			checks = append(checks, c)
			n := sc.checkArgument(call)
			if len(call.Args) == 0 || len(call.Args)-1 == n {
				return h, checks
			}
			h = call.Args[len(call.Args)-1]
			continue
		}
		if s, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) == 1 && (exprText(s) == sc.netHTTP()+".HandlerFunc") {
			h = call.Args[0]
			continue
		}
		return h, checks
	}
}

// checksIn is the named checks among middleware.
func (sc *routeScope) checksIn(list []ast.Expr) []checkUse {
	var out []checkUse
	for _, e := range list {
		if call, ok := e.(*ast.CallExpr); ok {
			if c, ok := sc.checkCall(call); ok {
				out = append(out, c)
			}
		}
	}
	return out
}

// checkCall says whether a call is one of the named checks, and reads its
// permission.
func (sc *routeScope) checkCall(call *ast.CallExpr) (checkUse, bool) {
	pkg, name := sc.target(call.Fun)
	if name == "" {
		return checkUse{}, false
	}
	for _, c := range sc.g.rt.checks {
		if c.pkg != pkg || c.function != name {
			continue
		}
		clause := sc.g.clause(sc.file, call.Pos())
		use := checkUse{clause: clause, check: exprText(call.Fun)}
		if c.argument > len(call.Args) {
			use.argument = "nothing"
			return use, true
		}
		arg := call.Args[c.argument-1]
		use.argument = exprText(arg)
		if p, ok := sc.g.stringValue(sc.file, arg); ok {
			use.permission, use.argument = p, strconv.Quote(p)
		}
		sc.g.citeFile(sc.file)
		return use, true
	}
	return checkUse{}, false
}

// checkArgument is the place of a check call's permission, from 0.
func (sc *routeScope) checkArgument(call *ast.CallExpr) int {
	pkg, name := sc.target(call.Fun)
	for _, c := range sc.g.rt.checks {
		if c.pkg == pkg && c.function == name {
			return c.argument - 1
		}
	}
	return -1
}

// target is the package and the name, Function or Type.Method, a call's
// function has by syntax.
func (sc *routeScope) target(fn ast.Expr) (string, string) {
	switch fn := fn.(type) {
	case *ast.Ident:
		return sc.g.pkgPath(path.Dir(sc.file.path)), fn.Name
	case *ast.SelectorExpr:
		x, ok := fn.X.(*ast.Ident)
		if !ok {
			return "", ""
		}
		if t := sc.types[x.Name]; t != "" {
			ip, name := splitKey(t)
			return ip, name + "." + fn.Sel.Name
		}
		if ip := sc.file.imports[x.Name]; ip != "" {
			return ip, fn.Sel.Name
		}
	}
	return "", ""
}

// handlerBody reads the handler a route names: the path parameters it
// reads, the body it decodes, and the named checks it calls.
func (sc *routeScope) handlerBody(r *goRoute, h ast.Expr) {
	g := sc.g
	var ft *ast.FuncType
	var body *ast.BlockStmt
	hs := sc
	switch h := h.(type) {
	case *ast.FuncLit:
		ft, body = h.Type, h.Body
	default:
		fd, f := sc.funcValue(h)
		if fd == nil || fd.Body == nil {
			return
		}
		ft, body = fd.Type, fd.Body
		hs = g.newScope(f)
		if fd.Recv != nil && len(fd.Recv.List) == 1 && len(fd.Recv.List[0].Names) == 1 {
			hs.types[fd.Recv.List[0].Names[0].Name] = g.typeKey(f, fd.Recv.List[0].Type)
		}
	}
	f := hs.file
	f.cited = true
	r.hFile, r.from, r.to = f, body.Pos(), body.End()
	r.handlerAt = g.clause(f, ft.Pos())
	if _, lit := h.(*ast.FuncLit); lit {
		r.handlerAt = g.clause(f, body.Pos())
	}
	request, context := "", ""
	for _, p := range ft.Params.List {
		t := exprText(p.Type)
		for _, n := range p.Names {
			hs.types[n.Name] = g.typeKey(f, p.Type)
			switch {
			case t == "*"+hs.netHTTP()+".Request":
				request = n.Name
			case strings.HasSuffix(t, ".Context") && (routerLibraries[f.imports[strings.TrimPrefix(strings.TrimSuffix(t, ".Context"), "*")]] == "gin" || routerLibraries[f.imports[strings.TrimSuffix(t, ".Context")]] == "echo"):
				context = n.Name
			}
		}
	}
	locals := map[string]ast.Expr{} // a name -> the type it is declared with
	varsOf := map[string]bool{}     // names that hold mux.Vars(request)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ValueSpec:
			if n.Type != nil {
				for _, id := range n.Names {
					locals[id.Name] = n.Type
				}
			}
		case *ast.AssignStmt:
			if len(n.Lhs) == 1 && len(n.Rhs) == 1 {
				id, ok := n.Lhs[0].(*ast.Ident)
				if !ok {
					break
				}
				rhs := n.Rhs[0]
				if u, ok := rhs.(*ast.UnaryExpr); ok && u.Op == gotoken.AND {
					rhs = u.X
				}
				switch v := rhs.(type) {
				case *ast.CompositeLit:
					if v.Type != nil {
						locals[id.Name] = v.Type
					}
				case *ast.CallExpr:
					if fn, ok := v.Fun.(*ast.Ident); ok && fn.Name == "new" && len(v.Args) == 1 {
						locals[id.Name] = v.Args[0]
					}
					if s, ok := v.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "Vars" && routerLibraries[f.imports[exprText(s.X)]] == "gorilla/mux" {
						varsOf[id.Name] = true
					}
				}
				if t := g.valueType(f, n.Rhs[0]); t != "" {
					hs.types[id.Name] = t
				}
			}
		}
		return true
	})
	seen := map[string]bool{}
	read := func(name string, call ast.Node, how string) {
		clause := g.clause(f, call.Pos())
		if !seen[name] {
			seen[name] = true
			r.reads = append(r.reads, paramRead{name: name, clause: clause})
		}
	}
	unread := func(call ast.Node, arg ast.Expr, how string) {
		clause := g.clause(f, call.Pos())
		g.question("should", fmt.Sprintf("%s: the handler of %s %s reads a path parameter whose name %s is not a literal. Which parameter does it read?", clause, r.method, r.path, exprText(arg)),
			[]string{"paths"}, "Syntax cannot tell which name a computed value holds.", g.at(clause, "Calls "+how+" with a name that is not a literal."))
	}
	ast.Inspect(body, func(n ast.Node) bool {
		if ix, ok := n.(*ast.IndexExpr); ok {
			fromVars := false
			if id, ok := ix.X.(*ast.Ident); ok && varsOf[id.Name] {
				fromVars = true
			}
			if c, ok := ix.X.(*ast.CallExpr); ok {
				if s, ok := c.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "Vars" && routerLibraries[f.imports[exprText(s.X)]] == "gorilla/mux" {
					fromVars = true
				}
			}
			if fromVars {
				if p, ok := g.stringValue(f, ix.Index); ok {
					read(p, ix, "mux.Vars")
				} else {
					unread(ix, ix.Index, "mux.Vars")
				}
			}
			return true
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if c, ok := hs.checkCall(call); ok {
			r.checks = append(r.checks, c)
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		x := exprText(sel.X)
		switch {
		case request != "" && x == request && sel.Sel.Name == "PathValue" && len(call.Args) == 1,
			context != "" && x == context && sel.Sel.Name == "Param" && len(call.Args) == 1:
			if p, ok := g.stringValue(f, call.Args[0]); ok {
				read(p, call, sel.Sel.Name)
			} else {
				unread(call, call.Args[0], x+"."+sel.Sel.Name)
			}
		case (sel.Sel.Name == "URLParam" || sel.Sel.Name == "URLParamFromCtx") && routerLibraries[f.imports[x]] == "chi" && len(call.Args) == 2:
			if p, ok := g.stringValue(f, call.Args[1]); ok {
				read(p, call, "chi."+sel.Sel.Name)
			} else {
				unread(call, call.Args[1], "chi."+sel.Sel.Name)
			}
		case sel.Sel.Name == "Decode" && len(call.Args) == 1 && request != "":
			dec, ok := sel.X.(*ast.CallExpr)
			if !ok || len(dec.Args) != 1 || exprText(dec.Fun) != f.importName("encoding/json")+".NewDecoder" || exprText(dec.Args[0]) != request+".Body" {
				break
			}
			sc.bodyInto(r, f, call, call.Args[0], locals)
		case context != "" && x == context && (sel.Sel.Name == "ShouldBindJSON" || sel.Sel.Name == "BindJSON" || sel.Sel.Name == "ShouldBind" || sel.Sel.Name == "Bind") && len(call.Args) == 1:
			sc.bodyInto(r, f, call, call.Args[0], locals)
		}
		return true
	})
}

// bodyInto notes the value a handler decodes its request body into.
func (sc *routeScope) bodyInto(r *goRoute, f *goFile, call *ast.CallExpr, arg ast.Expr, locals map[string]ast.Expr) {
	r.bodies++
	b := &bodyRead{clause: sc.g.clause(f, call.Pos()), file: f, text: exprText(arg)}
	if u, ok := arg.(*ast.UnaryExpr); ok && u.Op == gotoken.AND {
		arg = u.X
		b.text = exprText(arg)
	}
	if id, ok := arg.(*ast.Ident); ok {
		b.typ = locals[id.Name]
	}
	if r.body == nil {
		r.body = b
	}
}

// routePath writes a route's path in the {name} form, as each library
// writes its parameters, or says why the meta-model cannot hold it.
func (g *goReader) routePath(lib, p string) (string, string) {
	conv, why, _ := g.routePathNotes(lib, p)
	return conv, why
}

// routePathNotes is routePath with what it leaves out of a path it writes:
// a parameter's pattern.
func (g *goReader) routePathNotes(lib, p string) (string, string, []string) {
	var notes []string
	if p == "" {
		return "", "", nil
	}
	segs := strings.Split(p, "/")
	for i, s := range segs {
		switch lib {
		case "gin", "echo":
			if m := colonParam.FindStringSubmatch(s); m != nil {
				segs[i] = "{" + m[1] + "}"
				continue
			}
			if strings.HasPrefix(s, "*") {
				return "", "it ends in the catch-all " + s + ", which matches any number of segments, and a path parameter holds one", nil
			}
			if strings.ContainsAny(s, ":*") {
				return "", "the segment " + s + " joins a parameter to text, which a path of the meta-model does not hold", nil
			}
		case "net/http":
			if s == "{$}" && i == len(segs)-1 {
				segs[i] = ""
				continue
			}
			if strings.HasSuffix(s, "...}") {
				return "", "it ends in the wildcard " + s + ", which matches the rest of the path, and a path parameter holds one segment", nil
			}
		default: // chi and gorilla/mux
			if s == "*" || strings.HasSuffix(s, "*") {
				return "", "it ends in the catch-all " + s + ", which matches the rest of the path, and a path parameter holds one segment", nil
			}
			if m := routeWildcard.FindStringSubmatch(s); m != nil && m[0] == s {
				if m[2] != "" {
					notes = append(notes, "the parameter "+m[1]+" matches the pattern "+strings.TrimPrefix(m[2], ":")+", which is not written as its schema, since the router's pattern syntax is not JSON Schema's")
				}
				segs[i] = "{" + m[1] + "}"
			}
		}
	}
	return strings.Join(segs, "/"), "", notes
}

// joinPath joins a prefix and a path: one slash between them, and none at
// the end of a joined path.
func joinPath(prefix, p string) string {
	switch {
	case prefix == "":
		return p
	case p == "" || p == "/":
		return strings.TrimSuffix(prefix, "/")
	}
	return strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(p, "/")
}

// citeFile marks a file as giving something read.
func (g *goReader) citeFile(f *goFile) { f.cited = true }

// readClients reads every call of net/http's client with a literal method
// and URL as a call of another system.
func (g *goReader) readClients() {
	n := 0
	for _, f := range g.files {
		httpName := f.importName("net/http")
		if httpName == "" {
			continue
		}
		ast.Inspect(f.ast, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || exprText(sel.X) != httpName {
				return true
			}
			spec, ok := clientCalls[sel.Sel.Name]
			if !ok || len(call.Args) <= spec.url {
				return true
			}
			n++
			f.cited = true
			clause := g.clause(f, call.Pos())
			c := clientCall{clause: clause, file: f, pos: call.Pos(), method: spec.named}
			if spec.method >= 0 {
				m, ok := g.methodValue(f, call.Args[spec.method])
				if !ok {
					g.question("should", fmt.Sprintf("%s calls another system with %s, by the method %s, which is not a literal. Which method does it call?", clause, sel.Sel.Name, exprText(call.Args[spec.method])),
						[]string{"dependencies"}, "A method computed at run time is not known by syntax.", g.at(clause, "Calls "+sel.Sel.Name+" with a method that is not a literal."))
					return true
				}
				c.method = strings.ToUpper(m)
			}
			u, ok := g.stringValue(f, call.Args[spec.url])
			if !ok {
				g.question("should", fmt.Sprintf("%s calls another system with %s at a URL that is not a literal. Which system and operation does it call?", clause, sel.Sel.Name),
					[]string{"dependencies"}, "A URL computed at run time is not known by syntax, and a dependency is named by the system it calls.", g.at(clause, "Calls "+sel.Sel.Name+" with a URL that is not a literal."))
				return true
			}
			parsed, err := url.Parse(u)
			if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
				g.question("should", fmt.Sprintf("%s calls %s %s, which names no host. Which system does it call?", clause, c.method, u),
					[]string{"dependencies"}, "A dependency is named by the system it calls, and the URL does not name one.", g.at(clause, fmt.Sprintf("Calls %s %s.", c.method, u)))
				return true
			}
			c.dependency = camel(strings.ToLower(hostWord.ReplaceAllString(parsed.Hostname(), "_")))
			c.says = fmt.Sprintf("Calls %s %s with %s.", c.method, u, sel.Sel.Name)
			g.rt.clients = append(g.rt.clients, c)
			return true
		})
	}
	if n > 0 {
		g.res.say("clients: counted %s: every call of net/http's Get, Head, Post, PostForm, NewRequest and NewRequestWithContext", plural(n, "call"))
	}
}

// checkDecl is the declaration of a named check, found by syntax.
func (g *goReader) checkDecl(c permissionCheck) (*ast.FuncDecl, *goFile) {
	dir := g.dirOf(c.pkg)
	if dir == "" && g.modPath == "" {
		dir = c.pkg
	}
	if dir == "" {
		return nil, nil
	}
	if fd := g.funcs[dir][c.function]; fd != nil {
		return fd, g.funcFile[fd]
	}
	if fd := g.rt.methods[dir+"|"+c.function]; fd != nil {
		return fd, g.rt.methodOf[fd]
	}
	return nil, nil
}

// gatesInCheck finds a gate on a setting in a named check: in its own
// statements, or in those of the function it returns, an if statement that
// lets the request through while a setting is empty or false.
func (g *goReader) gatesInCheck(f *goFile, fd *ast.FuncDecl) {
	lists := [][]ast.Stmt{fd.Body.List}
	names := paramNames(fd.Type)
	for _, st := range fd.Body.List {
		ret, ok := st.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}
		e := ret.Results[0]
		if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 1 {
			e = c.Args[0]
		}
		if lit, ok := e.(*ast.FuncLit); ok {
			lists = append(lists, lit.Body.List)
			names = append(names, paramNames(lit.Type)...)
		}
	}
	through := func(body *ast.BlockStmt) bool {
		switch len(body.List) {
		case 1:
			ret, ok := body.List[0].(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				return false
			}
			switch r := ret.Results[0].(type) {
			case *ast.Ident:
				return r.Name == "nil" || r.Name == "true" || contains(names, r.Name)
			case *ast.CallExpr:
				id, ok := r.Fun.(*ast.Ident)
				return ok && contains(names, id.Name)
			}
		case 2:
			ret, ok := body.List[1].(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 0 {
				return false
			}
			es, ok := body.List[0].(*ast.ExprStmt)
			if !ok {
				return false
			}
			call, ok := es.X.(*ast.CallExpr)
			if !ok {
				return false
			}
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				return contains(names, fn.Name)
			case *ast.SelectorExpr:
				x, ok := fn.X.(*ast.Ident)
				return ok && contains(names, x.Name) && (fn.Sel.Name == "ServeHTTP" || fn.Sel.Name == "Next")
			}
		}
		return false
	}
	for _, list := range lists {
		g.gatesInList(f, fd, list, through)
	}
}

func paramNames(ft *ast.FuncType) []string {
	var out []string
	if ft.Params == nil {
		return nil
	}
	for _, p := range ft.Params.List {
		for _, n := range p.Names {
			out = append(out, n.Name)
		}
	}
	return out
}

// writeRoutes writes the routes read as operations into the path items,
// and notes the permissions they check.
func (g *goReader) writeRoutes(items map[string]*yaml.Node, uris *[]string, checkedBy map[string][]string, firstAt, firstVia map[string]string) {
	st := g.rt
	if len(st.routes) == 0 {
		return
	}
	// Where every route goes, and its name, as extract router names one.
	var held []*goRoute
	taken := map[string]*goRoute{}
	for _, ep := range g.endpoints {
		taken[ep.method+" "+ep.uri] = nil
	}
	for _, r := range st.routes {
		prefix, unknown := r.node.place()
		label := r.method + " " + r.path
		if unknown != "" {
			g.question("must", fmt.Sprintf("%s registers %s on a router whose place the reader does not know (%s). Under which path does the running system serve it?", r.clause, label, unknown),
				[]string{"paths"}, "A prefix computed at run time, or a router passed in from code the reader does not follow, is a guess for syntax; an operation's path is never written from a guess.", g.at(r.clause, "Registers "+label+"."))
			continue
		}
		r.full = joinPath(prefix, r.path)
		label = r.method + " " + r.full
		if !strings.HasPrefix(r.full, "/") {
			g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: the route %s: its path does not start with /; left out", r.clause, label), clause: r.clause, blocks: []string{"paths"}})
			continue
		}
		if !heldMethods[r.method] {
			g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: the route %s: the method %s has no operation in the meta-model, which holds GET, POST, PUT, PATCH and DELETE; left out", r.clause, label, r.method), clause: r.clause, blocks: []string{"paths"}})
			continue
		}
		params, reason := pathParameters(r.full)
		if reason != "" {
			g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: the route %s: %s; left out", r.clause, label, reason), clause: r.clause, blocks: []string{"paths"}})
			continue
		}
		if other := taken[r.method+" "+r.full]; other != nil && other.clause == r.clause {
			// One registration reached through two callers, such as the
			// service's own mux and a route table printer's recorder.
			continue
		}
		if other, dup := taken[r.method+" "+r.full]; dup {
			where := "a NewEndPoint call"
			if other != nil {
				where = other.clause
			}
			g.question("must", fmt.Sprintf("%s registers %s, which %s registers too. Which one does the running system serve?", r.clause, label, where),
				[]string{"#/paths/" + escapeToken(r.full) + "/" + strings.ToLower(r.method), "paths"}, "A router serves one handler for a method and path: the one registered first or last, by library, or it stops at the second.", g.at(r.clause, "Registers "+label+" a second time."))
			continue
		}
		taken[r.method+" "+r.full] = r
		r.params, r.key = params, strings.ToLower(r.method)
		held = append(held, r)
	}
	count := map[string]int{}
	for _, r := range held {
		if handlerName.MatchString(r.name) {
			count[r.name]++
		}
	}
	for _, r := range held {
		if handlerName.MatchString(r.name) && count[r.name] == 1 {
			r.id = strings.ToLower(r.name[:1]) + r.name[1:]
			continue
		}
		r.id = methodPathName(r.key, r.full)
		why := "serves more than one route"
		if !handlerName.MatchString(r.name) {
			why = "has no name an operationId can take"
		}
		g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: the route %s %s: its handler %s %s; the operation is named %s", r.clause, r.method, r.full, r.handler, why, r.id), clause: r.clause, blocks: []string{"#/paths/" + escapeToken(r.full) + "/" + r.key}})
	}
	for _, c := range st.clients {
		for _, r := range held {
			if r.hFile == c.file && c.pos >= r.from && c.pos < r.to && !contains(r.calls, c.dependency) {
				r.calls = append(r.calls, c.dependency)
			}
		}
	}
	snake := g.bodiesSnake(held)
	sort.SliceStable(held, func(i, j int) bool {
		if held[i].full != held[j].full {
			return held[i].full < held[j].full
		}
		return methodRank(held[i].key) < methodRank(held[j].key)
	})
	for _, r := range held {
		at := "#/paths/" + escapeToken(r.full)
		r.at = at + "/" + r.key
		label := r.method + " " + r.full
		item := items[r.full]
		if item == nil {
			item = mapping()
			items[r.full] = item
			*uris = append(*uris, r.full)
			if len(r.params) > 0 {
				var list []*yaml.Node
				var blocks []string
				for i, name := range r.params {
					list = append(list, flow(mapping("name", name, "in", "path", "required", true)))
					blocks = append(blocks, fmt.Sprintf("%s/parameters/%d/schema", at, i))
				}
				set(item, "parameters", list)
				g.question("must", fmt.Sprintf("What values does each path parameter of %s take: %s?", r.full, strings.Join(r.params, ", ")),
					blocks, "A path names its parameters and not the values they take.")
			}
		}
		for _, p := range r.reads {
			if !contains(r.params, p.name) {
				g.question("must", fmt.Sprintf("%s: the handler of %s reads the path parameter %s, which its path does not have, so it reads an empty value. Is %s a parameter of the operation, or is the read a mistake?", p.clause, label, p.name, p.name),
					[]string{r.at, r.at + "/parameters"}, "A path parameter the route's path does not name is always empty, so the code cannot work as written.", g.at(p.clause, "Reads the path parameter "+p.name+"."))
			}
		}
		op := mapping("operationId", r.id)
		permission := g.routePermission(r, label)
		if permission != "" {
			set(op, "permission", permission)
			if _, seen := firstAt[permission]; !seen {
				firstAt[permission] = r.clause
				firstVia[permission] = r.checks[0].check
			}
			checkedBy[permission] = append(checkedBy[permission], label)
		}
		if r.bodies > 1 {
			g.question("should", fmt.Sprintf("The handler of %s decodes its request body %d times. Which shape does the body take?", label, r.bodies),
				[]string{r.at + "/requestBody"}, "A request body is read once; the reader writes the first decoding, and the others may read another shape.", g.at(r.body.clause, "Decodes the request body into "+r.body.text+"."))
		}
		if rb := g.requestBody(r, label, snake); rb != nil {
			set(op, "requestBody", rb)
		}
		if len(r.calls) > 0 {
			set(op, "calls", r.calls)
		}
		set(op, "origin", "stated")
		cites := []*yaml.Node{g.at(r.clause, g.routeSays(r))}
		if r.handlerAt != "" {
			cites = append(cites, g.at(r.handlerAt, g.routeHandlerSays(r, label)))
		}
		set(op, "cites", cites)
		set(item, r.key, op)
		g.question("must", fmt.Sprintf("What does %s do, and what does it answer?", label),
			[]string{r.at + "/summary", r.at + "/responses"}, "The source registers a handler and does not say what it does or the responses it gives.")
		if r.handlerAt == "" {
			g.question("should", fmt.Sprintf("%s's handler %s is not a function the reader finds by syntax in the files read, such as a method of a value whose type it does not know or a function outside the module, so the parameters it reads and the body it decodes are not read. Which does it read?", label, r.handler),
				[]string{r.at}, "Syntax alone finds a function declared in the module and named directly, and no other.", g.at(r.clause, "Its handler is "+r.handler+"."))
		}
		g.question("must", fmt.Sprintf("Does the running system register %s? It is declared at %s, and no list the running system printed was read with it.", label, r.clause),
			[]string{r.at}, "Syntax shows a declaration and not what runs; a route table the running system prints says what it registers, and merging it answers this.", g.at(r.clause, "Registers "+label+"."))
	}
	if snake {
		g.wireSnake = true
	}
	g.writeDependencies(held)
	g.res.say("routes: wrote %s: one operation per route a router of net/http, chi, gin, echo or gorilla/mux registers with a literal method and path, outside a loop or a condition, under a prefix the reader knows", plural(len(held), "operation"))
}

// routePermission is the permission a route checks, or "" with a question.
func (g *goReader) routePermission(r *goRoute, label string) string {
	at := r.at + "/permission"
	if g.rt.checkFile == "" {
		g.question("must", fmt.Sprintf("%s: no implementation file names the project's permission check, so the reader does not know which permission %s checks. Which permission does it check, or is it meant to be open to everyone (public)?", r.clause, label),
			[]string{at}, "A check is a call the project writes its own way; the implementation file names it (bindings.http.permissionChecks), and without it no permission is read.")
		return ""
	}
	var names []string
	for _, c := range r.checks {
		if c.permission == "" {
			g.question("must", fmt.Sprintf("%s: %s checks a permission %s gives as %s, which is not a literal. Which permission does it check?", c.clause, label, c.check, c.argument),
				[]string{at}, "A permission computed at run time is not known by syntax, and an operation's permission is never guessed.", g.at(c.clause, fmt.Sprintf("Calls %s with %s.", c.check, c.argument)))
			return ""
		}
		if !contains(names, c.permission) {
			names = append(names, c.permission)
		}
	}
	switch {
	case len(names) == 0:
		g.question("must", fmt.Sprintf("%s checks no permission: none of the checks the implementation file names wraps its handler, is among its middleware or is called in its handler. Is it meant to be open to everyone (public), or which permission should it check?", label),
			[]string{at}, "An operation open to everyone is how an open endpoint is usually found, so it is asked, never assumed.")
	case len(names) > 1:
		g.question("must", fmt.Sprintf("%s checks the permissions %s, and an operation checks one. Which permission should it check?", label, joinAnd(names)),
			[]string{at}, "An operation names one permission; choosing one of several the code checks would write a check the code does not make.")
	case names[0] == "public" || !permissionWord.MatchString(names[0]):
		g.question("must", fmt.Sprintf("%s checks %s, which is not a permission name (lower-case words joined by dots). Which permission should it check?", label, names[0]),
			[]string{at}, "The meta-model names a permission in lower-case words joined by dots, and public means open to everyone, which a check the code makes is not.")
	default:
		return names[0]
	}
	return ""
}

func (g *goReader) routeSays(r *goRoute) string {
	lib := map[string]string{"net/http": "a net/http ServeMux", "chi": "a chi router", "gin": "a gin router", "echo": "an echo router", "gorilla/mux": "a gorilla/mux router"}[r.lib]
	s := fmt.Sprintf("Registers %s %s on %s; its handler is %s", r.method, r.full, lib, r.handler)
	if len(r.middleware) > 0 {
		s += ", its middleware " + joinAnd(r.middleware)
	}
	var checks []string
	for _, c := range r.checks {
		checks = append(checks, fmt.Sprintf("%s(%s)", c.check, c.argument))
	}
	if len(checks) > 0 {
		s += ", and it is checked by " + joinAnd(checks)
	}
	return s + "."
}

func (g *goReader) routeHandlerSays(r *goRoute, label string) string {
	var parts []string
	var names []string
	for _, p := range r.reads {
		names = append(names, p.name)
	}
	if len(names) > 0 {
		parts = append(parts, "reads the path "+parameterOrParameters(len(names))+" "+joinAnd(names))
	} else {
		parts = append(parts, "reads no path parameter by name")
	}
	if r.body != nil {
		parts = append(parts, "decodes its body into "+r.body.text)
	}
	if len(r.calls) > 0 {
		parts = append(parts, "calls "+joinAnd(r.calls))
	}
	return fmt.Sprintf("The handler of %s %s.", label, joinAnd(parts))
}

// structOf is the struct declaration a type expression names in the
// module, with the file it is in.
func (g *goReader) structOf(f *goFile, e ast.Expr) (*ast.TypeSpec, *goFile) {
	key := g.typeKey(f, e)
	if key == "" {
		return nil, nil
	}
	ip, name := splitKey(key)
	dir := g.dirOf(ip)
	if dir == "" && g.modPath == "" {
		dir = ip
	}
	ts := g.rt.types[dir+"|"+name]
	if ts == nil {
		return nil, nil
	}
	return ts, g.rt.typeFile[ts]
}

// jsonTag is a field's name on the wire and whether encoding/json leaves
// it out; tagged says the tag names it.
func jsonTag(fl *ast.Field, name string) (wire string, skip, tagged bool) {
	if fl.Tag == nil {
		return name, false, false
	}
	tag, err := strconv.Unquote(fl.Tag.Value)
	if err != nil {
		return name, false, false
	}
	v, ok := reflect.StructTag(tag).Lookup("json")
	if !ok {
		return name, false, false
	}
	n := strings.Split(v, ",")[0]
	switch n {
	case "-":
		return "", true, true
	case "":
		return name, false, false
	}
	return n, false, true
}

// bodiesSnake says whether the bodies read name their fields in
// snake_case, as extract openapi decides it for a document: a tagged name
// at least joins words with an underscore, and none has a capital.
func (g *goReader) bodiesSnake(routes []*goRoute) bool {
	underscore, capital := false, false
	seen := map[*ast.TypeSpec]bool{}
	var walk func(f *goFile, e ast.Expr)
	walk = func(f *goFile, e ast.Expr) {
		for {
			switch t := e.(type) {
			case *ast.StarExpr:
				e = t.X
				continue
			case *ast.ArrayType:
				e = t.Elt
				continue
			}
			break
		}
		ts, tf := g.structOf(f, e)
		if ts == nil || seen[ts] {
			return
		}
		seen[ts] = true
		stt, ok := ts.Type.(*ast.StructType)
		if !ok {
			return
		}
		for _, fl := range stt.Fields.List {
			for _, n := range fl.Names {
				wire, skip, tagged := jsonTag(fl, n.Name)
				if skip || !tagged || !n.IsExported() {
					continue
				}
				underscore = underscore || strings.Contains(wire, "_")
				capital = capital || strings.ToLower(wire) != wire
			}
			walk(tf, fl.Type)
		}
	}
	for _, r := range routes {
		if r.body != nil && r.body.typ != nil {
			walk(r.body.file, r.body.typ)
		}
	}
	return underscore && !capital
}

// requestBody writes the body a handler decodes from the struct it decodes
// it into, when the module declares the struct.
func (g *goReader) requestBody(r *goRoute, label string, snake bool) *yaml.Node {
	if r.body == nil {
		return nil
	}
	at := r.at + "/requestBody"
	b := r.body
	if b.typ == nil {
		g.question("should", fmt.Sprintf("%s: the handler of %s decodes its body into %s, whose type the reader does not find in the handler. Which shape does the body take?", b.clause, label, b.text),
			[]string{at}, "Syntax finds a type the handler declares its variable with, and no other.", g.at(b.clause, "Decodes the request body into "+b.text+"."))
		return nil
	}
	schema := g.fieldSchema(b.file, b.typ, at+"/content/application~1json/schema", label+", request body", snake, map[*ast.TypeSpec]bool{}, b.clause)
	if schema == nil {
		return nil
	}
	return mapping("content", mapping("application/json", mapping("schema", schema)))
}

// fieldSchema is the schema a Go type gives on the wire, by encoding/json's
// rules, or nil with a question.
func (g *goReader) fieldSchema(f *goFile, e ast.Expr, at, where string, snake bool, seen map[*ast.TypeSpec]bool, clause string) *yaml.Node {
	ask := func(text, why string) *yaml.Node {
		g.question("should", fmt.Sprintf("%s: %s. Which shape does it take?", where, text), []string{at}, why, g.at(clause, where+": "+text+"."))
		return nil
	}
	switch t := e.(type) {
	case *ast.StarExpr:
		s := g.fieldSchema(f, t.X, at, where, snake, seen, clause)
		if s == nil {
			return nil
		}
		if typ := child(s, "type"); typ != nil && typ.Kind == yaml.ScalarNode {
			nullable := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle, Content: []*yaml.Node{str(typ.Value), str("null")}}
			*typ = *nullable
		}
		return s
	case *ast.ArrayType:
		if t.Len != nil {
			return ask("it is a Go array of fixed length", "A fixed length is not a JSON Schema the reader writes.")
		}
		if exprText(t.Elt) == "byte" {
			return mapping("type", "string", "format", "byte")
		}
		items := g.fieldSchema(f, t.Elt, at+"/items", where, snake, seen, clause)
		if items == nil {
			return nil
		}
		return mapping("type", "array", "items", items)
	case *ast.MapType:
		return ask("it is a map, whose keys and values JSON writes as an object of any properties", "A map has no fixed properties, and the meta-model names each property of an object.")
	case *ast.InterfaceType:
		return ask("it is an interface, which holds any JSON value", "An interface says nothing of the value's shape.")
	case *ast.Ident:
		switch t.Name {
		case "string":
			return mapping("type", "string")
		case "bool":
			return mapping("type", "boolean")
		case "int32", "int16", "int8", "uint8", "uint16", "byte", "rune":
			return mapping("type", "integer", "format", "int32")
		case "uint32":
			return mapping("type", "integer", "format", "int64", "minimum", 0, "maximum", 4294967295)
		case "float64":
			return mapping("type", "number", "format", "double")
		case "int64", "uint64":
			g.question("must", fmt.Sprintf("%s: it is Go's %s, which encoding/json writes as a number, and a number above 2^53 loses digits in JavaScript and other readers. Is it bounded within 2^53, or carried as text?", where, t.Name),
				[]string{at + "/format"}, "Every integer in a specification has a width, and a 64-bit one is held only within 2^53 or as text; which one the code needs is not in the source.", g.at(clause, where+": Go's "+t.Name+"."))
			return mapping("type", "integer")
		case "int", "uint", "uintptr":
			g.question("must", fmt.Sprintf("%s: it is Go's %s, as wide as the platform's word. How wide is it on the wire?", where, t.Name),
				[]string{at + "/format"}, "Every integer in a specification has a width, and Go's int has the platform's.", g.at(clause, where+": Go's "+t.Name+"."))
			return mapping("type", "integer")
		case "float32", "complex64", "complex128", "any":
			return ask("it is Go's "+t.Name+", which the meta-model has no type for", "The meta-model's numbers are integers of a stated width and doubles.")
		}
	case *ast.SelectorExpr:
		if exprText(t) == f.importName("time")+".Time" {
			return mapping("type", "string", "format", "date-time")
		}
	}
	ts, tf := g.structOf(f, e)
	if ts == nil {
		return ask(exprText(e)+" is a type the reader does not find in the module's files", "A type declared outside the files read is not known by syntax.")
	}
	if seen[ts] {
		return ask(ts.Name.Name+" holds itself", "A schema that holds itself has no end the reader can write inline.")
	}
	stt, ok := ts.Type.(*ast.StructType)
	if !ok {
		return g.fieldSchema(tf, ts.Type, at, where, snake, seen, clause)
	}
	seen = copySeen(seen)
	seen[ts] = true
	props := mapping()
	var required []string
	for _, fl := range stt.Fields.List {
		if len(fl.Names) == 0 {
			g.question("should", fmt.Sprintf("%s: %s embeds %s, whose fields encoding/json writes as its own. Which of them does the body take?", where, ts.Name.Name, exprText(fl.Type)),
				[]string{at + "/properties"}, "The reader writes a struct's own fields, and an embedded struct's may be promoted or hidden by Go's rules.", g.at(g.clause(tf, fl.Pos()), ts.Name.Name+" embeds "+exprText(fl.Type)+"."))
			continue
		}
		for _, n := range fl.Names {
			if !n.IsExported() {
				continue
			}
			wire, skip, tagged := jsonTag(fl, n.Name)
			if skip {
				continue
			}
			fclause := g.clause(tf, fl.Pos())
			name := wire
			switch {
			case !tagged && !memberNameWord.MatchString(wire):
				g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: %s.%s has no json tag, so encoding/json writes it as %s, which is neither camelCase nor snake_case; left out", fclause, ts.Name.Name, n.Name, wire), clause: fclause, blocks: []string{at + "/properties"}})
				continue
			case snake:
				c, ok := wireCamel(wire)
				if !ok {
					g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: %s.%s: its name %s does not go back to itself under info.wireNames snake_case; left out", fclause, ts.Name.Name, n.Name, wire), clause: fclause, blocks: []string{at + "/properties"}})
					continue
				}
				name = c
			case !memberNameWord.MatchString(wire):
				g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: %s.%s: its name %s is not camelCase, and the bodies' other names are; a specification names its properties on the wire one way (info.wireNames); left out", fclause, ts.Name.Name, n.Name, wire), clause: fclause, blocks: []string{at + "/properties"}})
				continue
			}
			pat := at + "/properties/" + name
			s := g.fieldSchema(tf, fl.Type, pat, where+", property "+name, snake, seen, fclause)
			if s == nil {
				continue
			}
			if g.validateTags(tf, fl, s, pat, where+", property "+name, fclause) {
				required = append(required, name)
			}
			set(props, name, s)
		}
	}
	out := mapping("type", "object")
	if len(props.Content) > 0 {
		set(out, "properties", props)
	}
	if len(required) > 0 {
		set(out, "required", required)
	}
	return out
}

func copySeen(m map[*ast.TypeSpec]bool) map[*ast.TypeSpec]bool {
	out := map[*ast.TypeSpec]bool{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

// validateTags writes the rules of a validator the module imports into a
// field's schema, and says whether the field is required.
func (g *goReader) validateTags(f *goFile, fl *ast.Field, s *yaml.Node, at, where, clause string) bool {
	if fl.Tag == nil {
		return false
	}
	tag, err := strconv.Unquote(fl.Tag.Value)
	if err != nil {
		return false
	}
	required := false
	for _, key := range []string{"validate", "binding"} {
		v, ok := reflect.StructTag(tag).Lookup(key)
		if !ok || v == "" {
			continue
		}
		if !g.rt.validates[key] {
			g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: %s has the tag %s:%q, and the module imports no validator that reads %s tags; not read", clause, where, key, v, key), clause: clause, blocks: []string{at}})
			continue
		}
		typ := ""
		if t := child(s, "type"); t != nil {
			typ = t.Value
			if t.Kind == yaml.SequenceNode && len(t.Content) > 0 {
				typ = t.Content[0].Value
			}
		}
		for _, rule := range strings.Split(v, ",") {
			name, arg, _ := strings.Cut(rule, "=")
			kind := map[string]string{"string": "Length", "array": "Items"}[typ]
			done := true
			switch name {
			case "omitempty", "":
			case "required":
				required = true
				if t := child(s, "type"); t != nil && t.Kind == yaml.SequenceNode {
					*t = *str(t.Content[0].Value)
				}
			case "min", "max", "len", "gt", "gte", "lt", "lte":
				done = g.boundRule(s, typ, kind, name, arg)
			case "oneof":
				done = oneOf(s, typ, arg)
			case "email", "uuid", "uri", "url", "hostname", "ipv4", "ipv6":
				done = typ == "string"
				if done {
					set(s, "format", validatorFormats[name])
				}
			case "alpha", "alphanum", "numeric":
				done = typ == "string" && child(s, "pattern") == nil
				if done {
					set(s, "pattern", validatorPatterns[name])
				}
			default:
				done = false
			}
			if !done {
				g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: %s: the %s rule %s has no keyword of the meta-model the reader writes it as; not written", clause, where, key, rule), clause: clause, blocks: []string{at}})
			}
		}
	}
	return required
}

// boundRule writes min, max, len, gt, gte, lt and lte: a string's length
// in characters, a list's items, a number's value.
func (g *goReader) boundRule(s *yaml.Node, typ, kind, name, arg string) bool {
	n, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return false
	}
	if kind != "" {
		switch name {
		case "min", "gte":
			set(s, "min"+kind, n)
		case "gt":
			set(s, "min"+kind, n+1)
		case "max", "lte":
			set(s, "max"+kind, n)
		case "lt":
			set(s, "max"+kind, n-1)
		case "len":
			set(s, "min"+kind, n)
			set(s, "max"+kind, n)
		}
		return true
	}
	if typ != "integer" && typ != "number" {
		return false
	}
	switch name {
	case "min", "gte":
		set(s, "minimum", n)
	case "max", "lte":
		set(s, "maximum", n)
	case "gt":
		set(s, "exclusiveMinimum", n)
	case "lt":
		set(s, "exclusiveMaximum", n)
	default:
		return false
	}
	return true
}

// oneOf writes oneof as an enum of strings or of integers.
func oneOf(s *yaml.Node, typ, arg string) bool {
	values := strings.Fields(arg)
	if len(values) == 0 {
		return false
	}
	switch typ {
	case "string":
		set(s, "enum", values)
	case "integer":
		seq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
		for _, v := range values {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return false
			}
			seq.Content = append(seq.Content, value(n))
		}
		set(s, "enum", seq)
	default:
		return false
	}
	return true
}

// wireCamel is the camelCase name of a snake_case wire name, and whether it
// goes back to it.
func wireCamel(wire string) (string, bool) {
	c := camel(wire)
	return c, c != "" && wirename.Snake(c) == wire
}

// writeDependencies declares each system the source calls with a literal
// URL, by its host.
func (g *goReader) writeDependencies(held []*goRoute) {
	st := g.rt
	if len(st.clients) == 0 {
		return
	}
	byName := map[string][]clientCall{}
	var names []string
	for _, c := range st.clients {
		if byName[c.dependency] == nil {
			names = append(names, c.dependency)
		}
		byName[c.dependency] = append(byName[c.dependency], c)
	}
	sort.Strings(names)
	g.deps = &yaml.Node{Kind: yaml.MappingNode}
	for _, n := range names {
		var cites []*yaml.Node
		var outside []string
		for _, c := range byName[n] {
			cites = append(cites, g.at(c.clause, c.says))
			in := false
			for _, r := range held {
				if r.hFile == c.file && c.pos >= r.from && c.pos < r.to {
					in = true
				}
			}
			if !in {
				outside = append(outside, c.clause)
			}
		}
		at := "#/dependencies/" + n
		set(g.deps, n, mapping("origin", "stated", "cites", cites))
		g.question("must", fmt.Sprintf("What is the system %s the source calls, and how long may one call to it take before the operation gives up?", n),
			[]string{at + "/description", at + "/timeout"}, "The source names the URL it calls, and not what the system is or a time limit, which every dependency has.")
		if len(outside) > 0 {
			g.question("should", fmt.Sprintf("Which operations call %s? The source calls it at %s, outside a handler the reader reads.", n, joinAnd(outside)),
				[]string{at}, "An operation names the dependencies it calls, and syntax finds a call only in a handler's own body.", g.at(outside[0], "Calls "+n+"."))
		}
	}
}

// readFlags reads every flag the flag package declares by a literal name
// as a setting.
func (g *goReader) readFlags() {
	kinds := map[string]string{"String": "string", "Bool": "bool", "Int": "int", "Int64": "int", "Uint": "int", "Uint64": "int", "Float64": "number"}
	n := 0
	for _, f := range g.files {
		name := f.importName("flag")
		if name == "" {
			continue
		}
		ast.Inspect(f.ast, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || exprText(sel.X) != name {
				return true
			}
			fn, at := sel.Sel.Name, 0
			if strings.HasSuffix(fn, "Var") {
				fn, at = strings.TrimSuffix(fn, "Var"), 1
			}
			kind, ok := kinds[fn]
			if !ok || len(call.Args) < at+3 {
				return true
			}
			n++
			clause := g.clause(f, call.Pos())
			key, ok := g.stringValue(f, call.Args[at])
			if !ok {
				g.question("should", fmt.Sprintf("%s declares a flag with %s, by the name %s, which is not a literal. Which setting does it declare?", clause, sel.Sel.Name, exprText(call.Args[at])),
					[]string{"configuration"}, "A computed name is not known by syntax.", g.at(clause, "Calls "+sel.Sel.Name+" with a name that is not a literal."))
				return true
			}
			f.cited = true
			r := settingRead{key: key, kind: kind, clause: clause, says: fmt.Sprintf("Declares the flag %s with %s.", key, sel.Sel.Name)}
			r.value = g.literalValue(f, call.Args[at+1], kind)
			g.settings[key] = append(g.settings[key], r)
			return true
		})
	}
	if n > 0 {
		g.res.say("settings: counted %s: every call of the flag package that declares a flag of a string, a boolean or a number", plural(n, "flag"))
	}
}

// loadChecks reads the permission checks an implementation file names
// under bindings.http.permissionChecks.
func (g *goReader) loadChecks(file string) error {
	if file == "" {
		return nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) == 0 {
		return refuse("%s does not parse as YAML", file)
	}
	g.rt.checkFile = file
	list := child(child(child(doc.Content[0], "bindings"), "http"), "permissionChecks")
	if list == nil {
		g.res.say("checks: %s names no permission check under bindings.http.permissionChecks", file)
		return nil
	}
	for i, item := range list.Content {
		pkg, fn := scalar(child(item, "package")), scalar(child(item, "function"))
		n, err := strconv.Atoi(scalar(child(item, "permissionArgument")))
		if pkg == "" || fn == "" || err != nil || n < 1 {
			g.question("must", fmt.Sprintf("The implementation file %s names a permission check (item %d of bindings.http.permissionChecks) without its package, its function and the argument, from 1, that carries the permission. Which check is it?", file, i+1),
				[]string{"paths"}, "A check named in part cannot be found in the source, and the operations it guards would be read as open.")
			continue
		}
		g.rt.checks = append(g.rt.checks, permissionCheck{pkg: pkg, function: fn, argument: n})
	}
	var names []string
	for _, c := range g.rt.checks {
		names = append(names, c.pkg+"."+c.function)
	}
	if len(names) > 0 {
		word := "check"
		if len(names) > 1 {
			word = "checks"
		}
		g.res.say("checks: %s names the permission %s %s", file, word, joinAnd(names))
	}
	return nil
}
