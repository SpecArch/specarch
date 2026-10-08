package extract

import (
	"slices"
	"testing"
)

// TestPathParameters checks which route paths an OpenAPI path template
// holds, and the parameters read from those it does.
func TestPathParameters(t *testing.T) {
	held := map[string][]string{
		"/":                             nil,
		"/members":                      nil,
		"/members/":                     nil,
		"/loans/{loanId}/return":        {"loanId"},
		"/branches/{branch}/copies/{n}": {"branch", "n"},
		"/v1.2/files~old":               nil,
	}
	for path, want := range held {
		got, reason := pathParameters(path)
		if reason != "" {
			t.Errorf("%s: not held (%s), want held", path, reason)
		} else if !slices.Equal(got, want) {
			t.Errorf("%s: parameters %v, want %v", path, got, want)
		}
	}
	for _, path := range []string{
		"/files/{path...}",
		"/files/*",
		"/members//loans",
		"/members/{id}/{id}",
		"/members/id-{id}",
		"/members/:id",
	} {
		if _, reason := pathParameters(path); reason == "" {
			t.Errorf("%s: held, want a reason it is not", path)
		}
	}
}
