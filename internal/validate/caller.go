package validate

import (
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/metamodel"
	"github.com/SpecArch/specarch/internal/source"
)

// The caller (meta-model 0.3, ADR-095): one declared record of who is
// making the call, whose properties a role's rows rule names as
// caller.<property>. The name caller is reserved wherever an expression's
// names come from, in a file written against 0.3; a 0.2 file keeps the
// name and is read with the version warning (ADR-094).

// metaModelVersion is the version the root file says.
func (d *design) metaModelVersion() string {
	return source.Str(source.Child(d.root, "specarch"))
}

// reads is the version the file is read as: its own, or 1.0.0 for a
// version line this program does not read.
func (d *design) reads() string {
	return metamodel.Design.Reads(d.metaModelVersion())
}

// reservesCaller reports whether the file is written against a version
// that reserves the name caller: 0.3 and later.
func (d *design) reservesCaller() bool {
	return metamodel.AtLeast(d.reads(), 0, 3)
}

// callerEnv is the caller's properties as an expression names them,
// caller.<property>, each typed as a field.
func (d *design) callerEnv() expr.Env {
	env := expr.Env{}
	for _, p := range source.Pairs(source.Child(source.Child(d.root, "caller"), "properties")) {
		env[expr.CallerName+"."+p.Key.Value] = d.fieldType(p.Value)
	}
	return env
}

// checkCaller checks the caller's properties as a value object's, and
// that nothing an expression's names come from is named caller in a file
// that reserves the name.
func (c *checker) checkCaller(d *design) {
	for _, p := range source.Pairs(source.Child(source.Child(d.root, "caller"), "properties")) {
		c.checkField(p.Value, []string{"caller", "properties", p.Key.Value})
	}
	if !d.reservesCaller() {
		return
	}
	reserved := func(n *yaml.Node, ptr, what string) {
		c.add(n, ptr, RuleReservedName, "%s is named caller, which a file on meta-model %s reserves for the caller record, since an expression naming it could mean %s or the caller; rename it", what, d.metaModelVersion(), what)
	}
	for _, e := range source.Pairs(source.Child(d.root, "entities")) {
		if f := source.Child(source.Child(e.Value, "properties"), expr.CallerName); f != nil {
			reserved(source.Key(source.Child(e.Value, "properties"), expr.CallerName), source.Pointer("entities", e.Key.Value, "properties", expr.CallerName), "a field of entity "+e.Key.Value)
		}
	}
	for _, s := range source.Pairs(source.Child(d.root, "schemas")) {
		if f := source.Child(source.Child(s.Value, "properties"), expr.CallerName); f != nil {
			reserved(source.Key(source.Child(s.Value, "properties"), expr.CallerName), source.Pointer("schemas", s.Key.Value, "properties", expr.CallerName), "a property of schema "+s.Key.Value)
		}
	}
	for _, v := range source.Pairs(source.Child(d.root, "views")) {
		if f := source.Child(source.Child(v.Value, "properties"), expr.CallerName); f != nil {
			reserved(source.Key(source.Child(v.Value, "properties"), expr.CallerName), source.Pointer("views", v.Key.Value, "properties", expr.CallerName), "a property of view "+v.Key.Value)
		}
	}
	for _, a := range source.Pairs(source.Child(d.root, "algorithms")) {
		if f := source.Child(source.Child(a.Value, "inputs"), expr.CallerName); f != nil {
			reserved(source.Key(source.Child(a.Value, "inputs"), expr.CallerName), source.Pointer("algorithms", a.Key.Value, "inputs", expr.CallerName), "an input of algorithm "+a.Key.Value)
		}
	}
}

// checkRoleRows checks every role's row policies, under rowPolicies or
// rows, the key's spelling before 1.0.0: the entity exists, the rule
// parses, names only the entity's fields and the caller's properties, and
// gives true or false. A problem's pointer names the key the file gives.
func (c *checker) checkRoleRows(d *design) {
	callerEnv := d.callerEnv()
	for _, r := range source.Pairs(source.Child(d.root, "roles")) {
		key, policies := metamodel.RowPolicies(r.Value)
		for _, row := range source.Pairs(policies) {
			if d.spec != nil && d.spec.Derived[row.Value] {
				continue // checked once, at the entity, by checkTenant
			}
			entity := row.Key.Value
			ptr := source.Pointer("roles", r.Key.Value, key, entity)
			ent := d.entities[entity]
			if ent == nil {
				c.add(row.Key, ptr, RuleRowPolicies, "%s is not an entity of the specification, so the row policy has no rows to limit%s", entity, suggest(entity, d.entities))
				continue
			}
			n := row.Value
			if !source.IsScalar(n) {
				continue // the schema reports it
			}
			env := expr.Env{}
			for name, f := range fieldsOf(ent) {
				env[name] = d.fieldType(f)
			}
			for name, t := range callerEnv {
				env[name] = t
			}
			tree, errs := expr.Parse(n.Value)
			if len(errs) > 0 {
				c.rowsErrors(n, ptr, errs)
				continue
			}
			t, errs := expr.Check(tree, env)
			c.rowsErrors(n, ptr, errs)
			if len(errs) == 0 && (t.Kind != expr.Bool || t.Nullable) {
				c.addFile(c.fileOf(n), exprLine(n, 1), ptr, RuleRowPolicies, "the rule gives %s, but a row policy must give true or false for each record; compare the values with ==, <, > or similar", t)
			}
		}
	}
}

// checkTenant checks the tenant scope (ADR-179): its property is the
// caller's, every shared name is an entity that carries no key, there is
// a role to derive a rule for, every other entity carries the key or is
// limited by a rule of its own on every role, and the key compares with
// the property. A derived rule is checked here, once per entity, rather
// than at every role.
func (c *checker) checkTenant(d *design) {
	tenant := source.Child(d.root, "tenant")
	property := source.Child(tenant, "property")
	key := source.Str(source.Child(tenant, "key"))
	if tenant == nil || !source.IsScalar(property) || key == "" {
		return // the schema reports it
	}
	callerEnv := d.callerEnv()
	ruleOf := expr.CallerName + "." + property.Value
	_, declared := callerEnv[ruleOf]
	if !declared {
		c.add(property, source.Pointer("tenant", "property"), RuleTenant, "%s is not a property of the caller, so the tenant scope has nothing to compare %s with; %s", property.Value, key, callerPropertyList(d.root))
	}
	shared := map[string]bool{}
	for i, n := range source.Items(source.Child(tenant, "shared")) {
		ptr := source.Pointer("tenant", "shared", strconv.Itoa(i))
		ent := d.entities[n.Value]
		switch {
		case ent == nil:
			c.add(n, ptr, RuleTenant, "%s is not an entity of the specification, so there is nothing to share%s", n.Value, suggest(n.Value, d.entities))
		case fieldsOf(ent)[key] != nil:
			c.add(n, ptr, RuleTenant, "%s carries %s, so its rows are a tenant's, and shared says every tenant reads it whole; take it out of shared, or rename its field if it is not the tenant's key", n.Value, key)
		}
		shared[n.Value] = true
	}
	roles := source.Pairs(source.Child(d.root, "roles"))
	if len(roles) == 0 {
		c.add(source.Key(d.root, "tenant"), source.Pointer("tenant"), RuleTenant, "%s", "tenant limits the rows of each role, and the specification has no roles, so no rule is derived and every tenant would reach every record; add the roles that call the system under roles, or take tenant out if the system serves one tenant")
		return
	}
	rule := key + " == " + ruleOf
	for _, e := range source.Pairs(source.Child(d.root, "entities")) {
		if shared[e.Key.Value] {
			continue
		}
		// open are the roles with no rule of their own over the entity:
		// none, or the one derived from the scope.
		var open []string
		for _, r := range roles {
			if own := source.Child(rowPoliciesOf(r.Value), e.Key.Value); own == nil || d.spec != nil && d.spec.Derived[own] {
				open = append(open, r.Key.Value)
			}
		}
		fields := fieldsOf(e.Value)
		if fields[key] == nil {
			if len(open) > 0 {
				c.add(e.Key, source.Pointer("entities", e.Key.Value), RuleTenant, "%s carries no field %s and tenant.shared does not list it, so no row policy limits it for %s and every tenant would reach every record; add %s to %s, list %s under tenant.shared if every tenant reads it whole, or write a row policy for it on every role", e.Key.Value, key, joinAnd(open), key, e.Key.Value, e.Key.Value)
			}
			continue
		}
		if !declared || len(open) == 0 {
			continue // no rule is derived over the entity
		}
		env := expr.Env{}
		for name, f := range fields {
			env[name] = d.fieldType(f)
		}
		for name, t := range callerEnv {
			env[name] = t
		}
		tree, errs := expr.Parse(rule)
		if len(errs) == 0 {
			_, errs = expr.Check(tree, env)
		}
		for _, err := range errs {
			field := source.Key(source.Child(e.Value, "properties"), key)
			c.add(field, source.Pointer("entities", e.Key.Value, "properties", key), RuleTenant, "the tenant rule %s cannot be read over %s: %s", rule, e.Key.Value, err.Message)
		}
	}
}

// rowsErrors reports a row policy's expression errors: a name that is
// neither the entity's nor the caller's is row_policies, a property the
// caller lacks is caller_property, and the rest are the expression rules.
func (c *checker) rowsErrors(n *yaml.Node, ptr string, errs []expr.Error) {
	before := len(c.diags)
	c.exprErrors(n, ptr, "the row policy", errs)
	for i := before; i < len(c.diags); i++ {
		if c.diags[i].Rule == RuleExpressionName {
			c.diags[i].Rule = RuleRowPolicies
		}
	}
}

// rowsOf is every role with a rows rule for the entity, in document order,
// each with its rule.
func (d *design) rowsOf(entity string) []source.Pair {
	var out []source.Pair
	if entity == "" {
		return nil
	}
	for _, r := range source.Pairs(source.Child(d.root, "roles")) {
		if rule := source.Child(rowPoliciesOf(r.Value), entity); rule != nil && source.IsScalar(rule) {
			out = append(out, source.Pair{Key: r.Key, Value: rule})
		}
	}
	return out
}

// requestEntity is the entity an operation's request body is, or "".
func (d *design) requestEntity(op *yaml.Node) string {
	for _, c := range source.Pairs(source.Child(source.Child(op, "requestBody"), "content")) {
		if ref := source.Str(source.Child(source.Child(c.Value, "schema"), "$ref")); strings.HasPrefix(ref, "#/entities/") {
			return strings.TrimPrefix(ref, "#/entities/")
		}
	}
	return ""
}

// keyEntity is the entity a path parameter is the key of: the last path
// parameter named after an entity with Id after it (loanId, loan_id), or
// "".
func (d *design) keyEntity(o operation) string {
	byKey := map[string]string{}
	names := make([]string, 0, len(d.entities))
	for name := range d.entities {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if k := keyName(name + "Id"); byKey[k] == "" {
			byKey[k] = name
		}
	}
	found := ""
	for _, p := range append(source.Items(source.Child(o.pathItem, "parameters")), source.Items(source.Child(o.node, "parameters"))...) {
		if source.Str(source.Child(p, "in")) != "path" {
			continue
		}
		if e := byKey[keyName(source.Str(source.Child(p, "name")))]; e != "" {
			found = e
		}
	}
	return found
}

// keyName is a name with case, underscores and hyphens ignored.
func keyName(s string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(s))
}

// rowsEntity is the entity an operation reads or writes: its answer's,
// its request body's, its guard's, its list's, or the one a path
// parameter is the key of, in that order.
func (d *design) rowsEntity(o operation) string {
	if e := d.responseEntity(o.node); e != "" {
		return e
	}
	if e := d.requestEntity(o.node); e != "" {
		return e
	}
	if e := source.Str(source.Child(source.Child(o.node, "guard"), "entity")); e != "" {
		return e
	}
	if e := source.Str(source.Child(source.Child(o.node, "listOf"), "entity")); e != "" {
		return e
	}
	return d.keyEntity(o)
}

// rowsCases adds the cases of every rows rule over the entity an operation
// reads or writes: a list shows only the rows, a read by key outside them
// is not found, and a change that would leave them is refused (ADR-095).
func (d *design) rowsCases(s *subject, o operation, call string) {
	entity := d.rowsEntity(o)
	perm := source.Str(source.Child(o.node, "permission"))
	for _, r := range d.rowsOf(entity) {
		role, rule := r.Key.Value, r.Value.Value
		// A role that does not hold the operation's permission is refused
		// before any row is read: that is the denied case.
		if !d.roleHolds(d.roles[role])[perm] {
			continue
		}
		given := "a caller holding " + role + ", whose rows of " + entity + " are " + rule
		switch {
		case o.method == "get" && d.responseIsList(o.node):
			s.red("shows only the rows of "+role, frequent, given, call, "it lists only the "+entity+" records the rule holds for")
		case o.method == "get":
			s.red("a record outside the rows of "+role+" is not found", frequent, given, call+" for a "+entity+" the rule does not hold for", "it is refused as not found, as if no record had that key")
		default:
			s.red("a change outside the rows of "+role+" is refused", frequent, given, call+" with a change that would leave the rows", "it is refused and no "+entity+" changes")
		}
	}
}

// jobRowsCases adds the case of a job acting as a role with a rows rule
// over an entity it reads or writes: it acts on those rows only.
func (d *design) jobRowsCases(s *subject, job *yaml.Node, run string) {
	role := source.Str(source.Child(job, "role"))
	if role == "" {
		return
	}
	seen := map[string]bool{}
	for _, key := range []string{"reads", "writes"} {
		for _, e := range source.Items(source.Child(job, key)) {
			entity := e.Value
			if seen[entity] {
				continue
			}
			seen[entity] = true
			if rule := source.Child(rowPoliciesOf(d.roles[role]), entity); rule != nil {
				s.red("acts only on the rows of "+role+" in "+entity, frequent, "the job acts as "+role+", whose rows of "+entity+" are "+rule.Value, run, "it reads and writes only the "+entity+" records the rule holds for")
			}
		}
	}
}

// rowPoliciesOf is a role's row policies, under either spelling.
func rowPoliciesOf(role *yaml.Node) *yaml.Node {
	_, n := metamodel.RowPolicies(role)
	return n
}

// callerProperties lists the caller's properties, sorted.
func callerProperties(root *yaml.Node) []string {
	var out []string
	for _, p := range source.Pairs(source.Child(source.Child(root, "caller"), "properties")) {
		out = append(out, p.Key.Value)
	}
	sort.Strings(out)
	return out
}

// checkFixtureCaller checks a test fixture's caller: a role or public as a
// word, or an object with the role and values for the caller's properties.
func (c *checker) checkFixtureCaller(d *design, n *yaml.Node, ptr string) {
	if source.IsScalar(n) {
		if v := source.Str(n); v != "" && v != "public" && d.roles[v] == nil && !isCross(v) {
			c.add(n, ptr, RuleTestData, "caller %s is neither public nor a role of the specification%s", v, suggest(v, d.roles))
		}
		return
	}
	props := map[string]*yaml.Node{}
	for _, p := range source.Pairs(source.Child(source.Child(d.root, "caller"), "properties")) {
		props[p.Key.Value] = p.Value
	}
	for _, kv := range source.Pairs(n) {
		key := kv.Key.Value
		switch {
		case key == "role":
			if v := source.Str(kv.Value); v != "" && v != "public" && d.roles[v] == nil && !isCross(v) {
				c.add(kv.Value, ptr+"/role", RuleTestData, "caller role %s is neither public nor a role of the specification%s", v, suggest(v, d.roles))
			}
		case strings.HasPrefix(key, "x-"):
		case props[key] == nil:
			c.add(kv.Key, ptr+"/"+key, RuleTestData, "%s is not a property of the caller; %s", key, callerPropertyList(d.root))
		default:
			if _, msg := d.value(kv.Value, props[key]); msg != "" {
				c.add(kv.Value, ptr+"/"+key, RuleTestData, "caller property %s %s", key, msg)
			}
		}
	}
}

func callerPropertyList(root *yaml.Node) string {
	props := callerProperties(root)
	if len(props) == 0 {
		return "the specification declares no caller"
	}
	return "its properties are " + strings.Join(props, ", ")
}
