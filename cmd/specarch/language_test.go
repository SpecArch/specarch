package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/generate"
)

// languageRuns are the specifications every document target is written
// from in each language: SpecArch's own, which holds records, and the
// examples that configure documents, so every target has something to
// write.
var languageRuns = [][]string{
	{"../../spec", "../../examples/library-lending"},
	{"../../examples/lending-desk"},
	{"../../examples/release-watcher"},
}

var (
	mdHeading = regexp.MustCompile(`(?m)^#{1,6} (.+)$`)
	mdLink    = regexp.MustCompile(`\]\(#([^)\s]+)\)`)
)

// TestDocumentsInEveryLanguage writes every document target in every
// language a catalog gives, from the specifications above, and requires
// of each: that it is written, that no text it writes is missing from the
// language's catalog, that it differs from the English document, and
// that every link to a heading of the same Markdown document reaches one.
func TestDocumentsInEveryLanguage(t *testing.T) {
	for _, tag := range generate.Languages()[1:] {
		for _, target := range generate.DocumentTargets {
			if !generate.BuiltDocuments[target] {
				continue
			}
			for _, paths := range languageRuns {
				out := t.TempDir()
				var stdout, stderr bytes.Buffer
				status := run(append([]string{"document", target, "--language", tag, "--out", filepath.Join(out, tag)}, paths...), &stdout, &stderr)
				if status == 2 && target == "handover" && strings.Contains(stderr.String(), "has a handover") {
					continue // a specification with no handover has no handover document
				}
				if status == 2 {
					t.Fatalf("document %s --language %s %v: exit 2: %s%s", target, tag, paths, stdout.String(), stderr.String())
				}
				if missed := generate.Missed(); len(missed) > 0 {
					t.Errorf("document %s --language %s %v wrote %d texts the catalog of %s does not give:\n%q", target, tag, paths, len(missed), tag, missed)
				}
				stdout.Reset()
				stderr.Reset()
				run(append([]string{"document", target, "--out", filepath.Join(out, "en")}, paths...), &stdout, &stderr)
				files, _ := filepath.Glob(filepath.Join(out, tag, "*"))
				if len(files) == 0 {
					continue // a target with nothing to write from these specifications, such as handover without one
				}
				for _, f := range files {
					text, err := os.ReadFile(f)
					if err != nil {
						t.Fatal(err)
					}
					english, _ := os.ReadFile(filepath.Join(out, "en", filepath.Base(f)))
					if bytes.Equal(text, english) && !strings.HasSuffix(f, ".sarif") {
						t.Errorf("document %s --language %s %v: %s is the English document", target, tag, paths, filepath.Base(f))
					}
					if strings.HasSuffix(f, ".md") {
						checkAnchors(t, target+" "+tag+" "+strings.Join(paths, " ")+": "+filepath.Base(f), string(text))
						checkAnchors(t, target+" en "+strings.Join(paths, " ")+": "+filepath.Base(f), string(english))
					}
				}
			}
		}
	}
}

// checkAnchors requires every link to #anchor in a Markdown document to
// name the anchor of one of its headings, as GitHub makes them.
func checkAnchors(t *testing.T, what, text string) {
	t.Helper()
	text = regexp.MustCompile("(?s)```.*?```").ReplaceAllString(text, "")
	anchors := map[string]bool{}
	seen := map[string]int{}
	for _, m := range mdHeading.FindAllStringSubmatch(text, -1) {
		a := githubSlug(m[1])
		if n := seen[a]; n > 0 {
			anchors[a+"-"+strconv.Itoa(n)] = true
		} else {
			anchors[a] = true
		}
		seen[a]++
	}
	for _, m := range mdLink.FindAllStringSubmatch(text, -1) {
		if !anchors[m[1]] {
			t.Errorf("%s: the link #%s reaches no heading", what, m[1])
		}
	}
}

// githubSlug is the anchor GitHub gives a heading: lower case, spaces as
// hyphens, every character but letters, digits, hyphens and underscores
// dropped.
func githubSlug(s string) string {
	s = regexp.MustCompile("`|\\*\\*").ReplaceAllString(s, "")
	s = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`).ReplaceAllString(s, "$1")
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127:
			b.WriteRune(r)
		}
	}
	return b.String()
}
