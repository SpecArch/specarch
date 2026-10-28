package validate

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// TriedField is one field of a mapping example as specarch try evaluated
// it: sent, a field of their request from send, or kept, a field of our
// record from receive, with the steps of its expression.
type TriedField struct {
	Direction  string    `json:"direction"` // sent or kept
	Field      string    `json:"field"`
	Expression string    `json:"expression"`
	Steps      []TryStep `json:"steps"`
	Outcome    string    `json:"outcome"`
	Result     *string   `json:"result,omitempty"`
	Expected   *string   `json:"expected,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// TriedCall is what specarch try found for the mapping examples of one
// call of an operation.
type TriedCall struct {
	Call        string       `json:"call"`      // the call as the derived cases name it
	Reference   string       `json:"reference"` // the calls entry's call
	File        string       `json:"file"`
	Evaluations []Evaluation `json:"evaluations"`
}

// TriedOperation is what specarch try found for an operation: each of its
// calls whose mapping has examples (ADR-207).
type TriedOperation struct {
	Operation string      `json:"operation"`
	Calls     []TriedCall `json:"calls"`
}

// Evaluated is what specarch try evaluated, an algorithm's examples or an
// operation's mapping examples, for TriedHere.
type Evaluated interface{ pointers() []string }

// pointers are where the evaluated examples are, for TriedHere.
func (t *Tried) pointers() []string {
	if t == nil {
		return nil
	}
	var out []string
	for _, e := range t.Evaluations {
		if e.Number > 0 {
			out = append(out, source.Pointer("algorithms", t.Algorithm, "examples", fmt.Sprint(e.Number-1)))
		}
	}
	return out
}

func (t *TriedOperation) pointers() []string {
	if t == nil {
		return nil
	}
	var out []string
	for _, c := range t.Calls {
		for _, e := range c.Evaluations {
			out = append(out, e.pointer)
		}
	}
	return out
}

// TryOperation evaluates the mapping examples of an operation's calls,
// or the one example named, as Try evaluates an algorithm's (ADR-207).
// It returns false when the specification has no operation of that name,
// and a refusal when an algorithm has the name too or the operation has
// no mapping example.
func TryOperation(s *spec.Spec, name, example string) (*TriedOperation, string, bool) {
	if s.Root == nil {
		return nil, "", false
	}
	d := newDesign(s.Root)
	d.spec = s
	for _, o := range d.opList {
		if o.id != name {
			continue
		}
		if source.Child(source.Child(s.Root, "algorithms"), name) != nil {
			return nil, fmt.Sprintf("%s is both an algorithm and an operation's operationId, so it is not known which to try; rename one", name), true
		}
		c := &checker{file: s.RootFile, files: s.Files, root: s.Root}
		c.checkTimeZone(d)
		t, refusal := c.tryOperation(d, o, example)
		return t, refusal, true
	}
	return nil, "", false
}

// tryNames are the specification's algorithms, and its operations whose
// calls have mapping examples.
func tryNames(root *yaml.Node) (algorithms, operations []string) {
	for _, p := range source.Pairs(source.Child(root, "algorithms")) {
		algorithms = append(algorithms, p.Key.Value)
	}
	sort.Strings(algorithms)
	d := newDesign(root)
	for _, o := range d.opList {
		for _, e := range callEntries(o.node) {
			if len(source.Items(source.Child(source.Child(e.item, "mapping"), "examples"))) > 0 {
				operations = append(operations, o.id)
				break
			}
		}
	}
	sort.Strings(operations)
	return algorithms, operations
}

// tryOperation evaluates the mapping examples of an operation's calls, or
// the one example named.
func (c *checker) tryOperation(d *design, o operation, example string) (*TriedOperation, string) {
	t := &TriedOperation{Operation: o.id, Calls: []TriedCall{}}
	ours, _, oursFields := d.ours(o)
	entries := callEntries(o.node)
	names := callNames(entries)
	var labels []string
	unchecked := false
	for k, e := range entries {
		mapping := source.Child(e.item, "mapping")
		examples := source.Items(source.Child(mapping, "examples"))
		their, _ := d.dependencyOperation(e.ref.Value)
		if len(examples) > 0 && their == nil {
			unchecked = true
		}
		if len(examples) == 0 || their == nil {
			continue
		}
		send, receive := d.mappingSides(o, mapping, their, ours, oursFields)
		tc := TriedCall{Call: names[k], Reference: e.ref.Value, File: c.fileOf(mapping), Evaluations: []Evaluation{}}
		for i, ex := range examples {
			name := source.Str(source.Child(ex, "name"))
			if name != "" {
				labels = append(labels, expr.Quote(name))
			}
			if example != "" && name != example {
				continue
			}
			ev := Evaluation{Number: i + 1, Name: name, Inputs: []TryInput{}, Steps: []TryStep{}, Fields: []TriedField{}}
			ev.pointer = o.pointer(append(e.at[:2:2], "mapping", "examples", fmt.Sprint(i))...)
			sent, kept := source.Child(ex, "sent"), source.Child(ex, "kept")
			var sentVals, keptVals map[string]expr.Value
			switch {
			case sent == nil:
			case send.given == nil:
				ev.Unread = append(ev.Unread, "the mapping has no send, so nothing is sent")
			default:
				ev.Inputs, sentVals = c.tryGiven(d, &ev, send, source.Child(ex, "inputs"), "input")
			}
			mode := source.Str(source.Child(e.item, "mode"))
			switch {
			case kept == nil:
			case mode != "" && mode != "sync":
				ev.Unread = append(ev.Unread, fmt.Sprintf("the call is %s, so it hears no answer to keep", mode))
			case receive.given == nil:
				ev.Unread = append(ev.Unread, "the mapping has no receive, so nothing is kept")
			default:
				ev.Answer, keptVals = c.tryGiven(d, &ev, receive, source.Child(ex, "answer"), "answer")
			}
			if len(ev.Unread) > 0 {
				ev.Outcome = OutcomeUnread
				tc.Evaluations = append(tc.Evaluations, ev)
				continue
			}
			ev.Outcome = OutcomePass
			if sent != nil {
				c.tryFields(d, &ev, send, sentVals, sent, "sent")
			}
			if kept != nil {
				c.tryFields(d, &ev, receive, keptVals, kept, "kept")
			}
			tc.Evaluations = append(tc.Evaluations, ev)
		}
		if len(tc.Evaluations) > 0 {
			t.Calls = append(t.Calls, tc)
		}
	}
	if len(t.Calls) == 0 {
		switch {
		case example == "" && unchecked:
			return nil, fmt.Sprintf("operation %s has mapping examples only on calls through another specification's paths, whose fields are not known here, so they are read and not evaluated", o.id)
		case example == "":
			return nil, fmt.Sprintf("operation %s has no call whose mapping has examples; write them under a call's mapping, as examples", o.id)
		case len(labels) == 0:
			return nil, fmt.Sprintf("operation %s has no example %s; none of its mapping examples has a name", o.id, expr.Quote(example))
		default:
			return nil, fmt.Sprintf("operation %s has no example %s; its mapping examples are %s", o.id, expr.Quote(example), strings.Join(labels, ", "))
		}
	}
	return t, ""
}

// tryGiven reads the values a side of a mapping example gives, inputs or
// answer, in the order its expressions read them, and says in the
// evaluation which cannot be read.
func (c *checker) tryGiven(d *design, ev *Evaluation, side mappingSide, given *yaml.Node, what string) ([]TryInput, map[string]expr.Value) {
	shown := []TryInput{}
	vals := map[string]expr.Value{}
	reads := side.readsAll()
	for _, p := range source.Pairs(given) {
		if !slices.Contains(reads, p.Key.Value) {
			ev.Unread = append(ev.Unread, fmt.Sprintf("%s reads no %s", side.name, p.Key.Value))
		}
	}
	for _, name := range reads {
		n := source.Child(given, name)
		if n == nil {
			ev.Unread = append(ev.Unread, fmt.Sprintf("no value is given for %s %s, which %s reads", what, name, side.name))
			continue
		}
		v, msg := d.value(n, side.reads[name])
		if msg != "" {
			ev.Unread = append(ev.Unread, fmt.Sprintf("%s %s %s", what, name, msg))
			continue
		}
		vals[name] = v
		shown = append(shown, TryInput{Name: name, Value: expr.Show(v, side.env[name])})
	}
	return shown, vals
}

// tryFields evaluates each expression of a side of a mapping on the
// values given and compares it with the example's value.
func (c *checker) tryFields(d *design, ev *Evaluation, side mappingSide, vals map[string]expr.Value, expected *yaml.Node, direction string) {
	for _, p := range source.Pairs(expected) {
		if source.Child(side.given, p.Key.Value) == nil {
			ev.Outcome = OutcomeFail
			ev.Fields = append(ev.Fields, TriedField{Direction: direction, Field: p.Key.Value, Steps: []TryStep{}, Outcome: OutcomeFail, Error: fmt.Sprintf("%s gives no %s", side.name, p.Key.Value)})
		}
	}
	for _, m := range side.exprs {
		f := TriedField{Direction: direction, Field: m.field, Expression: source.Str(m.node), Steps: []TryStep{}}
		target := side.targets[m.field]
		node := source.Child(expected, m.field)
		var want expr.Value
		switch {
		case !m.ok:
			f.Outcome, f.Error = OutcomeError, "the expression does not check; the validator's lines above say why"
		case node == nil:
			f.Outcome, f.Error = OutcomeFail, fmt.Sprintf("the example gives no %s value for %s", direction, m.field)
		default:
			v, msg := d.value(node, target)
			if msg != "" {
				f.Outcome, f.Error = OutcomeUnread, fmt.Sprintf("the %s value %s", direction, msg)
				break
			}
			want = v
			text := expr.Show(v, d.fieldType(target))
			f.Expected = &text
		}
		if f.Outcome == "" {
			wantType := d.fieldType(target)
			got, steps, err := expr.Trace(m.tree, vals, c.zoneOf())
			for _, st := range steps {
				f.Steps = append(f.Steps, TryStep{Expression: expr.Text(st.Node), Line: exprLine(m.node, st.Node.Line), Column: c.exprColumn(m.node, st.Node.Line, st.Node.Column), Value: expr.Show(st.Value, m.types[st.Node])})
			}
			if err != nil {
				f.Outcome, f.Error = OutcomeError, err.Error()
			} else {
				got = widened(got, wantType)
				text := expr.Show(got, wantType)
				f.Result = &text
				f.Outcome = OutcomePass
				if !expr.Equal(got, want) {
					f.Outcome = OutcomeFail
				}
			}
		}
		if f.Outcome != OutcomePass && ev.Outcome == OutcomePass {
			ev.Outcome = f.Outcome
		}
		ev.Fields = append(ev.Fields, f)
	}
}
