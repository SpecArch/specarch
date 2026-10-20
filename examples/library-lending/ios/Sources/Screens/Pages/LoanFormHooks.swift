import Foundation

/// The starting values of the loan form's fields the implementation file
/// names under hooks: a copy is lent today, by the device's own calendar,
/// unless the desk records an earlier day.
enum LoanFormHooks {
    static let hooks: [String: FieldHook] = [
        "lentOn": { _ in Date().formatted(.iso8601.year().month().day()) },
    ]
}
