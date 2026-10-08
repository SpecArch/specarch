import Foundation

private func mapOf(_ n: YNode?) -> [String: YNode] {
    var m: [String: YNode] = [:]
    for p in pairs(n) { m[p.key.value] = p.value }
    return m
}

/// "a" or "an" before a word.
private func indefinite(_ word: String) -> String {
    "aeiou".contains(word.first ?? "x") ? "an" : "a"
}

/// The kind of subject a test names, and how a message names it.
private func testKindOf(_ t: YNode) -> (kind: String, label: String) {
    for k in ["operation", "command", "page", "requirement"] {
        let v = str(t.child(k))
        if !v.isEmpty { return (k, k + " " + v) }
    }
    let ent = str(t.child("entity"))
    if ent.isEmpty { return ("", "") }
    if t.child("constraint") != nil { return ("constraint", ent + " constraint " + str(t.child("constraint"))) }
    if t.child("transition") != nil { return ("transition", ent + " transition") }
    return ("flow", ent + " state machine")
}

/// Whether every name the expression uses has a value.
private func allGiven(_ n: ExprNode, _ vals: [String: ExprValue]) -> Bool {
    if n.op == .name && vals[n.text] == nil { return false }
    return n.args.allSatisfy { allGiven($0, vals) }
}

extension Design {
    /// Checks one value against a field's schema and returns the end of a
    /// sentence when it does not fit. An object or a list is not checked: a
    /// test names them only as a whole.
    func fieldValue(_ n: YNode, _ field: YNode?) -> String {
        guard let field else { return "" }
        let k = fieldType(field).kind
        if k == .object || k == .list { return "" }
        return value(n, field).1
    }

    /// The entity the response with a status returns, alone or in a list;
    /// with no status, the first success response's.
    func statusEntity(_ op: YNode, _ status: String) -> String {
        for r in pairs(op.child("responses")) {
            if !status.isEmpty && r.key.value != status || status.isEmpty && !r.key.value.hasPrefix("2") { continue }
            for ct in pairs(r.value.child("content")) {
                var schema = ct.value.child("schema")
                if let items = schema?.child("items") { schema = items }
                let ref = str(schema?.child("$ref"))
                if ref.hasPrefix("#/entities/") { return String(ref.dropFirst("#/entities/".count)) }
            }
            return ""
        }
        return ""
    }
}

extension Checker {
    /// Checks the structured fixture, input and expect of every test against
    /// the design, and that a test does not say the same thing in a data
    /// folder too.
    func checkTestData(_ d: Design) {
        for p in pairs(d.root.child("tests")) {
            let name = p.key.value, t = p.value
            let base = ["tests", name]
            let (kind, label) = testKindOf(t)
            if kind.isEmpty { continue } // the schema reports a test without a subject
            for kv in pairs(t.child("fixture")) {
                let ptr = pointer(base + ["fixture", kv.key.value])
                if kv.key.value == "caller" {
                    let v = str(kv.value)
                    if !v.isEmpty && v != "public" && d.roles[v] == nil {
                        add(kv.value, ptr, .testData, "caller \(v) is neither public nor a role of the specification\(suggest(v, d.roles))")
                    }
                } else if !kv.key.value.hasPrefix("x-") {
                    checkRecords(d, kv, ptr)
                }
            }
            if let input = t.child("input") {
                checkInput(d, t, kind, label, input, pointer(base + ["input"]))
            }
            if let ex = t.child("expect") {
                checkExpect(d, t, kind, label, ex, base + ["expect"])
            }
            if let dir = d.spec?.dir {
                let folder = joinPath(joinPath(dir, "tests"), name)
                for (key, sub) in [("input", "input"), ("expect", "expected")] where t.child(key) != nil {
                    var isDir: ObjCBool = false
                    if FileManager.default.fileExists(atPath: joinPath(folder, sub), isDirectory: &isDir), isDir.boolValue {
                        add(t.key(key), pointer(base + [key]), .testData,
                            "the test has \(key) and \(indefinite(sub)) \(sub)/ folder beside it, which say the same thing twice; keep \(key), or keep the folder for what \(key) cannot hold")
                    }
                }
            }
        }
    }

    /// Checks records by entity name: the entity and its fields exist, the
    /// values have their types, and the entity's check constraints hold on
    /// the fields a record gives.
    private func checkRecords(_ d: Design, _ kv: Pair, _ ptr: String) {
        let ent = kv.key.value
        guard let entity = d.entities[ent] else {
            add(kv.key, ptr, .testData, "\(ent) is not an entity of the specification\(suggest(ent, d.entities))")
            return
        }
        let fields = fieldsOf(entity)
        for (i, rec) in items(kv.value).enumerated() {
            let rptr = ptr + "/\(i)"
            var vals: [String: ExprValue] = [:]
            for f in pairs(rec) {
                let fptr = rptr + "/" + escapeToken(f.key.value)
                guard let field = fields[f.key.value] else {
                    add(f.key, fptr, .testData, "\(f.key.value) is not a field of \(ent)\(suggest(f.key.value, fields))")
                    continue
                }
                let msg = d.fieldValue(f.value, field)
                if !msg.isEmpty {
                    add(f.value, fptr, .testData, "\(f.key.value) of this \(ent) \(msg)")
                    continue
                }
                let (v, m) = d.value(f.value, field)
                if m.isEmpty { vals[f.key.value] = v }
            }
            for con in pairs(entity.child("constraints")) where str(con.value.child("kind")) == "check" {
                let (tree, errs) = parseExpr(str(con.value.child("expression")))
                guard let tree, errs.isEmpty, allGiven(tree, vals) else { continue }
                if let got = try? evalExpr(tree, vals), got.kind == .bool, !got.bool {
                    var msg = str(con.value.child("message"))
                    if msg.hasSuffix(".") { msg.removeLast() }
                    add(rec, rptr, .testData, "this \(ent) breaks \(con.key.value) (\(msg)); a test holds only records that could exist")
                }
            }
        }
    }

    /// Checks what a test's call carries against its subject.
    private func checkInput(_ d: Design, _ t: YNode, _ kind: String, _ label: String, _ input: YNode, _ ptr: String) {
        switch kind {
        case "operation":
            guard let o = d.operations[str(t.child("operation"))] else { return } // reported as test_subject
            var allowed: [String: YNode] = [:]
            var schemas: [String: YNode?] = [:]
            for p in items(o.pathItem.child("parameters")) + items(o.node.child("parameters")) {
                allowed[str(p.child("name"))] = p
                schemas[str(p.child("name"))] = p.child("schema")
            }
            let (body, _) = d.requestFields(o.node)
            for (k, v) in body {
                allowed[k] = v
                schemas[k] = v
            }
            for kv in pairs(input) {
                let kptr = ptr + "/" + escapeToken(kv.key.value)
                guard let schema = schemas[kv.key.value] else {
                    add(kv.key, kptr, .testData, "\(kv.key.value) is not a parameter or body field of operation \(o.id)\(suggest(kv.key.value, allowed))")
                    continue
                }
                let msg = d.fieldValue(kv.value, schema)
                if !msg.isEmpty { add(kv.value, kptr, .testData, "input \(kv.key.value) \(msg)") }
            }
        case "command":
            let cname = str(t.child("command"))
            guard let cmd = d.commands[cname] else { return }
            for kv in pairs(input) {
                let kptr = ptr + "/" + escapeToken(kv.key.value)
                switch kv.key.value {
                case "arguments":
                    var args: [String: YNode] = [:]
                    for a in items(cmd.child("arguments")) { args[str(a.child("name"))] = a }
                    for a in pairs(kv.value) {
                        let aptr = kptr + "/" + escapeToken(a.key.value)
                        guard let arg = args[a.key.value] else {
                            add(a.key, aptr, .testData, "\(a.key.value) is not an argument of command \(cname)\(suggest(a.key.value, args))")
                            continue
                        }
                        let values = str(arg.child("repeatable")) == "true" && a.value.kind == .sequence ? a.value.items : [a.value]
                        for v in values {
                            let msg = d.fieldValue(v, arg.child("schema"))
                            if !msg.isEmpty { add(v, aptr, .testData, "argument \(a.key.value) \(msg)") }
                        }
                    }
                case "options":
                    let opts = mapOf(cmd.child("options"))
                    for o in pairs(kv.value) {
                        let optr = kptr + "/" + escapeToken(o.key.value)
                        guard let opt = opts[o.key.value] else {
                            add(o.key, optr, .testData, "\(o.key.value) is not an option of command \(cname)\(suggest(o.key.value, opts))")
                            continue
                        }
                        let msg = d.fieldValue(o.value, opt.child("schema"))
                        if !msg.isEmpty { add(o.value, optr, .testData, "option \(o.key.value) \(msg)") }
                    }
                default:
                    add(kv.key, kptr, .testData, "\(kv.key.value) is not part of a command's input, which has arguments and options")
                }
            }
        case "page":
            let pname = str(t.child("page"))
            guard let page = d.pages[pname] else { return }
            let params = Set(pathParameters(str(page.child("route"))))
            for kv in pairs(input) {
                let kptr = ptr + "/" + escapeToken(kv.key.value)
                if kv.key.value == "action" {
                    var labels: [String: YNode] = [:]
                    for a in items(page.child("actions")) { labels[str(a.child("label"))] = a }
                    let v = str(kv.value)
                    if labels[v] == nil {
                        add(kv.value, kptr, .testData, "\(v) is not an action of page \(pname)\(suggest(v, labels))")
                    }
                    continue
                }
                if !params.contains(kv.key.value) {
                    add(kv.key, kptr, .testData, "\(kv.key.value) is not a route parameter of page \(pname); a page's input is its route parameters and action")
                }
            }
        default:
            add(t.key("input"), ptr, .testData, "input is for an operation, a command or a page, and this test is about \(label)")
        }
    }

    /// Checks a test's expected outcome against its subject.
    private func checkExpect(_ d: Design, _ t: YNode, _ kind: String, _ label: String, _ ex: YNode, _ base: [String]) {
        let o = kind == "operation" ? d.operations[str(t.child("operation"))] : nil
        func only(_ key: String, _ subject: String) -> Bool {
            if let n = ex.key(key), kind != subject {
                add(n, pointer(base + [key]), .testData, "\(key) is for \(indefinite(subject)) \(subject), and this test is about \(label)")
                return false
            }
            return true
        }
        if let n = ex.child("status"), only("status", "operation"), let o {
            if o.node.child("responses")?.child(n.value) == nil {
                add(n, pointer(base + ["status"]), .testData, "operation \(o.id) has no response \(n.value)\(suggest(n.value, mapOf(o.node.child("responses"))))")
            }
        }
        if let b = ex.child("body"), only("body", "operation"), let o {
            let ent = d.statusEntity(o.node, str(ex.child("status")))
            if !ent.isEmpty {
                let fields = fieldsOf(d.entities[ent])
                for kv in pairs(b) {
                    let ptr = pointer(base + ["body", kv.key.value])
                    guard let field = fields[kv.key.value] else {
                        add(kv.key, ptr, .testData, "\(kv.key.value) is not a field of \(ent), which the response returns\(suggest(kv.key.value, fields))")
                        continue
                    }
                    let msg = d.fieldValue(kv.value, field)
                    if !msg.isEmpty { add(kv.value, ptr, .testData, "\(kv.key.value) of the body \(msg)") }
                }
            }
        }
        if let n = ex.child("exit"), only("exit", "command") {
            let cname = str(t.child("command"))
            if let cmd = d.commands[cname], cmd.child("exitCodes")?.child(n.value) == nil {
                add(n, pointer(base + ["exit"]), .testData, "command \(cname) has no exit code \(n.value)\(suggest(n.value, mapOf(cmd.child("exitCodes"))))")
            }
        }
        _ = only("standardOutput", "command")
        for kv in pairs(ex.child("state")) {
            checkRecords(d, kv, pointer(base + ["state", kv.key.value]))
        }
        if let e = ex.child("emits"), only("emits", "operation"), let o {
            var emitted: [String: YNode] = [:]
            for m in items(o.node.child("emits")) { emitted[m.value] = m }
            for (i, m) in items(e).enumerated() where emitted[m.value] == nil {
                add(m, pointer(base + ["emits", "\(i)"]), .testData, "operation \(o.id) does not emit \(m.value)\(suggest(m.value, emitted))")
            }
        }
        if ex.child("emits") != nil && ex.child("emitsNothing") != nil {
            add(ex.key("emitsNothing"), pointer(base + ["emitsNothing"]), .testData, "emits and emitsNothing say opposite things; keep one")
        }
    }
}
