package extract

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// TestPatternCovers checks inclusion by the matchers' one rule (ADR-100).
func TestPatternCovers(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"/v1/*", "/v1/items", true},
		{"/v1/*", "/v1/items/{id}", true},
		{"/v1/*", "/v1/reports/*", true},
		{"/v1/*", "/v1", false},
		{"/v1/{kind}", "/v1/items", true},
		{"/v1/items", "/v1/{kind}", false},
		{"/v1/{kind}", "/v1/*", false},
		{"/v1/{kind}/export", "/v1/reports/*", false},
		{"/v1/reports/*", "/v1/{kind}/export", false},
		{"/v1/items", "/v1/items/{id}", false},
	} {
		if got := routePattern(c.a).covers(routePattern(c.b)); got != c.want {
			t.Errorf("%s covers %s: got %v, want %v", c.a, c.b, got, c.want)
		}
	}
	colon, _ := readPattern("/v1/:id", "colon")
	if colon.key() != routePattern("/v1/{id}").key() {
		t.Errorf("a colon parameter and a brace parameter give different keys: %s and %s", colon.key(), routePattern("/v1/{id}").key())
	}
	if _, why := readPattern("/v1/items*", "colon"); why == "" {
		t.Error("a * inside a segment is read")
	}
}

// TestRuleNames checks the fixed naming rule, and that names that meet
// take their parameters' words.
func TestRuleNames(t *testing.T) {
	rule := func(role, pattern, methods string) dumpRule {
		return dumpRule{Role: role, Pattern: pattern, Methods: json.RawMessage(methods), Matcher: "brace"}
	}
	pr := &permissionReader{
		t: &permissionTable{Path: "access", Rules: []dumpRule{
			rule("desk-staff", "/v1/items", `["GET"]`),
			rule("desk-staff", "/v1/items/{itemId}", `["GET", "DELETE"]`),
			rule("desk-staff", "/v1/*", `"*"`),
			rule("desk-staff", "/2fa/*", `"*"`),
		}},
		key: "code", res: &Result{}, questions: &yaml.Node{Kind: yaml.MappingNode}, wireOf: map[string]string{},
	}
	byRole, err := pr.readRules("permissions.json", map[string]bool{}, map[string]map[string][]string{}, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	got := byRole["desk-staff"]
	slices.Sort(got)
	want := []string{"v1.all", "v1.items.delete", "v1.items.get", "v1.items.itemid.get"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	all := pr.rulePermission("v1.all")
	if all == nil || !slices.Equal(all.implies, []string{"v1.items.delete", "v1.items.get", "v1.items.itemid.get"}) {
		t.Errorf("v1.all implies %v", all)
	}
	if len(pr.notHeld) != 1 || !strings.Contains(pr.notHeld[0].text, "the word 2fa does not start with a letter") {
		t.Errorf("not held: %v", pr.notHeld)
	}
	if _, err := (&permissionReader{t: &permissionTable{Rules: []dumpRule{{Role: "a", Pattern: "/x", Methods: json.RawMessage(`"*"`), Matcher: "glob"}}}, res: &Result{}, wireOf: map[string]string{}}).readRules("p.json", map[string]bool{}, map[string]map[string][]string{}, map[string]bool{}); err == nil || !strings.Contains(err.Error(), "glob") {
		t.Errorf("an unknown matcher: %v", err)
	}
	if _, err := (&permissionReader{t: &permissionTable{Rules: []dumpRule{{Role: "a", Pattern: "/x", Methods: json.RawMessage(`"*"`), Matcher: "colon", Through: "Admin_Web"}}}, res: &Result{}, wireOf: map[string]string{}}).readRules("p.json", map[string]bool{}, map[string]map[string][]string{}, map[string]bool{}); err == nil || !strings.Contains(err.Error(), "p.json, rule 1") || !strings.Contains(err.Error(), "Admin_Web") {
		t.Errorf("a through not in kebab-case: %v", err)
	}
}
