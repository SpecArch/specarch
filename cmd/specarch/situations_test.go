package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/metamodel"
)

// markdownSituation is the id a guide's section carries, in an HTML
// comment on the line before its heading.
var markdownSituation = regexp.MustCompile(`(?m)^<!-- situation: ([a-z][a-z-]*) -->\n## `)

// topicSituation is the id an entry of the situations help topic carries,
// in brackets at the start of its heading line.
var topicSituation = regexp.MustCompile(`(?m)^\[([a-z][a-z-]*)\] `)

func situationIDs(re *regexp.Regexp, text string) []string {
	var ids []string
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		ids = append(ids, m[1])
	}
	return ids
}

// TestGuideBySituation checks that the guide by situation, its Indonesian
// translation and the situations help topic hold the same situations in
// the same order, and that every feature of the program is named in both
// guides: each command the specification defines, each document target,
// each extract source, each plug-in in the repository, each help topic
// and each format the program reads, so the guide cannot drift from the
// program.
func TestGuideBySituation(t *testing.T) {
	noPlugins(t)
	read := func(path string) string {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	guides := map[string]string{
		"docs/guide-by-situation.md":    read("../../docs/guide-by-situation.md"),
		"docs/id/guide-by-situation.md": read("../../docs/id/guide-by-situation.md"),
	}
	topic := helpTopics()["situations"]
	if topic == "" {
		t.Fatal("docs/help has no situations.txt")
	}
	english := situationIDs(markdownSituation, guides["docs/guide-by-situation.md"])
	if len(english) == 0 {
		t.Fatal("docs/guide-by-situation.md has no section with a <!-- situation: id --> line before its heading")
	}
	for name, ids := range map[string][]string{
		"docs/id/guide-by-situation.md": situationIDs(markdownSituation, guides["docs/id/guide-by-situation.md"]),
		"docs/help/situations.txt":      situationIDs(topicSituation, topic),
	} {
		if strings.Join(ids, " ") != strings.Join(english, " ") {
			t.Errorf("%s holds the situations\n  %s\nand docs/guide-by-situation.md holds\n  %s\nthey must be the same ids in the same order", name, strings.Join(ids, " "), strings.Join(english, " "))
		}
	}
	seen := map[string]bool{}
	for _, id := range english {
		if seen[id] {
			t.Errorf("docs/guide-by-situation.md holds the situation %s twice", id)
		}
		seen[id] = true
	}

	// Every feature, with the words a guide names it by.
	var features []string
	for _, c := range definedCommands(t) {
		features = append(features, "specarch "+c.Name)
	}
	for _, target := range generate.DocumentTargets {
		features = append(features, "document "+target)
	}
	for _, source := range extractSources {
		features = append(features, "extract "+source)
	}
	plugins, err := filepath.Glob("../specarch-gen-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) == 0 {
		t.Fatal("no plug-in found under cmd/specarch-gen-*")
	}
	for _, dir := range plugins {
		features = append(features, filepath.Base(dir))
	}
	for name := range helpTopics() {
		features = append(features, "help "+name)
	}
	for _, f := range []metamodel.Format{metamodel.Design, metamodel.Implementation, metamodel.Record, metamodel.Idiom, metamodel.Estate, metamodel.Vocabulary} {
		features = append(features, "`"+f.Key+"`")
	}
	for name, text := range guides {
		for _, feature := range features {
			if !strings.Contains(text, feature) {
				t.Errorf("%s names no situation for %s; add it to the situation it belongs to, in both guides", name, feature)
			}
		}
	}
}
