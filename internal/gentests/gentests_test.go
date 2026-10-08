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
	for _, want := range []string{"func TestLendACopy(t *testing.T) {", "func TestAlgorithmLateFee(t *testing.T) {", `Value{Kind: "decimal", Text: "3.50"}`} {
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
	s := spec.Load("../../examples/library-lending/spec")
	if len(s.Problems) > 0 {
		t.Fatalf("the example does not load: %v", s.Problems)
	}
	out := t.TempDir()
	req := map[string]any{
		"specarch":      "0.1",
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
