package problems

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestRewriteKeepsEveryOtherLine(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		above map[int][]string
		want  string
	}{
		{
			name:  "marks above entries, old marks out, comments kept",
			in:    "# top\nitems:\n  # specarch-problem: error: old [x]\n  a: 1   # why\n\n  b: 2\n",
			above: map[int][]string{4: {"M1"}, 6: {"M2", "M3"}},
			want:  "# top\nitems:\n  M1\n  a: 1   # why\n\n  M2\n  M3\n  b: 2\n",
		},
		{
			name:  "the file as a whole, below its comments",
			in:    "# yaml-language-server: $schema=x\n# kept by hand\nitems: [\n",
			above: map[int][]string{0: {"M"}},
			want:  "# yaml-language-server: $schema=x\n# kept by hand\nM\nitems: [\n",
		},
		{
			name:  "no newline at the end",
			in:    "a: 1\nb: 2",
			above: map[int][]string{2: {"M"}},
			want:  "a: 1\nM\nb: 2",
		},
		{
			name:  "CRLF line ends",
			in:    "a: 1\r\n  # specarch-problem: warning: old [x]\r\nb: 2\r\n",
			above: map[int][]string{3: {"M"}},
			want:  "a: 1\r\nM\r\nb: 2\r\n",
		},
		{
			name:  "a file of comments only",
			in:    "# nothing yet",
			above: map[int][]string{0: {"M"}},
			want:  "# nothing yet\nM\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := rewrite(strings.SplitAfter(tt.in, "\n"), tt.above)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRewriteMovesLines(t *testing.T) {
	_, moved := rewrite(strings.SplitAfter("a: 1\n# specarch-problem: x\nb: 2\nc: 3\n", "\n"), map[int][]string{4: {"M1", "M2"}})
	want := map[int]int{1: 1, 3: 2, 4: 5}
	for old, now := range want {
		if moved[old] != now {
			t.Errorf("line %d moved to %d, want %d", old, moved[old], now)
		}
	}
}

func TestDashLine(t *testing.T) {
	lines := strings.SplitAfter("list:\n  -\n    name: a\n  - name: b\n", "\n")
	for _, tt := range []struct{ line, column, want int }{{3, 5, 2}, {4, 5, 4}} {
		item := &yaml.Node{Line: tt.line, Column: tt.column}
		if got := dashLine(lines, item); got != tt.want {
			t.Errorf("item at %d:%d: dash on line %d, want %d", tt.line, tt.column, got, tt.want)
		}
	}
}
