// Command specarch-gen-ui-typescript is the plug-in behind specarch
// generate ui for an implementation file in TypeScript: it writes the
// screens of a specification for Next.js's app router and IBM's Carbon
// design system. It reads specarch's request on standard input and answers
// with the files on standard output; it never touches the disk.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SpecArch/specarch/internal/genuits"
)

func main() {
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr))
}

func run(stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-ui-typescript: cannot read the request: %v\n", err)
		return 2
	}
	req, err := genuits.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-ui-typescript: the request is not specarch's JSON (%v); run this plug-in through specarch generate ui\n", err)
		return 2
	}
	out, err := json.Marshal(genuits.Generate(req))
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-ui-typescript: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-ui-typescript: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
