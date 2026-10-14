package validate

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// Given is the specifications given together in one run, by their
// info.name; a specification without one is not listed (ADR-112).
type Given map[string][]*spec.Spec

// NewGiven lists the specifications by name.
func NewGiven(specs []*spec.Spec) Given {
	g := Given{}
	for _, s := range specs {
		if name := SpecName(s.Root); name != "" {
			g[name] = append(g[name], s)
		}
	}
	return g
}

// SpecName is a specification's info.name, or "".
func SpecName(root *yaml.Node) string {
	return source.Str(source.Child(source.Child(root, "info"), "name"))
}

// A Reference names an element of another specification through a
// dependency: <dependency>#/<pointer>.
type Reference struct {
	Path          string // where it stands in this specification
	Line          int
	Text          string // as written
	Dependency    string
	Specification string // the dependency's specification
	Version       string // the dependency's version
	Waits         string // why it is not resolved
}

// crossRef splits a reference into the dependency and the pointer, the
// part after the #; ok is false for a name of this specification.
func crossRef(s string) (dep, pointer string, ok bool) {
	dep, pointer, ok = strings.Cut(s, "#")
	if !ok || dep == "" {
		return "", "", false
	}
	return dep, pointer, true
}

// isCross reports whether a name is a reference into another specification.
func isCross(s string) bool {
	_, _, ok := crossRef(s)
	return ok
}

// callDependency is the dependency a calls entry names: the entry, or its
// part before the # when it names one of the dependency's operations.
func callDependency(s string) string {
	if dep, _, ok := crossRef(s); ok {
		return dep
	}
	return s
}

// ref is one reference found in a specification, with what it stands in.
type ref struct {
	node    *yaml.Node
	path    []string
	text    string
	dep     string
	pointer string
	kind    string     // ref, target, entity, source, picker, calls or fulfilledBy
	owner   *yaml.Node // the relation, page or picker it stands in
	page    *yaml.Node // for a picker, its page
}

// collectRefs finds every reference of a specification, in document order.
func collectRefs(d *design) []ref {
	var out []ref
	add := func(n *yaml.Node, path []string, kind string, owner, page *yaml.Node) {
		dep, ptr, ok := crossRef(source.Str(n))
		if !ok {
			return
		}
		out = append(out, ref{node: n, path: path, text: n.Value, dep: dep, pointer: ptr, kind: kind, owner: owner, page: page})
	}
	walk(d.root, nil, func(n *yaml.Node, path []string) {
		at := func(key string) []string { return append(append([]string{}, path...), key) }
		if v := source.Child(n, "$ref"); source.IsScalar(v) {
			add(v, at("$ref"), "ref", n, nil)
		}
		switch {
		case len(path) == 4 && path[0] == "entities" && path[2] == "relations":
			add(source.Child(n, "target"), at("target"), "target", n, nil)
		case len(path) == 2 && path[0] == "requirements":
			add(source.Child(n, "fulfilledBy"), at("fulfilledBy"), "fulfilledBy", n, nil)
		case len(path) == 2 && path[0] == "pages":
			add(source.Child(n, "entity"), at("entity"), "entity", n, nil)
			add(source.Child(n, "source"), at("source"), "source", n, nil)
		case len(path) == 4 && path[0] == "pages" && path[2] == "pickers":
			add(source.Child(n, "source"), at("source"), "picker", n, d.pages[path[1]])
		case len(path) == 3 && path[0] == "paths" || len(path) == 2 && path[0] == "jobs":
			for i, e := range source.Items(source.Child(n, "calls")) {
				add(e, append(at("calls"), fmt.Sprint(i)), "calls", n, nil)
			}
		}
	})
	return out
}

// other is the specification a reference resolves in, read once.
type other struct {
	spec    *spec.Spec
	name    string
	design  *design
	version string
}

// referenceResult is what resolving one reference found: an error to
// report, or why it waits.
type referenceResult struct {
	problems []referenceProblem
	waits    string
}

type referenceProblem struct {
	node    *yaml.Node
	path    []string
	message string
}

// resolver resolves the references of one specification against the
// specifications given with it.
type resolver struct {
	d      *design
	given  Given
	others map[string]*other
}

func newResolver(d *design, given Given) *resolver {
	return &resolver{d: d, given: given, others: map[string]*other{}}
}

// otherOf is the one specification given under a name, nil when none is,
// with a problem when several are.
func (rv *resolver) otherOf(name string) (*other, string) {
	list := rv.given[name]
	switch len(list) {
	case 0:
		return nil, ""
	case 1:
	default:
		var dirs []string
		for _, s := range list {
			dirs = append(dirs, s.Dir)
		}
		return nil, fmt.Sprintf("the specifications given in %s are all named %s, so it cannot say which one is meant; give each its own info.name", strings.Join(dirs, " and "), name)
	}
	if o := rv.others[name]; o != nil {
		return o, ""
	}
	s := list[0]
	if s.Root == nil {
		return nil, "" // it did not parse; its own errors say why
	}
	o := &other{spec: s, name: name, design: newDesign(s.Root), version: source.Str(source.Child(source.Child(s.Root, "info"), "version"))}
	rv.others[name] = o
	return o, ""
}

// resolve checks one reference: what can be checked in this specification
// alone, and, when the other specification is given, the element it
// names.
func (rv *resolver) resolve(r ref) referenceResult {
	d := rv.d
	var res referenceResult
	fail := func(n *yaml.Node, path []string, format string, args ...any) referenceResult {
		res.problems = append(res.problems, referenceProblem{node: n, path: path, message: fmt.Sprintf(format, args...)})
		return res
	}
	depNode := d.dependencies[r.dep]
	if depNode == nil {
		if r.kind == "calls" {
			return res // the dependency check reports it
		}
		return fail(r.node, r.path, "%s is not a dependency of the specification, so %s names nothing; declare it under dependencies with the specification and version it names%s", r.dep, r.text, suggest(r.dep, d.dependencies))
	}
	name := source.Str(source.Child(depNode, "specification"))
	if name == "" {
		return fail(r.node, r.path, "%s is a dependency with no specification, so %s cannot name an element of it; add specification and version to %s, or name the system alone", r.dep, r.text, r.dep)
	}
	switch r.kind {
	case "target":
		if k := source.Str(source.Child(r.owner, "kind")); k != "many-to-one" && k != "one-to-one" {
			return fail(r.node, r.path, "%s is a %s relation, whose via is in the other module's tables or is a join entity; only a many-to-one or one-to-one relation, whose via is a field this entity keeps, may lead to another specification's entity", r.path[3], k)
		}
	case "entity", "source":
		if k := source.Str(source.Child(r.owner, "kind")); k != "list" {
			return fail(r.node, r.path, "%s is a %s page, and only a list page may show the records of another specification; show them on a list page, or keep the page's %s in this specification", r.path[1], k, r.kind)
		}
		other := source.Str(source.Child(r.owner, map[string]string{"entity": "source", "source": "entity"}[r.kind]))
		if odep, _, ok := crossRef(other); !ok || odep != r.dep {
			return fail(r.node, r.path, "%s's %s is %s, of another specification, and its %s is not of %s too; a list of another specification shows that specification's entity, so write both through %s", r.path[1], r.kind, r.text, map[string]string{"entity": "source", "source": "entity"}[r.kind], r.dep, r.dep)
		}
	case "picker":
		ent := d.entities[source.Str(source.Child(r.page, "entity"))]
		relation, target := pickerRelation(ent, r.path[3])
		if relation == "" {
			return res // the picker check reports it
		}
		if tdep, _, ok := crossRef(target); !ok || tdep != r.dep {
			return fail(r.node, r.path, "%s is a list of another specification, and the relation %s leads to %s; pick a record from a list of the relation's target", r.text, relation, target)
		}
	}
	o, problem := rv.otherOf(name)
	if problem != "" {
		return fail(r.node, r.path, "%s", problem)
	}
	if o == nil {
		res.waits = name + " is not given"
		return res
	}
	od := o.design
	tokens := pointerTokens(r.pointer)
	about := fmt.Sprintf("%s %s", o.name, o.version)
	if len(tokens) == 2 {
		section, el := tokens[0], tokens[1]
		elements := map[string]map[string]*yaml.Node{"entities": od.entities, "enums": od.enums, "views": od.views, "schemas": od.schemas, "requirements": od.requirements}[section]
		if elements == nil || elements[el] == nil {
			return fail(r.node, r.path, "%s is not %s of %s%s", el, map[string]string{"entities": "an entity", "enums": "an enum", "views": "a view", "schemas": "a schema", "requirements": "a requirement"}[section], about, suggest(el, elements))
		}
	}
	var op operation
	if len(tokens) == 3 && tokens[0] == "paths" {
		for _, x := range od.opList {
			if x.path == tokens[1] && x.method == tokens[2] {
				op = x
			}
		}
		if op.node == nil {
			return fail(r.node, r.path, "%s %s is not an operation of %s", strings.ToUpper(tokens[2]), tokens[1], about)
		}
	}
	switch r.kind {
	case "source", "picker":
		listed := source.Str(source.Child(source.Child(op.node, "listOf"), "entity"))
		want := source.Str(source.Child(r.page, "entity"))
		if r.kind == "source" {
			want = source.Str(source.Child(r.owner, "entity"))
		} else {
			_, want = pickerRelation(d.entities[want], r.path[3])
		}
		_, wantPtr, _ := crossRef(want)
		wantName := strings.TrimPrefix(wantPtr, "/entities/")
		if listed != wantName {
			return fail(r.node, r.path, "%s %s of %s does not list %s; name an operation of %s whose listOf names %s", strings.ToUpper(op.method), op.path, about, wantName, o.name, wantName)
		}
		fields := fieldsOf(od.entities[wantName])
		if r.kind == "source" {
			for _, key := range []string{"columns", "filters"} {
				for i, f := range source.Items(source.Child(r.owner, key)) {
					if fields[f.Value] == nil {
						res.problems = append(res.problems, referenceProblem{node: f, path: []string{"pages", r.path[1], key, fmt.Sprint(i)}, message: fmt.Sprintf("%s is not a field of %s in %s%s", f.Value, wantName, about, suggest(f.Value, fields))})
					}
				}
			}
		} else {
			base := []string{"pages", r.path[1], "pickers", r.path[3]}
			for i, f := range source.Items(source.Child(r.owner, "shows")) {
				if fields[f.Value] == nil {
					res.problems = append(res.problems, referenceProblem{node: f, path: append(base, "shows", fmt.Sprint(i)), message: fmt.Sprintf("%s is not a field of %s in %s, so the picker cannot show it%s", f.Value, wantName, about, suggest(f.Value, fields))})
				}
			}
			for _, kv := range source.Pairs(source.Child(r.owner, "fills")) {
				if fields[kv.Value.Value] == nil {
					res.problems = append(res.problems, referenceProblem{node: kv.Value, path: append(base, "fills", kv.Key.Value), message: fmt.Sprintf("%s is not a field of %s in %s, so the picker cannot fill from it%s", kv.Value.Value, wantName, about, suggest(kv.Value.Value, fields))})
				}
			}
		}
		if len(res.problems) > 0 {
			return res
		}
	}
	if q := blockingQuestion(o.spec.Root, source.Pointer(tokens...)); q != "" {
		res.waits = fmt.Sprintf("%s is blocked in %s by the must question %s", source.Pointer(tokens...), o.name, q)
	}
	return res
}

// pointerTokens splits the part of a reference after the # into its
// unescaped tokens.
func pointerTokens(ptr string) []string {
	ptr = strings.TrimPrefix(ptr, "/")
	if ptr == "" {
		return nil
	}
	parts := strings.Split(ptr, "/")
	for i, p := range parts {
		parts[i] = source.UnescapeToken(p)
	}
	return parts
}

// blockingQuestion is the first open must question, by ID, whose blocks
// name the element at the pointer or one it is inside, or "".
func blockingQuestion(root *yaml.Node, ptr string) string {
	var ids []string
	for _, p := range source.Pairs(source.Child(root, "questions")) {
		if source.Str(source.Child(p.Value, "priority")) != "must" {
			continue
		}
		for _, item := range source.Items(source.Child(p.Value, "blocks")) {
			bl, ok := spec.ParseBlock(item.Value)
			if !ok || !bl.IsPointer() {
				continue
			}
			e := source.Pointer(bl.Tokens...)
			if ptr == e || strings.HasPrefix(ptr, e+"/") {
				ids = append(ids, p.Key.Value)
				break
			}
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

// checkReferences reports every reference that is wrong, and every
// dependency whose version is not the one the specification it names is
// at, when that specification is given.
func (c *checker) checkReferences(d *design) {
	rv := newResolver(d, c.given)
	for _, r := range collectRefs(d) {
		for _, p := range rv.resolve(r).problems {
			c.add(p.node, source.Pointer(p.path...), RuleReference, "%s", p.message)
		}
	}
	for _, name := range sortedKeys(d.dependencies) {
		dep := d.dependencies[name]
		spName := source.Str(source.Child(dep, "specification"))
		vn := source.Child(dep, "version")
		if spName == "" || vn == nil {
			continue
		}
		o, _ := rv.otherOf(spName)
		if o != nil && o.version != vn.Value {
			c.add(vn, source.Pointer("dependencies", name, "version"), RuleReference, "%s is at version %s, and %s names %s; check the references to it against %s, then write that version here", spName, o.version, name, vn.Value, o.version)
		}
	}
}

// References lists the references of a specification that are not
// resolved: the specification they name is not given, or the element is
// blocked there by a must question. A wrong reference is an error and is
// not listed.
func References(s *spec.Spec, given Given) []Reference {
	if s.Root == nil {
		return nil
	}
	d := newDesign(s.Root)
	rv := newResolver(d, given)
	var out []Reference
	for _, r := range collectRefs(d) {
		res := rv.resolve(r)
		if len(res.problems) > 0 || res.waits == "" {
			continue
		}
		dep := d.dependencies[r.dep]
		out = append(out, Reference{
			Path: source.Pointer(r.path...), Line: r.node.Line, Text: r.text, Dependency: r.dep,
			Specification: source.Str(source.Child(dep, "specification")),
			Version:       source.Str(source.Child(dep, "version")),
			Waits:         res.waits,
		})
	}
	return out
}

// GeneratorReferences are the errors a code target meets in the references
// of a specification, since generate reads one specification at a time: a
// value typed by another specification's element (an entity's or view's
// field for sql, any for the other targets), and for the UI targets a list
// page or picker that reads another specification's list. A relation's
// target is not among them: its via field is this specification's.
func GeneratorReferences(s *spec.Spec, target string) []Diagnostic {
	if s.Root == nil || target == "bpmn" {
		return nil
	}
	ui := target == "ui" || target == "ui-typescript"
	c := &checker{file: s.RootFile, files: s.Files}
	d := newDesign(s.Root)
	for _, r := range collectRefs(d) {
		name := source.Str(source.Child(d.dependencies[r.dep], "specification"))
		switch {
		case r.kind == "ref" && (target != "sql" || r.path[0] == "entities" || r.path[0] == "views"):
			c.add(r.node, source.Pointer(r.path...), RuleReference, "generate %s reads one specification, and this value is typed by %s, of %s, which it does not read; give the value its own type here, or keep it out of what %s writes", target, r.text, name, target)
		case ui && (r.kind == "source" || r.kind == "picker"):
			c.add(r.node, source.Pointer(r.path...), RuleReference, "generate %s reads one specification, and this page reads the list %s, of %s, which it does not read, so it cannot write the list's columns or its call", target, r.text, name)
		}
	}
	SpecPlacer(s).Place(c.diags)
	Sort(c.diags)
	return c.diags
}
