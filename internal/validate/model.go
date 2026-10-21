package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// A Position is where a node of the model starts: its key in a mapping,
// or the item itself in a list, in the file it came from (ADR-138).
type Position struct {
	File   string
	Line   int
	Column int
}

// Positions maps the pointer of every node of a tree to its position: the
// root is "/", as the problems file writes it, and a key a mapping repeats
// is left out after its first entry, as the document's values leave it
// out. files names the file of each node; a node it does not name is in
// file.
func Positions(root *yaml.Node, files map[*yaml.Node]string, file string) map[string]Position {
	out := map[string]Position{}
	if root == nil {
		return out
	}
	fileOf := func(n *yaml.Node) string {
		if f, ok := files[n]; ok {
			return f
		}
		return file
	}
	var visit func(start, value *yaml.Node, ptr string)
	visit = func(start, value *yaml.Node, ptr string) {
		key := ptr
		if key == "" {
			key = "/"
		}
		out[key] = Position{File: fileOf(start), Line: start.Line, Column: start.Column}
		value = source.Deref(value)
		switch value.Kind {
		case yaml.MappingNode:
			seen := map[string]bool{}
			for _, p := range source.Pairs(value) {
				if seen[p.Key.Value] {
					continue
				}
				seen[p.Key.Value] = true
				visit(p.Key, p.Value, ptr+"/"+source.EscapeToken(p.Key.Value))
			}
		case yaml.SequenceNode:
			for i, item := range source.Items(value) {
				visit(item, item, fmt.Sprintf("%s/%d", ptr, i))
			}
		}
	}
	visit(root, root, "")
	return out
}

// A ModelReference is one resolved reference of a specification: the
// pointer of the entry that refers, the pointer of the element it names,
// and the keyword that made it (ADR-138).
type ModelReference struct {
	From    string
	To      string
	Through string
}

// The kinds of element a reference keyword names, each with how a name
// finds its element.
const (
	toRequirement = "requirement"
	toDecision    = "decision"
	toPermission  = "permission"
	toEntity      = "entity"
	toRole        = "role"
	toMessage     = "message"
	toDependency  = "dependency"
	toPage        = "page"
	toOperation   = "operation"
	toCommand     = "command"
	toClient      = "client"
	toToken       = "token"
	toHandshake   = "handshake"
	toMiddleware  = "middleware"
	toJob         = "job"
	toGateway     = "gateway"
	toWorkflow    = "workflow"
	toFlow        = "flow"
)

// referencePlaces are where a keyword names an element of the
// specification by its name: a path of keys, * for any key or item, and
// the kind of element named. satisfies, verifies and decidedIn name a
// requirement or a decision wherever they stand, and $ref a pointer. The
// keyword of a place is its last key that is not *.
var referencePlaces = []struct {
	path []string
	kind string
}{
	{[]string{"entities", "*", "relations", "*", "target"}, toEntity},
	{[]string{"paths", "*", "*", "guard", "entity"}, toEntity},
	{[]string{"commands", "*", "guard", "entity"}, toEntity},
	{[]string{"paths", "*", "*", "listOf", "entity"}, toEntity},
	{[]string{"pages", "*", "entity"}, toEntity},
	{[]string{"jobs", "*", "reads"}, toEntity},
	{[]string{"jobs", "*", "writes"}, toEntity},
	{[]string{"commands", "*", "reads"}, toEntity},
	{[]string{"commands", "*", "writes"}, toEntity},
	{[]string{"workflows", "*", "subject"}, toEntity},
	{[]string{"tests", "*", "entity"}, toEntity},
	{[]string{"paths", "*", "*", "permission"}, toPermission},
	{[]string{"commands", "*", "permission"}, toPermission},
	{[]string{"pages", "*", "permission"}, toPermission},
	{[]string{"pages", "*", "actions", "*", "permission"}, toPermission},
	{[]string{"workflows", "*", "steps", "*", "permission"}, toPermission},
	{[]string{"roles", "*", "permissions"}, toPermission},
	{[]string{"jobs", "*", "role"}, toRole},
	{[]string{"jobs", "*", "deadLetter", "role"}, toRole},
	{[]string{"workflows", "*", "steps", "*", "approvers"}, toRole},
	{[]string{"clients", "*", "roles"}, toRole},
	{[]string{"signoff", "signers", "*", "role"}, toRole},
	{[]string{"paths", "*", "*", "emits"}, toMessage},
	{[]string{"jobs", "*", "emits"}, toMessage},
	{[]string{"workflows", "*", "emits"}, toMessage},
	{[]string{"jobs", "*", "trigger", "consumes"}, toMessage},
	{[]string{"paths", "*", "*", "calls"}, toDependency},
	{[]string{"jobs", "*", "calls"}, toDependency},
	{[]string{"gateways", "*", "calls"}, toDependency},
	{[]string{"gateways", "*", "forwards", "*", "dependency"}, toDependency},
	{[]string{"channels", "*", "carriedBy"}, toDependency},
	{[]string{"menus", "*", "page"}, toPage},
	{[]string{"menus", "*", "items", "*", "page"}, toPage},
	{[]string{"menus", "*", "items", "*", "items", "*", "page"}, toPage},
	{[]string{"menus", "*", "items", "*", "items", "*", "items", "*", "page"}, toPage},
	{[]string{"flows", "*", "steps", "*", "page"}, toPage},
	{[]string{"tests", "*", "page"}, toPage},
	{[]string{"workflows", "*", "trigger"}, toOperation},
	{[]string{"workflows", "*", "steps", "*", "operation"}, toOperation},
	{[]string{"tests", "*", "operation"}, toOperation},
	{[]string{"tests", "*", "command"}, toCommand},
	{[]string{"paths", "*", "*", "callers"}, toClient},
	{[]string{"commands", "*", "callers"}, toClient},
	{[]string{"paths", "*", "*", "via"}, toClient},
	{[]string{"roles", "*", "through", "*"}, toClient},
	{[]string{"paths", "*", "*", "token"}, toToken},
	{[]string{"clients", "*", "proof", "token"}, toToken},
	{[]string{"gateways", "*", "token"}, toToken},
	{[]string{"paths", "*", "*", "handshake"}, toHandshake},
	{[]string{"paths", "*", "*", "middlewares"}, toMiddleware},
	{[]string{"gateways", "*", "middlewares"}, toMiddleware},
	{[]string{"tests", "*", "job"}, toJob},
	{[]string{"tests", "*", "gateway"}, toGateway},
	{[]string{"tests", "*", "workflow"}, toWorkflow},
	{[]string{"tests", "*", "flow"}, toFlow},
	{[]string{"tests", "*", "requirement"}, toRequirement},
}

// keyPlaces are where a key, not its value, names an element: a role's
// through is keyed by the permissions it narrows.
var keyPlaces = []struct {
	path []string
	kind string
}{
	{[]string{"roles", "*", "through", "*"}, toPermission},
}

// placeKeyword is the keyword of a place: its last key that is not *.
func placeKeyword(pattern []string) string {
	for i := len(pattern) - 1; i >= 0; i-- {
		if pattern[i] != "*" {
			return pattern[i]
		}
	}
	return ""
}

// anywhere are the keywords that name an element wherever they stand.
var anywhere = map[string]string{"satisfies": toRequirement, "verifies": toRequirement, "decidedIn": toDecision}

func placeMatches(pattern, path []string) bool {
	if len(pattern) != len(path) {
		return false
	}
	for i, p := range pattern {
		if p != "*" && p != path[i] {
			return false
		}
	}
	return true
}

// ModelReferences lists every reference of a specification that resolves
// to an element of it, in document order: a $ref that is a pointer into
// the specification, and a name under one of the reference keywords that
// names an element. A reference that does not resolve is the validator's
// problem and is left out; so is one into another specification.
func ModelReferences(root *yaml.Node) []ModelReference {
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	d := newDesign(root)
	var out []ModelReference
	target := func(kind, name string) string {
		section := ""
		switch kind {
		case toRequirement:
			section = "requirements"
		case toDecision:
			section = "decisions"
		case toPermission:
			section = "permissions"
		case toEntity:
			if d.entities[name] != nil {
				return source.Pointer("entities", name)
			}
			section = "views"
		case toRole:
			section = "roles"
		case toMessage:
			if d.message(name) == nil {
				return ""
			}
			ch, msg, _ := strings.Cut(name, "/")
			return source.Pointer("channels", ch, "messages", msg)
		case toDependency:
			section, name = "dependencies", callDependency(name)
		case toPage:
			section = "pages"
		case toOperation:
			if o, ok := d.operations[name]; ok {
				return o.pointer()
			}
			return ""
		case toCommand:
			section = "commands"
		case toClient:
			section = "clients"
		case toToken:
			section = "tokens"
		case toHandshake:
			section = "handshakes"
		case toMiddleware:
			section = "middlewares"
		case toJob:
			section = "jobs"
		case toGateway:
			section = "gateways"
		case toWorkflow:
			section = "workflows"
		case toFlow:
			section = "flows"
		}
		if name == "" || source.Child(source.Child(root, section), name) == nil {
			return ""
		}
		return source.Pointer(section, name)
	}
	names := func(n *yaml.Node, ptr []string, kind, through string) {
		add := func(item *yaml.Node, at []string) {
			if to := target(kind, source.Str(item)); to != "" {
				out = append(out, ModelReference{From: source.Pointer(at...), To: to, Through: through})
			}
		}
		n = source.Deref(n)
		if n.Kind == yaml.SequenceNode {
			for i, item := range source.Items(n) {
				add(item, append(append([]string{}, ptr...), fmt.Sprint(i)))
			}
			return
		}
		add(n, ptr)
	}
	walk(root, nil, func(n *yaml.Node, path []string) {
		for _, p := range source.Pairs(n) {
			key := p.Key.Value
			at := append(append([]string{}, path...), key)
			if key == "$ref" {
				if ref := source.Str(p.Value); strings.HasPrefix(ref, "#/") {
					if _, ok := source.Resolve(root, Tokens(ref[1:])); ok {
						out = append(out, ModelReference{From: source.Pointer(at...), To: ref[1:], Through: key})
					}
				}
				continue
			}
			if kind, ok := anywhere[key]; ok {
				names(p.Value, at, kind, key)
				continue
			}
			for _, place := range keyPlaces {
				if placeMatches(place.path, at) {
					if to := target(place.kind, key); to != "" {
						out = append(out, ModelReference{From: source.Pointer(at...), To: to, Through: placeKeyword(place.path)})
					}
					break
				}
			}
			for _, place := range referencePlaces {
				if placeMatches(place.path, at) {
					names(p.Value, at, place.kind, placeKeyword(place.path))
					break
				}
			}
		}
	})
	return out
}

// ImplementationReferences lists the references of an implementation file
// that resolve in the specification it implements: each design pointer
// under a layout entry's implements and a mapping's design, and each
// dependency of the design a server or an external step names by its key,
// with the keyword that holds it.
func ImplementationReferences(impl, design *yaml.Node) []ModelReference {
	if impl == nil || design == nil {
		return nil
	}
	var out []ModelReference
	walk(impl, nil, func(n *yaml.Node, path []string) {
		for _, p := range source.Pairs(n) {
			key := p.Key.Value
			at := append(append([]string{}, path...), key)
			if key == "dependency" {
				// A server that is a dependency of the design, or an
				// external step that reaches one, names it by its key.
				if name := source.Str(source.Deref(p.Value)); name != "" {
					if _, ok := source.Resolve(design, []string{"dependencies", name}); ok {
						out = append(out, ModelReference{From: source.Pointer(at...), To: source.Pointer("dependencies", name), Through: key})
					}
				}
				continue
			}
			if key != "implements" && !(key == "design" && len(path) >= 1 && path[0] == "mappings") {
				continue
			}
			add := func(item *yaml.Node, ptr []string) {
				ref := source.Str(item)
				if !strings.HasPrefix(ref, "#/") {
					return
				}
				if _, ok := source.Resolve(design, Tokens(ref[1:])); ok {
					out = append(out, ModelReference{From: source.Pointer(ptr...), To: ref[1:], Through: key})
				}
			}
			if v := source.Deref(p.Value); v.Kind == yaml.SequenceNode {
				for i, item := range source.Items(v) {
					add(item, append(append([]string{}, at...), fmt.Sprint(i)))
				}
			} else {
				add(v, at)
			}
		}
	})
	return out
}

// LayoutProblems are the problems found while reading a specification (a
// file not where the tree puts it, a section in the wrong stage, a file
// that did not parse), placed as the problems file places them.
func LayoutProblems(s *spec.Spec) []Diagnostic {
	c := &checker{file: s.RootFile, files: s.Files}
	for _, p := range s.Problems {
		c.addFile(p.File, p.Line, p.Path, Rule(p.Rule), "%s", p.Message)
		c.detail(p.Detail)
	}
	SpecPlacer(s).Place(c.diags)
	Sort(c.diags)
	return c.diags
}

// ReadProblems are the problems found while reading one file on its own,
// such as an implementation file: what of its YAML does not parse. Their
// ids name the file from dir.
func ReadProblems(path, dir string, doc *source.Doc) []Diagnostic {
	c := &checker{file: path}
	for _, p := range doc.Problems {
		c.addLine(p.Line, p.Path, Rule(p.Rule), "%s", p.Message)
		c.detail(p.Detail)
	}
	(&Placer{Dir: dir}).Place(c.diags)
	Sort(c.diags)
	return c.diags
}
