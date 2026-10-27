// Package specarch carries into the program what it says about itself: the
// definitions of its commands, read from its own specification, and the
// help topics. help, capabilities and guide answer from them, so an
// installed program describes exactly the commands it was built with.
package specarch

import "embed"

// Version is the program version, set at release time. Every plug-in built
// from this repository reports it too.
const Version = "1.3.0"

// Commands holds the specification's command definitions, one fragment per
// file, as spec/design/commands/<command>.yaml.
//
//go:embed spec/design/commands/*.yaml
var Commands embed.FS

// Help holds the help topics, as docs/help/<topic>.txt.
//
//go:embed docs/help/*.txt
var Help embed.FS
