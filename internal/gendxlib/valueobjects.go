package gendxlib

import (
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/genopenapi"
	"github.com/SpecArch/specarch/internal/gensql"
)

// Value objects (ADR-063, ADR-078). A parameter that holds a schema is read
// into a Go struct named after the schema, and a list of them into a slice
// of it; each part is a pointer, nil when it is not given, so a check can
// name the required part a value lacks. A create writes a value kept in
// columns into the column of each part, named as specarch-gen-sql names
// it, and a value kept as JSON as its text. A create and a read answer the
// stored row folded into the interface's shape: the columns of a value
// gathered into it, and a value kept as JSON read from its text.

// readValue is the getter of a parameter that holds a value object, which
// dxlib reads as JSON and the generated readValue decodes.
const readValue = "readValue"

// valueObjectOf names the schema a field holds, and whether it holds a list
// of them.
func valueObjectOf(s map[string]any) (string, bool) {
	if ref, ok := strings.CutPrefix(text(s["$ref"]), "#/schemas/"); ok {
		return ref, false
	}
	if ref, ok := strings.CutPrefix(text(obj(s["items"])["$ref"]), "#/schemas/"); ok {
		return ref, true
	}
	return "", false
}

// useValue marks a schema, and every schema its parts hold, to be written as
// a Go type.
func (g *gen) useValue(name string) {
	if g.values[name] || g.schemas[name] == nil {
		return
	}
	g.values[name] = true
	for _, part := range obj(obj(g.schemas[name])["properties"]) {
		if inner, _ := valueObjectOf(obj(part)); inner != "" {
			g.useValue(inner)
		}
	}
}

// useColumns marks a schema, and every schema one of its parts holds, to
// write its parts into the columns of a row.
func (g *gen) useColumns(name string) {
	if g.columns[name] || g.schemas[name] == nil {
		return
	}
	g.columns[name] = true
	for _, part := range obj(obj(g.schemas[name])["properties"]) {
		if inner, many := valueObjectOf(obj(part)); inner != "" && !many {
			g.useColumns(inner)
		}
	}
}

func (g *gen) helper(name string) { g.helpers[name] = true }

// holdsValues reports whether an entity has a field holding a schema.
func (g *gen) holdsValues(entity string) bool {
	for _, f := range obj(obj(g.entities[entity])["properties"]) {
		if name, _ := valueObjectOf(obj(f)); name != "" && g.schemas[name] != nil {
			return true
		}
	}
	return false
}

// missingCheck writes the check that a value a parameter holds has every
// required part, refused as dxlib refuses a required parameter left out.
func (g *gen) missingCheck(p param) {
	name, many := valueObjectOf(p.schema)
	if name == "" || g.schemas[name] == nil {
		return
	}
	g.imports["net/http"] = true
	refuse := "\t\treturn aepr.WriteResponseAndNewErrorf(http.StatusUnprocessableEntity, \"\", \"%%s\", \"REQUEST_FIELD_VALUE_IS_NOT_EXIST:\"+m)"
	if many {
		g.imports["strconv"] = true
		g.line("\tfor i := range r.%s {", p.field)
		g.line("\t\tif m := r.%s[i].missing(%q + strconv.Itoa(i) + \"]\"); m != \"\" {", p.field, p.wire+"[")
		g.line("\t" + refuse)
		g.line("\t\t}")
		g.line("\t}")
		return
	}
	g.line("\tif m := r.%s.missing(%q); m != \"\" {", p.field, p.wire)
	g.line(refuse)
	g.line("\t}")
}

// storeValue writes into a create's data a parameter that is a field of
// the entity holding a schema, and reports whether it was one.
func (g *gen) storeValue(entity string, p param) bool {
	field := obj(obj(obj(g.entities[entity])["properties"])[p.name])
	name, many := valueObjectOf(field)
	if name == "" || g.schemas[name] == nil {
		return false
	}
	if !many && text(field["storage"]) != "json" {
		g.useColumns(name)
		g.line("\tr.%s.columns(%q, data)", p.field, p.wire)
		return true
	}
	g.helper("jsonText")
	g.line("\tif r.Has%s {", p.field)
	g.line("\t\tdata[%q] = jsonText(r.%s)", p.wire, p.field)
	g.line("\t}")
	return true
}

// foldRow writes the call that gives a stored row of the entity the
// interface's shape, when the entity holds a value.
func (g *gen) foldRow(entity string) {
	if !g.holdsValues(entity) {
		return
	}
	g.folds[entity] = true
	g.line("\tif err := fold%s(row); err != nil {", entity)
	g.line("\t\treturn err")
	g.line("\t}")
}

// valueTypes writes the Go type of every value a request carries, the fold
// of every entity a handler answers, and the helpers they use.
func (g *gen) valueTypes() {
	for _, name := range keys(g.values) {
		g.valueType(name)
	}
	for _, entity := range keys(g.folds) {
		g.fold(entity)
	}
	for _, h := range keys(g.helpers) {
		switch h {
		case "readValue":
			g.imports["encoding/json"] = true
			g.imports["net/http"] = true
			g.line("// readValue reads a parameter that holds a value object, or a list of")
			g.line("// them, into its Go type; false when it is not given or is null.")
			g.line("func readValue[T any](aepr *api.DXAPIEndPointRequest, name string) (bool, T, error) {")
			g.line("\tvar v T")
			g.line("\thas, raw, err := aepr.GetParameterValueAsAny(name)")
			g.line("\tif err != nil || !has {")
			g.line("\t\treturn false, v, err")
			g.line("\t}")
			g.line("\ttext, err := json.Marshal(raw)")
			g.line("\tif err == nil {")
			g.line("\t\terr = json.Unmarshal(text, &v)")
			g.line("\t}")
			g.line("\tif err != nil {")
			g.line("\t\treturn false, v, aepr.WriteResponseAndNewErrorf(http.StatusUnprocessableEntity, \"\", \"INVALID_PARAMETER:%%s:%%s\", name, err.Error())")
			g.line("\t}")
			g.line("\treturn true, v, nil")
			g.line("}")
		case "jsonText":
			g.imports["encoding/json"] = true
			g.line("// jsonText is a value kept as JSON, as the text its column takes. The")
			g.line("// value was read from JSON, so it writes back without an error.")
			g.line("func jsonText(v any) string {")
			g.line("\ttext, _ := json.Marshal(v)")
			g.line("\treturn string(text)")
			g.line("}")
		case "fold":
			g.line("// fold gathers the columns of a value kept in the columns of its parts")
			g.line("// into the value, each part by its path, and takes them out of the row.")
			g.line("// A value, or a value inside it, whose columns are all null is left out.")
			g.line("func fold(row utils.JSON, field string, parts ...[]string) {")
			g.line("\tvalue := utils.JSON{}")
			g.line("\tfor _, part := range parts {")
			g.line("\t\tcolumn, path := part[0], part[1:]")
			g.line("\t\tv := row[column]")
			g.line("\t\tdelete(row, column)")
			g.line("\t\tif v == nil {")
			g.line("\t\t\tcontinue")
			g.line("\t\t}")
			g.line("\t\tm := value")
			g.line("\t\tfor _, p := range path[:len(path)-1] {")
			g.line("\t\t\tnext, _ := m[p].(utils.JSON)")
			g.line("\t\t\tif next == nil {")
			g.line("\t\t\t\tnext = utils.JSON{}")
			g.line("\t\t\t\tm[p] = next")
			g.line("\t\t\t}")
			g.line("\t\t\tm = next")
			g.line("\t\t}")
			g.line("\t\tm[path[len(path)-1]] = v")
			g.line("\t}")
			g.line("\tif len(value) == 0 {")
			g.line("\t\trow[field] = nil")
			g.line("\t\treturn")
			g.line("\t}")
			g.line("\trow[field] = value")
			g.line("}")
		case "unjson":
			g.imports["encoding/json"] = true
			g.line("// unjson reads a value kept as JSON from the text its column holds.")
			g.line("func unjson(row utils.JSON, column string) error {")
			g.line("\tvar text []byte")
			g.line("\tswitch v := row[column].(type) {")
			g.line("\tcase string:")
			g.line("\t\ttext = []byte(v)")
			g.line("\tcase []byte:")
			g.line("\t\ttext = v")
			g.line("\tdefault:")
			g.line("\t\treturn nil")
			g.line("\t}")
			g.line("\tvar value any")
			g.line("\tif err := json.Unmarshal(text, &value); err != nil {")
			g.line("\t\treturn err")
			g.line("\t}")
			g.line("\trow[column] = value")
			g.line("\treturn nil")
			g.line("}")
		}
		g.line("")
	}
}

// partType is the Go type of a part of a value: a pointer, nil when the
// part is not given, except for a list or an object, which are nil
// themselves.
func (g *gen) partType(s map[string]any) string {
	typ, _ := g.goType(s)
	if strings.HasPrefix(typ, "[]") || strings.HasPrefix(typ, "*") || typ == "utils.JSON" {
		return typ
	}
	return "*" + typ
}

// valueType writes a schema's Go type, with the method that names a
// required part the value lacks and, for a schema kept in columns, the
// method that writes its parts into a row.
func (g *gen) valueType(name string) {
	schema := obj(g.schemas[name])
	props := obj(schema["properties"])
	byWire := map[string]string{}
	for p := range props {
		byWire[genopenapi.Wire(p)] = p
	}
	wires := keys(byWire)
	typ := pascal(name)
	g.line("// %s is the schema %s as a request carries it; a part not given is nil.", typ, name)
	g.line("type %s struct {", typ)
	for _, w := range wires {
		p := byWire[w]
		g.line("\t%s %s `json:%q`", pascal(p), g.partType(obj(props[p])), w)
	}
	g.line("}")
	g.line("")

	g.line("// missing names the first required part the value lacks, by its path")
	g.line("// from at, or \"\" when it has them all.")
	g.line("func (v *%s) missing(at string) string {", typ)
	g.line("\tif v == nil {")
	g.line("\t\treturn \"\"")
	g.line("\t}")
	for _, r := range list(schema["required"]) {
		p := text(r)
		part := obj(props[p])
		if part == nil || nullableType(part) {
			continue // a part that may be null is given when it is null, which a nil cannot tell
		}
		g.line("\tif v.%s == nil {", pascal(p))
		g.line("\t\treturn at + %q", "."+genopenapi.Wire(p))
		g.line("\t}")
	}
	for _, w := range wires {
		p := byWire[w]
		inner, many := valueObjectOf(obj(props[p]))
		if inner == "" || g.schemas[inner] == nil {
			continue
		}
		if many {
			g.imports["strconv"] = true
			g.line("\tfor i := range v.%s {", pascal(p))
			g.line("\t\tif m := v.%s[i].missing(at + %q + strconv.Itoa(i) + \"]\"); m != \"\" {", pascal(p), "."+w+"[")
			g.line("\t\t\treturn m")
			g.line("\t\t}")
			g.line("\t}")
			continue
		}
		g.line("\tif m := v.%s.missing(at + %q); m != \"\" {", pascal(p), "."+w)
		g.line("\t\treturn m")
		g.line("\t}")
	}
	g.line("\treturn \"\"")
	g.line("}")
	g.line("")

	if !g.columns[name] {
		return
	}
	g.imports["github.com/donnyhardyanto/dxlib/utils"] = true
	g.line("// columns writes the value into data, a column per part named after the")
	g.line("// path to it as specarch-gen-sql names it; a part not given is left out.")
	g.line("func (v *%s) columns(prefix string, data utils.JSON) {", typ)
	g.line("\tif v == nil {")
	g.line("\t\treturn")
	g.line("\t}")
	for _, w := range wires {
		p := byWire[w]
		if inner, many := valueObjectOf(obj(props[p])); inner != "" && !many && g.schemas[inner] != nil {
			g.line("\tv.%s.columns(prefix+%q, data)", pascal(p), "_"+w)
			continue
		}
		if t := g.partType(obj(props[p])); strings.HasPrefix(t, "*") {
			g.line("\tif v.%s != nil {", pascal(p))
			g.line("\t\tdata[prefix+%q] = *v.%s", "_"+w, pascal(p))
			g.line("\t}")
			continue
		}
		g.line("\tif v.%s != nil {", pascal(p))
		g.line("\t\tdata[prefix+%q] = v.%s", "_"+w, pascal(p))
		g.line("\t}")
	}
	g.line("}")
	g.line("")
}

// nullableType reports whether a field's type list allows null.
func nullableType(s map[string]any) bool {
	for _, t := range list(s["type"]) {
		if text(t) == "null" {
			return true
		}
	}
	return false
}

// fold writes the function that gives a stored row of an entity the
// interface's shape: the columns of each value kept in columns, as
// specarch-gen-sql writes them out, gathered into the value, and each value
// kept as JSON read from its text.
func (g *gen) fold(entity string) {
	g.imports["github.com/donnyhardyanto/dxlib/utils"] = true
	e := obj(g.entities[entity])
	w := gensql.WriteOut(entity, e, g.schemas)
	byField := map[string][]string{}
	for col, from := range w.From {
		field, _, _ := strings.Cut(from, ".")
		byField[field] = append(byField[field], col)
	}
	g.line("// fold%s gives a stored %s the shape its interface gives it: the", entity, entity)
	g.line("// columns of a value kept in columns gathered into the value, and a value")
	g.line("// kept as JSON read from its text.")
	g.line("func fold%s(row utils.JSON) error {", entity)
	g.line("\tif row == nil {")
	g.line("\t\treturn nil")
	g.line("\t}")
	props := obj(e["properties"])
	for _, f := range sortedKeys(props) {
		name, many := valueObjectOf(obj(props[f]))
		if name == "" || g.schemas[name] == nil {
			continue
		}
		if many || text(obj(props[f])["storage"]) == "json" {
			g.helper("unjson")
			g.line("\tif err := unjson(row, %q); err != nil {", genopenapi.Wire(f))
			g.line("\t\treturn err")
			g.line("\t}")
			continue
		}
		g.helper("fold")
		cols := byField[f]
		sort.Strings(cols)
		var parts []string
		for _, col := range cols {
			path := []string{col}
			for _, seg := range strings.Split(w.From[col], ".")[1:] {
				path = append(path, genopenapi.Wire(seg))
			}
			parts = append(parts, "[]string{"+quoteAll(path)+"}")
		}
		g.line("\tfold(row, %q, %s)", genopenapi.Wire(f), strings.Join(parts, ", "))
	}
	g.line("\treturn nil")
	g.line("}")
	g.line("")
}

// keys are a map's keys in order.
func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
