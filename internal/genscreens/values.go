package genscreens

import (
	"strconv"
	"strings"

	"github.com/SpecArch/specarch/internal/wirename"
)

// Values on forms and views (ADR-079). An entity's field may hold a schema,
// or a list of them (ADR-063). A form draws one as a nested section of its
// parts and a list as a repeating group of items; a view shows the parts.
// The parts come in the order of their names, as every generator writes a
// schema's parts, since the specification reaches a plug-in as maps.

// valueOf names the schema a property holds, and whether it holds a list
// of them: '#/schemas/Name' itself, or as the items of an array.
func ValueOf(prop map[string]any) (string, bool) {
	if ref, ok := strings.CutPrefix(text(prop["$ref"]), "#/schemas/"); ok {
		return ref, false
	}
	if ref, ok := strings.CutPrefix(text(obj0(prop["items"])["$ref"]), "#/schemas/"); ok {
		return ref, true
	}
	return "", false
}

// valueField writes the data of a field holding a Value, or a list of
// them: the value-field or value-list-field part with the field of each
// part of the schema. What a value cannot take, a hook or a second entry,
// and a value where only a field is drawn, as in a child row, are errors
// at the field; a picker on one the validator refuses, since no relation
// holds a value.
func (g *Gen) valueField(in formFieldInput, schemaName string, many bool, label string) (string, []Member, bool) {
	shown := orText(in.path, in.name)
	part := "value-field"
	if many {
		part = "value-list-field"
	}
	ok := true
	if !in.inSections {
		g.Problem("error", in.at, "%s shows %s among the fields of a row, which holds a value of %s, and this version of %s draws a value in a form's sections only", in.pageName, shown, schemaName, g.Profile.Name)
		return "", nil, false
	}
	if in.hooked {
		g.Problem("error", in.at, "the ui target's settings.hooks give %s a hook for %s, which holds a value of %s, and a hook gives a field one text; take it out of settings.hooks", in.pageName, shown, schemaName)
		ok = false
	}
	if in.twice {
		g.Problem("error", in.at, "%s asks for %s twice, which holds a value of %s; a value is entered once", in.pageName, shown, schemaName)
		ok = false
	}
	if in.within[schemaName] {
		g.Problem("error", in.at, "%s shows %s, which holds %s inside a value of %s itself, and a form cannot draw a value without end; keep the inner one out of the form", in.pageName, shown, schemaName, schemaName)
		return "", nil, false
	}
	schema, found := obj(obj0(g.Spec["schemas"])[schemaName])
	if !found {
		g.Problem("error", in.at, "%s shows %s, which holds %s, and the specification has no schema of that name", in.pageName, shown, schemaName)
		return "", nil, false
	}
	if !ok {
		return "", nil, false
	}
	within := map[string]bool{schemaName: true}
	for s := range in.within {
		within[s] = true
	}
	labelAt := g.labelKey(in)
	partProps := obj0(schema["properties"])
	required := map[string]bool{}
	for _, r := range list(schema["required"]) {
		required[text(r)] = true
	}
	var parts []Value
	for _, p := range sortedKeys(partProps) {
		prop := obj0(partProps[p])
		field, done := g.formField(formFieldInput{pageName: in.pageName, at: in.at, name: p, prop: prop,
			required: required[p], readOnly: prop["readOnly"] == true, labelAt: labelAt + "." + p,
			path: shown + "." + p, inSections: true, within: within})
		if !done {
			ok = false
			continue
		}
		parts = append(parts, field)
	}
	if !ok {
		return "", nil, false
	}
	data := []Member{
		{"type", Str(g.Name(part, "type"))},
		{g.Name(part, "name"), Str(wirename.Of(g.WireNames, in.name))},
		{g.Name(part, "label"), Str(g.Say(labelAt, label))},
		{g.Name(part, "required"), Raw(strconv.FormatBool(in.required))},
		{g.Name(part, "parts"), Array(parts)},
	}
	if many {
		if m := text(in.prop["minItems"]); m != "" && m != "0" {
			data = append(data, Member{g.Name(part, "minimum"), Raw(m)})
		}
		if m := text(in.prop["maxItems"]); m != "" {
			data = append(data, Member{g.Name(part, "maximum"), Raw(m)})
		}
	}
	return part, data, true
}

// viewParts are the parts a view shows of a field holding a Value, each a
// field of label and Value, and whether the field holds a list of them;
// nil when it holds none. A schema met again inside itself is an error.
func (g *Gen) viewParts(pageName, at, path, labelAt string, prop map[string]any, within map[string]bool) ([]Value, bool, bool) {
	schemaName, many := ValueOf(prop)
	if schemaName == "" {
		return nil, false, true
	}
	if within[schemaName] {
		g.Problem("error", at, "%s shows %s, which holds %s inside a value of %s itself, and a view cannot show a value without end; leave the inner one out", pageName, path, schemaName, schemaName)
		return nil, false, false
	}
	inner := map[string]bool{schemaName: true}
	for s := range within {
		inner[s] = true
	}
	schema := obj0(obj0(g.Spec["schemas"])[schemaName])
	partProps := obj0(schema["properties"])
	parts := []Value{}
	ok := true
	for _, p := range sortedKeys(partProps) {
		partProp := obj0(partProps[p])
		label := orText(text(partProp["title"]), p)
		entry := []Member{{"name", Str(wirename.Of(g.WireNames, p))}, {"label", Str(g.Say(labelAt+"."+p, label))}}
		nested, list, done := g.viewParts(pageName, at, path+"."+p, labelAt+"."+p, partProp, inner)
		if !done {
			ok = false
			continue
		}
		if nested != nil {
			entry = append(entry, Member{"parts", Array(nested)})
			if list {
				entry = append(entry, Member{"list", Raw("true")})
			}
		}
		parts = append(parts, Object(entry))
	}
	return parts, many, ok
}
