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
// records beside it (ADR-237): its size by stage, the open questions by
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

	d.line(tr("# %s: progress"), str(info, "title"))
	d.blank()
	d.para(fmt.Sprintf(tr("Version %s of the specification: %s, %s, from the specification and the records beside it. Every figure is counted from what the specification and its records hold; no estimate is made, and an effort nobody recorded is shown as missing."),
		str(info, "version"), countText(total, tr("element"), tr("elements")), countText(len(d.questions), tr("open question"), tr("open questions"))))
	if state.Withheld != nil {
		d.para(tr("This document is for everyone: the internal elements, and the questions only about them, are not counted."))
	}
	d.stakeholdersNotice()

	d.section(tr("Size"))
	d.para(tr("The elements of each stage, a section of named elements counted by its entries and a section that holds one thing, such as the release, as one. The open questions are counted under Open work, not here."))
	d.line(tr("| Stage | Elements | Sections |"))
	d.line("|---|---|---|")
	for _, s := range size {
		d.line("| %s | %d | %s |", s.stage, s.count, cell(s.sections))
	}
	d.line(tr("| Total | %d | |"), total)
	d.blank()

	if len(pairs(root, "goals")) > 0 {
		d.section(tr("Goals"))
		d.progressGoals(root, state)
	}

	d.section(tr("Open work"))
	d.progressQuestions()

	d.section(tr("Unheard roles"))
	var unheard []Mark
	for _, m := range state.Marks {
		if m.Rule == "unheard" && strings.HasPrefix(m.Pointer, "/stakeholders/") {
			unheard = append(unheard, m)
		}
	}
	if len(unheard) == 0 {
		d.para(tr("The validator warns no role unheard."))
	} else {
		d.para(fmt.Sprintf(tr("%s the validator warns unheard: a role that something accepted relies on, while no record shows it was heard."), countText(len(unheard), tr("role"), tr("roles"))))
		for _, m := range unheard {
			d.line("- %s: %s [%s]", strings.TrimPrefix(m.Pointer, tr("/stakeholders/")), m.Message, m.ID)
		}
		d.blank()
	}

	d.section(tr("Stakeholder list"))
	d.progressStakeholderList(root, state)

	d.section(tr("Effort"))
	d.progressEffort(state)

	d.section(tr("Burn-up"))
	d.progressBurnUp(root, state, total)
	return d.String()
}

// progressStakeholderList says whether a role has confirmed the stakeholder
// list complete, as stakeholdersComplete records it (ADR-234): who, on what
// day and, when it names one, in what source, as written, with each problem
// the validator marks on the key; or, with no key, that it is not confirmed,
// naming the open must questions that hold the list up, since the validator
// does not warn while one does.
func (d *doc) progressStakeholderList(root *yaml.Node, state *State) {
	list := countText(len(pairs(root, "stakeholders")), tr("stakeholder"), tr("stakeholders"))
	if complete := get(root, "stakeholdersComplete"); complete != nil {
		line := fmt.Sprintf(tr("Confirmed: %s confirmed the list of %s complete on %s"), orNone(str(complete, "by"), tr("no role named")), list, orNone(str(complete, "date"), tr("no date written")))
		if src := str(complete, "source"); src != "" {
			line += tr(", recorded in the source ") + src
		}
		d.para(line + ".")
		var marks []Mark
		for _, m := range state.Marks {
			if m.Pointer == "/stakeholdersComplete" || strings.HasPrefix(m.Pointer, "/stakeholdersComplete/") {
				marks = append(marks, m)
			}
		}
		if len(marks) > 0 {
			d.para(fmt.Sprintf(tr("The validator marks the confirmation with %s:"), countText(len(marks), tr("problem"), tr("problems"))))
			for _, m := range marks {
				d.line("- %s %s: %s [%s]", m.Severity, m.Pointer, m.Message, m.ID)
			}
			d.blank()
		}
		return
	}
	var holding []string
	for _, q := range d.questions {
		if q.priority != "must" {
			continue
		}
		for _, b := range q.blocks {
			if !b.IsPointer() && (b.Section == "stakeholders" || b.Section == "" && b.Stage == spec.Sections["stakeholders"]) {
				holding = append(holding, q.id)
				break
			}
		}
	}
	if len(holding) > 0 {
		d.para(fmt.Sprintf(tr("Not confirmed: the open must %s %s %s up the list of %s until %s answered, so a role nobody has named yet may still be missing."),
			pick(len(holding) == 1, tr("question"), tr("questions")), strings.Join(holding, ", "), pick(len(holding) == 1, tr("holds"), tr("hold")), list, pick(len(holding) == 1, tr("it is"), tr("they are"))))
		return
	}
	d.para(fmt.Sprintf(tr("Not confirmed: the specification records no role confirming the list of %s complete, so a role nobody has named yet may still be missing."), list))
}

func orNone(s, none string) string {
	if s == "" {
		return none
	}
	return s
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
			row.sections = tr("none")
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
		d.para(tr("No question is open."))
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
			written := tr("missing")
			if n := source.Deref(get(q.node, "priority")); n != nil {
				written = valueText(n)
			}
			odd = append(odd, fmt.Sprintf(tr("- %s: priority %s (not counted as must, should or could; one of them is expected)"), q.id, cell(written)))
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
	says := fmt.Sprintf(tr("%s: %d must, %d should, %d could"), countText(len(qs), tr("open question"), tr("open questions")), totals["must"], totals["should"], totals["could"])
	if totals[other] > 0 {
		says += fmt.Sprintf(tr(", %d with another priority"), totals[other])
	}
	d.para(says + tr(", by the role that decides each or must provide what it asks for. A must question holds up everything that reads what it blocks; a should question holds up code generation and approval; a could question holds up nothing. The questions document lists each one."))
	if totals[other] > 0 {
		d.line(tr("| Decided by | Must | Should | Could | Other priority | Total |"))
		d.line("|---|---|---|---|---|---|")
	} else {
		d.line(tr("| Decided by | Must | Should | Could | Total |"))
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
		label := r
		if r == noRole {
			label = tr("no role named (decidedBy missing)")
		}
		row(cell(label), c, c["must"]+c["should"]+c["could"]+c[other])
	}
	row(tr("Total"), totals, len(qs))
	d.blank()
	if len(odd) > 0 {
		d.para(fmt.Sprintf(tr("%s a priority that is not must, should or could:"), countText(len(odd), tr("question has"), tr("questions have"))))
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
		return tr("missing")
	}
	var r big.Rat
	tag := n.ShortTag()
	if n.Kind != yaml.ScalarNode || tag != "!!int" && tag != "!!float" {
		return fmt.Sprintf(tr("%s (not counted: not a number; a positive number of hours is expected)"), cell(valueText(n)))
	}
	if _, ok := r.SetString(n.Value); !ok || r.Sign() <= 0 {
		return fmt.Sprintf(tr("%s (not counted: not above 0; a positive number of hours is expected)"), cell(n.Value))
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
		return tr("a list")
	case yaml.MappingNode:
		return tr("a mapping")
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
		d.para(tr("No change request is recorded."))
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
	says := fmt.Sprintf(tr("%s recorded beside the specification, with the effort each records in hours. "), countText(n, tr("change request is"), tr("change requests are")))
	says += sumText(tr("Estimated"), &estimated, n) + " " + sumText(tr("Spent"), &spent, n)
	if len(missing) > 0 {
		says += fmt.Sprintf(tr(" %d of %d %s no effort and %s counted as missing: %s. No figure is filled in."),
			len(missing), n, pick(len(missing) == 1, tr("records"), tr("record")), pick(len(missing) == 1, tr("is"), tr("are")), strings.Join(missing, ", "))
	} else {
		says += tr(" Every change request records its effort.")
	}
	d.para(says)
	d.line(tr("| Change request | Status | Estimated hours | Spent hours |"))
	d.line("|---|---|---|---|")
	for _, r := range rows {
		d.line("| %s, %s | %s | %s | %s |", r.id, cell(r.title), r.status, r.estimated, r.spent)
	}
	d.blank()
}

func sumText(label string, h *hours, n int) string {
	if h.count == 0 {
		return label + tr(": no change request records it.")
	}
	return fmt.Sprintf(tr("%s: %s hours over the %d of %d that record it."), label, h.String(), h.count, n)
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
		return "", tr("names no section that Size counts")
	}
	if spec.IsSingleSection(section) || spec.IsElement(tokens[:1]) {
		tokens = tokens[:1]
	} else {
		if len(tokens) < 2 {
			return "", tr("names a whole section, not one element")
		}
		if spec.IsExtensionKey(tokens[1]) {
			return "", tr("names an extension key, not an element")
		}
		tokens = tokens[:2]
	}
	if _, ok := source.Resolve(root, tokens); !ok {
		return "", tr("names an element the specification does not hold")
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
	d.para(fmt.Sprintf(tr("One point per release, oldest first. Each counts, from the first release to it, the distinct elements that the change requests it includes name under affects adds and changes, a requirement named by its ID or its pointer once and a pointer inside an element as that element, and a reference to what Size does not count named below the table and left out: approved once the request's decision approves it and it is not withdrawn or rejected, generated once the request is implemented or released, so the specification and the code carry it. The specification holds %s now, the scope line. A release that includes no change request does not rise; the elements the first version was written with are counted under Size, not here."),
		countText(scope, tr("element"), tr("elements"))))
	if len(releases) == 0 {
		d.para(tr("No release is recorded."))
		return
	}
	changes := map[string]*yaml.Node{}
	for _, c := range recordsOf(state, "change") {
		changes[str(c, "id")] = c
	}
	approved, generated := map[string]bool{}, map[string]bool{}
	seen := map[string]bool{}
	var notCounted []string
	d.line(tr("| Release | Status | Date | Change requests | Elements approved | Elements generated | Elements in the specification |"))
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
						notCounted = append(notCounted, fmt.Sprintf(tr("- %s names %s: not counted, it %s."), id, cell(ref), why))
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
			date = tr("not yet")
		}
		list := strings.Join(ids, ", ")
		if list == "" {
			list = tr("none")
		}
		d.line("| %s | %s | %s | %s | %d | %d | %d |", str(r, "version"), str(r, "status"), date, list, len(approved), len(generated), scope)
	}
	d.blank()
	if len(notCounted) > 0 {
		d.para(fmt.Sprintf(tr("%s under affects %s not counted, since Size does not count what %s:"), countText(len(notCounted), tr("reference"), tr("references")), pick(len(notCounted) == 1, tr("is"), tr("are")), pick(len(notCounted) == 1, tr("it names"), tr("they name"))))
		for _, l := range notCounted {
			d.line("%s", l)
		}
		d.blank()
	}
}
