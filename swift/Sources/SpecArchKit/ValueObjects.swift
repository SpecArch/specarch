// Value objects, under schemas: data passed around but not stored, with no
// key and no table, named as OpenAPI's components.schemas names them.

extension Checker {
    /// Checks that a schema shares no name with an entity or a view, that its
    /// required list names its own properties, and that no entity holds one
    /// in a field (a relation to one is refused where the relation is
    /// checked).
    func checkValueObjects(_ d: Design) {
        for s in pairs(d.root.child("schemas")) {
            let name = s.key.value
            for (kind, names) in [("an entity", d.entities), ("a view", d.views)] where names[name] != nil {
                add(s.key, pointer("schemas", name), .valueObject, "\(name) is the name of \(kind) too; a schema, an entity and a view are one namespace, since each becomes a schema of its own in the interface; rename the schema")
            }
            checkFieldList(s.value.child("required"), ["schemas", name, "required"], fieldsOf(s.value), name, "required")
        }
        for e in pairs(d.root.child("entities")) {
            walk(e.value.child("properties"), ["entities", e.key.value, "properties"]) { n, path in
                guard let ref = n.child("$ref"), ref.value.hasPrefix("#/schemas/") else { return }
                let name = String(ref.value.dropFirst("#/schemas/".count))
                let entity = e.key.value
                add(ref, pointer(path + ["$ref"]), .valueObject, "\(name) is a schema, which is passed around and never stored, and \(entity) is stored; hold the fields \(entity) needs in \(entity), or make \(name) an entity")
            }
        }
    }
}
