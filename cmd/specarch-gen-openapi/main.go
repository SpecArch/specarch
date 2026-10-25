// Command specarch-gen-openapi is the plug-in behind specarch generate
// openapi: it writes the OpenAPI 3.1 document of a specification, in the
// standard dialect. It reads specarch's request on standard input and
// answers with the document on standard output; it never touches the disk.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/capability"
	"github.com/SpecArch/specarch/internal/genopenapi"
)

// capabilities is what this plug-in answers to --capabilities.
var capabilities = capability.Plugin{
	Target:  "openapi",
	Version: specarch.Version,
	Summary: "Writes the OpenAPI 3.1 document of the specification's operations, in the standard dialect or the dxlib one.",
	Generates: []string{
		"openapi.yaml: the paths and operations, a schema per entity and enum, the security schemes the permissions or the declared tokens give",
		"x-specarch- extensions for what OpenAPI has no keyword for: permission, limits, emits, sensitivity and the others",
	},
	DoesNotGenerate: []string{
		"handlers or server code: go-dxlib, or a code generator of the stack, turns the document into code",
		"the rationale and the test hints of the specification",
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
		fmt.Fprintf(stderr, "specarch-gen-openapi: cannot read the request: %v\n", err)
		return 2
	}
	req, err := genopenapi.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-openapi: the request is not specarch's JSON (%v); run this plug-in through specarch generate openapi\n", err)
		return 2
	}
	out, err := json.Marshal(genopenapi.Generate(req))
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-openapi: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-openapi: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
