// Command specarch-gen-tests-dart is the plug-in behind
// specarch generate tests for an implementation file whose language is Dart.
// It reads specarch's request on standard input and answers with the Dart
// test files on standard output; it never touches the disk.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/capability"
	"github.com/SpecArch/specarch/internal/gentests"
)

// capabilities is what this plug-in answers to --capabilities.
var capabilities = capability.Plugin{
	Target:  "tests",
	Stack:   "dart",
	Version: specarch.Version,
	Summary: "Writes the tests of the worked examples and the design tests, in Dart for package:test or flutter_test.",
	Generates: []string{
		"specarch_design.dart: the values, the Harness interface and the checks",
		"specarch_design_test.dart: one test per worked example and per design test",
	},
	DoesNotGenerate: []string{
		"specarch_harness.dart with newHarness and each body<TestName> the design gives no call for, which the project writes",
	},
}

func main() {
	if len(os.Args) > 1 {
		os.Exit(capability.Serve(os.Args[1:], os.Stdout, os.Stderr, capabilities))
	}
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr))
}

func run(stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-tests-dart: cannot read the request: %v\n", err)
		return 2
	}
	req, err := gentests.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-tests-dart: the request is not specarch's JSON (%v); run this plug-in through specarch generate tests\n", err)
		return 2
	}
	out, err := json.Marshal(gentests.GenerateDart(req))
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-tests-dart: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-tests-dart: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
