// Package schema embeds the meta-model's JSON Schemas, the same files
// editors load, so the validator and the editors share one definition.
package schema

import _ "embed"

// Definition is the schema of a SpecArch Definition File.
//
//go:embed specarch-design-0.1.schema.json
var Definition []byte

// Implementation is the schema of a SpecArch Implementation File.
//
//go:embed specarch-implementation-0.1.schema.json
var Implementation []byte
