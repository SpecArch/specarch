package validate

import (
	"fmt"
	"slices"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/source"
)

// A call's mapping may hold worked examples (ADR-207): our values and
// what send gives from them, their answer and what receive keeps from
// it. The validator evaluates each, and specarch try prints the steps.

// mappingExpression is one field of send or receive: the field it gives,
// its expression parsed and checked, and the names it reads.
type mappingExpression struct {
	field string
	node  *yaml.Node
	tree  *expr.Node
	types map[*expr.Node]expr.Type
	gives expr.Type
	reads []string
	ok    bool // the expression parses and checks
}

// mappingSide is send or receive of a mapping: its expressions in the
// order written, the fields they may read with their types, and the
// fields their values are written to.
type mappingSide struct {
	name    string // send or receive
	given   *yaml.Node
	exprs   []mappingExpression
	env     expr.Env
	reads   map[string]*yaml.Node // the fields the expressions may read
	targets map[string]*yaml.Node // the fields they give
}

// mappingSides reads a call's send and receive for its examples: send
// reads our values and gives their request's fields, receive reads their
// answer's fields and gives our record's.
func (d *design) mappingSides(o operation, mapping, their *yaml.Node, ours expr.Env, oursFields map[string]*yaml.Node) (send, receive mappingSide) {
	send = mappingSide{name: "send", given: source.Child(mapping, "send"), env: ours, reads: oursFields, targets: map[string]*yaml.Node{}}
	for _, p := range source.Pairs(source.Child(source.Child(their, "request"), "properties")) {
		send.targets[p.Key.Value] = p.Value
	}
	answer, _ := successFields(their)
	receive = mappingSide{name: "receive", given: source.Child(mapping, "receive"), env: d.fieldsEnv(answer), reads: map[string]*yaml.Node{}, targets: map[string]*yaml.Node{}}
	for _, p := range source.Pairs(answer) {
		receive.reads[p.Key.Value] = p.Value
	}
	if ent := source.Str(source.Child(source.Child(o.node, "guard"), "entity")); d.entities[ent] != nil {
		receive.targets = fieldsOf(d.entities[ent])
	}
	for _, side := range []*mappingSide{&send, &receive} {
		for _, p := range source.Pairs(side.given) {
			m := mappingExpression{field: p.Key.Value, node: p.Value}
			if source.IsScalar(p.Value) {
				tree, errs := expr.Parse(p.Value.Value)
				if len(errs) == 0 {
					m.tree = tree
					m.gives, m.types, errs = expr.CheckTypes(tree, side.env)
					m.ok = len(errs) == 0 && side.targets[m.field] != nil
					for _, name := range exprNames(tree) {
						if side.reads[name] != nil {
							m.reads = append(m.reads, name)
						}
					}
				}
			}
			side.exprs = append(side.exprs, m)
		}
	}
	return send, receive
}

// readsAll is every name the expressions of a side read, in order.
func (s mappingSide) readsAll() []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range s.exprs {
		for _, name := range m.reads {
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	return out
}

// widened is a whole number read where a decimal is taken, as a decimal,
// so a value sent from an int compares with the decimal it is sent as.
func widened(v expr.Value, want expr.Type) expr.Value {
	if want.Kind == expr.Decimal && (v.Kind == expr.Int || v.Kind == expr.Uint) {
		v.Kind = expr.Decimal
	}
	return v
}

// checkMappingExamples checks each worked example of a call's mapping:
// its values read at their fields, the names given against the names
// read, and each field send and receive give against the example's.
func (c *checker) checkMappingExamples(d *design, o operation, e callEntry, their *yaml.Node, called, mode string, ours expr.Env, oursFields map[string]*yaml.Node) {
	mapping := source.Child(e.item, "mapping")
	examples := source.Items(source.Child(mapping, "examples"))
	if len(examples) == 0 {
		return
	}
	send, receive := d.mappingSides(o, mapping, their, ours, oursFields)
	for i, ex := range examples {
		base := append(e.at[:2:2], "mapping", "examples", fmt.Sprint(i))
		at := func(tokens ...string) string { return o.pointer(append(base, tokens...)...) }
		label := source.Str(source.Child(ex, "name"))
		if label == "" {
			label = fmt.Sprintf("example %d", i+1)
		}
		if sent := source.Child(ex, "sent"); sent != nil && send.given == nil {
			c.add(sent, at("sent"), RuleIntegration, "the mapping of the call to %s has no send, so %q has nothing sent to check; remove sent, or give send", called, label)
		} else if sent != nil {
			c.checkMappingSide(d, label, send, source.Child(ex, "inputs"), sent, "inputs", "sent", at)
		}
		kept := source.Child(ex, "kept")
		switch {
		case kept == nil:
		case mode != "sync":
			c.add(kept, at("kept"), RuleIntegration, "the call to %s is %s, so the operation hears no answer to keep; remove kept, or make the call sync", called, mode)
		case receive.given == nil:
			c.add(kept, at("kept"), RuleIntegration, "the mapping of the call to %s has no receive, so %q has nothing kept to check; remove kept, or give receive", called, label)
		default:
			c.checkMappingSide(d, label, receive, source.Child(ex, "answer"), kept, "answer", "kept", at)
		}
	}
}

// checkMappingSide checks one side of a mapping example: the values given
// (inputs or answer) and the values expected (sent or kept).
func (c *checker) checkMappingSide(d *design, label string, side mappingSide, given, expected *yaml.Node, givenKey, expectedKey string, at func(...string) string) {
	vals := map[string]expr.Value{}
	ok := true
	reads := side.readsAll()
	for _, p := range source.Pairs(given) {
		field := side.reads[p.Key.Value]
		switch {
		case field == nil && side.name == "send":
			c.add(p.Key, at(givenKey, p.Key.Value), RuleExampleInput, "%s is not a parameter, a field of the request or a field of the record the operation changes; remove it or correct its name%s", p.Key.Value, suggest(p.Key.Value, side.reads))
			ok = false
			continue
		case field == nil:
			c.add(p.Key, at(givenKey, p.Key.Value), RuleExampleInput, "%s is not a field of their answer; remove it or correct its name%s", p.Key.Value, suggest(p.Key.Value, side.reads))
			ok = false
			continue
		case !slices.Contains(reads, p.Key.Value):
			c.add(p.Key, at(givenKey, p.Key.Value), RuleExampleInput, "%s reads no %s, so %q need not give it; remove it", side.name, p.Key.Value, label)
			ok = false
			continue
		}
		v, msg := d.value(p.Value, field)
		if msg != "" {
			c.add(p.Value, at(givenKey, p.Key.Value), RuleExampleInput, "%s %s of %q %s", givenKey, p.Key.Value, label, msg)
			ok = false
			continue
		}
		vals[p.Key.Value] = v
	}
	var missing []string
	for _, name := range reads {
		if source.Child(given, name) == nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		where, ptr := given, at(givenKey)
		if where == nil {
			where, ptr = expected, at(expectedKey)
		}
		c.add(where, ptr, RuleExampleInput, "%q gives no value for %s, which %s reads; add each under %s", label, joinAnd(missing), side.name, givenKey)
		ok = false
	}
	want := map[string]expr.Value{}
	for _, p := range source.Pairs(expected) {
		field := side.targets[p.Key.Value]
		if source.Child(side.given, p.Key.Value) == nil || field == nil {
			c.add(p.Key, at(expectedKey, p.Key.Value), RuleExampleExpected, "%s gives no %s, so %q has no value to expect for it; remove it or correct its name%s", side.name, p.Key.Value, label, suggestPairs(p.Key.Value, side.given))
			continue
		}
		v, msg := d.value(p.Value, field)
		if msg != "" {
			c.add(p.Value, at(expectedKey, p.Key.Value), RuleExampleExpected, "%s %s of %q %s", expectedKey, p.Key.Value, label, msg)
			continue
		}
		want[p.Key.Value] = v
	}
	var left []string
	for _, m := range side.exprs {
		if source.Child(expected, m.field) == nil {
			left = append(left, m.field)
		}
	}
	if len(left) > 0 {
		c.add(expected, at(expectedKey), RuleExampleExpected, "%q gives no %s value for %s, which %s gives; add each under %s", label, expectedKey, joinAnd(left), side.name, expectedKey)
	}
	if !ok {
		return
	}
	what := "sent"
	if side.name == "receive" {
		what = "kept"
	}
	for _, m := range side.exprs {
		expectedValue, has := want[m.field]
		if !m.ok || !has {
			continue
		}
		wantType := d.fieldType(side.targets[m.field])
		got, err := expr.Eval(m.tree, vals, c.zoneOf())
		node := source.Child(expected, m.field)
		ptr := at(expectedKey, m.field)
		if err != nil {
			c.add(node, ptr, RuleExampleError, "the value %s as %s cannot be computed for %q: %v; correct the %s or the mapping", what, m.field, label, err, givenKey)
			continue
		}
		if got = widened(got, wantType); !expr.Equal(got, expectedValue) {
			c.add(node, ptr, RuleExampleMismatch, "the value %s as %s gives %s for %q, but the example expects %s; correct the example or the mapping", what, m.field, show(got, wantType), label, show(expectedValue, wantType))
		}
	}
}
