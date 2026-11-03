package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/disk"
	"github.com/SpecArch/specarch/internal/gentests"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// comparisonFormat is the version of the object specarch compare writes
// to comparison.json and under compare with --json (ADR-232).
const comparisonFormat = "1.0.0"

// comparison is one table of alternative implementation files of a
// specification. A cell is a value, a gap with its reason, or null where
// nothing is there to compare (a library not used, a finding not found).
type comparison struct {
	Format          string                  `json:"specarchComparison"`
	Title           string                  `json:"title"`
	Version         string                  `json:"version"`
	Implementations []compareImplementation `json:"implementations"`
	Cases           []compareCaseRow        `json:"cases"`
	Measurements    []compareMeasureRow     `json:"measurements"`
	Libraries       []compareLibraryRow     `json:"libraries"`
	Findings        []compareFindingRow     `json:"findings"`
	Gaps            []compareGap            `json:"gaps"`
}

// compareImplementation is one column: the implementation file, by its
// path from the folder that holds records/ and its SHA-256, its inputs and
// its summary. A pointer left nil is a gap.
type compareImplementation struct {
	Name         string             `json:"name"`
	File         string             `json:"file"`
	Version      string             `json:"version,omitempty"`
	SHA256       string             `json:"sha256"`
	Inputs       []compareInput     `json:"inputs"`
	Results      *compareResults    `json:"results"`
	Cases        compareCaseCounts  `json:"cases"`
	Findings     *compareSeverities `json:"findings"`
	Libraries    int                `json:"libraries"`
	BuildSeconds *int64             `json:"buildSeconds"`
	LinesOfCode  *int64             `json:"linesOfCode"`
	DeploySteps  *int64             `json:"deploySteps"`
}

// compareInput is a file an option gave, by its path from the folder that
// holds records/ (and the specification's folder), with its SHA-256.
type compareInput struct {
	Option string `json:"option"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type compareResults struct {
	Passed   int `json:"passed"`
	Failed   int `json:"failed"`
	Errored  int `json:"errored"`
	Skipped  int `json:"skipped"`
	Unmapped int `json:"unmapped"`
}

type compareCaseCounts struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Gaps    int `json:"gaps"`
}

type compareSeverities struct {
	Critical   int `json:"critical"`
	High       int `json:"high"`
	Medium     int `json:"medium"`
	Low        int `json:"low"`
	Negligible int `json:"negligible"`
	Unknown    int `json:"unknown"`
}

type compareCaseRow struct {
	Subject  string        `json:"subject"`
	Case     string        `json:"case"`
	Scenario string        `json:"scenario"`
	Tests    []string      `json:"tests"`
	Cells    []compareCell `json:"cells"`
}

// compareCell is a derived case's outcome in one implementation, or a gap.
type compareCell struct {
	Outcome string `json:"outcome,omitempty"`
	Gap     string `json:"gap,omitempty"`
}

type compareMeasureRow struct {
	Requirement string               `json:"requirement"`
	Statement   string               `json:"statement"`
	Quantity    string               `json:"quantity"`
	Cells       []compareMeasureCell `json:"cells"`
}

type compareMeasureCell struct {
	Value *float64 `json:"value,omitempty"`
	Unit  string   `json:"unit,omitempty"`
	How   string   `json:"how,omitempty"`
	Gap   string   `json:"gap,omitempty"`
}

type compareLibraryRow struct {
	Name  string                `json:"name"`
	Cells []*compareLibraryCell `json:"cells"`
}

type compareLibraryCell struct {
	Version string `json:"version"`
	Licence string `json:"licence"`
	Scope   string `json:"scope"`
}

type compareFindingRow struct {
	ID      string                `json:"id"`
	Package string                `json:"package"`
	Cells   []*compareFindingCell `json:"cells"`
}

type compareFindingCell struct {
	Severity string `json:"severity,omitempty"`
	Version  string `json:"version,omitempty"`
	Gap      string `json:"gap,omitempty"`
}

// compareGap is one missing result: the implementation it is missing in,
// or "" when it is missing in every one, the row, why, and what fills it.
type compareGap struct {
	Implementation string `json:"implementation,omitempty"`
	Row            string `json:"row"`
	Reason         string `json:"reason"`
	Fill           string `json:"fill"`
}

// compareOptions are the options of specarch compare that take
// <name>=<value>, once per implementation.
var compareOptions = []string{"test-run", "results", "scan", "measurements"}

// severityOrder ranks the severities of a finding, the gravest first.
var severityOrder = []string{"critical", "high", "medium", "low", "negligible", "unknown"}

var testRunID = regexp.MustCompile(`^TR-[0-9]+$`)

// measureQuantities are what a measurements file gives per requirement.
var measureQuantities = []string{"latency", "throughput"}

// runCompare writes one comparison of implementation files (ADR-232).
func runCompare(args []string, stdout, stderr io.Writer) int {
	var c *comparison
	return compareInto(args, stdout, stderr, &c)
}

// compareInto is runCompare that keeps the comparison for --json.
func compareInto(args []string, stdout, stderr io.Writer, kept **comparison) int {
	w := disk.Real
	out := ""
	given := map[string][]string{}
	var rest []string
	takes := map[string]bool{}
	for _, o := range compareOptions {
		takes[o] = true
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if a == "--dry-run" {
			w = disk.DryRun(stdout)
			continue
		}
		if !strings.HasPrefix(a, "-") || len(a) == 1 {
			rest = append(rest, a)
			continue
		}
		name, value, inline := strings.Cut(strings.TrimPrefix(a, "--"), "=")
		if !takes[name] && name != "out" {
			fmt.Fprintf(stderr, "specarch compare has no option %s; its options are --test-run, --results, --scan, --measurements, --out and --dry-run\n\n%s", a, usage)
			return 2
		}
		if !inline {
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "specarch compare: %s needs a value\n\n%s", a, usage)
				return 2
			}
			value = args[i+1]
			i++
		}
		if name == "out" {
			out = value
		} else {
			given[name] = append(given[name], value)
		}
	}
	if len(rest) < 2 {
		fmt.Fprintf(stderr, "specarch compare needs the specification's folder and at least one of its implementation files\n\n%s", usage)
		return 2
	}
	folder, files := rest[0], rest[1:]
	specs, invalid, status := readSpecs([]string{folder}, "compare", stdout, stderr)
	if status != 0 {
		return status
	}
	if len(specs) != 1 {
		fmt.Fprintf(stderr, "specarch compare: %s holds %d specifications; name the folder of the one the implementation files implement\n", folder, len(specs))
		return 2
	}
	l := specs[0]
	root := l.spec.Root
	refuse := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "specarch compare: "+format+"; nothing written\n", a...)
		return 1
	}
	if root == nil {
		return refuse("nothing of %s could be read, so no test or requirement can be compared; fix the errors above first", l.spec.RootFile)
	}

	// The implementation files, each one of the specification's.
	own := map[string]spec.Implementation{}
	var ownNames []string
	for _, impl := range l.spec.Implementations {
		abs, err := filepath.Abs(impl.Path)
		if err != nil {
			continue
		}
		own[abs] = impl
		ownNames = append(ownNames, filepath.ToSlash(impl.Path))
	}
	sort.Strings(ownNames)
	if len(ownNames) == 0 {
		ownNames = []string{"none"}
	}
	c := &comparison{Format: comparisonFormat, Cases: []compareCaseRow{}, Measurements: []compareMeasureRow{}, Libraries: []compareLibraryRow{}, Findings: []compareFindingRow{}, Gaps: []compareGap{}}
	info := source.Child(root, "info")
	c.Title, c.Version = source.Str(source.Child(info, "title")), source.Str(source.Child(info, "version"))
	index := map[string]int{}
	var implRoots []*yaml.Node
	for _, f := range files {
		abs, err := filepath.Abs(f)
		if err != nil {
			return refuse("%s cannot be placed: %s", f, spec.PlainIOError(err))
		}
		impl, ok := own[abs]
		if !ok {
			return refuse("%s is not an implementation file of %s; its implementation files are %s", f, l.spec.Dir, strings.Join(ownNames, ", "))
		}
		name := strings.TrimSuffix(filepath.Base(impl.Path), spec.ImplementationSuffix)
		if _, twice := index[name]; twice {
			return refuse("%s is given twice; name each implementation file once", f)
		}
		index[name] = len(c.Implementations)
		doc := source.Parse(impl.Data)
		implRoots = append(implRoots, doc.Root)
		c.Implementations = append(c.Implementations, compareImplementation{
			Name: name, File: filepath.ToSlash(impl.Path), SHA256: sha256Of(impl.Data), Inputs: []compareInput{},
			Version: source.Str(source.Child(source.Child(doc.Root, "info"), "version")),
		})
	}
	names := make([]string, len(c.Implementations))
	for i, im := range c.Implementations {
		names[i] = im.Name
	}

	// Each option's values, by implementation.
	byImpl := map[string]map[int]string{}
	for _, o := range compareOptions {
		byImpl[o] = map[int]string{}
		for _, v := range given[o] {
			name, value, ok := strings.Cut(v, "=")
			if !ok || name == "" || value == "" {
				return refuse("--%s %s is not <name>=<value>; write it as an implementation's name, =, and the value, such as %s=%s", o, v, names[0], exampleValue(o))
			}
			i, known := index[name]
			if !known {
				return refuse("--%s names %s, which is no implementation file given; the names are %s", o, name, strings.Join(names, ", "))
			}
			if _, twice := byImpl[o][i]; twice {
				return refuse("--%s names %s twice; give it once per implementation", o, name)
			}
			byImpl[o][i] = value
		}
	}
	for i, name := range names {
		if byImpl["test-run"][i] != "" && byImpl["results"][i] != "" {
			return refuse("%s has both --test-run and --results; give one, the test-run record when the run is recorded", name)
		}
	}

	recordsDir, err := filepath.Abs(l.spec.RecordsDir)
	if err != nil {
		return refuse("%s cannot be placed: %s", l.spec.RecordsDir, spec.PlainIOError(err))
	}
	recordsParent := filepath.Dir(recordsDir)
	for i := range c.Implementations {
		if full, err := filepath.Abs(c.Implementations[i].File); err == nil {
			if rel, err := filepath.Rel(recordsParent, full); err == nil {
				c.Implementations[i].File = filepath.ToSlash(rel)
			}
		}
	}
	// readInput reads a file an option names, which must be under records/.
	readInput := func(option, p string) ([]byte, compareInput, error) {
		full, err := filepath.Abs(p)
		if err != nil {
			return nil, compareInput{}, fmt.Errorf("--%s %s cannot be read: %s", option, p, spec.PlainIOError(err))
		}
		rel, err := filepath.Rel(recordsDir, full)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, compareInput{}, fmt.Errorf("--%s %s is not under %s/; put the file under records/ beside the specification's folder, such as records/evidence/, and name it from there, so the comparison can name it as its input", option, p, l.spec.RecordsDir)
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, compareInput{}, fmt.Errorf("--%s %s cannot be read: %s", option, p, spec.PlainIOError(err))
		}
		from, _ := filepath.Rel(recordsParent, full)
		return data, compareInput{Option: option, Path: filepath.ToSlash(from), SHA256: sha256Of(data)}, nil
	}

	// The test results, each mapped to its test.
	testsNode := source.Child(root, "tests")
	tests := sortedPairKeys(testsNode)
	specification, _ := l.spec.Value.(map[string]any)
	worked := gentests.WorkedTests(specification)
	outcomes := make([]map[string][]string, len(names)) // test -> outcomes, per implementation
	for i, name := range names {
		im := &c.Implementations[i]
		var results []compareOutcome
		switch {
		case byImpl["test-run"][i] != "":
			id := byImpl["test-run"][i]
			if !testRunID.MatchString(id) {
				return refuse("--test-run %s=%s is not a test-run record's ID, TR- and a number", name, id)
			}
			rec, input, found := testRunRecord(l.spec, recordsParent, id)
			if !found {
				ids := parsedRecords(l.spec).ids("test-run")
				if len(ids) == 0 {
					return refuse("--test-run %s=%s: the records hold no test-run record; record the run with specarch log test-run first", name, id)
				}
				sort.Slice(ids, func(a, b int) bool { return naturalLess(ids[a], ids[b]) })
				return refuse("--test-run %s=%s: the records hold no test-run %s; record the run with specarch log test-run, or name one of %s", name, id, id, strings.Join(ids, ", "))
			}
			im.Inputs = append(im.Inputs, input)
			for _, res := range source.Items(source.Child(rec, "results")) {
				results = append(results, compareOutcome{
					test:    source.Str(source.Child(res, "test")),
					element: source.Str(source.Child(res, "element")),
					outcome: source.Str(source.Child(res, "outcome")),
				})
			}
		case byImpl["results"][i] != "":
			data, input, err := readInput("results", byImpl["results"][i])
			if err != nil {
				return refuse("%v", err)
			}
			cases, err := readJUnit(data)
			if err != nil {
				return refuse("--results %s is not JUnit XML: %v", byImpl["results"][i], err)
			}
			if len(cases) == 0 {
				return refuse("--results %s holds no testcase, so there is no result to compare", byImpl["results"][i])
			}
			im.Inputs = append(im.Inputs, input)
			for _, tc := range cases {
				t, el, _ := mapTestCase(tc.name, tests, worked)
				results = append(results, compareOutcome{test: t, element: el, outcome: tc.outcome})
			}
		default:
			c.Gaps = append(c.Gaps, compareGap{Implementation: name, Row: "test results", Reason: "no test-run record or results file is given for it",
				Fill: fmt.Sprintf("--test-run %s=TR-<n>, or --results %s=<JUnit XML file under records/>", name, name)})
			continue
		}
		counts := &compareResults{}
		outcomes[i] = map[string][]string{}
		for _, r := range results {
			switch r.outcome {
			case "passed":
				counts.Passed++
			case "failed":
				counts.Failed++
			case "error":
				counts.Errored++
			case "skipped":
				counts.Skipped++
			}
			if r.test == "" && r.element == "" {
				counts.Unmapped++
			}
			if r.test != "" {
				outcomes[i][r.test] = append(outcomes[i][r.test], r.outcome)
			}
		}
		im.Results = counts
	}

	// One row per derived case that is chosen or covered.
	for _, ct := range validate.CoveredCases(root, l.given) {
		row := compareCaseRow{Subject: ct.Subject, Case: ct.Case, Scenario: ct.Scenario, Tests: ct.Tests}
		if row.Tests == nil {
			row.Tests = []string{}
			c.Gaps = append(c.Gaps, compareGap{Row: ct.Subject + ": " + ct.Case, Reason: "no test of the specification covers this derived case",
				Fill: "a test that covers it; specarch derive writes its draft"})
		}
		for i, name := range names {
			cell := caseCell(ct.Tests, outcomes[i])
			im := &c.Implementations[i]
			switch cell.Outcome {
			case "passed":
				im.Cases.Passed++
			case "failed":
				im.Cases.Failed++
			case "skipped":
				im.Cases.Skipped++
			default:
				im.Cases.Gaps++
			}
			if cell.Gap == "not run" {
				c.Gaps = append(c.Gaps, compareGap{Implementation: name, Row: ct.Subject + ": " + ct.Case,
					Reason: "no result of its tests, " + strings.Join(ct.Tests, ", ") + ", is in the run", Fill: "a run that holds them"})
			}
			row.Cells = append(row.Cells, cell)
		}
		c.Cases = append(c.Cases, row)
	}

	// The libraries of each implementation file.
	libraries := map[string][]*compareLibraryCell{}
	for i, r := range implRoots {
		for _, p := range source.Pairs(source.Child(r, "libraries")) {
			if libraries[p.Key.Value] == nil {
				libraries[p.Key.Value] = make([]*compareLibraryCell, len(names))
			}
			scope := source.Str(source.Child(p.Value, "scope"))
			if scope == "" {
				scope = "runtime"
			}
			libraries[p.Key.Value][i] = &compareLibraryCell{Version: source.Str(source.Child(p.Value, "version")), Licence: source.Str(source.Child(p.Value, "licence")), Scope: scope}
			c.Implementations[i].Libraries++
		}
	}
	for _, name := range sortedKeys(libraries) {
		c.Libraries = append(c.Libraries, compareLibraryRow{Name: name, Cells: libraries[name]})
	}

	// The scans.
	type findingKey struct{ id, pkg string }
	findings := map[findingKey][]*compareFindingCell{}
	scanned := make([]bool, len(names))
	for i, name := range names {
		p := byImpl["scan"][i]
		if p == "" {
			c.Gaps = append(c.Gaps, compareGap{Implementation: name, Row: "scan findings", Reason: "no scan of its software bill of materials is given for it",
				Fill: fmt.Sprintf("--scan %s=<grype or osv-scanner JSON under records/>", name)})
			continue
		}
		data, input, err := readInput("scan", p)
		if err != nil {
			return refuse("%v", err)
		}
		found, err := readScan(data)
		if err != nil {
			return refuse("--scan %s is not a scan compare reads: %v", p, err)
		}
		c.Implementations[i].Inputs = append(c.Implementations[i].Inputs, input)
		scanned[i] = true
		sev := &compareSeverities{}
		for _, f := range found {
			k := findingKey{f.id, f.pkg}
			if findings[k] == nil {
				findings[k] = make([]*compareFindingCell, len(names))
			}
			cell := findings[k][i]
			if cell == nil {
				findings[k][i] = &compareFindingCell{Severity: f.severity, Version: f.version}
				continue
			}
			if severityRank(f.severity) < severityRank(cell.Severity) {
				cell.Severity = f.severity
			}
			if f.version != "" && !strings.Contains(", "+cell.Version+", ", ", "+f.version+", ") {
				vs := append(strings.Split(cell.Version, ", "), f.version)
				sort.Strings(vs)
				cell.Version = strings.TrimPrefix(strings.Join(vs, ", "), ", ")
			}
		}
		for _, cells := range findings {
			if cell := cells[i]; cell != nil {
				sev.add(cell.Severity)
			}
		}
		c.Implementations[i].Findings = sev
	}
	var keys []findingKey
	for k, cells := range findings {
		keys = append(keys, k)
		for i := range names {
			if !scanned[i] {
				cells[i] = &compareFindingCell{Gap: "no scan"}
			}
		}
	}
	sort.Slice(keys, func(a, b int) bool {
		ra, rb := gravest(findings[keys[a]]), gravest(findings[keys[b]])
		if ra != rb {
			return ra < rb
		}
		if keys[a].id != keys[b].id {
			return keys[a].id < keys[b].id
		}
		return keys[a].pkg < keys[b].pkg
	})
	for _, k := range keys {
		c.Findings = append(c.Findings, compareFindingRow{ID: k.id, Package: k.pkg, Cells: findings[k]})
	}

	// The measurements.
	measured := make([]*measurements, len(names))
	for i, name := range names {
		p := byImpl["measurements"][i]
		if p == "" {
			continue
		}
		data, input, err := readInput("measurements", p)
		if err != nil {
			return refuse("%v", err)
		}
		m, err := readMeasurements(data, source.Child(root, "requirements"))
		if err != nil {
			return refuse("--measurements %s=%s: %v", name, p, err)
		}
		m.path = input.Path
		measured[i] = m
		c.Implementations[i].Inputs = append(c.Implementations[i].Inputs, input)
	}
	for _, key := range []struct{ key, row string }{{"buildSeconds", "build time"}, {"linesOfCode", "lines of code"}, {"deploySteps", "deploy steps"}} {
		for i, name := range names {
			m := measured[i]
			var v *int64
			if m != nil {
				v = m.whole[key.key]
			}
			switch key.key {
			case "buildSeconds":
				c.Implementations[i].BuildSeconds = v
			case "linesOfCode":
				c.Implementations[i].LinesOfCode = v
			case "deploySteps":
				c.Implementations[i].DeploySteps = v
			}
			if v == nil {
				c.Gaps = append(c.Gaps, measurementGap(name, key.row, key.key, m))
			}
		}
	}
	type measureKey struct{ requirement, quantity string }
	rows := map[measureKey]bool{}
	for _, m := range measured {
		if m == nil {
			continue
		}
		for req, qs := range m.requirements {
			for q := range qs {
				rows[measureKey{req, q}] = true
			}
		}
	}
	var mkeys []measureKey
	for k := range rows {
		mkeys = append(mkeys, k)
	}
	sort.Slice(mkeys, func(a, b int) bool {
		if mkeys[a].requirement != mkeys[b].requirement {
			return naturalLess(mkeys[a].requirement, mkeys[b].requirement)
		}
		return mkeys[a].quantity < mkeys[b].quantity
	})
	requirements := source.Child(root, "requirements")
	for _, k := range mkeys {
		row := compareMeasureRow{Requirement: k.requirement, Quantity: k.quantity,
			Statement: oneLineText(source.Str(source.Child(source.Child(requirements, k.requirement), "statement")))}
		for i, name := range names {
			m := measured[i]
			if m != nil {
				if v, ok := m.requirements[k.requirement][k.quantity]; ok {
					value := v.value
					row.Cells = append(row.Cells, compareMeasureCell{Value: &value, Unit: v.unit, How: v.how})
					continue
				}
			}
			row.Cells = append(row.Cells, compareMeasureCell{Gap: "not measured"})
			c.Gaps = append(c.Gaps, measurementGap(name, k.requirement+" "+k.quantity, "requirements."+k.requirement+"."+k.quantity, m))
		}
		c.Measurements = append(c.Measurements, row)
	}
	if kept != nil {
		*kept = c
	}

	// The output.
	markdown := compareMarkdown(c, l.spec, out)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "specarch compare: cannot encode the comparison as JSON: %v\n", err)
		return 2
	}
	data = append(data, '\n')
	summary := compareCounts(c)
	if out == "" {
		fmt.Fprint(stdout, markdown)
		fmt.Fprintf(stderr, "specarch compare: %s\n", summary)
	} else {
		mdPath, jsonPath := filepath.Join(out, "comparison.md"), filepath.Join(out, "comparison.json")
		if !w.Dry() {
			if err := os.MkdirAll(out, 0o755); err != nil {
				fmt.Fprintf(stderr, "specarch compare: cannot make %s: %v\n", out, err)
				return 2
			}
		}
		for _, f := range []struct {
			path string
			data []byte
		}{{mdPath, []byte(markdown)}, {jsonPath, data}} {
			if err := w.WriteFile(f.path, f.data, 0o644); err != nil {
				fmt.Fprintf(stderr, "specarch compare: cannot write %s: %v\n", f.path, err)
				return 2
			}
			if !w.Dry() {
				fmt.Fprintln(stdout, filepath.ToSlash(f.path))
			}
		}
		for _, line := range citeLines(c, l.spec.Dir, out, mdPath, data) {
			fmt.Fprintln(stdout, line)
		}
		if w.Dry() {
			fmt.Fprintf(stderr, "specarch compare: %s; nothing written (--dry-run)\n", summary)
		} else {
			fmt.Fprintf(stderr, "specarch compare: %s; written to %s and %s\n", summary, filepath.ToSlash(mdPath), filepath.ToSlash(jsonPath))
		}
	}
	if invalid {
		fmt.Fprintln(stderr, "specarch compare: the input has errors; they are printed above")
		return 1
	}
	return 0
}

// compareOutcome is one result of a run: the test or element it maps to,
// and its outcome.
type compareOutcome struct{ test, element, outcome string }

// caseCell is a derived case's cell in one implementation: failed when a
// result of a test that covers it failed or errored, else passed when one
// passed, else skipped when one was skipped; a gap otherwise.
func caseCell(tests []string, outcomes map[string][]string) compareCell {
	if len(tests) == 0 {
		return compareCell{Gap: "no test"}
	}
	if outcomes == nil {
		return compareCell{Gap: "no results"}
	}
	seen := map[string]bool{}
	for _, t := range tests {
		for _, o := range outcomes[t] {
			seen[o] = true
		}
	}
	switch {
	case seen["failed"] || seen["error"]:
		return compareCell{Outcome: "failed"}
	case seen["passed"]:
		return compareCell{Outcome: "passed"}
	case seen["skipped"]:
		return compareCell{Outcome: "skipped"}
	}
	return compareCell{Gap: "not run"}
}

// testRunRecord finds the test-run record of that ID, with its file as
// an input.
func testRunRecord(s *spec.Spec, recordsParent, id string) (*yaml.Node, compareInput, bool) {
	for _, r := range s.Records {
		doc := source.Parse(r.Data)
		if doc.Root == nil || source.Str(source.Child(doc.Root, "kind")) != "test-run" || source.Str(source.Child(doc.Root, "id")) != id {
			continue
		}
		full, _ := filepath.Abs(r.Path)
		from, _ := filepath.Rel(recordsParent, full)
		return doc.Root, compareInput{Option: "test-run", Path: filepath.ToSlash(from), SHA256: sha256Of(r.Data)}, true
	}
	return nil, compareInput{}, false
}

func sha256Of(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func exampleValue(option string) string {
	switch option {
	case "test-run":
		return "TR-1"
	case "results":
		return "records/evidence/junit.xml"
	case "scan":
		return "records/evidence/grype.json"
	}
	return "records/evidence/measurements.json"
}

// naturalLess orders IDs such as SA-2 before SA-10: the text before the
// last run of digits, then that number.
func naturalLess(a, b string) bool {
	pa, na := splitNumber(a)
	pb, nb := splitNumber(b)
	if pa != pb || na == nb {
		if pa == pb {
			return a < b
		}
		return pa < pb
	}
	return na < nb
}

func splitNumber(s string) (string, int) {
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	n, err := strconv.Atoi(s[i:])
	if err != nil {
		return s, -1
	}
	return s[:i], n
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func severityRank(s string) int {
	for i, o := range severityOrder {
		if o == s {
			return i
		}
	}
	return len(severityOrder)
}

// gravest is the rank of the gravest severity of a finding's cells.
func gravest(cells []*compareFindingCell) int {
	best := len(severityOrder)
	for _, c := range cells {
		if c != nil && c.Gap == "" && severityRank(c.Severity) < best {
			best = severityRank(c.Severity)
		}
	}
	return best
}

func (s *compareSeverities) add(severity string) {
	switch severity {
	case "critical":
		s.Critical++
	case "high":
		s.High++
	case "medium":
		s.Medium++
	case "low":
		s.Low++
	case "negligible":
		s.Negligible++
	default:
		s.Unknown++
	}
}

// scanFinding is one finding of a scan: a vulnerability in a package.
type scanFinding struct{ id, pkg, version, severity string }

// readScan reads grype's JSON, told by its matches, or osv-scanner's,
// told by its results.
func readScan(data []byte) ([]scanFinding, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("it is not a JSON object: %v", err)
	}
	if raw, ok := top["matches"]; ok {
		var matches []struct {
			Vulnerability struct {
				ID       string `json:"id"`
				Severity string `json:"severity"`
			} `json:"vulnerability"`
			Artifact struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"artifact"`
		}
		if err := json.Unmarshal(raw, &matches); err != nil {
			return nil, fmt.Errorf("its matches are not grype's: %v", err)
		}
		var out []scanFinding
		for i, m := range matches {
			if m.Vulnerability.ID == "" || m.Artifact.Name == "" {
				return nil, fmt.Errorf("match %d of %d names no vulnerability id or no artifact name, which grype's JSON gives each match", i+1, len(matches))
			}
			sev := strings.ToLower(m.Vulnerability.Severity)
			if severityRank(sev) == len(severityOrder) {
				sev = "unknown"
			}
			out = append(out, scanFinding{id: m.Vulnerability.ID, pkg: m.Artifact.Name, version: m.Artifact.Version, severity: sev})
		}
		return out, nil
	}
	if raw, ok := top["results"]; ok {
		var results []struct {
			Packages []struct {
				Package struct {
					Name    string `json:"name"`
					Version string `json:"version"`
				} `json:"package"`
				Vulnerabilities []struct {
					ID string `json:"id"`
				} `json:"vulnerabilities"`
				Groups []struct {
					IDs         []string `json:"ids"`
					MaxSeverity string   `json:"max_severity"`
				} `json:"groups"`
			} `json:"packages"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return nil, fmt.Errorf("its results are not osv-scanner's: %v", err)
		}
		var out []scanFinding
		for _, r := range results {
			for _, p := range r.Packages {
				if p.Package.Name == "" {
					return nil, fmt.Errorf("a package names no name, which osv-scanner's JSON gives each package")
				}
				if len(p.Groups) == 0 {
					for _, v := range p.Vulnerabilities {
						out = append(out, scanFinding{id: v.ID, pkg: p.Package.Name, version: p.Package.Version, severity: "unknown"})
					}
					continue
				}
				for _, g := range p.Groups {
					if len(g.IDs) == 0 {
						return nil, fmt.Errorf("a group of package %s names no id, which osv-scanner's JSON gives each group", p.Package.Name)
					}
					out = append(out, scanFinding{id: g.IDs[0], pkg: p.Package.Name, version: p.Package.Version, severity: cvssSeverity(g.MaxSeverity)})
				}
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("it is neither grype's JSON, which holds matches, nor osv-scanner's, which holds results")
}

// cvssSeverity reads a CVSS base score into the qualitative scale of
// CVSS v3.1, section 5; unknown when there is no score. 0.0, None on that
// scale, is negligible, grype's word for it, so both scanners' findings
// are counted in one list of words (ADR-232).
func cvssSeverity(score string) string {
	v, err := strconv.ParseFloat(strings.TrimSpace(score), 64)
	switch {
	case err != nil:
		return "unknown"
	case v >= 9.0:
		return "critical"
	case v >= 7.0:
		return "high"
	case v >= 4.0:
		return "medium"
	case v > 0:
		return "low"
	case v == 0:
		return "negligible"
	}
	return "unknown"
}

// measurements is what a measurements file gives.
type measurements struct {
	path         string
	whole        map[string]*int64
	requirements map[string]map[string]measured
}

type measured struct {
	value     float64
	unit, how string
}

// readMeasurements reads a measurements file, a JSON object:
// buildSeconds, linesOfCode and deploySteps, each a whole number from 0,
// and requirements, keyed by a requirement of the specification, each
// with latency or throughput as {value, unit, how}. Every key is optional
// and no other is read. It is JSON, not YAML, since every YAML file under
// records/ is read as a record.
func readMeasurements(data []byte, requirements *yaml.Node) (*measurements, error) {
	m := &measurements{whole: map[string]*int64{}, requirements: map[string]map[string]measured{}}
	top, err := jsonObject(data)
	if err != nil {
		return nil, fmt.Errorf("it is not a JSON object: %v; its keys are buildSeconds, linesOfCode, deploySteps and requirements", err)
	}
	for _, k := range sortedKeys(top) {
		raw := top[k]
		switch k {
		case "buildSeconds", "linesOfCode", "deploySteps":
			v, err := wholeNumber(raw)
			if err != nil {
				return nil, fmt.Errorf("%s is %s, not a whole number from 0", k, strings.TrimSpace(string(raw)))
			}
			m.whole[k] = &v
		case "requirements":
			reqs, err := jsonObject(raw)
			if err != nil {
				return nil, fmt.Errorf("requirements is not an object keyed by requirement ID: %v", err)
			}
			for _, id := range sortedKeys(reqs) {
				if source.Child(requirements, id) == nil {
					return nil, fmt.Errorf("%s is not a requirement of the specification", id)
				}
				qs, err := jsonObject(reqs[id])
				if err != nil {
					return nil, fmt.Errorf("%s is not an object of latency or throughput: %v", id, err)
				}
				m.requirements[id] = map[string]measured{}
				for _, q := range sortedKeys(qs) {
					if q != "latency" && q != "throughput" {
						return nil, fmt.Errorf("%s has the key %s, which compare does not read; its keys are latency and throughput", id, q)
					}
					got, err := readMeasured(qs[q])
					if err != nil {
						return nil, fmt.Errorf("%s %s %v", id, q, err)
					}
					m.requirements[id][q] = got
				}
			}
		default:
			return nil, fmt.Errorf("it has the key %s, which compare does not read; its keys are buildSeconds, linesOfCode, deploySteps and requirements", k)
		}
	}
	return m, nil
}

// jsonObject decodes a JSON object into its members, each left raw; a
// key given twice is refused, since which one counts would be a guess.
func jsonObject(data []byte) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("it is %s, not an object", strings.TrimSpace(string(data)))
	}
	out := map[string]json.RawMessage{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := tok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		if _, twice := out[key]; twice {
			return nil, fmt.Errorf("the key %s is given twice", key)
		}
		out[key] = raw
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("something follows the object")
	}
	return out, nil
}

// wholeNumber reads a JSON number with no fraction and no exponent, from 0.
func wholeNumber(raw json.RawMessage) (int64, error) {
	text := strings.TrimSpace(string(raw))
	v, err := strconv.ParseInt(text, 10, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("not a whole number from 0")
	}
	return v, nil
}

func readMeasured(raw json.RawMessage) (measured, error) {
	var out measured
	members, err := jsonObject(raw)
	if err != nil {
		return out, fmt.Errorf("is not an object of value, unit and how")
	}
	hasValue := false
	for _, k := range sortedKeys(members) {
		v := members[k]
		switch k {
		case "value":
			var n json.Number
			dec := json.NewDecoder(bytes.NewReader(v))
			dec.UseNumber()
			if err := dec.Decode(&n); err != nil || strings.HasPrefix(strings.TrimSpace(string(v)), `"`) {
				return out, fmt.Errorf("has the value %s, not a number", strings.TrimSpace(string(v)))
			}
			f, err := n.Float64()
			if err != nil {
				return out, fmt.Errorf("has the value %s, not a number", n)
			}
			out.value, hasValue = f, true
		case "unit", "how":
			var text string
			if err := json.Unmarshal(v, &text); err != nil || strings.TrimSpace(text) == "" {
				return out, fmt.Errorf("has a %s that is not words", k)
			}
			if k == "unit" {
				out.unit = oneLineText(text)
			} else {
				out.how = oneLineText(text)
			}
		default:
			return out, fmt.Errorf("has the key %s, which compare does not read; its keys are value, unit and how", k)
		}
	}
	if !hasValue || out.unit == "" {
		return out, fmt.Errorf("gives no value or no unit; both are needed")
	}
	return out, nil
}

// measurementGap is the gap of a value a measurements file does not give.
func measurementGap(name, row, key string, m *measurements) compareGap {
	if m == nil {
		return compareGap{Implementation: name, Row: row, Reason: "no measurements file is given for it", Fill: fmt.Sprintf("--measurements %s=<JSON file under records/>", name)}
	}
	return compareGap{Implementation: name, Row: row, Reason: "its measurements file " + m.path + " does not give " + key, Fill: key + " in " + m.path}
}

// citeLines are the lines that say how a decision cites the comparison:
// the sources entry, kind document, the SHA-256 of comparison.json its
// edition and the path of comparison.md from the specification's folder
// its url.
func citeLines(c *comparison, specDir, out, mdPath string, jsonData []byte) []string {
	key := "comparison"
	if k := kebabName(filepath.Base(filepath.Clean(out))); k != "" && k != "." {
		key = k
		if !strings.HasPrefix(k, "comparison") {
			key = "comparison-" + k
		}
	}
	url := filepath.ToSlash(mdPath)
	if absMD, err := filepath.Abs(mdPath); err == nil {
		if absDir, err := filepath.Abs(specDir); err == nil {
			if rel, err := filepath.Rel(absDir, absMD); err == nil {
				url = filepath.ToSlash(rel)
			}
		}
	}
	entry := map[string]string{"kind": "document", "title": compareTitle(c), "edition": sha256Of(jsonData), "url": url}
	var b strings.Builder
	b.WriteString("  " + key + ": {")
	for i, k := range []string{"kind", "title", "edition", "url"} {
		if i > 0 {
			b.WriteString(", ")
		}
		v, _ := json.Marshal(entry[k])
		b.WriteString(k + ": " + string(v))
	}
	b.WriteString("}")
	return []string{
		"to cite the comparison from a decision, declare it under sources in specarch.yaml:",
		b.String(),
		fmt.Sprintf("and cite it in the decision: cites: [{source: %s, clause: <the section, such as Derived cases>, says: <what it shows>}]", key),
	}
}

var kebabRun = regexp.MustCompile(`[^a-z0-9]+`)

// kebabName is a name made a source key: lower case, each run of other
// characters a dash, starting with a letter; "" when nothing is left.
func kebabName(s string) string {
	k := strings.Trim(kebabRun.ReplaceAllString(strings.ToLower(s), "-"), "-")
	for k != "" && (k[0] < 'a' || k[0] > 'z') {
		k = strings.TrimLeft(k[1:], "-")
	}
	return k
}

// compareTitle names the implementations compared.
func compareTitle(c *comparison) string {
	names := make([]string, len(c.Implementations))
	for i, im := range c.Implementations {
		names[i] = im.Name
	}
	switch len(names) {
	case 1:
		return "Comparison of " + names[0]
	case 2:
		return "Comparison of " + names[0] + " and " + names[1]
	}
	return "Comparison of " + strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// compareMarkdown writes the comparison as Markdown, the summary first.
func compareMarkdown(c *comparison, s *spec.Spec, out string) string {
	var b strings.Builder
	line := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	base := out
	if base == "" {
		base = "."
	}
	from := filepath.ToSlash(s.RootFile)
	if rel, err := relFrom(base, s.RootFile); err == nil {
		from = rel
	}
	var files []string
	for _, im := range c.Implementations {
		p := filepath.Join(filepath.Dir(s.RecordsDir), filepath.FromSlash(im.File))
		if rel, err := relFrom(base, p); err == nil {
			p = rel
		}
		files = append(files, filepath.ToSlash(p))
	}
	line("<!-- Generated by specarch compare from %s, version %s, and %s. Do not edit this file: run specarch compare again. -->", from, c.Version, strings.Join(files, ", "))
	line("")
	line("# %s", mdText(compareTitle(c)))
	line("")
	line("%s %s: %s compared; %s, each written gap in its cell and listed under Gaps.", mdText(c.Title), c.Version, plural(len(c.Implementations), "implementation file"), compareCounts(c))
	line("")
	header := func(first ...string) {
		cols := append([]string{}, first...)
		for _, im := range c.Implementations {
			cols = append(cols, mdText(im.Name))
		}
		line("| %s |", strings.Join(cols, " | "))
		seps := make([]string, len(cols))
		for i := range seps {
			seps[i] = "---"
		}
		line("|%s|", strings.Join(seps, "|"))
	}
	row := func(cells ...string) { line("| %s |", strings.Join(cells, " | ")) }
	per := func(label string, f func(im compareImplementation) string) {
		cells := []string{label}
		for _, im := range c.Implementations {
			cells = append(cells, f(im))
		}
		row(cells...)
	}
	header("Summary")
	per("Derived cases passed", func(im compareImplementation) string {
		return fmt.Sprintf("%d of %d", im.Cases.Passed, len(c.Cases))
	})
	per("Test results", func(im compareImplementation) string {
		if im.Results == nil {
			return "gap: no results"
		}
		r := im.Results
		return fmt.Sprintf("%d passed, %d failed, %d errored, %d skipped, %d unmapped", r.Passed, r.Failed, r.Errored, r.Skipped, r.Unmapped)
	})
	per("Scan findings", func(im compareImplementation) string {
		if im.Findings == nil {
			return "gap: no scan"
		}
		return severitiesText(im.Findings)
	})
	per("Libraries", func(im compareImplementation) string { return strconv.Itoa(im.Libraries) })
	whole := func(v *int64, unit string) string {
		if v == nil {
			return "gap: not measured"
		}
		return strconv.FormatInt(*v, 10) + unit
	}
	per("Build time", func(im compareImplementation) string { return whole(im.BuildSeconds, " s") })
	per("Lines of code", func(im compareImplementation) string { return whole(im.LinesOfCode, "") })
	per("Deploy steps", func(im compareImplementation) string { return whole(im.DeploySteps, "") })
	per("Gaps", func(im compareImplementation) string {
		n := 0
		for _, g := range c.Gaps {
			if g.Implementation == im.Name || g.Implementation == "" {
				n++
			}
		}
		return strconv.Itoa(n)
	})

	line("")
	line("## Derived cases")
	line("")
	if len(c.Cases) == 0 {
		line("The specification derives no case a test must cover.")
	} else {
		line("A case is failed when a result of a test that covers it failed or errored, passed when one passed, skipped when one was skipped.")
		line("")
		header("Subject", "Case", "Scenario", "Tests")
		for _, r := range c.Cases {
			cells := []string{mdText(r.Subject), mdText(r.Case), r.Scenario, mdText(orDash(strings.Join(r.Tests, ", ")))}
			for _, cell := range r.Cells {
				if cell.Gap != "" {
					cells = append(cells, "gap: "+cell.Gap)
				} else {
					cells = append(cells, cell.Outcome)
				}
			}
			row(cells...)
		}
	}

	line("")
	line("## Latency and throughput")
	line("")
	if len(c.Measurements) == 0 {
		line("No measurements file names a requirement.")
	} else {
		line("Each value stands beside the requirement's statement, which gives its target in words; read the two together.")
		line("")
		header("Requirement", "Statement", "Quantity")
		for _, r := range c.Measurements {
			cells := []string{r.Requirement, mdText(r.Statement), r.Quantity}
			for _, cell := range r.Cells {
				if cell.Gap != "" {
					cells = append(cells, "gap: "+cell.Gap)
					continue
				}
				text := strconv.FormatFloat(*cell.Value, 'f', -1, 64) + " " + cell.Unit
				if cell.How != "" {
					text += " (" + cell.How + ")"
				}
				cells = append(cells, mdText(text))
			}
			row(cells...)
		}
	}

	line("")
	line("## Libraries")
	line("")
	if len(c.Libraries) == 0 {
		line("No implementation file lists a library.")
	} else {
		header("Library")
		for _, r := range c.Libraries {
			cells := []string{mdText(r.Name)}
			for _, cell := range r.Cells {
				if cell == nil {
					cells = append(cells, "not used")
					continue
				}
				cells = append(cells, mdText(cell.Version+", "+cell.Licence+", "+cell.Scope))
			}
			row(cells...)
		}
	}

	line("")
	line("## Scan findings")
	line("")
	if len(c.Findings) == 0 {
		line("No scan given holds a finding.")
	} else {
		header("Finding", "Package")
		for _, r := range c.Findings {
			cells := []string{mdText(r.ID), mdText(r.Package)}
			for _, cell := range r.Cells {
				switch {
				case cell == nil:
					cells = append(cells, "not found")
				case cell.Gap != "":
					cells = append(cells, "gap: "+cell.Gap)
				case cell.Version != "":
					cells = append(cells, mdText(cell.Severity+", "+cell.Version))
				default:
					cells = append(cells, cell.Severity)
				}
			}
			row(cells...)
		}
	}

	line("")
	line("## Inputs")
	line("")
	line("| Implementation | Option | File | SHA-256 |")
	line("|---|---|---|---|")
	for _, im := range c.Implementations {
		line("| %s | %s | %s | %s |", mdText(im.Name), "implementation file", mdText(im.File), im.SHA256)
		for _, in := range im.Inputs {
			line("| %s | --%s | %s | %s |", mdText(im.Name), in.Option, mdText(in.Path), in.SHA256)
		}
	}

	line("")
	line("## Gaps")
	line("")
	if len(c.Gaps) == 0 {
		line("None: every cell holds a result.")
	} else {
		for _, g := range c.Gaps {
			who := g.Implementation
			if who == "" {
				who = "every implementation"
			}
			line("- %s: %s: %s; give %s.", mdText(who), mdText(g.Row), mdText(g.Reason), mdText(g.Fill))
		}
	}
	return b.String()
}

// compareCounts counts the rows and the gaps of a comparison.
func compareCounts(c *comparison) string {
	return fmt.Sprintf("%s, %s, %s, %s and %s",
		plural(len(c.Cases), "derived case"), counted(len(c.Measurements), "measured quantity", "measured quantities"),
		counted(len(c.Libraries), "library", "libraries"), plural(len(c.Findings), "scan finding"), plural(len(c.Gaps), "gap"))
}

// counted is plural for a word whose plural is not the word and an s.
func counted(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// relFrom is target's path from the folder base, with slashes.
func relFrom(base, target string) (string, error) {
	ab, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	at, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(ab, at)
	return filepath.ToSlash(rel), err
}

func severitiesText(s *compareSeverities) string {
	var parts []string
	for _, p := range []struct {
		n    int
		word string
	}{{s.Critical, "critical"}, {s.High, "high"}, {s.Medium, "medium"}, {s.Low, "low"}, {s.Negligible, "negligible"}, {s.Unknown, "unknown"}} {
		if p.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", p.n, p.word))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func orDash(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// mdText is text made safe for one Markdown table cell.
func mdText(s string) string {
	return strings.ReplaceAll(oneLineText(s), "|", `\|`)
}
