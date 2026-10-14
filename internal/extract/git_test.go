package extract

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// testGit runs git in dir as the conformance runner does, with a fixed
// author and date and no configuration of this machine, and returns what
// it printed.
func testGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=SpecArch test", "GIT_AUTHOR_EMAIL=test@specarch.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00+0000",
		"GIT_COMMITTER_NAME=SpecArch test", "GIT_COMMITTER_EMAIL=test@specarch.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00+0000")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, p, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gitlinkRepositories makes an outer repository holding the code's
// repository code as a gitlink, pinned at the code's first commit, which
// adds code/router/routes.ts; the code's second commit adds a file outside
// router, and its third changes router. The code is checked out at the
// pinned commit. It returns the outer folder and the three commits.
func gitlinkRepositories(t *testing.T) (string, []string) {
	t.Helper()
	outer := t.TempDir()
	outer, err := filepath.EvalSymlinks(outer)
	if err != nil {
		t.Fatal(err)
	}
	code := filepath.Join(outer, "code")
	writeFile(t, filepath.Join(code, "router", "routes.ts"), "export const routes = [];\n")
	testGit(t, code, "init", "-q", "-b", "main")
	var commits []string
	for i, step := range []struct{ file, text string }{
		{"router/routes.ts", "export const routes = [];\n"},
		{"README.md", "The code.\n"},
		{"router/later.ts", "export const later = 1;\n"},
	} {
		writeFile(t, filepath.Join(code, filepath.FromSlash(step.file)), step.text)
		testGit(t, code, "add", "--", step.file)
		testGit(t, code, "commit", "-q", "--no-gpg-sign", "-m", "Step "+string(rune('1'+i)))
		commits = append(commits, testGit(t, code, "rev-parse", "HEAD"))
	}
	testGit(t, code, "checkout", "-q", commits[0])
	testGit(t, outer, "init", "-q", "-b", "main")
	testGit(t, outer, "-c", "advice.addEmbeddedRepo=false", "add", "--", "code")
	writeFile(t, filepath.Join(outer, "routes.json"), "{}\n")
	testGit(t, outer, "add", "--", "routes.json")
	testGit(t, outer, "commit", "-q", "--no-gpg-sign", "-m", "Pin the code and add the dump")
	return outer, commits
}

// TestDumpThroughGitlink checks the code's checkout against the commit the
// gitlink pins: a checkout moved ahead with the same files under the path
// reads as the pinned one, one with other files there is refused, and so
// is a pinned commit the code's history does not hold.
func TestDumpThroughGitlink(t *testing.T) {
	outer, commits := gitlinkRepositories(t)
	dump := filepath.Join(outer, "routes.json")
	code := filepath.Join(outer, "code")
	read := func(repository string) (*Read, error) {
		r, _, err := openDump(dump, repository, "router", commits[0], "dump-routes.sh")
		return r, err
	}
	refused := func(err error, want string) {
		t.Helper()
		var refusal *Refusal
		if !errors.As(err, &refusal) {
			t.Fatalf("got %v, want a refusal saying %q", err, want)
		}
		if !strings.Contains(refusal.Reason, want) {
			t.Errorf("refusal %q does not say %q", refusal.Reason, want)
		}
	}

	r, err := read("code")
	if err != nil {
		t.Fatalf("pinned checkout: %v", err)
	}
	if r.Commit != commits[0] || r.Repository.Root != code {
		t.Errorf("read commit %s in %s, want %s in %s", r.Commit, r.Repository.Root, commits[0], code)
	}

	testGit(t, code, "checkout", "-q", commits[1])
	if r, err := read("code"); err != nil {
		t.Errorf("checkout moved ahead outside router: %v", err)
	} else if r.Commit != commits[0] {
		t.Errorf("checkout moved ahead outside router: commit %s, want %s", r.Commit, commits[0])
	}

	testGit(t, code, "checkout", "-q", commits[2])
	_, err = read("code")
	refused(err, "is checked out at commit "+commits[2])

	testGit(t, code, "checkout", "-q", commits[0])
	writeFile(t, filepath.Join(code, "router", "routes.ts"), "export const routes = [1];\n")
	_, err = read("code")
	refused(err, "changes not committed")
	testGit(t, code, "checkout", "-q", "--", "router/routes.ts")

	for _, bad := range []string{"../code", "/code", "code/", "./code", "."} {
		_, err = read(bad)
		refused(err, "is not a folder's path from the root of the repository holding the dump")
	}
	_, err = read("elsewhere")
	refused(err, "does not hold")

	// Pin a commit the code's history does not hold.
	missing := strings.Repeat("ab", 20)
	testGit(t, outer, "update-index", "--cacheinfo", "160000,"+missing+",code")
	testGit(t, outer, "commit", "-q", "--no-gpg-sign", "-m", "Pin a commit that is not there")
	_, err = read("code")
	refused(err, "that commit is not in its history")
}
