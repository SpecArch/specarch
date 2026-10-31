import Foundation

// Who an element is for (ADR-119): an internal element is left out of a
// document for everyone, so an element for everyone may not name one.

/// The sections whose entries are each an element that may carry audience.
private let keyedElements: Set<String> = [
    "stakeholders", "needs", "requirements", "glossary", "assumptions", "constraints",
    "enums", "entities", "views", "schemas", "permissions", "roles", "clients", "tokens", "separationOfDuties", "middlewares", "handshakes",
    "commands", "channels", "dependencies", "jobs", "processes", "gateways", "workflows", "errors", "pages", "flows",
    "algorithms", "tests", "decisions",
    "environments", "configuration", "migrations", "checks", "monitors",
]

/// Whether the path of the merged document names an element that may carry
/// audience: an entry of a section of named elements, the session,
/// release, rollback, backup, restore or sign-off, an operation, an
/// entity's relation or unique constraint, or a channel's message.
func isElement(_ path: [String]) -> Bool {
    switch path.count {
    case 1:
        return ["caller", "securityEvents", "session", "signIn", "release", "rollback", "signoff", "backup", "restore"].contains(path[0])
    case 2:
        return keyedElements.contains(path[0])
    case 3:
        return path[0] == "paths" && ["get", "post", "put", "patch", "delete"].contains(path[2])
    case 4:
        return path[0] == "entities" && (path[2] == "relations" || path[2] == "constraints") || path[0] == "channels" && path[2] == "messages"
    default:
        return false
    }
}

/// Splits a $ref of this specification, #/<section>/<name>.
private func localRef(_ s: String) -> (section: String, name: String)? {
    guard s.hasPrefix("#/") else { return nil }
    let rest = s.dropFirst(2)
    guard let slash = rest.firstIndex(of: "/") else { return nil }
    let section = String(rest[..<slash]), name = String(rest[rest.index(after: slash)...])
    if name.isEmpty || name.contains("/") { return nil }
    return (section, name)
}

extension Checker {
    /// Reports an element for everyone that names an internal element of
    /// the specification: through a $ref, a relation's target, a page's
    /// entity or a requirement's needs. A document for everyone leaves the
    /// internal element out, so it would name what it may not show. An
    /// element inside an internal one is internal with it.
    func checkAudience(_ d: Design) {
        func isInternal(_ n: YNode?) -> Bool { str(n?.child("audience")) == "internal" }
        func named(_ n: YNode, _ path: [String], _ element: [String], _ target: [String], _ node: YNode?) {
            guard isInternal(node) else { return }
            let from = pointer(element), to = pointer(target)
            add(n, pointer(path), .audience,
                "\(to) is internal, and \(from) is for everyone and names it, so a document for everyone would name what it may not show; give \(from) audience: internal too, or take audience: internal off \(to)")
        }
        func visit(_ n: YNode?, _ path: [String], _ outer: [String]?) {
            guard let n else { return }
            var element = outer
            switch n.kind {
            case .mapping:
                if isElement(path) {
                    if isInternal(n) { return }
                    element = path
                }
                if let v = n.child("$ref"), v.kind == .scalar, let element, let (section, name) = localRef(v.value) {
                    let sections: [String: [String: YNode]] = ["entities": d.entities, "enums": d.enums, "views": d.views, "schemas": d.schemas]
                    named(v, path + ["$ref"], element, [section, name], sections[section]?[name])
                }
                if path.count == 4 && path[0] == "entities" && path[2] == "relations" {
                    if let t = n.child("target"), t.kind == .scalar, !isCross(t.value), let element {
                        named(t, path + ["target"], element, ["entities", t.value], d.entities[t.value])
                    }
                } else if path.count == 2 && path[0] == "pages" {
                    if let e = n.child("entity"), e.kind == .scalar, !isCross(e.value), let element {
                        named(e, path + ["entity"], element, ["entities", e.value], d.entities[e.value])
                    }
                } else if path.count == 2 && path[0] == "requirements", let element {
                    for (i, item) in items(n.child("needs")).enumerated() {
                        named(item, path + ["needs", "\(i)"], element, ["needs", item.value], d.needs[item.value])
                    }
                }
                for p in n.pairs { visit(p.value, path + [p.key.value], element) }
            case .sequence:
                for (i, item) in n.items.enumerated() { visit(item, path + ["\(i)"], element) }
            case .scalar:
                break
            }
        }
        visit(d.root, [], nil)
    }
}
