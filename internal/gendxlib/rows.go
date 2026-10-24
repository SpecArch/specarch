package gendxlib

import (
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/implies"
	"github.com/SpecArch/specarch/internal/ownership"
)

// The rows a role reaches (meta-model 0.3, ADR-095): a role's rows rule
// over an entity, written or derived from the tenant scope (ADR-179),
// limits every operation and job that reads or writes the entity. The
// service's connection owns the tables, which row security does not bind
// (specarch-gen-sql), so a rule the generated code does not apply is
// applied by nothing: each one is a warning at the operation or the job,
// naming the rule, never left out silently.

// rowsRule is one role's rows rule over an entity.
type rowsRule struct{ role, rule string }

// rowsRules are the rows rules over an entity, in the order of the roles'
// names.
func (g *gen) rowsRules(entity string) []rowsRule {
	if entity == "" {
		return nil
	}
	roles := obj(g.spec["roles"])
	var out []rowsRule
	for _, role := range sortedKeys(roles) {
		if rule := text(obj(obj(roles[role])["rows"])[entity]); rule != "" {
			out = append(out, rowsRule{role, rule})
		}
	}
	return out
}

// rowsEntity is the entity an operation reads or writes, as specarch
// derive finds it for the rows cases: its answer's, its request body's,
// its guard's, its list's, or the one a path parameter is the key of, in
// that order.
func (g *gen) rowsEntity(o operation) string {
	responses := obj(o.node["responses"])
	for _, code := range sortedKeys(responses) {
		if !strings.HasPrefix(code, "2") {
			continue
		}
		content := obj(obj(responses[code])["content"])
		for _, media := range sortedKeys(content) {
			s := obj(obj(content[media])["schema"])
			if items := obj(s["items"]); items != nil {
				s = items
			}
			ref := text(s["$ref"])
			if e, ok := strings.CutPrefix(ref, "#/entities/"); ok {
				return e
			}
			if v, ok := strings.CutPrefix(ref, "#/views/"); ok {
				return text(obj(obj(g.spec["views"])[v])["from"])
			}
		}
	}
	content := obj(obj(o.node["requestBody"])["content"])
	for _, media := range sortedKeys(content) {
		if e, ok := strings.CutPrefix(text(obj(obj(content[media])["schema"])["$ref"]), "#/entities/"); ok {
			return e
		}
	}
	if e := text(obj(o.node["guard"])["entity"]); e != "" {
		return e
	}
	if l := obj(o.node["listOf"]); l != nil {
		if e := g.listEntity(l); e != "" {
			return e
		}
	}
	return g.keyEntity(o)
}

// keyEntity is the entity a path parameter is the key of: the last path
// parameter named after an entity with Id after it (loanId, loan_id), or
// "".
func (g *gen) keyEntity(o operation) string {
	byKey := map[string]string{}
	for _, name := range sortedKeys(g.entities) {
		if k := keyName(name + "Id"); byKey[k] == "" {
			byKey[k] = name
		}
	}
	found := ""
	for _, p := range append(list(o.item["parameters"]), list(o.node["parameters"])...) {
		pm := obj(p)
		if text(pm["in"]) != "path" {
			continue
		}
		if e := byKey[keyName(text(pm["name"]))]; e != "" {
			found = e
		}
	}
	return found
}

// keyName is a name with case, underscores and hyphens ignored.
func keyName(s string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(s))
}

// operationRules are the rows rules over the entity an operation reads or
// writes, of the roles that hold its permission: a role without it is
// refused before any row is read, as specarch derive's cases say.
func (g *gen) operationRules(o operation) (string, []rowsRule) {
	entity := g.rowsEntity(o)
	perm := text(o.node["permission"])
	var out []rowsRule
	for _, r := range g.rowsRules(entity) {
		if g.roleHolds(r.role)[perm] {
			out = append(out, r)
		}
	}
	return entity, out
}

// roleHolds is every permission a role grants, with the ones they imply.
func (g *gen) roleHolds(role string) map[string]bool {
	perms := obj(g.spec["permissions"])
	return implies.Holds(texts(list(obj(obj(g.spec["roles"])[role])["permissions"])), func(name string) []string {
		return texts(list(obj(perms[name])["implies"]))
	})
}

// rowsNotApplied warns once per rule over the entity an operation reads or
// writes that its handler does not apply.
func (g *gen) rowsNotApplied(o operation) {
	entity, rules := g.operationRules(o)
	for _, r := range rules {
		g.problem("warning", o.pointer(), "%s reads or writes %s, which the rows rule of %s limits (%s), and the generated handler does not apply the rule: the service's connection reaches every row, so the handler answers and changes the rows of every caller; apply the rule in the service", o.id, entity, r.role, r.rule)
	}
}

// jobRowsNotApplied warns once per rule of a job's role over an entity the
// job reads or writes: the job's function is the service's, and nothing
// generated applies the rule.
func (g *gen) jobRowsNotApplied() {
	jobs := obj(g.spec["jobs"])
	for _, name := range sortedKeys(jobs) {
		if g.owned.Covers(ownership.Entity("jobs", name)) {
			continue
		}
		job := obj(jobs[name])
		role := text(job["role"])
		if role == "" {
			continue
		}
		seen := map[string]bool{}
		var entities []string
		for _, key := range []string{"reads", "writes"} {
			for _, e := range texts(list(job[key])) {
				if !seen[e] {
					seen[e] = true
					entities = append(entities, e)
				}
			}
		}
		sort.Strings(entities)
		for _, entity := range entities {
			for _, r := range g.rowsRules(entity) {
				if r.role == role {
					g.problem("warning", "/jobs/"+name, "job %s acts as %s on %s, which the rows rule of %s limits (%s), and nothing generated applies the rule: the job's function, which the service writes, reaches every row unless it applies the rule itself", name, role, entity, r.role, r.rule)
				}
			}
		}
	}
}
