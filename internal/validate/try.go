package validate

import (
	"fmt"
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
	Number   int        `json:"number,omitempty"` // the example's place, from 1; 0 for given inputs
	Name     string     `json:"name"`             // the example's name, "" for one with none
	Outcome  string     `json:"outcome"`
	Inputs   []TryInput `json:"inputs"`
	Unread   []string   `json:"unread,omitempty"` // why an input or the expected value cannot be read
	Steps    []TryStep  `json:"steps"`
	Result   *string    `json:"result,omitempty"`
	Expected *string    `json:"expected,omitempty"`
	Error    string     `json:"error,omitempty"`
}

// Tried is what specarch try found for one algorithm.
type Tried struct {
	Algorithm   string       `json:"algorithm"`
	Formula     string       `json:"formula"`
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
		if len(names) == 0 {
			return nil, fmt.Sprintf("the specification has no algorithms, so no algorithm %s", algorithm)
		}
		return nil, fmt.Sprintf("the specification has no algorithm %s; it has %s", algorithm, strings.Join(names, ", "))
	}
	c := &checker{file: s.RootFile, files: s.Files, root: s.Root}
	d := newDesign(s.Root)
	d.spec = s
	c.checkTimeZone(d)
	formula := source.Child(alg, "formula")
	if formula == nil || !source.IsScalar(formula) {
		return nil, fmt.Sprintf("algorithm %s has no formula to evaluate; write one under formula", algorithm)
	}
	inputs := source.Pairs(source.Child(alg, "inputs"))
	fields := map[string]*yaml.Node{}
	env := expr.Env{}
	for _, p := range inputs {
		fields[p.Key.Value] = p.Value
		env[p.Key.Value] = d.fieldType(p.Value)
	}
	tree, errs := expr.Parse(formula.Value)
	var types map[*expr.Node]expr.Type
	if len(errs) == 0 {
		_, types, errs = expr.CheckTypes(tree, env)
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
			text := expr.Show(got, want)
			e.Result = &text
			switch {
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

// TriedHere reports whether a diagnostic is one that specarch try says in
// its own lines for the algorithm it tries: a worked example of it that
// does not hold or cannot be computed.
func TriedHere(d Diagnostic, algorithm string) bool {
	if d.Rule != RuleExampleMismatch && d.Rule != RuleExampleError {
		return false
	}
	return strings.HasPrefix(d.Path, source.Pointer("algorithms", algorithm, "examples")+"/")
}
