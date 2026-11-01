package genuits

import "github.com/SpecArch/specarch/internal/genscreens"

// The schema is printed through genscreens's printer, which lays a value
// out as prettier lays out an object literal.
var (
	flat      = genscreens.Flat
	quote     = genscreens.Quote
	key       = genscreens.Key
	statement = genscreens.Statement
)
