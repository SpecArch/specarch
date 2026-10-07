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

    init(label: String, node: YNode, path: String, yamlKey: String, name: String) {
        self.label = label
        self.node = node
        self.path = path
        self.yamlKey = yamlKey
        self.name = name
    }

    func red(_ name: String, _ given: String, _ when: String, _ then: String) {
        cases.append(DerivedCase(name: name, scenario: "red", given: given, when: when, then: then))
    }

    func golden(_ name: String, _ given: String, _ when: String, _ then: String) {
        cases.append(DerivedCase(name: name, scenario: "golden", given: given, when: when, then: then))
    }
}

/// A scenario the rest of the file says a subject needs.
struct DerivedCase {
    var name: String
    var scenario: String
    var given: String
    var when: String
    var then: String
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

private func denied(_ s: Subject, _ perm: String, _ what: String) {
    if perm.isEmpty || perm == "public" { return }
    s.red("denied without " + perm, "a caller without " + perm, what, "it is refused as not allowed")
}

extension Design {
    /// Every subject of the file with its derived cases: operations,
    /// commands, pages, then each entity's constraints and transitions.
    func subjects() -> [Subject] {
        var out: [Subject] = []
        for o in opList where !o.id.isEmpty { out.append(operationSubject(o)) }
        for p in pairs(root.child("commands")) { out.append(commandSubject(p)) }
        for p in pairs(root.child("pages")) { out.append(pageSubject(p)) }
        for e in pairs(root.child("entities")) {
            for c in pairs(e.value.child("constraints")) { out.append(constraintSubject(e.key.value, c)) }
            for (i, t) in items(e.value.child("transitions")).enumerated() { out.append(transitionSubject(e.key.value, i, t)) }
        }
        return out
    }

    func operationSubject(_ o: Operation) -> Subject {
        let s = Subject(label: "operation " + o.id, node: o.node, path: o.pointer(), yamlKey: "operation: " + o.id, name: kebab(o.id))
        let call = o.id + " is called"
        let (body, bodyRequired) = requestFields(o.node)
        for f in bodyRequired {
            s.red("missing " + f, "...", o.id + " is called without " + f, "it is refused")
        }
        for n in body.keys.sorted(by: byteLess) {
            limitCases(s, n, body[n], call)
        }
        for p in items(o.pathItem.child("parameters")) + items(o.node.child("parameters")) {
            let name = str(p.child("name"))
            limitCases(s, name, p.child("schema"), call)
            if str(p.child("in")) == "path" {
                s.red("not found " + name, "no record has that " + name, o.id + " is called with that " + name, "it is refused as not found")
            }
        }
        let ent = responseEntity(o.node)
        if !ent.isEmpty {
            for r in pairs(child(entities[ent], "relations")) {
                let kind = str(r.value.child("kind")), via = str(r.value.child("via"))
                if (kind == "many-to-one" || kind == "one-to-one") && body[via] != nil {
                    s.red("not found " + via, "no " + str(r.value.child("target")) + " has that " + via, o.id + " is called with that " + via, "it is refused as not found")
                }
            }
            if o.method == "post" && child(o.node.child("responses"), "201") != nil {
                for c in pairs(child(entities[ent], "constraints")) where str(c.value.child("kind")) == "unique" {
                    s.red("duplicate " + c.key.value, "a " + ent + " that " + c.key.value + " would clash with exists", call, "it is refused as a duplicate")
                }
            }
        }
        denied(s, str(o.node.child("permission")), call)
        var seenChannel = Set<String>()
        for e in items(o.node.child("emits")) {
            let ch = String(e.value.split(separator: "/", maxSplits: 1, omittingEmptySubsequences: false)[0])
            if !seenChannel.contains(ch) {
                seenChannel.insert(ch)
                s.red("dependency fails " + ch, ch + " cannot take the message", call, "...")
            }
        }
        for r in pairs(o.node.child("responses")) {
            let code = r.key.value
            if code.count == 3, code.first == "4" || code.first == "5" {
                s.red("response " + code, "...", call, "it answers " + code + ": " + str(r.value.child("description")))
            }
        }
        return s
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
        func num(_ key: String) -> String? { field.child(key).map { $0.str } }
        if let v = num("minimum") {
            s.red(name + " below minimum " + v, "...", with + " just below " + v, "it is refused")
            s.golden(name + " at minimum " + v, "...", with + " equal to " + v, "it succeeds")
        }
        if let v = num("maximum") {
            s.red(name + " above maximum " + v, "...", with + " just above " + v, "it is refused")
            s.golden(name + " at maximum " + v, "...", with + " equal to " + v, "it succeeds")
        }
        if let v = num("exclusiveMinimum") {
            s.red(name + " at exclusive minimum " + v, "...", with + " equal to " + v, "it is refused")
        }
        if let v = num("exclusiveMaximum") {
            s.red(name + " at exclusive maximum " + v, "...", with + " equal to " + v, "it is refused")
        }
        let minLen = num("minLength"), maxLen = num("maxLength")
        if let m = minLen, m != "0" {
            s.red(name + " shorter than " + chars(m), "...", with + " " + characters(m, -1), "it is refused")
            s.golden(name + " of " + chars(m), "...", with + " " + characters(m, 0), "it succeeds")
        }
        if let m = maxLen {
            s.red(name + " longer than " + chars(m), "...", with + " " + characters(m, 1), "it is refused")
            if minLen == nil || minLen != maxLen {
                s.golden(name + " of " + chars(m), "...", with + " " + characters(m, 0), "it succeeds")
            }
        }
        if let v = num("minItems"), v != "0" {
            s.red(name + " with fewer than " + v + " items", "...", with + " holding fewer than " + v + " items", "it is refused")
        }
        if let v = num("maxItems") {
            s.red(name + " with more than " + v + " items", "...", with + " holding more than " + v + " items", "it is refused")
        }
        if num("pattern") != nil {
            s.red(name + " not matching its pattern", "...", with + " in the wrong form", "it is refused")
        }
        if enumValues(field).isEnum {
            s.red(name + " not one of its values", "...", with + " set to a value it does not allow", "it is refused")
        }
        if let f = num("format"), validatingFormats.contains(f) {
            s.red(name + " not a valid " + f, "...", with + " that is not a valid " + f, "it is refused")
        }
    }

    func commandSubject(_ p: Pair) -> Subject {
        let name = p.key.value
        let s = Subject(label: "command " + name, node: p.key, path: pointer("commands", name), yamlKey: "command: " + name,
                        name: kebab(name.replacingOccurrences(of: " ", with: "-")))
        let run = name + " is run"
        s.red("usage error", "...", name + " is run with arguments it does not accept", "it prints how to use it and exits with the usage status")
        for c in pairs(p.value.child("exitCodes")) where c.key.value != "0" {
            s.red("exit " + c.key.value, "...", run, "it exits " + c.key.value + ": " + str(c.value))
        }
        denied(s, str(p.value.child("permission")), run)
        return s
    }

    func pageSubject(_ p: Pair) -> Subject {
        let name = p.key.value
        let s = Subject(label: "page " + name, node: p.key, path: pointer("pages", name), yamlKey: "page: " + name, name: name)
        let open = "the page " + name + " is opened"
        denied(s, str(p.value.child("permission")), open)
        for param in pathParameters(str(p.value.child("route"))) {
            s.red("not found " + param, "no record has that " + param, open + " for that " + param, "it says the record was not found")
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
            s.red("duplicate " + name, "a " + entity + " exists", "another " + entity + " with the same values is saved", "it is refused: " + msg)
        case "check":
            s.red("violates " + name, "...", "a " + entity + " breaking it is saved", "it is refused: " + msg)
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
        s.red("from wrong state", "the " + entity + " is not " + from, trigger + " happens", "it is refused and the state stays as it was")
        return s
    }
}

/// The subject a test names, in the same form as Subject.yamlKey, or "".
func testSubjectKey(_ t: YNode) -> String {
    if let v = t.child("operation"), !v.str.isEmpty { return "operation: " + v.str }
    if let v = t.child("command"), !v.str.isEmpty { return "command: " + v.str }
    if let v = t.child("page"), !v.str.isEmpty { return "page: " + v.str }
    let ent = str(t.child("entity"))
    if ent.isEmpty { return "" }
    if let c = t.child("constraint"), !c.str.isEmpty { return "entity: " + ent + ", constraint: " + c.str }
    if let tr = t.child("transition") {
        return "entity: \(ent), transition: { from: \(str(tr.child("from"))), to: \(str(tr.child("to"))) }"
    }
    return ""
}

/// A test in flow style the author can paste under tests.
private func skeleton(_ s: Subject, _ dc: DerivedCase, _ name: String) -> String {
    let covers = dc.name.isEmpty ? "" : ", covers: [" + dc.name + "]"
    return "\(name): { \(s.yamlKey), scenario: \(dc.scenario)\(covers), given: \(quote(dc.given)), when: \(quote(dc.when)), then: \(quote(dc.then)) }"
}

/// A suggested test name: the subject, then the case, without saying the
/// subject's own name twice.
private func testName(_ s: Subject, _ dc: DerivedCase) -> String {
    var c = dc.name
    if !s.raw.isEmpty, let r = c.range(of: s.raw) { c.replaceSubrange(r, with: "") }
    c = c.trimmingCharacters(in: .whitespaces)
    c = kebab(c).replacingOccurrences(of: " ", with: "-").replacingOccurrences(of: "_", with: "-").replacingOccurrences(of: ".", with: "-").lowercased()
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
            guard let s = byKey[key] else {
                add(p.key, pointer(base), .testSubject,
                    "test \(name) is about \(key.replacingOccurrences(of: ": ", with: " ")), which is not in the specification; name an operationId, command, page, or an entity's constraint or transition that exists")
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
            if !golden.contains(id) {
                warn(s.node, s.path, .testGoldenMissing, "\(s.label) has no golden scenario; add one under tests, for example " +
                     skeleton(s, DerivedCase(name: "", scenario: "golden", given: "...", when: "...", then: "it succeeds"), s.name + "-succeeds"))
            }
            var hasRedCase = false
            var seen = Set<String>()
            for dc in s.cases {
                if dc.scenario == "red" { hasRedCase = true }
                if covered[id, default: []].contains(dc.name) || seen.contains(dc.name) { continue }
                seen.insert(dc.name)
                warn(s.node, s.path, .testCaseMissing, "\(s.label) has no \(dc.scenario) scenario for \(quote(dc.name)); add under tests " + skeleton(s, dc, testName(s, dc)))
            }
            if !red.contains(id) && !hasRedCase {
                warn(s.node, s.path, .testRedMissing, "\(s.label) has no red scenario; add one under tests, for example " +
                     skeleton(s, DerivedCase(name: "", scenario: "red", given: "...", when: "...", then: "it is refused"), s.name + "-refused"))
            }
        }
    }
}
