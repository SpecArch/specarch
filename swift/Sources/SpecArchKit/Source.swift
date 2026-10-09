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

/// The parser errors whose line the YAML library gives as the start of the
/// unclosed block, and a plain sentence for each.
let unclosed: [String: String] = [
    "did not find expected ',' or '}'": "the { that starts on this line is not closed; close it with }, and separate its entries with commas",
    "did not find expected ',' or ']'": "the [ that starts on this line is not closed; close it with ], and separate its items with commas",
    "did not find expected key": "the mapping that starts on this line has a line that is not a key; check the indentation below it",
    "did not find expected '-' indicator": "the list that starts on this line has a line that is not an item; check the indentation below it",
]

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
            let parser = try Parser(yaml: source)
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
            doc.problems.append(syntaxProblem(error))
            return (doc, true)
        } catch {
            doc.problems.append(Problem(line: 1, path: "/", rule: "yaml_syntax", message: "\(error)"))
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
    doc.root = root
    var problems: [Problem] = []
    doc.value = jsonValue(root, path: "", duplicates: duplicates, problems: &problems)
    doc.problems.append(contentsOf: problems)
    return (doc, false)
}

private func syntaxProblem(_ error: YamlError) -> Problem {
    // The line rule other SpecArch implementations follow, from yaml.v3. It
    // counts lines from 0 and takes the line where the error's context
    // starts, or, when that is the first line or there is none, the line of
    // the problem; a scanner error adds 1. A line of 0 is not given, and
    // reads as line 1. For the unclosed-block messages below the line given
    // is where the block starts, counted from 0, so 1 is added.
    let context: Mark?, mark: Mark, problem: String, scanner: Bool
    switch error {
    case let .scanner(c, p, m, _):
        (context, problem, mark, scanner) = (c?.mark, p, m, true)
    case let .parser(c, p, m, _):
        (context, problem, mark, scanner) = (c?.mark, p, m, false)
    default:
        return Problem(line: 1, path: "/", rule: "yaml_syntax", message: "\(error)")
    }
    let contextLine = (context?.line ?? 1) - 1, problemLine = mark.line - 1
    var line = 0
    if contextLine != 0 {
        line = contextLine + (scanner ? 1 : 0)
    } else if problemLine != 0 {
        line = problemLine + (scanner ? 1 : 0)
    }
    if line == 0 { line = 1 }
    var message = problem
    if let plain = unclosed[problem] {
        line += 1
        message = plain
    }
    return Problem(line: line, path: "/", rule: "yaml_syntax", message: message)
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

private let intPattern = try! NSRegularExpression(pattern: "^[-+]?(0|[1-9][0-9_]*|0[0-7_]+|0o[0-7_]+|0x[0-9a-fA-F_]+|0b[01_]+)$")
private let floatPattern = try! NSRegularExpression(pattern: "^[-+]?(\\.[0-9]+|[0-9]+(\\.[0-9]*)?)([eE][-+]?[0-9]+)?$")
private let timestampPattern = try! NSRegularExpression(pattern: "^[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}([Tt ][0-9]{1,2}:[0-9]{1,2}:[0-9]{1,2}(\\.[0-9]+)?([Zz]|[-+][0-9]{1,2}(:[0-9]{2})?)?)?$")

func matches(_ re: NSRegularExpression, _ s: String) -> Bool {
    re.firstMatch(in: s, range: NSRange(s.startIndex..., in: s)) != nil
}

/// Resolves a scalar's tag the way YAML 1.2's core schema does: quoted and
/// block scalars are strings; a plain scalar is null, a bool, an int, a
/// float, a timestamp or a string by its form.
private func resolveTag(_ v: String, explicit: String, style: YNode.Style) -> String {
    switch explicit {
    case "tag:yaml.org,2002:str" where style != .plain: return "!!str"
    case "tag:yaml.org,2002:int": return "!!int"
    case "tag:yaml.org,2002:float": return "!!float"
    case "tag:yaml.org,2002:bool": return "!!bool"
    case "tag:yaml.org,2002:null": return "!!null"
    case "tag:yaml.org,2002:timestamp": return "!!timestamp"
    default: break
    }
    if style != .plain { return "!!str" }
    switch v {
    case "", "~", "null", "Null", "NULL": return "!!null"
    case "true", "True", "TRUE", "false", "False", "FALSE": return "!!bool"
    case ".inf", ".Inf", ".INF", "+.inf", "+.Inf", "+.INF", "-.inf", "-.Inf", "-.INF", ".nan", ".NaN", ".NAN": return "!!float"
    default: break
    }
    if matches(intPattern, v) { return "!!int" }
    if matches(floatPattern, v) { return "!!float" }
    if matches(timestampPattern, v) { return "!!timestamp" }
    return "!!str"
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

/// A scalar as a plain JSON value; a timestamp is its text.
func scalarValue(_ n: YNode) -> JSONValue {
    switch n.tag {
    case "!!null": return .null
    case "!!bool": return .bool(n.value.lowercased() == "true")
    case "!!int":
        let clean = n.value.replacingOccurrences(of: "_", with: "")
        if let i = Int64(clean) { return .number(Decimal(i), String(i)) }
        if let u = UInt64(clean) { return .number(Decimal(string: String(u))!, String(u)) }
        return .string(n.value)
    case "!!float":
        if let d = Double(n.value), d.isFinite {
            let text = "\(d)".replacingOccurrences(of: ".0e", with: "e")
            return .number(Decimal(string: n.value) ?? Decimal(d), text)
        }
        return .string(n.value)
    default:
        return .string(n.value)
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
