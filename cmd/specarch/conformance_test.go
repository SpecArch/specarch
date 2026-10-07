package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
			t.Chdir(work)
			if _, err := os.Stat(filepath.Join(work, "plugins")); err == nil {
				t.Setenv("PATH", filepath.Join(work, "plugins")+string(os.PathListSeparator)+os.Getenv("PATH"))
			} else {
				t.Setenv("PATH", filepath.Join(work, "no-plugins"))
			}
			var stdout, stderr bytes.Buffer
			status := run(c.Arguments, &stdout, &stderr)
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
// a case.yaml to run it with, and every case.yaml belongs to a test of a
// command.
func TestCommandTestsHaveCases(t *testing.T) {
	tests := child(designFile(t), "tests")
	for i := 0; tests != nil && i+1 < len(tests.Content); i += 2 {
		name := tests.Content[i].Value
		_, err := os.Stat(filepath.Join(conformanceDir, name, "case.yaml"))
		hasCase := err == nil
		isCommand := child(tests.Content[i+1], "command") != nil
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
