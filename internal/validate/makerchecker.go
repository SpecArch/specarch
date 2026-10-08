package validate

import (
	"fmt"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// Maker-checker on a page (docs/meta-model-0.2.md): the page that makes a
// request a workflow approves, the inbox of an approval step, and the
// messages a workflow publishes when it ends.

// workflowTriggers maps each operation that starts a workflow to the
// first workflow it starts.
func (d *design) workflowTriggers() map[string]string {
	triggers := map[string]string{}
	for _, w := range source.Pairs(source.Child(d.root, "workflows")) {
		if t := source.Str(source.Child(w.Value, "trigger")); t != "" && triggers[t] == "" {
			triggers[t] = w.Key.Value
		}
	}
	return triggers
}

// pendingEvent is the event a page that submits to a workflow's trigger
// raises on the 202 answer: a form's one onSubmitted, a task's under 202.
func pendingEvent(pg *yaml.Node) *yaml.Node {
	ev := source.Child(pg, "onSubmitted")
	if source.Str(source.Child(pg, "kind")) == "task" {
		return source.Child(ev, "202")
	}
	return ev
}

// checkMakerChecker checks that a form or task page submitting to a
// workflow's trigger acts on its 202 answer with a message, that an inbox
// is a list of the workflow's subject checking its approval step's
// permission, and that the messages a workflow publishes exist.
func (c *checker) checkMakerChecker(d *design) {
	workflows := map[string]*yaml.Node{}
	for _, w := range source.Pairs(source.Child(d.root, "workflows")) {
		workflows[w.Key.Value] = w.Value
		for i, e := range source.Items(source.Child(w.Value, "emits")) {
			if d.message(e.Value) == nil {
				c.add(e, source.Pointer("workflows", w.Key.Value, "emits", fmt.Sprint(i)), RuleEmits, "%s does not name a channel and one of its messages; write channel/Message for a message declared under channels", e.Value)
			}
		}
	}
	triggers := d.workflowTriggers()
	for _, p := range source.Pairs(source.Child(d.root, "pages")) {
		name, pg := p.Key.Value, p.Value
		kind := source.Str(source.Child(pg, "kind"))
		submit := source.Str(source.Child(pg, "submit"))
		if wf := triggers[submit]; wf != "" && (kind == "form" || kind == "task") {
			under := ""
			if kind == "task" {
				under = " under 202"
			}
			switch ev := pendingEvent(pg); {
			case ev == nil:
				at := source.Key(pg, "onSubmitted")
				if at == nil {
					at = source.Key(pg, "submit")
				}
				c.add(at, source.Pointer("pages", name, "onSubmitted"), RuleWorkflow, "%s submits to %s, which starts workflow %s and answers 202 while the request waits for approval; add onSubmitted%s with the message that says so", name, submit, wf, under)
			case source.Child(ev, "message") == nil:
				ptr := []string{"pages", name, "onSubmitted"}
				if kind == "task" {
					ptr = append(ptr, "202")
				}
				c.add(ev, source.Pointer(ptr...), RuleWorkflow, "%s submits to %s, whose 202 answer leaves the request waiting for approval in workflow %s; give the event a message that says so", name, submit, wf)
			}
		}
		inbox := source.Child(pg, "inbox")
		if inbox == nil {
			continue
		}
		base := []string{"pages", name, "inbox"}
		if kind != "list" {
			c.add(source.Key(pg, "inbox"), source.Pointer(base...), RuleWorkflow, "%s is a %s page, and an inbox is a list of the requests waiting at an approval; leave inbox out", name, kind)
			continue
		}
		wn := source.Child(inbox, "workflow")
		w := workflows[source.Str(wn)]
		if w == nil {
			c.add(wn, source.Pointer(append(base, "workflow")...), RuleWorkflow, "%s is not a workflow of the specification%s", source.Str(wn), suggest(source.Str(wn), workflows))
			continue
		}
		sn := source.Child(inbox, "step")
		approvals := map[string]*yaml.Node{}
		for _, st := range source.Items(source.Child(w, "steps")) {
			if source.Str(source.Child(st, "kind")) == "approval" {
				approvals[source.Str(source.Child(st, "name"))] = st
			}
		}
		step := approvals[source.Str(sn)]
		if step == nil {
			c.add(sn, source.Pointer(append(base, "step")...), RuleWorkflow, "%s is not an approval step of workflow %s%s", source.Str(sn), wn.Value, suggest(source.Str(sn), approvals))
			continue
		}
		if subject, ent := source.Str(source.Child(w, "subject")), source.Child(pg, "entity"); ent != nil && ent.Value != subject {
			c.add(ent, source.Pointer("pages", name, "entity"), RuleWorkflow, "%s is the inbox of workflow %s, whose requests wait as %s; list %s", name, wn.Value, subject, subject)
		}
		if perm, pn := source.Str(source.Child(step, "permission")), source.Child(pg, "permission"); pn != nil && pn.Value != perm {
			c.add(pn, source.Pointer("pages", name, "permission"), RuleWorkflow, "%s is the inbox of step %s of workflow %s, which checks %s; give the page that permission, so it opens to whoever may approve", name, sn.Value, wn.Value, perm)
		}
	}
}
