package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// packageFiles is a specification of one kept note, with an environment
// whose server the test package's requests go to, a golden test, a test
// that needs a record in place and a test an open question holds up.
var packageFiles = map[string]string{
	"spec/specarch.yaml": `specarch: "1.0.0"

info:
  title: Note keeper
  version: 0.1.0
  profile: lean

stages: [design, tests, deployment]

stakeholders:
  owner:
    description: The one who keeps notes.
`,
	"spec/design/design.yaml": `permissions:
  notes.write:
    description: Write and read a note.

roles:
  writer:
    description: Someone who keeps notes.
    permissions: [notes.write]

entities:
  Note:
    type: object
    primaryKey: [id]
    properties:
      id: { type: string, format: uuid }
      text: { type: string, minLength: 1, maxLength: 200 }
    required: [id, text]

paths:
  /notes:
    post:
      operationId: createNote
      summary: Keep a note
      permission: notes.write
      requestBody:
        content:
          application/json:
            schema: { $ref: '#/entities/Note' }
      responses:
        "201":
          description: The note is kept.
          content:
            application/json:
              schema: { $ref: '#/entities/Note' }
        "400":
          description: The note is not valid.
        "403":
          description: The caller may not keep notes.
  /notes/{noteId}:
    get:
      operationId: readNote
      summary: Read a note
      permission: notes.write
      parameters:
        - { name: noteId, in: path, required: true, schema: { type: string, format: uuid } }
      responses:
        "200":
          description: The note.
          content:
            application/json:
              schema: { $ref: '#/entities/Note' }
        "404":
          description: No note has that key.
`,
	"spec/deployment/environments.yaml": `environments:
  acceptance:
    description: The customer's acceptance host.
`,
	"spec/tests/keep-note-ok/test.yaml": `operation: createNote
level: system
scenario: golden
given: a writer
when: the writer keeps a note
then: it answers 201 with the note
fixture: { caller: writer }
input: { id: "6f1c2b9e-0000-4000-8000-000000000001", text: hello }
expect: { status: 201, body: { text: hello } }
`,
	"spec/tests/read-note-ok/test.yaml": `operation: readNote
level: system
scenario: golden
given: a kept note
when: the writer reads it
then: it answers 200 with the note
fixture:
  caller: writer
  Note: [{ id: "6f1c2b9e-0000-4000-8000-000000000002", text: kept }]
input: { noteId: "6f1c2b9e-0000-4000-8000-000000000002" }
expect: { status: 200, body: { text: kept } }
`,
	"spec/tests/keep-note-seen/test.yaml": `operation: createNote
level: system
scenario: golden
given: a writer
when: the writer keeps a note
`,
	"spec/tests/questions.yaml": `questions:
  Q-1:
    question: What does the writer see after keeping the note?
    kind: decision
    priority: must
    blocks: ["#/tests/keep-note-seen/then"]
    decidedBy: owner
`,
	"spec/notes.go.specarch-implementation.yaml": `specarchImplementation: "1.0.0"
info:
  title: Note keeper in Go
  version: 0.1.0
implements:
  file: specarch.yaml
  version: 0.1.0
stack:
  language: { name: Go, version: "1.26" }
deployments:
  acceptance:
    description: The customer's acceptance host.
    environment: acceptance
    servers:
      - { name: api, url: "http://127.0.0.1:1" }
      - { name: db, kind: database, url: "postgresql://127.0.0.1:5432" }
`,
}

// noteService is the fixture service a test package runs against: it
// keeps notes for a writer signed in with a token, refuses a note with a
// field missing or an id that is no UUID, and reads the records a setup
// program left in a file.
func noteService(t *testing.T, records string) *httptest.Server {
	uuid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	var mu sync.Mutex
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer writer-token" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/notes":
			var note map[string]any
			if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			id, _ := note["id"].(string)
			text, _ := note["text"].(string)
			if !uuid.MatchString(id) || text == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(note)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/notes/"):
			data, _ := os.ReadFile(records)
			var placed struct {
				Records map[string][]map[string]any `json:"records"`
			}
			json.Unmarshal(data, &placed)
			for _, n := range placed.Records["Note"] {
				if n["id"] == strings.TrimPrefix(r.URL.Path, "/notes/") {
					json.NewEncoder(w).Encode(n)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestTestPackageRuns makes a test package, runs it against the fixture
// service, and records the JUnit XML it writes: the golden test passes,
// the case that needs a record passes once a setup program places it and
// is skipped without one, the derived cases no test covers run as derive
// drafts them, the test an open question holds up is not in the package,
// and every testcase maps back to a test or an element.
func TestTestPackageRuns(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, packageFiles)
	specDir := filepath.Join(dir, "spec")
	pkg := filepath.Join(dir, "package")
	var out, errb bytes.Buffer
	if code := run([]string{"generate", "tests", "--package", "acceptance", "--unapproved", "--out", pkg, specDir}, &out, &errb); code != 0 {
		t.Fatalf("generate --package exited %d: %s%s", code, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "held back keep-note-seen, which Q-1 holds up") {
		t.Errorf("generate did not name the held test: %s", errb.String())
	}
	data, err := os.ReadFile(filepath.Join(pkg, "specarch-test-package.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"# Generated by specarch generate tests --package acceptance", "url: http://127.0.0.1:1", "name: keep-note-ok", "name: create-note-missing-text", "derived: true", "- name: keep-note-seen"} {
		if !strings.Contains(text, want) {
			t.Errorf("the package lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "postgresql") || strings.Contains(text, "writer-token") {
		t.Errorf("the package names a server that answers no request, or a credential:\n%s", text)
	}

	records := filepath.Join(dir, "placed.json")
	srv := noteService(t, records)
	defer srv.Close()
	callers := filepath.Join(dir, "callers.yaml")
	writeFiles(t, dir, map[string]string{"callers.yaml": "writer:\n  Authorization: Bearer writer-token\n"})
	results := filepath.Join(dir, "records", "evidence", "TR-1-junit.xml")

	// Without --setup, the case that needs a record is skipped with why.
	out.Reset()
	errb.Reset()
	code := run([]string{"run", "--base-url", srv.URL, "--callers", callers, "--results", results, pkg}, &out, &errb)
	junit, _ := os.ReadFile(results)
	if code != 0 {
		t.Fatalf("run exited %d: %s%s\n%s", code, out.String(), errb.String(), junit)
	}
	if !strings.Contains(string(junit), `<testcase name="read-note-ok"`) || !strings.Contains(string(junit), "give --setup &lt;program&gt;") {
		t.Errorf("the case that needs a record was not skipped with why:\n%s", junit)
	}

	// With a setup program that places the records, every case runs.
	setup := filepath.Join(dir, "setup.sh")
	if err := os.WriteFile(setup, []byte("#!/bin/sh\ncat > "+records+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	code = run([]string{"run", "--base-url", srv.URL, "--callers", callers, "--setup", setup, "--results", results, pkg}, &out, &errb)
	junit, _ = os.ReadFile(results)
	if code != 0 {
		t.Fatalf("run with --setup exited %d: %s%s\n%s", code, out.String(), errb.String(), junit)
	}
	cases, err := readJUnit(junit)
	if err != nil {
		t.Fatal(err)
	}
	outcomes := map[string]string{}
	for _, c := range cases {
		outcomes[c.name] = c.outcome
	}
	for name, want := range map[string]string{"keep-note-ok": "passed", "read-note-ok": "passed", "create-note-missing-id": "passed", "create-note-missing-text": "passed", "create-note-id-not-a-valid-uuid": "passed"} {
		if outcomes[name] != want {
			t.Errorf("%s: %q, want %q\n%s", name, outcomes[name], want, junit)
		}
	}
	if _, in := outcomes["keep-note-seen"]; in {
		t.Errorf("the held test ran:\n%s", junit)
	}

	// A wrong answer fails the case with what came back.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) }))
	defer bad.Close()
	out.Reset()
	errb.Reset()
	failedRun := filepath.Join(dir, "failed.xml")
	if code := run([]string{"run", "--base-url", bad.URL, "--callers", callers, "--results", failedRun, pkg}, &out, &errb); code != 1 {
		t.Errorf("a run with failures exited %d, want 1: %s%s", code, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "failed: keep-note-ok: answered 418, and the test expects 201") {
		t.Errorf("the failure does not say what came back: %s", out.String())
	}

	// The results map back: each testcase to its test or, for a derived
	// case, to its subject's element.
	out.Reset()
	errb.Reset()
	if code := run([]string{"log", "test-run", "--results", results, "--environment", "acceptance", "--at", "2026-11-04T10:00:00+07:00", "--by", "owner", specDir}, &out, &errb); code != 0 {
		t.Fatalf("log test-run exited %d: %s%s", code, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "0 unmapped") {
		t.Errorf("log test-run left results unmapped: %s%s", out.String(), errb.String())
	}
	record, _ := os.ReadFile(filepath.Join(dir, "records", "test-runs", "TR-1.yaml"))
	if !strings.Contains(string(record), "element: \"#/paths/~1notes\"") || !strings.Contains(string(record), "test: keep-note-ok") {
		t.Errorf("the record does not map the derived cases to their element and the tests to themselves:\n%s", record)
	}
}

// TestTestPackageRefusals checks what a package and a run refuse: a
// target other than tests, no --out, an environment the specification
// does not have, and a run that cannot choose a server.
func TestTestPackageRefusals(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, packageFiles)
	specDir := filepath.Join(dir, "spec")
	pkg := filepath.Join(dir, "package")
	for _, c := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"generate", "sql", "--package", "acceptance", "--out", pkg, specDir}, 2, "--package is a package of tests"},
		{[]string{"generate", "tests", "--package", "acceptance", specDir}, 2, "--package needs --out"},
		{[]string{"generate", "tests", "--package", "production", "--out", pkg, specDir}, 2, "production is not an environment of"},
		{[]string{"generate", "tests", "--package", "acceptance", "--out", pkg, specDir}, 1, "is not approved"},
	} {
		var out, errb bytes.Buffer
		if code := run(c.args, &out, &errb); code != c.code || !strings.Contains(errb.String(), c.want) {
			t.Errorf("%v: exit %d, want %d, and %q in: %s", c.args, code, c.code, c.want, errb.String())
		}
	}
	var out, errb bytes.Buffer
	if code := run([]string{"generate", "tests", "--package", "acceptance", "--unapproved", "--out", pkg, specDir}, &out, &errb); code != 0 {
		t.Fatalf("generate exited %d: %s", code, errb.String())
	}
	errb.Reset()
	if code := run([]string{"run", "--server", "web", "--results", filepath.Join(dir, "r.xml"), pkg}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "--server web is not a server of the package; its servers are api (acceptance, http://127.0.0.1:1)") {
		t.Errorf("an unknown server: exit %d: %s", code, errb.String())
	}
}

// scriptRepository is a git repository holding the files given, committed,
// as extract reads only what a commit names.
func scriptRepository(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, files)
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"commit", "-q", "--no-gpg-sign", "-m", "The scripts"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=SpecArch test", "GIT_AUTHOR_EMAIL=test@specarch.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00+0000",
			"GIT_COMMITTER_NAME=SpecArch test", "GIT_COMMITTER_EMAIL=test@specarch.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00+0000")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	return dir
}

// TestExtractedTestJoinsPackage follows a test script into the package:
// extract tests reads a scenario that names no expected result into a test
// inferred from it, with a question for its subject and one for the
// missing result; merge keeps it beside the design it exercises; the
// package holds it back while the questions are open; once both are
// decided and the keys written, it is in the package and passes against
// the fixture service.
func TestExtractedTestJoinsPackage(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{}
	for k, v := range packageFiles {
		if !strings.Contains(k, "tests/") && !strings.Contains(k, "implementation") {
			files[k] = v
		}
	}
	// The design as a reader's tree: it tracks origin and holds no test.
	files["spec/specarch.yaml"] = strings.Replace(strings.Replace(files["spec/specarch.yaml"], "  profile: lean\n", "  profile: lean\n  tracksOrigin: true\n", 1), "[design, tests, deployment]", "[design, deployment]", 1)
	writeFiles(t, dir, files)
	repo := scriptRepository(t, map[string]string{"notes.feature": "Feature: Notes\n\n  Scenario: Keep a long note\n    Given a signed-in writer\n    When the writer sends POST /notes with a text of 200 characters\n"})
	tree, merged, pkg := filepath.Join(dir, "tree"), filepath.Join(dir, "merged"), filepath.Join(dir, "package")
	step := func(args ...string) string {
		t.Helper()
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 0 {
			t.Fatalf("%s exited %d:\n%s%s", strings.Join(args[:2], " "), code, out.String(), errb.String())
		}
		return out.String() + errb.String()
	}
	said := step("extract", "tests", "--out", tree, filepath.Join(repo, "notes.feature"))
	if !strings.Contains(said, "wrote 1 test and 2 questions") {
		t.Errorf("extract tests did not ask for the subject and the missing result:\n%s", said)
	}
	test := filepath.Join(tree, "tests", "keep-a-long-note", "test.yaml")
	text, _ := os.ReadFile(test)
	for _, want := range []string{"origin: inferred", "when: the writer sends POST /notes with a text of 200 characters", "clause: line 3"} {
		if !strings.Contains(string(text), want) {
			t.Errorf("the extracted test lacks %q:\n%s", want, text)
		}
	}
	said = step("merge", "--out", merged, filepath.Join(dir, "spec"), tree)
	if !strings.Contains(said, "kept 1 test read from test scripts") {
		t.Errorf("merge did not keep the read test:\n%s", said)
	}
	writeFiles(t, merged, map[string]string{"notes.go.specarch-implementation.yaml": strings.Replace(packageFiles["spec/notes.go.specarch-implementation.yaml"], "http://127.0.0.1:1", "http://127.0.0.1:2", 1)})
	root := filepath.Join(merged, "specarch.yaml")
	data, _ := os.ReadFile(root)
	data = []byte(strings.Replace(string(data), "sources:\n", "sources:\n  owner-answers:\n    kind: interview\n    title: The owner's answers about the note tests\n    date: \"2026-11-04\"\n", 1))
	if err := os.WriteFile(root, data, 0o644); err != nil {
		t.Fatal(err)
	}

	said = step("generate", "tests", "--package", "acceptance", "--unapproved", "--out", pkg, merged)
	if !strings.Contains(said, "held back keep-a-long-note, which Q-1, Q-2 hold up") {
		t.Errorf("the package did not hold the extracted test back:\n%s", said)
	}

	// The owner decides each; what each decides is written before the next.
	edit := func(f func(string) string) {
		t.Helper()
		p := filepath.Join(merged, "tests", "keep-a-long-note", "test.yaml")
		text, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f(string(text))), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	long := strings.Repeat("n", 200)
	step("decide", "Q-1", "--by", "system-owner", "--answer", "createNote, as a writer, keeping a note of 200 characters.", "--source", "owner-answers", "--date", "2026-11-04", merged)
	edit(func(s string) string {
		s = strings.Replace(s, "level: system\n", "operation: createNote\nlevel: system\n", 1)
		s = strings.Replace(s, "origin: inferred\n", "origin: decided\ndecidedIn: ADR-001\n", 1)
		return s + "fixture: { caller: writer }\ninput: { id: \"6f1c2b9e-0000-4000-8000-000000000003\", text: \"" + long + "\" }\nexpect: { status: 201, body: { text: \"" + long + "\" } }\n"
	})
	said = step("generate", "tests", "--package", "acceptance", "--unapproved", "--out", pkg, merged)
	if !strings.Contains(said, "held back keep-a-long-note, which Q-2 holds up") {
		t.Errorf("the package did not hold the test back on its open question:\n%s", said)
	}
	step("decide", "Q-2", "--by", "system-owner", "--answer", "It answers 201 with the note.", "--source", "owner-answers", "--date", "2026-11-04", merged)
	edit(func(s string) string {
		s = strings.Replace(s, "origin: decided\ndecidedIn: ADR-001\n", "then: it answers 201 with the note\norigin: decided\ndecidedIn: ADR-002\n", 1)
		return s
	})
	said = step("generate", "tests", "--package", "acceptance", "--unapproved", "--out", pkg, merged)
	if strings.Contains(said, "held back keep-a-long-note") {
		t.Errorf("the decided test is still held back:\n%s", said)
	}
	srv := noteService(t, filepath.Join(dir, "none.json"))
	defer srv.Close()
	writeFiles(t, dir, map[string]string{"callers.yaml": "writer:\n  Authorization: Bearer writer-token\n"})
	results := filepath.Join(dir, "results.xml")
	var out, errb bytes.Buffer
	run([]string{"run", "--base-url", srv.URL, "--callers", filepath.Join(dir, "callers.yaml"), "--results", results, pkg}, &out, &errb)
	junit, _ := os.ReadFile(results)
	cases, err := readJUnit(junit)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range cases {
		if c.name == "keep-a-long-note" {
			found = true
			if c.outcome != "passed" {
				t.Errorf("keep-a-long-note: %s: %s", c.outcome, c.message)
			}
		}
	}
	if !found {
		t.Errorf("keep-a-long-note is not in the run:\n%s", junit)
	}
}
