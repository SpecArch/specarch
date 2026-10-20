import Foundation
import Testing
@testable import Screens

// The project's own tests of the screens, beside the ones generate
// derives: the rules a page checks before it sends a request.
struct RulesTests {
    @Test("a required field that is empty is a problem")
    func required() {
        let field = FieldSchema(type: "text", name: "code", label: "code", required: true)
        #expect(Rules.fieldProblem(field, "", Strings.texts("en")) == "Fill this in.")
        #expect(Rules.fieldProblem(field, "123456", Strings.texts("en")) == nil)
    }

    @Test("a rule across fields reads the values by their wire names")
    func rule() {
        let rule: Rule = ["==", ["field", "newPassword"], ["field", "confirmPassword"]]
        #expect(Rules.evaluate(rule, ["newPassword": .string("a"), "confirmPassword": .string("a")]) == .bool(true))
        #expect(Rules.evaluate(rule, ["newPassword": .string("a"), "confirmPassword": .string("b")]) == .bool(false))
    }

    @Test("a decimal compares exactly")
    func decimal() {
        let rule: Rule = ["<", ["field", "fee"], ["value", 0.3]]
        #expect(Rules.evaluate(rule, ["fee": .number(Decimal(string: "0.1")! + Decimal(string: "0.2")!)]) == .bool(false))
    }
}
