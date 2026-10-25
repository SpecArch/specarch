package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/metamodel"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/upgrade"
	"github.com/SpecArch/specarch/internal/validate"
)

// upgradeFile is one file of a specification upgrade reads, with its kind.
type upgradeFile struct {
	path string
	kind upgrade.Kind
	// current is, for a root file or a fragment, whether its
	// specification is read as 1.0 after the move, which moves its hint
	// and renamed keys.
	current bool
	// dir is the folder the ids of its problems name it from.
	dir string
}

// runUpgrade moves each specification given, and every file it is read
// from, to 1.0.0: the version lines, the editor hints and the renamed keys,
// edited in place. With --dry-run it writes nothing. A step it cannot take
// is printed as a problem line, and exits 1.
func runUpgrade(args []string, stdout, stderr io.Writer) int {
	dryRun := false
	to := metamodel.Current
	var paths []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			paths = append(paths, args[i+1:]...)
			i = len(args)
		case a == "--dry-run":
			dryRun = true
		case a == "--to" || strings.HasPrefix(a, "--to="):
			if v, ok := strings.CutPrefix(a, "--to="); ok {
				to = v
			} else if i+1 < len(args) {
				i++
				to = args[i]
			} else {
				fmt.Fprintf(stderr, "specarch upgrade: --to needs a version\n\n%s", usage)
				return 2
			}
		case len(a) > 1 && a[0] == '-':
			fmt.Fprintf(stderr, "specarch upgrade has no option %s\n\n%s", a, usage)
			return 2
		default:
			paths = append(paths, a)
		}
	}
	if to != metamodel.Current {
		fmt.Fprintf(stderr, "specarch upgrade: --to %s is not a version this program writes; it moves every file to %s, the newest, so leave --to out or give %s\n\n%s", to, metamodel.Current, metamodel.Current, usage)
		return 2
	}
	if len(paths) == 0 {
		fmt.Fprintf(stderr, "specarch upgrade needs at least one folder\n\n%s", usage)
		return 2
	}
	inputs, ioError := collect(paths, stderr)
	if ioError {
		return 2
	}
	var specs []*spec.Spec
	var estates, impls []string
	for _, in := range inputs {
		switch {
		case in.estate != nil:
			estates = append(estates, in.estate.Path)
		case in.root != "":
			specs = append(specs, spec.Load(in.root))
		case in.implementation != "":
			impls = append(impls, in.implementation)
		default:
			fmt.Fprintf(stderr, "specarch upgrade: %s is neither a specification nor an implementation file nor an estate; name the folder that holds %s\n", in.other, spec.RootFile)
			return 2
		}
	}
	if len(specs) == 0 && len(estates) == 0 && len(impls) == 0 {
		fmt.Fprintf(stderr, "specarch upgrade: no specification given; name a folder that holds %s\n", spec.RootFile)
		return 2
	}

	var files []upgradeFile
	seen := map[string]bool{}
	add := func(f upgradeFile) {
		if !seen[filepath.Clean(f.path)] {
			seen[filepath.Clean(f.path)] = true
			files = append(files, f)
		}
	}
	for _, e := range estates {
		add(upgradeFile{e, upgrade.Estate, true, filepath.Dir(e)})
	}
	for _, i := range impls {
		for _, f := range implementationFiles(i, filepath.Dir(i)) {
			add(f)
		}
	}
	for _, s := range specs {
		for _, f := range specFiles(s) {
			add(f)
		}
	}
	sort.SliceStable(files, func(i, j int) bool { return filepath.ToSlash(files[i].path) < filepath.ToSlash(files[j].path) })

	var changes []upgrade.Change
	var left []validate.Diagnostic
	byFile := map[string][]upgrade.Change{}
	data := map[string][]byte{}
	for _, f := range files {
		d, err := os.ReadFile(f.path)
		if err != nil {
			fmt.Fprintf(stderr, "specarch upgrade: cannot read %s: %s\n", f.path, spec.PlainIOError(err))
			return 2
		}
		cs := upgrade.Changes(filepath.ToSlash(f.path), d, f.kind, version, f.current)
		if st, ok := upgrade.Left(d, f.kind); ok {
			l := []validate.Diagnostic{{File: f.path, Line: st.Line, Column: st.Column, Severity: validate.Error, Path: st.Pointer, Rule: validate.RuleMetaModelVersion, Message: st.Message}}
			(&validate.Placer{Dir: f.dir}).Place(l)
			left = append(left, l...)
		}
		if len(cs) > 0 {
			data[f.path] = d
			byFile[f.path] = cs
			changes = append(changes, cs...)
		}
	}

	steps := upgradeSteps(specs)
	steps = append(steps, left...)
	validate.Sort(steps)
	for _, c := range changes {
		fmt.Fprintln(stdout, c.String())
	}
	for _, d := range steps {
		printDiagnostic(stdout, stderr, d)
	}
	if !dryRun {
		for _, f := range files {
			cs := byFile[f.path]
			if len(cs) == 0 {
				continue
			}
			mode := os.FileMode(0o644)
			if info, err := os.Stat(f.path); err == nil {
				mode = info.Mode().Perm()
			}
			if err := os.WriteFile(f.path, upgrade.Apply(data[f.path], cs), mode); err != nil {
				fmt.Fprintf(stderr, "specarch upgrade: cannot write %s: %s\n", f.path, spec.PlainIOError(err))
				return 2
			}
		}
	}
	verb := "made"
	if dryRun {
		verb = "to make"
	}
	fmt.Fprintf(stderr, "specarch upgrade: %s %s in %s, %s left to the author\n", plural(len(changes), "change"), verb, plural(len(byFile), "file"), plural(len(steps), "step"))
	for _, d := range steps {
		if d.Rule == validate.RuleTestCaseMissing || d.Rule == validate.RuleTestGoldenMissing {
			fmt.Fprintln(stderr, "specarch upgrade: the tests the steps name are missing; specarch derive writes them")
			break
		}
	}
	if len(steps) > 0 {
		return 1
	}
	return 0
}

// specFiles lists every file a specification is read from, with its kind:
// the root file, the stage files and tests, the implementation files and
// the idioms in the folder beside each, and the records. The hints and
// the renamed keys of the root file and the fragments move only when the
// root file's version line leaves the specification read as 1.0 after the
// move.
func specFiles(s *spec.Spec) []upgradeFile {
	current := !upgrade.StaysEarlier(metamodel.Design, source.Child(s.Root, metamodel.Design.Key))
	var out []upgradeFile
	paths := map[string]bool{}
	for _, p := range s.Files {
		paths[p] = true
	}
	for _, h := range s.Held {
		paths[h.File] = true
	}
	for p := range paths {
		kind := upgrade.Fragment
		if filepath.Clean(p) == filepath.Clean(s.RootFile) {
			kind = upgrade.Root
		}
		out = append(out, upgradeFile{p, kind, current, s.Dir})
	}
	if !paths[s.RootFile] {
		out = append(out, upgradeFile{s.RootFile, upgrade.Root, current, s.Dir})
	}
	for _, impl := range s.Implementations {
		out = append(out, implementationFiles(impl.Path, s.Dir)...)
	}
	for _, r := range s.Records {
		out = append(out, upgradeFile{r.Path, upgrade.Record, true, s.Dir})
	}
	return out
}

// implementationFiles is an implementation file and the idioms in the
// idioms/ folder beside it, whose problems' ids name them from dir.
func implementationFiles(path, dir string) []upgradeFile {
	out := []upgradeFile{{path, upgrade.Implementation, true, dir}}
	idioms, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "idioms", "*"+validate.IdiomSuffix))
	for _, i := range idioms {
		out = append(out, upgradeFile{i, upgrade.Idiom, true, dir})
	}
	return out
}

// upgradeSteps are the steps the move leaves to the author: every error a
// specification would have on 1.0.0 that it does not have on its own
// version, a key given under both spellings, and a file or an entry the
// parser cannot read. The other problems are validate's.
func upgradeSteps(specs []*spec.Spec) []validate.Diagnostic {
	given := validate.NewGiven(specs)
	type at struct{ rule, file, path string }
	var out []validate.Diagnostic
	for i, s := range specs {
		before, _ := validate.CheckSpecCovered(s, given)
		had := map[at]bool{}
		for _, d := range before {
			if d.Severity == validate.Error {
				had[at{string(d.Rule), d.File, d.Path}] = true
			}
			if d.Rule == validate.RuleYAMLSyntax {
				out = append(out, d)
			}
		}
		moved := spec.Load(s.Dir)
		if line := source.Child(moved.Root, metamodel.Design.Key); upgrade.Moves(metamodel.Design, line) {
			line.Value = metamodel.Current
			if m, ok := moved.Value.(map[string]any); ok {
				m["specarch"] = metamodel.Current
			}
		}
		others := append(append([]*spec.Spec(nil), specs[:i]...), moved)
		others = append(others, specs[i+1:]...)
		after, _ := validate.CheckSpecCovered(moved, validate.NewGiven(others))
		for _, d := range after {
			if d.Severity != validate.Error {
				continue
			}
			if d.Rule == validate.RuleMetaModelDeprecated || !had[at{string(d.Rule), d.File, d.Path}] {
				out = append(out, d)
			}
		}
	}
	validate.Sort(out)
	return out
}
