package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/disk"
	"github.com/SpecArch/specarch/internal/gentests"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// logUnmapped is a test case of a results file that maps to no test of
// the specification, and why.
type logUnmapped struct {
	Name string `json:"name"`
	Why  string `json:"why"`
}

// testRunOptions are the options of specarch log test-run that take one
// value, and testRunLists the ones that take several.
var (
	testRunOptions = []string{"results", "environment", "version", "build", "digest", "at", "by", "severity", "phase"}
	testRunLists   = []string{"open-defect"}
)

var (
	defectSeverities = []string{"critical", "major", "minor", "cosmetic"}
	defectPhases     = []string{"development", "acceptance", "production"}
	digestPattern    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// junitCase is one testcase of a JUnit XML file, as the record keeps it.
type junitCase struct {
	name    string
	outcome string // passed, failed, error or skipped
	message string
}

// junitTestcase is a testcase element, decoded.
type junitTestcase struct {
	Name     string         `xml:"name,attr"`
	Failures []junitProblem `xml:"failure"`
	Errors   []junitProblem `xml:"error"`
	Skipped  []junitProblem `xml:"skipped"`
}

type junitProblem struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

// line is what a failure, an error or a skip says, in one line: its
// message, or the first line of its text when it has none.
func (p junitProblem) line() string {
	if m := oneLineText(p.Message); m != "" {
		return m
	}
	for _, l := range strings.Split(p.Text, "\n") {
		if l = oneLineText(l); l != "" {
			return l
		}
	}
	return ""
}

func oneLineText(s string) string { return strings.Join(strings.Fields(s), " ") }

// readJUnit reads the testcases of a JUnit XML file in the file's order:
// a testsuites or a testsuite element at the root, testcase elements at
// any depth under it.
func readJUnit(data []byte) ([]junitCase, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var cases []junitCase
	rooted := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("it is not well-formed XML: %v", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if !rooted {
			if start.Name.Local != "testsuites" && start.Name.Local != "testsuite" {
				return nil, fmt.Errorf("its root element is %s, not testsuites or testsuite as JUnit XML has it", start.Name.Local)
			}
			rooted = true
			continue
		}
		if start.Name.Local != "testcase" {
			continue
		}
		var tc junitTestcase
		if err := dec.DecodeElement(&tc, &start); err != nil {
			return nil, fmt.Errorf("a testcase is not well-formed XML: %v", err)
		}
		c := junitCase{name: tc.Name, outcome: "passed"}
		switch {
		case len(tc.Failures) > 0:
			c.outcome, c.message = "failed", tc.Failures[0].line()
		case len(tc.Errors) > 0:
			c.outcome, c.message = "error", tc.Errors[0].line()
		case len(tc.Skipped) > 0:
			c.outcome, c.message = "skipped", tc.Skipped[0].line()
		}
		cases = append(cases, c)
	}
	if !rooted {
		return nil, errors.New("it holds no element; JUnit XML has testsuites or testsuite at its root")
	}
	return cases, nil
}

// mapTestCase names the test of the specification a test case runs: the
// test case's name, cut at the first / (a subtest) and with a trailing ()
// taken off, is the test's name, or Test or test followed by the name in
// Pascal case; or the name begins with the test's name and a colon. When
// none or more than one test matches, it says why instead (ADR-230).
func mapTestCase(name string, tests []string) (string, string) {
	base, _, _ := strings.Cut(name, "/")
	base = strings.TrimSuffix(base, "()")
	var found []string
	for _, t := range tests {
		if base == t || base == gentests.TestName("Test", t) || base == gentests.TestName("test", t) || strings.HasPrefix(name, t+":") {
			found = append(found, t)
		}
	}
	switch len(found) {
	case 1:
		return found[0], ""
	case 0:
		return "", "no test of the specification is named so, nor Test or test and its name in Pascal case, nor does the name begin with a test's name and a colon"
	}
	return "", "it maps to " + strings.Join(found, " and ") + ", more than one test"
}

// testAbout is the #/ pointer of the element a test is about: the path of
// its operation, or the command, page or entity it names; "" for none.
func testAbout(root, t *yaml.Node) string {
	if v := source.Str(source.Child(t, "operation")); v != "" {
		for _, p := range source.Pairs(source.Child(root, "paths")) {
			for _, op := range source.Pairs(p.Value) {
				if source.Str(source.Child(op.Value, "operationId")) == v {
					return "#" + source.Pointer("paths", p.Key.Value)
				}
			}
		}
		return ""
	}
	for _, k := range []string{"command", "page", "entity"} {
		if v := source.Str(source.Child(t, k)); v != "" {
			return "#" + source.Pointer(k+"s", v)
		}
	}
	return ""
}

// logTestRun records one run of the tests from a JUnit XML file (ADR-230).
func logTestRun(args []string, stdout, stderr io.Writer, kept **logResult) int {
	values := map[string]string{}
	lists := map[string][]string{}
	w := disk.Real
	var rest []string
	single := map[string]bool{}
	for _, o := range testRunOptions {
		single[o] = true
	}
	list := map[string]bool{}
	for _, o := range testRunLists {
		list[o] = true
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
		if !single[name] && !list[name] {
			fmt.Fprintf(stderr, "specarch log test-run has no option %s; its options are --%s, --%s and --dry-run, each but --dry-run with a value\n\n%s", a, strings.Join(testRunOptions, ", --"), strings.Join(testRunLists, ", --"), usage)
			return 2
		}
		if !inline {
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "specarch log test-run: %s needs a value\n\n%s", a, usage)
				return 2
			}
			value = args[i+1]
			i++
		}
		if single[name] {
			values[name] = value
		} else {
			lists[name] = append(lists[name], value)
		}
	}
	if len(rest) != 1 {
		fmt.Fprintf(stderr, "specarch log test-run needs one folder, the specification's\n\n%s", usage)
		return 2
	}
	folder := rest[0]
	specs, invalid, status := readSpecs([]string{folder}, "log", stdout, stderr)
	if status != 0 {
		return status
	}
	if len(specs) != 1 {
		fmt.Fprintf(stderr, "specarch log test-run: %s holds %d specifications; name the folder of the one the tests ran against\n", folder, len(specs))
		return 2
	}
	l := specs[0]
	root := l.spec.Root
	if root == nil {
		fmt.Fprintf(stderr, "specarch log test-run: nothing of %s could be read, so no test or environment can be checked; fix the errors above first\n", l.spec.RootFile)
		return 1
	}
	result := &logResult{Missing: []logMissing{}}
	if kept != nil {
		*kept = result
	}
	stakeholders := sortedPairKeys(source.Child(root, "stakeholders"))
	environments := sortedPairKeys(source.Child(root, "environments"))

	miss := func(option, ask string, choices []string) {
		if choices == nil {
			choices = []string{}
		}
		result.Missing = append(result.Missing, logMissing{Option: option, Ask: ask, Choices: choices})
	}
	if values["results"] == "" {
		miss("results", "where the JUnit XML file of the run is: put it under records/ beside the specification's folder, such as records/evidence/, and give its path", nil)
	}
	if values["environment"] == "" {
		miss("environment", "which environment of the specification the tests ran in", environments)
	}
	if values["at"] == "" {
		miss("at", "when the run started: the date, the time and the offset of the clock it was read on; written as 2026-11-03T14:05:00+07:00", nil)
	}
	if values["by"] == "" {
		miss("by", "who ran the tests, by role", stakeholders)
	}
	if len(lists["open-defect"]) > 0 && values["severity"] == "" {
		miss("severity", "how badly the failures to open defects for stop the system from meeting its requirements", defectSeverities)
	}
	if len(lists["open-defect"]) > 0 && values["phase"] == "" {
		miss("phase", "where the failures were found: in development, at the acceptance test on a build not yet released, or in production", defectPhases)
	}
	if len(result.Missing) > 0 {
		for _, m := range result.Missing {
			line := fmt.Sprintf("missing --%s: ask %s", m.Option, m.Ask)
			if len(m.Choices) > 0 {
				line += "; one of " + strings.Join(m.Choices, ", ")
			}
			fmt.Fprintln(stdout, line)
		}
		fmt.Fprintf(stderr, "specarch log test-run: %s not given; ask what each line above asks, and run it again with every option; nothing written\n", plural(len(result.Missing), "option"))
		return 1
	}

	refuse := func(format string, args ...any) int {
		fmt.Fprintf(stderr, "specarch log test-run: "+format+"; nothing written\n", args...)
		return 1
	}
	if source.Child(source.Child(root, "environments"), values["environment"]) == nil {
		return refuse("--environment %s is not an environment of %s; use one of %s", values["environment"], l.spec.Dir, strings.Join(environments, ", "))
	}
	if source.Child(source.Child(root, "stakeholders"), values["by"]) == nil {
		return refuse("--by %s is not a stakeholder of %s; use one of %s", values["by"], l.spec.Dir, strings.Join(stakeholders, ", "))
	}
	at, err := time.Parse(time.RFC3339, values["at"])
	if err != nil {
		return refuse("--at %s is not an instant with its offset; write it as RFC 3339 does, such as 2026-11-03T14:05:00+07:00, with the date and the offset of the clock it was read on", values["at"])
	}
	if d := values["digest"]; d != "" && !digestPattern.MatchString(d) {
		return refuse("--digest %s is not sha256: and 64 hexadecimal digits, the specification's digest as the approval record keeps it; an image digest goes in --build", d)
	}
	records := parsedRecords(l.spec)
	if v := values["version"]; v != "" && !hasRelease(records, v) {
		return refuse("--version %s is not a release: there is no records/releases/%s.yaml; add it with status planned, or name a release that exists", v, v)
	}
	if values["severity"] != "" && !oneOfValues(defectSeverities, values["severity"]) {
		return refuse("--severity %s is none of %s", values["severity"], strings.Join(defectSeverities, ", "))
	}
	if values["phase"] != "" && !oneOfValues(defectPhases, values["phase"]) {
		return refuse("--phase %s is none of %s", values["phase"], strings.Join(defectPhases, ", "))
	}
	if values["phase"] == "acceptance" && len(lists["open-defect"]) > 0 && values["version"] == "" {
		return refuse("a defect found at acceptance names the version of the build it was found on; give --version")
	}

	// The results file, under records/.
	p := values["results"]
	full, err := filepath.Abs(p)
	if err != nil {
		return refuse("--results %s cannot be read: %s", p, spec.PlainIOError(err))
	}
	recordsDir, err := filepath.Abs(l.spec.RecordsDir)
	if err != nil {
		return refuse("--results %s cannot be placed under %s: %s", p, l.spec.RecordsDir, spec.PlainIOError(err))
	}
	rel, err := filepath.Rel(recordsDir, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return refuse("--results %s is not under %s/; put the file under records/ beside the specification's folder, such as records/evidence/, and name it from there, so the record can keep it as its evidence", p, l.spec.RecordsDir)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return refuse("--results %s cannot be read: %s", p, spec.PlainIOError(err))
	}
	cases, err := readJUnit(data)
	if err != nil {
		return refuse("--results %s is not JUnit XML: %v", p, err)
	}
	if len(cases) == 0 {
		return refuse("--results %s holds no testcase, so there is no result to record", p)
	}
	mediaType := mediaTypeOf(p)
	if mediaType == "" {
		mediaType = "application/xml"
	}
	sum := sha256.Sum256(data)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	// Each test case mapped to its test.
	testsNode := source.Child(root, "tests")
	tests := sortedPairKeys(testsNode)
	mapped := make([]string, len(cases))
	failedTests := map[string]bool{}
	for i, c := range cases {
		t, why := mapTestCase(c.name, tests)
		mapped[i] = t
		if t == "" {
			result.Unmapped = append(result.Unmapped, logUnmapped{Name: c.name, Why: why})
		}
		if t != "" && (c.outcome == "failed" || c.outcome == "error") {
			failedTests[t] = true
		}
	}

	// The defects to open, each checked before anything is written.
	id := nextRecordID(records.ids("test-run"), "TR")
	type opened struct {
		test, id, path string
		node           *yaml.Node
	}
	var defects []opened
	defectIDs := records.ids("defect")
	seen := map[string]bool{}
	for _, t := range lists["open-defect"] {
		if seen[t] {
			continue
		}
		seen[t] = true
		tn := source.Child(testsNode, t)
		if tn == nil {
			return refuse("--open-defect %s is not a test of the specification", t)
		}
		if !failedTests[t] {
			return refuse("--open-defect %s: no result of the run maps to %s and failed or errored, so there is no failure to open a defect for", t, t)
		}
		var violates []string
		for _, v := range source.Items(source.Child(tn, "verifies")) {
			violates = append(violates, v.Value)
		}
		if len(violates) == 0 {
			if about := testAbout(root, tn); about != "" {
				violates = []string{about}
			}
		}
		if len(violates) == 0 {
			return refuse("--open-defect %s: the test verifies no requirement and is about no operation, command, page or entity, so the defect would violate nothing; add verifies to the test, or write the defect by hand", t)
		}
		did := nextRecordID(defectIDs, "DEF")
		defectIDs = append(defectIDs, did)
		message := ""
		outcome := ""
		for i, c := range cases {
			if mapped[i] == t && (c.outcome == "failed" || c.outcome == "error") {
				message, outcome = c.message, c.outcome
				break
			}
		}
		observed := message
		if observed == "" {
			if outcome == "error" {
				observed = fmt.Sprintf("test %s could not run to its end in test run %s, and the results file says no more", t, id)
			} else {
				observed = fmt.Sprintf("test %s failed in test run %s, and the results file says no more", t, id)
			}
		}
		expected := oneLineText(source.Str(source.Child(tn, "then")))
		if expected == "" {
			expected = fmt.Sprintf("test %s passes", t)
		}
		m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		add := func(key string, n *yaml.Node) { m.Content = append(m.Content, scalarNode(key), n) }
		add("specarchRecord", quotedNode("1.0.0"))
		add("kind", scalarNode("defect"))
		add("id", scalarNode(did))
		verb := "failed"
		if outcome == "error" {
			verb = "could not run to its end"
		}
		add("title", scalarNode(fmt.Sprintf("Test %s %s in test run %s", t, verb, id)))
		add("reportedBy", scalarNode(values["by"]))
		add("reported", quotedNode(at.Format("2006-01-02")))
		add("phase", scalarNode(values["phase"]))
		add("environment", scalarNode(values["environment"]))
		if values["phase"] == "acceptance" {
			against := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{scalarNode("version"), quotedNode(values["version"])}}
			if values["digest"] != "" {
				against.Content = append(against.Content, scalarNode("digest"), scalarNode(values["digest"]))
			}
			if values["build"] != "" {
				against.Content = append(against.Content, scalarNode("build"), scalarNode(values["build"]))
			}
			add("against", against)
		}
		add("observed", scalarNode(observed))
		add("expected", scalarNode(expected))
		add("severity", scalarNode(values["severity"]))
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		for _, v := range violates {
			n := scalarNode(v)
			if strings.HasPrefix(v, "#/") {
				n.Style = yaml.DoubleQuotedStyle
			}
			seq.Content = append(seq.Content, n)
		}
		add("violates", seq)
		add("test", scalarNode(t))
		add("status", scalarNode("reported"))
		path := filepath.Join(l.spec.RecordsDir, "defects", did+".yaml")
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(stderr, "specarch log test-run: %s exists although no defect record carries %s; read it, and name it after the record it holds or remove it\n", path, did)
			return 2
		}
		defects = append(defects, opened{test: t, id: did, path: path, node: m})
	}
	defectOf := map[string]string{}
	for _, d := range defects {
		defectOf[d.test] = d.id
	}

	// The record.
	recordPath := filepath.Join(l.spec.RecordsDir, "test-runs", id+".yaml")
	if _, err := os.Stat(recordPath); err == nil {
		fmt.Fprintf(stderr, "specarch log test-run: %s exists although no test-run record carries %s; read it, and name it after the record it holds or remove it\n", recordPath, id)
		return 2
	}
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	add := func(key string, n *yaml.Node) { m.Content = append(m.Content, scalarNode(key), n) }
	add("specarchRecord", quotedNode("1.0.0"))
	add("kind", scalarNode("test-run"))
	add("id", scalarNode(id))
	add("environment", scalarNode(values["environment"]))
	if values["version"] != "" {
		add("version", quotedNode(values["version"]))
	}
	if values["build"] != "" {
		add("build", scalarNode(values["build"]))
	}
	if values["digest"] != "" {
		add("digest", scalarNode(values["digest"]))
	}
	add("at", quotedNode(values["at"]))
	add("by", scalarNode(values["by"]))
	add("evidence", &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map", Style: yaml.FlowStyle, Content: []*yaml.Node{
		scalarNode("path"), scalarNode(filepath.ToSlash(rel)), scalarNode("mediaType"), scalarNode(mediaType), scalarNode("sha256"), scalarNode(digest),
	}}}})
	counts := map[string]int{}
	results := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for i, c := range cases {
		counts[c.outcome]++
		r := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		r.Content = append(r.Content, scalarNode("name"), scalarNode(c.name))
		if mapped[i] != "" {
			r.Content = append(r.Content, scalarNode("test"), scalarNode(mapped[i]))
		}
		r.Content = append(r.Content, scalarNode("outcome"), scalarNode(c.outcome))
		if c.message != "" {
			r.Content = append(r.Content, scalarNode("message"), scalarNode(c.message))
		}
		if d := defectOf[mapped[i]]; d != "" && (c.outcome == "failed" || c.outcome == "error") {
			r.Content = append(r.Content, scalarNode("defect"), scalarNode(d))
		}
		results.Content = append(results.Content, r)
	}
	add("results", results)

	encode := func(header string, n *yaml.Node) ([]byte, error) {
		var b bytes.Buffer
		b.WriteString(header)
		enc := yaml.NewEncoder(&b)
		enc.SetIndent(2)
		if err := enc.Encode(n); err != nil {
			return nil, err
		}
		if err := enc.Close(); err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	}
	type file struct {
		path string
		data []byte
	}
	var files []file
	data, err = encode(fmt.Sprintf("# Written by specarch log test-run from records/%s. Edit it as any record.\n", filepath.ToSlash(rel)), m)
	if err != nil {
		fmt.Fprintf(stderr, "specarch log test-run: cannot write the record: %v\n", err)
		return 2
	}
	files = append(files, file{recordPath, data})
	for _, d := range defects {
		data, err := encode(fmt.Sprintf("# Written by specarch log test-run from the failure of %s in %s. Edit it as any record.\n", d.test, id), d.node)
		if err != nil {
			fmt.Fprintf(stderr, "specarch log test-run: cannot write the defect: %v\n", err)
			return 2
		}
		files = append(files, file{d.path, data})
	}
	for _, f := range files {
		if !w.Dry() {
			if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
				fmt.Fprintf(stderr, "specarch log test-run: cannot make %s: %v\n", filepath.Dir(f.path), err)
				return 2
			}
		}
		if err := w.WriteFile(f.path, f.data, 0o644); err != nil {
			fmt.Fprintf(stderr, "specarch log test-run: cannot write %s: %v\n", f.path, err)
			return 2
		}
	}

	result.Record, result.ID = filepath.ToSlash(recordPath), id
	for _, d := range defects {
		result.Defects = append(result.Defects, filepath.ToSlash(d.path))
	}
	for _, u := range result.Unmapped {
		fmt.Fprintf(stdout, "unmapped: %s: %s\n", u.Name, u.Why)
	}
	summary := fmt.Sprintf("%s: %d passed, %d failed, %d errored, %d skipped, %d unmapped", plural(len(cases), "result"), counts["passed"], counts["failed"], counts["error"], counts["skipped"], len(result.Unmapped))
	if w.Dry() {
		for _, f := range files {
			fmt.Fprint(stdout, string(f.data))
		}
		fmt.Fprintf(stderr, "specarch log test-run: %s would be written to %s, %s; nothing written (--dry-run)\n", id, result.Record, summary)
	} else {
		fmt.Fprintf(stdout, "%s\n", result.Record)
		for _, d := range result.Defects {
			fmt.Fprintf(stdout, "%s\n", d)
		}
		fmt.Fprintf(stderr, "specarch log test-run: %s written to %s, %s\n", id, result.Record, summary)
	}
	if invalid {
		fmt.Fprintln(stderr, "specarch log test-run: the input has errors; they are printed above")
		return 1
	}
	return 0
}

func quotedNode(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s, Style: yaml.DoubleQuotedStyle}
}

// hasRelease reports whether a release record of that version is beside
// the specification.
func hasRelease(records recordIndex, version string) bool {
	for _, r := range records["release"] {
		if source.Str(source.Child(r, "version")) == version {
			return true
		}
	}
	return false
}
