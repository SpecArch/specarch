// Package vocabularies embeds the vocabularies SpecArch ships: the words
// of one language that extract documents and merge read a document by.
// docs/vocabularies.md is the format.
package vocabularies

import "embed"

// Shipped holds every shipped vocabulary, as <language>.specarch-vocabulary.yaml.
//
//go:embed *.specarch-vocabulary.yaml
var Shipped embed.FS
