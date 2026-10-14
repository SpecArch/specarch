package gensql

import "github.com/SpecArch/specarch/internal/ownership"

// The schema the migrations write, for a generator that writes against it,
// such as the queries of specarch-gen-sqlc: the same tables and columns,
// read by the same rules, so a query never names a column the migrations
// do not create.

// Column is one column of a table.
type Column struct {
	Name     string
	Type     string // the dialect's type, as the migration writes it
	Nullable bool
	// Field is the entity's property the column comes from, as the design
	// writes it: the field itself, or the value object whose part it holds.
	// It is empty for a column an idiom adds.
	Field     string
	Key       bool // part of the primary key
	ReadOnly  bool // the field is readOnly: the service sets it, never a caller
	Generated bool // the database draws the value (generatedDefault)
	// GeneratedDefault is the field's generatedDefault, such as
	// identifier, or "".
	GeneratedDefault string
	Role             string
}

// The roles of a column an idiom adds, or of an encrypted field's hash.
const (
	RoleCreated  = "created"  // an audit column set when the record is made
	RoleModified = "modified" // an audit column set at every change
	RoleDeleted  = "deleted"  // the soft-delete flag
	RoleHash     = "hash"     // the hash an encrypted field is looked up by
)

// Table is one entity's table.
type Table struct {
	Entity  string
	Name    string
	Columns []Column
	Keys    Keys
}

// Keys are a table's primary key, foreign keys, unique and check
// constraints and plain indexes, as the migration writes them.
type Keys struct {
	PrimaryKey  []string // its columns, in order
	ForeignKeys []ForeignKey
	Uniques     []Unique
	Checks      []Check
	Indexes     []Index
}

// ForeignKey is the foreign key of a relation.
type ForeignKey struct {
	Relation   string   // the relation's name in the entity
	Columns    []string // the columns of this table
	Table      string   // the table referred to
	References []string // its columns
	OnDelete   string   // restrict, cascade or set null
}

// Unique is a unique constraint with no condition.
type Unique struct {
	Name       string
	Columns    []string
	Constraint string // the entity's constraint it is written from
}

// Check is a check constraint and its condition in the dialect.
type Check struct {
	Name       string
	Expression string
	// Constraint is the entity's constraint it is written from, or "";
	// Field is the field whose type or parts ask for it, or "".
	Constraint string
	Field      string
}

// Index is a plain index the entity's mapping lists.
type Index struct {
	Name    string
	Columns []string
}

// Schema is the tables of the entities no other stakeholder owns, in name
// order, as Generate writes them for the request's dialect, and the version
// of the type-rendering idiom they are rendered through. When they cannot
// be written, it answers the diagnostics that say why.
func Schema(r *Request) ([]Table, string, []Diagnostic) {
	g, failed := prepare(r)
	if failed != nil {
		return nil, "", failed.Diagnostics
	}
	var out []Table
	entities := obj0(g.spec["entities"])
	for _, name := range sortedKeys(entities) {
		if g.owned.Covers(ownership.Entity("entities", name)) {
			continue
		}
		t := g.table(name, obj0(entities[name]))
		out = append(out, Table{Entity: name, Name: t.name, Columns: t.schema, Keys: t.keys})
	}
	if len(g.diags) > 0 {
		return nil, "", g.diags
	}
	return out, g.version, nil
}

// RelRoot is the root file's path as seen from the output folder, as the
// header of a generated file names it.
func RelRoot(root, output string) string { return relRoot(root, output) }
