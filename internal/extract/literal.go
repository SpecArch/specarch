package extract

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// A page schema file is read in the subset of TypeScript that is also
// JSON5 (ADR-057): imports, comments, and one exported object literal,
// optionally followed by satisfies and a type. Its values are JSON's.

// object is an object literal, its keys in the order written.
type object struct {
	keys   []string
	values map[string]any
	lines  map[string]int
}

func (o *object) get(key string) (any, bool) {
	v, ok := o.values[key]
	return v, ok
}

// reference is a bare name used as a value, such as a hook the schema
// imports; it is not a literal.
type reference struct{ name string }

// numeral is a number as it was written.
type numeral struct{ text string }

type tokenKind int

const (
	tokenEnd tokenKind = iota
	tokenName
	tokenString
	tokenNumber
	tokenPunct
)

type token struct {
	kind tokenKind
	text string // the name, the punctuation, the number as written, or the string's value
	line int
}

// literalError is where and why a file is outside the subset.
type literalError struct {
	line   int
	reason string
}

func (e *literalError) Error() string { return fmt.Sprintf("line %d: %s", e.line, e.reason) }

func outside(line int, format string, args ...any) error {
	return &literalError{line: line, reason: fmt.Sprintf(format, args...)}
}

func tokenize(src string) ([]token, error) {
	var out []token
	line := 1
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
		case i == 0 && strings.HasPrefix(src, "\xef\xbb\xbf"):
			i += 3
		case strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, outside(line, "a comment is not closed")
			}
			line += strings.Count(src[i:i+2+end], "\n")
			i += 2 + end + 2
		case c == '"' || c == '\'':
			s, n, err := readString(src[i:], line)
			if err != nil {
				return nil, err
			}
			out = append(out, token{tokenString, s, line})
			i += n
		case c >= '0' && c <= '9' || c == '.' && i+1 < len(src) && src[i+1] >= '0' && src[i+1] <= '9':
			j := i
			for j < len(src) && (src[j] >= '0' && src[j] <= '9' || src[j] == '.' || src[j] == 'e' || src[j] == 'E' ||
				(src[j] == '+' || src[j] == '-') && j > i && (src[j-1] == 'e' || src[j-1] == 'E')) {
				j++
			}
			text := src[i:j]
			if _, err := strconv.ParseFloat(text, 64); err != nil || j < len(src) && isNameByte(src[j]) {
				return nil, outside(line, "%s is not a decimal number", strings.TrimSpace(src[i:min(j+1, len(src))]))
			}
			out = append(out, token{tokenNumber, text, line})
			i = j
		case isNameStart(c):
			j := i
			for j < len(src) && isNameByte(src[j]) {
				j++
			}
			out = append(out, token{tokenName, src[i:j], line})
			i = j
		case strings.ContainsRune("{}[](),:;.<>=+-*|&?!", rune(c)):
			out = append(out, token{tokenPunct, string(c), line})
			i++
		case c == '`':
			return nil, outside(line, "a template literal is not a JSON5 string")
		default:
			r, _ := utf8.DecodeRuneInString(src[i:])
			return nil, outside(line, "%q is not in the subset", r)
		}
	}
	return append(out, token{tokenEnd, "", line}), nil
}

func isNameStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == '$'
}

func isNameByte(c byte) bool { return isNameStart(c) || c >= '0' && c <= '9' }

// readString reads a quoted string with JSON5's escapes; it returns the
// value and the bytes read.
func readString(src string, line int) (string, int, error) {
	quote := src[0]
	var b strings.Builder
	for i := 1; i < len(src); {
		c := src[i]
		switch {
		case c == quote:
			return b.String(), i + 1, nil
		case c == '\n':
			return "", 0, outside(line, "a string is not closed on its line")
		case c == '\\':
			if i+1 >= len(src) {
				return "", 0, outside(line, "a string is not closed")
			}
			e := src[i+1]
			switch e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case '\\', '/', '"', '\'':
				b.WriteByte(e)
			case 'u':
				if i+6 > len(src) {
					return "", 0, outside(line, "a \\u escape needs four hexadecimal digits")
				}
				n, err := strconv.ParseUint(src[i+2:i+6], 16, 16)
				if err != nil {
					return "", 0, outside(line, "a \\u escape needs four hexadecimal digits")
				}
				b.WriteRune(rune(n))
				i += 4
			default:
				return "", 0, outside(line, "the escape \\%c is not one JSON5 and TypeScript read alike", e)
			}
			i += 2
		default:
			b.WriteByte(c)
			i++
		}
	}
	return "", 0, outside(line, "a string is not closed")
}

type literalParser struct {
	toks []token
	pos  int
}

func (p *literalParser) peek() token { return p.toks[p.pos] }

func (p *literalParser) next() token {
	t := p.toks[p.pos]
	if t.kind != tokenEnd {
		p.pos++
	}
	return t
}

func (p *literalParser) is(kind tokenKind, text string) bool {
	t := p.peek()
	return t.kind == kind && t.text == text
}

func (p *literalParser) expect(kind tokenKind, text, what string) error {
	if !p.is(kind, text) {
		return outside(p.peek().line, "%s is expected, and %s is found", what, describe(p.peek()))
	}
	p.next()
	return nil
}

func describe(t token) string {
	switch t.kind {
	case tokenEnd:
		return "the end of the file"
	case tokenString:
		return "a string"
	default:
		return strconv.Quote(t.text)
	}
}

// parseSchemaFile reads a page schema file and returns its one exported
// object literal.
func parseSchemaFile(src string) (*object, error) {
	toks, err := tokenize(src)
	if err != nil {
		return nil, err
	}
	p := &literalParser{toks: toks}
	var found *object
	for p.peek().kind != tokenEnd {
		t := p.peek()
		switch {
		case p.is(tokenPunct, ";"):
			p.next()
		case p.is(tokenName, "import"):
			if err := p.skipImport(); err != nil {
				return nil, err
			}
		case p.is(tokenName, "export"):
			if found != nil {
				return nil, outside(t.line, "a second export; the file holds one exported object literal")
			}
			o, err := p.export()
			if err != nil {
				return nil, err
			}
			found = o
		default:
			return nil, outside(t.line, "%s starts a statement that is neither an import nor the export", describe(t))
		}
	}
	if found == nil {
		return nil, outside(p.peek().line, "the file exports no object literal")
	}
	return found, nil
}

// skipImport passes over an import statement: import "x", or import ...
// from "x", with or without a semicolon.
func (p *literalParser) skipImport() error {
	start := p.next()
	if p.peek().kind == tokenString {
		p.next()
		return nil
	}
	for !p.is(tokenName, "from") {
		if p.peek().kind == tokenEnd || p.is(tokenPunct, ";") {
			return outside(start.line, "an import names no module with from")
		}
		p.next()
	}
	p.next()
	if p.peek().kind != tokenString {
		return outside(start.line, "an import's from is not followed by the module's name in quotes")
	}
	p.next()
	return nil
}

// export reads export const name = literal, or export default literal,
// each with satisfies and a type, or as const, after it.
func (p *literalParser) export() (*object, error) {
	start := p.next()
	switch {
	case p.is(tokenName, "default"):
		p.next()
	case p.is(tokenName, "const"):
		p.next()
		if p.peek().kind != tokenName {
			return nil, outside(start.line, "export const is not followed by a name")
		}
		p.next()
		if p.is(tokenPunct, ":") {
			p.next()
			if err := p.skipType(); err != nil {
				return nil, err
			}
		}
		if err := p.expect(tokenPunct, "=", "="); err != nil {
			return nil, err
		}
	default:
		return nil, outside(start.line, "the export is neither export const nor export default")
	}
	if !p.is(tokenPunct, "{") {
		return nil, outside(p.peek().line, "the export is not an object literal")
	}
	v, err := p.value()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.is(tokenName, "satisfies"):
			p.next()
			if err := p.skipType(); err != nil {
				return nil, err
			}
			continue
		case p.is(tokenName, "as"):
			p.next()
			if !p.is(tokenName, "const") {
				return nil, outside(p.peek().line, "as is followed by %s, and only as const is in the subset", describe(p.peek()))
			}
			p.next()
			continue
		}
		break
	}
	t := p.peek()
	if t.kind != tokenEnd && !p.is(tokenPunct, ";") && t.line == p.toks[p.pos-1].line {
		return nil, outside(t.line, "%s follows the exported literal", describe(t))
	}
	return v.(*object), nil
}

// skipType passes over a type name: names joined by dots, with type
// arguments in angle brackets, which may be literal types.
func (p *literalParser) skipType() error {
	line := p.peek().line
	if p.peek().kind != tokenName {
		return outside(line, "a type name is expected, and %s is found", describe(p.peek()))
	}
	p.next()
	for p.is(tokenPunct, ".") {
		p.next()
		if p.peek().kind != tokenName {
			return outside(line, "a type name is expected after a dot")
		}
		p.next()
	}
	if !p.is(tokenPunct, "<") {
		return nil
	}
	depth := 0
	for {
		t := p.next()
		switch {
		case t.kind == tokenEnd:
			return outside(line, "a type's arguments are not closed")
		case t.kind == tokenPunct && t.text == "<":
			depth++
		case t.kind == tokenPunct && t.text == ">":
			depth--
			if depth == 0 {
				return nil
			}
		case t.kind == tokenName || t.kind == tokenString || t.kind == tokenNumber || t.kind == tokenPunct && strings.Contains(".,[]|&", t.text):
		default:
			return outside(t.line, "%s is not part of a type name", describe(t))
		}
	}
}

// value reads one JSON5 value; a bare name is a reference.
func (p *literalParser) value() (any, error) {
	t := p.next()
	switch t.kind {
	case tokenString:
		return t.text, nil
	case tokenNumber:
		return numeral{t.text}, nil
	case tokenName:
		switch t.text {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "null":
			return nil, nil
		case "Infinity", "NaN", "undefined", "function", "new":
			return nil, outside(t.line, "%s is not a JSON value", t.text)
		}
		if p.is(tokenPunct, "(") || p.is(tokenPunct, ".") {
			return nil, outside(t.line, "%s is followed by %s; only a literal or a bare name is in the subset", t.text, describe(p.peek()))
		}
		return reference{t.text}, nil
	case tokenPunct:
		switch t.text {
		case "-", "+":
			n := p.next()
			if n.kind != tokenNumber {
				return nil, outside(t.line, "%s is not followed by a number", t.text)
			}
			if t.text == "-" {
				return numeral{"-" + n.text}, nil
			}
			return numeral{n.text}, nil
		case "[":
			var items []any
			for !p.is(tokenPunct, "]") {
				if p.is(tokenPunct, ".") {
					return nil, outside(p.peek().line, "a spread is not in the subset")
				}
				v, err := p.value()
				if err != nil {
					return nil, err
				}
				items = append(items, v)
				if p.is(tokenPunct, ",") {
					p.next()
					continue
				}
				if !p.is(tokenPunct, "]") {
					return nil, outside(p.peek().line, "a comma or ] is expected, and %s is found", describe(p.peek()))
				}
			}
			p.next()
			return items, nil
		case "{":
			o := &object{values: map[string]any{}, lines: map[string]int{}}
			for !p.is(tokenPunct, "}") {
				k := p.next()
				if k.kind != tokenName && k.kind != tokenString {
					return nil, outside(k.line, "a key is a name or a quoted string, and %s is found", describe(k))
				}
				if !p.is(tokenPunct, ":") {
					return nil, outside(k.line, "the key %s is not followed by a colon; a shorthand or a method is not in the subset", k.text)
				}
				p.next()
				if _, dup := o.values[k.text]; dup {
					return nil, outside(k.line, "the key %s is given twice", k.text)
				}
				v, err := p.value()
				if err != nil {
					return nil, err
				}
				o.keys = append(o.keys, k.text)
				o.values[k.text] = v
				o.lines[k.text] = k.line
				if p.is(tokenPunct, ",") {
					p.next()
					continue
				}
				if !p.is(tokenPunct, "}") {
					return nil, outside(p.peek().line, "a comma or } is expected, and %s is found", describe(p.peek()))
				}
			}
			p.next()
			return o, nil
		}
	}
	return nil, outside(t.line, "%s is not a value", describe(t))
}
