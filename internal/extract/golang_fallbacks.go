package extract

import (
	"fmt"
	"go/ast"
	gotoken "go/token"
	"strings"
)

// A fallback handler and a catch-all route (ADR-143). gin's NoRoute and
// NoMethod, chi's NotFound and MethodNotAllowed, gorilla/mux's
// NotFoundHandler and MethodNotAllowedHandler, echo's HTTPErrorHandler
// and a ServeMux pattern of / alone with no method are where a gateway
// lives in each library, and where a plain 404 lives too; a route whose
// path ends in a catch-all, or gorilla/mux's PathPrefix given a handler
// that is no router, is the other way the same thing is written. The
// reader cannot tell which, so each is a must question blocking the
// design stage, counted as a registration, in place of silence or a line.

// fallbackWhy is the why of every question at a fallback handler.
const fallbackWhy = "A handler that takes every request no route answers is where a gateway lives, and where a plain 404 lives too; syntax cannot tell which, and a registration the reader knows is written or asked about, never dropped."

// catchAllWhy is the why of every question at a catch-all route.
const catchAllWhy = "A route whose path ends in a catch-all takes every request under its prefix, as a fallback handler takes every request no route answers; a path parameter holds one segment, so no operation is written, and syntax cannot tell a gateway from a file server."

// fallbackStage is what a fallback question blocks: the design stage,
// until the meta-model has a section for a gateway.
var fallbackStage = []string{"design"}

// fallbacks is what each library's fallback registration takes, by the
// method or field that sets it: the requests it is given.
var fallbacks = map[string]map[string]string{
	"gin":         {"NoRoute": "every request no route answers", "NoMethod": "every request whose path a route answers but not its method"},
	"chi":         {"NotFound": "every request no route answers", "MethodNotAllowed": "every request whose path a route answers but not its method"},
	"gorilla/mux": {"NotFoundHandler": "every request no route answers", "MethodNotAllowedHandler": "every request whose path a route answers but not its method"},
	"echo":        {"HTTPErrorHandler": "every error, a request no route answers among them"},
}

// fallbackField says whether a library sets its fallback as a field, by
// an assignment, rather than by a call.
func fallbackField(lib string) bool {
	return lib == "gorilla/mux" || lib == "echo"
}

// fallbackCall reads a call that sets a fallback handler on a router,
// and says whether it was one.
func (sc *routeScope) fallbackCall(recv *routerNode, call *ast.CallExpr, name string) bool {
	takes := fallbacks[recv.lib][name]
	if takes == "" || fallbackField(recv.lib) || len(call.Args) == 0 {
		return false
	}
	if !sc.countOnce(recv, call) {
		return true
	}
	sc.g.citeFile(sc.file)
	clause := sc.g.clause(sc.file, call.Pos())
	if sc.branched(clause) {
		return true
	}
	sc.askFallback(clause, name, "registers", sc.handlerText(call.Args[len(call.Args)-1]), takes)
	return true
}

// handlerText names a handler as the code writes it, net/http's
// HandlerFunc taken off.
func (sc *routeScope) handlerText(h ast.Expr) string {
	h, _ = sc.unwrap(h)
	return exprText(h)
}

// fallbackAssign reads an assignment to a field of a router that sets
// its fallback handler, the router held in a name or in a field of a
// struct of the module as eval reads one, and says whether it was one.
func (sc *routeScope) fallbackAssign(n *ast.AssignStmt) bool {
	if len(n.Lhs) != 1 || len(n.Rhs) != 1 || n.Tok != gotoken.ASSIGN {
		return false
	}
	sel, ok := n.Lhs[0].(*ast.SelectorExpr)
	if !ok {
		return false
	}
	recv := sc.eval(sel.X)
	if recv == nil || !fallbackField(recv.lib) {
		return false
	}
	takes := fallbacks[recv.lib][sel.Sel.Name]
	if takes == "" {
		return false
	}
	clause := sc.g.clause(sc.file, n.Pos())
	if !sc.countOnceAt(recv, n.Pos(), sel.Sel.Name+"="+exprText(n.Rhs[0])) {
		return true
	}
	sc.g.citeFile(sc.file)
	if sc.branched(clause) {
		return true
	}
	sc.askFallback(clause, sel.Sel.Name, "sets", sc.handlerText(n.Rhs[0]), takes)
	return true
}

// askFallback asks what a fallback handler answers or forwards.
func (sc *routeScope) askFallback(clause, by, how, handler, takes string) {
	g := sc.g
	g.question("must", fmt.Sprintf("%s %s %s with %s as the handler of %s. What does it answer or forward, and by which claim of the token does it pick the service?", clause, how, handler, by, takes),
		fallbackStage, fallbackWhy, g.at(clause, fmt.Sprintf("%s %s with %s.", strings.ToUpper(how[:1])+how[1:], handler, by)))
}

// askRootPattern asks about a ServeMux pattern of / alone with no
// method, which matches every request no other pattern answers.
func (sc *routeScope) askRootPattern(clause string, handler ast.Expr) {
	g := sc.g
	h := sc.handlerText(handler)
	g.question("must", fmt.Sprintf("%s registers %s for the pattern / with no method, which matches every request no other pattern answers. What does it answer or forward, and by which claim of the token does it pick the service?", clause, h),
		fallbackStage, fallbackWhy, g.at(clause, "Registers "+h+" for the pattern /."))
}

// catchAllSegment is the prefix before, and the segment that is, a
// catch-all in a route's path as each library writes one: gin's and
// echo's *name, a ServeMux {name...}, chi's * at the end; ok is false
// for a path with none. gorilla/mux has no catch-all segment: its
// catch-all is PathPrefix given a handler, which gorilla reads.
func catchAllSegment(lib, p string) (prefix, segment string, ok bool) {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		var hit bool
		switch lib {
		case "gin", "echo":
			hit = strings.HasPrefix(s, "*")
		case "net/http":
			hit = strings.HasSuffix(s, "...}")
		case "chi":
			hit = s == "*" || strings.HasSuffix(s, "*")
		}
		if hit {
			return strings.Join(segs[:i], "/"), s, true
		}
	}
	return "", "", false
}

// catchAll asks about a route whose path ends in a catch-all, naming
// its prefix under the router's place and its methods, and says whether
// the path holds one.
func (sc *routeScope) catchAll(recv *routerNode, clause, p string, methods []string) bool {
	pre, seg, ok := catchAllSegment(recv.lib, p)
	if !ok {
		return false
	}
	sc.askCatchAll(recv, clause, pre, "the rest of the path matched by "+seg, methods, p)
	return true
}

// askCatchAll asks what the handler of a catch-all answers or forwards:
// a route under the prefix pre of the router's place whose rest the
// catch-all `by` matches, registered for the methods, every method when
// none is named; `p` is the path or prefix as the code writes it.
func (sc *routeScope) askCatchAll(recv *routerNode, clause, pre, by string, methods []string, p string) {
	g := sc.g
	under := pre
	if place, unknown := recv.place(); unknown == "" {
		under = joinPath(place, pre)
	}
	if under == "" {
		under = "/"
	}
	what := "every method"
	if len(methods) > 0 {
		what = strings.Join(methods, ", ")
	}
	g.question("must", fmt.Sprintf("%s registers %s under %s with %s, a catch-all that takes every request under the prefix. What does its handler answer or forward, and by which claim of the token does it pick the service?", clause, what, under, by),
		fallbackStage, catchAllWhy, g.at(clause, fmt.Sprintf("Registers %s %s.", what, p)))
}
