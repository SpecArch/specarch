package genui

import (
	"fmt"
	"strconv"
	"strings"
)

// Block is one block of a content page, as every UI generator reads it
// (ADR-181): a heading, a paragraph, an image or a link.
type Block struct {
	// Kind is heading, text, image or link.
	Kind string
	// Level is a heading's level, 1 to 6.
	Level int
	// Text is a heading's or a paragraph's text, an image's alternative
	// text or a link's label.
	Text string
	// Path is where an image is served.
	Path string
	// Page is the page of the specification a link opens, and Href the
	// address outside it, one of the two.
	Page, Href string
	// At is the block's pointer.
	At string
}

// BlockProblem is what keeps a generator from writing a block, at its
// pointer.
type BlockProblem struct {
	At, Message string
}

// ContentBlocks reads the blocks of a content page. A block a generator
// cannot write as the design asks is a problem, named at it: an image
// without its alternative text, which a screen reader would have nothing
// to read for (WCAG 2.2, 1.1.1), a link without its label (2.4.4), and a
// link to a page that is not in the specification or whose route takes a
// parameter, which a content page has no record to give. The validator
// reports each too; a generator refuses the page rather than write it
// short of the design.
func ContentBlocks(name string, pg, pages map[string]any) ([]Block, []BlockProblem) {
	var blocks []Block
	var problems []BlockProblem
	for i, b := range list(pg["blocks"]) {
		bm := obj0(b)
		at := fmt.Sprintf("/pages/%s/blocks/%d", name, i)
		switch {
		case bm["heading"] != nil:
			h := obj0(bm["heading"])
			level, _ := strconv.Atoi(text(h["level"]))
			blocks = append(blocks, Block{Kind: "heading", Level: level, Text: text(h["text"]), At: at})
		case bm["text"] != nil:
			blocks = append(blocks, Block{Kind: "text", Text: text(bm["text"]), At: at})
		case bm["image"] != nil:
			img := obj0(bm["image"])
			alt := text(img["alternativeText"])
			if alt == "" {
				problems = append(problems, BlockProblem{at + "/image", fmt.Sprintf("%s shows the image %s with no alternativeText, and an image is written only with the text read in its place to who cannot see it (WCAG 2.2, 1.1.1); give it its alternativeText", name, text(img["path"]))})
				continue
			}
			blocks = append(blocks, Block{Kind: "image", Path: text(img["path"]), Text: alt, At: at})
		case bm["link"] != nil:
			l := obj0(bm["link"])
			page, href, label := text(l["page"]), text(l["href"]), text(l["label"])
			to := page
			if to == "" {
				to = href
			}
			if label == "" {
				problems = append(problems, BlockProblem{at + "/link", fmt.Sprintf("the link of %s to %s has no label, and a link is written only with the text that says where it leads (WCAG 2.2, 2.4.4); give it a label", name, to)})
				continue
			}
			if page != "" {
				target, ok := obj(pages[page])
				if !ok {
					problems = append(problems, BlockProblem{at + "/link/page", fmt.Sprintf("the link %s of %s opens %s, which is not a page of the specification", label, name, page)})
					continue
				}
				if ps := routeParameters(text(target["route"])); len(ps) > 0 {
					problems = append(problems, BlockProblem{at + "/link/page", fmt.Sprintf("the link %s of %s opens %s, whose route takes {%s}, which a content page has no record to give", label, name, page, strings.Join(ps, "} and {"))})
					continue
				}
			}
			blocks = append(blocks, Block{Kind: "link", Page: page, Href: href, Text: label, At: at})
		}
	}
	return blocks, problems
}
