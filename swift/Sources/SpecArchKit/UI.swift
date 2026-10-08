import Foundation

// The user interface of docs/ui-design.md: a page's events and where each
// leads.

extension Checker {
    /// Checks each event of a page: that it is raised by a page or an action
    /// of the kind that has it, that the page it leads to exists, and that
    /// with gives exactly the route parameters of that page from fields of
    /// the page's entity, or, on a task page, from properties of the body of
    /// the response the event follows.
    func checkPageEvents(_ d: Design) {
        for p in pairs(d.root.child("pages")) {
            let name = p.key.value, pg = p.value
            let kind = str(pg.child("kind"))
            let fields = fieldsOf(d.entities[str(pg.child("entity"))])
            for e in [("onSubmitted", "form", "a form or a task is submitted"), ("onSelect", "list", "a row of a list is selected")] {
                guard let n = pg.child(e.0) else { continue }
                if e.0 == "onSubmitted" && kind == "task" {
                    checkStatusEvents(d, name, pg, n)
                    continue
                }
                if kind != e.1 {
                    add(pg.key(e.0), pointer("pages", name, e.0), .flow, "\(name) is a \(kind) page, and \(e.0) is raised when \(e.2); leave it out")
                    continue
                }
                checkEvent(d, n, ["pages", name, e.0], fields, "a field of the page's entity")
            }
            for (i, a) in items(pg.child("actions")).enumerated() {
                guard let n = a.child("then") else { continue }
                if str(a.child("kind")) != "operation" {
                    add(a.key("then"), pointer("pages", name, "actions", "\(i)", "then"), .flow, "the action \(str(a.child("label"))) navigates already, and then follows an operation; leave it out")
                    continue
                }
                checkEvent(d, n, ["pages", name, "actions", "\(i)", "then"], fields, "a field of the page's entity")
            }
        }
    }

    /// Checks a task page's onSubmitted: each status is one its submit
    /// operation answers, and each event leads where the body of that
    /// response can take it.
    func checkStatusEvents(_ d: Design, _ name: String, _ pg: YNode, _ events: YNode) {
        let submit = str(pg.child("submit"))
        for kv in pairs(events) {
            let base = ["pages", name, "onSubmitted", kv.key.value]
            guard let op = d.operations[submit]?.node else { continue } // the page check reports the operation
            guard let r = op.child("responses")?.child(kv.key.value) else {
                let declared = pairs(op.child("responses")).map { $0.key.value }.filter { $0.hasPrefix("2") }
                let answers = declared.isEmpty ? "it declares no success" : "it answers " + declared.joined(separator: " and ")
                add(kv.key, pointer(base), .page, "\(submit) does not answer \(kv.key.value), so \(name) cannot act on it; \(answers)")
                continue
            }
            checkEvent(d, kv.value, base, d.responseBodyFields(r), "a property of the body of the " + kv.key.value + " response")
        }
    }

    /// Checks what a task page holds: no entity and no source, since it
    /// loads no record; no columns or filters, which belong to a list; fields
    /// that are properties of the submit operation's request body; and every
    /// property the body requires among them.
    func checkTaskPage(_ d: Design, _ name: String, _ pg: YNode) {
        let base = ["pages", name]
        for (key, why) in [
            ("entity", "it submits to an operation and shows no record of an entity"),
            ("source", "it submits to an operation without loading a record"),
            ("columns", "a list shows columns, and a task shows fields"),
            ("filters", "a list is filtered, and a task shows fields"),
        ] where pg.child(key) != nil {
            add(pg.key(key), pointer(base + [key]), .page, "\(name) is a task page, and \(why); leave \(key) out")
        }
        let submit = str(pg.child("submit"))
        guard let op = d.operations[submit]?.node else { return } // the schema asks for submit, and the reference check reports one that does not exist
        if pairs(op.child("requestBody")?.child("content")).isEmpty {
            add(pg.child("submit"), pointer(base + ["submit"]), .page, "\(submit) takes no request body, and a task page's fields are properties of the body it sends; submit to an operation that takes one")
            return
        }
        let (props, required) = d.requestFields(op)
        var shown = Set<String>()
        for (i, f) in items(pg.child("fields")).enumerated() {
            shown.insert(f.value)
            if props[f.value] == nil {
                add(f, pointer(base + ["fields", "\(i)"]), .page, "\(f.value) is not a property of the request body of \(submit), which \(name) submits to\(suggest(f.value, props))")
            }
        }
        for (i, sec) in items(pg.child("sections")).enumerated() {
            for (j, f) in items(sec.child("fields")).enumerated() {
                shown.insert(f.value)
                if props[f.value] == nil {
                    add(f, pointer(base + ["sections", "\(i)", "fields", "\(j)"]), .page, "\(f.value) is not a property of the request body of \(submit), which \(name) submits to\(suggest(f.value, props))")
                }
            }
        }
        let missing = required.filter { !shown.contains($0) }
        if !missing.isEmpty && !shown.isEmpty {
            let it = missing.count == 1 ? "it" : "them"
            add(pg.key("fields") ?? pg.key("sections"), pointer(base), .page, "\(name) submits to \(submit), whose request body requires \(missing.joined(separator: " and ")); show \(it) on the page, since the request cannot succeed without \(it)")
        }
    }

    /// Checks where one event leads, and that its message is a full
    /// sentence; the route parameters it gives come from fields, which from
    /// names.
    func checkEvent(_ d: Design, _ ev: YNode, _ base: [String], _ fields: [String: YNode], _ from: String) {
        if let m = ev.child("message"), !sentence(m.value) {
            add(m, pointer(base + ["message"]), .flow, "the message is not a full sentence; start it with a capital and end it with a full stop, so a screen reader reads it as one")
        }
        guard let nav = ev.child("navigate") else { return } // the schema asks for navigate beside with
        let with = ev.child("with")
        guard let target = d.pages[nav.value] else {
            add(nav, pointer(base + ["navigate"]), .flow, "\(nav.value) is not a page of the specification\(suggest(nav.value, d.pages))")
            return
        }
        let route = str(target.child("route"))
        let params = pathParameters(route)
        let want = Set(params)
        var given = Set<String>()
        for kv in pairs(with) {
            given.insert(kv.key.value)
            if !want.contains(kv.key.value) {
                add(kv.key, pointer(base + ["with", kv.key.value]), .flow, "\(kv.key.value) is not a route parameter of page \(nav.value), whose route is \(route)")
                continue
            }
            if fields[kv.value.value] == nil {
                add(kv.value, pointer(base + ["with", kv.key.value]), .flow, "\(kv.value.value) is not \(from)\(suggest(kv.value.value, fields))")
            }
        }
        let missing = params.filter { !given.contains($0) }
        if !missing.isEmpty {
            add(with ?? nav, pointer(base + ["navigate"]), .flow, "page \(nav.value) needs the route parameter \(missing.joined(separator: " and ")); give it under with, from \(from)")
        }
    }
}

/// The pages an event of a page leads to: one, or on a task page one per
/// status it acts on, and none when it leads nowhere or the page does not
/// raise it; raised says whether it does.
func eventTargets(_ pg: YNode, _ event: String, _ action: String) -> (targets: [String], raised: Bool) {
    func one(_ t: String) -> [String] { t.isEmpty ? [] : [t] }
    switch event {
    case "select":
        let n = pg.child("onSelect")
        return (one(str(n?.child("navigate"))), n != nil)
    case "submitted":
        switch str(pg.child("kind")) {
        case "form":
            return (one(str(pg.child("onSubmitted")?.child("navigate"))), true)
        case "task":
            var targets: [String] = []
            for kv in pairs(pg.child("onSubmitted")) {
                let t = str(kv.value.child("navigate"))
                if !t.isEmpty && !targets.contains(t) { targets.append(t) }
            }
            return (targets, true)
        default:
            break
        }
    case "action":
        for a in items(pg.child("actions")) where str(a.child("label")) == action {
            if str(a.child("kind")) == "navigate" { return (one(str(a.child("target"))), true) }
            return (one(str(a.child("then")?.child("navigate"))), true)
        }
    default:
        break
    }
    return ([], false)
}

/// The event of a step as a sentence names it.
func stepEvent(_ step: YNode) -> String {
    let e = str(step.child("event"))
    return e == "action" ? "the action " + str(step.child("action")) : e
}

extension Checker {
    /// Checks each flow: that its actor is a role that may open every page
    /// on the way, that each page exists and raises the step's event, and
    /// that each event leads to the next step's page.
    func checkFlows(_ d: Design) {
        for f in pairs(d.root.child("flows")) {
            let name = f.key.value
            let actorNode = f.value.child("actor")
            let actor = str(actorNode)
            let role = d.roles[actor]
            if let actorNode, role == nil {
                add(actorNode, pointer("flows", name, "actor"), .flow, "\(actor) is not a role of the specification\(suggest(actor, d.roles))")
            }
            var granted: Set<String> = ["public"]
            for p in items(role?.child("permissions")) { granted.insert(p.value) }
            let steps = items(f.value.child("steps"))
            for (i, step) in steps.enumerated() {
                let at = ["flows", name, "steps", "\(i)"]
                let pageNode = step.child("page")
                guard let pg = d.pages[str(pageNode)] else {
                    add(pageNode, pointer(at + ["page"]), .flow, "\(str(pageNode)) is not a page of the specification\(suggest(str(pageNode), d.pages))")
                    continue
                }
                let perm = str(pg.child("permission"))
                if role != nil && !granted.contains(perm) {
                    add(pageNode, pointer(at + ["page"]), .flow, "\(actor) cannot open \(str(pageNode)), which needs \(perm); grant it to the role, or give the flow another actor")
                }
                let event = str(step.child("event"))
                if event == "action" && role != nil,
                   let a = items(pg.child("actions")).first(where: { str($0.child("label")) == str(step.child("action")) }) {
                    let aperm = str(a.child("permission"))
                    if !aperm.isEmpty && !granted.contains(aperm) {
                        add(step.child("action"), pointer(at + ["action"]), .flow, "\(actor) cannot take the action \(str(step.child("action"))), which needs \(aperm); grant it to the role, or give the flow another actor")
                    }
                }
                let (targets, raised) = eventTargets(pg, event, str(step.child("action")))
                if !raised {
                    let what = ["select": "has no onSelect", "submitted": "is not a form or a task", "action": "has no action labelled " + str(step.child("action"))][event] ?? ""
                    add(step.child("event"), pointer(at + ["event"]), .flow, "page \(str(pageNode)) \(what), so it does not raise \(stepEvent(step))")
                    continue
                }
                if i + 1 < steps.count {
                    let next = str(steps[i + 1].child("page"))
                    if !targets.contains(next) && d.pages[next] != nil {
                        let leads = targets.isEmpty ? "leads nowhere" : "leads to " + targets.joined(separator: " or ")
                        add(step.child("event"), pointer(at + ["event"]), .flow, "\(stepEvent(step)) on page \(str(pageNode)) \(leads), and the next step is on \(next); make the event lead there, or correct the steps")
                    }
                }
            }
        }
    }
}

extension Design {
    /// The problem types a page can meet, in order, with the operation that
    /// answers each first: those of the operation it reads or submits and
    /// of every operation its actions run.
    func pageProblems(_ pg: YNode) -> (names: [String], by: [String: String]) {
        var by: [String: String] = [:]
        var names: [String] = []
        var ids = [str(pg.child("source")), str(pg.child("submit"))]
        for a in items(pg.child("actions")) where str(a.child("kind")) == "operation" {
            ids.append(str(a.child("target")))
        }
        for id in ids where !id.isEmpty {
            guard let o = operations[id] else { continue }
            for r in pairs(o.node.child("responses")) {
                let p = str(r.value.child("problem"))
                if !p.isEmpty && by[p] == nil {
                    by[p] = id
                    names.append(p)
                }
            }
        }
        return (names, by)
    }
}

/// Whether a message reads as a full sentence: a capital, a letter of a
/// script without case, or a digit first, and a full stop, question mark or
/// exclamation mark last, in Latin or CJK form. Only the ASCII spaces and a
/// byte order mark around it are trimmed, the same in both builds, since one
/// YAML reader drops a byte order mark and the other keeps it.
func sentence(_ s: String) -> Bool {
    let t = s.trimmingCharacters(in: CharacterSet(charactersIn: " \t\n\r\u{0B}\u{0C}\u{FEFF}"))
    guard let first = t.unicodeScalars.first, let last = t.unicodeScalars.last else { return false }
    let cats: [Unicode.GeneralCategory] = [.uppercaseLetter, .titlecaseLetter, .otherLetter, .decimalNumber]
    return cats.contains(first.properties.generalCategory) && ".?!。？！".unicodeScalars.contains(last)
}

extension Checker {
    /// Checks a page's states: a list has empty, a list with filters has
    /// filteredEmpty and a page without them has neither, every problem type
    /// the page can meet is named under failed or covered by its default and
    /// no other is named, a field a problem is about is a field the form
    /// shows, and every message is a full sentence. A page without states is
    /// not checked: its stack shows its own.
    func checkPageStates(_ d: Design) {
        for p in pairs(d.root.child("pages")) {
            let name = p.key.value, pg = p.value
            guard let states = pg.child("states") else { continue }
            let base = ["pages", name, "states"]
            let kind = str(pg.child("kind"))
            let filters = !items(pg.child("filters")).isEmpty
            func message(_ st: YNode, _ at: [String]) {
                if let m = st.child("message"), !sentence(m.value) {
                    add(m, pointer(at + ["message"]), .state, "the message is not a full sentence; start it with a capital and end it with a full stop, so a screen reader reads it as one")
                }
            }
            for k in ["empty", "filteredEmpty"] {
                if let st = states.child(k) { message(st, base + [k]) }
            }
            if kind == "list" && states.child("empty") == nil {
                add(pg.key("states"), pointer(base), .state, "\(name) is a list, so it shows an empty state when there are no records; add empty with its message")
            } else if kind != "list" && states.child("empty") != nil {
                add(states.key("empty"), pointer(base + ["empty"]), .state, "\(name) is a \(kind), which is never empty; leave empty out")
            }
            if filters && states.child("filteredEmpty") == nil {
                add(pg.key("states"), pointer(base), .state, "\(name) has filters, so it shows a state when no record matches them; add filteredEmpty with its message")
            } else if !filters && states.child("filteredEmpty") != nil {
                add(states.key("filteredEmpty"), pointer(base + ["filteredEmpty"]), .state, "\(name) has no filters, so nothing can filter it empty; leave filteredEmpty out")
            }
            let (problems, by) = d.pageProblems(pg)
            let failed = states.child("failed")
            let fields = Set(pageFields(pg).map { $0.value })
            var named: [String: YNode] = [:]
            for kv in pairs(failed) {
                let at = base + ["failed", kv.key.value]
                named[kv.key.value] = kv.value
                if kv.key.value != "default" && by[kv.key.value] == nil {
                    var known: [String: YNode] = [:]
                    for pr in problems { known[pr] = kv.key }
                    add(kv.key, pointer(at), .state, "\(kv.key.value) is not a problem type an operation of \(name) answers\(suggest(kv.key.value, known))")
                }
                message(kv.value, at)
                if let f = kv.value.child("field") {
                    if kind != "form" && kind != "task" {
                        add(f, pointer(at + ["field"]), .state, "field is for a form or a task, which shows a problem beside the field it is about; \(name) is a \(kind)")
                    } else if !fields.contains(f.value) {
                        add(f, pointer(at + ["field"]), .state, "\(f.value) is not a field the \(kind) \(name) shows")
                    }
                }
            }
            if named["default"] != nil { continue }
            let missing = problems.filter { named[$0] == nil }.map { $0 + " (from " + (by[$0] ?? "") + ")" }
            if !missing.isEmpty {
                add(states.key("failed") ?? pg.key("states"), pointer(base + ["failed"]), .state, "\(name) can fail with \(missing.joined(separator: ", ")); give each a message under failed, or a default for the rest")
            }
        }
    }
}

extension Checker {
    /// Checks that a page's compact columns are a list's own columns: what
    /// a compact screen keeps of them.
    func checkCompactColumns(_ d: Design) {
        for p in pairs(d.root.child("pages")) {
            let name = p.key.value, pg = p.value
            guard let cc = pg.child("compactColumns") else { continue }
            let kind = str(pg.child("kind"))
            if kind != "list" {
                add(pg.key("compactColumns"), pointer("pages", name, "compactColumns"), .page, "\(name) is a \(kind), and compactColumns is what a list keeps of its columns on a compact screen; leave it out")
                continue
            }
            var columns: [String: YNode] = [:]
            for col in items(pg.child("columns")) { columns[col.value] = col }
            for (i, col) in items(cc).enumerated() where columns[col.value] == nil {
                add(col, pointer("pages", name, "compactColumns", "\(i)"), .page, "\(col.value) is not a column of \(name), and a compact screen keeps only columns the list has\(suggest(col.value, columns))")
            }
        }
    }
}

extension Checker {
    /// Checks, once the specification names its target, what the design
    /// decides of it: every field a page shows or filters by has a title, its label (WCAG
    /// 2.2, 3.3.2 and 2.4.6), and no two actions of a page share a label, so
    /// each has a name of its own (4.1.2).
    func checkAccessibility(_ d: Design) {
        guard d.root.child("accessibility") != nil else { return }
        var shownOn: [String: [String]] = [:]
        var order: [String] = []
        for p in pairs(d.root.child("pages")) {
            let name = p.key.value, pg = p.value
            let ent = str(pg.child("entity"))
            for list in [items(pg.child("columns")), pageFields(pg), items(pg.child("filters"))] {
                for f in list {
                    let k = ent + "." + f.value
                    let l = shownOn[k] ?? []
                    if l.isEmpty { order.append(k) }
                    if l.last != name { shownOn[k] = l + [name] }
                }
            }
            for row in items(pg.child("childRows")) {
                let target = d.childRowTarget(pg, row)
                for f in items(row.child("fields")) {
                    let k = target + "." + f.value
                    let l = shownOn[k] ?? []
                    if target.isEmpty || l.last == name { continue }
                    if l.isEmpty { order.append(k) }
                    shownOn[k] = l + [name]
                }
            }
            var seen = Set<String>()
            for (i, a) in items(pg.child("actions")).enumerated() {
                guard let l = a.child("label") else { continue }
                if seen.contains(l.value) {
                    add(l, pointer("pages", name, "actions", "\(i)", "label"), .accessibility, "\(name) has two actions labelled \(l.value), which a screen reader cannot tell apart (WCAG 2.2, 4.1.2); give each its own label")
                }
                seen.insert(l.value)
            }
        }
        for k in order {
            let parts = k.split(separator: ".", maxSplits: 1).map(String.init)
            guard parts.count == 2 else { continue }
            let (ent, field) = (parts[0], parts[1])
            guard let f = fieldsOf(d.entities[ent])[field], f.child("title") == nil else { continue }
            let key = d.entities[ent]?.child("properties")?.key(field)
            add(key, pointer("entities", ent, "properties", field), .accessibility, "\(ent).\(field) is shown on \((shownOn[k] ?? []).joined(separator: ", ")) and has no title, the label a person reads beside it (WCAG 2.2, 3.3.2 and 2.4.6); give it a title")
        }
    }
}

/// The fields a form or a view shows, in order: its fields, or the fields
/// of its sections one after another.
func pageFields(_ pg: YNode) -> [YNode] {
    if let f = pg.child("fields") { return items(f) }
    return items(pg.child("sections")).flatMap { items($0.child("fields")) }
}

extension Checker {
    /// Checks that a form or a view gives its fields once, in fields or in
    /// sections, that no field is in two sections, and that a list, which
    /// shows columns, has no sections.
    func checkSections(_ d: Design) {
        for p in pairs(d.root.child("pages")) {
            let name = p.key.value, pg = p.value
            let kind = str(pg.child("kind"))
            let secsNode = pg.child("sections")
            if kind == "form" || kind == "view" || kind == "task" {
                let fields = pg.child("fields")
                if fields == nil && secsNode == nil {
                    add(p.key, pointer("pages", name), .page, "\(name) is a \(kind) and shows no field; give its fields, or its sections")
                } else if fields != nil && secsNode != nil {
                    add(pg.key("sections"), pointer("pages", name, "sections"), .page, "\(name) gives both fields and sections; name its fields once, in fields or in sections")
                    continue
                }
            }
            guard let secs = secsNode else { continue }
            if kind == "list" {
                add(pg.key("sections"), pointer("pages", name, "sections"), .page, "\(name) is a list, which shows columns, and sections group the fields of a form or a view; leave them out")
                continue
            }
            var inSection: [String: String] = [:]
            for (i, sec) in items(secs).enumerated() {
                let title = str(sec.child("title"))
                for (j, f) in items(sec.child("fields")).enumerated() {
                    if let other = inSection[f.value] {
                        add(f, pointer("pages", name, "sections", "\(i)", "fields", "\(j)"), .page, "\(f.value) is in the section \(other) already, and a field is shown once; leave it out of \(title)")
                        continue
                    }
                    inSection[f.value] = title
                }
            }
        }
    }
}

extension Checker {
    /// Checks the child rows of a form: only a form has them, each names a
    /// one-to-many relation of the form's entity once, and its fields are
    /// fields of the entity the relation reaches.
    func checkChildRows(_ d: Design) {
        for p in pairs(d.root.child("pages")) {
            let name = p.key.value, pg = p.value
            guard let rows = pg.child("childRows") else { continue }
            let kind = str(pg.child("kind"))
            if kind != "form" {
                add(pg.key("childRows"), pointer("pages", name, "childRows"), .page, "\(name) is a \(kind), and childRows are the records of a relation edited under a form; leave them out")
                continue
            }
            let ent = str(pg.child("entity"))
            guard let e = d.entities[ent] else { continue } // the entity check reports it
            var relations: [String: YNode] = [:]
            for r in pairs(e.child("relations")) { relations[r.key.value] = r.value }
            var seen = Set<String>()
            for (i, row) in items(rows).enumerated() {
                let at = ["pages", name, "childRows", "\(i)"]
                guard let rn = row.child("relation") else { continue } // the schema asks for it
                let rel = str(rn)
                if seen.contains(rel) {
                    add(rn, pointer(at + ["relation"]), .page, "\(name) has child rows of \(rel) already; edit the rows of a relation once")
                    continue
                }
                seen.insert(rel)
                guard let r = relations[rel] else {
                    add(rn, pointer(at + ["relation"]), .page, "\(rel) is not a relation of \(ent), the entity of \(name)\(suggest(rel, relations))")
                    continue
                }
                let k = str(r.child("kind"))
                if k != "one-to-many" {
                    add(rn, pointer(at + ["relation"]), .page, "\(rel) is a \(k) relation of \(ent), and child rows are the records of a one-to-many relation")
                    continue
                }
                let target = str(r.child("target"))
                guard let t = d.entities[target] else { continue } // the relation check reports it
                checkFieldList(row.child("fields"), at + ["fields"], fieldsOf(t), target, "shown in the rows of " + rel)
            }
        }
    }
}

extension Design {
    /// The entity the child rows of a form reach through their relation, or
    /// "" when the relation does not resolve.
    func childRowTarget(_ pg: YNode, _ row: YNode) -> String {
        str(entities[str(pg.child("entity"))]?.child("relations")?.child(str(row.child("relation")))?.child("target"))
    }
}
