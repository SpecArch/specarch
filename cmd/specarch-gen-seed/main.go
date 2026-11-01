// Command specarch-gen-seed is the plug-in behind specarch generate seed:
// it writes the development seed of a specification, made-up rows for the
// tables specarch-gen-sql writes, drawn from the entities with a fixed
// seed. It reads specarch's request on standard input and answers with the
// file on standard output; it never touches the disk or a database.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/capability"
	"github.com/SpecArch/specarch/internal/genseed"
)

// capabilities is what this plug-in answers to --capabilities.
var capabilities = capability.Plugin{
	Target:  "seed",
	Version: specarch.Version,
	Summary: "Writes seed.sql, made-up development rows for the tables generate sql writes, in the target's dialect: PostgreSQL, SQL Server, Oracle or MariaDB.",
	Generates: []string{
		"seed.sql: one INSERT per row, the tables in the order their foreign keys need, the counts from settings.rows (ten per entity otherwise), drawn from a fixed settings.seed so the file is the same on every run",
		"values from each field's type, format, bounds, enum, pattern and examples, keys and unique constraints that never repeat, check constraints satisfied, foreign keys pointing at rows of the file, audit columns and the soft-delete flag as the idioms mean them",
	},
	DoesNotGenerate: []string{
		"a value for a required field encrypted at rest: its key and the salt of its hash are the service's, so the entity is an error and such records are made through the service",
		"anything that truncates or deletes: the file is loaded once into a database the migrations have just built",
		"running the file against a database",
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
		fmt.Fprintf(stderr, "specarch-gen-seed: cannot read the request: %v\n", err)
		return 2
	}
	req, err := genseed.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-seed: the request is not specarch's JSON (%v); run this plug-in through specarch generate seed\n", err)
		return 2
	}
	out, err := json.Marshal(genseed.Generate(req))
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-seed: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-seed: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
