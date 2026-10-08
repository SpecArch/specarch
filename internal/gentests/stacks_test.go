package gentests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// asStack makes the example's implementation files stand in for a project
// on another stack: the Go testing framework is left out, so the plug-in
// writes for its stack's own.
func asStack(content map[string]any) { delete(content, "testing") }

// write puts files in a folder, making the folders they need.
func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func generated(t *testing.T, resp Response, want ...string) map[string]string {
	t.Helper()
	if len(resp.Diagnostics) > 0 {
		t.Fatalf("want no diagnostics, got %v", resp.Diagnostics)
	}
	files := map[string]string{}
	for _, f := range resp.Files {
		files[f.Path] = f.Content
	}
	if len(files) != len(want) {
		t.Fatalf("want the files %v, got %d", want, len(files))
	}
	for _, w := range want {
		if _, ok := files[w]; !ok {
			t.Fatalf("want the file %s, got %v", w, resp.Files)
		}
	}
	return files
}

func mustContain(t *testing.T, content string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(content, want) {
			t.Errorf("the generated file has no %q", want)
		}
	}
}

const swiftStub = `struct StubHarness: SpecArch.Harness {
    func signIn(role: String) async throws {}
    func insert(_ mapping: String, _ record: SpecArch.Record) async throws {}
    func records(_ mapping: String) async throws -> [SpecArch.Record] { [] }
    func request(_ mapping: String, method: String, path: String, input: SpecArch.Record) async throws -> SpecArch.Response {
        .init(status: 0, body: [:])
    }
    func run(_ mapping: String, command: String, arguments: [String: [SpecArch.Value]], options: SpecArch.Record) async throws -> SpecArch.CommandResult {
        .init(exit: 0, output: [])
    }
    func open(_ mapping: String, page: String, route: String, input: SpecArch.Record) async throws -> SpecArch.Response {
        .init(status: 0, body: [:])
    }
    func emitted() async throws -> [String] { [] }
    func compute(_ mapping: String, input: SpecArch.Record) async throws -> SpecArch.Value { .null }
}

func makeHarness() async throws -> any SpecArch.Harness { StubHarness() }
`

var swiftBodyCall = regexp.MustCompile(`\bbody([A-Za-z0-9]+)\(h\)`)

// swiftSix matches the version line of a Swift toolchain that has Swift
// Testing: 6.0 or later.
var swiftSix = regexp.MustCompile(`Swift version ([6-9]|[1-9][0-9])\.`)

// TestLibraryLendingSwiftCompiles generates the Swift tests of the library
// lending example and builds them in a package of their own beside a
// harness that does nothing and an empty body for every test the design
// gives no call for.
func TestLibraryLendingSwiftCompiles(t *testing.T) {
	r, out := libraryLending(t, asStack)
	content := generated(t, GenerateSwift(r), SwiftFile)[SwiftFile]
	mustContain(t, content, "func testLendACopy() async throws {", "func testAlgorithmLateFee(example: String, input: Record, expected: Value) async throws {", `Value(kind: "decimal", text: "3.50")`)

	if testing.Short() {
		t.Skip("building a Swift package is not short")
	}
	swift, err := exec.LookPath("swift")
	if err != nil {
		t.Skip("no swift on PATH to compile the generated file with")
	}
	if v, _ := exec.Command(swift, "--version").CombinedOutput(); !swiftSix.Match(v) {
		t.Skipf("Swift Testing needs Swift 6; this toolchain is %s", strings.SplitN(string(v), "\n", 2)[0])
	}
	var b strings.Builder
	b.WriteString(swiftStub)
	for _, n := range bodies(content, swiftBodyCall) {
		fmt.Fprintf(&b, "\nfunc body%s(_ h: any SpecArch.Harness) async throws {}\n", n)
	}
	write(t, out, map[string]string{
		"Package.swift":                    "// swift-tools-version:6.0\nimport PackageDescription\n\nlet package = Package(name: \"Lending\", targets: [.testTarget(name: \"LendingTests\")])\n",
		"Tests/LendingTests/" + SwiftFile:  content,
		"Tests/LendingTests/Harness.swift": b.String(),
	})
	cmd := exec.Command(swift, "build", "--build-tests")
	cmd.Dir = out
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the generated tests do not compile: %v\n%s", err, b)
	}
}

// A stand-in for package:test and flutter_test with the three functions
// the generated files use, so that the analysis needs no network.
const dartFramework = `typedef Body = dynamic Function();

void test(String description, Body body) {}

void group(String description, void Function() body) {}

Never fail(String message) => throw StateError(message);
`

const dartStub = `import 'specarch_design.dart';

class StubHarness implements Harness {
  @override
  Future<void> signIn(String role) async {}
  @override
  Future<void> insert(String mapping, Fields record) async {}
  @override
  Future<List<Fields>> records(String mapping) async => [];
  @override
  Future<Response> request(String mapping, String method, String path, Fields input) async => const Response(0, {});
  @override
  Future<CommandResult> run(String mapping, String command, Map<String, List<Value>> arguments, Fields options) async =>
      const CommandResult(0, []);
  @override
  Future<Response> open(String mapping, String page, String route, Fields input) async => const Response(0, {});
  @override
  Future<List<String>> emitted() async => [];
  @override
  Future<Value> compute(String mapping, Fields input) async => Value.none;
}

Future<Harness> newHarness() async => StubHarness();
`

var dartBodyCall = regexp.MustCompile(`\bbody([A-Za-z0-9]+)\(h\)`)

// TestLibraryLendingDartAnalyzes generates the Dart tests of the library
// lending example, for package:test and for flutter_test, and analyzes them
// beside a harness that does nothing and an empty body for every test the
// design gives no call for, against a stand-in for the framework.
func TestLibraryLendingDartAnalyzes(t *testing.T) {
	for _, fw := range []string{"package:test", "flutter_test"} {
		t.Run(fw, func(t *testing.T) {
			r, out := libraryLending(t, func(content map[string]any) {
				content["testing"] = map[string]any{"framework": fw}
			})
			files := generated(t, GenerateDart(r), DartLibraryFile, DartTestFile)
			mustContain(t, files[DartTestFile], "import '"+dartFrameworks[fw]+"';", "test('lend-a-copy: operation createLoan, golden scenario', () async {", "group('algorithm lateFee', () {", "const Value('decimal', '3.50')")
			mustContain(t, files[DartLibraryFile], "import '"+dartFrameworks[fw]+"';", "abstract interface class Harness {")

			if testing.Short() {
				t.Skip("analyzing a Dart package is not short")
			}
			dart, err := exec.LookPath("dart")
			if err != nil {
				t.Skip("no dart on PATH to analyze the generated files with")
			}
			// package:<pkg>/<lib>, the import the framework is used through
			pkg, lib, _ := strings.Cut(strings.TrimPrefix(dartFrameworks[fw], "package:"), "/")
			var b strings.Builder
			b.WriteString(dartStub)
			for _, n := range bodies(files[DartTestFile], dartBodyCall) {
				fmt.Fprintf(&b, "\nFuture<void> body%s(Harness h) async {}\n", n)
			}
			write(t, out, map[string]string{
				"framework/pubspec.yaml":      "name: " + pkg + "\nenvironment:\n  sdk: ^3.5.0\n",
				"framework/lib/" + lib:        dartFramework,
				"app/pubspec.yaml":            "name: lending\npublish_to: none\nenvironment:\n  sdk: ^3.5.0\ndev_dependencies:\n  " + pkg + ":\n    path: ../framework\n",
				"app/test/" + DartLibraryFile: files[DartLibraryFile],
				"app/test/" + DartTestFile:    files[DartTestFile],
				"app/test/" + DartHarnessFile: b.String(),
			})
			for _, args := range [][]string{{"pub", "get", "--offline"}, {"analyze", "--fatal-infos"}} {
				cmd := exec.Command(dart, args...)
				cmd.Dir = filepath.Join(out, "app")
				if b, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("dart %s: %v\n%s", strings.Join(args, " "), err, b)
				}
			}
		})
	}
}

// TestFrameworkRefused checks that a framework the plug-in does not write
// for is refused by name, with nothing written.
func TestFrameworkRefused(t *testing.T) {
	for name, gen := range map[string]func(*Request) Response{"swift": GenerateSwift, "dart": GenerateDart} {
		r, _ := libraryLending(t, func(content map[string]any) {
			content["testing"] = map[string]any{"framework": "XCTest"}
		})
		resp := gen(r)
		if len(resp.Files) != 0 || len(resp.Diagnostics) != 1 || resp.Diagnostics[0].Path != "/testing/framework" || !strings.Contains(resp.Diagnostics[0].Message, `"XCTest"`) {
			t.Errorf("%s: want one diagnostic on /testing/framework naming XCTest and no files, got %v", name, resp)
		}
	}
}
