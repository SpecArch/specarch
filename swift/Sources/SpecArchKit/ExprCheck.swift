import Foundation

/// The type of a value, as CEL names it, plus SpecArch's decimal, date and
/// enum.
public enum ExprKind: String, Sendable {
    case int, uint, double, decimal, string, bool, bytes, date, timestamp, duration
    case time, enumValue = "enum", list, object, null, bad
}

/// The scale of a decimal whose scale is not fixed, the result of /.
public let unknownScale = -1

/// The type of a name or of an expression.
public struct ExprType: CustomStringConvertible, Sendable {
    public var kind: ExprKind
    public var scale = 0       // for decimal: digits after the point, or unknownScale
    public var bits = 64       // for int: 32 or 64, the declared width
    public var enumName = ""   // for enum
    public var values: [String] = []
    public var nullable = false

    public init(_ kind: ExprKind, scale: Int = 0, bits: Int = 64) {
        self.kind = kind
        self.scale = scale
        self.bits = bits
    }

    public var description: String {
        var s: String
        switch kind {
        case .decimal: s = scale == unknownScale ? "a decimal of no fixed scale" : "a decimal of scale \(scale)"
        case .enumValue: s = enumName.isEmpty ? "an enum value" : "a value of \(enumName)"
        case .int: s = "an int"
        case .uint: s = "a uint"
        case .null: s = "null"
        default: s = "a " + kind.rawValue
        }
        if nullable { s += " or null" }
        return s
    }
}

func numeric(_ k: ExprKind) -> Bool { [.int, .uint, .double, .decimal].contains(k) }
func orderable(_ k: ExprKind) -> Bool { numeric(k) || [.string, .date, .timestamp, .duration, .time].contains(k) }

public typealias ExprEnv = [String: ExprType]

private let bad = ExprType(.bad)

/// Type-checks a parsed expression and returns its type.
public func checkExpr(_ n: ExprNode, _ env: ExprEnv) -> (ExprType, [ExprError]) {
    let c = TypeChecker(env)
    let t = c.check(n)
    return (t, c.errs)
}

private func isComparison(_ op: ExprNode.Op) -> Bool {
    if case .binary(let s) = op { return ["<", "<=", ">", ">="].contains(s) }
    return false
}

/// Says how to make two types meet.
func conversionHint(_ l: ExprType, _ r: ExprType) -> String {
    switch (l.kind, r.kind) {
    case (.date, .timestamp), (.timestamp, .date): return "take the date of the timestamp with date(x)"
    case (.int, .decimal), (.decimal, .int): return "convert the int with decimal(x, scale)"
    case (.int, .double), (.double, .int): return "convert the int with double(x)"
    case (.int, .uint), (.uint, .int): return "convert one side with int(x) or uint(x)"
    case (.decimal, .double), (.double, .decimal):
        return "a double cannot become a decimal; keep the value a decimal from the start, or convert the decimal with double(x)"
    default: return "write a conversion so both sides have one type"
    }
}

func maxScale(_ a: Int, _ b: Int) -> Int {
    a == unknownScale || b == unknownScale ? unknownScale : max(a, b)
}

let digitsPattern = try! NSRegularExpression(pattern: "^-?[0-9]+$")
let decimalTextPattern = try! NSRegularExpression(pattern: "^-?[0-9]+(\\.[0-9]+)?$")

private func isRounding(_ n: ExprNode) -> Bool {
    n.op == .call && ["round", "floor", "ceil"].contains(n.text)
}

private func plural(_ n: Int, _ w: String) -> String { n == 1 ? w : w + "s" }

/// Whether a formula's type can be returned as the declared output, and why
/// not.
public func fits(_ got: ExprType, _ want: ExprType) -> (Bool, String) {
    if got.kind != want.kind || (want.kind == .enumValue && got.enumName != want.enumName && !got.enumName.isEmpty) {
        return (false, "the formula gives \(got), but the output is \(want); \(conversionHint(got, want))")
    }
    if got.nullable && !want.nullable {
        return (false, "the formula may give null, but the output does not allow null")
    }
    if got.kind == .decimal && want.scale != unknownScale && (got.scale == unknownScale || got.scale > want.scale) {
        return (false, "the formula gives \(got), but the output's scale is \(want.scale); say how to round with round(x, \(want.scale))")
    }
    return (true, "")
}

private final class TypeChecker {
    var env: ExprEnv
    var errs: [ExprError] = []

    init(_ env: ExprEnv) { self.env = env }

    @discardableResult
    func fail(_ n: ExprNode, _ kind: ExprError.Kind, _ message: String) -> ExprType {
        errs.append(ExprError(line: n.line, column: n.column, kind: kind, message: message))
        return bad
    }

    func names(_ near: String) -> String {
        let names = env.keys.sorted(by: byteLess)
        if names.isEmpty { return "there are none" }
        if let n = names.first(where: { $0.lowercased() == near.lowercased() }) { return "did you mean \(n)?" }
        return "use one of " + names.joined(separator: ", ")
    }

    func check(_ n: ExprNode) -> ExprType {
        switch n.op {
        case .int: return ExprType(.int)
        case .uint: return ExprType(.uint)
        case .double: return ExprType(.double)
        case .text: return ExprType(.string)
        case .bool: return ExprType(.bool)
        case .null: return ExprType(.null)
        case .name:
            guard let t = env[n.text] else {
                return fail(n, .name, "\(n.text) is not a name this expression can use; \(names(n.text))")
            }
            return t
        case .neg:
            let t = check(n.args[0])
            if t.kind == .bad { return t }
            if [.int, .double, .decimal].contains(t.kind) && !t.nullable { return t }
            return fail(n, .typeMismatch, "- needs an int, a double or a decimal, and this is \(t)")
        case .not:
            let t = check(n.args[0])
            if t.kind == .bad || (t.kind == .bool && !t.nullable) { return t }
            return fail(n, .typeMismatch, "! needs a bool, and this is \(t)")
        case .cond:
            return cond(n)
        case .call:
            return call(n)
        case .binary(let op):
            switch op {
            case "+", "-", "*", "/":
                return arithmetic(n, op)
            case "&&", "||":
                let l = check(n.args[0])
                let r: ExprType
                if let name = nullTest(n.args[0], notNull: op == "&&") {
                    r = withNonNull(name, n.args[1])
                } else {
                    r = check(n.args[1])
                }
                if l.kind == .bad || r.kind == .bad { return bad }
                for t in [l, r] where t.kind != .bool || t.nullable {
                    return fail(n, .typeMismatch, "\(op) joins two bools, and one side is \(t)")
                }
                return ExprType(.bool)
            case "==", "!=":
                return equality(n, op)
            default:
                let l = check(n.args[0]), r = check(n.args[1])
                if l.kind == .bad || r.kind == .bad { return bad }
                if l.kind == .bool && isComparison(n.args[0].op) {
                    return fail(n, .typeMismatch, "comparisons do not chain; write a < b && b < c")
                }
                if l.nullable || r.nullable {
                    return fail(n, .typeMismatch, "\(op) cannot order a value that may be null; check it with == null first")
                }
                if !orderable(l.kind) || l.kind != r.kind {
                    return fail(n, .typeMismatch, "\(op) compares two values of one type, and this is \(l) \(op) \(r); \(conversionHint(l, r))")
                }
                return ExprType(.bool)
            }
        }
    }

    func arithmetic(_ n: ExprNode, _ op: String) -> ExprType {
        let l = check(n.args[0]), r = check(n.args[1])
        if l.kind == .bad || r.kind == .bad { return bad }
        if l.nullable || r.nullable {
            return fail(n, .typeMismatch, "\(op) cannot take a value that may be null; check it with == null first")
        }
        if (op == "+" || op == "-") && (l.kind == .date || l.kind == .duration) {
            return dateArithmetic(n, op, l, r)
        }
        if !numeric(l.kind) || l.kind != r.kind {
            if numeric(l.kind) && numeric(r.kind) {
                return fail(n, .typeMismatch, "\(op) needs two numbers of one type, and this is \(l) \(op) \(r); \(conversionHint(l, r))")
            }
            return fail(n, .typeMismatch, "\(op) works on numbers, and this is \(l) \(op) \(r)")
        }
        if l.kind == .uint && op == "-" { return ExprType(.uint) }
        if l.kind != .decimal { return ExprType(l.kind, bits: 64) }
        var scale = unknownScale
        if l.scale != unknownScale && r.scale != unknownScale {
            switch op {
            case "+", "-": scale = max(l.scale, r.scale)
            case "*": scale = l.scale + r.scale
            default: break
            }
        }
        return ExprType(.decimal, scale: scale)
    }

    /// A date moved by a whole number of days: a date plus or minus
    /// duration("P<n>D"), written in place.
    func dateArithmetic(_ n: ExprNode, _ op: String, _ l: ExprType, _ r: ExprType) -> ExprType {
        if l.kind == .duration && r.kind == .date && op == "+" {
            return fail(n, .typeMismatch, "write the date first: date + duration")
        }
        if l.kind == .duration {
            return fail(n, .typeMismatch, "\(op) works on numbers, or moves a date by a duration, and this is \(l) \(op) \(r)")
        }
        if r.kind == .int || r.kind == .uint {
            var days = "14"
            if n.args[1].op == .int || n.args[1].op == .uint { days = formatDecimal(n.args[1].intValue) }
            return fail(n, .typeMismatch, "\(op) cannot move a date by a number, since a number names no unit; write the days as a duration, such as duration(\"P\(days)D\")")
        }
        if r.kind != .duration {
            return fail(n, .typeMismatch, "\(op) moves a date by a duration, and this is \(l) \(op) \(r)")
        }
        let a = n.args[1]
        guard a.op == .call, a.text == "duration", a.args[0].op == .text else {
            return fail(a, .typeMismatch, "a date moves by whole days written in place, such as duration(\"P14D\")")
        }
        guard let d = parseDuration(a.args[0].text), d.isWholeDays else {
            return fail(a, .typeMismatch, "a date moves by whole days, and \(quote(a.args[0].text)) is not; write days only, such as \"P14D\"")
        }
        return ExprType(.date)
    }

    func equality(_ n: ExprNode, _ op: String) -> ExprType {
        let l = check(n.args[0]), r = check(n.args[1])
        if l.kind == .bad || r.kind == .bad { return bad }
        if l.kind == .null || r.kind == .null {
            let other = l.kind == .null ? r : l
            if other.kind != .null && !other.nullable {
                return fail(n, .typeMismatch, "this side is never null, so comparing it with null is always \(op == "!=" ? "true" : "false"); remove the comparison or allow null on the field")
            }
            return ExprType(.bool)
        }
        if l.kind == .enumValue || r.kind == .enumValue {
            var e = l, other = r, otherNode = n.args[1]
            if l.kind != .enumValue { e = r; other = l; otherNode = n.args[0] }
            if other.kind == .enumValue && other.enumName == e.enumName { return ExprType(.bool) }
            if other.kind == .string && otherNode.op == .text {
                if e.values.contains(otherNode.text) { return ExprType(.bool) }
                return fail(otherNode, .typeMismatch, "\(quote(otherNode.text)) is not \(e); use one of \(e.values.joined(separator: ", "))")
            }
            return fail(n, .typeMismatch, "\(op) compares \(e) with one of its values in quotes, and the other side is \(other)")
        }
        if l.kind != r.kind || l.kind == .list || l.kind == .object {
            return fail(n, .typeMismatch, "\(op) compares two values of one type, and this is \(l) \(op) \(r); \(conversionHint(l, r))")
        }
        return ExprType(.bool)
    }

    func cond(_ n: ExprNode) -> ExprType {
        let c = check(n.args[0])
        var a: ExprType, b: ExprType
        if let name = nullTest(n.args[0], notNull: true) {
            a = withNonNull(name, n.args[1]); b = check(n.args[2])
        } else if let name = nullTest(n.args[0], notNull: false) {
            a = check(n.args[1]); b = withNonNull(name, n.args[2])
        } else {
            a = check(n.args[1]); b = check(n.args[2])
        }
        if c.kind != .bad && (c.kind != .bool || c.nullable) {
            return fail(n.args[0], .typeMismatch, "the condition before ? must be a bool, and this is \(c)")
        }
        if c.kind == .bad || a.kind == .bad || b.kind == .bad { return bad }
        if a.kind == .null { b.nullable = true; return b }
        if b.kind == .null { a.nullable = true; return a }
        if a.kind != b.kind || (a.kind == .enumValue && a.enumName != b.enumName) {
            return fail(n, .typeMismatch, "both sides of : must have one type, and this is \(a) : \(b); \(conversionHint(a, b))")
        }
        if a.kind == .decimal { a.scale = maxScale(a.scale, b.scale) }
        a.nullable = a.nullable || b.nullable
        return a
    }

    /// The name in "name != null" (notNull) or "name == null", or nil.
    func nullTest(_ n: ExprNode, notNull: Bool) -> String? {
        guard n.op == .binary(notNull ? "!=" : "=="), n.args.count == 2 else { return nil }
        let a = n.args[0], b = n.args[1]
        if a.op == .name && b.op == .null { return a.text }
        if b.op == .name && a.op == .null { return b.text }
        return nil
    }

    /// Checks n with name known not to be null.
    func withNonNull(_ name: String, _ n: ExprNode) -> ExprType {
        guard var t = env[name], t.nullable else { return check(n) }
        let saved = env
        t.nullable = false
        env[name] = t
        defer { env = saved }
        return check(n)
    }

    func call(_ n: ExprNode) -> ExprType {
        var args: [ExprType] = []
        for a in n.args {
            let t = check(a)
            if t.kind == .bad { return bad }
            args.append(t)
        }
        func count(_ want: Int) -> Bool {
            if args.count != want {
                fail(n, .typeMismatch, "\(n.text) takes \(want) \(plural(want, "argument")), and here it has \(args.count)")
                return false
            }
            if want > 0 && args[0].nullable {
                fail(n, .typeMismatch, "\(n.text) cannot take a value that may be null; check it with == null first")
                return false
            }
            return true
        }
        let a0 = args.first ?? ExprType(.bad)
        switch n.text {
        case "int":
            guard count(1) else { return bad }
            switch a0.kind {
            case .int, .uint: return ExprType(.int)
            case .string:
                if n.args[0].op == .text && !matches(digitsPattern, n.args[0].text) {
                    return fail(n.args[0], .typeMismatch, "\(quote(n.args[0].text)) is not a whole number")
                }
                return ExprType(.int)
            case .decimal, .double:
                if isRounding(n.args[0]) { return ExprType(.int) }
                return fail(n, .typeMismatch, "int(x) of \(a0) would drop the fraction without saying how; write int(round(x, 0)), int(floor(x)) or int(ceil(x))")
            default:
                return fail(n, .typeMismatch, "int(x) takes a uint, a string of digits or a rounded number, and this is \(a0)")
            }
        case "uint":
            guard count(1) else { return bad }
            if [.int, .uint, .string].contains(a0.kind) { return ExprType(.uint) }
            return fail(n, .typeMismatch, "uint(x) takes an int or a string of digits, and this is \(a0)")
        case "double":
            guard count(1) else { return bad }
            if numeric(a0.kind) || a0.kind == .string { return ExprType(.double) }
            return fail(n, .typeMismatch, "double(x) takes a number or a string, and this is \(a0)")
        case "decimal":
            return decimal(n, args)
        case "string":
            guard count(1) else { return bad }
            if a0.kind == .list || a0.kind == .object {
                return fail(n, .typeMismatch, "string(x) takes a single value, and this is \(a0)")
            }
            return ExprType(.string)
        case "date":
            guard count(1) else { return bad }
            if a0.kind == .timestamp || a0.kind == .date { return ExprType(.date) }
            if n.args[0].op == .text {
                if !isDate(n.args[0].text) {
                    return fail(n.args[0], .typeMismatch, "\(quote(n.args[0].text)) is not a date; write it as YYYY-MM-DD")
                }
                return ExprType(.date)
            }
            return fail(n, .typeMismatch, "date(x) takes a timestamp, a date or a quoted date, and this is \(a0)")
        case "timestamp":
            guard count(1) else { return bad }
            if n.args[0].op == .text {
                if parseTimestamp(n.args[0].text) == nil {
                    return fail(n.args[0], .typeMismatch, "\(quote(n.args[0].text)) is not a timestamp; write it as RFC 3339, such as 2026-10-07T09:30:00Z")
                }
                return ExprType(.timestamp)
            }
            if a0.kind == .timestamp || a0.kind == .string { return ExprType(.timestamp) }
            return fail(n, .typeMismatch, "timestamp(x) takes a quoted RFC 3339 instant, and this is \(a0)")
        case "duration":
            guard count(1) else { return bad }
            if n.args[0].op == .text {
                if parseDuration(n.args[0].text) == nil {
                    return fail(n.args[0], .typeMismatch, "\(quote(n.args[0].text)) is not a duration; write it as ISO 8601 days, hours, minutes and seconds, such as \"P14D\" or \"PT2H\"")
                }
                return ExprType(.duration)
            }
            return fail(n, .typeMismatch, "duration(x) takes a quoted ISO 8601 duration, such as duration(\"P14D\"), and this is \(a0)")
        case "size":
            guard count(1) else { return bad }
            if [.string, .bytes, .list].contains(a0.kind) { return ExprType(.int) }
            return fail(n, .typeMismatch, "size(x) takes a string, bytes or a list, and this is \(a0)")
        case "round":
            guard count(2) else { return bad }
            if args[1].kind != .int || args[1].nullable {
                return fail(n.args[1], .typeMismatch, "the places of round(x, places) must be an int, and this is \(args[1])")
            }
            switch a0.kind {
            case .decimal:
                var scale = unknownScale
                if n.args[1].op == .int {
                    scale = NSDecimalNumber(decimal: n.args[1].intValue).intValue
                    if scale < 0 { return fail(n.args[1], .typeMismatch, "round cannot take a negative number of places") }
                }
                return ExprType(.decimal, scale: scale)
            case .double:
                return ExprType(.double)
            default:
                return fail(n, .typeMismatch, "round(x, places) takes a decimal or a double, and this is \(a0)")
            }
        case "floor", "ceil":
            guard count(1) else { return bad }
            switch a0.kind {
            case .decimal: return ExprType(.decimal, scale: 0)
            case .double: return ExprType(.double)
            default: return fail(n, .typeMismatch, "\(n.text)(x) takes a decimal or a double, and this is \(a0)")
            }
        case "min", "max":
            if args.count < 2 { return fail(n, .typeMismatch, "\(n.text) takes two or more values") }
            var out = args[0]
            for a in args {
                if !orderable(a.kind) || a.kind != args[0].kind || a.nullable {
                    return fail(n, .typeMismatch, "\(n.text) takes values of one orderable type, never null; here it gets \(args[0]) and \(a); \(conversionHint(args[0], a))")
                }
                if a.kind == .decimal { out.scale = maxScale(out.scale, a.scale) }
            }
            return out
        default:
            return fail(n, .name, "\(n.text) is not a function of SpecArch expressions; the functions are \(exprFunctionList)")
        }
    }

    func decimal(_ n: ExprNode, _ args: [ExprType]) -> ExprType {
        guard args.count == 2 else {
            return fail(n, .typeMismatch, "decimal takes a value and a scale, decimal(x, scale), and here it has \(args.count) \(plural(args.count, "argument"))")
        }
        guard n.args[1].op == .int, n.args[1].intValue >= 0 else {
            return fail(n.args[1], .typeMismatch, "the scale of decimal(x, scale) must be a whole number written in place, such as 2")
        }
        let scale = NSDecimalNumber(decimal: n.args[1].intValue).intValue
        let a = args[0]
        if a.nullable {
            return fail(n, .typeMismatch, "decimal cannot take a value that may be null; check it with == null first")
        }
        let out = ExprType(.decimal, scale: scale)
        switch a.kind {
        case .int, .uint:
            return out
        case .string:
            let lit = n.args[0]
            if lit.op == .text {
                if !matches(decimalTextPattern, lit.text) {
                    return fail(lit, .typeMismatch, "\(quote(lit.text)) is not a decimal number")
                }
                if let dot = lit.text.firstIndex(of: ".") {
                    let frac = lit.text[lit.text.index(after: dot)...].count
                    if frac > scale {
                        return fail(lit, .typeMismatch, "\(quote(lit.text)) has \(frac) decimal places, more than the scale \(scale); round it or raise the scale")
                    }
                }
            }
            return out
        case .decimal:
            if a.scale != unknownScale && a.scale <= scale { return out }
            return fail(n, .typeMismatch, "decimal(x, \(scale)) of \(a) would drop digits without saying how; round it first: decimal(round(x, \(scale)), \(scale))")
        case .double:
            return fail(n, .typeMismatch, "a double cannot become a decimal, because the digits it lost cannot be brought back; keep the value a decimal from the start")
        default:
            return fail(n, .typeMismatch, "decimal(x, scale) takes an int, a uint, a quoted number or a decimal, and this is \(a)")
        }
    }
}
