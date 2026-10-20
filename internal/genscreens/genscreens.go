// Package genscreens reads the screens of a specification for the UI
// generators that write a page as data: the task, list and form pages and
// the views, each as the members of a schema keyed by the names of the
// ui-components idiom for the generator's framework, with the routes its
// events lead to, the hooks it takes and whether it needs a permission;
// the session the target's settings name, the menu, the translations the
// settings name, the theme's colours by the design-tokens idiom, and the
// problems each reports. A generator (specarch-gen-ui-typescript,
// specarch-gen-ui-swiftui) gives its Profile, reads the pages through a Gen
// and writes them in its own stack's files.
package genscreens

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/genui"
	"github.com/SpecArch/specarch/internal/mark"
	"github.com/SpecArch/specarch/internal/wirename"
)

// Request is what specarch writes on a plug-in's standard input.
type Request struct {
	Specarch        string                 `json:"specarch"`
	Target          string                 `json:"target"`
	Root            string                 `json:"root"`
	Specification   map[string]any         `json:"specification"`
	Implementations []genui.Implementation `json:"implementations"`
	// APIDialect is the dialect the service serves its API in, dxlib or ""
	// for the standard one (ADR-161).
	APIDialect string       `json:"apiDialect"`
	Output     string       `json:"output"`
	Existing   []genui.File `json:"existing"`
	// Draft is the notice a draft carries, or "" for the approved output.
	Draft string `json:"draft"`
	// Problems are the problems to mark, in the order of the problems file.
	Problems []mark.Problem `json:"problems"`
	// Check is true when specarch only compares the output with the disk,
	// so a file built by hand is compared rather than refused.
	Check bool `json:"check"`
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

// Profile is what a generator says of itself: its name, the platform and
// framework it writes, the language of the implementation file it reads,
// and where its screens evaluate a rule.
type Profile struct {
	// Name is the plug-in's name, as its header and its messages give it.
	Name string
	// Platform is the ui target's platform the generator writes, web or
	// iphone.
	Platform string
	// Framework is the stack the idioms render for it, such as
	// nextjs-carbon or swiftui, the target's framework given or by default.
	Framework string
	// Language is the implementation file's language, as stack.language
	// names it.
	Language string
	// Where is where the screens evaluate a rule, in messages: the browser,
	// or the app.
	Where string
	// ComparesDecimals is true when the screens compare a decimal exactly,
	// so a rule may order one; the browser cannot.
	ComparesDecimals bool
	// OtherMode says, in a message, why a theme mode other than dark is
	// left out on this platform, after "the mode <name> ": on the web, it
	// has no media query of its own.
	OtherMode string
	// PagedByServer answers whether a server the generator writes pages an
	// operation that does not page itself, with the page size and maximum
	// it pages at; nil for a generator that writes no server.
	PagedByServer func(id string) (pageSize, maximum string, ok bool)
	// Seals is true when the screens call a service in the dxlib dialect
	// and perform its pre-key handshake, so a page may call an operation
	// that travels in that envelope (ADR-161).
	Seals bool
	// Streams is true when the screens stream a file: an upload from a
	// task page's submit and a download from an action (ADR-146, ADR-161).
	Streams bool
}

// dxlibEnvelope are the primitives of dxlib's pre-key envelope, the one
// envelope the screens seal (ADR-145).
var dxlibEnvelope = []struct{ key, value string }{{"keyAgreement", "x25519"}, {"signature", "ed25519"}, {"cipher", "aes-256-cbc"}}

// DefaultFramework is a ui target's framework when it names none, by its
// platform.
var DefaultFramework = map[string]string{"web": "plain-javascript", "iphone": "swiftui"}

// ScreenStrings are the screens' own texts, the generator's words where a
// page has none; a translation must give each of them too.
var ScreenStrings = map[string]string{
	"screens.email":          "Type an email address, such as name@example.org.",
	"screens.failed":         "This cannot be done right now. Try again in a moment.",
	"screens.maxLength":      "Use at most {count} characters.",
	"screens.minLength":      "Use at least {count} characters.",
	"screens.pattern":        "This is not in the form asked for.",
	"screens.required":       "Fill this in.",
	"screens.rule.pattern":   "It matches {pattern}.",
	"screens.actions":        "Actions",
	"screens.any":            "Any",
	"screens.cancel":         "Cancel",
	"screens.columns":        "Columns",
	"screens.filter":         "Filter",
	"screens.loading":        "Loading",
	"screens.menu":           "Menu",
	"screens.refresh":        "Refresh",
	"screens.refused":        "You do not hold the permission this page needs.",
	"screens.search":         "Search",
	"screens.again":          "The two entries differ.",
	"screens.back":           "Back",
	"screens.date":           "Type a day as yyyy-mm-dd.",
	"screens.maximum":        "Use {count} or less.",
	"screens.minimum":        "Use {count} or more.",
	"screens.next":           "Next",
	"screens.no":             "No",
	"screens.none":           "None",
	"screens.nothingMatches": "Nothing matches.",
	"screens.number":         "Type a number.",
	"screens.step":           "Use a multiple of {count}.",
	"screens.yes":            "Yes",
	"screens.addRow":         "Add a row",
	"screens.removeRow":      "Remove this row",
	"screens.rowsAtLeast":    "Add at least {count} rows.",
	"screens.rowsAtMost":     "Keep to {count} rows at most.",
	"screens.addItem":        "Add an item",
	"screens.removeItem":     "Remove this item",
	"screens.itemsAtLeast":   "Add at least {count} items.",
	"screens.itemsAtMost":    "Keep to {count} items at most.",
	"screens.reason":         "Give a reason.",
	"screens.language":       "Language",
}

// Gen reads one request's screens for a generator.
type Gen struct {
	Profile  Profile
	Spec     map[string]any
	Impl     genui.Implementation
	Settings map[string]any
	Root     string
	// WireNames is the specification's wire naming, info.wireNames.
	WireNames string
	Diags     []genui.Diagnostic
	// Names are the ui-components idiom's names for the framework, by part,
	// the project's override first.
	Names map[string]map[string]string
	// Strings are the texts the pages show, by string key.
	Strings map[string]string
	// Calls are the operations the pages call, by path and method.
	Calls map[string]map[string]string
	// Dialect is the dialect an implementation file serves the API in,
	// dxlib or standard.
	Dialect string
	// Handshakes are the handshakes a page's call travels in, by name, with
	// the operation that establishes each.
	Handshakes map[string]string
}

// New reads the request for a generator: the implementation file that
// names the target, its settings, its language and the target's platform
// and framework checked against the profile, and the idiom's names. An
// error is reported for what does not fit; the caller checks Failed.
func New(r *Request, p Profile) *Gen {
	g := &Gen{Profile: p, Spec: r.Specification, Root: r.Root, Strings: map[string]string{}, Calls: map[string]map[string]string{}, Dialect: "standard", Handshakes: map[string]string{}}
	if r.APIDialect == "dxlib" {
		g.Dialect = "dxlib"
		if !p.Seals {
			g.Problem("error", "/", "the service serves its API in the dxlib dialect, a POST at /<operationId> with every parameter in one JSON body and lists counted from page 0, and %s calls the standard dialect only", p.Name)
		}
	}
	g.WireNames = text(obj0(g.Spec["info"])["wireNames"])
	found := false
	for _, impl := range r.Implementations {
		t, ok := obj(obj0(impl.Content["targets"])[r.Target])
		if !ok && len(r.Implementations) != 1 {
			continue
		}
		found = true
		g.Impl = impl
		g.Settings = obj0(t["settings"])
		if impl.Settings != nil {
			g.Settings = impl.Settings
		}
		platform, framework := text(t["platform"]), text(t["framework"])
		if framework == "" {
			framework = DefaultFramework[platform]
		}
		if platform != p.Platform || framework != p.Framework {
			g.Problem("error", "/", "the %s target is platform %q and framework %q, and %s writes platform %s in %s only", r.Target, platform, text(t["framework"]), p.Name, p.Platform, p.Framework)
		}
		if lang := text(obj0(obj0(impl.Content["stack"])["language"])["name"]); !strings.EqualFold(lang, p.Language) {
			g.Problem("error", "/", "%s is written in %q, and %s writes %s; name %s under stack.language", impl.File, lang, p.Name, p.Language, p.Language)
		}
		break
	}
	if !found {
		g.Problem("error", "/", "no implementation file names the %s target; add it, platform %s and framework %s, to the %s file", r.Target, p.Platform, p.Framework, p.Language)
	}
	if text(g.Settings["language"]) == "" {
		g.Problem("error", "/", "the %s target's settings name no language, the language of the strings and the pages (WCAG 2.2, 3.1.1); add language, such as en", r.Target)
	}
	g.Names = g.idiomNames()
	return g
}

// Problem reports a problem at a path of the specification.
func (g *Gen) Problem(severity, path, format string, args ...any) {
	g.Diags = append(g.Diags, genui.Diagnostic{File: g.Root, Line: 1, Severity: severity, Path: path, Rule: "generator", Message: fmt.Sprintf(format, args...)})
}

// Failed says whether an error was reported.
func (g *Gen) Failed() bool {
	for _, d := range g.Diags {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

// FailedSince says whether an error was reported after the first before
// diagnostics.
func (g *Gen) FailedSince(before int) bool {
	for _, d := range g.Diags[before:] {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

// Header is the line every generated file starts with, after the comment
// opener the stack writes.
func (g *Gen) Header(r *Request) string {
	return fmt.Sprintf("Generated by %s from %s, version %s; meta-model %s. Do not edit this file: change the YAML and generate again.",
		g.Profile.Name, RelRoot(r.Root, r.Output), text(obj0(g.Spec["info"])["version"]), r.Specarch)
}

// Mark writes the draft notice under the header of every file, the marks
// of a page under the header of each file written for it, and the marks
// of the screens as a whole under the header of the file whole names
// (internal/mark). header is the header line with its comment opener and
// its line break, and comment the opener a mark's line starts with.
func (g *Gen) Mark(r *Request, files []genui.File, header, comment string, pageOf map[string]string, drawn map[string]bool, whole string) {
	var elements []string
	for p := range drawn {
		elements = append(elements, "/pages/"+mark.Escape(p))
	}
	placed := mark.Place(r.Problems, elements, []string{"/pages", "/menus"})
	for i, f := range files {
		var b strings.Builder
		b.WriteString(header)
		if r.Draft != "" {
			b.WriteString(comment + r.Draft + "\n")
		}
		at := ""
		switch {
		case pageOf[f.Path] != "":
			at = "/pages/" + mark.Escape(pageOf[f.Path])
		case f.Path == whole:
			at = mark.File
		default:
			files[i].Content = b.String() + strings.TrimPrefix(f.Content, header)
			continue
		}
		for _, p := range placed[at] {
			b.WriteString(comment + mark.Line(p) + "\n")
		}
		files[i].Content = b.String() + strings.TrimPrefix(f.Content, header)
	}
}

// HandWritten refuses to write over a file in the output folder that this
// plug-in did not write: a file written by hand is never touched. prefix
// is the start of the header line the plug-in writes, with its comment
// opener.
func (g *Gen) HandWritten(files, existing []genui.File, prefix string) bool {
	have := map[string]string{}
	for _, f := range existing {
		have[f.Path] = f.Content
	}
	found := false
	for _, f := range files {
		if content, ok := have[f.Path]; ok && !strings.HasPrefix(content, prefix) {
			g.Problem("error", "/", "%s is in the output folder and was not written by %s, so it is left as it is and nothing is written; move it, or delete it to have it generated", f.Path, g.Profile.Name)
			found = true
		}
	}
	return found
}

// Response is the plug-in's answer: its files and its diagnostics, each
// never null.
func (g *Gen) Response(files []genui.File) genui.Response {
	if files == nil {
		files = []genui.File{}
	}
	diags := g.Diags
	if diags == nil {
		diags = []genui.Diagnostic{}
	}
	return genui.Response{Files: files, Diagnostics: diags}
}

// idiomNames reads the names of every part of the ui-components idiom for
// the framework, the project's override first, key by key.
func (g *Gen) idiomNames() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, i := range g.Impl.Idioms {
		if i.Name != "ui-components" || i.As == "excluded" {
			continue
		}
		for _, src := range []map[string]any{i.Override, i.Content} {
			parts := obj0(src["parts"])
			for _, part := range sortedKeys(parts) {
				n := obj0(obj0(obj0(obj0(parts[part])["stack"])[g.Profile.Framework])["names"])
				if out[part] == nil {
					out[part] = map[string]string{}
				}
				for k, v := range n {
					if _, ok := out[part][k]; !ok {
						out[part][k] = text(v)
					}
				}
			}
		}
	}
	if len(out) == 0 {
		g.Problem("error", "/", "%s uses no ui-components idiom, which names what draws each page; use the shipped one, or a project idiom of that name", g.Impl.File)
	}
	return out
}

// Name is a name of a part of the idiom, or an error naming what is missing.
func (g *Gen) Name(part, role string) string {
	if v := g.Names[part][role]; v != "" {
		return v
	}
	g.Problem("error", "/", "the ui-components idiom's part %s names no %s for %s; add it under names", part, role, g.Profile.Framework)
	return role
}

// ImportOf is where a part's component and schema type come from: its own
// import, or the application's components.
func (g *Gen) ImportOf(part string) string {
	if i := g.Names[part]["import"]; i != "" {
		return i
	}
	return g.Name("application", "components")
}

// Say records a text under its string key and gives the key.
func (g *Gen) Say(key, text string) string {
	g.Strings[key] = text
	return key
}

// Page is one page read: its schema's members in the idiom's names, the
// pages its events lead to by their routes, and what its files need.
type Page struct {
	Name string
	// Kind is the page's kind: task, list, form or view.
	Kind string
	// Part is the idiom part that draws it: task-page, list-page,
	// form-page or view-page.
	Part string
	// Route is the page's route, with its {parameters}.
	Route string
	// Permission is the page's permission; public for a task page.
	Permission string
	// Schema are the members of the schema, in order.
	Schema []Member
	// Routes are the pages the page's events and actions lead to, by name,
	// each with its route.
	Routes map[string]string
	// Hooks are the fields of a form whose starting value a hook gives.
	Hooks []string
}

// Parameters are the parameters of the page's route, in order.
func (p *Page) Parameters() []string {
	var out []string
	for _, m := range PathParam.FindAllString(p.Route, -1) {
		out = append(out, m[1:len(m)-1])
	}
	return out
}

// field is one field of a task page, as its schema writes it.
type field struct {
	wire string
	data []Member
}

// TaskPage reads a task page: a form that submits to an operation without
// loading a record.
func (g *Gen) TaskPage(pageName string, pg map[string]any) (*Page, bool) {
	at := "/pages/" + pageName
	before := len(g.Diags)
	name := g.Profile.Name
	submit := text(pg["submit"])
	op, path, method := g.Operation(submit)
	if op == nil || method == "get" {
		g.Problem("error", at+"/submit", "%s submits to %s, which is not an operation of the specification that takes a request body", pageName, submit)
		return nil, false
	}
	if !g.PlainOnly(at+"/submit", pageName, "submits to", submit, op, "upload") {
		return nil, false
	}
	upload := text(obj0(op["stream"])["direction"]) == "upload"
	body := g.BodySchema(op)
	props := obj0(body["properties"])
	required := map[string]bool{}
	for _, r := range list(body["required"]) {
		required[text(r)] = true
	}
	if PathParam.MatchString(path) {
		g.Problem("error", at+"/submit", "%s submits to %s at %s, whose path takes a parameter, and this version of %s sends a task's fields in the body only", pageName, submit, path, name)
	}
	if upload && len(props) > 0 && g.Dialect != "dxlib" {
		g.Problem("error", at+"/submit", "%s submits to %s, which takes parameters beside the file it uploads (%s), and they travel in a header the stack names; the standard dialect names none, so %s would send the file without them (ADR-162)", pageName, submit, strings.Join(sortedKeys(props), ", "), name)
	}
	if pg["sections"] != nil {
		g.Problem("error", at+"/sections", "%s gives its fields in sections, and this version of %s writes a task's fields as one list; list them under fields", pageName, name)
	}
	if pg["actions"] != nil {
		g.Problem("error", at+"/actions", "%s has actions, and this version of %s does not write a task page's actions yet", pageName, name)
	}
	if text(pg["enabledBy"]) != "" {
		g.Problem("error", at+"/enabledBy", "%s is switched on by %s, and this version of %s does not read configuration, so the page would be served while it is off", pageName, text(pg["enabledBy"]), name)
	}
	if p := text(pg["permission"]); p != "public" {
		g.Problem("error", at+"/permission", "%s needs %s, and this version of %s writes public task pages only; the route guard that refuses a page comes with the lists", pageName, p, name)
	}
	if list(pg["enteredTwice"]) != nil {
		g.Problem("error", at+"/enteredTwice", "%s has fields entered twice, and this version of %s does not write them yet; compare the two with a check instead", pageName, name)
	}
	wires := map[string]string{}
	var fields []field
	for i, f := range list(pg["fields"]) {
		fname := text(f)
		wires[fname] = wirename.Of(g.WireNames, fname)
		fl, ok := g.field(pageName, fmt.Sprintf("%s/fields/%d", at, i), fname, obj0(props[fname]), required[fname])
		if ok {
			fields = append(fields, fl)
		}
	}
	var checks []Value
	checksMap := obj0(pg["checks"])
	for _, key := range sortedKeys(checksMap) {
		c := obj0(checksMap[key])
		ptr := at + "/checks/" + key
		node, errs := expr.Parse(text(c["expression"]))
		if len(errs) > 0 {
			g.Problem("error", ptr+"/expression", "the check %s does not parse: %s", key, errs[0].Message)
			continue
		}
		rule, ok := g.Rule(node, wires, ptr+"/expression")
		if !ok {
			continue
		}
		entry := []Member{{"name", Str(key)}, {"rule", rule}, {"message", Str(g.Say(pageName+".checks."+key, text(c["message"])))}}
		if f := text(c["field"]); f != "" {
			entry = append(entry, Member{"field", Str(wireOf(wires, f))})
		}
		checks = append(checks, Object(entry))
	}
	var failures []Value
	failedStates := obj0(obj0(pg["states"])["failed"])
	for _, problem := range sortedKeys(failedStates) {
		st := obj0(failedStates[problem])
		if problem == "default" {
			failures = append(failures, Object([]Member{{"problem", Str(problem)}, {"message", Str(g.Say(pageName+".failed.default", text(st["message"])))}}))
			continue
		}
		status := g.ProblemStatus(op, problem)
		if status == "" {
			g.Problem("error", at+"/states/failed/"+problem, "%s shows a message for %s, which %s does not answer with a status", pageName, problem, submit)
			continue
		}
		entry := []Member{{"status", Raw(status)}, {"problem", Str(problem)}, {"message", Str(g.Say(pageName+".failed."+problem, text(st["message"])))}}
		if f := text(st["field"]); f != "" {
			entry = append(entry, Member{"field", Str(wireOf(wires, f))})
		}
		failures = append(failures, Object(entry))
	}
	var events []Value
	routes := map[string]string{}
	pages := obj0(g.Spec["pages"])
	submitted := obj0(pg["onSubmitted"])
	for _, status := range sortedKeys(obj0(op["responses"])) {
		if _, ok := submitted[status]; !ok && strings.HasPrefix(status, "2") {
			g.Problem("error", at+"/onSubmitted", "%s answers %s, and %s says nothing of it, so the page could not tell the person what happened; add onSubmitted %q", submit, status, pageName, status)
		}
	}
	for _, status := range sortedKeys(submitted) {
		ev := obj0(submitted[status])
		entry := []Member{{"status", Raw(status)}}
		if target := text(ev["navigate"]); target != "" {
			tp := obj0(pages[target])
			routes[target] = text(tp["route"])
			entry = append(entry, Member{"navigate", Str(target)})
			if with := obj0(ev["with"]); len(with) > 0 {
				var ws []Member
				for _, k := range sortedKeys(with) {
					ws = append(ws, Member{k, Str(wirename.Of(g.WireNames, text(with[k])))})
				}
				entry = append(entry, Member{"with", Dictionary(ws)})
			}
			if m := text(ev["message"]); m != "" {
				entry = append(entry, Member{"message", Str(g.Say(pageName+".onSubmitted."+status, m))})
			}
			if text(tp["kind"]) == "task" {
				entry = append(entry, Member{"keepsReturnTo", Raw("true")})
			}
		} else if m := text(ev["message"]); m != "" {
			entry = append(entry, Member{"message", Str(g.Say(pageName+".onSubmitted."+status, m))})
		}
		events = append(events, Object(entry))
	}
	if len(g.Diags) > before && g.FailedSince(before) {
		return nil, false
	}
	part := "task-page"
	var fieldValues []Value
	for _, f := range fields {
		fieldValues = append(fieldValues, Object(f.data))
	}
	submitMembers := append([]Member{{"operation", Str(submit)}, {"method", Str(strings.ToUpper(method))}, {"path", Str(path)}}, g.Transport(op, path)...)
	if upload {
		file := []Member{
			{"label", Str(g.Say(pageName+".file.label", orText(text(obj0(op["requestBody"])["description"]), "File")))},
			{"button", Str(g.Say(pageName+".file.button", "Choose a file"))},
		}
		if m := text(obj0(op["limits"])["maxRequestBytes"]); m != "" {
			file = append(file, Member{"maxBytes", Raw(m)}, Member{"tooLarge", Str(g.Say(pageName+".file.tooLarge", "The file is larger than "+m+" bytes."))})
		}
		submitMembers = append(submitMembers, Member{"file", Object(file)})
	}
	schema := []Member{
		{g.Name(part, "title"), Str(g.Say(pageName+".title", text(pg["title"])))},
		{g.Name(part, "submit"), Object(submitMembers)},
		{g.Name(part, "fields"), Array(fieldValues)},
		{g.Name(part, "checks"), Array(checks)},
		{g.Name(part, "failed"), Array(failures)},
		{g.Name(part, "events"), Array(events)},
	}
	return &Page{Name: pageName, Kind: "task", Part: part, Route: text(pg["route"]), Permission: "public", Schema: schema, Routes: routes}, true
}

// field reads one field of a task page, a property of its request body.
func (g *Gen) field(pageName, at, fname string, prop map[string]any, required bool) (field, bool) {
	name := g.Profile.Name
	if prop == nil {
		g.Problem("error", at, "%s shows %s, which is not a property of the request body it sends", pageName, fname)
		return field{}, false
	}
	if t := text(prop["type"]); t != "string" {
		g.Problem("error", at, "%s shows %s, of type %s, and this version of %s writes task fields of type string only", pageName, fname, orText(t, "none"), name)
		return field{}, false
	}
	if prop["enum"] != nil {
		g.Problem("error", at, "%s shows %s, whose values are an enum, and this version of %s writes task fields that hold free text only", pageName, fname, name)
		return field{}, false
	}
	label := text(prop["title"])
	if label == "" {
		g.Problem("warning", at, "%s has no title, so its label is its name; give the property a title", fname)
		label = fname
	}
	var part string
	switch format := text(prop["format"]); format {
	case "":
		part = "text-field"
	case "email":
		part = "email-field"
	case "password":
		part = "password-field"
	default:
		g.Problem("error", at, "%s shows %s, of format %s, and this version of %s writes task fields of format email or password, or of none", pageName, fname, format, name)
		return field{}, false
	}
	wire := wirename.Of(g.WireNames, fname)
	data := []Member{
		{"type", Str(g.Name(part, "type"))},
		{g.Name(part, "name"), Str(wire)},
		{g.Name(part, "label"), Str(g.Say(pageName+".fields."+fname, label))},
		{g.Name(part, "required"), Raw(strconv.FormatBool(required))},
	}
	var rules []Value
	for _, kw := range []string{"minLength", "maxLength", "pattern"} {
		v, ok := prop[kw]
		if !ok {
			continue
		}
		if kw != "maxLength" && part == "email-field" {
			g.Problem("warning", at, "%s is an email address with %s, which the page does not check before it is sent; the service still checks it", fname, kw)
			continue
		}
		if kw == "pattern" {
			data = append(data, Member{g.Name(part, kw), Str(text(v))})
			rules = append(rules, Object([]Member{{"text", Str("screens.rule.pattern")}, {"pattern", Str(text(v))}}))
			continue
		}
		data = append(data, Member{g.Name(part, kw), Raw(text(v))})
		rules = append(rules, Object([]Member{{"text", Str("screens." + kw)}, {"count", Raw(text(v))}}))
	}
	if part == "password-field" {
		data = append(data, Member{g.Name(part, "rules"), Array(rules)})
	}
	return field{wire: wire, data: data}, true
}

// Rule turns an expression into the schema's rule, the operators the
// screens evaluate; anything else is refused at the check.
func (g *Gen) Rule(n *expr.Node, wires map[string]string, at string) (Value, bool) {
	sub := func(i int) (Value, bool) { return g.Rule(n.Args[i], wires, at) }
	switch n.Op {
	case expr.OpName:
		return RuleOf([]Value{Str("field"), Str(wireOf(wires, n.Text))}), true
	case expr.OpInt, expr.OpUint:
		return RuleOf([]Value{Str("value"), Raw(n.Int.String())}), true
	case expr.OpDouble:
		return RuleOf([]Value{Str("value"), Raw(strconv.FormatFloat(n.Double, 'g', -1, 64))}), true
	case expr.OpText:
		return RuleOf([]Value{Str("value"), Str(n.Text)}), true
	case expr.OpBool:
		return RuleOf([]Value{Str("value"), Raw(strconv.FormatBool(n.Bool))}), true
	case expr.OpNull:
		return RuleOf([]Value{Str("value"), Raw("null")}), true
	case expr.OpNot:
		a, ok := sub(0)
		return RuleOf([]Value{Str("!"), a}), ok
	case "==", "!=", "<", "<=", ">", ">=", "&&", "||":
		a, ok1 := sub(0)
		b, ok2 := sub(1)
		return RuleOf([]Value{Str(n.Op), a, b}), ok1 && ok2
	case expr.OpCall:
		if n.Text == "size" && len(n.Args) == 1 {
			a, ok := sub(0)
			return RuleOf([]Value{Str("size"), a}), ok
		}
		g.Problem("error", at, "the check calls %s, and this version of %s checks in %s with comparisons, &&, ||, ! and size only; check it on the server, or rewrite it with those", n.Text, g.Profile.Name, g.Profile.Where)
		return nil, false
	}
	g.Problem("error", at, "the check uses %s, and this version of %s checks in %s with comparisons, &&, ||, ! and size only; check it on the server, or rewrite it with those", n.Op, g.Profile.Name, g.Profile.Where)
	return nil, false
}

func wireOf(wires map[string]string, fname string) string {
	if w, ok := wires[fname]; ok {
		return w
	}
	return fname
}

// BodySchema is the JSON schema of an operation's request body, an entity
// it refers to read in its place.
func (g *Gen) BodySchema(op map[string]any) map[string]any {
	s := obj0(obj0(obj0(obj0(obj0(op["requestBody"])["content"])["application/json"])["schema"]))
	if st := obj0(op["stream"]); text(st["direction"]) == "upload" {
		// The body is the file; a task's fields are the parameters sent
		// beside it (ADR-162).
		s = obj0(st["parameters"])
	}
	if ref, ok := strings.CutPrefix(text(s["$ref"]), "#/entities/"); ok {
		return obj0(obj0(g.Spec["entities"])[ref])
	}
	return s
}

// ProblemStatus is the status under which an operation answers a problem.
func (g *Gen) ProblemStatus(op map[string]any, problem string) string {
	rs := obj0(op["responses"])
	for _, code := range sortedKeys(rs) {
		if _, err := strconv.Atoi(code); err == nil && text(obj0(rs[code])["problem"]) == problem {
			return code
		}
	}
	return ""
}

// PathParam matches a {parameter} of a route or a path.
var PathParam = regexp.MustCompile(`\{[^{}]+\}`)

// PlainOnly says whether an operation is one the screens can call: plain,
// or in the envelope of a handshake the profile seals, dxlib's pre-key
// envelope in the dxlib dialect (ADR-145, ADR-161); not one that streams a
// file, unless the profile streams files and the caller is where that
// direction is taken, allow: upload for a task page's submit, download for
// an action (ADR-146). An error at the page otherwise.
func (g *Gen) PlainOnly(at, pageName, does, id string, op map[string]any, allow string) bool {
	if h := text(op["handshake"]); h != "" {
		if !g.Profile.Seals {
			g.Problem("error", at, "%s %s %s, which travels in the envelope of handshake %s, and this version of %s performs no handshake, so the screen would send plain JSON the service refuses; see docs/roadmap.md", pageName, does, id, h, g.Profile.Name)
			return false
		}
		if g.Dialect != "dxlib" {
			g.Problem("error", at, "%s %s %s, which travels in the envelope of handshake %s, and %s seals an envelope in the dxlib dialect only, whose bytes dxlib's datablock fixes; no implementation file serves the API in the dxlib dialect", pageName, does, id, h, g.Profile.Name)
			return false
		}
		hs := obj0(obj0(g.Spec["handshakes"])[h])
		var differs []string
		for _, p := range dxlibEnvelope {
			if text(hs[p.key]) != p.value {
				differs = append(differs, p.key+" "+orText(text(hs[p.key]), "none"))
			}
		}
		if len(differs) > 0 {
			g.Problem("error", at, "%s %s %s, which travels in the envelope of handshake %s, whose %s is not dxlib's envelope (x25519, ed25519 and aes-256-cbc), the one %s seals", pageName, does, id, h, strings.Join(differs, " and "), g.Profile.Name)
			return false
		}
		g.Handshakes[h] = text(hs["establishedBy"])
	}
	if s := text(obj0(op["stream"])["direction"]); s != "" && (!g.Profile.Streams || s != allow) {
		if g.Profile.Streams {
			g.Problem("error", at, "%s %s %s, which streams a file (%s), and %s streams an upload only from a task page's submit and a download only from an action", pageName, does, id, s, g.Profile.Name)
		} else {
			g.Problem("error", at, "%s %s %s, which streams a file (%s), and this version of %s streams no file", pageName, does, id, s, g.Profile.Name)
		}
		return false
	}
	return true
}

// Transport are the members a call's schema carries beside its operation
// and path: the handshake whose envelope it travels in, and in the dxlib
// dialect each path parameter's name in the body, the snake-case name the
// dxlib dialect gives it.
func (g *Gen) Transport(op map[string]any, path string) []Member {
	var out []Member
	if h := text(op["handshake"]); h != "" {
		out = append(out, Member{"handshake", Str(h)})
	}
	switch text(obj0(op["stream"])["direction"]) {
	case "upload":
		out = append(out, Member{"stream", Str("upload")})
		for _, mt := range sortedKeys(obj0(obj0(op["requestBody"])["content"])) {
			out = append(out, Member{"mediaType", Str(mt)})
			break
		}
	case "download":
		out = append(out, Member{"stream", Str("download")})
	}
	if g.Dialect == "dxlib" {
		var wire []Member
		for _, p := range PathParam.FindAllString(path, -1) {
			name := p[1 : len(p)-1]
			wire = append(wire, Member{name, Str(wirename.Snake(name))})
		}
		if len(wire) > 0 {
			out = append(out, Member{"wire", Object(wire)})
		}
	}
	return out
}

// Operation finds an operation by its operationId: it, its path and its
// method, and records the call.
func (g *Gen) Operation(id string) (map[string]any, string, string) {
	paths := obj0(g.Spec["paths"])
	for _, p := range sortedKeys(paths) {
		item := obj0(paths[p])
		for _, m := range []string{"get", "post", "put", "patch", "delete"} {
			if op := obj0(item[m]); op != nil && text(op["operationId"]) == id {
				if g.Calls[p] == nil {
					g.Calls[p] = map[string]string{}
				}
				g.Calls[p][m] = id
				return op, p, m
			}
		}
	}
	return nil, "", ""
}

// RelRoot is the root file's path relative to the output folder, for the
// header.
func RelRoot(root, output string) string {
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

// Obj is a value as an object, when it is one.
func Obj(v any) (map[string]any, bool) { return obj(v) }

// Obj0 is a value as an object, or nil.
func Obj0(v any) map[string]any { return obj0(v) }

// List is a value as a list, or nil.
func List(v any) []any { return list(v) }

// Text is a value's text: a string as it is, a number as its digits.
func Text(v any) string { return text(v) }

// OrText is s, or fallback when s is empty.
func OrText(s, fallback string) string { return orText(s, fallback) }

// SortedKeys are a map's keys in order.
func SortedKeys(m map[string]any) []string { return sortedKeys(m) }

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

func anyMap(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
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

func orText(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
