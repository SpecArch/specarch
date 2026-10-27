package validate

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/source"
)

// Workflows: requests that finish after people approve them, a sequential
// subset of BPMN 2.0 (docs/meta-model-0.2.md, Workflows).

// checkWorkflows checks that a workflow starts from an operation that
// answers 202, holds its request in an entity, and that its steps name
// roles that grant the approval's permission, operations that exist and
// later approvals to escalate to, each step under its own name.
func (c *checker) checkWorkflows(d *design) {
	opNames := map[string]*yaml.Node{}
	for id, o := range d.operations {
		opNames[id] = o.node
	}
	for _, w := range source.Pairs(source.Child(d.root, "workflows")) {
		name := w.Key.Value
		triggerPerm := ""
		if t := source.Child(w.Value, "trigger"); t != nil {
			o, ok := d.operations[t.Value]
			switch {
			case !ok:
				c.add(t, source.Pointer("workflows", name, "trigger"), RuleWorkflow, "%s is not an operationId of the specification%s", t.Value, suggest(t.Value, opNames))
			case source.Child(o.node, "responses") != nil && source.Child(source.Child(o.node, "responses"), "202") == nil:
				// With no responses at all, the operation's answers are not
				// known yet, which the missing key itself reports.
				c.add(t, source.Pointer("workflows", name, "trigger"), RuleWorkflow, "operation %s does not answer 202; a workflow's trigger accepts the request and answers 202 while it waits for approval", t.Value)
			default:
				triggerPerm = source.Str(source.Child(o.node, "permission"))
			}
		}
		if s := source.Child(w.Value, "subject"); s != nil && d.entities[s.Value] == nil {
			c.add(s, source.Pointer("workflows", name, "subject"), RuleWorkflow, "%s is not an entity of the specification%s", s.Value, suggest(s.Value, d.entities))
		}
		steps := source.Items(source.Child(w.Value, "steps"))
		env := d.subjectEnv(w.Value)
		seen := map[string]bool{}
		for i, st := range steps {
			at := []string{"workflows", name, "steps", fmt.Sprint(i)}
			stepName := source.Child(st, "name")
			if stepName != nil {
				if seen[stepName.Value] {
					c.add(stepName, source.Pointer(append(at, "name")...), RuleWorkflow, "%s names two steps of workflow %s; give each step its own name", stepName.Value, name)
				}
				seen[stepName.Value] = true
			}
			if op := source.Child(st, "operation"); op != nil && d.operations[op.Value].node == nil {
				c.add(op, source.Pointer(append(at, "operation")...), RuleWorkflow, "%s is not an operationId of the specification%s", op.Value, suggest(op.Value, opNames))
			}
			later := steps[i+1:]
			switch source.Str(source.Child(st, "kind")) {
			case "approval":
				c.checkTimeout(source.Child(st, "deadline"), source.Pointer(append(at, "deadline")...), RuleWorkflow, "the approval")
				perm := source.Child(st, "permission")
				if perm != nil && triggerPerm != "" && perm.Value == triggerPerm {
					c.add(perm, source.Pointer(append(at, "permission")...), RuleWorkflow, "%s is the permission of the trigger too, so whoever may ask may also approve; give the approval a permission of its own", perm.Value)
				}
				if perm != nil && triggerPerm != "" && perm.Value != triggerPerm {
					c.checkWorkflowSeparation(d, perm, source.Pointer(append(at, "permission")...), triggerPerm)
				}
				c.checkStepRoles(d, st, at, "approvers", "the approval", "approver")
				if to := source.Child(st, "escalateTo"); to != nil && !laterStep(later, "approval", to.Value) {
					c.add(to, source.Pointer(append(at, "escalateTo")...), RuleWorkflow, "%s is not an approval step after %s; escalate to a later approval of the workflow", to.Value, source.Str(stepName))
				}
			case "task":
				c.checkTimeout(source.Child(st, "deadline"), source.Pointer(append(at, "deadline")...), RuleWorkflow, "the task")
				c.checkStepRoles(d, st, at, "performers", "the task", "performer")
				if to := source.Child(st, "escalateTo"); to != nil && !laterStep(later, "task", to.Value) {
					c.add(to, source.Pointer(append(at, "escalateTo")...), RuleWorkflow, "%s is not a task step after %s; escalate to a later task of the workflow", to.Value, source.Str(stepName))
				}
			case "operation":
				if comp := source.Child(st, "compensatedBy"); comp != nil && d.operations[comp.Value].node == nil {
					c.add(comp, source.Pointer(append(at, "compensatedBy")...), RuleWorkflow, "%s is not an operationId of the specification%s", comp.Value, suggest(comp.Value, opNames))
				}
				c.checkOneOf(source.Child(st, "onFailure"), append(append([]string{}, at...), "onFailure"), "a failure goes on at a later step or ends the request", "to", "end")
				if to := source.Child(source.Child(st, "onFailure"), "to"); to != nil && !laterStep(later, "", to.Value) {
					c.add(to, source.Pointer(append(at, "onFailure", "to")...), RuleWorkflow, "%s is not a step after %s; a failure goes on at a later step of the workflow, or ends it with end: failed", to.Value, source.Str(stepName))
				}
			case "decision":
				branches := source.Items(source.Child(st, "branches"))
				for j, br := range branches {
					bat := append(append([]string{}, at...), "branches", fmt.Sprint(j))
					c.checkOneOf(br, bat, "a branch has a condition or is taken otherwise", "when", "otherwise")
					c.checkOneOf(br, bat, "a branch goes on at a later step or ends the request", "to", "end")
					if env != nil {
						c.checkCondition(source.Child(br, "when"), source.Pointer(append(bat, "when")...), "the branch's condition", env)
					}
					if o := source.Child(br, "otherwise"); o != nil && j != len(branches)-1 {
						c.add(o, source.Pointer(append(bat, "otherwise")...), RuleWorkflow, "otherwise is on branch %d of %d of %s; only the last branch is taken when none holds, so move it to the end", j+1, len(branches), source.Str(stepName))
					}
					if j == len(branches)-1 && source.Child(br, "otherwise") == nil && source.Child(br, "when") != nil {
						c.add(source.Child(br, "when"), source.Pointer(append(bat, "when")...), RuleWorkflow, "the last branch of %s has a condition, so a request none of them holds for has nowhere to go; write otherwise: true in its place, or add a branch with otherwise", source.Str(stepName))
					}
					if to := source.Child(br, "to"); to != nil && !laterStep(later, "", to.Value) {
						c.add(to, source.Pointer(append(bat, "to")...), RuleWorkflow, "%s is not a step after %s; a branch goes on at a later step of the workflow, or ends it with end", to.Value, source.Str(stepName))
					}
				}
			case "wait":
				c.checkOneOf(st, at, "a wait waits for one thing", "message", "after", "at")
				c.checkTimeout(source.Child(st, "deadline"), source.Pointer(append(at, "deadline")...), RuleWorkflow, "the wait")
				if m := source.Child(st, "message"); m != nil && d.message(m.Value) == nil {
					c.add(m, source.Pointer(append(at, "message")...), RuleWorkflow, "%s does not name a channel and one of its messages; write channel/Message for a message declared under channels", m.Value)
				}
				if moment := source.Child(st, "at"); moment != nil && source.IsScalar(moment) && env != nil {
					required := itemValues(source.Child(d.entities[source.Str(source.Child(w.Value, "subject"))], "required"))
					c.checkMoment(moment, source.Pointer(append(at, "at")...), env, required)
				}
			}
		}
	}
}

// checkOneOf reports, at the second, two keys of which an entry gives one:
// the schema reports none of them given, as the one key missing.
func (c *checker) checkOneOf(n *yaml.Node, at []string, rule string, keys ...string) {
	var given []string
	for _, k := range keys {
		if source.Child(n, k) != nil {
			given = append(given, k)
		}
	}
	if len(given) < 2 {
		return
	}
	second := given[1]
	c.add(source.Child(n, second), source.Pointer(append(append([]string{}, at...), second)...), RuleWorkflow, "%s gives both %s and %s, and %s; keep one of %s", source.Str(source.Child(n, "name"))+orEntry(n), given[0], second, rule, strings.Join(keys, ", "))
}

// orEntry names an entry with no name of its own.
func orEntry(n *yaml.Node) string {
	if source.Child(n, "name") != nil {
		return ""
	}
	return "the entry"
}

// checkStepRoles checks that each role an approval or a task names is a
// role and grants the permission the step checks.
func (c *checker) checkStepRoles(d *design, st *yaml.Node, at []string, key, what, who string) {
	perm := source.Child(st, "permission")
	for j, r := range source.Items(source.Child(st, key)) {
		ptr := source.Pointer(append(append([]string{}, at...), key, fmt.Sprint(j))...)
		role := d.roles[r.Value]
		switch {
		case role == nil:
			c.add(r, ptr, RuleWorkflow, "%s is not a role of the specification%s", r.Value, suggest(r.Value, d.roles))
		case perm != nil && d.permissions[perm.Value] != nil && !d.roleHolds(role)[perm.Value]:
			c.add(r, ptr, RuleWorkflow, "%s does not grant %s, which %s checks; grant it to the role, or name another %s", r.Value, perm.Value, what, who)
		}
	}
}

// subjectEnv is the names a workflow's conditions and moments read: the
// fields of its subject, or nil when the subject is not an entity, which
// the subject's own check reports.
func (d *design) subjectEnv(w *yaml.Node) expr.Env {
	ent := d.entities[source.Str(source.Child(w, "subject"))]
	if ent == nil {
		return nil
	}
	env := expr.Env{}
	for name, f := range fieldsOf(ent) {
		env[name] = d.fieldType(f)
	}
	return env
}

// checkMoment checks a wait's at: it gives an instant, or a date read in
// the specification's timeZone, which must then be declared, and never
// null, since a wait with no moment would never end (ADR-193): a field
// the subject does not require may be null as much as one whose type
// says so.
func (c *checker) checkMoment(n *yaml.Node, ptr string, env expr.Env, required []string) {
	what := "the moment"
	tree, errs := expr.Parse(n.Value)
	if len(errs) > 0 {
		c.exprErrors(n, ptr, what, errs)
		return
	}
	t, errs := expr.Check(tree, env)
	c.exprErrors(n, ptr, what, errs)
	if len(errs) > 0 {
		return
	}
	switch {
	case t.Kind != expr.Timestamp && t.Kind != expr.Date:
		c.addFile(c.fileOf(n), exprLine(n, 1), ptr, RuleExpressionType, "%s gives %s, but a wait's at must give an instant or a date; name a date-time or date field of the subject", what, t)
	case t.Nullable || tree.Op == expr.OpName && !slices.Contains(required, tree.Text):
		c.addFile(c.fileOf(n), exprLine(n, 1), ptr, RuleExpressionType, "%s gives %s, which may be null, and a request waiting for no moment never goes on; make the field required, or wait for a field that always holds one", what, t)
	case t.Kind == expr.Date && !c.zoneDeclared:
		c.addFile(c.fileOf(n), exprLine(n, 1), ptr, RuleTimeZone, "%s gives a date, which is read as the first instant of that day in the specification's timeZone, and the specification declares none; declare timeZone at the root, a name of the IANA Time Zone Database such as Asia/Jakarta", what)
	default:
		c.zoneWarnings(n, ptr, what, tree, env)
	}
}

// checkWorkflowSeparation reports each role that grants both the trigger's
// permission and an approval's when a separation-of-duties set holds the
// pair, whatever the set's cardinality: such a role could make a request
// and approve it in one person.
func (c *checker) checkWorkflowSeparation(d *design, perm *yaml.Node, ptr, triggerPerm string) {
	for _, set := range source.Pairs(source.Child(d.root, "separationOfDuties")) {
		var names []string
		for _, p := range source.Items(source.Child(set.Value, "permissions")) {
			names = append(names, p.Value)
		}
		if !slices.Contains(names, triggerPerm) || !slices.Contains(names, perm.Value) {
			continue
		}
		for _, role := range source.Pairs(source.Child(d.root, "roles")) {
			if held := d.roleHolds(role.Value); held[triggerPerm] && held[perm.Value] {
				c.add(perm, ptr, RuleWorkflow, "role %s grants both %s, the trigger's permission, and %s, and set %s keeps them apart; take one of them from the role", role.Key.Value, triggerPerm, perm.Value, set.Key.Value)
			}
		}
	}
}

// laterStep reports whether one of the steps has that name and, unless
// kind is "", that kind.
func laterStep(steps []*yaml.Node, kind, name string) bool {
	return slices.ContainsFunc(steps, func(st *yaml.Node) bool {
		return (kind == "" || source.Str(source.Child(st, "kind")) == kind) && source.Str(source.Child(st, "name")) == name
	})
}

// workflowSubject is a workflow's tests: the approved path, a refusal and
// the deadline at each approval, the requester approving their own
// request, and an approval by someone without its permission.
func (d *design) workflowSubject(p source.Pair) *subject {
	name := p.Key.Value
	s := &subject{kind: "workflow", label: "workflow " + name, node: p.Key, path: source.Pointer("workflows", name),
		yamlKey: "workflow: " + name, name: name}
	trigger := source.Str(source.Child(p.Value, "trigger"))
	o := d.operations[trigger]
	var approvals, operations, perms []string
	for _, st := range source.Items(source.Child(p.Value, "steps")) {
		switch source.Str(source.Child(st, "kind")) {
		case "approval":
			approvals = append(approvals, source.Str(source.Child(st, "name")))
			if perm := stepPermission(st); !slices.Contains(perms, perm) {
				perms = append(perms, perm)
			}
		case "operation":
			op := source.Str(source.Child(st, "operation"))
			if op == "" {
				op = "the operation of " + source.Str(source.Child(st, "name"))
			}
			operations = append(operations, op)
		}
	}
	through := trigger
	if through == "" {
		through = "its trigger"
	}
	when := "a request is made through " + through
	if len(approvals) > 0 {
		when += " and is approved at " + joinAnd(approvals)
	}
	then := "it answers 202, and the request ends approved"
	if len(operations) > 0 {
		then = "it answers 202, then " + joinAnd(operations) + " is called, and the request ends approved"
	}
	s.success = golden(caller(source.Str(source.Child(o.node, "permission"))), when, then)
	stopped := "the request ends refused"
	if len(operations) > 0 {
		stopped += " and " + joinAnd(operations) + " is not called"
	}
	steps := source.Items(source.Child(p.Value, "steps"))
	for i, st := range steps {
		step := source.Str(source.Child(st, "name"))
		waiting := "a request waiting at " + step
		ended := func(how string) string {
			return "the request ends " + how + undone(steps[:i])
		}
		switch source.Str(source.Child(st, "kind")) {
		case "approval":
			perm := stepPermission(st)
			s.red("refused at "+step, frequent, waiting, "a caller with "+perm+" refuses it", stopped+undone(steps[:i]))
			passed := deadlinePassed(st, stopped+undone(steps[:i]))
			deadline := source.Str(source.Child(st, "deadline"))
			if deadline == "" {
				deadline = "the deadline"
			}
			s.byNature("deadline passes at "+step, waiting, deadline+" passes with no answer", passed)
			s.red("approval without "+perm, frequent, "a caller without "+perm, "they approve a request waiting at "+step, "it is refused as not allowed, and the request still waits")
		case "task":
			perm := stepPermission(st)
			if deadline := source.Str(source.Child(st, "deadline")); deadline != "" {
				s.byNature("deadline passes at "+step, waiting, deadline+" passes with the task not done", deadlinePassed(st, ended("refused")))
			}
			s.red("task without "+perm, frequent, "a caller without "+perm, "they do the task of a request waiting at "+step, "it is refused as not allowed, and the request still waits")
		case "wait":
			if deadline := source.Str(source.Child(st, "deadline")); deadline != "" {
				s.byNature("deadline passes at "+step, waiting+" for "+messageOf(st), deadline+" passes with no message", deadlinePassed(st, ended("refused")))
			}
		case "decision":
			branches := source.Items(source.Child(st, "branches"))
			for j, br := range branches {
				target := source.Str(source.Child(br, "to"))
				name, then := step+" goes on at "+target, "the request goes on at "+target
				if target == "" {
					// a question holds where the branch goes open
					name, then = fmt.Sprintf("%s takes branch %d", step, j+1), fmt.Sprintf("the request goes where branch %d of %s goes", j+1, step)
				}
				if end := source.Str(source.Child(br, "end")); end != "" {
					name, then = step+" ends "+end, ended(end)
				}
				given := "a request reaching " + step + " for which " + source.Str(source.Child(br, "when")) + " holds"
				if source.Child(br, "otherwise") != nil {
					given = "a request reaching " + step + " for which no other branch holds"
				}
				s.red(name, frequent, given, "the decision is taken", then)
			}
		case "operation":
			f := source.Child(st, "onFailure")
			if f == nil {
				continue
			}
			op := source.Str(source.Child(st, "operation"))
			if op == "" {
				op = "the operation of " + step
			}
			when := op + " fails"
			if limit, ok := source.Int(source.Child(source.Child(f, "retries"), "limit")); ok {
				when += fmt.Sprintf(", and on each of its %d retries", limit)
				if limit == 1 {
					when = op + " fails, and on its one retry"
				}
			}
			then := ended("failed")
			if to := source.Str(source.Child(f, "to")); to != "" {
				then = "the request goes on at " + to
			}
			s.byNature("operation fails at "+step, "a request reaching "+step, when, then)
		}
	}
	if len(approvals) > 0 {
		s.byNature("requester approves own request", "a request made by a caller who also holds "+joinAnd(perms), "the requester approves it",
			"it is refused, because the person who made a request never approves it, and the request still waits")
	}
	return s
}

// messageOf is the message a wait waits for, or what names it while a
// question holds it open.
func messageOf(st *yaml.Node) string {
	if m := source.Str(source.Child(st, "message")); m != "" {
		return m
	}
	return "the message of " + source.Str(source.Child(st, "name"))
}

// deadlinePassed is what follows when a step's deadline passes: refused
// when onDeadline says refuse, the escalation when it says escalate, and,
// while a question holds onDeadline open, where it goes.
func deadlinePassed(st *yaml.Node, refused string) string {
	switch source.Str(source.Child(st, "onDeadline")) {
	case "escalate":
		return "the request moves on to " + source.Str(source.Child(st, "escalateTo"))
	case "":
		return "the request goes where onDeadline of " + source.Str(source.Child(st, "name")) + " goes"
	}
	return refused
}

// undone is what a request that ends after the steps done says of their
// compensation: each operation step with compensatedBy, the latest first.
// Compensation undoes only a step that ran, so a step an earlier branch,
// failure or escalation can jump over is undone "if it ran".
func undone(done []*yaml.Node) string {
	var calls []string
	for i := len(done) - 1; i >= 0; i-- {
		st := done[i]
		if comp := source.Str(source.Child(st, "compensatedBy")); comp != "" && source.Str(source.Child(st, "kind")) == "operation" {
			name := source.Str(source.Child(st, "name"))
			call := comp + " undoes " + name
			if skippable(done, i) {
				call += " if " + name + " ran"
			}
			calls = append(calls, call)
		}
	}
	if len(calls) == 0 {
		return ""
	}
	return ", and " + strings.Join(calls, ", then ")
}

// skippable says whether a step before done[j] jumps past it: a decision's
// branch, an operation's onFailure or an escalation to a step after it.
func skippable(done []*yaml.Node, j int) bool {
	for k := 0; k < j; k++ {
		st := done[k]
		targets := []string{source.Str(source.Child(st, "escalateTo")), source.Str(source.Child(source.Child(st, "onFailure"), "to"))}
		for _, br := range source.Items(source.Child(st, "branches")) {
			targets = append(targets, source.Str(source.Child(br, "to")))
		}
		for _, t := range targets {
			if t == "" {
				continue
			}
			at := slices.IndexFunc(done, func(n *yaml.Node) bool { return source.Str(source.Child(n, "name")) == t })
			if at < 0 || at > j {
				return true
			}
		}
	}
	return false
}

// stepPermission is the permission an approval checks, or what names it
// while a question holds it open.
func stepPermission(st *yaml.Node) string {
	if perm := source.Str(source.Child(st, "permission")); perm != "" {
		return perm
	}
	return "the permission of " + source.Str(source.Child(st, "name"))
}

// workflowReason says why a chosen case of a workflow is written whatever
// the harm, or "".
func workflowReason(name string) string {
	switch {
	case strings.HasPrefix(name, "deadline passes at "):
		return "a deadline passing is a case nobody exercises by hand, so it is always written"
	case name == "requester approves own request":
		return "the four-eyes rule holds on every approval, so its case is always written"
	case strings.HasPrefix(name, "operation fails at "):
		return "a failure of an operation a workflow calls is a case nobody exercises by hand, so it is always written"
	}
	return ""
}
