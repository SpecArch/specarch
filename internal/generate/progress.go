package generate

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/semver"
	"github.com/SpecArch/specarch/internal/spec"
)

// Progress writes the progress report from the specification and the
// records beside it (ADR-227): its size by stage, the open questions by
// priority and deciding role, the unheard roles, the stakeholder list's
// confirmation, the effort each change request records, and a burn-up of
// the elements the change requests of each release name. Every figure is
// counted from what is written; none is estimated or filled in.
func Progress(root *yaml.Node, relRoot string, state *State) string {
	d := newRecordDoc("progress", root, relRoot, state)
	info := get(root, "info")
	size := sizeByStage(root)
	total := 0
	for _, s := range size {
		total += s.count
	}

	d.line("# %s: progress", str(info, "title"))
	d.blank()
	d.para(fmt.Sprintf("Version %s of the specification: %s, %s, from the specification and the records beside it. Every figure is counted from what the specification and its records hold; no estimate is made, and an effort nobody recorded is shown as missing.",
		str(info, "version"), countText(total, "element", "elements"), countText(len(d.questions), "open question", "open questions")))
	if state.Withheld != nil {
		d.para("This document is for everyone: the internal elements, and the questions only about them, are not counted.")
	}

	d.section("Size")
	d.para("The elements of each stage, a section of named elements counted by its entries and a section that holds one thing, such as the release, as one. The open questions are counted under Open work, not here.")
	d.line("| Stage | Elements | Sections |")
	d.line("|---|---|---|")
	for _, s := range size {
		d.line("| %s | %d | %s |", s.stage, s.count, cell(s.sections))
	}
	d.line("| Total | %d | |", total)
	d.blank()

	d.section("Open work")
	d.progressQuestions()

	d.section("Unheard roles")
	var unheard []Mark
	for _, m := range state.Marks {
		if m.Rule == "unheard" && strings.HasPrefix(m.Pointer, "/stakeholders/") {
			unheard = append(unheard, m)
		}
	}
	if len(unheard) == 0 {
		d.para("The validator warns no role unheard.")
	} else {
		d.para(fmt.Sprintf("%s the validator warns unheard: a role that something accepted relies on, while no record shows it was heard.", countText(len(unheard), "role", "roles")))
		for _, m := range unheard {
			d.line("- %s: %s [%s]", strings.TrimPrefix(m.Pointer, "/stakeholders/"), m.Message, m.ID)
		}
		d.blank()
	}

	d.section("Stakeholder list")
	if c := get(root, "stakeholdersComplete"); c != nil {
		says := fmt.Sprintf("Confirmed complete by %s on %s", str(c, "by"), str(c, "date"))
		if src := str(c, "source"); src != "" {
			says += ", from " + src
		}
		d.para(says + ".")
	} else {
		d.para(fmt.Sprintf("Not confirmed: no role has confirmed the list of %s complete, so a role nobody has named yet may still be missing.", countText(len(pairs(root, "stakeholders")), "stakeholder", "stakeholders")))
	}

	d.section("Effort")
	d.progressEffort(state)

	d.section("Burn-up")
	d.progressBurnUp(state, total)
	return d.String()
}

// stageSize is one row of the size table.
type stageSize struct {
	stage    string
	count    int
	sections string
}

// sizeByStage counts the elements of each stage that has sections, in
// life-cycle order: a section of named entries by its entries, a list by
// its items, and a section that is one element, such as the release, as
// one. Extension keys are not elements.
func sizeByStage(root *yaml.Node) []stageSize {
	var out []stageSize
	for _, stage := range spec.Stages {
		sections := spec.SectionsOf(stage)
		if len(sections) == 0 {
			continue
		}
		row := stageSize{stage: stage}
		var parts []string
		for _, s := range sections {
			n := sectionSize(root, s)
			if n == 0 {
				continue
			}
			row.count += n
			parts = append(parts, fmt.Sprintf("%s %d", s, n))
		}
		row.sections = strings.Join(parts, ", ")
		if row.sections == "" {
			row.sections = "none"
		}
		out = append(out, row)
	}
	return out
}

func sectionSize(root *yaml.Node, section string) int {
	n := get(root, section)
	if n == nil {
		return 0
	}
	if spec.IsSingleSection(section) || spec.IsElement([]string{section}) {
		return 1
	}
	switch n.Kind {
	case yaml.SequenceNode:
		return len(n.Content)
	case yaml.MappingNode:
		count := 0
		for _, p := range pairsOf(n) {
			if !spec.IsExtensionKey(p.Key.Value) {
				count++
			}
		}
		return count
	}
	return 1
}

// progressQuestions writes the open questions as a table of the deciding
// roles against the priorities.
func (d *doc) progressQuestions() {
	qs := d.questions
	if len(qs) == 0 {
		d.para("No question is open.")
		return
	}
	byRole := map[string]map[string]int{}
	var roles []string
	totals := map[string]int{}
	for _, q := range qs {
		role := str(q.node, "decidedBy")
		if byRole[role] == nil {
			byRole[role] = map[string]int{}
			roles = append(roles, role)
		}
		byRole[role][q.priority]++
		totals[q.priority]++
	}
	sort.Strings(roles)
	d.para(fmt.Sprintf("%s: %d must, %d should, %d could, by the role that decides each or must provide what it asks for. A must question holds up everything that reads what it blocks; a should question holds up code generation and approval; a could question holds up nothing. The questions document lists each one.",
		countText(len(qs), "open question", "open questions"), totals["must"], totals["should"], totals["could"]))
	d.line("| Decided by | Must | Should | Could | Total |")
	d.line("|---|---|---|---|---|")
	for _, r := range roles {
		c := byRole[r]
		d.line("| %s | %d | %d | %d | %d |", cell(r), c["must"], c["should"], c["could"], c["must"]+c["should"]+c["could"])
	}
	d.line("| Total | %d | %d | %d | %d |", totals["must"], totals["should"], totals["could"], len(qs))
	d.blank()
}

// hours sums the figures of one effort key exactly, as decimals, so 0.1
// and 0.2 make 0.3.
type hours struct {
	sum      big.Rat
	count    int
	decimals int
}

func (h *hours) add(text string) bool {
	var r big.Rat
	if _, ok := r.SetString(text); !ok {
		return false
	}
	h.sum.Add(&h.sum, &r)
	h.count++
	if _, frac, ok := strings.Cut(text, "."); ok && len(frac) > h.decimals {
		h.decimals = len(frac)
	}
	return true
}

func (h *hours) String() string {
	if h.sum.IsInt() {
		return h.sum.Num().String()
	}
	return h.sum.FloatString(h.decimals)
}

// progressEffort writes the effort each change request records, the word
// missing where it records none, the sum of each column over the requests
// that record it, and the requests with no effort, named.
func (d *doc) progressEffort(state *State) {
	changes := recordsOf(state, "change")
	if len(changes) == 0 {
		d.para("No change request is recorded.")
		return
	}
	var estimated, spent hours
	var missing []string
	type row struct{ id, title, status, estimated, spent string }
	var rows []row
	for _, c := range changes {
		r := row{id: str(c, "id"), title: str(c, "title"), status: str(c, "status"), estimated: "missing", spent: "missing"}
		effort := get(c, "effort")
		if effort == nil {
			missing = append(missing, r.id)
		}
		if v := str(effort, "estimatedHours"); v != "" && estimated.add(v) {
			r.estimated = v
		}
		if v := str(effort, "spentHours"); v != "" && spent.add(v) {
			r.spent = v
		}
		rows = append(rows, r)
	}
	n := len(changes)
	says := fmt.Sprintf("%s recorded beside the specification, with the effort each records in hours. ", countText(n, "change request is", "change requests are"))
	says += sumText("Estimated", &estimated, n) + " " + sumText("Spent", &spent, n)
	if len(missing) > 0 {
		says += fmt.Sprintf(" %d of %d %s no effort and %s counted as missing: %s. No figure is filled in.",
			len(missing), n, pick(len(missing) == 1, "records", "record"), pick(len(missing) == 1, "is", "are"), strings.Join(missing, ", "))
	} else {
		says += " Every change request records its effort."
	}
	d.para(says)
	d.line("| Change request | Status | Estimated hours | Spent hours |")
	d.line("|---|---|---|---|")
	for _, r := range rows {
		d.line("| %s, %s | %s | %s | %s |", r.id, cell(r.title), r.status, r.estimated, r.spent)
	}
	d.blank()
}

func sumText(label string, h *hours, n int) string {
	if h.count == 0 {
		return label + ": no change request records it."
	}
	return fmt.Sprintf("%s: %s hours over the %d of %d that record it.", label, h.String(), h.count, n)
}

func pick(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

// progressBurnUp writes one point per release, in version order: the
// distinct elements the change requests it includes name under affects
// adds and changes, cumulative from the first release, approved once the
// request's decision approves it and generated once it is implemented or
// released.
func (d *doc) progressBurnUp(state *State, scope int) {
	releases := recordsOf(state, "release")
	sort.SliceStable(releases, func(i, j int) bool {
		a, _ := semver.Parse(str(releases[i], "version"))
		b, _ := semver.Parse(str(releases[j], "version"))
		return semver.Compare(a, b) < 0
	})
	d.para(fmt.Sprintf("One point per release, oldest first. Each counts, from the first release to it, the distinct elements (requirement IDs and #/ pointers) that the change requests it includes name under affects adds and changes: approved once the request's decision approves it, generated once the request is implemented or released, so the specification and the code carry it. The specification holds %s now, the scope line. A release that includes no change request does not rise; the elements the first version was written with are counted under Size, not here.",
		countText(scope, "element", "elements")))
	if len(releases) == 0 {
		d.para("No release is recorded.")
		return
	}
	changes := map[string]*yaml.Node{}
	for _, c := range recordsOf(state, "change") {
		changes[str(c, "id")] = c
	}
	approved, generated := map[string]bool{}, map[string]bool{}
	d.line("| Release | Status | Date | Change requests | Elements approved | Elements generated | Elements in the specification |")
	d.line("|---|---|---|---|---|---|---|")
	for _, r := range releases {
		var ids []string
		for _, id := range strs(r, "includes") {
			c := changes[id]
			if c == nil {
				continue
			}
			ids = append(ids, id)
			status := str(c, "status")
			isApproved := str(get(c, "decision"), "outcome") == "approved" || status == "approved" || status == "implemented" || status == "released"
			isGenerated := status == "implemented" || status == "released"
			affects := get(c, "affects")
			for _, ref := range append(strs(affects, "adds"), strs(affects, "changes")...) {
				if state.Withheld != nil && state.Withheld.CoversRef(ref) {
					continue
				}
				if isApproved {
					approved[ref] = true
				}
				if isGenerated {
					generated[ref] = true
				}
			}
		}
		date := str(r, "date")
		if date == "" {
			date = "not yet"
		}
		list := strings.Join(ids, ", ")
		if list == "" {
			list = "none"
		}
		d.line("| %s | %s | %s | %s | %d | %d | %d |", str(r, "version"), str(r, "status"), date, list, len(approved), len(generated), scope)
	}
	d.blank()
}
