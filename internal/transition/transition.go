// Package transition lists, for each workflow that keeps its as-is beside
// the to-be while a business process changes, what moving from the as-is
// to the to-be adds, changes and removes: its steps, the roles that
// approve or do them, the operations it calls, the pages and the tests
// that rest on it (ADR-236). The changes document and specarch diff print
// the same list.
package transition

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// Part is one kind of thing a transition reaches, with what it does to
// each, in the order the list prints them.
type Part struct {
	Name  string   // workflow, steps, approvers and performers, operations, pages or tests
	Items []string // each one sentence, without a full stop
}

// Transition is what moving one workflow from its as-is to its to-be
// reaches; a part with nothing in it is left out.
type Transition struct {
	Workflow string // the workflow's name
	Pointer  string // its pointer, such as /workflows/refund
	Parts    []Part
}

// Of lists the transitions of a merged specification, one per workflow
// that keeps an as-is, in the order the workflows are written.
func Of(root *yaml.Node) []Transition {
	var out []Transition
	for _, w := range source.Pairs(source.Child(root, "workflows")) {
		asIs := source.Deref(source.Child(w.Value, "asIs"))
		if asIs == nil || asIs.Kind != yaml.MappingNode {
			continue
		}
		out = append(out, compare(root, w.Key.Value, asIs, w.Value))
	}
	return out
}

// Lines are the transition's parts, one line each, as specarch diff
// prints them: the part's name, then what it does to each, joined by
// semicolons.
func (t Transition) Lines() []string {
	var out []string
	for _, p := range t.Parts {
		out = append(out, p.Name+": "+strings.Join(p.Items, "; "))
	}
	return out
}

// step is one step of a variant, by its name.
type step struct {
	node  *yaml.Node
	kind  string
	index int
}

func stepsOf(w *yaml.Node) (map[string]step, []string) {
	byName := map[string]step{}
	var order []string
	for i, st := range source.Items(source.Child(w, "steps")) {
		name := source.Str(source.Child(st, "name"))
		if name == "" {
			continue
		}
		if _, dup := byName[name]; dup {
			continue
		}
		byName[name] = step{st, source.Str(source.Child(st, "kind")), i}
		order = append(order, name)
	}
	return byName, order
}

func compare(root *yaml.Node, name string, asIs, toBe *yaml.Node) Transition {
	t := Transition{Workflow: name, Pointer: source.Pointer("workflows", name)}
	add := func(part string, items []string) {
		if len(items) > 0 {
			t.Parts = append(t.Parts, Part{part, items})
		}
	}

	// The workflow's own keys: its trigger, subject and the messages it
	// publishes.
	var own []string
	for _, key := range []string{"trigger", "subject"} {
		was, is := source.Str(source.Child(asIs, key)), source.Str(source.Child(toBe, key))
		if was != is {
			own = append(own, fmt.Sprintf("changes the %s from %s to %s", key, orNone(was), orNone(is)))
		}
	}
	added, removed := setDiff(values(source.Child(asIs, "emits")), values(source.Child(toBe, "emits")))
	for _, m := range added {
		own = append(own, "publishes "+m+" at its end")
	}
	for _, m := range removed {
		own = append(own, "no longer publishes "+m)
	}
	add("workflow", own)

	// The steps, by name: those the to-be adds, those it removes, those
	// whose keys differ, and the order of those both keep.
	wasSteps, wasOrder := stepsOf(asIs)
	isSteps, isOrder := stepsOf(toBe)
	var steps []string
	var kept []string
	for _, n := range isOrder {
		if _, ok := wasSteps[n]; !ok {
			steps = append(steps, "adds "+n+" ("+isSteps[n].kind+")")
		}
	}
	for _, n := range wasOrder {
		if _, ok := isSteps[n]; !ok {
			steps = append(steps, "removes "+n+" ("+wasSteps[n].kind+")")
		} else {
			kept = append(kept, n)
		}
	}
	changed := map[string]bool{}
	for _, n := range isOrder {
		was, ok := wasSteps[n]
		if !ok {
			continue
		}
		if keys := differentKeys(was.node, isSteps[n].node); len(keys) > 0 {
			steps = append(steps, "changes "+n+" ("+strings.Join(keys, ", ")+")")
			changed[n] = true
		}
	}
	var keptInNew []string
	for _, n := range isOrder {
		if _, ok := wasSteps[n]; ok {
			keptInNew = append(keptInNew, n)
		}
	}
	if !slices.Equal(kept, keptInNew) {
		steps = append(steps, "orders the steps it keeps as "+strings.Join(keptInNew, ", "))
	}
	add("steps", steps)

	// The roles that approve a step or do its task: who gains or loses
	// work at each step.
	var roles []string
	for _, n := range isOrder {
		is := isSteps[n]
		was, ok := wasSteps[n]
		for _, key := range []string{"approvers", "performers"} {
			plus, minus := setDiff(values(source.Child(was.node, key)), values(source.Child(is.node, key)))
			if !ok {
				plus, minus = values(source.Child(is.node, key)), nil
			}
			for _, r := range plus {
				roles = append(roles, fmt.Sprintf("adds %s as %s at %s", r, role(key), n))
			}
			for _, r := range minus {
				roles = append(roles, fmt.Sprintf("removes %s as %s at %s", r, role(key), n))
			}
		}
	}
	for _, n := range wasOrder {
		if _, ok := isSteps[n]; ok {
			continue
		}
		for _, key := range []string{"approvers", "performers"} {
			for _, r := range values(source.Child(wasSteps[n].node, key)) {
				roles = append(roles, fmt.Sprintf("removes %s as %s at %s", r, role(key), n))
			}
		}
	}
	add("approvers and performers", roles)

	// The operations the workflow starts from, calls or undoes with.
	opsAdded, opsRemoved := setDiff(operations(asIs), operations(toBe))
	var ops []string
	for _, o := range opsAdded {
		ops = append(ops, "calls "+o)
	}
	for _, o := range opsRemoved {
		ops = append(ops, "no longer calls "+o)
	}
	add("operations", ops)

	// The pages: a form or task that submits to a trigger that changes,
	// and an inbox of a step the to-be removes or changes.
	wasTrigger, isTrigger := source.Str(source.Child(asIs, "trigger")), source.Str(source.Child(toBe, "trigger"))
	var pages []string
	for _, p := range source.Pairs(source.Child(root, "pages")) {
		pg := p.Value
		if submit := source.Str(source.Child(pg, "submit")); wasTrigger != isTrigger && submit != "" {
			switch submit {
			case wasTrigger:
				pages = append(pages, fmt.Sprintf("%s submits to %s, which starts the as-is and not the to-be", p.Key.Value, submit))
			case isTrigger:
				pages = append(pages, fmt.Sprintf("%s submits to %s, which starts the to-be", p.Key.Value, submit))
			}
		}
		inbox := source.Child(pg, "inbox")
		if source.Str(source.Child(inbox, "workflow")) != name {
			continue
		}
		at := source.Str(source.Child(inbox, "step"))
		_, inWas := wasSteps[at]
		_, inIs := isSteps[at]
		switch {
		case inWas && !inIs:
			pages = append(pages, fmt.Sprintf("%s lists the requests waiting at %s, which the to-be removes", p.Key.Value, at))
		case !inWas && inIs:
			pages = append(pages, fmt.Sprintf("%s lists the requests waiting at %s, which the to-be adds", p.Key.Value, at))
		case changed[at]:
			pages = append(pages, fmt.Sprintf("%s lists the requests waiting at %s, which the to-be changes", p.Key.Value, at))
		}
	}
	add("pages", pages)

	// The tests: those about the workflow, read again against the
	// to-be, and those about an operation it no longer or newly calls.
	touched := map[string]string{}
	for _, o := range opsAdded {
		touched[o] = "which the to-be calls"
	}
	for _, o := range opsRemoved {
		touched[o] = "which the to-be no longer calls"
	}
	var tests []string
	for _, p := range source.Pairs(source.Child(root, "tests")) {
		switch tc := p.Value; {
		case source.Str(source.Child(tc, "workflow")) == name:
			tests = append(tests, p.Key.Value+" is about the workflow, so read it against the to-be")
		case touched[source.Str(source.Child(tc, "operation"))] != "":
			op := source.Str(source.Child(tc, "operation"))
			tests = append(tests, fmt.Sprintf("%s is about %s, %s", p.Key.Value, op, touched[op]))
		}
	}
	add("tests", tests)
	return t
}

func role(key string) string {
	if key == "approvers" {
		return "approver"
	}
	return "performer"
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// operations lists the operations a variant names: its trigger, each
// operation step's operation and what undoes it, once each, in order.
func operations(w *yaml.Node) []string {
	var out []string
	seen := map[string]bool{}
	note := func(n *yaml.Node) {
		if v := source.Str(n); v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	note(source.Child(w, "trigger"))
	for _, st := range source.Items(source.Child(w, "steps")) {
		note(source.Child(st, "operation"))
		note(source.Child(st, "compensatedBy"))
	}
	return out
}

// values lists the scalar items of a list.
func values(n *yaml.Node) []string {
	var out []string
	for _, it := range source.Items(n) {
		out = append(out, source.Str(it))
	}
	return out
}

// setDiff is what b has that a has not, and what a has that b has not,
// each in its own list's order.
func setDiff(a, b []string) (added, removed []string) {
	for _, s := range b {
		if !slices.Contains(a, s) {
			added = append(added, s)
		}
	}
	for _, s := range a {
		if !slices.Contains(b, s) {
			removed = append(removed, s)
		}
	}
	return added, removed
}

// differentKeys names the keys of two steps whose values differ, the
// to-be's keys first in its order, then those only the as-is has.
func differentKeys(a, b *yaml.Node) []string {
	av, _ := source.ValueOf(a).(map[string]any)
	bv, _ := source.ValueOf(b).(map[string]any)
	var keys []string
	seen := map[string]bool{}
	for _, p := range source.Pairs(b) {
		keys = append(keys, p.Key.Value)
		seen[p.Key.Value] = true
	}
	var rest []string
	for k := range av {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	var out []string
	for _, k := range append(keys, rest...) {
		if k == "name" {
			continue
		}
		if !reflect.DeepEqual(av[k], bv[k]) {
			out = append(out, k)
		}
	}
	return out
}

// Select gives the specification, as plain values, with each workflow as
// one variant for the generators: the to-be, with no asIs, or with asIs
// the as-is in the workflow's place, with the workflow's own satisfies,
// audience and decidedIn, so a workflow that keeps none stays as it is. The specification given is not changed.
func Select(spec any, asIs bool) any {
	root, ok := spec.(map[string]any)
	if !ok {
		return spec
	}
	workflows, ok := root["workflows"].(map[string]any)
	if !ok {
		return spec
	}
	picked := make(map[string]any, len(workflows))
	for name, w := range workflows {
		entry, ok := w.(map[string]any)
		if !ok {
			picked[name] = w
			continue
		}
		if old, has := entry["asIs"].(map[string]any); has && asIs {
			// The keys of the element itself, which an as-is has no
			// place for, stay the workflow's.
			moved := make(map[string]any, len(old)+3)
			for k, v := range old {
				moved[k] = v
			}
			for _, k := range []string{"satisfies", "audience", "decidedIn"} {
				if v, ok := entry[k]; ok {
					moved[k] = v
				}
			}
			picked[name] = moved
			continue
		}
		if _, has := entry["asIs"]; !has {
			picked[name] = entry
			continue
		}
		toBe := make(map[string]any, len(entry))
		for k, v := range entry {
			if k != "asIs" {
				toBe[k] = v
			}
		}
		picked[name] = toBe
	}
	out := make(map[string]any, len(root))
	for k, v := range root {
		out[k] = v
	}
	out["workflows"] = picked
	return out
}

// keepsAsIs reports whether the workflow of that pointer token keeps an
// as-is.
func keepsAsIs(root *yaml.Node, token string) bool {
	asIs := source.Deref(source.Child(source.Child(source.Child(root, "workflows"), source.UnescapeToken(token)), "asIs"))
	return asIs != nil && asIs.Kind == yaml.MappingNode
}

// AsIsPointer is where a pointer into the merged specification stands in
// the specification Select gives with asIs true, so a problem is marked
// at the as-is's own entry: /workflows/<name>/asIs/... becomes
// /workflows/<name>/..., for a workflow that keeps an as-is. A pointer
// inside that workflow's to-be gives false, since the as-is output does
// not hold the entry; the workflow itself and every other pointer stay as
// they are. A leading # is kept.
func AsIsPointer(root *yaml.Node, ptr string) (string, bool) {
	hash := strings.HasPrefix(ptr, "#")
	tokens := strings.Split(strings.TrimPrefix(ptr, "#"), "/")
	if len(tokens) < 4 || tokens[0] != "" || tokens[1] != "workflows" || !keepsAsIs(root, tokens[2]) {
		return ptr, true
	}
	if tokens[3] != "asIs" {
		return "", false
	}
	out := strings.Join(append(tokens[:3:3], tokens[4:]...), "/")
	if hash {
		out = "#" + out
	}
	return out, true
}

// MergedPointer is the other way: a pointer a plug-in given the as-is
// answers with, back into the merged specification, so its diagnostic
// names the as-is's line. /workflows/<name>/... becomes
// /workflows/<name>/asIs/... for a workflow that keeps an as-is; the
// workflow itself and every other pointer stay as they are.
func MergedPointer(root *yaml.Node, ptr string) string {
	tokens := strings.Split(ptr, "/")
	if len(tokens) < 4 || tokens[0] != "" || tokens[1] != "workflows" || !keepsAsIs(root, tokens[2]) {
		return ptr
	}
	return strings.Join(append(append(tokens[:3:3], "asIs"), tokens[3:]...), "/")
}

// InAsIs reports whether a pointer, with or without a leading #, leads
// into the as-is of a workflow that keeps one.
func InAsIs(root *yaml.Node, ptr string) bool {
	tokens := strings.Split(strings.TrimPrefix(ptr, "#"), "/")
	return len(tokens) >= 4 && tokens[0] == "" && tokens[1] == "workflows" && tokens[3] == "asIs" && keepsAsIs(root, tokens[2])
}
