package approval

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// TestCommitOnlyOfACommittedFolder checks that the commit an approval
// records is HEAD when every file under the specification's folder is
// committed, nothing with a note while one is not, and nothing without a
// note outside a repository.
func TestCommitOnlyOfACommittedFolder(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	outside := t.TempDir()
	if c, note := Commit(outside); c != "" || note != "" {
		t.Errorf("outside a repository: commit %q, note %q", c, note)
	}
	repo := t.TempDir()
	specDir := filepath.Join(repo, "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(specDir, "specarch.yaml"), []byte("specarch: \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c, note := Commit(specDir); c != "" || !strings.Contains(note, "1 file under") {
		t.Errorf("before the first commit: commit %q, note %q", c, note)
	}
	git("add", ".")
	git("commit", "-q", "-m", "first")
	head := git("rev-parse", "HEAD")
	if c, note := Commit(specDir); c != head || note != "" {
		t.Errorf("committed: commit %q (HEAD %s), note %q", c, head, note)
	}
	if err := os.WriteFile(filepath.Join(specDir, "specarch.yaml"), []byte("specarch: \"1.0.0\"\ninfo: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c, note := Commit(specDir); c != "" || !strings.Contains(note, "has changes not committed") {
		t.Errorf("edited: commit %q, note %q", c, note)
	}
}
