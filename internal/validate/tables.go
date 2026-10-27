package validate

import (
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/source"
)

// A decision table (ADR-199): each rule is a box, one set of values per
// input, and overlap and gaps are found by intersecting and subtracting
// boxes. A set holds an enum's or a bool's members, an ordered type's
// intervals, or a string's values (or every string but them), and null.

type edge struct {
	v      *expr.Value // nil: unbounded
	closed bool
}

type interval struct{ lo, hi edge }

type valueSet struct {
	null    bool
	members []string   // enum, bool: in the column's order
	ivs     []interval // int, uint, decimal, double, date: sorted, apart
	except  bool       // string: every string but texts
	texts   []string   // string: sorted
}

type tableColumn struct {
	name   string
	field  *yaml.Node
	t      expr.Type
	domain valueSet
	ok     bool // a type a table tests
}

// condition is one comparison of a cell: op is "" for equality.
type condition struct {
	op string
	v  expr.Value
}

// cell is what a rule's condition on one input says, as written: a list of
// alternatives, each null or comparisons that all hold.
type cell struct {
	alternatives [][]condition // a nil slice in it is null
}

type tableRule struct {
	node      *yaml.Node
	when      map[string]valueSet // by input, as written (not cut to the domain)
	cells     []namedCell         // as written, in the when's order
	cellNodes map[string]*yaml.Node
	then      expr.Value
	box       []valueSet // by column, cut to the domain
}

type namedCell struct {
	name string
	cell cell
}

type decisionTable struct {
	policy  string
	inputs  []tableColumn // every input, in order
	columns []tableColumn // the inputs the table tests
	rules   []tableRule
	readOK  bool // every condition and output read
}

func ordered(k expr.Kind) bool {
	return k == expr.Int || k == expr.Uint || k == expr.Decimal || k == expr.Double || k == expr.Date
}

func cmpValue(a, b expr.Value) int {
	switch a.Kind {
	case expr.Double:
		switch {
		case a.Double < b.Double:
			return -1
		case a.Double > b.Double:
			return 1
		}
		return 0
	case expr.Date:
		return a.Time.Compare(b.Time)
	}
	return a.Num.Cmp(b.Num)
}

// step moves a value of a discrete type by one place: an int by 1, a
// decimal by its last digit, a date by a day; a double has no step.
func step(v expr.Value, t expr.Type, by int) (expr.Value, bool) {
	switch v.Kind {
	case expr.Int, expr.Uint:
		return expr.Value{Kind: v.Kind, Num: new(big.Rat).Add(v.Num, big.NewRat(int64(by), 1))}, true
	case expr.Decimal:
		unit := new(big.Rat).SetFrac(big.NewInt(int64(by)), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(t.Scale)), nil))
		return expr.Value{Kind: v.Kind, Num: new(big.Rat).Add(v.Num, unit)}, true
	case expr.Date:
		return expr.Value{Kind: v.Kind, Time: v.Time.AddDate(0, 0, by)}, true
	}
	return v, false
}

// normal closes an open edge of a discrete type on the next value in.
func normal(iv interval, t expr.Type) interval {
	if iv.lo.v != nil && !iv.lo.closed {
		if v, ok := step(*iv.lo.v, t, 1); ok {
			iv.lo = edge{&v, true}
		}
	}
	if iv.hi.v != nil && !iv.hi.closed {
		if v, ok := step(*iv.hi.v, t, -1); ok {
			iv.hi = edge{&v, true}
		}
	}
	return iv
}

func (iv interval) empty() bool {
	if iv.lo.v == nil || iv.hi.v == nil {
		return false
	}
	c := cmpValue(*iv.lo.v, *iv.hi.v)
	return c > 0 || c == 0 && !(iv.lo.closed && iv.hi.closed)
}

func lowerMax(a, b edge) edge {
	switch {
	case a.v == nil:
		return b
	case b.v == nil:
		return a
	}
	c := cmpValue(*a.v, *b.v)
	if c == 0 {
		return edge{a.v, a.closed && b.closed}
	}
	if c > 0 {
		return a
	}
	return b
}

func upperMin(a, b edge) edge {
	switch {
	case a.v == nil:
		return b
	case b.v == nil:
		return a
	}
	c := cmpValue(*a.v, *b.v)
	if c == 0 {
		return edge{a.v, a.closed && b.closed}
	}
	if c < 0 {
		return a
	}
	return b
}

// canonical sorts intervals and joins those that overlap or touch, a
// discrete type's when one ends a step before the next begins.
func canonical(ivs []interval, t expr.Type) []interval {
	var in []interval
	for _, iv := range ivs {
		if iv = normal(iv, t); !iv.empty() {
			in = append(in, iv)
		}
	}
	slices.SortStableFunc(in, func(a, b interval) int {
		switch {
		case a.lo.v == nil && b.lo.v == nil:
			return 0
		case a.lo.v == nil:
			return -1
		case b.lo.v == nil:
			return 1
		}
		if c := cmpValue(*a.lo.v, *b.lo.v); c != 0 {
			return c
		}
		switch {
		case a.lo.closed == b.lo.closed:
			return 0
		case a.lo.closed:
			return -1
		}
		return 1
	})
	var out []interval
	for _, iv := range in {
		if n := len(out); n > 0 && joins(out[n-1], iv, t) {
			last := &out[n-1]
			if iv.hi.v == nil || last.hi.v != nil && (cmpValue(*iv.hi.v, *last.hi.v) > 0 || cmpValue(*iv.hi.v, *last.hi.v) == 0 && iv.hi.closed) {
				last.hi = iv.hi
			}
			continue
		}
		out = append(out, iv)
	}
	return out
}

// joins tells whether b, which does not begin before a, overlaps or
// touches it.
func joins(a, b interval, t expr.Type) bool {
	if a.hi.v == nil || b.lo.v == nil {
		return true
	}
	c := cmpValue(*b.lo.v, *a.hi.v)
	if c < 0 || c == 0 && (a.hi.closed || b.lo.closed) {
		return true
	}
	if next, ok := step(*a.hi.v, t, 1); ok && a.hi.closed && b.lo.closed && cmpValue(next, *b.lo.v) == 0 {
		return true
	}
	return false
}

func (col *tableColumn) intersect(a, b valueSet) valueSet {
	out := valueSet{null: a.null && b.null}
	switch {
	case col.t.Kind == expr.Enum || col.t.Kind == expr.Bool:
		for _, m := range a.members {
			if slices.Contains(b.members, m) {
				out.members = append(out.members, m)
			}
		}
	case col.t.Kind == expr.String:
		switch {
		case !a.except && !b.except:
			out.texts = common(a.texts, b.texts, true)
		case !a.except:
			out.texts = common(a.texts, b.texts, false)
		case !b.except:
			out.texts = common(b.texts, a.texts, false)
		default:
			out.except, out.texts = true, sortedUnion(a.texts, b.texts)
		}
	default:
		var ivs []interval
		for _, x := range a.ivs {
			for _, y := range b.ivs {
				ivs = append(ivs, interval{lowerMax(x.lo, y.lo), upperMin(x.hi, y.hi)})
			}
		}
		out.ivs = canonical(ivs, col.t)
	}
	return out
}

// common keeps the texts of a that are (in) or are not (!in) in b.
func common(a, b []string, in bool) []string {
	var out []string
	for _, s := range a {
		if slices.Contains(b, s) == in {
			out = append(out, s)
		}
	}
	return out
}

func sortedUnion(a, b []string) []string {
	out := slices.Clone(a)
	for _, s := range b {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out
}

func (col *tableColumn) union(a, b valueSet) valueSet {
	return col.complement(col.intersect(col.complement(a), col.complement(b)))
}

// complement is every value of the column's type, and null, that a set
// does not hold; not cut to the domain.
func (col *tableColumn) complement(a valueSet) valueSet {
	out := valueSet{null: !a.null}
	switch {
	case col.t.Kind == expr.Enum || col.t.Kind == expr.Bool:
		for _, m := range universe(col.t) {
			if !slices.Contains(a.members, m) {
				out.members = append(out.members, m)
			}
		}
	case col.t.Kind == expr.String:
		out.except, out.texts = !a.except, a.texts
	default:
		ivs := []interval{{}}
		for _, x := range a.ivs {
			var next []interval
			for _, iv := range ivs {
				if x.lo.v != nil {
					next = append(next, interval{iv.lo, upperMin(iv.hi, edge{x.lo.v, !x.lo.closed})})
				}
				if x.hi.v != nil {
					next = append(next, interval{lowerMax(iv.lo, edge{x.hi.v, !x.hi.closed}), iv.hi})
				}
			}
			ivs = canonical(next, col.t)
		}
		out.ivs = ivs
	}
	return out
}

func universe(t expr.Type) []string {
	if t.Kind == expr.Bool {
		return []string{"false", "true"}
	}
	return t.Values
}

func (s valueSet) empty() bool {
	return !s.null && len(s.members) == 0 && len(s.ivs) == 0 && len(s.texts) == 0 && !s.except
}

func (col *tableColumn) equal(a, b valueSet) bool {
	return col.intersect(a, col.complement(b)).empty() && col.intersect(b, col.complement(a)).empty()
}

// contains tells whether a value is in a set.
func (col *tableColumn) contains(s valueSet, v expr.Value) bool {
	if v.Kind == expr.Null {
		return s.null
	}
	switch {
	case col.t.Kind == expr.Bool:
		return slices.Contains(s.members, fmt.Sprint(v.Bool))
	case col.t.Kind == expr.Enum:
		return slices.Contains(s.members, v.Text)
	case col.t.Kind == expr.String:
		return slices.Contains(s.texts, v.Text) != s.except
	}
	for _, iv := range s.ivs {
		if (iv.lo.v == nil || cmpValue(v, *iv.lo.v) > 0 || iv.lo.closed && cmpValue(v, *iv.lo.v) == 0) &&
			(iv.hi.v == nil || cmpValue(v, *iv.hi.v) < 0 || iv.hi.closed && cmpValue(v, *iv.hi.v) == 0) {
			return true
		}
	}
	return false
}

// pick is the value of a set a worked example takes: the lowest member,
// the first interval's closed end or a value inside it, and a non-null
// value before null.
func (col *tableColumn) pick(s valueSet) expr.Value {
	switch {
	case col.t.Kind == expr.Bool && len(s.members) > 0:
		return expr.Value{Kind: expr.Bool, Bool: s.members[0] == "true"}
	case col.t.Kind == expr.Enum && len(s.members) > 0:
		return expr.Value{Kind: expr.Enum, Text: s.members[0]}
	case col.t.Kind == expr.String && !s.except && len(s.texts) > 0:
		return expr.Value{Kind: expr.String, Text: s.texts[0]}
	case col.t.Kind == expr.String && s.except:
		text := "x"
		for slices.Contains(s.texts, text) {
			text += "x"
		}
		return expr.Value{Kind: expr.String, Text: text}
	case len(s.ivs) > 0:
		iv := s.ivs[0]
		switch {
		case iv.lo.v != nil && iv.lo.closed:
			return *iv.lo.v
		case iv.lo.v != nil && iv.hi.v != nil:
			return expr.Value{Kind: expr.Double, Double: (iv.lo.v.Double + iv.hi.v.Double) / 2}
		case iv.lo.v != nil:
			return expr.Value{Kind: expr.Double, Double: iv.lo.v.Double + 1}
		case iv.hi.v != nil && iv.hi.closed:
			return *iv.hi.v
		case iv.hi.v != nil:
			return expr.Value{Kind: expr.Double, Double: iv.hi.v.Double - 1}
		}
		return zero(col.t)
	}
	return expr.Value{Kind: expr.Null}
}

func zero(t expr.Type) expr.Value {
	switch t.Kind {
	case expr.Double:
		return expr.Value{Kind: expr.Double}
	case expr.Date:
		return expr.Value{Kind: expr.Date, Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}
	}
	return expr.Value{Kind: t.Kind, Num: new(big.Rat)}
}

// show writes a set as a condition: a value, a comparison or a list of
// them, in YAML, so that it can be pasted into a rule's when.
func (col *tableColumn) show(s valueSet) string {
	var parts []string
	if s.null {
		parts = append(parts, "null")
	}
	switch {
	case col.t.Kind == expr.Enum || col.t.Kind == expr.Bool:
		for _, m := range s.members {
			parts = append(parts, m)
		}
	case col.t.Kind == expr.String && !s.except:
		for _, t := range s.texts {
			parts = append(parts, expr.Quote(t))
		}
	case col.t.Kind == expr.String:
		var ands []string
		for _, t := range s.texts {
			ands = append(ands, "!= "+expr.Quote(t))
		}
		if len(ands) > 0 {
			parts = append(parts, yamlText(strings.Join(ands, " and ")))
		}
	default:
		for _, iv := range s.ivs {
			lo, hi := iv.lo, iv.hi
			switch {
			case lo.v != nil && hi.v != nil && lo.closed && hi.closed && cmpValue(*lo.v, *hi.v) == 0:
				parts = append(parts, expr.Show(*lo.v, col.t))
				continue
			}
			var ands []string
			if lo.v != nil && (col.domain.ivs == nil || !sameBound(lo, col.domain.ivs[0].lo)) {
				op := ">"
				if lo.closed {
					op = ">="
				}
				ands = append(ands, op+" "+expr.Show(*lo.v, col.t))
			}
			if hi.v != nil && (col.domain.ivs == nil || !sameBound(hi, col.domain.ivs[len(col.domain.ivs)-1].hi)) {
				op, v := "<", *hi.v
				if hi.closed {
					// A number's closed end is written as below the next
					// value, as a rule writes it: < "5.00", not <= "4.99".
					op = "<="
					if next, ok := step(v, col.t, 1); ok && v.Kind != expr.Date {
						op, v = "<", next
					}
				}
				ands = append(ands, op+" "+expr.Show(v, col.t))
			}
			if len(ands) == 0 {
				continue
			}
			parts = append(parts, yamlText(strings.Join(ands, " and ")))
		}
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func sameBound(a, b edge) bool {
	if a.v == nil || b.v == nil {
		return a.v == nil && b.v == nil
	}
	return a.closed == b.closed && cmpValue(*a.v, *b.v) == 0
}

// yamlText quotes a condition as YAML holds it: in double quotes, or in
// single quotes when it holds a double quote.
func yamlText(s string) string {
	if strings.Contains(s, `"`) {
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	return `"` + s + `"`
}

// domain is the values an input may take: its type's, within its minimum
// and maximum, an int32's range and a uint's zero, and null when it
// allows null.
func (d *design) tableDomain(field *yaml.Node, t expr.Type) valueSet {
	s := valueSet{null: t.Nullable}
	switch {
	case t.Kind == expr.Enum || t.Kind == expr.Bool:
		s.members = universe(t)
	case t.Kind == expr.String:
		s.except = true
	default:
		iv := interval{}
		limit := func(key string, lower, closed bool) {
			n := source.Child(field, key)
			if n == nil || n.Kind != yaml.ScalarNode {
				return
			}
			var v expr.Value
			switch t.Kind {
			case expr.Double:
				var f float64
				if n.Decode(&f) != nil {
					return
				}
				v = expr.Value{Kind: expr.Double, Double: f}
			case expr.Int, expr.Uint, expr.Decimal:
				r, ok := new(big.Rat).SetString(n.Value)
				if !ok {
					return
				}
				v = expr.Value{Kind: t.Kind, Num: r}
			default:
				return
			}
			b := edge{&v, closed}
			if lower {
				iv.lo = lowerMax(iv.lo, b)
			} else {
				iv.hi = upperMin(iv.hi, b)
			}
		}
		limit("minimum", true, true)
		limit("exclusiveMinimum", true, false)
		limit("maximum", false, true)
		limit("exclusiveMaximum", false, false)
		if t.Kind == expr.Uint {
			v := expr.Value{Kind: expr.Uint, Num: new(big.Rat)}
			iv.lo = lowerMax(iv.lo, edge{&v, true})
		}
		if t.Kind == expr.Int && t.Bits == 32 {
			lo := expr.Value{Kind: expr.Int, Num: big.NewRat(-1<<31, 1)}
			hi := expr.Value{Kind: expr.Int, Num: big.NewRat(1<<31-1, 1)}
			iv.lo, iv.hi = lowerMax(iv.lo, edge{&lo, true}), upperMin(iv.hi, edge{&hi, true})
		}
		s.ivs = canonical([]interval{iv}, t)
	}
	return s
}

// readTable reads an algorithm's table, reporting each condition and
// output that cannot be read at it.
func (c *checker) readTable(d *design, alg string, a *yaml.Node, report bool) *decisionTable {
	tn := source.Child(a, "table")
	tab := &decisionTable{policy: source.Str(source.Child(tn, "hitPolicy")), readOK: true}
	base := []string{"algorithms", alg, "table"}
	byName := map[string]int{}
	inputs := map[string]*yaml.Node{}
	for _, p := range source.Pairs(source.Child(a, "inputs")) {
		t := d.fieldType(p.Value)
		col := tableColumn{name: p.Key.Value, field: p.Value, t: t, ok: ordered(t.Kind) || t.Kind == expr.Enum || t.Kind == expr.Bool || t.Kind == expr.String}
		if col.ok {
			col.domain = d.tableDomain(p.Value, t)
			if col.domain.empty() {
				tab.readOK = false
				if report {
					c.add(p.Value, source.Pointer("algorithms", alg, "inputs", p.Key.Value), RuleDecisionTable,
						"input %s may take no value: its minimum and maximum leave none, so no rule of the table can match; correct them", p.Key.Value)
				}
			}
		}
		byName[p.Key.Value] = len(tab.columns)
		tab.columns = append(tab.columns, col)
		inputs[p.Key.Value] = p.Value
	}
	if tab.policy != "unique" && tab.policy != "first" {
		tab.readOK = false
	}
	output := source.Child(a, "output")
	fail := func(n *yaml.Node, ptr []string, format string, args ...any) {
		tab.readOK = false
		if report {
			c.add(n, source.Pointer(ptr...), RuleDecisionTable, format, args...)
		}
	}
	for i, rn := range source.Items(source.Child(tn, "rules")) {
		rb := append(slices.Clone(base), "rules", fmt.Sprint(i))
		r := tableRule{node: rn, when: map[string]valueSet{}, cellNodes: map[string]*yaml.Node{}}
		for _, p := range source.Pairs(source.Child(rn, "when")) {
			at := append(slices.Clone(rb), "when", p.Key.Value)
			k, known := byName[p.Key.Value]
			if !known {
				fail(p.Key, at, "%s is not an input of algorithm %s; remove it or correct its name%s", p.Key.Value, alg, suggest(p.Key.Value, inputs))
				continue
			}
			col := &tab.columns[k]
			if !col.ok {
				fail(p.Value, at, "input %s is %s, which a decision table does not test; a table tests int, uint, decimal, double, date, enum, bool and string inputs, so work this one out with a formula", p.Key.Value, col.t)
				continue
			}
			cl, set, msg := d.readCell(col, p.Value)
			if msg != "" {
				fail(p.Value, at, "the condition on %s of rule %d %s", p.Key.Value, i+1, msg)
				continue
			}
			r.when[p.Key.Value] = set
			r.cells = append(r.cells, namedCell{p.Key.Value, cl})
			r.cellNodes[p.Key.Value] = p.Value
		}
		if then := source.Child(rn, "then"); then != nil && output != nil {
			v, msg := d.value(then, output)
			if msg != "" {
				fail(then, append(slices.Clone(rb), "then"), "the output of rule %d %s", i+1, msg)
			}
			r.then = v
		} else {
			tab.readOK = false
		}
		tab.rules = append(tab.rules, r)
	}
	// An input of a type a table does not test, named by no rule, is
	// left out of the columns: every rule matches any value of it.
	tab.inputs = tab.columns
	tab.columns = nil
	for _, col := range tab.inputs {
		if col.ok {
			tab.columns = append(tab.columns, col)
		}
	}
	if !tab.readOK {
		return tab
	}
	for i := range tab.rules {
		r := &tab.rules[i]
		for k := range tab.columns {
			col := &tab.columns[k]
			set := col.domain
			if w, ok := r.when[col.name]; ok {
				set = col.intersect(set, w)
			}
			r.box = append(r.box, set)
		}
	}
	return tab
}

var tableOps = []string{"<=", ">=", "!=", "<", ">"}

// readCell reads a rule's condition on one input: a value, comparisons
// joined by and, null, or a list of these.
func (d *design) readCell(col *tableColumn, n *yaml.Node) (cell, valueSet, string) {
	items := []*yaml.Node{n}
	if n.Kind == yaml.SequenceNode {
		items = n.Content
	}
	var out cell
	var set valueSet
	for _, item := range items {
		if item.Kind != yaml.ScalarNode {
			return cell{}, valueSet{}, "is not a value, a comparison or null; write one, or a list of them"
		}
		alt, s, msg := d.readAlternative(col, item)
		if msg != "" {
			return cell{}, valueSet{}, msg
		}
		out.alternatives = append(out.alternatives, alt)
		set = col.union(set, s)
	}
	return out, set, ""
}

func (d *design) readAlternative(col *tableColumn, n *yaml.Node) ([]condition, valueSet, string) {
	if n.ShortTag() == "!!null" {
		if !col.t.Nullable {
			return nil, valueSet{}, "is null, but input " + col.name + " does not allow null; give a value"
		}
		return nil, valueSet{null: true}, ""
	}
	text := strings.TrimSpace(n.Value)
	isOp := n.ShortTag() == "!!str" && slices.ContainsFunc(tableOps, func(op string) bool { return strings.HasPrefix(text, op) })
	if !isOp {
		v, msg := d.value(n, col.field)
		if msg != "" {
			return nil, valueSet{}, msg
		}
		cond := condition{v: v}
		return []condition{cond}, col.conditionSet(cond), ""
	}
	var conds []condition
	set := col.complement(valueSet{null: true})
	rest := text
	for {
		op := ""
		for _, o := range tableOps {
			if strings.HasPrefix(rest, o) {
				op = o
				break
			}
		}
		if op == "" {
			return nil, valueSet{}, fmt.Sprintf("is %s, whose part %q does not begin with <, <=, >, >= or !=; join comparisons with and, such as \">= 3 and < 6\"", quoteYAML(text), rest)
		}
		rest = strings.TrimLeft(rest[len(op):], " ")
		operand, after := splitOperand(rest)
		if operand == "" {
			return nil, valueSet{}, fmt.Sprintf("is %s, whose %s has no value after it; write one, such as \"%s 6\"", quoteYAML(text), op, op)
		}
		var vn yaml.Node
		if err := yaml.Unmarshal([]byte(operand), &vn); err != nil || len(vn.Content) != 1 || vn.Content[0].Kind != yaml.ScalarNode {
			return nil, valueSet{}, fmt.Sprintf("is %s, whose value %s cannot be read; write it as a worked example writes it", quoteYAML(text), operand)
		}
		v, msg := d.value(vn.Content[0], col.field)
		if msg != "" {
			return nil, valueSet{}, fmt.Sprintf("is %s, whose value %s", quoteYAML(text), msg)
		}
		if v.Kind == expr.Null {
			return nil, valueSet{}, fmt.Sprintf("is %s, which compares with null; write null alone for an input that is null", quoteYAML(text))
		}
		if op != "!=" && !ordered(col.t.Kind) {
			return nil, valueSet{}, fmt.Sprintf("is %s, read as a comparison since it begins with %s, but %s has no order, so only a value or != is tested; write the values it may be as a list, and test a value that itself begins with <, > or != with a formula", quoteYAML(text), op, col.t)
		}
		cond := condition{op, v}
		conds = append(conds, cond)
		set = col.intersect(set, col.conditionSet(cond))
		if after == "" {
			break
		}
		rest = after
	}
	return conds, set, ""
}

// splitOperand cuts a comparison's value from what follows it: a quoted
// value runs to its closing quote, a plain one to " and ".
func splitOperand(s string) (string, string) {
	end := -1
	if s != "" && (s[0] == '"' || s[0] == '\'') {
		for i := 1; i < len(s); i++ {
			if s[i] == '\\' && s[0] == '"' {
				i++
				continue
			}
			if s[i] == s[0] {
				end = i + 1
				break
			}
		}
		if end < 0 {
			return s, ""
		}
	} else if i := strings.Index(s, " and "); i >= 0 {
		end = i
	} else {
		end = len(s)
	}
	operand, after := strings.TrimSpace(s[:end]), strings.TrimSpace(s[end:])
	if after == "" {
		return operand, ""
	}
	if !strings.HasPrefix(after, "and ") {
		return operand + " " + after, ""
	}
	return operand, strings.TrimSpace(after[len("and "):])
}

func quoteYAML(s string) string { return yamlText(s) }

// conditionSet is the values one comparison holds for, null left out.
func (col *tableColumn) conditionSet(c condition) valueSet {
	switch {
	case col.t.Kind == expr.Enum || col.t.Kind == expr.Bool:
		name := c.v.Text
		if col.t.Kind == expr.Bool {
			name = fmt.Sprint(c.v.Bool)
		}
		s := valueSet{members: []string{name}}
		if c.op == "!=" {
			s = col.complement(s)
			s.null = false
		}
		return s
	case col.t.Kind == expr.String:
		return valueSet{texts: []string{c.v.Text}, except: c.op == "!="}
	}
	v := c.v
	var ivs []interval
	switch c.op {
	case "":
		ivs = []interval{{edge{&v, true}, edge{&v, true}}}
	case "<":
		ivs = []interval{{hi: edge{&v, false}}}
	case "<=":
		ivs = []interval{{hi: edge{&v, true}}}
	case ">":
		ivs = []interval{{lo: edge{&v, false}}}
	case ">=":
		ivs = []interval{{lo: edge{&v, true}}}
	case "!=":
		ivs = []interval{{hi: edge{&v, false}}, {lo: edge{&v, false}}}
	}
	return valueSet{ivs: canonical(ivs, col.t)}
}

// text writes a cell as its rule holds it, for specarch try's steps.
func (col *tableColumn) text(cl cell) string {
	var alts []string
	for _, alt := range cl.alternatives {
		if alt == nil {
			alts = append(alts, "null")
			continue
		}
		var ands []string
		for _, c := range alt {
			if c.op == "" {
				ands = append(ands, expr.Show(c.v, col.t))
			} else {
				ands = append(ands, c.op+" "+expr.Show(c.v, col.t))
			}
		}
		alts = append(alts, strings.Join(ands, " and "))
	}
	if len(alts) == 1 {
		return alts[0]
	}
	return "[" + strings.Join(alts, ", ") + "]"
}

type box []valueSet

func (t *decisionTable) intersectBox(a, b box) (box, bool) {
	out := make(box, len(a))
	for k := range a {
		out[k] = t.columns[k].intersect(a[k], b[k])
		if out[k].empty() {
			return nil, false
		}
	}
	return out, true
}

// subtract is a box less another, as boxes apart: for each column, the
// part of a outside b there, within b on the columns before it.
func (t *decisionTable) subtract(a, b box) []box {
	if _, ok := t.intersectBox(a, b); !ok {
		return []box{a}
	}
	var out []box
	prefix := slices.Clone(a)
	for k := range a {
		col := &t.columns[k]
		piece := slices.Clone(prefix)
		piece[k] = col.intersect(a[k], col.complement(b[k]))
		if !piece[k].empty() {
			out = append(out, piece)
		}
		prefix[k] = col.intersect(a[k], b[k])
	}
	return out
}

func (t *decisionTable) subtractAll(from []box, b box) []box {
	var out []box
	for _, a := range from {
		out = append(out, t.subtract(a, b)...)
	}
	return out
}

// showBox writes a box as a when: each input whose values are not all its
// domain's.
func (t *decisionTable) showBox(b box) string {
	var parts []string
	for k, col := range t.columns {
		if !col.equal(b[k], col.domain) {
			parts = append(parts, col.name+": "+col.show(b[k]))
		}
	}
	if len(parts) == 0 {
		return "{ }"
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

// merge joins boxes that differ on one column only, so a gap reads as
// the fewest whens.
func (t *decisionTable) merge(boxes []box) []box {
	for {
		joined := false
		for i := 0; i < len(boxes) && !joined; i++ {
			for j := i + 1; j < len(boxes) && !joined; j++ {
				differ := -1
				for k := range t.columns {
					if !t.columns[k].equal(boxes[i][k], boxes[j][k]) {
						if differ >= 0 {
							differ = -2
							break
						}
						differ = k
					}
				}
				if differ == -2 {
					continue
				}
				if differ >= 0 {
					boxes[i][differ] = t.columns[differ].union(boxes[i][differ], boxes[j][differ])
				}
				boxes = slices.Delete(boxes, j, j+1)
				joined = true
			}
		}
		if !joined {
			return boxes
		}
	}
}

func (t *decisionTable) domainBox() box {
	b := make(box, len(t.columns))
	for k, col := range t.columns {
		b[k] = col.domain
	}
	return b
}

// reach is the inputs a rule gives the output for: under first, those no
// earlier rule matches; under unique, those no other rule matches.
func (t *decisionTable) reach(i int) []box {
	if t.emptyColumn(i) >= 0 {
		return nil
	}
	region := []box{t.rules[i].box}
	for j := range t.rules {
		if j == i || t.policy == "first" && j > i {
			continue
		}
		region = t.subtractAll(region, t.rules[j].box)
	}
	return region
}

// emptyColumn is the input on which a rule's condition holds for no value
// the input may take, or -1.
func (t *decisionTable) emptyColumn(i int) int {
	for k, s := range t.rules[i].box {
		if s.empty() {
			return k
		}
	}
	return -1
}

// example is a rule's worked example: inputs it gives the output for, by
// column, or false when there are none.
func (t *decisionTable) example(i int) ([]expr.Value, bool) {
	region := t.reach(i)
	if len(region) == 0 {
		return nil, false
	}
	var vals []expr.Value
	for k := range t.columns {
		vals = append(vals, t.columns[k].pick(region[0][k]))
	}
	return vals, true
}

// evaluate gives the table's output for inputs by name, the rules tested
// in order with whether each matched, and the rule that gave it, or an
// error.
func (t *decisionTable) evaluate(vals map[string]expr.Value) (expr.Value, []bool, int, error) {
	var tested []bool
	var matched []int
	for i, r := range t.rules {
		hit := true
		for k := range t.columns {
			col := &t.columns[k]
			if set, ok := r.when[col.name]; ok && !col.contains(set, vals[col.name]) {
				hit = false
				break
			}
		}
		tested = append(tested, hit)
		if hit {
			matched = append(matched, i)
			if t.policy == "first" {
				break
			}
		}
	}
	switch {
	case len(matched) == 0:
		return expr.Value{}, tested, -1, fmt.Errorf("no rule matches these inputs")
	case len(matched) > 1:
		return expr.Value{}, tested, -1, fmt.Errorf("rules %s match these inputs, and hit policy unique lets only one rule match", joinNumbers(matched))
	}
	return t.rules[matched[0]].then, tested, matched[0], nil
}

func joinNumbers(rules []int) string {
	var parts []string
	for _, i := range rules {
		parts = append(parts, fmt.Sprint(i+1))
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

// checkTable reads an algorithm's table, checks its rules for overlap and
// gaps by its hit policy, and checks its worked examples against it.
func (c *checker) checkTable(d *design, alg string, a *yaml.Node) {
	tab := c.readTable(d, alg, a, true)
	if !tab.readOK {
		return
	}
	rules := source.Child(source.Child(a, "table"), "rules")
	base := []string{"algorithms", alg, "table", "rules"}
	for j := range tab.rules {
		ptr := source.Pointer(append(slices.Clone(base), fmt.Sprint(j))...)
		if k := tab.emptyColumn(j); k >= 0 {
			name := tab.columns[k].name
			c.warn(tab.rules[j].cellNodes[name], source.Pointer(append(slices.Clone(base), fmt.Sprint(j), "when", name)...), RuleDecisionTable,
				"rule %d matches no inputs: its condition on %s holds for no value input %s may take, within its type, minimum and maximum; correct the condition, or remove the rule", j+1, name, name)
			continue
		}
		if tab.policy == "unique" {
			for i := 0; i < j; i++ {
				if both, ok := tab.intersectBox(tab.rules[i].box, tab.rules[j].box); ok {
					c.add(tab.rules[j].node, ptr, RuleTableOverlap,
						"rules %d and %d both match %s, and hit policy unique lets only one rule match any inputs; narrow one of them, or write hitPolicy first if the earlier rule is meant to win",
						i+1, j+1, tab.showBox(both))
				}
			}
			continue
		}
		if len(tab.reach(j)) == 0 {
			c.warn(tab.rules[j].node, ptr, RuleTableOverlap,
				"rule %d is never reached: the rules before it match every input it matches, and hit policy first takes the first rule that matches; remove it, or move it above the rules that take its inputs", j+1)
		}
	}
	left := []box{tab.domainBox()}
	for _, r := range tab.rules {
		left = tab.subtractAll(left, r.box)
	}
	if len(left) > 0 {
		left = tab.merge(left)
		var gaps []string
		for _, b := range left {
			gaps = append(gaps, tab.showBox(b))
		}
		advice := "add a rule for each"
		if tab.policy == "first" {
			advice += ", or a last rule with no when for every input the others leave"
		}
		c.add(rules, source.Pointer(base...), RuleTableGap, "no rule matches %s; %s", orList(gaps), advice)
	}
	output := source.Child(a, "output")
	if output == nil {
		return
	}
	want := d.fieldType(output)
	inputs := map[string]*yaml.Node{}
	for _, col := range tab.inputs {
		inputs[col.name] = col.field
	}
	for i, ex := range source.Items(source.Child(a, "examples")) {
		c.checkExampleWith(d, alg, i, ex, inputs, output, want, "the table", func(vals map[string]expr.Value) (expr.Value, error) {
			v, _, _, err := tab.evaluate(vals)
			return v, err
		})
	}
}

func orList(items []string) string {
	if len(items) == 1 {
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " or " + items[len(items)-1]
}
