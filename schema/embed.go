// Package schema embeds the meta-model's JSON Schemas, the same files
// editors load, so the validator and the editors share one definition.
package schema

import _ "embed"

// Definition is the schema of a SpecArch Definition File on meta-model
// 1.0.
//
//go:embed specarch-design-1.0.schema.json
var Definition []byte

// DefinitionEarlier is the schema of a Definition File on an earlier
// meta-model, 0.1, 0.2 or 0.3, read beside 1.0 with a warning.
//
//go:embed specarch-design-0.3.schema.json
var DefinitionEarlier []byte

// Implementation is the schema of a SpecArch Implementation File.
//
//go:embed specarch-implementation-1.0.schema.json
var Implementation []byte

// ImplementationEarlier is the schema of an Implementation File on 0.1, the format before 1.0.0,
// read beside it.
//
//go:embed specarch-implementation-0.1.schema.json
var ImplementationEarlier []byte

// Record is the schema of a SpecArch record: a change, defect, release,
// incident, commissioning run or approval.
//
//go:embed specarch-record-1.0.schema.json
var Record []byte

// RecordEarlier is the schema of a record on 0.1, the format before 1.0.0,
// read beside it.
//
//go:embed specarch-record-0.1.schema.json
var RecordEarlier []byte

// Idiom is the schema of an idiom, shipped or a project's.
//
//go:embed specarch-idiom-1.0.schema.json
var Idiom []byte

// IdiomEarlier is the schema of an idiom on 0.1, the format before 1.0.0,
// read beside it.
//
//go:embed specarch-idiom-0.1.schema.json
var IdiomEarlier []byte

// Estate is the schema of a SpecArch estate: the modules' specifications
// it lists and pins.
//
//go:embed specarch-estate-1.0.schema.json
var Estate []byte

// EstateEarlier is the schema of an estate on 0.1, the format before 1.0.0,
// read beside it.
//
//go:embed specarch-estate-0.1.schema.json
var EstateEarlier []byte
