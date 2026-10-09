package genopenapi

import (
	"sort"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/mark"
)

// marksKey is the extension that carries the problems marked at an object
// of the document: JSON keeps no comment, and OpenAPI allows x- keys on
// its objects. x-specarch-problems is the problem catalogue already, so
// the marks have a key of their own.
const marksKey = "x-specarch-marks"

// sections are the sections of the specification the document shows as a
// whole: a question that blocks one is marked on the document.
var sections = []string{"/info", "/paths", "/entities", "/enums", "/views", "/schemas", "/errors"}

// record keeps the object of the document an entry of the specification
// is written as, so its marks go there.
func (g *gen) record(ptr string, n *yaml.Node) *yaml.Node {
	if g.at == nil {
		g.at = map[string]*yaml.Node{}
	}
	g.at[ptr] = n
	return n
}

// markDoc writes each problem of the request at the object of the
// document that shows its entry, the document itself for a section or an
// error no object holds, and the draft notice in the info object.
func (g *gen) markDoc(doc *yaml.Node, r *Request) {
	if r.Draft != "" {
		if info := child(doc, "info"); info != nil {
			add(info, "x-specarch-draft", str(r.Draft))
		}
	}
	reachable := map[*yaml.Node]bool{}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n == nil || reachable[n] {
			return
		}
		reachable[n] = true
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(doc)
	var elements []string
	for ptr, n := range g.at {
		if reachable[n] && n.Kind == yaml.MappingNode {
			elements = append(elements, ptr)
		}
	}
	sort.Strings(elements)
	placed := mark.Place(r.Problems, elements, sections)
	for _, ptr := range append([]string{mark.File}, elements...) {
		ps := placed[ptr]
		if len(ps) == 0 {
			continue
		}
		at := doc
		if ptr != mark.File {
			at = g.at[ptr]
		}
		list := sequence()
		for _, p := range ps {
			m := mapping()
			add(m, "id", str(p.ID))
			add(m, "severity", str(p.Severity))
			add(m, "rule", str(p.Rule))
			add(m, "message", str(p.Message))
			list.Content = append(list.Content, m)
		}
		add(at, marksKey, list)
	}
}

// draftLine is the draft notice as a comment under the header, or "".
func draftLine(r *Request) string {
	if r.Draft == "" {
		return ""
	}
	return "# " + r.Draft + "\n"
}
