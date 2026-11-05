package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/disk"
	"github.com/SpecArch/specarch/internal/semver"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// runPlan works the work packages beside a specification (ADR-242,
// ADR-245, ADR-246). With no mode it proposes packages for the accepted
// requirements of a planned release that no package holds, in the order
// of the reference graph, and writes them as records with status
// proposed; --next prints the first package that can start, --brief one
// package's page for whoever takes it, --export the packages for a
// tracker, and --import writes back what the tracker owns. It works on a
// specification with errors too, after printing them (ADR-065), and exits
// 1 then.
func runPlan(args []string, stdout, stderr io.Writer) int {
	var (
		release, brief, export, importFile string
		next, dry                          bool
		rest                               []string
		modes                              []string
	)
	value := func(i *int, name string) (string, bool) {
		if *i+1 >= len(args) || strings.HasPrefix(args[*i+1], "--") {
			fmt.Fprintf(stderr, "specarch plan --%s needs a value\n\n%s", name, usage)
			return "", false
		}
		*i++
		return args[*i], true
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		var ok bool
		switch a {
		case "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
			continue
		case "--release":
			if release, ok = value(&i, "release"); !ok {
				return 2
			}
			continue
		case "--brief":
			if brief, ok = value(&i, "brief"); !ok {
				return 2
			}
			modes = append(modes, "--brief")
			continue
		case "--export":
			if export, ok = value(&i, "export"); !ok {
				return 2
			}
			modes = append(modes, "--export")
			continue
		case "--import":
			if importFile, ok = value(&i, "import"); !ok {
				return 2
			}
			modes = append(modes, "--import")
			continue
		case "--next":
			next = true
			modes = append(modes, "--next")
			continue
		case "--dry-run":
			dry = true
			continue
		}
		if len(a) > 1 && a[0] == '-' {
			fmt.Fprintf(stderr, "specarch plan has no option %s; its options are --release, --next, --brief, --export, --import and --dry-run\n\n%s", a, usage)
			return 2
		}
		rest = append(rest, a)
	}
	switch {
	case len(modes) > 1:
		fmt.Fprintf(stderr, "specarch plan takes one of --next, --brief, --export and --import at a time, and was given %s\n\n%s", joinAnd(modes), usage)
		return 2
	case release != "" && len(modes) > 0:
		fmt.Fprintf(stderr, "specarch plan --release chooses the release to propose packages for, and goes with no %s\n\n%s", modes[0], usage)
		return 2
	case dry && (next || brief != "" || export != ""):
		fmt.Fprintf(stderr, "specarch plan %s writes nothing, so it takes no --dry-run\n\n%s", modes[0], usage)
		return 2
	case export != "" && export != "csv" && export != "json":
		fmt.Fprintf(stderr, "specarch plan --export takes csv or json, not %s\n\n%s", export, usage)
		return 2
	case len(rest) != 1:
		fmt.Fprintf(stderr, "specarch plan needs one folder, the specification's\n\n%s", usage)
		return 2
	}
	folder := rest[0]
	specs, invalid, status := readSpecs([]string{folder}, "plan", stdout, stderr)
	if status != 0 {
		return status
	}
	if len(specs) != 1 {
		fmt.Fprintf(stderr, "specarch plan: %s holds %d specifications; name the folder of the one the work packages are beside\n", folder, len(specs))
		return 2
	}
	l := specs[0]
	plan := validate.PlanWorkPackages(l.spec)
	if plan == nil {
		fmt.Fprintf(stderr, "specarch plan: nothing of %s could be read, so no requirement can be planned; fix the errors above first\n", l.spec.RootFile)
		return 1
	}
	p := &planner{l: l, root: l.spec.Root, plan: plan, folder: folder}
	sort.SliceStable(p.plan.Packages, func(i, j int) bool { return naturalLess(p.plan.Packages[i].ID, p.plan.Packages[j].ID) })
	switch {
	case next:
		status = p.next(stdout)
	case brief != "":
		status = p.brief(brief, stdout, stderr)
	case export != "":
		status = p.export(export, stdout, stderr)
	case importFile != "":
		status = p.importFrom(importFile, disk.For(dry, stdout), stdout, stderr)
	default:
		status = p.propose(release, disk.For(dry, stdout), stdout, stderr)
	}
	if invalid && status == 0 {
		fmt.Fprintln(stderr, "specarch plan: the specification has errors; they are printed above")
		return 1
	}
	return status
}

type planner struct {
	l      loaded
	root   *yaml.Node
	plan   *validate.WorkPackagePlan
	folder string
}

func (p *planner) requirement(id string) *yaml.Node {
	return source.Child(source.Child(p.root, "requirements"), id)
}

func (p *planner) pkg(id string) *validate.WorkPackage {
	for i := range p.plan.Packages {
		if p.plan.Packages[i].ID == id {
			return &p.plan.Packages[i]
		}
	}
	return nil
}

// elementName is an element's pointer said as words: its section and
// its key, the key unescaped, such as entities Member or paths /loans.
func elementName(ptr string) string {
	tokens := strings.SplitN(strings.TrimPrefix(ptr, "/"), "/", 2)
	for i, t := range tokens {
		tokens[i] = strings.ReplaceAll(strings.ReplaceAll(t, "~1", "/"), "~0", "~")
	}
	return strings.Join(tokens, " ")
}

// --- Proposing packages ---

// group is one proposed package while it is made: the requirements it
// holds, the elements that meet them and the packages it comes after.
type group struct {
	reqs     []string
	elements []string
	first    int // the document position of its first requirement
	after    []string
	needs    map[int]bool // the groups of its release it needs, by index
	id       string
}

// propose writes a record with status proposed for each package it
// proposes: per release, the accepted requirements of the release that no
// package that is not withdrawn holds, grouped so requirements met by the
// same element go together and ordered by the reference graph, a package
// after the packages that build what it needs.
func (p *planner) propose(release string, w *disk.Writer, stdout, stderr io.Writer) int {
	releases := map[string]string{} // version -> status
	var versions []string
	for _, r := range parsedRecords(p.l.spec)["release"] {
		v := source.Str(source.Child(r, "version"))
		if _, seen := releases[v]; v != "" && !seen {
			releases[v] = source.Str(source.Child(r, "status"))
			versions = append(versions, v)
		}
	}
	sort.SliceStable(versions, func(i, j int) bool {
		a, okA := semver.Parse(versions[i])
		b, okB := semver.Parse(versions[j])
		if okA && okB {
			return semver.Compare(a, b) < 0
		}
		return versions[i] < versions[j]
	})
	var chosen []string
	if release != "" {
		if _, ok := releases[release]; !ok {
			have := "there is no release record"
			if len(versions) > 0 {
				have = "the release records are " + strings.Join(versions, ", ")
			}
			fmt.Fprintf(stderr, "specarch plan: release %s has no record, and a package names only a recorded release; %s; record it under %s first\n", release, have, filepath.ToSlash(filepath.Join(p.l.spec.RecordsDir, "releases", release+".yaml")))
			return 1
		}
		chosen = []string{release}
	} else {
		for _, v := range versions {
			if releases[v] == "planned" {
				chosen = append(chosen, v)
			}
		}
		if len(chosen) == 0 {
			fmt.Fprintln(stdout, "Nothing to plan: no release record is planned. Record the release the work is for under records/releases/ with status planned, or name one with --release.")
			return 0
		}
	}
	position := map[string]int{}
	for i, r := range source.Pairs(source.Child(p.root, "requirements")) {
		position[r.Key.Value] = i
	}
	// satisfiers is, per element, the requirements it satisfies.
	satisfiers := map[string][]string{}
	for req, elements := range p.plan.Satisfied {
		for _, e := range elements {
			satisfiers[e] = append(satisfiers[e], req)
		}
	}
	for e := range satisfiers {
		sort.Slice(satisfiers[e], func(i, j int) bool { return position[satisfiers[e][i]] < position[satisfiers[e][j]] })
	}
	ids := make([]string, 0, len(p.plan.Packages))
	for _, wp := range p.plan.Packages {
		ids = append(ids, wp.ID)
	}
	// builder is the package that builds an element: a recorded package
	// not withdrawn first, then one proposed for an earlier release.
	proposedBuilds := map[string]string{}
	builder := func(element string) string {
		for _, req := range satisfiers[element] {
			if h := p.plan.Holders[req]; len(h) > 0 {
				return h[0]
			}
		}
		return proposedBuilds[element]
	}
	var written []*group
	if release == "" {
		// A requirement naming no release takes its package's (ADR-245),
		// so only a release named on the command line plans it.
		var unreleased []string
		for _, r := range source.Pairs(source.Child(p.root, "requirements")) {
			if source.Str(source.Child(r.Value, "release")) == "" && source.Str(source.Child(r.Value, "status")) == "accepted" && len(p.plan.Holders[r.Key.Value]) == 0 {
				unreleased = append(unreleased, r.Key.Value)
			}
		}
		if len(unreleased) > 0 {
			fmt.Fprintf(stdout, "Left out, since %s no release and only --release says which one %s for: %s.\n", pick(len(unreleased) == 1, "this accepted requirement names", "these accepted requirements name"), pick(len(unreleased) == 1, "it is", "they are"), strings.Join(unreleased, ", "))
		}
	}
	for _, v := range chosen {
		var reqs, skipped []string
		for _, r := range source.Pairs(source.Child(p.root, "requirements")) {
			rv := source.Str(source.Child(r.Value, "release"))
			if (rv != v && (rv != "" || release != v)) || len(p.plan.Holders[r.Key.Value]) > 0 {
				continue
			}
			if st := source.Str(source.Child(r.Value, "status")); st != "accepted" {
				skipped = append(skipped, fmt.Sprintf("%s (%s)", r.Key.Value, orWord(st, "no status")))
				continue
			}
			reqs = append(reqs, r.Key.Value)
		}
		if len(skipped) > 0 {
			fmt.Fprintf(stdout, "Release %s: left out, since only an accepted requirement is planned: %s.\n", v, strings.Join(skipped, ", "))
		}
		if len(reqs) == 0 {
			fmt.Fprintf(stdout, "Release %s: nothing to plan; every accepted requirement of it is in a work package that is not withdrawn.\n", v)
			continue
		}
		groups := groupRequirements(reqs, p.plan.Satisfied, position)
		inGroup := map[string]int{}
		for gi, g := range groups {
			for _, e := range g.elements {
				inGroup[e] = gi
			}
		}
		for _, g := range groups {
			g.needs = map[int]bool{}
			for _, e := range g.elements {
				for _, n := range p.plan.Needs[e] {
					if slicesContains(g.elements, n) {
						continue
					}
					hi, ours := inGroup[n]
					if b := builder(n); b != "" {
						g.after = appendOnce(g.after, b) // a recorded package, or one proposed for an earlier release
					} else if ours {
						g.needs[hi] = true
					}
				}
			}
		}
		ordered := orderGroups(groups)
		for _, g := range ordered {
			g.id = nextRecordID(ids, "WP")
			ids = append(ids, g.id)
			for _, e := range g.elements {
				if _, ok := proposedBuilds[e]; !ok {
					proposedBuilds[e] = g.id
				}
			}
		}
		for _, g := range ordered {
			for hi := range g.needs {
				g.after = appendOnce(g.after, ordered[hi].id)
			}
			sort.SliceStable(g.after, func(i, j int) bool { return naturalLess(g.after[i], g.after[j]) })
			line := fmt.Sprintf("%s: %s", g.id, strings.Join(g.reqs, ", "))
			if len(g.after) > 0 {
				line += "; after " + strings.Join(g.after, ", ")
			}
			fmt.Fprintln(stdout, line+"; "+proposalTitle(g))
			if err := p.writeProposal(g, v, w); err != nil {
				fmt.Fprintf(stderr, "specarch plan: cannot write %s: %v\n", g.id, err)
				return 2
			}
			written = append(written, g)
		}
	}
	if len(written) == 0 {
		return 0
	}
	verb := "written"
	if w.Dry() {
		verb = "would be written; nothing written (--dry-run)"
	}
	fmt.Fprintf(stderr, "specarch plan: %s proposed under %s, %s; accept each by setting its status to planned, or split or merge them first\n", plural(len(written), "work package"), filepath.ToSlash(filepath.Join(p.l.spec.RecordsDir, "work-packages")), verb)
	return 0
}

// groupRequirements puts requirements met by a common element into one
// group; a requirement no element meets is a group of its own.
func groupRequirements(reqs []string, satisfied map[string][]string, position map[string]int) []*group {
	parent := map[string]string{}
	var find func(string) string
	find = func(x string) string {
		if parent[x] == x {
			return x
		}
		parent[x] = find(parent[x])
		return parent[x]
	}
	for _, r := range reqs {
		parent[r] = r
	}
	owner := map[string]string{}
	for _, r := range reqs {
		for _, e := range satisfied[r] {
			if !builds(e) {
				continue // a decision explains a requirement; nothing of it is built
			}
			if o, ok := owner[e]; ok {
				parent[find(r)] = find(o)
			} else {
				owner[e] = r
			}
		}
	}
	byRoot := map[string]*group{}
	var groups []*group
	for _, r := range reqs {
		root := find(r)
		g := byRoot[root]
		if g == nil {
			g = &group{first: position[r]}
			byRoot[root] = g
			groups = append(groups, g)
		}
		g.reqs = append(g.reqs, r)
		for _, e := range satisfied[r] {
			g.elements = appendOnce(g.elements, e)
		}
	}
	return groups
}

// orderGroups merges the groups that need each other, directly or
// through others, into one, and orders the rest so a group comes after
// the groups it needs; of the groups that can come next, the one whose
// first requirement comes first in the specification.
func orderGroups(groups []*group) []*group {
	// Tarjan's strongly connected components.
	index, low := map[int]int{}, map[int]int{}
	on := map[int]bool{}
	var stack []int
	comp := map[int]int{}
	var comps [][]int
	counter := 0
	var strong func(int)
	strong = func(v int) {
		index[v], low[v] = counter, counter
		counter++
		stack = append(stack, v)
		on[v] = true
		for _, w := range sortedInts(groups[v].needs) {
			if _, seen := index[w]; !seen {
				strong(w)
				low[v] = min(low[v], low[w])
			} else if on[w] {
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] == index[v] {
			var c []int
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				on[w] = false
				comp[w] = len(comps)
				c = append(c, w)
				if w == v {
					break
				}
			}
			comps = append(comps, c)
		}
	}
	for v := range groups {
		if _, seen := index[v]; !seen {
			strong(v)
		}
	}
	merged := make([]*group, len(comps))
	for ci, c := range comps {
		sort.Slice(c, func(i, j int) bool { return groups[c[i]].first < groups[c[j]].first })
		m := &group{first: groups[c[0]].first, needs: map[int]bool{}}
		for _, gi := range c {
			g := groups[gi]
			m.reqs = append(m.reqs, g.reqs...)
			for _, e := range g.elements {
				m.elements = appendOnce(m.elements, e)
			}
			for _, a := range g.after {
				m.after = appendOnce(m.after, a)
			}
			for hi := range g.needs {
				if comp[hi] != ci {
					m.needs[comp[hi]] = true
				}
			}
		}
		merged[ci] = m
	}
	// Order the merged groups; their needs now index merged.
	done := map[int]bool{}
	var out []*group
	for len(out) < len(merged) {
		best := -1
		for ci, m := range merged {
			if done[ci] {
				continue
			}
			ready := true
			for hi := range m.needs {
				if !done[hi] {
					ready = false
					break
				}
			}
			if ready && (best < 0 || m.first < merged[best].first) {
				best = ci
			}
		}
		done[best] = true
		out = append(out, merged[best])
	}
	return renumber(out, merged)
}

// renumber makes each group's needs index the list it is in.
func renumber(out, merged []*group) []*group {
	at := map[*group]int{}
	for i, g := range out {
		at[g] = i
	}
	for _, g := range out {
		needs := map[int]bool{}
		for hi := range g.needs {
			needs[at[merged[hi]]] = true
		}
		g.needs = needs
	}
	return out
}

func sortedInts(m map[int]bool) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

func appendOnce(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}

func orWord(s, none string) string {
	if s == "" {
		return none
	}
	return s
}

// builds says whether an element is something a package builds: every
// element but a decision, which explains a requirement.
func builds(element string) bool { return !strings.HasPrefix(element, "/decisions/") }

// built is the elements of a group a package builds, said as words.
func built(g *group) []string {
	var names []string
	for _, e := range g.elements {
		if builds(e) {
			names = append(names, elementName(e))
		}
	}
	return names
}

func slicesContains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func proposalTitle(g *group) string {
	names := built(g)
	if len(names) == 0 {
		return strings.Join(g.reqs, ", ") + ", which no design element meets yet"
	}
	if len(names) > 3 {
		names = append(names[:3], fmt.Sprintf("%d more", len(names)-3))
	}
	return "Build " + joinAnd(names)
}

func (p *planner) writeProposal(g *group, release string, w *disk.Writer) error {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	add := func(key string, n *yaml.Node) { m.Content = append(m.Content, scalarNode(key), n) }
	flow := func(list []string) *yaml.Node {
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		for _, v := range list {
			seq.Content = append(seq.Content, scalarNode(v))
		}
		return seq
	}
	add("specarchRecord", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "1.0.0", Style: yaml.DoubleQuotedStyle})
	add("kind", scalarNode("work-package"))
	add("id", scalarNode(g.id))
	add("release", scalarNode(release))
	add("title", scalarNode(proposalTitle(g)))
	add("requirements", flow(g.reqs))
	if len(g.after) > 0 {
		add("after", flow(g.after))
	}
	add("status", scalarNode("proposed"))
	why := "Proposed by specarch plan: "
	if names := built(g); len(names) == 0 {
		why += "no design element meets " + pick(len(g.reqs) == 1, "this requirement", "these requirements") + " yet, so nothing orders " + pick(len(g.reqs) == 1, "it", "them") + "."
	} else {
		why += pick(len(g.reqs) == 1, "the requirement", "the requirements") + " met by " + joinAnd(names)
		if len(g.after) > 0 {
			why += ", after the packages that build what " + pick(len(names) == 1, "it needs", "they need")
		}
		why += "."
	}
	add("why", scalarNode(why))
	var b bytes.Buffer
	b.WriteString("# Proposed by specarch plan. Accept it by setting status to planned, or split or merge it first.\n")
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	path := filepath.Join(p.l.spec.RecordsDir, "work-packages", g.id+".yaml")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s exists although no work-package record carries %s; read it, and name it after the record it holds or remove it", filepath.ToSlash(path), g.id)
	}
	return w.WriteFile(path, b.Bytes(), 0o644)
}

func pick(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

// --- The next package ---

// waits is why a package cannot start: each package it comes after that
// is neither done nor withdrawn, then what the readiness check finds.
func (p *planner) waits(wp *validate.WorkPackage) []string {
	var out []string
	for _, a := range wp.After {
		other := p.pkg(a)
		switch {
		case other == nil:
			out = append(out, fmt.Sprintf("it comes after %s, which has no record", a))
		case other.Status != "done" && other.Status != "withdrawn":
			out = append(out, fmt.Sprintf("it comes after %s, which is %s", a, other.Status))
		}
	}
	return append(out, wp.NotReady...)
}

// next prints the first package, by ID, that is planned or ready, whose
// after packages are done and which passes the readiness check.
func (p *planner) next(stdout io.Writer) int {
	var candidates []*validate.WorkPackage
	for i := range p.plan.Packages {
		if st := p.plan.Packages[i].Status; st == "planned" || st == "ready" {
			candidates = append(candidates, &p.plan.Packages[i])
		}
	}
	if len(candidates) == 0 {
		fmt.Fprintf(stdout, "No work package is planned or ready. specarch plan %s proposes packages, and a person sets each one to planned.\n", p.folder)
		return 1
	}
	for _, wp := range candidates {
		if len(p.waits(wp)) > 0 {
			continue
		}
		fmt.Fprintf(stdout, "Next: %s, %s\n\n", wp.ID, wp.Title)
		fmt.Fprintf(stdout, "- Release: %s\n", orWord(wp.Release, "none named"))
		fmt.Fprintf(stdout, "- Status: %s\n", wp.Status)
		fmt.Fprintf(stdout, "- Assigned to: %s\n", orWord(wp.AssignedTo, "nobody named"))
		fmt.Fprintf(stdout, "- Requirements: %s\n", strings.Join(wp.Requirements, ", "))
		if len(wp.After) > 0 {
			var after []string
			for _, a := range wp.After {
				after = append(after, fmt.Sprintf("%s (%s)", a, p.pkg(a).Status))
			}
			fmt.Fprintf(stdout, "- After: %s\n", strings.Join(after, ", "))
		}
		fmt.Fprintf(stdout, "\nIts brief: specarch plan --brief %s %s\n", wp.ID, p.folder)
		return 0
	}
	fmt.Fprintf(stdout, "No work package can start now: %s, and each waits.\n\n", pick(len(candidates) == 1, "1 package is planned or ready", fmt.Sprintf("%d packages are planned or ready", len(candidates))))
	for _, wp := range candidates {
		fmt.Fprintf(stdout, "- %s, %s: %s\n", wp.ID, wp.Title, strings.Join(p.waits(wp), "; "))
	}
	return 1
}

// --- The brief ---

// brief prints one page for whoever takes a package, a programmer or an
// agent alike: whether it can start, its requirements with their
// acceptance criteria, the elements to build, the tests that must pass,
// the goals it serves, what is out of scope near it, the quality goals in
// rank order, and what it changes in operation.
func (p *planner) brief(id string, stdout, stderr io.Writer) int {
	wp := p.pkg(id)
	if wp == nil {
		var ids []string
		for _, w := range p.plan.Packages {
			ids = append(ids, w.ID)
		}
		have := "there is no work-package record"
		if len(ids) > 0 {
			have = "the work packages are " + strings.Join(ids, ", ")
		}
		fmt.Fprintf(stderr, "specarch plan: there is no work package %s beside %s; %s\n", id, p.l.spec.Dir, have)
		return 1
	}
	out := &strings.Builder{}
	line := func(format string, args ...any) { fmt.Fprintf(out, format+"\n", args...) }
	line("# %s, %s", wp.ID, wp.Title)
	line("")
	facts := fmt.Sprintf("Release %s, status %s, assigned to %s.", orWord(wp.Release, "none named"), wp.Status, orWord(wp.AssignedTo, "nobody named"))
	if wp.Why != "" {
		facts += " " + oneParagraph(wp.Why)
	}
	line("%s", facts)
	line("")
	line("## Can it start")
	line("")
	if wp.Status == "withdrawn" {
		line("No: it is withdrawn, part of no plan.")
	} else if wp.Status == "done" || wp.Status == "in-progress" {
		line("It is %s already.", wp.Status)
	} else if waits := p.waits(wp); len(waits) == 0 {
		line("Yes: every package it comes after is done, and the readiness check finds nothing.")
	} else {
		line("Not yet:")
		line("")
		for _, w := range waits {
			line("- %s", w)
		}
	}
	line("")
	line("## Requirements")
	var needs []string
	for _, req := range wp.Requirements {
		line("")
		n := p.requirement(req)
		if n == nil {
			line("### %s", req)
			line("")
			line("Not a requirement of this specification; read it in the requirement set it belongs to.")
			continue
		}
		line("### %s", req)
		line("")
		line("%s", oneParagraph(source.Str(source.Child(n, "statement"))))
		line("")
		line("Priority %s, status %s.", orWord(source.Str(source.Child(n, "priority")), "not given"), orWord(source.Str(source.Child(n, "status")), "not given"))
		if acc := source.Items(source.Child(n, "acceptance")); len(acc) > 0 {
			line("")
			line("Acceptance:")
			line("")
			for _, a := range acc {
				line("- %s", oneParagraph(source.Str(a)))
			}
		} else {
			line("")
			line("Acceptance: none written, so no test can show it is met.")
		}
		for _, item := range source.Items(source.Child(n, "needs")) {
			needs = appendOnce(needs, source.Str(item))
		}
	}
	line("")
	line("## Design elements to build")
	line("")
	var elements []string
	for _, req := range wp.Requirements {
		for _, e := range p.plan.Satisfied[req] {
			elements = appendOnce(elements, e)
		}
	}
	if len(elements) == 0 {
		line("None: no element of the design satisfies its requirements yet.")
	}
	for _, e := range elements {
		var meets []string
		for _, req := range wp.Requirements {
			for _, x := range p.plan.Satisfied[req] {
				if x == e {
					meets = append(meets, req)
				}
			}
		}
		line("- %s, meets %s", e, strings.Join(meets, ", "))
	}
	line("")
	line("## Tests that must pass")
	line("")
	held := map[string]bool{}
	for _, req := range wp.Requirements {
		held[req] = true
	}
	tests := 0
	for _, t := range source.Pairs(source.Child(p.root, "tests")) {
		var verifies []string
		for _, v := range source.Items(source.Child(t.Value, "verifies")) {
			if held[v.Value] {
				verifies = append(verifies, v.Value)
			}
		}
		if len(verifies) == 0 {
			continue
		}
		tests++
		text := fmt.Sprintf("- %s (%s), verifies %s", t.Key.Value, orWord(source.Str(source.Child(t.Value, "scenario")), "no scenario"), strings.Join(verifies, ", "))
		if then := source.Str(source.Child(t.Value, "then")); then != "" {
			text += ": then " + oneParagraph(then)
		}
		line("%s", text)
	}
	if tests == 0 {
		line("None: no test verifies its requirements yet.")
	}
	line("")
	line("## Goals it serves")
	line("")
	var goals []string
	for _, need := range needs {
		for _, g := range source.Items(source.Child(source.Child(source.Child(p.root, "needs"), need), "goals")) {
			goals = appendOnce(goals, source.Str(g))
		}
	}
	if len(goals) == 0 {
		line("None named: no need its requirements refine names a goal.")
	}
	for _, g := range goals {
		n := source.Child(source.Child(p.root, "goals"), g)
		text := fmt.Sprintf("- %s: %s", g, oneParagraph(source.Str(source.Child(n, "statement"))))
		if m := source.Str(source.Child(n, "measure")); m != "" {
			text += fmt.Sprintf(" Measure: %s; baseline %s, target %s.", strings.TrimSuffix(oneParagraph(m), "."), orWord(source.Str(source.Child(n, "baseline")), "not given"), orWord(source.Str(source.Child(n, "target")), "not given"))
		}
		line("%s", text)
	}
	line("")
	line("## Out of scope near it")
	line("")
	// An entry is near when it rules out a need the requirements refine,
	// or a need serving a goal the package serves.
	var near, elsewhere []string
	for _, o := range source.Pairs(source.Child(p.root, "outOfScope")) {
		var why []string
		for _, n := range source.Items(source.Child(o.Value, "needs")) {
			if slicesContains(needs, n.Value) {
				why = append(why, n.Value+", which its requirements refine")
				continue
			}
			var common []string
			for _, g := range source.Items(source.Child(source.Child(source.Child(p.root, "needs"), n.Value), "goals")) {
				if slicesContains(goals, g.Value) {
					common = append(common, g.Value)
				}
			}
			if len(common) > 0 {
				why = append(why, fmt.Sprintf("%s, which serves %s as this package does", n.Value, joinAnd(common)))
			}
		}
		if len(why) == 0 {
			elsewhere = append(elsewhere, o.Key.Value)
			continue
		}
		near = append(near, fmt.Sprintf("- %s: %s It rules out %s.", o.Key.Value, oneParagraph(source.Str(source.Child(o.Value, "statement"))), joinAnd(why)))
	}
	if len(near) == 0 {
		line("None rules out a need its requirements refine or a need serving a goal it serves.")
	}
	for _, n := range near {
		line("%s", n)
	}
	if len(elsewhere) > 0 {
		line("")
		line("Out of scope elsewhere: %s.", strings.Join(elsewhere, ", "))
	}
	line("")
	line("## Quality goals, in rank order")
	line("")
	qgs := source.Pairs(source.Child(p.root, "qualityGoals"))
	sort.SliceStable(qgs, func(i, j int) bool {
		a, _ := strconv.Atoi(source.Str(source.Child(qgs[i].Value, "rank")))
		b, _ := strconv.Atoi(source.Str(source.Child(qgs[j].Value, "rank")))
		return a < b
	})
	if len(qgs) == 0 {
		line("None written: no rank decides a trade-off.")
	}
	for _, q := range qgs {
		line("%s. %s, %s: %s", source.Str(source.Child(q.Value, "rank")), q.Key.Value, source.Str(source.Child(q.Value, "quality")), oneParagraph(source.Str(source.Child(q.Value, "statement"))))
	}
	line("")
	line("## What it changes in operation")
	line("")
	reach := append([]string{}, elements...)
	for _, e := range elements {
		for _, n := range p.plan.Needs[e] {
			reach = appendOnce(reach, n)
		}
	}
	for _, m := range source.Pairs(source.Child(p.root, "monitors")) {
		for _, key := range []string{"verifies", "satisfies"} {
			for _, v := range source.Items(source.Child(m.Value, key)) {
				if held[v.Value] {
					reach = appendOnce(reach, "/monitors/"+m.Key.Value)
				}
			}
		}
	}
	any := false
	for _, s := range []struct{ section, label string }{{"configuration", "Settings"}, {"migrations", "Migrations"}, {"jobs", "Jobs"}, {"processes", "Processes"}, {"monitors", "Monitors"}} {
		var names []string
		for _, e := range reach {
			if name, ok := strings.CutPrefix(e, "/"+s.section+"/"); ok {
				names = appendOnce(names, strings.ReplaceAll(strings.ReplaceAll(name, "~1", "/"), "~0", "~"))
			}
		}
		if len(names) > 0 {
			any = true
			line("- %s: %s", s.label, strings.Join(names, ", "))
		}
	}
	if !any {
		line("Nothing: it adds no setting, migration, job, process or monitor.")
	}
	fmt.Fprint(stdout, out.String())
	return 0
}

// --- The tracker ---

// trackerColumns are the columns of an export, in order. The
// specification owns the scope and the order; the tracker owns the
// status, the assignee and the effort.
var trackerColumns = []string{"id", "title", "release", "status", "assignedTo", "estimatedHours", "spentHours", "after", "requirements"}

var specOwned = map[string]bool{"title": true, "release": true, "after": true, "requirements": true}

func (p *planner) export(format string, stdout, stderr io.Writer) int {
	if format == "csv" {
		cw := csv.NewWriter(stdout)
		cw.Write(trackerColumns)
		for _, wp := range p.plan.Packages {
			cw.Write([]string{wp.ID, wp.Title, wp.Release, wp.Status, wp.AssignedTo, wp.EstimatedHours, wp.SpentHours, strings.Join(wp.After, ", "), strings.Join(wp.Requirements, ", ")})
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			fmt.Fprintf(stderr, "specarch plan: cannot write the CSV: %v\n", err)
			return 2
		}
		return 0
	}
	type row struct {
		ID             string          `json:"id"`
		Title          string          `json:"title"`
		Release        string          `json:"release"`
		Status         string          `json:"status"`
		AssignedTo     string          `json:"assignedTo"`
		EstimatedHours json.RawMessage `json:"estimatedHours"`
		SpentHours     json.RawMessage `json:"spentHours"`
		After          []string        `json:"after"`
		Requirements   []string        `json:"requirements"`
	}
	hours := func(s string) json.RawMessage {
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return json.RawMessage("null")
		}
		return json.RawMessage(s)
	}
	out := struct {
		WorkPackages []row `json:"workPackages"`
	}{WorkPackages: []row{}}
	for _, wp := range p.plan.Packages {
		r := row{ID: wp.ID, Title: wp.Title, Release: wp.Release, Status: wp.Status, AssignedTo: wp.AssignedTo, EstimatedHours: hours(wp.EstimatedHours), SpentHours: hours(wp.SpentHours), After: wp.After, Requirements: wp.Requirements}
		if r.After == nil {
			r.After = []string{}
		}
		if r.Requirements == nil {
			r.Requirements = []string{}
		}
		out.WorkPackages = append(out.WorkPackages, r)
	}
	writeJSON(stdout, out)
	return 0
}

// trackerRow is one package as the tracker file gives it: the columns it
// has, by name, and where it stands in the file.
type trackerRow struct {
	at     string
	values map[string]string
}

var workPackageStatuses = []string{"proposed", "planned", "ready", "in-progress", "done", "withdrawn"}

// readTracker reads a tracker file written as --export writes it, CSV or
// JSON by its extension; a list in a CSV cell is its IDs split at commas.
func readTracker(path string, data []byte) ([]trackerRow, []string, error) {
	var rows []trackerRow
	var ignored []string
	known := map[string]bool{}
	for _, c := range trackerColumns {
		known[c] = true
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		cr := csv.NewReader(bytes.NewReader(data))
		cr.FieldsPerRecord = -1
		records, err := cr.ReadAll()
		if err != nil {
			return nil, nil, err
		}
		if len(records) == 0 {
			return nil, nil, fmt.Errorf("it has no header row; its first row names the columns, as specarch plan --export csv writes them")
		}
		header := records[0]
		hasID := false
		for _, h := range header {
			if h == "id" {
				hasID = true
			} else if !known[h] {
				ignored = append(ignored, h)
			}
		}
		if !hasID {
			return nil, nil, fmt.Errorf("its header row has no id column, and a row is matched to its package by id")
		}
		for i, rec := range records[1:] {
			r := trackerRow{at: fmt.Sprintf("%s:%d", filepath.ToSlash(path), i+2), values: map[string]string{}}
			for c, h := range header {
				if known[h] && c < len(rec) {
					r.values[h] = rec[c]
				}
			}
			rows = append(rows, r)
		}
	case ".json":
		var doc struct {
			WorkPackages []map[string]any `json:"workPackages"`
		}
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		if err := dec.Decode(&doc); err != nil {
			return nil, nil, err
		}
		if doc.WorkPackages == nil {
			return nil, nil, fmt.Errorf("it has no workPackages list, as specarch plan --export json writes it")
		}
		seen := map[string]bool{}
		for i, obj := range doc.WorkPackages {
			r := trackerRow{at: fmt.Sprintf("%s: workPackages[%d]", filepath.ToSlash(path), i), values: map[string]string{}}
			for k, v := range obj {
				if !known[k] {
					if !seen[k] {
						seen[k] = true
						ignored = append(ignored, k)
					}
					continue
				}
				switch t := v.(type) {
				case nil:
					r.values[k] = ""
				case string:
					r.values[k] = t
				case json.Number:
					r.values[k] = t.String()
				case []any:
					var parts []string
					for _, x := range t {
						parts = append(parts, fmt.Sprint(x))
					}
					r.values[k] = strings.Join(parts, ", ")
				default:
					r.values[k] = fmt.Sprint(t)
				}
			}
			rows = append(rows, r)
		}
		sort.Strings(ignored)
	default:
		return nil, nil, fmt.Errorf("its name ends in neither .csv nor .json, and plan reads the file by that ending")
	}
	return rows, ignored, nil
}

func splitIDs(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// importFrom writes into each work-package record what the tracker owns
// and the file changes: status, assignedTo and effort. A field the
// specification owns that the file changes is a warning naming both
// values, and the record keeps its own.
func (p *planner) importFrom(path string, w *disk.Writer, stdout, stderr io.Writer) int {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "specarch plan: cannot read %s: %s\n", path, spec.PlainIOError(err))
		return 2
	}
	rows, ignored, err := readTracker(path, data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch plan: cannot read %s as a tracker file: %v\n", path, err)
		return 2
	}
	warnings, updated := 0, 0
	warn := func(at, format string, args ...any) {
		warnings++
		fmt.Fprintf(stdout, "%s: warning: %s\n", at, fmt.Sprintf(format, args...))
	}
	if len(ignored) > 0 {
		warn(filepath.ToSlash(path), "%s %s %s not one plan reads, so %s left out; plan reads %s", pick(len(ignored) == 1, "column", "columns"), strings.Join(ignored, ", "), pick(len(ignored) == 1, "is", "are"), pick(len(ignored) == 1, "it is", "they are"), strings.Join(trackerColumns, ", "))
	}
	seen := map[string]bool{}
	for _, r := range rows {
		id := strings.TrimSpace(r.values["id"])
		if id == "" {
			warn(r.at, "the row has no id, so it matches no work package; give it the package's ID")
			continue
		}
		if seen[id] {
			warn(r.at, "%s is in the file twice, and only its first row is read; keep one row per package", id)
			continue
		}
		seen[id] = true
		wp := p.pkg(id)
		if wp == nil {
			warn(r.at, "%s is not a work package beside the specification, and the specification owns which packages there are, so it is not added; propose it with specarch plan or write its record, then export again", id)
			continue
		}
		mine := map[string]string{"title": wp.Title, "release": wp.Release, "after": strings.Join(wp.After, ", "), "requirements": strings.Join(wp.Requirements, ", ")}
		for _, key := range trackerColumns {
			v, ok := r.values[key]
			if !ok || !specOwned[key] {
				continue
			}
			theirs := strings.TrimSpace(v)
			if key == "after" || key == "requirements" {
				theirs = strings.Join(splitIDs(v), ", ")
			}
			if theirs != mine[key] {
				warn(r.at, "%s %s: the tracker has %q and the specification %q; the specification owns %s, so the record keeps %q; if the tracker is right, change it in %s and export again", id, key, theirs, mine[key], key, mine[key], filepath.ToSlash(wp.Path))
			}
		}
		changes, setters := []string{}, []func(*yaml.Node){}
		if v, ok := r.values["status"]; ok {
			v = strings.TrimSpace(v)
			switch {
			case v == wp.Status:
			case !oneOfValues(workPackageStatuses, v):
				warn(r.at, "%s status: the tracker has %q, which is not a status of a work package, so the record keeps %q; use one of %s", id, v, wp.Status, strings.Join(workPackageStatuses, ", "))
			default:
				changes = append(changes, fmt.Sprintf("status from %s to %s", wp.Status, v))
				setters = append(setters, func(root *yaml.Node) { setScalar(root, "status", v, "!!str") })
			}
		}
		if v, ok := r.values["assignedTo"]; ok {
			v = strings.TrimSpace(v)
			if v != wp.AssignedTo {
				changes = append(changes, fmt.Sprintf("assignedTo from %s to %s", orWord(wp.AssignedTo, "nobody"), orWord(v, "nobody")))
				setters = append(setters, func(root *yaml.Node) {
					if v == "" {
						removeKey(root, "assignedTo")
					} else {
						setScalar(root, "assignedTo", v, "!!str")
					}
				})
			}
		}
		for _, h := range []struct{ key, have string }{{"estimatedHours", wp.EstimatedHours}, {"spentHours", wp.SpentHours}} {
			v, ok := r.values[h.key]
			if !ok {
				continue
			}
			v = strings.TrimSpace(v)
			if v == h.have {
				continue
			}
			if v != "" {
				if f, err := strconv.ParseFloat(v, 64); err != nil || f <= 0 {
					warn(r.at, "%s %s: the tracker has %q, which is not a positive number of hours, so the record keeps %q; give the hours as a number above 0, or leave the cell empty", id, h.key, v, orWord(h.have, "none"))
					continue
				}
			}
			key := h.key
			changes = append(changes, fmt.Sprintf("%s from %s to %s", key, orWord(h.have, "none"), orWord(v, "none")))
			setters = append(setters, func(root *yaml.Node) { setHours(root, key, v) })
		}
		if len(changes) == 0 {
			continue
		}
		fmt.Fprintf(stdout, "%s (%s): %s\n", id, filepath.ToSlash(wp.Path), strings.Join(changes, ", "))
		if err := rewriteRecord(wp.Path, setters, w); err != nil {
			fmt.Fprintf(stderr, "specarch plan: cannot write %s: %v\n", wp.Path, err)
			return 2
		}
		updated++
	}
	verb := "written"
	if w.Dry() {
		verb = "would be written; nothing written (--dry-run)"
	}
	fmt.Fprintf(stderr, "specarch plan: %s from %s %s, %s\n", plural(updated, "work package"), filepath.ToSlash(path), verb, plural(warnings, "warning"))
	return 0
}

func setScalar(root *yaml.Node, key, value, tag string) {
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			root.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
			return
		}
	}
	root.Content = append(root.Content, scalarNode(key), &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value})
}

func removeKey(root *yaml.Node, key string) {
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			root.Content = append(root.Content[:i], root.Content[i+2:]...)
			return
		}
	}
}

// setHours sets or, for "", removes one hours key of the effort, and the
// effort with it when nothing is left.
func setHours(root *yaml.Node, key, value string) {
	var effort *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "effort" && root.Content[i+1].Kind == yaml.MappingNode {
			effort = root.Content[i+1]
		}
	}
	if value == "" {
		if effort != nil {
			removeKey(effort, key)
			if len(effort.Content) == 0 {
				removeKey(root, "effort")
			}
		}
		return
	}
	if effort == nil {
		effort = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content, scalarNode("effort"), effort)
	}
	tag := "!!float"
	if _, err := strconv.Atoi(value); err == nil {
		tag = "!!int"
	}
	setScalar(effort, key, value, tag)
}

// rewriteRecord applies the setters to a record's mapping and writes it
// back, its comments and the order of its keys kept.
func rewriteRecord(path string, setters []func(*yaml.Node), w *disk.Writer) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("it is not a mapping")
	}
	for _, set := range setters {
		set(doc.Content[0])
	}
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return w.WriteFile(path, b.Bytes(), 0o644)
}
