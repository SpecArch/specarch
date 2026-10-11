// Package literal reads a TypeScript file that holds one object literal of
// data, in the subset of TypeScript that is also JSON5, and compares two
// such files by value (ADR-093). generate --check uses it to compare a
// page's schema built by hand with the one the target writes, so that
// formatting, comments, quotes and trailing commas do not count.
//
// The subset: line and block comments; import statements; one
// `export const <name> = <literal>`, optionally followed by
// `satisfies <type>` and a semicolon. Inside the literal: objects with
// bare or quoted keys, arrays, strings in double or single quotes with
// JSON's escapes and \', decimal numbers, true, false and null, trailing
// commas allowed. Anything else is refused with its line, never guessed.
package literal

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// File is one file of the subset, read.
type File struct {
	Imports   []string // each import statement, its tokens joined by one space
	Name      string   // the exported constant
	Satisfies string   // the type after satisfies, or ""
	Value     any      // *Object, []any, string, Number, bool or nil
}

// Object is an object literal, its keys in the order written.
type Object struct {
	Keys   []string
	Values map[string]any
}

// Number is a decimal number as written; two are equal when their values
// are, so 1, 1.0 and 1e0 are one number.
type Number struct {
	Text string
	rat  *big.Rat
}

// Error is a place where the file leaves the subset.
type Error struct {
	Line    int
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Message) }

type token struct {
	kind string // "punct", "string", "number", "ident"
	text string // the punctuation, the string's value, the number or the name
	line int
}

// Parse reads a file of the subset.
func Parse(src []byte) (*File, error) {
	toks, err := lex(string(src))
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	return p.file()
}

func lex(s string) ([]token, error) {
	var out []token
	line := 1
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case strings.HasPrefix(s[i:], "//"):
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case strings.HasPrefix(s[i:], "/*"):
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				return nil, &Error{line, "a block comment is not closed"}
			}
			line += strings.Count(s[i:i+2+end], "\n")
			i += end + 4
		case c == '"' || c == '\'':
			text, n, err := quoted(s[i:], line)
			if err != nil {
				return nil, err
			}
			out = append(out, token{"string", text, line})
			i += n
		case c == '-' || c == '+' || c == '.' || c >= '0' && c <= '9':
			j := i
			if c == '-' || c == '+' {
				j++
			}
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == '.' || s[j] == 'e' || s[j] == 'E' ||
				(s[j] == '-' || s[j] == '+') && (s[j-1] == 'e' || s[j-1] == 'E')) {
				j++
			}
			out = append(out, token{"number", s[i:j], line})
			i = j
		case strings.ContainsRune("{}[]:,;=*", rune(c)):
			out = append(out, token{"punct", string(c), line})
			i++
		case c == '_' || c == '$' || c < utf8.RuneSelf && unicode.IsLetter(rune(c)):
			j := i
			for j < len(s) && (s[j] == '_' || s[j] == '$' || s[j] == '.' || s[j] < utf8.RuneSelf && (unicode.IsLetter(rune(s[j])) || unicode.IsDigit(rune(s[j])))) {
				j++
			}
			out = append(out, token{"ident", s[i:j], line})
			i = j
		default:
			r, _ := utf8.DecodeRuneInString(s[i:])
			return nil, &Error{line, fmt.Sprintf("%q is outside the object-literal subset", r)}
		}
	}
	return out, nil
}

// quoted reads a string in double or single quotes at the start of s and
// returns its value and its length in s.
func quoted(s string, line int) (string, int, error) {
	q := s[0]
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case c == q:
			return b.String(), i + 1, nil
		case c == '\n':
			return "", 0, &Error{line, "a string is not closed on its line"}
		case c == '\\':
			if i+1 >= len(s) {
				return "", 0, &Error{line, "a string ends in a backslash"}
			}
			i++
			switch e := s[i]; e {
			case '"', '\'', '\\', '/':
				b.WriteByte(e)
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				if i+4 >= len(s) {
					return "", 0, &Error{line, `a \u escape needs four hexadecimal digits`}
				}
				n, err := strconv.ParseUint(s[i+1:i+5], 16, 16)
				if err != nil {
					return "", 0, &Error{line, fmt.Sprintf(`\u%s is not four hexadecimal digits`, s[i+1:i+5])}
				}
				b.WriteRune(rune(n))
				i += 4
			default:
				return "", 0, &Error{line, fmt.Sprintf(`the escape \%c is outside the object-literal subset`, e)}
			}
		default:
			b.WriteByte(c)
		}
	}
	return "", 0, &Error{line, "a string is not closed"}
}

type parser struct {
	toks []token
	i    int
}

func (p *parser) peek() *token {
	if p.i < len(p.toks) {
		return &p.toks[p.i]
	}
	return nil
}

func (p *parser) line() int {
	if t := p.peek(); t != nil {
		return t.line
	}
	if len(p.toks) > 0 {
		return p.toks[len(p.toks)-1].line
	}
	return 1
}

func (p *parser) fail(format string, args ...any) error {
	return &Error{p.line(), fmt.Sprintf(format, args...)}
}

func (p *parser) found() string {
	t := p.peek()
	switch {
	case t == nil:
		return "the end of the file"
	case t.kind == "string":
		return "the string " + strconv.Quote(t.text)
	default:
		return strconv.Quote(t.text)
	}
}

func (p *parser) is(kind, text string) bool {
	t := p.peek()
	return t != nil && t.kind == kind && t.text == text
}

func (p *parser) expect(kind, text string) error {
	if !p.is(kind, text) {
		return p.fail("expected %q, found %s", text, p.found())
	}
	p.i++
	return nil
}

func (p *parser) file() (*File, error) {
	f := &File{}
	exported := false
	for p.peek() != nil {
		switch {
		case p.is("ident", "import"):
			start := p.i
			for p.peek() != nil && !p.is("punct", ";") {
				p.i++
			}
			if p.peek() == nil {
				return nil, p.fail("an import statement does not end in a semicolon")
			}
			var words []string
			for _, t := range p.toks[start:p.i] {
				if t.kind == "string" {
					words = append(words, strconv.Quote(t.text))
				} else {
					words = append(words, t.text)
				}
			}
			p.i++
			f.Imports = append(f.Imports, strings.Join(words, " "))
		case p.is("ident", "export") && !exported:
			p.i++
			if err := p.expect("ident", "const"); err != nil {
				return nil, err
			}
			t := p.peek()
			if t == nil || t.kind != "ident" {
				return nil, p.fail("expected the constant's name, found %s", p.found())
			}
			f.Name = t.text
			p.i++
			if err := p.expect("punct", "="); err != nil {
				return nil, err
			}
			if !p.is("punct", "{") {
				return nil, p.fail("the exported constant holds %s, not an object literal", p.found())
			}
			v, err := p.value()
			if err != nil {
				return nil, err
			}
			f.Value = v
			if p.is("ident", "satisfies") {
				p.i++
				t := p.peek()
				if t == nil || t.kind != "ident" {
					return nil, p.fail("expected a type name after satisfies, found %s", p.found())
				}
				f.Satisfies = t.text
				p.i++
			}
			if p.is("punct", ";") {
				p.i++
			}
			exported = true
		default:
			return nil, p.fail("found %s; the subset holds import statements and one export const of an object literal", p.found())
		}
	}
	if !exported {
		return nil, &Error{p.line(), "no export const of an object literal"}
	}
	return f, nil
}

func (p *parser) value() (any, error) {
	t := p.peek()
	if t == nil {
		return nil, p.fail("expected a value, found the end of the file")
	}
	switch {
	case t.kind == "punct" && t.text == "{":
		p.i++
		o := &Object{Values: map[string]any{}}
		for !p.is("punct", "}") {
			k := p.peek()
			if k == nil || k.kind != "string" && k.kind != "ident" || k.kind == "ident" && strings.Contains(k.text, ".") {
				return nil, p.fail("expected a key, found %s", p.found())
			}
			if _, dup := o.Values[k.text]; dup {
				return nil, p.fail("the key %q is given twice", k.text)
			}
			p.i++
			if err := p.expect("punct", ":"); err != nil {
				return nil, err
			}
			v, err := p.value()
			if err != nil {
				return nil, err
			}
			o.Keys = append(o.Keys, k.text)
			o.Values[k.text] = v
			if !p.is("punct", ",") {
				break
			}
			p.i++
		}
		if err := p.expect("punct", "}"); err != nil {
			return nil, err
		}
		return o, nil
	case t.kind == "punct" && t.text == "[":
		p.i++
		items := []any{}
		for !p.is("punct", "]") {
			v, err := p.value()
			if err != nil {
				return nil, err
			}
			items = append(items, v)
			if !p.is("punct", ",") {
				break
			}
			p.i++
		}
		if err := p.expect("punct", "]"); err != nil {
			return nil, err
		}
		return items, nil
	case t.kind == "string":
		p.i++
		return t.text, nil
	case t.kind == "number":
		r, ok := new(big.Rat).SetString(t.text)
		if !ok || strings.HasPrefix(t.text, "+") {
			return nil, p.fail("%s is not a decimal number", t.text)
		}
		p.i++
		return Number{Text: t.text, rat: r}, nil
	case t.kind == "ident" && (t.text == "true" || t.text == "false"):
		p.i++
		return t.text == "true", nil
	case t.kind == "ident" && t.text == "null":
		p.i++
		return nil, nil
	case t.kind == "ident":
		return nil, p.fail("the name %s is a reference, not data; the subset holds only literals", t.text)
	}
	return nil, p.fail("expected a value, found %s", p.found())
}

// Difference is one value two files disagree on.
type Difference struct {
	Path      string // where, such as "columns[2].sortable"
	Disk      string // the value in the file on disk, or "" when it has none
	Generated string // the value the target writes, or "" when it has none
}

func (d Difference) String() string {
	switch {
	case d.Disk == "":
		return fmt.Sprintf("%s: not on disk; generate writes %s", d.Path, d.Generated)
	case d.Generated == "":
		return fmt.Sprintf("%s: %s on disk; generate writes nothing there", d.Path, d.Disk)
	}
	return fmt.Sprintf("%s: %s on disk; generate writes %s", d.Path, d.Disk, d.Generated)
}

// Compare lists what the file on disk and the generated one disagree on:
// the imports, the constant's name, the type it satisfies, then each value
// of the literal in the generated file's order, keys only on disk last.
func Compare(disk, generated *File) []Difference {
	var out []Difference
	if strings.Join(disk.Imports, "\n") != strings.Join(generated.Imports, "\n") {
		out = append(out, Difference{"imports", orNone(strings.Join(disk.Imports, "; ")), orNone(strings.Join(generated.Imports, "; "))})
	}
	if disk.Name != generated.Name {
		out = append(out, Difference{"export const", disk.Name, generated.Name})
	}
	if disk.Satisfies != generated.Satisfies {
		out = append(out, Difference{"satisfies", orNone(disk.Satisfies), orNone(generated.Satisfies)})
	}
	return compareValue(out, "", disk.Value, generated.Value)
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

func compareValue(out []Difference, path string, d, g any) []Difference {
	switch gv := g.(type) {
	case *Object:
		dv, ok := d.(*Object)
		if !ok {
			break
		}
		for _, k := range gv.Keys {
			if dval, has := dv.Values[k]; has {
				out = compareValue(out, keyPath(path, k), dval, gv.Values[k])
			} else {
				out = append(out, Difference{keyPath(path, k), "", Render(gv.Values[k])})
			}
		}
		for _, k := range dv.Keys {
			if _, has := gv.Values[k]; !has {
				out = append(out, Difference{keyPath(path, k), Render(dv.Values[k]), ""})
			}
		}
		return out
	case []any:
		dv, ok := d.([]any)
		if !ok {
			break
		}
		for i := 0; i < len(gv) || i < len(dv); i++ {
			p := fmt.Sprintf("%s[%d]", path, i)
			switch {
			case i >= len(dv):
				out = append(out, Difference{p, "", Render(gv[i])})
			case i >= len(gv):
				out = append(out, Difference{p, Render(dv[i]), ""})
			default:
				out = compareValue(out, p, dv[i], gv[i])
			}
		}
		return out
	}
	if !equal(d, g) {
		out = append(out, Difference{orRoot(path), Render(d), Render(g)})
	}
	return out
}

func orRoot(path string) string {
	if path == "" {
		return "the literal"
	}
	return path
}

func equal(a, b any) bool {
	switch av := a.(type) {
	case Number:
		bv, ok := b.(Number)
		return ok && av.rat.Cmp(bv.rat) == 0
	case *Object, []any:
		return false // compared part by part above
	}
	return a == b
}

// keyPath names a key below path: .name for a plain name, ["a-b"] otherwise.
func keyPath(path, key string) string {
	plain := key != ""
	for i, r := range key {
		if !(r == '_' || r == '$' || unicode.IsLetter(r) || i > 0 && unicode.IsDigit(r)) {
			plain = false
		}
	}
	switch {
	case !plain:
		return path + "[" + strconv.Quote(key) + "]"
	case path == "":
		return key
	}
	return path + "." + key
}

// Render writes a value as compact JSON, numbers as written.
func Render(v any) string {
	switch x := v.(type) {
	case *Object:
		parts := make([]string, len(x.Keys))
		for i, k := range x.Keys {
			parts[i] = strconv.Quote(k) + ": " + Render(x.Values[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = Render(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case Number:
		return x.Text
	case nil:
		return "null"
	}
	data, _ := json.Marshal(v)
	return string(data)
}
