import Foundation

// The keywords about stored data from the dxlib study
// (docs/dxlib-lessons.md, section 2): how sensitive a field is, whether it
// is encrypted at rest, and the audit fields and soft delete of an entity.

/// The fields the system sets on an audited entity.
let auditFields = ["createdAt", "createdBy", "lastModifiedAt", "lastModifiedBy"]

/// The flag the system sets on an entity with soft deletion.
let deletedField = "deleted"

/// A field a response can carry, named as a reader finds it.
struct NamedField {
    let name: String // Member.email, or email for an inline field
    let node: YNode
}

extension Checker {
    /// Checks an entity's audit fields, its soft delete and the keys of its
    /// encrypted fields.
    func checkStored(_ name: String, _ e: YNode, _ fields: [String: YNode]) {
        let props = e.child("properties")
        func reserved(_ field: String, _ why: String) {
            if let k = props?.key(field) {
                add(k, pointer("entities", name, "properties", field), .audited, "\(name) \(why), so the system sets \(field); remove it from the entity's properties")
            }
        }
        if str(e.child("audited")) == "true" {
            for f in auditFields { reserved(f, "is audited") }
        }
        if str(e.child("deletion")) == "soft" {
            reserved(deletedField, "has soft deletion")
        }
        func keys(_ list: YNode?, _ ptr: [String], _ what: String) {
            for (i, item) in items(list).enumerated() {
                let f = fields[item.value]
                if str(child(f, "atRest")) == "encrypted" && str(child(f, "lookup")) != "hash" {
                    add(item, pointer(ptr + ["\(i)"]), .atRest, "\(item.value) is encrypted at rest, so it cannot be \(what) unless it is looked up by a hash; add lookup: hash to it")
                }
            }
        }
        keys(e.child("primaryKey"), ["entities", name, "primaryKey"], "part of the primary key")
        for p in pairs(e.child("constraints")) where str(p.value.child("kind")) == "unique" {
            keys(p.value.child("fields"), ["entities", name, "constraints", p.key.value, "fields"], "part of a unique constraint")
        }
    }

    /// Refuses a lookup on a field that is not encrypted at rest.
    func checkLookups(_ d: Design) {
        walk(d.root, []) { n, path in
            if let l = n.child("lookup"), l.kind == .scalar, str(n.child("atRest")) != "encrypted" {
                add(l, pointer(path + ["lookup"]), .atRest, "lookup says how an encrypted field is found, and this field is not encrypted at rest; add atRest: encrypted, or remove lookup")
            }
        }
    }

    /// Reports a credential field that is not writeOnly in any response of
    /// an operation, and, as a warning, a personal field in a response of a
    /// public operation.
    func checkExposed(_ d: Design, _ o: Operation) {
        let isPublic = str(o.node.child("permission")) == "public"
        for r in pairs(o.node.child("responses")) {
            for ct in pairs(r.value.child("content")) {
                for f in d.responseFields(ct.value.child("schema")) {
                    switch str(f.node.child("sensitivity")) {
                    case "credential":
                        if str(f.node.child("writeOnly")) != "true" {
                            add(r.key, o.pointer("responses", r.key.value), .sensitivityExposed, "the \(r.key.value) response of \(o.id) can carry \(f.name), which is a credential; mark the field writeOnly, or leave it out of the response")
                        }
                    case "personal":
                        if isPublic {
                            warn(r.key, o.pointer("responses", r.key.value), .sensitivityExposed, "\(o.id) is public, and its \(r.key.value) response carries \(f.name), which is personal; give the operation a permission, or leave the field out")
                        }
                    default:
                        break
                    }
                }
            }
        }
    }
}

extension Design {
    /// Every field a schema can carry: through a $ref to an entity, a view
    /// or a schema of the specification, its items, and its properties, each
    /// entity, view and schema once.
    func responseFields(_ schema: YNode?) -> [NamedField] {
        var out: [NamedField] = []
        var seen = Set<String>()
        func visit(_ s: YNode?, _ owner: String) {
            guard let s else { return }
            let ref = str(s.child("$ref"))
            if ref.hasPrefix("#/entities/") {
                let name = String(ref.dropFirst("#/entities/".count))
                guard !seen.contains(name), let e = entities[name] else { return }
                seen.insert(name)
                visit(e, name)
                return
            }
            if ref.hasPrefix("#/schemas/") {
                let name = String(ref.dropFirst("#/schemas/".count))
                guard !seen.contains("$" + name), let v = schemas[name] else { return }
                seen.insert("$" + name)
                visit(v, name)
                return
            }
            if ref.hasPrefix("#/views/") {
                // A view carries its entity's fields and the fields its paths read,
                let name = String(ref.dropFirst("#/views/".count))
                guard !seen.contains("#" + name), let v = views[name] else { return }
                seen.insert("#" + name)
                let fields = viewFields(v)
                for k in fields.keys.sorted() {
                    out.append(NamedField(name: name + "." + k, node: fields[k]!))
                }
                // and the records of each relation it carries as rows.
                for target in rowsTargets(v) where !seen.contains(target) {
                    if let e = entities[target] {
                        seen.insert(target)
                        visit(e, target)
                    }
                }
                return
            }
            visit(s.child("items"), owner)
            for p in pairs(s.child("properties")) {
                out.append(NamedField(name: owner.isEmpty ? p.key.value : owner + "." + p.key.value, node: p.value))
                visit(p.value, "")
            }
        }
        visit(schema, "")
        return out
    }
}
