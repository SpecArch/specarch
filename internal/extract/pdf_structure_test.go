package extract

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"github.com/ledongthuc/pdf"
)

// TestPDFTableRowsSpans lays spanned cells on a grid: a header spanning
// two columns and a cell spanning two rows leave the places they cover
// empty, so every other cell keeps its column.
func TestPDFTableRowsSpans(t *testing.T) {
	table := pdfStructTable{rows: [][]pdfStructCell{
		{{rowSpan: 1, colSpan: 2}, {rowSpan: 1, colSpan: 1}},
		{{rowSpan: 2, colSpan: 1}, {rowSpan: 1, colSpan: 1}, {rowSpan: 1, colSpan: 1}},
		{{rowSpan: 1, colSpan: 1}, {rowSpan: 3, colSpan: 1}},
	}}
	names := [][]string{{"Name", "Note"}, {"a", "b", "c"}, {"d", "e"}}
	got := pdfTableRows(table, func(r, c int) string { return names[r][c] })
	want := [][]string{
		{"Name", "", "Note"},
		{"a", "b", "c"},
		{"", "d", "e"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestPDFTaggedTable reads the table of a tagged PDF as its rows and
// cells, and the same file without its structure tree as lines.
func TestPDFTaggedTable(t *testing.T) {
	for _, c := range []struct {
		name  string
		table [][]string
	}{
		{"extract-documents-pdf-tagged", [][]string{
			{"Field", "Type", "Description"},
			{"loan number", "text", "The number the desk gives the loan"},
			{"member", "text", "The member who borrows the item"},
			{"due date", "date", "The day the item must be returned"},
		}},
		{"extract-documents-pdf-untagged", nil},
	} {
		data, err := os.ReadFile("../../spec/tests/" + c.name + "/docs/loan-register.pdf")
		if err != nil {
			t.Fatal(err)
		}
		in, err := readPDF("loan-register.pdf", data, 0)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		var tables [][][]string
		var texts []string
		for _, b := range in.blocks {
			switch b.kind {
			case blockTable:
				tables = append(tables, b.rows)
			case blockParagraph, blockHeading:
				texts = append(texts, b.text)
			case blockNotHeld:
				t.Errorf("%s: a line not held: %s", c.name, b.text)
			}
		}
		if c.table == nil {
			if len(tables) != 0 {
				t.Errorf("%s: got tables %q, want none", c.name, tables)
			}
			continue
		}
		if len(tables) != 1 || !reflect.DeepEqual(tables[0], c.table) {
			t.Errorf("%s: got tables %q, want %q", c.name, tables, c.table)
		}
		// The table's text is not read again as lines, and it stands
		// between the heading above it and the one below.
		want := []string{"1 Loan register", "The library keeps one record per loan.", "2 Loan fields", "3 Returns", "The system must record the return date of every loan."}
		if !reflect.DeepEqual(texts, want) {
			t.Errorf("%s: got text %q, want %q", c.name, texts, want)
		}
		order := []blockKind{}
		for _, b := range in.blocks {
			if b.kind == blockHeading || b.kind == blockTable {
				order = append(order, b.kind)
			}
		}
		if !reflect.DeepEqual(order, []blockKind{blockHeading, blockHeading, blockTable, blockHeading}) {
			t.Errorf("%s: headings and table in the order %v", c.name, order)
		}
	}
}

// TestPDFGlyphMarks checks that reading the content stream for marked
// content gives one MCID per glyph the library's Content gives, on the
// tagged fixtures, the one with ligatures set as actual text included.
func TestPDFGlyphMarks(t *testing.T) {
	for _, name := range []string{"extract-documents-pdf-tagged/docs/loan-register.pdf", "extract-documents-pdf/docs/lending-rules.pdf"} {
		data, err := os.ReadFile("../../spec/tests/" + name)
		if err != nil {
			t.Fatal(err)
		}
		r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		for n := 1; n <= r.NumPage(); n++ {
			p := r.Page(n)
			texts, marks := p.Content().Text, pdfGlyphMarks(p)
			if len(texts) != len(marks) {
				t.Errorf("%s page %d: %d glyphs, %d marks", name, n, len(texts), len(marks))
			}
			if len(texts) == 0 {
				continue
			}
			marked := 0
			for _, m := range marks {
				if m >= 0 {
					marked++
				}
			}
			if marked == 0 {
				t.Errorf("%s page %d: no glyph has an MCID", name, n)
			}
		}
	}
}
