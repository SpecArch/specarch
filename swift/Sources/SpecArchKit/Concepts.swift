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
    /// Refuses a time limit of zero, since a call given up at once is no
    /// call, and a specification on a store or a broker, which another
    /// party runs and which is no system specified on its own (ADR-134).
    func checkDependencies(_ d: Design) {
        for (name, dep) in d.dependencies {
            checkTimeout(dep.child("timeout"), pointer("dependencies", name, "timeout"), .dependency, "the call")
            let kind = str(dep.child("kind"))
            if ["database", "messageBroker", "fileStore"].contains(kind), let sp = dep.child("specification") {
                add(sp, pointer("dependencies", name, "specification"), .dependency,
                    "a dependency of kind \(kind) is a store or a broker, not a system specified on its own, so no specification describes it; remove specification and version, or give it kind system")
            }
        }
    }

    /// Checks that every dependency a channel is carried by is declared and
    /// of a kind that carries messages: a broker or a system (ADR-134).
    func checkChannels(_ d: Design) {
        for (name, ch) in d.channels {
            for (i, e) in items(ch.child("carriedBy")).enumerated() {
                let ptr = pointer("channels", name, "carriedBy", "\(i)")
                guard let dep = d.dependencies[e.value] else {
                    add(e, ptr, .channel, "\(e.value) is not a dependency of the specification; declare it under dependencies, or leave carriedBy out when the channel is this system's own\(suggest(e.value, d.dependencies))")
                    continue
                }
                let kind = str(dep.child("kind"))
                if ["database", "fileStore", "identityProvider"].contains(kind) {
                    add(e, ptr, .channel, "\(e.value) is a dependency of kind \(kind), which carries no messages; name the broker the channel lives on, or the system that receives its messages")
                }
            }
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
        for (i, e) in items(o.node.child("calls")).enumerated() where d.dependencies[callDependency(e.value)] == nil {
            let dep = callDependency(e.value)
            add(e, o.pointer("calls", "\(i)"), .dependency, "\(dep) is not a dependency of the specification; declare it under dependencies with its timeout\(suggest(dep, d.dependencies))")
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

    /// Checks that a guard's entity exists, and that what its writes says is
    /// a change no keyword of the entry already says, of fields the entity
    /// has other than its key and its state field (ADR-168). Its
    /// precondition and the values writes sets are checked with the
    /// expressions. trigger is the operationId or the command's words;
    /// method, path and body (its request body's $ref) are the operation's,
    /// or "" for a command.
    func checkGuard(_ d: Design, _ g: YNode?, _ ptr: String, _ trigger: String, _ method: String, _ path: String, _ body: String) {
        guard let g, let n = g.child("entity") else { return }
        if d.entities[n.value] == nil {
            add(n, ptr + "/entity", .guardRule, "\(n.value) is not an entity of the specification\(suggest(n.value, d.entities))")
        }
        guard let w = g.child("writes"), let ent = d.entities[n.value] else { return }
        let name = n.value
        let keyed = path.filter { $0 == "{" }.count == 1
        if method == "put" && keyed && body == "#/entities/" + name {
            add(w, ptr + "/writes", .guardRule, "a PUT of \(name) by its key already says what it writes, every field a caller may set; take writes out")
            return
        }
        if method == "delete" && keyed {
            add(w, ptr + "/writes", .guardRule, "a DELETE by its key already says what it writes, that the \(name) goes; take writes out")
            return
        }
        if let del = w.child("delete") {
            for t in items(ent.child("transitions")) where str(t.child("trigger")) == trigger {
                add(del, ptr + "/writes/delete", .guardRule, "\(trigger) triggers a transition of \(name) to \(str(t.child("to"))), and writes says it deletes the records; say one change: take delete out, or the transition")
                break
            }
        }
        let fields = fieldsOf(ent)
        let state = str(ent.child("stateField"))
        let key = items(ent.child("primaryKey")).map { $0.value }
        for p in pairs(w.child("set")) {
            let f = p.key.value
            let at = ptr + "/writes/set/" + escapeToken(f)
            if fields[f] == nil {
                add(p.key, at, .guardRule, "writes sets \(f), which is not a field of \(name)\(suggest(f, fields))")
            } else if key.contains(f) {
                add(p.key, at, .guardRule, "writes sets \(f), part of the key of \(name); the key names the records the change writes, so it is not set")
            } else if f == state {
                add(p.key, at, .guardRule, "writes sets \(f), the state field of \(name); a state is moved by a transition whose trigger is this entry, so name it in transitions instead")
            }
        }
    }
}
