package expr

import (
	"math/big"
	"strings"
	"testing"
	"time"
)

func dec(s string) Value {
	r, _ := new(big.Rat).SetString(s)
	return Value{Kind: Decimal, Num: r}
}

func eval(t *testing.T, src string, env Env, vals map[string]Value) (Value, error) {
	t.Helper()
	n, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("%s: %v", src, errs)
	}
	if _, errs := Check(n, env); len(errs) > 0 {
		t.Fatalf("%s: %v", src, errs)
	}
	return Eval(n, vals, time.UTC)
}

func TestRoundHalfAwayFromZero(t *testing.T) {
	for in, want := range map[string]string{
		"2.675": "2.68", "-2.675": "-2.68", "2.665": "2.67", "0.004": "0.00", "-0.005": "-0.01",
	} {
		r, _ := new(big.Rat).SetString(in)
		if got := Round(r, 2).FloatString(2); got != want {
			t.Errorf("round(%s, 2) = %s, want %s", in, got, want)
		}
	}
}

func TestFloorAndCeilOfNegativeDecimals(t *testing.T) {
	env := Env{"x": {Kind: Decimal, Scale: 2}}
	for src, want := range map[string]string{
		"floor(x)": "-3", "ceil(x)": "-2",
	} {
		v, err := eval(t, src, env, map[string]Value{"x": dec("-2.50")})
		if err != nil {
			t.Fatal(err)
		}
		if got := FormatDecimal(v.Num); got != want {
			t.Errorf("%s of -2.50 = %s, want %s", src, got, want)
		}
	}
}

func TestIntegerOverflowIsAnError(t *testing.T) {
	env := Env{"x": {Kind: Int, Bits: 64}}
	_, err := eval(t, "x * 2", env, map[string]Value{"x": {Kind: Int, Num: big.NewRat(1<<62, 1)}})
	if err == nil || !strings.Contains(err.Error(), "does not fit") {
		t.Errorf("want an overflow error, got %v", err)
	}
}

func TestIntegerDivisionTruncates(t *testing.T) {
	v, err := eval(t, "-7 / 2", Env{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatDecimal(v.Num); got != "-3" {
		t.Errorf("-7 / 2 = %s, want -3, as CEL divides toward zero", got)
	}
}

func TestDecimalScaleIsTracked(t *testing.T) {
	env := Env{"a": {Kind: Decimal, Scale: 2}, "b": {Kind: Decimal, Scale: 3}}
	for src, want := range map[string]int{"a + b": 3, "a * b": 5, "a / b": UnknownScale, "round(a / b, 2)": 2, "min(a, b)": 3} {
		n, _ := Parse(src)
		got, errs := Check(n, env)
		if len(errs) > 0 || got.Scale != want {
			t.Errorf("%s: scale %d (%v), want %d", src, got.Scale, errs, want)
		}
	}
}

func TestNoImplicitConversion(t *testing.T) {
	env := Env{"n": {Kind: Int, Bits: 32}, "d": {Kind: Decimal, Scale: 2}, "f": {Kind: Double}}
	for _, src := range []string{"n * d", "d + f", "n == d", "int(d)", "decimal(f, 2)", "n < 1u"} {
		n, errs := Parse(src)
		if len(errs) > 0 {
			t.Fatalf("%s: %v", src, errs)
		}
		if _, errs := Check(n, env); len(errs) == 0 {
			t.Errorf("%s: want a type error", src)
		}
	}
}

func TestOutsideTheSubsetIsRefusedByName(t *testing.T) {
	for src, word := range map[string]string{
		"a % 2": "%", "a in b": "in", "has(a)": "has", "a.b": ".", "[1, 2]": "lists", "a.size()": "method", "x.exists(y, y > 1)": "method",
	} {
		_, errs := Parse(src)
		if len(errs) == 0 || !strings.Contains(errs[0].Message, word) {
			t.Errorf("%s: want a refusal naming %q, got %v", src, word, errs)
		}
	}
}

// date() of an instant is the instant's calendar date in the zone given,
// whatever offset the instant was written with (ADR-193).
func TestDateOfInstantReadsZone(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	env := Env{"at": {Kind: Timestamp}}
	for _, c := range []struct {
		src, want string
		zone      *time.Location
	}{
		{`date(timestamp("2026-10-27T03:00:00+07:00"))`, "2026-10-26", time.UTC},
		{`date(timestamp("2026-10-27T03:00:00+07:00"))`, "2026-10-27", jakarta},
		{`date(timestamp("2026-10-26T20:00:00Z"))`, "2026-10-27", jakarta},
		{`date(timestamp("2026-10-26T16:59:59Z"))`, "2026-10-26", jakarta},
		{`date(at)`, "2026-10-27", jakarta},
		{`string(at)`, `"2026-10-26T20:00:00Z"`, time.UTC},
	} {
		n, errs := Parse(c.src)
		if len(errs) > 0 {
			t.Fatalf("%s: %v", c.src, errs)
		}
		at, _ := time.Parse(time.RFC3339, "2026-10-27T03:00:00+07:00")
		got, err := Eval(n, map[string]Value{"at": {Kind: Timestamp, Time: at.UTC()}}, c.zone)
		if err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		if got.String() != c.want {
			t.Errorf("%s in %s: got %s, want %s", c.src, c.zone, got, c.want)
		}
	}
	if _, errs := Check(mustParse(t, `date(at) == date("2026-10-27")`), env); len(errs) > 0 {
		t.Fatal(errs)
	}
	// A call's position is its opening parenthesis, as every call diagnostic has it.
	if dates := InstantDates(mustParse(t, `date(at) == date("2026-10-27") && size(string(at)) > 0`), env); len(dates) != 1 || dates[0].Column != 5 {
		t.Errorf("InstantDates: got %d calls, want the one at column 5", len(dates))
	}
}

func mustParse(t *testing.T, src string) *Node {
	t.Helper()
	n, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("%s: %v", src, errs)
	}
	return n
}
