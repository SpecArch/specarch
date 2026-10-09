package extract

import (
	"fmt"
	"go/ast"
	"path"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// The import paths of dxlib's packages whose calls declare tables.
const (
	dxlibModels = "github.com/donnyhardyanto/dxlib/databases/models"
	dxlibTypes  = "github.com/donnyhardyanto/dxlib/types"
	dxlibTables = "github.com/donnyhardyanto/dxlib/tables"
)

// dxlibColumnTypes is the column type each of dxlib's data types makes on
// PostgreSQL, as dxlib's types package declares it, written as the
// catalogue dump writes a column's type, so that a table read from source
// and the same table read from the catalogue give one field. A type not
// listed has no type in the meta-model the catalogue reader writes, such
// as a geometry.
var dxlibColumnTypes = map[string]string{
	"DataTypeEncryptedBlob": "bytea", "DataTypeBlob": "bytea",
	"DataTypeUID": "character varying(1024)", "DataTypeString": "character varying(1024)",
	"DataTypeProtectedString": "character varying(1024)", "DataTypeProtectedSQLString": "character varying(1024)",
	"DataTypeProtectedNonEmptyString": "character varying(1024)", "DataTypeNullableString": "character varying(1024)",
	"DataTypeNonEmptyString": "character varying(1024)", "DataTypeEmail": "character varying(255)",
	"DataTypePhoneNumber": "character varying(255)", "DataTypeNPWP": "character varying(255)",
	"DataTypeID": "bigint", "DataTypeInt64": "bigint", "DataTypeInt64P": "bigint", "DataTypeInt64ZP": "bigint",
	"DataTypeNullableInt64": "bigint", "DataTypeBigSerial": "bigint",
	"DataTypeInt32": "integer", "DataTypeInt32P": "integer", "DataTypeInt32ZP": "integer",
	"DataTypeNullableInt32": "integer", "DataTypeInt": "integer", "DataTypeSerial": "integer",
	"DataTypeFloat32": "real", "DataTypeFloat32P": "real", "DataTypeFloat32ZP": "real",
	"DataTypeFloat64": "double precision", "DataTypeFloat64P": "double precision", "DataTypeFloat64ZP": "double precision",
	"DataTypeBool": "boolean", "DataTypeISO8601": "timestamp with time zone", "DataTypeDate": "date",
	"DataTypeTime": "time without time zone", "DataTypeJSON": "jsonb", "DataTypeJSONPassthrough": "jsonb",
	"DataTypeArray": "jsonb", "DataTypeArrayJSONTemplate": "jsonb", "DataTypeMapStringString": "jsonb",
	"DataTypeArrayString": "text[]", "DataTypeArrayInt64": "bigint[]",
	"DataTypeString1": "character varying(1)", "DataTypeString5": "character varying(5)",
	"DataTypeString10": "character varying(10)", "DataTypeString20": "character varying(20)",
	"DataTypeString30": "character varying(30)", "DataTypeString50": "character varying(50)",
	"DataTypeString100": "character varying(100)", "DataTypeString255": "character varying(255)",
	"DataTypeString256": "character varying(256)", "DataTypeString500": "character varying(500)",
	"DataTypeString512": "character varying(512)", "DataTypeString1024": "character varying(1024)",
	"DataTypeString2048": "character varying(2048)", "DataTypeString8096": "character varying(8096)",
	"DataTypeString32768": "character varying(32768)",
	"DataTypeDecimal":     "numeric(30,4)", "DataTypeMoney": "numeric(23,4)",
}

// The data types whose column the database numbers itself, so that its
// value is read and never written, as the catalogue reader writes a column
// whose default is a sequence.
var dxlibSerialTypes = map[string]bool{"DataTypeSerial": true, "DataTypeBigSerial": true}

// The keys of a ModelDBField the reader reads; the others say how the
// database keeps a column, which the catalogue reads.
var modelFieldRead = map[string]bool{"Type": true, "IsPrimaryKey": true, "IsNotNull": true, "IsAutoIncrement": true, "Order": true}

// dxTableCalls are dxlib's constructors of a table the API serves: the
// argument that names the table (a literal name, or a ModelDBTable for the
// ones that wrap one), and how many arguments each takes. The last three
// are always the search, order and filter whitelists.
var dxTableCalls = map[string]struct {
	table int
	model bool
	args  int
}{
	"NewDXTableSimple":          {1, false, 13},
	"NewDXRawTableSimple":       {1, false, 13},
	"NewDXTableAuditOnlySimple": {1, false, 13},
	"NewDXTableWithEncryption":  {1, false, 14},
	"NewDXTable":                {1, true, 7},
	"NewDXTableWithView":        {1, true, 8},
}

// The methods of a dxlib table that serve a paging list, and the
// parameters they read.
var pagingMethods = map[string]bool{"RequestSearchPagingList": true, "RequestSearchPagingDownload": true}

// goAssign is one assignment to a name or a field, by syntax: its value
// and where it is.
type goAssign struct {
	file  *goFile
	value ast.Expr
}

// goTable is one table NewModelDBTable declares.
type goTable struct {
	file    *goFile
	line    int
	schema  string
	name    string
	entity  string
	columns []string
	node    *yaml.Node
}

// dxTable is one table a dxlib table constructor makes for the API, with
// its whitelists.
type dxTable struct {
	clause     string
	call       string
	table      string // "" when it is not read
	search     []string
	order      []string
	filter     []string
	listsKnown bool
}

// collectAssigns records every assignment of the module by the name or
// the field it assigns, qualified by the folder of its package, so that a
// name a call passes is found where it is given its value.
func (g *goReader) collectAssigns() {
	g.assigns = map[string][]goAssign{}
	for _, f := range g.files {
		dir := path.Dir(f.path)
		ast.Inspect(f.ast, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				if len(n.Lhs) == len(n.Rhs) {
					for i, l := range n.Lhs {
						g.assigns[dir+"|"+exprText(l)] = append(g.assigns[dir+"|"+exprText(l)], goAssign{f, n.Rhs[i]})
					}
				} else if len(n.Rhs) == 1 {
					// v, err := call(): the first value is the call's.
					g.assigns[dir+"|"+exprText(n.Lhs[0])] = append(g.assigns[dir+"|"+exprText(n.Lhs[0])], goAssign{f, n.Rhs[0]})
				}
			case *ast.ValueSpec:
				if len(n.Names) == len(n.Values) {
					for i, name := range n.Names {
						g.assigns[dir+"|"+name.Name] = append(g.assigns[dir+"|"+name.Name], goAssign{f, n.Values[i]})
					}
				}
			}
			return true
		})
	}
}

// valueOf is the one value a name or a field of the module is assigned,
// found by syntax: a name of the file's package, a name of another
// package of the module through its import, or a field written as the
// code writes it. ok is false when it is assigned nowhere the reader
// reads, or more than once.
func (g *goReader) valueOf(f *goFile, e ast.Expr) (goAssign, bool) {
	key := path.Dir(f.path) + "|" + exprText(e)
	if sel, ok := e.(*ast.SelectorExpr); ok {
		if x, ok := sel.X.(*ast.Ident); ok && g.modPath != "" {
			ip := f.imports[x.Name]
			if ip == g.modPath || strings.HasPrefix(ip, g.modPath+"/") {
				key = path.Join(g.modRoot, strings.TrimPrefix(strings.TrimPrefix(ip, g.modPath), "/")) + "|" + sel.Sel.Name
			}
		}
	}
	list := g.assigns[key]
	if len(list) != 1 {
		return goAssign{}, false
	}
	return list[0], true
}

// callTo is the call an expression is, to a function of the package the
// import path names, with the name given; nil otherwise.
func callTo(f *goFile, e ast.Expr, importPath string, names ...string) *ast.CallExpr {
	if u, ok := e.(*ast.UnaryExpr); ok {
		e = u.X
	}
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return nil
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok || f.imports[x.Name] != importPath {
		return nil
	}
	for _, n := range names {
		if sel.Sel.Name == n {
			return call
		}
	}
	return nil
}

// readTables reads every NewModelDBTable call of a file that imports
// dxlib's models package, and every table constructor of a file that
// imports dxlib's tables package.
func (g *goReader) readTables() {
	calls := map[string]int{}
	importing := 0
	for _, f := range g.files {
		models, tables := f.importName(dxlibModels), f.importName(dxlibTables)
		if models == "" && tables == "" {
			continue
		}
		importing++
		var stack []ast.Node
		ast.Inspect(f.ast, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if x, ok := sel.X.(*ast.Ident); ok {
						switch {
						case models != "" && x.Name == models && sel.Sel.Name == "NewModelDBTable":
							calls["NewModelDBTable"]++
							g.modelTable(f, call, stack)
						case tables != "" && x.Name == tables && dxTableCalls[sel.Sel.Name].args > 0:
							calls["table"]++
							g.dxTable(f, call, sel.Sel.Name, stack)
						}
					}
				}
			}
			stack = append(stack, n)
			return true
		})
	}
	if importing == 0 {
		return
	}
	g.res.say("dxlib: counted %s and %s: every call by that name in a file that imports dxlib's models package, and every call of dxlib's tables package that makes a table the API serves", plural(calls["NewModelDBTable"], "NewModelDBTable call"), plural(calls["table"], "table constructor"))
}

// schemaName is the name of the schema a NewModelDBTable call's first
// argument gives: a NewModelDBSchema call with a literal name, written
// there or assigned to the name the argument gives.
func (g *goReader) schemaName(f *goFile, e ast.Expr) (string, bool) {
	call := callTo(f, e, dxlibModels, "NewModelDBSchema")
	if call == nil {
		a, ok := g.valueOf(f, e)
		if !ok {
			return "", false
		}
		f, call = a.file, callTo(a.file, a.value, dxlibModels, "NewModelDBSchema")
	}
	if call == nil || len(call.Args) < 2 {
		return "", false
	}
	return g.stringValue(f, call.Args[1])
}

// modelTable reads one NewModelDBTable call into an entity named as
// extract database names the table, so that the two trees give one
// entity.
func (g *goReader) modelTable(f *goFile, call *ast.CallExpr, stack []ast.Node) {
	clause := g.clause(f, call.Pos())
	f.cited = true
	if len(call.Args) != 5 {
		g.question("must", fmt.Sprintf("%s calls NewModelDBTable with %d arguments, and dxlib's takes 5, so the reader does not know which table it declares. Which table does it declare?", clause, len(call.Args)),
			[]string{"entities"}, "A table the reader cannot read is one it would miss.", g.at(clause, fmt.Sprintf("Calls NewModelDBTable with %d arguments.", len(call.Args))))
		return
	}
	if b := branch(stack); b != nil {
		g.question("must", fmt.Sprintf("%s declares a table with NewModelDBTable %s at %s, so the reader cannot tell how many tables it declares, or whether it declares one. Which tables does the database hold?", clause, branchWord(b), g.clause(f, b.Pos())),
			[]string{"entities"}, "Syntax does not run the code; only the database's catalogue says what a loop or a condition declares.", g.at(clause, "Calls NewModelDBTable "+branchWord(b)+"."))
		return
	}
	name, nameOK := g.stringValue(f, call.Args[1])
	schema, schemaOK := g.schemaName(f, call.Args[0])
	if !nameOK || !schemaOK {
		var what []string
		if !schemaOK {
			what = append(what, "schema "+exprText(call.Args[0]))
		}
		if !nameOK {
			what = append(what, "name "+exprText(call.Args[1]))
		}
		g.question("must", fmt.Sprintf("%s declares a table whose %s the reader does not read as a literal. Which table does it declare?", clause, strings.Join(what, " and ")),
			[]string{"entities"}, "A table's schema and name are read where they are literals, or a NewModelDBSchema call with a literal name assigned once; an entity is never named from a guess.", g.at(clause, "Calls NewModelDBTable with "+strings.Join(what, " and ")+"."))
		return
	}
	qualified := schema + "." + name
	if !plainName.MatchString(name) || !plainName.MatchString(schema) {
		g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: table %s: its name has characters an entity name cannot hold; left out", clause, qualified), clause: clause, blocks: []string{"entities"}})
		return
	}
	entity := pascal(name)
	if schema != "public" {
		entity = pascal(schema + "_" + name)
	}
	for _, other := range g.tables {
		if other.entity == entity {
			g.question("must", fmt.Sprintf("%s declares the table %s, whose entity name %s is taken by %s.%s, declared at %s:%d. Which one does the database hold?", clause, qualified, entity, other.schema, other.name, other.file.path, other.line),
				[]string{"entities"}, "Two declarations give one entity, and at most one of them is the table the database holds.", g.at(clause, "Declares "+qualified+" a second time."))
			return
		}
	}
	t := &goTable{file: f, line: g.fset.Position(call.Pos()).Line, schema: schema, name: name, entity: entity}
	g.tables = append(g.tables, t)
	at := "#/entities/" + entity
	ent := mapping("type", "object")
	fields, ok := call.Args[3].(*ast.CompositeLit)
	if !ok {
		g.question("must", fmt.Sprintf("%s declares the table %s with fields %s, which the reader does not read as a literal map. Which columns does it have, and which identify a record?", clause, qualified, exprText(call.Args[3])),
			[]string{at + "/properties", at + "/primaryKey"}, "A table's columns are read from a literal map of ModelDBField; the catalogue gives them once it is read.", g.at(clause, "Declares "+qualified+" with fields that are not a literal map."))
		set(ent, "origin", "stated")
		set(ent, "cites", []*yaml.Node{g.at(clause, fmt.Sprintf("NewModelDBTable declares table %s.", qualified))})
		t.node = ent
		g.registeredTable(t, clause)
		return
	}
	type column struct {
		name     string
		order    int
		hasOrder bool
		node     *yaml.Node
		nullable bool
		key      bool
		left     []string
	}
	var cols []column
	allOrdered := true
	for _, el := range fields.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		at2 := g.clause(f, kv.Pos())
		col, ok := g.stringValue(f, kv.Key)
		if !ok {
			g.question("should", fmt.Sprintf("%s: a column of table %s is named %s, which is not a literal. Which column is it?", at2, qualified, exprText(kv.Key)),
				[]string{at + "/properties"}, "A computed name is not known by syntax.", g.at(at2, "Names a column with something other than a literal."))
			continue
		}
		v := kv.Value
		if u, ok := v.(*ast.UnaryExpr); ok {
			v = u.X
		}
		lit, ok := v.(*ast.CompositeLit)
		if !ok {
			g.question("should", fmt.Sprintf("%s: the column %s.%s is declared by %s, which the reader does not read as a literal ModelDBField. What type is it, and may it be empty?", at2, qualified, col, exprText(kv.Value)),
				[]string{at + "/properties"}, "A field built by a function is not known by syntax.", g.at(at2, "Declares "+col+" with "+exprText(kv.Value)+"."))
			continue
		}
		if !plainName.MatchString(col) || !memberName.MatchString(camel(col)) {
			g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: column %s.%s: its name has characters a field name cannot hold; left out", at2, qualified, col), clause: at2, blocks: []string{at}})
			continue
		}
		c := column{name: col, nullable: true}
		typ := ""
		for _, item := range lit.Elts {
			kv, ok := item.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			k, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			switch {
			case k.Name == "Type":
				if sel, ok := kv.Value.(*ast.SelectorExpr); ok {
					if x, ok := sel.X.(*ast.Ident); ok && f.imports[x.Name] == dxlibTypes {
						typ = sel.Sel.Name
						break
					}
				}
				typ = "?" + exprText(kv.Value)
			case k.Name == "IsPrimaryKey":
				c.key = isTrue(kv.Value)
			case k.Name == "IsNotNull":
				c.nullable = !isTrue(kv.Value)
			case k.Name == "IsAutoIncrement":
				if isTrue(kv.Value) {
					typ += "!"
				}
			case k.Name == "Order":
				if b, ok := kv.Value.(*ast.BasicLit); ok {
					c.order, _ = strconv.Atoi(b.Value)
					c.hasOrder = true
				}
			case !modelFieldRead[k.Name]:
				c.left = append(c.left, k.Name)
			}
		}
		if !c.hasOrder {
			allOrdered = false
		}
		auto := strings.HasSuffix(typ, "!")
		typ = strings.TrimSuffix(typ, "!")
		if c.key {
			// A primary key is never null, whatever IsNotNull says.
			c.nullable = false
		}
		var sqlType string
		switch {
		case typ == "":
			g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: column %s.%s: it names no Type; left out", at2, qualified, col), clause: at2, blocks: []string{at}})
			continue
		case strings.HasPrefix(typ, "?"):
			g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: column %s.%s: its Type %s is not one of dxlib's data types by name; left out", at2, qualified, col, typ[1:]), clause: at2, blocks: []string{at}})
			continue
		default:
			sqlType = dxlibColumnTypes[typ]
			if sqlType == "" {
				g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: column %s.%s, %s: the meta-model has no type for it; left out", at2, qualified, col, typ), clause: at2, blocks: []string{at}})
				continue
			}
		}
		n := g.columnType(qualified, at, col, sqlType, at2)
		if n == nil {
			continue
		}
		if c.nullable {
			n.Content[1] = flow(value([]string{n.Content[1].Value, "null"}))
		}
		if auto || dxlibSerialTypes[typ] {
			set(n, "readOnly", true)
		}
		c.node = n
		if len(c.left) > 0 {
			sort.Strings(c.left)
		}
		cols = append(cols, c)
	}
	if allOrdered {
		sort.SliceStable(cols, func(i, j int) bool { return cols[i].order < cols[j].order })
	}
	props := &yaml.Node{Kind: yaml.MappingNode}
	var required, pk, says, left []string
	for _, c := range cols {
		set(props, camel(c.name), flow(c.node))
		t.columns = append(t.columns, c.name)
		if !c.nullable {
			required = append(required, camel(c.name))
		}
		if c.key {
			pk = append(pk, camel(c.name))
		}
		s := c.name
		if c.key {
			s += " (its key)"
		}
		says = append(says, s)
		for _, l := range c.left {
			if !contains(left, l) {
				left = append(left, l)
			}
		}
	}
	set(ent, "properties", props)
	if len(required) > 0 {
		set(ent, "required", required)
	}
	if len(pk) > 0 {
		set(ent, "primaryKey", pk)
	}
	set(ent, "origin", "stated")
	set(ent, "cites", []*yaml.Node{g.at(clause, fmt.Sprintf("NewModelDBTable declares table %s with the columns %s.", qualified, joinAnd(says)))})
	t.node = ent
	if len(left) > 0 {
		sort.Strings(left)
		g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: table %s: its fields' %s say how the database keeps a column, which the catalogue dump reads; left to it", clause, qualified, joinAnd(left)), clause: clause, blocks: []string{at}})
	}
	if len(pk) == 0 {
		g.question("must", fmt.Sprintf("Table %s, declared at %s, marks no column IsPrimaryKey. Which fields identify one of its records?", qualified, clause),
			[]string{at + "/primaryKey"}, "Every entity names the fields that identify a record, and the declaration gives none.", g.at(clause, "Declares "+qualified+" with no primary key."))
	}
	g.registeredTable(t, clause)
}

// registeredTable asks whether the database holds a table the source
// declares, which a catalogue read with it answers in the merge.
func (g *goReader) registeredTable(t *goTable, clause string) {
	qualified := t.schema + "." + t.name
	g.question("must", fmt.Sprintf("Does the database hold table %s? It is declared at %s, and no catalogue of the running database was read with it.", qualified, clause),
		[]string{"#/entities/" + t.entity}, "Syntax shows a declaration and not what the database holds; the catalogue dump after every migration says it, and merging it answers this.", g.at(clause, "Declares "+qualified+" with NewModelDBTable."))
}

// columnType writes a column's type keywords as extract database writes
// the same column type, and moves what it says it cannot hold, or asks,
// onto the column's line.
func (g *goReader) columnType(table, at, col, sqlType, clause string) *yaml.Node {
	d := &dbReader{key: g.key, res: &Result{Tree: newTree()}, questions: &yaml.Node{Kind: yaml.MappingNode}, enums: map[string][]string{}, entities: map[string]string{}}
	f := &field{column: col, name: camel(col)}
	n := d.columnType(table, at, table+"."+col, sqlType, f)
	for _, h := range d.notHeld {
		h.clause = clause
		h.text = clause + ": " + h.text
		g.notHeld = append(g.notHeld, h)
	}
	for _, p := range source.Pairs(d.questions) {
		q := p.Value
		var blocks []string
		for _, b := range source.Items(source.Child(q, "blocks")) {
			blocks = append(blocks, b.Value)
		}
		g.question(source.Str(source.Child(q, "priority")), clause+": "+source.Str(source.Child(q, "question")), blocks, source.Str(source.Child(q, "why")),
			g.at(clause, fmt.Sprintf("Declares the column %s as %s.", col, sqlType)))
	}
	return n
}

// dxTable reads one of dxlib's table constructors: the table it serves,
// and the whitelists of the paging list that reads it.
func (g *goReader) dxTable(f *goFile, call *ast.CallExpr, name string, stack []ast.Node) {
	clause := g.clause(f, call.Pos())
	f.cited = true
	spec := dxTableCalls[name]
	t := &dxTable{clause: clause, call: name}
	if len(call.Args) != spec.args {
		g.res.say("nothing read: %s calls %s with %d arguments, and dxlib's takes %d", clause, name, len(call.Args), spec.args)
		return
	}
	if spec.model {
		if a, ok := g.valueOf(f, call.Args[spec.table]); ok {
			if c := callTo(a.file, a.value, dxlibModels, "NewModelDBTable"); c != nil && len(c.Args) == 5 {
				schema, ok1 := g.schemaName(a.file, c.Args[0])
				table, ok2 := g.stringValue(a.file, c.Args[1])
				if ok1 && ok2 {
					t.table = schema + "." + table
				}
			}
		}
	} else if s, ok := g.stringValue(f, call.Args[spec.table]); ok {
		t.table = s
	}
	n := len(call.Args)
	search, ok1 := g.stringList(f, call.Args[n-3])
	order, ok2 := g.stringList(f, call.Args[n-2])
	filter, ok3 := g.stringList(f, call.Args[n-1])
	t.search, t.order, t.filter, t.listsKnown = search, order, filter, ok1 && ok2 && ok3
	// The table belongs to the name or field it is assigned to.
	for i := len(stack) - 1; i >= 0; i-- {
		if as, ok := stack[i].(*ast.AssignStmt); ok && len(as.Lhs) == len(as.Rhs) {
			for j, r := range as.Rhs {
				if r == call {
					g.dxTables[path.Dir(f.path)+"|"+exprText(as.Lhs[j])] = t
				}
			}
			break
		}
		if vs, ok := stack[i].(*ast.ValueSpec); ok && len(vs.Names) == len(vs.Values) {
			for j, r := range vs.Values {
				if r == call {
					g.dxTables[path.Dir(f.path)+"|"+vs.Names[j].Name] = t
				}
			}
			break
		}
	}
}

// says is what a table constructor tells of the paging list that reads it.
func (t *dxTable) says() string {
	table := t.table
	if table == "" {
		table = "a table not named by a literal"
	}
	if !t.listsKnown {
		return fmt.Sprintf("%s makes the table %s, with whitelists that are not literal lists of names.", t.call, table)
	}
	list := func(names []string) string {
		if len(names) == 0 {
			return "none"
		}
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s makes the table %s: its paging list searches %s; orders by %s; filters by %s.", t.call, table, list(t.search), list(t.order), list(t.filter))
}

// pagingTable is the dxlib table whose paging list method a handler is,
// found by syntax, or nil.
func (g *goReader) pagingTable(f *goFile, h ast.Expr) (*dxTable, string) {
	sel, ok := h.(*ast.SelectorExpr)
	if !ok || !pagingMethods[sel.Sel.Name] {
		return nil, ""
	}
	key := path.Dir(f.path) + "|" + exprText(sel.X)
	if x, ok := sel.X.(*ast.SelectorExpr); ok {
		if id, ok := x.X.(*ast.Ident); ok && g.modPath != "" {
			ip := f.imports[id.Name]
			if ip == g.modPath || strings.HasPrefix(ip, g.modPath+"/") {
				key = path.Join(g.modRoot, strings.TrimPrefix(strings.TrimPrefix(ip, g.modPath), "/")) + "|" + x.Sel.Name
			}
		}
	}
	return g.dxTables[key], sel.Sel.Name
}

// writeTables writes the entities in the order of their names.
func (g *goReader) writeTables() {
	sort.SliceStable(g.tables, func(i, j int) bool { return g.tables[i].entity < g.tables[j].entity })
	for _, t := range g.tables {
		g.entityFiles["design/entities/"+kebab(t.entity)+".yaml"] = mapping("entities", mapping(t.entity, t.node))
	}
}

func isTrue(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "true"
}
