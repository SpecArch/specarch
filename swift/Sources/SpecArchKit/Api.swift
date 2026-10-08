import Foundation

// The keywords about an operation's interface from the dxlib study
// (docs/dxlib-lessons.md, section 2): a list's whitelists and page size,
// the limits a client keeps to, and the catalogue of problem types.

extension Checker {
    /// Checks that a list names an entity and its fields, that no encrypted
    /// field is searched or sorted, or filtered without a hash, and that the
    /// default page fits the maximum.
    func checkListOf(_ d: Design, _ o: Operation) {
        guard let l = o.node.child("listOf") else { return }
        var entNode = l.child("entity")
        let fields: [String: YNode]
        if let viewNode = l.child("view") {
            guard let v = d.views[viewNode.value] else {
                add(viewNode, o.pointer("listOf", "view"), .listOf, "\(viewNode.value) is not a view of the specification\(suggest(viewNode.value, d.views))")
                return
            }
            entNode = viewNode
            fields = d.viewFields(v)
        } else {
            guard let e = d.entities[str(entNode)] else {
                if let entNode {
                    add(entNode, o.pointer("listOf", "entity"), .listOf, "\(entNode.value) is not an entity of the specification\(suggest(entNode.value, d.entities))")
                }
                return
            }
            fields = fieldsOf(e)
        }
        for list in ["searchable", "filterable", "sortable"] {
            for (i, item) in items(l.child(list)).enumerated() {
                let ptr = o.pointer("listOf", list, "\(i)")
                guard let f = fields[item.value] else {
                    add(item, ptr, .listOf, "\(item.value) is not a field of \(str(entNode)), so it cannot be \(list)\(suggest(item.value, fields))")
                    continue
                }
                guard str(f.child("atRest")) == "encrypted" else { continue }
                if list != "filterable" {
                    add(item, ptr, .listOf, "\(item.value) is encrypted at rest, so it cannot be \(list): storage cannot read it; leave it out of \(list)")
                } else if str(f.child("lookup")) != "hash" {
                    add(item, ptr, .listOf, "\(item.value) is encrypted at rest, so it can be filtered by only through a hash of it; add lookup: hash to the field, or leave it out of filterable")
                }
            }
        }
        let size = l.child("pageSize")
        if let def = size?.child("default"), let max = size?.child("maximum") {
            let dv = Int(def.value) ?? 0, mv = Int(max.value) ?? 0
            if dv > mv {
                add(def, o.pointer("listOf", "pageSize", "default"), .listOf, "the default page of \(dv) is above the maximum of \(mv); lower the default, or raise the maximum")
            }
        }
    }

    /// Refuses a rate over no time, and a burst below the rate.
    func checkLimits(_ o: Operation) {
        guard let rate = o.node.child("limits")?.child("rate") else { return }
        checkTimeout(rate.child("per"), o.pointer("limits", "rate", "per"), .limits, "the rate")
        if let burst = rate.child("burst"), let requests = rate.child("requests") {
            let b = Int(burst.value) ?? 0, r = Int(requests.value) ?? 0
            if b < r {
                add(burst, o.pointer("limits", "rate", "burst"), .limits, "a burst of \(b) is below the \(r) requests the rate allows, so it would lower the limit; raise it to at least \(r), or remove it")
            }
        }
    }

    /// Checks every response's problem type against the catalogue, and that
    /// a specification with a catalogue names a type on every 4xx and 5xx
    /// response.
    func checkProblems(_ d: Design, _ o: Operation) {
        let errors = d.root.child("errors")
        for r in pairs(o.node.child("responses")) {
            let code = r.key.value
            guard let p = r.value.child("problem") else {
                if !pairs(errors).isEmpty && code.count == 3 && (code.first == "4" || code.first == "5") {
                    add(r.key, o.pointer("responses", code), .problem, "the specification declares its problem types under errors, so the \(code) response of \(o.id) names one under problem")
                }
                continue
            }
            let ptr = o.pointer("responses", code, "problem")
            guard let t = errors?.child(p.value) else {
                var valid: [String: YNode] = [:]
                for e in pairs(errors) { valid[e.key.value] = e.value }
                add(p, ptr, .problem, "\(p.value) is not a problem type under errors\(suggest(p.value, valid))")
                continue
            }
            let status = str(t.child("status"))
            if status != code {
                add(p, ptr, .problem, "\(p.value) is a \(status) problem, and this is the \(code) response; name a problem type of status \(code)")
            }
        }
    }
}
