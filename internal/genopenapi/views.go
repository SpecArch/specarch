package genopenapi

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Views: a view is written as a schema of its own, its entity's fields and
// the fields it adds, every added one read-only. A path has the type of the
// field it ends in, and may be null when a relation on the way may have no
// record or the field is not required; a count is a 64-bit integer; rows
// are an array of the relation's target.

// viewAdded is what a view adds to its entity's fields, as plain fields,
// and their names.
func (g *gen) viewAdded(name string) (map[string]any, []any) {
	v := obj(obj(g.spec["views"])[name])
	from := text(v["from"])
	out := map[string]any{}
	var names []any
	props := obj(v["properties"])
	for _, p := range sortedKeys(props) {
		prop := obj(props[p])
		if path := text(prop["path"]); path != "" {
			f, optional := g.pathField(from, path)
			if f == nil {
				continue // the validator reports a path that does not resolve
			}
			if f["writeOnly"] == true {
				g.diags = append(g.diags, g.problem("/views/"+name+"/properties/"+p+"/path", "%s ends in a field that is written and never read, and a view is only read; leave it out of the view", path))
				continue
			}
			names = append(names, p)
			f = g.inlineEnum(f)
			field := map[string]any{}
			for k, val := range f {
				field[k] = val
			}
			field["readOnly"] = true
			delete(field, "default")
			delete(field, "description")
			if text(field["$ref"]) != "" {
				// A $ref takes no type beside it, so null cannot be added
				// to it, and the description is only the one written here.
				if optional {
					g.warnOnce("/views/"+name+"/properties/"+p+"/path",
						fmt.Sprintf("%s may have no value, and the document does not say so: it ends in a $ref, which takes no type beside it to add null to", path))
				}
			} else {
				if optional && !nullable(field) {
					field["type"] = []any{text(field["type"]), "null"}
				}
				field["description"] = "Read through " + path + "."
			}
			if d := text(prop["description"]); d != "" {
				field["description"] = d
			}
			out[p] = field
			continue
		}
		if rel := text(prop["rows"]); rel != "" {
			target := text(obj(obj(obj(obj(g.spec["entities"])[from])["relations"])[rel])["target"])
			if target == "" {
				continue // the validator reports a relation that does not resolve
			}
			names = append(names, p)
			field := map[string]any{"type": "array", "items": map[string]any{"$ref": "#/entities/" + target}, "readOnly": true,
				"description": "The records of " + rel + ", softly deleted records left out."}
			if d := text(prop["description"]); d != "" {
				field["description"] = d
			}
			out[p] = field
			continue
		}
		names = append(names, p)
		field := map[string]any{"type": "integer", "format": "int64", "minimum": json.Number("0"), "readOnly": true,
			"description": "The number of " + text(prop["count"]) + ", softly deleted records left out."}
		if d := text(prop["description"]); d != "" {
			field["description"] = d
		}
		out[p] = field
	}
	return out, names
}

// pathField follows a path from an entity through its relations to the
// field it ends in, or nil, and says whether it may be null: a relation
// on the way may have no record (its via field is not required, or may be
// null), or the field itself is not required.
func (g *gen) pathField(entity, path string) (map[string]any, bool) {
	entities := obj(g.spec["entities"])
	hops := strings.Split(path, ".")
	cur := obj(entities[entity])
	optional := false
	for _, hop := range hops[:len(hops)-1] {
		rel := obj(obj(cur["relations"])[hop])
		via := text(rel["via"])
		required := false
		for _, r := range list(cur["required"]) {
			required = required || text(r) == via
		}
		if !required || nullable(obj(obj(cur["properties"])[via])) {
			optional = true
		}
		cur = obj(entities[text(rel["target"])])
	}
	last := hops[len(hops)-1]
	required := false
	for _, r := range list(cur["required"]) {
		required = required || text(r) == last
	}
	f, _ := obj(cur["properties"])[last].(map[string]any)
	return f, optional || !required
}

// inlineEnum is a field that names an enum by $ref written out as the
// enum's values, so that null can be added to its type.
func (g *gen) inlineEnum(f map[string]any) map[string]any {
	r := text(f["$ref"])
	if !strings.HasPrefix(r, "#/enums/") {
		return f
	}
	e := obj(obj(g.spec["enums"])[strings.TrimPrefix(r, "#/enums/")])
	t := text(e["type"])
	if t == "" {
		t = "string"
	}
	out := map[string]any{"type": t, "enum": e["enum"]}
	for k, v := range f {
		if k != "$ref" {
			out[k] = v
		}
	}
	return out
}

// listSubject is what a list pages through: its name, the fields its
// whitelists name, and the entity whose records it lists.
func (g *gen) listSubject(l map[string]any) (string, map[string]any, map[string]any) {
	if v := text(l["view"]); v != "" {
		from := obj(obj(g.spec["entities"])[text(obj(obj(g.spec["views"])[v])["from"])])
		fields := map[string]any{}
		for k, f := range obj(from["properties"]) {
			fields[k] = f
		}
		added, _ := g.viewAdded(v)
		for k, f := range added {
			fields[k] = f
		}
		return v, fields, from
	}
	e := obj(obj(g.spec["entities"])[text(l["entity"])])
	return text(l["entity"]), obj(e["properties"]), e
}

// subjectName is the entity or view a list names.
func subjectName(l map[string]any) string {
	if v := text(l["view"]); v != "" {
		return v
	}
	return text(l["entity"])
}

// warnOnce reports a warning at a path, once however often the element is
// written: a view is read for its own schema and for each list over it.
func (g *gen) warnOnce(path, message string) {
	for _, d := range g.diags {
		if d.Path == path && d.Message == message {
			return
		}
	}
	g.diags = append(g.diags, Diagnostic{File: g.root, Line: 1, Severity: "warning", Path: path, Rule: "generator", Message: message})
}
