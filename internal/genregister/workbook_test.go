package genregister

import (
	"bytes"
	"testing"

	"github.com/SpecArch/specarch/internal/xlsx"
)

// TestWorkbookRoundTrip writes a workbook and reads it back with the
// reader every register reader uses.
func TestWorkbookRoundTrip(t *testing.T) {
	var b bytes.Buffer
	err := WriteWorkbook(&b, "Loans register", []Sheet{
		{Name: "Loans", Rows: [][]Cell{
			{Text("Loan ID"), Text("Due date"), Text("Renewed"), Text("Fee"), Text("Note & more")},
			{{xlsx.Number, "1"}, {xlsx.Date, "2026-10-31"}, {xlsx.Boolean, "true"}, {xlsx.Number, "0.5"}, Text("a <b>")},
		}},
		{Name: "Instructions", Rows: [][]Cell{{Text("Field")}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	wb, err := xlsx.Read(bytes.NewReader(b.Bytes()), int64(b.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if wb.Title != "Loans register" || len(wb.Sheets) != 2 || wb.Sheets[0].Name != "Loans" || wb.Sheets[1].Name != "Instructions" {
		t.Fatalf("workbook = %+v", wb)
	}
	want := map[string][2]string{"A1": {"text", "Loan ID"}, "B1": {"text", "Due date"}, "C1": {"text", "Renewed"}, "D1": {"text", "Fee"}, "E1": {"text", "Note & more"}, "A2": {"number", "1"}, "B2": {"date", "2026-10-31"}, "C2": {"boolean", "true"}, "D2": {"number", "0.5"}, "E2": {"text", "a <b>"}}
	for _, c := range wb.Sheets[0].Cells {
		w, ok := want[c.Ref]
		if !ok || string(c.Kind) != w[0] || c.Value != w[1] {
			t.Errorf("cell %s = %s %q, want %v", c.Ref, c.Kind, c.Value, w)
		}
		delete(want, c.Ref)
	}
	for ref := range want {
		t.Errorf("cell %s was not read back", ref)
	}
}
