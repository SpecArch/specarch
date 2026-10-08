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
