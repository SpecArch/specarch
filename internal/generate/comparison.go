package generate

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// Comparison writes the old documents against the refined specification
// (ADR-157): what was kept from the sources, what was clarified, what a
// stakeholder added or corrected, and which clauses of the sources
// produced nothing, each derived from the elements' origin, their
// citations and the decisions. It is never a draft: it is read while the
// questions are open, like the questions document.
func Comparison(root *yaml.Node, relRoot string, impls []Implementation, state *State) string {
	d := newDoc("comparison", root, relRoot, impls, state)
	info := get(root, "info")
	els := namedElements(root)
	var stated, inferred, decided, none []element
	for _, e := range els {
		switch str(e.node, "origin") {
		case "stated":
			stated = append(stated, e)
		case "inferred":
			inferred = append(inferred, e)
		case "decided":
			decided = append(decided, e)
		default:
			none = append(none, e)
		}
	}
	added, corrected := 0, 0
	for _, e := range decided {
		if d.corrects(e.node) {
			corrected++
		} else {
			added++
		}
	}
	outlined := d.outlines(root)
	dropped := 0
	for _, o := range outlined {
		dropped += len(o.dropped)
	}

	d.line("# %s: the old documents against the refined specification", str(info, "title"))
	d.blank()
	summary := fmt.Sprintf("Version %s of the specification against its %s: %s kept from them, %s clarified, %s added, %s corrected, and of the clauses the sources list, %s produced nothing.",
		str(info, "version"), countText(len(pairs(root, "sources")), "source", "sources"),
		countText(len(stated), "element", "elements"), countText(len(inferred), "element", "elements"),
		countText(added, "element", "elements"), countText(corrected, "element", "elements"),
		countText(dropped, "clause", "clauses"))
	if len(none) > 0 {
		summary += fmt.Sprintf(" %s without an origin %s left out of the four groups.", countText(len(none), "element", "elements"), isAre(len(none)))
	}
	d.para(summary)
	d.para("Kept: a source says it, and the element cites where. Clarified: the sources left it implicit, and the element says from what it was concluded. Added or corrected: a stakeholder decided it, answering a question; corrected when the element also cites a source, which said something else there, added when no source said anything. Dropped: a clause of a source's outline that no element cites, either not read yet or holding nothing the specification needs; a citation from a decision or a question does not count, since neither is compared, and a clause only a question cites is named as asked about.")
	d.para(d.withheld.audienceNotice())
	d.problemsNotice("comparison")

	d.section("Kept")
	if len(stated) == 0 {
		d.para("No element is stated by a source.")
	} else {
		bySource := map[string][]element{}
		var uncited []element
		// An element citing two sources, or a named and a nameless one, is
		// in two tables; its marks are written once, under the first.
		marked := map[string]bool{}
		marksOnce := func(rows []element) {
			var first []element
			for _, e := range rows {
				if !marked[e.label] {
					marked[e.label] = true
					first = append(first, e)
				}
			}
			d.rowMarks(first)
		}
		for _, e := range stated {
			cites := items(e.node, "cites")
			if len(cites) == 0 {
				uncited = append(uncited, e)
				continue
			}
			seen := map[string]bool{}
			for _, c := range cites {
				key := str(c, "source")
				if !seen[key] {
					seen[key] = true
					bySource[key] = append(bySource[key], e)
				}
			}
		}
		for _, p := range pairs(root, "sources") {
			key := p.Key.Value
			in := bySource[key]
			if len(in) == 0 {
				continue
			}
			d.cite(key)
			d.heading(3, fmt.Sprintf("%s (%s)", d.sourceTitle(key), key))
			d.line("| Element | Clause | Says |")
			d.line("|---|---|---|")
			for _, e := range in {
				for _, c := range items(e.node, "cites") {
					if str(c, "source") != key {
						continue
					}
					d.line("| %s | %s | %s |", cell(e.label), cell(str(c, "clause")), cell(oneParagraph(str(c, "says"))))
				}
			}
			d.blank()
			marksOnce(in)
		}
		if in := bySource[""]; len(in) > 0 {
			delete(bySource, "")
			d.heading(3, "Stated, citing without naming a source")
			d.para("These elements cite a clause and name no source for it; the validator reports each (schema). Name the source at the citation.")
			d.line("| Element | Clause | Says |")
			d.line("|---|---|---|")
			for _, e := range in {
				for _, c := range items(e.node, "cites") {
					if str(c, "source") == "" {
						d.line("| %s | %s | %s |", cell(e.label), cell(str(c, "clause")), cell(oneParagraph(str(c, "says"))))
					}
				}
			}
			d.blank()
			marksOnce(in)
		}
		var unknown []string
		for key := range bySource {
			if get(d.sources, key) == nil {
				unknown = append(unknown, key)
			}
		}
		sort.Strings(unknown)
		for _, key := range unknown {
			d.heading(3, fmt.Sprintf("%s, a source the root file does not declare", key))
			d.line("| Element | Clause | Says |")
			d.line("|---|---|---|")
			for _, e := range bySource[key] {
				for _, c := range items(e.node, "cites") {
					if str(c, "source") == key {
						d.line("| %s | %s | %s |", cell(e.label), cell(str(c, "clause")), cell(oneParagraph(str(c, "says"))))
					}
				}
			}
			d.blank()
			marksOnce(bySource[key])
		}
		if len(uncited) > 0 {
			d.heading(3, "Stated, citing nothing")
			d.para("These elements say a source states them and cite none; the validator reports each (origin_citation). Add the citation, or change the origin.")
			d.list(uncited)
		}
	}

	d.section("Clarified")
	if len(inferred) == 0 {
		d.para("No element was inferred.")
	} else {
		d.line("| Element | Concluded from |")
		d.line("|---|---|")
		for _, e := range inferred {
			from := oneParagraph(str(e.node, "why"))
			if from == "" {
				from = "not said; the validator reports it (origin_reason)"
			}
			d.line("| %s | %s |", cell(e.label), cell(from))
		}
		d.blank()
		d.rowMarks(inferred)
	}

	d.section("Added or corrected")
	if len(decided) == 0 {
		d.para("No element was decided.")
	} else {
		d.line("| Element | Decided in | Which | Why |")
		d.line("|---|---|---|---|")
		for _, e := range decided {
			id := str(e.node, "decidedIn")
			which := "added"
			if d.corrects(e.node) {
				which = "corrected"
			}
			dec := get(d.decisions, id)
			why := oneParagraph(str(dec, "why"))
			if why == "" {
				why = oneParagraph(str(dec, "decision"))
			}
			in := id
			if id == "" {
				in = "not said; the validator reports it (origin_decision)"
			} else if dec == nil {
				in = id + ", which the specification does not hold"
			} else if title := oneParagraph(str(dec, "title")); title != "" {
				in = id + ", " + title
			}
			d.line("| %s | %s | %s | %s |", cell(e.label), cell(in), which, cell(why))
		}
		d.blank()
		d.rowMarks(decided)
	}

	d.section("Dropped")
	if len(outlined) == 0 {
		d.para("No source lists its clauses, so nothing can be said to be dropped: list each source's outline under clauses, and this section names the clauses no element cites.")
	} else {
		for _, o := range outlined {
			d.cite(o.key)
			d.heading(3, fmt.Sprintf("%s (%s)", d.sourceTitle(o.key), o.key))
			d.para(fmt.Sprintf("%s of %d produced nothing.", countText(len(o.dropped), "clause", "clauses"), o.total))
			if len(o.dropped) > 0 {
				d.line("| Clause | Title |")
				d.line("|---|---|")
				for _, c := range o.dropped {
					d.line("| %s | %s |", cell(c.clause), cell(c.title))
				}
				d.blank()
			}
			if len(o.asked) > 0 {
				var lines []string
				for _, c := range o.asked {
					lines = append(lines, fmt.Sprintf("%s (%s)", c.clause, strings.Join(c.questions, ", ")))
				}
				d.para("Cited by a question only, so asked about rather than dropped: " + strings.Join(lines, "; ") + ".")
			}
		}
	}

	if len(none) > 0 {
		d.section("Without an origin")
		if str(info, "tracksOrigin") == "true" {
			d.para("These elements carry no origin although the specification tracks it; the validator reports each (origin_missing). Say for each whether a source states it, it was inferred, or a decision settled it.")
		} else {
			d.para("These elements carry no origin: they were written spec-first, or the specification does not track origin yet (info.tracksOrigin).")
		}
		d.list(none)
	}
	d.sourcesIndex("Sources")
	return d.String()
}

// element is one named element of a section, with its pointer as its label.
type element struct {
	label string
	node  *yaml.Node
}

// namedElements lists the named elements of every section in document
// order, as originCounts counts them: the entries of a section of named
// elements, each operation of paths, and a section that is one element.
func namedElements(root *yaml.Node) []element {
	var out []element
	ptrs := pointers(root)
	for _, p := range source.Pairs(root) {
		name := p.Key.Value
		if _, isSection := spec.Sections[name]; !isSection || name == "decisions" || name == "questions" {
			continue // the decisions and the questions are the refinement's own record, not elements of the system
		}
		switch name {
		case "release", "rollback", "signoff", "handover", "backup", "restore":
			out = append(out, element{"#" + ptrs[source.Deref(p.Value)], p.Value})
		case "paths":
			for _, o := range operations(root) {
				out = append(out, element{"#" + ptrs[source.Deref(o.node)], o.node})
			}
		default:
			for _, e := range source.Pairs(p.Value) {
				if source.Deref(e.Value).Kind == yaml.MappingNode {
					out = append(out, element{"#" + ptrs[source.Deref(e.Value)], e.Value})
				}
			}
		}
	}
	return out
}

// corrects tells whether a decided element corrects a source rather than
// adding what none said: it cites a source other than an interview.
func (d *doc) corrects(n *yaml.Node) bool {
	for _, c := range items(n, "cites") {
		if str(get(d.sources, str(c, "source")), "kind") != "interview" {
			return true
		}
	}
	return false
}

func (d *doc) sourceTitle(key string) string {
	if t := strings.TrimSpace(str(get(d.sources, key), "title")); t != "" {
		return t
	}
	return key
}

// cite records a source as cited, for the Sources index.
func (d *doc) cite(key string) {
	if d.cited == nil {
		d.cited = map[string]bool{}
	}
	d.cited[key] = true
}

// rowMarks writes the open questions and the problems at each row of a
// table, labelled with the row, so that every problem is marked at its
// entry.
func (d *doc) rowMarks(rows []element) {
	for _, e := range rows {
		d.open(e.label, e.node)
		d.marksAt(e.label, e.node)
	}
}

// list writes elements as a bulleted list, each with its marks.
func (d *doc) list(rows []element) {
	for _, e := range rows {
		d.line("- %s", e.label)
	}
	d.blank()
	d.rowMarks(rows)
}

// outline is one source that lists its clauses, with the clauses no
// element cites and the clauses only questions cite.
type outline struct {
	key     string
	total   int
	dropped []outlineClause
	asked   []outlineClause
}

type outlineClause struct {
	clause, title string
	questions     []string
}

// outlines finds, for every source that lists its clauses, which clauses
// no element cites: a citation falls under the longest listed clause it
// equals or starts with, as the coverage does. A citation from a question
// counts for asked, not for produced; one from a decision counts for
// neither, since the decisions are not compared.
func (d *doc) outlines(root *yaml.Node) []outline {
	var out []outline
	byElements := citers(root, nil)
	questionCites := map[string]map[string][]string{} // source -> clause -> question IDs
	for _, q := range pairs(root, "questions") {
		for _, c := range items(q.Value, "cites") {
			key := str(c, "source")
			if questionCites[key] == nil {
				questionCites[key] = map[string][]string{}
			}
			questionCites[key][str(c, "clause")] = append(questionCites[key][str(c, "clause")], q.Key.Value)
		}
	}
	for _, p := range pairs(root, "sources") {
		key := p.Key.Value
		clauses := items(p.Value, "clauses")
		if len(clauses) == 0 {
			continue
		}
		o := outline{key: key, total: len(clauses)}
		produced := make([]bool, len(clauses))
		asked := make([][]string, len(clauses))
		under := func(cited string) int {
			best := -1
			for i, cl := range clauses {
				if spec.ClauseUnder(cited, str(cl, "clause")) && (best < 0 || len(str(cl, "clause")) > len(str(clauses[best], "clause"))) {
					best = i
				}
			}
			return best
		}
		for _, c := range byElements[key] {
			if strings.HasPrefix(c.element, "#/questions/") || strings.HasPrefix(c.element, "#/decisions/") {
				continue // neither is compared, so a citation from one makes no clause produced
			}
			if i := under(c.clause); i >= 0 {
				produced[i] = true
			}
		}
		for clause, ids := range questionCites[key] {
			if i := under(clause); i >= 0 {
				asked[i] = append(asked[i], ids...)
			}
		}
		for i, cl := range clauses {
			if produced[i] {
				continue
			}
			c := outlineClause{clause: str(cl, "clause"), title: str(cl, "title")}
			if len(asked[i]) > 0 {
				c.questions = uniqueSorted(asked[i])
				o.asked = append(o.asked, c)
			} else {
				o.dropped = append(o.dropped, c)
			}
		}
		out = append(out, o)
	}
	return out
}

func isAre(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}
