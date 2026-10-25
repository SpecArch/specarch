// Command specarch-gen-bpmn is the plug-in behind specarch generate
// bpmn: it writes each workflow of a specification as a BPMN 2.0 XML file
// with its diagram, and the diagram as SVG. It reads specarch's request
// on standard input and answers with the files on standard output; it
// never touches the disk.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/capability"
	"github.com/SpecArch/specarch/internal/genbpmn"
)

// capabilities is what this plug-in answers to --capabilities.
var capabilities = capability.Plugin{
	Target:  "bpmn",
	Version: specarch.Version,
	Summary: "Writes each workflow as a BPMN 2.0 XML file and the same diagram as an SVG picture.",
	Generates: []string{
		"<workflow>.bpmn, the process a modeller or an engine opens",
		"<workflow>.svg, the diagram for documents",
	},
	DoesNotGenerate: []string{
		"the engine's deployment or the service tasks' code",
		"anything beyond the sequential subset of BPMN the meta-model takes",
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
		fmt.Fprintf(stderr, "specarch-gen-bpmn: cannot read the request: %v\n", err)
		return 2
	}
	req, err := genbpmn.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-bpmn: the request is not specarch's JSON (%v); run this plug-in through specarch generate bpmn\n", err)
		return 2
	}
	out, err := json.Marshal(genbpmn.Generate(req))
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-bpmn: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-bpmn: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
