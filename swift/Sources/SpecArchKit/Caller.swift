import Foundation

// The caller (meta-model 0.3, ADR-095): one declared record of who is
// making the call, whose properties a role's rows rule names as
// caller.<property>. The name caller is reserved wherever an expression's
// names come from, in a file written against 0.3; a 0.2 file keeps the
// name and is read with the version warning (ADR-094).

extension Design {
    /// The version the root file says.
    var metaModelVersion: String { str(root.child("specarch")) }

    /// The version the file is read as: its own, or 1.0.0 for a version
    /// line this program does not read.
    var reads: String { MetaModelFormat.design.reads(metaModelVersion) }

    /// Whether the file is written against a version that reserves the
    /// name caller: 0.3 and later.
    var reservesCaller: Bool { metaModelAtLeast(reads, 0, 3) }

    /// The key a handover gives the way the source reaches the customer
    /// under, in the file's version: source before 1.0.0.
    var sourceDeliveryKeyOfFile: String {
        MetaModelFormat.design.isEarlier(metaModelVersion) ? sourceDeliveryOldKey : sourceDeliveryKey
    }

    /// The caller's properties as an expression names them,
    /// caller.<property>, each typed as a field.
    func callerEnv() -> ExprEnv {
        var env: ExprEnv = [:]
        for p in pairs(root.child("caller")?.child("properties")) {
            env[exprCallerName + "." + p.key.value] = fieldType(p.value)
        }
        return env
    }

    /// Every role with a rows rule for the entity, in document order, each
    /// with its rule.
    func rowsOf(_ entity: String) -> [Pair] {
        if entity.isEmpty { return [] }
        var out: [Pair] = []
        for r in pairs(root.child("roles")) {
            if let rule = rowPolicies(r.value).node?.child(entity), rule.kind == .scalar {
                out.append((key: r.key, value: rule))
            }
        }
        return out
    }

    /// The entity an operation's request body is, or "".
    func requestEntity(_ op: YNode) -> String {
        for c in pairs(op.child("requestBody")?.child("content")) {
            let ref = str(c.value.child("schema")?.child("$ref"))
            if ref.hasPrefix("#/entities/") { return String(ref.dropFirst("#/entities/".count)) }
        }
        return ""
    }

    /// The entity a path parameter is the key of: the last path parameter
    /// named after an entity with Id after it (loanId, loan_id), or "".
    func keyEntity(_ o: Operation) -> String {
        var byKey: [String: String] = [:]
        for name in entities.keys.sorted() where byKey[keyName(name + "Id")] == nil {
            byKey[keyName(name + "Id")] = name
        }
        var found = ""
        for p in items(o.pathItem.child("parameters")) + items(o.node.child("parameters")) where str(p.child("in")) == "path" {
            if let e = byKey[keyName(str(p.child("name")))] { found = e }
        }
        return found
    }

    /// The entity an operation reads or writes: its answer's, its request
    /// body's, its guard's, its list's, or the one a path parameter is the
    /// key of, in that order.
    func rowsEntity(_ o: Operation) -> String {
        for e in [responseEntity(o.node), requestEntity(o.node), str(o.node.child("guard")?.child("entity")), str(o.node.child("listOf")?.child("entity"))] where !e.isEmpty {
            return e
        }
        return keyEntity(o)
    }

    /// Adds the cases of every rows rule over the entity an operation reads
    /// or writes: a list shows only the rows, a read by key outside them is
    /// not found, and a change that would leave them is refused (ADR-095).
    func rowsCases(_ s: Subject, _ o: Operation, _ call: String) {
        let entity = rowsEntity(o)
        let perm = str(o.node.child("permission"))
        for r in rowsOf(entity) {
            let role = r.key.value, rule = r.value.value
            // A role that does not hold the operation's permission is refused
            // before any row is read: that is the denied case.
            guard let held = roles[role], roleHolds(held).contains(perm) else { continue }
            let given = "a caller holding " + role + ", whose rows of " + entity + " are " + rule
            if o.method == "get" && responseIsList(o.node) {
                s.red("shows only the rows of " + role, frequent, given, call, "it lists only the " + entity + " records the rule holds for")
            } else if o.method == "get" {
                s.red("a record outside the rows of " + role + " is not found", frequent, given, call + " for a " + entity + " the rule does not hold for", "it is refused as not found, as if no record had that key")
            } else {
                s.red("a change outside the rows of " + role + " is refused", frequent, given, call + " with a change that would leave the rows", "it is refused and no " + entity + " changes")
            }
        }
    }

    /// Adds the case of a job acting as a role with a rows rule over an
    /// entity it reads or writes: it acts on those rows only.
    func jobRowsCases(_ s: Subject, _ job: YNode, _ run: String) {
        let role = str(job.child("role"))
        if role.isEmpty { return }
        var seen = Set<String>()
        for key in ["reads", "writes"] {
            for e in items(job.child(key)) {
                let entity = e.value
                if seen.contains(entity) { continue }
                seen.insert(entity)
                if let rule = rowPolicies(roles[role]).node?.child(entity) {
                    s.red("acts only on the rows of " + role + " in " + entity, frequent, "the job acts as " + role + ", whose rows of " + entity + " are " + rule.value, run, "it reads and writes only the " + entity + " records the rule holds for")
                }
            }
        }
    }
}

/// The caller's properties, sorted.
func callerProperties(_ root: YNode) -> [String] {
    pairs(root.child("caller")?.child("properties")).map { $0.key.value }.sorted(by: byteLess)
}

func callerPropertyList(_ root: YNode) -> String {
    let props = callerProperties(root)
    if props.isEmpty { return "the specification declares no caller" }
    return "its properties are " + props.joined(separator: ", ")
}

extension Checker {
    /// Checks the caller's properties as a value object's, and that nothing
    /// an expression's names come from is named caller in a file that
    /// reserves the name.
    func checkCaller(_ d: Design) {
        for p in pairs(d.root.child("caller")?.child("properties")) {
            checkField(p.value, ["caller", "properties", p.key.value])
        }
        guard d.reservesCaller else { return }
        func reserved(_ n: YNode?, _ ptr: String, _ what: String) {
            add(n, ptr, .reservedName, "\(what) is named caller, which a file on meta-model \(d.metaModelVersion) reserves for the caller record, since an expression naming it could mean \(what) or the caller; rename it")
        }
        for e in pairs(d.root.child("entities")) {
            if let props = e.value.child("properties"), props.child(exprCallerName) != nil {
                reserved(props.key(exprCallerName), pointer("entities", e.key.value, "properties", exprCallerName), "a field of entity " + e.key.value)
            }
        }
        for s in pairs(d.root.child("schemas")) {
            if let props = s.value.child("properties"), props.child(exprCallerName) != nil {
                reserved(props.key(exprCallerName), pointer("schemas", s.key.value, "properties", exprCallerName), "a property of schema " + s.key.value)
            }
        }
        for v in pairs(d.root.child("views")) {
            if let props = v.value.child("properties"), props.child(exprCallerName) != nil {
                reserved(props.key(exprCallerName), pointer("views", v.key.value, "properties", exprCallerName), "a property of view " + v.key.value)
            }
        }
        for a in pairs(d.root.child("algorithms")) {
            if let inputs = a.value.child("inputs"), inputs.child(exprCallerName) != nil {
                reserved(inputs.key(exprCallerName), pointer("algorithms", a.key.value, "inputs", exprCallerName), "an input of algorithm " + a.key.value)
            }
        }
    }

    /// Checks every role's row policies, under rowPolicies or rows, the
    /// key's spelling before 1.0.0: the entity exists, the rule parses,
    /// names only the entity's fields and the caller's properties, and
    /// gives true or false. A problem's pointer names the key the file
    /// gives.
    func checkRoleRows(_ d: Design) {
        let callerEnv = d.callerEnv()
        for r in pairs(d.root.child("roles")) {
            let (key, policies) = rowPolicies(r.value)
            for row in pairs(policies) {
                if d.spec?.derived.contains(ObjectIdentifier(row.value)) == true {
                    continue // checked once, at the entity, by checkTenant
                }
                let entity = row.key.value
                let ptr = pointer("roles", r.key.value, key, entity)
                guard let ent = d.entities[entity] else {
                    add(row.key, ptr, .rowPolicies, "\(entity) is not an entity of the specification, so the row policy has no rows to limit\(suggest(entity, d.entities))")
                    continue
                }
                let n = row.value
                guard n.kind == .scalar else { continue } // the schema reports it
                var env: ExprEnv = [:]
                for (name, f) in fieldsOf(ent) { env[name] = d.fieldType(f) }
                for (name, t) in callerEnv { env[name] = t }
                let (tree, perrs) = parseExpr(n.value)
                guard let tree, perrs.isEmpty else {
                    rowsErrors(n, ptr, perrs)
                    continue
                }
                let (t, errs) = checkExpr(tree, env)
                rowsErrors(n, ptr, errs)
                if errs.isEmpty && (t.kind != .bool || t.nullable) {
                    addFile(fileOf(n), exprLine(n, 1), ptr, .rowPolicies, "the rule gives \(t), but a row policy must give true or false for each record; compare the values with ==, <, > or similar")
                }
                if errs.isEmpty { zoneWarnings(n, ptr, "the row policy", tree, env) }
            }
        }
    }

    /// Reports a row policy's expression errors: a name that is neither the
    /// entity's nor the caller's is row_policies, a property the caller
    /// lacks is caller_property, and the rest are the expression rules.
    func rowsErrors(_ n: YNode, _ ptr: String, _ errs: [ExprError]) {
        let before = diags.count
        exprErrors(n, ptr, "the row policy", errs)
        for i in before..<diags.count where diags[i].rule == .expressionName {
            diags[i].rule = .rowPolicies
        }
    }

    /// Checks a test fixture's caller: a role or public as a word, or an
    /// object with the role and values for the caller's properties.
    func checkFixtureCaller(_ d: Design, _ n: YNode, _ ptr: String) {
        if n.kind == .scalar {
            let v = n.value
            if !v.isEmpty && v != "public" && d.roles[v] == nil && !isCross(v) {
                add(n, ptr, .testData, "caller \(v) is neither public nor a role of the specification\(suggest(v, d.roles))")
            }
            return
        }
        var props: [String: YNode] = [:]
        for p in pairs(d.root.child("caller")?.child("properties")) { props[p.key.value] = p.value }
        for kv in pairs(n) {
            let key = kv.key.value
            if key == "role" {
                let v = str(kv.value)
                if !v.isEmpty && v != "public" && d.roles[v] == nil && !isCross(v) {
                    add(kv.value, ptr + "/role", .testData, "caller role \(v) is neither public nor a role of the specification\(suggest(v, d.roles))")
                }
            } else if key.hasPrefix("x-") {
            } else if let field = props[key] {
                let (_, msg) = d.value(kv.value, field)
                if !msg.isEmpty {
                    add(kv.value, ptr + "/" + key, .testData, "caller property \(key) \(msg)")
                }
            } else {
                add(kv.key, ptr + "/" + key, .testData, "\(key) is not a property of the caller; \(callerPropertyList(d.root))")
            }
        }
    }
}

/// A name with case, underscores and hyphens ignored.
func keyName(_ s: String) -> String {
    s.replacingOccurrences(of: "_", with: "").replacingOccurrences(of: "-", with: "").lowercased()
}
