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
			entry := strings.Join(lines[i:min(i+3, len(lines))], "\n")
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
