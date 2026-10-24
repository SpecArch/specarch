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

// In the dxlib dialect the sign-in callback is warned of at
// /signIn/callback, naming the GET route the provider comes back on, since
// the dialect serves every operation as a POST (ADR-182); a generator that
// calls the standard dialect only is refused there already.
func TestSignInCallbackDxlib(t *testing.T) {
	spec := map[string]any{
		"signIn": map[string]any{"callback": "finishSignIn"},
		"paths":  map[string]any{"/auth/callback": map[string]any{"get": map[string]any{"operationId": "finishSignIn"}}},
	}
	warned := func(dialect string, seals bool) []string {
		g := New(&Request{Specification: spec, APIDialect: dialect}, Profile{Name: "specarch-gen-ui-typescript", Seals: seals})
		var out []string
		for _, d := range g.Diags {
			if d.Path == "/signIn/callback" {
				out = append(out, d.Severity+": "+d.Message)
			}
		}
		return out
	}
	got := warned("dxlib", true)
	if len(got) != 1 || !strings.HasPrefix(got[0], "warning: ") || !strings.Contains(got[0], "adds a GET route at /auth/callback") || !strings.Contains(got[0], "POST /finishSignIn") {
		t.Fatalf("the dxlib dialect: %v", got)
	}
	if got := warned("", true); len(got) != 0 {
		t.Fatalf("the standard dialect: %v", got)
	}
	if got := warned("dxlib", false); len(got) != 0 {
		t.Fatalf("a generator refused the dxlib dialect: %v", got)
	}
}
