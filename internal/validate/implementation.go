package validate

import (
	"fmt"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// designKeys are keys that only a specification has. In an implementation
// file each is reported as design_key instead of an unknown key.
var designKeys = map[string]bool{
	"specarch": true, "stages": true, "sources": true,
	"stakeholders": true, "needs": true, "requirements": true, "glossary": true, "assumptions": true,
	"enums": true, "entities": true, "permissions": true, "roles": true, "paths": true, "commands": true,
	"channels": true, "dependencies": true, "session": true, "pages": true, "algorithms": true, "tests": true,
	"environments": true, "release": true, "rollback": true, "migrations": true, "checks": true, "signoff": true,
	"relations": true, "constraints": true, "transitions": true, "operationId": true, "responses": true,
	"requestBody": true, "parameters": true, "permission": true, "formula": true, "examples": true,
	"properties": true, "primaryKey": true, "stateField": true, "messages": true, "payload": true,
	"calls": true, "idempotencyKey": true, "guard": true, "validity": true,
	"sensitivity": true, "atRest": true, "lookup": true, "audited": true, "deletion": true,
	"listOf": true, "limits": true, "problem": true, "errors": true, "jobs": true, "menus": true, "workflows": true,
	"views": true, "count": true, "schemas": true,
}

// namedMaps are the implementation file's maps whose keys the author
// chooses (a folder, a target, a library), so a key there is never a
// design keyword by mistake.
var namedMaps = map[string]bool{
	"layout": true, "mappings": true, "targets": true, "tasks": true, "libraries": true,
	"deployments": true, "decisions": true, "suites": true, "configuration": true, "bindings": true, "idioms": true,
}

func (c *checker) checkBoundaryImplementation() {
	walk(c.root, nil, func(n *yaml.Node, path []string) {
		for _, tok := range path {
			if tok == "settings" {
				return // free-form target and framework settings
			}
		}
		if len(path) > 0 && namedMaps[path[len(path)-1]] {
			return
		}
		for _, p := range source.Pairs(n) {
			if designKeys[p.Key.Value] {
				c.add(p.Key, source.Pointer(append(path, p.Key.Value)...), RuleDesignKey,
					"%s belongs to the specification, not to one implementation; move it to the specification's files", p.Key.Value)
			}
		}
	})
}

// checkImplementation checks the file against the specification it
// implements: s when the file is part of it, otherwise the one its
// implements.file names, read through load.
func (c *checker) checkImplementation(s *spec.Spec, load Loader) {
	impl := source.Child(c.root, "implements")
	fileNode := source.Child(impl, "file")
	rel := source.Str(fileNode)
	if rel == "" || KindOf(rel) != KindDesign {
		return // the schema reports it
	}
	if s == nil {
		dir := filepath.Dir(filepath.Join(filepath.Dir(c.file), filepath.FromSlash(rel)))
		if load == nil {
			return
		}
		s = load(dir)
		if s.Root == nil {
			c.add(fileNode, "/implements/file", RuleImplements, "the specification's root file %s cannot be read as YAML; run specarch validate on %s first", rel, dir)
			return
		}
	} else if filepath.Clean(filepath.Join(filepath.Dir(c.file), filepath.FromSlash(rel))) != filepath.Clean(s.RootFile) {
		c.add(fileNode, "/implements/file", RuleImplements, "this file is under %s but implements names %s; point it at the root file of the specification it is part of", s.Dir, rel)
		return
	}
	if s.Root == nil || source.Child(s.Root, "specarch") == nil {
		c.add(fileNode, "/implements/file", RuleImplements, "%s is not a specification's root file; run specarch validate on it first", rel)
		return
	}
	verNode := source.Child(impl, "version")
	want := source.Str(source.Child(source.Child(s.Root, "info"), "version"))
	if got := source.Str(verNode); got != "" && got != want {
		c.add(verNode, "/implements/version", RuleImplements,
			"this file implements version %s of %s, but that specification is now version %s; review the design change, then update this version", got, rel, want)
	}
	d := newDesign(s.Root)
	d.spec = s

	for _, p := range source.Pairs(source.Child(c.root, "layout")) {
		for i, ref := range source.Items(source.Child(p.Value, "implements")) {
			c.checkDesignRef(s.Root, ref, ref.Value, source.Pointer("layout", p.Key.Value, "implements", fmt.Sprint(i)), rel)
		}
	}
	for _, p := range source.Pairs(source.Child(c.root, "mappings")) {
		c.checkDesignRef(s.Root, p.Key, p.Key.Value, source.Pointer("mappings", p.Key.Value), rel)
		if by := source.Child(p.Value, "ownedBy"); by != nil && by.Value != "" && d.stakeholders[by.Value] == nil {
			c.add(by, source.Pointer("mappings", p.Key.Value, "ownedBy"), RuleStakeholder, "%s is not a stakeholder of the specification%s", by.Value, suggest(by.Value, d.stakeholders))
		}
	}
	decisions := map[string]*yaml.Node{}
	for k, v := range d.decisions {
		decisions[k] = v
	}
	for _, p := range source.Pairs(source.Child(c.root, "decisions")) {
		decisions[p.Key.Value] = p.Value
	}
	c.checkOrigin(c.root, decisions, func(path []string) bool { return len(path) == 2 && path[0] == "decisions" })
	for _, p := range source.Pairs(source.Child(c.root, "decisions")) {
		if d.decisions[p.Key.Value] != nil {
			c.add(p.Key, source.Pointer("decisions", p.Key.Value), RuleDecision,
				"%s is already a decision of the specification; give this implementation decision its own number", p.Key.Value)
		}
	}
	c.checkRequirementLinks(c.root, nil, d)
	c.checkCitations(c.root, nil, d)
	c.checkDeployments(d)
	c.checkSuites(d, rel)
	c.checkIdioms(s)
}

// checkDeployments checks each deployment names an environment of the
// specification when it declares any, and gives values only to settings
// the specification declares and that are not secret.
func (c *checker) checkDeployments(d *design) {
	for _, p := range source.Pairs(source.Child(c.root, "deployments")) {
		base := []string{"deployments", p.Key.Value}
		envNode := source.Child(p.Value, "environment")
		env := source.Str(envNode)
		switch {
		case env != "" && d.environments[env] == nil:
			c.add(envNode, source.Pointer(append(base, "environment")...), RuleEnvironment, "%s is not an environment of the specification%s", env, suggest(env, d.environments))
		case env == "" && len(d.environments) > 0:
			c.add(p.Key, source.Pointer(base...), RuleEnvironment, "the specification declares environments, so this deployment must say which one it installs; add environment with one of %s", strings.Join(sortedKeys(d.environments), ", "))
		}
		for _, m := range source.Pairs(source.Child(p.Value, "monitors")) {
			if d.monitors[m.Key.Value] == nil {
				c.add(m.Key, source.Pointer(append(base, "monitors", m.Key.Value)...), RuleMonitor, "%s is not a monitor of the specification%s", m.Key.Value, suggest(m.Key.Value, d.monitors))
			}
		}
		for _, cfg := range source.Pairs(source.Child(p.Value, "configuration")) {
			ptr := source.Pointer(append(base, "configuration", cfg.Key.Value)...)
			setting := d.settings[cfg.Key.Value]
			if setting == nil {
				c.add(cfg.Key, ptr, RuleSetting, "%s is not a setting of the specification's configuration%s", cfg.Key.Value, suggest(cfg.Key.Value, d.settings))
				continue
			}
			if source.Str(source.Child(setting, "secret")) == "true" {
				c.add(cfg.Value, ptr, RuleSecretValue, "%s is a secret, and a secret's value is never written in a specification; remove it and say in the setting's description where the value comes from", cfg.Key.Value)
			}
		}
	}
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
					"%s is not a test of the specification%s", n.Value, suggest(n.Value, tests))
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
						"the specification has no test about %s %s; name a subject that has tests", p.Key.Value, p.Value.Value)
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
		c.add(at, ptr, RuleDesignRef, "%s does not point at anything in the specification; correct the pointer (a / inside a name is written ~1)", ref)
	}
}
