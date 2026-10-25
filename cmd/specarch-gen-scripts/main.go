// Command specarch-gen-scripts is the plug-in behind specarch generate
// scripts: it writes the guarded operational scripts a specification
// derives, against the tables specarch-gen-sql writes. It reads
// specarch's request on standard input and answers with the files on
// standard output; it never touches the disk.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/capability"
	"github.com/SpecArch/specarch/internal/genscripts"
)

// capabilities is what this plug-in answers to --capabilities.
var capabilities = capability.Plugin{
	Target:  "scripts",
	Version: specarch.Version,
	Summary: "Writes a guarded script per operation or command that carries a guard, for psql or the mariadb client.",
	Generates: []string{
		"one script per guarded operation or command: the precondition checked, the exact count of rows changed checked, the change derived by keyword or by the guard's writes",
	},
	DoesNotGenerate: []string{
		"running the script against a live system",
		"a data change with no guard",
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
		fmt.Fprintf(stderr, "specarch-gen-scripts: cannot read the request: %v\n", err)
		return 2
	}
	req, err := genscripts.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-scripts: the request is not specarch's JSON (%v); run this plug-in through specarch generate scripts\n", err)
		return 2
	}
	out, err := json.Marshal(genscripts.Generate(req))
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-scripts: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-scripts: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
