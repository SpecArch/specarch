package extract

import (
	"fmt"
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/spec"
)

// The import path of dxlib's api package, whose calls the Go reader knows
// (ADR-076).
const dxlibAPI = "github.com/donnyhardyanto/dxlib/api"

// The argument counts of dxlib's registration calls: NewEndPoint(title,
// description, uri, method, endPointType, contentType, parameters,
// onExecute, onWSLoop, responsePossibilities, middlewares, privileges,
// requestMaxContentLength, rateLimitGroupNameId) and NewWSEndPoint(title,
// description, uri, method, onOpen, onMessage, onClose, onPeriodic,
// periodicInterval, middlewares, privileges, rateLimitGroupNameId).
const (
	newEndPointArgs   = 14
	newWSEndPointArgs = 12
)

// The calls on a dxlib request that answer a refusal with a literal status
// and reason, and the one that answers a status alone.
var problemCalls = map[string]bool{
	"WriteResponseAndNewErrorf":            true,
	"WriteResponseAsErrorMessageNotLogged": true,
	"WriteResponseAndLogAsError":           true,
	"WriteResponseAsError":                 true,
}

// The methods net/http names as constants.
var httpMethods = map[string]string{
	"MethodGet": "GET", "MethodHead": "HEAD", "MethodPost": "POST", "MethodPut": "PUT", "MethodPatch": "PATCH",
	"MethodDelete": "DELETE", "MethodConnect": "CONNECT", "MethodOptions": "OPTIONS", "MethodTrace": "TRACE",
}

// The refusal statuses net/http names as constants.
var httpStatuses = map[string]int{
	"StatusBadRequest": 400, "StatusUnauthorized": 401, "StatusPaymentRequired": 402, "StatusForbidden": 403,
	"StatusNotFound": 404, "StatusMethodNotAllowed": 405, "StatusNotAcceptable": 406, "StatusProxyAuthRequired": 407,
	"StatusRequestTimeout": 408, "StatusConflict": 409, "StatusGone": 410, "StatusLengthRequired": 411,
	"StatusPreconditionFailed": 412, "StatusRequestEntityTooLarge": 413, "StatusRequestURITooLong": 414,
	"StatusUnsupportedMediaType": 415, "StatusRequestedRangeNotSatisfiable": 416, "StatusExpectationFailed": 417,
	"StatusTeapot": 418, "StatusMisdirectedRequest": 421, "StatusUnprocessableEntity": 422, "StatusLocked": 423,
	"StatusFailedDependency": 424, "StatusTooEarly": 425, "StatusUpgradeRequired": 426, "StatusPreconditionRequired": 428,
	"StatusTooManyRequests": 429, "StatusRequestHeaderFieldsTooLarge": 431, "StatusUnavailableForLegalReasons": 451,
	"StatusInternalServerError": 500, "StatusNotImplemented": 501, "StatusBadGateway": 502, "StatusServiceUnavailable": 503,
	"StatusGatewayTimeout": 504, "StatusHTTPVersionNotSupported": 505, "StatusVariantAlsoNegotiates": 506,
	"StatusInsufficientStorage": 507, "StatusLoopDetected": 508, "StatusNotExtended": 510, "StatusNetworkAuthenticationRequired": 511,
}

var (
	goModule    = regexp.MustCompile(`(?m)^module\s+"?([^\s"]+)"?\s*$`)
	problemWord = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
)

// goFile is one Go file read: its path from the repository's root, its
// syntax, and the names its imports give.
type goFile struct {
	path    string
	ast     *ast.File
	imports map[string]string // the name a file uses -> the import path
	cited   bool              // something written cites it
}

// goEndpoint is one NewEndPoint call the reader writes as an operation.
type goEndpoint struct {
	file       *goFile
	line       int
	method     string
	key        string // the path item's method key
	uri        string
	params     []string // the path's parameters
	title      string
	desc       string
	declared   map[string]bool // the parameters' names, nil when they are not all literal
	handler    string          // as the code writes it
	chain      []string        // the middlewares, as the code writes them
	chainExprs []ast.Expr
	chainKnown bool
	privileges []string // nil when the call gives nil
	privKnown  bool
	id         string
	reads      []string // the parameters the handler reads, in order, each once
	responses  map[string]string
	answered   []goProblem
	at         string   // the operation's pointer in the tree
	handlerAt  string   // the handler's path:line, "" when it was not found
	paging     *dxTable // the dxlib table whose paging list the handler is
	pagingCall string   // that method's name
}

// goProblem is one refusal a handler answers.
type goProblem struct {
	clause string
	status int
	reason string
}

// Go reads the tracked Go files under the paths with the standard
// library's parser (ADR-075), and dxlib's registration calls in them
// (ADR-076): each endpoint NewEndPoint registers with literal values is an
// operation citing its file and line, with its handler's parameter reads
// and the problems it answers.
func Go(paths []string, out, key string) (*Result, error) {
	r, err := Open(paths)
	if err != nil {
		return nil, err
	}
	res := &Result{Tree: newTree()}
	commitLine(res, r)
	g := &goReader{r: r, key: key, res: res, questions: &yaml.Node{Kind: yaml.MappingNode}, fset: gotoken.NewFileSet(),
		funcs: map[string]map[string]*ast.FuncDecl{}, funcFile: map[*ast.FuncDecl]*goFile{}}
	if err := g.parse(); err != nil {
		return nil, err
	}
	g.collectAssigns()
	g.dxTables, g.entityFiles = map[string]*dxTable{}, map[string]*yaml.Node{}
	g.readTables()
	g.readSeeds()
	g.fileRead = map[string]bool{}
	g.readSettings()
	g.read()
	var clauses []*yaml.Node
	for _, f := range g.files {
		clauses = append(clauses, flow(mapping("clause", f.path, "title", "Go source, read by its syntax")))
	}
	var read []string
	for p := range g.fileRead {
		read = append(read, p)
	}
	sort.Strings(read)
	for _, p := range read {
		clauses = append(clauses, flow(mapping("clause", p, "title", "A configuration file, read as data")))
	}
	if len(clauses) == 0 {
		clauses = append(clauses, flow(mapping("clause", strings.Join(r.Paths, ", "), "title", "Holds no Go source the reader reads")))
	}
	src, err := codeSource(r, out, clauses)
	if err != nil {
		return nil, err
	}
	set(src, "reading", "parsed")
	has := map[string]bool{}
	design := mapping()
	if g.permits != nil {
		set(design, "permissions", g.permits)
	}
	if len(g.paths.Content) > 0 {
		set(design, "paths", g.paths)
	}
	if g.problems != nil {
		set(design, "errors", g.problems)
	}
	if len(design.Content) > 0 {
		has["design"] = true
		res.Tree.put("design/paths.yaml", design)
	}
	if g.roles != nil {
		has["design"] = true
		res.Tree.put("design/roles.yaml", mapping("roles", g.roles))
	}
	g.writeTables()
	for name, n := range g.entityFiles {
		has["design"] = true
		res.Tree.put(name, n)
	}
	if g.config != nil {
		has["deployment"] = true
		res.Tree.put("deployment/configuration.yaml", mapping("configuration", g.config))
	}
	if len(g.questions.Content) > 0 {
		has["requirements"], has["design"] = true, true
		res.Tree.put("design/questions.yaml", mapping("questions", g.questions))
	}
	if g.deployQs != nil {
		has["requirements"], has["deployment"] = true, true
		res.Tree.put("deployment/questions.yaml", mapping("questions", g.deployQs))
	}
	if has["requirements"] {
		res.Tree.put("requirements/stakeholders.yaml", mapping("stakeholders", ownerStakeholder()))
	}
	var stages []string
	for _, s := range spec.Stages {
		if has[s] {
			stages = append(stages, s)
		}
	}
	description := fmt.Sprintf("The Go source under %s, read at commit %s by the standard library's parser, by its syntax alone. Every operation cites the line that registers it; what the source does not say as a literal, or through a call the reader knows, is a question, and so is what the meta-model cannot hold.\n", strings.Join(r.Paths, ", "), r.Commit)
	res.Tree.put("specarch.yaml", rootFile("Go source of "+strings.Join(r.Paths, ", "), description, stages, mapping(key, src)))
	return res, nil
}

type goReader struct {
	r         *Read
	key       string
	res       *Result
	fset      *gotoken.FileSet
	files     []*goFile
	modPath   string                              // the module path go.mod names, "" when none is read
	modRoot   string                              // go.mod's folder from the repository's root
	funcs     map[string]map[string]*ast.FuncDecl // folder -> function name -> declaration
	funcFile  map[*ast.FuncDecl]*goFile
	endpoints []*goEndpoint
	methodsOn map[string]map[string]bool // URI -> the methods registered on it, as dxlib counts them
	paths     *yaml.Node
	permits   *yaml.Node
	problems  *yaml.Node
	questions *yaml.Node
	nextID    int
	notHeld   []notHeld

	assigns     map[string][]goAssign // folder|name or field -> its assignments
	tables      []*goTable
	dxTables    map[string]*dxTable // folder|name or field -> the table constructor assigned to it
	entityFiles map[string]*yaml.Node
	seeds       *seeds
	roles       *yaml.Node
	settings    map[string][]settingRead // the key as the source names it -> where it is read
	config      *yaml.Node
	fileRead    map[string]bool // configuration files read as data
	gates       []goGate
	deployQs    *yaml.Node // the questions on configuration, which sit in the deployment stage
}

func (g *goReader) question(priority, text string, blocks []string, why string, cites ...*yaml.Node) {
	into := g.questionsFor(blocks)
	g.nextID++
	q := mapping(
		"question", text,
		"kind", "decision",
		"priority", priority,
		"blocks", blocks,
		"decidedBy", owner,
		"why", why,
	)
	if len(cites) > 0 {
		set(q, "cites", cites)
	}
	set(into, fmt.Sprintf("Q-%d", g.nextID), q)
}

// questionsFor is where a question on these blocks sits: one on the
// configuration in the deployment stage, any other in the design stage.
func (g *goReader) questionsFor(blocks []string) *yaml.Node {
	for _, b := range blocks {
		if b != "configuration" && !strings.HasPrefix(b, "#/configuration/") {
			return g.questions
		}
	}
	if len(blocks) == 0 {
		return g.questions
	}
	if g.deployQs == nil {
		g.deployQs = &yaml.Node{Kind: yaml.MappingNode}
	}
	return g.deployQs
}

// at is a citation of a place in the source.
func (g *goReader) at(clause, says string) *yaml.Node {
	return citation(g.key, clause, says)
}

func (g *goReader) clause(f *goFile, pos gotoken.Pos) string {
	return fmt.Sprintf("%s:%d", f.path, g.fset.Position(pos).Line)
}

// parse reads every tracked Go file under the paths that the go command
// builds: not a test file, and not in a folder below a path read named
// testdata or vendor, which hold test inputs and other modules' code. A
// file that does not parse is a question, and the rest are read.
func (g *goReader) parse() error {
	skipped, broken := 0, 0
	for _, f := range g.r.Files {
		if path.Base(f) == "go.mod" && g.modPath == "" {
			data, err := os.ReadFile(filepath.Join(g.r.Repository.Root, filepath.FromSlash(f)))
			if err != nil {
				return err
			}
			if m := goModule.FindSubmatch(data); m != nil {
				g.modPath, g.modRoot = string(m[1]), path.Dir(f)
			}
		}
	}
	for _, f := range g.r.Files {
		if !strings.HasSuffix(f, ".go") {
			continue
		}
		// Below the path read, as the go command leaves them out of a
		// pattern such as ./...; a path that names such a folder itself
		// is read.
		below := f
		for _, p := range g.r.Paths {
			if p == "." {
				break
			}
			if strings.HasPrefix(f, p+"/") {
				below = strings.TrimPrefix(f, p+"/")
				break
			}
		}
		parts := strings.Split(below, "/")
		left := strings.HasSuffix(f, "_test.go")
		for _, p := range parts[:len(parts)-1] {
			left = left || p == "testdata" || p == "vendor" || strings.HasPrefix(p, ".") || strings.HasPrefix(p, "_")
		}
		if left {
			skipped++
			continue
		}
		full := filepath.Join(g.r.Repository.Root, filepath.FromSlash(f))
		src, err := os.ReadFile(full)
		if err != nil {
			return err
		}
		if line, text, ok := generatedMark(full); ok {
			g.res.say("generated: %s:%d says it is generated from another source: %s", f, line, text)
		}
		file, err := parser.ParseFile(g.fset, f, src, parser.SkipObjectResolution)
		if err != nil {
			reason := err.Error()
			if list, ok := err.(interface{ Unwrap() []error }); ok && len(list.Unwrap()) > 0 {
				reason = list.Unwrap()[0].Error()
			}
			g.question("must", fmt.Sprintf("%s does not parse as Go (%s), so what it registers is not read. Does it register an endpoint?", f, reason),
				[]string{"paths"}, "A file the parser cannot read may register an endpoint, and an endpoint not read is one a reader must not miss.", g.at(f, "Does not parse as Go."))
			broken++
			continue
		}
		gf := &goFile{path: f, ast: file, imports: map[string]string{}}
		for _, imp := range file.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			name := path.Base(p)
			if imp.Name != nil {
				name = imp.Name.Name
			}
			gf.imports[name] = p
		}
		g.files = append(g.files, gf)
		dir := path.Dir(f)
		for _, d := range file.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil {
				if g.funcs[dir] == nil {
					g.funcs[dir] = map[string]*ast.FuncDecl{}
				}
				g.funcs[dir][fd.Name.Name] = fd
				g.funcFile[fd] = gf
			}
		}
	}
	dxlib := 0
	for _, f := range g.files {
		if f.importName(dxlibAPI) != "" {
			dxlib++
		}
	}
	g.res.say("counted %s, %d of them importing dxlib's api package and %d that %s not parse, and left out %d: every tracked Go file under %s but test files and files in a folder below it named testdata or vendor, or that the go command ignores", plural(len(g.files)+broken, "Go file"), dxlib, broken, doOrDoes(broken), skipped, strings.Join(g.r.Paths, ", "))
	return nil
}

// importName is the name a file gives an import path, or "".
func (f *goFile) importName(p string) string {
	for name, ip := range f.imports {
		if ip == p && name != "_" && name != "." {
			return name
		}
	}
	return ""
}

// read finds dxlib's registration calls in every file that imports dxlib's
// api package, then writes the operations in the order of their paths and
// methods.
func (g *goReader) read() {
	calls := map[string]int{}
	for _, f := range g.files {
		if f.importName(dxlibAPI) == "" {
			continue
		}
		var stack []ast.Node
		ast.Inspect(f.ast, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					switch sel.Sel.Name {
					case "NewEndPoint", "NewWSEndPoint", "RegisterHandler":
						calls[sel.Sel.Name]++
						g.registration(f, call, sel.Sel.Name, stack)
					}
				}
			}
			stack = append(stack, n)
			return true
		})
	}
	g.res.say("dxlib: counted %s, %s and %s: every call by that name in a file that imports dxlib's api package", plural(calls["NewEndPoint"], "NewEndPoint call"), plural(calls["NewWSEndPoint"], "NewWSEndPoint call"), plural(calls["RegisterHandler"], "RegisterHandler call"))
	g.readGates()
	g.writeGates()
	g.writeSettings()
	g.write()
	for _, f := range g.files {
		if f.importName(dxlibAPI) != "" && !f.cited {
			g.res.say("nothing read: %s imports dxlib's api package, and the reader found no endpoint, parameter read or problem in it", f.path)
		}
	}
	askNotHeld(g.res, g.questionsFor, &g.nextID, g.key, g.notHeld)
}

// branch is the loop or condition between a call and the function it is
// in, or nil: a registration there may run any number of times, or not at
// all, which syntax cannot tell.
func branch(stack []ast.Node) ast.Node {
	for i := len(stack) - 1; i >= 0; i-- {
		switch s := stack[i].(type) {
		case *ast.FuncLit, *ast.FuncDecl:
			return nil
		case *ast.ForStmt, *ast.RangeStmt:
			return s
		case *ast.IfStmt, *ast.CaseClause, *ast.CommClause:
			return s
		}
	}
	return nil
}

func branchWord(n ast.Node) string {
	switch n.(type) {
	case *ast.ForStmt, *ast.RangeStmt:
		return "in a loop"
	}
	return "behind a condition"
}

// registration reads one call by a name of dxlib's registration calls.
func (g *goReader) registration(f *goFile, call *ast.CallExpr, name string, stack []ast.Node) {
	clause := g.clause(f, call.Pos())
	f.cited = true
	switch name {
	case "RegisterHandler":
		id, _ := g.stringValue(f, firstArg(call))
		if id == "" {
			id = "an operation the call does not name with a literal"
		}
		handler := "none"
		if len(call.Args) > 1 {
			handler = exprText(call.Args[1])
		}
		g.notHeld = append(g.notHeld, notHeld{
			text:   fmt.Sprintf("%s: RegisterHandler binds %s of the OpenAPI document the service loads to the handler %s; the document gives its method and path, and extract openapi reads it, so the handler is left out here", clause, id, handler),
			clause: clause, blocks: []string{"paths"},
		})
		return
	case "NewWSEndPoint":
		if len(call.Args) != newWSEndPointArgs {
			break
		}
		uri, _ := g.stringValue(f, call.Args[2])
		if uri == "" {
			uri = "an endpoint whose URI is not a literal"
		}
		g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: NewWSEndPoint registers the WebSocket endpoint %s, and the meta-model has no socket; left out", clause, uri), clause: clause, blocks: []string{"paths"}})
		return
	}
	want := newEndPointArgs
	if name == "NewWSEndPoint" {
		want = newWSEndPointArgs
	}
	if len(call.Args) != want {
		g.question("must", fmt.Sprintf("%s calls %s with %d arguments, and dxlib's takes %d, so the reader does not know what it registers. Which endpoint does it register?", clause, name, len(call.Args), want),
			[]string{"paths"}, "A registration the reader cannot read is an endpoint it would miss.", g.at(clause, fmt.Sprintf("Calls %s with %d arguments.", name, len(call.Args))))
		return
	}
	if b := branch(stack); b != nil {
		g.question("must", fmt.Sprintf("%s registers an endpoint with NewEndPoint %s at %s, so the reader cannot tell how many endpoints it registers, or whether it registers one. Which endpoints does the running system register here?", clause, branchWord(b), g.clause(f, b.Pos())),
			[]string{"paths"}, "Syntax does not run the code; only the running system's own list of endpoints, such as the document dxlib emits, says what a loop or a condition registers.", g.at(clause, "Calls NewEndPoint "+branchWord(b)+"."))
		return
	}
	ep := &goEndpoint{file: f, line: g.fset.Position(call.Pos()).Line}
	uri, uriOK := g.stringValue(f, call.Args[2])
	method, methodOK := g.methodValue(f, call.Args[3])
	if !uriOK || !methodOK {
		var what []string
		if !uriOK {
			what = append(what, "URI "+exprText(call.Args[2]))
		}
		if !methodOK {
			what = append(what, "method "+exprText(call.Args[3]))
		}
		g.question("must", fmt.Sprintf("%s registers an endpoint whose %s the reader does not read as a literal. Which method and path does it register?", clause, strings.Join(what, " and ")),
			[]string{"paths"}, "A path or a method computed at run time is a guess for syntax; an endpoint is never written from a guess.", g.at(clause, "Calls NewEndPoint with "+strings.Join(what, " and ")+"."))
		return
	}
	ep.uri, ep.method = uri, strings.ToUpper(method)
	label := ep.method + " " + ep.uri
	if g.methodsOn == nil {
		g.methodsOn = map[string]map[string]bool{}
	}
	if g.methodsOn[ep.uri] == nil {
		g.methodsOn[ep.uri] = map[string]bool{}
	}
	g.methodsOn[ep.uri][ep.method] = true
	if !heldMethods[ep.method] {
		g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: the endpoint %s: the method %s has no operation in the meta-model, which holds GET, POST, PUT, PATCH and DELETE; left out", clause, label, ep.method), clause: clause, blocks: []string{"paths"}})
		return
	}
	if !strings.HasPrefix(ep.uri, "/") {
		g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: the endpoint %s: its URI does not start with /; left out", clause, label), clause: clause, blocks: []string{"paths"}})
		return
	}
	params, reason := pathParameters(ep.uri)
	if reason != "" {
		g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: the endpoint %s: %s; left out", clause, label, reason), clause: clause, blocks: []string{"paths"}})
		return
	}
	if et, ok := call.Args[4].(*ast.SelectorExpr); ok && et.Sel.Name == "EndPointTypeWS" {
		g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: NewEndPoint registers %s as a WebSocket endpoint, and the meta-model has no socket; left out", clause, label), clause: clause, blocks: []string{"paths"}})
		return
	}
	for _, other := range g.endpoints {
		if other.method == ep.method && other.uri == ep.uri {
			g.question("must", fmt.Sprintf("%s registers %s, which %s:%d registers too; dxlib stops the service at the second. Which one does the running system serve?", clause, label, other.file.path, other.line),
				[]string{"#/paths/" + escapeToken(ep.uri) + "/" + strings.ToLower(ep.method), "paths"}, "dxlib refuses a method and URI registered twice when it starts, so at most one of these runs.", g.at(clause, "Registers "+label+" a second time."))
			return
		}
	}
	ep.key = strings.ToLower(ep.method)
	ep.params = params
	ep.title, _ = g.stringValue(f, call.Args[0])
	ep.desc, _ = g.stringValue(f, call.Args[1])
	ep.declared = parameterNames(g, f, call.Args[6])
	ep.handler = exprText(call.Args[7])
	ep.chain, ep.chainKnown = exprList(call.Args[10])
	if cl, ok := call.Args[10].(*ast.CompositeLit); ok {
		ep.chainExprs = cl.Elts
	}
	if names, ok := g.stringList(f, call.Args[11]); ok {
		ep.privileges, ep.privKnown = names, true
	}
	ep.responses = map[string]string{}
	g.endpoints = append(g.endpoints, ep)
	g.handlerBody(ep, call.Args[7])
}

// handlerBody reads the parameters the handler reads and the problems it
// answers, when the handler is a function literal or a function the
// module declares, found by syntax.
func (g *goReader) handlerBody(ep *goEndpoint, h ast.Expr) {
	if t, method := g.pagingTable(ep.file, h); t != nil {
		// dxlib's own paging list: what it reads is dxlib's, and the
		// table's constructor gives its whitelists.
		ep.paging, ep.pagingCall, ep.handlerAt = t, method, t.clause
		label := ep.method + " " + ep.uri
		if t.listsKnown {
			g.notHeld = append(g.notHeld, notHeld{
				text:   fmt.Sprintf("%s: the paging list of %s searches, orders and filters by the fields its table's constructor lists, and the meta-model holds no such list on an operation; cited at the operation", t.clause, label),
				clause: t.clause, blocks: []string{"#/paths/" + escapeToken(ep.uri) + "/" + strings.ToLower(ep.method)},
			})
		} else {
			g.question("should", fmt.Sprintf("%s: the table whose paging list %s serves is made with whitelists that are not literal lists of names. Which fields does its list search, order and filter by?", t.clause, label),
				[]string{"#/paths/" + escapeToken(ep.uri) + "/" + strings.ToLower(ep.method)}, "A whitelist computed at run time is not known by syntax, and only the whitelist says which fields a caller may search, order and filter by.", g.at(t.clause, t.says()))
		}
		return
	}
	var ft *ast.FuncType
	var body *ast.BlockStmt
	file := ep.file
	switch h := h.(type) {
	case *ast.FuncLit:
		ft, body = h.Type, h.Body
		ep.handler = "a function literal"
	case *ast.Ident:
		if fd := g.funcs[path.Dir(ep.file.path)][h.Name]; fd != nil {
			ft, body, file = fd.Type, fd.Body, g.funcFile[fd]
		}
	case *ast.SelectorExpr:
		if x, ok := h.X.(*ast.Ident); ok && g.modPath != "" {
			ip := ep.file.imports[x.Name]
			if ip == g.modPath || strings.HasPrefix(ip, g.modPath+"/") {
				dir := path.Join(g.modRoot, strings.TrimPrefix(strings.TrimPrefix(ip, g.modPath), "/"))
				if fd := g.funcs[dir][h.Sel.Name]; fd != nil {
					ft, body, file = fd.Type, fd.Body, g.funcFile[fd]
				}
			}
		}
	}
	if body == nil {
		return
	}
	ep.handlerAt = g.clause(file, body.Pos())
	if _, isLit := h.(*ast.FuncLit); !isLit {
		ep.handlerAt = g.clause(file, ft.Pos())
	}
	file.cited = true
	request := ""
	if ft.Params != nil && len(ft.Params.List) > 0 && len(ft.Params.List[0].Names) > 0 {
		request = ft.Params.List[0].Names[0].Name
	}
	if request == "" || request == "_" {
		return
	}
	seen := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); !ok || x.Name != request {
			return true
		}
		clause := g.clause(file, call.Pos())
		name := sel.Sel.Name
		switch {
		case strings.HasPrefix(name, "GetParameterValue") && name != "GetParameterValues":
			p, ok := g.stringValue(file, firstArg(call))
			if !ok {
				g.question("should", fmt.Sprintf("%s: the handler of %s %s reads a parameter whose name %s is not a literal. Which parameter does it read?", clause, ep.method, ep.uri, exprText(firstArg(call))),
					[]string{"#/paths/" + escapeToken(ep.uri) + "/" + ep.key}, "Syntax cannot tell which name a computed value holds.", g.at(clause, "Calls "+name+" with a name that is not a literal."))
				return true
			}
			if !seen[p] {
				seen[p] = true
				ep.reads = append(ep.reads, p)
			}
			if ep.declared != nil && !ep.declared[p] && !contains(ep.params, p) {
				g.question("must", fmt.Sprintf("%s: the handler of %s %s reads the parameter %s, which the endpoint does not declare; dxlib answers such a read with an error at run time. Is %s a parameter of the operation, or is the read a mistake?", clause, ep.method, ep.uri, p, p),
					[]string{"#/paths/" + escapeToken(ep.uri) + "/" + ep.key, "#/paths/" + escapeToken(ep.uri) + "/" + ep.key + "/parameters"}, "dxlib's getter fails for a name the endpoint does not declare, so the code cannot work as written.", g.at(clause, fmt.Sprintf("Calls %s(%q).", name, p)))
			}
		case problemCalls[name]:
			g.problemCall(ep, file, call, name, clause)
		}
		return true
	})
}

// problemCall reads one refusal: a literal status, and a literal reason
// that names the problem type.
func (g *goReader) problemCall(ep *goEndpoint, file *goFile, call *ast.CallExpr, name, clause string) {
	label := ep.method + " " + ep.uri
	opAt := "#/paths/" + escapeToken(ep.uri) + "/" + ep.key
	status, ok := g.statusValue(file, firstArg(call))
	if !ok || status < 400 || status > 599 {
		g.question("should", fmt.Sprintf("%s: the handler of %s answers with %s the status %s, which the reader does not read as a literal refusal status. Which status does it answer, and when?", clause, label, name, exprText(firstArg(call))),
			[]string{opAt + "/responses"}, "Only a literal status between 400 and 599 says which refusal the code answers.", g.at(clause, "Calls "+name+" with a status that is not a literal refusal status."))
		return
	}
	code := strconv.Itoa(status)
	reason := ""
	if name != "WriteResponseAsError" && len(call.Args) > 1 {
		var lit bool
		reason, lit = g.stringValue(file, call.Args[1])
		if !lit {
			g.question("must", fmt.Sprintf("%s: the handler of %s answers %s with the reason %s, which is not a literal. Which problem does it answer?", clause, label, code, exprText(call.Args[1])),
				[]string{opAt + "/responses/" + code + "/problem"}, "A reason declared outside the files read, or computed, is not known by syntax.", g.at(clause, fmt.Sprintf("Answers %s with a reason that is not a literal.", code)))
		} else if reason == "" {
			g.question("must", fmt.Sprintf("%s: the handler of %s answers %s with an empty reason, so dxlib answers the status's own text. Which problem does it answer?", clause, label, code),
				[]string{opAt + "/responses/" + code + "/problem"}, "A refusal with no reason of its own names no problem type, and a response names one once the specification declares them.", g.at(clause, fmt.Sprintf("Answers %s with an empty reason.", code)))
		}
	} else {
		g.question("must", fmt.Sprintf("%s: the handler of %s answers %s with %s, which gives no reason of its own. Which problem does it answer?", clause, label, code, name),
			[]string{opAt + "/responses/" + code + "/problem"}, "A refusal with no reason of its own names no problem type, and a response names one once the specification declares them.", g.at(clause, fmt.Sprintf("Answers %s with %s.", code, name)))
	}
	ep.answered = append(ep.answered, goProblem{clause: clause, status: status, reason: reason})
	if _, seen := ep.responses[code]; !seen {
		ep.responses[code] = ""
	}
}

// write writes the operations in the order of their paths and methods,
// the permissions their privileges name, the problems their handlers
// answer, and the questions on what the source does not say.
func (g *goReader) write() {
	g.paths = &yaml.Node{Kind: yaml.MappingNode}
	byURI := map[string][]*goEndpoint{}
	var uris []string
	var lone []string
	for _, ep := range g.endpoints {
		if byURI[ep.uri] == nil {
			uris = append(uris, ep.uri)
		}
		byURI[ep.uri] = append(byURI[ep.uri], ep)
		if ep.privKnown && len(distinct(ep.privileges)) == 1 {
			lone = append(lone, ep.privileges[0])
		}
	}
	// One rule over every privilege name the source gives, so that two
	// names that give one permission are found wherever they are named.
	privileges := newPrivilegeNames(append(lone, g.seedNames()...))
	sort.Strings(uris)
	problemAt := map[string]goProblem{} // problem name -> where it is first answered
	problemStatus := map[string]map[int]bool{}
	var problemOrder []string
	for _, ep := range g.endpoints {
		for _, p := range ep.answered {
			if p.reason == "" {
				continue
			}
			name := strings.ToLower(strings.NewReplacer("_", "-", " ", "-").Replace(p.reason))
			if !problemWord.MatchString(name) {
				continue
			}
			if problemStatus[name] == nil {
				problemStatus[name] = map[int]bool{}
				problemAt[name] = p
				problemOrder = append(problemOrder, name)
			}
			problemStatus[name][p.status] = true
		}
	}
	conflicts := map[string][]string{} // a problem answered at two statuses -> the responses that would name it
	checkedBy := map[string][]string{}
	firstAt := map[string]string{}
	mappedFrom := map[string]string{}
	for _, uri := range uris {
		eps := byURI[uri]
		sort.Slice(eps, func(i, j int) bool { return methodRank(eps[i].key) < methodRank(eps[j].key) })
		item := mapping()
		at := "#/paths/" + escapeToken(uri)
		if params := eps[0].params; len(params) > 0 {
			var list []*yaml.Node
			var blocks []string
			for i, name := range params {
				list = append(list, flow(mapping("name", name, "in", "path", "required", true)))
				blocks = append(blocks, fmt.Sprintf("%s/parameters/%d/schema", at, i))
			}
			set(item, "parameters", list)
			g.question("must", fmt.Sprintf("What values does each path parameter of %s take: %s?", uri, strings.Join(params, ", ")),
				blocks, "A URI names its parameters and not the values they take; the document dxlib emits gives each one's type.")
		}
		for _, ep := range eps {
			ep.at = at + "/" + ep.key
			label := ep.method + " " + ep.uri
			regAt := fmt.Sprintf("%s:%d", ep.file.path, ep.line)
			// dxlib names an endpoint after its URI, and after its method
			// too when the URI has others (OpenAPIOperationId).
			dx := strings.NewReplacer("{", "", "}", "").Replace(strings.ReplaceAll(strings.TrimPrefix(ep.uri, "/"), "/", "_"))
			if len(g.methodsOn[ep.uri]) > 1 {
				dx = ep.key + "_" + dx
			}
			ep.id = dx
			if !memberNameWord.MatchString(dx) {
				ep.id = methodPathName(ep.key, ep.uri)
				g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: the endpoint %s: dxlib names it %s from its URI, which is not camelCase, as an operationId is; it is named %s", regAt, label, dx, ep.id), clause: regAt, blocks: []string{ep.at}})
			}
			op := mapping("operationId", ep.id)
			set(op, "summary", nonEmpty(ep.title))
			set(op, "description", nonEmpty(ep.desc))
			permission := ""
			switch {
			case !ep.privKnown:
				g.question("must", fmt.Sprintf("%s registers %s with privileges %s, which the reader does not read as a literal list. Which permission does it check?", regAt, label, "that are not a literal list"),
					[]string{ep.at + "/permission"}, "A privilege computed at run time is not known by syntax, and an operation's permission is never guessed.", g.at(regAt, "Names its privileges with something other than a literal list."))
			case len(distinct(ep.privileges)) == 0:
				g.question("must", fmt.Sprintf("%s checks no privilege: it registers it with none, so dxlib lets through every caller its middlewares admit (%s). Is it meant to be open to everyone (public), or which permission should it check?", label, g.chainText(ep)),
					[]string{ep.at + "/permission"}, "dxlib checks no privilege on an endpoint that names none, and whether a caller must sign in is decided by its middlewares, which are code; an open endpoint is never written as public without the owner's word.")
			case len(distinct(ep.privileges)) > 1:
				g.question("must", fmt.Sprintf("%s checks the privileges %s: dxlib lets a caller holding any one of them through, and an operation checks one permission. Which permission should it check?", label, joinAnd(distinct(ep.privileges))),
					[]string{ep.at + "/permission"}, "An operation names one permission; choosing one of dxlib's would write a check the code does not make.")
			case ep.privileges[0] == "public":
				g.question("must", fmt.Sprintf("%s checks the privilege public, which dxlib grants only to a role that holds it, while public in a specification means open to everyone. Which permission should it check, or is it meant to be open?", label),
					[]string{ep.at + "/permission"}, "Writing the privilege public as the permission public would open the operation to everyone, which the code does not do.")
			default:
				name := ep.privileges[0]
				p, mapped, collides := privileges.read(name)
				switch {
				case len(collides) > 0:
					var others []string
					for _, c := range collides {
						if c != name {
							others = append(others, c)
						}
					}
					g.question("must", fmt.Sprintf("%s checks the privilege %s, which gives the permission %s, and so %s %s, which dxlib checks as %s. Which permission should it check?", label, name, p, doOrDoes(len(others)), joinAnd(others), anotherPrivilege(len(others))),
						[]string{ep.at + "/permission"}, "Two privileges dxlib tells apart would become one permission, which would write a check the code does not make.")
				case p == "":
					g.question("must", fmt.Sprintf("%s checks the privilege %s, which is not a permission name (lower-case words joined by dots), nor a privilege in capitals that the rule of ADR-076 maps to one. Which permission should it check?", label, name),
						[]string{ep.at + "/permission"}, "The meta-model names a permission in lower-case words joined by dots, and renaming the privilege by a rule of the reader's own would write a check the code does not make.")
				default:
					permission = p
					if _, seen := firstAt[p]; !seen {
						firstAt[p] = regAt
					}
					if mapped {
						mappedFrom[p] = name
					}
					checkedBy[p] = append(checkedBy[p], label)
				}
			}
			set(op, "permission", nonEmpty(permission))
			if len(ep.responses) > 0 {
				responses := mapping()
				for _, c := range sortedCodes(ep.responses) {
					resp := mapping()
					names := map[string]bool{}
					for _, p := range ep.answered {
						if strconv.Itoa(p.status) == c && p.reason != "" {
							names[strings.ToLower(strings.NewReplacer("_", "-", " ", "-").Replace(p.reason))] = true
						}
					}
					var list []string
					for n := range names {
						list = append(list, n)
					}
					sort.Strings(list)
					switch {
					case len(list) == 1 && problemWord.MatchString(list[0]) && len(problemStatus[list[0]]) == 1:
						set(resp, "problem", list[0])
					case len(list) == 1 && problemWord.MatchString(list[0]):
						conflicts[list[0]] = append(conflicts[list[0]], ep.at+"/responses/"+c+"/problem")
					case len(list) == 1:
						g.question("must", fmt.Sprintf("The handler of %s answers %s with a reason that gives no problem name in kebab-case. Which problem does %s answer?", label, c, c),
							[]string{ep.at + "/responses/" + c + "/problem"}, "A problem type is named in lower-case words joined by hyphens, and the reason does not give one.")
					case len(list) > 1:
						g.question("must", fmt.Sprintf("The handler of %s answers %s with the reasons %s, and a response names one problem. Which problem does %s answer, or should the reasons be one?", label, c, joinAnd(list), c),
							[]string{ep.at + "/responses/" + c + "/problem"}, "A response in the meta-model names one problem type.")
					}
					set(responses, c, resp)
				}
				set(op, "responses", responses)
			}
			set(op, "origin", "stated")
			cites := []*yaml.Node{g.at(regAt, g.registeredSays(ep))}
			if ep.handlerAt != "" {
				cites = append(cites, g.at(ep.handlerAt, g.handlerSays(ep)))
			}
			set(op, "cites", cites)
			set(item, ep.key, op)
			if ep.title == "" {
				g.question("must", fmt.Sprintf("What does %s do? Its title is not a literal or is empty.", label),
					[]string{ep.at + "/summary"}, "An operation needs a summary, and NewEndPoint's title gives none here.")
			}
			// With no response written, the responses are what is missing;
			// with some, their descriptions, so that a tree that describes
			// them answers it.
			blocks := []string{ep.at + "/responses"}
			if len(ep.responses) > 0 {
				blocks = nil
			}
			for _, c := range sortedCodes(ep.responses) {
				blocks = append(blocks, ep.at+"/responses/"+c+"/description")
			}
			g.question("must", fmt.Sprintf("What does %s answer when it succeeds, and what does each response mean?", label),
				blocks, "The source names the refusals a handler answers with a literal status, not its success response or what each response means; the document dxlib emits declares them.")
			if ep.handlerAt == "" {
				g.question("should", fmt.Sprintf("%s's handler %s is not a function the reader finds by syntax in the files read, such as a method or a function outside the module, so the parameters it reads and the problems it answers are not read. Which does it read and answer?", label, ep.handler),
					[]string{ep.at}, "Syntax alone finds a function declared in the module and named directly, and no other.", g.at(regAt, "Its handler is "+ep.handler+"."))
			}
			g.question("must", fmt.Sprintf("Does the running system register %s? It is declared at %s, and no list the running system printed was read with it.", label, regAt),
				[]string{ep.at}, "Syntax shows a declaration and not what runs; the document dxlib emits, or a route table, says what the running system registers, and merging it answers this.", g.at(regAt, "Registers "+label+" with NewEndPoint."))
		}
		set(g.paths, uri, item)
	}
	seeded := map[string]*seedPermission{}
	g.writeSeeds(privileges, func(p, privilege, description, clause, says string) {
		s := seeded[p]
		if s == nil {
			s = &seedPermission{}
			seeded[p] = s
		}
		if p != privilege {
			mappedFrom[p] = privilege
		}
		if description != "" && s.description == "" {
			s.description = description
		}
		if !strings.HasPrefix(says, "Inserts") {
			s.granted = true
		}
		s.cites = append(s.cites, g.at(clause, says))
	})
	g.declarePermissions(checkedBy, firstAt, mappedFrom, seeded)
	g.declareProblems(problemOrder, problemAt, problemStatus, conflicts)
	permissions := len(checkedBy)
	g.res.say("wrote %s on %s, %s, %s and %s: one operation per endpoint NewEndPoint registers with a literal method and URI, outside a loop or a condition, one permission per privilege an endpoint names alone, one problem per literal reason a handler answers, one question per thing the source does not say, and one per thing the meta-model cannot hold",
		plural(len(g.endpoints), "operation"), plural(len(uris), "path"), plural(permissions, "permission"), plural(len(problemOrder), "problem"), plural(g.nextID+couldCount(g.questionsFor, g.notHeld), "question"))
	roles, settings := 0, 0
	if g.roles != nil {
		roles = len(g.roles.Content) / 2
	}
	if g.config != nil {
		settings = len(g.config.Content) / 2
	}
	if len(g.tables) > 0 || roles > 0 || settings > 0 {
		g.res.say("wrote %s, %s and %s: one entity per table NewModelDBTable declares with a literal schema and name, one role per role the seeds grant something, and one setting per name the source reads or a configuration file holds", plural(len(g.tables), "entity"), plural(roles, "role"), plural(settings, "setting"))
	}
}

func (g *goReader) chainText(ep *goEndpoint) string {
	switch {
	case !ep.chainKnown:
		return "its middlewares are not a literal list"
	case len(ep.chain) == 0:
		return "it has no middleware"
	}
	return "its middlewares are " + joinAnd(ep.chain)
}

func (g *goReader) registeredSays(ep *goEndpoint) string {
	checks := "checks no privilege"
	switch {
	case !ep.privKnown:
		checks = "names its privileges with something other than a literal list"
	case len(ep.privileges) > 0:
		checks = "checks " + joinAnd(distinct(ep.privileges))
	}
	return fmt.Sprintf("NewEndPoint registers %s %s; its handler is %s, %s, and it %s.", ep.method, ep.uri, ep.handler, g.chainText(ep), checks)
}

func (g *goReader) handlerSays(ep *goEndpoint) string {
	if ep.paging != nil {
		return fmt.Sprintf("The handler of %s %s is dxlib's %s of this table. %s", ep.method, ep.uri, ep.pagingCall, ep.paging.says())
	}
	var parts []string
	if len(ep.reads) > 0 {
		parts = append(parts, "reads the "+parameterOrParameters(len(ep.reads))+" "+joinAnd(ep.reads))
	} else {
		parts = append(parts, "reads no parameter by name")
	}
	var answers []string
	for _, p := range ep.answered {
		a := strconv.Itoa(p.status)
		if p.reason != "" {
			a += " " + p.reason
		}
		if !contains(answers, a) {
			answers = append(answers, a)
		}
	}
	if len(answers) > 0 {
		parts = append(parts, "answers "+joinAnd(answers))
	}
	return fmt.Sprintf("The handler of %s %s %s.", ep.method, ep.uri, joinAnd(parts))
}

// seedPermission is what the seeds say of one permission.
type seedPermission struct {
	description string
	granted     bool
	cites       []*yaml.Node
}

// declarePermissions declares every permission an endpoint's privilege
// names, as extract openapi does, citing the first registration that
// checks it, and every permission the seeds insert or grant, citing them.
func (g *goReader) declarePermissions(checkedBy map[string][]string, firstAt, mappedFrom map[string]string, seeded map[string]*seedPermission) {
	var names []string
	for n := range checkedBy {
		names = append(names, n)
	}
	for n := range seeded {
		if checkedBy[n] == nil {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return
	}
	sort.Strings(names)
	g.permits = &yaml.Node{Kind: yaml.MappingNode}
	var blocks, grants, undescribed, ungranted []string
	for _, n := range names {
		named := n
		if mappedFrom[n] != "" {
			named = mappedFrom[n]
		}
		var cites []*yaml.Node
		if by := checkedBy[n]; by != nil {
			cites = append(cites, g.at(firstAt[n], fmt.Sprintf("%s %s %s in its privileges.", joinAnd(by), checkOrChecks(len(by)), named)))
		}
		s := seeded[n]
		if s != nil {
			cites = append(cites, s.cites...)
		}
		perm := mapping()
		if s != nil && s.description != "" {
			set(perm, "description", s.description)
		} else {
			undescribed = append(undescribed, n)
			blocks = append(blocks, "#/permissions/"+escapeToken(n)+"/description")
		}
		if mappedFrom[n] != "" {
			set(perm, "origin", "inferred")
			set(perm, "why", privilegeWhy(named, n))
		} else {
			set(perm, "origin", "stated")
		}
		set(perm, "cites", cites)
		set(g.permits, n, perm)
		if s == nil || !s.granted {
			ungranted = append(ungranted, n)
			grants = append(grants, "#/permissions/"+escapeToken(n))
		}
	}
	if len(undescribed) > 0 {
		why := "The source names the privilege an endpoint checks and not what it is for."
		if len(seeded) > 0 {
			why = "The source names the privilege an endpoint checks or a seed grants, and a seed's description is read only as a literal of the privilege's insert."
		}
		g.question("must", fmt.Sprintf("What does each permission allow: %s?", strings.Join(undescribed, ", ")), blocks, why)
	}
	if len(ungranted) > 0 {
		g.question("must", fmt.Sprintf("Which role grants each permission: %s?", strings.Join(ungranted, ", ")),
			grants, "The source names the privilege an endpoint checks and not who holds it.")
	}
}

// declareProblems declares every problem a handler answers with a literal
// reason at one status, named after the reason in kebab-case; a reason
// answered at two statuses is a question and no problem.
func (g *goReader) declareProblems(order []string, at map[string]goProblem, statuses map[string]map[int]bool, conflicts map[string][]string) {
	sort.Strings(order)
	var blocks []string
	for _, name := range order {
		p := at[name]
		if len(statuses[name]) > 1 {
			var codes []string
			for s := range statuses[name] {
				codes = append(codes, strconv.Itoa(s))
			}
			sort.Strings(codes)
			g.question("must", fmt.Sprintf("Handlers answer the reason %s with the statuses %s, and a problem has one status. Which status is it, or are these two problems?", p.reason, joinAnd(codes)),
				append([]string{"errors"}, conflicts[name]...), "A problem type in the meta-model has one status, so the reason is not written as one, and the responses that answer it name no problem.", g.at(p.clause, fmt.Sprintf("Answers %d with the reason %s.", p.status, p.reason)))
			continue
		}
		if g.problems == nil {
			g.problems = &yaml.Node{Kind: yaml.MappingNode}
		}
		set(g.problems, name, mapping(
			"status", p.status,
			"origin", "stated",
			"cites", []*yaml.Node{g.at(p.clause, fmt.Sprintf("Answers %d with the reason %s.", p.status, p.reason))},
		))
		blocks = append(blocks, "#/errors/"+name+"/title", "#/errors/"+name+"/condition")
	}
	if len(blocks) > 0 {
		var names []string
		for _, n := range order {
			if len(statuses[n]) == 1 {
				names = append(names, n)
			}
		}
		g.question("must", fmt.Sprintf("What is each problem's title, and when is it answered: %s?", strings.Join(names, ", ")),
			blocks, "The source gives a problem's status and reason, and not a title a person reads or the condition it is answered under.")
	}
}

// stringValue is the value of a string literal, or of literals joined by
// +; ok is false for anything else.
func (g *goReader) stringValue(f *goFile, e ast.Expr) (string, bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind == gotoken.STRING {
			s, err := strconv.Unquote(e.Value)
			return s, err == nil
		}
	case *ast.BinaryExpr:
		if e.Op == gotoken.ADD {
			a, okA := g.stringValue(f, e.X)
			b, okB := g.stringValue(f, e.Y)
			return a + b, okA && okB
		}
	case *ast.ParenExpr:
		return g.stringValue(f, e.X)
	}
	return "", false
}

// methodValue is a method as a string literal or a net/http constant.
func (g *goReader) methodValue(f *goFile, e ast.Expr) (string, bool) {
	if s, ok := g.stringValue(f, e); ok {
		return s, routeMethod.MatchString(strings.ToUpper(s))
	}
	if sel, ok := e.(*ast.SelectorExpr); ok {
		if x, ok := sel.X.(*ast.Ident); ok && f.imports[x.Name] == "net/http" {
			m, known := httpMethods[sel.Sel.Name]
			return m, known
		}
	}
	return "", false
}

// statusValue is a status as an integer literal or a net/http constant.
func (g *goReader) statusValue(f *goFile, e ast.Expr) (int, bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind == gotoken.INT {
			n, err := strconv.Atoi(e.Value)
			return n, err == nil
		}
	case *ast.SelectorExpr:
		if x, ok := e.X.(*ast.Ident); ok && f.imports[x.Name] == "net/http" {
			n, known := httpStatuses[e.Sel.Name]
			return n, known
		}
	}
	return 0, false
}

// stringList is a literal []string{...} of string literals, or nil.
func (g *goReader) stringList(f *goFile, e ast.Expr) ([]string, bool) {
	if id, ok := e.(*ast.Ident); ok && id.Name == "nil" {
		return nil, true
	}
	cl, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	var out []string
	for _, el := range cl.Elts {
		s, ok := g.stringValue(f, el)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// parameterNames is the names of the parameters a literal list declares,
// or nil when the list, or a name in it, is not a literal.
func parameterNames(g *goReader, f *goFile, e ast.Expr) map[string]bool {
	if id, ok := e.(*ast.Ident); ok && id.Name == "nil" {
		return map[string]bool{}
	}
	cl, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	names := map[string]bool{}
	for _, el := range cl.Elts {
		item, ok := el.(*ast.CompositeLit)
		if !ok {
			return nil
		}
		found := false
		for _, kv := range item.Elts {
			kv, ok := kv.(*ast.KeyValueExpr)
			if !ok {
				return nil
			}
			if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "NameId" {
				name, ok := g.stringValue(f, kv.Value)
				if !ok {
					return nil
				}
				names[name] = true
				found = true
			}
		}
		if !found {
			return nil
		}
	}
	return names
}

// exprList is the items of a literal list as the code writes them, or
// none for nil; ok is false for anything else.
func exprList(e ast.Expr) ([]string, bool) {
	if id, ok := e.(*ast.Ident); ok && id.Name == "nil" {
		return nil, true
	}
	cl, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	var out []string
	for _, el := range cl.Elts {
		out = append(out, exprText(el))
	}
	return out, true
}

// exprText writes a name, a selector or a call's function as the code does;
// anything else by what it is.
func exprText(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprText(e.X) + "." + e.Sel.Name
	case *ast.BasicLit:
		return e.Value
	case *ast.FuncLit:
		return "a function literal"
	case *ast.CallExpr:
		return exprText(e.Fun) + "(...)"
	case *ast.StarExpr:
		return "*" + exprText(e.X)
	case *ast.ParenExpr:
		return exprText(e.X)
	case nil:
		return "nothing"
	}
	return "an expression"
}

func firstArg(call *ast.CallExpr) ast.Expr {
	if len(call.Args) == 0 {
		return nil
	}
	return call.Args[0]
}

func parameterOrParameters(n int) string {
	if n == 1 {
		return "parameter"
	}
	return "parameters"
}

func sortedCodes(m map[string]string) []string {
	var codes []string
	for c := range m {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	return codes
}

func distinct(list []string) []string {
	var out []string
	for _, s := range list {
		if !contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}
