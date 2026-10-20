package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	"channels": true, "dependencies": true, "session": true, "caller": true, "clients": true, "tokens": true, "securityEvents": true, "pages": true, "algorithms": true, "tests": true,
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
	c.checkDependsOn()
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
		c.checkServers(d, base, source.Child(p.Value, "servers"))
		c.checkLinks(d, base, p.Value)
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

// checkDependsOn checks that every package a layout entry depends on is
// another key of the layout (ADR-133). A cycle is no error: Go forbids
// one and other stacks allow it, and the file describes the code as it is.
func (c *checker) checkDependsOn() {
	layout := source.Child(c.root, "layout")
	for _, p := range source.Pairs(layout) {
		for i, e := range source.Items(source.Child(p.Value, "dependsOn")) {
			ptr := source.Pointer("layout", p.Key.Value, "dependsOn", fmt.Sprint(i))
			switch {
			case e.Value == p.Key.Value:
				c.add(e, ptr, RuleDependsOn, "%s names itself; a package does not depend on itself, so remove the entry", e.Value)
			case source.Child(layout, e.Value) == nil:
				c.add(e, ptr, RuleDependsOn, "%s is not a key of layout; add it to layout, or remove the entry when it is a library, which goes under libraries%s", e.Value, suggestPairs(e.Value, layout))
			}
		}
	}
}

// checkServers checks a deployment's servers (ADR-136): a name used once,
// what each runs is in the file, packages only on a service, and a server
// that is a dependency names a declared one and says nothing of its own
// kind or what it runs.
func (c *checker) checkServers(d *design, base []string, servers *yaml.Node) {
	layout := topMap(c.root, "layout")
	targets := topMap(c.root, "targets")
	named := map[string]bool{}
	for i, srv := range source.Items(servers) {
		at := append(append([]string{}, base...), "servers", fmt.Sprint(i))
		if n := source.Child(srv, "name"); n != nil && n.Value != "" {
			if named[n.Value] {
				c.add(n, source.Pointer(append(at, "name")...), RuleServer, "%s names another server of this deployment already; give each server its own name", n.Value)
			}
			named[n.Value] = true
		}
		kindNode, runs := source.Child(srv, "kind"), source.Child(srv, "runs")
		kind := source.Str(kindNode)
		if kind == "" {
			kind = "service"
		}
		if dep := source.Child(srv, "dependency"); dep != nil && dep.Value != "" {
			if d.dependencies[dep.Value] == nil {
				c.add(dep, source.Pointer(append(at, "dependency")...), RuleServer, "%s is not a dependency of the specification%s", dep.Value, suggest(dep.Value, d.dependencies))
			}
			for _, key := range []string{"kind", "runs"} {
				if k := source.Child(srv, key); k != nil {
					c.add(k, source.Pointer(append(at, key)...), RuleServer, "this server is the dependency %s in this environment, which takes its kind from the dependency and runs nothing of this implementation; remove %s", dep.Value, key)
				}
			}
			continue
		}
		for j, pkg := range source.Items(source.Child(runs, "packages")) {
			ptr := source.Pointer(append(at, "runs", "packages", fmt.Sprint(j))...)
			switch {
			case layout[pkg.Value] == nil:
				c.add(pkg, ptr, RuleServer, "%s is not a key of layout%s", pkg.Value, suggest(pkg.Value, layout))
			case kind != "service":
				c.add(pkg, ptr, RuleServer, "a server of kind %s runs no package of this implementation; give it kind service, or move the package to the server that runs it", kind)
			}
		}
		for j, t := range source.Items(source.Child(runs, "targets")) {
			if targets[t.Value] == nil {
				c.add(t, source.Pointer(append(at, "runs", "targets", fmt.Sprint(j))...), RuleServer, "%s is not a key of targets%s", t.Value, suggest(t.Value, targets))
			}
		}
	}
}

// checkLinks checks a deployment's links against its servers (ADR-137):
// each joins two named servers on a port the target listens on. When the
// deployment declares links, it then warns at each server whose packages
// call a dependency, use a channel one carries or reach the database with
// no link that allows it: the firewall check.
func (c *checker) checkLinks(d *design, base []string, dep *yaml.Node) {
	servers := source.Items(source.Child(dep, "servers"))
	named := map[string]*yaml.Node{}
	var names []string
	for _, s := range servers {
		if n := source.Str(source.Child(s, "name")); n != "" && named[n] == nil {
			named[n] = s
			names = append(names, n)
		}
	}
	linksNode := source.Child(dep, "links")
	linked := map[[2]string]bool{}
	for i, l := range source.Items(linksNode) {
		at := append(append([]string{}, base...), "links", fmt.Sprint(i))
		from, to := source.Child(l, "from"), source.Child(l, "to")
		ok := true
		for _, end := range []struct {
			key  string
			node *yaml.Node
		}{{"from", from}, {"to", to}} {
			if end.node != nil && named[end.node.Value] == nil {
				c.add(end.node, source.Pointer(append(at, end.key)...), RuleLink, "%s is not a named server of this deployment%s", end.node.Value, suggest(end.node.Value, named))
				ok = false
			}
		}
		if !ok || from == nil || to == nil {
			continue
		}
		if from.Value == to.Value {
			c.add(to, source.Pointer(append(at, "to")...), RuleLink, "the link goes from %s to itself; a server reaches itself with no link, so remove it", from.Value)
			continue
		}
		linked[[2]string{from.Value, to.Value}] = true
		if port := source.Child(l, "port"); port != nil {
			var ports []string
			listened := false
			for _, ln := range source.Items(source.Child(named[to.Value], "listens")) {
				p := source.Str(source.Child(ln, "port"))
				ports = append(ports, p)
				listened = listened || p == port.Value
			}
			switch {
			case listened:
			case len(ports) == 0:
				c.add(port, source.Pointer(append(at, "port")...), RuleLink, "%s listens on no port, so no link can reach it; add the port under its listens", to.Value)
			default:
				c.add(port, source.Pointer(append(at, "port")...), RuleLink, "%s does not listen on %s; it listens on %s", to.Value, port.Value, strings.Join(ports, ", "))
			}
		}
	}
	if linksNode == nil {
		return
	}
	// What each dependency is in this deployment, and its database.
	depServer := map[string]string{}
	database := ""
	for _, s := range servers {
		n := source.Str(source.Child(s, "name"))
		if n == "" {
			continue
		}
		if dn := source.Str(source.Child(s, "dependency")); dn != "" {
			if _, seen := depServer[dn]; !seen {
				depServer[dn] = n
			}
			continue
		}
		if source.Str(source.Child(s, "kind")) == "database" && database == "" {
			for _, t := range source.Items(source.Child(source.Child(s, "runs"), "targets")) {
				if t.Value == "sql" {
					database = n
				}
			}
		}
	}
	layout := topMap(c.root, "layout")
	for i, s := range servers {
		n := source.Str(source.Child(s, "name"))
		if n == "" || source.Child(s, "dependency") != nil {
			continue
		}
		got := c.serverReach(d, layout, source.Items(source.Child(source.Child(s, "runs"), "packages")))
		at := source.Pointer(append(append([]string{}, base...), "servers", fmt.Sprint(i))...)
		var deps []string
		for dn := range got.dependencies {
			deps = append(deps, dn)
		}
		sort.Strings(deps)
		for _, dn := range deps {
			target, ok := depServer[dn]
			if !ok || linked[[2]string{n, target}] {
				continue
			}
			c.warn(s, at, RuleLinkMissing, "%s runs the packages implementing %s, which reach the dependency %s, and no link leads from %s to %s, the server that is %s here; add a link from %s to %s on a port it listens on", n, strings.Join(got.dependencies[dn], ", "), dn, n, target, dn, n, target)
		}
		if len(got.data) > 0 && database != "" && database != n && !linked[[2]string{n, database}] {
			c.warn(s, at, RuleLinkMissing, "%s runs the packages implementing %s, which read or write the database, and no link leads from %s to %s, the server that runs the sql target; add a link from %s to %s on a port it listens on", n, strings.Join(got.data, ", "), n, database, n, database)
		}
	}
}

// reach is what the packages of one server reach: each dependency with
// the design objects that reach it, and the objects that touch the data.
type reach struct {
	dependencies map[string][]string
	data         []string
}

// serverReach follows the design objects the packages implement: an
// operation or a job reaches the dependencies it calls and those carrying
// the channels it emits on or consumes from; an entity, or a job that
// reads or writes one, reaches the database.
func (c *checker) serverReach(d *design, layout map[string]*yaml.Node, packages []*yaml.Node) reach {
	r := reach{dependencies: map[string][]string{}}
	add := func(dep, by string) {
		if d.dependencies[dep] != nil && !contains(r.dependencies[dep], by) {
			r.dependencies[dep] = append(r.dependencies[dep], by)
		}
	}
	carriers := func(message, by string) {
		ch, _, _ := strings.Cut(message, "/")
		for _, e := range source.Items(source.Child(d.channels[ch], "carriedBy")) {
			add(e.Value, by)
		}
	}
	uses := func(n *yaml.Node, by string) {
		for _, call := range source.Items(source.Child(n, "calls")) {
			dep, _, _ := strings.Cut(call.Value, "#")
			add(dep, by)
		}
		for _, e := range source.Items(source.Child(n, "emits")) {
			carriers(e.Value, by)
		}
	}
	for _, pkg := range packages {
		for _, ref := range source.Items(source.Child(layout[pkg.Value], "implements")) {
			if !strings.HasPrefix(ref.Value, "#/") {
				continue // the schema reports it
			}
			var tokens []string
			for _, t := range strings.Split(strings.TrimPrefix(ref.Value, "#/"), "/") {
				tokens = append(tokens, source.UnescapeToken(t))
			}
			node, ok := source.Resolve(d.root, tokens)
			if !ok || len(tokens) < 2 {
				continue
			}
			switch tokens[0] {
			case "entities":
				if !contains(r.data, tokens[1]) {
					r.data = append(r.data, tokens[1])
				}
			case "jobs":
				uses(node, tokens[1])
				if consumes := source.Str(source.Child(source.Child(node, "trigger"), "consumes")); consumes != "" {
					carriers(consumes, tokens[1])
				}
				if (source.Child(node, "reads") != nil || source.Child(node, "writes") != nil) && !contains(r.data, tokens[1]) {
					r.data = append(r.data, tokens[1])
				}
			case "paths":
				if len(tokens) == 3 {
					uses(node, source.Str(source.Child(node, "operationId")))
					continue
				}
				for _, m := range []string{"get", "post", "put", "patch", "delete"} {
					if op := source.Child(node, m); op != nil {
						uses(op, source.Str(source.Child(op, "operationId")))
					}
				}
			}
		}
	}
	return r
}
