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

// The outcomes of an evaluation of specarch try.
const (
	OutcomePass     = "pass"               // an example whose expected value the formula gives
	OutcomeFail     = "fail"               // an example whose expected value it does not give
	OutcomeError    = "cannot be computed" // the formula fails on the inputs
	OutcomeUnread   = "cannot be read"     // an input or the expected value does not fit its field
	OutcomeComputed = "computed"           // given inputs, which have no expected value
)

// TryStep is one operation or call of a formula as it was computed: the
// sub-expression in the subset's own form, its line and column in the
// file as a problem line counts them (an operator's own place, a call's
// name), and the value it gave, as a worked example writes it.
type TryStep struct {
	Expression string `json:"expression"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Value      string `json:"value"`
}

// TryInput is one input of an evaluation, as a worked example writes it.
type TryInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Evaluation is one evaluation of specarch try: a worked example, or the
// inputs given on the command line.
type Evaluation struct {
	Number   int        `json:"number,omitempty"` // the example's place, from 1; 0 for given inputs and a rule's example
	Rule     int        `json:"rule,omitempty"`   // a decision table's rule whose example this is, from 1
	Name     string     `json:"name"`             // the example's name, "" for one with none
	Outcome  string     `json:"outcome"`
	Inputs   []TryInput `json:"inputs"`
	Unread   []string   `json:"unread,omitempty"` // why an input or the expected value cannot be read
	Steps    []TryStep  `json:"steps"`
	Result   *string    `json:"result,omitempty"`
	Expected *string    `json:"expected,omitempty"`
	Error    string     `json:"error,omitempty"`
	// A mapping example's answer and each field sent or kept (ADR-207).
	Answer  []TryInput   `json:"answer,omitempty"`
	Fields  []TriedField `json:"fields,omitempty"`
	pointer string       // where a mapping example is, for TriedHere
}

// Tried is what specarch try found for one algorithm.
type Tried struct {
	Algorithm   string       `json:"algorithm"`
	Formula     string       `json:"formula"`             // "" for a decision table
	HitPolicy   string       `json:"hitPolicy,omitempty"` // a decision table's
	Rules       int          `json:"rules,omitempty"`     // how many rules a decision table has
	File        string       `json:"file"`
	Evaluations []Evaluation `json:"evaluations"`
}

// Try evaluates an algorithm of a specification with the validator's
// evaluator, on each worked example, on the one named example, or once on
// given inputs, a mapping written as an example's inputs are (ADR-195). It
// returns a refusal when the algorithm, its formula or the example is not
// there to evaluate. The specification is validated apart.
func Try(s *spec.Spec, algorithm, example string, given *yaml.Node) (*Tried, string) {
	if s.Root == nil {
		return nil, "the specification could not be read; the lines above say why"
	}
	algs := source.Child(s.Root, "algorithms")
	alg := source.Child(algs, algorithm)
	if alg == nil {
		var names []string
		for _, p := range source.Pairs(algs) {
			names = append(names, p.Key.Value)
		}
		sort.Strings(names)
		refusal := fmt.Sprintf("the specification has no algorithm %s; it has %s", algorithm, strings.Join(names, ", "))
		if len(names) == 0 {
			refusal = fmt.Sprintf("the specification has no algorithms, so no algorithm %s", algorithm)
		}
		if _, operations := tryNames(s.Root); len(operations) > 0 {
			refusal += fmt.Sprintf(", and no operation %s; the operations whose calls have mapping examples are %s", algorithm, strings.Join(operations, ", "))
		}
		return nil, refusal
	}
	c := &checker{file: s.RootFile, files: s.Files, root: s.Root}
	d := newDesign(s.Root)
	d.spec = s
	c.checkTimeZone(d)
	if source.Child(alg, "table") != nil {
		return c.tryTable(d, algorithm, alg, example, given)
	}
	formula := source.Child(alg, "formula")
	if formula == nil || !source.IsScalar(formula) {
		return nil, fmt.Sprintf("algorithm %s has no formula or table to evaluate; write one under formula or table", algorithm)
	}
	inputs := source.Pairs(source.Child(alg, "inputs"))
	fields := map[string]*yaml.Node{}
	env := expr.Env{}
	for _, p := range inputs {
		fields[p.Key.Value] = p.Value
		env[p.Key.Value] = d.fieldType(p.Value)
	}
	tree, errs := expr.Parse(formula.Value)
	var gives expr.Type
	var types map[*expr.Node]expr.Type
	if len(errs) == 0 {
		gives, types, errs = expr.CheckTypes(tree, env)
	}
	if len(errs) > 0 {
		return nil, fmt.Sprintf("the formula of algorithm %s does not check: %s; the validator's lines above say where", algorithm, errs[0].Message)
	}
	output := source.Child(alg, "output")
	if output == nil {
		return nil, fmt.Sprintf("algorithm %s has no output, so its result has no type to be read at; declare one under output", algorithm)
	}
	want := d.fieldType(output)
	t := &Tried{Algorithm: algorithm, Formula: formula.Value, File: c.fileOf(formula), Evaluations: []Evaluation{}}
	run := func(e Evaluation, values *yaml.Node, expected *yaml.Node) {
		e.Inputs, e.Steps = []TryInput{}, []TryStep{}
		vals := map[string]expr.Value{}
		for _, p := range source.Pairs(values) {
			field := fields[p.Key.Value]
			if field == nil {
				e.Unread = append(e.Unread, fmt.Sprintf("%s is not an input of algorithm %s%s", p.Key.Value, algorithm, suggest(p.Key.Value, fields)))
				continue
			}
			v, msg := d.value(p.Value, field)
			if msg != "" {
				e.Unread = append(e.Unread, fmt.Sprintf("input %s %s", p.Key.Value, msg))
				continue
			}
			vals[p.Key.Value] = v
		}
		for _, p := range inputs {
			v, ok := vals[p.Key.Value]
			switch {
			case ok:
				e.Inputs = append(e.Inputs, TryInput{Name: p.Key.Value, Value: expr.Show(v, env[p.Key.Value])})
			case source.Child(values, p.Key.Value) == nil:
				e.Unread = append(e.Unread, fmt.Sprintf("no value is given for input %s", p.Key.Value))
			}
		}
		var expectedValue expr.Value
		hasExpected := false
		if expected != nil {
			v, msg := d.value(expected, output)
			if msg != "" {
				e.Unread = append(e.Unread, "the expected value "+msg)
			} else {
				expectedValue, hasExpected = v, true
				text := expr.Show(v, want)
				e.Expected = &text
			}
		}
		if len(e.Unread) > 0 {
			e.Outcome = OutcomeUnread
			t.Evaluations = append(t.Evaluations, e)
			return
		}
		got, steps, err := expr.Trace(tree, vals, c.zoneOf())
		for _, st := range steps {
			line := exprLine(formula, st.Node.Line)
			e.Steps = append(e.Steps, TryStep{Expression: expr.Text(st.Node), Line: line, Column: c.exprColumn(formula, st.Node.Line, st.Node.Column), Value: expr.Show(st.Value, types[st.Node])})
		}
		switch {
		case err != nil:
			e.Outcome, e.Error = OutcomeError, err.Error()
		default:
			fits, why := expr.Fits(gives, want)
			shown := want
			if !fits {
				shown = gives
			}
			text := expr.Show(got, shown)
			e.Result = &text
			switch {
			case !fits:
				e.Outcome, e.Error = OutcomeFail, why
			case want.Kind == expr.Int && want.Bits == 32 && !expr.FitsInt(got.Num, 32):
				e.Outcome, e.Error = OutcomeFail, fmt.Sprintf("%s does not fit the output's int32", got)
			case expected == nil:
				e.Outcome = OutcomeComputed
			case hasExpected && expr.Equal(got, expectedValue):
				e.Outcome = OutcomePass
			default:
				e.Outcome = OutcomeFail
			}
		}
		t.Evaluations = append(t.Evaluations, e)
	}
	if given != nil {
		run(Evaluation{Name: "given inputs"}, given, nil)
		return t, ""
	}
	examples := source.Items(source.Child(alg, "examples"))
	if len(examples) == 0 {
		return nil, fmt.Sprintf("algorithm %s has no worked examples; give inputs with --inputs, or write examples under examples", algorithm)
	}
	var names []string
	for i, ex := range examples {
		name := source.Str(source.Child(ex, "name"))
		if name != "" {
			names = append(names, expr.Quote(name))
		}
		if example != "" && name != example {
			continue
		}
		run(Evaluation{Number: i + 1, Name: name}, source.Child(ex, "inputs"), source.Child(ex, "expected"))
	}
	if len(t.Evaluations) == 0 {
		if len(names) == 0 {
			return nil, fmt.Sprintf("algorithm %s has no example %s; none of its examples has a name", algorithm, expr.Quote(example))
		}
		return nil, fmt.Sprintf("algorithm %s has no example %s; its examples are %s", algorithm, expr.Quote(example), strings.Join(names, ", "))
	}
	return t, ""
}

// tryTable evaluates an algorithm's decision table on each rule's example
// and each worked example, or on the one named example, or once on given
// inputs; each step is a rule tested and whether it matches (ADR-199).
func (c *checker) tryTable(d *design, algorithm string, alg *yaml.Node, example string, given *yaml.Node) (*Tried, string) {
	tab := c.readTable(d, algorithm, alg, false)
	if !tab.readOK {
		return nil, fmt.Sprintf("the table of algorithm %s cannot be read; the validator's lines above say why", algorithm)
	}
	output := source.Child(alg, "output")
	if output == nil {
		return nil, fmt.Sprintf("algorithm %s has no output, so its result has no type to be read at; declare one under output", algorithm)
	}
	want := d.fieldType(output)
	tn := source.Child(alg, "table")
	t := &Tried{Algorithm: algorithm, File: c.fileOf(tn), HitPolicy: tab.policy, Rules: len(tab.rules), Evaluations: []Evaluation{}}
	fields := map[string]*yaml.Node{}
	for _, col := range tab.inputs {
		fields[col.name] = col.field
	}
	run := func(e Evaluation, vals map[string]expr.Value, expected *expr.Value) {
		e.Inputs, e.Steps = []TryInput{}, []TryStep{}
		for _, col := range tab.inputs {
			if v, ok := vals[col.name]; ok {
				e.Inputs = append(e.Inputs, TryInput{Name: col.name, Value: expr.Show(v, col.t)})
			}
		}
		if expected != nil {
			text := expr.Show(*expected, want)
			e.Expected = &text
		}
		if len(e.Unread) > 0 {
			e.Outcome = OutcomeUnread
			t.Evaluations = append(t.Evaluations, e)
			return
		}
		got, tested, _, err := tab.evaluate(vals)
		for i, hit := range tested {
			r := tab.rules[i]
			var parts []string
			for _, nc := range r.cells {
				col := tab.columns[slices.IndexFunc(tab.columns, func(col tableColumn) bool { return col.name == nc.name })]
				parts = append(parts, nc.name+" "+col.text(nc.cell))
			}
			text := fmt.Sprintf("rule %d: any inputs", i+1)
			if len(parts) > 0 {
				text = fmt.Sprintf("rule %d: %s", i+1, strings.Join(parts, ", "))
			}
			e.Steps = append(e.Steps, TryStep{Expression: text, Line: r.node.Line, Column: r.node.Column, Value: fmt.Sprint(hit)})
		}
		switch {
		case err != nil:
			e.Outcome, e.Error = OutcomeError, err.Error()
		default:
			text := expr.Show(got, want)
			e.Result = &text
			switch {
			case expected == nil:
				e.Outcome = OutcomeComputed
			case expr.Equal(got, *expected):
				e.Outcome = OutcomePass
			default:
				e.Outcome = OutcomeFail
			}
		}
		t.Evaluations = append(t.Evaluations, e)
	}
	read := func(e *Evaluation, values *yaml.Node) map[string]expr.Value {
		vals := map[string]expr.Value{}
		for _, p := range source.Pairs(values) {
			field := fields[p.Key.Value]
			if field == nil {
				e.Unread = append(e.Unread, fmt.Sprintf("%s is not an input of algorithm %s%s", p.Key.Value, algorithm, suggest(p.Key.Value, fields)))
				continue
			}
			v, msg := d.value(p.Value, field)
			if msg != "" {
				e.Unread = append(e.Unread, fmt.Sprintf("input %s %s", p.Key.Value, msg))
				continue
			}
			vals[p.Key.Value] = v
		}
		for _, col := range tab.inputs {
			if _, ok := vals[col.name]; !ok && source.Child(values, col.name) == nil {
				e.Unread = append(e.Unread, fmt.Sprintf("no value is given for input %s", col.name))
			}
		}
		return vals
	}
	if given != nil {
		e := Evaluation{Name: "given inputs"}
		run(e, read(&e, given), nil)
		return t, ""
	}
	if example == "" {
		for i := range tab.rules {
			e := Evaluation{Rule: i + 1}
			inputs, ok := tab.example(i)
			if !ok {
				e.Inputs, e.Steps = []TryInput{}, []TryStep{}
				e.Outcome, e.Error = OutcomeError, "no inputs reach this rule: the rules before it match every input it matches"
				switch k := tab.emptyColumn(i); {
				case k >= 0:
					e.Error = fmt.Sprintf("no inputs match this rule: its condition on %s holds for no value the input may take", tab.columns[k].name)
				case tab.policy == "unique":
					e.Error = "no inputs reach this rule alone: other rules match every input it matches"
				}
				t.Evaluations = append(t.Evaluations, e)
				continue
			}
			vals := map[string]expr.Value{}
			for k, col := range tab.columns {
				vals[col.name] = inputs[k]
			}
			then := tab.rules[i].then
			run(e, vals, &then)
		}
	}
	var names []string
	for i, ex := range source.Items(source.Child(alg, "examples")) {
		name := source.Str(source.Child(ex, "name"))
		if name != "" {
			names = append(names, expr.Quote(name))
		}
		if example != "" && name != example {
			continue
		}
		e := Evaluation{Number: i + 1, Name: name}
		vals := read(&e, source.Child(ex, "inputs"))
		var expected *expr.Value
		if n := source.Child(ex, "expected"); n != nil {
			if v, msg := d.value(n, output); msg != "" {
				e.Unread = append(e.Unread, "the expected value "+msg)
			} else {
				expected = &v
			}
		}
		run(e, vals, expected)
	}
	if example != "" && len(t.Evaluations) == 0 {
		if len(names) == 0 {
			return nil, fmt.Sprintf("algorithm %s has no example %s; none of its examples has a name", algorithm, expr.Quote(example))
		}
		return nil, fmt.Sprintf("algorithm %s has no example %s; its examples are %s", algorithm, expr.Quote(example), strings.Join(names, ", "))
	}
	return t, ""
}

// TriedHere reports whether a diagnostic is one that specarch try says in
// its own lines: a worked example it evaluated that does not hold or
// cannot be computed. An example it did not evaluate keeps its line.
func TriedHere(d Diagnostic, t Evaluated) bool {
	if d.Rule != RuleExampleMismatch && d.Rule != RuleExampleError {
		return false
	}
	for _, base := range t.pointers() {
		if d.Path == base || strings.HasPrefix(d.Path, base+"/") {
			return true
		}
	}
	return false
}
