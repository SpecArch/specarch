import Foundation

extension Checker {
    /// Checks each permission's implies: every name it lists is declared
    /// and is not public, public implies nothing, and no permission implies
    /// itself through the implications. A cycle is reported at every
    /// permission on it, each with the way round.
    func checkImplies(_ d: Design) {
        let names = pairs(d.root.child("permissions")).map { $0.key.value }.sorted()
        for name in names {
            guard let list = d.permissions[name]?.child("implies") else { continue }
            if name == "public" && !items(list).isEmpty {
                add(list, pointer("permissions", name, "implies"), .implies,
                    "public implies \(joinAnd(items(list).map { $0.value })), which would give it to everyone, signed in or not; grant it to a role instead")
                continue
            }
            if name == permissionSignedIn && !items(list).isEmpty {
                add(list, pointer("permissions", name, "implies"), .implies,
                    "signedIn implies \(joinAnd(items(list).map { $0.value })), which would give it to everyone signed in, whatever their roles; grant it to a role instead")
                continue
            }
            for (i, p) in items(list).enumerated() {
                let ptr = pointer("permissions", name, "implies", "\(i)")
                if p.value == "public" || p.value == permissionSignedIn {
                    add(p, ptr, .implies, "permission \(name) implies \(p.value), which everyone holds already; take it out of the list")
                } else if d.permissions[p.value] == nil {
                    let hint = suggest(p.value, d.permissions).replacingOccurrences(of: "; there are none in the specification", with: "")
                    add(p, ptr, .implies, "permission \(name) implies \(p.value), which is not declared; add it under permissions\(hint)")
                }
            }
            if let way = d.impliesCycle(name) {
                add(list, pointer("permissions", name, "implies"), .implies,
                    "permission \(name) implies itself through \(way.joined(separator: " → ")); take one of the implications out")
            }
        }
    }
}

extension Design {
    /// The shortest way from a permission back to itself through the
    /// implications, starting and ending with its name, or nil when there
    /// is none. The implications are followed in the order they are
    /// written, so the way is the same in every run.
    func impliesCycle(_ name: String) -> [String]? {
        var from: [String: String] = [:]
        var queue = [name]
        while !queue.isEmpty {
            let at = queue.removeFirst()
            for next in impliedBy(at) {
                if next == name {
                    var way = [name]
                    var p = at
                    while p != name {
                        way.insert(p, at: 0)
                        p = from[p]!
                    }
                    return [name] + way
                }
                if from[next] == nil {
                    from[next] = at
                    queue.append(next)
                }
            }
        }
        return nil
    }

    /// The declared, non-public permissions a permission lists under
    /// implies, in the order written.
    func impliedBy(_ name: String) -> [String] {
        items(permissions[name]?.child("implies")).map { $0.value }.filter { $0 != "public" && $0 != permissionSignedIn && permissions[$0] != nil }
    }

    /// Every permission a holder of the given ones holds: those and every
    /// one they imply, transitively.
    func holds(_ perms: [String]) -> Set<String> {
        var held = Set<String>()
        var queue = perms
        while !queue.isEmpty {
            let p = queue.removeFirst()
            if held.contains(p) { continue }
            held.insert(p)
            queue += impliedBy(p)
        }
        return held
    }

    /// Every permission a role grants, with the ones they imply.
    func roleHolds(_ role: YNode?) -> Set<String> {
        holds(items(role?.child("permissions")).map { $0.value })
    }
}
