package expr

import (
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Kind is the type of a value, as CEL names it, plus SpecArch's decimal,
// date and enum.
type Kind string

const (
	Int       Kind = "int"
	Uint      Kind = "uint"
	Double    Kind = "double"
	Decimal   Kind = "decimal"
	String    Kind = "string"
	Bool      Kind = "bool"
	Bytes     Kind = "bytes"
	Date      Kind = "date"
	Timestamp Kind = "timestamp"
	Duration  Kind = "duration"
	TimeOfDay Kind = "time"
	Enum      Kind = "enum"
	List      Kind = "list"
	Object    Kind = "object"
	Null      Kind = "null"
	bad       Kind = "bad" // a part that already has an error
)

// UnknownScale marks a decimal whose scale is not fixed, the result of /.
const UnknownScale = -1

// Type is the type of a name or of an expression.
type Type struct {
	Kind     Kind
	Scale    int      // for Decimal: digits after the point, or UnknownScale
	Bits     int      // for Int: 32 or 64, the declared width
	EnumName string   // for Enum
	Values   []string // for Enum
	Nullable bool
}

func (t Type) String() string {
	var s string
	switch t.Kind {
	case Decimal:
		if t.Scale == UnknownScale {
			s = "a decimal of no fixed scale"
		} else {
			s = fmt.Sprintf("a decimal of scale %d", t.Scale)
		}
	case Enum:
		if t.EnumName != "" {
			s = "a value of " + t.EnumName
		} else {
			s = "an enum value"
		}
	case Int:
		s = "an int"
	case Uint:
		s = "a uint"
	case Null:
		s = "null"
	default:
		s = "a " + string(t.Kind)
	}
	if t.Nullable {
		s += " or null"
	}
	return s
}

func numeric(k Kind) bool { return k == Int || k == Uint || k == Double || k == Decimal }

func orderable(k Kind) bool {
	return numeric(k) || k == String || k == Date || k == Timestamp || k == Duration || k == TimeOfDay
}

// Env holds the names an expression may use and their types.
type Env map[string]Type

// Check type-checks a parsed expression and returns its type.
func Check(n *Node, env Env) (Type, []Error) {
	c := &checker{env: env}
	t := c.check(n)
	return t, c.errs
}

type checker struct {
	env  Env
	errs []Error
}

var badType = Type{Kind: bad}

func (c *checker) fail(n *Node, kind ErrorKind, format string, args ...any) Type {
	c.errs = append(c.errs, Error{Line: n.Line, Column: n.Column, Kind: kind, Message: fmt.Sprintf(format, args...)})
	return badType
}

func (c *checker) check(n *Node) Type {
	switch n.Op {
	case OpInt:
		return Type{Kind: Int, Bits: 64}
	case OpUint:
		return Type{Kind: Uint}
	case OpDouble:
		return Type{Kind: Double}
	case OpText:
		return Type{Kind: String}
	case OpBool:
		return Type{Kind: Bool}
	case OpNull:
		return Type{Kind: Null}
	case OpName:
		t, ok := c.env[n.Text]
		if !ok {
			return c.fail(n, Name, "%s is not a name this expression can use; %s", n.Text, c.names(n.Text))
		}
		return t
	case OpNeg:
		t := c.check(n.Args[0])
		if t.Kind == bad {
			return t
		}
		if (t.Kind == Int || t.Kind == Double || t.Kind == Decimal) && !t.Nullable {
			return t
		}
		return c.fail(n, TypeMismatch, "- needs an int, a double or a decimal, and this is %s", t)
	case OpNot:
		t := c.check(n.Args[0])
		if t.Kind == bad || t.Kind == Bool && !t.Nullable {
			return t
		}
		return c.fail(n, TypeMismatch, "! needs a bool, and this is %s", t)
	case OpCond:
		return c.cond(n)
	case OpCall:
		return c.call(n)
	case "+", "-", "*", "/":
		return c.arithmetic(n)
	case "&&", "||":
		l := c.check(n.Args[0])
		// x == null || x > y, and x != null && x > y: on the right, x is
		// known not to be null.
		var r Type
		if name := nullTest(n.Args[0], n.Op == "&&"); name != "" {
			r = c.withNonNull(name, n.Args[1])
		} else {
			r = c.check(n.Args[1])
		}
		if l.Kind == bad || r.Kind == bad {
			return badType
		}
		for _, t := range []Type{l, r} {
			if t.Kind != Bool || t.Nullable {
				return c.fail(n, TypeMismatch, "%s joins two bools, and one side is %s", n.Op, t)
			}
		}
		return Type{Kind: Bool}
	case "==", "!=":
		return c.equality(n)
	case "<", "<=", ">", ">=":
		l, r := c.check(n.Args[0]), c.check(n.Args[1])
		if l.Kind == bad || r.Kind == bad {
			return badType
		}
		if l.Kind == Bool && isComparison(n.Args[0].Op) {
			return c.fail(n, TypeMismatch, "comparisons do not chain; write a < b && b < c")
		}
		if l.Nullable || r.Nullable {
			return c.fail(n, TypeMismatch, "%s cannot order a value that may be null; check it with == null first", n.Op)
		}
		if !orderable(l.Kind) || l.Kind != r.Kind {
			return c.fail(n, TypeMismatch, "%s compares two values of one type, and this is %s %s %s; %s", n.Op, l, n.Op, r, conversionHint(l, r))
		}
		return Type{Kind: Bool}
	}
	return c.fail(n, Syntax, "this is not part of SpecArch expressions")
}

func isComparison(op string) bool { return op == "<" || op == "<=" || op == ">" || op == ">=" }

// conversionHint says how to make two types meet.
func conversionHint(l, r Type) string {
	switch {
	case (l.Kind == Date && r.Kind == Timestamp) || (l.Kind == Timestamp && r.Kind == Date):
		return "take the date of the timestamp with date(x)"
	case l.Kind == Int && r.Kind == Decimal, l.Kind == Decimal && r.Kind == Int:
		return "convert the int with decimal(x, scale)"
	case l.Kind == Int && r.Kind == Double, l.Kind == Double && r.Kind == Int:
		return "convert the int with double(x)"
	case l.Kind == Int && r.Kind == Uint, l.Kind == Uint && r.Kind == Int:
		return "convert one side with int(x) or uint(x)"
	case l.Kind == Decimal && r.Kind == Double, l.Kind == Double && r.Kind == Decimal:
		return "a double cannot become a decimal; keep the value a decimal from the start, or convert the decimal with double(x)"
	}
	return "write a conversion so both sides have one type"
}

func (c *checker) arithmetic(n *Node) Type {
	l, r := c.check(n.Args[0]), c.check(n.Args[1])
	if l.Kind == bad || r.Kind == bad {
		return badType
	}
	if l.Nullable || r.Nullable {
		return c.fail(n, TypeMismatch, "%s cannot take a value that may be null; check it with == null first", n.Op)
	}
	if !numeric(l.Kind) || l.Kind != r.Kind {
		if numeric(l.Kind) && numeric(r.Kind) {
			return c.fail(n, TypeMismatch, "%s needs two numbers of one type, and this is %s %s %s; %s", n.Op, l, n.Op, r, conversionHint(l, r))
		}
		return c.fail(n, TypeMismatch, "%s works on numbers, and this is %s %s %s", n.Op, l, n.Op, r)
	}
	if l.Kind == Uint && n.Op == "-" {
		return Type{Kind: Uint}
	}
	if l.Kind != Decimal {
		return Type{Kind: l.Kind, Bits: 64}
	}
	scale := UnknownScale
	if l.Scale != UnknownScale && r.Scale != UnknownScale {
		switch n.Op {
		case "+", "-":
			scale = max(l.Scale, r.Scale)
		case "*":
			scale = l.Scale + r.Scale
		}
	}
	return Type{Kind: Decimal, Scale: scale}
}

func (c *checker) names(near string) string {
	var names []string
	for n := range c.env {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "there are none"
	}
	for _, n := range names {
		if strings.EqualFold(n, near) {
			return "did you mean " + n + "?"
		}
	}
	return "use one of " + strings.Join(names, ", ")
}

func (c *checker) equality(n *Node) Type {
	l, r := c.check(n.Args[0]), c.check(n.Args[1])
	if l.Kind == bad || r.Kind == bad {
		return badType
	}
	if l.Kind == Null || r.Kind == Null {
		other := l
		if l.Kind == Null {
			other = r
		}
		if other.Kind != Null && !other.Nullable {
			return c.fail(n, TypeMismatch, "this side is never null, so comparing it with null is always %v; remove the comparison or allow null on the field", n.Op == "!=")
		}
		return Type{Kind: Bool}
	}
	if l.Kind == Enum || r.Kind == Enum {
		e, other, otherNode := l, r, n.Args[1]
		if l.Kind != Enum {
			e, other, otherNode = r, l, n.Args[0]
		}
		switch {
		case other.Kind == Enum && other.EnumName == e.EnumName:
			return Type{Kind: Bool}
		case other.Kind == String && otherNode.Op == OpText:
			for _, v := range e.Values {
				if v == otherNode.Text {
					return Type{Kind: Bool}
				}
			}
			return c.fail(otherNode, TypeMismatch, "%q is not %s; use one of %s", otherNode.Text, e, strings.Join(e.Values, ", "))
		}
		return c.fail(n, TypeMismatch, "%s compares %s with one of its values in quotes, and the other side is %s", n.Op, e, other)
	}
	if l.Kind != r.Kind || l.Kind == List || l.Kind == Object {
		return c.fail(n, TypeMismatch, "%s compares two values of one type, and this is %s %s %s; %s", n.Op, l, n.Op, r, conversionHint(l, r))
	}
	return Type{Kind: Bool}
}

func (c *checker) cond(n *Node) Type {
	cond := c.check(n.Args[0])
	var a, b Type
	if name := nullTest(n.Args[0], true); name != "" {
		a, b = c.withNonNull(name, n.Args[1]), c.check(n.Args[2])
	} else if name := nullTest(n.Args[0], false); name != "" {
		a, b = c.check(n.Args[1]), c.withNonNull(name, n.Args[2])
	} else {
		a, b = c.check(n.Args[1]), c.check(n.Args[2])
	}
	if cond.Kind != bad && (cond.Kind != Bool || cond.Nullable) {
		return c.fail(n.Args[0], TypeMismatch, "the condition before ? must be a bool, and this is %s", cond)
	}
	if cond.Kind == bad || a.Kind == bad || b.Kind == bad {
		return badType
	}
	switch {
	case a.Kind == Null:
		b.Nullable = true
		return b
	case b.Kind == Null:
		a.Nullable = true
		return a
	case a.Kind != b.Kind || (a.Kind == Enum && a.EnumName != b.EnumName):
		return c.fail(n, TypeMismatch, "both sides of : must have one type, and this is %s : %s; %s", a, b, conversionHint(a, b))
	}
	if a.Kind == Decimal {
		a.Scale = maxScale(a.Scale, b.Scale)
	}
	a.Nullable = a.Nullable || b.Nullable
	return a
}

func maxScale(a, b int) int {
	if a == UnknownScale || b == UnknownScale {
		return UnknownScale
	}
	return max(a, b)
}

var (
	digits      = regexp.MustCompile(`^-?[0-9]+$`)
	decimalText = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)
)

// isRounding reports whether a node is a call that states its rounding.
func isRounding(n *Node) bool {
	return n.Op == OpCall && (n.Text == "round" || n.Text == "floor" || n.Text == "ceil")
}

func (c *checker) call(n *Node) Type {
	args := make([]Type, len(n.Args))
	for i, a := range n.Args {
		args[i] = c.check(a)
		if args[i].Kind == bad {
			return badType
		}
	}
	count := func(want int) bool {
		if len(args) != want {
			c.fail(n, TypeMismatch, "%s takes %d %s, and here it has %d", n.Text, want, plural(want, "argument"), len(args))
			return false
		}
		if want > 0 && args[0].Nullable {
			c.fail(n, TypeMismatch, "%s cannot take a value that may be null; check it with == null first", n.Text)
			return false
		}
		return true
	}
	a0 := Type{}
	if len(args) > 0 {
		a0 = args[0]
	}
	switch n.Text {
	case "int":
		if !count(1) {
			return badType
		}
		switch {
		case a0.Kind == Int || a0.Kind == Uint:
			return Type{Kind: Int, Bits: 64}
		case a0.Kind == String:
			if n.Args[0].Op == OpText && !digits.MatchString(n.Args[0].Text) {
				return c.fail(n.Args[0], TypeMismatch, "%q is not a whole number", n.Args[0].Text)
			}
			return Type{Kind: Int, Bits: 64}
		case (a0.Kind == Decimal || a0.Kind == Double) && isRounding(n.Args[0]):
			return Type{Kind: Int, Bits: 64}
		case a0.Kind == Decimal || a0.Kind == Double:
			return c.fail(n, TypeMismatch, "int(x) of %s would drop the fraction without saying how; write int(round(x, 0)), int(floor(x)) or int(ceil(x))", a0)
		}
		return c.fail(n, TypeMismatch, "int(x) takes a uint, a string of digits or a rounded number, and this is %s", a0)
	case "uint":
		if !count(1) {
			return badType
		}
		if a0.Kind == Int || a0.Kind == Uint || a0.Kind == String {
			return Type{Kind: Uint}
		}
		return c.fail(n, TypeMismatch, "uint(x) takes an int or a string of digits, and this is %s", a0)
	case "double":
		if !count(1) {
			return badType
		}
		if numeric(a0.Kind) || a0.Kind == String {
			return Type{Kind: Double}
		}
		return c.fail(n, TypeMismatch, "double(x) takes a number or a string, and this is %s", a0)
	case "decimal":
		return c.decimal(n, args)
	case "string":
		if !count(1) {
			return badType
		}
		if a0.Kind == List || a0.Kind == Object {
			return c.fail(n, TypeMismatch, "string(x) takes a single value, and this is %s", a0)
		}
		return Type{Kind: String}
	case "date":
		if !count(1) {
			return badType
		}
		switch {
		case a0.Kind == Timestamp || a0.Kind == Date:
			return Type{Kind: Date}
		case n.Args[0].Op == OpText:
			if _, err := time.Parse("2006-01-02", n.Args[0].Text); err != nil {
				return c.fail(n.Args[0], TypeMismatch, "%q is not a date; write it as YYYY-MM-DD", n.Args[0].Text)
			}
			return Type{Kind: Date}
		}
		return c.fail(n, TypeMismatch, "date(x) takes a timestamp, a date or a quoted date, and this is %s", a0)
	case "timestamp":
		if !count(1) {
			return badType
		}
		if n.Args[0].Op == OpText {
			if _, err := time.Parse(time.RFC3339Nano, n.Args[0].Text); err != nil {
				return c.fail(n.Args[0], TypeMismatch, "%q is not a timestamp; write it as RFC 3339, such as 2026-10-07T09:30:00Z", n.Args[0].Text)
			}
			return Type{Kind: Timestamp}
		}
		if a0.Kind == Timestamp || a0.Kind == String {
			return Type{Kind: Timestamp}
		}
		return c.fail(n, TypeMismatch, "timestamp(x) takes a quoted RFC 3339 instant, and this is %s", a0)
	case "size":
		if !count(1) {
			return badType
		}
		if a0.Kind == String || a0.Kind == Bytes || a0.Kind == List {
			return Type{Kind: Int, Bits: 64}
		}
		return c.fail(n, TypeMismatch, "size(x) takes a string, bytes or a list, and this is %s", a0)
	case "round":
		if !count(2) {
			return badType
		}
		if args[1].Kind != Int || args[1].Nullable {
			return c.fail(n.Args[1], TypeMismatch, "the places of round(x, places) must be an int, and this is %s", args[1])
		}
		switch a0.Kind {
		case Decimal:
			scale := UnknownScale
			if n.Args[1].Op == OpInt {
				scale = int(n.Args[1].Int.Int64())
				if scale < 0 {
					return c.fail(n.Args[1], TypeMismatch, "round cannot take a negative number of places")
				}
			}
			return Type{Kind: Decimal, Scale: scale}
		case Double:
			return Type{Kind: Double}
		}
		return c.fail(n, TypeMismatch, "round(x, places) takes a decimal or a double, and this is %s", a0)
	case "floor", "ceil":
		if !count(1) {
			return badType
		}
		switch a0.Kind {
		case Decimal:
			return Type{Kind: Decimal, Scale: 0}
		case Double:
			return Type{Kind: Double}
		}
		return c.fail(n, TypeMismatch, "%s(x) takes a decimal or a double, and this is %s", n.Text, a0)
	case "min", "max":
		if len(args) < 2 {
			return c.fail(n, TypeMismatch, "%s takes two or more values", n.Text)
		}
		out := args[0]
		for _, a := range args {
			if !orderable(a.Kind) || a.Kind != args[0].Kind || a.Nullable {
				return c.fail(n, TypeMismatch, "%s takes values of one orderable type, never null; here it gets %s and %s; %s", n.Text, args[0], a, conversionHint(args[0], a))
			}
			if a.Kind == Decimal {
				out.Scale = maxScale(out.Scale, a.Scale)
			}
		}
		return out
	}
	return c.fail(n, Name, "%s is not a function of SpecArch expressions; the functions are %s", n.Text, FunctionList)
}

func (c *checker) decimal(n *Node, args []Type) Type {
	if len(args) != 2 {
		return c.fail(n, TypeMismatch, "decimal takes a value and a scale, decimal(x, scale), and here it has %d %s", len(args), plural(len(args), "argument"))
	}
	if n.Args[1].Op != OpInt || n.Args[1].Int.Sign() < 0 {
		return c.fail(n.Args[1], TypeMismatch, "the scale of decimal(x, scale) must be a whole number written in place, such as 2")
	}
	scale := int(n.Args[1].Int.Int64())
	a := args[0]
	if a.Nullable {
		return c.fail(n, TypeMismatch, "decimal cannot take a value that may be null; check it with == null first")
	}
	out := Type{Kind: Decimal, Scale: scale}
	switch a.Kind {
	case Int, Uint:
		return out
	case String:
		if lit := n.Args[0]; lit.Op == OpText {
			if !decimalText.MatchString(lit.Text) {
				return c.fail(lit, TypeMismatch, "%q is not a decimal number", lit.Text)
			}
			if _, frac, ok := strings.Cut(lit.Text, "."); ok && len(frac) > scale {
				return c.fail(lit, TypeMismatch, "%q has %d decimal places, more than the scale %d; round it or raise the scale", lit.Text, len(frac), scale)
			}
		}
		return out
	case Decimal:
		if a.Scale != UnknownScale && a.Scale <= scale {
			return out
		}
		return c.fail(n, TypeMismatch, "decimal(x, %d) of %s would drop digits without saying how; round it first: decimal(round(x, %d), %d)", scale, a, scale, scale)
	case Double:
		return c.fail(n, TypeMismatch, "a double cannot become a decimal, because the digits it lost cannot be brought back; keep the value a decimal from the start")
	}
	return c.fail(n, TypeMismatch, "decimal(x, scale) takes an int, a uint, a quoted number or a decimal, and this is %s", a)
}

// Fits reports whether a formula's type can be returned as the declared
// output, and says why not.
func Fits(got, want Type) (bool, string) {
	if got.Kind != want.Kind || (want.Kind == Enum && got.EnumName != want.EnumName && got.EnumName != "") {
		return false, fmt.Sprintf("the formula gives %s, but the output is %s; %s", got, want, conversionHint(got, want))
	}
	if got.Nullable && !want.Nullable {
		return false, fmt.Sprintf("the formula may give null, but the output does not allow null")
	}
	if got.Kind == Decimal && want.Scale != UnknownScale && (got.Scale == UnknownScale || got.Scale > want.Scale) {
		return false, fmt.Sprintf("the formula gives %s, but the output's scale is %d; say how to round with round(x, %d)", got, want.Scale, want.Scale)
	}
	return true, ""
}

func plural(n int, w string) string {
	if n == 1 {
		return w
	}
	return w + "s"
}

// ratOf turns an exact integer into a rational.
func ratOf(i *big.Int) *big.Rat { return new(big.Rat).SetInt(i) }

// nullTest returns the name in "name != null" (when notNull) or
// "name == null" (when not), or "".
func nullTest(n *Node, notNull bool) string {
	op := "=="
	if notNull {
		op = "!="
	}
	if n.Op != op || len(n.Args) != 2 {
		return ""
	}
	a, b := n.Args[0], n.Args[1]
	if a.Op == OpName && b.Op == OpNull {
		return a.Text
	}
	if b.Op == OpName && a.Op == OpNull {
		return b.Text
	}
	return ""
}

// withNonNull checks n with name known not to be null.
func (c *checker) withNonNull(name string, n *Node) Type {
	t, ok := c.env[name]
	if !ok || !t.Nullable {
		return c.check(n)
	}
	inner := Env{}
	for k, v := range c.env {
		inner[k] = v
	}
	t.Nullable = false
	inner[name] = t
	saved := c.env
	c.env = inner
	defer func() { c.env = saved }()
	return c.check(n)
}
