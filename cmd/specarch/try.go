package main

import (
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// runTry evaluates an algorithm on its worked examples or on given inputs
// and prints each step (ADR-195). It works on a specification with errors
// too, after printing them (ADR-065), and exits 1 then.
func runTry(args []string, stdout, stderr io.Writer) int {
	return tryInto(args, stdout, stderr, nil)
}

// tryInto is runTry that also keeps what it found, for --json.
func tryInto(args []string, stdout, stderr io.Writer, found **validate.Tried) int {
	example, inputs, file := "", "", ""
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, value := "", ""
		switch {
		case a == "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
			continue
		case a == "--example" || a == "--inputs" || a == "--file":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "specarch try: %s needs a value\n\n%s", a, usage)
				return 2
			}
			name, value = a, args[i+1]
			i++
		case strings.HasPrefix(a, "--example=") || strings.HasPrefix(a, "--inputs=") || strings.HasPrefix(a, "--file="):
			name, value, _ = strings.Cut(a, "=")
		case strings.HasPrefix(a, "-") && len(a) > 1:
			fmt.Fprintf(stderr, "specarch try has no option %s; its options are --example, --inputs and --file\n\n%s", a, usage)
			return 2
		default:
			rest = append(rest, a)
			continue
		}
		switch name {
		case "--example":
			example = value
		case "--inputs":
			inputs = value
		case "--file":
			file = value
		}
	}
	switch {
	case len(rest) != 2:
		fmt.Fprintf(stderr, "specarch try needs the algorithm's name and one folder, in that order\n\n%s", usage)
		return 2
	case example != "" && inputs != "":
		fmt.Fprintf(stderr, "specarch try takes --example or --inputs, not both: --example evaluates a worked example, --inputs the inputs given\n\n%s", usage)
		return 2
	}
	var given *yaml.Node
	if inputs != "" {
		doc := source.Parse([]byte(inputs))
		if doc.Root == nil || doc.Root.Kind != yaml.MappingNode || len(doc.Problems) > 0 {
			fmt.Fprintf(stderr, "specarch try: --inputs is %q, which is not a YAML mapping; write the inputs as an example's inputs are, such as '{ daysLate: 7, dailyRate: \"0.50\" }'\n\n%s", inputs, usage)
			return 2
		}
		given = doc.Root
	}
	algorithm, folder := rest[0], rest[1]
	ins, ioError := collect([]string{folder}, stderr)
	if ioError {
		return 2
	}
	var roots []string
	for _, in := range ins {
		if in.root != "" {
			roots = append(roots, in.root)
		}
	}
	if len(roots) != 1 {
		fmt.Fprintf(stderr, "specarch try: %s holds %d specifications; name the folder of the one the algorithm is in\n", folder, len(roots))
		return 2
	}
	s := spec.Load(roots[0])
	if file != "" {
		reason, readError := s.Overlay(file, "algorithms", algorithm)
		if readError {
			fmt.Fprintf(stderr, "specarch try: %s\n", reason)
			return 2
		}
		if reason != "" {
			fmt.Fprintf(stderr, "specarch try: %s\n", reason)
			return 1
		}
	}
	invalid := false
	for _, d := range validate.CheckSpec(s, validate.NewGiven([]*spec.Spec{s})) {
		if d.Severity == validate.Error && !validate.TriedHere(d, algorithm) {
			printDiagnostic(stdout, stderr, d)
			invalid = true
		}
	}
	t, refusal := validate.Try(s, algorithm, example, given)
	if refusal != "" {
		fmt.Fprintf(stderr, "specarch try: %s\n", refusal)
		return 1
	}
	if found != nil {
		*found = t
	}
	failed := printTried(stdout, t, given != nil)
	if invalid || failed {
		return 1
	}
	return 0
}

// printTried prints a summary line, then each evaluation with its inputs,
// its steps and its result, and tells whether any did not pass or compute.
func printTried(w io.Writer, t *validate.Tried, givenInputs bool) bool {
	counts := map[string]int{}
	for _, e := range t.Evaluations {
		counts[e.Outcome]++
	}
	if givenInputs {
		fmt.Fprintf(w, "%s: given inputs, %s\n", t.Algorithm, t.Evaluations[0].Outcome)
	} else {
		summary := fmt.Sprintf("%s: %s, %d pass", t.Algorithm, plural(len(t.Evaluations), "example"), counts[validate.OutcomePass])
		for _, o := range []string{validate.OutcomeFail, validate.OutcomeError, validate.OutcomeUnread} {
			if counts[o] > 0 {
				summary += fmt.Sprintf(", %d %s", counts[o], o)
			}
		}
		fmt.Fprintln(w, summary)
	}
	failed := false
	for _, e := range t.Evaluations {
		switch {
		case e.Number == 0:
			fmt.Fprintf(w, "given inputs: %s\n", e.Outcome)
		case e.Name == "":
			fmt.Fprintf(w, "example %d: %s\n", e.Number, e.Outcome)
		default:
			fmt.Fprintf(w, "example %d %s: %s\n", e.Number, quoted(e.Name), e.Outcome)
		}
		if e.Outcome != validate.OutcomePass && e.Outcome != validate.OutcomeComputed {
			failed = true
		}
		if len(e.Inputs) > 0 {
			var parts []string
			for _, in := range e.Inputs {
				parts = append(parts, in.Name+" "+in.Value)
			}
			fmt.Fprintf(w, "  inputs: %s\n", strings.Join(parts, ", "))
		}
		for _, u := range e.Unread {
			fmt.Fprintf(w, "  cannot be read: %s\n", u)
		}
		for _, st := range e.Steps {
			fmt.Fprintf(w, "  %s = %s\n", st.Expression, st.Value)
		}
		switch {
		case e.Outcome == validate.OutcomeError:
			fmt.Fprintf(w, "  cannot be computed: %s\n", e.Error)
		case e.Result != nil && e.Expected != nil:
			fmt.Fprintf(w, "  result %s, expected %s\n", *e.Result, *e.Expected)
		case e.Result != nil:
			fmt.Fprintf(w, "  result %s\n", *e.Result)
		}
		if e.Outcome == validate.OutcomeFail && e.Error != "" {
			fmt.Fprintf(w, "  %s\n", e.Error)
		}
	}
	return failed
}

// quoted writes a name in double quotes, as both builds do.
func quoted(s string) string {
	return `"` + strings.NewReplacer(`"`, `\"`, `\`, `\\`, "\n", `\n`, "\t", `\t`).Replace(s) + `"`
}
