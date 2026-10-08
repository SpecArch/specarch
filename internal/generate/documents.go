package generate

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

type sourcePair = source.Pair

// Document writes the document of a target from a specification and its
// implementation files. ok is false for a target this package does not
// write.
func Document(target string, root *yaml.Node, relRoot string, impls []Implementation, state *State) (text string, ok bool) {
	switch target {
	case "techspec":
		return Techspec(root, relRoot, impls), true
	case "requirements":
		return Requirements(root, relRoot, impls), true
	case "testplan":
		return Testplan(root, relRoot, impls, state), true
	case "traceability":
		return Traceability(root, relRoot, impls), true
	case "deployment":
		return DeploymentGuide(root, relRoot, impls), true
	case "commissioning":
		return Commissioning(root, relRoot, impls), true
	case "questions":
		return Questions(root, relRoot, impls, state), true
	case "changes":
		return Changes(root, relRoot, state), true
	case "releases":
		return Releases(root, relRoot, state), true
	}
	return "", false
}

// DocumentName is the file a target writes in the folder it owns.
func DocumentName(target string) string { return target + ".md" }

// countText is "1 thing" or "n things".
func countText(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// Requirements writes the requirements specification: the stakeholders,
// their needs, and the requirements with every attribute, in the order of
// a requirements specification of ISO/IEC/IEEE 29148.
func Requirements(root *yaml.Node, relRoot string, impls []Implementation) string {
	d := newDoc("requirements", root, relRoot, impls)
	info := get(root, "info")
	reqs := pairs(root, "requirements")
	needs := pairs(root, "needs")
	d.line("# %s: requirements specification", str(info, "title"))
	d.blank()
	d.para(fmt.Sprintf("Version %s of the specification: %s, %s and %s. The order follows the requirements specification of ISO/IEC/IEEE 29148: who has a stake, what they need, then each requirement with its attributes.",
		str(info, "version"), countText(len(pairs(root, "stakeholders")), "stakeholder", "stakeholders"), countText(len(needs), "need", "needs"), countText(len(reqs), "requirement", "requirements")))
	d.draftNotice("requirements")

	d.section("Purpose and scope")
	d.para(str(info, "description"))
	if owners := strs(info, "owners"); len(owners) > 0 {
		d.para("Owners: " + strings.Join(owners, ", ") + ".")
	}

	if sh := pairs(root, "stakeholders"); len(sh) > 0 {
		d.section("Stakeholders")
		d.line("| Stakeholder | Who they are | Concerns |")
		d.line("|---|---|---|")
		for _, s := range sh {
			d.line("| %s | %s | %s |", s.Key.Value, cell(str(s.Value, "description")), cell(joinSentences(strs(s.Value, "concerns"))))
		}
		d.blank()
		d.explainRows(rowsOf(sh))
	}

	if len(needs) > 0 {
		refinedBy := map[string][]string{}
		for _, r := range reqs {
			for _, n := range strs(r.Value, "needs") {
				refinedBy[n] = append(refinedBy[n], r.Key.Value)
			}
		}
		d.section("Needs")
		d.para("What the stakeholders said they need, before it was shaped into requirements, and the requirements that refine each need.")
		d.line("| Need | Statement | Stakeholders | Status | Refined by |")
		d.line("|---|---|---|---|---|")
		for _, n := range needs {
			d.line("| %s | %s | %s | %s | %s |", n.Key.Value, cell(str(n.Value, "statement")), cell(strings.Join(strs(n.Value, "stakeholders"), ", ")), cell(str(n.Value, "status")), cell(strings.Join(refinedBy[n.Key.Value], ", ")))
		}
		d.blank()
		d.explainRows(rowsOf(needs))
	}

	if len(reqs) > 0 {
		d.section("Requirements")
		d.line("| Requirement | Kind | Priority | Status | Statement |")
		d.line("|---|---|---|---|---|")
		for _, r := range reqs {
			d.line("| %s | %s | %s | %s | %s |", r.Key.Value, str(r.Value, "kind"), str(r.Value, "priority"), str(r.Value, "status"), cell(str(r.Value, "statement")))
		}
		d.blank()
		for _, r := range reqs {
			d.heading(3, r.Key.Value)
			d.para(str(r.Value, "statement"))
			var attrs []string
			for _, a := range []struct{ label, key string }{{"Kind", "kind"}, {"priority", "priority"}, {"status", "status"}} {
				if v := str(r.Value, a.key); v != "" {
					attrs = append(attrs, a.label+": "+v)
				}
			}
			if len(attrs) > 0 {
				attrs[0] = strings.ToUpper(attrs[0][:1]) + attrs[0][1:]
			}
			if v := str(r.Value, "verification"); v != "" {
				attrs = append(attrs, "verified by "+v)
			}
			if n := strs(r.Value, "needs"); len(n) > 0 {
				attrs = append(attrs, "refines "+strings.Join(n, ", "))
			}
			if h := strs(r.Value, "harm"); len(h) > 0 {
				attrs = append(attrs, "harm if not met: "+strings.Join(h, ", "))
			}
			if len(attrs) > 0 {
				d.para(strings.Join(attrs, "; ") + ".")
			}
			if acc := strs(r.Value, "acceptance"); len(acc) > 0 {
				d.line("Acceptance criteria:")
				d.blank()
				for _, a := range acc {
					d.line("- %s", oneParagraph(a))
				}
				d.blank()
			} else {
				d.para("No acceptance criteria yet.")
			}
			d.explain(r.Value)
		}
	}

	if cons := pairs(root, "constraints"); len(cons) > 0 {
		d.section("Constraints")
		d.line("| Constraint | Kind | Statement |")
		d.line("|---|---|---|")
		for _, c := range cons {
			d.line("| %s | %s | %s |", c.Key.Value, str(c.Value, "kind"), cell(str(c.Value, "statement")))
		}
		d.blank()
		d.explainRows(rowsOf(cons))
	}
	if as := pairs(root, "assumptions"); len(as) > 0 {
		d.section("Assumptions")
		d.line("| Assumption | Statement |")
		d.line("|---|---|")
		for _, a := range as {
			d.line("| %s | %s |", a.Key.Value, cell(str(a.Value, "statement")))
		}
		d.blank()
		d.explainRows(rowsOf(as))
	}
	if terms := pairs(root, "glossary"); len(terms) > 0 {
		d.section("Glossary")
		d.line("| Term | Meaning |")
		d.line("|---|---|")
		for _, t := range terms {
			d.line("| %s | %s |", cell(t.Key.Value), cell(str(t.Value, "definition")))
		}
		d.blank()
		d.explainRows(rowsOf(terms))
	}
	var sets []string
	for _, s := range pairs(root, "sources") {
		if str(s.Value, "kind") == "requirement-set" {
			sets = append(sets, fmt.Sprintf("Requirements with the prefix %s are in %s.", str(s.Value, "prefix"), strings.TrimSpace(str(s.Value, "title"))))
		}
	}
	if len(sets) > 0 {
		d.section("Requirements kept elsewhere")
		d.para(strings.Join(sets, " "))
	}
	d.sourcesIndex("Sources")
	return d.String()
}

// Testplan writes the test plan and the test case specifications: the
// levels, how each implementation runs the tests, then every design test
// grouped by its subject, then the derived cases left out.
func Testplan(root *yaml.Node, relRoot string, impls []Implementation, state *State) string {
	d := newDoc("testplan", root, relRoot, impls)
	info := get(root, "info")
	tests := pairs(root, "tests")
	golden, red, na := 0, 0, 0
	var order []string
	bySubject := map[string][]row{}
	for _, t := range tests {
		switch {
		case str(t.Value, "notApplicable") != "":
			na++
		case str(t.Value, "scenario") == "red":
			red++
		default:
			golden++
		}
		s := testSubject(t.Value)
		if s == "" {
			s = "other"
		}
		if bySubject[s] == nil {
			order = append(order, s)
		}
		bySubject[s] = append(bySubject[s], row{t.Key.Value, t.Value})
	}
	d.line("# %s: test plan", str(info, "title"))
	d.blank()
	d.para(fmt.Sprintf("Version %s of the specification: %s, %d golden and %d red, about %s. Golden tests show a path that succeeds, red tests a path that is refused. The test cases follow the test case specification of ISO/IEC/IEEE 29119-3.",
		str(info, "version"), countText(len(tests), "design test", "design tests"), golden, red, countText(len(order), "subject", "subjects")))
	d.draftNotice("testplan")
	if na > 0 {
		d.para(fmt.Sprintf("%s marked not applicable, with the reason.", countText(na, "test is", "tests are")))
	}

	d.section("Levels and how the tests run")
	levels := map[string]int{}
	for _, t := range tests {
		levels[str(t.Value, "level")]++
	}
	d.line("| Level | Design tests |")
	d.line("|---|---|")
	for _, l := range sortedKeys(levels) {
		d.line("| %s | %d |", cell(l), levels[l])
	}
	d.blank()
	d.para("System and acceptance tests are design tests, written in the specification and run by every implementation. Unit and integration tests belong to one implementation and are listed with it below.")
	for _, i := range impls {
		testing := get(i.Node, "testing")
		if testing == nil {
			continue
		}
		d.heading(3, "Implementation: "+str(get(i.Node, "info"), "title"))
		d.para(fmt.Sprintf("Framework: %s. Run: `%s`.", str(testing, "framework"), str(testing, "run")))
		d.line("| Suite | Level | Runs | Command |")
		d.line("|---|---|---|---|")
		for _, s := range pairs(testing, "suites") {
			d.line("| %s | %s | %s | `%s` |", s.Key.Value, str(s.Value, "level"), cell(suiteRuns(s.Value)), cell(str(s.Value, "run")))
		}
		d.blank()
		d.explainRows(pairRows(testing, "suites"))
	}

	if len(tests) > 0 {
		d.section("Test cases")
		for _, s := range order {
			d.heading(3, strings.ToUpper(s[:1])+s[1:])
			for _, t := range bySubject[s] {
				d.heading(4, t.label)
				attrs := []string{"Scenario: " + str(t.node, "scenario"), "level: " + str(t.node, "level")}
				if cov := strs(t.node, "covers"); len(cov) > 0 {
					attrs = append(attrs, "covers "+strings.Join(cov, ", "))
				}
				if v := strs(t.node, "verifies"); len(v) > 0 {
					attrs = append(attrs, "verifies "+strings.Join(v, ", "))
				}
				d.para(strings.Join(attrs, "; ") + ".")
				if reason := str(t.node, "notApplicable"); reason != "" {
					d.para("Not applicable: " + oneParagraph(reason))
				} else {
					d.line("- Given: %s", oneParagraph(str(t.node, "given")))
					d.line("- When: %s", oneParagraph(str(t.node, "when")))
					d.line("- Then: %s", oneParagraph(str(t.node, "then")))
					d.blank()
				}
				d.explain(t.node)
			}
		}
	}
	stateMachines(d, root, state)
	if state != nil && len(state.LeftOut) > 0 {
		d.section("Derived cases left out")
		d.para(fmt.Sprintf("%s the design implies %s no test and %s not written by default: none is about a subject that satisfies a requirement with a harm, none is a case nobody exercises by hand (a failing dependency, two writers on one record), and none is a mistake users make often. Writing a test that covers one removes it from this list.",
			countText(len(state.LeftOut), "case", "cases"), map[bool]string{true: "has", false: "have"}[len(state.LeftOut) == 1], map[bool]string{true: "is", false: "are"}[len(state.LeftOut) == 1]))
		d.line("| Subject | Case | Scenario | Why it is left out |")
		d.line("|---|---|---|---|")
		for _, c := range state.LeftOut {
			d.line("| %s | %s | %s | %s |", cell(c.Subject), cell(c.Case), c.Scenario, cell(c.Reason))
		}
		d.blank()
	}
	if len(pairs(root, "checks")) > 0 {
		d.para("The checks run on the installed system before it is handed over are in the commissioning procedure.")
	}
	d.sourcesIndex("Sources")
	return d.String()
}

// harmCol is the Harm column of a traceability matrix, there only when a
// requirement names a harm.
type harmCol struct {
	header, rule string
	harm         map[string][]string
}

func harmColumn(reqs []sourcePair) harmCol {
	h := harmCol{harm: map[string][]string{}}
	for _, r := range reqs {
		if v := strs(r.Value, "harm"); len(v) > 0 {
			h.harm[r.Key.Value] = v
			h.header, h.rule = " Harm |", "---|"
		}
	}
	return h
}

// cell is the requirement's cell with its separator, or "" without the
// column.
func (h harmCol) cell(id string) string {
	if h.header == "" {
		return ""
	}
	return " " + cell(strings.Join(h.harm[id], ", ")) + " |"
}

// suiteRuns says which tests a suite runs.
func suiteRuns(s *yaml.Node) string {
	if names := strs(s, "designTests"); len(names) > 0 {
		return "design tests " + strings.Join(names, ", ")
	}
	var of []string
	for _, subj := range items(s, "designTestsOf") {
		for _, p := range pairsOf(subj) {
			of = append(of, p.Key.Value+" "+p.Value.Value)
		}
	}
	if len(of) > 0 {
		return "every design test of " + strings.Join(of, ", ")
	}
	return "tests of this implementation only"
}

// Traceability writes the traceability matrix from needs through
// requirements to what satisfies and verifies them, and lists the gaps. It
// names elements by ID only; their Insights are in the documents that
// describe them.
func Traceability(root *yaml.Node, relRoot string, impls []Implementation) string {
	d := newDoc("traceability", root, relRoot, impls)
	info := get(root, "info")
	reqs := pairs(root, "requirements")
	needs := pairs(root, "needs")
	satisfied, verified := traceLinks(root, impls)

	refinedBy := map[string][]string{}
	for _, r := range reqs {
		for _, n := range strs(r.Value, "needs") {
			refinedBy[n] = append(refinedBy[n], r.Key.Value)
		}
	}
	hasDesign := false
	for _, s := range []string{"enums", "entities", "permissions", "roles", "session", "paths", "commands", "channels", "dependencies", "jobs", "errors", "pages", "algorithms", "decisions"} {
		if get(root, s) != nil {
			hasDesign = true
		}
	}
	hasTests := len(pairs(root, "tests")) > 0 || len(pairs(root, "checks")) > 0 || len(pairs(root, "monitors")) > 0
	var unrefined, noAcceptance, unsatisfied, unverified []string
	for _, n := range needs {
		if str(n.Value, "status") != "rejected" && len(refinedBy[n.Key.Value]) == 0 {
			unrefined = append(unrefined, n.Key.Value)
		}
	}
	for _, r := range reqs {
		id := r.Key.Value
		if s := str(r.Value, "status"); s == "rejected" || s == "retired" {
			continue
		}
		if get(r.Value, "acceptance") == nil {
			noAcceptance = append(noAcceptance, id)
		}
		if hasDesign && len(satisfied[id]) == 0 {
			unsatisfied = append(unsatisfied, id)
		}
		if hasTests && len(verified[id]) == 0 {
			unverified = append(unverified, id)
		}
	}
	gaps := len(unrefined) + len(noAcceptance) + len(unsatisfied) + len(unverified)

	d.line("# %s: traceability matrix", str(info, "title"))
	d.blank()
	summary := fmt.Sprintf("Version %s of the specification: %s, %s, and %s.", str(info, "version"),
		countText(len(needs), "need", "needs"), countText(len(reqs), "requirement", "requirements"), countText(gaps, "gap", "gaps"))
	d.para(summary + " Each requirement is traced back to the needs it refines and forward to what satisfies it in the design and what verifies it in the tests and commissioning checks.")
	d.draftNotice("traceability")

	if len(needs) > 0 {
		d.section("Needs to requirements")
		d.line("| Need | Status | Refined by |")
		d.line("|---|---|---|")
		for _, n := range needs {
			d.line("| %s | %s | %s |", n.Key.Value, cell(str(n.Value, "status")), cell(strings.Join(refinedBy[n.Key.Value], ", ")))
		}
		d.blank()
	}

	d.section("Requirements to design and verification")
	harm := harmColumn(reqs)
	d.line("| Requirement |%s Needs | Satisfied by | Verified by |", harm.header)
	d.line("|---|%s---|---|---|", harm.rule)
	ids := map[string]bool{}
	reqNode := map[string]*yaml.Node{}
	for _, r := range reqs {
		ids[r.Key.Value] = true
		reqNode[r.Key.Value] = r.Value
	}
	for k := range satisfied {
		ids[k] = true
	}
	for k := range verified {
		ids[k] = true
	}
	for _, k := range sortedLinks(ids) {
		d.line("| %s |%s %s | %s | %s |", k, harm.cell(k), cell(strings.Join(strs(reqNode[k], "needs"), ", ")), cell(strings.Join(unique(satisfied[k]), "; ")), cell(strings.Join(unique(verified[k]), "; ")))
	}
	d.blank()

	d.section("Gaps")
	if gaps == 0 {
		d.para("None: every need is refined, and every requirement has acceptance criteria, is satisfied once there is a design, and is verified once there are tests, checks or monitors.")
	} else {
		d.para("The work the specification still owes, the same gaps the validator warns about. Rejected needs and rejected or retired requirements are left out.")
		for _, g := range []struct {
			ids  []string
			what string
		}{
			{unrefined, "Needs no requirement refines"},
			{noAcceptance, "Requirements without acceptance criteria"},
			{unsatisfied, "Requirements no design element satisfies"},
			{unverified, "Requirements no test, check or monitor verifies"},
		} {
			if len(g.ids) > 0 {
				d.line("- %s: %s.", g.what, strings.Join(g.ids, ", "))
			}
		}
		d.blank()
	}
	return d.String()
}

// DeploymentGuide writes the deployment guide: the environments and the
// promotion path, the settings and their values in each installation, the
// installations of each implementation, then release, rollback and the
// migrations.
func DeploymentGuide(root *yaml.Node, relRoot string, impls []Implementation) string {
	d := newDoc("deployment", root, relRoot, impls)
	info := get(root, "info")
	envs := pairs(root, "environments")
	cfg := pairs(root, "configuration")
	d.line("# %s: deployment guide", str(info, "title"))
	d.blank()
	d.para(fmt.Sprintf("Version %s of the specification: %s, %s and %s. Secrets are named here with where their value comes from, never with a value.",
		str(info, "version"), countText(len(envs), "environment", "environments"), countText(len(cfg), "setting", "settings"), countText(len(pairs(root, "migrations")), "migration", "migrations")))
	d.draftNotice("deployment")

	if len(envs) > 0 {
		d.section("Environments")
		if path := promotionPath(envs); path != "" {
			d.para("A release goes " + path + ".")
		}
		d.line("| Environment | Purpose | Promotes to |")
		d.line("|---|---|---|")
		for _, e := range envs {
			d.line("| %s | %s | %s |", e.Key.Value, cell(str(e.Value, "description")), cell(str(e.Value, "promotesTo")))
		}
		d.blank()
		d.explainRows(rowsOf(envs))
	}

	if len(cfg) > 0 {
		d.section("Configuration")
		d.line("| Setting | Type | Secret | Description |")
		d.line("|---|---|---|---|")
		for _, c := range cfg {
			secret := ""
			if str(c.Value, "secret") == "true" {
				secret = "yes"
			}
			d.line("| %s | %s | %s | %s |", c.Key.Value, cell(typeText(get(c.Value, "schema"))), cell(secret), cell(str(c.Value, "description")))
		}
		d.blank()
		d.explainRows(rowsOf(cfg))
	}

	for _, i := range impls {
		deps := pairs(i.Node, "deployments")
		if len(deps) == 0 {
			continue
		}
		d.section("Installations: " + str(get(i.Node, "info"), "title"))
		for _, dep := range deps {
			d.heading(3, dep.Key.Value)
			line := strings.TrimSpace(str(dep.Value, "description"))
			if e := str(dep.Value, "environment"); e != "" {
				line += " Environment: " + e + "."
			}
			d.para(line)
			if servers := items(dep.Value, "servers"); len(servers) > 0 {
				d.line("| Server | Description |")
				d.line("|---|---|")
				for _, s := range servers {
					d.line("| %s | %s |", cell(str(s, "url")), cell(str(s, "description")))
				}
				d.blank()
			}
			if values := pairs(dep.Value, "configuration"); len(values) > 0 || len(cfg) > 0 {
				given := map[string]string{}
				for _, v := range values {
					given[v.Key.Value] = v.Value.Value
				}
				d.line("| Setting | Value |")
				d.line("|---|---|")
				for _, c := range cfg {
					v := given[c.Key.Value]
					switch {
					case str(c.Value, "secret") == "true":
						v = "a secret; its description says where the value comes from"
					case v == "":
						v = "not set"
					}
					d.line("| %s | %s |", c.Key.Value, cell(v))
				}
				d.blank()
			}
			deploymentMonitors(d, dep.Value)
			d.explain(dep.Value)
		}
	}

	if release := get(root, "release"); release != nil {
		d.section("Release")
		d.para(str(release, "description"))
		d.explain(release)
		stepsTable(d, get(release, "steps"))
	}
	if rollback := get(root, "rollback"); rollback != nil {
		d.section("Rollback")
		d.para(str(rollback, "description"))
		d.explain(rollback)
		stepsTable(d, get(rollback, "steps"))
	}
	if mons := pairs(root, "monitors"); len(mons) > 0 {
		d.section("Monitors")
		monitorsTable(d, mons)
	}
	if migs := pairs(root, "migrations"); len(migs) > 0 {
		d.section("Migrations")
		for _, m := range migs {
			d.heading(3, m.Key.Value)
			d.para(str(m.Value, "description"))
			d.explain(m.Value)
			stepsTable(d, get(m.Value, "steps"))
			if rb := get(m.Value, "rollback"); rb != nil {
				d.para("Reversed by:")
				stepsTable(d, rb)
			} else {
				d.para("This migration cannot be reversed.")
			}
		}
	}
	d.sourcesIndex("Sources")
	return d.String()
}

// promotionPath writes the path a release takes, from the environment
// nothing promotes to: "to development, then staging, then production".
func promotionPath(envs []sourcePair) string {
	next := map[string]string{}
	target := map[string]bool{}
	for _, e := range envs {
		if p := str(e.Value, "promotesTo"); p != "" {
			next[e.Key.Value] = p
			target[p] = true
		}
	}
	for _, e := range envs {
		if target[e.Key.Value] || next[e.Key.Value] == "" {
			continue
		}
		path := []string{e.Key.Value}
		seen := map[string]bool{e.Key.Value: true}
		for cur := next[e.Key.Value]; cur != "" && !seen[cur]; cur = next[cur] {
			path = append(path, cur)
			seen[cur] = true
		}
		return "to " + strings.Join(path, ", then ")
	}
	return ""
}

// Commissioning writes the commissioning test procedure, one section per
// environment in the order a release reaches them, with a column for each
// step's result, and the sign-off sheet.
func Commissioning(root *yaml.Node, relRoot string, impls []Implementation) string {
	d := newDoc("commissioning", root, relRoot, impls)
	info := get(root, "info")
	checks := pairs(root, "checks")
	d.line("# %s: commissioning procedure", str(info, "title"))
	d.blank()
	d.para(fmt.Sprintf("Version %s of the specification: %s run on the installed system before it is handed over, and the sign-off sheet. Fill in the Result column as each step is run; the filled-in run is kept as a record under `records/commissioning/`, beside the specification, not in it.",
		str(info, "version"), countText(len(checks), "check", "checks")))
	d.draftNotice("commissioning")

	envOrder := []string{}
	envs := pairs(root, "environments")
	if path := promotionPath(envs); path != "" {
		envOrder = strings.Split(strings.TrimPrefix(path, "to "), ", then ")
	}
	for _, e := range envs {
		if !contains(envOrder, e.Key.Value) {
			envOrder = append(envOrder, e.Key.Value)
		}
	}
	byEnv := map[string][]sourcePair{}
	for _, c := range checks {
		env := str(c.Value, "environment")
		if !contains(envOrder, env) {
			envOrder = append(envOrder, env)
		}
		byEnv[env] = append(byEnv[env], c)
	}
	for _, env := range envOrder {
		list := byEnv[env]
		if len(list) == 0 {
			continue
		}
		title := env
		if title == "" {
			title = "any environment"
		}
		d.section("Checks in " + title)
		for _, c := range list {
			d.heading(3, fmt.Sprintf("%s (%s)", c.Key.Value, str(c.Value, "kind")))
			d.para(str(c.Value, "description"))
			if v := strs(c.Value, "verifies"); len(v) > 0 {
				d.para("Verifies " + strings.Join(v, ", ") + ".")
			}
			d.explain(c.Value)
			steps := itemsOf(get(c.Value, "steps"))
			if len(steps) > 0 {
				d.line("| Step | Action | Expected | Result |")
				d.line("|---|---|---|---|")
				for i, s := range steps {
					d.line("| %d. %s | %s | %s | %s |", i+1, cell(str(s, "name")), cell(str(s, "action")), cell(str(s, "check")), cell(""))
				}
				d.blank()
				d.explainRows(stepRows(get(c.Value, "steps")))
			}
		}
	}

	if signoff := get(root, "signoff"); signoff != nil {
		d.section("Sign-off sheet")
		d.line("| Run | Entry |")
		d.line("|---|---|")
		d.line("| Specification version | %s |", str(info, "version"))
		d.line("| Build | %s |", cell(""))
		d.line("| Environment | %s |", cell(""))
		d.line("| Date | %s |", cell(""))
		d.blank()
		d.para("The system is accepted when every criterion is met:")
		d.line("| Criterion | Met |")
		d.line("|---|---|")
		for _, c := range strs(signoff, "criteria") {
			d.line("| %s | %s |", cell(c), cell(""))
		}
		d.blank()
		d.line("| Role | Description | Name | Signature | Date |")
		d.line("|---|---|---|---|---|")
		for _, s := range items(signoff, "signers") {
			d.line("| %s | %s | %s | %s | %s |", str(s, "role"), cell(str(s, "description")), cell(""), cell(""), cell(""))
		}
		d.blank()
		d.explain(signoff)
	}
	d.sourcesIndex("Sources")
	return d.String()
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// joinSentences joins short sentences into one cell: "a; b".
func joinSentences(list []string) string {
	var out []string
	for _, s := range list {
		out = append(out, strings.TrimSuffix(strings.TrimSpace(s), "."))
	}
	return strings.Join(out, "; ")
}

// stateMachines is the test plan's section on each entity with a state
// machine: its diagram, then each path from an initial to a terminal state
// with the tests that walk it.
func stateMachines(d *doc, root *yaml.Node, state *State) {
	var entities []string
	for _, e := range pairs(root, "entities") {
		if str(e.Value, "stateField") != "" && len(items(e.Value, "transitions")) > 0 {
			entities = append(entities, e.Key.Value)
		}
	}
	if len(entities) == 0 {
		return
	}
	d.section("State machines")
	d.para("Each entity with a state field is a state machine. A path runs from a state no move reaches to one no move leaves; a test about the entity alone walks one, and names it under covers. A path with no test is listed under the derived cases left out, or warned about when it moves through a transition that satisfies a requirement with a harm.")
	for _, e := range entities {
		d.heading(3, "States of "+e)
		diagram, _ := stateDiagram(root, e)
		d.block(diagram)
		var paths []string
		if state != nil {
			paths = state.StatePaths[e]
		}
		if len(paths) == 0 {
			d.para("It has no path from an initial to a terminal state.")
			continue
		}
		d.line("| Path | Tests |")
		d.line("|---|---|")
		for _, p := range paths {
			var tests []string
			for _, t := range pairs(root, "tests") {
				if str(t.Value, "entity") == e && str(t.Value, "constraint") == "" && get(t.Value, "transition") == nil && slices.Contains(strs(t.Value, "covers"), p) {
					tests = append(tests, t.Key.Value)
				}
			}
			d.line("| %s | %s |", cell(p), cell(strings.Join(tests, ", ")))
		}
		d.blank()
	}
}

