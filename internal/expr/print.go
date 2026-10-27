package expr

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Text writes a parsed expression in the subset's own form: the operators
// with one space either side, a call as name(arguments, ...), parentheses
// only where CEL's grammar needs them, a double literal with its point. The
// Swift build writes the same text (ADR-195).
func Text(n *Node) string {
	var b strings.Builder
	write(&b, n)
	return b.String()
}

// precedence is CEL's grammar's level of a node, higher binding tighter.
func precedence(n *Node) int {
	switch n.Op {
	case OpCond:
		return 0
	case "||":
		return 1
	case "&&":
		return 2
	case "==", "!=", "<", "<=", ">", ">=":
		return 3
	case "+", "-":
		return 4
	case "*", "/":
		return 5
	case OpNeg, OpNot:
		return 6
	}
	return 7
}

// writeAbove writes n, in parentheses when its level is below min.
func writeAbove(b *strings.Builder, n *Node, min int) {
	if precedence(n) < min {
		b.WriteString("(")
		write(b, n)
		b.WriteString(")")
		return
	}
	write(b, n)
}

func write(b *strings.Builder, n *Node) {
	switch n.Op {
	case OpInt:
		b.WriteString(n.Int.String())
	case OpUint:
		b.WriteString(n.Int.String() + "u")
	case OpDouble:
		b.WriteString(doubleLiteral(n.Double))
	case OpText:
		b.WriteString(Quote(n.Text))
	case OpBool:
		b.WriteString(strconv.FormatBool(n.Bool))
	case OpNull:
		b.WriteString("null")
	case OpName:
		b.WriteString(n.Text)
	case OpNeg, OpNot:
		b.WriteString(map[string]string{OpNeg: "-", OpNot: "!"}[n.Op])
		writeAbove(b, n.Args[0], 6)
	case OpCond:
		writeAbove(b, n.Args[0], 1)
		b.WriteString(" ? ")
		writeAbove(b, n.Args[1], 1)
		b.WriteString(" : ")
		write(b, n.Args[2])
	case OpCall:
		b.WriteString(n.Text + "(")
		for i, a := range n.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			write(b, a)
		}
		b.WriteString(")")
	default:
		p := precedence(n)
		writeAbove(b, n.Args[0], p)
		b.WriteString(" " + n.Op + " ")
		// && and || are associative; every other operator groups to
		// the left, so an operand of its own level on the right keeps
		// its parentheses.
		if n.Op == "&&" || n.Op == "||" {
			writeAbove(b, n.Args[1], p)
		} else {
			writeAbove(b, n.Args[1], p+1)
		}
	}
}

// doubleLiteral writes a double so that it reads back as a double.
func doubleLiteral(f float64) string {
	s := FormatDouble(f)
	if !strings.ContainsAny(s, ".e") {
		s += ".0"
	}
	return s
}

// Quote writes a string literal as both builds do: in double quotes, with
// a quote, a backslash, a newline and a tab escaped.
func Quote(s string) string {
	r := strings.NewReplacer(`"`, `\"`, `\`, `\\`, "\n", `\n`, "\t", `\t`)
	return `"` + r.Replace(s) + `"`
}

// FormatDouble writes a double as both builds do: a whole number below
// 1e21 with its digits, any other in the shortest form that reads back,
// plain between 1e-4 and 1e16 and with an exponent of at least two digits
// outside.
func FormatDouble(f float64) string {
	switch {
	case math.IsNaN(f):
		return "nan"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case f == math.Trunc(f) && math.Abs(f) < 1e21:
		return strconv.FormatFloat(f, 'f', 0, 64)
	}
	e := strconv.FormatFloat(f, 'e', -1, 64) // such as -1.2345e+06
	mantissa, exp, _ := strings.Cut(e, "e")
	x, _ := strconv.Atoi(exp)
	if x < -4 || x >= 16 {
		sign := "+"
		if x < 0 {
			sign, x = "-", -x
		}
		return fmt.Sprintf("%se%s%02d", mantissa, sign, x)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// FormatDuration writes a duration in ISO 8601's day-and-time form, as a
// worked example does: P14D, P1DT2H, PT1H30M, PT0.5S, -PT15M, PT0S.
func FormatDuration(d time.Duration) string {
	sign := ""
	if d < 0 {
		sign, d = "-", -d
	}
	if d == 0 {
		return "PT0S"
	}
	days, h, m := d/(24*time.Hour), d%(24*time.Hour)/time.Hour, d%time.Hour/time.Minute
	ns := d % time.Minute
	s := sign + "P"
	if days > 0 {
		s += fmt.Sprintf("%dD", days)
	}
	if h == 0 && m == 0 && ns == 0 {
		return s
	}
	s += "T"
	if h > 0 {
		s += fmt.Sprintf("%dH", h)
	}
	if m > 0 {
		s += fmt.Sprintf("%dM", m)
	}
	if ns > 0 {
		secs := strconv.FormatInt(int64(ns/time.Second), 10)
		if frac := ns % time.Second; frac > 0 {
			secs += strings.TrimRight(fmt.Sprintf(".%09d", frac), "0")
		}
		s += secs + "S"
	}
	return s
}

// Show writes a value as a worked example writes it, at its type, so that
// it can be copied into one: a decimal in quotes at the type's scale, or
// with up to 20 places when its scale is not fixed; a date, an instant, a
// time of day and a duration in quotes, the duration in ISO 8601; a double
// as FormatDouble; a number and a bool as they are.
func Show(v Value, t Type) string {
	switch v.Kind {
	case Decimal:
		if t.Kind == Decimal && t.Scale >= 0 {
			return Quote(v.Num.FloatString(t.Scale))
		}
		// A decimal of no fixed scale, the result of /, with the places
		// it needs up to 20, rounded half away from zero past them: the
		// Swift build holds 38 digits and this one more, and both agree
		// to 20 places.
		return Quote(FormatDecimal(Round(v.Num, 20)))
	case Double:
		return FormatDouble(v.Double)
	case Duration:
		return Quote(FormatDuration(v.Dur))
	case String, Enum:
		return Quote(v.Text)
	case Date, Timestamp, TimeOfDay:
		return Quote(v.String())
	}
	return v.String()
}
