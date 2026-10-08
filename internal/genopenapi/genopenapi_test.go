package genopenapi

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// request builds the request specarch would send for a specification.
func request(t *testing.T, dir string) *Request {
	t.Helper()
	s := spec.Load(dir)
	if len(s.Problems) > 0 {
		t.Fatalf("the specification does not load: %v", s.Problems)
	}
	var impls []map[string]any
	for _, impl := range s.Implementations {
		root := source.Parse(impl.Data).Root
		var idioms []map[string]any
		for _, u := range validate.IdiomUses(impl.Path, root, s) {
			i := map[string]any{"name": u.Name, "version": u.Version, "as": u.As}
			if u.As == "shipped" {
				i["content"] = source.ValueOf(validate.ShippedIdioms()[u.Name].Root)
			}
			idioms = append(idioms, i)
		}
		impls = append(impls, map[string]any{"file": impl.Path, "content": source.ValueOf(root), "idioms": idioms})
	}
	data, err := json.Marshal(map[string]any{"specarch": "0.1", "target": "openapi", "root": filepath.ToSlash(s.RootFile),
		"specification": s.Value, "implementations": impls, "output": filepath.ToSlash(filepath.Join(dir, "..", "openapi"))})
	if err != nil {
		t.Fatal(err)
	}
	r, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// at walks a parsed document by keys and sequence indexes.
func at(t *testing.T, n any, keys ...any) any {
	t.Helper()
	for _, k := range keys {
		switch key := k.(type) {
		case string:
			m, ok := n.(map[string]any)
			if !ok {
				t.Fatalf("%v: not a mapping at %q", keys, key)
			}
			n = m[key]
		case int:
			l, ok := n.([]any)
			if !ok || key >= len(l) {
				t.Fatalf("%v: no item %d", keys, key)
			}
			n = l[key]
		}
	}
	return n
}

// TestLibraryLending generates the example's document and checks what the
// standard dialect promises: OpenAPI 3.1, a list paged by the idiom's
// names in the idiom's envelope, problem documents for the refusals, the
// audit fields, and SpecArch's own keywords as extensions.
func TestLibraryLending(t *testing.T) {
	resp := Generate(request(t, "../../examples/library-lending/spec"))
	if len(resp.Diagnostics) > 0 || len(resp.Files) != 1 {
		t.Fatalf("want one file and no diagnostics, got %d files and %v", len(resp.Files), resp.Diagnostics)
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(resp.Files[0].Content), &doc); err != nil {
		t.Fatalf("the document is not YAML: %v", err)
	}
	if v := at(t, doc, "openapi"); v != "3.1.0" {
		t.Errorf("openapi is %v", v)
	}
	var names []string
	for _, p := range at(t, doc, "paths", "/loans", "get", "parameters").([]any) {
		names = append(names, at(t, p, "name").(string))
	}
	if got := strings.Join(names, " "); got != "status memberId sort page pageSize" {
		t.Errorf("listLoans takes %s", got)
	}
	env := at(t, doc, "paths", "/loans", "get", "responses", "200", "content", "application/json", "schema", "properties")
	for _, k := range []string{"items", "totalItems", "totalPages"} {
		if at(t, env, k) == nil {
			t.Errorf("the envelope has no %s", k)
		}
	}
	if at(t, doc, "paths", "/loans", "post", "responses", "409", "content", "application/problem+json", "schema", "$ref") != "#/components/schemas/Problem" {
		t.Error("createLoan's 409 is not a problem document")
	}
	if at(t, doc, "paths", "/loans", "post", "responses", "409", "x-specarch-problem") != "lending-refused" {
		t.Error("createLoan's 409 does not name its problem type")
	}
	member := at(t, doc, "components", "schemas", "Member", "properties")
	for _, f := range []string{"createdAt", "createdBy", "lastModifiedAt", "lastModifiedBy"} {
		if at(t, member, f, "readOnly") != true {
			t.Errorf("Member has no read-only %s", f)
		}
	}
	if at(t, member, "email", "x-specarch-sensitivity") != "personal" {
		t.Error("Member.email does not carry its sensitivity")
	}
	if at(t, doc, "components", "x-specarch-problems", "loan-closed", "status") != 409 {
		t.Error("the problem catalogue is missing")
	}
}

// TestListByPost puts a list's paging into its request body, and refuses a
// body that is a reference to an entity.
func TestListByPost(t *testing.T) {
	r := request(t, "../../examples/library-lending/spec")
	paths := r.Specification["paths"].(map[string]any)
	post := paths["/loans"].(map[string]any)["get"].(map[string]any)
	delete(paths["/loans"].(map[string]any), "get")
	paths["/loans/search"] = map[string]any{"post": post}
	resp := Generate(r)
	if len(resp.Diagnostics) > 0 {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(resp.Files[0].Content), &doc); err != nil {
		t.Fatal(err)
	}
	props := at(t, doc, "paths", "/loans/search", "post", "requestBody", "content", "application/json", "schema", "properties")
	for _, k := range []string{"sort", "page", "pageSize"} {
		if at(t, props, k) == nil {
			t.Errorf("the body has no %s", k)
		}
	}

	post["requestBody"] = map[string]any{"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/entities/Loan"}}}}
	resp = Generate(r)
	if len(resp.Diagnostics) != 1 || !strings.Contains(resp.Diagnostics[0].Message, "not an inline object") {
		t.Errorf("want one diagnostic about the body, got %v", resp.Diagnostics)
	}
}
