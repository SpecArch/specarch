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
    public var pairs: [(key: YNode, value: YNode)] = []
    public var items: [YNode] = []

    init(kind: Kind, value: String = "", tag: String = "", style: Style = .plain, line: Int) {
        self.kind = kind
        self.value = value
        self.tag = tag
        self.style = style
        self.line = line
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

/// Parse reads one YAML document. A syntax error is returned as a Problem,
/// with the root left nil.
public func parseYAML(_ text: String) -> Doc {
    var doc = Doc()
    var source = text
    // Yams refuses a mapping with a repeated key outright. To report each
    // repeat on its own line, as other implementations do, every later
    // repeat is renamed in place and the text is parsed again.
    var duplicates: [(key: String, first: Int, again: Int)] = []
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
            guard let renamed = renameDuplicates(in: source, keys: keys, line: context.mark.line, column: context.mark.column, found: &duplicates) else {
                doc.problems.append(Problem(line: context.mark.line, path: "/", rule: "yaml_syntax",
                                            message: "a mapping repeats the key \(keys.joined(separator: ", ")); give each key once"))
                return doc
            }
            source = renamed
        } catch let error as YamlError {
            doc.problems.append(syntaxProblem(error))
            return doc
        } catch {
            doc.problems.append(Problem(line: 1, path: "/", rule: "yaml_syntax", message: "\(error)"))
            return doc
        }
    }
    guard let first = nodes.first else {
        doc.problems.append(Problem(line: 1, path: "/", rule: "yaml_syntax", message: "the file is empty"))
        return doc
    }
    if nodes.count > 1 {
        doc.problems.append(Problem(line: nodes[1].mark?.line ?? 1, path: "/", rule: "yaml_syntax", message: "a file holds one YAML document; found a second"))
        return doc
    }
    let root = convert(first)
    doc.root = root
    var problems: [Problem] = []
    doc.value = jsonValue(root, path: "", duplicates: duplicates, problems: &problems)
    doc.problems.append(contentsOf: problems)
    return doc
}

private func syntaxProblem(_ error: YamlError) -> Problem {
    // The line rule other SpecArch implementations follow, from yaml.v3: a
    // scanner error is reported on the line where its context starts, or
    // where the problem is; a parser error on the line before that, which
    // the unclosed-block messages below correct.
    switch error {
    case let .scanner(context, problem, mark, _):
        return Problem(line: max(context?.mark.line ?? mark.line, 1), path: "/", rule: "yaml_syntax", message: unclosed[problem] ?? problem)
    case let .parser(context, problem, mark, _):
        var line = (context?.mark.line ?? mark.line) - 1
        var message = problem
        if let plain = unclosed[problem] {
            line += 1
            message = plain
        }
        return Problem(line: max(line, 1), path: "/", rule: "yaml_syntax", message: message)
    default:
        return Problem(line: 1, path: "/", rule: "yaml_syntax", message: "\(error)")
    }
}

/// Renames every key of the block mapping at (line, column) that repeats an
/// earlier key, keeping the text's lines as they are. Returns nil when the
/// repeat cannot be found, such as in a flow mapping.
private func renameDuplicates(in text: String, keys: [String], line: Int, column: Int,
                              found: inout [(key: String, first: Int, again: Int)]) -> String? {
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

private func convert(_ n: Node) -> YNode {
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
        return YNode(kind: .scalar, value: s.string, tag: tag, style: style, line: s.mark?.line ?? 1)
    case .mapping(let m):
        let node = YNode(kind: .mapping, line: m.mark?.line ?? 1)
        for (k, v) in m { node.pairs.append((convert(k), convert(v))) }
        return node
    case .sequence(let q):
        let node = YNode(kind: .sequence, line: q.mark?.line ?? 1)
        node.items = q.map(convert)
        return node
    case .alias:
        return YNode(kind: .scalar, value: "", tag: "!!null", line: n.mark?.line ?? 1)
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
        var out: [(String, JSONValue)] = []
        var kept: [YNode] = []
        for (k, v) in n.pairs {
            if k.value.hasPrefix(duplicateMarker), let i = Int(k.value.dropFirst(duplicateMarker.count)), i < duplicates.count {
                let d = duplicates[i]
                problems.append(Problem(line: d.again, path: path + "/" + escapeToken(d.key), rule: "duplicate_key",
                                        message: "key \(quote(d.key)) is already defined on line \(d.first)"))
                continue
            }
            kept.append(k)
            out.append((k.value, jsonValue(v, path: path + "/" + escapeToken(k.value), duplicates: duplicates, problems: &problems)))
        }
        n.pairs = n.pairs.filter { !$0.key.value.hasPrefix(duplicateMarker) }
        return .object(out)
    case .sequence:
        return .array(n.items.enumerated().map { jsonValue($1, path: path + "/\($0)", duplicates: duplicates, problems: &problems) })
    case .scalar:
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
        case "!!timestamp":
            problems.append(Problem(line: n.line, path: path.isEmpty ? "/" : path, rule: "unquoted_date",
                                    message: "\(n.value) is read by YAML as a timestamp; quote it: \"\(n.value)\""))
            return .string(n.value)
        default:
            return .string(n.value)
        }
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
