package validate

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// derive --fill names why a case keeps its status alone, one reason per
// cause, never one cause in place of another: a record no field names, a
// required field no value can be made for, a value derive does not make,
// a value its field does not take, and a permission no role holds.
func TestFillNamesWhyACaseKeepsItsStatusAlone(t *testing.T) {
	const text = `
specarch: "1.0"
info: { title: Case, version: 1.0.0 }
permissions:
  things.read: { description: See things. }
  codes.write: { description: Write codes. }
  things.archive: { description: Archive things. }
roles:
  clerk: { description: Staff., permissions: [things.read, codes.write] }
  visitor: { description: Someone looking., permissions: [things.read] }
enums:
  Shade: { enum: [light, dark] }
paths:
  /things/{thingNumber}:
    get:
      operationId: getThing
      permission: things.read
      parameters:
        - { name: thingNumber, in: path, required: true, schema: { type: string, maxLength: 12 } }
      responses:
        "200": { description: The thing. }
        "403": { description: Not allowed. }
        "404": { description: No such thing. }
  /codes:
    post:
      operationId: writeCode
      permission: codes.write
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                code: { type: string, pattern: "^[A-Z]{3}$" }
              required: [code]
      responses:
        "201": { description: Written. }
        "400": { description: The request cannot be read. }
        "403": { description: Not allowed. }
  /shades:
    post:
      operationId: pickShade
      permission: codes.write
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                shade: { $ref: "#/enums/Shade" }
              required: [shade]
      responses:
        "201": { description: Picked. }
        "400": { description: The request cannot be read. }
        "403": { description: Not allowed. }
  /archive:
    post:
      operationId: archiveThing
      permission: things.archive
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name: { type: string, maxLength: 20 }
              required: [name]
      responses:
        "201": { description: Archived. }
        "400": { description: The request cannot be read. }
        "403": { description: Not allowed. }
tests:
  get-thing-unknown:
    operation: getThing
    scenario: red
    covers: [not found thingNumber, denied without things.read]
  write-code-bad:
    operation: writeCode
    scenario: red
    covers: [missing code, code not matching its pattern, response 403]
  pick-shade-bad:
    operation: pickShade
    scenario: red
    covers: [shade not one of its values, response 403]
  archive-thing-missing-name:
    operation: archiveThing
    scenario: red
    covers: [missing name, response 403]
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		t.Fatal(err)
	}
	reasons := map[string]string{}
	for _, f := range Fills(doc.Content[0], nil) {
		reasons[f.Name] = f.Reason
	}
	for name, want := range map[string][]string{
		"get-thing-unknown": {"its case not found thingNumber keeps its status alone because derive finds no record that thingNumber names", "would leave it unchecked"},
		"write-code-bad": {
			"its case missing code keeps its status alone because derive can make no value for its required field code (an object, a list or a text with a pattern), so the call cannot be made",
			"its case code not matching its pattern keeps its status alone because derive makes no value for it: it does not make a text that does not match a pattern",
			"would leave them unchecked",
		},
		"pick-shade-bad":             {"its case shade not one of its values keeps its status alone because the value it is about is not one shade takes"},
		"archive-thing-missing-name": {"its case missing name keeps its status alone because no role holds things.archive, so no caller may make the call"},
	} {
		got, ok := reasons[name]
		if !ok {
			t.Errorf("%s: not listed by Fills, and it should be left as it is with a reason", name)
			continue
		}
		for _, w := range want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: the reason does not say %q\ngot: %s", name, w, got)
			}
		}
	}
}
