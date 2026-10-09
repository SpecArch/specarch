package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/source"
)

// The page elements of docs/conventions.md, Page elements: pickers, an
// action's when and reason, a field's conditions, and a form's or a task's
// checks and fields entered twice.

// checkPageElements checks the page elements of every page.
func (c *checker) checkPageElements(d *design) {
	for _, p := range source.Pairs(source.Child(d.root, "pages")) {
		name, pg := p.Key.Value, p.Value
		kind := source.Str(source.Child(pg, "kind"))
		ent := d.entities[source.Str(source.Child(pg, "entity"))]
		shown := map[string]*yaml.Node{}
		for _, f := range pageFields(pg) {
			if fields := fieldsOf(ent); fields[f.Value] != nil {
				shown[f.Value] = fields[f.Value]
			}
		}
		c.checkPickers(d, name, pg, kind, ent, shown)
		c.checkActionElements(d, name, pg, kind, ent)
		c.checkFieldConditions(d, name, pg, kind, ent, shown)
		if kind == "task" {
			shown = d.taskFields(pg)
		}
		c.checkFormChecks(d, name, pg, kind, shown)
	}
}

// taskFields are the properties of the request body a task page sends
// that the page shows, or nil when its submit is missing, names no
// operation or names one that takes no body; the schema, the reference
// check and the task page check report those.
func (d *design) taskFields(pg *yaml.Node) map[string]*yaml.Node {
	op := d.operations[source.Str(source.Child(pg, "submit"))].node
	props, _ := d.requestFields(op)
	if op == nil || props == nil {
		return nil
	}
	shown := map[string]*yaml.Node{}
	for _, f := range pageFields(pg) {
		if props[f.Value] != nil {
			shown[f.Value] = props[f.Value]
		}
	}
	return shown
}

// sendsChecks says whether a page of the kind is sent with checks and
// fields entered twice: a form, or a task.
func sendsChecks(kind string) bool {
	return kind == "form" || kind == "task"
}

// loadsRecord says whether a page has a record before anything is
// entered: a list's rows, a view's record, or the record an edit form
// loads. A form without source creates one.
func loadsRecord(pg *yaml.Node) bool {
	return source.Str(source.Child(pg, "kind")) != "form" || source.Child(pg, "source") != nil
}

// recordEnv is what an expression over a page's record may name: the
// entity's fields, or on a form that creates a record, the fields it shows.
func (d *design) recordEnv(pg *yaml.Node, ent *yaml.Node, shown map[string]*yaml.Node) expr.Env {
	env := expr.Env{}
	fields := fieldsOf(ent)
	if !loadsRecord(pg) {
		fields = shown
	}
	for n, f := range fields {
		env[n] = d.fieldType(f)
	}
	return env
}

// checkCondition checks an expression that decides something on a page:
// it parses, names only what env holds, and gives true or false.
func (c *checker) checkCondition(n *yaml.Node, ptr, what string, env expr.Env) {
	if n == nil || !source.IsScalar(n) {
		return
	}
	tree, errs := expr.Parse(n.Value)
	if len(errs) > 0 {
		c.exprErrors(n, ptr, what, errs)
		return
	}
	t, errs := expr.Check(tree, env)
	c.exprErrors(n, ptr, what, errs)
	if len(errs) == 0 && (t.Kind != expr.Bool || t.Nullable) {
		c.addFile(c.fileOf(n), exprLine(n, 1), ptr, RuleExpressionType, "%s gives %s, but it must give true or false; compare the values with ==, <, > or similar", what, t)
	}
}

// checkSyntax reports an expression that does not parse, for one whose
// names cannot be typed.
func (c *checker) checkSyntax(n *yaml.Node, ptr, what string) {
	if n == nil || !source.IsScalar(n) {
		return
	}
	if _, errs := expr.Parse(n.Value); len(errs) > 0 {
		c.exprErrors(n, ptr, what, errs)
	}
}

// checkMessage reports a check's message that is not a full sentence.
func (c *checker) checkMessage(m *yaml.Node, ptr string) {
	if m != nil && !sentence(m.Value) {
		c.add(m, ptr, RuleFormField, "the message is not a full sentence; start it with a capital and end it with a full stop, so a screen reader reads it as one")
	}
}

// pickerRelation is the many-to-one relation of an entity whose via is
// the field, and its target, or "" when there is none.
func pickerRelation(ent *yaml.Node, field string) (relation, target string) {
	for _, r := range source.Pairs(source.Child(ent, "relations")) {
		if source.Str(source.Child(r.Value, "kind")) == "many-to-one" && source.Str(source.Child(r.Value, "via")) == field {
			return r.Key.Value, source.Str(source.Child(r.Value, "target"))
		}
	}
	return "", ""
}

// sameType says whether two fields hold values of one type, null aside.
func (d *design) sameType(a, b *yaml.Node) bool {
	ta, tb := d.fieldType(a), d.fieldType(b)
	ta.Nullable, tb.Nullable = false, false
	return ta.String() == tb.String()
}

// checkPickers checks a form's pickers: each is for a field the form shows
// that a many-to-one relation holds, its source lists the relation's
// target, it shows and fills from fields of the target, and everyone who
// may open the form may read the list.
func (c *checker) checkPickers(d *design, name string, pg *yaml.Node, kind string, ent *yaml.Node, shown map[string]*yaml.Node) {
	pickers := source.Child(pg, "pickers")
	if pickers == nil {
		return
	}
	base := []string{"pages", name, "pickers"}
	if kind != "form" {
		c.add(source.Key(pg, "pickers"), source.Pointer(base...), RulePicker, "%s is a %s, and a picker is how a form fills a field; leave pickers out", name, kind)
		return
	}
	entName := source.Str(source.Child(pg, "entity"))
	for _, p := range source.Pairs(pickers) {
		field, pk := p.Key.Value, p.Value
		at := append(base, field)
		if shown[field] == nil {
			c.add(p.Key, source.Pointer(at...), RulePicker, "%s is not a field the form %s shows%s", field, name, suggest(field, shown))
			continue
		}
		relation, target := pickerRelation(ent, field)
		if relation == "" {
			c.add(p.Key, source.Pointer(at...), RulePicker, "no many-to-one relation of %s holds %s, so there is no record to pick; add the relation with via: %s, or leave the picker out", entName, field, field)
			continue
		}
		targetEnt := d.entities[target]
		if targetEnt == nil {
			continue // the relation check reports it
		}
		targetFields := fieldsOf(targetEnt)
		srcNode := source.Child(pk, "source")
		src := source.Str(srcNode)
		o, ok := d.operations[src]
		switch {
		case srcNode == nil:
		case !ok || o.node == nil:
			opNames := map[string]*yaml.Node{}
			for id, op := range d.operations {
				opNames[id] = op.node
			}
			c.add(srcNode, source.Pointer(append(at, "source")...), RulePicker, "%s is not an operationId of the specification%s", src, suggest(src, opNames))
		case source.Str(source.Child(source.Child(o.node, "listOf"), "entity")) != target:
			c.add(srcNode, source.Pointer(append(at, "source")...), RulePicker, "%s does not list %s, the target of the relation %s; name an operation whose listOf names %s", src, target, relation, target)
		default:
			c.checkPickerAccess(d, name, pg, srcNode, at, src, source.Str(source.Child(o.node, "permission")))
		}
		for i, f := range source.Items(source.Child(pk, "shows")) {
			if targetFields[f.Value] == nil {
				c.add(f, source.Pointer(append(at, "shows", fmt.Sprint(i))...), RulePicker, "%s is not a field of %s, so the picker cannot show it%s", f.Value, target, suggest(f.Value, targetFields))
			}
		}
		for _, kv := range source.Pairs(source.Child(pk, "fills")) {
			fat := source.Pointer(append(at, "fills", kv.Key.Value)...)
			switch {
			case kv.Key.Value == field || shown[kv.Key.Value] == nil:
				c.add(kv.Key, fat, RulePicker, "%s is not another field the form %s shows, so the picker cannot fill it", kv.Key.Value, name)
			case targetFields[kv.Value.Value] == nil:
				c.add(kv.Value, fat, RulePicker, "%s is not a field of %s, so the picker cannot fill from it%s", kv.Value.Value, target, suggest(kv.Value.Value, targetFields))
			case !d.sameType(shown[kv.Key.Value], targetFields[kv.Value.Value]):
				c.add(kv.Value, fat, RulePicker, "%s is %s and %s.%s is %s; fill a field from one of its own type", kv.Key.Value, d.fieldType(shown[kv.Key.Value]), target, kv.Value.Value, d.fieldType(targetFields[kv.Value.Value]))
			}
		}
	}
}

// checkPickerAccess checks that every role that may open a page may also
// read the list its picker reads.
func (c *checker) checkPickerAccess(d *design, name string, pg, srcNode *yaml.Node, at []string, src, need string) {
	perm := source.Str(source.Child(pg, "permission"))
	if need == "" || need == "public" || need == perm {
		return
	}
	for _, r := range source.Pairs(source.Child(d.root, "roles")) {
		granted := map[string]bool{}
		for _, p := range source.Items(source.Child(r.Value, "permissions")) {
			granted[p.Value] = true
		}
		if (perm == "public" || granted[perm]) && !granted[need] {
			c.add(srcNode, source.Pointer(append(at, "source")...), RulePicker, "%s may open %s but not read %s, which needs %s; grant it to the role, or pick from a list the role may read", r.Key.Value, name, src, need)
		}
	}
}

// checkActionElements checks an action's when and reason: when is over a
// record the page has, and the reason is a string the operation's request
// body requires.
func (c *checker) checkActionElements(d *design, name string, pg *yaml.Node, kind string, ent *yaml.Node) {
	for i, a := range source.Items(source.Child(pg, "actions")) {
		at := []string{"pages", name, "actions", fmt.Sprint(i)}
		label := source.Str(source.Child(a, "label"))
		if w := source.Child(a, "when"); w != nil {
			if !loadsRecord(pg) {
				c.add(source.Key(a, "when"), source.Pointer(append(at, "when")...), RuleAction, "%s creates a record and loads none, so the action %s has no record for when to test; leave when out", name, label)
			} else if ent != nil {
				c.checkCondition(w, source.Pointer(append(at, "when")...), "when", d.recordEnv(pg, ent, nil))
			}
		}
		r := source.Child(a, "reason")
		if r == nil {
			continue
		}
		ptr := source.Pointer(append(at, "reason")...)
		if source.Str(source.Child(a, "kind")) != "operation" {
			c.add(source.Key(a, "reason"), ptr, RuleAction, "the action %s navigates, and a reason is sent with an operation; leave it out", label)
			continue
		}
		o, ok := d.operations[source.Str(source.Child(a, "target"))]
		if !ok || o.node == nil {
			continue // the page check reports it
		}
		fields, required := d.requestFields(o.node)
		f := fields[r.Value]
		switch {
		case f == nil:
			c.add(r, ptr, RuleAction, "%s is not a property of the request body of %s, so the reason cannot be sent%s", r.Value, o.id, suggest(r.Value, fields))
		case !contains(required, r.Value):
			c.add(r, ptr, RuleAction, "the request body of %s does not require %s, and a reason asked for is always sent; add it to the body's required", o.id, r.Value)
		case d.fieldType(f).Kind != expr.String:
			c.add(r, ptr, RuleAction, "%s is %s, and a reason is text a person types; make it a string", r.Value, d.fieldType(f))
		}
	}
}

// checkFieldConditions checks when a page's fields are read-only or
// hidden: each is a field the page shows, read-only only on a form, and
// each condition an expression over the record.
func (c *checker) checkFieldConditions(d *design, name string, pg *yaml.Node, kind string, ent *yaml.Node, shown map[string]*yaml.Node) {
	conds := source.Child(pg, "fieldConditions")
	if conds == nil {
		return
	}
	base := []string{"pages", name, "fieldConditions"}
	if kind != "form" && kind != "view" {
		c.add(source.Key(pg, "fieldConditions"), source.Pointer(base...), RuleFormField, "%s is a %s, and fieldConditions are for the fields of a form or a view; leave them out", name, kind)
		return
	}
	for _, p := range source.Pairs(conds) {
		at := append(base, p.Key.Value)
		if shown[p.Key.Value] == nil {
			c.add(p.Key, source.Pointer(at...), RuleFormField, "%s is not a field %s shows%s", p.Key.Value, name, suggest(p.Key.Value, shown))
			continue
		}
		for _, k := range []string{"readOnly", "readOnlyWhen"} {
			if kind == "view" && source.Child(p.Value, k) != nil {
				c.add(source.Key(p.Value, k), source.Pointer(append(at, k)...), RuleFormField, "%s is a view, where every field is read-only; leave %s out", name, k)
			}
		}
		if kind == "form" && source.Child(p.Value, "readOnly") != nil && source.Child(p.Value, "readOnlyWhen") != nil {
			c.add(source.Key(p.Value, "readOnlyWhen"), source.Pointer(append(at, "readOnlyWhen")...), RuleFormField, "%s is read-only on %s already; leave readOnlyWhen out, or readOnly", p.Key.Value, name)
		}
		if ent == nil {
			continue
		}
		env := d.recordEnv(pg, ent, shown)
		for _, k := range []string{"readOnlyWhen", "hiddenWhen"} {
			c.checkCondition(source.Child(p.Value, k), source.Pointer(append(at, k)...), k, env)
		}
	}
}

// checkFormChecks checks a form's or a task's checks across its fields
// and its fields entered twice: only a form or a task has them, each names
// fields it shows, and each message is a full sentence. shown is nil for a
// task whose fields are not known, as its submit has no request body: then
// each expression is still parsed and each message read, and what names a
// field is left to the page's own error.
func (c *checker) checkFormChecks(d *design, name string, pg *yaml.Node, kind string, shown map[string]*yaml.Node) {
	for _, key := range []string{"checks", "enteredTwice"} {
		if source.Child(pg, key) != nil && !sendsChecks(kind) {
			c.add(source.Key(pg, key), source.Pointer("pages", name, key), RuleFormField, "%s is a %s, and %s are checked when a form or a task is sent; leave them out", name, kind, key)
		}
	}
	if !sendsChecks(kind) {
		return
	}
	if shown == nil {
		for _, p := range source.Pairs(source.Child(pg, "checks")) {
			at := []string{"pages", name, "checks", p.Key.Value}
			c.checkSyntax(source.Child(p.Value, "expression"), source.Pointer(append(at, "expression")...), "the check")
			c.checkMessage(source.Child(p.Value, "message"), source.Pointer(append(at, "message")...))
		}
		return
	}
	env := expr.Env{}
	for n, f := range shown {
		env[n] = d.fieldType(f)
	}
	for _, p := range source.Pairs(source.Child(pg, "checks")) {
		at := []string{"pages", name, "checks", p.Key.Value}
		c.checkCondition(source.Child(p.Value, "expression"), source.Pointer(append(at, "expression")...), "the check", env)
		c.checkMessage(source.Child(p.Value, "message"), source.Pointer(append(at, "message")...))
		if f := source.Child(p.Value, "field"); f != nil && shown[f.Value] == nil {
			c.add(f, source.Pointer(append(at, "field")...), RuleFormField, "%s is not a field the %s %s shows%s", f.Value, kind, name, suggest(f.Value, shown))
		}
	}
	for i, f := range source.Items(source.Child(pg, "enteredTwice")) {
		if shown[f.Value] == nil {
			c.add(f, source.Pointer("pages", name, "enteredTwice", fmt.Sprint(i)), RuleFormField, "%s is not a field the %s %s shows, so it cannot be entered twice%s", f.Value, kind, name, suggest(f.Value, shown))
		}
	}
}

// elementCases are the cases of a page's elements: a picker that finds
// nothing, an action its when withholds, a confirmation without its
// reason, a check across fields broken in each way it can be, and a field
// entered twice differently.
func (d *design) elementCases(s *subject, pg *yaml.Node, open string) {
	ent := d.entities[source.Str(source.Child(pg, "entity"))]
	// Only a form has pickers; on another page they are errors, and give
	// no case.
	var pickers *yaml.Node
	if source.Str(source.Child(pg, "kind")) == "form" {
		pickers = source.Child(pg, "pickers")
	}
	for _, p := range source.Pairs(pickers) {
		_, target := pickerRelation(ent, p.Key.Value)
		if target == "" {
			continue
		}
		s.red("picker "+p.Key.Value+" finds nothing", occasional, "no "+target+" matches what is typed", "a "+target+" is looked for to fill "+p.Key.Value, "it says nothing matches, and "+p.Key.Value+" stays empty")
	}
	for _, a := range source.Items(source.Child(pg, "actions")) {
		label := source.Str(source.Child(a, "label"))
		if w := source.Str(source.Child(a, "when")); w != "" && loadsRecord(pg) {
			s.red(label+" not offered", occasional, "a record for which "+w+" is false", open, "it does not offer "+label+" for that record")
		}
		if r := source.Str(source.Child(a, "reason")); r != "" && source.Str(source.Child(a, "kind")) == "operation" {
			s.red(label+" without a reason", frequent, "...", "the action "+label+" is confirmed with no "+r, "it is not sent, and the "+r+" is asked for")
		}
	}
	checkCases(s, pg)
}

// checkCases are the cases of a form's or a task's checks: a check across
// fields broken in each way it can be, and a field entered twice
// differently. On another page they are errors, and give no case.
func checkCases(s *subject, pg *yaml.Node) {
	kind := source.Str(source.Child(pg, "kind"))
	if !sendsChecks(kind) {
		return
	}
	sent := "the form is submitted"
	if kind == "task" {
		sent = "the page is submitted"
	}
	for _, p := range source.Pairs(source.Child(pg, "checks")) {
		name, msg := p.Key.Value, source.Str(source.Child(p.Value, "message"))
		rules := falsifiers(source.Str(source.Child(p.Value, "expression")))
		if len(rules) < 2 {
			s.red("violates "+name, frequent, "...", sent+" breaking it", "it is not sent: "+msg)
			continue
		}
		for _, rule := range rules {
			falsehood := strings.Join(rule, " is false and ") + " is false"
			s.red("violates "+name+": "+falsehood, frequent, "...", sent+" with "+falsehood, "it is not sent: "+msg)
		}
	}
	for _, f := range source.Items(source.Child(pg, "enteredTwice")) {
		s.red(f.Value+" entered twice differently", frequent, "...", sent+" with two different entries of "+f.Value, "it is not sent, and says the two entries differ")
	}
}
