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

// TestSnapshot answers only the snapshot when nothing changed.
func TestSnapshot(t *testing.T) {
	snap := firstSnapshot(t, "postgresql")
	again := Generate(request(t, "postgresql", snap, File{Path: "0001_expand.sql", Content: "..."}))
	if len(again.Diagnostics) > 0 || len(again.Files) != 1 || again.Files[0].Path != SnapshotName || again.Files[0].Content != snap.Content {
		t.Errorf("an unchanged schema should answer only its snapshot, got %v and %v", again.Files, again.Diagnostics)
	}
}

func firstSnapshot(t *testing.T, dialect string) File {
	t.Helper()
	for _, f := range Generate(request(t, dialect)).Files {
		if f.Path == SnapshotName {
			return f
		}
	}
	t.Fatal("the first run wrote no snapshot")
	return File{}
}

// changed runs the differ on the example after a change, on a dialect.
func changed(t *testing.T, dialect string, destructive bool, change func(entities map[string]any)) Response {
	t.Helper()
	snap := firstSnapshot(t, dialect)
	r := request(t, dialect, snap, File{Path: "0001_expand.sql", Content: "..."})
	if destructive {
		r.Implementations[0].Settings = map[string]any{"destructive": true}
	}
	change(r.Specification["entities"].(map[string]any))
	return Generate(r)
}

func file(resp Response, path string) string {
	for _, f := range resp.Files {
		if f.Path == path {
			return f.Content
		}
	}
	return ""
}

func book(e map[string]any) map[string]any { return e["Book"].(map[string]any) }
func bookProps(e map[string]any) map[string]any {
	return book(e)["properties"].(map[string]any)
}

// TestDiffer checks the migration of each kind of change.
func TestDiffer(t *testing.T) {
	resp := changed(t, "postgresql", false, func(e map[string]any) {
		bookProps(e)["edition"] = map[string]any{"type": "string", "maxLength": json.Number("40")}
		bookProps(e)["title"].(map[string]any)["maxLength"] = json.Number("800")
	})
	exp := file(resp, "0002_expand.sql")
	for _, want := range []string{"ALTER TABLE books ADD COLUMN edition VARCHAR(40);", "ALTER TABLE books ALTER COLUMN title TYPE VARCHAR(800);"} {
		if !strings.Contains(exp, want) {
			t.Errorf("0002_expand.sql has no %q:\n%s%v", want, exp, resp.Diagnostics)
		}
	}
	if file(resp, SnapshotName) == "" {
		t.Error("the snapshot is not written again")
	}

	resp = changed(t, "postgresql", false, func(e map[string]any) {
		bookProps(e)["edition"] = map[string]any{"type": "string", "maxLength": json.Number("40")}
		book(e)["required"] = append(book(e)["required"].([]any), "edition")
	})
	if len(resp.Diagnostics) != 1 || !strings.Contains(resp.Diagnostics[0].Message, "new required column") {
		t.Errorf("a new required column without a default should be refused, got %v", resp.Diagnostics)
	}

	drop := func(e map[string]any) {
		delete(bookProps(e), "author")
		var req []any
		for _, r := range book(e)["required"].([]any) {
			if r != "author" {
				req = append(req, r)
			}
		}
		book(e)["required"] = req
	}
	resp = changed(t, "postgresql", false, drop)
	if len(resp.Diagnostics) != 1 || !strings.Contains(resp.Diagnostics[0].Message, "drop the column author of books") {
		t.Errorf("a dropped column should need destructive: true, got %v", resp.Diagnostics)
	}
	resp = changed(t, "postgresql", true, drop)
	if c := file(resp, "0002_contract.sql"); !strings.Contains(c, "ALTER TABLE books DROP COLUMN author;") {
		t.Errorf("0002_contract.sql has no DROP COLUMN: %q %v", c, resp.Diagnostics)
	}

	resp = changed(t, "postgresql", false, func(e map[string]any) {
		bookProps(e)["title"].(map[string]any)["maxLength"] = json.Number("100")
	})
	if len(resp.Diagnostics) != 1 || !strings.Contains(resp.Diagnostics[0].Message, "narrow title of books") {
		t.Errorf("a narrower column should need destructive: true, got %v", resp.Diagnostics)
	}

	resp = changed(t, "postgresql", false, func(e map[string]any) {
		bookProps(e)["copiesOwned"] = map[string]any{"type": "string", "maxLength": json.Number("10")}
	})
	if len(resp.Diagnostics) != 1 || !strings.Contains(resp.Diagnostics[0].Message, "the type of copiesOwned changed") {
		t.Errorf("a changed type should be refused, got %v", resp.Diagnostics)
	}

	resp = changed(t, "postgresql", false, func(e map[string]any) {
		e["Shelf"] = map[string]any{"type": "object", "properties": map[string]any{
			"id": map[string]any{"type": "string", "format": "uuid"}, "bookId": map[string]any{"type": "string", "format": "uuid"}},
			"required": []any{"id", "bookId"}, "primaryKey": []any{"id"},
			"relations": map[string]any{"book": map[string]any{"target": "Book", "kind": "many-to-one", "via": "bookId"}}}
	})
	exp = file(resp, "0002_expand.sql")
	for _, want := range []string{"CREATE TABLE shelf (", "ALTER TABLE shelf ADD CONSTRAINT fk_shelf_book_id FOREIGN KEY (book_id) REFERENCES books (id) ON DELETE RESTRICT;"} {
		if !strings.Contains(exp, want) {
			t.Errorf("0002_expand.sql has no %q:\n%s%v", want, exp, resp.Diagnostics)
		}
	}

	resp = changed(t, "oracle", false, func(e map[string]any) {
		bookProps(e)["edition"] = map[string]any{"type": "string", "maxLength": json.Number("40")}
	})
	if exp := file(resp, "0002_expand.sql"); !strings.Contains(exp, "ALTER TABLE books ADD (edition VARCHAR2(40 CHAR));") {
		t.Errorf("Oracle adds a column in parentheses: %q %v", exp, resp.Diagnostics)
	}
}

// TestDifferEnum widens an enum's check for a new value, and needs
// destructive: true to take one away.
func TestDifferEnum(t *testing.T) {
	snap := firstSnapshot(t, "postgresql")
	first := File{Path: "0001_expand.sql", Content: "..."}
	r := request(t, "postgresql", snap, first)
	status := r.Specification["enums"].(map[string]any)["LoanStatus"].(map[string]any)
	status["enum"] = append(status["enum"].([]any), "damaged")
	resp := Generate(r)
	exp := file(resp, "0002_expand.sql")
	for _, want := range []string{"ALTER TABLE loans DROP CONSTRAINT ck_loans_status;", "ALTER TABLE loans ADD CONSTRAINT ck_loans_status CHECK (status IN ('open', 'overdue', 'returned', 'lost', 'damaged'));"} {
		if !strings.Contains(exp, want) {
			t.Errorf("0002_expand.sql has no %q:\n%s%v", want, exp, resp.Diagnostics)
		}
	}
	r = request(t, "postgresql", snap, first)
	status = r.Specification["enums"].(map[string]any)["LoanStatus"].(map[string]any)
	status["enum"] = []any{"open", "returned", "lost", "overdue"}[:3]
	resp = Generate(r)
	if len(resp.Diagnostics) == 0 || !strings.Contains(resp.Diagnostics[0].Message, "fewer values in status of loans") {
		t.Errorf("a removed enum value should need destructive: true, got %v", resp.Diagnostics)
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

// withViews adds a view of loans with their member's name and book's
// title, and a view of members with their loans counted.
func withViews(r *Request) {
	r.Specification["views"] = map[string]any{
		"LoanRow": map[string]any{"from": "Loan", "properties": map[string]any{
			"memberName": map[string]any{"path": "member.fullName"},
			"bookTitle":  map[string]any{"path": "book.title"}}},
		"MemberRow": map[string]any{"from": "Member", "properties": map[string]any{
			"openLoans": map[string]any{"count": "loans"}}},
	}
}

// TestViews writes each view after the tables, on each dialect: the
// entity's columns, a LEFT JOIN per relation a path follows, and a count
// in a subquery.
func TestViews(t *testing.T) {
	want := map[string][]string{
		"postgresql": {"CREATE VIEW loan_row AS\nSELECT\n    t0.id,", "    t2.full_name AS member_name\nFROM loans t0\n    LEFT JOIN books t1 ON t1.id = t0.book_id\n    LEFT JOIN members t2 ON t2.id = t0.member_id;",
			"t1.title AS book_title,", "t0.is_deleted,", "t0.email_hash,", "(SELECT COUNT(*) FROM loans c WHERE c.member_id = t0.id) AS open_loans\nFROM members t0;"},
		"sqlserver": {"EXEC('CREATE VIEW loan_row AS", "(SELECT COUNT_BIG(*) FROM loans c WHERE c.member_id = t0.id) AS open_loans\nFROM members t0');"},
		"oracle":    {"FROM loans t0\n    LEFT JOIN books t1 ON t1.id = t0.book_id"},
		"mariadb":   {"CREATE VIEW member_row AS"},
	}
	for dialect, lines := range want {
		r := request(t, dialect)
		withViews(r)
		sql := migration(t, r)
		for _, l := range lines {
			if !strings.Contains(sql, l) {
				t.Errorf("%s: the migration has no %q:\n%s", dialect, l, sql)
			}
		}
		if strings.Index(sql, "CREATE VIEW") < strings.LastIndex(sql, "FOREIGN KEY") {
			t.Errorf("%s: a view is created before the foreign keys", dialect)
		}
	}
}

// TestViewCountsSoftDeleted leaves softly deleted records out of a count.
func TestViewCountsSoftDeleted(t *testing.T) {
	r := request(t, "postgresql")
	withViews(r)
	r.Specification["entities"].(map[string]any)["Loan"].(map[string]any)["deletion"] = "soft"
	if sql := migration(t, r); !strings.Contains(sql, "WHERE c.member_id = t0.id AND c.is_deleted = false)") {
		t.Errorf("the count does not leave out deleted loans:\n%s", sql)
	}
}

// TestDifferViews drops and creates a view again around a change of an
// entity it reads, and leaves it alone otherwise.
func TestDifferViews(t *testing.T) {
	first := Generate(func() *Request { r := request(t, "postgresql"); withViews(r); return r }())
	snap := File{Path: SnapshotName, Content: file(first, SnapshotName)}
	if !strings.Contains(snap.Content, "views:") {
		t.Fatalf("the snapshot has no views:\n%s", snap.Content)
	}
	r := request(t, "postgresql", snap, File{Path: "0001_expand.sql", Content: "..."})
	withViews(r)
	bookProps(r.Specification["entities"].(map[string]any))["edition"] = map[string]any{"type": "string", "maxLength": json.Number("40")}
	exp := file(Generate(r), "0002_expand.sql")
	if !strings.HasPrefix(strings.SplitN(exp, "\n", 3)[2], "DROP VIEW loan_row;") || !strings.Contains(exp, "t1.title AS book_title") || strings.Contains(exp, "member_row") {
		t.Errorf("0002_expand.sql should drop and create loan_row, which reads books, and only it:\n%s", exp)
	}

	r = request(t, "postgresql", snap, File{Path: "0001_expand.sql", Content: "..."})
	withViews(r)
	r.Implementations[0].Settings = map[string]any{"destructive": true}
	m := r.Specification["entities"].(map[string]any)["Member"].(map[string]any)
	m["properties"].(map[string]any)["fullName"].(map[string]any)["maxLength"] = json.Number("100")
	resp := Generate(r)
	con := file(resp, "0003_contract.sql")
	if !strings.Contains(con, "DROP VIEW loan_row;\n\nDROP VIEW member_row;\n\nALTER TABLE members ALTER COLUMN full_name TYPE VARCHAR(100);") || !strings.HasSuffix(con, "FROM members t0;\n") {
		t.Errorf("0003_contract.sql should drop the views reading members, narrow the column and create them again:\n%s%v", con, resp.Diagnostics)
	}
}

// TestDifferViewsEnum makes a view again when an enum of a field it reads
// changes, since PostgreSQL refuses to widen a column a view reads.
func TestDifferViewsEnum(t *testing.T) {
	first := Generate(func() *Request { r := request(t, "postgresql"); withViews(r); return r }())
	snap := File{Path: SnapshotName, Content: file(first, SnapshotName)}
	r := request(t, "postgresql", snap, File{Path: "0001_expand.sql", Content: "..."})
	withViews(r)
	status := r.Specification["enums"].(map[string]any)["LoanStatus"].(map[string]any)
	status["enum"] = append(status["enum"].([]any), "returned-damaged-beyond-repair")
	exp := file(Generate(r), "0002_expand.sql")
	drop, alter := strings.Index(exp, "DROP VIEW loan_row;"), strings.Index(exp, "ALTER COLUMN status TYPE")
	if drop < 0 || alter < 0 || drop > alter || strings.Index(exp, "CREATE VIEW loan_row") < alter {
		t.Errorf("loan_row should be dropped before status widens and made again after:\n%s", exp)
	}
}

// TestViewRefusals refuses an added field named like an audit column, a
// path to a field that is only written, and a join entity with two
// relations to one side; and counts through a join entity on SQL Server,
// leaving out softly deleted records.
func TestViewRefusals(t *testing.T) {
	refused := func(change func(r *Request), want string) {
		t.Helper()
		r := request(t, "postgresql")
		withViews(r)
		change(r)
		resp := Generate(r)
		if len(resp.Diagnostics) != 1 || !strings.Contains(resp.Diagnostics[0].Message, want) {
			t.Errorf("want one diagnostic with %q, got %v", want, resp.Diagnostics)
		}
	}
	views := func(r *Request) map[string]any { return r.Specification["views"].(map[string]any) }
	entities := func(r *Request) map[string]any { return r.Specification["entities"].(map[string]any) }
	refused(func(r *Request) {
		views(r)["MemberRow"].(map[string]any)["properties"].(map[string]any)["createdAt"] = map[string]any{"count": "loans"}
	}, "an audit, deleted or hash column")
	refused(func(r *Request) {
		entities(r)["Member"].(map[string]any)["properties"].(map[string]any)["fullName"].(map[string]any)["writeOnly"] = true
	}, "written and never read")

	shelving := func(r *Request, extra bool) {
		rels := map[string]any{"book": map[string]any{"target": "Book", "kind": "many-to-one", "via": "bookId"},
			"member": map[string]any{"target": "Member", "kind": "many-to-one", "via": "memberId"}}
		if extra {
			rels["sponsor"] = map[string]any{"target": "Member", "kind": "many-to-one", "via": "sponsorId"}
		}
		entities(r)["Shelving"] = map[string]any{"properties": map[string]any{
			"bookId": map[string]any{"type": "string", "format": "uuid"}, "memberId": map[string]any{"type": "string", "format": "uuid"},
			"sponsorId": map[string]any{"type": "string", "format": "uuid"}},
			"required": []any{"bookId", "memberId", "sponsorId"}, "primaryKey": []any{"bookId", "memberId"}, "relations": rels}
		entities(r)["Member"].(map[string]any)["relations"].(map[string]any)["shelved"] = map[string]any{"target": "Book", "kind": "many-to-many", "via": "Shelving"}
		views(r)["MemberRow"].(map[string]any)["properties"].(map[string]any)["shelvedBooks"] = map[string]any{"count": "shelved"}
	}
	refused(func(r *Request) { shelving(r, true) }, "exactly one many-to-one relation")

	r := request(t, "sqlserver")
	withViews(r)
	shelving(r, false)
	entities(r)["Book"].(map[string]any)["deletion"] = "soft"
	sql := migration(t, r)
	if !strings.Contains(sql, "(SELECT COUNT_BIG(*) FROM shelving c INNER JOIN books r ON r.id = c.book_id WHERE c.member_id = t0.id AND r.is_deleted = 0) AS shelved_books") {
		t.Errorf("the count through Shelving is wrong:\n%s", sql)
	}
}
