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

// valueFixture is a specification whose member holds value objects.
const valueFixture = "../../spec/tests/generate-go-dxlib-value-objects/project"

// envelopeFixture is a specification with a handshake and streamed files.
const envelopeFixture = "../../spec/tests/generate-go-dxlib-envelope-stream/project"

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
	return generateFrom(t, fixture, change, owned)
}

// generateFrom runs the generator on the specification in a folder.
func generateFrom(t *testing.T, fixture string, change func(specification map[string]any), owned []string) genopenapi.Response {
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
	data, err := json.Marshal(map[string]any{"specarch": "0.3", "target": "go-dxlib", "root": filepath.ToSlash(s.RootFile),
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
	if len(resp.Files) != 2 {
		t.Fatalf("want the service file and the client, got %d and %v", len(resp.Files), resp.Diagnostics)
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

var bodyCall = regexp.MustCompile(`return (body([A-Za-z0-9]+))\(aepr, r(, UploadStream\(aepr, [0-9]+\))?\)`)

// TestCompiles builds the file of each fixture against dxlib, beside the
// bodies and jobs a service writes. It needs a dxlib checkout, named by
// SPECARCH_DXLIB, and is skipped without one.
func TestCompiles(t *testing.T) {
	dxlib := os.Getenv("SPECARCH_DXLIB")
	if dxlib == "" {
		t.Skip("SPECARCH_DXLIB names no dxlib checkout, so the file cannot be compiled against dxlib here")
	}
	for _, f := range []string{fixture, valueFixture, envelopeFixture} {
		t.Run(filepath.Base(filepath.Dir(f)), func(t *testing.T) {
			compiles(t, dxlib, generateFrom(t, f, func(map[string]any) {}, nil).Files)
		})
	}
}

var packageName = regexp.MustCompile(`(?m)^package (\w+)$`)

func compiles(t *testing.T, dxlib string, generated []genopenapi.File) {
	src := generated[0].Content
	pkg := packageName.FindStringSubmatch(src)[1]
	var stub strings.Builder
	stub.WriteString("package " + pkg + "\n\nimport (\n\t\"io\"\n\n\t\"github.com/donnyhardyanto/dxlib/api\"\n\t\"github.com/donnyhardyanto/dxlib/task\"\n)\n\nvar _ = api.DXAPI{}\n\nvar _ = task.DXTask{}\n\nvar _ io.Reader\n\n")
	for _, m := range bodyCall.FindAllStringSubmatch(src, -1) {
		if m[3] != "" {
			fmt.Fprintf(&stub, "func %s(aepr *api.DXAPIEndPointRequest, r %sRequest, body io.Reader) error { return nil }\n", m[1], m[2])
			continue
		}
		fmt.Fprintf(&stub, "func %s(aepr *api.DXAPIEndPointRequest, r %sRequest) error { return nil }\n", m[1], m[2])
	}
	for _, m := range regexp.MustCompile(`(job[A-Z][A-Za-z0-9]*)\)`).FindAllStringSubmatch(src, -1) {
		fmt.Fprintf(&stub, "func %s(t *task.DXTask) error { return nil }\n", m[1])
	}
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":            "module " + pkg + "\n\ngo 1.27.1\n\nrequire github.com/donnyhardyanto/dxlib v0.0.0\n\nreplace github.com/donnyhardyanto/dxlib => " + dxlib + "\n",
		"service_bodies.go": stub.String(),
	}
	for _, f := range generated {
		files[f.Path] = f.Content
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
	if len(resp.Files) != 2 || !strings.Contains(resp.Files[0].Content, `"book", "book", "book_row", "", "id",`) {
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
	if len(r.Files) != 2 {
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

// TestRoleNames writes each role's name id as it goes on the wire when the
// specification names its roles in upper snake case (ADR-099).
func TestRoleNames(t *testing.T) {
	resp := generateWith(t, func(sp map[string]any) {
		sp["info"].(map[string]any)["roleNames"] = "upper_snake"
		sp["roles"].(map[string]any)["desk-supervisor"] = map[string]any{"description": "Supervises the desk.", "permissions": []any{"books.read"}}
	})
	if len(resp.Files) != 2 {
		t.Fatalf("want the service file and the client, got %d and %v", len(resp.Files), resp.Diagnostics)
	}
	src := resp.Files[0].Content
	for _, want := range []string{
		`{"DESK", "The desk.", []string{"books.read", "books.write"}}`,
		`{"DESK_SUPERVISOR", "Supervises the desk.", []string{"books.read"}}`,
		"// wire (info.roleNames upper_snake).",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the file has no %q", want)
		}
	}
}

// TestImplies writes after each role's own privileges the ones they imply,
// since the check reads each privilege by its name (ADR-100).
func TestImplies(t *testing.T) {
	resp := generateWith(t, func(sp map[string]any) {
		perms := sp["permissions"].(map[string]any)
		perms["books.all"] = map[string]any{"description": "Every call on books.", "implies": []any{"books.write"}}
		perms["books.write"].(map[string]any)["implies"] = []any{"books.read"}
		sp["roles"].(map[string]any)["desk-supervisor"] = map[string]any{"description": "Supervises the desk.", "permissions": []any{"books.all"}}
	})
	if len(resp.Files) != 2 {
		t.Fatalf("want the service file and the client, got %d and %v", len(resp.Files), resp.Diagnostics)
	}
	src := resp.Files[0].Content
	for _, want := range []string{
		`{"desk", "The desk.", []string{"books.read", "books.write"}}`,
		`{"desk-supervisor", "Supervises the desk.", []string{"books.all", "books.read", "books.write"}}`,
		"// and after them the ones those imply, since the check reads each",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the file has no %q", want)
		}
	}
}

// TestMaps reads a map of text with dxlib's own map getter, any other
// typed map as JSON decoded into map[string]T, a map whose values may be
// null into pointers, and a map of any values as JSON, as dxlib passes it
// through.
func TestMaps(t *testing.T) {
	resp := generateWith(t, func(s map[string]any) {
		book := s["entities"].(map[string]any)["Book"].(map[string]any)
		props := book["properties"].(map[string]any)
		props["labels"] = map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}}
		props["counts"] = map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer", "format": "int32"}}
		props["ratings"] = map[string]any{"type": "object", "additionalProperties": map[string]any{"type": []any{"number", "null"}, "format": "double"}}
		props["extra"] = map[string]any{"type": "object", "additionalProperties": true}
	})
	if len(resp.Files) != 2 {
		t.Fatalf("want the service file and the client, got %d and %v", len(resp.Files), resp.Diagnostics)
	}
	src := resp.Files[0].Content
	for _, want := range []string{
		`aepr.GetParameterValueAsMapStringString("labels")`,
		`readValue[map[string]int32](aepr, "counts")`,
		`readValue[map[string]*float64](aepr, "ratings")`,
		`aepr.GetParameterValueAsJSON("extra")`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the file has no %q", want)
		}
	}
	for _, field := range []string{`Labels +map\[string\]string`, `Counts +map\[string\]int32`, `Ratings +map\[string\]\*float64`, `Extra +utils\.JSON`} {
		if !regexp.MustCompile(`\t` + field + `\n`).MatchString(src) {
			t.Errorf("the request has no field %s", field)
		}
	}
	if dxlib := os.Getenv("SPECARCH_DXLIB"); dxlib != "" {
		compiles(t, dxlib, resp.Files)
	}
}

// TestStreamDeclared checks that a download stream answers the status the
// design gives its file and the client sends an upload with the media type
// the design gives it, in place of 200 and octet-stream (ADR-146).
func TestStreamDeclared(t *testing.T) {
	resp := generateFrom(t, envelopeFixture, func(s map[string]any) {
		paths := s["paths"].(map[string]any)
		download := paths["/attachments/download"].(map[string]any)["post"].(map[string]any)
		responses := download["responses"].(map[string]any)
		responses["206"] = responses["200"]
		delete(responses, "200")
		upload := paths["/attachments"].(map[string]any)["post"].(map[string]any)
		upload["requestBody"].(map[string]any)["content"] = map[string]any{"image/png": map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}}}
	}, nil)
	if len(resp.Files) != 2 {
		t.Fatalf("want the service file and the client, got %d and %v", len(resp.Files), resp.Diagnostics)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == "error" {
			t.Errorf("unexpected error: %v", d)
		}
	}
	service, client := resp.Files[0].Content, resp.Files[1].Content
	for _, want := range []string{"\tDownloadAttachmentStatus    = 206\n", "\tDownloadAttachmentMediaType = \"application/octet-stream\"\n", "\tExportVaultStatus    = 200\n", "\tw.WriteHeader(status)\n"} {
		if !strings.Contains(service, want) {
			t.Errorf("the service file has no %q", want)
		}
	}
	if !strings.Contains(client, `return c.upload(ctx, "uploadAttachment", "image/png", parameters, r, size)`) {
		t.Error("the client does not send the upload as image/png, the media type the design gives it")
	}
	if dxlib := os.Getenv("SPECARCH_DXLIB"); dxlib != "" {
		compiles(t, dxlib, resp.Files)
	}
}
