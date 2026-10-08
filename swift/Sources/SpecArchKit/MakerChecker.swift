import Foundation

// Maker-checker on a page (docs/meta-model-0.2.md): the page that makes a
// request a workflow approves, the inbox of an approval step, and the
// messages a workflow publishes when it ends.

extension Design {
    /// Each operation that starts a workflow, with the first workflow it
    /// starts.
    func workflowTriggers() -> [String: String] {
        var triggers: [String: String] = [:]
        for w in pairs(root.child("workflows")) {
            let t = str(w.value.child("trigger"))
            if !t.isEmpty && triggers[t] == nil { triggers[t] = w.key.value }
        }
        return triggers
    }

    /// A form that submits to a workflow's trigger is answered 202 and
    /// shows the message that the request waits. A task page has it
    /// already, as its case for the 202 answer.
    func pendingCase(_ s: Subject, _ pg: YNode) {
        guard let wf = workflowTriggers()[str(pg.child("submit"))], str(pg.child("kind")) == "form" else { return }
        var then = "it is answered 202 and the request waits for approval in workflow " + wf
        let ev = pendingEvent(pg)
        let nav = str(ev?.child("navigate"))
        if !nav.isEmpty { then += ", leads to the page " + nav }
        let m = str(ev?.child("message"))
        if !m.isEmpty { then += ", saying: " + m }
        s.cases.append(DerivedCase(name: "sent for approval", scenario: "golden", given: "...", when: "the form is submitted with every field valid", then: then, frequency: frequent))
    }
}

/// The event a page that submits to a workflow's trigger raises on the 202
/// answer: a form's one onSubmitted, a task's under 202.
func pendingEvent(_ pg: YNode) -> YNode? {
    let ev = pg.child("onSubmitted")
    if str(pg.child("kind")) == "task" { return ev?.child("202") }
    return ev
}

extension Checker {
    /// Checks that a form or task page submitting to a workflow's trigger
    /// acts on its 202 answer with a message, that an inbox is a list of
    /// the workflow's subject checking its approval step's permission, and
    /// that the messages a workflow publishes exist.
    func checkMakerChecker(_ d: Design) {
        var workflows: [String: YNode] = [:]
        for w in pairs(d.root.child("workflows")) {
            workflows[w.key.value] = w.value
            for (i, e) in items(w.value.child("emits")).enumerated() where d.message(e.value) == nil {
                add(e, pointer("workflows", w.key.value, "emits", "\(i)"), .emits, "\(e.value) does not name a channel and one of its messages; write channel/Message for a message declared under channels")
            }
        }
        let triggers = d.workflowTriggers()
        for p in pairs(d.root.child("pages")) {
            let name = p.key.value, pg = p.value
            let kind = str(pg.child("kind"))
            let submit = str(pg.child("submit"))
            if let wf = triggers[submit], kind == "form" || kind == "task" {
                let under = kind == "task" ? " under 202" : ""
                if let ev = pendingEvent(pg) {
                    if ev.child("message") == nil {
                        let ptr = kind == "task" ? pointer("pages", name, "onSubmitted", "202") : pointer("pages", name, "onSubmitted")
                        add(ev, ptr, .workflow, "\(name) submits to \(submit), whose 202 answer leaves the request waiting for approval in workflow \(wf); give the event a message that says so")
                    }
                } else {
                    add(pg.key("onSubmitted") ?? pg.key("submit"), pointer("pages", name, "onSubmitted"), .workflow, "\(name) submits to \(submit), which starts workflow \(wf) and answers 202 while the request waits for approval; add onSubmitted\(under) with the message that says so")
                }
            }
            guard let inbox = pg.child("inbox") else { continue }
            let base = ["pages", name, "inbox"]
            if kind != "list" {
                add(pg.key("inbox"), pointer(base), .workflow, "\(name) is a \(kind) page, and an inbox is a list of the requests waiting at an approval; leave inbox out")
                continue
            }
            let wn = inbox.child("workflow")
            guard let w = workflows[str(wn)] else {
                add(wn, pointer(base + ["workflow"]), .workflow, "\(str(wn)) is not a workflow of the specification\(suggest(str(wn), workflows))")
                continue
            }
            let sn = inbox.child("step")
            var approvals: [String: YNode] = [:]
            for st in items(w.child("steps")) where str(st.child("kind")) == "approval" {
                approvals[str(st.child("name"))] = st
            }
            guard let step = approvals[str(sn)] else {
                add(sn, pointer(base + ["step"]), .workflow, "\(str(sn)) is not an approval step of workflow \(str(wn))\(suggest(str(sn), approvals))")
                continue
            }
            let subject = str(w.child("subject"))
            if let ent = pg.child("entity"), ent.value != subject {
                add(ent, pointer("pages", name, "entity"), .workflow, "\(name) is the inbox of workflow \(str(wn)), whose requests wait as \(subject); list \(subject)")
            }
            let perm = str(step.child("permission"))
            if let pn = pg.child("permission"), pn.value != perm {
                add(pn, pointer("pages", name, "permission"), .workflow, "\(name) is the inbox of step \(str(sn)) of workflow \(str(wn)), which checks \(perm); give the page that permission, so it opens to whoever may approve")
            }
        }
    }
}
