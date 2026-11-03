package transition

import (
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"
)

// TestSelect checks that the to-be loses asIs, that --as-is puts the
// as-is in the workflow's place, that a workflow with no as-is is the
// same either way, and that the specification given is not changed.
func TestSelect(t *testing.T) {
	asIs := map[string]any{"trigger": "askRefund", "steps": []any{"check"}}
	spec := map[string]any{
		"info": map[string]any{"title": "Case"},
		"workflows": map[string]any{
			"refund": map[string]any{"trigger": "askRefund", "steps": []any{"route"}, "satisfies": []any{"CASE-1"}, "asIs": asIs},
			"plain":  map[string]any{"trigger": "askPlain"},
		},
	}
	toBe := Select(spec, false).(map[string]any)
	want := map[string]any{"trigger": "askRefund", "steps": []any{"route"}, "satisfies": []any{"CASE-1"}}
	if got := toBe["workflows"].(map[string]any)["refund"]; !reflect.DeepEqual(got, want) {
		t.Errorf("the to-be is %v, want %v", got, want)
	}
	old := Select(spec, true).(map[string]any)
	wantOld := map[string]any{"trigger": "askRefund", "steps": []any{"check"}, "satisfies": []any{"CASE-1"}}
	if got := old["workflows"].(map[string]any)["refund"]; !reflect.DeepEqual(got, wantOld) {
		t.Errorf("the as-is is %v, want %v", got, wantOld)
	}
	if _, ok := asIs["satisfies"]; ok {
		t.Error("Select changed the as-is it was given")
	}
	for _, s := range []map[string]any{toBe, old} {
		if got := s["workflows"].(map[string]any)["plain"]; !reflect.DeepEqual(got, map[string]any{"trigger": "askPlain"}) {
			t.Errorf("a workflow with no as-is became %v", got)
		}
		if !reflect.DeepEqual(s["info"], spec["info"]) {
			t.Errorf("the rest of the specification changed: %v", s["info"])
		}
	}
	if _, ok := spec["workflows"].(map[string]any)["refund"].(map[string]any)["asIs"]; !ok {
		t.Error("Select changed the specification it was given")
	}
}

// TestPointers checks that a problem's pointer moves to where a plug-in
// given the as-is sees the entry, and that a diagnostic's moves back.
func TestPointers(t *testing.T) {
	var doc yaml.Node
	src := "workflows:\n  refund: { trigger: a, asIs: { trigger: b } }\n  plain: { trigger: c }\n"
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	root := doc.Content[0]
	for _, c := range []struct {
		in, out string
		ok      bool
	}{
		{"/workflows/refund/asIs/steps/3", "/workflows/refund/steps/3", true},
		{"#/workflows/refund/asIs/steps/3", "#/workflows/refund/steps/3", true},
		{"/workflows/refund/asIs", "/workflows/refund", true},
		{"/workflows/refund/steps/2", "", false},
		{"/workflows/refund", "/workflows/refund", true},
		{"/workflows/plain/steps/0", "/workflows/plain/steps/0", true},
		{"/paths/~1refunds/post", "/paths/~1refunds/post", true},
		{"", "", true},
	} {
		if out, ok := AsIsPointer(root, c.in); out != c.out || ok != c.ok {
			t.Errorf("AsIsPointer(%q) = %q, %v; want %q, %v", c.in, out, ok, c.out, c.ok)
		}
	}
	for in, want := range map[string]string{
		"/workflows/refund/steps/3": "/workflows/refund/asIs/steps/3",
		"/workflows/refund":         "/workflows/refund",
		"/workflows/plain/steps/0":  "/workflows/plain/steps/0",
		"/entities/Refund":          "/entities/Refund",
	} {
		if got := MergedPointer(root, in); got != want {
			t.Errorf("MergedPointer(%q) = %q, want %q", in, got, want)
		}
	}
	if !InAsIs(root, "#/workflows/refund/asIs/steps/0") || InAsIs(root, "/workflows/refund/steps/0") || InAsIs(root, "/workflows/plain/asIs") {
		t.Error("InAsIs does not tell the as-is's pointers from the rest")
	}
}
