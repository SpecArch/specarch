// Package disk is where the commands write their files. A Writer either
// writes them or, for --dry-run, lists each file it would write or
// remove and writes nothing, so every command that writes says the same
// lines on a dry run and cannot write by another way.
package disk

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Writer writes files, or with DryRun lists them on List.
type Writer struct {
	DryRun bool
	// List takes one line per file on a dry run: would write <path> or
	// would remove <path>.
	List   io.Writer
	listed map[string]bool
}

// Real is the writer that writes.
var Real = &Writer{}

// DryRun is a writer that lists on list and writes nothing.
func DryRun(list io.Writer) *Writer { return &Writer{DryRun: true, List: list} }

// For is DryRun(list) when dry is true, Real otherwise.
func For(dry bool, list io.Writer) *Writer {
	if dry {
		return DryRun(list)
	}
	return Real
}

// WriteFile writes data at path, making its folder. On a dry run it lists
// the path once, unless the file holds those bytes already.
func (w *Writer) WriteFile(path string, data []byte, perm fs.FileMode) error {
	if w == nil || !w.DryRun {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, data, perm)
	}
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	w.list("would write", path)
	return nil
}

// Remove removes the file at path. On a dry run it lists the path once.
func (w *Writer) Remove(path string) error {
	if w == nil || !w.DryRun {
		return os.Remove(path)
	}
	w.list("would remove", path)
	return nil
}

// Dry says whether the writer writes nothing.
func (w *Writer) Dry() bool { return w != nil && w.DryRun }

func (w *Writer) list(verb, path string) {
	if w.listed == nil {
		w.listed = map[string]bool{}
	}
	key := verb + " " + path
	if w.listed[key] {
		return
	}
	w.listed[key] = true
	fmt.Fprintf(w.List, "%s %s\n", verb, path)
}
