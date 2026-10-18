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

	"github.com/SpecArch/specarch/idioms"
	"github.com/SpecArch/specarch/internal/expr"
)

// The catalogue dump tools/catalogue/catalogue.sql writes.
type catalogue struct {
	Catalogue  string         `json:"catalogue"`
	Path       string         `json:"path"`
	Commit     string         `json:"commit"`
	Repository string         `json:"repository"`
	Tables     []dumpTable    `json:"tables"`
	Views      []dumpView     `json:"views"`
	Enums      []dumpEnumType `json:"enums"`
}

type dumpTable struct {
	Schema  string  `json:"schema"`
	Name    string  `json:"name"`
	Comment *string `json:"comment"`
	// PartitionBy is the partition key of a partitioned table, as
	// pg_get_partkeydef prints it; PartitionOf the schema.name of the table
	// a partition is a partition of, and PartitionBound its bound. A dump
	// of an earlier catalogue.sql has none of the three, and reads as a
	// dump with no partition (ADR-128).
	PartitionBy    *string          `json:"partitionBy"`
	PartitionOf    *string          `json:"partitionOf"`
	PartitionBound *string          `json:"partitionBound"`
	Columns        []dumpColumn     `json:"columns"`
	Constraints    []dumpConstraint `json:"constraints"`
	Indexes        []dumpIndex      `json:"indexes"`
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
	// Inherited is true for a partition's constraint taken from its table.
	Inherited bool `json:"inherited"`
}

type dumpIndex struct {
	Name            string `json:"name"`
	Unique          bool   `json:"unique"`
	Primary         bool   `json:"primary"`
	BacksConstraint bool   `json:"backsConstraint"`
	// Inherited is true for a partition's index taken from its table.
	Inherited  bool   `json:"inherited"`
	Definition string `json:"definition"`
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

// Database reads a catalogue dump into entities, one per table. With
// fragment, the mappings it knows, each table and its plain indexes, go
// into Result.Implementation, the implementation fragment
// --implementation-out names (ADR-121).
func Database(dumpPath, out, key string, fragment bool) (*Result, error) {
	c, r, dumpName, err := openCatalogue(dumpPath)
	if err != nil {
		return nil, err
	}
	res := &Result{Tree: newTree()}
	res.say("commit %s: the last change to %s, as the dump %s names it", r.Commit, c.Path, dumpName)
	d := &dbReader{c: c, key: key, clause: c.Path, res: res, enums: map[string][]string{}, entities: map[string]string{}, generated: generatedDefaults()}
	if fragment {
		d.mappings = &yaml.Node{Kind: yaml.MappingNode}
	}
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
	if fragment {
		res.Implementation = mapping("mappings", d.mappings)
		res.ImplementationNote = fmt.Sprintf("The mappings specarch extract database read from the catalogue dump %s, made at commit %s: the table of each entity and its plain indexes. They go under mappings in an implementation file of the specification; each cites the source %s.", dumpName, r.Commit, key)
		res.say("wrote %s into the implementation fragment, with %s", plural(len(d.mappings.Content)/2, "mapping"), plural(d.indexes, "index"))
	}
	return res, nil
}

// openCatalogue reads a catalogue dump and checks it against the commit
// it names.
func openCatalogue(dumpPath string) (*catalogue, *Read, string, error) {
	data, err := os.ReadFile(dumpPath)
	if err != nil {
		return nil, nil, "", err
	}
	var c catalogue
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, nil, "", refuse("%s is not a catalogue dump: %v", dumpPath, err)
	}
	if c.Catalogue != "postgresql" {
		return nil, nil, "", refuse("%s is not a catalogue dump of tools/catalogue/catalogue.sql: its catalogue is %q, not postgresql", dumpPath, c.Catalogue)
	}
	r, dumpName, err := openDump(dumpPath, c.Repository, c.Path, c.Commit, "tools/catalogue/dump-catalogue.sh")
	if err != nil {
		return nil, nil, "", err
	}
	return &c, r, dumpName, nil
}

// generatedDefaults are the expressions the shipped type-rendering idiom
// names for PostgreSQL's generated defaults, each with its name.
func generatedDefaults() map[string]string {
	out := map[string]string{}
	data, err := idioms.Shipped.ReadFile("type-rendering/type-rendering.specarch-idiom.yaml")
	if err != nil {
		return out
	}
	var doc struct {
		Parts map[string]struct {
			Stack map[string]struct {
				Names map[string]string `yaml:"names"`
			} `yaml:"stack"`
		} `yaml:"parts"`
	}
	if yaml.Unmarshal(data, &doc) != nil {
		return out
	}
	for _, name := range []string{"identifier", "currentTime"} {
		if e := doc.Parts["defaults"].Stack["postgresql"].Names[name]; e != "" {
			out[e] = name
		}
	}
	return out
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
	generated   map[string]string // the generatedDefault of each expression the idiom names
	mappings    *yaml.Node        // the implementation fragment's mappings, or nil
	indexes     int               // the indexes written into it
	// roots is the table each partition is read as, by schema.name: the
	// one partitionOf leads to that is no partition (ADR-128). partitions
	// are the partitions read as each table, in the dump's order.
	roots      map[string]string
	partitions map[string][]dumpTable
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
	byName := map[string]dumpTable{}
	for _, t := range tables {
		byName[t.Schema+"."+t.Name] = t
	}
	// Name every table first, so that a foreign key finds its target. A
	// partition is read as its table and gets no entity (ADR-128).
	byEntity := map[string]string{}
	d.roots = map[string]string{}
	d.partitions = map[string][]dumpTable{}
	for _, t := range tables {
		qualified := t.Schema + "." + t.Name
		if t.PartitionOf != nil {
			if root := rootOf(t, byName); root != qualified {
				d.roots[qualified] = root
				d.partitions[root] = append(d.partitions[root], t)
				continue
			}
		}
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
		if t.PartitionOf != nil {
			d.gap(qualified, "#/entities/"+name, "table %s: it is a partition of %s, which the dump does not list; read as a table of its own", qualified, *t.PartitionOf)
		}
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
	tablesCounted := plural(len(tables), "table")
	perTable := "one per table whose name an entity can hold"
	switch partitions := len(d.roots); {
	case partitions == 1:
		tablesCounted += ", one of them a partition"
		perTable = "one per table that is not a partition and whose name an entity can hold"
	case partitions > 1:
		tablesCounted += fmt.Sprintf(", %d of them partitions", partitions)
		perTable = "one per table that is not a partition and whose name an entity can hold"
	}
	d.res.say("counted %s, %s, %s (%s, %s, %s), %s besides those of keys, %s and %s: every entry of the dump's lists, which hold every schema but pg_catalog, information_schema and pg_toast",
		tablesCounted, plural(columns, "column"), plural(constraints, "constraint"),
		plural(foreign, "foreign key"), plural(uniques, "unique key"), plural(checks, "check"),
		plural(indexes, "index"), plural(len(d.c.Views), "view"), plural(len(d.c.Enums), "enum type"))
	d.res.say("wrote %s and %s: %s, one per thing the catalogue does not say, and one per thing the meta-model cannot hold", plural(len(d.entities), "entity"), plural(len(d.questions.Content)/2+couldCount(oneQuestions(d.questions), d.notHeld), "question"), perTable)
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
			ref, _ := d.referenced(k)
			targets[ref]++
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
	var indexes []*yaml.Node
	var indexSays []string
	for _, ix := range t.Indexes {
		if ix.BacksConstraint {
			continue
		}
		cols, plain := plainIndexColumns(ix.Definition, qualified)
		names, held := fieldNames(cols, fields)
		switch {
		case ix.Unique:
			d.gap(qualified, at, "index %s of %s, %s: a unique index without a constraint; the meta-model says uniqueness only as a unique constraint", ix.Name, qualified, ix.Definition)
		case !plain:
			d.gap(qualified, at, "index %s of %s, %s: an index under a mapping holds plain fields, and this one has an expression, a condition, an order, included columns or a method other than btree", ix.Name, qualified, ix.Definition)
		case !held:
			d.gap(qualified, at, "index %s of %s, %s: a column of it is left out", ix.Name, qualified, ix.Definition)
		case d.mappings == nil:
			d.gap(qualified, at, "index %s of %s, %s: an index is physical design, which an implementation file holds under the entity's mapping; run specarch extract database with --implementation-out to write it there", ix.Name, qualified, ix.Definition)
		default:
			n := flow(mapping("fields", names))
			if t.Schema != "public" || ix.Name != "ix_"+t.Name+"_"+strings.Join(cols, "_") {
				set(n, "name", ix.Name)
			}
			indexes = append(indexes, n)
			indexSays = append(indexSays, ix.Definition)
		}
	}
	if d.mappings != nil {
		target := "table " + t.Name
		if t.Schema != "public" {
			target = "table " + qualified
		}
		m := mapping("target", target)
		says := fmt.Sprintf("Table %s.", qualified)
		if len(indexes) > 0 {
			set(m, "indexes", indexes)
			says = fmt.Sprintf("Table %s: %s.", qualified, strings.Join(indexSays, "; "))
			d.indexes += len(indexes)
		}
		set(m, "origin", "stated")
		set(m, "cites", []*yaml.Node{citation(d.key, d.clause+" "+qualified, says)})
		set(d.mappings, "#/entities/"+name, m)
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
	d.partitioned(t, at)
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
	set(entity, "cites", []*yaml.Node{citation(d.key, d.clause+" "+qualified, tableSays(t, d.partitions[qualified]))})
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

// A partitioned table's index is printed with ON ONLY, since the statement
// makes it on the table alone and each partition has its own attached to
// it; the index is the table's all the same (ADR-128).
var indexDefinition = regexp.MustCompile(`^CREATE INDEX ([a-z_][a-z0-9_$]*) ON (?:ONLY )?([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*) USING btree \(([^()]*)\)$`)

// plainIndexColumns are the columns of an index the catalogue prints as a
// btree of plain columns of the table, with no expression, condition,
// order or included column; ok is false for any other.
func plainIndexColumns(def, table string) ([]string, bool) {
	m := indexDefinition.FindStringSubmatch(def)
	if m == nil || m[2] != table {
		return nil, false
	}
	cols := strings.Split(m[3], ", ")
	for _, c := range cols {
		if !plainName.MatchString(c) {
			return nil, false
		}
	}
	return cols, true
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

// tableSays is what the catalogue says of a table: its columns, its
// constraints and, when it is partitioned, its partitions.
func tableSays(t dumpTable, partitions []dumpTable) string {
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
	if len(partitions) > 0 {
		says += fmt.Sprintf(" Partitioned by %s: %s.", deref(t.PartitionBy), partitionList(t.Schema+"."+t.Name, partitions))
	}
	return says
}

// rootOf is the table a partition is read as: the one partitionOf leads
// to that is no partition. A partition whose table the dump does not list,
// or whose chain of tables comes back on itself, is read as a table of its
// own, and so is its own root.
func rootOf(t dumpTable, byName map[string]dumpTable) string {
	seen := map[string]bool{t.Schema + "." + t.Name: true}
	for t.PartitionOf != nil {
		parent, ok := byName[*t.PartitionOf]
		if !ok || seen[*t.PartitionOf] {
			return t.Schema + "." + t.Name
		}
		seen[*t.PartitionOf] = true
		t = parent
	}
	return t.Schema + "." + t.Name
}

// partitioned asks about a table's partitions (ADR-128): one could
// question listing them, since partitioning is physical design the
// meta-model cannot hold, and one for each thing a partition has of its
// own, a constraint or an index not taken from the table, or a column
// setting that differs from the table's; what a partition takes from its
// table is read once, at the table.
func (d *dbReader) partitioned(t dumpTable, at string) {
	qualified := t.Schema + "." + t.Name
	partitions := d.partitions[qualified]
	if len(partitions) == 0 {
		return
	}
	d.gap(qualified, at, "table %s, partitioned by %s: %s; partitioning is physical design, which the meta-model cannot hold, so each partition is read as the table", qualified, deref(t.PartitionBy), partitionList(qualified, partitions))
	own := map[string]dumpColumn{}
	for _, c := range t.Columns {
		own[c.Name] = c
	}
	for _, p := range partitions {
		pq := p.Schema + "." + p.Name
		for _, c := range p.Columns {
			r, ok := own[c.Name]
			if !ok {
				d.gap(pq, at, "column %s.%s: the partition has it and the table %s does not; left out", pq, c.Name, qualified)
				continue
			}
			if diff := columnDifferences(c, r, qualified); diff != "" {
				d.gap(pq, at, "column %s.%s: %s; a partition's own column setting holds for its rows alone, which the meta-model cannot say; read as the table's column", pq, c.Name, diff)
			}
		}
		keys := append([]dumpConstraint(nil), p.Constraints...)
		sort.SliceStable(keys, func(i, j int) bool { return keys[i].Name < keys[j].Name })
		for _, k := range keys {
			if k.Inherited {
				continue
			}
			d.gap(pq, at, "%s %s of partition %s, %s: a partition's own constraint holds for its rows alone, which the meta-model cannot say; left out", constraintWords(k.Kind), k.Name, pq, k.Definition)
		}
		for _, ix := range p.Indexes {
			if ix.BacksConstraint || ix.Inherited {
				continue
			}
			d.gap(pq, at, "index %s of partition %s, %s: a partition's own index is physical design of that partition alone, which neither the meta-model nor a mapping holds; left out", ix.Name, pq, ix.Definition)
		}
	}
}

// partitionList names each partition of a table with its bound, the
// partition it is a partition of when that is not the table, and its own
// partition key when it is partitioned itself.
func partitionList(table string, partitions []dumpTable) string {
	var out []string
	for _, p := range partitions {
		s := p.Schema + "." + p.Name
		if p.PartitionOf != nil && *p.PartitionOf != table {
			s += " of " + *p.PartitionOf
		}
		if p.PartitionBound != nil {
			s += " " + *p.PartitionBound
		}
		if p.PartitionBy != nil {
			s += ", itself partitioned by " + *p.PartitionBy
		}
		out = append(out, s)
	}
	return strings.Join(out, "; ")
}

// columnDifferences says how a partition's column differs from the
// table's, or nothing when it does not.
func columnDifferences(c, table dumpColumn, qualified string) string {
	var out []string
	if c.Type != table.Type {
		out = append(out, fmt.Sprintf("type %s where the table %s has %s", c.Type, qualified, table.Type))
	}
	switch {
	case c.NotNull && !table.NotNull:
		out = append(out, fmt.Sprintf("not null where the table %s has it nullable", qualified))
	case !c.NotNull && table.NotNull:
		out = append(out, fmt.Sprintf("nullable where the table %s has it not null", qualified))
	}
	for _, s := range []struct {
		what   string
		c, own *string
	}{{"default", c.Default, table.Default}, {"identity", c.Identity, table.Identity}, {"generated expression", c.Generated, table.Generated}} {
		switch {
		case s.c == nil && s.own == nil, s.c != nil && s.own != nil && *s.c == *s.own:
		case s.c == nil:
			out = append(out, fmt.Sprintf("no %s where the table %s has %s", s.what, qualified, *s.own))
		case s.own == nil:
			out = append(out, fmt.Sprintf("%s %s where the table %s has none", s.what, *s.c, qualified))
		default:
			out = append(out, fmt.Sprintf("%s %s where the table %s has %s", s.what, *s.c, qualified, *s.own))
		}
	}
	return strings.Join(out, ", ")
}

// constraintWords is a constraint's kind as the lines say it.
func constraintWords(kind string) string {
	switch kind {
	case "primary":
		return "primary key"
	case "foreign":
		return "foreign key"
	case "unique":
		return "unique key"
	case "check":
		return "check"
	case "exclusion":
		return "exclusion constraint"
	}
	return kind + " constraint"
}

// referenced is the table a foreign key refers to, read as the table a
// partition is a partition of when it refers to a partition (ADR-128), and
// the partition it named then.
func (d *dbReader) referenced(k dumpConstraint) (ref, partition string) {
	ref = *k.ReferencesSchema + "." + *k.ReferencesTable
	if root, ok := d.roots[ref]; ok {
		return root, ref
	}
	return ref, ""
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
	ref, partition := d.referenced(k)
	refName := *k.ReferencesTable
	if partition != "" {
		d.gap(qualified, at, "foreign key %s of %s, %s: it references %s, a partition, whose rows are those of %s; read as a relation to that table", k.Name, qualified, k.Definition, partition, ref)
		refName = ref[strings.Index(ref, ".")+1:]
	}
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
	name := camel(refName)
	if targets[ref] > 1 {
		name = camel(refName) + "Via" + pascal(k.Columns[0])
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
	case col.Default != nil && d.generated[*col.Default] != "" && generatedFits(d.generated[*col.Default], n):
		set(n, "generatedDefault", d.generated[*col.Default])
	case col.Default != nil:
		switch v := defaultValue(*col.Default, f); {
		case v != nil:
			set(n, "default", v)
		case d.generated[*col.Default] != "":
			name := d.generated[*col.Default]
			d.gap(table, at, "column %s: its default %s is the generated default %s, which only a field of format %s takes", qualified, *col.Default, name, map[string]string{"identifier": "uuid", "currentTime": "date-time"}[name])
		default:
			d.gap(table, at, "column %s: its default %s is neither a value the meta-model can hold nor an expression the type-rendering idiom names for a generated default (%s)", qualified, *col.Default, d.generatedList())
		}
	}
	f.node = n
	return f
}

// generatedList names each expression the idiom names, with its generated
// default.
func (d *dbReader) generatedList() string {
	var out []string
	for e, name := range d.generated {
		out = append(out, e+" for "+name)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// generatedFits says whether a field can take the generated default: an
// identifier is a uuid, the current time a date-time.
func generatedFits(name string, n *yaml.Node) bool {
	format := ""
	if f := child(n, "format"); f != nil {
		format = f.Value
	}
	return (name == "identifier" && format == "uuid") || (name == "currentTime" && format == "date-time")
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
