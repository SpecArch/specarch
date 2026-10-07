package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/validate"
)

// targets are the generator targets of the design; built says which this
// program has.
var (
	targets = []string{"techspec", "openapi", "sql", "ui", "tests", "manual", "scripts"}
	built   = map[string]bool{"techspec": true}
)

// planned is one file generate wants on disk.
type planned struct {
	path    string
	content string
}

// runGenerate implements the generate command and its checkStatus
// algorithm: 2 for a usage or read error, 1 for invalid input, a marker
// error or, with --check, a difference, 0 otherwise.
func runGenerate(args []string, stdout, stderr io.Writer) int {
	target, out, check, paths, msg := parseGenerate(args)
	if msg != "" {
		fmt.Fprintf(stderr, "%s\n\n%s", msg, usage)
		return 2
	}
	if !built[target] {
		fmt.Fprintf(stderr, "specarch generate: this build has no %s generator; it has techspec\n", target)
		return 2
	}
	files, ioError := collect(paths, stderr)
	if ioError {
		return 2
	}
	type input struct {
		path string
		root *yaml.Node
	}
	var designs, impls []input
	invalid := false
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(stderr, "specarch: cannot read %s: %v\n", f, err)
			return 2
		}
		diags := validate.Check(f, data, os.ReadFile)
		if validate.Errors(diags) > 0 {
			for _, d := range diags {
				if d.Severity == validate.Error {
					fmt.Fprintln(stdout, d.String())
				}
			}
			invalid = true
			continue
		}
		doc := source.Parse(data)
		switch validate.KindOf(f) {
		case validate.KindDesign:
			designs = append(designs, input{f, doc.Root})
		case validate.KindImplementation:
			impls = append(impls, input{f, doc.Root})
		}
	}
	if invalid {
		fmt.Fprintln(stderr, "specarch generate: the input has errors, so nothing was generated")
		return 1
	}
	if len(designs) == 0 {
		fmt.Fprintf(stderr, "specarch generate: no design file given; name at least one *%s\n", validate.DesignSuffix)
		return 2
	}

	var plan []planned
	failed := false
	outFolders := map[string]bool{}
	for _, d := range designs {
		var mine []input
		for _, i := range impls {
			named := filepath.Join(filepath.Dir(i.path), filepath.FromSlash(source.Str(source.Child(source.Child(i.root, "implements"), "file"))))
			if filepath.Clean(named) == filepath.Clean(d.path) {
				mine = append(mine, i)
			}
		}
		folder := out
		if folder == "" {
			for _, i := range mine {
				o := source.Str(source.Child(source.Child(source.Child(i.root, "generators"), target), "output"))
				if o == "" {
					continue
				}
				f := filepath.Clean(filepath.Join(filepath.Dir(i.path), filepath.FromSlash(o)))
				if folder != "" && folder != f {
					fmt.Fprintf(stderr, "specarch generate: the implementation files of %s name different folders for %s (%s and %s); give --out, or let one of them name it\n", d.path, target, folder, f)
					return 2
				}
				folder = f
			}
		}
		if folder == "" {
			fmt.Fprintf(stderr, "specarch generate: no output folder for %s; give --out, or an implementation file whose generators name %s and its output\n", d.path, target)
			return 2
		}
		folder = filepath.Clean(folder)
		outFolders[folder] = true
		relDesign := relSlash(folder, d.path)
		var given []generate.Implementation
		for _, i := range mine {
			given = append(given, generate.Implementation{Root: i.root, Rel: relSlash(folder, i.path)})
		}
		plan = append(plan, planned{filepath.Join(folder, generate.TechspecName(d.path)), generate.Techspec(d.root, relDesign, given)})

		companion := strings.TrimSuffix(d.path, ".yaml") + ".md"
		if text, err := os.ReadFile(companion); err == nil {
			rewritten, errs := generate.RewriteMarkers(string(text), d.root)
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
		fmt.Fprintln(stderr, "specarch generate: the markers have errors, so nothing was written")
		return 1
	}

	if check {
		return checkPlan(plan, outFolders, stdout, stderr)
	}
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
	fmt.Fprintf(stderr, "specarch generate: %s written, %s already current\n", plural(written, "file"), plural(len(plan)-written, "file"))
	return 0
}

// checkPlan compares the plan with the disk, writing nothing.
func checkPlan(plan []planned, outFolders map[string]bool, stdout, stderr io.Writer) int {
	differs := 0
	wanted := map[string]bool{}
	for _, p := range plan {
		wanted[filepath.Clean(p.path)] = true
		old, err := os.ReadFile(p.path)
		switch {
		case os.IsNotExist(err):
			fmt.Fprintf(stdout, "%s: missing; run specarch generate without --check\n", p.path)
			differs++
		case err != nil:
			fmt.Fprintf(stderr, "specarch: cannot read %s: %v\n", p.path, err)
			return 2
		case !bytes.Equal(old, []byte(p.content)):
			fmt.Fprintf(stdout, "%s: differs from what generate writes; run specarch generate without --check\n", p.path)
			differs++
		}
	}
	var folders []string
	for f := range outFolders {
		folders = append(folders, f)
	}
	sort.Strings(folders)
	for _, f := range folders {
		entries, _ := os.ReadDir(f)
		for _, e := range entries {
			p := filepath.Join(f, e.Name())
			if strings.HasSuffix(e.Name(), ".techspec.md") && !wanted[p] {
				fmt.Fprintf(stdout, "%s: no design file given produces it; delete it, or give its design file\n", p)
				differs++
			}
		}
	}
	fmt.Fprintf(stderr, "specarch generate --check: %s checked, %s differ\n", plural(len(plan), "file"), plural(differs, "file"))
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

// parseGenerate reads: <target> [--out <folder>] [--check] <path>...
func parseGenerate(args []string) (target, out string, check bool, paths []string, msg string) {
	if len(args) == 0 {
		return "", "", false, nil, "specarch generate needs a target and at least one file"
	}
	target = args[0]
	known := false
	for _, t := range targets {
		if t == target {
			known = true
		}
	}
	if !known {
		return "", "", false, nil, fmt.Sprintf("specarch generate has no target %q; the targets are %s", target, strings.Join(targets, ", "))
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
			return "", "", false, nil, fmt.Sprintf("specarch generate has no option %s; its options are --out and --check", a)
		default:
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 {
		return "", "", false, nil, "specarch generate needs at least one file"
	}
	return target, out, check, paths, ""
}
