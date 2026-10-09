package extract

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The file-system routers extract pages reads, told apart by the name of
// the folder it is given and, for pages, by the framework the package
// above it names (ADR-084).
const (
	routerApp        = "app"
	routerNextPages  = "next-pages"
	routerNuxtPages  = "nuxt-pages"
	routerNuxtServer = "nuxt-server"
)

var (
	// The extensions Next.js serves a page or an API route from by default
	// (its pageExtensions), and those Nuxt serves a page from.
	nextPageExtensions = map[string]bool{".tsx": true, ".ts": true, ".jsx": true, ".js": true}
	nuxtPageExtensions = map[string]bool{".vue": true, ".tsx": true, ".ts": true, ".jsx": true, ".js": true, ".mjs": true}
	// The extensions Nuxt's server (Nitro) reads a handler from, and the
	// methods a handler's file name can end in.
	serverExtensions = map[string]bool{".ts": true, ".js": true, ".mjs": true}
	serverMethods    = map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true, "head": true, "options": true, "connect": true, "trace": true}
	// Next.js runs middleware.ts, which it names proxy.ts from version 16
	// on, from the folder that holds app or pages.
	nextMiddlewareFiles  = []string{"middleware.js", "middleware.ts", "proxy.js", "proxy.ts"}
	nuxtGlobalMiddleware = regexp.MustCompile(`^[^/]+\.global\.(ts|js|mjs)$`)
)

// routeFile is a file a router serves an operation from: by its name when
// the name ends in the method, otherwise every method the handler takes.
type routeFile struct {
	file, path, says string
	method           string // in capitals, or "" when the name does not give it
	key              string // the path item's method key, or ""
	params           []string
	handler          *nextHandler // what the code exports for the method, when a dump is read
	action           *jsFact      // the server action a page's form submits to, or nil
	actionAt         string       // where the form is
}

func (rd *pageReader) routerName() string {
	switch rd.router {
	case routerNextPages:
		return "the Pages Router of Next.js"
	case routerNuxtPages:
		return "the pages of Nuxt"
	case routerNuxtServer:
		return "the server routes of Nuxt (Nitro)"
	}
	return "the App Router of Next.js"
}

// files is the tracked files under the root folder, without the files
// beside it the reader also read.
func (rd *pageReader) files() []string {
	var out []string
	for _, f := range rd.r.Files {
		if rd.base == "." || strings.HasPrefix(f, rd.base+"/") {
			out = append(out, f)
		}
	}
	return out
}

// markGenerated keeps a line for a file read that says it is generated.
func (rd *pageReader) markGenerated(f string) {
	if line, text, ok := generatedMark(filepath.Join(rd.r.Repository.Root, filepath.FromSlash(f))); ok {
		rd.generated = append(rd.generated, fmt.Sprintf("generated: %s:%d says it is generated from another source: %s", f, line, text))
	}
}

// trackedAt is the tracked files among or under the paths, from the
// repository's root.
func (rd *pageReader) trackedAt(paths ...string) ([]string, error) {
	out, err := git(rd.r.Repository.Root, append([]string{"ls-files", "-z", "--"}, paths...)...)
	if err != nil {
		return nil, err
	}
	var files []string
	for f := range strings.SplitSeq(out, "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	return files, nil
}

// beside tells which router serves the folder, and finds the files beside
// it that the reader reads by their place: the package that names the
// framework and the middleware the router runs.
func (rd *pageReader) beside() ([]string, error) {
	parent := path.Dir(rd.base)
	switch path.Base(rd.base) {
	case "server":
		rd.router = routerNuxtServer
		rd.routerWhy = "the folder is named server"
		return nil, nil
	case "pages":
		var err error
		if err = rd.pagesFramework(parent); err != nil {
			return nil, err
		}
	default:
		rd.router = routerApp
		rd.routerWhy = "the folder is named neither pages nor server"
	}
	var err error
	if rd.router == routerNuxtPages {
		found, err := rd.trackedAt(path.Join(parent, "middleware"))
		if err != nil {
			return nil, err
		}
		for _, f := range found {
			if nuxtGlobalMiddleware.MatchString(strings.TrimPrefix(f, path.Join(parent, "middleware")+"/")) {
				rd.middleware = append(rd.middleware, f)
			}
		}
	} else {
		var candidates []string
		for _, name := range nextMiddlewareFiles {
			candidates = append(candidates, path.Join(parent, name))
		}
		if rd.middleware, err = rd.trackedAt(candidates...); err != nil {
			return nil, err
		}
	}
	beside := append([]string{}, rd.middleware...)
	if rd.packageFile != "" {
		beside = append(beside, rd.packageFile)
	}
	return beside, nil
}

// pagesFramework tells a pages folder of Next.js from one of Nuxt by the
// dependencies of the nearest tracked package.json at or above the folder
// that holds it; when that names neither or both, a folder with .vue files
// is Nuxt's, and the guess is asked.
func (rd *pageReader) pagesFramework(dir string) error {
	for {
		p := path.Join(dir, "package.json")
		found, err := rd.trackedAt(p)
		if err != nil {
			return err
		}
		if len(found) > 0 {
			rd.packageFile = p
			break
		}
		if dir == "." || dir == "/" {
			break
		}
		dir = path.Dir(dir)
	}
	var names string
	if rd.packageFile != "" {
		var pkg struct {
			Dependencies    map[string]json.RawMessage `json:"dependencies"`
			DevDependencies map[string]json.RawMessage `json:"devDependencies"`
		}
		data, err := os.ReadFile(filepath.Join(rd.r.Repository.Root, filepath.FromSlash(rd.packageFile)))
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &pkg); err != nil {
			names = rd.packageFile + " is not JSON"
		} else {
			_, next := pkg.Dependencies["next"]
			_, devNext := pkg.DevDependencies["next"]
			_, nuxt := pkg.Dependencies["nuxt"]
			_, devNuxt := pkg.DevDependencies["nuxt"]
			switch {
			case (next || devNext) && !(nuxt || devNuxt):
				rd.router = routerNextPages
				rd.routerWhy = rd.packageFile + " names next among its dependencies"
				return nil
			case (nuxt || devNuxt) && !(next || devNext):
				rd.router = routerNuxtPages
				rd.routerWhy = rd.packageFile + " names nuxt among its dependencies"
				return nil
			case next || devNext:
				names = rd.packageFile + " names both next and nuxt among its dependencies"
			default:
				names = rd.packageFile + " names neither next nor nuxt among its dependencies"
			}
		}
	} else {
		names = "no tracked package.json at or above " + rd.base + " names the framework"
	}
	vue := false
	for _, f := range rd.files() {
		if path.Ext(f) == ".vue" {
			vue = true
			break
		}
	}
	if vue {
		rd.router = routerNuxtPages
		rd.routerWhy = names + "; the folder holds .vue files, which only Nuxt serves"
	} else {
		rd.router = routerNextPages
		rd.routerWhy = names + "; the folder holds no .vue file, so it is read as Next.js's"
	}
	rd.routerAsk = names
	return nil
}

// collectNextPages takes the Pages Router's files: one page per file, and
// the files under api route files.
func (rd *pageReader) collectNextPages() []pageCandidate {
	var cands []pageCandidate
	pages, routes := 0, 0
	for _, f := range rd.files() {
		rel := rd.within(f)
		ext := path.Ext(rel)
		if !nextPageExtensions[ext] {
			rd.gap("pages", f, "%s: a file the router serves only when its configuration adds the extension %s, which the files do not say; left out", f, ext)
			continue
		}
		rd.markGenerated(f)
		segs := strings.Split(strings.TrimSuffix(rel, ext), "/")
		if segs[len(segs)-1] == "index" {
			segs = segs[:len(segs)-1]
		}
		if strings.HasPrefix(rel, "api/") {
			routes++
			route, _, reason := segmentRoute(segs, "segment", "a path", routerNextPages)
			if reason != "" {
				rd.gap("paths", f, "%s: %s; left out", f, reason)
				continue
			}
			rd.addRouteFile(f, route, "", fmt.Sprintf("The file %s is under %s/api, so the router serves the API route %s from it.", rel, rd.base, route))
			continue
		}
		pages++
		if len(segs) == 1 {
			switch segs[0] {
			case "_app", "_document":
				rd.gap("pages", f, "%s: the router wraps every page in it, and it is no page of its own; left out", f)
				continue
			case "_error", "404", "500":
				rd.gap("pages", f, "%s: an error page the router shows in place of another route's page; the meta-model has no page for it; left out", f)
				continue
			}
		}
		route, words, reason := segmentRoute(segs, "segment", "a page's route", routerNextPages)
		if reason != "" {
			rd.gap("pages", f, "%s: %s; left out", f, reason)
			continue
		}
		cands = append(cands, pageCandidate{file: f, folder: rel, route: route, words: words, says: fmt.Sprintf("The file %s is under %s, so the router serves a page at %s.", rel, rd.base, route)})
	}
	rd.res.say("counted %s and %s: the tracked files under %s with the extension .tsx, .ts, .jsx or .js, those under %s/api API routes, each once", plural(pages, "page file"), plural(routes, "API route file"), rd.base, rd.base)
	return cands
}

// collectNuxtPages takes Nuxt's pages: one page per file, index the
// folder's own route, a folder in parentheses a group.
func (rd *pageReader) collectNuxtPages() []pageCandidate {
	files := rd.files()
	folders := map[string]bool{}
	indexed := map[string]bool{}
	for _, f := range files {
		rel := rd.within(f)
		for d := path.Dir(rel); d != "."; d = path.Dir(d) {
			folders[d] = true
		}
		if nuxtPageExtensions[path.Ext(rel)] && strings.TrimSuffix(path.Base(rel), path.Ext(rel)) == "index" {
			indexed[path.Dir(rel)] = true
		}
	}
	var cands []pageCandidate
	pages := 0
	for _, f := range files {
		rel := rd.within(f)
		ext := path.Ext(rel)
		if !nuxtPageExtensions[ext] {
			rd.gap("pages", f, "%s: a file the router serves only when its configuration adds the extension %s, which the files do not say; left out", f, ext)
			continue
		}
		pages++
		rd.markGenerated(f)
		stem := strings.TrimSuffix(rel, ext)
		if folders[stem] && indexed[stem] {
			rd.gap("pages", f, "%s: it wraps the pages of the folder %s, a nested route whose index page is the page at its route; the meta-model has no page that holds others; left out", f, stem)
			continue
		}
		segs := strings.Split(stem, "/")
		if segs[len(segs)-1] == "index" {
			segs = segs[:len(segs)-1]
		}
		route, words, reason := segmentRoute(segs, "segment", "a page's route", routerNuxtPages)
		if reason != "" {
			rd.gap("pages", f, "%s: %s; left out", f, reason)
			continue
		}
		cands = append(cands, pageCandidate{file: f, folder: rel, route: route, words: words, says: fmt.Sprintf("The file %s is under %s, so the router serves a page at %s.", rel, rd.base, route)})
	}
	rd.res.say("counted %s: the tracked files under %s with the extension .vue, .tsx, .ts, .jsx, .js or .mjs, each once", plural(pages, "page file"), rd.base)
	return cands
}

// collectNuxtServer takes the route files under server/api, served under
// /api, and server/routes, served at the root, and the middleware under
// server/middleware.
func (rd *pageReader) collectNuxtServer() {
	routes := 0
	for _, f := range rd.files() {
		rel := rd.within(f)
		segs := strings.Split(rel, "/")
		var prefix []string
		switch segs[0] {
		case "api":
			prefix = []string{"api"}
		case "routes":
		case "middleware":
			if serverExtensions[path.Ext(rel)] {
				rd.middleware = append(rd.middleware, f)
			}
			continue
		default:
			continue
		}
		if len(segs) == 1 {
			continue
		}
		ext := path.Ext(rel)
		if !serverExtensions[ext] {
			rd.gap("paths", f, "%s: a file the server serves a route from only when its configuration adds the extension %s, which the files do not say; left out", f, ext)
			continue
		}
		routes++
		rd.markGenerated(f)
		name := strings.TrimSuffix(path.Base(rel), ext)
		method := ""
		if i := strings.LastIndex(name, "."); i > 0 && serverMethods[name[i+1:]] {
			method = strings.ToUpper(name[i+1:])
			name = name[:i]
		}
		rest := append(append([]string{}, segs[1:len(segs)-1]...), name)
		if name == "index" {
			rest = rest[:len(rest)-1]
		}
		route, _, reason := segmentRoute(append(prefix, rest...), "segment", "a path", routerNuxtServer)
		if reason != "" {
			rd.gap("paths", f, "%s: %s; left out", f, reason)
			continue
		}
		if method != "" && !heldMethods[method] {
			rd.gap("paths", f, "%s: the method %s has no operation in the meta-model, which holds GET, POST, PUT, PATCH and DELETE; left out", f, method)
			continue
		}
		says := fmt.Sprintf("The file %s is under %s, so the server serves %s from it.", rel, rd.base, route)
		if method != "" {
			says = fmt.Sprintf("The file %s is under %s and its name ends in the method, so the server serves %s %s from it.", rel, rd.base, method, route)
		}
		rd.addRouteFile(f, route, method, says)
	}
	rd.res.say("counted %s and %s: the tracked files under %s/api and %s/routes, and under %s/middleware, with the extension .ts, .js or .mjs, each once", plural(routes, "route file"), plural(len(rd.middleware), "middleware file"), rd.base, rd.base, rd.base)
}

func (rd *pageReader) addRouteFile(f, route, method, says string) {
	params, _ := pathParameters(route)
	rf := &routeFile{file: f, path: route, method: method, says: says, params: params}
	if method != "" {
		rf.key = strings.ToLower(method)
	}
	rd.routeFiles = append(rd.routeFiles, rf)
}

// settleRouteFiles leaves out a route file on a page's route, which the
// App Router refuses, and a second file for one method and path, then
// orders them by path and method.
func (rd *pageReader) settleRouteFiles(pages map[string]*page) {
	var kept []*routeFile
	seen := map[string]*routeFile{}
	for _, rf := range rd.routeFiles {
		if pg := pages[rf.path]; pg != nil && rd.router == routerApp {
			rd.gap("paths", rf.file, "%s: %s serves a page on the same route, and the router refuses a page and a route handler on one route; left out", rf.file, pg.file)
			continue
		}
		pair := rf.method + " " + rf.path
		if prev := seen[pair]; prev != nil {
			rd.gap("paths", rf.file, "%s: %s already serves %s, and a route has one handler; left out", rf.file, prev.file, strings.TrimSpace(pair))
			continue
		}
		seen[pair] = rf
		kept = append(kept, rf)
	}
	sort.SliceStable(kept, func(i, j int) bool {
		if kept[i].path != kept[j].path {
			return kept[i].path < kept[j].path
		}
		return routeRank(kept[i]) < routeRank(kept[j])
	})
	rd.routeFiles = kept
}

// routeRank orders a path's route files by method, a file that serves
// every method last.
func routeRank(rf *routeFile) int {
	if rf.key == "" {
		return 99
	}
	return methodRank(rf.key)
}

// citedQuestion asks a must question citing the file it is about.
func (rd *pageReader) citedQuestion(text string, blocks []string, why, file, says string) {
	rd.nextID++
	set(rd.questions, fmt.Sprintf("Q-%d", rd.nextID), mapping(
		"question", text,
		"kind", "decision",
		"priority", "must",
		"blocks", blocks,
		"decidedBy", owner,
		"why", why,
		"cites", []*yaml.Node{citation(rd.key, file, says)},
	))
}

// writeRoutes writes an operation for each route file whose name gives
// its method, and asks which methods every other route file serves.
func (rd *pageReader) writeRoutes() {
	var paths []string
	byPath := map[string][]*routeFile{}
	for _, rf := range rd.routeFiles {
		if rf.key == "" {
			continue
		}
		if byPath[rf.path] == nil {
			paths = append(paths, rf.path)
		}
		byPath[rf.path] = append(byPath[rf.path], rf)
	}
	if len(paths) > 0 {
		rd.paths = &yaml.Node{Kind: yaml.MappingNode}
	}
	rd.opPermissions = map[string]string{}
	used := map[string][]string{}
	permCites := map[string]*yaml.Node{}
	defer func() { rd.writeNextPermissions(used, permCites) }()
	for _, p := range paths {
		item := mapping()
		at := "#/paths/" + escapeToken(p)
		files := byPath[p]
		if params := files[0].params; len(params) > 0 {
			var list []*yaml.Node
			var blocks []string
			for i, name := range params {
				list = append(list, flow(mapping("name", name, "in", "path", "required", true)))
				blocks = append(blocks, fmt.Sprintf("%s/parameters/%d/schema", at, i))
			}
			set(item, "parameters", list)
			text := fmt.Sprintf("What values does each path parameter of %s take: %s?", p, strings.Join(params, ", "))
			if values, sat, why := rd.staticParams(files[0].file); sat != "" && why == "" {
				text += fmt.Sprintf(" generateStaticParams at %s gives %s, the values it is built for.", sat, describeParams(values))
			}
			rd.question(text, blocks, "A route file's place names a path's parameters and not the values they take.")
		}
		for _, rf := range files {
			opAt := at + "/" + rf.key
			label := rf.method + " " + rf.path
			opID := methodPathName(rf.key, rf.path)
			if rf.action != nil {
				opID = strings.ToLower(rf.action.Name[:1]) + rf.action.Name[1:]
			}
			op := mapping("operationId", opID)
			cites := []*yaml.Node{citation(rd.key, rf.file, rf.says)}
			permission, unguarded := "", ""
			if rd.js != nil && (rf.handler != nil || rf.action != nil) {
				h := nextHandler{}
				if rf.handler != nil {
					h = *rf.handler
					cites = append(cites, rd.js.at(h.at, "Exports the handler of "+label+"."))
				} else {
					h = nextHandler{fn: rf.action, at: rd.js.clause(rf.action)}
					cites = append(cites, rd.js.at(h.at, "The server action "+rf.action.Name+"."), rd.js.at(rf.actionAt, "The form submits to "+rf.action.Name+"."))
				}
				permission, unguarded = rd.handlerPermission(h, label)
				if permission != "" {
					cites = append(cites, rd.js.at(h.at, fmt.Sprintf("Its check names %s.", permission)))
				} else if mp, mw := rd.middlewarePermission(rf.path); mp != "" {
					permission = mp
					cites = append(cites, rd.js.at(mw.at, fmt.Sprintf("The middleware %s checks %s before %s.", mw.file, mp, label)))
				}
				if permission != "" {
					set(op, "permission", permission)
					rd.opPermissions[opAt] = permission
					if used[permission] == nil {
						permCites[permission] = rd.js.at(h.at, fmt.Sprintf("%s checks %s.", label, permission))
					}
					used[permission] = append(used[permission], label)
				}
				if rf.key == "post" || rf.key == "put" || rf.key == "patch" {
					if name := rd.handlerBody(h.fn); name != "" {
						set(op, "requestBody", mapping("required", true, "content", mapping("application/json", flow(mapping("schema", flow(mapping("$ref", "#/schemas/"+name)))))))
					}
				}
			}
			set(op, "origin", "stated")
			set(op, "cites", cites)
			set(item, rf.key, op)
			rd.question(
				fmt.Sprintf("What does %s do, and what does it answer?", label),
				[]string{opAt + "/summary", opAt + "/responses"},
				"A route file's place names the method and the path, and not what the handler does or the responses it gives.",
			)
			if rf.action != nil {
				rd.question(
					fmt.Sprintf("The server action %s has no path of its own: Next.js posts it to the page that shows its form, %s, naming the action in a header. Is %s the path it is called at, and is it called from anywhere else?", rf.action.Name, rf.path, label),
					[]string{opAt},
					"A server action is a function, and the router gives it no route; the path is where the page that calls it is served, which is what a client sees.",
				)
			}
			if permission == "" && !rd.middlewareAsks(rf.path) {
				asked := fmt.Sprintf("%s checks no permission that its file's place names.", label)
				if unguarded != "" {
					asked = unguarded
				}
				rd.question(
					asked+" Is it meant to be open to everyone (public), or which permission should it check?",
					[]string{opAt + "/permission"},
					"An operation open to everyone is how an open endpoint is usually found, so it is asked, never assumed.",
				)
			}
		}
		set(rd.paths, p, item)
	}
	for _, rf := range rd.routeFiles {
		if rf.key != "" {
			continue
		}
		text := fmt.Sprintf("Which methods does %s serve at %s, and for each, what does it do, what does it answer and which permission does it check?", rf.file, rf.path)
		if len(rf.params) > 0 {
			text += fmt.Sprintf(" Its path parameters are %s.", joinAnd(rf.params))
		}
		why := "A route file's place gives its path and not its methods, which the handlers it exports decide; an operation is a method and a path, so none is written until the JavaScript reader reads the handlers or the owner says."
		if rd.js != nil && rd.js.files[rf.file] != nil {
			why = "A route file's place gives its path and not its methods; its code exports no handler named after a method, or its handler compares req.method with no literal, so the methods are not known by syntax."
			if rf.handler != nil {
				text += fmt.Sprintf(" Its handler is at %s.", rf.handler.at)
				rd.js.cite(rf.handler.at)
			}
		}
		rd.citedQuestion(text, []string{"paths"}, why, rf.file, rf.says)
	}
}

// askMiddleware asks, for each middleware file the router runs, what it
// covers and checks: its place says it runs, and not where or what.
func (rd *pageReader) askMiddleware() {
	if len(rd.middleware) == 0 {
		return
	}
	var blocks []string
	if rd.router != routerNuxtServer {
		for _, pg := range rd.pages {
			blocks = append(blocks, "#/pages/"+pg.name+"/permission")
		}
	}
	if rd.router != routerNuxtPages {
		unknown := false
		for _, rf := range rd.routeFiles {
			if rf.key == "" {
				unknown = true
				continue
			}
			blocks = append(blocks, "#/paths/"+escapeToken(rf.path)+"/"+rf.key+"/permission")
		}
		if unknown {
			blocks = append(blocks, "paths")
		}
	}
	if len(blocks) == 0 {
		return
	}
	for _, f := range rd.middleware {
		if mw := rd.middlewareRead(f); mw != nil {
			rd.askReadMiddleware(mw)
			continue
		}
		var text string
		switch rd.router {
		case routerNuxtPages:
			text = fmt.Sprintf("%s is global route middleware, which runs before every page is shown and may check who is signed in or send them elsewhere. What does it check on each page?", f)
		case routerNuxtServer:
			text = fmt.Sprintf("%s is server middleware, which runs before every route the server serves and may check who is signed in or answer by itself. Which routes does it act on, and what does it check on them?", f)
		default:
			text = fmt.Sprintf("%s runs before every route its matcher selects, pages and route handlers alike, and may check who is signed in, send them elsewhere or answer by itself. Which pages and operations does it cover, and what does it check on them?", f)
		}
		rd.citedQuestion(text, blocks,
			"Its place says the router runs it, and not which routes it acts on or what it checks; a check that runs before every route is how a permission is usually given, so it is asked, never assumed, until the JavaScript reader reads it.",
			f, "Its place says the router runs it before the routes it covers.")
	}
}

// askRouter asks which framework serves a pages folder when the package
// above it does not say.
func (rd *pageReader) askRouter() {
	if rd.routerAsk == "" {
		return
	}
	blocks := []string{"pages"}
	if len(rd.routeFiles) > 0 {
		blocks = append(blocks, "paths")
	}
	text := fmt.Sprintf("Which framework serves the folder %s, Next.js or Nuxt? The reader read it as %s, since %s.", rd.base, rd.routerName(), rd.routerWhy)
	why := "Next.js and Nuxt both serve pages from a folder named pages, by different rules, and the package that names the framework does not say which; the reader's choice is a guess, so it is asked."
	if rd.packageFile != "" {
		rd.citedQuestion(text, blocks, why, rd.packageFile, "It does not name exactly one of next and nuxt among its dependencies.")
		return
	}
	rd.question(text, blocks, why)
}

// expandRouteFiles turns each Next.js route file whose code the dump
// holds into one route file per method its handlers serve, and adds the
// server actions pages' forms submit to.
func (rd *pageReader) expandRouteFiles(pages map[string]*page) {
	if rd.router != routerApp && rd.router != routerNextPages {
		return
	}
	var out []*routeFile
	for _, rf := range rd.routeFiles {
		if rf.key != "" {
			out = append(out, rf)
			continue
		}
		hs := rd.nextHandlers(rf)
		switch {
		case len(hs) == 0:
			out = append(out, rf)
			continue
		case len(hs) == 1 && hs[0].method == "":
			c := *rf
			c.handler = &hs[0]
			out = append(out, &c)
			continue
		}
		if hs[0].others != "" {
			var ms []string
			for _, h := range hs {
				ms = append(ms, h.method)
			}
			rd.js.question("must", fmt.Sprintf("The handler of %s at %s serves %s, and %s. Which other methods does it serve, and what does each do?", rf.path, hs[0].at, joinAnd(ms), hs[0].others),
				[]string{"paths"}, "An API route's handler serves every method the request comes with; the reader reads the methods it names by a literal, and not what the rest of its code does with the others.", rd.js.at(hs[0].at, "The handler of "+rf.path+"."))
		}
		for i := range hs {
			h := hs[i]
			if !heldMethods[h.method] {
				rd.gap("paths", h.at, "%s: %s serves %s %s, a method the meta-model holds no operation for, which holds GET, POST, PUT, PATCH and DELETE; left out", h.at, rf.file, h.method, rf.path)
				rd.js.cite(h.at)
				continue
			}
			c := *rf
			c.method, c.key, c.handler = h.method, strings.ToLower(h.method), &h
			out = append(out, &c)
		}
	}
	seen := map[string]*routeFile{}
	for _, rf := range out {
		if rf.key != "" {
			seen[rf.method+" "+rf.path] = rf
		}
	}
	for _, a := range rd.serverActions(pages) {
		pair := a.method + " " + a.path
		if prev := seen[pair]; prev != nil {
			what := prev.file
			if prev.action != nil {
				what = "the server action " + prev.action.Name
			}
			rd.js.question("must", fmt.Sprintf("The server action %s, which the form at %s submits to, would be %s, which %s already serves. Which path is it called at?", a.action.Name, a.actionAt, pair, what),
				[]string{"paths"}, "A server action has no path of its own, and two operations cannot share a method and a path.", rd.js.at(a.actionAt, "Submits to "+a.action.Name+"."))
			continue
		}
		seen[pair] = a
		out = append(out, a)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].path != out[j].path {
			return out[i].path < out[j].path
		}
		return routeRank(out[i]) < routeRank(out[j])
	})
	rd.routeFiles = out
}

// middlewareRead is what the dump says of a middleware file, or nil.
func (rd *pageReader) middlewareRead(file string) *nextMiddleware {
	for _, mw := range rd.mw {
		if mw.file == file {
			return mw
		}
	}
	return nil
}

// askReadMiddleware asks about the pages and operations a middleware's
// code may cover and gives no permission: those its matcher may select,
// where its check names none, or one that is not a literal.
func (rd *pageReader) askReadMiddleware(mw *nextMiddleware) {
	var blocks, routes []string
	consider := func(route, at string, has bool) {
		sure, may := mw.covers(route)
		if !may || has || (sure && mw.permission != "") {
			return
		}
		if !contains(blocks, at) {
			blocks = append(blocks, at)
		}
		if !contains(routes, route) {
			routes = append(routes, route)
		}
	}
	if rd.router != routerNuxtServer {
		for _, pg := range rd.pages {
			consider(pg.route, "#/pages/"+pg.name+"/permission", pg.permission != "")
		}
	}
	for _, rf := range rd.routeFiles {
		if rf.key == "" {
			consider(rf.path, "paths", false)
			continue
		}
		at := "#/paths/" + escapeToken(rf.path) + "/" + rf.key + "/permission"
		consider(rf.path, at, rd.opPermissions[at[:len(at)-len("/permission")]] != "")
	}
	if len(blocks) == 0 {
		return
	}
	var why string
	switch {
	case mw.unknown != "":
		why = fmt.Sprintf("The reader cannot tell which routes %s covers, since %s.", mw.file, mw.unknown)
	case mw.permission == "" && mw.checks > 0:
		why = fmt.Sprintf("%s calls a check the implementation file names with no one literal permission.", mw.file)
	case mw.permission == "" && rd.js.checkFile == "":
		why = fmt.Sprintf("No implementation file names the project's checks, so the reader knows none that %s calls.", mw.file)
	case mw.permission == "":
		why = fmt.Sprintf("%s calls no check the implementation file names.", mw.file)
	default:
		why = fmt.Sprintf("%s checks %s, and its matcher may or may not select these routes, since a route's parameter can take a matcher's literal segment.", mw.file, mw.permission)
	}
	rd.citedQuestion(fmt.Sprintf("%s runs before %s and may check who is signed in, send them elsewhere or answer by itself. %s Which permission does each of them check, or is it open to everyone (public)?", mw.file, joinAnd(routes), why), blocks,
		"A check that runs before every route it covers is how a permission is usually given, so what the code does not say is asked, never assumed.",
		mw.file, "The router runs it before the routes its matcher selects.")
}

// middlewareAsks says whether a read middleware may cover a route and
// gives it no permission, so that its question asks for the route's
// permission in the place of the route's own.
func (rd *pageReader) middlewareAsks(route string) bool {
	for _, mw := range rd.mw {
		sure, may := mw.covers(route)
		if may && !(sure && mw.permission != "") {
			return true
		}
	}
	return false
}
