import Foundation

/// The outcomes of an evaluation of specarch try.
enum TryOutcome: String {
    case pass = "pass"                       // an example whose expected value the formula gives
    case fail = "fail"                       // an example whose expected value it does not give
    case error = "cannot be computed"        // the formula fails on the inputs
    case unread = "cannot be read"           // an input or the expected value does not fit its field
    case computed = "computed"               // given inputs, which have no expected value
}

/// One evaluation of specarch try: a worked example, or the inputs given
/// on the command line.
struct Evaluation {
    var number = 0          // the example's place, from 1; 0 for given inputs
    var name = ""           // the example's name, "" for one with none
    var outcome = TryOutcome.computed
    var inputs: [(name: String, value: String)] = []
    var unread: [String] = [] // why an input or the expected value cannot be read
    var steps: [(expression: String, value: String)] = []
    var result: String?
    var expected: String?
    var error = ""
}

/// What specarch try found for one algorithm.
struct Tried {
    let algorithm: String
    var evaluations: [Evaluation] = []
}

/// Evaluates an algorithm of a specification with the validator's
/// evaluator, on each worked example, on the one named example, or once on
/// given inputs, a mapping written as an example's inputs are (ADR-195). It
/// returns a refusal when the algorithm, its formula or the example is not
/// there to evaluate. The specification is validated apart.
func tryAlgorithm(_ s: Spec, _ algorithm: String, _ example: String, _ given: YNode?) -> (Tried?, String) {
    guard let root = s.root else { return (nil, "the specification could not be read; the lines above say why") }
    let algs = root.child("algorithms")
    guard let alg = algs?.child(algorithm) else {
        let names = pairs(algs).map { $0.key.value }.sorted(by: byteLess)
        if names.isEmpty { return (nil, "the specification has no algorithms, so no algorithm \(algorithm)") }
        return (nil, "the specification has no algorithm \(algorithm); it has \(names.joined(separator: ", "))")
    }
    let c = Checker(file: s.rootPath, files: s.files)
    c.root = root
    let d = Design(root)
    d.spec = s
    c.checkTimeZone(d)
    guard let formula = alg.child("formula"), formula.kind == .scalar else {
        return (nil, "algorithm \(algorithm) has no formula to evaluate; write one under formula")
    }
    let inputs = pairs(alg.child("inputs"))
    var fields: [String: YNode] = [:]
    var env: ExprEnv = [:]
    for p in inputs {
        fields[p.key.value] = p.value
        env[p.key.value] = d.fieldType(p.value)
    }
    let (parsed, perrs) = parseExpr(formula.value)
    var errs = perrs
    var types: [ObjectIdentifier: ExprType] = [:]
    if let parsed, errs.isEmpty {
        (_, types, errs) = checkExprTypes(parsed, env)
    }
    guard let tree = parsed, errs.isEmpty else {
        let first = errs.first?.message ?? "it does not parse"
        return (nil, "the formula of algorithm \(algorithm) does not check: \(first); the validator's lines above say where")
    }
    guard let output = alg.child("output") else {
        return (nil, "algorithm \(algorithm) has no output, so its result has no type to be read at; declare one under output")
    }
    let want = d.fieldType(output)
    var t = Tried(algorithm: algorithm)
    func run(_ start: Evaluation, _ values: YNode?, _ expected: YNode?) {
        var e = start
        var vals: [String: ExprValue] = [:]
        for p in pairs(values) {
            guard let field = fields[p.key.value] else {
                e.unread.append("\(p.key.value) is not an input of algorithm \(algorithm)\(suggest(p.key.value, fields))")
                continue
            }
            let (v, msg) = d.value(p.value, field)
            if !msg.isEmpty {
                e.unread.append("input \(p.key.value) \(msg)")
                continue
            }
            vals[p.key.value] = v
        }
        for p in inputs {
            if let v = vals[p.key.value] {
                e.inputs.append((name: p.key.value, value: showValue(v, env[p.key.value])))
            } else if values?.child(p.key.value) == nil {
                e.unread.append("no value is given for input \(p.key.value)")
            }
        }
        var expectedValue: ExprValue?
        if let expected {
            let (v, msg) = d.value(expected, output)
            if !msg.isEmpty {
                e.unread.append("the expected value " + msg)
            } else {
                expectedValue = v
                e.expected = showValue(v, want)
            }
        }
        if !e.unread.isEmpty {
            e.outcome = .unread
            t.evaluations.append(e)
            return
        }
        let (got, steps, err) = traceExpr(tree, vals, c.zoneOf())
        for st in steps {
            e.steps.append((expression: exprText(st.node), value: showValue(st.value, types[ObjectIdentifier(st.node)])))
        }
        if let err {
            e.outcome = .error
            e.error = err.message
        } else if let got {
            e.result = showValue(got, want)
            if want.kind == .int && want.bits == 32 && !fitsInt(got.num, 32) {
                e.outcome = .fail
                e.error = "\(got) does not fit the output's int32"
            } else if expected == nil {
                e.outcome = .computed
            } else if let expectedValue, exprEqual(got, expectedValue) {
                e.outcome = .pass
            } else {
                e.outcome = .fail
            }
        }
        t.evaluations.append(e)
    }
    if let given {
        run(Evaluation(name: "given inputs"), given, nil)
        return (t, "")
    }
    let examples = items(alg.child("examples"))
    if examples.isEmpty {
        return (nil, "algorithm \(algorithm) has no worked examples; give inputs with --inputs, or write examples under examples")
    }
    var names: [String] = []
    for (i, ex) in examples.enumerated() {
        let name = str(ex.child("name"))
        if !name.isEmpty { names.append(quote(name)) }
        if !example.isEmpty && name != example { continue }
        run(Evaluation(number: i + 1, name: name), ex.child("inputs"), ex.child("expected"))
    }
    if t.evaluations.isEmpty {
        if names.isEmpty {
            return (nil, "algorithm \(algorithm) has no example \(quote(example)); none of its examples has a name")
        }
        return (nil, "algorithm \(algorithm) has no example \(quote(example)); its examples are \(names.joined(separator: ", "))")
    }
    return (t, "")
}

/// Whether a diagnostic is one specarch try says in its own lines for the
/// algorithm it tries: a worked example of it that does not hold or cannot
/// be computed.
func triedHere(_ d: Diagnostic, _ algorithm: String) -> Bool {
    guard d.rule == .exampleMismatch || d.rule == .exampleError else { return false }
    return d.path.hasPrefix(pointer("algorithms", algorithm, "examples") + "/")
}

/// The try command: evaluates an algorithm on its worked examples or on
/// given inputs and prints each step (ADR-195). It works on a specification
/// with errors too, after printing them (ADR-065), and exits 1 then.
func runTry(_ args: [String], _ stdout: TextSink, _ stderr: TextSink) -> Int32 {
    var example = "", inputs = "", file = ""
    var rest: [String] = []
    var i = 0
    while i < args.count {
        let a = args[i]
        var name = "", value = ""
        if a == "--" {
            rest += args[(i + 1)...]
            break
        } else if a == "--example" || a == "--inputs" || a == "--file" {
            guard i + 1 < args.count else {
                stderr.write("specarch try: \(a) needs a value\n\n\(usage)")
                return 2
            }
            name = a
            value = args[i + 1]
            i += 1
        } else if a.hasPrefix("--example=") || a.hasPrefix("--inputs=") || a.hasPrefix("--file=") {
            let cut = a.firstIndex(of: "=")!
            name = String(a[..<cut])
            value = String(a[a.index(after: cut)...])
        } else if a.hasPrefix("-") && a.count > 1 {
            stderr.write("specarch try has no option \(a); its options are --example, --inputs and --file\n\n\(usage)")
            return 2
        } else {
            rest.append(a)
            i += 1
            continue
        }
        switch name {
        case "--example": example = value
        case "--inputs": inputs = value
        default: file = value
        }
        i += 1
    }
    if rest.count != 2 {
        stderr.write("specarch try needs the algorithm's name and one folder, in that order\n\n\(usage)")
        return 2
    }
    if !example.isEmpty && !inputs.isEmpty {
        stderr.write("specarch try takes --example or --inputs, not both: --example evaluates a worked example, --inputs the inputs given\n\n\(usage)")
        return 2
    }
    var given: YNode?
    if !inputs.isEmpty {
        let doc = parseYAML(inputs)
        guard let r = doc.root, r.kind == .mapping, doc.problems.isEmpty else {
            stderr.write("specarch try: --inputs is \(quote(inputs)), which is not a YAML mapping; write the inputs as an example's inputs are, such as '{ daysLate: 7, dailyRate: \"0.50\" }'\n\n\(usage)")
            return 2
        }
        given = r
    }
    let algorithm = rest[0], folder = rest[1]
    let (ins, ioError) = collect([folder], stderr)
    if ioError { return 2 }
    var roots: [String] = []
    for input in ins { if case .root(let dir) = input { roots.append(dir) } }
    if roots.count != 1 {
        stderr.write("specarch try: \(folder) holds \(roots.count) specifications; name the folder of the one the algorithm is in\n")
        return 2
    }
    let s = Spec(dir: roots[0])
    if !file.isEmpty {
        let (reason, readError) = s.overlay(file, "algorithms", algorithm)
        if readError {
            stderr.write("specarch try: \(reason)\n")
            return 2
        }
        if !reason.isEmpty {
            stderr.write("specarch try: \(reason)\n")
            return 1
        }
    }
    var invalid = false
    for d in checkSpec(s, makeGiven([s])) where d.severity == .error && !triedHere(d, algorithm) {
        stdout.write(d.description + "\n")
        if let l = d.detailLine { stderr.write(l + "\n") }
        invalid = true
    }
    let (tried, refusal) = tryAlgorithm(s, algorithm, example, given)
    guard let tried else {
        stderr.write("specarch try: \(refusal)\n")
        return 1
    }
    let failed = printTried(stdout, tried, given != nil)
    return invalid || failed ? 1 : 0
}

/// Prints a summary line, then each evaluation with its inputs, its steps
/// and its result, and tells whether any did not pass or compute.
private func printTried(_ w: TextSink, _ t: Tried, _ givenInputs: Bool) -> Bool {
    var counts: [TryOutcome: Int] = [:]
    for e in t.evaluations { counts[e.outcome, default: 0] += 1 }
    if givenInputs {
        w.write("\(t.algorithm): given inputs, \(t.evaluations[0].outcome.rawValue)\n")
    } else {
        let n = t.evaluations.count
        var summary = "\(t.algorithm): \(n == 1 ? "1 example" : "\(n) examples"), \(counts[.pass] ?? 0) pass"
        for o in [TryOutcome.fail, .error, .unread] where (counts[o] ?? 0) > 0 {
            summary += ", \(counts[o]!) \(o.rawValue)"
        }
        w.write(summary + "\n")
    }
    var failed = false
    for e in t.evaluations {
        if e.number == 0 {
            w.write("given inputs: \(e.outcome.rawValue)\n")
        } else if e.name.isEmpty {
            w.write("example \(e.number): \(e.outcome.rawValue)\n")
        } else {
            w.write("example \(e.number) \(quote(e.name)): \(e.outcome.rawValue)\n")
        }
        if e.outcome != .pass && e.outcome != .computed { failed = true }
        if !e.inputs.isEmpty {
            w.write("  inputs: " + e.inputs.map { $0.name + " " + $0.value }.joined(separator: ", ") + "\n")
        }
        for u in e.unread { w.write("  cannot be read: \(u)\n") }
        for st in e.steps { w.write("  \(st.expression) = \(st.value)\n") }
        if e.outcome == .error {
            w.write("  cannot be computed: \(e.error)\n")
        } else if let r = e.result, let x = e.expected {
            w.write("  result \(r), expected \(x)\n")
        } else if let r = e.result {
            w.write("  result \(r)\n")
        }
        if e.outcome == .fail && !e.error.isEmpty { w.write("  \(e.error)\n") }
    }
    return failed
}
