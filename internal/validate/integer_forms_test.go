package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/spec"
)

// TestIntegerForms checks that every check reading an integer out of a
// design reads it as the schema check does (ADR-178): the library-lending
// example with one value written in hexadecimal, octal, binary, with
// underscores or with a leading zero gives the same problems, case names
// included, as with the value in decimal.
func TestIntegerForms(t *testing.T) {
	example := filepath.Join("..", "..", "examples", "library-lending", "spec")
	for _, r := range []struct {
		reader, file string
		line         int
		old, written string // written holds %s for the value
		value        int
	}{
		{"list_of page size", "design/paths/paths.yaml", 15, "maximum: 100", "maximum: %s", 100},
		{"query maxLength", "design/paths/paths.yaml", 12, "maxLength: 200", "maxLength: %s", 200},
		{"maxRequestBytes", "design/paths/paths.yaml", 285, "maxRequestBytes: 1024", "maxRequestBytes: %s", 1024},
		{"rate burst", "design/paths/paths.yaml", 286, "burst: 60", "burst: %s", 60},
		{"rate requests", "design/paths/paths.yaml", 286, "requests: 30", "requests: %s", 61},
		{"minItems", "design/paths/paths.yaml", 210, "minItems: 1", "minItems: %s", 1},
		{"maxItems", "design/paths/paths.yaml", 211, "maxItems: 6", "maxItems: %s", 6},
		{"child rows maximum", "design/pages/member-loans.yaml", 14, "maximum: 6", "maximum: %s", 6},
		{"cardinality", "design/separation-of-duties.yaml", 6, "cardinality: 2", "cardinality: %s", 3},
		{"decimal scale", "design/entities/book.yaml", 13, "scale: 2", "scale: %s", 2},
		{"int64 bound", "design/algorithms/late-fee.yaml", 8, "format: int32, minimum: 0", "format: int64, minimum: 0, maximum: %s", 100},
	} {
		check := func(text string) []string {
			dir := filepath.Join(t.TempDir(), "spec")
			if err := os.CopyFS(dir, os.DirFS(example)); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, filepath.FromSlash(r.file))
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(string(data), "\n")
			if !strings.Contains(lines[r.line-1], r.old) {
				t.Fatalf("%s line %d no longer holds %q", r.file, r.line, r.old)
			}
			lines[r.line-1] = strings.Replace(lines[r.line-1], r.old, fmt.Sprintf(r.written, text), 1)
			if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
				t.Fatal(err)
			}
			var out []string
			for _, d := range CheckSpec(spec.Load(dir)) {
				rel, _ := filepath.Rel(dir, d.File)
				out = append(out, fmt.Sprintf("%s %s %s %s: %s", rel, d.Severity, d.Path, d.Rule, d.Message))
			}
			return out
		}
		want := strings.Join(check(fmt.Sprint(r.value)), "\n")
		for _, form := range integerForms(r.value) {
			if got := strings.Join(check(form), "\n"); got != want {
				t.Errorf("%s written %s gives\n%s\nwhere %d gives\n%s", r.reader, form, got, r.value, want)
			}
		}
	}
}

// integerForms are the ways YAML writes v other than in decimal.
func integerForms(v int) []string {
	forms := []string{fmt.Sprintf("0x%x", v), fmt.Sprintf("0o%o", v), fmt.Sprintf("0b%b", v), fmt.Sprintf("0%o", v)}
	if s := fmt.Sprint(v); len(s) > 1 {
		forms = append(forms, s[:1]+"_"+s[1:])
	}
	return forms
}
