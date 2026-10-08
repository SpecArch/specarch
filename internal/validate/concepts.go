package validate

import (
	"fmt"
	"sort"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/source"
)

// The concepts the waiting red paths of docs/test-generation.md need:
// dependencies with a time limit, an idempotency key, validity on an
// entity, a session, and a guard on a data change.

// idempotentMethods are the methods RFC 9110 (9.2.2) makes idempotent by
// themselves, so an idempotency key on one says nothing.
var idempotentMethods = map[string]bool{"get": true, "put": true, "delete": true}

// checkDependencies refuses a time limit of zero: a call given up at once
// is no call.
func (c *checker) checkDependencies(d *design) {
	for name, dep := range d.dependencies {
		c.checkTimeout(source.Child(dep, "timeout"), source.Pointer("dependencies", name, "timeout"), RuleDependency, "the call")
	}
}

// checkSession refuses a session timeout of zero.
func (c *checker) checkSession(d *design) {
	s := source.Child(d.root, "session")
	for _, key := range []string{"idleTimeout", "absoluteTimeout"} {
		c.checkTimeout(source.Child(s, key), source.Pointer("session", key), RuleSession, "the session")
	}
}

func (c *checker) checkTimeout(n *yaml.Node, ptr string, rule Rule, what string) {
	if n == nil {
		return
	}
	if dur, ok := expr.ParseDuration(n.Value); ok && dur == 0 {
		c.add(n, ptr, rule, "%s is no time at all; give %s a limit above zero", n.Value, what)
	}
}

// checkCalls checks that every dependency an operation calls is declared.
func (c *checker) checkCalls(d *design, o operation) {
	for i, e := range source.Items(source.Child(o.node, "calls")) {
		if d.dependencies[e.Value] == nil {
			c.add(e, o.pointer("calls", fmt.Sprint(i)), RuleDependency, "%s is not a dependency of the specification; declare it under dependencies with its timeout%s", e.Value, suggest(e.Value, d.dependencies))
		}
	}
}

// checkIdempotencyKey checks that the key names a header parameter of the
// operation and that the method is one a repeat can change.
func (c *checker) checkIdempotencyKey(o operation) {
	n := source.Child(o.node, "idempotencyKey")
	if n == nil {
		return
	}
	ptr := o.pointer("idempotencyKey")
	if idempotentMethods[o.method] {
		c.add(n, ptr, RuleIdempotencyKey, "%s is idempotent by itself (RFC 9110, 9.2.2), so idempotencyKey says nothing here; remove it", o.method)
		return
	}
	if headerParameter(o, n.Value) == nil {
		c.add(n, ptr, RuleIdempotencyKey, "%s is not a header parameter of %s; declare it under parameters with in: header, or name one that is", n.Value, o.id)
	}
}

// headerParameter is the operation's header parameter of that name, or nil.
func headerParameter(o operation, name string) *yaml.Node {
	for _, p := range append(source.Items(source.Child(o.pathItem, "parameters")), source.Items(source.Child(o.node, "parameters"))...) {
		if source.Str(source.Child(p, "in")) == "header" && source.Str(source.Child(p, "name")) == name {
			return p
		}
	}
	return nil
}

// checkValidity checks that an entity's validity names its own date or
// date-time fields, both of one format.
func (c *checker) checkValidity(name string, e *yaml.Node, fields map[string]*yaml.Node) {
	v := source.Child(e, "validity")
	if v == nil {
		return
	}
	formats := map[string]string{}
	for _, key := range []string{"from", "until"} {
		n := source.Child(v, key)
		if n == nil {
			continue
		}
		ptr := source.Pointer("entities", name, "validity", key)
		f := fields[n.Value]
		if f == nil {
			c.add(n, ptr, RuleValidity, "%s is not a field of %s, so it cannot say when a record is valid%s", n.Value, name, suggest(n.Value, fields))
			continue
		}
		format := source.Str(source.Child(f, "format"))
		if format != "date" && format != "date-time" {
			if format == "" {
				format = "no format"
			}
			c.add(n, ptr, RuleValidity, "%s has %s, but a validity field must be a date or a date-time", n.Value, format)
			continue
		}
		formats[key] = format
	}
	if formats["from"] != "" && formats["until"] != "" && formats["from"] != formats["until"] {
		c.add(source.Child(v, "from"), source.Pointer("entities", name, "validity", "from"), RuleValidity,
			"from %s is a %s and until %s is a %s; give both the same format", source.Str(source.Child(v, "from")), formats["from"], source.Str(source.Child(v, "until")), formats["until"])
	}
}

// checkGuard checks that a guard's entity exists. Its precondition is
// checked with the expressions.
func (c *checker) checkGuard(d *design, g *yaml.Node, ptr string) {
	if g == nil {
		return
	}
	n := source.Child(g, "entity")
	if n != nil && d.entities[n.Value] == nil {
		c.add(n, ptr+"/entity", RuleGuard, "%s is not an entity of the specification%s", n.Value, suggest(n.Value, d.entities))
	}
}

// guards lists every guard of the specification with its pointer, in a
// fixed order: operations in document order, then commands by name.
func (d *design) guards() []guard {
	var out []guard
	for _, o := range d.opList {
		if g := source.Child(o.node, "guard"); g != nil {
			out = append(out, guard{node: g, ptr: o.pointer("guard")})
		}
	}
	names := make([]string, 0, len(d.commands))
	for name := range d.commands {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if g := source.Child(d.commands[name], "guard"); g != nil {
			out = append(out, guard{node: g, ptr: source.Pointer("commands", name, "guard")})
		}
	}
	return out
}

type guard struct {
	node *yaml.Node
	ptr  string
}
