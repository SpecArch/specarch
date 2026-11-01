package extract

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/vocabulary"
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

func TestReadTabFileRows(t *testing.T) {
	// An empty line is no record and counts as a row; a quoted field that
	// holds a line break is one row; the header is refused when it holds
	// no cell with a value.
	v := vocabulary.English()
	f, err := readTabFile("loans.csv", "loans.csv", []byte("a,b\n1,x\n\n2,\"y\nz\"\n3,w\n"), v)
	if err != nil {
		t.Fatal(err)
	}
	tb := f.Tables[0]
	var rows []int
	for _, row := range tb.Rows {
		rows = append(rows, row[0].Row)
	}
	if got := fmt.Sprint(rows); got != "[2 4 5]" {
		t.Errorf("rows %s, want [2 4 5]: the empty line 3 counts as a row, and the field of two lines is one row", got)
	}
	if tb.Rows[2][1].Line != 6 || tb.Rows[2][1].Column != 3 {
		t.Errorf("w at line %d column %d, want 6 and 3", tb.Rows[2][1].Line, tb.Rows[2][1].Column)
	}
	if tb.Range() != "A1:B5" {
		t.Errorf("range %s, want A1:B5", tb.Range())
	}
	if _, err := readTabFile("loans.csv", "loans.csv", []byte(",,\n1,2,3\n"), v); err == nil || !strings.Contains(err.Error(), "its first row, line 1, holds no cell with a value") {
		t.Errorf("empty header: err = %v", err)
	}
	if _, err := readTabFile("loans.csv", "loans.csv", []byte("\n\n,\n1,2\n"), v); err == nil || !strings.Contains(err.Error(), "its first row, line 3,") {
		t.Errorf("empty header after empty lines: err = %v", err)
	}
}

func TestEnumValues(t *testing.T) {
	for _, c := range []struct {
		typ, format, values, want, wrong string
	}{
		{"integer", "int32", "1|2|3", "!!int 1, !!int 2, !!int 3", ""},
		{"integer", "int32", "1|x|4294967301", "!!int 1", "x, 4294967301"},
		{"boolean", "", "Yes|true", "!!bool true", "Yes"},
		{"string", "int64", "4294967301|1.5", "!!str 4294967301", "1.5"},
		{"string", "", "open|closed", "!!str open, !!str closed", ""},
	} {
		f := mapping("type", c.typ)
		if c.format != "" {
			set(f, "format", c.format)
		}
		nodes, wrong := enumValues(strings.Split(c.values, "|"), f)
		var got []string
		for _, n := range nodes {
			got = append(got, n.Tag+" "+n.Value)
		}
		if g, w := strings.Join(got, ", "), strings.Join(wrong, ", "); g != c.want || w != c.wrong {
			t.Errorf("%s %s %s: nodes %q wrong %q, want %q and %q", c.typ, c.format, c.values, g, w, c.want, c.wrong)
		}
	}
}

func TestReadWorkbookHiddenSheet(t *testing.T) {
	// A hidden sheet is read as the others are, and said in a line, not
	// set aside.
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	put := func(name, body string) {
		w, _ := z.Create(name)
		w.Write([]byte(body))
	}
	const ns = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
	put("xl/workbook.xml", `<workbook `+ns+`><sheets><sheet name="Loans" sheetId="1" r:id="rId1"/><sheet name="Old" sheetId="2" state="hidden" r:id="rId2"/></sheets></workbook>`)
	put("xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/></Relationships>`)
	put("xl/worksheets/sheet1.xml", `<worksheet `+ns+`><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>Loan ID</t></is></c></row><row r="2"><c r="A2"><v>1</v></c></row></sheetData></worksheet>`)
	put("xl/worksheets/sheet2.xml", `<worksheet `+ns+`><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>Loan ID</t></is></c></row><row r="2"><c r="A2"><v>2</v></c></row></sheetData></worksheet>`)
	z.Close()
	f, err := readTabFile("loans.xlsx", "loans.xlsx", buf.Bytes(), vocabulary.English())
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Tables) != 2 || len(f.Aside) != 0 {
		t.Fatalf("tables %d, aside %d; want both sheets read and nothing set aside", len(f.Tables), len(f.Aside))
	}
	if want := "sheet Old of loans.xlsx is hidden in the workbook; it is read as the others are"; len(f.Says) != 2 || f.Says[0] != want {
		t.Errorf("says %q, want %q first", f.Says, want)
	}
}
