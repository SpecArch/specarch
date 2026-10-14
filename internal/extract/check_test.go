package extract

import "testing"

// TestPostgresType checks that only a type's spelling is made the
// catalogue's: a width or a kind that differs still differs (ADR-124).
func TestPostgresType(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"VARCHAR(10)", "character varying(10)"},
		{"character varying(10)", "character varying(10)"},
		{"CHAR(10)", "character(10)"},
		{"NUMERIC(20,0)", "numeric(20,0)"},
		{"DECIMAL(6, 2)", "numeric(6,2)"},
		{"INT", "integer"},
		{"BIGINT", "bigint"},
		{"TIMESTAMP WITH TIME ZONE", "timestamp with time zone"},
		{"timestamptz", "timestamp with time zone"},
		{"TIME", "time without time zone"},
		{"DOUBLE PRECISION", "double precision"},
		{"VARCHAR(8)[]", "character varying(8)[]"},
		{"JSONB", "jsonb"},
	} {
		if got := postgresType(c.in); got != c.want {
			t.Errorf("postgresType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if postgresType("VARCHAR(10)") == postgresType("character(10)") {
		t.Error("a VARCHAR and a CHAR of one width compare equal")
	}
	if postgresType("VARCHAR(10)") == postgresType("VARCHAR(12)") {
		t.Error("two widths compare equal")
	}
}
