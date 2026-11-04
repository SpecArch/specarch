package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	// StandardError, when the case has the key, is the exact standard
	// error the run must print; a case without it does not check it.
	StandardError *string `yaml:"standardError"`
	// StandardInput, when the case has the key, is what the run reads on
	// standard input, for specarch mcp; a case without it gives none.
	StandardInput *string `yaml:"standardInput"`
	// Plugins names the targets whose plug-in from this repository the
	// case runs with, such as openapi for cmd/specarch-gen-openapi: each is
	// built once and put on PATH.
	Plugins []string `yaml:"plugins"`
	// Removed names the files of the case the run must remove, for decide,
	// which removes a questions file left with nothing but comments.
	Removed []string `yaml:"removed"`
	// Repository makes a folder of the case a git repository before the
	// run, for the cases of extract, which read a commit.
	Repository *caseRepository `yaml:"repository"`
	// Repositories are made in order, for a case that reads one repository
	// through another: a commit of a later one that lists the folder of an
	// earlier one records that repository's checked-out commit as a
	// gitlink, as a submodule does.
	Repositories []*caseRepository `yaml:"repositories"`
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
		caseGit(t, dir, append([]string{"-c", "advice.addEmbeddedRepo=false", "add", "--"}, paths...)...)
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

// plugins builds the repository's own plug-ins for one top-level test, each
// once, into a temporary folder of its own. The test that made it removes
// the folder when it ends, passed, failed or panicked: Go runs the cleanups
// of a panicking test and of every test above it before the program stops,
// so a panic in any case of the test removes it too. A folder is made per
// top-level test, never per run, because a folder shared by the whole run
// could only be removed after m.Run returns, which a panic never reaches.
// The folder keeps the specarch-plugins- prefix, so a sweep of the
// temporary folder still finds one left by a run that timed out or was
// killed, which runs no cleanups at all.
type plugins struct {
	sync.Mutex
	dir   string
	built map[string]bool
}

// newPlugins makes the plug-in folder of the top-level test t and removes
// it when t ends.
func newPlugins(t *testing.T) *plugins {
	t.Helper()
	dir, err := os.MkdirTemp("", "specarch-plugins-")
	if err != nil {
		t.Fatalf("cannot make the temporary folder the plug-ins are built into: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("cannot remove %s, the folder the plug-ins were built into: %v; remove it by hand", dir, err)
		}
	})
	return &plugins{dir: dir, built: map[string]bool{}}
}

// build builds cmd/specarch-gen-<target> once and returns the folder it is
// in. It runs before the case changes its working folder.
func (p *plugins) build(t *testing.T, target string) string {
	t.Helper()
	p.Lock()
	defer p.Unlock()
	if !p.built[target] {
		src, err := filepath.Abs("../specarch-gen-" + target)
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("go", "build", "-o", filepath.Join(p.dir, "specarch-gen-"+target), ".")
		cmd.Dir = src
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cannot build the plug-in for %s: %v\n%s", target, err, out)
		}
		p.built[target] = true
	}
	return p.dir
}

// pluginEndVariable makes TestPluginFolderRemoved the child it starts: the
// child builds a plug-in in a case, prints the folder, and then the case
// passes, fails or panics, as the variable says.
const pluginEndVariable = "SPECARCH_TEST_PLUGIN_FOLDER_END"

// TestPluginFolderRemoved proves that the folder the plug-ins are built
// into is gone once its test ends, whether its case passed, failed or
// panicked. A failing or panicking case fails or stops the binary that runs
// it, so each runs in the test binary started again as a child process,
// with its own TMPDIR; afterwards the folder the child named, which must be
// in that TMPDIR, is gone, and no specarch-plugins- folder is left there.
func TestPluginFolderRemoved(t *testing.T) {
	if end := os.Getenv(pluginEndVariable); end != "" {
		built := newPlugins(t)
		t.Run("case", func(t *testing.T) {
			fmt.Printf("plug-in folder: %s\n", built.build(t, "register"))
			switch end {
			case "fail":
				t.Fatal("the case fails, as asked")
			case "panic":
				panic("the case panics, as asked")
			}
		})
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	named := regexp.MustCompile(`plug-in folder: (\S+)`)
	for _, c := range []struct {
		end    string
		status int // the child's exit status: 1 for a failed test, 2 for a panic
	}{{"pass", 0}, {"fail", 1}, {"panic", 2}} {
		t.Run(c.end, func(t *testing.T) {
			tmp := t.TempDir()
			cmd := exec.Command(exe, "-test.run=^TestPluginFolderRemoved$", "-test.v")
			cmd.Env = append(os.Environ(), pluginEndVariable+"="+c.end, "TMPDIR="+tmp)
			out, err := cmd.CombinedOutput()
			status := 0
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				status = exit.ExitCode()
			} else if err != nil {
				t.Fatalf("cannot run the child test binary %s: %v", exe, err)
			}
			if status != c.status {
				t.Fatalf("the child exited with status %d, want %d:\n%s", status, c.status, out)
			}
			m := named.FindSubmatch(out)
			if m == nil {
				t.Fatalf("the child named no plug-in folder:\n%s", out)
			}
			dir := string(m[1])
			if !strings.HasPrefix(dir, tmp+string(filepath.Separator)) {
				t.Fatalf("the child built into %s, outside its TMPDIR %s", dir, tmp)
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Errorf("%s is still there after the child ended (stat: %v):\n%s", dir, err, out)
			}
			left, err := filepath.Glob(filepath.Join(tmp, "specarch-plugins-*"))
			if err != nil {
				t.Fatal(err)
			}
			if len(left) > 0 {
				t.Errorf("the child left %s", strings.Join(left, ", "))
			}
		})
	}
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
	built := newPlugins(t)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		src := filepath.Join(conformanceDir, e.Name())
		t.Run(e.Name(), func(t *testing.T) { runCase(t, src, built) })
	}
}

// runCase runs one case folder: it copies the inputs into a folder of its
// own, runs the program there and compares the run with the case, or, with
// recordVariable set, records the run into the case. An expected/ file
// equal to the case's input of the same name is refused before anything
// runs or is recorded: the case fails naming the file, and the run and the
// record are skipped, so a record never writes expected/ without it.
func runCase(t *testing.T, src string, builder *plugins) {
	t.Helper()
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
		c.Repositories = append([]*caseRepository{c.Repository}, c.Repositories...)
	}
	for _, r := range c.Repositories {
		makeRepository(t, work, r)
	}
	if len(c.Repositories) > 0 {
		inputs = readTree(t, work) // what the run must leave as it found it
	}
	refused := false
	for name, content := range expected {
		// expected/ holds what the run changes or adds; a file equal
		// to the case's input belongs with the inputs alone, since a
		// record run writes expected/ afresh from what the run
		// changed and would drop it.
		if in, ok := inputs[name]; ok && bytes.Equal(in, content) {
			t.Errorf("expected/%s equals the case's %s: expected/ holds only what the run changes or adds, and a record run (%s) drops a file equal to its input; make the input differ, or remove the file from expected/", name, name, recordVariable)
			refused = true
		}
	}
	if refused {
		return // neither run nor recorded: the record would rewrite expected/ without the file
	}
	var built string
	for _, target := range c.Plugins {
		built = builder.build(t, target)
	}
	gitDir := ""
	if len(c.Repositories) > 0 {
		gitPath, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		gitDir = filepath.Dir(gitPath)
	}
	caseDir, err := filepath.Abs(src)
	if err != nil {
		t.Fatal(err)
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
	if len(c.Repositories) > 0 {
		// extract reads the case's repository with git; only git's
		// own folder is added.
		t.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+gitDir)
	}
	var stdout, stderr bytes.Buffer
	input := ""
	if c.StandardInput != nil {
		input = *c.StandardInput
	}
	standardInput = strings.NewReader(input)
	status := run(c.Arguments, &stdout, &stderr)
	standardInput = os.Stdin
	t.Logf("standard error:\n%s", stderr.String())
	if os.Getenv(recordVariable) != "" {
		recordCase(t, caseDir, raw, status, stdout.String(), stderr.String(), inputs, readTree(t, work), c.Removed)
		return
	}
	if status != c.ExitStatus {
		t.Errorf("exit status %d, want %d\nstandard error:\n%s", status, c.ExitStatus, stderr.String())
	}
	if got := stdout.String(); got != c.StandardOutput {
		t.Errorf("standard output differs\ngot:\n%s\nwant:\n%s", got, c.StandardOutput)
	}
	if c.StandardError != nil && stderr.String() != *c.StandardError {
		t.Errorf("standard error differs\ngot:\n%s\nwant:\n%s", stderr.String(), *c.StandardError)
	}
	after := readTree(t, work)
	want := map[string][]byte{}
	for k, v := range inputs {
		want[k] = v
	}
	for k, v := range expected {
		want[k] = v
	}
	for _, name := range c.Removed {
		if _, ok := after[name]; ok {
			t.Errorf("%s is still there, and the case expects the run to remove it", name)
		}
		delete(want, name)
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
	for _, out := range treeOut(c) {
		// A tree a command writes passes validate on its own, with no
		// error: merge checks each tree before it merges it, a
		// question's pointer never points below a key the tree does
		// not have, where the trees disagree merge keeps one valid
		// form and asks, derive names a draft from its subject made
		// a valid name, and approve writes its record into a valid
		// tree (ADR-122).
		var vout, verr bytes.Buffer
		if status := run([]string{"validate", out}, &vout, &verr); status != 0 {
			t.Errorf("validate %s exits %d on the tree the case writes; every tree a command writes passes validate on its own\n%s%s", out, status, vout.String(), verr.String())
		}
	}
}

// caseVariable names a case folder for TestRecordRefusesExpectedEqualToInput
// run again as a child process: that process runs the folder as a case
// and nothing else.
const caseVariable = "SPECARCH_CONFORMANCE_CASE"

// TestRecordRefusesExpectedEqualToInput proves that a record run stops
// before it records when an expected/ file equals the case's input: the
// case fails, names the file, and leaves case.yaml and expected/ as they
// were. A failed case fails the test that runs it, so the case runs in the
// test binary started again as a child process, with the record variable
// set and the case folder named in caseVariable; this test is that child
// when caseVariable is set.
func TestRecordRefusesExpectedEqualToInput(t *testing.T) {
	if dir := os.Getenv(caseVariable); dir != "" {
		runCase(t, dir, newPlugins(t))
		return
	}
	dir := t.TempDir()
	caseYAML := []byte("arguments: [version]\nexitStatus: 0\nstandardOutput: \"\"\n")
	input := []byte("the same bytes\n")
	files := map[string][]byte{
		"case.yaml":          caseYAML,
		"notes.txt":          input,
		"expected/notes.txt": input,
	}
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRecordRefusesExpectedEqualToInput$", "-test.v")
	cmd.Env = append(os.Environ(), caseVariable+"="+dir, recordVariable+"=1")
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if err == nil {
		t.Errorf("the record run passed; it must fail, since expected/notes.txt equals notes.txt\n%s", out)
	} else if !errors.As(err, &exit) {
		t.Fatalf("cannot run the test binary again: %v\n%s", err, out)
	}
	want := "expected/notes.txt equals the case's notes.txt"
	if !strings.Contains(string(out), want) {
		t.Errorf("the record run does not name the file; want %q in:\n%s", want, out)
	}
	if got := readTree(t, dir); len(got) != len(files) {
		t.Errorf("the record run changed the case's files: %d files, want %d", len(got), len(files))
	} else {
		for name, content := range files {
			if !bytes.Equal(got[name], content) {
				t.Errorf("the record run rewrote %s:\n%s", name, got[name])
			}
		}
	}
}

// recordVariable, set to anything, makes TestConformance write what each
// run gives into its case instead of comparing: for a change that moves the
// output of many cases at once, such as a version line, whose diff is then
// read like any other change.
const recordVariable = "SPECARCH_CONFORMANCE_RECORD"

// textBlock writes a key and a text as a YAML literal block, or as "" when
// the text is empty.
func textBlock(key, text string) []string {
	if text == "" {
		return []string{key + `: ""`}
	}
	indicator := "|"
	switch {
	case !strings.HasSuffix(text, "\n"):
		indicator = "|-"
	case strings.HasSuffix(text, "\n\n"):
		indicator = "|+"
	}
	if strings.HasPrefix(text, " ") {
		indicator = "|2" + strings.TrimPrefix(indicator, "|")
	}
	block := []string{key + ": " + indicator}
	for _, l := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if l == "" {
			block = append(block, "")
		} else {
			block = append(block, "  "+l)
		}
	}
	return block
}

// recordCase writes a run into its case: the exit status and the standard
// output into case.yaml, and the standard error when it has that key, its
// other keys and comments kept, and every file the run changed or added,
// apart from those it removes, into expected/.
func recordCase(t *testing.T, dir string, raw []byte, status int, stdout, stderr string, inputs, after map[string][]byte, removed []string) {
	t.Helper()
	lines := strings.Split(string(raw), "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		var block []string
		switch {
		case strings.HasPrefix(l, "standardOutput:"):
			block = textBlock("standardOutput", stdout)
		case strings.HasPrefix(l, "standardError:"):
			block = textBlock("standardError", stderr)
		}
		switch {
		case strings.HasPrefix(l, "exitStatus:"):
			out = append(out, fmt.Sprintf("exitStatus: %d", status))
		case block != nil:
			out = append(out, block...)
			for i+1 < len(lines) && (strings.HasPrefix(lines[i+1], " ") || lines[i+1] == "" && i+2 < len(lines) && strings.HasPrefix(lines[i+2], " ")) {
				i++
			}
			if rest := lines[i+1:]; strings.TrimSpace(strings.Join(rest, "")) == "" && len(rest) > 1 {
				i = len(lines) - 2 // blank lines kept by |+ up to the end of the file
			}
		default:
			out = append(out, l)
		}
	}
	text := strings.Join(out, "\n")
	var was conformanceCase
	if yaml.Unmarshal(raw, &was) == nil && was.ExitStatus == status && was.StandardOutput == stdout && (was.StandardError == nil || *was.StandardError == stderr) {
		text = string(raw) // unchanged: keep the case as it is written
	}
	var check conformanceCase
	if err := yaml.Unmarshal([]byte(text), &check); err != nil || check.StandardOutput != stdout || check.ExitStatus != status || check.StandardError != nil && *check.StandardError != stderr {
		t.Fatalf("cannot record the case: case.yaml would not read back as the run (%v); record it by hand", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "case.yaml"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	gone := map[string]bool{}
	for _, r := range removed {
		gone[r] = true
	}
	if err := os.RemoveAll(filepath.Join(dir, "expected")); err != nil {
		t.Fatal(err)
	}
	for name, content := range after {
		if gone[name] || bytes.Equal(inputs[name], content) && inputs[name] != nil {
			continue
		}
		p := filepath.Join(dir, "expected", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// treeOut is the folders a case writes a tree into: the folder an extract
// or merge case writes, each folder a derive case writes its drafts into
// and each an approve case writes its record into, and the folder an init
// case writes its specification into; none for a case of another command
// or one that exits other than 0.
func treeOut(c conformanceCase) []string {
	if len(c.Arguments) == 0 || c.ExitStatus != 0 {
		return nil
	}
	switch c.Arguments[0] {
	case "extract", "merge":
		for i, a := range c.Arguments {
			if a == "--out" && i+1 < len(c.Arguments) {
				return []string{c.Arguments[i+1]}
			}
		}
	case "derive", "approve", "init":
		var out []string
		args := c.Arguments[1:]
		for i := 0; i < len(args); i++ {
			switch a := args[i]; {
			case a == "--dry-run":
				return nil // writes nothing
			case a == "--by" || a == "--date" || a == "--session" || a == "--title" || a == "--profile":
				i++ // approve's and init's options that take the next argument
			case !strings.HasPrefix(a, "-"):
				out = append(out, a)
			}
		}
		return out
	}
	return nil
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
