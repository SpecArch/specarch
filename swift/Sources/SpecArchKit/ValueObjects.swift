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
        let rv = ReferenceResolver(d, given)
        for e in pairs(d.root.child("entities")) {
            let entity = e.key.value
            let required = Set(items(e.value.child("required")).map { str($0) })
            for f in pairs(e.value.child("properties")) {
                let path = ["entities", entity, "properties", f.key.value]
                checkStorageOutside(f.value, path)
                checkValueObjectField(d, rv, entity, f, path, !required.contains(f.key.value) || nullable(f.value))
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
        if let values = child(n, "additionalProperties"), values.kind == .mapping {
            checkField(values, path + ["additionalProperties"])
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
    func checkValueObjectField(_ d: Design, _ rv: ReferenceResolver, _ entity: String, _ f: Pair, _ path: [String], _ optional: Bool) {
        let field = f.key.value
        let storage = f.value.child("storage")
        let (name, ownList) = valueObjectOf(f.value)
        var (ref, list) = ("#/schemas/" + name, ownList)
        if name.isEmpty {
            let (other, otherList) = foreignValueObjectOf(f.value)
            if other.isEmpty {
                if let storage {
                    add(storage, pointer(path + ["storage"]), .valueObject, "\(field) holds no schema, and storage says how a field holding one is kept; leave storage out")
                }
                return
            }
            // A schema of another specification is written as this one's
            // own (ADR-184): a list is json only, and one value in columns is
            // checked as one of this specification's schemas, in the module
            // that declares it.
            if otherList {
                if let storage, str(storage) == "columns" {
                    add(storage, pointer(path + ["storage"]), .valueObject, "\(field) is a list of \(other), of another specification, and a list has no columns in \(entity)'s row; store it as json")
                }
                return
            }
            (ref, list) = (other, false)
        }
        if str(storage) == "json" && !list { return }
        let cc = ColumnsCheck(self, rv, specName(d.root), f.key, path)
        guard let held = cc.resolve(d, ref) else {
            cc.report(field)
            return // the reference is refused as ref_type or reference
        }
        if list {
            if str(storage) == "columns" {
                add(storage, pointer(path + ["storage"]), .valueObject, "\(field) is a list of \(name), and a list has no columns in \(entity)'s row; store it as json, or make \(name) an entity related to \(entity)")
            }
            return
        }
        cc.columns(field, held, optional, [held.key])
        cc.report(field)
    }
}

/// A schema a value kept in columns holds, with the design and the module
/// that declare it.
struct HeldSchema {
    let d: Design
    let module: String
    let name: String
    let node: YNode
    var key: String { module + "#/schemas/" + name }
}

/// Checks one entity field kept in columns, reading each reference in the
/// module that declares it, and keeps the modules the run does not give,
/// whose parts it cannot check.
final class ColumnsCheck {
    let c: Checker
    let rv: ReferenceResolver
    let own: String // the name of the specification checked
    let at: YNode
    let path: [String]
    var unchecked: [String] = [] // the schemas not checked, as label()
    var notGiven: [String] = [] // the modules that declare them

    init(_ c: Checker, _ rv: ReferenceResolver, _ own: String, _ at: YNode, _ path: [String]) {
        self.c = c
        self.rv = rv
        self.own = own
        self.at = at
        self.path = path
    }

    /// Names a schema or an entity of a module: by its name in the
    /// specification checked, and with its module's name in another.
    func label(_ module: String, _ name: String) -> String {
        module == own ? name : name + " of " + module
    }

    /// The schema a reference made in design d names, '#/schemas/Name' or
    /// '<dependency>#/schemas/Name'; nil when it names none, or a module the
    /// run does not give, which is kept for report.
    func resolve(_ d: Design, _ ref: String) -> HeldSchema? {
        let module = specName(d.root)
        if ref.hasPrefix("#/schemas/") {
            let name = String(ref.dropFirst("#/schemas/".count))
            guard let node = d.schemas[name] else { return nil }
            return HeldSchema(d: d, module: module, name: name, node: node)
        }
        guard let (dep, ptr) = crossRef(ref), ptr.hasPrefix("/schemas/") else { return nil }
        let name = String(ptr.dropFirst("/schemas/".count))
        let spec = d.specificationOf(dep)
        if spec.isEmpty { return nil } // a reference error says why
        let (found, problem) = rv.otherOf(spec)
        if !problem.isEmpty { return nil }
        guard let o = found else {
            if (rv.given[spec] ?? []).isEmpty {
                let l = label(spec, name)
                if !unchecked.contains(l) { unchecked.append(l) }
                if !notGiven.contains(spec) { notGiven.append(spec) }
            }
            return nil // given and not read: its own errors say why
        }
        if spec == own {
            guard let node = rv.d.schemas[name] else { return nil }
            return HeldSchema(d: rv.d, module: own, name: name, node: node)
        }
        guard let node = o.design.schemas[name] else { return nil }
        return HeldSchema(d: o.design, module: spec, name: name, node: node)
    }

    /// The schema a part of a value holds, read in the design that declares
    /// the value, and whether it holds a list of them; nil when the part
    /// holds no schema it can read.
    func partSchema(_ d: Design, _ part: YNode) -> (HeldSchema, Bool)? {
        var ref = str(part.child("$ref"))
        var list = false
        if ref.isEmpty && typeIs(part, "array") {
            (ref, list) = (str(child(part.child("items"), "$ref")), true)
        }
        if !ref.hasPrefix("#/schemas/") && !(crossRef(ref)?.pointer.hasPrefix("/schemas/") ?? false) { return nil }
        guard let held = resolve(d, ref) else { return nil }
        return (held, list)
    }

    /// The schema of another module a part of a value holds as a list,
    /// '<dependency>#/schemas/Name' as its items, read in the design that
    /// declares the value. A list has no columns whatever the schema holds,
    /// so the name comes from the reference and the module need not be given.
    func crossList(_ d: Design, _ part: YNode) -> String? {
        guard typeIs(part, "array"), part.child("$ref") == nil,
              let (dep, ptr) = crossRef(str(child(part.child("items"), "$ref"))),
              ptr.hasPrefix("/schemas/") else { return nil }
        let name = String(ptr.dropFirst("/schemas/".count))
        let spec = d.specificationOf(dep)
        return spec.isEmpty ? name + " of " + dep : label(spec, name)
    }

    /// Checks a schema stored in columns of its own, at the field that holds
    /// it: no list of schemas and no cycle, since neither has a fixed set of
    /// columns, no reference to an entity, which is a relation and not a
    /// part, and no optional schema without a required property. A schema
    /// is optional where it is not required of what holds it.
    func columns(_ field: String, _ held: HeldSchema, _ optional: Bool, _ seen: [String]) {
        let ptr = pointer(path)
        let top = String(field.split(separator: ".", maxSplits: 1, omittingEmptySubsequences: false)[0])
        let name = label(held.module, held.name)
        if optional && !alwaysSet(held, [held.key]) {
            c.add(at, ptr, .valueObject, "\(field) is optional and \(name) has no required part that is never null, so a row without it and one where every part of it is empty are the same; make a part of \(name) required and not nullable, make \(field) required, or store it as json")
        }
        let required = Set(items(held.node.child("required")).map { str($0) })
        for p in pairs(held.node.child("properties")) {
            let part = field + "." + p.key.value
            var entityRef = str(p.value.child("$ref"))
            if !entityRef.hasPrefix("#/entities/") { entityRef = str(child(p.value.child("items"), "$ref")) }
            if entityRef.hasPrefix("#/entities/") {
                let target = label(held.module, String(entityRef.dropFirst("#/entities/".count)))
                c.add(at, ptr, .valueObject, "\(part) refers to \(target), a record of its own, and a value kept in columns holds no relation; relate \(path[1]) to \(target), or store \(top) as json")
                continue
            }
            if let other = crossList(held.d, p.value) {
                c.add(at, ptr, .valueObject, "\(part) is a list of \(other), and a list has no columns of its own; store \(top) as json")
                continue
            }
            guard let (inner, list) = partSchema(held.d, p.value) else { continue }
            let innerName = label(inner.module, inner.name)
            if list {
                c.add(at, ptr, .valueObject, "\(part) is a list of \(innerName), and a list has no columns of its own; store \(top) as json")
            } else if seen.contains(inner.key) {
                c.add(at, ptr, .valueObject, "\(part) is \(innerName) again, inside \(innerName), so its columns never end; store \(top) as json")
            } else {
                columns(part, inner, !required.contains(p.key.value) || nullable(p.value), seen + [inner.key])
            }
        }
    }

    /// Whether a schema kept in columns has a part whose column is set
    /// whenever the value is: a required part that is not nullable, a scalar
    /// or a schema that has one in its turn. A part holding a schema of a
    /// module the run does not give counts as set, and is kept for report.
    func alwaysSet(_ held: HeldSchema, _ seen: [String]) -> Bool {
        let required = Set(items(held.node.child("required")).map { str($0) })
        for p in pairs(held.node.child("properties")) {
            if !required.contains(p.key.value) || nullable(p.value) { continue }
            if crossList(held.d, p.value) != nil { return true }
            guard let (inner, list) = partSchema(held.d, p.value), !list else { return true }
            if !seen.contains(inner.key) && alwaysSet(inner, seen + [inner.key]) { return true }
        }
        return false
    }

    /// Warns once at the field when the value reaches a schema of a module
    /// the run does not give, whose parts are therefore not checked.
    func report(_ field: String) {
        if notGiven.isEmpty { return }
        let modules = joinAnd(notGiven)
        c.warn(at, pointer(path), .valueObjectUnchecked, "\(field) is kept in columns and holds \(joinAnd(unchecked)), and the run does not give \(modules), so the rules on its parts are not checked: an optional value needs a required part that is never null, and no list, cycle or entity may be inside it; give \(modules) in the same run or through the estate, or store \(field) as json")
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

/// The schema of another specification a field holds, as
/// <dependency>#/schemas/<Name>, itself or as the items of an array, and
/// whether it holds a list of them.
func foreignValueObjectOf(_ field: YNode?) -> (String, Bool) {
    func isSchema(_ ref: String) -> Bool { crossRef(ref)?.pointer.hasPrefix("/schemas/") ?? false }
    let ref = str(child(field, "$ref"))
    if isSchema(ref) { return (ref, false) }
    if typeIs(field, "array") {
        let items = str(child(child(field, "items"), "$ref"))
        if isSchema(items) { return (items, true) }
    }
    return ("", false)
}

/// Whether a field's type is t, alone or beside null.
func typeIs(_ field: YNode?, _ t: String) -> Bool {
    str(child(field, "type")) == t || items(child(field, "type")).contains { str($0) == t }
}

/// Whether a field's type list allows null.
func nullable(_ field: YNode?) -> Bool {
    items(child(field, "type")).contains { str($0) == "null" }
}
