package extract

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/SpecArch/specarch/internal/vocabulary"
	"github.com/SpecArch/specarch/internal/xlsx"
)

// A data file extract tables reads: a CSV or TSV file, one table, or a
// workbook, one table per worksheet. Every table is a grid of cells with
// its first row the header, cited as a spreadsheet program shows the file
// (C17; in a workbook Register!C17), so that the person who opens the file
// finds the cell.

// tabCell is one cell of a table.
type tabCell struct {
	Value string
	Kind  xlsx.Kind // Text for every CSV cell; a workbook's own kind otherwise
	Row   int       // from 1, the header row
	Col   int       // from 1
	// Line and Column are the cell's place in a CSV file, for check's
	// diagnostics; 0 in a workbook, where Row and Col place it.
	Line, Column int
}

// Blank says whether the cell holds nothing but space.
func (c tabCell) Blank() bool { return strings.TrimSpace(c.Value) == "" }

// tabTable is one table: a header row and the rows under it.
type tabTable struct {
	File   *tabFile
	Sheet  string // the worksheet's name, or "" in a CSV or TSV file
	Name   string // the register's name as the file gives it: the sheet, or the file's stem
	Header []tabCell
	Rows   [][]tabCell // each at least as long as the header, blanks added
	// Extra holds the cells of a row beyond the header's columns.
	Extra [][]tabCell
	// FirstRow, FirstCol, LastRow and LastCol bound the table.
	FirstRow, FirstCol, LastRow, LastCol int
	Instructions                         bool // headed by the vocabulary's Field word: a table of fields
}

// Ref is a cell's reference as the file's readers cite it.
func (t *tabTable) Ref(row, col int) string {
	ref := xlsx.Ref(row, col)
	if t.Sheet != "" {
		return xlsx.SheetRef(t.Sheet) + "!" + ref
	}
	return ref
}

// Range is the table's range.
func (t *tabTable) Range() string {
	r := xlsx.Ref(t.FirstRow, t.FirstCol) + ":" + xlsx.Ref(t.LastRow, t.LastCol)
	if t.Sheet != "" {
		return xlsx.SheetRef(t.Sheet) + "!" + r
	}
	return r
}

// ColumnRange is a column's range: C1:C41.
func (t *tabTable) ColumnRange(col int) string {
	r := xlsx.Ref(t.FirstRow, col) + ":" + xlsx.Ref(t.LastRow, col)
	if t.Sheet != "" {
		return xlsx.SheetRef(t.Sheet) + "!" + r
	}
	return r
}

// Where names the table for a message: the file, and the sheet in a
// workbook.
func (t *tabTable) Where() string {
	if t.Sheet != "" {
		return t.File.Name + " (sheet " + t.Sheet + ")"
	}
	return t.File.Name
}

// tabFile is one file read.
type tabFile struct {
	Path   string // as given
	Name   string // the file's name
	Rel    string // from the repository's root
	Key    string // the source key
	Title  string // the workbook's title, or the file's name
	Tables []*tabTable
	Says   []string  // lines printed after the commit line: the dialect, the sheets
	Aside  []notHeld // what the file holds that is not read
}

// tabFormat is the format a file's extension names.
func tabFormat(name string) (format string, delimiter rune, ok bool) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".csv":
		return "csv", 0, true
	case ".tsv", ".tab":
		return "tsv", '\t', true
	case ".xlsx":
		return "xlsx", 0, true
	}
	return "", 0, false
}

const tabFormats = "CSV (.csv), TSV (.tsv, .tab) and Excel (.xlsx)"

// readTabFile reads one data file into its tables.
func readTabFile(path, rel string, data []byte, v *vocabulary.Vocabulary) (*tabFile, error) {
	f := &tabFile{Path: path, Name: filepath.Base(path), Rel: rel, Title: filepath.Base(path)}
	format, delimiter, ok := tabFormat(path)
	if !ok {
		return nil, refuse("%s is not a data file this build reads: it reads %s, by the file's extension; save the register in one of these formats and read that file", path, tabFormats)
	}
	stem := strings.TrimSuffix(f.Name, filepath.Ext(f.Name))
	if format == "xlsx" {
		if err := f.readWorkbook(data, v); err != nil {
			return nil, refuse("%s cannot be read as an Excel workbook: %v", path, err)
		}
		return f, nil
	}
	d, err := detectDialect(f.Name, data, delimiter)
	if err != nil {
		return nil, refuse("%s cannot be read as %s: %s", path, strings.ToUpper(format), err)
	}
	f.Says = append(f.Says, "dialect "+f.Name+": "+d.String())
	r := csv.NewReader(bytes.NewReader(d.text))
	r.Comma = d.delimiter
	r.FieldsPerRecord = -1
	r.ReuseRecord = false
	var grid [][]tabCell
	// A record's row is the one a spreadsheet program shows it on: the
	// reader skips an empty line, which is no record, so the lines it
	// skips between two records count as rows; a quoted field that holds
	// line breaks spans lines but is one row.
	nextRow, lineAfter := 1, 1
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, refuse("%s cannot be read as %s: %v; a field that holds the delimiter, a quote or a line break is quoted with \" and a quote inside it doubled (RFC 4180)", path, strings.ToUpper(format), err)
		}
		first, _ := r.FieldPos(0)
		rowNumber := nextRow + max(first-lineAfter, 0)
		row := make([]tabCell, len(rec))
		for i, s := range rec {
			line, col := r.FieldPos(i)
			row[i] = tabCell{Value: s, Kind: xlsx.Text, Row: rowNumber, Col: i + 1, Line: line, Column: col}
		}
		last, _ := r.FieldPos(len(rec) - 1)
		lineAfter = last + strings.Count(rec[len(rec)-1], "\n") + 1
		nextRow = rowNumber + 1
		grid = append(grid, row)
	}
	if len(grid) == 0 {
		return nil, refuse("%s holds no row; a register starts with its header row", path)
	}
	t := tableOf(grid, "", stem, v)
	if len(t.Header) == 0 {
		return nil, refuse("%s: its first row, line %d, holds no cell with a value, and a register starts with its header row; put the header row first and read that file", path, grid[0][0].Line)
	}
	t.File = f
	f.Tables = []*tabTable{t}
	return f, nil
}

// readWorkbook reads every worksheet's first region as a table; the cells
// outside it, a sheet with no cells and a chart sheet are set aside.
func (f *tabFile) readWorkbook(data []byte, v *vocabulary.Vocabulary) error {
	wb, err := xlsx.Read(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	if wb.Title != "" {
		f.Title = wb.Title
	}
	var sheets []string
	for _, name := range wb.NotRead {
		f.Aside = append(f.Aside, notHeld{text: fmt.Sprintf("the sheet %s of %s is a chart or a dialog sheet, which holds no cells; it is not read", name, f.Name), clause: "sheet " + name, blocks: []string{"entities"}})
	}
	for _, s := range wb.Sheets {
		kinds := map[string]xlsx.Cell{}
		for _, c := range s.Cells {
			kinds[c.Ref] = c
		}
		regions := s.Regions()
		if len(regions) == 0 {
			f.Aside = append(f.Aside, notHeld{text: fmt.Sprintf("the sheet %s of %s holds no cell; it is not read", s.Name, f.Name), clause: "sheet " + s.Name, blocks: []string{"entities"}})
			continue
		}
		if s.Hidden {
			f.Says = append(f.Says, fmt.Sprintf("sheet %s of %s is hidden in the workbook; it is read as the others are", s.Name, f.Name))
		}
		first := regions[0]
		grid := make([][]tabCell, len(first.Rows))
		for i, row := range first.Rows {
			grid[i] = make([]tabCell, len(row))
			for j := range row {
				r, c := first.FirstRow+i, first.FirstColumn+j
				cell := tabCell{Row: r, Col: c}
				if x, ok := kinds[xlsx.Ref(r, c)]; ok {
					cell.Value, cell.Kind = x.Value, x.Kind
				}
				grid[i][j] = cell
			}
		}
		t := tableOf(grid, s.Name, s.Name, v)
		if len(t.Header) == 0 {
			f.Aside = append(f.Aside, notHeld{text: fmt.Sprintf("the sheet %s of %s holds no value in the first row of its table at %s, which a register's header row is; it is not read", s.Name, f.Name, first.Range()), clause: "sheet " + s.Name, blocks: []string{"entities"}})
			continue
		}
		t.File = f
		f.Tables = append(f.Tables, t)
		kind := "a register"
		if t.Instructions {
			kind = "a table of fields"
		}
		sheets = append(sheets, fmt.Sprintf("%s %s of %s", s.Name, kind, plural(len(t.Rows), "row")))
		for _, r := range regions[1:] {
			f.Aside = append(f.Aside, notHeld{text: fmt.Sprintf("the cells at %s of %s are outside the sheet's table at %s; they are not read", r.Range(), f.Name, t.Range()), clause: r.Range(), blocks: []string{"entities"}})
		}
	}
	if len(sheets) == 0 {
		return fmt.Errorf("it holds no worksheet with cells")
	}
	f.Says = append(f.Says, fmt.Sprintf("workbook %s: %s", f.Name, strings.Join(sheets, ", ")))
	return nil
}

// tableOf makes a table of a grid whose first row is the header. The
// header is cut to its last cell that holds text, and a table whose
// header is cut to nothing is the caller's to refuse or set aside; a
// row's cells beyond the header go to Extra, and a shorter row is filled
// with blanks.
func tableOf(grid [][]tabCell, sheet, name string, v *vocabulary.Vocabulary) *tabTable {
	header := grid[0]
	width := len(header)
	for width > 0 && header[width-1].Blank() {
		width--
	}
	t := &tabTable{Sheet: sheet, Name: name, Header: header[:width], FirstRow: header[0].Row, FirstCol: header[0].Col}
	t.LastRow, t.LastCol = grid[len(grid)-1][0].Row, t.FirstCol+width-1
	if width > 0 {
		t.Instructions = vocabulary.Key(v.Headers, header[0].Value) == vocabulary.Field
	}
	for _, row := range grid[1:] {
		cells := make([]tabCell, width)
		copy(cells, row)
		for i := len(row); i < width; i++ {
			cells[i] = tabCell{Row: row[0].Row, Col: t.FirstCol + i, Line: row[0].Line}
		}
		var extra []tabCell
		for _, c := range row[width:] {
			if !c.Blank() {
				extra = append(extra, c)
			}
		}
		t.Rows = append(t.Rows, cells)
		t.Extra = append(t.Extra, extra)
	}
	return t
}

// dialect is how a CSV or TSV file is written, detected from its bytes.
type dialect struct {
	text      []byte // the file without its byte order mark
	bom       bool
	crlf      bool
	delimiter rune
	quoted    bool
	columns   int // 1 for a file with no delimiter
}

func (d dialect) String() string {
	enc := "UTF-8"
	if d.bom {
		enc = "UTF-8 with a byte order mark"
	}
	ends := "LF line ends"
	if d.crlf {
		ends = "CRLF line ends"
	}
	delim := fmt.Sprintf("delimiter %s", delimiterName(d.delimiter))
	if d.columns == 1 {
		delim = "no delimiter, one column"
	}
	quoted := "no quoted field"
	if d.quoted {
		quoted = `fields quoted with "`
	}
	return strings.Join([]string{enc, ends, delim, quoted}, ", ")
}

func delimiterName(r rune) string {
	switch r {
	case ',':
		return "','"
	case ';':
		return "';'"
	case '\t':
		return "tab"
	case '|':
		return "'|'"
	}
	return fmt.Sprintf("%q", r)
}

// detectDialect reads the encoding, the line ends and the delimiter. A
// TSV file's delimiter is the tab; a CSV file's is the one of comma,
// semicolon, tab and bar that the most records hold the same number of
// times, outside quotes, as the header record does, and then the most
// often, so a ragged row does not hide the delimiter; a file that is not
// UTF-8, one with lone CR line ends and two delimiters that fit equally
// are refused.
func detectDialect(name string, data []byte, given rune) (*dialect, error) {
	d := &dialect{text: data}
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		d.bom, d.text = true, data[3:]
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}) || bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return nil, fmt.Errorf("it starts with a UTF-16 byte order mark; the readers read UTF-8, so save the file as UTF-8 (CSV UTF-8 in Excel) and read that")
	}
	if !utf8.Valid(d.text) {
		at := 0
		for at < len(d.text) {
			r, n := utf8.DecodeRune(d.text[at:])
			if r == utf8.RuneError && n == 1 {
				break
			}
			at += n
		}
		if d.bom {
			at += 3
		}
		return nil, fmt.Errorf("it is not UTF-8: byte %d (0x%02X) starts a sequence that is not UTF-8; save the file as UTF-8 and read that", at+1, data[at])
	}
	d.crlf = bytes.Contains(d.text, []byte("\r\n"))
	if n := bytes.Count(d.text, []byte("\r")); n > bytes.Count(d.text, []byte("\r\n")) {
		return nil, fmt.Errorf("it holds a CR line end with no LF after it, which no CSV reader takes as a line end; save the file with LF or CRLF line ends and read that")
	}
	d.quoted = bytes.Contains(d.text, []byte(`"`))
	if given != 0 {
		d.delimiter = given
		d.columns = 2
		return d, nil
	}
	candidates := []rune{',', ';', '\t', '|'}
	counts := map[rune][]int{}
	record := 0
	for _, c := range candidates {
		counts[c] = []int{0}
	}
	inQuote := false
	for _, r := range string(d.text) {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == '\n' && !inQuote:
			record++
			for _, c := range candidates {
				counts[c] = append(counts[c], 0)
			}
		case !inQuote:
			if n, ok := counts[r]; ok {
				n[record]++
			}
		}
	}
	// The last record may end without a line end; an empty last record is
	// the line end of the one before.
	records := record + 1
	if bytes.HasSuffix(bytes.TrimRight(d.text, "\r"), []byte("\n")) || len(bytes.TrimSpace(d.text)) == 0 {
		records = record
	}
	var fit []rune
	best, bestFitting := 0, 0
	for _, c := range candidates {
		n := counts[c][:records]
		if len(n) == 0 || n[0] == 0 {
			continue
		}
		fitting := 0
		for _, k := range n {
			if k == n[0] {
				fitting++
			}
		}
		switch {
		case fitting > bestFitting || fitting == bestFitting && n[0] > best:
			fit, best, bestFitting = []rune{c}, n[0], fitting
		case fitting == bestFitting && n[0] == best:
			fit = append(fit, c)
		}
	}
	switch {
	case len(fit) == 1:
		d.delimiter = fit[0]
		d.columns = best + 1
	case len(fit) == 0:
		d.delimiter = ','
		d.columns = 1
	default:
		var names []string
		for _, c := range fit {
			names = append(names, delimiterName(c))
		}
		return nil, fmt.Errorf("two delimiters fit equally: %s each occur %d times in the header record and in %d of %d records; save the file with one delimiter, the fields that hold the other quoted, and read that", strings.Join(names, " and "), best, bestFitting, records)
	}
	return d, nil
}
