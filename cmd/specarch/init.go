package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/extract"
	"github.com/SpecArch/specarch/internal/metamodel"
)

// runInit writes the smallest specification that validates into a folder
// that holds none (ADR-210): the schema line, the meta-model version and
// the info the format requires, with the profile when one is given, and
// the intent given with --problem, --goal and --out-of-scope, or asked on
// a terminal when none is given (ADR-244).
func runInit(args []string, stdout, stderr io.Writer) int {
	title, profile, dryRun := "", "", false
	var in intent
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, value := "", ""
		switch {
		case a == "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
			continue
		case a == "--dry-run":
			dryRun = true
			continue
		case a == "--title" || a == "--profile" || a == "--problem" || a == "--goal" || a == "--out-of-scope":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "specarch init: %s needs a value\n\n%s", a, usage)
				return 2
			}
			name, value = a, args[i+1]
			i++
		case strings.HasPrefix(a, "--title=") || strings.HasPrefix(a, "--profile=") || strings.HasPrefix(a, "--problem=") || strings.HasPrefix(a, "--goal=") || strings.HasPrefix(a, "--out-of-scope="):
			name, value, _ = strings.Cut(a, "=")
		case strings.HasPrefix(a, "-") && len(a) > 1:
			fmt.Fprintf(stderr, "specarch init has no option %s; its options are --title, --profile, --problem, --goal, --out-of-scope and --dry-run\n\n%s", a, usage)
			return 2
		default:
			rest = append(rest, a)
			continue
		}
		switch name {
		case "--title":
			if strings.TrimSpace(value) == "" {
				fmt.Fprintf(stderr, "specarch init: --title is empty; give the specification's title, or leave --title out to take the folder's name\n\n%s", usage)
				return 2
			}
			title = value
		case "--profile":
			if value != "lean" && value != "full" {
				fmt.Fprintf(stderr, "specarch init: --profile is %q; the profiles are lean and full\n\n%s", value, usage)
				return 2
			}
			profile = value
		case "--problem", "--goal", "--out-of-scope":
			if strings.TrimSpace(value) == "" {
				fmt.Fprintf(stderr, "specarch init: %s is empty; give the answer as a plain sentence, or leave %s out\n\n%s", name, name, usage)
				return 2
			}
			switch name {
			case "--problem":
				if in.problem != "" {
					fmt.Fprintf(stderr, "specarch init: --problem is given twice; a specification has one problem, so give it once\n\n%s", usage)
					return 2
				}
				in.problem = strings.TrimSpace(value)
			case "--goal":
				in.goals = append(in.goals, strings.TrimSpace(value))
			default:
				in.outOfScope = append(in.outOfScope, strings.TrimSpace(value))
			}
		}
	}
	if len(rest) != 1 {
		fmt.Fprintf(stderr, "specarch init needs one folder, the one the specification goes in\n\n%s", usage)
		return 2
	}
	folder := rest[0]
	path := filepath.Join(folder, "specarch.yaml")
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(stderr, "specarch init: %s already exists; init never overwrites a specification, so give a folder without specarch.yaml, or edit that file\n", path)
		return 2
	} else if !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(stderr, "specarch init: cannot read %s to see whether a specification is there: %v\n", path, err)
		return 2
	}
	if title == "" {
		abs, err := filepath.Abs(folder)
		if err != nil {
			fmt.Fprintf(stderr, "specarch init: cannot read the name of the folder %s to take it as the title: %v; give the title with --title\n", folder, err)
			return 2
		}
		title = filepath.Base(abs)
	}
	if in.empty() && terminal(standardInput) {
		in = askIntent(standardInput, stderr)
	}
	text := initText(title, profile) + in.text()
	if dryRun {
		fmt.Fprintf(stdout, "would write %s\n%s", path, text)
		fmt.Fprintf(stderr, "specarch init: nothing written (--dry-run)\n")
		return 0
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		fmt.Fprintf(stderr, "specarch init: cannot write the folder %s: %v\n", folder, err)
		return 2
	}
	// O_EXCL: a file made between the check above and now is not overwritten.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		fmt.Fprintf(stderr, "specarch init: cannot write %s: %v\n", path, err)
		return 2
	}
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		fmt.Fprintf(stderr, "specarch init: cannot write %s: %v\n", path, err)
		return 2
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(stderr, "specarch init: cannot write %s: %v\n", path, err)
		return 2
	}
	fmt.Fprintln(stdout, path)
	fmt.Fprintf(stderr, "specarch init: next, run specarch validate %s after each edit; specarch help start shows a first entity and operation that validate\n", folder)
	return 0
}

// initText is the smallest specification that validates, with the title
// written as a YAML scalar and the profile when one is given.
func initText(title, profile string) string {
	var b strings.Builder
	b.WriteString(extract.SchemaLine)
	fmt.Fprintf(&b, "specarch: %q\n\n", metamodel.Current)
	b.WriteString("info:\n")
	fmt.Fprintf(&b, "  title: %s\n", yamlScalar(title))
	b.WriteString("  version: 0.1.0\n")
	if profile != "" {
		fmt.Fprintf(&b, "  profile: %s\n", profile)
	}
	return b.String()
}

// yamlScalar writes a string as one YAML scalar on one line: plain when
// YAML reads it back as the same string, quoted otherwise.
func yamlScalar(s string) string {
	var n yaml.Node
	n.SetString(s)
	out, err := yaml.Marshal(&n)
	if err != nil || strings.Count(strings.TrimRight(string(out), "\n"), "\n") > 0 {
		return fmt.Sprintf("%q", s)
	}
	return strings.TrimRight(string(out), "\n")
}

// intent is what init writes of the intent (ADR-244): the problem, the
// goals and what is out of scope, each a plain sentence.
type intent struct {
	problem    string
	goals      []string
	outOfScope []string
}

func (in intent) empty() bool {
	return in.problem == "" && len(in.goals) == 0 && len(in.outOfScope) == 0
}

// text writes the intent as YAML: problem.statement, GOAL-n with status
// proposed, since nobody has accepted a goal at init, and OUT-n.
func (in intent) text() string {
	if in.empty() {
		return ""
	}
	var b strings.Builder
	if in.problem != "" {
		fmt.Fprintf(&b, "\nproblem:\n  statement: %s\n", yamlScalar(in.problem))
	}
	if len(in.goals) > 0 {
		b.WriteString("\ngoals:\n")
		for i, g := range in.goals {
			fmt.Fprintf(&b, "  GOAL-%d:\n    statement: %s\n    status: proposed\n", i+1, yamlScalar(g))
		}
	}
	if len(in.outOfScope) > 0 {
		b.WriteString("\noutOfScope:\n")
		for i, o := range in.outOfScope {
			fmt.Fprintf(&b, "  OUT-%d:\n    statement: %s\n", i+1, yamlScalar(o))
		}
	}
	return b.String()
}

// terminal reports whether standard input is a terminal, where a person
// can be asked; a pipe, a file, the null device or a test's input is not.
// The null device is a character device as a terminal is, so it is told
// apart by being the same file as os.DevNull.
func terminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if null, err := os.Stat(os.DevNull); err == nil && os.SameFile(fi, null) {
		return false
	}
	return true
}

// askIntent asks the three questions of the intent on standard error and
// reads each answer as a line: one problem, then goals and out-of-scope
// entries until an empty answer. An empty answer skips a question.
func askIntent(r io.Reader, stderr io.Writer) intent {
	in := bufio.NewReader(r)
	read := func() (string, bool) {
		line, err := in.ReadString('\n')
		line = strings.TrimSpace(line)
		return line, err == nil || line != ""
	}
	var out intent
	fmt.Fprintln(stderr, "specarch init asks three questions; answer each in a sentence, or press Enter to skip it.")
	fmt.Fprint(stderr, "What problem does the system solve: whom does it hurt, and what does it cost them today?\n> ")
	if line, ok := read(); ok {
		out.problem = line
	}
	fmt.Fprint(stderr, "What outcome counts as success? One goal a line; an empty line ends the goals.\n> ")
	for {
		line, ok := read()
		if !ok || line == "" {
			break
		}
		out.goals = append(out.goals, line)
		fmt.Fprint(stderr, "> ")
	}
	fmt.Fprint(stderr, "What will the system not do, decided now? One a line; an empty line ends them.\n> ")
	for {
		line, ok := read()
		if !ok || line == "" {
			break
		}
		out.outOfScope = append(out.outOfScope, line)
		fmt.Fprint(stderr, "> ")
	}
	return out
}
