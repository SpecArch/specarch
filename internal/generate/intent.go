package generate

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The intent of a specification (ADR-242, ADR-244): the problem it
// solves, the goals with how they are measured, what is out of scope and
// the quality goals in rank order, the first thing a reviewer and an
// agent read.

// intentSections are the sections of the requirements stage that hold
// the intent.
var intentSections = []string{"problem", "goals", "outOfScope", "qualityGoals"}

// HasIntent reports whether a specification states any of its intent.
func HasIntent(root *yaml.Node) bool {
	for _, s := range intentSections {
		if get(root, s) != nil {
			return true
		}
	}
	return false
}

// Intent writes intent.md: the problem, the goals with their measures,
// what is out of scope and the quality goals in rank order, about one
// page, in the shape of section 1 of arc42. A part the specification does
// not state says so and names the key that states it.
func Intent(root *yaml.Node, relRoot string, impls []Implementation, state *State) string {
	d := newDoc("intent", root, relRoot, impls, state)
	info := get(root, "info")
	d.line(tr("# %s: intent"), str(info, "title"))
	d.blank()
	d.para(fmt.Sprintf(tr("Version %s of the specification: why the system is built and what counts as success, %s, %s, %s and %s, in the shape of section 1 of arc42. Every other part of the specification is judged against this page."),
		str(info, "version"), problemCount(root), countText(len(pairs(root, "goals")), tr("goal"), tr("goals")),
		countText(len(pairs(root, "outOfScope")), tr("out-of-scope entry"), tr("out-of-scope entries")),
		countText(len(pairs(root, "qualityGoals")), tr("quality goal"), tr("quality goals"))))
	d.draftNotice("intent")
	intentParts(d, root, 2, true)
	d.sourcesIndex(tr("Sources"))
	return d.String()
}

// problemCount is "a problem" or "no problem".
func problemCount(root *yaml.Node) string {
	if get(root, "problem") != nil {
		return tr("the problem")
	}
	return tr("no problem")
}

// intentParts writes the four parts of the intent as headings of a level:
// each part the specification states, and, when missing is true, each it
// does not, saying what is missing and the key that states it.
func intentParts(d *doc, root *yaml.Node, level int, missing bool) {
	number := func(title string) string {
		if level == 2 {
			d.sections++
			return fmt.Sprintf("%d. %s", d.sections, title)
		}
		return title
	}
	if p := get(root, "problem"); p != nil {
		d.heading(level, number(tr("Problem")))
		d.para(str(p, "statement"))
		if sh := strs(p, "stakeholders"); len(sh) > 0 {
			d.para(tr("Who it hurts: ") + strings.Join(sh, ", ") + ".")
		}
		if today := str(p, "today"); today != "" {
			d.para(tr("Today: ") + oneParagraph(today))
		}
		d.explain(p)
	} else if missing {
		d.heading(level, number(tr("Problem")))
		d.para(tr("No problem is written, so nothing says what the system is for. Add problem with statement, the problem the system solves in plain words, and the stakeholders it hurts and how things stand today when they are known."))
	}

	goals := pairs(root, "goals")
	if len(goals) > 0 {
		servedBy := goalNeeds(root)
		d.heading(level, number(tr("Goals")))
		d.para(tr("The outcomes the owner of the system wants, each with what is counted, the value today and the value wanted, and the needs that serve it."))
		d.line(tr("| Goal | Statement | Measure | Baseline | Target | By | Priority | Status | Served by |"))
		d.line("|---|---|---|---|---|---|---|---|---|")
		for _, g := range goals {
			d.line("| %s | %s | %s | %s | %s | %s | %s | %s | %s |", markInternal(g.Key.Value, g.Value), cell(str(g.Value, "statement")), cell(str(g.Value, "measure")),
				cell(str(g.Value, "baseline")), cell(str(g.Value, "target")), cell(str(g.Value, "by")), cell(str(g.Value, "priority")), cell(str(g.Value, "status")), cell(strings.Join(servedBy[g.Key.Value], ", ")))
		}
		d.blank()
		for _, g := range goals {
			if m := strs(g.Value, "measuredBy"); len(m) > 0 {
				d.para(fmt.Sprintf(tr("%s is read on the live system by %s."), g.Key.Value, strings.Join(m, ", ")))
			}
		}
		d.explainRows(rowsOf(goals))
	} else if missing {
		d.heading(level, number(tr("Goals")))
		d.para(tr("No goal is written, so nothing says what counts as success. Add goals, each with statement, measure, baseline and target, and name the goals each need serves under goals on the need."))
	}

	outs := pairs(root, "outOfScope")
	if len(outs) > 0 {
		d.heading(level, number(tr("Out of scope")))
		d.para(tr("What the system will not do, decided on purpose, so a question or a change request that asks for it is answered here."))
		d.line(tr("| Entry | Statement | Until | Rules out |"))
		d.line("|---|---|---|---|")
		for _, o := range outs {
			until := str(o.Value, "until")
			if until == "" {
				until = tr("not planned")
			}
			d.line("| %s | %s | %s | %s |", markInternal(o.Key.Value, o.Value), cell(str(o.Value, "statement")), cell(until), cell(strings.Join(strs(o.Value, "needs"), ", ")))
		}
		d.blank()
		d.explainRows(rowsOf(outs))
	} else if missing {
		d.heading(level, number(tr("Out of scope")))
		d.para(tr("Nothing is written out of scope, so a change request is triaged by a meeting, not a lookup. Add outOfScope with statement and why for each thing the system will not do."))
	}

	qgs := rankedQualityGoals(root)
	if len(qgs) > 0 {
		d.heading(level, number(tr("Quality goals")))
		d.para(tr("The qualities that matter most, in rank order: when two pull apart, the higher rank wins."))
		d.line(tr("| Rank | Quality goal | Quality | Statement | Requirements |"))
		d.line("|---|---|---|---|---|")
		for _, q := range qgs {
			d.line("| %s | %s | %s | %s | %s |", str(q.Value, "rank"), markInternal(q.Key.Value, q.Value), cell(str(q.Value, "quality")), cell(str(q.Value, "statement")), cell(strings.Join(strs(q.Value, "requirements"), ", ")))
		}
		d.blank()
		d.explainRows(rowsOf(qgs))
	} else if missing {
		d.heading(level, number(tr("Quality goals")))
		d.para(tr("No quality goal is written, so a trade-off is argued rather than decided. Add qualityGoals, three to five, each with rank, quality and statement."))
	}
}

// rankedQualityGoals are the quality goals by rank, then in the order
// written.
func rankedQualityGoals(root *yaml.Node) []sourcePair {
	qgs := append([]sourcePair{}, pairs(root, "qualityGoals")...)
	rank := func(p sourcePair) int {
		n, err := strconv.Atoi(str(p.Value, "rank"))
		if err != nil {
			return 1 << 30
		}
		return n
	}
	sort.SliceStable(qgs, func(i, j int) bool { return rank(qgs[i]) < rank(qgs[j]) })
	return qgs
}

// goalNeeds maps each goal to the needs that name it, in the order of
// the needs.
func goalNeeds(root *yaml.Node) map[string][]string {
	out := map[string][]string{}
	for _, n := range pairs(root, "needs") {
		for _, g := range strs(n.Value, "goals") {
			out[g] = append(out[g], n.Key.Value)
		}
	}
	return out
}

// requirementGoals maps each requirement to the goals its needs serve,
// each once, in the order first met.
func requirementGoals(root *yaml.Node) map[string][]string {
	needGoals := map[string][]string{}
	for _, n := range pairs(root, "needs") {
		needGoals[n.Key.Value] = strs(n.Value, "goals")
	}
	out := map[string][]string{}
	for _, r := range pairs(root, "requirements") {
		seen := map[string]bool{}
		for _, n := range strs(r.Value, "needs") {
			for _, g := range needGoals[n] {
				if !seen[g] {
					seen[g] = true
					out[r.Key.Value] = append(out[r.Key.Value], g)
				}
			}
		}
	}
	return out
}

// weighsLine is the line a decision's weighs makes in the techspec: each
// quality goal it weighed with its rank and quality, highest rank first,
// so the reader sees which side of the trade-off the decision took.
func weighsLine(root, dec *yaml.Node) string {
	names := strs(dec, "weighs")
	if len(names) == 0 {
		return ""
	}
	byID := map[string]*yaml.Node{}
	for _, q := range pairs(root, "qualityGoals") {
		byID[q.Key.Value] = q.Value
	}
	var parts []string
	for _, q := range rankedQualityGoals(root) {
		for _, n := range names {
			if n == q.Key.Value {
				parts = append(parts, fmt.Sprintf(tr("%s (rank %s, %s)"), n, str(q.Value, "rank"), str(q.Value, "quality")))
			}
		}
	}
	for _, n := range names {
		if byID[n] == nil {
			parts = append(parts, n)
		}
	}
	return tr("Quality goals weighed, highest rank first: ") + strings.Join(parts, ", ") + "."
}

// progressGoals writes, for a specification with goals, per goal, the baseline, the target, the latest
// value a monitor read and how many of the requirements that serve it are
// met: a requirement is met once the release it names is released.
func (d *doc) progressGoals(root *yaml.Node, state *State) {
	goals := pairs(root, "goals")
	released := map[string]bool{}
	for _, r := range recordsOf(state, "release") {
		if str(r, "status") == "released" {
			released[str(r, "version")] = true
		}
	}
	byGoal := map[string][]string{}
	reqGoals := requirementGoals(root)
	status := map[string]string{}
	release := map[string]string{}
	for _, r := range pairs(root, "requirements") {
		status[r.Key.Value] = str(r.Value, "status")
		release[r.Key.Value] = str(r.Value, "release")
		for _, g := range reqGoals[r.Key.Value] {
			byGoal[g] = append(byGoal[g], r.Key.Value)
		}
	}
	d.para(tr("Each goal with the value it started from, the value wanted, the latest value a monitor read, and how many of the requirements that serve it, through the needs that name the goal, are met: a requirement is met once the release it names is released. Rejected and retired requirements are not counted. No record holds a monitor's reading yet, so the latest value is none recorded."))
	d.line(tr("| Goal | Status | Baseline | Target | Latest reading | Requirements met |"))
	d.line("|---|---|---|---|---|---|")
	for _, g := range goals {
		total, met := 0, 0
		for _, id := range byGoal[g.Key.Value] {
			if s := status[id]; s == "rejected" || s == "retired" {
				continue
			}
			total++
			if released[release[id]] {
				met++
			}
		}
		share := tr("none serves it")
		if total > 0 {
			share = fmt.Sprintf(tr("%d of %d"), met, total)
		}
		d.line("| %s | %s | %s | %s | %s | %s |", g.Key.Value, cell(str(g.Value, "status")), cell(str(g.Value, "baseline")), cell(str(g.Value, "target")), tr("none recorded"), share)
	}
	d.blank()
}
