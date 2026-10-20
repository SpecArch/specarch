package validate

import (
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
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

// reservesCaller reports whether the file is written against a version
// that reserves the name caller: 0.3 and later.
func (d *design) reservesCaller() bool {
	v := d.metaModelVersion()
	return v != "0.1" && v != "0.2"
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

// checkRows checks every role's rows rules: the entity exists, the rule
// parses, names only the entity's fields and the caller's properties, and
// gives true or false.
func (c *checker) checkRoleRows(d *design) {
	callerEnv := d.callerEnv()
	for _, r := range source.Pairs(source.Child(d.root, "roles")) {
		for _, row := range source.Pairs(source.Child(r.Value, "rows")) {
			entity := row.Key.Value
			ptr := source.Pointer("roles", r.Key.Value, "rows", entity)
			ent := d.entities[entity]
			if ent == nil {
				c.add(row.Key, ptr, RuleRows, "%s is not an entity of the specification, so the rule has no rows to limit%s", entity, suggest(entity, d.entities))
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
				c.addFile(c.fileOf(n), exprLine(n, 1), ptr, RuleRows, "the rule gives %s, but a rows rule must give true or false for each record; compare the values with ==, <, > or similar", t)
			}
		}
	}
}

// rowsErrors reports a rows rule's expression errors: a name that is
// neither the entity's nor the caller's is the rows rule, a property the
// caller lacks is caller_property, and the rest are the expression rules.
func (c *checker) rowsErrors(n *yaml.Node, ptr string, errs []expr.Error) {
	before := len(c.diags)
	c.exprErrors(n, ptr, "the rows rule", errs)
	for i := before; i < len(c.diags); i++ {
		if c.diags[i].Rule == RuleExpressionName {
			c.diags[i].Rule = RuleRows
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
		if rule := source.Child(source.Child(r.Value, "rows"), entity); rule != nil && source.IsScalar(rule) {
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

// rowsCases adds the cases of every rows rule over the entity an operation
// reads or writes: a list shows only the rows, a read by key outside them
// is not found, and a change that would leave them is refused (ADR-095).
func (d *design) rowsCases(s *subject, o operation, call string) {
	entity := d.responseEntity(o.node)
	if entity == "" {
		entity = d.requestEntity(o.node)
	}
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
			if rule := source.Child(source.Child(d.roles[role], "rows"), entity); rule != nil {
				s.red("acts only on the rows of "+role+" in "+entity, frequent, "the job acts as "+role+", whose rows of "+entity+" are "+rule.Value, run, "it reads and writes only the "+entity+" records the rule holds for")
			}
		}
	}
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
		if v := source.Str(n); v != "" && v != "public" && d.roles[v] == nil {
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
			if v := source.Str(kv.Value); v != "" && v != "public" && d.roles[v] == nil {
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
