package testpackage

import (
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/gentests"
)

// noteSpecification is a specification of one operation keeping a note,
// with a path parameter, a query parameter and a body field in camelCase,
// and one test of it; wireNames, when given, is its info.wireNames.
func noteSpecification(t *testing.T, wireNames string) *gentests.Request {
	t.Helper()
	info := `"title": "Notes", "version": "0.1.0"`
	if wireNames != "" {
		info += `, "wireNames": "` + wireNames + `"`
	}
	r, err := gentests.Decode([]byte(`{"specification": {
		"info": {` + info + `},
		"entities": {"Note": {"properties": {"noteText": {"type": "string"}, "pageCount": {"type": "string", "format": "decimal"}}}},
		"paths": {"/boards/{boardId}/notes": {"post": {"operationId": "keepNote",
			"parameters": [{"name": "boardId", "in": "path", "schema": {"type": "string"}}, {"name": "dryRun", "in": "query", "schema": {"type": "boolean"}}],
			"requestBody": {"content": {"application/json": {"schema": {"$ref": "#/entities/Note"}}}},
			"responses": {"201": {"description": "Kept.", "content": {"application/json": {"schema": {"$ref": "#/entities/Note"}}}}}}}},
		"tests": {"keep-note": {"operation": "keepNote", "scenario": "golden", "given": "a board", "when": "a note is kept", "then": "it is kept",
			"input": {"boardId": "b 1", "dryRun": true, "noteText": "hi", "pageCount": "2.50"},
			"expect": {"status": 201, "body": {"noteText": "hi", "pageCount": "2.5"}}}}
	}}`))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestBuildWritesTheWire checks a case's request as it goes on the wire:
// the path parameter written in and escaped, the query parameter as the
// query, the body fields named by info.wireNames, the expected body named
// the same way and its decimal compared as a number.
func TestBuildWritesTheWire(t *testing.T) {
	for _, c := range []struct{ wireNames, body, expect, numeric string }{
		{"", `{"noteText":"hi","pageCount":"2.50"}`, `{"noteText":"hi","pageCount":"2.5"}`, "pageCount"},
		{"snake_case", `{"note_text":"hi","page_count":"2.50"}`, `{"note_text":"hi","page_count":"2.5"}`, "page_count"},
	} {
		p := Build(Input{Request: noteSpecification(t, c.wireNames), Environment: "acceptance"})
		if len(p.Cases) != 1 || p.Cases[0].Request == nil {
			t.Fatalf("wireNames %q: cases %+v", c.wireNames, p.Cases)
		}
		got := p.Cases[0]
		if got.Request.Method != "POST" || got.Request.Path != "/boards/b%201/notes?dryRun=true" || got.Request.Body != c.body {
			t.Errorf("wireNames %q: request %+v", c.wireNames, got.Request)
		}
		if got.Expect.Status != 201 || got.Expect.Body != c.expect || strings.Join(got.Expect.Numeric, ",") != c.numeric {
			t.Errorf("wireNames %q: expect %+v", c.wireNames, got.Expect)
		}
		if why := compareBody(got.Expect.Body, got.Expect.Numeric, []byte(strings.Replace(c.expect, `"2.5"`, `"2.50"`, 1))); why != "" {
			t.Errorf("wireNames %q: a decimal written another way does not compare equal: %s", c.wireNames, why)
		}
	}
}

// TestBuildSkipsTheDxlibDialect checks that an API served in the dxlib
// dialect, whose envelope a package does not write, has its request cases
// skipped with that reason rather than sent in the wrong shape.
func TestBuildSkipsTheDxlibDialect(t *testing.T) {
	p := Build(Input{Request: noteSpecification(t, ""), Environment: "acceptance", Dialect: "dxlib"})
	if len(p.Cases) != 1 || p.Cases[0].Request != nil || !strings.Contains(p.Cases[0].Skip, "dxlib dialect") {
		t.Errorf("the dxlib dialect's case: %+v", p.Cases)
	}
}
