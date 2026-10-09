package gensql

import (
	"strings"
	"unicode/utf8"
)

// Identifier length (ADR-078). Each engine limits the length of a name, and
// a column named after the path to a part of a value (ADR-063), or a check
// named after its table and column, can pass it. PostgreSQL keeps at most
// NAMEDATALEN-1, 63, bytes of an identifier and silently cuts the rest, so
// two long names that share their first 63 bytes become one; the other
// engines refuse the statement. A name over the dialect's limit is an error
// at the field or the entity it comes from.

// identifierLimit is the longest name a dialect takes, and whether it is
// counted in bytes rather than characters.
var identifierLimit = map[string]struct {
	max   int
	bytes bool
}{
	"postgresql": {63, true},   // NAMEDATALEN-1 bytes; longer names are truncated
	"sqlserver":  {128, false}, // sysname, nvarchar(128)
	"oracle":     {128, true},  // 128 bytes from Oracle Database 12.2, the oldest release still supported being 19c
	"mariadb":    {64, false},  // 64 characters for a table, column, index or constraint
}

// checkNames reports every name a table is written with that is longer than
// the dialect takes: the table's, its columns', and its constraints',
// indexes' and foreign keys'.
func (g *gen) checkNames(entity string, t table) {
	limit, ok := identifierLimit[g.dialect]
	if !ok {
		return
	}
	at := "/entities/" + entity
	length := func(name string) int {
		if limit.bytes {
			return len(name)
		}
		return utf8.RuneCountInString(name)
	}
	unit := "characters"
	if limit.bytes {
		unit = "bytes"
	}
	check := func(path, what, name string) {
		if n := length(name); n > limit.max {
			g.problem(path, "the %s %s is %d %s long, and %s takes at most %d; shorten the names it is made of", what, name, n, unit, g.dialect, limit.max)
		}
	}
	check(at, "table", t.name)
	for _, l := range t.columns {
		col := strings.Fields(l)[0]
		check(g.columnAt(entity, col), "column", col)
	}
	for _, l := range append(append(append([]string{}, t.constraints...), t.indexes...), t.foreignKeys...) {
		for _, word := range []string{"CONSTRAINT ", "INDEX "} {
			if _, rest, found := strings.Cut(l, word); found {
				check(at, "constraint", strings.Fields(rest)[0])
				break
			}
		}
	}
}

// columnAt is the pointer of the field a column comes from: the field
// itself, or the field holding the value whose part it is, or the entity.
func (g *gen) columnAt(entity, col string) string {
	at := "/entities/" + entity
	props := obj0(obj0(g.original[entity])["properties"])
	if props == nil {
		props = obj0(obj0(obj0(g.spec["entities"])[entity])["properties"])
	}
	for _, f := range sortedKeys(props) {
		if snake(f) == col {
			return at + "/properties/" + f
		}
		if name, _ := valueObjectOf(obj0(props[f])); name != "" && strings.HasPrefix(col, snake(f)+"_") {
			return at + "/properties/" + f
		}
	}
	return at
}
