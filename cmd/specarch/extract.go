package main

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/SpecArch/specarch/internal/extract"
)

// extractSources are the surfaces this build reads, in the order the usage
// text lists them.
var extractSources = []string{"outline", "database", "router", "documents", "openapi", "permissions", "pages", "workflows"}

var sourceKey = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// runExtract is the extract command: 0 when the tree was written, 1 when the
// surface cannot be read as the source expects (no commit names it, or it is
// not what the reader takes), 2 on a usage error, a source this build does
// not read, or a path that cannot be read or written.
func runExtract(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "specarch extract needs a source and at least one path\n\n%s", usage)
		return 2
	}
	source := args[0]
	out, key := "", ""
	var paths []string
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case a == "--":
			paths = append(paths, rest[i+1:]...)
			i = len(rest)
		case a == "--out" || a == "--source-key":
			if i+1 >= len(rest) {
				fmt.Fprintf(stderr, "specarch extract: %s needs a value\n\n%s", a, usage)
				return 2
			}
			if a == "--out" {
				out = rest[i+1]
			} else {
				key = rest[i+1]
			}
			i++
		case len(a) > 1 && a[0] == '-':
			fmt.Fprintf(stderr, "specarch extract has no option %s\n\n%s", a, usage)
			return 2
		default:
			paths = append(paths, a)
		}
	}
	offered := false
	for _, s := range extractSources {
		offered = offered || s == source
	}
	if !offered {
		fmt.Fprintf(stderr, "specarch extract: this build does not read the source %s; it reads %s, and the others are built in the steps of docs/extraction.md\n", source, sourceList())
		return 2
	}
	switch {
	case out == "":
		fmt.Fprintf(stderr, "specarch extract needs --out, the folder the specification is written into\n\n%s", usage)
		return 2
	case len(paths) == 0:
		fmt.Fprintf(stderr, "specarch extract %s needs at least one path\n\n%s", source, usage)
		return 2
	case key != "" && !sourceKey.MatchString(key):
		fmt.Fprintf(stderr, "specarch extract: --source-key %s is not a source key; write it in kebab-case, such as code\n", key)
		return 2
	case source == "database" && len(paths) != 1:
		fmt.Fprintf(stderr, "specarch extract database reads one catalogue dump, and was given %d paths\n", len(paths))
		return 2
	case source == "router" && len(paths) != 1:
		fmt.Fprintf(stderr, "specarch extract router reads one route table, and was given %d paths\n", len(paths))
		return 2
	case source == "documents" && len(paths) != 1:
		fmt.Fprintf(stderr, "specarch extract documents reads one document, and was given %d paths; one source is written per document file\n", len(paths))
		return 2
	case source == "pages" && len(paths) != 1:
		fmt.Fprintf(stderr, "specarch extract pages reads one router's root folder, and was given %d paths\n", len(paths))
		return 2
	case source == "workflows" && len(paths) != 1:
		fmt.Fprintf(stderr, "specarch extract workflows reads one BPMN 2.0 XML file, and was given %d paths\n", len(paths))
		return 2
	case source == "openapi" && len(paths) != 1:
		fmt.Fprintf(stderr, "specarch extract openapi reads one OpenAPI document, and was given %d paths; one source is written per document file\n", len(paths))
		return 2
	}
	if key == "" && source != "documents" && source != "openapi" {
		key = "code"
	}
	var res *extract.Result
	var err error
	switch source {
	case "outline":
		res, err = extract.Outline(paths, out, key)
	case "database":
		res, err = extract.Database(paths[0], out, key)
	case "router":
		res, err = extract.Router(paths[0], out, key)
	case "documents":
		res, err = extract.Documents(paths[0], out, key)
	case "openapi":
		res, err = extract.OpenAPI(paths[0], out, key)
	case "permissions":
		res, err = extract.Permissions(paths[0], out, key)
	case "pages":
		res, err = extract.Pages(paths[0], out, key)
	case "workflows":
		res, err = extract.Workflows(paths[0], out, key)
	}
	if err != nil {
		var refusal *extract.Refusal
		if errors.As(err, &refusal) {
			fmt.Fprintf(stderr, "specarch extract %s: %s\n", source, refusal.Reason)
			return 1
		}
		fmt.Fprintf(stderr, "specarch extract %s: %v\n", source, err)
		return 2
	}
	if err := res.Tree.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch extract %s: cannot write %s: %v\n", source, out, err)
		return 2
	}
	for _, line := range res.Lines {
		fmt.Fprintln(stdout, line)
	}
	return 0
}

// sourceList names the sources this build reads, as a sentence lists them.
func sourceList() string {
	n := len(extractSources)
	if n == 1 {
		return extractSources[0]
	}
	return strings.Join(extractSources[:n-1], ", ") + " and " + extractSources[n-1]
}
