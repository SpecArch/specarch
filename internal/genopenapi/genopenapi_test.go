package genopenapi

import (
	"encoding/json"
	"path/filepath"
	"slices"
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

// withViews adds a view of loans with their member's name and a view of
// members with their loans counted, and lists loans through the first.
func withViews(r *Request) {
	r.Specification["views"] = map[string]any{
		"LoanRow": map[string]any{"from": "Loan", "description": "A loan as the desk's list shows it.", "properties": map[string]any{
			"memberName": map[string]any{"path": "member.fullName"},
			"memberTier": map[string]any{"path": "member.tier"}}},
		"MemberRow": map[string]any{"from": "Member", "properties": map[string]any{"openLoans": map[string]any{"count": "loans"}}},
	}
	get := r.Specification["paths"].(map[string]any)["/loans"].(map[string]any)["get"].(map[string]any)
	l := get["listOf"].(map[string]any)
	delete(l, "entity")
	l["view"] = "LoanRow"
	l["sortable"] = []any{"dueOn", "memberName"}
	for _, resp := range get["responses"].(map[string]any) {
		for _, c := range resp.(map[string]any)["content"].(map[string]any) {
			if s := c.(map[string]any)["schema"].(map[string]any); s["items"] != nil {
				s["items"] = map[string]any{"$ref": "#/views/LoanRow"}
			}
		}
	}
}

// TestViewRows writes a view's rows as a read-only array of the
// relation's target, which the view's schema requires.
func TestViewRows(t *testing.T) {
	resp := Generate(request(t, "../../examples/library-lending/spec"))
	if len(resp.Diagnostics) > 0 {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(resp.Files[0].Content), &doc); err != nil {
		t.Fatal(err)
	}
	loans := at(t, doc, "components", "schemas", "MemberWithLoans", "properties", "loans")
	if at(t, loans, "type") != "array" || at(t, loans, "readOnly") != true || at(t, loans, "items", "$ref") != "#/components/schemas/Loan" {
		t.Errorf("loans should be a read-only array of Loan: %v", loans)
	}
	if !slices.Contains(at(t, doc, "components", "schemas", "MemberWithLoans", "required").([]any), any("loans")) {
		t.Error("MemberWithLoans should require loans, which is empty when there are none")
	}
}

// TestViews writes a view as a read-only schema of its entity's fields and
// the ones it adds, and lets a list sort by an added field.
func TestViews(t *testing.T) {
	r := request(t, "../../examples/library-lending/spec")
	withViews(r)
	loan := r.Specification["entities"].(map[string]any)["Loan"].(map[string]any)
	var req []any
	for _, f := range loan["required"].([]any) {
		if f != "memberId" {
			req = append(req, f)
		}
	}
	loan["required"] = req
	resp := Generate(r)
	if len(resp.Diagnostics) > 0 {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(resp.Files[0].Content), &doc); err != nil {
		t.Fatal(err)
	}
	row := at(t, doc, "components", "schemas", "LoanRow")
	if at(t, row, "readOnly") != true || at(t, row, "x-specarch-view", "from") != "Loan" || at(t, row, "properties", "dueOn") == nil {
		t.Errorf("LoanRow is not a read-only view of Loan with its fields: %v", row)
	}
	name := at(t, row, "properties", "memberName")
	if at(t, name, "readOnly") != true || at(t, name, "maxLength") != 200 || at(t, name, "type", 1) != "null" {
		t.Errorf("memberName should be read-only text of 200 that may be null, since memberId may be left out: %v", name)
	}
	if at(t, row, "properties", "memberTier", "enum") == nil {
		t.Errorf("memberTier should carry its enum's values: %v", at(t, row, "properties", "memberTier"))
	}
	if at(t, row, "properties", "memberTier", "type", 1) != "null" {
		t.Error("memberTier may be null, as memberId may be left out")
	}

	// A path to a field its entity does not require may be null too.
	r = request(t, "../../examples/library-lending/spec")
	withViews(r)
	r.Specification["views"].(map[string]any)["LoanRow"].(map[string]any)["properties"].(map[string]any)["memberNote"] = map[string]any{"path": "member.membershipEndsOn"}
	member := r.Specification["entities"].(map[string]any)["Member"].(map[string]any)
	var mreq []any
	for _, f := range member["required"].([]any) {
		if f != "membershipEndsOn" {
			mreq = append(mreq, f)
		}
	}
	member["required"] = mreq
	resp2 := Generate(r)
	var doc2 map[string]any
	if err := yaml.Unmarshal([]byte(resp2.Files[0].Content), &doc2); err != nil {
		t.Fatal(err)
	}
	if at(t, doc2, "components", "schemas", "LoanRow", "properties", "memberName", "type") != "string" {
		t.Error("memberName may not be null when its relation and field are required")
	}
	if at(t, doc2, "components", "schemas", "LoanRow", "properties", "memberNote", "type", 1) != "null" {
		t.Error("memberNote reads a field Member does not require, so it may be null")
	}

	count := at(t, doc, "components", "schemas", "MemberRow", "properties", "openLoans")
	if at(t, count, "format") != "int64" || at(t, count, "minimum") != 0 {
		t.Errorf("openLoans should be a 64-bit count: %v", count)
	}
	if !strings.Contains(resp.Files[0].Content, "- -memberName") {
		t.Error("listLoans cannot sort by memberName")
	}
	if at(t, doc, "paths", "/loans", "get", "responses", "200", "content", "application/json", "schema", "properties", "items", "items", "$ref") != "#/components/schemas/LoanRow" {
		t.Error("listLoans does not answer LoanRow")
	}
}

// A schema is written under components.schemas as it is, and a body
// referring to it refers to that component.
func TestValueObjects(t *testing.T) {
	r := request(t, "../../examples/library-lending/spec")
	r.Specification["schemas"] = map[string]any{
		"Diagnostic": map[string]any{
			"type":        "object",
			"description": "One thing a check found.",
			"properties":  map[string]any{"text": map[string]any{"type": "string", "maxLength": json.Number("200")}},
			"required":    []any{"text"},
		},
	}
	resp := Generate(r)
	if len(resp.Diagnostics) > 0 {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(resp.Files[0].Content), &doc); err != nil {
		t.Fatal(err)
	}
	d := at(t, doc, "components", "schemas", "Diagnostic")
	for _, c := range []struct {
		keys []any
		want any
	}{{[]any{"type"}, "object"}, {[]any{"description"}, "One thing a check found."}, {[]any{"properties", "text", "maxLength"}, 200}, {[]any{"required", 0}, "text"}} {
		if got := at(t, d, c.keys...); got != c.want {
			t.Errorf("Diagnostic %v is %#v, not %#v", c.keys, got, c.want)
		}
	}
}
