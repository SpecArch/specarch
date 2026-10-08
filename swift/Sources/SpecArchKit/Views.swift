import Foundation

// The read model of the dxlib study (docs/dxlib-lessons.md, section 2):
// views, an entity's row with fields read through its relations and
// counts of its related records added.

/// The relation kinds a path follows and a count counts.
let toOne: Set<String> = ["many-to-one", "one-to-one"]
let toMany: Set<String> = ["one-to-many", "many-to-many"]

/// The relations of an entity, by name.
func relationsOf(_ entity: YNode?) -> [String: YNode] {
    var m: [String: YNode] = [:]
    for p in pairs(entity?.child("relations")) { m[p.key.value] = p.value }
    return m
}

/// Says an entity has no relation of a name, with the close one or the ones
/// it has.
func noRelation(_ entity: String, _ name: String, _ rels: [String: YNode]) -> String {
    if rels.isEmpty { return entity + " has no relation " + name + "; it has no relations" }
    return entity + " has no relation " + name + suggest(name, rels)
}

extension Design {
    /// Follows a path from an entity: the field it ends in, or nil and the
    /// reason it does not resolve.
    func viewPath(_ entity: String, _ path: String) -> (YNode?, String) {
        let hops = path.split(separator: ".").map(String.init)
        var cur = entity
        for hop in hops.dropLast() {
            let rels = relationsOf(entities[cur])
            guard let r = rels[hop] else {
                return (nil, noRelation(cur, hop, rels))
            }
            let kind = str(r.child("kind"))
            if !toOne.contains(kind) {
                return (nil, cur + "." + hop + " is " + kind + ", which leads to many records; a path follows only many-to-one and one-to-one relations, so the view keeps one row per record; count it instead")
            }
            cur = str(r.child("target"))
            if entities[cur] == nil { return (nil, "") } // the relation's own check reports the target
        }
        let last = hops.last ?? ""
        let fields = fieldsOf(entities[cur])
        if let f = fields[last] { return (f, "") }
        return (nil, cur + " has no field " + last + suggest(last, fields))
    }

    /// The fields of a view: every field of its entity, and each property it
    /// adds, a path as the field it reads.
    func viewFields(_ view: YNode) -> [String: YNode] {
        let from = str(view.child("from"))
        var m = fieldsOf(entities[from])
        for p in pairs(view.child("properties")) {
            m[p.key.value] = p.value
            let path = str(p.value.child("path"))
            if !path.isEmpty, entities[from] != nil, let f = viewPath(from, path).0 {
                m[p.key.value] = f
            }
        }
        return m
    }
}

extension Checker {
    /// Checks that a view reads from an entity, that each path follows
    /// relations to one record and ends in a field, that each count counts a
    /// relation to many, that no property repeats a field of the entity, that
    /// no view shares an entity's name, and that no request body names a view.
    func checkViews(_ d: Design) {
        for v in pairs(d.root.child("views")) {
            let name = v.key.value
            if d.entities[name] != nil {
                add(v.key, pointer("views", name), .view, "\(name) is the name of an entity too; a view and an entity are one namespace, since each becomes a table or view and a schema of its own; rename the view")
            }
            let fromNode = v.value.child("from")
            let from = str(fromNode)
            guard d.entities[from] != nil else {
                if let fromNode {
                    add(fromNode, pointer("views", name, "from"), .view, "\(from) is not an entity of the specification\(suggest(from, d.entities))")
                }
                continue
            }
            let fields = fieldsOf(d.entities[from])
            for p in pairs(v.value.child("properties")) {
                let prop = p.key.value
                if fields[prop] != nil {
                    add(p.key, pointer("views", name, "properties", prop), .view, "\(prop) is a field of \(from) already, and the view carries every field of its entity; leave it out, or give the added field another name")
                }
                if let n = p.value.child("path") {
                    let why = d.viewPath(from, n.value).1
                    if !why.isEmpty {
                        add(n, pointer("views", name, "properties", prop, "path"), .view, "\(n.value) does not resolve: \(why)")
                    }
                }
                if let n = p.value.child("count") {
                    let rels = relationsOf(d.entities[from])
                    if let r = rels[n.value] {
                        let kind = str(r.child("kind"))
                        if !toMany.contains(kind) {
                            add(n, pointer("views", name, "properties", prop, "count"), .view, "\(from).\(n.value) is \(kind), which leads to one record; a count counts one-to-many and many-to-many relations; read it with a path instead")
                        }
                    } else {
                        add(n, pointer("views", name, "properties", prop, "count"), .view, noRelation(from, n.value, rels))
                    }
                }
            }
        }
        for o in d.opList {
            walk(o.node.child("requestBody"), []) { n, path in
                if let ref = n.child("$ref"), ref.value.hasPrefix("#/views/") {
                    add(ref, SpecArchKit.pointer(["paths", o.path, o.method, "requestBody"] + path + ["$ref"]), .view, "\(String(ref.value.dropFirst("#/views/".count))) is a view, and a view is never written; take the entity it reads from, or the fields the operation takes")
                }
            }
        }
    }
}
