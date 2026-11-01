package genscreens

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
)

// A schema is printed as JavaScript or TypeScript data, laid out as
// prettier lays out an object literal: one line when it fits in width, and
// otherwise one member or element a line. The TypeScript and the Vue
// generators print through it, so a page reads the same in both.

// width is the longest line the printer writes where a value can be
// broken; a string is never broken, so a line holding a long one is
// longer.
const width = 140

// Flat is a value on one line.
func Flat(v Value) string {
	switch x := v.(type) {
	case Literal:
		if x.String {
			return Quote(x.Text)
		}
		return x.Text
	case ObjectValue:
		if len(x.Members) == 0 {
			return "{}"
		}
		parts := make([]string, len(x.Members))
		for i, m := range x.Members {
			parts[i] = Key(m.Key) + ": " + Flat(m.Val)
		}
		return "{ " + strings.Join(parts, ", ") + " }"
	case ArrayValue:
		parts := make([]string, len(x.Items))
		for i, e := range x.Items {
			parts[i] = Flat(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return ""
}

// Quote writes a string as a JavaScript string literal in double quotes.
func Quote(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}

var identifier = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// Key writes an object key, quoted only when it is not an identifier.
func Key(k string) string {
	if identifier.MatchString(k) {
		return k
	}
	return Quote(k)
}

// Statement writes prefix, the value and suffix, the value on one line
// when the whole fits in width, and broken otherwise.
func Statement(prefix string, v Value, suffix string) string {
	var b strings.Builder
	b.WriteString(prefix)
	write(&b, v, "", utf8.RuneCountInString(prefix), utf8.RuneCountInString(suffix))
	b.WriteString(suffix)
	b.WriteString("\n")
	return b.String()
}

// forcesBreak says whether an array is always broken, one element a line:
// two or more elements, every one an object of more than one key or every
// one an array of more than one element, as prettier lays arrays out.
func forcesBreak(v Value) bool {
	a, ok := v.(ArrayValue)
	if !ok || len(a.Items) < 2 {
		return false
	}
	_, objects := a.Items[0].(ObjectValue)
	for _, e := range a.Items {
		switch x := e.(type) {
		case ObjectValue:
			if !objects || len(x.Members) < 2 {
				return false
			}
		case ArrayValue:
			if objects || len(x.Items) < 2 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// write writes v at an indent, with used characters already on its first
// line and after characters to follow on its last.
func write(b *strings.Builder, v Value, indent string, used, after int) {
	one := Flat(v)
	if !forcesBreak(v) && used+utf8.RuneCountInString(one)+after <= width {
		b.WriteString(one)
		return
	}
	inner := indent + "  "
	switch x := v.(type) {
	case ObjectValue:
		if len(x.Members) == 0 {
			b.WriteString(one)
			return
		}
		b.WriteString("{\n")
		for _, m := range x.Members {
			k := Key(m.Key) + ": "
			b.WriteString(inner + k)
			write(b, m.Val, inner, len(inner)+utf8.RuneCountInString(k), 1)
			b.WriteString(",\n")
		}
		b.WriteString(indent + "}")
	case ArrayValue:
		if len(x.Items) == 0 {
			b.WriteString(one)
			return
		}
		b.WriteString("[\n")
		for _, e := range x.Items {
			b.WriteString(inner)
			write(b, e, inner, len(inner), 1)
			b.WriteString(",\n")
		}
		b.WriteString(indent + "]")
	default:
		b.WriteString(one)
	}
}
