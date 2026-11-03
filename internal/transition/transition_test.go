package transition

import (
	"reflect"
	"testing"
)

// TestSelect checks that the to-be loses asIs, that --as-is puts the
// as-is in the workflow's place, that a workflow with no as-is is the
// same either way, and that the specification given is not changed.
func TestSelect(t *testing.T) {
	asIs := map[string]any{"trigger": "askRefund", "steps": []any{"check"}}
	spec := map[string]any{
		"info": map[string]any{"title": "Case"},
		"workflows": map[string]any{
			"refund": map[string]any{"trigger": "askRefund", "steps": []any{"route"}, "asIs": asIs},
			"plain":  map[string]any{"trigger": "askPlain"},
		},
	}
	toBe := Select(spec, false).(map[string]any)
	want := map[string]any{"trigger": "askRefund", "steps": []any{"route"}}
	if got := toBe["workflows"].(map[string]any)["refund"]; !reflect.DeepEqual(got, want) {
		t.Errorf("the to-be is %v, want %v", got, want)
	}
	old := Select(spec, true).(map[string]any)
	if got := old["workflows"].(map[string]any)["refund"]; !reflect.DeepEqual(got, asIs) {
		t.Errorf("the as-is is %v, want %v", got, asIs)
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
