package genuits

import (
	"embed"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/SpecArch/specarch/internal/genscreens"
	"github.com/SpecArch/specarch/internal/genui"
	"github.com/SpecArch/specarch/internal/ownership"
	"github.com/SpecArch/specarch/internal/wirename"
	"github.com/SpecArch/specarch/internal/xref"
)

// server is what the ui target's settings say of the server between the
// browser and the service: the path its routes answer under, the
// environment variable that says where the service is, how the session's
// token reaches the service, and the lists its routes page.
type server struct {
	routes, service        string
	cookie, header, scheme string
	pages                  map[string]paging
	// services are the environment variables that say where another
	// module is, by the dependency that names it (ADR-184).
	services map[string]string
}

// paging is how a route pages a list its operation answers whole.
type paging struct {
	pageSize, maximum int
}

var (
	environmentName = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	routesPath      = regexp.MustCompile(`^(/[a-z0-9][a-z0-9-]*)+$`)
	headerName      = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")
	cookieName      = headerName
)

// readServer reads settings.server; nil when the settings name none, and
// the screens then call the service directly.
func (g *gen) readServer() *server {
	s, ok := genscreens.Obj(g.Settings["server"])
	if !ok {
		if _, named := g.Settings["server"]; named {
			g.Problem("error", "/", "the ui target's settings.server is not an object; give routes, service and token under it")
		}
		return nil
	}
	at := "/"
	out := &server{routes: text(s["routes"]), service: text(s["service"]), pages: map[string]paging{}, services: map[string]string{}}
	ok = true
	if !routesPath.MatchString(out.routes) {
		g.Problem("error", at, "the ui target's settings.server.routes is %q, and the server routes answer under a path of lower-case segments, such as /api", out.routes)
		ok = false
	}
	switch {
	case !environmentName.MatchString(out.service):
		g.Problem("error", at, "the ui target's settings.server.service is %q, and it names the environment variable that says where the service is, such as SERVICE_URL", out.service)
		ok = false
	case strings.HasPrefix(out.service, "NEXT_PUBLIC_"):
		g.Problem("error", at, "the ui target's settings.server.service is %s, and Next.js writes a NEXT_PUBLIC_ variable into the pages it sends the browser; name one the server alone reads", out.service)
		ok = false
	}
	token, _ := genscreens.Obj(s["token"])
	out.cookie, out.header, out.scheme = text(token["cookie"]), text(token["header"]), text(token["scheme"])
	if !cookieName.MatchString(out.cookie) || !headerName.MatchString(out.header) {
		g.Problem("error", at, "the ui target's settings.server.token names no cookie and header, the cookie that holds the session's token and the header that carries it to the service; give both, such as { cookie: session, header: Authorization, scheme: Bearer }")
		ok = false
	}
	if out.scheme != "" && !headerName.MatchString(out.scheme) {
		g.Problem("error", at, "the ui target's settings.server.token.scheme is %q, which is not one word, such as Bearer", out.scheme)
		ok = false
	}
	pages := obj0(s["pages"])
	for _, id := range sortedKeys(pages) {
		p := obj0(pages[id])
		size, err1 := strconv.Atoi(text(p["pageSize"]))
		maximum, err2 := strconv.Atoi(text(p["maximum"]))
		if err1 != nil || err2 != nil || size < 1 || size > maximum {
			g.Problem("error", at, "the ui target's settings.server.pages gives %s no pageSize and maximum, whole numbers with the page size at most the maximum, such as { pageSize: 20, maximum: 100 }", id)
			continue
		}
		out.pages[id] = paging{pageSize: size, maximum: maximum}
	}
	if _, named := s["services"]; named && obj0(s["services"]) == nil {
		g.Problem("error", at, "the ui target's settings.server.services is not an object; give each dependency whose module a page reads the environment variable that says where it is, such as { customerRegistry: CUSTOMER_REGISTRY_URL }")
		ok = false
	}
	deps := obj0(g.Spec["dependencies"])
	services := obj0(s["services"])
	for _, dep := range sortedKeys(services) {
		v := text(services[dep])
		switch {
		case text(obj0(deps[dep])["specification"]) == "":
			g.Problem("error", at, "the ui target's settings.server.services names %s, which is not a dependency of the specification that names another specification; name the dependency a page reads through", dep)
			ok = false
		case !environmentName.MatchString(v):
			g.Problem("error", at, "the ui target's settings.server.services gives %s %q, and it names the environment variable that says where that module is, such as %s_URL", dep, v, strings.ToUpper(wirename.Snake(dep)))
			ok = false
		case strings.HasPrefix(v, "NEXT_PUBLIC_"):
			g.Problem("error", at, "the ui target's settings.server.services gives %s %s, and Next.js writes a NEXT_PUBLIC_ variable into the pages it sends the browser; name one the server alone reads", dep, v)
			ok = false
		default:
			out.services[dep] = v
		}
	}
	if !ok {
		return nil
	}
	pagesOf := obj0(g.Spec["pages"])
	for _, pageName := range sortedKeys(pagesOf) {
		route := text(obj0(pagesOf[pageName])["route"])
		if route == out.routes || strings.HasPrefix(route, out.routes+"/") {
			g.Problem("error", "/pages/"+pageName+"/route", "%s is at %s, under %s, where the server routes answer; move the page, or the routes in the ui target's settings.server", pageName, route, out.routes)
		}
	}
	return out
}

// reach is where the screens call another module's operation: under the
// server routes, after the module's name, whose route forwards it to the
// address the environment variable settings.server.services names for the
// dependency (ADR-184).
func (g *gen) reach(dep string, d xref.Dependency) (string, string) {
	switch {
	case g.server == nil:
		return "", fmt.Sprintf("the screens call one service, at NEXT_PUBLIC_API_URL; add settings.server, whose routes forward a call to %s to the address the environment variable settings.server.services.%s names", d.Specification, dep)
	case g.server.services[dep] == "":
		return "", fmt.Sprintf("the ui target's settings.server.services names no environment variable for %s, the address of %s; add one, such as %s: %s_URL", dep, d.Specification, dep, strings.ToUpper(wirename.Snake(dep)))
	}
	return "/" + d.Specification, ""
}

// checkPaged refuses an operation the settings say a route pages that is
// not one a route can page: a GET whose 200 answer is an array, read by a
// list, that does not page itself.
func (g *gen) checkPaged(srv *server, listed map[string]bool) {
	if srv == nil {
		return
	}
	for _, id := range sortedKeys(anyPaging(srv.pages)) {
		op, _, method := g.Operation(id)
		schema := obj0(obj0(obj0(obj0(obj0(obj0(op["responses"])["200"])["content"])["application/json"])["schema"]))
		switch {
		case op == nil || method != "get":
			g.Problem("error", "/", "the ui target's settings.server.pages names %s, which is not a GET operation of the specification", id)
		case op["listOf"] != nil:
			g.Problem("error", "/", "the ui target's settings.server.pages names %s, which pages itself through its listOf; take it out of the settings", id)
		case text(schema["type"]) != "array":
			g.Problem("error", "/", "the ui target's settings.server.pages names %s, whose 200 answer is not an array a route can page", id)
		case !listed[id]:
			g.Problem("warning", "/", "the ui target's settings.server.pages names %s, which no list page reads; its route forwards it whole", id)
		}
	}
}

func anyPaging(m map[string]paging) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// serverFiles are the server routes, one route.ts per path a page calls,
// with a handler per method that forwards it, its settings, and their
// derived test. An operation another stakeholder owns gets no handler.
func (g *gen) serverFiles(srv *server, header string) []genui.File {
	owned := ownership.Of(g.Impl.Content)
	var files []genui.File
	slugs := map[string]string{} // a dynamic segment's folder, by the folder above it
	headers := map[string]bool{"accept": true, "content-type": true}
	paths := make([]string, 0, len(g.Calls))
	for p := range g.Calls {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		var handlers strings.Builder
		for _, m := range []string{"get", "post", "put", "patch", "delete"} {
			id, ok := g.Calls[p][m]
			if !ok || owned.Covers(ownership.Operation(p, m)) {
				continue
			}
			op, _, _ := g.Operation(id)
			if key := text(op["idempotencyKey"]); key != "" {
				headers[strings.ToLower(key)] = true
			}
			method := strings.ToUpper(m)
			if pg, ok := srv.pages[id]; ok && m == "get" {
				fmt.Fprintf(&handlers, "\n/** %s: %s %s, read whole from the service and answered a page at a time. */\n", id, method, p)
				fmt.Fprintf(&handlers, "export function %s(request: Request): Promise<Response> {\n  return paged(request, server, { pageSize: %d, maximum: %d });\n}\n", method, pg.pageSize, pg.maximum)
				continue
			}
			fmt.Fprintf(&handlers, "\n/** %s: %s %s, forwarded to the service with the session's token. */\n", id, method, p)
			fmt.Fprintf(&handlers, "export function %s(request: Request): Promise<Response> {\n  return forward(request, server);\n}\n", method)
		}
		if handlers.Len() == 0 {
			continue
		}
		folder := "app" + srv.routes
		for _, segment := range strings.Split(strings.Trim(p, "/"), "/") {
			if pathParam.MatchString(segment) {
				slug := routeFolder(segment)
				if other, ok := slugs[folder]; ok && other != slug {
					g.Problem("error", "/paths/"+ownership.Escape(p), "%s takes %s where another path a page calls takes %s, and Next.js routes one folder by one name; name the parameter alike in both", p, segment, other)
				}
				slugs[folder] = slug
				segment = slug
			}
			folder += "/" + segment
		}
		files = append(files, genui.File{Path: folder + "/route.ts", Content: header + routeImports(handlers.String()) + handlers.String()})
	}
	// A call to another module goes to a route under the module's name,
	// which forwards it to that module's address (ADR-184).
	var services []member
	for _, dep := range sortedKeys(anyRemote(g.Remote)) {
		d := g.Deps[dep]
		for _, p := range paths {
			if p == "/"+d.Specification || strings.HasPrefix(p, "/"+d.Specification+"/") {
				g.Problem("error", "/paths/"+ownership.Escape(p), "%s is under /%s, where the server routes forward the calls to the module %s; move the path, or the route", p, d.Specification, d.Specification)
			}
		}
		services = append(services, member{Key: dep, Val: raw("{ ...server, routes: server.routes + " + quoteTS("/"+d.Specification) + ", service: process.env." + srv.services[dep] + " ?? \"\" }")})
		for _, p := range sortedKeys(anyCalls(g.Remote[dep])) {
			var handlers strings.Builder
			for _, m := range []string{"get", "post", "put", "patch", "delete"} {
				id, ok := g.Remote[dep][p][m]
				if !ok {
					continue
				}
				fmt.Fprintf(&handlers, "\n/** %s of %s: %s %s, forwarded to that module with the session's token. */\n", id, d.Specification, strings.ToUpper(m), p)
				fmt.Fprintf(&handlers, "export function %s(request: Request): Promise<Response> {\n  return forward(request, services.%s);\n}\n", strings.ToUpper(m), dep)
			}
			folder := "app" + srv.routes + "/" + d.Specification
			for _, segment := range strings.Split(strings.Trim(p, "/"), "/") {
				folder += "/" + segment
			}
			imports := "import { forward } from \"@/server/forward\";\nimport { services } from \"@/server/settings\";\n"
			files = append(files, genui.File{Path: folder + "/route.ts", Content: header + imports + handlers.String()})
		}
	}
	var names []value
	for _, h := range sortedKeys(anyBool(headers)) {
		names = append(names, str(h))
	}
	wire := g.ListNames()
	settings := object([]member{
		{Key: "routes", Val: str(srv.routes)},
		{Key: "service", Val: raw("process.env." + srv.service + " ?? \"\"")},
		{Key: "cookie", Val: str(srv.cookie)},
		{Key: "header", Val: str(srv.header)},
		{Key: "scheme", Val: str(srv.scheme)},
		{Key: "headers", Val: array(names)},
		{Key: "wire", Val: object([]member{
			{Key: "page", Val: str(wire["page"])}, {Key: "pageSize", Val: str(wire["pageSize"])},
			{Key: "items", Val: str(wire["items"])}, {Key: "totalItems", Val: str(wire["totalItems"])}, {Key: "totalPages", Val: str(wire["totalPages"])},
		})},
	})
	var b strings.Builder
	b.WriteString("import type { ServerSettings } from \"./forward\";\n\n")
	b.WriteString("/**\n * Where the server routes answer and forward, how they carry the session's\n * token to the service, and the request headers they send on.\n */\n")
	b.WriteString(statement("export const server: ServerSettings = ", settings, ";"))
	if len(services) > 0 {
		b.WriteString("\n/** Where the server routes forward a call to another module, by the dependency that names it. */\n")
		b.WriteString(statement("export const services: Record<string, ServerSettings> = ", object(services), ";"))
	}
	data, _ := serverTemplates.ReadFile("server/forward.ts")
	files = append(files,
		genui.File{Path: "server/forward.ts", Content: header + string(data)},
		genui.File{Path: "server/settings.ts", Content: header + b.String()},
		genui.File{Path: "tests/server.test.ts", Content: header + serverTest},
	)
	return files
}

// routeImports imports what a route's handlers call.
func routeImports(handlers string) string {
	var called []string
	for _, f := range []string{"forward", "paged"} {
		if strings.Contains(handlers, "return "+f+"(") {
			called = append(called, f)
		}
	}
	return "import { " + strings.Join(called, ", ") + " } from \"@/server/forward\";\nimport { server } from \"@/server/settings\";\n"
}

func anyRemote(m map[string]map[string]map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func anyCalls(m map[string]map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// quoteTS is a string as a TypeScript literal.
func quoteTS(s string) string {
	return strconv.Quote(s)
}

func anyBool(m map[string]bool) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

//go:embed server
var serverTemplates embed.FS

// serverTest is the derived test of the server routes, with the service
// stood in for by a stub of fetch: a route sends the session's token and
// no cookie, answers what the service answers, pages a list it reads whole
// and refuses a page size above the maximum.
const serverTest = `import assert from "node:assert/strict";
import { afterEach, test } from "node:test";
import { forward, paged } from "../server/forward.ts";
import { server } from "../server/settings.ts";

const settings = { ...server, service: "http://service.test/v1" };
const realFetch = globalThis.fetch;
const sent: Request[] = [];

function serve(answer: () => Response): void {
  globalThis.fetch = async (input: string | URL | Request, init?: RequestInit) => {
    sent.push(new Request(input, init));
    return answer();
  };
}

afterEach(() => {
  globalThis.fetch = realFetch;
  sent.length = 0;
});

function at(value: unknown, path: string): unknown {
  return path.split(".").reduce<unknown>((into, name) => (typeof into === "object" && into !== null ? (into as Record<string, unknown>)[name] : undefined), value);
}

test("a route forwards to the service with the session's token and no cookie", async () => {
  serve(() => new Response(JSON.stringify({ ok: true }), { status: 201, headers: { "content-type": "application/json", "set-cookie": "kept=1; Path=/; HttpOnly" } }));
  const request = new Request("http://app.test" + server.routes + "/things/7?q=a%20b", {
    method: "POST",
    headers: { cookie: "other=1; " + server.cookie + "=the-token", "content-type": "application/json", "x-other": "no" },
    body: JSON.stringify({ name: "x" }),
  });
  const answer = await forward(request, settings);
  assert.equal(sent.length, 1);
  assert.equal(sent[0].url, "http://service.test/v1/things/7?q=a+b");
  assert.equal(sent[0].method, "POST");
  assert.equal(sent[0].headers.get(server.header), server.scheme === "" ? "the-token" : server.scheme + " the-token");
  assert.equal(sent[0].headers.get("cookie"), null);
  assert.equal(sent[0].headers.get("x-other"), null);
  assert.deepEqual(await sent[0].json(), { name: "x" });
  assert.equal(answer.status, 201);
  assert.deepEqual(await answer.json(), { ok: true });
  assert.deepEqual(answer.headers.getSetCookie(), ["kept=1; Path=/; HttpOnly"]);
});

test("a route sends no token to someone without the session's cookie", async () => {
  serve(() => new Response(null, { status: 204 }));
  const answer = await forward(new Request("http://app.test" + server.routes + "/things"), settings);
  assert.equal(sent[0].headers.get(server.header), null);
  assert.equal(answer.status, 204);
});

test("a route answers 502 when the service cannot be reached", async () => {
  globalThis.fetch = async () => {
    throw new TypeError("unreachable");
  };
  assert.equal((await forward(new Request("http://app.test" + server.routes + "/things"), settings)).status, 502);
  assert.equal((await forward(new Request("http://app.test" + server.routes + "/things"), { ...settings, service: "" })).status, 502);
});

test("a route pages a list the service answers whole", async () => {
  serve(() => Response.json(Array.from({ length: 45 }, (_, i) => ({ n: i }))));
  const query = "?" + server.wire.page + "=2&" + server.wire.pageSize + "=20&status=open";
  const answer = await paged(new Request("http://app.test" + server.routes + "/things" + query), settings, { pageSize: 20, maximum: 50 });
  assert.equal(sent[0].url, "http://service.test/v1/things?status=open");
  const body: unknown = await answer.json();
  assert.deepEqual(at(body, server.wire.items), Array.from({ length: 20 }, (_, i) => ({ n: 20 + i })));
  assert.equal(at(body, server.wire.totalItems), 45);
  assert.equal(at(body, server.wire.totalPages), 3);
});

test("a route refuses a page size above the maximum without calling the service", async () => {
  serve(() => Response.json([]));
  const query = "?" + server.wire.pageSize + "=51";
  const answer = await paged(new Request("http://app.test" + server.routes + "/things" + query), settings, { pageSize: 20, maximum: 50 });
  assert.equal(answer.status, 400);
  assert.equal(sent.length, 0);
});
`

// pagedByServer is how a server route pages an operation's list, when the
// settings say one does.
func (g *gen) pagedByServer(id string) (string, string, bool) {
	if g.server == nil {
		return "", "", false
	}
	p, ok := g.server.pages[id]
	return strconv.Itoa(p.pageSize), strconv.Itoa(p.maximum), ok
}

// serviceValue is where the screens call the service: the server routes,
// or the service itself at NEXT_PUBLIC_API_URL.
func (g *gen) serviceValue() value {
	if g.server != nil {
		return str(g.server.routes)
	}
	return raw(`process.env.NEXT_PUBLIC_API_URL ?? ""`)
}
