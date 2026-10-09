package generate

import (
	"encoding/base64"
	"fmt"
	"html"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// SiteParts are what the html target shows that other generators make,
// and the problems it lists; specarch gathers them before the page is
// written.
type SiteParts struct {
	// Workflows are each workflow's diagram as specarch-gen-bpmn draws it
	// in SVG, by the workflow's name.
	Workflows map[string]string
	// Previews are each page's screen as specarch-gen-ui writes it, with
	// its style sheet inside and its scripts left out, by the page's name.
	Previews map[string]string
	// NoPreview says, by page name, why a page has no screen preview, and
	// NoPreviews why no page has one, when that is so.
	NoPreview  map[string]string
	NoPreviews string
	// Problems are every error, warning and open question in the order of
	// the problems file.
	Problems []Listed
}

// Listed is one problem as the problems file lists it, and the pointer of
// the entry it marks ("" for one outside the specification).
type Listed struct {
	ID, Severity, Rule, Message string
	// Line is the problem's line in the problems file, and Notes its
	// notes there.
	Line    string
	Notes   []string
	Pointer string
}

// httpMethods are the keys of a path that are operations.
var httpMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

// siteGroups are the parts of the page after its overview, each with the
// sections it shows, in life-cycle order.
var siteGroups = []struct {
	title string
	keys  []string
}{
	{"Requirements", []string{"stakeholders", "needs", "requirements", "glossary", "assumptions", "constraints", "sources"}},
	{"Data", []string{"enums", "entities", "views", "schemas"}},
	{"Roles and permissions", []string{"permissions", "roles", "separationOfDuties", "session"}},
	{"Operations", []string{"paths", "commands", "channels", "dependencies", "jobs", "errors"}},
	{"Pages and menus", []string{"pages", "menus", "flows", "accessibility", "theme"}},
	{"Workflows", []string{"workflows"}},
	{"Algorithms and decisions", []string{"algorithms", "decisions"}},
	{"Tests and traceability", []string{"tests"}},
	{"Deployment, commissioning and operation", []string{"environments", "configuration", "release", "rollback", "migrations", "checks", "signoff", "monitors"}},
	{"Open questions", []string{"questions"}},
}

// oneObject are the sections that hold one object rather than named
// elements; the overview shows the keys of the root file.
var (
	oneObject   = map[string]bool{"session": true, "release": true, "rollback": true, "signoff": true, "accessibility": true, "theme": true}
	overviewKey = map[string]bool{"specarch": true, "info": true, "stages": true}
)

// sectionTitles are the headings of the sections.
var sectionTitles = map[string]string{
	"separationOfDuties": "Separation of duties", "paths": "HTTP operations",
}

// linkTo says, for a key, which sections a value under it names an
// element of, in the order they are tried.
var linkTo = map[string][]string{
	"satisfies": {"requirements"}, "verifies": {"requirements"}, "requirements": {"requirements"}, "refines": {"requirements"},
	"needs": {"needs"}, "stakeholders": {"stakeholders"}, "decidedBy": {"stakeholders"}, "owner": {"stakeholders"},
	"permission": {"permissions"}, "permissions": {"permissions"},
	"role": {"roles"}, "roles": {"roles"}, "approvers": {"roles"},
	"entity": {"entities", "views"}, "subject": {"entities", "operations"}, "target": {"entities", "pages", "operations"},
	"page": {"pages"}, "navigate": {"pages"}, "pages": {"pages"},
	"source": {"operations", "sources"}, "operation": {"operations"}, "trigger": {"operations"}, "operations": {"operations"}, "submit": {"operations"},
	"decidedIn": {"decisions"}, "algorithm": {"algorithms"}, "workflow": {"workflows"}, "job": {"jobs"},
	"environment": {"environments"}, "environments": {"environments"}, "problem": {"errors"},
}

// Site writes the specification as one HTML page that needs no server and
// makes no request: an overview first, then every section with an anchor
// at every element, named by its pointer, the problems linked both ways
// with the entries they mark, a search and light and dark colours.
func Site(root *yaml.Node, relRoot string, impls []Implementation, state *State) string {
	s := newSite(root, relRoot, impls, state)
	s.collecting = true
	s.write()
	s.place()
	s.collecting = false
	s.Reset()
	s.write()
	return s.String()
}

type site struct {
	strings.Builder
	root       *yaml.Node
	header     string
	d          *doc
	state      *State
	parts      *SiteParts
	collecting bool
	shown      map[string]bool
	at         map[string][]Listed   // anchor -> the problems marked there
	asked      map[string][]question // anchor -> the questions blocking what is there
	exists     map[string]map[string]bool
	operations map[string]string // operationId -> the operation's pointer
	traced     *traced
}

func newSite(root *yaml.Node, relRoot string, impls []Implementation, state *State) *site {
	d := newDoc("html", root, relRoot, impls, state)
	header := strings.TrimSpace(d.String())
	d.Reset()
	if state == nil {
		state = &State{}
	}
	parts := state.Site
	if parts == nil {
		parts = &SiteParts{}
	}
	s := &site{root: root, header: header, d: d, state: state, parts: parts, exists: map[string]map[string]bool{}, operations: map[string]string{}}
	for key := range spec.Sections {
		s.exists[key] = map[string]bool{}
		for _, p := range pairs(root, key) {
			s.exists[key][p.Key.Value] = true
		}
	}
	s.exists["sources"] = map[string]bool{}
	for _, p := range pairs(root, "sources") {
		s.exists["sources"][p.Key.Value] = true
	}
	for _, o := range operations(root) {
		s.operations[o.id] = source.Pointer("paths", o.path, o.method)
	}
	return s
}

// place puts each problem at the deepest element the page shows that
// holds its pointer, and each open question at what it blocks.
func (s *site) place() {
	s.at = map[string][]Listed{}
	for _, p := range s.parts.Problems {
		if at, ok := deepestShown(s.shown, p.Pointer); ok {
			s.at[at] = append(s.at[at], p)
		}
	}
	s.asked = map[string][]question{}
	for _, q := range s.d.questions {
		var here []string
		for _, b := range q.blocks {
			if !b.IsPointer() {
				continue
			}
			if at, ok := deepestShown(s.shown, source.Pointer(b.Tokens...)); ok && !slices.Contains(here, at) {
				here = append(here, at)
				s.asked[at] = append(s.asked[at], q)
			}
		}
	}
}

// anchor is a pointer as an id and a fragment: the pointer's URI fragment
// form of RFC 6901, section 6, every byte RFC 3986 does not allow in a
// fragment percent-encoded, so the id is the fragment a link carries.
func anchor(ptr string) string {
	var b strings.Builder
	for i := 0; i < len(ptr); i++ {
		c := ptr[i]
		if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || strings.IndexByte("-._~!$&'()*+,;=:@/?", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func esc(s string) string { return html.EscapeString(s) }

func (s *site) line(format string, args ...any) { fmt.Fprintf(s, format+"\n", args...) }

// id writes the id attribute of the element at ptr, and on the first
// writing notes that the page shows it.
func (s *site) id(ptr string) string {
	if s.collecting {
		if s.shown == nil {
			s.shown = map[string]bool{}
		}
		s.shown[ptr] = true
	}
	return fmt.Sprintf(` id="%s"`, esc(anchor(ptr)))
}

// href is a link to the element at ptr, at the deepest element shown
// that holds it.
func (s *site) href(ptr string) string {
	if !s.collecting {
		if at, ok := deepestShown(s.shown, ptr); ok {
			ptr = at
		}
	}
	return "#" + esc(anchor(ptr))
}

// marks writes the problems and the open questions placed at ptr.
func (s *site) marks(ptr string) {
	if s.collecting {
		return
	}
	for _, p := range s.at[ptr] {
		label := p.Severity
		if p.Severity == "question" {
			label = "open question"
		}
		s.line(`<p class="mark %s"><a href="#%s">%s</a> %s</p>`, esc(p.Severity), esc(anchor("problem-"+p.ID)), esc(label), esc(markText(p)))
	}
	for _, q := range s.asked[ptr] {
		s.line(`<p class="mark question">Held open by <a href="%s">%s</a>: %s Decided by %s.</p>`, s.href(source.Pointer("questions", q.id)), esc(q.label()), esc(oneParagraph(str(q.node, "question"))), esc(str(q.node, "decidedBy")))
	}
}

// markText is what a mark says of its problem: the rule, the message and
// the id.
func markText(p Listed) string {
	return fmt.Sprintf("%s: %s [%s]", p.Rule, p.Message, p.ID)
}

func (s *site) write() {
	info := get(s.root, "info")
	title := str(info, "title")
	if title == "" {
		title = "Specification"
	}
	s.line("<!doctype html>")
	s.line("%s", s.header)
	s.line(`<html lang="en">`)
	s.line("<head>")
	s.line(`<meta charset="utf-8">`)
	s.line(`<meta name="viewport" content="width=device-width, initial-scale=1">`)
	s.line(`<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src data:">`)
	s.line("<title>%s %s</title>", esc(title), esc(str(info, "version")))
	s.line("<style>%s</style>", siteCSS)
	s.line("</head>")
	s.line("<body>")
	s.line(`<header class="top"><a class="home" href="#overview">%s</a> <span class="version">version %s</span>`, esc(title), esc(str(info, "version")))
	s.line(`<input id="search" type="search" placeholder="Search the specification" aria-label="Search the specification" autocomplete="off"> <span id="found" role="status" aria-live="polite"></span>`)
	s.line(`<button id="theme" type="button" aria-label="Switch between light and dark">Light / dark</button></header>`)
	s.nav()
	s.line("<main>")
	s.overview()
	used := map[string]bool{}
	for k := range overviewKey {
		used[k] = true
	}
	for _, g := range siteGroups {
		var keys []string
		for _, k := range g.keys {
			used[k] = true
			if get(s.root, k) != nil {
				keys = append(keys, k)
			}
		}
		if len(keys) == 0 && g.title != "Tests and traceability" {
			continue
		}
		if g.title == "Tests and traceability" && len(keys) == 0 && len(pairs(s.root, "requirements")) == 0 {
			continue
		}
		s.line(`<section class="group" id="%s">`, groupID(g.title))
		s.line("<h2>%s</h2>", esc(g.title))
		if g.title == "Data" && len(pairs(s.root, "entities")) > 0 {
			s.line(`<figure class="diagram">%s<figcaption>The entities and their relations; each box links to its entity.</figcaption></figure>`, s.entityDiagram())
		}
		if g.title == "Roles and permissions" {
			s.permissionMatrix()
		}
		if g.title == "Operations" {
			s.operationTable()
		}
		if g.title == "Pages and menus" && s.parts.NoPreviews != "" {
			s.line(`<p class="note">No screen previews: %s</p>`, esc(s.parts.NoPreviews))
		}
		for _, k := range keys {
			s.section(k)
		}
		if g.title == "Tests and traceability" {
			s.traceability()
		}
		s.line("</section>")
	}
	var rest []string
	for _, p := range source.Pairs(s.root) {
		if !used[p.Key.Value] {
			rest = append(rest, p.Key.Value)
		}
	}
	if len(rest) > 0 {
		s.line(`<section class="group" id="other">`)
		s.line("<h2>Other sections</h2>")
		s.line("<p>Sections the meta-model does not know; the problems list says what is wrong with each.</p>")
		for _, k := range rest {
			s.section(k)
		}
		s.line("</section>")
	}
	s.problems()
	s.line("</main>")
	s.line("<script>%s</script>", siteJS)
	s.line("</body>")
	s.line("</html>")
}

func groupID(title string) string {
	return strings.ReplaceAll(strings.ToLower(strings.NewReplacer(",", "").Replace(title)), " ", "-")
}

func sectionTitle(key string) string {
	if t, ok := sectionTitles[key]; ok {
		return t
	}
	return strings.ToUpper(key[:1]) + key[1:]
}

func (s *site) nav() {
	s.line(`<nav class="side" aria-label="Contents"><ol>`)
	s.line(`<li><a href="#overview">Overview</a></li>`)
	for _, g := range siteGroups {
		var keys []string
		for _, k := range g.keys {
			if get(s.root, k) != nil {
				keys = append(keys, k)
			}
		}
		trace := g.title == "Tests and traceability" && len(pairs(s.root, "requirements")) > 0
		if len(keys) == 0 && !trace {
			continue
		}
		s.line(`<li><a href="#%s">%s</a><ol>`, groupID(g.title), esc(g.title))
		for _, k := range keys {
			s.line(`<li><a href="#%s">%s</a> <span class="count">%s</span></li>`, esc(anchor("/"+k)), esc(sectionTitle(k)), s.count(k))
		}
		if trace {
			s.line(`<li><a href="#traceability">Traceability</a></li>`)
		}
		s.line("</ol></li>")
	}
	s.line(`<li><a href="#problems">Problems</a> <span class="count">%d</span></li>`, len(s.parts.Problems))
	s.line("</ol></nav>")
}

// count is how many elements a section holds, or "" for one object.
func (s *site) count(key string) string {
	n := source.Deref(get(s.root, key))
	if n == nil || oneObject[key] {
		return ""
	}
	if key == "paths" {
		return strconv.Itoa(len(operations(s.root)))
	}
	switch n.Kind {
	case yaml.MappingNode:
		return strconv.Itoa(len(n.Content) / 2)
	case yaml.SequenceNode:
		return strconv.Itoa(len(n.Content))
	}
	return ""
}

// overview is the summary first: what the specification is, how far it
// stands, what it holds, and what is wrong with it.
func (s *site) overview() {
	info := get(s.root, "info")
	s.line(`<section class="group" id="overview">`)
	s.line("<h1>%s</h1>", esc(str(info, "title")))
	if desc := strings.TrimSpace(str(info, "description")); desc != "" {
		s.paragraphs(desc)
	}
	var facts []string
	facts = append(facts, "Version "+esc(str(info, "version")))
	if v := str(s.root, "specarch"); v != "" {
		facts = append(facts, "meta-model "+esc(v))
	}
	if st := strs(s.root, "stages"); len(st) > 0 {
		facts = append(facts, "stages "+esc(strings.Join(st, ", ")))
	}
	if owners := strs(info, "owners"); len(owners) > 0 {
		facts = append(facts, "owners "+esc(strings.Join(owners, ", ")))
	}
	s.line("<p>%s.</p>", strings.Join(facts, "; "))

	if ids := holding(s.d.questions, []string{"*"}); len(ids) > 0 {
		s.line(`<p class="notice draft"><strong>Draft:</strong> %s hold up what this page shows (%s); see <a href="#%s">Open questions</a>.</p>`, esc(countText(len(ids), "must or should question", "must or should questions")), esc(strings.Join(ids, ", ")), esc(anchor("/questions")))
	}
	if s.state.Approval != "" {
		s.line(`<p class="notice"><strong>Approval:</strong> %s</p>`, esc(s.state.Approval))
	}
	errs, warns, qs := 0, 0, 0
	for _, p := range s.parts.Problems {
		switch p.Severity {
		case "error":
			errs++
		case "warning":
			warns++
		default:
			qs++
		}
	}
	if errs > 0 {
		s.line(`<p class="notice error"><strong>Invalid:</strong> the specification has %s; this page shows what could be read of it, and each error is marked at its entry.</p>`, esc(countText(errs, "error", "errors")))
	}
	s.line(`<p><a href="#problems">%s, %s and %s</a>; each is listed under Problems and marked at the entry it concerns.</p>`, esc(countText(errs, "error", "errors")), esc(countText(warns, "warning", "warnings")), esc(countText(qs, "open question", "open questions")))

	s.line(`<table class="counts"><thead><tr><th scope="col">Section</th><th scope="col">Elements</th></tr></thead><tbody>`)
	for _, g := range siteGroups {
		for _, k := range g.keys {
			if get(s.root, k) == nil {
				continue
			}
			c := s.count(k)
			if c == "" {
				c = "one"
			}
			s.line(`<tr><td><a href="#%s">%s</a></td><td>%s</td></tr>`, esc(anchor("/"+k)), esc(sectionTitle(k)), c)
		}
	}
	s.line("</tbody></table>")
	s.line(`<p class="how">Every element carries its id, the pointer beside its name, such as <code>/entities/Name</code>. Link to it with <code>#</code> and the pointer after this page's address, and give the id when you report what you see, so the remark reaches the element it is about.</p>`)
	s.line("</section>")
}

// section writes one section: its elements as cards, its rows as a
// table, or its one object.
func (s *site) section(key string) {
	n := source.Deref(get(s.root, key))
	ptr := "/" + key
	s.line(`<section class="section"%s>`, s.id(ptr))
	s.line(`<h3>%s <a class="ptr" href="%s">%s</a></h3>`, esc(sectionTitle(key)), s.href(ptr), esc(ptr))
	s.marks(ptr)
	switch {
	case n == nil:
	case key == "paths" && n.Kind == yaml.MappingNode:
		s.paths(n)
	case oneObject[key] || n.Kind == yaml.ScalarNode:
		s.body(n, ptr, key)
	case n.Kind == yaml.MappingNode && tableMode(n):
		s.table(n, ptr, true)
	case n.Kind == yaml.MappingNode:
		for _, p := range source.Pairs(n) {
			s.card(key, p.Key.Value, p.Value, source.Pointer(key, p.Key.Value))
		}
	case n.Kind == yaml.SequenceNode:
		for i, item := range n.Content {
			name := str(item, "name")
			if name == "" {
				name = str(item, "id")
			}
			if name == "" {
				name = fmt.Sprintf("%s %d", key, i+1)
			}
			s.card(key, name, item, source.Pointer(key, strconv.Itoa(i)))
		}
	}
	s.line("</section>")
}

// card writes one element: its name and id, its marks, what other
// generators show of it, and its definition.
func (s *site) card(key, name string, n *yaml.Node, ptr string) {
	s.line(`<article class="element" data-search%s>`, s.id(ptr))
	s.line(`<h4>%s <a class="ptr" href="%s">%s</a></h4>`, esc(s.title(key, name, n)), s.href(ptr), esc(ptr))
	s.marks(ptr)
	s.extras(key, name, n)
	s.body(n, ptr, "")
	s.line("</article>")
}

// title is an element's heading: its name, and what it is for in a few
// words when it says.
func (s *site) title(key, name string, n *yaml.Node) string {
	for _, k := range []string{"title", "summary"} {
		if t := str(n, k); t != "" && t != name && !strings.Contains(t, "\n") {
			return name + ": " + t
		}
	}
	return name
}

// extras are what an element gets beside its definition: the workflow's
// diagram, the page's screen, what satisfies and verifies a requirement,
// and the combinations of roles a separation of duties forbids.
func (s *site) extras(key, name string, n *yaml.Node) {
	switch key {
	case "workflows":
		if svg, ok := s.parts.Workflows[name]; ok {
			s.line(`<figure class="diagram"><img alt="The workflow %s as BPMN 2.0" src="data:image/svg+xml;base64,%s"><figcaption>As specarch generate bpmn draws it.</figcaption></figure>`, esc(name), base64.StdEncoding.EncodeToString([]byte(svg)))
		}
	case "pages":
		if screen, ok := s.parts.Previews[name]; ok {
			s.line(`<figure class="preview"><iframe sandbox="" loading="lazy" title="The screen %s" srcdoc="%s"></iframe><figcaption>The screen as specarch generate ui writes it, without its scripts, so it shows no data.</figcaption></figure>`, esc(name), esc(screen))
		} else if why, ok := s.parts.NoPreview[name]; ok {
			s.line(`<p class="note">No screen preview: %s</p>`, esc(why))
		}
	case "requirements":
		trace := s.trace()
		s.line(`<p class="trace"><strong>Satisfied by:</strong> %s. <strong>Verified by:</strong> %s.</p>`, s.links(trace.satisfied[name], "nothing yet"), s.links(trace.verified[name], "nothing yet"))
	case "separationOfDuties":
		cardinality := 2
		if c, err := strconv.Atoi(str(n, "cardinality")); err == nil {
			cardinality = c
		}
		var combos []string
		for _, c := range roleCombinations(s.root, strs(n, "permissions"), cardinality) {
			var roles []string
			for _, r := range c {
				roles = append(roles, s.linkName("role", r))
			}
			combos = append(combos, strings.Join(roles, " with "))
		}
		if len(combos) == 0 {
			s.line(`<p class="trace"><strong>Roles never given to one person:</strong> none; no roles together reach this set.</p>`)
		} else {
			s.line(`<p class="trace"><strong>Roles never given to one person:</strong> %s.</p>`, strings.Join(combos, "; "))
		}
	}
}

// links is a list of links to pointers, or none when there are none.
func (s *site) links(ptrs []string, none string) string {
	if len(ptrs) == 0 {
		return esc(none)
	}
	var out []string
	for _, p := range ptrs {
		out = append(out, fmt.Sprintf(`<a href="%s">%s</a>`, s.href(p), esc(p)))
	}
	return strings.Join(out, ", ")
}

// body writes an element's definition: a scalar as text, a list as a
// list, a mapping as its Origin, Insight and Notes and then its keys.
func (s *site) body(n *yaml.Node, ptr, key string) {
	n = source.Deref(n)
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.ScalarNode:
		if strings.Contains(n.Value, "\n") || len(n.Value) > 120 {
			s.paragraphs(n.Value)
		} else {
			s.line("<p>%s</p>", s.scalar(key, n.Value))
		}
	case yaml.SequenceNode:
		s.list(n, ptr, key)
	case yaml.MappingNode:
		s.mapping(n, ptr)
	}
}

func (s *site) paragraphs(text string) {
	for _, para := range strings.Split(strings.TrimSpace(text), "\n\n") {
		if para = strings.TrimSpace(para); para != "" {
			s.line("<p>%s</p>", esc(oneParagraph(para)))
		}
	}
}

// annotated are the keys a mapping's annotations show instead of its
// list of keys.
var annotated = map[string]bool{"why": true, "cites": true, "origin": true}

func (s *site) mapping(n *yaml.Node, ptr string) {
	if o := s.d.origin(n); o != "" {
		s.line(`<p class="origin">%s</p>`, bold(o))
	}
	if why := strings.TrimSpace(str(n, "why")); why != "" {
		s.line(`<p class="insight"><strong>Insight:</strong> %s</p>`, esc(oneParagraph(why)))
	}
	for _, c := range items(n, "cites") {
		s.line(`<p class="note"><strong>Note:</strong> %s</p>`, s.citation(c))
	}
	var rows []source.Pair
	for _, p := range source.Pairs(n) {
		if annotated[p.Key.Value] || (p.Key.Value == "decidedIn" && str(n, "origin") == "decided") {
			continue
		}
		rows = append(rows, p)
	}
	if len(rows) == 0 {
		return
	}
	s.line("<dl>")
	for _, p := range rows {
		v := source.Deref(p.Value)
		child := ptr + "/" + source.EscapeToken(p.Key.Value)
		s.line("<dt>%s</dt>", esc(p.Key.Value))
		switch {
		case v == nil:
			s.line("<dd></dd>")
		case v.Kind == yaml.ScalarNode && !strings.Contains(v.Value, "\n") && len(v.Value) <= 120:
			s.line("<dd>%s</dd>", s.scalar(p.Key.Value, v.Value))
		case v.Kind == yaml.ScalarNode:
			s.line("<dd>")
			s.paragraphs(v.Value)
			s.line("</dd>")
		case v.Kind == yaml.MappingNode && p.Key.Value == "properties" && fieldsMode(v):
			s.line("<dd%s>", s.id(child))
			s.marks(child)
			s.fields(v, child, strs(n, "required"))
			s.line("</dd>")
		case v.Kind == yaml.MappingNode && tableMode(v):
			s.line("<dd%s>", s.id(child))
			s.marks(child)
			s.table(v, child, false)
			s.line("</dd>")
		case v.Kind == yaml.MappingNode:
			s.line("<dd%s>", s.id(child))
			s.marks(child)
			s.mapping(v, child)
			s.line("</dd>")
		default:
			s.line("<dd>")
			s.list(v, child, p.Key.Value)
			s.line("</dd>")
		}
	}
	s.line("</dl>")
}

// list writes a list of scalars on one line, and any other list item by
// item, each with its id.
func (s *site) list(n *yaml.Node, ptr, key string) {
	scalars := true
	for _, item := range n.Content {
		if source.Deref(item) == nil || source.Deref(item).Kind != yaml.ScalarNode || len(item.Value) > 120 || strings.Contains(item.Value, "\n") {
			scalars = false
		}
	}
	if scalars {
		var out []string
		for _, item := range n.Content {
			out = append(out, s.scalar(key, item.Value))
		}
		s.line("<p>%s</p>", strings.Join(out, ", "))
		return
	}
	s.line(`<ol class="items">`)
	for i, item := range n.Content {
		child := ptr + "/" + strconv.Itoa(i)
		s.line("<li%s>", s.id(child))
		s.marks(child)
		s.body(item, child, key)
		s.line("</li>")
	}
	s.line("</ol>")
}

// tableMode reports whether a mapping of mappings reads best as a table:
// each row's values short enough for a cell, and few columns.
func tableMode(n *yaml.Node) bool {
	n = source.Deref(n)
	if n == nil || n.Kind != yaml.MappingNode || len(n.Content) == 0 {
		return false
	}
	columns := map[string]bool{}
	for _, p := range source.Pairs(n) {
		v := source.Deref(p.Value)
		if v == nil || v.Kind != yaml.MappingNode {
			return false
		}
		for _, c := range source.Pairs(v) {
			if annotated[c.Key.Value] && c.Key.Value != "origin" {
				return false
			}
			columns[c.Key.Value] = true
			cv := source.Deref(c.Value)
			if cv != nil && cv.Kind != yaml.ScalarNode && len(compact(cv)) > 80 {
				return false
			}
		}
	}
	return len(columns) <= 7
}

// fieldsMode reports whether a schema's properties are each a mapping,
// so they read as a table of fields.
func fieldsMode(n *yaml.Node) bool {
	for _, p := range source.Pairs(n) {
		if v := source.Deref(p.Value); v == nil || v.Kind != yaml.MappingNode {
			return false
		}
	}
	return len(n.Content) > 0
}

// fieldKeys are the keys of a field the table of fields shows in columns
// of their own.
var fieldKeys = map[string]bool{"type": true, "format": true, "$ref": true, "precision": true, "scale": true, "items": true, "title": true, "description": true}

// fields writes a schema's properties as a table: the name, the type,
// whether it is required, the title and description, and every other key.
func (s *site) fields(n *yaml.Node, ptr string, required []string) {
	s.line(`<div class="scroll"><table class="fields"><thead><tr><th scope="col">Field</th><th scope="col">Type</th><th scope="col">Required</th><th scope="col">Title and description</th><th scope="col">Also</th></tr></thead><tbody>`)
	for _, p := range source.Pairs(n) {
		row := ptr + "/" + source.EscapeToken(p.Key.Value)
		v := source.Deref(p.Value)
		s.line(`<tr%s><th scope="row">%s <a class="ptr" href="%s">%s</a>`, s.id(row), esc(p.Key.Value), s.href(row), esc(row))
		s.marks(row)
		s.line("</th>")
		typ := esc(typeText(v))
		if ref := str(v, "$ref"); ref != "" {
			typ = s.scalar("$ref", ref)
		}
		req := ""
		if slices.Contains(required, p.Key.Value) {
			req = "yes"
		}
		var text []string
		if t := str(v, "title"); t != "" {
			text = append(text, "<strong>"+esc(t)+"</strong>")
		}
		if d := str(v, "description"); d != "" {
			text = append(text, esc(oneParagraph(d)))
		}
		var also []string
		for _, c := range source.Pairs(v) {
			if !fieldKeys[c.Key.Value] {
				also = append(also, esc(c.Key.Value)+": "+s.cellValue(c.Key.Value, c.Value))
			}
		}
		s.line("<td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>", typ, req, strings.Join(text, " "), strings.Join(also, "; "))
	}
	s.line("</tbody></table></div>")
}

// cellValue is a value inside a cell: a scalar or a list of scalars with
// its links, anything else on one line.
func (s *site) cellValue(key string, n *yaml.Node) string {
	n = source.Deref(n)
	switch {
	case n == nil:
		return ""
	case n.Kind == yaml.ScalarNode:
		return s.scalar(key, n.Value)
	case n.Kind == yaml.SequenceNode && allScalars(n):
		var out []string
		for _, item := range n.Content {
			out = append(out, s.scalar(key, item.Value))
		}
		return strings.Join(out, ", ")
	}
	return "<code>" + esc(compact(n)) + "</code>"
}

// compact is a value on one line, as a cell shows it.
func compact(n *yaml.Node) string {
	n = source.Deref(n)
	if n == nil {
		return ""
	}
	switch n.Kind {
	case yaml.MappingNode:
		var parts []string
		for _, p := range source.Pairs(n) {
			parts = append(parts, p.Key.Value+": "+compact(p.Value))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case yaml.SequenceNode:
		var parts []string
		for _, item := range n.Content {
			parts = append(parts, compact(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return n.Value
}

// table writes a mapping of mappings as a table, a row per entry with its
// id; rows of a section are searched like cards.
func (s *site) table(n *yaml.Node, ptr string, searched bool) {
	var columns []string
	for _, p := range source.Pairs(n) {
		for _, c := range source.Pairs(source.Deref(p.Value)) {
			if !slices.Contains(columns, c.Key.Value) {
				columns = append(columns, c.Key.Value)
			}
		}
	}
	s.line(`<div class="scroll"><table><thead><tr><th scope="col">Name</th>`)
	for _, c := range columns {
		s.line(`<th scope="col">%s</th>`, esc(c))
	}
	s.line("</tr></thead><tbody>")
	for _, p := range source.Pairs(n) {
		row := ptr + "/" + source.EscapeToken(p.Key.Value)
		attr := ""
		if searched {
			attr = " data-search"
		}
		s.line(`<tr%s%s><th scope="row">%s <a class="ptr" href="%s">%s</a>`, attr, s.id(row), esc(p.Key.Value), s.href(row), esc(row))
		s.marks(row)
		s.line("</th>")
		v := source.Deref(p.Value)
		for _, c := range columns {
			s.line("<td>%s</td>", s.cellValue(c, get(v, c)))
		}
		s.line("</tr>")
	}
	s.line("</tbody></table></div>")
}

func allScalars(n *yaml.Node) bool {
	for _, item := range n.Content {
		if d := source.Deref(item); d == nil || d.Kind != yaml.ScalarNode {
			return false
		}
	}
	return true
}

// scalar is a value as text, a link when it names an element: a $ref, a
// question's block, or a name under a key that names elements.
func (s *site) scalar(key, v string) string {
	if strings.HasPrefix(v, "#/") {
		ptr := v[1:]
		if i := strings.Index(ptr, "#"); i >= 0 {
			ptr = ptr[:i]
		}
		return fmt.Sprintf(`<a href="%s">%s</a>`, s.href(ptr), esc(v))
	}
	if key != "" {
		if link := s.linkName(key, v); link != esc(v) {
			return link
		}
	}
	return esc(v)
}

// linkName is a name under a key as a link to the element it names, when
// one of the sections the key names elements of holds it.
func (s *site) linkName(key, name string) string {
	for _, section := range linkTo[key] {
		if section == "operations" {
			if ptr, ok := s.operations[name]; ok {
				return fmt.Sprintf(`<a href="%s">%s</a>`, s.href(ptr), esc(name))
			}
			continue
		}
		if s.exists[section][name] {
			return fmt.Sprintf(`<a href="%s">%s</a>`, s.href(source.Pointer(section, name)), esc(name))
		}
	}
	return esc(name)
}

// bold turns a document's **Label:** into HTML.
func bold(md string) string {
	if strings.HasPrefix(md, "**") {
		if i := strings.Index(md[2:], "**"); i >= 0 {
			return "<strong>" + esc(md[2:2+i]) + "</strong>" + esc(md[4+i:])
		}
	}
	return esc(md)
}

// citation is a Note: the source and its edition, the clause, what it
// says, and a link to where to read it.
func (s *site) citation(c *yaml.Node) string {
	key := str(c, "source")
	src := get(s.d.sources, key)
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
	text := fmt.Sprintf(`From <a href="%s">%s</a>: %s`, s.href(source.Pointer("sources", key)), esc(from), esc(oneParagraph(str(c, "says"))))
	if u := str(src, "url"); u != "" && !strings.HasSuffix(str(src, "kind"), "-set") && (strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://")) {
		text += fmt.Sprintf(` <a rel="noreferrer" href="%s">%s</a>`, esc(u), esc(u))
	}
	return text
}

// paths writes the operations, each a card, under its path.
func (s *site) paths(n *yaml.Node) {
	for _, route := range source.Pairs(n) {
		rptr := source.Pointer("paths", route.Key.Value)
		s.line(`<div class="route"%s>`, s.id(rptr))
		s.line(`<h4 class="route-name">%s</h4>`, esc(route.Key.Value))
		s.marks(rptr)
		rest := &yaml.Node{Kind: yaml.MappingNode}
		v := source.Deref(route.Value)
		for _, p := range source.Pairs(v) {
			if !slices.Contains(httpMethods, p.Key.Value) {
				rest.Content = append(rest.Content, p.Key, p.Value)
			}
		}
		if len(rest.Content) > 0 {
			s.mapping(rest, rptr)
		}
		for _, p := range source.Pairs(v) {
			if !slices.Contains(httpMethods, p.Key.Value) {
				continue
			}
			optr := source.Pointer("paths", route.Key.Value, p.Key.Value)
			name := strings.ToUpper(p.Key.Value) + " " + route.Key.Value
			if id := str(p.Value, "operationId"); id != "" {
				name += " (" + id + ")"
			}
			s.line(`<article class="element" data-search%s>`, s.id(optr))
			s.line(`<h4>%s <a class="ptr" href="%s">%s</a></h4>`, esc(name), s.href(optr), esc(optr))
			s.marks(optr)
			s.body(p.Value, optr, "")
			s.line("</article>")
		}
		s.line("</div>")
	}
}

// operationTable lists every HTTP operation with its route and the
// permission it needs.
func (s *site) operationTable() {
	ops := operations(s.root)
	if len(ops) == 0 {
		return
	}
	s.line(`<div class="scroll"><table class="overview"><caption>Every HTTP operation, the permission it needs, and what it does.</caption>`)
	s.line(`<thead><tr><th scope="col">Method</th><th scope="col">Route</th><th scope="col">Operation</th><th scope="col">Permission</th><th scope="col">Summary</th></tr></thead><tbody>`)
	for _, o := range ops {
		ptr := source.Pointer("paths", o.path, o.method)
		id := o.id
		if id == "" {
			id = strings.ToUpper(o.method) + " " + o.path
		}
		s.line(`<tr><td>%s</td><td><code>%s</code></td><td><a href="%s">%s</a></td><td>%s</td><td>%s</td></tr>`, esc(strings.ToUpper(o.method)), esc(o.path), s.href(ptr), esc(id), s.linkName("permission", str(o.node, "permission")), esc(str(o.node, "summary")))
	}
	s.line("</tbody></table></div>")
}

// permissionMatrix shows which role grants which permission.
func (s *site) permissionMatrix() {
	perms := pairs(s.root, "permissions")
	roles := pairs(s.root, "roles")
	if len(perms) == 0 || len(roles) == 0 {
		return
	}
	s.line(`<div class="scroll"><table class="matrix"><caption>Which role grants which permission. Access is fail-closed: an operation, command or page needs the one permission it names.</caption><thead><tr><th scope="col">Permission</th>`)
	for _, r := range roles {
		s.line(`<th scope="col">%s</th>`, s.linkName("role", r.Key.Value))
	}
	s.line("</tr></thead><tbody>")
	for _, p := range perms {
		s.line(`<tr><th scope="row">%s</th>`, s.linkName("permission", p.Key.Value))
		for _, r := range roles {
			if slices.Contains(strs(r.Value, "permissions"), p.Key.Value) {
				s.line(`<td class="yes">yes</td>`)
			} else {
				s.line("<td></td>")
			}
		}
		s.line("</tr>")
	}
	s.line("</tbody></table></div>")
}

// traced is, per requirement, the elements that satisfy it and the ones
// that verify it.
type traced struct {
	satisfied, verified map[string][]string
}

func (s *site) trace() traced {
	if s.traced != nil {
		return *s.traced
	}
	t := traced{satisfied: map[string][]string{}, verified: map[string][]string{}}
	var walk func(n *yaml.Node, path []string)
	walk = func(n *yaml.Node, path []string) {
		n = source.Deref(n)
		if n == nil {
			return
		}
		switch n.Kind {
		case yaml.MappingNode:
			ptr := source.Pointer(path...)
			for _, r := range strs(n, "satisfies") {
				t.satisfied[r] = append(t.satisfied[r], ptr)
			}
			for _, r := range strs(n, "verifies") {
				t.verified[r] = append(t.verified[r], ptr)
			}
			for _, p := range source.Pairs(n) {
				walk(p.Value, append(append([]string{}, path...), p.Key.Value))
			}
		case yaml.SequenceNode:
			for i, item := range n.Content {
				walk(item, append(append([]string{}, path...), strconv.Itoa(i)))
			}
		}
	}
	walk(s.root, nil)
	s.traced = &t
	return t
}

// traceability is the matrix from each need through its requirements to
// what satisfies and verifies them.
func (s *site) traceability() {
	reqs := pairs(s.root, "requirements")
	if len(reqs) == 0 {
		return
	}
	t := s.trace()
	s.line(`<section class="section" id="traceability">`)
	s.line("<h3>Traceability</h3>")
	s.line("<p>Each requirement, the needs it refines, and the elements that satisfy and verify it. A requirement nothing satisfies or verifies is a gap.</p>")
	s.line(`<div class="scroll"><table><thead><tr><th scope="col">Requirement</th><th scope="col">Needs</th><th scope="col">Satisfied by</th><th scope="col">Verified by</th></tr></thead><tbody>`)
	for _, r := range reqs {
		var needs []string
		for _, n := range strs(r.Value, "needs") {
			needs = append(needs, s.linkName("needs", n))
		}
		gap := ""
		if len(t.satisfied[r.Key.Value]) == 0 || len(t.verified[r.Key.Value]) == 0 {
			gap = ` class="gap"`
		}
		s.line(`<tr data-search%s><th scope="row">%s</th><td>%s</td><td>%s</td><td>%s</td></tr>`, gap, s.linkName("requirements", r.Key.Value), strings.Join(needs, ", "), s.links(t.satisfied[r.Key.Value], "nothing"), s.links(t.verified[r.Key.Value], "nothing"))
	}
	s.line("</tbody></table></div>")
	s.line("</section>")
}

// problems lists every error, warning and open question as the problems
// file does, each linked to the entry it marks.
func (s *site) problems() {
	s.line(`<section class="group" id="problems">`)
	s.line("<h2>Problems</h2>")
	if len(s.parts.Problems) == 0 {
		s.line("<p>The specification has no errors, no warnings and no open questions.</p>")
		s.line("</section>")
		return
	}
	s.line("<p>Every error, warning and open question, in the order of the problems file, each with the file and line it is at and the entry it marks. The same problems are in problems.txt and problems.sarif.</p>")
	s.line(`<ol class="problems">`)
	for _, p := range s.parts.Problems {
		s.line(`<li class="%s" data-search id="%s">`, esc(p.Severity), esc(anchor("problem-"+p.ID)))
		s.line("<p><code>%s</code></p>", esc(p.Line))
		for _, n := range p.Notes {
			s.line(`<p class="problem-note"><code>%s</code></p>`, esc(n))
		}
		at, ok := deepestShown(s.shown, p.Pointer)
		switch {
		case s.collecting:
		case ok:
			s.line(`<p>Marked at <a href="#%s">%s</a>.</p>`, esc(anchor(at)), esc(at))
		case p.Pointer == "" || p.Pointer == "/":
			s.line("<p>Not at an entry of the specification; the file and line above say where it is.</p>")
		default:
			s.line("<p>At <code>%s</code>, which this page does not show.</p>", esc(p.Pointer))
		}
		s.line("</li>")
	}
	s.line("</ol>")
	s.line("</section>")
}

// entityDiagram draws the entities and their relations as SVG: a box per
// entity with its fields, in a grid in the order the specification lists
// them, and a line per relation, drawn once.
func (s *site) entityDiagram() string {
	entities := pairs(s.root, "entities")
	const width, head, rowH, gapX, gapY, maxFields = 230, 26, 17, 150, 70, 14
	cols := 1
	for cols*cols < len(entities) {
		cols++
	}
	type box struct{ x, y, h int }
	boxes := map[string]box{}
	heights := make([]int, (len(entities)+cols-1)/cols)
	for i, e := range entities {
		n := len(pairs(e.Value, "properties"))
		if n > maxFields {
			n = maxFields + 1
		}
		h := head + n*rowH + 8
		if h > heights[i/cols] {
			heights[i/cols] = h
		}
	}
	tops := make([]int, len(heights))
	y := 20
	for r, h := range heights {
		tops[r] = y
		y += h + gapY
	}
	totalW := cols*(width+gapX) - gapX + 40
	totalH := y - gapY + 20
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="er" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img" aria-label="Entity relationship diagram">`, totalW, totalH, totalW, totalH)
	b.WriteString(`<defs><marker id="er-head" viewBox="0 0 10 10" refX="10" refY="5" markerWidth="8" markerHeight="8" orient="auto-start-reverse"><path class="er-head" d="M0,0 L10,5 L0,10 z"/></marker></defs>`)
	for i, e := range entities {
		n := len(pairs(e.Value, "properties"))
		if n > maxFields {
			n = maxFields + 1
		}
		boxes[e.Key.Value] = box{20 + (i%cols)*(width+gapX), tops[i/cols], head + n*rowH + 8}
	}
	drawn := map[string]bool{}
	var labels strings.Builder
	for _, e := range entities {
		for _, r := range pairs(e.Value, "relations") {
			target, via := str(r.Value, "target"), str(r.Value, "via")
			kind := str(r.Value, "kind")
			key := e.Key.Value + "|" + target + "|" + via
			if kind == "one-to-many" {
				key = target + "|" + e.Key.Value + "|" + via
			}
			if drawn[key] {
				continue
			}
			drawn[key] = true
			from, ok1 := boxes[e.Key.Value]
			to, ok2 := boxes[target]
			if !ok1 || !ok2 {
				continue
			}
			x1, y1 := from.x+width/2, from.y+from.h/2
			x2, y2 := to.x+width/2, to.y+to.h/2
			if e.Key.Value == target {
				fmt.Fprintf(&b, `<path class="er-line" d="M%d,%d C%d,%d %d,%d %d,%d" marker-end="url(#er-head)"/>`, from.x+width, from.y+head, from.x+width+50, from.y+head, from.x+width+50, from.y+head+40, from.x+width, from.y+head+40)
				fmt.Fprintf(&labels, `<text class="er-label" x="%d" y="%d">%s</text>`, from.x+width+8, from.y+head+24, esc(r.Key.Value+" ("+kind+")"))
				continue
			}
			sx, sy := clip(x1, y1, x2, y2, width/2, from.h/2)
			ex, ey := clip(x2, y2, x1, y1, width/2, to.h/2)
			fmt.Fprintf(&b, `<line class="er-line" x1="%d" y1="%d" x2="%d" y2="%d" marker-end="url(#er-head)"/>`, sx, sy, ex, ey)
			fmt.Fprintf(&labels, `<text class="er-label" x="%d" y="%d" text-anchor="middle">%s</text>`, (sx+ex)/2, (sy+ey)/2-4, esc(r.Key.Value+" ("+kind+")"))
		}
	}
	for _, e := range entities {
		bx := boxes[e.Key.Value]
		fmt.Fprintf(&b, `<a href="%s"><rect class="er-box" x="%d" y="%d" width="%d" height="%d" rx="4"/>`, s.href(source.Pointer("entities", e.Key.Value)), bx.x, bx.y, width, bx.h)
		fmt.Fprintf(&b, `<rect class="er-title" x="%d" y="%d" width="%d" height="%d" rx="4"/>`, bx.x, bx.y, width, head)
		fmt.Fprintf(&b, `<text class="er-name" x="%d" y="%d">%s</text></a>`, bx.x+10, bx.y+18, esc(e.Key.Value))
		required := strs(e.Value, "required")
		keys := strs(e.Value, "primaryKey")
		for i, f := range pairs(e.Value, "properties") {
			fy := bx.y + head + 14 + i*rowH
			if i == maxFields {
				fmt.Fprintf(&b, `<text class="er-field" x="%d" y="%d">and %d more</text>`, bx.x+10, fy, len(pairs(e.Value, "properties"))-maxFields)
				break
			}
			mark := ""
			if slices.Contains(keys, f.Key.Value) {
				mark = " (key)"
			} else if !slices.Contains(required, f.Key.Value) {
				mark = "?"
			}
			fmt.Fprintf(&b, `<text class="er-field" x="%d" y="%d">%s%s</text>`, bx.x+10, fy, esc(f.Key.Value), esc(mark))
			fmt.Fprintf(&b, `<text class="er-type" x="%d" y="%d" text-anchor="end">%s</text>`, bx.x+width-10, fy, esc(typeLabel(f.Value)))
		}
	}
	b.WriteString(labels.String())
	b.WriteString("</svg>")
	return b.String()
}

// clip is where the line from (x1, y1) to (x2, y2) leaves the box of half
// width w and half height h centred on (x1, y1).
func clip(x1, y1, x2, y2, w, h int) (int, int) {
	dx, dy := float64(x2-x1), float64(y2-y1)
	if dx == 0 && dy == 0 {
		return x1, y1
	}
	t := 1.0
	if dx != 0 {
		t = min(t, float64(w)/abs(dx))
	}
	if dy != 0 {
		t = min(t, float64(h)/abs(dy))
	}
	return x1 + int(dx*t), y1 + int(dy*t)
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

const siteCSS = `
:root{color-scheme:light;--bg:#ffffff;--fg:#1f2328;--muted:#59636e;--line:#d1d9e0;--soft:#f6f8fa;--accent:#0969da;--error:#cf222e;--warning:#9a6700;--question:#8250df;--target:#fff8c5}
@media (prefers-color-scheme:dark){:root:not([data-theme="light"]){color-scheme:dark;--bg:#0d1117;--fg:#e6edf3;--muted:#9198a1;--line:#3d444d;--soft:#151b23;--accent:#4493f8;--error:#f85149;--warning:#d29922;--question:#ab7df8;--target:#3a2f0b}}
:root[data-theme="dark"]{color-scheme:dark;--bg:#0d1117;--fg:#e6edf3;--muted:#9198a1;--line:#3d444d;--soft:#151b23;--accent:#4493f8;--error:#f85149;--warning:#d29922;--question:#ab7df8;--target:#3a2f0b}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);font:15px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif}
a{color:var(--accent)}
code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:13px;overflow-wrap:anywhere}
header.top{position:sticky;top:0;z-index:2;display:flex;flex-wrap:wrap;gap:8px 12px;align-items:center;padding:8px 16px;background:var(--soft);border-bottom:1px solid var(--line)}
header.top .home{font-weight:600;text-decoration:none;color:var(--fg)}
header.top .version{color:var(--muted)}
#search{flex:1 1 220px;min-width:0;padding:6px 10px;border:1px solid var(--line);border-radius:6px;background:var(--bg);color:var(--fg);font:inherit}
#found{color:var(--muted);font-size:13px}
#theme{padding:6px 10px;border:1px solid var(--line);border-radius:6px;background:var(--bg);color:var(--fg);font:inherit;cursor:pointer}
nav.side{padding:8px 16px;border-bottom:1px solid var(--line)}
nav.side ol{list-style:none;margin:0;padding:0}
nav.side ol ol{padding-left:14px;font-size:14px}
nav.side .count{color:var(--muted);font-size:12px}
main{padding:0 16px 48px;max-width:1100px}
@media (min-width:900px){body{display:grid;grid-template-columns:260px 1fr;grid-template-rows:auto 1fr}header.top{grid-column:1/3}nav.side{position:sticky;top:50px;align-self:start;max-height:calc(100vh - 50px);overflow:auto;border-bottom:0;border-right:1px solid var(--line)}main{padding:0 32px 64px}}
h1{font-size:28px;margin:20px 0 8px}
h2{font-size:22px;margin:32px 0 8px;padding-top:8px;border-top:2px solid var(--line)}
h3{font-size:18px;margin:24px 0 8px}
h4{font-size:16px;margin:0 0 6px}
.ptr{font:12px ui-monospace,SFMono-Regular,Menlo,monospace;color:var(--muted);text-decoration:none;margin-left:6px;overflow-wrap:anywhere}
.ptr:hover{text-decoration:underline}
article.element{border:1px solid var(--line);border-radius:8px;padding:12px 14px;margin:10px 0;background:var(--bg)}
.route{margin:16px 0}
.route-name{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}
:target{background:var(--target);outline:2px solid var(--accent);outline-offset:2px;scroll-margin-top:60px}
dl{display:grid;grid-template-columns:minmax(90px,max-content) 1fr;gap:2px 12px;margin:6px 0}
dt{color:var(--muted);font-size:13px;padding-top:1px}
dd{margin:0;min-width:0}
dd>p:first-child{margin-top:0}
dd>p:last-child{margin-bottom:0}
dd dl{margin:0;padding-left:8px;border-left:2px solid var(--line)}
ol.items{margin:4px 0;padding-left:22px}
.scroll{overflow-x:auto;max-width:100%}
table{border-collapse:collapse;margin:8px 0;font-size:14px}
th,td{border:1px solid var(--line);padding:4px 8px;text-align:left;vertical-align:top}
thead th{background:var(--soft)}
caption{text-align:left;color:var(--muted);font-size:13px;padding-bottom:4px}
td.yes{text-align:center}
tr.gap>th{color:var(--warning)}
.mark{margin:6px 0;padding:6px 10px;border-left:4px solid var(--warning);background:var(--soft);font-size:14px}
.mark.error{border-color:var(--error)}
.mark.question{border-color:var(--question)}
.mark>a:first-child{font-weight:600}
.notice{padding:8px 12px;border-radius:6px;background:var(--soft);border-left:4px solid var(--accent)}
.notice.draft{border-color:var(--warning)}
.notice.error{border-color:var(--error)}
.origin,.insight,.note,.trace{font-size:14px}
.note{color:var(--muted)}
.how{color:var(--muted);font-size:14px}
figure{margin:12px 0}
figcaption{color:var(--muted);font-size:13px}
figure.diagram{overflow-x:auto}
figure.diagram img{max-width:none;background:var(--bg)}
figure.preview iframe{width:100%;height:340px;border:1px solid var(--line);border-radius:6px;background:#fff}
svg.er{max-width:none}
.er-box{fill:var(--bg);stroke:var(--fg);stroke-width:1.5}
.er-title{fill:var(--soft);stroke:var(--fg);stroke-width:1.5}
.er-name{fill:var(--fg);font:600 13px system-ui,sans-serif}
.er-field{fill:var(--fg);font:12px ui-monospace,Menlo,monospace}
.er-type{fill:var(--muted);font:12px ui-monospace,Menlo,monospace}
.er-line{fill:none;stroke:var(--muted);stroke-width:1.5}
.er-head{fill:var(--muted)}
.er-label{fill:var(--fg);font:11px system-ui,sans-serif;paint-order:stroke;stroke:var(--bg);stroke-width:3px}
ol.problems{padding-left:22px}
ol.problems>li{margin:8px 0;padding:6px 10px;border-left:4px solid var(--warning)}
ol.problems>li.error{border-color:var(--error)}
ol.problems>li.question{border-color:var(--question)}
ol.problems p{margin:2px 0}
.problem-note{padding-left:16px;color:var(--muted)}
[hidden]{display:none!important}
`

const siteJS = `
(function(){
var root=document.documentElement,button=document.getElementById("theme");
function stored(){try{return localStorage.getItem("specarch-theme")}catch(e){return null}}
var t=stored();if(t==="light"||t==="dark")root.setAttribute("data-theme",t);
button.addEventListener("click",function(){
var dark=root.getAttribute("data-theme")?root.getAttribute("data-theme")==="dark":matchMedia("(prefers-color-scheme: dark)").matches;
var next=dark?"light":"dark";root.setAttribute("data-theme",next);
try{localStorage.setItem("specarch-theme",next)}catch(e){}
});
var input=document.getElementById("search"),found=document.getElementById("found");
var items=Array.prototype.slice.call(document.querySelectorAll("[data-search]"));
var sections=Array.prototype.slice.call(document.querySelectorAll("section.section"));
var texts=items.map(function(e){return e.textContent.toLowerCase()});
input.addEventListener("input",function(){
var q=input.value.trim().toLowerCase(),n=0;
items.forEach(function(e,i){var hit=!q||texts[i].indexOf(q)>=0;e.hidden=!hit;if(hit)n++});
sections.forEach(function(s){
var own=s.querySelectorAll("[data-search]");
if(!own.length){s.hidden=!!q&&s.textContent.toLowerCase().indexOf(q)<0;return}
var any=false;for(var i=0;i<own.length;i++)if(!own[i].hidden){any=true;break}
s.hidden=!any;
});
found.textContent=q?n+(n===1?" match":" matches"):"";
});
})();
`
