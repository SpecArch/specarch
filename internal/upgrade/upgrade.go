// Package upgrade finds what moving one file to the newest version of its
// format changes (docs/meta-model-1.0.0.md, specarch upgrade): its version
// line, its editor hint and the keys 1.0.0 renamed. Each change is an edit
// of the text at the line and column the parser gives, so the rest of the
// file stays byte for byte as it was.
package upgrade

import (
	"bytes"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/metamodel"
	"github.com/SpecArch/specarch/internal/source"
)

// Kind is what a file of a specification holds, which says which version
// line it has.
type Kind int

const (
	// Root is a specification's root file, whose version line is specarch.
	Root Kind = iota
	// Fragment is a file under a stage folder or a test, with no version
	// line of its own.
	Fragment
	// Implementation is an implementation file.
	Implementation
	// Idiom is an idiom beside an implementation file.
	Idiom
	// Record is a record beside a specification.
	Record
	// Estate is an estate file.
	Estate
)

// formats gives the format of each kind that has a version line.
var formats = map[Kind]metamodel.Format{
	Root:           metamodel.Design,
	Implementation: metamodel.Implementation,
	Idiom:          metamodel.Idiom,
	Record:         metamodel.Record,
	Estate:         metamodel.Estate,
}

// Change is one edit of a file: at the line and column (from 1, in
// characters, as the parser gives them), the text old becomes new.
type Change struct {
	File         string
	Line, Column int
	Old, New     string
	offset       int // in bytes, from the file's start
}

// String is the change as upgrade prints it.
func (c Change) String() string {
	return c.File + ":" + strconv.Itoa(c.Line) + ":" + strconv.Itoa(c.Column) + ": " + c.Old + " -> " + c.New
}

const hintPrefix = "# yaml-language-server: $schema="

// schemaFile matches the file name of a SpecArch schema before 1.0, its kind
// and version apart.
var schemaFile = regexp.MustCompile(`(specarch-(?:design|fragment-[a-z]+|implementation|record|idiom|estate))-(0\.[0-9]+)(\.schema\.json)$`)

// releaseURL is SpecArch's repository's files at a commit or tag.
var releaseURL = regexp.MustCompile(`^(https://raw\.githubusercontent\.com/SpecArch/specarch/)([^/]+)(/schema/)`)

// Changes lists what moving the file at path to 1.0.0 changes in it, in
// the order they appear. program is the program's version, whose release
// tag a hint that names a release URL moves to. A file the parser cannot
// read gives none; validate's problem at the parser's position is the
// step left to the author.
func Changes(path string, data []byte, kind Kind, program string) []Change {
	var out []Change
	if c, ok := hint(path, data, program); ok {
		out = append(out, c)
	}
	doc := source.Parse(data)
	if doc.Root == nil || doc.Root.Kind != yaml.MappingNode || len(doc.Problems) > 0 {
		return out
	}
	if f, ok := formats[kind]; ok {
		if n := source.Child(doc.Root, f.Key); n != nil && n.Kind == yaml.ScalarNode && n.Tag == "!!str" && f.IsEarlier(n.Value) {
			if c, ok := scalarChange(path, data, n, metamodel.Current); ok {
				out = append(out, c)
			}
		}
	}
	if kind == Root || kind == Fragment {
		for _, r := range source.Pairs(source.Child(doc.Root, "roles")) {
			if c, ok := keyChange(path, data, r.Value, metamodel.RowPoliciesOldKey, metamodel.RowPoliciesKey); ok {
				out = append(out, c)
			}
		}
		if c, ok := keyChange(path, data, source.Child(doc.Root, "handover"), metamodel.SourceDeliveryOldKey, metamodel.SourceDeliveryKey); ok {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].offset < out[j].offset })
	return out
}

// hint is the change of the first line's editor hint to the 1.0 schema of
// its kind: a release URL of SpecArch's repository moves to the program's
// tag, and a relative path keeps its folder.
func hint(path string, data []byte, program string) (Change, bool) {
	if !bytes.HasPrefix(data, []byte(hintPrefix)) {
		return Change{}, false
	}
	end := bytes.IndexByte(data, '\n')
	if end < 0 {
		end = len(data)
	}
	old := strings.TrimRight(string(data[len(hintPrefix):end]), "\r")
	m := schemaFile.FindStringSubmatchIndex(old)
	if m == nil {
		return Change{}, false
	}
	next := old[:m[3]] + "-1.0" + old[m[6]:]
	if u := releaseURL.FindStringSubmatch(next); u != nil {
		next = u[1] + "v" + program + u[3] + next[len(u[0]):]
	}
	if next == old {
		return Change{}, false
	}
	return Change{File: path, Line: 1, Column: len(hintPrefix) + 1, Old: old, New: next, offset: len(hintPrefix)}, true
}

// scalarChange rewrites a scalar value to value, keeping its quoting.
func scalarChange(path string, data []byte, n *yaml.Node, value string) (Change, bool) {
	at, ok := offsetOf(data, n.Line, n.Column)
	if !ok {
		return Change{}, false
	}
	old := token(data[at:])
	if old == "" {
		return Change{}, false
	}
	next := value
	switch old[0] {
	case '"':
		next = `"` + value + `"`
	case '\'':
		next = "'" + value + "'"
	}
	return Change{File: path, Line: n.Line, Column: n.Column, Old: old, New: next, offset: at}, true
}

// keyChange renames the key old of the mapping obj to key, keeping its
// quoting, unless obj gives both: that is a step left to the author,
// reported by validate.
func keyChange(path string, data []byte, obj *yaml.Node, old, key string) (Change, bool) {
	k := source.Key(obj, old)
	if k == nil || source.Child(obj, key) != nil {
		return Change{}, false
	}
	at, ok := offsetOf(data, k.Line, k.Column)
	if !ok {
		return Change{}, false
	}
	text := token(data[at:])
	if text != old && text != `"`+old+`"` && text != "'"+old+"'" {
		return Change{}, false // not the key as written: one the reader added
	}
	next := key
	if text[0] == '"' || text[0] == '\'' {
		next = text[:1] + key + text[:1]
	}
	return Change{File: path, Line: k.Line, Column: k.Column, Old: text, New: next, offset: at}, true
}

// offsetOf is the byte offset of a line and a column counted in
// characters, both from 1.
func offsetOf(data []byte, line, column int) (int, bool) {
	at := 0
	for l := 1; l < line; l++ {
		i := bytes.IndexByte(data[at:], '\n')
		if i < 0 {
			return 0, false
		}
		at += i + 1
	}
	for c := 1; c < column; c++ {
		if at >= len(data) || data[at] == '\n' {
			return 0, false
		}
		_, size := utf8.DecodeRune(data[at:])
		at += size
	}
	return at, at < len(data)
}

// token is the scalar that starts the text: a double-quoted one with its
// escapes, a single-quoted one with its doubled quotes, or a plain one up
// to the space, comment, colon or flow indicator that ends it.
func token(text []byte) string {
	if len(text) == 0 {
		return ""
	}
	switch text[0] {
	case '"':
		for i := 1; i < len(text); i++ {
			switch text[i] {
			case '\\':
				i++
			case '"':
				return string(text[:i+1])
			case '\n':
				return ""
			}
		}
		return ""
	case '\'':
		for i := 1; i < len(text); i++ {
			if text[i] == '\'' {
				if i+1 < len(text) && text[i+1] == '\'' {
					i++
					continue
				}
				return string(text[:i+1])
			}
			if text[i] == '\n' {
				return ""
			}
		}
		return ""
	}
	end := len(text)
	for i, b := range text {
		if b == '\n' || b == '\r' || b == ',' || b == '}' || b == ']' || b == ' ' && i+1 < len(text) && text[i+1] == '#' || b == ':' && (i+1 == len(text) || text[i+1] == ' ' || text[i+1] == '\n') {
			end = i
			break
		}
	}
	return strings.TrimRight(string(text[:end]), " \t")
}

// Apply makes the changes of one file to its text. The changes are the
// ones Changes gave for that text.
func Apply(data []byte, changes []Change) []byte {
	sorted := append([]Change(nil), changes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].offset > sorted[j].offset })
	out := append([]byte(nil), data...)
	for _, c := range sorted {
		out = append(out[:c.offset], append([]byte(c.New), out[c.offset+len(c.Old):]...)...)
	}
	return out
}
