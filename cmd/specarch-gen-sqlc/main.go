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

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/capability"
	"github.com/SpecArch/specarch/internal/gensqlc"
)

// capabilities is what this plug-in answers to --capabilities.
var capabilities = capability.Plugin{
	Target:  "sqlc",
	Version: specarch.Version,
	Summary: "Writes the sqlc queries the design derives, in PostgreSQL, against the tables the sql target writes.",
	Generates: []string{
		"<table>.sql per table: insert, read by key, change, and a list page with its total, each derived by keyword and named after its operation",
	},
	DoesNotGenerate: []string{
		"a query no keyword derives, which is written by hand",
		"the sqlc configuration and the Go code sqlc generates",
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
