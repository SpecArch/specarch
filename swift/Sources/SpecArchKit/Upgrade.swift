import Foundation

// specarch upgrade (docs/meta-model-1.0.0.md): what moving one file to the
// newest version of its format changes, its version line, its editor hint
// and the keys 1.0.0 renamed, each an edit of the text at the line and
// column the parser gives. internal/upgrade and cmd/specarch/upgrade.go in
// the Go build.

/// What a file of a specification holds, which says which version line it
/// has.
enum UpgradeKind {
    case root, fragment, implementation, idiom, record, estate

    var format: MetaModelFormat? {
        switch self {
        case .root: return .design
        case .implementation: return .implementation
        case .idiom: return .idiom
        case .record: return .record
        case .estate: return .estate
        case .fragment: return nil
        }
    }
}

/// One edit of a file: at the line and column (from 1, in characters), the
/// text old becomes new.
struct UpgradeChange {
    let file: String
    let line: Int
    let column: Int
    let old: String
    let new: String
    let offset: Int // in bytes, from the file's start

    var description: String { "\(file):\(line):\(column): \(old) -> \(new)" }
}

private let hintPrefix = "# yaml-language-server: $schema="
private let schemaFilePattern = try! NSRegularExpression(pattern: #"(specarch-(?:design|fragment-[a-z]+|implementation|record|idiom|estate))-(0\.[0-9]+)(\.schema\.json)$"#)
private let releaseURLPattern = try! NSRegularExpression(pattern: #"^(https://raw\.githubusercontent\.com/SpecArch/specarch/)([^/]+)(/schema/)"#)

/// Lists what moving the file at path to 1.0.0 changes in it, in the order
/// they appear. program is the program's version, whose release tag a hint
/// that names a release URL moves to. specCurrent says, for a root file or
/// a fragment, whether its specification is read as 1.0 after the move
/// (staysEarlier of the root file's line is false); a file of another kind
/// goes by its own version line. The hint and the renamed keys move only in
/// a file read as 1.0 after the move, since an earlier schema refuses the
/// new keys. Of a file the parser reads only in part, the entries it read
/// are edited; validate's problem at the parser's position is the step
/// left to the author.
func upgradeChanges(_ path: String, _ data: [UInt8], _ kind: UpgradeKind, _ program: String, _ specCurrent: Bool) -> [UpgradeChange] {
    var out: [UpgradeChange] = []
    let doc = parseYAML(yamlText(Data(data)))
    let root = doc.root.flatMap { $0.kind == .mapping ? $0 : nil }
    var current = specCurrent
    if let f = kind.format, kind != .root {
        current = root == nil || !staysEarlier(f, root?.child(f.key))
    }
    if current, let c = upgradeHint(path, data, program) { out.append(c) }
    guard let root else { return out }
    if let f = kind.format, let n = root.child(f.key), upgradeMoves(f, n),
       let c = scalarChange(path, data, n, currentMetaModel) {
        out.append(c)
    }
    if current && (kind == .root || kind == .fragment) {
        for r in pairs(root.child("roles")) {
            if let c = keyChange(path, data, r.value, rowPoliciesOldKey, rowPoliciesKey) { out.append(c) }
        }
        if let c = keyChange(path, data, root.child("handover"), sourceDeliveryOldKey, sourceDeliveryKey) { out.append(c) }
    }
    return out.enumerated().sorted { ($0.element.offset, $0.offset) < ($1.element.offset, $1.offset) }.map(\.element)
}

/// Whether the version line n moves to 1.0.0: a string on an earlier
/// version. A number, such as an unquoted 0.3, is not rewritten, since its
/// type is the author's error, which validate reports.
func upgradeMoves(_ f: MetaModelFormat, _ n: YNode?) -> Bool {
    guard let n else { return false }
    return n.kind == .scalar && n.tag == "!!str" && f.isEarlier(n.value)
}

/// Whether a file whose version line of format f is line is still read as
/// a version before 1.0.0 after the move: the line is read as an earlier
/// version, as validate reads a number such as an unquoted 0.3, and is not
/// one upgrade moves. A line upgrade moves, one already on 1.0, and one the
/// program does not read, whose file it reads as 1.0.0, are not.
func staysEarlier(_ f: MetaModelFormat, _ line: YNode?) -> Bool {
    f.isEarlier(str(line)) && !upgradeMoves(f, line)
}

/// A version line the move leaves off 1.0.0, a step left to the author:
/// at the line and column (from 1), the pointer of the line, or / for a
/// file that has none, and what to do.
struct UpgradeStep {
    let line: Int
    let column: Int
    let pointer: String
    let message: String
}

/// The step the move leaves at the version line of a file of a kind that
/// has one: a missing line, a value that is not a string, such as an
/// unquoted 0.3 the parser reads as a number, or a string that is neither
/// on 1.0 nor an earlier version upgrade moves, such as "0.4" or "1.0".
/// After the move, a file with no step reads 1.0.x on its version line. A
/// file the parser cannot read is validate's step.
func upgradeLeft(_ data: [UInt8], _ kind: UpgradeKind) -> UpgradeStep? {
    guard let f = kind.format else { return nil }
    let doc = parseYAML(yamlText(Data(data)))
    guard let root = doc.root, root.kind == .mapping else { return nil }
    let again = ", and run specarch upgrade again"
    guard let n = root.child(f.key) else {
        return UpgradeStep(line: root.line, column: root.column, pointer: "/", message: "the file has no version line \(f.key), which specarch upgrade does not add; add \(f.key) with the version the file is written in" + again)
    }
    let at = pointer(f.key)
    if n.kind != .scalar || n.tag != "!!str" {
        let what = n.kind == .scalar && (n.tag == "!!float" || n.tag == "!!int") ? "a number, not a string" : "not a string"
        return UpgradeStep(line: n.line, column: n.column, pointer: at, message: "the version line \(f.key) is \(what), which specarch upgrade does not rewrite; quote it, such as \"\(f.earlier[f.earlier.count - 1])\"" + again)
    }
    switch f.classify(n.value) {
    case .known, .earlier: return nil
    default: break
    }
    let earlier = Array(f.earlier.reversed())
    let moved = earlier.count > 1 ? earlier.dropLast().joined(separator: ", ") + " and " + earlier[earlier.count - 1] : earlier[0]
    return UpgradeStep(line: n.line, column: n.column, pointer: at, message: "the version line \(f.key) is \"\(n.value)\", which specarch upgrade does not move: it moves \(moved) to \(currentMetaModel) and leaves 1.0.x as it is; give the version the file is written in" + again)
}

/// The change of the first line's editor hint to the 1.0 schema of its
/// kind: a release URL of SpecArch's repository moves to the program's tag,
/// and a relative path keeps its folder.
private func upgradeHint(_ path: String, _ data: [UInt8], _ program: String) -> UpgradeChange? {
    let prefix = Array(hintPrefix.utf8)
    guard data.starts(with: prefix) else { return nil }
    let end = data.firstIndex(of: UInt8(ascii: "\n")) ?? data.count
    var old = String(decoding: data[prefix.count..<end], as: UTF8.self)
    while old.hasSuffix("\r") { old.removeLast() }
    let ns = old as NSString
    guard let m = schemaFilePattern.firstMatch(in: old, range: NSRange(location: 0, length: ns.length)) else { return nil }
    var next = ns.substring(to: m.range(at: 1).location + m.range(at: 1).length) + "-1.0" + ns.substring(from: m.range(at: 3).location)
    let nn = next as NSString
    if let u = releaseURLPattern.firstMatch(in: next, range: NSRange(location: 0, length: nn.length)) {
        next = nn.substring(with: u.range(at: 1)) + "v" + program + nn.substring(with: u.range(at: 3)) + nn.substring(from: u.range.location + u.range.length)
    }
    if next == old { return nil }
    return UpgradeChange(file: path, line: 1, column: hintPrefix.count + 1, old: old, new: next, offset: prefix.count)
}

/// Rewrites a scalar value to value, keeping its quoting.
private func scalarChange(_ path: String, _ data: [UInt8], _ n: YNode, _ value: String) -> UpgradeChange? {
    guard let at = offsetOf(data, n.line, n.column) else { return nil }
    let old = scalarToken(data[at...])
    guard let first = old.first else { return nil }
    var next = value
    if first == "\"" || first == "'" { next = String(first) + value + String(first) }
    return UpgradeChange(file: path, line: n.line, column: n.column, old: old, new: next, offset: at)
}

/// Renames the key old of the mapping obj to key, keeping its quoting,
/// unless obj gives both: that is a step left to the author, reported by
/// validate.
private func keyChange(_ path: String, _ data: [UInt8], _ obj: YNode?, _ old: String, _ key: String) -> UpgradeChange? {
    guard let obj, let k = obj.key(old), obj.child(key) == nil, let at = offsetOf(data, k.line, k.column) else { return nil }
    let text = scalarToken(data[at...])
    guard text == old || text == "\"" + old + "\"" || text == "'" + old + "'" else {
        return nil // not the key as written: one the reader added
    }
    var next = key
    if let first = text.first, first == "\"" || first == "'" { next = String(first) + key + String(first) }
    return UpgradeChange(file: path, line: k.line, column: k.column, old: text, new: next, offset: at)
}

/// The byte offset of a line and a column counted in characters, both from
/// 1.
private func offsetOf(_ data: [UInt8], _ line: Int, _ column: Int) -> Int? {
    var at = 0
    var l = 1
    while l < line {
        guard let i = data[at...].firstIndex(of: UInt8(ascii: "\n")) else { return nil }
        at = i + 1
        l += 1
    }
    var c = 1
    while c < column {
        guard at < data.count, data[at] != UInt8(ascii: "\n") else { return nil }
        at += utf8Width(data[at])
        c += 1
    }
    return at < data.count ? at : nil
}

private func utf8Width(_ b: UInt8) -> Int {
    switch b {
    case 0xF0...: return 4
    case 0xE0...: return 3
    case 0xC0...: return 2
    default: return 1
    }
}

/// The scalar that starts the text: a double-quoted one with its escapes, a
/// single-quoted one with its doubled quotes, or a plain one up to the
/// space, comment, colon or flow indicator that ends it.
private func scalarToken(_ text: ArraySlice<UInt8>) -> String {
    let t = Array(text)
    guard let first = t.first else { return "" }
    func string(_ n: Int) -> String { String(decoding: t[0..<n], as: UTF8.self) }
    if first == UInt8(ascii: "\"") {
        var i = 1
        while i < t.count {
            switch t[i] {
            case UInt8(ascii: "\\"): i += 1
            case UInt8(ascii: "\""): return string(i + 1)
            case UInt8(ascii: "\n"): return ""
            default: break
            }
            i += 1
        }
        return ""
    }
    if first == UInt8(ascii: "'") {
        var i = 1
        while i < t.count {
            if t[i] == UInt8(ascii: "'") {
                if i + 1 < t.count && t[i + 1] == UInt8(ascii: "'") {
                    i += 2
                    continue
                }
                return string(i + 1)
            }
            if t[i] == UInt8(ascii: "\n") { return "" }
            i += 1
        }
        return ""
    }
    var end = t.count
    for (i, b) in t.enumerated() {
        let next: UInt8? = i + 1 < t.count ? t[i + 1] : nil
        if b == UInt8(ascii: "\n") || b == UInt8(ascii: "\r") || b == UInt8(ascii: ",") || b == UInt8(ascii: "}") || b == UInt8(ascii: "]")
            || b == UInt8(ascii: " ") && next == UInt8(ascii: "#")
            || b == UInt8(ascii: ":") && (next == nil || next == UInt8(ascii: " ") || next == UInt8(ascii: "\n")) {
            end = i
            break
        }
    }
    var s = string(end)
    while s.hasSuffix(" ") || s.hasSuffix("\t") { s.removeLast() }
    return s
}

/// Makes the changes of one file to its text.
func applyUpgrade(_ data: [UInt8], _ changes: [UpgradeChange]) -> [UInt8] {
    var out = data
    for c in changes.sorted(by: { $0.offset > $1.offset }) {
        out.replaceSubrange(c.offset..<(c.offset + c.old.utf8.count), with: Array(c.new.utf8))
    }
    return out
}

/// The upgrade command: moves each specification given, and every file it
/// is read from, to 1.0.0, edited in place; with --dry-run it writes
/// nothing. A step it cannot take is printed as a problem line, and exits 1.
func runUpgrade(_ args: [String], _ stdout: TextSink, _ stderr: TextSink) -> Int32 {
    func count(_ n: Int, _ word: String) -> String { n == 1 ? "1 " + word : "\(n) \(word)s" }
    var dryRun = false
    var to = currentMetaModel
    var paths: [String] = []
    var i = 0
    while i < args.count {
        let a = args[i]
        if a == "--" {
            paths += args[(i + 1)...]
            break
        } else if a == "--dry-run" {
            dryRun = true
        } else if a == "--to" || a.hasPrefix("--to=") {
            if a.hasPrefix("--to=") {
                to = String(a.dropFirst("--to=".count))
            } else if i + 1 < args.count {
                i += 1
                to = args[i]
            } else {
                stderr.write("specarch upgrade: --to needs a version\n\n\(usage)")
                return 2
            }
        } else if a.count > 1 && a.hasPrefix("-") {
            stderr.write("specarch upgrade has no option \(a)\n\n\(usage)")
            return 2
        } else {
            paths.append(a)
        }
        i += 1
    }
    if to != currentMetaModel {
        stderr.write("specarch upgrade: --to \(to) is not a version this program writes; it moves every file to \(currentMetaModel), the newest, so leave --to out or give \(currentMetaModel)\n\n\(usage)")
        return 2
    }
    if paths.isEmpty {
        stderr.write("specarch upgrade needs at least one folder\n\n\(usage)")
        return 2
    }
    let (inputs, ioError) = collect(paths, stderr)
    if ioError { return 2 }
    var specs: [Spec] = []
    var estates: [String] = []
    var impls: [String] = []
    for input in inputs {
        switch input {
        case .estate(let e): estates.append(e.path)
        case .root(let dir): specs.append(Spec(dir: dir))
        case .implementation(let path): impls.append(path)
        case .other(let name):
            stderr.write("specarch upgrade: \(name) is neither a specification nor an implementation file nor an estate; name the folder that holds \(rootFile)\n")
            return 2
        }
    }
    if specs.isEmpty && estates.isEmpty && impls.isEmpty {
        stderr.write("specarch upgrade: no specification given; name a folder that holds \(rootFile)\n")
        return 2
    }

    var files: [UpgradeFile] = []
    var seen = Set<String>()
    func add(_ f: UpgradeFile) {
        if seen.insert(cleanPath(f.path)).inserted { files.append(f) }
    }
    for e in estates { add(UpgradeFile(path: e, kind: .estate, current: true, dir: dirPath(e))) }
    for i in impls {
        for f in implementationFiles(i, dirPath(i)) { add(f) }
    }
    for s in specs {
        for f in specFiles(s) { add(f) }
    }
    files.sort { byteLess($0.path, $1.path) }

    var changes: [UpgradeChange] = []
    var left: [Diagnostic] = []
    var byFile: [String: [UpgradeChange]] = [:]
    var data: [String: [UInt8]] = [:]
    for f in files {
        let d: [UInt8]
        do { d = Array(try readFile(f.path)) } catch {
            stderr.write("specarch upgrade: cannot read \(f.path): \(plainIOError(error))\n")
            return 2
        }
        let cs = upgradeChanges(f.path, d, f.kind, programVersion, f.current)
        if let st = upgradeLeft(d, f.kind) {
            var l = [Diagnostic(file: f.path, line: st.line, column: st.column, severity: .error, path: st.pointer, rule: .metaModelVersion, message: st.message)]
            Placer(dir: f.dir).place(&l)
            left += l
        }
        if !cs.isEmpty {
            data[f.path] = d
            byFile[f.path] = cs
            changes += cs
        }
    }

    var steps = upgradeSteps(specs) + left
    sortDiagnostics(&steps)
    for c in changes { stdout.write(c.description + "\n") }
    for d in steps {
        stdout.write(d.description + "\n")
        if let l = d.detailLine { stderr.write(l + "\n") }
    }
    if !dryRun {
        for f in files {
            guard let cs = byFile[f.path], let d = data[f.path] else { continue }
            do {
                try Data(applyUpgrade(d, cs)).write(to: URL(fileURLWithPath: f.path))
            } catch {
                stderr.write("specarch upgrade: cannot write \(f.path): \(plainIOError(error))\n")
                return 2
            }
        }
    }
    let verb = dryRun ? "to make" : "made"
    stderr.write("specarch upgrade: \(count(changes.count, "change")) \(verb) in \(count(byFile.count, "file")), \(count(steps.count, "step")) left to the author\n")
    if steps.contains(where: { $0.rule == .testCaseMissing || $0.rule == .testGoldenMissing }) {
        stderr.write("specarch upgrade: the tests the steps name are missing; specarch derive writes them\n")
    }
    return steps.isEmpty ? 0 : 1
}

/// One file of a specification upgrade reads, with its kind. current is,
/// for a root file or a fragment, whether its specification is read as 1.0
/// after the move, which moves its hint and renamed keys; dir is the folder
/// the ids of its problems name it from.
private struct UpgradeFile {
    let path: String
    let kind: UpgradeKind
    let current: Bool
    let dir: String
}

/// Every file a specification is read from, with its kind: the root file,
/// the stage files and tests, the implementation files and the idioms in
/// the folder beside each, and the records. The hints and the renamed keys
/// of the root file and the fragments move only when the root file's
/// version line leaves the specification read as 1.0 after the move.
private func specFiles(_ s: Spec) -> [UpgradeFile] {
    let current = !staysEarlier(.design, s.root?.child(MetaModelFormat.design.key))
    var out: [UpgradeFile] = []
    var paths = Set(s.files.values)
    for h in s.held { paths.insert(h.file) }
    for p in paths {
        out.append(UpgradeFile(path: p, kind: cleanPath(p) == cleanPath(s.rootPath) ? .root : .fragment, current: current, dir: s.dir))
    }
    if !paths.contains(s.rootPath) { out.append(UpgradeFile(path: s.rootPath, kind: .root, current: current, dir: s.dir)) }
    for impl in s.implementations { out += implementationFiles(impl.path, s.dir) }
    for r in s.records { out.append(UpgradeFile(path: r.path, kind: .record, current: true, dir: s.dir)) }
    return out
}

/// An implementation file and the idioms in the idioms/ folder beside it,
/// whose problems' ids name them from dir.
private func implementationFiles(_ path: String, _ dir: String) -> [UpgradeFile] {
    var out = [UpgradeFile(path: path, kind: .implementation, current: true, dir: dir)]
    let folder = joinPath(dirPath(path), "idioms")
    let names = ((try? FileManager.default.contentsOfDirectory(atPath: folder)) ?? []).sorted(by: byteLess)
    for name in names where name.hasSuffix(idiomSuffix) {
        out.append(UpgradeFile(path: joinPath(folder, name), kind: .idiom, current: true, dir: dir))
    }
    return out
}

/// The steps the move leaves to the author: every error a specification
/// would have on 1.0.0 that it does not have on its own version, a key
/// given under both spellings, and a file or an entry the parser cannot
/// read. The other problems are validate's.
private func upgradeSteps(_ specs: [Spec]) -> [Diagnostic] {
    struct At: Hashable { let rule: String; let file: String; let path: String }
    let given = makeGiven(specs)
    var out: [Diagnostic] = []
    for (i, s) in specs.enumerated() {
        let before = checkSpec(s, given)
        var had = Set<At>()
        for d in before {
            if d.severity == .error { had.insert(At(rule: d.rule.rawValue, file: d.file, path: d.path)) }
            if d.rule == .yamlSyntax { out.append(d) }
        }
        let moved = Spec(dir: s.dir)
        if let root = moved.root, let idx = root.pairs.firstIndex(where: { $0.key.value == "specarch" }),
           upgradeMoves(.design, root.pairs[idx].value) {
            let old = root.pairs[idx].value
            let line = YNode(kind: .scalar, value: currentMetaModel, tag: "!!str", style: .quoted, line: old.line, column: old.column)
            moved.files[ObjectIdentifier(line)] = moved.files[ObjectIdentifier(old)]
            root.pairs[idx].value = line
            if case .object(var members) = moved.value, let j = members.firstIndex(where: { $0.0 == "specarch" }) {
                members[j].1 = .string(currentMetaModel)
                moved.value = .object(members)
            }
        }
        var others = specs
        others[i] = moved
        for d in checkSpec(moved, makeGiven(others)) where d.severity == .error {
            if d.rule == .metaModelDeprecated || !had.contains(At(rule: d.rule.rawValue, file: d.file, path: d.path)) {
                out.append(d)
            }
        }
    }
    sortDiagnostics(&out)
    return out
}
