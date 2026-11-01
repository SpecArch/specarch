package genregister

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/SpecArch/specarch/internal/xlsx"
)

// A workbook writer for the register templates: the smallest Office Open
// XML package (ISO/IEC 29500) a spreadsheet program opens, with text as
// inline strings, numbers, booleans and dates under the built-in date
// format, and no shared strings, formulas or styles beyond the date's.
// internal/xlsx reads back what it writes.

// Cell is one cell to write: its kind and its value.
type Cell struct {
	Kind  xlsx.Kind // Text, Number, Boolean or Date (an ISO 8601 date or date-time)
	Value string
}

// Text is a text cell.
func Text(s string) Cell { return Cell{xlsx.Text, s} }

// Sheet is one worksheet: its name and its rows, each a list of cells
// from column A; an empty row holds no cell.
type Sheet struct {
	Name string
	Rows [][]Cell
}

// WriteWorkbook writes the sheets as an .xlsx package, titled in its
// properties.
func WriteWorkbook(w io.Writer, title string, sheets []Sheet) error {
	z := zip.NewWriter(w)
	put := func(name, content string) error {
		f, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, content)
		return err
	}
	var types, rels, sheetsXML strings.Builder
	types.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
		`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>` +
		`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>`)
	rels.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`)
	for i, s := range sheets {
		n := i + 1
		types.WriteString(fmt.Sprintf(`<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, n))
		rels.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, n, n))
		sheetsXML.WriteString(fmt.Sprintf(`<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, escape(s.Name), n, n))
		if err := put(fmt.Sprintf("xl/worksheets/sheet%d.xml", n), sheetXML(s)); err != nil {
			return err
		}
	}
	types.WriteString(`</Types>`)
	rels.WriteString(`</Relationships>`)
	if err := put("[Content_Types].xml", types.String()); err != nil {
		return err
	}
	if err := put("_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>`+
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>`+
		`</Relationships>`); err != nil {
		return err
	}
	if err := put("docProps/core.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+
		`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/">`+
		`<dc:title>`+escape(title)+`</dc:title></cp:coreProperties>`); err != nil {
		return err
	}
	if err := put("xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`+
		`<sheets>`+sheetsXML.String()+`</sheets></workbook>`); err != nil {
		return err
	}
	if err := put("xl/_rels/workbook.xml.rels", rels.String()); err != nil {
		return err
	}
	// Style 0 is the default; style 1 is the built-in date format 14.
	if err := put("xl/styles.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+
		`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`+
		`<fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts>`+
		`<fills count="1"><fill><patternFill patternType="none"/></fill></fills>`+
		`<borders count="1"><border/></borders>`+
		`<cellXfs count="2"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/><xf numFmtId="14" fontId="0" fillId="0" borderId="0" applyNumberFormat="1"/></cellXfs>`+
		`</styleSheet>`); err != nil {
		return err
	}
	return z.Close()
}

// sheetXML is one worksheet part.
func sheetXML(s Sheet) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for r, row := range s.Rows {
		if len(row) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf(`<row r="%d">`, r+1))
		for c, cell := range row {
			ref := xlsx.Ref(r+1, c+1)
			switch cell.Kind {
			case xlsx.Number:
				b.WriteString(fmt.Sprintf(`<c r="%s"><v>%s</v></c>`, ref, escape(cell.Value)))
			case xlsx.Boolean:
				v := "0"
				if strings.EqualFold(cell.Value, "true") {
					v = "1"
				}
				b.WriteString(fmt.Sprintf(`<c r="%s" t="b"><v>%s</v></c>`, ref, v))
			case xlsx.Date:
				b.WriteString(fmt.Sprintf(`<c r="%s" s="1"><v>%s</v></c>`, ref, serial(cell.Value)))
			default:
				if cell.Value == "" {
					continue
				}
				b.WriteString(fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, escape(cell.Value)))
			}
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

// serial is the serial number of an ISO 8601 date or date-time in the
// 1900 date system, as the workbook stores it.
func serial(iso string) string {
	t, err := time.Parse("2006-01-02T15:04:05", iso)
	if err != nil {
		if t, err = time.Parse("2006-01-02", iso); err != nil {
			return "0"
		}
	}
	epoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	days := t.Sub(epoch).Hours() / 24
	if days == math.Trunc(days) {
		return fmt.Sprintf("%d", int64(days))
	}
	return fmt.Sprintf("%g", days)
}

func escape(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}
