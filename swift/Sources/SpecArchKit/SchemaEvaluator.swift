import Foundation

/// A JSON Schema 2020-12 evaluator for exactly the keywords SpecArch's two
/// schemas use. A schema with any other keyword is refused when it is
/// loaded, so a schema change cannot be ignored without anyone noticing.
/// It holds only what it read at load time, so it is safe to share.
final class SchemaEvaluator: @unchecked Sendable {
    /// One failed keyword, at an instance location.
    struct Failure {
        indirect enum Kind {
            case required([String])
            case additionalProperties([String])
            case type(got: String, want: [String])
            case enumValue(got: JSONValue, want: [JSONValue])
            case constValue(want: JSONValue)
            case pattern(got: String, want: String)
            case format(got: JSONValue, want: String)
            case minItems(got: Int, want: Int)
            case maxItems(got: Int, want: Int)
            case maxProperties(got: Int, want: Int)
            case minLength(got: Int, want: Int)
            case minProperties(got: Int, want: Int)
            case uniqueItems(Int, Int)
            case minimum(got: Decimal, want: Decimal)
            case exclusiveMinimum(got: Decimal, want: Decimal)
            case oneOfMany([Int])
            case not
            case falseSchema
            case propertyName(String, causes: [Failure])
            case contains
        }
        let kind: Kind
        let location: [String]
    }

    struct LoadError: Error, CustomStringConvertible {
        let description: String
    }

    private let root: [String: Any]

    static let known: Set<String> = [
        "$schema", "$id", "$ref", "$defs", "title", "description", "default", "examples",
        "type", "properties", "additionalProperties", "patternProperties", "propertyNames",
        "required", "enum", "const", "pattern", "format", "minItems", "maxItems", "uniqueItems",
        "minLength", "maxLength", "minimum", "maximum", "exclusiveMinimum", "minProperties", "maxProperties", "items",
        "oneOf", "anyOf", "allOf", "if", "then", "else", "not", "contains", "dependentRequired",
    ]

    init(json: String) throws {
        guard let data = json.data(using: .utf8),
              let obj = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw LoadError(description: "the schema is not a JSON object")
        }
        root = obj
        try Self.checkKeywords(obj, at: "#")
    }

    /// Refuses any keyword this evaluator does not implement.
    private static func checkKeywords(_ schema: Any, at path: String) throws {
        guard let s = schema as? [String: Any] else { return }
        for (k, v) in s {
            if !known.contains(k) {
                throw LoadError(description: "the schema uses \(k) at \(path), which this evaluator does not implement")
            }
            switch k {
            case "$defs", "properties", "patternProperties", "dependentRequired":
                if k == "dependentRequired" { continue }
                for (name, sub) in v as? [String: Any] ?? [:] {
                    try checkKeywords(sub, at: path + "/" + k + "/" + name)
                }
            case "additionalProperties", "propertyNames", "items", "if", "then", "else", "not", "contains":
                try checkKeywords(v, at: path + "/" + k)
            case "oneOf", "anyOf", "allOf":
                for (i, sub) in (v as? [Any] ?? []).enumerated() {
                    try checkKeywords(sub, at: path + "/\(k)/\(i)")
                }
            default:
                break
            }
        }
    }

    func validate(_ value: JSONValue) -> [Failure] {
        evaluate(root, value, [])
    }

    private func resolveRef(_ ref: String) -> Any {
        guard ref.hasPrefix("#/") else { return [String: Any]() }
        var cur: Any = root
        for token in ref.dropFirst(2).split(separator: "/") {
            cur = (cur as? [String: Any])?[unescapeToken(String(token))] ?? [String: Any]()
        }
        return cur
    }

    private func evaluate(_ schemaAny: Any, _ value: JSONValue, _ loc: [String]) -> [Failure] {
        if let b = schemaAny as? Bool {
            return b ? [] : [Failure(kind: .falseSchema, location: loc)]
        }
        guard let s = schemaAny as? [String: Any] else { return [] }
        var out: [Failure] = []

        if let ref = s["$ref"] as? String {
            out += evaluate(resolveRef(ref), value, loc)
        }
        if let t = s["type"] {
            let want = (t as? [String]) ?? [(t as? String) ?? ""]
            if !want.contains(where: { typeMatches($0, value) }) {
                out.append(Failure(kind: .type(got: typeName(value), want: want), location: loc))
                return out
            }
        }
        if let c = s["const"], !jsonEqual(value, fromAny(c)) {
            out.append(Failure(kind: .constValue(want: fromAny(c)), location: loc))
        }
        if let e = s["enum"] as? [Any] {
            let want = e.map(fromAny)
            if !want.contains(where: { jsonEqual(value, $0) }) {
                out.append(Failure(kind: .enumValue(got: value, want: want), location: loc))
            }
        }
        switch value {
        case .string(let str):
            if let min = s["minLength"] as? Int, str.unicodeScalars.count < min {
                out.append(Failure(kind: .minLength(got: str.unicodeScalars.count, want: min), location: loc))
            }
            if let p = s["pattern"] as? String, let re = try? NSRegularExpression(pattern: p), !matches(re, str) {
                out.append(Failure(kind: .pattern(got: str, want: p), location: loc))
            }
            if let f = s["format"] as? String, !formatHolds(f, str) {
                out.append(Failure(kind: .format(got: value, want: f), location: loc))
            }
        case .number(let n, _):
            if let m = s["minimum"], let want = decimal(m), n < want {
                out.append(Failure(kind: .minimum(got: n, want: want), location: loc))
            }
            if let m = s["exclusiveMinimum"], let want = decimal(m), n <= want {
                out.append(Failure(kind: .exclusiveMinimum(got: n, want: want), location: loc))
            }
        case .array(let items):
            if let min = s["minItems"] as? Int, items.count < min {
                out.append(Failure(kind: .minItems(got: items.count, want: min), location: loc))
            }
            if let max = s["maxItems"] as? Int, items.count > max {
                out.append(Failure(kind: .maxItems(got: items.count, want: max), location: loc))
            }
            if (s["uniqueItems"] as? Bool) == true {
                outer: for i in 0..<items.count {
                    for j in (i + 1)..<max(i + 1, items.count) where jsonEqual(items[i], items[j]) {
                        out.append(Failure(kind: .uniqueItems(i, j), location: loc))
                        break outer
                    }
                }
            }
            if let itemSchema = s["items"] {
                for (i, item) in items.enumerated() {
                    out += evaluate(itemSchema, item, loc + ["\(i)"])
                }
            }
            if let contains = s["contains"] {
                if !items.contains(where: { evaluate(contains, $0, []).isEmpty }) {
                    out.append(Failure(kind: .contains, location: loc))
                }
            }
        case .object(let pairs):
            let keys = pairs.map { $0.0 }
            if let min = s["minProperties"] as? Int, pairs.count < min {
                out.append(Failure(kind: .minProperties(got: pairs.count, want: min), location: loc))
            }
            if let max = s["maxProperties"] as? Int, pairs.count > max {
                out.append(Failure(kind: .maxProperties(got: pairs.count, want: max), location: loc))
            }
            if let req = s["required"] as? [String] {
                let missing = req.filter { !keys.contains($0) }
                if !missing.isEmpty {
                    out.append(Failure(kind: .required(missing), location: loc))
                }
            }
            if let dep = s["dependentRequired"] as? [String: [String]] {
                for (prop, needs) in dep.sorted(by: { $0.key < $1.key }) where keys.contains(prop) {
                    let missing = needs.filter { !keys.contains($0) }
                    if !missing.isEmpty {
                        out.append(Failure(kind: .required(missing), location: loc))
                    }
                }
            }
            if let names = s["propertyNames"] {
                for k in keys {
                    let causes = evaluate(names, .string(k), loc)
                    if !causes.isEmpty {
                        out.append(Failure(kind: .propertyName(k, causes: causes), location: loc))
                    }
                }
            }
            let props = s["properties"] as? [String: Any] ?? [:]
            let patterns = (s["patternProperties"] as? [String: Any] ?? [:]).compactMap { p, sub -> (NSRegularExpression, Any)? in
                guard let re = try? NSRegularExpression(pattern: p) else { return nil }
                return (re, sub)
            }
            var extra: [String] = []
            for (k, v) in pairs {
                var evaluated = false
                if let sub = props[k] {
                    out += evaluate(sub, v, loc + [k])
                    evaluated = true
                }
                for (re, sub) in patterns where matches(re, k) {
                    out += evaluate(sub, v, loc + [k])
                    evaluated = true
                }
                if !evaluated, let add = s["additionalProperties"] {
                    if let b = add as? Bool {
                        if !b { extra.append(k) }
                    } else {
                        out += evaluate(add, v, loc + [k])
                    }
                }
            }
            if !extra.isEmpty {
                out.append(Failure(kind: .additionalProperties(extra), location: loc))
            }
        default:
            break
        }
        if let all = s["allOf"] as? [Any] {
            for sub in all { out += evaluate(sub, value, loc) }
        }
        if let any = s["anyOf"] as? [Any] {
            let results = any.map { evaluate($0, value, loc) }
            if !results.contains(where: { $0.isEmpty }) {
                out += results.flatMap { $0 }
            }
        }
        if let one = s["oneOf"] as? [Any] {
            let results = one.map { evaluate($0, value, loc) }
            let matched = results.enumerated().filter { $0.element.isEmpty }.map { $0.offset }
            if matched.isEmpty {
                out += results.flatMap { $0 }
            } else if matched.count > 1 {
                out.append(Failure(kind: .oneOfMany(matched), location: loc))
            }
        }
        if let not = s["not"], evaluate(not, value, loc).isEmpty {
            out.append(Failure(kind: .not, location: loc))
        }
        if let cond = s["if"] {
            if evaluate(cond, value, loc).isEmpty {
                if let then = s["then"] { out += evaluate(then, value, loc) }
            } else if let els = s["else"] {
                out += evaluate(els, value, loc)
            }
        }
        return out
    }

    private func typeMatches(_ t: String, _ v: JSONValue) -> Bool {
        switch (t, v) {
        case ("null", .null), ("boolean", .bool), ("string", .string), ("array", .array), ("object", .object), ("number", .number):
            return true
        case ("integer", .number(let n, _)):
            var r = Decimal()
            var x = n
            NSDecimalRound(&r, &x, 0, .plain)
            return r == n
        default:
            return false
        }
    }

    /// The type of the value in JSON Schema's instance data model, which
    /// has six types: integer is a value of the type keyword that matches
    /// a number with no fractional part, not a type a value has, so 1 and
    /// 1.0 are both a number (ADR-072).
    private func typeName(_ v: JSONValue) -> String {
        switch v {
        case .null: return "null"
        case .bool: return "boolean"
        case .string: return "string"
        case .array: return "array"
        case .object: return "object"
        case .number: return "number"
        }
    }

    private func formatHolds(_ f: String, _ s: String) -> Bool {
        switch f {
        case "date":
            return isDate(s)
        case "regex":
            return (try? NSRegularExpression(pattern: s)) != nil
        default:
            return true
        }
    }
}

/// A calendar date written YYYY-MM-DD.
func isDate(_ s: String) -> Bool {
    let parts = s.split(separator: "-", omittingEmptySubsequences: false)
    guard parts.count == 3, parts[0].count == 4, parts[1].count == 2, parts[2].count == 2,
          let y = Int(parts[0]), let m = Int(parts[1]), let d = Int(parts[2]), (1...12).contains(m), d >= 1 else { return false }
    let days = [31, (y % 4 == 0 && (y % 100 != 0 || y % 400 == 0)) ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]
    return d <= days[m - 1]
}

func decimal(_ any: Any) -> Decimal? {
    if let n = any as? NSNumber { return Decimal(string: n.stringValue) }
    return nil
}

func fromAny(_ any: Any) -> JSONValue {
    switch any {
    case is NSNull: return .null
    case let s as String: return .string(s)
    case let n as NSNumber:
        if CFGetTypeID(n) == CFBooleanGetTypeID() { return .bool(n.boolValue) }
        return .number(Decimal(string: n.stringValue) ?? 0, n.stringValue)
    case let a as [Any]: return .array(a.map(fromAny))
    case let o as [String: Any]: return .object(o.sorted { $0.key < $1.key }.map { ($0.key, fromAny($0.value)) })
    default: return .null
    }
}

func jsonEqual(_ a: JSONValue, _ b: JSONValue) -> Bool {
    switch (a, b) {
    case (.null, .null): return true
    case let (.bool(x), .bool(y)): return x == y
    case let (.number(x, _), .number(y, _)): return x == y
    case let (.string(x), .string(y)): return x == y
    case let (.array(x), .array(y)): return x.count == y.count && zip(x, y).allSatisfy { jsonEqual($0, $1) }
    case let (.object(x), .object(y)):
        let dx = Dictionary(x, uniquingKeysWith: { a, _ in a }), dy = Dictionary(y, uniquingKeysWith: { a, _ in a })
        return dx.count == dy.count && dx.allSatisfy { k, v in dy[k].map { jsonEqual(v, $0) } ?? false }
    default: return false
    }
}
