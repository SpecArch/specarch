// Package xlsx reads the cells of an Office Open XML workbook (.xlsx, ISO/IEC
// 29500) with the standard library alone: every sheet in the workbook's
// order, every cell with its reference, its value as the workbook stores it
// and its kind, and the regions of cells that touch. It reads values, never
// computes a formula (a formula's cell holds the value last computed), and
// writes nothing.
package xlsx

import (
	"archive/zip"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SpecArch/specarch/internal/ooxml"
)

// Kind is what a cell holds.
type Kind string

const (
	Text    Kind = "text"
	Number  Kind = "number"
	Boolean Kind = "boolean"
	Date    Kind = "date" // a number under a date format, given as an ISO 8601 date or date-time
	Error   Kind = "error"
)

// Cell is one cell that holds a value. Row and Column count from 1.
type Cell struct {
	Ref     string // as A1
	Row     int
	Column  int
	Value   string
	Kind    Kind
	Formula bool // the value is the one the workbook last computed
}

// Sheet is one worksheet, its cells in row-major order.
type Sheet struct {
	Name   string
	Hidden bool
	Cells  []Cell
}

// Workbook is the worksheets in the workbook's order. NotRead names the
// sheets that are not worksheets (chart sheets, dialog sheets), which hold
// no cells to read.
type Workbook struct {
	Title   string // docProps/core.xml's dc:title, or ""
	Sheets  []Sheet
	NotRead []string
}

// Region is a rectangle of cells: a band of rows with no empty row of the
// sheet in it, cut at the columns empty in every row of the band. Rows
// holds every cell of the rectangle, "" where a cell is empty.
type Region struct {
	Sheet                                      string
	FirstRow, FirstColumn, LastRow, LastColumn int
	Rows                                       [][]string
}

// Range is the region's reference as a formula names it, Sheet!A1:C4, with
// the sheet's name quoted when it holds anything but letters, digits and
// underscores.
func (r Region) Range() string {
	return SheetRef(r.Sheet) + "!" + Ref(r.FirstRow, r.FirstColumn) + ":" + Ref(r.LastRow, r.LastColumn)
}

var plainSheetName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// SheetRef is a sheet's name as a reference writes it: as it is, or in
// single quotes with a quote doubled.
func SheetRef(name string) string {
	if plainSheetName.MatchString(name) {
		return name
	}
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

// ColumnName is the letters of a column, 1 for A.
func ColumnName(column int) string {
	var b []byte
	for column > 0 {
		column--
		b = append([]byte{byte('A' + column%26)}, b...)
		column /= 26
	}
	return string(b)
}

// Ref is a cell's reference, as B4.
func Ref(row, column int) string { return ColumnName(column) + strconv.Itoa(row) }

var refPattern = regexp.MustCompile(`^([A-Z]{1,3})([0-9]+)$`)

// ParseRef reads a reference such as B4 into its row and column.
func ParseRef(ref string) (row, column int, err error) {
	m := refPattern.FindStringSubmatch(strings.ReplaceAll(ref, "$", ""))
	if m == nil {
		return 0, 0, fmt.Errorf("%q is not a cell reference such as B4", ref)
	}
	for _, c := range m[1] {
		column = column*26 + int(c-'A'+1)
	}
	row, _ = strconv.Atoi(m[2])
	return row, column, nil
}

// Regions splits the sheet into its rectangles of touching cells, top to
// bottom and then left to right: a blank row ends a band of rows, and in a
// band a blank column ends a region.
func (s Sheet) Regions() []Region {
	if len(s.Cells) == 0 {
		return nil
	}
	byRow := map[int][]Cell{}
	var rows []int
	for _, c := range s.Cells {
		if strings.TrimSpace(c.Value) == "" {
			continue
		}
		if _, ok := byRow[c.Row]; !ok {
			rows = append(rows, c.Row)
		}
		byRow[c.Row] = append(byRow[c.Row], c)
	}
	sort.Ints(rows)
	var out []Region
	for i := 0; i < len(rows); {
		j := i
		for j+1 < len(rows) && rows[j+1] == rows[j]+1 {
			j++
		}
		cols := map[int]bool{}
		for _, r := range rows[i : j+1] {
			for _, c := range byRow[r] {
				cols[c.Column] = true
			}
		}
		var cs []int
		for c := range cols {
			cs = append(cs, c)
		}
		sort.Ints(cs)
		for a := 0; a < len(cs); {
			b := a
			for b+1 < len(cs) && cs[b+1] == cs[b]+1 {
				b++
			}
			reg := Region{Sheet: s.Name, FirstRow: rows[i], LastRow: rows[j], FirstColumn: cs[a], LastColumn: cs[b]}
			for r := reg.FirstRow; r <= reg.LastRow; r++ {
				line := make([]string, reg.LastColumn-reg.FirstColumn+1)
				for _, c := range byRow[r] {
					if c.Column >= reg.FirstColumn && c.Column <= reg.LastColumn {
						line[c.Column-reg.FirstColumn] = strings.TrimSpace(c.Value)
					}
				}
				reg.Rows = append(reg.Rows, line)
			}
			out = append(out, reg)
			a = b + 1
		}
		i = j + 1
	}
	return out
}

// Read reads a workbook from its bytes.
func Read(r io.ReaderAt, size int64) (*Workbook, error) {
	z, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("not a workbook: the file is not a zip package (%v)", err)
	}
	return FromZip(z)
}

// FromZip reads a workbook from its opened package.
func FromZip(z *zip.Reader) (*Workbook, error) {
	files := ooxml.Files(z)
	var book struct {
		Sheets []struct {
			Name  string `xml:"name,attr"`
			State string `xml:"state,attr"`
			ID    string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
		Date1904 struct {
			Value string `xml:"date1904,attr"`
		} `xml:"workbookPr"`
	}
	if err := ooxml.Decode(files, "xl/workbook.xml", &book); err != nil {
		return nil, err
	}
	rels, err := ooxml.Relationships(files, "xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	var shared []string
	if _, ok := files["xl/sharedStrings.xml"]; ok {
		var sst struct {
			Items []richText `xml:"si"`
		}
		if err := ooxml.Decode(files, "xl/sharedStrings.xml", &sst); err != nil {
			return nil, err
		}
		for _, si := range sst.Items {
			shared = append(shared, si.String())
		}
	}
	dates, err := dateStyles(files)
	if err != nil {
		return nil, err
	}
	wb := &Workbook{Title: ooxml.CoreTitle(files)}
	epoch1904 := book.Date1904.Value == "1" || book.Date1904.Value == "true"
	for _, s := range book.Sheets {
		rel, ok := rels[s.ID]
		if !ok || !strings.HasSuffix(rel.Type, "/worksheet") {
			wb.NotRead = append(wb.NotRead, s.Name)
			continue
		}
		cells, err := readSheet(files, rel.Target, shared, dates, epoch1904)
		if err != nil {
			return nil, fmt.Errorf("sheet %s: %v", s.Name, err)
		}
		wb.Sheets = append(wb.Sheets, Sheet{Name: s.Name, Hidden: s.State == "hidden" || s.State == "veryHidden", Cells: cells})
	}
	return wb, nil
}

type richText struct {
	T    *string `xml:"t"`
	Runs []struct {
		T string `xml:"t"`
	} `xml:"r"`
}

func (r richText) String() string {
	if r.T != nil {
		return *r.T
	}
	var b strings.Builder
	for _, run := range r.Runs {
		b.WriteString(run.T)
	}
	return b.String()
}

func readSheet(files map[string]*zip.File, name string, shared []string, dates map[int]bool, epoch1904 bool) ([]Cell, error) {
	var ws struct {
		Rows []struct {
			Cells []struct {
				Ref    string   `xml:"r,attr"`
				Type   string   `xml:"t,attr"`
				Style  int      `xml:"s,attr"`
				Value  *string  `xml:"v"`
				F      *string  `xml:"f"`
				Inline richText `xml:"is"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := ooxml.Decode(files, name, &ws); err != nil {
		return nil, err
	}
	var out []Cell
	for _, row := range ws.Rows {
		for _, c := range row.Cells {
			r, col, err := ParseRef(c.Ref)
			if err != nil {
				return nil, err
			}
			cell := Cell{Ref: Ref(r, col), Row: r, Column: col, Formula: c.F != nil}
			v := ""
			if c.Value != nil {
				v = *c.Value
			}
			switch c.Type {
			case "s":
				i, err := strconv.Atoi(v)
				if err != nil || i < 0 || i >= len(shared) {
					return nil, fmt.Errorf("cell %s names shared string %q, which the workbook does not hold", c.Ref, v)
				}
				cell.Value, cell.Kind = shared[i], Text
			case "inlineStr":
				cell.Value, cell.Kind = c.Inline.String(), Text
			case "str":
				cell.Value, cell.Kind = v, Text
			case "b":
				cell.Kind = Boolean
				cell.Value = map[string]string{"1": "true", "0": "false"}[v]
			case "e":
				cell.Value, cell.Kind = v, Error
			case "d":
				cell.Value, cell.Kind = v, Date
			default:
				if c.Value == nil {
					continue
				}
				cell.Value, cell.Kind = v, Number
				if dates[c.Style] {
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						cell.Value, cell.Kind = serialDate(f, epoch1904), Date
					}
				}
			}
			if cell.Value == "" && cell.Kind != Text {
				continue
			}
			out = append(out, cell)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Row != out[j].Row {
			return out[i].Row < out[j].Row
		}
		return out[i].Column < out[j].Column
	})
	return out, nil
}

// serialDate turns a serial day number into an ISO 8601 date, or a
// date-time when it holds a part of a day.
func serialDate(f float64, epoch1904 bool) string {
	base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	if epoch1904 {
		base = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	days := math.Floor(f)
	secs := math.Round((f - days) * 86400)
	t := base.AddDate(0, 0, int(days)).Add(time.Duration(secs) * time.Second)
	if secs == 0 {
		return t.Format("2006-01-02")
	}
	return t.Format("2006-01-02T15:04:05")
}

// dateStyles is the cell styles whose number format is a date: the
// built-in formats 14 to 22 and 45 to 47, and a custom format that holds a
// day, month or year outside quotes and brackets.
func dateStyles(files map[string]*zip.File) (map[int]bool, error) {
	out := map[int]bool{}
	if _, ok := files["xl/styles.xml"]; !ok {
		return out, nil
	}
	var st struct {
		NumFmts []struct {
			ID   int    `xml:"numFmtId,attr"`
			Code string `xml:"formatCode,attr"`
		} `xml:"numFmts>numFmt"`
		Xfs []struct {
			NumFmt int `xml:"numFmtId,attr"`
		} `xml:"cellXfs>xf"`
	}
	if err := ooxml.Decode(files, "xl/styles.xml", &st); err != nil {
		return nil, err
	}
	custom := map[int]bool{}
	for _, f := range st.NumFmts {
		custom[f.ID] = isDateFormat(f.Code)
	}
	for i, x := range st.Xfs {
		if d, ok := custom[x.NumFmt]; ok {
			out[i] = d
		} else {
			out[i] = (x.NumFmt >= 14 && x.NumFmt <= 22) || (x.NumFmt >= 45 && x.NumFmt <= 47)
		}
	}
	return out, nil
}

var quotedOrBracketed = regexp.MustCompile(`"[^"]*"|\[[^\]]*\]|\\.`)

func isDateFormat(code string) bool {
	code = strings.ToLower(quotedOrBracketed.ReplaceAllString(code, ""))
	return strings.ContainsAny(code, "dy") || (strings.Contains(code, "m") && !strings.Contains(code, "0"))
}
