// Package generate writes documents from SpecArch files. The techspec
// target writes an arc42 technical specification in Markdown with Mermaid
// diagrams, and rewrites the generated regions of hand-written Markdown.
package generate

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// node helpers, short names for the generator's many lookups.
func get(n *yaml.Node, key string) *yaml.Node { return source.Child(n, key) }
func str(n *yaml.Node, key string) string     { return source.Str(source.Child(n, key)) }
func pairs(n *yaml.Node, key string) []source.Pair {
	return source.Pairs(source.Child(n, key))
}
func items(n *yaml.Node, key string) []*yaml.Node { return source.Items(source.Child(n, key)) }

func strs(n *yaml.Node, key string) []string {
	var out []string
	for _, i := range items(n, key) {
		out = append(out, i.Value)
	}
	return out
}

// operation is one HTTP operation of a design file.
type operation struct {
	id, method, path string
	node, pathItem   *yaml.Node
}

var methods = []string{"get", "post", "put", "patch", "delete"}

func operations(root *yaml.Node) []operation {
	var out []operation
	for _, p := range pairs(root, "paths") {
		for _, m := range methods {
			if op := get(p.Value, m); op != nil {
				out = append(out, operation{id: str(op, "operationId"), method: m, path: p.Key.Value, node: op, pathItem: p.Value})
			}
		}
	}
	return out
}

func findOperation(root *yaml.Node, id string) *operation {
	for _, o := range operations(root) {
		if o.id == id {
			return &o
		}
	}
	return nil
}

// typeLabel names a field's type the way docs/conventions.md does: the
// concrete type, or the enum or entity it refers to.
func typeLabel(f *yaml.Node) string {
	if ref := str(f, "$ref"); ref != "" {
		return ref[strings.LastIndex(ref, "/")+1:]
	}
	base := str(f, "type")
	for _, t := range items(f, "type") {
		if t.Value != "null" {
			base = t.Value
		}
	}
	switch format := str(f, "format"); format {
	case "int32", "int64", "uint64", "double", "decimal", "date", "duration", "time", "uuid":
		return format
	case "date-time":
		return "timestamp"
	case "byte", "binary":
		return "bytes"
	}
	switch base {
	case "boolean":
		return "bool"
	case "array":
		if it := get(f, "items"); it != nil {
			return "list of " + typeLabel(it)
		}
		return "list"
	case "":
		return "value"
	}
	return base
}

// typeText is typeLabel with the details a reader needs: decimal size,
// whether null is allowed, and how a large integer travels.
func typeText(f *yaml.Node) string {
	t := typeLabel(f)
	if str(f, "format") == "decimal" && str(f, "precision") != "" {
		t = fmt.Sprintf("decimal(%s, %s)", str(f, "precision"), str(f, "scale"))
	}
	if (str(f, "format") == "int64" || str(f, "format") == "uint64") && str(f, "type") == "string" {
		t += ", as a JSON string"
	}
	for _, i := range items(f, "type") {
		if i.Value == "null" {
			t += " or null"
		}
	}
	return t
}

// limits lists a field's constraints in plain words.
func limits(f *yaml.Node) string {
	var out []string
	add := func(key, word string) {
		if v := str(f, key); v != "" {
			out = append(out, word+" "+v)
		}
	}
	add("minimum", "at least")
	add("maximum", "at most")
	add("exclusiveMinimum", "above")
	add("exclusiveMaximum", "below")
	if v := str(f, "minLength"); v != "" {
		out = append(out, "at least "+characters(v))
	}
	if v := str(f, "maxLength"); v != "" {
		out = append(out, "at most "+characters(v))
	}
	if v := str(f, "pattern"); v != "" {
		out = append(out, "matches `"+v+"`")
	}
	if f := str(f, "format"); f == "email" || f == "uri" || f == "hostname" || f == "ipv4" || f == "ipv6" {
		out = append(out, "a valid "+f)
	}
	if str(f, "readOnly") == "true" {
		out = append(out, "set by the system")
	}
	if v := str(f, "mistakes"); v != "" {
		out = append(out, "users get it wrong "+map[string]string{"frequent": "often", "rare": "rarely"}[v])
	}
	return strings.Join(out, ", ")
}

func characters(n string) string {
	if n == "1" {
		return "1 character"
	}
	return n + " characters"
}

// cell makes text safe inside a Markdown table cell.
func cell(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	s = strings.ReplaceAll(s, "|", "\\|")
	if s == "" {
		return " "
	}
	return s
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// mermaidID turns a name into an identifier Mermaid accepts everywhere.
func mermaidID(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// mermaidText makes text safe inside a quoted Mermaid label.
func mermaidText(s string) string {
	return strings.NewReplacer(`"`, "'", "\n", " ", ";", ",", "#", "").Replace(s)
}
