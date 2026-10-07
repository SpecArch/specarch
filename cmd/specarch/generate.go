package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/approval"
	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/source"
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
}

type pluginImplementation struct {
	File     string `json:"file"`
	Content  any    `json:"content"`
	Settings any    `json:"settings,omitempty"`
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
// program, or the plug-in specarch-gen-<target> found on PATH, which gets
// the specification on its standard input and answers with the files to
// write. specarch writes them, so --check and the output folder are the
// same for every target.
func runGenerate(args []string, stdout, stderr io.Writer) int {
	target, out, check, paths, set, msg := parseTargetArgs("generate", nil, []string{"unapproved"}, args)
	if msg != "" {
		fmt.Fprintf(stderr, "%s\n\n%s", msg, usage)
		return 2
	}
	plugin := ""
	if !builtGenerators[target] {
		if !validTarget(target) {
			fmt.Fprintf(stderr, "specarch generate: %q is not a target name; a target is kebab-case, such as openapi or sql\n", target)
			return 2
		}
		exe, err := exec.LookPath(pluginPrefix + target)
		if err != nil {
			fmt.Fprintf(stderr, "specarch generate: no generator for %s: this build has none built in, and no %s%s was found on PATH; install the plug-in or check its name\n", target, pluginPrefix, target)
			return 2
		}
		plugin = exe
	}
	specs, status := loadSpecs(paths, "generate", stdout, stderr)
	if status != 0 {
		return status
	}
	for _, l := range specs {
		if status := gate(l, target, set["unapproved"], stderr); status != 0 {
			return status
		}
	}
	var plan []planned
	failed := false
	for _, l := range specs {
		folder, status := outputFolder(l, target, out, stderr)
		if status != 0 {
			return status
		}
		req := pluginRequest{Specarch: source.Str(source.Child(l.spec.Root, "specarch")), Target: target, Root: filepath.ToSlash(l.spec.RootFile), Specification: l.spec.Value, Output: filepath.ToSlash(folder)}
		for _, i := range l.impls {
			settings := source.Child(source.Child(source.Child(i.Node, "targets"), target), "settings")
			pi := pluginImplementation{File: filepath.ToSlash(i.Path), Content: source.ValueOf(i.Node)}
			if settings != nil {
				pi.Settings = source.ValueOf(settings)
			}
			req.Implementations = append(req.Implementations, pi)
		}
		resp, status := runPlugin(plugin, target, req, stderr)
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
				fmt.Fprintf(stderr, "specarch generate: %s%s answered with the path %q, which is not inside the output folder; a plug-in writes only there\n", pluginPrefix, target, f.Path)
				return 2
			}
			plan = append(plan, planned{filepath.Join(folder, filepath.FromSlash(clean)), f.Content})
		}
	}
	if failed {
		fmt.Fprintf(stderr, "specarch generate: %s%s reported errors, so nothing was written\n", pluginPrefix, target)
		return 1
	}
	sort.SliceStable(plan, func(i, j int) bool { return plan[i].path < plan[j].path })
	if check {
		return checkPlan("generate", plan, stdout, stderr)
	}
	return writePlan("generate", plan, stderr)
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
func runPlugin(exe, target string, req pluginRequest, stderr io.Writer) (*pluginResponse, int) {
	in, err := json.Marshal(req)
	if err != nil {
		fmt.Fprintf(stderr, "specarch generate: cannot encode the request for %s%s: %v\n", pluginPrefix, target, err)
		return nil, 2
	}
	cmd := exec.Command(exe)
	cmd.Stdin = bytes.NewReader(in)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "specarch generate: %s%s failed (%v); nothing was written\n", pluginPrefix, target, err)
		return nil, 2
	}
	var resp pluginResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		fmt.Fprintf(stderr, "specarch generate: %s%s did not answer with JSON holding files and diagnostics (%v); nothing was written\n", pluginPrefix, target, err)
		return nil, 2
	}
	return &resp, 0
}
