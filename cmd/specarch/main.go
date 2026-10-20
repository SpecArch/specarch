// Command specarch checks SpecArch specifications and makes documents and
// code from them.
//
//	specarch validate <folder or file>...
//	specarch document <target> [--out <folder>] [--check] [--audience external] <folder>...
//	specarch generate <target> [--out <folder>] [--check] <folder>...
//	specarch extract <source> [--source-key <key>] [--implementation <file>] [--implementation-out <file>] [--facts <dump>] [--routes <dump>] [--language <tag>] [--vocabulary <file>] [--given-outside] [--table] --out <folder> <path>...
//	specarch merge [--vocabulary <file>]... --out <folder> <tree>...
//	specarch diff <old> <new>
//	specarch check [--allowlist <file>] [--since <revision>] <specification> <dump>...
//	specarch derive <folder>...
//	specarch idioms [diff <idiom>] <folder>...
//	specarch model <folder or file>...
//	specarch version
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
	"github.com/SpecArch/specarch/schema"
)

// version is the program version, set at release time.
const version = "0.7.3"

const usage = `usage:
  specarch validate <folder or file>...    check specifications and implementation files
  specarch gaps <folder>...                 list the open questions and what they hold up
  specarch document <target> [--out <folder>] [--check] [--audience external] <folder>...
                                            write a document from a specification
  specarch decide <question> --by <stakeholder> --answer <text> [--source <key>] [--date <date>] <folder>
                                            answer an open question: write the decision record and take the question out
  specarch approve --by <stakeholder> [--date <date>] <folder>...
                                            record that the documents were read and the specification is approved
  specarch generate <target> [--out <folder>] [--check] [--unapproved] <folder>...
                                            write code or data from an approved specification
  specarch extract <source> [--source-key <key>] [--implementation <file>] [--implementation-out <file>] [--facts <dump>] [--routes <dump>] [--language <tag>] [--vocabulary <file>] [--given-outside] [--table] --out <folder> <path>...
                                            write a specification from one surface of existing code or
                                            documents; the sources are outline, database, router, documents, openapi,
                                            permissions, pages, workflows, gateway, go, swift, javascript and dart; go, swift,
                                            javascript, dart and pages read the permission checks the implementation
                                            file names, pages reads Next.js and Nuxt files' code through --facts, and
                                            permissions gives each route of the route table --routes its permission;
                                            documents reads the words of --language or --vocabulary, English by default;
                                            workflows reads a BPMN 2.0 XML file, or with --table a workflow table;
                                            gateway reads a gateway table
  specarch facts <language> <folder> <dump>
                                            write the code-facts dump extract reads of a committed folder;
                                            the languages are javascript, swift and dart
  specarch merge [--vocabulary <file>]... --out <folder> <tree>...
                                            merge the trees extract wrote into one specification
  specarch diff <old folder> <new folder>   list what changed between two versions and check the release
  specarch check [--allowlist <file>] [--since <revision>] <specification> <dump>...
                                            compare a specification with the route tables, permission tables,
                                            catalogue dumps and gateway tables of its code, and fail on a difference the
                                            allowlist does not name or a line of it that names none or was
                                            added since the revision
  specarch derive <folder>...               write a draft test for every derived case no test covers
  specarch idioms <folder>...               list the idioms each implementation file uses, and how
  specarch idioms diff <idiom> <folder>...  print a shipped idiom's parts beside each override's
  specarch model <folder or file>...        print the specifications as one JSON document, for a program
  specarch version                          print the program version

A specification is a folder holding specarch.yaml. A folder given here is
searched for specifications, for *.specarch-implementation.yaml files
outside one, and for estates (specarch-estate.yaml), which bring the
specifications of the modules they list.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "gaps":
		return runGaps(args[1:], stdout, stderr)
	case "decide":
		return runDecide(args[1:], stdout, stderr)
	case "approve":
		return runApprove(args[1:], stdout, stderr)
	case "document":
		return runDocument(args[1:], stdout, stderr)
	case "generate":
		return runGenerate(args[1:], stdout, stderr)
	case "extract":
		return runExtract(args[1:], stdout, stderr)
	case "merge":
		return runMerge(args[1:], stdout, stderr)
	case "diff":
		return runDiff(args[1:], stdout, stderr)
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "derive":
		return runDerive(args[1:], stdout, stderr)
	case "idioms":
		return runIdioms(args[1:], stdout, stderr)
	case "facts":
		return runFacts(args[1:], stdout, stderr)
	case "model":
		return runModel(args[1:], stdout, stderr)
	case "version":
		if len(args) > 1 {
			fmt.Fprintf(stderr, "specarch version takes no arguments\n\n%s", usage)
			return 2
		}
		return runVersion(stdout)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	}
	fmt.Fprintf(stderr, "specarch has no command %q\n\n%s", args[0], usage)
	return 2
}

func runVersion(stdout io.Writer) int {
	fmt.Fprintf(stdout, "specarch %s\n", version)
	fmt.Fprintf(stdout, "specifications: %s\n", metaModelVersions(schema.Definition, "specarch"))
	fmt.Fprintf(stdout, "implementation files: %s\n", metaModelVersions(schema.Implementation, "specarchImplementation"))
	return 0
}

// metaModelVersions names the versions an embedded schema's version key
// allows: the last is the current one, the earlier ones are read until 1.0.
// The schemas are the one source both builds share, so the line follows the
// meta-model the validator reads.
func metaModelVersions(schemaJSON []byte, key string) string {
	var document struct {
		Properties map[string]struct {
			Const string   `json:"const"`
			Enum  []string `json:"enum"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schemaJSON, &document); err != nil {
		panic(fmt.Sprintf("the embedded schema is not JSON: %v", err))
	}
	property := document.Properties[key]
	versions := property.Enum
	if property.Const != "" {
		versions = []string{property.Const}
	}
	if len(versions) == 0 {
		panic(fmt.Sprintf("the embedded schema names no version under %s", key))
	}
	current := versions[len(versions)-1]
	if len(versions) == 1 {
		return "meta-model " + current
	}
	earlier := make([]string, 0, len(versions)-1)
	for i := len(versions) - 2; i >= 0; i-- {
		earlier = append(earlier, versions[i])
	}
	list := earlier[len(earlier)-1]
	if len(earlier) > 1 {
		list = strings.Join(earlier[:len(earlier)-1], ", ") + " and " + list
	}
	return fmt.Sprintf("meta-model %s, and %s until 1.0, with a warning", current, list)
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
		fmt.Fprintln(stdout, d.String())
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

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
