package generate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// scenarioTopic is the scenarios of one topic, in document order; the
// topics stand in the order each is first named.
type scenarioTopic struct {
	name      string
	scenarios []sourcePair
}

// scenarioTopics groups the scenarios by topic (ADR-251). A scenario with
// no topic, which the schema reports, stands under "No topic".
func scenarioTopics(root *yaml.Node) []scenarioTopic {
	var out []scenarioTopic
	at := map[string]int{}
	for _, s := range pairs(root, "scenarios") {
		name := str(s.Value, "topic")
		if name == "" {
			name = tr("No topic")
		}
		i, ok := at[name]
		if !ok {
			i = len(out)
			at[name] = i
			out = append(out, scenarioTopic{name: name})
		}
		out[i].scenarios = append(out[i].scenarios, s)
	}
	return out
}

// scenarioProvers gives, for each scenario, the tests whose proves names
// it, in document order.
func scenarioProvers(root *yaml.Node) map[string][]string {
	out := map[string][]string{}
	for _, t := range pairs(root, "tests") {
		for _, id := range strs(t.Value, "proves") {
			out[id] = append(out[id], t.Key.Value)
		}
	}
	return out
}

// scenarios writes the scenarios grouped by topic: a heading per topic at
// level, and under it each scenario at the level below, with its
// description, its stakeholder, its steps as a table, the sequence
// diagram drawn from them, the tests that prove it, and its marks.
func (d *doc) scenarios(root *yaml.Node, level int) {
	provers := scenarioProvers(root)
	for _, t := range scenarioTopics(root) {
		d.heading(level, t.name)
		for _, s := range t.scenarios {
			id := s.Key.Value
			anchor := d.heading(level+1, fmt.Sprintf("%s: %s", id, str(s.Value, "title")))
			d.headed("scenarios", id, anchor)
			if desc := str(s.Value, "description"); desc != "" {
				d.para(desc)
			}
			steps := items(s.Value, "steps")
			branches := 0
			for _, st := range steps {
				if str(st, "when") != "" {
					branches++
				}
			}
			d.para(fmt.Sprintf(tr("For %s; %s, %s."), str(s.Value, "stakeholder"),
				countText(len(steps), tr("step"), tr("steps")), countText(branches, tr("branch"), tr("branches"))))
			d.explain(s.Value)
			if len(steps) > 0 {
				d.line(tr("| Step | Actor | Before | What happens | Exercises | Result |"))
				d.line("|---|---|---|---|---|---|")
				for i, st := range steps {
					n := fmt.Sprint(i + 1)
					if w := str(st, "when"); w != "" {
						n = fmt.Sprintf(tr("%d, when %s"), i+1, w)
					}
					d.line("| %s | %s | %s | %s | %s | %s |", cell(n), cell(stepActor(s.Value, st)), cell(str(st, "precondition")),
						cell(str(st, "description")), cell(stepExercises(st)), cell(str(st, "result")))
				}
				d.blank()
				d.block(scenarioSequence(root, s.Value))
			}
			if by := provers[id]; len(by) > 0 {
				d.para(tr("Proved by ") + strings.Join(by, ", ") + ".")
			} else {
				d.para(tr("No test proves it yet: a golden or red test names it under proves."))
			}
		}
	}
}

// stepActor is who does a step: its actor, or the scenario's stakeholder.
func stepActor(scenario, step *yaml.Node) string {
	if a := str(step, "actor"); a != "" {
		return a
	}
	return str(scenario, "stakeholder")
}

// stepExercises is what a step names in the design, in the order the
// schema lists the keys.
func stepExercises(step *yaml.Node) string {
	var parts []string
	if op := str(step, "operation"); op != "" {
		parts = append(parts, tr("operation ")+op)
	}
	if r := strs(step, "requirements"); len(r) > 0 {
		parts = append(parts, tr("requirements ")+strings.Join(r, ", "))
	}
	if e := str(step, "entity"); e != "" {
		if c := str(step, "change"); c != "" {
			parts = append(parts, fmt.Sprintf(tr("entity %s: %s"), e, strings.TrimSuffix(c, ".")))
		} else {
			parts = append(parts, tr("entity ")+e)
		}
	}
	if t := strs(step, "tests"); len(t) > 0 {
		parts = append(parts, tr("tests ")+strings.Join(t, ", "))
	}
	return strings.Join(parts, "; ")
}

// scenarioSequence draws a scenario's steps as a sequence diagram: each
// actor, the scenario's stakeholder first, and the system; an operation
// step as the actor's call and the system's answer, an entity change as
// the system's own step, a step naming only requirements or tests as a
// note, and the condition of a branch as a note before its step.
func scenarioSequence(root *yaml.Node, scenario *yaml.Node) string {
	steps := items(scenario, "steps")
	actors := map[string]string{}
	var order []string
	note := func(name string) {
		if name != "" && actors[name] == "" {
			actors[name] = fmt.Sprintf("A%d", len(order)+1)
			order = append(order, name)
		}
	}
	note(str(scenario, "stakeholder"))
	for _, st := range steps {
		note(stepActor(scenario, st))
	}
	var b strings.Builder
	b.WriteString(tr("sequenceDiagram\n"))
	for _, a := range order {
		fmt.Fprintf(&b, tr("  actor %s as %s\n"), actors[a], mermaidText(a))
	}
	fmt.Fprintf(&b, tr("  participant S as %s\n"), mermaidText(str(get(root, "info"), "title")))
	for i, st := range steps {
		a := actors[stepActor(scenario, st)]
		if a == "" {
			a = "S"
		}
		if w := str(st, "when"); w != "" {
			fmt.Fprintf(&b, tr("  Note over %s,S: %d. when %s\n"), a, i+1, mermaidText(w))
		}
		switch {
		case str(st, "operation") != "":
			op := str(st, "operation")
			label := op
			if o := findOperation(root, op); o != nil {
				label = fmt.Sprintf("%s (%s %s)", op, strings.ToUpper(o.method), o.path)
			}
			fmt.Fprintf(&b, "  %s->>S: %d. %s\n", a, i+1, mermaidText(label))
			if e := str(st, "entity"); e != "" {
				fmt.Fprintf(&b, "  S->>S: %s\n", mermaidText(entityChange(e, str(st, "change"))))
			}
			if r := str(st, "result"); r != "" {
				fmt.Fprintf(&b, "  S-->>%s: %s\n", a, mermaidText(r))
			}
		case str(st, "entity") != "":
			fmt.Fprintf(&b, "  S->>S: %d. %s\n", i+1, mermaidText(entityChange(str(st, "entity"), str(st, "change"))))
		default:
			fmt.Fprintf(&b, "  Note over %s,S: %d. %s\n", a, i+1, mermaidText(firstNonEmpty(str(st, "description"), stepExercises(st))))
		}
	}
	return fence(b.String())
}

// entityChange is an entity and how a step changes it.
func entityChange(entity, change string) string {
	if change == "" {
		return entity
	}
	return entity + ": " + strings.TrimSuffix(change, ".")
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}
