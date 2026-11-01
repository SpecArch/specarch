// Command specarch-gen-ui-vue-htm is the plug-in behind specarch generate
// ui for a ui target of platform web and framework vue-htm: it writes the
// screens of a specification for the web with no build step, on Vue's
// runtime-only build and htm. It reads specarch's request on standard
// input and answers with the files on standard output; it never touches
// the disk.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/capability"
	"github.com/SpecArch/specarch/internal/genvuehtm"
)

// capabilities is what this plug-in answers to --capabilities.
var capabilities = capability.Plugin{
	Target:  "ui",
	Stack:   "vue-htm",
	Version: specarch.Version,
	Summary: "Writes the screens of a ui target on platform web with framework vue-htm, from an implementation file in JavaScript: the menus, pages and actions as data files, one data file per page, the calls through one api.call, the components on Vue's runtime-only build and htm with no build step, index.html loading the vendored files with their integrity, and a vendor manifest.",
	Generates: []string{
		"pages/<page>.js per task, list, form, view, content and custom page, one data file in one shape",
		"config/menus.js, config/pages.js, config/actions.js and config/application.js, the application's data",
		"app.js, api.js, transport.js, identity-provider.js, strings.js, theme.css, index.html and vendor/manifest.js, once",
		"the components under screens/, and a derived test per guarded page and for the menu under tests/",
	},
	DoesNotGenerate: []string{
		"the service the screens call",
		"a vendored library's file, which the project commits beside the generated code and names under libraries.<name>.files",
		"a custom page's screen (custom/<page>.js) or a form's hooks (hooks/<page>.js), which the project writes",
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
		fmt.Fprintf(stderr, "specarch-gen-ui-vue-htm: cannot read the request: %v\n", err)
		return 2
	}
	req, err := genvuehtm.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-ui-vue-htm: the request is not specarch's JSON (%v); run this plug-in through specarch generate ui\n", err)
		return 2
	}
	out, err := json.Marshal(genvuehtm.Generate(req))
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-ui-vue-htm: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-ui-vue-htm: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
