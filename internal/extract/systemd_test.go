package extract

import "testing"

// TestTimeSpan checks the systemd time spans read as a duration and the
// ones refused: a span below a second or in weeks has no duration.
func TestTimeSpan(t *testing.T) {
	for _, c := range []struct {
		in   string
		want string
		ok   bool
	}{
		{"30", "PT30S", true},
		{"5s", "PT5S", true},
		{"10min", "PT10M", true},
		{"1min 30s", "PT1M30S", true},
		{"2h", "PT2H", true},
		{"1d 2h", "P1DT2H", true},
		{"1day", "P1D", true},
		{"500ms", "", false},
		{"1w", "", false},
		{"infinity", "", false},
		{"5 apples", "", false},
	} {
		total, ok := timeSpan(c.in)
		if ok != c.ok || ok && isoDuration(total) != c.want {
			t.Errorf("timeSpan(%q) = %d, %v (%s); want %s, %v", c.in, total, ok, isoDuration(total), c.want, c.ok)
		}
	}
}

// TestUnitName checks the camelCase names of units and variables.
func TestUnitName(t *testing.T) {
	for in, want := range map[string]string{
		"release-watcher": "releaseWatcher",
		"REGISTRY_TOKEN":  "registryToken",
		"log_level":       "logLevel",
		"app@1":           "app1",
		"9lives":          "unit9lives",
	} {
		if got := unitName(in); got != want {
			t.Errorf("unitName(%q) = %q, want %q", in, got, want)
		}
	}
}
