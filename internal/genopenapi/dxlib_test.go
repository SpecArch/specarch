package genopenapi

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// dxlibDocument generates the dxlib dialect document of the
// generate-openapi-dxlib fixture.
func dxlibDocument(t *testing.T) (string, map[string]any) {
	t.Helper()
	resp := Generate(request(t, "../../spec/tests/generate-openapi-dxlib/project"))
	// The fixture's edition is required and may be null, which dxlib reads
	// as one case, not given: a warning on each place it is required, and
	// nothing else.
	warned := 0
	for _, d := range resp.Diagnostics {
		if d.Severity == "warning" && strings.HasSuffix(d.Path, "/properties/edition") {
			warned++
		} else {
			t.Errorf("unexpected diagnostic: %v", d)
		}
	}
	if len(resp.Files) != 1 || warned != 2 {
		t.Fatalf("want one file and the edition warning twice, got %d files and %v", len(resp.Files), resp.Diagnostics)
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(resp.Files[0].Content), &doc); err != nil {
		t.Fatalf("the document is not YAML: %v", err)
	}
	return resp.Files[0].Content, doc
}

// TestDxlibDialect checks what the dxlib reader needs: one POST per
// operation at /<operationId>, no constraint dxlib does not enforce (each
// listed as unenforced instead), a dxlib type on every field, no security
// scheme, privileges, dxlib's error body and list envelope.
func TestDxlibDialect(t *testing.T) {
	content, doc := dxlibDocument(t)
	for path, item := range at(t, doc, "paths").(map[string]any) {
		ops := item.(map[string]any)
		if len(ops) != 1 || ops["post"] == nil {
			t.Errorf("%s has %d methods; dxlib binds one POST per path", path, len(ops))
		}
		if id := at(t, ops, "post", "operationId"); "/"+id.(string) != path {
			t.Errorf("%s is not /<operationId> (%v)", path, id)
		}
		if at(t, ops, "post", "x-dxlib-endpoint-type") != "EndPointTypeHTTPJSON" {
			t.Errorf("%s carries no endpoint type", path)
		}
	}
	for _, refused := range []string{"maxLength:", "pattern:", "maximum:", "readOnly:", "securitySchemes", "security:", "format: uuid", "format: decimal"} {
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), refused) {
				t.Errorf("the document says %q, which the dxlib reader refuses: %s", refused, line)
			}
		}
	}
	title := at(t, doc, "paths", "/createBook", "post", "requestBody", "content", "application/json", "schema", "properties", "title")
	if at(t, title, "x-dxlib-type") != "non-empty-string" || at(t, title, "x-specarch-unenforced", 0) != "maxLength: 300" {
		t.Errorf("title is %v", title)
	}
	if at(t, doc, "paths", "/createBook", "post", "x-dxlib-privileges", 0) != "books.write" {
		t.Error("createBook carries no privilege")
	}
	if _, ok := at(t, doc, "paths", "/getBook", "post").(map[string]any)["x-dxlib-privileges"]; ok {
		t.Error("a public operation carries privileges")
	}
	if at(t, doc, "paths", "/getBook", "post", "responses", "404", "x-dxlib-response-name") != "book_not_found" ||
		at(t, doc, "paths", "/getBook", "post", "responses", "404", "content", "application/json", "schema", "properties", "reason_message") == nil {
		t.Error("the 404 is not dxlib's error body named by its problem type")
	}
	if at(t, doc, "paths", "/listBooks", "post", "responses", "200", "content", "application/json", "schema", "properties", "list", "properties", "rows") == nil {
		t.Error("the list is not in dxlib's list envelope")
	}
	if at(t, doc, "paths", "/listBooks", "post", "requestBody", "content", "application/json", "schema", "properties", "page_index", "minimum") != 0 {
		t.Error("page_index does not count from 0")
	}
	// In dxlib a null and a left-out parameter are one case, not given: a
	// field that may be null takes dxlib's nullable type and is not required.
	if at(t, doc, "components", "schemas", "Book", "properties", "subtitle", "x-dxlib-type") != "nullable-string" {
		t.Error("subtitle, which may be null, is not dxlib's nullable-string")
	}
	for _, r := range at(t, doc, "components", "schemas", "Book", "required").([]any) {
		if r == "edition" {
			t.Error("edition, which may be null, is required in the dxlib dialect")
		}
	}
}

// TestDxlibReader runs dxlib's own OpenAPI reader on the document. It needs
// a dxlib checkout, named by SPECARCH_DXLIB, and is skipped without one.
func TestDxlibReader(t *testing.T) {
	dxlib := os.Getenv("SPECARCH_DXLIB")
	if dxlib == "" {
		t.Skip("SPECARCH_DXLIB names no dxlib checkout, so dxlib's reader cannot run here")
	}
	content, _ := dxlibDocument(t)
	dir := t.TempDir()
	files := map[string]string{
		"go.mod": "module dxlibcheck\n\ngo 1.27.1\n\nrequire github.com/donnyhardyanto/dxlib v0.0.0\n\nreplace github.com/donnyhardyanto/dxlib => " + dxlib + "\n",
		"main.go": `package main

import (
	"fmt"
	"os"

	"github.com/donnyhardyanto/dxlib/api"
)

func main() {
	doc, err := api.ReadOpenAPIFile(os.Args[1])
	if err == nil {
		err = doc.Validate()
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
`,
		"openapi.yaml": content,
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Skipf("the dxlib module cannot be resolved here (%v): %s", err, out)
	}
	run := exec.Command("go", "run", ".", "openapi.yaml")
	run.Dir = dir
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("dxlib's reader refuses the document: %v\n%s", err, out)
	}
}

// TestDxlibViews writes a view in the dxlib dialect, after the entities,
// and a list over it sorts by the view's fields.
func TestDxlibViews(t *testing.T) {
	r := request(t, "../../examples/library-lending/spec")
	withViews(r)
	r.Implementations[0].Content["targets"].(map[string]any)["openapi"].(map[string]any)["dialect"] = "dxlib"
	paths := r.Specification["paths"].(map[string]any)
	delete(paths["/books"].(map[string]any), "get") // a bare list, which dxlib does not answer
	resp := Generate(r)
	for _, d := range resp.Diagnostics {
		if strings.HasPrefix(d.Path, "/views/") {
			t.Errorf("a view should add no diagnostic of its own: %v", d)
		}
	}
	if len(resp.Files) != 1 {
		t.Fatalf("no document: %v", resp.Diagnostics)
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(resp.Files[0].Content), &doc); err != nil {
		t.Fatal(err)
	}
	if at(t, doc, "components", "schemas", "LoanRow", "properties", "member_name", "x-dxlib-type") == nil {
		t.Errorf("LoanRow has no member_name: %v", at(t, doc, "components", "schemas", "LoanRow"))
	}
	if at(t, doc, "components", "schemas", "MemberRow", "properties", "open_loans", "x-dxlib-type") != "int64zp" {
		t.Errorf("open_loans should be dxlib's int64zp: %v", at(t, doc, "components", "schemas", "MemberRow", "properties", "open_loans"))
	}
	if !strings.Contains(resp.Files[0].Content, "member_name") || !strings.Contains(resp.Files[0].Content, "/listLoans") {
		t.Error("listLoans does not sort by member_name")
	}
}
