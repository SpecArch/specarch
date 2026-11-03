package generate

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// The document written from the test-run records (ADR-230): the runs, and
// the failures per element and per requirement run by run.

// testElementKeys are the keys a test names its subject by, with the
// section of the specification that subject is under.
var testElementKeys = []struct{ key, section string }{
	{"command", "commands"}, {"page", "pages"}, {"job", "jobs"}, {"process", "processes"}, {"route", "routes"},
	{"gateway", "gateways"}, {"workflow", "workflows"}, {"flow", "flows"}, {"algorithm", "algorithms"}, {"entity", "entities"},
}

// testRunElement is the #/ pointer of the element a test is about: the
// path of its operation, or the command, page, job, process, route,
// gateway, workflow, flow, algorithm or entity it names; the test itself
// when it names none of them.
func testRunElement(root *yaml.Node, name string, t *yaml.Node) string {
	if v := str(t, "operation"); v != "" {
		for _, path := range pairs(root, "paths") {
			for _, op := range source.Pairs(path.Value) {
				if isMethodKey(op.Key.Value) && str(op.Value, "operationId") == v {
					return "#" + source.Pointer("paths", path.Key.Value)
				}
			}
		}
	}
	for _, k := range testElementKeys {
		if v := str(t, k.key); v != "" {
			return "#" + source.Pointer(k.section, v)
		}
	}
	return "#" + source.Pointer("tests", name)
}

// elementOfRef is the element a #/ pointer points into, its first two
// tokens, or "" for a requirement ID.
func elementOfRef(ref string) string {
	if !strings.HasPrefix(ref, "#/") {
		return ""
	}
	tokens := strings.SplitN(strings.TrimPrefix(ref, "#/"), "/", 3)
	if len(tokens) < 2 {
		return ""
	}
	return "#/" + tokens[0] + "/" + tokens[1]
}

// runCounts are a test run's results counted by outcome.
type runCounts struct{ passed, failed, errored, skipped, unmapped int }

func countRun(r *yaml.Node) runCounts {
	var c runCounts
	for _, res := range items(r, "results") {
		switch str(res, "outcome") {
		case "passed":
			c.passed++
		case "failed":
			c.failed++
		case "error":
			c.errored++
		case "skipped":
			c.skipped++
		}
		if get(res, "test") == nil {
			c.unmapped++
		}
	}
	return c
}

// trendRow is one element or requirement of the trend: the tests that
// count for it and the open defects on it.
type trendRow struct {
	key     string
	tests   map[string]bool
	defects []string
}

// trendCell is the failed and errored results of a row's tests in a run,
// or a dash when none of them ran: a skipped result did not run.
func trendCell(row *trendRow, run *yaml.Node) string {
	ran, failed := false, 0
	for _, res := range items(run, "results") {
		if !row.tests[str(res, "test")] {
			continue
		}
		switch str(res, "outcome") {
		case "passed":
			ran = true
		case "failed", "error":
			ran = true
			failed++
		}
	}
	if !ran {
		return "-"
	}
	return fmt.Sprint(failed)
}

// TestRuns writes the test runs and their failure trend from the records.
func TestRuns(root *yaml.Node, relRoot string, state *State) string {
	d := newRecordDoc("testruns", root, relRoot, state)
	info := get(root, "info")
	loc := zoneOf(root)
	runs := recordsOf(state, "test-run")
	sort.SliceStable(runs, func(i, j int) bool {
		a, b := placeInTime(str(runs[i], "at"), loc), placeInTime(str(runs[j], "at"), loc)
		if !a.Equal(b) {
			return a.Before(b)
		}
		return idLess(str(runs[i], "id"), str(runs[j], "id"))
	})
	d.line("# %s: test runs and failure trend", str(info, "title"))
	d.blank()
	intro := fmt.Sprintf("Version %s of the specification: %s recorded beside it", str(info, "version"), countText(len(runs), "test run is", "test runs are"))
	if len(runs) > 0 {
		last := runs[len(runs)-1]
		c := countRun(last)
		intro += fmt.Sprintf(", from %s to %s. The latest, %s, has %s: %d passed, %d failed, %d errored, %d skipped, and %d %s to no test",
			str(runs[0], "at"), str(last, "at"), str(last, "id"), countText(len(items(last, "results")), "result", "results"),
			c.passed, c.failed, c.errored, c.skipped, c.unmapped, oneOrMany(c.unmapped, "maps", "map"))
	}
	d.para(intro + ". The document is made from the records in `records/test-runs/` and `records/defects/`: each result names the test it ran, and through the test the element it is about and the requirements it verifies.")

	d.section("Test runs")
	if len(runs) == 0 {
		d.para("No test run is recorded. specarch log test-run records one from a JUnit XML file.")
		return d.String()
	}
	d.line("| Run | At | Environment | Version | Build | Passed | Failed | Errored | Skipped | Unmapped |")
	d.line("|---|---|---|---|---|---|---|---|---|---|")
	for _, r := range runs {
		c := countRun(r)
		d.line("| %s | %s | %s | %s | %s | %d | %d | %d | %d | %d |", str(r, "id"), str(r, "at"), str(r, "environment"), cell(str(r, "version")), cell(str(r, "build")), c.passed, c.failed, c.errored, c.skipped, c.unmapped)
	}
	d.blank()

	// The rows: each element and each requirement that a test counts for.
	elements, requirements := map[string]*trendRow{}, map[string]*trendRow{}
	row := func(m map[string]*trendRow, key string) *trendRow {
		if m[key] == nil {
			m[key] = &trendRow{key: key, tests: map[string]bool{}}
		}
		return m[key]
	}
	testElementOf := map[string]string{}
	for _, t := range pairs(root, "tests") {
		el := testRunElement(root, t.Key.Value, t.Value)
		testElementOf[t.Key.Value] = el
		row(elements, el).tests[t.Key.Value] = true
		for _, req := range strs(t.Value, "verifies") {
			row(requirements, req).tests[t.Key.Value] = true
		}
	}
	for _, def := range recordsOf(state, "defect") {
		if !isOpen(def) {
			continue
		}
		id := str(def, "id")
		onElement := map[string]bool{}
		if el := testElementOf[str(def, "test")]; el != "" {
			onElement[el] = true
		}
		for _, v := range strs(def, "violates") {
			if el := elementOfRef(v); el != "" {
				onElement[el] = true
			} else if r := requirements[v]; r != nil {
				r.defects = append(r.defects, id)
			}
		}
		for el := range onElement {
			if r := elements[el]; r != nil {
				r.defects = append(r.defects, id)
			}
		}
	}
	trend := func(title, what, first, one, many string, m map[string]*trendRow, label func(string) string) {
		d.section(title)
		d.para(what)
		var keys []string
		for k := range m {
			// A document for everyone leaves an internal element and an
			// internal requirement out (ADR-119).
			if state.Withheld.CoversRef(k) {
				continue
			}
			keys = append(keys, k)
		}
		sort.SliceStable(keys, func(i, j int) bool { return idLess(keys[i], keys[j]) })
		header := "| " + first
		rule := "|---"
		for _, r := range runs {
			header += " | " + str(r, "id")
			rule += "|---"
		}
		d.line("%s | Open defects |", header)
		d.line("%s|---|", rule)
		var never []string
		for _, k := range keys {
			ran := false
			for _, r := range runs {
				if trendCell(m[k], r) != "-" {
					ran = true
				}
			}
			if !ran {
				never = append(never, label(k))
				continue
			}
			line := "| " + label(k)
			for _, r := range runs {
				line += " | " + trendCell(m[k], r)
			}
			defects := "none"
			if len(m[k].defects) > 0 {
				defects = strings.Join(sortedUnique(m[k].defects), ", ")
			}
			d.line("%s | %s |", line, defects)
		}
		d.blank()
		if len(never) > 0 {
			d.para(fmt.Sprintf("No test of %s ran in a recorded run: %s.", countText(len(never), "other "+one, "other "+many), strings.Join(never, ", ")))
		}
	}
	trend("Failures per element", "Each run is a column, oldest first. A cell counts the failed and errored results of the tests about the element in that run; a dash says none of those tests ran, since a skipped result did not run. The open defects are the ones reported or confirmed that violate the element or name one of its tests.",
		"Element", "element", "elements", elements, func(k string) string { return "`" + k + "`" })
	trend("Failures per requirement", "Each run is a column, oldest first. A cell counts the failed and errored results of the tests that verify the requirement in that run; a dash says none of those tests ran. The open defects are the ones reported or confirmed that violate the requirement.",
		"Requirement", "requirement", "requirements", requirements, func(k string) string { return k })

	d.section("Results that map to no test")
	var unmapped []string
	for _, r := range runs {
		for _, res := range items(r, "results") {
			if get(res, "test") == nil {
				unmapped = append(unmapped, fmt.Sprintf("| %s | %s | %s | %s |", str(r, "id"), cell(str(res, "name")), str(res, "outcome"), cell(str(res, "message"))))
			}
		}
	}
	if len(unmapped) == 0 {
		d.para("Every result names the test it ran.")
	} else {
		d.para("These results name no test of the specification, so they count for no element or requirement above; each is a test the specification does not have yet, or one whose name its record must give under test.")
		d.line("| Run | Result | Outcome | Message |")
		d.line("|---|---|---|---|")
		for _, l := range unmapped {
			d.line("%s", l)
		}
		d.blank()
	}

	d.section("Results by run")
	for _, r := range runs {
		head := fmt.Sprintf("%s: %s in %s, by %s", str(r, "id"), str(r, "at"), str(r, "environment"), str(r, "by"))
		d.heading(3, head)
		var on []string
		if v := str(r, "version"); v != "" {
			on = append(on, "version "+v)
		}
		if b := str(r, "build"); b != "" {
			on = append(on, "build "+b)
		}
		if g := str(r, "digest"); g != "" {
			on = append(on, "the specification of digest "+g)
		}
		if len(on) > 0 {
			d.para("Run on " + strings.Join(on, ", ") + ".")
		}
		if ev := items(r, "evidence"); len(ev) > 0 {
			var files []string
			for _, e := range ev {
				files = append(files, fmt.Sprintf("`%s` (%s, %s)", str(e, "path"), str(e, "mediaType"), str(e, "sha256")))
			}
			d.para("Evidence: " + strings.Join(files, ", ") + ".")
		} else {
			d.para("Recorded without a results file as evidence.")
		}
		d.line("| Result | Test | Outcome | Message | Defect |")
		d.line("|---|---|---|---|---|")
		for _, res := range items(r, "results") {
			test := str(res, "test")
			if test == "" {
				test = "none"
			}
			d.line("| %s | %s | %s | %s | %s |", cell(str(res, "name")), test, str(res, "outcome"), cell(str(res, "message")), cell(str(res, "defect")))
		}
		d.blank()
	}
	return d.String()
}

func sortedUnique(in []string) []string {
	out := uniqueStrings(in)
	sort.SliceStable(out, func(i, j int) bool { return idLess(out[i], out[j]) })
	return out
}
