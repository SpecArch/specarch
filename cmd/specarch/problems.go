package main

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/problems"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// runProblems writes the problems target: problems.txt and problems.sarif
// for each specification, valid or not (ADR-065). An invalid specification
// prints its errors, as every document does, gets its files all the same,
// and makes the status 1.
func runProblems(paths []string, out string, check bool, stdout, stderr io.Writer) int {
	inputs, ioError := collect(paths, stderr)
	if ioError {
		return 2
	}
	var plan []planned
	invalid := false
	skipped := 0
	for _, in := range inputs {
		if in.root == "" {
			name := in.implementation
			if name == "" {
				name = in.other
			}
			fmt.Fprintf(stderr, "specarch document: %s is not a specification; name the folder that holds %s\n", name, spec.RootFile)
			return 2
		}
		s := spec.Load(in.root)
		diags, covered := validate.CheckSpecCovered(s)
		if validate.Errors(diags) > 0 {
			for _, d := range diags {
				if d.Severity == validate.Error {
					fmt.Fprintln(stdout, d.String())
				}
			}
			invalid = true
		}
		l := loaded{spec: s}
		for _, impl := range s.Implementations {
			if doc := source.Parse(impl.Data); doc.Root != nil {
				l.impls = append(l.impls, generate.Implementation{Node: doc.Root, Path: impl.Path})
			}
		}
		if out == "" && !namesOutput(l, "problems") {
			fmt.Fprintln(stdout, noOutputFolder(l, "problems").String())
			skipped++
			continue
		}
		folder, status := outputFolder(l, "problems", out, stderr)
		if status != 0 {
			return status
		}
		ps := problems.Collect(s, diags, covered, folder)
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
			planned{filepath.Join(folder, problems.TextName), problems.Text(name, version, relSlash(folder, s.RootFile), ps)},
			planned{filepath.Join(folder, problems.SARIFName), problems.SARIF(ps)})
	}
	if len(plan) == 0 && skipped > 0 {
		fmt.Fprintln(stderr, "specarch: no output folder for problems in any specification given; give --out, or an implementation file whose targets name problems and its output")
		return 2
	}
	if len(plan) == 0 {
		fmt.Fprintf(stderr, "specarch document: no specification given; name a folder that holds %s\n", spec.RootFile)
		return 2
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
