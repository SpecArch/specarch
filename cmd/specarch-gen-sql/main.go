// Command specarch-gen-sql is the plug-in behind specarch generate sql: it
// writes the SQL migrations of a specification in the dialect of the
// target, through the type-rendering idiom. It reads specarch's request on
// standard input and answers with the files on standard output; it never
// touches the disk.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SpecArch/specarch/internal/gensql"
)

func main() {
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr))
}

func run(stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-sql: cannot read the request: %v\n", err)
		return 2
	}
	req, err := gensql.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-sql: the request is not specarch's JSON (%v); run this plug-in through specarch generate sql\n", err)
		return 2
	}
	out, err := json.Marshal(gensql.Generate(req))
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-sql: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-sql: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
