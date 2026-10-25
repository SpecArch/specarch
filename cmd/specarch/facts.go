package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/SpecArch/specarch/internal/disk"
	"github.com/SpecArch/specarch/internal/facts"
)

// runFacts writes the code-facts dump of a committed folder with the
// reader of its language (ADR-116): the JavaScript reader inside this
// binary, or the Swift or Dart reader built once in the user's cache.
func runFacts(args []string, stdout, stderr io.Writer) int {
	w := disk.Real
	var rest []string
	for i, a := range args {
		if a == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if a == "--dry-run" {
			w = disk.DryRun(stdout)
			continue
		}
		if len(a) > 1 && a[0] == '-' {
			fmt.Fprintf(stderr, "specarch facts has no option %s; its one option is --dry-run\n\n%s", a, usage)
			return 2
		}
		rest = append(rest, a)
	}
	if len(rest) != 3 {
		fmt.Fprintf(stderr, "specarch facts takes a language, a folder and a dump file, and was given %d arguments\n\n%s", len(rest), usage)
		return 2
	}
	err := facts.Dump(rest[0], rest[1], rest[2], w, stdout, stderr)
	var u *facts.Usage
	var r *facts.Refusal
	switch {
	case err == nil:
		return 0
	case errors.As(err, &u):
		fmt.Fprintf(stderr, "specarch facts: %s\n", u.Message)
		return 2
	case errors.As(err, &r):
		fmt.Fprintf(stderr, "specarch facts: %s\n", r.Message)
		return 1
	}
	fmt.Fprintf(stderr, "specarch facts: %s\n", err)
	return 1
}
