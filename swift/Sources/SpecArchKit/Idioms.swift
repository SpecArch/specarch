import Foundation

/// Ends the name of every idiom file.
public let idiomSuffix = ".specarch-idiom.yaml"

/// One idiom file, parsed.
struct Idiom {
    let path: String // idioms/<concern>/<name>.specarch-idiom.yaml for a shipped one, the file for a project's
    let root: YNode

    var name: String { str(root.child("name")) }
    var version: String { str(root.child("version")) }
}

/// The idioms this build ships, by name.
nonisolated(unsafe) let shippedIdioms: [String: Idiom] = { // read-only once built
    var out: [String: Idiom] = [:]
    for f in shippedIdiomFiles {
        guard let root = parseYAML(f.text).root else { continue }
        let i = Idiom(path: f.path, root: root)
        out[i.name] = i
    }
    return out
}()

/// What a type row matches a field on.
struct FieldShape {
    var type = "", format = ""
    var isEnum = false
    var maxLength: Int?
    var precision: Int?
    var itemsType = "", itemsFormat = ""

    /// Reads a field's shape: its JSON type with null dropped, a $ref to an
    /// enum as an enum string and to an entity as an object.
    init(_ f: YNode) {
        let ref = str(f.child("$ref"))
        if ref.hasPrefix("#/enums/") {
            type = "string"; isEnum = true
            return
        }
        if ref.hasPrefix("#/entities/") {
            type = "object"
            return
        }
        let t = f.child("type")
        type = str(t)
        for item in items(t) where item.value != "null" { type = item.value }
        format = str(f.child("format"))
        isEnum = f.child("enum") != nil
        if let n = f.child("maxLength") { maxLength = Int(n.value) ?? 0 }
        if let n = f.child("precision") { precision = Int(n.value) ?? 0 }
        if let it = f.child("items") {
            let s = FieldShape(it)
            itemsType = s.type; itemsFormat = s.format
        }
    }

    /// Whether a row renders the field. A row without a format matches a
    /// field of any format that no row of the rendering names, so a decimal
    /// never falls through to a row for plain text.
    func matches(_ row: YNode, _ claimed: Set<String>) -> Bool {
        if str(row.child("type")) != type { return false }
        let f = str(row.child("format"))
        if !f.isEmpty && f != format || f.isEmpty && claimed.contains(format) { return false }
        if str(row.child("enum")) == "true" && !isEnum { return false }
        if let n = row.child("maxLengthAtMost"), maxLength == nil || maxLength! > (Int(n.value) ?? 0) { return false }
        if let n = row.child("precisionAtMost"), precision == nil || precision! > (Int(n.value) ?? 0) { return false }
        let it = str(row.child("itemsType"))
        if !it.isEmpty && it != itemsType { return false }
        let iform = str(row.child("itemsFormat"))
        if !iform.isEmpty && iform != itemsFormat { return false }
        return true
    }

    var description: String {
        var parts = [type]
        if !format.isEmpty { parts.append("format " + format) }
        if isEnum { parts.append("enum") }
        if let m = maxLength { parts.append("maxLength \(m)") }
        if let p = precision { parts.append("precision \(p)") }
        return parts.joined(separator: ", ")
    }
}

extension Checker {
    /// The stacks of an implementation file: its language, named by its file
    /// name, and the dialect of every target that has one (a target named
    /// sql has postgresql when it names none), each with the node and the
    /// path a problem with it is reported at.
    func implementationStacks() -> (stacks: [String], at: [String: YNode?], ptr: [String: String]) {
        var stacks: [String] = []
        var at: [String: YNode?] = [:]
        var ptr: [String: String] = [:]
        var name = String(file.split(separator: "/").last ?? "")
        if name.hasSuffix(implementationSuffix) { name.removeLast(implementationSuffix.count) }
        if let dot = name.lastIndex(of: ".") {
            let lang = String(name[name.index(after: dot)...])
            stacks.append(lang)
            at[lang] = root.key("stack"); ptr[lang] = "/stack"
        }
        for t in pairs(root.child("targets")) {
            let d = t.value.child("dialect")
            var dialect = str(d)
            var node = d
            if dialect.isEmpty && t.key.value == "sql" { dialect = "postgresql"; node = t.key }
            if dialect.isEmpty || at[dialect] != nil { continue }
            stacks.append(dialect)
            at[dialect] = node
            ptr[dialect] = d != nil ? pointer("targets", t.key.value, "dialect") : pointer("targets", t.key.value)
        }
        return (stacks, at, ptr)
    }

    /// Reads the idioms folder beside the implementation file: every
    /// *.specarch-idiom.yaml, checked against the idiom schema. Any other
    /// YAML file there is a layout problem.
    func projectIdioms() -> [String: Idiom] {
        let dir = joinPath(dirPath(file), "idioms")
        var isDir: ObjCBool = false
        guard FileManager.default.fileExists(atPath: dir, isDirectory: &isDir), isDir.boolValue,
              let names = try? FileManager.default.contentsOfDirectory(atPath: dir).sorted(by: byteLess) else { return [:] }
        var out: [String: Idiom] = [:]
        for name in names {
            let p = joinPath(dir, name)
            var sub: ObjCBool = false
            FileManager.default.fileExists(atPath: p, isDirectory: &sub)
            if sub.boolValue || name.hasPrefix(".") || !name.hasSuffix(".yaml") && !name.hasSuffix(".yml") { continue }
            if !name.hasSuffix(idiomSuffix) {
                addFile(p, 1, "/", .layout, "a file in idioms/ is an idiom named <name>\(idiomSuffix); rename it, or move it out of the folder")
                continue
            }
            let data: Data
            do { data = try Data(contentsOf: URL(fileURLWithPath: p)) } catch {
                addFile(p, 1, "/", .layout, "cannot read it (\(plainIOError(error)))")
                continue
            }
            let ic = Checker(file: p)
            let doc = parseYAML(String(decoding: data, as: UTF8.self))
            for pr in doc.problems { ic.addLine(pr.line, pr.path, Rule(rawValue: pr.rule)!, pr.message) }
            if let r = doc.root {
                ic.root = r
                ic.checkSchema(.idiom, doc.value)
            }
            diags += withoutEchoes(ic.diags)
            guard let r = doc.root, !ic.diags.contains(where: { $0.severity == .error }) else { continue }
            let i = Idiom(path: p, root: r)
            let want = String(name.dropLast(idiomSuffix.count))
            if i.name != want {
                addFile(p, r.key("name")?.line ?? 1, "/name", .layout, "the file is named \(want) but the idiom is \(i.name); name the file after the idiom")
                continue
            }
            out[i.name] = i
        }
        return out
    }

    /// Checks the idioms of an implementation file against the specification
    /// it implements: what the file excludes and overrides, the overrides
    /// themselves, and the contract of every idiom that applies.
    func checkIdioms(_ s: Spec) {
        let (stacks, at, ptr) = implementationStacks()
        var isStack: Set<String> = ["any"]
        for st in stacks { isStack.insert(st) }
        let project = projectIdioms()
        var excluded = Set<String>()
        var override: [String: Idiom] = [:]

        for u in pairs(root.child("idioms")) {
            let name = u.key.value
            let base = pointer("idioms", name)
            if shippedIdioms[name] == nil && project[name] == nil {
                add(u.key, base, .idiomUnknown, "\(name) is not an idiom SpecArch ships nor one in this file's idioms folder\(suggestIdiom(name, [shippedIdioms, project]))")
                continue
            }
            if str(u.value.child("exclude")) == "true" {
                excluded.insert(name)
                if str(u.value.child("why")).trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                    add(u.key, base, .idiomOverrideReason, "\(name) is excluded without why; say what the project does instead")
                }
                continue
            }
            let file = str(u.value.child("override"))
            if file.isEmpty { continue }
            var stem = String(file.split(separator: "/").last ?? "")
            if stem.hasSuffix(idiomSuffix) { stem.removeLast(idiomSuffix.count) }
            guard let o = project[stem], o.root.child("overrides") != nil else {
                add(u.value.child("override"), base + "/override", .idiomUnknown, "\(file) is not an override in this file's idioms folder; add the file with overrides naming \(name), or correct the path")
                continue
            }
            let idiom = str(o.root.child("overrides")?.child("idiom"))
            if idiom != name {
                add(u.value.child("override"), base + "/override", .idiomUnknown, "\(file) overrides \(idiom), not \(name); name it under idioms.\(idiom), or correct its overrides")
                continue
            }
            override[name] = o
        }

        for name in project.keys.sorted(by: byteLess) {
            let o = project[name]!
            guard let ov = o.root.child("overrides") else { continue }
            let oc = Checker(file: o.path)
            oc.root = o.root
            oc.checkOverride(o, ov, isStack)
            diags += oc.diags
        }

        guard let specRoot = s.root else { return }
        var apply: [String: Idiom] = [:]
        for (name, i) in shippedIdioms where !excluded.contains(name) { apply[name] = i }
        for (name, i) in project where i.root.child("overrides") == nil && !excluded.contains(name) { apply[name] = i }
        for name in apply.keys.sorted(by: byteLess) {
            let i = apply[name]!
            guard idiomApplies(i, specRoot, isStack) else { continue }
            let o = override[name]
            for st in stacks where items(i.root.child("stacks")).contains(where: { $0.value == st }) {
                for part in pairs(i.root.child("parts")) {
                    guard let r = resolvedRendering(i, o, part.key.value, st), r.child("rows") != nil else { continue }
                    checkRows(i, part.key.value, st, r, specRoot, at[st] ?? nil, ptr[st] ?? "/")
                }
            }
        }
    }

    /// Checks one override file against the shipped idiom it overrides. The
    /// receiver's file is the override's.
    func checkOverride(_ o: Idiom, _ ov: YNode, _ isStack: Set<String>) {
        let idiomNode = ov.child("idiom")
        guard let shipped = shippedIdioms[str(idiomNode)] else {
            add(idiomNode, "/overrides/idiom", .idiomUnknown, "\(str(idiomNode)) is not an idiom SpecArch ships, so there is nothing to override; drop overrides to make this the project's own idiom, or name a shipped one\(suggestIdiom(str(idiomNode), [shippedIdioms]))")
            return
        }
        if str(o.root.child("why")).trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            add(o.root.key("overrides"), "/overrides", .idiomOverrideReason, "the override of \(shipped.name) has no why; say why the project renders it differently")
        }
        let shippedParts = shipped.root.child("parts")
        var listed = Set<String>()
        for (i, p) in items(ov.child("parts")).enumerated() {
            listed.insert(p.value)
            if shippedParts?.child(p.value) == nil {
                var valid: [String: YNode] = [:]
                for sp in pairs(shippedParts) { valid[sp.key.value] = sp.value }
                add(p, pointer("overrides", "parts", "\(i)"), .idiomPartUnknown, "\(shipped.name) has no part \(p.value)\(suggest(p.value, valid))")
            }
        }
        let whole = str(ov.child("whole")) == "true"
        for p in pairs(o.root.child("parts")) {
            if !whole && !listed.contains(p.key.value) {
                add(p.key, pointer("parts", p.key.value), .idiomPartUnknown, "the part \(p.key.value) is not listed under overrides.parts; list it, or remove it so the shipped one applies")
            }
            for st in pairs(p.value.child("stack")) where !isStack.contains(st.key.value) {
                add(st.key, pointer("parts", p.key.value, "stack", st.key.value), .idiomStack, "\(st.key.value) is not a stack of the implementation file; it renders \(isStack.sorted(by: byteLess).joined(separator: ", "))")
            }
        }
        let shippedContract = shipped.root.child("contract")
        for st in pairs(o.root.child("contract")) {
            guard let was = shippedContract?.child(st.key.value) else { continue }
            if str(st.value.child("statement")) != str(was.child("statement")) || str(st.value.child("check")) != str(was.child("check")) {
                add(st.key, pointer("contract", st.key.value), .idiomContract, "\(st.key.value) is a contract statement of \(shipped.name), which an override may not change; keep it as shipped, or exclude the idiom and write the project's own")
            }
        }
        let verNode = ov.child("version")
        if let from = Version(str(verNode)), let now = Version(shipped.version), compareVersions(from, now) < 0,
           str(ov.child("staysBehind")) != "true" {
            warn(verNode, "/overrides/version", .idiomVersionBehind, "copied from \(shipped.name) \(str(verNode)), and SpecArch now ships \(shipped.version); compare them with specarch idioms diff \(shipped.name), then move to it, or set staysBehind: true and say why")
        }
    }

    /// Reports every entity field that no row of a rendering matches.
    func checkRows(_ i: Idiom, _ part: String, _ stack: String, _ r: YNode, _ specRoot: YNode, _ at: YNode?, _ ptr: String) {
        let rows = items(r.child("rows"))
        var claimed = Set<String>() // the formats a row names: only such a row renders them
        for row in rows {
            let f = str(row.child("format"))
            if !f.isEmpty { claimed.insert(f) }
        }
        for e in pairs(specRoot.child("entities")) {
            for f in pairs(e.value.child("properties")) {
                let field = FieldShape(f.value)
                if !rows.contains(where: { field.matches($0, claimed) }) {
                    add(at, ptr, .idiomContract, "\(i.name) \(i.version) has no \(stack) row in its \(part) part for #/entities/\(e.key.value)/properties/\(f.key.value) (\(field.description)); add one in an override of the part, or exclude the idiom with the reason")
                }
            }
        }
    }
}

/// Whether an idiom applies to a specification and an implementation file:
/// it renders one of the file's stacks, or any, and the specification uses
/// one of the keywords it reads, as a section or as a key anywhere inside
/// one.
func idiomApplies(_ i: Idiom, _ root: YNode, _ isStack: Set<String>) -> Bool {
    guard items(i.root.child("stacks")).contains(where: { isStack.contains($0.value) }) else { return false }
    for r in items(i.root.child("reads")) where usesKeyword(root, r.value) {
        return true
    }
    return false
}

/// Whether the design uses a keyword: a section of that name that is not
/// empty, or the key anywhere inside a section.
func usesKeyword(_ root: YNode, _ keyword: String) -> Bool {
    if let n = root.child(keyword), !n.pairs.isEmpty || !n.items.isEmpty { return true }
    var found = false
    walk(root, []) { n, path in
        if !path.isEmpty && n.key(keyword) != nil { found = true }
    }
    return found
}

/// The rendering of one part for one stack, by the lookup order: the
/// override's part for the stack, the shipped part for the stack, the
/// shipped part under any.
func resolvedRendering(_ i: Idiom, _ o: Idiom?, _ part: String, _ stack: String) -> YNode? {
    if let o, let ov = o.root.child("overrides") {
        let replaces = str(ov.child("whole")) == "true" || items(ov.child("parts")).contains { $0.value == part }
        if replaces, let r = o.root.child("parts")?.child(part)?.child("stack")?.child(stack) { return r }
    }
    let stacks = i.root.child("parts")?.child(part)?.child("stack")
    return stacks?.child(stack) ?? stacks?.child("any")
}

func suggestIdiom(_ name: String, _ sets: [[String: Idiom]]) -> String {
    var all: [String: YNode] = [:]
    for set in sets { for (k, v) in set { all[k] = v.root } }
    return suggest(name, all)
}
