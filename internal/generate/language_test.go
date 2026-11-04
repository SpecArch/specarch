package generate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// texts is every text this package's code gives tr, with the files that
// give it.
func texts(t *testing.T) map[string][]string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) != 1 {
				return true
			}
			if id, ok := c.Fun.(*ast.Ident); !ok || id.Name != "tr" {
				return true
			}
			if lit, ok := c.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				s, _ := strconv.Unquote(lit.Value)
				out[s] = append(out[s], file)
			}
			return true
		})
	}
	return out
}

// TestCatalogsComplete requires of every language's catalog that it
// gives every text the code writes in a document, translated or kept,
// and nothing the code no longer writes.
func TestCatalogsComplete(t *testing.T) {
	used := texts(t)
	for _, tag := range Languages()[1:] {
		c, err := LoadCatalog(tag)
		if err != nil {
			t.Fatal(err)
		}
		var missing, stale []string
		for s, files := range used {
			if !HasWords(s) {
				continue
			}
			if _, ok := c.Translations[s]; !ok && !c.Same[s] {
				missing = append(missing, files[0]+": "+strconv.Quote(s))
			}
		}
		for s := range c.Translations {
			if _, ok := used[s]; !ok && !c.Dynamic[s] {
				stale = append(stale, strconv.Quote(s))
			}
		}
		for s := range c.Same {
			if _, ok := used[s]; !ok {
				stale = append(stale, strconv.Quote(s))
			}
		}
		sort.Strings(missing)
		sort.Strings(stale)
		if len(missing) > 0 {
			t.Errorf("the catalog of %s gives no text for %d texts the documents write:\n%s", tag, len(missing), strings.Join(missing, "\n"))
		}
		if len(stale) > 0 {
			t.Errorf("the catalog of %s gives %d texts no document writes:\n%s", tag, len(stale), strings.Join(stale, "\n"))
		}
	}
}

var indexedVerb = regexp.MustCompile(`^%\[(\d+)\]`)

// verbs is the fmt verbs of a text by the argument each formats, from 1;
// a verb without an index takes the argument after the one before it.
func verbs(s string) map[int]string {
	out := map[int]string{}
	next := 1
	for _, v := range Verb.FindAllString(s, -1) {
		if v == "%%" {
			continue
		}
		at := next
		if m := indexedVerb.FindStringSubmatch(v); m != nil {
			at, _ = strconv.Atoi(m[1])
		}
		out[at] = v[len(v)-1:]
		next = at + 1
	}
	return out
}

// TestCatalogVerbs requires a translation to format the same arguments
// with the same verbs as its English text, in any order, so that no
// translation garbles or drops a value.
func TestCatalogVerbs(t *testing.T) {
	for _, tag := range Languages()[1:] {
		c, err := LoadCatalog(tag)
		if err != nil {
			t.Fatal(err)
		}
		for en, other := range c.Translations {
			want, got := verbs(en), verbs(other)
			if len(want) != len(got) {
				t.Errorf("%s: %q formats %d values, its translation %q %d", tag, en, len(want), other, len(got))
				continue
			}
			for i, v := range want {
				if got[i] != v {
					t.Errorf("%s: %q formats value %d with %%%s, its translation %q with %q", tag, en, i, v, other, got[i])
				}
			}
		}
	}
}

// TestNoTrAtStartUp refuses a call of tr outside a function, in a
// package variable: it runs when the program starts, before a language
// is chosen, so its text stays English in every document.
func TestNoTrAtStartUp(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			g, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			ast.Inspect(g, func(n ast.Node) bool {
				if _, ok := n.(*ast.FuncLit); ok {
					return false
				}
				if c, ok := n.(*ast.CallExpr); ok {
					if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "tr" {
						t.Errorf("%s: tr is called in a package variable, when the program starts and before a language is chosen; give the texts from a function", fset.Position(c.Pos()))
					}
				}
				return true
			})
		}
	}
}
