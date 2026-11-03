package generate

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/semver"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// Progress writes the progress report from the specification and the
// records beside it (ADR-235): its size by stage, the open questions by
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
	d.para(fmt.Sprintf("Not confirmed: the specification records no role confirming the list of %s complete, so a role nobody has named yet may still be missing.", countText(len(pairs(root, "stakeholders")), "stakeholder", "stakeholders")))

	d.section("Effort")
	d.progressEffort(state)

	d.section("Burn-up")
	d.progressBurnUp(root, state, total)
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
// roles against the priorities. A question whose priority is not must,
// should or could, which the schema refuses, is counted under Other
// priority and named below the table with the value as written, and a
// question with no decidedBy is counted under a row that says so, so every
// question is in one cell of its row and the rows add up to the total.
func (d *doc) progressQuestions() {
	qs := d.questions
	if len(qs) == 0 {
		d.para("No question is open.")
		return
	}
	const noRole = "no role named (decidedBy missing)"
	const other = "other"
	byRole := map[string]map[string]int{}
	var roles []string
	totals := map[string]int{}
	var odd []string
	for _, q := range qs {
		role := str(q.node, "decidedBy")
		if role == "" {
			role = noRole
		}
		if byRole[role] == nil {
			byRole[role] = map[string]int{}
			roles = append(roles, role)
		}
		priority := q.priority
		if priority != "must" && priority != "should" && priority != "could" {
			priority = other
			written := "missing"
			if n := source.Deref(get(q.node, "priority")); n != nil {
				written = valueText(n)
			}
			odd = append(odd, fmt.Sprintf("- %s: priority %s (not counted as must, should or could; one of them is expected)", q.id, cell(written)))
		}
		byRole[role][priority]++
		totals[priority]++
	}
	// The row of questions with no role named comes last.
	sort.Slice(roles, func(i, j int) bool {
		if (roles[i] == noRole) != (roles[j] == noRole) {
			return roles[j] == noRole
		}
		return roles[i] < roles[j]
	})
	says := fmt.Sprintf("%s: %d must, %d should, %d could", countText(len(qs), "open question", "open questions"), totals["must"], totals["should"], totals["could"])
	if totals[other] > 0 {
		says += fmt.Sprintf(", %d with another priority", totals[other])
	}
	d.para(says + ", by the role that decides each or must provide what it asks for. A must question holds up everything that reads what it blocks; a should question holds up code generation and approval; a could question holds up nothing. The questions document lists each one.")
	if totals[other] > 0 {
		d.line("| Decided by | Must | Should | Could | Other priority | Total |")
		d.line("|---|---|---|---|---|---|")
	} else {
		d.line("| Decided by | Must | Should | Could | Total |")
		d.line("|---|---|---|---|---|")
	}
	row := func(label string, c map[string]int, total int) {
		if totals[other] > 0 {
			d.line("| %s | %d | %d | %d | %d | %d |", label, c["must"], c["should"], c["could"], c[other], total)
			return
		}
		d.line("| %s | %d | %d | %d | %d |", label, c["must"], c["should"], c["could"], total)
	}
	for _, r := range roles {
		c := byRole[r]
		row(cell(r), c, c["must"]+c["should"]+c["could"]+c[other])
	}
	row("Total", totals, len(qs))
	d.blank()
	if len(odd) > 0 {
		d.para(fmt.Sprintf("%s a priority that is not must, should or could:", countText(len(odd), "question has", "questions have")))
		for _, l := range odd {
			d.line("%s", l)
		}
		d.blank()
	}
}

// hours sums the figures of one effort key exactly, as rationals, so 0.1
// and 0.2 make 0.3.
type hours struct {
	sum   big.Rat
	count int
}

// add counts the value of one effort key when it is a number above 0, as
// the schema asks, and returns the cell the table shows for it: the figure,
// missing when the key is absent, or the value as written with why it is
// not counted. A value the schema refuses is marked here and kept out of
// the sum, never read as a figure.
func (h *hours) add(n *yaml.Node) string {
	n = source.Deref(n)
	if n == nil {
		return "missing"
	}
	var r big.Rat
	tag := n.ShortTag()
	if n.Kind != yaml.ScalarNode || tag != "!!int" && tag != "!!float" {
		return fmt.Sprintf("%s (not counted: not a number; a positive number of hours is expected)", cell(valueText(n)))
	}
	if _, ok := r.SetString(n.Value); !ok || r.Sign() <= 0 {
		return fmt.Sprintf("%s (not counted: not above 0; a positive number of hours is expected)", cell(n.Value))
	}
	h.sum.Add(&h.sum, &r)
	h.count++
	return ratText(&r)
}

func (h *hours) String() string { return ratText(&h.sum) }

// valueText is a value as written, a scalar's text or the kind of a list
// or mapping.
func valueText(n *yaml.Node) string {
	switch n.Kind {
	case yaml.SequenceNode:
		return "a list"
	case yaml.MappingNode:
		return "a mapping"
	}
	if n.ShortTag() == "!!str" {
		return fmt.Sprintf("%q", n.Value)
	}
	return n.Value
}

// ratText writes a number in decimal with as many decimals as it needs, so
// 1e-1 reads 0.1 and 3.250 reads 3.25.
func ratText(r *big.Rat) string {
	if r.IsInt() {
		return r.Num().String()
	}
	scaled := new(big.Rat).Set(r)
	ten := big.NewRat(10, 1)
	for places := 1; places <= 30; places++ {
		scaled.Mul(scaled, ten)
		if scaled.IsInt() {
			return r.FloatString(places)
		}
	}
	return r.FloatString(30)
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
		effort := get(c, "effort")
		counted := estimated.count + spent.count
		r := row{id: str(c, "id"), title: str(c, "title"), status: str(c, "status"),
			estimated: estimated.add(get(effort, "estimatedHours")), spent: spent.add(get(effort, "spentHours"))}
		// A request records its effort when one of its figures is counted:
		// an effort with only extension keys, or with values that are not
		// a positive number, records none and is named as missing.
		if estimated.count+spent.count == counted {
			missing = append(missing, r.id)
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

// elementOf is the element a reference names, as Size counts it, so one
// element is counted once however it is named: a requirement ID is the
// requirement's pointer, and a pointer into an element is cut to the
// element, the entry of a section or a single section. A reference Size
// does not count, because it names no stage's section (a question), an
// extension key, a whole section, or an element the specification does not
// hold, such as a requirement of another specification, returns "" and
// why, so the burn-up names it rather than counting it past the scope line.
func elementOf(root *yaml.Node, ref string) (string, string) {
	var tokens []string
	if !strings.HasPrefix(ref, "#/") {
		tokens = []string{"requirements", ref}
	} else {
		for _, t := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			tokens = append(tokens, source.UnescapeToken(t))
		}
	}
	section := tokens[0]
	if spec.Sections[section] == "" || spec.IsExtensionKey(section) {
		return "", "names no section that Size counts"
	}
	if spec.IsSingleSection(section) || spec.IsElement(tokens[:1]) {
		tokens = tokens[:1]
	} else {
		if len(tokens) < 2 {
			return "", "names a whole section, not one element"
		}
		if spec.IsExtensionKey(tokens[1]) {
			return "", "names an extension key, not an element"
		}
		tokens = tokens[:2]
	}
	if _, ok := source.Resolve(root, tokens); !ok {
		return "", "names an element the specification does not hold"
	}
	return source.Pointer(tokens...), ""
}

// progressBurnUp writes one point per release, in version order: the
// distinct elements the change requests it includes name under affects
// adds and changes, each as the element it names, cumulative from the
// first release, approved once the request's decision approves it and it
// is not withdrawn or rejected, and generated once it is implemented or
// released. A reference that names no element Size counts is listed below
// the table with why, and not counted.
func (d *doc) progressBurnUp(root *yaml.Node, state *State, scope int) {
	releases := recordsOf(state, "release")
	sort.SliceStable(releases, func(i, j int) bool {
		a, _ := semver.Parse(str(releases[i], "version"))
		b, _ := semver.Parse(str(releases[j], "version"))
		return semver.Compare(a, b) < 0
	})
	d.para(fmt.Sprintf("One point per release, oldest first. Each counts, from the first release to it, the distinct elements that the change requests it includes name under affects adds and changes, a requirement named by its ID or its pointer once and a pointer inside an element as that element, and a reference to what Size does not count named below the table and left out: approved once the request's decision approves it and it is not withdrawn or rejected, generated once the request is implemented or released, so the specification and the code carry it. The specification holds %s now, the scope line. A release that includes no change request does not rise; the elements the first version was written with are counted under Size, not here.",
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
	seen := map[string]bool{}
	var notCounted []string
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
			isApproved := status != "withdrawn" && status != "rejected" && (str(get(c, "decision"), "outcome") == "approved" || status == "approved" || status == "implemented" || status == "released")
			isGenerated := status == "implemented" || status == "released"
			affects := get(c, "affects")
			for _, ref := range append(strs(affects, "adds"), strs(affects, "changes")...) {
				if ref == "" || state.Withheld != nil && state.Withheld.CoversRef(ref) {
					continue
				}
				element, why := elementOf(root, ref)
				if element == "" {
					key := id + " " + ref
					if !seen[key] {
						seen[key] = true
						notCounted = append(notCounted, fmt.Sprintf("- %s names %s: not counted, it %s.", id, cell(ref), why))
					}
					continue
				}
				if isApproved {
					approved[element] = true
				}
				if isGenerated {
					generated[element] = true
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
	if len(notCounted) > 0 {
		d.para(fmt.Sprintf("%s under affects %s not counted, since Size does not count what %s:", countText(len(notCounted), "reference", "references"), pick(len(notCounted) == 1, "is", "are"), pick(len(notCounted) == 1, "it names", "they name")))
		for _, l := range notCounted {
			d.line("%s", l)
		}
		d.blank()
	}
}
