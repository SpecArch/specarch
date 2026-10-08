package genui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// request builds the request specarch would send for the library lending
// example's ui target, after a change to its specification and its
// implementation file's content.
func request(t *testing.T, change func(specification, content map[string]any)) *Request {
	t.Helper()
	s := spec.Load("../../examples/library-lending/spec")
	if len(s.Problems) > 0 {
		t.Fatalf("the example does not load: %v", s.Problems)
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
	specification := s.Value.(map[string]any)
	change(specification, content)
	data, err := json.Marshal(map[string]any{"specarch": "0.1", "target": "ui", "root": filepath.ToSlash(s.RootFile),
		"specification": specification, "output": "../../examples/library-lending/web",
		"implementations": []any{map[string]any{"file": impl.Path, "content": content, "idioms": idioms}}})
	if err != nil {
		t.Fatal(err)
	}
	r, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestExample checks the generator writes exactly the screens committed in
// the example, the first of which was written by hand before it.
func TestExample(t *testing.T) {
	resp := Generate(request(t, func(map[string]any, map[string]any) {}))
	for _, d := range resp.Diagnostics {
		if d.Severity == "error" {
			t.Fatalf("diagnostics: %v", resp.Diagnostics)
		}
	}
	if len(resp.Files) != 6 {
		t.Fatalf("want six files, got %d", len(resp.Files))
	}
	for _, f := range resp.Files {
		want, err := os.ReadFile(filepath.Join("../../examples/library-lending/web", f.Path))
		if err != nil {
			t.Fatal(err)
		}
		if f.Content != string(want) {
			t.Errorf("%s differs from the example's", f.Path)
		}
	}
}

// TestRefusals refuses a target that is not plain JavaScript for the web,
// a target without a language, a filter its list cannot ask for, a list
// that does not page, and an API in the dxlib dialect.
func TestRefusals(t *testing.T) {
	ui := func(content map[string]any) map[string]any {
		return content["targets"].(map[string]any)["ui"].(map[string]any)
	}
	for _, c := range []struct {
		name   string
		change func(specification, content map[string]any)
		want   string
	}{
		{"framework", func(_, c map[string]any) { ui(c)["framework"] = "react" }, "plain-javascript only"},
		{"language", func(_, c map[string]any) { delete(ui(c)["settings"].(map[string]any), "language") }, "name no language"},
		{"filter", func(s, _ map[string]any) {
			pg := s["pages"].(map[string]any)["loans-list"].(map[string]any)
			pg["filters"] = []any{"dueOn"}
		}, "dueOn, which is neither a query parameter of listLoans"},
		{"paging", func(s, _ map[string]any) {
			get := s["paths"].(map[string]any)["/loans"].(map[string]any)["get"].(map[string]any)
			delete(get, "listOf")
		}, "listLoans has no listOf, so it does not page"},
		{"dxlib", func(_, c map[string]any) {
			c["targets"].(map[string]any)["openapi"].(map[string]any)["dialect"] = "dxlib"
		}, "the standard dialect only"},
	} {
		resp := Generate(request(t, c.change))
		found := false
		for _, d := range resp.Diagnostics {
			found = found || d.Severity == "error" && strings.Contains(d.Message, c.want)
		}
		if !found || len(resp.Files) != 0 {
			t.Errorf("%s: want an error with %q and no file, got %v", c.name, c.want, resp.Diagnostics)
		}
	}
}
