import Foundation

// The keywords of the dxlib study for what runs on its own and how a user
// finds a page (docs/dxlib-lessons.md, section 2): jobs and menus.

extension Checker {
    /// Checks that a job acts as a role, reads and writes entities, consumes
    /// and publishes messages, and calls dependencies that exist.
    func checkJobs(_ d: Design) {
        for j in pairs(d.root.child("jobs")) {
            let name = j.key.value
            if let r = j.value.child("role"), d.roles[r.value] == nil {
                add(r, pointer("jobs", name, "role"), .job, "\(r.value) is not a role of the specification\(suggest(r.value, d.roles))")
            }
            for list in ["reads", "writes"] {
                for (i, e) in items(j.value.child(list)).enumerated() where d.entities[e.value] == nil {
                    add(e, pointer("jobs", name, list, "\(i)"), .job, "\(e.value) is not an entity of the specification\(suggest(e.value, d.entities))")
                }
            }
            if let m = j.value.child("trigger")?.child("consumes"), d.message(m.value) == nil {
                add(m, pointer("jobs", name, "trigger", "consumes"), .job, "\(m.value) does not name a channel and one of its messages; write channel/Message for a message declared under channels")
            }
            for (i, e) in items(j.value.child("emits")).enumerated() where d.message(e.value) == nil {
                add(e, pointer("jobs", name, "emits", "\(i)"), .emits, "\(e.value) does not name a channel and one of its messages; write channel/Message for a message declared under channels")
            }
            for (i, e) in items(j.value.child("calls")).enumerated() where d.dependencies[e.value] == nil {
                add(e, pointer("jobs", name, "calls", "\(i)"), .dependency, "\(e.value) is not a dependency of the specification; declare it under dependencies with its timeout\(suggest(e.value, d.dependencies))")
            }
        }
    }

    /// Checks that every menu entry that opens a page names one.
    func checkMenus(_ d: Design) {
        func visit(_ entries: YNode?, _ path: [String]) {
            for m in pairs(entries) {
                let p = path + [m.key.value]
                if let page = m.value.child("page"), d.pages[page.value] == nil {
                    add(page, pointer(p + ["page"]), .menu, "\(page.value) is not a page of the specification\(suggest(page.value, d.pages))")
                }
                visit(m.value.child("items"), p + ["items"])
            }
        }
        visit(d.root.child("menus"), ["menus"])
    }
}
