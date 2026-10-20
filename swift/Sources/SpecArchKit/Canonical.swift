import Foundation

/// A JSON value for the canonical writer: what a model is built of.
indirect enum Canon {
    case null
    case bool(Bool)
    case number(Double)
    case string(String)
    case array([Canon])
    case object([String: Canon])
}

extension Canon {
    /// The plain value of a document, its numbers read as doubles.
    init(_ v: JSONValue) {
        switch v {
        case .null: self = .null
        case .bool(let b): self = .bool(b)
        case .number(let d, let text): self = .number(Double(text) ?? NSDecimalNumber(decimal: d).doubleValue)
        case .string(let s): self = .string(s)
        case .array(let a): self = .array(a.map(Canon.init))
        case .object(let pairs):
            var m: [String: Canon] = [:]
            for (k, v) in pairs where m[k] == nil { m[k] = Canon(v) }
            self = .object(m)
        }
    }
}

/// Writes a value in the canonical form of RFC 8785, the JSON
/// Canonicalization Scheme: the members of every object sorted by their
/// names as UTF-16 code units, no whitespace, every string in one spelling
/// and every number as ECMAScript prints a double. The Go build writes the
/// same bytes for the same value.
func canonicalJSON(_ v: Canon) -> String {
    var out = ""
    writeCanon(v, &out)
    return out
}

private func writeCanon(_ v: Canon, _ out: inout String) {
    switch v {
    case .null: out += "null"
    case .bool(let b): out += b ? "true" : "false"
    case .number(let d): out += ecmaNumber(d)
    case .string(let s): writeCanonString(s, &out)
    case .array(let a):
        out += "["
        for (i, e) in a.enumerated() {
            if i > 0 { out += "," }
            writeCanon(e, &out)
        }
        out += "]"
    case .object(let m):
        out += "{"
        for (i, k) in m.keys.sorted(by: utf16Less).enumerated() {
            if i > 0 { out += "," }
            writeCanonString(k, &out)
            out += ":"
            writeCanon(m[k]!, &out)
        }
        out += "}"
    }
}

/// Orders two strings by their UTF-16 code units (RFC 8785, 3.2.3).
func utf16Less(_ a: String, _ b: String) -> Bool {
    Array(a.utf16).lexicographicallyPrecedes(Array(b.utf16))
}

/// Escapes only what JSON requires (RFC 8785, 3.2.2.2).
private func writeCanonString(_ s: String, _ out: inout String) {
    out += "\""
    for c in s.unicodeScalars {
        switch c {
        case "\"": out += "\\\""
        case "\\": out += "\\\\"
        case "\u{8}": out += "\\b"
        case "\u{c}": out += "\\f"
        case "\n": out += "\\n"
        case "\r": out += "\\r"
        case "\t": out += "\\t"
        default:
            if c.value < 0x20 {
                let hex = String(c.value, radix: 16)
                out += "\\u" + String(repeating: "0", count: 4 - hex.count) + hex
            } else {
                out.unicodeScalars.append(c)
            }
        }
    }
    out += "\""
}

/// A double as ECMAScript's Number.prototype.toString prints it (RFC 8785,
/// 3.2.2.3), from the shortest digits that read back as it.
func ecmaNumber(_ d: Double) -> String {
    if d == 0 { return "0" }
    let sign = d < 0 ? "-" : ""
    // Swift's description is the shortest digits that read back as the
    // double, in either plain or exponent form.
    let text = "\(abs(d))".lowercased()
    var mantissa = Substring(text), exp = 0
    if let e = text.firstIndex(of: "e") {
        mantissa = text[..<e]
        exp = Int(text[text.index(after: e)...].replacingOccurrences(of: "+", with: "")) ?? 0
    }
    let parts = mantissa.split(separator: ".", omittingEmptySubsequences: false)
    let intPart = String(parts[0]), frac = parts.count > 1 ? String(parts[1]) : ""
    var digits = Array(intPart + frac)
    var n = intPart.count + exp
    while digits.first == "0" { digits.removeFirst(); n -= 1 }
    while digits.last == "0" { digits.removeLast() }
    return sign + ecmaFormat(String(digits), n)
}

/// Lays out the digits of a positive double, k of them with no leading or
/// trailing zero, whose value is 0.digits × 10^n, by the rules of
/// ECMAScript's Number::toString.
func ecmaFormat(_ digits: String, _ n: Int) -> String {
    let k = digits.count
    if k <= n && n <= 21 { return digits + String(repeating: "0", count: n - k) }
    if 0 < n && n <= 21 { return String(digits.prefix(n)) + "." + String(digits.dropFirst(n)) }
    if -6 < n && n <= 0 { return "0." + String(repeating: "0", count: -n) + digits }
    let e = n - 1
    let exp = (e < 0 ? "-" : "+") + String(abs(e))
    if k == 1 { return digits + "e" + exp }
    return String(digits.prefix(1)) + "." + String(digits.dropFirst()) + "e" + exp
}
