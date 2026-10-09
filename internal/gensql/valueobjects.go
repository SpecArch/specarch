package gensql

import (
	"maps"
	"strings"
)

// Value objects in an entity's fields (ADR-063). Before any table is
// rendered, each such field is written out as the fields its storage
// gives: in columns, one field per part, named after the path to it
// (address_street, address_geo_lat), with a check that an optional value
// is either wholly absent or has its required parts; in json, one field of
// type object, which the type rows render as the dialect's JSON column. A
// list of schemas is always json. The tables, the snapshot and the differ
// then read the written-out fields, so a part added to a schema is a
// column added by the next migration.

// valueObjectOf names the schema a field holds, and whether it holds a list
// of them.
func valueObjectOf(field map[string]any) (string, bool) {
	if ref, ok := strings.CutPrefix(text(field["$ref"]), "#/schemas/"); ok {
		return ref, false
	}
	if typeIs(field, "array") {
		if ref, ok := strings.CutPrefix(text(obj0(field["items"])["$ref"]), "#/schemas/"); ok {
			return ref, true
		}
	}
	return "", false
}

// typeIs reports whether a field's type is t, alone or beside null.
func typeIs(field map[string]any, t string) bool {
	if text(field["type"]) == t {
		return true
	}
	for _, item := range list(field["type"]) {
		if text(item) == t {
			return true
		}
	}
	return false
}

// holdsValueObject reports whether an entity has a field holding a schema
// the specification has.
func holdsValueObject(e map[string]any, schemas map[string]any) bool {
	for _, field := range obj0(e["properties"]) {
		if name, _ := valueObjectOf(obj0(field)); name != "" && schemas[name] != nil {
			return true
		}
	}
	return false
}

// flattenValueObjects rewrites the specification's entities with every
// field holding a schema written out as its storage says.
func (g *gen) flattenValueObjects() {
	schemas := obj0(g.spec["schemas"])
	entities := obj0(g.spec["entities"])
	if len(schemas) == 0 || len(entities) == 0 {
		return
	}
	out := map[string]any{}
	for _, name := range sortedKeys(entities) {
		out[name] = entities[name]
		if e := obj0(entities[name]); holdsValueObject(e, schemas) {
			out[name] = g.flattenEntity(name, e, schemas)
		}
	}
	g.original = entities
	spec := maps.Clone(g.spec)
	spec["entities"] = out
	g.spec = spec
}

// flat collects the fields an entity's value objects are written out as.
type flat struct {
	g           *gen
	at, table   string
	schemas     map[string]any
	props       map[string]any
	required    []any
	constraints map[string]any
	from        map[string]any // a written-out field, by its column, to the part it comes from
}

func (g *gen) flattenEntity(name string, e map[string]any, schemas map[string]any) map[string]any {
	out, _ := g.flattenEntityFrom(name, e, schemas)
	return out
}

// flattenEntityFrom is flattenEntity, with each part's column and the path
// of the part it comes from.
func (g *gen) flattenEntityFrom(name string, e map[string]any, schemas map[string]any) (map[string]any, map[string]any) {
	props := obj0(e["properties"])
	f := &flat{g: g, at: "/entities/" + name, table: g.tableName(name), schemas: schemas, props: map[string]any{}, constraints: maps.Clone(obj0(e["constraints"])), from: map[string]any{}}
	if f.constraints == nil {
		f.constraints = map[string]any{}
	}
	required := map[string]bool{}
	for _, r := range list(e["required"]) {
		required[text(r)] = true
	}
	columns := map[string]bool{}
	for _, field := range sortedKeys(props) {
		fd := obj0(props[field])
		schema, many := valueObjectOf(fd)
		switch {
		case schema == "" || schemas[schema] == nil:
			f.props[field] = props[field]
		case many || text(fd["storage"]) == "json":
			f.props[field] = map[string]any{"type": "object", "storage": "json"}
		default:
			columns[field] = true
			present := required[field] && !nullType(fd)
			cols, presence := f.parts(snake(field), field, obj0(schemas[schema]), present)
			if !present && len(presence) > 0 {
				f.presence(snake(field), cols, presence)
			}
		}
	}
	for _, r := range list(e["required"]) {
		if !columns[text(r)] {
			f.required = append(f.required, r)
		}
	}
	for _, col := range sortedKeys(f.from) {
		part := text(f.from[col])
		for _, field := range sortedKeys(props) {
			if !columns[field] && snake(field) == col {
				g.problem(f.at+"/properties/"+strings.SplitN(part, ".", 2)[0], "%s is kept in the column %s, which %s has too; rename one, or store %s as json", part, col, field, strings.SplitN(part, ".", 2)[0])
			}
		}
	}
	for _, pk := range list(e["primaryKey"]) {
		if columns[text(pk)] {
			g.problem(f.at+"/primaryKey", "%s holds a value kept in the columns of its parts, and a key over a value object is not written; key the entity on another field", text(pk))
		}
	}
	for _, cn := range sortedKeys(f.constraints) {
		for _, field := range list(obj0(f.constraints[cn])["fields"]) {
			if columns[text(field)] {
				g.problem(f.at+"/constraints/"+cn, "%s holds a value kept in the columns of its parts, and a unique constraint over a value object is not written; name the fields it is unique over", text(field))
			}
		}
	}
	out := maps.Clone(e)
	out["properties"] = f.props
	if _, had := e["required"]; had || len(f.required) > 0 {
		out["required"] = f.required
	}
	if len(f.constraints) > 0 {
		out["constraints"] = f.constraints
	}
	return out, f.from
}

// parts writes out a schema's parts under prefix, the column of the value
// that holds them, and part, its path in the design. present says the value
// is always there, so a required part's column is NOT NULL. It returns the
// columns of every part and the columns that are set whenever the value is.
func (f *flat) parts(prefix, part string, schema map[string]any, present bool) ([]string, []string) {
	required := map[string]bool{}
	for _, r := range list(schema["required"]) {
		required[text(r)] = true
	}
	props := obj0(schema["properties"])
	var all, presence []string
	for _, p := range sortedKeys(props) {
		pd := obj0(props[p])
		col := prefix + "_" + snake(p)
		req := required[p] && !nullType(pd)
		if inner, many := valueObjectOf(pd); inner != "" && !many && f.schemas[inner] != nil {
			cols, sub := f.parts(col, part+"."+p, obj0(f.schemas[inner]), present && req)
			all = append(all, cols...)
			if req {
				presence = append(presence, sub...)
			} else if len(sub) > 0 {
				f.presence(col, cols, sub)
			}
			continue
		}
		field := maps.Clone(pd)
		if _, many := valueObjectOf(pd); many {
			field = map[string]any{"type": "object", "storage": "json"}
		}
		if other := text(f.from[col]); other != "" {
			f.g.problem(f.at+"/properties/"+strings.SplitN(part, ".", 2)[0], "%s.%s is kept in the column %s, which %s has too; rename one, or store one as json", part, p, col, other)
		}
		f.props[col] = field
		f.from[col] = part + "." + p
		if present && req {
			f.required = append(f.required, col)
		}
		all = append(all, col)
		if req {
			presence = append(presence, col)
		}
	}
	return all, presence
}

// presence adds the check that an optional value is wholly absent or has
// every required part: either all its columns are null, or none of the
// columns its required parts have is.
func (f *flat) presence(col string, all, presence []string) {
	if len(all) == 1 {
		return // one column, set whenever the value is: NULL in it is the value's absence
	}
	name := "ck_" + f.table + "_" + col
	if f.constraints[name] != nil {
		f.g.problem(f.at+"/constraints/"+name, "%s is the name of the check that an optional value is wholly absent or has its required parts; rename the constraint", name)
		return
	}
	var absent, there []string
	for _, c := range all {
		absent = append(absent, c+" == null")
	}
	for _, c := range presence {
		there = append(there, c+" != null")
	}
	f.constraints[name] = map[string]any{"kind": "check", "expression": "(" + strings.Join(absent, " && ") + ") || (" + strings.Join(there, " && ") + ")"}
}

// WrittenOut is an entity as generate sql writes it out before it renders a
// table: Fields holds every field, a part kept in columns by its column's
// name; Required the fields that are NOT NULL; Checks the expression of
// each check, the presence checks included; From each part's column and
// the path of the part it comes from (address.street).
type WrittenOut struct {
	Fields   map[string]any
	Required []string
	Checks   map[string]string
	From     map[string]string
}

// WriteOut writes out an entity's fields holding a schema as generate sql
// does, so that what reads columns back reads them by the same names and
// the same presence checks (ADR-077). e and schemas are as the design
// writes them.
func WriteOut(name string, e map[string]any, schemas map[string]any) WrittenOut {
	out, from := e, map[string]any{}
	if holdsValueObject(e, schemas) {
		out, from = (&gen{}).flattenEntityFrom(name, e, schemas)
	}
	w := WrittenOut{Fields: obj0(out["properties"]), Checks: map[string]string{}, From: map[string]string{}}
	for col, part := range from {
		w.From[col] = text(part)
	}
	for _, r := range list(out["required"]) {
		w.Required = append(w.Required, text(r))
	}
	for cn, c := range obj0(out["constraints"]) {
		if text(obj0(c)["kind"]) == "check" {
			w.Checks[cn] = text(obj0(c)["expression"])
		}
	}
	return w
}
