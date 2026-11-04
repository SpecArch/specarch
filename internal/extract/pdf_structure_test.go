package extract

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

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

// TestPDFTableOverPages reads a table that runs onto a second page, a
// page holding nothing but the table's rows: it is one table, and the
// second page is not said to hold no text.
func TestPDFTableOverPages(t *testing.T) {
	data, err := os.ReadFile("testdata/long-table.pdf")
	if err != nil {
		t.Fatal(err)
	}
	in, err := readPDF("long-table.pdf", data, 0)
	if err != nil {
		t.Fatal(err)
	}
	var tables [][][]string
	for _, b := range in.blocks {
		switch b.kind {
		case blockTable:
			tables = append(tables, b.rows)
		case blockNotHeld:
			t.Errorf("a line not held at %s: %s", b.place, b.text)
		}
	}
	if len(tables) != 1 || len(tables[0]) != 71 {
		t.Fatalf("got %d tables, want one of 71 rows", len(tables))
	}
	if got := tables[0][70]; !reflect.DeepEqual(got, []string{"field 70", "text"}) {
		t.Errorf("the last row is %q, want [field 70 text]", got)
	}
}

// pdfStructReader builds a PDF of the given objects, the first the
// catalog, and opens it.
func pdfStructReader(t *testing.T, objects []string) *pdf.Reader {
	t.Helper()
	var b strings.Builder
	b.WriteString("%PDF-1.7\n")
	var offsets []int
	for i, o := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, o := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	data := []byte(b.String())
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// pdfStructOverBudget reads the tables of a crafted tree and wants none,
// over the budget, within a few seconds.
func pdfStructOverBudget(t *testing.T, objects []string) {
	t.Helper()
	r := pdfStructReader(t, objects)
	start := time.Now()
	tables, over := pdfStructTables(r)
	if !over || tables != nil {
		t.Errorf("got %d tables, over %v; want none, over the budget", len(tables), over)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("the walk took %v", d)
	}
}

// TestPDFStructBudget walks a crafted tree whose element lists itself
// twice in K: the walk branches at every level, and only the budget of
// nodes keeps it from running for hours. The tree gives no table, and
// says it was over the budget.
func TestPDFStructBudget(t *testing.T) {
	pdfStructOverBudget(t, []string{
		"<< /Type /Catalog /Pages 2 0 R /StructTreeRoot 3 0 R >>",
		"<< /Type /Pages /Kids [] /Count 0 >>",
		"<< /Type /StructTreeRoot /K 4 0 R >>",
		"<< /S /Table /K [5 0 R 5 0 R] >>",
		"<< /S /TBody /K [5 0 R 5 0 R] >>",
	})
}

// TestPDFStructBudgetWide walks crafted trees whose element lists itself
// 2000 times in K: each visit would resolve all 2000 entries, so the
// budget counts every entry of K and not only the nodes visited. One
// tree is a Div among the document's elements, one a table whose row
// lists a cell 2000 times and whose cell lists itself 2000 times.
func TestPDFStructBudgetWide(t *testing.T) {
	wide := func(ref string) string {
		return "[" + strings.TrimSpace(strings.Repeat(ref+" ", 2000)) + "]"
	}
	t.Run("Div", func(t *testing.T) {
		pdfStructOverBudget(t, []string{
			"<< /Type /Catalog /Pages 2 0 R /StructTreeRoot 3 0 R >>",
			"<< /Type /Pages /Kids [] /Count 0 >>",
			"<< /Type /StructTreeRoot /K 4 0 R >>",
			"<< /S /Div /K " + wide("4 0 R") + " >>",
		})
	})
	t.Run("TableCell", func(t *testing.T) {
		pdfStructOverBudget(t, []string{
			"<< /Type /Catalog /Pages 2 0 R /StructTreeRoot 3 0 R >>",
			"<< /Type /Pages /Kids [] /Count 0 >>",
			"<< /Type /StructTreeRoot /K 4 0 R >>",
			"<< /S /Table /K 5 0 R >>",
			"<< /S /TR /K " + wide("6 0 R") + " >>",
			"<< /S /TD /K " + wide("6 0 R") + " >>",
		})
	})
}
