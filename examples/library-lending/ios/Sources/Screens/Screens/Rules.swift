import Foundation

/// The values a page holds, by field, as typed.
public typealias Values = [String: String]

/// The values a rule reads: a page's fields, or a row's, as the service has them.
public typealias Fields = [String: JSON]

/// The rules the pages check before a request is sent, and the rules of
/// the specification evaluated over a record.
public enum Rules {
    static let email = try! NSRegularExpression(pattern: "^[^\\s@]+@[^\\s@]+$")

    /// Whether a value matches a pattern; one the app cannot read matches nothing.
    static func matches(_ pattern: String, _ value: String) -> Bool {
        guard let expression = try? NSRegularExpression(pattern: pattern) else {
            return false
        }
        return expression.firstMatch(in: value, range: NSRange(value.startIndex..., in: value)) != nil
    }

    static func length(_ value: String) -> Int {
        value.count
    }

    /// What is wrong with a field's value by its keywords; nil when nothing is.
    public static func fieldProblem(_ field: FieldSchema, _ value: String, _ texts: Texts) -> String? {
        if value.isEmpty {
            return field.required ? say(texts, "screens.required") : nil
        }
        if let min = field.minLength, length(value) < min {
            return say(texts, Message(text: "screens.minLength", count: Double(min)))
        }
        if let max = field.maxLength, length(value) > max {
            return say(texts, Message(text: "screens.maxLength", count: Double(max)))
        }
        if let pattern = field.pattern, !matches(pattern, value) {
            return say(texts, "screens.pattern")
        }
        if field.type == .email, email.firstMatch(in: value, range: NSRange(value.startIndex..., in: value)) == nil {
            return say(texts, "screens.email")
        }
        if field.type == .number {
            guard let number = Decimal(string: value, locale: Locale(identifier: "en_US_POSIX")), number.isFinite else {
                return say(texts, "screens.number")
            }
            if let step = field.step, step > 0, !isMultiple(number, of: Decimal(step)) {
                return say(texts, Message(text: "screens.step", count: step))
            }
            if let min = field.minimum, number < Decimal(min) {
                return say(texts, Message(text: "screens.minimum", count: min))
            }
            if let max = field.maximum, number > Decimal(max) {
                return say(texts, Message(text: "screens.maximum", count: max))
            }
        }
        if field.type == .date, !isDate(value) {
            return say(texts, "screens.date")
        }
        return nil
    }

    static func isMultiple(_ number: Decimal, of step: Decimal) -> Bool {
        var quotient = number / step
        var rounded = Decimal()
        NSDecimalRound(&rounded, &quotient, 0, .plain)
        return rounded == quotient
    }

    /// Whether a value is a day of the calendar, written yyyy-mm-dd.
    static func isDate(_ value: String) -> Bool {
        let parts = value.split(separator: "-", omittingEmptySubsequences: false)
        guard parts.count == 3, parts[0].count == 4, parts[1].count == 2, parts[2].count == 2,
              let year = Int(parts[0]), let month = Int(parts[1]), let day = Int(parts[2]) else {
            return false
        }
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: "UTC")!
        let components = DateComponents(year: year, month: month, day: day)
        return calendar.date(from: components).map { calendar.dateComponents([.year, .month, .day], from: $0) == components } ?? false
    }

    /// The value of a rule over the values of a page or a row.
    public static func evaluate(_ rule: Rule, _ values: Fields) -> JSON {
        guard case .list(let parts) = rule, let first = parts.first, case .string(let op) = first else {
            return constant(rule)
        }
        let args = Array(parts.dropFirst())
        switch op {
        case "field":
            if case .string(let name)? = args.first {
                return values[name] ?? .null
            }
            return .null
        case "value":
            return args.first.map(constant) ?? .null
        case "!":
            return .bool(args.first.map { evaluate($0, values) } != .bool(true))
        case "size":
            if case .string(let s)? = args.first.map({ evaluate($0, values) }) {
                return .number(Decimal(length(s)))
            }
            return .number(0)
        case "&&":
            return .bool(args.count == 2 && evaluate(args[0], values) == .bool(true) && evaluate(args[1], values) == .bool(true))
        case "||":
            return .bool(args.count == 2 && (evaluate(args[0], values) == .bool(true) || evaluate(args[1], values) == .bool(true)))
        default:
            break
        }
        guard args.count == 2 else {
            return .bool(false)
        }
        let left = evaluate(args[0], values)
        let right = evaluate(args[1], values)
        switch op {
        case "==": return .bool(left == right)
        case "!=": return .bool(left != right)
        default: break
        }
        switch (left, right) {
        case (.number(let l), .number(let r)):
            return .bool(compare(op, l, r))
        case (.string(let l), .string(let r)):
            return .bool(compare(op, l, r))
        default:
            return .bool(false)
        }
    }

    static func compare<T: Comparable>(_ op: String, _ l: T, _ r: T) -> Bool {
        switch op {
        case "<": return l < r
        case "<=": return l <= r
        case ">": return l > r
        case ">=": return l >= r
        default: return false
        }
    }

    /// A rule that is a constant, as a value.
    static func constant(_ rule: Rule) -> JSON {
        switch rule {
        case .string(let s): return .string(s)
        case .number(let n): return .number(n)
        case .bool(let b): return .bool(b)
        case .null: return .null
        case .list(let items): return .array(items.map(constant))
        }
    }

    /// What stops the page being sent: each field's problem by field, then
    /// each rule across fields that does not hold, under its field or, with
    /// none, under "".
    public static func problemsOf(_ schema: TaskPageSchema, _ values: Values, _ texts: Texts) -> [String: String] {
        var problems: [String: String] = [:]
        for field in schema.fields {
            if let problem = fieldProblem(field, values[field.name] ?? "", texts) {
                problems[field.name] = problem
            }
        }
        let record = Fields(uniqueKeysWithValues: values.map { ($0.key, JSON.string($0.value)) })
        for check in schema.checks {
            let at = check.field ?? ""
            if problems[at] == nil, evaluate(check.rule, record) != .bool(true) {
                problems[at] = say(texts, check.message)
            }
        }
        return problems
    }

    /// The key under which a field's second entry is held.
    public static func againOf(_ field: FieldSchema) -> String {
        field.name + "#again"
    }

    /// A field's value as the service has it: a number, a boolean, text, or null when it is empty.
    public static func typed(_ field: FieldSchema, _ value: String) -> JSON {
        if field.type == .checkbox {
            return .bool(value == "true")
        }
        if value.isEmpty {
            return .null
        }
        if field.type == .number, let number = Decimal(string: value, locale: Locale(identifier: "en_US_POSIX")) {
            return .number(number)
        }
        return .string(value)
    }

    /// The values a form's rules read: the loaded record's fields, then the form's own, typed.
    public static func recordOf(_ fields: [FieldSchema], _ values: Values, _ loaded: Fields) -> Fields {
        var record = loaded
        for field in fields {
            record[field.name] = typed(field, values[field.name] ?? "")
        }
        return record
    }

    /// Whether a field is left out of the page while its rule holds.
    public static func hidden(_ rule: Rule?, _ record: Fields) -> Bool {
        rule.map { evaluate($0, record) == .bool(true) } ?? false
    }

    /// Whether a field is shown and not changed.
    public static func locked(_ field: FieldSchema, _ record: Fields) -> Bool {
        field.readOnly == true || hidden(field.readOnlyWhen, record)
    }

    /// What stops a form being sent: each field shown that the person may
    /// change, by its keywords and its second entry, then each check across the
    /// fields that does not hold, under its field or, with none, under "".
    public static func formProblems(_ fields: [FieldSchema], _ checks: [CheckSchema], _ values: Values, _ record: Fields, _ texts: Texts) -> [String: String] {
        var problems: [String: String] = [:]
        for field in fields {
            if hidden(field.hiddenWhen, record) || locked(field, record) {
                continue
            }
            if let problem = fieldProblem(field, values[field.name] ?? "", texts) {
                problems[field.name] = problem
            } else if field.twice != nil, (values[againOf(field)] ?? "") != (values[field.name] ?? "") {
                problems[againOf(field)] = say(texts, "screens.again")
            }
        }
        for check in checks {
            let at = check.field ?? ""
            if problems[at] == nil, evaluate(check.rule, record) != .bool(true) {
                problems[at] = say(texts, check.message)
            }
        }
        return problems
    }
}
