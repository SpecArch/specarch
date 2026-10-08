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
