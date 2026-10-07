package generate

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// row is one element shown as a row of a table: the name the reader sees
// in the row, and the element's node.
type row struct {
	label string
	node  *yaml.Node
}

// explain writes an element's Insight, from its why, and one Note for each
// of its citations, under the element's own heading.
func (d *doc) explain(n *yaml.Node) {
	d.annotate("", n)
}

// explainRows writes, after a table, the Insight and Notes of every row
// that has them, each labelled with the row's name, since a table cell
// cannot hold a paragraph.
func (d *doc) explainRows(rows []row) {
	for _, r := range rows {
		d.annotate(r.label, r.node)
	}
}

func (d *doc) annotate(label string, n *yaml.Node) {
	on := ""
	if label != "" {
		on = " on " + label
	}
	if o := d.origin(n); o != "" {
		if label != "" {
			o = strings.Replace(o, "**Origin:**", "**Origin"+on+":**", 1)
		}
		d.para(o)
	}
	d.open(label, n)
	if why := strings.TrimSpace(str(n, "why")); why != "" {
		d.para(fmt.Sprintf("**Insight%s:** %s", on, oneParagraph(why)))
	}
	for _, c := range items(n, "cites") {
		d.para(fmt.Sprintf("**Note%s:** %s", on, d.citation(c)))
	}
}

// citation is a Note's text: from the source's title and edition, the
// clause, what it says, and where to read it.
func (d *doc) citation(c *yaml.Node) string {
	key := str(c, "source")
	src := get(d.sources, key)
	if d.cited == nil {
		d.cited = map[string]bool{}
	}
	d.cited[key] = true
	from := strings.TrimSpace(str(src, "title"))
	if from == "" {
		from = key
	}
	if e := str(src, "edition"); e != "" {
		from += ", " + e
	}
	if cl := str(c, "clause"); cl != "" {
		from += ", clause " + cl
	}
	text := "From " + from + ": " + oneParagraph(str(c, "says"))
	if u := str(src, "url"); u != "" && str(src, "kind") != "requirement-set" {
		text += " <" + u + ">"
	}
	return text
}

// sourcesIndex lists every source the document cited in a Note.
func (d *doc) sourcesIndex(title string) {
	if len(d.cited) == 0 {
		return
	}
	keys := make([]string, 0, len(d.cited))
	for k := range d.cited {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	d.heading(2, title)
	d.para("Every source a Note in this document cites.")
	d.line("| Source | Title | Edition | Author | Where to read it |")
	d.line("|---|---|---|---|---|")
	for _, k := range keys {
		s := get(d.sources, k)
		d.line("| %s | %s | %s | %s | %s |", k, cell(str(s, "title")), cell(str(s, "edition")), cell(str(s, "author")), cell(str(s, "url")))
	}
	d.blank()
}

// oneParagraph joins the lines of a block scalar into one paragraph.
func oneParagraph(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// pairRows makes the rows of a section's named objects.
func pairRows(n *yaml.Node, key string) []row {
	var out []row
	for _, p := range pairs(n, key) {
		out = append(out, row{p.Key.Value, p.Value})
	}
	return out
}

// rowsOf makes rows of named objects already listed.
func rowsOf(ps []source.Pair) []row {
	var out []row
	for _, p := range ps {
		out = append(out, row{p.Key.Value, p.Value})
	}
	return out
}

// stepRows makes the rows of a list of steps, by number and name.
func stepRows(steps *yaml.Node) []row {
	var out []row
	for i, s := range itemsOf(steps) {
		out = append(out, row{fmt.Sprintf("step %d, %s", i+1, str(s, "name")), s})
	}
	return out
}
