package genscreens

import (
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/genui"
)

// A translation that misses a key fails, one that keeps a key no screen
// uses is warned of, and one that drops a placeholder fails.
func TestTranslations(t *testing.T) {
	keys := map[string]string{"a.title": "Members", "screens.minLength": "Use at least {count} characters."}
	run := func(content string) (*Gen, bool) {
		g := &Gen{Settings: map[string]any{"language": "en", "translations": []any{"id"}}}
		_, _, ok := g.Translations(keys, []genui.File{{Path: "strings.id.json", Content: content}})
		return g, ok
	}
	if g, ok := run(`{"a.title": "Anggota", "screens.minLength": "Paling sedikit {count} karakter."}`); !ok || len(g.Diags) != 0 {
		t.Fatalf("a full translation: %v", g.Diags)
	}
	g, ok := run(`{"a.title": "Anggota"}`)
	if ok || len(g.Diags) != 1 || !strings.Contains(g.Diags[0].Message, "no entry for screens.minLength") {
		t.Fatalf("a missing key: %v", g.Diags)
	}
	g, ok = run(`{"a.title": "Anggota", "screens.minLength": "Paling sedikit karakter.", "b.old": "Lama"}`)
	if ok || len(g.Diags) != 2 || g.Diags[0].Severity != "warning" || !strings.Contains(g.Diags[1].Message, "without {count}") {
		t.Fatalf("a stale key and a dropped placeholder: %v", g.Diags)
	}
	g = &Gen{Settings: map[string]any{"language": "en", "translations": []any{"id"}}}
	if _, _, ok := g.Translations(keys, nil); ok || !strings.Contains(g.Diags[0].Message, "strings.id.json is not in the output folder") {
		t.Fatalf("a missing file: %v", g.Diags)
	}
}
