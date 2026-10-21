package extract

import (
	"fmt"
	"go/ast"
	gotoken "go/token"
	"path"
	"regexp"
	"sort"
	"strings"
)

// An operation's handler is followed into the code and recorded as its
// logic, step by step, with the source of every function followed
// (ADR-144): the record goes under the operation's mapping in the
// implementation fragment --implementation-out writes, the documents draw
// it as a flowchart beside the source, and validate tells when the code
// has moved on. While the handler is read, its code is checked against
// what the operation declares, and each fault is a question on the
// operation, certain or suspected.

// handlerDepth is how many calls from the handler a function of the module
// is followed: the handler is depth 0.
const handlerDepth = 3

// handlerFunctions is the most functions one handler's record expands.
const handlerFunctions = 40

// goStep is one step of a function's logic.
type goStep struct {
	id, kind, text string
	file           *goFile
	first, last    int
	syntax         ast.Node
	parameter      string
	table, access  string
	transaction    bool
	function       string
	reached        string // for a call of a function literal: called where it is written, passed to a call, or defined for a later call
	stopped        string
	dependency     string
	status         int
	problem        string
	label          string // the label of a loop or a choice, or the one a break or a continue names
	then           []*goStep
	otherwise      []*goStep
	hasOtherwise   bool
	cases          []goCase
	body           []*goStep
	// For the findings.
	call          *ast.CallExpr
	givenRequest  bool // a call not followed that is given the request, the writer or the context
	returnsError  bool // a return whose value is a name an error is held in
	responseValue ast.Expr
	fixedValue    bool // a response whose body is literal values alone
}

// goCase is one case of a choice.
type goCase struct {
	when  string
	steps []*goStep
}

// goFunction is one function followed: the handler or one its steps call.
type goFunction struct {
	name        string
	file        *goFile
	first, last int
	steps       []*goStep
	source      string
	depth       int
	fd          *ast.FuncDecl
	lit         *ast.FuncLit
	hasResponse bool
}

// goHandler is one operation's handler as read.
type goHandler struct {
	g             *goReader
	at            string // the operation's pointer
	label         string // "POST /loans"
	target        string // the handler as a mapping names it
	regClause     string // where the operation is registered
	ep            *goEndpoint
	route         *goRoute
	functions     []*goFunction
	byDecl        map[ast.Node]*goFunction
	dxlib         bool
	lastRequested *goStep // the latest input step, for dxlib's own refusal of a failed read
}

// fnScope is what the walk of one function knows about its names.
type fnScope struct {
	h       *goHandler
	fn      *goFunction
	file    *goFile
	types   map[string]string // a local name -> its type key, importpath.Type
	outside map[string]bool   // a local name -> it holds a value of a type outside the module
	tx      map[string]bool   // a local name -> it holds a transaction
	// A local name -> the function literal assigned to it, so that a call
	// of the name is a call of the literal; nil when the literal was not
	// followed.
	literals map[string]*goFunction
	request  string   // the request parameter, dxlib's or net/http's
	writer   string   // net/http's response writer
	context  string   // gin's or echo's context
	given    []string // the names a call is given to count as given the request
	// The statements a break may leave, innermost last: a loop, a switch
	// or a select, each with its label when the code gives it one.
	breakable []goBreakable
	label     string // the label of the statement about to be walked
}

// goBreakable is a statement a break may leave.
type goBreakable struct {
	kind, label string
}

// enter notes that the walk goes into a loop, a switch or a select, with
// the label the code gave it, and returns that label.
func (sc *fnScope) enter(kind string) string {
	label := sc.label
	sc.label = ""
	sc.breakable = append(sc.breakable, goBreakable{kind: kind, label: label})
	return label
}

// leave notes that the walk left the innermost loop, switch or select.
func (sc *fnScope) leave() {
	sc.breakable = sc.breakable[:len(sc.breakable)-1]
}

// breakTarget is the statement a break leaves: the one its label names,
// or the innermost loop, switch or select (Go spec, Break statements). A
// continue goes to a loop alone.
func (sc *fnScope) breakTarget(label *ast.Ident, loopsOnly bool) goBreakable {
	for i := len(sc.breakable) - 1; i >= 0; i-- {
		b := sc.breakable[i]
		switch {
		case label != nil:
			if b.label == label.Name {
				return b
			}
		case !loopsOnly || b.kind == "loop":
			return b
		}
	}
	if label != nil {
		return goBreakable{kind: "statement", label: label.Name}
	}
	return goBreakable{kind: "loop"}
}

// readHandlers follows every operation's handler and records it, then
// checks each against what the operation declares.
func (g *goReader) readHandlers() {
	var hs []*goHandler
	for _, ep := range g.endpoints {
		if ep.at == "" {
			continue
		}
		h := &goHandler{g: g, at: ep.at, label: ep.method + " " + ep.uri, regClause: g.clause(ep.file, ep.pos), ep: ep, byDecl: map[ast.Node]*goFunction{}, dxlib: true}
		switch {
		case ep.paging != nil:
			h.target = fmt.Sprintf("dxlib's %s of the table %s", ep.pagingCall, nonEmptyText(ep.paging.table, "whose name is not a literal"))
		case ep.hDecl != nil, ep.hLit != nil:
			h.target = h.functionName(ep.hFile, ep.hDecl, ep.hLit)
			h.follow(ep.hFile, ep.hDecl, ep.hLit, 0)
		default:
			h.target = ep.handler
		}
		hs = append(hs, h)
	}
	for _, r := range g.rt.routes {
		if r.at == "" {
			continue
		}
		h := &goHandler{g: g, at: r.at, label: r.method + " " + r.full, regClause: r.clause, route: r, byDecl: map[ast.Node]*goFunction{}}
		if r.hDecl != nil || r.hLit != nil {
			h.target = h.functionName(r.hFile, r.hDecl, r.hLit)
			h.follow(r.hFile, r.hDecl, r.hLit, 0)
		} else {
			h.target = r.handler
		}
		hs = append(hs, h)
	}
	sort.SliceStable(hs, func(i, j int) bool { return hs[i].at < hs[j].at })
	for _, h := range hs {
		h.number()
	}
	g.handlers = hs
	for _, h := range hs {
		h.findings()
	}
	g.askFaults()
	g.askTransactionTokens()
}

// askTransactionTokens asks, at every operation whose handler reads the
// Txn-Token header, for the transaction token it accepts and its via
// (ADR-111, ADR-173).
func (g *goReader) askTransactionTokens() {
	for _, h := range g.handlers {
		var read *goStep
		h.each(func(_ *goFunction, s *goStep, _ []*goStep) {
			if read == nil && s.kind == "input" && isTxnTokenHeader(s.parameter) {
				read = s
			}
		})
		if read == nil {
			continue
		}
		askTransactionToken(g.gateHost(), g.clause(read.file, read.syntax.Pos()), h.label, h.at, read.parameter)
	}
}

// number gives every step its id, in the order the record writes them:
// the handler's steps first, then each function's, each in document order.
func (h *goHandler) number() {
	n := 0
	for _, fn := range h.functions {
		walkSteps(fn.steps, nil, func(s *goStep, _ []*goStep) {
			n++
			s.id = fmt.Sprintf("s%d", n)
		})
	}
}

// functionName is a function's name as the record writes it: the package,
// the type for a method, and the function; a literal is named by where
// it is written.
func (h *goHandler) functionName(f *goFile, fd *ast.FuncDecl, lit *ast.FuncLit) string {
	if lit != nil {
		return fmt.Sprintf("a function literal at %s", h.g.clause(f, lit.Pos()))
	}
	name := f.ast.Name.Name + "."
	if t := recvType(fd); t != "" {
		name += t + "."
	}
	return name + fd.Name.Name
}

// follow reads one function into the record, at the depth it was reached,
// and returns it; a function already followed is returned as it is.
func (h *goHandler) follow(f *goFile, fd *ast.FuncDecl, lit *ast.FuncLit, depth int) *goFunction {
	return h.followWith(f, fd, lit, depth, nil)
}

// followWith is follow with the scope a function literal is written in,
// whose names the literal sees: the request, the writer, a transaction
// and the types of the locals. A declared function is given no scope.
func (h *goHandler) followWith(f *goFile, fd *ast.FuncDecl, lit *ast.FuncLit, depth int, parent *fnScope) *goFunction {
	var key ast.Node
	var ft *ast.FuncType
	var body *ast.BlockStmt
	var start gotoken.Pos
	if lit != nil {
		key, ft, body, start = lit, lit.Type, lit.Body, lit.Pos()
	} else {
		key, ft, body, start = fd, fd.Type, fd.Body, fd.Pos()
		if fd.Doc != nil {
			start = fd.Doc.Pos()
		}
	}
	if fn := h.byDecl[key]; fn != nil {
		return fn
	}
	g := h.g
	fn := &goFunction{name: h.functionName(f, fd, lit), file: f, fd: fd, lit: lit, depth: depth,
		first: g.fset.Position(start).Line, last: g.fset.Position(body.End()).Line}
	fn.source = sourceLines(f, fn.first, fn.last)
	h.byDecl[key] = fn
	h.functions = append(h.functions, fn)
	sc := &fnScope{h: h, fn: fn, file: f, types: map[string]string{}, outside: map[string]bool{}, tx: map[string]bool{}, literals: map[string]*goFunction{}}
	if parent != nil {
		for k, v := range parent.types {
			sc.types[k] = v
		}
		for k, v := range parent.literals {
			sc.literals[k] = v
		}
		for k, v := range parent.outside {
			sc.outside[k] = v
		}
		for k, v := range parent.tx {
			sc.tx[k] = v
		}
		sc.request, sc.writer, sc.context = parent.request, parent.writer, parent.context
	}
	if fd != nil && fd.Recv != nil && len(fd.Recv.List) == 1 && len(fd.Recv.List[0].Names) == 1 {
		sc.bindType(fd.Recv.List[0].Names[0].Name, fd.Recv.List[0].Type)
	}
	for _, p := range ft.Params.List {
		for _, n := range p.Names {
			sc.bindType(n.Name, p.Type)
			if transactionType(g.typeKey(f, p.Type)) {
				sc.tx[n.Name] = true
			}
			t := exprText(p.Type)
			switch {
			case strings.HasSuffix(t, ".DXAPIEndPointRequest") && h.dxlibType(f, p.Type):
				sc.request = n.Name
			case t == "*"+f.importName("net/http")+".Request" && f.importName("net/http") != "":
				sc.request = n.Name
			case t == f.importName("net/http")+".ResponseWriter" && f.importName("net/http") != "":
				sc.writer = n.Name
			case strings.HasSuffix(t, ".Context") && (routerLibraries[f.imports[strings.TrimPrefix(strings.TrimSuffix(t, ".Context"), "*")]] == "gin" || routerLibraries[f.imports[strings.TrimSuffix(t, ".Context")]] == "echo"):
				sc.context = n.Name
			}
		}
	}
	for _, n := range []string{sc.request, sc.writer, sc.context} {
		if n != "" {
			sc.given = append(sc.given, n)
		}
	}
	fn.steps = sc.block(body.List)
	return fn
}

// transactionType says whether a type key is a transaction of
// database/sql, pgx or sqlx.
func transactionType(key string) bool {
	ip, name := splitKey(key)
	return name == "Tx" && (ip == "database/sql" || strings.Contains(ip, "jackc/pgx") || strings.HasSuffix(ip, "sqlx"))
}

// dxlibType says whether a type expression names a type of dxlib's api
// package.
func (h *goHandler) dxlibType(f *goFile, e ast.Expr) bool {
	if s, ok := e.(*ast.StarExpr); ok {
		e = s.X
	}
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && f.imports[x.Name] == dxlibAPI
}

// sourceLines is the source of a file from one line to another, each line
// without its trailing spaces and tabs, since YAML cannot keep them in a
// readable block, and without the indentation of the first line, which a
// function literal written inside a function has and a YAML block cannot
// begin with when it is a tab.
func sourceLines(f *goFile, first, last int) string {
	lines := strings.Split(string(f.src), "\n")
	if last > len(lines) {
		last = len(lines)
	}
	indent := ""
	if first >= 1 && first <= len(lines) {
		indent = lines[first-1][:len(lines[first-1])-len(strings.TrimLeft(lines[first-1], " \t"))]
	}
	var out []string
	for i := first; i <= last; i++ {
		out = append(out, strings.TrimRight(strings.TrimPrefix(lines[i-1], indent), " \t"))
	}
	return strings.Join(out, "\n")
}

// bindType notes a local name's type: a type of the module, or one outside
// it, whose methods are not the system's logic.
func (sc *fnScope) bindType(name string, t ast.Expr) {
	if name == "_" || name == "" {
		return
	}
	g := sc.h.g
	if key := g.typeKey(sc.file, t); key != "" {
		sc.bindKey(name, key)
		return
	}
	e := t
	for {
		switch x := e.(type) {
		case *ast.StarExpr:
			e = x.X
			continue
		case *ast.ArrayType:
			e = x.Elt
			continue
		case *ast.Ident:
			if builtinTypes[x.Name] {
				sc.outside[name] = true
			}
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok && sc.file.imports[id.Name] != "" {
				sc.outside[name] = true
			}
		case *ast.MapType, *ast.FuncType, *ast.ChanType, *ast.InterfaceType:
			sc.outside[name] = true
		}
		return
	}
}

var builtinTypes = map[string]bool{"string": true, "bool": true, "byte": true, "rune": true, "error": true, "any": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true, "uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	"float32": true, "float64": true, "complex64": true, "complex128": true}

var builtinFuncs = map[string]bool{"append": true, "cap": true, "clear": true, "close": true, "complex": true, "copy": true, "delete": true, "imag": true, "len": true,
	"make": true, "max": true, "min": true, "new": true, "print": true, "println": true, "real": true, "recover": true}

// step makes a step of a kind at a node, numbered in the record.
func (sc *fnScope) step(kind string, n ast.Node, text string) *goStep {
	g := sc.h.g
	return &goStep{kind: kind, text: text, file: sc.file, syntax: n,
		first: g.fset.Position(n.Pos()).Line, last: g.fset.Position(n.End()).Line}
}

// block walks the statements of a block in order, and notes the ones
// that follow a return or a panic in it, which never run.
func (sc *fnScope) block(stmts []ast.Stmt) []*goStep {
	var out []*goStep
	for i, st := range stmts {
		out = append(out, sc.statement(st)...)
		if ends(st) && i+1 < len(stmts) {
			sc.unreachable(st, stmts[i+1], stmts[len(stmts)-1])
			break
		}
	}
	return out
}

// ends says whether a statement leaves the block for good: a return or a
// panic.
func ends(st ast.Stmt) bool {
	switch st := st.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.ExprStmt:
		if call, ok := st.X.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "panic" {
				return true
			}
		}
	}
	return false
}

// statement walks one statement: the steps of the expressions it holds,
// then the step it is itself, when it is one.
func (sc *fnScope) statement(st ast.Stmt) []*goStep {
	var out []*goStep
	switch st := st.(type) {
	case *ast.ExprStmt:
		out = sc.expression(st.X)
		if call, ok := st.X.(*ast.CallExpr); ok {
			if fd, _ := sc.callee(call); fd != nil && lastResultIsError(fd) {
				sc.ignoredError("certain", st, sc.text(call, call))
			}
		}
	case *ast.AssignStmt:
		for _, r := range st.Rhs {
			out = append(out, sc.expression(r)...)
		}
		sc.assigned(st)
	case *ast.DeclStmt:
		if gd, ok := st.Decl.(*ast.GenDecl); ok {
			for _, s := range gd.Specs {
				if vs, ok := s.(*ast.ValueSpec); ok {
					for i, v := range vs.Values {
						out = append(out, sc.expression(v)...)
						if lit, ok := v.(*ast.FuncLit); ok && i < len(vs.Names) {
							sc.bindLiteral(vs.Names[i].Name, lit)
						}
					}
					if vs.Type != nil {
						for _, n := range vs.Names {
							sc.bindType(n.Name, vs.Type)
						}
					}
				}
			}
		}
	case *ast.ReturnStmt:
		for _, r := range st.Results {
			out = append(out, sc.expression(r)...)
		}
		text := "returns"
		if len(st.Results) > 0 {
			text += " " + sc.text(st.Results[0], st.Results[len(st.Results)-1])
		}
		returnsError := false
		for _, r := range st.Results {
			if id, ok := r.(*ast.Ident); ok && (id.Name == "err" || strings.HasSuffix(id.Name, "Err") || strings.HasSuffix(id.Name, "Error")) {
				returnsError = true
			}
			if c, ok := r.(*ast.CallExpr); ok {
				if sc.respondsIn(c) {
					returnsError = true
					text = "returns the error the refusal gives"
				} else if sc.returnsErrorType() && r == st.Results[len(st.Results)-1] {
					returnsError = true
				}
			}
		}
		s := sc.step("return", st, text)
		s.returnsError = returnsError
		out = append(out, s)
	case *ast.IfStmt:
		if st.Init != nil {
			out = append(out, sc.statement(st.Init)...)
		}
		out = append(out, sc.expression(st.Cond)...)
		if id, ok := st.Cond.(*ast.Ident); ok && (id.Name == "true" || id.Name == "false") {
			sc.constantCondition(st)
		}
		s := sc.step("branch", st, "if "+sc.text(st.Cond, st.Cond))
		s.then = sc.block(st.Body.List)
		if st.Else != nil {
			s.hasOtherwise = true
			switch e := st.Else.(type) {
			case *ast.BlockStmt:
				s.otherwise = sc.block(e.List)
			default:
				s.otherwise = sc.statement(e)
			}
		}
		out = append(out, s)
	case *ast.ForStmt:
		if st.Init != nil {
			out = append(out, sc.statement(st.Init)...)
		}
		text := "for ever"
		if st.Cond != nil {
			out = append(out, sc.expression(st.Cond)...)
			text = "while " + sc.text(st.Cond, st.Cond)
		}
		s := sc.step("loop", st, text)
		s.label = sc.enter("loop")
		s.body = sc.block(st.Body.List)
		sc.leave()
		out = append(out, s)
	case *ast.RangeStmt:
		out = append(out, sc.expression(st.X)...)
		text := "for each element of " + sc.text(st.X, st.X)
		if st.Value != nil {
			text = "for each " + exprText(st.Value) + " of " + sc.text(st.X, st.X)
		} else if st.Key != nil {
			text = "for each " + exprText(st.Key) + " of " + sc.text(st.X, st.X)
		}
		s := sc.step("loop", st, text)
		s.label = sc.enter("loop")
		s.body = sc.block(st.Body.List)
		sc.leave()
		out = append(out, s)
	case *ast.SwitchStmt:
		if st.Init != nil {
			out = append(out, sc.statement(st.Init)...)
		}
		text := "switch"
		if st.Tag != nil {
			out = append(out, sc.expression(st.Tag)...)
			text = "switch on " + sc.text(st.Tag, st.Tag)
		}
		s := sc.step("choice", st, text)
		s.label = sc.enter("switch")
		s.cases = sc.caseClauses(st.Body.List)
		sc.leave()
		out = append(out, s)
	case *ast.TypeSwitchStmt:
		if st.Init != nil {
			out = append(out, sc.statement(st.Init)...)
		}
		s := sc.step("choice", st, "switch on the type of "+sc.text(st.Assign, st.Assign))
		s.label = sc.enter("switch")
		s.cases = sc.caseClauses(st.Body.List)
		sc.leave()
		out = append(out, s)
	case *ast.SelectStmt:
		s := sc.step("choice", st, "select the first channel ready")
		s.label = sc.enter("select")
		for _, c := range st.Body.List {
			cc, ok := c.(*ast.CommClause)
			if !ok {
				continue
			}
			when := "otherwise"
			if cc.Comm != nil {
				when = sc.text(cc.Comm, cc.Comm)
			}
			s.cases = append(s.cases, goCase{when: when, steps: sc.block(cc.Body)})
		}
		sc.leave()
		out = append(out, s)
	case *ast.BlockStmt:
		out = sc.block(st.List)
	case *ast.LabeledStmt:
		// The label is the statement's own only on a loop, a switch or a
		// select, which a break or a continue may name; on a block or an
		// if it is a goto's target alone, and the loop inside is unlabelled.
		switch st.Stmt.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			sc.label = st.Label.Name
		}
		out = sc.statement(st.Stmt)
		sc.label = ""
	case *ast.BranchStmt:
		out = append(out, sc.branchStep(st))
	case *ast.DeferStmt:
		out = append(out, sc.step("statement", st, "defers "+sc.text(st.Call, st.Call)))
	case *ast.GoStmt:
		out = append(out, sc.step("statement", st, "starts a goroutine running "+sc.text(st.Call, st.Call)))
	case *ast.IncDecStmt:
		out = sc.expression(st.X)
	case *ast.SendStmt:
		out = append(sc.expression(st.Chan), sc.expression(st.Value)...)
	}
	return out
}

// branchStep is the step of a break, a continue, a goto or a fallthrough:
// an unlabelled break leaves the innermost loop, switch or select, a
// labelled one the statement its label names, and a continue goes on
// with the innermost loop or the one named.
func (sc *fnScope) branchStep(st *ast.BranchStmt) *goStep {
	switch st.Tok {
	case gotoken.BREAK:
		target := sc.breakTarget(st.Label, false)
		s := sc.step("statement", st, "leaves the "+target.kind+labelledText(st.Label))
		if st.Label != nil {
			s.label = target.label
		}
		return s
	case gotoken.CONTINUE:
		target := sc.breakTarget(st.Label, true)
		s := sc.step("statement", st, "goes on with the next round of the loop"+labelledText(st.Label))
		if st.Label != nil {
			s.label = target.label
		}
		return s
	case gotoken.GOTO:
		return sc.step("statement", st, "goes to "+exprText(st.Label))
	}
	return sc.step("statement", st, "falls through to the next case")
}

// labelledText names the label a break or a continue gives, or is empty.
func labelledText(label *ast.Ident) string {
	if label == nil {
		return ""
	}
	return " labelled " + label.Name
}

// returnsErrorType says whether the function's last result is an error.
func (sc *fnScope) returnsErrorType() bool {
	var ft *ast.FuncType
	if sc.fn.fd != nil {
		ft = sc.fn.fd.Type
	} else {
		ft = sc.fn.lit.Type
	}
	if ft.Results == nil || len(ft.Results.List) == 0 {
		return false
	}
	id, ok := ft.Results.List[len(ft.Results.List)-1].Type.(*ast.Ident)
	return ok && id.Name == "error"
}

// caseClauses walks the cases of a switch.
func (sc *fnScope) caseClauses(list []ast.Stmt) []goCase {
	var out []goCase
	for _, c := range list {
		cc, ok := c.(*ast.CaseClause)
		if !ok {
			continue
		}
		when := "otherwise"
		if len(cc.List) > 0 {
			when = sc.text(cc.List[0], cc.List[len(cc.List)-1])
		}
		out = append(out, goCase{when: when, steps: sc.block(cc.Body)})
	}
	return out
}

// lastResultIsError says whether a function's last result is an error.
func lastResultIsError(fd *ast.FuncDecl) bool {
	rs := fd.Type.Results
	if rs == nil || len(rs.List) == 0 {
		return false
	}
	id, ok := rs.List[len(rs.List)-1].Type.(*ast.Ident)
	return ok && id.Name == "error"
}

// assigned notes what an assignment tells about its names: the type a
// name is given, a value from outside the module, and a transaction begun;
// and asks about a result dropped that is, or may be, an error.
func (sc *fnScope) assigned(st *ast.AssignStmt) {
	if len(st.Rhs) == 1 && len(st.Lhs) >= 1 {
		if call, ok := st.Rhs[0].(*ast.CallExpr); ok {
			last, isBlank := st.Lhs[len(st.Lhs)-1].(*ast.Ident)
			isBlank = isBlank && last.Name == "_"
			switch fd, _ := sc.callee(call); {
			case fd != nil:
				if isBlank && lastResultIsError(fd) {
					sc.ignoredError("certain", st, sc.text(call, call))
				}
			case isBlank && len(st.Lhs) >= 2:
				sc.ignoredError("suspected", st, sc.text(call, call))
			}
		}
	}
	if len(st.Lhs) == len(st.Rhs) {
		for i, l := range st.Lhs {
			id, ok := l.(*ast.Ident)
			if !ok {
				continue
			}
			sc.noteValue(id.Name, st.Rhs[i], 0)
		}
		return
	}
	if len(st.Rhs) == 1 {
		for i, l := range st.Lhs {
			if id, ok := l.(*ast.Ident); ok {
				sc.noteValue(id.Name, st.Rhs[0], i)
			}
		}
	}
}

// noteValue notes the type of the i-th value an expression gives a name.
func (sc *fnScope) noteValue(name string, e ast.Expr, i int) {
	if name == "_" {
		return
	}
	delete(sc.literals, name)
	if lit, ok := e.(*ast.FuncLit); ok {
		sc.bindLiteral(name, lit)
		return
	}
	g := sc.h.g
	if t := g.valueType(sc.file, e); t != "" {
		sc.bindKey(name, t)
		return
	}
	call, ok := e.(*ast.CallExpr)
	if !ok {
		if _, ok := e.(*ast.BasicLit); ok {
			sc.outside[name] = true
		}
		return
	}
	if fd, cf := sc.callee(call); fd != nil {
		if fd.Type.Results != nil {
			var results []ast.Expr
			for _, r := range fd.Type.Results.List {
				n := len(r.Names)
				if n == 0 {
					n = 1
				}
				for j := 0; j < n; j++ {
					results = append(results, r.Type)
				}
			}
			if i < len(results) {
				saved := sc.file
				sc.file = cf
				sc.bindType(name, results[i])
				sc.file = saved
			}
		}
		return
	}
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if beginNames[sel.Sel.Name] && i == 0 {
			sc.tx[name] = true
			return
		}
		if id, ok := sel.X.(*ast.Ident); ok && (id.Name == sc.request || id.Name == sc.context) {
			sc.outside[name] = true
			return
		}
	}
	// A value a call outside the module gives, or one the reader does not
	// place: its methods are not followed either way.
	sc.outside[name] = true
}

// bindLiteral notes that a local name holds a function literal, read
// already as a function of its own when its step was made, so that a call
// of the name is a call of that function; a literal not followed is noted
// as such.
func (sc *fnScope) bindLiteral(name string, lit *ast.FuncLit) {
	if name == "_" || name == "" {
		return
	}
	sc.literals[name] = sc.h.byDecl[lit]
}

// bindKey notes a local name's type by its key.
func (sc *fnScope) bindKey(name, key string) {
	g := sc.h.g
	sc.types[name] = key
	ip, _ := splitKey(key)
	if g.dirOf(ip) == "" && ip != g.pkgPath(path.Dir(sc.file.path)) {
		sc.outside[name] = true
	}
}

var beginNames = map[string]bool{"Begin": true, "BeginTx": true, "BeginTransaction": true, "Beginx": true}

// text is the source of the nodes from the first to the last, on one line,
// shortened when it is long.
func (sc *fnScope) text(from, to ast.Node) string {
	g := sc.h.g
	src := sc.file.src
	a, b := g.fset.Position(from.Pos()).Offset, g.fset.Position(to.End()).Offset
	if a < 0 || b > len(src) || a > b {
		return exprText(from.(ast.Expr))
	}
	s := strings.Join(strings.Fields(string(src[a:b])), " ")
	if len(s) > 100 {
		s = s[:97] + "..."
	}
	return s
}

// expression walks an expression and gives the steps of the calls in it,
// the arguments of a call before the call.
func (sc *fnScope) expression(e ast.Expr) []*goStep {
	var out []*goStep
	switch e := e.(type) {
	case nil:
	case *ast.CallExpr:
		if lit, ok := e.Fun.(*ast.FuncLit); ok {
			for _, a := range e.Args {
				out = append(out, sc.expression(a)...)
			}
			return append(out, sc.literalStep(lit, "calls"))
		}
		out = append(out, sc.expression(e.Fun)...)
		for _, a := range e.Args {
			if lit, ok := a.(*ast.FuncLit); ok {
				out = append(out, sc.literalStep(lit, "passes", sc.text(e.Fun, e.Fun)))
				continue
			}
			out = append(out, sc.expression(a)...)
		}
		if s := sc.call(e); s != nil {
			out = append(out, s)
		}
	case *ast.SelectorExpr:
		out = sc.expression(e.X)
	case *ast.IndexExpr:
		out = append(sc.expression(e.X), sc.expression(e.Index)...)
		if s := sc.varsIndex(e); s != nil {
			out = append(out, s)
		}
	case *ast.IndexListExpr:
		out = sc.expression(e.X)
	case *ast.SliceExpr:
		out = append(out, sc.expression(e.X)...)
		out = append(out, sc.expression(e.Low)...)
		out = append(out, sc.expression(e.High)...)
	case *ast.StarExpr:
		out = sc.expression(e.X)
	case *ast.UnaryExpr:
		out = sc.expression(e.X)
	case *ast.BinaryExpr:
		out = append(sc.expression(e.X), sc.expression(e.Y)...)
	case *ast.ParenExpr:
		out = sc.expression(e.X)
	case *ast.TypeAssertExpr:
		out = sc.expression(e.X)
	case *ast.KeyValueExpr:
		out = append(sc.expression(e.Key), sc.expression(e.Value)...)
	case *ast.CompositeLit:
		for _, el := range e.Elts {
			out = append(out, sc.expression(el)...)
		}
	case *ast.FuncLit:
		out = append(out, sc.literalStep(e, "defines"))
	}
	return out
}

// literalStep is the call step of a function literal, followed as a
// function of its own one call deeper, with the names of the function it
// is written in: its returns are its own, not the enclosing function's.
// The verb says how the literal is reached: called where it is written,
// passed to a call (named by to), or defined for a later call.
func (sc *fnScope) literalStep(lit *ast.FuncLit, verb string, to ...string) *goStep {
	h := sc.h
	name := h.functionName(sc.file, nil, lit)
	text := verb + " " + name
	if len(to) > 0 {
		text += " to " + to[0]
	}
	s := sc.step("call", lit, text)
	s.reached = map[string]string{"calls": "called", "passes": "passed", "defines": "defined"}[verb]
	if fn := h.byDecl[lit]; fn != nil {
		s.function = fn.name
		return s
	}
	// A literal not followed sees the request and the writer, so what it
	// reads or answers is not known, as for a call given them.
	givenRequest := len(sc.given) > 0
	if sc.fn.depth+1 > handlerDepth {
		s.stopped, s.givenRequest = fmt.Sprintf("deeper than the %d calls followed from the handler", handlerDepth), givenRequest
		return s
	}
	if len(h.functions) >= handlerFunctions {
		s.stopped, s.givenRequest = fmt.Sprintf("the record already expands %d functions", handlerFunctions), givenRequest
		return s
	}
	fn := h.followWith(sc.file, nil, lit, sc.fn.depth+1, sc)
	s.function = fn.name
	return s
}

// varsIndex is the input step of gorilla/mux's Vars(request)["name"].
func (sc *fnScope) varsIndex(ix *ast.IndexExpr) *goStep {
	c, ok := ix.X.(*ast.CallExpr)
	if !ok {
		return nil
	}
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != "Vars" || routerLibraries[sc.file.imports[exprText(s.X)]] != "gorilla/mux" {
		return nil
	}
	return sc.input(ix, "path parameter", ix.Index)
}

// input is an input step reading a request value by a name.
func (sc *fnScope) input(n ast.Node, what string, name ast.Expr) *goStep {
	g := sc.h.g
	if p, ok := g.stringValue(sc.file, name); ok {
		s := sc.step("input", n, "reads the "+what+" "+p)
		s.parameter = p
		sc.h.lastRequested = s
		return s
	}
	s := sc.step("input", n, "reads a "+what+" whose name, "+exprText(name)+", is not a literal")
	sc.h.lastRequested = s
	return s
}

// callee is the function or method of the module a call names, found by
// syntax: a function of the file's package, one of another package of
// the module through its import, or a method of a type the scope knows.
func (sc *fnScope) callee(call *ast.CallExpr) (*ast.FuncDecl, *goFile) {
	g := sc.h.g
	switch f := call.Fun.(type) {
	case *ast.Ident:
		if sc.types[f.Name] != "" || sc.outside[f.Name] {
			return nil, nil
		}
		if fd := g.funcs[path.Dir(sc.file.path)][f.Name]; fd != nil {
			return fd, g.funcFile[fd]
		}
	case *ast.SelectorExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			if sc.types[id.Name] == "" && !sc.outside[id.Name] {
				if dir := g.dirOf(sc.file.imports[id.Name]); dir != "" {
					if fd := g.funcs[dir][f.Sel.Name]; fd != nil {
						return fd, g.funcFile[fd]
					}
				}
			}
		}
		if key := sc.typeOf(f.X); key != "" {
			return g.methodOf(key, f.Sel.Name)
		}
	}
	return nil, nil
}

// methodOf is the method of a type of the module, by the type's key.
func (g *goReader) methodOf(key, method string) (*ast.FuncDecl, *goFile) {
	ip, name := splitKey(key)
	dir := g.dirOf(ip)
	if dir == "" {
		if g.modPath != "" {
			return nil, nil
		}
		dir = ip
	}
	if fd := g.rt.methods[dir+"|"+name+"."+method]; fd != nil {
		return fd, g.rt.methodOf[fd]
	}
	return nil, nil
}

// typeOf is the type key of an expression the scope knows: a local name,
// or a field of a struct of the module reached from one.
func (sc *fnScope) typeOf(e ast.Expr) string {
	g := sc.h.g
	switch e := e.(type) {
	case *ast.Ident:
		if t := sc.types[e.Name]; t != "" || sc.outside[e.Name] || sc.isLocalFunc(e.Name) {
			return t
		}
		// A package-level name of this package: the type its one
		// assignment gives.
		return g.packageValueType(path.Dir(sc.file.path), e.Name)
	case *ast.ParenExpr:
		return sc.typeOf(e.X)
	case *ast.StarExpr:
		return sc.typeOf(e.X)
	case *ast.SelectorExpr:
		if id, ok := e.X.(*ast.Ident); ok && sc.types[id.Name] == "" && !sc.outside[id.Name] {
			// A package-level name of the module, through its import or
			// of this package: the type its one assignment gives.
			dir := g.dirOf(sc.file.imports[id.Name])
			if dir != "" {
				return g.packageValueType(dir, e.Sel.Name)
			}
		}
		owner := sc.typeOf(e.X)
		if owner == "" {
			return ""
		}
		ts, tf := g.typeSpecOf(owner)
		if ts == nil {
			return ""
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return ""
		}
		for _, fl := range st.Fields.List {
			for _, n := range fl.Names {
				if n.Name == e.Sel.Name {
					return g.typeKey(tf, fl.Type)
				}
			}
		}
	}
	return ""
}

// typeSpecOf is the declaration of a type of the module, by key.
func (g *goReader) typeSpecOf(key string) (*ast.TypeSpec, *goFile) {
	ip, name := splitKey(key)
	dir := g.dirOf(ip)
	if dir == "" {
		if g.modPath != "" {
			return nil, nil
		}
		dir = ip
	}
	ts := g.rt.types[dir+"|"+name]
	if ts == nil {
		return nil, nil
	}
	return ts, g.rt.typeFile[ts]
}

// packageValueType is the type of a package-level name of a folder of the
// module: the type its var declaration names, or the one its one
// assignment gives, or "" when syntax does not say.
func (g *goReader) packageValueType(dir, name string) string {
	if f, vs := g.packageVarSpec(dir, name); vs != nil && vs.Type != nil {
		return g.typeKey(f, vs.Type)
	}
	as := g.assigns[dir+"|"+name]
	if len(as) != 1 {
		return ""
	}
	return g.valueType(as[0].file, as[0].value)
}

// packageVarSpec is the var declaration of a package-level name of a
// folder, or nil.
func (g *goReader) packageVarSpec(dir, name string) (*goFile, *ast.ValueSpec) {
	for _, f := range g.files {
		if path.Dir(f.path) != dir {
			continue
		}
		for _, d := range f.ast.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != gotoken.VAR {
				continue
			}
			for _, sp := range gd.Specs {
				if vs, ok := sp.(*ast.ValueSpec); ok {
					for _, n := range vs.Names {
						if n.Name == name {
							return f, vs
						}
					}
				}
			}
		}
	}
	return nil, nil
}

// packageVar says whether a folder declares a package-level var by a name.
func (g *goReader) packageVar(dir, name string) bool {
	_, vs := g.packageVarSpec(dir, name)
	return vs != nil
}

// respondsIn says whether a call writes a response, for a return that
// holds one.
func (sc *fnScope) respondsIn(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == sc.request && sc.request != "" && strings.HasPrefix(sel.Sel.Name, "WriteResponse")
}

var sqlVerb = regexp.MustCompile(`(?i)^\s*(select|insert|update|delete|with|merge|upsert|truncate|create|alter|drop)\b`)
var sqlTable = regexp.MustCompile(`(?i)\b(?:from|into|update|join|table)\s+("?[A-Za-z_][A-Za-z0-9_.]*"?)`)

// sqlStep is the database step of a call that runs a SQL statement given as
// a literal, or nil when the first string argument is not SQL.
func (sc *fnScope) sqlStep(call *ast.CallExpr, inTx bool) *goStep {
	g := sc.h.g
	for _, a := range call.Args {
		q, ok := g.stringValue(sc.file, a)
		if !ok {
			continue
		}
		m := sqlVerb.FindStringSubmatch(q)
		if m == nil {
			continue
		}
		verb := strings.ToUpper(m[1])
		access := "write"
		if verb == "SELECT" || verb == "WITH" {
			access = "read"
		}
		table := ""
		if t := sqlTable.FindStringSubmatch(q); t != nil {
			table = strings.Trim(t[1], `"`)
		}
		short := strings.Join(strings.Fields(q), " ")
		if len(short) > 80 {
			short = short[:77] + "..."
		}
		text := map[string]string{"read": "reads", "write": "writes"}[access] + " "
		if table != "" {
			text += table
		} else {
			text += "a table the statement does not name"
		}
		s := sc.step("database", call, text+": "+short)
		s.table, s.access, s.transaction = table, access, inTx
		return s
	}
	return nil
}

// writeMethods are the dxlib table methods that change rows; any other
// reads.
func dxAccess(method string) string {
	m := strings.TrimPrefix(method, "Tx")
	for _, w := range []string{"Insert", "Update", "Delete", "Upsert", "Save", "Create", "Remove", "Set", "Soft", "Hard", "Edit"} {
		if strings.HasPrefix(m, w) {
			return "write"
		}
	}
	return "read"
}

// call is the step a call is, or nil for a call that is not a step: a
// builtin, a call into the standard library, or one on a value of a type
// outside the module.
func (sc *fnScope) call(call *ast.CallExpr) *goStep {
	h := sc.h
	g := h.g
	f := sc.file
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		switch {
		case fn.Name == "panic":
			return sc.step("statement", call, "panics: "+sc.argsText(call))
		case builtinFuncs[fn.Name]:
			return nil
		}
		if lit, ok := sc.literals[fn.Name]; ok {
			// A call of a local the code assigned a function literal: the
			// literal runs here, so this step carries it, not the definition.
			s := sc.step("call", call, "calls "+sc.text(call, call))
			s.reached, s.call = "called", call
			if lit != nil {
				s.function = lit.name
			} else {
				s.stopped, s.givenRequest = "the function literal assigned to "+fn.Name+", which was not followed", len(sc.given) > 0
			}
			return s
		}
		if fd, cf := sc.callee(call); fd != nil {
			return sc.callStep(call, fd, cf)
		}
		if sc.types[fn.Name] != "" || sc.outside[fn.Name] || sc.isLocalFunc(fn.Name) || g.packageVar(path.Dir(f.path), fn.Name) {
			s := sc.step("call", call, "calls "+sc.text(call, call))
			s.stopped, s.call = "a function value, which syntax does not follow", call
			s.givenRequest = sc.givenRequest(call)
			return s
		}
		s := sc.step("call", call, "calls "+sc.text(call, call))
		s.stopped, s.call = "not a function of the files read", call
		s.givenRequest = sc.givenRequest(call)
		return s
	case *ast.FuncLit:
		return nil
	case *ast.SelectorExpr:
		name := fn.Sel.Name
		// The request, the writer and the context.
		if id, ok := fn.X.(*ast.Ident); ok {
			switch {
			case sc.request != "" && id.Name == sc.request:
				if s := sc.requestCall(call, name); s != nil {
					return s
				}
				return nil
			case sc.writer != "" && id.Name == sc.writer:
				return sc.writerCall(call, name)
			case sc.context != "" && id.Name == sc.context:
				return sc.contextCall(call, name)
			case sc.tx[id.Name]:
				switch name {
				case "Commit":
					return sc.step("transaction", call, "commits the transaction")
				case "Rollback":
					return sc.step("transaction", call, "rolls back the transaction")
				}
				if s := sc.sqlStep(call, true); s != nil {
					return s
				}
				s := sc.step("database", call, "runs "+name+" in the transaction with a statement that is not a literal")
				s.transaction = true
				return s
			}
			if t := g.dxTableOf(f, fn.X); t != nil {
				return sc.dxTableStep(call, t, name)
			}
			if ip := f.imports[id.Name]; ip != "" && sc.types[id.Name] == "" && !sc.outside[id.Name] {
				return sc.packageCall(call, ip, name)
			}
		}
		if x, ok := fn.X.(*ast.SelectorExpr); ok {
			if sc.isRequestHeader(x) && (name == "Get" || name == "Values") && len(call.Args) == 1 {
				return sc.input(call, "header", call.Args[0])
			}
			if t := g.dxTableOf(f, x); t != nil {
				return sc.dxTableStep(call, t, name)
			}
			if exprText(x) == f.importName("net/http")+".DefaultClient" && f.importName("net/http") != "" && name == "Do" {
				return sc.externalStep(call, "", nil, "sends the request "+sc.argsText(call)+" with the default HTTP client")
			}
		}
		if fd, cf := sc.callee(call); fd != nil {
			return sc.callStep(call, fd, cf)
		}
		// json.NewDecoder(request.Body).Decode(&v), and the encoder on the
		// writer.
		if inner, ok := fn.X.(*ast.CallExpr); ok {
			if s := sc.chained(call, inner, name); s != nil {
				return s
			}
			if sc.callOutside(inner) {
				return nil
			}
		}
		key := sc.typeOf(fn.X)
		switch {
		case key != "":
			ip, tname := splitKey(key)
			switch {
			case ip == "database/sql" || strings.Contains(ip, "jackc/pgx") || strings.HasSuffix(ip, "sqlx"):
				if beginNames[name] {
					return sc.step("transaction", call, "begins a transaction")
				}
				if s := sc.sqlStep(call, false); s != nil {
					return s
				}
				if strings.HasPrefix(name, "Query") || strings.HasPrefix(name, "Exec") || strings.HasPrefix(name, "Select") || strings.HasPrefix(name, "Get") {
					return sc.step("database", call, "runs "+name+" with a statement that is not a literal")
				}
				return nil
			case ip == "net/http" && tname == "Client" && name == "Do":
				return sc.externalStep(call, "", nil, "sends the request "+sc.argsText(call)+" with an HTTP client")
			case g.dirOf(ip) != "" || (g.modPath == "" && ip == path.Dir(f.path)):
				ts, _ := g.typeSpecOf(key)
				why := fmt.Sprintf("a method of %s that the files read do not declare", tname)
				if ts != nil {
					if _, isIface := ts.Type.(*ast.InterfaceType); isIface {
						why = fmt.Sprintf("a method of the interface %s, whose implementation syntax does not pick", tname)
					}
				}
				s := sc.step("call", call, "calls "+sc.text(call, call))
				s.stopped, s.call, s.givenRequest = why, call, sc.givenRequest(call)
				return s
			default:
				return nil // a method of a type outside the module
			}
		case sc.isOutside(fn.X):
			return nil
		}
		s := sc.step("call", call, "calls "+sc.text(call, call))
		s.stopped, s.call, s.givenRequest = "a method of a value whose type syntax does not give", call, sc.givenRequest(call)
		return s
	}
	s := sc.step("call", call, "calls "+sc.text(call, call))
	s.stopped, s.call, s.givenRequest = "a call the reader does not place", call, sc.givenRequest(call)
	return s
}

// isLocalFunc says whether a name is a parameter or a local of the
// function, which may hold a function value.
func (sc *fnScope) isLocalFunc(name string) bool {
	found := false
	var ft *ast.FuncType
	if sc.fn.fd != nil {
		ft = sc.fn.fd.Type
	} else {
		ft = sc.fn.lit.Type
	}
	for _, p := range ft.Params.List {
		for _, n := range p.Names {
			found = found || n.Name == name
		}
	}
	var body *ast.BlockStmt
	if sc.fn.fd != nil {
		body = sc.fn.fd.Body
	} else {
		body = sc.fn.lit.Body
	}
	ast.Inspect(body, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok && as.Tok == gotoken.DEFINE {
			for _, l := range as.Lhs {
				if id, ok := l.(*ast.Ident); ok && id.Name == name {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// isOutside says whether an expression's value is of a type outside the
// module, as far as the scope knows: a local noted so, or a field or a
// call reached from one.
func (sc *fnScope) isOutside(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.Ident:
		return sc.outside[e.Name]
	case *ast.SelectorExpr:
		if id, ok := e.X.(*ast.Ident); ok {
			if ip := sc.file.imports[id.Name]; ip != "" && sc.types[id.Name] == "" && !sc.outside[id.Name] {
				return sc.h.g.dirOf(ip) == ""
			}
		}
		if owner := sc.typeOf(e.X); owner != "" {
			// A field of a module type whose type is not a module type.
			return true
		}
		return sc.isOutside(e.X)
	case *ast.CallExpr:
		return sc.callOutside(e)
	case *ast.ParenExpr:
		return sc.isOutside(e.X)
	case *ast.StarExpr:
		return sc.isOutside(e.X)
	case *ast.IndexExpr:
		return sc.isOutside(e.X)
	}
	return false
}

// callOutside says whether a call's value comes from outside the module.
func (sc *fnScope) callOutside(call *ast.CallExpr) bool {
	if fd, _ := sc.callee(call); fd != nil {
		return false
	}
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return builtinFuncs[fn.Name]
	case *ast.SelectorExpr:
		if id, ok := fn.X.(*ast.Ident); ok {
			if ip := sc.file.imports[id.Name]; ip != "" && sc.types[id.Name] == "" && !sc.outside[id.Name] {
				return sc.h.g.dirOf(ip) == ""
			}
			if id.Name == sc.request || id.Name == sc.writer || id.Name == sc.context {
				return true
			}
		}
		if key := sc.typeOf(fn.X); key != "" {
			ip, _ := splitKey(key)
			return sc.h.g.dirOf(ip) == ""
		}
		return sc.isOutside(fn.X)
	}
	return false
}

// givenRequest says whether a call is given the request, the writer or
// the context, so that the function called may read or answer.
func (sc *fnScope) givenRequest(call *ast.CallExpr) bool {
	for _, a := range call.Args {
		for _, n := range sc.given {
			if id, ok := a.(*ast.Ident); ok && id.Name == n {
				return true
			}
			if sel, ok := a.(*ast.SelectorExpr); ok && exprText(sel) == n+".Request" {
				return true
			}
		}
	}
	return false
}

// argsText is the arguments of a call as the code writes them.
func (sc *fnScope) argsText(call *ast.CallExpr) string {
	if len(call.Args) == 0 {
		return "nothing"
	}
	return sc.text(call.Args[0], call.Args[len(call.Args)-1])
}

// callStep is the step of a call of a function of the module: followed,
// when the depth and the record's size allow, or stopped with why.
func (sc *fnScope) callStep(call *ast.CallExpr, fd *ast.FuncDecl, cf *goFile) *goStep {
	h := sc.h
	s := sc.step("call", call, "calls "+sc.text(call, call))
	s.call = call
	if fn := h.byDecl[fd]; fn != nil {
		s.function = fn.name
		if fn == sc.fn {
			s.function = ""
			s.stopped = "a call of the function itself"
		}
		return s
	}
	if fd.Body == nil {
		s.stopped, s.givenRequest = "a function with no body in Go", sc.givenRequest(call)
		return s
	}
	if sc.fn.depth+1 > handlerDepth {
		s.stopped, s.givenRequest = fmt.Sprintf("deeper than the %d calls followed from the handler", handlerDepth), sc.givenRequest(call)
		return s
	}
	if len(h.functions) >= handlerFunctions {
		s.stopped, s.givenRequest = fmt.Sprintf("the record already expands %d functions", handlerFunctions), sc.givenRequest(call)
		return s
	}
	fn := h.follow(cf, fd, nil, sc.fn.depth+1)
	s.function = fn.name
	return s
}

// packageCall is the step of a call of a package's function: the module's
// are followed, net/http's clients are external calls, the standard
// library's are not steps, and any other library's is a call not
// followed.
func (sc *fnScope) packageCall(call *ast.CallExpr, ip, name string) *goStep {
	g := sc.h.g
	if fd, cf := sc.callee(call); fd != nil {
		return sc.callStep(call, fd, cf)
	}
	if ip == "net/http" {
		if spec, ok := clientCalls[name]; ok && len(call.Args) > spec.url {
			return sc.externalStep(call, name, &spec, "")
		}
		switch name {
		case "Error":
			if len(call.Args) == 3 && sc.writer != "" && exprText(call.Args[0]) == sc.writer {
				status, _ := g.statusValue(sc.file, call.Args[2])
				text := "answers " + statusText(status)
				if msg, ok := g.stringValue(sc.file, call.Args[1]); ok {
					text += ": " + msg
				}
				s := sc.step("response", call, text)
				s.status = status
				sc.fn.hasResponse = true
				return s
			}
		case "NotFound":
			s := sc.step("response", call, "answers 404")
			s.status = 404
			sc.fn.hasResponse = true
			return s
		case "Redirect":
			if len(call.Args) == 4 {
				status, _ := g.statusValue(sc.file, call.Args[3])
				s := sc.step("response", call, "redirects with "+statusText(status)+" to "+sc.text(call.Args[2], call.Args[2]))
				s.status = status
				sc.fn.hasResponse = true
				return s
			}
		case "ServeFile", "ServeContent":
			s := sc.step("response", call, "answers with a file")
			sc.fn.hasResponse = true
			return s
		}
		return nil
	}
	if routerLibraries[ip] == "chi" && (name == "URLParam" || name == "URLParamFromCtx") && len(call.Args) == 2 {
		return sc.input(call, "path parameter", call.Args[1])
	}
	if g.dirOf(ip) != "" {
		s := sc.step("call", call, "calls "+sc.text(call, call))
		s.stopped, s.call, s.givenRequest = "not a function of the files read", call, sc.givenRequest(call)
		return s
	}
	if standardLibrary(ip) {
		return nil
	}
	s := sc.step("call", call, "calls "+sc.text(call, call))
	s.stopped, s.call, s.givenRequest = "outside the module ("+ip+")", call, sc.givenRequest(call)
	return s
}

// standardLibrary says whether an import path is the standard library's:
// its first element has no dot.
func standardLibrary(ip string) bool {
	first := ip
	if i := strings.Index(ip, "/"); i >= 0 {
		first = ip[:i]
	}
	return !strings.Contains(first, ".")
}

// statusText is a status as the chart says it.
func statusText(status int) string {
	if status == 0 {
		return "a status that is not a literal"
	}
	return fmt.Sprint(status)
}

// externalStep is the step of a call of another system over HTTP.
func (sc *fnScope) externalStep(call *ast.CallExpr, name string, spec *struct {
	method, url int
	named       string
}, text string) *goStep {
	g := sc.h.g
	if spec == nil {
		return sc.step("external", call, text)
	}
	method := spec.named
	if spec.method >= 0 {
		if m, ok := g.methodValue(sc.file, call.Args[spec.method]); ok {
			method = strings.ToUpper(m)
		} else {
			method = "a method that is not a literal"
		}
	}
	u, ok := g.stringValue(sc.file, call.Args[spec.url])
	if !ok {
		return sc.step("external", call, "calls another system with "+method+" at a URL that is not a literal")
	}
	s := sc.step("external", call, "calls "+method+" "+u)
	if dep := dependencyOf(u); dep != "" {
		s.dependency = dep
	}
	return s
}

// dxTableOf is the dxlib table an expression names: a name of this
// package or of another package of the module through its import.
func (g *goReader) dxTableOf(f *goFile, e ast.Expr) *dxTable {
	key := path.Dir(f.path) + "|" + exprText(e)
	if x, ok := e.(*ast.SelectorExpr); ok {
		if id, ok := x.X.(*ast.Ident); ok {
			if dir := g.dirOf(f.imports[id.Name]); dir != "" {
				key = dir + "|" + x.Sel.Name
			}
		}
	}
	return g.dxTables[key]
}

// dxTableStep is a database step on a dxlib table.
func (sc *fnScope) dxTableStep(call *ast.CallExpr, t *dxTable, method string) *goStep {
	access := dxAccess(method)
	verb := map[string]string{"read": "reads", "write": "writes"}[access]
	text := verb + " "
	if t.table != "" {
		text += t.table
	} else {
		text += "a table whose name is not a literal"
	}
	s := sc.step("database", call, text+" with "+method)
	s.table, s.access = t.table, access
	s.transaction = strings.HasPrefix(method, "Tx")
	return s
}

// requestCall is the step of a call on dxlib's request or net/http's.
func (sc *fnScope) requestCall(call *ast.CallExpr, name string) *goStep {
	g := sc.h.g
	f := sc.file
	switch {
	case strings.HasPrefix(name, "GetParameterValue") && name != "GetParameterValues":
		return sc.input(call, "parameter", firstArg(call))
	case name == "GetParameterValues":
		s := sc.step("input", call, "reads the parameters as a map")
		return s
	case problemCalls[name]:
		status, _ := g.statusValue(f, firstArg(call))
		s := sc.step("response", call, "refuses with "+statusText(status))
		s.status = status
		if name != "WriteResponseAsError" && len(call.Args) > 1 {
			if reason, ok := g.stringValue(f, call.Args[1]); ok && problemName(reason) != "" {
				s.problem = problemName(reason)
				s.text += " " + s.problem
			} else if !ok {
				s.text += ", a reason that is not a literal"
			}
		}
		sc.fn.hasResponse = true
		return s
	case strings.HasPrefix(name, "WriteResponse"):
		status, _ := g.statusValue(f, firstArg(call))
		with := strings.TrimPrefix(name, "WriteResponseAs")
		text := "answers " + statusText(status)
		switch with {
		case "JSON":
			text += " with JSON"
		case "String":
			text += " with a string"
		case "Bytes":
			text += " with bytes"
		case "File":
			text += " with a file"
		}
		s := sc.step("response", call, text)
		s.status = status
		if len(call.Args) >= 3 {
			s.responseValue = call.Args[len(call.Args)-1]
			s.fixedValue = literalValuesOnly(call.Args[len(call.Args)-1])
		}
		sc.fn.hasResponse = true
		return s
	case name == "PathValue" && len(call.Args) == 1:
		return sc.input(call, "path parameter", call.Args[0])
	case (name == "FormValue" || name == "PostFormValue") && len(call.Args) == 1:
		return sc.input(call, "form value", call.Args[0])
	case name == "Cookie" && len(call.Args) == 1:
		return sc.input(call, "cookie", call.Args[0])
	case name == "ParseForm", name == "ParseMultipartForm":
		return sc.step("input", call, "reads the form")
	}
	return nil
}

// chained is the step of a call on a call's value: a decoder on the
// request's body, an encoder on the writer, the query of the request's URL.
func (sc *fnScope) chained(call *ast.CallExpr, inner *ast.CallExpr, name string) *goStep {
	f := sc.file
	json := f.importName("encoding/json")
	innerFun := exprText(inner.Fun)
	switch {
	case json != "" && innerFun == json+".NewDecoder" && name == "Decode" && len(inner.Args) == 1 && len(call.Args) == 1 && sc.isRequestBody(inner.Args[0]):
		s := sc.step("input", call, "decodes the request body into "+strings.TrimPrefix(sc.text(call.Args[0], call.Args[0]), "&"))
		s.parameter = "the body"
		sc.h.lastRequested = s
		return s
	case json != "" && innerFun == json+".NewEncoder" && name == "Encode" && len(inner.Args) == 1 && sc.writer != "" && exprText(inner.Args[0]) == sc.writer:
		s := sc.step("response", call, "answers with JSON")
		if len(call.Args) == 1 {
			s.responseValue = call.Args[0]
			s.fixedValue = literalValuesOnly(call.Args[0])
		}
		sc.fn.hasResponse = true
		return s
	case name == "Get" && len(call.Args) == 1 && sc.request != "" && (exprText(inner) == sc.request+".URL.Query()" || exprText(inner.Fun) == sc.request+".URL.Query"):
		return sc.input(call, "query parameter", call.Args[0])
	}
	return nil
}

// isRequestBody says whether an expression is the request's body.
func (sc *fnScope) isRequestBody(e ast.Expr) bool {
	t := exprText(e)
	return sc.request != "" && (t == sc.request+".Body" || t == sc.request+".Request.Body")
}

// isRequestHeader says whether an expression is the request's headers:
// net/http's Header field on the request, on dxlib's request through its
// Request, or on echo's context through Request().
func (sc *fnScope) isRequestHeader(e *ast.SelectorExpr) bool {
	if e.Sel.Name != "Header" {
		return false
	}
	t := exprText(e.X)
	if sc.request != "" && (t == sc.request || t == sc.request+".Request") {
		return true
	}
	return sc.context != "" && t == sc.context+".Request(...)"
}

// writerCall is the step of a call on net/http's response writer.
func (sc *fnScope) writerCall(call *ast.CallExpr, name string) *goStep {
	g := sc.h.g
	switch name {
	case "WriteHeader":
		status, _ := g.statusValue(sc.file, firstArg(call))
		s := sc.step("response", call, "writes the status "+statusText(status))
		s.status = status
		sc.fn.hasResponse = true
		return s
	case "Write", "WriteString":
		s := sc.step("response", call, "writes the response body")
		sc.fn.hasResponse = true
		return s
	}
	return nil
}

var contextResponses = map[string]bool{"JSON": true, "IndentedJSON": true, "PureJSON": true, "SecureJSON": true, "AsciiJSON": true, "JSONP": true, "XML": true, "YAML": true, "TOML": true, "ProtoBuf": true,
	"String": true, "Data": true, "Status": true, "AbortWithStatus": true, "AbortWithStatusJSON": true, "AbortWithError": true, "Redirect": true, "HTML": true, "Blob": true, "NoContent": true, "Stream": true, "File": true, "Attachment": true, "Inline": true, "JSONBlob": true, "JSONPretty": true}

// contextCall is the step of a call on gin's or echo's context.
func (sc *fnScope) contextCall(call *ast.CallExpr, name string) *goStep {
	g := sc.h.g
	switch {
	case name == "Param" && len(call.Args) == 1:
		return sc.input(call, "path parameter", call.Args[0])
	case (name == "Query" || name == "DefaultQuery" || name == "QueryParam") && len(call.Args) >= 1:
		return sc.input(call, "query parameter", call.Args[0])
	case (name == "GetHeader") && len(call.Args) == 1:
		return sc.input(call, "header", call.Args[0])
	case (name == "PostForm" || name == "FormValue" || name == "DefaultPostForm") && len(call.Args) >= 1:
		return sc.input(call, "form value", call.Args[0])
	case strings.HasPrefix(name, "ShouldBind") || strings.HasPrefix(name, "Bind") || name == "MustBind":
		text := "decodes the request body"
		if len(call.Args) == 1 {
			text += " into " + strings.TrimPrefix(sc.text(call.Args[0], call.Args[0]), "&")
		}
		s := sc.step("input", call, text)
		s.parameter = "the body"
		sc.h.lastRequested = s
		return s
	case contextResponses[name]:
		status := 0
		if len(call.Args) >= 1 {
			status, _ = g.statusValue(sc.file, call.Args[0])
		}
		text := "answers " + statusText(status)
		switch {
		case strings.Contains(name, "JSON"):
			text += " with JSON"
		case name == "String":
			text += " with a string"
		case name == "Redirect":
			text = "redirects with " + statusText(status)
		}
		s := sc.step("response", call, text)
		s.status = status
		if strings.Contains(name, "JSON") && len(call.Args) >= 2 {
			s.responseValue = call.Args[1]
			s.fixedValue = literalValuesOnly(call.Args[1])
		}
		sc.fn.hasResponse = true
		return s
	}
	return nil
}

// literalValuesOnly says whether an expression is literal values alone: a
// literal, or a composite literal of literals, which a handler answering
// it has made up rather than read.
func literalValuesOnly(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		return e.Name == "true" || e.Name == "false"
	case *ast.UnaryExpr:
		return literalValuesOnly(e.X)
	case *ast.KeyValueExpr:
		return literalValuesOnly(e.Value)
	case *ast.CompositeLit:
		if len(e.Elts) == 0 {
			return false
		}
		for _, el := range e.Elts {
			if !literalValuesOnly(el) {
				return false
			}
		}
		return true
	}
	return false
}
