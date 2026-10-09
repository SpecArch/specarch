package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/approval"
	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/mark"
	"github.com/SpecArch/specarch/internal/problems"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// pluginPrefix starts the name of every generator plug-in on PATH.
const pluginPrefix = "specarch-gen-"

// builtGenerators are the code targets this program has itself. None yet:
// every target is a plug-in.
var builtGenerators = map[string]bool{}

// pluginRequest is what specarch writes on a plug-in's standard input, as
// JSON: the validated specification and the implementation files as plain
// values, the target's settings from each implementation file, and the
// folder the output is for.
type pluginRequest struct {
	Specarch        string                 `json:"specarch"`
	Target          string                 `json:"target"`
	Root            string                 `json:"root"`
	Specification   any                    `json:"specification"`
	Implementations []pluginImplementation `json:"implementations"`
	Output          string                 `json:"output"`
	// Existing are the text files already in the output folder, so a
	// plug-in that adds files (a migration, a snapshot) knows what is
	// there without reading the disk.
	Existing []pluginFile `json:"existing"`
	// Draft is the notice every file of a draft carries, saying why it is
	// not the approved output, or "" when the output is approved
	// (ADR-086).
	Draft string `json:"draft"`
	// Problems are the specification's warnings and open questions, in
	// the order of the problems file, and on the second run the errors
	// the plug-in reported, so the output marks each one at its entry
	// (docs/diagnostics.md).
	Problems []mark.Problem `json:"problems"`
}

type pluginImplementation struct {
	File     string        `json:"file"`
	Content  any           `json:"content"`
	Settings any           `json:"settings,omitempty"`
	Idioms   []pluginIdiom `json:"idioms"`
}

// pluginIdiom is one idiom an implementation file uses, as the plug-in
// needs it to render: how it applies, the idiom file that applies (the
// shipped one, or the project's own), and the override file when the
// project overrides it. An excluded idiom carries no content.
type pluginIdiom struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	As       string `json:"as"`
	Content  any    `json:"content,omitempty"`
	Override any    `json:"override,omitempty"`
}

// pluginResponse is what a plug-in writes on its standard output, as JSON:
// the files to write, with paths relative to the output folder, and its
// diagnostics in the validator's fields.
type pluginResponse struct {
	Files       []pluginFile       `json:"files"`
	Diagnostics []pluginDiagnostic `json:"diagnostics"`
}

type pluginFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type pluginDiagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column,omitempty"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

// runGenerate implements the generate command: a target built into this
// program, or a plug-in found on PATH, which gets the specification on its
// standard input and answers with the files to write. specarch writes
// them, so --check and the output folder are the same for every target.
// The plug-in is looked up per implementation file, by its stack first
// (pluginGroups).
func runGenerate(args []string, stdout, stderr io.Writer) int {
	target, out, check, paths, set, msg := parseTargetArgs("generate", nil, []string{"unapproved"}, args)
	if msg != "" {
		fmt.Fprintf(stderr, "%s\n\n%s", msg, usage)
		return 2
	}
	if !builtGenerators[target] && !validTarget(target) {
		fmt.Fprintf(stderr, "specarch generate: %q is not a target name; a target is kebab-case, such as openapi or sql\n", target)
		return 2
	}
	all, status := loadSpecs(paths, "generate", stdout, stderr)
	if status != 0 {
		return status
	}
	var specs []loaded
	for _, l := range all {
		if out == "" && !namesOutput(l, target) {
			fmt.Fprintln(stdout, noOutputFolder(l, target, "output").String())
			continue
		}
		specs = append(specs, l)
	}
	if len(specs) == 0 {
		fmt.Fprintf(stderr, "specarch generate: no output folder for %s in any specification given; give --out, or an implementation file whose targets name %s and its output\n", target, target)
		return 2
	}
	groups := make([][]pluginGroup, len(specs))
	for i, l := range specs {
		g, status := pluginGroups(l, target, stderr)
		if status != 0 {
			return status
		}
		if len(g) > 1 && out != "" {
			fmt.Fprintf(stderr, "specarch generate: the implementation files of %s take %s to %d plug-ins or output folders, so one --out cannot hold them; name an output for %s in each implementation file instead\n", l.spec.Dir, target, len(g), target)
			return 2
		}
		groups[i] = g
	}
	drafts := make([]string, len(specs))
	for i, l := range specs {
		draft, status := gate(l, target, set["unapproved"], stderr)
		if status != 0 {
			return status
		}
		drafts[i] = draft
	}
	var plan []planned
	failed := false
	for i, l := range specs {
		for _, g := range groups[i] {
			part := loaded{spec: l.spec, impls: g.impls, covered: l.covered}
			folder, status := outputFolder(part, target, out, stderr)
			if status != 0 {
				return status
			}
			req := newPluginRequest(l, target, folder, g.impls)
			req.Draft = drafts[i]
			resp, status := runPlugin(g.exe, g.name, req, stderr)
			if status != 0 {
				return status
			}
			var own []mark.Problem
			for _, d := range placePlugin(l.spec, resp.Diagnostics) {
				fmt.Fprintln(stdout, d.String())
				if d.Severity == validate.Error {
					failed = true
				}
				own = append(own, mark.Problem{ID: d.ID, Severity: string(d.Severity), Rule: string(d.Rule), Message: d.Message, Pointer: pluginPointer(l, d), Target: true})
			}
			// A target that cannot express an entry writes the rest with
			// that entry marked: the plug-in runs again with what it
			// reported among the problems, so its files mark them with the
			// ids given here, and an error makes the run exit 1 (ADR-086).
			if len(own) > 0 {
				req.Problems = append(req.Problems, own...)
				if resp, status = runPlugin(g.exe, g.name, req, stderr); status != 0 {
					return status
				}
				// The second run reports what the first did; anything new
				// is printed and counts as well.
				printed := map[string]bool{}
				for _, p := range own {
					printed[p.ID] = true
				}
				for _, d := range placePlugin(l.spec, resp.Diagnostics) {
					if printed[d.ID] {
						continue
					}
					fmt.Fprintln(stdout, d.String())
					if d.Severity == validate.Error {
						failed = true
					}
				}
			}
			for _, f := range resp.Files {
				clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(f.Path)))
				if f.Path == "" || filepath.IsAbs(f.Path) || clean == ".." || strings.HasPrefix(clean, "../") {
					fmt.Fprintf(stderr, "specarch generate: %s answered with the path %q, which is not inside the output folder; a plug-in writes only there\n", g.name, f.Path)
					return 2
				}
				plan = append(plan, planned{filepath.Join(folder, filepath.FromSlash(clean)), f.Content})
			}
		}
	}
	sort.SliceStable(plan, func(i, j int) bool { return plan[i].path < plan[j].path })
	if check {
		status = checkPlan("generate", plan, stdout, stderr)
	} else {
		status = writePlan("generate", plan, stderr)
	}
	if status == 0 && failed {
		if len(plan) == 0 {
			fmt.Fprintf(stderr, "specarch generate: %s reported errors and answered no files\n", target)
		} else {
			fmt.Fprintf(stderr, "specarch generate: %s cannot express every entry; the rest is made, with each entry it could not write marked at its place\n", target)
		}
		return 1
	}
	return status
}

// pluginPointer is the pointer into the merged specification a placed
// plug-in diagnostic is marked at: its path when it is about the
// specification, and "" when it is about an implementation file.
func pluginPointer(l loaded, d validate.Diagnostic) string {
	for _, i := range l.spec.Implementations {
		if filepath.Clean(i.Path) == filepath.Clean(d.File) {
			return ""
		}
	}
	return d.Path
}

// pluginGroup is one plug-in, the implementation files it is run with,
// and the output folder they name, empty when none does.
type pluginGroup struct {
	name, exe, folder string
	impls             []generate.Implementation
}

// targetOutput is the output folder an implementation file names for a
// target, resolved from the file's folder, or empty.
func targetOutput(i generate.Implementation, target string) string {
	o := source.Str(source.Child(source.Child(source.Child(i.Node, "targets"), target), "output"))
	if o == "" {
		return ""
	}
	return filepath.Clean(filepath.Join(filepath.Dir(i.Path), filepath.FromSlash(o)))
}

// pluginGroups finds the plug-in for each implementation file that names
// the target (every implementation file when none does):
// specarch-gen-<target>-<stack> when it is on PATH, where the stack is the
// file's language in lower case, and specarch-gen-<target> otherwise. Files
// that find the same plug-in and name the same output folder run it
// together, once; a file that names none joins the others of its plug-in.
func pluginGroups(l loaded, target string, stderr io.Writer) ([]pluginGroup, int) {
	var impls []generate.Implementation
	for _, i := range l.impls {
		if source.Child(source.Child(i.Node, "targets"), target) != nil {
			impls = append(impls, i)
		}
	}
	if len(impls) == 0 {
		impls = l.impls
	}
	generic := pluginPrefix + target
	var groups []pluginGroup
	add := func(name, exe string, i *generate.Implementation) {
		folder := ""
		if i != nil {
			folder = targetOutput(*i, target)
		}
		for k := range groups {
			if groups[k].exe == exe && (folder == "" || groups[k].folder == "" || groups[k].folder == folder) {
				if i != nil {
					groups[k].impls = append(groups[k].impls, *i)
				}
				if groups[k].folder == "" {
					groups[k].folder = folder
				}
				return
			}
		}
		g := pluginGroup{name: name, exe: exe, folder: folder}
		if i != nil {
			g.impls = []generate.Implementation{*i}
		}
		groups = append(groups, g)
	}
	if len(impls) == 0 {
		exe, err := exec.LookPath(generic)
		if err != nil {
			fmt.Fprintf(stderr, "specarch generate: no generator for %s: this build has none built in, and no %s was found on PATH; install the plug-in or check its name\n", target, generic)
			return nil, 2
		}
		add(generic, exe, nil)
		return groups, 0
	}
	for k := range impls {
		stack := stackName(impls[k].Node)
		tried := []string{}
		if stack != "" {
			name := generic + "-" + stack
			tried = append(tried, name)
			if exe, err := exec.LookPath(name); err == nil {
				add(name, exe, &impls[k])
				continue
			}
		}
		tried = append(tried, generic)
		exe, err := exec.LookPath(generic)
		if err != nil {
			fmt.Fprintf(stderr, "specarch generate: no generator for %s: this build has none built in, and neither %s was found on PATH; install the plug-in or check its name\n", target, strings.Join(tried, " nor "))
			return nil, 2
		}
		add(generic, exe, &impls[k])
	}
	return groups, 0
}

// pluginProblems are the warnings and open questions of a specification in
// the order of its problems file, each with the pointer a mark is placed by.
func pluginProblems(l loaded, folder string) []mark.Problem {
	pointers := map[string]string{}
	for _, m := range l.marks(folder) {
		pointers[m.ID] = m.Pointer
	}
	out := []mark.Problem{}
	for _, p := range problems.Collect(l.spec, l.diags, l.covered, folder) {
		pp := mark.Problem{ID: p.ID, Severity: string(p.Severity), Rule: p.Rule, Message: p.Message, Pointer: pointers[p.ID]}
		if p.Severity == problems.Question {
			pp.Pointer, pp.Blocks = p.Path, p.Blocks
		}
		out = append(out, pp)
	}
	return out
}

// stackName is an implementation file's language as a plug-in name ends
// with it: lower case, a space as a dash, so Go is go and Objective C is
// objective-c.
func stackName(impl *yaml.Node) string {
	name := source.Str(source.Child(source.Child(source.Child(impl, "stack"), "language"), "name"))
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), " ", "-")
}

// newPluginRequest is the request a plug-in gets for one specification and
// the implementation files that reach it.
func newPluginRequest(l loaded, target, folder string, impls []generate.Implementation) pluginRequest {
	req := pluginRequest{Specarch: source.Str(source.Child(l.spec.Root, "specarch")), Target: target, Root: filepath.ToSlash(l.spec.RootFile), Specification: l.spec.Value, Output: filepath.ToSlash(folder), Implementations: []pluginImplementation{}, Existing: existingFiles(folder), Problems: pluginProblems(l, folder)}
	for _, i := range impls {
		settings := source.Child(source.Child(source.Child(i.Node, "targets"), target), "settings")
		pi := pluginImplementation{File: filepath.ToSlash(i.Path), Content: source.ValueOf(i.Node), Idioms: []pluginIdiom{}}
		if settings != nil {
			pi.Settings = source.ValueOf(settings)
		}
		shipped := validate.ShippedIdioms()
		for _, u := range validate.IdiomUses(i.Path, i.Node, l.spec) {
			pid := pluginIdiom{Name: u.Name, Version: u.Version, As: u.As}
			switch u.As {
			case "shipped":
				pid.Content = source.ValueOf(shipped[u.Name].Root)
			case "overridden":
				pid.Content = source.ValueOf(shipped[u.Name].Root)
				if data, err := os.ReadFile(u.Override); err == nil {
					pid.Override = source.Parse(data).Value
				}
			case "project":
				if data, err := os.ReadFile(projectIdiomPath(i.Path, u.Name)); err == nil {
					pid.Content = source.Parse(data).Value
				}
			}
			pi.Idioms = append(pi.Idioms, pid)
		}
		req.Implementations = append(req.Implementations, pi)
	}
	return req
}

// existingFiles lists the text files under an output folder, by their path
// in it. A file that is not UTF-8, or larger than 4 MiB, is left out.
func existingFiles(folder string) []pluginFile {
	out := []pluginFile{}
	_ = filepath.WalkDir(folder, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return nil
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 4<<20 {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil || !utf8.Valid(data) {
			return nil
		}
		rel, err := filepath.Rel(folder, p)
		if err != nil {
			return nil
		}
		out = append(out, pluginFile{Path: filepath.ToSlash(rel), Content: string(data)})
		return nil
	})
	return out
}

// projectIdiomPath is where the project's own idiom of a name sits, beside
// its implementation file.
func projectIdiomPath(implPath, name string) string {
	return filepath.Join(filepath.Dir(implPath), "idioms", name+validate.IdiomSuffix)
}

// gate refuses to generate a target while a must or should question blocks
// a section it reads, and without an approval of the files as they are.
// With --unapproved it lets both through and answers the notice the draft
// carries; a specification with a blocking question is never approved, so
// both are the one case of output from a specification not yet approved
// (ADR-066, ADR-086).
func gate(l loaded, target string, unapproved bool, stderr io.Writer) (string, int) {
	var reasons []string
	if ids := generate.Holding(l.spec.Root, l.impls, target); len(ids) > 0 {
		verb := "block"
		if len(ids) == 1 {
			verb = "blocks"
		}
		if !unapproved {
			fmt.Fprintf(stderr, "specarch generate: %s of %s %s what %s reads (%s); answer them before generating, as specarch gaps lists them, or pass --unapproved for a draft\n",
				plural(len(ids), "open question"), l.spec.Dir, verb, target, strings.Join(ids, ", "))
			return "", 1
		}
		noun := "open question"
		if len(ids) > 1 {
			noun = "open questions"
		}
		reasons = append(reasons, fmt.Sprintf("%s %s %s what %s reads", noun, strings.Join(ids, ", "), verb, target))
	}
	version := source.Str(source.Child(source.Child(l.spec.Root, "info"), "version"))
	if approved, text := approval.State(l.spec.Dir, version); !approved {
		if !unapproved {
			fmt.Fprintf(stderr, "specarch generate: %s is %s; read the documents and run specarch approve --by <stakeholder> %s, or pass --unapproved for a draft\n", l.spec.Dir, text, l.spec.Dir)
			return "", 1
		}
		reasons = append(reasons, "the specification is "+text)
	}
	if len(reasons) == 0 {
		return "", 0
	}
	// The notice goes into comments of every form, so it is one line.
	return strings.Join(strings.Fields("Draft: "+strings.Join(reasons, ", and ")+"; this file is not the approved output."), " "), 0
}

func validTarget(t string) bool {
	if t == "" || t[0] < 'a' || t[0] > 'z' {
		return false
	}
	for i := 0; i < len(t); i++ {
		c := t[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' && i > 0 && i < len(t)-1 && t[i-1] != '-') {
			return false
		}
	}
	return true
}

// runPlugin runs one plug-in on one specification.
func runPlugin(exe, name string, req pluginRequest, stderr io.Writer) (*pluginResponse, int) {
	in, err := json.Marshal(req)
	if err != nil {
		fmt.Fprintf(stderr, "specarch generate: cannot encode the request for %s: %v\n", name, err)
		return nil, 2
	}
	cmd := exec.Command(exe)
	cmd.Stdin = bytes.NewReader(in)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "specarch generate: %s failed (%v); nothing was written\n", name, err)
		return nil, 2
	}
	var resp pluginResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		fmt.Fprintf(stderr, "specarch generate: %s did not answer with JSON holding files and diagnostics (%v); nothing was written\n", name, err)
		return nil, 2
	}
	return &resp, 0
}

// placePlugin puts a plug-in's diagnostics in validate's line. A plug-in
// names the entry by its pointer and the root file; one whose pointer leads
// to an entry of the merged specification is put at the fragment, line and
// column that entry was read from. The column, when not given, and the id
// are made as validate makes them.
func placePlugin(s *spec.Spec, ds []pluginDiagnostic) []validate.Diagnostic {
	out := make([]validate.Diagnostic, 0, len(ds))
	for _, d := range ds {
		v := validate.Diagnostic{File: d.File, Line: d.Line, Column: d.Column, Severity: validate.Severity(d.Severity),
			Path: d.Path, Rule: validate.Rule(d.Rule), Message: d.Message}
		if v.Path == "" {
			v.Path = "/"
		}
		if s.Root != nil && d.File == s.RootFile {
			if start, value, ok := validate.Locate(s.Root, validate.Tokens(v.Path)); ok {
				file := s.Files[value]
				if file == "" {
					file = s.Files[start]
				}
				if file != "" {
					v.File, v.Line, v.Column = file, start.Line, 0
				}
			}
		}
		if v.Line < 1 {
			v.Line = 1
		}
		out = append(out, v)
	}
	validate.SpecPlacer(s).Place(out)
	return out
}
