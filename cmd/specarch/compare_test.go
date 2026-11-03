package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// compareCase is the conformance case of two implementation files, and
// the command line it runs compare with.
const compareCase = "../../spec/tests/compare-two-implementations"

func compareArgs(out string) []string {
	args := []string{"compare",
		"--results", "lending.go=records/evidence/go-junit.xml", "--test-run", "lending.swift=TR-1",
		"--scan", "lending.go=records/evidence/go-grype.json", "--scan", "lending.swift=records/evidence/swift-osv.json",
		"--measurements", "lending.go=records/evidence/go-measurements.json", "--measurements", "lending.swift=records/evidence/swift-measurements.json"}
	if out != "" {
		args = append(args, "--out", out)
	}
	return append(args, "project", "project/implementation/go/lending.go.specarch-implementation.yaml", "project/implementation/swift/lending.swift.specarch-implementation.yaml")
}

// TestCompareDecisionCitesIt checks that a decision citing the comparison
// through the sources entry compare prints validates without an error
// (SA-127, ADR-232).
func TestCompareDecisionCitesIt(t *testing.T) {
	noPlugins(t)
	dir := copyFolder(t, compareCase)
	status, out := runIn(t, dir, compareArgs("records/comparisons/lending"))
	if status != 0 {
		t.Fatalf("compare exited %d:\n%s", status, out)
	}
	var entry string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "  comparison-lending: {kind: \"document\"") {
			entry = l
		}
	}
	if entry == "" {
		t.Fatalf("compare printed no sources entry:\n%s", out)
	}
	root := filepath.Join(dir, "project", "specarch.yaml")
	data, err := os.ReadFile(root)
	if err != nil {
		t.Fatal(err)
	}
	cited := string(data) + "sources:\n" + entry + "\n" + `decisions:
  ADR-001:
    title: Build the lending service in Go
    status: accepted
    date: "2026-11-04"
    context: Two implementations of the specification were built and compared.
    decision: Build it in Go.
    consequences: The Swift build is kept as a reference only.
    cites:
      - source: comparison-lending
        clause: Latency and throughput
        says: The Go build saves a loan within 140 ms for nine calls in ten, the Swift build within 185 ms.
`
	if err := os.WriteFile(root, []byte(cited), 0o644); err != nil {
		t.Fatal(err)
	}
	status, out = runIn(t, dir, []string{"validate", "project"})
	if status != 0 || !strings.Contains(out, "0 errors") {
		t.Fatalf("validate of a decision citing the comparison exited %d:\n%s", status, out)
	}
}

// TestCompareJSON checks that --json holds the comparison under compare,
// the same object comparison.json holds, and that a run is the same run
// after run, so a decision's edition stays the digest of the file.
func TestCompareJSON(t *testing.T) {
	noPlugins(t)
	want, err := os.ReadFile(filepath.Join(compareCase, "expected", "records", "comparisons", "lending", "comparison.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := copyFolder(t, compareCase)
	t.Chdir(dir)
	var stdout, stderr bytes.Buffer
	if status := run(append([]string{"--json"}, compareArgs("records/comparisons/lending")...), &stdout, &stderr); status != 0 {
		t.Fatalf("compare --json exited %d:\n%s%s", status, stdout.String(), stderr.String())
	}
	var o struct {
		Compare json.RawMessage `json:"compare"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &o); err != nil || len(o.Compare) == 0 {
		t.Fatalf("--json printed no compare object (%v):\n%s", err, stdout.String())
	}
	written, err := os.ReadFile(filepath.Join(dir, "records", "comparisons", "lending", "comparison.json"))
	if err != nil {
		t.Fatal(err)
	}
	var a, b any
	if err := json.Unmarshal(o.Compare, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(written, &b); err != nil {
		t.Fatal(err)
	}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if !bytes.Equal(ja, jb) {
		t.Errorf("--json's compare differs from comparison.json:\n%s\n%s", ja, jb)
	}
	if !bytes.Equal(written, want) {
		t.Errorf("a second run wrote another comparison.json than the case expects")
	}
}

// TestCompareRefusesInputs checks the refusals of inputs that are given
// but wrong: each exits 1 and writes nothing.
func TestCompareRefusesInputs(t *testing.T) {
	noPlugins(t)
	impls := []string{"project", "project/implementation/go/lending.go.specarch-implementation.yaml", "project/implementation/swift/lending.swift.specarch-implementation.yaml"}
	cases := []struct {
		name  string
		file  string // written under records/evidence/ first
		data  string
		args  []string
		reads string
	}{
		{name: "both results and a test run", args: []string{"--results", "lending.swift=records/evidence/go-junit.xml", "--test-run", "lending.swift=TR-1"}, reads: "both --test-run and --results"},
		{name: "a test run the records do not hold", args: []string{"--test-run", "lending.swift=TR-9"}, reads: "the records hold no test-run TR-9; record the run with specarch log test-run, or name one of TR-1"},
		{name: "a name given twice", args: []string{"--scan", "lending.go=records/evidence/go-grype.json", "--scan", "lending.go=records/evidence/go-grype.json"}, reads: "--scan names lending.go twice"},
		{name: "a value without a name", args: []string{"--scan", "records/evidence/go-grype.json"}, reads: "is not <name>=<value>"},
		{name: "a scan of no format compare reads", file: "trivy.json", data: `{"Results": []}`, args: []string{"--scan", "lending.go=records/evidence/trivy.json"}, reads: "neither grype's JSON, which holds matches, nor osv-scanner's, which holds results"},
		{name: "a measurement of no requirement", file: "m.json", data: `{"requirements": {"CASE-9": {"latency": {"value": 1, "unit": "ms"}}}}`, args: []string{"--measurements", "lending.go=records/evidence/m.json"}, reads: "CASE-9 is not a requirement of the specification"},
		{name: "a measurement of a key compare does not read", file: "m.json", data: `{"buildMinutes": 2}`, args: []string{"--measurements", "lending.go=records/evidence/m.json"}, reads: "it has the key buildMinutes, which compare does not read"},
		{name: "a build time that is no whole number", file: "m.json", data: `{"buildSeconds": 2.5}`, args: []string{"--measurements", "lending.go=records/evidence/m.json"}, reads: "buildSeconds is 2.5, not a whole number from 0"},
		{name: "a value written as words", file: "m.json", data: `{"requirements": {"CASE-3": {"latency": {"value": "140", "unit": "ms"}}}}`, args: []string{"--measurements", "lending.go=records/evidence/m.json"}, reads: `CASE-3 latency has the value "140", not a number`},
		{name: "a results file that is not JUnit XML", file: "notes.xml", data: `<notes/>`, args: []string{"--results", "lending.go=records/evidence/notes.xml"}, reads: "is not JUnit XML"},
		{name: "an implementation file not the specification's", args: []string{"--", "project", "project/specarch.yaml"}, reads: "is not an implementation file of project"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := copyFolder(t, compareCase)
			if c.file != "" {
				if err := os.WriteFile(filepath.Join(dir, "records", "evidence", c.file), []byte(c.data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			args := append([]string{"compare", "--out", "records/comparisons/lending"}, c.args...)
			if c.args[0] != "--" {
				args = append(args, impls...)
			}
			status, out := runIn(t, dir, args)
			if status != 1 || !strings.Contains(out, c.reads) || !strings.Contains(out, "nothing written") {
				t.Errorf("compare exited %d, want 1 and a refusal reading %q:\n%s", status, c.reads, out)
			}
			if _, err := os.Stat(filepath.Join(dir, "records", "comparisons")); err == nil {
				t.Errorf("a refused compare wrote records/comparisons")
			}
		})
	}
}

func TestCVSSSeverity(t *testing.T) {
	for score, want := range map[string]string{"9.0": "critical", "10": "critical", "8.9": "high", "7.0": "high", "6.9": "medium", "4.0": "medium", "3.9": "low", "0.1": "low", "0": "negligible", "0.0": "negligible", "-1": "unknown", "": "unknown", "CVSS:3.1/AV:N": "unknown"} {
		if got := cvssSeverity(score); got != want {
			t.Errorf("cvssSeverity(%q) = %s, want %s", score, got, want)
		}
	}
}
