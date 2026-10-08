import Foundation

/// Each kind of record and its folder under records/.
let recordFolders: [String: String] = [
    "change": "changes", "defect": "defects", "release": "releases",
    "incident": "incidents", "commissioning": "commissioning", "approval": "approvals",
]

/// One record file, read and checked against its schema.
final class Record {
    let path: String
    let folder: String
    var kind = "" // a kind of recordFolders, or "" when it has none
    var root: YNode?
    let c: Checker

    init(path: String, folder: String) {
        self.path = path
        self.folder = folder
        c = Checker(file: path)
    }

    func str(_ key: String) -> String { SpecArchKit.str(root?.child(key)) }

    /// What the record is named by: its ID, its version, or the date and
    /// environment of a commissioning run; "" when the fields are missing.
    var key: String {
        switch kind {
        case "change", "defect", "incident": return str("id")
        case "release", "approval": return str("version")
        case "commissioning":
            let date = str("date"), env = str("environment")
            return date.isEmpty || env.isEmpty ? "" : date + "-" + env
        default: return ""
        }
    }
}

/// Checks the records beside a specification: each against the record
/// schema, then against the specification and the other records.
func checkRecords(_ s: Spec, _ d: Design) -> [Diagnostic] {
    var recs: [Record] = []
    var set: [String: [String: Record]] = [:]
    for f in s.records {
        let r = Record(path: f.path, folder: f.folder)
        recs.append(r)
        let doc = parseYAML(String(decoding: f.data, as: UTF8.self))
        for p in doc.problems {
            r.c.addLine(p.line, p.path, Rule(rawValue: p.rule)!, p.message)
        }
        guard let root = doc.root else { continue }
        r.root = root
        r.c.root = root
        r.c.checkSchema(.record, doc.value)
        guard root.kind == .mapping else { continue }
        let k = r.str("kind")
        if recordFolders[k] != nil {
            r.kind = k
            let key = r.key
            if !key.isEmpty && set[k]?[key] == nil {
                set[k, default: [:]][key] = r
            }
        }
    }
    let rc = RecordChecks(d: d, set: set)
    for r in recs where !r.kind.isEmpty {
        rc.check(r)
    }
    let rootChecker = Checker(file: s.rootPath, files: s.files)
    rc.checkReleases(recs, rootChecker)
    rc.checkRequirementReleases(rootChecker)
    var out = rootChecker.diags
    for r in recs {
        out += withoutEchoes(r.c.diags)
    }
    return out
}

/// What a reference into the specification finds.
enum Resolution {
    case missing
    case found    // a requirement, or an element a pointer reaches
    case retired  // a requirement whose status is retired
    case external // a requirement of a declared requirement set
}

private let openChange: Set<String> = ["proposed", "analysed", "approved"]

func prefixOf(_ id: String) -> String {
    String(id.split(separator: "-", maxSplits: 1, omittingEmptySubsequences: false).first ?? "")
}

struct RecordChecks {
    let d: Design
    let set: [String: [String: Record]]

    func has(_ kind: String, _ key: String) -> Bool { set[kind]?[key] != nil }

    func check(_ r: Record) {
        checkName(r)
        let root = r.root!
        switch r.kind {
        case "change":
            role(r, "raisedBy")
            role(r, "decision", "by")
            release(r, "release")
            checkApplied(r)
            checkDecision(r)
        case "defect":
            role(r, "reportedBy")
            environment(r, "environment")
            for (i, item) in items(root.child("violates")).enumerated() {
                reference(r, item, pointer("violates", String(i)))
            }
            test(r)
            for (i, item) in items(root.child("incidents")).enumerated() {
                recordID(r, item, pointer("incidents", String(i)), ["incident"])
            }
            if let n = root.child("change") {
                recordID(r, n, "/change", ["change"])
            }
            release(r, "release")
            checkDefectTest(r)
            checkDuplicate(r)
        case "release":
            for (i, item) in items(root.child("includes")).enumerated() {
                recordID(r, item, pointer("includes", String(i)), ["change", "defect"])
            }
            for (i, item) in items(root.child("commissioning")).enumerated() {
                let v = item.value
                if !v.isEmpty && !has("commissioning", v) {
                    r.c.add(item, pointer("commissioning", String(i)), .recordRef,
                            "\(v) is not a commissioning run: there is no records/commissioning/\(v).yaml")
                }
            }
        case "incident":
            environment(r, "environment")
            if let n = root.child("monitor") {
                let v = str(n)
                if !v.isEmpty && d.monitors[v] == nil {
                    r.c.add(n, "/monitor", .recordRef, "\(v) is not a monitor of the specification\(suggest(v, d.monitors))")
                }
            }
            role(r, "reportedBy")
            for (i, item) in items(root.child("defects")).enumerated() {
                recordID(r, item, pointer("defects", String(i)), ["defect"])
            }
            for (i, item) in items(root.child("changes")).enumerated() {
                recordID(r, item, pointer("changes", String(i)), ["change"])
            }
            checkIncidentLink(r)
        case "commissioning":
            environment(r, "environment")
            role(r, "operator")
            role(r, "signoff", "by")
            checkCommissioning(r)
        case "approval":
            role(r, "approvedBy")
        default:
            break
        }
    }

    /// Reports a record outside its kind's folder, or one whose file name
    /// is not what it is named by.
    func checkName(_ r: Record) {
        let root = r.root!
        let want = recordFolders[r.kind]!
        if r.folder != want {
            r.c.add(root.child("kind"), "/kind", .recordName, "a record of kind \(r.kind) lives in records/\(want)/; move it there")
        }
        let key = r.key
        var name = (r.path as NSString).lastPathComponent
        if name.hasSuffix(".yaml") { name = String(name.dropLast(5)) }
        if key.isEmpty || name == key { return }
        switch r.kind {
        case "change", "defect", "incident":
            r.c.add(root.child("id"), "/id", .recordName, "the file is named \(name) but the record's id is \(key); name the file \(key).yaml")
        case "release", "approval":
            r.c.add(root.child("version"), "/version", .recordName, "the file is named \(name) but the record's version is \(key); name the file \(key).yaml")
        case "commissioning":
            r.c.add(root.child("date"), "/date", .recordName, "the file is named \(name) but the run's date and environment are \(key); name the file \(key).yaml")
        default:
            break
        }
    }

    /// Checks that the value at a key is a stakeholder of the specification.
    func role(_ r: Record, _ keys: String...) {
        var n: YNode? = r.root
        for k in keys { n = n?.child(k) }
        guard let n else { return }
        let v = str(n)
        if !v.isEmpty && d.stakeholders[v] == nil {
            r.c.add(n, pointer(keys), .recordRef, "\(v) is not a stakeholder of the specification\(suggest(v, d.stakeholders))")
        }
    }

    func environment(_ r: Record, _ key: String) {
        guard let n = r.root?.child(key) else { return }
        let v = str(n)
        if !v.isEmpty && d.environments[v] == nil {
            r.c.add(n, pointer(key), .recordRef, "\(v) is not an environment of the specification\(suggest(v, d.environments))")
        }
    }

    func release(_ r: Record, _ key: String) {
        guard let n = r.root?.child(key) else { return }
        let v = str(n)
        if !v.isEmpty && !has("release", v) {
            r.c.add(n, pointer(key), .recordRef, "\(v) is not a release: there is no records/releases/\(v).yaml")
        }
    }

    /// Checks that a requirement's release is a release record that is
    /// planned or released.
    func checkRequirementReleases(_ root: Checker) {
        for (id, req) in d.requirements {
            guard let n = req.child("release") else { continue }
            let v = str(n)
            // The schema reports a version that is not one.
            guard let ver = Version(v), ver.pre.isEmpty else { continue }
            let ptr = pointer("requirements", id, "release")
            guard let rel = set["release"]?[v] else {
                root.add(n, ptr, .recordRef, "\(v) is not a release: there is no records/releases/\(v).yaml; add it with status planned, or name a release that exists")
                continue
            }
            if rel.str("status") == "withdrawn" {
                root.add(n, ptr, .recordRef, "release \(v) is withdrawn, so no requirement is meant for it; name a planned or released release")
            }
        }
    }

    func test(_ r: Record) {
        guard let n = r.root?.child("test") else { return }
        let v = str(n)
        if !v.isEmpty && d.root.child("tests")?.child(v) == nil {
            var tests: [String: YNode] = [:]
            for p in pairs(d.root.child("tests")) { tests[p.key.value] = p.value }
            r.c.add(n, "/test", .recordRef, "\(v) is not a test of the specification\(suggest(v, tests))")
        }
    }

    /// Checks that an ID names a record of one of the kinds, or falls in a
    /// declared change-set or defect-set.
    func recordID(_ r: Record, _ n: YNode, _ path: String, _ kinds: [String]) {
        let v = str(n)
        if v.isEmpty { return }
        let prefix = prefixOf(v)
        var whereFiles: [String] = [], sets: [String] = []
        for k in kinds {
            if has(k, v) { return }
            if k == "change" || k == "defect" {
                if d.trackerPrefixes(k + "-set")[prefix] != nil { return }
                sets.append(k + "-set")
            }
            whereFiles.append("records/\(recordFolders[k]!)/\(v).yaml")
        }
        let what = ["change": "a change request", "defect": "a defect", "incident": "an incident"]
        var msg = "\(v) is not \(kinds.map { what[$0]! }.joined(separator: " or ")): there is no \(whereFiles.joined(separator: " or "))"
        if !sets.isEmpty {
            msg += " and no source of kind \(sets.joined(separator: " or ")) with prefix \(prefix)"
        }
        r.c.add(n, path, .recordRef, msg)
    }

    func resolveRef(_ ref: String) -> Resolution {
        if ref.hasPrefix("#/") {
            let tokens = ref.dropFirst(2).split(separator: "/", omittingEmptySubsequences: false).map { unescapeToken(String($0)) }
            return resolve(d.root, tokens).1 ? .found : .missing
        }
        if let req = d.requirements[ref] {
            return str(req.child("status")) == "retired" ? .retired : .found
        }
        return d.requirementSetPrefixes()[prefixOf(ref)] != nil ? .external : .missing
    }

    /// Checks a requirement ID or #/ pointer a record names.
    func reference(_ r: Record, _ n: YNode, _ path: String) {
        let v = str(n)
        if v.isEmpty || resolveRef(v) != .missing { return }
        if v.hasPrefix("#/") {
            r.c.add(n, path, .recordRef, "\(v) does not resolve in the specification; point at an element that exists")
            return
        }
        r.c.add(n, path, .recordRef, "\(v) is not a requirement of the specification\(suggest(v, d.requirements))")
    }

    /// Checks a change's affects against the specification as it is now:
    /// carried out once implemented, still to be done while open.
    func checkApplied(_ r: Record) {
        let id = r.str("id"), status = r.str("status")
        let done = status == "implemented" || status == "released"
        if !done && !openChange.contains(status) { return }
        let affects = r.root?.child("affects")
        for list in ["adds", "changes", "removes"] {
            for (i, item) in items(affects?.child(list)).enumerated() {
                let v = item.value
                if v.isEmpty { continue }
                let res = resolveRef(v)
                let path = pointer("affects", list, String(i))
                let present = res == .found || res == .external
                if done && list == "adds" && !present {
                    r.c.add(item, path, .changeApplied, "\(id) is \(status) but \(v), which it adds, is not in the specification; add it, or set the change back to approved")
                } else if done && list == "changes" && !present {
                    r.c.add(item, path, .changeApplied, "\(id) is \(status) but \(v), which it changes, is not in the specification; correct the reference")
                } else if done && list == "removes" && res == .found {
                    r.c.add(item, path, .changeApplied, "\(id) is \(status) but \(v), which it removes, is still in the specification; remove it, or give the requirement status retired")
                } else if !done && list != "adds" && res == .missing {
                    r.c.add(item, path, .changeApplied, "\(v), which \(id) \(list), is not in the specification; a change can only change or remove what exists")
                } else if !done && list == "adds" && present {
                    r.c.warn(item, path, .changeApplied, "\(v), which \(id) adds, is already in the specification; if the change is carried out, set its status to implemented, or name it under changes")
                }
            }
        }
    }

    /// Checks that a decided change carries the decision.
    func checkDecision(_ r: Record) {
        let id = r.str("id"), status = r.str("status")
        let want: String
        switch status {
        case "approved", "implemented", "released": want = "approved"
        case "rejected": want = "rejected"
        default: return
        }
        guard let decision = r.root?.child("decision") else {
            r.c.add(r.root?.child("status"), "/status", .changeDecision, "\(id) is \(status) but has no decision; add decision with by, date, outcome \(want) and why")
            return
        }
        if let outcome = decision.child("outcome"), !str(outcome).isEmpty, str(outcome) != want {
            r.c.add(outcome, "/decision/outcome", .changeDecision, "\(id) is \(status) but its decision's outcome is \(str(outcome)); a \(status) change needs outcome \(want)")
        }
    }

    /// Checks that a fixed defect names the test that shows the fix, and
    /// that the test is about what the defect breaks.
    func checkDefectTest(_ r: Record) {
        let id = r.str("id"), status = r.str("status")
        if status != "fixed" && status != "released" { return }
        let tn = r.root?.child("test")
        let name = str(tn)
        if name.isEmpty {
            r.c.add(r.root?.child("status"), "/status", .defectTest, "\(id) is \(status) but names no test; add test with the test that fails before the fix and passes after")
            return
        }
        guard let t = d.root.child("tests")?.child(name) else { return } // reported as record_ref
        let verifies = Set(items(t.child("verifies")).map(\.value))
        let subject = testElement(t)
        for item in items(r.root?.child("violates")) {
            let v = item.value
            if verifies.contains(v) { return }
            if !subject.isEmpty && v.hasPrefix("#/") && elementOf(v) == subject { return }
        }
        r.c.add(tn, "/test", .defectTest, "test \(name) neither verifies a requirement \(id) violates nor is about an element it violates; add the requirement to the test's verifies, or name the test that shows the fix")
    }

    /// The pointer of the element a test is about, such as /entities/Loan,
    /// or "".
    func testElement(_ t: YNode) -> String {
        let op = str(t.child("operation"))
        if !op.isEmpty {
            if let o = d.operations[op] { return pointer("paths", o.path) }
            return ""
        }
        for k in ["command", "page", "entity"] {
            let v = str(t.child(k))
            if !v.isEmpty { return pointer(k + "s", v) }
        }
        return ""
    }

    /// The element a #/ pointer points into: its first two tokens.
    func elementOf(_ ref: String) -> String {
        let tokens = ref.dropFirst(2).split(separator: "/", maxSplits: 2, omittingEmptySubsequences: false)
        if tokens.count < 2 { return "" }
        return "/" + tokens[0] + "/" + tokens[1]
    }

    /// Checks that a duplicate names the defect it repeats.
    func checkDuplicate(_ r: Record) {
        let id = r.str("id")
        if r.str("status") != "duplicate" { return }
        let n = r.root?.child("duplicateOf")
        let of = str(n)
        if of.isEmpty {
            r.c.add(r.root?.child("status"), "/status", .defectDuplicate, "\(id) is a duplicate but names no defect; add duplicateOf with the defect it repeats")
            return
        }
        if let other = set["defect"]?[of] {
            if other.str("status") == "duplicate" {
                r.c.add(n, "/duplicateOf", .defectDuplicate, "\(of) is itself a duplicate; name the defect \(of) repeats instead")
            }
            return
        }
        let prefix = prefixOf(of)
        if d.trackerPrefixes("defect-set")[prefix] != nil { return }
        r.c.add(n, "/duplicateOf", .defectDuplicate, "\(of) is not a defect: there is no records/defects/\(of).yaml and no source of kind defect-set with prefix \(prefix)")
    }

    /// Warns about a resolved incident that explains nothing.
    func checkIncidentLink(_ r: Record) {
        if r.str("status") != "resolved" { return }
        for k in ["defects", "changes", "noChange"] where r.root?.child(k) != nil { return }
        r.c.warn(r.root?.child("status"), "/status", .incidentLink, "\(r.str("id")) is resolved but leads to no defect and no change; name the defects or changes that follow from it, or say in noChange why there are none")
    }

    /// Checks a run's results name checks of the specification and its
    /// version is a release.
    func checkCommissioning(_ r: Record) {
        for p in pairs(r.root?.child("results")) where d.checks[p.key.value] == nil {
            r.c.add(p.key, pointer("results", p.key.value), .commissioningRecord, "\(p.key.value) is not a check of the specification\(suggest(p.key.value, d.checks))")
        }
        if let n = r.root?.child("version") {
            let v = str(n)
            if !v.isEmpty && !has("release", v) {
                r.c.add(n, "/version", .commissioningRecord, "\(v) is not a release: there is no records/releases/\(v).yaml; a commissioning run accepts a release")
            }
        }
    }
}

extension Design {
    /// The prefixes of the sources of one kind: requirement-set, change-set
    /// or defect-set.
    func trackerPrefixes(_ kind: String) -> [String: YNode] {
        var out: [String: YNode] = [:]
        for (_, src) in sources where str(src.child("kind")) == kind {
            let prefix = str(src.child("prefix"))
            if !prefix.isEmpty { out[prefix] = src }
        }
        return out
    }
}
