package validate

import (
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// The read model of the dxlib study (docs/dxlib-lessons.md, section 2):
// views, an entity's row with fields read through its relations, counts
// of its related records and the rows of a one-to-many relation added.

// toOne and toMany are the relation kinds a path follows and a count
// counts.
var (
	toOne  = map[string]bool{"many-to-one": true, "one-to-one": true}
	toMany = map[string]bool{"one-to-many": true, "many-to-many": true}
)

// viewPath follows a path from an entity: the field it ends in, or "" and
// the reason it does not resolve.
func (d *design) viewPath(entity, path string) (*yaml.Node, string) {
	hops := strings.Split(path, ".")
	cur := entity
	for _, hop := range hops[:len(hops)-1] {
		rels := topMap(d.entities[cur], "relations")
		r := rels[hop]
		if r == nil {
			return nil, noRelation(cur, hop, rels)
		}
		if kind := source.Str(source.Child(r, "kind")); !toOne[kind] {
			return nil, cur + "." + hop + " is " + kind + ", which leads to many records; a path follows only many-to-one and one-to-one relations, so the view keeps one row per record; count it instead"
		}
		cur = source.Str(source.Child(r, "target"))
		if d.entities[cur] == nil {
			return nil, "" // the relation's own check reports the target
		}
	}
	last := hops[len(hops)-1]
	fields := fieldsOf(d.entities[cur])
	if f := fields[last]; f != nil {
		return f, ""
	}
	return nil, cur + " has no field " + last + suggest(last, fields)
}

// rowsTargets are the entities whose records a view carries as rows.
func (d *design) rowsTargets(view *yaml.Node) []string {
	from := source.Str(source.Child(view, "from"))
	var out []string
	for _, p := range source.Pairs(source.Child(view, "properties")) {
		if rel := source.Str(source.Child(p.Value, "rows")); rel != "" {
			if t := source.Str(source.Child(topMap(d.entities[from], "relations")[rel], "target")); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
}

// noRelation says an entity has no relation of a name, with the close one
// or the ones it has.
func noRelation(entity, name string, rels map[string]*yaml.Node) string {
	if len(rels) == 0 {
		return entity + " has no relation " + name + "; it has no relations"
	}
	return entity + " has no relation " + name + suggest(name, rels)
}

// viewFields are the fields of a view: every field of its entity, and each
// property it adds, a path as the field it reads. Rows are records, not a
// field, and are left out.
func (d *design) viewFields(view *yaml.Node) map[string]*yaml.Node {
	from := source.Str(source.Child(view, "from"))
	m := fieldsOf(d.entities[from])
	for _, p := range source.Pairs(source.Child(view, "properties")) {
		if source.Child(p.Value, "rows") != nil {
			continue
		}
		m[p.Key.Value] = p.Value
		if path := source.Str(source.Child(p.Value, "path")); path != "" && d.entities[from] != nil {
			if f, _ := d.viewPath(from, path); f != nil {
				m[p.Key.Value] = f
			}
		}
	}
	return m
}

// checkViews checks that a view reads from an entity, that each path
// follows relations to one record and ends in a field, that each count
// counts a relation to many, that each rows names a one-to-many relation,
// that no property repeats a field of the
// entity, that no view shares an entity's name, and that no request body
// names a view.
func (c *checker) checkViews(d *design) {
	for _, v := range source.Pairs(source.Child(d.root, "views")) {
		name := v.Key.Value
		if d.entities[name] != nil {
			c.add(v.Key, source.Pointer("views", name), RuleView, "%s is the name of an entity too; a view and an entity are one namespace, since each becomes a table or view and a schema of its own; rename the view", name)
		}
		fromNode := source.Child(v.Value, "from")
		from := source.Str(fromNode)
		if d.entities[from] == nil {
			if fromNode != nil {
				c.add(fromNode, source.Pointer("views", name, "from"), RuleView, "%s is not an entity of the specification%s", from, suggest(from, d.entities))
			}
			continue
		}
		fields := fieldsOf(d.entities[from])
		for _, p := range source.Pairs(source.Child(v.Value, "properties")) {
			prop := p.Key.Value
			if fields[prop] != nil {
				c.add(p.Key, source.Pointer("views", name, "properties", prop), RuleView, "%s is a field of %s already, and the view carries every field of its entity; leave it out, or give the added field another name", prop, from)
			}
			if n := source.Child(p.Value, "path"); n != nil {
				if _, why := d.viewPath(from, n.Value); why != "" {
					c.add(n, source.Pointer("views", name, "properties", prop, "path"), RuleView, "%s does not resolve: %s", n.Value, why)
				}
			}
			if n := source.Child(p.Value, "count"); n != nil {
				rels := topMap(d.entities[from], "relations")
				switch r := rels[n.Value]; {
				case r == nil:
					c.add(n, source.Pointer("views", name, "properties", prop, "count"), RuleView, "%s", noRelation(from, n.Value, rels))
				case !toMany[source.Str(source.Child(r, "kind"))]:
					c.add(n, source.Pointer("views", name, "properties", prop, "count"), RuleView, "%s.%s is %s, which leads to one record; a count counts one-to-many and many-to-many relations; read it with a path instead", from, n.Value, source.Str(source.Child(r, "kind")))
				}
			}
			if n := source.Child(p.Value, "rows"); n != nil {
				rels := topMap(d.entities[from], "relations")
				switch r := rels[n.Value]; {
				case r == nil:
					c.add(n, source.Pointer("views", name, "properties", prop, "rows"), RuleView, "%s", noRelation(from, n.Value, rels))
				case source.Str(source.Child(r, "kind")) == "many-to-many":
					c.add(n, source.Pointer("views", name, "properties", prop, "rows"), RuleView, "%s.%s is many-to-many, and its rows are records of its join entity; carry them through a one-to-many relation of %s to the join entity instead", from, n.Value, from)
				case source.Str(source.Child(r, "kind")) != "one-to-many":
					c.add(n, source.Pointer("views", name, "properties", prop, "rows"), RuleView, "%s.%s is %s, which leads to one record; rows are the records of a one-to-many relation; read its fields with a path instead", from, n.Value, source.Str(source.Child(r, "kind")))
				}
			}
		}
	}
	for _, o := range d.opList {
		body := source.Child(o.node, "requestBody")
		walk(body, nil, func(n *yaml.Node, path []string) {
			ref := source.Child(n, "$ref")
			if ref != nil && strings.HasPrefix(ref.Value, "#/views/") {
				c.add(ref, o.pointer(append(append([]string{"requestBody"}, path...), "$ref")...), RuleView, "%s is a view, and a view is never written; take the entity it reads from, or the fields the operation takes", strings.TrimPrefix(ref.Value, "#/views/"))
			}
		})
	}
}
