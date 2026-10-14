// Command specarch-gen-sqlc is the plug-in behind specarch generate sqlc:
// it writes the sqlc queries a specification derives, against the tables
// specarch-gen-sql writes. It reads specarch's request on standard input
// and answers with the files on standard output; it never touches the
// disk.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SpecArch/specarch/internal/gensqlc"
)

func main() {
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr))
}

func run(stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-sqlc: cannot read the request: %v\n", err)
		return 2
	}
	req, err := gensqlc.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-sqlc: the request is not specarch's JSON (%v); run this plug-in through specarch generate sqlc\n", err)
		return 2
	}
	out, err := json.Marshal(gensqlc.Generate(req))
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-sqlc: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-sqlc: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
