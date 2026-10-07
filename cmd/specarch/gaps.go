package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/validate"
)

// runGaps prints the open questions document of every specification given:
// the questions by stage with what they block, and which outputs are
// ready, drafts or waiting. It exits 1 while a must or should question is
// open, 0 when none is, and 2 on a usage error or a specification with
// errors.
func runGaps(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(stderr, "specarch gaps needs at least one folder\n\n%s", usage)
		return 2
	}
	for _, a := range args {
		if len(a) > 1 && a[0] == '-' {
			fmt.Fprintf(stderr, "specarch gaps has no option %s\n\n%s", a, usage)
			return 2
		}
	}
	specs, status := loadSpecs(args, "gaps", stdout, stderr)
	if status != 0 {
		if status == 1 {
			return 2
		}
		return status
	}
	holding := 0
	for _, l := range specs {
		rel := filepath.ToSlash(l.spec.RootFile)
		text := generate.Questions(l.spec.Root, rel, l.impls, l.state())
		// The document's generated-from header is for a file on disk; on
		// the terminal the document starts at its title.
		if _, rest, ok := strings.Cut(text, "\n\n"); ok && strings.HasPrefix(text, "<!--") {
			text = rest
		}
		fmt.Fprint(stdout, text)
		must, should, _ := validate.Questions(l.spec.Root)
		holding += must + should
	}
	if holding > 0 {
		return 1
	}
	return 0
}
