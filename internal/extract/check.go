package extract

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/gensql"
	"github.com/SpecArch/specarch/internal/implies"
	"github.com/SpecArch/specarch/internal/ownership"
	"github.com/SpecArch/specarch/internal/source"
)

// What specarch check compares a specification with: the dumps the readers
// read, each matched by the merge's own matching (ADR-124).

// The formats of the dumps check reads, by the key that names each.
const (
	RouteTableDump      = "route table"
	PermissionTableDump = "permission table"
	CatalogueDump       = "catalogue dump"
)

// RuleFormats are the rules of check's differences, each with the format
// of the dump it compares.
var RuleFormats = map[string]string{
	"operation_not_served":             RouteTableDump,
	"route_not_specified":              RouteTableDump,
	"operation_permission_differs":     RouteTableDump,
	"grant_not_in_code":                PermissionTableDump,
	"grant_not_specified":              PermissionTableDump,
	"table_not_in_database":            CatalogueDump,
	"table_not_specified":              CatalogueDump,
	"column_not_in_database":           CatalogueDump,
	"column_not_specified":             CatalogueDump,
	"column_type_differs":              CatalogueDump,
	"column_nullability_differs":       CatalogueDump,
	"column_generated_default_differs": CatalogueDump,
	"foreign_key_not_in_database":      CatalogueDump,
	"foreign_key_not_specified":        CatalogueDump,
	"foreign_key_differs":              CatalogueDump,
	"unique_not_in_database":           CatalogueDump,
	"unique_not_specified":             CatalogueDump,
	"check_not_in_database":            CatalogueDump,
	"check_not_specified":              CatalogueDump,
	"check_expression_differs":         CatalogueDump,
	"index_not_in_database":            CatalogueDump,
	"index_differs":                    CatalogueDump,
	"index_not_listed":                 CatalogueDump,
}

// Difference is one thing a dump and the specification disagree on.
type Difference struct {
	Rule    string
	Subject string // what it is about, by the matching: POST /loans, members.phone_number
	Message string
	Report  bool // a report: printed as a warning, never a failure
	// Pointer is the specification's entry it is at, or "" for one only
	// the dump has, which is at Line and Column of the dump File.
	Pointer      string
	File         string
	Line, Column int
	DumpPointer  string // the entry's pointer in the dump
}

// ID is the difference's id: its rule and what it is about, the same
// wherever the entry is written.
func (d Difference) ID() string { return d.Rule + " " + d.Subject }

// DumpFormat is the format of a dump, by the key that names it.
func DumpFormat(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return "", refuse("%s is not a dump: it is not a JSON object (%v)", path, err)
	}
	switch {
	case keys["routeTable"] != nil:
		return RouteTableDump, nil
	case keys["permissionTable"] != nil:
		return PermissionTableDump, nil
	case keys["catalogue"] != nil:
		return CatalogueDump, nil
	}
	return "", refuse("%s is no dump check reads: it has none of the keys routeTable (a route table), permissionTable (a permission table) and catalogue (a catalogue dump)", path)
}

// dumpNodes is a dump read as YAML nodes, for the line and column of an
// entry a difference is at.
type dumpNodes struct{ root *yaml.Node }

func readDumpNodes(path string) dumpNodes {
	data, err := os.ReadFile(path)
	if err != nil {
		return dumpNodes{}
	}
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil || len(doc.Content) == 0 {
		return dumpNodes{}
	}
	return dumpNodes{doc.Content[0]}
}

// at is the node at a path of keys and list indexes, or nil.
func (d dumpNodes) at(steps ...any) *yaml.Node {
	n := d.root
	for _, s := range steps {
		if n == nil {
			return nil
		}
		switch s := s.(type) {
		case string:
			n = source.Child(n, s)
		case int:
			if n.Kind != yaml.SequenceNode || s >= len(n.Content) {
				return nil
			}
			n = n.Content[s]
		}
	}
	return n
}

// place sets a difference in the dump at the node of the path, which
// DumpPointer names.
func (d dumpNodes) place(diff *Difference, file string, steps ...any) {
	diff.File, diff.Line, diff.Column = file, 1, 1
	for _, s := range steps {
		diff.DumpPointer += "/" + escapeToken(fmt.Sprint(s))
	}
	if n := d.at(steps...); n != nil {
		diff.Line, diff.Column = n.Line, n.Column
	}
}

// CheckRoutes compares a route table with the specification's operations:
// an operation by its method and path, and the permission it checks.
// Under bindings.http.listRoutes, each list served by GET is served by
// POST at its twin's path too (ADR-121); an operation another stakeholder
// owns is not compared.
func CheckRoutes(dumpPath string, root *yaml.Node, implementations []*yaml.Node) ([]Difference, error) {
	t, _, name, err := openRouteTable(dumpPath)
	if err != nil {
		return nil, err
	}
	nodes := readDumpNodes(dumpPath)
	type specRoute struct{ permission, pointer, operation string }
	owned := ownership.Owned{}
	segment := ""
	for _, impl := range implementations {
		if content, ok := source.ValueOf(impl).(map[string]any); ok {
			for k, v := range ownership.Of(content) {
				owned[k] = v
			}
		}
		if s := source.Str(source.Child(source.Child(source.Child(source.Child(impl, "bindings"), "http"), "listRoutes"), "postSegment")); s != "" && segment == "" {
			segment = s
		}
	}
	spec := map[string]specRoute{}
	var order []string
	add := func(key string, r specRoute) {
		if _, ok := spec[key]; !ok {
			order = append(order, key)
		}
		spec[key] = r
	}
	for _, p := range source.Pairs(source.Child(root, "paths")) {
		for _, method := range pathMethods {
			op := source.Child(p.Value, method)
			if op == nil || owned.Covers(ownership.Operation(p.Key.Value, method)) {
				continue
			}
			r := specRoute{permission: openToAll(source.Str(source.Child(op, "permission"))), pointer: "/paths/" + escapeToken(p.Key.Value) + "/" + method, operation: source.Str(source.Child(op, "operationId"))}
			add(strings.ToUpper(method)+" "+p.Key.Value, r)
			if method == "get" && segment != "" && source.Child(op, "listOf") != nil {
				add("POST "+strings.TrimSuffix(p.Key.Value, "/")+"/"+segment, r)
			}
		}
	}
	type codeRoute struct {
		permission, handler string
		index               int
	}
	code := map[string]codeRoute{}
	for i, r := range t.Routes {
		code[strings.ToUpper(r.Method)+" "+r.Path] = codeRoute{permission: openToAll(printedPermission(r.Permission)), handler: r.Handler, index: i}
	}
	var out []Difference
	for _, key := range order {
		s := spec[key]
		c, ok := code[key]
		if !ok {
			out = append(out, Difference{Rule: "operation_not_served", Subject: key, Pointer: s.pointer,
				Message: fmt.Sprintf("operation %s, %s, is in the specification and the route table %s does not serve it; build it, or take it out of the specification", s.operation, key, name)})
			continue
		}
		if c.permission != s.permission {
			d := Difference{Rule: "operation_permission_differs", Subject: key, Pointer: s.pointer,
				Message: fmt.Sprintf("operation %s, %s, checks %s in the specification and %s in the route table %s", s.operation, key, permissionOrNone(s.permission), permissionOrNone(c.permission), name)}
			out = append(out, d)
		}
	}
	for i, r := range t.Routes {
		key := strings.ToUpper(r.Method) + " " + r.Path
		if _, ok := spec[key]; ok || code[key].index != i {
			continue
		}
		d := Difference{Rule: "route_not_specified", Subject: key,
			Message: fmt.Sprintf("%s is served, by the handler %s, checking %s, and the specification names no operation for it; specify it, or take the route out", key, r.Handler, permissionOrNone(code[key].permission))}
		nodes.place(&d, dumpPath, "routes", i)
		out = append(out, d)
	}
	return out, nil
}

// printedPermission is the permission a route table names, or "" for none.
func printedPermission(raw json.RawMessage) string {
	var s string
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

// openToAll is a permission with public, an operation open to everyone,
// as a route that checks none: the route table prints null for it.
func openToAll(p string) string {
	if p == "public" {
		return ""
	}
	return p
}

func permissionOrNone(p string) string {
	if p == "" {
		return "no permission (public)"
	}
	return p
}

// CheckGrants compares a permission table with the roles of the
// specification: a grant by its role and permission. A grant one side
// declares is held by the other when it grants it, itself or through a
// permission it grants that implies it, by the specification's implies
// (ADR-100). A grant of public is no grant, and the gates are not
// compared.
func CheckGrants(dumpPath string, root *yaml.Node) ([]Difference, error) {
	t, _, name, err := openPermissionTable(dumpPath)
	if err != nil {
		return nil, err
	}
	pr := &permissionReader{t: t, res: &Result{Tree: newTree()}, questions: &yaml.Node{Kind: yaml.MappingNode}, bypassBlocks: map[string]*yaml.Node{}}
	if err := pr.read(name); err != nil {
		return nil, err
	}
	nodes := readDumpNodes(dumpPath)
	impliedBy := func(p string) []string {
		var out []string
		for _, q := range source.Items(source.Child(source.Child(source.Child(root, "permissions"), p), "implies")) {
			out = append(out, q.Value)
		}
		return out
	}
	held := func(grants []string) map[string]bool { return implies.Holds(grants, impliedBy) }
	specGrants := map[string][]string{}
	var roles []string
	for _, r := range source.Pairs(source.Child(root, "roles")) {
		roles = append(roles, r.Key.Value)
		specGrants[r.Key.Value] = []string{}
		for _, p := range source.Items(source.Child(r.Value, "permissions")) {
			specGrants[r.Key.Value] = append(specGrants[r.Key.Value], p.Value)
		}
	}
	codeGrants := map[string][]string{}
	for _, g := range pr.compared {
		if codeGrants[g.role] == nil {
			roles = append(roles, g.role)
		}
		codeGrants[g.role] = append(codeGrants[g.role], g.permission)
	}
	var out []Difference
	sort.Strings(roles)
	seen := map[string]bool{}
	for _, role := range roles {
		if seen[role] {
			continue
		}
		seen[role] = true
		inCode := held(codeGrants[role])
		for _, p := range specGrants[role] {
			if !inCode[p] {
				out = append(out, Difference{Rule: "grant_not_in_code", Subject: role + " " + p, Pointer: "/roles/" + escapeToken(role) + "/permissions",
					Message: fmt.Sprintf("role %s grants %s in the specification, and the permission table %s grants neither it nor a permission that implies it", role, p, name)})
			}
		}
	}
	for _, g := range pr.compared {
		if held(specGrants[g.role])[g.permission] {
			continue
		}
		d := Difference{Rule: "grant_not_specified", Subject: g.role + " " + g.permission,
			Message: fmt.Sprintf("the permission table grants %s to %s, and the specification grants it neither that permission nor one that implies it; specify the grant, or take it out", g.permission, g.role)}
		if specGrants[g.role] == nil {
			d.Message = fmt.Sprintf("the permission table grants %s to %s, a role the specification does not declare; specify the role, or take the grant out", g.permission, g.role)
		}
		nodes.place(&d, dumpPath, "grants", g.index)
		out = append(out, d)
	}
	return out, nil
}

// CheckCatalogue compares a catalogue dump with the tables
// specarch generate sql writes for PostgreSQL: a table by its name and a
// column by its table and name, and their types, nullability, generated
// defaults, keys, constraints and listed indexes.
func CheckCatalogue(dumpPath string, tables []gensql.Table) ([]Difference, error) {
	c, _, name, err := openCatalogue(dumpPath)
	if err != nil {
		return nil, err
	}
	nodes := readDumpNodes(dumpPath)
	generated := generatedDefaults()
	type dbTable struct {
		t     dumpTable
		index int
	}
	db := map[string]dbTable{}
	tableName := func(schema, table string) string {
		if schema == "" || schema == "public" {
			return table
		}
		return schema + "." + table
	}
	for i, t := range c.Tables {
		db[tableName(t.Schema, t.Name)] = dbTable{t, i}
	}
	var out []Difference
	specTables := map[string]bool{}
	for _, st := range tables {
		specTables[st.Name] = true
		at := "/entities/" + escapeToken(st.Entity)
		dt, ok := db[st.Name]
		if !ok {
			out = append(out, Difference{Rule: "table_not_in_database", Subject: st.Name, Pointer: at,
				Message: fmt.Sprintf("table %s of entity %s is not in the catalogue %s; write its migration, or take the entity out", st.Name, st.Entity, name)})
			continue
		}
		out = append(out, compareTable(st, dt.t, dt.index, at, name, dumpPath, nodes, generated)...)
	}
	for i, t := range c.Tables {
		n := tableName(t.Schema, t.Name)
		if specTables[n] {
			continue
		}
		d := Difference{Rule: "table_not_specified", Subject: n,
			Message: fmt.Sprintf("table %s is in the database, and no entity of the specification is held in it; specify it, or drop it", n)}
		nodes.place(&d, dumpPath, "tables", i)
		out = append(out, d)
	}
	return out, nil
}

func compareTable(st gensql.Table, t dumpTable, ti int, at, name, dumpPath string, nodes dumpNodes, generated map[string]string) []Difference {
	var out []Difference
	fieldAt := func(field string) string {
		if field == "" {
			return at
		}
		return at + "/properties/" + escapeToken(field)
	}
	dbCols := map[string]int{}
	for i, col := range t.Columns {
		dbCols[col.Name] = i
	}
	specCols := map[string]bool{}
	for _, sc := range st.Columns {
		specCols[sc.Name] = true
		subject := st.Name + "." + sc.Name
		ci, ok := dbCols[sc.Name]
		if !ok {
			out = append(out, Difference{Rule: "column_not_in_database", Subject: subject, Pointer: fieldAt(sc.Field),
				Message: fmt.Sprintf("column %s, %s, is not in the catalogue %s; write its migration, or take the field out", subject, sc.Type, name)})
			continue
		}
		col := t.Columns[ci]
		place := func(d Difference) Difference {
			d.Pointer = fieldAt(sc.Field)
			return d
		}
		if a, b := postgresType(sc.Type), postgresType(col.Type); a != b {
			out = append(out, place(Difference{Rule: "column_type_differs", Subject: subject,
				Message: fmt.Sprintf("column %s is %s in the specification's table and %s in the database", subject, a, b)}))
		}
		if sc.Nullable == col.NotNull {
			out = append(out, place(Difference{Rule: "column_nullability_differs", Subject: subject,
				Message: fmt.Sprintf("column %s %s in the specification's table and %s in the database", subject, nullWords(sc.Nullable), nullWords(!col.NotNull))}))
		}
		dbGenerated := ""
		if col.Default != nil {
			dbGenerated = generated[*col.Default]
		}
		if sc.GeneratedDefault != dbGenerated {
			out = append(out, place(Difference{Rule: "column_generated_default_differs", Subject: subject,
				Message: fmt.Sprintf("column %s is drawn by %s in the specification's table and by %s in the database", subject, generatedWords(sc.GeneratedDefault), generatedDBWords(dbGenerated, col.Default))}))
		}
	}
	for i, col := range t.Columns {
		if specCols[col.Name] {
			continue
		}
		subject := st.Name + "." + col.Name
		d := Difference{Rule: "column_not_specified", Subject: subject,
			Message: fmt.Sprintf("column %s, %s, is in the database, and no field of entity %s is held in it; specify it, or drop it", subject, col.Type, st.Entity)}
		nodes.place(&d, dumpPath, "tables", ti, "columns", i)
		out = append(out, d)
	}
	qualified := t.Schema + "." + t.Name
	// The primary key.
	var pk []string
	for _, k := range t.Constraints {
		if k.Kind == "primary" {
			pk = k.Columns
		}
	}
	if strings.Join(pk, ", ") != strings.Join(st.Keys.PrimaryKey, ", ") {
		out = append(out, Difference{Rule: "primary_key_differs", Subject: st.Name, Pointer: at + "/primaryKey",
			Message: fmt.Sprintf("the primary key of %s is %s in the specification's table and %s in the database", st.Name, columnsWords(st.Keys.PrimaryKey), columnsWords(pk))})
	}
	// Foreign keys, by their columns.
	dbFK := map[string]int{}
	for i, k := range t.Constraints {
		if k.Kind == "foreign" {
			dbFK[strings.Join(k.Columns, ", ")] = i
		}
	}
	specFK := map[string]bool{}
	for _, fk := range st.Keys.ForeignKeys {
		cols := strings.Join(fk.Columns, ", ")
		specFK[cols] = true
		subject := st.Name + "(" + cols + ")"
		pointer := at + "/relations/" + escapeToken(fk.Relation)
		i, ok := dbFK[cols]
		if !ok {
			out = append(out, Difference{Rule: "foreign_key_not_in_database", Subject: subject, Pointer: pointer,
				Message: fmt.Sprintf("the foreign key of %s, from %s to %s (%s), is not in the catalogue %s", st.Name, cols, fk.Table, strings.Join(fk.References, ", "), name)})
			continue
		}
		k := t.Constraints[i]
		refTable := deref(k.ReferencesTable)
		if k.ReferencesSchema != nil && *k.ReferencesSchema != "public" {
			refTable = *k.ReferencesSchema + "." + refTable
		}
		want := fmt.Sprintf("%s (%s) on delete %s", fk.Table, strings.Join(fk.References, ", "), fk.OnDelete)
		got := fmt.Sprintf("%s (%s) on delete %s", refTable, strings.Join(k.ReferencesColumns, ", "), onDelete(k.OnDelete))
		if want != got {
			out = append(out, Difference{Rule: "foreign_key_differs", Subject: subject, Pointer: pointer,
				Message: fmt.Sprintf("the foreign key of %s from %s refers to %s in the specification's table and to %s in the database, as its constraint %s says", st.Name, cols, want, got, k.Name)})
		}
	}
	for i, k := range t.Constraints {
		cols := strings.Join(k.Columns, ", ")
		if k.Kind != "foreign" || specFK[cols] {
			continue
		}
		d := Difference{Rule: "foreign_key_not_specified", Subject: st.Name + "(" + cols + ")",
			Message: fmt.Sprintf("the foreign key %s of %s, from %s to %s, is in the database, and no relation of entity %s gives it; specify the relation, or drop the key", k.Name, st.Name, cols, deref(k.ReferencesTable), st.Entity)}
		nodes.place(&d, dumpPath, "tables", ti, "constraints", i)
		out = append(out, d)
	}
	// Unique constraints, by their columns.
	dbUnique := map[string]bool{}
	for _, k := range t.Constraints {
		if k.Kind == "unique" {
			dbUnique[strings.Join(k.Columns, ", ")] = true
		}
	}
	specUnique := map[string]bool{}
	for _, u := range st.Keys.Uniques {
		cols := strings.Join(u.Columns, ", ")
		specUnique[cols] = true
		if !dbUnique[cols] {
			out = append(out, Difference{Rule: "unique_not_in_database", Subject: st.Name + "(" + cols + ")", Pointer: at + "/constraints/" + escapeToken(u.Constraint),
				Message: fmt.Sprintf("the unique constraint %s of %s, on %s, is not in the catalogue %s", u.Name, st.Name, cols, name)})
		}
	}
	for i, k := range t.Constraints {
		cols := strings.Join(k.Columns, ", ")
		if k.Kind != "unique" || specUnique[cols] {
			continue
		}
		d := Difference{Rule: "unique_not_specified", Subject: st.Name + "(" + cols + ")",
			Message: fmt.Sprintf("the unique constraint %s of %s, on %s, is in the database, and no constraint of entity %s gives it; specify it, or drop it", k.Name, st.Name, cols, st.Entity)}
		nodes.place(&d, dumpPath, "tables", ti, "constraints", i)
		out = append(out, d)
	}
	// Check constraints, by their names; their text is a report.
	dbCheck := map[string]int{}
	for i, k := range t.Constraints {
		if k.Kind == "check" {
			dbCheck[k.Name] = i
		}
	}
	specCheck := map[string]bool{}
	for _, ck := range st.Keys.Checks {
		specCheck[ck.Name] = true
		pointer := fieldAt(ck.Field)
		if ck.Constraint != "" {
			pointer = at + "/constraints/" + escapeToken(ck.Constraint)
		}
		subject := st.Name + "." + ck.Name
		i, ok := dbCheck[ck.Name]
		if !ok {
			out = append(out, Difference{Rule: "check_not_in_database", Subject: subject, Pointer: pointer,
				Message: fmt.Sprintf("the check %s of %s, %s, is not in the catalogue %s", ck.Name, st.Name, ck.Expression, name)})
			continue
		}
		def := strings.TrimPrefix(t.Constraints[i].Definition, "CHECK ")
		if checkText(def) != checkText(ck.Expression) {
			out = append(out, Difference{Rule: "check_expression_differs", Subject: subject, Pointer: pointer, Report: true,
				Message: fmt.Sprintf("the check %s of %s reads %s in the specification's table and %s in the database; the database rewrites a check's text, so this is a report", ck.Name, st.Name, ck.Expression, def)})
		}
	}
	for i, k := range t.Constraints {
		if k.Kind != "check" || specCheck[k.Name] {
			continue
		}
		d := Difference{Rule: "check_not_specified", Subject: st.Name + "." + k.Name,
			Message: fmt.Sprintf("the check %s of %s, %s, is in the database, and no constraint of entity %s writes it; specify it, or drop it", k.Name, st.Name, k.Definition, st.Entity)}
		nodes.place(&d, dumpPath, "tables", ti, "constraints", i)
		out = append(out, d)
	}
	// Plain indexes the mapping lists, by their names; an index it does
	// not list is a report.
	dbIndex := map[string]int{}
	for i, ix := range t.Indexes {
		if !ix.Primary && !ix.BacksConstraint {
			dbIndex[ix.Name] = i
		}
	}
	specIndex := map[string]bool{}
	for _, ix := range st.Keys.Indexes {
		specIndex[ix.Name] = true
		subject := st.Name + "." + ix.Name
		i, ok := dbIndex[ix.Name]
		if !ok {
			out = append(out, Difference{Rule: "index_not_in_database", Subject: subject, Pointer: at,
				Message: fmt.Sprintf("the index %s of %s, on %s, which the mapping of entity %s lists, is not in the catalogue %s", ix.Name, st.Name, strings.Join(ix.Columns, ", "), st.Entity, name)})
			continue
		}
		cols, plain := plainIndexColumns(t.Indexes[i].Definition, qualified)
		if !plain || strings.Join(cols, ", ") != strings.Join(ix.Columns, ", ") {
			out = append(out, Difference{Rule: "index_differs", Subject: subject, Pointer: at,
				Message: fmt.Sprintf("the index %s of %s is a plain index on %s in the mapping of entity %s and %s in the database", ix.Name, st.Name, strings.Join(ix.Columns, ", "), st.Entity, t.Indexes[i].Definition)})
		}
	}
	for i, ix := range t.Indexes {
		if ix.Primary || ix.BacksConstraint || specIndex[ix.Name] {
			continue
		}
		d := Difference{Rule: "index_not_listed", Subject: st.Name + "." + ix.Name, Report: true,
			Message: fmt.Sprintf("the index %s of %s, %s, is in the database and the mapping of entity %s does not list it; an index is physical design, so this is a report: list it under the mapping's indexes when it is plain", ix.Name, st.Name, ix.Definition, st.Entity)}
		nodes.place(&d, dumpPath, "tables", ti, "indexes", i)
		out = append(out, d)
	}
	return out
}

func nullWords(nullable bool) string {
	if nullable {
		return "takes null"
	}
	return "is not null"
}

func generatedWords(g string) string {
	if g == "" {
		return "no generated default"
	}
	return "the generated default " + g
}

func generatedDBWords(g string, def *string) string {
	switch {
	case g != "":
		return "the generated default " + g + " (" + *def + ")"
	case def != nil:
		return "no generated default (its default is " + *def + ")"
	}
	return "no generated default"
}

func columnsWords(cols []string) string {
	if len(cols) == 0 {
		return "none"
	}
	return "(" + strings.Join(cols, ", ") + ")"
}

// The short spellings of PostgreSQL's types, as an idiom may write them,
// by the spelling format_type prints in the catalogue.
var (
	typeSpelling = map[string]string{
		"int": "integer", "int4": "integer", "int8": "bigint", "int2": "smallint",
		"bool": "boolean", "float8": "double precision", "float4": "real",
		"timestamptz": "timestamp with time zone", "timestamp": "timestamp without time zone",
		"time": "time without time zone", "timetz": "time with time zone",
		"serial": "integer", "bigserial": "bigint", "smallserial": "smallint",
	}
	typeWithWidth = regexp.MustCompile(`^(varchar|char|decimal|numeric)\s*\(\s*([0-9]+)\s*(?:,\s*([0-9]+)\s*)?\)$`)
	spaces        = regexp.MustCompile(`\s+`)
)

// postgresType is a type as PostgreSQL's format_type spells it: only the
// spelling changes, never a width or the kind.
func postgresType(t string) string {
	t = strings.ToLower(strings.TrimSpace(spaces.ReplaceAllString(t, " ")))
	suffix := ""
	for strings.HasSuffix(t, "[]") {
		t, suffix = strings.TrimSpace(strings.TrimSuffix(t, "[]")), suffix+"[]"
	}
	if s, ok := typeSpelling[t]; ok {
		return s + suffix
	}
	if m := typeWithWidth.FindStringSubmatch(t); m != nil {
		base := map[string]string{"varchar": "character varying", "char": "character", "decimal": "numeric", "numeric": "numeric"}[m[1]]
		if m[3] != "" {
			return base + "(" + m[2] + "," + m[3] + ")" + suffix
		}
		return base + "(" + m[2] + ")" + suffix
	}
	if t == "char" {
		return "character(1)" + suffix
	}
	return t + suffix
}

// checkText is a check's text with its spaces and parentheses taken out
// and in lower case, so a report is made only where the words differ.
func checkText(s string) string {
	return strings.ToLower(strings.NewReplacer(" ", "", "(", "", ")", "", "\n", "", "\t", "").Replace(s))
}
