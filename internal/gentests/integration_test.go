package gentests

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/xref"
)

// readingRoom is the request specarch makes for the tests of the reading
// room in examples/estate, given the catalogue service as the estate gives
// it: change edits the catalogue's specification first, and calls the
// reading room's, so a test can stand for a contract change or a call
// into a second module.
func readingRoom(t *testing.T, catalogue, room func(s map[string]any)) (*Request, string) {
	t.Helper()
	s := spec.Load("../../examples/estate/reading-room/spec")
	c := spec.Load("../../examples/estate/catalogue")
	if len(s.Problems) > 0 || len(c.Problems) > 0 {
		t.Fatalf("the example does not load: %v %v", s.Problems, c.Problems)
	}
	out := t.TempDir()
	content := deepCopyJSON(t, c.Value)
	if catalogue != nil {
		catalogue(content)
	}
	value := deepCopyJSON(t, s.Value)
	if room != nil {
		room(value)
	}
	deps := map[string]xref.Dependency{"catalogue": {Specification: "catalogue-service", Version: "0.1.0", Content: content}}
	if _, ok := mapAt(value, "dependencies", "archive").(map[string]any); ok {
		archive := deepCopyJSON(t, c.Value)
		archive["info"].(map[string]any)["name"] = "archive-service"
		deps["archive"] = xref.Dependency{Specification: "archive-service", Version: "0.1.0", Content: archive}
	}
	var impls []map[string]any
	for _, impl := range s.Implementations {
		if !strings.Contains(impl.Path, "/go/") {
			continue
		}
		impls = append(impls, map[string]any{"file": impl.Path, "content": source.ValueOf(source.Parse(impl.Data).Root)})
	}
	data, err := json.Marshal(map[string]any{"specarch": "1.0.0", "target": "tests", "root": filepath.ToSlash(s.RootFile),
		"specification": value, "output": filepath.ToSlash(out), "implementations": impls, "dependencies": deps})
	if err != nil {
		t.Fatal(err)
	}
	r, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return r, out
}

func deepCopyJSON(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func byPath(resp Response) map[string]string {
	files := map[string]string{}
	for _, f := range resp.Files {
		files[f.Path] = f.Content
	}
	return files
}

const integrationStub = `package specarchtests

import "testing"

type stubIntegration struct{}

func (stubIntegration) Call(t *testing.T, c ModuleCall, r CallRequest) Answer { return Answer{} }
func (stubIntegration) Arrange(t *testing.T, c ModuleCall, status int)       {}
func (stubIntegration) Break(t *testing.T, c ModuleCall, how string)         {}

func newIntegrationHarness(t *testing.T) IntegrationHarness { return stubIntegration{} }
`

// TestReadingRoomIntegration generates the reading room's tests across the
// estate: one test per call into the catalogue, the picker's listBooks and
// requestBook's getBook, with the paths the catalogue's contract implies,
// in a file of the catalogue's own; and compiles them beside a harness
// that does nothing.
func TestReadingRoomIntegration(t *testing.T) {
	r, out := readingRoom(t, nil, nil)
	resp := GenerateGo(r)
	files := generated(t, resp, GoFile, "specarch_integration_catalogue_service_test.go")
	it := files["specarch_integration_catalogue_service_test.go"]
	mustContain(t, it,
		"// The integration tests of the calls reading-room 0.1.0 makes into catalogue-service 0.1.0, through",
		"func TestCallCatalogueServiceGetBook(t *testing.T) {", "func TestCallCatalogueServiceListBooks(t *testing.T) {",
		`Module: "catalogue-service", Version: "0.1.0", Operation: "getBook", Method: "GET", Path: "/books/{barcode}", Timeout: "PT2S", Places: []string{"#/paths/~1requests/post/calls/0"}}`,
		`Places: []string{"#/pages/request-book/pickers/barcode/source"}}`,
		`t.Run("golden", func(t *testing.T) {`, `expectAnswerFields(t, got, "author", "barcode", "title")`,
		`t.Run("denied without books.read", func(t *testing.T) {`, `got := h.Call(t, c, CallRequest{Input: in})`,
		`t.Run("answers 404", func(t *testing.T) {`, "h.Arrange(t, c, 404)",
		`h.Break(t, c, "fails")`, `h.Break(t, c, "times out")`, "expectGaveUp(t, got)")
	if strings.Count(it, "\nfunc Test") != 2 {
		t.Errorf("want two tests, one per call, got:\n%s", it)
	}
	mustContain(t, files[GoFile], "type IntegrationHarness interface {")

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go on PATH to compile the generated files with")
	}
	var b strings.Builder
	b.WriteString(stub)
	for _, n := range bodies(files[GoFile], bodyCall) {
		fmt.Fprintf(&b, "\nfunc body%s(t *testing.T, h Harness) {}\n", n)
	}
	write(t, out, map[string]string{
		"go.mod":                               "module example.com/specarchtests\n\ngo 1.26\n",
		GoFile:                                 files[GoFile],
		GoIntegrationFile("catalogue-service"): it,
		"harness_test.go":                      b.String(),
		"integration_harness_test.go":          integrationStub,
	})
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = out
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the generated tests do not compile: %v\n%s", err, b)
	}
}

// TestContractChangeChangesOnlyItsFile calls a second module beside the
// catalogue and changes the catalogue's contract: the file of the calls
// into the catalogue changes, and the design tests' file and the file of
// the other module's calls stay as they were.
func TestContractChangeChangesOnlyItsFile(t *testing.T) {
	twoModules := func(s map[string]any) {
		deps := s["dependencies"].(map[string]any)
		deps["archive"] = map[string]any{"description": "The archive.", "timeout": "PT5S", "specification": "archive-service", "version": "0.1.0"}
		op := mapAt(s, "paths", "/requests", "post").(map[string]any)
		op["calls"] = append(op["calls"].([]any), "archive#/paths/~1books/get")
	}
	before, _ := readingRoom(t, nil, twoModules)
	after, _ := readingRoom(t, func(c map[string]any) {
		op := mapAt(c, "paths", "/books/{barcode}", "get").(map[string]any)
		op["responses"].(map[string]any)["410"] = map[string]any{"description": "The book was withdrawn."}
	}, twoModules)
	a, b := byPath(GenerateGo(before)), byPath(GenerateGo(after))
	if len(a) != 3 || len(b) != 3 {
		t.Fatalf("want the design tests' file and one file per module, got %d and %d files", len(a), len(b))
	}
	catalogue, archive := GoIntegrationFile("catalogue-service"), GoIntegrationFile("archive-service")
	if a[catalogue] == b[catalogue] || !strings.Contains(b[catalogue], `t.Run("answers 410"`) {
		t.Errorf("the catalogue's file does not change with its contract:\n%s", b[catalogue])
	}
	if a[archive] != b[archive] {
		t.Errorf("the archive's file changed with the catalogue's contract")
	}
	if a[GoFile] != b[GoFile] {
		t.Errorf("the design tests' file changed with the catalogue's contract")
	}
}

// TestRequiredFieldLeftOut gives the called operation a required body
// field and a 422: the call has a path that leaves the field out and
// checks the 422; a path parameter gets none.
func TestRequiredFieldLeftOut(t *testing.T) {
	r, _ := readingRoom(t, func(c map[string]any) {
		op := mapAt(c, "paths", "/books/{barcode}", "get").(map[string]any)
		op["parameters"] = []any{map[string]any{"name": "branch", "in": "query", "required": true, "schema": map[string]any{"type": "string"}}}
		op["responses"].(map[string]any)["422"] = map[string]any{"description": "A value is missing."}
	}, nil)
	it := byPath(GenerateGo(r))[GoIntegrationFile("catalogue-service")]
	mustContain(t, it, `t.Run("missing branch", func(t *testing.T) {`, `CallRequest{Holding: []string{"books.read"}, Input: in, Omit: "branch"}`, "expectAnswer(t, got, 422)")
	if strings.Contains(it, `"missing barcode"`) || strings.Contains(it, `"answers 422"`) {
		t.Errorf("a path parameter is left out, or the 422 of the missing field is arranged again:\n%s", it)
	}
}

// TestCallIntoModuleNotGiven runs without the estate: the call into the
// catalogue is named in the design tests' file, and no file is written
// for it.
func TestCallIntoModuleNotGiven(t *testing.T) {
	r, _ := readingRoom(t, nil, nil)
	r.Dependencies = nil
	files := byPath(GenerateGo(r))
	if len(files) != 1 {
		t.Fatalf("want the design tests' file alone, got %d files", len(files))
	}
	mustContain(t, files[GoFile],
		"// The call catalogue#/paths/~1books~1{barcode}/get at #/paths/~1requests/post/calls/0 is not written: the run does not give catalogue-service; give its estate with --estate.",
		"// The call catalogue#/paths/~1books/get at #/pages/request-book/pickers/barcode/source is not written:")
}

// TestIntegrationSameOnEveryStack checks that the Swift and Dart plug-ins
// write the same tests and paths as the Go one, and compiles or analyzes
// them where the toolchain is on PATH.
func TestIntegrationSameOnEveryStack(t *testing.T) {
	r, out := readingRoom(t, nil, nil)
	swift := byPath(GenerateSwift(r))
	dart := byPath(GenerateDart(r))
	sw, dt := swift[SwiftIntegrationFile("catalogue-service")], dart[DartIntegrationFile("catalogue-service")]
	in := BuildIntegration(r)
	for _, m := range in.Modules {
		for _, c := range m.Calls {
			for _, p := range c.Paths {
				mustContain(t, sw, fmt.Sprintf("@Test(%s)", swiftString(c.Name+": "+p.Name)))
				mustContain(t, dt, fmt.Sprintf("test(%s, () async {", dartString(p.Name)))
			}
			mustContain(t, dt, fmt.Sprintf("group(%s, () {", dartString(c.Name)))
		}
	}
	mustContain(t, swift[SwiftFile], "protocol IntegrationHarness {")
	mustContain(t, dart[DartLibraryFile], "abstract interface class IntegrationHarness {")
	if testing.Short() {
		t.Skip("building Swift and Dart packages is not short")
	}
	t.Run("swift", func(t *testing.T) {
		bin, err := exec.LookPath("swift")
		if err != nil {
			t.Skip("no swift on PATH")
		}
		if v, _ := exec.Command(bin, "--version").CombinedOutput(); !swiftSix.Match(v) {
			t.Skip("Swift Testing needs Swift 6")
		}
		dir := filepath.Join(out, "swift")
		var b strings.Builder
		b.WriteString(swiftStub)
		for _, n := range bodies(swift[SwiftFile], swiftBodyCall) {
			fmt.Fprintf(&b, "\nfunc body%s(_ h: any SpecArch.Harness) async throws {}\n", n)
		}
		b.WriteString(`
struct StubIntegration: SpecArch.IntegrationHarness {
    func call(_ c: SpecArch.ModuleCall, _ r: SpecArch.CallRequest) async throws -> SpecArch.Answer { .init(status: 0, body: .null) }
    func arrange(_ c: SpecArch.ModuleCall, status: Int) async throws {}
    func breakCall(_ c: SpecArch.ModuleCall, how: String) async throws {}
}

func makeIntegrationHarness() async throws -> any SpecArch.IntegrationHarness { StubIntegration() }
`)
		write(t, dir, map[string]string{
			"Package.swift":                "// swift-tools-version:6.0\nimport PackageDescription\n\nlet package = Package(name: \"Room\", targets: [.testTarget(name: \"RoomTests\")])\n",
			"Tests/RoomTests/" + SwiftFile: swift[SwiftFile],
			"Tests/RoomTests/" + SwiftIntegrationFile("catalogue-service"): sw,
			"Tests/RoomTests/Harness.swift":                                b.String(),
		})
		cmd := exec.Command(bin, "build", "--build-tests")
		cmd.Dir = dir
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("the generated tests do not compile: %v\n%s", err, b)
		}
	})
	t.Run("dart", func(t *testing.T) {
		bin, err := exec.LookPath("dart")
		if err != nil {
			t.Skip("no dart on PATH")
		}
		dir := filepath.Join(out, "dart")
		var b strings.Builder
		b.WriteString(dartStub)
		for _, n := range bodies(dart[DartTestFile], dartBodyCall) {
			fmt.Fprintf(&b, "\nFuture<void> body%s(Harness h) async {}\n", n)
		}
		b.WriteString(`
class StubIntegration implements IntegrationHarness {
  @override
  Future<Answer> call(ModuleCall c, CallRequest r) async => const Answer(0, Value.none);
  @override
  Future<void> arrange(ModuleCall c, int status) async {}
  @override
  Future<void> breakCall(ModuleCall c, String how) async {}
}

Future<IntegrationHarness> newIntegrationHarness() async => StubIntegration();
`)
		write(t, dir, map[string]string{
			"framework/pubspec.yaml":                               "name: test\nenvironment:\n  sdk: ^3.5.0\n",
			"framework/lib/test.dart":                              dartFramework,
			"app/pubspec.yaml":                                     "name: room\npublish_to: none\nenvironment:\n  sdk: ^3.5.0\ndev_dependencies:\n  test:\n    path: ../framework\n",
			"app/test/" + DartLibraryFile:                          dart[DartLibraryFile],
			"app/test/" + DartTestFile:                             dart[DartTestFile],
			"app/test/" + DartIntegrationFile("catalogue-service"): dt,
			"app/test/" + DartHarnessFile:                          b.String(),
		})
		cmd := exec.Command(bin, "pub", "get", "--offline")
		cmd.Dir = filepath.Join(dir, "app")
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("dart pub get: %v\n%s", err, b)
		}
		// The design tests' file of a specification whose tests check
		// nothing imports the library unused, which analyze warns of; only
		// what it finds in the files the integration tests add fails here.
		cmd = exec.Command(bin, "analyze", "--no-fatal-warnings")
		cmd.Dir = filepath.Join(dir, "app")
		b2, err := cmd.CombinedOutput()
		for _, f := range []string{DartLibraryFile + ":", DartIntegrationFile("catalogue-service") + ":"} {
			if strings.Contains(string(b2), f) {
				t.Fatalf("dart analyze finds problems in %s:\n%s", strings.TrimSuffix(f, ":"), b2)
			}
		}
		if err != nil && !strings.Contains(string(b2), "issue") {
			t.Fatalf("dart analyze: %v\n%s", err, b2)
		}
	})
}
