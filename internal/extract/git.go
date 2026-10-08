// Package extract holds the readers of specarch extract. Each reads one
// surface of an existing system into a partial specification tree, at the
// commit that names what was read (docs/extraction.md, Building extract).
package extract

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Refusal is a source that cannot be read as the reader expects: no commit
// names it, or its content is not what the reader takes. It is exit status
// 1; any other error is status 2.
type Refusal struct{ Reason string }

func (r *Refusal) Error() string { return r.Reason }

func refuse(format string, args ...any) error {
	return &Refusal{Reason: fmt.Sprintf(format, args...)}
}

// git runs git in dir with neither the user's nor the system's
// configuration, so that a hook, a signing key or a display setting of
// this machine cannot change what is read.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-c", "core.quotePath=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "LC_ALL=C")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// Repository is the git repository a reader reads from: the one holding the
// paths, a submodule's own when they are in one.
type Repository struct {
	Root string // the repository's folder, with symbolic links resolved
}

// Read is what a reader read in a repository: the paths from its root, the
// tracked files under them, and the commit that names them.
type Read struct {
	Repository Repository
	Paths      []string // from the repository's root, slash-separated
	Files      []string // the tracked files under Paths, sorted
	Commit     string   // the newest of the commits that last changed each path
}

// absolute resolves a path given on the command line, symbolic links
// included, so that it compares with the repository's folder.
func absolute(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// Open finds the repository holding the paths and checks that a commit
// names what is under them: every path is in that one repository, tracked,
// with no change not committed and no file that is neither tracked nor
// ignored, and the clone is not shallow.
func Open(paths []string) (*Read, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no path to read")
	}
	r := &Read{}
	seen := map[string]bool{}
	for _, p := range paths {
		abs, err := absolute(p)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, err
		}
		dir := abs
		if !info.IsDir() {
			dir = filepath.Dir(abs)
		}
		top, err := git(dir, "rev-parse", "--show-toplevel")
		if err != nil {
			return nil, refuse("%s is not in a git repository, so no commit names what would be read (%v)", p, err)
		}
		top, err = filepath.EvalSymlinks(top)
		if err != nil {
			return nil, err
		}
		if r.Repository.Root == "" {
			r.Repository.Root = top
		} else if r.Repository.Root != top {
			return nil, refuse("%s is in another repository than %s; read each repository on its own", p, paths[0])
		}
		rel, err := filepath.Rel(top, abs)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		if !seen[rel] {
			seen[rel] = true
			r.Paths = append(r.Paths, rel)
		}
	}
	sort.Strings(r.Paths)
	root := r.Repository.Root
	shallow, err := git(root, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return nil, err
	}
	if shallow == "true" {
		return nil, refuse("the repository holding %s is a shallow clone, whose history cannot name the last change to a path; fetch its full history (git fetch --unshallow)", r.label())
	}
	status, err := git(root, append([]string{"status", "--porcelain", "--untracked-files=all", "--"}, r.Paths...)...)
	if err != nil {
		return nil, err
	}
	if status != "" {
		var names []string
		for line := range strings.SplitSeq(status, "\n") {
			if len(line) > 3 {
				names = append(names, line[3:])
			}
		}
		return nil, refuse("%s %s changes not committed or files not tracked (%s); commit them, or ignore them, so that a commit names what is read", strings.Join(r.Paths, ", "), hasOrHave(len(r.Paths)), strings.Join(names, ", "))
	}
	files, err := git(root, append([]string{"ls-files", "-z", "--"}, r.Paths...)...)
	if err != nil {
		return nil, err
	}
	for f := range strings.SplitSeq(files, "\x00") {
		if f != "" {
			r.Files = append(r.Files, f)
		}
	}
	sort.Strings(r.Files)
	for _, p := range r.Paths {
		if !tracked(p, r.Files) {
			return nil, refuse("%s holds no tracked file; only tracked files are read", p)
		}
	}
	commit, err := git(root, append([]string{"log", "-1", "--format=%H", "--"}, r.Paths...)...)
	if err != nil {
		return nil, err
	}
	if commit == "" {
		return nil, refuse("no commit has changed %s", strings.Join(r.Paths, ", "))
	}
	r.Commit = commit
	return r, nil
}

// LastChange is the commit that last changed a path from the repository's
// root, or "" when none has.
func (r *Read) LastChange(path string) (string, error) {
	return git(r.Repository.Root, "log", "-1", "--format=%H", "--", path)
}

// Exists says whether a commit is in the repository's history.
func (r *Read) Exists(commit string) bool {
	_, err := git(r.Repository.Root, "cat-file", "-e", commit+"^{commit}")
	return err == nil
}

func (r *Read) label() string { return strings.Join(r.Paths, ", ") }

func tracked(p string, files []string) bool {
	if p == "." {
		return len(files) > 0
	}
	for _, f := range files {
		if f == p || strings.HasPrefix(f, p+"/") {
			return true
		}
	}
	return false
}

func hasOrHave(n int) string {
	if n == 1 {
		return "has"
	}
	return "have"
}

// openDump checks a dump made from code, such as a catalogue or a route
// table: it names a full commit hash and the path it was made from, that
// path is in the dump's own repository, nothing under either is left
// uncommitted, and the commit is the last change to the path, so the dump
// is not stale. script names what makes the dump again. It returns the
// read, with the dump's commit as its commit, and the dump's path from the
// repository's root.
func openDump(dumpPath, madeFrom, commit, script string) (*Read, string, error) {
	if !fullHash.MatchString(commit) {
		return nil, "", refuse("%s names no full commit hash it was made from (commit: %q)", dumpPath, commit)
	}
	if madeFrom == "" {
		return nil, "", refuse("%s names no path it was made from", dumpPath)
	}
	dumpRead, err := Open([]string{dumpPath})
	if err != nil {
		return nil, "", err
	}
	name := dumpRead.Paths[0]
	made := filepath.Join(dumpRead.Repository.Root, filepath.FromSlash(madeFrom))
	if _, err := os.Stat(made); err != nil {
		return nil, "", refuse("%s was made from %s, which is not in the repository", name, madeFrom)
	}
	r, err := Open([]string{dumpPath, made})
	if err != nil {
		return nil, "", err
	}
	if !r.Exists(commit) {
		return nil, "", refuse("%s was made at commit %s, which is not in the repository's history", name, commit)
	}
	last, err := r.LastChange(madeFrom)
	if err != nil {
		return nil, "", err
	}
	if last != commit {
		return nil, "", refuse("%s is stale: it was made at commit %s, and %s was last changed at commit %s; run %s again and commit the dump", name, commit, madeFrom, last, script)
	}
	r.Commit = commit
	return r, name, nil
}
