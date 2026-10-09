import Foundation

extension Checker {
    /// Checks the open questions: who decides is a stakeholder, every block
    /// names a stage, a section or an element of the specification, a
    /// question sits in the stage of what it blocks, and no accepted
    /// decision answers a question that is still open.
    func checkQuestions(_ d: Design) {
        for (id, q) in d.questions {
            let ptr = pointer("questions", id)
            if let by = q.child("decidedBy"), !by.value.isEmpty, d.stakeholders[by.value] == nil {
                add(by, ptr + "/decidedBy", .stakeholder, "\(by.value) is not a stakeholder of the specification\(suggest(by.value, d.stakeholders))")
            }
            var stageList: [String] = []
            let blocks = items(q.child("blocks"))
            for (i, item) in blocks.enumerated() {
                guard let b = checkBlock(d, item, "\(ptr)/blocks/\(i)") else { continue }
                if let names = q.child("names"), blocks.count == 1 {
                    checkNames(d, id, names, b)
                }
                if !stageList.contains(b.stage) { stageList.append(b.stage) }
            }
            if let names = q.child("names"), blocks.count != 1 {
                add(names, ptr + "/names", .questionBlock, "question \(id) names \(names.value) for one key, and blocks \(blocks.count) entries; block only the key the name is for, such as #/workflows/<name>/trigger")
            }
            let key = d.root.child("questions")?.key(id)
            if stageList.count > 1 {
                add(key, ptr, .questionStage, "question \(id) blocks the \(joinAnd(stageList)) stages; a question is about one stage, so split it")
            } else if stageList.count == 1, let spec = d.spec {
                let want = stageList[0]
                let at = spec.stageOfFile(fileOf(key))
                if at.isEmpty && spec.listed(want) {
                    add(key, ptr, .questionStage, "question \(id) is about the \(want) stage, which has a folder; move it to \(want)/")
                } else if !at.isEmpty && at != want {
                    add(key, ptr, .questionStage, "question \(id) sits under \(at)/ but is about the \(want) stage; move it to \(want)/")
                }
            }
        }
        for (id, dec) in d.decisions where str(dec.child("status")) == "accepted" {
            for (i, a) in items(dec.child("answers")).enumerated() where d.questions[a.value] != nil {
                add(a, pointer("decisions", id, "answers", "\(i)"), .questionAnswered,
                    "\(id) answers question \(a.value), but \(a.value) is still open; remove the question now that it is answered, or set the decision's status to proposed")
            }
        }
    }

    /// Checks that a question naming what the source gives blocks one key,
    /// and that the specification leaves that key out.
    func checkNames(_ d: Design, _ id: String, _ names: YNode, _ b: Block) {
        let ptr = pointer("questions", id, "names")
        if b.key.isEmpty {
            add(names, ptr, .questionBlock, "question \(id) names \(names.value) for one key, and blocks #\(pointer(b.tokens)), which is not a key; block the key the name is for, such as #/workflows/<name>/trigger")
            return
        }
        if resolve(d.root, b.tokens).1 {
            add(names, ptr, .questionBlock, "question \(id) names \(names.value) for #\(pointer(b.tokens)), which the specification gives already; leave the key out until the name resolves, or remove the question")
        }
    }

    /// Checks one entry of a question's blocks and returns it parsed.
    func checkBlock(_ d: Design, _ item: YNode, _ ptr: String) -> Block? {
        let text = item.value
        guard let b = parseBlock(text) else {
            if text.hasPrefix("#/") {
                add(item, ptr, .questionBlock, "\(text) does not point into a section; a pointer is #/<section>/<name>, such as #/entities/Loan, with at most one key after them")
            } else {
                add(item, ptr, .questionBlock, "\(text) is neither a stage, a section nor a #/ pointer to an element; name the stage (\(stages.joined(separator: ", "))), a section, or #/<section>/<name>")
            }
            return nil
        }
        if !b.isPointer { return b }
        let section = d.root.child(b.tokens[0])
        guard let element = section?.child(b.tokens[1]) else {
            var valid: [String: YNode] = [:]
            for p in pairs(section) { valid[p.key.value] = p.value }
            var fix = suggest(b.tokens[1], valid)
            if !fix.hasPrefix("; did you mean") {
                fix = "; write the element, down to an empty mapping when only its name is known, or block the section \(b.tokens[0])"
            }
            add(item, ptr, .questionBlock, "\(text) does not point at anything in the specification\(fix)")
            return nil
        }
        var n = element
        let rest = Array(b.tokens.dropFirst(2))
        for (i, t) in rest.enumerated() {
            guard let next = n.child(t) else {
                if i == rest.count - 1 { return b } // the one missing key the question is about
                add(item, ptr, .questionBlock, "\(text) points below a key that does not exist; name the element, or one missing key of it")
                return nil
            }
            n = next
        }
        return b
    }

    /// Checks every element that says how it is known: stated cites
    /// something, inferred says why, decided names an accepted decision,
    /// and decidedIn goes only with decided. decisions are the records a
    /// decidedIn may name; isDecision tells whether a path is a decision
    /// record itself, which nothing decides.
    func checkOrigin(_ root: YNode?, _ decisions: [String: YNode], _ isDecision: ([String]) -> Bool) {
        walk(root, []) { n, path in
            let origin = n.child("origin")
            let decidedIn = n.child("decidedIn")
            if origin == nil && decidedIn == nil { return }
            let ptr = pointer(path)
            let kind = str(origin)
            switch kind {
            case "stated":
                if items(n.child("cites")).isEmpty {
                    add(origin, ptr + "/origin", .originCitation, "origin is stated but nothing is cited; add cites with the source and where in it this is stated")
                }
            case "inferred":
                if str(n.child("why")).trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                    add(origin, ptr + "/origin", .originReason, "origin is inferred but why is missing; say from what evidence this was concluded")
                }
            case "decided":
                if isDecision(path) {
                    add(origin, ptr + "/origin", .originDecision, "a decision is not decided by another decision; set supersededBy on the one it replaces, or state or infer it")
                } else if decidedIn == nil {
                    add(origin, ptr + "/origin", .originDecision, "origin is decided but decidedIn is missing; name the decision that settled this")
                } else if let decidedIn, decisions[decidedIn.value] == nil {
                    add(decidedIn, ptr + "/decidedIn", .originDecision, "\(decidedIn.value) is not a decision of the specification\(suggest(decidedIn.value, decisions))")
                } else if let decidedIn, str(decisions[decidedIn.value]?.child("status")) != "accepted" {
                    add(decidedIn, ptr + "/decidedIn", .originDecision, "\(decidedIn.value) is \(str(decisions[decidedIn.value]?.child("status"))), not accepted; an element rests only on an accepted decision")
                }
            default:
                break
            }
            if let decidedIn, kind != "decided", !kind.isEmpty {
                add(decidedIn, ptr + "/decidedIn", .originDecision, "decidedIn is set but origin is \(kind); set origin: decided, or remove decidedIn")
            }
            if let decidedIn, kind.isEmpty, origin == nil {
                add(decidedIn, ptr + "/decidedIn", .originDecision, "decidedIn is set but origin is missing; set origin: decided, or remove decidedIn")
            }
        }
    }

    /// Reports, when the root file says the specification tracks origin,
    /// every element of a section that carries none.
    func checkOriginTracked(_ d: Design) {
        guard str(child(d.root.child("info"), "tracksOrigin")) == "true" else { return }
        for p in d.root.pairs {
            let name = p.key.value
            guard sections[name] != nil else { continue }
            switch name {
            case "release", "rollback", "signoff":
                if p.value.child("origin") == nil {
                    warn(p.key, pointer(name), .originMissing, "\(name) carries no origin, and the specification tracks origin; add origin: stated, inferred or decided")
                }
            case "paths":
                for o in d.opList where o.node.child("origin") == nil {
                    warn(o.pathItem.key(o.method), o.pointer(), .originMissing, "operation \(o.id) carries no origin, and the specification tracks origin; add origin: stated, inferred or decided")
                }
            default:
                for e in pairs(p.value) where e.value.kind == .mapping && e.value.child("origin") == nil {
                    warn(e.key, pointer(name, e.key.value), .originMissing, "\(e.key.value) carries no origin, and the specification tracks origin; add origin: stated, inferred or decided")
                }
            }
        }
    }
}

func joinAnd(_ items: [String]) -> String {
    switch items.count {
    case 0: return ""
    case 1: return items[0]
    default: return items.dropLast().joined(separator: ", ") + " and " + items[items.count - 1]
    }
}

/// Splits the diagnostics into the ones to report and the ones an open must
/// question covers: a required key missing at or under a pointer the
/// question blocks, the warnings about that element, the problem a response
/// must name once problem types are declared when the question blocks that
/// problem, and a permission no role grants when the question blocks that
/// permission. A pointer to an
/// element known only by name, an empty mapping such as a field with
/// nothing but its name, covers everything under it. A wrong value next to
/// the gap stays an error.
func coveredByQuestions(_ ds: [Diagnostic], _ root: YNode) -> (kept: [Diagnostic], covered: [Diagnostic]) {
    var elements: [String] = []
    var keys: [String: [String]] = [:]
    for p in pairs(root.child("questions")) where str(p.value.child("priority")) == "must" {
        for item in items(p.value.child("blocks")) {
            guard let b = parseBlock(item.value), b.isPointer else { continue }
            let k = b.key
            if !k.isEmpty {
                keys[pointer(Array(b.tokens.dropLast())), default: []].append(k)
                let (n, reached) = resolve(root, b.tokens)
                if reached && n.kind == .mapping && n.pairs.isEmpty {
                    elements.append(pointer(b.tokens))
                }
            } else {
                elements.append(b.element)
            }
        }
    }
    if elements.isEmpty && keys.isEmpty { return (ds, []) }
    func under(_ path: String) -> Bool {
        elements.contains { path == $0 || path.hasPrefix($0 + "/") }
    }
    var kept: [Diagnostic] = [], covered: [Diagnostic] = []
    let suffix = " is missing; add it here"
    for d in ds {
        var isCovered = false
        if d.rule == .schema && d.message.hasSuffix(suffix) {
            let missing = String(d.message.dropLast(suffix.count))
            isCovered = under(d.path) || (keys[d.path] ?? []).contains(missing)
        } else if d.severity == .warning {
            isCovered = under(d.path) || (d.rule == .acceptanceMissing && (keys[d.path] ?? []).contains("acceptance"))
        } else if d.rule == .problem && d.message.hasSuffix(" names one under problem") {
            // A response whose problem the source does not name, while the
            // specification declares problem types: the problem is the key
            // missing, and a question on it says it is not known.
            isCovered = under(d.path) || (keys[d.path] ?? []).contains("problem")
        } else if d.rule == .permissionUngranted {
            // The grant is what is missing, and it is written in a role,
            // not under the permission; a question on the permission
            // itself is the one place that can say it is not known.
            isCovered = under(d.path)
        }
        if isCovered { covered.append(d) } else { kept.append(d) }
    }
    return (kept, covered)
}

/// Counts the open questions of a specification.
func openQuestions(_ root: YNode?) -> Int {
    pairs(root?.child("questions")).count
}
