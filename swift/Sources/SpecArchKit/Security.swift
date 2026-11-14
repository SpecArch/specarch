import Foundation

// The design security rules (ADR-247): weaknesses a specification shows
// before any code is written. Each is a warning at the element to change,
// so the specification is still read whole, and each rule's name is
// stable, since a tool that traces a finding into the code cites it.

/// The design security rules, in the order of the design's Rule enum.
let securityRules: [Rule] = [
    .objectUnscoped, .internalKeyExposed, .refusalRevealsRecord, .queryFromCaller,
    .dutiesUnseparated, .secretUnprotected, .auditNotAppendOnly,
]

let controlSeparationOfDuties = "separationOfDuties"
let controlAuditTrail = "auditTrail"

/// Whether every caller holds a permission: public, and signedIn once
/// signed in, which no role grants.
func everyoneHolds(_ perm: String) -> Bool {
    perm == "public" || perm == permissionSignedIn
}

/// One parameter of an operation, the path item's or its own, with its
/// pointer.
struct SecurityParam {
    let node: YNode
    let ptr: String
}

extension Operation {
    /// The operation's parameters: the path item's first, then its own.
    func params() -> [SecurityParam] {
        var out: [SecurityParam] = []
        for (i, p) in items(pathItem.child("parameters")).enumerated() {
            out.append(SecurityParam(node: p, ptr: SpecArchKit.pointer("paths", path, "parameters", "\(i)")))
        }
        for (i, p) in items(node.child("parameters")).enumerated() {
            out.append(SecurityParam(node: p, ptr: pointer("parameters", "\(i)")))
        }
        return out
    }
}

/// Whether a field holds a whole number: an integer, or a string of format
/// int64 or uint64.
func isWholeNumber(_ f: YNode?) -> Bool {
    guard let f else { return false }
    if carriesAs(f, "integer") { return true }
    let format = str(f.child("format"))
    return carriesAs(f, "string") && (format == "int64" || format == "uint64")
}

/// Whether a name names an identifier: id, or a name that ends in Id, ID
/// or _id.
func identifierName(_ name: String) -> Bool {
    name == "id" || name == "ID" || name.hasSuffix("Id") || name.hasSuffix("ID") || name.hasSuffix("_id")
}

/// The names of a parameter or body property that takes part of a query
/// from the caller, compared with case, underscores and hyphens ignored.
let securitySortNames: Set<String> = ["sort", "sortby", "sortfield", "sortorder", "order", "orderby", "ordering"]
let securityFilterNames: Set<String> = ["filter", "filters", "where", "condition", "conditions", "criteria", "having", "groupby", "select", "fields", "columns"]
let securityRawNames: Set<String> = ["sql", "rawsql", "rawquery", "sqlquery", "statement"]

/// Whether a schema allows only values the specification lists: an enum or
/// a const, a $ref to a declared shape, a boolean or a number, an array of
/// such, or an object with its properties and additionalProperties false.
func closedSchema(_ s: YNode?) -> Bool {
    guard let s else { return false }
    if s.child("enum") != nil || s.child("const") != nil || !str(s.child("$ref")).isEmpty { return true }
    if let it = s.child("items") { return closedSchema(it) }
    if carriesAs(s, "boolean") || carriesAs(s, "integer") || carriesAs(s, "number") { return true }
    if carriesAs(s, "object") {
        return !pairs(s.child("properties")).isEmpty && str(s.child("additionalProperties")) == "false"
    }
    return false
}

/// Whether an element lists a requirement under satisfies.
func satisfiedBy(_ n: YNode?, _ id: String) -> Bool {
    items(child(n, "satisfies")).contains { $0.value == id }
}

/// The endings of a field name that name a secret, compared with case,
/// underscores and hyphens ignored.
let credentialNames = ["password", "passphrase", "secret", "apikey", "privatekey", "accesstoken", "refreshtoken"]

func credentialName(_ name: String) -> Bool {
    let k = keyName(name)
    return credentialNames.contains { k.hasSuffix($0) }
}

extension Design {
    /// The entity an operation reaches one record of by its key in the
    /// path: the one a path parameter <entity>Id is the key of, or, for a
    /// path parameter named id, the entity the operation reads or writes;
    /// "" for an operation with no such parameter.
    func addressedEntity(_ o: Operation) -> String {
        let e = keyEntity(o)
        if !e.isEmpty { return e }
        for p in o.params() where str(p.node.child("in")) == "path" && keyName(str(p.node.child("name"))) == "id" {
            return rowsEntity(o)
        }
        return ""
    }

    /// Each entity field that is a whole-number primary key, by identity,
    /// to its name, Entity.field.
    func internalKeys() -> [ObjectIdentifier: String] {
        var out: [ObjectIdentifier: String] = [:]
        for (name, e) in entities {
            let fields = fieldsOf(e)
            for k in items(e.child("primaryKey")) {
                if let f = fields[k.value], isWholeNumber(f) {
                    out[ObjectIdentifier(f)] = name + "." + k.value
                }
            }
        }
        return out
    }

    /// The controls entries asking for a control, by requirement ID.
    func controlsOf(_ name: String) -> [(id: String, node: YNode, ptr: String)] {
        var out: [(id: String, node: YNode, ptr: String)] = []
        for id in requirements.keys.sorted(by: byteLess) {
            for (i, ctl) in items(requirements[id]?.child("controls")).enumerated() where ctl.value == name {
                out.append((id, ctl, SpecArchKit.pointer("requirements", id, "controls", "\(i)")))
            }
        }
        return out
    }
}

extension Checker {
    /// Runs the design security rules.
    func checkSecurity(_ d: Design) {
        let keys = d.internalKeys()
        var seen = Set<String>()
        for o in d.opList {
            checkObjectScope(d, o)
            checkInternalKeys(d, o, keys, &seen)
            checkCallerQuery(o, &seen)
        }
        checkDutiesSeparated(d)
        checkSecrets(d)
        checkAuditTrail(d)
    }

    /// Warns about an operation that reaches one record by its key with a
    /// permission every caller holds and no row policy on the entity
    /// (object_unscoped), and about a 403 on such an operation, which can
    /// only refuse that record (refusal_reveals_record). A declared 404 is
    /// not asked for: a key no record has, or one outside a row policy, is
    /// derived as refused as not found whether or not the operation lists
    /// it.
    func checkObjectScope(_ d: Design, _ o: Operation) {
        let entity = d.addressedEntity(o)
        guard !entity.isEmpty, d.entities[entity] != nil else { return }
        let permNode = o.node.child("permission")
        let perm = str(permNode)
        let held = perm == "public" ? "every caller, signed in or not" : "every caller once signed in"
        // A public read is open to everyone on purpose; what it shows is
        // sensitivity_exposed's to judge.
        if everyoneHolds(perm) && !(perm == "public" && o.method == "get") && d.rowsOf(entity).isEmpty {
            warn(permNode, o.pointer("permission"), .objectUnscoped,
                 "\(o.id) reaches one \(entity) by its key, and its permission \(perm) is held by \(held), with no row policy on \(entity), so any caller reaches any \(entity) by changing the key; give the operation a permission a role grants, or give the roles a row policy on \(entity) that compares a field of it with caller.<property>")
        }
        let responses = o.node.child("responses")
        if everyoneHolds(perm), let r = responses?.key("403") {
            warn(r, o.pointer("responses", "403"), .refusalRevealsRecord,
                 "\(o.id) reaches one \(entity) by its key, and its permission \(perm) is held by \(held), so a 403 can only refuse that record, which tells the caller that a \(entity) with that key exists; answer 404, as for a key no record has")
        }
    }

    /// Warns about a whole-number key in an operation's URL, request body or
    /// response (internal_key_exposed): an entity's whole-number primary key,
    /// or a whole number named as an identifier.
    func checkInternalKeys(_ d: Design, _ o: Operation, _ keys: [ObjectIdentifier: String], _ seen: inout Set<String>) {
        for p in o.params() {
            let name = str(p.node.child("name"))
            let inside = str(p.node.child("in"))
            guard inside == "path" || inside == "query", identifierName(name), isWholeNumber(p.node.child("schema")) else { continue }
            let at = Rule.internalKeyExposed.rawValue + p.ptr
            if !seen.contains(at) {
                seen.insert(at)
                warn(p.node, p.ptr, .internalKeyExposed,
                     "\(o.id) carries \(name) in the URL as a whole number, which a caller counts up or down to reach records nobody gave it; address the record by a public identifier instead, a field of format uuid with generatedDefault: identifier, and keep the number inside the system")
            }
        }
        func carried(_ contents: YNode?) -> [String] {
            var set = Set<String>()
            for ct in pairs(contents) {
                for f in d.responseFields(ct.value.child("schema")) {
                    let last = f.name.split(separator: ".", omittingEmptySubsequences: false).last.map(String.init) ?? f.name
                    if keys[ObjectIdentifier(f.node)] != nil || identifierName(last) && isWholeNumber(f.node) {
                        set.insert(f.name)
                    }
                }
            }
            return set.sorted(by: byteLess)
        }
        let advice = "a whole number a caller counts up or down to reach records nobody gave it; carry a public identifier instead, a field of format uuid with generatedDefault: identifier, and keep the number inside the system"
        let names = carried(o.node.child("requestBody")?.child("content"))
        if !names.isEmpty {
            warn(o.node.key("requestBody"), o.pointer("requestBody"), .internalKeyExposed,
                 "the request body of \(o.id) carries \(joinAnd(names)), \(advice)")
        }
        for r in pairs(o.node.child("responses")) {
            let names = carried(r.value.child("content"))
            if !names.isEmpty {
                warn(r.key, o.pointer("responses", r.key.value), .internalKeyExposed,
                     "the \(r.key.value) response of \(o.id) carries \(joinAnd(names)), \(advice)")
            }
        }
    }

    /// Warns about a parameter that takes the field to sort by, a filter,
    /// the fields to return or a query from the caller with no list of the
    /// values it allows, and about a request body property that takes a
    /// query (query_from_caller). A body property named order, condition or
    /// fields is as often a value of the record as part of a query, so only
    /// a query's own names are read in a body.
    func checkCallerQuery(_ o: Operation, _ seen: inout Set<String>) {
        func check(_ n: YNode, _ ptr: String, _ name: String, _ schema: YNode?, _ place: String, _ rawOnly: Bool) {
            let k = keyName(name)
            let advice: String
            if rawOnly && !securityRawNames.contains(k) {
                return
            } else if securityRawNames.contains(k) {
                advice = "a query the caller writes runs with the service's rights over every table; name what a caller may ask for as operations of their own, each with its parameters"
            } else if securitySortNames.contains(k) {
                advice = "the query is built from a field name the caller chose; list the fields under listOf.sortable, which gives the parameter its enum, or give it an enum"
            } else if securityFilterNames.contains(k) {
                advice = "the query is built from field names and conditions the caller chose; list the fields under listOf.filterable, or give it a closed schema, an object with its properties and additionalProperties: false"
            } else {
                return
            }
            if closedSchema(schema) { return }
            let at = Rule.queryFromCaller.rawValue + ptr
            if !seen.contains(at) {
                seen.insert(at)
                warn(n, ptr, .queryFromCaller, "\(o.id) takes \(place) \(name) from the caller with no list of the values it allows, so \(advice)")
            }
        }
        for p in o.params() {
            check(p.node, p.ptr, str(p.node.child("name")), p.node.child("schema"), "the parameter", false)
        }
        for ct in pairs(o.node.child("requestBody")?.child("content")) {
            let schema = ct.value.child("schema")
            for p in pairs(schema?.child("properties")) {
                check(p.key, o.pointer("requestBody", "content", ct.key.value, "schema", "properties", p.key.value), p.key.value, p.value, "the request body property", true)
            }
        }
    }

    /// Warns about a requirement asking for separation of duties that no set
    /// satisfies, and a permission that satisfies it and no set satisfying
    /// it names (duties_unseparated). A workflow's approval is left alone:
    /// its derived case already refuses a requester who approves their own
    /// request.
    func checkDutiesSeparated(_ d: Design) {
        let sets = pairs(d.root.child("separationOfDuties"))
        for ctl in d.controlsOf(controlSeparationOfDuties) {
            let id = ctl.id
            var named = Set<String>()
            var found = false
            for s in sets where satisfiedBy(s.value, id) {
                found = true
                for p in items(s.value.child("permissions")) { named.insert(p.value) }
            }
            if !found {
                warn(ctl.node, ctl.ptr, .dutiesUnseparated,
                     "\(id) asks for separation of duties, and no set under separationOfDuties satisfies it, so nothing keeps one person from holding the permissions it would separate; add a set naming them, with satisfies: [\(id)]")
                continue
            }
            for p in pairs(d.root.child("permissions")) where satisfiedBy(p.value, id) && !named.contains(p.key.value) {
                warn(p.key, SpecArchKit.pointer("permissions", p.key.value), .dutiesUnseparated,
                     "\(p.key.value) satisfies \(id), which asks for separation of duties, and no set satisfying \(id) names it, so a role may grant it beside the permissions it should be kept from; name it in the set")
            }
        }
    }

    /// Warns about an entity's credential field, or a field named as one
    /// with no sensitivity, kept with no atRest, and about a secret setting
    /// with no rotation (secret_unprotected).
    func checkSecrets(_ d: Design) {
        for e in pairs(d.root.child("entities")) {
            for f in pairs(e.value.child("properties")) {
                let sens = str(f.value.child("sensitivity"))
                if sens != "credential" && (!sens.isEmpty || !credentialName(f.key.value)) || f.value.child("atRest") != nil { continue }
                let mark = sens.isEmpty ? ", and mark it sensitivity: credential" : ""
                warn(f.key, SpecArchKit.pointer("entities", e.key.value, "properties", f.key.value), .secretUnprotected,
                     "\(e.key.value).\(f.key.value) is a credential kept as it is, so whoever reads the table or a backup of it reads the value; add atRest: hashed for a value the system only compares with what a caller sends, such as a password, or atRest: encrypted for one it must send on, such as a key to another system\(mark)")
            }
        }
        for s in pairs(d.root.child("configuration")) where str(s.value.child("secret")) == "true" && s.value.child("rotation") == nil {
            warn(s.value.key("secret"), SpecArchKit.pointer("configuration", s.key.value, "secret"), .secretUnprotected,
                 "setting \(s.key.value) is a secret with no rotation, so a value that leaks stays good until someone notices; add rotation, the longest time one value is used before it is replaced, such as P90D")
        }
    }

    /// Warns about a requirement asking for an audit trail that no entity or
    /// securityEvents satisfies, an entity that satisfies it and is not
    /// appendOnly, and an append-only entity that an operation changes or
    /// deletes, or that has soft deletion (audit_not_append_only).
    func checkAuditTrail(_ d: Design) {
        let entities = pairs(d.root.child("entities"))
        for ctl in d.controlsOf(controlAuditTrail) {
            let id = ctl.id
            var kept = satisfiedBy(d.root.child("securityEvents"), id)
            for e in entities where satisfiedBy(e.value, id) {
                kept = true
                if str(e.value.child("appendOnly")) != "true" {
                    warn(e.key, SpecArchKit.pointer("entities", e.key.value), .auditNotAppendOnly,
                         "\(e.key.value) keeps the audit trail \(id) asks for, and nothing stops a record of it from being changed or deleted; add appendOnly: true")
                }
            }
            if !kept {
                warn(ctl.node, ctl.ptr, .auditNotAppendOnly,
                     "\(id) asks for an audit trail, and no entity or securityEvents satisfies it, so nothing in the design keeps one; name the entity that keeps it, with appendOnly: true and satisfies: [\(id)]")
            }
        }
        for e in entities where str(e.value.child("appendOnly")) == "true" {
            let name = e.key.value
            if str(e.value.child("deletion")) == "soft" {
                warn(e.value.key("deletion"), SpecArchKit.pointer("entities", name, "deletion"), .auditNotAppendOnly,
                     "\(name) is append-only, and soft deletion changes a record to mark it deleted; remove deletion")
            }
            for o in d.opList {
                let changes = o.method == "put" || o.method == "patch" || o.method == "delete"
                if changes && d.rowsEntity(o) == name || str(o.node.child("guard")?.child("entity")) == name {
                    warn(o.pathItem.key(o.method), o.pointer(), .auditNotAppendOnly,
                         "\(name) is append-only, and \(o.id) changes or deletes a record of it; record the change as a new \(name), or leave appendOnly out")
                }
            }
        }
    }
}
