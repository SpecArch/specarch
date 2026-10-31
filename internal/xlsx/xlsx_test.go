package xlsx

import (
	"archive/zip"
	"bytes"
	"reflect"
	"testing"
)

func TestRefs(t *testing.T) {
	for _, c := range []struct {
		row, col int
		ref      string
	}{{1, 1, "A1"}, {4, 2, "B4"}, {10, 26, "Z10"}, {3, 27, "AA3"}, {7, 702, "ZZ7"}, {9, 703, "AAA9"}} {
		if got := Ref(c.row, c.col); got != c.ref {
			t.Errorf("Ref(%d, %d) = %s, want %s", c.row, c.col, got, c.ref)
		}
		r, col, err := ParseRef("$" + c.ref)
		if err != nil || r != c.row || col != c.col {
			t.Errorf("ParseRef(%s) = %d, %d, %v", c.ref, r, col, err)
		}
	}
	if _, _, err := ParseRef("b4"); err == nil {
		t.Error("ParseRef(b4) gave no error")
	}
	if got := SheetRef("Account fields"); got != "'Account fields'" {
		t.Errorf("SheetRef = %s", got)
	}
	if got := SheetRef("Bob's"); got != "'Bob''s'" {
		t.Errorf("SheetRef = %s", got)
	}
	if got := SheetRef("Register_2"); got != "Register_2" {
		t.Errorf("SheetRef = %s", got)
	}
}

func TestSerialDate(t *testing.T) {
	if got := serialDate(46326, false); got != "2026-10-31" {
		t.Errorf("46326 = %s", got)
	}
	if got := serialDate(46326.5, false); got != "2026-10-31T12:00:00" {
		t.Errorf("46326.5 = %s", got)
	}
	if got := serialDate(0, true); got != "1904-01-01" {
		t.Errorf("0 in 1904 = %s", got)
	}
}

func TestDateFormat(t *testing.T) {
	for code, want := range map[string]bool{"yyyy-mm-dd": true, "d mmm": true, "0.00": false, "#,##0": false, `"day" 0`: false, "[Red]0": false, "General": false, "mm:ss": true} {
		if got := isDateFormat(code); got != want {
			t.Errorf("isDateFormat(%q) = %v, want %v", code, got, want)
		}
	}
}

func TestRegions(t *testing.T) {
	s := Sheet{Name: "S", Cells: []Cell{
		{Ref: "A1", Row: 1, Column: 1, Value: "Field"}, {Ref: "B1", Row: 1, Column: 2, Value: "Type"},
		{Ref: "A2", Row: 2, Column: 1, Value: "name"}, {Ref: "B2", Row: 2, Column: 2, Value: "string"},
		{Ref: "D1", Row: 1, Column: 4, Value: "note"},
		{Ref: "A4", Row: 4, Column: 1, Value: "after a blank row"}, {Ref: "C4", Row: 4, Column: 3, Value: " "},
	}}
	var got []string
	for _, r := range s.Regions() {
		got = append(got, r.Range())
	}
	want := []string{"S!A1:B2", "S!D1:D2", "S!A4:A4"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Regions = %v, want %v", got, want)
	}
	if rows := s.Regions()[0].Rows; !reflect.DeepEqual(rows, [][]string{{"Field", "Type"}, {"name", "string"}}) {
		t.Errorf("rows = %v", rows)
	}
}

func TestRead(t *testing.T) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	put := func(name, body string) {
		w, _ := z.Create(name)
		w.Write([]byte(body))
	}
	const ns = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
	put("xl/workbook.xml", `<workbook `+ns+`><workbookPr date1904="0"/><sheets><sheet name="One" sheetId="1" r:id="rId1"/><sheet name="Hidden" sheetId="2" state="hidden" r:id="rId2"/><sheet name="Chart" sheetId="3" r:id="rId3"/></sheets></workbook>`)
	put("xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="/xl/worksheets/sheet2.xml"/><Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chartsheet" Target="chartsheets/sheet1.xml"/></Relationships>`)
	put("xl/sharedStrings.xml", `<sst `+ns+`><si><t>shared</t></si><si><r><t>ri</t></r><r><t>ch</t></r></si></sst>`)
	put("xl/styles.xml", `<styleSheet `+ns+`><numFmts><numFmt numFmtId="164" formatCode="dd/mm/yyyy"/></numFmts><cellXfs><xf numFmtId="0"/><xf numFmtId="164"/><xf numFmtId="14"/></cellXfs></styleSheet>`)
	put("xl/worksheets/sheet1.xml", `<worksheet `+ns+`><sheetData><row r="1"><c r="B1" t="s"><v>1</v></c><c r="A1" t="s"><v>0</v></c><c r="C1"><v>42</v></c><c r="D1" s="1"><v>46326</v></c><c r="E1" s="2"><v>46327</v></c><c r="F1" t="b"><v>1</v></c><c r="G1" t="e"><v>#DIV/0!</v></c><c r="H1" t="str"><f>A1</f><v>shared</v></c><c r="I1" t="inlineStr"><is><t>inline</t></is></c><c r="J1"/></row></sheetData></worksheet>`)
	put("xl/worksheets/sheet2.xml", `<worksheet `+ns+`><sheetData/></worksheet>`)
	put("docProps/core.xml", `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title> Register </dc:title></cp:coreProperties>`)
	z.Close()
	wb, err := Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if wb.Title != "Register" || len(wb.Sheets) != 2 || !wb.Sheets[1].Hidden || !reflect.DeepEqual(wb.NotRead, []string{"Chart"}) {
		t.Fatalf("workbook = %+v", wb)
	}
	var got []string
	for _, c := range wb.Sheets[0].Cells {
		got = append(got, c.Ref+"="+string(c.Kind)+":"+c.Value)
	}
	want := []string{"A1=text:shared", "B1=text:rich", "C1=number:42", "D1=date:2026-10-31", "E1=date:2026-11-01", "F1=boolean:true", "G1=error:#DIV/0!", "H1=text:shared", "I1=text:inline"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cells = %v\nwant    %v", got, want)
	}
	if !wb.Sheets[0].Cells[7].Formula {
		t.Error("H1 is not marked a formula")
	}
	if _, err := Read(bytes.NewReader([]byte("not a zip")), 9); err == nil {
		t.Error("a file that is not a zip gave no error")
	}
}
