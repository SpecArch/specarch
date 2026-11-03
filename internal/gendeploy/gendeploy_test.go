package gendeploy

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

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
	deploymentOf := func() map[string]any {
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
		"deployments": map[string]any{"a": deploymentOf(), "b": deploymentOf(), "c": deploymentOf()},
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
		"a/fresh.sh":            `"${API_TOKEN:-}"`,
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

// TestFreshScriptFindsTheMigrationsAndTheSeed runs a generated fresh.sh
// with a compose command that records what it is asked, in the layout an
// implementation file gives: the script, written in its deployment's
// folder under the output, must find the sql target's migrations and the
// seed target's seed.sql, load them in order, run the build and the
// suite in the project, and tear the project down.
// writeFreshScript generates the deploy target of a small specification
// with a compose deployment under fresh into a temporary project and
// returns the folder above the project, the project and the deploy output.
func writeFreshScript(t *testing.T) (root, project, out string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fresh.sh is a POSIX shell script")
	}
	root = t.TempDir()
	project = filepath.Join(root, "project")
	write := func(p, content string) { writeTestFile(t, p, content) }
	write(filepath.Join(project, "migrations", "0001_expand.sql"), "-- first\n")
	write(filepath.Join(project, "migrations", "0002_expand.sql"), "-- second\n")
	write(filepath.Join(project, "seed", "seed.sql"), "-- seed\n")
	spec := map[string]any{
		"info":         map[string]any{"name": "desk", "version": "1.0.0"},
		"roles":        map[string]any{"desk-staff": map[string]any{"description": "Staff."}, "auditor": map[string]any{"description": "An auditor."}},
		"environments": map[string]any{"development": map[string]any{"description": "A machine."}},
		"configuration": map[string]any{
			"apiToken":   map[string]any{"description": "A token.", "secret": true, "schema": map[string]any{"type": "string"}},
			"channelUrl": map[string]any{"description": "A URL.", "schema": map[string]any{"type": "string"}},
		},
	}
	impl := map[string]any{
		"targets": map[string]any{
			"sql":  map[string]any{"output": "../../../migrations", "dialect": "postgresql"},
			"seed": map[string]any{"output": "../../../seed"},
			"deploy": map[string]any{"output": "../../../deploy", "settings": map[string]any{
				"deployments": map[string]any{"local": map[string]any{"form": "compose"}},
				"images":      map[string]any{"api": "localhost/api:1", "db": "docker.io/library/postgres:16"},
				"fresh":       []any{"local"},
			}},
		},
		"tasks":   map[string]any{"build": map[string]any{"run": "echo built >build.mark"}},
		"testing": map[string]any{"run": "printf %s \"$CHANNEL_URL\" >test.mark"},
		"deployments": map[string]any{"local": map[string]any{
			"environment": "development",
			"servers": []any{
				map[string]any{"name": "api", "url": "http://localhost:8080"},
				map[string]any{"name": "db", "kind": "database", "url": "postgresql://localhost:5432"},
			},
			"links":         []any{map[string]any{"from": "api", "to": "db", "port": 5432}},
			"configuration": map[string]any{"channelUrl": "http://localhost:8081/?a=1&b=it's"},
		}},
	}
	out = filepath.Join(project, "deploy")
	r := &Request{Request: gensql.Request{Specarch: "1.0.0", Digest: "sha256:0", Target: "deploy", Root: filepath.Join(project, "spec", "specarch.yaml"), Specification: spec, Output: out,
		Implementations: []gensql.Implementation{{File: filepath.Join(project, "spec", "implementation", "go", "desk.go.specarch-implementation.yaml"), Content: impl}}}}
	data, _ := json.Marshal(r)
	req, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	resp := Generate(req)
	for _, d := range resp.Diagnostics {
		t.Errorf("unexpected diagnostic: %s", d.Message)
	}
	for _, f := range resp.Files {
		write(filepath.Join(out, filepath.FromSlash(f.Path)), f.Content)
	}
	return root, project, out
}

func writeTestFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFreshScriptFindsTheMigrationsAndTheSeed(t *testing.T) {
	root, project, out := writeFreshScript(t)
	write := func(p, content string) { writeTestFile(t, p, content) }
	calls, loaded := filepath.Join(root, "calls"), filepath.Join(root, "loaded")
	stub := filepath.Join(root, "compose-stub")
	write(stub, `echo "$*" >>"$CALLS"
case "$*" in *"exec -T db sh -c psql"*) cat >>"$LOADED" ;; esac
`)
	cmd := exec.Command("sh", filepath.Join(out, "local", "fresh.sh"), project)
	cmd.Env = append(os.Environ(), "COMPOSE=sh "+stub, "CALLS="+calls, "LOADED="+loaded, "DB_PASSWORD=throwaway", "API_TOKEN=token")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fresh.sh: %v\n%s\n%s", err, output, readFile(filepath.Join(out, "local", "fresh.sh")))
	}
	if got, _ := os.ReadFile(loaded); string(got) != "CREATE ROLE auditor NOLOGIN;\nCREATE ROLE desk_staff NOLOGIN;\n-- first\n-- second\n-- seed\n" {
		t.Errorf("fresh.sh loaded %q, want the roles, the two migrations in order and then the seed\n%s", got, output)
	}
	if got, _ := os.ReadFile(filepath.Join(project, "test.mark")); string(got) != "http://localhost:8081/?a=1&b=it's" {
		t.Errorf("the suite saw CHANNEL_URL as %q, want the env file's value as it stands\n%s", got, output)
	}
	for _, mark := range []string{"build.mark", "test.mark"} {
		if _, err := os.Stat(filepath.Join(project, mark)); err != nil {
			t.Errorf("fresh.sh did not run the command that writes %s in the project: %v\n%s", mark, err, output)
		}
	}
	got, _ := os.ReadFile(calls)
	if !strings.Contains(string(got), "down --volumes --remove-orphans") {
		t.Errorf("fresh.sh did not tear the project down; the compose calls:\n%s", got)
	}
	// Compose refuses a project name with anything but lowercase letters,
	// digits, hyphens and underscores.
	for _, call := range strings.Split(strings.TrimSpace(string(got)), "\n") {
		f := strings.Fields(call)
		if len(f) < 2 || f[0] != "-p" || !regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`).MatchString(f[1]) {
			t.Errorf("fresh.sh called compose without a project name Compose takes: %s", call)
		}
	}
}

// TestFreshScriptTearsDownOnASignal checks that an interrupt, a
// termination or a hang-up during a run takes the project down: dash runs
// no EXIT trap when a signal ends the shell, so the script turns each into
// an exit with the signal's status. The script runs under sh and, where
// the host has it, dash.
func TestFreshScriptTearsDownOnASignal(t *testing.T) {
	root, project, out := writeFreshScript(t)
	shells := []string{"sh"}
	if _, err := exec.LookPath("dash"); err == nil {
		shells = append(shells, "dash")
	}
	signals := []struct {
		name   string
		signal syscall.Signal
		status int
	}{{"INT", syscall.SIGINT, 130}, {"TERM", syscall.SIGTERM, 143}, {"HUP", syscall.SIGHUP, 129}}
	for _, shell := range shells {
		for _, sg := range signals {
			t.Run(shell+"/"+sg.name, func(t *testing.T) {
				calls, started := filepath.Join(root, shell+"-"+sg.name+".calls"), filepath.Join(root, shell+"-"+sg.name+".up")
				stub := filepath.Join(root, "compose-stub-signal")
				// up marks that the project started, then holds the run.
				writeTestFile(t, stub, `echo "$*" >>"$CALLS"
case "$*" in *" up -d"*) : >"$STARTED"; sleep 1 ;; esac
`)
				cmd := exec.Command(shell, filepath.Join(out, "local", "fresh.sh"), project)
				cmd.Env = append(os.Environ(), "COMPOSE=sh "+stub, "CALLS="+calls, "STARTED="+started, "DB_PASSWORD=throwaway", "API_TOKEN=token")
				var output strings.Builder
				cmd.Stdout, cmd.Stderr = &output, &output
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				deadline := time.Now().Add(10 * time.Second)
				for {
					if _, err := os.Stat(started); err == nil {
						break
					}
					if time.Now().After(deadline) {
						_ = cmd.Process.Kill()
						_ = cmd.Wait()
						t.Fatalf("fresh.sh under %s did not reach compose up within 10 s\n%s", shell, output.String())
					}
					time.Sleep(20 * time.Millisecond)
				}
				if err := cmd.Process.Signal(sg.signal); err != nil {
					t.Fatal(err)
				}
				_ = cmd.Wait()
				if got := cmd.ProcessState.ExitCode(); got != sg.status {
					t.Errorf("fresh.sh under %s on %s exited %d (%s), want %d\n%s", shell, sg.name, got, cmd.ProcessState, sg.status, output.String())
				}
				if got := readFile(calls); !strings.Contains(got, "down --volumes --remove-orphans") {
					t.Errorf("fresh.sh under %s on %s did not tear the project down; the compose calls:\n%s\n%s", shell, sg.name, got, output.String())
				}
			})
		}
	}
}

func readFile(p string) string {
	b, _ := os.ReadFile(p)
	return string(b)
}

// TestCloudInitMapsTheDatabasePassword checks that a database host reads
// its password as <SERVER>_PASSWORD, as every other form does: the unit
// checks that line in secrets.env, then copies it under the image's own
// variable into a file of /run that the container reads. The two
// ExecStartPre commands are run as systemd would hand them to sh ($$ as
// $), with the host's folders moved into a temporary one.
func TestCloudInitMapsTheDatabasePassword(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the unit's commands are POSIX shell")
	}
	spec := map[string]any{
		"info":         map[string]any{"name": "desk", "version": "1.0.0"},
		"environments": map[string]any{"production": map[string]any{"description": "The hosts."}},
	}
	impl := map[string]any{
		"targets": map[string]any{"deploy": map[string]any{"output": "deploy", "settings": map[string]any{
			"deployments": map[string]any{"production": map[string]any{"form": "cloud-init"}},
			"images":      map[string]any{"db": "docker.io/library/postgres:16"},
		}}},
		"deployments": map[string]any{"production": map[string]any{
			"environment": "production",
			"servers":     []any{map[string]any{"name": "db", "kind": "database", "url": "postgresql://db.internal:5432"}},
		}},
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
	var unit string
	for _, f := range resp.Files {
		if f.Path == "production/db.cloud-init.yaml" {
			unit = f.Content
		}
	}
	for _, want := range []string{"holding DB_PASSWORD;", "EnvironmentFile=/run/desk/db.env", "s/^DB_PASSWORD=/POSTGRES_PASSWORD=/p"} {
		if !strings.Contains(unit, want) {
			t.Errorf("db.cloud-init.yaml does not hold %q:\n%s", want, unit)
		}
	}
	if strings.Contains(unit, "EnvironmentFile=/etc/desk/secrets.env") {
		t.Errorf("the database's container reads the whole secrets.env:\n%s", unit)
	}
	dir := t.TempDir()
	var pre []string
	for _, line := range strings.Split(unit, "\n") {
		line = strings.TrimSpace(line)
		if c, ok := strings.CutPrefix(line, "ExecStartPre=/bin/sh -c '"); ok {
			c = strings.TrimSuffix(c, "'")
			c = strings.ReplaceAll(c, "$$", "$")
			c = strings.ReplaceAll(c, "/etc/desk", dir+"/etc")
			c = strings.ReplaceAll(c, "/run/desk", dir+"/run")
			pre = append(pre, c)
		}
	}
	if len(pre) != 2 {
		t.Fatalf("want the check and the copy as two ExecStartPre lines, got %d:\n%s", len(pre), unit)
	}
	run := func() (string, error) {
		out, err := exec.Command("sh", "-c", pre[0]+" && "+pre[1]).CombinedOutput()
		return string(out), err
	}
	if err := os.MkdirAll(dir+"/etc", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/etc/secrets.env", []byte("OTHER=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := run(); err == nil || !strings.Contains(out, "DB_PASSWORD is not in") {
		t.Errorf("the unit started without DB_PASSWORD in secrets.env: %v %s", err, out)
	}
	if err := os.WriteFile(dir+"/etc/secrets.env", []byte("OTHER=1\nDB_PASSWORD=p&$w'd\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := run(); err != nil {
		t.Fatalf("the unit's commands: %v %s", err, out)
	}
	got, err := os.ReadFile(dir + "/run/db.env")
	if err != nil || string(got) != "POSTGRES_PASSWORD=p&$w'd\n" {
		t.Errorf("db.env holds %q (%v), want the password under the image's variable, as it stands", got, err)
	}
	if fi, err := os.Stat(dir + "/run/db.env"); err == nil && fi.Mode().Perm() != 0o600 {
		t.Errorf("db.env is %v, want 0600", fi.Mode().Perm())
	}
}
