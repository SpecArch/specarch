import Foundation

// The intent of the requirements stage (ADR-244): the problem, the goals,
// the out-of-scope entries and the quality goals. Nothing here refuses a
// specification for missing intent, and need_without_goal stays silent
// until a goal is written, so a specification without intent reads as it
// did, with problem_missing as its one new warning.

/// The message of problem_missing, the same in both validators.
let problemMissingMessage = "the specification has requirements and no problem, so nothing says what the system is for, and no goal or requirement can be judged against it; add problem with statement, the problem the system solves in plain words, and, when they are known, the stakeholders it hurts, how things stand today and why"

extension Design {
    /// Whether the specification has requirements and no problem, on a
    /// meta-model whose schema has the key, with no open must question
    /// blocking the problem.
    var problemMissing: Bool {
        !requirements.isEmpty && root.child("problem") == nil && !mustBlocked("problem") && !MetaModelFormat.design.isEarlier(metaModelVersion)
    }
}

extension Checker {
    /// Checks the references of the intent sections and the six warnings
    /// of ADR-242.
    func checkIntent(_ d: Design) {
        var goals: [String: YNode] = [:], qualityGoals: [String: YNode] = [:]
        for p in pairs(d.root.child("goals")) { goals[p.key.value] = p.value }
        for p in pairs(d.root.child("qualityGoals")) { qualityGoals[p.key.value] = p.value }
        for (i, sh) in items(d.root.child("problem")?.child("stakeholders")).enumerated() where !sh.value.isEmpty && d.stakeholders[sh.value] == nil {
            add(sh, pointer("problem", "stakeholders", "\(i)"), .stakeholder,
                "\(sh.value) is not a stakeholder of the specification\(suggest(sh.value, d.stakeholders))")
        }
        for (name, g) in goals {
            for (i, m) in items(g.child("measuredBy")).enumerated() where !m.value.isEmpty && d.monitors[m.value] == nil {
                add(m, pointer("goals", name, "measuredBy", "\(i)"), .monitor,
                    "\(m.value) is not a monitor of the specification\(suggest(m.value, d.monitors))")
            }
        }
        for p in pairs(d.root.child("outOfScope")) {
            for (i, n) in items(p.value.child("needs")).enumerated() where !n.value.isEmpty && d.needs[n.value] == nil {
                add(n, pointer("outOfScope", p.key.value, "needs", "\(i)"), .need,
                    "\(n.value) is not a need of the specification\(suggest(n.value, d.needs))")
            }
        }
        for (name, q) in qualityGoals {
            for (i, r) in items(q.child("requirements")).enumerated() where !r.value.isEmpty && d.requirements[r.value] == nil {
                add(r, pointer("qualityGoals", name, "requirements", "\(i)"), .requirement,
                    "\(r.value) is not a requirement of the specification\(suggest(r.value, d.requirements))")
            }
        }
        for (name, need) in d.needs {
            for (i, g) in items(need.child("goals")).enumerated() where !g.value.isEmpty && goals[g.value] == nil {
                add(g, pointer("needs", name, "goals", "\(i)"), .goal,
                    "\(g.value) is not a goal of the specification\(suggest(g.value, goals))")
            }
        }
        for (name, dec) in d.decisions {
            for (i, q) in items(dec.child("weighs")).enumerated() where !q.value.isEmpty && qualityGoals[q.value] == nil {
                add(q, pointer("decisions", name, "weighs", "\(i)"), .qualityGoal,
                    "\(q.value) is not a quality goal of the specification\(suggest(q.value, qualityGoals))")
            }
        }

        if d.problemMissing && !d.lean {
            warn(d.root.child("requirements"), "/requirements", .problemMissing, problemMissingMessage)
        }
        let firstGoal = pairs(d.root.child("goals")).first?.key.value ?? ""
        var served = Set<String>()
        for p in pairs(d.root.child("needs")) where str(p.value.child("status")) == "accepted" {
            let named = items(p.value.child("goals"))
            for g in named { served.insert(g.value) }
            if !goals.isEmpty && named.isEmpty {
                warn(p.key, pointer("needs", p.key.value), .needWithoutGoal,
                     "need \(p.key.value) names no goal it serves, while the specification has goals; add goals with the IDs of the goals it serves, such as goals: [\(firstGoal)], or set the need's status to rejected when it serves none")
            }
        }
        for p in pairs(d.root.child("goals")) where str(p.value.child("status")) == "accepted" {
            let id = p.key.value
            let ptr = pointer("goals", id)
            let missing = ["measure", "target"].filter { p.value.child($0) == nil }
            if !missing.isEmpty {
                warn(p.key, ptr, .goalUnmeasured,
                     "goal \(id) is accepted with no \(missing.joined(separator: " and no ")), so it cannot be met or missed; add measure, what is counted, and target, the value wanted, each a plain sentence with a number and its unit")
            }
            if !served.contains(id) {
                warn(p.key, ptr, .goalUnserved,
                     "no accepted need serves goal \(id), so nothing the stakeholders asked for leads to it; add goals: [\(id)] to the need it answers, or set the goal's status to dropped")
            }
        }
        var ruledOut: [String: [String]] = [:]
        for p in pairs(d.root.child("outOfScope")) {
            for n in items(p.value.child("needs")) { ruledOut[n.value, default: []].append(p.key.value) }
        }
        for p in pairs(d.root.child("requirements")) where str(p.value.child("status")) == "accepted" {
            for (i, n) in items(p.value.child("needs")).enumerated() {
                for out in ruledOut[n.value] ?? [] {
                    warn(n, pointer("requirements", p.key.value, "needs", "\(i)"), .scopeConflict,
                         "requirement \(p.key.value) is accepted and refines need \(n.value), which out-of-scope entry \(out) rules out; set the requirement's status to rejected, or, when the boundary has moved, take \(n.value) out of the needs of \(out)")
                }
            }
        }
        var byRank: [String: [String]] = [:]
        for p in pairs(d.root.child("qualityGoals")) {
            if let r = p.value.child("rank"), !r.value.isEmpty { byRank[r.value, default: []].append(p.key.value) }
        }
        for p in pairs(d.root.child("qualityGoals")) {
            guard let r = p.value.child("rank"), let ids = byRank[r.value], ids.count > 1 else { continue }
            warn(r, pointer("qualityGoals", p.key.value, "rank"), .qualityGoalRank,
                 "quality goals \(joinAnd(ids)) share rank \(r.value), so the order does not decide a trade-off between them; give each its own rank")
        }
    }
}
