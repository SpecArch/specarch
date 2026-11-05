package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

// TestAskIntent checks the answers init reads at a terminal (ADR-244): one
// problem, then goals and out-of-scope entries until an empty answer, an
// empty answer skipping a question, and an input that ends early keeping
// what was answered.
func TestAskIntent(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  intent
	}{
		{"every question answered", "P\nG1\nG2\n\nO\n\n",
			intent{problem: "P", goals: []string{"G1", "G2"}, outOfScope: []string{"O"}}},
		{"each question skipped", "\n\n\n", intent{}},
		{"the problem skipped", "\nG1\n\n\n", intent{goals: []string{"G1"}}},
		{"answers trimmed", "  P  \n\tG1\t\n\n", intent{problem: "P", goals: []string{"G1"}}},
		{"input ends inside the goals", "P\nG1", intent{problem: "P", goals: []string{"G1"}}},
		{"input ends after the problem", "P\n", intent{problem: "P"}},
		{"no input at all", "", intent{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stderr bytes.Buffer
			got := askIntent(strings.NewReader(c.input), &stderr)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("askIntent(%q) = %+v, want %+v", c.input, got, c.want)
			}
			for _, q := range []string{"What problem does the system solve", "What outcome counts as success?", "What will the system not do"} {
				if !strings.Contains(stderr.String(), q) {
					t.Errorf("askIntent(%q) did not ask %q on standard error; it wrote:\n%s", c.input, q, stderr.String())
				}
			}
		})
	}
}
