package gentests

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// The Harness methods with the arguments a stub needs to satisfy them.
const stub = `package specarchtests

import "testing"

type stubHarness struct{}

func (stubHarness) SignIn(t *testing.T, role string)              {}
func (stubHarness) Insert(t *testing.T, mapping string, r Record) {}
func (stubHarness) Records(t *testing.T, mapping string) []Record { return nil }
func (stubHarness) Request(t *testing.T, mapping, method, path string, in Record) Response {
	return Response{}
}
func (stubHarness) Run(t *testing.T, mapping, command string, arguments map[string][]Value, options Record) Result {
	return Result{}
}
func (stubHarness) Open(t *testing.T, mapping, page, route string, in Record) Response {
	return Response{}
}
func (stubHarness) Emitted(t *testing.T) []string                  { return nil }
func (stubHarness) Compute(t *testing.T, mapping string, in Record) Value { return Value{} }

func newHarness(t *testing.T) Harness { return stubHarness{} }
`

var bodyCall = regexp.MustCompile(`\bbody([A-Za-z0-9]+)\(t, h\)`)

// TestLibraryLendingCompiles generates the tests of the library lending
// example and compiles them beside a project's side as small as it can be:
// a harness that does nothing and an empty body for every test the design
// gives no call for. A file that does not compile there would not compile
// in any project.
func TestLibraryLendingCompiles(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go on PATH to compile the generated file with")
	}
	r, out := libraryLending(t, nil)
	resp := GenerateGo(r)
	if len(resp.Diagnostics) > 0 || len(resp.Files) != 1 {
		t.Fatalf("want one file and no diagnostics, got %d files and %v", len(resp.Files), resp.Diagnostics)
	}
	content := resp.Files[0].Content
	for _, want := range []string{"func TestLendACopy(t *testing.T) {", "func TestAlgorithmLateFee(t *testing.T) {", `Value{Kind: "decimal", Text: "3.50"}`,
		"type MappingHarness interface {", "func TestMappingReturnLoanFeeLedgerPostCharge(t *testing.T) {", `h.Send(t, "returnLoan", "feeLedger#/operations/postCharge", Record{`} {
		if !strings.Contains(content, want) {
			t.Errorf("the generated file has no %q", want)
		}
	}

	var b strings.Builder
	b.WriteString(stub)
	for _, n := range bodies(content, bodyCall) {
		fmt.Fprintf(&b, "\nfunc body%s(t *testing.T, h Harness) {}\n", n)
	}
	files := map[string]string{
		"go.mod":          "module example.com/specarchtests\n\ngo 1.26\n",
		GoFile:            content,
		"harness_test.go": b.String(),
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(out, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = out
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the generated tests do not compile: %v\n%s", err, b)
	}
}

// libraryLending is the request specarch makes for the library lending
// example's tests, with an output folder of the test's own; change edits
// each implementation file's content first, standing in for a project on
// another stack.
func libraryLending(t *testing.T, change func(content map[string]any)) (*Request, string) {
	t.Helper()
	return requestFor(t, "../../examples/library-lending/spec", nil, change)
}

// requestFor is the request specarch makes for the tests of the
// specification in folder, as libraryLending; changeSpec edits the
// specification first, and may be nil.
func requestFor(t *testing.T, folder string, changeSpec, change func(content map[string]any)) (*Request, string) {
	t.Helper()
	s := spec.Load(folder)
	if len(s.Problems) > 0 {
		t.Fatalf("%s does not load: %v", folder, s.Problems)
	}
	if changeSpec != nil {
		m, _ := s.Value.(map[string]any)
		changeSpec(m)
	}
	out := t.TempDir()
	req := map[string]any{
		"specarch": "1.0.0",
		"target":        "tests",
		"root":          filepath.ToSlash(s.RootFile),
		"specification": s.Value,
		"output":        filepath.ToSlash(out),
	}
	var impls []map[string]any
	for _, impl := range s.Implementations {
		content, _ := source.ValueOf(source.Parse(impl.Data).Root).(map[string]any)
		if change != nil {
			change(content)
		}
		impls = append(impls, map[string]any{"file": impl.Path, "content": content})
	}
	req["implementations"] = impls
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return r, out
}

// bodies are the names of the bodies a generated file calls, sorted.
func bodies(content string, call *regexp.Regexp) []string {
	seen := map[string]bool{}
	for _, m := range call.FindAllStringSubmatch(content, -1) {
		seen[m[1]] = true
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// TestWorkedTestsNameWhatGeneratorsWrite checks that every function and
// display name the Go, Swift and Dart generators write for an algorithm's
// or a call mapping's worked examples is one WorkedTests gives, with the
// element it runs, so specarch log test-run maps each back (ADR-230).
func TestWorkedTestsNameWhatGeneratorsWrite(t *testing.T) {
	r, _ := libraryLending(t, asStack)
	worked := WorkedTests(r.Specification)
	functions, displays := map[string]string{}, map[string]string{}
	for _, w := range worked {
		for _, f := range w.Functions {
			functions[f] = w.Element
		}
		displays[w.Display] = w.Element
	}
	if functions["TestAlgorithmLateFee"] != "#/algorithms/lateFee" || functions["TestAlgorithmMayLend"] != "#/algorithms/mayLend" {
		t.Errorf("the algorithms' tests map to %q and %q", functions["TestAlgorithmLateFee"], functions["TestAlgorithmMayLend"])
	}
	if el := functions["TestMappingReturnLoanFeeLedgerPostCharge"]; !strings.HasPrefix(el, "#/paths/") || strings.Count(el, "/") != 2 {
		t.Errorf("the mapping's test maps to %q, not the path of its operation", el)
	}
	check := func(stack, content string, names *regexp.Regexp, known map[string]string) {
		n := 0
		for _, m := range names.FindAllStringSubmatch(content, -1) {
			n++
			if known[m[1]] == "" {
				t.Errorf("%s writes %q, which WorkedTests does not name", stack, m[1])
			}
		}
		if n == 0 {
			t.Errorf("%s writes no test for a worked example; the check reads nothing", stack)
		}
	}
	goFile := generated(t, GenerateGo(r), GoFile)[GoFile]
	check("Go", goFile, regexp.MustCompile(`func (Test(?:Algorithm|Mapping)[A-Za-z0-9]*)\(`), functions)
	swiftFile := generated(t, GenerateSwift(r), SwiftFile)[SwiftFile]
	check("Swift", swiftFile, regexp.MustCompile(`func (test(?:Algorithm|Mapping)[A-Za-z0-9]*)\(`), functions)
	check("Swift", swiftFile, regexp.MustCompile(`@Test\("((?:algorithm|mapping of) [^"]*?): [^"]*"`), displays)
	dartFile := generated(t, GenerateDart(r), DartLibraryFile, DartTestFile)[DartTestFile]
	check("Dart", strings.ReplaceAll(dartFile, `\'`, "\x00"), regexp.MustCompile(`group\('((?:algorithm|mapping of) [^']*)'`), func() map[string]string {
		escaped := map[string]string{}
		for k, v := range displays {
			escaped[strings.ReplaceAll(k, "'", "\x00")] = v
		}
		return escaped
	}())
}

// TestWorkedTestsReadsAWrongSpecification checks WorkedTests reads a
// specification whose calls entries are neither a reference nor a map, as
// specarch log test-run gives it whatever the validator could read.
func TestWorkedTestsReadsAWrongSpecification(t *testing.T) {
	spec := map[string]any{
		"paths": map[string]any{"/loans": map[string]any{"post": map[string]any{
			"operationId": "makeLoan",
			"calls":       []any{json.Number("42"), map[string]any{"call": json.Number("7")}, map[string]any{"mapping": map[string]any{"examples": []any{map[string]any{"name": "x"}}}}},
		}}},
		"algorithms": map[string]any{"cap": "not a map"},
	}
	worked := WorkedTests(spec)
	for _, w := range worked {
		if w.Element != "#/paths/~1loans" {
			t.Errorf("a worked test of the wrong specification maps to %q", w.Element)
		}
	}
}
