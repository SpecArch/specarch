import Foundation
import Yams

/// One node of a YAML file, with the line it starts on.
public final class YNode {
    public enum Kind { case mapping, sequence, scalar }
    public enum Style { case plain, quoted, literal, folded }

    public let kind: Kind
    public let value: String
    /// The resolved tag of a scalar: !!str, !!int, !!float, !!bool, !!null or !!timestamp.
    public let tag: String
    public let style: Style
    public let line: Int
    /// From 1, in Unicode characters.
    public let column: Int
    public var pairs: [(key: YNode, value: YNode)] = []
    public var items: [YNode] = []

    init(kind: Kind, value: String = "", tag: String = "", style: Style = .plain, line: Int, column: Int = 1) {
        self.kind = kind
        self.value = value
        self.tag = tag
        self.style = style
        self.line = line
        self.column = column
    }

    /// The value under key in a mapping, or the item at an index in a sequence.
    public func child(_ token: String) -> YNode? {
        switch kind {
        case .mapping:
            return pairs.first { $0.key.value == token }?.value
        case .sequence:
            if let i = Int(token), i >= 0, i < items.count { return items[i] }
            return nil
        case .scalar:
            return nil
        }
    }

    /// The key node for key in a mapping.
    public func key(_ key: String) -> YNode? {
        kind == .mapping ? pairs.first { $0.key.value == key }?.key : nil
    }

    /// A scalar's text, or "" for anything else.
    public var str: String { kind == .scalar ? value : "" }
}

/// A plain JSON value, for the schema check. Numbers keep their exact text.
public indirect enum JSONValue {
    case null
    case bool(Bool)
    case number(Decimal, String)
    case string(String)
    case array([JSONValue])
    case object([(String, JSONValue)])
}

/// Something wrong with the YAML itself, found while reading.
public struct Problem {
    public let line: Int
    public let path: String
    public let rule: String
    public let message: String
    /// The parser's own message, for a yaml_syntax error the parser gave.
    public var detail: String = ""
}

/// One parsed file.
public struct Doc {
    public var root: YNode?
    public var value: JSONValue = .null
    public var problems: [Problem] = []
    /// The pointers of the entries that did not parse, kept by their names only or left out.
    public var held: [String] = []
}

// MARK: - Reading

private let duplicateMarker = "specarch-repeated-key-marker-"

/// Parse reads one YAML document. A file that does not parse keeps what can
/// be read of it (see readable); when nothing can, the syntax error is
/// returned as a Problem, with the root left nil.
public func parseYAML(_ text: String) -> Doc {
    let (doc, syntax) = parseWhole(text)
    if syntax, let read = readable(text) {
        return read
    }
    return doc
}

/// Reads one YAML document as it is, and tells whether it failed on a
/// syntax error.
func parseWhole(_ text: String) -> (Doc, Bool) {
    var doc = Doc()
    var source = text
    // Yams refuses a mapping with a repeated key outright. To report each
    // repeat on its own line, as other implementations do, every later
    // repeat is renamed in place and the text is parsed again.
    var duplicates: [(key: String, first: Int, again: Int)] = []
    var shifts: [Int: Shift] = [:]
    var nodes: [Node] = []
    for _ in 0..<100 {
        do {
            let parser = try Parser(yaml: source, resolver: untaggedResolver)
            nodes = []
            while let node = try parser.nextRoot() {
                nodes.append(node)
                if nodes.count > 1 { break }
            }
            break
        } catch let YamlError.duplicatedKeysInMapping(keys, context) {
            guard let renamed = renameDuplicates(in: source, keys: keys, line: context.mark.line, column: context.mark.column, found: &duplicates, shifts: &shifts) else {
                doc.problems.append(Problem(line: context.mark.line, path: "/", rule: "yaml_syntax",
                                            message: "a mapping repeats the key \(keys.joined(separator: ", ")); give each key once"))
                return (doc, false)
            }
            source = renamed
        } catch let error as YamlError {
            doc.problems.append(syntaxProblem(source, error))
            return (doc, true)
        } catch {
            doc.problems.append(Problem(line: 1, path: "/", rule: "yaml_syntax", message: syntaxSentence["other"]!, detail: "\(error)"))
            return (doc, false)
        }
    }
    guard let first = nodes.first else {
        doc.problems.append(Problem(line: 1, path: "/", rule: "yaml_syntax", message: "the file is empty"))
        return (doc, false)
    }
    if nodes.count > 1 {
        doc.problems.append(Problem(line: nodes[1].mark?.line ?? 1, path: "/", rule: "yaml_syntax", message: "a file holds one YAML document; found a second"))
        return (doc, false)
    }
    let root = convert(first, shifts)
    if let p = flowProblem(text, root) {
        doc.problems.append(p)
        return (doc, true)
    }
    doc.root = root
    var problems: [Problem] = []
    doc.value = jsonValue(root, path: "", duplicates: duplicates, problems: &problems)
    doc.problems.append(contentsOf: problems)
    return (doc, false)
}

/// A parser's error in the validator's words, with the parser's own
/// message as its detail.
private func syntaxProblem(_ text: String, _ error: YamlError) -> Problem {
    // The line rule other SpecArch implementations follow, from yaml.v3. It
    // counts lines from 0 and takes the line where the error's context
    // starts, or, when that is the first line or there is none, the line of
    // the problem; a scanner error adds 1. A line of 0 is not given, and
    // reads as line 1. A reader or composer error gives no line in yaml.v3,
    // so it reads as line 1. For the unclosed-block messages the line given
    // is where the block starts, counted from 0 (syntaxProblemAt).
    var line = 0
    let problem: String
    switch error {
    case let .scanner(c, p, m, _), let .parser(c, p, m, _):
        var scanner = false
        if case .scanner = error { scanner = true }
        let contextLine = (c?.mark.line ?? 1) - 1, problemLine = m.line - 1
        if contextLine != 0 {
            line = contextLine + (scanner ? 1 : 0)
        } else if problemLine != 0 {
            line = problemLine + (scanner ? 1 : 0)
        }
        problem = p
    case let .reader(p, _, _, _), let .composer(_, p, _, _):
        problem = p
    default:
        problem = String(describing: error)
    }
    let given = line != 0
    if line == 0 { line = 1 }
    // Yams describes an error as its position, kind, context and problem,
    // then the line of the text it is on; the detail is the part before the
    // text, on one line.
    var detail = String(describing: error)
    if let r = detail.range(of: problem + ":\n") {
        detail = String(detail[..<r.lowerBound]) + problem
    }
    detail = detail.replacingOccurrences(of: "\n", with: " ")
    if problem == "control characters are not allowed" && text.unicodeScalars.contains(notUTF8) {
        detail = "the file holds bytes that are not UTF-8, read as U+FFFE; " + detail
    }
    let (at, message) = syntaxProblemAt(text, line, given, problem)
    return Problem(line: at, path: "/", rule: "yaml_syntax", message: message, detail: detail)
}

/// Renames every key of the block mapping at (line, column) that repeats an
/// earlier key, keeping the text's lines as they are. Returns nil when the
/// repeat cannot be found, such as in a flow mapping.
private func renameDuplicates(in text: String, keys: [String], line: Int, column: Int,
                              found: inout [(key: String, first: Int, again: Int)], shifts: inout [Int: Shift]) -> String? {
    var lines = text.components(separatedBy: "\n")
    let indent = column - 1
    var firstLine: [String: Int] = [:]
    var changed = false
    var i = line - 1
    while i < lines.count {
        let l = lines[i]
        let trimmed = l.drop { $0 == " " }
        let lead = l.count - trimmed.count
        if !trimmed.isEmpty && !trimmed.hasPrefix("#") {
            if lead < indent { break }
            if lead == indent {
                for key in keys {
                    for spelled in [key, "\"\(key)\"", "'\(key)'"] where trimmed.hasPrefix(spelled + ":") {
                        if let first = firstLine[key] {
                            let n = found.count
                            let replacement = duplicateMarker + "\(n)"
                            lines[i] = String(repeating: " ", count: lead) + replacement + trimmed.dropFirst(spelled.count)
                            found.append((key, first, i + 1))
                            shifts[i + 1] = Shift(after: lead + 1, by: replacement.unicodeScalars.count - spelled.unicodeScalars.count)
                            changed = true
                        } else {
                            firstLine[key] = i + 1
                        }
                    }
                }
            }
        }
        i += 1
    }
    return changed ? lines.joined(separator: "\n") : nil
}

/// How far a renamed repeated key moved the columns after it on its line,
/// so that a node keeps the column it has in the text as written.
struct Shift {
    let after: Int
    let by: Int
}

private func convert(_ n: Node, _ shifts: [Int: Shift]) -> YNode {
    let line = n.mark?.line ?? 1
    var column = n.mark?.column ?? 1
    if let s = shifts[line], column > s.after {
        column -= s.by
    }
    switch n {
    case .scalar(let s):
        let style: YNode.Style
        switch s.style {
        case .literal: style = .literal
        case .folded: style = .folded
        case .singleQuoted, .doubleQuoted: style = .quoted
        default: style = .plain
        }
        let tag = resolveTag(s.string, explicit: s.tag.description, style: style)
        return YNode(kind: .scalar, value: s.string, tag: tag, style: style, line: line, column: column)
    case .mapping(let m):
        let node = YNode(kind: .mapping, line: line, column: column)
        for (k, v) in m { node.pairs.append((convert(k, shifts), convert(v, shifts))) }
        return node
    case .sequence(let q):
        let node = YNode(kind: .sequence, line: line, column: column)
        node.items = q.map { convert($0, shifts) }
        return node
    case .alias:
        return YNode(kind: .scalar, value: "", tag: "!!null", line: line, column: column)
    }
}

/// The tag Yams gives a scalar written with no tag, so that its own
/// resolver, which follows YAML 1.1 and runs when a mapping's keys are
/// compared, never takes the place of the tag written.
private let untagged = Tag.Name(rawValue: "specarch:untagged")
let untaggedResolver = Resolver.basic.appending(try! Resolver.Rule(untagged, "^"))

private let floatPattern = try! NSRegularExpression(pattern: "^[-+]?(\\.[0-9]+|[0-9]+(\\.[0-9]*)?)([eE][-+]?[0-9]+)?$")
private let dotFloatPattern = try! NSRegularExpression(pattern: "^\\.[0-9]+([eE][-+]?[0-9]+)?$")

func matches(_ re: NSRegularExpression, _ s: String) -> Bool {
    re.firstMatch(in: s, range: NSRange(s.startIndex..., in: s)) != nil
}

/// A scalar's tag as yaml.v3 sets it while parsing, so that both builds
/// read every scalar alike (ADR-176): a tag written other than ! is kept,
/// tag:yaml.org,2002:int as !!int; a quoted or block scalar is !!str; a
/// plain one takes the tag resolve gives its text.
private func resolveTag(_ v: String, explicit: String, style: YNode.Style) -> String {
    if explicit != "", explicit != "!", explicit != untagged.rawValue {
        let prefix = "tag:yaml.org,2002:"
        return explicit.hasPrefix(prefix) ? "!!" + explicit.dropFirst(prefix.count) : explicit
    }
    if style != .plain { return "!!str" }
    return resolveScalar("", v)?.tag ?? "!!str"
}

/// A scalar's value as yaml.v3 resolves it.
enum Resolved {
    case null, bool(Bool), int(Int64), uint(UInt64), float(Double), text
}

private let resolveMap: [String: (String, Resolved)] = {
    var m: [String: (String, Resolved)] = [:]
    for s in ["true", "True", "TRUE"] { m[s] = ("!!bool", .bool(true)) }
    for s in ["false", "False", "FALSE"] { m[s] = ("!!bool", .bool(false)) }
    for s in ["", "~", "null", "Null", "NULL"] { m[s] = ("!!null", .null) }
    for s in [".nan", ".NaN", ".NAN"] { m[s] = ("!!float", .float(.nan)) }
    for s in [".inf", ".Inf", ".INF", "+.inf", "+.Inf", "+.INF"] { m[s] = ("!!float", .float(.infinity)) }
    for s in ["-.inf", "-.Inf", "-.INF"] { m[s] = ("!!float", .float(-.infinity)) }
    m["<<"] = ("!!merge", .text)
    return m
}()

/// yaml.v3's resolve(tag, in) (ADR-176): the tag a scalar's text resolves
/// to under no tag ("") or a tag of the core schema, and its value; nil
/// where yaml.v3 fails, a text that does not fit the tag given, such as
/// !!int 1.5 or !!bool yes. An !!int text under !!float is its float. Any
/// other tag is kept, its value text.
func resolveScalar(_ tag: String, _ v: String) -> (tag: String, value: Resolved)? {
    guard ["", "!!str", "!!bool", "!!int", "!!float", "!!null", "!!timestamp"].contains(tag) else { return (tag, .text) }
    let (rtag, out) = resolveText(tag, v)
    if tag == "" || tag == rtag || tag == "!!str" { return (rtag, out) }
    if tag == "!!float", rtag == "!!int", case .int(let i) = out { return ("!!float", .float(Double(i))) }
    return nil
}

/// The body of yaml.v3's resolve: its first character hints at what a
/// text may be; a digit or a sign is tried as a timestamp (with no tag or
/// !!timestamp), then, with every underscore dropped, as an integer by its
/// prefix, a float, and a 0b or 0o integer with a sign after the prefix.
private func resolveText(_ tag: String, _ v: String) -> (String, Resolved) {
    let first = v.utf8.first
    let hint: Character?
    switch first {
    case nil: hint = "N"
    case UInt8(ascii: "+"), UInt8(ascii: "-"): hint = "S"
    case let c? where c >= UInt8(ascii: "0") && c <= UInt8(ascii: "9"): hint = "D"
    case let c? where "yYnNtTfFoO~".utf8.contains(c): hint = "M"
    case UInt8(ascii: "."): hint = "."
    default: hint = nil
    }
    guard let hint, tag != "!!str" else { return ("!!str", .text) }
    if let item = resolveMap[v] { return item }
    switch hint {
    case ".":
        if let f = goParseFloat(v) { return ("!!float", .float(f)) }
    case "D", "S":
        if tag == "" || tag == "!!timestamp", isTimestamp(v) { return ("!!timestamp", .text) }
        let plain = v.replacingOccurrences(of: "_", with: "")
        if let i = goParseInt(plain, base: 0) { return ("!!int", .int(i)) }
        if let u = goParseUint(plain, base: 0) { return ("!!int", .uint(u)) }
        if matches(floatPattern, plain), let f = Double(plain), f.isFinite { return ("!!float", .float(f)) }
        for (prefix, base) in [("0b", 2), ("0o", 8)] {
            if plain.hasPrefix(prefix) {
                let digits = String(plain.dropFirst(2))
                if let i = goParseInt(digits, base: base) { return ("!!int", .int(i)) }
                if let u = goParseUint(digits, base: base) { return ("!!int", .uint(u)) }
            } else if plain.hasPrefix("-" + prefix) {
                if let i = goParseInt("-" + plain.dropFirst(3), base: base) { return ("!!int", .int(i)) }
            }
        }
    default:
        break
    }
    return ("!!str", .text)
}

/// Go's strconv.ParseInt with underscores already dropped: a sign, then
/// the digits ParseUint takes; nil past Int64.
func goParseInt(_ s: String, base: Int) -> Int64? {
    var body = Substring(s)
    var negative = false
    if let f = body.first, f == "-" || f == "+" {
        negative = f == "-"
        body = body.dropFirst()
    }
    guard let magnitude = goParseUint(String(body), base: base) else { return nil }
    if negative {
        if magnitude == UInt64(Int64.max) + 1 { return Int64.min }
        return magnitude <= UInt64(Int64.max) ? -Int64(magnitude) : nil
    }
    return magnitude <= UInt64(Int64.max) ? Int64(magnitude) : nil
}

/// Go's strconv.ParseUint with no underscores: no sign; base 0 reads 0x,
/// 0o and 0b before at least one more character, and any other leading 0
/// as octal, so 017 is 15 and 0 is 0; nil past UInt64.
func goParseUint(_ s: String, base: Int) -> UInt64? {
    var b = Array(s.utf8)
    guard !b.isEmpty else { return nil }
    var radix = UInt64(base)
    if base == 0 {
        radix = 10
        if b[0] == UInt8(ascii: "0") {
            let second = b.count >= 3 ? b[1] | 0x20 : 0
            switch second {
            case UInt8(ascii: "b"): radix = 2; b.removeFirst(2)
            case UInt8(ascii: "o"): radix = 8; b.removeFirst(2)
            case UInt8(ascii: "x"): radix = 16; b.removeFirst(2)
            default: radix = 8; b.removeFirst()
            }
        }
    }
    var n: UInt64 = 0
    for c in b {
        let d: UInt64
        switch c {
        case UInt8(ascii: "0")...UInt8(ascii: "9"): d = UInt64(c - UInt8(ascii: "0"))
        case UInt8(ascii: "a")...UInt8(ascii: "z"): d = UInt64(c - UInt8(ascii: "a")) + 10
        case UInt8(ascii: "A")...UInt8(ascii: "Z"): d = UInt64(c - UInt8(ascii: "A")) + 10
        default: return nil
        }
        guard d < radix else { return nil }
        let (m, o1) = n.multipliedReportingOverflow(by: radix)
        let (a, o2) = m.addingReportingOverflow(d)
        if o1 || o2 { return nil }
        n = a
    }
    return n
}

/// Go's strconv.ParseFloat for a text that starts with a dot, the only
/// one resolve gives it as written: an underscore only between two digits,
/// then dropped; nil when it is no number or past a float's range.
private func goParseFloat(_ v: String) -> Double? {
    let chars = Array(v.utf8)
    for (i, c) in chars.enumerated() where c == UInt8(ascii: "_") {
        guard i > 0, i + 1 < chars.count, isDigit(chars[i - 1]), isDigit(chars[i + 1]) else { return nil }
    }
    let clean = v.replacingOccurrences(of: "_", with: "")
    guard matches(dotFloatPattern, clean), let f = Double(clean), f.isFinite else { return nil }
    return f
}

private func isDigit(_ c: UInt8) -> Bool { c >= UInt8(ascii: "0") && c <= UInt8(ascii: "9") }

/// Whether a text is a timestamp as yaml.v3 reads one: four digits and a
/// dash, then Go's time.Parse of one of its layouts, 2006-1-2T15:4:5.999999999Z07:00,
/// the same with a small t, 2006-1-2 15:4:5.999999999 and 2006-1-2. A
/// month or day takes one or two digits, a fraction a dot or a comma and
/// any number of digits, the space one or more spaces; the date must be
/// one of the calendar, the time within the day, a zone Z or ±hh:mm.
func isTimestamp(_ s: String) -> Bool {
    let b = Array(s.utf8)
    var i = 0
    func num() -> Int? {
        guard i < b.count, isDigit(b[i]) else { return nil }
        var n = Int(b[i] - 48)
        i += 1
        if i < b.count, isDigit(b[i]) { n = n * 10 + Int(b[i] - 48); i += 1 }
        return n
    }
    func take(_ c: Character) -> Bool {
        guard i < b.count, b[i] == c.asciiValue! else { return false }
        i += 1
        return true
    }
    guard b.count > 4, b[0..<4].allSatisfy(isDigit), b[4] == UInt8(ascii: "-") else { return false }
    let year = b[0..<4].reduce(0) { $0 * 10 + Int($1 - 48) }
    i = 5
    guard let month = num(), take("-"), let day = num() else { return false }
    guard (1...12).contains(month) else { return false }
    let leap = year % 4 == 0 && (year % 100 != 0 || year % 400 == 0)
    let days = [31, leap ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31][month - 1]
    guard (1...days).contains(day) else { return false }
    if i == b.count { return true }
    let separator = b[i]
    i += 1
    if separator == UInt8(ascii: " ") {
        while i < b.count, b[i] == UInt8(ascii: " ") { i += 1 }
    } else if separator != UInt8(ascii: "T") && separator != UInt8(ascii: "t") {
        return false
    }
    guard let hour = num(), take(":"), let minute = num(), take(":"), let second = num(),
          hour < 24, minute < 60, second < 60 else { return false }
    if i + 1 < b.count, b[i] == UInt8(ascii: ".") || b[i] == UInt8(ascii: ","), isDigit(b[i + 1]) {
        i += 1
        while i < b.count, isDigit(b[i]) { i += 1 }
    }
    if separator == UInt8(ascii: " ") { return i == b.count }
    if take("Z") { return i == b.count }
    guard b.count - i == 6, b[i] == UInt8(ascii: "+") || b[i] == UInt8(ascii: "-"), b[i + 3] == UInt8(ascii: ":"),
          isDigit(b[i + 1]), isDigit(b[i + 2]), isDigit(b[i + 4]), isDigit(b[i + 5]) else { return false }
    let zoneHour = Int(b[i + 1] - 48) * 10 + Int(b[i + 2] - 48)
    let zoneMinute = Int(b[i + 4] - 48) * 10 + Int(b[i + 5] - 48)
    return zoneHour <= 24 && zoneMinute <= 60
}

private func jsonValue(_ n: YNode, path: String, duplicates: [(key: String, first: Int, again: Int)], problems: inout [Problem]) -> JSONValue {
    switch n.kind {
    case .mapping:
        // A repeated key stays in the node tree under its own name, as
        // yaml.v3 keeps it, and is left out of the plain value.
        var out: [(String, JSONValue)] = []
        for (i, (k, v)) in n.pairs.enumerated() {
            if k.value.hasPrefix(duplicateMarker), let j = Int(k.value.dropFirst(duplicateMarker.count)), j < duplicates.count {
                let d = duplicates[j]
                problems.append(Problem(line: d.again, path: path + "/" + escapeToken(d.key), rule: "duplicate_key",
                                        message: "key \(quote(d.key)) is already defined on line \(d.first)"))
                n.pairs[i].key = YNode(kind: .scalar, value: d.key, tag: "!!str", style: k.style, line: k.line)
                restoreKeys(v, duplicates)
                continue
            }
            out.append((k.value, jsonValue(v, path: path + "/" + escapeToken(k.value), duplicates: duplicates, problems: &problems)))
        }
        return .object(out)
    case .sequence:
        return .array(n.items.enumerated().map { jsonValue($1, path: path + "/\($0)", duplicates: duplicates, problems: &problems) })
    case .scalar:
        if n.tag == "!!timestamp" {
            problems.append(Problem(line: n.line, path: path.isEmpty ? "/" : path, rule: "unquoted_date",
                                    message: "\(n.value) is read by YAML as a timestamp; quote it: \"\(n.value)\""))
        }
        return scalarValue(n)
    }
}

/// Gives a repeated key inside a value left out of the plain value its own
/// name back, reporting nothing, as yaml.v3 reports nothing there.
private func restoreKeys(_ n: YNode, _ duplicates: [(key: String, first: Int, again: Int)]) {
    for (i, (k, v)) in n.pairs.enumerated() {
        if k.value.hasPrefix(duplicateMarker), let j = Int(k.value.dropFirst(duplicateMarker.count)), j < duplicates.count {
            n.pairs[i].key = YNode(kind: .scalar, value: duplicates[j].key, tag: "!!str", style: k.style, line: k.line)
        }
        restoreKeys(v, duplicates)
    }
    for item in n.items { restoreKeys(item, duplicates) }
}

/// A scalar as a plain JSON value, read as the Go build reads it from its
/// tag (ADR-176): a number or a bool only where yaml.v3 decodes the text
/// as one under that tag, and a float only when it is finite; anything
/// else, a timestamp included, is its text.
func scalarValue(_ n: YNode) -> JSONValue {
    switch n.tag {
    case "!!null":
        return .null
    case "!!bool":
        if case .bool(let b)? = resolveScalar(n.tag, n.value)?.value { return .bool(b) }
    case "!!int":
        switch resolveScalar(n.tag, n.value)?.value {
        case .int(let i)?: return .number(Decimal(i), String(i))
        case .uint(let u)?: return .number(Decimal(string: String(u))!, String(u))
        default: break
        }
    case "!!float":
        if let d = decodeFloat(n), d.isFinite {
            let text = "\(d)".replacingOccurrences(of: ".0e", with: "e")
            return .number(Decimal(string: text) ?? Decimal(d), text)
        }
    default:
        break
    }
    return .string(n.value)
}

/// A scalar decoded as a float, as yaml.v3 decodes one into a float64: an
/// integer or a float under its tag, the infinities and NaN included; nil
/// for anything else.
func decodeFloat(_ n: YNode) -> Double? {
    guard n.kind == .scalar, n.tag != "!!str" else { return nil }
    switch resolveScalar(n.tag, n.value)?.value {
    case .int(let i)?: return Double(i)
    case .uint(let u)?: return Double(u)
    case .float(let f)?: return f
    default: return nil
    }
}

// MARK: - Pointers

public func escapeToken(_ s: String) -> String {
    s.replacingOccurrences(of: "~", with: "~0").replacingOccurrences(of: "/", with: "~1")
}

public func unescapeToken(_ s: String) -> String {
    s.replacingOccurrences(of: "~1", with: "/").replacingOccurrences(of: "~0", with: "~")
}

public func pointer(_ tokens: [String]) -> String {
    tokens.isEmpty ? "/" : tokens.map { "/" + escapeToken($0) }.joined()
}

public func pointer(_ tokens: String...) -> String { pointer(tokens) }

/// Follows tokens from n; returns the deepest node reached and whether it
/// reached the end.
public func resolve(_ n: YNode, _ tokens: [String]) -> (YNode, Bool) {
    var cur = n
    for t in tokens {
        guard let next = cur.child(t) else { return (cur, false) }
        cur = next
    }
    return (cur, true)
}

/// Go's %q for the strings SpecArch prints.
public func quote(_ s: String) -> String {
    var out = "\""
    for c in s.unicodeScalars {
        switch c {
        case "\"": out += "\\\""
        case "\\": out += "\\\\"
        case "\n": out += "\\n"
        case "\t": out += "\\t"
        default: out.unicodeScalars.append(c)
        }
    }
    return out + "\""
}
