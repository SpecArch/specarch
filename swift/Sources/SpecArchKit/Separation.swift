import Foundation

extension Checker {
    /// Checks each separation-of-duties set: every permission it names is
    /// declared and is not public, its cardinality is no more than the
    /// permissions it names, and no role grants cardinality or more of
    /// them. Which person holds which roles is data, so two roles that
    /// together reach a set are left to the techspec's list.
    func checkSeparationOfDuties(_ d: Design) {
        for set in pairs(d.root.child("separationOfDuties")) {
            let name = set.key.value
            var perms: [String] = []
            var seen = Set<String>()
            for (i, p) in items(set.value.child("permissions")).enumerated() {
                let ptr = pointer("separationOfDuties", name, "permissions", "\(i)")
                if p.value == "public" {
                    add(p, ptr, .separationOfDuties, "set \(name) names public, which everyone holds, so no role can be kept from it; name only permissions a role grants")
                } else if d.permissions[p.value] == nil {
                    let hint = suggest(p.value, d.permissions).replacingOccurrences(of: "; there are none in the specification", with: "")
                    add(p, ptr, .separationOfDuties, "set \(name) names permission \(p.value), which is not declared; add it under permissions\(hint)")
                }
                if !p.value.isEmpty && !seen.contains(p.value) {
                    seen.insert(p.value)
                    perms.append(p.value)
                }
            }
            var cardinality = 2
            if let n = set.value.child("cardinality") {
                guard let v = Int(n.value), v >= 2 else { continue }
                cardinality = v
                if cardinality > perms.count && perms.count >= 2 {
                    add(n, pointer("separationOfDuties", name, "cardinality"), .separationOfDuties,
                        "set \(name) asks that no holder reach \(cardinality) of its permissions but names only \(perms.count), so no role can ever break it; lower cardinality to \(perms.count) or name more permissions")
                    continue
                }
            }
            for role in pairs(d.root.child("roles")) {
                let grants = Set(items(role.value.child("permissions")).map { $0.value })
                let held = perms.filter { grants.contains($0) }
                if held.count >= cardinality {
                    add(role.key, pointer("roles", role.key.value), .separationOfDuties,
                        "role \(role.key.value) grants \(joinAnd(held)) of set \(name), and no holder may have \(cardinality) of its permissions; split them between roles")
                }
            }
        }
    }
}
