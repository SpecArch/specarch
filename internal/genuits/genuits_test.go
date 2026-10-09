package genuits

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/genui"
)

// Every key of the screens' own texts a component names has a text, so
// no screen shows a key in place of words.
func TestScreenStringsCoverComponents(t *testing.T) {
	used := regexp.MustCompile(`"(screens\.[A-Za-z.]+)"`)
	err := fs.WalkDir(screens, "screens", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, _ := screens.ReadFile(path)
		for _, m := range used.FindAllStringSubmatch(string(data), -1) {
			if m[1] == "screens.notice" {
				continue // the key Notice keeps a message under in the session's storage, not a text
			}
			if _, ok := screenStrings[m[1]]; !ok {
				t.Errorf("%s names %s, which has no text in screenStrings", path, m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A translation that misses a key fails, one that keeps a key no screen
// uses is warned of, and one that drops a placeholder fails.
func TestTranslations(t *testing.T) {
	keys := map[string]string{"a.title": "Members", "screens.minLength": "Use at least {count} characters."}
	run := func(content string) (*gen, bool) {
		g := &gen{settings: map[string]any{"language": "en", "translations": []any{"id"}}}
		_, _, ok := g.translations(keys, []genui.File{{Path: "strings.id.json", Content: content}})
		return g, ok
	}
	if g, ok := run(`{"a.title": "Anggota", "screens.minLength": "Paling sedikit {count} karakter."}`); !ok || len(g.diags) != 0 {
		t.Fatalf("a full translation: %v", g.diags)
	}
	g, ok := run(`{"a.title": "Anggota"}`)
	if ok || len(g.diags) != 1 || !strings.Contains(g.diags[0].Message, "no entry for screens.minLength") {
		t.Fatalf("a missing key: %v", g.diags)
	}
	g, ok = run(`{"a.title": "Anggota", "screens.minLength": "Paling sedikit karakter.", "b.old": "Lama"}`)
	if ok || len(g.diags) != 2 || g.diags[0].Severity != "warning" || !strings.Contains(g.diags[1].Message, "without {count}") {
		t.Fatalf("a stale key and a dropped placeholder: %v", g.diags)
	}
	g = &gen{settings: map[string]any{"language": "en", "translations": []any{"id"}}}
	if _, _, ok := g.translations(keys, nil); ok || !strings.Contains(g.diags[0].Message, "strings.id.json is not in the output folder") {
		t.Fatalf("a missing file: %v", g.diags)
	}
}
