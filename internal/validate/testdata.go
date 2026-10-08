package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/source"
)

// checkTestData checks the structured fixture, input and expect of every
// test against the design, and that a test does not say the same thing in
// a data folder too.
func (c *checker) checkTestData(d *design) {
	for _, p := range source.Pairs(source.Child(d.root, "tests")) {
		name, t := p.Key.Value, p.Value
		base := []string{"tests", name}
		kind, label := testKindOf(t)
		if kind == "" {
			continue // the schema reports a test without a subject
		}
		if f := source.Child(t, "fixture"); f != nil {
			for _, kv := range source.Pairs(f) {
				ptr := source.Pointer(append(base, "fixture", kv.Key.Value)...)
				switch {
				case kv.Key.Value == "caller":
					if v := source.Str(kv.Value); v != "" && v != "public" && d.roles[v] == nil {
						c.add(kv.Value, ptr, RuleTestData, "caller %s is neither public nor a role of the specification%s", v, suggest(v, d.roles))
					}
				case strings.HasPrefix(kv.Key.Value, "x-"):
				default:
					c.checkRecords(d, kv, ptr)
				}
			}
		}
		if in := source.Child(t, "input"); in != nil {
			c.checkInput(d, t, kind, label, in, source.Pointer(append(base, "input")...))
		}
		if ex := source.Child(t, "expect"); ex != nil {
			c.checkExpect(d, t, kind, label, ex, append(base, "expect"))
		}
		if d.spec != nil {
			folder := filepath.Join(d.spec.Dir, "tests", name)
			for _, pair := range [][2]string{{"input", "input"}, {"expect", "expected"}} {
				n := source.Child(t, pair[0])
				if n == nil {
					continue
				}
				if info, err := os.Stat(filepath.Join(folder, pair[1])); err == nil && info.IsDir() {
					c.add(source.Key(t, pair[0]), source.Pointer(append(base, pair[0])...), RuleTestData,
						"the test has %s and %s %s/ folder beside it, which say the same thing twice; keep %s, or keep the folder for what %s cannot hold", pair[0], indefinite(pair[1]), pair[1], pair[0], pair[0])
				}
			}
		}
	}
}

// testKindOf is the kind of subject a test names, and how a message names
// it.
func testKindOf(t *yaml.Node) (kind, label string) {
	for _, k := range []string{"operation", "command", "page", "job", "requirement"} {
		if v := source.Str(source.Child(t, k)); v != "" {
			return k, k + " " + v
		}
	}
	if v := source.Str(source.Child(t, "flow")); v != "" {
		return "screenFlow", "flow " + v
	}
	ent := source.Str(source.Child(t, "entity"))
	switch {
	case ent == "":
		return "", ""
	case source.Child(t, "constraint") != nil:
		return "constraint", ent + " constraint " + source.Str(source.Child(t, "constraint"))
	case source.Child(t, "transition") != nil:
		return "transition", ent + " transition"
	}
	return "flow", ent + " state machine"
}

// fieldValue checks one value against a field's schema and returns the end
// of a sentence when it does not fit. An object or a list is not checked:
// a test names them only as a whole.
func (d *design) fieldValue(n, field *yaml.Node) string {
	if field == nil {
		return ""
	}
	switch d.fieldType(field).Kind {
	case expr.Object, expr.List:
		return ""
	}
	_, msg := d.value(n, field)
	return msg
}

// checkRecords checks records by entity name: the entity and its fields
// exist, the values have their types, and the entity's check constraints
// hold on the fields a record gives.
func (c *checker) checkRecords(d *design, kv source.Pair, ptr string) {
	ent := kv.Key.Value
	entity := d.entities[ent]
	if entity == nil {
		c.add(kv.Key, ptr, RuleTestData, "%s is not an entity of the specification%s", ent, suggest(ent, d.entities))
		return
	}
	fields := fieldsOf(entity)
	for i, rec := range source.Items(kv.Value) {
		rptr := ptr + "/" + fmt.Sprint(i)
		vals := map[string]expr.Value{}
		for _, f := range source.Pairs(rec) {
			field := fields[f.Key.Value]
			fptr := rptr + "/" + source.EscapeToken(f.Key.Value)
			if field == nil {
				c.add(f.Key, fptr, RuleTestData, "%s is not a field of %s%s", f.Key.Value, ent, suggest(f.Key.Value, fields))
				continue
			}
			if msg := d.fieldValue(f.Value, field); msg != "" {
				c.add(f.Value, fptr, RuleTestData, "%s of this %s %s", f.Key.Value, ent, msg)
				continue
			}
			if v, msg := d.value(f.Value, field); msg == "" {
				vals[f.Key.Value] = v
			}
		}
		for _, con := range source.Pairs(source.Child(entity, "constraints")) {
			if source.Str(source.Child(con.Value, "kind")) != "check" {
				continue
			}
			tree, errs := expr.Parse(source.Str(source.Child(con.Value, "expression")))
			if len(errs) > 0 || !allGiven(tree, vals) {
				continue
			}
			if got, err := expr.Eval(tree, vals); err == nil && got.Kind == expr.Bool && !got.Bool {
				c.add(rec, rptr, RuleTestData, "this %s breaks %s (%s); a test holds only records that could exist", ent, con.Key.Value, strings.TrimSuffix(source.Str(source.Child(con.Value, "message")), "."))
			}
		}
	}
}

// allGiven reports whether every name the expression uses has a value.
func allGiven(n *expr.Node, vals map[string]expr.Value) bool {
	if n.Op == expr.OpName {
		if _, ok := vals[n.Text]; !ok {
			return false
		}
	}
	for _, a := range n.Args {
		if !allGiven(a, vals) {
			return false
		}
	}
	return true
}

// checkInput checks what a test's call carries against its subject.
func (c *checker) checkInput(d *design, t *yaml.Node, kind, label string, in *yaml.Node, ptr string) {
	switch kind {
	case "operation":
		o, ok := d.operations[source.Str(source.Child(t, "operation"))]
		if !ok {
			return // reported as test_subject
		}
		allowed := map[string]*yaml.Node{}
		for _, p := range append(source.Items(source.Child(o.pathItem, "parameters")), source.Items(source.Child(o.node, "parameters"))...) {
			allowed[source.Str(source.Child(p, "name"))] = source.Child(p, "schema")
		}
		body, _ := d.requestFields(o.node)
		for k, v := range body {
			allowed[k] = v
		}
		for _, kv := range source.Pairs(in) {
			field, known := allowed[kv.Key.Value]
			kptr := ptr + "/" + source.EscapeToken(kv.Key.Value)
			if !known {
				c.add(kv.Key, kptr, RuleTestData, "%s is not a parameter or body field of operation %s%s", kv.Key.Value, o.id, suggest(kv.Key.Value, allowed))
				continue
			}
			if msg := d.fieldValue(kv.Value, field); msg != "" {
				c.add(kv.Value, kptr, RuleTestData, "input %s %s", kv.Key.Value, msg)
			}
		}
	case "command":
		cmd := d.commands[source.Str(source.Child(t, "command"))]
		if cmd == nil {
			return
		}
		for _, kv := range source.Pairs(in) {
			kptr := ptr + "/" + source.EscapeToken(kv.Key.Value)
			switch kv.Key.Value {
			case "arguments":
				args := map[string]*yaml.Node{}
				for _, a := range source.Items(source.Child(cmd, "arguments")) {
					args[source.Str(source.Child(a, "name"))] = a
				}
				for _, a := range source.Pairs(kv.Value) {
					arg := args[a.Key.Value]
					aptr := kptr + "/" + source.EscapeToken(a.Key.Value)
					if arg == nil {
						c.add(a.Key, aptr, RuleTestData, "%s is not an argument of command %s%s", a.Key.Value, source.Str(source.Child(t, "command")), suggest(a.Key.Value, args))
						continue
					}
					values := []*yaml.Node{a.Value}
					if source.Str(source.Child(arg, "repeatable")) == "true" && source.Deref(a.Value).Kind == yaml.SequenceNode {
						values = source.Items(a.Value)
					}
					for _, v := range values {
						if msg := d.fieldValue(v, source.Child(arg, "schema")); msg != "" {
							c.add(v, aptr, RuleTestData, "argument %s %s", a.Key.Value, msg)
						}
					}
				}
			case "options":
				opts := topMap(cmd, "options")
				for _, o := range source.Pairs(kv.Value) {
					opt := opts[o.Key.Value]
					optr := kptr + "/" + source.EscapeToken(o.Key.Value)
					if opt == nil {
						c.add(o.Key, optr, RuleTestData, "%s is not an option of command %s%s", o.Key.Value, source.Str(source.Child(t, "command")), suggest(o.Key.Value, opts))
						continue
					}
					if msg := d.fieldValue(o.Value, source.Child(opt, "schema")); msg != "" {
						c.add(o.Value, optr, RuleTestData, "option %s %s", o.Key.Value, msg)
					}
				}
			default:
				c.add(kv.Key, kptr, RuleTestData, "%s is not part of a command's input, which has arguments and options", kv.Key.Value)
			}
		}
	case "page":
		page := d.pages[source.Str(source.Child(t, "page"))]
		if page == nil {
			return
		}
		params := map[string]*yaml.Node{}
		for _, m := range pathParam.FindAllStringSubmatch(source.Str(source.Child(page, "route")), -1) {
			params[m[1]] = page
		}
		for _, kv := range source.Pairs(in) {
			kptr := ptr + "/" + source.EscapeToken(kv.Key.Value)
			if kv.Key.Value == "action" {
				labels := map[string]*yaml.Node{}
				for _, a := range source.Items(source.Child(page, "actions")) {
					labels[source.Str(source.Child(a, "label"))] = a
				}
				if v := source.Str(kv.Value); labels[v] == nil {
					c.add(kv.Value, kptr, RuleTestData, "%s is not an action of page %s%s", v, source.Str(source.Child(t, "page")), suggest(v, labels))
				}
				continue
			}
			if params[kv.Key.Value] == nil {
				c.add(kv.Key, kptr, RuleTestData, "%s is not a route parameter of page %s; a page's input is its route parameters and action", kv.Key.Value, source.Str(source.Child(t, "page")))
			}
		}
	default:
		c.add(source.Key(t, "input"), ptr, RuleTestData, "input is for an operation, a command or a page, and this test is about %s", label)
	}
}

// checkExpect checks a test's expected outcome against its subject.
func (c *checker) checkExpect(d *design, t *yaml.Node, kind, label string, ex *yaml.Node, base []string) {
	var o operation
	isOp := false
	if kind == "operation" {
		o, isOp = d.operations[source.Str(source.Child(t, "operation"))]
	}
	only := func(key, subject string) bool {
		if n := source.Key(ex, key); n != nil && kind != subject {
			c.add(n, source.Pointer(append(base, key)...), RuleTestData, "%s is for %s %s, and this test is about %s", key, indefinite(subject), subject, label)
			return false
		}
		return true
	}
	if n := source.Child(ex, "status"); n != nil && only("status", "operation") && isOp {
		if source.Child(source.Child(o.node, "responses"), n.Value) == nil {
			c.add(n, source.Pointer(append(base, "status")...), RuleTestData, "operation %s has no response %s%s", o.id, n.Value, suggest(n.Value, topMap(o.node, "responses")))
		}
	}
	if b := source.Child(ex, "body"); b != nil && only("body", "operation") && isOp {
		if ent := d.statusEntity(o.node, source.Str(source.Child(ex, "status"))); ent != "" {
			fields := fieldsOf(d.entities[ent])
			for _, kv := range source.Pairs(b) {
				ptr := source.Pointer(append(base, "body", kv.Key.Value)...)
				field := fields[kv.Key.Value]
				if field == nil {
					c.add(kv.Key, ptr, RuleTestData, "%s is not a field of %s, which the response returns%s", kv.Key.Value, ent, suggest(kv.Key.Value, fields))
					continue
				}
				if msg := d.fieldValue(kv.Value, field); msg != "" {
					c.add(kv.Value, ptr, RuleTestData, "%s of the body %s", kv.Key.Value, msg)
				}
			}
		}
	}
	if n := source.Child(ex, "exit"); n != nil && only("exit", "command") {
		if cmd := d.commands[source.Str(source.Child(t, "command"))]; cmd != nil && source.Child(source.Child(cmd, "exitCodes"), n.Value) == nil {
			c.add(n, source.Pointer(append(base, "exit")...), RuleTestData, "command %s has no exit code %s%s", source.Str(source.Child(t, "command")), n.Value, suggest(n.Value, topMap(cmd, "exitCodes")))
		}
	}
	only("standardOutput", "command")
	if s := source.Child(ex, "state"); s != nil {
		for _, kv := range source.Pairs(s) {
			c.checkRecords(d, kv, source.Pointer(append(base, "state", kv.Key.Value)...))
		}
	}
	if e := source.Child(ex, "emits"); e != nil && only("emits", "operation") && isOp {
		emitted := map[string]*yaml.Node{}
		for _, m := range source.Items(source.Child(o.node, "emits")) {
			emitted[m.Value] = m
		}
		for i, m := range source.Items(e) {
			if emitted[m.Value] == nil {
				c.add(m, source.Pointer(append(base, "emits", fmt.Sprint(i))...), RuleTestData, "operation %s does not emit %s%s", o.id, m.Value, suggest(m.Value, emitted))
			}
		}
	}
	if source.Child(ex, "emits") != nil && source.Child(ex, "emitsNothing") != nil {
		c.add(source.Key(ex, "emitsNothing"), source.Pointer(append(base, "emitsNothing")...), RuleTestData, "emits and emitsNothing say opposite things; keep one")
	}
}

// statusEntity is the entity the response with a status returns, alone or
// in a list; with no status, the first success response's.
func (d *design) statusEntity(op *yaml.Node, status string) string {
	for _, r := range source.Pairs(source.Child(op, "responses")) {
		if status != "" && r.Key.Value != status || status == "" && !strings.HasPrefix(r.Key.Value, "2") {
			continue
		}
		for _, ct := range source.Pairs(source.Child(r.Value, "content")) {
			schema := source.Child(ct.Value, "schema")
			if items := source.Child(schema, "items"); items != nil {
				schema = items
			}
			if ref := source.Str(source.Child(schema, "$ref")); strings.HasPrefix(ref, "#/entities/") {
				return strings.TrimPrefix(ref, "#/entities/")
			}
		}
		return ""
	}
	return ""
}

// indefinite is "a" or "an" before a word.
func indefinite(word string) string {
	if strings.ContainsRune("aeiou", rune(word[0])) {
		return "an"
	}
	return "a"
}
