import Foundation

// The integration contract (ADR-200): how this system proves itself to a
// dependency and where the secret it sends is kept, how often it may call,
// the operations of a dependency with no specification of its own, and on
// each call how our values map to theirs and back, the idempotency key
// sent, the retries, which of their answers become ours and whether the
// call waits for the answer.

/// One entry of a calls list: ref, the node naming what is called, and at,
/// its place under calls, the entry itself or, for an entry that gives the
/// call's contract, its call (ADR-200).
struct CallEntry {
    let ref: YNode
    let item: YNode
    let at: [String]
}

/// The entries of n's calls that name something; an object with no call is
/// the schema's to report.
func callEntries(_ n: YNode?) -> [CallEntry] {
    var out: [CallEntry] = []
    for (i, e) in items(n?.child("calls")).enumerated() {
        if e.kind == .scalar {
            out.append(CallEntry(ref: e, item: e, at: ["calls", "\(i)"]))
        } else if let r = e.child("call"), r.kind == .scalar {
            out.append(CallEntry(ref: r, item: e, at: ["calls", "\(i)", "call"]))
        }
    }
    return out
}

/// "; did you mean X?" or the names of a mapping's keys, as suggest gives.
func suggestPairs(_ name: String, _ n: YNode?) -> String {
    var all: [String: YNode] = [:]
    for p in pairs(n) { all[p.key.value] = p.value }
    return suggest(name, all)
}

extension Design {
    /// The operation a calls entry names under its dependency's operations,
    /// with its name, or nil when it names none.
    func dependencyOperation(_ ref: String) -> (YNode?, String) {
        guard let (dep, ptr) = crossRef(ref), ptr.hasPrefix("/operations/") else { return (nil, "") }
        let name = String(ptr.dropFirst("/operations/".count))
        return (dependencies[dep]?.child("operations")?.child(name), name)
    }

    /// The names a call's send expressions and idempotency key may use: the
    /// operation's parameters, its request body's fields and the fields of
    /// the record its guard names, with the names two of them share.
    func ours(_ o: Operation) -> (ExprEnv, [String: [String]]) {
        var env: ExprEnv = [:]
        var from: [String: [String]] = [:]
        func put(_ name: String, _ whereFrom: String, _ t: ExprType) {
            if env[name] == nil { env[name] = t }
            if !(from[name] ?? []).contains(whereFrom) { from[name, default: []].append(whereFrom) }
        }
        for p in items(o.pathItem.child("parameters")) + items(o.node.child("parameters")) {
            let name = str(p.child("name"))
            if !name.isEmpty, let schema = p.child("schema") { put(name, "a parameter", fieldType(schema)) }
            else if !name.isEmpty { put(name, "a parameter", ExprType(.string)) }
        }
        let (body, _) = requestFields(o.node)
        for name in body.keys.sorted(by: byteLess) { put(name, "a field of the request", fieldType(body[name]!)) }
        let ent = str(o.node.child("guard")?.child("entity"))
        if let e = entities[ent] {
            let fields = fieldsOf(e)
            for name in fields.keys.sorted(by: byteLess) { put(name, "a field of " + ent, fieldType(fields[name]!)) }
        }
        return (env, from)
    }

    /// The names and types of an object schema's properties.
    func fieldsEnv(_ props: YNode?) -> ExprEnv {
        var env: ExprEnv = [:]
        for p in pairs(props) { env[p.key.value] = fieldType(p.value) }
        return env
    }
}

/// The fields of a dependency operation's first answer that succeeds.
func successFields(_ op: YNode?) -> YNode? {
    for r in pairs(op?.child("responses")) where r.key.value.hasPrefix("2") {
        return r.value.child("properties")
    }
    return nil
}

/// Whether a value of type t may be written where want is taken: the same
/// kind, a whole number where a decimal or a double is taken, or an enum
/// value as a string. An object or a list is not looked into.
func assignable(_ t: ExprType, _ want: ExprType) -> Bool {
    if t.kind == want.kind || (t.kind == .null && want.nullable) { return true }
    if [.object, .list].contains(t.kind) || [.object, .list].contains(want.kind) { return true }
    if [.int, .uint].contains(t.kind) && [.decimal, .double].contains(want.kind) { return true }
    return t.kind == .enumValue && want.kind == .string
}

/// Whether a value of type t, assignable where want is taken, may carry a
/// null that want does not allow.
func mayBeNull(_ t: ExprType, _ want: ExprType) -> Bool { t.nullable && !want.nullable }

/// The names an expression reads, each once, in order.
func exprNames(_ n: ExprNode) -> [String] {
    var out: [String] = []
    func walk(_ n: ExprNode) {
        if n.op == .name && !out.contains(n.text) { out.append(n.text) }
        for a in n.args { walk(a) }
    }
    walk(n)
    return out
}

extension Checker {
    /// Checks each dependency's contract and each call's.
    func checkIntegrations(_ d: Design) {
        for p in pairs(d.root.child("dependencies")) {
            checkDependencyContract(d, p.key.value, p.value)
        }
        for o in d.opList {
            for e in callEntries(o.node) { checkCall(d, o, e) }
        }
        for j in pairs(d.root.child("jobs")) {
            for e in callEntries(j.value) { _ = checkCallReference(d, e, pointer(["jobs", j.key.value] + e.at)) }
        }
        for g in pairs(d.root.child("gateways")) {
            for e in callEntries(g.value) { _ = checkCallReference(d, e, pointer(["gateways", g.key.value] + e.at)) }
        }
    }

    /// Checks a dependency's auth and credential, and its operations.
    func checkDependencyContract(_ d: Design, _ name: String, _ dep: YNode) {
        func at(_ tokens: String...) -> String { pointer(["dependencies", name] + tokens) }
        let auth = str(dep.child("auth"))
        let cred = dep.child("credential")
        if !auth.isEmpty && auth != "none" && cred == nil {
            add(dep.child("auth"), at("auth"), .integration, "\(name) proves itself by \(auth) and names no credential; name the setting under configuration, marked secret, that holds it")
        } else if auth == "none", let cred {
            add(cred, at("credential"), .integration, "\(name) is called with auth none, so it is sent no credential; remove credential, or say how this system proves itself")
        } else if let cred {
            if let setting = d.settings[cred.value] {
                if str(setting.child("secret")) != "true" {
                    add(cred, at("credential"), .integration, "\(cred.value) is a setting that is not secret, and a credential is; mark it secret: true, so its value is never written in a specification")
                }
            } else {
                add(cred, at("credential"), .setting, "\(cred.value) is not a setting of the specification's configuration\(suggest(cred.value, d.settings))")
            }
        }
        guard let ops = dep.child("operations") else { return }
        if dep.child("specification") != nil {
            add(ops, at("operations"), .integration, "\(name) names its specification, whose operations are its paths; call them as \(name)#/paths/..., and remove operations")
            return
        }
        for op in pairs(ops) {
            let n = op.value
            let ptr = at("operations", op.key.value)
            let method = n.child("method") != nil, channel = n.child("channel") != nil
            if !method && !channel {
                add(op.key, ptr, .integration, "\(op.key.value) of \(name) says neither method and path nor channel and message, so nothing says how it is called; give one of them")
            } else if method && channel {
                add(op.key, ptr, .integration, "\(op.key.value) of \(name) gives both method and path and channel and message; an operation is a request or a message sent, so keep one")
            } else if channel, let r = n.child("responses") {
                add(r, ptr + "/responses", .integration, "\(op.key.value) of \(name) is a message sent, which has no answer; remove responses, or make it a request with method and path")
            }
            let props = n.child("request")?.child("properties")
            for (i, r) in items(n.child("request")?.child("required")).enumerated() where props?.child(r.value) == nil {
                add(r, ptr + "/request/required/\(i)", .integration, "\(r.value) is required, and the request has no field \(r.value)\(suggestPairs(r.value, props))")
            }
        }
    }

    /// Checks a calls entry that names an operation under a dependency's
    /// operations: the dependency declares it and has no specification.
    func checkCallReference(_ d: Design, _ e: CallEntry, _ ptr: String) -> Bool {
        guard let (dep, p) = crossRef(e.ref.value), p.hasPrefix("/operations/"), let depNode = d.dependencies[dep] else { return true }
        let name = String(p.dropFirst("/operations/".count))
        if depNode.child("specification") != nil {
            add(e.ref, ptr, .integration, "\(dep) names its specification, so its operations are named by their paths; write \(dep)#/paths/<path>/<method>")
            return false
        }
        let ops = depNode.child("operations")
        if ops?.child(name) == nil {
            add(e.ref, ptr, .integration, "\(name) is not an operation of \(dep); declare it under \(dep)'s operations, with its method and path or its channel and message\(suggestPairs(name, ops))")
            return false
        }
        return true
    }

    /// Checks one calls entry of an operation: what it names and, for an
    /// entry that gives the call's contract, each part of it.
    func checkCall(_ d: Design, _ o: Operation, _ e: CallEntry) {
        if !checkCallReference(d, e, SpecArchKit.pointer(["paths", o.path, o.method] + e.at)) || e.item.kind == .scalar { return }
        let n = e.item
        func at(_ tokens: String...) -> String { SpecArchKit.pointer(["paths", o.path, o.method] + Array(e.at.prefix(2)) + tokens) }
        let dep = callDependency(e.ref.value)
        let (their, theirName) = d.dependencyOperation(e.ref.value)
        let cross = crossRef(e.ref.value) != nil
        let called = their != nil ? dep + "'s " + theirName : dep
        var mode = str(n.child("mode"))
        if mode.isEmpty { mode = "sync" }

        // The answer: a call that does not wait hears none.
        if mode != "sync" {
            if let k = n.child("errors") {
                add(k, at("errors"), .integration, "the call to \(called) is \(mode), so the operation hears no answer to map; remove errors, or make the call sync")
            }
            if let r = n.child("mapping")?.child("receive") {
                add(r, at("mapping", "receive"), .integration, "the call to \(called) is \(mode), so the operation hears no answer to keep; remove receive, or make the call sync")
            }
        }
        let cb = n.child("callback")
        if mode == "async" && cb == nil {
            add(n.child("mode"), at("mode"), .integration, "the call to \(called) is async and names no callback, so nothing says how its answer comes back; name the channel message under callback")
        } else if mode != "async", let cb {
            add(cb, at("callback"), .integration, "the call to \(called) is \(mode), and only an async call is answered by a callback; make it async, or remove callback")
        } else if let cb, d.message(cb.value) == nil {
            add(cb, at("callback"), .emits, "\(cb.value) does not name a channel and one of its messages; write channel/Message for a message declared under channels")
        }

        let (ours, from) = d.ours(o)
        func expression(_ v: YNode?, _ ptr: String, _ what: String, _ env: ExprEnv, _ shared: [String: [String]]) -> ExprType? {
            guard let v, v.kind == .scalar else { return nil }
            let (tree, perrs) = parseExpr(v.value)
            guard let tree, perrs.isEmpty else {
                exprErrors(v, ptr, what, perrs)
                return nil
            }
            let (t, errs) = checkExpr(tree, env)
            exprErrors(v, ptr, what, errs)
            if !errs.isEmpty { return nil }
            for name in exprNames(tree) where (shared[name] ?? []).count > 1 {
                add(v, ptr, .integration, "\(what) names \(name), which is both \(shared[name]!.joined(separator: " and ")); rename one, so the call reads one value")
                return nil
            }
            return t
        }

        // The mapping.
        let mapping = n.child("mapping")
        if let mapping, their == nil, !cross {
            add(mapping, at("mapping"), .integration, "the call names \(dep) alone, so its fields are not known; name the operation called, as \(dep)#/operations/<name>, and declare it under \(dep)'s operations")
        }
        if let mapping, let their {
            let props = their.child("request")?.child("properties")
            let send = mapping.child("send")
            for p in pairs(send) {
                let ptr = at("mapping", "send", p.key.value)
                guard let field = props?.child(p.key.value) else {
                    add(p.key, ptr, .integration, "\(p.key.value) is not a field of \(called)'s request\(suggestPairs(p.key.value, props))")
                    continue
                }
                if let t = expression(p.value, ptr, "the value sent as " + p.key.value, ours, from) {
                    let want = d.fieldType(field)
                    if !assignable(t, want) {
                        addFile(fileOf(p.value), exprLine(p.value, 1), ptr, .expressionType, "the value sent as \(p.key.value) gives \(t), and \(called)'s \(p.key.value) takes \(want)")
                    } else if mayBeNull(t, want) {
                        addFile(fileOf(p.value), exprLine(p.value, 1), ptr, .expressionType, "the value sent as \(p.key.value) gives \(t), and \(called)'s \(p.key.value) takes \(want), so a null would be sent where none is taken; send a value that is never null, as x != null ? x : a default")
                        diags[diags.count - 1].severity = .warning
                    }
                }
            }
            var missing: [String] = []
            for r in items(their.child("request")?.child("required")) where props?.child(r.value) != nil && send?.child(r.value) == nil {
                missing.append(r.value)
            }
            if !missing.isEmpty {
                let (whereNode, ptr) = send != nil ? (send, at("mapping", "send")) : (Optional(mapping), at("mapping"))
                add(whereNode, ptr, .integration, "\(called)'s request requires \(joinAnd(missing)), which send does not give; add each under send")
            }
            if let receive = mapping.child("receive"), mode == "sync" {
                let ent = str(o.node.child("guard")?.child("entity"))
                let fields = fieldsOf(d.entities[ent])
                let answer = successFields(their)
                let theirs = d.fieldsEnv(answer)
                for p in pairs(receive) {
                    let ptr = at("mapping", "receive", p.key.value)
                    if d.entities[ent] == nil {
                        add(p.key, ptr, .integration, "\(o.id) changes no record, since its guard names no entity, so there is nothing to keep \(p.key.value) in; give the operation a guard, or remove receive")
                        break
                    }
                    guard let field = fields[p.key.value] else {
                        add(p.key, ptr, .integration, "\(p.key.value) is not a field of \(ent)\(suggest(p.key.value, fields))")
                        continue
                    }
                    if answer == nil {
                        add(p.key, ptr, .integration, "\(called) declares no answer that succeeds with fields, so \(p.key.value) has nothing to be kept from; declare a 2xx response with its properties")
                        continue
                    }
                    if let t = expression(p.value, ptr, "the value kept as " + p.key.value, theirs, [:]) {
                        let want = d.fieldType(field)
                        if !assignable(t, want) {
                            addFile(fileOf(p.value), exprLine(p.value, 1), ptr, .expressionType, "the value kept as \(p.key.value) gives \(t), and \(ent)'s \(p.key.value) is \(want)")
                        } else if mayBeNull(t, want) {
                            addFile(fileOf(p.value), exprLine(p.value, 1), ptr, .expressionType, "the value kept as \(p.key.value) gives \(t), and \(ent)'s \(p.key.value) is \(want), so a null would be kept where none is allowed; keep a value that is never null, as x != null ? x : a default")
                            diags[diags.count - 1].severity = .warning
                        }
                    }
                }
            }
        }

        // The idempotency key.
        let key = n.child("idempotencyKey")
        if let key {
            _ = expression(key.child("value"), at("idempotencyKey", "value"), "the idempotency key", ours, from)
        }

        // Their answers, by status, when the operation called is declared.
        func theirStatus(_ v: YNode, _ ptr: String, _ what: String) {
            let s = v.value
            if s == "timeout" { return }
            if s.hasPrefix("2") {
                add(v, ptr, .integration, "\(s) is an answer that succeeds, and \(what) names only answers that fail; remove it")
            } else if let their, their.child("responses") != nil, their.child("responses")?.child(s) == nil {
                add(v, ptr, .integration, "\(s) is not an answer of \(called); name one of its responses that fails, or timeout")
            }
        }
        let retries = n.child("retries")
        for (i, s) in items(retries?.child("on")).enumerated() {
            theirStatus(s, at("retries", "on", "\(i)"), "retries")
        }
        if let retries, key == nil {
            let m = str(their?.child("method"))
            if m == "post" || m == "patch" {
                warn(retries, at("retries"), .integration, "the call to \(called) is tried again, and a \(m) is not idempotent, so a try whose answer was lost may act twice; send an idempotencyKey")
            }
        }
        for p in pairs(n.child("errors")) {
            let ptr = at("errors", p.key.value)
            theirStatus(p.key, ptr, "errors")
            if o.node.child("responses")?.child(p.value.value) == nil {
                add(p.value, ptr, .integration, "\(o.id) answers no \(p.value.value); declare that response, or map \(p.key.value) to one it declares")
            }
        }
    }
}
