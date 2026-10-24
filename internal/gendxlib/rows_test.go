package gendxlib

import (
	"strings"
	"testing"
)

// TestRowsCondition writes rules in the SQL every engine dxlib serves
// reads, each caller property and literal a parameter, and refuses, with
// the reason, a rule outside it (ADR-180).
func TestRowsCondition(t *testing.T) {
	for _, c := range []struct {
		rule, sql, why string
		callers        []string
		literals       [][2]string
	}{
		{rule: "organisationId == caller.organisationId", sql: "organisation_id = :caller_organisation_id", callers: []string{"organisationId"}},
		{rule: "caller.memberId == memberId && !archived", sql: "(:caller_member_id = member_id AND NOT (archived = :rows_loan_1))", callers: []string{"memberId"}, literals: [][2]string{{"rows_loan_1", "true"}}},
		{rule: "memberId == null || pages >= 10", sql: "(member_id IS NULL OR pages >= :rows_loan_1)", literals: [][2]string{{"rows_loan_1", "10"}}},
		{rule: `title != "draft"`, sql: "title <> :rows_loan_1", literals: [][2]string{{"rows_loan_1", `"draft"`}}},
		{rule: "true", sql: "1 = 1"},
		{rule: "size(title) > 3", why: "the function size"},
		{rule: "pages + 1 > 3", why: "arithmetic"},
		{rule: "caller.memberId == caller.organisationId", why: "compares no field of Loan"},
		{rule: "secret == caller.memberId", why: "encrypted at rest"},
	} {
		g := &gen{spec: map[string]any{
			"caller": map[string]any{"properties": map[string]any{
				"memberId":       map[string]any{"type": []any{"string", "null"}},
				"organisationId": map[string]any{"type": "string"},
			}},
			"roles": map[string]any{
				"member": map[string]any{"rows": map[string]any{"Loan": c.rule}},
				"admin":  map[string]any{},
			},
		}, entities: map[string]any{"Loan": map[string]any{"properties": map[string]any{
			"memberId":       map[string]any{"type": "string"},
			"organisationId": map[string]any{"type": "string"},
			"archived":       map[string]any{"type": "boolean"},
			"pages":          map[string]any{"type": "integer"},
			"title":          map[string]any{"type": "string"},
			"secret":         map[string]any{"type": "string", "atRest": "encrypted"},
		}}}}
		got := g.rowsCondition("Loan")
		if c.why != "" {
			if !strings.Contains(got.why, c.why) || len(got.roles) != 0 {
				t.Errorf("%s: want it refused for %q, got %+v", c.rule, c.why, got)
			}
			continue
		}
		if got.why != "" || len(got.roles) != 1 {
			t.Errorf("%s: want one rule written, got %+v", c.rule, got)
			continue
		}
		r := got.roles[0]
		if r.sql != c.sql || strings.Join(r.callers, ",") != strings.Join(c.callers, ",") || len(r.literals) != len(c.literals) {
			t.Errorf("%s: got %q with %v and %v, want %q with %v and %v", c.rule, r.sql, r.callers, r.literals, c.sql, c.callers, c.literals)
			continue
		}
		for i := range c.literals {
			if r.literals[i] != c.literals[i] {
				t.Errorf("%s: literal %d is %v, want %v", c.rule, i, r.literals[i], c.literals[i])
			}
		}
		if strings.Join(got.whole, ",") != "admin" {
			t.Errorf("%s: the roles with no rule are %v, want admin", c.rule, got.whole)
		}
	}
}

// TestGrouped puts a condition in parentheses once.
func TestGrouped(t *testing.T) {
	for in, want := range map[string]string{
		"a = 1":              "(a = 1)",
		"(a = 1 AND b = 2)":  "(a = 1 AND b = 2)",
		"(a = 1) OR (b = 2)": "((a = 1) OR (b = 2))",
	} {
		if got := grouped(in); got != want {
			t.Errorf("grouped(%q) = %q, want %q", in, got, want)
		}
	}
}
