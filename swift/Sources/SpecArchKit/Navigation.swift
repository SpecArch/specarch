import Foundation

// The navigation a person goes through (ADR-197): the page each role is
// shown first, and the gaps a walkthrough of the screens asks about.

/// Whether a holder of the permissions held may open what needs perm:
/// public and signedIn are held by every signed-in caller.
func opens(_ held: Set<String>, _ perm: String) -> Bool {
    perm == permissionPublic || perm == permissionSignedIn || held.contains(perm)
}

/// A leaf of the menus: the entry and the page it opens.
struct MenuEntry {
    let node: YNode
    let path: [String]
    let page: String
}

/// Every page a page's actions, events and links lead to.
func pageTargets(_ pg: YNode) -> [String] {
    var out: [String] = []
    func add(_ n: YNode?) {
        let t = str(n?.child("navigate"))
        if !t.isEmpty { out.append(t) }
    }
    for a in items(pg.child("actions")) {
        if str(a.child("kind")) == "navigate" { out.append(str(a.child("target"))) }
        add(a.child("then"))
    }
    add(pg.child("onSelect"))
    if str(pg.child("kind")) == "task" {
        for kv in pairs(pg.child("onSubmitted")) { add(kv.value) }
    } else {
        add(pg.child("onSubmitted"))
    }
    for b in items(pg.child("blocks")) {
        let t = str(b.child("link")?.child("page"))
        if !t.isEmpty { out.append(t) }
    }
    return out
}

extension Design {
    /// Every leaf of the menus, in the order written.
    var menuEntries: [MenuEntry] {
        var out: [MenuEntry] = []
        func visit(_ entries: YNode?, _ path: [String]) {
            for m in pairs(entries) {
                let p = path + [m.key.value]
                if let page = m.value.child("page") { out.append(MenuEntry(node: m.key, path: p, page: page.value)) }
                visit(m.value.child("items"), p + ["items"])
            }
        }
        visit(root.child("menus"), ["menus"])
        return out
    }

    /// The roles no person signs in as: those a job acts as or a service
    /// client holds, which open no page.
    var personless: Set<String> {
        var out = Set<String>()
        for j in pairs(root.child("jobs")) { out.insert(str(j.value.child("role"))) }
        for cl in clientList where str(cl.value.child("kind")) == "service" {
            for r in items(cl.value.child("roles")) { out.insert(r.value) }
        }
        return out
    }
}

extension Checker {
    /// Checks each role's home: a page of the specification, opened with no
    /// route parameters, which the role may open.
    func checkRoleHomes(_ d: Design) {
        for r in pairs(d.root.child("roles")) {
            guard let home = r.value.child("home") else { continue }
            let at = pointer("roles", r.key.value, "home")
            guard let pg = d.pages[home.value] else {
                add(home, at, .roleHome, "\(home.value) is not a page of the specification\(suggest(home.value, d.pages))")
                continue
            }
            let params = pathParameters(str(pg.child("route")))
            if !params.isEmpty {
                add(home, at, .roleHome, "\(home.value) takes the route parameters \(params.joined(separator: ", ")), and a home page is opened with none; name a page whose route takes none")
            }
            let perm = str(pg.child("permission"))
            if !perm.isEmpty && !opens(d.roleHolds(r.value), perm) {
                add(home, at, .roleHome, "\(r.key.value) cannot open its home \(home.value), which needs \(perm); grant it to the role, or name a page the role can open")
            }
        }
    }

    /// Reports, as warnings, the gaps a walkthrough of the screens asks
    /// about: a page nothing leads to, a role that can open no menu entry, a
    /// menu entry no role can open, an operation action whose problem types
    /// a page without states shows nothing for, and a form that says nothing
    /// of where it leads once sent. The ones about menus are checked once
    /// the specification has menus.
    func checkNavigation(_ d: Design) {
        let pages = pairs(d.root.child("pages"))
        let entries = d.menuEntries
        let menus = d.root.child("menus") != nil
        var led = Set<String>()
        for e in entries { led.insert(e.page) }
        for p in pages {
            for t in pageTargets(p.value) where t != p.key.value { led.insert(t) }
        }
        for r in pairs(d.root.child("roles")) { led.insert(str(r.value.child("home"))) }
        for p in pages {
            let name = p.key.value, pg = p.value
            let at = pointer("pages", name)
            let perm = str(pg.child("permission"))
            if menus && !led.contains(name) && perm != permissionPublic {
                warn(p.key, at, .navigation, "nothing leads to \(name): no menu entry, action, event or link opens it, and it is no role's home; add a way to it, make it a role's home, or remove it")
            }
            let kind = str(pg.child("kind"))
            if kind == "form" && pg.child("onSubmitted") == nil {
                warn(p.key, at, .navigation, "\(name) is a form with no onSubmitted, so nothing says where the person goes or what they are told once it is sent; add onSubmitted")
            }
            if pg.child("states") != nil { continue } // checkPageStates reports a problem type without a message
            for (i, a) in items(pg.child("actions")).enumerated() where str(a.child("kind")) == "operation" {
                guard let o = d.operations[str(a.child("target"))] else { continue }
                var problems: [String] = []
                for r in pairs(o.node.child("responses")) {
                    let pr = str(r.value.child("problem"))
                    if !pr.isEmpty && !problems.contains(pr) { problems.append(pr) }
                }
                if !problems.isEmpty {
                    warn(a, pointer("pages", name, "actions", "\(i)"), .navigation, "the action \(str(a.child("label"))) runs \(o.id), which can answer \(joinAnd(problems)), and \(name) has no states to say what the person is shown then; add states.failed with a message for each, or a default")
                }
            }
        }
        if !menus { return }
        let personless = d.personless
        for r in pairs(d.root.child("roles")) where !personless.contains(r.key.value) {
            let held = d.roleHolds(r.value)
            let reaches = entries.contains { e in
                guard let pg = d.pages[e.page] else { return false }
                return opens(held, str(pg.child("permission")))
            }
            if !reaches {
                warn(r.key, pointer("roles", r.key.value), .navigation, "\(r.key.value) can open no menu entry, so a person holding it finds no page; grant it the permission of an entry's page, or add an entry it can open")
            }
        }
        if d.root.child("roles") == nil { return } // the roles are another module's, which permission_ungranted reads
        for e in entries {
            guard let pg = d.pages[e.page] else { continue } // checkMenus reports it
            let perm = str(pg.child("permission"))
            let reaches = pairs(d.root.child("roles")).contains { opens(d.roleHolds($0.value), perm) }
            if perm == permissionNobody {
                warn(e.node, pointer(e.path), .navigation, "\(e.page) is closed to every caller with nobody, so the entry is shown to nobody; take the entry out")
            } else if !reaches && perm != permissionPublic && perm != permissionSignedIn {
                warn(e.node, pointer(e.path), .navigation, "no role can open \(e.page), which needs \(perm), so the entry is shown to nobody; grant \(perm) to a role, or take the entry out")
            }
        }
    }
}
