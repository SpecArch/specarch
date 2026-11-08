package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/generate"
)

// usageCommand is a command line of the usage text: its words up to the
// first option or argument, such as "plan", "log test-run" or "idioms diff".
var usageCommand = regexp.MustCompile(`(?m)^  specarch ([a-z][a-z-]*(?: [a-z][a-z-]*)*)`)

// codeSpan is a Markdown code span.
var codeSpan = regexp.MustCompile("`([^`]+)`")

// usageCommands are the commands the usage text lists, each once, in order.
func usageCommands() []string {
	var names []string
	seen := map[string]bool{}
	for _, m := range usageCommand.FindAllStringSubmatch(usage, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			names = append(names, m[1])
		}
	}
	return names
}

// readmeSection is the text of a README section from its heading line to
// the next heading of the same or a higher level.
func readmeSection(t *testing.T, readme, heading string) string {
	t.Helper()
	start := strings.Index(readme, "\n"+heading+"\n")
	if start < 0 {
		t.Fatalf("README.md has no heading %q", heading)
	}
	level := strings.Index(heading, " ")
	rest := readme[start+len(heading)+2:]
	for _, line := range strings.SplitAfter(rest, "\n") {
		if n := strings.Index(line, " "); n > 0 && n <= level && strings.Trim(line[:n], "#") == "" {
			return rest[:strings.Index(rest, line)]
		}
	}
	return rest
}

// tableRows are the cells of every table row in a text, the separator rows
// left out; a `\|` stays inside its cell, as Markdown reads it.
func tableRows(text string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "|") || strings.HasPrefix(line, "|---") {
			continue
		}
		line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
		cells := strings.Split(strings.ReplaceAll(line, `\|`, "\x00"), "|")
		for i, c := range cells {
			cells[i] = strings.TrimSpace(strings.ReplaceAll(c, "\x00", "|"))
		}
		rows = append(rows, cells)
	}
	return rows
}

// commandOf is the command a code span names: its words up to the first
// argument or option, so "`log test-run`" and "`extract <source>`" are
// log test-run and extract.
func commandOf(span string) string {
	var words []string
	for _, w := range strings.Fields(span) {
		if strings.HasPrefix(w, "<") || strings.HasPrefix(w, "[") || strings.HasPrefix(w, "-") {
			break
		}
		words = append(words, w)
	}
	return strings.Join(words, " ")
}

// exampleCommand is the longest command of the list an example runs.
func exampleCommand(example string, commands []string) string {
	best := ""
	for _, c := range commands {
		if (example == "specarch "+c || strings.HasPrefix(example, "specarch "+c+" ")) && len(c) > len(best) {
			best = c
		}
	}
	return best
}

// shellWords splits a command line as a shell does for plain words and
// single or double quotes, which is all the README's examples use.
func shellWords(line string) []string {
	var words []string
	var word strings.Builder
	inWord := false
	var quote rune
	for _, r := range line {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			word.WriteRune(r)
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ':
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		words = append(words, word.String())
	}
	return words
}

type readmeExample struct {
	command string
	line    string
}

// readmeCommands checks the Commands section of the README: one row for
// every command of the usage, each with an example of that command, one
// row for every document target, and, in Installing and running, one row
// for every plug-in. It returns the examples in the order of the rows.
func readmeCommands(t *testing.T) []readmeExample {
	t.Helper()
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	install := readmeSection(t, string(data), "## Installing and running")
	section := readmeSection(t, install, "### Commands")
	commands := usageCommands()
	if len(commands) < 30 {
		t.Fatalf("the usage text lists %d commands; the pattern that reads them no longer matches it", len(commands))
	}
	rows := tableRows(section)
	named := map[string]bool{}
	var examples []readmeExample
	for _, row := range rows {
		if len(row) < 3 || row[0] == "Command" {
			if len(row) >= 1 {
				if m := codeSpan.FindStringSubmatch(row[0]); m != nil {
					named["target "+m[1]] = true
				}
			}
			continue
		}
		m := codeSpan.FindStringSubmatch(row[0])
		if m == nil {
			t.Errorf("README.md, Commands: the row %q names no command in its first cell", strings.Join(row, " | "))
			continue
		}
		command := commandOf(m[1])
		named[command] = true
		found := false
		for _, span := range codeSpan.FindAllStringSubmatch(row[2], -1) {
			if !strings.HasPrefix(span[1], "specarch") {
				continue
			}
			if got := exampleCommand(span[1], commands); got != command {
				t.Errorf("README.md, Commands: the example %q in the row of %s runs %q", span[1], command, got)
			}
			found = true
			examples = append(examples, readmeExample{command, span[1]})
		}
		if !found {
			t.Errorf("README.md, Commands: the row of %s has no example; give one in the Example column", command)
		}
	}
	for _, c := range commands {
		if !named[c] {
			t.Errorf("README.md, Commands: the usage lists specarch %s and no table row names it; add a row with what it does and an example", c)
		}
	}
	for _, target := range generate.DocumentTargets {
		if !named["target "+target] {
			t.Errorf("README.md, Commands: the document target %s has no row in the table of document targets; add it with the file it writes", target)
		}
	}
	plugins, err := filepath.Glob("../specarch-gen-*")
	if err != nil || len(plugins) == 0 {
		t.Fatalf("no plug-in found under cmd/specarch-gen-* (%v)", err)
	}
	pluginRows := map[string]bool{}
	for _, row := range tableRows(install) {
		if m := codeSpan.FindStringSubmatch(row[0]); m != nil {
			pluginRows[m[1]] = true
		}
	}
	for _, dir := range plugins {
		if !pluginRows[filepath.Base(dir)] {
			t.Errorf("README.md, Installing and running: the plug-in %s has no row in the table of generators", filepath.Base(dir))
		}
	}
	return examples
}

// TestReadmeCommands checks that the README's Commands section names every
// command of the usage with an example, every document target and every
// plug-in, so the README cannot drift from the program.
func TestReadmeCommands(t *testing.T) {
	readmeCommands(t)
}

// readmeSkipped are the commands whose examples the test does not run: the
// generators are plug-ins (noPlugins takes them off PATH), run needs the
// package generate tests writes, mcp serves a client on standard input,
// and check reads route tables made at a commit of this repository's
// history, which the copy of examples/ does not have.
var readmeSkipped = map[string]bool{"generate": true, "generate tests": true, "run": true, "mcp": true, "check": true}

// TestReadmeExamples runs every example of the README's Commands section,
// in order, on a copy of examples/ committed to a git repository of its
// own (extract, facts and check name the commit they read), and fails on
// exit status 2, a usage error or a path the example names that is not
// there. Exit status 1 is a finding of the command (an open question, a
// document not current) and is left to the command's own tests.
func TestReadmeExamples(t *testing.T) {
	if testing.Short() {
		t.Skip("copies examples/ and runs every README example")
	}
	noPlugins(t)
	examples := readmeCommands(t)
	work := t.TempDir()
	if err := os.CopyFS(filepath.Join(work, "examples"), os.DirFS("../../examples")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.name=test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "examples"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	t.Chdir(work)
	for _, e := range examples {
		if readmeSkipped[e.command] {
			continue
		}
		words := shellWords(e.line)
		var stdout, stderr bytes.Buffer
		if code := run(words[1:], &stdout, &stderr); code == 2 {
			t.Errorf("README.md example %q exited 2:\n%s", e.line, stderr.String())
		}
	}
}
