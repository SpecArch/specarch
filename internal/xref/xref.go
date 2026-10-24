// Package xref reads, for a generator, the elements a specification names
// in another specification of its estate through a dependency, as
// <dependency>#/<pointer> (ADR-112). specarch hands a plug-in the
// specifications of the estate's modules its dependencies name; this
// package imports a schema, an entity or an enum a field is typed by into
// the specification the generator writes, under the element's own name and
// marked with RefKey, and finds an operation a page or a picker reads
// there, so each generator writes them as it writes its own (ADR-184). A
// reference inside an imported element is the other module's text, so it
// resolves through that module's own dependencies (ADR-185).
package xref

import (
	"fmt"
	"sort"
	"strings"
)

// RefKey marks an element imported from another specification, with the
// module's name and the pointer to the element there, such as
// customer-registry#/schemas/Address. The OpenAPI document carries it as
// it is, beside the schema written inline.
const RefKey = "x-specarch-ref"

// Dependency is a specification another names through a dependency, as
// specarch hands it to a plug-in: the module's info.name and info.version,
// the specification as plain values, the dialect its API is served in,
// dxlib or "" for the standard one, its idioms, and its own dependencies.
// A module reached only through another's dependencies has no dialect or
// idioms, which only a list the specification reads itself uses.
type Dependency struct {
	Specification string         `json:"specification"`
	Version       string         `json:"version"`
	Content       map[string]any `json:"content"`
	APIDialect    string         `json:"apiDialect,omitempty"`
	// Idioms are the idioms of the implementation file that serves the
	// module's API, so a list it answers is paged by its own names.
	Idioms []Idiom `json:"idioms,omitempty"`
	// Dependencies are the module's own dependencies that name a
	// specification, by their key there, so a reference in an element
	// imported from it means what it means in the module.
	Dependencies map[string]Pin `json:"dependencies,omitempty"`
}

// Pin is the specification a dependency names and the info.version its
// references were written against.
type Pin struct {
	Specification string `json:"specification"`
	Version       string `json:"version"`
}

// Idiom is one idiom of a dependency's implementation file, as specarch
// hands every idiom to a plug-in.
type Idiom struct {
	Name     string         `json:"name"`
	Version  string         `json:"version"`
	As       string         `json:"as"`
	Content  map[string]any `json:"content,omitempty"`
	Override map[string]any `json:"override,omitempty"`
}

// Problem is what cannot be imported, at the JSON pointer of the entry
// that names it.
type Problem struct {
	Path    string
	Message string
}

// Parse splits a reference into the dependency's name and the pointer's
// segments, unescaped; ok is false for a reference into the same
// specification or one that is not a pointer.
func Parse(ref string) (dep string, pointer []string, ok bool) {
	i := strings.Index(ref, "#/")
	if i <= 0 {
		return "", nil, false
	}
	return ref[:i], segments(ref[i+2:]), true
}

// segments is a JSON pointer's segments, unescaped, without its leading #/.
func segments(p string) []string {
	var out []string
	for _, s := range strings.Split(p, "/") {
		out = append(out, strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~"))
	}
	return out
}

// Named is the reference as the module's name and the pointer, the form
// RefKey and the messages give it: customer-registry#/schemas/Address.
func Named(deps map[string]Dependency, ref string) string {
	dep, _, ok := Parse(ref)
	d, given := deps[dep]
	if !ok || !given {
		return ref
	}
	return d.Specification + ref[len(dep):]
}

// Operation finds the operation a reference names, as
// <dependency>#/paths/<path>/<method>: it, its path and its method, and
// the dependency; ok is false when the reference is not of that form or
// names no operation of a specification given.
func Operation(deps map[string]Dependency, ref string) (op map[string]any, path, method string, d Dependency, ok bool) {
	dep, ptr, parsed := Parse(ref)
	d, given := deps[dep]
	if !parsed || !given || len(ptr) != 3 || ptr[0] != "paths" {
		return nil, "", "", Dependency{}, false
	}
	op, ok = obj(obj0(obj0(d.Content["paths"])[ptr[1]])[ptr[2]])
	return op, ptr[1], ptr[2], d, ok
}

// Entity finds the entity a reference names, as
// <dependency>#/entities/<name>: it, its name and the dependency.
func Entity(deps map[string]Dependency, ref string) (entity map[string]any, name string, d Dependency, ok bool) {
	dep, ptr, parsed := Parse(ref)
	d, given := deps[dep]
	if !parsed || !given || len(ptr) != 2 || ptr[0] != "entities" {
		return nil, "", Dependency{}, false
	}
	entity, ok = obj(obj0(d.Content["entities"])[ptr[1]])
	return entity, ptr[1], d, ok
}

// Import is the specification with every value typed by another
// specification's element typed by a copy of it in its own section: a
// schema or an entity as a schema of the same name, holding the entity's
// properties and required fields only, and an enum as an enum. A copy
// marks where it came from with RefKey, and what it names in its own
// specification is imported in turn, through that specification's own
// dependencies: modules holds, by info.name, the specifications they reach
// that deps does not. The specification given is not changed. A reference
// of this specification whose dependency is not given is left as it is,
// since specarch refuses it before the plug-in runs. A view, a name this
// specification has already, a schema that holds itself through another
// specification, and a module a copy names that the run does not give are
// problems at the entry, and the value is left untyped.
func Import(spec map[string]any, deps, modules map[string]Dependency) (map[string]any, []Problem) {
	out, _ := deepCopy(spec).(map[string]any)
	if len(deps) == 0 || out == nil {
		return out, nil
	}
	byName := map[string]Dependency{}
	for name, d := range modules {
		byName[name] = d
	}
	for _, d := range deps {
		byName[d.Specification] = d
	}
	self, _ := obj0(out["info"])["name"].(string)
	if self == "" {
		self = "this specification"
	}
	im := &importer{spec: out, deps: deps, byName: byName, self: self, done: map[string]string{}, busy: map[string]bool{}}
	for _, k := range sortedKeys(out) {
		if k == "tests" || k == "questions" || k == "decisions" || k == "requirements" {
			continue
		}
		im.walk(out[k], "/"+escape(k), nil)
	}
	return out, im.problems
}

type importer struct {
	spec     map[string]any
	deps     map[string]Dependency
	byName   map[string]Dependency // every module given, by info.name
	self     string                // this specification's info.name
	done     map[string]string     // reference as module#/pointer: its local reference
	busy     map[string]bool       // references being imported, to find a cycle
	chain    []string              // the references being imported, outermost first
	problems []Problem
}

// walk imports every $ref under v. within is the module whose
// specification v was copied from, nil for this one: a reference there is
// that module's, through its own dependencies, and a problem in it is
// reported at the entry of this specification that named the copy, at.
func (im *importer) walk(v any, at string, within *Dependency) {
	child := func(k string) string {
		if within != nil {
			return at
		}
		return at + "/" + k
	}
	switch t := v.(type) {
	case map[string]any:
		for _, k := range sortedKeys(t) {
			if k == "$ref" {
				if r, ok := t[k].(string); ok {
					if local, ok := im.ref(r, child("$ref"), within); ok {
						t[k] = local
					} else if local == "" {
						delete(t, k)
					}
				}
				continue
			}
			im.walk(t[k], child(escape(k)), within)
		}
	case []any:
		for i, e := range t {
			im.walk(e, child(fmt.Sprint(i)), within)
		}
	}
}

// ref imports what one reference names and answers the local reference
// that replaces it; ok false with local unchanged ("-") leaves the
// reference as it is, ok false with local "" drops it after a problem.
func (im *importer) ref(r, at string, within *Dependency) (local string, ok bool) {
	var d Dependency
	var ptr []string
	var named string
	switch {
	case within != nil && strings.HasPrefix(r, "#/"):
		ptr, d, named = segments(r[2:]), *within, within.Specification+r
	case within != nil:
		dep, p, parsed := Parse(r)
		if !parsed {
			return "-", false
		}
		pin, declared := within.Dependencies[dep]
		if !declared {
			im.problem(at, "%s names %s, and %s has no dependency %s that names a specification, so the value cannot be typed by it; correct %s", im.outer(), r, within.Specification, dep, within.Specification)
			return "", false
		}
		named = pin.Specification + r[len(dep):]
		var given bool
		if d, given = im.byName[pin.Specification]; !given {
			im.problem(at, "%s names %s through %s's dependency %s on %s %s (%s), and this run does not give %s, so the value cannot be typed by it; give %s in the run, as a module of the estate or a folder beside the others; %s need not declare it", im.outer(), named, within.Specification, dep, pin.Specification, pin.Version, im.path(pin.Specification), pin.Specification, pin.Specification, im.self)
			return "", false
		}
		ptr = p
	default:
		dep, p, parsed := Parse(r)
		var given bool
		if d, given = im.deps[dep]; !parsed || !given {
			return "-", false
		}
		ptr, named = p, d.Specification+r[len(dep):]
	}
	if l, seen := im.done[named]; seen {
		return l, true
	}
	if im.busy[named] {
		im.problem(at, "%s holds itself through %s, so a copy of it would never end; type the value in this specification", named, named)
		return "", false
	}
	if len(ptr) != 2 {
		im.problem(at, "%s names no schema, entity or enum, so the value cannot be typed by it", named)
		return "", false
	}
	section, name := ptr[0], ptr[1]
	el, found := obj(obj0(d.Content[section])[name])
	if !found {
		im.problem(at, "%s is not in %s %s, so the value cannot be typed by it", named, d.Specification, d.Version)
		return "", false
	}
	var copied map[string]any
	into := section
	switch section {
	case "schemas":
		copied, _ = deepCopy(el).(map[string]any)
	case "entities":
		copied = map[string]any{}
		for _, k := range []string{"type", "title", "description", "properties", "required"} {
			if v, ok := el[k]; ok {
				copied[k] = deepCopy(v)
			}
		}
		into = "schemas"
	case "enums":
		copied, _ = deepCopy(el).(map[string]any)
	case "views":
		im.problem(at, "%s is a view of %s, and a value is typed by a module's schema, entity or enum, not by a view, whose fields are read from its entity there; type it by the entity or a schema of %s", named, d.Specification, d.Specification)
		return "", false
	default:
		im.problem(at, "%s names no schema, entity or enum, so the value cannot be typed by it", named)
		return "", false
	}
	if mine, taken := obj(im.spec[into]); taken {
		if _, clash := mine[name]; clash {
			im.problem(at, "%s would be written as %s %s, which this specification already declares; rename one of them, or type the value by this specification's own %s", named, singular(into), name, name)
			return "", false
		}
	}
	for _, other := range []string{"schemas", "entities", "views", "enums"} {
		if other == into {
			continue
		}
		if _, clash := obj0(im.spec[other])[name]; clash {
			im.problem(at, "%s would be written as %s %s, and this specification has a %s of that name, and the generators write both under one name; rename one of them", named, singular(into), name, singular(other))
			return "", false
		}
	}
	local = "#/" + into + "/" + name
	im.busy[named] = true
	im.chain = append(im.chain, named)
	im.walk(copied, at, &d)
	im.chain = im.chain[:len(im.chain)-1]
	delete(im.busy, named)
	copied[RefKey] = named
	sec := obj0(im.spec[into])
	if sec == nil {
		sec = map[string]any{}
		im.spec[into] = sec
	}
	sec[name] = copied
	im.done[named] = local
	return local, true
}

// outer is the imported element the reference being read stands in: the
// innermost of the copies being imported.
func (im *importer) outer() string {
	return im.chain[len(im.chain)-1]
}

// path is the chain of modules from this specification through the copies
// being imported to the module named last, as loans → customer-registry
// → places.
func (im *importer) path(last string) string {
	mods := []string{im.self}
	for _, c := range im.chain {
		mod, _, _ := strings.Cut(c, "#")
		if mod != mods[len(mods)-1] {
			mods = append(mods, mod)
		}
	}
	return strings.Join(append(mods, last), " → ")
}

func (im *importer) problem(at, format string, args ...any) {
	im.problems = append(im.problems, Problem{Path: at, Message: fmt.Sprintf(format, args...)})
}

// Imported is the reference an element was imported from, from its
// RefKey, or "".
func Imported(el map[string]any) string {
	s, _ := el[RefKey].(string)
	return s
}

func singular(section string) string {
	switch section {
	case "schemas":
		return "schema"
	case "entities":
		return "entity"
	case "enums":
		return "enum"
	}
	return "view"
}

func escape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

func deepCopy(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, e := range t {
			m[k] = deepCopy(e)
		}
		return m
	case []any:
		l := make([]any, len(t))
		for i, e := range t {
			l[i] = deepCopy(e)
		}
		return l
	}
	return v
}

func obj(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func obj0(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
