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
	bound      *permissionCheck
	// names are what a gate's argument names as literals: clients,
	// applications, services or an audience; nil when it gives them
	// otherwise, as argument says.
	names []string
	// trailing are the string literals a gate's call passes after the
	// argument that names, which may be more names or something else.
	trailing []string
}

// permissionCheck is one check the implementation file names.
type permissionCheck struct {
	pkg, function string // the function, or Type.Method
	argument      int    // the permission's argument, or the names' for a gate, from 1; 0 for a gate that names none
	bypassedFor   string // the kind of caller the check skips, "" when none (ADR-114)
	gate          string // client, signature, application, caller, audience or event for a check that is not a permission's, "" for one that is (ADR-154)
}

// name is the check's name in a gate's question: the function's, or the
// method's alone, as a gate on a setting names it.
func (c *permissionCheck) name() string {
	return c.function[strings.LastIndex(c.function, ".")+1:]
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
	hDecl      *ast.FuncDecl // the handler, when it is a function of the module
	hLit       *ast.FuncLit  // the handler, when it is a function literal
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
	walked    map[string]*routerNode // a function and what a call bound to it -> what it returns
	seen      map[*ast.FuncDecl]bool
	active    map[*ast.FuncDecl]bool
	litCalled map[*ast.FuncLit]bool // the function literals walked at a call, with its arguments bound
	fields    map[string]*routerNode
	deflt     *routerNode
	routes    []*goRoute
	count     int
	countedAt map[string]bool // each registration counted: its call, router and arguments
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
	st.litCalled, st.countedAt = map[*ast.FuncLit]bool{}, map[string]bool{}
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
				// A function passed to a call as a value is called
				// from where it is passed, which the walk follows.
				for _, a := range call.Args {
					if id, ok := a.(*ast.Ident); ok {
						called[id.Name] = true
					}
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
	// What a name holds besides a router, as a call's arguments bind it:
	// a literal string, or a function value (a literal written in the
	// function walked, or a function of the module).
	strs  map[string]string
	funcs map[string]heldFunc
	// The function literals held in local names, in order; each is walked
	// at every call of its name with the arguments bound, and one no call
	// names is walked once with nothing bound, after the function.
	lits   []*ast.FuncLit
	active map[*ast.FuncLit]bool // the literals being walked, so one that calls itself ends
	// blocks holds, per block being walked, what each function name
	// declared in it held before, so a name declared in an inner block
	// holds its literal only to the end of that block.
	blocks []*walkBlock
	// outer is the loop or condition a function a literal was passed to
	// calls it behind, while the literal is walked in the scope it is
	// written in; outerFile is that function's file.
	outer     ast.Node
	outerFile *goFile
}

// walkBlock is one block being walked: what each name declared in it held
// before, and the function literals declared in it.
type walkBlock struct {
	saved map[string]heldName
	lits  []*ast.FuncLit
}

// heldName is what a name held before a block declared it again.
type heldName struct {
	env                         *routerNode
	typ, str                    string
	fn                          heldFunc
	hadEnv, hadTyp, hadStr, had bool
}

// declare notes that name is declared in the innermost block being
// walked, so what it held before comes back at the block's end.
func (sc *routeScope) declare(name string) {
	if len(sc.blocks) == 0 {
		return
	}
	top := sc.blocks[len(sc.blocks)-1].saved
	if _, ok := top[name]; !ok {
		h := heldName{}
		h.env, h.hadEnv = sc.env[name]
		h.typ, h.hadTyp = sc.types[name]
		h.str, h.hadStr = sc.strs[name]
		h.fn, h.had = sc.funcs[name]
		top[name] = h
	}
}

// endBlock walks each function literal declared in the block that no
// call walked, the last declared first, while the names around it still hold, then gives each name
// the block declared back what it held before.
func (sc *routeScope) endBlock() {
	b := sc.blocks[len(sc.blocks)-1]
	// Last first: a literal declared later may call one declared before
	// it, which is then walked with what that call binds.
	for i := len(b.lits) - 1; i >= 0; i-- {
		if lit := b.lits[i]; !sc.g.rt.litCalled[lit] {
			sc.walkLit(lit, nil, true)
		}
	}
	for name, h := range b.saved {
		restoreName(h.hadEnv, func() { sc.env[name] = h.env }, func() { delete(sc.env, name) })
		restoreName(h.hadTyp, func() { sc.types[name] = h.typ }, func() { delete(sc.types, name) })
		restoreName(h.hadStr, func() { sc.strs[name] = h.str }, func() { delete(sc.strs, name) })
		restoreName(h.had, func() { sc.funcs[name] = h.fn }, func() { delete(sc.funcs, name) })
	}
	sc.blocks = sc.blocks[:len(sc.blocks)-1]
}

func restoreName(had bool, set, unset func()) {
	if had {
		set()
	} else {
		unset()
	}
}

// isBlock says whether a node opens a block of its own for the names
// declared in it.
func isBlock(n ast.Node) bool {
	switch n.(type) {
	case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause, *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		return true
	}
	return false
}

// heldFunc is a function a name holds: a literal with the scope it is
// written in, or a function or method of the module.
type heldFunc struct {
	lit   *ast.FuncLit
	scope *routeScope // where the literal is written, whose names it sees
	fd    *ast.FuncDecl
	file  *goFile
}

// binding is what a call passes to one parameter, as far as syntax shows:
// a router, a literal string, or a function value. A helper that takes a
// prefix and a function registering routes under it is followed through
// all three.
type binding struct {
	router *routerNode
	str    string
	isStr  bool
	fn     heldFunc
}

func (b binding) key() string {
	switch {
	case b.router != nil:
		return fmt.Sprintf("r%p", b.router)
	case b.isStr:
		return "s" + strconv.Quote(b.str)
	case b.fn.lit != nil:
		return fmt.Sprintf("l%p@%p", b.fn.lit, b.fn.scope)
	case b.fn.fd != nil:
		return fmt.Sprintf("f%p", b.fn.fd)
	}
	return ""
}

// walkFunc walks one function with its parameters bound to what a call
// passes them, and gives the router it returns, if any. given makes a
// router parameter with nothing bound one whose place is not known.
func (g *goReader) walkFunc(f *goFile, fd *ast.FuncDecl, bound map[int]binding, given bool) *routerNode {
	st := g.rt
	key := fmt.Sprintf("%p", fd)
	var idx []int
	for i := range bound {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	for _, i := range idx {
		key += fmt.Sprintf("|%d=%s", i, bound[i].key())
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
	sc.bindParams(fd.Type, bound, given, fd.Pos())
	sc.walk(fd.Body)
	sc.walkUncalled()
	st.walked[key] = sc.ret
	return sc.ret
}

func (g *goReader) newScope(f *goFile) *routeScope {
	return &routeScope{g: g, file: f, env: map[string]*routerNode{}, types: map[string]string{}, nodes: map[ast.Expr]*routerNode{}, skip: map[ast.Node]bool{},
		strs: map[string]string{}, funcs: map[string]heldFunc{}, active: map[*ast.FuncLit]bool{}}
}

// bindParams binds a function's parameters to what a call passes them, and
// gives back a function that restores what the names held before, for a
// literal walked inside the function it is written in. given makes a
// router parameter with nothing bound one whose place is not known, at
// the clause of pos.
func (sc *routeScope) bindParams(ft *ast.FuncType, bound map[int]binding, given bool, pos gotoken.Pos) func() {
	g := sc.g
	type held struct {
		env      *routerNode
		typ, str string
		fn       heldFunc
		hadEnv   bool
		hadTyp   bool
		hadStr   bool
		hadFn    bool
	}
	before := map[string]held{}
	i := 0
	if ft.Params != nil {
		for _, p := range ft.Params.List {
			t := g.typeKey(sc.file, p.Type)
			if len(p.Names) == 0 {
				i++
				continue
			}
			for _, n := range p.Names {
				name := n.Name
				h := held{}
				h.env, h.hadEnv = sc.env[name]
				h.typ, h.hadTyp = sc.types[name]
				h.str, h.hadStr = sc.strs[name]
				h.fn, h.hadFn = sc.funcs[name]
				before[name] = h
				delete(sc.env, name)
				delete(sc.types, name)
				delete(sc.strs, name)
				delete(sc.funcs, name)
				if t != "" {
					sc.types[name] = t
				}
				switch b := bound[i]; {
				case b.router != nil:
					sc.env[name] = b.router
				case b.isStr:
					sc.strs[name] = b.str
				case b.fn.lit != nil || b.fn.fd != nil:
					sc.funcs[name] = b.fn
				case given && g.routerKind(t) != "":
					sc.env[name] = &routerNode{lib: g.routerKind(t), unknown: fmt.Sprintf("%s is given the router %s, and the reader found no call that passes it one whose path it knows", g.clause(sc.file, pos), name)}
				}
				i++
			}
		}
	}
	return func() {
		for name, h := range before {
			restore := func(had bool, set, unset func()) {
				if had {
					set()
				} else {
					unset()
				}
			}
			restore(h.hadEnv, func() { sc.env[name] = h.env }, func() { delete(sc.env, name) })
			restore(h.hadTyp, func() { sc.types[name] = h.typ }, func() { delete(sc.types, name) })
			restore(h.hadStr, func() { sc.strs[name] = h.str }, func() { delete(sc.strs, name) })
			restore(h.hadFn, func() { sc.funcs[name] = h.fn }, func() { delete(sc.funcs, name) })
		}
	}
}

// walkLit walks a function literal with its parameters bound to what a
// call passes them, in the scope of the function it is written in, so it
// sees the routers and names around it, and gives the router it returns,
// if any. The literal's return is its own: the return of the function it
// is written in is kept apart from it.
func (sc *routeScope) walkLit(lit *ast.FuncLit, bound map[int]binding, given bool) *routerNode {
	if sc.active[lit] {
		return nil
	}
	sc.active[lit], sc.g.rt.litCalled[lit] = true, true
	defer delete(sc.active, lit)
	restore := sc.bindParams(lit.Type, bound, given, lit.Pos())
	// The routers the literal's own expressions give depend on what is
	// bound, so each walk evaluates them afresh.
	nodes, ret := sc.nodes, sc.ret
	sc.nodes, sc.ret = map[ast.Expr]*routerNode{}, nil
	sc.walk(lit.Body)
	got := sc.ret
	sc.nodes, sc.ret = nodes, ret
	restore()
	return got
}

// walkUncalled walks each function literal held in a local name that no
// call of the function named, once, with nothing bound: a router it takes
// is one whose place is not known.
func (sc *routeScope) walkUncalled() {
	for i := 0; i < len(sc.lits); i++ {
		if lit := sc.lits[i]; !sc.g.rt.litCalled[lit] {
			sc.walkLit(lit, nil, true)
		}
	}
}

// walkBound walks a function of the module with what a call passes it,
// now when a router is among it or the function takes none, else later
// as one given routers from nowhere; then walks, with nothing bound, each
// function literal passed that the function never called, so a route it
// registers is counted and asked about rather than dropped.
func (sc *routeScope) walkBound(f *goFile, fd *ast.FuncDecl, bound map[int]binding, routers int) *routerNode {
	var ret *routerNode
	if routers > 0 || !sc.g.hasRouterParam(f, fd) {
		ret = sc.g.walkFunc(f, fd, bound, false)
	}
	sc.walkUnused(bound)
	return ret
}

// walkUnused walks each function literal a call passed that was never
// called, in the order passed.
func (sc *routeScope) walkUnused(bound map[int]binding) {
	var idx []int
	for i := range bound {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	for _, i := range idx {
		if hf := bound[i].fn; hf.lit != nil && !sc.g.rt.litCalled[hf.lit] {
			hf.home(sc).walkLit(hf.lit, nil, true)
		}
	}
}

// stringValue is a string literal, a concatenation of them, or a name a
// call bound to one.
func (sc *routeScope) stringValue(e ast.Expr) (string, bool) {
	switch e := e.(type) {
	case *ast.Ident:
		s, ok := sc.strs[e.Name]
		return s, ok
	case *ast.ParenExpr:
		return sc.stringValue(e.X)
	case *ast.BinaryExpr:
		if e.Op == gotoken.ADD {
			a, okA := sc.stringValue(e.X)
			b, okB := sc.stringValue(e.Y)
			return a + b, okA && okB
		}
	}
	return sc.g.stringValue(sc.file, e)
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
			if isBlock(sc.stack[len(sc.stack)-1]) {
				sc.endBlock()
			}
			sc.stack = sc.stack[:len(sc.stack)-1]
			return true
		}
		descend := sc.visit(n)
		if descend {
			sc.stack = append(sc.stack, n)
			if isBlock(n) {
				sc.blocks = append(sc.blocks, &walkBlock{saved: map[string]heldName{}})
			}
		}
		return descend
	})
}

func (sc *routeScope) visit(n ast.Node) bool {
	switch n := n.(type) {
	case *ast.AssignStmt:
		sc.assignResults(n.Lhs, n.Rhs)
		for i, lhs := range n.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok || len(n.Rhs) != len(n.Lhs) {
				continue
			}
			if n.Tok == gotoken.DEFINE {
				sc.declare(id.Name)
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
			sc.declare(id.Name)
			if i < len(n.Values) {
				sc.assign(id.Name, n.Values[i])
			}
		}
		var names []ast.Expr
		for _, id := range n.Names {
			names = append(names, id)
		}
		sc.assignResults(names, n.Values)
	case *ast.ReturnStmt:
		if len(n.Results) == 1 && sc.ret == nil {
			sc.ret = sc.eval(n.Results[0])
		}
	case *ast.CallExpr:
		if sc.skip[n] {
			return true
		}
		return sc.call(n)
	case *ast.FuncLit:
		// A literal held in a name or handed to a call the reader
		// follows is walked from there with its parameters bound; any
		// other is walked here, a router it takes one whose place is
		// not known.
		if !sc.skip[n] {
			sc.walkLit(n, nil, true)
		}
		return false
	}
	return true
}

func (sc *routeScope) assign(name string, rhs ast.Expr) {
	if lit, ok := rhs.(*ast.FuncLit); ok {
		sc.funcs[name] = heldFunc{lit: lit, scope: sc}
		sc.lits = append(sc.lits, lit)
		if len(sc.blocks) > 0 {
			top := sc.blocks[len(sc.blocks)-1]
			top.lits = append(top.lits, lit)
		}
		sc.skip[lit] = true
		return
	}
	if node := sc.eval(rhs); node != nil {
		sc.env[name] = node
	}
	if t := sc.g.valueType(sc.file, rhs); t != "" {
		sc.types[name] = t
	} else if call, ok := rhs.(*ast.CallExpr); ok {
		if t := sc.resultType(call, 0, 1); t != "" {
			sc.types[name] = t
		}
	}
}

// assignResults types the names one call's results are assigned to, as
// server, err := rest.NewServer() does.
func (sc *routeScope) assignResults(lhs, rhs []ast.Expr) {
	if len(rhs) != 1 || len(lhs) < 2 {
		return
	}
	call, ok := rhs[0].(*ast.CallExpr)
	if !ok {
		return
	}
	for i, l := range lhs {
		if id, ok := l.(*ast.Ident); ok && id.Name != "_" {
			if t := sc.resultType(call, i, len(lhs)); t != "" {
				sc.types[id.Name] = t
			}
		}
	}
}

// resultType is the type the call's result i has, when the call names a
// function or method of the module that returns n values: a value a
// constructor returns has the type the constructor declares, so a router
// passed to a method on it is followed into that method.
func (sc *routeScope) resultType(call *ast.CallExpr, i, n int) string {
	fd, f := sc.funcValue(call.Fun)
	if fd == nil || fd.Type.Results == nil {
		return ""
	}
	var types []ast.Expr
	for _, r := range fd.Type.Results.List {
		k := len(r.Names)
		if k == 0 {
			k = 1
		}
		for j := 0; j < k; j++ {
			types = append(types, r.Type)
		}
	}
	if len(types) != n {
		return ""
	}
	return sc.g.typeKey(f, types[i])
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
// subrouter, what a function literal held in a local name or bound to a
// parameter returns, or what a function of the module returns.
func (sc *routeScope) evalCall(call *ast.CallExpr) *routerNode {
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
	if id, ok := call.Fun.(*ast.Ident); ok {
		if hf, ok := sc.funcs[id.Name]; ok {
			// A helper that takes a prefix and returns the group under it
			// is followed through its value here; the statement walk,
			// which reaches the call after the name it is assigned to,
			// leaves it alone.
			bound, routers := sc.bindings(call.Args)
			sc.skip[call] = true
			ret, _ := sc.callHeld(hf, bound, routers)
			return ret
		}
	}
	if fd, f := sc.callee(call); fd != nil {
		bound, routers := sc.bindings(call.Args)
		return sc.walkBound(f, fd, bound, routers)
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
	p, ok := sc.stringValue(e)
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
		if hf, ok := sc.heldFunc(fn); ok {
			_, _ = sc.callHeld(hf, map[int]binding{0: {router: child}}, 1)
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

// bindings is what a call passes to a function's parameters, by place: a
// router, a literal string, or a function value; and how many routers.
func (sc *routeScope) bindings(args []ast.Expr) (map[int]binding, int) {
	bound := map[int]binding{}
	routers := 0
	for i, a := range args {
		if n := sc.eval(a); n != nil {
			bound[i] = binding{router: n}
			routers++
			continue
		}
		if s, ok := sc.stringValue(a); ok {
			bound[i] = binding{str: s, isStr: true}
			continue
		}
		if hf, ok := sc.heldFunc(a); ok {
			bound[i] = binding{fn: hf}
		}
	}
	return bound, routers
}

// heldFunc is the function value an expression is: a literal, a name a
// call bound to one or a local name holding one, or a function or method
// of the module.
func (sc *routeScope) heldFunc(e ast.Expr) (heldFunc, bool) {
	switch e := e.(type) {
	case *ast.FuncLit:
		sc.skip[e] = true
		return heldFunc{lit: e, scope: sc}, true
	case *ast.ParenExpr:
		return sc.heldFunc(e.X)
	case *ast.Ident:
		if hf, ok := sc.funcs[e.Name]; ok {
			return hf, true
		}
	}
	if fd, f := sc.funcValue(e); fd != nil {
		return heldFunc{fd: fd, file: f}, true
	}
	return heldFunc{}, false
}

// home is the scope a literal is written in, whose routers and names it
// sees wherever it is called from: sc when nothing recorded one.
func (hf heldFunc) home(sc *routeScope) *routeScope {
	if hf.scope != nil {
		return hf.scope
	}
	return sc
}

// callHeld walks a function value a call names with its parameters bound:
// a literal in the scope it is written in, even when a function it was
// passed to calls it, and a function of the module as any call of one. It
// gives the router the function returns, if any, and says whether the
// call's arguments are to be descended into.
func (sc *routeScope) callHeld(hf heldFunc, bound map[int]binding, routers int) (*routerNode, bool) {
	if hf.lit != nil {
		home := hf.home(sc)
		var ret *routerNode
		if home != sc {
			// The literal is walked where it is written, but a loop or a
			// condition this function calls it behind still holds.
			outer, outerFile := home.outer, home.outerFile
			if b := sc.branchNode(); b != nil {
				home.outer, home.outerFile = b, sc.branchFile()
			}
			ret = home.walkLit(hf.lit, bound, false)
			home.outer, home.outerFile = outer, outerFile
		} else {
			ret = home.walkLit(hf.lit, bound, false)
		}
		sc.walkUnused(bound)
		return ret, false
	}
	return sc.walkBound(hf.file, hf.fd, bound, routers), true
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
	if id, ok := call.Fun.(*ast.Ident); ok {
		if hf, ok := sc.funcs[id.Name]; ok {
			bound, routers := sc.bindings(call.Args)
			_, descend := sc.callHeld(hf, bound, routers)
			return descend
		}
	}
	if fd, f := sc.callee(call); fd != nil {
		bound, routers := sc.bindings(call.Args)
		sc.walkBound(f, fd, bound, routers)
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
			if p, ok := sc.stringValue(c.Args[0]); ok {
				strip, h = p, c.Args[1]
			}
		}
	}
	target := sc.eval(h)
	under := exprText(prefix)
	if p, ok := sc.stringValue(prefix); ok {
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
	if !sc.countOnce(recv, chain...) {
		return
	}
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
		if v, ok := sc.stringValue(pathExpr); ok {
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
		if !sc.countOnce(recv, call) {
			return true
		}
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
			if sc.countOnce(recv, call) {
				g.citeFile(sc.file)
				sc.anyMethod(clause, args[0])
			}
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
				if !sc.countOnce(recv, call) {
					return true
				}
				g.citeFile(sc.file)
				g.question("must", fmt.Sprintf("%s registers a route with Match on methods %s, which is not a literal list. Which methods and path does it register?", clause, exprText(args[0])),
					[]string{"paths"}, "A method computed at run time is a guess for syntax; an operation is never written from a guess.", g.at(clause, "Calls Match with methods that are not a literal list."))
				return true
			}
			methods, pathExpr, args = list.Elts, args[1], args[1:]
		case name == "Any" && len(args) >= 2:
			if sc.countOnce(recv, call) {
				g.citeFile(sc.file)
				sc.anyMethod(clause, args[0])
			}
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
	if !sc.countOnce(recv, call) {
		return true
	}
	g.citeFile(sc.file)
	sc.record(recv, call, clause, methods, pathExpr, handler, middleware)
	return true
}

// countOnce counts a registration once for each call, router, arguments
// and whether it is behind a loop or a condition, however often the walk
// reaches it, and says whether this is the first time: a helper walked
// once per string its callers pass registers a route the string does not
// change once, so it is counted, and written or asked about, once.
func (sc *routeScope) countOnce(recv *routerNode, calls ...*ast.CallExpr) bool {
	root := recv
	for root.parent != nil {
		root = root.parent
	}
	place, unknown := recv.place()
	key := fmt.Sprintf("%p|%s|%s|%t", root, place, unknown, sc.branchNode() != nil)
	for _, c := range calls {
		key += fmt.Sprintf("|%d", c.Pos())
		for _, a := range c.Args {
			if v, ok := sc.stringValue(a); ok {
				key += "|" + strconv.Quote(v)
			} else {
				key += "|" + exprText(a)
			}
		}
	}
	if sc.g.rt.countedAt[key] {
		return false
	}
	sc.g.rt.countedAt[key] = true
	sc.g.rt.count++
	return true
}

// servemux reads a ServeMux pattern, "[METHOD ][HOST]/[PATH]".
func (sc *routeScope) servemux(recv *routerNode, call *ast.CallExpr, clause string, patternExpr, handler ast.Expr) bool {
	g := sc.g
	if sc.branched(clause) {
		return true
	}
	pattern, ok := sc.stringValue(patternExpr)
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
	if s, ok := sc.stringValue(pathExpr); ok {
		p = s
	}
	g.question("must", fmt.Sprintf("%s registers %s for every method. Which methods does the running system answer there, and what does each do?", clause, p),
		[]string{"paths"}, "A route registered for every method answers any method, and an operation has one; writing one per method would write operations the code may refuse.", g.at(clause, "Registers "+p+" for every method."))
}

// record writes one route per method a registration names.
// branched asks about a registration in a loop or behind a condition, and
// says whether it is one.
// branchNode is the loop or condition a registration is behind: one
// between it and the function it is in, else the one a function it was
// passed to calls the literal it is in behind.
func (sc *routeScope) branchNode() ast.Node {
	if b := branch(sc.stack); b != nil {
		return b
	}
	return sc.outer
}

// branchFile is the file the branch of branchNode is in.
func (sc *routeScope) branchFile() *goFile {
	if branch(sc.stack) != nil || sc.outerFile == nil {
		return sc.file
	}
	return sc.outerFile
}

func (sc *routeScope) branched(clause string) bool {
	g := sc.g
	b := sc.branchNode()
	if b == nil {
		return false
	}
	g.question("must", fmt.Sprintf("%s registers a route %s at %s, so the reader cannot tell how many routes it registers, or whether it registers one. Which routes does the running system register here?", clause, branchWord(b), g.clause(sc.branchFile(), b.Pos())),
		[]string{"paths"}, "Syntax does not run the code; only the running system's own list of routes, such as a route table, says what a loop or a condition registers.", g.at(clause, "Registers a route "+branchWord(b)+"."))
	return true
}

func (sc *routeScope) record(recv *routerNode, call ast.Node, clause string, methods []ast.Expr, pathExpr, handler ast.Expr, middleware []ast.Expr) {
	g := sc.g
	if sc.branched(clause) {
		return
	}
	p, ok := sc.stringValue(pathExpr)
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
		} else if c, ok := sc.gateValue(e); ok {
			out = append(out, c)
		}
	}
	return out
}

// gateValue says whether a middleware named without a call, such as
// r.Use(s.RequireClient), is one of the gates the implementation file
// names; a permission check always takes its permission in a call.
func (sc *routeScope) gateValue(e ast.Expr) (checkUse, bool) {
	pkg, name := sc.target(e)
	if name == "" {
		return checkUse{}, false
	}
	for i, c := range sc.g.rt.checks {
		if c.gate != "" && c.pkg == pkg && c.function == name {
			sc.g.citeFile(sc.file)
			return checkUse{clause: sc.g.clause(sc.file, e.Pos()), check: exprText(e), bound: &sc.g.rt.checks[i]}, true
		}
	}
	return checkUse{}, false
}

// checkCall says whether a call is one of the named checks, and reads its
// permission.
func (sc *routeScope) checkCall(call *ast.CallExpr) (checkUse, bool) {
	pkg, name := sc.target(call.Fun)
	if name == "" {
		return checkUse{}, false
	}
	for i, c := range sc.g.rt.checks {
		if c.pkg != pkg || c.function != name {
			continue
		}
		clause := sc.g.clause(sc.file, call.Pos())
		use := checkUse{clause: clause, check: exprText(call.Fun), bound: &sc.g.rt.checks[i]}
		if c.gate != "" {
			if c.argument > 0 {
				use.names, use.argument, use.trailing = sc.gateNames(call, c.argument)
			}
			sc.g.citeFile(sc.file)
			return use, true
		}
		if c.argument > len(call.Args) {
			use.argument = "nothing"
			return use, true
		}
		arg := call.Args[c.argument-1]
		use.argument = exprText(arg)
		if p, ok := sc.stringValue(arg); ok {
			use.permission, use.argument = p, strconv.Quote(p)
		}
		sc.g.citeFile(sc.file)
		return use, true
	}
	return checkUse{}, false
}

// gateNames reads the names a gate's argument n, from 1, gives: a string
// or a list of strings. A name that is not a literal, such as a setting,
// gives no names and the argument as the code writes it. The string
// literals after argument n are returned apart: syntax does not say
// whether they are more names of a variadic check or something else,
// such as a reason, so they are asked, never written.
func (sc *routeScope) gateNames(call *ast.CallExpr, n int) ([]string, string, []string) {
	if n > len(call.Args) {
		return nil, "nothing", nil
	}
	arg := call.Args[n-1]
	var trailing []string
	for _, e := range call.Args[n:] {
		if s, ok := sc.stringValue(e); ok {
			trailing = append(trailing, s)
		}
	}
	if lit, ok := arg.(*ast.CompositeLit); ok {
		var names []string
		for _, e := range lit.Elts {
			s, ok := sc.stringValue(e)
			if !ok {
				return nil, exprText(arg), trailing
			}
			names = append(names, s)
		}
		return names, exprText(arg), trailing
	}
	if s, ok := sc.stringValue(arg); ok {
		return []string{s}, exprText(arg), trailing
	}
	return nil, exprText(arg), trailing
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
		r.hLit = h
	default:
		fd, f := sc.funcValue(h)
		if fd == nil || fd.Body == nil {
			return
		}
		ft, body = fd.Type, fd.Body
		r.hDecl = fd
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
	bypassed := map[*permissionCheck]*bypassedCheck{}
	gated := map[gateKey]*bypassedCheck{}
	trailing := map[string]*trailingNames{} // by the call's clause
	var trailingOrder []string
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
		var clientUses, callerUses []checkUse
		for _, c := range r.checks {
			if c.bound.gate != "" {
				k := gateKey{c.bound, c.names != nil}
				b := gated[k]
				if b == nil {
					b = &bypassedCheck{check: c.bound, clause: c.clause}
					gated[k] = b
				}
				block := r.at + "/callers"
				switch c.bound.gate {
				case "caller":
					block = r.at + "/via"
				case "application", "audience", "event":
					block = r.at
				}
				if !contains(b.at, block) {
					b.at = append(b.at, block)
				}
				b.labels = append(b.labels, label)
				for _, n := range c.names {
					if !contains(b.names, n) {
						b.names = append(b.names, n)
					}
				}
				if c.bound.argument > 0 && c.names == nil && b.setting == "" {
					b.setting = c.argument
				}
				switch {
				case c.bound.gate == "client" && c.names != nil:
					clientUses = append(clientUses, c)
				case c.bound.gate == "caller" && c.names != nil:
					callerUses = append(callerUses, c)
				}
				if len(c.trailing) > 0 {
					t := trailing[c.clause]
					if t == nil {
						t = &trailingNames{use: c}
						trailing[c.clause] = t
						trailingOrder = append(trailingOrder, c.clause)
					}
					if !contains(t.at, block) {
						t.at = append(t.at, block)
					}
				}
				continue
			}
			if c.bound.bypassedFor != "" {
				b := bypassed[c.bound]
				if b == nil {
					b = &bypassedCheck{check: c.bound, clause: c.clause}
					bypassed[c.bound] = b
				}
				if !contains(b.at, r.at+"/permission") {
					b.at = append(b.at, r.at+"/permission")
				}
			}
		}
		opCallers := g.sequencedNames(label, r.at+"/callers", "a", "caller", clientUses)
		opVia := g.sequencedNames(label, r.at+"/via", "an", "immediate caller", callerUses)
		op := mapping("operationId", r.id)
		permission := g.routePermission(r, label)
		for _, c := range r.checks {
			if b := gated[gateKey{c.bound, c.names != nil}]; b != nil && permission != "" && !contains(b.perms, permission) {
				b.perms = append(b.perms, permission)
			}
		}
		if permission != "" {
			set(op, "permission", permission)
			if _, seen := firstAt[permission]; !seen {
				firstAt[permission] = r.clause
				firstVia[permission] = permissionUses(r)[0].check
			}
			checkedBy[permission] = append(checkedBy[permission], label)
		}
		if opCallers != nil {
			set(op, "callers", opCallers)
		}
		if opVia != nil {
			set(op, "via", opVia)
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
	for _, clause := range trailingOrder {
		t := trailing[clause]
		c := t.use
		names := c.argument
		if c.names != nil {
			names = joinAnd(goQuoted(c.names))
		}
		ask := "Are they names the check admits too, or something else, such as a reason?"
		if len(c.trailing) == 1 {
			ask = "Is it a name the check admits too, or something else, such as a reason?"
		}
		g.question("must", fmt.Sprintf("%s: %s passes %s after %s, the argument the implementation file names as the check's. %s", clause, c.check, joinAnd(goQuoted(c.trailing)), names, ask),
			t.at, "Syntax does not say whether a check takes its names as one argument or as every argument from it on; the reader writes the named argument alone and asks about the rest, so neither a name is dropped nor a reason admitted as a caller.",
			g.at(clause, fmt.Sprintf("Calls %s with %s, then %s.", c.check, c.argument, joinAnd(goQuoted(c.trailing)))))
	}
	g.writeBypassed(bypassed)
	g.writeGated(gated, checkedBy)
	g.writeDependencies(held)
	g.res.say("routes: wrote %s: one operation per route a router of net/http, chi, gin, echo or gorilla/mux registers with a literal method and path, outside a loop or a condition, under a prefix the reader knows", plural(len(held), "operation"))
}

// trailingNames is one gate call that passes string literals after the
// argument that names, and the blocks of the operations it guards.
type trailingNames struct {
	use checkUse
	at  []string
}

// sequencedNames is what the literal gates of one kind on a route admit
// together. Each runs in turn and refuses a name it does not list, so
// only the names every one lists pass; where the lists differ, that set
// is written and asked at the block, and none is written when it is
// empty, since a list that silently replaced another would drop a
// definition.
func (g *goReader) sequencedNames(label, block, article, who string, uses []checkUse) []string {
	if len(uses) == 0 {
		return nil
	}
	out := append([]string(nil), uses[0].names...)
	differ := false
	for _, u := range uses[1:] {
		var kept []string
		for _, n := range out {
			if contains(u.names, n) {
				kept = append(kept, n)
			}
		}
		if len(kept) != len(out) || len(u.names) != len(uses[0].names) {
			differ = true
		}
		out = kept
	}
	if !differ {
		return out
	}
	var calls []string
	var cites []*yaml.Node
	for _, u := range uses {
		calls = append(calls, fmt.Sprintf("%s with %s", u.check, joinAnd(goQuoted(u.names))))
		cites = append(cites, g.at(u.clause, fmt.Sprintf("Calls %s with %s.", u.check, joinAnd(goQuoted(u.names)))))
	}
	passes := "only " + joinAnd(out) + map[bool]string{true: " passes", false: " pass"}[len(out) == 1] + " every one, written as its " + block[strings.LastIndex(block, "/")+1:]
	if len(out) == 0 {
		passes = "no " + who + " passes every one, and none is written"
	}
	g.question("must", fmt.Sprintf("%s calls %s in turn, and each refuses %s %s it does not list, so %s. Which %ss is it meant to admit?", label, joinAnd(calls), article, who, passes, who),
		[]string{block}, "Checks in sequence admit only the names every one lists; lists that disagree are as often a mistake as a narrowing, so the set the code admits is written and asked, never one list chosen.", cites...)
	if len(out) == 0 {
		return nil
	}
	return out
}

// gateKey is a gate and whether its argument gave names as literals: the
// operations where it does and the ones where it reads them otherwise are
// asked about apart.
type gateKey struct {
	check   *permissionCheck
	literal bool
}

// bypassedCheck is a check the implementation file says one kind of
// caller skips, with the operations that call it.
type bypassedCheck struct {
	check  *permissionCheck
	clause string   // the first call of it read
	at     []string // the permission of each operation it guards
	// For a gate: the operations it guards, the names its argument gives
	// as literals at any of them, the setting it reads them from where it
	// gives none, and the permissions of the operations.
	labels  []string
	names   []string
	setting string
	perms   []string
}

// writeBypassed asks about every check one kind of caller skips, at each
// operation it guards, as the permission table's reader asks about a
// bypassed gate, so that the merge joins them by the check's name
// (ADR-114). Syntax does not show the skip, which is a test of the
// caller's token written the project's way; the implementation file says
// it.
func (g *goReader) writeBypassed(bypassed map[*permissionCheck]*bypassedCheck) {
	var list []*bypassedCheck
	for _, b := range bypassed {
		list = append(list, b)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].check.name() != list[j].check.name() {
			return list[i].check.name() < list[j].check.name()
		}
		return list[i].check.pkg < list[j].check.pkg
	})
	for _, b := range list {
		c := b.check
		g.res.say("gate: %s names the check %s.%s as skipped for every %s, and it guards %s", g.rt.checkFile, c.pkg, c.function, c.bypassedFor, plural(len(b.at), "operation"))
		q, why := bypassQuestion(c.name(), c.bypassedFor)
		g.question("must", q, b.at, why, g.at(b.clause, fmt.Sprintf("Calls %s, which the implementation file says is skipped for every %s.", c.function, c.bypassedFor)))
	}
}

// writeGated asks about every check the implementation file names as a
// client's or a signature's, once per check, blocking the callers of each
// operation it guards (ADR-154). Syntax shows that a check admits clients
// or verifies a signature, and not which clients, which algorithm or which
// parts of the request: those are in a key or a token the code reads at
// run time. callers names its clients one by one (ADR-110), so the reader
// writes none and asks.
func (g *goReader) writeGated(gated map[gateKey]*bypassedCheck, checkedBy map[string][]string) {
	var list []*bypassedCheck
	for _, b := range gated {
		list = append(list, b)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].check.gate != list[j].check.gate {
			return list[i].check.gate < list[j].check.gate
		}
		if list[i].check.function != list[j].check.function {
			return list[i].check.function < list[j].check.function
		}
		if list[i].check.pkg != list[j].check.pkg {
			return list[i].check.pkg < list[j].check.pkg
		}
		return len(list[i].names) > len(list[j].names)
	})
	for _, b := range list {
		c := b.check
		guards := plural(len(b.at), "operation")
		switch {
		case c.gate == "client" && len(b.names) > 0:
			g.res.say("gate: %s names %s.%s as a client check, and it guards %s, whose callers are the clients it names", g.rt.checkFile, c.pkg, c.function, guards)
			g.question("must", fmt.Sprintf("The check %s admits only the %s %s, written as the callers of %s. Declare each under clients with its kind, the roles that grant the permissions it calls with and its proof, here or in the identity module. Do people call any of them?", c.name(), pluralWord(len(b.names), "client"), joinAnd(b.names), guards),
				b.at, "The code names the clients and not what each is: its kind, its roles and its proof are the registration's, never in syntax. callers names who may call one by one (ADR-110).",
				g.at(b.clause, fmt.Sprintf("Calls %s with %s, which the implementation file names as a client check.", c.function, joinAnd(goQuoted(b.names)))))
		case c.gate == "client":
			g.res.say("gate: %s names %s.%s as a client check, and it guards %s", g.rt.checkFile, c.pkg, c.function, guards)
			g.question("must", fmt.Sprintf("The check %s admits only a client, a caller on its own account rather than a person's, and it guards %s. Which clients call each of them, and do people call any?", c.name(), guards),
				b.at, "Syntax shows that a check admits clients and not which ones, which a key or a token read at run time says. callers names who may call one by one, people and each client, with no word for every client, since a client added later would then be admitted without anyone writing it (ADR-110).",
				g.at(b.clause, fmt.Sprintf("Calls %s, which the implementation file names as a client check.", c.function)))
		case c.gate == "signature":
			g.res.say("gate: %s names %s.%s as a signature check, and it guards %s", g.rt.checkFile, c.pkg, c.function, guards)
			g.question("must", fmt.Sprintf("The check %s verifies a request's signature, and it guards %s. Which client signs the requests, with which algorithm, and over which parts of the request: the client's proof?", c.name(), guards),
				b.at, "A signature's signer, algorithm and covered parts are in a key and a header the code reads at run time, never in syntax. A client's proof names them (ADR-096), and the operation names the client in callers.",
				g.at(b.clause, fmt.Sprintf("Calls %s, which the implementation file names as a signature check.", c.function)))
		case c.gate == "caller" && len(b.names) > 0:
			g.res.say("gate: %s names %s.%s as a caller check, and it guards %s, whose via are the services it names", g.rt.checkFile, c.pkg, c.function, guards)
			g.question("must", fmt.Sprintf("The check %s admits only the %s %s as the immediate caller on a person's behalf, written as the via of %s. Declare each as a client of kind service with its workload-identity proof, mutualTls, spiffe or wimse, here or in the identity module, and the transaction token the operations accept with it.", c.name(), pluralWord(len(b.names), "service"), joinAnd(b.names), guards),
				b.at, "The code names the services and not how each proves itself, which is the deployment's; via names the immediate callers one by one, each with a workload-identity proof, and needs the transaction token it is checked with (ADR-111).",
				g.at(b.clause, fmt.Sprintf("Calls %s with %s, which the implementation file names as a caller check.", c.function, joinAnd(goQuoted(b.names)))))
		case c.gate == "caller":
			g.res.say("gate: %s names %s.%s as a caller check, and it guards %s", g.rt.checkFile, c.pkg, c.function, guards)
			g.question("must", fmt.Sprintf("The check %s compares the immediate caller's workload identity with the services %s gives, and it guards %s. Which services call each of them on a person's behalf: its via?", c.name(), gateSource(b.setting), guards),
				b.at, "Names read at run time are not known by syntax. via names the immediate callers one by one, each a service with a workload-identity proof, with no wildcard, since a hop added later would then be admitted without anyone writing it (ADR-111).",
				g.at(b.clause, fmt.Sprintf("Calls %s, which the implementation file names as a caller check.", c.function)))
		case c.gate == "application":
			g.res.say("gate: %s names %s.%s as an application check, and it guards %s", g.rt.checkFile, c.pkg, c.function, guards)
			apps := "applications " + gateSource(b.setting)
			if len(b.names) > 0 {
				apps = pluralWord(len(b.names), "application") + " " + joinAnd(b.names)
			}
			var elsewhere []string
			for _, p := range b.perms {
				for _, l := range checkedBy[p] {
					if !contains(b.labels, l) && !contains(elsewhere, l) {
						elsewhere = append(elsewhere, l)
					}
				}
			}
			text := fmt.Sprintf("The check %s admits only the %s, the one the person signed in through, at %s, which check %s. On every role that grants %s, write through with the applications the check admits, here or in the identity module.", c.name(), apps, guards, joinAnd(b.perms), map[bool]string{true: "it", false: "them"}[len(b.perms) == 1])
			if len(b.perms) == 0 {
				text = fmt.Sprintf("The check %s admits only the %s, the one the person signed in through, at %s. Which permission does it narrow, and on which roles: their through?", c.name(), apps, guards)
			} else if len(elsewhere) > 0 {
				text += fmt.Sprintf(" %s check the same without it, and through narrows the grant there too: is it meant there?", joinAnd(elsewhere))
			}
			g.question("must", text, []string{"roles"}, "The application is a condition on a grant, written on the role keyed by the permission (ADR-106); the reader writes no role, and the code shows the check, not which grant it narrows.",
				g.at(b.clause, fmt.Sprintf("Calls %s, which the implementation file names as an application check.", c.function)))
		case c.gate == "audience":
			g.res.say("gate: %s names %s.%s as an audience check, and it guards %s", g.rt.checkFile, c.pkg, c.function, guards)
			text := fmt.Sprintf("The check %s compares a token's aud with %s at %s. Which audience names this service: the const of aud on the access token it accepts? A value per environment goes in the implementation file's deployments.", c.name(), gateSource(b.setting), guards)
			if len(b.names) > 0 {
				text = fmt.Sprintf("The check %s compares a token's aud with %s at %s. Write it as the const of aud on the access token they accept, or say which value per environment the implementation file's deployments give.", c.name(), joinAnd(goQuoted(b.names)), guards)
			}
			g.question("must", text, []string{"tokens"}, "A token accepted without a fixed audience is refused, since a token minted for one service could be replayed at the next (ADR-109); the reader writes no token, and an audience read from a setting is the deployment's value.",
				g.at(b.clause, fmt.Sprintf("Calls %s, which the implementation file names as an audience check.", c.function)))
		case c.gate == "event":
			g.res.say("gate: %s names %s.%s as an event writer, and it guards %s; every security event is derived from the refusal that writes it, so the reader writes nothing from it", g.rt.checkFile, c.pkg, c.function, guards)
		}
	}
}

// gateSource says where a gate's names come from when they are not
// literals.
func gateSource(setting string) string {
	if setting == "" || setting == "nothing" {
		return "no argument gives"
	}
	return setting
}

// permissionUses is the checks of a route that read a permission, without
// the gates that read none.
func permissionUses(r *goRoute) []checkUse {
	var out []checkUse
	for _, c := range r.checks {
		if c.bound.gate == "" {
			out = append(out, c)
		}
	}
	return out
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
	for _, c := range permissionUses(r) {
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
	var checks, gates []string
	for _, c := range r.checks {
		if c.bound.gate != "" {
			gates = append(gates, c.check)
			continue
		}
		checks = append(checks, fmt.Sprintf("%s(%s)", c.check, c.argument))
	}
	if len(checks) > 0 {
		s += ", and it is checked by " + joinAnd(checks)
	}
	if len(gates) > 0 {
		s += ", and gated by " + joinAnd(gates)
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
	note := func(wire string) {
		underscore = underscore || strings.Contains(wire, "_")
		capital = capital || strings.ToLower(wire) != wire
	}
	for _, r := range routes {
		if r.body != nil && r.body.typ != nil {
			g.structWires(r.body.file, r.body.typ, note, map[*ast.TypeSpec]bool{})
		}
	}
	return underscore && !capital
}

// structWires notes the name each tagged field of a struct, and of the
// structs it holds, has on the wire.
func (g *goReader) structWires(f *goFile, e ast.Expr, note func(string), seen map[*ast.TypeSpec]bool) {
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
	var stt *ast.StructType
	tf := f
	if inPlace, ok := e.(*ast.StructType); ok {
		stt = inPlace
	} else {
		ts, found := g.structOf(f, e)
		if ts == nil || seen[ts] {
			return
		}
		seen[ts] = true
		stt, _ = ts.Type.(*ast.StructType)
		tf = found
		if stt == nil {
			return
		}
	}
	for _, fl := range stt.Fields.List {
		for _, n := range fl.Names {
			wire, skip, tagged := jsonTag(fl, n.Name)
			if skip || !tagged || !n.IsExported() {
				continue
			}
			note(wire)
		}
		g.structWires(tf, fl.Type, note, seen)
	}
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
		// A map is an object on the wire whose members the source does not
		// name, and whose values are all of one type: additionalProperties.
		// encoding/json writes every key as text, an integer in decimal and
		// a TextMarshaler by its text, so the key's Go type is not in the
		// schema. Values of any JSON value are written open, true, and
		// their shape is the question, blocking the values only.
		values := at + "/additionalProperties"
		if isOpenValue(t.Value) {
			g.question("should", fmt.Sprintf("%s: it is a map, %s, whose values may be any JSON value. Which shape do the values take?", where, exprText(t)),
				[]string{values}, "An interface says nothing of the value's shape, and the map is written open until it is known.", g.at(clause, where+": it is a map, "+exprText(t)+"."))
			return mapping("type", "object", "additionalProperties", true)
		}
		v := g.fieldSchema(f, t.Value, values, where+", a value of the map", snake, seen, clause)
		if v == nil {
			// The question is asked at the values; the map is written open.
			return mapping("type", "object", "additionalProperties", true)
		}
		return mapping("type", "object", "additionalProperties", v)
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
	case *ast.StructType:
		// A struct type written in place, as in var req struct{...}: it
		// has no name and cannot hold itself, so seen stays as it is.
		return g.structSchema(f, t, "", at, where, snake, seen, clause)
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
	return g.structSchema(tf, stt, ts.Name.Name, at, where, snake, seen, clause)
}

// isOpenValue reports whether a Go type holds any JSON value: an interface,
// any among them, whose dynamic value encoding/json writes.
func isOpenValue(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.InterfaceType:
		return true
	case *ast.Ident:
		return t.Name == "any"
	}
	return false
}

// structSchema is the object a struct's fields give on the wire. owner is
// the struct's name, or "" for a struct type written in place.
func (g *goReader) structSchema(tf *goFile, stt *ast.StructType, owner string, at, where string, snake bool, seen map[*ast.TypeSpec]bool, clause string) *yaml.Node {
	field := func(name string) string {
		if owner == "" {
			return "the field " + name + " of the struct written in place"
		}
		return owner + "." + name
	}
	embeds := func(typ string) string {
		if owner == "" {
			return "the struct written in place embeds " + typ
		}
		return owner + " embeds " + typ
	}
	props := mapping()
	var required []string
	for _, fl := range stt.Fields.List {
		if len(fl.Names) == 0 {
			g.question("should", fmt.Sprintf("%s: %s, whose fields encoding/json writes as its own. Which of them does the body take?", where, embeds(exprText(fl.Type))),
				[]string{at + "/properties"}, "The reader writes a struct's own fields, and an embedded struct's may be promoted or hidden by Go's rules.", g.at(g.clause(tf, fl.Pos()), embeds(exprText(fl.Type))+"."))
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
				g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: %s has no json tag, so encoding/json writes it as %s, which is neither camelCase nor snake_case; left out", fclause, field(n.Name), wire), clause: fclause, blocks: []string{at + "/properties"}})
				continue
			case snake:
				c, ok := wireCamel(wire)
				if !ok {
					g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: %s: its name %s does not go back to itself under info.wireNames snake_case; left out", fclause, field(n.Name), wire), clause: fclause, blocks: []string{at + "/properties"}})
					continue
				}
				name = c
			case !memberNameWord.MatchString(wire):
				g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: %s: its name %s is not camelCase, and the bodies' other names are; a specification names its properties on the wire one way (info.wireNames); left out", fclause, field(n.Name), wire), clause: fclause, blocks: []string{at + "/properties"}})
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
	g.loadHandlers(doc.Content[0])
	g.layout = child(doc.Content[0], "layout")
	binding := child(child(doc.Content[0], "bindings"), "http")
	g.loadPermissionChecks(file, binding)
	g.loadGates(file, binding)
	return nil
}

// loadPermissionChecks reads the checks an implementation file names under
// bindings.http.permissionChecks.
func (g *goReader) loadPermissionChecks(file string, binding *yaml.Node) {
	list := child(binding, "permissionChecks")
	if list == nil {
		g.res.say("checks: %s names no permission check under bindings.http.permissionChecks", file)
		return
	}
	for i, item := range list.Content {
		pkg, fn := scalar(child(item, "package")), scalar(child(item, "function"))
		n, err := strconv.Atoi(scalar(child(item, "permissionArgument")))
		if pkg == "" || fn == "" || err != nil || n < 1 {
			g.question("must", fmt.Sprintf("The implementation file %s names a permission check (item %d of bindings.http.permissionChecks) without its package, its function and the argument, from 1, that carries the permission. Which check is it?", file, i+1),
				[]string{"paths"}, "A check named in part cannot be found in the source, and the operations it guards would be read as open.")
			continue
		}
		c := permissionCheck{pkg: pkg, function: fn, argument: n}
		if kind := scalar(child(item, "bypassedFor")); kind != "" {
			if callerKinds[kind] {
				c.bypassedFor = kind
			} else {
				g.question("must", fmt.Sprintf("The implementation file %s says the check %s.%s is skipped for %q (item %d of bindings.http.permissionChecks), and the kind of caller a check is skipped for is client. Which callers does it skip?", file, pkg, fn, kind, i+1),
					[]string{"paths"}, "A check skipped for a kind of caller fails open for that kind, so the kind is asked, never guessed; the check is read as one that skips none.")
			}
		}
		g.rt.checks = append(g.rt.checks, c)
	}
	var names []string
	for _, c := range g.rt.checks {
		if c.gate == "" {
			names = append(names, c.pkg+"."+c.function)
		}
	}
	if len(names) > 0 {
		word := "check"
		if len(names) > 1 {
			word = "checks"
		}
		g.res.say("checks: %s names the permission %s %s", file, word, joinAnd(names))
	}
}

// loadGates reads the checks an implementation file names under
// bindings.http.clientChecks and bindings.http.signatureChecks: gates on a
// route that read no permission (ADR-154).
func (g *goReader) loadGates(file string, binding *yaml.Node) {
	for _, kind := range []struct{ key, gate, word, argument string }{
		{"clientChecks", "client", "client", "clientArgument"},
		{"signatureChecks", "signature", "signature", ""},
		{"applicationChecks", "application", "application", "applicationArgument"},
		{"callerChecks", "caller", "caller", "callerArgument"},
		{"audienceChecks", "audience", "audience", "audienceArgument"},
		{"eventWriters", "event", "event writer", ""},
	} {
		list := child(binding, kind.key)
		if list == nil {
			continue
		}
		var names []string
		for i, item := range list.Content {
			pkg, fn := scalar(child(item, "package")), scalar(child(item, "function"))
			if pkg == "" || fn == "" {
				g.question("must", fmt.Sprintf("The implementation file %s names a %s check (item %d of bindings.http.%s) without its package and its function. Which check is it?", file, kind.word, i+1, kind.key),
					[]string{"paths"}, "A check named in part cannot be found in the source, and the operations it guards would be read as open to people.")
				continue
			}
			if named := g.namedCheck(pkg, fn); named != "" {
				g.question("must", fmt.Sprintf("The implementation file %s names %s.%s as a %s (item %d of bindings.http.%s), and also as a %s. Which is it?", file, pkg, fn, gateWord(kind.gate), i+1, kind.key, named),
					[]string{"paths"}, "A call is read as one check; the reader reads it as the one named first, and the other would be left unread.")
				continue
			}
			n := 0
			if kind.argument != "" {
				if v := scalar(child(item, kind.argument)); v != "" {
					n, _ = strconv.Atoi(v)
				}
			}
			g.rt.checks = append(g.rt.checks, permissionCheck{pkg: pkg, function: fn, gate: kind.gate, argument: n})
			names = append(names, pkg+"."+fn)
		}
		if len(names) > 0 {
			word := "check"
			if len(names) > 1 {
				word = "checks"
			}
			if kind.gate == "event" {
				g.res.say("checks: %s names the %s %s", file, pluralWord(len(names), "event writer"), joinAnd(names))
			} else {
				g.res.say("checks: %s names the %s %s %s", file, kind.word, word, joinAnd(names))
			}
		}
	}
}

// gateWord names a kind of gate: a client check, an event writer.
func gateWord(gate string) string {
	if gate == "event" {
		return "event writer"
	}
	return gate + " check"
}

// namedCheck is what an earlier item names a function as, "" when none
// does.
func (g *goReader) namedCheck(pkg, fn string) string {
	for _, c := range g.rt.checks {
		if c.pkg == pkg && c.function == fn {
			if c.gate == "" {
				return "permission check"
			}
			return gateWord(c.gate)
		}
	}
	return ""
}

// goQuoted quotes each name as Go writes a string.
func goQuoted(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = strconv.Quote(n)
	}
	return out
}
