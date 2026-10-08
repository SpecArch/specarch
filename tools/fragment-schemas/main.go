// Command fragment-schemas writes the schema of each kind of fragment file
// from the design schema, and points every fragment of the repository's
// own specifications at its schema.
//
// A tree's files each hold one stage's sections, and the design schema
// describes the merged document, so an editor checking one file against it
// reports what the other files hold as missing. Each fragment schema keeps
// only that stage's sections, copied from the design schema with the
// definitions they reach, so there is one definition to maintain:
//
//   - specarch-fragment-<stage>-<version>.schema.json for a file under the
//     requirements, design, deployment, commissioning or operation folder;
//   - specarch-fragment-test-<version>.schema.json for tests/<name>/test.yaml;
//   - specarch-fragment-questions-<version>.schema.json for a file directly
//     under tests/ or implementation/, which holds only questions.
//
// Run from the repository root:
//
//	go run ./tools/fragment-schemas          write the schemas and the hint lines
//	go run ./tools/fragment-schemas -check   exit 1 when either is not current
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/spec"
)

// designFile is the design schema, under schema/.
const designFile = "specarch-design-0.1.schema.json"

// Trees are the repository's own specifications whose fragments carry a
// hint line.
var Trees = []string{"spec", "examples/library-lending/spec", "examples/lending-desk/spec"}

// hintPrefix starts the line that tells an editor a file's schema.
const hintPrefix = "# yaml-language-server: $schema="

func main() {
	check := flag.Bool("check", false, "report what is not current and exit 1, writing nothing")
	flag.Parse()
	if err := run(".", *check, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "fragment-schemas:", err)
		os.Exit(1)
	}
}

// run writes, or with check compares, the schemas and the hint lines under
// the repository root.
func run(root string, check bool, out io.Writer) error {
	want, err := Expected(root)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(want))
	for p := range want {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	stale := 0
	for _, p := range paths {
		have, err := os.ReadFile(filepath.Join(root, p))
		if err == nil && bytes.Equal(have, want[p]) {
			continue
		}
		if check {
			fmt.Fprintf(out, "not current: %s\n", p)
			stale++
			continue
		}
		if err := os.WriteFile(filepath.Join(root, p), want[p], 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "wrote %s\n", p)
	}
	if stale > 0 {
		return fmt.Errorf("%d files are not current; run go run ./tools/fragment-schemas", stale)
	}
	return nil
}

// Expected gives every file this command writes, by its path from the
// repository root: the fragment schemas, and each fragment with its hint.
func Expected(root string) (map[string][]byte, error) {
	design, err := os.ReadFile(filepath.Join(root, "schema", designFile))
	if err != nil {
		return nil, err
	}
	schemas, err := Schemas(design)
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for name, data := range schemas {
		out["schema/"+name] = data
	}
	for _, tree := range Trees {
		frags, err := Fragments(filepath.Join(root, tree))
		if err != nil {
			return nil, err
		}
		for _, f := range frags {
			name, ok := schemaFor(f.Kind, schemas)
			if !ok {
				return nil, fmt.Errorf("no schema for %s", f.Kind)
			}
			data, err := os.ReadFile(filepath.Join(root, tree, f.Path))
			if err != nil {
				return nil, err
			}
			depth := strings.Count(filepath.ToSlash(filepath.Join(tree, f.Path)), "/")
			out[filepath.ToSlash(filepath.Join(tree, f.Path))] = WithHint(data, strings.Repeat("../", depth)+"schema/"+name)
		}
	}
	return out, nil
}

// schemaFor names the schema file of a fragment kind.
func schemaFor(kind string, schemas map[string][]byte) (string, bool) {
	for name := range schemas {
		if strings.HasPrefix(name, "specarch-fragment-"+kind+"-") {
			return name, true
		}
	}
	return "", false
}

// WithHint gives a file's content with its first line the hint naming
// schema, replacing a hint already there.
func WithHint(data []byte, schema string) []byte {
	if bytes.HasPrefix(data, []byte(hintPrefix)) {
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			data = data[i+1:]
		} else {
			data = nil
		}
	}
	return append([]byte(hintPrefix+schema+"\n"), data...)
}

// Fragment is one fragment file of a tree and the kind of schema it takes:
// a stage name, "test" or "questions".
type Fragment struct {
	Path string // from the tree's folder, with slashes
	Kind string
}

// Fragments lists the fragment files of the tree at dir, in path order:
// every file the loader reads but the root file and the implementation
// files, which have schemas of their own.
func Fragments(dir string) ([]Fragment, error) {
	var out []Fragment
	for _, stage := range spec.Stages {
		folder := filepath.Join(dir, stage)
		if _, err := os.Stat(folder); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(folder, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if p != folder && strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".yaml") {
				return nil
			}
			rel, _ := filepath.Rel(dir, p)
			rel = filepath.ToSlash(rel)
			parts := strings.Split(rel, "/")
			switch {
			case stage != "tests" && stage != "implementation":
				out = append(out, Fragment{rel, stage})
			case len(parts) == 2:
				out = append(out, Fragment{rel, "questions"})
			case stage == "tests" && len(parts) == 3 && parts[2] == spec.TestFile:
				out = append(out, Fragment{rel, "test"})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Schemas writes the fragment schemas from the design schema, by file name.
func Schemas(design []byte) (map[string][]byte, error) {
	d, err := parse(design)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", designFile, err)
	}
	props := d.get("properties")
	defs := d.get("$defs")
	version, _ := props.get("specarch").get("const").string()
	id, _ := d.get("$id").string()
	if props == nil || defs == nil || version == "" || !strings.HasSuffix(id, "/"+designFile) {
		return nil, fmt.Errorf("%s: no properties, $defs, specarch const or $id naming it", designFile)
	}
	base := strings.TrimSuffix(id, designFile)
	out := map[string][]byte{}
	add := func(kind, title, description string, body []member) error {
		name := "specarch-fragment-" + kind + "-" + version + ".schema.json"
		s := &value{object: true}
		s.set("$schema", str("https://json-schema.org/draft/2020-12/schema"))
		s.set("$id", str(base+name))
		s.set("title", str(title+", meta-model "+version))
		s.set("description", str(description+" The validator merges every file of a tree into one document, which "+designFile+" describes, and checks what spans files, such as references and names defined twice; this schema checks one file alone. Written by tools/fragment-schemas from that schema; do not edit, run it."))
		for _, m := range body {
			s.set(m.key, m.value)
		}
		reached := map[string]bool{}
		if err := reach(s, defs, reached); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		used := &value{object: true}
		for _, m := range defs.members {
			if reached[m.key] {
				used.set(m.key, m.value)
			}
		}
		s.set("$defs", used)
		data, err := s.encode()
		if err != nil {
			return err
		}
		out[name] = data
		return nil
	}
	for _, stage := range spec.Stages {
		if stage == "tests" || stage == "implementation" {
			continue
		}
		sections := append(spec.SectionsOf(stage), spec.QuestionsSection)
		p := &value{object: true}
		for _, s := range sections {
			v := props.get(s)
			if v == nil {
				return nil, fmt.Errorf("%s has no section %s", designFile, s)
			}
			p.set(s, v)
		}
		err := add(stage, "SpecArch "+stage+" file",
			"One file under the "+stage+"/ folder of a SpecArch specification: a mapping of that stage's sections ("+strings.Join(sections, ", ")+"). A file under a sub-folder named after a section holds only that section.",
			[]member{{"type", str("object")}, {"properties", p}, {"additionalProperties", boolean(false)}})
		if err != nil {
			return nil, err
		}
	}
	tests := props.get("tests").get("additionalProperties")
	if tests == nil {
		return nil, fmt.Errorf("%s: tests has no additionalProperties", designFile)
	}
	body := []member{}
	for _, m := range tests.members {
		body = append(body, m)
	}
	if err := add("test", "SpecArch test file",
		"The test.yaml of a test folder tests/<name>/ in a SpecArch specification: one design test, named by its folder.",
		body); err != nil {
		return nil, err
	}
	questions := props.get(spec.QuestionsSection)
	if questions == nil {
		return nil, fmt.Errorf("%s has no section %s", designFile, spec.QuestionsSection)
	}
	q := &value{object: true}
	q.set(spec.QuestionsSection, questions)
	if err := add("questions", "SpecArch questions file",
		"A file directly under the tests/ or implementation/ folder of a SpecArch specification, whose other entries are folders: it holds only open questions.",
		[]member{{"type", str("object")}, {"properties", q}, {"additionalProperties", boolean(false)}}); err != nil {
		return nil, err
	}
	return out, nil
}

// reach marks every definition v refers to, and those they refer to.
func reach(v *value, defs *value, reached map[string]bool) error {
	if v == nil {
		return nil
	}
	for _, m := range v.members {
		// A key $ref holding an object is a property of that name, as
		// in a field's schema; only a string is a reference.
		if ref, ok := m.value.string(); ok && m.key == "$ref" && v.object {
			name, ok := strings.CutPrefix(ref, "#/$defs/")
			if !ok {
				return fmt.Errorf("a $ref that is not to $defs: %s", ref)
			}
			if reached[name] {
				continue
			}
			def := defs.get(name)
			if def == nil {
				return fmt.Errorf("a $ref to a missing definition: %s", ref)
			}
			reached[name] = true
			if err := reach(def, defs, reached); err != nil {
				return err
			}
			continue
		}
		if err := reach(m.value, defs, reached); err != nil {
			return err
		}
	}
	return nil
}

// value is a JSON value that keeps its keys in order and its numbers and
// strings as written, so a definition is copied byte for byte.
type value struct {
	object  bool
	array   bool
	members []member // an object's members, or an array's items with no key
	raw     []byte   // a scalar, encoded
}

type member struct {
	key   string
	value *value
}

func (v *value) get(key string) *value {
	if v == nil || !v.object {
		return nil
	}
	for _, m := range v.members {
		if m.key == key {
			return m.value
		}
	}
	return nil
}

func (v *value) set(key string, x *value) {
	for i, m := range v.members {
		if m.key == key {
			v.members[i].value = x
			return
		}
	}
	v.members = append(v.members, member{key, x})
}

func (v *value) string() (string, bool) {
	if v == nil || v.object || v.array {
		return "", false
	}
	var s string
	if json.Unmarshal(v.raw, &s) != nil {
		return "", false
	}
	return s, true
}

func str(s string) *value {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return &value{raw: bytes.TrimSuffix(b.Bytes(), []byte("\n"))}
}

func boolean(b bool) *value {
	return &value{raw: fmt.Append(nil, b)}
}

func parse(data []byte) (*value, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decode(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("more than one value")
	}
	return v, nil
}

func decode(dec *json.Decoder) (*value, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := t.(type) {
	case json.Delim:
		v := &value{object: t == '{', array: t == '['}
		for dec.More() {
			key := ""
			if v.object {
				k, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key = k.(string)
			}
			x, err := decode(dec)
			if err != nil {
				return nil, err
			}
			v.members = append(v.members, member{key, x})
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return v, nil
	case string:
		return str(t), nil
	case json.Number:
		return &value{raw: []byte(t.String())}, nil
	case nil:
		return &value{raw: []byte("null")}, nil
	default:
		return &value{raw: fmt.Append(nil, t)}, nil
	}
}

// encode writes v as the schema files are written: two spaces of indent
// and a final newline.
func (v *value) encode() ([]byte, error) {
	var compact bytes.Buffer
	v.compact(&compact)
	var out bytes.Buffer
	if err := json.Indent(&out, compact.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

func (v *value) compact(b *bytes.Buffer) {
	switch {
	case v.object, v.array:
		open, close := byte('{'), byte('}')
		if v.array {
			open, close = '[', ']'
		}
		b.WriteByte(open)
		for i, m := range v.members {
			if i > 0 {
				b.WriteByte(',')
			}
			if v.object {
				b.Write(str(m.key).raw)
				b.WriteByte(':')
			}
			m.value.compact(b)
		}
		b.WriteByte(close)
	default:
		b.Write(v.raw)
	}
}
