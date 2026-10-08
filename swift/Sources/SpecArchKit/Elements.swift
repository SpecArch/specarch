import Foundation

// The page elements of docs/conventions.md, Page elements: pickers, an
// action's when and reason, a field's conditions, and a form's checks and
// fields entered twice.

/// Whether a page has a record before anything is entered: a list's rows, a
/// view's record, or the record an edit form loads. A form without source
/// creates one.
func loadsRecord(_ pg: YNode) -> Bool {
    str(pg.child("kind")) != "form" || pg.child("source") != nil
}

/// The many-to-one relation of an entity whose via is the field, and its
/// target, or "" when there is none.
func pickerRelation(_ ent: YNode?, _ field: String) -> (relation: String, target: String) {
    for r in pairs(ent?.child("relations")) where str(r.value.child("kind")) == "many-to-one" && str(r.value.child("via")) == field {
        return (r.key.value, str(r.value.child("target")))
    }
    return ("", "")
}

extension Design {
    /// What an expression over a page's record may name: the entity's
    /// fields, or on a form that creates a record, the fields it shows.
    func recordEnv(_ pg: YNode, _ ent: YNode?, _ shown: [String: YNode]) -> ExprEnv {
        var env: ExprEnv = [:]
        for (n, f) in loadsRecord(pg) ? fieldsOf(ent) : shown { env[n] = fieldType(f) }
        return env
    }

    /// Whether two fields hold values of one type, null aside.
    func sameType(_ a: YNode, _ b: YNode) -> Bool {
        var ta = fieldType(a), tb = fieldType(b)
        ta.nullable = false
        tb.nullable = false
        return "\(ta)" == "\(tb)"
    }

    /// The cases of a page's elements: a picker that finds nothing, an
    /// action its when withholds, a confirmation without its reason, a check
    /// across fields broken in each way it can be, and a field entered twice
    /// differently.
    func elementCases(_ s: Subject, _ pg: YNode, _ open: String) {
        let ent = entities[str(pg.child("entity"))]
        // Only a form has pickers, checks and fields entered twice; on another
        // page they are errors, and give no case.
        let form = str(pg.child("kind")) == "form"
        let pickers = form ? pg.child("pickers") : nil, checks = form ? pg.child("checks") : nil, twice = form ? pg.child("enteredTwice") : nil
        for p in pairs(pickers) {
            let target = pickerRelation(ent, p.key.value).target
            if target.isEmpty { continue }
            s.red("picker " + p.key.value + " finds nothing", occasional, "no " + target + " matches what is typed", "a " + target + " is looked for to fill " + p.key.value, "it says nothing matches, and " + p.key.value + " stays empty")
        }
        for a in items(pg.child("actions")) {
            let label = str(a.child("label"))
            let w = str(a.child("when"))
            if !w.isEmpty && loadsRecord(pg) {
                s.red(label + " not offered", occasional, "a record for which " + w + " is false", open, "it does not offer " + label + " for that record")
            }
            let r = str(a.child("reason"))
            if !r.isEmpty && str(a.child("kind")) == "operation" {
                s.red(label + " without a reason", frequent, "...", "the action " + label + " is confirmed with no " + r, "it is not sent, and the " + r + " is asked for")
            }
        }
        for p in pairs(checks) {
            let name = p.key.value, msg = str(p.value.child("message"))
            let rules = falsifiers(str(p.value.child("expression")))
            if rules.count < 2 {
                s.red("violates " + name, frequent, "...", "the form is submitted breaking it", "it is not sent: " + msg)
                continue
            }
            for rule in rules {
                let falsehood = rule.joined(separator: " is false and ") + " is false"
                s.red("violates " + name + ": " + falsehood, frequent, "...", "the form is submitted with " + falsehood, "it is not sent: " + msg)
            }
        }
        for f in items(twice) {
            s.red(f.value + " entered twice differently", frequent, "...", "the form is submitted with two different entries of " + f.value, "it is not sent, and says the two entries differ")
        }
    }
}

extension Checker {
    /// Checks the page elements of every page.
    func checkPageElements(_ d: Design) {
        for p in pairs(d.root.child("pages")) {
            let name = p.key.value, pg = p.value
            let kind = str(pg.child("kind"))
            let ent = d.entities[str(pg.child("entity"))]
            let fields = fieldsOf(ent)
            var shown: [String: YNode] = [:]
            for f in pageFields(pg) { if let n = fields[f.value] { shown[f.value] = n } }
            checkPickers(d, name, pg, kind, ent, shown)
            checkActionElements(d, name, pg, ent)
            checkFieldConditions(d, name, pg, kind, ent, shown)
            checkFormChecks(d, name, pg, kind, shown)
        }
    }

    /// Checks an expression that decides something on a page: it parses,
    /// names only what env holds, and gives true or false.
    func checkCondition(_ n: YNode?, _ ptr: String, _ what: String, _ env: ExprEnv) {
        guard let n, n.kind == .scalar else { return }
        let (tree, perrs) = parseExpr(n.value)
        guard let tree, perrs.isEmpty else {
            exprErrors(n, ptr, what, perrs)
            return
        }
        let (t, errs) = checkExpr(tree, env)
        exprErrors(n, ptr, what, errs)
        if errs.isEmpty && (t.kind != .bool || t.nullable) {
            addFile(fileOf(n), exprLine(n, 1), ptr, .expressionType, "\(what) gives \(t), but it must give true or false; compare the values with ==, <, > or similar")
        }
    }

    /// Checks a form's pickers: each is for a field the form shows that a
    /// many-to-one relation holds, its source lists the relation's target, it
    /// shows and fills from fields of the target, and everyone who may open
    /// the form may read the list.
    func checkPickers(_ d: Design, _ name: String, _ pg: YNode, _ kind: String, _ ent: YNode?, _ shown: [String: YNode]) {
        guard let pickers = pg.child("pickers") else { return }
        let base = ["pages", name, "pickers"]
        if kind != "form" {
            add(pg.key("pickers"), pointer(base), .picker, "\(name) is a \(kind), and a picker is how a form fills a field; leave pickers out")
            return
        }
        let entName = str(pg.child("entity"))
        for p in pairs(pickers) {
            let field = p.key.value, pk = p.value
            let at = base + [field]
            if shown[field] == nil {
                add(p.key, pointer(at), .picker, "\(field) is not a field the form \(name) shows\(suggest(field, shown))")
                continue
            }
            let (relation, target) = pickerRelation(ent, field)
            if relation.isEmpty {
                add(p.key, pointer(at), .picker, "no many-to-one relation of \(entName) holds \(field), so there is no record to pick; add the relation with via: \(field), or leave the picker out")
                continue
            }
            guard let targetEnt = d.entities[target] else { continue } // the relation check reports it
            let targetFields = fieldsOf(targetEnt)
            if let srcNode = pk.child("source") {
                let src = srcNode.value
                if let o = d.operations[src] {
                    if str(o.node.child("listOf")?.child("entity")) != target {
                        add(srcNode, pointer(at + ["source"]), .picker, "\(src) does not list \(target), the target of the relation \(relation); name an operation whose listOf names \(target)")
                    } else {
                        checkPickerAccess(d, name, pg, srcNode, at, src, str(o.node.child("permission")))
                    }
                } else {
                    var opNames: [String: YNode] = [:]
                    for (id, op) in d.operations { opNames[id] = op.node }
                    add(srcNode, pointer(at + ["source"]), .picker, "\(src) is not an operationId of the specification\(suggest(src, opNames))")
                }
            }
            for (i, f) in items(pk.child("shows")).enumerated() where targetFields[f.value] == nil {
                add(f, pointer(at + ["shows", "\(i)"]), .picker, "\(f.value) is not a field of \(target), so the picker cannot show it\(suggest(f.value, targetFields))")
            }
            for kv in pairs(pk.child("fills")) {
                let fat = pointer(at + ["fills", kv.key.value])
                if kv.key.value == field || shown[kv.key.value] == nil {
                    add(kv.key, fat, .picker, "\(kv.key.value) is not another field the form \(name) shows, so the picker cannot fill it")
                } else if targetFields[kv.value.value] == nil {
                    add(kv.value, fat, .picker, "\(kv.value.value) is not a field of \(target), so the picker cannot fill from it\(suggest(kv.value.value, targetFields))")
                } else if let a = shown[kv.key.value], let b = targetFields[kv.value.value], !d.sameType(a, b) {
                    add(kv.value, fat, .picker, "\(kv.key.value) is \(d.fieldType(a)) and \(target).\(kv.value.value) is \(d.fieldType(b)); fill a field from one of its own type")
                }
            }
        }
    }

    /// Checks that every role that may open a page may also read the list
    /// its picker reads.
    func checkPickerAccess(_ d: Design, _ name: String, _ pg: YNode, _ srcNode: YNode, _ at: [String], _ src: String, _ need: String) {
        let perm = str(pg.child("permission"))
        if need.isEmpty || need == "public" || need == perm { return }
        for r in pairs(d.root.child("roles")) {
            let granted = Set(items(r.value.child("permissions")).map { $0.value })
            if (perm == "public" || granted.contains(perm)) && !granted.contains(need) {
                add(srcNode, pointer(at + ["source"]), .picker, "\(r.key.value) may open \(name) but not read \(src), which needs \(need); grant it to the role, or pick from a list the role may read")
            }
        }
    }

    /// Checks an action's when and reason: when is over a record the page
    /// has, and the reason is a string the operation's request body requires.
    func checkActionElements(_ d: Design, _ name: String, _ pg: YNode, _ ent: YNode?) {
        for (i, a) in items(pg.child("actions")).enumerated() {
            let at = ["pages", name, "actions", "\(i)"]
            let label = str(a.child("label"))
            if let w = a.child("when") {
                if !loadsRecord(pg) {
                    add(a.key("when"), pointer(at + ["when"]), .action, "\(name) creates a record and loads none, so the action \(label) has no record for when to test; leave when out")
                } else if ent != nil {
                    checkCondition(w, pointer(at + ["when"]), "when", d.recordEnv(pg, ent, [:]))
                }
            }
            guard let r = a.child("reason") else { continue }
            let ptr = pointer(at + ["reason"])
            if str(a.child("kind")) != "operation" {
                add(a.key("reason"), ptr, .action, "the action \(label) navigates, and a reason is sent with an operation; leave it out")
                continue
            }
            guard let o = d.operations[str(a.child("target"))] else { continue } // the page check reports it
            let (fields, required) = d.requestFields(o.node)
            if let f = fields[r.value] {
                if !required.contains(r.value) {
                    add(r, ptr, .action, "the request body of \(o.id) does not require \(r.value), and a reason asked for is always sent; add it to the body's required")
                } else if d.fieldType(f).kind != .string {
                    add(r, ptr, .action, "\(r.value) is \(d.fieldType(f)), and a reason is text a person types; make it a string")
                }
            } else {
                add(r, ptr, .action, "\(r.value) is not a property of the request body of \(o.id), so the reason cannot be sent\(suggest(r.value, fields))")
            }
        }
    }

    /// Checks when a page's fields are read-only or hidden: each is a field
    /// the page shows, read-only only on a form, and each condition an
    /// expression over the record.
    func checkFieldConditions(_ d: Design, _ name: String, _ pg: YNode, _ kind: String, _ ent: YNode?, _ shown: [String: YNode]) {
        guard let conds = pg.child("fieldConditions") else { return }
        let base = ["pages", name, "fieldConditions"]
        if kind != "form" && kind != "view" {
            add(pg.key("fieldConditions"), pointer(base), .formField, "\(name) is a \(kind), and fieldConditions are for the fields of a form or a view; leave them out")
            return
        }
        for p in pairs(conds) {
            let at = base + [p.key.value]
            if shown[p.key.value] == nil {
                add(p.key, pointer(at), .formField, "\(p.key.value) is not a field \(name) shows\(suggest(p.key.value, shown))")
                continue
            }
            for k in ["readOnly", "readOnlyWhen"] where kind == "view" && p.value.child(k) != nil {
                add(p.value.key(k), pointer(at + [k]), .formField, "\(name) is a view, where every field is read-only; leave \(k) out")
            }
            if kind == "form" && p.value.child("readOnly") != nil && p.value.child("readOnlyWhen") != nil {
                add(p.value.key("readOnlyWhen"), pointer(at + ["readOnlyWhen"]), .formField, "\(p.key.value) is read-only on \(name) already; leave readOnlyWhen out, or readOnly")
            }
            if ent == nil { continue }
            let env = d.recordEnv(pg, ent, shown)
            for k in ["readOnlyWhen", "hiddenWhen"] {
                checkCondition(p.value.child(k), pointer(at + [k]), k, env)
            }
        }
    }

    /// Checks a form's checks across its fields and its fields entered
    /// twice: only a form has them, each names fields it shows, and each
    /// message is a full sentence.
    func checkFormChecks(_ d: Design, _ name: String, _ pg: YNode, _ kind: String, _ shown: [String: YNode]) {
        for key in ["checks", "enteredTwice"] where pg.child(key) != nil && kind != "form" {
            add(pg.key(key), pointer("pages", name, key), .formField, "\(name) is a \(kind), and \(key) are checked when a form is sent; leave them out")
        }
        if kind != "form" { return }
        var env: ExprEnv = [:]
        for (n, f) in shown { env[n] = d.fieldType(f) }
        for p in pairs(pg.child("checks")) {
            let at = ["pages", name, "checks", p.key.value]
            checkCondition(p.value.child("expression"), pointer(at + ["expression"]), "the check", env)
            if let m = p.value.child("message"), !sentence(m.value) {
                add(m, pointer(at + ["message"]), .formField, "the message is not a full sentence; start it with a capital and end it with a full stop, so a screen reader reads it as one")
            }
            if let f = p.value.child("field"), shown[f.value] == nil {
                add(f, pointer(at + ["field"]), .formField, "\(f.value) is not a field the form \(name) shows\(suggest(f.value, shown))")
            }
        }
        for (i, f) in items(pg.child("enteredTwice")).enumerated() where shown[f.value] == nil {
            add(f, pointer("pages", name, "enteredTwice", "\(i)"), .formField, "\(f.value) is not a field the form \(name) shows, so it cannot be entered twice\(suggest(f.value, shown))")
        }
    }
}
