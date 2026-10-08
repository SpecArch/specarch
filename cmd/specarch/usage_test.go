package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestUsageMarksExtractNotBuilt checks that help says extract is designed,
// not built, so nobody plans around a verb this build answers with status 2.
func TestUsageMarksExtractNotBuilt(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run([]string{"--help"}, &stdout, &stderr); status != 0 {
		t.Fatalf("--help exited %d", status)
	}
	lines := strings.Split(stdout.String(), "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "specarch extract ") {
			entry := line
			if i+1 < len(lines) {
				entry += "\n" + lines[i+1]
			}
			if !strings.Contains(entry, "designed, not built") {
				t.Fatalf("the extract entry of the usage does not say it is designed, not built:\n%s", entry)
			}
			return
		}
	}
	t.Fatal("the usage has no extract line")
}
