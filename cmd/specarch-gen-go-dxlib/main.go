// Command specarch-gen-go-dxlib is the plug-in behind specarch generate
// go-dxlib: it writes the Go of a service on dxlib from a specification. It
// reads specarch's request on standard input and answers with the file on
// standard output; it never touches the disk.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/capability"
	"github.com/SpecArch/specarch/internal/gendxlib"
	"github.com/SpecArch/specarch/internal/genopenapi"
)

// capabilities is what this plug-in answers to --capabilities.
var capabilities = capability.Plugin{
	Target:  "go-dxlib",
	Version: specarch.Version,
	Summary: "Writes the Go file that binds a dxlib service to the specification.",
	Generates: []string{
		"specarch_dxlib.go: a dxlib table per entity with the search, order and filter fields its lists allow, Register binding each handler to its operationId, and a request struct per operation",
	},
	DoesNotGenerate: []string{
		"the handlers' bodies, written by hand against the generated structs",
		"the database schema, which the sql target writes",
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
		fmt.Fprintf(stderr, "specarch-gen-go-dxlib: cannot read the request: %v\n", err)
		return 2
	}
	req, err := genopenapi.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-go-dxlib: the request is not specarch's JSON (%v); run this plug-in through specarch generate go-dxlib\n", err)
		return 2
	}
	out, err := json.Marshal(gendxlib.Generate(req))
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-go-dxlib: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-go-dxlib: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
