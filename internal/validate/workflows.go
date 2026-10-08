package validate

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

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
			case source.Child(source.Child(o.node, "responses"), "202") == nil:
				c.add(t, source.Pointer("workflows", name, "trigger"), RuleWorkflow, "operation %s does not answer 202; a workflow's trigger accepts the request and answers 202 while it waits for approval", t.Value)
			default:
				triggerPerm = source.Str(source.Child(o.node, "permission"))
			}
		}
		if s := source.Child(w.Value, "subject"); s != nil && d.entities[s.Value] == nil {
			c.add(s, source.Pointer("workflows", name, "subject"), RuleWorkflow, "%s is not an entity of the specification%s", s.Value, suggest(s.Value, d.entities))
		}
		steps := source.Items(source.Child(w.Value, "steps"))
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
			if source.Str(source.Child(st, "kind")) != "approval" {
				continue
			}
			c.checkTimeout(source.Child(st, "deadline"), source.Pointer(append(at, "deadline")...), RuleWorkflow, "the approval")
			perm := source.Child(st, "permission")
			if perm != nil && triggerPerm != "" && perm.Value == triggerPerm {
				c.add(perm, source.Pointer(append(at, "permission")...), RuleWorkflow, "%s is the permission of the trigger too, so whoever may ask may also approve; give the approval a permission of its own", perm.Value)
			}
			if perm != nil && triggerPerm != "" && perm.Value != triggerPerm {
				c.checkWorkflowSeparation(d, perm, source.Pointer(append(at, "permission")...), triggerPerm)
			}
			for j, r := range source.Items(source.Child(st, "approvers")) {
				ptr := source.Pointer(append(at, "approvers", fmt.Sprint(j))...)
				role := d.roles[r.Value]
				switch {
				case role == nil:
					c.add(r, ptr, RuleWorkflow, "%s is not a role of the specification%s", r.Value, suggest(r.Value, d.roles))
				case perm != nil && d.permissions[perm.Value] != nil && !grants(role, perm.Value):
					c.add(r, ptr, RuleWorkflow, "%s does not grant %s, which the approval checks; grant it to the role, or name another approver", r.Value, perm.Value)
				}
			}
			if to := source.Child(st, "escalateTo"); to != nil && !laterApproval(steps[i+1:], to.Value) {
				c.add(to, source.Pointer(append(at, "escalateTo")...), RuleWorkflow, "%s is not an approval step after %s; escalate to a later approval of the workflow", to.Value, source.Str(stepName))
			}
		}
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
			if grants(role.Value, triggerPerm) && grants(role.Value, perm.Value) {
				c.add(perm, ptr, RuleWorkflow, "role %s grants both %s, the trigger's permission, and %s, and set %s keeps them apart; take one of them from the role", role.Key.Value, triggerPerm, perm.Value, set.Key.Value)
			}
		}
	}
}

// grants reports whether a role grants a permission.
func grants(role *yaml.Node, perm string) bool {
	for _, p := range source.Items(source.Child(role, "permissions")) {
		if p.Value == perm {
			return true
		}
	}
	return false
}

// laterApproval reports whether one of the steps is an approval of that
// name.
func laterApproval(steps []*yaml.Node, name string) bool {
	return slices.ContainsFunc(steps, func(st *yaml.Node) bool {
		return source.Str(source.Child(st, "kind")) == "approval" && source.Str(source.Child(st, "name")) == name
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
			if perm := source.Str(source.Child(st, "permission")); !slices.Contains(perms, perm) {
				perms = append(perms, perm)
			}
		case "operation":
			operations = append(operations, source.Str(source.Child(st, "operation")))
		}
	}
	when := "a request is made through " + trigger
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
	for _, st := range source.Items(source.Child(p.Value, "steps")) {
		if source.Str(source.Child(st, "kind")) != "approval" {
			continue
		}
		step := source.Str(source.Child(st, "name"))
		perm := source.Str(source.Child(st, "permission"))
		waiting := "a request waiting at " + step
		s.red("refused at "+step, frequent, waiting, "a caller with "+perm+" refuses it", stopped)
		passed := stopped
		if source.Str(source.Child(st, "onDeadline")) == "escalate" {
			passed = "the request moves on to " + source.Str(source.Child(st, "escalateTo"))
		}
		s.byNature("deadline passes at "+step, waiting, source.Str(source.Child(st, "deadline"))+" passes with no answer", passed)
		s.red("approval without "+perm, frequent, "a caller without "+perm, "they approve a request waiting at "+step, "it is refused as not allowed, and the request still waits")
	}
	if len(approvals) > 0 {
		s.byNature("requester approves own request", "a request made by a caller who also holds "+joinAnd(perms), "the requester approves it",
			"it is refused, because the person who made a request never approves it, and the request still waits")
	}
	return s
}

// workflowReason says why a chosen case of a workflow is written whatever
// the harm, or "".
func workflowReason(name string) string {
	switch {
	case strings.HasPrefix(name, "deadline passes at "):
		return "a deadline passing is a case nobody exercises by hand, so it is always written"
	case name == "requester approves own request":
		return "the four-eyes rule holds on every approval, so its case is always written"
	}
	return ""
}
