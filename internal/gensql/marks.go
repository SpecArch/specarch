package gensql

import (
	"strings"

	"github.com/SpecArch/specarch/internal/mark"
	"github.com/SpecArch/specarch/internal/ownership"
)

// sections are the sections of the specification the migrations show as a
// whole: a question that blocks one is marked under each file's header.
var sections = []string{"/entities", "/enums", "/views"}

// place places the request's problems at the tables, columns and views the
// migrations write (internal/mark), from the entities as the design writes
// them, before their value objects are written out.
func (g *gen) place(r *Request) {
	var elements []string
	for name, e := range obj0(g.spec["entities"]) {
		if g.owned.Covers(ownership.Entity("entities", name)) {
			continue
		}
		at := "/entities/" + mark.Escape(name)
		elements = append(elements, at)
		for f := range obj0(obj0(e)["properties"]) {
			elements = append(elements, at+"/properties/"+mark.Escape(f))
		}
	}
	for name := range obj0(g.spec["views"]) {
		if !g.owned.Covers(ownership.Entity("views", name)) {
			elements = append(elements, "/views/"+mark.Escape(name))
		}
	}
	g.problems = r.Problems
	g.placed = mark.Place(r.Problems, elements, sections)
}

// marks are the comment lines of the problems placed at an element.
func (g *gen) marks(ptr, indent string) string {
	var b strings.Builder
	for _, p := range g.placed[ptr] {
		b.WriteString(indent + "-- " + mark.Line(p) + "\n")
	}
	return b.String()
}

// fieldOf is the property of an entity, as the design writes it, that a
// field of its table comes from: the field itself, or the value object
// whose parts are written out in columns named after it.
func (g *gen) fieldOf(entity, field string) string {
	props := obj0(obj0(g.originalEntities()[entity])["properties"])
	if _, ok := props[field]; ok {
		return field
	}
	best := ""
	for p := range props {
		if strings.HasPrefix(snake(field), snake(p)+"_") && len(p) > len(best) {
			best = p
		}
	}
	return best
}

// originalEntities are the entities before their value objects are
// written out.
func (g *gen) originalEntities() map[string]any {
	if g.original != nil {
		return g.original
	}
	return obj0(g.spec["entities"])
}

// entityMarks are the marks of an entity and of its fields, in the order
// of the problems, each once: what a later migration writes above the
// first statement that changes the entity's table.
func (g *gen) entityMarks(entity string) string {
	group := map[string]bool{}
	for _, ptr := range append([]string{"/entities/" + mark.Escape(entity)}, g.fieldPointers(entity)...) {
		for _, p := range g.placed[ptr] {
			group[p.ID] = true
		}
	}
	var b strings.Builder
	for _, p := range g.problems {
		if group[p.ID] {
			b.WriteString("-- " + mark.Line(p) + "\n")
			delete(group, p.ID)
		}
	}
	return b.String()
}

func (g *gen) fieldPointers(entity string) []string {
	var out []string
	for _, f := range sortedKeys(obj0(obj0(g.originalEntities()[entity])["properties"])) {
		out = append(out, "/entities/"+mark.Escape(entity)+"/properties/"+mark.Escape(f))
	}
	return out
}

// draftMigration says whether a migration was written by a draft.
func draftMigration(content string) bool {
	for _, l := range strings.Split(content, "\n") {
		if strings.HasPrefix(l, "-- Draft: ") {
			return true
		}
		if l != "" && !strings.HasPrefix(l, "--") {
			return false
		}
	}
	return false
}
