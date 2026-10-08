package validate

import (
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// Value objects, under schemas: data passed around but not stored, with
// no key and no table, named as OpenAPI's components.schemas names them.

// checkValueObjects checks that a schema shares no name with an entity or
// a view, that its required list names its own properties, and that no
// entity holds one in a field (a relation to one is refused where the
// relation is checked).
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
	}
	for _, e := range source.Pairs(source.Child(d.root, "entities")) {
		walk(source.Child(e.Value, "properties"), []string{"entities", e.Key.Value, "properties"}, func(n *yaml.Node, path []string) {
			ref := source.Child(n, "$ref")
			if ref == nil || !strings.HasPrefix(ref.Value, "#/schemas/") {
				return
			}
			name := strings.TrimPrefix(ref.Value, "#/schemas/")
			c.add(ref, source.Pointer(append(path, "$ref")...), RuleValueObject, "%s is a schema, which is passed around and never stored, and %s is stored; hold the fields %s needs in %s, or make %s an entity", name, e.Key.Value, e.Key.Value, e.Key.Value, name)
		})
	}
}
