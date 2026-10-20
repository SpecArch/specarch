package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	"channels": true, "dependencies": true, "session": true, "caller": true, "clients": true, "pages": true, "algorithms": true, "tests": true,
	"environments": true, "release": true, "rollback": true, "migrations": true, "handover": true, "checks": true, "signoff": true,
	"backup": true, "restore": true, "relations": true, "constraints": true, "transitions": true, "operationId": true, "responses": true,
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
		for i, tok := range path {
			if tok == "settings" {
				return // free-form target and framework settings
			}
			if tok == "handler" && i == 2 && path[0] == "mappings" {
				return // a handler record holds the code as it was read (ADR-144)
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
		c.checkIndexes(d, p.Key.Value, source.Child(p.Value, "indexes"))
		c.checkHandler(d, s, p.Key.Value, p.Value)
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

// checkHandler checks a handler record against the code it was read from
// (ADR-144): the source it holds for each function must still be what the
// cited file holds at the function's lines, the file found through the
// folder the code source names under url, from the specification's root.
// The working tree is read, not the commit, since neither build runs git;
// the commit the record names tells the reader what it was taken from.
func (c *checker) checkHandler(d *design, s *spec.Spec, key string, m *yaml.Node) {
	rec := source.Child(m, "handler")
	if rec == nil {
		return
	}
	ptr := source.Pointer("mappings", key, "handler")
	srcKey := ""
	for _, cite := range source.Items(source.Child(m, "cites")) {
		if k := source.Str(source.Child(cite, "source")); k != "" {
			srcKey = k
			break
		}
	}
	if srcKey == "" {
		c.warn(rec, ptr, RuleHandlerSource, "the handler record of %s cites no source, so the folder its functions are read from is not known; cite the code source under cites", key)
		return
	}
	src := d.sources[srcKey]
	url := source.Str(source.Child(src, "url"))
	if src == nil || url == "" || strings.Contains(url, "://") {
		c.warn(rec, ptr, RuleHandlerSource, "the source %s that the handler record of %s cites names no folder under url, so the record cannot be checked against the code", srcKey, key)
		return
	}
	root := filepath.Join(s.Dir, filepath.FromSlash(url))
	commit := shortCommit(source.Str(source.Child(rec, "commit")))
	for i, fn := range source.Items(source.Child(rec, "functions")) {
		fptr := source.Pointer("mappings", key, "handler", "functions", fmt.Sprint(i), "source")
		file := source.Str(source.Child(fn, "file"))
		name := source.Str(source.Child(fn, "name"))
		first, _ := strconv.Atoi(source.Str(source.Child(fn, "firstLine")))
		last, _ := strconv.Atoi(source.Str(source.Child(fn, "lastLine")))
		srcNode := source.Child(fn, "source")
		if file == "" || srcNode == nil || first < 1 || last < first {
			continue // the schema reports it
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			c.warn(source.Child(fn, "file"), source.Pointer("mappings", key, "handler", "functions", fmt.Sprint(i), "file"), RuleHandlerSource,
				"%s, from which the handler record of %s read %s at commit %s, is not under %s, so the record cannot be checked against the code; run specarch extract go with --implementation-out again and replace the record", file, key, name, commit, url)
			continue
		}
		lines := trimmedLines(string(data))
		want := trimmedLines(source.Str(srcNode))
		if last <= len(lines) && equalLines(dedented(lines[first-1:last]), want) {
			continue
		}
		if at := findLines(lines, want); at > 0 {
			c.warn(srcNode, fptr, RuleHandlerSource,
				"the handler record of %s read %s at commit %s from %s lines %d-%d, and that source is now at lines %d-%d; run specarch extract go with --implementation-out again and replace the record", key, name, commit, file, first, last, at, at+len(want)-1)
			continue
		}
		differs := first
		var held []string
		if first-1 < len(lines) {
			held = dedented(lines[first-1:])
		}
		for j := 0; j < len(want); j++ {
			if j >= len(held) || held[j] != want[j] {
				differs = first + j
				break
			}
		}
		c.warn(srcNode, fptr, RuleHandlerSource,
			"the handler record of %s read %s at commit %s, and %s lines %d-%d no longer hold that source (line %d differs), so the flowchart no longer matches the code; run specarch extract go with --implementation-out again and replace the record", key, name, commit, file, first, last, differs)
	}
}

// shortCommit is the first seven characters of a commit.
func shortCommit(c string) string {
	if len(c) > 7 {
		return c[:7]
	}
	return c
}

// trimmedLines splits text into lines, each without its trailing spaces
// and tabs, as a handler record holds them.
func trimmedLines(text string) []string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return lines
}

// dedented is lines without the indentation of the first one, as a handler
// record holds a function's source: a function literal written inside a
// function begins indented, and a YAML block cannot begin with a tab.
func dedented(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	indent := lines[0][:len(lines[0])-len(strings.TrimLeft(lines[0], " \t"))]
	if indent == "" {
		return lines
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.TrimPrefix(l, indent)
	}
	return out
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// findLines is the line, from 1, at which want starts in lines, or 0.
func findLines(lines, want []string) int {
	if len(want) == 0 {
		return 0
	}
	for i := 0; i+len(want) <= len(lines); i++ {
		if equalLines(dedented(lines[i:i+len(want)]), want) {
			return i + 1
		}
	}
	return 0
}

// checkIndexes checks a mapping's indexes are on an entity and hold its
// fields (ADR-121).
func (c *checker) checkIndexes(d *design, key string, indexes *yaml.Node) {
	if indexes == nil {
		return
	}
	name, ok := strings.CutPrefix(key, "#/entities/")
	entity := d.entities[name]
	if !ok || entity == nil {
		c.add(indexes, source.Pointer("mappings", key, "indexes"), RuleField,
			"indexes are a table's, and %s is not an entity of the specification; put them on the mapping of the entity whose table holds them", key)
		return
	}
	fields := fieldsOf(entity)
	for i, ix := range source.Items(indexes) {
		c.checkFieldList(source.Child(ix, "fields"), []string{"mappings", key, "indexes", fmt.Sprint(i), "fields"}, fields, name, "part of an index")
	}
}

// checkDeployments checks each deployment names an environment of the
// specification when it declares any, gives values only to settings the
// specification declares and that are not secret, names tasks of this file
// in its build, and, when the customer runs the environment, says how the
// build is made, how it is installed and what the hosts must provide
// (ADR-129).
func (c *checker) checkDeployments(d *design) {
	tasks := topMap(c.root, "tasks")
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
		build := source.Child(p.Value, "build")
		for _, k := range []string{"task", "sbom"} {
			if t := source.Child(build, k); t != nil && t.Value != "" && tasks[t.Value] == nil {
				c.add(t, source.Pointer(append(base, "build", k)...), RuleBuildTask, "%s is not a task of this implementation file%s", t.Value, suggest(t.Value, tasks))
			}
		}
		if env != "" && source.Str(source.Child(d.environments[env], "runBy")) == "customer" {
			for _, k := range [][2]string{
				{"build", "how the artifact is made from the source"},
				{"method", "how the build is put on the hosts"},
				{"infrastructure", "what the hosts must provide"},
			} {
				if source.Child(p.Value, k[0]) == nil {
					c.add(p.Key, source.Pointer(base...), RuleHandover, "%s is run by the customer, who builds and installs it from this file, and this deployment has no %s saying %s; add %s", env, k[0], k[1], k[0])
				}
			}
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
