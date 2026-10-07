package validate

import (
	"fmt"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// designKeys are keys that only a design file has. In an implementation file
// each is reported as design_key instead of an unknown key.
var designKeys = map[string]bool{
	"specarch": true, "requirementSources": true, "enums": true, "entities": true, "permissions": true,
	"roles": true, "paths": true, "commands": true, "channels": true, "pages": true, "algorithms": true,
	"relations": true, "constraints": true, "transitions": true, "operationId": true, "responses": true,
	"requestBody": true, "parameters": true, "permission": true, "formula": true, "examples": true,
	"properties": true, "primaryKey": true, "stateField": true, "messages": true, "payload": true,
}

func (c *checker) checkBoundaryImplementation() {
	walk(c.root, nil, func(n *yaml.Node, path []string) {
		if len(path) > 0 && path[len(path)-1] == "settings" {
			return
		}
		for _, tok := range path {
			if tok == "settings" {
				return // free-form generator and framework settings
			}
		}
		for _, p := range source.Pairs(n) {
			if designKeys[p.Key.Value] {
				c.add(p.Key, source.Pointer(append(path, p.Key.Value)...), RuleDesignKey,
					"%s belongs to the design, not to one implementation; move it to the design file (%s)", p.Key.Value, "*"+DesignSuffix)
			}
		}
	})
}

func (c *checker) checkImplementation(read ReadFile) {
	impl := source.Child(c.root, "implements")
	fileNode := source.Child(impl, "file")
	rel := source.Str(fileNode)
	if rel == "" || !strings.HasSuffix(rel, DesignSuffix) {
		return // the schema reports it
	}
	designPath := filepath.Join(filepath.Dir(c.file), filepath.FromSlash(rel))
	data, err := read(designPath)
	if err != nil {
		c.add(fileNode, "/implements/file", RuleImplements, "the design file %s cannot be read (%s); correct the path, which is relative to this file", rel, plainIOError(err))
		return
	}
	doc := source.Parse(data)
	if doc.Root == nil || source.Child(doc.Root, "specarch") == nil {
		c.add(fileNode, "/implements/file", RuleImplements, "%s is not a valid design file; run specarch validate on it first", rel)
		return
	}
	verNode := source.Child(impl, "version")
	want := source.Str(source.Child(source.Child(doc.Root, "info"), "version"))
	if got := source.Str(verNode); got != "" && got != want {
		c.add(verNode, "/implements/version", RuleImplements,
			"this file implements version %s of %s, but that file is now version %s; review the design change, then update this version", got, rel, want)
	}
	d := newDesign(doc.Root)

	for _, p := range source.Pairs(source.Child(c.root, "layout")) {
		for i, ref := range source.Items(source.Child(p.Value, "implements")) {
			c.checkDesignRef(doc.Root, ref, ref.Value, source.Pointer("layout", p.Key.Value, "implements", fmt.Sprint(i)), rel)
		}
	}
	for _, p := range source.Pairs(source.Child(c.root, "mappings")) {
		c.checkDesignRef(doc.Root, p.Key, p.Key.Value, source.Pointer("mappings", p.Key.Value), rel)
	}
	for _, p := range source.Pairs(source.Child(c.root, "decisions")) {
		if d.decisions[p.Key.Value] != nil {
			c.add(p.Key, source.Pointer("decisions", p.Key.Value), RuleDecision,
				"%s is already a decision of the design file %s; give this implementation decision its own number", p.Key.Value, rel)
		}
	}
	c.checkRequirementLinks(source.Child(c.root, "decisions"), []string{"decisions"}, d.sources)
	c.checkSuites(d, rel)
}

// checkSuites checks every suite runs design tests that exist, or says it
// is implementation-only.
func (c *checker) checkSuites(d *design, rel string) {
	tests := map[string]*yaml.Node{}
	for _, p := range source.Pairs(source.Child(d.root, "tests")) {
		tests[p.Key.Value] = p.Value
	}
	for _, s := range source.Pairs(source.Child(source.Child(c.root, "testing"), "suites")) {
		base := []string{"testing", "suites", s.Key.Value}
		for i, n := range source.Items(source.Child(s.Value, "designTests")) {
			if tests[n.Value] == nil {
				c.add(n, source.Pointer(append(base, "designTests", fmt.Sprint(i))...), RuleSuite,
					"%s is not a test of the design file %s%s", n.Value, rel, suggest(n.Value, tests))
			}
		}
		for i, subj := range source.Items(source.Child(s.Value, "designTestsOf")) {
			for _, p := range source.Pairs(subj) {
				key := p.Key.Value + ": " + p.Value.Value
				found := false
				for _, t := range tests {
					if testSubjectKey(t) == key {
						found = true
						break
					}
				}
				if !found {
					c.add(p.Value, source.Pointer(append(base, "designTestsOf", fmt.Sprint(i), p.Key.Value)...), RuleSuite,
						"the design file %s has no test about %s %s; name a subject that has tests", rel, p.Key.Value, p.Value.Value)
				}
			}
		}
	}
}

func (c *checker) checkDesignRef(designRoot, at *yaml.Node, ref, ptr, rel string) {
	if !strings.HasPrefix(ref, "#/") {
		return // the schema reports it
	}
	var tokens []string
	for _, t := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		tokens = append(tokens, source.UnescapeToken(t))
	}
	if _, ok := source.Resolve(designRoot, tokens); !ok {
		c.add(at, ptr, RuleDesignRef, "%s does not point at anything in %s; correct the pointer (a / inside a name is written ~1)", ref, rel)
	}
}

func plainIOError(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 {
		return msg[i+2:]
	}
	return msg
}
