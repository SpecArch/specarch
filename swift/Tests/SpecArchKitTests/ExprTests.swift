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
    return try evalExpr(n!, vals)
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
