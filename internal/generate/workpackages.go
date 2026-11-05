package generate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The work-package records in the documents (ADR-245): a section of the
// progress report with one row per package and the requirements no
// package plans, and a section of the test plan with the design tests
// grouped by the package whose requirements they verify. Both appear only
// once a work-package record exists.

// livePackages is the work packages that are not withdrawn, by ID.
func livePackages(state *State) []*yaml.Node {
	var out []*yaml.Node
	for _, p := range recordsOf(state, "work-package") {
		if str(p, "status") != "withdrawn" {
			out = append(out, p)
		}
	}
	return out
}

// progressWorkPackages writes one row per work package, what blocks each,
// and the accepted requirements of a planned release that no package
// holds.
func (d *doc) progressWorkPackages(root *yaml.Node, state *State) {
	packages := recordsOf(state, "work-package")
	status := map[string]string{}
	for _, p := range packages {
		status[str(p, "id")] = str(p, "status")
	}
	d.para(fmt.Sprintf(tr("%s recorded beside the specification. A work package is one piece of the delivery, what a tracker may call an epic or a batch: the requirements it holds, the release it belongs to and the packages that must be done before it. A package is blocked while a package it comes after is not done, or while the validator warns it; a done package the validator warns is listed with the warning, since QA's exit criterion does not hold for it. A withdrawn package is part of no plan."),
		countText(len(packages), tr("work package is"), tr("work packages are"))))
	d.line(tr("| Package | Release | Status | Assigned to | After | Requirements | Estimated hours | Spent hours | Blocked by |"))
	d.line("|---|---|---|---|---|---|---|---|---|")
	for _, p := range packages {
		id := str(p, "id")
		var estimated, spent hours
		effort := get(p, "effort")
		var blocked []string
		if st := str(p, "status"); st != "withdrawn" {
			if st != "done" {
				for _, a := range strs(p, "after") {
					if s := status[a]; s != "done" {
						blocked = append(blocked, fmt.Sprintf(tr("%s (%s)"), a, orNone(s, tr("no record"))))
					}
				}
			}
			seen := map[string]bool{}
			for _, m := range state.Marks {
				if strings.HasSuffix(m.File, "work-packages/"+id+".yaml") && !seen[m.Rule] {
					seen[m.Rule] = true
					blocked = append(blocked, m.Rule)
				}
			}
		}
		assigned := orNone(str(p, "assignedTo"), tr("nobody named"))
		if state.Withheld != nil && str(p, "assignedTo") != "" {
			assigned = tr("not shown in a document for everyone")
		}
		d.line("| %s, %s | %s | %s | %s | %s | %s | %s | %s | %s |", id, cell(str(p, "title")), orNone(str(p, "release"), tr("none named")), str(p, "status"),
			cell(assigned), orNone(strings.Join(strs(p, "after"), ", "), tr("none")), strings.Join(strs(p, "requirements"), ", "),
			estimated.add(get(effort, "estimatedHours")), spent.add(get(effort, "spentHours")), cell(orNone(strings.Join(blocked, ", "), tr("nothing"))))
	}
	d.blank()
	planned := map[string]bool{}
	for _, r := range recordsOf(state, "release") {
		if str(r, "status") == "planned" {
			planned[str(r, "version")] = true
		}
	}
	held := map[string]bool{}
	for _, p := range livePackages(state) {
		for _, id := range strs(p, "requirements") {
			held[id] = true
		}
	}
	var unplanned []string
	for _, r := range pairs(root, "requirements") {
		if str(r.Value, "status") == "accepted" && planned[str(r.Value, "release")] && !held[r.Key.Value] && (state.Withheld == nil || !state.Withheld.CoversRef(r.Key.Value)) {
			unplanned = append(unplanned, fmt.Sprintf("%s (%s)", r.Key.Value, str(r.Value, "release")))
		}
	}
	if len(unplanned) == 0 {
		d.para(tr("Unplanned: every accepted requirement of a planned release is in a work package that is not withdrawn."))
		return
	}
	d.para(fmt.Sprintf(tr("Unplanned: %s of a planned release %s in no work package, so nobody is planned to build %s: %s."),
		countText(len(unplanned), tr("accepted requirement"), tr("accepted requirements")), pick(len(unplanned) == 1, tr("is"), tr("are")), pick(len(unplanned) == 1, tr("it"), tr("them")), strings.Join(unplanned, ", ")))
}

// testsByPackage groups the design tests by the work package whose
// requirements they verify, each package with its requirements and the
// tests of each, then the tests no package covers.
func testsByPackage(d *doc, tests []sourcePair, state *State) {
	if state == nil || len(recordsOf(state, "work-package")) == 0 {
		return
	}
	packages := livePackages(state)
	verifiedBy := map[string][]string{}
	for _, t := range tests {
		for _, id := range strs(t.Value, "verifies") {
			verifiedBy[id] = append(verifiedBy[id], t.Key.Value)
		}
	}
	covered := map[string]bool{}
	d.section(tr("Tests by work package"))
	d.para(fmt.Sprintf(tr("%s not withdrawn, each with its requirements and the design tests that verify them, so each package is tested as it lands; a package is done once each of these tests has passed."),
		countText(len(packages), tr("work package is"), tr("work packages are"))))
	for _, p := range packages {
		title := fmt.Sprintf("%s, %s", str(p, "id"), str(p, "title"))
		d.heading(3, title)
		d.para(fmt.Sprintf(tr("Release %s, status %s."), orNone(str(p, "release"), tr("none named")), str(p, "status")))
		d.line(tr("| Requirement | Verified by |"))
		d.line("|---|---|")
		for _, id := range strs(p, "requirements") {
			by := tr("none")
			if t := verifiedBy[id]; len(t) > 0 {
				by = strings.Join(t, ", ")
				for _, name := range t {
					covered[name] = true
				}
			}
			d.line("| %s | %s |", id, cell(by))
		}
		d.blank()
	}
	var rest []string
	for _, t := range tests {
		if !covered[t.Key.Value] {
			rest = append(rest, t.Key.Value)
		}
	}
	if len(rest) == 0 {
		d.para(tr("Every design test verifies a requirement of a work package."))
		return
	}
	d.para(fmt.Sprintf(tr("In no work package: %s, since %s no requirement a package holds: %s."),
		countText(len(rest), tr("design test"), tr("design tests")), pick(len(rest) == 1, tr("it verifies"), tr("they verify")), strings.Join(rest, ", ")))
}
