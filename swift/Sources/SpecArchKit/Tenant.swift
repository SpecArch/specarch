// The tenant scope (ADR-179): tenant names the caller's property, the key
// every tenant-owned entity carries and the entities every tenant reads
// whole. The document as read holds the rule <key> == caller.<property>
// under every role's rows for every entity that carries the key and is not
// shared, unless the role writes its own rule for it; each added node sits
// at the position of tenant. The file keeps the shorthand, and the
// validator checks the scope at tenant and at the entities.

extension Spec {
    /// Adds the rows rules the tenant scope derives to the merged document
    /// and returns the value node of each, so that a check reports a
    /// problem of a derived rule once, at the entity, and not at a rule
    /// nobody wrote. A scope with no property or no key derives nothing;
    /// the schema reports it.
    func tenantRules(_ merged: YNode) -> Set<ObjectIdentifier> {
        guard let tenantKey = merged.key("tenant") else { return [] }
        let tenant = merged.child("tenant")
        let property = str(tenant?.child("property"))
        let key = str(tenant?.child("key"))
        guard !property.isEmpty, !key.isEmpty else { return [] }
        let shared = Set(items(tenant?.child("shared")).map { str($0) })
        let owned = pairs(merged.child("entities")).filter {
            !shared.contains($0.key.value) && $0.value.child("properties")?.child(key) != nil
        }.map { $0.key.value }
        guard !owned.isEmpty else { return [] }
        let file = files[ObjectIdentifier(tenantKey)]
        func node(_ kind: YNode.Kind, _ value: String = "") -> YNode {
            let n = YNode(kind: kind, value: value, tag: kind == .mapping ? "" : "!!str", line: tenantKey.line, column: tenantKey.column)
            files[ObjectIdentifier(n)] = file
            return n
        }
        let rule = key + " == caller." + property
        var derived: Set<ObjectIdentifier> = []
        for r in pairs(merged.child("roles")) where r.value.kind == .mapping {
            let rows: YNode
            if let existing = r.value.child("rows") {
                guard existing.kind == .mapping else { continue } // the schema reports it
                rows = existing
            } else {
                rows = node(.mapping)
                r.value.pairs.append((node(.scalar, "rows"), rows))
            }
            for entity in owned where rows.key(entity) == nil {
                let v = node(.scalar, rule)
                rows.pairs.append((node(.scalar, entity), v))
                derived.insert(ObjectIdentifier(v))
            }
        }
        return derived
    }
}

extension Checker {
    /// Checks the tenant scope (ADR-179): its property is the caller's,
    /// every shared name is an entity that carries no key, there is a
    /// role to derive a rule for, every other entity carries the key or
    /// is limited by a rule of its own on every role, and the key
    /// compares with the property. A derived rule is checked here, once
    /// per entity, rather than at every role.
    func checkTenant(_ d: Design) {
        guard let tenant = d.root.child("tenant"), let property = tenant.child("property"), property.kind == .scalar else { return }
        let key = str(tenant.child("key"))
        guard !key.isEmpty else { return } // the schema reports it
        let callerEnv = d.callerEnv()
        let ruleOf = exprCallerName + "." + property.value
        let declared = callerEnv[ruleOf] != nil
        if !declared {
            add(property, pointer("tenant", "property"), .tenant, "\(property.value) is not a property of the caller, so the tenant scope has nothing to compare \(key) with; \(callerPropertyList(d.root))")
        }
        var shared: Set<String> = []
        for (i, n) in items(tenant.child("shared")).enumerated() {
            let ptr = pointer("tenant", "shared", String(i))
            if let ent = d.entities[n.value] {
                if fieldsOf(ent)[key] != nil {
                    add(n, ptr, .tenant, "\(n.value) carries \(key), so its rows are a tenant's, and shared says every tenant reads it whole; take it out of shared, or rename its field if it is not the tenant's key")
                }
            } else {
                add(n, ptr, .tenant, "\(n.value) is not an entity of the specification, so there is nothing to share\(suggest(n.value, d.entities))")
            }
            shared.insert(n.value)
        }
        let roles = pairs(d.root.child("roles"))
        if roles.isEmpty {
            add(d.root.key("tenant"), pointer("tenant"), .tenant, "tenant limits the rows of each role, and the specification has no roles, so no rule is derived and every tenant would reach every record; add the roles that call the system under roles, or take tenant out if the system serves one tenant")
            return
        }
        let rule = key + " == " + ruleOf
        for e in pairs(d.root.child("entities")) where !shared.contains(e.key.value) {
            // open are the roles with no rule of their own over the entity:
            // none, or the one derived from the scope.
            let open = roles.filter {
                guard let own = $0.value.child("rows")?.child(e.key.value) else { return true }
                return d.spec?.derived.contains(ObjectIdentifier(own)) == true
            }.map { $0.key.value }
            let fields = fieldsOf(e.value)
            guard fields[key] != nil else {
                if !open.isEmpty {
                    add(e.key, pointer("entities", e.key.value), .tenant, "\(e.key.value) carries no field \(key) and tenant.shared does not list it, so no rows rule limits it for \(joinAnd(open)) and every tenant would reach every record; add \(key) to \(e.key.value), list \(e.key.value) under tenant.shared if every tenant reads it whole, or write a rows rule for it on every role")
                }
                continue
            }
            guard declared, !open.isEmpty else { continue } // no rule is derived over the entity
            var env: ExprEnv = [:]
            for (name, f) in fields { env[name] = d.fieldType(f) }
            for (name, t) in callerEnv { env[name] = t }
            let (tree, perrs) = parseExpr(rule)
            var errs = perrs
            if let tree, perrs.isEmpty { errs = checkExpr(tree, env).1 }
            for err in errs {
                add(e.value.child("properties")?.key(key), pointer("entities", e.key.value, "properties", key), .tenant, "the tenant rule \(rule) cannot be read over \(e.key.value): \(err.message)")
            }
        }
    }
}
