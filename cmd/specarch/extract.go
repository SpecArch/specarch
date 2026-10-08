package main

import (
	"fmt"
	"io"
)

// runExtract is the extract command: designed in the specification, built
// later. This build answers with status 2, as for any command it does not
// offer.
func runExtract(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "specarch extract needs a source and at least one path\n\n%s", usage)
		return 2
	}
	fmt.Fprintln(stderr, "specarch extract: this build does not offer extract; it is designed in spec/ and described in docs/extraction.md, and not built yet")
	return 2
}
