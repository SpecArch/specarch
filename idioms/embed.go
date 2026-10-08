// Package idioms embeds the idioms SpecArch ships, so a project that
// installs one release has one idiom set. docs/idioms.md is the design.
package idioms

import "embed"

// Shipped holds every shipped idiom, as idioms/<concern>/<name>.specarch-idiom.yaml.
//
//go:embed */*.specarch-idiom.yaml
var Shipped embed.FS
