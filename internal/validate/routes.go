package validate

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// The fixed routes a server answers beside its operations (ADR-216): the
// browser application's document, the files under a folder, the health
// of the service. GET and HEAD alone, at one path or under one prefix.

// cspDirectives are the directives Content Security Policy Level 3
// defines, by name.
var cspDirectives = map[string]bool{
	"child-src": true, "connect-src": true, "default-src": true, "font-src": true, "frame-src": true,
	"img-src": true, "manifest-src": true, "media-src": true, "object-src": true, "script-src": true,
	"script-src-elem": true, "script-src-attr": true, "style-src": true, "style-src-elem": true,
	"style-src-attr": true, "worker-src": true, "base-uri": true, "sandbox": true, "form-action": true,
	"frame-ancestors": true, "report-to": true, "require-trusted-types-for": true, "trusted-types": true,
	"upgrade-insecure-requests": true,
}

// cspNonceDirectives are the directives a nonce is read in: a nonce
// matches a script or a style element, and default-src stands in for
// their directives when they are absent.
var cspNonceDirectives = map[string]bool{"default-src": true, "script-src": true, "script-src-elem": true, "style-src": true, "style-src-elem": true}

// cspKeywords are the keyword sources CSP Level 3 writes in single quotes;
// written without them, each names a host.
var cspKeywords = map[string]bool{
	"self": true, "none": true, "unsafe-inline": true, "unsafe-eval": true, "strict-dynamic": true,
	"unsafe-hashes": true, "report-sample": true, "wasm-unsafe-eval": true, "inline-speculation-rules": true,
}

// cspNonce is the word that stands for the nonce the server makes for
// each response.
const cspNonce = "nonce"

// checkRoutes checks the fixed routes: a path of the shape its kind
// needs, one route per path and no path an operation answers, a content
// security policy on an application route alone and well formed, a
// caching that says one thing, a nonce never stored, and every page's
// route served by an application route once one is declared.
func (c *checker) checkRoutes(d *design) {
	routes := source.Pairs(source.Child(d.root, "routes"))
	if len(routes) == 0 {
		return
	}
	// A key an open must question asks for is not an error while it waits
	// for the answer, as a reader writes it (docs/principles.md).
	held := blockedPointers(d.root)
	waits := func(parent, key string) bool { return held.under(parent) || contains(held.keys[parent], key) }
	byPath := map[string]string{}
	opPaths := map[string]string{}
	for _, o := range d.opList {
		if _, ok := opPaths[o.path]; !ok {
			opPaths[o.path] = o.id
		}
	}
	var applications []string
	for _, p := range routes {
		name := p.Key.Value
		at := func(tokens ...string) string {
			return source.Pointer(append([]string{"routes", name}, tokens...)...)
		}
		serves := source.Str(source.Child(p.Value, "serves"))
		path := source.Str(source.Child(p.Value, "path"))
		if path != "" {
			switch {
			case strings.Contains(path, "{"):
				c.add(source.Child(p.Value, "path"), at("path"), RuleRoute, "route %s has a parameter in its path %s; a route answers a fixed path, and a path with a parameter is an operation under paths", name, path)
			case (serves == "application" || serves == "files") && !strings.HasSuffix(path, "/"):
				c.add(source.Child(p.Value, "path"), at("path"), RuleRoute, "route %s serves %s at %s, which is a prefix and ends with a slash, so that every path under it is answered; write %s/", name, serves, path, path)
			case serves == "files" && path == "/":
				c.add(source.Child(p.Value, "path"), at("path"), RuleRoute, "route %s serves files at /, so every path no operation answers would be read as a file; give the folder a prefix of its own, such as /assets/", name)
			case serves == "health" && strings.HasSuffix(path, "/"):
				c.add(source.Child(p.Value, "path"), at("path"), RuleRoute, "route %s is a health check at %s, one path, which does not end with a slash; write %s", name, path, strings.TrimSuffix(path, "/"))
			}
			if other, dup := byPath[path]; dup {
				c.add(source.Child(p.Value, "path"), at("path"), RuleRoute, "route %s answers %s, which route %s answers already; one route per path", name, path, other)
			} else {
				byPath[path] = name
			}
			if op, ok := opPaths[path]; ok {
				c.add(source.Child(p.Value, "path"), at("path"), RuleRoute, "route %s answers %s, which operation %s answers already; a path is a route or an operation, not both", name, path, op)
			}
			if serves == "application" && !strings.Contains(path, "{") && strings.HasSuffix(path, "/") {
				applications = append(applications, path)
			}
		}
		csp := source.Child(p.Value, "contentSecurityPolicy")
		switch {
		case csp != nil && serves != "application" && serves != "":
			c.add(source.Key(p.Value, "contentSecurityPolicy"), at("contentSecurityPolicy"), RuleRoute, "route %s serves %s, which is no document, so a content security policy says nothing there; remove contentSecurityPolicy, or serve the application", name, serves)
		case csp == nil && serves == "application" && !waits(at(), "contentSecurityPolicy"):
			c.warn(p.Key, at(), RuleRoute, "route %s serves the application's document with no content security policy, so every script source is allowed; write contentSecurityPolicy, with script-src naming nonce", name)
		}
		hasNonce := c.checkPolicy(name, p.Value, at)
		caching := source.Child(p.Value, "caching")
		store := source.Str(source.Child(caching, "store"))
		maxAge := source.Child(caching, "maxAgeSeconds")
		immutable := source.Str(source.Child(caching, "immutable")) == "true"
		switch {
		case caching == nil:
		case store == "false" && maxAge != nil:
			c.add(source.Key(caching, "maxAgeSeconds"), at("caching", "maxAgeSeconds"), RuleRoute, "route %s is never stored, so maxAgeSeconds says nothing; remove it, or write store: true", name)
		case store == "false" && source.Child(caching, "immutable") != nil:
			c.add(source.Key(caching, "immutable"), at("caching", "immutable"), RuleRoute, "route %s is never stored, so immutable says nothing; remove it, or write store: true", name)
		case store == "true" && maxAge == nil && !waits(at("caching"), "maxAgeSeconds"):
			c.add(source.Key(caching, "store"), at("caching", "store"), RuleRoute, "route %s may be stored but says for how long nowhere; write maxAgeSeconds, since a cache's own default differs from one cache to the next", name)
		case store == "true" && immutable && source.Str(maxAge) == "0":
			c.add(source.Key(caching, "immutable"), at("caching", "immutable"), RuleRoute, "route %s is immutable while fresh and fresh for no time, so the two say nothing together; give maxAgeSeconds, or remove immutable", name)
		}
		if hasNonce && store == "true" {
			c.add(source.Key(caching, "store"), at("caching", "store"), RuleRoute, "route %s puts a nonce made for each response in its policy and lets a cache store the document, so a stored document carries a nonce a later response's policy does not name and its scripts are refused; write store: false", name)
		}
	}
	if len(applications) == 0 {
		return
	}
	sort.Strings(applications)
	for _, pg := range source.Pairs(source.Child(d.root, "pages")) {
		route := source.Str(source.Child(pg.Value, "route"))
		if route == "" {
			continue
		}
		served := false
		for _, prefix := range applications {
			if strings.HasPrefix(route, prefix) {
				served = true
				break
			}
		}
		if !served {
			c.add(source.Child(pg.Value, "route"), source.Pointer("pages", pg.Key.Value, "route"), RuleRoute, "page %s is at %s, where no application route serves the document, so a browser opening it gets no page; put it under %s, or add a route serving the application there", pg.Key.Value, route, routeOr(applications))
		}
	}
}

// checkPolicy checks one route's content security policy, and reports
// whether it names the nonce: every directive is one CSP Level 3 defines,
// a keyword is in single quotes, nonce stands only where a nonce is read
// and never beside 'unsafe-inline', which a browser that reads the nonce
// ignores, and no nonce value is written, since one is made per response.
func (c *checker) checkPolicy(name string, rt *yaml.Node, at func(...string) string) (hasNonce bool) {
	csp := source.Child(rt, "contentSecurityPolicy")
	if csp == nil {
		return false
	}
	governed := false
	for _, dir := range source.Pairs(csp) {
		directive := dir.Key.Value
		if !cspDirectives[directive] {
			c.add(dir.Key, at("contentSecurityPolicy", directive), RuleRoute, "%s is not a directive of Content Security Policy Level 3%s", directive, suggest(directive, boolKeys(cspDirectives)))
			continue
		}
		if directive == "default-src" || directive == "script-src" {
			governed = true
		}
		nonceHere, inlineHere := false, false
		for i, v := range source.Items(dir.Value) {
			ptr := at("contentSecurityPolicy", directive, fmt.Sprint(i))
			value := v.Value
			switch {
			case value == cspNonce:
				nonceHere = true
				if !cspNonceDirectives[directive] {
					c.add(v, ptr, RuleRoute, "nonce in %s says nothing: a nonce is read in script-src, script-src-elem, style-src, style-src-elem and default-src alone; remove it", directive)
				}
			case strings.HasPrefix(value, "'nonce-"):
				c.add(v, ptr, RuleRoute, "%s writes a nonce value, and a nonce is made for each response; write the word nonce, which stands for it", directive)
			case value == "'unsafe-inline'":
				inlineHere = true
			case cspKeywords[value]:
				c.add(v, ptr, RuleRoute, "%s in %s is a keyword, which CSP Level 3 writes in single quotes, '%s'; without them it names a host called %s", value, directive, value, value)
			case cspKeywords[strings.Trim(value, "'")] && !strings.HasPrefix(value, "'"):
				c.add(v, ptr, RuleRoute, "%s in %s is a keyword, which CSP Level 3 writes in single quotes, '%s'", value, directive, strings.Trim(value, "'"))
			}
		}
		if nonceHere {
			hasNonce = true
		}
		if nonceHere && inlineHere {
			c.add(dir.Key, at("contentSecurityPolicy", directive), RuleRoute, "%s names nonce beside 'unsafe-inline', and a browser that reads the nonce ignores 'unsafe-inline' (CSP Level 3), so the pair says two things; keep nonce", directive)
		}
	}
	if !governed && len(source.Pairs(csp)) > 0 {
		c.warn(source.Key(rt, "contentSecurityPolicy"), at("contentSecurityPolicy"), RuleRoute, "the policy of route %s names neither default-src nor script-src, so scripts are not governed by it; write script-src", name)
	}
	return hasNonce
}

// routeOr joins items as "a, b or c".
func routeOr(items []string) string {
	if len(items) == 1 {
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " or " + items[len(items)-1]
}

// boolKeys lists the keys of a set as a map suggest reads.
func boolKeys(set map[string]bool) map[string]*yaml.Node {
	out := map[string]*yaml.Node{}
	for k := range set {
		out[k] = &yaml.Node{}
	}
	return out
}

// routeStatuses lists the statuses a route answers, by its kind and its
// permission: 200 and 405 always, 304 when a cache may store it, 401 when
// a caller must be signed in or a session may expire, 403 when a
// permission is needed, 404 for a file that is not there, 503 for a
// service that is not healthy.
func routeStatuses(root, rt *yaml.Node) []string {
	out := []string{"200"}
	if source.Str(source.Child(source.Child(rt, "caching"), "store")) == "true" {
		out = append(out, "304")
	}
	perm := source.Str(source.Child(rt, "permission"))
	if perm == permissionSignedIn || perm != "" && perm != permissionPublic && source.Child(root, "session") != nil {
		out = append(out, "401")
	}
	if perm != "" && perm != permissionPublic && perm != permissionSignedIn {
		out = append(out, "403")
	}
	if source.Str(source.Child(rt, "serves")) == "files" {
		out = append(out, "404")
	}
	out = append(out, "405")
	if source.Str(source.Child(rt, "serves")) == "health" {
		out = append(out, "503")
	}
	return out
}

// routeCaseStatus is the status a derived case of a route expects, or ""
// when the case does not fix one.
func routeCaseStatus(name string) string {
	switch {
	case name == "":
		return "200"
	case name == "method not allowed":
		return "405"
	case name == "not found file":
		return "404"
	case name == "unhealthy":
		return "503"
	case name == "denied when not signed in", name == "denied with expired session":
		return "401"
	case strings.HasPrefix(name, "denied without "):
		return "403"
	}
	return ""
}

// routeSubject is a route's tests: the request it answers, the callers it
// refuses, a method it does not take, and by its kind a file that is not
// there, a path that climbs out of the folder, and a service that is not
// healthy.
func (d *design) routeSubject(p source.Pair) *subject {
	name := p.Key.Value
	v := p.Value
	s := &subject{kind: "route", label: "route " + name, node: p.Key, path: source.Pointer("routes", name),
		yamlKey: "route: " + name, name: name}
	path := source.Str(source.Child(v, "path"))
	serves := source.Str(source.Child(v, "serves"))
	perm := source.Str(source.Child(v, "permission"))
	get := "GET " + path + " is requested"
	then := "it answers 200"
	switch serves {
	case "application":
		then = "it answers 200 with the document, its policy naming a nonce made for this response"
		if source.Str(source.Child(source.Child(v, "caching"), "store")) == "false" {
			then += ", and Cache-Control: no-store"
		}
	case "files":
		get = "a file under " + path + " is requested"
		then = "it answers 200 with the file, its content type and its caching"
	case "health":
		then = "it answers 200: the service is healthy"
	}
	s.success = golden(caller(perm), get, then)
	s.closed = perm == permissionNobody
	d.denied(s, perm, get)
	d.throughCases(s, perm, get)
	s.red("method not allowed", occasional, "...", path+" is requested with POST", "it answers 405 with Allow: GET, HEAD")
	switch serves {
	case "files":
		s.red("not found file", frequent, "no file of that name is under the folder", "a file that is not there is requested under "+path, "it answers 404")
		s.byNature("path outside the folder", "a request whose path climbs out of the folder with ..", "it is requested under "+path, "it is refused, and no file outside the folder is read")
	case "health":
		s.byNature("unhealthy", "what the service needs is not reachable", get, "it answers 503, naming what is not healthy")
	}
	return s
}
