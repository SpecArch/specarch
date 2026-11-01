package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/vocabulary"
	"github.com/SpecArch/specarch/internal/xlsx"
)

// RegisterDump is the format of a register file check compares: a CSV,
// TSV or Excel file, known by its extension, read as extract tables reads
// it and compared with the entity of its name.
const RegisterDump = "register"

// CheckRegister compares a register file with the specification's entity
// of its name, the sheet's or the file's in PascalCase (ADR-215): a column
// no field has, a required field the file lacks (an optional one is a
// report), a blank in a required column, a value outside the field's enum
// a value that does not read as the field's type and a value in a row
// beyond the header's columns, each at the cell.
func CheckRegister(path string, root *yaml.Node) ([]Difference, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := readTabFile(path, filepath.ToSlash(path), data, vocabulary.English())
	if err != nil {
		return nil, err
	}
	entities := source.Child(root, "entities")
	var out []Difference
	for _, tb := range f.Tables {
		if tb.Instructions {
			continue
		}
		name := namePascal(tb.Name)
		entity := source.Child(entities, name)
		if entity == nil {
			d := Difference{Rule: "register_not_specified", Subject: name,
				Message: fmt.Sprintf("the register %s is compared with the entity %s, which the specification does not declare; specify the entity, or name the sheet or the file after one", tb.Where(), name)}
			placeCell(&d, path, tb, tb.Header[0])
			out = append(out, d)
			continue
		}
		at := "/entities/" + escapeToken(name)
		fields := source.Child(entity, "properties")
		byName, byTitle := map[string]*yaml.Node{}, map[string]*yaml.Node{}
		var order []string
		for _, p := range source.Pairs(fields) {
			byName[p.Key.Value] = p.Value
			order = append(order, p.Key.Value)
			if title := strings.ToLower(strings.TrimSpace(source.Str(source.Child(p.Value, "title")))); title != "" {
				byTitle[title] = p.Value
			}
		}
		required := map[string]bool{}
		for _, r := range source.Items(source.Child(entity, "required")) {
			required[r.Value] = true
		}
		present := map[string]bool{}
		type column struct {
			index int
			field string
		}
		var columns []column
		for i, h := range tb.Header {
			field := nameCamel(h.Value)
			if byName[field] == nil {
				if byTitle[strings.ToLower(strings.TrimSpace(h.Value))] != nil {
					field = fieldNamed(fields, byTitle[strings.ToLower(strings.TrimSpace(h.Value))])
				} else {
					d := Difference{Rule: "register_column_not_specified", Subject: name + " " + xlsx.ColumnName(h.Col),
						Message: fmt.Sprintf("column %s of %s is headed %q, and entity %s of the specification has no field %s; specify the field, or take the column out", xlsx.ColumnName(h.Col), tb.Where(), h.Value, name, field)}
					placeCell(&d, path, tb, h)
					out = append(out, d)
					continue
				}
			}
			if present[field] {
				continue
			}
			present[field] = true
			columns = append(columns, column{i, field})
		}
		for _, field := range order {
			if present[field] {
				continue
			}
			d := Difference{Rule: "register_column_missing", Subject: name + "." + field, Pointer: at + "/properties/" + escapeToken(field),
				Message: fmt.Sprintf("entity %s of the specification has the field %s, and %s has no column for it", name, field, tb.Where())}
			if !required[field] {
				d.Report = true
				d.Message += ", which is not required"
			}
			out = append(out, d)
		}
		for r, row := range tb.Rows {
			for _, c := range tb.Extra[r] {
				d := Difference{Rule: "register_value_beyond_header", Subject: name + " " + tb.Ref(c.Row, c.Col),
					Message: fmt.Sprintf("cell %s of %s holds %q beyond the %d columns of the header, and no field takes it; take the value out, or head its column", tb.Ref(c.Row, c.Col), tb.Where(), strings.TrimSpace(c.Value), len(tb.Header))}
				placeCell(&d, path, tb, c)
				out = append(out, d)
			}
			for _, col := range columns {
				field := col.field
				c := row[col.index]
				f := byName[field]
				subject := name + "." + field + " " + tb.Ref(c.Row, c.Col)
				if c.Blank() {
					if required[field] {
						d := Difference{Rule: "register_blank_required", Subject: subject,
							Message: fmt.Sprintf("cell %s of %s is blank, and %s of %s is required", tb.Ref(c.Row, c.Col), tb.Where(), field, name)}
						placeCell(&d, path, tb, c)
						out = append(out, d)
					}
					continue
				}
				value := strings.TrimSpace(c.Value)
				if values := fieldEnum(root, f); values != nil {
					allowed := false
					for _, v := range values {
						allowed = allowed || v == value
					}
					if !allowed {
						d := Difference{Rule: "register_value_not_in_enum", Subject: subject,
							Message: fmt.Sprintf("cell %s of %s holds %q, and %s of %s allows %s", tb.Ref(c.Row, c.Col), tb.Where(), value, field, name, joinAnd(quoteAll(values)))}
						placeCell(&d, path, tb, c)
						out = append(out, d)
					}
					continue
				}
				if want, ok := fieldReads(f, c); !ok {
					d := Difference{Rule: "register_value_not_of_type", Subject: subject,
						Message: fmt.Sprintf("cell %s of %s holds %q, and %s of %s is %s", tb.Ref(c.Row, c.Col), tb.Where(), value, field, name, want)}
					placeCell(&d, path, tb, c)
					out = append(out, d)
				}
			}
		}
	}
	return out, nil
}

// placeCell puts a difference at a cell: its line and column in a CSV
// file, or its row and column in a workbook.
func placeCell(d *Difference, path string, tb *tabTable, c tabCell) {
	d.File, d.Line, d.Column = path, c.Line, c.Column
	if c.Line == 0 {
		d.File, d.Line, d.Column = path, c.Row, c.Col
	}
	d.DumpPointer = "/" + escapeToken(tb.Ref(c.Row, c.Col))
}

func fieldNamed(fields, f *yaml.Node) string {
	for _, p := range source.Pairs(fields) {
		if p.Value == f {
			return p.Key.Value
		}
	}
	return ""
}

// fieldEnum is the values a field allows: its own enum, or the enum it
// refers to.
func fieldEnum(root, f *yaml.Node) []string {
	if ref := source.Str(source.Child(f, "$ref")); strings.HasPrefix(ref, "#/enums/") {
		f = source.Child(source.Child(root, "enums"), strings.TrimPrefix(ref, "#/enums/"))
	}
	items := source.Items(source.Child(f, "enum"))
	if len(items) == 0 {
		return nil
	}
	var out []string
	for _, i := range items {
		out = append(out, i.Value)
	}
	return out
}

// fieldReads says whether a cell reads as the field's type, and names the
// type when it does not.
func fieldReads(f *yaml.Node, c tabCell) (string, bool) {
	kind, s := valueKind(c)
	typ := source.Str(source.Child(f, "type"))
	if t := source.Child(f, "type"); t != nil && t.Kind == yaml.SequenceNode {
		for _, i := range t.Content {
			if i.Value != "null" {
				typ = i.Value
			}
		}
	}
	format := source.Str(source.Child(f, "format"))
	switch {
	case typ == "integer" && format == "int32":
		n, err := strconv.ParseInt(s, 10, 64)
		return "an integer of 32 bits", kind == "integer" && err == nil && n >= -1<<31 && n <= 1<<31-1
	case typ == "integer":
		return "an integer", kind == "integer"
	case typ == "number":
		return "a number", kind == "integer" || kind == "decimal"
	case typ == "boolean":
		return "a boolean", kind == "boolean"
	case typ == "string" && format == "decimal":
		return "a decimal", kind == "integer" || kind == "decimal"
	case typ == "string" && (format == "int64" || format == "uint64"):
		return "an integer carried as text", kind == "integer"
	case typ == "string" && format == "date":
		return "a date", kind == "date"
	case typ == "string" && format == "date-time":
		return "a date-time", kind == "date-time"
	case typ == "string" && format == "email":
		return "an email address", kind == "email"
	case typ == "string" && format == "uuid":
		return "a UUID", kind == "uuid"
	}
	return "", true
}
