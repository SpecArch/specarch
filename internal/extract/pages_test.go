package extract

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseSchemaFile(t *testing.T) {
	o, err := parseSchemaFile(`// A list.
import type { PageSchema } from "@example/screens";
import "./styles.css"
import { hook } from './page.hooks';

export default {
  kind: 'list',
  "title": "Loans A\n",
  count: -1.5e2,
  open: true, gone: null,
  columns: ["a", "b",],
  nested: { deep: [1, { x: hook }] },
} as const satisfies PageSchema<"list", Record<string, 1>>;
`)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(o.keys, []string{"kind", "title", "count", "open", "gone", "columns", "nested"}) {
		t.Fatalf("keys %v", o.keys)
	}
	if o.values["title"] != "Loans A\n" || o.values["kind"] != "list" || o.values["count"] != (numeral{"-1.5e2"}) || o.values["open"] != true || o.values["gone"] != nil {
		t.Fatalf("values %v", o.values)
	}
	if o.lines["columns"] != 11 {
		t.Fatalf("columns is on line %d, not 11", o.lines["columns"])
	}
	deep := o.values["nested"].(*object).values["deep"].([]any)
	if deep[1].(*object).values["x"] != (reference{"hook"}) {
		t.Fatalf("a bare name is not a reference: %v", deep[1])
	}
}

func TestParseSchemaFileOutside(t *testing.T) {
	for src, want := range map[string]string{
		"export const a = { t: `x` };":                     "line 1: a template literal",
		"export const a = { b: 1 };\nexport const c = {};": "line 2: a second export",
		"const a = {};":                           "neither an import nor the export",
		"export const a = [1];":                   "not an object literal",
		"export const a = { b, };":                "shorthand",
		"export const a = { b: 1, b: 2 };":        "given twice",
		"export const a = { b: f() };":            "only a literal or a bare name",
		"export const a = { ...b };":              "a key is a name",
		"export const a = { b: 0x10 };":           "not a decimal number",
		"export const a = { b: 'x\\q' };":         "escape",
		"export const a = { b: 1 } satisfies T x": "\"x\" follows the exported literal",
		"// nothing":                              "exports no object literal",
	} {
		_, err := parseSchemaFile(src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want %q", src, err, want)
		}
	}
}

func TestFolderRoute(t *testing.T) {
	for folder, want := range map[string][2]string{
		".":                                      {"/", ""},
		"(shop)/orders/[orderId]/(detail)/items": {"/orders/{orderId}/items", "orders by order-id items"},
		"[a]/x/[a]":                              {"", "the parameter a appears twice"},
		"[a-b]":                                  {"", "is not an identifier"},
		"docs/[[...slug]]":                       {"", "catch-all"},
		"@modal/x":                               {"", "parallel route"},
		"(..)photo":                              {"", "intercepting route"},
		"_lib":                                   {"", "private"},
		"a b":                                    {"", "neither a fixed word"},
		"docs/[[id]]":                            {"", "optional parameter"},
	} {
		route, words, reason := segmentRoute(folderSegments(folder), "folder", "a page's route", routerApp)
		if want[0] != "" && (route != want[0] || strings.Join(words, " ") != want[1]) {
			t.Errorf("%s: got %s %v %q, want %s %s", folder, route, words, reason, want[0], want[1])
		}
		if want[0] == "" && !strings.Contains(reason, want[1]) {
			t.Errorf("%s: got %q, want %q", folder, reason, want[1])
		}
	}
}

func TestSegmentRouteOtherRouters(t *testing.T) {
	for _, c := range []struct {
		segs         []string
		router, want string
	}{
		{[]string{"(group)", "x"}, routerNuxtPages, "/x"},
		{[]string{"(group)", "x"}, routerNextPages, "neither a fixed word nor one whole parameter"},
		{[]string{"_lib"}, routerNextPages, "/_lib"},
		{[]string{"api", "loans", "[id]"}, routerNuxtServer, "/api/loans/{id}"},
		{[]string{"user-[id]"}, routerNuxtPages, "neither a fixed word, a group nor one whole parameter"},
	} {
		route, _, reason := segmentRoute(c.segs, "segment", "a path", c.router)
		got := route
		if reason != "" {
			got = reason
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("%v on %s: got %q, want %q", c.segs, c.router, got, c.want)
		}
	}
}
