package gensql

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// request builds the request specarch would send for the library lending
// example, with its sql target set to a dialect.
func request(t *testing.T, dialect string, existing ...File) *Request {
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
	content["targets"].(map[string]any)["sql"].(map[string]any)["dialect"] = dialect
	data, err := json.Marshal(map[string]any{"specarch": "0.1", "target": "sql", "root": filepath.ToSlash(s.RootFile),
		"specification": s.Value, "output": "../../examples/library-lending/migrations",
		"implementations": []any{map[string]any{"file": impl.Path, "content": content, "idioms": idioms}},
		"existing":        existing})
	if err != nil {
		t.Fatal(err)
	}
	r, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func migration(t *testing.T, r *Request) string {
	t.Helper()
	resp := Generate(r)
	if len(resp.Diagnostics) > 0 {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	for _, f := range resp.Files {
		if f.Path == "0001_expand.sql" {
			return f.Content
		}
	}
	t.Fatalf("no 0001_expand.sql among %d files", len(resp.Files))
	return ""
}

// TestDialects renders the example on each dialect and checks what each
// must say: the column types the type-rendering idiom gives it, the enum
// and boolean checks, the defaults, the encrypted email and its hash, and
// the foreign keys' ON DELETE.
func TestDialects(t *testing.T) {
	want := map[string][]string{
		"postgresql": {
			"full_name VARCHAR(200) NOT NULL", "late_fee NUMERIC(10,2) NOT NULL", "loaned_at TIMESTAMP WITH TIME ZONE NOT NULL",
			"email BYTEA NOT NULL", "email_hash CHAR(64) NOT NULL", "CONSTRAINT member_email_unique UNIQUE (email_hash)",
			"status VARCHAR(8) NOT NULL", "CHECK (status IN ('open', 'overdue', 'returned', 'lost'))",
			"is_deleted BOOLEAN DEFAULT false NOT NULL", "returned_at TIMESTAMP WITH TIME ZONE,",
			"CHECK ((due_on > CAST(loaned_at AS DATE)))", "REFERENCES members (id) ON DELETE RESTRICT",
		},
		"sqlserver": {
			"full_name NVARCHAR(200) NOT NULL", "late_fee DECIMAL(10,2) NOT NULL", "loaned_at DATETIMEOFFSET NOT NULL",
			"email VARBINARY(MAX) NOT NULL", "id UNIQUEIDENTIFIER NOT NULL", "is_deleted BIT DEFAULT 0 NOT NULL",
			"REFERENCES members (id) ON DELETE NO ACTION",
		},
		"oracle": {
			"full_name VARCHAR2(200 CHAR) NOT NULL", "late_fee NUMBER(10,2) NOT NULL", "email BLOB NOT NULL",
			"id VARCHAR2(36 CHAR) NOT NULL", "is_deleted NUMBER(1) DEFAULT 0 NOT NULL", "CHECK (is_deleted IN (0, 1))",
			"CHECK ((due_on > TRUNC(loaned_at)))", "REFERENCES members (id);",
		},
		"mariadb": {
			"full_name VARCHAR(200) NOT NULL", "late_fee DECIMAL(10,2) NOT NULL", "loaned_at VARCHAR(35) NOT NULL",
			"email LONGBLOB NOT NULL", "id CHAR(36) NOT NULL", "is_deleted BOOLEAN DEFAULT false NOT NULL",
		},
	}
	for dialect, lines := range want {
		sql := migration(t, request(t, dialect))
		for _, l := range lines {
			if !strings.Contains(sql, l) {
				t.Errorf("%s: the migration has no %q", dialect, l)
			}
		}
		if dialect == "oracle" && strings.Contains(sql, " VARCHAR(") {
			t.Errorf("oracle: the migration has a VARCHAR; Oracle text is VARCHAR2")
		}
	}
}

// TestOracleWideText renders text wider than VARCHAR2 always holds as a
// CLOB, and refuses it as a key.
func TestOracleWideText(t *testing.T) {
	r := request(t, "oracle")
	book := r.Specification["entities"].(map[string]any)["Book"].(map[string]any)
	book["properties"].(map[string]any)["summary"] = map[string]any{"type": "string", "maxLength": json.Number("4000")}
	if sql := migration(t, r); !strings.Contains(sql, "summary CLOB,") {
		t.Errorf("text of 4000 characters is not a CLOB on Oracle:\n%s", sql)
	}
	book["constraints"].(map[string]any)["book_summary_unique"] = map[string]any{"kind": "unique", "fields": []any{"summary"}}
	resp := Generate(r)
	if len(resp.Diagnostics) != 1 || !strings.Contains(resp.Diagnostics[0].Message, "maxLength of at most 255") {
		t.Errorf("want one diagnostic about the key's width, got %v", resp.Diagnostics)
	}
}

// TestSnapshot answers only the snapshot when nothing changed, and refuses
// to write a second migration until the differ exists.
func TestSnapshot(t *testing.T) {
	first := Generate(request(t, "postgresql"))
	var snap File
	for _, f := range first.Files {
		if f.Path == SnapshotName {
			snap = f
		}
	}
	again := Generate(request(t, "postgresql", snap, File{Path: "0001_expand.sql", Content: "..."}))
	if len(again.Diagnostics) > 0 || len(again.Files) != 1 || again.Files[0].Path != SnapshotName || again.Files[0].Content != snap.Content {
		t.Errorf("an unchanged schema should answer only its snapshot, got %v and %v", again.Files, again.Diagnostics)
	}
	r := request(t, "postgresql", snap, File{Path: "0001_expand.sql", Content: "..."})
	book := r.Specification["entities"].(map[string]any)["Book"].(map[string]any)
	book["properties"].(map[string]any)["edition"] = map[string]any{"type": "string", "maxLength": json.Number("40")}
	changed := Generate(r)
	if len(changed.Diagnostics) != 1 || !strings.Contains(changed.Diagnostics[0].Message, "changed since migration 0001") {
		t.Errorf("a changed schema should be reported, got %v", changed.Diagnostics)
	}
}

// TestUnwritableCheck refuses a check whose function SQL is not given.
func TestUnwritableCheck(t *testing.T) {
	r := request(t, "postgresql")
	loan := r.Specification["entities"].(map[string]any)["Loan"].(map[string]any)
	loan["constraints"].(map[string]any)["loan_fee_text"] = map[string]any{"kind": "check", "expression": `string(lateFee) != ""`}
	resp := Generate(r)
	if len(resp.Diagnostics) != 1 || !strings.Contains(resp.Diagnostics[0].Message, "the function string") {
		t.Errorf("want one diagnostic about the function, got %v", resp.Diagnostics)
	}
}
