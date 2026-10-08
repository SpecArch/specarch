// Command specarch-gen-ui is the plug-in behind specarch generate ui: it
// writes the screens of a specification for the web in plain JavaScript.
// It reads specarch's request on standard input and answers with the files
// on standard output; it never touches the disk.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SpecArch/specarch/internal/genui"
)

func main() {
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr))
}

func run(stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-ui: cannot read the request: %v\n", err)
		return 2
	}
	req, err := genui.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-ui: the request is not specarch's JSON (%v); run this plug-in through specarch generate ui\n", err)
		return 2
	}
	out, err := json.Marshal(genui.Generate(req))
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-ui: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-ui: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
