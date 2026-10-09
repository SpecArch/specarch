import Foundation

/// Gives diagnostics their column and their id (ADR-065). root and files
/// are the merged specification and the file of each of its nodes; a
/// pointer that does not lead to a node of the diagnostic's file there is
/// followed in that file parsed on its own. dir is the folder ids name
/// files from.
final class Placer {
    let root: YNode?
    let files: [ObjectIdentifier: String]
    let rootFile: String
    let dir: String
    private var texts: [String: [String]] = [:]
    private var trees: [String: YNode?] = [:]

    init(root: YNode? = nil, files: [ObjectIdentifier: String] = [:], rootFile: String = "", dir: String) {
        self.root = root
        self.files = files
        self.rootFile = rootFile
        self.dir = dir
    }

    /// The placer of a specification's diagnostics: ids name files from
    /// the specification's folder.
    convenience init(spec s: Spec) {
        self.init(root: s.root, files: s.files, rootFile: s.rootPath, dir: s.dir)
    }

    /// Sets the column of each diagnostic that has none and the id of
    /// every one, telling apart the ids one rule reports at one pointer of
    /// one file.
    func place(_ ds: inout [Diagnostic]) {
        for i in ds.indices {
            if ds[i].column < 1 {
                let tokens = pointerTokens(ds[i].path)
                ds[i].column = column(ds[i].file, ds[i].line, treeOf(ds[i].file, tokens), tokens)
            }
            ds[i].id = "\(ds[i].rule.rawValue)@\(relSlash(dir, ds[i].file))#\(ds[i].path)"
        }
        numberIDs(&ds)
    }

    /// The tree a pointer into file is into: the merged specification when
    /// the pointer leads there to a node of that file, and otherwise the
    /// file parsed on its own.
    func treeOf(_ file: String, _ tokens: [String]) -> YNode? {
        if let root {
            let (n, ok) = resolve(root, tokens)
            if ok && files[ObjectIdentifier(n)] == file { return root }
            if tokens.isEmpty && file == rootFile { return root }
        }
        if let t = trees[file] { return t }
        var t: YNode?
        if let data = try? readFile(file) {
            t = parseYAML(String(decoding: data, as: UTF8.self)).root
        }
        trees[file] = t
        return t
    }

    /// The column of the node the pointer names when the problem is on its
    /// line (its value, or else its key), and otherwise that of the first
    /// character on the line that is not a space.
    private func column(_ file: String, _ line: Int, _ root: YNode?, _ tokens: [String]) -> Int {
        if let root {
            let (start, value, whole) = locate(root, tokens)
            if whole {
                if value.line == line && value.column > 0 { return value.column }
                if start.line == line && start.column > 0 { return start.column }
            }
        }
        return indent(file, line) + 1
    }

    /// Counts the spaces and tabs a line of a file starts with.
    func indent(_ file: String, _ line: Int) -> Int {
        var n = 0
        for c in text(file, line).unicodeScalars {
            if c != " " && c != "\t" { return n }
            n += 1
        }
        return 0
    }

    /// The text of a line of a file, from 1; "" past its end.
    func text(_ file: String, _ line: Int) -> String {
        if texts[file] == nil {
            var lines: [String] = []
            if let data = try? readFile(file) {
                lines = String(decoding: data, as: UTF8.self).components(separatedBy: "\n")
            }
            texts[file] = lines
        }
        let lines = texts[file]!
        guard line >= 1 && line <= lines.count else { return "" }
        return lines[line - 1]
    }
}

/// Follows a pointer from root. Returns where the deepest entry reached
/// starts (its key, or its item in a list), its value, and whether the
/// whole pointer was reached. For the empty pointer both are the root.
func locate(_ root: YNode, _ tokens: [String]) -> (start: YNode, value: YNode, whole: Bool) {
    var start = root, value = root
    for t in tokens {
        guard let next = value.child(t) else { return (start, value, false) }
        if value.kind == .mapping {
            start = value.key(t) ?? next
        } else {
            start = next
        }
        value = next
    }
    return (start, value, true)
}

/// Splits a JSON pointer into its unescaped tokens; none for "/".
func pointerTokens(_ ptr: String) -> [String] {
    if ptr.isEmpty || ptr == "/" { return [] }
    return ptr.dropFirst().split(separator: "/", omittingEmptySubsequences: false).map { unescapeToken(String($0)) }
}

/// Names a file relative to a folder, with slashes, as Go's filepath.Rel
/// does from both paths made absolute.
func relSlash(_ from: String, _ to: String) -> String {
    let a = absoluteParts(from), b = absoluteParts(to)
    var i = 0
    while i < a.count && i < b.count && a[i] == b[i] { i += 1 }
    let parts = Array(repeating: "..", count: a.count - i) + b[i...]
    return parts.isEmpty ? "." : parts.joined(separator: "/")
}

private func absoluteParts(_ path: String) -> [String] {
    let full = path.hasPrefix("/") ? path : FileManager.default.currentDirectoryPath + "/" + path
    var out: [String] = []
    for part in full.split(separator: "/") {
        switch part {
        case ".": continue
        case "..": if !out.isEmpty { out.removeLast() }
        default: out.append(String(part))
        }
    }
    return out
}

/// Tells apart the diagnostics of one rule at one pointer of one file:
/// each gets .<tag> after the rule, eight hex digits of the FNV-1a hash of
/// its message, so that fixing one leaves the others' ids as they are.
/// Diagnostics with the same message as well are numbered -2, -3 and on.
private func numberIDs(_ ds: inout [Diagnostic]) {
    var groups: [String: [Int]] = [:]
    var order: [String] = []
    for (i, d) in ds.enumerated() {
        if groups[d.id] == nil { order.append(d.id) }
        groups[d.id, default: []].append(i)
    }
    for id in order {
        let idx = groups[id]!
        if idx.count < 2 { continue }
        var seen: [String: Int] = [:]
        for i in idx {
            var hash: UInt32 = 2166136261
            for byte in ds[i].message.utf8 {
                hash ^= UInt32(byte)
                hash = hash &* 16777619
            }
            var tag = String(format: "%08x", hash)
            seen[tag, default: 0] += 1
            if seen[tag]! > 1 { tag += "-\(seen[tag]!)" }
            let rule = ds[i].rule.rawValue
            if let r = ds[i].id.range(of: rule + "@") {
                ds[i].id.replaceSubrange(r, with: rule + "." + tag + "@")
            }
        }
    }
}
