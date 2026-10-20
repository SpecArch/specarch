package validate

import (
	"fmt"
	"slices"
	"sort"
	"strings"

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

// checkDependencies refuses a time limit of zero, since a call given up at
// once is no call, and a specification on a store or a broker, which
// another party runs and which is no system specified on its own
// (ADR-134).
func (c *checker) checkDependencies(d *design) {
	for name, dep := range d.dependencies {
		c.checkTimeout(source.Child(dep, "timeout"), source.Pointer("dependencies", name, "timeout"), RuleDependency, "the call")
		switch kind := source.Str(source.Child(dep, "kind")); kind {
		case "database", "messageBroker", "fileStore":
			if sp := source.Child(dep, "specification"); sp != nil {
				c.add(sp, source.Pointer("dependencies", name, "specification"), RuleDependency,
					"a dependency of kind %s is a store or a broker, not a system specified on its own, so no specification describes it; remove specification and version, or give it kind system", kind)
			}
		}
	}
}

// checkChannels checks that every dependency a channel is carried by is
// declared and of a kind that carries messages: a broker or a system
// (ADR-134).
func (c *checker) checkChannels(d *design) {
	for name, ch := range d.channels {
		for i, e := range source.Items(source.Child(ch, "carriedBy")) {
			ptr := source.Pointer("channels", name, "carriedBy", fmt.Sprint(i))
			dep, ok := d.dependencies[e.Value]
			if !ok {
				c.add(e, ptr, RuleChannel, "%s is not a dependency of the specification; declare it under dependencies, or leave carriedBy out when the channel is this system's own%s", e.Value, suggest(e.Value, d.dependencies))
				continue
			}
			switch kind := source.Str(source.Child(dep, "kind")); kind {
			case "database", "fileStore", "identityProvider":
				c.add(e, ptr, RuleChannel, "%s is a dependency of kind %s, which carries no messages; name the broker the channel lives on, or the system that receives its messages", e.Value, kind)
			}
		}
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
		if dep := callDependency(e.Value); d.dependencies[dep] == nil {
			c.add(e, o.pointer("calls", fmt.Sprint(i)), RuleDependency, "%s is not a dependency of the specification; declare it under dependencies with its timeout%s", dep, suggest(dep, d.dependencies))
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

// checkGuard checks that a guard's entity exists, and that what its writes
// says is a change no keyword of the entry already says, of fields the
// entity has other than its key and its state field (ADR-168). Its
// precondition and the values writes sets are checked with the
// expressions. trigger is the operationId or the command's words; method,
// path and body (its request body's $ref) are the operation's, or "" for a
// command.
func (c *checker) checkGuard(d *design, g *yaml.Node, ptr, trigger, method, path, body string) {
	if g == nil {
		return
	}
	n := source.Child(g, "entity")
	if n != nil && d.entities[n.Value] == nil {
		c.add(n, ptr+"/entity", RuleGuard, "%s is not an entity of the specification%s", n.Value, suggest(n.Value, d.entities))
	}
	w := source.Child(g, "writes")
	ent := d.entities[source.Str(n)]
	if w == nil || ent == nil {
		return
	}
	name := n.Value
	keyed := strings.Count(path, "{") == 1
	switch {
	case method == "put" && keyed && body == "#/entities/"+name:
		c.add(w, ptr+"/writes", RuleGuard, "a PUT of %s by its key already says what it writes, every field a caller may set; take writes out", name)
		return
	case method == "delete" && keyed:
		c.add(w, ptr+"/writes", RuleGuard, "a DELETE by its key already says what it writes, that the %s goes; take writes out", name)
		return
	}
	if del := source.Child(w, "delete"); del != nil {
		for _, t := range source.Items(source.Child(ent, "transitions")) {
			if source.Str(source.Child(t, "trigger")) == trigger {
				c.add(del, ptr+"/writes/delete", RuleGuard, "%s triggers a transition of %s to %s, and writes says it deletes the records; say one change: take delete out, or the transition", trigger, name, source.Str(source.Child(t, "to")))
				break
			}
		}
	}
	fields := fieldsOf(ent)
	state := source.Str(source.Child(ent, "stateField"))
	for _, p := range source.Pairs(source.Child(w, "set")) {
		f := p.Key.Value
		at := ptr + "/writes/set/" + source.EscapeToken(f)
		switch {
		case fields[f] == nil:
			c.add(p.Key, at, RuleGuard, "writes sets %s, which is not a field of %s%s", f, name, suggest(f, fields))
		case slices.Contains(itemValues(source.Child(ent, "primaryKey")), f):
			c.add(p.Key, at, RuleGuard, "writes sets %s, part of the key of %s; the key names the records the change writes, so it is not set", f, name)
		case f == state:
			c.add(p.Key, at, RuleGuard, "writes sets %s, the state field of %s; a state is moved by a transition whose trigger is this entry, so name it in transitions instead", f, name)
		}
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
