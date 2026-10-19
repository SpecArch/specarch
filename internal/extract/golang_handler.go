package extract

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	gotoken "go/token"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// What a dxlib handler gives of its request body and its responses when
// the registration declares none (ADR-140): a service whose every
// NewEndPoint passes nil for its parameters and its response possibilities
// decodes the body itself, json.Unmarshal(aepr.RequestBodyAsBytes, &req),
// checks the fields it needs before it goes on, and answers each outcome
// with aepr.WriteResponseAsJSON(status, header, utils.JSON{...}). The
// struct the body is decoded into is the body's schema, the checks that
// refuse before the handler goes on are its required fields, bounds and
// enums, and each literal answer is a response under its status.

// dxlibUtils is the package that declares utils.JSON, the map a handler
// answers with.
const dxlibUtils = "github.com/donnyhardyanto/dxlib/utils"

// handlerDecode is where a handler decodes its request body.
type handlerDecode struct {
	clause string
	file   *goFile
	text   string   // the target, as the code writes it
	name   string   // the variable decoded into, "" when the target is not one
	typ    ast.Expr // the type the variable is declared with, nil when not found
	from   string   // what is decoded: "RequestBodyAsBytes" or "Request.Body"
	source string   // what is decoded, as the code writes it
	guards []handlerGuard
}

// handlerGuard is one check of a decoded field that refuses the request.
type handlerGuard struct {
	clause string
	path   []string // the Go names from the variable to the field
	kind   string   // "required", "nonEmpty", "length", "value", "enum", or "" for one not read
	text   string   // the condition, as the code writes it
	op     gotoken.Token
	n      string   // the bound, as the code writes it
	values []string // the enum's values
}

// handlerAnswer is one WriteResponseAsJSON call of a handler.
type handlerAnswer struct {
	clause string
	file   *goFile
	call   *ast.CallExpr
}

// handlerRead is what the handler of an endpoint gives beside its
// parameter reads and refusals.
type handlerRead struct {
	decode  *handlerDecode
	decodes int
	answers []handlerAnswer
	locals  map[string]ast.Expr // a name -> the type it is declared with
	file    *goFile
}

// readHandlerShapes notes the body decode and the JSON answers of a
// handler, and the guards on the decoded value.
func (g *goReader) readHandlerShapes(ep *goEndpoint, file *goFile, body *ast.BlockStmt, request string) {
	h := &handlerRead{locals: handlerLocals(body), file: file}
	jsonName := file.importName("encoding/json")
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch {
		case exprText(sel.X) == request && sel.Sel.Name == "WriteResponseAsJSON" && len(call.Args) == 3:
			h.answers = append(h.answers, handlerAnswer{clause: g.clause(file, call.Pos()), file: file, call: call})
		case jsonName != "" && exprText(call.Fun) == jsonName+".Unmarshal" && len(call.Args) == 2 && exprText(call.Args[0]) == request+".RequestBodyAsBytes":
			h.note(g, file, call, call.Args[1], "RequestBodyAsBytes", exprText(call.Args[0]))
		case jsonName != "" && sel.Sel.Name == "Decode" && len(call.Args) == 1:
			dec, ok := sel.X.(*ast.CallExpr)
			if !ok || len(dec.Args) != 1 || exprText(dec.Fun) != jsonName+".NewDecoder" {
				break
			}
			switch src := exprText(dec.Args[0]); {
			case src == request+".Request.Body":
				h.note(g, file, call, call.Args[0], "Request.Body", src)
			case readerOfBytes(file, dec.Args[0], request):
				h.note(g, file, call, call.Args[0], "RequestBodyAsBytes", exprText(dec.Args[0]))
			}
		}
		return true
	})
	if h.decode != nil && h.decode.name != "" {
		h.decode.guards = g.handlerGuards(file, body, request, h.decode.name)
	}
	if h.decode != nil || len(h.answers) > 0 {
		ep.shapes = h
	}
}

// readerOfBytes says whether e is bytes.NewReader or bytes.NewBuffer of
// the request's body.
func readerOfBytes(f *goFile, e ast.Expr, request string) bool {
	c, ok := e.(*ast.CallExpr)
	b := f.importName("bytes")
	if !ok || b == "" || len(c.Args) != 1 || exprText(c.Args[0]) != request+".RequestBodyAsBytes" {
		return false
	}
	fn := exprText(c.Fun)
	return fn == b+".NewReader" || fn == b+".NewBuffer"
}

// note records a decode of the body; a handler that decodes it twice is
// read at its first.
func (h *handlerRead) note(g *goReader, f *goFile, call *ast.CallExpr, target ast.Expr, from, source string) {
	h.decodes++
	if h.decode != nil {
		return
	}
	if u, ok := target.(*ast.UnaryExpr); ok && u.Op == gotoken.AND {
		target = u.X
	}
	d := &handlerDecode{clause: g.clause(f, call.Pos()), file: f, text: exprText(target), from: from, source: source}
	if id, ok := target.(*ast.Ident); ok {
		d.name = id.Name
		d.typ = h.locals[id.Name]
	}
	h.decode = d
}

// typeText is a type as the code writes it, a struct type written in place
// as struct{...}.
func typeText(e ast.Expr) string {
	if _, ok := e.(*ast.StructType); ok {
		return "struct{...}"
	}
	return exprText(e)
}

// handlerLocals is the type each name of a handler is declared with:
// var x T, x := T{...}, x := &T{...} and x := new(T).
func handlerLocals(body *ast.BlockStmt) map[string]ast.Expr {
	locals := map[string]ast.Expr{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ValueSpec:
			if n.Type != nil {
				for _, id := range n.Names {
					locals[id.Name] = n.Type
				}
			} else if len(n.Names) == len(n.Values) {
				for i, id := range n.Names {
					if t := declaredType(n.Values[i]); t != nil {
						locals[id.Name] = t
					}
				}
			}
		case *ast.AssignStmt:
			if n.Tok == gotoken.DEFINE && len(n.Lhs) == 1 && len(n.Rhs) == 1 {
				if id, ok := n.Lhs[0].(*ast.Ident); ok {
					if t := declaredType(n.Rhs[0]); t != nil {
						locals[id.Name] = t
					}
				}
			}
		}
		return true
	})
	return locals
}

// declaredType is the type a composite literal, a pointer to one or new(T)
// gives, or nil.
func declaredType(e ast.Expr) ast.Expr {
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == gotoken.AND {
		e = u.X
	}
	switch v := e.(type) {
	case *ast.CompositeLit:
		return v.Type
	case *ast.CallExpr:
		if fn, ok := v.Fun.(*ast.Ident); ok && fn.Name == "new" && len(v.Args) == 1 {
			return v.Args[0]
		}
	}
	return nil
}

// handlerGuards reads the checks of the decoded variable that refuse the
// request: an if of the handler's body whose condition compares a field
// and whose block refuses, an else if of its chain when each branch before
// it refuses too, and a switch of the body on a field whose default
// refuses. Only those hold the field to a rule before the handler goes
// on; a refusing check under another condition, a loop or a closure holds
// it only there, and is one not read.
func (g *goReader) handlerGuards(f *goFile, body *ast.BlockStmt, request, name string) []handlerGuard {
	var out []handlerGuard
	read := func(s *ast.IfStmt) {
		clause := g.clause(f, s.Pos())
		for _, c := range disjuncts(s.Cond) {
			gd, ok := guardOf(c, name)
			if !ok {
				if !mentions(c, name) {
					continue
				}
				gd = handlerGuard{}
			}
			gd.clause, gd.text = clause, g.source(c)
			out = append(out, gd)
		}
	}
	for _, st := range body.List {
		switch s := st.(type) {
		case *ast.IfStmt:
			before, named := true, false
			var conds []string
			var cur ast.Stmt = s
			for cur != nil {
				is, ok := cur.(*ast.IfStmt)
				if !ok {
					out = append(out, g.nestedGuards(f, cur, request, name, "in the else of "+strings.Join(conds, " and "), named)...)
					break
				}
				refusing := g.refuses(f, is.Body, request)
				switch {
				case refusing && before:
					read(is)
				case refusing && mentions(is.Cond, name):
					out = append(out, handlerGuard{clause: g.clause(f, is.Pos()), text: g.source(is.Cond) + ", in the else of " + strings.Join(conds, " and ")})
				}
				named = named || mentions(is.Cond, name)
				out = append(out, g.nestedGuards(f, is.Body, request, name, "under "+g.source(is.Cond), mentions(is.Cond, name))...)
				before = before && refusing
				conds = append(conds, g.source(is.Cond))
				cur = is.Else
			}
		case *ast.SwitchStmt:
			out = append(out, g.switchGuard(f, s, request, name)...)
			where, named := "in a case of the switch at "+g.clause(f, s.Pos()), false
			if s.Tag != nil {
				where, named = "in a case of the switch on "+g.source(s.Tag), mentions(s.Tag, name)
			}
			for _, cs := range s.Body.List {
				out = append(out, g.nestedGuards(f, cs, request, name, where, named)...)
			}
		default:
			out = append(out, g.nestedGuards(f, st, request, name, "inside the statement at "+g.clause(f, st.Pos()), false)...)
		}
	}
	return out
}

// switchGuard is the enum a switch on a field whose cases are strings and
// whose default refuses gives, or nothing.
func (g *goReader) switchGuard(f *goFile, s *ast.SwitchStmt, request, name string) []handlerGuard {
	path, ok := fieldPath(s.Tag, name)
	if !ok {
		return nil
	}
	var values []string
	refused, literal := false, true
	for _, st := range s.Body.List {
		cc := st.(*ast.CaseClause)
		if cc.List == nil {
			refused = g.refuses(f, &ast.BlockStmt{List: cc.Body}, request)
			continue
		}
		for _, e := range cc.List {
			v, ok := stringLit(e)
			literal = literal && ok
			values = append(values, v)
		}
	}
	if !refused || !literal || len(values) == 0 {
		return nil
	}
	return []handlerGuard{{clause: g.clause(f, s.Pos()), path: path, kind: "enum", values: values}}
}

// nestedGuards is each refusing check below a statement of the handler's
// body that names a field, or sits under a condition that does (named), a
// check not read, with where it sits.
func (g *goReader) nestedGuards(f *goFile, n ast.Node, request, name, where string, named bool) []handlerGuard {
	var out []handlerGuard
	ast.Inspect(n, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.IfStmt:
			if g.refuses(f, s.Body, request) && (named || mentions(s.Cond, name)) {
				out = append(out, handlerGuard{clause: g.clause(f, s.Pos()), text: g.source(s.Cond) + ", " + where})
			}
		case *ast.SwitchStmt:
			if gd := g.switchGuard(f, s, request, name); gd != nil {
				out = append(out, handlerGuard{clause: gd[0].clause, text: "a switch on " + g.source(s.Tag) + " has no case it takes, " + where})
			}
		}
		return true
	})
	return out
}

// mentions says whether an expression names a field of the variable.
func mentions(e ast.Expr, name string) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if _, ok := fieldPath(sel, name); ok {
				found = true
			}
		}
		return !found
	})
	return found
}

// source is an expression as the code writes it.
func (g *goReader) source(e ast.Expr) string {
	var b bytes.Buffer
	if printer.Fprint(&b, g.fset, e) != nil {
		return exprText(e)
	}
	return b.String()
}

// refuses says whether a block answers a refusal through the request by
// a statement of its own: a return of, or a call to, one of the problem
// calls, or WriteResponseAsJSON with a literal status from 400 up. A
// refusal inside a statement of the block, an if, a switch, a loop or a
// closure, is answered only there, so the block does not refuse.
func (g *goReader) refuses(f *goFile, b *ast.BlockStmt, request string) bool {
	for _, st := range b.List {
		var calls []ast.Expr
		switch s := st.(type) {
		case *ast.ExprStmt:
			calls = []ast.Expr{s.X}
		case *ast.ReturnStmt:
			calls = s.Results
		}
		for _, e := range calls {
			if g.refusal(f, e, request) {
				return true
			}
		}
	}
	return false
}

// refusal says whether an expression is a call that answers a refusal
// through the request.
func (g *goReader) refusal(f *goFile, e ast.Expr, request string) bool {
	call, ok := unparen(e).(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || exprText(sel.X) != request {
		return false
	}
	switch {
	case problemCalls[sel.Sel.Name]:
		return true
	case sel.Sel.Name == "WriteResponseAsJSON" && len(call.Args) == 3:
		s, ok := g.answerStatus(f, call.Args[0])
		return ok && s >= 400
	}
	return false
}

// disjuncts splits a condition at ||.
func disjuncts(e ast.Expr) []ast.Expr {
	e = unparen(e)
	if b, ok := e.(*ast.BinaryExpr); ok && b.Op == gotoken.LOR {
		return append(disjuncts(b.X), disjuncts(b.Y)...)
	}
	return []ast.Expr{e}
}

// conjuncts splits a condition at &&.
func conjuncts(e ast.Expr) []ast.Expr {
	e = unparen(e)
	if b, ok := e.(*ast.BinaryExpr); ok && b.Op == gotoken.LAND {
		return append(conjuncts(b.X), conjuncts(b.Y)...)
	}
	return []ast.Expr{e}
}

func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

// fieldPath is the Go names from the variable to a field: req.A.B gives
// A and B.
func fieldPath(e ast.Expr, name string) ([]string, bool) {
	var path []string
	e = unparen(e)
	for {
		switch x := e.(type) {
		case *ast.SelectorExpr:
			path = append([]string{x.Sel.Name}, path...)
			e = unparen(x.X)
			continue
		case *ast.Ident:
			return path, x.Name == name && len(path) > 0
		}
		return nil, false
	}
}

func stringLit(e ast.Expr) (string, bool) {
	b, ok := unparen(e).(*ast.BasicLit)
	if !ok || b.Kind != gotoken.STRING {
		return "", false
	}
	s, err := strconv.Unquote(b.Value)
	return s, err == nil
}

// numberLit is an integer or a float literal, negative too, as written.
func numberLit(e ast.Expr) (string, bool) {
	e = unparen(e)
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == gotoken.SUB {
		s, ok := numberLit(u.X)
		return "-" + s, ok
	}
	b, ok := e.(*ast.BasicLit)
	if !ok || (b.Kind != gotoken.INT && b.Kind != gotoken.FLOAT) {
		return "", false
	}
	return b.Value, true
}

// flipped is a comparison with its operands swapped: 3 > x is x < 3.
var flipped = map[gotoken.Token]gotoken.Token{gotoken.LSS: gotoken.GTR, gotoken.GTR: gotoken.LSS, gotoken.LEQ: gotoken.GEQ, gotoken.GEQ: gotoken.LEQ, gotoken.EQL: gotoken.EQL, gotoken.NEQ: gotoken.NEQ}

// guardOf reads one condition that refuses: a field equal to "" or nil, a
// length or a value against a literal bound, or a field unequal to each of
// a list of strings none of which is "".
func guardOf(c ast.Expr, name string) (handlerGuard, bool) {
	if parts := conjuncts(c); len(parts) > 1 {
		var path []string
		var values []string
		for _, p := range parts {
			b, ok := p.(*ast.BinaryExpr)
			if !ok || b.Op != gotoken.NEQ {
				return handlerGuard{}, false
			}
			fp, ok := fieldPath(b.X, name)
			v, lit := stringLit(b.Y)
			if !ok || !lit || v == "" || (path != nil && strings.Join(fp, ".") != strings.Join(path, ".")) {
				return handlerGuard{}, false
			}
			path = fp
			values = append(values, v)
		}
		return handlerGuard{path: path, kind: "enum", values: values}, true
	}
	b, ok := c.(*ast.BinaryExpr)
	if !ok || flipped[b.Op] == 0 {
		return handlerGuard{}, false
	}
	x, y, op := b.X, b.Y, b.Op
	if _, isField := fieldPath(x, name); !isField && !isLen(x, name) {
		x, y, op = y, x, flipped[op]
	}
	if call, ok := unparen(x).(*ast.CallExpr); ok && isLen(call, name) {
		path, _ := fieldPath(call.Args[0], name)
		n, lit := numberLit(y)
		if !lit {
			return handlerGuard{}, false
		}
		if op == gotoken.EQL && n == "0" {
			return handlerGuard{path: path, kind: "nonEmpty"}, true
		}
		if op == gotoken.EQL || op == gotoken.NEQ {
			return handlerGuard{}, false
		}
		return handlerGuard{path: path, kind: "length", op: op, n: n}, true
	}
	path, ok := fieldPath(x, name)
	if !ok {
		return handlerGuard{}, false
	}
	switch {
	case op == gotoken.EQL && exprText(y) == "nil":
		return handlerGuard{path: path, kind: "required"}, true
	case op == gotoken.EQL:
		if s, lit := stringLit(y); lit && s == "" {
			return handlerGuard{path: path, kind: "nonEmpty"}, true
		}
	case op == gotoken.NEQ:
		// x.F != "" refusing tests that the field is there, which says
		// nothing of the value it may hold, so it is a check not read.
		if s, lit := stringLit(y); lit && s != "" {
			return handlerGuard{path: path, kind: "enum", values: []string{s}}, true
		}
	default:
		if n, lit := numberLit(y); lit {
			return handlerGuard{path: path, kind: "value", op: op, n: n}, true
		}
	}
	return handlerGuard{}, false
}

func isLen(e ast.Expr, name string) bool {
	call, ok := unparen(e).(*ast.CallExpr)
	if !ok || exprText(call.Fun) != "len" || len(call.Args) != 1 {
		return false
	}
	_, ok = fieldPath(call.Args[0], name)
	return ok
}

// answerStatus is a literal status, or a constant of net/http's.
func (g *goReader) answerStatus(f *goFile, e ast.Expr) (int, bool) {
	if n, ok := g.statusValue(f, e); ok {
		return n, true
	}
	if sel, ok := e.(*ast.SelectorExpr); ok {
		if x, isIdent := sel.X.(*ast.Ident); isIdent && f.imports[x.Name] == "net/http" {
			n, ok := httpStatusCodes[sel.Sel.Name]
			return n, ok
		}
	}
	return 0, false
}

// omitted says whether encoding/json leaves a field out when it is empty:
// its json tag has omitempty or omitzero.
func omitted(fl *ast.Field) bool {
	if fl.Tag == nil {
		return false
	}
	tag, err := strconv.Unquote(fl.Tag.Value)
	if err != nil {
		return false
	}
	v, _ := reflect.StructTag(tag).Lookup("json")
	opts := strings.Split(v, ",")
	for _, o := range opts[1:] {
		if o == "omitempty" || o == "omitzero" {
			return true
		}
	}
	return false
}

// structFields is the struct a type expression gives, a pointer taken
// off and a named type followed to the struct it is, with the file it is
// in.
func (g *goReader) structFields(f *goFile, e ast.Expr) (*ast.StructType, *goFile) {
	for i := 0; i < 8; i++ {
		switch t := e.(type) {
		case *ast.StarExpr:
			e = t.X
			continue
		case *ast.StructType:
			return t, f
		}
		ts, tf := g.structOf(f, e)
		if ts == nil {
			return nil, nil
		}
		if st, ok := ts.Type.(*ast.StructType); ok {
			return st, tf
		}
		e, f = ts.Type, tf
	}
	return nil, nil
}

// propertyName is the name a field's wire name is written by, as
// structSchema writes it.
func propertyName(wire string, snake bool) (string, bool) {
	if snake {
		return wireCamel(wire)
	}
	return wire, memberNameWord.MatchString(wire)
}

// markRequired writes the fields encoding/json always writes, those with
// no omitempty, as required, in the order of their declaration, beside
// those a validator's tag already requires.
func (g *goReader) markRequired(f *goFile, e ast.Expr, s *yaml.Node, snake bool, depth int) {
	if s == nil || depth > 8 {
		return
	}
	for {
		switch t := e.(type) {
		case *ast.StarExpr:
			e = t.X
			continue
		case *ast.ArrayType:
			g.markRequired(f, t.Elt, child(s, "items"), snake, depth+1)
			return
		case *ast.MapType:
			if ap := child(s, "additionalProperties"); ap != nil && ap.Kind == yaml.MappingNode {
				g.markRequired(f, t.Value, ap, snake, depth+1)
			}
			return
		}
		break
	}
	st, tf := g.structFields(f, e)
	props := child(s, "properties")
	if st == nil || props == nil {
		return
	}
	already := map[string]bool{}
	for _, r := range scalarList(child(s, "required")) {
		already[r] = true
	}
	var required []string
	for _, fl := range st.Fields.List {
		for _, n := range fl.Names {
			wire, skip, _ := jsonTag(fl, n.Name)
			if skip || !n.IsExported() {
				continue
			}
			name, ok := propertyName(wire, snake)
			p := child(props, name)
			if !ok || p == nil {
				continue
			}
			if already[name] || !omitted(fl) {
				required = append(required, name)
			}
			g.markRequired(tf, fl.Type, p, snake, depth+1)
		}
	}
	if len(required) > 0 {
		set(s, "required", required)
	}
}

// guardTarget is the schema of the field a guard checks, the object it is
// a property of and its name there.
func (g *goReader) guardTarget(f *goFile, typ ast.Expr, path []string, s *yaml.Node, snake bool) (*yaml.Node, *yaml.Node, string) {
	var parent *yaml.Node
	name := ""
	for _, goName := range path {
		st, tf := g.structFields(f, typ)
		if st == nil {
			return nil, nil, ""
		}
		var next ast.Expr
		for _, fl := range st.Fields.List {
			for _, n := range fl.Names {
				if n.Name != goName {
					continue
				}
				wire, skip, _ := jsonTag(fl, n.Name)
				pn, ok := propertyName(wire, snake)
				if skip || !ok {
					return nil, nil, ""
				}
				next, name = fl.Type, pn
			}
		}
		if next == nil {
			return nil, nil, ""
		}
		parent = s
		s = child(child(s, "properties"), name)
		if s == nil {
			return nil, nil, ""
		}
		typ, f = next, tf
	}
	return s, parent, name
}

// applyGuard writes what one guard says into the body's schema, and says
// whether it was written.
func (g *goReader) applyGuard(d *handlerDecode, schema *yaml.Node, gd handlerGuard, snake bool) bool {
	s, parent, name := g.guardTarget(d.file, d.typ, gd.path, schema, snake)
	if s == nil {
		return false
	}
	base := ""
	if t := child(s, "type"); t != nil {
		base = t.Value
		if t.Kind == yaml.SequenceNode {
			for _, v := range scalarList(t) {
				if v != "null" {
					base = v
				}
			}
		}
	}
	require := func() {
		if t := child(s, "type"); t != nil && t.Kind == yaml.SequenceNode {
			*t = *str(base)
		}
		if !contains(scalarList(child(parent, "required")), name) {
			if req := child(parent, "required"); req != nil {
				req.Content = append(req.Content, str(name))
			} else {
				set(parent, "required", []string{name})
			}
		}
	}
	kind := map[string]string{"string": "Length", "array": "Items"}[base]
	switch gd.kind {
	case "required":
		require()
	case "nonEmpty":
		if kind == "" {
			return false
		}
		require()
		set(s, "min"+kind, 1)
	case "length":
		n, err := strconv.ParseInt(gd.n, 10, 64)
		if kind == "" || err != nil {
			return false
		}
		switch gd.op {
		case gotoken.GTR:
			set(s, "max"+kind, n)
		case gotoken.GEQ:
			set(s, "max"+kind, n-1)
		case gotoken.LSS:
			set(s, "min"+kind, n)
		case gotoken.LEQ:
			set(s, "min"+kind, n+1)
		}
	case "value":
		if base != "integer" && base != "number" {
			return false
		}
		v := numberNode(gd.n)
		switch gd.op {
		case gotoken.GTR:
			set(s, "maximum", v)
		case gotoken.GEQ:
			set(s, "exclusiveMaximum", v)
		case gotoken.LSS:
			set(s, "minimum", v)
		case gotoken.LEQ:
			set(s, "exclusiveMinimum", v)
		}
	case "enum":
		if base != "string" {
			return false
		}
		set(s, "enum", gd.values)
	default:
		return false
	}
	if s.Style == yaml.FlowStyle && len(s.Content) > 8 {
		s.Style = 0
	}
	return true
}

func numberNode(n string) *yaml.Node {
	tag := "!!int"
	if strings.ContainsAny(n, ".eE") {
		tag = "!!float"
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: n}
}

// handlerBodySchema writes the body a handler decodes from the struct it
// decodes it into, with its guards, or says why it is not written.
func (w *declaredWriter) handlerBodySchema(ep *goEndpoint, label, schemaAt string) (*yaml.Node, string) {
	g, d := w.g, ep.shapes.decode
	switch {
	case d.from == "Request.Body":
		return nil, fmt.Sprintf("its handler decodes %s at %s into %s, a stream dxlib has already read to its end into RequestBodyAsBytes for a JSON endpoint, and finds no data there", d.source, d.clause, d.text)
	case d.typ == nil:
		return nil, fmt.Sprintf("its handler decodes its body at %s into %s, whose type the handler does not declare", d.clause, d.text)
	}
	t := g.underlying(d.file, d.typ)
	notObject := fmt.Sprintf("its handler decodes its body at %s into %s, of type %s, and dxlib reads a JSON body into an object and refuses any other with 422 before the handler runs", d.clause, d.text, exprText(d.typ))
	switch {
	case g.isJSONMap(d.file, t) || isOpenMap(t):
		return nil, fmt.Sprintf("its handler decodes its body at %s into %s, of type %s, a map whose keys and values the code does not name", d.clause, d.text, typeText(d.typ))
	case isListType(t):
		return nil, notObject
	}
	if id, ok := t.(*ast.Ident); ok && goScalars[id.Name] {
		return nil, notObject
	}
	schema := g.fieldSchema(d.file, d.typ, schemaAt, "operation "+label+", request body", w.o.snake, map[*ast.TypeSpec]bool{}, d.clause)
	if schema == nil {
		return nil, ""
	}
	g.markRequired(d.file, d.typ, schema, w.o.snake, 0)
	for _, gd := range d.guards {
		if !g.applyGuard(d, schema, gd, w.o.snake) {
			g.question("should", fmt.Sprintf("%s: the handler of %s refuses the request when %s, a check of its body the reader does not write as a rule of the body's schema. Which rule does it hold the field to?", gd.clause, label, gd.text),
				[]string{schemaAt}, "The reader writes a check that compares a field with an empty value, a literal bound or a list of strings; any other is code whose rule is not read by syntax.", g.at(gd.clause, "Refuses the request when "+gd.text+"."))
		}
	}
	return schema, ""
}

// underlying is the type a type expression is, a pointer taken off and a
// named type of the module followed to its declaration; one the reader
// does not find is given back as it is.
func (g *goReader) underlying(f *goFile, e ast.Expr) ast.Expr {
	for i := 0; i < 8; i++ {
		if s, ok := e.(*ast.StarExpr); ok {
			e = s.X
			continue
		}
		if g.isJSONMap(f, e) {
			return e
		}
		ts, tf := g.structOf(f, e)
		if ts == nil {
			return e
		}
		e, f = ts.Type, tf
	}
	return e
}

// goScalars is Go's own types that hold a string, a number or a boolean.
var goScalars = map[string]bool{"string": true, "bool": true, "byte": true, "rune": true, "int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true, "float32": true, "float64": true, "complex64": true, "complex128": true}

func isListType(e ast.Expr) bool {
	_, ok := e.(*ast.ArrayType)
	return ok
}

// isJSONMap says whether a type is dxlib's utils.JSON.
func (g *goReader) isJSONMap(f *goFile, e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && f.imports[x.Name] == dxlibUtils && sel.Sel.Name == "JSON"
}

// guardsText says where a body's guards are, for its citation.
func guardsText(d *handlerDecode) string {
	var at []string
	for _, gd := range d.guards {
		if gd.kind != "" && !contains(at, gd.clause) {
			at = append(at, gd.clause)
		}
	}
	if len(at) == 0 {
		return ""
	}
	return "; the checks at " + joinAnd(at) + " that refuse it give its required fields, bounds and enums"
}

// handlerAnswered is one status a handler answers with WriteResponseAsJSON.
type handlerAnswered struct {
	code    string
	clauses []string
	content *yaml.Node // the object schema of its body; nil for no body
	keys    []string   // its wire names
	text    string     // the body as the code writes it
	texts   []string   // each answer's body, when the status is answered more than once
	why     string     // why its body is not written, "" when it is
}

// handlerResponses reads the handler's WriteResponseAsJSON calls into the
// responses of an operation, beside the ones its registration declares:
// those win, and a body of a status both give that names other fields is
// a disagreement, asked about. It says which codes it wrote.
func (w *declaredWriter) handlerResponses(ep *goEndpoint, responses *yaml.Node, declared map[string]bool, label string) []string {
	g := w.g
	if ep.shapes == nil || len(ep.shapes.answers) == 0 {
		return nil
	}
	if ep.decl != nil && ep.decl.respWhy != "" {
		// The registration's responses are asked for already, with why.
		return nil
	}
	byCode := map[string]*handlerAnswered{}
	var order []string
	for _, a := range ep.shapes.answers {
		status, ok := g.answerStatus(a.file, a.call.Args[0])
		if !ok || status < 100 || status > 599 {
			g.question("must", fmt.Sprintf("%s: the handler of %s answers with WriteResponseAsJSON the status %s, which is not a literal status. Which status does it answer, with which body, and when?", a.clause, label, exprText(a.call.Args[0])),
				[]string{ep.at + "/responses"}, "A status held in a variable or computed is not known by syntax, and a response is keyed by its status.", g.at(a.clause, "Answers with WriteResponseAsJSON and the status "+exprText(a.call.Args[0])+"."))
			continue
		}
		code := strconv.Itoa(status)
		at := ep.at + "/responses/" + code
		got := w.answerBody(ep, a, label, at)
		got.code = code
		if h := headerText(a.call.Args[1]); h != "" {
			g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: the handler of %s answers %s with %s; the meta-model holds no header on a response; left out", a.clause, label, code, h), clause: a.clause, blocks: []string{at}})
		}
		if prior := byCode[code]; prior != nil {
			// Two answers on one status are one response only when both
			// are literal maps of one shape, or both carry no body.
			prior.clauses = append(prior.clauses, a.clause)
			prior.texts = append(prior.texts, got.text)
			if prior.why != "" || got.why != "" || !sameSchema(prior.content, got.content) {
				prior.why = "differs"
			}
			continue
		}
		got.texts = []string{got.text}
		byCode[code] = got
		order = append(order, code)
	}
	sort.Strings(order)
	var wrote []string
	for _, code := range order {
		a := byCode[code]
		at := ep.at + "/responses/" + code
		if declared[code] {
			if a.why == "" {
				names := declaredKeys(ep.decl.responses[code])
				if strings.Join(names, ",") != strings.Join(sortedCopy(a.keys), ",") {
					g.question("must", fmt.Sprintf("Two sources disagree on the body of %s's response %s: its registration declares %s, and its handler answers with %s at %s. Which fields does the response carry?", label, code, fieldsText(names), fieldsText(sortedCopy(a.keys)), joinAnd(a.clauses)),
						[]string{at + "/content"}, "The registration declares the response and is written; the handler is what the client receives, and the two name other fields.", g.at(a.clauses[0], "Answers "+code+" with "+a.text+"."))
				}
			}
			continue
		}
		resp := mapping()
		switch {
		case a.why == "differs":
			g.question("must", fmt.Sprintf("The handler of %s answers %s at %s with bodies the reader cannot tell are one: %s. Which body does the response carry, or are they two outcomes that should have two statuses?", label, code, joinAnd(a.clauses), joinAnd(a.texts)),
				[]string{at + "/content"}, "A response carries one body, and the handler answers the status with more than one.", g.at(a.clauses[0], "Answers "+code+" with more than one body."))
		case a.why != "":
			g.question("must", fmt.Sprintf("%s: the handler of %s answers %s with the body %s, %s. Which fields does it carry, with their types?", a.clauses[0], label, code, a.text, a.why),
				[]string{at + "/content"}, "A body built outside the call is not known by syntax; only a literal map gives its fields.", g.at(a.clauses[0], "Answers "+code+" with "+a.text+"."))
		case a.content != nil:
			set(resp, "content", mapping("application/json", mapping("schema", a.content)))
		}
		set(responses, code, resp)
		wrote = append(wrote, code)
	}
	return wrote
}

// headerText names the headers an answer sets, "" for none.
func headerText(e ast.Expr) string {
	if exprText(e) == "nil" {
		return ""
	}
	cl, ok := unparen(e).(*ast.CompositeLit)
	if !ok {
		return "the headers " + exprText(e) + " holds"
	}
	var names []string
	for _, el := range cl.Elts {
		if kv, ok := el.(*ast.KeyValueExpr); ok {
			n, lit := stringLit(kv.Key)
			names = append(names, nonEmptyText(map[bool]string{true: n}[lit], exprText(kv.Key)))
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return "the " + pluralWord(len(names), "header") + " " + joinAnd(names)
}

func declaredKeys(r *dxResponse) []string {
	if r == nil || r.content == nil {
		return nil
	}
	var names []string
	names = append(names, keys(child(r.content, "properties"))...)
	for _, u := range r.unread {
		if len(u.path) == 2 {
			names = append(names, u.name)
		}
	}
	return sortedCopy(names)
}

func sortedCopy(list []string) []string {
	out := append([]string(nil), list...)
	sort.Strings(out)
	return out
}

func fieldsText(names []string) string {
	if len(names) == 0 {
		return "no field"
	}
	return "the " + pluralWord(len(names), "field") + " " + joinAnd(names)
}

func sameSchema(a, b *yaml.Node) bool {
	if a == nil || b == nil {
		return a == b
	}
	var x, y bytes.Buffer
	if yaml.NewEncoder(&x).Encode(a) != nil || yaml.NewEncoder(&y).Encode(b) != nil {
		return false
	}
	return x.String() == y.String()
}

// answerBody reads the body of one WriteResponseAsJSON call.
func (w *declaredWriter) answerBody(ep *goEndpoint, a handlerAnswer, label, at string) *handlerAnswered {
	body := a.call.Args[2]
	out := &handlerAnswered{clauses: []string{a.clause}, text: shortText(body)}
	if exprText(body) == "nil" {
		out.text = "no body of its own"
		return out
	}
	cl, ok := unparen(body).(*ast.CompositeLit)
	if !ok || !w.g.isJSONLiteral(a.file, cl) {
		out.why = "which is not a literal map, so its fields are built elsewhere"
		return out
	}
	where := "operation " + label + ", response " + at[strings.LastIndex(at, "/")+1:]
	out.content, out.keys = w.literalObject(ep, a.file, cl, at+"/content/application~1json/schema", where, a.clause, 0)
	return out
}

// isJSONLiteral says whether a composite literal is a map with string
// keys: utils.JSON, map[string]any or map[string]interface{}.
func (g *goReader) isJSONLiteral(f *goFile, cl *ast.CompositeLit) bool {
	if g.isJSONMap(f, cl.Type) {
		return true
	}
	m, ok := cl.Type.(*ast.MapType)
	return ok && exprText(m.Key) == "string" && isOpenValue(m.Value)
}

// dxlibEnvelopeKeys are the keys WriteResponseAsJSON adds to a body that
// lacks them; dxlib's emitter leaves them out of a data template, so a
// literal answer's are not written either.
var dxlibEnvelopeKeys = map[string]bool{"status": true, "status_code": true, "reason": true, "reason_message": true}

// literalObject is the object schema a literal map gives: each key a
// property, required, since the handler always writes it, and of the type
// its value gives; at the top, dxlib's envelope keys are left out.
func (w *declaredWriter) literalObject(ep *goEndpoint, f *goFile, cl *ast.CompositeLit, at, where, clause string, depth int) (*yaml.Node, []string) {
	g, snake := w.g, w.o.snake
	props := mapping()
	var required, wires []string
	for _, el := range cl.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		wire, lit := stringLit(kv.Key)
		if !lit {
			g.question("must", fmt.Sprintf("%s: %s has the key %s, which is not a literal. Which field is it, with its type?", clause, where, exprText(kv.Key)),
				[]string{at}, "A key held in a variable or computed is not known by syntax.", g.at(clause, where+": the key "+exprText(kv.Key)+"."))
			continue
		}
		if depth == 0 && dxlibEnvelopeKeys[wire] {
			continue
		}
		name, held := propertyName(wire, snake)
		if !held {
			g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: %s: the key %s is not camelCase, nor snake_case under info.wireNames; left out", clause, where, wire), clause: clause, blocks: []string{at + "/properties"}})
			continue
		}
		wires = append(wires, wire)
		pat := at + "/properties/" + name
		s := w.valueSchema(ep, f, kv.Value, pat, where+", property "+name, clause, depth+1)
		if s == nil {
			s = mapping()
		}
		set(props, name, s)
		required = append(required, name)
	}
	out := mapping("type", "object")
	if len(props.Content) > 0 {
		set(out, "properties", props)
	}
	if len(required) > 0 {
		set(out, "required", required)
	}
	return out, wires
}

// valueSchema is the schema a value in a literal map gives: a literal's
// own, a nested literal map's, a composite literal's type, or the type a
// name of the handler is declared with; nil with a question when the code
// does not say.
func (w *declaredWriter) valueSchema(ep *goEndpoint, f *goFile, e ast.Expr, at, where, clause string, depth int) *yaml.Node {
	g := w.g
	ask := func(why string) *yaml.Node {
		g.question("must", fmt.Sprintf("%s: %s is %s, %s. Which type does it take?", clause, where, exprText(e), why),
			[]string{at}, "A value's type is read from a literal or from a declaration the handler makes; a value a call returns is not known by syntax.", g.at(clause, where+": "+exprText(e)+"."))
		return nil
	}
	if depth > 8 {
		return ask("nested deeper than the reader follows")
	}
	typed := func(t ast.Expr, tf *goFile) *yaml.Node {
		s := g.fieldSchema(tf, t, at, where, w.o.snake, map[*ast.TypeSpec]bool{}, clause)
		g.markRequired(tf, t, s, w.o.snake, 0)
		return s
	}
	e = unparen(e)
	switch v := e.(type) {
	case *ast.BasicLit:
		switch v.Kind {
		case gotoken.STRING:
			return mapping("type", "string")
		case gotoken.INT:
			return typed(ast.NewIdent("int"), f)
		case gotoken.FLOAT:
			return mapping("type", "number", "format", "double")
		}
	case *ast.UnaryExpr:
		if v.Op == gotoken.AND {
			if cl, ok := v.X.(*ast.CompositeLit); ok {
				return w.valueSchema(ep, f, cl, at, where, clause, depth)
			}
		}
		if _, ok := numberLit(v); ok {
			return w.valueSchema(ep, f, v.X, at, where, clause, depth)
		}
	case *ast.CompositeLit:
		if g.isJSONLiteral(f, v) {
			s, _ := w.literalObject(ep, f, v, at, where, clause, depth)
			return s
		}
		if arr, ok := v.Type.(*ast.ArrayType); ok && arr.Len == nil && len(v.Elts) > 0 {
			if el, ok := v.Elts[0].(*ast.CompositeLit); ok && (el.Type == nil || g.isJSONLiteral(f, el)) && (g.isJSONMap(f, arr.Elt) || isOpenMap(arr.Elt)) {
				items, _ := w.literalObject(ep, f, el, at+"/items", where+", items", clause, depth)
				for _, other := range v.Elts[1:] {
					o, ok := other.(*ast.CompositeLit)
					if !ok {
						return ask("a list whose items are not all literal maps")
					}
					if _, k := w.literalObject(ep, f, o, at+"/items", where+", items", clause, depth); strings.Join(k, ",") != strings.Join(keysOfObject(items), ",") {
						return ask("a list whose items name other fields")
					}
				}
				return mapping("type", "array", "items", items)
			}
		}
		if v.Type != nil && !g.isJSONMap(f, v.Type) && !isOpenMap(v.Type) {
			return typed(v.Type, f)
		}
		return ask("a map whose entries are not literal")
	case *ast.Ident:
		switch v.Name {
		case "true", "false":
			return mapping("type", "boolean")
		case "nil":
			return ask("nil, which is JSON null and gives no type")
		}
		if t := ep.shapes.locals[v.Name]; t != nil {
			if g.isJSONMap(ep.shapes.file, t) || isOpenMap(t) {
				return ask("a map the handler fills elsewhere")
			}
			return typed(t, ep.shapes.file)
		}
	case *ast.SelectorExpr:
		if root, ok := v.X.(*ast.Ident); ok {
			if t := ep.shapes.locals[root.Name]; t != nil {
				if ft, tf := g.fieldType(ep.shapes.file, t, v.Sel.Name); ft != nil {
					return typed(ft, tf)
				}
			}
		}
	}
	return ask("which is not a literal, nor a name the handler declares with its type")
}

// keysOfObject is the wire names an object schema's properties were read
// from, in order, as literalObject gives them back.
func keysOfObject(s *yaml.Node) []string {
	return keys(child(s, "properties"))
}

func isOpenMap(e ast.Expr) bool {
	m, ok := e.(*ast.MapType)
	return ok && exprText(m.Key) == "string" && isOpenValue(m.Value)
}

// fieldType is the type of a struct's field by its Go name.
func (g *goReader) fieldType(f *goFile, typ ast.Expr, name string) (ast.Expr, *goFile) {
	st, tf := g.structFields(f, typ)
	if st == nil {
		return nil, nil
	}
	for _, fl := range st.Fields.List {
		for _, n := range fl.Names {
			if n.Name == name {
				return fl.Type, tf
			}
		}
	}
	return nil, nil
}

// handlerWires notes the wire names a handler's body and answers give, so
// that whether they are snake_case is decided with the declared ones.
func (g *goReader) handlerWires(ep *goEndpoint, note func(string)) {
	if ep.shapes == nil {
		return
	}
	if d := ep.shapes.decode; d != nil && d.typ != nil {
		g.structWires(d.file, d.typ, note, map[*ast.TypeSpec]bool{})
	}
	for _, a := range ep.shapes.answers {
		ast.Inspect(a.call.Args[2], func(n ast.Node) bool {
			if kv, ok := n.(*ast.KeyValueExpr); ok {
				if s, lit := stringLit(kv.Key); lit {
					note(s)
				}
			}
			return true
		})
	}
}

// handlerCites cites where the handler decodes its body and each status it
// answers with WriteResponseAsJSON.
func (g *goReader) handlerCites(ep *goEndpoint, label string) []*yaml.Node {
	if ep.shapes == nil {
		return nil
	}
	var out []*yaml.Node
	if d := ep.shapes.decode; d != nil {
		says := fmt.Sprintf("The handler of %s decodes its body from %s into %s", label, d.from, d.text)
		if d.typ != nil {
			says += ", of type " + typeText(d.typ)
		}
		out = append(out, g.at(d.clause, says+guardsText(d)+"."))
	}
	for _, a := range ep.shapes.answers {
		out = append(out, g.at(a.clause, fmt.Sprintf("The handler of %s answers with WriteResponseAsJSON(%s, %s, %s).", label, exprText(a.call.Args[0]), shortText(a.call.Args[1]), shortText(a.call.Args[2]))))
	}
	return out
}

// shortText is an expression as the code writes it, a literal map by its
// keys.
func shortText(e ast.Expr) string {
	cl, ok := unparen(e).(*ast.CompositeLit)
	if !ok {
		return exprText(e)
	}
	var ks []string
	for _, el := range cl.Elts {
		if kv, ok := el.(*ast.KeyValueExpr); ok {
			ks = append(ks, exprText(kv.Key))
		}
	}
	return typeText(cl.Type) + "{" + strings.Join(ks, ", ") + "}"
}

// topWires is the wire names of a struct's own fields, false when the type
// is not a struct of the module.
func (g *goReader) topWires(f *goFile, typ ast.Expr) ([]string, bool) {
	st, _ := g.structFields(f, typ)
	if st == nil {
		return nil, false
	}
	var out []string
	for _, fl := range st.Fields.List {
		for _, n := range fl.Names {
			wire, skip, _ := jsonTag(fl, n.Name)
			if !skip && n.IsExported() {
				out = append(out, wire)
			}
		}
	}
	return out, true
}
