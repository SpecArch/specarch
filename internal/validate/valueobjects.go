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
	rv := newResolver(d, c.given)
	for _, e := range source.Pairs(source.Child(d.root, "entities")) {
		entity := e.Key.Value
		required := map[string]bool{}
		for _, r := range source.Items(source.Child(e.Value, "required")) {
			required[source.Str(r)] = true
		}
		for _, f := range source.Pairs(source.Child(e.Value, "properties")) {
			path := []string{"entities", entity, "properties", f.Key.Value}
			c.checkStorageOutside(f.Value, path)
			c.checkValueObjectField(d, rv, entity, f, path, !required[f.Key.Value] || nullable(f.Value))
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

// foreignValueObjectOf names the schema of another specification a field
// holds, as <dependency>#/schemas/<Name>, itself or as the items of an
// array, and whether it holds a list of them.
func foreignValueObjectOf(field *yaml.Node) (string, bool) {
	isSchema := func(ref string) bool {
		_, ptr, ok := crossRef(ref)
		return ok && strings.HasPrefix(ptr, "/schemas/")
	}
	if ref := source.Str(source.Child(field, "$ref")); isSchema(ref) {
		return ref, false
	}
	if typeIs(field, "array") {
		if ref := source.Str(source.Child(source.Child(field, "items"), "$ref")); isSchema(ref) {
			return ref, true
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
	if values := source.Child(n, "additionalProperties"); values != nil && values.Kind == yaml.MappingNode {
		c.checkField(values, append(append([]string{}, path...), "additionalProperties"))
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
func (c *checker) checkValueObjectField(d *design, rv *resolver, entity string, f source.Pair, path []string, optional bool) {
	field := f.Key.Value
	storage := source.Child(f.Value, "storage")
	name, list := valueObjectOf(f.Value)
	ref := "#/schemas/" + name
	if name == "" {
		other, otherList := foreignValueObjectOf(f.Value)
		if other == "" {
			if storage != nil {
				c.add(storage, source.Pointer(append(path, "storage")...), RuleValueObject, "%s holds no schema, and storage says how a field holding one is kept; leave storage out", field)
			}
			return
		}
		// A schema of another specification is written as this one's own
		// (ADR-184): a list is json only, and one value in columns is
		// checked as one of this specification's schemas, in the module
		// that declares it.
		if otherList {
			if source.Str(storage) == "columns" {
				c.add(storage, source.Pointer(append(path, "storage")...), RuleValueObject, "%s is a list of %s, of another specification, and a list has no columns in %s's row; store it as json", field, other, entity)
			}
			return
		}
		ref, list = other, false
	}
	if source.Str(storage) == "json" && !list {
		return
	}
	cc := &columnsCheck{c: c, rv: rv, own: SpecName(d.root), at: f.Key, path: path}
	held, ok := cc.resolve(d, ref)
	if !ok {
		cc.report(field)
		return // the reference is refused as ref_type or reference
	}
	if list {
		if source.Str(storage) == "columns" {
			c.add(storage, source.Pointer(append(path, "storage")...), RuleValueObject, "%s is a list of %s, and a list has no columns in %s's row; store it as json, or make %s an entity related to %s", field, name, entity, name, entity)
		}
		return
	}
	cc.columns(field, held, optional, []string{held.key()})
	cc.report(field)
}

// A heldSchema is a schema a value kept in columns holds, with the design
// and the module that declare it.
type heldSchema struct {
	d      *design
	module string
	name   string
	node   *yaml.Node
}

func (h heldSchema) key() string { return h.module + "#/schemas/" + h.name }

// columnsCheck checks one entity field kept in columns, reading each
// reference in the module that declares it, and keeps the modules the run
// does not give, whose parts it cannot check.
type columnsCheck struct {
	c         *checker
	rv        *resolver
	own       string // the name of the specification checked
	at        *yaml.Node
	path      []string
	unchecked []string // the schemas not checked, as label()
	notGiven  []string // the modules that declare them
}

// label names a schema or an entity of a module: by its name in the
// specification checked, and with its module's name in another.
func (cc *columnsCheck) label(module, name string) string {
	if module == cc.own {
		return name
	}
	return name + " of " + module
}

// resolve is the schema a reference made in design d names, '#/schemas/Name'
// or '<dependency>#/schemas/Name'; false when it names none, or a module
// the run does not give, which is kept for report.
func (cc *columnsCheck) resolve(d *design, ref string) (heldSchema, bool) {
	module := SpecName(d.root)
	if name, ok := strings.CutPrefix(ref, "#/schemas/"); ok {
		if d.schemas[name] == nil {
			return heldSchema{}, false
		}
		return heldSchema{d: d, module: module, name: name, node: d.schemas[name]}, true
	}
	dep, ptr, ok := crossRef(ref)
	name, isSchema := strings.CutPrefix(ptr, "/schemas/")
	spec := d.specificationOf(dep)
	if !ok || !isSchema || spec == "" {
		return heldSchema{}, false // a reference error says why
	}
	o, problem := cc.rv.otherOf(spec)
	switch {
	case problem != "":
		return heldSchema{}, false
	case o == nil:
		if len(cc.rv.given[spec]) == 0 {
			if l := cc.label(spec, name); !contains(cc.unchecked, l) {
				cc.unchecked = append(cc.unchecked, l)
			}
			if !contains(cc.notGiven, spec) {
				cc.notGiven = append(cc.notGiven, spec)
			}
		}
		return heldSchema{}, false // given and not read: its own errors say why
	case o.design.schemas[name] == nil:
		return heldSchema{}, false
	}
	if spec == cc.own {
		return heldSchema{d: cc.rv.d, module: cc.own, name: name, node: cc.rv.d.schemas[name]}, true
	}
	return heldSchema{d: o.design, module: spec, name: name, node: o.design.schemas[name]}, true
}

// partSchema is the schema a part of a value holds, read in the design
// that declares the value, and whether it holds a list of them; ok is
// false when the part holds no schema it can read.
func (cc *columnsCheck) partSchema(d *design, part *yaml.Node) (held heldSchema, list, ok bool) {
	ref := source.Str(source.Child(part, "$ref"))
	if ref == "" && typeIs(part, "array") {
		ref, list = source.Str(source.Child(source.Child(part, "items"), "$ref")), true
	}
	if !strings.HasPrefix(ref, "#/schemas/") {
		if _, ptr, cross := crossRef(ref); !cross || !strings.HasPrefix(ptr, "/schemas/") {
			return heldSchema{}, false, false
		}
	}
	held, ok = cc.resolve(d, ref)
	return held, list, ok
}

// crossList names the schema of another module a part of a value holds as
// a list, '<dependency>#/schemas/Name' as its items, read in the design that
// declares the value. A list has no columns whatever the schema holds, so
// the name comes from the reference and the module need not be given.
func (cc *columnsCheck) crossList(d *design, part *yaml.Node) (string, bool) {
	if !typeIs(part, "array") || source.Child(part, "$ref") != nil {
		return "", false
	}
	dep, ptr, ok := crossRef(source.Str(source.Child(source.Child(part, "items"), "$ref")))
	name, isSchema := strings.CutPrefix(ptr, "/schemas/")
	if !ok || !isSchema {
		return "", false
	}
	if spec := d.specificationOf(dep); spec != "" {
		return cc.label(spec, name), true
	}
	return name + " of " + dep, true
}

// columns checks a schema stored in columns of its own, at the field that
// holds it: no list of schemas and no cycle, since neither has a fixed set
// of columns, no reference to an entity, which is a relation and not a
// part, and no optional schema without a required property. A schema is
// optional where it is not required of what holds it.
func (cc *columnsCheck) columns(field string, held heldSchema, optional bool, seen []string) {
	c, ptr, top := cc.c, source.Pointer(cc.path...), strings.SplitN(field, ".", 2)[0]
	name := cc.label(held.module, held.name)
	if optional && !cc.alwaysSet(held, []string{held.key()}) {
		c.add(cc.at, ptr, RuleValueObject, "%s is optional and %s has no required part that is never null, so a row without it and one where every part of it is empty are the same; make a part of %s required and not nullable, make %s required, or store it as json", field, name, name, field)
	}
	required := map[string]bool{}
	for _, r := range source.Items(source.Child(held.node, "required")) {
		required[source.Str(r)] = true
	}
	for _, p := range source.Pairs(source.Child(held.node, "properties")) {
		part := field + "." + p.Key.Value
		entityRef, isEntity := strings.CutPrefix(source.Str(source.Child(p.Value, "$ref")), "#/entities/")
		if !isEntity {
			entityRef, isEntity = strings.CutPrefix(source.Str(source.Child(source.Child(p.Value, "items"), "$ref")), "#/entities/")
		}
		if isEntity {
			c.add(cc.at, ptr, RuleValueObject, "%s refers to %s, a record of its own, and a value kept in columns holds no relation; relate %s to %s, or store %s as json", part, cc.label(held.module, entityRef), cc.path[1], cc.label(held.module, entityRef), top)
			continue
		}
		if other, isList := cc.crossList(held.d, p.Value); isList {
			c.add(cc.at, ptr, RuleValueObject, "%s is a list of %s, and a list has no columns of its own; store %s as json", part, other, top)
			continue
		}
		inner, list, ok := cc.partSchema(held.d, p.Value)
		innerName := cc.label(inner.module, inner.name)
		switch {
		case !ok:
		case list:
			c.add(cc.at, ptr, RuleValueObject, "%s is a list of %s, and a list has no columns of its own; store %s as json", part, innerName, top)
		case slices.Contains(seen, inner.key()):
			c.add(cc.at, ptr, RuleValueObject, "%s is %s again, inside %s, so its columns never end; store %s as json", part, innerName, innerName, top)
		default:
			cc.columns(part, inner, !required[p.Key.Value] || nullable(p.Value), append(append([]string{}, seen...), inner.key()))
		}
	}
}

// alwaysSet reports whether a schema kept in columns has a part whose
// column is set whenever the value is: a required part that is not
// nullable, a scalar or a schema that has one in its turn. A part holding
// a schema of a module the run does not give counts as set, and is kept
// for report.
func (cc *columnsCheck) alwaysSet(held heldSchema, seen []string) bool {
	required := map[string]bool{}
	for _, r := range source.Items(source.Child(held.node, "required")) {
		required[source.Str(r)] = true
	}
	for _, p := range source.Pairs(source.Child(held.node, "properties")) {
		if !required[p.Key.Value] || nullable(p.Value) {
			continue
		}
		if _, isList := cc.crossList(held.d, p.Value); isList {
			return true
		}
		inner, list, ok := cc.partSchema(held.d, p.Value)
		if !ok || list {
			return true
		}
		if !slices.Contains(seen, inner.key()) && cc.alwaysSet(inner, append(append([]string{}, seen...), inner.key())) {
			return true
		}
	}
	return false
}

// report warns once at the field when the value reaches a schema of a
// module the run does not give, whose parts are therefore not checked.
func (cc *columnsCheck) report(field string) {
	if len(cc.notGiven) == 0 {
		return
	}
	modules := joinAnd(cc.notGiven)
	cc.c.warn(cc.at, source.Pointer(cc.path...), RuleValueObjectUnchecked, "%s is kept in columns and holds %s, and the run does not give %s, so the rules on its parts are not checked: an optional value needs a required part that is never null, and no list, cycle or entity may be inside it; give %s in the same run or through the estate, or store %s as json", field, joinAnd(cc.unchecked), modules, modules, field)
}
