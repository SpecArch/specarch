package generate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// Withheld is what a document for everyone leaves out (ADR-119): the
// pointer of every internal element, outermost only, and how many each
// section lost, in document order.
type Withheld struct {
	Pointers []string
	// Questions are the pointers of the open questions left out, whose
	// every pointer is at or under an element left out.
	Questions []string
	Sections  []string        // the sections that lost an element, in document order
	Counts    map[string]int  // by section
	ids       map[string]bool // the requirements and needs left out, by ID
}

// Covers reports whether a pointer is at or under an element left out.
func (w *Withheld) Covers(ptr string) bool {
	if w == nil {
		return false
	}
	for _, p := range append(append([]string{}, w.Pointers...), w.Questions...) {
		if ptr == p || strings.HasPrefix(ptr, p+"/") {
			return true
		}
	}
	return false
}

// Total is the number of elements left out.
func (w *Withheld) Total() int {
	if w == nil {
		return 0
	}
	return len(w.Pointers)
}

// ForEveryone is the specification as a document for everyone reads it:
// every element whose audience is internal left out, with what is inside
// it; the satisfies, verifies and needs entries that name a requirement or
// need left out; and the open questions whose every pointer is at or under
// an element left out. The nodes that stay are the specification's own,
// so their pointers do not change.
func ForEveryone(root *yaml.Node) (*yaml.Node, *Withheld) {
	w := &Withheld{Counts: map[string]int{}, ids: map[string]bool{}}
	ids := w.ids
	var find func(n *yaml.Node, path []string)
	find = func(n *yaml.Node, path []string) {
		n = source.Deref(n)
		if n == nil {
			return
		}
		switch n.Kind {
		case yaml.MappingNode:
			if spec.IsElement(path) && str(n, "audience") == "internal" {
				w.Pointers = append(w.Pointers, source.Pointer(path...))
				if w.Counts[path[0]] == 0 {
					w.Sections = append(w.Sections, path[0])
				}
				w.Counts[path[0]]++
				if len(path) == 2 && (path[0] == "requirements" || path[0] == "needs") {
					ids[path[1]] = true
				}
				return
			}
			for _, p := range source.Pairs(n) {
				find(p.Value, append(append([]string{}, path...), p.Key.Value))
			}
		case yaml.SequenceNode:
			for i, item := range n.Content {
				find(item, append(append([]string{}, path...), fmt.Sprint(i)))
			}
		}
	}
	find(root, nil)
	if w.Total() == 0 {
		return root, w
	}
	var keep func(n *yaml.Node, path []string) *yaml.Node
	keep = func(n *yaml.Node, path []string) *yaml.Node {
		d := source.Deref(n)
		if d == nil {
			return n
		}
		switch d.Kind {
		case yaml.MappingNode:
			out := *d
			out.Content = nil
			for _, p := range source.Pairs(d) {
				at := append(append([]string{}, path...), p.Key.Value)
				ptr := source.Pointer(at...)
				if contains(w.Pointers, ptr) {
					continue
				}
				if len(at) == 2 && at[0] == spec.QuestionsSection && w.blocksOnlyWithheld(p.Value) {
					w.Questions = append(w.Questions, ptr)
					continue
				}
				out.Content = append(out.Content, p.Key, keep(p.Value, at))
			}
			return &out
		case yaml.SequenceNode:
			last := ""
			if len(path) > 0 {
				last = path[len(path)-1]
			}
			out := *d
			out.Content = nil
			for i, item := range d.Content {
				if (last == "satisfies" || last == "verifies" || last == "needs") && ids[source.Str(item)] {
					continue
				}
				out.Content = append(out.Content, keep(item, append(append([]string{}, path...), fmt.Sprint(i))))
			}
			return &out
		}
		return n
	}
	return keep(root, nil), w
}

// ForEveryoneImplementation is an implementation file as a document for
// everyone reads it: the mappings of an element left out are left out,
// and so are the satisfies entries that name a requirement left out.
func (w *Withheld) ForEveryoneImplementation(root *yaml.Node) *yaml.Node {
	if w.Total() == 0 {
		return root
	}
	var keep func(n *yaml.Node, path []string) *yaml.Node
	keep = func(n *yaml.Node, path []string) *yaml.Node {
		d := source.Deref(n)
		if d == nil {
			return n
		}
		switch d.Kind {
		case yaml.MappingNode:
			out := *d
			out.Content = nil
			for _, p := range source.Pairs(d) {
				if len(path) == 1 && path[0] == "mappings" && w.Covers(strings.TrimPrefix(p.Key.Value, "#")) {
					continue
				}
				out.Content = append(out.Content, p.Key, keep(p.Value, append(append([]string{}, path...), p.Key.Value)))
			}
			return &out
		case yaml.SequenceNode:
			out := *d
			out.Content = nil
			for i, item := range d.Content {
				if len(path) > 0 && path[len(path)-1] == "satisfies" && w.ids[source.Str(item)] {
					continue
				}
				out.Content = append(out.Content, keep(item, append(append([]string{}, path...), fmt.Sprint(i))))
			}
			return &out
		}
		return n
	}
	return keep(root, nil)
}

// blocksOnlyWithheld reports whether every pointer a question blocks is at
// or under an element left out, and it blocks one at least.
func (w *Withheld) blocksOnlyWithheld(q *yaml.Node) bool {
	found := false
	for _, item := range items(q, "blocks") {
		b, ok := spec.ParseBlock(source.Str(item))
		if !ok || !b.IsPointer() {
			return false
		}
		if !w.Covers(source.Pointer(b.Tokens...)) {
			return false
		}
		found = true
	}
	return found
}

// audienceNotice is the sentence a document for everyone starts with: how
// many internal elements it left out, in each section.
func (w *Withheld) audienceNotice() string {
	if w == nil {
		return ""
	}
	if w.Total() == 0 {
		return "**Audience:** everyone; the specification has no internal element."
	}
	var parts []string
	for _, s := range w.Sections {
		parts = append(parts, fmt.Sprintf("%d in %s", w.Counts[s], s))
	}
	return fmt.Sprintf("**Audience:** everyone; %s left out (%s), with the problems and open questions that concern only them.", countText(w.Total(), "internal element is", "internal elements are"), strings.Join(parts, ", "))
}
