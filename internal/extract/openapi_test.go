package extract

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func openapiField(t *testing.T, v30 bool, schema string) (string, *openapiReader) {
	t.Helper()
	var n yaml.Node
	if err := yaml.Unmarshal([]byte(schema), &n); err != nil {
		t.Fatal(err)
	}
	o := &openapiReader{doc: &yaml.Node{Kind: yaml.MappingNode}, v30: v30, written: map[string]string{"Loan": "entities"}}
	out := o.field(n.Content[0], "the field", "#/entities/X/properties/f", 0)
	b, err := yaml.Marshal(flow(out))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b)), o
}

// TestOpenAPIField writes OpenAPI 3.0's nullable, example and boolean
// exclusive bounds in the 3.1 form, and a reference to a written
// component schema as a reference to its entity.
func TestOpenAPIField(t *testing.T) {
	for _, c := range []struct {
		v30          bool
		schema, want string
	}{
		{true, `{type: string, format: date, nullable: true}`, `{type: [string, "null"], format: date}`},
		{true, `{type: integer, format: int32, minimum: 0, exclusiveMinimum: true, example: 3}`, `{type: integer, format: int32, exclusiveMinimum: 0, examples: [3]}`},
		{false, `{type: integer, format: int32, exclusiveMinimum: 0}`, `{type: integer, format: int32, exclusiveMinimum: 0}`},
		{false, `{$ref: "#/components/schemas/Loan"}`, `{$ref: '#/entities/Loan'}`},
		{false, `{type: array, items: {type: string, format: uuid}}`, `{type: array, items: {type: string, format: uuid}}`},
	} {
		got, o := openapiField(t, c.v30, c.schema)
		if got != c.want {
			t.Errorf("%s: got %s, want %s", c.schema, got, c.want)
		}
		if len(o.widths) != 0 || len(o.notHeld) != 0 {
			t.Errorf("%s: asked %v, not held %v", c.schema, o.widths, o.notHeld)
		}
	}
}

// TestOpenAPIWidth asks the width of an integer or number with none the
// meta-model holds, an int64 without bounds inside 2^53 among them, and
// leaves the format out.
func TestOpenAPIWidth(t *testing.T) {
	for _, c := range []struct {
		schema, want string
		asked        bool
	}{
		{`{type: integer}`, `{type: integer}`, true},
		{`{type: integer, format: int64}`, `{type: integer}`, true},
		{`{type: number, format: float}`, `{type: number}`, true},
		{`{type: integer, format: int64, minimum: 1, maximum: 9007199254740991}`, `{type: integer, format: int64, minimum: 1, maximum: 9007199254740991}`, false},
		{`{type: integer, format: int64, minimum: 1, maximum: 9007199254740992}`, `{type: integer, minimum: 1, maximum: 9007199254740992}`, true},
	} {
		got, o := openapiField(t, false, c.schema)
		if got != c.want {
			t.Errorf("%s: got %s, want %s", c.schema, got, c.want)
		}
		if (len(o.widths) == 1) != c.asked {
			t.Errorf("%s: asked %v", c.schema, o.widths)
		}
	}
}

// TestJSONNode keeps a JSON object's keys in the order written and each
// number as written.
func TestJSONNode(t *testing.T) {
	n, err := jsonNode([]byte(`{"zeta": 1, "alpha": [0.50, true, null], "mid": {"b": "x", "a": 2}}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := yaml.Marshal(flow(n))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(b)), `{zeta: 1, alpha: [0.50, true, null], mid: {b: x, a: 2}}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if _, err := jsonNode([]byte(`{"a": 1} {"b": 2}`)); err == nil {
		t.Error("text after the document was read")
	}
}

// TestOpenAPIImpliedType reads properties with no type as an object and
// items with no type as an array.
func TestOpenAPIImpliedType(t *testing.T) {
	for schema, want := range map[string]string{
		`{properties: {name: {type: string}}}`: `{type: object, properties: {name: {type: string}}}`,
		`{items: {type: string}}`:              `{type: array, items: {type: string}}`,
	} {
		if got, _ := openapiField(t, false, schema); got != want {
			t.Errorf("%s: got %s, want %s", schema, got, want)
		}
	}
}

// TestOpenAPIEntityNeedsAProperty writes an object schema none of whose
// properties can be held in place, so no reference points at an entity
// that is not written.
func TestOpenAPIEntityNeedsAProperty(t *testing.T) {
	var doc yaml.Node
	src := `
openapi: 3.1.0
paths: {}
components:
  schemas:
    Token: {type: object, properties: {access_token: {type: string}}}
    Wrapper: {type: object, properties: {token: {$ref: "#/components/schemas/Token"}}}
    Problem: {properties: {title: {type: string}}}
`
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	o := &openapiReader{doc: doc.Content[0], key: "api", res: &Result{Tree: newTree()}, questions: &yaml.Node{Kind: yaml.MappingNode}}
	o.read()
	if got, want := o.written, map[string]string{"Wrapper": "entities", "Problem": "entities"}; len(got) != len(want) || got["Wrapper"] != want["Wrapper"] || got["Problem"] != want["Problem"] {
		t.Errorf("written %v, want %v", got, want)
	}
}
