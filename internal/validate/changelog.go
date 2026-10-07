package validate

import (
	"regexp"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// changeLogPhrase finds the few phrases that turn a description into a
// change log. The list is short on purpose: it catches the obvious cases and
// never a sentence about the design itself.
var changeLogPhrase = regexp.MustCompile(`(?i)\b(previously|formerly|changed from|was changed|updated on|new in (version|v?[0-9]))\b`)

var proseKeys = map[string]bool{
	"description": true, "summary": true, "title": true, "context": true,
	"decision": true, "consequences": true, "note": true, "message": true,
}

// checkChangeLog warns when prose in the file tells how it changed instead
// of what is true now.
func (c *checker) checkChangeLog() {
	walk(c.root, nil, func(n *yaml.Node, path []string) {
		for _, p := range source.Pairs(n) {
			if !proseKeys[p.Key.Value] || !source.IsScalar(p.Value) {
				continue
			}
			if m := changeLogPhrase.FindString(p.Value.Value); m != "" {
				c.warn(p.Value, source.Pointer(append(path, p.Key.Value)...), RuleChangeLog,
					"%q reads as a change log; say what is true now, and record the change in the project's history instead", m)
			}
		}
	})
}
