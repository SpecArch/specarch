package gensql

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/ownership"
)

// The rows a role reaches (meta-model 0.3, ADR-095): a role's rows rule
// over an entity becomes a row security policy on PostgreSQL, one per
// role and table, permissive, so a caller holding several roles reaches
// the union. Once a table has a rule, every other role gets a policy
// that holds for every row, so that a role with no rule reaches the
// table whole, as it did before; the table's owner is not bound, so the
// service's own connection reaches it whole too. The caller's properties
// are read from the connection as the implementation file maps them
// (#/caller/properties/<name>, a SQL expression), by default from a
// setting named after the property, an empty setting read as NULL; the
// role is the database role the mapping of #/roles/<name> names, by
// default the role's name with underscores. An engine with no row
// security policy this version writes gets nothing for the rule, a
// warning at the rule and a comment at the table, so that the service's
// own filter is seen to be the only one; the rule is never dropped.

// callerSetting is the prefix of the connection setting a caller property
// is read from when its mapping names no expression.
const callerSetting = "specarch.caller."

// policy is one row security policy, as the migration writes it and the
// snapshot keeps it.
type policy struct {
	Role   string // the role of the design
	Name   string // the policy's name
	DBRole string // the database role the policy applies to
	Using  string // the condition, in SQL
}

// callerColumn is a caller property as the translator writes it: the SQL
// expression that reads it and whether it is boolean.
type callerColumn struct {
	sql     string
	boolean bool
}

// callerColumns are the caller's properties, each with the expression that
// reads it from the connection.
func (g *gen) callerColumns() map[string]callerColumn {
	out := map[string]callerColumn{}
	mappings := obj0(g.impl.Content["mappings"])
	for name, p := range obj0(obj0(g.spec["caller"])["properties"]) {
		field := obj0(p)
		col := callerColumn{boolean: text(field["type"]) == "boolean"}
		if target := text(obj0(mappings["#/caller/properties/"+name])["target"]); target != "" {
			col.sql = target
		} else if typ, _, ok := g.render(field, snake(name)); ok {
			col.sql = fmt.Sprintf("CAST(NULLIF(current_setting(%s, true), '') AS %s)", quote(callerSetting+snake(name)), typ)
		} else {
			col.sql = fmt.Sprintf("NULLIF(current_setting(%s, true), '')", quote(callerSetting+snake(name)))
		}
		out[expr.CallerName+"."+name] = col
	}
	return out
}

// dbRole is the database role a role of the design is on this stack: the
// mapping's "role <name>", or the role's name with underscores.
func (g *gen) dbRole(role string) string {
	target := text(obj0(obj0(g.impl.Content["mappings"])["#/roles/"+role])["target"])
	if name, found := strings.CutPrefix(target, "role "); found && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return strings.ReplaceAll(role, "-", "_")
}

// rowsRules are the roles with a rows rule over the entity, in name order,
// each with its rule, and the roles without one.
func (g *gen) rowsRules(entity string) ([][2]string, []string) {
	var out [][2]string
	var whole []string
	roles := obj0(g.spec["roles"])
	for _, role := range sortedKeys(roles) {
		if rule := text(obj0(obj0(roles[role])["rows"])[entity]); rule != "" {
			out = append(out, [2]string{role, rule})
		} else {
			whole = append(whole, role)
		}
	}
	return out, whole
}

// everyRow is the condition of the policy of a role with no rule.
const everyRow = "true"

// policies are the row security policies of an entity's table, on
// PostgreSQL; on another dialect each rule is a warning and a comment,
// and no policy.
func (g *gen) policies(entity, tableName string, props map[string]any) ([]policy, []string) {
	rules, whole := g.rowsRules(entity)
	if len(rules) == 0 {
		return nil, nil
	}
	if g.dialect != "postgresql" {
		var comments []string
		for _, r := range rules {
			role, rule := r[0], r[1]
			g.warn("/roles/"+role+"/rows/"+entity, "the rows rule of %s on %s (%s) is not written: %s has no row security policy this version writes, so the service's own filter is the only one", role, entity, rule, g.dialect)
			comments = append(comments, fmt.Sprintf("-- The rows rule of %s on %s, %s, is not written: %s has no row security policy this version writes, so the service's own filter is the only one.", role, entity, rule, g.dialect))
		}
		return nil, comments
	}
	caller := g.callerColumns()
	var out []policy
	for _, r := range rules {
		role, rule := r[0], r[1]
		using, why := g.rowsCondition(rule, props, caller)
		if why != "" {
			g.problem("/roles/"+role+"/rows/"+entity, "the rows rule of %s on %s cannot be written in SQL: %s", role, entity, why)
			continue
		}
		out = append(out, policy{Role: role, Name: "rls_" + tableName + "_" + strings.ReplaceAll(role, "-", "_"), DBRole: g.dbRole(role), Using: using})
	}
	for _, role := range whole {
		out = append(out, policy{Role: role, Name: "rls_" + tableName + "_" + strings.ReplaceAll(role, "-", "_"), DBRole: g.dbRole(role), Using: everyRow})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Role < out[j].Role })
	return out, nil
}

// rowsCondition translates a rows rule into the dialect, with the caller's
// properties as the connection reads them.
func (g *gen) rowsCondition(src string, props map[string]any, caller map[string]callerColumn) (string, string) {
	n, errs := expr.Parse(src)
	if len(errs) > 0 || n == nil {
		return "", "it does not parse"
	}
	t := &translator{g: g, props: props, caller: caller}
	out := t.tr(n, true)
	return out, t.err
}

// create is the statement of one policy.
func (p policy) create(tableName string) string {
	return fmt.Sprintf("CREATE POLICY %s ON %s FOR ALL TO %s USING (%s) WITH CHECK (%s)", p.Name, tableName, p.DBRole, p.Using, p.Using)
}

// Row security is switched on and not forced: the table's owner, the
// service's own connection, reaches every row, and a request reaches the
// rows of the roles it takes with SET LOCAL ROLE.
func enableRowSecurity(tableName string) []string {
	return []string{"ALTER TABLE " + tableName + " ENABLE ROW LEVEL SECURITY"}
}

func disableRowSecurity(tableName string) []string {
	return []string{"ALTER TABLE " + tableName + " DISABLE ROW LEVEL SECURITY"}
}

// policyStatements are the statements after a table that put its
// policies in place.
func policyStatements(tableName string, ps []policy) []string {
	if len(ps) == 0 {
		return nil
	}
	out := enableRowSecurity(tableName)
	for _, p := range ps {
		out = append(out, p.create(tableName))
	}
	return out
}

// snapshotRows is what the snapshot keeps of the policies: per entity,
// per role, the database role and the condition as written, so that a
// changed rule or mapping is a changed policy.
func (g *gen) snapshotRows() map[string]any {
	out := map[string]any{}
	if g.dialect != "postgresql" {
		return out
	}
	for name, e := range obj0(g.spec["entities"]) {
		if g.owned.Covers(ownership.Entity("entities", name)) {
			continue
		}
		ps, _ := g.policies(name, g.tableName(name), obj0(obj0(e)["properties"]))
		if len(ps) == 0 {
			continue
		}
		kept := map[string]any{}
		for _, p := range ps {
			kept[p.Role] = map[string]any{"role": p.DBRole, "using": p.Using}
		}
		out["#/entities/"+name] = kept
	}
	return out
}

// diffPolicies writes the statements that move a table's policies from
// the snapshot's to the specification's: a policy added, changed or
// removed, row security switched on with the first and off with the last.
// A policy holds no data, so every step is in the expand file.
func (g *gen) diffPolicies(s *steps, tableName string, prev, cur map[string]any) {
	if g.dialect != "postgresql" {
		return
	}
	roles := map[string]bool{}
	for r := range prev {
		roles[r] = true
	}
	for r := range cur {
		roles[r] = true
	}
	sorted := make([]string, 0, len(roles))
	for r := range roles {
		sorted = append(sorted, r)
	}
	sort.Strings(sorted)
	if len(prev) == 0 && len(cur) > 0 {
		for _, stmt := range enableRowSecurity(tableName) {
			s.add(stmt)
		}
	}
	for _, role := range sorted {
		p, c := obj0(prev[role]), obj0(cur[role])
		name := "rls_" + tableName + "_" + strings.ReplaceAll(role, "-", "_")
		same := p != nil && c != nil && text(p["role"]) == text(c["role"]) && text(p["using"]) == text(c["using"])
		if same {
			continue
		}
		if p != nil {
			s.add("DROP POLICY " + name + " ON " + tableName)
		}
		if c != nil {
			s.add(policy{Role: role, Name: name, DBRole: text(c["role"]), Using: text(c["using"])}.create(tableName))
		}
	}
	if len(prev) > 0 && len(cur) == 0 {
		for _, stmt := range disableRowSecurity(tableName) {
			s.add(stmt)
		}
	}
}
