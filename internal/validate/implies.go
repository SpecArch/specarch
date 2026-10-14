package validate

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/implies"
	"github.com/SpecArch/specarch/internal/source"
)

// checkImplies checks each permission's implies: every name it lists is
// declared and is not public, public implies nothing, and no permission
// implies itself through the implications. A cycle is reported at every
// permission on it, each with the way round.
func (c *checker) checkImplies(d *design) {
	var names []string
	for _, p := range source.Pairs(source.Child(d.root, "permissions")) {
		names = append(names, p.Key.Value)
	}
	sort.Strings(names)
	for _, name := range names {
		list := source.Child(d.permissions[name], "implies")
		if list == nil {
			continue
		}
		if name == "public" && len(source.Items(list)) > 0 {
			c.add(list, source.Pointer("permissions", name, "implies"), RuleImplies,
				"public implies %s, which would give it to everyone, signed in or not; grant it to a role instead", joinAnd(itemValues(list)))
			continue
		}
		for i, p := range source.Items(list) {
			ptr := source.Pointer("permissions", name, "implies", fmt.Sprint(i))
			switch {
			case p.Value == "public":
				c.add(p, ptr, RuleImplies, "permission %s implies public, which everyone holds already; take it out of the list", name)
			case d.permissions[p.Value] == nil:
				c.add(p, ptr, RuleImplies, "permission %s implies %s, which is not declared; add it under permissions%s", name, p.Value, strings.Replace(suggest(p.Value, d.permissions), "; there are none in the specification", "", 1))
			}
		}
		if way := d.impliesCycle(name); way != nil {
			c.add(list, source.Pointer("permissions", name, "implies"), RuleImplies,
				"permission %s implies itself through %s; take one of the implications out", name, strings.Join(way, " → "))
		}
	}
}

// impliesCycle is the shortest way from a permission back to itself
// through the implications, starting and ending with its name, or nil when
// there is none. The implications are followed in the order they are
// written, so the way is the same in every run.
func (d *design) impliesCycle(name string) []string {
	from := map[string]string{}
	queue := []string{name}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		for _, next := range d.impliedBy(at) {
			if next == name {
				way := []string{name}
				for p := at; p != name; p = from[p] {
					way = append([]string{p}, way...)
				}
				return append([]string{name}, way...)
			}
			if _, seen := from[next]; !seen {
				from[next] = at
				queue = append(queue, next)
			}
		}
	}
	return nil
}

// impliedBy is the declared, non-public permissions a permission lists
// under implies, in the order written.
func (d *design) impliedBy(name string) []string {
	var out []string
	for _, p := range source.Items(source.Child(d.permissions[name], "implies")) {
		if p.Value != "public" && d.permissions[p.Value] != nil {
			out = append(out, p.Value)
		}
	}
	return out
}

// holds is every permission a holder of the given ones holds: those and
// every one they imply, transitively.
func (d *design) holds(perms []string) map[string]bool {
	return implies.Holds(perms, d.impliedBy)
}

// roleHolds is every permission a role grants, with the ones they imply.
func (d *design) roleHolds(role *yaml.Node) map[string]bool {
	return d.holds(itemValues(source.Child(role, "permissions")))
}

func itemValues(list *yaml.Node) []string {
	var out []string
	for _, p := range source.Items(list) {
		out = append(out, p.Value)
	}
	return out
}
