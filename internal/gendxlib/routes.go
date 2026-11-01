package gendxlib

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The fixed routes a service answers beside its operations (ADR-216):
// the browser application's document, the files under a folder, the
// health of the service. dxlib serves an operation as a POST with a JSON
// body, so each route is a raw handler on the API's mux, with what the
// design fixes written here, the method, the permission, the caching and
// the content security policy with a nonce made for each response, and
// what only the service has, the document, the folder and the health
// check, supplied by it in a Routes value.

// route is one fixed route as the generator reads it.
type route struct {
	name, field, path, serves, permission string
	node                                  map[string]any
}

// routes lists the fixed routes in the order of their names.
func (g *gen) routes() []route {
	var out []route
	rs := obj(g.spec["routes"])
	for _, name := range sortedKeys(rs) {
		r := obj(rs[name])
		out = append(out, route{name: name, field: pascal(name), path: text(r["path"]), serves: text(r["serves"]), permission: text(r["permission"]), node: r})
	}
	return out
}

// fixedRoutes writes the Routes type the service fills, RegisterRoutes,
// which mounts every route, and the handlers of the three kinds.
func (g *gen) fixedRoutes() {
	rs := g.routes()
	if len(rs) == 0 {
		return
	}
	for _, r := range rs {
		if r.field == "Allowed" {
			g.problem("error", "/routes/"+r.name, "route %s would be the field Allowed of the Routes type, which holds the permission check; give the route another name", r.name)
			return
		}
	}
	for _, i := range []string{"context", "crypto/rand", "encoding/base64", "errors", "fmt", "io", "io/fs", "net/http", "strconv", "strings"} {
		g.imports[i] = true
	}
	g.imports["github.com/donnyhardyanto/dxlib/api"] = true
	g.line("// Routes holds what the service supplies for its fixed routes: the")
	g.line("// document of each application route, built with the nonce of the")
	g.line("// response; the folder of each files route; the check of each health")
	g.line("// route; and Allowed, which says whether the request's caller holds a")
	g.line("// permission, called for every route whose permission is not public.")
	g.line("type Routes struct {")
	for _, r := range rs {
		g.w.WriteString(g.marks("/routes/"+r.name, "\t"))
		switch r.serves {
		case "application":
			g.line("\t// %s is the document of route %s, GET %s: the HTML with the nonce on every script and style element.", r.field, r.name, r.path)
			g.line("\t%s func(nonce string) ([]byte, error)", r.field)
		case "files":
			g.line("\t// %s is the folder of route %s, GET %s.", r.field, r.name, r.path)
			g.line("\t%s http.FileSystem", r.field)
		case "health":
			g.line("\t// %s is the check of route %s, GET %s: nil while the service is healthy, else why it is not.", r.field, r.name, r.path)
			g.line("\t%s func(ctx context.Context) error", r.field)
		}
	}
	g.line("\t// Allowed reports whether the request's caller holds the permission. An error")
	g.line("\t// is answered with its text: 401 when it is or wraps ErrNotSignedIn, the caller")
	g.line("\t// not signed in or the session expired, and 403 otherwise; a route whose")
	g.line("\t// permission is signedIn answers 401 for every error.")
	g.line("\tAllowed func(r *http.Request, permission string) error")
	g.line("}")
	g.line("")
	g.line("// RegisterRoutes mounts the fixed routes on the API's mux beside the")
	g.line("// operations: GET and HEAD alone, each with its permission, its caching")
	g.line("// and, for the application's document, its content security policy")
	g.line("// with a nonce made for the response.")
	g.line("func RegisterRoutes(a *api.DXAPI, r Routes) {")
	for _, r := range rs {
		caching := strconv.Quote(cacheControl(r))
		switch r.serves {
		case "application":
			g.line("\ta.RegisterRawHandler(%q, routeApplication(r, %q, %q, %s, %s, r.%s))", r.path, r.name, r.permission, caching, policyLiteral(r.node), r.field)
		case "files":
			g.line("\ta.RegisterRawHandler(%q, routeFiles(r, %q, %q, %s, %q, r.%s))", r.path, r.name, r.permission, caching, r.path, r.field)
		case "health":
			g.line("\ta.RegisterRawHandler(%q, routeHealth(r, %q, %q, %s, r.%s))", r.path, r.name, r.permission, caching, r.field)
		}
	}
	g.line("}")
	g.line("")
	g.w.WriteString(routeHelpers)
}

// cacheControl is the Cache-Control a route answers with: no-store, or
// max-age with public or private by the permission and immutable when the
// design says so (RFC 9111, RFC 8246).
func cacheControl(r route) string {
	caching := obj(r.node["caching"])
	if text(caching["store"]) != "true" {
		return "no-store"
	}
	scope := "private"
	if r.permission == "public" {
		scope = "public"
	}
	out := scope + ", max-age=" + text(caching["maxAgeSeconds"])
	if text(caching["immutable"]) == "true" {
		out += ", immutable"
	}
	return out
}

// policyLiteral is the Go literal of a route's content security policy,
// one string per directive, with {nonce} where the nonce of the response
// goes.
func policyLiteral(node map[string]any) string {
	csp := obj(node["contentSecurityPolicy"])
	if len(csp) == 0 {
		return "nil"
	}
	var directives []string
	for _, name := range sortedKeys(csp) {
		var values []string
		for _, v := range list(csp[name]) {
			if text(v) == "nonce" {
				values = append(values, "'nonce-{nonce}'")
				continue
			}
			values = append(values, text(v))
		}
		directives = append(directives, strconv.Quote(strings.TrimSpace(name+" "+strings.Join(values, " "))))
	}
	sort.Strings(directives)
	return fmt.Sprintf("[]string{%s}", strings.Join(directives, ", "))
}

// routeHelpers are the handlers of the three kinds of route and the guard
// they share, written once when the design has a route.
const routeHelpers = `// ErrNotSignedIn is what Routes.Allowed returns, or wraps, for a caller
// who is not signed in or whose session expired: the route answers 401,
// where any other error is 403.
var ErrNotSignedIn = errors.New("not signed in")

// routeGuard answers what every fixed route refuses: a method other than
// GET or HEAD, 405; a caller not signed in, 401; a caller without the
// route's permission, 403.
func routeGuard(r Routes, name, permission string, w http.ResponseWriter, req *http.Request) bool {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, fmt.Sprintf("route %s answers GET and HEAD, not %s", name, req.Method), http.StatusMethodNotAllowed)
		return false
	}
	if permission == "public" {
		return true
	}
	if r.Allowed == nil {
		http.Error(w, fmt.Sprintf("route %s needs permission %s, and Routes.Allowed is nil, so nothing can check it; set Routes.Allowed", name, permission), http.StatusInternalServerError)
		return false
	}
	if err := r.Allowed(req, permission); err != nil {
		status := http.StatusForbidden
		if permission == "signedIn" || errors.Is(err, ErrNotSignedIn) {
			status = http.StatusUnauthorized
		}
		http.Error(w, fmt.Sprintf("route %s needs permission %s: %v", name, permission, err), status)
		return false
	}
	return true
}

// routeApplication serves the application's document: a nonce of 128
// random bits is made for the response, named in the policy and handed to
// the document, which puts it on every script and style element.
func routeApplication(r Routes, name, permission, caching string, policy []string, document func(nonce string) ([]byte, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !routeGuard(r, name, permission, w, req) {
			return
		}
		if document == nil {
			http.Error(w, fmt.Sprintf("route %s has no document: its field of Routes is nil; set it to the function that builds the HTML", name), http.StatusInternalServerError)
			return
		}
		raw := make([]byte, 16)
		if _, err := rand.Read(raw); err != nil {
			http.Error(w, fmt.Sprintf("route %s could not make a nonce: %v", name, err), http.StatusInternalServerError)
			return
		}
		nonce := base64.StdEncoding.EncodeToString(raw)
		body, err := document(nonce)
		if err != nil {
			http.Error(w, fmt.Sprintf("route %s could not build its document: %v", name, err), http.StatusInternalServerError)
			return
		}
		h := w.Header()
		if len(policy) > 0 {
			h.Set("Content-Security-Policy", strings.ReplaceAll(strings.Join(policy, "; "), "{nonce}", nonce))
		}
		h.Set("Cache-Control", caching)
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
		if req.Method != http.MethodHead {
			_, _ = w.Write(body)
		}
	})
}

// routeFiles serves the files of a folder under a prefix: a file is
// answered with its content type and the route's caching, a folder or a
// missing file 404, and a path that climbs out of the folder is cleaned by
// the server before any file is read.
func routeFiles(r Routes, name, permission, caching, prefix string, folder http.FileSystem) http.Handler {
	var server http.Handler
	if folder != nil {
		server = http.StripPrefix(prefix, http.FileServer(routeFolder{folder}))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !routeGuard(r, name, permission, w, req) {
			return
		}
		if server == nil {
			http.Error(w, fmt.Sprintf("route %s has no folder: its field of Routes is nil; set it to the file system of the files", name), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", caching)
		server.ServeHTTP(w, req)
	})
}

// routeFolder opens files alone: a folder under it is not found, so
// nothing is ever listed.
type routeFolder struct{ files http.FileSystem }

func (f routeFolder) Open(name string) (http.File, error) {
	file, err := f.files.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if info.IsDir() {
		_ = file.Close()
		return nil, fs.ErrNotExist
	}
	return file, nil
}

// routeHealth answers 200 while the check passes and 503 with the reason
// while it does not, as text that is never stored.
func routeHealth(r Routes, name, permission, caching string, check func(ctx context.Context) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !routeGuard(r, name, permission, w, req) {
			return
		}
		if check == nil {
			http.Error(w, fmt.Sprintf("route %s has no check: its field of Routes is nil; set it to the function that says whether the service is healthy", name), http.StatusInternalServerError)
			return
		}
		h := w.Header()
		h.Set("Cache-Control", caching)
		h.Set("Content-Type", "text/plain; charset=utf-8")
		if err := check(req.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			if req.Method != http.MethodHead {
				_, _ = fmt.Fprintf(w, "route %s: the service is not healthy: %v\n", name, err)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		if req.Method != http.MethodHead {
			_, _ = io.WriteString(w, "healthy\n")
		}
	})
}

`
