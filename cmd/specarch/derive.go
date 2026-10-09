package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SpecArch/specarch/internal/validate"
)

// runDerive writes a draft test folder for every derived case of a
// specification that no test covers: 0 when it wrote or had nothing to
// write, 1 when a specification has errors or a draft's name is taken by
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
	specs, status := loadSpecs(args, "derive", stdout, stderr)
	if status != 0 {
		return status
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
