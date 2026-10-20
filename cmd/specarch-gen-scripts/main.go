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

	"github.com/SpecArch/specarch/internal/genscripts"
)

func main() {
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
