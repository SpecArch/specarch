package expr

import (
	"encoding/base64"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"
	"unicode/utf8"
)

// Value is a value during evaluation. Int, uint and decimal values are
// exact; a double is a float64, as CEL has it.
type Value struct {
	Kind   Kind
	Num    *big.Rat // Int, Uint, Decimal
	Double float64
	Text   string // String, Enum, Bytes (raw)
	Bool   bool
	Time   time.Time     // Date, Timestamp, TimeOfDay
	Dur    time.Duration // Duration
	Items  int           // List: the length
}

// String shows a value the way a worked example writes it.
func (v Value) String() string {
	switch v.Kind {
	case Int, Uint:
		return v.Num.Num().String()
	case Decimal:
		return fmt.Sprintf("%q", FormatDecimal(v.Num))
	case Double:
		return strconv.FormatFloat(v.Double, 'g', -1, 64)
	case String, Enum:
		return fmt.Sprintf("%q", v.Text)
	case Bool:
		return fmt.Sprint(v.Bool)
	case Date:
		return v.Time.Format("2006-01-02")
	case Timestamp:
		return v.Time.Format(time.RFC3339Nano)
	case TimeOfDay:
		return v.Time.Format("15:04:05")
	case Duration:
		return v.Dur.String()
	case Null:
		return "null"
	}
	return string(v.Kind)
}

// FormatDecimal writes an exact number with as many places as it needs.
func FormatDecimal(r *big.Rat) string {
	if r.IsInt() {
		return r.Num().String()
	}
	ten := big.NewInt(10)
	for places := 1; places <= 40; places++ {
		scaled := new(big.Rat).Mul(r, ratOf(new(big.Int).Exp(ten, big.NewInt(int64(places)), nil)))
		if scaled.IsInt() {
			return r.FloatString(places)
		}
	}
	return r.FloatString(40)
}

// EvalError is a value an expression cannot compute.
type EvalError struct {
	Line, Column int
	Message      string
}

func (e *EvalError) Error() string { return e.Message }

func evalErr(n *Node, format string, args ...any) error {
	return &EvalError{Line: n.Line, Column: n.Column, Message: fmt.Sprintf(format, args...)}
}

var (
	minInt64  = ratOf(big.NewInt(math.MinInt64))
	maxInt64  = ratOf(big.NewInt(math.MaxInt64))
	maxUint64 = ratOf(new(big.Int).SetUint64(math.MaxUint64))
)

// FitsInt reports whether r is a whole number inside a signed width.
func FitsInt(r *big.Rat, bits int) bool {
	if !r.IsInt() {
		return false
	}
	lo, hi := minInt64, maxInt64
	if bits == 32 {
		lo, hi = ratOf(big.NewInt(math.MinInt32)), ratOf(big.NewInt(math.MaxInt32))
	}
	return r.Cmp(lo) >= 0 && r.Cmp(hi) <= 0
}

func checkInt(n *Node, v Value) (Value, error) {
	switch v.Kind {
	case Int:
		if !FitsInt(v.Num, 64) {
			return Value{}, evalErr(n, "the result %s does not fit in an int (64 bits)", v.Num.Num())
		}
	case Uint:
		if v.Num.Sign() < 0 || v.Num.Cmp(maxUint64) > 0 {
			return Value{}, evalErr(n, "the result %s does not fit in a uint (0 to 2^64 - 1)", v.Num.Num())
		}
	}
	return v, nil
}

// Eval computes a checked expression with the given values.
func Eval(n *Node, vals map[string]Value) (Value, error) {
	switch n.Op {
	case OpInt:
		return Value{Kind: Int, Num: ratOf(n.Int)}, nil
	case OpUint:
		return Value{Kind: Uint, Num: ratOf(n.Int)}, nil
	case OpDouble:
		return Value{Kind: Double, Double: n.Double}, nil
	case OpText:
		return Value{Kind: String, Text: n.Text}, nil
	case OpBool:
		return Value{Kind: Bool, Bool: n.Bool}, nil
	case OpNull:
		return Value{Kind: Null}, nil
	case OpName:
		v, ok := vals[n.Text]
		if !ok {
			return Value{}, evalErr(n, "%s has no value", n.Text)
		}
		return v, nil
	case OpNeg:
		v, err := Eval(n.Args[0], vals)
		if err != nil {
			return v, err
		}
		if v.Kind == Double {
			return Value{Kind: Double, Double: -v.Double}, nil
		}
		return checkInt(n, Value{Kind: v.Kind, Num: new(big.Rat).Neg(v.Num)})
	case OpNot:
		v, err := Eval(n.Args[0], vals)
		if err != nil {
			return v, err
		}
		return Value{Kind: Bool, Bool: !v.Bool}, nil
	case "&&", "||":
		l, err := Eval(n.Args[0], vals)
		if err != nil {
			return l, err
		}
		if (n.Op == "&&" && !l.Bool) || (n.Op == "||" && l.Bool) {
			return l, nil
		}
		return Eval(n.Args[1], vals)
	case OpCond:
		c, err := Eval(n.Args[0], vals)
		if err != nil {
			return c, err
		}
		if c.Bool {
			return Eval(n.Args[1], vals)
		}
		return Eval(n.Args[2], vals)
	case OpCall:
		return call(n, vals)
	}
	l, err := Eval(n.Args[0], vals)
	if err != nil {
		return l, err
	}
	r, err := Eval(n.Args[1], vals)
	if err != nil {
		return r, err
	}
	switch n.Op {
	case "+", "-", "*", "/":
		return arithmetic(n, l, r)
	case "==", "!=":
		return Value{Kind: Bool, Bool: Equal(l, r) == (n.Op == "==")}, nil
	}
	cmp, ok := compare(l, r)
	if !ok {
		return Value{}, evalErr(n, "%s cannot compare %s and %s", n.Op, l, r)
	}
	b := map[string]bool{"<": cmp < 0, "<=": cmp <= 0, ">": cmp > 0, ">=": cmp >= 0}[n.Op]
	return Value{Kind: Bool, Bool: b}, nil
}

func arithmetic(n *Node, l, r Value) (Value, error) {
	if l.Kind == Date {
		days := int(r.Dur / (24 * time.Hour))
		if n.Op == "-" {
			days = -days
		}
		return Value{Kind: Date, Time: l.Time.AddDate(0, 0, days)}, nil
	}
	if l.Kind == Double {
		var f float64
		switch n.Op {
		case "+":
			f = l.Double + r.Double
		case "-":
			f = l.Double - r.Double
		case "*":
			f = l.Double * r.Double
		case "/":
			f = l.Double / r.Double
		}
		return Value{Kind: Double, Double: f}, nil
	}
	out := new(big.Rat)
	switch n.Op {
	case "+":
		out.Add(l.Num, r.Num)
	case "-":
		out.Sub(l.Num, r.Num)
	case "*":
		out.Mul(l.Num, r.Num)
	case "/":
		if r.Num.Sign() == 0 {
			return Value{}, evalErr(n, "division by zero")
		}
		out.Quo(l.Num, r.Num)
		if l.Kind != Decimal {
			// CEL divides integers toward zero.
			q := new(big.Int).Quo(out.Num(), out.Denom())
			out = ratOf(q)
		}
	}
	return checkInt(n, Value{Kind: l.Kind, Num: out})
}

// Equal compares two values of one type; decimals by value, so "3.50"
// equals "3.5".
func Equal(a, b Value) bool {
	if a.Kind == Null || b.Kind == Null {
		return a.Kind == b.Kind
	}
	if cmp, ok := compare(a, b); ok {
		return cmp == 0
	}
	switch {
	case (a.Kind == String || a.Kind == Enum) && (b.Kind == String || b.Kind == Enum):
		return a.Text == b.Text
	case a.Kind == Bool && b.Kind == Bool:
		return a.Bool == b.Bool
	case a.Kind == Bytes && b.Kind == Bytes:
		return a.Text == b.Text
	}
	return false
}

func compare(a, b Value) (int, bool) {
	if a.Kind != b.Kind {
		return 0, false
	}
	switch a.Kind {
	case Int, Uint, Decimal:
		return a.Num.Cmp(b.Num), true
	case Double:
		switch {
		case a.Double < b.Double:
			return -1, true
		case a.Double > b.Double:
			return 1, true
		}
		return 0, true
	case String:
		switch {
		case a.Text < b.Text:
			return -1, true
		case a.Text > b.Text:
			return 1, true
		}
		return 0, true
	case Date, Timestamp, TimeOfDay:
		return a.Time.Compare(b.Time), true
	case Duration:
		switch {
		case a.Dur < b.Dur:
			return -1, true
		case a.Dur > b.Dur:
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func call(n *Node, vals map[string]Value) (Value, error) {
	args := make([]Value, len(n.Args))
	for i, a := range n.Args {
		v, err := Eval(a, vals)
		if err != nil {
			return v, err
		}
		args[i] = v
	}
	a := Value{}
	if len(args) > 0 {
		a = args[0]
	}
	switch n.Text {
	case "int", "uint":
		kind := Int
		if n.Text == "uint" {
			kind = Uint
		}
		switch a.Kind {
		case String:
			i, ok := new(big.Int).SetString(a.Text, 10)
			if !ok {
				return Value{}, evalErr(n, "%q is not a whole number", a.Text)
			}
			return checkInt(n, Value{Kind: kind, Num: ratOf(i)})
		case Double:
			if math.IsNaN(a.Double) || math.IsInf(a.Double, 0) {
				return Value{}, evalErr(n, "%s cannot become an int", a)
			}
			r := new(big.Rat)
			r.SetFloat64(a.Double)
			return checkInt(n, Value{Kind: kind, Num: r})
		}
		if !a.Num.IsInt() {
			return Value{}, evalErr(n, "%s is not a whole number", a)
		}
		return checkInt(n, Value{Kind: kind, Num: a.Num})
	case "double":
		switch a.Kind {
		case String:
			f, err := strconv.ParseFloat(a.Text, 64)
			if err != nil {
				return Value{}, evalErr(n, "%q is not a number", a.Text)
			}
			return Value{Kind: Double, Double: f}, nil
		}
		f, _ := a.Num.Float64()
		return Value{Kind: Double, Double: f}, nil
	case "decimal":
		scale := int(n.Args[1].Int.Int64())
		var r *big.Rat
		if a.Kind == String {
			var ok bool
			if r, ok = new(big.Rat).SetString(a.Text); !ok || !decimalText.MatchString(a.Text) {
				return Value{}, evalErr(n, "%q is not a decimal number", a.Text)
			}
		} else {
			r = a.Num
		}
		if !new(big.Rat).Mul(r, ratOf(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil))).IsInt() {
			return Value{}, evalErr(n, "%s has more than %d decimal places; round it first", FormatDecimal(r), scale)
		}
		return Value{Kind: Decimal, Num: r}, nil
	case "string":
		switch a.Kind {
		case String, Enum:
			return Value{Kind: String, Text: a.Text}, nil
		case Decimal:
			return Value{Kind: String, Text: FormatDecimal(a.Num)}, nil
		case Bytes:
			return Value{Kind: String, Text: a.Text}, nil
		}
		s := a.String()
		return Value{Kind: String, Text: s}, nil
	case "date":
		switch a.Kind {
		case Date:
			return a, nil
		case Timestamp:
			t := a.Time
			return Value{Kind: Date, Time: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)}, nil
		case String:
			t, err := time.Parse("2006-01-02", a.Text)
			if err != nil {
				return Value{}, evalErr(n, "%q is not a date", a.Text)
			}
			return Value{Kind: Date, Time: t}, nil
		}
	case "timestamp":
		if a.Kind == Timestamp {
			return a, nil
		}
		t, err := time.Parse(time.RFC3339Nano, a.Text)
		if err != nil {
			return Value{}, evalErr(n, "%q is not a timestamp", a.Text)
		}
		return Value{Kind: Timestamp, Time: t}, nil
	case "duration":
		d, ok := ParseDuration(a.Text)
		if !ok {
			return Value{}, evalErr(n, "%q is not a duration", a.Text)
		}
		return Value{Kind: Duration, Dur: d}, nil
	case "size":
		switch a.Kind {
		case String:
			return Value{Kind: Int, Num: ratOf(big.NewInt(int64(utf8.RuneCountInString(a.Text))))}, nil
		case Bytes:
			return Value{Kind: Int, Num: ratOf(big.NewInt(int64(len(a.Text))))}, nil
		case List:
			return Value{Kind: Int, Num: ratOf(big.NewInt(int64(a.Items)))}, nil
		}
	case "round":
		places := args[1].Num
		if places.Sign() < 0 || places.Num().Int64() > 30 {
			return Value{}, evalErr(n, "round needs from 0 to 30 places, and got %s", args[1])
		}
		p := int(places.Num().Int64())
		if a.Kind == Double {
			scale := math.Pow(10, float64(p))
			return Value{Kind: Double, Double: math.Round(a.Double*scale) / scale}, nil
		}
		return Value{Kind: Decimal, Num: Round(a.Num, p)}, nil
	case "floor", "ceil":
		if a.Kind == Double {
			f := math.Floor(a.Double)
			if n.Text == "ceil" {
				f = math.Ceil(a.Double)
			}
			return Value{Kind: Double, Double: f}, nil
		}
		q := new(big.Int)
		m := new(big.Int)
		q.DivMod(a.Num.Num(), a.Num.Denom(), m) // Euclidean: rounds toward minus infinity for a positive divisor
		if n.Text == "ceil" && m.Sign() != 0 {
			q.Add(q, big.NewInt(1))
		}
		return Value{Kind: Decimal, Num: ratOf(q)}, nil
	case "min", "max":
		best := args[0]
		for _, v := range args[1:] {
			cmp, ok := compare(v, best)
			if !ok {
				return Value{}, evalErr(n, "%s cannot compare %s and %s", n.Text, v, best)
			}
			if (n.Text == "min" && cmp < 0) || (n.Text == "max" && cmp > 0) {
				best = v
			}
		}
		return best, nil
	}
	return Value{}, evalErr(n, "%s cannot take %s", n.Text, a)
}

// Round rounds half away from zero to the given number of decimal places.
func Round(r *big.Rat, places int) *big.Rat {
	scale := ratOf(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil))
	x := new(big.Rat).Mul(r, scale)
	neg := x.Sign() < 0
	x.Abs(x)
	x.Add(x, big.NewRat(1, 2))
	q := new(big.Int).Quo(x.Num(), x.Denom())
	if neg {
		q.Neg(q)
	}
	return new(big.Rat).Quo(ratOf(q), scale)
}

// DecodeBytes reads a base64 example value.
func DecodeBytes(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	return string(b), err
}
