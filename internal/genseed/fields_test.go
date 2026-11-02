package genseed

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// These tests draw one branch of fieldValue, fromExample and fill each,
// over a small specification written inline, so a change to one branch
// is caught where it is made.

const inlineHead = `specarch: "0.1"
info: { title: Case, version: 1.0.0 }
stages: [implementation]
stakeholders:
  owner: { description: The product owner. }
`

const inlineImplementation = `specarchImplementation: "0.1"
info: { title: Case in Go, version: 1.0.0 }
implements: { file: ../../specarch.yaml, version: 1.0.0 }
stack: { language: { name: Go, version: "1.26" } }
targets:
  sql: { output: ../../../migrations, dialect: postgresql }
  seed: { output: ../../../seed }
`

// inline writes a specification and its Go implementation file into a
// folder of the test and answers the folder.
func inline(t *testing.T, body string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(filepath.Join(dir, "implementation", "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "specarch.yaml"), []byte(inlineHead+body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "implementation", "go", "case.go.specarch-implementation.yaml"), []byte(inlineImplementation), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// generated runs the generator on an inline specification with the rows
// given and answers the file and the error diagnostics as "path: message"
// lines.
func generated(t *testing.T, body string, rows map[string]any) (string, []string) {
	t.Helper()
	settings := map[string]any{"seed": 3}
	if rows != nil {
		settings["rows"] = rows
	}
	resp := Generate(request(t, inline(t, body), "postgresql", settings))
	var errs []string
	for _, d := range resp.Diagnostics {
		if d.Severity == "error" {
			errs = append(errs, d.Path+": "+d.Message)
		}
	}
	if len(resp.Files) != 1 {
		t.Fatalf("files: %v", resp.Files)
	}
	return resp.Files[0].Content, errs
}

// inserts are the INSERT lines of one table, their values only.
func inserts(file, table string) []string {
	var out []string
	for _, line := range strings.Split(file, "\n") {
		if rest, ok := strings.CutPrefix(line, "INSERT INTO "+table+" ("); ok {
			out = append(out, rest[strings.Index(rest, "VALUES (")+8:len(rest)-2])
		}
	}
	return out
}

// rowsOf are the inserts of a table, and the test fails when they are not
// n, so no loop over them passes empty.
func rowsOf(t *testing.T, file, table string, n int) []string {
	t.Helper()
	rows := inserts(file, table)
	if len(rows) != n {
		t.Fatalf("%d rows of %s, want %d:\n%s", len(rows), table, n, file)
	}
	return rows
}

func mustHaveNoErrors(t *testing.T, errs []string) {
	t.Helper()
	if len(errs) > 0 {
		t.Fatalf("errors: %s", strings.Join(errs, "\n"))
	}
}

const tagEntity = `entities:
  Tag:
    type: object
    properties:
      id: { type: string, format: uuid }
%s
    required: [id%s]
    primaryKey: [id]
`

// one is a Tag entity with the fields given, each required when its name
// is listed.
func one(fields string, required ...string) string {
	extra := ""
	if len(required) > 0 {
		extra = ", " + strings.Join(required, ", ")
	}
	var b strings.Builder
	for _, f := range strings.Split(strings.TrimSpace(fields), "\n") {
		b.WriteString("      " + strings.TrimSpace(f) + "\n")
	}
	return strings.ReplaceAll(strings.ReplaceAll(tagEntity, "%s\n", b.String()), "%s", extra)
}

// A const is every row's value.
func TestConst(t *testing.T) {
	file, errs := generated(t, one(`kind: { type: string, const: fixed, maxLength: 10 }`, "kind"), map[string]any{"Tag": 3})
	mustHaveNoErrors(t, errs)
	for _, row := range rowsOf(t, file, "tag", 3) {
		if !strings.HasSuffix(row, ", 'fixed'") {
			t.Errorf("row %s", row)
		}
	}
}

// An integer lies inside its exclusive bounds and on its multipleOf; a
// maximum alone counts from zero.
func TestIntegerBounds(t *testing.T) {
	file, errs := generated(t, one(`n: { type: integer, format: int32, exclusiveMinimum: 0, exclusiveMaximum: 100, multipleOf: 5 }
m: { type: integer, format: int32, maximum: 10 }`, "n", "m"), map[string]any{"Tag": 20})
	mustHaveNoErrors(t, errs)
	re := regexp.MustCompile(`, (\d+), (\d+)$`)
	for _, row := range rowsOf(t, file, "tag", 20) {
		m := re.FindStringSubmatch(row)
		if m == nil {
			t.Fatalf("row %s", row)
		}
		mv, nv := atoi(t, m[1]), atoi(t, m[2])
		if nv < 5 || nv > 95 || nv%5 != 0 {
			t.Errorf("n %d is not a multiple of 5 in (0, 100)", nv)
		}
		if mv < 0 || mv > 10 {
			t.Errorf("m %d is not in [0, 10]", mv)
		}
	}
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

// An address lies in the ranges reserved for documentation, and a host
// name under one of the reserved domains.
func TestAddressFormats(t *testing.T) {
	file, errs := generated(t, one(`v4: { type: string, format: ipv4 }
v6: { type: string, format: ipv6 }
host: { type: string, format: hostname, maxLength: 100 }`, "v4", "v6", "host"), map[string]any{"Tag": 6})
	mustHaveNoErrors(t, errs)
	re := regexp.MustCompile(`'(host-\d+\.example\.(com|org|net))', '((192\.0\.2|198\.51\.100|203\.0\.113)\.\d+)', '(2001:db8::[0-9a-f]+)'$`)
	for _, row := range rowsOf(t, file, "tag", 6) {
		if !re.MatchString(row) {
			t.Errorf("row %s", row)
		}
	}
}

// Plain text that does not fit its maxLength is the row's number padded
// to minLength with the label's letters and zeros; a minLength above the
// maxLength is refused at the field.
func TestShortText(t *testing.T) {
	file, errs := generated(t, one(`code: { type: string, minLength: 3, maxLength: 3 }
n: { type: string, minLength: 6, maxLength: 6 }
k: { type: string, maxLength: 2 }`, "code", "n", "k"), map[string]any{"Tag": 2})
	mustHaveNoErrors(t, errs)
	rows := inserts(file, "tag")
	if len(rows) != 2 || !strings.HasSuffix(rows[0], "'Co1', '1', 'N00001'") || !strings.HasSuffix(rows[1], "'Co2', '2', 'N00002'") {
		t.Errorf("rows %q", rows)
	}
	_, errs = generated(t, one(`code: { type: string, minLength: 4, maxLength: 3 }`, "code"), map[string]any{"Tag": 2})
	if len(errs) != 1 || !strings.Contains(errs[0], "/entities/Tag/properties/code: Tag.code gets no value: its minLength 4 is above its maxLength 3") {
		t.Errorf("errors %q", errs)
	}
}

// A key with examples counts on from after them, skipping a value an
// example holds; so does a unique text column.
func TestExamplesOnKey(t *testing.T) {
	file, errs := generated(t, `entities:
  Tag:
    type: object
    properties:
      id: { type: integer, format: int32, examples: [1, 2] }
      code: { type: string, maxLength: 10, examples: [Code 2] }
    required: [id, code]
    primaryKey: [id]
    constraints:
      tag_code_unique: { kind: unique, fields: [code], message: The code is taken. }
`, map[string]any{"Tag": 4})
	mustHaveNoErrors(t, errs)
	if got := strings.Join(inserts(file, "tag"), "; "); got != "1, 'Code 2'; 2, 'Code 1'; 0, 'Code 3'; 3, 'Code 4'" {
		t.Errorf("rows %s", got)
	}
}

// A key that is drawn rather than counted (a uuid) with examples draws
// once for each row after them, as a row without examples does, so the
// random stream moves by one value a row: the rows after the examples
// are the rows a key without them gives.
func TestExamplesOnDrawnKey(t *testing.T) {
	withExamples, errs := generated(t, `entities:
  Tag:
    type: object
    properties:
      id: { type: string, format: uuid, examples: [8f6a1c2e-4b3d-4e5f-8a9b-0c1d2e3f4a5b, 1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d] }
      code: { type: string, maxLength: 20 }
    required: [id, code]
    primaryKey: [id]
`, map[string]any{"Tag": 4})
	mustHaveNoErrors(t, errs)
	without, errs := generated(t, `entities:
  Tag:
    type: object
    properties:
      id: { type: string, format: uuid }
      code: { type: string, maxLength: 20 }
    required: [id, code]
    primaryKey: [id]
`, map[string]any{"Tag": 2})
	mustHaveNoErrors(t, errs)
	rows := rowsOf(t, withExamples, "tag", 4)
	plain := rowsOf(t, without, "tag", 2)
	if !strings.HasPrefix(rows[0], "'8f6a1c2e-4b3d-4e5f-8a9b-0c1d2e3f4a5b'") || !strings.HasPrefix(rows[1], "'1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d'") {
		t.Errorf("the examples are not the first rows: %q", rows[:2])
	}
	for n := 0; n < 2; n++ {
		if id(rows[n+2]) != id(plain[n]) {
			t.Errorf("row %d with examples holds %s, a key without them gives %s for its row %d", n+2, id(rows[n+2]), id(plain[n]), n)
		}
	}
}

// id is the first value of an INSERT's values.
func id(values string) string {
	return values[:strings.Index(values, ",")]
}

// Examples of a value object kept as JSON are read from the design's
// field, each member typed by the schema's part; an object example of a
// field with its own properties keeps a number a number, written as it
// was given.
func TestObjectExamples(t *testing.T) {
	file, errs := generated(t, `schemas:
  Meta:
    type: object
    properties:
      pages: { type: integer, format: int32 }
      note: { type: string, maxLength: 20 }
      price: { type: string, format: decimal, precision: 10, scale: 2 }
    required: [pages]
entities:
  Tag:
    type: object
    properties:
      id: { type: string, format: uuid }
      meta: { $ref: "#/schemas/Meta", storage: json, examples: [{ pages: 300, note: first, price: "12.50" }] }
      extra: { type: object, properties: { n: { type: integer, format: int32 }, f: { type: number, format: double } }, examples: [{ n: 3, f: 1.5, other: 7 }] }
      list: { type: array, items: { $ref: "#/schemas/Meta" }, storage: json, examples: [[{ pages: 12 }]] }
    required: [id, meta, extra, list]
    primaryKey: [id]
`, map[string]any{"Tag": 2})
	mustHaveNoErrors(t, errs)
	rows := inserts(file, "tag")
	if len(rows) != 2 || !strings.HasSuffix(rows[0], `'{"f":1.5,"n":3,"other":7}', '[{"pages":12}]', '{"note":"first","pages":300,"price":"12.50"}'`) {
		t.Errorf("rows %q", rows)
	}
	if strings.Contains(rows[1], `"pages":"`) || !strings.Contains(rows[1], `"pages":`) {
		t.Errorf("the second row's drawn meta: %s", rows[1])
	}
}

// A check that reads a list is evaluated on the list's length, so every
// row holds it.
func TestCheckReadsList(t *testing.T) {
	file, errs := generated(t, `entities:
  Tag:
    type: object
    properties:
      id: { type: string, format: uuid }
      tags: { type: array, items: { type: string, maxLength: 20 }, maxItems: 3 }
    required: [id, tags]
    primaryKey: [id]
    constraints:
      tag_two: { kind: check, expression: "size(tags) >= 2", message: Two tags at least. }
`, map[string]any{"Tag": 8})
	mustHaveNoErrors(t, errs)
	rows := inserts(file, "tag")
	if len(rows) != 8 {
		t.Fatalf("%d rows", len(rows))
	}
	for _, row := range rows {
		if strings.Count(row, "'Tags ") < 2 {
			t.Errorf("row %s", row)
		}
	}
}

// A negated character class draws from its printable ASCII.
func TestNegatedClass(t *testing.T) {
	file, errs := generated(t, one(`handle: { type: string, pattern: "^[^@ ]{3,8}$", maxLength: 8 }`, "handle"), map[string]any{"Tag": 10})
	mustHaveNoErrors(t, errs)
	re := regexp.MustCompile(`'([^']*)'$`)
	for _, row := range rowsOf(t, file, "tag", 10) {
		m := re.FindStringSubmatch(row)
		if m == nil {
			t.Fatalf("row %s", row)
		}
		for _, c := range m[1] {
			if c < '!' || c > '~' || c == '@' {
				t.Errorf("%q holds %q", m[1], c)
			}
		}
	}
	r := newRandom(5)
	for i := 0; i < 50; i++ {
		s, why := patternValue(`^[^@ ]{3,8}$`, r)
		if why != "" {
			t.Fatal(why)
		}
		for _, c := range s {
			if c < '!' || c > '~' {
				t.Errorf("%q holds %q", s, c)
			}
		}
	}
}

// A required relation to the entity itself and a cycle of required
// relations are errors at the entry.
func TestSelfAndCycle(t *testing.T) {
	_, errs := generated(t, `entities:
  Node:
    type: object
    properties:
      id: { type: string, format: uuid }
      parentId: { type: string, format: uuid }
    required: [id, parentId]
    primaryKey: [id]
    relations:
      parent: { target: Node, kind: many-to-one, via: parentId, onDelete: restrict }
  A:
    type: object
    properties:
      id: { type: string, format: uuid }
      bId: { type: string, format: uuid }
    required: [id, bId]
    primaryKey: [id]
    relations:
      b: { target: B, kind: many-to-one, via: bId, onDelete: restrict }
  B:
    type: object
    properties:
      id: { type: string, format: uuid }
      aId: { type: string, format: uuid }
    required: [id, aId]
    primaryKey: [id]
    relations:
      a: { target: A, kind: many-to-one, via: aId, onDelete: restrict }
`, nil)
	want := []string{"/entities/A: the required relations of A, B form a cycle", "/entities/B: the required relations of A, B form a cycle", "/entities/Node/relations/parent: Node.parent is a required relation to Node itself"}
	if len(errs) != 3 {
		t.Fatalf("errors %q", errs)
	}
	for i, w := range want {
		if !strings.HasPrefix(errs[i], w) {
			t.Errorf("error %d: %s, want %s", i, errs[i], w)
		}
	}
}

// An optional one-to-one relation points at the row of the same rank and
// holds NULL beyond the other side's rows; a required one with too few
// rows across is an error naming the optional way out.
func TestOneToOne(t *testing.T) {
	spec := `entities:
  Account:
    type: object
    properties:
      id: { type: string, format: uuid }
    required: [id]
    primaryKey: [id]
  Profile:
    type: object
    properties:
      id: { type: string, format: uuid }
      accountId: { type: [string, "null"], format: uuid }
    required: [id]
    primaryKey: [id]
    relations:
      account: { target: Account, kind: one-to-one, via: accountId, onDelete: cascade }
`
	file, errs := generated(t, spec, map[string]any{"Account": 3, "Profile": 5})
	mustHaveNoErrors(t, errs)
	accounts := inserts(file, "account")
	profiles := inserts(file, "profile")
	if len(accounts) != 3 || len(profiles) != 5 {
		t.Fatalf("%d accounts, %d profiles", len(accounts), len(profiles))
	}
	for i, p := range profiles {
		want := "NULL"
		if i < 3 {
			want = accounts[i]
		}
		if !strings.HasSuffix(p, ", "+want) {
			t.Errorf("profile %d: %s, want %s", i+1, p, want)
		}
	}
	_, errs = generated(t, strings.ReplaceAll(strings.ReplaceAll(spec, `accountId: { type: [string, "null"], format: uuid }`, `accountId: { type: string, format: uuid }`), "required: [id]\n    primaryKey: [id]\n    relations", "required: [id, accountId]\n    primaryKey: [id]\n    relations"), map[string]any{"Account": 2, "Profile": 5})
	if len(errs) != 1 || !strings.HasSuffix(errs[0], "give Account at least 5 rows or Profile at most 2, or make the relation optional, so the rows beyond hold NULL") {
		t.Errorf("errors %q", errs)
	}
}

// The hundred-draw error names the unique constraint or the check the
// last draw failed.
func TestHundredDrawsNameTheCause(t *testing.T) {
	_, errs := generated(t, `entities:
  Author:
    type: object
    properties:
      id: { type: string, format: uuid }
    required: [id]
    primaryKey: [id]
  Book:
    type: object
    properties:
      id: { type: string, format: uuid }
      authorId: { type: string, format: uuid }
      editorId: { type: string, format: uuid }
      pages: { type: integer, format: int32, minimum: 1, maximum: 100 }
    required: [id, authorId, editorId, pages]
    primaryKey: [id]
    relations:
      author: { target: Author, kind: many-to-one, via: authorId, onDelete: restrict }
      editor: { target: Author, kind: many-to-one, via: editorId, onDelete: restrict }
    constraints:
      book_pair_unique: { kind: unique, fields: [authorId, editorId], message: One book a pair. }
  Note:
    type: object
    properties:
      id: { type: string, format: uuid }
      pages: { type: integer, format: int32, minimum: 1, maximum: 100 }
    required: [id, pages]
    primaryKey: [id]
    constraints:
      note_long: { kind: check, expression: "pages > 5000", message: A note is long. }
`, map[string]any{"Author": 1, "Book": 2, "Note": 1})
	if len(errs) != 2 {
		t.Fatalf("errors %q", errs)
	}
	if !strings.Contains(errs[0], "it repeated the unique constraint book_pair_unique (author_id, editor_id) of an earlier row; give its columns more values to draw from") {
		t.Errorf("book: %s", errs[0])
	}
	if !strings.Contains(errs[1], "it broke note_long (A note is long); give the fields the constraint reads bounds or examples") {
		t.Errorf("note: %s", errs[1])
	}
}
