// Package typerows matches a field of the design to a row of a type
// rendering (docs/idioms.md, "Stacks and type rows"). The validator asks
// whether a row exists; a generator asks which one, and both ask here, so
// the two answers agree.
package typerows

import (
	"fmt"
	"strconv"
	"strings"
)

// Shape is what a row matches a field on.
type Shape struct {
	Type, Format         string
	Enum                 bool
	MaxLength, Precision *int
	ItemsType            string
	ItemsFormat          string
}

// Row is one row of a type rendering.
type Row struct {
	Type, Format                     string
	Enum                             bool
	MaxLengthAtMost, PrecisionAtMost *int
	ItemsType, ItemsFormat           string
	Render, Check                    string
	Plain                            map[string]any // the row as written
}

// ShapeOf reads a field given as plain values: its JSON type with null
// dropped, a $ref to an enum as an enum string and to an entity as an
// object.
func ShapeOf(f map[string]any) Shape {
	var s Shape
	ref := text(f["$ref"])
	switch {
	case strings.HasPrefix(ref, "#/enums/"):
		s.Type, s.Enum = "string", true
		return s
	case strings.HasPrefix(ref, "#/entities/"):
		s.Type = "object"
		return s
	}
	switch t := f["type"].(type) {
	case []any:
		for _, item := range t {
			if text(item) != "null" {
				s.Type = text(item)
			}
		}
	default:
		s.Type = text(t)
	}
	s.Format = text(f["format"])
	_, s.Enum = f["enum"]
	s.MaxLength = intOf(f["maxLength"])
	s.Precision = intOf(f["precision"])
	if items, ok := f["items"].(map[string]any); ok {
		is := ShapeOf(items)
		s.ItemsType, s.ItemsFormat = is.Type, is.Format
	}
	return s
}

// RowsOf reads the rows of a rendering given as plain values.
func RowsOf(rows []any) []Row {
	var out []Row
	for _, r := range rows {
		m, _ := r.(map[string]any)
		out = append(out, Row{
			Type: text(m["type"]), Format: text(m["format"]), Enum: text(m["enum"]) == "true",
			MaxLengthAtMost: intOf(m["maxLengthAtMost"]), PrecisionAtMost: intOf(m["precisionAtMost"]),
			ItemsType: text(m["itemsType"]), ItemsFormat: text(m["itemsFormat"]),
			Render: text(m["render"]), Check: text(m["check"]), Plain: m,
		})
	}
	return out
}

// Match is the first row that renders the field. A row without a format
// matches a field of any format that no row of the rendering names, so a
// decimal never falls through to a row for plain text.
func Match(s Shape, rows []Row) (Row, bool) {
	claimed := map[string]bool{}
	for _, r := range rows {
		if r.Format != "" {
			claimed[r.Format] = true
		}
	}
	for _, r := range rows {
		if r.Type != s.Type {
			continue
		}
		if r.Format != "" && r.Format != s.Format || r.Format == "" && claimed[s.Format] {
			continue
		}
		if r.Enum && !s.Enum {
			continue
		}
		if r.MaxLengthAtMost != nil && (s.MaxLength == nil || *s.MaxLength > *r.MaxLengthAtMost) {
			continue
		}
		if r.PrecisionAtMost != nil && (s.Precision == nil || *s.Precision > *r.PrecisionAtMost) {
			continue
		}
		if r.ItemsType != "" && r.ItemsType != s.ItemsType {
			continue
		}
		if r.ItemsFormat != "" && r.ItemsFormat != s.ItemsFormat {
			continue
		}
		return r, true
	}
	return Row{}, false
}

// Describe names a shape in a message: string, format uuid, maxLength 36.
func (s Shape) Describe() string {
	parts := []string{s.Type}
	if s.Format != "" {
		parts = append(parts, "format "+s.Format)
	}
	if s.Enum {
		parts = append(parts, "enum")
	}
	if s.MaxLength != nil {
		parts = append(parts, "maxLength "+strconv.Itoa(*s.MaxLength))
	}
	if s.Precision != nil {
		parts = append(parts, "precision "+strconv.Itoa(*s.Precision))
	}
	return strings.Join(parts, ", ")
}

func intOf(v any) *int {
	if v == nil {
		return nil
	}
	n, err := strconv.Atoi(text(v))
	if err != nil {
		n = 0
	}
	return &n
}

func text(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	}
	return fmt.Sprint(v)
}
