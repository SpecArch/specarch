package extract

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// The handler records as the implementation fragment writes them, and the
// comparison with the records an implementation file already holds.

// dependencyOf is the dependency name a literal URL gives: its host in
// camelCase, or "" when the URL names no host.
func dependencyOf(u string) string {
	parsed, err := url.Parse(u)
	if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return camel(strings.ToLower(hostWord.ReplaceAllString(parsed.Hostname(), "_")))
}

// handlerMappings is the mappings node of the implementation fragment: one
// mapping per operation, its handler as the target and, when the handler
// was followed, the record.
func (g *goReader) handlerMappings() *yaml.Node {
	maps := mapping()
	for _, h := range g.handlers {
		m := mapping("target", h.target)
		if len(h.functions) > 0 {
			set(m, "handler", h.node())
		} else {
			set(m, "description", h.notFollowed())
		}
		set(m, "origin", "stated")
		clause := h.regClause
		says := fmt.Sprintf("Registers %s with the handler %s.", h.label, h.target)
		if len(h.functions) > 0 {
			fn := h.functions[0]
			clause = fmt.Sprintf("%s:%d", fn.file.path, fn.first)
			says = fmt.Sprintf("The handler of %s, followed into %s.", h.label, plural(len(h.functions), "function"))
		}
		set(m, "cites", []*yaml.Node{g.at(clause, says)})
		set(maps, h.at, m)
	}
	return maps
}

// notFollowed says why a handler has no record.
func (h *goHandler) notFollowed() string {
	if h.ep != nil && h.ep.paging != nil {
		return "The handler is dxlib's own paging list, whose logic is dxlib's; its whitelists are cited at the operation."
	}
	return "The handler is not a function literal, a function of the same package or a function of another package of the module named through its import, so it was not followed."
}

// node is the handler record.
func (h *goHandler) node() *yaml.Node {
	var fns []*yaml.Node
	for _, fn := range h.functions {
		fns = append(fns, fn.node())
	}
	return mapping("commit", h.g.r.Commit, "depth", handlerDepth, "functions", fns)
}

func (fn *goFunction) node() *yaml.Node {
	steps := stepNodes(fn.steps)
	n := mapping("name", fn.name, "file", fn.file.path, "firstLine", fn.first, "lastLine", fn.last, "steps", steps)
	src := &yaml.Node{Kind: yaml.ScalarNode, Value: fn.source, Style: yaml.LiteralStyle}
	set(n, "source", src)
	return n
}

// stepNodes writes steps in order; an empty list is written as [].
func stepNodes(steps []*goStep) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	if len(steps) == 0 {
		seq.Style = yaml.FlowStyle
	}
	for _, s := range steps {
		seq.Content = append(seq.Content, s.node())
	}
	return seq
}

func (s *goStep) node() *yaml.Node {
	n := mapping("id", s.id, "kind", s.kind, "text", s.text, "file", s.file.path, "firstLine", s.first, "lastLine", s.last)
	if s.parameter != "" {
		set(n, "parameter", s.parameter)
	}
	switch s.kind {
	case "branch":
		set(n, "then", stepNodes(s.then))
		if s.hasOtherwise {
			set(n, "otherwise", stepNodes(s.otherwise))
		}
	case "choice":
		var cases []*yaml.Node
		for _, c := range s.cases {
			cases = append(cases, mapping("when", c.when, "steps", stepNodes(c.steps)))
		}
		set(n, "cases", cases)
	case "loop":
		set(n, "body", stepNodes(s.body))
	case "database":
		if s.table != "" {
			set(n, "table", s.table)
		}
		if s.access != "" {
			set(n, "access", s.access)
		}
		if s.transaction {
			set(n, "transaction", true)
		}
	case "call":
		if s.function != "" {
			set(n, "function", s.function)
		}
		if s.stopped != "" {
			set(n, "stopped", s.stopped)
		}
	case "external":
		if s.dependency != "" {
			set(n, "dependency", s.dependency)
		}
	case "response":
		if s.status != 0 {
			set(n, "status", s.status)
		}
		if s.problem != "" {
			set(n, "problem", s.problem)
		}
	}
	return n
}

// walkSteps calls fn on every step, in document order, with the steps that
// enclose it, outermost first.
func walkSteps(steps []*goStep, parents []*goStep, fn func(s *goStep, parents []*goStep)) {
	for _, s := range steps {
		fn(s, parents)
		inner := append(append([]*goStep{}, parents...), s)
		walkSteps(s.then, inner, fn)
		walkSteps(s.otherwise, inner, fn)
		for _, c := range s.cases {
			walkSteps(c.steps, inner, fn)
		}
		walkSteps(s.body, inner, fn)
	}
}

// Comparing with the records an implementation file holds.

// previousHandler is a handler record as the implementation file given
// with --implementation holds it.
type previousHandler struct {
	commit    string
	functions []previousFunction
}

type previousFunction struct {
	name        string
	file        string
	first, last int
	lines       []string
}

// loadHandlers reads the handler records of the implementation file, by
// the operation's pointer.
func (g *goReader) loadHandlers(doc *yaml.Node) {
	g.previous = map[string]*previousHandler{}
	for _, p := range source.Pairs(child(doc, "mappings")) {
		rec := child(p.Value, "handler")
		if rec == nil {
			continue
		}
		prev := &previousHandler{commit: scalar(child(rec, "commit"))}
		for _, fn := range source.Items(child(rec, "functions")) {
			pf := previousFunction{name: scalar(child(fn, "name")), file: scalar(child(fn, "file"))}
			fmt.Sscan(scalar(child(fn, "firstLine")), &pf.first)
			fmt.Sscan(scalar(child(fn, "lastLine")), &pf.last)
			pf.lines = stepLinesOf(child(fn, "steps"), 0)
			prev.functions = append(prev.functions, pf)
		}
		g.previous[p.Key.Value] = prev
	}
}

// stepLinesOf flattens recorded steps into the lines the comparison
// reads: the kind and the text, indented by nesting.
func stepLinesOf(steps *yaml.Node, depth int) []string {
	var out []string
	for _, s := range source.Items(steps) {
		out = append(out, strings.Repeat("  ", depth)+scalar(child(s, "kind"))+": "+scalar(child(s, "text")))
		out = append(out, stepLinesOf(child(s, "then"), depth+1)...)
		if o := child(s, "otherwise"); o != nil {
			out = append(out, strings.Repeat("  ", depth)+"otherwise:")
			out = append(out, stepLinesOf(o, depth+1)...)
		}
		for _, c := range source.Items(child(s, "cases")) {
			out = append(out, strings.Repeat("  ", depth)+"when "+scalar(child(c, "when"))+":")
			out = append(out, stepLinesOf(child(c, "steps"), depth+1)...)
		}
		out = append(out, stepLinesOf(child(s, "body"), depth+1)...)
	}
	return out
}

// stepLines flattens a function's steps as stepLinesOf does.
func stepLines(steps []*goStep, depth int) []string {
	var out []string
	for _, s := range steps {
		out = append(out, strings.Repeat("  ", depth)+s.kind+": "+s.text)
		out = append(out, stepLines(s.then, depth+1)...)
		if s.hasOtherwise {
			out = append(out, strings.Repeat("  ", depth)+"otherwise:")
			out = append(out, stepLines(s.otherwise, depth+1)...)
		}
		for _, c := range s.cases {
			out = append(out, strings.Repeat("  ", depth)+"when "+c.when+":")
			out = append(out, stepLines(c.steps, depth+1)...)
		}
		out = append(out, stepLines(s.body, depth+1)...)
	}
	return out
}

// compareHandlers prints how each handler's record differs from the one
// the implementation file holds, step by step.
func (g *goReader) compareHandlers() {
	if g.previous == nil {
		return
	}
	unchanged, changed, fresh := 0, 0, 0
	var lines []string
	for _, h := range g.handlers {
		if len(h.functions) == 0 {
			continue
		}
		prev := g.previous[h.at]
		if prev == nil {
			fresh++
			continue
		}
		var diff []string
		seen := map[string]bool{}
		for _, fn := range h.functions {
			seen[fn.name] = true
			var pf *previousFunction
			for i := range prev.functions {
				if prev.functions[i].name == fn.name {
					pf = &prev.functions[i]
				}
			}
			now := stepLines(fn.steps, 0)
			if pf == nil {
				diff = append(diff, fmt.Sprintf("  function %s (%s:%d-%d) is followed now and was not recorded", fn.name, fn.file.path, fn.first, fn.last))
				for _, l := range now {
					diff = append(diff, "    + "+l)
				}
				continue
			}
			added, removed := diffLines(pf.lines, now)
			moved := pf.file != fn.file.path || pf.first != fn.first || pf.last != fn.last
			if len(added)+len(removed) == 0 && !moved {
				continue
			}
			where := fmt.Sprintf("%s:%d-%d", fn.file.path, fn.first, fn.last)
			if moved {
				where += fmt.Sprintf(" (was %s:%d-%d)", pf.file, pf.first, pf.last)
			}
			diff = append(diff, fmt.Sprintf("  function %s, %s: %s added, %d removed, %d unchanged", fn.name, where, plural(len(added), "step line"), len(removed), len(now)-len(added)))
			for _, l := range mergedDiff(pf.lines, now) {
				if !strings.HasPrefix(l, "  ") {
					diff = append(diff, "    "+l)
				}
			}
		}
		for _, pf := range prev.functions {
			if !seen[pf.name] {
				diff = append(diff, fmt.Sprintf("  function %s (%s:%d-%d) was recorded and is not followed now", pf.name, pf.file, pf.first, pf.last))
			}
		}
		if len(diff) == 0 {
			unchanged++
			continue
		}
		changed++
		lines = append(lines, fmt.Sprintf("changed: the handler of %s (%s), recorded at commit %s and read at commit %s:", h.label, h.target, short(prev.commit), short(g.r.Commit)))
		lines = append(lines, diff...)
	}
	gone := 0
	have := map[string]bool{}
	for _, h := range g.handlers {
		have[h.at] = true
	}
	var goneAt []string
	for at := range g.previous {
		if !have[at] {
			gone++
			goneAt = append(goneAt, at)
		}
	}
	sort.Strings(goneAt)
	g.res.say("handlers: compared with the records of %s: %d unchanged, %d changed, %d not recorded there, %d recorded there and not read now", g.rt.checkFile, unchanged, changed, fresh, gone)
	for _, l := range lines {
		g.res.say("%s", l)
	}
	for _, at := range goneAt {
		g.res.say("gone: %s has a handler record in %s, and the source registers no such operation now", at, g.rt.checkFile)
	}
}

// short is the first seven characters of a commit.
func short(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

// diffLines gives the lines only the old list has and only the new has, by
// a longest common subsequence.
func diffLines(old, now []string) (added, removed []string) {
	for _, l := range mergedDiff(old, now) {
		switch {
		case strings.HasPrefix(l, "+ "):
			added = append(added, l[2:])
		case strings.HasPrefix(l, "- "):
			removed = append(removed, l[2:])
		}
	}
	return added, removed
}

// mergedDiff is the two lists merged as a diff: lines of both unmarked,
// lines only the old has marked -, lines only the new has marked +.
func mergedDiff(old, now []string) []string {
	n, m := len(old), len(now)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if old[i] == now[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var out []string
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case old[i] == now[j]:
			out = append(out, "  "+old[i])
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, "- "+old[i])
			i++
		default:
			out = append(out, "+ "+now[j])
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, "- "+old[i])
	}
	for ; j < m; j++ {
		out = append(out, "+ "+now[j])
	}
	return out
}
