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
// ready, drafts or waiting, after the errors of a specification that has
// them (ADR-065). It exits 1 while a must or should question is open or the
// specification has errors, 0 otherwise, and 2 on a usage error.
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
	// The errors are listed in the document, so they are not printed
	// before it as well.
	specs, invalid, status := readSpecs(args, "gaps", io.Discard, stderr)
	if status != 0 {
		return status
	}
	holding := 0
	for _, l := range specs {
		rel := filepath.ToSlash(l.spec.RootFile)
		st := l.state()
		st.Marks = l.marks(".")
		text := generate.Questions(l.spec.Root, rel, l.impls, st)
		// The document's generated-from header is for a file on disk; on
		// the terminal the document starts at its title.
		if _, rest, ok := strings.Cut(text, "\n\n"); ok && strings.HasPrefix(text, "<!--") {
			text = rest
		}
		fmt.Fprint(stdout, text)
		must, should, _ := validate.Questions(l.spec.Root)
		holding += must + should
	}
	if holding > 0 || invalid {
		return 1
	}
	return 0
}
