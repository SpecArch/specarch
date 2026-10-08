import Foundation

// The user interface of docs/ui-design.md: a page's events and where each
// leads.

extension Checker {
    /// Checks each event of a page: that it is raised by a page or an action
    /// of the kind that has it, that the page it leads to exists, and that
    /// with gives exactly the route parameters of that page from fields of
    /// the page's entity.
    func checkPageEvents(_ d: Design) {
        for p in pairs(d.root.child("pages")) {
            let name = p.key.value, pg = p.value
            let kind = str(pg.child("kind"))
            let fields = fieldsOf(d.entities[str(pg.child("entity"))])
            for e in [("onSubmitted", "form", "a form is submitted"), ("onSelect", "list", "a row of a list is selected")] {
                guard let n = pg.child(e.0) else { continue }
                if kind != e.1 {
                    add(pg.key(e.0), pointer("pages", name, e.0), .flow, "\(name) is a \(kind) page, and \(e.0) is raised when \(e.2); leave it out")
                    continue
                }
                checkEvent(d, n, ["pages", name, e.0], fields)
            }
            for (i, a) in items(pg.child("actions")).enumerated() {
                guard let n = a.child("then") else { continue }
                if str(a.child("kind")) != "operation" {
                    add(a.key("then"), pointer("pages", name, "actions", "\(i)", "then"), .flow, "the action \(str(a.child("label"))) navigates already, and then follows an operation; leave it out")
                    continue
                }
                checkEvent(d, n, ["pages", name, "actions", "\(i)", "then"], fields)
            }
        }
    }

    /// Checks where one event leads.
    func checkEvent(_ d: Design, _ ev: YNode, _ base: [String], _ fields: [String: YNode]) {
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
                add(kv.value, pointer(base + ["with", kv.key.value]), .flow, "\(kv.value.value) is not a field of the page's entity\(suggest(kv.value.value, fields))")
            }
        }
        let missing = params.filter { !given.contains($0) }
        if !missing.isEmpty {
            add(with ?? nav, pointer(base + ["navigate"]), .flow, "page \(nav.value) needs the route parameter \(missing.joined(separator: " and ")); give it under with, from a field of the page's entity")
        }
    }
}

/// The page an event of a page leads to, or "" when it leads nowhere or the
/// page does not raise it; raised says whether it does.
func eventTarget(_ pg: YNode, _ event: String, _ action: String) -> (target: String, raised: Bool) {
    switch event {
    case "select":
        let n = pg.child("onSelect")
        return (str(n?.child("navigate")), n != nil)
    case "submitted":
        return (str(pg.child("onSubmitted")?.child("navigate")), str(pg.child("kind")) == "form")
    case "action":
        for a in items(pg.child("actions")) where str(a.child("label")) == action {
            if str(a.child("kind")) == "navigate" { return (str(a.child("target")), true) }
            return (str(a.child("then")?.child("navigate")), true)
        }
    default:
        break
    }
    return ("", false)
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
                let (target, raised) = eventTarget(pg, event, str(step.child("action")))
                if !raised {
                    let what = ["select": "has no onSelect", "submitted": "is not a form", "action": "has no action labelled " + str(step.child("action"))][event] ?? ""
                    add(step.child("event"), pointer(at + ["event"]), .flow, "page \(str(pageNode)) \(what), so it does not raise \(stepEvent(step))")
                    continue
                }
                if i + 1 < steps.count {
                    let next = str(steps[i + 1].child("page"))
                    if target != next {
                        let leads = target.isEmpty ? "leads nowhere" : "leads to " + target
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

/// Whether a message reads as a full sentence: a capital or a digit first,
/// and a full stop, question mark or exclamation mark last.
func sentence(_ s: String) -> Bool {
    let t = s.trimmingCharacters(in: .whitespacesAndNewlines)
    guard let first = t.unicodeScalars.first, let last = t.unicodeScalars.last else { return false }
    let cat = first.properties.generalCategory
    return (cat == .uppercaseLetter || cat == .decimalNumber) && ".?!".unicodeScalars.contains(last)
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
                    add(m, pointer(at + ["message"]), .state, "\(quote(m.value)) is not a full sentence; start it with a capital and end it with a full stop, so a screen reader reads it as one")
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
            let fields = Set(items(pg.child("fields")).map { $0.value })
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
                    if kind != "form" {
                        add(f, pointer(at + ["field"]), .state, "field is for a form, which shows a problem beside the field it is about; \(name) is a \(kind)")
                    } else if !fields.contains(f.value) {
                        add(f, pointer(at + ["field"]), .state, "\(f.value) is not a field the form \(name) shows")
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
