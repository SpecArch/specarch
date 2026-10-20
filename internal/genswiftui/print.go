package genswiftui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/SpecArch/specarch/internal/genscreens"
)

// A schema is printed as one Swift literal: the top value as its type's
// initialiser, a nested object as .init, whose type the compiler infers, a
// dictionary as a Swift dictionary, an array and a rule as array literals.
// It is laid out on one line when it fits in width, and otherwise one
// argument or element a line.

// width is the longest line the printer writes where a value can be
// broken; a string is never broken, so a line holding a long one is
// longer.
const width = 140

// flat is a value on one line; typeName is the top object's type, or ""
// for .init.
func flat(v value, typeName string) string {
	switch x := v.(type) {
	case genscreens.Literal:
		return scalar(x)
	case genscreens.ObjectValue:
		if x.Dictionary {
			if len(x.Members) == 0 {
				return "[:]"
			}
			parts := make([]string, len(x.Members))
			for i, m := range x.Members {
				parts[i] = quote(m.Key) + ": " + flat(m.Val, "")
			}
			return "[" + strings.Join(parts, ", ") + "]"
		}
		parts := make([]string, len(x.Members))
		for i, m := range x.Members {
			parts[i] = label(m.Key) + ": " + flat(m.Val, "")
		}
		return opener(typeName) + strings.Join(parts, ", ") + ")"
	case genscreens.ArrayValue:
		parts := make([]string, len(x.Items))
		for i, e := range x.Items {
			parts[i] = flat(e, "")
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return ""
}

// opener starts an object: its type's initialiser, or .init.
func opener(typeName string) string {
	if typeName == "" {
		return ".init("
	}
	return typeName + "("
}

// scalar is a literal as Swift writes it: a string quoted, null as nil,
// a number, true or false as given.
func scalar(l genscreens.Literal) string {
	if l.String {
		return quote(l.Text)
	}
	if l.Text == "null" {
		return "nil"
	}
	return l.Text
}

// quote writes a string as a Swift string literal in double quotes.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case 0:
			b.WriteString(`\0`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u{%X}`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// swiftKeywords are the words an argument label must be written in
// backticks for.
var swiftKeywords = map[string]bool{
	"as": true, "associatedtype": true, "break": true, "case": true, "catch": true, "class": true, "continue": true, "default": true,
	"defer": true, "deinit": true, "do": true, "else": true, "enum": true, "extension": true, "fallthrough": true, "false": true,
	"fileprivate": true, "for": true, "func": true, "guard": true, "if": true, "import": true, "in": true, "init": true, "inout": true,
	"internal": true, "is": true, "let": true, "nil": true, "operator": true, "precedencegroup": true, "private": true, "protocol": true,
	"public": true, "repeat": true, "rethrows": true, "return": true, "self": true, "Self": true, "static": true, "struct": true,
	"subscript": true, "super": true, "switch": true, "throw": true, "throws": true, "true": true, "try": true, "typealias": true,
	"var": true, "where": true, "while": true,
}

// label writes an argument label, in backticks when it is a keyword.
func label(k string) string {
	if swiftKeywords[k] {
		return "`" + k + "`"
	}
	return k
}

// statement writes prefix, the value and suffix, the value on one line
// when the whole fits in width, and broken otherwise.
func statement(prefix string, v value, typeName, suffix string) string {
	var b strings.Builder
	b.WriteString(prefix)
	write(&b, v, typeName, "", utf8.RuneCountInString(prefix), utf8.RuneCountInString(suffix))
	b.WriteString(suffix)
	b.WriteString("\n")
	return b.String()
}

// forcesBreak says whether an array is always broken, one element a line:
// two or more elements, every one an object of more than one key or every
// one an array of more than one element.
func forcesBreak(v value) bool {
	a, ok := v.(genscreens.ArrayValue)
	if !ok || len(a.Items) < 2 {
		return false
	}
	_, objects := a.Items[0].(genscreens.ObjectValue)
	for _, e := range a.Items {
		switch x := e.(type) {
		case genscreens.ObjectValue:
			if !objects || len(x.Members) < 2 {
				return false
			}
		case genscreens.ArrayValue:
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
func write(b *strings.Builder, v value, typeName, indent string, used, after int) {
	one := flat(v, typeName)
	if !forcesBreak(v) && used+utf8.RuneCountInString(one)+after <= width {
		b.WriteString(one)
		return
	}
	inner := indent + "    "
	switch x := v.(type) {
	case genscreens.ObjectValue:
		if len(x.Members) == 0 {
			b.WriteString(one)
			return
		}
		open, close := opener(typeName), ")"
		if x.Dictionary {
			open, close = "[", "]"
		}
		b.WriteString(open + "\n")
		for i, m := range x.Members {
			k := label(m.Key) + ": "
			if x.Dictionary {
				k = quote(m.Key) + ": "
			}
			b.WriteString(inner + k)
			write(b, m.Val, "", inner, len(inner)+utf8.RuneCountInString(k), 1)
			if i < len(x.Members)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(indent + close)
	case genscreens.ArrayValue:
		if len(x.Items) == 0 {
			b.WriteString(one)
			return
		}
		b.WriteString("[\n")
		for i, e := range x.Items {
			b.WriteString(inner)
			write(b, e, "", inner, len(inner), 1)
			if i < len(x.Items)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(indent + "]")
	default:
		b.WriteString(one)
	}
}
