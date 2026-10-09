package approval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDigestLeavesOutMarks(t *testing.T) {
	plain, marked, edited := t.TempDir(), t.TempDir(), t.TempDir()
	write := func(dir, text string) {
		if err := os.WriteFile(filepath.Join(dir, "specarch.yaml"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(plain, "info:\n  title: Case\n")
	write(marked, "info:\n  # specarch-problem: warning: rule: message [id]\n  title: Case\n")
	write(edited, "info:\n  title: Case two\n")
	digest := func(dir string) string {
		d, err := Digest(dir)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	if digest(plain) != digest(marked) {
		t.Error("a mark changed the digest; writing marks would void an approval")
	}
	if digest(plain) == digest(edited) {
		t.Error("an edit left the digest as it was")
	}
}
