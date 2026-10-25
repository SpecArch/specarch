package facts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/SpecArch/specarch/internal/disk"
	"github.com/SpecArch/specarch/readers"
)

// Languages are the languages specarch facts reads, in the order its
// usage names them.
var Languages = []string{"javascript", "swift", "dart"}

// A Refusal is a folder or a dump file the facts cannot be read from or
// written to as asked: specarch facts exits 1 with its message.
type Refusal struct{ Message string }

func (r *Refusal) Error() string { return r.Message }

func refuse(format string, args ...any) error { return &Refusal{fmt.Sprintf(format, args...)} }

// A Usage error is a command given wrongly: specarch facts exits 2.
type Usage struct{ Message string }

func (u *Usage) Error() string { return u.Message }

// source is the committed folder a dump is made of.
type source struct {
	root       string // the repository's root, absolute
	path       string // the folder from the root, or "."
	commit     string // the last commit that changed the folder
	repository string // the submodule's folder in the dump file's repository, or ""
	files      []string
}

var (
	jsListed   = regexp.MustCompile(`(\.(ts|tsx|mts|cts|js|jsx|mjs|cjs|vue)$|(^|/)(tsconfig|jsconfig)\.json$)`)
	jsDeclared = regexp.MustCompile(`\.d\.[cm]?ts$`)
)

// listed reports whether a tracked file is one a language's reader reads.
func listed(language, f string) bool {
	switch language {
	case "javascript":
		return jsListed.MatchString(f) && !jsDeclared.MatchString(f)
	case "swift":
		return strings.HasSuffix(f, ".swift")
	case "dart":
		return strings.HasSuffix(f, ".dart")
	}
	return false
}

// git runs git in dir without the user's or the system's configuration,
// as specarch extract does, so the same commit gives the same answer
// everywhere.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-c", "core.quotePath=false", "-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s in %s: %s", strings.Join(args, " "), dir, msg)
	}
	return out.String(), nil
}

// read finds the committed folder a dump is made of: its repository, its
// path, the commit that last changed it and its tracked files. A change
// not committed, or a file that is neither tracked nor ignored, is
// refused, since no commit would name what the facts were read from.
func read(language, folder, dump string) (*source, error) {
	info, err := os.Stat(folder)
	if err != nil || !info.IsDir() {
		return nil, &Usage{fmt.Sprintf("%s is not a folder", folder)}
	}
	top, err := git(folder, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, refuse("%s is not in a git repository, and a dump names the commit its facts were read at: %s", folder, err)
	}
	prefix, err := git(folder, "rev-parse", "--show-prefix")
	if err != nil {
		return nil, refuse("%s", err)
	}
	s := &source{root: strings.TrimSpace(top), path: strings.TrimSuffix(strings.TrimSpace(prefix), "/")}
	if s.path == "" {
		s.path = "."
	}
	status, err := git(folder, "status", "--porcelain", "--untracked-files=all", "--", ".")
	if err != nil {
		return nil, refuse("%s", err)
	}
	if strings.TrimSpace(status) != "" {
		return nil, refuse("%s has changes not committed or files not tracked; commit them first:\n%s", s.path, strings.TrimRight(status, "\n"))
	}
	commit, err := git(folder, "log", "-1", "--format=%H", "--", ".")
	if err != nil {
		return nil, refuse("%s", err)
	}
	if s.commit = strings.TrimSpace(commit); s.commit == "" {
		return nil, refuse("no commit has changed %s", s.path)
	}
	if s.repository, err = repositoryKey(s.root, dump); err != nil {
		return nil, err
	}
	out, err := git(s.root, "ls-files", "--", s.path)
	if err != nil {
		return nil, refuse("%s", err)
	}
	for _, f := range strings.Split(out, "\n") {
		if f != "" && listed(language, f) {
			s.files = append(s.files, f)
		}
	}
	return s, nil
}

// repositoryKey is the folder of the code's repository from the root of
// the repository the dump file is in, or "" when the dump file is in the
// code's own repository or in no repository. A dump file in a repository
// that does not hold the code's is refused, since no gitlink there could
// pin the code's commit (docs/extraction.md).
func repositoryKey(codeRoot, dump string) (string, error) {
	dir := filepath.Dir(dump)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", &Usage{fmt.Sprintf("the folder %s of the dump file does not exist", dir)}
	}
	top, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", nil
	}
	code, err := filepath.EvalSymlinks(codeRoot)
	if err != nil {
		return "", refuse("cannot read %s: %s", codeRoot, err)
	}
	dumpRoot, err := filepath.EvalSymlinks(strings.TrimSpace(top))
	if err != nil {
		return "", refuse("cannot read %s: %s", top, err)
	}
	if code == dumpRoot {
		return "", nil
	}
	if !strings.HasPrefix(code, dumpRoot+string(filepath.Separator)) {
		return "", refuse("the dump file %s is in the repository %s, which does not hold the code's repository %s as a folder; write the dump in the code's repository, or in one that holds it as a submodule", dump, dumpRoot, code)
	}
	key := filepath.ToSlash(strings.TrimPrefix(code, dumpRoot+string(filepath.Separator)))
	if strings.ContainsAny(key, `"\`) {
		return "", &Usage{fmt.Sprintf("the folder %s has a quote or a backslash, which a dump does not take in its repository", key)}
	}
	return key, nil
}

// withRepository writes the key into the dump as its repository, on the
// line after the opening brace.
func withRepository(data []byte, key string) ([]byte, error) {
	if key == "" {
		return data, nil
	}
	if !bytes.HasPrefix(data, []byte("{\n")) {
		return nil, refuse("the dump does not start with a line holding only {, so the repository cannot be written into it")
	}
	return append([]byte(fmt.Sprintf("{\n    \"repository\": %q,\n", key)), data[2:]...), nil
}

// Dump writes the code-facts dump of a committed folder (ADR-089,
// ADR-116), and says on out what it wrote. progress is told each step a
// reader's toolchain takes. On a dry run of w it reads the folder and
// lists the dump; for the Swift or the Dart reader it neither installs
// nor builds nor runs the reader, and lists the dump and, when the reader
// is not in the cache yet, the cache folder it would be written to.
func Dump(language, folder, dump string, w *disk.Writer, out, progress io.Writer) error {
	if !listedLanguage(language) {
		return &Usage{fmt.Sprintf("there is no reader of %s; the languages are javascript, swift and dart", language)}
	}
	abs, err := filepath.Abs(dump)
	if err != nil {
		return &Usage{fmt.Sprintf("cannot read the path %s: %s", dump, err)}
	}
	s, err := read(language, folder, abs)
	if err != nil {
		return err
	}
	if w.Dry() && language != "javascript" {
		dir, err := cacheDir(language)
		if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(dir, ".complete")); err != nil {
			fmt.Fprintf(w.List, "would write %s\n", dir)
		}
		fmt.Fprintf(w.List, "would write %s\n", abs)
		return nil
	}
	var data []byte
	switch language {
	case "javascript":
		data, err = JavaScript(s.root, s.path, s.commit, s.files)
	default:
		data, err = runHelper(language, s, progress)
	}
	if err != nil {
		return err
	}
	if data, err = withRepository(data, s.repository); err != nil {
		return err
	}
	if w.Dry() {
		return w.WriteFile(abs, data, 0o644)
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return &Usage{fmt.Sprintf("cannot write %s: %s", abs, err)}
	}
	via := ""
	if s.repository != "" {
		via = ", read through the submodule " + s.repository
	}
	fmt.Fprintf(out, "specarch facts: wrote %s from %s at commit %s%s\n", dump, s.path, s.commit, via)
	return nil
}

func listedLanguage(l string) bool {
	for _, x := range Languages {
		if x == l {
			return true
		}
	}
	return false
}

// cacheDir is the folder a helper reader is written to and built in: the
// user's cache, named by the language and a hash of the reader's files and
// lock file, so a binary with another reader never runs this one.
func cacheDir(language string) (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		var err error
		if base, err = os.UserCacheDir(); err != nil {
			return "", refuse("no cache folder for the %s reader: %s; set XDG_CACHE_HOME", language, err)
		}
	}
	h := sha256.New()
	err := fs.WalkDir(readers.Files, language, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := readers.Files.ReadFile(p)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", p, len(data))
		h.Write(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "specarch", "readers", language+"-"+hex.EncodeToString(h.Sum(nil))[:16]), nil
}

// install writes a helper reader into the cache once.
func install(language string, progress io.Writer) (string, error) {
	dir, err := cacheDir(language)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(dir, ".complete")); err == nil {
		return dir, nil
	}
	fmt.Fprintf(progress, "specarch facts: writing the %s reader into %s\n", language, dir)
	tmp := dir + ".tmp"
	os.RemoveAll(tmp)
	err = fs.WalkDir(readers.Files, language, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		to := filepath.Join(tmp, filepath.FromSlash(strings.TrimPrefix(p, language)))
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		data, err := readers.Files.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(to, data, 0o644)
	})
	if err != nil {
		return "", refuse("cannot write the %s reader into %s: %s", language, tmp, err)
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", refuse("cannot make %s: %s", filepath.Dir(dir), err)
	}
	if err := os.Rename(tmp, dir); err != nil && !errors.Is(err, fs.ErrExist) {
		if _, statErr := os.Stat(dir); statErr != nil {
			return "", refuse("cannot move the %s reader into %s: %s", language, dir, err)
		}
		os.RemoveAll(tmp)
	}
	return dir, nil
}

// toolchain runs a step of a helper's toolchain, its output on progress.
func toolchain(progress io.Writer, dir string, name string, args ...string) error {
	fmt.Fprintf(progress, "specarch facts: running %s %s\n", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = progress, progress
	if err := cmd.Run(); err != nil {
		return refuse("%s %s in %s failed: %s; the reader needs that toolchain installed", name, strings.Join(args, " "), dir, err)
	}
	return nil
}

// runHelper builds the Swift or the Dart reader in the cache with its
// pinned lock file, once, and runs it on the folder.
func runHelper(language string, s *source, progress io.Writer) ([]byte, error) {
	dir, err := install(language, progress)
	if err != nil {
		return nil, err
	}
	var name string
	var args []string
	switch language {
	case "swift":
		bin := filepath.Join(dir, ".build", "release", "code-facts-swift")
		if _, err := os.Stat(bin); err != nil {
			if err := toolchain(progress, dir, "swift", "build", "-c", "release", "--product", "code-facts-swift"); err != nil {
				return nil, err
			}
		}
		name, args = bin, []string{s.path, s.commit}
	case "dart":
		if _, err := os.Stat(filepath.Join(dir, ".dart_tool", "package_config.json")); err != nil {
			if err := toolchain(progress, dir, "dart", "pub", "get", "--enforce-lockfile"); err != nil {
				return nil, err
			}
		}
		name, args = "dart", []string{"run", path.Join(filepath.ToSlash(dir), "bin", "code_facts_dart.dart"), s.path, s.commit}
	}
	os.WriteFile(filepath.Join(dir, ".complete"), nil, 0o644)
	cmd := exec.Command(name, args...)
	cmd.Dir = s.root
	list := ""
	for _, f := range s.files {
		list += f + "\n"
	}
	cmd.Stdin = strings.NewReader(list)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, refuse("the %s reader failed on %s: %s: %s", language, s.path, err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}
