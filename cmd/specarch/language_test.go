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
	"go.yaml.in/yaml/v3"
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
		catalog, err := generate.LoadCatalog(tag)
		if err != nil {
			t.Fatal(err)
		}
		for _, target := range generate.DocumentTargets {
			if !generate.BuiltDocuments[target] {
				continue
			}
			for _, paths := range languageRuns {
				words := specWords(t, paths)
				out := t.TempDir()
				companions := readCompanions(t, paths)
				var stdout, stderr bytes.Buffer
				status := run(append([]string{"document", target, "--language", tag, "--out", filepath.Join(out, tag)}, paths...), &stdout, &stderr)
				if status == 2 && target == "handover" && strings.Contains(stderr.String(), "has a handover") {
					continue // a specification with no handover has no handover document
				}
				if status == 2 {
					t.Fatalf("document %s --language %s %v: exit 2: %s%s", target, tag, paths, stdout.String(), stderr.String())
				}
				if after := readCompanions(t, paths); !equalMaps(companions, after) {
					t.Errorf("document %s --language %s %v wrote a region of specarch.md; a document in another language writes none", target, tag, paths)
				}
				if missed := generate.Missed(); len(missed) > 0 {
					t.Errorf("document %s --language %s %v wrote %d texts the catalog of %s does not give:\n%q", target, tag, paths, len(missed), tag, missed)
				}
				stdout.Reset()
				stderr.Reset()
				run(append([]string{"document", target, "--out", filepath.Join(out, "en")}, paths...), &stdout, &stderr)
				run(append([]string{"document", target, "--language", "en", "--out", filepath.Join(out, "en-named")}, paths...), &stdout, &stderr)
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
					if named, _ := os.ReadFile(filepath.Join(out, "en-named", filepath.Base(f))); !bytes.Equal(named, english) {
						t.Errorf("document %s --language en %v: %s differs from the document written without --language", target, paths, filepath.Base(f))
					}
					if bytes.Equal(text, english) && !strings.HasSuffix(f, ".sarif") {
						t.Errorf("document %s --language %s %v: %s is the English document", target, tag, paths, filepath.Base(f))
					}
					checkUntranslated(t, catalog, words, target+" "+tag+" "+strings.Join(paths, " ")+": "+filepath.Base(f), string(text))
					if strings.HasSuffix(f, ".md") {
						checkAnchors(t, target+" "+tag+" "+strings.Join(paths, " ")+": "+filepath.Base(f), string(text))
						checkAnchors(t, target+" en "+strings.Join(paths, " ")+": "+filepath.Base(f), string(english))
					}
				}
			}
		}
	}
}

var (
	mdCell   = regexp.MustCompile(`\|([^|]*)`)
	htmlCell = regexp.MustCompile(`<t[dh][^>]*>([^<]*)</t[dh]>`)
)

// checkUntranslated requires of a document in another language that no
// table cell or heading is an English text its catalog translates, unless
// the specification itself holds that text as a key or a value, which a
// document writes as it is: such a text reached the document without
// going through the catalog, as a value the code set before it was
// written, which Missed, recording only what passes through the catalog,
// does not see.
func checkUntranslated(t *testing.T, c *generate.Catalog, words map[string]bool, what, text string) {
	t.Helper()
	text = regexp.MustCompile("(?s)```.*?```").ReplaceAllString(text, "")
	var found []string
	look := func(s string) {
		s = strings.TrimSpace(s)
		if tr, ok := c.Translations[s]; ok && tr != s && !words[s] && !derivedWords[s] {
			found = append(found, strconv.Quote(s))
		}
	}
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "|"):
			for _, m := range mdCell.FindAllStringSubmatch(strings.TrimSuffix(line, "|"), -1) {
				look(m[1])
			}
		case mdHeading.MatchString(line):
			look(strings.TrimLeft(line, "# "))
		}
	}
	for _, m := range htmlCell.FindAllStringSubmatch(text, -1) {
		look(m[1])
	}
	if len(found) > 0 {
		t.Errorf("%s: %d table cells or headings are English texts the catalog of %s translates, so they reached the document without it: %s", what, len(found), c.Language, strings.Join(found, ", "))
	}
}

// derivedWords are the names of the derived cases that are also texts a
// catalog translates; a derived case is written as derive names it, in
// English (ADR-241).
var derivedWords = map[string]bool{"empty": true, "filtered empty": true}

// specWords is every key and scalar value of the YAML files under the
// paths given: the specification's own words, which a document writes in
// the language they are written in.
func specWords(t *testing.T, paths []string) map[string]bool {
	t.Helper()
	words := map[string]bool{}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.ScalarNode {
			words[strings.TrimSpace(n.Value)] = true
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	for _, p := range paths {
		err := filepath.WalkDir(p, func(path string, e os.DirEntry, err error) error {
			if err != nil || e.IsDir() || !strings.HasSuffix(path, ".yaml") {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var n yaml.Node
			if yaml.Unmarshal(data, &n) == nil {
				walk(&n)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return words
}

// readCompanions reads the specarch.md beside each specification given,
// by path.
func readCompanions(t *testing.T, paths []string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, p := range paths {
		matches, _ := filepath.Glob(filepath.Join(p, "*", "specarch.md"))
		direct, _ := filepath.Glob(filepath.Join(p, "specarch.md"))
		for _, m := range append(matches, direct...) {
			data, err := os.ReadFile(m)
			if err != nil {
				t.Fatal(err)
			}
			out[m] = string(data)
		}
	}
	return out
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
