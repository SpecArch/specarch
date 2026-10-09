package genuits

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
)

// width is the longest line the printer writes where a value can be
// broken; a string is never broken, so a line holding a long one is
// longer.
const width = 140

// value is a value of an object literal, as the printer lays it out.
type value interface {
	// flat is the value on one line.
	flat() string
}

// member is one key and its value in an object.
type member struct {
	key string
	val value
}

type literal string

func (l literal) flat() string { return string(l) }

type objectValue []member

func (o objectValue) flat() string {
	if len(o) == 0 {
		return "{}"
	}
	parts := make([]string, len(o))
	for i, m := range o {
		parts[i] = key(m.key) + ": " + m.val.flat()
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

type arrayValue []value

func (a arrayValue) flat() string {
	parts := make([]string, len(a))
	for i, v := range a {
		parts[i] = v.flat()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func str(s string) value        { return literal(quote(s)) }
func raw(s string) value        { return literal(s) }
func object(ms []member) value  { return objectValue(ms) }
func array(vs []value) value    { return arrayValue(vs) }
func (o objectValue) size() int { return len(o) }
func (a arrayValue) size() int  { return len(a) }

// quote writes a string as a TypeScript string literal in double quotes.
func quote(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}

var identifier = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// key writes an object key, quoted only when it is not an identifier.
func key(k string) string {
	if identifier.MatchString(k) {
		return k
	}
	return quote(k)
}

// statement writes prefix, the value and suffix, the value on one line
// when the whole fits in width, and broken otherwise.
func statement(prefix string, v value, suffix string) string {
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
func forcesBreak(v value) bool {
	a, ok := v.(arrayValue)
	if !ok || len(a) < 2 {
		return false
	}
	_, objects := a[0].(objectValue)
	for _, e := range a {
		switch x := e.(type) {
		case objectValue:
			if !objects || x.size() < 2 {
				return false
			}
		case arrayValue:
			if objects || x.size() < 2 {
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
func write(b *strings.Builder, v value, indent string, used, after int) {
	flat := v.flat()
	if !forcesBreak(v) && used+utf8.RuneCountInString(flat)+after <= width {
		b.WriteString(flat)
		return
	}
	inner := indent + "  "
	switch x := v.(type) {
	case objectValue:
		if len(x) == 0 {
			b.WriteString(flat)
			return
		}
		b.WriteString("{\n")
		for _, m := range x {
			k := key(m.key) + ": "
			b.WriteString(inner + k)
			write(b, m.val, inner, len(inner)+utf8.RuneCountInString(k), 1)
			b.WriteString(",\n")
		}
		b.WriteString(indent + "}")
	case arrayValue:
		if len(x) == 0 {
			b.WriteString(flat)
			return
		}
		b.WriteString("[\n")
		for _, e := range x {
			b.WriteString(inner)
			write(b, e, inner, len(inner), 1)
			b.WriteString(",\n")
		}
		b.WriteString(indent + "]")
	default:
		b.WriteString(flat)
	}
}
