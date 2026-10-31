package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/capability"
)

// serve runs specarch mcp on the lines given and decodes each answer.
func serve(t *testing.T, lines ...string) []map[string]any {
	t.Helper()
	standardInput = strings.NewReader(strings.Join(lines, "\n") + "\n")
	defer func() { standardInput = os.Stdin }()
	var stdout, stderr bytes.Buffer
	if status := run([]string{"mcp"}, &stdout, &stderr); status != 0 {
		t.Fatalf("mcp exited %d: %s", status, stderr.String())
	}
	if stderr.Len() > 0 {
		t.Fatalf("mcp printed on standard error:\n%s", stderr.String())
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("mcp wrote a line that is not one JSON object: %v\n%s", err, line)
		}
		out = append(out, m)
	}
	return out
}

func call(name string, arguments map[string]any) string {
	data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": arguments}})
	return string(data)
}

// TestMCPToolsAreTheListing checks that tools/list lists exactly the tools
// capabilities --json lists but mcp's own, each with the same name, title,
// description, inputSchema and annotations (ADR-214).
func TestMCPToolsAreTheListing(t *testing.T) {
	noPlugins(t)
	answers := serve(t, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	result := answers[0]["result"].(map[string]any)
	var got []map[string]any
	for _, tool := range result["tools"].([]any) {
		got = append(got, tool.(map[string]any))
	}
	commands := definedCommands(t)
	var want []map[string]any
	for _, tool := range listing(commands, true).Tools {
		if tool.Name == "mcp" {
			continue
		}
		data, _ := json.Marshal(mcpTool{Name: tool.Name, Title: tool.Title, Description: tool.Description, InputSchema: tool.InputSchema, Annotations: tool.Annotations})
		var m map[string]any
		json.Unmarshal(data, &m)
		want = append(want, m)
	}
	g, _ := json.Marshal(got)
	w, _ := json.Marshal(want)
	if !bytes.Equal(g, w) {
		t.Fatalf("tools/list differs from capabilities --json's tools\ngot:  %.400s\nwant: %.400s", g, w)
	}
	if len(got) != len(commands)-1 {
		t.Fatalf("tools/list holds %d tools, and the program has %d commands but mcp", len(got), len(commands)-1)
	}
	for _, tool := range got {
		if strings.Contains(string(mustJSON(tool)), `"$ref"`) {
			t.Errorf("the inputSchema of %s holds a $ref, which a client cannot resolve", tool["name"])
		}
	}
}

func mustJSON(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}

// targetFirst are values that get each command that reads its target or
// source first past it, so that the order the server writes is tested.
var targetFirst = map[string]string{"document": "techspec", "generate": "sql", "extract": "outline", "facts": "javascript", "idioms diff": "audit-fields", "decide": "Q-1", "decision why": "ADR-001", "decision rests": "ADR-001", "try": "exitStatus"}

// misread is what a command says when the server put an input where the
// command does not read it.
var misread = regexp.MustCompile(`has no option|takes no argument|no command|is not a target name|has no target "-|does not read the source -|not given together`)

// TestMCPCallEveryTool calls every tool with every input given, in a
// folder of its own, and checks that each command reads the command line
// the server makes: no option refused, no option read as a target.
func TestMCPCallEveryTool(t *testing.T) {
	noPlugins(t)
	commands := definedCommands(t)
	for _, tool := range listing(commands, false).Tools {
		if tool.Name == "mcp" {
			continue
		}
		c, ok := toolCommand(tool, commands)
		if !ok {
			t.Fatalf("no command runs the tool %s", tool.Name)
		}
		input := map[string]any{}
		for _, a := range c.Arguments {
			v := "no-such-folder"
			if first, ok := targetFirst[c.Name]; ok && a.Name == c.Arguments[0].Name {
				v = first
			}
			if a.Repeatable {
				input[a.Name] = []any{v}
			} else {
				input[a.Name] = v
			}
		}
		for _, o := range c.Options {
			switch {
			case o.Boolean():
				input[o.Name] = o.Name == "dry-run"
			case o.Repeatable():
				input[o.Name] = []any{"x"}
			default:
				input[o.Name] = "x"
			}
		}
		if c.Name == "skill" {
			delete(input, "dry-run") // skill takes --check or --dry-run; check is false
		}
		args, err := capability.CommandLine(tool, c, input)
		if err != nil {
			t.Errorf("%s: %v", tool.Name, err)
			continue
		}
		t.Chdir(t.TempDir())
		var stdout, stderr bytes.Buffer
		mcpRunJSON(args, &stdout, &stderr)
		if m := misread.FindString(stdout.String() + stderr.String()); m != "" {
			t.Errorf("specarch %s: the command did not read the command line the server made (%q):\n%s%s", strings.Join(args, " "), m, stdout.String(), stderr.String())
		}
	}
}

// TestMCPCommandLineOrder checks the order of the command line: the
// arguments not repeated, the options, then the repeated arguments, and a
// target a generate tool fixes.
func TestMCPCommandLineOrder(t *testing.T) {
	commands := definedCommands(t)
	var generate capability.Command
	for _, c := range commands {
		if c.Name == "generate" {
			generate = c
		}
	}
	tool := capability.ToolOf(generate)
	args, err := capability.CommandLine(tool, generate, map[string]any{"target": "sql", "paths": []any{"a", "b"}, "out": "o", "estate": []any{"e1", "e2"}, "unapproved": true, "dry-run": false})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(args, " "), "generate sql --out o --estate e1 --estate e2 --unapproved a b"; got != want {
		t.Fatalf("the command line is %q, want %q", got, want)
	}
	found := []capability.Found{{Capabilities: &capability.Plugin{Plugin: "specarch-gen-sql", Target: "sql"}}}
	fixed := capability.GenerateTools(generate, found)[0]
	args, err = capability.CommandLine(fixed, generate, map[string]any{"paths": []any{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(args, " "), "generate sql a"; got != want {
		t.Fatalf("the command line of %s is %q, want %q", fixed.Name, got, want)
	}
	if _, err := capability.CommandLine(fixed, generate, map[string]any{"target": "openapi", "paths": []any{"a"}}); err == nil {
		t.Fatalf("%s took a target, which it fixes", fixed.Name)
	}
}

// TestMCPInputErrors checks that an input a command line cannot carry is
// answered as a tool error naming it, before anything runs.
func TestMCPInputErrors(t *testing.T) {
	noPlugins(t)
	t.Chdir(t.TempDir())
	cases := []struct {
		line, says string
	}{
		{call("init", map[string]any{"folder": "notes", "name": "x"}), `init takes no input "name"`},
		{call("init", map[string]any{}), `init needs the input "folder"`},
		{call("init", map[string]any{"folder": 3}), `the input "folder" of init is a string, and was given a number`},
		{call("init", map[string]any{"folder": "notes", "dry-run": "yes"}), `the input "dry-run" of init is true or false, and was given a string`},
		{call("validate", map[string]any{"paths": "spec"}), `the input "paths" of validate is a list of strings, and was given a string`},
		{call("init", map[string]any{"folder": "--title"}), `which starts with -`},
	}
	for _, c := range cases {
		answer := serve(t, c.line)[0]
		result, ok := answer["result"].(map[string]any)
		if !ok || result["isError"] != true {
			t.Errorf("%s was answered %v, want a tool error", c.line, answer)
			continue
		}
		text := result["content"].([]any)[0].(map[string]any)["text"].(string)
		if !strings.Contains(text, c.says) || !strings.HasSuffix(text, "nothing was run") {
			t.Errorf("%s was answered %q, want it to say %q and that nothing was run", c.line, text, c.says)
		}
	}
	if _, err := os.Stat("notes"); err == nil {
		t.Fatal("a call answered as an input error wrote notes/")
	}
}

// TestMCPAnswersEveryRevision checks that initialize answers the revision
// the client asks for when the server speaks it, and its own otherwise.
func TestMCPAnswersEveryRevision(t *testing.T) {
	noPlugins(t)
	for asked, want := range map[string]string{"2025-11-25": "2025-11-25", "2025-06-18": "2025-06-18", "2025-03-26": "2025-03-26", "2024-11-05": "2024-11-05", "2026-07-28": mcpRevision, "": mcpRevision} {
		line := `{"jsonrpc":"2.0","id":"a","method":"initialize","params":{"protocolVersion":"` + asked + `","capabilities":{}}}`
		answer := serve(t, line)[0]
		if answer["id"] != "a" {
			t.Errorf("initialize was answered with id %v, want the request's, a", answer["id"])
		}
		if got := answer["result"].(map[string]any)["protocolVersion"]; got != want {
			t.Errorf("initialize asking for %q was answered %v, want %s", asked, got, want)
		}
	}
}

// TestSkillFrontMatter checks SKILL.md's front matter as the Agent Skills
// format asks: YAML holding the name, the folder's own, lower case with
// hyphens and at most 64 characters, and a description of at most 1024.
func TestSkillFrontMatter(t *testing.T) {
	noPlugins(t)
	commands := definedCommands(t)
	files := capability.Skill(listing(commands, false), commands)
	text := files[0].Text
	rest, ok := strings.CutPrefix(text, "---\n")
	front, _, ok2 := strings.Cut(rest, "\n---\n")
	if files[0].Path != "SKILL.md" || !ok || !ok2 {
		t.Fatalf("the skill's first file is %s, without front matter between --- lines", files[0].Path)
	}
	var meta map[string]any
	if err := yaml.Unmarshal([]byte(front), &meta); err != nil {
		t.Fatalf("SKILL.md's front matter is not YAML: %v\n%s", err, front)
	}
	name, _ := meta["name"].(string)
	description, _ := meta["description"].(string)
	if name != capability.SkillName || len(name) > 64 || !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(name) {
		t.Errorf("the skill's name is %q", name)
	}
	if description == "" || len(description) > 1024 {
		t.Errorf("the skill's description is %d characters; it must be 1 to 1024", len(description))
	}
	if len(files) != len(commands)+1 {
		t.Errorf("the skill holds %d files, and the program has %d commands", len(files), len(commands))
	}
	for _, c := range commands {
		if !strings.Contains(text, "("+capability.ReferencePath(c)+")") {
			t.Errorf("SKILL.md does not link the reference of %s", c.Name)
		}
	}
}

// TestSkillFolder checks that the repository's skills/specarch is the
// skill the program writes; -update writes it again.
func TestSkillFolder(t *testing.T) {
	noPlugins(t)
	out, err := filepath.Abs("../../skills")
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"skill", "--out", out, "--check"}
	if *update {
		args = args[:3]
	}
	var stdout, stderr bytes.Buffer
	if status := run(args, &stdout, &stderr); status != 0 {
		t.Fatalf("skills/specarch is not the skill specarch skill writes; run go test ./cmd/specarch -run TestSkillFolder -update\n%s%s", stdout.String(), stderr.String())
	}
}
