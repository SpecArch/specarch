import Foundation

// A file that does not parse keeps what can be read (ADR-080). YAML reads a
// document as one unit and refuses it whole at the first syntax error, so
// every entry of the file would be lost and every reference to one of them
// would be reported again. Instead the file is read entry by entry, by its
// indentation: an entry is parsed with the lines of the keys above it and
// every other line made a comment, so each value keeps its line and column.
// An entry that parses with the entries kept before it is kept. One that does not is read entry by entry in
// its turn when it is a key whose value is on the lines below, and is
// otherwise kept by its name with no value (held), its line cut after the
// key, so whatever refers to it still finds it; a top-level key is left out
// instead, since nothing refers to a section by name. Each entry that does
// not parse is a syntax error at its own line and pointer, and nothing else
// is reported at it, inside it, or of it as missing (Doc.held).

private let tabNote = "this line is indented with a tab, and YAML indents with spaces only; indent it with spaces"
private let heldNote = "; the other entries of the file are read, and this one is kept by its name only until it parses"
private let droppedNote = "; the other entries of the file are read, and this one is left out until it parses"

private func trimLeftSpaces(_ s: String) -> String {
    String(s.drop { $0 == " " })
}

private final class Reader {
    var lines: [String]
    var held: [String] = []
    var problems: [Problem] = []

    init(_ lines: [String]) { self.lines = lines }

    /// The file with the lines of every given map as they are given and
    /// every other line a comment.
    func masked(_ parts: [[Int: String]]) -> String {
        var out: [String] = []
        for i in lines.indices {
            var l = "#"
            for p in parts {
                if let given = p[i] { l = given }
            }
            out.append(l)
        }
        return out.joined(separator: "\n")
    }

    /// The syntax error of the file made of the given lines, or nil when it parses.
    func parses(_ parts: [Int: String]...) -> Problem? {
        let (doc, syntax) = parseWhole(masked(parts))
        return syntax ? doc.problems[0] : nil
    }

    /// Reads the entries between lines lo and hi, under the keys of ctx,
    /// and returns the lines to read them by.
    func block(_ ctx: [Int: String], _ lo: Int, _ hi: Int, _ ptr: String, _ depth: Int) -> [Int: String] {
        var starts: [Int] = []
        var indent = -1, items = false
        for i in lo..<max(lo, hi) {
            let t = trimLeftSpaces(lines[i])
            if t.isEmpty || t.hasPrefix("#") { continue }
            let at = lines[i].utf8.count - t.utf8.count
            let item = t == "-" || t.hasPrefix("- ")
            if indent < 0 { (indent, items) = (at, item) }
            // A list written at the indentation of the key that holds it
            // belongs to that key.
            if at <= indent && (items || !item || at < indent) {
                starts.append(i)
            }
        }
        var out: [Int: String] = [:]
        for (n, s) in starts.enumerated() {
            let e = n + 1 < starts.count ? starts[n + 1] : hi
            var chunk: [Int: String] = [:]
            for i in s..<e { chunk[i] = lines[i] }
            guard var problem = parses(ctx, out, chunk) else {
                out.merge(chunk) { $1 }
                continue
            }
            if problem.line < s + 1 || problem.line > e {
                // Read with the lines after it made comments, an entry left
                // open fails where the file ends; the error is the entry's.
                problem = Problem(line: s + 1, path: problem.path, rule: problem.rule, message: problem.message)
            }
            let line = lines[s]
            let entry = entryOf(line)
            var at = ptr
            if let entry {
                at = ptr + "/" + escapeToken(entry.item ? String(n) : entry.key)
                let after = String(decoding: Array(line.utf8)[entry.cut.utf8.count...], as: UTF8.self)
                    .trimmingCharacters(in: .whitespacesAndNewlines)
                if !entry.item && e - s > 1 && (after.isEmpty || after.hasPrefix("#")) {
                    // A key whose value is on the lines below: read its entries.
                    let (problemCount, heldCount) = (problems.count, held.count)
                    var inner = ctx
                    inner[s] = entry.cut
                    var sub = block(inner, s + 1, e, at, depth + 1)
                    sub[s] = entry.cut
                    if problems.count > problemCount && parses(ctx, out, sub) == nil {
                        out.merge(sub) { $1 }
                        continue
                    }
                    problems.removeLast(problems.count - problemCount)
                    held.removeLast(held.count - heldCount)
                }
                if depth > 0 && parses(ctx, out, [s: entry.cut]) == nil {
                    out[s] = entry.cut
                    held.append(at)
                    problems.append(Problem(line: problem.line, path: at, rule: problem.rule, message: problem.message + heldNote))
                    continue
                }
            }
            if entry != nil { held.append(at) }
            problems.append(Problem(line: problem.line, path: at.isEmpty ? "/" : at, rule: problem.rule, message: problem.message + droppedNote))
        }
        return out
    }
}

/// Reads what can be read of a file that does not parse; nil when no entry
/// can be told apart from the error.
func readable(_ text: String) -> Doc? {
    let r = Reader(text.components(separatedBy: "\n"))
    var ctx: [Int: String] = [:]
    var lo = 0
    // A directive or a document start before the first key is kept as it is.
    while lo < r.lines.count {
        let t = trimLeftSpaces(r.lines[lo])
        if !t.isEmpty && !t.hasPrefix("#") && !t.hasPrefix("%") && !t.hasPrefix("---") { break }
        if !t.isEmpty && !t.hasPrefix("#") { ctx[lo] = r.lines[lo] }
        lo += 1
    }
    // A line indented with a tab is read by no YAML parser; it is left out.
    for (i, l) in r.lines.enumerated() where l.hasPrefix("\t") && !l.trimmingCharacters(in: .whitespaces).isEmpty {
        r.problems.append(Problem(line: i + 1, path: "/", rule: "yaml_syntax", message: tabNote + droppedNote))
        r.lines[i] = "#"
    }
    let out = r.block(ctx, lo, r.lines.count, "", 0)
    if r.problems.isEmpty { return nil }
    var (doc, _) = parseWhole(r.masked([ctx, out]))
    guard doc.root != nil else { return nil }
    doc.problems = r.problems + doc.problems
    doc.held = r.held
    return doc
}

/// The start of an entry's first line: a list's item, whose line is cut
/// after its dash, or a key, plain or quoted without escapes, cut after its
/// colon. nil when the line starts no entry that can be told.
private func entryOf(_ line: String) -> (key: String, cut: String, item: Bool)? {
    let bytes = Array(line.utf8)
    var lead = 0
    while lead < bytes.count && bytes[lead] == UInt8(ascii: " ") { lead += 1 }
    let t = Array(bytes[lead...])
    let text = { (b: ArraySlice<UInt8>) in String(decoding: b, as: UTF8.self) }
    if t == [UInt8(ascii: "-")] || (t.count >= 2 && t[0] == UInt8(ascii: "-") && t[1] == UInt8(ascii: " ")) {
        return ("", text(bytes[..<(lead + 1)]), true)
    }
    guard let first = t.first else { return nil }
    var key: String, rest: ArraySlice<UInt8>
    if first == UInt8(ascii: "\"") || first == UInt8(ascii: "'") {
        guard let end = t[1...].firstIndex(of: first) else { return nil }
        if first == UInt8(ascii: "\"") && t[1..<end].contains(UInt8(ascii: "\\")) { return nil }
        if first == UInt8(ascii: "'") && end + 1 < t.count && t[end + 1] == first { return nil }
        key = text(t[1..<end])
        rest = t[(end + 1)...]
    } else if Array("[]{},&*!|>%@`#?:-\t".utf8).contains(first) {
        return nil
    } else {
        var colon = -1
        for i in t.indices {
            if t[i] == UInt8(ascii: ":") && (i + 1 == t.count || t[i + 1] == UInt8(ascii: " ")) {
                colon = i
                break
            }
            if t[i] == UInt8(ascii: "#") && i > 0 && t[i - 1] == UInt8(ascii: " ") { break }
        }
        if colon <= 0 { return nil }
        var k = colon
        while k > 0 && t[k - 1] == UInt8(ascii: " ") { k -= 1 }
        key = text(t[..<k])
        rest = t[colon...]
    }
    guard rest.first == UInt8(ascii: ":"), rest.count == 1 || rest[rest.startIndex + 1] == UInt8(ascii: " ") else { return nil }
    return (key, text(bytes[..<(bytes.count - rest.count + 1)]), false)
}
