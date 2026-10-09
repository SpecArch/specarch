package validate

import (
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// Value objects, under schemas: data passed around but not stored, with
// no key and no table, named as OpenAPI's components.schemas names them.
// An entity's field may hold one: in columns of the entity's table, one per
// part, or as one JSON value (ADR-063).

// checkValueObjects checks that a schema shares no name with an entity or
// a view, that its required list names its own properties, and that every
// entity field holding one can be stored as its storage says (a relation to
// one is refused where the relation is checked).
func (c *checker) checkValueObjects(d *design) {
	for _, s := range source.Pairs(source.Child(d.root, "schemas")) {
		name := s.Key.Value
		for _, other := range []struct {
			kind  string
			names map[string]*yaml.Node
		}{{"an entity", d.entities}, {"a view", d.views}} {
			if other.names[name] != nil {
				c.add(s.Key, source.Pointer("schemas", name), RuleValueObject, "%s is the name of %s too; a schema, an entity and a view are one namespace, since each becomes a schema of its own in the interface; rename the schema", name, other.kind)
			}
		}
		c.checkFieldList(source.Child(s.Value, "required"), []string{"schemas", name, "required"}, fieldsOf(s.Value), name, "required")
		for _, p := range source.Pairs(source.Child(s.Value, "properties")) {
			c.checkField(p.Value, []string{"schemas", name, "properties", p.Key.Value})
		}
	}
	for _, e := range source.Pairs(source.Child(d.root, "entities")) {
		entity := e.Key.Value
		required := map[string]bool{}
		for _, r := range source.Items(source.Child(e.Value, "required")) {
			required[source.Str(r)] = true
		}
		for _, f := range source.Pairs(source.Child(e.Value, "properties")) {
			path := []string{"entities", entity, "properties", f.Key.Value}
			c.checkStorageOutside(f.Value, path)
			c.checkValueObjectField(d, entity, f, path, !required[f.Key.Value] || nullable(f.Value))
		}
	}
}

// valueObjectOf names the schema a field holds, and whether it holds a list
// of them: '#/schemas/Name' itself, or as the items of an array.
func valueObjectOf(field *yaml.Node) (string, bool) {
	if ref := source.Str(source.Child(field, "$ref")); strings.HasPrefix(ref, "#/schemas/") {
		return strings.TrimPrefix(ref, "#/schemas/"), false
	}
	if typeIs(field, "array") {
		if ref := source.Str(source.Child(source.Child(field, "items"), "$ref")); strings.HasPrefix(ref, "#/schemas/") {
			return strings.TrimPrefix(ref, "#/schemas/"), true
		}
	}
	return "", false
}

// typeIs reports whether a field's type is t, alone or beside null.
func typeIs(field *yaml.Node, t string) bool {
	if source.Str(source.Child(field, "type")) == t {
		return true
	}
	for _, item := range source.Items(source.Child(field, "type")) {
		if source.Str(item) == t {
			return true
		}
	}
	return false
}

// nullable reports whether a field's type list allows null.
func nullable(field *yaml.Node) bool {
	for _, t := range source.Items(source.Child(field, "type")) {
		if source.Str(t) == "null" {
			return true
		}
	}
	return false
}

// checkStorageOutside refuses storage on every field inside n, a field of
// an entity: storage says how an entity's own field is stored, and a value
// inside another one is stored with it.
func (c *checker) checkStorageOutside(n *yaml.Node, path []string) {
	for _, p := range source.Pairs(source.Child(n, "properties")) {
		c.checkField(p.Value, append(append([]string{}, path...), "properties", p.Key.Value))
	}
	if items := source.Child(n, "items"); items != nil {
		c.checkField(items, append(append([]string{}, path...), "items"))
	}
}

// checkField refuses storage on a field that is not an entity's own, and
// on every field inside it.
func (c *checker) checkField(n *yaml.Node, path []string) {
	if s := source.Child(n, "storage"); s != nil {
		c.add(s, source.Pointer(append(path, "storage")...), RuleValueObject, "storage says how an entity's field holding a schema is kept, and this value is stored with what holds it; leave storage out here")
	}
	c.checkStorageOutside(n, path)
}

// checkValueObjectField checks one field of an entity: storage only on a
// field that holds a schema, a list of schemas only as json, and, in
// columns, a schema whose every part has a column and whose absence can be
// told from a value with every part empty.
func (c *checker) checkValueObjectField(d *design, entity string, f source.Pair, path []string, optional bool) {
	field := f.Key.Value
	storage := source.Child(f.Value, "storage")
	name, list := valueObjectOf(f.Value)
	if name == "" {
		if storage != nil {
			c.add(storage, source.Pointer(append(path, "storage")...), RuleValueObject, "%s holds no schema, and storage says how a field holding one is kept; leave storage out", field)
		}
		return
	}
	schema := d.schemas[name]
	if schema == nil {
		return // the reference is refused as ref_type
	}
	kind := source.Str(storage)
	if list {
		if kind == "columns" {
			c.add(storage, source.Pointer(append(path, "storage")...), RuleValueObject, "%s is a list of %s, and a list has no columns in %s's row; store it as json, or make %s an entity related to %s", field, name, entity, name, entity)
		}
		return
	}
	if kind == "json" {
		return
	}
	c.checkColumns(d, f.Key, path, field, name, schema, optional, []string{name})
}

// checkColumns checks a schema stored in columns of its own, at the field
// that holds it: no list of schemas and no cycle, since neither has a fixed
// set of columns, no reference to an entity, which is a relation and not a
// part, and no optional schema without a required property. A
// schema is optional where it is not required of what holds it.
func (c *checker) checkColumns(d *design, at *yaml.Node, path []string, field, name string, schema *yaml.Node, optional bool, seen []string) {
	ptr := source.Pointer(path...)
	if optional && !d.alwaysSet(schema, []string{name}) {
		c.add(at, ptr, RuleValueObject, "%s is optional and %s has no required part that is never null, so a row without it and one where every part of it is empty are the same; make a part of %s required and not nullable, make %s required, or store it as json", field, name, name, field)
	}
	required := map[string]bool{}
	for _, r := range source.Items(source.Child(schema, "required")) {
		required[source.Str(r)] = true
	}
	for _, p := range source.Pairs(source.Child(schema, "properties")) {
		part := field + "." + p.Key.Value
		inner, list := valueObjectOf(p.Value)
		entityRef, isEntity := strings.CutPrefix(source.Str(source.Child(p.Value, "$ref")), "#/entities/")
		if !isEntity {
			entityRef, isEntity = strings.CutPrefix(source.Str(source.Child(source.Child(p.Value, "items"), "$ref")), "#/entities/")
		}
		switch {
		case isEntity:
			c.add(at, ptr, RuleValueObject, "%s refers to %s, a record of its own, and a value kept in columns holds no relation; relate %s to %s, or store %s as json", part, entityRef, path[1], entityRef, strings.SplitN(field, ".", 2)[0])
		case inner == "":
		case list:
			c.add(at, ptr, RuleValueObject, "%s is a list of %s, and a list has no columns of its own; store %s as json", part, inner, strings.SplitN(field, ".", 2)[0])
		case slices.Contains(seen, inner):
			c.add(at, ptr, RuleValueObject, "%s is %s again, inside %s, so its columns never end; store %s as json", part, inner, inner, strings.SplitN(field, ".", 2)[0])
		case d.schemas[inner] != nil:
			c.checkColumns(d, at, path, part, inner, d.schemas[inner], !required[p.Key.Value] || nullable(p.Value), append(append([]string{}, seen...), inner))
		}
	}
}

// alwaysSet reports whether a schema kept in columns has a part whose
// column is set whenever the value is: a required part that is not
// nullable, a scalar or a schema that has one in its turn.
func (d *design) alwaysSet(schema *yaml.Node, seen []string) bool {
	required := map[string]bool{}
	for _, r := range source.Items(source.Child(schema, "required")) {
		required[source.Str(r)] = true
	}
	for _, p := range source.Pairs(source.Child(schema, "properties")) {
		if !required[p.Key.Value] || nullable(p.Value) {
			continue
		}
		inner, list := valueObjectOf(p.Value)
		if inner == "" || list || d.schemas[inner] == nil {
			return true
		}
		if !slices.Contains(seen, inner) && d.alwaysSet(d.schemas[inner], append(append([]string{}, seen...), inner)) {
			return true
		}
	}
	return false
}
