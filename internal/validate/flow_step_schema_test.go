package validate

import (
	"bytes"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/SpecArch/specarch/schema"
)

// flowStepBefore is the flowStep rule as it stood before BUG-063: its if
// passed for a step with no event, so such a step was also told its action
// was missing.
const flowStepBefore = `{
  "type": "object",
  "properties": {
    "page": {"type": "string", "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"},
    "event": {"type": "string", "enum": ["select", "submitted", "action"]},
    "action": {"type": "string", "minLength": 1}
  },
  "required": ["page", "event"],
  "if": {"properties": {"event": {"const": "action"}}},
  "then": {"required": ["action"]},
  "else": {"not": {"required": ["action"]}},
  "patternProperties": {"^x-": {}},
  "additionalProperties": false
}`

// TestFlowStepAcceptsWhatItDid checks that the flowStep rule accepts and
// refuses every combination of page, event and action as it did before
// BUG-063, and that a step with no event fails on event alone.
func TestFlowStepAcceptsWhatItDid(t *testing.T) {
	c := jsonschema.NewCompiler()
	for id, raw := range map[string][]byte{designSchemaID: schema.Definition, "before.json": []byte(flowStepBefore)} {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddResource(id, doc); err != nil {
			t.Fatal(err)
		}
	}
	after, err := c.Compile(designSchemaID + "#/$defs/flowStep")
	if err != nil {
		t.Fatal(err)
	}
	before, err := c.Compile("before.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range []any{nil, "members-list", "Members"} {
		for _, event := range []any{nil, "select", "submitted", "action", "tapped", 1} {
			for _, action := range []any{nil, "Back", "", 1} {
				for _, extension := range []bool{false, true} {
					step := map[string]any{}
					for key, value := range map[string]any{"page": page, "event": event, "action": action} {
						if value != nil {
							step[key] = value
						}
					}
					if extension {
						step["x-note"] = "kept"
					}
					errBefore, errAfter := before.Validate(step), after.Validate(step)
					if (errBefore == nil) != (errAfter == nil) {
						t.Errorf("%v: accepted before %v, after %v", step, errBefore == nil, errAfter == nil)
					}
					if event == nil && page == "members-list" && (action == nil || action == "Back") {
						if got := errAfter.Error(); strings.Contains(got, "action") || !strings.Contains(got, "event") {
							t.Errorf("%v: want only event missing, got %s", step, got)
						}
					}
				}
			}
		}
	}
}
