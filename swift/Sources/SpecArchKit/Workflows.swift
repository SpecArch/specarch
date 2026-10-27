import Foundation

// Workflows: requests that finish after people approve them, a sequential
// subset of BPMN 2.0 (docs/meta-model-0.2.md, Workflows).

extension Checker {
    /// Checks that a workflow starts from an operation that answers 202,
    /// holds its request in an entity, and that its steps name roles that
    /// grant the approval's permission, operations that exist and later
    /// approvals to escalate to, each step under its own name.
    func checkWorkflows(_ d: Design) {
        var opNames: [String: YNode] = [:]
        for (id, o) in d.operations { opNames[id] = o.node }
        for w in pairs(d.root.child("workflows")) {
            let name = w.key.value
            var triggerPerm = ""
            if let t = w.value.child("trigger") {
                if let o = d.operations[t.value] {
                    // With no responses at all, the operation's answers are
                    // not known yet, which the missing key itself reports.
                    if let responses = o.node.child("responses"), responses.child("202") == nil {
                        add(t, pointer("workflows", name, "trigger"), .workflow, "operation \(t.value) does not answer 202; a workflow's trigger accepts the request and answers 202 while it waits for approval")
                    } else {
                        triggerPerm = str(o.node.child("permission"))
                    }
                } else {
                    add(t, pointer("workflows", name, "trigger"), .workflow, "\(t.value) is not an operationId of the specification\(suggest(t.value, opNames))")
                }
            }
            if let s = w.value.child("subject"), d.entities[s.value] == nil {
                add(s, pointer("workflows", name, "subject"), .workflow, "\(s.value) is not an entity of the specification\(suggest(s.value, d.entities))")
            }
            let steps = items(w.value.child("steps"))
            let env = d.subjectEnv(w.value)
            var seen = Set<String>()
            for (i, st) in steps.enumerated() {
                let at = ["workflows", name, "steps", "\(i)"]
                let stepName = st.child("name")
                if let stepName {
                    if seen.contains(stepName.value) {
                        add(stepName, pointer(at + ["name"]), .workflow, "\(stepName.value) names two steps of workflow \(name); give each step its own name")
                    }
                    seen.insert(stepName.value)
                }
                if let op = st.child("operation"), d.operations[op.value] == nil {
                    add(op, pointer(at + ["operation"]), .workflow, "\(op.value) is not an operationId of the specification\(suggest(op.value, opNames))")
                }
                let later = Array(steps[(i + 1)...])
                switch str(st.child("kind")) {
                case "approval":
                    checkTimeout(st.child("deadline"), pointer(at + ["deadline"]), .workflow, "the approval")
                    let perm = st.child("permission")
                    if let perm, !triggerPerm.isEmpty, perm.value == triggerPerm {
                        add(perm, pointer(at + ["permission"]), .workflow, "\(perm.value) is the permission of the trigger too, so whoever may ask may also approve; give the approval a permission of its own")
                    }
                    if let perm, !triggerPerm.isEmpty, perm.value != triggerPerm {
                        checkWorkflowSeparation(d, perm, pointer(at + ["permission"]), triggerPerm)
                    }
                    checkStepRoles(d, st, at, "approvers", "the approval", "approver")
                    if let to = st.child("escalateTo"), !laterStep(later, "approval", to.value) {
                        add(to, pointer(at + ["escalateTo"]), .workflow, "\(to.value) is not an approval step after \(str(stepName)); escalate to a later approval of the workflow")
                    }
                case "task":
                    checkTimeout(st.child("deadline"), pointer(at + ["deadline"]), .workflow, "the task")
                    checkStepRoles(d, st, at, "performers", "the task", "performer")
                    if let to = st.child("escalateTo"), !laterStep(later, "task", to.value) {
                        add(to, pointer(at + ["escalateTo"]), .workflow, "\(to.value) is not a task step after \(str(stepName)); escalate to a later task of the workflow")
                    }
                case "operation":
                    if let comp = st.child("compensatedBy"), d.operations[comp.value] == nil {
                        add(comp, pointer(at + ["compensatedBy"]), .workflow, "\(comp.value) is not an operationId of the specification\(suggest(comp.value, opNames))")
                    }
                    if let to = st.child("onFailure")?.child("to"), !laterStep(later, "", to.value) {
                        add(to, pointer(at + ["onFailure", "to"]), .workflow, "\(to.value) is not a step after \(str(stepName)); a failure goes on at a later step of the workflow, or ends it with end: failed")
                    }
                case "decision":
                    let branches = items(st.child("branches"))
                    for (j, br) in branches.enumerated() {
                        let bat = at + ["branches", "\(j)"]
                        if let env {
                            checkCondition(br.child("when"), pointer(bat + ["when"]), "the branch's condition", env)
                        }
                        if let o = br.child("otherwise"), j != branches.count - 1 {
                            add(o, pointer(bat + ["otherwise"]), .workflow, "otherwise is on branch \(j + 1) of \(branches.count) of \(str(stepName)); only the last branch is taken when none holds, so move it to the end")
                        }
                        if j == branches.count - 1, br.child("otherwise") == nil, let when = br.child("when") {
                            add(when, pointer(bat + ["when"]), .workflow, "the last branch of \(str(stepName)) has a condition, so a request none of them holds for has nowhere to go; write otherwise: true in its place, or add a branch with otherwise")
                        }
                        if let to = br.child("to"), !laterStep(later, "", to.value) {
                            add(to, pointer(bat + ["to"]), .workflow, "\(to.value) is not a step after \(str(stepName)); a branch goes on at a later step of the workflow, or ends it with end")
                        }
                    }
                case "wait":
                    checkTimeout(st.child("deadline"), pointer(at + ["deadline"]), .workflow, "the wait")
                    if let m = st.child("message"), d.message(m.value) == nil {
                        add(m, pointer(at + ["message"]), .workflow, "\(m.value) does not name a channel and one of its messages; write channel/Message for a message declared under channels")
                    }
                    if let moment = st.child("at"), moment.kind == .scalar, let env {
                        let required = items(d.entities[str(w.value.child("subject"))]?.child("required")).map { $0.value }
                        checkMoment(moment, pointer(at + ["at"]), env, required)
                    }
                default:
                    break
                }
            }
        }
    }

    /// Checks that each role an approval or a task names is a role and
    /// grants the permission the step checks.
    func checkStepRoles(_ d: Design, _ st: YNode, _ at: [String], _ key: String, _ what: String, _ who: String) {
        let perm = st.child("permission")
        for (j, r) in items(st.child(key)).enumerated() {
            let ptr = pointer(at + [key, "\(j)"])
            if let role = d.roles[r.value] {
                if let perm, d.permissions[perm.value] != nil, !d.roleHolds(role).contains(perm.value) {
                    add(r, ptr, .workflow, "\(r.value) does not grant \(perm.value), which \(what) checks; grant it to the role, or name another \(who)")
                }
            } else {
                add(r, ptr, .workflow, "\(r.value) is not a role of the specification\(suggest(r.value, d.roles))")
            }
        }
    }

    /// Checks a wait's at: it gives an instant, or a date read in the
    /// specification's timeZone, which must then be declared, and never
    /// null, since a wait with no moment would never end (ADR-193): a
    /// field the subject does not require may be null as much as one whose
    /// type says so.
    func checkMoment(_ n: YNode, _ ptr: String, _ env: ExprEnv, _ required: [String]) {
        let what = "the moment"
        let (tree, perrs) = parseExpr(n.value)
        guard let tree, perrs.isEmpty else {
            exprErrors(n, ptr, what, perrs)
            return
        }
        let (t, errs) = checkExpr(tree, env)
        exprErrors(n, ptr, what, errs)
        guard errs.isEmpty else { return }
        if t.kind != .timestamp && t.kind != .date {
            addFile(fileOf(n), exprLine(n, 1), ptr, .expressionType, "\(what) gives \(t), but a wait's at must give an instant or a date; name a date-time or date field of the subject")
        } else if t.nullable || (tree.op == .name && !required.contains(tree.text)) {
            addFile(fileOf(n), exprLine(n, 1), ptr, .expressionType, "\(what) gives \(t), which may be null, and a request waiting for no moment never goes on; make the field required, or wait for a field that always holds one")
        } else if t.kind == .date && !zoneDeclared {
            addFile(fileOf(n), exprLine(n, 1), ptr, .timeZone, "\(what) gives a date, which is read as the first instant of that day in the specification's timeZone, and the specification declares none; declare timeZone at the root, a name of the IANA Time Zone Database such as Asia/Jakarta")
        } else {
            zoneWarnings(n, ptr, what, tree, env)
        }
    }
}

extension Design {
    /// The names a workflow's conditions and moments read: the fields of
    /// its subject, or nil when the subject is not an entity, which the
    /// subject's own check reports.
    func subjectEnv(_ w: YNode) -> ExprEnv? {
        guard let ent = entities[str(w.child("subject"))] else { return nil }
        var env: ExprEnv = [:]
        for (name, f) in fieldsOf(ent) { env[name] = fieldType(f) }
        return env
    }
}

extension Checker {
    /// Reports each role that grants both the trigger's permission and an
    /// approval's when a separation-of-duties set holds the pair, whatever
    /// the set's cardinality: such a role could make a request and approve
    /// it in one person.
    func checkWorkflowSeparation(_ d: Design, _ perm: YNode, _ ptr: String, _ triggerPerm: String) {
        for set in pairs(d.root.child("separationOfDuties")) {
            let names = items(set.value.child("permissions")).map { $0.value }
            guard names.contains(triggerPerm), names.contains(perm.value) else { continue }
            for role in pairs(d.root.child("roles")) where d.roleHolds(role.value).isSuperset(of: [triggerPerm, perm.value]) {
                add(perm, ptr, .workflow, "role \(role.key.value) grants both \(triggerPerm), the trigger's permission, and \(perm.value), and set \(set.key.value) keeps them apart; take one of them from the role")
            }
        }
    }
}

/// Whether one of the steps has that name and, unless kind is "", that
/// kind.
private func laterStep(_ steps: [YNode], _ kind: String, _ name: String) -> Bool {
    steps.contains { (kind.isEmpty || str($0.child("kind")) == kind) && str($0.child("name")) == name }
}

extension Design {
    /// A workflow's tests: the approved path, a refusal and the deadline at
    /// each approval, the requester approving their own request, and an
    /// approval by someone without its permission.
    func workflowSubject(_ p: Pair) -> Subject {
        let name = p.key.value
        let s = Subject(label: "workflow " + name, node: p.key, path: pointer("workflows", name), yamlKey: "workflow: " + name, name: name)
        let trigger = str(p.value.child("trigger"))
        var approvals: [String] = [], operations: [String] = [], perms: [String] = []
        for st in items(p.value.child("steps")) {
            switch str(st.child("kind")) {
            case "approval":
                approvals.append(str(st.child("name")))
                let perm = stepPermission(st)
                if !perms.contains(perm) { perms.append(perm) }
            case "operation":
                let op = str(st.child("operation"))
                operations.append(op.isEmpty ? "the operation of " + str(st.child("name")) : op)
            default:
                break
            }
        }
        var when = "a request is made through " + (trigger.isEmpty ? "its trigger" : trigger)
        if !approvals.isEmpty { when += " and is approved at " + joinAnd(approvals) }
        var then = "it answers 202, and the request ends approved"
        if !operations.isEmpty { then = "it answers 202, then " + joinAnd(operations) + " is called, and the request ends approved" }
        let perm = str(self.operations[trigger]?.node.child("permission"))
        s.success = DerivedCase(name: "succeeds", scenario: "golden", given: perm.isEmpty || perm == "public" ? "any caller" : "a caller with " + perm,
                                when: when, then: then, frequency: "")
        var stopped = "the request ends refused"
        if !operations.isEmpty { stopped += " and " + joinAnd(operations) + " is not called" }
        let steps = items(p.value.child("steps"))
        for (i, st) in steps.enumerated() {
            let step = str(st.child("name"))
            let waiting = "a request waiting at " + step
            let done = Array(steps[..<i])
            func ended(_ how: String) -> String { "the request ends " + how + undone(done) }
            switch str(st.child("kind")) {
            case "approval":
                let perm = stepPermission(st)
                s.red("refused at " + step, frequent, waiting, "a caller with " + perm + " refuses it", stopped + undone(done))
                let passed = str(st.child("onDeadline")) == "escalate" ? "the request moves on to " + str(st.child("escalateTo")) : stopped + undone(done)
                let deadline = str(st.child("deadline"))
                s.byNature("deadline passes at " + step, waiting, (deadline.isEmpty ? "the deadline" : deadline) + " passes with no answer", passed)
                s.red("approval without " + perm, frequent, "a caller without " + perm, "they approve a request waiting at " + step, "it is refused as not allowed, and the request still waits")
            case "task":
                let perm = stepPermission(st)
                let deadline = str(st.child("deadline"))
                if !deadline.isEmpty {
                    let passed = str(st.child("onDeadline")) == "escalate" ? "the request moves on to " + str(st.child("escalateTo")) : ended("refused")
                    s.byNature("deadline passes at " + step, waiting, deadline + " passes with the task not done", passed)
                }
                s.red("task without " + perm, frequent, "a caller without " + perm, "they do the task of a request waiting at " + step, "it is refused as not allowed, and the request still waits")
            case "wait":
                let deadline = str(st.child("deadline"))
                if !deadline.isEmpty {
                    s.byNature("deadline passes at " + step, waiting + " for " + str(st.child("message")), deadline + " passes with no message", ended("refused"))
                }
            case "decision":
                for br in items(st.child("branches")) {
                    let target = str(br.child("to"))
                    var name = step + " goes on at " + target, then = "the request goes on at " + target
                    let end = str(br.child("end"))
                    if !end.isEmpty { name = step + " ends " + end; then = ended(end) }
                    let given = br.child("otherwise") != nil ? "a request reaching " + step + " for which no other branch holds"
                        : "a request reaching " + step + " for which " + str(br.child("when")) + " holds"
                    s.red(name, frequent, given, "the decision is taken", then)
                }
            case "operation":
                guard let f = st.child("onFailure") else { break }
                var op = str(st.child("operation"))
                if op.isEmpty { op = "the operation of " + step }
                var when = op + " fails"
                if let limit = intValue(f.child("retries")?.child("limit")) {
                    when = limit == 1 ? op + " fails, and on its one retry" : op + " fails, and on each of its \(limit) retries"
                }
                let to = str(f.child("to"))
                s.byNature("operation fails at " + step, "a request reaching " + step, when, to.isEmpty ? ended("failed") : "the request goes on at " + to)
            default:
                break
            }
        }
        if !approvals.isEmpty {
            s.byNature("requester approves own request", "a request made by a caller who also holds " + joinAnd(perms), "the requester approves it",
                       "it is refused, because the person who made a request never approves it, and the request still waits")
        }
        return s
    }

    /// What a request that ends after the steps done says of their
    /// compensation: each operation step with compensatedBy, the latest
    /// first.
    func undone(_ done: [YNode]) -> String {
        let calls = done.reversed().compactMap { st -> String? in
            let comp = str(st.child("compensatedBy"))
            return comp.isEmpty || str(st.child("kind")) != "operation" ? nil : comp + " undoes " + str(st.child("name"))
        }
        return calls.isEmpty ? "" : ", and " + calls.joined(separator: ", then ")
    }

    /// The permission an approval checks, or what names it while a
    /// question holds it open.
    func stepPermission(_ st: YNode) -> String {
        let perm = str(st.child("permission"))
        return perm.isEmpty ? "the permission of " + str(st.child("name")) : perm
    }
}
