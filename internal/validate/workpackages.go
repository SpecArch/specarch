package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// The work-package records (ADR-245): one piece of the delivery each,
// naming the requirements it holds, the release it belongs to and the
// packages it comes after. Its design elements are those that satisfy its
// requirements and its tests those that verify them. A cycle of after
// links is an error at each link on it; every other rule is a warning at
// the record, and a withdrawn package is held to none of them.

// packageRequirements checks that each requirement a work package names is
// a requirement of the specification, or of a declared requirement set,
// named by its ID.
func (rc *recordChecks) packageRequirements(r *record) {
	for i, item := range source.Items(source.Child(r.root, "requirements")) {
		v := source.Str(item)
		ptr := source.Pointer("requirements", fmt.Sprint(i))
		switch {
		case v == "":
		case strings.HasPrefix(v, "#/"):
			r.c.add(item, ptr, RuleRecordRef, "%s is a pointer, and a work package names requirements by their ID only, its design elements being those that satisfy them; name each requirement by its ID", v)
		case rc.d.resolve(v) == refMissing:
			r.c.add(item, ptr, RuleRecordRef, "%s is not a requirement of the specification%s", v, suggest(v, rc.d.requirements))
		}
	}
}

// packagePlan is what the work-package rules read: the packages that are
// not withdrawn, the packages that hold each requirement, the design
// elements that satisfy each requirement, and the elements each element
// names, from the reference graph the diagrams walk.
type packagePlan struct {
	known     map[string]*record
	live      []*record           // the packages not withdrawn, in path order
	holders   map[string][]string // requirement ID -> the IDs of the live packages that hold it
	satisfied map[string][]string // requirement ID -> the elements that satisfy it, in document order
	satisfies map[string][]string // element -> the requirement IDs it satisfies
	needs     map[string][]string // element -> the other elements it names, in document order
}

// elementOfPointer is the element a pointer into the specification is in:
// the section for a section that holds one thing, otherwise the section
// and the entry's key; "" for a pointer to a whole section.
func elementOfPointer(ptr string) string {
	tokens := strings.Split(strings.TrimPrefix(ptr, "/"), "/")
	if len(tokens) == 0 || tokens[0] == "" {
		return ""
	}
	if spec.IsSingleSection(tokens[0]) {
		return "/" + tokens[0]
	}
	if len(tokens) < 2 {
		return ""
	}
	return "/" + tokens[0] + "/" + tokens[1]
}

func appendOnce(list []string, s string) []string {
	if contains(list, s) {
		return list
	}
	return append(list, s)
}

func (rc *recordChecks) plan(recs []*record) *packagePlan {
	pl := &packagePlan{known: rc.set["work-package"], holders: map[string][]string{}, satisfied: map[string][]string{}, satisfies: map[string][]string{}, needs: map[string][]string{}}
	for _, r := range recs {
		if r.kind != "work-package" || r.str("status") == "withdrawn" {
			continue
		}
		pl.live = append(pl.live, r)
		for _, item := range source.Items(source.Child(r.root, "requirements")) {
			if v := source.Str(item); v != "" {
				pl.holders[v] = appendOnce(pl.holders[v], r.str("id"))
			}
		}
	}
	walk(rc.d.root, nil, func(n *yaml.Node, path []string) {
		items := source.Items(source.Child(n, "satisfies"))
		if len(items) == 0 {
			return
		}
		element := elementOfPointer(source.Pointer(path...))
		if element == "" {
			return
		}
		for _, item := range items {
			pl.satisfied[item.Value] = appendOnce(pl.satisfied[item.Value], element)
			pl.satisfies[element] = appendOnce(pl.satisfies[element], item.Value)
		}
	})
	for _, ref := range ModelReferences(rc.d.root) {
		if _, everywhere := anywhere[ref.Through]; everywhere {
			continue // satisfies, verifies and decidedIn name what the element meets, not what it needs
		}
		if pointsBack(rc.d.root, ref.From) {
			continue
		}
		from, to := elementOfPointer(ref.From), elementOfPointer(ref.To)
		if from == "" || to == "" || from == to {
			continue
		}
		pl.needs[from] = appendOnce(pl.needs[from], to)
	}
	return pl
}

// pointsBack reports whether a reference is the target of a one-to-many or
// many-to-many relation: the related rows hold the key, or a table between
// them does, so the entity does not need its target to be built first.
func pointsBack(root *yaml.Node, from string) bool {
	tokens := Tokens(from)
	if len(tokens) != 5 || tokens[0] != "entities" || tokens[2] != "relations" || tokens[4] != "target" {
		return false
	}
	rel := source.Child(source.Child(source.Child(source.Child(root, "entities"), tokens[1]), "relations"), tokens[3])
	kind := source.Str(source.Child(rel, "kind"))
	return kind == "one-to-many" || kind == "many-to-many"
}

// afterChain is the chain of after links from one package back to
// another, the packages on it from the first to the last, or nil when
// there is none; seen keeps the walk out of a cycle that does not reach
// the last.
func afterChain(from, to string, known map[string]*record, seen map[string]bool) []string {
	if from == to {
		return []string{to}
	}
	if seen[from] || known[from] == nil {
		return nil
	}
	seen[from] = true
	for _, link := range source.Items(source.Child(known[from].root, "after")) {
		if rest := afterChain(link.Value, to, known, seen); rest != nil {
			return append([]string{from}, rest...)
		}
	}
	return nil
}

func afterChainText(id string, chain []string) string {
	if len(chain) == 1 {
		return id + " comes after itself"
	}
	return id + " comes after " + strings.Join(chain, ", which comes after ") + ", so the chain comes back to " + id
}

// before is the package and every package its after chain reaches.
func (pl *packagePlan) before(id string) map[string]bool {
	out := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if out[id] {
			return
		}
		out[id] = true
		if r := pl.known[id]; r != nil {
			for _, link := range source.Items(source.Child(r.root, "after")) {
				visit(link.Value)
			}
		}
	}
	visit(id)
	return out
}

// builtIn is the live packages that hold a requirement an element
// satisfies, in the order the requirements and packages come.
func (pl *packagePlan) builtIn(element string) []string {
	var out []string
	for _, req := range pl.satisfies[element] {
		for _, id := range pl.holders[req] {
			out = appendOnce(out, id)
		}
	}
	return out
}

// checkWorkPackages applies the work-package rules of ADR-245 across the
// records: the cycle, then per live package its order, overlap, release
// and configuration, and the entry and exit criteria of a package marked
// ready or done.
func (rc *recordChecks) checkWorkPackages(recs []*record) {
	pl := rc.plan(recs)
	for _, r := range recs {
		if r.kind != "work-package" {
			continue
		}
		id := r.str("id")
		for i, link := range source.Items(source.Child(r.root, "after")) {
			if pl.known[link.Value] == nil {
				continue
			}
			if chain := afterChain(link.Value, id, pl.known, map[string]bool{}); chain != nil {
				r.c.add(link, source.Pointer("after", fmt.Sprint(i)), RuleWorkPackageCycle, "%s; a package comes only after packages that can be done before it, so take one link out", afterChainText(id, chain))
			}
		}
	}
	if len(pl.live) == 0 {
		return
	}
	deployed, anyValue := rc.deployedSettings()
	for _, r := range pl.live {
		id := r.str("id")
		before := pl.before(id)
		release := r.str("release")
		misordered := false
		for i, item := range source.Items(source.Child(r.root, "requirements")) {
			req := source.Str(item)
			ptr := source.Pointer("requirements", fmt.Sprint(i))
			if req == "" {
				continue
			}
			if others := without(pl.holders[req], id); len(others) > 0 {
				r.c.warn(item, ptr, RuleWorkPackageOverlap, "requirement %s is also in %s, and a requirement belongs to one package; split it into one requirement per package, or take it out of all but one", req, strings.Join(others, ", "))
			}
			node := rc.d.requirements[req]
			if node == nil {
				continue // a requirement of a requirement set, or none: the reference is checked above
			}
			if rv := source.Str(source.Child(node, "release")); release != "" && rv != "" && rv != release {
				r.c.warn(item, ptr, RuleWorkPackageRelease, "requirement %s is meant for release %s and %s for release %s; move the requirement to a package of release %s, or change its release", req, rv, id, release, rv)
			}
			for _, m := range pl.misorders(req, before) {
				misordered = true
				r.c.warn(item, ptr, RuleWorkPackageOrder, "%s", m.text(id))
			}
			if anyValue {
				for _, setting := range pl.settingsOf(req) {
					name := strings.TrimPrefix(setting, "/configuration/")
					node := rc.d.settings[name]
					if node == nil || source.Str(source.Child(node, "secret")) == "true" || source.Child(source.Child(node, "schema"), "default") != nil || deployed[name] {
						continue
					}
					r.c.warn(item, ptr, RuleWorkPackageUnconfigured, "requirement %s needs setting %s, which has no default and no deployment of an implementation file gives a value, so the package cannot run once installed; give it a value under configuration in each deployment, or a default in its schema", req, name)
				}
			}
		}
		switch r.str("status") {
		case "ready":
			if reasons := rc.notReady(r, pl, misordered); len(reasons) > 0 {
				r.c.warn(source.Child(r.root, "status"), "/status", RuleWorkPackageNotReady, "%s is marked ready, but %s; settle %s, or set status to planned", id, strings.Join(reasons, "; "), pick(len(reasons) == 1, "it", "them"))
			}
		case "done":
			if tests := rc.unverified(r, recs); len(tests) > 0 {
				r.c.warn(source.Child(r.root, "status"), "/status", RuleWorkPackageUnverified, "%s is marked done, but %s %s %s no passed result in the test-run records; record a run that passes with specarch log test-run, or set status to in-progress", id, pick(len(tests) == 1, "the test", "the tests"), joinAnd(tests), pick(len(tests) == 1, "has", "have"))
			}
		}
	}
}

// misorder is one element a package's requirement needs that only
// packages outside its after chain build.
type misorder struct {
	requirement, element, need string
	builtIn                    []string
}

func (m misorder) text(id string) string {
	return fmt.Sprintf("requirement %s is met by %s, which needs %s, and only %s %s it, outside the after chain of %s; add %s to after, or move what %s needs into this package", m.requirement, m.element, m.need, strings.Join(m.builtIn, ", "), pick(len(m.builtIn) == 1, "builds", "build"), id, m.builtIn[0], m.element)
}

// misorders is what work_package_order warns at a requirement of a
// package whose after chain, the package included, is before.
func (pl *packagePlan) misorders(req string, before map[string]bool) []misorder {
	var out []misorder
	for _, element := range pl.satisfied[req] {
		for _, need := range pl.needs[element] {
			in := pl.builtIn(need)
			if len(in) == 0 || anyIn(in, before) {
				continue
			}
			out = append(out, misorder{req, element, need, in})
		}
	}
	return out
}

func without(list []string, s string) []string {
	var out []string
	for _, v := range list {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}

func anyIn(list []string, set map[string]bool) bool {
	for _, v := range list {
		if set[v] {
			return true
		}
	}
	return false
}

func pick(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

// settingsOf is the settings a requirement adds: a setting that satisfies
// it, and a setting an element that satisfies it names, such as a
// process's configuration.
func (pl *packagePlan) settingsOf(req string) []string {
	var out []string
	for _, element := range pl.satisfied[req] {
		if strings.HasPrefix(element, "/configuration/") {
			out = appendOnce(out, element)
		}
		for _, need := range pl.needs[element] {
			if strings.HasPrefix(need, "/configuration/") {
				out = appendOnce(out, need)
			}
		}
	}
	return out
}

// deployedSettings is the settings some deployment of an implementation
// file gives a value, and whether any deployment gives one at all.
func (rc *recordChecks) deployedSettings() (map[string]bool, bool) {
	out := map[string]bool{}
	if rc.d.spec == nil {
		return out, false
	}
	for _, impl := range rc.d.spec.Implementations {
		doc := source.Parse(impl.Data)
		for _, dep := range source.Pairs(source.Child(doc.Root, "deployments")) {
			for _, cfg := range source.Pairs(source.Child(dep.Value, "configuration")) {
				out[cfg.Key.Value] = true
			}
		}
	}
	return out, len(out) > 0
}

// notReady is why a package marked ready does not meet QA's entry
// criterion, in the order of its requirements: a requirement not accepted
// or without acceptance criteria, an open question that blocks an element
// satisfying one, and the order warned above.
func (rc *recordChecks) notReady(r *record, pl *packagePlan, misordered bool) []string {
	var reasons []string
	for _, item := range source.Items(source.Child(r.root, "requirements")) {
		req := source.Str(item)
		node := rc.d.requirements[req]
		if node == nil {
			continue
		}
		if st := source.Str(source.Child(node, "status")); st != "accepted" {
			reasons = append(reasons, fmt.Sprintf("requirement %s is %s, not accepted", req, orText(st, "without a status")))
		}
		if source.Child(node, "acceptance") == nil {
			reasons = append(reasons, fmt.Sprintf("requirement %s has no acceptance criteria", req))
		}
		for _, element := range pl.satisfied[req] {
			for _, q := range source.Pairs(source.Child(rc.d.root, "questions")) {
				for _, b := range source.Items(source.Child(q.Value, "blocks")) {
					bl, ok := spec.ParseBlock(b.Value)
					if !ok || !bl.IsPointer() {
						continue
					}
					if at := source.Pointer(bl.Tokens...); at == element || strings.HasPrefix(at, element+"/") {
						reasons = appendOnce(reasons, fmt.Sprintf("question %s is open on %s, which meets requirement %s", q.Key.Value, element, req))
						break
					}
				}
			}
		}
	}
	if misordered {
		reasons = append(reasons, "it comes before a package that builds what it needs (work_package_order)")
	}
	return reasons
}

func orText(s, none string) string {
	if s == "" {
		return none
	}
	return s
}

// unverified is the golden and red tests that verify a requirement of the
// package and have no passed result in any test-run record, in the order
// of the tests; a test marked not applicable is not run, so it is left
// out.
func (rc *recordChecks) unverified(r *record, recs []*record) []string {
	held := map[string]bool{}
	for _, item := range source.Items(source.Child(r.root, "requirements")) {
		held[source.Str(item)] = true
	}
	passed := map[string]bool{}
	for _, run := range recs {
		if run.kind != "test-run" {
			continue
		}
		for _, res := range source.Items(source.Child(run.root, "results")) {
			if source.Str(source.Child(res, "outcome")) == "passed" {
				passed[source.Str(source.Child(res, "test"))] = true
			}
		}
	}
	var out []string
	for _, t := range source.Pairs(source.Child(rc.d.root, "tests")) {
		if sc := source.Str(source.Child(t.Value, "scenario")); sc != "golden" && sc != "red" || source.Child(t.Value, "notApplicable") != nil || passed[t.Key.Value] {
			continue
		}
		for _, v := range source.Items(source.Child(t.Value, "verifies")) {
			if held[v.Value] {
				out = append(out, t.Key.Value)
				break
			}
		}
	}
	return out
}
