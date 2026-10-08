import Foundation

/// Something tests are written about: an operation, a command, a page, or
/// an entity's constraint or transition.
final class Subject {
    let label: String    // how a message names it: "operation createLoan"
    let node: YNode      // where a missing scenario is reported
    let path: String
    let yamlKey: String  // the subject keys of a test, in flow style
    let name: String     // for the suggested test name
    var raw = ""         // the subject's own name inside its case names, if any
    var cases: [DerivedCase] = []
    /// The golden path the design implies: what a caller allowed to do it
    /// sends inside every limit, and the outcome.
    var success: DerivedCase?
    /// True when a requirement the subject satisfies names a harm.
    var critical = false
    /// True for a requirement or a state machine: every case is golden and
    /// asked for on its own, so neither needs a scenario of each kind.
    var whole = false

    init(label: String, node: YNode, path: String, yamlKey: String, name: String) {
        self.label = label
        self.node = node
        self.path = path
        self.yamlKey = yamlKey
        self.name = name
    }

    func red(_ name: String, _ frequency: String, _ given: String, _ when: String, _ then: String) {
        cases.append(DerivedCase(name: name, scenario: "red", given: given, when: when, then: then, frequency: frequency))
    }

    /// A red case that is critical on its own.
    func byNature(_ name: String, _ given: String, _ when: String, _ then: String) {
        var dc = DerivedCase(name: name, scenario: "red", given: given, when: when, then: then, frequency: rare)
        dc.critical = true
        cases.append(dc)
    }

    /// A case about one field, whose mistakes key may replace the frequency.
    func fieldCase(_ scenario: String, _ name: String, _ frequency: String, _ fieldName: String, _ field: YNode?,
                   _ given: String, _ when: String, _ then: String) {
        cases.append(DerivedCase(name: name, scenario: scenario, given: given, when: when, then: then,
                                 frequency: frequency, field: field, fieldName: fieldName))
    }

    /// Critical for a case of a critical subject or a case critical on its
    /// own, frequent for a case users get wrong often, other otherwise.
    func rank(_ dc: DerivedCase) -> String {
        if critical || dc.critical { return rankCritical }
        let m = dc.mistakes
        return (m.isEmpty ? dc.frequency : m) == frequent ? rankFrequent : rankOther
    }

    /// A case whose name the subject derives more than once is chosen when
    /// any of them is.
    func chosen(_ name: String) -> Bool {
        cases.contains { $0.name == name && rank($0) != rankOther }
    }
}

// The frequencies of the case kinds, from the table in docs/conventions.md.
let frequent = "frequent", occasional = "occasional", rare = "rare"

// The ranks of a derived case. Critical and frequent cases are chosen: the
// validator warns when no test covers one.
let rankCritical = "critical", rankFrequent = "frequent", rankOther = "other"

/// A scenario the rest of the file says a subject needs.
struct DerivedCase {
    var name: String
    var scenario: String
    var given: String
    var when: String
    var then: String
    var frequency: String
    var field: YNode? = nil
    var fieldName = ""
    /// Marks one case critical on its own: a path through a transition that
    /// satisfies a requirement with a harm, or a case nobody exercises by hand
    /// (a failing dependency, two writers on one record).
    var critical = false

    /// The field's own frequency, or "".
    var mistakes: String { str(child(field, "mistakes")) }
}

/// createLoan becomes create-loan.
func kebab(_ s: String) -> String {
    var out = ""
    for (i, c) in s.unicodeScalars.enumerated() {
        if c.properties.isUppercase {
            if i > 0 { out += "-" }
            out += String(c).lowercased()
        } else if c == " " || c == "_" || c == "." {
            out += "-"
        } else {
            out.unicodeScalars.append(c)
        }
    }
    return out
}

private func chars(_ n: String) -> String { n == "1" ? "1 character" : n + " characters" }

private func characters(_ n: String, _ off: Int) -> String {
    let v = (Int(n) ?? 0) + off
    return v == 1 ? "of 1 character" : "of \(v) characters"
}

private let validatingFormats: Set<String> = ["date", "date-time", "time", "duration", "email", "uuid", "uri", "hostname", "ipv4", "ipv6", "decimal"]

/// Who may do something that needs a permission.
private func caller(_ perm: String) -> String {
    perm.isEmpty || perm == "public" ? "any caller" : "a caller with " + perm
}

private func golden(_ given: String, _ when: String, _ then: String) -> DerivedCase {
    DerivedCase(name: "succeeds", scenario: "golden", given: given, when: when, then: then, frequency: "")
}

/// The cases of a guard on an operation or a command: the precondition not
/// holding, and a second writer on the same record.
/// The cases of a list: a page after the last, a page larger than the
/// maximum, and a sort or filter outside the lists.
private func pageCases(_ s: Subject, _ o: Operation, _ call: String) {
    guard let l = o.node.child("listOf") else { return }
    s.cases.append(DerivedCase(name: "page beyond last", scenario: "golden", given: "fewer records than fill the pages asked for",
                               when: call + " for a page after the last", then: "it answers an empty page with the true totals", frequency: occasional))
    let max = str(l.child("pageSize")?.child("maximum"))
    if !max.isEmpty {
        s.red("page size above " + max, occasional, "...", call + " with a page size of \((Int(max) ?? 0) + 1)", "it is refused")
    }
    if l.child("sortable") != nil {
        s.red("sort by a field not sortable", occasional, "...", call + " sorted by a field that is not sortable", "it is refused, not ignored")
    }
    if l.child("filterable") != nil {
        s.red("filter by a field not filterable", occasional, "...", call + " filtered by a field that is not filterable", "it is refused, not ignored")
    }
}

/// The cases of an operation's limits: a body too large, and more requests
/// than the rate allows.
private func limitsCases(_ s: Subject, _ o: Operation, _ call: String) {
    let l = o.node.child("limits")
    let n = str(l?.child("maxRequestBytes"))
    if !n.isEmpty {
        s.red("request larger than " + n + " bytes", occasional, "...", call + " with a body larger than " + n + " bytes", failureOr(o.node, "413", "it is refused as too large"))
    }
    if let rate = l?.child("rate") {
        let r = str(rate.child("requests")), per = str(rate.child("per"))
        s.red("rate exceeded", occasional, "a caller who has made " + r + " requests within " + per, call + " once more", failureOr(o.node, "429", "it is refused as too many requests"))
    }
}

/// The response of a status, or the plain refusal when the operation does
/// not declare it.
private func failureOr(_ op: YNode, _ status: String, _ plain: String) -> String {
    let then = failureResponse(op, status)
    return then == "..." ? plain : then
}

private func guardCases(_ s: Subject, _ g: YNode?, _ what: String) {
    guard let g else { return }
    let ent = str(g.child("entity"))
    let pre = str(g.child("precondition"))
    if !pre.isEmpty {
        s.red("guard precondition fails", occasional, "a " + ent + " for which " + pre + " does not hold", what, "it is refused and no " + ent + " changes")
    }
    s.byNature("concurrent write", "another caller changed the " + ent + " after this caller read it", what, "it is refused and the other caller's change stands")
}

/// The response an operation gives when a dependency fails or does not
/// answer in time, when the design declares one of the statuses given, or
/// "...".
private func failureResponse(_ op: YNode, _ statuses: String...) -> String {
    for code in statuses {
        if let r = op.child("responses")?.child(code) { return "it answers " + code + ": " + str(r.child("description")) }
    }
    return "..."
}

extension Design {
    /// The cases of a permission other than public: a caller without it,
    /// and, once the specification declares a session, a caller whose session
    /// has expired.
    func denied(_ s: Subject, _ perm: String, _ what: String) {
        if perm.isEmpty || perm == "public" { return }
        s.red("denied without " + perm, frequent, "a caller without " + perm, what, "it is refused as not allowed")
        if root.child("session") != nil {
            s.red("denied with expired session", frequent, "a caller whose session has expired", what, "it is refused as not signed in")
        }
    }

    /// Every subject of the file with its derived cases: operations,
    /// commands, pages, then each entity's constraints and transitions.
    func subjects() -> [Subject] {
        var out: [Subject] = []
        for o in opList where !o.id.isEmpty { out.append(withHarm(operationSubject(o), o.node)) }
        for p in pairs(root.child("commands")) { out.append(withHarm(commandSubject(p), p.value)) }
        for p in pairs(root.child("jobs")) { out.append(withHarm(jobSubject(p), p.value)) }
        for p in pairs(root.child("pages")) { out.append(withHarm(pageSubject(p), p.value)) }
        for e in pairs(root.child("entities")) {
            for c in pairs(e.value.child("constraints")) { out.append(withHarm(constraintSubject(e.key.value, c), c.value)) }
            for (i, t) in items(e.value.child("transitions")).enumerated() { out.append(withHarm(transitionSubject(e.key.value, i, t), t)) }
            if let s = flowSubject(e) { out.append(s) }
        }
        for r in pairs(root.child("requirements")) {
            if let s = requirementSubject(r) { out.append(s) }
        }
        return out
    }

    /// A requirement's acceptance tests: one golden case per acceptance
    /// criterion, with the criterion as the outcome. They are chosen when
    /// the requirement names a harm.
    func requirementSubject(_ r: Pair) -> Subject? {
        let id = r.key.value, status = str(r.value.child("status"))
        let criteria = items(r.value.child("acceptance"))
        if criteria.isEmpty || status == "rejected" || status == "retired" { return nil }
        let s = Subject(label: "requirement " + id, node: r.key, path: pointer("requirements", id), yamlKey: "requirement: " + id, name: id.lowercased())
        s.whole = true
        s.critical = !items(r.value.child("harm")).isEmpty
        for (i, c) in criteria.enumerated() {
            s.cases.append(DerivedCase(name: "acceptance \(i + 1)", scenario: "golden", given: "...", when: "...", then: c.str, frequency: occasional))
        }
        return s
    }

    /// An entity's state machine: one golden case per path from an initial
    /// state (one no transition reaches) to a terminal state (one no
    /// transition leaves), taking the transitions in document order and
    /// never visiting a state twice. A path is chosen when one of its
    /// transitions satisfies a requirement with a harm.
    func flowSubject(_ e: Pair) -> Subject? {
        let entity = e.key.value
        let transitions = items(e.value.child("transitions"))
        if transitions.isEmpty { return nil }
        var reached = Set<String>(), leaves = Set<String>(), starts: [String] = []
        for t in transitions {
            reached.insert(str(t.child("to")))
            leaves.insert(str(t.child("from")))
        }
        for t in transitions {
            let from = str(t.child("from"))
            if !reached.contains(from) && !starts.contains(from) { starts.append(from) }
        }
        let s = Subject(label: entity + " state machine", node: e.key, path: pointer("entities", entity), yamlKey: "entity: " + entity, name: kebab(entity))
        s.whole = true
        func walk(_ state: String, _ states: [String], _ triggers: [String], _ critical: Bool) {
            if !leaves.contains(state) {
                if triggers.isEmpty { return }
                var dc = DerivedCase(name: states.joined(separator: " to "), scenario: "golden",
                                     given: "a " + entity + " that is " + states[0], when: triggers.joined(separator: ", then ") + " happen",
                                     then: "the " + entity + " ends " + state + ", having been " + joinAnd(Array(states.dropLast())), frequency: occasional)
                dc.critical = critical
                s.cases.append(dc)
                return
            }
            for t in transitions where str(t.child("from")) == state {
                let to = str(t.child("to"))
                if states.contains(to) { continue }
                var trigger = str(t.child("trigger"))
                if trigger.isEmpty { trigger = "the move to " + to }
                let harmful = items(t.child("satisfies")).contains { !items(child(requirements[$0.value], "harm")).isEmpty }
                walk(to, states + [to], triggers + [trigger], critical || harmful)
            }
        }
        for start in starts { walk(start, [start], [], false) }
        return s.cases.isEmpty ? nil : s
    }

    /// Marks the subject critical when a requirement its own satisfies
    /// names, under the node given, has a harm.
    func withHarm(_ s: Subject, _ n: YNode) -> Subject {
        for id in items(n.child("satisfies")) where !items(child(requirements[id.value], "harm")).isEmpty {
            s.critical = true
        }
        return s
    }

    func operationSubject(_ o: Operation) -> Subject {
        let s = Subject(label: "operation " + o.id, node: o.node, path: o.pointer(), yamlKey: "operation: " + o.id, name: kebab(o.id))
        let call = o.id + " is called"
        var success = golden(caller(str(o.node.child("permission"))), call, "it succeeds")
        if !pairs(o.node.child("requestBody")?.child("content")).isEmpty || !items(o.node.child("parameters")).isEmpty || !items(o.pathItem.child("parameters")).isEmpty {
            success.when = call + " with values inside every limit"
        }
        if let r = pairs(o.node.child("responses")).first(where: { $0.key.value.hasPrefix("2") }) {
            success.then = "it answers " + r.key.value + ": " + str(r.value.child("description"))
        }
        s.success = success
        let (body, bodyRequired) = requestFields(o.node)
        for f in bodyRequired {
            s.fieldCase("red", "missing " + f, frequent, f, body[f], "...", o.id + " is called without " + f, "it is refused")
        }
        for n in body.keys.sorted(by: byteLess) {
            limitCases(s, n, body[n], call)
        }
        for p in items(o.pathItem.child("parameters")) + items(o.node.child("parameters")) {
            let name = str(p.child("name"))
            limitCases(s, name, p.child("schema"), call)
            if str(p.child("in")) == "path" {
                s.fieldCase("red", "not found " + name, frequent, name, p.child("schema"), "no record has that " + name, o.id + " is called with that " + name, "it is refused as not found")
            }
        }
        let ent = responseEntity(o.node)
        if !ent.isEmpty {
            for r in pairs(child(entities[ent], "relations")) {
                let kind = str(r.value.child("kind")), via = str(r.value.child("via"))
                if (kind == "many-to-one" || kind == "one-to-one") && body[via] != nil {
                    let target = str(r.value.child("target"))
                    s.fieldCase("red", "not found " + via, frequent, via, body[via], "no " + target + " has that " + via, o.id + " is called with that " + via, "it is refused as not found")
                    if let v = child(entities[target], "validity") {
                        s.fieldCase("red", "expired " + via, occasional, via, body[via], "the " + target + " that " + via + " names is past its " + str(v.child("until")), o.id + " is called with that " + via, "it is refused as expired")
                        let from = str(v.child("from"))
                        if !from.isEmpty {
                            s.fieldCase("red", "not yet valid " + via, occasional, via, body[via], "the " + target + " that " + via + " names is before its " + from, o.id + " is called with that " + via, "it is refused as not yet valid")
                        }
                    }
                }
            }
            if str(child(entities[ent], "deletion")) == "soft" && o.method == "get" {
                softDeleteCases(s, o, ent, call)
            }
            if o.method == "post" && child(o.node.child("responses"), "201") != nil {
                for c in pairs(child(entities[ent], "constraints")) where str(c.value.child("kind")) == "unique" {
                    s.red("duplicate " + c.key.value, occasional, "a " + ent + " that " + c.key.value + " would clash with exists", call, "it is refused as a duplicate")
                }
            }
        }
        denied(s, str(o.node.child("permission")), call)
        var seenChannel = Set<String>()
        for e in items(o.node.child("emits")) {
            let ch = String(e.value.split(separator: "/", maxSplits: 1, omittingEmptySubsequences: false)[0])
            if !seenChannel.contains(ch) {
                seenChannel.insert(ch)
                s.byNature("dependency fails " + ch, ch + " cannot take the message", call, "...")
            }
        }
        for e in items(o.node.child("calls")) {
            guard let dep = dependencies[e.value] else { continue }
            s.byNature("dependency fails " + e.value, e.value + " answers with an error", call, failureResponse(o.node, "502", "503"))
            s.byNature("dependency times out " + e.value, e.value + " does not answer within " + str(dep.child("timeout")), call, failureResponse(o.node, "504", "503"))
        }
        let key = str(o.node.child("idempotencyKey"))
        if !key.isEmpty {
            let schema = headerParameter(o, key)?.child("schema")
            let given = o.id + " has answered a request that carried " + key
            s.fieldCase("golden", "repeated with the same " + key, frequent, key, schema, given, o.id + " is called again with the same " + key + " and the same request", "it answers as the first call did and nothing changes a second time")
            s.fieldCase("red", key + " reused for another request", occasional, key, schema, given, o.id + " is called with the same " + key + " and a different request", "it is refused")
        }
        guardCases(s, o.node.child("guard"), call)
        pageCases(s, o, call)
        limitsCases(s, o, call)
        for r in pairs(o.node.child("responses")) {
            let code = r.key.value
            if code.count == 3, code.first == "4" || code.first == "5" {
                s.red("response " + code, occasional, "...", call, "it answers " + code + ": " + str(r.value.child("description")))
            }
        }
        return s
    }

    /// The cases of a read of an entity with soft deletion: a list leaves a
    /// deleted record out, and a read by id answers as for a record that
    /// does not exist.
    func softDeleteCases(_ s: Subject, _ o: Operation, _ ent: String, _ call: String) {
        let deleted = "a " + ent + " that is deleted"
        if responseIsList(o.node) {
            s.cases.append(DerivedCase(name: "deleted " + ent + " not listed", scenario: "golden", given: deleted, when: call,
                                       then: "the deleted " + ent + " is not in the answer", frequency: occasional))
            return
        }
        for p in items(o.pathItem.child("parameters")) + items(o.node.child("parameters")) where str(p.child("in")) == "path" {
            s.red("deleted " + ent + " read", occasional, deleted, o.id + " is called with its " + str(p.child("name")), failureOr(o.node, "404", "it is refused as not found"))
            return
        }
    }

    /// Whether a successful response returns a list.
    func responseIsList(_ op: YNode) -> Bool {
        for r in pairs(op.child("responses")) where r.key.value.hasPrefix("2") {
            for c in pairs(r.value.child("content")) where child(c.value.child("schema"), "items") != nil {
                return true
            }
        }
        return false
    }

    /// The request body's fields and its required ones. For a $ref to an
    /// entity, fields the system sets (readOnly) are left out.
    func requestFields(_ op: YNode) -> ([String: YNode], [String]) {
        let content = pairs(child(op.child("requestBody"), "content"))
        guard let first = content.first else { return ([:], []) }
        var schema = first.value.child("schema")
        var readOnlyLeftOut = false
        let ref = str(child(schema, "$ref"))
        if ref.hasPrefix("#/entities/") {
            schema = entities[String(ref.dropFirst("#/entities/".count))]
            readOnlyLeftOut = true
        }
        var fields: [String: YNode] = [:]
        for p in pairs(child(schema, "properties")) {
            if readOnlyLeftOut && str(p.value.child("readOnly")) == "true" { continue }
            fields[p.key.value] = p.value
        }
        let required = items(child(schema, "required")).map { $0.value }.filter { fields[$0] != nil }
        return (fields, required)
    }

    /// The entity a successful response returns, alone or in a list, or "".
    func responseEntity(_ op: YNode) -> String {
        for r in pairs(op.child("responses")) where r.key.value.hasPrefix("2") {
            for c in pairs(r.value.child("content")) {
                var schema = c.value.child("schema")
                if let it = child(schema, "items") { schema = it }
                let ref = str(child(schema, "$ref"))
                if ref.hasPrefix("#/entities/") { return String(ref.dropFirst("#/entities/".count)) }
            }
        }
        return ""
    }

    /// A field's boundary cases: just outside every limit is red, exactly
    /// on it is golden.
    func limitCases(_ s: Subject, _ name: String, _ field: YNode?, _ call: String) {
        guard let field, !name.isEmpty else { return }
        let with = call + " with " + name
        func red(_ caseName: String, _ frequency: String, _ when: String) {
            s.fieldCase("red", caseName, frequency, name, field, "...", when, "it is refused")
        }
        func golden(_ caseName: String, _ when: String) {
            s.fieldCase("golden", caseName, occasional, name, field, "...", when, "it succeeds")
        }
        func num(_ key: String) -> String? { field.child(key).map { $0.str } }
        if let v = num("minimum") {
            red(name + " below minimum " + v, occasional, with + " just below " + v)
            golden(name + " at minimum " + v, with + " equal to " + v)
        }
        if let v = num("maximum") {
            red(name + " above maximum " + v, occasional, with + " just above " + v)
            golden(name + " at maximum " + v, with + " equal to " + v)
        }
        if let v = num("exclusiveMinimum") {
            red(name + " at exclusive minimum " + v, occasional, with + " equal to " + v)
        }
        if let v = num("exclusiveMaximum") {
            red(name + " at exclusive maximum " + v, occasional, with + " equal to " + v)
        }
        let minLen = num("minLength"), maxLen = num("maxLength")
        if let m = minLen, m != "0" {
            red(name + " shorter than " + chars(m), occasional, with + " " + characters(m, -1))
            golden(name + " of " + chars(m), with + " " + characters(m, 0))
        }
        if let m = maxLen {
            red(name + " longer than " + chars(m), occasional, with + " " + characters(m, 1))
            if minLen == nil || minLen != maxLen {
                golden(name + " of " + chars(m), with + " " + characters(m, 0))
            }
        }
        if let v = num("minItems"), v != "0" {
            red(name + " with fewer than " + v + " items", occasional, with + " holding fewer than " + v + " items")
        }
        if let v = num("maxItems") {
            red(name + " with more than " + v + " items", occasional, with + " holding more than " + v + " items")
        }
        if num("pattern") != nil {
            red(name + " not matching its pattern", frequent, with + " in the wrong form")
        }
        if enumValues(field).isEnum {
            red(name + " not one of its values", frequent, with + " set to a value it does not allow")
        }
        if let f = num("format"), validatingFormats.contains(f) {
            red(name + " not a valid " + f, frequent, with + " that is not a valid " + f)
        }
    }

    func commandSubject(_ p: Pair) -> Subject {
        let name = p.key.value
        let s = Subject(label: "command " + name, node: p.key, path: pointer("commands", name), yamlKey: "command: " + name,
                        name: kebab(name.replacingOccurrences(of: " ", with: "-")))
        let run = name + " is run"
        var success = golden("...", name + " is run with arguments it accepts", "it exits 0")
        if let ok = p.value.child("exitCodes")?.child("0") { success.then = "it exits 0: " + str(ok) }
        s.success = success
        s.red("usage error", frequent, "...", name + " is run with arguments it does not accept", "it prints how to use it and exits with the usage status")
        for c in pairs(p.value.child("exitCodes")) where c.key.value != "0" {
            s.red("exit " + c.key.value, frequent, "...", run, "it exits " + c.key.value + ": " + str(c.value))
        }
        denied(s, str(p.value.child("permission")), run)
        guardCases(s, p.value.child("guard"), run)
        return s
    }

    /// A job's tests: it runs, it runs twice over the same records without a
    /// second effect, its dependencies fail, and an item fails every time.
    func jobSubject(_ p: Pair) -> Subject {
        let name = p.key.value
        let s = Subject(label: "job " + name, node: p.key, path: pointer("jobs", name), yamlKey: "job: " + name, name: kebab(name))
        var run = "the job " + name + " runs"
        var again = "it runs again over the same records"
        let m = str(p.value.child("trigger")?.child("consumes"))
        if !m.isEmpty {
            run = "a " + m + " arrives for the job " + name
            again = "the same " + m + " arrives again"
        }
        s.success = golden("...", run, "it completes")
        s.cases.append(DerivedCase(name: "runs twice", scenario: "golden", given: "the job " + name + " has run", when: again,
                                   then: "nothing changes a second time", frequency: rare, critical: true))
        for e in items(p.value.child("calls")) {
            guard let dep = dependencies[e.value] else { continue }
            s.byNature("dependency fails " + e.value, e.value + " answers with an error", run, "...")
            s.byNature("dependency times out " + e.value, e.value + " does not answer within " + str(dep.child("timeout")), run, "...")
        }
        if let r = p.value.child("retries") {
            let limit = str(r.child("limit"))
            let then = str(r.child("then")) == "discard" ? "after " + limit + " tries the item is dropped" : "after " + limit + " tries the item is set aside for a person"
            s.red("an item fails every try", occasional, "an item that fails every time it is tried", run, then)
        }
        return s
    }

    func pageSubject(_ p: Pair) -> Subject {
        let name = p.key.value
        let s = Subject(label: "page " + name, node: p.key, path: pointer("pages", name), yamlKey: "page: " + name, name: name)
        let open = "the page " + name + " is opened"
        var success = golden(caller(str(p.value.child("permission"))), open, "it shows the page")
        if !pathParameters(str(p.value.child("route"))).isEmpty { success.when = open + " for a record that exists" }
        s.success = success
        denied(s, str(p.value.child("permission")), open)
        for param in pathParameters(str(p.value.child("route"))) {
            s.red("not found " + param, frequent, "no record has that " + param, open + " for that " + param, "it says the record was not found")
        }
        return s
    }

    func constraintSubject(_ entity: String, _ c: Pair) -> Subject {
        let name = c.key.value
        let s = Subject(label: entity + " constraint " + name, node: c.key, path: pointer("entities", entity, "constraints", name),
                        yamlKey: "entity: " + entity + ", constraint: " + name, name: name.replacingOccurrences(of: "_", with: "-"))
        s.raw = name
        let msg = str(c.value.child("message"))
        switch str(c.value.child("kind")) {
        case "unique":
            s.success = golden("no " + entity + " with the same values exists", "a " + entity + " is saved", "it is saved")
            s.red("duplicate " + name, occasional, "a " + entity + " exists", "another " + entity + " with the same values is saved", "it is refused: " + msg)
        case "check":
            s.success = golden("...", "a " + entity + " keeping it is saved", "it is saved")
            let rules = falsifiers(str(c.value.child("expression")))
            if rules.count < 2 {
                s.red("violates " + name, occasional, "...", "a " + entity + " breaking it is saved", "it is refused: " + msg)
                break
            }
            // A decision table: one case per way the expression can be false.
            for rule in rules {
                let falsehood = rule.joined(separator: " is false and ") + " is false"
                s.red("violates " + name + ": " + falsehood, occasional, "...", "a " + entity + " is saved with " + falsehood, "it is refused: " + msg)
            }
        default:
            break
        }
        return s
    }

    func transitionSubject(_ entity: String, _ i: Int, _ t: YNode) -> Subject {
        let from = str(t.child("from")), to = str(t.child("to"))
        let s = Subject(label: "\(entity) transition \(from) to \(to)", node: t, path: pointer("entities", entity, "transitions", "\(i)"),
                        yamlKey: "entity: \(entity), transition: { from: \(from), to: \(to) }", name: kebab(entity) + "-" + from + "-to-" + to)
        var trigger = str(t.child("trigger"))
        if trigger.isEmpty { trigger = "the move" }
        s.success = golden("the " + entity + " is " + from, trigger + " happens", "the " + entity + " is " + to)
        s.red("from wrong state", frequent, "the " + entity + " is not " + from, trigger + " happens", "it is refused and the state stays as it was")
        return s
    }
}

/// The subject a test names, in the same form as Subject.yamlKey, or "".
func testSubjectKey(_ t: YNode) -> String {
    if let v = t.child("operation"), !v.str.isEmpty { return "operation: " + v.str }
    if let v = t.child("command"), !v.str.isEmpty { return "command: " + v.str }
    if let v = t.child("page"), !v.str.isEmpty { return "page: " + v.str }
    if let v = t.child("job"), !v.str.isEmpty { return "job: " + v.str }
    if let v = t.child("requirement"), !v.str.isEmpty { return "requirement: " + v.str }
    let ent = str(t.child("entity"))
    if ent.isEmpty { return "" }
    if let c = t.child("constraint"), !c.str.isEmpty { return "entity: " + ent + ", constraint: " + c.str }
    if let tr = t.child("transition") {
        return "entity: \(ent), transition: { from: \(str(tr.child("from"))), to: \(str(tr.child("to"))) }"
    }
    return "entity: " + ent // the entity's state machine
}

/// A test in flow style the author can paste under tests.
private func skeleton(_ s: Subject, _ dc: DerivedCase, _ name: String) -> String {
    let covers = dc.name.isEmpty ? "" : ", covers: [" + flowScalar(dc.name) + "]"
    return "\(name): { \(s.yamlKey), scenario: \(dc.scenario)\(covers), given: \(quote(dc.given)), when: \(quote(dc.when)), then: \(quote(dc.then)) }"
}

/// A case name as YAML takes it inside a flow sequence: plain, or quoted
/// when it holds a character that would end or change it.
private func flowScalar(_ s: String) -> String {
    s.contains(where: { ":,[]{}#\"'".contains($0) }) ? quote(s) : s
}

/// A suggested test name: the subject, then the case, without saying the
/// subject's own name twice.
private func testName(_ s: Subject, _ dc: DerivedCase) -> String {
    var c = dc.name
    if !s.raw.isEmpty, let r = c.range(of: s.raw) { c.replaceSubrange(r, with: "") }
    c = c.trimmingCharacters(in: .whitespaces)
    c = kebab(c).replacingOccurrences(of: " ", with: "-").replacingOccurrences(of: "_", with: "-").replacingOccurrences(of: ".", with: "-").lowercased()
    // A decision-table case names clauses of an expression; a test name
    // keeps only their words and numbers.
    c = String(c.unicodeScalars.map { ("a"..."z").contains($0) || ("0"..."9").contains($0) || $0 == "-" ? Character($0) : "-" })
    while c.contains("--") { c = c.replacingOccurrences(of: "--", with: "-") }
    return s.name + "-" + c.trimmingCharacters(in: CharacterSet(charactersIn: "-"))
}

private func listCases(_ s: Subject) -> String {
    if s.cases.isEmpty { return "it has no derived cases, so leave covers out" }
    var seen = Set<String>(), names: [String] = []
    for dc in s.cases where !seen.contains(dc.name) {
        seen.insert(dc.name)
        names.append(dc.name)
    }
    return "its cases are: " + names.joined(separator: ", ")
}

extension Checker {
    func checkTests(_ d: Design) {
        let subjects = d.subjects()
        var byKey: [String: Subject] = [:]
        for s in subjects { byKey[s.yamlKey] = s }
        var golden = Set<ObjectIdentifier>(), red = Set<ObjectIdentifier>()
        var covered: [ObjectIdentifier: Set<String>] = [:]

        for p in pairs(d.root.child("tests")) {
            let name = p.key.value, t = p.value
            let base = ["tests", name]
            let key = testSubjectKey(t)
            if key.isEmpty { continue } // the schema reports a test without a subject
            if byKey[key] == nil && key.hasPrefix("entity: ") && !key.contains(", ") && d.entities[String(key.dropFirst(8))] != nil {
                add(p.key, pointer(base), .testSubject,
                    "test \(name) is about the state machine of \(key.dropFirst(8)), which has no path from an initial to a terminal state; give it transitions, or name one of its constraints or transitions")
                continue
            }
            guard let s = byKey[key] else {
                add(p.key, pointer(base), .testSubject,
                    "test \(name) is about \(key.replacingOccurrences(of: ": ", with: " ")), which is not in the specification; name an operationId, command, page, job, requirement with acceptance criteria, entity with a state machine, or an entity's constraint or transition that exists")
                continue
            }
            let id = ObjectIdentifier(s)
            let scenario = str(t.child("scenario"))
            if scenario == "golden" { golden.insert(id) } else if scenario == "red" { red.insert(id) }
            var known: [String: DerivedCase] = [:]
            for dc in s.cases where known[dc.name] == nil { known[dc.name] = dc }
            for (i, item) in items(t.child("covers")).enumerated() {
                let ptr = pointer(base + ["covers", "\(i)"])
                guard let dc = known[item.value] else {
                    add(item, ptr, .testCase, "\(quote(item.value)) is not a case of \(s.label); \(listCases(s))")
                    continue
                }
                if !scenario.isEmpty && dc.scenario != scenario && t.child("notApplicable") == nil {
                    add(item, ptr, .testCase, "\(quote(item.value)) is a \(dc.scenario) case, but this test is \(scenario); move it to a \(dc.scenario) test")
                    continue
                }
                covered[id, default: []].insert(item.value)
            }
        }

        for s in subjects {
            let id = ObjectIdentifier(s)
            if !golden.contains(id) && !s.whole {
                // Any golden test of the subject is its success case, so the
                // skeleton names no case.
                var success = s.success ?? DerivedCase(name: "", scenario: "golden", given: "...", when: "...", then: "it succeeds", frequency: "")
                success.name = ""
                warn(s.node, s.path, .testGoldenMissing, "\(s.label) has no golden scenario; add one under tests, for example " +
                     skeleton(s, success, s.name + "-succeeds"))
            }
            var hasRedCase = false
            var seen = Set<String>()
            for dc in s.cases {
                if dc.scenario == "red" { hasRedCase = true }
                if covered[id, default: []].contains(dc.name) || seen.contains(dc.name) || !s.chosen(dc.name) { continue }
                seen.insert(dc.name)
                warn(s.node, s.path, .testCaseMissing, "\(s.label) has no \(dc.scenario) scenario for \(quote(dc.name)); add under tests " + skeleton(s, dc, testName(s, dc)))
            }
            if !red.contains(id) && !hasRedCase && !s.whole {
                warn(s.node, s.path, .testRedMissing, "\(s.label) has no red scenario; add one under tests, for example " +
                     skeleton(s, DerivedCase(name: "", scenario: "red", given: "...", when: "...", then: "it is refused", frequency: ""), s.name + "-refused"))
            }
        }
    }
}

/// The ways a boolean expression can be false, each as the clauses that
/// are false together: one per clause of an && made false alone, and for
/// an || every disjunct false at once. The clauses are the expression's
/// own text, so every build names them alike.
func falsifiers(_ text: String) -> [[String]] {
    let expr = trimSpace(text)
    let ors = splitTop(expr, "||")
    if ors.count > 1 {
        var out: [[String]] = [[]]
        for p in ors {
            var next: [[String]] = []
            for prefix in out {
                for f in falsifiers(p) { next.append(prefix + f) }
            }
            out = next
        }
        return out
    }
    let ands = splitTop(expr, "&&")
    if ands.count > 1 { return ands.flatMap { falsifiers($0) } }
    if let inner = unwrap(expr) { return falsifiers(inner) }
    return [[expr]]
}

private func trimSpace(_ s: String) -> String { s.trimmingCharacters(in: .whitespacesAndNewlines) }

/// Splits an expression at an operator that is outside every bracket and
/// string.
private func splitTop(_ expr: String, _ op: String) -> [String] {
    let b = Array(expr.utf8), o = Array(op.utf8)
    var parts: [String] = []
    var depth = 0, start = 0, i = 0
    var quote: UInt8 = 0
    func text(_ from: Int, _ to: Int) -> String { trimSpace(String(decoding: b[from..<to], as: UTF8.self)) }
    while i < b.count {
        let ch = b[i]
        if quote != 0 {
            if ch == UInt8(ascii: "\\") { i += 1 } else if ch == quote { quote = 0 }
        } else if ch == UInt8(ascii: "\"") || ch == UInt8(ascii: "'") {
            quote = ch
        } else if ch == UInt8(ascii: "(") || ch == UInt8(ascii: "[") || ch == UInt8(ascii: "{") {
            depth += 1
        } else if ch == UInt8(ascii: ")") || ch == UInt8(ascii: "]") || ch == UInt8(ascii: "}") {
            depth -= 1
        } else if depth == 0 && i + o.count <= b.count && Array(b[i..<(i + o.count)]) == o {
            parts.append(text(start, i))
            i += o.count - 1
            start = i + 1
        }
        i += 1
    }
    parts.append(text(start, b.count))
    return parts
}

/// Removes the parentheses around a whole expression.
private func unwrap(_ expr: String) -> String? {
    let b = Array(expr.utf8)
    guard b.first == UInt8(ascii: "("), b.last == UInt8(ascii: ")") else { return nil }
    var depth = 0
    for (i, ch) in b.enumerated() {
        if ch == UInt8(ascii: "(") {
            depth += 1
        } else if ch == UInt8(ascii: ")") {
            depth -= 1
            if depth == 0 && i != b.count - 1 { return nil }
        }
    }
    return trimSpace(String(decoding: b[1..<(b.count - 1)], as: UTF8.self))
}

