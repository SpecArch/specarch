package extract

import (
	"fmt"
	"go/ast"
	gotoken "go/token"
	"path"
	"strconv"
	"strings"
)

// A package-level string constant and a package-level list, as the route
// reader resolves them (ADR-143). A constant of a string type is a
// compile-time value, so Go gives it as surely as a literal; a list is a
// var, which Go lets any statement of the module change, in its own
// package by its name and in another through the import, so it is read
// only when no statement of the module assigns to it, appends to it or
// takes its address.

// goConst is one package-level constant: the file that declares it and
// its value expression, read as a string when the expression is one.
type goConst struct {
	file *goFile
	expr ast.Expr
}

// goList is one package-level var holding a composite literal: the file
// that declares it, the literal, and what stops the reader from reading it
// as a literal list, "" when nothing does.
type goList struct {
	file    *goFile
	lit     *ast.CompositeLit
	stopped string
}

// collectPackageValues records, per folder, every package-level constant
// and every package-level var whose value is a composite literal, then
// scans every file of the module for the statements that change a var.
func (g *goReader) collectPackageValues() {
	for _, f := range g.files {
		dir := path.Dir(f.path)
		for _, d := range f.ast.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			switch gd.Tok {
			case gotoken.CONST:
				// A spec with no value repeats the one before it, as a
				// const block lets it.
				var last []ast.Expr
				for _, s := range gd.Specs {
					vs, ok := s.(*ast.ValueSpec)
					if !ok {
						continue
					}
					values := vs.Values
					if len(values) == 0 {
						values = last
					}
					last = values
					for i, n := range vs.Names {
						if i < len(values) && n.Name != "_" {
							if g.consts[dir] == nil {
								g.consts[dir] = map[string]*goConst{}
							}
							g.consts[dir][n.Name] = &goConst{file: f, expr: values[i]}
						}
					}
				}
			case gotoken.VAR:
				for _, s := range gd.Specs {
					vs, ok := s.(*ast.ValueSpec)
					if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
						continue
					}
					if g.lists[dir] == nil {
						g.lists[dir] = map[string]*goList{}
					}
					l := &goList{file: f}
					if cl, ok := vs.Values[0].(*ast.CompositeLit); ok {
						l.lit = cl
					} else {
						l.stopped = "is a package-level var whose value is not a literal list"
					}
					g.lists[dir][vs.Names[0].Name] = l
				}
			}
		}
	}
	if len(g.lists) == 0 {
		return
	}
	for _, f := range g.files {
		g.scanListChanges(f)
	}
}

// scanListChanges marks each list that a statement of the file assigns
// to, declares again, appends to or takes the address of, with the first
// such place: a list of the file's own package by its name, one of
// another package of the module through the import name the file gives
// it. A name declared again in a function counts, since the reader does
// not resolve which declaration a use names.
func (g *goReader) scanListChanges(f *goFile) {
	own := path.Dir(f.path)
	stop := func(dir, name string, pos gotoken.Pos, what string) {
		if l := g.lists[dir][name]; l != nil && l.stopped == "" && l.lit != nil {
			l.stopped = fmt.Sprintf("is a package-level list %s at %s", what, g.clause(f, pos))
		}
	}
	// target is the folder and name of the package-level list an
	// expression changes, through an index, a slice or parentheses.
	target := func(e ast.Expr) (string, string, bool) {
		for {
			switch x := e.(type) {
			case *ast.Ident:
				return own, x.Name, true
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok {
					if dir := g.dirOf(f.imports[id.Name]); dir != "" {
						return dir, x.Sel.Name, true
					}
				}
				return "", "", false
			case *ast.IndexExpr:
				e = x.X
			case *ast.SliceExpr:
				e = x.X
			case *ast.ParenExpr:
				e = x.X
			default:
				return "", "", false
			}
		}
	}
	for _, d := range f.ast.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Body == nil {
				continue
			}
		case *ast.GenDecl:
			if d.Tok != gotoken.VAR {
				continue
			}
		default:
			continue
		}
		ast.Inspect(d, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				for _, lhs := range n.Lhs {
					if dir, name, ok := target(lhs); ok {
						what := "assigned again"
						if n.Tok == gotoken.DEFINE {
							what = "declared again"
						}
						stop(dir, name, n.Pos(), what)
					}
				}
			case *ast.RangeStmt:
				if n.Tok == gotoken.ASSIGN {
					for _, e := range []ast.Expr{n.Key, n.Value} {
						if e != nil {
							if dir, name, ok := target(e); ok {
								stop(dir, name, n.Pos(), "assigned again")
							}
						}
					}
				}
			case *ast.IncDecStmt:
				if dir, name, ok := target(n.X); ok {
					stop(dir, name, n.Pos(), "assigned again")
				}
			case *ast.CallExpr:
				if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "append" && len(n.Args) > 0 {
					if dir, name, ok := target(n.Args[0]); ok {
						stop(dir, name, n.Pos(), "appended to")
					}
				}
			case *ast.UnaryExpr:
				if n.Op == gotoken.AND {
					if dir, name, ok := target(n.X); ok {
						stop(dir, name, n.Pos(), "whose address is taken")
					}
				}
			case *ast.ValueSpec:
				// A var declared again in a function, as the reader does
				// not resolve which declaration a use names.
				if _, inFunc := d.(*ast.FuncDecl); inFunc {
					for _, id := range n.Names {
						stop(own, id.Name, n.Pos(), "declared again")
					}
				}
			}
			return true
		})
	}
}

// constOf is the package-level constant an identifier or an imported
// selector names: one of the file's own package, or of another package of
// the module through its import.
func (g *goReader) constOf(f *goFile, e ast.Expr) *goConst {
	switch e := e.(type) {
	case *ast.Ident:
		return g.consts[path.Dir(f.path)][e.Name]
	case *ast.SelectorExpr:
		if x, ok := e.X.(*ast.Ident); ok {
			if dir := g.dirOf(f.imports[x.Name]); dir != "" {
				return g.consts[dir][e.Sel.Name]
			}
		}
	}
	return nil
}

// listOf is the package-level list an identifier or an imported selector
// names, as constOf finds a constant.
func (g *goReader) listOf(f *goFile, e ast.Expr) *goList {
	switch e := e.(type) {
	case *ast.Ident:
		return g.lists[path.Dir(f.path)][e.Name]
	case *ast.SelectorExpr:
		if x, ok := e.X.(*ast.Ident); ok {
			if dir := g.dirOf(f.imports[x.Name]); dir != "" {
				return g.lists[dir][e.Sel.Name]
			}
		}
	}
	return nil
}

// constString is a string literal, literals and constants joined by +, or
// a package-level constant whose value is one, followed through at most
// eight constants, each read in the file that declares it.
func (g *goReader) constString(f *goFile, e ast.Expr, depth int) (string, bool) {
	if depth > 8 {
		return "", false
	}
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == gotoken.STRING {
			s, err := strconv.Unquote(x.Value)
			return s, err == nil
		}
	case *ast.ParenExpr:
		return g.constString(f, x.X, depth+1)
	case *ast.BinaryExpr:
		if x.Op == gotoken.ADD {
			a, okA := g.constString(f, x.X, depth+1)
			b, okB := g.constString(f, x.Y, depth+1)
			return a + b, okA && okB
		}
	case *ast.Ident, *ast.SelectorExpr:
		if c := g.constOf(f, x); c != nil {
			if s, ok := g.constString(c.file, c.expr, depth+1); ok {
				g.citeFile(c.file)
				return s, true
			}
		}
	}
	return "", false
}

// constMethod is a method as a string literal, a net/http constant, or a
// package-level constant whose value is a method, read in the file that
// declares it.
func (g *goReader) constMethod(f *goFile, e ast.Expr) (string, bool) {
	if m, ok := g.methodValue(f, e); ok {
		return m, true
	}
	if s, ok := g.constString(f, e, 0); ok {
		return s, routeMethod.MatchString(strings.ToUpper(s))
	}
	return "", false
}

// methodList is a package-level list read as the literal list of methods
// it holds, each a string literal, a string constant or a net/http method
// constant, as string literals at the list's place; found is false when
// the name is no package-level list at all, and why says what stopped the
// reader.
func (g *goReader) methodList(f *goFile, e ast.Expr) (list []ast.Expr, why string, found bool) {
	l := g.listOf(f, e)
	if l == nil {
		return nil, "", false
	}
	if l.stopped != "" {
		return nil, l.stopped, true
	}
	var out []ast.Expr
	for _, el := range l.lit.Elts {
		m, ok := g.constMethod(l.file, el)
		if !ok {
			return nil, fmt.Sprintf("is a package-level list holding %s, which is not a method literal, a string constant or a net/http method constant", exprText(el)), true
		}
		out = append(out, &ast.BasicLit{Kind: gotoken.STRING, Value: strconv.Quote(m), ValuePos: el.Pos()})
	}
	g.citeFile(l.file)
	return out, "", true
}
