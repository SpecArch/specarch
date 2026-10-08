package extract

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// A check constraint as PostgreSQL prints it (pg_get_constraintdef) is read
// into a small tree and written in SpecArch's expression subset
// (docs/conventions.md, Expressions). Only what both say the same way is
// translated: columns, literals, comparisons, arithmetic, AND, OR, NOT, IS
// NULL, length and a list of allowed values. Anything else is refused with
// the reason, and the reader asks instead of guessing.

type sqlNode struct {
	op   string // "col", "num", "str", "bool", "null", "call", "any", "not", "isnull", "notnull", "neg", or a binary operator
	text string
	args []*sqlNode
	cast string // the type the value was cast to with ::, if any
}

type sqlParser struct {
	toks []string
	pos  int
	err  error
}

// parseCheck reads the definition "CHECK (<expression>)", with an optional
// trailing NOT VALID.
func parseCheck(def string) (*sqlNode, error) {
	toks, err := sqlTokens(def)
	if err != nil {
		return nil, err
	}
	p := &sqlParser{toks: toks}
	if !p.word("CHECK") || !p.take("(") {
		return nil, fmt.Errorf("it does not start with CHECK (")
	}
	n := p.or()
	if p.err != nil {
		return nil, p.err
	}
	if !p.take(")") {
		return nil, fmt.Errorf("it has more after the expression than the closing parenthesis")
	}
	if p.word("NO") {
		p.word("INHERIT")
	}
	if p.word("NOT") && !p.word("VALID") {
		return nil, fmt.Errorf("it ends in something other than NOT VALID")
	}
	if p.pos != len(p.toks) {
		return nil, fmt.Errorf("it has %q after the expression", p.toks[p.pos])
	}
	return n, nil
}

func sqlTokens(s string) ([]string, error) {
	var toks []string
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '\'':
			j := i + 1
			var b strings.Builder
			for {
				if j >= len(s) {
					return nil, fmt.Errorf("a quoted text is not closed")
				}
				if s[j] == '\'' {
					if j+1 < len(s) && s[j+1] == '\'' {
						b.WriteByte('\'')
						j += 2
						continue
					}
					break
				}
				b.WriteByte(s[j])
				j++
			}
			toks = append(toks, "'"+b.String())
			i = j + 1
		case c == '"':
			j := strings.IndexByte(s[i+1:], '"')
			if j < 0 {
				return nil, fmt.Errorf("a quoted name is not closed")
			}
			toks = append(toks, "\""+s[i+1:i+1+j])
			i += j + 2
		case c >= '0' && c <= '9':
			j := i
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == '.') {
				j++
			}
			toks = append(toks, s[i:j])
			i = j
		case c == '_' || unicode.IsLetter(rune(c)):
			j := i
			for j < len(s) && (s[j] == '_' || s[j] >= '0' && s[j] <= '9' || unicode.IsLetter(rune(s[j]))) {
				j++
			}
			toks = append(toks, s[i:j])
			i = j
		default:
			two := ""
			if i+1 < len(s) {
				two = s[i : i+2]
			}
			switch two {
			case "::", "<=", ">=", "<>", "!=":
				toks = append(toks, two)
				i += 2
				continue
			}
			if strings.IndexByte("()[],=<>+-*/", c) < 0 {
				return nil, fmt.Errorf("%q is not part of what is translated", string(c))
			}
			toks = append(toks, string(c))
			i++
		}
	}
	return toks, nil
}

func (p *sqlParser) peek() string {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return ""
}

func (p *sqlParser) take(t string) bool {
	if p.peek() == t {
		p.pos++
		return true
	}
	return false
}

// word takes a keyword, in any case.
func (p *sqlParser) word(w string) bool {
	if strings.EqualFold(p.peek(), w) && !strings.HasPrefix(p.peek(), "\"") {
		p.pos++
		return true
	}
	return false
}

func (p *sqlParser) fail(format string, args ...any) *sqlNode {
	if p.err == nil {
		p.err = fmt.Errorf(format, args...)
	}
	return &sqlNode{op: "null"}
}

func (p *sqlParser) or() *sqlNode {
	n := p.and()
	for p.err == nil && p.word("OR") {
		n = &sqlNode{op: "||", args: []*sqlNode{n, p.and()}}
	}
	return n
}

func (p *sqlParser) and() *sqlNode {
	n := p.not()
	for p.err == nil && p.word("AND") {
		n = &sqlNode{op: "&&", args: []*sqlNode{n, p.not()}}
	}
	return n
}

func (p *sqlParser) not() *sqlNode {
	if p.word("NOT") {
		return &sqlNode{op: "not", args: []*sqlNode{p.not()}}
	}
	return p.is()
}

func (p *sqlParser) is() *sqlNode {
	n := p.comparison()
	if p.word("IS") {
		switch {
		case p.word("NULL"):
			return &sqlNode{op: "isnull", args: []*sqlNode{n}}
		case p.word("NOT") && p.word("NULL"):
			return &sqlNode{op: "notnull", args: []*sqlNode{n}}
		}
		return p.fail("IS is translated only before NULL or NOT NULL")
	}
	return n
}

func (p *sqlParser) comparison() *sqlNode {
	n := p.sum()
	switch op := p.peek(); op {
	case "=", "<>", "!=", "<", "<=", ">", ">=":
		p.pos++
		if op == "=" && p.word("ANY") {
			return &sqlNode{op: "any", args: []*sqlNode{n, p.anyList()}}
		}
		r := p.sum()
		switch op {
		case "=":
			op = "=="
		case "<>":
			op = "!="
		}
		return &sqlNode{op: op, args: []*sqlNode{n, r}}
	}
	return n
}

// anyList reads (ARRAY[v, ...])::type[] after = ANY.
func (p *sqlParser) anyList() *sqlNode {
	if !p.take("(") {
		return p.fail("ANY is translated only with a list of values")
	}
	depth := 0
	for p.take("(") {
		depth++
	}
	if !p.word("ARRAY") || !p.take("[") {
		return p.fail("ANY is translated only with ARRAY[...] of values")
	}
	list := &sqlNode{op: "list"}
	for {
		list.args = append(list.args, p.cast(p.primary()))
		if p.take("]") {
			break
		}
		if !p.take(",") {
			return p.fail("the ARRAY of values is not closed")
		}
	}
	for ; depth > 0; depth-- {
		if !p.take(")") {
			return p.fail("a parenthesis around the ARRAY is not closed")
		}
		p.castSuffix()
	}
	p.castSuffix()
	if !p.take(")") {
		return p.fail("ANY ( is not closed")
	}
	return list
}

func (p *sqlParser) sum() *sqlNode {
	n := p.product()
	for p.err == nil && (p.peek() == "+" || p.peek() == "-") {
		op := p.toks[p.pos]
		p.pos++
		n = &sqlNode{op: op, args: []*sqlNode{n, p.product()}}
	}
	return n
}

func (p *sqlParser) product() *sqlNode {
	n := p.unary()
	for p.err == nil && (p.peek() == "*" || p.peek() == "/") {
		op := p.toks[p.pos]
		p.pos++
		n = &sqlNode{op: op, args: []*sqlNode{n, p.unary()}}
	}
	return n
}

func (p *sqlParser) unary() *sqlNode {
	if p.take("-") {
		return &sqlNode{op: "neg", args: []*sqlNode{p.unary()}}
	}
	return p.cast(p.primary())
}

// castSuffix reads ::type, with [] and a width, and returns the type.
func (p *sqlParser) castSuffix() string {
	cast := ""
	for p.take("::") {
		var words []string
		for {
			t := p.peek()
			if t == "" || !(unicode.IsLetter(rune(t[0])) || t[0] == '"' || t[0] == '_') || strings.EqualFold(t, "AND") || strings.EqualFold(t, "OR") || strings.EqualFold(t, "IS") {
				break
			}
			words = append(words, strings.ToLower(strings.TrimPrefix(t, "\"")))
			p.pos++
		}
		if p.take("(") {
			for p.peek() != ")" && p.peek() != "" {
				p.pos++
			}
			p.take(")")
		}
		for p.peek() == "[" && p.pos+1 < len(p.toks) && p.toks[p.pos+1] == "]" {
			p.pos += 2
			words = append(words, "[]")
		}
		cast = strings.Join(words, " ")
	}
	return cast
}

func (p *sqlParser) cast(n *sqlNode) *sqlNode {
	if c := p.castSuffix(); c != "" {
		n.cast = c
	}
	return n
}

func (p *sqlParser) primary() *sqlNode {
	t := p.peek()
	switch {
	case t == "":
		return p.fail("the expression ends too early")
	case t == "(":
		p.pos++
		n := p.or()
		if !p.take(")") {
			return p.fail("a parenthesis is not closed")
		}
		return n
	case strings.HasPrefix(t, "'"):
		p.pos++
		return &sqlNode{op: "str", text: t[1:]}
	case t[0] >= '0' && t[0] <= '9':
		p.pos++
		return &sqlNode{op: "num", text: t}
	case strings.EqualFold(t, "true") || strings.EqualFold(t, "false"):
		p.pos++
		return &sqlNode{op: "bool", text: strings.ToLower(t)}
	case strings.EqualFold(t, "null"):
		p.pos++
		return &sqlNode{op: "null"}
	}
	if t[0] == '"' || t[0] == '_' || unicode.IsLetter(rune(t[0])) {
		p.pos++
		name := strings.TrimPrefix(t, "\"")
		if p.take("(") {
			call := &sqlNode{op: "call", text: strings.ToLower(name)}
			if !p.take(")") {
				for {
					call.args = append(call.args, p.or())
					if p.take(")") {
						break
					}
					if !p.take(",") {
						return p.fail("the arguments of %s are not closed", name)
					}
				}
			}
			return call
		}
		return &sqlNode{op: "col", text: name}
	}
	return p.fail("%q is not part of what is translated", t)
}

// celWriter writes a sqlNode in the expression subset, given the table's
// columns: each column's field name and type.
type celWriter struct {
	fields map[string]*field // by column name
}

var precedence = map[string]int{"||": 1, "&&": 2, "==": 3, "!=": 3, "<": 3, "<=": 3, ">": 3, ">=": 3, "+": 4, "-": 4, "*": 5, "/": 5}

func (w *celWriter) write(n *sqlNode) (string, int, error) {
	switch n.op {
	case "col":
		f := w.fields[n.text]
		if f == nil {
			return "", 9, fmt.Errorf("%s is not a column of the table", n.text)
		}
		if n.cast != "" && !castKeeps(n.cast, f) {
			return "", 9, fmt.Errorf("the column %s is cast to %s, which the expression subset does not do", n.text, n.cast)
		}
		return f.name, 9, nil
	case "num":
		if n.cast == "numeric" || strings.Contains(n.text, ".") {
			places := 0
			if i := strings.IndexByte(n.text, '.'); i >= 0 {
				places = len(n.text) - i - 1
			}
			return fmt.Sprintf("decimal(%q, %d)", n.text, places), 9, nil
		}
		if n.cast != "" && n.cast != "integer" && n.cast != "bigint" && n.cast != "smallint" {
			return "", 9, fmt.Errorf("the number %s is cast to %s, which the expression subset does not do", n.text, n.cast)
		}
		return n.text, 9, nil
	case "str":
		switch n.cast {
		case "", "text", "character varying", "character", "bpchar", "varchar":
			return strconv.Quote(n.text), 9, nil
		case "date":
			return fmt.Sprintf("date(%s)", strconv.Quote(n.text)), 9, nil
		case "numeric":
			places := 0
			if i := strings.IndexByte(n.text, '.'); i >= 0 {
				places = len(n.text) - i - 1
			}
			return fmt.Sprintf("decimal(%s, %d)", strconv.Quote(n.text), places), 9, nil
		}
		return "", 9, fmt.Errorf("the text %s is cast to %s, which the expression subset does not do", strconv.Quote(n.text), n.cast)
	case "bool":
		return n.text, 9, nil
	case "null":
		return "null", 9, nil
	case "neg":
		s, prec, err := w.write(n.args[0])
		if err != nil {
			return "", 0, err
		}
		if prec < 9 {
			s = "(" + s + ")"
		}
		return "-" + s, 8, nil
	case "not":
		s, prec, err := w.write(n.args[0])
		if err != nil {
			return "", 0, err
		}
		if prec < 9 {
			s = "(" + s + ")"
		}
		return "!" + s, 8, nil
	case "isnull", "notnull":
		s, _, err := w.write(n.args[0])
		if err != nil {
			return "", 0, err
		}
		if n.op == "isnull" {
			return s + " == null", 3, nil
		}
		return s + " != null", 3, nil
	case "call":
		if (n.text == "length" || n.text == "char_length" || n.text == "character_length") && len(n.args) == 1 {
			s, _, err := w.write(n.args[0])
			if err != nil {
				return "", 0, err
			}
			return "size(" + s + ")", 9, nil
		}
		return "", 9, fmt.Errorf("the function %s is not part of what is translated", n.text)
	case "any":
		col, _, err := w.write(n.args[0])
		if err != nil {
			return "", 0, err
		}
		var parts []string
		for _, v := range n.args[1].args {
			s, _, err := w.write(v)
			if err != nil {
				return "", 0, err
			}
			parts = append(parts, col+" == "+s)
		}
		if len(parts) == 1 {
			return parts[0], 3, nil
		}
		return strings.Join(parts, " || "), 1, nil
	}
	prec, ok := precedence[n.op]
	if !ok {
		return "", 0, fmt.Errorf("%s is not part of what is translated", n.op)
	}
	l, lp, err := w.write(n.args[0])
	if err != nil {
		return "", 0, err
	}
	var r string
	var rp int
	if days := n.args[1]; (n.op == "+" || n.op == "-") && w.isDate(n.args[0]) && days.op == "num" && days.cast == "" && !strings.Contains(days.text, ".") {
		// PostgreSQL adds a number of days to a date.
		r, rp = fmt.Sprintf("duration(\"P%sD\")", days.text), 9
	} else if r, rp, err = w.write(n.args[1]); err != nil {
		return "", 0, err
	}
	if lp < prec || (prec == 3 && lp == 3) {
		l = "(" + l + ")"
	}
	if rp <= prec && !(rp == prec && (n.op == "&&" || n.op == "||")) {
		r = "(" + r + ")"
	}
	return l + " " + n.op + " " + r, prec, nil
}

// isDate says whether a node is a date: a date column, a quoted date, or
// a date moved by days.
func (w *celWriter) isDate(n *sqlNode) bool {
	switch n.op {
	case "col":
		f := w.fields[n.text]
		return f != nil && f.kind == "date" && (n.cast == "" || n.cast == "date")
	case "str":
		return n.cast == "date"
	case "+", "-":
		return w.isDate(n.args[0])
	}
	return false
}

// castKeeps says whether casting a column to a type leaves its value as the
// expression subset sees it: a text column to text, a number to its own
// kind of number.
func castKeeps(cast string, f *field) bool {
	switch cast {
	case "text", "character varying", "varchar", "bpchar", "character":
		return f.kind == "string"
	case "numeric":
		return f.kind == "decimal"
	case "integer", "bigint", "smallint":
		return f.kind == "int"
	case "date":
		return f.kind == "date"
	}
	return false
}

// anyValues is the column and the values of a check that is exactly
// "column = ANY (ARRAY[...])": the column's allowed values.
func anyValues(n *sqlNode) (string, []string, bool) {
	if n.op != "any" || n.args[0].op != "col" {
		return "", nil, false
	}
	var values []string
	for _, v := range n.args[1].args {
		if v.op != "str" {
			return "", nil, false
		}
		values = append(values, v.text)
	}
	return n.args[0].text, values, true
}
