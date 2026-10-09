package gentests

import (
	"encoding/json"
	"testing"
)

// TestValueWithoutField gives an object or a list whose field is not known
// its kind as written, with its parts and items.
func TestValueWithoutField(t *testing.T) {
	g := &walker{spec: map[string]any{}}
	v := g.value(map[string]any{"city": "York", "location": map[string]any{"latitude": json.Number("53.9")}}, nil)
	if v.Kind != "object" || len(v.Parts) != 2 || v.Parts[1].Value.Kind != "object" || v.Parts[1].Value.Parts[0].Value.Kind != "double" {
		t.Errorf("an object as written is %+v", v)
	}
	l := g.value([]any{"a", true}, nil)
	if l.Kind != "list" || len(l.Items) != 2 || l.Items[1].Kind != "bool" {
		t.Errorf("a list as written is %+v", l)
	}
}

// TestValueOfSchema types a value object's parts, and a list's items, by
// the schema the field refers to.
func TestValueOfSchema(t *testing.T) {
	g := &walker{spec: map[string]any{"schemas": map[string]any{
		"Fee": map[string]any{"properties": map[string]any{"amount": map[string]any{"type": "string", "format": "decimal"}}},
	}}}
	v := g.value(map[string]any{"amount": "12.50"}, map[string]any{"$ref": "#/schemas/Fee"})
	if v.Kind != "object" || v.Parts[0].Value.Kind != "decimal" {
		t.Errorf("a fee is %+v", v)
	}
	l := g.value([]any{map[string]any{"amount": "1"}}, map[string]any{"type": "array", "items": map[string]any{"$ref": "#/schemas/Fee"}})
	if l.Kind != "list" || l.Items[0].Parts[0].Value.Kind != "decimal" {
		t.Errorf("a list of fees is %+v", l)
	}
}
