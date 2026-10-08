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
	"github.com/SpecArch/specarch/internal/source"
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
	specs, status := loadSpecs(paths, "generate", stdout, stderr)
	if status != 0 {
		return status
	}
	groups := make([][]pluginGroup, len(specs))
	for i, l := range specs {
		g, status := pluginGroups(l, target, stderr)
		if status != 0 {
			return status
		}
		if len(g) > 1 && out != "" {
			fmt.Fprintf(stderr, "specarch generate: the implementation files of %s take %s to %d plug-ins, so one --out cannot hold them; name an output for %s in each implementation file instead\n", l.spec.Dir, target, len(g), target)
			return 2
		}
		groups[i] = g
	}
	for _, l := range specs {
		if status := gate(l, target, set["unapproved"], stderr); status != 0 {
			return status
		}
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
			resp, status := runPlugin(g.exe, g.name, req, stderr)
			if status != 0 {
				return status
			}
			for _, d := range resp.Diagnostics {
				fmt.Fprintf(stdout, "%s:%d: %s: %s: %s: %s\n", d.File, d.Line, d.Severity, d.Path, d.Rule, d.Message)
				if d.Severity == "error" {
					failed = true
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
	if failed {
		fmt.Fprintf(stderr, "specarch generate: the plug-in for %s reported errors, so nothing was written\n", target)
		return 1
	}
	sort.SliceStable(plan, func(i, j int) bool { return plan[i].path < plan[j].path })
	if check {
		return checkPlan("generate", plan, stdout, stderr)
	}
	return writePlan("generate", plan, stderr)
}

// pluginGroup is one plug-in and the implementation files it is run with.
type pluginGroup struct {
	name, exe string
	impls     []generate.Implementation
}

// pluginGroups finds the plug-in for each implementation file that names
// the target (every implementation file when none does):
// specarch-gen-<target>-<stack> when it is on PATH, where the stack is the
// file's language in lower case, and specarch-gen-<target> otherwise. Files
// that find the same plug-in run it together, once.
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
		for k := range groups {
			if groups[k].exe == exe {
				if i != nil {
					groups[k].impls = append(groups[k].impls, *i)
				}
				return
			}
		}
		g := pluginGroup{name: name, exe: exe}
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
	req := pluginRequest{Specarch: source.Str(source.Child(l.spec.Root, "specarch")), Target: target, Root: filepath.ToSlash(l.spec.RootFile), Specification: l.spec.Value, Output: filepath.ToSlash(folder), Implementations: []pluginImplementation{}, Existing: existingFiles(folder)}
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
// a section it reads, and without an approval of the files as they are,
// unless --unapproved was given.
func gate(l loaded, target string, unapproved bool, stderr io.Writer) int {
	if ids := generate.Holding(l.spec.Root, l.impls, target); len(ids) > 0 {
		verb := "block"
		if len(ids) == 1 {
			verb = "blocks"
		}
		fmt.Fprintf(stderr, "specarch generate: %s of %s %s what %s reads (%s); answer them before generating, as specarch gaps lists them\n",
			plural(len(ids), "open question"), l.spec.Dir, verb, target, strings.Join(ids, ", "))
		return 1
	}
	if unapproved {
		return 0
	}
	version := source.Str(source.Child(source.Child(l.spec.Root, "info"), "version"))
	approved, text := approval.State(l.spec.Dir, version)
	if !approved {
		fmt.Fprintf(stderr, "specarch generate: %s is %s; read the documents and run specarch approve --by <stakeholder> %s, or pass --unapproved\n", l.spec.Dir, text, l.spec.Dir)
		return 1
	}
	return 0
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
