// Package expr is SpecArch's expression subset of CEL: it parses with the
// CEL parser, refuses whatever is outside the subset, checks types and
// evaluates with exact numbers.
package expr

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/parser"
)

// Error is one problem in an expression. Line and Column count from 1,
// inside the expression text.
type Error struct {
	Line    int
	Column  int
	Kind    ErrorKind
	Message string
}

// ErrorKind sorts errors into the validator's rules.
type ErrorKind int

const (
	Syntax ErrorKind = iota
	Name
	TypeMismatch
)

// Node is one node of a parsed expression.
type Node struct {
	Op     string   // see the constants below
	Int    *big.Int // for OpInt and OpUint
	Double float64  // for OpDouble
	Text   string   // a string literal, a name, or a function name
	Bool   bool
	Args   []*Node
	Line   int
	Column int
}

// Node operations.
const (
	OpInt    = "int"
	OpUint   = "uint"
	OpDouble = "double"
	OpText   = "text"
	OpBool   = "bool"
	OpNull   = "null"
	OpName   = "name"
	OpCall   = "call" // Text is the function name
	OpNeg    = "-x"
	OpNot    = "!"
	OpCond   = "?:"
	// Binary operators use their own symbol: + - * / == != < <= > >= && ||
)

var binary = map[string]string{
	"_+_": "+", "_-_": "-", "_*_": "*", "_/_": "/",
	"_==_": "==", "_!=_": "!=", "_<_": "<", "_<=_": "<=", "_>_": ">", "_>=_": ">=",
	"_&&_": "&&", "_||_": "||",
}

// Functions lists the functions of the subset.
var Functions = map[string]bool{
	"int": true, "uint": true, "double": true, "decimal": true, "string": true,
	"date": true, "timestamp": true, "duration": true, "size": true, "round": true, "floor": true,
	"ceil": true, "min": true, "max": true,
}

// FunctionList names the functions for messages.
const FunctionList = "int, uint, double, decimal, string, date, timestamp, duration, size, round, floor, ceil, min and max"

var celParser = func() *parser.Parser {
	p, err := parser.NewParser()
	if err != nil {
		panic(err)
	}
	return p
}()

// Parse parses one expression and turns it into the subset's nodes. It
// refuses anything outside the subset by name.
func Parse(src string) (*Node, []Error) {
	if strings.TrimSpace(src) == "" {
		return nil, []Error{{Line: 1, Column: 1, Kind: Syntax, Message: "the expression is empty; write one"}}
	}
	tree, iss := celParser.Parse(common.NewTextSource(src))
	if iss != nil && len(iss.GetErrors()) > 0 {
		var errs []Error
		for _, e := range iss.GetErrors() {
			errs = append(errs, Error{Line: e.Location.Line(), Column: e.Location.Column() + 1, Kind: Syntax, Message: plainSyntax(e.Message)})
			break // the first syntax error is the one worth reading
		}
		return nil, errs
	}
	b := &builder{src: src, info: tree.SourceInfo()}
	n := b.node(tree.Expr())
	return n, b.errs
}

type builder struct {
	src  string
	info *ast.SourceInfo
	errs []Error
}

func (b *builder) at(e ast.Expr) (int, int) {
	loc := b.info.GetStartLocation(e.ID())
	return loc.Line(), loc.Column() + 1
}

func (b *builder) refuse(e ast.Expr, format string, args ...any) *Node {
	line, col := b.at(e)
	b.errs = append(b.errs, Error{Line: line, Column: col, Kind: Syntax, Message: fmt.Sprintf(format, args...)})
	return nil
}

func (b *builder) node(e ast.Expr) *Node {
	line, col := b.at(e)
	n := &Node{Line: line, Column: col}
	switch e.Kind() {
	case ast.LiteralKind:
		return b.literal(e, n)
	case ast.IdentKind:
		n.Op, n.Text = OpName, e.AsIdent()
		return n
	case ast.SelectKind:
		return b.refuse(e, "field access with . is not part of SpecArch expressions; name the field directly")
	case ast.ListKind:
		return b.refuse(e, "lists [ ] are not part of SpecArch expressions")
	case ast.MapKind, ast.StructKind:
		return b.refuse(e, "maps and objects { } are not part of SpecArch expressions")
	case ast.ComprehensionKind:
		return b.refuse(e, "macros such as all, exists, map and filter are not part of SpecArch expressions")
	case ast.CallKind:
		return b.call(e, n)
	}
	return b.refuse(e, "this is not part of SpecArch expressions")
}

func (b *builder) literal(e ast.Expr, n *Node) *Node {
	switch v := e.AsLiteral().Value().(type) {
	case bool:
		n.Op, n.Bool = OpBool, v
	case string:
		n.Op, n.Text = OpText, v
	case int64:
		n.Op, n.Int = OpInt, big.NewInt(v)
	case uint64:
		n.Op, n.Int = OpUint, new(big.Int).SetUint64(v)
	case float64:
		n.Op, n.Double = OpDouble, v
	case []byte:
		return b.refuse(e, "byte strings (written b\"...\") are not part of SpecArch expressions")
	default:
		if e.AsLiteral().Type().TypeName() == "null_type" {
			n.Op = OpNull
			return n
		}
		return b.refuse(e, "this value is not part of SpecArch expressions")
	}
	return n
}

func (b *builder) call(e ast.Expr, n *Node) *Node {
	c := e.AsCall()
	fn := c.FunctionName()
	if c.IsMemberFunction() {
		return b.refuse(e, "the method form x.%s() is not part of SpecArch expressions; write %s(x)", fn, fn)
	}
	args := c.Args()
	if op, ok := binary[fn]; ok {
		n.Op = op
		return b.withArgs(n, args)
	}
	switch fn {
	case "-_":
		n.Op = OpNeg
		return b.withArgs(n, args)
	case "!_":
		n.Op = OpNot
		return b.withArgs(n, args)
	case "_?_:_":
		n.Op = OpCond
		return b.withArgs(n, args)
	case "_%_":
		return b.refuse(e, "%% (remainder) is not part of SpecArch expressions")
	case "@in":
		return b.refuse(e, "in is not part of SpecArch expressions; compare with == and join with ||")
	case "_[_]":
		return b.refuse(e, "indexing with [ ] is not part of SpecArch expressions")
	}
	if strings.HasPrefix(fn, "_") || strings.HasPrefix(fn, "@") {
		return b.refuse(e, "the operator %s is not part of SpecArch expressions", strings.Trim(fn, "_@"))
	}
	if !Functions[fn] {
		return b.refuse(e, "%s is not a function of SpecArch expressions; the functions are %s", fn, FunctionList)
	}
	n.Op, n.Text = OpCall, fn
	return b.withArgs(n, args)
}

func (b *builder) withArgs(n *Node, args []ast.Expr) *Node {
	ok := true
	for _, a := range args {
		an := b.node(a)
		if an == nil {
			ok = false
		}
		n.Args = append(n.Args, an)
	}
	if !ok {
		return nil
	}
	return n
}

var (
	mismatchEOF = regexp.MustCompile(`^mismatched input '<EOF>' expecting .*$`)
	mismatch    = regexp.MustCompile(`^(?:mismatched|extraneous) input '([^']*)' expecting .*$`)
	tokenError  = regexp.MustCompile(`^token recognition error at: '([^']*)'$`)
	noViable    = regexp.MustCompile(`^no viable alternative at input '([^']*)'$`)
)

// plainSyntax rewrites a parser message into one plain sentence.
func plainSyntax(msg string) string {
	msg = strings.TrimPrefix(msg, "Syntax error: ")
	switch {
	case mismatchEOF.MatchString(msg):
		return "the expression ends where a value or a closing bracket was expected; complete it"
	case mismatch.MatchString(msg):
		return fmt.Sprintf("%s is not expected here; check the operators and brackets around it", mismatch.FindStringSubmatch(msg)[1])
	case tokenError.MatchString(msg):
		tok := tokenError.FindStringSubmatch(msg)[1]
		return fmt.Sprintf("%s is not a character SpecArch expressions use; write * for times, / for divided by, && for and, || for or", tok)
	case noViable.MatchString(msg):
		return fmt.Sprintf("the expression cannot be read near %s; check the operators and brackets", noViable.FindStringSubmatch(msg)[1])
	}
	return msg
}
