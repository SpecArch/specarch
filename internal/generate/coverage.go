package generate

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// citer is one element that cites a source, and the clause it names.
type citer struct {
	element string // how the element is named: a pointer, or the mapping of one
	clause  string
}

// citers lists, per source key, every element of the specification and of
// its implementation files that cites it.
func citers(root *yaml.Node, impls []Implementation) map[string][]citer {
	out := map[string][]citer{}
	var walk func(n *yaml.Node, p []string, name func([]string) string)
	walk = func(n *yaml.Node, p []string, name func([]string) string) {
		n = source.Deref(n)
		if n == nil {
			return
		}
		switch n.Kind {
		case yaml.MappingNode:
			for _, c := range items(n, "cites") {
				key := str(c, "source")
				out[key] = append(out[key], citer{element: name(p), clause: str(c, "clause")})
			}
			for _, kv := range source.Pairs(n) {
				if kv.Key.Value == "cites" {
					continue
				}
				walk(kv.Value, append(append([]string{}, p...), kv.Key.Value), name)
			}
		case yaml.SequenceNode:
			for i, item := range n.Content {
				walk(item, append(append([]string{}, p...), fmt.Sprint(i)), name)
			}
		}
	}
	walk(root, nil, func(p []string) string { return "#" + source.Pointer(p...) })
	for _, impl := range impls {
		file := path.Base(impl.Rel)
		walk(impl.Node, nil, func(p []string) string {
			if len(p) >= 2 && p[0] == "mappings" {
				return "mapping of " + p[1]
			}
			return file + " #" + source.Pointer(p...)
		})
	}
	return out
}

// under reports whether a cited clause falls under a listed one: it is the
// same, or starts with it followed by a dot, a colon, a slash or a space.
func under(cited, listed string) bool {
	if cited == listed {
		return true
	}
	if !strings.HasPrefix(cited, listed) {
		return false
	}
	switch cited[len(listed)] {
	case '.', ':', '/', ' ':
		return true
	}
	return false
}

// coverage writes, for every source that lists its clauses, which elements
// each clause produced and which clauses produced nothing, so that the
// reader sees what of a document or of the code the specification has not
// read yet. A source without clauses is left out: there is no outline to
// compare with.
func (d *doc) coverage(root *yaml.Node, impls []Implementation) {
	var keys []string
	for _, p := range pairs(root, "sources") {
		if len(items(p.Value, "clauses")) > 0 {
			keys = append(keys, p.Key.Value)
		}
	}
	if len(keys) == 0 {
		return
	}
	cited := citers(root, impls)
	d.section("Coverage")
	d.para("What each source's sections produced: the elements that cite a clause, or the clauses under it. A clause that produced nothing is either not read yet or holds nothing the specification needs; say which by citing it from an element or a question.")
	for _, key := range keys {
		src := get(get(root, "sources"), key)
		title := strings.TrimSpace(str(src, "title"))
		clauses := items(src, "clauses")
		byClause := make([][]string, len(clauses))
		var outside []string
		for _, c := range cited[key] {
			best := -1
			for i, cl := range clauses {
				if under(c.clause, str(cl, "clause")) && (best < 0 || len(str(cl, "clause")) > len(str(clauses[best], "clause"))) {
					best = i
				}
			}
			label := c.element
			if best < 0 {
				if c.clause == "" {
					outside = append(outside, label+" (no clause)")
				} else {
					outside = append(outside, label+" ("+c.clause+")")
				}
				continue
			}
			byClause[best] = append(byClause[best], label)
		}
		empty := 0
		for _, els := range byClause {
			if len(els) == 0 {
				empty++
			}
		}
		d.heading(3, fmt.Sprintf("%s (%s)", title, key))
		d.para(fmt.Sprintf("%s of %d produced nothing.", countText(empty, "clause", "clauses"), len(clauses)))
		d.line("| Clause | Title | Produced |")
		d.line("|---|---|---|")
		for i, cl := range clauses {
			els := uniqueSorted(byClause[i])
			produced := "nothing"
			if len(els) > 0 {
				produced = strings.Join(els, ", ")
			}
			d.line("| %s | %s | %s |", cell(str(cl, "clause")), cell(str(cl, "title")), cell(produced))
		}
		d.blank()
		if outside = uniqueSorted(outside); len(outside) > 0 {
			d.para("Cited outside the listed clauses: " + strings.Join(outside, ", ") + ". Add the clause to the source's clauses, or correct the citation.")
		}
	}
}

func uniqueSorted(list []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range list {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
