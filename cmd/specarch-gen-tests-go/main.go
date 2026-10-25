// Command specarch-gen-tests-go is the plug-in behind
// specarch generate tests for an implementation file whose language is Go.
// It reads specarch's request on standard input and answers with the Go
// test file on standard output; it never touches the disk.
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
	Stack:   "go",
	Version: specarch.Version,
	Summary: "Writes the tests of the worked examples and the design tests, in Go.",
	Generates: []string{
		"specarch_design_test.go: one test per worked example and per design test, with given, when and then steps, calling a Harness interface",
	},
	DoesNotGenerate: []string{
		"newHarness, the project's file that signs a caller in, inserts records and calls operations",
		"the body of a test whose sentences cannot be turned into code, left for a person to write",
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
		fmt.Fprintf(stderr, "specarch-gen-tests-go: cannot read the request: %v\n", err)
		return 2
	}
	req, err := gentests.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-tests-go: the request is not specarch's JSON (%v); run this plug-in through specarch generate tests\n", err)
		return 2
	}
	out, err := json.Marshal(gentests.GenerateGo(req))
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-tests-go: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-tests-go: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
