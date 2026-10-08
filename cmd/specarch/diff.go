package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/diff"
	"github.com/SpecArch/specarch/internal/semver"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// runDiff compares two versions of a specification and checks the release
// of the new one against the change list: 0 when every check passes, 1
// when one fails or the new version has no release record, 2 on a usage
// error, an unreadable folder or a specification with errors.
func runDiff(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	for _, a := range args {
		if len(a) > 1 && a[0] == '-' {
			fmt.Fprintf(stderr, "specarch diff has no option %s\n\n%s", a, usage)
			return 2
		}
	}
	if len(args) != 2 {
		fmt.Fprintf(stderr, "specarch diff needs two folders, the old specification and the new one\n\n%s", usage)
		return 2
	}
	var specs [2]*spec.Spec
	invalid := false
	for i, a := range args {
		info, err := os.Stat(a)
		if err != nil {
			fmt.Fprintf(stderr, "specarch diff: cannot read %s: no such file or directory\n", a)
			return 2
		}
		if _, err := os.Stat(filepath.Join(a, spec.RootFile)); err != nil || !info.IsDir() {
			fmt.Fprintf(stderr, "specarch diff: %s is not a specification; name the folder that holds %s\n", a, spec.RootFile)
			return 2
		}
		s := spec.Load(a)
		for _, d := range validate.CheckSpec(s) {
			if d.Severity == validate.Error {
				fmt.Fprintln(stdout, d.String())
				invalid = true
			}
		}
		specs[i] = s
	}
	if invalid {
		fmt.Fprintln(stderr, "specarch diff: the input has errors, so nothing was compared")
		return 2
	}
	oldSpec, newSpec := specs[0], specs[1]
	changes := diff.Compare(oldSpec.Root, newSpec.Root)
	for _, c := range changes {
		fmt.Fprintln(stdout, c.String())
	}
	var problems []string
	warnings := 0
	oldText := infoVersion(oldSpec)
	newText := infoVersion(newSpec)
	oldV, _ := semver.Parse(oldText)
	newV, _ := semver.Parse(newText)
	ver := newV.Core()
	rs := readRecords(newSpec)
	release := rs.find("release", ver)
	if release == nil {
		problems = append(problems, fmt.Sprintf("error: version: there is no records/releases/%s.yaml for the new version %s; add it with the changes and defects it includes", ver, newText))
	} else {
		problems = append(problems, versionProblems(oldV, oldText, ver, changes)...)
		cov, warn := coverageProblems(rs, release, ver, changes)
		problems = append(problems, cov...)
		warnings = warn
	}
	for _, p := range problems {
		fmt.Fprintln(stdout, p)
	}
	errors := len(problems) - warnings
	fmt.Fprintf(stderr, "specarch diff: %s changed, %s, %s\n", plural(len(changes), "element"), plural(errors, "error"), plural(warnings, "warning"))
	if errors > 0 {
		return 1
	}
	return 0
}

func infoVersion(s *spec.Spec) string {
	return source.Str(source.Child(source.Child(s.Root, "info"), "version"))
}

// versionProblems checks that the release steps from the old version by at
// least the largest impact in the change list.
func versionProblems(oldV semver.Version, oldText, ver string, changes []diff.Change) []string {
	newV, _ := semver.Parse(ver)
	if semver.Compare(newV, oldV) <= 0 {
		return []string{fmt.Sprintf("error: version: %s is not after the old version %s", ver, oldText)}
	}
	need, why := semver.None, ""
	for _, c := range changes {
		if n := semver.Needed(oldV, c.Impact); n > need {
			need, why = n, c.Pointer
		}
	}
	step := semver.Step(oldV, newV)
	if step >= need {
		return nil
	}
	stepText := "a " + semver.StepNames[step] + " step"
	if step == semver.None {
		stepText = "no step of major, minor or patch"
	}
	return []string{fmt.Sprintf("error: version: %s to %s is %s, but #%s needs a %s step; release it as %s", oldText, ver, stepText, why, semver.StepNames[need], semver.Next(oldV, need))}
}

// coverageProblems checks that a change request or defect the release
// includes names every element in the list.
func coverageProblems(rs records, release *recordFile, ver string, changes []diff.Change) (problems []string, warnings int) {
	var names [][]string
	var tracker []string
	for _, item := range source.Items(source.Child(release.root, "includes")) {
		id := item.Value
		switch {
		case rs.find("change", id) != nil:
			affects := source.Child(rs.find("change", id).root, "affects")
			for _, list := range []string{"adds", "changes", "removes"} {
				for _, e := range source.Items(source.Child(affects, list)) {
					names = append(names, entryTokens(e.Value))
				}
			}
		case rs.find("defect", id) != nil:
			for _, e := range source.Items(source.Child(rs.find("defect", id).root, "violates")) {
				names = append(names, entryTokens(e.Value))
			}
		default:
			tracker = append(tracker, id)
		}
	}
	for _, c := range changes {
		el := strings.Split(strings.TrimPrefix(c.Pointer, "/"), "/")
		for i := range el {
			el[i] = source.UnescapeToken(el[i])
		}
		if namedBy(el, names) {
			continue
		}
		if len(tracker) > 0 {
			problems = append(problems, fmt.Sprintf("warning: covered: #%s is %s and no record release %s includes names it; %s, kept in a tracker, may", c.Pointer, c.What, ver, strings.Join(tracker, ", ")))
			warnings++
			continue
		}
		problems = append(problems, fmt.Sprintf("error: covered: #%s is %s, but no change request or defect release %s includes names it; name it in the affects of one, or include the one that does", c.Pointer, c.What, ver))
	}
	return problems, warnings
}

// entryTokens reads an affects or violates entry: a requirement ID or a
// #/ pointer.
func entryTokens(e string) []string {
	if !strings.HasPrefix(e, "#/") {
		return []string{"requirements", e}
	}
	var out []string
	for _, t := range strings.Split(strings.TrimPrefix(e, "#/"), "/") {
		out = append(out, source.UnescapeToken(t))
	}
	return out
}

// namedBy reports whether one entry and the element are a prefix of each
// other.
func namedBy(el []string, names [][]string) bool {
	for _, n := range names {
		short := min(len(n), len(el))
		match := short > 0
		for i := 0; i < short; i++ {
			if n[i] != el[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// recordFile is one record file beside the new specification, parsed.
type recordFile struct {
	kind, key string
	root      *yaml.Node
}

type records []*recordFile

func (rs records) find(kind, key string) *recordFile {
	for _, r := range rs {
		if r.kind == kind && r.key == key {
			return r
		}
	}
	return nil
}

// readRecords parses the records beside a specification; the validator
// has already checked them.
func readRecords(s *spec.Spec) records {
	var out records
	for _, f := range s.Records {
		doc := source.Parse(f.Data)
		if doc.Root == nil {
			continue
		}
		kind := source.Str(source.Child(doc.Root, "kind"))
		key := source.Str(source.Child(doc.Root, "id"))
		if kind == "release" {
			key = source.Str(source.Child(doc.Root, "version"))
		}
		out = append(out, &recordFile{kind: kind, key: key, root: doc.Root})
	}
	return out
}
