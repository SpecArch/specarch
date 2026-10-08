package validate

import (
	"testing"

	"github.com/SpecArch/specarch/internal/source"
)

// TestContrast checks the contrast of colours whose ratio WCAG's own
// examples and common tools agree on.
func TestContrast(t *testing.T) {
	colour := func(text string) float64 {
		t.Helper()
		return relativeLuminance(source.Parse([]byte(text)).Root)
	}
	white := colour("{ colorSpace: srgb, components: [1, 1, 1] }")
	for _, c := range []struct{ value, want string }{
		{"{ colorSpace: srgb, components: [0, 0, 0] }", "21.00:1"},
		{"{ colorSpace: srgb, components: [0.4627, 0.4627, 0.4627] }", "4.54:1"}, // #767676
		{"{ colorSpace: srgb, components: [0.4667, 0.4667, 0.4667] }", "4.47:1"}, // #777777
		{"{ colorSpace: srgb, components: [1, 1, 1] }", "1.00:1"},
	} {
		if got := ratioText(contrast(colour(c.value), white)); got != c.want {
			t.Errorf("%s on white: got %s, want %s", c.value, got, c.want)
		}
	}
}
