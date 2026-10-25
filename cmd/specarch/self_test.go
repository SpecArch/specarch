package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SpecArch/specarch/internal/capability"
)

var update = flag.Bool("update", false, "write AGENTS.md's guide section from the program")

// noPlugins puts an empty folder and git's on PATH, so what the program
// says about itself does not depend on the plug-ins installed on this
// machine.
func noPlugins(t *testing.T) {
	t.Helper()
	path := t.TempDir()
	if git, err := exec.LookPath("git"); err == nil {
		path += string(os.PathListSeparator) + filepath.Dir(git)
	}
	t.Setenv("PATH", path)
}

func definedCommands(t *testing.T) []capability.Command {
	t.Helper()
	commands, err := ownCommands()
	if err != nil {
		t.Fatal(err)
	}
	return commands
}

// TestCommandsMatchDefinitions checks that the program runs exactly the
// commands spec/design/commands defines, so help and capabilities name
// every command and only those.
func TestCommandsMatchDefinitions(t *testing.T) {
	defined := map[string]bool{}
	for _, c := range definedCommands(t) {
		defined[c.Words()[0]] = true
	}
	for name := range commands {
		if !defined[name] {
			t.Errorf("the program runs %s, which spec/design/commands does not define; write spec/design/commands/%s.yaml", name, name)
		}
	}
	for name := range defined {
		if commands[name] == nil {
			t.Errorf("spec/design/commands defines %s, which the program does not run", name)
		}
	}
}

// firstArguments are values that get each command past its positional
// arguments to its options.
var firstArguments = map[string][]string{
	"document": {"techspec"},
	"generate": {"sql"},
	"extract":  {"outline"},
	"facts":    {"javascript", "no-such-folder", "no-such-dump.json"},
	"decide":   {"Q-1"},
	"check":    {"no-such-specification"},
	"diff":     {"no-such-old", "no-such-new"},
}

// refusesOption says whether a command's standard error refuses an option
// as one it does not take.
var refusesOption = regexp.MustCompile(`has no option|takes no argument|no command|takes --check or --dry-run`)

// TestDefinedOptionsAreTaken checks that every option a command's
// definition names is one the command takes.
func TestDefinedOptionsAreTaken(t *testing.T) {
	noPlugins(t)
	for _, c := range definedCommands(t) {
		for _, o := range c.Options {
			args := append(append([]string{}, c.Words()...), firstArguments[c.Words()[0]]...)
			args = append(args, "--"+o.Name)
			if !o.Boolean() {
				args = append(args, "x")
			}
			if len(c.Arguments) > 0 && c.Name != "help" {
				args = append(args, "no-such-folder")
			}
			var stdout, stderr bytes.Buffer
			run(args, &stdout, &stderr)
			if m := refusesOption.FindString(stderr.String()); m != "" {
				t.Errorf("specarch %s refuses --%s, which its definition names:\n%s", strings.Join(args, " "), o.Name, stderr.String())
			}
		}
	}
}

// commandFiles are the files of the program's commands: each option a
// file names as a string literal is one of these commands' options.
var commandFiles = map[string][]string{
	"approve.go":  {"approve"},
	"check.go":    {"check"},
	"decide.go":   {"decide"},
	"derive.go":   {"derive"},
	"diff.go":     {"diff"},
	"document.go": {"document", "generate"},
	"extract.go":  {"extract"},
	"facts.go":    {"facts"},
	"gaps.go":     {"gaps"},
	"generate.go": {"generate"},
	"idioms.go":   {"idioms", "idioms diff"},
	"merge.go":    {"merge"},
	"model.go":    {"model"},
	"upgrade.go":  {"upgrade"},
	"problems.go": {"document"},
	"site.go":     {"document"},
	"self.go":     {"help", "guide", "capabilities"},
	"main.go":     {"validate", "version"},
	"json.go":     {},
}

// globalOptions are taken before any command, or by every command.
var globalOptions = map[string]bool{"--json": true, "--help": true, "--agent-info": true}

var optionLiteral = regexp.MustCompile(`^--[a-z][a-z0-9-]*=?$`)

// TestTakenOptionsAreDefined checks that every option the program's code
// names is in the definition of a command of its file, so no option is
// taken that help and capabilities do not list.
func TestTakenOptionsAreDefined(t *testing.T) {
	byName := map[string]capability.Command{}
	for _, c := range definedCommands(t) {
		byName[c.Name] = c
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		owners, ok := commandFiles[file]
		if !ok {
			t.Errorf("%s is not in commandFiles; name the commands whose options it parses", file)
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			// The options of a program specarch runs, such as git, are
			// that program's.
			if call, ok := n.(*ast.CallExpr); ok {
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					if fun.Name == "git" {
						return false
					}
				case *ast.SelectorExpr:
					if fun.Sel.Name == "Command" || fun.Sel.Name == "CommandContext" {
						return false
					}
				}
			}
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil || !optionLiteral.MatchString(s) {
				return true
			}
			name := strings.TrimSuffix(s, "=")
			if globalOptions[name] {
				return true
			}
			for _, owner := range owners {
				if _, ok := byName[owner].Option(strings.TrimPrefix(name, "--")); ok {
					return true
				}
			}
			t.Errorf("%s takes %s, which the definition of none of %v names; add it under options in spec/design/commands", file, name, owners)
			return true
		})
	}
}

// jsonRuns are the command lines the --json test runs for every command:
// one that the command takes, where it needs no files, and one it refuses.
func jsonRuns(t *testing.T) [][]string {
	runs := [][]string{
		{"version"}, {"help"}, {"help", "generate"}, {"guide"}, {"guide", "--agent"},
		{"capabilities"}, {"validate", "../../examples/library-lending"},
		{"gaps", "../../examples/lending-desk"}, {"model", "../../examples/library-lending"},
		{"idioms", "../../examples/library-lending"},
	}
	for _, c := range definedCommands(t) {
		runs = append(runs, append(append([]string{}, c.Words()...), "--no-such-option"))
		args := append(append([]string{}, c.Words()...), firstArguments[c.Words()[0]]...)
		runs = append(runs, append(args, "no-such-folder"))
	}
	return runs
}

// TestJSONEveryCommand checks that every command run with --json prints
// one JSON object on standard output and nothing else, and on standard
// error nothing, or on exit 2 one error object with a code the listing
// names.
func TestJSONEveryCommand(t *testing.T) {
	noPlugins(t)
	codes := map[string]bool{}
	for _, e := range capability.ErrorCodes {
		codes[e.Code] = true
	}
	for _, args := range jsonRuns(t) {
		var stdout, stderr bytes.Buffer
		status := run(append([]string{"--json"}, args...), &stdout, &stderr)
		line := "specarch --json " + strings.Join(args, " ")
		out, ok := oneObject(stdout.Bytes())
		if !ok {
			t.Errorf("%s printed on standard output what is not one JSON object:\n%s", line, stdout.String())
			continue
		}
		if args[0] == "capabilities" || args[0] == "model" && status == 0 {
			continue
		}
		if out["specarchOutput"] != jsonFormat || int(out["exitStatus"].(float64)) != status {
			t.Errorf("%s printed an object without specarchOutput %s and exitStatus %d:\n%s", line, jsonFormat, status, stdout.String())
		}
		if status != 2 {
			if stderr.Len() > 0 {
				t.Errorf("%s exited %d and printed on standard error:\n%s", line, status, stderr.String())
			}
			continue
		}
		e, ok := oneObject(stderr.Bytes())
		if !ok || e["specarchError"] != jsonFormat {
			t.Errorf("%s exited 2 without one error object on standard error:\n%s", line, stderr.String())
			continue
		}
		if code, _ := e["code"].(string); !codes[code] || e["message"] == "" {
			t.Errorf("%s printed an error object whose code %q is not in errorCodes, or with no message:\n%s", line, code, stderr.String())
		}
	}
}

// oneObject decodes data as exactly one JSON object.
func oneObject(data []byte) (map[string]any, bool) {
	dec := json.NewDecoder(bytes.NewReader(data))
	var v map[string]any
	if err := dec.Decode(&v); err != nil || v == nil {
		return nil, false
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, false
	}
	return v, true
}

// TestCapabilitiesListing checks capabilities --json: one tool per
// command, whose inputSchema is draft 2020-12 and whose properties are
// exactly the command's arguments and options.
func TestCapabilitiesListing(t *testing.T) {
	noPlugins(t)
	var stdout, stderr bytes.Buffer
	if status := run([]string{"capabilities", "--json"}, &stdout, &stderr); status != 0 {
		t.Fatalf("capabilities --json exited %d: %s", status, stderr.String())
	}
	var l capability.Listing
	if err := json.Unmarshal(stdout.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	if l.Format != capability.Format || l.Version != version || len(l.Plugins) != 0 {
		t.Fatalf("the listing says format %q, version %q, %d plug-ins", l.Format, l.Version, len(l.Plugins))
	}
	tools := map[string]capability.Tool{}
	for _, tool := range l.Tools {
		tools[tool.Name] = tool
	}
	for _, c := range definedCommands(t) {
		tool, ok := tools[strings.ReplaceAll(c.Name, " ", "-")]
		if !ok {
			t.Errorf("the listing has no tool for %s", c.Name)
			continue
		}
		if tool.InputSchema["$schema"] != capability.SchemaDialect || tool.InputSchema["type"] != "object" {
			t.Errorf("the inputSchema of %s is not a draft 2020-12 object schema", c.Name)
		}
		var want, got []string
		for _, a := range c.Arguments {
			want = append(want, a.Name)
		}
		for _, o := range c.Options {
			want = append(want, o.Name)
		}
		for p := range tool.InputSchema["properties"].(map[string]any) {
			got = append(got, p)
		}
		sort.Strings(want)
		sort.Strings(got)
		if strings.Join(want, " ") != strings.Join(got, " ") {
			t.Errorf("the inputSchema of %s has %v, its definition %v", c.Name, got, want)
		}
		if len(tool.ExitCodes) == 0 || tool.Usage == "" || tool.Description == "" {
			t.Errorf("the tool %s has no exit codes, usage or description", c.Name)
		}
	}
}

// TestPluginsAnswerCapabilities builds every plug-in and checks that each
// answers --capabilities in the shape capabilities gathers, under its own
// name, saying what it writes and what it does not write.
func TestPluginsAnswerCapabilities(t *testing.T) {
	dirs, err := filepath.Glob("../specarch-gen-*")
	if err != nil {
		t.Fatal(err)
	}
	var dir string
	for _, d := range dirs {
		dir = buildPlugin(t, strings.TrimPrefix(filepath.Base(d), "specarch-gen-"))
	}
	found := capability.Gather(dir, 30*time.Second)
	if len(found) != len(dirs) {
		t.Fatalf("found %d plug-ins, built %d", len(found), len(dirs))
	}
	for _, f := range found {
		if f.Capabilities == nil {
			t.Errorf("%s: %s", f.Name(), f.Error)
			continue
		}
		p := f.Capabilities
		if p.Version != version || p.Summary == "" || len(p.Generates) == 0 || len(p.DoesNotGenerate) == 0 {
			t.Errorf("%s says version %q, summary %q, %d things it writes and %d it does not", f.Name(), p.Version, p.Summary, len(p.Generates), len(p.DoesNotGenerate))
		}
	}
}

// TestAgentsFile checks that AGENTS.md holds the agent guide the program
// prints, up to the plug-ins. -update writes it.
func TestAgentsFile(t *testing.T) {
	noPlugins(t)
	commands := definedCommands(t)
	guide := capability.AgentCore(listing(commands, false), commands)
	const begin, end = "<!-- specarch guide --agent: begin -->\n", "<!-- specarch guide --agent: end -->\n"
	path := "../../AGENTS.md"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	i, j := strings.Index(text, begin), strings.Index(text, end)
	if i < 0 || j < i {
		t.Fatalf("AGENTS.md has no section between %q and %q", begin, end)
	}
	if text[i+len(begin):j] == guide {
		return
	}
	if !*update {
		t.Fatalf("AGENTS.md's guide section is not what specarch guide --agent prints; run go test ./cmd/specarch -run TestAgentsFile -update")
	}
	if err := os.WriteFile(path, []byte(text[:i+len(begin)]+guide+text[j:]), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestHelpTopics checks that every help topic and every command's
// reference prints, its prose within 80 columns.
func TestHelpTopics(t *testing.T) {
	noPlugins(t)
	topics := helpTopics()
	for _, want := range []string{"format", "validate", "gaps", "generate", "extract", "upgrade", "plugins", "conventions", "examples"} {
		if topics[want] == "" {
			t.Errorf("docs/help has no %s.txt", want)
		}
	}
	for _, topic := range topicNames(topics, definedCommands(t)) {
		var stdout, stderr bytes.Buffer
		if status := run(append([]string{"help"}, strings.Fields(topic)...), &stdout, &stderr); status != 0 {
			t.Errorf("help %s exited %d: %s", topic, status, stderr.String())
		}
		for _, line := range strings.Split(stdout.String(), "\n") {
			// An indented line is an example, a command or a problem
			// line, copied as it is.
			if len(line) > 80 && !strings.HasPrefix(line, "  ") {
				t.Errorf("help %s has a line of %d columns: %s", topic, len(line), line)
			}
		}
	}
}

// TestDryRunWritesNothing runs every command that writes with --dry-run
// in one copy of a folder and without it in another, and checks that the
// dry run changed nothing and listed exactly the files the real run wrote.
func TestDryRunWritesNothing(t *testing.T) {
	noPlugins(t)
	cases := []struct {
		name  string
		from  string
		git   bool // commit the copy, for extract, which reads a commit
		setup [][]string
		args  []string
	}{
		{name: "document", from: "../../spec/tests/document-techspec-content", args: []string{"document", "techspec", "--out", "docs", "project"}},
		{name: "document problems", from: "../../examples/lending-desk", args: []string{"document", "problems", "--out", "out", "spec"}},
		{name: "derive", from: "../../spec/tests/derive-keeps-existing", args: []string{"derive", "project"}},
		{name: "approve", from: "../../spec/tests/approve-writes-record", args: []string{"approve", "--by", "owner", "--date", "2026-10-08", "project"}},
		{name: "decide", from: "../../spec/tests/decide-writes-decision", args: []string{"decide", "Q-1", "--by", "owner", "--answer", "Must.", "--source", "owner-answer", "--date", "2026-10-20", "project"}},
		{name: "decide removing a file", from: "../../spec/tests/decide-removes-questions-file", args: []string{"decide", "Q-3", "--by", "owner", "--answer", "No.", "--date", "2026-10-20", "project"}},
		{name: "extract", from: "../../examples/lending-desk", git: true, args: []string{"extract", "documents", "--out", "trees/manual", "sources/manual.md"}},
		{name: "merge", from: "../../examples/lending-desk", git: true,
			setup: [][]string{
				{"extract", "documents", "--out", "trees/manual", "sources/manual.md"},
				{"extract", "openapi", "--out", "trees/openapi", "sources/openapi/openapi.yaml"},
			},
			args: []string{"merge", "--out", "merged", "trees/manual", "trees/openapi"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := os.Stat(c.from); err != nil {
				t.Skipf("%s is not there: %v", c.from, err)
			}
			real, dry := copyFolder(t, c.from), copyFolder(t, c.from)
			for _, dir := range []string{real, dry} {
				if c.git {
					caseGit(t, dir, "init", "-q", "-b", "main")
					caseGit(t, dir, "add", "-A")
					caseGit(t, dir, "commit", "-q", "--no-gpg-sign", "-m", "The example")
				}
				for _, args := range c.setup {
					if status, out := runIn(t, dir, args); status != 0 {
						t.Fatalf("specarch %s exited %d:\n%s", strings.Join(args, " "), status, out)
					}
				}
			}
			before := snapshot(t, dry)
			realStatus, realOut := runIn(t, real, c.args)
			dryStatus, dryOut := runIn(t, dry, append(append([]string{}, c.args...), "--dry-run"))
			if realStatus != dryStatus {
				t.Fatalf("the run exited %d, the dry run %d:\n%s", realStatus, dryStatus, dryOut)
			}
			if after := snapshot(t, dry); !equalMaps(before, after) {
				t.Fatalf("the dry run changed files: %v", changedFiles(before, after))
			}
			wrote := changedFiles(before, snapshot(t, real))
			if len(wrote) == 0 {
				t.Fatalf("the run, which exited %d, wrote nothing, so the case checks nothing:\n%s", realStatus, realOut)
			}
			var listed []string
			for _, line := range strings.Split(dryOut, "\n") {
				for _, verb := range []string{"would write ", "would remove "} {
					if p, ok := strings.CutPrefix(line, verb); ok {
						listed = append(listed, relativeTo(t, dry, p))
					}
				}
			}
			sort.Strings(listed)
			if strings.Join(listed, "\n") != strings.Join(wrote, "\n") {
				t.Fatalf("the run wrote\n%s\nthe dry run listed\n%s\nits output:\n%s\nthe run's output:\n%s", strings.Join(wrote, "\n"), strings.Join(listed, "\n"), dryOut, realOut)
			}
		})
	}
}

// copyFolder copies from into a temporary folder and returns the copy.
func copyFolder(t *testing.T, from string) string {
	t.Helper()
	to := t.TempDir()
	if err := os.CopyFS(to, os.DirFS(from)); err != nil {
		t.Fatal(err)
	}
	return to
}

// runIn runs the program in dir, its standard output and error together.
func runIn(t *testing.T, dir string, args []string) (int, string) {
	t.Helper()
	t.Chdir(dir)
	var out bytes.Buffer
	status := run(args, &out, &out)
	return status, out.String()
}

// relativeTo is a listed path relative to dir, which an absolute one is
// made, both read through their symbolic links.
func relativeTo(t *testing.T, dir, p string) string {
	t.Helper()
	if !filepath.IsAbs(p) {
		return filepath.ToSlash(filepath.Clean(p))
	}
	d, err1 := filepath.EvalSymlinks(dir)
	q, err2 := filepath.EvalSymlinks(filepath.Dir(p))
	if err1 != nil || err2 != nil {
		q = filepath.Dir(p)
		d = dir
	}
	rel, err := filepath.Rel(d, filepath.Join(q, filepath.Base(p)))
	if err != nil {
		return p
	}
	return filepath.ToSlash(rel)
}

// snapshot is every file under dir but git's own, by its path.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		files[filepath.ToSlash(rel)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func equalMaps(a, b map[string]string) bool {
	return len(changedFiles(a, b)) == 0
}

// changedFiles are the files of after that are new or differ from before.
func changedFiles(before, after map[string]string) []string {
	var out []string
	for p, data := range after {
		if old, ok := before[p]; !ok || old != data {
			out = append(out, p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}
