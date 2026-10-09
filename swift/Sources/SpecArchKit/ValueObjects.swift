// Value objects, under schemas: data passed around but not stored, with no
// key and no table, named as OpenAPI's components.schemas names them. An
// entity's field may hold one: in columns of the entity's table, one per
// part, or as one JSON value (ADR-063).

extension Checker {
    /// Checks that a schema shares no name with an entity or a view, that its
    /// required list names its own properties, and that every entity field
    /// holding one can be stored as its storage says (a relation to one is
    /// refused where the relation is checked).
    func checkValueObjects(_ d: Design) {
        for s in pairs(d.root.child("schemas")) {
            let name = s.key.value
            for (kind, names) in [("an entity", d.entities), ("a view", d.views)] where names[name] != nil {
                add(s.key, pointer("schemas", name), .valueObject, "\(name) is the name of \(kind) too; a schema, an entity and a view are one namespace, since each becomes a schema of its own in the interface; rename the schema")
            }
            checkFieldList(s.value.child("required"), ["schemas", name, "required"], fieldsOf(s.value), name, "required")
            for p in pairs(s.value.child("properties")) {
                checkField(p.value, ["schemas", name, "properties", p.key.value])
            }
        }
        for e in pairs(d.root.child("entities")) {
            let entity = e.key.value
            let required = Set(items(e.value.child("required")).map { str($0) })
            for f in pairs(e.value.child("properties")) {
                let path = ["entities", entity, "properties", f.key.value]
                checkStorageOutside(f.value, path)
                checkValueObjectField(d, entity, f, path, !required.contains(f.key.value) || nullable(f.value))
            }
        }
    }

    /// Refuses storage on every field inside n, a field of an entity:
    /// storage says how an entity's own field is stored, and a value inside
    /// another one is stored with it.
    func checkStorageOutside(_ n: YNode?, _ path: [String]) {
        for p in pairs(child(n, "properties")) {
            checkField(p.value, path + ["properties", p.key.value])
        }
        if let items = child(n, "items") {
            checkField(items, path + ["items"])
        }
    }

    /// Refuses storage on a field that is not an entity's own, and on every
    /// field inside it.
    func checkField(_ n: YNode, _ path: [String]) {
        if let s = n.child("storage") {
            add(s, pointer(path + ["storage"]), .valueObject, "storage says how an entity's field holding a schema is kept, and this value is stored with what holds it; leave storage out here")
        }
        checkStorageOutside(n, path)
    }

    /// Checks one field of an entity: storage only on a field that holds a
    /// schema, a list of schemas only as json, and, in columns, a schema
    /// whose every part has a column and whose absence can be told from a
    /// value with every part empty.
    func checkValueObjectField(_ d: Design, _ entity: String, _ f: Pair, _ path: [String], _ optional: Bool) {
        let field = f.key.value
        let storage = f.value.child("storage")
        let (name, list) = valueObjectOf(f.value)
        if name.isEmpty {
            if let storage {
                add(storage, pointer(path + ["storage"]), .valueObject, "\(field) holds no schema, and storage says how a field holding one is kept; leave storage out")
            }
            return
        }
        guard let schema = d.schemas[name] else { return } // the reference is refused as ref_type
        let kind = str(storage)
        if list {
            if kind == "columns" {
                add(storage, pointer(path + ["storage"]), .valueObject, "\(field) is a list of \(name), and a list has no columns in \(entity)'s row; store it as json, or make \(name) an entity related to \(entity)")
            }
            return
        }
        if kind == "json" { return }
        checkColumns(d, f.key, path, field, name, schema, optional, [name])
    }

    /// Checks a schema stored in columns of its own, at the field that holds
    /// it: no list of schemas and no cycle, since neither has a fixed set of
    /// columns, no reference to an entity, which is a relation and not a
    /// part, and no optional schema without a required property. A schema
    /// is optional where it is not required of what holds it.
    func checkColumns(_ d: Design, _ at: YNode, _ path: [String], _ field: String, _ name: String, _ schema: YNode, _ optional: Bool, _ seen: [String]) {
        let ptr = pointer(path)
        let top = String(field.split(separator: ".", maxSplits: 1, omittingEmptySubsequences: false)[0])
        if optional && !alwaysSet(d, schema, [name]) {
            add(at, ptr, .valueObject, "\(field) is optional and \(name) has no required part that is never null, so a row without it and one where every part of it is empty are the same; make a part of \(name) required and not nullable, make \(field) required, or store it as json")
        }
        let required = Set(items(schema.child("required")).map { str($0) })
        for p in pairs(schema.child("properties")) {
            let part = field + "." + p.key.value
            let (inner, list) = valueObjectOf(p.value)
            var entityRef = str(p.value.child("$ref"))
            if !entityRef.hasPrefix("#/entities/") { entityRef = str(child(p.value.child("items"), "$ref")) }
            if entityRef.hasPrefix("#/entities/") {
                let target = String(entityRef.dropFirst("#/entities/".count))
                add(at, ptr, .valueObject, "\(part) refers to \(target), a record of its own, and a value kept in columns holds no relation; relate \(path[1]) to \(target), or store \(top) as json")
            } else if inner.isEmpty {
            } else if list {
                add(at, ptr, .valueObject, "\(part) is a list of \(inner), and a list has no columns of its own; store \(top) as json")
            } else if seen.contains(inner) {
                add(at, ptr, .valueObject, "\(part) is \(inner) again, inside \(inner), so its columns never end; store \(top) as json")
            } else if let next = d.schemas[inner] {
                checkColumns(d, at, path, part, inner, next, !required.contains(p.key.value) || nullable(p.value), seen + [inner])
            }
        }
    }
}

/// The schema a field holds, and whether it holds a list of them:
/// '#/schemas/Name' itself, or as the items of an array.
func valueObjectOf(_ field: YNode?) -> (String, Bool) {
    let ref = str(child(field, "$ref"))
    if ref.hasPrefix("#/schemas/") { return (String(ref.dropFirst("#/schemas/".count)), false) }
    if typeIs(field, "array") {
        let items = str(child(child(field, "items"), "$ref"))
        if items.hasPrefix("#/schemas/") { return (String(items.dropFirst("#/schemas/".count)), true) }
    }
    return ("", false)
}

/// Whether a field's type is t, alone or beside null.
func typeIs(_ field: YNode?, _ t: String) -> Bool {
    str(child(field, "type")) == t || items(child(field, "type")).contains { str($0) == t }
}

/// Whether a schema kept in columns has a part whose column is set whenever
/// the value is: a required part that is not nullable, a scalar or a schema
/// that has one in its turn.
func alwaysSet(_ d: Design, _ schema: YNode, _ seen: [String]) -> Bool {
    let required = Set(items(schema.child("required")).map { str($0) })
    for p in pairs(schema.child("properties")) {
        if !required.contains(p.key.value) || nullable(p.value) { continue }
        let (inner, list) = valueObjectOf(p.value)
        guard !inner.isEmpty, !list, let next = d.schemas[inner] else { return true }
        if !seen.contains(inner) && alwaysSet(d, next, seen + [inner]) { return true }
    }
    return false
}

/// Whether a field's type list allows null.
func nullable(_ field: YNode?) -> Bool {
    items(child(field, "type")).contains { str($0) == "null" }
}
