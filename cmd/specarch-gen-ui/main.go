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

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/capability"
	"github.com/SpecArch/specarch/internal/genui"
)

// capabilities is what this plug-in answers to --capabilities.
var capabilities = capability.Plugin{
	Target:  "ui",
	Version: specarch.Version,
	Summary: "Writes the screens of a ui target on platform web with framework plain-javascript, as native ES modules with no build step.",
	Generates: []string{
		"<page>.html and <page>.js per list, view, form, task and content page, and the route of each custom page around a screen the project writes",
		"events.js, transport.js, service.js and theme.css, once",
	},
	DoesNotGenerate: []string{
		"the server the screens call",
		"a third-party file, a package or a bundler configuration",
		"a layout the specification does not describe",
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
