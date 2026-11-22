package pdf

import "github.com/SpecArch/specarch/internal/mdbook"

// The book's model is mdbook's, which the Word writer reads too.
type (
	block    = mdbook.Block
	span     = mdbook.Span
	tocEntry = mdbook.TOCEntry
)

const (
	headingBlock   = mdbook.HeadingBlock
	paragraphBlock = mdbook.ParagraphBlock
	tableBlock     = mdbook.TableBlock
	listBlock      = mdbook.ListBlock
	codeBlock      = mdbook.CodeBlock
	quoteBlock     = mdbook.QuoteBlock
	ruleBlock      = mdbook.RuleBlock
	strong         = mdbook.Strong
	emphasis       = mdbook.Emphasis
	monoStyle      = mdbook.Mono
)

var plainText = mdbook.PlainText
