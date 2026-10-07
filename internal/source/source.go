// Package source reads a SpecArch YAML file into a node tree that keeps the
// line of every value, and into the plain value the JSON Schema check needs.
package source

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Problem is something wrong with the YAML itself, found while reading.
type Problem struct {
	Line    int
	Path    string
	Rule    string
	Message string
}

// Doc is one parsed file.
type Doc struct {
	Root     *yaml.Node // the top-level mapping, nil when the file did not parse
	Value    any        // the same content as JSON values
	Problems []Problem
}

var yamlLine = regexp.MustCompile(`^yaml: line (\d+): (.*)$`)

var unclosed = map[string]string{
	"did not find expected ',' or '}'":    "the { that starts on this line is not closed; close it with }, and separate its entries with commas",
	"did not find expected ',' or ']'":    "the [ that starts on this line is not closed; close it with ], and separate its items with commas",
	"did not find expected key":           "the mapping that starts on this line has a line that is not a key; check the indentation below it",
	"did not find expected '-' indicator": "the list that starts on this line has a line that is not an item; check the indentation below it",
}

// Parse reads one YAML document. A syntax error is returned as a Problem,
// with Root left nil.
func Parse(data []byte) *Doc {
	d := &Doc{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var file yaml.Node
	if err := dec.Decode(&file); err != nil {
		if errors.Is(err, io.EOF) {
			d.Problems = append(d.Problems, Problem{Line: 1, Path: "/", Rule: "yaml_syntax", Message: "the file is empty"})
			return d
		}
		d.Problems = append(d.Problems, syntaxProblem(err))
		return d
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err == nil {
		d.Problems = append(d.Problems, Problem{Line: extra.Line, Path: "/", Rule: "yaml_syntax", Message: "a file holds one YAML document; found a second"})
		return d
	} else if !errors.Is(err, io.EOF) {
		d.Problems = append(d.Problems, syntaxProblem(err))
		return d
	}
	root := file.Content[0]
	d.Root = root
	d.Value = d.convert(root, "")
	return d
}

// ValueOf converts a node tree into plain JSON values, reporting nothing:
// a repeated key keeps its first value, and a date is its text.
func ValueOf(n *yaml.Node) any {
	d := &Doc{}
	return d.convert(n, "")
}

func syntaxProblem(err error) Problem {
	msg := err.Error()
	line := 1
	if m := yamlLine.FindStringSubmatch(msg); m != nil {
		line, _ = strconv.Atoi(m[1])
		msg = m[2]
	} else {
		msg = strings.TrimPrefix(msg, "yaml: ")
	}
	// For these parser errors the YAML library gives the line where the
	// unclosed block starts, counted from 0.
	if plain, ok := unclosed[msg]; ok {
		line++
		msg = plain
	}
	return Problem{Line: line, Path: "/", Rule: "yaml_syntax", Message: msg}
}

func (d *Doc) convert(n *yaml.Node, path string) any {
	switch n.Kind {
	case yaml.AliasNode:
		return d.convert(n.Alias, path)
	case yaml.MappingNode:
		m := make(map[string]any, len(n.Content)/2)
		seen := map[string]int{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			key := k.Value
			child := path + "/" + EscapeToken(key)
			if first, ok := seen[key]; ok {
				d.Problems = append(d.Problems, Problem{Line: k.Line, Path: child, Rule: "duplicate_key",
					Message: fmt.Sprintf("key %q is already defined on line %d", key, first)})
				continue
			}
			seen[key] = k.Line
			m[key] = d.convert(v, child)
		}
		return m
	case yaml.SequenceNode:
		s := make([]any, 0, len(n.Content))
		for i, c := range n.Content {
			s = append(s, d.convert(c, path+"/"+strconv.Itoa(i)))
		}
		return s
	case yaml.ScalarNode:
		return d.scalar(n, path)
	}
	return nil
}

func (d *Doc) scalar(n *yaml.Node, path string) any {
	switch n.ShortTag() {
	case "!!null":
		return nil
	case "!!bool":
		var b bool
		if n.Decode(&b) == nil {
			return b
		}
	case "!!int":
		var i int64
		if n.Decode(&i) == nil {
			return json.Number(strconv.FormatInt(i, 10))
		}
		var u uint64
		if n.Decode(&u) == nil {
			return json.Number(strconv.FormatUint(u, 10))
		}
	case "!!float":
		var f float64
		if n.Decode(&f) == nil {
			s := strconv.FormatFloat(f, 'g', -1, 64)
			if _, err := strconv.ParseFloat(s, 64); err == nil && !strings.ContainsAny(s, "IN") {
				return json.Number(s)
			}
		}
	case "!!timestamp":
		d.Problems = append(d.Problems, Problem{Line: n.Line, Path: orRoot(path), Rule: "unquoted_date",
			Message: fmt.Sprintf("%s is read by YAML as a timestamp; quote it: \"%s\"", n.Value, n.Value)})
	}
	return n.Value
}

func orRoot(p string) string {
	if p == "" {
		return "/"
	}
	return p
}

// EscapeToken escapes one JSON pointer token.
func EscapeToken(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

// UnescapeToken reverses EscapeToken.
func UnescapeToken(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
}

// Pointer joins tokens into a JSON pointer, "/" for none.
func Pointer(tokens ...string) string {
	if len(tokens) == 0 {
		return "/"
	}
	var b strings.Builder
	for _, t := range tokens {
		b.WriteByte('/')
		b.WriteString(EscapeToken(t))
	}
	return b.String()
}

// Resolve follows a pointer's tokens from n. It returns the deepest node it
// reached and whether it reached the end. For a mapping key it returns the
// value node.
func Resolve(n *yaml.Node, tokens []string) (*yaml.Node, bool) {
	cur := n
	for _, t := range tokens {
		next := Child(cur, t)
		if next == nil {
			return cur, false
		}
		cur = next
	}
	return cur, true
}

// Child returns the value under key in a mapping, or the item at an index in
// a sequence, following aliases; nil when there is none.
func Child(n *yaml.Node, token string) *yaml.Node {
	n = Deref(n)
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == token {
				return Deref(n.Content[i+1])
			}
		}
	case yaml.SequenceNode:
		i, err := strconv.Atoi(token)
		if err == nil && i >= 0 && i < len(n.Content) {
			return Deref(n.Content[i])
		}
	}
	return nil
}

// Key returns the key node for key in a mapping, or nil.
func Key(n *yaml.Node, key string) *yaml.Node {
	n = Deref(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i]
		}
	}
	return nil
}

// Deref follows an alias.
func Deref(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

// Pair is one key and value of a mapping.
type Pair struct {
	Key   *yaml.Node
	Value *yaml.Node
}

// Pairs lists a mapping's entries in document order; nil for anything else.
func Pairs(n *yaml.Node) []Pair {
	n = Deref(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	out := make([]Pair, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		out = append(out, Pair{n.Content[i], Deref(n.Content[i+1])})
	}
	return out
}

// Items lists a sequence's items; nil for anything else.
func Items(n *yaml.Node) []*yaml.Node {
	n = Deref(n)
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]*yaml.Node, len(n.Content))
	for i, c := range n.Content {
		out[i] = Deref(c)
	}
	return out
}

// Str returns a scalar's text, or "" for anything else.
func Str(n *yaml.Node) string {
	n = Deref(n)
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// IsScalar reports whether n is a scalar.
func IsScalar(n *yaml.Node) bool {
	n = Deref(n)
	return n != nil && n.Kind == yaml.ScalarNode
}
