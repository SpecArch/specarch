// Package canonical writes a JSON value in the canonical form of RFC 8785,
// the JSON Canonicalization Scheme: the members of every object sorted by
// their names as UTF-16 code units, no whitespace, every string in one
// spelling and every number as ECMAScript prints a double. Two programs
// that write one value this way write the same bytes, which is how the Go
// and the Swift build of specarch are compared.
package canonical

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Marshal writes v, built of nil, bool, string, json.Number, int,
// float64, []any and map[string]any, in canonical form. A number that is
// not finite, or a value of another type, is an error.
func Marshal(v any) ([]byte, error) {
	var b strings.Builder
	if err := write(&b, v); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func write(b *strings.Builder, v any) error {
	switch v := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if v {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case string:
		writeString(b, v)
	case int:
		return writeNumber(b, float64(v))
	case float64:
		return writeNumber(b, v)
	case json.Number:
		f, err := strconv.ParseFloat(string(v), 64)
		if err != nil {
			return fmt.Errorf("the number %s cannot be read as a double: %v", v, err)
		}
		return writeNumber(b, f)
	case []any:
		b.WriteByte('[')
		for i, e := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := write(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return utf16Less(keys[i], keys[j]) })
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			writeString(b, k)
			b.WriteByte(':')
			if err := write(b, v[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return fmt.Errorf("a value of type %T has no JSON form", v)
	}
	return nil
}

// utf16Less orders two strings by their UTF-16 code units (RFC 8785, 3.2.3).
func utf16Less(a, b string) bool {
	ua, ub := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}

// writeString escapes only what JSON requires (RFC 8785, 3.2.2.2): the
// quotation mark, the reverse solidus and the control characters, the
// five with a short form by it and the rest as \u00xx in lower case.
func writeString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// writeNumber prints a double as ECMAScript's Number.prototype.toString
// does (RFC 8785, 3.2.2.3), from the shortest digits that read back as it.
func writeNumber(b *strings.Builder, f float64) error {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return fmt.Errorf("the number %v is not finite, and JSON has no form for it", f)
	}
	if f == 0 {
		b.WriteByte('0')
		return nil
	}
	if f < 0 {
		b.WriteByte('-')
		f = -f
	}
	// The shortest digits, as d.ddddde±x.
	e := strconv.FormatFloat(f, 'e', -1, 64)
	mantissa, exp, _ := strings.Cut(e, "e")
	digits := strings.Replace(mantissa, ".", "", 1)
	x, _ := strconv.Atoi(exp)
	b.WriteString(Format(digits, x+1))
	return nil
}

// Format lays out the digits of a positive double, k of them with no
// leading or trailing zero, whose value is 0.digits × 10^n, by the rules
// of ECMAScript's Number::toString.
func Format(digits string, n int) string {
	k := len(digits)
	switch {
	case k <= n && n <= 21:
		return digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return "0." + strings.Repeat("0", -n) + digits
	}
	sign := "+"
	if n-1 < 0 {
		sign = "-"
	}
	exp := n - 1
	if exp < 0 {
		exp = -exp
	}
	if k == 1 {
		return digits + "e" + sign + strconv.Itoa(exp)
	}
	return digits[:1] + "." + digits[1:] + "e" + sign + strconv.Itoa(exp)
}
