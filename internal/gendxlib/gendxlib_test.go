package gendxlib

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/genopenapi"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

const fixture = "../../spec/tests/generate-go-dxlib/project"

// generate runs the generator on the fixture, with the go-dxlib target's
// settings as specarch passes them.
func generate(t *testing.T) genopenapi.Response {
	t.Helper()
	return generateWith(t, func(map[string]any) {})
}

// generateWith runs the generator on the fixture after a change to its
// specification.
func generateWith(t *testing.T, change func(specification map[string]any)) genopenapi.Response {
	t.Helper()
	return generateOwning(t, change, nil)
}

// generateOwning runs the generator on the fixture after a change to its
// specification, with the mappings at the pointers given marked as owned
// by another stakeholder.
func generateOwning(t *testing.T, change func(specification map[string]any), owned []string) genopenapi.Response {
	t.Helper()
	s := spec.Load(fixture)
	if len(s.Problems) > 0 {
		t.Fatalf("the fixture does not load: %v", s.Problems)
	}
	impl := s.Implementations[0]
	root := source.Parse(impl.Data).Root
	var idioms []map[string]any
	for _, u := range validate.IdiomUses(impl.Path, root, s) {
		i := map[string]any{"name": u.Name, "version": u.Version, "as": u.As}
		if u.As == "shipped" {
			i["content"] = source.ValueOf(validate.ShippedIdioms()[u.Name].Root)
		}
		idioms = append(idioms, i)
	}
	content := source.ValueOf(root).(map[string]any)
	settings := content["targets"].(map[string]any)["go-dxlib"].(map[string]any)["settings"]
	if len(owned) > 0 {
		mappings, _ := content["mappings"].(map[string]any)
		if mappings == nil {
			mappings = map[string]any{}
			content["mappings"] = mappings
		}
		for _, ptr := range owned {
			mappings[ptr] = map[string]any{"target": "another team's", "ownedBy": "owner"}
		}
	}
	change(s.Value.(map[string]any))
	data, err := json.Marshal(map[string]any{"specarch": "0.1", "target": "go-dxlib", "root": filepath.ToSlash(s.RootFile),
		"specification": s.Value, "output": filepath.Join(fixture, "..", "testdata"),
		"implementations": []any{map[string]any{"file": impl.Path, "content": content, "settings": settings, "idioms": idioms}}})
	if err != nil {
		t.Fatal(err)
	}
	r, err := genopenapi.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return Generate(r)
}

// TestParts checks the file holds the table, the handlers bound by
// operationId with their standard operations and checks, the seeds and
// the task.
func TestParts(t *testing.T) {
	resp := generate(t)
	if len(resp.Files) != 1 {
		t.Fatalf("want one file, got %d and %v", len(resp.Files), resp.Diagnostics)
	}
	src := resp.Files[0].Content
	for _, want := range []string{
		`Book: tables.NewDXTableSimple("catalogue", "book", "book", "", "", "id"`,
		`a.RegisterHandler("listBooks", ListBooks)`,
		`return Tables.Book.RequestSearchPagingList(aepr)`,
		`aepr.GetParameterValueAsDecimal("price")`,
		`utf8.RuneCountInString(r.Title) > 300`,
		`Tables.Book.SetInsertAuditFields(aepr, data)`,
		`Tables.Book.GetByUidNotDeletedAuto(aepr.Context, &aepr.Log, r.BookId)`,
		`{"desk", "The desk.", []string{"books.read", "books.write"}}`,
		`task.Manager.NewTask("sweepWithdrawn", "always", 3600, jobSweepWithdrawn)`,
		// A null and a left-out edition are one case in dxlib, not given,
		// and the design's default fills it.
		"if !r.HasEdition {\n\t\tr.Edition = 1\n\t}",
		`data["edition"] = r.Edition`,
		`aepr.GetParameterValueAsString("subtitle")`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the file has no %q", want)
		}
	}
	if strings.Contains(src, `{"public"`) {
		t.Error("public is seeded as a privilege")
	}
}

var bodyCall = regexp.MustCompile(`return (body([A-Za-z0-9]+))\(aepr, r\)`)

// TestCompiles builds the file against dxlib, beside the bodies and jobs a
// service writes. It needs a dxlib checkout, named by SPECARCH_DXLIB, and
// is skipped without one.
func TestCompiles(t *testing.T) {
	dxlib := os.Getenv("SPECARCH_DXLIB")
	if dxlib == "" {
		t.Skip("SPECARCH_DXLIB names no dxlib checkout, so the file cannot be compiled against dxlib here")
	}
	src := generate(t).Files[0].Content
	var stub strings.Builder
	stub.WriteString("package catalogue\n\nimport (\n\t\"github.com/donnyhardyanto/dxlib/api\"\n\t\"github.com/donnyhardyanto/dxlib/task\"\n)\n\nvar _ = api.DXAPI{}\n\n")
	for _, m := range bodyCall.FindAllStringSubmatch(src, -1) {
		fmt.Fprintf(&stub, "func %s(aepr *api.DXAPIEndPointRequest, r %sRequest) error { return nil }\n", m[1], m[2])
	}
	for _, m := range regexp.MustCompile(`(job[A-Z][A-Za-z0-9]*)\)`).FindAllStringSubmatch(src, -1) {
		fmt.Fprintf(&stub, "func %s(t *task.DXTask) error { return nil }\n", m[1])
	}
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":            "module catalogue\n\ngo 1.27.1\n\nrequire github.com/donnyhardyanto/dxlib v0.0.0\n\nreplace github.com/donnyhardyanto/dxlib => " + dxlib + "\n",
		FileName:            src,
		"service_bodies.go": stub.String(),
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
	vet := exec.Command("go", "vet", "./...")
	vet.Dir = dir
	if out, err := vet.CombinedOutput(); err != nil {
		t.Fatalf("the file does not compile against dxlib: %v\n%s", err, out)
	}
}

// TestListView pages a list over a view through the table's list view, and
// refuses two views of one entity.
func TestListView(t *testing.T) {
	listOf := func(sp map[string]any) map[string]any {
		for _, item := range sp["paths"].(map[string]any) {
			for _, op := range item.(map[string]any) {
				if m, ok := op.(map[string]any); !ok {
					continue // the path item's parameters
				} else if l, ok := m["listOf"].(map[string]any); ok {
					return l
				}
			}
		}
		t.Fatal("the fixture has no list")
		return nil
	}
	resp := generateWith(t, func(sp map[string]any) {
		sp["views"] = map[string]any{"BookRow": map[string]any{"from": "Book", "properties": map[string]any{}}}
		l := listOf(sp)
		delete(l, "entity")
		l["view"] = "BookRow"
	})
	if len(resp.Files) != 1 || !strings.Contains(resp.Files[0].Content, `"book", "book", "book_row", "", "id",`) {
		t.Errorf("the table does not page through book_row: %v\n%v", resp.Diagnostics, resp.Files)
	}
	resp = generateWith(t, func(sp map[string]any) {
		sp["views"] = map[string]any{"BookRow": map[string]any{"from": "Book"}, "ShelfRow": map[string]any{"from": "Book"}}
		l := listOf(sp)
		delete(l, "entity")
		l["view"] = "BookRow"
		sp["paths"].(map[string]any)["/shelf-books"] = map[string]any{"get": map[string]any{"operationId": "listShelfBooks", "permission": "public",
			"listOf":    map[string]any{"view": "ShelfRow", "pageSize": map[string]any{"default": json.Number("20"), "maximum": json.Number("100")}},
			"responses": map[string]any{"200": map[string]any{"description": "The books."}}}}
	})
	if len(resp.Diagnostics) != 1 || !strings.Contains(resp.Diagnostics[0].Message, "pages through one list view") {
		t.Errorf("two views of one entity should be refused once, got %v", resp.Diagnostics)
	}
	resp = generateWith(t, func(sp map[string]any) {
		sp["views"] = map[string]any{"BookRow": map[string]any{"from": "Book"}}
		sp["paths"].(map[string]any)["/shelf-books"] = map[string]any{"get": map[string]any{"operationId": "listShelfBooks", "permission": "public",
			"listOf":    map[string]any{"view": "BookRow", "pageSize": map[string]any{"default": json.Number("20"), "maximum": json.Number("100")}},
			"responses": map[string]any{"200": map[string]any{"description": "The books."}}}}
	})
	if len(resp.Diagnostics) != 1 || !strings.Contains(resp.Diagnostics[0].Message, "through itself and through the view BookRow") && !strings.Contains(resp.Diagnostics[0].Message, "through the view BookRow and through itself") {
		t.Errorf("a list of Book and a list of its view should be refused, got %v", resp.Diagnostics)
	}
}

// TestOwned checks that an owned operation gets no handler and an owned
// job no task, while the other operations and the table handles stay.
func TestOwned(t *testing.T) {
	r := generateOwning(t, func(map[string]any) {}, []string{"#/paths/~1books/post", "#/jobs/sweepWithdrawn", "#/entities/Book"})
	if len(r.Files) != 1 {
		t.Fatalf("got %d files and %v", len(r.Files), r.Diagnostics)
	}
	code := r.Files[0].Content
	for _, gone := range []string{`"createBook"`, "func CreateBook", "sweepWithdrawn", "SweepWithdrawn"} {
		if strings.Contains(code, gone) {
			t.Errorf("the owned element is still written: %s", gone)
		}
	}
	for _, kept := range []string{`"listBooks"`, `"getBook"`, "\tBook *tables."} {
		if !strings.Contains(code, kept) {
			t.Errorf("%s is missing", kept)
		}
	}
}
