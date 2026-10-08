package validate

import (
	"strconv"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/wirename"
)

// checkWireNames refuses two properties of one object that go on the wire
// under one name, once the specification names its wire names (ADR-062):
// an entity's, a view's together with its entity's, and the inline objects
// of a body, a parameter's schema and a message's payload, at any depth.
func (c *checker) checkWireNames(d *design) {
	rule := source.Str(source.Child(source.Child(d.root, "info"), "wireNames"))
	if rule != wirename.SnakeCase {
		return
	}
	w := wireCheck{c: c, rule: rule}
	for _, e := range source.Pairs(source.Child(d.root, "entities")) {
		w.object(nil, source.Child(e.Value, "properties"), []string{"entities", e.Key.Value})
	}
	for _, v := range source.Pairs(source.Child(d.root, "views")) {
		from := d.entities[source.Str(source.Child(v.Value, "from"))]
		w.object(source.Child(from, "properties"), source.Child(v.Value, "properties"), []string{"views", v.Key.Value})
	}
	for _, p := range source.Pairs(source.Child(d.root, "paths")) {
		w.parameters(source.Child(p.Value, "parameters"), []string{"paths", p.Key.Value})
		for _, m := range methods {
			op := source.Child(p.Value, m)
			if op == nil {
				continue
			}
			at := []string{"paths", p.Key.Value, m}
			w.parameters(source.Child(op, "parameters"), at)
			w.content(source.Child(source.Child(op, "requestBody"), "content"), append(at, "requestBody"))
			for _, r := range source.Pairs(source.Child(op, "responses")) {
				w.content(source.Child(r.Value, "content"), append(append([]string{}, at...), "responses", r.Key.Value))
			}
		}
	}
	for _, ch := range source.Pairs(source.Child(d.root, "channels")) {
		for _, m := range source.Pairs(source.Child(ch.Value, "messages")) {
			w.field(source.Child(m.Value, "payload"), []string{"channels", ch.Key.Value, "messages", m.Key.Value, "payload"})
		}
	}
}

type wireCheck struct {
	c    *checker
	rule string
}

func (w wireCheck) parameters(params *yaml.Node, at []string) {
	if params == nil || params.Kind != yaml.SequenceNode {
		return
	}
	for i, p := range params.Content {
		w.field(source.Child(p, "schema"), append(append([]string{}, at...), "parameters", strconv.Itoa(i), "schema"))
	}
}

func (w wireCheck) content(content *yaml.Node, at []string) {
	for _, mt := range source.Pairs(content) {
		w.field(source.Child(mt.Value, "schema"), append(append([]string{}, at...), "content", mt.Key.Value, "schema"))
	}
}

// field checks a field's own properties and its items', at any depth.
func (w wireCheck) field(f *yaml.Node, at []string) {
	if f == nil || f.Kind != yaml.MappingNode {
		return
	}
	w.object(nil, source.Child(f, "properties"), at)
	w.field(source.Child(f, "items"), append(append([]string{}, at...), "items"))
}

// object checks the properties of one object: first those it carries from
// elsewhere (a view's entity), which are reported against nothing, then its
// own, each reported where it takes a wire name already taken.
func (w wireCheck) object(carried, props *yaml.Node, at []string) {
	taken := map[string]string{}
	for _, p := range source.Pairs(carried) {
		if _, ok := taken[wirename.Of(w.rule, p.Key.Value)]; !ok {
			taken[wirename.Of(w.rule, p.Key.Value)] = p.Key.Value
		}
	}
	for _, p := range source.Pairs(props) {
		name := p.Key.Value
		wire := wirename.Of(w.rule, name)
		path := append(append([]string{}, at...), "properties", name)
		if other, ok := taken[wire]; ok && other != name {
			w.c.add(p.Key, source.Pointer(path...), RuleWireName, "%s and %s both go on the wire as %s (info.wireNames: %s); rename one", other, name, wire, w.rule)
		} else if !ok {
			taken[wire] = name
		}
		w.field(p.Value, path)
	}
}
