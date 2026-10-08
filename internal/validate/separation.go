package validate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SpecArch/specarch/internal/source"
)

// checkSeparationOfDuties checks each separation-of-duties set: every
// permission it names is declared and is not public, its cardinality is no
// more than the permissions it names, and no role grants cardinality or
// more of them. Which person holds which roles is data, so two roles that
// together reach a set are left to the techspec's list.
func (c *checker) checkSeparationOfDuties(d *design) {
	for _, set := range source.Pairs(source.Child(d.root, "separationOfDuties")) {
		name := set.Key.Value
		var perms []string
		seen := map[string]bool{}
		for i, p := range source.Items(source.Child(set.Value, "permissions")) {
			ptr := source.Pointer("separationOfDuties", name, "permissions", fmt.Sprint(i))
			switch {
			case p.Value == "public":
				c.add(p, ptr, RuleSeparationOfDuties, "set %s names public, which everyone holds, so no role can be kept from it; name only permissions a role grants", name)
			case d.permissions[p.Value] == nil:
				c.add(p, ptr, RuleSeparationOfDuties, "set %s names permission %s, which is not declared; add it under permissions%s", name, p.Value, strings.Replace(suggest(p.Value, d.permissions), "; there are none in the specification", "", 1))
			}
			if p.Value != "" && !seen[p.Value] {
				seen[p.Value] = true
				perms = append(perms, p.Value)
			}
		}
		cardinality := 2
		if n := source.Child(set.Value, "cardinality"); n != nil {
			v, err := strconv.Atoi(n.Value)
			if err != nil || v < 2 {
				continue
			}
			cardinality = v
			if cardinality > len(perms) && len(perms) >= 2 {
				c.add(n, source.Pointer("separationOfDuties", name, "cardinality"), RuleSeparationOfDuties,
					"set %s asks that no holder reach %d of its permissions but names only %d, so no role can ever break it; lower cardinality to %d or name more permissions", name, cardinality, len(perms), len(perms))
				continue
			}
		}
		for _, role := range source.Pairs(source.Child(d.root, "roles")) {
			grants := map[string]bool{}
			for _, p := range source.Items(source.Child(role.Value, "permissions")) {
				grants[p.Value] = true
			}
			var held []string
			for _, p := range perms {
				if grants[p] {
					held = append(held, p)
				}
			}
			if len(held) >= cardinality {
				c.add(role.Key, source.Pointer("roles", role.Key.Value), RuleSeparationOfDuties,
					"role %s grants %s of set %s, and no holder may have %d of its permissions; split them between roles", role.Key.Value, joinAnd(held), name, cardinality)
			}
		}
	}
}
