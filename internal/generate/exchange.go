package generate

import (
	"fmt"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// bodyMethods are the methods whose request usually carries a body, so an
// operation of one that declares none says so.
var bodyMethods = map[string]bool{"post": true, "put": true, "patch": true}

// operationParameters are the parameters an operation takes: the ones its
// path declares, then its own, an own parameter replacing the path's of
// the same name and place, as OpenAPI has it.
func operationParameters(o operation) []*yaml.Node {
	own := items(o.node, "parameters")
	var out []*yaml.Node
	for _, p := range items(o.pathItem, "parameters") {
		replaced := false
		for _, q := range own {
			replaced = replaced || (str(q, "name") == str(p, "name") && str(q, "in") == str(p, "in"))
		}
		if !replaced {
			out = append(out, p)
		}
	}
	return append(out, own...)
}

// operationRequest writes what an operation takes: its parameters and its
// request body, each field with its type, whether it is required, its
// limits and its description. explained holds the path parameters whose
// notes are already written under an earlier operation of the same path.
func operationRequest(d *doc, root *yaml.Node, o operation, explained map[*yaml.Node]bool) {
	at := []string{"paths", o.path, o.method}
	params := operationParameters(o)
	body := get(o.node, "requestBody")
	if len(params) == 0 && body == nil {
		text := "Takes no parameters and no request body"
		if get(o.node, "listOf") != nil {
			text = "Declares no parameter beyond those of the list below, and no request body"
		}
		d.para(text + d.openUnder(at, "parameters", "requestBody") + ".")
		return
	}
	if len(params) > 0 {
		d.para("Parameters:")
		d.line("| Parameter | In | Type | Required | Limits | Description |")
		d.line("|---|---|---|---|---|---|")
		var rows []row
		for _, p := range params {
			req := ""
			if str(p, "required") == "true" {
				req = "yes"
			}
			schema := get(p, "schema")
			d.line("| %s | %s | %s | %s | %s | %s |", cell(str(p, "name")), str(p, "in"), cell(linkedType(root, schema)), cell(req), cell(limits(schema)), cell(fieldDescription(p, schema)))
			if !explained[p] {
				explained[p] = true
				rows = append(rows, row{"parameter " + str(p, "name"), p})
			}
		}
		d.blank()
		d.explainRows(rows)
	}
	if body == nil {
		if bodyMethods[o.method] {
			d.para("Takes no request body" + d.openUnder(at, "requestBody") + ".")
		}
		return
	}
	need := "optional"
	if str(body, "required") == "true" {
		need = "required"
	}
	d.para(str(body, "description"))
	content := pairs(body, "content")
	for _, c := range content {
		d.body(root, fmt.Sprintf("Request body, %s, %s", c.Key.Value, need), "request body", get(c.Value, "schema"))
	}
	d.explainRows([]row{{"the request body", body}})
}

// operationResponses writes every response an operation declares: its
// status, what it means and its body, then the fields of each body
// declared in place.
func operationResponses(d *doc, root *yaml.Node, o operation) {
	responses := pairs(o.node, "responses")
	if len(responses) == 0 {
		d.para("Declares no response" + d.openUnder([]string{"paths", o.path, o.method}, "responses") + ".")
		return
	}
	d.para("Responses:")
	d.line("| Status | Meaning | Body |")
	d.line("|---|---|---|")
	var rows []row
	for _, r := range responses {
		var bodies []string
		for _, c := range pairs(r.Value, "content") {
			text := bodySummary(root, get(c.Value, "schema"))
			if c.Key.Value != "application/json" || len(pairs(r.Value, "content")) > 1 {
				text = c.Key.Value + ": " + text
			}
			bodies = append(bodies, text)
		}
		if p := str(r.Value, "problem"); p != "" {
			bodies = append(bodies, "the problem document "+p+" (RFC 9457)")
		}
		if len(bodies) == 0 {
			bodies = []string{"none"}
		}
		d.line("| %s | %s | %s |", cell(r.Key.Value), cell(str(r.Value, "description")), cell(strings.Join(bodies, "; ")))
		rows = append(rows, row{"response " + r.Key.Value, r.Value})
	}
	d.blank()
	d.explainRows(rows)
	for _, r := range responses {
		for _, c := range pairs(r.Value, "content") {
			schema := get(c.Value, "schema")
			if len(bodyFields(schema)) == 0 {
				continue
			}
			lead := "Body of the " + r.Key.Value + " response"
			if c.Key.Value != "application/json" || len(pairs(r.Value, "content")) > 1 {
				lead += ", " + c.Key.Value
			}
			d.body(root, lead, "response "+r.Key.Value, schema)
		}
	}
}

// bodySummary is a body as a cell says it: its type, linked where it is
// declared, or that its fields are listed below.
func bodySummary(root *yaml.Node, schema *yaml.Node) string {
	if len(bodyFields(schema)) > 0 {
		if str(schema, "type") == "array" {
			return withLimits("a list of objects, fields below", schema)
		}
		return "an object, fields below"
	}
	return withLimits(linkedType(root, schema), schema)
}

func withLimits(text string, f *yaml.Node) string {
	if l := limits(f); l != "" {
		return text + ", " + l
	}
	return text
}

// bodyFields are the fields a body declares in place: an object's
// properties, or those of each item of a list of objects; nil for a body
// that is a reference or a single value.
func bodyFields(schema *yaml.Node) []fieldRow {
	if str(schema, "type") == "array" {
		return fieldRows(get(schema, "items"), "")
	}
	return fieldRows(schema, "")
}

// body writes a body: its fields in a table when it declares them in
// place, or else its type in one sentence.
func (d *doc) body(root *yaml.Node, lead, label string, schema *yaml.Node) {
	fields := bodyFields(schema)
	if len(fields) == 0 {
		d.para(lead + ": " + withLimits(linkedType(root, schema), schema) + ".")
		return
	}
	if str(schema, "type") == "array" {
		d.para(lead + ": " + withLimits("a list", schema) + ", each item with these fields:")
	} else {
		d.para(lead + ":")
	}
	d.line("| Field | Type | Required | Limits | Description |")
	d.line("|---|---|---|---|---|")
	var rows []row
	for _, f := range fields {
		req := ""
		if f.required {
			req = "yes"
		}
		d.line("| %s | %s | %s | %s | %s |", cell(f.path), cell(linkedType(root, f.node)), cell(req), cell(limits(f.node)), cell(fieldDescription(f.node, f.node)))
		rows = append(rows, row{label + " field " + f.path, f.node})
	}
	d.blank()
	d.explainRows(rows)
}

// fieldRow is one field of a body declared in place, at any depth: its
// path, a nested object's field after a dot and a list's item field after
// [], and whether the object holding it requires it.
type fieldRow struct {
	path     string
	node     *yaml.Node
	required bool
}

func fieldRows(object *yaml.Node, prefix string) []fieldRow {
	required := map[string]bool{}
	for _, r := range strs(object, "required") {
		required[r] = true
	}
	var out []fieldRow
	for _, p := range pairs(object, "properties") {
		path := prefix + p.Key.Value
		out = append(out, fieldRow{path, p.Value, required[p.Key.Value]})
		switch {
		case get(p.Value, "properties") != nil:
			out = append(out, fieldRows(p.Value, path+".")...)
		case get(get(p.Value, "items"), "properties") != nil:
			out = append(out, fieldRows(get(p.Value, "items"), path+"[].")...)
		}
	}
	return out
}

// fieldDescription is what a field is for: its description, or else its
// title. n holds the description; a parameter has its own, beside its
// schema.
func fieldDescription(n, schema *yaml.Node) string {
	if s := str(n, "description"); s != "" {
		return s
	}
	if s := str(schema, "description"); s != "" {
		return s
	}
	return str(schema, "title")
}

// linkSections are the sections a reference names whose elements have a
// heading of their own in the techspec, and the section whose elements
// share one table.
var (
	linkSections  = map[string]bool{"entities": true, "views": true, "schemas": true}
	tableSections = map[string]string{"enums": "Enums"}
)

// linkedType is typeText with each reference to an entity, view, schema or
// enum of this specification linked to where the techspec describes it,
// and a reference into another specification named with its version.
func linkedType(root *yaml.Node, f *yaml.Node) string {
	t := typeText(f)
	for _, ref := range references(f) {
		name := ref[strings.LastIndex(ref, "/")+1:]
		if !strings.HasPrefix(ref, "#") {
			if of := referenceOf(root, ref); of != "" {
				t = strings.Replace(t, ref, ref+" ("+of+")", 1)
			}
			continue
		}
		parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
		if len(parts) != 2 || get(get(root, parts[0]), name) == nil {
			continue
		}
		anchor := ""
		switch {
		case linkSections[parts[0]]:
			anchor = slug(name)
		case tableSections[parts[0]] != "":
			anchor = slug(tableSections[parts[0]])
		default:
			continue
		}
		word := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
		done := false
		t = word.ReplaceAllStringFunc(t, func(m string) string {
			if done {
				return m
			}
			done = true
			return "[" + m + "](#" + anchor + ")"
		})
	}
	return t
}

// references are the references a field makes: its own, its items' and
// its map values'.
func references(f *yaml.Node) []string {
	var out []string
	for f != nil {
		if ref := str(f, "$ref"); ref != "" {
			out = append(out, ref)
		}
		next := get(f, "items")
		if values, ok := mapValues(f); ok && values != nil {
			next = values
		}
		f = next
	}
	return out
}

// slug is the anchor a Markdown renderer gives a heading, as GitHub makes
// it: lower case, spaces as hyphens, other punctuation dropped.
func slug(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}

// openUnder names the open questions that block a key of an element, or
// anything under it, as "; <key> is open in <IDs>", for each key that has
// any; "" when none does.
func (d *doc) openUnder(at []string, keys ...string) string {
	var out []string
	for _, key := range keys {
		ptr := source.Pointer(append(append([]string{}, at...), key)...)
		var ids []string
		for _, q := range d.questions {
			for _, b := range q.blocks {
				if !b.IsPointer() {
					continue
				}
				if p := source.Pointer(b.Tokens...); p == ptr || strings.HasPrefix(p, ptr+"/") {
					ids = append(ids, q.id)
					break
				}
			}
		}
		if len(ids) > 0 {
			out = append(out, fmt.Sprintf("; %s is open in %s", map[string]string{"parameters": "what it takes", "requestBody": "the request body", "responses": "what it answers"}[key], joinAnd(ids)))
		}
	}
	return strings.Join(out, "")
}
