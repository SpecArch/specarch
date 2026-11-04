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

// TestElement is the #/ pointer of the element a test is about: the
// path of its operation, or the command, page, job, process, route,
// gateway, workflow, flow, algorithm or entity it names; "" when it names
// none of them, or an operation no path has. specarch log test-run opens
// a defect on it and the testruns document counts the test's results for
// it, so both place a test on the same element (ADR-230).
func TestElement(root, t *yaml.Node) string {
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
	return ""
}

// testRunElement is the element the testruns document counts a test's
// results for: its TestElement, or the test itself when it has none.
func testRunElement(root *yaml.Node, name string, t *yaml.Node) string {
	if el := TestElement(root, t); el != "" {
		return el
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
		if get(res, "test") == nil && get(res, "element") == nil {
			c.unmapped++
		}
	}
	return c
}

// trendRow is one element or requirement of the trend: the tests that
// count for it, whether the results that name it as their element count
// too (an element row's do), and the open defects on it.
type trendRow struct {
	key       string
	tests     map[string]bool
	byElement bool
	defects   []string
}

// counts reports whether a result counts for the row: it names one of
// the row's tests, or, on an element row, it runs the worked examples of
// that element (ADR-230).
func (row *trendRow) counts(res *yaml.Node) bool {
	if t := str(res, "test"); t != "" {
		return row.tests[t]
	}
	return row.byElement && elementOfRef(str(res, "element")) == row.key
}

// trendCell is the failed and errored results that count for a row in a
// run, or a dash when none of them ran: a skipped result did not run.
func trendCell(row *trendRow, run *yaml.Node) string {
	ran, failed := false, 0
	for _, res := range items(run, "results") {
		if !row.counts(res) {
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
	d.line(tr("# %s: test runs and failure trend"), str(info, "title"))
	d.blank()
	intro := fmt.Sprintf(tr("Version %s of the specification: %s recorded beside it"), str(info, "version"), countText(len(runs), tr("test run is"), tr("test runs are")))
	if len(runs) > 0 {
		last := runs[len(runs)-1]
		c := countRun(last)
		intro += fmt.Sprintf(tr(", from %s to %s. The latest, %s, has %s: %d passed, %d failed, %d errored, %d skipped, and %d %s to no test"),
			str(runs[0], "at"), str(last, "at"), str(last, "id"), countText(len(items(last, "results")), tr("result"), tr("results")),
			c.passed, c.failed, c.errored, c.skipped, c.unmapped, oneOrMany(c.unmapped, tr("maps"), tr("map")))
	}
	d.para(intro + tr(". The document is made from the records in `records/test-runs/` and `records/defects/`: each result names the test it ran, and through the test the element it is about and the requirements it verifies, or the algorithm or operation whose worked examples it ran."))
	d.stakeholdersNotice()

	d.section(tr("Test runs"))
	if len(runs) == 0 {
		d.para(tr("No test run is recorded. specarch log test-run records one from a JUnit XML file."))
		return d.String()
	}
	d.line(tr("| Run | At | Environment | Version | Build | Passed | Failed | Errored | Skipped | Unmapped |"))
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
	// A result that runs an algorithm's or a call mapping's worked
	// examples names its element instead of a test, and counts for it.
	for _, r := range runs {
		for _, res := range items(r, "results") {
			if el := elementOfRef(str(res, "element")); el != "" && get(res, "test") == nil {
				row(elements, el)
			}
		}
	}
	for _, r := range elements {
		r.byElement = true
	}

	// A document for everyone leaves out a result whose test, or the
	// element that test is about, or the element it names, is internal;
	// and it names no result that maps to nothing, since nothing tells
	// whether that one runs an internal element (ADR-119).
	everyone := state.Withheld != nil
	withheld := func(res *yaml.Node) bool {
		if !everyone {
			return false
		}
		t, el := str(res, "test"), str(res, "element")
		switch {
		case t != "":
			return state.Withheld.CoversRef("#"+source.Pointer("tests", t)) || state.Withheld.CoversRef(testElementOf[t])
		case el != "":
			return state.Withheld.CoversRef(el)
		}
		return true
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
		d.line(tr("%s | Open defects |"), header)
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
			d.para(fmt.Sprintf(tr("No test of %s ran in a recorded run: %s."), countText(len(never), fmt.Sprintf(tr("other %s"), one), fmt.Sprintf(tr("other %s"), many)), strings.Join(never, ", ")))
		}
	}
	trend(tr("Failures per element"), tr("Each run is a column, oldest first. A cell counts the failed and errored results of the tests about the element in that run, and those of the results that name it as the algorithm or operation whose worked examples they ran; a dash says none of those tests ran, since a skipped result did not run. The open defects are the ones reported or confirmed that violate the element or name one of its tests."),
		tr("Element"), tr("element"), tr("elements"), elements, func(k string) string { return "`" + k + "`" })
	trend(tr("Failures per requirement"), tr("Each run is a column, oldest first. A cell counts the failed and errored results of the tests that verify the requirement in that run; a dash says none of those tests ran. The open defects are the ones reported or confirmed that violate the requirement."),
		tr("Requirement"), tr("requirement"), tr("requirements"), requirements, func(k string) string { return k })

	d.section(tr("Results that map to no test"))
	var unmapped []string
	for _, r := range runs {
		for _, res := range items(r, "results") {
			if get(res, "test") == nil && get(res, "element") == nil {
				unmapped = append(unmapped, fmt.Sprintf("| %s | %s | %s | %s |", str(r, "id"), cell(str(res, "name")), str(res, "outcome"), cell(str(res, "message"))))
			}
		}
	}
	switch {
	case len(unmapped) == 0:
		d.para(tr("Every result names the test it ran, or the element whose worked examples it ran."))
	case everyone:
		d.para(fmt.Sprintf(tr("%s to no test of the specification, so %s for no element or requirement above. A document for everyone does not name %s, since nothing tells whether %s an internal element."),
			countText(len(unmapped), tr("result maps"), tr("results map")), oneOrMany(len(unmapped), tr("it counts"), tr("they count")), oneOrMany(len(unmapped), tr("it"), tr("them")), oneOrMany(len(unmapped), tr("it runs"), tr("they run"))))
	default:
		d.para(tr("These results name no test of the specification, so they count for no element or requirement above; each is a test the specification does not have yet, or one whose name its record must give under test."))
		d.line(tr("| Run | Result | Outcome | Message |"))
		d.line("|---|---|---|---|")
		for _, l := range unmapped {
			d.line("%s", l)
		}
		d.blank()
	}

	d.section(tr("Results by run"))
	for _, r := range runs {
		head := fmt.Sprintf(tr("%s: %s in %s, by %s"), str(r, "id"), str(r, "at"), str(r, "environment"), str(r, "by"))
		d.heading(3, head)
		var on []string
		if v := str(r, "version"); v != "" {
			on = append(on, tr("version ")+v)
		}
		if b := str(r, "build"); b != "" {
			on = append(on, tr("build ")+b)
		}
		if g := str(r, "digest"); g != "" {
			on = append(on, tr("the specification of digest ")+g)
		}
		if len(on) > 0 {
			d.para(tr("Run on ") + strings.Join(on, ", ") + ".")
		}
		if ev := items(r, "evidence"); len(ev) > 0 {
			var files []string
			for _, e := range ev {
				files = append(files, fmt.Sprintf("`%s` (%s, %s)", str(e, "path"), str(e, "mediaType"), str(e, "sha256")))
			}
			d.para(tr("Evidence: ") + strings.Join(files, ", ") + ".")
		} else {
			d.para(tr("Recorded without a results file as evidence."))
		}
		d.line(tr("| Result | Test | Outcome | Message | Defect |"))
		d.line("|---|---|---|---|---|")
		left := 0
		for _, res := range items(r, "results") {
			if withheld(res) {
				left++
				continue
			}
			test := str(res, "test")
			if test == "" {
				test = "none"
				if el := str(res, "element"); el != "" {
					test = tr("the worked examples of `") + el + "`"
				}
			}
			d.line("| %s | %s | %s | %s | %s |", cell(str(res, "name")), test, str(res, "outcome"), cell(str(res, "message")), cell(str(res, "defect")))
		}
		d.blank()
		if left > 0 {
			d.para(fmt.Sprintf(tr("%s of this run %s left out of a document for everyone: %s an internal test or element, or %s to no test (ADR-119)."), countText(left, tr("result"), tr("results")), oneOrMany(left, tr("is"), tr("are")), oneOrMany(left, tr("it runs"), tr("they run")), oneOrMany(left, tr("maps"), tr("map"))))
		}
	}
	return d.String()
}

func sortedUnique(in []string) []string {
	out := uniqueStrings(in)
	sort.SliceStable(out, func(i, j int) bool { return idLess(out[i], out[j]) })
	return out
}
