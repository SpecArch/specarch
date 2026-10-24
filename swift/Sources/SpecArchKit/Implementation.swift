import Foundation

/// Keys that only a specification has. In an implementation file each is
/// reported as design_key instead of an unknown key.
let designKeys: Set<String> = [
    "specarch", "stages", "sources",
    "stakeholders", "needs", "requirements", "glossary", "assumptions",
    "enums", "entities", "permissions", "roles", "paths", "commands",
    "channels", "dependencies", "session", "caller", "clients", "tokens", "securityEvents", "pages", "algorithms", "tests",
    "environments", "release", "rollback", "migrations", "handover", "checks", "signoff",
    "backup", "restore", "relations", "constraints", "transitions", "operationId", "responses",
    "requestBody", "parameters", "permission", "formula", "examples",
    "properties", "primaryKey", "stateField", "messages", "payload",
    "calls", "idempotencyKey", "guard", "validity",
    "sensitivity", "atRest", "lookup", "audited", "deletion",
    "listOf", "limits", "problem", "errors", "jobs", "gateways", "menus", "workflows",
    "views", "count", "schemas",
]

/// The implementation file's maps whose keys the author chooses (a folder,
/// a target, a library), so a key there is never a design keyword by
/// mistake.
let namedMaps: Set<String> = [
    "layout", "mappings", "targets", "tasks", "libraries",
    "deployments", "decisions", "suites", "configuration", "bindings", "idioms",
]

/// Cleans a slash path the way Go's filepath.Clean does: no ".", no
/// "name/..", no doubled slashes.
public func cleanPath(_ p: String) -> String {
    if p.isEmpty { return "." }
    let rooted = p.hasPrefix("/")
    var out: [Substring] = []
    for part in p.split(separator: "/", omittingEmptySubsequences: true) {
        if part == "." { continue }
        if part == ".." {
            if let last = out.last, last != ".." { out.removeLast(); continue }
            if rooted { continue }
        }
        out.append(part)
    }
    let joined = out.joined(separator: "/")
    if rooted { return "/" + joined }
    return joined.isEmpty ? "." : joined
}

/// The folder of a path, as Go's filepath.Dir gives it.
public func dirPath(_ p: String) -> String {
    guard let slash = p.lastIndex(of: "/") else { return "." }
    let dir = String(p[..<slash])
    return dir.isEmpty ? "/" : cleanPath(dir)
}

public func joinPath(_ a: String, _ b: String) -> String {
    cleanPath(a + "/" + b)
}

/// The last part of a read error, in the words the C library uses.
public func plainIOError(_ error: Error) -> String {
    let ns = error as NSError
    if let posix = ns.userInfo[NSUnderlyingErrorKey] as? NSError, posix.domain == NSPOSIXErrorDomain,
       let message = strerror(Int32(posix.code)) {
        // Go's error texts are the C library's in lower case.
        let text = String(cString: message)
        return text.prefix(1).lowercased() + text.dropFirst()
    }
    switch ns.code {
    case NSFileReadNoSuchFileError, NSFileNoSuchFileError: return "no such file or directory"
    case NSFileReadNoPermissionError: return "permission denied"
    default: return ns.localizedDescription
    }
}

extension Checker {
    func checkBoundaryImplementation() {
        walk(root, []) { n, path in
            if path.contains("settings") { return } // free-form target and framework settings
            if path.count > 2, path[0] == "mappings", path[2] == "handler" { return } // a handler record holds the code as it was read (ADR-144)
            if let last = path.last, namedMaps.contains(last) { return }
            // A deployment's clients give each client's client_id in its environment (ADR-106).
            let deployment = path.count == 2 && path[0] == "deployments"
            for p in n.pairs where designKeys.contains(p.key.value) && !(deployment && p.key.value == "clients") {
                add(p.key, pointer(path + [p.key.value]), .designKey,
                    "\(p.key.value) belongs to the specification, not to one implementation; move it to the specification's files")
            }
        }
    }

    /// Checks the file against the specification it implements: s when the
    /// file is part of it, otherwise the one its implements.file names, read
    /// through load.
    func checkImplementation(_ given: Spec?, _ load: Loader?) {
        let impl = root.child("implements")
        let fileNode = child(impl, "file")
        let rel = str(fileNode)
        if rel.isEmpty || kindOf(rel) != .design { return } // the schema reports it
        let s: Spec
        if let given {
            if cleanPath(joinPath(dirPath(file), rel)) != cleanPath(given.rootPath) {
                add(fileNode, "/implements/file", .implements, "this file is under \(given.dir) but implements names \(rel); point it at the root file of the specification it is part of")
                return
            }
            s = given
        } else {
            let dir = dirPath(joinPath(dirPath(file), rel))
            guard let load else { return }
            s = load(dir)
            if s.root == nil {
                add(fileNode, "/implements/file", .implements, "the specification's root file \(rel) cannot be read as YAML; run specarch validate on \(dir) first")
                return
            }
        }
        guard let specRoot = s.root, specRoot.child("specarch") != nil else {
            add(fileNode, "/implements/file", .implements, "\(rel) is not a specification's root file; run specarch validate on it first")
            return
        }
        let verNode = child(impl, "version")
        let want = str(child(specRoot.child("info"), "version"))
        let got = str(verNode)
        if !got.isEmpty && got != want {
            add(verNode, "/implements/version", .implements,
                "this file implements version \(got) of \(rel), but that specification is now version \(want); review the design change, then update this version")
        }
        let d = Design(specRoot)
        d.spec = s
        for p in pairs(root.child("layout")) {
            for (i, ref) in items(p.value.child("implements")).enumerated() {
                checkDesignRef(specRoot, ref, ref.value, pointer("layout", p.key.value, "implements", "\(i)"))
            }
        }
        checkDependsOn()
        for p in pairs(root.child("mappings")) {
            checkDesignRef(specRoot, p.key, p.key.value, pointer("mappings", p.key.value))
            if let by = p.value.child("ownedBy"), !by.value.isEmpty, d.stakeholders[by.value] == nil {
                add(by, pointer("mappings", p.key.value, "ownedBy"), .stakeholder, "\(by.value) is not a stakeholder of the specification\(suggest(by.value, d.stakeholders))")
            }
            checkIndexes(d, p.key.value, p.value.child("indexes"))
            checkHandler(d, s, p.key.value, p.value)
        }
        var decisions = d.decisions
        for p in pairs(root.child("decisions")) { decisions[p.key.value] = p.value }
        checkOrigin(root, decisions) { path in path.count == 2 && path[0] == "decisions" }
        for p in pairs(root.child("decisions")) where d.decisions[p.key.value] != nil {
            add(p.key, pointer("decisions", p.key.value), .decision,
                "\(p.key.value) is already a decision of the specification; give this implementation decision its own number")
        }
        checkRequirementLinks(root, [], d)
        checkCitations(root, [], d)
        checkDeployments(d)
        checkSuites(d)
        checkIdioms(s)
    }

    /// Checks a handler record against the code it was read from (ADR-144):
    /// the source it holds for each function must still be what the cited
    /// file holds at the function's lines, the file found through the folder
    /// the code source names under url, from the specification's root. The
    /// working tree is read, not the commit, since neither build runs git.
    func checkHandler(_ d: Design, _ s: Spec, _ key: String, _ m: YNode) {
        guard let rec = m.child("handler") else { return }
        let ptr = pointer("mappings", key, "handler")
        var srcKey = ""
        for cite in items(m.child("cites")) {
            let k = str(cite.child("source"))
            if !k.isEmpty { srcKey = k; break }
        }
        if srcKey.isEmpty {
            warn(rec, ptr, .handlerSource, "the handler record of \(key) cites no source, so the folder its functions are read from is not known; cite the code source under cites")
            return
        }
        let src = d.sources[srcKey]
        let url = str(src?.child("url"))
        if src == nil || url.isEmpty || url.contains("://") {
            warn(rec, ptr, .handlerSource, "the source \(srcKey) that the handler record of \(key) cites names no folder under url, so the record cannot be checked against the code")
            return
        }
        let root = joinPath(s.dir, url)
        let commit = shortCommit(str(rec.child("commit")))
        for (i, fn) in items(rec.child("functions")).enumerated() {
            let fptr = pointer("mappings", key, "handler", "functions", "\(i)", "source")
            let file = str(fn.child("file"))
            let name = str(fn.child("name"))
            let first = Int(intValue(fn.child("firstLine")) ?? 0)
            let last = Int(intValue(fn.child("lastLine")) ?? 0)
            guard !file.isEmpty, let srcNode = fn.child("source"), first >= 1, last >= first else { continue }
            guard let data = try? readFile(joinPath(root, file)), let text = String(data: data, encoding: .utf8) else {
                warn(fn.child("file"), pointer("mappings", key, "handler", "functions", "\(i)", "file"), .handlerSource,
                     "\(file), from which the handler record of \(key) read \(name) at commit \(commit), is not under \(url), so the record cannot be checked against the code; run specarch extract go with --implementation-out again and replace the record")
                continue
            }
            let lines = trimmedLines(text)
            let want = trimmedLines(str(srcNode))
            if last <= lines.count, dedented(Array(lines[(first - 1)..<last])) == want { continue }
            if let at = findLines(lines, want) {
                warn(srcNode, fptr, .handlerSource,
                     "the handler record of \(key) read \(name) at commit \(commit) from \(file) lines \(first)-\(last), and that source is now at lines \(at)-\(at + want.count - 1); run specarch extract go with --implementation-out again and replace the record")
                continue
            }
            var differs = first
            let held = first - 1 < lines.count ? dedented(Array(lines[(first - 1)...])) : []
            for j in 0..<want.count {
                if j >= held.count || held[j] != want[j] { differs = first + j; break }
            }
            warn(srcNode, fptr, .handlerSource,
                 "the handler record of \(key) read \(name) at commit \(commit), and \(file) lines \(first)-\(last) no longer hold that source (line \(differs) differs), so the flowchart no longer matches the code; run specarch extract go with --implementation-out again and replace the record")
        }
    }

    /// The first seven characters of a commit.
    func shortCommit(_ c: String) -> String { c.count > 7 ? String(c.prefix(7)) : c }

    /// The lines of a text, each without its trailing spaces and tabs, as a
    /// handler record holds them.
    func trimmedLines(_ text: String) -> [String] {
        text.split(separator: "\n", omittingEmptySubsequences: false).map { line in
            var l = Substring(line)
            while let last = l.last, last == " " || last == "\t" { l.removeLast() }
            return String(l)
        }
    }

    /// Lines without the indentation of the first one, as a handler record
    /// holds a function's source: a function literal written inside a
    /// function begins indented, and a YAML block cannot begin with a tab.
    func dedented(_ lines: [String]) -> [String] {
        guard let firstLine = lines.first else { return lines }
        let indent = String(firstLine.prefix { $0 == " " || $0 == "\t" })
        if indent.isEmpty { return lines }
        return lines.map { $0.hasPrefix(indent) ? String($0.dropFirst(indent.count)) : $0 }
    }

    /// The line, from 1, at which want starts in lines, or nil.
    func findLines(_ lines: [String], _ want: [String]) -> Int? {
        if want.isEmpty { return nil }
        var i = 0
        while i + want.count <= lines.count {
            if dedented(Array(lines[i..<(i + want.count)])) == want { return i + 1 }
            i += 1
        }
        return nil
    }

    /// Checks a mapping's indexes are on an entity and hold its fields
    /// (ADR-121).
    func checkIndexes(_ d: Design, _ key: String, _ indexes: YNode?) {
        guard let indexes else { return }
        let name = key.hasPrefix("#/entities/") ? String(key.dropFirst("#/entities/".count)) : ""
        guard !name.isEmpty, let entity = d.entities[name] else {
            add(indexes, pointer("mappings", key, "indexes"), .field,
                "indexes are a table's, and \(key) is not an entity of the specification; put them on the mapping of the entity whose table holds them")
            return
        }
        let fields = fieldsOf(entity)
        for (i, ix) in items(indexes).enumerated() {
            checkFieldList(ix.child("fields"), ["mappings", key, "indexes", "\(i)", "fields"], fields, name, "part of an index")
        }
    }

    /// Checks that a deployment gives the facts of the design's signIn that
    /// differ per environment (see the Go build's checkDeploymentSignIn).
    func checkDeploymentSignIn(_ d: Design, _ base: [String], _ p: Pair) {
        let si = p.value.child("signIn")
        guard let design = d.signIn else {
            if si != nil {
                add(p.value.key("signIn"), pointer(base + ["signIn"]), .signIn, "the specification declares no signIn, so there is no identity provider for this deployment to name; remove signIn, or declare signIn in the design")
            }
            return
        }
        if si == nil {
            add(p.key, pointer(base), .signIn, "the specification's signIn has people sign in at \(str(design.child("provider"))), and this deployment does not say which provider that is here; add signIn with issuerUrl, the provider's issuer")
        }
        let client = str(design.child("client"))
        if !client.isEmpty && d.client(client) != nil && p.value.child("clients")?.child(client)?.child("clientId") == nil {
            add(p.key, pointer(base), .signIn, "people sign in through \(client), whose client_id the identity token's aud carries, and this deployment gives none; add clients.\(client).clientId, as the provider registered it")
        }
    }

    /// Checks each deployment names an environment of the specification
    /// when it declares any, gives values only to settings the
    /// specification declares and that are not secret, names tasks of this
    /// file in its build, and, when the customer runs the environment, says
    /// how the build is made, how it is installed and what the hosts must
    /// provide (ADR-129).
    func checkDeployments(_ d: Design) {
        var tasks: [String: YNode] = [:]
        for p in pairs(root.child("tasks")) { tasks[p.key.value] = p.value }
        for p in pairs(root.child("deployments")) {
            let base = ["deployments", p.key.value]
            let envNode = p.value.child("environment")
            let env = str(envNode)
            if !env.isEmpty && d.environments[env] == nil {
                add(envNode, pointer(base + ["environment"]), .environment, "\(env) is not an environment of the specification\(suggest(env, d.environments))")
            } else if env.isEmpty && !d.environments.isEmpty {
                add(p.key, pointer(base), .environment, "the specification declares environments, so this deployment must say which one it installs; add environment with one of \(d.environments.keys.sorted(by: byteLess).joined(separator: ", "))")
            }
            let build = p.value.child("build")
            for k in ["task", "sbom"] {
                if let t = child(build, k), !t.value.isEmpty, tasks[t.value] == nil {
                    add(t, pointer(base + ["build", k]), .buildTask, "\(t.value) is not a task of this implementation file\(suggest(t.value, tasks))")
                }
            }
            if !env.isEmpty && str(child(d.environments[env], "runBy")) == "customer" {
                for (key, what) in [
                    ("build", "how the artifact is made from the source"),
                    ("method", "how the build is put on the hosts"),
                    ("infrastructure", "what the hosts must provide"),
                ] where p.value.child(key) == nil {
                    add(p.key, pointer(base), .handover, "\(env) is run by the customer, who builds and installs it from this file, and this deployment has no \(key) saying \(what); add \(key)")
                }
            }
            checkServers(d, base, p.value.child("servers"))
            checkLinks(d, base, p.value)
            checkDeploymentSignIn(d, base, p)
            for m in pairs(p.value.child("monitors")) where d.monitors[m.key.value] == nil {
                add(m.key, pointer(base + ["monitors", m.key.value]), .monitor, "\(m.key.value) is not a monitor of the specification\(suggest(m.key.value, d.monitors))")
            }
            for cfg in pairs(p.value.child("configuration")) {
                let ptr = pointer(base + ["configuration", cfg.key.value])
                guard let setting = d.settings[cfg.key.value] else {
                    add(cfg.key, ptr, .setting, "\(cfg.key.value) is not a setting of the specification's configuration\(suggest(cfg.key.value, d.settings))")
                    continue
                }
                if str(setting.child("secret")) == "true" {
                    add(cfg.value, ptr, .secretValue, "\(cfg.key.value) is a secret, and a secret's value is never written in a specification; remove it and say in the setting's description where the value comes from")
                }
            }
        }
    }

    /// Checks every suite runs design tests that exist, or says it is
    /// implementation-only.
    func checkSuites(_ d: Design) {
        var tests: [String: YNode] = [:]
        for p in pairs(d.root.child("tests")) { tests[p.key.value] = p.value }
        for s in pairs(child(root.child("testing"), "suites")) {
            let base = ["testing", "suites", s.key.value]
            for (i, n) in items(s.value.child("designTests")).enumerated() where tests[n.value] == nil {
                add(n, pointer(base + ["designTests", "\(i)"]), .suite, "\(n.value) is not a test of the specification\(suggest(n.value, tests))")
            }
            for (i, subject) in items(s.value.child("designTestsOf")).enumerated() {
                for p in pairs(subject) {
                    let key = p.key.value + ": " + p.value.value
                    if !tests.values.contains(where: { testSubjectKey($0) == key }) {
                        add(p.value, pointer(base + ["designTestsOf", "\(i)", p.key.value]), .suite,
                            "the specification has no test about \(p.key.value) \(p.value.value); name a subject that has tests")
                    }
                }
            }
        }
    }

    func checkDesignRef(_ designRoot: YNode, _ at: YNode, _ ref: String, _ ptr: String) {
        guard ref.hasPrefix("#/") else { return } // the schema reports it
        let tokens = ref.dropFirst(2).split(separator: "/", omittingEmptySubsequences: false).map { unescapeToken(String($0)) }
        if !resolve(designRoot, tokens).1 {
            add(at, ptr, .designRef, "\(ref) does not point at anything in the specification; correct the pointer (a / inside a name is written ~1)")
        }
    }

    // MARK: change log

    /// Warns when prose in the file tells how it changed instead of what is
    /// true now.
    func checkChangeLog() {
        walk(root, []) { n, path in
            for p in n.pairs where proseKeys.contains(p.key.value) && p.value.kind == .scalar {
                let v = p.value.value
                if let r = changeLogPhrase.firstMatch(in: v, range: NSRange(v.startIndex..., in: v)), let range = Range(r.range, in: v) {
                    warn(p.value, pointer(path + [p.key.value]), .changeLog,
                         "\(quote(String(v[range]))) reads as a change log; say what is true now, and record the change in the project's history instead")
                }
            }
        }
    }

    // MARK: integers

    /// Refuses an int64 or uint64 carried as a JSON number unless its bounds
    /// keep it inside 2^53, where every JSON reader holds it exactly.
    func checkIntegers() {
        walk(root, []) { n, path in
            let format = str(n.child("format"))
            guard format == "int64" || format == "uint64", carriesAs(n, "integer") else { return }
            let low = bound(n, "minimum"), high = bound(n, "maximum")
            var safe = high != nil && high! <= maxSafe
            if format == "int64" { safe = safe && low != nil && low! >= -maxSafe }
            if !safe {
                add(n.child("format"), pointer(path + ["format"]), .unsafeInteger,
                    "an \(format) sent as a JSON number loses digits above 2^53 in JavaScript and other readers; carry it as type: string, format: \(format), or bound it with minimum and maximum inside 9007199254740991")
            }
        }
    }
}

/// The few phrases that turn a description into a change log. The list is
/// short on purpose: it catches the obvious cases and never a sentence about
/// the design itself.
let changeLogPhrase = try! NSRegularExpression(pattern: "\\b(previously|formerly|changed from|was changed|updated on|new in (version|v?[0-9]))\\b", options: [.caseInsensitive])

let proseKeys: Set<String> = [
    "description", "summary", "title", "context", "decision", "consequences", "note", "message",
    "statement", "definition", "why", "says", "action", "check",
]

/// 2^53 - 1, the largest integer a JavaScript number holds exactly.
let maxSafe = Decimal(9007199254740991)

func carriesAs(_ n: YNode, _ t: String) -> Bool {
    let typ = n.child("type")
    return str(typ) == t || items(typ).contains { $0.value == t }
}

func bound(_ n: YNode, _ key: String) -> Decimal? {
    guard let v = n.child(key) else { return nil }
    if let i = intValue(v) { return Decimal(i) }
    return Decimal(string: v.value)
}

extension Checker {
    /// Checks that every package a layout entry depends on is another key
    /// of the layout (ADR-133). A cycle is no error: Go forbids one and
    /// other stacks allow it, and the file describes the code as it is.
    func checkDependsOn() {
        let layout = root.child("layout")
        var keys: [String: YNode] = [:]
        for p in pairs(layout) { keys[p.key.value] = p.value }
        for p in pairs(layout) {
            for (i, e) in items(p.value.child("dependsOn")).enumerated() {
                let ptr = pointer("layout", p.key.value, "dependsOn", "\(i)")
                if e.value == p.key.value {
                    add(e, ptr, .dependsOn, "\(e.value) names itself; a package does not depend on itself, so remove the entry")
                } else if keys[e.value] == nil {
                    add(e, ptr, .dependsOn, "\(e.value) is not a key of layout; add it to layout, or remove the entry when it is a library, which goes under libraries\(suggest(e.value, keys))")
                }
            }
        }
    }
}

extension Checker {
    /// Checks a deployment's servers (ADR-136): a name used once, what each
    /// runs is in the file, packages only on a service, and a server that
    /// is a dependency names a declared one and says nothing of its own
    /// kind or what it runs.
    func checkServers(_ d: Design, _ base: [String], _ servers: YNode?) {
        var layout: [String: YNode] = [:]
        for p in pairs(root.child("layout")) { layout[p.key.value] = p.value }
        var targets: [String: YNode] = [:]
        for p in pairs(root.child("targets")) { targets[p.key.value] = p.value }
        var named: Set<String> = []
        for (i, srv) in items(servers).enumerated() {
            let at = base + ["servers", "\(i)"]
            if let n = srv.child("name"), !n.value.isEmpty {
                if named.contains(n.value) {
                    add(n, pointer(at + ["name"]), .server, "\(n.value) names another server of this deployment already; give each server its own name")
                }
                named.insert(n.value)
            }
            var kind = str(srv.child("kind"))
            if kind.isEmpty { kind = "service" }
            let runs = srv.child("runs")
            if let dep = srv.child("dependency"), !dep.value.isEmpty {
                if d.dependencies[dep.value] == nil {
                    add(dep, pointer(at + ["dependency"]), .server, "\(dep.value) is not a dependency of the specification\(suggest(dep.value, d.dependencies))")
                }
                for key in ["kind", "runs"] {
                    if let k = srv.child(key) {
                        add(k, pointer(at + [key]), .server, "this server is the dependency \(dep.value) in this environment, which takes its kind from the dependency and runs nothing of this implementation; remove \(key)")
                    }
                }
                continue
            }
            for (j, pkg) in items(child(runs, "packages")).enumerated() {
                let ptr = pointer(at + ["runs", "packages", "\(j)"])
                if layout[pkg.value] == nil {
                    add(pkg, ptr, .server, "\(pkg.value) is not a key of layout\(suggest(pkg.value, layout))")
                } else if kind != "service" {
                    add(pkg, ptr, .server, "a server of kind \(kind) runs no package of this implementation; give it kind service, or move the package to the server that runs it")
                }
            }
            for (j, t) in items(child(runs, "targets")).enumerated() where targets[t.value] == nil {
                add(t, pointer(at + ["runs", "targets", "\(j)"]), .server, "\(t.value) is not a key of targets\(suggest(t.value, targets))")
            }
        }
    }
}

/// What the packages of one server reach: each dependency with the design
/// objects that reach it, and the objects that touch the data.
struct ServerReach {
    var dependencies: [String: [String]] = [:]
    var data: [String] = []
}

extension Checker {
    /// Checks a deployment's links against its servers (ADR-137): each
    /// joins two named servers on a port the target listens on. When the
    /// deployment declares links, it then warns at each server whose
    /// packages call a dependency, use a channel one carries or reach the
    /// database with no link that allows it: the firewall check.
    func checkLinks(_ d: Design, _ base: [String], _ dep: YNode) {
        let servers = items(dep.child("servers"))
        var named: [String: YNode] = [:]
        for s in servers {
            let n = str(s.child("name"))
            if !n.isEmpty && named[n] == nil { named[n] = s }
        }
        let linksNode = dep.child("links")
        var linked: Set<String> = []
        for (i, l) in items(linksNode).enumerated() {
            let at = base + ["links", "\(i)"]
            let from = l.child("from"), to = l.child("to")
            var ok = true
            for (key, end) in [("from", from), ("to", to)] {
                if let end, named[end.value] == nil {
                    add(end, pointer(at + [key]), .link, "\(end.value) is not a named server of this deployment\(suggest(end.value, named))")
                    ok = false
                }
            }
            guard ok, let from, let to else { continue }
            if from.value == to.value {
                add(to, pointer(at + ["to"]), .link, "the link goes from \(from.value) to itself; a server reaches itself with no link, so remove it")
                continue
            }
            linked.insert(from.value + "\u{0}" + to.value)
            if let port = l.child("port") {
                var ports: [String] = []
                var listened = false
                for ln in items(child(named[to.value], "listens")) {
                    let p = str(ln.child("port"))
                    ports.append(p)
                    if p == port.value { listened = true }
                }
                if listened {
                } else if ports.isEmpty {
                    add(port, pointer(at + ["port"]), .link, "\(to.value) listens on no port, so no link can reach it; add the port under its listens")
                } else {
                    add(port, pointer(at + ["port"]), .link, "\(to.value) does not listen on \(port.value); it listens on \(ports.joined(separator: ", "))")
                }
            }
        }
        // A must question on the environment the deployment installs holds
        // the links still to write (ADR-137, ADR-172).
        guard linksNode != nil, !d.mustBlockedEnvironment(str(dep.child("environment"))) else { return }
        // What each dependency is in this deployment, and its database.
        var depServer: [String: String] = [:]
        var database = ""
        for s in servers {
            let n = str(s.child("name"))
            if n.isEmpty { continue }
            let dn = str(s.child("dependency"))
            if !dn.isEmpty {
                if depServer[dn] == nil { depServer[dn] = n }
                continue
            }
            if str(s.child("kind")) == "database" && database.isEmpty
                && items(child(s.child("runs"), "targets")).contains(where: { $0.value == "sql" }) {
                database = n
            }
        }
        var layout: [String: YNode] = [:]
        for p in pairs(root.child("layout")) { layout[p.key.value] = p.value }
        for (i, s) in servers.enumerated() {
            let n = str(s.child("name"))
            if n.isEmpty || s.child("dependency") != nil { continue }
            let got = serverReach(d, layout, items(child(s.child("runs"), "packages")))
            let at = pointer(base + ["servers", "\(i)"])
            for dn in got.dependencies.keys.sorted(by: byteLess) {
                guard let target = depServer[dn], !linked.contains(n + "\u{0}" + target) else { continue }
                warn(s, at, .linkMissing, "\(n) runs the packages implementing \(got.dependencies[dn]!.joined(separator: ", ")), which reach the dependency \(dn), and no link leads from \(n) to \(target), the server that is \(dn) here; add a link from \(n) to \(target) on a port it listens on")
            }
            if !got.data.isEmpty && !database.isEmpty && database != n && !linked.contains(n + "\u{0}" + database) {
                warn(s, at, .linkMissing, "\(n) runs the packages implementing \(got.data.joined(separator: ", ")), which read or write the database, and no link leads from \(n) to \(database), the server that runs the sql target; add a link from \(n) to \(database) on a port it listens on")
            }
        }
    }

    /// Follows the design objects the packages implement: an operation or
    /// a job reaches the dependencies it calls and those carrying the
    /// channels it emits on or consumes from, a gateway those it forwards
    /// to and calls; an entity, or a job that
    /// reads or writes one, reaches the database.
    func serverReach(_ d: Design, _ layout: [String: YNode], _ packages: [YNode]) -> ServerReach {
        var r = ServerReach()
        func reached(_ dep: String, _ by: String) {
            if d.dependencies[dep] != nil && !(r.dependencies[dep] ?? []).contains(by) {
                r.dependencies[dep, default: []].append(by)
            }
        }
        func carriers(_ message: String, _ by: String) {
            let ch = String(message.split(separator: "/", maxSplits: 1, omittingEmptySubsequences: false).first ?? "")
            for e in items(child(d.channels[ch], "carriedBy")) { reached(e.value, by) }
        }
        func uses(_ n: YNode?, _ by: String) {
            for call in items(child(n, "calls")) {
                reached(String(call.value.split(separator: "#", maxSplits: 1, omittingEmptySubsequences: false).first ?? ""), by)
            }
            for e in items(child(n, "emits")) { carriers(e.value, by) }
        }
        for pkg in packages {
            for ref in items(child(layout[pkg.value], "implements")) {
                let tokens = ref.value.hasPrefix("#/") ? ref.value.dropFirst(2).split(separator: "/", omittingEmptySubsequences: false).map { unescapeToken(String($0)) } : [ref.value]
                let (node, ok) = resolve(d.root, tokens)
                if !ok || tokens.count < 2 { continue }
                switch tokens[0] {
                case "entities":
                    if !r.data.contains(tokens[1]) { r.data.append(tokens[1]) }
                case "jobs":
                    uses(node, tokens[1])
                    let consumes = str(child(node.child("trigger"), "consumes"))
                    if !consumes.isEmpty { carriers(consumes, tokens[1]) }
                    if (node.child("reads") != nil || node.child("writes") != nil) && !r.data.contains(tokens[1]) {
                        r.data.append(tokens[1])
                    }
                case "gateways":
                    uses(node, tokens[1])
                    for dep in forwardedDependencies(node) { reached(dep, tokens[1]) }
                case "paths":
                    if tokens.count == 3 {
                        uses(node, str(node.child("operationId")))
                        continue
                    }
                    for m in ["get", "post", "put", "patch", "delete"] {
                        if let op = node.child(m) { uses(op, str(op.child("operationId"))) }
                    }
                default:
                    break
                }
            }
        }
        return r
    }
}
