package extract

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The files a file-system router serves a page from, and the schema file
// a page's component library renders it from (ADR-057).
var (
	pageFiles      = map[string]bool{"page.tsx": true, "page.ts": true, "page.jsx": true, "page.js": true}
	routeFiles     = map[string]bool{"route.tsx": true, "route.ts": true, "route.jsx": true, "route.js": true}
	otherPageFile  = regexp.MustCompile(`^page\.[A-Za-z0-9]+$`)
	pageSchemaFile = "page.schema.ts"
)

var (
	typeWord   = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	memberWord = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
	paramName  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	pageKind   = map[string]bool{"list": true, "form": true, "view": true, "task": true}
)

// The schema keys read as the page keywords of the same name, and the two
// named in a question, in the order the page writes them.
var (
	readKeys    = []string{"kind", "title", "entity", "permission", "columns", "compactColumns", "fields", "sections", "filters"}
	namedKeys   = []string{"source", "submit"}
	fieldLists  = []string{"columns", "compactColumns", "fields", "filters"}
	keyForKinds = map[string][]string{
		"columns": {"list"}, "compactColumns": {"list"}, "filters": {"list"},
		"fields": {"form", "view", "task"}, "sections": {"form", "view", "task"},
	}
)

// page is one page the reader writes.
type page struct {
	name, route, folder string
	file                string // the page file, from the repository's root
	says                string // what the page file's place says, as its citation
	schema              string // the schema file, or ""
	kind, title         string
	entity, permission  string
	lists               map[string][]string // columns, compactColumns, fields, filters
	sections            [][2]any            // title, fields
	named               map[string]string   // source and submit the schema names
	read                []string            // the keys the schema gave, in the order written
}

// Pages reads a file-system router's root folder into one page per file or
// folder it serves a page from, with the content of each page that has a
// schema file from it, and one operation per route file whose name gives
// its method. The router is told by the folder's name (ADR-084).
func Pages(root, out, key string) (*Result, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, refuse("%s is not a folder; extract pages reads a file-system router's root folder, such as app, pages or server", root)
	}
	r, err := Open([]string{root})
	if err != nil {
		return nil, err
	}
	rd := &pageReader{r: r, key: key, res: &Result{Tree: newTree()}, base: r.Paths[0]}
	beside, err := rd.beside()
	if err != nil {
		return nil, err
	}
	if len(beside) > 0 {
		paths := []string{root}
		for _, f := range beside {
			paths = append(paths, filepath.Join(r.Repository.Root, filepath.FromSlash(f)))
		}
		if r, err = Open(paths); err != nil {
			return nil, err
		}
		rd.r = r
	}
	commitLine(rd.res, r)
	if err := rd.read(); err != nil {
		return nil, err
	}
	var clauses []*yaml.Node
	for _, pg := range rd.pages {
		clauses = append(clauses, flow(mapping("clause", pg.file, "title", "The page file of "+pg.route)))
		if pg.schema != "" {
			clauses = append(clauses, flow(mapping("clause", pg.schema, "title", "The schema of the page at "+pg.route)))
		}
	}
	for _, rf := range rd.routeFiles {
		clauses = append(clauses, flow(mapping("clause", rf.file, "title", "The route file of "+rf.path)))
	}
	for _, f := range rd.middleware {
		clauses = append(clauses, flow(mapping("clause", f, "title", "Middleware the router runs before the routes it covers")))
	}
	if rd.packageFile != "" {
		clauses = append(clauses, flow(mapping("clause", rd.packageFile, "title", "The package that names the framework")))
	}
	sort.Slice(clauses, func(i, j int) bool {
		return source0(clauses[i]) < source0(clauses[j])
	})
	src, err := codeSource(r, out, clauses)
	if err != nil {
		return nil, err
	}
	res := rd.res
	if len(rd.pages) > 0 {
		design := mapping()
		if rd.entities != nil {
			set(design, "entities", rd.entities)
		}
		if rd.permissions != nil {
			set(design, "permissions", rd.permissions)
		}
		set(design, "pages", rd.pageNodes)
		res.Tree.put("design/pages.yaml", design)
	}
	if rd.paths != nil {
		res.Tree.put("design/paths.yaml", mapping("paths", rd.paths))
	}
	res.Tree.put("requirements/stakeholders.yaml", mapping("stakeholders", ownerStakeholder()))
	res.Tree.put("design/questions.yaml", mapping("questions", rd.questions))
	description := fmt.Sprintf("The pages and operations the file-system router rooted at %s serves, read from the files git tracks there at commit %s, with the content of each page that has a schema file from that file. Every page and operation cites its file; what the files' places and the schemas do not say is a question, and so is what the meta-model cannot hold.\n", rd.base, r.Commit)
	title := "Pages of " + rd.base
	if rd.router == routerNuxtServer {
		title = "Routes of " + rd.base
	}
	res.Tree.put("specarch.yaml", rootFile(title, description, []string{"requirements", "design"}, mapping(key, src)))
	return res, nil
}

func source0(n *yaml.Node) string { return n.Content[1].Value }

type pageReader struct {
	r           *Read
	key         string
	res         *Result
	base        string // the root folder, from the repository's root
	pages       []*page
	pageNodes   *yaml.Node
	entities    *yaml.Node
	permissions *yaml.Node
	questions   *yaml.Node
	nextID      int
	notHeld     []pageNotHeld

	router      string       // which file-system router serves the folder
	routerWhy   string       // what told the reader which router it is
	routerAsk   string       // why the router is a guess, or ""
	packageFile string       // the package.json that names the framework, or ""
	middleware  []string     // the middleware files the router runs, by their place
	routeFiles  []*routeFile // the route files, in the order of their paths
	paths       *yaml.Node   // the operations route files give, or nil
	generated   []string     // a line per file read that says it is generated
}

// pageNotHeld is a thing not held, about a page whose name is known only
// once every page is read.
type pageNotHeld struct {
	notHeld
	pg  *page  // the page it is about, or nil for a section
	key string // the page's key it is about, or "" for the page
	ask bool   // a question about the page's key asks for it instead
}

// gap records what the meta-model cannot hold of no page this reader
// writes, at the file, blocking the section it would be in.
func (rd *pageReader) gap(section, clause string, format string, args ...any) {
	rd.notHeld = append(rd.notHeld, pageNotHeld{notHeld: notHeld{text: fmt.Sprintf(format, args...), clause: clause, blocks: []string{section}}})
}

// pageGap records what the meta-model cannot hold of a page's key, or of
// the page when key is "", at the clause.
func (rd *pageReader) pageGap(pg *page, key string, ask bool, clause string, format string, args ...any) {
	rd.notHeld = append(rd.notHeld, pageNotHeld{notHeld: notHeld{text: fmt.Sprintf(format, args...), clause: clause}, pg: pg, key: key, ask: ask})
}

// held is what was not held, with the pages' final names in its blocks.
func (rd *pageReader) held() []notHeld {
	var out []notHeld
	for _, h := range rd.notHeld {
		if h.pg != nil {
			at := "#/pages/" + h.pg.name
			if h.key != "" {
				at += "/" + h.key
			}
			h.blocks = []string{at}
			if h.ask {
				h.asked = at
			}
		}
		out = append(out, h.notHeld)
	}
	return out
}

func (rd *pageReader) question(text string, blocks []string, why string) {
	rd.namedQuestion(text, blocks, "", why)
}

// namedQuestion asks a must question; names is the name the schema gives
// for the one key the question blocks, or "".
func (rd *pageReader) namedQuestion(text string, blocks []string, names, why string) {
	rd.nextID++
	q := mapping(
		"question", text,
		"kind", "decision",
		"priority", "must",
		"blocks", blocks,
		"decidedBy", owner,
	)
	if names != "" {
		set(q, "names", names)
	}
	set(q, "why", why)
	set(rd.questions, fmt.Sprintf("Q-%d", rd.nextID), q)
}

// within is a file's path below the root folder.
func (rd *pageReader) within(f string) string {
	if rd.base == "." {
		return f
	}
	return strings.TrimPrefix(f, rd.base+"/")
}

// pageCandidate is a page file a router serves a page from, before the
// reader checks that no other file serves its route.
type pageCandidate struct {
	file, folder, route, says string
	words                     []string
}

func (rd *pageReader) read() error {
	var cands []pageCandidate
	schemas := map[string]string{} // folder below the root -> schema file
	rd.res.say("router: %s, since %s", rd.routerName(), rd.routerWhy)
	switch rd.router {
	case routerApp:
		cands, schemas = rd.collectApp()
	case routerNextPages:
		cands = rd.collectNextPages()
	case routerNuxtPages:
		cands = rd.collectNuxtPages()
	case routerNuxtServer:
		rd.collectNuxtServer()
	}
	byRoute := map[string]*page{}
	byFolder := map[string]*page{}
	for _, c := range cands {
		if prev := byRoute[c.route]; prev != nil {
			if rd.router == routerApp {
				return refuse("%s and %s both give the route %s; a file-system router refuses two pages on one route", prev.file, c.file, c.route)
			}
			rd.gap("pages", c.file, "%s: %s already serves the page at %s, and a route has one page; left out", c.file, prev.file, c.route)
			continue
		}
		pg := &page{route: c.route, folder: c.folder, file: c.file, says: c.says, lists: map[string][]string{}, named: map[string]string{}}
		pg.name = strings.Join(c.words, "-")
		if pg.name == "" {
			pg.name = "root"
		}
		if pg.name[0] < 'a' || pg.name[0] > 'z' {
			pg.name = "page-" + pg.name
		}
		byRoute[c.route] = pg
		byFolder[c.folder] = pg
		rd.pages = append(rd.pages, pg)
	}
	rd.settleRouteFiles(byRoute)
	schemaFolders := make([]string, 0, len(schemas))
	for folder := range schemas {
		schemaFolders = append(schemaFolders, folder)
	}
	sort.Strings(schemaFolders)
	read := 0
	for _, folder := range schemaFolders {
		pg := byFolder[folder]
		if pg == nil {
			rd.gap("pages", schemas[folder], "%s: no page this reader writes is served from its folder; not read", schemas[folder])
			continue
		}
		pg.schema = schemas[folder]
		read++
		if err := rd.readSchema(pg); err != nil {
			return err
		}
	}
	if len(rd.pages) == 0 && len(rd.routeFiles) == 0 {
		return refuse("%s holds no file %s serves a page or an operation from whose route the meta-model can hold; extract pages reads a file-system router's root folder, such as app, pages or server", rd.base, rd.routerName())
	}
	rd.res.Lines = append(rd.res.Lines, rd.generated...)
	rd.names()
	rd.write()
	rd.writeRoutes()
	rd.askMiddleware()
	rd.askRouter()
	permissions := 0
	if rd.permissions != nil {
		permissions = len(rd.permissions.Content) / 2
	}
	entities := 0
	if rd.entities != nil {
		entities = len(rd.entities.Content) / 2
	}
	operations := 0
	for _, rf := range rd.routeFiles {
		if rf.key != "" {
			operations++
		}
	}
	rd.res.say("wrote %s, %s, %s, %s and %s: one page per file or folder the router serves a page from whose route the meta-model holds, %s read, one operation per route file whose name gives a method the meta-model holds, one entity and one permission per name a schema gives, one question per thing the files' places and the schemas do not say, and one per thing the meta-model cannot hold", plural(len(rd.pages), "page"), plural(operations, "operation"), plural(entities, "entity"), plural(permissions, "permission"), plural(rd.nextID+couldCount(oneQuestions(rd.questions), rd.held()), "question"), plural(read, "schema"))
	askNotHeld(rd.res, oneQuestions(rd.questions), &rd.nextID, rd.key, rd.held())
	return nil
}

// collectApp takes the App Router's page, schema and route files: one page
// per folder that holds a page file.
func (rd *pageReader) collectApp() ([]pageCandidate, map[string]string) {
	schemas := map[string]string{}
	var pageList, routeList []string
	for _, f := range rd.files() {
		rel := rd.within(f)
		name := path.Base(rel)
		folder := path.Dir(rel)
		switch {
		case pageFiles[name]:
			pageList = append(pageList, f)
		case name == pageSchemaFile:
			schemas[folder] = f
		case routeFiles[name]:
			routeList = append(routeList, f)
		case otherPageFile.MatchString(name):
			rd.gap("pages", f, "%s: a page file the router serves only when its configuration adds the extension, which the folders do not say; left out", f)
		default:
			continue
		}
		rd.markGenerated(f)
	}
	var cands []pageCandidate
	byFolder := map[string]string{}
	for _, f := range pageList {
		folder := path.Dir(rd.within(f))
		if prev := byFolder[folder]; prev != "" {
			rd.gap("pages", f, "%s: %s already serves the page of this folder; left out", f, path.Base(prev))
			continue
		}
		route, words, reason := segmentRoute(folderSegments(folder), "folder", "a page's route", routerApp)
		if reason != "" {
			rd.gap("pages", f, "%s: %s; left out", f, reason)
			continue
		}
		byFolder[folder] = f
		says := fmt.Sprintf("The folder %s holds %s, so the router serves a page at %s.", folder, path.Base(f), route)
		if folder == "." {
			says = fmt.Sprintf("The root folder holds %s, so the router serves a page at %s.", path.Base(f), route)
		}
		cands = append(cands, pageCandidate{file: f, folder: folder, route: route, says: says, words: words})
	}
	for _, f := range routeList {
		folder := path.Dir(rd.within(f))
		route, _, reason := segmentRoute(folderSegments(folder), "folder", "a path", routerApp)
		if reason != "" {
			rd.gap("paths", f, "%s: %s; left out", f, reason)
			continue
		}
		rd.addRouteFile(f, route, "", fmt.Sprintf("The folder %s holds %s, so the router serves %s from it.", folder, path.Base(f), route))
	}
	rd.res.say("counted %s, %s and %s: the tracked files under %s named page.tsx, page.ts, page.jsx or page.js, the route files named route.ts, route.js, route.tsx or route.jsx, and the files named %s beside the pages, each once", plural(len(pageList), "page file"), plural(len(routeList), "route file"), plural(len(schemas), "schema file"), rd.base, pageSchemaFile)
	return cands, schemas
}

func folderSegments(folder string) []string {
	if folder == "." {
		return nil
	}
	return strings.Split(folder, "/")
}

// segmentRoute is the route a router serves from the segments of a file's
// place below its root, the words of a page's name, or why the meta-model
// cannot hold it. noun names a segment in a reason: folder or segment;
// holder what a parameter is a part of: a page's route or a path.
func segmentRoute(segs []string, noun, holder, router string) (string, []string, string) {
	var route, words []string
	seen := map[string]bool{}
	for _, seg := range segs {
		group := strings.HasPrefix(seg, "(") && strings.HasSuffix(seg, ")")
		switch {
		case router == routerApp && strings.HasPrefix(seg, "(."):
			return "", nil, fmt.Sprintf("the folder %s is an intercepting route, which shows another route's page in place; the meta-model has no page for it", seg)
		case group && (router == routerApp || router == routerNuxtPages):
			continue
		case router == routerApp && strings.HasPrefix(seg, "@"):
			return "", nil, fmt.Sprintf("the folder %s is a parallel route, a slot of a layout and not a route; the meta-model has no page for it", seg)
		case router == routerApp && strings.HasPrefix(seg, "_"):
			return "", nil, fmt.Sprintf("the folder %s is private, which the router does not serve", seg)
		case strings.HasPrefix(seg, "[...") || strings.HasPrefix(seg, "[[..."):
			return "", nil, fmt.Sprintf("the %s %s is a catch-all segment, and %s holds one whole parameter per segment", noun, seg, holder)
		case strings.HasPrefix(seg, "[[") && strings.HasSuffix(seg, "]]"):
			return "", nil, fmt.Sprintf("the %s %s is an optional parameter, which serves the route with and without it, and %s holds each parameter it has", noun, seg, holder)
		case strings.HasPrefix(seg, "[") && strings.HasSuffix(seg, "]") && strings.Count(seg, "[") == 1:
			name := seg[1 : len(seg)-1]
			if !paramName.MatchString(name) {
				return "", nil, fmt.Sprintf("the parameter %s is not an identifier", name)
			}
			if seen[name] {
				return "", nil, fmt.Sprintf("the parameter %s appears twice", name)
			}
			seen[name] = true
			route = append(route, "{"+name+"}")
			words = append(words, "by", kebab(name))
		case plainSegment.MatchString(seg) && !strings.ContainsAny(seg, "[]()"):
			route = append(route, seg)
			if w := pageWord(seg); w != "" {
				words = append(words, w)
			}
		case router == routerApp || router == routerNuxtPages:
			return "", nil, fmt.Sprintf("the %s %s is neither a fixed word, a group nor one whole parameter", noun, seg)
		default:
			return "", nil, fmt.Sprintf("the %s %s is neither a fixed word nor one whole parameter", noun, seg)
		}
	}
	return "/" + strings.Join(route, "/"), words, ""
}

var notPageWord = regexp.MustCompile(`[^a-z0-9]+`)

// pageWord is a fixed segment in the words of a kebab-case name.
func pageWord(seg string) string {
	return strings.Trim(notPageWord.ReplaceAllString(kebab(seg), "-"), "-")
}

// names makes the pages' names unique, in the order of their routes.
func (rd *pageReader) names() {
	sort.Slice(rd.pages, func(i, j int) bool { return rd.pages[i].route < rd.pages[j].route })
	taken := map[string]bool{}
	for _, pg := range rd.pages {
		name := pg.name
		for n := 2; taken[name]; n++ {
			name = fmt.Sprintf("%s-%d", pg.name, n)
		}
		if name != pg.name {
			rd.pageGap(pg, "", false, pg.file, "page at %s: its name %s is taken by another route's page; it is named %s", pg.route, pg.name, name)
		}
		taken[name] = true
		pg.name = name
	}
	sort.Slice(rd.pages, func(i, j int) bool { return rd.pages[i].name < rd.pages[j].name })
}

// readSchema takes a page's content from its schema file.
func (rd *pageReader) readSchema(pg *page) error {
	data, err := os.ReadFile(filepath.Join(rd.r.Repository.Root, filepath.FromSlash(pg.schema)))
	if err != nil {
		return err
	}
	o, err := parseSchemaFile(string(data))
	if err != nil {
		rd.pageGap(pg, "kind", true, pg.schema, "%s: outside the subset of TypeScript that is also JSON5 at %v; its content is asked for instead", pg.schema, err)
		return nil
	}
	var unread []string
	for _, k := range o.keys {
		v := o.values[k]
		where := fmt.Sprintf("%s:%d", pg.schema, o.lines[k])
		if !contains(readKeys, k) && !contains(namedKeys, k) {
			unread = append(unread, k)
			continue
		}
		if ref, ok := v.(reference); ok {
			rd.pageGap(pg, k, true, where, "%s: %s is the name %s, not a literal; asked for instead", where, k, ref.name)
			continue
		}
		switch k {
		case "kind", "title", "entity", "permission", "source", "submit":
			s, ok := v.(string)
			if !ok || s == "" {
				rd.pageGap(pg, k, true, where, "%s: %s is not a text; asked for instead", where, k)
				continue
			}
			switch {
			case k == "kind" && !pageKind[s]:
				rd.pageGap(pg, k, true, where, "%s: the kind %q is not list, form, view or task; asked for instead", where, s)
			case k == "entity" && !typeWord.MatchString(s):
				rd.pageGap(pg, k, true, where, "%s: the entity %q is not PascalCase, which an entity's name is; asked for instead", where, s)
			case k == "permission" && !permissionWord.MatchString(s):
				rd.pageGap(pg, k, true, where, "%s: the permission %q is not lower-case words joined by dots; asked for instead", where, s)
			case (k == "source" || k == "submit") && !memberWord.MatchString(s):
				rd.pageGap(pg, k, true, where, "%s: the operation %q is not camelCase, which an operationId is; not named", where, s)
			case k == "kind":
				pg.kind = s
			case k == "title":
				pg.title = s
			case k == "entity":
				pg.entity = s
			case k == "permission":
				pg.permission = s
			default:
				pg.named[k] = s
			}
		case "sections":
			sections, reason := schemaSections(v)
			if reason != "" {
				rd.pageGap(pg, k, true, where, "%s: sections %s; asked for instead", where, reason)
				continue
			}
			pg.sections = sections
		default:
			list, reason := fieldList(v)
			if reason != "" {
				rd.pageGap(pg, k, true, where, "%s: %s %s; asked for instead", where, k, reason)
				continue
			}
			pg.lists[k] = list
		}
		pg.read = append(pg.read, k)
	}
	if len(unread) > 0 {
		rd.pageGap(pg, "", false, pg.schema, "%s: %s %s not read; the reader takes only the page keywords whose values a tree of pages can hold", pg.schema, joinAnd(unread), isOrAre(len(unread)))
	}
	rd.settle(pg)
	return nil
}

// settle drops what a page of its kind cannot hold and what contradicts
// another key, each with a line.
func (rd *pageReader) settle(pg *page) {
	if pg.kind == "task" && pg.entity != "" {
		rd.pageGap(pg, "", false, pg.schema, "%s: entity %s is for a list, a form or a view, and the page is a task, which submits without loading a record; left out", pg.schema, pg.entity)
		pg.entity = ""
		var read []string
		for _, k := range pg.read {
			if k != "entity" {
				read = append(read, k)
			}
		}
		pg.read = read
	}
	for _, k := range []string{"columns", "compactColumns", "filters", "fields", "sections"} {
		has := pg.lists[k] != nil || k == "sections" && pg.sections != nil
		if !has || pg.kind == "" || contains(keyForKinds[k], pg.kind) {
			continue
		}
		rd.pageGap(pg, "", false, pg.schema, "%s: %s is for a %s, and the page is a %s; left out", pg.schema, k, strings.Join(keyForKinds[k], " or a "), pg.kind)
		delete(pg.lists, k)
		if k == "sections" {
			pg.sections = nil
		}
	}
	if pg.lists["fields"] != nil && pg.sections != nil {
		rd.pageGap(pg, "", false, pg.schema, "%s: the page gives both fields and sections, and a page gives its fields once; sections left out", pg.schema)
		pg.sections = nil
	}
	if cc := pg.lists["compactColumns"]; cc != nil {
		var kept, dropped []string
		for _, c := range cc {
			if contains(pg.lists["columns"], c) {
				kept = append(kept, c)
			} else {
				dropped = append(dropped, c)
			}
		}
		if len(dropped) > 0 {
			rd.pageGap(pg, "", false, pg.schema, "%s: compactColumns %s %s not among the page's columns; left out", pg.schema, joinAnd(dropped), isOrAre(len(dropped)))
		}
		if len(kept) == 0 {
			delete(pg.lists, "compactColumns")
		} else {
			pg.lists["compactColumns"] = kept
		}
	}
}

// fieldList reads a list of field names.
func fieldList(v any) ([]string, string) {
	items, ok := v.([]any)
	if !ok {
		return nil, "is not a list"
	}
	var out []string
	for _, it := range items {
		s, ok := it.(string)
		if !ok || !memberWord.MatchString(s) {
			return nil, "holds an item that is not a field's name in camelCase"
		}
		if contains(out, s) {
			return nil, fmt.Sprintf("names %s twice", s)
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, "is empty"
	}
	return out, ""
}

// schemaSections reads sections, each a title and its fields.
func schemaSections(v any) ([][2]any, string) {
	items, ok := v.([]any)
	if !ok || len(items) == 0 {
		return nil, "is not a list of sections"
	}
	var out [][2]any
	var all []string
	for _, it := range items {
		o, ok := it.(*object)
		if !ok || len(o.keys) != 2 {
			return nil, "holds an item that is not a title and its fields"
		}
		title, ok := o.values["title"].(string)
		if !ok || title == "" {
			return nil, "holds a section with no title in text"
		}
		fields, reason := fieldList(o.values["fields"])
		if reason != "" {
			return nil, "holds a section whose fields " + reason
		}
		for _, f := range fields {
			if contains(all, f) {
				return nil, fmt.Sprintf("names %s in two sections", f)
			}
			all = append(all, f)
		}
		out = append(out, [2]any{title, fields})
	}
	return out, ""
}

// shown is every field a page shows, in the order of their names.
func (pg *page) shown() []string {
	var out []string
	for _, k := range fieldLists {
		for _, f := range pg.lists[k] {
			if !contains(out, f) {
				out = append(out, f)
			}
		}
	}
	for _, s := range pg.sections {
		for _, f := range s[1].([]string) {
			if !contains(out, f) {
				out = append(out, f)
			}
		}
	}
	sort.Strings(out)
	return out
}

// write writes the pages, the entities and permissions their schemas
// name, and the questions, in the order of the pages' names.
func (rd *pageReader) write() {
	rd.pageNodes = &yaml.Node{Kind: yaml.MappingNode}
	rd.questions = &yaml.Node{Kind: yaml.MappingNode}
	entityFields := map[string][]string{}
	entityCites := map[string][]*page{}
	permissionCites := map[string][]*page{}
	for _, pg := range rd.pages {
		n := mapping()
		set(n, "kind", nonEmpty(pg.kind))
		set(n, "title", nonEmpty(pg.title))
		set(n, "route", pg.route)
		set(n, "entity", nonEmpty(pg.entity))
		set(n, "permission", nonEmpty(pg.permission))
		for _, k := range []string{"columns", "compactColumns"} {
			if l := pg.lists[k]; l != nil {
				set(n, k, l)
			}
		}
		if l := pg.lists["fields"]; l != nil {
			set(n, "fields", l)
		}
		if pg.sections != nil {
			var secs []*yaml.Node
			for _, s := range pg.sections {
				secs = append(secs, flow(mapping("title", s[0].(string), "fields", s[1].([]string))))
			}
			set(n, "sections", secs)
		}
		if l := pg.lists["filters"]; l != nil {
			set(n, "filters", l)
		}
		set(n, "origin", "stated")
		cites := []*yaml.Node{citation(rd.key, pg.file, pg.says)}
		if pg.schema != "" {
			says := "The page's schema gives none of the page's keys."
			if len(pg.read) > 0 {
				says = fmt.Sprintf("The page's schema gives its %s.", joinAnd(pg.read))
			}
			cites = append(cites, citation(rd.key, pg.schema, says))
		}
		set(n, "cites", cites)
		set(rd.pageNodes, pg.name, n)
		if pg.entity != "" {
			entityCites[pg.entity] = append(entityCites[pg.entity], pg)
			for _, f := range pg.shown() {
				if !contains(entityFields[pg.entity], f) {
					entityFields[pg.entity] = append(entityFields[pg.entity], f)
				}
			}
		}
		if pg.permission != "" {
			permissionCites[pg.permission] = append(permissionCites[pg.permission], pg)
		}
		rd.askContent(pg)
		if pg.permission == "" {
			rd.question(
				fmt.Sprintf("Which permission does the page at %s check, or is it open to everyone (public)?", pg.route),
				[]string{"#/pages/" + pg.name + "/permission"},
				"The folders name a page's route and not who may open it, and a page open to everyone is how an open screen is usually found, so it is asked, never assumed.",
			)
		}
	}
	rd.writeEntities(entityFields, entityCites)
	rd.writePermissions(permissionCites)
}

// askContent asks for what a page needs and neither its folder nor its
// schema gives.
func (rd *pageReader) askContent(pg *page) {
	at := "#/pages/" + pg.name + "/"
	var keys, what []string
	add := func(k, w string) {
		keys = append(keys, at+k)
		what = append(what, w)
	}
	if pg.kind == "" {
		add("kind", "its kind (list, form, view or task)")
	}
	if pg.title == "" {
		add("title", "its title")
	}
	if pg.entity == "" && pg.kind != "task" {
		add("entity", "the entity it shows")
	}
	if (pg.kind == "" || pg.kind == "list" || pg.kind == "view") && pg.named["source"] == "" {
		add("source", "the operation it reads")
	}
	if (pg.kind == "" || pg.kind == "form" || pg.kind == "task") && pg.named["submit"] == "" {
		add("submit", "the operation it submits to")
	}
	if (pg.kind == "" || pg.kind == "list") && pg.lists["columns"] == nil {
		add("columns", "the columns it lists")
	}
	if len(keys) > 0 {
		why := "The folders name a page's route and not what it shows."
		if pg.schema != "" {
			why = "The folders name a page's route, and its schema does not give these."
		}
		rd.question(fmt.Sprintf("For the page at %s, what %s %s?", pg.route, isOrAre(len(what)), joinAnd(what)), keys, why)
	}
	// An operation the schema names is declared by the tree of the router
	// or the interface document, not this one, so the key is left out and
	// its question carries the name for specarch merge to write.
	for _, k := range namedKeys {
		op := pg.named[k]
		if op == "" {
			continue
		}
		role := "reads"
		if k == "submit" {
			role = "submits to"
		}
		rd.namedQuestion(
			fmt.Sprintf("The page at %s %s operation %s, which its schema names; this tree does not declare it, so it waits for the tree that does. Is it that operation?", pg.route, role, op),
			[]string{at + k}, op,
			fmt.Sprintf("The schema names the operation and not its method or path, so the page's %s waits for the tree that declares it; specarch merge writes it once one does.", k),
		)
	}
}

// writeEntities writes each entity a schema names, known by name with the
// fields its pages show, and asks for its primary key and their types.
func (rd *pageReader) writeEntities(fields map[string][]string, cites map[string][]*page) {
	if len(cites) == 0 {
		return
	}
	var names []string
	for n := range cites {
		names = append(names, n)
	}
	sort.Strings(names)
	rd.entities = &yaml.Node{Kind: yaml.MappingNode}
	for _, name := range names {
		fs := fields[name]
		sort.Strings(fs)
		props := &yaml.Node{Kind: yaml.MappingNode}
		at := "#/entities/" + name
		blocks := []string{at + "/primaryKey"}
		for _, f := range fs {
			set(props, f, flow(mapping()))
			blocks = append(blocks, at+"/properties/"+f)
		}
		if len(fs) == 0 {
			blocks = append(blocks, at+"/properties")
		}
		var cs []*yaml.Node
		var routes []string
		for _, pg := range cites[name] {
			cs = append(cs, citation(rd.key, pg.schema, fmt.Sprintf("The page at %s shows %s.", pg.route, name)))
			routes = append(routes, pg.route)
		}
		set(rd.entities, name, mapping("type", "object", "properties", props, "origin", "stated", "cites", cs))
		text := fmt.Sprintf("What is the primary key of %s?", name)
		if len(fs) > 0 {
			text = fmt.Sprintf("What is the primary key of %s, and what does each field the pages show hold: %s?", name, strings.Join(fs, ", "))
		}
		rd.question(text, blocks, fmt.Sprintf("The schema of the page at %s names the entity and the fields it shows, not their types or the entity's key.", joinAnd(routes)))
	}
}

// writePermissions declares each permission a schema names and asks what
// it allows.
func (rd *pageReader) writePermissions(cites map[string][]*page) {
	if len(cites) == 0 {
		return
	}
	var names []string
	for n := range cites {
		names = append(names, n)
	}
	sort.Strings(names)
	rd.permissions = &yaml.Node{Kind: yaml.MappingNode}
	var blocks, grants, asked []string
	for _, n := range names {
		var routes []string
		for _, pg := range cites[n] {
			routes = append(routes, pg.route)
		}
		first := cites[n][0]
		cite := []*yaml.Node{citation(rd.key, first.schema, fmt.Sprintf("The page at %s %s %s.", joinAnd(routes), checkOrChecks(len(routes)), n))}
		if n == "public" {
			// public is the meta-model's own word for open to everyone, so
			// what it allows is known and no role grants it.
			set(rd.permissions, n, mapping("description", "Open to everyone, signed in or not.", "origin", "stated", "cites", cite))
			continue
		}
		set(rd.permissions, n, mapping("origin", "stated", "cites", cite))
		blocks = append(blocks, "#/permissions/"+escapeToken(n)+"/description")
		grants = append(grants, "#/permissions/"+escapeToken(n))
		asked = append(asked, n)
	}
	if len(asked) == 0 {
		return
	}
	names = asked
	rd.question(
		fmt.Sprintf("What does each permission allow: %s?", strings.Join(names, ", ")),
		blocks,
		"A page schema names the permission a page checks and not what it is for.",
	)
	rd.question(
		fmt.Sprintf("Which role grants each permission: %s?", strings.Join(names, ", ")),
		grants,
		"A page schema names the permission a page checks and not who holds it.",
	)
}

func isOrAre(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}
