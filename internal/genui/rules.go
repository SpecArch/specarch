package genui

import (
	"strconv"
	"strings"

	"github.com/SpecArch/specarch/internal/expr"
)

// A page's expressions, an action's when, a field's condition and a
// check across fields, are written as JavaScript functions of the record.
// The browser evaluates the operators whose meaning is the same there:
// comparisons, &&, ||, ! and size. Anything else is an error at the
// expression, and the element it belongs to is left out.

// rule writes an expression as a function of the record, or reports why
// the browser cannot evaluate it.
func (g *gen) rule(expression, at, what string, props map[string]any) (string, bool) {
	node, errs := expr.Parse(expression)
	if len(errs) > 0 {
		g.problem("error", at, "%s does not parse: %s", what, errs[0].Message)
		return "", false
	}
	if f := ordersDecimal(node, props); f != "" {
		g.problem("error", at, "%s orders %s, a decimal, which the browser cannot compare exactly; check it on the server", what, f)
		return "", false
	}
	body, ok := g.js(node, at, what)
	if !ok {
		return "", false
	}
	return "(record) => " + body, true
}

// js writes one node of an expression.
func (g *gen) js(n *expr.Node, at, what string) (string, bool) {
	sub := func(i int) (string, bool) { return g.js(n.Args[i], at, what) }
	switch n.Op {
	case expr.OpName:
		return "record." + n.Text, true
	case expr.OpInt, expr.OpUint:
		return n.Int.String(), true
	case expr.OpDouble:
		return strconv.FormatFloat(n.Double, 'g', -1, 64), true
	case expr.OpText:
		return jsString(n.Text), true
	case expr.OpBool:
		return strconv.FormatBool(n.Bool), true
	case expr.OpNull:
		return "null", true
	case expr.OpNot:
		a, ok := sub(0)
		return "!(" + a + ")", ok
	case "==", "!=":
		a, ok1 := sub(0)
		b, ok2 := sub(1)
		// A field the record leaves out is null, as the specification reads it.
		if n.Args[0].Op == expr.OpName && n.Args[1].Op == expr.OpNull || n.Args[1].Op == expr.OpName && n.Args[0].Op == expr.OpNull {
			a, b = nullable(n.Args[0], a), nullable(n.Args[1], b)
		}
		return a + " " + n.Op + "= " + b, ok1 && ok2
	case "<", "<=", ">", ">=":
		a, ok1 := sub(0)
		b, ok2 := sub(1)
		return a + " " + n.Op + " " + b, ok1 && ok2
	case "&&", "||":
		a, ok1 := sub(0)
		b, ok2 := sub(1)
		return group(n.Args[0], n.Op, a) + " " + n.Op + " " + group(n.Args[1], n.Op, b), ok1 && ok2
	case expr.OpCall:
		if n.Text == "size" && len(n.Args) == 1 {
			// size counts a text's characters, as [...text] does, and a list's items.
			a, ok := sub(0)
			return "[...(" + a + " ?? [])].length", ok
		}
		g.problem("error", at, "%s calls %s, and specarch-gen-ui evaluates comparisons, &&, ||, ! and size in the browser only; check it on the server, or rewrite it with those", what, n.Text)
		return "", false
	}
	g.problem("error", at, "%s uses %s, and specarch-gen-ui evaluates comparisons, &&, ||, ! and size in the browser only; check it on the server, or rewrite it with those", what, n.Op)
	return "", false
}

// nullable writes a field compared with null so a field left out counts.
func nullable(n *expr.Node, s string) string {
	if n.Op == expr.OpName {
		return "(" + s + " ?? null)"
	}
	return s
}

// group puts an operand of && or || in parentheses when it is the other
// one, so the reader need not know which binds tighter.
func group(n *expr.Node, op, s string) string {
	if (n.Op == "&&" || n.Op == "||") && n.Op != op {
		return "(" + s + ")"
	}
	return s
}

// ordersDecimal names a field of format decimal that an expression orders
// with <, <=, > or >=, which the browser cannot do exactly; "" when none.
func ordersDecimal(n *expr.Node, props map[string]any) string {
	switch n.Op {
	case "<", "<=", ">", ">=":
		for _, a := range n.Args {
			if a.Op == expr.OpName && text(obj0(props[a.Text])["format"]) == "decimal" {
				return a.Text
			}
		}
	}
	for _, a := range n.Args {
		if found := ordersDecimal(a, props); found != "" {
			return found
		}
	}
	return ""
}

// routeParameters are the names of a route's parameters, in order.
func routeParameters(route string) []string {
	var out []string
	for _, p := range pathParam.FindAllString(route, -1) {
		out = append(out, strings.Trim(p, "{}"))
	}
	return out
}
