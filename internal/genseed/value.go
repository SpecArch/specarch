package genseed

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SpecArch/specarch/internal/expr"
)

// kind is what a generated value is, which decides its literal on each
// dialect and its form in an expression.
type kind int

const (
	kNull kind = iota
	kBool
	kInt     // an integer of any width, in num
	kDecimal // an exact decimal, in text with its scale
	kDouble
	kText // text, an enum value, a uuid, an email and every other string format
	kDate
	kTimestamp
	kTimeOfDay // the seconds since midnight, in num
	kDuration
	kBytes
	kList // a list of values, in items
	kJSON // an object, as its members
)

// value is one generated value.
type value struct {
	kind  kind
	b     bool
	num   int64
	text  string
	f     float64
	t     time.Time
	d     time.Duration
	bytes []byte
	items []value
	// members are an object's values by their design name, written out as
	// JSON with sorted keys.
	members map[string]value
	// enum marks text that is an enum value, which an expression compares
	// as one.
	enum bool
	// uint marks an integer the design carries as unsigned.
	uint bool
	// json marks a list kept in a JSON column, written as JSON text on
	// every dialect rather than as the dialect's array.
	json bool
}

var null = value{kind: kNull}

func textValue(s string) value    { return value{kind: kText, text: s} }
func intValue(n int64) value      { return value{kind: kInt, num: n} }
func boolValue(b bool) value      { return value{kind: kBool, b: b} }
func dateValue(t time.Time) value { return value{kind: kDate, t: t} }

// quote is a SQL string literal.
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// literal writes the value in the dialect, for an INSERT.
func (v value) literal(dialect string, literals map[string]string) string {
	switch v.kind {
	case kNull:
		return "NULL"
	case kBool:
		if l := literals[strconv.FormatBool(v.b)]; l != "" {
			return l
		}
		return strconv.FormatBool(v.b)
	case kInt:
		return strconv.FormatInt(v.num, 10)
	case kDecimal:
		return v.text
	case kDouble:
		return strconv.FormatFloat(v.f, 'f', 2, 64)
	case kText:
		return quote(v.text)
	case kDate:
		s := v.t.Format("2006-01-02")
		if dialect == "postgresql" || dialect == "oracle" {
			return "DATE " + quote(s)
		}
		return quote(s)
	case kTimestamp:
		switch dialect {
		case "postgresql":
			return "TIMESTAMP WITH TIME ZONE " + quote(v.t.UTC().Format("2006-01-02 15:04:05+00"))
		case "oracle":
			return "TIMESTAMP " + quote(v.t.UTC().Format("2006-01-02 15:04:05 +00:00"))
		case "mariadb":
			// The column is UTC text with nine fractional digits
			// (type-rendering, defaults, mariadb).
			return quote(v.t.UTC().Format("2006-01-02T15:04:05.000000000Z"))
		}
		return quote(v.t.UTC().Format("2006-01-02T15:04:05Z"))
	case kTimeOfDay:
		s := clock(v.num)
		switch dialect {
		case "postgresql":
			return "TIME " + quote(s)
		case "oracle":
			return "INTERVAL " + quote("0 "+s) + " DAY TO SECOND"
		}
		return quote(s)
	case kDuration:
		if dialect == "oracle" {
			days := int64(v.d / (24 * time.Hour))
			rest := int64((v.d % (24 * time.Hour)) / time.Second)
			return "INTERVAL " + quote(fmt.Sprintf("%d %s", days, clock(rest))) + " DAY TO SECOND"
		}
		s := isoDuration(v.d)
		if dialect == "postgresql" {
			return "INTERVAL " + quote(s)
		}
		return quote(s)
	case kBytes:
		h := hex.EncodeToString(v.bytes)
		switch dialect {
		case "postgresql":
			return quote(`\x` + h)
		case "sqlserver":
			return "0x" + strings.ToUpper(h)
		case "oracle":
			return "HEXTORAW(" + quote(strings.ToUpper(h)) + ")"
		}
		return "X" + quote(strings.ToUpper(h))
	case kList:
		if dialect == "postgresql" && !v.json {
			if len(v.items) == 0 {
				return "'{}'"
			}
			parts := make([]string, len(v.items))
			for i, it := range v.items {
				parts[i] = it.literal(dialect, literals)
			}
			return "ARRAY[" + strings.Join(parts, ", ") + "]"
		}
		return quote(v.jsonText())
	case kJSON:
		return quote(v.jsonText())
	}
	return "NULL"
}

// clock is a time of day or a length under a day as HH:MM:SS.
func clock(seconds int64) string {
	return fmt.Sprintf("%02d:%02d:%02d", seconds/3600, seconds%3600/60, seconds%60)
}

// isoDuration writes a duration as ISO 8601, PT2H30M.
func isoDuration(d time.Duration) string {
	var b strings.Builder
	b.WriteString("P")
	if days := d / (24 * time.Hour); days > 0 {
		fmt.Fprintf(&b, "%dD", days)
		d -= days * 24 * time.Hour
	}
	if d > 0 {
		b.WriteString("T")
		if h := d / time.Hour; h > 0 {
			fmt.Fprintf(&b, "%dH", h)
			d -= h * time.Hour
		}
		if m := d / time.Minute; m > 0 {
			fmt.Fprintf(&b, "%dM", m)
			d -= m * time.Minute
		}
		if s := d / time.Second; s > 0 {
			fmt.Fprintf(&b, "%dS", s)
		}
	}
	if b.Len() == 1 {
		return "PT0S"
	}
	return b.String()
}

// jsonText writes the value as JSON, keys sorted.
func (v value) jsonText() string {
	data, _ := json.Marshal(v.jsonValue())
	return string(data)
}

func (v value) jsonValue() any {
	switch v.kind {
	case kNull:
		return nil
	case kBool:
		return v.b
	case kInt:
		return json.Number(strconv.FormatInt(v.num, 10))
	case kDecimal:
		return v.text
	case kDouble:
		return json.Number(strconv.FormatFloat(v.f, 'f', 2, 64))
	case kText:
		return v.text
	case kDate:
		return v.t.Format("2006-01-02")
	case kTimestamp:
		return v.t.UTC().Format("2006-01-02T15:04:05Z")
	case kTimeOfDay:
		return clock(v.num)
	case kDuration:
		return isoDuration(v.d)
	case kBytes:
		return v.bytes // base64 in JSON
	case kList:
		out := make([]any, len(v.items))
		for i, it := range v.items {
			out[i] = it.jsonValue()
		}
		return out
	case kJSON:
		out := map[string]any{}
		for k, m := range v.members {
			out[k] = m.jsonValue()
		}
		return out
	}
	return nil
}

// key is the value's identity, for telling two rows apart in a key or a
// unique constraint.
func (v value) key() string {
	switch v.kind {
	case kList, kJSON:
		return v.jsonText()
	}
	return v.literal("postgresql", nil)
}

// expression is the value as the expression evaluator reads it.
func (v value) expression() expr.Value {
	switch v.kind {
	case kNull:
		return expr.Value{Kind: expr.Null}
	case kBool:
		return expr.Value{Kind: expr.Bool, Bool: v.b}
	case kInt:
		k := expr.Int
		if v.uint {
			k = expr.Uint
		}
		return expr.Value{Kind: k, Num: new(big.Rat).SetInt64(v.num)}
	case kDecimal:
		r, _ := new(big.Rat).SetString(v.text)
		return expr.Value{Kind: expr.Decimal, Num: r}
	case kDouble:
		return expr.Value{Kind: expr.Double, Double: v.f}
	case kText:
		if v.enum {
			return expr.Value{Kind: expr.Enum, Text: v.text}
		}
		return expr.Value{Kind: expr.String, Text: v.text}
	case kDate:
		return expr.Value{Kind: expr.Date, Time: v.t}
	case kTimestamp:
		return expr.Value{Kind: expr.Timestamp, Time: v.t.UTC()}
	case kTimeOfDay:
		// A time of day the evaluator reads from "15:04:05" lies on the
		// zero date, so this one does too.
		return expr.Value{Kind: expr.TimeOfDay, Time: time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(v.num) * time.Second)}
	case kDuration:
		return expr.Value{Kind: expr.Duration, Dur: v.d}
	case kBytes:
		return expr.Value{Kind: expr.Bytes, Text: string(v.bytes)}
	case kList:
		return expr.Value{Kind: expr.List, Items: len(v.items)}
	}
	return expr.Value{Kind: expr.Object}
}

// sortedKeys are a map's keys in order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
