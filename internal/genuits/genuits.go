// Package genuits writes the screens of a specification for the web in
// TypeScript, on Next.js's app router and IBM's Carbon design system: the
// logic of the specarch-gen-ui-typescript plug-in. Each page becomes a
// schema file that holds data only and a thin page.tsx; the components
// that read the schemas are written once under screens/, and every text
// once in strings.ts. What draws each page kind and field type, and the
// keys of its schema, are the names of the ui-components idiom.
package genuits

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/genui"
	"github.com/SpecArch/specarch/internal/ownership"
	"github.com/SpecArch/specarch/internal/wirename"
)

// Request is what specarch writes on a plug-in's standard input.
type Request struct {
	Specarch        string                 `json:"specarch"`
	Target          string                 `json:"target"`
	Root            string                 `json:"root"`
	Specification   map[string]any         `json:"specification"`
	Implementations []genui.Implementation `json:"implementations"`
	Output          string                 `json:"output"`
	Existing        []genui.File           `json:"existing"`
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

// The plug-in's name, as its header and its messages give it.
const name = "specarch-gen-ui-typescript"

// The framework this generator writes, the stack the idiom renders.
const framework = "nextjs-carbon"

//go:embed screens
var screens embed.FS

// The screens' own texts, the generator's words where a page has none.
var screenStrings = map[string]string{
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
	"screens.reason":         "Give a reason.",
	"screens.language":       "Language",
}

type gen struct {
	spec      map[string]any
	impl      genui.Implementation
	settings  map[string]any
	root      string
	wireNames string
	diags     []genui.Diagnostic
	names     map[string]map[string]string // the ui-components idiom's names, by part
	strings   map[string]string
	calls     map[string]map[string]string // the operations the pages call, by path and method
	server    *server
}

func (g *gen) problem(severity, path, format string, args ...any) {
	g.diags = append(g.diags, genui.Diagnostic{File: g.root, Line: 1, Severity: severity, Path: path, Rule: "generator", Message: fmt.Sprintf(format, args...)})
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
func Generate(r *Request) genui.Response {
	g := &gen{spec: r.Specification, root: r.Root, strings: map[string]string{}, calls: map[string]map[string]string{}}
	g.wireNames = text(obj0(g.spec["info"])["wireNames"])
	found := false
	for _, impl := range r.Implementations {
		t, ok := obj(obj0(impl.Content["targets"])[r.Target])
		if !ok && len(r.Implementations) != 1 {
			continue
		}
		found = true
		g.impl = impl
		g.settings = obj0(t["settings"])
		if impl.Settings != nil {
			g.settings = impl.Settings
		}
		if p, f := text(t["platform"]), text(t["framework"]); p != "web" || f != framework {
			g.problem("error", "/", "the %s target is platform %q and framework %q, and %s writes platform web in %s only", r.Target, p, f, name, framework)
		}
		if lang := text(obj0(obj0(impl.Content["stack"])["language"])["name"]); !strings.EqualFold(lang, "TypeScript") {
			g.problem("error", "/", "%s is written in %q, and %s writes TypeScript; name TypeScript under stack.language", impl.File, lang, name)
		}
		break
	}
	if !found {
		g.problem("error", "/", "no implementation file names the %s target; add it, platform web and framework %s, to the TypeScript file", r.Target, framework)
	}
	if text(g.settings["language"]) == "" {
		g.problem("error", "/", "the %s target's settings name no language, the language of the strings and the pages (WCAG 2.2, 3.1.1); add language, such as en", r.Target)
	}
	g.names = g.idiomNames()
	if g.failed() {
		return g.response(nil)
	}
	g.server = g.readServer()
	header := fmt.Sprintf("// Generated by %s from %s, version %s; meta-model %s. Do not edit this file: change the YAML and generate again.\n",
		name, relRoot(r.Root, r.Output), text(obj0(g.spec["info"])["version"]), r.Specarch)
	var files []genui.File
	sess := g.readSession()
	pages := obj0(g.spec["pages"])
	owned := ownership.Of(g.impl.Content)
	drawn := map[string]bool{}
	hooks, layouts := g.formSettings()
	for _, pageName := range sortedKeys(pages) {
		pg := obj0(pages[pageName])
		if owned.Covers(ownership.Entity("pages", pageName)) {
			continue // another stakeholder's page; a link to it stays
		}
		folder := "app" + routeFolder(text(pg["route"]))
		switch text(pg["kind"]) {
		case "task":
			schema, page, ok := g.taskPage(pageName, pg)
			if !ok {
				continue
			}
			files = append(files,
				genui.File{Path: folder + "/page.schema.ts", Content: header + schema},
				genui.File{Path: folder + "/page.tsx", Content: header + page})
			drawn[pageName] = true
		case "list":
			schema, page, test, ok := g.listPage(pageName, pg, sess)
			if !ok {
				continue
			}
			files = append(files,
				genui.File{Path: folder + "/page.schema.ts", Content: header + schema},
				genui.File{Path: folder + "/page.tsx", Content: header + page})
			drawn[pageName] = true
			if test != "" {
				files = append(files, genui.File{Path: "tests/" + pageName + ".test.ts", Content: header + test})
			}
		case "form", "view":
			var out pageOut
			var ok bool
			if text(pg["kind"]) == "form" {
				out, ok = g.formPage(pageName, pg, sess, hooks[pageName], layouts[pageName])
			} else {
				out, ok = g.viewPage(pageName, pg, sess)
			}
			if !ok {
				continue
			}
			files = append(files,
				genui.File{Path: folder + "/page.schema.ts", Content: header + out.schema},
				genui.File{Path: folder + "/page.tsx", Content: header + out.page})
			if out.client != "" {
				files = append(files, genui.File{Path: folder + "/page.client.tsx", Content: header + out.client})
			}
			drawn[pageName] = true
			if out.test != "" {
				files = append(files, genui.File{Path: "tests/" + pageName + ".test.ts", Content: header + out.test})
			}
		default:
			g.problem("warning", "/pages/"+pageName, "%s is a %s, which this version of %s does not write; it is left out", pageName, text(pg["kind"]), name)
		}
	}
	if g.server != nil {
		listed := map[string]bool{}
		for _, pageName := range sortedKeys(pages) {
			if pg := obj0(pages[pageName]); text(pg["kind"]) == "list" && drawn[pageName] {
				listed[text(pg["source"])] = true
			}
		}
		g.checkPaged(g.server, listed)
		files = append(files, g.serverFiles(g.server, header)...)
	}
	app, menuEntries := g.application(sess, drawn)
	files = append(files, genui.File{Path: "application.ts", Content: header + app})
	if len(menuEntries) > 0 {
		files = append(files, genui.File{Path: "tests/menu.test.ts", Content: header + menuTest()})
	}
	strs, _ := g.stringsFile(r.Existing)
	theme, themed := g.themeFile()
	if g.failed() {
		return g.response(nil)
	}
	entries, _ := screens.ReadDir("screens")
	for _, e := range entries {
		data, _ := screens.ReadFile("screens/" + e.Name())
		files = append(files, genui.File{Path: "screens/" + e.Name(), Content: header + string(data)})
	}
	files = append(files,
		genui.File{Path: "strings.ts", Content: header + strs},
		genui.File{Path: "language.ts", Content: header + g.languageFile()})
	if themed {
		files = append(files, genui.File{Path: "theme.scss", Content: header + theme})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	g.handWritten(files, r.Existing)
	return g.response(files)
}

// handWritten refuses to write over a file in the output folder that this
// plug-in did not write: a file written by hand is never touched.
func (g *gen) handWritten(files, existing []genui.File) {
	have := map[string]string{}
	for _, f := range existing {
		have[f.Path] = f.Content
	}
	for _, f := range files {
		if content, ok := have[f.Path]; ok && !strings.HasPrefix(content, "// Generated by "+name+" ") {
			g.problem("error", "/", "%s is in the output folder and was not written by %s, so it is left as it is and nothing is written; move it, or delete it to have it generated", f.Path, name)
		}
	}
}

func (g *gen) response(files []genui.File) genui.Response {
	if files == nil || g.failed() {
		files = []genui.File{}
	}
	diags := g.diags
	if diags == nil {
		diags = []genui.Diagnostic{}
	}
	return genui.Response{Files: files, Diagnostics: diags}
}

// idiomNames reads the names of every part of the ui-components idiom for
// nextjs-carbon, the project's override first, key by key.
func (g *gen) idiomNames() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, i := range g.impl.Idioms {
		if i.Name != "ui-components" || i.As == "excluded" {
			continue
		}
		for _, src := range []map[string]any{i.Override, i.Content} {
			parts := obj0(src["parts"])
			for _, part := range sortedKeys(parts) {
				n := obj0(obj0(obj0(obj0(parts[part])["stack"])[framework])["names"])
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
		g.problem("error", "/", "%s uses no ui-components idiom, which names what draws each page; use the shipped one, or a project idiom of that name", g.impl.File)
	}
	return out
}

// name is a name of a part of the idiom, or an error naming what is missing.
func (g *gen) name(part, role string) string {
	if v := g.names[part][role]; v != "" {
		return v
	}
	g.problem("error", "/", "the ui-components idiom's part %s names no %s for %s; add it under names", part, role, framework)
	return role
}

// say records a text under its string key and gives the key.
func (g *gen) say(key, text string) string {
	g.strings[key] = text
	return key
}

// field is one field of a task page, as its schema writes it.
type field struct {
	wire string
	data []member
}

// taskPage writes the schema and the page of a task page.
func (g *gen) taskPage(pageName string, pg map[string]any) (string, string, bool) {
	at := "/pages/" + pageName
	before := len(g.diags)
	submit := text(pg["submit"])
	op, path, method := g.operation(submit)
	if op == nil || method == "get" {
		g.problem("error", at+"/submit", "%s submits to %s, which is not an operation of the specification that takes a request body", pageName, submit)
		return "", "", false
	}
	body := g.bodySchema(op)
	props := obj0(body["properties"])
	required := map[string]bool{}
	for _, r := range list(body["required"]) {
		required[text(r)] = true
	}
	if pathParam.MatchString(path) {
		g.problem("error", at+"/submit", "%s submits to %s at %s, whose path takes a parameter, and this version of %s sends a task's fields in the body only", pageName, submit, path, name)
	}
	if pg["sections"] != nil {
		g.problem("error", at+"/sections", "%s gives its fields in sections, and this version of %s writes a task's fields as one list; list them under fields", pageName, name)
	}
	if pg["actions"] != nil {
		g.problem("error", at+"/actions", "%s has actions, and this version of %s does not write a task page's actions yet", pageName, name)
	}
	if text(pg["enabledBy"]) != "" {
		g.problem("error", at+"/enabledBy", "%s is switched on by %s, and this version of %s does not read configuration, so the page would be served while it is off", pageName, text(pg["enabledBy"]), name)
	}
	if p := text(pg["permission"]); p != "public" {
		g.problem("error", at+"/permission", "%s needs %s, and this version of %s writes public task pages only; the route guard that refuses a page comes with the lists", pageName, p, name)
	}
	if list(pg["enteredTwice"]) != nil {
		g.problem("error", at+"/enteredTwice", "%s has fields entered twice, and this version of %s does not write them yet; compare the two with a check instead", pageName, name)
	}
	wires := map[string]string{}
	var fields []field
	for i, f := range list(pg["fields"]) {
		fname := text(f)
		wires[fname] = wirename.Of(g.wireNames, fname)
		fl, ok := g.field(pageName, fmt.Sprintf("%s/fields/%d", at, i), fname, obj0(props[fname]), required[fname])
		if ok {
			fields = append(fields, fl)
		}
	}
	var checks []value
	checksMap := obj0(pg["checks"])
	for _, key := range sortedKeys(checksMap) {
		c := obj0(checksMap[key])
		ptr := at + "/checks/" + key
		node, errs := expr.Parse(text(c["expression"]))
		if len(errs) > 0 {
			g.problem("error", ptr+"/expression", "the check %s does not parse: %s", key, errs[0].Message)
			continue
		}
		rule, ok := g.rule(node, wires, ptr+"/expression")
		if !ok {
			continue
		}
		entry := []member{{"name", str(key)}, {"rule", rule}, {"message", str(g.say(pageName+".checks."+key, text(c["message"])))}}
		if f := text(c["field"]); f != "" {
			entry = append(entry, member{"field", str(wireOf(wires, f))})
		}
		checks = append(checks, object(entry))
	}
	var failures []value
	failedStates := obj0(obj0(pg["states"])["failed"])
	for _, problem := range sortedKeys(failedStates) {
		st := obj0(failedStates[problem])
		if problem == "default" {
			failures = append(failures, object([]member{{"problem", str(problem)}, {"message", str(g.say(pageName+".failed.default", text(st["message"])))}}))
			continue
		}
		status := g.problemStatus(op, problem)
		if status == "" {
			g.problem("error", at+"/states/failed/"+problem, "%s shows a message for %s, which %s does not answer with a status", pageName, problem, submit)
			continue
		}
		entry := []member{{"status", raw(status)}, {"problem", str(problem)}, {"message", str(g.say(pageName+".failed."+problem, text(st["message"])))}}
		if f := text(st["field"]); f != "" {
			entry = append(entry, member{"field", str(wireOf(wires, f))})
		}
		failures = append(failures, object(entry))
	}
	var events []value
	routes := map[string]string{}
	pages := obj0(g.spec["pages"])
	submitted := obj0(pg["onSubmitted"])
	for _, status := range sortedKeys(obj0(op["responses"])) {
		if _, ok := submitted[status]; !ok && strings.HasPrefix(status, "2") {
			g.problem("error", at+"/onSubmitted", "%s answers %s, and %s says nothing of it, so the page could not tell the person what happened; add onSubmitted %q", submit, status, pageName, status)
		}
	}
	for _, status := range sortedKeys(submitted) {
		ev := obj0(submitted[status])
		entry := []member{{"status", raw(status)}}
		if target := text(ev["navigate"]); target != "" {
			tp := obj0(pages[target])
			routes[target] = text(tp["route"])
			entry = append(entry, member{"navigate", str(target)})
			if with := obj0(ev["with"]); len(with) > 0 {
				var ws []member
				for _, k := range sortedKeys(with) {
					ws = append(ws, member{k, str(wirename.Of(g.wireNames, text(with[k])))})
				}
				entry = append(entry, member{"with", object(ws)})
			}
			if m := text(ev["message"]); m != "" {
				entry = append(entry, member{"message", str(g.say(pageName+".onSubmitted."+status, m))})
			}
			if text(tp["kind"]) == "task" {
				entry = append(entry, member{"keepsReturnTo", raw("true")})
			}
		} else if m := text(ev["message"]); m != "" {
			entry = append(entry, member{"message", str(g.say(pageName+".onSubmitted."+status, m))})
		}
		events = append(events, object(entry))
	}
	if len(g.diags) > before && g.failedSince(before) {
		return "", "", false
	}
	part := "task-page"
	var fieldValues []value
	for _, f := range fields {
		fieldValues = append(fieldValues, object(f.data))
	}
	schemaValue := object([]member{
		{g.name(part, "title"), str(g.say(pageName+".title", text(pg["title"])))},
		{g.name(part, "submit"), object([]member{{"operation", str(submit)}, {"method", str(strings.ToUpper(method))}, {"path", str(path)}})},
		{g.name(part, "fields"), array(fieldValues)},
		{g.name(part, "checks"), array(checks)},
		{g.name(part, "failed"), array(failures)},
		{g.name(part, "events"), array(events)},
	})
	components := g.name("application", "components")
	schemaType := g.name(part, "schemaType")
	component := g.name(part, "component")
	var schema strings.Builder
	fmt.Fprintf(&schema, "import type { %s } from %q;\n\n", schemaType, components)
	schema.WriteString(statement("export const schema = ", schemaValue, " satisfies "+schemaType+";"))
	var page strings.Builder
	fmt.Fprintf(&page, "import type { Metadata } from \"next\";\nimport { %s } from %q;\nimport { chosenLanguage } from \"@/language\";\nimport { stringOf, texts } from \"@/strings\";\nimport { schema } from \"./page.schema\";\n\n", component, components)
	page.WriteString(metadataOf(pageName))
	page.WriteString("const routes = {\n")
	for _, target := range sortedKeys(anyMap(routes)) {
		fmt.Fprintf(&page, "  %s: %s,\n", key(target), quote(routes[target]))
	}
	page.WriteString("};\n\n")
	page.WriteString("export default async function Page({ searchParams }: { readonly searchParams: Promise<Record<string, string | string[] | undefined>> }) {\n")
	page.WriteString("  const { returnTo } = await searchParams;\n  const t = texts(schema, await chosenLanguage());\n")
	fmt.Fprintf(&page, "  return <%s schema={schema} texts={t} routes={routes} returnTo={typeof returnTo === \"string\" ? returnTo : undefined} />;\n}\n", component)
	return schema.String(), page.String(), true
}

func (g *gen) failedSince(before int) bool {
	for _, d := range g.diags[before:] {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

// field reads one field of a task page, a property of its request body.
func (g *gen) field(pageName, at, fname string, prop map[string]any, required bool) (field, bool) {
	if prop == nil {
		g.problem("error", at, "%s shows %s, which is not a property of the request body it sends", pageName, fname)
		return field{}, false
	}
	if t := text(prop["type"]); t != "string" {
		g.problem("error", at, "%s shows %s, of type %s, and this version of %s writes task fields of type string only", pageName, fname, orText(t, "none"), name)
		return field{}, false
	}
	if prop["enum"] != nil {
		g.problem("error", at, "%s shows %s, whose values are an enum, and this version of %s writes task fields that hold free text only", pageName, fname, name)
		return field{}, false
	}
	label := text(prop["title"])
	if label == "" {
		g.problem("warning", at, "%s has no title, so its label is its name; give the property a title", fname)
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
		g.problem("error", at, "%s shows %s, of format %s, and this version of %s writes task fields of format email or password, or of none", pageName, fname, format, name)
		return field{}, false
	}
	wire := wirename.Of(g.wireNames, fname)
	data := []member{
		{"type", str(g.name(part, "type"))},
		{g.name(part, "name"), str(wire)},
		{g.name(part, "label"), str(g.say(pageName+".fields."+fname, label))},
		{g.name(part, "required"), raw(strconv.FormatBool(required))},
	}
	var rules []value
	for _, kw := range []string{"minLength", "maxLength", "pattern"} {
		v, ok := prop[kw]
		if !ok {
			continue
		}
		if kw != "maxLength" && part == "email-field" {
			g.problem("warning", at, "%s is an email address with %s, which the page does not check before it is sent; the service still checks it", fname, kw)
			continue
		}
		if kw == "pattern" {
			data = append(data, member{g.name(part, kw), str(text(v))})
			rules = append(rules, object([]member{{"text", str("screens.rule.pattern")}, {"pattern", str(text(v))}}))
			continue
		}
		data = append(data, member{g.name(part, kw), raw(text(v))})
		rules = append(rules, object([]member{{"text", str("screens." + kw)}, {"count", raw(text(v))}}))
	}
	if part == "password-field" {
		data = append(data, member{g.name(part, "rules"), array(rules)})
	}
	return field{wire: wire, data: data}, true
}

// rule turns an expression into the schema's rule, the operators the
// screens evaluate; anything else is refused at the check.
func (g *gen) rule(n *expr.Node, wires map[string]string, at string) (value, bool) {
	sub := func(i int) (value, bool) { return g.rule(n.Args[i], wires, at) }
	switch n.Op {
	case expr.OpName:
		return array([]value{str("field"), str(wireOf(wires, n.Text))}), true
	case expr.OpInt, expr.OpUint:
		return array([]value{str("value"), raw(n.Int.String())}), true
	case expr.OpDouble:
		return array([]value{str("value"), raw(strconv.FormatFloat(n.Double, 'g', -1, 64))}), true
	case expr.OpText:
		return array([]value{str("value"), str(n.Text)}), true
	case expr.OpBool:
		return array([]value{str("value"), raw(strconv.FormatBool(n.Bool))}), true
	case expr.OpNull:
		return array([]value{str("value"), raw("null")}), true
	case expr.OpNot:
		a, ok := sub(0)
		return array([]value{str("!"), a}), ok
	case "==", "!=", "<", "<=", ">", ">=", "&&", "||":
		a, ok1 := sub(0)
		b, ok2 := sub(1)
		return array([]value{str(n.Op), a, b}), ok1 && ok2
	case expr.OpCall:
		if n.Text == "size" && len(n.Args) == 1 {
			a, ok := sub(0)
			return array([]value{str("size"), a}), ok
		}
		g.problem("error", at, "the check calls %s, and this version of %s checks in the browser with comparisons, &&, ||, ! and size only; check it on the server, or rewrite it with those", n.Text, name)
		return nil, false
	}
	g.problem("error", at, "the check uses %s, and this version of %s checks in the browser with comparisons, &&, ||, ! and size only; check it on the server, or rewrite it with those", n.Op, name)
	return nil, false
}

func wireOf(wires map[string]string, fname string) string {
	if w, ok := wires[fname]; ok {
		return w
	}
	return fname
}

// bodySchema is the JSON schema of an operation's request body, an entity
// it refers to read in its place.
func (g *gen) bodySchema(op map[string]any) map[string]any {
	s := obj0(obj0(obj0(obj0(obj0(op["requestBody"])["content"])["application/json"])["schema"]))
	if ref, ok := strings.CutPrefix(text(s["$ref"]), "#/entities/"); ok {
		return obj0(obj0(g.spec["entities"])[ref])
	}
	return s
}

// problemStatus is the status under which an operation answers a problem.
func (g *gen) problemStatus(op map[string]any, problem string) string {
	rs := obj0(op["responses"])
	for _, code := range sortedKeys(rs) {
		if _, err := strconv.Atoi(code); err == nil && text(obj0(rs[code])["problem"]) == problem {
			return code
		}
	}
	return ""
}

var pathParam = regexp.MustCompile(`\{[^{}]+\}`)

// operation finds an operation by its operationId: it, its path and its
// method.
func (g *gen) operation(id string) (map[string]any, string, string) {
	paths := obj0(g.spec["paths"])
	for _, p := range sortedKeys(paths) {
		item := obj0(paths[p])
		for _, m := range []string{"get", "post", "put", "patch", "delete"} {
			if op := obj0(item[m]); op != nil && text(op["operationId"]) == id {
				if g.calls[p] == nil {
					g.calls[p] = map[string]string{}
				}
				g.calls[p][m] = id
				return op, p, m
			}
		}
	}
	return nil, "", ""
}

// routeFolder is the app router's folder of a route: {param} becomes
// [param].
func routeFolder(route string) string {
	return pathParam.ReplaceAllStringFunc(route, func(p string) string { return "[" + p[1:len(p)-1] + "]" })
}

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
