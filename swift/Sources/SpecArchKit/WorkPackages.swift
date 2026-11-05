import Foundation

// The work-package records (ADR-245): one piece of the delivery each,
// naming the requirements it holds, the release it belongs to and the
// packages it comes after. Its design elements are those that satisfy its
// requirements and its tests those that verify them. A cycle of after
// links is an error at each link on it; every other rule is a warning at
// the record, and a withdrawn package is held to none of them.

/// The keywords that name what an element meets, not what it needs.
private let meetsKeywords: Set<String> = ["satisfies", "verifies", "decidedIn"]

/// The element a pointer into the specification is in: the section for a
/// section that holds one thing, otherwise the section and the entry's
/// key; "" for a pointer to a whole section.
func elementOfPointer(_ ptr: String) -> String {
    let trimmed = ptr.hasPrefix("/") ? String(ptr.dropFirst()) : ptr
    let tokens = trimmed.split(separator: "/", omittingEmptySubsequences: false).map(String.init)
    guard let first = tokens.first, !first.isEmpty else { return "" }
    if singleSections.contains(first) { return "/" + first }
    if tokens.count < 2 { return "" }
    return "/" + first + "/" + tokens[1]
}

private func appendOnce(_ list: inout [String], _ s: String) {
    if !list.contains(s) { list.append(s) }
}

/// What the work-package rules read: the packages that are not withdrawn,
/// the packages that hold each requirement, the design elements that
/// satisfy each requirement, and the elements each element names, from the
/// reference graph the diagrams walk.
final class PackagePlan {
    let known: [String: Record]
    var live: [Record] = []
    var holders: [String: [String]] = [:]
    var satisfied: [String: [String]] = [:]
    var satisfies: [String: [String]] = [:]
    var needs: [String: [String]] = [:]

    init(known: [String: Record]) { self.known = known }

    /// The package and every package its after chain reaches.
    func before(_ id: String) -> Set<String> {
        var out = Set<String>()
        func visit(_ id: String) {
            if out.contains(id) { return }
            out.insert(id)
            if let r = known[id] {
                for link in items(r.root?.child("after")) { visit(link.value) }
            }
        }
        visit(id)
        return out
    }

    /// The live packages that hold a requirement an element satisfies, in
    /// the order the requirements and packages come.
    func builtIn(_ element: String) -> [String] {
        var out: [String] = []
        for req in satisfies[element] ?? [] {
            for id in holders[req] ?? [] { appendOnce(&out, id) }
        }
        return out
    }

    /// The settings a requirement adds: a setting that satisfies it, and a
    /// setting an element that satisfies it names, such as a process's
    /// configuration.
    func settingsOf(_ req: String) -> [String] {
        var out: [String] = []
        for element in satisfied[req] ?? [] {
            if element.hasPrefix("/configuration/") { appendOnce(&out, element) }
            for need in needs[element] ?? [] where need.hasPrefix("/configuration/") {
                appendOnce(&out, need)
            }
        }
        return out
    }
}

/// Whether a reference is the target of a one-to-many or many-to-many
/// relation: the related rows hold the key, or a table between them does,
/// so the entity does not need its target to be built first.
private func pointsBack(_ root: YNode, _ from: String) -> Bool {
    let tokens = from.dropFirst().split(separator: "/", omittingEmptySubsequences: false).map { unescapeToken(String($0)) }
    guard tokens.count == 5, tokens[0] == "entities", tokens[2] == "relations", tokens[4] == "target" else { return false }
    let kind = str(root.child("entities")?.child(tokens[1])?.child("relations")?.child(tokens[3])?.child("kind"))
    return kind == "one-to-many" || kind == "many-to-many"
}

/// The chain of after links from one package back to another, the
/// packages on it from the first to the last, or nil when there is none;
/// seen keeps the walk out of a cycle that does not reach the last.
private func afterChain(_ from: String, _ to: String, _ known: [String: Record], _ seen: inout Set<String>) -> [String]? {
    if from == to { return [to] }
    guard !seen.contains(from), let r = known[from] else { return nil }
    seen.insert(from)
    for link in items(r.root?.child("after")) {
        if let rest = afterChain(link.value, to, known, &seen) { return [from] + rest }
    }
    return nil
}

private func afterChainText(_ id: String, _ chain: [String]) -> String {
    if chain.count == 1 { return id + " comes after itself" }
    return id + " comes after " + chain.joined(separator: ", which comes after ") + ", so the chain comes back to " + id
}

extension RecordChecks {
    /// Checks that each requirement a work package names is a requirement
    /// of the specification, or of a declared requirement set, named by
    /// its ID.
    func packageRequirements(_ r: Record) {
        for (i, item) in items(r.root?.child("requirements")).enumerated() {
            let v = str(item)
            let ptr = pointer("requirements", String(i))
            if v.isEmpty { continue }
            if v.hasPrefix("#/") {
                r.c.add(item, ptr, .recordRef, "\(v) is a pointer, and a work package names requirements by their ID only, its design elements being those that satisfy them; name each requirement by its ID")
            } else if resolveRef(v) == .missing {
                r.c.add(item, ptr, .recordRef, "\(v) is not a requirement of the specification\(suggest(v, d.requirements))")
            }
        }
    }

    func plan(_ recs: [Record]) -> PackagePlan {
        let pl = PackagePlan(known: set["work-package"] ?? [:])
        for r in recs where r.kind == "work-package" && r.str("status") != "withdrawn" {
            pl.live.append(r)
            for item in items(r.root?.child("requirements")) {
                let v = str(item)
                if !v.isEmpty { appendOnce(&pl.holders[v, default: []], r.str("id")) }
            }
        }
        walk(d.root, []) { n, path in
            let list = items(n.child("satisfies"))
            if list.isEmpty { return }
            let element = elementOfPointer(pointer(path))
            if element.isEmpty { return }
            for item in list {
                appendOnce(&pl.satisfied[item.value, default: []], element)
                appendOnce(&pl.satisfies[element, default: []], item.value)
            }
        }
        for ref in modelReferences(d.root) where !meetsKeywords.contains(ref.through) && !pointsBack(d.root, ref.from) {
            let from = elementOfPointer(ref.from), to = elementOfPointer(ref.to)
            if from.isEmpty || to.isEmpty || from == to { continue }
            appendOnce(&pl.needs[from, default: []], to)
        }
        return pl
    }

    /// Applies the work-package rules of ADR-245 across the records: the
    /// cycle, then per live package its order, overlap, release and
    /// configuration, and the entry and exit criteria of a package marked
    /// ready or done.
    func checkWorkPackages(_ recs: [Record]) {
        let pl = plan(recs)
        for r in recs where r.kind == "work-package" {
            let id = r.str("id")
            for (i, link) in items(r.root?.child("after")).enumerated() {
                if pl.known[link.value] == nil { continue }
                var seen = Set<String>()
                if let chain = afterChain(link.value, id, pl.known, &seen) {
                    r.c.add(link, pointer("after", String(i)), .workPackageCycle, "\(afterChainText(id, chain)); a package comes only after packages that can be done before it, so take one link out")
                }
            }
        }
        if pl.live.isEmpty { return }
        let (deployed, anyValue) = deployedSettings()
        for r in pl.live {
            let id = r.str("id")
            let before = pl.before(id)
            let release = r.str("release")
            var misordered = false
            for (i, item) in items(r.root?.child("requirements")).enumerated() {
                let req = str(item)
                let ptr = pointer("requirements", String(i))
                if req.isEmpty { continue }
                let others = (pl.holders[req] ?? []).filter { $0 != id }
                if !others.isEmpty {
                    r.c.warn(item, ptr, .workPackageOverlap, "requirement \(req) is also in \(others.joined(separator: ", ")), and a requirement belongs to one package; split it into one requirement per package, or take it out of all but one")
                }
                guard let node = d.requirements[req] else { continue }
                let rv = str(node.child("release"))
                if !release.isEmpty && !rv.isEmpty && rv != release {
                    r.c.warn(item, ptr, .workPackageRelease, "requirement \(req) is meant for release \(rv) and \(id) for release \(release); move the requirement to a package of release \(rv), or change its release")
                }
                for element in pl.satisfied[req] ?? [] {
                    for need in pl.needs[element] ?? [] {
                        let built = pl.builtIn(need)
                        if built.isEmpty || built.contains(where: { before.contains($0) }) { continue }
                        misordered = true
                        r.c.warn(item, ptr, .workPackageOrder, "requirement \(req) is met by \(element), which needs \(need), and only \(built.joined(separator: ", ")) \(built.count == 1 ? "builds" : "build") it, outside the after chain of \(id); add \(built[0]) to after, or move what \(element) needs into this package")
                    }
                }
                if anyValue {
                    for setting in pl.settingsOf(req) {
                        let name = String(setting.dropFirst("/configuration/".count))
                        guard let s = d.settings[name] else { continue }
                        if str(s.child("secret")) == "true" || s.child("schema")?.child("default") != nil || deployed.contains(name) { continue }
                        r.c.warn(item, ptr, .workPackageUnconfigured, "requirement \(req) needs setting \(name), which has no default and no deployment of an implementation file gives a value, so the package cannot run once installed; give it a value under configuration in each deployment, or a default in its schema")
                    }
                }
            }
            switch r.str("status") {
            case "ready":
                let reasons = notReady(r, pl, misordered)
                if !reasons.isEmpty {
                    r.c.warn(r.root?.child("status"), "/status", .workPackageNotReady, "\(id) is marked ready, but \(reasons.joined(separator: "; ")); settle \(reasons.count == 1 ? "it" : "them"), or set status to planned")
                }
            case "done":
                let tests = unverified(r, recs)
                if !tests.isEmpty {
                    r.c.warn(r.root?.child("status"), "/status", .workPackageUnverified, "\(id) is marked done, but \(tests.count == 1 ? "the test" : "the tests") \(joinAnd(tests)) \(tests.count == 1 ? "has" : "have") no passed result in the test-run records; record a run that passes with specarch log test-run, or set status to in-progress")
                }
            default:
                break
            }
        }
    }

    /// The settings some deployment of an implementation file gives a
    /// value, and whether any deployment gives one at all.
    func deployedSettings() -> (Set<String>, Bool) {
        var out = Set<String>()
        for impl in d.spec?.implementations ?? [] {
            let doc = parseYAML(yamlText(impl.data))
            for dep in pairs(doc.root?.child("deployments")) {
                for cfg in pairs(dep.value.child("configuration")) { out.insert(cfg.key.value) }
            }
        }
        return (out, !out.isEmpty)
    }

    /// Why a package marked ready does not meet QA's entry criterion, in
    /// the order of its requirements: a requirement not accepted or without
    /// acceptance criteria, an open question that blocks an element
    /// satisfying one, and the order warned above.
    func notReady(_ r: Record, _ pl: PackagePlan, _ misordered: Bool) -> [String] {
        var reasons: [String] = []
        for item in items(r.root?.child("requirements")) {
            let req = str(item)
            guard let node = d.requirements[req] else { continue }
            let st = str(node.child("status"))
            if st != "accepted" {
                reasons.append("requirement \(req) is \(st.isEmpty ? "without a status" : st), not accepted")
            }
            if node.child("acceptance") == nil {
                reasons.append("requirement \(req) has no acceptance criteria")
            }
            for element in pl.satisfied[req] ?? [] {
                for q in pairs(d.root.child("questions")) {
                    for b in items(q.value.child("blocks")) {
                        guard let bl = parseBlock(b.value), bl.isPointer else { continue }
                        let at = pointer(bl.tokens)
                        if at == element || at.hasPrefix(element + "/") {
                            appendOnce(&reasons, "question \(q.key.value) is open on \(element), which meets requirement \(req)")
                            break
                        }
                    }
                }
            }
        }
        if misordered {
            reasons.append("it comes before a package that builds what it needs (work_package_order)")
        }
        return reasons
    }

    /// The golden and red tests that verify a requirement of the package
    /// and have no passed result in any test-run record, in the order of
    /// the tests; a test marked not applicable is not run, so it is left
    /// out.
    func unverified(_ r: Record, _ recs: [Record]) -> [String] {
        var held = Set<String>()
        for item in items(r.root?.child("requirements")) { held.insert(str(item)) }
        var passed = Set<String>()
        for run in recs where run.kind == "test-run" {
            for res in items(run.root?.child("results")) where str(res.child("outcome")) == "passed" {
                passed.insert(str(res.child("test")))
            }
        }
        var out: [String] = []
        for t in pairs(d.root.child("tests")) {
            let sc = str(t.value.child("scenario"))
            if sc != "golden" && sc != "red" || t.value.child("notApplicable") != nil || passed.contains(t.key.value) { continue }
            if items(t.value.child("verifies")).contains(where: { held.contains($0.value) }) {
                out.append(t.key.value)
            }
        }
        return out
    }
}
