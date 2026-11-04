package generate

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// openChange is a change request not yet carried out, as the documents
// show it at the elements it reaches (docs/acceptance.md, section 4):
// proposed, analysed or approved.
type openChange struct {
	node     *yaml.Node
	id       string
	pointers []string // what its affects and seenAt name, as pointers into the merged specification
}

// openChangesOf lists the open change requests of the records, in ID
// order.
func openChangesOf(state *State) []openChange {
	if state == nil {
		return nil
	}
	var out []openChange
	for _, r := range recordsOf(state, "change") {
		if !isOpen(r) {
			continue
		}
		c := openChange{node: r, id: str(r, "id")}
		affects := get(r, "affects")
		for _, list := range []string{"adds", "changes", "removes"} {
			for _, ref := range strs(affects, list) {
				c.pointers = append(c.pointers, pointerOfRef(ref))
			}
		}
		for _, ref := range strs(r, "seenAt") {
			c.pointers = append(c.pointers, pointerOfRef(ref))
		}
		c.pointers = uniqueSorted(c.pointers)
		out = append(out, c)
	}
	return out
}

// pointerOfRef reads a reference of a record as a pointer into the merged
// specification: a #/ pointer without its #, or a requirement ID under
// /requirements.
func pointerOfRef(ref string) string {
	if strings.HasPrefix(ref, "#/") {
		return strings.TrimPrefix(ref, "#")
	}
	return source.Pointer("requirements", ref)
}

// at reports whether the change names the element at ptr, or something
// under it, and returns what it names there.
func (c openChange) at(ptr string) (named []string) {
	for _, p := range c.pointers {
		if p == ptr || strings.HasPrefix(p, ptr+"/") {
			named = append(named, p)
		}
	}
	return named
}

// openChanges writes, at an element, each open change request that names
// it or something under it, beside the open questions.
func (d *doc) openChanges(label string, n *yaml.Node) {
	ptr, ok := d.pointerOf[source.Deref(n)]
	if !ok || ptr == "" {
		return
	}
	on := ""
	if label != "" {
		on = tr(" on ") + label
	}
	for _, c := range d.changes {
		named := c.at(ptr)
		if len(named) == 0 {
			continue
		}
		where := ""
		if len(named) != 1 || named[0] != ptr {
			var under []string
			for _, p := range named {
				if p != ptr {
					under = append(under, "`#"+p+"`")
				}
			}
			if len(under) > 0 {
				where = tr(" at ") + strings.Join(under, ", ")
			}
		}
		d.para(fmt.Sprintf(tr("**Open change request %s%s%s:** %s %s"), c.id, on, where, sentence(str(c.node, "title")), raisedLine(c.node)))
	}
}

// raisedLine says where and by whom a change was raised, where it stands
// and its cause: "Raised in acceptance by librarian on 2026-10-21;
// approved; cause omitted."
func raisedLine(c *yaml.Node) string {
	line := tr("Raised in ") + str(c, "phase")
	if by := str(c, "raisedBy"); by != "" {
		line += tr(" by ") + by
	}
	if on := str(c, "raised"); on != "" {
		line += tr(" on ") + on
	}
	line += "; " + str(c, "status")
	if cause := str(c, "cause"); cause != "" {
		line += tr("; cause ") + cause
	}
	return line + "."
}

// reach writes what an open change reaches in the specification as it is
// now: the elements its pointers name, the tests, the pages, operations,
// commands and channels that read an entity or enum among them, the code
// targets whose reads take a section it touches, the documents that show
// one, and the approval it voids (docs/acceptance.md, section 4).
func (d *doc) reach(root *yaml.Node, impls []Implementation, state *State, c openChange) {
	elements := map[string]bool{}
	sections := map[string]bool{}
	var unresolved []string
	for _, p := range c.pointers {
		found := false
		for _, e := range elementPointers(root) {
			if e == p || strings.HasPrefix(p, e+"/") || strings.HasPrefix(e, p+"/") {
				elements[e] = true
				found = true
			}
		}
		if !found {
			unresolved = append(unresolved, p)
		}
		sections[strings.SplitN(strings.TrimPrefix(p, "/"), "/", 2)[0]] = true
	}
	var lines []string
	if len(elements) > 0 {
		lines = append(lines, tr("elements ")+refList(withHash(sortedSet(elements))))
	}
	if len(unresolved) > 0 {
		lines = append(lines, tr("nothing yet at ")+refList(withHash(unresolved))+tr(", which the change adds"))
	}
	var tests []string
	for _, p := range pairs(root, "tests") {
		verifies := strs(p.Value, "verifies")
		hit := false
		for _, v := range verifies {
			hit = hit || elements[source.Pointer("requirements", v)]
		}
		if e := changeTestSubject(root, p.Value); e != "" && elements[e] {
			hit = true
		}
		if hit {
			tests = append(tests, p.Key.Value)
		}
	}
	if len(tests) > 0 {
		lines = append(lines, tr("the ")+oneOrMany(len(tests), tr("test"), tr("tests"))+" "+strings.Join(tests, ", "))
	}
	var readers []string
	for _, p := range pairs(root, "pages") {
		reads := map[string]bool{}
		if e := str(p.Value, "entity"); e != "" {
			reads[source.Pointer("entities", e)] = true
		}
		walkTypeRefs(p.Value, func(ref string) { reads[strings.TrimPrefix(ref, "#")] = true })
		if readsAny(reads, elements) {
			readers = append(readers, tr("the page ")+p.Key.Value)
		}
	}
	for _, path := range pairs(root, "paths") {
		for _, op := range source.Pairs(path.Value) {
			if !isMethodKey(op.Key.Value) {
				continue
			}
			reads := map[string]bool{}
			walkTypeRefs(op.Value, func(ref string) { reads[strings.TrimPrefix(ref, "#")] = true })
			if readsAny(reads, elements) {
				readers = append(readers, tr("the operation ")+str(op.Value, "operationId"))
			}
		}
	}
	for _, section := range []struct{ key, word string }{{"commands", "command"}, {"channels", "channel"}} {
		for _, p := range pairs(root, section.key) {
			reads := map[string]bool{}
			walkTypeRefs(p.Value, func(ref string) { reads[strings.TrimPrefix(ref, "#")] = true })
			if readsAny(reads, elements) {
				readers = append(readers, tr("the ")+section.word+" "+p.Key.Value)
			}
		}
	}
	if len(readers) > 0 {
		lines = append(lines, strings.Join(readers, ", ")+tr(", reading an entity or an enum it touches"))
	}
	var targets, every []string
	for _, i := range impls {
		for _, t := range source.Pairs(get(i.Node, "targets")) {
			if IsDocumentTarget(t.Key.Value) {
				continue
			}
			reads := strs(t.Value, "reads")
			name := t.Key.Value + tr(" of ") + i.Rel
			if len(reads) == 0 {
				every = append(every, name)
				continue
			}
			for _, s := range reads {
				if sections[s] {
					targets = append(targets, name)
					break
				}
			}
		}
	}
	if len(targets) > 0 {
		lines = append(lines, tr("the code ")+oneOrMany(len(targets), tr("target"), tr("targets"))+" "+strings.Join(targets, ", ")+tr(", whose reads take a section it touches"))
	}
	if len(every) > 0 {
		lines = append(lines, tr("the code ")+oneOrMany(len(every), tr("target"), tr("targets"))+" "+strings.Join(every, ", ")+tr(", reading every section"))
	}
	var docs []string
	for _, t := range DocumentTargets {
		if recordDocuments[t] || RefinementDocuments[t] {
			continue
		}
		for _, s := range documentReads[t] {
			if s == "*" || sections[s] {
				docs = append(docs, t)
				break
			}
		}
	}
	if len(docs) > 0 {
		lines = append(lines, tr("the ")+oneOrMany(len(docs), tr("document"), tr("documents"))+" "+strings.Join(docs, ", "))
	}
	if state != nil {
		if state.Approved {
			lines = append(lines, tr("the approval it voids: the specification is ")+state.Approval+tr(", and any change to a file voids it"))
		} else {
			lines = append(lines, tr("no approval to void: ")+state.Approval)
		}
	}
	d.para(tr("Reaches, as the specification is now:"))
	for _, l := range lines {
		d.line("- %s", sentence(l))
	}
	d.blank()
}

// elementPointers lists the pointer of every element of the merged
// specification: each named entry of a section, and each operation under
// paths.
func elementPointers(root *yaml.Node) []string {
	var out []string
	for _, section := range source.Pairs(root) {
		if section.Key.Value == "paths" {
			for _, path := range source.Pairs(section.Value) {
				for _, op := range source.Pairs(path.Value) {
					if isMethodKey(op.Key.Value) {
						out = append(out, source.Pointer("paths", path.Key.Value, op.Key.Value))
					}
				}
			}
			continue
		}
		if source.Deref(section.Value) != nil && source.Deref(section.Value).Kind == yaml.MappingNode {
			for _, p := range source.Pairs(section.Value) {
				out = append(out, source.Pointer(section.Key.Value, p.Key.Value))
			}
		}
	}
	return out
}

func isMethodKey(k string) bool {
	switch k {
	case "get", "put", "post", "delete", "patch", "head", "options":
		return true
	}
	return false
}

// testSubject is the pointer of the element a test is about: the
// operation's path and method, or the command, page or entity it names.
func changeTestSubject(root *yaml.Node, t *yaml.Node) string {
	if v := str(t, "operation"); v != "" {
		for _, path := range pairs(root, "paths") {
			for _, op := range source.Pairs(path.Value) {
				if isMethodKey(op.Key.Value) && str(op.Value, "operationId") == v {
					return source.Pointer("paths", path.Key.Value, op.Key.Value)
				}
			}
		}
		return ""
	}
	for _, k := range []string{"command", "page", "entity"} {
		if v := str(t, k); v != "" {
			return source.Pointer(k+"s", v)
		}
	}
	return ""
}

// walkTypeRefs calls fn with every $ref to an entity or an enum under n.
func walkTypeRefs(n *yaml.Node, fn func(string)) {
	n = source.Deref(n)
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.MappingNode:
		for _, p := range source.Pairs(n) {
			if p.Key.Value == "$ref" && source.IsScalar(p.Value) {
				if v := p.Value.Value; strings.HasPrefix(v, "#/entities/") || strings.HasPrefix(v, "#/enums/") {
					fn(v)
				}
				continue
			}
			walkTypeRefs(p.Value, fn)
		}
	case yaml.SequenceNode:
		for _, item := range source.Items(n) {
			walkTypeRefs(item, fn)
		}
	}
}

func readsAny(reads, elements map[string]bool) bool {
	for r := range reads {
		if elements[r] {
			return true
		}
	}
	return false
}

func sortedSet(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func withHash(ptrs []string) []string {
	out := make([]string, len(ptrs))
	for i, p := range ptrs {
		out[i] = "#" + p
	}
	return out
}
