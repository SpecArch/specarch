package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/problems"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// runProblems writes the problems target: problems.txt and problems.sarif
// for each specification, valid or not (ADR-065), and the marks of the
// problems into the YAML files of the specification (ADR-066), so that the
// lines the problems file gives are the lines on disk. An invalid specification
// prints its errors, as every document does, gets its files all the same,
// and makes the status 1.
func runProblems(paths []string, out string, check bool, stdout, stderr io.Writer) int {
	inputs, ioError := collect(paths, stderr)
	if ioError {
		return 2
	}
	var plan []planned
	marked := map[string]bool{} // fragments whose marks were written
	invalid := false
	skipped := 0
	// Each specification is checked with the others of the run, as
	// validate checks it, so references and the identity module's cases
	// resolve the same way.
	var all []*spec.Spec
	for _, in := range inputs {
		if in.estate == nil && in.root != "" {
			all = append(all, spec.Load(in.root))
		}
	}
	given := validate.NewGiven(all)
	for _, in := range inputs {
		if in.estate != nil {
			// An estate has no problems file of its own: what is wrong in
			// it is printed, and makes the status 1 (ADR-113).
			ds := validate.CheckEstate(in.estate, estateFolders(inputs))
			printErrors(ds, nil, stdout)
			invalid = invalid || validate.Errors(ds) > 0
			continue
		}
		if in.root == "" {
			name := in.implementation
			if name == "" {
				name = in.other
			}
			fmt.Fprintf(stderr, "specarch document: %s is not a specification; name the folder that holds %s\n", name, spec.RootFile)
			return 2
		}
		s := spec.Load(in.root)
		diags, covered := validate.CheckSpecCovered(s, given)
		invalid = invalid || validate.Errors(diags) > 0
		l := loaded{spec: s}
		for _, impl := range s.Implementations {
			if doc := source.Parse(impl.Data); doc.Root != nil {
				l.impls = append(l.impls, generate.Implementation{Node: doc.Root, Path: impl.Path})
			}
		}
		if out == "" && !namesOutput(l, "problems") {
			printErrors(diags, nil, stdout)
			fmt.Fprintln(stdout, noOutputFolder(l, "problems", "document").String())
			skipped++
			continue
		}
		folder, status := outputFolder(l, "problems", out, stderr)
		if status != 0 {
			return status
		}
		// The marks go into the fragments first, and the problems file
		// gives the lines they leave (ADR-066). A message that names a
		// line, such as a key defined twice, moves with the marks, so the
		// marks are written and the specification read again until the
		// two agree.
		ps := problems.Collect(s, diags, covered, folder)
		fragments, moved := problems.Mark(s, ps, problems.Files(s, ps))
		for round := 0; !check && changed(fragments) && round < 4; round++ {
			for _, f := range fragments {
				if f.Changed {
					if err := os.WriteFile(f.Path, []byte(f.Content), 0o644); err != nil {
						fmt.Fprintf(stderr, "specarch: cannot write %s: %v\n", f.Path, err)
						return 2
					}
					marked[f.Path] = true
				}
			}
			s = spec.Load(in.root)
			diags, covered = validate.CheckSpecCovered(s, given)
			ps = problems.Collect(s, diags, covered, folder)
			fragments, moved = problems.Mark(s, ps, problems.Files(s, ps))
		}
		for _, f := range fragments {
			if f.Changed {
				plan = append(plan, planned{path: f.Path, content: f.Content})
			}
		}
		ps = moved.Problems(ps)
		printErrors(diags, moved, stdout)
		name := relSlash(folder, s.RootFile)
		version := ""
		if s.Root != nil {
			info := source.Child(s.Root, "info")
			if title := source.Str(source.Child(info, "title")); title != "" {
				name = title
			}
			version = source.Str(source.Child(info, "version"))
		}
		plan = append(plan,
			planned{path: filepath.Join(folder, problems.TextName), content: problems.Text(name, version, relSlash(folder, s.RootFile), ps)},
			planned{path: filepath.Join(folder, problems.SARIFName), content: problems.SARIF(name, version, relSlash(folder, s.RootFile), ps)})
	}
	if len(plan) == 0 && skipped > 0 {
		fmt.Fprintln(stderr, "specarch: no output folder for problems in any specification given; give --out, or an implementation file whose targets name problems and its output")
		return 2
	}
	if len(plan) == 0 {
		fmt.Fprintf(stderr, "specarch document: no specification given; name a folder that holds %s\n", spec.RootFile)
		return 2
	}
	if len(marked) > 0 {
		fmt.Fprintf(stderr, "specarch document: marked the problems in %s\n", plural(len(marked), "file"))
	}
	var status int
	if check {
		status = checkPlan("document", plan, stdout, stderr)
	} else {
		status = writePlan("document", plan, stderr)
	}
	if status == 0 && invalid {
		fmt.Fprintln(stderr, "specarch document: the input has errors; they are listed in the problems file")
		return 1
	}
	return status
}

// changed tells whether any fragment's marks differ from the disk.
func changed(fragments []problems.Fragment) bool {
	for _, f := range fragments {
		if f.Changed {
			return true
		}
	}
	return false
}

// markWritten writes the marks of a specification's problems into the
// files of it that a command has just written (ADR-066): every YAML file
// of the tree when files is nil, as extract and merge write one, or only
// the files given, as derive writes draft tests into a specification an
// author keeps. given are the specifications of the run, which the check
// reads references and the identity module from; nil for a tree alone.
func markWritten(dir string, files []string, given validate.Given) error {
	// As in runProblems, a message that names a line moves with the
	// marks, so they are written until they agree with the problems.
	for round := 0; round < 4; round++ {
		s := spec.Load(dir)
		diags, covered := validate.CheckSpecCovered(s, given)
		ps := problems.Collect(s, diags, covered, s.Dir)
		marking := files
		if marking == nil {
			marking = problems.Files(s, ps)
		}
		fragments, _ := problems.Mark(s, ps, marking)
		if !changed(fragments) {
			return nil
		}
		for _, f := range fragments {
			if f.Changed {
				if err := os.WriteFile(f.Path, []byte(f.Content), 0o644); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// printErrors prints the errors among diags, at their lines after the
// marks when the run marks their files.
func printErrors(diags []validate.Diagnostic, moved problems.Moved, stdout io.Writer) {
	for _, d := range diags {
		if d.Severity == validate.Error {
			d.Line = moved.Line(d.File, d.Line)
			fmt.Fprintln(stdout, d.String())
		}
	}
}

// estateFolders reads the specifications among the inputs, by folder, for
// an estate's check.
func estateFolders(inputs []input) map[string]*spec.Spec {
	out := map[string]*spec.Spec{}
	for _, in := range inputs {
		if in.root != "" {
			out[filepath.Clean(in.root)] = spec.Load(in.root)
		}
	}
	return out
}
