package genseed

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/SpecArch/specarch/internal/metamodel"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// request builds the request specarch would send for a conformance case's
// project, with the seed target's settings and the sql target's dialect.
func request(t *testing.T, project, dialect string, settings map[string]any) *Request {
	t.Helper()
	s := spec.Load(project)
	if len(s.Problems) > 0 {
		t.Fatalf("%s does not load: %v", project, s.Problems)
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
	content["targets"].(map[string]any)["sql"].(map[string]any)["dialect"] = dialect
	data, err := json.Marshal(map[string]any{"specarch": "1.0.0", "target": "seed", "root": filepath.ToSlash(s.RootFile),
		"specification": metamodel.Normalized(s.Value), "output": filepath.Join(project, "..", "seed"),
		"implementations": []any{map[string]any{"file": impl.Path, "content": content, "idioms": idioms, "settings": settings}},
		"existing":        []any{}})
	if err != nil {
		t.Fatal(err)
	}
	r, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

const caseProject = "../../spec/tests/generate-seed/project"

func seedOf(t *testing.T, r *Request) string {
	t.Helper()
	resp := Generate(r)
	for _, d := range resp.Diagnostics {
		if d.Severity == "error" {
			t.Fatalf("diagnostics: %v", resp.Diagnostics)
		}
	}
	if len(resp.Files) != 1 || resp.Files[0].Path != FileName {
		t.Fatalf("files: %v", resp.Files)
	}
	return resp.Files[0].Content
}

// The same request gives the same bytes twice, and another seed other
// rows: the file is deterministic and the seed is read.
func TestDeterministic(t *testing.T) {
	settings := map[string]any{"rows": map[string]any{"Author": 3, "Book": 5, "Review": 4}, "seed": 7}
	a := seedOf(t, request(t, caseProject, "postgresql", settings))
	b := seedOf(t, request(t, caseProject, "postgresql", settings))
	if a != b {
		t.Fatal("two runs differ")
	}
	settings["seed"] = 8
	if c := seedOf(t, request(t, caseProject, "postgresql", settings)); c == a {
		t.Fatal("another seed gives the same rows")
	}
	if !strings.Contains(a, "seed 7.") || strings.Count(a, "INSERT INTO author ") != 3 || strings.Count(a, "INSERT INTO books ") != 5 {
		t.Fatalf("header or counts:\n%s", a)
	}
}

// Every dialect writes the file, each with its own literals.
func TestDialects(t *testing.T) {
	settings := map[string]any{"rows": map[string]any{"Author": 2, "Book": 2, "Review": 2}}
	for dialect, want := range map[string]string{
		"postgresql": "TIMESTAMP WITH TIME ZONE '2026-01-01 00:00:00+00'",
		"sqlserver":  "'2026-01-01T00:00:00Z'",
		"oracle":     "TIMESTAMP '2026-01-01 00:00:00 +00:00'",
		"mariadb":    "'2026-01-01T00:00:00.000000000Z'",
	} {
		s := seedOf(t, request(t, caseProject, dialect, settings))
		if !strings.Contains(s, want) {
			t.Errorf("%s: no %s in\n%s", dialect, want, s)
		}
	}
}

// A required encrypted field is an error at the field, and an entity that
// needs its rows is an error too; the file still names both.
func TestEncryptedRefused(t *testing.T) {
	r := request(t, "../../spec/tests/generate-sql/project", "postgresql", nil)
	resp := Generate(r)
	var paths []string
	for _, d := range resp.Diagnostics {
		paths = append(paths, d.Path)
	}
	if strings.Join(paths, " ") != "/entities/Author/properties/email /entities/Book" {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if len(resp.Files) != 1 || !strings.Contains(resp.Files[0].Content, "-- Author: no rows; email is encrypted at rest.") {
		t.Fatalf("files: %v", resp.Files)
	}
}

// A value drawn from a pattern matches it.
func TestPatternValue(t *testing.T) {
	r := newRandom(3)
	for _, p := range []string{`^[0-9]{13}$`, `^\+?[0-9 ]{6,20}$`, `^[A-Z]{2}-[0-9]+$`, `^(red|green|blue)$`, `^[a-z]+@example\.(com|org)$`, `^97[89][0-9]{10}$`} {
		re := regexp.MustCompile(p)
		for i := 0; i < 20; i++ {
			s, why := patternValue(p, r)
			if why != "" || !re.MatchString(s) {
				t.Errorf("%s: %q (%s)", p, s, why)
			}
		}
	}
	if _, why := patternValue(`\bword\b`, r); why == "" {
		t.Error("a word boundary is not refused")
	}
}

func TestLiterals(t *testing.T) {
	cases := []struct {
		v       value
		dialect string
		want    string
	}{
		{value{kind: kDuration, d: 2*time.Hour + 30*time.Minute}, "postgresql", "INTERVAL 'PT2H30M'"},
		{value{kind: kDuration, d: 26 * time.Hour}, "oracle", "INTERVAL '1 02:00:00' DAY TO SECOND"},
		{value{kind: kTimeOfDay, num: 3661}, "oracle", "INTERVAL '0 01:01:01' DAY TO SECOND"},
		{value{kind: kTimeOfDay, num: 3661}, "postgresql", "TIME '01:01:01'"},
		{value{kind: kBytes, bytes: []byte{10, 255}}, "postgresql", `'\x0aff'`},
		{value{kind: kBytes, bytes: []byte{10, 255}}, "sqlserver", "0x0AFF"},
		{value{kind: kBytes, bytes: []byte{10, 255}}, "oracle", "HEXTORAW('0AFF')"},
		{value{kind: kBytes, bytes: []byte{10, 255}}, "mariadb", "X'0AFF'"},
		{value{kind: kDate, t: base}, "sqlserver", "'2026-01-01'"},
		{value{kind: kText, text: "it's"}, "mariadb", "'it''s'"},
		{value{kind: kList, items: []value{textValue("a")}}, "postgresql", "ARRAY['a']"},
		{value{kind: kList, items: []value{textValue("a")}}, "mariadb", `'["a"]'`},
		{value{kind: kList, items: []value{textValue("a")}, json: true}, "postgresql", `'["a"]'`},
		{value{kind: kBool, b: true}, "sqlserver", "1"},
		{value{kind: kDecimal, text: "12.50"}, "oracle", "12.50"},
	}
	literals := map[string]string{"true": "1", "false": "0"}
	for _, c := range cases {
		if got := c.v.literal(c.dialect, literals); got != c.want {
			t.Errorf("%s: %s, want %s", c.dialect, got, c.want)
		}
	}
	if got := scaled(-1234, 2); got != "-12.34" {
		t.Errorf("scaled: %s", got)
	}
	if got := scaled(5, 3); got != "0.005" {
		t.Errorf("scaled: %s", got)
	}
	if got := words("lastModifiedBy"); got != "Last modified by" {
		t.Errorf("words: %s", got)
	}
	if got := snake("replyToId"); got != "reply_to_id" {
		t.Errorf("snake: %s", got)
	}
}
