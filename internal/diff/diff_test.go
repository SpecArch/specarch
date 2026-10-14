package diff

import "testing"

// TestIsMapKeyword tells a field's additionalProperties from a property of
// that name, including under a property named properties.
func TestIsMapKeyword(t *testing.T) {
	for _, c := range []struct {
		path []string
		want bool
	}{
		{[]string{"properties", "values", "additionalProperties"}, true},
		{[]string{"properties", "additionalProperties"}, false},
		{[]string{"properties", "properties", "additionalProperties"}, true},
		{[]string{"properties", "properties", "properties", "additionalProperties"}, false},
		{[]string{"additionalProperties"}, true},
		{[]string{"properties", "values"}, false},
	} {
		if got := isMapKeyword(c.path); got != c.want {
			t.Errorf("%v: got %v, want %v", c.path, got, c.want)
		}
	}
}
