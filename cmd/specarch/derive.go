package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SpecArch/specarch/internal/extract"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// runDerive writes a draft test folder for every derived case of a
// specification that no test covers: 0 when it wrote or had nothing to
// write, 1 when a specification has errors other than the missing tests
// it writes, or a draft's name is taken by
// another draft or another subject's test, 2 on a usage or read error or a
// specification that keeps its tests in the root file.
func runDerive(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(stderr, "specarch derive needs at least one folder\n\n%s", usage)
		return 2
	}
	for _, a := range args {
		if len(a) > 1 && a[0] == '-' {
			fmt.Fprintf(stderr, "specarch derive has no option %s\n\n%s", a, usage)
			return 2
		}
	}
	// The tests a 0.2 file is missing are errors, and they are what derive
	// writes, so they do not stop it; any other error does (ADR-055).
	specs, invalid, status := readSpecsSettling(args, "derive", stdout, stderr, validate.DeriveWrites)
	if status != 0 {
		return status
	}
	if invalid {
		fmt.Fprintln(stderr, "specarch derive: the input has errors other than the missing tests derive writes, so nothing was produced")
		return 1
	}
	for _, l := range specs {
		if !l.spec.Listed("tests") {
			fmt.Fprintf(stderr, "specarch derive: %s keeps its tests in %s; list tests in stages and give it a tests/ folder, where each test is a folder of its own\n", l.spec.Dir, filepath.Base(l.spec.RootFile))
			return 2
		}
	}
	written, collisions := 0, 0
	for _, l := range specs {
		var drafts []string
		for _, dr := range validate.Drafts(l.spec.Root) {
			folder := filepath.Join(l.spec.Dir, "tests", dr.Name)
			if len(dr.BlockedBy) > 0 {
				fmt.Fprintf(stderr, "specarch derive: left out %s for %s, which %s holds up\n", dr.Name, dr.Subject, strings.Join(dr.BlockedBy, ", "))
				continue
			}
			if dr.Collision != "" {
				fmt.Fprintf(stderr, "specarch derive: error: did not write %s for %s: %s; write this test by hand under a name of its own\n", filepath.ToSlash(folder), dr.Subject, dr.Collision)
				collisions++
				continue
			}
			if _, err := os.Stat(folder); err == nil {
				fmt.Fprintf(stderr, "specarch derive: kept %s, which exists\n", filepath.ToSlash(folder))
				continue
			}
			if err := os.MkdirAll(folder, 0o755); err != nil {
				fmt.Fprintf(stderr, "specarch derive: cannot write %s: %v\n", folder, err)
				return 2
			}
			file := filepath.Join(folder, "test.yaml")
			if err := os.WriteFile(file, []byte(dr.Text), 0o644); err != nil {
				fmt.Fprintf(stderr, "specarch derive: cannot write %s: %v\n", file, err)
				return 2
			}
			fmt.Fprintln(stdout, filepath.ToSlash(file))
			if dr.Renamed != "" {
				fmt.Fprintf(stderr, "specarch derive: question: %s for %s: %s; rename the subject if the suffix does not suit\n", filepath.ToSlash(folder), dr.Subject, dr.Renamed)
			}
			drafts = append(drafts, file)
			written++
		}
		if len(drafts) > 0 {
			if err := markWritten(l.spec.Dir, drafts); err != nil {
				fmt.Fprintf(stderr, "specarch derive: cannot mark the problems in the drafts of %s: %v\n", l.spec.Dir, err)
				return 2
			}
		}
	}
	fmt.Fprintf(stderr, "specarch derive: %s written\n", plural(written, "draft test"))
	if collisions > 0 {
		return 1
	}
	return 0
}

// deriveInto runs the derivation into a tree extract or merge has just
// written, so that its output carries the tests derive writes (ADR-055):
// each draft that is not held up and whose name is free goes under
// tests/, and tests is listed among the stages when one was written. It
// returns the lines that say what it wrote and left out.
func deriveInto(out string, tree *extract.Tree) ([]string, error) {
	s := spec.Load(out)
	var write []validate.Draft
	var lines []string
	held := map[string][]string{} // subject to the questions holding it up
	heldDrafts := map[string]int{}
	var heldOrder []string
	for _, dr := range validate.Drafts(s.Root) {
		switch {
		case len(dr.BlockedBy) > 0:
			if held[dr.Subject] == nil {
				heldOrder = append(heldOrder, dr.Subject)
			}
			held[dr.Subject] = dr.BlockedBy
			heldDrafts[dr.Subject]++
		case dr.Collision != "":
			lines = append(lines, fmt.Sprintf("not derived: test %s for %s: %s; write this test by hand under a name of its own", dr.Name, dr.Subject, dr.Collision))
		default:
			write = append(write, dr)
			if dr.Renamed != "" {
				lines = append(lines, fmt.Sprintf("question: test %s for %s: %s; rename the subject if the suffix does not suit", dr.Name, dr.Subject, dr.Renamed))
			}
		}
	}
	if len(write) > 0 {
		tree.ListStage("tests")
		if err := tree.Write(out); err != nil {
			return nil, err
		}
		for _, dr := range write {
			folder := filepath.Join(out, "tests", dr.Name)
			if err := os.MkdirAll(folder, 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(filepath.Join(folder, "test.yaml"), []byte(dr.Text), 0o644); err != nil {
				return nil, err
			}
		}
	}
	for _, subject := range heldOrder {
		lines = append(lines, fmt.Sprintf("not derived: %s for %s, held up by %s; the validator warns for them until the %s answered", plural(heldDrafts[subject], "draft test"), subject, strings.Join(held[subject], ", "), map[bool]string{true: "question is", false: "questions are"}[len(held[subject]) == 1]))
	}
	head := fmt.Sprintf("derived %s under tests, as specarch derive writes them: the success of each subject the design gives one, and each chosen case no test covers", plural(len(write), "draft test"))
	return append([]string{head}, lines...), nil
}
