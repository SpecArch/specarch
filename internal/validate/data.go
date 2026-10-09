package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// The keywords about stored data from the dxlib study
// (docs/dxlib-lessons.md, section 2): how sensitive a field is, whether it
// is encrypted at rest, and the audit fields and soft delete of an entity.

// auditFields are the fields the system sets on an audited entity.
var auditFields = []string{"createdAt", "createdBy", "lastModifiedAt", "lastModifiedBy"}

// deletedField is the flag the system sets on an entity with soft deletion.
const deletedField = "deleted"

// checkStored checks an entity's audit fields, its soft delete and the keys
// of its encrypted fields.
func (c *checker) checkStored(name string, e *yaml.Node, fields map[string]*yaml.Node) {
	props := source.Child(e, "properties")
	reserved := func(field, why string) {
		if k := source.Key(props, field); k != nil {
			c.add(k, source.Pointer("entities", name, "properties", field), RuleAudited, "%s %s, so the system sets %s; remove it from the entity's properties", name, why, field)
		}
	}
	if source.Str(source.Child(e, "audited")) == "true" {
		for _, f := range auditFields {
			reserved(f, "is audited")
		}
	}
	if source.Str(source.Child(e, "deletion")) == "soft" {
		reserved(deletedField, "has soft deletion")
	}
	keys := func(list *yaml.Node, ptr []string, what string) {
		for i, item := range source.Items(list) {
			f := fields[item.Value]
			if source.Str(source.Child(f, "atRest")) == "encrypted" && source.Str(source.Child(f, "lookup")) != "hash" {
				c.add(item, source.Pointer(append(ptr, fmt.Sprint(i))...), RuleAtRest, "%s is encrypted at rest, so it cannot be %s unless it is looked up by a hash; add lookup: hash to it", item.Value, what)
			}
		}
	}
	keys(source.Child(e, "primaryKey"), []string{"entities", name, "primaryKey"}, "part of the primary key")
	for _, p := range source.Pairs(source.Child(e, "constraints")) {
		if source.Str(source.Child(p.Value, "kind")) == "unique" {
			keys(source.Child(p.Value, "fields"), []string{"entities", name, "constraints", p.Key.Value, "fields"}, "part of a unique constraint")
		}
	}
}

// checkLookups refuses a lookup on a field that is not encrypted at rest.
func (c *checker) checkLookups(d *design) {
	walk(d.root, nil, func(n *yaml.Node, path []string) {
		if l := source.Child(n, "lookup"); l != nil && source.IsScalar(l) && source.Str(source.Child(n, "atRest")) != "encrypted" {
			c.add(l, source.Pointer(append(path, "lookup")...), RuleAtRest, "lookup says how an encrypted field is found, and this field is not encrypted at rest; add atRest: encrypted, or remove lookup")
		}
	})
}

// checkExposed reports a credential field that is not writeOnly in any
// response of an operation, and, as a warning, a personal field in a
// response of a public operation.
func (c *checker) checkExposed(d *design, o operation) {
	public := source.Str(source.Child(o.node, "permission")) == "public"
	for _, r := range source.Pairs(source.Child(o.node, "responses")) {
		for _, ct := range source.Pairs(source.Child(r.Value, "content")) {
			for _, f := range d.responseFields(source.Child(ct.Value, "schema")) {
				switch source.Str(source.Child(f.node, "sensitivity")) {
				case "credential":
					if source.Str(source.Child(f.node, "writeOnly")) != "true" {
						c.add(r.Key, o.pointer("responses", r.Key.Value), RuleSensitivityExposed, "the %s response of %s can carry %s, which is a credential; mark the field writeOnly, or leave it out of the response", r.Key.Value, o.id, f.name)
					}
				case "personal":
					if public {
						c.warn(r.Key, o.pointer("responses", r.Key.Value), RuleSensitivityExposed, "%s is public, and its %s response carries %s, which is personal; give the operation a permission, or leave the field out", o.id, r.Key.Value, f.name)
					}
				}
			}
		}
	}
}

// namedField is a field a response can carry, named as a reader finds it.
type namedField struct {
	name string // Member.email, or email for an inline field
	node *yaml.Node
}

// responseFields lists every field a schema can carry: through a $ref to an
// entity, a view or a schema of the specification, its items, and its
// properties, each entity, view and schema once.
func (d *design) responseFields(schema *yaml.Node) []namedField {
	var out []namedField
	seen := map[string]bool{}
	var visit func(s *yaml.Node, owner string)
	visit = func(s *yaml.Node, owner string) {
		s = source.Deref(s)
		if s == nil {
			return
		}
		if ref := source.Str(source.Child(s, "$ref")); strings.HasPrefix(ref, "#/entities/") {
			name := strings.TrimPrefix(ref, "#/entities/")
			if seen[name] || d.entities[name] == nil {
				return
			}
			seen[name] = true
			visit(d.entities[name], name)
			return
		}
		if ref := source.Str(source.Child(s, "$ref")); strings.HasPrefix(ref, "#/schemas/") {
			name := strings.TrimPrefix(ref, "#/schemas/")
			if seen["$"+name] || d.schemas[name] == nil {
				return
			}
			seen["$"+name] = true
			visit(d.schemas[name], name)
			return
		}
		if ref := source.Str(source.Child(s, "$ref")); strings.HasPrefix(ref, "#/views/") {
			// A view carries its entity's fields and the fields its paths read,
			name := strings.TrimPrefix(ref, "#/views/")
			v := d.views[name]
			if seen["#"+name] || v == nil {
				return
			}
			seen["#"+name] = true
			fields := d.viewFields(v)
			for _, k := range sortedKeys(fields) {
				out = append(out, namedField{name: name + "." + k, node: fields[k]})
			}
			// and the records of each relation it carries as rows.
			for _, target := range d.rowsTargets(v) {
				if !seen[target] && d.entities[target] != nil {
					seen[target] = true
					visit(d.entities[target], target)
				}
			}
			return
		}
		visit(source.Child(s, "items"), owner)
		for _, p := range source.Pairs(source.Child(s, "properties")) {
			label := p.Key.Value
			if owner != "" {
				label = owner + "." + label
			}
			out = append(out, namedField{name: label, node: p.Value})
			visit(p.Value, "")
		}
	}
	visit(schema, "")
	return out
}
