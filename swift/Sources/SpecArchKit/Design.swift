import Foundation

typealias Pair = (key: YNode, value: YNode)

func pairs(_ n: YNode?) -> [Pair] { n?.kind == .mapping ? n!.pairs : [] }
func items(_ n: YNode?) -> [YNode] { n?.kind == .sequence ? n!.items : [] }
func child(_ n: YNode?, _ key: String) -> YNode? { n?.child(key) }
func str(_ n: YNode?) -> String { n?.str ?? "" }

/// One HTTP operation of a specification.
struct Operation {
    let id: String
    let node: YNode
    let path: String
    let method: String
    let pathItem: YNode

    func pointer(_ tokens: String...) -> String {
        SpecArchKit.pointer(["paths", path, method] + tokens)
    }
}

let methods = ["get", "post", "put", "patch", "delete"]

/// Indexes the named objects of a specification.
final class Design {
    let root: YNode
    var spec: Spec? // the specification on disk, when known
    let entities, enums, permissions, roles, commands, channels, dependencies, pages, algorithms, decisions, sources: [String: YNode]
    let stakeholders, needs, requirements, environments, settings, checks, monitors, questions: [String: YNode]
    var operations: [String: Operation] = [:] // by operationId, the first definition
    var opList: [Operation] = []              // every operation in document order

    init(_ root: YNode) {
        func topMap(_ key: String) -> [String: YNode] {
            var m: [String: YNode] = [:]
            for p in pairs(root.child(key)) { m[p.key.value] = p.value }
            return m
        }
        self.root = root
        entities = topMap("entities")
        enums = topMap("enums")
        permissions = topMap("permissions")
        roles = topMap("roles")
        commands = topMap("commands")
        channels = topMap("channels")
        dependencies = topMap("dependencies")
        pages = topMap("pages")
        algorithms = topMap("algorithms")
        decisions = topMap("decisions")
        sources = topMap("sources")
        stakeholders = topMap("stakeholders")
        needs = topMap("needs")
        requirements = topMap("requirements")
        environments = topMap("environments")
        settings = topMap("configuration")
        checks = topMap("checks")
        monitors = topMap("monitors")
        questions = topMap("questions")
        for p in pairs(root.child("paths")) {
            for m in methods {
                guard let op = p.value.child(m) else { continue }
                let o = Operation(id: str(op.child("operationId")), node: op, path: p.key.value, method: m, pathItem: p.value)
                opList.append(o)
                if !o.id.isEmpty && operations[o.id] == nil { operations[o.id] = o }
            }
        }
    }

    /// A channel's message node for "channel/Message", or nil.
    func message(_ ref: String) -> YNode? {
        guard let slash = ref.firstIndex(of: "/") else { return nil }
        let ch = String(ref[..<slash]), msg = String(ref[ref.index(after: slash)...])
        return child(child(channels[ch], "messages"), msg)
    }

    /// The values a field may take when it is an enum, by $ref or inline,
    /// and whether it is one.
    func enumValues(_ field: YNode?) -> (values: [String: YNode], name: String, isEnum: Bool) {
        let ref = str(child(field, "$ref"))
        if ref.hasPrefix("#/enums/") {
            let name = String(ref.dropFirst("#/enums/".count))
            guard let e = enums[name] else { return ([:], name, false) }
            var values: [String: YNode] = [:]
            for v in items(e.child("enum")) { values[v.value] = v }
            return (values, name, true)
        }
        if let list = child(field, "enum") {
            var values: [String: YNode] = [:]
            for v in items(list) { values[v.value] = v }
            return (values, "", true)
        }
        return ([:], "", false)
    }

    func isTrigger(_ t: String) -> Bool {
        operations[t] != nil || commands[t] != nil || algorithms[t] != nil || message(t) != nil
    }

    /// The prefixes of the external requirement sets declared under sources.
    func requirementSetPrefixes() -> [String: YNode] {
        var out: [String: YNode] = [:]
        for (_, src) in sources where str(src.child("kind")) == "requirement-set" {
            let prefix = str(src.child("prefix"))
            if !prefix.isEmpty { out[prefix] = src }
        }
        return out
    }

    /// Whether the specification has any section of a stage.
    func covers(_ stage: String) -> Bool {
        if let spec { return spec.covers(stage) }
        return sectionsOf(stage).contains { root.child($0) != nil }
    }
}

/// An entity's fields by name.
func fieldsOf(_ entity: YNode?) -> [String: YNode] {
    var m: [String: YNode] = [:]
    for p in pairs(child(entity, "properties")) { m[p.key.value] = p.value }
    return m
}

/// "; did you mean X?" for a close name, or a list of the valid names when
/// there are few, or "".
func suggest(_ name: String, _ valid: [String: YNode]) -> String {
    let names = valid.keys.sorted(by: byteLess)
    var best = "", bestDist = 3
    for n in names {
        let dist = distance(name.lowercased(), n.lowercased())
        if dist < bestDist { best = n; bestDist = dist }
    }
    if !best.isEmpty { return "; did you mean \(best)?" }
    if names.isEmpty { return "; there are none in the specification" }
    if names.count <= 8 { return "; use one of " + names.joined(separator: ", ") }
    return ""
}

func distance(_ a: String, _ b: String) -> Int {
    let ra = Array(a.unicodeScalars), rb = Array(b.unicodeScalars)
    var prev = Array(0...rb.count)
    if ra.isEmpty { return rb.count }
    for i in 1...ra.count {
        var cur = [i] + Array(repeating: 0, count: rb.count)
        for j in stride(from: 1, through: rb.count, by: 1) {
            let cost = ra[i - 1] == rb[j - 1] ? 0 : 1
            cur[j] = min(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + cost)
        }
        prev = cur
    }
    return prev[rb.count]
}

/// Calls fn on every mapping in the tree, with its path.
func walk(_ n: YNode?, _ path: [String], _ fn: (YNode, [String]) -> Void) {
    guard let n else { return }
    switch n.kind {
    case .mapping:
        fn(n, path)
        for p in n.pairs { walk(p.value, path + [p.key.value], fn) }
    case .sequence:
        for (i, item) in n.items.enumerated() { walk(item, path + ["\(i)"], fn) }
    case .scalar:
        break
    }
}

let pathParam = try! NSRegularExpression(pattern: "\\{([^{}]+)\\}")

func pathParameters(_ path: String) -> [String] {
    pathParam.matches(in: path, range: NSRange(path.startIndex..., in: path)).map {
        String(path[Range($0.range(at: 1), in: path)!])
    }
}

/// The stackSpecificKey pattern of the design schema.
let stackKeyPattern = try! NSRegularExpression(pattern: "^x-(oapi-codegen|ogen|openapi-generator|codegen|protoc|grpc|go|java|kotlin|python|typescript|javascript|rust|swift|dotnet|csharp|php|ruby|framework|router|middleware|cli-library|server|servers|host|port|deploy|deployment|environment)(-|$)")

extension Checker {
    func checkDesign(_ d: Design) {
        checkRefs(d)
        checkIntegers()
        checkRequirementLinks(d.root, [], d)
        checkCitations(d.root, [], d)
        checkRequirementsStage(d)
        checkEnums(d)
        checkEntities(d)
        checkLookups(d)
        checkOperations(d)
        checkCommands(d)
        checkDependencies(d)
        checkSession(d)
        checkPages(d)
        checkDecisions(d)
        checkAccess(d)
        checkExpressions(d)
        checkTests(d)
        checkTestData(d)
        checkDeploymentStage(d)
        checkTraceability(d)
        checkQuestions(d)
        checkOrigin(d.root, d.decisions) { path in path.count == 2 && path[0] == "decisions" }
        checkOriginTracked(d)
    }

    /// Finds every $ref in the file and checks its target exists.
    func checkRefs(_ d: Design) {
        walk(d.root, []) { n, path in
            for p in n.pairs where p.key.value == "$ref" && p.value.kind == .scalar {
                let ref = p.value.value
                let ptr = pointer(path + ["$ref"])
                if ref.hasPrefix("#/entities/") {
                    let name = String(ref.dropFirst("#/entities/".count))
                    if d.entities[name] == nil {
                        add(p.value, ptr, .refType, "\(name) is not an entity of the specification\(suggest(name, d.entities))")
                    }
                } else if ref.hasPrefix("#/enums/") {
                    let name = String(ref.dropFirst("#/enums/".count))
                    if d.enums[name] == nil {
                        add(p.value, ptr, .refType, "\(name) is not an enum of the specification\(suggest(name, d.enums))")
                    }
                }
            }
        }
    }

    /// Checks every satisfies and verifies list names a requirement of the
    /// specification, or one of an external set declared under sources.
    func checkRequirementLinks(_ root: YNode?, _ base: [String], _ d: Design) {
        let prefixes = d.requirementSetPrefixes()
        walk(root, base) { n, path in
            for key in ["satisfies", "verifies"] {
                for (i, item) in items(n.child(key)).enumerated() {
                    let link = str(item)
                    if d.requirements[link] != nil { continue }
                    let dash = link.firstIndex(of: "-")
                    let prefix = dash.map { String(link[..<$0]) } ?? link
                    if dash != nil && prefixes[prefix] != nil { continue }
                    let ptr = pointer(path + [key, "\(i)"])
                    if d.requirements.isEmpty && prefixes.isEmpty {
                        add(item, ptr, .requirement, "\(link) is not a requirement: the specification has none under requirements and no source of kind requirement-set; add the requirement, or declare the set under sources with prefix \(prefix)")
                    } else if !d.requirements.isEmpty {
                        add(item, ptr, .requirement, "\(link) is not a requirement of the specification\(suggest(link, d.requirements))")
                    } else {
                        add(item, ptr, .requirement, "\(link) is not in a declared requirement set; the sets under sources have the prefixes \(prefixes.keys.sorted(by: byteLess).joined(separator: ", "))")
                    }
                }
            }
        }
    }

    /// Checks every citation names a declared source.
    func checkCitations(_ root: YNode?, _ base: [String], _ d: Design) {
        walk(root, base) { n, path in
            for (i, item) in items(n.child("cites")).enumerated() {
                let srcNode = item.child("source")
                let name = str(srcNode)
                if name.isEmpty || d.sources[name] != nil { continue }
                let ptr = pointer(path + ["cites", "\(i)", "source"])
                if d.sources.isEmpty {
                    add(srcNode, ptr, .source, "\(name) is not a declared source: the specification declares none; add it under sources in \(rootFile)")
                    continue
                }
                add(srcNode, ptr, .source, "\(name) is not a source declared under sources\(suggest(name, d.sources))")
            }
        }
    }

    /// Checks the links inside the requirements stage: a requirement's
    /// needs exist, a need's stakeholders exist.
    func checkRequirementsStage(_ d: Design) {
        for (name, need) in d.needs {
            for (i, sh) in items(need.child("stakeholders")).enumerated() where !sh.value.isEmpty && d.stakeholders[sh.value] == nil {
                add(sh, pointer("needs", name, "stakeholders", "\(i)"), .stakeholder,
                    "\(sh.value) is not a stakeholder of the specification\(suggest(sh.value, d.stakeholders))")
            }
        }
        for (name, req) in d.requirements {
            for (i, n) in items(req.child("needs")).enumerated() where !n.value.isEmpty && d.needs[n.value] == nil {
                add(n, pointer("requirements", name, "needs", "\(i)"), .need,
                    "\(n.value) is not a need of the specification\(suggest(n.value, d.needs))")
            }
        }
    }

    /// Checks the links inside the deployment, commissioning and operation
    /// stages, and that no secret carries a value.
    func checkDeploymentStage(_ d: Design) {
        for (name, env) in d.environments {
            let next = env.child("promotesTo")
            let v = str(next)
            if !v.isEmpty && d.environments[v] == nil {
                add(next, pointer("environments", name, "promotesTo"), .environment,
                    "\(v) is not an environment of the specification\(suggest(v, d.environments))")
            }
        }
        for (name, chk) in d.checks {
            let env = chk.child("environment")
            let v = str(env)
            if !v.isEmpty && d.environments[v] == nil {
                add(env, pointer("checks", name, "environment"), .environment,
                    "\(v) is not an environment of the specification\(suggest(v, d.environments))")
            }
        }
        for (name, mon) in d.monitors {
            let env = mon.child("environment")
            let v = str(env)
            if !v.isEmpty && d.environments[v] == nil {
                add(env, pointer("monitors", name, "environment"), .environment,
                    "\(v) is not an environment of the specification\(suggest(v, d.environments))")
            }
        }
        for (name, setting) in d.settings where str(setting.child("secret")) == "true" {
            if let def = child(setting.child("schema"), "default") {
                add(def, pointer("configuration", name, "schema", "default"), .secretValue,
                    "\(name) is a secret, and a secret's value is never written in a specification; remove the default and say in the description where the value comes from")
            }
        }
    }

    /// Reports what the stages a specification covers leave open: a need no
    /// requirement refines, a requirement without acceptance criteria, a
    /// requirement nothing satisfies once there is a design, and a
    /// requirement nothing verifies once there are tests, checks or monitors. All are
    /// warnings.
    func checkTraceability(_ d: Design) {
        var satisfied = Set<String>(), verified = Set<String>()
        walk(d.root, []) { n, _ in
            for item in items(n.child("satisfies")) { satisfied.insert(item.value) }
            for item in items(n.child("verifies")) { verified.insert(item.value) }
        }
        for impl in d.spec?.implementations ?? [] {
            let doc = parseYAML(String(decoding: impl.data, as: UTF8.self))
            walk(doc.root, []) { n, _ in
                for item in items(n.child("satisfies")) { satisfied.insert(item.value) }
            }
        }
        var refined = Set<String>()
        for (_, req) in d.requirements {
            for n in items(req.child("needs")) { refined.insert(n.value) }
        }
        let hasDesign = d.covers("design")
        let hasTests = !pairs(d.root.child("tests")).isEmpty || !d.checks.isEmpty || !d.monitors.isEmpty
        for p in pairs(d.root.child("needs")) {
            if str(p.value.child("status")) == "rejected" { continue } // a rejected need will not be met, so no requirement refines it
            if !refined.contains(p.key.value) {
                warn(p.key, pointer("needs", p.key.value), .needUnrefined,
                     "no requirement refines need \(p.key.value); add a requirement with needs: [\(p.key.value)], or set the need's status to rejected")
            }
        }
        for p in pairs(d.root.child("requirements")) {
            let id = p.key.value
            let ptr = pointer("requirements", id)
            let status = str(p.value.child("status"))
            if status == "rejected" || status == "retired" { continue }
            if p.value.child("acceptance") == nil {
                warn(p.key, ptr, .acceptanceMissing, "requirement \(id) has no acceptance criteria, so no test can show it is met; add acceptance with one verifiable sentence per criterion")
            }
            if hasDesign && !satisfied.contains(id) {
                warn(p.key, ptr, .requirementUnsatisfied, "no design element satisfies requirement \(id); add satisfies: [\(id)] to the entity, operation, command, page, algorithm or decision that meets it")
            }
            if hasTests && !verified.contains(id) {
                warn(p.key, ptr, .requirementUnverified, "no test, commissioning check or monitor verifies requirement \(id); add verifies: [\(id)] to the test that shows it is met")
            }
        }
    }

    func checkEnums(_ d: Design) {
        for (name, e) in d.enums {
            var values: [String: YNode] = [:]
            for v in items(e.child("enum")) { values[v.value] = v }
            for p in pairs(e.child("valueDescriptions")) where values[p.key.value] == nil {
                add(p.key, pointer("enums", name, "valueDescriptions", p.key.value), .enumValue,
                    "\(p.key.value) is not a value of enum \(name); describe only its values\(suggest(p.key.value, values))")
            }
        }
    }

    func checkFieldList(_ list: YNode?, _ ptr: [String], _ fields: [String: YNode], _ entity: String, _ what: String) {
        for (i, item) in items(list).enumerated() {
            let name = str(item)
            if !name.isEmpty && fields[name] == nil {
                add(item, pointer(ptr + ["\(i)"]), .field, "\(name) is not a field of \(entity), so it cannot be \(what)\(suggest(name, fields))")
            }
        }
    }

    func checkEntities(_ d: Design) {
        for (name, e) in d.entities {
            let fields = fieldsOf(e)
            let base = ["entities", name]
            checkFieldList(e.child("required"), base + ["required"], fields, name, "required")
            checkFieldList(e.child("primaryKey"), base + ["primaryKey"], fields, name, "part of the primary key")
            for p in pairs(e.child("constraints")) where str(p.value.child("kind")) == "unique" {
                checkFieldList(p.value.child("fields"), base + ["constraints", p.key.value, "fields"], fields, name, "part of a unique constraint")
            }
            for p in pairs(e.child("relations")) {
                checkRelation(d, name, fields, p)
            }
            checkStates(d, name, e, fields)
            checkValidity(name, e, fields)
            checkStored(name, e, fields)
        }
    }

    func checkRelation(_ d: Design, _ entity: String, _ fields: [String: YNode], _ p: Pair) {
        let base = ["entities", entity, "relations", p.key.value]
        let targetNode = p.value.child("target")
        let target = str(targetNode)
        if !target.isEmpty && d.entities[target] == nil {
            add(targetNode, pointer(base + ["target"]), .relationTarget, "\(target) is not an entity of the specification\(suggest(target, d.entities))")
            return
        }
        let viaNode = p.value.child("via")
        let via = str(viaNode)
        if via.isEmpty || target.isEmpty { return }
        let ptr = pointer(base + ["via"])
        switch str(p.value.child("kind")) {
        case "many-to-one", "one-to-one":
            if fields[via] == nil {
                add(viaNode, ptr, .relationVia, "\(via) is not a field of \(entity); for this kind of relation, via names the field on \(entity) that holds the key\(suggest(via, fields))")
            }
        case "one-to-many":
            let targetFields = fieldsOf(d.entities[target])
            if targetFields[via] == nil {
                add(viaNode, ptr, .relationVia, "\(via) is not a field of \(target); for one-to-many, via names the field on the target that points back\(suggest(via, targetFields))")
            }
        case "many-to-many":
            if d.entities[via] == nil {
                add(viaNode, ptr, .relationVia, "\(via) is not an entity of the specification; for many-to-many, via names the join entity\(suggest(via, d.entities))")
            }
        default:
            break
        }
    }

    func checkStates(_ d: Design, _ name: String, _ e: YNode, _ fields: [String: YNode]) {
        let sfNode = e.child("stateField")
        let sf = str(sfNode)
        if sf.isEmpty { return }
        let base = ["entities", name]
        guard let field = fields[sf] else {
            add(sfNode, pointer(base + ["stateField"]), .stateField, "\(sf) is not a field of \(name)\(suggest(sf, fields))")
            return
        }
        let (values, _, isEnum) = d.enumValues(field)
        if !isEnum {
            if str(field.child("$ref")).hasPrefix("#/enums/") { return }
            add(sfNode, pointer(base + ["stateField"]), .stateField, "\(sf) is not an enum field, so its values cannot be states; give it a $ref to an enum")
            return
        }
        for (i, t) in items(e.child("transitions")).enumerated() {
            let tp = base + ["transitions", "\(i)"]
            for end in ["from", "to"] {
                let n = t.child(end)
                let v = str(n)
                if !v.isEmpty && values[v] == nil {
                    add(n, pointer(tp + [end]), .stateValue, "\(v) is not a value of \(sf)\(suggest(v, values))")
                }
            }
            let trig = t.child("trigger")
            let tr = str(trig)
            if !tr.isEmpty && !d.isTrigger(tr) {
                add(trig, pointer(tp + ["trigger"]), .trigger, "\(tr) is not an operationId, a command, a channel/Message or an algorithm of the specification; name the one that causes this move")
            }
        }
    }

    func checkOperations(_ d: Design) {
        var seen = Set<String>()
        for o in d.opList {
            if !o.id.isEmpty {
                if seen.contains(o.id), let first = d.operations[o.id] {
                    add(o.node.child("operationId"), o.pointer("operationId"), .duplicateOperation,
                        "operationId \(o.id) is already used by \(first.method.uppercased()) \(first.path); give this operation its own name")
                }
                seen.insert(o.id)
            }
            if let alg = o.node.child("algorithm"), d.algorithms[alg.value] == nil {
                add(alg, o.pointer("algorithm"), .algorithm, "\(alg.value) is not an algorithm of the specification\(suggest(alg.value, d.algorithms))")
            }
            for (i, e) in items(o.node.child("emits")).enumerated() where d.message(e.value) == nil {
                add(e, o.pointer("emits", "\(i)"), .emits, "\(e.value) does not name a channel and one of its messages; write channel/Message for a message declared under channels")
            }
            checkPathParameters(o)
            checkCalls(d, o)
            checkIdempotencyKey(o)
            checkExposed(d, o)
            checkListOf(d, o)
            checkLimits(o)
            checkProblems(d, o)
            checkGuard(d, o.node.child("guard"), o.pointer("guard"))
        }
    }

    func checkPathParameters(_ o: Operation) {
        let inTemplate = Set(pathParameters(o.path))
        var declared = Set<String>()
        func check(_ list: YNode?, _ base: [String]) {
            for (i, p) in items(list).enumerated() where str(p.child("in")) == "path" {
                let name = str(p.child("name"))
                declared.insert(name)
                if !inTemplate.contains(name) {
                    add(p.child("name"), pointer(base + ["\(i)", "name"]), .pathParameter,
                        "path parameter \(name) does not appear in \(o.path); add {\(name)} to the path or remove the parameter")
                }
            }
        }
        check(o.pathItem.child("parameters"), ["paths", o.path, "parameters"])
        check(o.node.child("parameters"), ["paths", o.path, o.method, "parameters"])
        for n in inTemplate.sorted(by: byteLess) where !declared.contains(n) {
            add(o.node, o.pointer(), .pathParameter, "{\(n)} in \(o.path) has no path parameter; declare it with in: path and required: true")
        }
    }

    func checkCommands(_ d: Design) {
        for (name, cmd) in d.commands {
            if let alg = cmd.child("algorithm"), d.algorithms[alg.value] == nil {
                add(alg, pointer("commands", name, "algorithm"), .algorithm, "\(alg.value) is not an algorithm of the specification\(suggest(alg.value, d.algorithms))")
            }
            let args = items(cmd.child("arguments"))
            for (i, a) in args.enumerated() where str(a.child("repeatable")) == "true" && i != args.count - 1 {
                add(a.child("repeatable"), pointer("commands", name, "arguments", "\(i)", "repeatable"), .schema,
                    "only the last argument may be repeatable; move this argument to the end or make it a single value")
            }
            checkGuard(d, cmd.child("guard"), pointer("commands", name, "guard"))
        }
    }

    func checkPages(_ d: Design) {
        var opNames: [String: YNode] = [:]
        for (id, o) in d.operations { opNames[id] = o.node }
        for (name, pg) in d.pages {
            let base = ["pages", name]
            let entNode = pg.child("entity")
            let ent = str(entNode)
            if !ent.isEmpty && d.entities[ent] == nil {
                add(entNode, pointer(base + ["entity"]), .refType, "\(ent) is not an entity of the specification\(suggest(ent, d.entities))")
            } else if !ent.isEmpty {
                let fields = fieldsOf(d.entities[ent])
                checkFieldList(pg.child("columns"), base + ["columns"], fields, ent, "a column")
                checkFieldList(pg.child("fields"), base + ["fields"], fields, ent, "shown on this page")
                checkFieldList(pg.child("filters"), base + ["filters"], fields, ent, "a filter")
            }
            for key in ["source", "submit"] {
                let n = pg.child(key)
                let id = str(n)
                if !id.isEmpty && d.operations[id] == nil {
                    add(n, pointer(base + [key]), .operation, "\(id) is not an operationId of the specification\(suggest(id, opNames))")
                }
            }
            for (i, a) in items(pg.child("actions")).enumerated() {
                let tn = a.child("target")
                let t = str(tn)
                let ptr = pointer(base + ["actions", "\(i)", "target"])
                switch str(a.child("kind")) {
                case "navigate":
                    if !t.isEmpty && d.pages[t] == nil {
                        add(tn, ptr, .page, "\(t) is not a page of the specification\(suggest(t, d.pages))")
                    }
                case "operation":
                    if !t.isEmpty && d.operations[t] == nil {
                        add(tn, ptr, .operation, "\(t) is not an operationId of the specification\(suggest(t, opNames))")
                    }
                default:
                    break
                }
            }
        }
    }

    func checkDecisions(_ d: Design) {
        for (id, dec) in d.decisions {
            if let s = dec.child("supersededBy"), d.decisions[s.value] == nil {
                add(s, pointer("decisions", id, "supersededBy"), .decision, "\(s.value) is not a decision of the specification\(suggest(s.value, d.decisions))")
            }
        }
    }

    /// The fail-closed rule: every permission used is declared, and every
    /// declared permission is granted by a role or is public.
    func checkAccess(_ d: Design) {
        func use(_ n: YNode?, _ ptr: String, _ who: String) {
            let p = str(n)
            if !p.isEmpty && d.permissions[p] == nil {
                let hint = suggest(p, d.permissions).replacingOccurrences(of: "; there are none in the specification", with: "")
                add(n, ptr, .permissionUndeclared, "\(who) needs permission \(p), which is not declared; add it under permissions\(hint)")
            }
        }
        for o in d.opList { use(o.node.child("permission"), o.pointer("permission"), "operation " + o.id) }
        for (name, cmd) in d.commands { use(cmd.child("permission"), pointer("commands", name, "permission"), "command " + name) }
        for (name, pg) in d.pages {
            use(pg.child("permission"), pointer("pages", name, "permission"), "page " + name)
            for (i, a) in items(pg.child("actions")).enumerated() {
                use(a.child("permission"), pointer("pages", name, "actions", "\(i)", "permission"), "an action of page " + name)
            }
        }
        var granted = Set<String>()
        for (name, r) in d.roles {
            for (i, p) in items(r.child("permissions")).enumerated() {
                granted.insert(p.value)
                use(p, pointer("roles", name, "permissions", "\(i)"), "role " + name)
            }
        }
        for p in pairs(d.root.child("permissions")) where p.key.value != "public" && !granted.contains(p.key.value) {
            add(p.key, pointer("permissions", p.key.value), .permissionUngranted,
                "no role grants \(p.key.value), so nobody can use what it guards; add it to a role, or use public if it is meant for everyone")
        }
    }

    func checkBoundaryDesign() {
        walk(root, []) { n, path in
            for p in n.pairs where matches(stackKeyPattern, p.key.value) {
                add(p.key, pointer(path + [p.key.value]), .stackKey,
                    "\(p.key.value) belongs to one implementation, not to the design; move it to the implementation file (*\(implementationSuffix)), under mappings or generators")
            }
        }
    }
}
