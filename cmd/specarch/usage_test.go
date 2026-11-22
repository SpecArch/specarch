package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestUsageNamesExtractSources checks that help names every source extract
// reads, so nobody plans around a reader this build does not have.
func TestUsageNamesExtractSources(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run([]string{"--help"}, &stdout, &stderr); status != 0 {
		t.Fatalf("--help exited %d", status)
	}
	lines := strings.Split(stdout.String(), "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "specarch extract ") {
			entry := strings.Join(lines[i:min(i+4, len(lines))], "\n")
			for _, s := range extractSources {
				if !strings.Contains(entry, s) {
					t.Fatalf("the extract entry of the usage does not name the source %s:\n%s", s, entry)
				}
			}
			return
		}
	}
	t.Fatal("the usage has no extract line")
}

// TestDocumentTargetBeforeOptions checks the usage errors of a target
// that does not come first: the pdf target's message names books wherever
// pdf stands, and any other option before the target says the target
// comes first instead of calling the option a target.
func TestDocumentTargetBeforeOptions(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"document", "pdf", "spec"}, "has no target pdf: the four books are specarch document books"},
		{[]string{"document", "--out", "docs", "pdf", "spec"}, "has no target pdf: the four books are specarch document books"},
		{[]string{"document", "--check", "--out=docs", "pdf", "spec"}, "has no target pdf: the four books are specarch document books"},
		{[]string{"document", "--out", "docs", "techspec", "spec"}, "specarch document takes its target first, before the option --out: specarch document <target> [options] <folder>..."},
	} {
		var stdout, stderr bytes.Buffer
		status := run(c.args, &stdout, &stderr)
		if status != 2 || !strings.Contains(stderr.String(), c.want) {
			t.Errorf("%v exited %d with\n%s\nwant 2 and %q", c.args, status, stderr.String(), c.want)
		}
	}
}
