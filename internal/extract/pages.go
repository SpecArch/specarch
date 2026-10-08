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
	pageKind   = map[string]bool{"list": true, "form": true, "view": true}
)

// The schema keys read as the page keywords of the same name, and the two
// named in a question, in the order the page writes them.
var (
	readKeys    = []string{"kind", "title", "entity", "permission", "columns", "compactColumns", "fields", "sections", "filters"}
	namedKeys   = []string{"source", "submit"}
	fieldLists  = []string{"columns", "compactColumns", "fields", "filters"}
	keyForKinds = map[string][]string{
		"columns": {"list"}, "compactColumns": {"list"}, "filters": {"list"},
		"fields": {"form", "view"}, "sections": {"form", "view"},
	}
)

// page is one page the reader writes.
type page struct {
	name, route, folder string
	file                string // the page file, from the repository's root
	schema              string // the schema file, or ""
	kind, title         string
	entity, permission  string
	lists               map[string][]string // columns, compactColumns, fields, filters
	sections            [][2]any            // title, fields
	named               map[string]string   // source and submit the schema names
	read                []string            // the keys the schema gave, in the order written
}

// Pages reads a file-system router's root folder into one page per folder
// that holds a page file, with the content of each page that has a schema
// file from it.
func Pages(root, out, key string) (*Result, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, refuse("%s is not a folder; extract pages reads a file-system router's root folder, such as app", root)
	}
	r, err := Open([]string{root})
	if err != nil {
		return nil, err
	}
	rd := &pageReader{r: r, key: key, res: &Result{Tree: newTree()}, base: r.Paths[0]}
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
	sort.Slice(clauses, func(i, j int) bool {
		return source0(clauses[i]) < source0(clauses[j])
	})
	src, err := codeSource(r, out, clauses)
	if err != nil {
		return nil, err
	}
	design := mapping()
	if rd.entities != nil {
		set(design, "entities", rd.entities)
	}
	if rd.permissions != nil {
		set(design, "permissions", rd.permissions)
	}
	set(design, "pages", rd.pageNodes)
	res := rd.res
	res.Tree.put("design/pages.yaml", design)
	res.Tree.put("requirements/stakeholders.yaml", mapping("stakeholders", ownerStakeholder()))
	res.Tree.put("design/questions.yaml", mapping("questions", rd.questions))
	description := fmt.Sprintf("The pages the file-system router rooted at %s serves, read from the folders git tracks there at commit %s, with the content of each page that has a schema file from that file. Every page cites its page file; what the folders and the schemas do not say is a question.\n", rd.base, r.Commit)
	res.Tree.put("specarch.yaml", rootFile("Pages of "+rd.base, description, []string{"requirements", "design"}, mapping(key, src)))
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
	notHeld     []string
}

func (rd *pageReader) gap(format string, args ...any) {
	rd.notHeld = append(rd.notHeld, "not held: "+fmt.Sprintf(format, args...))
}

func (rd *pageReader) question(text string, blocks []string, why string) {
	rd.nextID++
	set(rd.questions, fmt.Sprintf("Q-%d", rd.nextID), mapping(
		"question", text,
		"kind", "decision",
		"priority", "must",
		"blocks", blocks,
		"decidedBy", owner,
		"why", why,
	))
}

// within is a file's path below the root folder.
func (rd *pageReader) within(f string) string {
	if rd.base == "." {
		return f
	}
	return strings.TrimPrefix(f, rd.base+"/")
}

func (rd *pageReader) read() error {
	schemas := map[string]string{} // folder below the root -> schema file
	var pageList []string
	var generated []string
	for _, f := range rd.r.Files {
		rel := rd.within(f)
		name := path.Base(rel)
		folder := path.Dir(rel)
		switch {
		case pageFiles[name]:
			pageList = append(pageList, f)
		case name == pageSchemaFile:
			schemas[folder] = f
		case routeFiles[name]:
			rd.gap("%s: a route handler serves an operation, which the router reader reads, not a page; left out", f)
		case otherPageFile.MatchString(name):
			rd.gap("%s: a page file the router serves only when its configuration adds the extension, which the folders do not say; left out", f)
		default:
			continue
		}
		if line, text, ok := generatedMark(filepath.Join(rd.r.Repository.Root, filepath.FromSlash(f))); ok {
			generated = append(generated, fmt.Sprintf("generated: %s:%d says it is generated from another source: %s", f, line, text))
		}
	}
	byRoute := map[string]*page{}
	byFolder := map[string]*page{}
	for _, f := range pageList {
		folder := path.Dir(rd.within(f))
		if prev := byFolder[folder]; prev != nil {
			rd.gap("%s: %s already serves the page of this folder; left out", f, path.Base(prev.file))
			continue
		}
		route, nameWords, reason := folderRoute(folder)
		if reason != "" {
			rd.gap("%s: %s; left out", f, reason)
			continue
		}
		if prev := byRoute[route]; prev != nil {
			return refuse("%s and %s both give the route %s; a file-system router refuses two pages on one route", prev.file, f, route)
		}
		pg := &page{route: route, folder: folder, file: f, lists: map[string][]string{}, named: map[string]string{}}
		pg.name = strings.Join(nameWords, "-")
		if pg.name == "" {
			pg.name = "root"
		}
		if pg.name[0] < 'a' || pg.name[0] > 'z' {
			pg.name = "page-" + pg.name
		}
		byRoute[route] = pg
		byFolder[folder] = pg
		rd.pages = append(rd.pages, pg)
	}
	schemaFolders := make([]string, 0, len(schemas))
	for folder := range schemas {
		schemaFolders = append(schemaFolders, folder)
	}
	sort.Strings(schemaFolders)
	read := 0
	for _, folder := range schemaFolders {
		pg := byFolder[folder]
		if pg == nil {
			rd.gap("%s: no page this reader writes is served from its folder; not read", schemas[folder])
			continue
		}
		pg.schema = schemas[folder]
		read++
		if err := rd.readSchema(pg); err != nil {
			return err
		}
	}
	if len(rd.pages) == 0 {
		return refuse("%s holds no page file the meta-model can hold (page.tsx, page.ts, page.jsx or page.js in a folder whose route it can say); extract pages reads a file-system router's root folder, such as app", rd.base)
	}
	rd.res.say("counted %s and %s: the tracked files under %s named page.tsx, page.ts, page.jsx or page.js, and the files named %s beside them, each once", plural(len(pageList), "page file"), plural(len(schemas), "schema file"), rd.base, pageSchemaFile)
	rd.res.Lines = append(rd.res.Lines, generated...)
	rd.names()
	rd.write()
	permissions := 0
	if rd.permissions != nil {
		permissions = len(rd.permissions.Content) / 2
	}
	entities := 0
	if rd.entities != nil {
		entities = len(rd.entities.Content) / 2
	}
	rd.res.say("wrote %s, %s, %s and %s: one page per folder with a page file whose route the meta-model holds, %s read, one entity and one permission per name a schema gives, and one question per thing the folders and the schemas do not say", plural(len(rd.pages), "page"), plural(entities, "entity"), plural(permissions, "permission"), plural(rd.nextID, "question"), plural(read, "schema"))
	rd.res.Lines = append(rd.res.Lines, rd.notHeld...)
	return nil
}

// folderRoute is the route a folder below the root serves and the words of
// the page's name, or why the meta-model cannot hold it.
func folderRoute(folder string) (string, []string, string) {
	if folder == "." {
		return "/", nil, ""
	}
	var route, words []string
	seen := map[string]bool{}
	for _, seg := range strings.Split(folder, "/") {
		switch {
		case strings.HasPrefix(seg, "(."):
			return "", nil, fmt.Sprintf("the folder %s is an intercepting route, which shows another route's page in place; the meta-model has no page for it", seg)
		case strings.HasPrefix(seg, "(") && strings.HasSuffix(seg, ")"):
			continue
		case strings.HasPrefix(seg, "@"):
			return "", nil, fmt.Sprintf("the folder %s is a parallel route, a slot of a layout and not a route; the meta-model has no page for it", seg)
		case strings.HasPrefix(seg, "_"):
			return "", nil, fmt.Sprintf("the folder %s is private, which the router does not serve", seg)
		case strings.HasPrefix(seg, "[...") || strings.HasPrefix(seg, "[[..."):
			return "", nil, fmt.Sprintf("the folder %s is a catch-all segment, and a page's route holds one whole parameter per segment", seg)
		case strings.HasPrefix(seg, "[") && strings.HasSuffix(seg, "]"):
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
		default:
			return "", nil, fmt.Sprintf("the folder %s is neither a fixed word, a group nor one whole parameter", seg)
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
			rd.gap("page at %s: its name %s is taken by another route's page; it is named %s", pg.route, pg.name, name)
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
		rd.gap("%s: outside the subset of TypeScript that is also JSON5 at %v; its content is asked for instead", pg.schema, err)
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
			rd.gap("%s: %s is the name %s, not a literal; asked for instead", where, k, ref.name)
			continue
		}
		switch k {
		case "kind", "title", "entity", "permission", "source", "submit":
			s, ok := v.(string)
			if !ok || s == "" {
				rd.gap("%s: %s is not a text; asked for instead", where, k)
				continue
			}
			switch {
			case k == "kind" && !pageKind[s]:
				rd.gap("%s: the kind %q is not list, form or view; asked for instead", where, s)
			case k == "entity" && !typeWord.MatchString(s):
				rd.gap("%s: the entity %q is not PascalCase, which an entity's name is; asked for instead", where, s)
			case k == "permission" && !permissionWord.MatchString(s):
				rd.gap("%s: the permission %q is not lower-case words joined by dots; asked for instead", where, s)
			case (k == "source" || k == "submit") && !memberWord.MatchString(s):
				rd.gap("%s: the operation %q is not camelCase, which an operationId is; not named", where, s)
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
				rd.gap("%s: sections %s; asked for instead", where, reason)
				continue
			}
			pg.sections = sections
		default:
			list, reason := fieldList(v)
			if reason != "" {
				rd.gap("%s: %s %s; asked for instead", where, k, reason)
				continue
			}
			pg.lists[k] = list
		}
		pg.read = append(pg.read, k)
	}
	if len(unread) > 0 {
		rd.gap("%s: %s %s not read; the reader takes only the page keywords whose values a tree of pages can hold", pg.schema, joinAnd(unread), isOrAre(len(unread)))
	}
	rd.settle(pg)
	return nil
}

// settle drops what a page of its kind cannot hold and what contradicts
// another key, each with a line.
func (rd *pageReader) settle(pg *page) {
	for _, k := range []string{"columns", "compactColumns", "filters", "fields", "sections"} {
		has := pg.lists[k] != nil || k == "sections" && pg.sections != nil
		if !has || pg.kind == "" || contains(keyForKinds[k], pg.kind) {
			continue
		}
		rd.gap("%s: %s is for a %s, and the page is a %s; left out", pg.schema, k, strings.Join(keyForKinds[k], " or a "), pg.kind)
		delete(pg.lists, k)
		if k == "sections" {
			pg.sections = nil
		}
	}
	if pg.lists["fields"] != nil && pg.sections != nil {
		rd.gap("%s: the page gives both fields and sections, and a page gives its fields once; sections left out", pg.schema)
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
			rd.gap("%s: compactColumns %s %s not among the page's columns; left out", pg.schema, joinAnd(dropped), isOrAre(len(dropped)))
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
		folder := "The folder " + pg.folder
		if pg.folder == "." {
			folder = "The root folder"
		}
		cites := []*yaml.Node{citation(rd.key, pg.file, fmt.Sprintf("%s holds %s, so the router serves a page at %s.", folder, path.Base(pg.file), pg.route))}
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
		if pg.permission != "" && pg.permission != "public" {
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
		add("kind", "its kind (list, form or view)")
	}
	if pg.title == "" {
		add("title", "its title")
	}
	if pg.entity == "" {
		add("entity", "the entity it shows")
	}
	if pg.kind == "" || pg.kind == "list" || pg.kind == "view" {
		add("source", "the operation it reads")
	}
	if pg.kind == "" || pg.kind == "form" {
		add("submit", "the operation it submits to")
	}
	if (pg.kind == "" || pg.kind == "list") && pg.lists["columns"] == nil {
		add("columns", "the columns it lists")
	}
	if len(keys) == 0 {
		return
	}
	text := fmt.Sprintf("For the page at %s, what %s %s?", pg.route, isOrAre(len(what)), joinAnd(what))
	for _, k := range namedKeys {
		if op := pg.named[k]; op != "" {
			role := "the operation it reads"
			if k == "submit" {
				role = "the operation it submits to"
			}
			text += fmt.Sprintf(" Its schema names %s as %s, which only the router's tree holds.", op, role)
		}
	}
	why := "The folders name a page's route and not what it shows."
	if pg.schema != "" {
		why = "The folders name a page's route, and its schema does not give these, or names operations this tree does not hold."
	}
	rd.question(text, keys, why)
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
	var blocks, grants []string
	for _, n := range names {
		var routes []string
		for _, pg := range cites[n] {
			routes = append(routes, pg.route)
		}
		first := cites[n][0]
		set(rd.permissions, n, mapping(
			"origin", "stated",
			"cites", []*yaml.Node{citation(rd.key, first.schema, fmt.Sprintf("The page at %s %s %s.", joinAnd(routes), checkOrChecks(len(routes)), n))},
		))
		blocks = append(blocks, "#/permissions/"+escapeToken(n)+"/description")
		grants = append(grants, "#/permissions/"+escapeToken(n))
	}
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
