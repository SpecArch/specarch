package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// TestPluginRequestIdioms checks that each implementation file in a
// plug-in's request carries the idioms it uses, with the content of the
// idiom that applies, so a plug-in renders through them without reading
// the disk.
func TestPluginRequestIdioms(t *testing.T) {
	s := spec.Load("../../examples/library-lending/spec")
	l := loaded{spec: s}
	for _, impl := range s.Implementations {
		// The Go file names the openapi target; the TypeScript file
		// builds the web screens only.
		if strings.HasSuffix(impl.Path, ".go.specarch-implementation.yaml") {
			l.impls = append(l.impls, generate.Implementation{Node: source.Parse(impl.Data).Root, Path: impl.Path})
		}
	}
	req, notes := newPluginRequest(l, "openapi", "out", l.impls)
	if len(notes) > 0 {
		t.Errorf("want no notes, got %q", notes)
	}
	if len(req.Implementations) != 1 {
		t.Fatalf("want one implementation file, got %d", len(req.Implementations))
	}
	got := map[string]pluginIdiom{}
	for _, i := range req.Implementations[0].Idioms {
		got[i.Name] = i
	}
	for _, name := range []string{"paginated-list", "type-rendering"} {
		i, ok := got[name]
		if !ok {
			t.Errorf("the request carries no %s", name)
			continue
		}
		if i.As != "shipped" || i.Content == nil {
			t.Errorf("%s is %s with content %v; want shipped, with the shipped idiom", name, i.As, i.Content != nil)
		}
	}
}

// TestExistingFilesNamesWhatItCannotRead checks that an output folder not
// made yet holds nothing and says nothing, and that a file in it that
// cannot be read is named with the reason instead of being dropped.
func TestExistingFilesNamesWhatItCannotRead(t *testing.T) {
	dir := t.TempDir()
	files, notes := existingFiles(filepath.Join(dir, "not-made-yet"))
	if len(files) != 0 || len(notes) != 0 {
		t.Fatalf("a folder not made yet: got %d files and notes %q, want none", len(files), notes)
	}
	if err := os.WriteFile(filepath.Join(dir, "kept.sql"), []byte("select 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(dir, "locked.sql")
	if err := os.WriteFile(locked, []byte("select 2;\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(locked); err == nil {
		t.Skip("this user can read a file with mode 000, so the case cannot be made")
	}
	files, notes = existingFiles(dir)
	if len(files) != 1 || files[0].Path != "kept.sql" {
		t.Errorf("got files %v, want kept.sql only", files)
	}
	want := "cannot read " + locked + " (permission denied), so the plug-in is not told it is in the output folder"
	if len(notes) != 1 || notes[0] != want {
		t.Errorf("got notes %q, want [%q]", notes, want)
	}
}

// TestAnswerStartQuotesAndCuts checks that what a plug-in wrote instead of
// JSON is shown quoted, and cut after 400 bytes.
func TestAnswerStartQuotesAndCuts(t *testing.T) {
	if got := answerStart([]byte("usage: x\n")); got != `"usage: x\n"` {
		t.Errorf("short answer: got %s", got)
	}
	long := answerStart([]byte(strings.Repeat("a", 500)))
	if !strings.HasSuffix(long, `"...`) || len(long) != 400+2+3 {
		t.Errorf("long answer: got %d bytes ending %q", len(long), long[len(long)-5:])
	}
}
