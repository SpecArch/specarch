package extract

import (
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/xlsx"
)

func TestDetectDialect(t *testing.T) {
	for _, c := range []struct {
		name, data, given, want, err string
	}{
		{"bom-crlf-semicolon", "\xEF\xBB\xBFa;b\r\n1;2\r\n", "", "UTF-8 with a byte order mark, CRLF line ends, delimiter ';', no quoted field", ""},
		{"comma-quoted", "a,b\n1,\"x, y\"\n2,z\n", "", "UTF-8, LF line ends, delimiter ',', fields quoted with \"", ""},
		{"quoted-line-break", "a,b\n1,\"x\ny\"\n2,z\n", "", "UTF-8, LF line ends, delimiter ',', fields quoted with \"", ""},
		{"tsv", "a\tb\n1\t2\n", "\t", "UTF-8, LF line ends, delimiter tab, no quoted field", ""},
		{"one-column", "a\n1\n2\n", "", "UTF-8, LF line ends, no delimiter, one column, no quoted field", ""},
		{"no-final-newline", "a;b\n1;2", "", "UTF-8, LF line ends, delimiter ';', no quoted field", ""},
		{"tie", "a,b;c\n1,2;3\n", "", "", "two delimiters fit equally: ',' and ';' each occur 1 times in the header record and in 2 of 2 records"},
		{"ragged", "a,b\n1,2,3\n4,5\n", "", "UTF-8, LF line ends, delimiter ',', no quoted field", ""},
		{"ragged-wins", "a;b,c\n1;2\n3;4\n", "", "UTF-8, LF line ends, delimiter ';', no quoted field", ""},
		{"utf16", "\xFF\xFEa\x00", "", "", "UTF-16 byte order mark"},
		{"latin1", "a,b\n1,caf\xE9\n", "", "", "byte 10 (0xE9) starts a sequence that is not UTF-8"},
		{"lone-cr", "a,b\r1,2\r", "", "", "CR line end with no LF"},
	} {
		var given rune
		if c.given != "" {
			given = rune(c.given[0])
		}
		d, err := detectDialect(c.name, []byte(c.data), given)
		switch {
		case c.err != "":
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("%s: err = %v, want %q", c.name, err, c.err)
			}
		case err != nil:
			t.Errorf("%s: %v", c.name, err)
		case d.String() != c.want:
			t.Errorf("%s: %s, want %s", c.name, d.String(), c.want)
		}
	}
}

func TestValueKind(t *testing.T) {
	for _, c := range []struct {
		value string
		kind  xlsx.Kind
		want  string
	}{
		{"42", xlsx.Text, "integer"}, {"-7", xlsx.Text, "integer"}, {"007", xlsx.Text, "text"}, {"0", xlsx.Text, "integer"},
		{"3.50", xlsx.Text, "decimal"}, {"1,000", xlsx.Text, "text"},
		{"2026-10-31", xlsx.Text, "date"}, {"2026-13-45", xlsx.Text, "text"}, {"2026-10-31T12:00:00Z", xlsx.Text, "date-time"},
		{"31/10/2026", xlsx.Text, "date of two readings"}, {"10.31.26", xlsx.Text, "date of two readings"},
		{"TRUE", xlsx.Text, "boolean"}, {"yes", xlsx.Text, "text"},
		{"a@example.org", xlsx.Text, "email"}, {"6b1f0a7e-2c4d-4c8e-9a0b-1d2e3f4a5b6c", xlsx.Text, "uuid"},
		{"1E+15", xlsx.Number, "integer"}, {"0.5", xlsx.Number, "decimal"}, {"46326", xlsx.Date, "date"},
		{"1", xlsx.Boolean, "boolean"}, {"#N/A", xlsx.Error, "error"},
	} {
		if got, _ := valueKind(tabCell{Value: c.value, Kind: c.kind}); got != c.want {
			t.Errorf("valueKind(%q, %s) = %s, want %s", c.value, c.kind, got, c.want)
		}
	}
}

func cellsOfValues(values ...string) []tabCellAt {
	tb := &tabTable{File: &tabFile{Name: "f.csv"}}
	var out []tabCellAt
	for i, v := range values {
		out = append(out, tabCellAt{tb, tabCell{Value: v, Kind: xlsx.Text, Row: i + 2, Col: 1}})
	}
	return out
}

func TestInfer(t *testing.T) {
	for _, c := range []struct {
		name    string
		values  []string
		typ     string
		format  string
		problem string
		enum    int
		words   string
	}{
		{"int32", []string{"1", "2", "3"}, "integer", "int32", "", 0, "integer"},
		{"int64", []string{"1", "3000000000"}, "string", "int64", "", 0, "integer beyond 32 bits, carried as text"},
		{"decimal", []string{"1234.5", "0.25", "7"}, "string", "decimal", "", 0, "decimal of precision 6 and scale 2"},
		{"date", []string{"2026-01-01", "2026-02-01"}, "string", "date", "", 0, "date"},
		{"mixed", []string{"1", "2", "n/a"}, "string", "", "mixed", 0, "text, since the values are mixed"},
		{"ambiguous", []string{"01/02/2026", "03/04/2026"}, "string", "", "ambiguous", 0, "text, since its dates read as day-month or as month-day"},
		{"empty", nil, "string", "", "empty", 0, "text, since the column holds no value"},
		{"enum", []string{"open", "closed", "open", "open", "closed"}, "string", "", "", 2, `one of "closed" and "open"`},
		{"not-enum", []string{"a", "b", "c", "d"}, "string", "", "", 0, "text"},
		{"boolean", []string{"true", "False"}, "boolean", "", "", 0, "boolean"},
	} {
		in := infer(cellsOfValues(c.values...))
		if in.typ != c.typ || in.format != c.format || in.problem != c.problem || len(in.enum) != c.enum || in.words != c.words {
			t.Errorf("%s: %+v", c.name, in)
		}
	}
}

func TestSplitValues(t *testing.T) {
	got := splitValues(" open | closed;archived, open\nlost ")
	if strings.Join(got, "/") != "open/closed/archived/lost" {
		t.Errorf("splitValues = %v", got)
	}
}
