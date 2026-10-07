import Foundation

/// A value during evaluation. Int, uint and decimal values are exact
/// decimals of up to 38 digits; a double is a 64-bit float, as CEL has it.
public struct ExprValue: CustomStringConvertible {
    public var kind: ExprKind
    public var num: Decimal = 0       // int, uint, decimal
    public var double: Double = 0
    public var text = ""              // string, enum, bytes
    public var bool = false
    public var seconds: Decimal = 0   // date, timestamp, time: seconds since 1970 or since midnight; duration
    public var items = 0              // list: the length

    public init(_ kind: ExprKind) { self.kind = kind }

    /// Shows a value the way a worked example writes it.
    public var description: String {
        switch kind {
        case .int, .uint: return formatDecimal(num)
        case .decimal: return quote(formatDecimal(num))
        case .double: return formatDouble(double)
        case .string, .enumValue: return quote(text)
        case .bool: return bool ? "true" : "false"
        case .date: return formatDate(seconds)
        case .timestamp: return formatTimestamp(seconds)
        case .time: return formatTimeOfDay(seconds)
        case .duration: return "\(formatDecimal(seconds))s"
        case .null: return "null"
        default: return kind.rawValue
        }
    }
}

/// Writes an exact number with as many places as it needs.
public func formatDecimal(_ d: Decimal) -> String {
    var d = d
    var r = Decimal()
    NSDecimalRound(&r, &d, 38, .plain)
    return NSDecimalNumber(decimal: r).description(withLocale: Locale(identifier: "en_US_POSIX"))
}

/// Writes a number with exactly `places` digits after the point, rounding
/// half away from zero.
public func fixedDecimal(_ d: Decimal, _ places: Int) -> String {
    let r = roundDecimal(d, places)
    var s = formatDecimal(r)
    if places == 0 { return s }
    if !s.contains(".") { s += "." }
    let have = s.count - 1 - s.distance(from: s.startIndex, to: s.firstIndex(of: ".")!)
    return s + String(repeating: "0", count: max(0, places - have))
}

func formatDouble(_ d: Double) -> String {
    if d == d.rounded(), abs(d) < 1e21 { return String(Int64(d)) }
    return "\(d)"
}

public struct EvalError: Error, CustomStringConvertible {
    public let line: Int
    public let column: Int
    public let message: String
    public var description: String { message }
}

private func evalErr(_ n: ExprNode, _ message: String) -> EvalError {
    EvalError(line: n.line, column: n.column, message: message)
}

let minInt64 = Decimal(string: "-9223372036854775808")!
let maxInt64 = Decimal(string: "9223372036854775807")!
let maxUint64 = Decimal(string: "18446744073709551615")!

/// Whether d is a whole number inside a signed width.
public func fitsInt(_ d: Decimal, _ bits: Int) -> Bool {
    guard isWhole(d) else { return false }
    if bits == 32 { return d >= Decimal(Int32.min) && d <= Decimal(Int32.max) }
    return d >= minInt64 && d <= maxInt64
}

func isWhole(_ d: Decimal) -> Bool {
    var x = d, r = Decimal()
    NSDecimalRound(&r, &x, 0, .down)
    return r == d
}

private func checkInt(_ n: ExprNode, _ v: ExprValue) throws -> ExprValue {
    switch v.kind {
    case .int where !fitsInt(v.num, 64):
        throw evalErr(n, "the result \(formatDecimal(v.num)) does not fit in an int (64 bits)")
    case .uint where v.num < 0 || v.num > maxUint64:
        throw evalErr(n, "the result \(formatDecimal(v.num)) does not fit in a uint (0 to 2^64 - 1)")
    default:
        return v
    }
}

/// Rounds half away from zero to the given number of decimal places.
public func roundDecimal(_ d: Decimal, _ places: Int) -> Decimal {
    var x = d, r = Decimal()
    NSDecimalRound(&r, &x, places, .plain)
    return r
}

/// Toward zero.
private func truncate(_ d: Decimal) -> Decimal {
    var x = d < 0 ? -d : d, r = Decimal()
    NSDecimalRound(&r, &x, 0, .down)
    return d < 0 ? -r : r
}

/// Computes a checked expression with the given values.
public func evalExpr(_ n: ExprNode, _ vals: [String: ExprValue]) throws -> ExprValue {
    switch n.op {
    case .int: var v = ExprValue(.int); v.num = n.intValue; return v
    case .uint: var v = ExprValue(.uint); v.num = n.intValue; return v
    case .double: var v = ExprValue(.double); v.double = n.doubleValue; return v
    case .text: var v = ExprValue(.string); v.text = n.text; return v
    case .bool: var v = ExprValue(.bool); v.bool = n.boolValue; return v
    case .null: return ExprValue(.null)
    case .name:
        guard let v = vals[n.text] else { throw evalErr(n, "\(n.text) has no value") }
        return v
    case .neg:
        var v = try evalExpr(n.args[0], vals)
        if v.kind == .double { v.double = -v.double; return v }
        v.num = -v.num
        return try checkInt(n, v)
    case .not:
        var v = try evalExpr(n.args[0], vals)
        v.bool = !v.bool
        return v
    case .cond:
        let c = try evalExpr(n.args[0], vals)
        return try evalExpr(c.bool ? n.args[1] : n.args[2], vals)
    case .call:
        return try call(n, vals)
    case .binary(let op):
        if op == "&&" || op == "||" {
            let l = try evalExpr(n.args[0], vals)
            if (op == "&&" && !l.bool) || (op == "||" && l.bool) { return l }
            return try evalExpr(n.args[1], vals)
        }
        let l = try evalExpr(n.args[0], vals), r = try evalExpr(n.args[1], vals)
        switch op {
        case "+", "-", "*", "/":
            return try arithmetic(n, op, l, r)
        case "==", "!=":
            var v = ExprValue(.bool)
            v.bool = exprEqual(l, r) == (op == "==")
            return v
        default:
            guard let cmp = compare(l, r) else { throw evalErr(n, "\(op) cannot compare \(l) and \(r)") }
            var v = ExprValue(.bool)
            switch op {
            case "<": v.bool = cmp < 0
            case "<=": v.bool = cmp <= 0
            case ">": v.bool = cmp > 0
            default: v.bool = cmp >= 0
            }
            return v
        }
    }
}

private func arithmetic(_ n: ExprNode, _ op: String, _ l: ExprValue, _ r: ExprValue) throws -> ExprValue {
    var v = ExprValue(l.kind)
    if l.kind == .double {
        switch op {
        case "+": v.double = l.double + r.double
        case "-": v.double = l.double - r.double
        case "*": v.double = l.double * r.double
        default: v.double = l.double / r.double
        }
        return v
    }
    switch op {
    case "+": v.num = l.num + r.num
    case "-": v.num = l.num - r.num
    case "*": v.num = l.num * r.num
    default:
        if r.num == 0 { throw evalErr(n, "division by zero") }
        v.num = l.num / r.num
        // CEL divides integers toward zero.
        if l.kind != .decimal { v.num = truncate(v.num) }
    }
    return try checkInt(n, v)
}

/// Compares two values of one type; decimals by value, so "3.50" equals "3.5".
public func exprEqual(_ a: ExprValue, _ b: ExprValue) -> Bool {
    if a.kind == .null || b.kind == .null { return a.kind == b.kind }
    if let c = compare(a, b) { return c == 0 }
    if [.string, .enumValue].contains(a.kind) && [.string, .enumValue].contains(b.kind) { return a.text == b.text }
    if a.kind == .bool && b.kind == .bool { return a.bool == b.bool }
    if a.kind == .bytes && b.kind == .bytes { return a.text == b.text }
    return false
}

private func compare(_ a: ExprValue, _ b: ExprValue) -> Int? {
    guard a.kind == b.kind else { return nil }
    func cmp<T: Comparable>(_ x: T, _ y: T) -> Int { x < y ? -1 : (x > y ? 1 : 0) }
    switch a.kind {
    case .int, .uint, .decimal: return cmp(a.num, b.num)
    case .double: return cmp(a.double, b.double)
    case .string: return byteLess(a.text, b.text) ? -1 : (a.text == b.text ? 0 : 1)
    case .date, .timestamp, .time, .duration: return cmp(a.seconds, b.seconds)
    default: return nil
    }
}

private func call(_ n: ExprNode, _ vals: [String: ExprValue]) throws -> ExprValue {
    let args = try n.args.map { try evalExpr($0, vals) }
    let a = args.first ?? ExprValue(.null)
    switch n.text {
    case "int", "uint":
        var v = ExprValue(n.text == "uint" ? .uint : .int)
        switch a.kind {
        case .string:
            guard matches(digitsPattern, a.text), let d = Decimal(string: a.text) else { throw evalErr(n, "\(quote(a.text)) is not a whole number") }
            v.num = d
        case .double:
            guard a.double.isFinite else { throw evalErr(n, "\(a) cannot become an int") }
            v.num = Decimal(a.double)
        default:
            guard isWhole(a.num) else { throw evalErr(n, "\(a) is not a whole number") }
            v.num = a.num
        }
        return try checkInt(n, v)
    case "double":
        var v = ExprValue(.double)
        if a.kind == .string {
            guard let d = Double(a.text) else { throw evalErr(n, "\(quote(a.text)) is not a number") }
            v.double = d
        } else {
            v.double = NSDecimalNumber(decimal: a.num).doubleValue
        }
        return v
    case "decimal":
        let scale = NSDecimalNumber(decimal: n.args[1].intValue).intValue
        var v = ExprValue(.decimal)
        if a.kind == .string {
            guard matches(decimalTextPattern, a.text), let d = Decimal(string: a.text) else { throw evalErr(n, "\(quote(a.text)) is not a decimal number") }
            v.num = d
        } else {
            v.num = a.num
        }
        if roundDecimal(v.num, scale) != v.num {
            throw evalErr(n, "\(formatDecimal(v.num)) has more than \(scale) decimal places; round it first")
        }
        return v
    case "string":
        var v = ExprValue(.string)
        switch a.kind {
        case .string, .enumValue, .bytes: v.text = a.text
        case .decimal: v.text = formatDecimal(a.num)
        default: v.text = a.description
        }
        return v
    case "date":
        var v = ExprValue(.date)
        switch a.kind {
        case .date: return a
        case .timestamp:
            v.seconds = Decimal(Int64((NSDecimalNumber(decimal: a.seconds).doubleValue / 86400).rounded(.down))) * 86400
        default:
            guard let s = parseDate(a.text) else { throw evalErr(n, "\(quote(a.text)) is not a date") }
            v.seconds = s
        }
        return v
    case "timestamp":
        if a.kind == .timestamp { return a }
        guard let s = parseTimestamp(a.text) else { throw evalErr(n, "\(quote(a.text)) is not a timestamp") }
        var v = ExprValue(.timestamp)
        v.seconds = s
        return v
    case "size":
        var v = ExprValue(.int)
        switch a.kind {
        case .string: v.num = Decimal(a.text.unicodeScalars.count)
        case .bytes: v.num = Decimal(a.text.utf8.count)
        default: v.num = Decimal(a.items)
        }
        return v
    case "round":
        let places = NSDecimalNumber(decimal: args[1].num).intValue
        if args[1].num < 0 || places > 30 { throw evalErr(n, "round needs from 0 to 30 places, and got \(args[1])") }
        var v = a
        if a.kind == .double {
            let scale = pow(10, Double(places))
            v.double = (a.double * scale).rounded(.toNearestOrAwayFromZero) / scale
        } else {
            v.num = roundDecimal(a.num, places)
        }
        return v
    case "floor", "ceil":
        var v = a
        if a.kind == .double {
            v.double = n.text == "floor" ? a.double.rounded(.down) : a.double.rounded(.up)
        } else {
            var x = a.num, r = Decimal()
            NSDecimalRound(&r, &x, 0, n.text == "floor" ? .down : .up)
            v.num = r
        }
        return v
    case "min", "max":
        var best = args[0]
        for v in args.dropFirst() {
            guard let c = compare(v, best) else { throw evalErr(n, "\(n.text) cannot compare \(v) and \(best)") }
            if (n.text == "min" && c < 0) || (n.text == "max" && c > 0) { best = v }
        }
        return best
    default:
        throw evalErr(n, "\(n.text) cannot take \(a)")
    }
}

// MARK: - Calendar values, as seconds

/// Days since 1970-01-01 for a proleptic Gregorian date.
private func daysFromCivil(_ y0: Int, _ m: Int, _ d: Int) -> Int {
    let y = m <= 2 ? y0 - 1 : y0
    let era = (y >= 0 ? y : y - 399) / 400
    let yoe = y - era * 400
    let doy = (153 * (m + (m > 2 ? -3 : 9)) + 2) / 5 + d - 1
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy
    return era * 146097 + doe - 719468
}

private func civilFromDays(_ z0: Int) -> (Int, Int, Int) {
    let z = z0 + 719468
    let era = (z >= 0 ? z : z - 146096) / 146097
    let doe = z - era * 146097
    let yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365
    let y = yoe + era * 400
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100)
    let mp = (5 * doy + 2) / 153
    let d = doy - (153 * mp + 2) / 5 + 1
    let m = mp + (mp < 10 ? 3 : -9)
    return (m <= 2 ? y + 1 : y, m, d)
}

func parseDate(_ s: String) -> Decimal? {
    guard isDate(s) else { return nil }
    let p = s.split(separator: "-").map { Int($0)! }
    return Decimal(daysFromCivil(p[0], p[1], p[2])) * 86400
}

private let timestampPattern = try! NSRegularExpression(pattern: "^([0-9]{4})-([0-9]{2})-([0-9]{2})[Tt]([0-9]{2}):([0-9]{2}):([0-9]{2})(\\.[0-9]+)?([Zz]|([-+])([0-9]{2}):([0-9]{2}))$")

/// Seconds since 1970 for an RFC 3339 instant.
func parseTimestamp(_ s: String) -> Decimal? {
    guard let m = timestampPattern.firstMatch(in: s, range: NSRange(s.startIndex..., in: s)) else { return nil }
    func group(_ i: Int) -> String? {
        guard let r = Range(m.range(at: i), in: s) else { return nil }
        return String(s[r])
    }
    let y = Int(group(1)!)!, mo = Int(group(2)!)!, d = Int(group(3)!)!
    let h = Int(group(4)!)!, mi = Int(group(5)!)!, se = Int(group(6)!)!
    guard isDate(String(format: "%04d-%02d-%02d", y, mo, d)), h < 24, mi < 60, se < 61 else { return nil }
    var total = Decimal(daysFromCivil(y, mo, d)) * 86400 + Decimal(h * 3600 + mi * 60 + se)
    if let frac = group(7), let f = Decimal(string: "0" + frac) { total += f }
    if let sign = group(9) {
        let offset = Decimal(Int(group(10)!)! * 3600 + Int(group(11)!)! * 60)
        total += sign == "-" ? offset : -offset
    }
    return total
}

func parseTimeOfDay(_ s: String) -> Decimal? {
    let p = s.split(separator: ":")
    guard p.count == 3, let h = Int(p[0]), let m = Int(p[1]), let sec = Int(p[2]), h < 24, m < 60, sec < 60 else { return nil }
    return Decimal(h * 3600 + m * 60 + sec)
}

private func formatDate(_ seconds: Decimal) -> String {
    let days = Int(NSDecimalNumber(decimal: seconds).doubleValue / 86400)
    let (y, m, d) = civilFromDays(days)
    return String(format: "%04d-%02d-%02d", y, m, d)
}

private func formatTimestamp(_ seconds: Decimal) -> String {
    let whole = truncate(seconds)
    let secs = NSDecimalNumber(decimal: whole).intValue
    let days = Int((Double(secs) / 86400).rounded(.down))
    let rest = secs - days * 86400
    let (y, m, d) = civilFromDays(days)
    var s = String(format: "%04d-%02d-%02dT%02d:%02d:%02d", y, m, d, rest / 3600, rest / 60 % 60, rest % 60)
    let frac = seconds - whole
    if frac != 0 { s += String(formatDecimal(frac).dropFirst()) }
    return s + "Z"
}

private func formatTimeOfDay(_ seconds: Decimal) -> String {
    let s = NSDecimalNumber(decimal: seconds).intValue
    return String(format: "%02d:%02d:%02d", s / 3600, s / 60 % 60, s % 60)
}
