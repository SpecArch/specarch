import Foundation

/// Writes a parsed expression in the subset's own form: the operators with
/// one space either side, a call as name(arguments, ...), parentheses only
/// where CEL's grammar needs them, a double literal with its point. The Go
/// build writes the same text (ADR-195).
public func exprText(_ n: ExprNode) -> String {
    var out = ""
    writeExpr(&out, n)
    return out
}

/// CEL's grammar's level of a node, higher binding tighter.
private func precedence(_ n: ExprNode) -> Int {
    switch n.op {
    case .cond: return 0
    case .binary(let op):
        switch op {
        case "||": return 1
        case "&&": return 2
        case "==", "!=", "<", "<=", ">", ">=": return 3
        case "+", "-": return 4
        default: return 5
        }
    case .neg, .not: return 6
    default: return 7
    }
}

/// Writes n, in parentheses when its level is below min.
private func writeAbove(_ out: inout String, _ n: ExprNode, _ min: Int) {
    if precedence(n) < min {
        out += "("
        writeExpr(&out, n)
        out += ")"
        return
    }
    writeExpr(&out, n)
}

private func writeExpr(_ out: inout String, _ n: ExprNode) {
    switch n.op {
    case .int: out += formatDecimal(n.intValue)
    case .uint: out += formatDecimal(n.intValue) + "u"
    case .double:
        let s = formatTracedDouble(n.doubleValue)
        out += s.contains(".") || s.contains("e") ? s : s + ".0"
    case .text: out += quote(n.text)
    case .bool: out += n.boolValue ? "true" : "false"
    case .null: out += "null"
    case .name: out += n.text
    case .neg, .not:
        out += n.op == .neg ? "-" : "!"
        writeAbove(&out, n.args[0], 6)
    case .cond:
        writeAbove(&out, n.args[0], 1)
        out += " ? "
        writeAbove(&out, n.args[1], 1)
        out += " : "
        writeExpr(&out, n.args[2])
    case .call:
        out += n.text + "("
        for (i, a) in n.args.enumerated() {
            if i > 0 { out += ", " }
            writeExpr(&out, a)
        }
        out += ")"
    case .binary(let op):
        let p = precedence(n)
        writeAbove(&out, n.args[0], p)
        out += " \(op) "
        // && and || are associative; every other operator groups to the
        // left, so an operand of its own level on the right keeps its
        // parentheses.
        writeAbove(&out, n.args[1], op == "&&" || op == "||" ? p : p + 1)
    }
}

/// Writes a double as both builds do: a whole number below 1e21 with its
/// digits, any other in the shortest form that reads back, plain between
/// 1e-4 and 1e16 and with an exponent of at least two digits outside.
public func formatTracedDouble(_ d: Double) -> String {
    if d.isNaN { return "nan" }
    if d.isInfinite { return d < 0 ? "-inf" : "inf" }
    if d == d.rounded(.towardZero) && abs(d) < 1e21 { return String(format: "%.0f", d) }
    return "\(d)"
}

/// Writes a duration of seconds in ISO 8601's day-and-time form, as a
/// worked example does: P14D, P1DT2H, PT1H30M, PT0.5S, -PT15M, PT0S.
public func formatISODuration(_ seconds: Decimal) -> String {
    if seconds == 0 { return "PT0S" }
    var total = seconds < 0 ? -seconds : seconds
    func whole(_ d: Decimal) -> Decimal {
        var x = d, r = Decimal()
        NSDecimalRound(&r, &x, 0, .down)
        return r
    }
    let days = whole(total / 86400)
    total -= days * 86400
    let h = whole(total / 3600)
    total -= h * 3600
    let m = whole(total / 60)
    total -= m * 60
    var s = (seconds < 0 ? "-" : "") + "P"
    if days > 0 { s += formatDecimal(days) + "D" }
    if h == 0 && m == 0 && total == 0 { return s }
    s += "T"
    if h > 0 { s += formatDecimal(h) + "H" }
    if m > 0 { s += formatDecimal(m) + "M" }
    if total > 0 { s += formatDecimal(total) + "S" }
    return s
}

/// Writes a value as a worked example writes it, at its type, so that it
/// can be copied into one: a decimal in quotes at the type's scale, or with
/// up to 20 places when its scale is not fixed; a date, an instant, a time
/// of day and a duration in quotes, the duration in ISO 8601; a double as
/// formatTracedDouble; a number and a bool as they are.
public func showValue(_ v: ExprValue, _ t: ExprType?) -> String {
    switch v.kind {
    case .decimal:
        if let t, t.kind == .decimal, t.scale >= 0 { return quote(fixedDecimal(v.num, t.scale)) }
        // A decimal of no fixed scale, the result of /, with the places it
        // needs up to 20, rounded half away from zero past them: this build
        // holds 38 digits and the Go build more, and both agree to 20 places.
        return quote(formatDecimal(roundDecimal(v.num, 20)))
    case .double: return formatTracedDouble(v.double)
    case .duration: return quote(formatISODuration(v.seconds))
    case .string, .enumValue: return quote(v.text)
    case .date, .timestamp, .time: return quote(v.description)
    default: return v.description
    }
}
