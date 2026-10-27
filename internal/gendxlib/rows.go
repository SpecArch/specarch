package gendxlib

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/genopenapi"
	"github.com/SpecArch/specarch/internal/implies"
	"github.com/SpecArch/specarch/internal/ownership"
	"github.com/SpecArch/specarch/internal/wirename"
)

// The rows a role reaches (meta-model 0.3, ADR-095): a role's rows rule
// over an entity, written or derived from the tenant scope (ADR-179),
// limits every operation and job that reads or writes the entity. The
// service's connection owns the tables, which row security does not bind
// (specarch-gen-sql), so the handlers apply the rules themselves
// (ADR-180): the list, the read by key and the create add the condition
// rows<Entity> builds from the Caller the service answers from callerOf.
// A rule the generated code does not apply is applied by nothing, so each
// one is a warning at the operation or the job, naming the rule, never
// left out silently.

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
		if rule := text(obj(obj(roles[role])["rowPolicies"])[entity]); rule != "" {
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
		if perm == "signedIn" || g.roleHolds(r.role)[perm] {
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
// writes that its handler does not apply: every rule, when the entity's
// rules are not all written in SQL, with the reason.
func (g *gen) rowsNotApplied(o operation) {
	entity, rules := g.operationRules(o)
	if len(rules) == 0 {
		return
	}
	c := g.rowsCondition(entity)
	for _, r := range rules {
		switch {
		case c.why != "":
			g.problem("warning", o.pointer(), "%s reads or writes %s, which the rows rule of %s limits (%s), and no handler applies the rules over %s: %s; the service's connection reaches every row, so apply the rule in the service", o.id, entity, r.role, r.rule, entity, c.why)
		default:
			g.problem("warning", o.pointer(), "%s reads or writes %s, which the rows rule of %s limits (%s), and the generated handler leaves the rule to the body the service writes, while the service's connection reaches every row: apply it there, with the condition rows%s(caller) gives", o.id, entity, r.role, r.rule, pascal(entity))
		}
	}
}

// applies reports whether an operation's handler applies the rows rules
// over its entity: a list, a read by key or a create over an entity whose
// rules are all written in SQL.
func (g *gen) applies(o operation, kind, entity string) bool {
	switch kind {
	case "list", "read", "create":
	default:
		return false
	}
	return len(g.rowsRules(entity)) > 0 && g.rowsEntity(o) == entity && g.rowsCondition(entity).why == ""
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

// rowsCond is the SQL of every rule over an entity, or why one cannot be
// written.
type rowsCond struct {
	roles []rowsSQL // in the order of the roles' names
	whole []string  // the roles with no rule over the entity
	why   string
}

// rowsSQL is one role's rule in SQL: the condition, the caller's properties
// it reads, in the order of their first use, and its literals' parameters.
type rowsSQL struct {
	role, sql string
	callers   []string
	literals  [][2]string // parameter name, Go literal
}

// rowsCondition writes the rules over an entity in SQL, once.
func (g *gen) rowsCondition(entity string) rowsCond {
	if c, ok := g.rowsConds[entity]; ok {
		return c
	}
	var c rowsCond
	rules := g.rowsRules(entity)
	ruled := map[string]bool{}
	n := 0
	for _, r := range rules {
		ruled[r.role] = true
		t := &rowsTranslator{g: g, entity: entity, props: obj(obj(g.entities[entity])["properties"]), n: &n, seen: map[string]bool{}}
		node, errs := expr.Parse(r.rule)
		if len(errs) > 0 || node == nil {
			t.fail("the rule of %s does not parse", r.role)
		} else {
			t.out.sql = t.tr(node, true)
		}
		if t.err != "" {
			c.why = fmt.Sprintf("the rule of %s, %s, cannot be written in the SQL every engine dxlib serves reads: %s", r.role, r.rule, t.err)
			break
		}
		t.out.role = r.role
		c.roles = append(c.roles, t.out)
	}
	for _, role := range sortedKeys(obj(g.spec["roles"])) {
		if !ruled[role] {
			c.whole = append(c.whole, role)
		}
	}
	if c.why != "" {
		c.roles, c.whole = nil, nil
	}
	if g.rowsConds == nil {
		g.rowsConds = map[string]rowsCond{}
	}
	g.rowsConds[entity] = c
	return c
}

// rowsTranslator writes one rule in SQL that every engine dxlib serves
// reads, every caller property and literal a bound parameter.
type rowsTranslator struct {
	g      *gen
	entity string
	props  map[string]any
	n      *int // the entity's literals so far, shared by its rules
	seen   map[string]bool
	out    rowsSQL
	err    string
}

func (t *rowsTranslator) fail(format string, args ...any) string {
	if t.err == "" {
		t.err = fmt.Sprintf(format, args...)
	}
	return ""
}

// column is the column of a field of the entity, or "" when the name is
// none.
func (t *rowsTranslator) column(n *expr.Node) string {
	if n.Op != expr.OpName || strings.HasPrefix(n.Text, expr.CallerName+".") {
		return ""
	}
	f := obj(t.props[n.Text])
	if f == nil {
		return t.fail("%s is not a field of %s", n.Text, t.entity)
	}
	if text(f["atRest"]) == "encrypted" {
		return t.fail("%s is encrypted at rest, and the engine cannot compare it", n.Text)
	}
	return genopenapi.Wire(n.Text)
}

// literal is the parameter of a literal, as a Go literal.
func (t *rowsTranslator) literal(goValue string) string {
	*t.n++
	name := fmt.Sprintf("rows_%s_%d", genopenapi.Wire(t.entity), *t.n)
	t.out.literals = append(t.out.literals, [2]string{name, goValue})
	return ":" + name
}

// value is the SQL of a comparison's operand that is not a column: a caller
// property or a literal, as a parameter.
func (t *rowsTranslator) value(n *expr.Node) string {
	switch n.Op {
	case expr.OpName:
		name, ok := strings.CutPrefix(n.Text, expr.CallerName+".")
		if !ok {
			return t.column(n)
		}
		if obj(obj(obj(t.g.spec["caller"])["properties"])[name]) == nil {
			return t.fail("the caller has no property %s", name)
		}
		if !t.seen[name] {
			t.seen[name] = true
			t.out.callers = append(t.out.callers, name)
		}
		return ":caller_" + genopenapi.Wire(name)
	case expr.OpInt, expr.OpUint:
		return t.literal(n.Int.String())
	case expr.OpDouble:
		return t.literal(strconv.FormatFloat(n.Double, 'g', -1, 64))
	case expr.OpText:
		return t.literal(strconv.Quote(n.Text))
	case expr.OpBool:
		return t.literal(strconv.FormatBool(n.Bool))
	}
	return t.fail("%s is not a field, a caller property or a literal", describe(n))
}

var sqlComparison = map[string]string{"==": "=", "!=": "<>", "<": "<", "<=": "<=", ">": ">", ">=": ">="}

// tr writes a node; cond says it stands where a condition is wanted.
func (t *rowsTranslator) tr(n *expr.Node, cond bool) string {
	if t.err != "" {
		return ""
	}
	switch {
	case n.Op == "&&" || n.Op == "||":
		op := map[string]string{"&&": "AND", "||": "OR"}[n.Op]
		return "(" + t.tr(n.Args[0], true) + " " + op + " " + t.tr(n.Args[1], true) + ")"
	case n.Op == expr.OpNot:
		return "NOT (" + t.tr(n.Args[0], true) + ")"
	case sqlComparison[n.Op] != "":
		l, r := n.Args[0], n.Args[1]
		lc, rc := t.column(l), t.column(r)
		if t.err != "" {
			return ""
		}
		for _, side := range []*expr.Node{l, r} {
			switch side.Op {
			case expr.OpName, expr.OpNull, expr.OpInt, expr.OpUint, expr.OpDouble, expr.OpText, expr.OpBool:
			default:
				return t.fail("%s is outside what the condition writes: a field, a caller property or a literal on each side of a comparison", describe(side))
			}
		}
		if lc == "" && rc == "" {
			return t.fail("%s compares no field of %s", n.Op, t.entity)
		}
		if l.Op == expr.OpNull || r.Op == expr.OpNull {
			col := lc
			if col == "" {
				col = rc
			}
			switch n.Op {
			case "==":
				return col + " IS NULL"
			case "!=":
				return col + " IS NOT NULL"
			}
			return t.fail("%s orders against null", n.Op)
		}
		if lc == "" {
			lc = t.value(l)
		}
		if rc == "" {
			rc = t.value(r)
		}
		return lc + " " + sqlComparison[n.Op] + " " + rc
	case n.Op == expr.OpName && cond:
		col := t.column(n)
		if col == "" {
			return t.fail("%s stands alone as a condition, and only a field of the entity can", n.Text)
		}
		if text(obj(t.props[n.Text])["type"]) != "boolean" {
			return t.fail("%s is not a boolean field", n.Text)
		}
		return col + " = " + t.literal("true")
	case n.Op == expr.OpBool && cond:
		if n.Bool {
			return "1 = 1"
		}
		return "1 = 0"
	}
	return t.fail("%s is outside what the condition writes: comparisons of a field, &&, || and !", describe(n))
}

// describe names a node for a reason.
func describe(n *expr.Node) string {
	switch n.Op {
	case expr.OpCall:
		return "the function " + n.Text
	case expr.OpName:
		return n.Text
	case "+", "-", "*", "/", expr.OpNeg:
		return "arithmetic"
	case expr.OpCond:
		return "a conditional"
	}
	return "a " + n.Op
}

// rowsSection writes Caller and, per entity whose rules are all written in
// SQL, the function that builds the condition of the rows a caller
// reaches.
func (g *gen) rowsSection() {
	var entities []string
	for _, e := range sortedKeys(g.entities) {
		if len(g.rowsRules(e)) > 0 && g.rowsCondition(e).why == "" {
			entities = append(entities, e)
		}
	}
	if len(entities) == 0 {
		return
	}
	g.imports["github.com/donnyhardyanto/dxlib/api"] = true
	g.imports["github.com/donnyhardyanto/dxlib/utils"] = true
	props := obj(obj(g.spec["caller"])["properties"])
	g.line("// Caller is who makes a request, as the service's session knows them: the")
	g.line("// roles they hold, by the name id Roles gives each, and the properties of")
	g.line("// the caller of the specification, nil for a property that is null. The")
	g.line("// service answers it from callerOf, which it writes beside this file:")
	g.line("//")
	g.line("//\tfunc callerOf(aepr *api.DXAPIEndPointRequest) (Caller, error)")
	g.line("//")
	g.line("// and the handlers read and change only the rows the caller's roles reach.")
	g.line("type Caller interface {")
	g.line("\tHasRole(nameId string) bool")
	for _, name := range sortedKeys(props) {
		g.line("\t%s() %s", pascal(name), g.callerType(obj(props[name])))
	}
	g.line("}")
	g.line("")
	g.line("// The compiler checks callerOf's signature here, and names it when the")
	g.line("// service has not written it.")
	g.line("var _ func(*api.DXAPIEndPointRequest) (Caller, error) = callerOf")
	g.line("")
	rule := text(obj(g.spec["info"])["roleNames"])
	for _, e := range entities {
		c := g.rowsCondition(e)
		name := "rows" + pascal(e)
		g.line("// %s is the condition the rows of %s a caller reaches meet, and its", name, e)
		g.line("// parameters: the rules of the roles they hold, joined with OR; no")
		g.line("// condition when a role they hold has no rule over %s; and no row", e)
		g.line("// when they hold none of the roles.")
		g.line("func %s(c Caller) (string, utils.JSON) {", name)
		if len(c.whole) > 0 {
			var held []string
			for _, role := range c.whole {
				held = append(held, fmt.Sprintf("c.HasRole(%q)", wirename.Role(rule, role)))
			}
			g.line("\tif %s {", strings.Join(held, " || "))
			g.line("\t\treturn \"\", nil")
			g.line("\t}")
		}
		g.line("\tvar reached []string")
		g.line("\targs := utils.JSON{}")
		for _, r := range c.roles {
			g.line("\tif c.HasRole(%q) {", wirename.Role(rule, r.role))
			g.line("\t\treached = append(reached, %q)", grouped(r.sql))
			for _, p := range r.callers {
				g.line("\t\targs[%q] = c.%s()", "caller_"+genopenapi.Wire(p), pascal(p))
			}
			for _, l := range r.literals {
				g.line("\t\targs[%q] = %s", l[0], l[1])
			}
			g.line("\t}")
		}
		g.imports["strings"] = true
		g.line("\tif len(reached) == 0 {")
		g.line("\t\treturn \"1 = 0\", nil")
		g.line("\t}")
		g.line("\treturn \"(\" + strings.Join(reached, \" OR \") + \")\", args")
		g.line("}")
		g.line("")
	}
}

// grouped is a condition in parentheses, once.
func grouped(sql string) string {
	if strings.HasPrefix(sql, "(") {
		depth := 0
		for i, r := range sql {
			switch r {
			case '(':
				depth++
			case ')':
				depth--
			}
			if depth == 0 {
				if i == len(sql)-1 {
					return sql
				}
				break
			}
		}
	}
	return "(" + sql + ")"
}

// callerType is the Go type of a caller property: a pointer for a nullable
// one, except a list or a map, which is nil itself.
func (g *gen) callerType(s map[string]any) string {
	typ, _ := g.goType(s)
	if nullableType(s) && !strings.HasPrefix(typ, "[]") && !strings.HasPrefix(typ, "*") && !strings.HasPrefix(typ, "map[") && typ != "utils.JSON" {
		return "*" + typ
	}
	return typ
}

// createInRows writes the rest of a create that applies the rows rules: the
// insert in a transaction, the new row selected by its key and the
// condition, and, when the rule does not hold for it, the insert rolled
// back and 403 answered, as WITH CHECK refuses a row outside the policy.
func (g *gen) createInRows(entity, uid string) {
	g.imports["github.com/donnyhardyanto/dxlib/databases"] = true
	g.line("\tif err := Tables.%s.EnsureDatabase(); err != nil {", entity)
	g.line("\t\treturn err")
	g.line("\t}")
	g.line("\tdtx, err := Tables.%s.Database.TransactionBegin(aepr.Context, databases.LevelReadCommitted)", entity)
	g.line("\tif err != nil {")
	g.line("\t\treturn err")
	g.line("\t}")
	if g.standard(entity) {
		g.line("\t_, _, err = Tables.%s.TxInsertWithAudit(aepr, dtx, data, nil)", entity)
	} else {
		g.line("\t_, _, err = Tables.%s.TxInsert(dtx, data, nil)", entity)
	}
	g.line("\tvar row utils.JSON")
	g.line("\tif err == nil {")
	g.line("\t\tqb := Tables.%s.NewTableSelectQueryBuilder()", entity)
	g.line("\t\tqb.AndWithParams(%q, utils.JSON{%q: data[%q]})", uid+" = :rows_key", "rows_key", uid)
	g.line("\t\tqb.AndWithParams(rows%s(caller))", pascal(entity))
	g.line("\t\t_, row, err = Tables.%s.TxSelectOneWithBuilder(dtx, qb)", entity)
	g.line("\t}")
	g.line("\tif err != nil || row == nil {")
	g.line("\t\t_ = dtx.Rollback()")
	g.line("\t\tif err != nil {")
	g.line("\t\t\treturn err")
	g.line("\t\t}")
	g.line("\t\treturn aepr.WriteResponseAndNewErrorf(http.StatusForbidden, \"\", \"%%s\", %q)", strings.ToUpper(snakeName(entity))+"_OUTSIDE_ROWS")
	g.line("\t}")
	g.line("\tif err := dtx.Commit(); err != nil {")
	g.line("\t\treturn err")
	g.line("\t}")
	g.foldRow(entity)
	g.line("\taepr.WriteResponseAsJSON(http.StatusCreated, nil, row)")
	g.line("\treturn nil")
}
