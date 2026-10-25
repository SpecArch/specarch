// Command specarch checks SpecArch specifications and makes documents and
// code from them.
//
//	specarch validate <folder or file>...
//	specarch document <target> [--out <folder>] [--check | --dry-run] [--audience external] <folder>...
//	specarch generate <target> [--out <folder>] [--check | --dry-run] [--unapproved] [--estate <file>] <folder>...
//	specarch extract <source> [--source-key <key>] [--implementation <file>] [--implementation-out <file>] [--facts <dump>] [--routes <dump>] [--language <tag>] [--vocabulary <file>] [--given-outside] [--table] [--dry-run] --out <folder> <path>...
//	specarch merge [--vocabulary <file>]... [--dry-run] --out <folder> <tree>...
//	specarch diff <old> <new>
//	specarch check [--allowlist <file>] [--since <revision>] <specification> <dump>...
//	specarch derive [--dry-run] <folder>...
//	specarch idioms [diff <idiom>] <folder>...
//	specarch model <folder or file>...
//	specarch version
//	specarch help [<topic>]
//	specarch guide [--agent]
//	specarch capabilities [--json]
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/metamodel"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// version is the program version, set at release time in the root
// package.
const version = specarch.Version

const usage = `usage:
  specarch validate <folder or file>...    check specifications and implementation files
  specarch gaps <folder>...                 list the open questions and what they hold up
  specarch document <target> [--out <folder>] [--check | --dry-run] [--audience external] <folder>...
                                            write a document from a specification
  specarch decide <question> --by <stakeholder> --answer <text> [--source <key>] [--date <date>] [--dry-run] <folder>
                                            answer an open question: write the decision record and take the question out
  specarch approve --by <stakeholder> [--date <date>] [--dry-run] <folder>...
                                            record that the documents were read and the specification is approved
  specarch generate <target> [--out <folder>] [--check | --dry-run] [--unapproved] [--estate <file>] <folder>...
                                            write code or data from an approved specification
  specarch extract <source> [--source-key <key>] [--implementation <file>] [--implementation-out <file>] [--facts <dump>] [--routes <dump>] [--language <tag>] [--vocabulary <file>] [--given-outside] [--table] [--dry-run] --out <folder> <path>...
                                            write a specification from one surface of existing code or
                                            documents; the sources are outline, database, router, documents, openapi,
                                            permissions, pages, workflows, gateway, go, swift, javascript and dart; go, swift,
                                            javascript, dart and pages read the permission checks the implementation
                                            file names, pages reads Next.js and Nuxt files' code through --facts, and
                                            permissions gives each route of the route table --routes its permission;
                                            documents reads the words of --language or --vocabulary, English by default;
                                            workflows reads a BPMN 2.0 XML file, or with --table a workflow table;
                                            gateway reads a gateway table
  specarch facts [--dry-run] <language> <folder> <dump>
                                            write the code-facts dump extract reads of a committed folder;
                                            the languages are javascript, swift and dart
  specarch merge [--vocabulary <file>]... [--dry-run] --out <folder> <tree>...
                                            merge the trees extract wrote into one specification
  specarch diff <old folder> <new folder>   list what changed between two versions and check the release
  specarch check [--allowlist <file>] [--since <revision>] <specification> <dump>...
                                            compare a specification with the route tables, permission tables,
                                            catalogue dumps and gateway tables of its code, and fail on a difference the
                                            allowlist does not name or a line of it that names none or was
                                            added since the revision
  specarch derive [--dry-run] <folder>...   write a draft test for every derived case no test covers
  specarch upgrade [--dry-run] [--to <version>] <folder or file>...
                                            move specifications and their files to the newest version of each format
  specarch idioms <folder>...               list the idioms each implementation file uses, and how
  specarch idioms diff <idiom> <folder>...  print a shipped idiom's parts beside each override's
  specarch model <folder or file>...        print the specifications as one JSON document, for a program
  specarch version                          print the program version
  specarch help [<topic>]                   explain an area or a command: the format, extract, generate, plugins...
  specarch guide [--agent]                  print the order of work, for a person or, with --agent, an agent
  specarch capabilities [--json]            list what this program and the plug-ins on PATH can do;
                                            --json is a tool listing with each command's inputs as JSON Schema

A specification is a folder holding specarch.yaml. A folder given here is
searched for specifications, for *.specarch-implementation.yaml files
outside one, and for estates (specarch-estate.yaml), which bring the
specifications of the modules they list.

Every command takes --json: one JSON object on standard output, and on
exit 2 one error object on standard error. Every command that writes
takes --dry-run: it lists what it would write and writes nothing.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// commands are the program's commands by their first word, each run with
// the arguments after it. help also answers to -h and --help.
var commands = map[string]func(args []string, stdout, stderr io.Writer) int{
	"validate":     runValidate,
	"gaps":         runGaps,
	"decide":       runDecide,
	"approve":      runApprove,
	"document":     runDocument,
	"generate":     runGenerate,
	"extract":      runExtract,
	"merge":        runMerge,
	"diff":         runDiff,
	"check":        runCheck,
	"derive":       runDerive,
	"upgrade":      runUpgrade,
	"idioms":       runIdioms,
	"facts":        runFacts,
	"model":        runModel,
	"version":      runVersionCommand,
	"help":         runHelp,
	"capabilities": runCapabilities,
	"guide":        runGuide,
}

func run(args []string, stdout, stderr io.Writer) int {
	if rest, ok := takeJSON(args); ok {
		return runJSON(rest, stdout, stderr)
	}
	return dispatch(args, stdout, stderr)
}

func dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	name := args[0]
	if name == "-h" || name == "--help" {
		name = "help"
	}
	if c, ok := commands[name]; ok {
		return c(args[1:], stdout, stderr)
	}
	if name == "--agent-info" {
		fmt.Fprintf(stderr, "specarch has no command --agent-info; what it can do, as JSON, is specarch capabilities --json\n\n%s", usage)
		return 2
	}
	fmt.Fprintf(stderr, "specarch has no command %q\n\n%s", args[0], usage)
	return 2
}

func runVersionCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintf(stderr, "specarch version takes no arguments\n\n%s", usage)
		return 2
	}
	return runVersion(stdout)
}

func runVersion(stdout io.Writer) int {
	fmt.Fprintf(stdout, "specarch %s\n", version)
	fmt.Fprintf(stdout, "specifications: meta-model %s\n", metamodel.Design.Reading())
	fmt.Fprintf(stdout, "implementation files, records, idioms and estates: %s\n", metamodel.Implementation.Reading())
	fmt.Fprintf(stdout, "vocabularies: %s\n", metamodel.Vocabulary.Reading())
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
	// Every specification is read before any is checked, so that a
	// reference into another one given in the same run resolves there.
	loaded := map[string]*spec.Spec{}
	byFolder := map[string]*spec.Spec{}
	var specs []*spec.Spec
	for _, in := range inputs {
		if in.root != "" {
			s := spec.Load(in.root)
			loaded[in.root] = s
			byFolder[filepath.Clean(in.root)] = s
			specs = append(specs, s)
		}
	}
	given := validate.NewGiven(specs)
	var all []validate.Diagnostic
	open, unresolved := 0, 0
	for _, in := range inputs {
		switch {
		case in.estate != nil:
			all = append(all, validate.CheckEstate(in.estate, byFolder)...)
		case in.root != "":
			s := loaded[in.root]
			all = append(all, validate.CheckSpec(s, given)...)
			must, should, could := validate.Questions(s.Root)
			open += must + should + could
			unresolved += len(validate.References(s, given))
		case in.implementation != "":
			data, err := os.ReadFile(in.implementation)
			if err != nil {
				fmt.Fprintf(stderr, "specarch: cannot read %s: %s\n", in.implementation, spec.PlainIOError(err))
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
		printDiagnostic(stdout, stderr, d)
	}
	errors := validate.Errors(all)
	questions := ""
	if open > 0 {
		questions = ", " + plural(open, "open question")
	}
	if unresolved > 0 {
		questions += ", " + plural(unresolved, "unresolved reference")
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
// implementation file outside any specification, an estate, or a file
// given by name that is none of them.
type input struct {
	root           string
	implementation string
	estate         *validate.Estate
	other          string
}

// collect turns the arguments into inputs: a folder is searched for
// specifications, standalone implementation files and estates; a file is
// taken as given. An estate brings the specifications of its modules, each
// once however often it is named.
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
	addEstate := func(path string) {
		if seen["estate:"+filepath.Clean(path)] {
			return
		}
		e, err := validate.LoadEstate(path)
		if err != nil {
			fmt.Fprintf(stderr, "specarch: cannot read %s: %s\n", path, spec.PlainIOError(err))
			ioError = true
			return
		}
		add("estate:"+filepath.Clean(path), input{estate: e})
		for _, r := range e.Roots() {
			add("root:"+filepath.Clean(r), input{root: r})
		}
	}
	for _, a := range args {
		info, err := os.Stat(a)
		if err != nil {
			fmt.Fprintf(stderr, "specarch: cannot read %s: %s\n", a, spec.PlainIOError(err))
			ioError = true
			continue
		}
		if !info.IsDir() {
			switch validate.KindOf(filepath.ToSlash(a)) {
			case validate.KindDesign:
				add("root:"+filepath.Clean(filepath.Dir(a)), input{root: filepath.Dir(a)})
			case validate.KindImplementation:
				add("impl:"+filepath.Clean(a), input{implementation: a})
			case validate.KindEstate:
				addEstate(a)
			default:
				add("other:"+filepath.Clean(a), input{other: a})
			}
			continue
		}
		roots, impls, estates, err := spec.Find(a)
		if err != nil {
			fmt.Fprintf(stderr, "specarch: cannot read %s: %s\n", a, spec.PlainIOError(err))
			ioError = true
			continue
		}
		if len(roots)+len(impls)+len(estates) == 0 {
			fmt.Fprintf(stderr, "specarch: %s holds no %s, no *%s file and no %s; name a folder that does\n", a, spec.RootFile, spec.ImplementationSuffix, spec.EstateFile)
			ioError = true
		}
		for _, e := range estates {
			addEstate(e)
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

// printDiagnostic prints a problem line on standard output and, for a YAML
// syntax error, the parser's own message on standard error, so that the
// problem line is the same bytes in both builds and the parser's message
// is still shown (ADR-177).
func printDiagnostic(stdout, stderr io.Writer, d validate.Diagnostic) {
	fmt.Fprintln(stdout, d.String())
	if l := d.DetailLine(); l != "" {
		fmt.Fprintln(stderr, l)
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
