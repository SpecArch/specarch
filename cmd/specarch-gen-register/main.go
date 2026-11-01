// Command specarch-gen-register is the plug-in behind specarch generate
// register: it writes, per entity, the register template people fill in
// and specarch extract tables reads back, as CSV files and as a workbook
// with the instructions sheet. It reads specarch's request on standard
// input and answers with the files on standard output; it never touches
// the disk.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/capability"
	"github.com/SpecArch/specarch/internal/genregister"
)

// capabilities is what this plug-in answers to --capabilities.
var capabilities = capability.Plugin{
	Target:  "register",
	Version: specarch.Version,
	Summary: "Writes each entity's register template: the header row of its fields, and the instructions, a table of fields with each column's type, whether it is required, its sensitivity, its allowed values and its description.",
	Generates: []string{
		"<Entity>.csv, the header row, which a filled copy keeps",
		"<Entity>-instructions.csv, the table of fields, in the English vocabulary's words",
		"<Entity>.xlsx, both as the sheets <Entity> and Instructions",
	},
	DoesNotGenerate: []string{
		"a column for a field that holds a list or an object",
		"a draft notice or a problem mark inside a CSV file, which has no comment; the workbook's title carries the draft notice",
		"the words of another language's vocabulary",
	},
}

func main() {
	if len(os.Args) > 1 {
		os.Exit(capability.Serve(os.Args[1:], os.Stdout, os.Stderr, capabilities))
	}
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr))
}

// answer is the plug-in's answer: a workbook is not text, so it goes in
// base64 with its encoding named.
type answer struct {
	Files       []file                   `json:"files"`
	Diagnostics []genregister.Diagnostic `json:"diagnostics"`
}

type file struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding,omitempty"`
}

func run(stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-register: cannot read the request: %v\n", err)
		return 2
	}
	req, err := genregister.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-register: the request is not specarch's JSON (%v); run this plug-in through specarch generate register\n", err)
		return 2
	}
	resp := genregister.Generate(req)
	a := answer{Files: []file{}, Diagnostics: resp.Diagnostics}
	for _, f := range resp.Files {
		if strings.HasSuffix(f.Path, ".xlsx") {
			a.Files = append(a.Files, file{Path: f.Path, Content: base64.StdEncoding.EncodeToString([]byte(f.Content)), Encoding: "base64"})
		} else {
			a.Files = append(a.Files, file{Path: f.Path, Content: f.Content})
		}
	}
	out, err := json.Marshal(a)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-register: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-register: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
