// Package genui writes the screens of a specification for the web in plain
// JavaScript: the logic of the specarch-gen-ui plug-in. Each list page
// becomes an HTML page and an ES module, the events of every page are
// declared once in events.js, and the theme becomes theme.css. Nothing
// needs a package, a bundler or a build step.
package genui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/SpecArch/specarch/internal/mark"
	"github.com/SpecArch/specarch/internal/ownership"
)

// Request is what specarch writes on a plug-in's standard input.
type Request struct {
	Specarch        string           `json:"specarch"`
	Target          string           `json:"target"`
	Root            string           `json:"root"`
	Specification   map[string]any   `json:"specification"`
	Implementations []Implementation `json:"implementations"`
	// APIDialect is the dialect the service serves its API in, dxlib or ""
	// for the standard one (ADR-161).
	APIDialect string `json:"apiDialect"`
	Output     string `json:"output"`
	// Draft is the notice a draft carries, or "" for the approved output.
	Draft string `json:"draft"`
	// Problems are the problems to mark, in the order of the problems file.
	Problems []mark.Problem `json:"problems"`
}

// Implementation is one implementation file in the request.
type Implementation struct {
	File     string         `json:"file"`
	Content  map[string]any `json:"content"`
	Settings map[string]any `json:"settings,omitempty"`
	Idioms   []Idiom        `json:"idioms"`
}

// Idiom is one idiom an implementation file uses.
type Idiom struct {
	Name     string         `json:"name"`
	As       string         `json:"as"`
	Content  map[string]any `json:"content,omitempty"`
	Override map[string]any `json:"override,omitempty"`
}

// File is one file the plug-in answers with.
type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	// Comparison is "object-literal" for a file that is one object
	// literal of data, which generate --check compares by value (ADR-093).
	Comparison string `json:"comparison,omitempty"`
}

// Diagnostic is one problem the plug-in reports, in the validator's fields.
type Diagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

// Response is what the plug-in writes on its standard output.
type Response struct {
	Files       []File       `json:"files"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Decode reads a request, keeping numbers as their text.
func Decode(data []byte) (*Request, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var r Request
	if err := dec.Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

type gen struct {
	spec     map[string]any
	impl     Implementation
	settings map[string]any
	root     string
	diags    []Diagnostic
	list     map[string]string // the paginated-list idiom's names
	// opened are the pages a link may open: those written and those
	// another stakeholder owns; menuEntry the menu entries written.
	opened, menuEntry map[string]bool
}

func (g *gen) problem(severity, path, format string, args ...any) {
	g.diags = append(g.diags, Diagnostic{File: g.root, Line: 1, Severity: severity, Path: path, Rule: "generator", Message: fmt.Sprintf(format, args...)})
}

func (g *gen) failed() bool {
	for _, d := range g.diags {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

// Generate writes the screens of a request.
func Generate(r *Request) Response {
	g := &gen{spec: r.Specification, root: r.Root}
	for _, impl := range r.Implementations {
		if t, ok := obj(obj0(impl.Content["targets"])[r.Target]); ok || len(r.Implementations) == 1 {
			g.impl = impl
			g.settings = obj0(t["settings"])
			if impl.Settings != nil {
				g.settings = impl.Settings
			}
			if p, f := text(t["platform"]), text(t["framework"]); p != "web" || f != "plain-javascript" {
				g.problem("error", "/", "the %s target is platform %q and framework %q, and specarch-gen-ui writes platform web in plain-javascript only", r.Target, p, f)
			}
			break
		}
	}
	if r.APIDialect == "dxlib" {
		g.problem("error", "/", "the service serves its API in the dxlib dialect, whose lists are a POST per operation counted from page 0, and specarch-gen-ui writes screens for the standard dialect only")
	}
	if text(g.settings["language"]) == "" {
		g.problem("error", "/", "the %s target's settings name no language, the page's lang (WCAG 2.2, 3.1.1); add language, such as en", r.Target)
	}
	g.list = g.idiomNames("paginated-list", "parameters")
	for k, v := range g.idiomNames("paginated-list", "envelope") {
		g.list[k] = v
	}
	if g.failed() {
		return g.response(nil)
	}
	pages := obj0(g.spec["pages"])
	owned := ownership.Of(g.impl.Content)
	// screen is one page written: its HTML, its module and its events.
	type screen interface {
		html(g *gen) string
		js() string
		events() []event
	}
	type written struct {
		name string
		p    screen
	}
	var ws []written
	var elements []string
	var lists []*page
	var views []*viewPage
	var forms []*formPage
	g.opened = map[string]bool{}
	for _, name := range sortedKeys(pages) {
		pg := obj0(pages[name])
		if owned.Covers(ownership.Entity("pages", name)) {
			g.opened[name] = true
			continue // another stakeholder's page; a link to it stays
		}
		switch text(pg["kind"]) {
		case "list":
			if p := g.page(name, pg); p != nil {
				lists = append(lists, p)
				ws = append(ws, written{name, p})
			}
		case "view":
			if v := g.view(name, pg); v != nil {
				views = append(views, v)
				ws = append(ws, written{name, v})
			}
		case "form", "task":
			if f := g.form(name, pg); f != nil {
				forms = append(forms, f)
				ws = append(ws, written{name, f})
			}
		default:
			g.problem("warning", "/pages/"+name, "%s is a %s, which specarch-gen-ui does not write; it is left out", name, text(pg["kind"]))
			continue
		}
	}
	for _, w := range ws {
		g.opened[w.name] = true
		elements = append(elements, "/pages/"+mark.Escape(w.name))
	}
	// A link to a page that is not written is left out, reported at it.
	keepActions := func(actions []action) []action {
		var out []action
		for _, a := range actions {
			if a.link != nil && len(g.keep([]link{*a.link})) == 0 {
				continue
			}
			if a.navigate != nil && len(g.keep([]link{*a.navigate})) == 0 {
				a.navigate = nil
			}
			out = append(out, a)
		}
		return out
	}
	for _, p := range lists {
		p.links = g.keep(p.links)
		p.actions = keepActions(p.actions)
		if p.selected != nil && len(g.keep([]link{*p.selected})) == 0 {
			p.selected = nil
		}
	}
	for _, v := range views {
		v.actions = keepActions(v.actions)
	}
	for _, f := range forms {
		for status, n := range f.next {
			if n.page != "" && len(g.keep([]link{{label: f.name, page: n.page, at: "/pages/" + mark.Escape(f.name) + "/onSubmitted"}})) == 0 {
				n.page = ""
				f.next[status] = n
			}
		}
	}
	g.checkMenus()
	var events []event
	for _, w := range ws {
		events = append(events, w.p.events()...)
	}
	docs := map[string]string{}
	for _, e := range events {
		if d, ok := docs[e.name]; ok && d != e.doc {
			g.problem("error", "/pages", "two events of the screens are both named %s (%s; and %s); rename a page or an operation so their names differ", e.name, d, e.doc)
		}
		docs[e.name] = e.doc
	}
	// Each page is a file of its own, so a page's marks stand under its
	// files' header; the draft notice stands in every file, and the marks
	// of the screens as a whole in events.js, which every page loads.
	placed := mark.Place(r.Problems, elements, []string{"/pages", "/menus"})
	header := func(open, close string, marks []mark.Problem) string {
		info := obj0(g.spec["info"])
		h := fmt.Sprintf("%sGenerated by specarch-gen-ui from %s, version %s; meta-model %s. Do not edit this file: change the YAML and generate again.%s\n",
			open, relRoot(r.Root, r.Output), text(info["version"]), r.Specarch, close)
		// A comment's text may not close it: "--" is written "- -" in
		// HTML, as in XML, and "*/" is written "* /" in CSS.
		safe := func(t string) string {
			if close == " -->" {
				for strings.Contains(t, "--") {
					t = strings.ReplaceAll(t, "--", "- -")
				}
			}
			if close == " */" {
				t = strings.ReplaceAll(t, "*/", "* /")
			}
			return t
		}
		if r.Draft != "" {
			h += open + safe(r.Draft) + close + "\n"
		}
		for _, p := range marks {
			h += open + safe(mark.Line(p)) + close + "\n"
		}
		return h
	}
	var files []File
	for _, w := range ws {
		h := w.p.html(g)
		files = append(files,
			File{Path: w.name + ".html", Content: strings.Replace(h, "\n", "\n"+header("<!-- ", " -->", placed["/pages/"+mark.Escape(w.name)]), 1)},
			File{Path: w.name + ".js", Content: header("// ", "", placed["/pages/"+mark.Escape(w.name)]) + w.p.js()})
	}
	files = append(files, File{Path: "events.js", Content: header("// ", "", placed[mark.File]) + eventsJS(events)})
	files = append(files, File{Path: "pages.js", Content: header("// ", "", nil) + g.pagesJS(sortedSet(g.opened))})
	asks := false
	for _, p := range lists {
		asks = asks || len(operations(p.actions)) > 0
	}
	for _, v := range views {
		asks = asks || len(operations(v.actions)) > 0
	}
	if asks {
		files = append(files, File{Path: "ask.js", Content: header("// ", "", nil) + askRuntime})
	}
	if len(views) > 0 {
		files = append(files, File{Path: "view.js", Content: header("// ", "", nil) + viewRuntime})
	}
	if len(forms) > 0 {
		files = append(files, File{Path: "form.js", Content: header("// ", "", nil) + formRuntime})
	}
	files = append(files, File{Path: "theme.css", Content: header("/* ", " */", nil) + g.themeCSS()})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return g.response(files)
}

func (g *gen) response(files []File) Response {
	if files == nil {
		files = []File{}
	}
	diags := g.diags
	if diags == nil {
		diags = []Diagnostic{}
	}
	return Response{Files: files, Diagnostics: diags}
}

// idiomNames reads the names of a part of an idiom for any stack, the
// project's override first.
func (g *gen) idiomNames(idiom, part string) map[string]string {
	out := map[string]string{}
	for _, i := range g.impl.Idioms {
		if i.Name != idiom || i.As == "excluded" {
			continue
		}
		for _, src := range []map[string]any{i.Override, i.Content} {
			if n, ok := obj(obj0(obj0(obj0(obj0(src["parts"])[part])["stack"])["any"])["names"]); ok {
				for k, v := range n {
					if out[k] == "" {
						out[k] = text(v)
					}
				}
			}
		}
	}
	return out
}

// event is one event of the screens, as events.js declares it.
type event struct{ name, doc string }

// action is an operation a list runs on a row.
type action struct {
	label, operation, method, path, confirm, message string
	refusals                                         [][2]string // status and message
	when                                             string      // a function of the record, or ""
	reason                                           *reason
	// navigate is the page the action leads to once it succeeds; link is
	// set instead on an action that opens a page and runs nothing.
	navigate, link *link
}

// reason is the property of an action's request body that carries the
// reason its confirmation asks for.
type reason struct{ name, label, minLength, maxLength string }

// page is what one list page needs.
type page struct {
	name, title, source, sourcePath, upper string
	language                               string
	columns, titles, filters               []string
	wide                                   map[string]bool
	filterHTML                             []string
	pageSize, key                          string
	filterKeys                             map[string]string // a filter's name in the query, where it is not its own
	empty, filteredEmpty, failed           string
	sourceRefusals                         [][2]string
	actions                                []action
	links                                  []link // pages the list opens, above its table
	selected                               *link  // the page a selected row opens
	list                                   map[string]string
}

// The generator's own words, where a page declares no state of its own.
const (
	defaultEmpty         = "There is nothing here yet."
	defaultFilteredEmpty = "Nothing matches these filters."
	defaultFailed        = "This cannot be done right now. Try again in a moment."
)

// page reads a list page, or reports why it cannot be written.
func (g *gen) page(name string, pg map[string]any) *page {
	at := "/pages/" + name
	entity := obj0(obj0(g.spec["entities"])[text(pg["entity"])])
	fields := obj0(entity["properties"])
	p := &page{name: name, title: text(pg["title"]), source: text(pg["source"]), upper: upperSnake(name),
		language: text(g.settings["language"]), wide: map[string]bool{}, list: g.list}
	src, srcPath, srcMethod := g.operation(p.source)
	if src == nil || srcMethod != "get" {
		g.problem("error", at+"/source", "%s reads %s, which is not a GET operation of the specification; this version reads a list by GET", name, p.source)
		return nil
	}
	if !g.plainOnly(at+"/source", name, "reads", p.source, src) {
		return nil
	}
	if pathParam.MatchString(srcPath) {
		g.problem("error", at+"/source", "%s reads %s at %s, whose path takes a parameter, and this version of specarch-gen-ui reads a list whose path takes none", name, p.source, srcPath)
		return nil
	}
	p.sourcePath = jsTemplate(srcPath)
	listOf, ok := obj(src["listOf"])
	size := obj0(listOf["pageSize"])
	p.pageSize = orText(text(size["default"]), text(size["maximum"]))
	switch {
	case !ok:
		g.problem("error", at+"/source", "%s pages through its list, and %s has no listOf, so it does not page; give it listOf", name, p.source)
		return nil
	case p.pageSize == "":
		g.problem("error", at+"/source", "%s pages through its list, and the listOf of %s gives no page size; give pageSize a default", name, p.source)
		return nil
	case g.list["page"] == "" || g.list["items"] == "":
		g.problem("error", at+"/source", "%s pages through the paginated-list idiom, which the implementation file excludes; keep the idiom", name)
		return nil
	}
	p.filterKeys = map[string]string{}
	compact := map[string]bool{}
	for _, c := range list(pg["compactColumns"]) {
		compact[text(c)] = true
	}
	for _, c := range list(pg["columns"]) {
		p.columns = append(p.columns, text(c))
		p.titles = append(p.titles, fieldTitle(fields, text(c)))
		if len(compact) > 0 && !compact[text(c)] {
			p.wide[text(c)] = true
		}
	}
	params := map[string]bool{}
	for _, prm := range list(src["parameters"]) {
		if text(obj0(prm)["in"]) == "query" {
			params[text(obj0(prm)["name"])] = true
		}
	}
	filterable := map[string]bool{}
	for _, f := range list(listOf["filterable"]) {
		filterable[text(f)] = true
	}
	for _, f := range list(pg["filters"]) {
		name := text(f)
		switch {
		case params[name]:
		case filterable[name] && g.list["filter"] != "":
			p.filterKeys[name] = g.list["filter"] + "[" + name + "]"
		default:
			g.problem("error", at+"/filters", "%s filters by %s, which is neither a query parameter of %s nor filterable in its listOf, so the screen cannot ask for it; add it to the operation", p.name, name, p.source)
			continue
		}
		p.filters = append(p.filters, name)
		p.filterHTML = append(p.filterHTML, g.control(fields, name)...)
	}
	states := obj0(pg["states"])
	p.empty = orText(text(obj0(states["empty"])["message"]), defaultEmpty)
	p.filteredEmpty = orText(text(obj0(states["filteredEmpty"])["message"]), defaultFilteredEmpty)
	failed := obj0(states["failed"])
	p.failed = orText(text(obj0(failed["default"])["message"]), defaultFailed)
	p.sourceRefusals = refusals(src, failed)
	pk := list(entity["primaryKey"])
	if len(pk) == 1 {
		p.key = text(pk[0])
	}
	for i, a := range list(pg["actions"]) {
		am := obj0(a)
		at := fmt.Sprintf("%s/actions/%d", at, i)
		if text(am["kind"]) == "navigate" {
			l, row, ok := g.listLink(at, am, fields)
			switch {
			case !ok:
			case row:
				p.actions = append(p.actions, action{label: l.label, link: &l, when: l.when})
			default:
				p.links = append(p.links, l)
			}
			continue
		}
		if a, ok := g.operationAction(at, am, fields, p.key, failed, nil, "row"); ok {
			p.actions = append(p.actions, a)
		}
	}
	if sel := obj0(pg["onSelect"]); text(sel["navigate"]) != "" {
		target := text(sel["navigate"])
		if with, ok := g.withJS(at+"/onSelect", name+"'s selected row", target, obj0(sel["with"]), fields, nil, "row"); ok {
			p.selected = &link{label: name + "'s selected row", page: target, with: with, at: at + "/onSelect"}
		}
	}
	return p
}

var pathParam = regexp.MustCompile(`\{[^{}]+\}`)

// refusals are an operation's refusals the page names a message for, by
// status, in the order of its responses.
func refusals(op, failed map[string]any) [][2]string {
	var out [][2]string
	rs := obj0(op["responses"])
	for _, code := range sortedKeys(rs) {
		problem := text(obj0(rs[code])["problem"])
		if _, err := strconv.Atoi(code); err != nil {
			continue // a response for any other status is the page's default
		}
		if m := text(obj0(failed[problem])["message"]); problem != "" && m != "" {
			out = append(out, [2]string{code, m})
		}
	}
	return out
}

// operation finds an operation by its operationId: it, its path and its
// method.
// plainOnly says whether an operation is one the screens can call plain: not
// one that travels in a handshake's envelope or streams a file, which this
// version does not perform (ADR-145, ADR-146); an error at the page otherwise.
func (g *gen) plainOnly(at, pageName, does, id string, op map[string]any) bool {
	if h := text(op["handshake"]); h != "" {
		g.problem("error", at, "%s %s %s, which travels in the envelope of handshake %s, and this version of specarch-gen-ui performs no handshake, so the screen would send plain JSON the service refuses; see docs/roadmap.md", pageName, does, id, h)
		return false
	}
	if s := text(obj0(op["stream"])["direction"]); s != "" {
		g.problem("error", at, "%s %s %s, which streams a file (%s), and this version of specarch-gen-ui streams no file", pageName, does, id, s)
		return false
	}
	return true
}

func (g *gen) operation(id string) (map[string]any, string, string) {
	paths := obj0(g.spec["paths"])
	for _, p := range sortedKeys(paths) {
		item := obj0(paths[p])
		for _, m := range []string{"get", "post", "put", "patch", "delete"} {
			if op := obj0(item[m]); op != nil && text(op["operationId"]) == id {
				merged := map[string]any{}
				for k, v := range op {
					merged[k] = v
				}
				merged["parameters"] = append(append([]any{}, list(item["parameters"])...), list(op["parameters"])...)
				return merged, p, m
			}
		}
	}
	return nil, "", ""
}

// fieldTitle is a field's title, or its name when it has none.
func fieldTitle(fields map[string]any, name string) string {
	return orText(text(obj0(fields[name])["title"]), name)
}

// control is the form control of a filter: a list of the enum's values, or
// a text input.
func (g *gen) control(fields map[string]any, name string) []string {
	f := obj0(fields[name])
	values := list(f["enum"])
	if ref := text(f["$ref"]); strings.HasPrefix(ref, "#/enums/") {
		values = list(obj0(obj0(g.spec["enums"])[strings.TrimPrefix(ref, "#/enums/")])["enum"])
	}
	out := []string{"<label>" + html.EscapeString(fieldTitle(fields, name))}
	if len(values) > 0 {
		out = append(out, "<select name="+quoted(name)+">", `<option value="">Any</option>`)
		for _, v := range values {
			e := html.EscapeString(text(v))
			out = append(out, "<option value="+quoted(text(v))+">"+e+"</option>")
		}
		out = append(out, "</select>")
	} else {
		out = append(out, "<input name="+quoted(name)+` type="text">`)
	}
	return append(out, "</label>")
}

// events are the events a page raises, and those of its actions.
func (p *page) events() []event {
	filters := "{}"
	if len(p.filters) > 0 {
		filters = "{ " + strings.Join(p.filters, ", ") + " }"
	}
	out := []event{
		{p.upper + "_FILTERED", "The filters of the " + p.lowerTitle() + " list were applied: " + filters},
		{p.upper + "_PAGED", "Another page of the " + p.lowerTitle() + " list was asked for: { page }"},
		{p.upper + "_LOADED", "A page of " + p.lowerTitle() + " arrived: { items, totalItems, totalPages, page, filtered }"},
		{p.upper + "_FAILED", "The " + p.lowerTitle() + " could not be read: { status }"},
	}
	return append(out, actionEvents(p.actions)...)
}

// actionEvents are the events of the actions that run an operation.
func actionEvents(actions []action) []event {
	var out []event
	for _, a := range actions {
		if a.link == nil {
			out = append(out, event{upperSnake(a.operation) + "_SUCCEEDED", a.operation + " succeeded: { id }"},
				event{upperSnake(a.operation) + "_FAILED", a.operation + " was refused or failed: { id, status }"})
		}
	}
	return out
}

// lowerTitle is the page's title in lower case, safe inside a comment.
func (p *page) lowerTitle() string {
	return strings.ReplaceAll(strings.ToLower(p.title), "*/", "* /")
}

// eventsJS declares every event once, and the bus that carries them.
func eventsJS(events []event) string {
	var b strings.Builder
	b.WriteString(`// The events the screens raise, each name declared once with its payload.
// A component subscribes where it is created; no name is a wildcard or
// computed. Set DEBUG to true to log every event.

export const DEBUG = false;

`)
	seen := map[string]bool{}
	for _, e := range events {
		if seen[e.name] {
			continue
		}
		seen[e.name] = true
		fmt.Fprintf(&b, "/** %s */\nexport const %s = %q;\n", strings.ReplaceAll(e.doc, "*/", "* /"), e.name, e.name)
	}
	b.WriteString(`
const handlers = new Map();

/** Calls handler with the payload of every event of that name. */
export function on(name, handler) {
  if (!handlers.has(name)) {
    handlers.set(name, []);
  }
  handlers.get(name).push(handler);
}

/** Raises an event. */
export function emit(name, payload) {
  if (DEBUG) {
    console.log(name, payload);
  }
  for (const handler of handlers.get(name) ?? []) {
    handler(payload);
  }
}
`)
	return b.String()
}

// quoted is a value as an HTML attribute, in double quotes. The text
// escape already writes a double quote as &#34;; the replacement after it
// handles any quote the escape would leave, right where the attribute is
// built, so the value cannot end the attribute whatever the escape does.
func quoted(s string) string {
	return `"` + strings.ReplaceAll(html.EscapeString(s), `"`, "&quot;") + `"`
}

// themeCSS writes the theme's tokens as custom properties, the modes, and
// the rules every screen shares, from the tokens the target's settings
// give each part.
func (g *gen) themeCSS() string {
	var b strings.Builder
	b.WriteString("/* The theme's design tokens as custom properties, then the rules every\n   screen shares. */\n\n")
	theme := obj0(g.spec["theme"])
	var toks []token
	walkTokens(obj0(theme["tokens"]), "", &toks)
	names := map[string]string{}
	for _, t := range toks {
		at := "/theme/tokens/" + strings.ReplaceAll(t.path, ".", "/")
		if !cssIdent.MatchString(strings.ReplaceAll(t.path, ".", "-")) {
			g.problem("error", at, "%s cannot name a CSS custom property; use letters, digits, - and _ in a token's name", t.path)
			continue
		}
		if other, ok := names[cssName(t.path)]; ok {
			g.problem("error", at, "%s and %s are both the custom property --%s; rename one", other, t.path, cssName(t.path))
		}
		names[cssName(t.path)] = t.path
	}
	if len(toks) > 0 {
		b.WriteString(":root {\n")
		for _, t := range toks {
			fmt.Fprintf(&b, "  --%s: %s;\n", cssName(t.path), cssValue(t.typ, t.value))
		}
		b.WriteString("}\n\n")
	}
	modes := obj0(theme["modes"])
	for _, m := range sortedKeys(modes) {
		if m != "dark" {
			g.problem("warning", "/theme/modes/"+m, "the mode %s has no media query of its own on the web, and this version of specarch-gen-ui writes the dark mode only; it is left out", m)
			continue
		}
		b.WriteString("@media (prefers-color-scheme: dark) {\n  :root {\n")
		over := obj0(modes[m])
		for _, t := range toks {
			if v, ok := over[t.path]; ok {
				fmt.Fprintf(&b, "    --%s: %s;\n", cssName(t.path), cssValue(t.typ, v))
			}
		}
		b.WriteString("  }\n}\n\n")
	}
	role := func(name string) string {
		if p := text(obj0(g.settings["tokens"])[name]); p != "" {
			return "var(--" + cssName(p) + ")"
		}
		return ""
	}
	rule := func(selector string, decls ...[2]string) {
		var lines []string
		for _, d := range decls {
			if d[1] != "" {
				lines = append(lines, "  "+d[0]+": "+d[1]+";")
			}
		}
		if len(lines) == 0 {
			return
		}
		b.WriteString(selector + " {\n" + strings.Join(lines, "\n") + "\n}\n\n")
	}
	border := ""
	if r := role("border"); r != "" {
		border = "1px solid " + r
	}
	rule("body", [2]string{"margin", "0"}, [2]string{"padding", role("space")}, [2]string{"color", role("text")}, [2]string{"background", role("background")}, [2]string{"font-family", role("font")})
	rule("a", [2]string{"color", role("accent")})
	rule("table", [2]string{"border-collapse", "collapse"})
	rule("th,\ntd", [2]string{"padding", role("space")}, [2]string{"border-bottom", border}, [2]string{"text-align", "start"})
	rule("button,\ninput,\nselect", [2]string{"min-width", "24px"}, [2]string{"min-height", "24px"}, [2]string{"font", "inherit"})
	rule("form", [2]string{"display", "flex"}, [2]string{"flex-wrap", "wrap"}, [2]string{"gap", role("space")}, [2]string{"align-items", "end"})
	rule("label", [2]string{"display", "flex"}, [2]string{"flex-direction", "column"})
	rule(`[role="status"].failed`, [2]string{"color", role("danger")})
	b.WriteString("@media (max-width: 599px) {\n  .wide {\n    display: none;\n  }\n}\n")
	return b.String()
}

// token is one design token: its path and its value.
type token struct {
	path, typ string
	value     any
}

// walkTokens lists the tokens of a group in order, each with the type it
// or a group above it gives.
func walkTokens(group map[string]any, prefix string, out *[]token) {
	walkTyped(group, prefix, "", out)
}

func walkTyped(group map[string]any, prefix, typ string, out *[]token) {
	if t := text(group["$type"]); t != "" {
		typ = t
	}
	for _, k := range sortedKeys(group) {
		if strings.HasPrefix(k, "$") {
			continue
		}
		path := strings.TrimPrefix(prefix+"."+k, ".")
		v := obj0(group[k])
		if val, ok := v["$value"]; ok {
			*out = append(*out, token{path: path, typ: orText(text(v["$type"]), typ), value: val})
			continue
		}
		walkTyped(v, path, typ, out)
	}
}

// cssIdent is what a custom property's name may hold after its --.
var cssIdent = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// genericFamilies are CSS's generic font families, written without quotes.
var genericFamilies = map[string]bool{"serif": true, "sans-serif": true, "monospace": true, "cursive": true, "fantasy": true,
	"system-ui": true, "ui-serif": true, "ui-sans-serif": true, "ui-monospace": true, "ui-rounded": true, "math": true, "emoji": true, "fangsong": true}

// cssFamily writes a font family: a generic one as it is, any other as a
// CSS string.
func cssFamily(name string) string {
	if genericFamilies[name] {
		return name
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\a `).Replace(name) + `"`
}

// cssName is a token's path as a custom property's name.
func cssName(path string) string { return strings.ReplaceAll(path, ".", "-") }

// CSSValue writes a token's value in CSS, as cssValue does, for another
// generator of the web.
func CSSValue(typ string, v any) string { return cssValue(typ, v) }

// cssValue writes a token's value in CSS: an alias as the property it
// names, a colour as its hex or, translucent, as rgb with its alpha, a
// dimension or a duration with its unit, a font family as its list.
func cssValue(typ string, v any) string {
	if s, ok := v.(string); ok && strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		return "var(--" + cssName(strings.Trim(s, "{}")) + ")"
	}
	if s, ok := v.(string); ok && typ == "fontFamily" {
		return cssFamily(s)
	}
	if m, ok := v.(map[string]any); ok {
		if comps := list(m["components"]); len(comps) == 3 {
			var c [3]int
			for i, x := range comps {
				f, _ := strconv.ParseFloat(text(x), 64)
				c[i] = int(math.Round(f * 255))
			}
			if a := text(m["alpha"]); a != "" && a != "1" {
				return fmt.Sprintf("rgb(%d %d %d / %s)", c[0], c[1], c[2], a)
			}
			return fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2])
		}
		return text(m["value"]) + text(m["unit"])
	}
	if l, ok := v.([]any); ok {
		var names []string
		for _, n := range l {
			names = append(names, cssFamily(text(n)))
		}
		return strings.Join(names, ", ")
	}
	return text(v)
}

// upperSnake writes loans-list as LOANS_LIST, returnLoan as RETURN_LOAN and
// getHTTPStatus as GET_HTTP_STATUS.
func upperSnake(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case r == '-' || r == '.' || r == ' ':
			b.WriteByte('_')
		case unicode.IsUpper(r) && i > 0 && (unicode.IsLower(rs[i-1]) || unicode.IsDigit(rs[i-1]) || i+1 < len(rs) && unicode.IsLower(rs[i+1]) && unicode.IsUpper(rs[i-1])):
			b.WriteByte('_')
			b.WriteRune(r)
		default:
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	return b.String()
}

// jsString writes a string as a JavaScript string literal in double quotes.
func jsString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteRune('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r < 0x20 || r == 0x2028 || r == 0x2029:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// joinAnd joins words as a sentence does: a, b and c.
func joinAnd(words []string) string {
	if len(words) < 2 {
		return strings.Join(words, "")
	}
	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}

// wrap breaks text into lines of at most width characters.
func wrap(text string, width int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		if line != "" && len([]rune(line))+1+len([]rune(word)) > width {
			lines = append(lines, line)
			line = word
			continue
		}
		line = strings.TrimPrefix(line+" "+word, " ")
	}
	return append(lines, line)
}

func orText(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// relRoot is the root file's path as seen from the output folder.
func relRoot(root, output string) string {
	absRoot, err1 := filepath.Abs(filepath.FromSlash(root))
	absOut, err2 := filepath.Abs(filepath.FromSlash(output))
	if err1 != nil || err2 != nil {
		return root
	}
	rel, err := filepath.Rel(absOut, absRoot)
	if err != nil {
		return root
	}
	return filepath.ToSlash(rel)
}

func obj(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func obj0(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func text(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	}
	return fmt.Sprint(v)
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// jsTemplate escapes text for a JavaScript template literal.
func jsTemplate(s string) string {
	return strings.NewReplacer("\\", "\\\\", "`", "\\`", "${", "\\${").Replace(s)
}
