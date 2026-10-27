import Foundation
import Testing
@testable import SpecArchKit

private func dec(_ s: String) -> ExprValue {
    var v = ExprValue(.decimal)
    v.num = Decimal(string: s)!
    return v
}

private func evaluate(_ src: String, _ env: ExprEnv = [:], _ vals: [String: ExprValue] = [:]) throws -> ExprValue {
    let (n, errs) = parseExpr(src)
    #expect(errs.isEmpty, "\(src): \(errs)")
    let (_, cerrs) = checkExpr(n!, env)
    #expect(cerrs.isEmpty, "\(src): \(cerrs)")
    return try evalExpr(n!, vals, TimeZone(identifier: "UTC")!)
}

@Test func roundHalfAwayFromZero() {
    for (input, want) in ["2.675": "2.68", "-2.675": "-2.68", "2.665": "2.67", "0.004": "0.00", "-0.005": "-0.01"] {
        #expect(fixedDecimal(roundDecimal(Decimal(string: input)!, 2), 2) == want, "round(\(input), 2)")
    }
}

@Test func floorAndCeilOfNegativeDecimals() throws {
    let env: ExprEnv = ["x": ExprType(.decimal, scale: 2)]
    #expect(formatDecimal(try evaluate("floor(x)", env, ["x": dec("-2.50")]).num) == "-3")
    #expect(formatDecimal(try evaluate("ceil(x)", env, ["x": dec("-2.50")]).num) == "-2")
}

@Test func integerOverflowIsAnError() throws {
    var x = ExprValue(.int)
    x.num = Decimal(string: "4611686018427387904")!
    #expect(throws: EvalError.self) { try evaluate("x * 2", ["x": ExprType(.int)], ["x": x]) }
}

@Test func integerDivisionTruncates() throws {
    #expect(formatDecimal(try evaluate("-7 / 2").num) == "-3")
}

@Test func decimalScaleIsTracked() {
    let env: ExprEnv = ["a": ExprType(.decimal, scale: 2), "b": ExprType(.decimal, scale: 3)]
    for (src, want) in ["a + b": 3, "a * b": 5, "a / b": unknownScale, "round(a / b, 2)": 2, "min(a, b)": 3] {
        let (n, _) = parseExpr(src)
        let (t, errs) = checkExpr(n!, env)
        #expect(errs.isEmpty && t.scale == want, "\(src): scale \(t.scale), want \(want)")
    }
}

@Test func noImplicitConversion() {
    let env: ExprEnv = ["n": ExprType(.int, bits: 32), "d": ExprType(.decimal, scale: 2), "f": ExprType(.double)]
    for src in ["n * d", "d + f", "n == d", "int(d)", "decimal(f, 2)", "n < 1u"] {
        let (node, errs) = parseExpr(src)
        #expect(errs.isEmpty, "\(src)")
        #expect(!checkExpr(node!, env).1.isEmpty, "\(src): want a type error")
    }
}

@Test func outsideTheSubsetIsRefusedByName() {
    for (src, word) in ["a % 2": "%", "a in b": "in", "has(a)": "has", "a.b": ".", "[1, 2]": "lists", "a.size()": "method", "x.exists(y, y > 1)": "method"] {
        let (_, errs) = parseExpr(src)
        #expect(errs.first?.message.contains(word) == true, "\(src): \(errs)")
    }
}

/// The positions CEL gives: an identifier at its first character, an
/// operator at the operator, a call at its opening bracket.
@Test func positionsFollowCEL() {
    let env: ExprEnv = ["count": ExprType(.int, bits: 32), "price": ExprType(.decimal, scale: 2)]
    let (n, _) = parseExpr("count * price")
    #expect(checkExpr(n!, env).1.first?.column == 7)
    #expect(parseExpr("count % 2").1.first?.column == 7)
    #expect(parseExpr("decimal(count, 0) * ").1.first?.column == 21)
    #expect(parseExpr("has(a)").1.first?.column == 4)
}

/// date() of an instant is the instant's calendar date in the zone given,
/// whatever offset the instant was written with (ADR-193).
@Test func dateOfInstantReadsZone() throws {
    let utc = TimeZone(identifier: "UTC")!, jakarta = TimeZone(identifier: "Asia/Jakarta")!
    let env: ExprEnv = ["at": ExprType(.timestamp)]
    var at = ExprValue(.timestamp)
    at.seconds = parseTimestamp("2026-10-27T03:00:00+07:00")!
    let cases: [(String, TimeZone, String)] = [
        ("date(timestamp(\"2026-10-27T03:00:00+07:00\"))", utc, "2026-10-26"),
        ("date(timestamp(\"2026-10-27T03:00:00+07:00\"))", jakarta, "2026-10-27"),
        ("date(timestamp(\"2026-10-26T20:00:00Z\"))", jakarta, "2026-10-27"),
        ("date(timestamp(\"2026-10-26T16:59:59Z\"))", jakarta, "2026-10-26"),
        ("date(at)", jakarta, "2026-10-27"),
        ("string(at)", utc, "\"2026-10-26T20:00:00Z\""),
    ]
    for (src, zone, want) in cases {
        let (n, errs) = parseExpr(src)
        #expect(errs.isEmpty, "\(src): \(errs)")
        #expect(try evalExpr(n!, ["at": at], zone).description == want, "\(src) in \(zone.identifier)")
    }
    // A call's position is its opening parenthesis, as every call diagnostic has it.
    let (n, _) = parseExpr("date(at) == date(\"2026-10-27\") && size(string(at)) > 0")
    let dates = instantDates(n!, env)
    #expect(dates.count == 1 && dates.first?.column == 5)
}

/// Both builds hold a timeZone to the database's own spelling and refuse
/// the names a platform's library knows beyond it.
@Test func zoneNamesAreTheDatabases() {
    for name in ["Asia/Jakarta", "UTC", "Etc/UTC", "EST", "Etc/GMT+7", "America/Argentina/Buenos_Aires"] {
        #expect(zoneNamed(name) != nil, "\(name)")
    }
    for name in ["asia/jakarta", "Asia/JAKARTA", "utc", "Local", "", "PST", "GMT+7", "WIB", "+07:00", "posixrules"] {
        #expect(zoneNamed(name) == nil, "\(name)")
    }
}

/// Runs traceExpr on a checked expression and writes each step as
/// specarch try prints it.
private func trace(_ src: String, _ env: ExprEnv, _ vals: [String: ExprValue]) -> ([String], EvalError?) {
    let (n, errs) = parseExpr(src)
    #expect(errs.isEmpty, "\(src): \(errs)")
    let (_, types, cerrs) = checkExprTypes(n!, env)
    #expect(cerrs.isEmpty, "\(src): \(cerrs)")
    let (_, steps, err) = traceExpr(n!, vals, TimeZone(identifier: "UTC")!)
    return (steps.map { exprText($0.node) + " = " + showValue($0.value, types[ObjectIdentifier($0.node)]) }, err)
}

private func int(_ i: Int) -> ExprValue {
    var v = ExprValue(.int)
    v.num = Decimal(i)
    return v
}

@Test func traceStepsInOrderAtTheirScale() {
    let env: ExprEnv = ["daysLate": ExprType(.int, bits: 32), "dailyRate": ExprType(.decimal, scale: 2), "replacementCost": ExprType(.decimal, scale: 2)]
    let (got, err) = trace("min(decimal(daysLate, 0) * dailyRate, replacementCost)", env, ["daysLate": int(90), "dailyRate": dec("0.50"), "replacementCost": dec("25.00")])
    #expect(err == nil)
    #expect(got == [#"decimal(daysLate, 0) = "90""#, #"decimal(daysLate, 0) * dailyRate = "45.00""#, #"min(decimal(daysLate, 0) * dailyRate, replacementCost) = "25.00""#])
}

@Test func traceLeavesOutWhatIsNotComputed() {
    let env: ExprEnv = ["a": ExprType(.int), "b": ExprType(.int)]
    let vals = ["a": int(0), "b": int(4)]
    for (src, want) in [
        // The right operand of && is not read once the left is false.
        "a > 0 && b / a > 1": ["a > 0 = false", "a > 0 && b / a > 1 = false"],
        // The branch of ?: not taken is not computed, so b / a never fails.
        "a == 0 ? 0 : b / a": ["a == 0 = true", "a == 0 ? 0 : b / a = 0"],
        "a == 0 || b > 1": ["a == 0 = true", "a == 0 || b > 1 = true"],
    ] {
        let (got, err) = trace(src, env, vals)
        #expect(err == nil && got == want, "\(src): \(got)")
    }
    // A failure ends the trace after the steps computed before it.
    let (got, err) = trace("b - 1 + b / a", env, vals)
    #expect(err != nil && got == ["b - 1 = 3"], "division by zero: \(got)")
}

@Test func textKeepsOnlyTheParenthesesNeeded() {
    for (src, want) in [
        "(a + b) * c": "(a + b) * c",
        "a + (b * c)": "a + b * c",
        "a - (b - c)": "a - (b - c)",
        "(a - b) - c": "a - b - c",
        "!(x && y) || z": "!(x && y) || z",
        "x ? (y ? 1 : 2) : 3": "x ? (y ? 1 : 2) : 3",
        "x ? 1 : y ? 2 : 3": "x ? 1 : y ? 2 : 3",
        "-(a + b)": "-(a + b)",
        #"size("a\"b") + 2u"#: #"size("a\"b") + 2u"#,
        "2.0 * 0.50 + 1e-5": "2.0 * 0.5 + 1e-05",
        "caller.age >= 18": "caller.age >= 18",
        "duration('PT90M')": #"duration("PT90M")"#,
    ] {
        let (n, errs) = parseExpr(src)
        #expect(errs.isEmpty, "\(src): \(errs)")
        #expect(exprText(n!) == want, "exprText(\(src))")
    }
}

@Test func showWritesAsAnExampleDoes() {
    func duration(_ s: Decimal) -> ExprValue {
        var v = ExprValue(.duration)
        v.seconds = s
        return v
    }
    func double(_ d: Double) -> ExprValue {
        var v = ExprValue(.double)
        v.double = d
        return v
    }
    var third = ExprValue(.decimal)
    third.num = Decimal(10) / Decimal(3)
    var twoThirds = ExprValue(.decimal)
    twoThirds.num = Decimal(2) / Decimal(3)
    let cases: [(ExprValue, ExprType, String)] = [
        (duration(5400), ExprType(.duration), #""PT1H30M""#),
        (duration(-1.5), ExprType(.duration), #""-PT1.5S""#),
        (duration(0), ExprType(.duration), #""PT0S""#),
        (duration(14 * 86400), ExprType(.duration), #""P14D""#),
        (duration(30 * 3600), ExprType(.duration), #""P1DT6H""#),
        (third, ExprType(.decimal, scale: unknownScale), #""3.33333333333333333333""#),
        (twoThirds, ExprType(.decimal, scale: unknownScale), #""0.66666666666666666667""#),
        (double(1234567.5), ExprType(.double), "1234567.5"),
        (double(1.5e20), ExprType(.double), "150000000000000000000"),
        (double(1.25e21), ExprType(.double), "1.25e+21"),
        (dec("2.5"), ExprType(.decimal, scale: unknownScale), #""2.5""#),
        (dec("2.5"), ExprType(.decimal, scale: 2), #""2.50""#),
    ]
    for (v, t, want) in cases {
        #expect(showValue(v, t) == want, "showValue(\(v))")
    }
}
