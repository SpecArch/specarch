package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStep2ReferenceComparesEqual checks step 7 of docs/ui-nextjs-carbon.md
// on the library lending example: its sign-in and second-factor schemas,
// written by hand before the generator (testdata/step-2-reference, as
// committed then), and the rest of its Carbon application without the
// generated header, compare equal with what generate writes; a list
// schema with one column changed is reported as that column (ADR-093).
func TestStep2ReferenceComparesEqual(t *testing.T) {
	work := t.TempDir()
	copyTree(t, "../../examples/library-lending/spec", filepath.Join(work, "spec"))
	// Only the Carbon application's file, so the run has one output folder.
	for _, p := range []string{"implementation/go", "implementation/swift", "implementation/typescript/library-lending-acme.typescript.specarch-implementation.yaml"} {
		if err := os.RemoveAll(filepath.Join(work, "spec", p)); err != nil {
			t.Fatal(err)
		}
	}
	src := filepath.Join(work, "web-nextjs", "carbon", "src")
	copyTree(t, "../../examples/library-lending/web-nextjs/carbon/src", src)
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(p, []byte(unmarked(string(data))), 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"sign-in", "sign-in/second-factor"} {
		data, err := os.ReadFile(filepath.Join("testdata/step-2-reference", p, "page.schema.ts"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(src, "app", p, "page.schema.ts"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	built := buildPlugin(t, "ui-typescript")
	t.Chdir(work)
	t.Setenv("PATH", built)

	var stdout, stderr bytes.Buffer
	if status := run([]string{"generate", "ui", "--check", "--unapproved", "spec"}, &stdout, &stderr); status != 0 {
		t.Fatalf("exit status %d\n%s%s", status, stdout.String(), stderr.String())
	}
	for _, p := range []string{"sign-in", "sign-in/second-factor"} {
		want := "web-nextjs/carbon/src/app/" + p + "/page.schema.ts: built by hand, and matches what generate writes as data\n"
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("standard output lacks %q\n%s", want, stdout.String())
		}
	}

	members := filepath.Join(src, "app", "members", "page.schema.ts")
	data, err := os.ReadFile(members)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(data), `"members-list.columns.email", sortable: false`, `"members-list.columns.email", sortable: true`, 1)
	if changed == string(data) {
		t.Fatal("the members list's email column is not where the test expects it")
	}
	if err := os.WriteFile(members, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if status := run([]string{"generate", "ui", "--check", "--unapproved", "spec"}, &stdout, &stderr); status != 1 {
		t.Fatalf("exit status %d, want 1\n%s", status, stdout.String())
	}
	want := "web-nextjs/carbon/src/app/members/page.schema.ts: built by hand, and differs from what generate writes\n  columns[2].sortable: true on disk; generate writes false\n"
	if !strings.Contains(stdout.String(), want) {
		t.Errorf("standard output lacks %q\n%s", want, stdout.String())
	}
}

// copyTree copies every file under from into to.
func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		dst := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
