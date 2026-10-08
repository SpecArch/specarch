import Foundation

/// Keys that only a specification has. In an implementation file each is
/// reported as design_key instead of an unknown key.
let designKeys: Set<String> = [
    "specarch", "stages", "sources",
    "stakeholders", "needs", "requirements", "glossary", "assumptions",
    "enums", "entities", "permissions", "roles", "paths", "commands",
    "channels", "dependencies", "session", "pages", "algorithms", "tests",
    "environments", "release", "rollback", "migrations", "checks", "signoff",
    "relations", "constraints", "transitions", "operationId", "responses",
    "requestBody", "parameters", "permission", "formula", "examples",
    "properties", "primaryKey", "stateField", "messages", "payload",
    "calls", "idempotencyKey", "guard", "validity",
    "sensitivity", "atRest", "lookup", "audited", "deletion",
    "listOf", "limits", "problem", "errors", "jobs", "menus",
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
            if let last = path.last, namedMaps.contains(last) { return }
            for p in n.pairs where designKeys.contains(p.key.value) {
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
        for p in pairs(root.child("mappings")) {
            checkDesignRef(specRoot, p.key, p.key.value, pointer("mappings", p.key.value))
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

    /// Checks each deployment names an environment of the specification
    /// when it declares any, and gives values only to settings the
    /// specification declares and that are not secret.
    func checkDeployments(_ d: Design) {
        for p in pairs(root.child("deployments")) {
            let base = ["deployments", p.key.value]
            let envNode = p.value.child("environment")
            let env = str(envNode)
            if !env.isEmpty && d.environments[env] == nil {
                add(envNode, pointer(base + ["environment"]), .environment, "\(env) is not an environment of the specification\(suggest(env, d.environments))")
            } else if env.isEmpty && !d.environments.isEmpty {
                add(p.key, pointer(base), .environment, "the specification declares environments, so this deployment must say which one it installs; add environment with one of \(d.environments.keys.sorted(by: byteLess).joined(separator: ", "))")
            }
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
    return Decimal(string: v.value)
}
