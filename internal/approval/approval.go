// Package approval reads and writes the record that says the owner read the
// documents of a specification and approves it for code generation. The
// record lives beside the specification's folder, under
// records/approvals/<version>.yaml, and holds a digest of the
// specification's files, so that any later change voids it.
package approval

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Record is one approval.
type Record struct {
	Version    string   `yaml:"version"`
	ApprovedBy string   `yaml:"approvedBy"`
	Date       string   `yaml:"date"`
	Documents  []string `yaml:"documents"`
	Digest     string   `yaml:"digest"`
}

type file struct {
	SpecarchRecord string   `yaml:"specarchRecord"`
	Kind           string   `yaml:"kind"`
	Version        string   `yaml:"version"`
	ApprovedBy     string   `yaml:"approvedBy"`
	Date           string   `yaml:"date"`
	Documents      []string `yaml:"documents"`
	Digest         string   `yaml:"digest"`
}

// Folder is where a specification's approvals live: records/approvals/
// beside the specification's folder.
func Folder(specDir string) string {
	abs, err := filepath.Abs(specDir)
	if err != nil {
		abs = specDir
	}
	return filepath.Join(filepath.Dir(abs), "records", "approvals")
}

// Path is the record file of one version.
func Path(specDir, version string) string {
	return filepath.Join(Folder(specDir), version+".yaml")
}

// RelPath is the record file as it is shown to the reader: relative to the
// working directory when that is possible.
func RelPath(specDir, version string) string {
	p := Path(specDir, version)
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, p); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(p)
}

// Digest is the SHA-256 of every .yaml file under the specification's
// folder, taken in byte order of their paths relative to that folder, each
// as its path, a zero byte, its bytes and a zero byte.
func Digest(specDir string) (string, error) {
	var paths []string
	err := filepath.WalkDir(specDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != specDir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".yaml") {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	rel := map[string]string{}
	var keys []string
	for _, p := range paths {
		r, err := filepath.Rel(specDir, p)
		if err != nil {
			return "", err
		}
		r = filepath.ToSlash(r)
		rel[r] = p
		keys = append(keys, r)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		data, err := os.ReadFile(rel[k])
		if err != nil {
			return "", err
		}
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// Read returns the approval of a version, nil when there is none, or an
// error when the file is there but is not an approval record.
func Read(specDir, version string) (*Record, error) {
	data, err := os.ReadFile(Path(specDir, version))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s is not a record file (%v)", RelPath(specDir, version), err)
	}
	switch {
	case f.Kind != "approval":
		return nil, fmt.Errorf("%s is a record of kind %q, not an approval", RelPath(specDir, version), f.Kind)
	case f.Version != version:
		return nil, fmt.Errorf("%s says version %s, but its name says %s", RelPath(specDir, version), f.Version, version)
	case f.Digest == "" || f.ApprovedBy == "" || f.Date == "":
		return nil, fmt.Errorf("%s is missing digest, approvedBy or date", RelPath(specDir, version))
	}
	return &Record{Version: f.Version, ApprovedBy: f.ApprovedBy, Date: f.Date, Documents: f.Documents, Digest: f.Digest}, nil
}

// Write writes the approval of r.Version, replacing an earlier one.
func Write(specDir string, r Record) error {
	var b strings.Builder
	b.WriteString("specarchRecord: \"0.1\"\n")
	b.WriteString("kind: approval\n")
	fmt.Fprintf(&b, "version: %s\n", r.Version)
	fmt.Fprintf(&b, "approvedBy: %s\n", r.ApprovedBy)
	fmt.Fprintf(&b, "date: %q\n", r.Date)
	if len(r.Documents) > 0 {
		fmt.Fprintf(&b, "documents: [%s]\n", strings.Join(r.Documents, ", "))
	}
	fmt.Fprintf(&b, "digest: %s\n", r.Digest)
	p := Path(specDir, r.Version)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(b.String()), 0o644)
}

// State says, in one sentence, where a version's approval stands against
// the files as they are now: approved, not approved, or void.
func State(specDir, version string) (approved bool, text string) {
	rec, err := Read(specDir, version)
	switch {
	case err != nil:
		return false, err.Error()
	case rec == nil:
		return false, fmt.Sprintf("not approved: there is no %s", RelPath(specDir, version))
	}
	digest, err := Digest(specDir)
	if err != nil {
		return false, fmt.Sprintf("not approved: the files cannot be read (%v)", err)
	}
	if digest != rec.Digest {
		return false, fmt.Sprintf("not approved: the approval of %s by %s no longer matches the files", rec.Date, rec.ApprovedBy)
	}
	return true, fmt.Sprintf("approved on %s by %s", rec.Date, rec.ApprovedBy)
}
