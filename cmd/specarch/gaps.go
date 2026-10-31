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
// them (ADR-065), each estate's problems before them (ADR-113). With
// --next it prints only the question to ask next per specification
// (ADR-209). It exits 1 while a must or should question is open or the
// specification has errors, 0 otherwise, and 2 on a usage error.
func runGaps(args []string, stdout, stderr io.Writer) int {
	return gapsInto(args, stdout, stderr, nil)
}

// gapsInto is runGaps that also keeps, with --next, the questions it
// printed, for --json.
func gapsInto(args []string, stdout, stderr io.Writer, kept *[]generate.Next) int {
	next := false
	var paths []string
	for i, a := range args {
		if a == "--" {
			paths = append(paths, args[i+1:]...)
			break
		}
		if a == "--next" {
			next = true
			continue
		}
		if len(a) > 1 && a[0] == '-' {
			fmt.Fprintf(stderr, "specarch gaps has no option %s\n\n%s", a, usage)
			return 2
		}
		paths = append(paths, a)
	}
	args = paths
	if len(args) == 0 {
		fmt.Fprintf(stderr, "specarch gaps needs at least one folder\n\n%s", usage)
		return 2
	}
	if next {
		return gapsNext(args, stdout, stderr, kept)
	}
	// The errors are listed in the document, so they are not printed
	// before it as well; an estate's are in no module's document, so they
	// are printed first.
	specs, estates, invalid, status := readInputs(args, "gaps", io.Discard, stderr, nil)
	if status != 0 {
		return status
	}
	for _, ce := range estates {
		for _, d := range ce.diags {
			printDiagnostic(stdout, stderr, d)
		}
		if len(ce.diags) > 0 {
			fmt.Fprintln(stdout)
		}
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

// gapsNext prints the errors of the specifications given as problem
// lines, then each one's next question (ADR-209).
func gapsNext(args []string, stdout, stderr io.Writer, kept *[]generate.Next) int {
	specs, _, invalid, status := readInputs(args, "gaps", stdout, stderr, nil)
	if status != 0 {
		return status
	}
	holding := 0
	for i, l := range specs {
		if i > 0 || invalid {
			fmt.Fprintln(stdout)
		}
		st := l.state()
		n := generate.NextOf(l.spec.Root, filepath.ToSlash(l.spec.RootFile), l.impls, st)
		fmt.Fprint(stdout, generate.NextText(n))
		if kept != nil {
			*kept = append(*kept, n)
		}
		must, should, _ := validate.Questions(l.spec.Root)
		holding += must + should
	}
	if holding > 0 || invalid {
		return 1
	}
	return 0
}
