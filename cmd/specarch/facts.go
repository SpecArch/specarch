package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/SpecArch/specarch/internal/facts"
)

// runFacts writes the code-facts dump of a committed folder with the
// reader of its language (ADR-116): the JavaScript reader inside this
// binary, or the Swift or Dart reader built once in the user's cache.
func runFacts(args []string, stdout, stderr io.Writer) int {
	if len(args) != 3 {
		fmt.Fprintf(stderr, "specarch facts takes a language, a folder and a dump file, and was given %d arguments\n\n%s", len(args), usage)
		return 2
	}
	err := facts.Dump(args[0], args[1], args[2], stdout, stderr)
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
