// Package gensql writes the SQL migrations of a specification: the logic
// of the specarch-gen-sql plug-in. Every column type comes from the
// type-rendering idiom for the target's dialect, so PostgreSQL, SQL Server,
// Oracle and MariaDB are four renderings of one design, and a field no row
// renders fails generation. Migrations are new files only: the first run
// writes the whole schema and a snapshot of it beside the migration.
package gensql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/ownership"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/typerows"
)

// Request is what specarch writes on a plug-in's standard input.
type Request struct {
	Specarch        string           `json:"specarch"`
	Target          string           `json:"target"`
	Root            string           `json:"root"`
	Specification   map[string]any   `json:"specification"`
	Implementations []Implementation `json:"implementations"`
	Output          string           `json:"output"`
	Existing        []File           `json:"existing"`
}

// Implementation is one implementation file in the request.
type Implementation struct {
	File     string         `json:"file"`
	Content  map[string]any `json:"content"`
	Settings map[string]any `json:"settings,omitempty"`
	Idioms   []Idiom        `json:"idioms"`
}

// Idiom is one idiom an implementation file uses.
type Idiom struct {
	Name     string         `json:"name"`
	Version  string         `json:"version"`
	As       string         `json:"as"`
	Content  map[string]any `json:"content,omitempty"`
	Override map[string]any `json:"override,omitempty"`
}

// File is one file in the output folder, or one the plug-in answers with.
type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Diagnostic is one problem the plug-in reports, in the validator's fields.
type Diagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

// Response is what the plug-in writes on its standard output.
type Response struct {
	Files       []File       `json:"files"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// SnapshotName is the snapshot of the schema as it stood after the last
// migration, kept beside the migrations.
const SnapshotName = "snapshot.yaml"

// Decode reads a request, keeping numbers as their text.
func Decode(data []byte) (*Request, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var r Request
	if err := dec.Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Dialects are the SQL dialects the type-rendering idiom renders.
var Dialects = map[string]bool{"postgresql": true, "sqlserver": true, "oracle": true, "mariadb": true}

type gen struct {
	spec     map[string]any
	impl     Implementation
	owned    ownership.Owned // elements another stakeholder owns, which get no statement
	dialect  string
	idioms   map[string]Idiom
	diags    []Diagnostic
	root     string
	rows     []typerows.Row
	defaults map[string]string
	version  string         // of type-rendering
	original map[string]any // the entities as the design writes them, before their value objects are written out
}

// Generate writes the migrations of a request.
func Generate(r *Request) Response {
	g := &gen{spec: r.Specification, root: r.Root, idioms: map[string]Idiom{}}
	for _, impl := range r.Implementations {
		if t, ok := obj(obj0(impl.Content["targets"])[r.Target]); ok || len(r.Implementations) == 1 {
			g.impl = impl
			g.dialect = text(t["dialect"])
			break
		}
	}
	if g.dialect == "" {
		g.dialect = "postgresql"
	}
	g.owned = ownership.Of(g.impl.Content)
	if !Dialects[g.dialect] {
		return g.fail("/", "%s is not a SQL dialect specarch-gen-sql writes; it writes postgresql, sqlserver, oracle and mariadb", g.dialect)
	}
	for _, i := range g.impl.Idioms {
		g.idioms[i.Name] = i
	}
	tr, ok := g.idioms["type-rendering"]
	if !ok || tr.As == "excluded" || tr.Content == nil {
		return g.fail("/", "the implementation file excludes the type-rendering idiom, so there is nothing to render a column type through; keep it, or override it")
	}
	g.version = tr.Version
	g.rows = typerows.RowsOf(list(g.rendering("type-rendering", "types", g.dialect)["rows"]))
	if len(g.rows) == 0 {
		return g.fail("/", "type-rendering %s has no %s rows; add them in an override of its types part", g.version, g.dialect)
	}
	g.defaults = names(g.rendering("type-rendering", "defaults", g.dialect))
	g.flattenValueObjects()

	tables := g.tables()
	if len(g.diags) > 0 {
		return Response{Files: []File{}, Diagnostics: g.diags}
	}
	snapshot := g.snapshot()
	var previous string
	var last int
	migration := regexp.MustCompile(`^([0-9]{4})_[a-z]+\.sql$`)
	for _, f := range r.Existing {
		if f.Path == SnapshotName {
			previous = f.Content
		}
		if m := migration.FindStringSubmatch(f.Path); m != nil {
			n, _ := strconv.Atoi(m[1])
			last = max(last, n)
		}
	}
	switch {
	case previous == snapshot:
		return Response{Files: []File{{Path: SnapshotName, Content: snapshot}}, Diagnostics: []Diagnostic{}}
	case previous == "" && last > 0:
		return g.fail("/", "the output folder holds migrations up to %04d but no %s, so what they built cannot be known; restore the snapshot, or start a new folder", last, SnapshotName)
	case previous != "":
		return g.next(r, previous, last, snapshot)
	}
	var b strings.Builder
	b.WriteString(g.header(r))
	var fks []string
	for _, t := range tables {
		b.WriteString("\n")
		b.WriteString(t.create(g.dialect))
		fks = append(fks, t.foreignKeys...)
	}
	for _, fk := range fks {
		b.WriteString("\n" + fk + ";\n")
	}
	views := g.createViews()
	if len(g.diags) > 0 {
		return Response{Files: []File{}, Diagnostics: g.diags}
	}
	for _, v := range views {
		b.WriteString("\n" + v + ";\n")
	}
	return Response{Files: []File{{Path: "0001_expand.sql", Content: b.String()}, {Path: SnapshotName, Content: snapshot}}, Diagnostics: []Diagnostic{}}
}

// header is the first line of every migration.
func (g *gen) header(r *Request) string {
	info := obj0(g.spec["info"])
	return fmt.Sprintf("-- Generated by specarch-gen-sql from %s, version %s; meta-model %s; %s through type-rendering %s. Do not edit this file: a change to the schema is a new migration.\n", relRoot(r.Root, r.Output), text(info["version"]), r.Specarch, g.dialect, g.version)
}

// next writes the migrations of what changed since the snapshot: an expand
// file, and a contract file when the target's settings allow destructive
// steps.
func (g *gen) next(r *Request, previous string, last int, snapshot string) Response {
	s := g.diff(previous)
	if len(g.diags) > 0 || s == nil {
		return Response{Files: []File{}, Diagnostics: g.diags}
	}
	if len(s.contract) > 0 && g.impl.Settings["destructive"] != true {
		for _, what := range s.destructive {
			g.problem("/", "the change would %s, which can lose data; set destructive: true in the sql target's settings for the run that writes it in a contract migration of its own, then take the setting out", what)
		}
		return Response{Files: []File{}, Diagnostics: g.diags}
	}
	files := []File{}
	write := func(kind string, stmts []string) {
		if len(stmts) == 0 {
			return
		}
		last++
		var b strings.Builder
		b.WriteString(g.header(r))
		for _, stmt := range stmts {
			b.WriteString("\n" + stmt + ";\n")
		}
		files = append(files, File{Path: fmt.Sprintf("%04d_%s.sql", last, kind), Content: b.String()})
	}
	write("expand", s.expand)
	write("contract", s.contract)
	files = append(files, File{Path: SnapshotName, Content: snapshot})
	return Response{Files: files, Diagnostics: []Diagnostic{}}
}

func (g *gen) fail(path, format string, args ...any) Response {
	g.problem(path, format, args...)
	return Response{Files: []File{}, Diagnostics: g.diags}
}

func (g *gen) problem(path, format string, args ...any) {
	g.diags = append(g.diags, Diagnostic{File: g.root, Line: 1, Severity: "error", Path: path, Rule: "generator", Message: fmt.Sprintf(format, args...)})
}

// rendering is one part of an idiom for one stack, by the lookup order:
// the override's part for the stack, the shipped part for the stack, the
// shipped part under any.
func (g *gen) rendering(idiom, part, stack string) map[string]any {
	i, ok := g.idioms[idiom]
	if !ok || i.As == "excluded" {
		return nil
	}
	if i.Override != nil {
		ov := obj0(i.Override["overrides"])
		replaces := ov["whole"] == true
		for _, p := range list(ov["parts"]) {
			replaces = replaces || text(p) == part
		}
		if r, ok := obj(obj0(obj0(obj0(i.Override["parts"])[part])["stack"])[stack]); replaces && ok {
			return r
		}
	}
	stacks := obj0(obj0(obj0(i.Content["parts"])[part])["stack"])
	if r, ok := obj(stacks[stack]); ok {
		return r
	}
	r, _ := obj(stacks["any"])
	return r
}

func names(r map[string]any) map[string]string {
	out := map[string]string{}
	for k, v := range obj0(r["names"]) {
		out[k] = text(v)
	}
	return out
}

// snapshot is the part of the specification the schema is made from, with
// the dialect and the type-rendering version, as YAML with sorted keys.
func (g *gen) snapshot() string {
	entities := map[string]any{}
	for name, e := range obj0(g.spec["entities"]) {
		entities[name] = keep(obj0(e), entityKeys)
	}
	enums := map[string]any{}
	for name, e := range obj0(g.spec["enums"]) {
		enums[name] = keep(obj0(e), map[string]any{"type": true, "enum": true})
	}
	s := map[string]any{"dialect": g.dialect, "typeRendering": g.version, "entities": entities, "enums": enums, "mappings": tableMappings(g.impl)}
	if views := obj0(g.spec["views"]); len(views) > 0 {
		kept := map[string]any{}
		for name, v := range views {
			kept[name] = keep(obj0(v), viewKeys)
		}
		s["views"] = kept // only when there are views, so a schema without any keeps its snapshot
	}
	// The owned entities and views keep their shape above, so taking one
	// back starts from it, and are listed here, so the next migration
	// neither alters nor drops them; only when there are any, so a schema
	// without any keeps its snapshot.
	owned := map[string]any{}
	for _, section := range []string{"entities", "views"} {
		for name := range obj0(g.spec[section]) {
			if ptr := ownership.Entity(section, name); g.owned.Covers(ptr) {
				owned[ptr] = true
			}
		}
	}
	if len(owned) > 0 {
		s["owned"] = sortedKeys(owned)
	}
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(plainValues(s)); err != nil {
		return ""
	}
	return "# The schema as it stood after the last migration, written by specarch-gen-sql. Do not edit this file.\n" + b.String()
}

// fieldKeys are the keys of a field the schema is made from; true keeps a
// key's value whole, a map keeps the listed keys of each entry.
var fieldKeys = map[string]any{"$ref": true, "type": true, "format": true, "enum": true, "maxLength": true,
	"precision": true, "scale": true, "default": true, "atRest": true, "lookup": true, "items": true, "storage": true}

var entityKeys = map[string]any{
	"required": true, "primaryKey": true, "audited": true, "deletion": true,
	"properties":  fieldKeys,
	"relations":   map[string]any{"target": true, "kind": true, "via": true, "onDelete": true},
	"constraints": map[string]any{"kind": true, "fields": true, "expression": true, "where": true},
}

// keep is a value with only the listed keys; a key listed with a map
// keeps that map's keys of each of its entries, and items lists of fields.
func keep(v map[string]any, keys map[string]any) map[string]any {
	out := map[string]any{}
	for k, val := range v {
		switch want := keys[k].(type) {
		case bool:
			if k == "items" {
				out[k] = keep(obj0(val), fieldKeys)
			} else {
				out[k] = val
			}
		case map[string]any:
			entries := map[string]any{}
			for name, e := range obj0(val) {
				entries[name] = keep(obj0(e), want)
			}
			out[k] = entries
		}
	}
	return out
}

// plainValues turns json.Number into a YAML-friendly form.
func plainValues(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := map[string]any{}
		for k, val := range x {
			m[k] = plainValues(val)
		}
		return m
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = plainValues(val)
		}
		return out
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		f, _ := x.Float64()
		return f
	}
	return v
}

func tableMappings(impl Implementation) map[string]any {
	out := map[string]any{}
	for k, v := range obj0(impl.Content["mappings"]) {
		if strings.HasPrefix(k, "#/entities/") || strings.HasPrefix(k, "#/views/") {
			out[k] = text(obj0(v)["target"])
		}
	}
	return out
}

// table is one table to create.
type table struct {
	name        string
	columns     []string
	constraints []string
	indexes     []string // statements after the table, such as a filtered unique index
	foreignKeys []string // statements after every table
}

func (t table) create(dialect string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE %s (\n", t.name)
	lines := append(append([]string{}, t.columns...), t.constraints...)
	for i, l := range lines {
		b.WriteString("    " + l)
		if i < len(lines)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(");\n")
	for _, ix := range t.indexes {
		b.WriteString(ix + ";\n")
	}
	return b.String()
}

// column is what a field becomes.
type column struct {
	name, hash string // hash is the companion of an encrypted field looked up by hash
	nullable   bool
}

// tables renders every entity as a table, in name order.
func (g *gen) tables() []table {
	entities := obj0(g.spec["entities"])
	var out []table
	for _, name := range sortedKeys(entities) {
		if g.owned.Covers(ownership.Entity("entities", name)) {
			continue
		}
		t := g.table(name, obj0(entities[name]))
		g.checkNames(name, t)
		out = append(out, t)
	}
	return out
}

func (g *gen) table(entity string, e map[string]any) table {
	t := table{name: g.tableName(entity)}
	at := "/entities/" + entity
	props := obj0(e["properties"])
	required := map[string]bool{}
	for _, r := range list(e["required"]) {
		required[text(r)] = true
	}
	pk := texts(list(e["primaryKey"]))
	order := append([]string{}, pk...)
	inPK := map[string]bool{}
	for _, p := range pk {
		inPK[p] = true
	}
	for _, f := range sortedKeys(props) {
		if !inPK[f] {
			order = append(order, f)
		}
	}
	cols := map[string]column{}
	for _, f := range order {
		field := obj0(props[f])
		if field == nil {
			continue
		}
		fc, ok := g.fieldColumn(t.name, at, f, field, required[f], inPK[f])
		if !ok {
			continue
		}
		cols[f] = fc.column
		t.columns = append(t.columns, fc.lines...)
		if fc.check != "" {
			t.constraints = append(t.constraints, fc.check)
		}
	}
	if e["audited"] == true {
		audit := names(g.rendering("audit-fields", "columns", "any"))
		for _, a := range []struct{ key, format string }{{"createdAt", "date-time"}, {"createdBy", ""}, {"createdByName", ""}, {"lastModifiedAt", "date-time"}, {"lastModifiedBy", ""}, {"lastModifiedByName", ""}} {
			name := audit[a.key]
			if name == "" {
				continue
			}
			field := map[string]any{"type": "string", "maxLength": json.Number("255")}
			if a.format != "" {
				field = map[string]any{"type": "string", "format": a.format}
			}
			typ, _, _ := g.render(field, name)
			t.columns = append(t.columns, columnLine(name, typ, "", false))
		}
	}
	if text(e["deletion"]) == "soft" {
		name := names(g.rendering("soft-delete", "column", "any"))["deleted"]
		typ, check, _ := g.render(map[string]any{"type": "boolean"}, name)
		t.columns = append(t.columns, columnLine(name, typ, g.defaults["false"], false))
		if check != "" {
			t.constraints = append(t.constraints, fmt.Sprintf("CONSTRAINT ck_%s_%s CHECK (%s)", t.name, name, check))
		}
	}
	g.addConstraints(&t, at, props, cols, pk, obj0(e["constraints"]))
	rels := obj0(e["relations"])
	for _, rn := range sortedKeys(rels) {
		rel := obj0(rels[rn])
		if kind := text(rel["kind"]); kind == "many-to-one" || kind == "one-to-one" {
			t.foreignKeys = append(t.foreignKeys, g.foreignKey(t.name, rel))
		}
	}
	return t
}

// fixedFormats are the string formats whose width the format itself fixes.
var fixedFormats = map[string]bool{"uuid": true, "date": true, "date-time": true, "time": true, "duration": true, "decimal": true, "int64": true, "uint64": true}

// keyable reports whether a field can be part of a key on every engine: not
// a list or an object, and text no wider than 255 characters. SQL Server
// indexes at most 900 bytes and MariaDB with utf8mb4 at most 3072; 255
// characters fit under both.
func keyable(field map[string]any) bool {
	s := typerows.ShapeOf(field)
	switch {
	case s.Type == "array" || s.Type == "object":
		return false
	case s.Type != "string" || s.Enum || fixedFormats[s.Format]:
		return true
	}
	return s.MaxLength != nil && *s.MaxLength <= 255
}

// addConstraints adds a table's primary key and its declared unique and
// check constraints. cols holds the column of each field.
func (g *gen) addConstraints(t *table, at string, props map[string]any, cols map[string]column, pk []string, cs map[string]any) {
	keyCols := func(fields []string) ([]string, bool) {
		var out []string
		nullable := false
		for _, f := range fields {
			if field := obj0(props[f]); field != nil && text(field["atRest"]) != "encrypted" && !keyable(field) {
				g.problem(at+"/properties/"+f, "%s is part of a key or a unique constraint, and its text has no maxLength of at most 255, so not every engine can index it (the key-width statement of type-rendering); give it a maxLength", f)
			}
			c := cols[f]
			name := c.name
			if c.hash != "" {
				name = c.hash
			}
			out = append(out, name)
			nullable = nullable || c.nullable
		}
		return out, nullable
	}
	if len(pk) > 0 {
		k, _ := keyCols(pk)
		t.constraints = append([]string{fmt.Sprintf("CONSTRAINT pk_%s PRIMARY KEY (%s)", t.name, strings.Join(k, ", "))}, t.constraints...)
	}
	for _, name := range sortedKeys(cs) {
		c := obj0(cs[name])
		switch text(c["kind"]) {
		case "unique":
			k, nullable := keyCols(texts(list(c["fields"])))
			if where := text(c["where"]); where != "" {
				g.partialUnique(t, at, name, where, props, k, nullable)
				continue
			}
			if g.dialect == "sqlserver" && nullable {
				// SQL Server treats two NULLs as equal in a unique
				// constraint; the other engines do not, so a filtered index
				// gives the same meaning.
				var where []string
				for _, col := range k {
					where = append(where, col+" IS NOT NULL")
				}
				t.indexes = append(t.indexes, fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s) WHERE %s", name, t.name, strings.Join(k, ", "), strings.Join(where, " AND ")))
				continue
			}
			t.constraints = append(t.constraints, fmt.Sprintf("CONSTRAINT %s UNIQUE (%s)", name, strings.Join(k, ", ")))
		case "check":
			sql, err := g.check(text(c["expression"]), props)
			if err != "" {
				g.problem(at+"/constraints/"+name, "the check %s cannot be written in %s: %s", name, g.dialect, err)
				continue
			}
			t.constraints = append(t.constraints, fmt.Sprintf("CONSTRAINT %s CHECK (%s)", name, sql))
		}
	}
}

// partialUnique adds a unique constraint that holds only where its
// condition does, as a partial unique index (PostgreSQL) or a filtered one
// (SQL Server). Oracle and MariaDB have no index with a condition, so the
// constraint is refused there rather than written as a plain unique one,
// which would refuse rows the design allows.
func (g *gen) partialUnique(t *table, at, name, where string, props map[string]any, k []string, nullable bool) {
	at += "/constraints/" + name
	if g.dialect != "postgresql" && g.dialect != "sqlserver" {
		g.problem(at, "the unique constraint %s holds only where %s, and %s has no partial unique index; generate it for postgresql or sqlserver, or take the condition out", name, where, g.dialect)
		return
	}
	n, errs := expr.Parse(where)
	if len(errs) > 0 || n == nil {
		g.problem(at, "the condition of %s cannot be written in %s: it does not parse", name, g.dialect)
		return
	}
	tr := &translator{g: g, props: props, filter: g.dialect == "sqlserver"}
	if tr.filter {
		if why := filterPredicate(n, props); why != "" {
			g.problem(at, "the condition of %s cannot be a SQL Server filtered index: %s", name, why)
			return
		}
	}
	conds := []string{tr.tr(n, true)}
	if tr.err != "" {
		g.problem(at, "the condition of %s cannot be written in %s: %s", name, g.dialect, tr.err)
		return
	}
	if g.dialect == "sqlserver" && nullable {
		// As for a unique constraint without a condition: SQL Server treats
		// two NULLs as equal, the others do not.
		for _, col := range k {
			conds = append(conds, col+" IS NOT NULL")
		}
	}
	t.indexes = append(t.indexes, fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s) WHERE %s", name, t.name, strings.Join(k, ", "), strings.Join(conds, " AND ")))
}

// filterPredicate says why a condition is not one SQL Server takes in a
// filtered index, or "": its filter is conditions joined by AND, each a
// column compared with a constant (CREATE INDEX, filter_predicate). A
// boolean field on its own, or negated, is the column compared with 1 or 0.
func filterPredicate(n *expr.Node, props map[string]any) string {
	isField := func(a *expr.Node) bool { return a.Op == expr.OpName }
	isBoolField := func(a *expr.Node) bool {
		return isField(a) && text(obj0(props[a.Text])["type"]) == "boolean"
	}
	isConstant := func(a *expr.Node) bool {
		switch a.Op {
		case expr.OpInt, expr.OpUint, expr.OpDouble, expr.OpText, expr.OpBool, expr.OpNull:
			return true
		case expr.OpNeg:
			return a.Args[0].Op == expr.OpInt || a.Args[0].Op == expr.OpDouble
		}
		return false
	}
	switch n.Op {
	case "&&":
		if why := filterPredicate(n.Args[0], props); why != "" {
			return why
		}
		return filterPredicate(n.Args[1], props)
	case "==", "!=", "<", "<=", ">", ">=":
		if isField(n.Args[0]) && isConstant(n.Args[1]) {
			return ""
		}
		if isConstant(n.Args[0]) && isField(n.Args[1]) {
			return "write the field before the value it is compared with"
		}
		return "it compares only a field with a value"
	case expr.OpName:
		if isBoolField(n) {
			return ""
		}
	case expr.OpNot:
		if isBoolField(n.Args[0]) {
			return ""
		}
	case "||":
		return "it joins conditions only with &&"
	}
	return "it takes only fields compared with values, joined with &&"
}

// constraintsOnly renders the declared constraints of an entity given as
// plain values, without its columns, for the differ.
func (g *gen) constraintsOnly(entity string, e map[string]any) table {
	t := table{name: g.tableName(entity)}
	props := obj0(e["properties"])
	required := map[string]bool{}
	for _, r := range list(e["required"]) {
		required[text(r)] = true
	}
	cols := map[string]column{}
	suffix := names(g.rendering("encrypted-column", "column", "any"))["hashSuffix"]
	for f, v := range props {
		field := obj0(v)
		c := column{name: snake(f), nullable: !required[f] || nullType(field)}
		if text(field["atRest"]) == "encrypted" && text(field["lookup"]) == "hash" {
			c.hash = c.name + suffix
		}
		cols[f] = c
	}
	g.addConstraints(&t, "/entities/"+entity, props, cols, nil, obj0(e["constraints"]))
	return t
}

// foreignKey is the statement that adds the foreign key of a relation.
func (g *gen) foreignKey(tableName string, rel map[string]any) string {
	via, target := text(rel["via"]), text(rel["target"])
	var tk []string
	for _, f := range texts(list(obj0(obj0(g.spec["entities"])[target])["primaryKey"])) {
		tk = append(tk, snake(f))
	}
	fk := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT fk_%s_%s FOREIGN KEY (%s) REFERENCES %s (%s)", tableName, tableName, snake(via), snake(via), g.tableName(target), strings.Join(tk, ", "))
	if on := g.onDelete(text(rel["onDelete"])); on != "" {
		fk += " " + on
	}
	return fk
}

// renderPattern is the render of the row that matches a field, unfilled.
func (g *gen) renderPattern(field map[string]any) (string, bool, bool) {
	row, ok := typerows.Match(typerows.ShapeOf(field), g.rows)
	return row.Render, ok, ok
}

// fieldCol is what one field becomes: its column, the column lines (the
// value, and the hash beside an encrypted value), its type, and the check
// constraint its row asks for.
type fieldCol struct {
	column
	lines []string
	typ   string
	check string
}

// fieldColumn renders one field of a table.
func (g *gen) fieldColumn(tableName, at, f string, field map[string]any, required, inPK bool) (fieldCol, bool) {
	fc := fieldCol{column: column{name: snake(f), nullable: !required || nullType(field)}}
	def := ""
	if v, ok := field["default"]; ok {
		def = g.literal(v)
	}
	var check string
	if text(field["atRest"]) == "encrypted" {
		enc := names(g.rendering("encrypted-column", "column", g.dialect))
		if enc["ciphertext"] == "" {
			g.problem(at+"/properties/"+f, "%s is encrypted at rest, and the encrypted-column idiom has no %s column for it; add one in an override of its column part", f, g.dialect)
			return fc, false
		}
		fc.typ = enc["ciphertext"]
		if text(field["lookup"]) == "hash" {
			fc.hash = fc.name + names(g.rendering("encrypted-column", "column", "any"))["hashSuffix"]
		}
		if inPK {
			g.problem(at+"/primaryKey", "%s is encrypted at rest and part of the primary key; a key on the hash of a value is not written yet, so give the entity another key", f)
		}
	} else {
		var ok bool
		fc.typ, check, ok = g.render(field, fc.name)
		if !ok {
			g.problem(at+"/properties/"+f, "type-rendering %s has no %s row for %s (%s); add one in an override of its types part", g.version, g.dialect, f, typerows.ShapeOf(field).Describe())
			return fc, false
		}
	}
	fc.lines = append(fc.lines, columnLine(fc.name, fc.typ, def, fc.nullable))
	if fc.hash != "" {
		fc.lines = append(fc.lines, columnLine(fc.hash, names(g.rendering("encrypted-column", "column", g.dialect))["hash"], "", fc.nullable))
	}
	if check != "" {
		fc.check = fmt.Sprintf("CONSTRAINT ck_%s_%s CHECK (%s)", tableName, fc.name, check)
	}
	return fc, true
}

// onDelete is the ON DELETE clause of a relation on the dialect.
func (g *gen) onDelete(on string) string {
	switch on {
	case "cascade":
		return "ON DELETE CASCADE"
	case "set_null":
		return "ON DELETE SET NULL"
	}
	switch g.dialect {
	case "oracle":
		return "" // Oracle restricts by default and has no keyword for it
	case "sqlserver":
		return "ON DELETE NO ACTION" // SQL Server's name for restrict
	}
	return "ON DELETE RESTRICT"
}

func columnLine(name, typ, def string, nullable bool) string {
	l := name + " " + typ
	if def != "" {
		// DEFAULT before NOT NULL: Oracle refuses the other order, and
		// every engine takes this one.
		l += " DEFAULT " + def
	}
	if !nullable {
		l += " NOT NULL"
	}
	return l
}

// render is a field's column type and the check its row asks for.
func (g *gen) render(field map[string]any, column string) (typ, check string, ok bool) {
	shape := typerows.ShapeOf(field)
	values := g.enumValues(field)
	row, ok := typerows.Match(shape, g.rows)
	if !ok {
		return "", "", false
	}
	typ = row.Render
	longest := 0
	var quoted []string
	for _, v := range values {
		longest = max(longest, len([]rune(v)))
		quoted = append(quoted, quote(v))
	}
	scale := text(field["scale"])
	if scale == "" {
		scale = "0"
	}
	repl := map[string]string{
		"{maxLength}": text(field["maxLength"]), "{precision}": text(field["precision"]), "{scale}": scale,
		"{longestValue}": strconv.Itoa(longest), "{column}": column, "{values}": strings.Join(quoted, ", "),
	}
	if strings.Contains(typ, "{items}") {
		items, _ := obj(field["items"])
		it, _, ok := g.render(items, column)
		if !ok {
			return "", "", false
		}
		repl["{items}"] = it
	}
	for k, v := range repl {
		typ = strings.ReplaceAll(typ, k, v)
	}
	check = row.Check
	for k, v := range repl {
		check = strings.ReplaceAll(check, k, v)
	}
	return typ, check, true
}

// enumValues are the values of an enum field, by $ref or inline.
func (g *gen) enumValues(field map[string]any) []string {
	if ref := text(field["$ref"]); strings.HasPrefix(ref, "#/enums/") {
		return texts(list(obj0(obj0(g.spec["enums"])[strings.TrimPrefix(ref, "#/enums/")])["enum"]))
	}
	return texts(list(field["enum"]))
}

func nullType(field map[string]any) bool {
	for _, t := range list(field["type"]) {
		if text(t) == "null" {
			return true
		}
	}
	return false
}

// literal writes a default value in the dialect.
func (g *gen) literal(v any) string {
	switch x := v.(type) {
	case bool:
		return g.defaults[strconv.FormatBool(x)]
	case json.Number:
		return x.String()
	}
	return quote(text(v))
}

func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// tableName is the entity's table: the mapping's when it names one
// (table loans), otherwise the entity's name in snake case.
func (g *gen) tableName(entity string) string {
	target := text(obj0(obj0(g.impl.Content["mappings"])["#/entities/"+entity])["target"])
	if rest, ok := strings.CutPrefix(target, "table "); ok {
		if i := strings.IndexAny(rest, " ,"); i >= 0 {
			rest = rest[:i]
		}
		if rest != "" {
			return rest
		}
	}
	return snake(entity)
}

// snake writes a camelCase or PascalCase name in snake case: dueOn is
// due_on, LoanStatus is loan_status.
func snake(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		if unicode.IsUpper(r) {
			if i > 0 && (unicode.IsLower(rs[i-1]) || unicode.IsDigit(rs[i-1]) || i+1 < len(rs) && unicode.IsLower(rs[i+1]) && unicode.IsUpper(rs[i-1])) {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// check translates a check expression into the dialect, or says why it
// cannot be.
func (g *gen) check(src string, props map[string]any) (string, string) {
	n, errs := expr.Parse(src)
	if len(errs) > 0 || n == nil {
		return "", "it does not parse"
	}
	t := &translator{g: g, props: props}
	out := t.tr(n, true)
	return out, t.err
}

type translator struct {
	g     *gen
	props map[string]any
	err   string
	// filter is set for a SQL Server filtered index, whose filter has no
	// NOT: a negated boolean field is written as the field equal to 0.
	filter bool
}

func (t *translator) fail(format string, args ...any) string {
	if t.err == "" {
		t.err = fmt.Sprintf(format, args...)
	}
	return ""
}

func (t *translator) isBoolean(n *expr.Node) bool {
	if n.Op == expr.OpName {
		return text(obj0(t.props[n.Text])["type"]) == "boolean"
	}
	return false
}

// tr writes a node; cond says the node stands where a condition is wanted.
func (t *translator) tr(n *expr.Node, cond bool) string {
	numericBool := t.g.dialect == "sqlserver" || t.g.dialect == "oracle"
	switch n.Op {
	case expr.OpName:
		f := obj0(t.props[n.Text])
		if f == nil {
			return t.fail("%s is not a field of the entity", n.Text)
		}
		if text(f["atRest"]) == "encrypted" {
			return t.fail("%s is encrypted at rest, and the engine cannot read it in a check", n.Text)
		}
		if cond && numericBool && t.isBoolean(n) {
			return "(" + snake(n.Text) + " = 1)"
		}
		return snake(n.Text)
	case expr.OpInt, expr.OpUint:
		return n.Int.String()
	case expr.OpDouble:
		return strconv.FormatFloat(n.Double, 'f', -1, 64)
	case expr.OpText:
		return quote(n.Text)
	case expr.OpBool:
		switch {
		case numericBool && cond && n.Bool:
			return "(1 = 1)"
		case numericBool && cond:
			return "(1 = 0)"
		case numericBool && n.Bool:
			return "1"
		case numericBool:
			return "0"
		case n.Bool:
			return "TRUE"
		}
		return "FALSE"
	case expr.OpNull:
		return "NULL"
	case expr.OpNot:
		if t.filter && t.isBoolean(n.Args[0]) {
			return "(" + snake(n.Args[0].Text) + " = 0)"
		}
		return "NOT " + t.tr(n.Args[0], true)
	case expr.OpNeg:
		return "(-" + t.tr(n.Args[0], false) + ")"
	case expr.OpCond:
		return "(CASE WHEN " + t.tr(n.Args[0], true) + " THEN " + t.tr(n.Args[1], false) + " ELSE " + t.tr(n.Args[2], false) + " END)"
	case "&&", "||":
		op := "AND"
		if n.Op == "||" {
			op = "OR"
		}
		return "(" + t.tr(n.Args[0], true) + " " + op + " " + t.tr(n.Args[1], true) + ")"
	case "==", "!=":
		l, r := n.Args[0], n.Args[1]
		if r.Op == expr.OpNull || l.Op == expr.OpNull {
			if l.Op == expr.OpNull {
				l = r
			}
			if n.Op == "==" {
				return "(" + t.tr(l, false) + " IS NULL)"
			}
			return "(" + t.tr(l, false) + " IS NOT NULL)"
		}
		op := "="
		if n.Op == "!=" {
			op = "<>"
		}
		return "(" + t.tr(l, false) + " " + op + " " + t.tr(r, false) + ")"
	case "+", "-":
		if d := n.Args[1]; d.Op == expr.OpCall && d.Text == "duration" {
			return t.addDays(n.Op, t.tr(n.Args[0], false), d)
		}
		return "(" + t.tr(n.Args[0], false) + " " + n.Op + " " + t.tr(n.Args[1], false) + ")"
	case "<", "<=", ">", ">=", "*", "/":
		return "(" + t.tr(n.Args[0], false) + " " + n.Op + " " + t.tr(n.Args[1], false) + ")"
	case expr.OpCall:
		return t.call(n)
	}
	return t.fail("%s is not written in SQL by this version", n.Op)
}

// addDays writes a date moved by whole days, which validate has checked
// the duration to be: PostgreSQL and Oracle add a number of days to a date
// and keep a date, where adding an interval would give a timestamp.
func (t *translator) addDays(op, date string, d *expr.Node) string {
	dur, ok := expr.ParseDuration(d.Args[0].Text)
	if !ok || dur%(24*time.Hour) != 0 {
		return t.fail("%s is not a whole number of days", d.Args[0].Text)
	}
	days := int64(dur / (24 * time.Hour))
	if op == "-" {
		days = -days
	}
	switch t.g.dialect {
	case "sqlserver":
		return fmt.Sprintf("DATEADD(day, %d, %s)", days, date)
	case "mariadb":
		return fmt.Sprintf("(%s + INTERVAL %d DAY)", date, days)
	}
	if days < 0 {
		return fmt.Sprintf("(%s - %d)", date, -days)
	}
	return fmt.Sprintf("(%s + %d)", date, days)
}

func (t *translator) call(n *expr.Node) string {
	d := t.g.dialect
	args := make([]string, len(n.Args))
	for i, a := range n.Args {
		args[i] = t.tr(a, false)
	}
	switch n.Text {
	case "date":
		if d == "oracle" {
			return "TRUNC(" + args[0] + ")"
		}
		return "CAST(" + args[0] + " AS DATE)"
	case "size":
		if d == "sqlserver" {
			return "LEN(" + args[0] + ")"
		}
		return "LENGTH(" + args[0] + ")"
	case "round":
		return "ROUND(" + strings.Join(args, ", ") + ")"
	case "floor":
		return "FLOOR(" + args[0] + ")"
	case "ceil":
		if d == "sqlserver" {
			return "CEILING(" + args[0] + ")"
		}
		return "CEIL(" + args[0] + ")"
	case "min":
		return "LEAST(" + strings.Join(args, ", ") + ")" // SQL Server 2022 and later
	case "max":
		return "GREATEST(" + strings.Join(args, ", ") + ")" // SQL Server 2022 and later
	}
	return t.fail("the function %s is not written in SQL by this version", n.Text)
}

// relRoot is the root file's path as seen from the output folder.
func relRoot(root, output string) string {
	absRoot, err1 := filepath.Abs(filepath.FromSlash(root))
	absOut, err2 := filepath.Abs(filepath.FromSlash(output))
	if err1 != nil || err2 != nil {
		return root
	}
	rel, err := filepath.Rel(absOut, absRoot)
	if err != nil {
		return root
	}
	return filepath.ToSlash(rel)
}

func obj(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func obj0(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func texts(l []any) []string {
	var out []string
	for _, v := range l {
		out = append(out, text(v))
	}
	return out
}

func text(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	}
	return fmt.Sprint(v)
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
