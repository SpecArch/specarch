import Foundation

/// The name of a specification's root file.
public let rootFile = "specarch.yaml"

/// The file each test folder holds.
public let testFile = "test.yaml"

/// The life-cycle stages in order; each is a folder name.
public let stages = ["requirements", "design", "implementation", "tests", "deployment", "commissioning", "operation"]

/// The one section every stage folder may hold: a question is written with
/// what it is about, and questions arise at every stage. The loader merges
/// them into one section.
public let questionsSection = "questions"

/// Every section and its stage.
public let sections: [String: String] = [
    "stakeholders": "requirements", "needs": "requirements", "requirements": "requirements",
    "glossary": "requirements", "assumptions": "requirements", "constraints": "requirements",
    "enums": "design", "entities": "design", "permissions": "design", "roles": "design", "paths": "design",
    "commands": "design", "channels": "design", "pages": "design", "algorithms": "design", "decisions": "design",
    "tests": "tests",
    "environments": "deployment", "configuration": "deployment", "release": "deployment",
    "rollback": "deployment", "migrations": "deployment",
    "checks": "commissioning", "signoff": "commissioning",
    "monitors": "operation",
]

/// The order sections take in the merged document: life-cycle order, the
/// questions last.
let sectionOrder = [
    "stakeholders", "needs", "requirements", "glossary", "assumptions", "constraints",
    "enums", "entities", "permissions", "roles", "paths", "commands", "channels", "pages", "algorithms",
    "tests", "decisions",
    "environments", "configuration", "release", "rollback", "migrations",
    "checks", "signoff",
    "monitors",
    questionsSection,
]

/// Sections that hold one object, not a map of named objects, so they
/// cannot be split across files.
let singleSections: Set<String> = ["release", "rollback", "signoff"]

/// The keys that live only in the root file.
let rootOnly: Set<String> = ["specarch", "info", "stages", "sources"]

/// The sections of a stage, in order.
func sectionsOf(_ stage: String) -> [String] {
    sectionOrder.filter { sections[$0] == stage }
}

/// Something wrong with a file or the layout, found while reading.
struct SpecProblem {
    let file: String
    let line: Int
    let path: String
    let rule: String
    let message: String
}

/// One implementation file of a specification.
struct SpecImplementation {
    let path: String
    let stack: String
    let data: Data
}

/// One specification as read from disk: the root file and the stage folders
/// beside it, merged into one document. Every node remembers its file.
final class Spec {
    let dir: String
    let rootPath: String
    var root: YNode?
    var value: JSONValue = .null
    var files: [ObjectIdentifier: String] = [:]
    var problems: [SpecProblem] = []
    var listedStages: [String] = []
    var implementations: [SpecImplementation] = []

    /// Reads the specification rooted at dir.
    init(dir: String) {
        self.dir = dir
        rootPath = joinPath(dir, rootFile)
        load()
    }

    private func load() {
        let data: Data
        do {
            data = try readFile(rootPath)
        } catch {
            problem(rootPath, 1, "/", "layout", "cannot read \(rootFile) (\(plainIOError(error)))")
            return
        }
        let doc = parseYAML(String(decoding: data, as: UTF8.self))
        for p in doc.problems { problem(rootPath, p.line, p.path, p.rule, p.message) }
        guard let docRoot = doc.root else { return }
        record(docRoot, rootPath)
        if docRoot.kind != .mapping {
            root = docRoot
            value = doc.value
            return
        }
        let merged = YNode(kind: .mapping, line: docRoot.line)
        files[ObjectIdentifier(merged)] = rootPath
        var found: [String: YNode] = [:]
        var listed = Set<String>()
        for item in items(docRoot.child("stages")) {
            listed.insert(item.value)
            listedStages.append(item.value)
        }
        // The root file's own keys: specarch, info, stages, sources, and the
        // sections of stages that have no folder.
        for p in docRoot.pairs {
            let key = p.key.value
            if key == questionsSection {
                mergeSection(&found, key, p.value, rootPath)
                continue
            }
            let stage = sections[key]
            if let stage, listed.contains(stage) {
                problem(rootPath, p.key.line, pointer(key), "layout",
                        "\(key) belongs to the \(stage) stage, which stages lists, so it lives under \(stage)/ and not in \(rootFile); move it there")
                continue
            }
            if stage != nil && !singleSections.contains(key) {
                mergeSection(&found, key, p.value, rootPath)
                continue
            }
            merged.pairs.append(p)
            if stage != nil { found[key] = p.value }
        }
        // The stage folders.
        let entries: [Entry]
        do {
            entries = try readDir(dir)
        } catch {
            problem(rootPath, 1, "/", "layout", "cannot read the folder \(dir) (\(plainIOError(error)))")
            entries = []
        }
        var present = Set<String>()
        for e in entries where e.isDir && !e.name.hasPrefix(".") {
            present.insert(e.name)
            if listed.contains(e.name) { continue }
            let stage = sections[e.name] ?? ""
            if stages.contains(e.name) {
                problem(rootPath, 1, "/stages", "layout", "the folder \(e.name)/ exists but stages in \(rootFile) does not list \(e.name); add it to stages, or move its files into \(rootFile)")
            } else if !stage.isEmpty && listed.contains(stage) {
                problem(rootPath, 1, "/", "layout", "the folder \(e.name)/ is a section of the \(stage) stage, not a stage; move it to \(stage)/\(e.name)/")
            } else if !stage.isEmpty {
                problem(rootPath, 1, "/", "layout", "the folder \(e.name)/ is a section of the \(stage) stage, not a stage; move it to \(stage)/\(e.name)/ and list \(stage) in stages")
            } else {
                problem(rootPath, 1, "/", "layout", "the folder \(e.name)/ is not a stage; a specification's folders are \(stages.joined(separator: ", "))")
            }
        }
        // Implementation files beside the root file belong to the
        // specification when the implementation stage has no folder, as
        // sections belong in the root file when their stage has none.
        for e in entries where !e.isDir && e.name.hasSuffix(implementationSuffix) {
            let p = joinPath(dir, e.name)
            if listed.contains("implementation") {
                problem(p, 1, "/", "layout", "stages lists implementation, so an implementation file lives under implementation/<stack>/; move it there")
                continue
            }
            let data: Data
            do { data = try readFile(p) } catch {
                problem(p, 1, "/", "layout", "cannot read it (\(plainIOError(error)))")
                continue
            }
            let name = String(e.name.dropLast(implementationSuffix.count))
            implementations.append(SpecImplementation(path: p, stack: lastDotPart(name), data: data))
        }
        for stage in listedStages {
            if !present.contains(stage) {
                problem(rootPath, 1, "/stages", "layout", "stages lists \(stage) but there is no \(stage)/ folder beside \(rootFile); create it, or remove \(stage) from stages")
                continue
            }
            let folder = joinPath(dir, stage)
            switch stage {
            case "tests": loadTests(folder, &found)
            case "implementation": loadImplementations(folder, &found)
            default: loadStage(stage, folder, &found)
            }
        }
        // Sections in life-cycle order after the root file's own keys.
        for name in sectionOrder {
            guard let n = found[name], merged.key(name) == nil else { continue }
            let key = YNode(kind: .scalar, value: name, tag: "!!str", line: n.line)
            files[ObjectIdentifier(key)] = files[ObjectIdentifier(n)]
            merged.pairs.append((key, n))
        }
        root = merged
        value = valueOf(merged)
    }

    /// Adds the entries of one file's section mapping to the merged section,
    /// reporting a name already defined by another file.
    private func mergeSection(_ found: inout [String: YNode], _ name: String, _ value: YNode, _ file: String) {
        var target = found[name]
        if target == nil {
            let t = YNode(kind: .mapping, line: value.line)
            files[ObjectIdentifier(t)] = file
            found[name] = t
            target = t
        }
        if value.kind != .mapping {
            // Not a mapping: keep it so the schema reports the shape, unless
            // a mapping is already there.
            if target!.pairs.isEmpty {
                found[name] = value
            } else {
                problem(file, value.line, pointer(name), "layout", "\(name) must be a mapping of named objects here")
            }
            return
        }
        guard let t = found[name], t.kind == .mapping else { return }
        for p in value.pairs {
            if let first = t.key(p.key.value) {
                problem(file, p.key.line, pointer(name, p.key.value), "duplicate_key",
                        "\(p.key.value) is already defined in \(fileOf(first)) on line \(first.line); a name is defined once in the whole specification")
                continue
            }
            t.pairs.append(p)
        }
    }

    /// Reads every YAML file under a stage folder. A file holds only
    /// sections of its stage; under a sub-folder named after a section, only
    /// that section.
    private func loadStage(_ stage: String, _ folder: String, _ found: inout [String: YNode]) {
        let allowed = sectionsOf(stage)
        let walked: [String]
        do {
            walked = try walkFiles(folder)
        } catch {
            problem(folder, 1, "/", "layout", "cannot read the folder (\(plainIOError(error)))")
            return
        }
        for rel in walked {
            let p = joinPath(folder, rel)
            let name = (rel as NSString).lastPathComponent
            if name.hasSuffix(".yml") {
                problem(p, 1, "/", "layout", "a SpecArch file ends in .yaml; rename it")
                continue
            }
            if !name.hasSuffix(".yaml") { continue }
            var only = ""
            for part in dirPath(rel).split(separator: "/") where sections[String(part)] == stage || String(part) == questionsSection {
                only = String(part)
            }
            let data: Data
            do { data = try readFile(p) } catch {
                problem(p, 1, "/", "layout", "cannot read it (\(plainIOError(error)))")
                continue
            }
            let doc = parseYAML(String(decoding: data, as: UTF8.self))
            for pr in doc.problems { problem(p, pr.line, pr.path, pr.rule, pr.message) }
            guard let docRoot = doc.root else { continue }
            record(docRoot, p)
            if docRoot.kind != .mapping {
                problem(p, docRoot.line, "/", "layout", "a file of the \(stage) stage is a mapping of sections (\(allowed.joined(separator: ", ")))")
                continue
            }
            for pair in docRoot.pairs {
                let key = pair.key.value
                let keyStage = sections[key] ?? ""
                if !only.isEmpty && key != only {
                    problem(p, pair.key.line, pointer(key), "layout", "a file under \(stage)/\(only)/ holds only \(only); move \(key) to \(stage)/\(keyStage == stage || key == questionsSection ? key : "<section>")/")
                } else if key == questionsSection {
                    mergeSection(&found, key, pair.value, p)
                } else if keyStage == stage && singleSections.contains(key) {
                    if let first = found[key] {
                        problem(p, pair.key.line, pointer(key), "duplicate_key", "\(key) is already defined in \(fileOf(first)) on line \(first.line); it is one object, written in one file")
                        continue
                    }
                    found[key] = pair.value
                } else if keyStage == stage {
                    mergeSection(&found, key, pair.value, p)
                } else if !keyStage.isEmpty {
                    problem(p, pair.key.line, pointer(key), "layout", "\(key) belongs to the \(keyStage) stage, not to \(stage); move it under \(keyStage)/")
                } else if rootOnly.contains(key) {
                    problem(p, pair.key.line, pointer(key), "layout", "\(key) is written only in \(rootFile); remove it here")
                } else {
                    problem(p, pair.key.line, pointer(key), "layout", "\(key) is not a section of the \(stage) stage; the sections are \(allowed.joined(separator: ", "))")
                }
            }
        }
    }

    /// Reads tests/<name>/test.yaml for every test folder.
    private func loadTests(_ folder: String, _ found: inout [String: YNode]) {
        let entries: [Entry]
        do {
            entries = try readDir(folder)
        } catch {
            problem(folder, 1, "/", "layout", "cannot read the folder (\(plainIOError(error)))")
            return
        }
        let tests = YNode(kind: .mapping, line: 1)
        files[ObjectIdentifier(tests)] = rootPath
        for e in entries where !e.name.hasPrefix(".") {
            let p = joinPath(folder, e.name)
            if !e.isDir {
                if e.name.hasSuffix(".yaml") && loadQuestionsFile(p, &found) { continue }
                if e.name.hasSuffix(".yaml") || e.name.hasSuffix(".yml") {
                    problem(p, 1, "/", "layout", "a file directly under tests/ holds only questions; each test is a folder tests/<name>/ holding \(testFile)")
                }
                continue
            }
            let tf = joinPath(p, testFile)
            guard let data = try? readFile(tf) else {
                problem(p, 1, "/", "layout", "the test folder tests/\(e.name) has no \(testFile); add one with the test's subject, scenario, level, given, when and then")
                continue
            }
            let doc = parseYAML(String(decoding: data, as: UTF8.self))
            for pr in doc.problems { problem(tf, pr.line, pr.path, pr.rule, pr.message) }
            guard let docRoot = doc.root else { continue }
            record(docRoot, tf)
            let key = YNode(kind: .scalar, value: e.name, tag: "!!str", line: max(docRoot.line, 1))
            files[ObjectIdentifier(key)] = tf
            tests.pairs.append((key, docRoot))
        }
        if !tests.pairs.isEmpty { found["tests"] = tests }
    }

    /// Reads a YAML file directly under tests/ or implementation/, whose
    /// entries are otherwise folders: it holds only questions. Returns
    /// whether the file was one.
    private func loadQuestionsFile(_ p: String, _ found: inout [String: YNode]) -> Bool {
        guard let data = try? readFile(p) else { return false }
        let doc = parseYAML(String(decoding: data, as: UTF8.self))
        guard let docRoot = doc.root, docRoot.kind == .mapping else { return false }
        for pair in docRoot.pairs where pair.key.value != questionsSection { return false }
        for pr in doc.problems { problem(p, pr.line, pr.path, pr.rule, pr.message) }
        record(docRoot, p)
        for pair in docRoot.pairs { mergeSection(&found, questionsSection, pair.value, p) }
        return true
    }

    /// Which stage folder a file of the specification sits under, or "" for
    /// the root file and the files beside it.
    func stageOfFile(_ file: String) -> String {
        let base = cleanPath(dir)
        let clean = cleanPath(file)
        var rel: String
        if base == "." {
            rel = clean.hasPrefix("./") ? String(clean.dropFirst(2)) : clean
        } else if clean.hasPrefix(base + "/") {
            rel = String(clean.dropFirst(base.count + 1))
        } else {
            return ""
        }
        let first = String(rel.split(separator: "/").first ?? "")
        return stages.contains(first) ? first : ""
    }

    /// Whether the root file lists a stage, so that it has a folder.
    func listed(_ stage: String) -> Bool { listedStages.contains(stage) }

    /// Finds implementation/<stack>/<name>.<stack>.specarch-implementation.yaml.
    private func loadImplementations(_ folder: String, _ found: inout [String: YNode]) {
        let entries: [Entry]
        do {
            entries = try readDir(folder)
        } catch {
            problem(folder, 1, "/", "layout", "cannot read the folder (\(plainIOError(error)))")
            return
        }
        for e in entries where !e.name.hasPrefix(".") {
            let p = joinPath(folder, e.name)
            if !e.isDir {
                if e.name.hasSuffix(".yaml") && loadQuestionsFile(p, &found) { continue }
                if e.name.hasSuffix(".yaml") || e.name.hasSuffix(".yml") {
                    problem(p, 1, "/", "layout", "a file directly under implementation/ holds only questions; an implementation file lives in implementation/<stack>/, named <name>.<stack>\(implementationSuffix)")
                }
                continue
            }
            let stack = e.name
            let inner: [Entry]
            do {
                inner = try readDir(p)
            } catch {
                problem(p, 1, "/", "layout", "cannot read the folder (\(plainIOError(error)))")
                continue
            }
            var count = 0
            for f in inner {
                let fp = joinPath(p, f.name)
                if f.isDir || !f.name.hasSuffix(".yaml") && !f.name.hasSuffix(".yml") { continue }
                if !f.name.hasSuffix(implementationSuffix) {
                    problem(fp, 1, "/", "layout", "a file under implementation/\(stack)/ is an implementation file named <name>.\(stack)\(implementationSuffix); rename it, or move it to its stage")
                    continue
                }
                count += 1
                let name = String(f.name.dropLast(implementationSuffix.count))
                if !name.contains(".") || lastDotPart(name) != stack {
                    problem(fp, 1, "/", "layout", "the folder says the stack is \(stack) but the file name says \(lastDotPart(name)); make them agree")
                    continue
                }
                let data: Data
                do { data = try readFile(fp) } catch {
                    problem(fp, 1, "/", "layout", "cannot read it (\(plainIOError(error)))")
                    continue
                }
                implementations.append(SpecImplementation(path: fp, stack: stack, data: data))
            }
            if count == 0 {
                problem(p, 1, "/", "layout", "the folder implementation/\(stack)/ holds no implementation file; add <name>.\(stack)\(implementationSuffix) or remove the folder")
            }
        }
    }

    /// Remembers the file of every node under n.
    private func record(_ n: YNode, _ file: String) {
        files[ObjectIdentifier(n)] = file
        for p in n.pairs {
            record(p.key, file)
            record(p.value, file)
        }
        for i in n.items { record(i, file) }
    }

    /// The file a node came from, or the root file.
    func fileOf(_ n: YNode) -> String {
        files[ObjectIdentifier(n)] ?? rootPath
    }

    private func problem(_ file: String, _ line: Int, _ path: String, _ rule: String, _ message: String) {
        problems.append(SpecProblem(file: file, line: max(line, 1), path: path, rule: rule, message: message))
    }

    /// Whether the specification has any section of a stage.
    func covers(_ stage: String) -> Bool {
        guard let root else { return false }
        return sectionsOf(stage).contains { root.child($0) != nil }
    }
}

/// The part of a name after its last dot, or the whole name.
private func lastDotPart(_ s: String) -> String {
    guard let dot = s.lastIndex(of: ".") else { return s }
    return String(s[s.index(after: dot)...])
}

/// One entry of a folder.
struct Entry {
    let name: String
    let isDir: Bool
}

/// The entries of a folder, sorted by name as Go's os.ReadDir gives them.
func readDir(_ dir: String) throws -> [Entry] {
    var isDir: ObjCBool = false
    guard FileManager.default.fileExists(atPath: dir, isDirectory: &isDir) else {
        throw NSError(domain: NSPOSIXErrorDomain, code: Int(ENOENT), userInfo: [NSUnderlyingErrorKey: NSError(domain: NSPOSIXErrorDomain, code: Int(ENOENT))])
    }
    guard isDir.boolValue else {
        throw NSError(domain: NSPOSIXErrorDomain, code: Int(ENOTDIR), userInfo: [NSUnderlyingErrorKey: NSError(domain: NSPOSIXErrorDomain, code: Int(ENOTDIR))])
    }
    let names = try FileManager.default.contentsOfDirectory(atPath: dir).sorted(by: byteLess)
    return names.map { name in
        var sub: ObjCBool = false
        FileManager.default.fileExists(atPath: joinPath(dir, name), isDirectory: &sub)
        return Entry(name: name, isDir: sub.boolValue)
    }
}

/// Every file under a folder, as slash paths relative to it, in the order
/// Go's filepath.WalkDir visits them; folders whose name starts with a dot
/// are skipped.
func walkFiles(_ folder: String) throws -> [String] {
    var out: [String] = []
    func visit(_ rel: String) throws {
        for e in try readDir(rel.isEmpty ? folder : joinPath(folder, rel)) {
            let r = rel.isEmpty ? e.name : rel + "/" + e.name
            if e.isDir {
                if e.name.hasPrefix(".") { continue }
                try visit(r)
            } else {
                out.append(r)
            }
        }
    }
    try visit("")
    return out
}

/// The specification a file belongs to: the nearest ancestor folder holding
/// specarch.yaml, or nil.
func rootOf(_ path: String) -> String? {
    var dir = dirPath(path)
    while true {
        if FileManager.default.fileExists(atPath: joinPath(dir, rootFile)) { return dir }
        let parent = dirPath(dir)
        if parent == dir { return nil }
        dir = parent
    }
}

/// The specifications and the standalone implementation files under a
/// folder: a folder holding specarch.yaml is a specification, and nothing
/// under it is searched further.
func findSpecs(_ dir: String) throws -> (roots: [String], implementations: [String]) {
    var roots: [String] = [], impls: [String] = []
    func visit(_ d: String) throws {
        if FileManager.default.fileExists(atPath: joinPath(d, rootFile)) {
            roots.append(d)
            return
        }
        for e in try readDir(d) {
            let p = joinPath(d, e.name)
            if e.isDir {
                if e.name.hasPrefix(".") { continue }
                try visit(p)
            } else if e.name.hasSuffix(implementationSuffix) {
                impls.append(p)
            }
        }
    }
    try visit(dir)
    return (roots.sorted(by: byteLess), impls.sorted(by: byteLess))
}

/// A node tree as plain JSON values, reporting nothing: a repeated key keeps
/// its first value, and a date is its text.
func valueOf(_ n: YNode) -> JSONValue {
    switch n.kind {
    case .mapping:
        var out: [(String, JSONValue)] = []
        var seen = Set<String>()
        for p in n.pairs where !seen.contains(p.key.value) {
            seen.insert(p.key.value)
            out.append((p.key.value, valueOf(p.value)))
        }
        return .object(out)
    case .sequence:
        return .array(n.items.map(valueOf))
    case .scalar:
        return scalarValue(n)
    }
}

/// One entry of a question's blocks: a stage, a section, or a pointer to
/// an element of the specification or to one key of it.
struct Block {
    var stage = ""          // the stage the block is about
    var section = ""        // the section, for a section or a pointer
    var tokens: [String] = [] // for a pointer: section, name, and the keys below

    var isPointer: Bool { !tokens.isEmpty }

    /// The pointer of the element the block is about, such as /entities/Loan,
    /// or "" for a stage or a section.
    var element: String { tokens.count < 2 ? "" : pointer(Array(tokens[0..<2])) }

    /// The key of the element the block is about, when it names one.
    var key: String { tokens.count < 3 ? "" : tokens[tokens.count - 1] }

    /// The sections the block holds up: every section of its stage, or the
    /// one section.
    var sectionsHeld: [String] { section.isEmpty ? sectionsOf(stage) : [section] }
}

/// Reads one entry of a question's blocks; nil when the text is neither a
/// stage, a section, nor a pointer whose first token is a section. Whether
/// the pointer resolves is the validator's check.
func parseBlock(_ s: String) -> Block? {
    if stages.contains(s) { return Block(stage: s) }
    if let stage = sections[s] { return Block(stage: stage, section: s) }
    guard s.hasPrefix("#/") else { return nil }
    let tokens = s.dropFirst(2).split(separator: "/", omittingEmptySubsequences: false).map { unescapeToken(String($0)) }
    guard tokens.count >= 2, let stage = sections[tokens[0]] else { return nil }
    return Block(stage: stage, section: tokens[0], tokens: tokens)
}
