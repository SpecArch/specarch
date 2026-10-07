package validate

import (
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/source"
)

// fieldType gives a field's type in the expression language: JSON Schema's
// type says how the value travels, its format what it is.
func (d *design) fieldType(f *yaml.Node) expr.Type {
	if ref := source.Str(source.Child(f, "$ref")); ref != "" {
		if strings.HasPrefix(ref, "#/enums/") {
			values, name, _ := d.enumValues(f)
			return expr.Type{Kind: expr.Enum, EnumName: name, Values: sortedKeys(values)}
		}
		return expr.Type{Kind: expr.Object}
	}
	t := expr.Type{}
	typ := source.Child(f, "type")
	base := source.Str(typ)
	for _, item := range source.Items(typ) {
		if item.Value == "null" {
			t.Nullable = true
		} else {
			base = item.Value
		}
	}
	format := source.Str(source.Child(f, "format"))
	if values, _, ok := d.enumValues(f); ok && base == "string" {
		t.Kind, t.Values = expr.Enum, sortedKeys(values)
		return t
	}
	switch format {
	case "int32":
		t.Kind, t.Bits = expr.Int, 32
	case "int64":
		t.Kind, t.Bits = expr.Int, 64
	case "uint64":
		t.Kind = expr.Uint
	case "double":
		t.Kind = expr.Double
	case "decimal":
		t.Kind, t.Scale = expr.Decimal, 0
		if sc := source.Child(f, "scale"); sc != nil {
			fmt.Sscan(sc.Value, &t.Scale)
		}
	case "date":
		t.Kind = expr.Date
	case "date-time":
		t.Kind = expr.Timestamp
	case "duration":
		t.Kind = expr.Duration
	case "time":
		t.Kind = expr.TimeOfDay
	case "byte", "binary":
		t.Kind = expr.Bytes
	default:
		switch base {
		case "integer":
			t.Kind, t.Bits = expr.Int, 64
		case "number":
			t.Kind = expr.Double
		case "boolean":
			t.Kind = expr.Bool
		case "array":
			t.Kind = expr.List
		case "object":
			t.Kind = expr.Object
		default:
			t.Kind = expr.String
		}
	}
	return t
}

func sortedKeys(m map[string]*yaml.Node) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// exprLine is the file line of a line inside an expression scalar.
func exprLine(n *yaml.Node, line int) int {
	if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return n.Line + line
	}
	return n.Line + line - 1
}

func (c *checker) exprErrors(n *yaml.Node, ptr, what string, errs []expr.Error) {
	for _, e := range errs {
		rule := RuleExpressionSyntax
		switch e.Kind {
		case expr.Name:
			rule = RuleExpressionName
		case expr.TypeMismatch:
			rule = RuleExpressionType
		}
		c.addFile(c.fileOf(n), exprLine(n, e.Line), ptr, rule, "%s, column %d: %s", what, e.Column, e.Message)
	}
}

func (c *checker) checkExpressions(d *design) {
	for _, e := range source.Pairs(source.Child(d.root, "entities")) {
		env := expr.Env{}
		for name, f := range fieldsOf(e.Value) {
			env[name] = d.fieldType(f)
		}
		for _, con := range source.Pairs(source.Child(e.Value, "constraints")) {
			n := source.Child(con.Value, "expression")
			if n == nil || !source.IsScalar(n) {
				continue
			}
			ptr := source.Pointer("entities", e.Key.Value, "constraints", con.Key.Value, "expression")
			tree, errs := expr.Parse(n.Value)
			if len(errs) > 0 {
				c.exprErrors(n, ptr, "the check", errs)
				continue
			}
			t, errs := expr.Check(tree, env)
			c.exprErrors(n, ptr, "the check", errs)
			if len(errs) == 0 && (t.Kind != expr.Bool || t.Nullable) {
				c.addFile(c.fileOf(n), exprLine(n, 1), ptr, RuleExpressionType, "the check gives %s, but a check must give true or false; compare the values with ==, <, > or similar", t)
			}
		}
	}
	for _, a := range source.Pairs(source.Child(d.root, "algorithms")) {
		c.checkAlgorithm(d, a.Key.Value, a.Value)
	}
}

func (c *checker) checkAlgorithm(d *design, name string, alg *yaml.Node) {
	n := source.Child(alg, "formula")
	if n == nil || !source.IsScalar(n) {
		return
	}
	ptr := source.Pointer("algorithms", name, "formula")
	inputs := map[string]*yaml.Node{}
	env := expr.Env{}
	for _, p := range source.Pairs(source.Child(alg, "inputs")) {
		inputs[p.Key.Value] = p.Value
		env[p.Key.Value] = d.fieldType(p.Value)
	}
	tree, errs := expr.Parse(n.Value)
	if len(errs) > 0 {
		c.exprErrors(n, ptr, "the formula", errs)
		return
	}
	t, errs := expr.Check(tree, env)
	if len(errs) > 0 {
		c.exprErrors(n, ptr, "the formula", errs)
		return
	}
	output := source.Child(alg, "output")
	if output == nil {
		return
	}
	want := d.fieldType(output)
	if ok, why := expr.Fits(t, want); !ok {
		c.addFile(c.fileOf(n), exprLine(n, 1), ptr, RuleExpressionType, "%s", why)
		return
	}
	for i, ex := range source.Items(source.Child(alg, "examples")) {
		c.checkExample(d, name, i, ex, tree, inputs, output, want)
	}
}

func (c *checker) checkExample(d *design, alg string, i int, ex *yaml.Node, tree *expr.Node, inputs map[string]*yaml.Node, output *yaml.Node, want expr.Type) {
	base := []string{"algorithms", alg, "examples", fmt.Sprint(i)}
	label := source.Str(source.Child(ex, "name"))
	if label == "" {
		label = fmt.Sprintf("example %d", i+1)
	}
	given := source.Child(ex, "inputs")
	vals := map[string]expr.Value{}
	ok := true
	for _, p := range source.Pairs(given) {
		field := inputs[p.Key.Value]
		if field == nil {
			c.add(p.Key, source.Pointer(append(base, "inputs", p.Key.Value)...), RuleExampleInput,
				"%s is not an input of algorithm %s; remove it or correct its name%s", p.Key.Value, alg, suggest(p.Key.Value, inputs))
			ok = false
			continue
		}
		v, msg := d.value(p.Value, field)
		if msg != "" {
			c.add(p.Value, source.Pointer(append(base, "inputs", p.Key.Value)...), RuleExampleInput, "input %s of %q %s", p.Key.Value, label, msg)
			ok = false
			continue
		}
		vals[p.Key.Value] = v
	}
	for _, in := range sortedKeys(inputs) {
		if source.Child(given, in) == nil && given != nil {
			c.add(given, source.Pointer(append(base, "inputs")...), RuleExampleInput, "%q gives no value for input %s; add it", label, in)
			ok = false
		}
	}
	expNode := source.Child(ex, "expected")
	if expNode == nil {
		return
	}
	expected, msg := d.value(expNode, output)
	if msg != "" {
		c.add(expNode, source.Pointer(append(base, "expected")...), RuleExampleExpected, "the expected value of %q %s", label, msg)
		return
	}
	if !ok {
		return
	}
	got, err := expr.Eval(tree, vals)
	if err != nil {
		c.add(ex, source.Pointer(base...), RuleExampleError, "the formula cannot be computed for %q: %v; correct the inputs or the formula", label, err)
		return
	}
	if want.Kind == expr.Int && want.Bits == 32 && !expr.FitsInt(got.Num, 32) {
		c.add(expNode, source.Pointer(append(base, "expected")...), RuleExampleMismatch,
			"the formula gives %s for %q, which does not fit the output's int32; correct the formula or widen the output", got, label)
		return
	}
	if !expr.Equal(got, expected) {
		c.add(expNode, source.Pointer(append(base, "expected")...), RuleExampleMismatch,
			"the formula gives %s for %q, but the example expects %s; correct the example or the formula", show(got, want), label, show(expected, want))
	}
}

var decimalText = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

// value reads a worked example's value as the field declares it. It returns
// the end of a plain sentence when the value does not fit.
func (d *design) value(n *yaml.Node, field *yaml.Node) (expr.Value, string) {
	t := d.fieldType(field)
	tag := n.ShortTag()
	if tag == "!!null" {
		if t.Nullable {
			return expr.Value{Kind: expr.Null}, ""
		}
		return expr.Value{}, "is null, but the field does not allow null; give a value"
	}
	wireString := source.Str(source.Child(field, "type")) == "string"
	switch t.Kind {
	case expr.Int, expr.Uint:
		if wireString && tag != "!!str" {
			return expr.Value{}, fmt.Sprintf("travels as a string, so write it in quotes: \"%s\"", n.Value)
		}
		if !wireString && tag != "!!int" {
			return expr.Value{}, fmt.Sprintf("must be a whole number, and is %s", n.Value)
		}
		i, ok := new(big.Int).SetString(n.Value, 10)
		if !ok {
			return expr.Value{}, fmt.Sprintf("must be a whole number, and is %s", n.Value)
		}
		r := new(big.Rat).SetInt(i)
		if t.Kind == expr.Int && !expr.FitsInt(r, t.Bits) {
			return expr.Value{}, fmt.Sprintf("is %s, which does not fit in an int%d", n.Value, t.Bits)
		}
		if t.Kind == expr.Uint && i.Sign() < 0 {
			return expr.Value{}, fmt.Sprintf("is %s, but a uint cannot be negative", n.Value)
		}
		return expr.Value{Kind: t.Kind, Num: r}, ""
	case expr.Double:
		if tag != "!!int" && tag != "!!float" {
			return expr.Value{}, fmt.Sprintf("must be a number, and is %s", n.Value)
		}
		var f float64
		if err := n.Decode(&f); err != nil {
			return expr.Value{}, fmt.Sprintf("must be a number, and is %s", n.Value)
		}
		return expr.Value{Kind: expr.Double, Double: f}, ""
	case expr.Decimal:
		if tag != "!!str" || !decimalText.MatchString(n.Value) {
			return expr.Value{}, fmt.Sprintf("must be a decimal in quotes, such as \"%s\", and is %s; quote it", exampleDecimal(t.Scale), n.Value)
		}
		if _, frac, found := strings.Cut(n.Value, "."); found && len(frac) > t.Scale {
			return expr.Value{}, fmt.Sprintf("has %d decimal places, but the field's scale is %d; round it", len(frac), t.Scale)
		}
		r, _ := new(big.Rat).SetString(n.Value)
		return expr.Value{Kind: expr.Decimal, Num: r}, ""
	case expr.Bool:
		if tag != "!!bool" {
			return expr.Value{}, fmt.Sprintf("must be true or false, and is %s", n.Value)
		}
		return expr.Value{Kind: expr.Bool, Bool: n.Value == "true"}, ""
	case expr.Enum:
		for _, v := range t.Values {
			if v == n.Value && tag == "!!str" {
				return expr.Value{Kind: expr.Enum, Text: n.Value}, ""
			}
		}
		return expr.Value{}, fmt.Sprintf("is %s, which is not one of %s", n.Value, strings.Join(t.Values, ", "))
	case expr.Date, expr.Timestamp, expr.TimeOfDay:
		layout := map[expr.Kind]string{expr.Date: "2006-01-02", expr.Timestamp: time.RFC3339Nano, expr.TimeOfDay: "15:04:05"}[t.Kind]
		tm, err := time.Parse(layout, n.Value)
		if err != nil || tag != "!!str" {
			return expr.Value{}, fmt.Sprintf("must be a %s in quotes written as %s, and is %s", t.Kind, layoutName(t.Kind), n.Value)
		}
		return expr.Value{Kind: t.Kind, Time: tm}, ""
	case expr.Duration:
		dur, ok := parseDuration(n.Value)
		if !ok || tag != "!!str" {
			return expr.Value{}, fmt.Sprintf("must be an ISO 8601 duration in quotes, such as \"PT2H\", and is %s", n.Value)
		}
		return expr.Value{Kind: expr.Duration, Dur: dur}, ""
	case expr.Bytes:
		b, err := expr.DecodeBytes(n.Value)
		if err != nil || tag != "!!str" {
			return expr.Value{}, fmt.Sprintf("must be base64 in quotes, and is %s", n.Value)
		}
		return expr.Value{Kind: expr.Bytes, Text: b}, ""
	case expr.String:
		if tag != "!!str" {
			return expr.Value{}, fmt.Sprintf("is a string, so write it in quotes: \"%s\"", n.Value)
		}
		return expr.Value{Kind: expr.String, Text: n.Value}, ""
	}
	return expr.Value{}, fmt.Sprintf("is %s, which worked examples cannot hold; use a single value", t)
}

var isoDuration = regexp.MustCompile(`^P(?:([0-9]+)D)?(?:T(?:([0-9]+)H)?(?:([0-9]+)M)?(?:([0-9]+)S)?)?$`)

// parseDuration reads the day-and-time part of ISO 8601: P1DT2H3M4S.
func parseDuration(s string) (time.Duration, bool) {
	m := isoDuration.FindStringSubmatch(s)
	if m == nil || s == "P" || s == "PT" {
		return 0, false
	}
	units := []time.Duration{24 * time.Hour, time.Hour, time.Minute, time.Second}
	var d time.Duration
	for i, u := range units {
		if m[i+1] != "" {
			var v int64
			fmt.Sscan(m[i+1], &v)
			d += time.Duration(v) * u
		}
	}
	return d, true
}

func layoutName(k expr.Kind) string {
	switch k {
	case expr.Date:
		return "YYYY-MM-DD"
	case expr.Timestamp:
		return "YYYY-MM-DDThh:mm:ssZ"
	}
	return "hh:mm:ss"
}

func exampleDecimal(scale int) string {
	if scale <= 0 {
		return "10"
	}
	return "10." + strings.Repeat("0", scale)
}

// show writes a value as a worked example would, a decimal with the
// output's scale.
func show(v expr.Value, t expr.Type) string {
	if v.Kind == expr.Decimal && t.Scale >= 0 {
		return fmt.Sprintf("%q", v.Num.FloatString(t.Scale))
	}
	return v.String()
}
