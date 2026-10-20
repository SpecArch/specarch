import Foundation

/// The version of the shape of the document model prints.
let modelVersion = "0.1"

/// Where a node of the model starts: its key in a mapping, or the item
/// itself in a list, in the file it came from (ADR-138).
struct Position {
    let file: String
    let line: Int
    let column: Int
}

/// Maps the pointer of every node of a tree to its position: the root is
/// "/", as the problems file writes it, and a key a mapping repeats is left
/// out after its first entry, as the document's values leave it out. files
/// names the file of each node; a node it does not name is in file.
func positions(_ root: YNode?, _ files: [ObjectIdentifier: String], _ file: String) -> [String: Position] {
    var out: [String: Position] = [:]
    guard let root else { return out }
    func visit(_ start: YNode, _ value: YNode, _ ptr: String) {
        out[ptr.isEmpty ? "/" : ptr] = Position(file: files[ObjectIdentifier(start)] ?? file, line: start.line, column: start.column)
        switch value.kind {
        case .mapping:
            var seen = Set<String>()
            for p in value.pairs where !seen.contains(p.key.value) {
                seen.insert(p.key.value)
                visit(p.key, p.value, ptr + "/" + escapeToken(p.key.value))
            }
        case .sequence:
            for (i, item) in value.items.enumerated() { visit(item, item, ptr + "/\(i)") }
        case .scalar:
            break
        }
    }
    visit(root, root, "")
    return out
}

/// One resolved reference of a specification: the pointer of the entry
/// that refers, the pointer of the element it names, and the keyword that
/// made it (ADR-138).
struct ModelReference {
    let from: String
    let to: String
    let through: String
}

/// Where a keyword names an element of the specification by its name: a
/// path of keys, * for any key or item, and the kind of element named.
/// satisfies, verifies and decidedIn name a requirement or a decision
/// wherever they stand, and $ref a pointer. The Go build has the same list.
private let referencePlaces: [(path: [String], kind: String)] = [
    (["entities", "*", "relations", "*", "target"], "entity"),
    (["paths", "*", "*", "guard", "entity"], "entity"),
    (["commands", "*", "guard", "entity"], "entity"),
    (["paths", "*", "*", "listOf", "entity"], "entity"),
    (["pages", "*", "entity"], "entity"),
    (["jobs", "*", "reads"], "entity"),
    (["jobs", "*", "writes"], "entity"),
    (["commands", "*", "reads"], "entity"),
    (["commands", "*", "writes"], "entity"),
    (["workflows", "*", "subject"], "entity"),
    (["tests", "*", "entity"], "entity"),
    (["paths", "*", "*", "permission"], "permission"),
    (["commands", "*", "permission"], "permission"),
    (["pages", "*", "permission"], "permission"),
    (["pages", "*", "actions", "*", "permission"], "permission"),
    (["workflows", "*", "steps", "*", "permission"], "permission"),
    (["roles", "*", "permissions"], "permission"),
    (["jobs", "*", "role"], "role"),
    (["workflows", "*", "steps", "*", "approvers"], "role"),
    (["clients", "*", "roles"], "role"),
    (["signoff", "signers", "*", "role"], "role"),
    (["paths", "*", "*", "emits"], "message"),
    (["jobs", "*", "emits"], "message"),
    (["workflows", "*", "emits"], "message"),
    (["jobs", "*", "trigger", "consumes"], "message"),
    (["paths", "*", "*", "calls"], "dependency"),
    (["jobs", "*", "calls"], "dependency"),
    (["menus", "*", "page"], "page"),
    (["menus", "*", "items", "*", "page"], "page"),
    (["menus", "*", "items", "*", "items", "*", "page"], "page"),
    (["menus", "*", "items", "*", "items", "*", "items", "*", "page"], "page"),
    (["flows", "*", "steps", "*", "page"], "page"),
    (["tests", "*", "page"], "page"),
    (["workflows", "*", "trigger"], "operation"),
    (["workflows", "*", "steps", "*", "operation"], "operation"),
    (["tests", "*", "operation"], "operation"),
    (["tests", "*", "command"], "command"),
    (["paths", "*", "*", "callers"], "client"),
    (["commands", "*", "callers"], "client"),
    (["paths", "*", "*", "token"], "token"),
    (["clients", "*", "proof", "token"], "token"),
    (["paths", "*", "*", "handshake"], "handshake"),
    (["paths", "*", "*", "middlewares"], "middleware"),
    (["tests", "*", "job"], "job"),
    (["tests", "*", "workflow"], "workflow"),
    (["tests", "*", "flow"], "flow"),
    (["tests", "*", "requirement"], "requirement"),
]

/// The keywords that name an element wherever they stand.
private let anywhere = ["satisfies": "requirement", "verifies": "requirement", "decidedIn": "decision"]

private let sectionOfKind = [
    "requirement": "requirements", "decision": "decisions", "permission": "permissions", "role": "roles",
    "page": "pages", "command": "commands", "client": "clients", "token": "tokens", "handshake": "handshakes",
    "middleware": "middlewares", "job": "jobs", "workflow": "workflows", "flow": "flows",
]

private func placeMatches(_ pattern: [String], _ path: [String]) -> Bool {
    pattern.count == path.count && zip(pattern, path).allSatisfy { $0 == "*" || $0 == $1 }
}

/// Lists every reference of a specification that resolves to an element of
/// it, in document order: a $ref that is a pointer into the specification,
/// and a name under one of the reference keywords that names an element. A
/// reference that does not resolve is the validator's problem and is left
/// out; so is one into another specification.
func modelReferences(_ root: YNode?) -> [ModelReference] {
    guard let root, root.kind == .mapping else { return [] }
    let d = Design(root)
    var out: [ModelReference] = []
    func target(_ kind: String, _ nameIn: String) -> String {
        var name = nameIn
        var section = sectionOfKind[kind] ?? ""
        switch kind {
        case "entity":
            if d.entities[name] != nil { return pointer("entities", name) }
            section = "views"
        case "message":
            guard d.message(name) != nil, let slash = name.firstIndex(of: "/") else { return "" }
            return pointer("channels", String(name[..<slash]), "messages", String(name[name.index(after: slash)...]))
        case "dependency":
            section = "dependencies"
            if let hash = name.firstIndex(of: "#"), hash != name.startIndex { name = String(name[..<hash]) }
        case "operation":
            return d.operations[name]?.pointer() ?? ""
        default:
            break
        }
        if name.isEmpty || child(child(root, section), name) == nil { return "" }
        return pointer(section, name)
    }
    func names(_ n: YNode, _ at: [String], _ kind: String, _ through: String) {
        func add(_ item: YNode, _ ptr: [String]) {
            let to = target(kind, item.str)
            if !to.isEmpty { out.append(ModelReference(from: pointer(ptr), to: to, through: through)) }
        }
        if n.kind == .sequence {
            for (i, item) in n.items.enumerated() { add(item, at + ["\(i)"]) }
            return
        }
        add(n, at)
    }
    walk(root, []) { n, path in
        for p in n.pairs {
            let key = p.key.value
            let at = path + [key]
            if key == "$ref" {
                let ref = p.value.str
                if ref.hasPrefix("#/"), resolve(root, pointerTokens(String(ref.dropFirst()))).1 {
                    out.append(ModelReference(from: pointer(at), to: String(ref.dropFirst()), through: key))
                }
                continue
            }
            if let kind = anywhere[key] {
                names(p.value, at, kind, key)
                continue
            }
            if let place = referencePlaces.first(where: { placeMatches($0.path, at) }) {
                names(p.value, at, place.kind, key)
            }
        }
    }
    return out
}

/// Lists the references of an implementation file that resolve in the
/// specification it implements: each design pointer under a layout entry's
/// implements and a mapping's design, with the keyword that holds it.
func implementationReferences(_ impl: YNode?, _ design: YNode?) -> [ModelReference] {
    guard let impl, let design else { return [] }
    var out: [ModelReference] = []
    walk(impl, []) { n, path in
        for p in n.pairs {
            let key = p.key.value
            guard key == "implements" || key == "design" && path.first == "mappings" else { continue }
            let at = path + [key]
            func add(_ item: YNode, _ ptr: [String]) {
                let ref = item.str
                if ref.hasPrefix("#/"), resolve(design, pointerTokens(String(ref.dropFirst()))).1 {
                    out.append(ModelReference(from: pointer(ptr), to: String(ref.dropFirst()), through: key))
                }
            }
            if p.value.kind == .sequence {
                for (i, item) in p.value.items.enumerated() { add(item, at + ["\(i)"]) }
            } else {
                add(p.value, at)
            }
        }
    }
    return out
}

/// The problems found while reading a specification, placed as the
/// problems file places them.
func layoutProblems(_ s: Spec) -> [Diagnostic] {
    let c = Checker(file: s.rootPath, files: s.files)
    for p in s.problems { c.addFile(p.file, p.line, p.path, Rule(rawValue: p.rule)!, p.message) }
    var ds = c.diags
    Placer(spec: s).place(&ds)
    sortDiagnostics(&ds)
    return ds
}

/// The problems found while reading one file on its own: what of its YAML
/// does not parse. Their ids name the file from dir.
func readProblems(_ path: String, _ dir: String, _ doc: Doc) -> [Diagnostic] {
    let c = Checker(file: path)
    for p in doc.problems { c.addLine(p.line, p.path, Rule(rawValue: p.rule)!, p.message) }
    var ds = c.diags
    Placer(dir: dir).place(&ds)
    sortDiagnostics(&ds)
    return ds
}

/// The model command: the specifications found under the paths given as
/// one JSON document in RFC 8785's canonical form (ADR-138).
func runModel(_ argsIn: [String], _ stdout: TextSink, _ stderr: TextSink) -> Int32 {
    var args = argsIn
    if args.first == "--" { args.removeFirst() }
    if args.isEmpty {
        stderr.write("specarch model needs at least one folder or file\n\n\(usage)")
        return 2
    }
    for a in args where a.count > 1 && a.hasPrefix("-") {
        stderr.write("specarch model has no option \(a); to read a path whose name starts with -, write -- before it\n\n\(usage)")
        return 2
    }
    let (inputs, ioError) = collect(args, stderr)
    if ioError { return 2 }
    var specs: [(String, Canon)] = [], impls: [(String, Canon)] = [], estates: [(String, Canon)] = []
    var byFolder: [String: Spec] = [:]
    for input in inputs {
        if case .root(let dir) = input {
            let s = Spec(dir: dir)
            byFolder[cleanPath(dir)] = s
            specs.append((s.rootPath, modelSpec(s)))
        }
    }
    for input in inputs {
        switch input {
        case .implementation(let path):
            let data: Data
            do { data = try readFile(path) } catch {
                stderr.write("specarch: cannot read \(path): \(plainIOError(error))\n")
                return 2
            }
            impls.append((path, modelImplementation(path, dirPath(path), data, nil)))
        case .estate(let e):
            estates.append((e.path, modelEstate(e, byFolder)))
        default:
            break
        }
    }
    func sorted(_ list: [(String, Canon)]) -> Canon {
        .array(list.enumerated().sorted { a, b in
            a.element.0 != b.element.0 ? byteLess(a.element.0, b.element.0) : a.offset < b.offset
        }.map { $0.element.1 })
    }
    let doc: Canon = .object([
        "specarchModel": .string(modelVersion),
        "program": .string(programVersion),
        "specifications": sorted(specs),
        "implementations": sorted(impls),
        "estates": sorted(estates),
    ])
    stdout.write(canonicalJSON(doc) + "\n")
    return 0
}

private func scalarText(_ n: YNode?) -> Canon {
    guard let n, n.kind == .scalar else { return .null }
    return .string(n.value)
}

private func modelSpec(_ s: Spec) -> Canon {
    let info = child(s.root, "info")
    return .object([
        "root": .string(s.rootPath),
        "name": scalarText(child(info, "name")),
        "version": scalarText(child(info, "version")),
        "metaModel": scalarText(child(s.root, "specarch")),
        "document": Canon(s.value),
        "positions": modelPositions(positions(s.root, s.files, s.rootPath)),
        "references": canonReferences(modelReferences(s.root)),
        "problems": canonProblems(layoutProblems(s)),
        "implementations": .array(s.implementations.map { modelImplementation($0.path, s.dir, $0.data, s.root) }),
    ])
}

/// One implementation file: of a specification when design is its merged
/// document, or on its own.
private func modelImplementation(_ path: String, _ dir: String, _ data: Data, _ design: YNode?) -> Canon {
    let doc = parseYAML(String(decoding: data, as: UTF8.self))
    let base = (path as NSString).lastPathComponent
    let name = String(base.dropLast(implementationSuffix.count))
    let stack = name.lastIndex(of: ".").map { String(name[name.index(after: $0)...]) } ?? name
    return .object([
        "file": .string(path),
        "stack": .string(stack),
        "document": Canon(doc.value),
        "positions": modelPositions(positions(doc.root, [:], path)),
        "references": canonReferences(implementationReferences(doc.root, design)),
        "problems": canonProblems(readProblems(path, dir, doc)),
    ])
}

private func modelEstate(_ e: Estate, _ byFolder: [String: Spec]) -> Canon {
    .object([
        "file": .string(e.path),
        "modules": .array(e.modules.map { m in
            var resolved: Canon = .null
            if !m.location.isEmpty, let s = byFolder[cleanPath(m.location)] { resolved = .string(s.rootPath) }
            return .object([
                "name": .string(m.key),
                "location": .string(str(child(m.value, "location"))),
                "version": .string(m.version),
                "specification": resolved,
            ])
        }),
    ])
}

private func modelPositions(_ ps: [String: Position]) -> Canon {
    .object(ps.mapValues { .object(["file": .string($0.file), "line": .number(Double($0.line)), "column": .number(Double($0.column))]) })
}

private func canonReferences(_ rs: [ModelReference]) -> Canon {
    .array(rs.map { .object(["from": .string($0.from), "to": .string($0.to), "through": .string($0.through)]) })
}

private func canonProblems(_ ds: [Diagnostic]) -> Canon {
    .array(ds.map {
        .object([
            "file": .string($0.file), "line": .number(Double($0.line)), "column": .number(Double($0.column)),
            "severity": .string($0.severity.rawValue), "path": .string($0.path), "rule": .string($0.rule.rawValue),
            "message": .string($0.message), "id": .string($0.id),
        ])
    })
}
