import Foundation

// Scenarios (ADR-251): cases told as a sequence across operations,
// requirements, entity changes and tests. A scenario with a problem is
// still read whole: every problem is listed at its entry, none stops the
// rest. A requirement of priority must that no step names is warned of
// once the specification has a scenario, so a specification without
// scenarios reads as it did; info.scenarioCoverage error makes it an
// error, scenarios or none.

/// The message of scenario_uncovered, the same in both validators, and
/// what it adds when info.scenarioCoverage makes it an error.
func uncoveredMessage(_ id: String) -> String {
    "requirement \(id) has priority must and no step of any scenario names it, so no case shows it in sequence with the operations it runs through; add \(id) to the requirements of the scenario step that exercises it, or write a scenario for it"
}
let uncoveredStrict = "(info.scenarioCoverage is error)"

/// Whether info.scenarioCoverage makes an uncovered must requirement an
/// error.
func scenarioCoverageError(_ root: YNode) -> Bool {
    str(root.child("info")?.child("scenarioCoverage")) == "error"
}

/// The requirements of priority must, neither rejected nor retired nor
/// held elsewhere, that no step of any scenario names, in document order;
/// none while the specification has no scenario, unless
/// info.scenarioCoverage is error.
func uncoveredMust(_ root: YNode) -> [String] {
    if root.child("scenarios") == nil && !scenarioCoverageError(root) { return [] }
    var named = Set<String>()
    for p in pairs(root.child("scenarios")) {
        for st in items(p.value.child("steps")) {
            for r in items(st.child("requirements")) where !str(r).isEmpty { named.insert(str(r)) }
        }
    }
    var out: [String] = []
    for p in pairs(root.child("requirements")) {
        if str(p.value.child("priority")) != "must" || !str(p.value.child("ownedBy")).isEmpty { continue }
        let status = str(p.value.child("status"))
        if status == "rejected" || status == "retired" { continue }
        if !named.contains(p.key.value) { out.append(p.key.value) }
    }
    return out
}

extension Checker {
    /// Checks each scenario's stakeholder, steps and the names its steps
    /// give, each test's proves, and the must requirements no step names.
    func checkScenarios(_ d: Design) {
        var tests: [String: YNode] = [:], scenarios: [String: YNode] = [:], operations: [String: YNode] = [:]
        for p in pairs(d.root.child("tests")) { tests[p.key.value] = p.value }
        for p in pairs(d.root.child("scenarios")) { scenarios[p.key.value] = p.value }
        for (id, o) in d.operations { operations[id] = o.node }
        let prefixes = d.requirementSetPrefixes()
        for p in pairs(d.root.child("scenarios")) {
            let id = p.key.value
            if let sh = p.value.child("stakeholder"), !str(sh).isEmpty, d.stakeholders[str(sh)] == nil {
                add(sh, pointer("scenarios", id, "stakeholder"), .stakeholder,
                    "\(str(sh)) is not a stakeholder of the specification\(suggest(str(sh), d.stakeholders))")
            }
            let steps = items(p.value.child("steps"))
            if steps.isEmpty {
                add(p.value.child("steps") ?? p.key, pointer("scenarios", id), .scenario,
                    "scenario \(id) has no step, so it shows no case; add steps, each naming the operation, requirements, entity or tests it exercises")
                continue
            }
            for (i, st) in steps.enumerated() {
                let at = ["scenarios", id, "steps", "\(i)"]
                if let a = st.child("actor"), !str(a).isEmpty, d.stakeholders[str(a)] == nil {
                    add(a, pointer(at + ["actor"]), .stakeholder,
                        "\(str(a)) is not a stakeholder of the specification\(suggest(str(a), d.stakeholders))")
                }
                var named = false
                if let op = st.child("operation") {
                    named = true
                    if !str(op).isEmpty && operations[str(op)] == nil {
                        add(op, pointer(at + ["operation"]), .operation,
                            "\(str(op)) is not an operationId of the specification\(suggest(str(op), operations))")
                    }
                }
                for (j, r) in items(st.child("requirements")).enumerated() {
                    named = true
                    checkRequirementLink(d, prefixes, r, pointer(at + ["requirements", "\(j)"]))
                }
                if let e = st.child("entity") {
                    named = true
                    if !str(e).isEmpty && d.entities[str(e)] == nil {
                        add(e, pointer(at + ["entity"]), .scenario,
                            "\(str(e)) is not an entity of the specification\(suggest(str(e), d.entities))")
                    }
                }
                for (j, t) in items(st.child("tests")).enumerated() {
                    named = true
                    if !str(t).isEmpty && tests[str(t)] == nil {
                        add(t, pointer(at + ["tests", "\(j)"]), .scenario,
                            "\(str(t)) is not a test of the specification\(suggest(str(t), tests))")
                    }
                }
                if !named {
                    add(st, pointer(at), .scenario,
                        "step \(i + 1) of scenario \(id) names no operation, requirements, entity or tests, so nothing ties it to the design; name what the step exercises")
                }
            }
        }
        for t in pairs(d.root.child("tests")) {
            for (i, s) in items(t.value.child("proves")).enumerated() where !str(s).isEmpty && scenarios[str(s)] == nil {
                add(s, pointer("tests", t.key.value, "proves", "\(i)"), .scenario,
                    "\(str(s)) is not a scenario of the specification\(suggest(str(s), scenarios))")
            }
        }
        let strict = scenarioCoverageError(d.root)
        let uncovered = Set(uncoveredMust(d.root))
        for p in pairs(d.root.child("requirements")) where uncovered.contains(p.key.value) {
            let id = p.key.value
            if strict {
                add(p.key, pointer("requirements", id), .scenarioUncovered, uncoveredMessage(id) + " " + uncoveredStrict)
            } else {
                warn(p.key, pointer("requirements", id), .scenarioUncovered, uncoveredMessage(id))
            }
        }
    }
}
