package source

import (
	"os"
	"regexp"
	"testing"
)

// The Swift build's tables of YAML syntax classes and sentences are this
// build's, entry for entry, so that both print the same words (ADR-177).
func TestSwiftSyntaxTablesMatch(t *testing.T) {
	data, err := os.ReadFile("../../swift/Sources/SpecArchKit/Syntax.swift")
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []struct {
		name string
		want map[string]string
	}{
		{"syntaxClass", syntaxClass},
		{"syntaxSentence", syntaxSentence},
	} {
		body := regexp.MustCompile(`(?s)let ` + table.name + `: \[String: String\] = \[\n(.*?)\n\]`).FindSubmatch(data)
		if body == nil {
			t.Fatalf("Syntax.swift has no table %s", table.name)
		}
		got := map[string]string{}
		for _, m := range regexp.MustCompile(`(?m)^\s*"((?:[^"\\]|\\.)*)": "((?:[^"\\]|\\.)*)",$`).FindAllSubmatch(body[1], -1) {
			got[unescape(string(m[1]))] = unescape(string(m[2]))
		}
		if len(got) != len(table.want) {
			t.Errorf("%s: Swift has %d entries, Go %d", table.name, len(got), len(table.want))
		}
		for k, v := range table.want {
			if got[k] != v {
				t.Errorf("%s[%q]: Swift has %q, Go %q", table.name, k, got[k], v)
			}
		}
	}
	for problem, class := range syntaxClass {
		if _, ok := syntaxSentence[class]; !ok {
			t.Errorf("class %s of %q has no sentence", class, problem)
		}
	}
}

func unescape(s string) string {
	return regexp.MustCompile(`\\(.)`).ReplaceAllString(s, "$1")
}

func TestFlowClass(t *testing.T) {
	for _, c := range []struct {
		text  string
		class string
	}{
		{"[!!str, a]", "tag_runs_on"},
		{"[&x !!str, a]", "tag_runs_on"},
		{"[!!null]", "tag_runs_on"},
		{"[!!str a, b]", ""},
		{"[!<tag:yaml.org,2002:str> a]", ""},
		{"[hello !world, a]", ""},
		{"{a: b, ? c ? d}", "question_in_flow"},
		{"[a?b]", "question_in_flow"},
		{"[?b]", "question_in_flow"},
		{"{? c d}", ""},
		{`["what?", 'a?b']`, ""},
		{"[a, b]  # what?", ""},
	} {
		if got, _ := flowClass([]string{c.text}, 1, 1); got != c.class {
			t.Errorf("flowClass(%q) = %q, want %q", c.text, got, c.class)
		}
	}
}
