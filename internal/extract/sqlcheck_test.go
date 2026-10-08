package extract

import "testing"

// TestTranslateCheck covers the forms PostgreSQL prints a check in, and the
// ones the expression subset cannot say.
func TestTranslateCheck(t *testing.T) {
	fields := map[string]*field{
		"loaned_on":   {name: "loanedOn", kind: "date"},
		"due_on":      {name: "dueOn", kind: "date"},
		"returned_on": {name: "returnedOn", kind: "date", nullable: true},
		"copies":      {name: "copies", kind: "int", bits: 32},
		"lent":        {name: "lent", kind: "int", bits: 32},
		"fee":         {name: "fee", kind: "decimal", scale: 2},
		"title":       {name: "title", kind: "string"},
		"active":      {name: "active", kind: "bool"},
	}
	for _, c := range []struct{ def, want string }{
		{"CHECK ((due_on > loaned_on))", "dueOn > loanedOn"},
		{"CHECK (((returned_on IS NULL) OR (returned_on >= loaned_on)))", "returnedOn == null || returnedOn >= loanedOn"},
		{"CHECK ((char_length((title)::text) > 0))", "size(title) > 0"},
		{"CHECK ((fee >= (0)::numeric))", `fee >= decimal("0", 0)`},
		{"CHECK ((fee <= 99.50))", `fee <= decimal("99.50", 2)`},
		{"CHECK (((copies - lent) >= 0)) NOT VALID", "copies - lent >= 0"},
		{"CHECK ((copies - (lent - 1) > 0))", "copies - (lent - 1) > 0"},
		{"CHECK ((NOT (active AND (copies = 0))))", "!(active && copies == 0)"},
		{"CHECK (((title)::text = ANY ((ARRAY['a'::character varying, 'b'::character varying])::text[])))", `title == "a" || title == "b"`},
		{"CHECK ((title <> 'it''s'::text))", `title != "it's"`},
		{"CHECK ((due_on = (loaned_on + 14)))", `dueOn == loanedOn + duration("P14D")`},
		{"CHECK ((due_on > ((loaned_on + 14) - 2)))", `dueOn > loanedOn + duration("P14D") - duration("P2D")`},
		{"CHECK ((due_on <= ('2026-01-01'::date + 30)))", `dueOn <= date("2026-01-01") + duration("P30D")`},
	} {
		got, reason := translateCheck(c.def, fields)
		if reason != "" || got != c.want {
			t.Errorf("%s\n got %q (%s)\nwant %q", c.def, got, reason, c.want)
		}
	}
	for _, def := range []string{
		"CHECK ((copies = (loaned_on + 14)))",
		"CHECK ((due_on = (loaned_on + 1.5)))",
		"CHECK ((title ~ '^[A-Z]'::text))",
		"CHECK ((upper(title) = title))",
		"CHECK ((missing > 0))",
	} {
		if got, reason := translateCheck(def, fields); reason == "" {
			t.Errorf("%s was translated to %q; it should be refused", def, got)
		}
	}
}
