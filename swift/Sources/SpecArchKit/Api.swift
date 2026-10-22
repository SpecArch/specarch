import Foundation

// The keywords about an operation's interface from the dxlib study
// (docs/dxlib-lessons.md, section 2): a list's whitelists and page size,
// the limits a client keeps to, and the catalogue of problem types.

extension Checker {
    /// Checks that a list names an entity and its fields, that no encrypted
    /// field is searched or sorted, or filtered without a hash, and that the
    /// default page fits the maximum.
    func checkListOf(_ d: Design, _ o: Operation) {
        guard let l = o.node.child("listOf") else { return }
        var entNode = l.child("entity")
        let fields: [String: YNode]
        if let viewNode = l.child("view") {
            guard let v = d.views[viewNode.value] else {
                add(viewNode, o.pointer("listOf", "view"), .listOf, "\(viewNode.value) is not a view of the specification\(suggest(viewNode.value, d.views))")
                return
            }
            for p in pairs(v.child("properties")) where p.value.child("rows") != nil {
                add(viewNode, o.pointer("listOf", "view"), .listOf, "\(viewNode.value) carries the rows of \(str(p.value.child("rows"))) in \(p.key.value), and a list holds one row per record; list a view without rows, and read the rows by the record's identifier")
                return
            }
            entNode = viewNode
            fields = d.viewFields(v)
        } else {
            guard let e = d.entities[str(entNode)] else {
                if let entNode {
                    add(entNode, o.pointer("listOf", "entity"), .listOf, "\(entNode.value) is not an entity of the specification\(suggest(entNode.value, d.entities))")
                }
                return
            }
            fields = fieldsOf(e)
        }
        for list in ["searchable", "filterable", "sortable"] {
            for (i, item) in items(l.child(list)).enumerated() {
                let ptr = o.pointer("listOf", list, "\(i)")
                guard let f = fields[item.value] else {
                    add(item, ptr, .listOf, "\(item.value) is not a field of \(str(entNode)), so it cannot be \(list)\(suggest(item.value, fields))")
                    continue
                }
                guard str(f.child("atRest")) == "encrypted" else { continue }
                if list != "filterable" {
                    add(item, ptr, .listOf, "\(item.value) is encrypted at rest, so it cannot be \(list): storage cannot read it; leave it out of \(list)")
                } else if str(f.child("lookup")) != "hash" {
                    add(item, ptr, .listOf, "\(item.value) is encrypted at rest, so it can be filtered by only through a hash of it; add lookup: hash to the field, or leave it out of filterable")
                }
            }
        }
        let size = l.child("pageSize")
        if let def = size?.child("default"), let dv = intValue(def), let mv = intValue(size?.child("maximum")) {
            if dv > mv {
                add(def, o.pointer("listOf", "pageSize", "default"), .listOf, "the default page of \(dv) is above the maximum of \(mv); lower the default, or raise the maximum")
            }
        }
    }

    /// Refuses a rate over no time, and a burst below the rate.
    func checkLimits(_ o: Operation) {
        guard let rate = o.node.child("limits")?.child("rate") else { return }
        checkTimeout(rate.child("per"), o.pointer("limits", "rate", "per"), .limits, "the rate")
        if let burst = rate.child("burst"), let b = intValue(burst), let r = intValue(rate.child("requests")) {
            if b < r {
                add(burst, o.pointer("limits", "rate", "burst"), .limits, "a burst of \(b) is below the \(r) requests the rate allows, so it would lower the limit; raise it to at least \(r), or remove it")
            }
        }
    }

    /// Checks every response's problem type against the catalogue, and that
    /// a specification with a catalogue names a type on every 4xx and 5xx
    /// response.
    func checkProblems(_ d: Design, _ o: Operation) {
        let errors = d.root.child("errors")
        for r in pairs(o.node.child("responses")) {
            let code = r.key.value
            guard let p = r.value.child("problem") else {
                if !pairs(errors).isEmpty && code.count == 3 && (code.first == "4" || code.first == "5") {
                    add(r.key, o.pointer("responses", code), .problem, "the specification declares its problem types under errors, so the \(code) response of \(o.id) names one under problem")
                }
                continue
            }
            let ptr = o.pointer("responses", code, "problem")
            guard let t = errors?.child(p.value) else {
                var valid: [String: YNode] = [:]
                for e in pairs(errors) { valid[e.key.value] = e.value }
                add(p, ptr, .problem, "\(p.value) is not a problem type under errors\(suggest(p.value, valid))")
                continue
            }
            let status = str(t.child("status"))
            if status != code {
                add(p, ptr, .problem, "\(p.value) is a \(status) problem, and this is the \(code) response; name a problem type of status \(code)")
            }
        }
    }

    /// Warns of a request body that takes any object, because its schema
    /// is an object that names no property and does not say
    /// additionalProperties, and of responses with no success among them:
    /// in either, what the operation takes or answers is not written, and a
    /// reader that did not find it must not leave that silent (ADR-130).
    func checkInterface(_ o: Operation) {
        for mt in pairs(o.node.child("requestBody")?.child("content")) {
            guard let s = mt.value.child("schema"), objectOnly(s.child("type")) else { continue }
            if s.child("properties") != nil || s.child("additionalProperties") != nil { continue }
            warn(mt.key, o.pointer("requestBody", "content", mt.key.value, "schema"), .requestBodyOpen, "the \(mt.key.value) body of \(o.id) is an object that names no property, so what it takes is not written; list its fields under properties, or say additionalProperties: true if it takes any")
        }
        let responses = pairs(o.node.child("responses"))
        if responses.isEmpty { return }
        var codes: [String] = []
        for r in responses {
            let code = r.key.value
            if code == "default" { return }
            // Not a status code: the schema reports the key.
            if code.utf8.count != 3 { continue }
            if "123".contains(code.first!) { return }
            codes.append(code)
        }
        // A route closed on purpose never succeeds: its refusal is its answer.
        if codes.isEmpty || str(o.node.child("permission")) == permissionNobody { return }
        warn(o.node.key("responses"), o.pointer("responses"), .successResponseMissing, "\(o.id) answers \(codes.joined(separator: ", ")) and no success, so what it answers when it succeeds is not written; add its success response")
    }
}

extension Checker {
    /// Checks each middleware's refusals against the catalogue of problem
    /// types, as a response's are checked (ADR-131).
    func checkMiddlewares(_ d: Design) {
        for m in pairs(d.root.child("middlewares")) {
            checkRefusals(d, m.value.child("refusals"), pointer("middlewares", m.key.value, "refusals"), "middleware " + m.key.value)
        }
    }

    /// Checks that an operation's middlewares are declared, and warns of an
    /// operation that names none once the specification declares its
    /// middlewares, as a refusal names its problem once problem types are
    /// declared (ADR-131).
    func checkOperationMiddlewares(_ d: Design, _ o: Operation) {
        var declared: [String: YNode] = [:]
        for p in pairs(d.root.child("middlewares")) { declared[p.key.value] = p.value }
        guard let list = o.node.child("middlewares") else {
            if !declared.isEmpty {
                warn(o.pathItem.key(o.method), o.pointer(), .middlewareMissing, "the specification declares its middlewares, and \(o.id) names none; list the ones it runs, in order, under middlewares, or write middlewares: [] if it runs none")
            }
            return
        }
        for (i, item) in items(list).enumerated() where declared[item.value] == nil {
            add(item, o.pointer("middlewares", "\(i)"), .middleware, "\(item.value) is not a middleware of the specification\(suggest(item.value, declared))")
        }
    }
}

/// Says whether a type is object, alone or with null.
func objectOnly(_ t: YNode?) -> Bool {
    guard let t else { return false }
    if t.kind == .scalar { return t.value == "object" }
    var object = false
    for n in t.items {
        switch n.value {
        case "object": object = true
        case "null": break
        default: return false
        }
    }
    return object
}
