package genopenapi

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/mark"
	"github.com/SpecArch/specarch/internal/ownership"
	"github.com/SpecArch/specarch/internal/wirename"
)

// The dxlib dialect: the document dxlib's OpenAPI reader binds
// (api/OPENAPI.md in dxlib). The reader is strict, so the document says
// only what dxlib's server does:
//
//   - one method per path: every operation is POST /<operationId> with one
//     JSON body holding all of its parameters, dxlib's own command
//     convention, since dxlib routes by the URI alone;
//   - a field keeps only the constraints dxlib's validator enforces; the
//     others are listed in x-specarch-unenforced on the field, so the
//     document says what the server does not check, and go-dxlib's handler
//     checks them;
//   - every field carries its dxlib type in x-dxlib-type;
//   - permissions are x-dxlib-privileges, and no security scheme is written,
//     since the reader refuses one it cannot enforce;
//   - a refusal answers dxlib's error body, named by its problem type.

// dxFormats are the formats the dxlib reader accepts, by dxlib type.
var dxFormats = map[string]string{
	"email": "email", "iso8601": "date-time", "date": "date", "time": "time",
	"int32": "int32", "int32p": "int32", "int32zp": "int32", "nullable-int32": "int32",
	"int64": "int64", "int64p": "int64", "int64zp": "int64", "nullable-int64": "int64",
	"float64": "double", "float64p": "double", "float64zp": "double",
	"blob": "binary",
}

// dxJSONType is the JSON type of a dxlib type.
func dxJSONType(t string) string {
	t = strings.TrimPrefix(t, "nullable-")
	switch {
	case strings.HasPrefix(t, "int"):
		return "integer"
	case strings.HasPrefix(t, "float"):
		return "number"
	case t == "bool":
		return "boolean"
	case t == "json" || t == "json-passthrough":
		return "object"
	case strings.HasPrefix(t, "array"):
		return "array"
	}
	return "string"
}

// dxType is the dxlib parameter type of a field, or "" with the reason it
// has none. A field that may be null takes dxlib's nullable type where its
// base has one: in dxlib a nullable parameter is one that may be not given,
// whether left out or sent as null, and its getter answers both alike.
func dxType(f map[string]any, enums map[string]any) (string, string) {
	t, why := dxBaseType(f, enums)
	if nullable(f) && (t == "string" || t == "int32" || t == "int64") {
		return "nullable-" + t, ""
	}
	return t, why
}

// dxBaseType is the dxlib parameter type of a field, nullability aside.
func dxBaseType(f map[string]any, enums map[string]any) (string, string) {
	if r := text(f["$ref"]); r != "" {
		if strings.HasPrefix(r, "#/enums/") {
			return "string", ""
		}
		return "json", ""
	}
	typ := text(f["type"])
	for _, t := range list(f["type"]) {
		if text(t) != "null" {
			typ = text(t)
		}
	}
	format := text(f["format"])
	min := text(f["minimum"])
	switch typ {
	case "boolean":
		return "bool", ""
	case "object":
		if obj(f["properties"]) != nil {
			return "json", ""
		}
		return "json-passthrough", ""
	case "array":
		items := obj(f["items"])
		switch it, _ := dxType(items, enums); {
		case items == nil:
			return "array", ""
		case it == "string":
			return "array-string", ""
		case strings.HasPrefix(it, "int64"):
			return "array-int64", ""
		case it == "json" || it == "json-passthrough":
			return "array-json-template", ""
		}
		return "array", ""
	case "number":
		switch {
		case text(f["exclusiveMinimum"]) == "0":
			return "float64p", ""
		case min == "0":
			return "float64zp", ""
		}
		return "float64", ""
	case "integer":
		if format == "uint64" {
			return "", "dxlib has no unsigned 64-bit type"
		}
		base := "int64"
		if format == "int32" {
			base = "int32"
		}
		switch min {
		case "1":
			return base + "p", ""
		case "0":
			return base + "zp", ""
		}
		return base, ""
	}
	switch format {
	case "decimal":
		return "money", ""
	case "int64":
		return "int64", ""
	case "uint64":
		return "", "dxlib has no unsigned 64-bit type"
	case "date-time":
		return "iso8601", ""
	case "date", "time", "email":
		return format, ""
	case "binary":
		return "blob", ""
	case "byte":
		return "", "dxlib has no base64 text type"
	}
	sens := text(f["sensitivity"])
	protected := sens == "personal" || sens == "credential"
	nonEmpty := text(f["minLength"]) == "1"
	switch {
	case protected && nonEmpty:
		return "protected-non-empty-string", ""
	case protected:
		return "protected-string", ""
	case nonEmpty:
		return "non-empty-string", ""
	}
	return "string", ""
}

// dxSchema writes a field in the dxlib dialect. at is its pointer, which
// names it in a diagnostic and in its marks.
func (g *gen) dxSchema(f map[string]any, at string) *yaml.Node {
	return g.record(at, g.dxField(f, at))
}

func (g *gen) dxField(f map[string]any, at string) *yaml.Node {
	if text(f["$ref"]) != "" {
		return g.dxRef(f, at)
	}
	t, why := dxType(f, obj(g.spec["enums"]))
	if t == "" {
		g.diags = append(g.diags, g.problem(at, "%s cannot be carried in the dxlib dialect: %s", at, why))
		return mapping()
	}
	n := mapping()
	jt := dxJSONType(t)
	if nullable(f) {
		add(n, "type", plain([]any{jt, "null"}))
	} else {
		add(n, "type", str(jt))
	}
	if fm := dxFormats[t]; fm != "" {
		add(n, "format", str(fm))
	}
	if d := text(f["description"]); d != "" {
		add(n, "description", str(d))
	}
	if e := list(f["enum"]); len(e) > 0 {
		add(n, "enum", plain(e))
	}
	switch {
	case strings.HasSuffix(t, "zp") && !strings.HasPrefix(t, "float"):
		add(n, "minimum", number("0"))
	case strings.HasSuffix(t, "p") && strings.HasPrefix(t, "int"):
		add(n, "minimum", number("1"))
	case t == "float64zp":
		add(n, "minimum", number("0"))
	case t == "float64p":
		add(n, "exclusiveMinimum", number("0"))
	case strings.Contains(t, "non-empty"):
		add(n, "minLength", number("1"))
	}
	if items, ok := f["items"].(map[string]any); ok {
		add(n, "items", g.dxSchema(items, at+"/items"))
	}
	if props, ok := f["properties"].(map[string]any); ok {
		pn := mapping()
		byWire := map[string]string{}
		for p := range props {
			byWire[Wire(p)] = p
		}
		for _, w := range sortedKeys(toAny(byWire)) {
			p := byWire[w]
			add(pn, w, g.dxSchema(obj(props[p]), at+"/properties/"+mark.Escape(p)))
		}
		add(n, "properties", pn)
		if req := list(f["required"]); len(req) > 0 {
			var wires []any
			for _, r := range req {
				if nullable(obj(props[text(r)])) {
					g.diags = append(g.diags, Diagnostic{File: g.root, Line: 1, Severity: "warning", Path: at + "/properties/" + text(r), Rule: "generator",
						Message: fmt.Sprintf("%s is required and may be null; dxlib reads a null and a left-out parameter alike, as not given, so it is not required in the dxlib dialect and the handler decides what not given means", text(r))})
					continue
				}
				wires = append(wires, Wire(text(r)))
			}
			if len(wires) > 0 {
				add(n, "required", plain(uniqueSorted(wires)))
			}
		}
	}
	add(n, "x-dxlib-type", str(t))
	g.dxExtensions(n, f, t)
	return n
}

// dxExtensions writes the constraints dxlib does not enforce and
// SpecArch's own keywords of a field of dxlib type t.
func (g *gen) dxExtensions(n *yaml.Node, f map[string]any, t string) {
	if u := unenforced(f, t); len(u) > 0 {
		add(n, "x-specarch-unenforced", plain(u))
	}
	for _, k := range extensionKeys {
		if v, ok := f[k]; ok {
			add(n, "x-specarch-"+k, plain(v))
		}
	}
}

// dxRef writes a field that is a $ref in the dxlib dialect. dxlib's reader
// takes nothing but extensions beside a $ref, so the constraints beside it
// are listed as unenforced, as on any other field, and each other keyword
// the dialect carries on a field is reported at the field rather than
// dropped: a description as a warning, and a type, enum, items,
// properties or required, which would change what the value may be, as
// an error. A title, default, examples, readOnly, writeOnly or
// deprecated is left out here as on every field of the dialect.
func (g *gen) dxRef(f map[string]any, at string) *yaml.Node {
	r := text(f["$ref"])
	n := ref("#/components/schemas/" + r[strings.LastIndex(r, "/")+1:])
	if text(f["description"]) != "" {
		g.diags = append(g.diags, Diagnostic{File: g.root, Line: 1, Severity: "warning", Path: at, Rule: "generator",
			Message: "the description beside the $ref is left out of the dxlib dialect: dxlib's reader takes nothing but extensions beside a $ref"})
	}
	for _, k := range []string{"type", "enum", "items", "properties", "required"} {
		if _, ok := f[k]; !ok {
			continue
		}
		g.diags = append(g.diags, g.problem(at, "%s beside the $ref cannot be carried in the dxlib dialect: dxlib's reader takes nothing but extensions beside a $ref, and leaving it out would change what the value may be", k))
	}
	t, _ := dxType(f, obj(g.spec["enums"]))
	g.dxExtensions(n, f, t)
	return n
}

// Wire is a field's name in the dxlib dialect: its column name, the field's
// name in snake case, since dxlib's standard operations take a parameter's
// name as the column it reads or writes.
func Wire(name string) string { return wirename.Snake(name) }

func toAny(m map[string]string) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

// DXType is the dxlib parameter type of a field, or "" with the reason it
// has none; the go-dxlib generator reads it so the two agree.
func DXType(f map[string]any) (string, string) { return dxType(f, nil) }

// Unenforced lists the constraints of a field that dxlib's validator does
// not apply, as "key: value", in a fixed order. go-dxlib checks them in
// the handler.
func Unenforced(f map[string]any) []string {
	t, _ := dxType(f, nil)
	var out []string
	for _, v := range unenforced(f, t) {
		out = append(out, text(v))
	}
	return out
}

func unenforced(f map[string]any, t string) []any {
	var out []any
	for _, k := range []string{"pattern", "minLength", "maxLength", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum",
		"multipleOf", "minItems", "maxItems", "uniqueItems", "const", "format"} {
		v, ok := f[k]
		if !ok {
			continue
		}
		val := text(v)
		switch k {
		case "minLength":
			if val == "1" && strings.Contains(t, "non-empty") {
				continue
			}
		case "minimum":
			if (val == "0" && strings.HasSuffix(t, "zp")) || (val == "1" && strings.HasSuffix(t, "p") && strings.HasPrefix(t, "int")) {
				continue
			}
		case "exclusiveMinimum":
			if val == "0" && t == "float64p" {
				continue
			}
		case "format":
			if dxFormats[t] == val || (t == "money" && val == "decimal") || t == "int64" && val == "int64" {
				continue
			}
		}
		out = append(out, k+": "+val)
	}
	return out
}

func nullable(f map[string]any) bool {
	for _, t := range list(f["type"]) {
		if text(t) == "null" {
			return true
		}
	}
	return false
}

// generateDxlib writes the document in the dxlib dialect.
func (g *gen) generateDxlib(r *Request) Response {
	doc := mapping()
	add(doc, "openapi", str("3.1.0"))
	info := mapping()
	specInfo := obj(g.spec["info"])
	add(info, "title", str(text(specInfo["title"])))
	add(info, "version", str(text(specInfo["version"])))
	if d := text(specInfo["description"]); d != "" {
		add(info, "description", str(d))
	}
	add(doc, "info", info)
	paths := mapping()
	type op struct {
		path, method string
		node         map[string]any
		item         map[string]any
	}
	var ops []op
	for _, p := range sortedKeys(obj(g.spec["paths"])) {
		item := obj(obj(g.spec["paths"])[p])
		for _, m := range []string{"get", "put", "post", "delete", "patch"} {
			if o, ok := item[m].(map[string]any); ok && !g.owned.Covers(ownership.Operation(p, m)) {
				ops = append(ops, op{p, m, o, item})
			}
		}
	}
	sort.SliceStable(ops, func(i, j int) bool { return text(ops[i].node["operationId"]) < text(ops[j].node["operationId"]) })
	for _, o := range ops {
		id := text(o.node["operationId"])
		pi := mapping()
		add(pi, "post", g.record("/paths/"+mark.Escape(o.path)+"/"+o.method, g.dxOperation(o.path, o.method, o.node, o.item)))
		add(paths, "/"+id, pi)
	}
	add(doc, "paths", paths)
	comps := mapping()
	schemas := mapping()
	for _, name := range sortedKeys(obj(g.spec["enums"])) {
		if g.owned.Covers(ownership.Entity("enums", name)) {
			continue
		}
		e := obj(obj(g.spec["enums"])[name])
		n := mapping()
		add(n, "type", str("string"))
		if d := text(e["description"]); d != "" {
			add(n, "description", str(d))
		}
		add(n, "enum", plain(e["enum"]))
		add(n, "x-dxlib-type", str("string"))
		add(schemas, name, g.record("/enums/"+mark.Escape(name), n))
	}
	viewSchemas := map[string]*yaml.Node{}
	for _, name := range sortedKeys(obj(g.spec["entities"])) {
		e := obj(obj(g.spec["entities"])[name])
		props := obj(e["properties"])
		withAudit := map[string]any{}
		for k, v := range props {
			withAudit[k] = v
		}
		if e["audited"] == true {
			names, _ := g.idiomNames("audit-fields", "columns", "any")
			for _, a := range []struct{ key, format, description string }{
				{"createdAt", "date-time", "When the record was created; set by the system."}, {"createdBy", "", "Who created the record; set by the system."},
				{"createdByName", "", "The name of who created the record; set by the system."}, {"lastModifiedAt", "date-time", "When the record was last changed; set by the system."},
				{"lastModifiedBy", "", "Who last changed the record; set by the system."}, {"lastModifiedByName", "", "The name of who last changed the record; set by the system."}} {
				if names[a.key] == "" {
					continue
				}
				fm := map[string]any{"type": "string", "description": a.description}
				if a.format != "" {
					fm["format"] = a.format
				}
				withAudit[names[a.key]] = fm
			}
		}
		// An owned entity gets no schema; a view of it is still the project's.
		if !g.owned.Covers(ownership.Entity("entities", name)) {
			schemas.Content = append(schemas.Content, str(name), g.dxSchema(map[string]any{"type": "object", "description": e["description"], "properties": withAudit, "required": e["required"]}, "/entities/"+mark.Escape(name)))
		}
		// A view of the entity is its fields and the fields the view adds.
		for _, vn := range sortedKeys(obj(g.spec["views"])) {
			v := obj(obj(g.spec["views"])[vn])
			if text(v["from"]) != name {
				continue
			}
			extra, added := g.viewAdded(vn)
			all := map[string]any{}
			for k, f := range withAudit {
				all[k] = f
			}
			for k, f := range extra {
				all[k] = f
			}
			// A field that may be null is not required in the dxlib
			// dialect; the entity's schema warns about it once.
			var req []any
			for _, r := range append(append([]any{}, list(e["required"])...), added...) {
				if !nullable(obj(all[text(r)])) {
					req = append(req, r)
				}
			}
			viewSchemas[vn] = g.dxSchema(map[string]any{"type": "object", "description": v["description"], "properties": all, "required": req}, "/views/"+vn)
		}
	}
	for _, vn := range sortedKeys(obj(g.spec["views"])) {
		if n := viewSchemas[vn]; n != nil && !g.owned.Covers(ownership.Entity("views", vn)) {
			add(schemas, vn, n)
		}
	}
	// A schema is its own fields; it has no table, so no audit fields.
	for _, sn := range sortedKeys(obj(g.spec["schemas"])) {
		if !g.owned.Covers(ownership.Entity("schemas", sn)) {
			v := obj(obj(g.spec["schemas"])[sn])
			add(schemas, sn, g.dxSchema(map[string]any{"type": "object", "description": v["description"], "properties": v["properties"], "required": v["required"]}, "/schemas/"+sn))
		}
	}
	add(comps, "schemas", schemas)
	if errs := obj(g.spec["errors"]); len(errs) > 0 {
		cat := mapping()
		for _, name := range sortedKeys(errs) {
			e := obj(errs[name])
			n := mapping()
			for _, k := range []string{"status", "title", "condition", "type"} {
				if v, ok := e[k]; ok {
					add(n, k, plain(v))
				}
			}
			add(cat, name, n)
		}
		add(comps, "x-specarch-problems", cat)
	}
	add(doc, "components", comps)
	g.markDoc(doc, r)
	var b bytes.Buffer
	fmt.Fprintf(&b, "# Generated by specarch-gen-openapi from %s, version %s; meta-model %s; the dxlib dialect. Do not edit this file: change the YAML and generate again.\n", relRoot(r.Root, r.Output), text(specInfo["version"]), r.Specarch)
	b.WriteString(draftLine(r))
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return Response{Files: []File{}, Diagnostics: []Diagnostic{g.problem("/", "the document cannot be written as YAML (%v); this is a bug in specarch-gen-openapi", err)}}
	}
	return Response{Files: []File{{Path: FileName, Content: b.String()}}, Diagnostics: append([]Diagnostic{}, g.diags...)}
}

// dxOperation writes one operation as dxlib binds it: a POST whose JSON
// body holds every parameter.
func (g *gen) dxOperation(path, method string, op, item map[string]any) *yaml.Node {
	id := text(op["operationId"])
	at := "/paths/" + strings.NewReplacer("~", "~0", "/", "~1").Replace(path) + "/" + method
	n := mapping()
	add(n, "operationId", str(id))
	if s := text(op["summary"]); s != "" {
		add(n, "summary", str(s))
	}
	if d := text(op["description"]); d != "" {
		add(n, "description", str(d))
	}
	props := map[string]any{}
	var required []any
	itemAt := at[:strings.LastIndex(at, "/")]
	var params []any
	var paramAt []string
	for i, p := range list(item["parameters"]) {
		params, paramAt = append(params, p), append(paramAt, fmt.Sprintf("%s/parameters/%d", itemAt, i))
	}
	for i, p := range list(op["parameters"]) {
		params, paramAt = append(params, p), append(paramAt, fmt.Sprintf("%s/parameters/%d", at, i))
	}
	for i, p := range params {
		pm := obj(p)
		if text(pm["in"]) == "header" {
			continue // dxlib reads a header itself; it is not a parameter of the body
		}
		s := obj(pm["schema"])
		if d := text(pm["description"]); d != "" && text(s["description"]) == "" {
			if text(s["$ref"]) != "" {
				g.diags = append(g.diags, Diagnostic{File: g.root, Line: 1, Severity: "warning", Path: paramAt[i], Rule: "generator",
					Message: "the description is left out of the dxlib dialect: the parameter's schema is a $ref, and dxlib's reader takes nothing but extensions beside one"})
			} else {
				s = withKey(s, "description", d)
			}
		}
		props[text(pm["name"])] = s
		if pm["required"] == true {
			required = append(required, text(pm["name"]))
		}
	}
	for _, c := range obj(obj(op["requestBody"])["content"]) {
		schema := obj(obj(c)["schema"])
		if r := text(schema["$ref"]); strings.HasPrefix(r, "#/entities/") || strings.HasPrefix(r, "#/schemas/") {
			section, name, _ := strings.Cut(strings.TrimPrefix(r, "#/"), "/")
			e := obj(obj(g.spec[section])[name])
			req := map[string]bool{}
			for _, x := range list(e["required"]) {
				req[text(x)] = true
			}
			for k, v := range obj(e["properties"]) {
				if obj(v)["readOnly"] == true {
					continue
				}
				props[k] = v
				if req[k] {
					required = append(required, k)
				}
			}
		} else {
			for k, v := range obj(schema["properties"]) {
				props[k] = v
			}
			required = append(required, list(schema["required"])...)
		}
		break
	}
	l, isList := op["listOf"].(map[string]any)
	if isList && !g.dxListParameters(at, l, props) {
		return n
	}
	if len(props) > 0 {
		body := mapping()
		add(body, "required", boolean(true))
		c := mapping()
		m := mapping()
		sortReq := uniqueSorted(required)
		add(m, "schema", g.dxSchema(map[string]any{"type": "object", "properties": props, "required": sortReq}, at+"/requestBody"))
		add(c, "application/json", m)
		add(body, "content", c)
		add(n, "requestBody", body)
	}
	add(n, "responses", g.dxResponses(at, op, l, isList))
	add(n, "x-dxlib-endpoint-type", str("EndPointTypeHTTPJSON"))
	if perm := text(op["permission"]); perm != "" && perm != "public" {
		add(n, "x-dxlib-privileges", plain([]any{perm}))
	}
	if lim := obj(op["limits"]); lim != nil {
		if m := text(lim["maxRequestBytes"]); m != "" {
			add(n, "x-dxlib-max-content-length", number(m))
		}
		if lim["rate"] != nil {
			add(n, "x-dxlib-rate-limit-group", str(id))
		}
	}
	if perm := text(op["permission"]); perm != "" {
		add(n, "x-specarch-permission", str(perm))
	}
	return n
}

func wires(names []any) []any {
	out := make([]any, len(names))
	for i, n := range names {
		out[i] = Wire(text(n))
	}
	return out
}

func withKey(m map[string]any, k string, v any) map[string]any {
	out := map[string]any{}
	for key, val := range m {
		out[key] = val
	}
	out[k] = v
	return out
}

func uniqueSorted(items []any) []any {
	seen := map[string]bool{}
	var names []string
	for _, i := range items {
		if !seen[text(i)] {
			seen[text(i)] = true
			names = append(names, text(i))
		}
	}
	sort.Strings(names)
	out := make([]any, len(names))
	for i, n := range names {
		out[i] = n
	}
	return out
}

// dxListParameters adds a list's parameters by the dxlib names of the
// paginated-list idiom, and reports false when the idiom does not apply.
func (g *gen) dxListParameters(at string, l, props map[string]any) bool {
	names, ok := g.idiomNames("paginated-list", "parameters", "dxlib")
	if !ok {
		g.diags = append(g.diags, g.problem(at+"/listOf", "the operation lists %s, but the paginated-list idiom has no dxlib rendering here; keep the shipped idiom, or override its dxlib part", subjectName(l)))
		return false
	}
	_, fields, entity := g.listSubject(l)
	if len(list(l["searchable"])) > 0 {
		props[names["search"]] = map[string]any{"type": "string", "description": "Free text, matched against " + joinList(list(l["searchable"])) + "."}
	}
	if f := list(l["filterable"]); len(f) > 0 {
		fp := map[string]any{}
		for _, name := range f {
			fp[text(name)] = fields[text(name)]
		}
		props[names["filter"]] = map[string]any{"type": "object", "properties": fp}
	}
	if f := list(l["sortable"]); len(f) > 0 {
		props[names["sort"]] = map[string]any{"type": "array", "items": map[string]any{"type": "object",
			"properties": map[string]any{
				names["sortField"]:     map[string]any{"type": "string", "enum": wires(f)},
				names["sortDirection"]: map[string]any{"type": "string", "enum": []any{"asc", "desc"}}},
			"required": []any{names["sortField"], names["sortDirection"]}}}
	}
	props[names["page"]] = map[string]any{"type": "integer", "format": "int64", "minimum": "0", "description": "The page, counted from 0."}
	size := obj(l["pageSize"])
	ps := map[string]any{"type": "integer", "format": "int64", "minimum": "1", "maximum": size["maximum"]}
	props[names["pageSize"]] = ps
	if text(entity["deletion"]) == "soft" {
		props[names["includeDeleted"]] = map[string]any{"type": "boolean", "description": "Also list the records that are deleted."}
	}
	return true
}

// dxResponses writes the responses: a success named success, a list in the
// idiom's envelope, and a refusal as dxlib's error body named by its
// problem type.
func (g *gen) dxResponses(at string, op, l map[string]any, isList bool) *yaml.Node {
	out := mapping()
	rs := obj(op["responses"])
	for _, code := range sortedKeys(rs) {
		r := obj(rs[code])
		n := mapping()
		add(n, "description", str(text(r["description"])))
		name := "success"
		if !strings.HasPrefix(code, "2") {
			name = "error_" + code
		}
		if p := text(r["problem"]); p != "" {
			name = strings.ReplaceAll(p, "-", "_")
		}
		add(n, "x-dxlib-response-name", str(name))
		var schema *yaml.Node
		switch {
		case !strings.HasPrefix(code, "2"):
			schema = g.dxErrorBody(at)
		case isList:
			schema = g.dxEnvelope(at, obj(obj(obj(r["content"])["application/json"])["schema"]))
		default:
			for _, c := range obj(r["content"]) {
				s := obj(obj(c)["schema"])
				if s["items"] != nil {
					g.diags = append(g.diags, g.problem(at+"/responses/"+code, "the %s response is a bare list, and dxlib answers an object; give the operation listOf, or answer an object", code))
				} else {
					schema = g.dxSchema(s, at+"/responses/"+code)
				}
				break
			}
		}
		if schema != nil {
			c := mapping()
			m := mapping()
			add(m, "schema", schema)
			add(c, "application/json", m)
			add(n, "content", c)
		}
		add(out, code, n)
	}
	return out
}

// dxErrorBody is dxlib's error body, by the names of the error-response
// idiom's dxlib rendering.
func (g *gen) dxErrorBody(at string) *yaml.Node {
	names, ok := g.idiomNames("error-response", "shape", "dxlib")
	if !ok {
		names = map[string]string{"status": "status", "statusCode": "status_code", "reason": "reason", "reasonMessage": "reason_message"}
	}
	return g.dxSchema(map[string]any{"type": "object", "properties": map[string]any{
		names["status"]:        map[string]any{"type": "string"},
		names["statusCode"]:    map[string]any{"type": "integer", "format": "int32"},
		names["reason"]:        map[string]any{"type": "string"},
		names["reasonMessage"]: map[string]any{"type": "string"},
	}}, at+"/error")
}

// dxEnvelope wraps a list's records in the idiom's dxlib envelope, whose
// names may be dotted to nest.
func (g *gen) dxEnvelope(at string, listSchema map[string]any) *yaml.Node {
	names, ok := g.idiomNames("paginated-list", "envelope", "dxlib")
	if !ok {
		return g.dxSchema(listSchema, at)
	}
	root := map[string]any{"type": "object", "properties": map[string]any{}}
	put := func(dotted string, v map[string]any) {
		parts := strings.Split(dotted, ".")
		cur := root
		for _, p := range parts[:len(parts)-1] {
			props := cur["properties"].(map[string]any)
			next, ok := props[p].(map[string]any)
			if !ok {
				next = map[string]any{"type": "object", "properties": map[string]any{}}
				props[p] = next
			}
			cur = next
		}
		cur["properties"].(map[string]any)[parts[len(parts)-1]] = v
	}
	put(names["items"], listSchema)
	total := map[string]any{"type": "integer", "format": "int64", "minimum": "0"}
	put(names["totalItems"], total)
	put(names["totalPages"], total)
	return g.dxSchema(root, at+"/envelope")
}

// idiomNames reads the names of an idiom's part for a stack, override
// first, from the request's idioms.
func (g *gen) idiomNames(idiom, part, stack string) (map[string]string, bool) {
	for _, i := range g.idioms {
		if i.Name != idiom || i.As == "excluded" || i.Content == nil {
			continue
		}
		pick := func(m map[string]any) map[string]any {
			return obj(obj(obj(obj(obj(m["parts"])[part])["stack"])[stack])["names"])
		}
		src := pick(i.Content)
		if i.Override != nil {
			if o := pick(i.Override); len(o) > 0 {
				src = o
			}
		}
		if len(src) == 0 {
			return nil, false
		}
		out := map[string]string{}
		for k, v := range src {
			out[k] = text(v)
		}
		return out, true
	}
	return nil, false
}
