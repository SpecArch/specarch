import Foundation

/// Keys that only a design file has. In an implementation file each is
/// reported as design_key instead of an unknown key.
let designKeys: Set<String> = [
    "specarch", "requirementSources", "enums", "entities", "permissions",
    "roles", "paths", "commands", "channels", "pages", "algorithms",
    "relations", "constraints", "transitions", "operationId", "responses",
    "requestBody", "parameters", "permission", "formula", "examples",
    "properties", "primaryKey", "stateField", "messages", "payload",
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
        return String(cString: message)
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
            if path.contains("settings") { return } // free-form generator and framework settings
            for p in n.pairs where designKeys.contains(p.key.value) {
                add(p.key, pointer(path + [p.key.value]), .designKey,
                    "\(p.key.value) belongs to the design, not to one implementation; move it to the design file (*\(designSuffix))")
            }
        }
    }

    func checkImplementation(_ read: ReadFile) {
        let impl = root.child("implements")
        let fileNode = child(impl, "file")
        let rel = str(fileNode)
        if rel.isEmpty || !rel.hasSuffix(designSuffix) { return } // the schema reports it
        let designPath = joinPath(dirPath(file), rel)
        let data: Data
        do {
            data = try read(designPath)
        } catch {
            add(fileNode, "/implements/file", .implements, "the design file \(rel) cannot be read (\(plainIOError(error))); correct the path, which is relative to this file")
            return
        }
        let doc = parseYAML(String(decoding: data, as: UTF8.self))
        guard let designRoot = doc.root, designRoot.child("specarch") != nil else {
            add(fileNode, "/implements/file", .implements, "\(rel) is not a valid design file; run specarch validate on it first")
            return
        }
        let verNode = child(impl, "version")
        let want = str(child(designRoot.child("info"), "version"))
        let got = str(verNode)
        if !got.isEmpty && got != want {
            add(verNode, "/implements/version", .implements,
                "this file implements version \(got) of \(rel), but that file is now version \(want); review the design change, then update this version")
        }
        let d = Design(designRoot)
        for p in pairs(root.child("layout")) {
            for (i, ref) in items(p.value.child("implements")).enumerated() {
                checkDesignRef(designRoot, ref, ref.value, pointer("layout", p.key.value, "implements", "\(i)"), rel)
            }
        }
        for p in pairs(root.child("mappings")) {
            checkDesignRef(designRoot, p.key, p.key.value, pointer("mappings", p.key.value), rel)
        }
        for p in pairs(root.child("decisions")) where d.decisions[p.key.value] != nil {
            add(p.key, pointer("decisions", p.key.value), .decision,
                "\(p.key.value) is already a decision of the design file \(rel); give this implementation decision its own number")
        }
        checkRequirementLinks(root.child("decisions"), ["decisions"], d.sources)
        checkSuites(d, rel)
    }

    /// Checks every suite runs design tests that exist, or says it is
    /// implementation-only.
    func checkSuites(_ d: Design, _ rel: String) {
        var tests: [String: YNode] = [:]
        for p in pairs(d.root.child("tests")) { tests[p.key.value] = p.value }
        for s in pairs(child(root.child("testing"), "suites")) {
            let base = ["testing", "suites", s.key.value]
            for (i, n) in items(s.value.child("designTests")).enumerated() where tests[n.value] == nil {
                add(n, pointer(base + ["designTests", "\(i)"]), .suite, "\(n.value) is not a test of the design file \(rel)\(suggest(n.value, tests))")
            }
            for (i, subject) in items(s.value.child("designTestsOf")).enumerated() {
                for p in pairs(subject) {
                    let key = p.key.value + ": " + p.value.value
                    if !tests.values.contains(where: { testSubjectKey($0) == key }) {
                        add(p.value, pointer(base + ["designTestsOf", "\(i)", p.key.value]), .suite,
                            "the design file \(rel) has no test about \(p.key.value) \(p.value.value); name a subject that has tests")
                    }
                }
            }
        }
    }

    func checkDesignRef(_ designRoot: YNode, _ at: YNode, _ ref: String, _ ptr: String, _ rel: String) {
        guard ref.hasPrefix("#/") else { return } // the schema reports it
        let tokens = ref.dropFirst(2).split(separator: "/", omittingEmptySubsequences: false).map { unescapeToken(String($0)) }
        if !resolve(designRoot, tokens).1 {
            add(at, ptr, .designRef, "\(ref) does not point at anything in \(rel); correct the pointer (a / inside a name is written ~1)")
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

let proseKeys: Set<String> = ["description", "summary", "title", "context", "decision", "consequences", "note", "message"]

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
