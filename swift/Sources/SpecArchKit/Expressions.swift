import Foundation

extension Design {
    /// A field's type in the expression language: JSON Schema's type says how
    /// the value travels, its format what it is.
    func fieldType(_ f: YNode) -> ExprType {
        let ref = str(f.child("$ref"))
        if !ref.isEmpty {
            if ref.hasPrefix("#/enums/") {
                let (values, name, _) = enumValues(f)
                var t = ExprType(.enumValue)
                t.enumName = name
                t.values = values.keys.sorted(by: byteLess)
                return t
            }
            return ExprType(.object)
        }
        var t = ExprType(.string)
        let typ = f.child("type")
        var base = str(typ)
        for item in items(typ) {
            if item.value == "null" { t.nullable = true } else { base = item.value }
        }
        let format = str(f.child("format"))
        let (values, _, isEnum) = enumValues(f)
        if isEnum && base == "string" {
            t.kind = .enumValue
            t.values = values.keys.sorted(by: byteLess)
            return t
        }
        switch format {
        case "int32": t.kind = .int; t.bits = 32
        case "int64": t.kind = .int; t.bits = 64
        case "uint64": t.kind = .uint
        case "double": t.kind = .double
        case "decimal":
            t.kind = .decimal
            t.scale = Int(str(f.child("scale"))) ?? 0
        case "date": t.kind = .date
        case "date-time": t.kind = .timestamp
        case "duration": t.kind = .duration
        case "time": t.kind = .time
        case "byte", "binary": t.kind = .bytes
        default:
            switch base {
            case "integer": t.kind = .int; t.bits = 64
            case "number": t.kind = .double
            case "boolean": t.kind = .bool
            case "array": t.kind = .list
            case "object": t.kind = .object
            default: t.kind = .string
            }
        }
        return t
    }

    /// Reads a worked example's value as the field declares it. Returns the
    /// end of a plain sentence when the value does not fit.
    func value(_ n: YNode, _ field: YNode) -> (ExprValue, String) {
        let t = fieldType(field)
        let tag = n.tag
        if tag == "!!null" {
            if t.nullable { return (ExprValue(.null), "") }
            return (ExprValue(.null), "is null, but the field does not allow null; give a value")
        }
        let wireString = str(field.child("type")) == "string"
        switch t.kind {
        case .int, .uint:
            if wireString && tag != "!!str" {
                return (ExprValue(.null), "travels as a string, so write it in quotes: \"\(n.value)\"")
            }
            if !wireString && tag != "!!int" {
                return (ExprValue(.null), "must be a whole number, and is \(n.value)")
            }
            guard matches(digitsPattern, n.value), let d = Decimal(string: n.value) else {
                return (ExprValue(.null), "must be a whole number, and is \(n.value)")
            }
            if t.kind == .int && !fitsInt(d, t.bits) {
                return (ExprValue(.null), "is \(n.value), which does not fit in an int\(t.bits)")
            }
            if t.kind == .uint && d < 0 {
                return (ExprValue(.null), "is \(n.value), but a uint cannot be negative")
            }
            var v = ExprValue(t.kind)
            v.num = d
            return (v, "")
        case .double:
            guard tag == "!!int" || tag == "!!float", let d = Double(n.value) else {
                return (ExprValue(.null), "must be a number, and is \(n.value)")
            }
            var v = ExprValue(.double)
            v.double = d
            return (v, "")
        case .decimal:
            guard tag == "!!str", matches(decimalTextPattern, n.value), let d = Decimal(string: n.value) else {
                return (ExprValue(.null), "must be a decimal in quotes, such as \"\(exampleDecimal(t.scale))\", and is \(n.value); quote it")
            }
            if let dot = n.value.firstIndex(of: ".") {
                let frac = n.value[n.value.index(after: dot)...].count
                if frac > t.scale {
                    return (ExprValue(.null), "has \(frac) decimal places, but the field's scale is \(t.scale); round it")
                }
            }
            var v = ExprValue(.decimal)
            v.num = d
            return (v, "")
        case .bool:
            guard tag == "!!bool" else { return (ExprValue(.null), "must be true or false, and is \(n.value)") }
            var v = ExprValue(.bool)
            v.bool = n.value == "true"
            return (v, "")
        case .enumValue:
            if tag == "!!str" && t.values.contains(n.value) {
                var v = ExprValue(.enumValue)
                v.text = n.value
                return (v, "")
            }
            return (ExprValue(.null), "is \(n.value), which is not one of \(t.values.joined(separator: ", "))")
        case .date, .timestamp, .time:
            let seconds: Decimal?
            switch t.kind {
            case .date: seconds = parseDate(n.value)
            case .timestamp: seconds = parseTimestamp(n.value)
            default: seconds = parseTimeOfDay(n.value)
            }
            guard let s = seconds, tag == "!!str" else {
                return (ExprValue(.null), "must be a \(t.kind.rawValue) in quotes written as \(layoutName(t.kind)), and is \(n.value)")
            }
            var v = ExprValue(t.kind)
            v.seconds = s
            return (v, "")
        case .duration:
            guard tag == "!!str", let s = parseDuration(n.value) else {
                return (ExprValue(.null), "must be an ISO 8601 duration in quotes, such as \"PT2H\", and is \(n.value)")
            }
            var v = ExprValue(.duration)
            v.seconds = s
            return (v, "")
        case .bytes:
            guard tag == "!!str", let data = Data(base64Encoded: n.value) else {
                return (ExprValue(.null), "must be base64 in quotes, and is \(n.value)")
            }
            var v = ExprValue(.bytes)
            v.text = String(decoding: data, as: UTF8.self)
            return (v, "")
        case .string:
            guard tag == "!!str" else { return (ExprValue(.null), "is a string, so write it in quotes: \"\(n.value)\"") }
            var v = ExprValue(.string)
            v.text = n.value
            return (v, "")
        default:
            return (ExprValue(.null), "is \(t), which worked examples cannot hold; use a single value")
        }
    }
}

private func layoutName(_ k: ExprKind) -> String {
    switch k {
    case .date: return "YYYY-MM-DD"
    case .timestamp: return "YYYY-MM-DDThh:mm:ssZ"
    default: return "hh:mm:ss"
    }
}

private func exampleDecimal(_ scale: Int) -> String {
    scale <= 0 ? "10" : "10." + String(repeating: "0", count: scale)
}

private let isoDuration = try! NSRegularExpression(pattern: "^P(?:([0-9]+)D)?(?:T(?:([0-9]+)H)?(?:([0-9]+)M)?(?:([0-9]+)S)?)?$")

/// The day-and-time part of ISO 8601: P1DT2H3M4S, in seconds.
func parseDuration(_ s: String) -> Decimal? {
    guard s != "P", s != "PT", let m = isoDuration.firstMatch(in: s, range: NSRange(s.startIndex..., in: s)) else { return nil }
    let units = [86400, 3600, 60, 1]
    var total = 0
    for (i, u) in units.enumerated() {
        if let r = Range(m.range(at: i + 1), in: s), let v = Int(s[r]) { total += v * u }
    }
    return Decimal(total)
}

extension Decimal {
    /// Whether a duration in seconds is a whole number of days.
    var isWholeDays: Bool {
        var q = self / 86400, whole = Decimal()
        NSDecimalRound(&whole, &q, 0, .down)
        return whole == q
    }
}

/// The file line of a line inside an expression scalar.
func exprLine(_ n: YNode, _ line: Int) -> Int {
    n.style == .literal || n.style == .folded ? n.line + line : n.line + line - 1
}

/// Writes a value as a worked example would, a decimal with the output's
/// scale.
private func show(_ v: ExprValue, _ t: ExprType) -> String {
    if v.kind == .decimal && t.scale >= 0 { return quote(fixedDecimal(v.num, t.scale)) }
    return v.description
}

extension Checker {
    func exprErrors(_ n: YNode, _ ptr: String, _ what: String, _ errs: [ExprError]) {
        for e in errs {
            let rule: Rule
            switch e.kind {
            case .name: rule = .expressionName
            case .typeMismatch: rule = .expressionType
            case .syntax: rule = .expressionSyntax
            }
            addFile(fileOf(n), exprLine(n, e.line), ptr, rule, "\(what), column \(e.column): \(e.message)")
        }
    }

    func checkExpressions(_ d: Design) {
        for e in pairs(d.root.child("entities")) {
            var env: ExprEnv = [:]
            for (name, f) in fieldsOf(e.value) { env[name] = d.fieldType(f) }
            for con in pairs(e.value.child("constraints")) {
                // A check's expression, and the condition a unique constraint
                // holds under.
                for key in ["expression", "where"] {
                    guard let n = con.value.child(key), n.kind == .scalar else { continue }
                    let (what, kind) = key == "where" ? ("the condition", "a condition") : ("the check", "a check")
                    let ptr = pointer("entities", e.key.value, "constraints", con.key.value, key)
                    let (tree, perrs) = parseExpr(n.value)
                    guard let tree, perrs.isEmpty else {
                        exprErrors(n, ptr, what, perrs)
                        continue
                    }
                    let (t, errs) = checkExpr(tree, env)
                    exprErrors(n, ptr, what, errs)
                    if errs.isEmpty && (t.kind != .bool || t.nullable) {
                        addFile(fileOf(n), exprLine(n, 1), ptr, .expressionType, "\(what) gives \(t), but \(kind) must give true or false; compare the values with ==, <, > or similar")
                    }
                }
            }
        }
        for g in d.guards() {
            checkPrecondition(d, g)
        }
        for a in pairs(d.root.child("algorithms")) {
            checkAlgorithm(d, a.key.value, a.value)
        }
    }

    /// Checks a guard's precondition as a check constraint of its entity: it
    /// parses, names the entity's fields, and gives true or false.
    func checkPrecondition(_ d: Design, _ g: Guard) {
        guard let n = g.node.child("precondition"), n.kind == .scalar, let ent = d.entities[str(g.node.child("entity"))] else { return }
        var env: ExprEnv = [:]
        for (name, f) in fieldsOf(ent) { env[name] = d.fieldType(f) }
        let ptr = g.ptr + "/precondition"
        let (tree, perrs) = parseExpr(n.value)
        guard let tree, perrs.isEmpty else {
            exprErrors(n, ptr, "the precondition", perrs)
            return
        }
        let (t, errs) = checkExpr(tree, env)
        exprErrors(n, ptr, "the precondition", errs)
        if errs.isEmpty && (t.kind != .bool || t.nullable) {
            addFile(fileOf(n), exprLine(n, 1), ptr, .expressionType, "the precondition gives \(t), but a precondition must give true or false; compare the values with ==, <, > or similar")
        }
    }

    func checkAlgorithm(_ d: Design, _ name: String, _ alg: YNode) {
        guard let n = alg.child("formula"), n.kind == .scalar else { return }
        let ptr = pointer("algorithms", name, "formula")
        var inputs: [String: YNode] = [:]
        var env: ExprEnv = [:]
        for p in pairs(alg.child("inputs")) {
            inputs[p.key.value] = p.value
            env[p.key.value] = d.fieldType(p.value)
        }
        let (tree, perrs) = parseExpr(n.value)
        guard let tree, perrs.isEmpty else {
            exprErrors(n, ptr, "the formula", perrs)
            return
        }
        let (t, errs) = checkExpr(tree, env)
        if !errs.isEmpty {
            exprErrors(n, ptr, "the formula", errs)
            return
        }
        guard let output = alg.child("output") else { return }
        let want = d.fieldType(output)
        let (ok, why) = fits(t, want)
        if !ok {
            addFile(fileOf(n), exprLine(n, 1), ptr, .expressionType, why)
            return
        }
        for (i, ex) in items(alg.child("examples")).enumerated() {
            checkExample(d, name, i, ex, tree, inputs, output, want)
        }
    }

    func checkExample(_ d: Design, _ alg: String, _ i: Int, _ ex: YNode, _ tree: ExprNode, _ inputs: [String: YNode], _ output: YNode, _ want: ExprType) {
        let base = ["algorithms", alg, "examples", "\(i)"]
        var label = str(ex.child("name"))
        if label.isEmpty { label = "example \(i + 1)" }
        let given = ex.child("inputs")
        var vals: [String: ExprValue] = [:]
        var ok = true
        for p in pairs(given) {
            guard let field = inputs[p.key.value] else {
                add(p.key, pointer(base + ["inputs", p.key.value]), .exampleInput,
                    "\(p.key.value) is not an input of algorithm \(alg); remove it or correct its name\(suggest(p.key.value, inputs))")
                ok = false
                continue
            }
            let (v, msg) = d.value(p.value, field)
            if !msg.isEmpty {
                add(p.value, pointer(base + ["inputs", p.key.value]), .exampleInput, "input \(p.key.value) of \(quote(label)) \(msg)")
                ok = false
                continue
            }
            vals[p.key.value] = v
        }
        if let given {
            for name in inputs.keys.sorted(by: byteLess) where given.child(name) == nil {
                add(given, pointer(base + ["inputs"]), .exampleInput, "\(quote(label)) gives no value for input \(name); add it")
                ok = false
            }
        }
        guard let expNode = ex.child("expected") else { return }
        let (expected, msg) = d.value(expNode, output)
        if !msg.isEmpty {
            add(expNode, pointer(base + ["expected"]), .exampleExpected, "the expected value of \(quote(label)) \(msg)")
            return
        }
        if !ok { return }
        let got: ExprValue
        do {
            got = try evalExpr(tree, vals)
        } catch {
            add(ex, pointer(base), .exampleError, "the formula cannot be computed for \(quote(label)): \(error); correct the inputs or the formula")
            return
        }
        if want.kind == .int && want.bits == 32 && !fitsInt(got.num, 32) {
            add(expNode, pointer(base + ["expected"]), .exampleMismatch,
                "the formula gives \(got) for \(quote(label)), which does not fit the output's int32; correct the formula or widen the output")
            return
        }
        if !exprEqual(got, expected) {
            add(expNode, pointer(base + ["expected"]), .exampleMismatch,
                "the formula gives \(show(got, want)) for \(quote(label)), but the example expects \(show(expected, want)); correct the example or the formula")
        }
    }
}
