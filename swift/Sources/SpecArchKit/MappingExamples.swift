import Foundation

// A call's mapping may hold worked examples (ADR-207): our values and
// what send gives from them, their answer and what receive keeps from
// it. The validator evaluates each, and specarch try prints the steps.

/// One field of send or receive: the field it gives, its expression
/// parsed and checked, and the names it reads.
struct MappingExpression {
    let field: String
    let node: YNode
    var tree: ExprNode?
    var types: [ObjectIdentifier: ExprType] = [:]
    var reads: [String] = []
    var ok = false // the expression parses and checks
}

/// send or receive of a mapping: its expressions in the order written, the
/// fields they may read with their types, and the fields their values are
/// written to.
struct MappingSide {
    let name: String // send or receive
    let given: YNode?
    var exprs: [MappingExpression] = []
    var env: ExprEnv
    var reads: [String: YNode]
    var targets: [String: YNode] = [:]

    /// Every name the expressions of the side read, in order.
    var readsAll: [String] {
        var out: [String] = []
        for m in exprs { for name in m.reads where !out.contains(name) { out.append(name) } }
        return out
    }
}

extension Design {
    /// A call's send and receive, read for its examples: send reads our
    /// values and gives their request's fields, receive reads their
    /// answer's fields and gives our record's.
    func mappingSides(_ o: Operation, _ mapping: YNode, _ their: YNode, _ ours: ExprEnv, _ oursFields: [String: YNode]) -> (MappingSide, MappingSide) {
        var send = MappingSide(name: "send", given: mapping.child("send"), env: ours, reads: oursFields)
        for p in pairs(their.child("request")?.child("properties")) { send.targets[p.key.value] = p.value }
        let answer = successFields(their)
        var receiveReads: [String: YNode] = [:]
        for p in pairs(answer) { receiveReads[p.key.value] = p.value }
        var receive = MappingSide(name: "receive", given: mapping.child("receive"), env: fieldsEnv(answer), reads: receiveReads)
        let ent = str(o.node.child("guard")?.child("entity"))
        if let e = entities[ent] { receive.targets = fieldsOf(e) }
        func read(_ side: inout MappingSide) {
            for p in pairs(side.given) {
                var m = MappingExpression(field: p.key.value, node: p.value)
                if p.value.kind == .scalar {
                    let (tree, perrs) = parseExpr(p.value.value)
                    if let tree, perrs.isEmpty {
                        m.tree = tree
                        let (_, types, errs) = checkExprTypes(tree, side.env)
                        m.types = types
                        m.ok = errs.isEmpty && side.targets[m.field] != nil
                        for name in exprNames(tree) where side.reads[name] != nil { m.reads.append(name) }
                    }
                }
                side.exprs.append(m)
            }
        }
        read(&send)
        read(&receive)
        return (send, receive)
    }
}

/// A whole number read where a decimal is taken, as a decimal, so a value
/// sent from an int compares with the decimal it is sent as.
func widened(_ v: ExprValue, _ want: ExprType) -> ExprValue {
    var v = v
    if want.kind == .decimal && (v.kind == .int || v.kind == .uint) { v.kind = .decimal }
    return v
}

extension Checker {
    /// Checks each worked example of a call's mapping: its values read at
    /// their fields, the names given against the names read, and each field
    /// send and receive give against the example's.
    func checkMappingExamples(_ d: Design, _ o: Operation, _ e: CallEntry, _ their: YNode, _ called: String, _ mode: String, _ ours: ExprEnv, _ oursFields: [String: YNode]) {
        guard let mapping = e.item.child("mapping") else { return }
        let examples = items(mapping.child("examples"))
        if examples.isEmpty { return }
        let (send, receive) = d.mappingSides(o, mapping, their, ours, oursFields)
        for (i, ex) in examples.enumerated() {
            let base = ["paths", o.path, o.method] + Array(e.at.prefix(2)) + ["mapping", "examples", "\(i)"]
            func at(_ tokens: [String]) -> String { pointer(base + tokens) }
            var label = str(ex.child("name"))
            if label.isEmpty { label = "example \(i + 1)" }
            if let sent = ex.child("sent") {
                if send.given == nil {
                    add(sent, at(["sent"]), .integration, "the mapping of the call to \(called) has no send, so \(quote(label)) has nothing sent to check; remove sent, or give send")
                } else {
                    checkMappingSide(d, label, send, ex.child("inputs"), sent, "inputs", "sent", at)
                }
            }
            if let kept = ex.child("kept") {
                if mode != "sync" {
                    add(kept, at(["kept"]), .integration, "the call to \(called) is \(mode), so the operation hears no answer to keep; remove kept, or make the call sync")
                } else if receive.given == nil {
                    add(kept, at(["kept"]), .integration, "the mapping of the call to \(called) has no receive, so \(quote(label)) has nothing kept to check; remove kept, or give receive")
                } else {
                    checkMappingSide(d, label, receive, ex.child("answer"), kept, "answer", "kept", at)
                }
            }
        }
    }

    /// Checks one side of a mapping example: the values given (inputs or
    /// answer) and the values expected (sent or kept).
    func checkMappingSide(_ d: Design, _ label: String, _ side: MappingSide, _ given: YNode?, _ expected: YNode, _ givenKey: String, _ expectedKey: String, _ at: ([String]) -> String) {
        var vals: [String: ExprValue] = [:]
        var ok = true
        let reads = side.readsAll
        for p in pairs(given) {
            guard let field = side.reads[p.key.value] else {
                if side.name == "send" {
                    add(p.key, at([givenKey, p.key.value]), .exampleInput, "\(p.key.value) is not a parameter, a field of the request or a field of the record the operation changes; remove it or correct its name\(suggest(p.key.value, side.reads))")
                } else {
                    add(p.key, at([givenKey, p.key.value]), .exampleInput, "\(p.key.value) is not a field of their answer; remove it or correct its name\(suggest(p.key.value, side.reads))")
                }
                ok = false
                continue
            }
            if !reads.contains(p.key.value) {
                add(p.key, at([givenKey, p.key.value]), .exampleInput, "\(side.name) reads no \(p.key.value), so \(quote(label)) need not give it; remove it")
                ok = false
                continue
            }
            let (v, msg) = d.value(p.value, field)
            if !msg.isEmpty {
                add(p.value, at([givenKey, p.key.value]), .exampleInput, "\(givenKey) \(p.key.value) of \(quote(label)) \(msg)")
                ok = false
                continue
            }
            vals[p.key.value] = v
        }
        let missing = reads.filter { given?.child($0) == nil }
        if !missing.isEmpty {
            if let given {
                add(given, at([givenKey]), .exampleInput, "\(quote(label)) gives no value for \(joinAnd(missing)), which \(side.name) reads; add each under \(givenKey)")
            } else {
                add(expected, at([expectedKey]), .exampleInput, "\(quote(label)) gives no value for \(joinAnd(missing)), which \(side.name) reads; add each under \(givenKey)")
            }
            ok = false
        }
        var want: [String: ExprValue] = [:]
        for p in pairs(expected) {
            guard side.given?.child(p.key.value) != nil, let field = side.targets[p.key.value] else {
                add(p.key, at([expectedKey, p.key.value]), .exampleExpected, "\(side.name) gives no \(p.key.value), so \(quote(label)) has no value to expect for it; remove it or correct its name\(suggestPairs(p.key.value, side.given))")
                continue
            }
            let (v, msg) = d.value(p.value, field)
            if !msg.isEmpty {
                add(p.value, at([expectedKey, p.key.value]), .exampleExpected, "\(expectedKey) \(p.key.value) of \(quote(label)) \(msg)")
                continue
            }
            want[p.key.value] = v
        }
        let left = side.exprs.map { $0.field }.filter { expected.child($0) == nil }
        if !left.isEmpty {
            add(expected, at([expectedKey]), .exampleExpected, "\(quote(label)) gives no \(expectedKey) value for \(joinAnd(left)), which \(side.name) gives; add each under \(expectedKey)")
        }
        if !ok { return }
        let what = side.name == "receive" ? "kept" : "sent"
        for m in side.exprs {
            guard m.ok, let tree = m.tree, let expectedValue = want[m.field], let node = expected.child(m.field), let target = side.targets[m.field] else { continue }
            let wantType = d.fieldType(target)
            let ptr = at([expectedKey, m.field])
            do {
                let got = widened(try evalExpr(tree, vals, zoneOf()), wantType)
                if !exprEqual(got, expectedValue) {
                    add(node, ptr, .exampleMismatch, "the value \(what) as \(m.field) gives \(show(got, wantType)) for \(quote(label)), but the example expects \(show(expectedValue, wantType)); correct the example or the mapping")
                }
            } catch {
                add(node, ptr, .exampleError, "the value \(what) as \(m.field) cannot be computed for \(quote(label)): \(error); correct the \(givenKey) or the mapping")
            }
        }
    }
}

/// One field of a mapping example as specarch try evaluated it: sent, a
/// field of their request from send, or kept, a field of our record from
/// receive, with the steps of its expression.
struct TriedField {
    let direction: String // sent or kept
    let field: String
    var steps: [(expression: String, value: String)] = []
    var outcome = TryOutcome.pass
    var result: String?
    var expected: String?
    var error = ""
}

/// What specarch try found for the mapping examples of one call.
struct TriedCall {
    let call: String // the call as the derived cases name it
    var evaluations: [Evaluation] = []
}

/// What specarch try found for an operation: each of its calls whose
/// mapping has examples (ADR-207).
struct TriedOperation {
    let operation: String
    var calls: [TriedCall] = []

    /// Where the evaluated examples are, for triedHere.
    var pointers: [String] { calls.flatMap { $0.evaluations.map { $0.pointer } } }
}

/// The specification's operations whose calls have mapping examples.
func operationsWithMappingExamples(_ root: YNode) -> [String] {
    var out: [String] = []
    for o in Design(root).opList where callEntries(o.node).contains(where: { !items($0.item.child("mapping")?.child("examples")).isEmpty }) {
        out.append(o.id)
    }
    return out.sorted(by: byteLess)
}

/// Evaluates the mapping examples of an operation's calls, or the one
/// example named, as tryAlgorithm evaluates an algorithm's (ADR-207). It
/// returns false when the specification has no operation of that name, and
/// a refusal when an algorithm has the name too or the operation has no
/// mapping example.
func tryOperation(_ s: Spec, _ name: String, _ example: String) -> (TriedOperation?, String, Bool) {
    guard let root = s.root else { return (nil, "", false) }
    let d = Design(root)
    d.spec = s
    guard let o = d.opList.first(where: { $0.id == name }) else { return (nil, "", false) }
    if root.child("algorithms")?.child(name) != nil {
        return (nil, "\(name) is both an algorithm and an operation's operationId, so it is not known which to try; rename one", true)
    }
    let c = Checker(file: s.rootPath, files: s.files)
    c.root = root
    c.checkTimeZone(d)
    let (t, refusal) = c.tryOperation(d, o, example)
    return (t, refusal, true)
}

extension Checker {
    func tryOperation(_ d: Design, _ o: Operation, _ example: String) -> (TriedOperation?, String) {
        var t = TriedOperation(operation: o.id)
        let (ours, _, oursFields) = d.ours(o)
        let entries = callEntries(o.node)
        let names = callNames(entries)
        var labels: [String] = []
        var unchecked = false
        for (k, e) in entries.enumerated() {
            guard let mapping = e.item.child("mapping") else { continue }
            let examples = items(mapping.child("examples"))
            let theirs = d.dependencyOperation(e.ref.value).0
            if !examples.isEmpty && theirs == nil { unchecked = true }
            guard !examples.isEmpty, let their = theirs else { continue }
            let (send, receive) = d.mappingSides(o, mapping, their, ours, oursFields)
            var tc = TriedCall(call: names[k])
            for (i, ex) in examples.enumerated() {
                let name = str(ex.child("name"))
                if !name.isEmpty { labels.append(quote(name)) }
                if !example.isEmpty && name != example { continue }
                var ev = Evaluation(number: i + 1, name: name)
                ev.pointer = pointer(["paths", o.path, o.method] + Array(e.at.prefix(2)) + ["mapping", "examples", "\(i)"])
                let sent = ex.child("sent"), kept = ex.child("kept")
                var sentVals: [String: ExprValue] = [:], keptVals: [String: ExprValue] = [:]
                if sent != nil {
                    if send.given == nil {
                        ev.unread.append("the mapping has no send, so nothing is sent")
                    } else {
                        (ev.inputs, sentVals) = tryGiven(d, &ev, send, ex.child("inputs"), "input")
                    }
                }
                let mode = str(e.item.child("mode"))
                if kept != nil {
                    if !mode.isEmpty && mode != "sync" {
                        ev.unread.append("the call is \(mode), so it hears no answer to keep")
                    } else if receive.given == nil {
                        ev.unread.append("the mapping has no receive, so nothing is kept")
                    } else {
                        (ev.answer, keptVals) = tryGiven(d, &ev, receive, ex.child("answer"), "answer")
                    }
                }
                if !ev.unread.isEmpty {
                    ev.outcome = .unread
                    tc.evaluations.append(ev)
                    continue
                }
                ev.outcome = .pass
                if let sent { tryFields(d, &ev, send, sentVals, sent, "sent") }
                if let kept { tryFields(d, &ev, receive, keptVals, kept, "kept") }
                tc.evaluations.append(ev)
            }
            if !tc.evaluations.isEmpty { t.calls.append(tc) }
        }
        if t.calls.isEmpty {
            if example.isEmpty && unchecked {
                return (nil, "operation \(o.id) has mapping examples only on calls through another specification's paths, whose fields are not known here, so they are read and not evaluated")
            }
            if example.isEmpty {
                return (nil, "operation \(o.id) has no call whose mapping has examples; write them under a call's mapping, as examples")
            }
            if labels.isEmpty {
                return (nil, "operation \(o.id) has no example \(quote(example)); none of its mapping examples has a name")
            }
            return (nil, "operation \(o.id) has no example \(quote(example)); its mapping examples are \(labels.joined(separator: ", "))")
        }
        return (t, "")
    }

    /// Reads the values a side of a mapping example gives, inputs or
    /// answer, in the order its expressions read them, and says in the
    /// evaluation which cannot be read.
    func tryGiven(_ d: Design, _ ev: inout Evaluation, _ side: MappingSide, _ given: YNode?, _ what: String) -> ([(name: String, value: String)], [String: ExprValue]) {
        var shown: [(name: String, value: String)] = []
        var vals: [String: ExprValue] = [:]
        let reads = side.readsAll
        for p in pairs(given) where !reads.contains(p.key.value) {
            ev.unread.append("\(side.name) reads no \(p.key.value)")
        }
        for name in reads {
            guard let n = given?.child(name) else {
                ev.unread.append("no value is given for \(what) \(name), which \(side.name) reads")
                continue
            }
            let (v, msg) = d.value(n, side.reads[name]!)
            if !msg.isEmpty {
                ev.unread.append("\(what) \(name) \(msg)")
                continue
            }
            vals[name] = v
            shown.append((name: name, value: showValue(v, side.env[name])))
        }
        return (shown, vals)
    }

    /// Evaluates each expression of a side of a mapping on the values given
    /// and compares it with the example's value.
    func tryFields(_ d: Design, _ ev: inout Evaluation, _ side: MappingSide, _ vals: [String: ExprValue], _ expected: YNode, _ direction: String) {
        for p in pairs(expected) where side.given?.child(p.key.value) == nil {
            ev.outcome = .fail
            ev.fields.append(TriedField(direction: direction, field: p.key.value, outcome: .fail, error: "\(side.name) gives no \(p.key.value)"))
        }
        for m in side.exprs {
            var f = TriedField(direction: direction, field: m.field)
            var decided = true
            var want: ExprValue?
            if !m.ok {
                f.outcome = .error
                f.error = "the expression does not check; the validator's lines above say why"
            } else if let node = expected.child(m.field), let target = side.targets[m.field] {
                let (v, msg) = d.value(node, target)
                if !msg.isEmpty {
                    f.outcome = .unread
                    f.error = "the \(direction) value \(msg)"
                } else {
                    want = v
                    f.expected = showValue(v, d.fieldType(target))
                    decided = false
                }
            } else {
                f.outcome = .fail
                f.error = "the example gives no \(direction) value for \(m.field)"
            }
            if !decided, let tree = m.tree, let want, let target = side.targets[m.field] {
                let wantType = d.fieldType(target)
                let (got, steps, err) = traceExpr(tree, vals, zoneOf())
                for st in steps {
                    f.steps.append((expression: exprText(st.node), value: showValue(st.value, m.types[ObjectIdentifier(st.node)])))
                }
                if let err {
                    f.outcome = .error
                    f.error = err.message
                } else if let got {
                    let g = widened(got, wantType)
                    f.result = showValue(g, wantType)
                    f.outcome = exprEqual(g, want) ? .pass : .fail
                }
            }
            if f.outcome != .pass && ev.outcome == .pass { ev.outcome = f.outcome }
            ev.fields.append(f)
        }
    }
}

/// Prints, for each call of an operation whose mapping has examples, a
/// summary line, then each example with its inputs, each field sent with
/// its steps, the answer and each field kept, and tells whether any did not
/// pass.
func printTriedOperation(_ w: TextSink, _ t: TriedOperation) -> Bool {
    var failed = false
    for c in t.calls {
        var counts: [TryOutcome: Int] = [:]
        for e in c.evaluations { counts[e.outcome, default: 0] += 1 }
        let n = c.evaluations.count
        var summary = "\(t.operation), call \(c.call): \(n == 1 ? "1 example" : "\(n) examples"), \(counts[.pass] ?? 0) pass"
        for o in [TryOutcome.fail, .error, .unread] where (counts[o] ?? 0) > 0 {
            summary += ", \(counts[o]!) \(o.rawValue)"
        }
        w.write(summary + "\n")
        for e in c.evaluations {
            if e.name.isEmpty {
                w.write("example \(e.number): \(e.outcome.rawValue)\n")
            } else {
                w.write("example \(e.number) \(quote(e.name)): \(e.outcome.rawValue)\n")
            }
            if e.outcome != .pass { failed = true }
            func values(_ label: String, _ vs: [(name: String, value: String)]) {
                if vs.isEmpty { return }
                w.write("  \(label): " + vs.map { $0.name + " " + $0.value }.joined(separator: ", ") + "\n")
            }
            values("inputs", e.inputs)
            for u in e.unread { w.write("  cannot be read: \(u)\n") }
            var answered = false
            for f in e.fields {
                if f.direction == "kept" && !answered {
                    values("answer", e.answer)
                    answered = true
                }
                for st in f.steps { w.write("  \(st.expression) = \(st.value)\n") }
                if let r = f.result, let x = f.expected {
                    var line = "  \(f.direction) \(f.field) \(r), expected \(x)"
                    if f.outcome != .pass { line += ", " + f.outcome.rawValue }
                    w.write(line + "\n")
                } else {
                    w.write("  \(f.direction) \(f.field) \(f.outcome.rawValue): \(f.error)\n")
                }
            }
            if !answered { values("answer", e.answer) }
        }
    }
    return failed
}
