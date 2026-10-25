// Command specarch-gen-tests-swift is the plug-in behind
// specarch generate tests for an implementation file whose language is Swift.
// It reads specarch's request on standard input and answers with the Swift
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
	Stack:   "swift",
	Version: specarch.Version,
	Summary: "Writes the tests of the worked examples and the design tests, for Swift Testing.",
	Generates: []string{
		"SpecArchDesignTests.swift: the values, the Harness protocol, the checks and one test per worked example and per design test",
	},
	DoesNotGenerate: []string{
		"makeHarness and each body<TestName> the design gives no call for, which the project writes",
		"tests for any framework but Swift Testing",
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
		fmt.Fprintf(stderr, "specarch-gen-tests-swift: cannot read the request: %v\n", err)
		return 2
	}
	req, err := gentests.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-tests-swift: the request is not specarch's JSON (%v); run this plug-in through specarch generate tests\n", err)
		return 2
	}
	out, err := json.Marshal(gentests.GenerateSwift(req))
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-tests-swift: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-tests-swift: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
