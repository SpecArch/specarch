package validate

import (
	"fmt"
	"slices"
	"strings"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// checkTargetFormats checks the formats each target of an implementation
// file names (ADR-249): only a document target that writes more than one
// form takes them, and each must be one it writes. A value outside md,
// docx and pdf is the schema's to report.
func (c *checker) checkTargetFormats() {
	for _, t := range source.Pairs(source.Child(c.root, "targets")) {
		key := source.Key(t.Value, "formats")
		node := source.Child(t.Value, "formats")
		if key == nil {
			continue
		}
		target := t.Key.Value
		at := source.Pointer("targets", target, "formats")
		has := spec.FormatsOf(target)
		if has == nil {
			if slices.Contains(spec.DocumentTargets, target) {
				c.add(key, at, RuleDocumentFormat, "%s writes %s only and takes no formats; remove formats", target, spec.OwnFiles(target))
			} else {
				c.add(key, at, RuleDocumentFormat, "%s is not a document target, so formats says nothing for it; formats is read only for a document target and for books; remove it", target)
			}
			continue
		}
		for i, item := range source.Items(node) {
			f := item.Value
			if !slices.Contains(spec.DocumentFormats, f) || slices.Contains(has, f) {
				continue
			}
			p := source.Pointer("targets", target, "formats", fmt.Sprint(i))
			if target == "books" && f == "md" {
				c.add(item, p, RuleDocumentFormat, "books has no format md: a book is made from the Markdown documents, which each document target writes as <target>.md; its formats are pdf and docx")
				continue
			}
			c.add(item, p, RuleDocumentFormat, "%s has no format %s; its formats are %s", target, f, strings.Join(has, ", "))
		}
	}
}
