import Foundation

/// The second-factor page as data, read by TaskPage.
let secondFactorSchema = TaskPageSchema(
    title: "second-factor.title",
    submit: .init(operation: "confirmSecondFactor", method: "POST", path: "/sessions/second-factor"),
    fields: [.init(type: "text", name: "code", label: "second-factor.fields.code", required: true, maxLength: 6, pattern: "^[0-9]{6}$")],
    checks: [],
    failed: [.init(status: 401, problem: "second-factor-refused", message: "second-factor.failed.second-factor-refused", field: "code")],
    events: [.init(status: 200, navigate: "members-list", message: "second-factor.onSubmitted.200")]
)
