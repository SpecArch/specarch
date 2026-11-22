package generate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// BRD writes the business requirements document (ADR-248), in the order
// of the business requirements specification of ISO/IEC/IEEE 29148: the
// purpose, the intent parts the specification states, the stakeholders,
// the needs, each requirement at business level, the constraints, the
// assumptions and the glossary. A requirement shows what the business
// asks and why, not how it is designed or verified; those are the
// requirements specification's.
func BRD(root *yaml.Node, relRoot string, impls []Implementation, state *State) string {
	d := newDoc("brd", root, relRoot, impls, state)
	info := get(root, "info")
	reqs := pairs(root, "requirements")
	needs := pairs(root, "needs")
	d.line(tr("# %s: business requirements document"), str(info, "title"))
	d.blank()
	d.para(fmt.Sprintf(tr("Version %s of the specification: why the system is built, for whom, and what the business asks of it, %s, %s and %s, in the order of the business requirements specification of ISO/IEC/IEEE 29148. How each requirement is designed and verified is in the requirements specification."),
		str(info, "version"), countText(len(pairs(root, "stakeholders")), tr("stakeholder"), tr("stakeholders")), countText(len(needs), tr("need"), tr("needs")), countText(len(reqs), tr("requirement"), tr("requirements"))))
	d.draftNotice("brd")

	d.section(tr("Purpose and scope"))
	if desc := str(info, "description"); desc != "" {
		d.para(desc)
	} else {
		d.para(tr("The specification has no description, so nothing here says what the system is in a few sentences; add info.description."))
	}
	if owners := strs(info, "owners"); len(owners) > 0 {
		d.para(tr("Owners: ") + strings.Join(owners, ", ") + ".")
	}

	intentParts(d, root, 2, true)

	if sh := pairs(root, "stakeholders"); len(sh) > 0 {
		d.section(tr("Stakeholders"))
		d.line(tr("| Stakeholder | Who they are | Concerns |"))
		d.line("|---|---|---|")
		for _, s := range sh {
			d.line("| %s | %s | %s |", s.Key.Value, cell(str(s.Value, "description")), cell(joinSentences(strs(s.Value, "concerns"))))
		}
		d.blank()
		d.explainRows(rowsOf(sh))
	} else {
		d.section(tr("Stakeholders"))
		d.para(tr("No stakeholder is written, so nothing says whom the system serves; add stakeholders, each with description and concerns."))
	}

	refinedBy := map[string][]string{}
	for _, r := range reqs {
		for _, n := range strs(r.Value, "needs") {
			refinedBy[n] = append(refinedBy[n], r.Key.Value)
		}
	}
	goalsOf := map[string][]string{}
	if len(needs) > 0 {
		d.section(tr("Business needs"))
		d.para(tr("What the stakeholders said they need, the goals each need serves, and the requirements that refine it."))
		d.line(tr("| Need | Statement | Stakeholders | Goals | Status | Refined by |"))
		d.line("|---|---|---|---|---|---|")
		for _, n := range needs {
			goalsOf[n.Key.Value] = strs(n.Value, "goals")
			d.line("| %s | %s | %s | %s | %s | %s |", n.Key.Value, cell(str(n.Value, "statement")), cell(strings.Join(strs(n.Value, "stakeholders"), ", ")), cell(strings.Join(strs(n.Value, "goals"), ", ")), cell(str(n.Value, "status")), cell(strings.Join(refinedBy[n.Key.Value], ", ")))
		}
		d.blank()
		d.explainRows(rowsOf(needs))
	}

	d.section(tr("Business requirements"))
	if len(reqs) == 0 {
		d.para(tr("No requirement is written, so nothing says what the business asks of the system; add requirements, each with statement, priority and the needs it refines."))
	} else {
		d.para(tr("Each requirement as the business asks it: its statement, how important it is, where it stands, the needs it refines and the goals those needs serve."))
		d.line(tr("| Requirement | Statement | Priority | Status | Needs | Goals |"))
		d.line("|---|---|---|---|---|---|")
		for _, r := range reqs {
			var goals []string
			seen := map[string]bool{}
			for _, n := range strs(r.Value, "needs") {
				for _, g := range goalsOf[n] {
					if !seen[g] {
						seen[g] = true
						goals = append(goals, g)
					}
				}
			}
			d.line("| %s | %s | %s | %s | %s | %s |", r.Key.Value, cell(str(r.Value, "statement")), cell(str(r.Value, "priority")), cell(str(r.Value, "status")), cell(strings.Join(strs(r.Value, "needs"), ", ")), cell(strings.Join(goals, ", ")))
		}
		d.blank()
		var unrefined []string
		for _, r := range reqs {
			if len(strs(r.Value, "needs")) == 0 && heldElsewhere(r.Value) == "" {
				unrefined = append(unrefined, r.Key.Value)
			}
		}
		if len(unrefined) > 0 {
			d.para(fmt.Sprintf(tr("%s refine no need, so nothing says which stakeholder asks for them: %s."), countText(len(unrefined), tr("requirement"), tr("requirements")), strings.Join(unrefined, ", ")))
		}
		d.explainRows(rowsOf(reqs))
	}

	if cons := pairs(root, "constraints"); len(cons) > 0 {
		d.section(tr("Constraints"))
		d.line(tr("| Constraint | Kind | Statement |"))
		d.line("|---|---|---|")
		for _, c := range cons {
			d.line("| %s | %s | %s |", c.Key.Value, str(c.Value, "kind"), cell(str(c.Value, "statement")))
		}
		d.blank()
		d.explainRows(rowsOf(cons))
	}
	if as := pairs(root, "assumptions"); len(as) > 0 {
		d.section(tr("Assumptions"))
		d.line(tr("| Assumption | Statement |"))
		d.line("|---|---|")
		for _, a := range as {
			d.line("| %s | %s |", a.Key.Value, cell(str(a.Value, "statement")))
		}
		d.blank()
		d.explainRows(rowsOf(as))
	}
	if terms := pairs(root, "glossary"); len(terms) > 0 {
		d.section(tr("Glossary"))
		d.line(tr("| Term | Meaning |"))
		d.line("|---|---|")
		for _, t := range terms {
			d.line("| %s | %s |", cell(t.Key.Value), cell(str(t.Value, "definition")))
		}
		d.blank()
		d.explainRows(rowsOf(terms))
	}
	d.sourcesIndex(tr("Sources"))
	return d.String()
}
