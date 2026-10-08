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
                    if o.node.child("responses")?.child("202") == nil {
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
                guard str(st.child("kind")) == "approval" else { continue }
                checkTimeout(st.child("deadline"), pointer(at + ["deadline"]), .workflow, "the approval")
                let perm = st.child("permission")
                if let perm, !triggerPerm.isEmpty, perm.value == triggerPerm {
                    add(perm, pointer(at + ["permission"]), .workflow, "\(perm.value) is the permission of the trigger too, so whoever may ask may also approve; give the approval a permission of its own")
                }
                if let perm, !triggerPerm.isEmpty, perm.value != triggerPerm {
                    checkWorkflowSeparation(d, perm, pointer(at + ["permission"]), triggerPerm)
                }
                for (j, r) in items(st.child("approvers")).enumerated() {
                    let ptr = pointer(at + ["approvers", "\(j)"])
                    if let role = d.roles[r.value] {
                        if let perm, d.permissions[perm.value] != nil, !grants(role, perm.value) {
                            add(r, ptr, .workflow, "\(r.value) does not grant \(perm.value), which the approval checks; grant it to the role, or name another approver")
                        }
                    } else {
                        add(r, ptr, .workflow, "\(r.value) is not a role of the specification\(suggest(r.value, d.roles))")
                    }
                }
                if let to = st.child("escalateTo"), !laterApproval(Array(steps[(i + 1)...]), to.value) {
                    add(to, pointer(at + ["escalateTo"]), .workflow, "\(to.value) is not an approval step after \(str(stepName)); escalate to a later approval of the workflow")
                }
            }
        }
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
            for role in pairs(d.root.child("roles")) where grants(role.value, triggerPerm) && grants(role.value, perm.value) {
                add(perm, ptr, .workflow, "role \(role.key.value) grants both \(triggerPerm), the trigger's permission, and \(perm.value), and set \(set.key.value) keeps them apart; take one of them from the role")
            }
        }
    }
}

/// Whether a role grants a permission.
private func grants(_ role: YNode, _ perm: String) -> Bool {
    items(role.child("permissions")).contains { $0.value == perm }
}

/// Whether one of the steps is an approval of that name.
private func laterApproval(_ steps: [YNode], _ name: String) -> Bool {
    steps.contains { str($0.child("kind")) == "approval" && str($0.child("name")) == name }
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
                let perm = str(st.child("permission"))
                if !perms.contains(perm) { perms.append(perm) }
            case "operation":
                operations.append(str(st.child("operation")))
            default:
                break
            }
        }
        var when = "a request is made through " + trigger
        if !approvals.isEmpty { when += " and is approved at " + joinAnd(approvals) }
        var then = "it answers 202, and the request ends approved"
        if !operations.isEmpty { then = "it answers 202, then " + joinAnd(operations) + " is called, and the request ends approved" }
        let perm = str(self.operations[trigger]?.node.child("permission"))
        s.success = DerivedCase(name: "succeeds", scenario: "golden", given: perm.isEmpty || perm == "public" ? "any caller" : "a caller with " + perm,
                                when: when, then: then, frequency: "")
        var stopped = "the request ends refused"
        if !operations.isEmpty { stopped += " and " + joinAnd(operations) + " is not called" }
        for st in items(p.value.child("steps")) where str(st.child("kind")) == "approval" {
            let step = str(st.child("name"))
            let perm = str(st.child("permission"))
            let waiting = "a request waiting at " + step
            s.red("refused at " + step, frequent, waiting, "a caller with " + perm + " refuses it", stopped)
            let passed = str(st.child("onDeadline")) == "escalate" ? "the request moves on to " + str(st.child("escalateTo")) : stopped
            s.byNature("deadline passes at " + step, waiting, str(st.child("deadline")) + " passes with no answer", passed)
            s.red("approval without " + perm, frequent, "a caller without " + perm, "they approve a request waiting at " + step, "it is refused as not allowed, and the request still waits")
        }
        if !approvals.isEmpty {
            s.byNature("requester approves own request", "a request made by a caller who also holds " + joinAnd(perms), "the requester approves it",
                       "it is refused, because the person who made a request never approves it, and the request still waits")
        }
        return s
    }
}
