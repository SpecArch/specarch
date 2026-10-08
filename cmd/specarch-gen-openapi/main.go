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

	"github.com/SpecArch/specarch/internal/genopenapi"
)

func main() {
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
