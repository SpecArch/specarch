// Package genregister writes a register template per entity: the logic of
// the specarch-gen-register plug-in. A template is the file people fill
// in and specarch extract tables reads back (ADR-215): the header row of
// the entity's fields, and the instructions, a table of fields stating
// each column's type, whether it is required, its sensitivity, its allowed
// values and its description, in the words of the English vocabulary.
package genregister

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/mark"
	"github.com/SpecArch/specarch/internal/ownership"
	"github.com/SpecArch/specarch/internal/xlsx"
)

// Request is what specarch writes on a plug-in's standard input.
type Request struct {
	Specarch string `json:"specarch"`
	// Digest is the specification's digest, as the approval record keeps
	// it, so every generated file names the specification it came from
	// and a remark from acceptance can name the build (ADR-083).
	Digest          string           `json:"digest"`
	Target          string           `json:"target"`
	Root            string           `json:"root"`
	Specification   map[string]any   `json:"specification"`
	Implementations []Implementation `json:"implementations"`
	Output          string           `json:"output"`
	// Draft is the notice a draft carries, or "" for the approved output.
	Draft string `json:"draft"`
	// Problems are the problems to mark, in the order of the problems file.
	Problems []mark.Problem `json:"problems"`
}

// Implementation is one implementation file in the request.
type Implementation struct {
	File    string         `json:"file"`
	Content map[string]any `json:"content"`
}

// File is one file the plug-in answers with.
type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Diagnostic is one problem the plug-in reports, in the validator's fields.
type Diagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

// Response is what the plug-in writes on its standard output.
type Response struct {
	Files       []File       `json:"files"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Decode reads a request, keeping numbers as their text.
func Decode(data []byte) (*Request, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var r Request
	if err := dec.Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// The header of the instructions, in the English vocabulary's words.
var instructionsHeader = []string{"Field", "Type", "Required", "Sensitivity", "Values", "Description"}

// column is one column of a template.
type column struct {
	header, typ, required, sensitivity, values, description string
}

// Generate writes, per entity, <Entity>.csv with the header row,
// <Entity>-instructions.csv with the table of fields, and <Entity>.xlsx
// holding both as the sheets <Entity> and Instructions. The columns are
// the primary key's fields first, then the others by name, since the
// request carries the fields as a map. A field that holds a list or an
// object is no column, and is named in a warning.
func Generate(r *Request) Response {
	spec := r.Specification
	owned := ownership.Owned{}
	for _, impl := range r.Implementations {
		for ptr, by := range ownership.Of(impl.Content) {
			owned[ptr] = by
		}
	}
	resp := Response{Files: []File{}, Diagnostics: []Diagnostic{}}
	entities := obj(spec["entities"])
	for _, name := range sortedKeys(entities) {
		if owned.Covers("#/entities/" + ownership.Escape(name)) {
			continue
		}
		e := obj(entities[name])
		required := map[string]bool{}
		for _, f := range list(e["required"]) {
			required[text(f)] = true
		}
		var columns []column
		props := obj(e["properties"])
		var order []string
		inKey := map[string]bool{}
		for _, k := range list(e["primaryKey"]) {
			if props[text(k)] != nil && !inKey[text(k)] {
				order = append(order, text(k))
				inKey[text(k)] = true
			}
		}
		for _, field := range sortedKeys(props) {
			if !inKey[field] {
				order = append(order, field)
			}
		}
		for _, field := range order {
			f := obj(props[field])
			c, ok := columnOf(spec, field, f)
			if !ok {
				resp.Diagnostics = append(resp.Diagnostics, Diagnostic{File: r.Root, Line: 1, Severity: "warning", Path: "/entities/" + name + "/properties/" + field, Rule: "register_column",
					Message: fmt.Sprintf("field %s of %s holds a list or an object, which a column of a register cannot; the template leaves it out", field, name)})
				continue
			}
			if required[field] {
				c.required = "yes"
			} else {
				c.required = "no"
			}
			columns = append(columns, c)
		}
		title := name + " register template"
		if r.Draft != "" {
			title += " (" + r.Draft + ")"
		}
		var header, rows [][]string
		header = [][]string{headersOf(columns)}
		rows = [][]string{instructionsHeader}
		for _, c := range columns {
			rows = append(rows, []string{c.header, c.typ, c.required, c.sensitivity, c.values, c.description})
		}
		resp.Files = append(resp.Files,
			File{Path: name + ".csv", Content: csvOf(header)},
			File{Path: name + "-instructions.csv", Content: csvOf(rows)},
			File{Path: name + ".xlsx", Content: workbookOf(title, name, header, rows)})
	}
	return resp
}

// columnOf is a field as a column: its type in the vocabulary's words,
// with its allowed values and description; false for a list or an object.
func columnOf(spec map[string]any, field string, f map[string]any) (column, bool) {
	c := column{header: text(f["title"]), sensitivity: text(f["sensitivity"]), description: text(f["description"])}
	if c.header == "" {
		c.header = field
	}
	if ref := text(f["$ref"]); strings.HasPrefix(ref, "#/") {
		target := spec
		var ok bool
		parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
		for _, p := range parts {
			target, ok = target[unescape(p)].(map[string]any)
			if !ok {
				c.typ = "text"
				return c, true
			}
		}
		switch parts[0] {
		case "enums":
			c.typ = "text"
			c.values = valuesOf(list(target["enum"]))
			if c.description == "" {
				c.description = text(target["description"])
			}
			return c, true
		case "entities":
			key := list(target["primaryKey"])
			props := obj(target["properties"])
			if len(key) == 1 {
				if kc, ok := columnOf(spec, text(key[0]), obj(props[text(key[0])])); ok {
					c.typ = kc.typ
				}
			}
			if c.typ == "" {
				c.typ = "text"
			}
			if c.description == "" {
				c.description = fmt.Sprintf("The %s it names, by its %s.", parts[1], strings.Join(textList(key), " and "))
			}
			return c, true
		}
		c.typ = "text"
		return c, true
	}
	typ := text(f["type"])
	if ts := list(f["type"]); len(ts) > 0 {
		for _, t := range ts {
			if text(t) != "null" {
				typ = text(t)
			}
		}
	}
	format := text(f["format"])
	switch {
	case typ == "array" || typ == "object":
		return c, false
	case typ == "integer":
		c.typ = "integer"
	case typ == "number":
		c.typ = "decimal"
	case typ == "boolean":
		c.typ = "boolean"
	case format == "decimal":
		c.typ = "decimal"
		if p, s := text(f["precision"]), text(f["scale"]); p != "" && s != "" {
			c.description = strings.TrimSpace(fmt.Sprintf("%s digits, %s after the point. %s", p, s, c.description))
		}
	case format == "date":
		c.typ = "date"
	case format == "date-time":
		c.typ = "date-time"
	case format == "email":
		c.typ = "email"
	case format == "uuid":
		c.typ = "uuid"
	case format == "int64" || format == "uint64":
		c.typ = "64-bit integer"
	default:
		c.typ = "text"
	}
	if values := list(f["enum"]); len(values) > 0 {
		c.values = valuesOf(values)
	}
	return c, true
}

func headersOf(columns []column) []string {
	var out []string
	for _, c := range columns {
		out = append(out, c.header)
	}
	return out
}

func valuesOf(values []any) string {
	return strings.Join(textList(values), " | ")
}

func textList(values []any) []string {
	var out []string
	for _, v := range values {
		out = append(out, text(v))
	}
	return out
}

// csvOf writes rows as RFC 4180 CSV with CRLF line ends, which every
// spreadsheet program opens as it is.
func csvOf(rows [][]string) string {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.UseCRLF = true
	for _, row := range rows {
		_ = w.Write(row)
	}
	w.Flush()
	return b.String()
}

// workbookOf writes the register sheet and the instructions sheet.
func workbookOf(title, name string, header, rows [][]string) string {
	cells := func(rs [][]string) [][]Cell {
		var out [][]Cell
		for _, r := range rs {
			var row []Cell
			for _, v := range r {
				row = append(row, Cell{xlsx.Text, v})
			}
			out = append(out, row)
		}
		return out
	}
	var b bytes.Buffer
	_ = WriteWorkbook(&b, title, []Sheet{{Name: name, Rows: cells(header)}, {Name: "Instructions", Rows: cells(rows)}})
	return b.String()
}

func unescape(s string) string {
	return strings.NewReplacer("~1", "/", "~0", "~").Replace(s)
}

func obj(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func text(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "true"
		}
		return "false"
	}
	return fmt.Sprint(v)
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
