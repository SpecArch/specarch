package spec

import (
	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/metamodel"
	"github.com/SpecArch/specarch/internal/source"
)

// The tenant scope (ADR-179): tenant names the caller's property, the key
// every tenant-owned entity carries and the entities every tenant reads
// whole. The document as read holds the rule <key> == caller.<property>
// under every role's row policies for every entity that carries the key and is not
// shared, unless the role writes its own rule for it; each added node sits
// at the position of tenant. The file keeps the shorthand, and the
// validator checks the scope at tenant and at the entities.

// tenantRules adds the row policies the tenant scope derives to the merged
// document and returns the value node of each, so that a check reports a
// problem of a derived rule once, at the entity, and not at a rule nobody
// wrote. A scope with no property or no key derives nothing; the schema
// reports it.
func tenantRules(s *Spec, merged *yaml.Node) map[*yaml.Node]bool {
	tenantKey := source.Key(merged, "tenant")
	tenant := source.Child(merged, "tenant")
	property := source.Str(source.Child(tenant, "property"))
	key := source.Str(source.Child(tenant, "key"))
	if tenantKey == nil || property == "" || key == "" {
		return nil
	}
	shared := map[string]bool{}
	for _, e := range source.Items(source.Child(tenant, "shared")) {
		shared[source.Str(e)] = true
	}
	var owned []string
	for _, e := range source.Pairs(source.Child(merged, "entities")) {
		if !shared[e.Key.Value] && source.Child(source.Child(e.Value, "properties"), key) != nil {
			owned = append(owned, e.Key.Value)
		}
	}
	if len(owned) == 0 {
		return nil
	}
	file := s.Files[tenantKey]
	node := func(kind yaml.Kind, tag, value string) *yaml.Node {
		n := &yaml.Node{Kind: kind, Tag: tag, Value: value, Line: tenantKey.Line, Column: tenantKey.Column}
		s.Files[n] = file
		return n
	}
	rule := key + " == caller." + property
	derived := map[*yaml.Node]bool{}
	for _, r := range source.Pairs(source.Child(merged, "roles")) {
		if r.Value.Kind != yaml.MappingNode {
			continue // the schema reports it
		}
		rows := rowsOf(s, r.Value, node, policiesKey(merged))
		if rows == nil {
			continue
		}
		for _, entity := range owned {
			if source.Key(rows, entity) != nil {
				continue // the role's own rule stands alone
			}
			v := node(yaml.ScalarNode, "!!str", rule)
			rows.Content = append(rows.Content, node(yaml.ScalarNode, "!!str", entity), v)
			derived[v] = true
		}
	}
	return derived
}

// policiesKey is the key a role's row policies are added under in the
// file's version: rows before 1.0.0, whose schema has no rowPolicies.
func policiesKey(merged *yaml.Node) string {
	if metamodel.Design.IsEarlier(source.Str(source.Child(merged, "specarch"))) {
		return metamodel.RowPoliciesOldKey
	}
	return metamodel.RowPoliciesKey
}

// rowsOf is a role's row policies, under the key the role gives them or,
// when it gives none, under key, added; one shared through an alias is
// copied, so that a rule derived for one role is not added to another.
// Policies that are no mapping are nil; the schema reports it.
func rowsOf(s *Spec, role *yaml.Node, node func(yaml.Kind, string, string) *yaml.Node, key string) *yaml.Node {
	if given, _ := metamodel.RowPolicies(role); source.Child(role, given) != nil {
		key = given
	}
	for i := 0; i+1 < len(role.Content); i += 2 {
		if role.Content[i].Value != key {
			continue
		}
		rows := source.Deref(role.Content[i+1])
		if rows == nil || rows.Kind != yaml.MappingNode {
			return nil
		}
		if role.Content[i+1].Kind == yaml.AliasNode {
			cp := *rows
			cp.Content = append([]*yaml.Node(nil), rows.Content...)
			role.Content[i+1] = &cp
			s.Files[&cp] = s.Files[rows]
			return &cp
		}
		return rows
	}
	rows := node(yaml.MappingNode, "!!map", "")
	role.Content = append(role.Content, node(yaml.ScalarNode, "!!str", key), rows)
	return rows
}
