package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// documentTargets are the document targets of the design; builtDocuments
// says which this program has.
var (
	documentTargets = []string{"techspec", "requirements", "testplan", "traceability", "deployment", "commissioning", "manual", "operations"}
	builtDocuments  = map[string]bool{"techspec": true}
)

// planned is one file a target wants on disk.
type planned struct {
	path    string
	content string
}

// loaded is one validated specification with its implementation files
// parsed.
type loaded struct {
	spec  *spec.Spec
	impls []generate.Implementation
}

// runDocument implements the document command and its checkStatus
// algorithm: 2 for a usage or read error, 1 for invalid input, a marker
// error or, with --check, a difference, 0 otherwise.
func runDocument(args []string, stdout, stderr io.Writer) int {
	target, out, check, paths, msg := parseTargetArgs("document", documentTargets, args)
	if msg != "" {
		fmt.Fprintf(stderr, "%s\n\n%s", msg, usage)
		return 2
	}
	if !builtDocuments[target] {
		fmt.Fprintf(stderr, "specarch document: this build has no %s documentor; it has techspec\n", target)
		return 2
	}
	specs, status := loadSpecs(paths, "document", stdout, stderr)
	if status != 0 {
		return status
	}
	var plan []planned
	failed := false
	for _, l := range specs {
		folder, status := outputFolder(l, target, out, stderr)
		if status != 0 {
			return status
		}
		var impls []generate.Implementation
		for _, i := range l.impls {
			impls = append(impls, generate.Implementation{Node: i.Node, Rel: relSlash(folder, i.Path), Path: i.Path})
		}
		plan = append(plan, planned{filepath.Join(folder, generate.TechspecName), generate.Techspec(l.spec.Root, relSlash(folder, l.spec.RootFile), impls)})

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
	if check {
		return checkPlan("document", plan, stdout, stderr)
	}
	return writePlan("document", plan, stderr)
}

// loadSpecs reads and validates every specification under paths. An
// invalid one prints its errors; nothing is produced then.
func loadSpecs(paths []string, verb string, stdout, stderr io.Writer) ([]loaded, int) {
	inputs, ioError := collect(paths, stderr)
	if ioError {
		return nil, 2
	}
	var specs []loaded
	invalid := false
	for _, in := range inputs {
		if in.root == "" {
			name := in.implementation
			if name == "" {
				name = in.other
			}
			fmt.Fprintf(stderr, "specarch %s: %s is not a specification; name the folder that holds %s\n", verb, name, spec.RootFile)
			return nil, 2
		}
		s := spec.Load(in.root)
		diags := validate.CheckSpec(s)
		if validate.Errors(diags) > 0 {
			for _, d := range diags {
				if d.Severity == validate.Error {
					fmt.Fprintln(stdout, d.String())
				}
			}
			invalid = true
			continue
		}
		l := loaded{spec: s}
		for _, impl := range s.Implementations {
			doc := source.Parse(impl.Data)
			l.impls = append(l.impls, generate.Implementation{Node: doc.Root, Path: impl.Path})
		}
		specs = append(specs, l)
	}
	if invalid {
		fmt.Fprintf(stderr, "specarch %s: the input has errors, so nothing was produced\n", verb)
		return nil, 1
	}
	if len(specs) == 0 {
		fmt.Fprintf(stderr, "specarch %s: no specification given; name a folder that holds %s\n", verb, spec.RootFile)
		return nil, 2
	}
	return specs, 0
}

// outputFolder is --out, or the folder the implementation files name for
// the target under targets; they must agree.
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
// With known given, the target must be one of them.
func parseTargetArgs(verb string, known []string, args []string) (target, out string, check bool, paths []string, msg string) {
	if len(args) == 0 {
		return "", "", false, nil, fmt.Sprintf("specarch %s needs a target and at least one folder", verb)
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
			return "", "", false, nil, fmt.Sprintf("specarch %s has no target %q; the targets are %s", verb, target, strings.Join(known, ", "))
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
				return "", "", false, nil, "--out needs a folder"
			}
			out = rest[i+1]
			i++
		case strings.HasPrefix(a, "--out="):
			out = strings.TrimPrefix(a, "--out=")
		case strings.HasPrefix(a, "-") && len(a) > 1:
			return "", "", false, nil, fmt.Sprintf("specarch %s has no option %s; its options are --out and --check", verb, a)
		default:
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 {
		return "", "", false, nil, fmt.Sprintf("specarch %s needs at least one folder", verb)
	}
	return target, out, check, paths, ""
}
