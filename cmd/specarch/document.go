package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/approval"
	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// documentTargets are the document targets of the design; builtDocuments
// says which this program has.
var (
	documentTargets = generate.DocumentTargets
	builtDocuments  = generate.BuiltDocuments
)

// planned is one file a target wants on disk.
type planned struct {
	path    string
	content string
}

// loaded is one validated specification with its implementation files
// parsed, and the diagnostics its open questions cover.
type loaded struct {
	spec    *spec.Spec
	impls   []generate.Implementation
	diags   []validate.Diagnostic // the errors and warnings validate keeps
	covered []validate.Diagnostic
}

// marks are the errors and warnings of the specification as the documents
// in folder mark them: files named from there, and the pointer kept only
// for a problem in a fragment of the merged specification, which is any
// file in the specification's folder but an implementation file, so that
// a fragment that does not parse is one too.
func (l loaded) marks(folder string) []generate.Mark {
	implementations := map[string]bool{}
	for _, i := range l.spec.Implementations {
		implementations[filepath.Clean(i.Path)] = true
	}
	dir := filepath.Clean(l.spec.Dir) + string(filepath.Separator)
	var out []generate.Mark
	for _, d := range l.diags {
		m := generate.Mark{Severity: string(d.Severity), File: relSlash(folder, d.File), Line: d.Line, Column: d.Column, Rule: string(d.Rule), Message: d.Message, ID: d.ID}
		if file := filepath.Clean(d.File); strings.HasPrefix(file, dir) && !implementations[file] {
			m.Pointer = d.Path
		}
		out = append(out, m)
	}
	return out
}

// state gathers what the documents need beyond the
// specification: the keys the open questions cover, where the approval
// of this version stands, and the derived cases the test plan lists as
// left out.
func (l loaded) state() *generate.State {
	st := &generate.State{Missing: map[string][]string{}}
	for _, d := range l.covered {
		if d.Rule == validate.RuleSchema && strings.HasSuffix(d.Message, " is missing; add it here") {
			st.Missing[d.Path] = append(st.Missing[d.Path], strings.TrimSuffix(d.Message, " is missing; add it here"))
		}
	}
	version := source.Str(source.Child(source.Child(l.spec.Root, "info"), "version"))
	st.Approved, st.Approval = approval.State(l.spec.Dir, version)
	for _, r := range l.spec.Records {
		if doc := source.Parse(r.Data); doc.Root != nil {
			st.Records = append(st.Records, doc.Root)
		}
	}
	st.StatePaths = validate.StatePaths(l.spec.Root)
	st.Places = l.namedPlaces()
	for _, c := range validate.LeftOutCases(l.spec.Root) {
		st.LeftOut = append(st.LeftOut, generate.LeftOut{Subject: c.Subject, Case: c.Case, Scenario: c.Scenario, Reason: c.Reason})
	}
	return st
}

// namedPlaces finds, for each question that names what the source gives,
// the file and line of the element whose key it leaves out, from the
// specification's root.
func (l loaded) namedPlaces() map[string]string {
	places := map[string]string{}
	for _, q := range source.Pairs(source.Child(l.spec.Root, "questions")) {
		blocks := source.Items(source.Child(q.Value, "blocks"))
		if source.Child(q.Value, "names") == nil || len(blocks) != 1 {
			continue
		}
		b, ok := spec.ParseBlock(blocks[0].Value)
		if !ok || b.Key() == "" {
			continue
		}
		// The element is the key's parent; its key node, or its item in a
		// list, is where it starts.
		var at *yaml.Node
		n := l.spec.Root
		for _, t := range b.Tokens[:len(b.Tokens)-1] {
			parent := source.Deref(n)
			n = source.Child(parent, t)
			if parent != nil && parent.Kind == yaml.MappingNode {
				at = source.Key(parent, t)
			} else {
				at = n
			}
		}
		if at == nil || n == nil {
			continue
		}
		file, err := filepath.Rel(l.spec.Dir, l.spec.Files[at])
		if err != nil || l.spec.Files[at] == "" {
			continue
		}
		places[blocks[0].Value] = fmt.Sprintf("%s:%d", filepath.ToSlash(file), at.Line)
	}
	return places
}

// runDocument implements the document command and its checkStatus
// algorithm: 2 for a usage or read error, 1 for invalid input, a marker
// error or, with --check, a difference, 0 otherwise.
func runDocument(args []string, stdout, stderr io.Writer) int {
	target, out, check, paths, _, msg := parseTargetArgs("document", documentTargets, nil, args)
	if msg != "" {
		fmt.Fprintf(stderr, "%s\n\n%s", msg, usage)
		return 2
	}
	if !builtDocuments[target] {
		var built []string
		for _, t := range documentTargets {
			if builtDocuments[t] {
				built = append(built, t)
			}
		}
		fmt.Fprintf(stderr, "specarch document: this build has no %s documentor; it has %s\n", target, strings.Join(built, ", "))
		return 2
	}
	if target == "problems" {
		return runProblems(paths, out, check, stdout, stderr)
	}
	specs, invalid, status := readSpecs(paths, "document", stdout, stderr)
	if status != 0 {
		return status
	}
	var plan []planned
	failed := false
	skipped := 0
	for _, l := range specs {
		if out == "" && !namesOutput(l, target) {
			fmt.Fprintln(stdout, noOutputFolder(l, target, "document").String())
			skipped++
			continue
		}
		folder, status := outputFolder(l, target, out, stderr)
		if status != 0 {
			return status
		}
		var impls []generate.Implementation
		for _, i := range l.impls {
			impls = append(impls, generate.Implementation{Node: i.Node, Rel: relSlash(folder, i.Path), Path: i.Path, Idioms: i.Idioms})
		}
		st := l.state()
		st.RecordsRel = relSlash(folder, l.spec.RecordsDir)
		st.Marks = l.marks(folder)
		text, _ := generate.Document(target, l.spec.Root, relSlash(folder, l.spec.RootFile), impls, st)
		plan = append(plan, planned{filepath.Join(folder, generate.DocumentName(target)), text})
		if target != "techspec" {
			continue
		}

		companion := filepath.Join(l.spec.Dir, generate.CompanionName)
		if text, err := os.ReadFile(companion); err == nil {
			rewritten, errs := generate.RewriteMarkers(string(text), l.spec.Root)
			for _, e := range errs {
				fmt.Fprintf(stdout, "%s:%d: error: %s\n", companion, e.Line, e.Message)
				failed = true
			}
			plan = append(plan, planned{companion, rewritten})
		} else if !os.IsNotExist(err) {
			fmt.Fprintf(stderr, "specarch: cannot read %s: %v\n", companion, err)
			return 2
		}
	}
	if failed {
		fmt.Fprintln(stderr, "specarch document: the markers have errors, so nothing was written")
		return 1
	}
	if skipped == len(specs) {
		fmt.Fprintf(stderr, "specarch: no output folder for %s in any specification given; give --out, or an implementation file whose targets name %s and its output\n", target, target)
		return 2
	}
	if check {
		status = checkPlan("document", plan, stdout, stderr)
	} else {
		status = writePlan("document", plan, stderr)
	}
	if status == 0 && invalid {
		fmt.Fprintln(stderr, "specarch document: the input has errors; they are marked in the documents and listed in the problems file")
		return 1
	}
	return status
}

// loadSpecs reads and validates every specification under paths. An
// invalid one prints its errors; nothing is produced then.
func loadSpecs(paths []string, verb string, stdout, stderr io.Writer) ([]loaded, int) {
	specs, invalid, status := readSpecs(paths, verb, stdout, stderr)
	if status != 0 {
		return nil, status
	}
	if invalid {
		fmt.Fprintf(stderr, "specarch %s: the input has errors, so nothing was produced\n", verb)
		return nil, 1
	}
	return specs, 0
}

// readSpecs reads and validates every specification under paths, and keeps
// an invalid one too, with its errors printed, for the commands that work
// on what can be read (ADR-065). invalid says whether one had errors.
func readSpecs(paths []string, verb string, stdout, stderr io.Writer) (specs []loaded, invalid bool, status int) {
	inputs, ioError := collect(paths, stderr)
	if ioError {
		return nil, false, 2
	}
	for _, in := range inputs {
		if in.root == "" {
			name := in.implementation
			if name == "" {
				name = in.other
			}
			fmt.Fprintf(stderr, "specarch %s: %s is not a specification; name the folder that holds %s\n", verb, name, spec.RootFile)
			return nil, false, 2
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
		l := loaded{spec: s, diags: diags, covered: covered}
		for _, impl := range s.Implementations {
			doc := source.Parse(impl.Data)
			if doc.Root == nil {
				continue
			}
			l.impls = append(l.impls, generate.Implementation{Node: doc.Root, Path: impl.Path, Idioms: idiomUses(impl.Path, doc.Root, s)})
		}
		specs = append(specs, l)
	}
	if len(specs) == 0 {
		fmt.Fprintf(stderr, "specarch %s: no specification given; name a folder that holds %s\n", verb, spec.RootFile)
		return nil, false, 2
	}
	return specs, invalid, 0
}

// namesOutput says whether an implementation file of the specification
// names an output folder for the target under targets.
func namesOutput(l loaded, target string) bool {
	for _, i := range l.impls {
		if source.Str(source.Child(source.Child(source.Child(i.Node, "targets"), target), "output")) != "" {
			return true
		}
	}
	return false
}

// noOutputFolder is the warning for a specification that names no output
// folder for the target (ADR-073), at the targets of the first
// implementation file that has them, or at that file, or at the root file
// when there is none, placed and given its id as validate's are.
func noOutputFolder(l loaded, target, kind string) validate.Diagnostic {
	d := validate.Diagnostic{File: l.spec.RootFile, Line: 1, Severity: validate.Warning, Path: "/", Rule: validate.RuleOutputFolder,
		Message: fmt.Sprintf("no implementation file names an output folder for %s under targets, so this specification has no %s %s; name one, or give --out", target, target, kind)}
	if len(l.impls) > 0 {
		d.File = l.impls[0].Path
	}
	for _, i := range l.impls {
		if k := source.Key(i.Node, "targets"); k != nil {
			d.File, d.Line, d.Path = i.Path, k.Line, "/targets"
			break
		}
	}
	ds := []validate.Diagnostic{d}
	validate.SpecPlacer(l.spec).Place(ds)
	return ds[0]
}

// outputFolder is --out, or the folder the implementation files name for
// the target under targets; they must agree. document and generate pass
// over a specification that names none before it gets here (ADR-073,
// ADR-086); generate still stops with exit 2 at a plug-in whose
// implementation files name none while another plug-in's do.
func outputFolder(l loaded, target, out string, stderr io.Writer) (string, int) {
	if out != "" {
		return filepath.Clean(out), 0
	}
	folder := ""
	for _, i := range l.impls {
		o := source.Str(source.Child(source.Child(source.Child(i.Node, "targets"), target), "output"))
		if o == "" {
			continue
		}
		f := filepath.Clean(filepath.Join(filepath.Dir(i.Path), filepath.FromSlash(o)))
		if folder != "" && f != folder {
			fmt.Fprintf(stderr, "specarch: the implementation files of %s name different output folders for %s (%s and %s); give --out\n", l.spec.Dir, target, folder, f)
			return "", 2
		}
		folder = f
	}
	if folder == "" {
		fmt.Fprintf(stderr, "specarch: no output folder for %s; give --out, or an implementation file whose targets name %s and its output\n", l.spec.Dir, target)
		return "", 2
	}
	return folder, 0
}

// writePlan writes every planned file that differs from the disk.
func writePlan(verb string, plan []planned, stderr io.Writer) int {
	written := 0
	for _, p := range plan {
		if old, err := os.ReadFile(p.path); err == nil && bytes.Equal(old, []byte(p.content)) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
			fmt.Fprintf(stderr, "specarch: cannot write %s: %v\n", p.path, err)
			return 2
		}
		if err := os.WriteFile(p.path, []byte(p.content), 0o644); err != nil {
			fmt.Fprintf(stderr, "specarch: cannot write %s: %v\n", p.path, err)
			return 2
		}
		written++
	}
	fmt.Fprintf(stderr, "specarch %s: %s written, %s already current\n", verb, plural(written, "file"), plural(len(plan)-written, "file"))
	return 0
}

// checkPlan compares the plan with the disk, writing nothing.
func checkPlan(verb string, plan []planned, stdout, stderr io.Writer) int {
	differs := 0
	for _, p := range plan {
		old, err := os.ReadFile(p.path)
		switch {
		case os.IsNotExist(err):
			fmt.Fprintf(stdout, "%s: missing; run specarch %s without --check\n", p.path, verb)
			differs++
		case err != nil:
			fmt.Fprintf(stderr, "specarch: cannot read %s: %v\n", p.path, err)
			return 2
		case !bytes.Equal(old, []byte(p.content)):
			fmt.Fprintf(stdout, "%s: differs from what %s writes; run specarch %s without --check\n", p.path, verb, verb)
			differs++
		}
	}
	fmt.Fprintf(stderr, "specarch %s --check: %s checked, %s differ\n", verb, plural(len(plan), "file"), plural(differs, "file"))
	if differs > 0 {
		return 1
	}
	return 0
}

func relSlash(from, to string) string {
	absFrom, err1 := filepath.Abs(from)
	absTo, err2 := filepath.Abs(to)
	if err1 != nil || err2 != nil {
		return filepath.ToSlash(to)
	}
	rel, err := filepath.Rel(absFrom, absTo)
	if err != nil {
		return filepath.ToSlash(to)
	}
	return filepath.ToSlash(rel)
}

// parseTargetArgs reads: <target> [--out <folder>] [--check] <path>...
// With known given, the target must be one of them. flags names the verb's
// other boolean options; set says which were given.
func parseTargetArgs(verb string, known, flags []string, args []string) (target, out string, check bool, paths []string, set map[string]bool, msg string) {
	set = map[string]bool{}
	if len(args) == 0 {
		return "", "", false, nil, set, fmt.Sprintf("specarch %s needs a target and at least one folder", verb)
	}
	target = args[0]
	if known != nil {
		found := false
		for _, t := range known {
			if t == target {
				found = true
			}
		}
		if !found {
			return "", "", false, nil, set, fmt.Sprintf("specarch %s has no target %q; the targets are %s", verb, target, strings.Join(known, ", "))
		}
	}
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case a == "--":
			paths = append(paths, rest[i+1:]...)
			i = len(rest)
		case a == "--check":
			check = true
		case a == "--out":
			if i+1 >= len(rest) {
				return "", "", false, nil, set, "--out needs a folder"
			}
			out = rest[i+1]
			i++
		case strings.HasPrefix(a, "--out="):
			out = strings.TrimPrefix(a, "--out=")
		case strings.HasPrefix(a, "--") && contains(flags, a[2:]):
			set[a[2:]] = true
		case strings.HasPrefix(a, "-") && len(a) > 1:
			options := "--out and --check"
			for _, f := range flags {
				options = "--out, --check and --" + f
			}
			return "", "", false, nil, set, fmt.Sprintf("specarch %s has no option %s; its options are %s", verb, a, options)
		default:
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 {
		return "", "", false, nil, set, fmt.Sprintf("specarch %s needs at least one folder", verb)
	}
	return target, out, check, paths, set, ""
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// idiomUses resolves the idioms an implementation file uses, for the
// documents.
func idiomUses(path string, root *yaml.Node, s *spec.Spec) []generate.IdiomUse {
	var out []generate.IdiomUse
	for _, u := range validate.IdiomUses(path, root, s) {
		out = append(out, generate.IdiomUse{Name: u.Name, Version: u.Version, As: u.As, From: u.From, Why: u.Why, Parts: u.Parts})
	}
	return out
}
