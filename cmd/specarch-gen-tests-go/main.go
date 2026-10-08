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

	"github.com/SpecArch/specarch/internal/gentests"
)

func main() {
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
