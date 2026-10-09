package genbpmn

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"
)

const cases = "../../spec/tests"

// roundTrips are the extract cases whose input is a generate case's
// output, and the workflow both are about.
var roundTrips = []struct{ generate, extract, project, file, workflow string }{
	{"generate-bpmn", "extract-workflows-reads-generated-bpmn", "project/specarch.yaml", "waiver.bpmn", "waiver"},
	{"generate-bpmn-approved", "extract-workflows-reads-generated-last-approval", "project/specarch.yaml", "reinstate.bpmn", "reinstate"},
}

// TestRoundTrip checks that what generate bpmn writes is what extract
// workflows reads in its cases, and that extract gives back the workflow
// the file was generated from: everything BPMN holds of it, and the
// operations by name in the questions for the specification's tree.
func TestRoundTrip(t *testing.T) {
	for _, rt := range roundTrips {
		written, err := os.ReadFile(filepath.Join(cases, rt.generate, "expected/out", rt.file))
		if err != nil {
			t.Fatal(err)
		}
		read, err := os.ReadFile(filepath.Join(cases, rt.extract, "repo/workflows", rt.file))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(written, read) {
			t.Errorf("%s: the file extract reads is not the file generate writes in %s; copy it again", rt.extract, rt.generate)
		}
		original := workflowOf(t, filepath.Join(cases, rt.generate, rt.project), rt.workflow)
		extracted := workflowOf(t, filepath.Join(cases, rt.extract, "expected/spec/design/workflows.yaml"), rt.workflow)
		if got, want := held(extracted), held(original); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: extract gives %v, and the specification says %v", rt.workflow, got, want)
		}
		named := map[string]bool{}
		for _, q := range obj(load(t, filepath.Join(cases, rt.extract, "expected/spec/design/questions.yaml"))["questions"]) {
			if n := text(obj(q)["names"]); n != "" {
				named[n] = true
			}
		}
		want := []string{text(original["trigger"])}
		for _, s := range list(original["steps"]) {
			if op := text(obj(s)["operation"]); op != "" {
				want = append(want, op)
			}
		}
		for _, op := range want {
			if !named[op] {
				t.Errorf("%s: no question of the extracted tree names operation %s", rt.workflow, op)
			}
		}
	}
}

// held is what BPMN holds of a workflow.
func held(w map[string]any) []any {
	out := []any{w["description"]}
	for _, s := range list(w["steps"]) {
		st := obj(s)
		out = append(out, []any{st["name"], st["kind"], st["approvers"], st["deadline"], st["onDeadline"], st["escalateTo"]})
	}
	return out
}

func workflowOf(t *testing.T, file, name string) map[string]any {
	return obj(obj(load(t, file)["workflows"])[name])
}

func load(t *testing.T, file string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := yaml.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestComment(t *testing.T) {
	for in, want := range map[string]string{
		"plain":       "<!-- plain -->",
		"a -- b":      "<!-- a - - b -->",
		"a --- b":     "<!-- a - - - b -->",
		"run --check": "<!-- run - -check -->",
	} {
		if got := comment(in); got != want {
			t.Errorf("comment(%q) = %q, want %q", in, got, want)
		}
	}
}
