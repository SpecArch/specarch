import CryptoKit
import Foundation

// The checks of the records that keep who said what and when: a
// communication, a session, a test session and a deploy (ADR-220).

/// The types of a communication record an element may cite: what a
/// stakeholder said, not what they asked for.
private let citedTypes: Set<String> = ["answer", "information"]

extension RecordChecks {
    /// Checks that every item of a list is a stakeholder.
    func roles(_ r: Record, _ key: String) {
        for (i, item) in items(r.root?.child(key)).enumerated() {
            let v = item.value
            if !v.isEmpty && d.stakeholders[v] == nil {
                r.c.add(item, pointer(key, String(i)), .recordRef, "\(v) is not a stakeholder of the specification\(suggest(v, d.stakeholders))")
            }
        }
    }

    /// Checks that a communication's session is a session or a test
    /// session record.
    func session(_ r: Record) {
        guard let n = r.root?.child("session") else { return }
        let v = str(n)
        if v.isEmpty || has("session", v) || has("test-session", v) { return }
        r.c.add(n, "/session", .recordRef, "\(v) is not a session or a test session: there is no records/sessions/\(v).yaml or records/test-sessions/\(v).yaml")
    }

    /// Checks what a record links: questions open or answered, decisions,
    /// elements, and the change, defect and idea records.
    func checkLinks(_ r: Record) {
        let links = r.root?.child("links")
        if r.str("type") == "answer", let links, links.child("questions") == nil, links.child("changes") == nil {
            r.c.add(links, "/links", .recordRef, "\(r.str("id")) is an answer that links neither a question nor a change request; name the question it answers under links.questions, or the change request it settles under links.changes")
        }
        for (i, item) in items(links?.child("questions")).enumerated() {
            let v = item.value
            if !v.isEmpty && d.questions[v] == nil && !answeredQuestion(v) {
                r.c.add(item, pointer("links", "questions", String(i)), .recordRef, "\(v) is not a question of the specification: it is not open under questions and no decision answers it")
            }
        }
        for (i, item) in items(links?.child("decisions")).enumerated() {
            let v = item.value
            if !v.isEmpty && d.decisions[v] == nil {
                r.c.add(item, pointer("links", "decisions", String(i)), .recordRef, "\(v) is not a decision of the specification\(suggest(v, d.decisions))")
            }
        }
        for (i, item) in items(links?.child("elements")).enumerated() {
            reference(r, item, pointer("links", "elements", String(i)))
        }
        for (key, kind) in [("changes", "change"), ("defects", "defect"), ("ideas", "idea")] {
            for (i, item) in items(links?.child(key)).enumerated() {
                recordID(r, item, pointer("links", key, String(i)), [kind])
            }
        }
    }

    /// Checks that each file a record names as evidence is under records/
    /// and still has the SHA-256 the record holds.
    func checkEvidence(_ r: Record) {
        for (i, item) in items(r.root?.child("evidence")).enumerated() {
            let pathNode = item.child("path")
            let p = str(pathNode)
            if p.isEmpty { continue }
            let ptr = pointer("evidence", String(i), "path")
            let base = URL(fileURLWithPath: dir).standardizedFileURL
            let full = URL(fileURLWithPath: p, relativeTo: base).standardizedFileURL
            let basePath = base.path.hasSuffix("/") ? base.path : base.path + "/"
            if !full.path.hasPrefix(basePath) {
                r.c.add(pathNode, ptr, .evidence, "\(p) is not under records/; evidence lives under records/, named by its path from there")
                continue
            }
            guard let data = FileManager.default.contents(atPath: full.path) else {
                r.c.add(pathNode, ptr, .evidence, "\(p) is not under records/: there is no records/\(p); put the file there, or correct the path")
                continue
            }
            let got = "sha256:" + SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
            let want = str(item.child("sha256"))
            if !want.isEmpty && want != got {
                r.c.add(item.child("sha256"), pointer("evidence", String(i), "sha256"), .evidence, "records/\(p) has changed since it was recorded: its SHA-256 is now \(got), not \(want); restore the file, or record it again as it is now")
            }
        }
    }

    /// Checks every citation that names a communication record: the record
    /// exists, its type is one a citation may rest on, and a decision that
    /// answers questions and cites an answer cites one to one of them, said
    /// by the role that decided; information cited by a decision is held to
    /// neither, since it answers nothing (ADR-220).
    func checkCitedRecords(_ c: Checker) {
        walk(d.root, []) { n, path in
            for (i, item) in items(n.child("cites")).enumerated() {
                let recNode = item.child("record")
                let id = str(recNode)
                if id.isEmpty { continue }
                let ptr = pointer(path + ["cites", "\(i)", "record"])
                guard let rec = set["communication"]?[id] else {
                    c.add(recNode, ptr, .recordCited, "\(id) is not a communication record: there is no records/communications/\(id).yaml; record what was said with specarch log, then cite it")
                    continue
                }
                let typ = rec.str("type")
                if !citedTypes.contains(typ) {
                    c.add(recNode, ptr, .recordCited, "\(id) is \(typeArticle(typ)), which settles nothing by itself: cite the record of the answer given, or the decision it led to; a citation rests on an answer or on information")
                    continue
                }
                if path.count == 2 && path[0] == "decisions" && typ == "answer" {
                    let answers = items(n.child("answers")).map(\.value)
                    if answers.isEmpty { continue }
                    // An answer links the question it answers, or the
                    // change request it settles (ADR-221).
                    let links = rec.root?.child("links")
                    let linked = items(links?.child("questions")).map(\.value) + items(links?.child("changes")).map(\.value)
                    if !answers.contains(where: { linked.contains($0) }) {
                        c.add(recNode, ptr, .recordCited, "\(id) answers \(eitherOf(linked)), not \(eitherOf(answers)), which \(path[1]) answers; cite the record of the answer to \(eitherOf(answers)), or link \(id) to it under questions or changes")
                        continue
                    }
                    let from = rec.str("from")
                    let by = str(n.child("decidedBy"))
                    if from != by {
                        c.add(recNode, ptr, .recordCited, "\(id) was said by \(from), not \(by), which \(path[1]) names as decidedBy; a decision that answers a question is decided by the role that answered it: write decidedBy \(from), or cite the record of the answer \(by) gave")
                    }
                }
            }
        }
    }

    /// Warns of a stakeholder that an accepted requirement needs, or that
    /// an approved specification's page is for, while no communication,
    /// session, interview or decision shows it was ever heard (ADR-220). A
    /// specification that keeps no communication, session or test session
    /// record has heard nobody yet, and the warning says to log the first
    /// conversation.
    func checkUnheard(_ c: Checker) {
        let kept = (set["communication"]?.count ?? 0) + (set["session"]?.count ?? 0) + (set["test-session"]?.count ?? 0)
        let hint = kept == 0 ? "; no communication, session or test session record is kept yet, so start by logging the first conversation" : ""
        var heard: Set<String> = []
        for (_, r) in set["communication"] ?? [:] {
            heard.insert(r.str("from"))
            heard.insert(r.str("to"))
        }
        for kind in ["session", "test-session"] {
            for (_, r) in set[kind] ?? [:] {
                for v in items(r.root?.child("present")) { heard.insert(v.value) }
            }
        }
        for (_, src) in d.sources {
            for v in items(src.child("participants")) { heard.insert(v.value) }
        }
        for (_, dec) in d.decisions {
            heard.insert(str(dec.child("decidedBy")))
        }
        var relies: [String: [String]] = [:] // role -> what relies on it, as named
        func add(_ role: String, _ what: String) {
            if role.isEmpty || heard.contains(role) || d.stakeholders[role] == nil { return }
            if relies[role, default: []].contains(what) { return }
            relies[role, default: []].append(what)
        }
        for id in d.requirements.keys.sorted(by: byteLess) {
            let req = d.requirements[id]!
            if str(req.child("status")) != "accepted" { continue }
            add(str(req.child("ownedBy")), "requirement " + id)
            for needID in items(req.child("needs")).map(\.value) {
                for role in items(d.needs[needID]?.child("stakeholders")).map(\.value) {
                    add(role, "requirement " + id)
                }
            }
        }
        let version = str(d.root.child("info")?.child("version"))
        if has("approval", version) {
            for name in d.pages.keys.sorted(by: byteLess) {
                let perm = str(d.pages[name]?.child("permission"))
                if perm.isEmpty || perm == "public" || perm == permissionSignedIn { continue }
                for role in d.roles.keys.sorted(by: byteLess) where d.roleHolds(d.roles[role]).contains(perm) {
                    add(role, "page " + name)
                }
            }
        }
        for role in relies.keys.sorted(by: byteLess) {
            let what = relies[role]!
            c.warn(d.stakeholders[role], pointer("stakeholders", role), .unheard, "\(role) was never heard: no communication record names it, no session lists it present, no interview lists it and no decision is by it, while \(joinAnd(what)) \(what.count == 1 ? "relies" : "rely") on it; hear it before acceptance, and record what it said with specarch log\(hint)")
        }
    }
}

extension RecordChecks {
    /// Checks the record an approval names as the session in which its
    /// role read the documents: a session, a test session or a
    /// communication record, under exactly one of those kinds, that lists
    /// the role under present, or as from or to (ADR-239).
    func checkApprovalSession(_ r: Record) {
        guard let n = r.root?.child("session") else { return }
        let id = str(n)
        if id.isEmpty { return }
        let role = r.str("approvedBy")
        var found: [Record] = []
        var kinds: [String] = []
        for (kind, folder) in [("session", "sessions"), ("test-session", "test-sessions"), ("communication", "communications")] {
            if let rec = set[kind]?[id] {
                found.append(rec)
                kinds.append("records/\(folder)/\(id).yaml")
            }
        }
        if found.isEmpty {
            r.c.add(n, "/session", .recordRef, "\(id) is not a session, a test session or a communication: there is no records/sessions/\(id).yaml, records/test-sessions/\(id).yaml or records/communications/\(id).yaml; name the record in which \(role) read the documents")
            return
        }
        if found.count > 1 {
            r.c.add(n, "/session", .recordRef, "\(id) names more than one record: \(joinAnd(kinds)); give each record an ID of its own, and name the one in which \(role) read the documents")
            return
        }
        let rec = found[0]
        if rec.kind == "communication" {
            let from = rec.str("from"), to = rec.str("to")
            if from == role || to == role { return }
            r.c.add(n, "/session", .approvalSession, "\(id) is from \(from) to \(to), not from or to \(role), so it does not show that \(role) read the documents; name the session or the communication in which \(role) read them")
            return
        }
        let present = items(rec.root?.child("present")).map(\.value)
        if present.contains(role) { return }
        let what = present.isEmpty ? "lists nobody under present" : "lists \(joinAnd(present)) under present"
        r.c.add(n, "/session", .approvalSession, "\(id) \(what), not \(role), so it does not show that \(role) read the documents there; name the session in which \(role) read them, or add \(role) under present when it took part")
    }
}

/// A communication's type with its article, for a message.
func typeArticle(_ typ: String) -> String {
    switch typ {
    case "answer", "objection", "information": return "an " + typ
    case "": return "of no type"
    default: return "a " + typ
    }
}

/// Joins names with "or".
func eitherOf(_ items: [String]) -> String {
    switch items.count {
    case 0: return "nothing"
    case 1: return items[0]
    default: return items.dropLast().joined(separator: ", ") + " or " + items.last!
    }
}
