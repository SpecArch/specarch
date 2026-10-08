package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// A conformance case, as conformance/<name>/case.yaml holds it.
type conformanceCase struct {
	Arguments      []string `yaml:"arguments"`
	ExitStatus     int      `yaml:"exitStatus"`
	StandardOutput string   `yaml:"standardOutput"`
	// Plugins names the targets whose plug-in from this repository the
	// case runs with, such as openapi for cmd/specarch-gen-openapi: each is
	// built once and put on PATH.
	Plugins []string `yaml:"plugins"`
	// Repository makes a folder of the case a git repository before the
	// run, for the cases of extract, which read a commit.
	Repository *caseRepository `yaml:"repository"`
}

// A throwaway git repository a case runs in. Its commits are made with a
// fixed author, committer, date and message, so their hashes never change
// and the case can name them.
type caseRepository struct {
	// Folder is the folder of the case that becomes the repository.
	Folder string `yaml:"folder"`
	// Commits are made in order; each adds the paths it lists, from the
	// repository's folder. A file in no commit stays untracked.
	Commits [][]string `yaml:"commits"`
	// Changed files get one more line after the commits, as a change not
	// committed.
	Changed []string `yaml:"changed"`
	// Shallow replaces the repository with a clone of depth 1.
	Shallow bool `yaml:"shallow"`
}

// caseGit runs git for a case's repository with no user or system
// configuration, so no hook or signing key of the machine takes part.
func caseGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=SpecArch conformance", "GIT_AUTHOR_EMAIL=conformance@specarch.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00+0000",
		"GIT_COMMITTER_NAME=SpecArch conformance", "GIT_COMMITTER_EMAIL=conformance@specarch.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00+0000",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// makeRepository turns the case's folder into the repository it describes.
func makeRepository(t *testing.T, work string, r *caseRepository) {
	t.Helper()
	dir := filepath.Join(work, filepath.FromSlash(r.Folder))
	caseGit(t, dir, "init", "-q", "-b", "main")
	for i, paths := range r.Commits {
		caseGit(t, dir, append([]string{"add", "--"}, paths...)...)
		caseGit(t, dir, "commit", "-q", "--no-gpg-sign", "-m", fmt.Sprintf("Commit %d of the case", i+1))
	}
	for _, p := range r.Changed {
		f, err := os.OpenFile(filepath.Join(dir, filepath.FromSlash(p)), os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.WriteString("-- changed after the commit\n")
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	if r.Shallow {
		clone := dir + ".clone"
		caseGit(t, work, "clone", "-q", "--depth", "1", "file://"+dir, clone)
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(clone, dir); err != nil {
			t.Fatal(err)
		}
	}
}

// builtPlugins is the folder the repository's own plug-ins are built into
// for the cases that name them.
var builtPlugins struct {
	sync.Mutex
	dir   string
	built map[string]bool
}

// buildPlugin builds cmd/specarch-gen-<target> once and returns its folder.
// It runs before the case changes its working folder.
func buildPlugin(t *testing.T, target string) string {
	t.Helper()
	builtPlugins.Lock()
	defer builtPlugins.Unlock()
	if builtPlugins.dir == "" {
		dir, err := os.MkdirTemp("", "specarch-plugins-")
		if err != nil {
			t.Fatal(err)
		}
		builtPlugins.dir, builtPlugins.built = dir, map[string]bool{}
	}
	if !builtPlugins.built[target] {
		src, err := filepath.Abs("../specarch-gen-" + target)
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("go", "build", "-o", filepath.Join(builtPlugins.dir, "specarch-gen-"+target), ".")
		cmd.Dir = src
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cannot build the plug-in for %s: %v\n%s", target, err, out)
		}
		builtPlugins.built[target] = true
	}
	return builtPlugins.dir
}

// The conformance suite is the tests stage of SpecArch's own specification:
// each test folder holds test.yaml (the design test), case.yaml (the
// arguments, exit status and output) and the files the case runs on.
const conformanceDir = "../../spec/tests"

// TestConformance runs every case of the language-neutral conformance suite:
// the design tests of the commands. A case runs in a copy of its folder,
// without test.yaml; afterwards every file must equal the one in expected/
// when there is one there, and be unchanged otherwise.
func TestConformance(t *testing.T) {
	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		src := filepath.Join(conformanceDir, e.Name())
		t.Run(e.Name(), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(src, "case.yaml"))
			if os.IsNotExist(err) {
				t.Skip("not a test of a command")
			}
			if err != nil {
				t.Fatal(err)
			}
			var c conformanceCase
			if err := yaml.Unmarshal(raw, &c); err != nil {
				t.Fatal(err)
			}
			inputs := readTree(t, src, "case.yaml", "test.yaml", "expected")
			expected := readTree(t, filepath.Join(src, "expected"))
			work := t.TempDir()
			for name, content := range inputs {
				p := filepath.Join(work, name)
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				mode := os.FileMode(0o644)
				if strings.HasPrefix(name, "plugins/") {
					mode = 0o755 // a plug-in the case puts on PATH
				}
				if err := os.WriteFile(p, content, mode); err != nil {
					t.Fatal(err)
				}
			}
			if c.Repository != nil {
				makeRepository(t, work, c.Repository)
				inputs = readTree(t, work) // what the run must leave as it found it
			}
			var built string
			for _, target := range c.Plugins {
				built = buildPlugin(t, target)
			}
			gitDir := ""
			if c.Repository != nil {
				gitPath, err := exec.LookPath("git")
				if err != nil {
					t.Fatal(err)
				}
				gitDir = filepath.Dir(gitPath)
			}
			t.Chdir(work)
			switch _, err := os.Stat(filepath.Join(work, "plugins")); {
			case err == nil:
				t.Setenv("PATH", filepath.Join(work, "plugins")+string(os.PathListSeparator)+os.Getenv("PATH"))
			case built != "":
				t.Setenv("PATH", built)
			default:
				t.Setenv("PATH", filepath.Join(work, "no-plugins"))
			}
			if c.Repository != nil {
				// extract reads the case's repository with git; only git's
				// own folder is added.
				t.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+gitDir)
			}
			var stdout, stderr bytes.Buffer
			status := run(c.Arguments, &stdout, &stderr)
			t.Logf("standard error:\n%s", stderr.String())
			if status != c.ExitStatus {
				t.Errorf("exit status %d, want %d\nstandard error:\n%s", status, c.ExitStatus, stderr.String())
			}
			if got := stdout.String(); got != c.StandardOutput {
				t.Errorf("standard output differs\ngot:\n%s\nwant:\n%s", got, c.StandardOutput)
			}
			after := readTree(t, work)
			want := map[string][]byte{}
			for k, v := range inputs {
				want[k] = v
			}
			for k, v := range expected {
				want[k] = v
			}
			for name, content := range after {
				w, ok := want[name]
				switch {
				case !ok:
					t.Errorf("%s was written, and expected/ does not hold it", name)
				case !bytes.Equal(content, w):
					t.Errorf("%s differs from what the case expects\ngot:\n%s", name, content)
				}
			}
			for name := range want {
				if _, ok := after[name]; !ok {
					t.Errorf("%s is missing after the run", name)
				}
			}
		})
	}
}

// readTree reads every file under dir, keyed by its slash path, leaving out
// the top-level names given.
func readTree(t *testing.T, dir string, skip ...string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return out
	}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		for _, s := range skip {
			if rel == s {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir // a case's repository, made by the runner
			}
			return nil
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = content
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// designFile reads SpecArch's own specification, merged.
func designFile(t *testing.T) *yaml.Node {
	t.Helper()
	s := spec.Load("../../spec")
	if s.Root == nil {
		t.Fatal("spec/ does not load")
	}
	return s.Root
}

func child(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// TestCommandTestsHaveCases checks that every design test of a command has
// a case.yaml to run it with (a test marked not applicable needs none), and
// every case.yaml belongs to a test of a command.
func TestCommandTestsHaveCases(t *testing.T) {
	tests := child(designFile(t), "tests")
	for i := 0; tests != nil && i+1 < len(tests.Content); i += 2 {
		name := tests.Content[i].Value
		_, err := os.Stat(filepath.Join(conformanceDir, name, "case.yaml"))
		hasCase := err == nil
		isCommand := child(tests.Content[i+1], "command") != nil && child(tests.Content[i+1], "notApplicable") == nil
		switch {
		case isCommand && !hasCase:
			t.Errorf("spec/tests/%s is a test of a command but has no case.yaml", name)
		case !isCommand && hasCase:
			t.Errorf("spec/tests/%s has a case.yaml but is not a test of a command", name)
		}
	}
}

// TestRulesMatchDesign checks that the validator's rules are exactly the
// values of the design's Rule enum.
func TestRulesMatchDesign(t *testing.T) {
	var design []string
	for _, v := range child(child(child(designFile(t), "enums"), "Rule"), "enum").Content {
		design = append(design, v.Value)
	}
	var code []string
	for _, r := range validate.Rules {
		code = append(code, string(r))
	}
	sort.Strings(design)
	sort.Strings(code)
	if strings.Join(design, " ") != strings.Join(code, " ") {
		t.Errorf("rules differ\ndesign: %v\ncode:   %v", design, code)
	}
}
