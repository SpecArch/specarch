package extract

import (
	"fmt"
	"go/ast"
	"sort"
	"strings"
)

// The import paths of golang-jwt, the JSON Web Token library the reader
// knows: the module and each major version after the first.
var jwtPackages = map[string]bool{
	"github.com/golang-jwt/jwt":    true,
	"github.com/golang-jwt/jwt/v4": true,
	"github.com/golang-jwt/jwt/v5": true,
}

// readMinted asks about every token the module mints with golang-jwt: a
// call of NewWithClaims, with the claims of a MapClaims literal it is
// given, or one the same function assigns to the name it is given, listed
// by their keys (ADR-154). A token is design (ADR-097): its format,
// lifetime, signature and checks are not in the call, so the reader
// declares no token and asks, listing what it found.
func (g *goReader) readMinted() {
	n := 0
	for _, f := range g.files {
		alias := ""
		for name, ip := range f.imports {
			if jwtPackages[ip] && (alias == "" || name < alias) {
				alias = name
			}
		}
		if alias == "" {
			continue
		}
		for _, d := range f.ast.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			maps := map[string]*ast.CompositeLit{}
			ast.Inspect(fd.Body, func(node ast.Node) bool {
				switch x := node.(type) {
				case *ast.AssignStmt:
					for i, lhs := range x.Lhs {
						id, ok := lhs.(*ast.Ident)
						if ok && i < len(x.Rhs) {
							if lit := mapClaims(x.Rhs[i], alias); lit != nil {
								maps[id.Name] = lit
							}
						}
					}
				case *ast.CallExpr:
					sel, ok := x.Fun.(*ast.SelectorExpr)
					if !ok || exprText(sel.X) != alias || sel.Sel.Name != "NewWithClaims" || len(x.Args) != 2 {
						return true
					}
					n++
					g.mintedAt(f, x, maps)
				}
				return true
			})
		}
	}
	if n > 0 {
		g.res.say("tokens: counted %s: every call of golang-jwt's NewWithClaims", plural(n, "minted token"))
	}
}

// mapClaims is the MapClaims literal an expression is, or nil.
func mapClaims(e ast.Expr, alias string) *ast.CompositeLit {
	lit, ok := e.(*ast.CompositeLit)
	if !ok || exprText(lit.Type) != alias+".MapClaims" {
		return nil
	}
	return lit
}

// mintedAt asks about one NewWithClaims call.
func (g *goReader) mintedAt(f *goFile, call *ast.CallExpr, maps map[string]*ast.CompositeLit) {
	clause := g.clause(f, call.Pos())
	f.cited = true
	arg := call.Args[1]
	lit := mapClaims(arg, strings.TrimSuffix(exprText(call.Fun), ".NewWithClaims"))
	if id, ok := arg.(*ast.Ident); ok && lit == nil {
		lit = maps[id.Name]
	}
	why := "A token is design: its format, lifetime, signature algorithm, claims and checks are declared under tokens (ADR-097), and a call that mints one shows the claims it sets and none of the rest."
	if lit == nil {
		g.question("must", fmt.Sprintf("%s mints a token with the claims %s, which is not a MapClaims literal, so the claims are not read. Which token is it, and which claims does it carry?", clause, exprText(arg)),
			[]string{"tokens"}, why, g.at(clause, "Mints a token with "+exprText(arg)+"."))
		return
	}
	var keys, unread []string
	for _, e := range lit.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if k, ok := g.stringValue(f, kv.Key); ok {
			if !contains(keys, k) {
				keys = append(keys, k)
			}
		} else {
			unread = append(unread, exprText(kv.Key))
		}
	}
	sort.Strings(keys)
	found := "no claim the reader reads"
	if len(keys) > 0 {
		found = "the claims " + joinAnd(keys)
	}
	if len(unread) > 0 {
		found += fmt.Sprintf(", and %s whose names are not literals (%s)", plural(len(unread), "claim"), strings.Join(unread, ", "))
	}
	g.question("must", fmt.Sprintf("%s mints a token with %s. Which token is it: its format, lifetime, signature algorithm, the type of each claim and the checks over them?", clause, found),
		[]string{"tokens"}, why, g.at(clause, "Mints a token with "+found+"."))
}
