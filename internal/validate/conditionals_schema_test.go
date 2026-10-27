package validate

import (
	"bytes"
	"maps"
	"slices"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/schema"
)

// conditional is a rule of the design schema whose if reads a key: on 1.0
// the if also requires the key, so an object without it is told only that
// it is missing.
type conditional struct {
	def string
	// key is the key the if reads, values every value it is tried with.
	key    string
	values []any
	// keys are the keys the branches ask for or refuse, each tried absent
	// and as the object found gives it.
	keys []string
	// objects finds the objects of the kind in a specification's value.
	objects func(root map[string]any) []map[string]any
	// added are the keys 1.0 added to the kind, which the 0.3 schema does
	// not know; they are taken out before the two are compared.
	added []string
}

// TestConditionalsAcceptWhatTheyDid checks that every conditional fixed for
// 1.0 accepts and refuses each object as the 0.3 rule does: objects of the
// kind from the library lending example and from SpecArch's own
// specification, with the key the if reads absent or given each value, and
// each key a branch asks for or refuses absent or kept. Only the messages
// change, never which file is valid (docs/meta-model-1.0.0.md).
// workflowStepAdded are the keys 1.x gave a workflow step beside its
// approvals and operations (ADR-198); none decides an approval or an
// operation step, which the two rules compare.
var workflowStepAdded = []string{"performers", "onFailure", "compensatedBy", "branches", "after", "at", "message"}

func TestConditionalsAcceptWhatTheyDid(t *testing.T) {
	c := jsonschema.NewCompiler()
	for id, raw := range map[string][]byte{designSchemaID: schema.Definition, earlierDesignSchemaID: schema.DefinitionEarlier} {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddResource(id, doc); err != nil {
			t.Fatal(err)
		}
	}
	var roots []map[string]any
	for _, dir := range []string{"../../examples/library-lending/spec", "../../spec"} {
		s := spec.Load(dir)
		root, ok := s.Value.(map[string]any)
		if !ok {
			t.Fatalf("%s does not load", dir)
		}
		roots = append(roots, root)
	}
	rules := []conditional{
		{def: "source", key: "kind", values: []any{"requirement-set", "change-set", "defect-set", "code", "standard", "other", 1}, keys: []string{"prefix", "reading"},
			objects: func(r map[string]any) []map[string]any { return mapsOf(r["sources"]) }},
		{def: "constraint", key: "kind", values: []any{"unique", "check", "other"}, keys: []string{"fields", "expression", "where"},
			objects: func(r map[string]any) []map[string]any {
				var out []map[string]any
				for _, e := range mapsOf(r["entities"]) {
					out = append(out, mapsOf(e["constraints"])...)
				}
				return out
			}},
		{def: "parameter", key: "in", values: []any{"path", "query", "header", "cookie"}, keys: []string{"required"},
			objects: func(r map[string]any) []map[string]any {
				var out []map[string]any
				for _, p := range mapsOf(r["paths"]) {
					for _, op := range mapsOf(p) {
						out = append(out, listOf(op["parameters"])...)
					}
				}
				return out
			}},
		{def: "page", key: "kind", values: []any{"list", "form", "view", "task", "content", "custom", "other"}, keys: []string{"columns", "source", "entity", "submit", "blocks", "onSubmitted"},
			// fieldConditions are left out: 1.0 gave a field's condition
			// readOnlyUnless and hiddenUnless, and none decides the kind. A
			// page of signedIn, a name 1.0's permission pattern admits and
			// 0.3's refuses, is left out too.
			objects: func(r map[string]any) []map[string]any {
				var out []map[string]any
				for _, pg := range mapsOf(r["pages"]) {
					if pg["permission"] != "signedIn" {
						out = append(out, pg)
					}
				}
				return out
			}, added: []string{"fieldConditions"}},
		{def: "workflowStep", key: "kind", values: []any{"approval", "operation", "other"}, keys: []string{"approvers", "permission", "deadline", "onDeadline", "escalateTo", "operation"},
			objects: func(r map[string]any) []map[string]any {
				var out []map[string]any
				for _, w := range mapsOf(r["workflows"]) {
					out = append(out, listOf(w["steps"])...)
				}
				return out
			}, added: workflowStepAdded},
		{def: "workflowStep", key: "onDeadline", values: []any{"refuse", "escalate", "other"}, keys: []string{"escalateTo"},
			objects: func(r map[string]any) []map[string]any {
				var out []map[string]any
				for _, w := range mapsOf(r["workflows"]) {
					out = append(out, listOf(w["steps"])...)
				}
				return out
			}, added: workflowStepAdded},
		{def: "decision", key: "status", values: []any{"proposed", "accepted", "deprecated", "superseded", "other"}, keys: []string{"supersededBy"},
			objects: func(r map[string]any) []map[string]any { return mapsOf(r["decisions"]) }, added: []string{"restsOn"}},
	}
	for _, rule := range rules {
		after, err := c.Compile(designSchemaID + "#/$defs/" + rule.def)
		if err != nil {
			t.Fatal(err)
		}
		before, err := c.Compile(earlierDesignSchemaID + "#/$defs/" + rule.def)
		if err != nil {
			t.Fatal(err)
		}
		tried, accepted := 0, 0
		for _, root := range roots {
			for _, found := range rule.objects(root) {
				for _, value := range append([]any{nil}, rule.values...) {
					for mask := 0; mask < 1<<len(rule.keys); mask++ {
						obj := maps.Clone(found)
						delete(obj, rule.key)
						for _, k := range rule.added {
							delete(obj, k)
						}
						if value != nil {
							obj[rule.key] = value
						}
						for i, k := range rule.keys {
							if mask&(1<<i) != 0 {
								delete(obj, k)
							}
						}
						errBefore, errAfter := before.Validate(obj), after.Validate(obj)
						tried++
						if errBefore == nil {
							accepted++
						}
						if (errBefore == nil) != (errAfter == nil) {
							t.Errorf("%s %v: accepted on 0.3 %v, on 1.0 %v", rule.def, obj, errBefore == nil, errAfter == nil)
						}
					}
				}
			}
		}
		if accepted == 0 {
			t.Errorf("%s on %s: none of the %d objects tried is valid, so the comparison proves nothing; add a specification that has one", rule.def, rule.key, tried)
		}
	}
}

func mapsOf(v any) []map[string]any {
	m, _ := v.(map[string]any)
	var out []map[string]any
	for _, k := range sortedMapKeys(m) {
		if o, ok := m[k].(map[string]any); ok {
			out = append(out, o)
		}
	}
	return out
}

func listOf(v any) []map[string]any {
	l, _ := v.([]any)
	var out []map[string]any
	for _, x := range l {
		if o, ok := x.(map[string]any); ok {
			out = append(out, o)
		}
	}
	return out
}

func sortedMapKeys(m map[string]any) []string {
	return slices.Sorted(maps.Keys(m))
}
