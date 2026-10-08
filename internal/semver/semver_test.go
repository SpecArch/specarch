package semver

import "testing"

// The precedence example of SemVer 2.0.0, rule 11, in ascending order.
var ordered = []string{
	"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
	"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "2.0.0", "2.1.0", "2.1.1",
}

func TestPrecedence(t *testing.T) {
	for i := range ordered {
		for j := range ordered {
			a, _ := Parse(ordered[i])
			b, _ := Parse(ordered[j])
			want := 0
			switch {
			case i < j:
				want = -1
			case i > j:
				want = 1
			}
			if got := Compare(a, b); got != want {
				t.Errorf("Compare(%s, %s) = %d, want %d", ordered[i], ordered[j], got, want)
			}
		}
	}
}

func TestNeeded(t *testing.T) {
	for _, c := range []struct {
		prev   string
		impact int
		want   int
		next   string
	}{
		{"1.4.2", Major, Major, "2.0.0"},
		{"1.4.2", Minor, Minor, "1.5.0"},
		{"1.4.2", Patch, Patch, "1.4.3"},
		{"0.2.0", Major, Minor, "0.3.0"},
		{"0.2.0", Minor, Patch, "0.2.1"},
		{"0.2.0", None, None, ""},
	} {
		prev, _ := Parse(c.prev)
		got := Needed(prev, c.impact)
		if got != c.want {
			t.Errorf("Needed(%s, %d) = %d, want %d", c.prev, c.impact, got, c.want)
		}
		if got != None {
			if next := Next(prev, got); next != c.next {
				t.Errorf("Next(%s, %d) = %s, want %s", c.prev, got, next, c.next)
			}
		}
	}
}
