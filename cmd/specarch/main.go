// Command specarch checks SpecArch specifications and makes documents and
// code from them.
//
//	specarch validate <folder or file>...
//	specarch document <target> [--out <folder>] [--check] <folder>...
//	specarch generate <target> [--out <folder>] [--check] <folder>...
//	specarch extract <source> ...
//	specarch diff <old> <new>
//	specarch version
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// version is the program version, set at release time.
const version = "0.2.0"

const usage = `usage:
  specarch validate <folder or file>...    check specifications and implementation files
  specarch gaps <folder>...                 list the open questions and what they hold up
  specarch document <target> [--out <folder>] [--check] <folder>...
                                            write a document from a specification
  specarch approve --by <stakeholder> [--date <date>] <folder>...
                                            record that the documents were read and the specification is approved
  specarch generate <target> [--out <folder>] [--check] [--unapproved] <folder>...
                                            write code or data from an approved specification
  specarch extract <source> ...             write a specification from existing code or documents
  specarch diff <old folder> <new folder>   list what changed between two versions and check the release
  specarch version                          print the program version

A specification is a folder holding specarch.yaml. A folder given here is
searched for specifications and for *.specarch-implementation.yaml files
outside one.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "gaps":
		return runGaps(args[1:], stdout, stderr)
	case "approve":
		return runApprove(args[1:], stdout, stderr)
	case "document":
		return runDocument(args[1:], stdout, stderr)
	case "generate":
		return runGenerate(args[1:], stdout, stderr)
	case "extract":
		return runExtract(args[1:], stdout, stderr)
	case "diff":
		return runDiff(args[1:], stdout, stderr)
	case "version":
		if len(args) > 1 {
			fmt.Fprintf(stderr, "specarch version takes no arguments\n\n%s", usage)
			return 2
		}
		return runVersion(stdout)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	}
	fmt.Fprintf(stderr, "specarch has no command %q\n\n%s", args[0], usage)
	return 2
}

func runVersion(stdout io.Writer) int {
	fmt.Fprintf(stdout, "specarch %s\n", version)
	fmt.Fprintln(stdout, "specifications: meta-model 0.1")
	fmt.Fprintln(stdout, "implementation files: meta-model 0.1")
	return 0
}

// runValidate implements the exitStatus algorithm of the design: 2 for a
// usage or read error, 1 when any diagnostic was printed, 0 otherwise.
func runValidate(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(stderr, "specarch validate needs at least one folder or file\n\n%s", usage)
		return 2
	}
	for _, a := range args {
		if len(a) > 1 && a[0] == '-' {
			fmt.Fprintf(stderr, "specarch validate has no option %s; to check a path whose name starts with -, write -- before it\n\n%s", a, usage)
			return 2
		}
	}
	inputs, ioError := collect(args, stderr)
	var all []validate.Diagnostic
	open := 0
	for _, in := range inputs {
		switch {
		case in.root != "":
			s := spec.Load(in.root)
			all = append(all, validate.CheckSpec(s)...)
			must, should, could := validate.Questions(s.Root)
			open += must + should + could
		case in.implementation != "":
			data, err := os.ReadFile(in.implementation)
			if err != nil {
				fmt.Fprintf(stderr, "specarch: cannot read %s: %v\n", in.implementation, err)
				ioError = true
				continue
			}
			all = append(all, validate.CheckImplementation(in.implementation, data, spec.Load)...)
		default:
			all = append(all, validate.CheckNamed(in.other)...)
		}
	}
	validate.Sort(all)
	for _, d := range all {
		fmt.Fprintln(stdout, d.String())
	}
	errors := validate.Errors(all)
	questions := ""
	if open > 0 {
		questions = ", " + plural(open, "open question")
	}
	fmt.Fprintf(stderr, "specarch: %s checked: %s, %s%s\n", plural(len(inputs), "input"), plural(errors, "error"), plural(len(all)-errors, "warning"), questions)
	switch {
	case ioError:
		return 2
	case errors > 0:
		return 1
	}
	return 0
}

// An input is one thing to check: a specification (its root folder), an
// implementation file outside any specification, or a file given by name
// that is neither.
type input struct {
	root           string
	implementation string
	other          string
}

// collect turns the arguments into inputs: a folder is searched for
// specifications and standalone implementation files; a file is taken as
// given.
func collect(args []string, stderr io.Writer) ([]input, bool) {
	var inputs []input
	ioError := false
	seen := map[string]bool{}
	add := func(key string, in input) {
		if !seen[key] {
			seen[key] = true
			inputs = append(inputs, in)
		}
	}
	for _, a := range args {
		info, err := os.Stat(a)
		if err != nil {
			fmt.Fprintf(stderr, "specarch: cannot read %s: %v\n", a, err)
			ioError = true
			continue
		}
		if !info.IsDir() {
			switch validate.KindOf(filepath.ToSlash(a)) {
			case validate.KindDesign:
				add("root:"+filepath.Clean(filepath.Dir(a)), input{root: filepath.Dir(a)})
			case validate.KindImplementation:
				add("impl:"+filepath.Clean(a), input{implementation: a})
			default:
				add("other:"+filepath.Clean(a), input{other: a})
			}
			continue
		}
		roots, impls, err := spec.Find(a)
		if err != nil {
			fmt.Fprintf(stderr, "specarch: cannot read %s: %v\n", a, err)
			ioError = true
			continue
		}
		if len(roots)+len(impls) == 0 {
			fmt.Fprintf(stderr, "specarch: %s holds no %s and no *%s file; name a folder that does\n", a, spec.RootFile, spec.ImplementationSuffix)
			ioError = true
		}
		for _, r := range roots {
			add("root:"+filepath.Clean(r), input{root: r})
		}
		for _, i := range impls {
			add("impl:"+filepath.Clean(i), input{implementation: i})
		}
	}
	return inputs, ioError
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
