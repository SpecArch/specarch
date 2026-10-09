package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/SpecArch/specarch/internal/extract"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// runMerge is the merge command: 0 when the merged specification was
// written, 1 when a tree cannot be merged, 2 on a usage error or a folder
// that cannot be read or written.
func runMerge(args []string, stdout, stderr io.Writer) int {
	out := ""
	var trees []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			trees = append(trees, args[i+1:]...)
			i = len(args)
		case a == "--out":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "specarch merge: --out needs a value\n\n%s", usage)
				return 2
			}
			out = args[i+1]
			i++
		case len(a) > 1 && a[0] == '-':
			fmt.Fprintf(stderr, "specarch merge has no option %s\n\n%s", a, usage)
			return 2
		default:
			trees = append(trees, a)
		}
	}
	switch {
	case out == "":
		fmt.Fprintf(stderr, "specarch merge needs --out, the folder the merged specification is written into\n\n%s", usage)
		return 2
	case len(trees) < 2:
		fmt.Fprintf(stderr, "specarch merge needs at least two trees, and was given %d\n\n%s", len(trees), usage)
		return 2
	}
	outAbs, err := filepath.Abs(out)
	if err != nil {
		fmt.Fprintf(stderr, "specarch merge: %v\n", err)
		return 2
	}
	var specs []*spec.Spec
	for _, t := range trees {
		if _, err := os.Stat(filepath.Join(t, spec.RootFile)); err != nil {
			fmt.Fprintf(stderr, "specarch merge: %s holds no %s (%s)\n", t, spec.RootFile, spec.PlainIOError(err))
			return 2
		}
		if abs, err := filepath.Abs(t); err == nil && abs == outAbs {
			fmt.Fprintf(stderr, "specarch merge: --out %s is one of the trees; write the merge into a folder of its own\n", out)
			return 2
		}
		s := spec.Load(t)
		if n := validate.Errors(validate.CheckSpec(s)); n > 0 {
			fmt.Fprintf(stderr, "specarch merge: validate reports %s in %s; each tree is checked on its own before it is merged (specarch validate %s)\n", plural(n, "error"), t, t)
			return 1
		}
		specs = append(specs, s)
	}
	res, err := extract.Merge(specs, out)
	if err != nil {
		var refusal *extract.Refusal
		if errors.As(err, &refusal) {
			fmt.Fprintf(stderr, "specarch merge: %s\n", refusal.Reason)
			return 1
		}
		fmt.Fprintf(stderr, "specarch merge: %v\n", err)
		return 2
	}
	if err := res.Tree.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch merge: cannot write %s: %v\n", out, err)
		return 2
	}
	if err := markWritten(out, nil); err != nil {
		fmt.Fprintf(stderr, "specarch merge: cannot mark the problems in %s: %v\n", out, err)
		return 2
	}
	for _, line := range res.Lines {
		fmt.Fprintln(stdout, line)
	}
	return 0
}
