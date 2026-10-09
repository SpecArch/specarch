package extract

import (
	"testing"

	"go.yaml.in/yaml/v3"
)

// TestPresenceForm reads a presence check the same whatever its order and
// grouping, as generate sql and the database reader write it, and refuses
// any other check.
func TestPresenceForm(t *testing.T) {
	generated, ok := presenceForm("(address_city == null && address_street == null) || (address_city != null && address_street != null)", camel)
	if !ok {
		t.Fatal("the check generate sql writes is not read as a presence check")
	}
	read, ok := presenceForm("addressStreet == null && addressCity == null || addressStreet != null && addressCity != null", nil)
	if !ok || read != generated {
		t.Errorf("the database reader's check reads %q, and generate sql's %q", read, generated)
	}
	for _, other := range []string{
		"addressStreet == null || size(addressCity) > 0",
		"(addressStreet == null || addressCity != null) && addressCity == null",
		"addressStreet == addressCity",
	} {
		if form, ok := presenceForm(other, nil); ok && form == generated {
			t.Errorf("%q is read as the presence check", other)
		}
	}
	if _, ok := presenceForm("(addressStreet == null || addressCity != null) && addressCity == null", nil); ok {
		t.Error("an or inside an and is read as a presence check")
	}
}

// TestOpenAPIValueInColumns writes an entity's property that refers to a
// schema with a required part as a reference with no storage, since the
// design keeps it in columns by default.
func TestOpenAPIValueInColumns(t *testing.T) {
	var doc yaml.Node
	src := `
openapi: 3.0.3
paths:
  /members:
    post:
      responses:
        "201": {description: Made., content: {application/json: {schema: {$ref: "#/components/schemas/Member"}}}}
components:
  schemas:
    Member: {type: object, properties: {card: {type: string}, address: {$ref: "#/components/schemas/Address"}, phones: {type: array, items: {$ref: "#/components/schemas/Phone"}}}}
    Address: {type: object, properties: {street: {type: string}, city: {type: string}}, required: [street]}
    Phone: {type: object, properties: {number: {type: string}}, required: [number]}
`
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	o := &openapiReader{doc: doc.Content[0], key: "api", res: &Result{Tree: newTree()}, questions: &yaml.Node{Kind: yaml.MappingNode}}
	o.v30 = true
	o.read()
	props := child(child(o.entities, "Member"), "properties")
	if address := child(props, "address"); scalar(child(address, "$ref")) != "#/schemas/Address" || child(address, "storage") != nil {
		t.Errorf("address is written %s; want a reference to Address with no storage", inlineNode(address))
	}
	if phones := child(props, "phones"); scalar(child(child(phones, "items"), "$ref")) != "#/schemas/Phone" || child(phones, "storage") != nil {
		t.Errorf("phones is written %s; want a list of Phone, which is json with no storage said", inlineNode(phones))
	}
}
