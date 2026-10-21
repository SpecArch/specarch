package validate

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// An Estate is an estate file as read from disk: the modules'
// specifications it lists and pins (ADR-113).
type Estate struct {
	Path    string
	Root    *yaml.Node // nil when nothing of the file could be read
	Modules []Module   // in the order the file lists them
	doc     *source.Doc
}

// A Module is one module an estate lists.
type Module struct {
	Key      string
	Location string // the folder of its specification, from the estate's folder; "" when the entry gives none
	Version  string
	key      *yaml.Node
	value    *yaml.Node
}

// LocationText is the module's location as the estate writes it.
func (m Module) LocationText() string {
	return source.Str(source.Child(m.value, "location"))
}

// Description is the estate's description of the module.
func (m Module) Description() string {
	return strings.TrimSpace(source.Str(source.Child(m.value, "description")))
}

// LoadEstate reads an estate file. Only a file that cannot be read is an
// error; what is wrong inside it is reported by CheckEstate.
func LoadEstate(path string) (*Estate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	e := &Estate{Path: path, doc: source.Parse(data)}
	e.Root = e.doc.Root
	if e.Root == nil || e.Root.Kind != yaml.MappingNode {
		return e, nil
	}
	for _, p := range source.Pairs(source.Child(e.Root, "modules")) {
		m := Module{Key: p.Key.Value, key: p.Key, value: p.Value, Version: source.Str(source.Child(p.Value, "version"))}
		if loc := source.Str(source.Child(p.Value, "location")); loc != "" {
			m.Location = filepath.Join(filepath.Dir(path), filepath.FromSlash(loc))
		}
		e.Modules = append(e.Modules, m)
	}
	return e, nil
}

// Roots are the folders of the modules whose specification is there: the
// ones the estate gives the run to check with it.
func (e *Estate) Roots() []string {
	var out []string
	for _, m := range e.Modules {
		if m.Location != "" && isFile(filepath.Join(m.Location, spec.RootFile)) {
			out = append(out, m.Location)
		}
	}
	return out
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// CheckEstate checks an estate against the specifications read in the
// run, by their folder: that each module is where it is listed, named and
// at the version the estate lists, and that no module's calls come back to
// it through the others.
func CheckEstate(e *Estate, specs map[string]*spec.Spec) []Diagnostic {
	c := &checker{file: e.Path}
	for _, p := range e.doc.Problems {
		c.addLine(p.Line, p.Path, Rule(p.Rule), "%s", p.Message)
		c.detail(p.Detail)
	}
	if e.Root != nil {
		c.root = e.Root
		c.checkSchema(KindEstate, e.doc.Value)
		if e.Root.Kind == yaml.MappingNode {
			c.checkModules(e, specs)
		}
	}
	ds := withoutEchoes(withoutHeld(c.diags, heldIn(e.Path, e.doc)))
	(&Placer{Dir: filepath.Dir(e.Path)}).Place(ds)
	Sort(ds)
	return ds
}

func (c *checker) checkModules(e *Estate, specs map[string]*spec.Spec) {
	found := map[string]*spec.Spec{}
	for _, m := range e.Modules {
		if m.Location == "" {
			continue // the schema reports it
		}
		ptr := source.Pointer("modules", m.Key)
		loc := source.Str(source.Child(m.value, "location"))
		s := specs[m.Location]
		if s == nil {
			why := "it holds no " + spec.RootFile
			if _, err := os.Stat(m.Location); err != nil {
				why = "there is no such folder"
			}
			c.add(source.Child(m.value, "location"), ptr+"/location", RuleModule,
				"%s is listed at %s, and %s; write the folder that holds the module's %s, relative to %s", m.Key, loc, why, spec.RootFile, filepath.Base(e.Path))
			continue
		}
		if s.Root == nil || s.Root.Kind != yaml.MappingNode {
			continue // the specification's own check reports it
		}
		found[m.Key] = s
		info := source.Child(s.Root, "info")
		switch name := SpecName(s.Root); {
		case name == "":
			c.add(m.key, ptr, RuleModule, "the specification at %s has no info.name, so it cannot be told to be %s; add name: %s to its info", loc, m.Key, m.Key)
		case name != m.Key:
			c.add(m.key, ptr, RuleModule, "the specification at %s is named %s, not %s; list it under its info.name, or correct the name", loc, name, m.Key)
		}
		if v := source.Str(source.Child(info, "version")); v != "" && m.Version != "" && v != m.Version {
			c.add(source.Child(m.value, "version"), ptr+"/version", RuleModule,
				"%s is at version %s, and the estate lists %s; check it against the other modules, then write %s here", m.Key, v, m.Version, v)
		}
	}
	// An access token is minted for one service, so no two modules accept
	// one audience value; every module of a trust domain accepts the one
	// transaction token, whose audience is the trust domain (ADR-109).
	first := map[string]string{}
	for _, m := range e.Modules {
		s := found[m.Key]
		if s == nil {
			continue
		}
		for _, aud := range newDesign(s.Root).acceptedAudiences() {
			if other, ok := first[aud]; ok {
				c.add(m.key, source.Pointer("modules", m.Key), RuleTokenAudience,
					"%s and %s both accept an access token for the audience %s, so a token minted for one is accepted by the other; give each module an audience of its own", other, m.Key, aud)
				continue
			}
			first[aud] = m.Key
		}
	}
	edges := moduleCalls(found)
	for _, m := range e.Modules {
		if cycle := callCycle(m.Key, edges); cycle != nil {
			c.add(m.key, source.Pointer("modules", m.Key), RuleModule,
				"the operations of %s call round a cycle of modules: %s; a call waits on one that waits on it, so drop one of the calls or pass that work by a channel message", m.Key, cycleText(cycle))
		}
	}
}

// moduleCalls are, for each module, the modules its operations and jobs
// call through a dependency whose specification is one of them, sorted.
func moduleCalls(found map[string]*spec.Spec) map[string][]string {
	edges := map[string][]string{}
	for key, s := range found {
		deps := map[string]string{}
		for _, d := range source.Pairs(source.Child(s.Root, "dependencies")) {
			deps[d.Key.Value] = source.Str(source.Child(d.Value, "specification"))
		}
		var callers []*yaml.Node
		for _, p := range source.Pairs(source.Child(s.Root, "paths")) {
			for _, op := range source.Pairs(p.Value) {
				callers = append(callers, op.Value)
			}
		}
		for _, j := range source.Pairs(source.Child(s.Root, "jobs")) {
			callers = append(callers, j.Value)
		}
		seen := map[string]bool{}
		for _, n := range callers {
			for _, e := range source.Items(source.Child(n, "calls")) {
				to := deps[callDependency(e.Value)]
				if found[to] != nil && !seen[to] {
					seen[to] = true
					edges[key] = append(edges[key], to)
				}
			}
		}
		sort.Strings(edges[key])
	}
	return edges
}

// callCycle is the shortest cycle of calls through module key, starting
// at the smallest key on it, or nil when key is on none.
func callCycle(key string, edges map[string][]string) []string {
	var best []string
	for _, start := range cycleMembers(key, edges) {
		path := shortestPath(start, start, edges)
		if path != nil && contains(path, key) && (best == nil || len(path) < len(best)) {
			best = path
		}
	}
	return best
}

// cycleMembers are the modules from which key is reached and which key
// reaches, sorted: those that share a cycle with it.
func cycleMembers(key string, edges map[string][]string) []string {
	var out []string
	keys := make([]string, 0, len(edges))
	for k := range edges {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if shortestPath(key, k, edges) != nil && shortestPath(k, key, edges) != nil {
			out = append(out, k)
		}
	}
	return out
}

// shortestPath is the shortest path of one call or more from one module to
// another, taking each module's calls in order, or nil.
func shortestPath(from, to string, edges map[string][]string) []string {
	prev := map[string]string{}
	queue := []string{from}
	visited := map[string]bool{}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, next := range edges[n] {
			if next == to {
				path := []string{to}
				for at := n; at != from; at = prev[at] {
					path = append(path, at)
				}
				path = append(path, from)
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
				return path
			}
			if !visited[next] && next != from {
				visited[next] = true
				prev[next] = n
				queue = append(queue, next)
			}
		}
	}
	return nil
}

// cycleText says a cycle in words: a calls b, which calls a.
func cycleText(cycle []string) string {
	var b strings.Builder
	b.WriteString(cycle[0])
	for i, k := range cycle[1:] {
		if i == 0 {
			b.WriteString(" calls ")
		} else {
			b.WriteString(", which calls ")
		}
		b.WriteString(k)
	}
	return b.String()
}
