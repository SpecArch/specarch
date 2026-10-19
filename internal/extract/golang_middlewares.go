package extract

import (
	"fmt"
	"go/ast"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The middleware chain an endpoint is registered with (ADR-131): each
// middleware of the chains is declared once under middlewares, by the
// kebab-case name of its function, with what its doc comment says it does
// and the refusals its body answers; each operation lists the ones it runs,
// in order.

// goMiddleware is one middleware the chains name.
type goMiddleware struct {
	name     string // kebab-case
	text     string // as the first chain that names it writes it
	firstAt  string // that registration's path:line
	others   []string
	fd       *ast.FuncDecl
	file     *goFile
	clause   string // the function's path:line, "" when it is not found
	doc      string
	refusals []goProblem
	unread   []string        // refusals the reader could not read, as questions say them
	unreadAt map[string]bool // the refusals whose reason is not a literal, by clause
}

// middlewareName is the name a middleware is declared by: its function's
// name in kebab-case, a factory's for a middleware a call makes.
func middlewareName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return kebab(x.Name)
	case *ast.SelectorExpr:
		return kebab(x.Sel.Name)
	case *ast.CallExpr:
		return middlewareName(x.Fun)
	case *ast.ParenExpr:
		return middlewareName(x.X)
	case *ast.IndexExpr:
		return middlewareName(x.X)
	}
	return ""
}

// chainList is the middlewares of a chain: a literal list, nil, or one a
// name, a pointer or a function of the module gives as literalList follows
// it, or a call to a function of the module whose one statement returns its
// variadic parameter, such as mw(a, b).
func (g *goReader) chainList(f *goFile, e ast.Expr) ([]ast.Expr, *goFile, string) {
	if call, ok := e.(*ast.CallExpr); ok {
		if fd, _ := g.funcOf(f, call.Fun); fd != nil && fd.Body != nil && len(fd.Body.List) == 1 && fd.Type.Params != nil {
			if ret, ok := fd.Body.List[0].(*ast.ReturnStmt); ok && len(ret.Results) == 1 {
				params := fd.Type.Params.List
				if id, ok := ret.Results[0].(*ast.Ident); ok && len(params) > 0 {
					last := params[len(params)-1]
					if _, variadic := last.Type.(*ast.Ellipsis); variadic && len(last.Names) == 1 && last.Names[0].Name == id.Name && len(params) == 1 && call.Ellipsis == 0 {
						return call.Args, f, ""
					}
				}
			}
		}
	}
	return g.literalList(f, e, 0)
}

// readMiddlewares declares every middleware the endpoints' chains name, in
// the order of their names, and reads each one's doc comment and refusals.
func (g *goReader) readMiddlewares() {
	g.middlewareOf = map[string]*goMiddleware{}
	for _, ep := range g.endpoints {
		if !ep.chainKnown {
			continue
		}
		for _, m := range ep.chainExprs {
			name := middlewareName(m)
			text := exprText(m)
			if name == "" || !problemWord.MatchString(name) {
				continue
			}
			mw := g.middlewareOf[name]
			if mw == nil {
				mw = &goMiddleware{name: name, text: text, firstAt: fmt.Sprintf("%s:%d", ep.file.path, ep.line)}
				g.middlewareOf[name] = mw
				g.middlewareNames = append(g.middlewareNames, name)
				fd, file := g.funcOf(ep.chainFile, m)
				if _, call := m.(*ast.CallExpr); !call && fd != nil && fd.Body != nil {
					mw.fd, mw.file, mw.clause = fd, file, g.clause(file, fd.Pos())
					file.cited = true
					if fd.Doc != nil {
						mw.doc = strings.Join(strings.Fields(fd.Doc.Text()), " ")
					}
					g.refusalsIn(mw)
				}
				continue
			}
			if text != mw.text && !contains(mw.others, text) {
				mw.others = append(mw.others, text)
			}
		}
	}
	sort.Strings(g.middlewareNames)
}

// refusalsIn reads the refusals a middleware's body answers through its
// request, as a handler's are read.
func (g *goReader) refusalsIn(mw *goMiddleware) {
	ft := mw.fd.Type
	if ft.Params == nil || len(ft.Params.List) == 0 || len(ft.Params.List[0].Names) == 0 {
		return
	}
	request := ft.Params.List[0].Names[0].Name
	ast.Inspect(mw.fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !problemCalls[sel.Sel.Name] {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); !ok || x.Name != request {
			return true
		}
		clause := g.clause(mw.file, call.Pos())
		status, ok := g.statusValue(mw.file, firstArg(call))
		if !ok || status < 400 || status > 599 {
			mw.unread = append(mw.unread, fmt.Sprintf("%s answers with %s the status %s, which is not a literal refusal status", clause, sel.Sel.Name, exprText(firstArg(call))))
			return true
		}
		reason := ""
		if sel.Sel.Name != "WriteResponseAsError" && len(call.Args) > 1 {
			r, lit := g.stringValue(mw.file, call.Args[1])
			if !lit {
				mw.unread = append(mw.unread, fmt.Sprintf("%s answers %d with the reason %s, which is not a literal", clause, status, exprText(call.Args[1])))
				if mw.unreadAt == nil {
					mw.unreadAt = map[string]bool{}
				}
				mw.unreadAt[clause] = true
			}
			reason = r
		}
		mw.refusals = append(mw.refusals, goProblem{clause: clause, status: status, reason: reason})
		return true
	})
}

// chainNames is the names of an endpoint's middlewares, in order.
func chainNames(ep *goEndpoint) []string {
	names := []string{}
	for _, m := range ep.chainExprs {
		if n := middlewareName(m); n != "" && problemWord.MatchString(n) {
			names = append(names, n)
		}
	}
	return names
}

// writeChain writes the middlewares an operation runs, or asks for them.
func (g *goReader) writeChain(ep *goEndpoint, op *yaml.Node, regAt, label string) {
	if !ep.chainKnown {
		g.question("must", fmt.Sprintf("%s registers %s with the middlewares %s, which %s, so the steps a request passes before its handler are not read. Which middlewares does it run, in order?", regAt, label, ep.chainText, ep.chainWhy),
			[]string{ep.at + "/middlewares"}, "The middlewares decide whether a caller must sign in, which permission is checked and what a refused request is answered; syntax reads only a chain it can follow to a literal.", g.at(regAt, "Names its middlewares with "+ep.chainText+"."))
		return
	}
	names := chainNames(ep)
	if len(names) != len(ep.chainExprs) {
		var odd []string
		for _, m := range ep.chainExprs {
			if n := middlewareName(m); n == "" || !problemWord.MatchString(n) {
				odd = append(odd, exprText(m))
			}
		}
		g.question("must", fmt.Sprintf("%s registers %s with the middlewares %s, of which %s %s no function the reader can name. Which middlewares does it run, in order?", regAt, label, strings.Join(ep.chain, ", "), joinAnd(odd), doOrDo(len(odd), "names", "name")),
			[]string{ep.at + "/middlewares"}, "A middleware is declared by the name of its function, and an expression that is not one gives none.", g.at(regAt, "Names its middlewares with "+strings.Join(ep.chain, ", ")+"."))
	}
	set(op, "middlewares", flow(value(names)))
}

func doOrDo(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// writeMiddlewares declares every middleware the chains name, and asks for
// what the source does not say of them.
func (g *goReader) writeMiddlewares(problemStatus map[string]map[int]bool, conflicts map[string][]string) {
	if len(g.middlewareNames) == 0 {
		return
	}
	g.middlewares = &yaml.Node{Kind: yaml.MappingNode}
	var undescribed []string
	for _, name := range g.middlewareNames {
		mw := g.middlewareOf[name]
		at := "#/middlewares/" + name
		m := mapping()
		if mw.doc != "" {
			set(m, "description", mw.doc)
		}
		var refusals []*yaml.Node
		for i, r := range mw.refusals {
			n := mapping("status", r.status)
			problem := problemName(r.reason)
			switch {
			case problem != "" && len(problemStatus[problem]) == 1:
				set(n, "problem", problem)
			case problem != "":
				conflicts[problem] = append(conflicts[problem], fmt.Sprintf("%s/refusals/%d/problem", at, i))
			case r.reason == "" && mw.unreadAt[r.clause]:
				// Asked below with what was not read.
			case r.reason == "":
				g.question("must", fmt.Sprintf("%s: the middleware %s answers %d with no reason of its own. Which problem does it answer?", r.clause, name, r.status),
					[]string{fmt.Sprintf("%s/refusals/%d/problem", at, i)}, "A refusal with no reason of its own names no problem type, and a refusal names one once the specification declares them.", g.at(r.clause, fmt.Sprintf("Answers %d with no reason.", r.status)))
			default:
				g.question("must", fmt.Sprintf("%s: the middleware %s answers %d with the reason %s, which gives no problem name in kebab-case. Which problem does it answer?", r.clause, name, r.status, r.reason),
					[]string{fmt.Sprintf("%s/refusals/%d/problem", at, i)}, "A problem type is named in lower-case words joined by hyphens, and the reason does not give one.", g.at(r.clause, fmt.Sprintf("Answers %d with the reason %s.", r.status, r.reason)))
			}
			refusals = append(refusals, flow(n))
		}
		if len(refusals) > 0 {
			set(m, "refusals", refusals)
		}
		set(m, "origin", "stated")
		var cites []*yaml.Node
		if mw.clause != "" {
			says := fmt.Sprintf("%s is a middleware; it answers no refusal through its request.", mw.text)
			if len(mw.unread) > 0 {
				says = fmt.Sprintf("%s is a middleware; it answers a refusal the reader cannot read.", mw.text)
			}
			if len(mw.refusals) > 0 {
				var answers []string
				for _, r := range mw.refusals {
					a := strconv.Itoa(r.status)
					if r.reason != "" {
						a += " " + r.reason
					}
					if !contains(answers, a) {
						answers = append(answers, a)
					}
				}
				says = fmt.Sprintf("%s is a middleware; it answers %s.", mw.text, joinAnd(answers))
			}
			cites = append(cites, g.at(mw.clause, says))
		}
		cites = append(cites, g.at(mw.firstAt, fmt.Sprintf("Runs %s before the handler.", mw.text)))
		set(m, "cites", cites)
		set(g.middlewares, name, m)
		switch {
		case mw.clause == "":
			g.question("must", fmt.Sprintf("The middleware %s, which %s runs, is not a function the reader finds by syntax in the files read, such as a function outside the module or one a call makes, so what it checks and what it refuses with are not read. What does it check or do, and what does it refuse with?", mw.text, mw.firstAt),
				[]string{at + "/description", at + "/refusals"}, "Syntax alone finds a function declared in the module and named directly, and no other.", g.at(mw.firstAt, "Runs "+mw.text+"."))
		case mw.doc == "":
			undescribed = append(undescribed, name)
		}
		for _, u := range mw.unread {
			g.question("should", fmt.Sprintf("The middleware %s: %s. Which status and problem does it answer, and when?", name, u),
				[]string{at + "/refusals"}, "Only a literal status and a literal reason say which refusal the code answers.", g.at(mw.clause, "Answers a refusal the reader cannot read."))
		}
		if len(mw.others) > 0 {
			g.question("must", fmt.Sprintf("The chains name %s and %s, which give the one middleware name %s. Are they one middleware, or which name does each take?", mw.text, joinAnd(mw.others), name),
				[]string{at}, "A middleware is declared by its function's name, and two functions of one name in different packages may do different things.", g.at(mw.firstAt, "Runs "+mw.text+"."))
		}
	}
	if len(undescribed) > 0 {
		var blocks []string
		for _, n := range undescribed {
			blocks = append(blocks, "#/middlewares/"+n+"/description")
		}
		g.question("must", fmt.Sprintf("What does each middleware check or do: %s? Its function has no doc comment.", strings.Join(undescribed, ", ")),
			blocks, "A middleware's description says what it checks or does, and a function without a doc comment does not say it.")
	}
}

// problemName is the problem a literal reason names, "" when it names
// none.
func problemName(reason string) string {
	name := strings.ToLower(strings.NewReplacer("_", "-", " ", "-").Replace(reason))
	if reason == "" || !problemWord.MatchString(name) {
		return ""
	}
	return name
}
