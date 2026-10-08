package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

const root = "../.."

// TestCopiesByteForByte holds the JSON reader and writer to the design
// schema's own form, so a definition copied into a fragment schema reads
// exactly as it does there.
func TestCopiesByteForByte(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(root, "schema", designFile))
	if err != nil {
		t.Fatal(err)
	}
	v, err := parse(data)
	if err != nil {
		t.Fatal(err)
	}
	got, err := v.encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("%s does not read and write back the same", designFile)
	}
}

// TestCurrent fails when the design schema changed and the fragment
// schemas, or a fragment's hint line, were not written again.
func TestCurrent(t *testing.T) {
	var out bytes.Buffer
	if err := run(root, true, &out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
}

// TestFragmentsValidate checks every fragment of the repository's own
// specifications against the schema its hint line names, as an editor
// does. A required key an open must question blocks is the one thing an
// editor reports that the validator does not: the question is the gap's
// record, and it may sit in another file, which one file's schema cannot
// see.
func TestFragmentsValidate(t *testing.T) {
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	compiled := map[string]*jsonschema.Schema{}
	count := 0
	for _, tree := range Trees {
		frags, err := Fragments(filepath.Join(root, tree))
		if err != nil {
			t.Fatal(err)
		}
		blocked := blockedKeys(spec.Load(filepath.Join(root, tree)).Root)
		for _, f := range frags {
			p := filepath.Join(root, tree, f.Path)
			data, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			line, _, _ := strings.Cut(string(data), "\n")
			ref, ok := strings.CutPrefix(line, hintPrefix)
			if !ok {
				t.Errorf("%s: no hint line", p)
				continue
			}
			if !strings.Contains(ref, "specarch-fragment-"+f.Kind+"-") {
				t.Errorf("%s: the hint names %s, not the %s schema", p, ref, f.Kind)
				continue
			}
			file := filepath.Clean(filepath.Join(filepath.Dir(p), ref))
			s := compiled[file]
			if s == nil {
				raw, err := os.ReadFile(file)
				if err != nil {
					t.Fatalf("%s: %v", p, err)
				}
				doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
				if err != nil {
					t.Fatal(err)
				}
				if err := c.AddResource(file, doc); err != nil {
					t.Fatal(err)
				}
				if s, err = c.Compile(file); err != nil {
					t.Fatal(err)
				}
				compiled[file] = s
			}
			parsed := source.Parse(data)
			if len(parsed.Problems) > 0 || parsed.Root == nil {
				t.Errorf("%s: does not parse: %v", p, parsed.Problems)
				continue
			}
			var prefix []string
			if f.Kind == "test" {
				prefix = []string{"tests", strings.Split(f.Path, "/")[1]}
			}
			if err := s.Validate(parsed.Value); err != nil && !excused(err.(*jsonschema.ValidationError), prefix, blocked) {
				t.Errorf("%s: %v", p, err)
			}
			count++
		}
	}
	if count == 0 {
		t.Fatal("no fragment was checked")
	}
	if len(compiled) != 7 {
		t.Errorf("the fragments used %d schemas, want all 7", len(compiled))
	}
}

// TestRefusesWhatTheLoaderRefuses holds each schema to the loader's rules
// for one file: a section of another stage, and in a test file a key
// that is not a test's.
func TestRefusesWhatTheLoaderRefuses(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(root, "schema", designFile))
	if err != nil {
		t.Fatal(err)
	}
	schemas, err := Schemas(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ kind, yaml string }{
		{"design", "stakeholders: {}\n"},
		{"requirements", "entities: {}\n"},
		{"design", "specarch: \"0.1\"\n"},
		{"questions", "requirements: {}\n"},
		{"test", "command: validate\nlevel: system\nscenario: red\ngiven: a\nwhen: b\nthen: c\nunknown: d\n"},
	} {
		name, _ := schemaFor(tc.kind, schemas)
		c := jsonschema.NewCompiler()
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemas[name]))
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddResource(name, doc); err != nil {
			t.Fatal(err)
		}
		s, err := c.Compile(name)
		if err != nil {
			t.Fatal(err)
		}
		if s.Validate(source.Parse([]byte(tc.yaml)).Value) == nil {
			t.Errorf("%s accepts %q", name, tc.yaml)
		}
	}
}

// blockedKeys gives the pointers an open must question of the merged
// specification blocks.
func blockedKeys(root *yaml.Node) map[string]bool {
	out := map[string]bool{}
	for _, q := range source.Pairs(source.Child(root, spec.QuestionsSection)) {
		if source.Str(source.Child(q.Value, "priority")) != "must" {
			continue
		}
		for _, item := range source.Items(source.Child(q.Value, "blocks")) {
			if b, ok := spec.ParseBlock(item.Value); ok && b.IsPointer() {
				out[source.Pointer(b.Tokens...)] = true
			}
		}
	}
	return out
}

// excused reports whether every failure under ve is a required key that
// is blocked, or sits under a blocked element.
func excused(ve *jsonschema.ValidationError, prefix []string, blocked map[string]bool) bool {
	if len(ve.Causes) > 0 {
		for _, c := range ve.Causes {
			if !excused(c, prefix, blocked) {
				return false
			}
		}
		return true
	}
	r, ok := ve.ErrorKind.(*kind.Required)
	if !ok {
		return false
	}
	at := append(append([]string{}, prefix...), ve.InstanceLocation...)
	for _, m := range r.Missing {
		covered := blocked[source.Pointer(append(at, m)...)]
		for i := 1; i <= len(at); i++ {
			covered = covered || blocked[source.Pointer(at[:i]...)]
		}
		if !covered {
			return false
		}
	}
	return true
}
