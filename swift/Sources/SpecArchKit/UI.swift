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
