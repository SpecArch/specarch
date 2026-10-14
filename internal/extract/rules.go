package extract

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// dumpRule is one row of a policy that checks a role against a path
// pattern and a method, as the permission table's rules hold it (ADR-100):
// the role, the pattern, the methods (a list, or "*" for every one) and
// the name of the path matcher that reads the pattern.
type dumpRule struct {
	Role    string          `json:"role"`
	Pattern string          `json:"pattern"`
	Methods json.RawMessage `json:"methods"`
	Matcher string          `json:"matcher"`
}

// The path matchers a rule may name. Both read `*` as the last segment as
// the rest of the path, slashes included; colon reads a segment `:name` as
// any one segment, and brace a segment `{name}`.
var ruleMatchers = map[string]bool{"colon": true, "brace": true}

// The kinds of a pattern's segment.
const (
	literalSegment = iota
	parameterSegment
	restSegment
)

type patternSegment struct {
	kind int
	text string // a literal's text, or a parameter's name
}

// pathPattern is a pattern read into its segments, whichever matcher wrote
// it, so that two matchers' patterns compare.
type pathPattern []patternSegment

// readPattern reads a rule's pattern by its matcher, or says why the
// meta-model cannot hold it.
func readPattern(p, matcher string) (pathPattern, string) {
	segs := strings.Split(p[1:], "/")
	var out pathPattern
	for i, seg := range segs {
		switch {
		case seg == "*" && i == len(segs)-1:
			out = append(out, patternSegment{kind: restSegment})
		case strings.Contains(seg, "*"):
			return nil, fmt.Sprintf("the matcher %s reads * only as the whole last segment, the rest of the path, and %s has it elsewhere", matcher, p)
		case matcher == "colon" && strings.HasPrefix(seg, ":") && len(seg) > 1:
			out = append(out, patternSegment{kind: parameterSegment, text: seg[1:]})
		case matcher == "brace" && routeSegment.MatchString(seg):
			out = append(out, patternSegment{kind: parameterSegment, text: routeSegment.FindStringSubmatch(seg)[1]})
		default:
			out = append(out, patternSegment{kind: literalSegment, text: seg})
		}
	}
	return out, ""
}

// routePattern reads a route table's path, whose parameters are written
// {name}, as a pattern.
func routePattern(p string) pathPattern {
	pat, _ := readPattern(p, "brace")
	return pat
}

// key is the pattern with the parameters' names left out, the same for
// every matcher that writes it.
func (p pathPattern) key() string {
	var b strings.Builder
	for _, s := range p {
		b.WriteString("/")
		switch s.kind {
		case literalSegment:
			b.WriteString(s.text)
		case parameterSegment:
			b.WriteString("{}")
		default:
			b.WriteString("*")
		}
	}
	return b.String()
}

// covers reports whether every path b matches, a matches too, by the
// matchers' one rule: a literal matches itself, a parameter any one
// segment, and the rest of the path any segments after the slash before
// it, none included.
func (a pathPattern) covers(b pathPattern) bool {
	for i, s := range a {
		if s.kind == restSegment {
			return len(b) > i
		}
		if i >= len(b) || b[i].kind == restSegment {
			return false
		}
		if s.kind == literalSegment && (b[i].kind != literalSegment || b[i].text != s.text) {
			return false
		}
	}
	return len(a) == len(b)
}

// rulePermission is one permission the rules give: one distinct pattern
// and method pair.
type rulePermission struct {
	pattern pathPattern
	method  string // "*" for every method
	name    string
	rows    []ruleRow
	implies []string
	reached bool // a route of the route table takes it, or is reached by it
}

type ruleRow struct {
	role    string // the role's name as the table writes it
	pattern string
	methods string // as the table writes them: "every method", or GET and POST
}

// covers reports whether a holder of p may call everything q guards.
func (p *rulePermission) covers(q *rulePermission) bool {
	return p.pattern.covers(q.pattern) && (p.method == "*" || p.method == q.method)
}

// reaches reports whether p guards the route.
func (p *rulePermission) reaches(method string, path pathPattern) bool {
	return p.pattern.covers(path) && (p.method == "*" || p.method == method)
}

// readMethods reads a rule's methods: "*", or a list of HTTP methods in
// capitals.
func readMethods(raw json.RawMessage) ([]string, bool) {
	var all string
	if json.Unmarshal(raw, &all) == nil {
		return []string{"*"}, all == "*"
	}
	var list []string
	if json.Unmarshal(raw, &list) != nil || len(list) == 0 {
		return nil, false
	}
	seen := map[string]bool{}
	for _, m := range list {
		if !routeMethod.MatchString(m) || seen[m] {
			return nil, false
		}
		seen[m] = true
	}
	return list, true
}

// permissionWords is the words of the name the fixed rule gives a pattern:
// each literal segment's words in lower case, and, when long, each
// parameter's name's words and any for the rest of the path. It says why
// when a word cannot start a part of a permission name.
func permissionWords(p pathPattern, long bool) ([]string, string) {
	var words []string
	for _, s := range p {
		text := s.text
		switch {
		case s.kind == restSegment && long:
			text = "any"
		case s.kind != literalSegment && !long:
			continue
		}
		for _, w := range notWord.Split(text, -1) {
			if w == "" {
				continue
			}
			w = strings.ToLower(w)
			if w[0] < 'a' || w[0] > 'z' {
				return nil, fmt.Sprintf("the word %s does not start with a letter, which every part of a permission name does", w)
			}
			words = append(words, w)
		}
	}
	return words, ""
}

func methodWord(m string) string {
	if m == "*" {
		return "all"
	}
	return strings.ToLower(m)
}

// readRules reads the table's rules into one permission per distinct
// pattern and method pair, named by the fixed rule, each with the ones it
// implies, and the permissions each role's rows give it. A rule whose role
// cannot be read, or whose pattern or name the meta-model cannot hold, is
// left out with a line.
func (pr *permissionReader) readRules(dumpName string, isRole map[string]bool) (map[string][]string, error) {
	byRole := map[string][]string{}
	if len(pr.t.Rules) == 0 {
		return byRole, nil
	}
	type at struct{ key, method string }
	found := map[at]*rulePermission{}
	var order []*rulePermission
	seen := map[string]bool{}
	roleOf := map[*rulePermission][]string{}
	for i, r := range pr.t.Rules {
		where := fmt.Sprintf("%s, rule %d", dumpName, i+1)
		if r.Role == "" || r.Pattern == "" || r.Matcher == "" || len(r.Methods) == 0 {
			return nil, refuse("%s names no role, pattern, methods or matcher; a rule is all four", where)
		}
		if !ruleMatchers[r.Matcher] {
			return nil, refuse("%s: the matcher %q is not one this reader knows; it reads colon, where a segment :name is one segment, and brace, where a segment {name} is", where, r.Matcher)
		}
		if !strings.HasPrefix(r.Pattern, "/") {
			return nil, refuse("%s: the pattern %q does not start with /", where, r.Pattern)
		}
		methods, ok := readMethods(r.Methods)
		if !ok {
			return nil, refuse("%s: the methods are neither \"*\" nor a list of HTTP methods in capitals, each once", where)
		}
		row := ruleRow{role: r.Role, pattern: r.Pattern, methods: "every method"}
		if methods[0] != "*" {
			row.methods = joinAnd(methods)
		}
		dup := r.Role + " " + r.Matcher + " " + r.Pattern + " " + row.methods
		if seen[dup] {
			return nil, refuse("%s: %s reaching %s with %s is listed twice", where, r.Role, r.Pattern, row.methods)
		}
		seen[dup] = true
		pat, reason := readPattern(r.Pattern, r.Matcher)
		if reason != "" {
			pr.gap(r.Role, "rule of %s on %s: %s; left out", r.Role, r.Pattern, reason)
			continue
		}
		role, readable := pr.roleNames.name(r.Role)
		if !readable {
			pr.gap(r.Role, "rule of %s on %s: %s; left out", r.Role, r.Pattern, pr.roleNames.why(r.Role))
			continue
		}
		for _, m := range methods {
			k := at{pat.key(), m}
			p := found[k]
			if p == nil {
				p = &rulePermission{pattern: pat, method: m}
				found[k] = p
				order = append(order, p)
			}
			p.rows = append(p.rows, row)
			roleOf[p] = append(roleOf[p], role)
		}
		isRole[role] = true
		pr.wireOf[role] = r.Role
	}
	// Name each permission by the fixed rule; names that meet take their
	// parameters' words too, and a name that still meets another is left
	// out.
	named := map[string][]*rulePermission{}
	for _, p := range order {
		words, reason := permissionWords(p.pattern, false)
		if reason != "" {
			pr.gap(p.rows[0].role, "rule of %s on %s: %s; left out", p.rows[0].role, p.rows[0].pattern, reason)
			continue
		}
		p.name = strings.Join(append(words, methodWord(p.method)), ".")
		named[p.name] = append(named[p.name], p)
	}
	for name, ps := range named {
		if len(ps) == 1 {
			continue
		}
		for _, p := range ps {
			words, _ := permissionWords(p.pattern, true)
			p.name = strings.Join(append(words, methodWord(p.method)), ".")
		}
		pr.res.say("rules: %s would all be named %s, so each is named with its parameters' words too", joinAnd(patternsOf(ps)), name)
	}
	byName := map[string]*rulePermission{}
	sort.SliceStable(order, func(i, j int) bool { return order[i].name < order[j].name })
	for _, p := range order {
		if p.name == "" {
			continue
		}
		if other := byName[p.name]; other != nil {
			pr.gap(p.rows[0].role, "rule of %s on %s with %s: its permission would be named %s, as the one of %s is; left out", p.rows[0].role, p.rows[0].pattern, p.rows[0].methods, p.name, other.rows[0].pattern)
			p.name = ""
			continue
		}
		byName[p.name] = p
		pr.rulePerms = append(pr.rulePerms, p)
	}
	// A permission implies each one it covers that no third one between
	// them covers: the implications written are the fewest that give all.
	for _, p := range pr.rulePerms {
		for _, q := range pr.rulePerms {
			if p == q || !p.covers(q) {
				continue
			}
			direct := true
			for _, r := range pr.rulePerms {
				if r != p && r != q && p.covers(r) && r.covers(q) {
					direct = false
					break
				}
			}
			if direct {
				p.implies = append(p.implies, q.name)
			}
		}
		for _, role := range roleOf[p] {
			byRole[role] = append(byRole[role], p.name)
		}
	}
	implications := 0
	for _, p := range pr.rulePerms {
		implications += len(p.implies)
	}
	pr.res.say("rules: %s give %s, one per distinct pattern and method, named by the fixed rule, with %s from the patterns each covers", plural(len(pr.t.Rules), "rule"), plural(len(pr.rulePerms), "permission"), plural(implications, "implication"))
	return byRole, nil
}

func patternsOf(ps []*rulePermission) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.rows[0].pattern+" with "+methodText(p.method))
	}
	sort.Strings(out)
	return out
}

func methodText(m string) string {
	if m == "*" {
		return "every method"
	}
	return m
}

// rulePermission is the permission of that name the rules give, or nil.
func (pr *permissionReader) rulePermission(name string) *rulePermission {
	for _, p := range pr.rulePerms {
		if p.name == name {
			return p
		}
	}
	return nil
}

// ruleSays is what the permission table says of a permission's rows.
func ruleSays(p *rulePermission) string {
	var parts []string
	for _, r := range p.rows {
		parts = append(parts, fmt.Sprintf("%s reaches %s with %s", r.role, r.pattern, r.methods))
	}
	return strings.Join(parts, "; ") + "."
}

// rulePermissionEntry is a permission the rules give, as the tree writes
// it: inferred, since its name comes from the path, with the why of the
// fixed rule, the rows it stands for and the ones it implies.
func (pr *permissionReader) rulePermissionEntry(p *rulePermission) *yaml.Node {
	e := mapping()
	if len(p.implies) > 0 {
		set(e, "implies", p.implies)
	}
	set(e, "origin", "inferred")
	set(e, "why", fmt.Sprintf("The permission table's rules reach %s with %s, and by the fixed rule of ADR-100 a pattern and method pair is the permission named by the pattern's literal words in lower case, then all for every method or the method: %s.", p.rows[0].pattern, methodText(p.method), p.name))
	set(e, "cites", []*yaml.Node{citation(pr.key, pr.t.Path+" role "+p.rows[0].role, ruleSays(p))})
	return e
}

// routePermission is the permission the rules give a route: the one
// matching permission every other matching one implies. Otherwise it is
// the question to ask at the operation, and why.
func (pr *permissionReader) routePermission(method, path string) (name string, cite *yaml.Node, question, why string) {
	pat := routePattern(path)
	var matches []*rulePermission
	for _, p := range pr.rulePerms {
		if p.reaches(method, pat) {
			matches = append(matches, p)
		}
	}
	for _, p := range matches {
		p.reached = true
	}
	if len(matches) == 0 {
		return "", nil, fmt.Sprintf("No rule of the permission table reaches %s %s. Is it closed on purpose, so that nobody may call it, or should a rule reach it?", method, path),
			"A route no rule reaches refuses every caller, and so does a route a rule was forgotten for, so it is asked, never assumed."
	}
	for _, c := range matches {
		every := true
		for _, o := range matches {
			if o != c && !o.covers(c) {
				every = false
				break
			}
		}
		if every {
			return c.name, citation(pr.key, pr.t.Path+" role "+c.rows[0].role, ruleSays(c)), "", ""
		}
	}
	// Name the narrowest matches: the ones that imply no other match.
	var names []string
	for _, p := range matches {
		narrowest := true
		for _, o := range matches {
			if o != p && p.covers(o) {
				narrowest = false
				break
			}
		}
		if narrowest {
			names = append(names, p.name)
		}
	}
	neither := "neither implies the other"
	if len(names) > 2 {
		neither = "none of them implies another"
	}
	return "", nil, fmt.Sprintf("%s %s is reached by %s, and %s. Which one is its permission?", method, path, joinAnd(names), neither),
		"An operation needs exactly one permission, and rules that reach it with none covering the others leave it ambiguous."
}

// roleRows is what the permission table's rules say a role, as the table
// writes it, reaches, or "" when no rule names it.
func (pr *permissionReader) roleRows(wire string) string {
	var parts []string
	for _, r := range pr.t.Rules {
		if r.Role != wire {
			continue
		}
		methods, ok := readMethods(r.Methods)
		text := "every method"
		if ok && methods[0] != "*" {
			text = joinAnd(methods)
		}
		parts = append(parts, fmt.Sprintf("%s with %s", r.Pattern, text))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("%s reaches %s.", wire, strings.Join(parts, "; "))
}

// couldQuestion writes a could question: one nothing waits for.
func (pr *permissionReader) couldQuestion(text string, blocks []string, why string, cites ...*yaml.Node) {
	pr.nextID++
	q := mapping(
		"question", text,
		"kind", "decision",
		"priority", "could",
		"blocks", blocks,
		"decidedBy", owner,
		"why", why,
	)
	if len(cites) > 0 {
		set(q, "cites", cites)
	}
	set(pr.questions, fmt.Sprintf("Q-%d", pr.nextID), q)
}

// renameQuestion offers to rename the permissions the rules give, since
// their names come from paths and not from a person.
func (pr *permissionReader) renameQuestion() {
	if len(pr.rulePerms) == 0 {
		return
	}
	var names, blocks []string
	for _, p := range pr.rulePerms {
		names = append(names, p.name)
		blocks = append(blocks, "#/permissions/"+escapeToken(p.name))
	}
	pr.couldQuestion(
		fmt.Sprintf("The permissions the rules give are named from their patterns' words and their methods: %s. Should any be renamed to say what it allows?", strings.Join(names, ", ")),
		blocks,
		"The names come from paths and not from a person, and a name is what a reviewer reads in the matrix. Nothing waits for the answer.",
	)
}

// deadRules asks about each permission the rules give that no route of the
// route table takes or is reached by: a rule left from a route that was
// removed, or one that guards a route the table does not show.
func (pr *permissionReader) deadRules() {
	for _, p := range pr.rulePerms {
		if p.reached {
			continue
		}
		pr.res.say("dead rule: %s, from %s, reaches no route of the route table", p.name, strings.TrimSuffix(ruleSays(p), "."))
		pr.couldQuestion(
			fmt.Sprintf("%s reaches no route of the route table (%s). Is it left from a route that was removed, or does it guard a route the table does not show?", p.name, strings.TrimSuffix(ruleSays(p), ".")),
			[]string{"#/permissions/" + escapeToken(p.name)},
			"A rule no route matches grants nothing today, and would grant a route added later that matches it without anyone writing it.",
			citation(pr.key, pr.t.Path+" role "+p.rows[0].role, ruleSays(p)),
		)
	}
}
