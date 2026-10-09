package extract

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
)

// The catalogue dump tools/catalogue/catalogue.sql writes.
type catalogue struct {
	Catalogue string         `json:"catalogue"`
	Path      string         `json:"path"`
	Commit    string         `json:"commit"`
	Tables    []dumpTable    `json:"tables"`
	Views     []dumpView     `json:"views"`
	Enums     []dumpEnumType `json:"enums"`
}

type dumpTable struct {
	Schema      string           `json:"schema"`
	Name        string           `json:"name"`
	Comment     *string          `json:"comment"`
	Columns     []dumpColumn     `json:"columns"`
	Constraints []dumpConstraint `json:"constraints"`
	Indexes     []dumpIndex      `json:"indexes"`
}

type dumpColumn struct {
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	NotNull   bool    `json:"notNull"`
	Default   *string `json:"default"`
	Identity  *string `json:"identity"`
	Generated *string `json:"generated"`
	Comment   *string `json:"comment"`
}

type dumpConstraint struct {
	Name              string   `json:"name"`
	Kind              string   `json:"kind"`
	Columns           []string `json:"columns"`
	Definition        string   `json:"definition"`
	ReferencesSchema  *string  `json:"referencesSchema"`
	ReferencesTable   *string  `json:"referencesTable"`
	ReferencesColumns []string `json:"referencesColumns"`
	OnDelete          *string  `json:"onDelete"`
}

type dumpIndex struct {
	Name            string `json:"name"`
	Unique          bool   `json:"unique"`
	Primary         bool   `json:"primary"`
	BacksConstraint bool   `json:"backsConstraint"`
	Definition      string `json:"definition"`
}

type dumpView struct {
	Schema       string `json:"schema"`
	Name         string `json:"name"`
	Materialized bool   `json:"materialized"`
}

type dumpEnumType struct {
	Schema string   `json:"schema"`
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// field is one column as the reader writes it.
type field struct {
	column   string
	name     string
	kind     string // the expression subset's kind: string, int, decimal, double, bool, date, timestamp, time, duration, bytes, enum, list
	bits     int
	scale    int
	nullable bool
	values   []string // an enum's values
	node     *yaml.Node
}

var fullHash = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
var plainName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
var constraintName = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// Database reads a catalogue dump into entities, one per table.
func Database(dumpPath, out, key string) (*Result, error) {
	data, err := os.ReadFile(dumpPath)
	if err != nil {
		return nil, err
	}
	var c catalogue
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, refuse("%s is not a catalogue dump: %v", dumpPath, err)
	}
	if c.Catalogue != "postgresql" {
		return nil, refuse("%s is not a catalogue dump of tools/catalogue/catalogue.sql: its catalogue is %q, not postgresql", dumpPath, c.Catalogue)
	}
	r, dumpName, err := openDump(dumpPath, c.Path, c.Commit, "tools/catalogue/dump-catalogue.sh")
	if err != nil {
		return nil, err
	}
	res := &Result{Tree: newTree()}
	res.say("commit %s: the last change to %s, as the dump %s names it", r.Commit, c.Path, dumpName)
	d := &dbReader{c: &c, key: key, clause: c.Path, res: res, enums: map[string][]string{}, entities: map[string]string{}}
	for _, e := range c.Enums {
		d.enums[e.Name] = e.Values
		d.enums[e.Schema+"."+e.Name] = e.Values
	}
	d.read()
	src, err := codeSource(r, out, []*yaml.Node{flow(mapping("clause", c.Path, "title", "The database catalogue after every migration here is applied"))})
	if err != nil {
		return nil, err
	}
	set(src, "reading", "printed")
	stages := []string{"design"}
	if len(d.questions.Content) > 0 {
		stages = []string{"requirements", "design"}
		res.Tree.put("requirements/stakeholders.yaml", mapping("stakeholders", ownerStakeholder()))
		res.Tree.put("design/questions.yaml", mapping("questions", d.questions))
	}
	if len(d.entityFiles) == 0 && len(d.questions.Content) == 0 {
		stages = nil
	}
	for name, n := range d.entityFiles {
		res.Tree.put(name, n)
	}
	description := fmt.Sprintf("The tables of the database that the migrations in %s make, read from the catalogue dump %s, made at commit %s. Every entity cites its table; what the catalogue does not say is a question, and so is what the meta-model cannot hold.\n", c.Path, dumpName, r.Commit)
	res.Tree.put("specarch.yaml", rootFile("Database of "+c.Path, description, stages, mapping(key, src)))
	return res, nil
}

type dbReader struct {
	c           *catalogue
	key, clause string
	res         *Result
	enums       map[string][]string
	entities    map[string]string // entity name by schema.table
	entityFiles map[string]*yaml.Node
	questions   *yaml.Node
	nextID      int
	notHeld     []notHeld
}

// gap records what the meta-model cannot hold, at the table or view the
// catalogue lists it under (schema.name), blocking the entry it is about.
func (d *dbReader) gap(table, block string, format string, args ...any) {
	d.notHeld = append(d.notHeld, notHeld{text: fmt.Sprintf(format, args...), clause: d.clause + " " + table, blocks: []string{block}})
}

func (d *dbReader) read() {
	d.entityFiles = map[string]*yaml.Node{}
	d.questions = &yaml.Node{Kind: yaml.MappingNode}
	tables := append([]dumpTable(nil), d.c.Tables...)
	sort.SliceStable(tables, func(i, j int) bool {
		if tables[i].Schema != tables[j].Schema {
			return tables[i].Schema < tables[j].Schema
		}
		return tables[i].Name < tables[j].Name
	})
	// Name every table first, so that a foreign key finds its target.
	byEntity := map[string]string{}
	for _, t := range tables {
		qualified := t.Schema + "." + t.Name
		if !plainName.MatchString(t.Name) || !plainName.MatchString(t.Schema) {
			d.gap(qualified, "entities", "table %s: its name has characters an entity name cannot hold; left out", qualified)
			continue
		}
		name := pascal(t.Name)
		if t.Schema != "public" {
			name = pascal(t.Schema + "_" + t.Name)
		}
		if other, ok := byEntity[name]; ok {
			d.gap(qualified, "entities", "table %s: its entity name %s is taken by %s; left out", qualified, name, other)
			continue
		}
		byEntity[name] = qualified
		d.entities[qualified] = name
	}
	columns, constraints, checks, uniques, foreign, indexes := 0, 0, 0, 0, 0, 0
	for _, t := range tables {
		columns += len(t.Columns)
		for _, k := range t.Constraints {
			constraints++
			switch k.Kind {
			case "check":
				checks++
			case "unique":
				uniques++
			case "foreign":
				foreign++
			}
		}
		for _, ix := range t.Indexes {
			if !ix.BacksConstraint {
				indexes++
			}
		}
		if name, ok := d.entities[t.Schema+"."+t.Name]; ok {
			d.table(t, name)
		}
	}
	for _, v := range d.c.Views {
		kind := "view"
		if v.Materialized {
			kind = "materialized view"
		}
		d.gap(v.Schema+"."+v.Name, "views", "%s %s.%s: a view is not read from the catalogue; describe it under views by hand", kind, v.Schema, v.Name)
	}
	d.res.say("counted %s, %s, %s (%s, %s, %s), %s besides those of keys, %s and %s: every entry of the dump's lists, which hold every schema but pg_catalog, information_schema and pg_toast",
		plural(len(tables), "table"), plural(columns, "column"), plural(constraints, "constraint"),
		plural(foreign, "foreign key"), plural(uniques, "unique key"), plural(checks, "check"),
		plural(indexes, "index"), plural(len(d.c.Views), "view"), plural(len(d.c.Enums), "enum type"))
	d.res.say("wrote %s and %s: one per table whose name an entity can hold, one per thing the catalogue does not say, and one per thing the meta-model cannot hold", plural(len(d.entities), "entity"), plural(len(d.questions.Content)/2+couldCount(oneQuestions(d.questions), d.notHeld), "question"))
	askNotHeld(d.res, oneQuestions(d.questions), &d.nextID, d.key, d.notHeld)
}

func (d *dbReader) question(text, kind string, blocks []string, why string) {
	d.ask("must", text, kind, blocks, why)
}

func (d *dbReader) ask(priority, text, kind string, blocks []string, why string) {
	d.nextID++
	q := mapping(
		"question", text,
		"kind", kind,
		"priority", priority,
		"blocks", blocks,
		"decidedBy", owner,
		"why", why,
	)
	set(d.questions, fmt.Sprintf("Q-%d", d.nextID), q)
}

func (d *dbReader) table(t dumpTable, name string) {
	qualified := t.Schema + "." + t.Name
	at := "#/entities/" + name
	fields := map[string]*field{}
	var order []*field
	for _, col := range t.Columns {
		f := d.column(t, col)
		if f == nil {
			continue
		}
		fields[col.Name] = f
		order = append(order, f)
	}
	// A check that is exactly "column = ANY (ARRAY[...])" is the column's
	// list of allowed values, as an enum renders.
	var checks []dumpConstraint
	for _, k := range t.Constraints {
		if k.Kind != "check" {
			continue
		}
		if n, err := parseCheck(k.Definition); err == nil {
			if col, values, ok := anyValues(n); ok && fields[col] != nil && fields[col].kind == "string" && len(k.Columns) == 1 {
				f := fields[col]
				f.kind, f.values = "enum", values
				set(f.node, "enum", values)
				continue
			}
		}
		checks = append(checks, k)
	}
	props := &yaml.Node{Kind: yaml.MappingNode}
	var required []string
	for _, f := range order {
		set(props, f.name, flow(f.node))
		if !f.nullable {
			required = append(required, f.name)
		}
	}
	entity := mapping("type", "object")
	if t.Comment != nil && *t.Comment != "" {
		entity = mapping("description", *t.Comment, "type", "object")
	}
	set(entity, "properties", props)
	if len(required) > 0 {
		set(entity, "required", required)
	}
	var pk []string
	relations := &yaml.Node{Kind: yaml.MappingNode}
	constraintNodes := &yaml.Node{Kind: yaml.MappingNode}
	var messages []string
	targets := map[string]int{}
	for _, k := range t.Constraints {
		if k.Kind == "foreign" && k.ReferencesTable != nil {
			targets[*k.ReferencesSchema+"."+*k.ReferencesTable]++
		}
	}
	keys := append([]dumpConstraint(nil), t.Constraints...)
	sort.SliceStable(keys, func(i, j int) bool { return keys[i].Name < keys[j].Name })
	for _, k := range keys {
		switch k.Kind {
		case "primary":
			names, ok := fieldNames(k.Columns, fields)
			if !ok {
				d.gap(qualified, at, "primary key %s of %s: a column of it is left out", k.Name, qualified)
				continue
			}
			pk = names
		case "foreign":
			d.foreignKey(t, k, fields, relations, targets)
		case "unique":
			names, ok := fieldNames(k.Columns, fields)
			switch {
			case !ok:
				d.gap(qualified, at, "unique key %s of %s: a column of it is left out", k.Name, qualified)
			case !constraintName.MatchString(k.Name):
				d.gap(qualified, at, "unique key %s of %s: a constraint name is snake_case", k.Name, qualified)
			default:
				set(constraintNodes, k.Name, flow(mapping("kind", "unique", "fields", names)))
				messages = append(messages, k.Name)
			}
		case "exclusion":
			d.gap(qualified, at, "exclusion constraint %s of %s, %s: the meta-model has no exclusion constraint", k.Name, qualified, k.Definition)
		}
	}
	for _, ix := range t.Indexes {
		if ix.BacksConstraint {
			continue
		}
		what := "the meta-model has no index"
		if ix.Unique {
			what = "a unique index without a constraint; the meta-model says uniqueness only as a unique constraint"
		}
		d.gap(qualified, at, "index %s of %s, %s: %s", ix.Name, qualified, ix.Definition, what)
	}
	for _, k := range checks {
		if !constraintName.MatchString(k.Name) {
			d.gap(qualified, at, "check %s of %s: a constraint name is snake_case", k.Name, qualified)
			continue
		}
		expression, reason := translateCheck(k.Definition, fields)
		if reason != "" {
			d.gap(qualified, at, "check %s of %s, %s: %s", k.Name, qualified, k.Definition, reason)
			d.notHeld[len(d.notHeld)-1].asked = at + "/constraints/" + k.Name + "/expression"
			set(constraintNodes, k.Name, flow(mapping("kind", "check")))
			d.question(
				fmt.Sprintf("How is the check %s of table %s, %s, written in SpecArch's expressions, and what is a user told when it fails?", k.Name, qualified, k.Definition),
				"decision",
				[]string{at + "/constraints/" + k.Name + "/expression", at + "/constraints/" + k.Name + "/message"},
				"The catalogue holds the check, and it cannot be translated: "+reason+".",
			)
			continue
		}
		set(constraintNodes, k.Name, flow(mapping("kind", "check", "expression", expression)))
		messages = append(messages, k.Name)
	}
	if pk != nil {
		set(entity, "primaryKey", pk)
	}
	if len(relations.Content) > 0 {
		set(entity, "relations", relations)
	}
	if len(constraintNodes.Content) > 0 {
		sortPairs(constraintNodes)
		set(entity, "constraints", constraintNodes)
	}
	sort.Strings(messages)
	set(entity, "origin", "stated")
	set(entity, "cites", []*yaml.Node{citation(d.key, d.clause+" "+qualified, tableSays(t))})
	if pk == nil {
		d.question(
			fmt.Sprintf("Table %s has no primary key in the catalogue. Which fields identify one of its records?", qualified),
			"decision",
			[]string{at + "/primaryKey"},
			"Every entity names the fields that identify a record, and the catalogue gives none for this table.",
		)
	}
	if len(messages) > 0 {
		var blocks []string
		for _, m := range messages {
			blocks = append(blocks, at+"/constraints/"+m+"/message")
		}
		d.question(
			fmt.Sprintf("What is a user told when each constraint of table %s fails: %s?", qualified, strings.Join(messages, ", ")),
			"decision",
			blocks,
			"A constraint carries the message a user reads, and a database catalogue holds none.",
		)
	}
	d.entityFiles["design/entities/"+kebab(name)+".yaml"] = mapping("entities", mapping(name, entity))
}

// sortPairs puts a mapping's keys in the order of their names.
func sortPairs(m *yaml.Node) {
	type pair struct{ k, v *yaml.Node }
	var pairs []pair
	for i := 0; i+1 < len(m.Content); i += 2 {
		pairs = append(pairs, pair{m.Content[i], m.Content[i+1]})
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].k.Value < pairs[j].k.Value })
	m.Content = m.Content[:0]
	for _, p := range pairs {
		m.Content = append(m.Content, p.k, p.v)
	}
}

func tableSays(t dumpTable) string {
	var cols []string
	for _, c := range t.Columns {
		s := c.Name + " " + c.Type
		if c.NotNull {
			s += " not null"
		}
		if c.Default != nil {
			s += " default " + *c.Default
		}
		if c.Identity != nil {
			s += " identity"
		}
		cols = append(cols, s)
	}
	says := fmt.Sprintf("Table %s.%s: %s.", t.Schema, t.Name, strings.Join(cols, ", "))
	var keys []string
	for _, k := range t.Constraints {
		keys = append(keys, k.Name+" "+k.Definition)
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		says += " " + strings.Join(keys, "; ") + "."
	}
	return says
}

func fieldNames(columns []string, fields map[string]*field) ([]string, bool) {
	var names []string
	for _, c := range columns {
		f := fields[c]
		if f == nil {
			return nil, false
		}
		names = append(names, f.name)
	}
	return names, len(names) > 0
}

func (d *dbReader) foreignKey(t dumpTable, k dumpConstraint, fields map[string]*field, relations *yaml.Node, targets map[string]int) {
	qualified := t.Schema + "." + t.Name
	at := "#/entities/" + d.entities[qualified]
	ref := *k.ReferencesSchema + "." + *k.ReferencesTable
	target, ok := d.entities[ref]
	switch {
	case !ok:
		d.gap(qualified, at, "foreign key %s of %s: it references %s, which is left out", k.Name, qualified, ref)
		return
	case len(k.Columns) != 1:
		d.gap(qualified, at, "foreign key %s of %s, %s: a relation holds one field, and this key has %d", k.Name, qualified, k.Definition, len(k.Columns))
		return
	case fields[k.Columns[0]] == nil:
		d.gap(qualified, at, "foreign key %s of %s: its column is left out", k.Name, qualified)
		return
	}
	refTable := d.tableOf(ref)
	if refTable == nil || !referencesPrimaryKey(*refTable, k.ReferencesColumns) {
		d.gap(qualified, at, "foreign key %s of %s, %s: a relation points at the target's primary key, and this key references other columns", k.Name, qualified, k.Definition)
		return
	}
	kind := "many-to-one"
	for _, other := range t.Constraints {
		if (other.Kind == "unique" || other.Kind == "primary") && len(other.Columns) == 1 && other.Columns[0] == k.Columns[0] {
			kind = "one-to-one"
		}
	}
	rel := mapping("target", target, "kind", kind, "via", fields[k.Columns[0]].name)
	switch onDelete(k.OnDelete) {
	case "restrict":
		set(rel, "onDelete", "restrict")
	case "cascade":
		set(rel, "onDelete", "cascade")
	case "set null":
		set(rel, "onDelete", "set_null")
	default:
		set(rel, "onDelete", "restrict")
		d.gap(qualified, at, "foreign key %s of %s: on delete %s is not a choice of the meta-model; written as restrict", k.Name, qualified, onDelete(k.OnDelete))
	}
	name := camel(*k.ReferencesTable)
	if targets[ref] > 1 {
		name = camel(*k.ReferencesTable) + "Via" + pascal(k.Columns[0])
	}
	if !memberName.MatchString(name) {
		d.gap(qualified, at, "foreign key %s of %s: no relation name can be made from %s", k.Name, qualified, ref)
		return
	}
	set(relations, name, flow(rel))
}

var memberName = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)

// onDelete is the action as the meta-model reads it: no action, which
// checks at the end of the statement, refuses the delete as restrict does.
func onDelete(s *string) string {
	if s == nil || *s == "no action" {
		return "restrict"
	}
	return *s
}

func (d *dbReader) tableOf(qualified string) *dumpTable {
	for i := range d.c.Tables {
		if d.c.Tables[i].Schema+"."+d.c.Tables[i].Name == qualified {
			return &d.c.Tables[i]
		}
	}
	return nil
}

func referencesPrimaryKey(t dumpTable, columns []string) bool {
	for _, k := range t.Constraints {
		if k.Kind == "primary" {
			return strings.Join(k.Columns, ",") == strings.Join(columns, ",")
		}
	}
	return false
}

var (
	widthType = regexp.MustCompile(`^(character varying|character|bit varying|bit|numeric|timestamp|time)\((\d+)(?:,(\d+))?\)(.*)$`)
	nextval   = regexp.MustCompile(`^nextval\('.*'::regclass\)$`)
	typedText = regexp.MustCompile(`^'((?:[^']|'')*)'::([a-z ]+)(\(\d+(,\d+)?\))?$`)
	number    = regexp.MustCompile(`^\(?(-?[0-9]+(\.[0-9]+)?)\)?(::[a-z ]+)?$`)
)

// column reads one column into a field, or prints why it cannot.
func (d *dbReader) column(t dumpTable, col dumpColumn) *field {
	table := t.Schema + "." + t.Name
	at := "#/entities/" + d.entities[table]
	qualified := table + "." + col.Name
	if !plainName.MatchString(col.Name) {
		d.gap(table, at, "column %s: its name has characters a field name cannot hold; left out", qualified)
		return nil
	}
	f := &field{column: col.Name, name: camel(col.Name), nullable: !col.NotNull}
	if !memberName.MatchString(f.name) {
		d.gap(table, at, "column %s: no field name can be made from it; left out", qualified)
		return nil
	}
	n := d.columnType(table, at, qualified, col.Type, f)
	if n == nil {
		return nil
	}
	if f.nullable {
		t := n.Content[1].Value
		n.Content[1] = flow(value([]string{t, "null"}))
	}
	if col.Comment != nil && *col.Comment != "" {
		set(n, "description", *col.Comment)
	}
	switch {
	case col.Generated != nil:
		set(n, "readOnly", true)
		d.gap(table, at, "column %s: it is generated by the database from %s, which the meta-model cannot hold; written as read-only", qualified, deref(col.Default))
	case col.Identity != nil:
		set(n, "readOnly", true)
	case col.Default != nil && nextval.MatchString(*col.Default):
		set(n, "readOnly", true)
	case col.Default != nil:
		if v := defaultValue(*col.Default, f); v != nil {
			set(n, "default", v)
		} else {
			d.gap(table, at, "column %s: its default %s is not a value the meta-model can hold", qualified, *col.Default)
		}
	}
	f.node = n
	return f
}

func deref(s *string) string {
	if s == nil {
		return "an expression"
	}
	return *s
}

// columnType writes the field's type keywords, following the PostgreSQL
// rows of the type-rendering idiom backwards.
func (d *dbReader) columnType(table, at, qualified, typ string, f *field) *yaml.Node {
	if strings.HasSuffix(typ, "[]") {
		item := &field{}
		in := d.columnType(table, at, qualified, strings.TrimSuffix(typ, "[]"), item)
		if in == nil {
			return nil
		}
		f.kind = "list"
		return mapping("type", "array", "items", flow(in))
	}
	base, width, scale := typ, -1, -1
	if m := widthType.FindStringSubmatch(typ); m != nil {
		base = m[1] + m[4]
		width, _ = strconv.Atoi(m[2])
		if m[3] != "" {
			scale, _ = strconv.Atoi(m[3])
		}
	}
	switch base {
	case "text":
		f.kind = "string"
		return mapping("type", "string")
	case "character varying":
		f.kind = "string"
		if width < 0 {
			return mapping("type", "string")
		}
		return mapping("type", "string", "maxLength", width)
	case "character":
		f.kind = "string"
		if width < 0 {
			width = 1
		}
		d.gap(table, at, "column %s, %s: CHAR pads its value with spaces to its width, which the meta-model cannot say; written as a string of at most %d characters", qualified, typ, width)
		return mapping("type", "string", "maxLength", width)
	case "smallint":
		f.kind, f.bits = "int", 32
		return mapping("type", "integer", "format", "int32", "minimum", -32768, "maximum", 32767)
	case "integer":
		f.kind, f.bits = "int", 32
		return mapping("type", "integer", "format", "int32")
	case "bigint":
		f.kind, f.bits = "int64text", 64
		return mapping("type", "string", "format", "int64")
	case "numeric":
		if width < 0 {
			d.gap(table, at, "column %s, %s: a decimal needs its precision and scale, and the column has none; left out", qualified, typ)
			return nil
		}
		if scale < 0 {
			scale = 0
		}
		f.kind, f.scale = "decimal", scale
		return mapping("type", "string", "format", "decimal", "precision", width, "scale", scale)
	case "double precision":
		f.kind = "double"
		return mapping("type", "number", "format", "double")
	case "real":
		f.kind = "double"
		d.gap(table, at, "column %s, real: a 32-bit float; written as a double, which holds every value of it", qualified)
		return mapping("type", "number", "format", "double")
	case "boolean":
		f.kind = "bool"
		return mapping("type", "boolean")
	case "date":
		f.kind = "date"
		return mapping("type", "string", "format", "date")
	case "timestamp with time zone":
		f.kind = "timestamp"
		return mapping("type", "string", "format", "date-time")
	case "timestamp without time zone":
		f.kind = "timestamp"
		d.gap(table, at, "column %s, %s: a time without its zone, which the meta-model cannot say; written as an instant", qualified, typ)
		return mapping("type", "string", "format", "date-time")
	case "time without time zone":
		f.kind = "time"
		return mapping("type", "string", "format", "time")
	case "interval":
		f.kind = "duration"
		return mapping("type", "string", "format", "duration")
	case "uuid":
		f.kind = "string"
		return mapping("type", "string", "format", "uuid")
	case "json", "jsonb":
		// Any JSON value: the catalogue does not say its shape, which a
		// schema says, so the field is an object and the schema a
		// question (ADR-077).
		f.kind = "json"
		d.ask("should", fmt.Sprintf("Which schema does the column %s hold? It is %s, and the catalogue does not say the shape of the JSON value in it.", qualified, typ), "decision", []string{at + "/properties/" + f.name},
			"A JSON column holds any JSON value; the schema it holds is what the code that writes it or a document says, and an entity's field holding a schema says how it is kept (ADR-063).")
		return mapping("type", "object")
	case "bytea":
		f.kind = "bytes"
		return mapping("type", "string", "format", "byte")
	}
	if values, ok := d.enums[strings.Trim(typ, "\"")]; ok {
		f.kind, f.values = "enum", values
		return mapping("type", "string", "enum", values)
	}
	d.gap(table, at, "column %s, %s: the meta-model has no type for it; left out", qualified, typ)
	return nil
}

// defaultValue is a column default that is a plain value of the field's
// type, or nil.
func defaultValue(def string, f *field) *yaml.Node {
	if m := typedText.FindStringSubmatch(def); m != nil {
		text := strings.ReplaceAll(m[1], "''", "'")
		switch f.kind {
		case "string", "enum", "date", "timestamp", "time", "duration":
			return str(text)
		case "decimal", "int64text":
			if number.MatchString(text) {
				return str(text)
			}
		}
		return nil
	}
	switch def {
	case "true", "false":
		if f.kind == "bool" {
			return value(def == "true")
		}
		return nil
	}
	if m := number.FindStringSubmatch(def); m != nil {
		switch f.kind {
		case "int":
			if m[2] == "" {
				return literal("!!int", m[1])
			}
		case "double":
			return literal("!!float", m[1])
		case "decimal", "int64text":
			return str(m[1])
		}
	}
	return nil
}

// translateCheck writes a check in the expression subset and checks it
// against the table's fields as the validator will; the reason is set when
// it cannot.
func translateCheck(def string, fields map[string]*field) (string, string) {
	n, err := parseCheck(def)
	if err != nil {
		return "", err.Error()
	}
	w := &celWriter{fields: fields}
	s, _, err := w.write(n)
	if err != nil {
		return "", err.Error()
	}
	parsed, errs := expr.Parse(s)
	if len(errs) > 0 {
		return "", "its translation " + s + " does not parse: " + errs[0].Message
	}
	env := expr.Env{}
	for _, f := range fields {
		env[f.name] = exprType(f)
	}
	if _, errs := expr.Check(parsed, env); len(errs) > 0 {
		return "", "its translation " + s + " is refused by SpecArch's expressions: " + errs[0].Message
	}
	return s, ""
}

// exprType is a field's type in the expression subset, as the validator
// reads it from the field's keywords.
func exprType(f *field) expr.Type {
	t := expr.Type{Nullable: f.nullable}
	switch f.kind {
	case "string", "int64text":
		t.Kind = expr.String
		if f.kind == "int64text" {
			t.Kind, t.Bits = expr.Int, 64
		}
	case "int":
		t.Kind, t.Bits = expr.Int, f.bits
	case "decimal":
		t.Kind, t.Scale = expr.Decimal, f.scale
	case "double":
		t.Kind = expr.Double
	case "bool":
		t.Kind = expr.Bool
	case "date":
		t.Kind = expr.Date
	case "timestamp":
		t.Kind = expr.Timestamp
	case "time":
		t.Kind = expr.TimeOfDay
	case "duration":
		t.Kind = expr.Duration
	case "bytes":
		t.Kind = expr.Bytes
	case "enum":
		values := append([]string(nil), f.values...)
		sort.Strings(values)
		t.Kind, t.Values = expr.Enum, values
	default:
		t.Kind = expr.List
	}
	return t
}

// pascal writes a snake_case name in PascalCase: loan_items is LoanItems.
func pascal(s string) string {
	var b strings.Builder
	up := true
	for _, r := range s {
		if r == '_' {
			up = true
			continue
		}
		if up && r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		up = false
		b.WriteRune(r)
	}
	return b.String()
}

// camel writes a snake_case name in camelCase: card_number is cardNumber.
func camel(s string) string {
	p := pascal(s)
	if p == "" {
		return p
	}
	return strings.ToLower(p[:1]) + p[1:]
}
