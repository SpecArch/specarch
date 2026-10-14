package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// checkAudience reports an element for everyone that names an internal
// element of the specification (ADR-119): through a $ref, a relation's
// target, a page's entity or a requirement's needs. A document for
// everyone leaves the internal element out, so it would name what it may
// not show. An element inside an internal one is internal with it.
func (c *checker) checkAudience(d *design) {
	internal := func(n *yaml.Node) bool { return source.Str(source.Child(n, "audience")) == "internal" }
	named := func(n *yaml.Node, path []string, element []string, target []string, node *yaml.Node) {
		if node == nil || !internal(node) {
			return
		}
		from, to := source.Pointer(element...), source.Pointer(target...)
		c.add(n, source.Pointer(path...), RuleAudience,
			"%s is internal, and %s is for everyone and names it, so a document for everyone would name what it may not show; give %s audience: internal too, or take audience: internal off %s", to, from, from, to)
	}
	var visit func(n *yaml.Node, path, element []string)
	visit = func(n *yaml.Node, path, element []string) {
		n = source.Deref(n)
		if n == nil {
			return
		}
		at := func(key string) []string { return append(append([]string{}, path...), key) }
		switch n.Kind {
		case yaml.MappingNode:
			if spec.IsElement(path) {
				if internal(n) {
					return
				}
				element = path
			}
			if v := source.Child(n, "$ref"); source.IsScalar(v) && element != nil {
				if section, name, ok := localRef(v.Value); ok {
					sections := map[string]map[string]*yaml.Node{"entities": d.entities, "enums": d.enums, "views": d.views, "schemas": d.schemas}
					named(v, at("$ref"), element, []string{section, name}, sections[section][name])
				}
			}
			switch {
			case len(path) == 4 && path[0] == "entities" && path[2] == "relations":
				if t := source.Child(n, "target"); source.IsScalar(t) && !isCross(t.Value) {
					named(t, at("target"), element, []string{"entities", t.Value}, d.entities[t.Value])
				}
			case len(path) == 2 && path[0] == "pages":
				if e := source.Child(n, "entity"); source.IsScalar(e) && !isCross(e.Value) {
					named(e, at("entity"), element, []string{"entities", e.Value}, d.entities[e.Value])
				}
			case len(path) == 2 && path[0] == "requirements":
				for i, item := range source.Items(source.Child(n, "needs")) {
					named(item, append(at("needs"), fmt.Sprint(i)), element, []string{"needs", item.Value}, d.needs[item.Value])
				}
			}
			for _, p := range source.Pairs(n) {
				visit(p.Value, at(p.Key.Value), element)
			}
		case yaml.SequenceNode:
			for i, item := range source.Items(n) {
				visit(item, at(fmt.Sprint(i)), element)
			}
		}
	}
	visit(d.root, nil, nil)
}

// localRef splits a $ref of this specification, #/<section>/<name>.
func localRef(s string) (section, name string, ok bool) {
	rest, ok := strings.CutPrefix(s, "#/")
	if !ok {
		return "", "", false
	}
	section, name, ok = strings.Cut(rest, "/")
	return section, name, ok && name != "" && !strings.Contains(name, "/")
}
