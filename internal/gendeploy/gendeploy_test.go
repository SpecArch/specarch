package gendeploy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/gensql"
)

// TestSecretsAreReferencedNeverWritten checks the one rule every form
// keeps: a secret setting is named where the form reads it from the host,
// and no file holds a value for it, while a non-secret value is written.
func TestSecretsAreReferencedNeverWritten(t *testing.T) {
	spec := map[string]any{
		"info": map[string]any{"name": "desk", "version": "1.0.0"},
		"environments": map[string]any{
			"development": map[string]any{"description": "A machine."},
		},
		"configuration": map[string]any{
			"apiToken":  map[string]any{"description": "A token.", "secret": true, "schema": map[string]any{"type": "string"}},
			"dailyRate": map[string]any{"description": "A rate.", "secret": false, "schema": map[string]any{"type": "string"}},
		},
	}
	deploymentOf := func(form string) map[string]any {
		return map[string]any{
			"environment":   "development",
			"servers":       []any{map[string]any{"name": "api", "url": "http://localhost:8080"}},
			"configuration": map[string]any{"dailyRate": "0.50"},
		}
	}
	impl := map[string]any{
		"targets": map[string]any{"deploy": map[string]any{"output": "deploy", "settings": map[string]any{
			"deployments": map[string]any{"a": map[string]any{"form": "compose"}, "b": map[string]any{"form": "kubernetes"}, "c": map[string]any{"form": "cloud-init"}},
			"images":      map[string]any{"api": "localhost/api:1"},
			"fresh":       []any{"a"},
		}}},
		"tasks":       map[string]any{"build": map[string]any{"run": "make image"}},
		"testing":     map[string]any{"run": "make test"},
		"deployments": map[string]any{"a": deploymentOf("compose"), "b": deploymentOf("kubernetes"), "c": deploymentOf("cloud-init")},
	}
	r := &Request{Request: gensql.Request{Specarch: "1.0.0", Digest: "sha256:0", Target: "deploy", Root: "spec/specarch.yaml", Specification: spec, Output: "deploy",
		Implementations: []gensql.Implementation{{File: "spec/implementation/go/desk.go.specarch-implementation.yaml", Content: impl}}}}
	data, _ := json.Marshal(r)
	req, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	resp := Generate(req)
	for _, d := range resp.Diagnostics {
		t.Errorf("unexpected diagnostic: %s", d.Message)
	}
	want := map[string]string{
		"a/compose.yaml":        `API_TOKEN: "${API_TOKEN:?`,
		"a/a.env":               "# API_TOKEN: apiToken is a secret",
		"a/fresh.sh":            `"${API_TOKEN:?`,
		"b/kubernetes.yaml":     "key: API_TOKEN",
		"c/api.cloud-init.yaml": "EnvironmentFile=/etc/desk/secrets.env",
	}
	seen := map[string]bool{}
	for _, f := range resp.Files {
		seen[f.Path] = true
		for _, line := range strings.Split(f.Content, "\n") {
			if strings.Contains(line, "API_TOKEN=") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
				t.Errorf("%s writes a value for the secret: %s", f.Path, line)
			}
		}
		if w, ok := want[f.Path]; ok && !strings.Contains(f.Content, w) {
			t.Errorf("%s does not reference the secret as %q:\n%s", f.Path, w, f.Content)
		}
		if strings.HasSuffix(f.Path, ".env") && !strings.Contains(f.Content, "DAILY_RATE=0.50") {
			t.Errorf("%s does not write the non-secret value:\n%s", f.Path, f.Content)
		}
	}
	for p := range want {
		if !seen[p] {
			t.Errorf("%s was not written; written: %v", p, resp.Files)
		}
	}
}

// TestNoNameRefusesTheForms checks that a specification without info.name
// gets the runbooks and no form file, with the reason at info.
func TestNoNameRefusesTheForms(t *testing.T) {
	spec := map[string]any{
		"info":         map[string]any{"version": "1.0.0"},
		"environments": map[string]any{"development": map[string]any{"description": "A machine."}},
		"release":      map[string]any{"steps": []any{map[string]any{"name": "Ship", "action": "Ship it.", "check": "It runs."}}},
	}
	impl := map[string]any{
		"targets":     map[string]any{"deploy": map[string]any{"output": "deploy", "settings": map[string]any{"deployments": map[string]any{"a": map[string]any{"form": "compose"}}, "images": map[string]any{"api": "localhost/api:1"}}}},
		"deployments": map[string]any{"a": map[string]any{"environment": "development", "servers": []any{map[string]any{"name": "api", "url": "http://localhost:8080"}}}},
	}
	r := &Request{Request: gensql.Request{Specarch: "1.0.0", Target: "deploy", Root: "spec/specarch.yaml", Specification: spec, Output: "deploy",
		Implementations: []gensql.Implementation{{File: "spec/implementation/go/x.go.specarch-implementation.yaml", Content: impl}}}}
	data, _ := json.Marshal(r)
	req, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	resp := Generate(req)
	if len(resp.Diagnostics) != 1 || resp.Diagnostics[0].Path != "/info" || !strings.Contains(resp.Diagnostics[0].Message, "info.name") {
		t.Errorf("expected one error at /info naming info.name, got %v", resp.Diagnostics)
	}
	if len(resp.Files) != 1 || resp.Files[0].Path != "release.sh" {
		t.Errorf("expected release.sh alone, got %v", resp.Files)
	}
}
