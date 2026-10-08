import Foundation

// The concepts the waiting red paths of docs/test-generation.md need:
// dependencies with a time limit, an idempotency key, validity on an
// entity, a session, and a guard on a data change.

/// The methods RFC 9110 (9.2.2) makes idempotent by themselves, so an
/// idempotency key on one says nothing.
private let idempotentMethods: Set<String> = ["get", "put", "delete"]

/// One guard with its pointer.
struct Guard {
    let node: YNode
    let ptr: String
}

extension Design {
    /// Every guard of the specification with its pointer, in a fixed order:
    /// operations in document order, then commands by name.
    func guards() -> [Guard] {
        var out: [Guard] = []
        for o in opList {
            if let g = o.node.child("guard") { out.append(Guard(node: g, ptr: o.pointer("guard"))) }
        }
        for name in commands.keys.sorted(by: byteLess) {
            if let g = commands[name]?.child("guard") { out.append(Guard(node: g, ptr: pointer("commands", name, "guard"))) }
        }
        return out
    }
}

/// The operation's header parameter of that name, or nil.
func headerParameter(_ o: Operation, _ name: String) -> YNode? {
    for p in items(o.pathItem.child("parameters")) + items(o.node.child("parameters")) {
        if str(p.child("in")) == "header" && str(p.child("name")) == name { return p }
    }
    return nil
}

extension Checker {
    /// Refuses a time limit of zero: a call given up at once is no call.
    func checkDependencies(_ d: Design) {
        for (name, dep) in d.dependencies {
            checkTimeout(dep.child("timeout"), pointer("dependencies", name, "timeout"), .dependency, "the call")
        }
    }

    /// Refuses a session timeout of zero.
    func checkSession(_ d: Design) {
        let s = d.root.child("session")
        for key in ["idleTimeout", "absoluteTimeout"] {
            checkTimeout(child(s, key), pointer("session", key), .session, "the session")
        }
    }

    func checkTimeout(_ n: YNode?, _ ptr: String, _ rule: Rule, _ what: String) {
        guard let n else { return }
        if let dur = parseDuration(n.value), dur == 0 {
            add(n, ptr, rule, "\(n.value) is no time at all; give \(what) a limit above zero")
        }
    }

    /// Checks that every dependency an operation calls is declared.
    func checkCalls(_ d: Design, _ o: Operation) {
        for (i, e) in items(o.node.child("calls")).enumerated() where d.dependencies[e.value] == nil {
            add(e, o.pointer("calls", "\(i)"), .dependency, "\(e.value) is not a dependency of the specification; declare it under dependencies with its timeout\(suggest(e.value, d.dependencies))")
        }
    }

    /// Checks that the key names a header parameter of the operation and
    /// that the method is one a repeat can change.
    func checkIdempotencyKey(_ o: Operation) {
        guard let n = o.node.child("idempotencyKey") else { return }
        let ptr = o.pointer("idempotencyKey")
        if idempotentMethods.contains(o.method) {
            add(n, ptr, .idempotencyKey, "\(o.method) is idempotent by itself (RFC 9110, 9.2.2), so idempotencyKey says nothing here; remove it")
            return
        }
        if headerParameter(o, n.value) == nil {
            add(n, ptr, .idempotencyKey, "\(n.value) is not a header parameter of \(o.id); declare it under parameters with in: header, or name one that is")
        }
    }

    /// Checks that an entity's validity names its own date or date-time
    /// fields, both of one format.
    func checkValidity(_ name: String, _ e: YNode, _ fields: [String: YNode]) {
        guard let v = e.child("validity") else { return }
        var formats: [String: String] = [:]
        for key in ["from", "until"] {
            guard let n = v.child(key) else { continue }
            let ptr = pointer("entities", name, "validity", key)
            guard let f = fields[n.value] else {
                add(n, ptr, .validity, "\(n.value) is not a field of \(name), so it cannot say when a record is valid\(suggest(n.value, fields))")
                continue
            }
            var format = str(f.child("format"))
            if format != "date" && format != "date-time" {
                if format.isEmpty { format = "no format" }
                add(n, ptr, .validity, "\(n.value) has \(format), but a validity field must be a date or a date-time")
                continue
            }
            formats[key] = format
        }
        if let from = formats["from"], let until = formats["until"], from != until {
            add(v.child("from"), pointer("entities", name, "validity", "from"), .validity,
                "from \(str(v.child("from"))) is a \(from) and until \(str(v.child("until"))) is a \(until); give both the same format")
        }
    }

    /// Checks that a guard's entity exists. Its precondition is checked with
    /// the expressions.
    func checkGuard(_ d: Design, _ g: YNode?, _ ptr: String) {
        guard let g, let n = g.child("entity") else { return }
        if d.entities[n.value] == nil {
            add(n, ptr + "/entity", .guardRule, "\(n.value) is not an entity of the specification\(suggest(n.value, d.entities))")
        }
    }
}
