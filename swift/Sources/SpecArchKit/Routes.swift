import Foundation

// The fixed routes a server answers beside its operations (ADR-216): the
// browser application's document, the files under a folder, the health
// of the service. GET and HEAD alone, at one path or under one prefix.

/// The directives Content Security Policy Level 3 defines, by name.
private let cspDirectives: Set<String> = [
    "child-src", "connect-src", "default-src", "font-src", "frame-src",
    "img-src", "manifest-src", "media-src", "object-src", "script-src",
    "script-src-elem", "script-src-attr", "style-src", "style-src-elem",
    "style-src-attr", "worker-src", "base-uri", "sandbox", "form-action",
    "frame-ancestors", "report-to", "require-trusted-types-for", "trusted-types",
    "upgrade-insecure-requests",
]

/// The directives a nonce is read in: a nonce matches a script or a style
/// element, and default-src stands in for their directives when absent.
private let cspNonceDirectives: Set<String> = ["default-src", "script-src", "script-src-elem", "style-src", "style-src-elem"]

/// The keyword sources CSP Level 3 writes in single quotes; written
/// without them, each names a host.
private let cspKeywords: Set<String> = [
    "self", "none", "unsafe-inline", "unsafe-eval", "strict-dynamic",
    "unsafe-hashes", "report-sample", "wasm-unsafe-eval", "inline-speculation-rules",
]

/// The word that stands for the nonce the server makes for each response.
private let cspNonce = "nonce"

extension Checker {
    /// Checks the fixed routes: a path of the shape its kind needs, one
    /// route per path and no path an operation answers, a content security
    /// policy on an application route alone and well formed, a caching
    /// that says one thing, a nonce never stored, and every page's route
    /// served by an application route once one is declared.
    func checkRoutes(_ d: Design) {
        let routes = pairs(d.root.child("routes"))
        if routes.isEmpty { return }
        // A key an open must question asks for is not an error while it
        // waits for the answer, as a reader writes it (docs/principles.md).
        let held = blockedPointers(d.root)
        func waits(_ parent: String, _ key: String) -> Bool { held.under(parent) || (held.keys[parent] ?? []).contains(key) }
        var byPath: [String: String] = [:]
        var opPaths: [String: String] = [:]
        for o in d.opList where opPaths[o.path] == nil { opPaths[o.path] = o.id }
        var applications: [String] = []
        for p in routes {
            let name = p.key.value
            func at(_ tokens: String...) -> String { pointer(["routes", name] + tokens) }
            let serves = str(p.value.child("serves"))
            let path = str(p.value.child("path"))
            if !path.isEmpty {
                if path.contains("{") {
                    add(p.value.child("path"), at("path"), .route, "route \(name) has a parameter in its path \(path); a route answers a fixed path, and a path with a parameter is an operation under paths")
                } else if (serves == "application" || serves == "files") && !path.hasSuffix("/") {
                    add(p.value.child("path"), at("path"), .route, "route \(name) serves \(serves) at \(path), which is a prefix and ends with a slash, so that every path under it is answered; write \(path)/")
                } else if serves == "files" && path == "/" {
                    add(p.value.child("path"), at("path"), .route, "route \(name) serves files at /, so every path no operation answers would be read as a file; give the folder a prefix of its own, such as /assets/")
                } else if serves == "health" && path.hasSuffix("/") {
                    add(p.value.child("path"), at("path"), .route, "route \(name) is a health check at \(path), one path, which does not end with a slash; write \(String(path.dropLast()))")
                }
                if let other = byPath[path] {
                    add(p.value.child("path"), at("path"), .route, "route \(name) answers \(path), which route \(other) answers already; one route per path")
                } else {
                    byPath[path] = name
                }
                if let op = opPaths[path] {
                    add(p.value.child("path"), at("path"), .route, "route \(name) answers \(path), which operation \(op) answers already; a path is a route or an operation, not both")
                }
                if serves == "application" && !path.contains("{") && path.hasSuffix("/") {
                    applications.append(path)
                }
            }
            let csp = p.value.child("contentSecurityPolicy")
            if csp != nil && serves != "application" && !serves.isEmpty {
                add(p.value.key("contentSecurityPolicy"), at("contentSecurityPolicy"), .route, "route \(name) serves \(serves), which is no document, so a content security policy says nothing there; remove contentSecurityPolicy, or serve the application")
            } else if csp == nil && serves == "application" && !waits(at(), "contentSecurityPolicy") {
                warn(p.key, at(), .route, "route \(name) serves the application's document with no content security policy, so every script source is allowed; write contentSecurityPolicy, with script-src naming nonce")
            }
            let hasNonce = checkPolicy(name, p.value, at)
            let caching = p.value.child("caching")
            let store = str(caching?.child("store"))
            let maxAge = caching?.child("maxAgeSeconds")
            let immutable = str(caching?.child("immutable")) == "true"
            if caching == nil {
            } else if store == "false" && maxAge != nil {
                add(caching?.key("maxAgeSeconds"), at("caching", "maxAgeSeconds"), .route, "route \(name) is never stored, so maxAgeSeconds says nothing; remove it, or write store: true")
            } else if store == "false" && caching?.child("immutable") != nil {
                add(caching?.key("immutable"), at("caching", "immutable"), .route, "route \(name) is never stored, so immutable says nothing; remove it, or write store: true")
            } else if store == "true" && maxAge == nil && !waits(at("caching"), "maxAgeSeconds") {
                add(caching?.key("store"), at("caching", "store"), .route, "route \(name) may be stored but says for how long nowhere; write maxAgeSeconds, since a cache's own default differs from one cache to the next")
            } else if store == "true" && immutable && str(maxAge) == "0" {
                add(caching?.key("immutable"), at("caching", "immutable"), .route, "route \(name) is immutable while fresh and fresh for no time, so the two say nothing together; give maxAgeSeconds, or remove immutable")
            }
            if hasNonce && store == "true" {
                add(caching?.key("store"), at("caching", "store"), .route, "route \(name) puts a nonce made for each response in its policy and lets a cache store the document, so a stored document carries a nonce a later response's policy does not name and its scripts are refused; write store: false")
            }
        }
        if applications.isEmpty { return }
        applications.sort()
        for pg in pairs(d.root.child("pages")) {
            let route = str(pg.value.child("route"))
            if route.isEmpty { continue }
            if !applications.contains(where: { route.hasPrefix($0) }) {
                add(pg.value.child("route"), pointer("pages", pg.key.value, "route"), .route, "page \(pg.key.value) is at \(route), where no application route serves the document, so a browser opening it gets no page; put it under \(orList(applications)), or add a route serving the application there")
            }
        }
    }

    /// Checks one route's content security policy, and reports whether it
    /// names the nonce: every directive is one CSP Level 3 defines, a
    /// keyword is in single quotes, nonce stands only where a nonce is read
    /// and never beside 'unsafe-inline', which a browser that reads the
    /// nonce ignores, and no nonce value is written, since one is made per
    /// response.
    func checkPolicy(_ name: String, _ rt: YNode, _ at: (String...) -> String) -> Bool {
        guard let csp = rt.child("contentSecurityPolicy") else { return false }
        var hasNonce = false
        var governed = false
        var valid: [String: YNode] = [:]
        for d in cspDirectives { valid[d] = YNode(kind: .scalar, line: 0) }
        for dir in pairs(csp) {
            let directive = dir.key.value
            if !cspDirectives.contains(directive) {
                add(dir.key, at("contentSecurityPolicy", directive), .route, "\(directive) is not a directive of Content Security Policy Level 3\(suggest(directive, valid))")
                continue
            }
            if directive == "default-src" || directive == "script-src" { governed = true }
            var nonceHere = false, inlineHere = false
            for (i, v) in items(dir.value).enumerated() {
                let ptr = at("contentSecurityPolicy", directive, "\(i)")
                let value = v.value
                if value == cspNonce {
                    nonceHere = true
                    if !cspNonceDirectives.contains(directive) {
                        add(v, ptr, .route, "nonce in \(directive) says nothing: a nonce is read in script-src, script-src-elem, style-src, style-src-elem and default-src alone; remove it")
                    }
                } else if value.hasPrefix("'nonce-") {
                    add(v, ptr, .route, "\(directive) writes a nonce value, and a nonce is made for each response; write the word nonce, which stands for it")
                } else if value == "'unsafe-inline'" {
                    inlineHere = true
                } else if cspKeywords.contains(value) {
                    add(v, ptr, .route, "\(value) in \(directive) is a keyword, which CSP Level 3 writes in single quotes, '\(value)'; without them it names a host called \(value)")
                } else if cspKeywords.contains(value.trimmingCharacters(in: CharacterSet(charactersIn: "'"))) && !value.hasPrefix("'") {
                    add(v, ptr, .route, "\(value) in \(directive) is a keyword, which CSP Level 3 writes in single quotes, '\(value.trimmingCharacters(in: CharacterSet(charactersIn: "'")))'")
                }
            }
            if nonceHere { hasNonce = true }
            if nonceHere && inlineHere {
                add(dir.key, at("contentSecurityPolicy", directive), .route, "\(directive) names nonce beside 'unsafe-inline', and a browser that reads the nonce ignores 'unsafe-inline' (CSP Level 3), so the pair says two things; keep nonce")
            }
        }
        if !governed && !pairs(csp).isEmpty {
            warn(rt.key("contentSecurityPolicy"), at("contentSecurityPolicy"), .route, "the policy of route \(name) names neither default-src nor script-src, so scripts are not governed by it; write script-src")
        }
        return hasNonce
    }
}

/// The statuses a route answers, by its kind and its permission: 200 and
/// 405 always, 304 when a cache may store it, 401 when a caller must be
/// signed in or a session may expire, 403 when a permission is needed, 404
/// for a file that is not there, 503 for a service that is not healthy.
func routeStatuses(_ root: YNode, _ rt: YNode) -> [String] {
    var out = ["200"]
    if str(rt.child("caching")?.child("store")) == "true" { out.append("304") }
    let perm = str(rt.child("permission"))
    if perm == permissionSignedIn || (!perm.isEmpty && perm != permissionPublic && root.child("session") != nil) { out.append("401") }
    if !perm.isEmpty && perm != permissionPublic && perm != permissionSignedIn { out.append("403") }
    if str(rt.child("serves")) == "files" { out.append("404") }
    out.append("405")
    if str(rt.child("serves")) == "health" { out.append("503") }
    return out
}

/// Whether a content security policy names the nonce made for each
/// response in any of its directives.
func policyNamesNonce(_ csp: YNode?) -> Bool {
    for dir in pairs(csp) {
        for v in items(dir.value) where v.value == cspNonce { return true }
    }
    return false
}

extension Design {
    /// A route's tests: the request it answers, the callers it refuses, a
    /// method it does not take, and by its kind a file that is not there, a
    /// path that climbs out of the folder, and a service that is not
    /// healthy.
    func routeSubject(_ p: Pair) -> Subject {
        let name = p.key.value
        let v = p.value
        let s = Subject(label: "route " + name, node: p.key, path: pointer("routes", name), yamlKey: "route: " + name, name: name)
        let path = str(v.child("path"))
        let serves = str(v.child("serves"))
        let perm = str(v.child("permission"))
        var get = "GET " + path + " is requested"
        var then = "it answers 200"
        switch serves {
        case "application":
            then = "it answers 200 with the document"
            if policyNamesNonce(v.child("contentSecurityPolicy")) { then += ", its policy naming a nonce made for this response" }
            if str(v.child("caching")?.child("store")) == "false" { then += ", and Cache-Control: no-store" }
        case "files":
            get = "a file under " + path + " is requested"
            then = "it answers 200 with the file, its content type and its caching"
        case "health":
            then = "it answers 200: the service is healthy"
        default:
            break
        }
        s.success = golden(caller(perm), get, then)
        s.closed = perm == permissionNobody
        denied(s, perm, get)
        throughCases(s, perm, get)
        s.red("method not allowed", occasional, "...", path + " is requested with POST", "it answers 405 with Allow: GET, HEAD")
        switch serves {
        case "files":
            s.red("not found file", frequent, "no file of that name is under the folder", "a file that is not there is requested under " + path, "it answers 404")
            s.byNature("path outside the folder", "a request whose path climbs out of the folder with ..", "it is requested under " + path, "it is refused, and no file outside the folder is read")
        case "health":
            s.byNature("unhealthy", "what the service needs is not reachable", get, "it answers 503, naming what is not healthy")
        default:
            break
        }
        return s
    }
}
