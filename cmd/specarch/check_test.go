package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckAllowlistOnlyShrinks checks --since on the golden case of
// check: a line the allowlist did not hold at the revision fails as
// allowlist_added, even though it names a difference there is, and the
// same allowlist passes once that revision holds it (ADR-124).
func TestCheckAllowlistOnlyShrinks(t *testing.T) {
	work := t.TempDir()
	copyTree(t, "../../spec/tests/check-succeeds", work)
	allowlist := filepath.Join(work, "service", "check-allowlist.txt")
	full, err := os.ReadFile(allowlist)
	if err != nil {
		t.Fatal(err)
	}
	const added = "column_type_differs copies.label"
	if !strings.Contains(string(full), added+"\n") {
		t.Fatalf("the golden case's allowlist no longer names %s", added)
	}
	// The revision's allowlist lacks the line, which comes after it.
	if err := os.WriteFile(allowlist, []byte(strings.Replace(string(full), added+"\n", "", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	makeRepository(t, work, &caseRepository{Folder: "service", Commits: [][]string{
		{"migrations", "router", "access"},
		{"catalogue.json", "routes.json", "permissions.json", "check-allowlist.txt"},
	}})
	if err := os.WriteFile(allowlist, full, 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"check", "--allowlist", allowlist, "--since", "HEAD", filepath.Join(work, "project")}
	for _, d := range []string{"routes.json", "permissions.json", "catalogue.json"} {
		args = append(args, filepath.Join(work, "service", d))
	}
	var stdout, stderr bytes.Buffer
	if status := run(args, &stdout, &stderr); status != 1 {
		t.Fatalf("status %d with a line added since HEAD, want 1\n%s%s", status, stdout.String(), stderr.String())
	}
	want := "check-allowlist.txt:4:1: error: /: allowlist_added: the allowlist at HEAD does not name " + added
	if !strings.Contains(stdout.String(), want) {
		t.Errorf("standard output lacks %q\n%s", want, stdout.String())
	}
	if strings.Count(stdout.String(), "allowlist_") != 2 { // the line and its id
		t.Errorf("only the added line should fail\n%s", stdout.String())
	}
	caseGit(t, filepath.Join(work, "service"), "commit", "-q", "--no-gpg-sign", "-a", "-m", "Allow the label")
	stdout.Reset()
	stderr.Reset()
	if status := run(args, &stdout, &stderr); status != 0 {
		t.Fatalf("status %d once HEAD holds the line, want 0\n%s%s", status, stdout.String(), stderr.String())
	}
}
