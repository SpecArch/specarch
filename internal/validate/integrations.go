package validate

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/source"
)

// The integration contract (ADR-200): how this system proves itself to a
// dependency and where the secret it sends is kept, how often it may call,
// the operations of a dependency with no specification of its own, and on
// each call how our values map to theirs and back, the idempotency key
// sent, the retries, which of their answers become ours and whether the
// call waits for the answer.

// checkIntegrations checks each dependency's contract and each call's.
func (c *checker) checkIntegrations(d *design) {
	for _, p := range source.Pairs(source.Child(d.root, "dependencies")) {
		c.checkDependencyContract(d, p.Key.Value, p.Value)
	}
	for _, o := range d.opList {
		for _, e := range callEntries(o.node) {
			c.checkCall(d, o, e)
		}
	}
	for _, j := range source.Pairs(source.Child(d.root, "jobs")) {
		for _, e := range callEntries(j.Value) {
			c.checkCallReference(d, e, source.Pointer(append([]string{"jobs", j.Key.Value}, e.at...)...))
		}
	}
	for _, g := range source.Pairs(source.Child(d.root, "gateways")) {
		for _, e := range callEntries(g.Value) {
			c.checkCallReference(d, e, source.Pointer(append([]string{"gateways", g.Key.Value}, e.at...)...))
		}
	}
}

// checkDependencyContract checks a dependency's auth and credential, and
// its operations.
func (c *checker) checkDependencyContract(d *design, name string, dep *yaml.Node) {
	at := func(tokens ...string) string {
		return source.Pointer(append([]string{"dependencies", name}, tokens...)...)
	}
	auth := source.Str(source.Child(dep, "auth"))
	cred := source.Child(dep, "credential")
	switch {
	case auth != "" && auth != "none" && cred == nil:
		c.add(source.Child(dep, "auth"), at("auth"), RuleIntegration, "%s proves itself by %s and names no credential; name the setting under configuration, marked secret, that holds it", name, auth)
	case auth == "none" && cred != nil:
		c.add(cred, at("credential"), RuleIntegration, "%s is called with auth none, so it is sent no credential; remove credential, or say how this system proves itself", name)
	case cred != nil:
		setting := d.settings[cred.Value]
		switch {
		case setting == nil:
			c.add(cred, at("credential"), RuleSetting, "%s is not a setting of the specification's configuration%s", cred.Value, suggest(cred.Value, d.settings))
		case source.Str(source.Child(setting, "secret")) != "true":
			c.add(cred, at("credential"), RuleIntegration, "%s is a setting that is not secret, and a credential is; mark it secret: true, so its value is never written in a specification", cred.Value)
		}
	}
	ops := source.Child(dep, "operations")
	if ops == nil {
		return
	}
	if source.Child(dep, "specification") != nil {
		c.add(ops, at("operations"), RuleIntegration, "%s names its specification, whose operations are its paths; call them as %s#/paths/..., and remove operations", name, name)
		return
	}
	for _, op := range source.Pairs(ops) {
		n := op.Value
		ptr := at("operations", op.Key.Value)
		method, channel := source.Child(n, "method") != nil, source.Child(n, "channel") != nil
		switch {
		case !method && !channel:
			c.add(op.Key, ptr, RuleIntegration, "%s of %s says neither method and path nor channel and message, so nothing says how it is called; give one of them", op.Key.Value, name)
		case method && channel:
			c.add(op.Key, ptr, RuleIntegration, "%s of %s gives both method and path and channel and message; an operation is a request or a message sent, so keep one", op.Key.Value, name)
		case channel && source.Child(n, "responses") != nil:
			c.add(source.Child(n, "responses"), ptr+"/responses", RuleIntegration, "%s of %s is a message sent, which has no answer; remove responses, or make it a request with method and path", op.Key.Value, name)
		}
		props := source.Child(source.Child(n, "request"), "properties")
		for i, r := range source.Items(source.Child(source.Child(n, "request"), "required")) {
			if source.Child(props, r.Value) == nil {
				c.add(r, ptr+"/request/required/"+fmt.Sprint(i), RuleIntegration, "%s is required, and the request has no field %s%s", r.Value, r.Value, suggestPairs(r.Value, props))
			}
		}
	}
}

// dependencyOperation is the operation a calls entry names under its
// dependency's operations, with its name, or nil when it names none.
func (d *design) dependencyOperation(ref string) (*yaml.Node, string) {
	dep, ptr, ok := crossRef(ref)
	if !ok || !strings.HasPrefix(ptr, "/operations/") {
		return nil, ""
	}
	name := strings.TrimPrefix(ptr, "/operations/")
	return source.Child(source.Child(d.dependencies[dep], "operations"), name), name
}

// checkCallReference checks a calls entry that names an operation under a
// dependency's operations: the dependency declares it and has no
// specification.
func (c *checker) checkCallReference(d *design, e callEntry, ptr string) bool {
	dep, p, ok := crossRef(e.ref.Value)
	if !ok || !strings.HasPrefix(p, "/operations/") || d.dependencies[dep] == nil {
		return true
	}
	name := strings.TrimPrefix(p, "/operations/")
	depNode := d.dependencies[dep]
	if source.Child(depNode, "specification") != nil {
		c.add(e.ref, ptr, RuleIntegration, "%s names its specification, so its operations are named by their paths; write %s#/paths/<path>/<method>", dep, dep)
		return false
	}
	ops := source.Child(depNode, "operations")
	if source.Child(ops, name) == nil {
		c.add(e.ref, ptr, RuleIntegration, "%s is not an operation of %s; declare it under %s's operations, with its method and path or its channel and message%s", name, dep, dep, suggestPairs(name, ops))
		return false
	}
	return true
}

// ours are the names a call's send expressions and idempotency key may
// use: the operation's parameters, its request body's fields and the
// fields of the record its guard names, with the names two of them share.
func (d *design) ours(o operation) (expr.Env, map[string][]string) {
	env := expr.Env{}
	from := map[string][]string{}
	put := func(name, where string, t expr.Type) {
		if _, ok := env[name]; !ok {
			env[name] = t
		}
		if !slices.Contains(from[name], where) {
			from[name] = append(from[name], where)
		}
	}
	for _, p := range append(source.Items(source.Child(o.pathItem, "parameters")), source.Items(source.Child(o.node, "parameters"))...) {
		if name := source.Str(source.Child(p, "name")); name != "" {
			put(name, "a parameter", d.fieldType(source.Child(p, "schema")))
		}
	}
	body, _ := d.requestFields(o.node)
	for _, name := range sortedKeys(body) {
		put(name, "a field of the request", d.fieldType(body[name]))
	}
	if ent := source.Str(source.Child(source.Child(o.node, "guard"), "entity")); d.entities[ent] != nil {
		fields := fieldsOf(d.entities[ent])
		for _, name := range sortedKeys(fields) {
			put(name, "a field of "+ent, d.fieldType(fields[name]))
		}
	}
	return env, from
}

// fieldsEnv is the names and types of an object schema's properties.
func (d *design) fieldsEnv(props *yaml.Node) expr.Env {
	env := expr.Env{}
	for _, p := range source.Pairs(props) {
		env[p.Key.Value] = d.fieldType(p.Value)
	}
	return env
}

// successFields are the fields of a dependency operation's first answer
// that succeeds, and its status.
func successFields(op *yaml.Node) (*yaml.Node, string) {
	for _, r := range source.Pairs(source.Child(op, "responses")) {
		if strings.HasPrefix(r.Key.Value, "2") {
			return source.Child(r.Value, "properties"), r.Key.Value
		}
	}
	return nil, ""
}

// checkCall checks one calls entry of an operation: what it names and, for
// an entry that gives the call's contract, each part of it.
func (c *checker) checkCall(d *design, o operation, e callEntry) {
	if !c.checkCallReference(d, e, o.pointer(e.at...)) || source.IsScalar(e.item) {
		return
	}
	n := e.item
	at := func(tokens ...string) string { return o.pointer(append(e.at[:2:2], tokens...)...) }
	dep := callDependency(e.ref.Value)
	their, theirName := d.dependencyOperation(e.ref.Value)
	_, _, cross := crossRef(e.ref.Value)
	called := dep
	if their != nil {
		called = dep + "'s " + theirName
	}
	mode := source.Str(source.Child(n, "mode"))
	if mode == "" {
		mode = "sync"
	}

	// The answer: a call that does not wait hears none.
	if mode != "sync" {
		if k := source.Child(n, "errors"); k != nil {
			c.add(k, at("errors"), RuleIntegration, "the call to %s is %s, so the operation hears no answer to map; remove errors, or make the call sync", called, mode)
		}
		if r := source.Child(source.Child(n, "mapping"), "receive"); r != nil {
			c.add(r, at("mapping", "receive"), RuleIntegration, "the call to %s is %s, so the operation hears no answer to keep; remove receive, or make the call sync", called, mode)
		}
	}
	cb := source.Child(n, "callback")
	switch {
	case mode == "async" && cb == nil:
		c.add(source.Child(n, "mode"), at("mode"), RuleIntegration, "the call to %s is async and names no callback, so nothing says how its answer comes back; name the channel message under callback", called)
	case mode != "async" && cb != nil:
		c.add(cb, at("callback"), RuleIntegration, "the call to %s is %s, and only an async call is answered by a callback; make it async, or remove callback", called, mode)
	case cb != nil && d.message(cb.Value) == nil:
		c.add(cb, at("callback"), RuleEmits, "%s does not name a channel and one of its messages; write channel/Message for a message declared under channels", cb.Value)
	}

	ours, from := d.ours(o)
	expression := func(v *yaml.Node, ptr, what string, env expr.Env, shared map[string][]string) (expr.Type, bool) {
		if v == nil || !source.IsScalar(v) {
			return expr.Type{}, false
		}
		tree, errs := expr.Parse(v.Value)
		if len(errs) > 0 {
			c.exprErrors(v, ptr, what, errs)
			return expr.Type{}, false
		}
		t, errs := expr.Check(tree, env)
		c.exprErrors(v, ptr, what, errs)
		if len(errs) > 0 {
			return expr.Type{}, false
		}
		for _, name := range exprNames(tree) {
			if len(shared[name]) > 1 {
				c.add(v, ptr, RuleIntegration, "%s names %s, which is both %s; rename one, so the call reads one value", what, name, strings.Join(shared[name], " and "))
				return expr.Type{}, false
			}
		}
		return t, true
	}

	// The mapping.
	mapping := source.Child(n, "mapping")
	if mapping != nil && their == nil && !cross {
		c.add(mapping, at("mapping"), RuleIntegration, "the call names %s alone, so its fields are not known; name the operation called, as %s#/operations/<name>, and declare it under %s's operations", dep, dep, dep)
	}
	if mapping != nil && their != nil {
		props := source.Child(source.Child(their, "request"), "properties")
		send := source.Child(mapping, "send")
		for _, p := range source.Pairs(send) {
			ptr := at("mapping", "send", p.Key.Value)
			field := source.Child(props, p.Key.Value)
			if field == nil {
				c.add(p.Key, ptr, RuleIntegration, "%s is not a field of %s's request%s", p.Key.Value, called, suggestPairs(p.Key.Value, props))
				continue
			}
			if t, ok := expression(p.Value, ptr, "the value sent as "+p.Key.Value, ours, from); ok {
				if want := d.fieldType(field); !assignable(t, want) {
					c.addFile(c.fileOf(p.Value), exprLine(p.Value, 1), ptr, RuleExpressionType, "the value sent as %s gives %s, and %s's %s takes %s", p.Key.Value, t, called, p.Key.Value, want)
				}
			}
		}
		var missing []string
		for _, r := range source.Items(source.Child(source.Child(their, "request"), "required")) {
			if source.Child(props, r.Value) != nil && source.Child(send, r.Value) == nil {
				missing = append(missing, r.Value)
			}
		}
		if len(missing) > 0 {
			where, ptr := send, at("mapping", "send")
			if where == nil {
				where, ptr = mapping, at("mapping")
			}
			c.add(where, ptr, RuleIntegration, "%s's request requires %s, which send does not give; add each under send", called, joinAnd(missing))
		}
		if receive := source.Child(mapping, "receive"); receive != nil && mode == "sync" {
			ent := source.Str(source.Child(source.Child(o.node, "guard"), "entity"))
			fields := fieldsOf(d.entities[ent])
			answer, _ := successFields(their)
			theirs := d.fieldsEnv(answer)
			for _, p := range source.Pairs(receive) {
				ptr := at("mapping", "receive", p.Key.Value)
				if d.entities[ent] == nil {
					c.add(p.Key, ptr, RuleIntegration, "%s changes no record, since its guard names no entity, so there is nothing to keep %s in; give the operation a guard, or remove receive", o.id, p.Key.Value)
					break
				}
				field := fields[p.Key.Value]
				if field == nil {
					c.add(p.Key, ptr, RuleIntegration, "%s is not a field of %s%s", p.Key.Value, ent, suggest(p.Key.Value, fields))
					continue
				}
				if answer == nil {
					c.add(p.Key, ptr, RuleIntegration, "%s declares no answer that succeeds with fields, so %s has nothing to be kept from; declare a 2xx response with its properties", called, p.Key.Value)
					continue
				}
				if t, ok := expression(p.Value, ptr, "the value kept as "+p.Key.Value, theirs, nil); ok {
					if want := d.fieldType(field); !assignable(t, want) {
						c.addFile(c.fileOf(p.Value), exprLine(p.Value, 1), ptr, RuleExpressionType, "the value kept as %s gives %s, and %s's %s is %s", p.Key.Value, t, ent, p.Key.Value, want)
					}
				}
			}
		}
	}

	// The idempotency key.
	key := source.Child(n, "idempotencyKey")
	if key != nil {
		expression(source.Child(key, "value"), at("idempotencyKey", "value"), "the idempotency key", ours, from)
	}

	// Their answers, by status, when the operation called is declared.
	declared := func(status string) bool {
		return source.Child(source.Child(their, "responses"), status) != nil
	}
	theirStatus := func(v *yaml.Node, ptr, what string) {
		s := v.Value
		switch {
		case s == "timeout":
		case strings.HasPrefix(s, "2"):
			c.add(v, ptr, RuleIntegration, "%s is an answer that succeeds, and %s names only answers that fail; remove it", s, what)
		case their != nil && source.Child(their, "responses") != nil && !declared(s):
			c.add(v, ptr, RuleIntegration, "%s is not an answer of %s; name one of its responses that fails, or timeout", s, called)
		}
	}
	retries := source.Child(n, "retries")
	for i, s := range source.Items(source.Child(retries, "on")) {
		theirStatus(s, at("retries", "on", fmt.Sprint(i)), "retries")
	}
	if retries != nil && key == nil {
		if m := source.Str(source.Child(their, "method")); m == "post" || m == "patch" {
			c.warn(retries, at("retries"), RuleIntegration, "the call to %s is tried again, and a %s is not idempotent, so a try whose answer was lost may act twice; send an idempotencyKey", called, m)
		}
	}
	for _, p := range source.Pairs(source.Child(n, "errors")) {
		ptr := at("errors", p.Key.Value)
		theirStatus(p.Key, ptr, "errors")
		if source.Child(source.Child(o.node, "responses"), p.Value.Value) == nil {
			c.add(p.Value, ptr, RuleIntegration, "%s answers no %s; declare that response, or map %s to one it declares", o.id, p.Value.Value, p.Key.Value)
		}
	}
}

// assignable says whether a value of type t may be written where want is
// taken: the same kind, a whole number where a decimal or a double is
// taken, or an enum value as a string. An object or a list is not looked
// into.
func assignable(t, want expr.Type) bool {
	switch {
	case t.Kind == want.Kind, t.Kind == expr.Null && want.Nullable:
		return true
	case t.Kind == expr.Object || t.Kind == expr.List || want.Kind == expr.Object || want.Kind == expr.List:
		return true
	case (t.Kind == expr.Int || t.Kind == expr.Uint) && (want.Kind == expr.Decimal || want.Kind == expr.Double):
		return true
	case t.Kind == expr.Enum && want.Kind == expr.String:
		return true
	}
	return false
}

// exprNames are the names an expression reads, each once, in order.
func exprNames(n *expr.Node) []string {
	var out []string
	var walk func(*expr.Node)
	walk = func(n *expr.Node) {
		if n == nil {
			return
		}
		if n.Op == expr.OpName && !slices.Contains(out, n.Text) {
			out = append(out, n.Text)
		}
		for _, a := range n.Args {
			walk(a)
		}
	}
	walk(n)
	return out
}
