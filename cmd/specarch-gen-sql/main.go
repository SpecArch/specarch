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

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/capability"
	"github.com/SpecArch/specarch/internal/gensql"
)

// capabilities is what this plug-in answers to --capabilities.
var capabilities = capability.Plugin{
	Target:  "sql",
	Version: specarch.Version,
	Summary: "Writes the SQL migrations in the target's dialect: PostgreSQL, SQL Server, Oracle or MariaDB.",
	Generates: []string{
		"0001_expand.sql and snapshot.yaml, then the next NNNN_expand.sql and, when the settings allow destructive changes, NNNN_contract.sql for each change of the schema",
		"tables with columns typed through the type-rendering idiom, checks, keys, indexes and views",
	},
	DoesNotGenerate: []string{
		"a change it cannot tell from a rewrite (a new type, key or default): refused with what to do by hand",
		"data corrections or backfills: the scripts target writes guarded scripts",
		"running the migrations against a database",
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
