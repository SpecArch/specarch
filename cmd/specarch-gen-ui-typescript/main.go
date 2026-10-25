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

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/capability"
	"github.com/SpecArch/specarch/internal/genuits"
)

// capabilities is what this plug-in answers to --capabilities.
var capabilities = capability.Plugin{
	Target:  "ui",
	Stack:   "typescript",
	Version: specarch.Version,
	Summary: "Writes the pages of a ui target on platform web with framework nextjs-carbon, in TypeScript.",
	Generates: []string{
		"page.schema.ts and page.tsx per page under app/",
		"the components that read the schemas under screens/, strings.ts with every text, and the server routes",
	},
	DoesNotGenerate: []string{
		"the Next.js project's package.json and configuration",
		"a component the ui-components idiom does not name",
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
