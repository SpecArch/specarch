import Foundation

/// The specifications given together in one run, by their info.name; a
/// specification without one is not listed (ADR-112).
typealias Given = [String: [Spec]]

/// Lists the specifications by name.
func makeGiven(_ specs: [Spec]) -> Given {
    var g: Given = [:]
    for s in specs {
        let name = specName(s.root)
        if !name.isEmpty { g[name, default: []].append(s) }
    }
    return g
}

/// A specification's info.name, or "".
func specName(_ root: YNode?) -> String {
    str(root?.child("info")?.child("name"))
}

/// A reference into another specification that is not resolved.
struct Reference {
    let path: String
    let text: String
    let waits: String
}

/// Splits a reference into the dependency and the pointer, the part after
/// the #; nil for a name of this specification.
func crossRef(_ s: String) -> (dep: String, pointer: String)? {
    guard let hash = s.firstIndex(of: "#"), hash != s.startIndex else { return nil }
    return (String(s[..<hash]), String(s[s.index(after: hash)...]))
}

/// Whether a name is a reference into another specification.
func isCross(_ s: String) -> Bool { crossRef(s) != nil }

/// The dependency a calls entry names: the entry, or its part before the #
/// when it names one of the dependency's operations.
func callDependency(_ s: String) -> String { crossRef(s)?.dep ?? s }

/// One reference found in a specification, with what it stands in.
private struct Ref {
    let node: YNode
    let path: [String]
    let text: String
    let dep: String
    let pointer: String
    let kind: String   // ref, target, entity, source, picker, calls, fulfilledBy, permission, client or role
    let owner: YNode   // the relation, page or picker it stands in
    let page: YNode?   // for a picker, its page
}

/// Finds every reference of a specification, in document order.
private func collectRefs(_ d: Design) -> [Ref] {
    var out: [Ref] = []
    func add(_ n: YNode?, _ path: [String], _ kind: String, _ owner: YNode, _ page: YNode?) {
        guard let n, n.kind == .scalar, let (dep, ptr) = crossRef(n.value) else { return }
        out.append(Ref(node: n, path: path, text: n.value, dep: dep, pointer: ptr, kind: kind, owner: owner, page: page))
    }
    walk(d.root, []) { n, path in
        if let v = n.child("$ref"), v.kind == .scalar {
            add(v, path + ["$ref"], "ref", n, nil)
        }
        if path.count == 4 && path[0] == "entities" && path[2] == "relations" {
            add(n.child("target"), path + ["target"], "target", n, nil)
        } else if path.count == 2 && path[0] == "requirements" {
            add(n.child("fulfilledBy"), path + ["fulfilledBy"], "fulfilledBy", n, nil)
        } else if path.count == 2 && path[0] == "pages" {
            add(n.child("entity"), path + ["entity"], "entity", n, nil)
            add(n.child("source"), path + ["source"], "source", n, nil)
        } else if path.count == 4 && path[0] == "pages" && path[2] == "pickers" {
            add(n.child("source"), path + ["source"], "picker", n, d.pages[path[1]])
        } else if path.count == 3 && path[0] == "paths" || path.count == 2 && (path[0] == "jobs" || path[0] == "gateways") {
            for (i, e) in items(n.child("calls")).enumerated() {
                add(e, path + ["calls", "\(i)"], "calls", n, nil)
            }
        }
        if path.count == 2 && path[0] == "roles" {
            for (i, e) in items(n.child("permissions")).enumerated() {
                add(e, path + ["permissions", "\(i)"], "permission", n, nil)
            }
            for p in pairs(n.child("through")) {
                add(p.key, path + ["through", p.key.value], "permission", n, nil)
            }
        } else if path.count == 3 && path[0] == "paths" || path.count == 2 && path[0] == "commands" {
            for key in ["callers", "via"] {
                for (i, e) in items(n.child(key)).enumerated() {
                    add(e, path + [key, "\(i)"], "client", n, nil)
                }
            }
        } else if path.count == 2 && path[0] == "tests" {
            let caller = n.child("fixture")?.child("caller")
            if caller?.kind == .scalar {
                add(caller, path + ["fixture", "caller"], "role", n, nil)
            } else {
                add(caller?.child("role"), path + ["fixture", "caller", "role"], "role", n, nil)
            }
        }
    }
    return out
}

/// The specification a reference resolves in, read once.
private final class Other {
    let spec: Spec
    let name: String
    let design: Design
    let version: String
    init(_ spec: Spec, _ name: String, _ root: YNode) {
        self.spec = spec
        self.name = name
        design = Design(root)
        version = str(root.child("info")?.child("version"))
    }
}

private struct ReferenceProblem {
    let node: YNode
    let path: [String]
    let message: String
}

/// Resolves the references of one specification against the
/// specifications given with it.
private final class Resolver {
    let d: Design
    let given: Given
    var others: [String: Other] = [:]

    init(_ d: Design, _ given: Given) {
        self.d = d
        self.given = given
    }

    /// The one specification given under a name, nil when none is, with a
    /// problem when several are.
    func otherOf(_ name: String) -> (Other?, String) {
        let list = given[name] ?? []
        if list.isEmpty { return (nil, "") }
        if list.count > 1 {
            let dirs = list.map { $0.dir }
            return (nil, "the specifications given in \(dirs.joined(separator: " and ")) are all named \(name), so it cannot say which one is meant; give each its own info.name")
        }
        if let o = others[name] { return (o, "") }
        guard let root = list[0].root else { return (nil, "") }
        let o = Other(list[0], name, root)
        others[name] = o
        return (o, "")
    }

    /// Checks one reference: what can be checked in this specification
    /// alone, and, when the other specification is given, the element it
    /// names.
    func resolve(_ r: Ref) -> (problems: [ReferenceProblem], waits: String) {
        var problems: [ReferenceProblem] = []
        func fail(_ n: YNode, _ path: [String], _ message: String) -> ([ReferenceProblem], String) {
            problems.append(ReferenceProblem(node: n, path: path, message: message))
            return (problems, "")
        }
        guard let depNode = d.dependencies[r.dep] else {
            if r.kind == "calls" { return ([], "") } // the dependency check reports it
            return fail(r.node, r.path, "\(r.dep) is not a dependency of the specification, so \(r.text) names nothing; declare it under dependencies with the specification and version it names\(suggest(r.dep, d.dependencies))")
        }
        let name = str(depNode.child("specification"))
        if name.isEmpty {
            return fail(r.node, r.path, "\(r.dep) is a dependency with no specification, so \(r.text) cannot name an element of it; add specification and version to \(r.dep), or name the system alone")
        }
        let otherKey = ["entity": "source", "source": "entity"]
        switch r.kind {
        case "target":
            let k = str(r.owner.child("kind"))
            if k != "many-to-one" && k != "one-to-one" {
                return fail(r.node, r.path, "\(r.path[3]) is a \(k) relation, whose via is in the other module's tables or is a join entity; only a many-to-one or one-to-one relation, whose via is a field this entity keeps, may lead to another specification's entity")
            }
        case "entity", "source":
            let k = str(r.owner.child("kind"))
            if k != "list" {
                return fail(r.node, r.path, "\(r.path[1]) is a \(k) page, and only a list page may show the records of another specification; show them on a list page, or keep the page's \(r.kind) in this specification")
            }
            let other = str(r.owner.child(otherKey[r.kind]!))
            if crossRef(other)?.dep != r.dep {
                return fail(r.node, r.path, "\(r.path[1])'s \(r.kind) is \(r.text), of another specification, and its \(otherKey[r.kind]!) is not of \(r.dep) too; a list of another specification shows that specification's entity, so write both through \(r.dep)")
            }
        case "picker":
            let ent = d.entities[str(r.page?.child("entity"))]
            let (relation, target) = pickerRelation(ent, r.path[3])
            if relation.isEmpty { return ([], "") } // the picker check reports it
            if crossRef(target)?.dep != r.dep {
                return fail(r.node, r.path, "\(r.text) is a list of another specification, and the relation \(relation) leads to \(target); pick a record from a list of the relation's target")
            }
        default:
            break
        }
        let (found, problem) = otherOf(name)
        if !problem.isEmpty { return fail(r.node, r.path, problem) }
        guard let o = found else { return ([], name + " is not given") }
        let od = o.design
        let tokens = referenceTokens(r.pointer)
        let about = "\(o.name) \(o.version)"
        if let want = ["permission": "permissions", "client": "clients", "role": "roles"][r.kind], tokens.count != 2 || tokens[0] != want {
            return fail(r.node, r.path, "\(r.text) names no \(want.dropLast()); write \(r.dep)#/\(want)/<name>")
        }
        if tokens.count == 2 {
            let section = tokens[0], el = tokens[1]
            let sets: [String: [String: YNode]] = ["entities": od.entities, "enums": od.enums, "views": od.views, "schemas": od.schemas, "requirements": od.requirements, "permissions": od.permissions, "clients": od.clientNames, "roles": od.roles]
            let what: [String: String] = ["entities": "an entity", "enums": "an enum", "views": "a view", "schemas": "a schema", "requirements": "a requirement", "permissions": "a permission", "clients": "a client", "roles": "a role"]
            let elements = sets[section] ?? [:]
            if sets[section] == nil || elements[el] == nil {
                return fail(r.node, r.path, "\(el) is not \(what[section] ?? "") of \(about)\(suggest(el, elements))")
            }
        }
        var op: Operation?
        if tokens.count == 3 && tokens[0] == "paths" {
            for x in od.opList where x.path == tokens[1] && x.method == tokens[2] { op = x }
            if op == nil {
                return fail(r.node, r.path, "\(tokens[2].uppercased()) \(tokens[1]) is not an operation of \(about)")
            }
        }
        if r.kind == "source" || r.kind == "picker", let op {
            let listed = str(op.node.child("listOf")?.child("entity"))
            var want = ""
            if r.kind == "source" {
                want = str(r.owner.child("entity"))
            } else {
                want = pickerRelation(d.entities[str(r.page?.child("entity"))], r.path[3]).target
            }
            var wantName = crossRef(want)?.pointer ?? ""
            if wantName.hasPrefix("/entities/") { wantName = String(wantName.dropFirst("/entities/".count)) }
            if listed != wantName {
                return fail(r.node, r.path, "\(op.method.uppercased()) \(op.path) of \(about) does not list \(wantName); name an operation of \(o.name) whose listOf names \(wantName)")
            }
            let fields = fieldsOf(od.entities[wantName])
            if r.kind == "source" {
                for key in ["columns", "filters"] {
                    for (i, f) in items(r.owner.child(key)).enumerated() where fields[f.value] == nil {
                        problems.append(ReferenceProblem(node: f, path: ["pages", r.path[1], key, "\(i)"], message: "\(f.value) is not a field of \(wantName) in \(about)\(suggest(f.value, fields))"))
                    }
                }
            } else {
                let base = ["pages", r.path[1], "pickers", r.path[3]]
                for (i, f) in items(r.owner.child("shows")).enumerated() where fields[f.value] == nil {
                    problems.append(ReferenceProblem(node: f, path: base + ["shows", "\(i)"], message: "\(f.value) is not a field of \(wantName) in \(about), so the picker cannot show it\(suggest(f.value, fields))"))
                }
                for kv in pairs(r.owner.child("fills")) where fields[kv.value.value] == nil {
                    problems.append(ReferenceProblem(node: kv.value, path: base + ["fills", kv.key.value], message: "\(kv.value.value) is not a field of \(wantName) in \(about), so the picker cannot fill from it\(suggest(kv.value.value, fields))"))
                }
            }
            if !problems.isEmpty { return (problems, "") }
        }
        if let root = o.spec.root {
            let q = blockingQuestion(root, pointer(tokens))
            if !q.isEmpty {
                return ([], "\(pointer(tokens)) is blocked in \(o.name) by the must question \(q)")
            }
        }
        return ([], "")
    }
}

/// Splits the part of a reference after the # into its unescaped tokens.
private func referenceTokens(_ ptr: String) -> [String] {
    let p = ptr.hasPrefix("/") ? String(ptr.dropFirst()) : ptr
    if p.isEmpty { return [] }
    return p.split(separator: "/", omittingEmptySubsequences: false).map { unescapeToken(String($0)) }
}

/// The first open must question, by ID, whose blocks name the element at
/// the pointer or one it is inside, or "".
private func blockingQuestion(_ root: YNode, _ ptr: String) -> String {
    var ids: [String] = []
    for p in pairs(root.child("questions")) where str(p.value.child("priority")) == "must" {
        for item in items(p.value.child("blocks")) {
            guard let bl = parseBlock(item.value), bl.isPointer else { continue }
            let e = pointer(bl.tokens)
            if ptr == e || ptr.hasPrefix(e + "/") {
                ids.append(p.key.value)
                break
            }
        }
    }
    return ids.sorted(by: byteLess).first ?? ""
}

extension Checker {
    /// Reports every reference that is wrong, and every dependency whose
    /// version is not the one the specification it names is at, when that
    /// specification is given.
    func checkReferences(_ d: Design) {
        let rv = Resolver(d, given)
        for r in collectRefs(d) {
            for p in rv.resolve(r).problems {
                add(p.node, pointer(p.path), .reference, p.message)
            }
        }
        // A forward by aud into a module given in the run names one of the
        // audiences the module accepts (ADR-141).
        for f in audienceForwards(d) {
            guard let o = rv.otherOf(f.spec).0 else { continue } // not given: an unresolved reference
            let auds = o.design.acceptedAudiences
            if auds.isEmpty {
                add(f.value, pointer(f.path), .gateway, "gateway \(f.gateway) forwards \(f.text) to \(f.dep), the module \(f.spec), which accepts no access token, so nothing there checks the token's audience; have \(f.spec) accept a token whose aud is \(f.text)")
            } else if !auds.contains(f.text) {
                add(f.value, pointer(f.path), .gateway, "gateway \(f.gateway) forwards \(f.text) to \(f.dep), the module \(f.spec), which accepts no access token for that audience, so the token is refused there; it accepts \(auds.joined(separator: ", ")); write its audience here, or have \(f.spec) declare this one")
            }
        }
        for name in d.dependencies.keys.sorted(by: byteLess) {
            let dep = d.dependencies[name]!
            let spName = str(dep.child("specification"))
            guard !spName.isEmpty, let vn = dep.child("version") else { continue }
            if let o = rv.otherOf(spName).0, o.version != vn.value {
                add(vn, pointer("dependencies", name, "version"), .reference, "\(spName) is at version \(o.version), and \(name) names \(vn.value); check the references to it against \(o.version), then write that version here")
            }
        }
    }
}

/// Lists the references of a specification that are not resolved: the
/// specification they name is not given, or the element is blocked there
/// by a must question. A wrong reference is an error and is not listed.
func unresolvedReferences(_ s: Spec, _ given: Given) -> [Reference] {
    guard let root = s.root else { return [] }
    let d = Design(root)
    let rv = Resolver(d, given)
    var out: [Reference] = []
    let identity = d.identityDependency
    for p in rv.ungrantedWaiting() {
        out.append(Reference(path: pointer("permissions", p.key.value), text: identity + "#/roles", waits: rv.identityGrant(p.key.value).waits))
    }
    for r in collectRefs(d) {
        let res = rv.resolve(r)
        if !res.problems.isEmpty || res.waits.isEmpty { continue }
        out.append(Reference(path: pointer(r.path), text: r.text, waits: res.waits))
    }
    for f in audienceForwards(d) where rv.otherOf(f.spec).0 == nil {
        out.append(Reference(path: pointer(f.path), text: f.text, waits: f.spec + " is not given"))
    }
    return out
}

/// One row of a gateway forwarding by aud whose dependency names another
/// specification.
private struct GatewayForward {
    let value: YNode
    let path: [String]
    let dep: String, spec: String, gateway: String, text: String
}

/// The rows of the gateways forwarding by aud whose dependency names a
/// specification, in document order.
private func audienceForwards(_ d: Design) -> [GatewayForward] {
    var out: [GatewayForward] = []
    for g in d.gatewayList where str(g.value.child("claim")) == "aud" {
        for (i, row) in items(g.value.child("forwards")).enumerated() {
            let dep = str(row.child("dependency"))
            let sp = d.specificationOf(dep)
            if !sp.isEmpty, let v = row.child("value") {
                out.append(GatewayForward(value: v, path: ["gateways", g.key.value, "forwards", "\(i)", "value"], dep: dep, spec: sp, gateway: g.key.value, text: v.value))
            }
        }
    }
    return out
}

// The identity module holds the estate's clients, roles and trust domain,
// and every other module names them by reference through a dependency of
// kind identityProvider (see the Go build's identity.go, ADR-107).

extension Design {
    /// The dependency that names the identity module, of kind
    /// identityProvider and with a specification, the first by name; "".
    var identityDependency: String {
        for name in dependencies.keys.sorted(by: byteLess) {
            let dep = dependencies[name]!
            if str(dep.child("kind")) == "identityProvider" && !str(dep.child("specification")).isEmpty { return name }
        }
        return ""
    }

    /// The specification a dependency names, or "".
    func specificationOf(_ dep: String) -> String { str(dependencies[dep]?.child("specification")) }

    /// Gives the design the identity module given with it, so that its
    /// derived cases read the through of the identity module's roles and
    /// the security events and trust domain it holds (ADR-171).
    @discardableResult
    func withIdentity(_ given: Given) -> Design {
        let dep = identityDependency
        if dep.isEmpty || given.isEmpty { return self }
        let (found, problem) = Resolver(self, given).otherOf(specificationOf(dep))
        if let o = found, problem.isEmpty, o.name != specName(root) {
            identity = (o.name, o.design)
        }
        return self
    }

    /// Whether a case a test covers is one the identity module derives
    /// while that module is not given: the through case of one of its
    /// roles, and the cases of one of its clients named in callers, which a
    /// run with the module checks, as a reference waits for the
    /// specification it names (ADR-107, ADR-171).
    func identityCaseWaits(_ name: String) -> Bool {
        let dep = identityDependency
        if dep.isEmpty || identity != nil { return false }
        if name.hasPrefix("refused through a client not listed for ") && name.hasSuffix(" of " + specificationOf(dep)) {
            return true
        }
        // The cases of a client of the identity module named in callers: its
        // signature, its version, and the deputy's cases once it is a service
        // (BUG-046).
        let signature = "refused with a bad signature from "
        if name.hasPrefix(signature) {
            return name.dropFirst(signature.count).hasPrefix(dep + "#")
        }
        let version = " at a version outside its admitted range"
        if name.hasPrefix("refused with ") && name.hasSuffix(version) {
            return name.dropFirst("refused with ".count).hasPrefix(dep + "#")
        }
        return serviceCases.contains(name) && !hasService && namesIdentityCaller
    }

    /// The client a reference names in the identity module given with the
    /// design, <dependency>#/clients/<name>; nil when the reference names
    /// another dependency or the module is not given (ADR-107, ADR-171).
    func identityClient(_ text: String) -> YNode? {
        guard let (dep, ptr) = crossRef(text), let identity, dep == identityDependency else { return nil }
        let tokens = pointerTokens(ptr)
        guard tokens.count == 2, tokens[0] == "clients" else { return nil }
        return identity.design.client(tokens[1])
    }

    /// The client an entry of callers names: one of the design's own, or
    /// one of the identity module's by reference.
    func callerClient(_ name: String) -> YNode? {
        isCross(name) ? identityClient(name) : client(name)
    }

    /// Whether an operation names a client of the identity module in
    /// callers.
    var namesIdentityCaller: Bool {
        let dep = identityDependency
        return opList.contains { items($0.node.child("callers")).contains { crossRef($0.value)?.dep == dep } }
    }

    /// The trust domain of the design, or of the identity module given with
    /// it; "" when neither says one.
    var trustDomain: String {
        let t = str(root.child("trustDomain"))
        guard t.isEmpty, let identity else { return t }
        return str(identity.design.root.child("trustDomain"))
    }

    /// Every permission a role of the specification grants, directly or by
    /// implies.
    var grantedHere: Set<String> {
        var out = Set<String>()
        for r in roles.values { out.formUnion(roleHolds(r)) }
        return out
    }
}

extension Other {
    /// Whether one of the roles grants the permission perm of the module
    /// named module, by a reference through one of its dependencies.
    func grants(_ roles: [String], _ module: String, _ perm: String) -> Bool {
        for r in roles {
            for p in design.roleHolds(design.roles[r]) {
                if let (dep, ptr) = crossRef(p), design.specificationOf(dep) == module, ptr == "/permissions/" + perm { return true }
            }
        }
        return false
    }
}

extension Resolver {
    /// The element a reference names in a specification given with this
    /// one, with that specification.
    func element(_ text: String) -> (Other, YNode)? {
        guard let (dep, ptr) = crossRef(text) else { return nil }
        let name = d.specificationOf(dep)
        if name.isEmpty { return nil }
        let (found, problem) = otherOf(name)
        guard let o = found, problem.isEmpty else { return nil }
        let tokens = referenceTokens(ptr)
        guard tokens.count == 2, let el = o.design.root.child(tokens[0])?.child(tokens[1]) else { return nil }
        return (o, el)
    }

    /// Whether a permission no role here grants is granted by a role of the
    /// identity module, or why it cannot be told.
    func identityGrant(_ perm: String) -> (granted: Bool, waits: String, identity: String) {
        let dep = d.identityDependency
        if dep.isEmpty { return (false, "", "") }
        let name = d.specificationOf(dep)
        let (found, problem) = otherOf(name)
        if !problem.isEmpty { return (false, "", name) }
        guard let o = found else { return (false, name + " is not given, so the role that grants " + perm + " cannot be found", name) }
        return (o.grants(o.design.roles.keys.sorted(by: byteLess), specName(d.root), perm), "", name)
    }

    /// The permissions no role here grants whose grant waits for the
    /// identity module.
    func ungrantedWaiting() -> [Pair] {
        let granted = d.grantedHere
        return pairs(d.root.child("permissions")).filter { p in
            p.key.value != permissionPublic && p.key.value != permissionNobody && !granted.contains(p.key.value) && !identityGrant(p.key.value).waits.isEmpty
        }
    }
}

/// A client of the identity module named in via or callers, once that
/// module is given and holds it.
struct ReferencedClient {
    let module: String
    let node: YNode
    let grants: (_ roles: [String], _ perm: String) -> Bool
}

extension Checker {
    func referencedClient(_ d: Design, _ text: String) -> ReferencedClient? {
        guard let (_, ptr) = crossRef(text), ptr.hasPrefix("/clients/") else { return nil }
        let rv = Resolver(d, given)
        guard let (o, cl) = rv.element(text) else { return nil }
        let module = specName(d.root)
        return ReferencedClient(module: o.name, node: cl) { roles, perm in o.grants(roles, module, perm) }
    }

    func identityGrant(_ d: Design, _ perm: String) -> (granted: Bool, waits: String, identity: String) {
        Resolver(d, given).identityGrant(perm)
    }

    /// Whether a specification given with this one names the client in an
    /// operation's via, through a dependency on this specification.
    func namedInViaElsewhere(_ d: Design, _ client: String) -> Bool {
        let module = specName(d.root)
        if module.isEmpty { return false }
        for list in given.values {
            for s in list {
                guard let root = s.root, specName(root) != module else { continue }
                let od = Design(root)
                for o in od.opList {
                    for v in items(o.node.child("via")) {
                        if let (dep, ptr) = crossRef(v.value), od.specificationOf(dep) == module, ptr == "/clients/" + client { return true }
                    }
                }
            }
        }
        return false
    }

    /// Checks that the trust domain is a URI.
    func checkTrustDomain(_ d: Design) {
        guard let n = d.root.child("trustDomain") else { return }
        let s = str(n)
        if let u = URLComponents(string: s), let scheme = u.scheme, !scheme.isEmpty, let host = u.host, !host.isEmpty { return }
        add(n, "/trustDomain", .tokenAudience, "trustDomain \(s) is not a URI; it is the aud of every transaction token and the base of every security event's identifier, such as https://estate.example.org")
    }
}

// The chain of a call is found by induction across the modules given
// together (see the Go build's identity.go, ADR-111).

/// The name of the client a via entry names.
func clientKey(_ entry: String) -> String {
    guard let (_, ptr) = crossRef(entry) else { return entry }
    return ptr.hasPrefix("/clients/") ? String(ptr.dropFirst("/clients/".count)) : ptr
}

/// The operations of a design that call operation o of the module named
/// module, through a dependency on it, in document order.
func callersOf(_ od: Design, _ module: String, _ o: Operation) -> [Operation] {
    var out: [Operation] = []
    for x in od.opList {
        for e in items(x.node.child("calls")) {
            let cross = crossRef(e.value)
            let dep = cross?.dep ?? e.value
            if od.specificationOf(dep) != module { continue }
            if let cross {
                let tokens = referenceTokens(cross.pointer)
                if tokens.count != 3 || tokens[0] != "paths" || tokens[1] != o.path || tokens[2] != o.method { continue }
            }
            out.append(x)
            break
        }
    }
    return out
}

extension Design {
    /// The aud values of the access tokens an operation accepts, sorted.
    var acceptedAudiences: [String] {
        var seen = Set<String>()
        for name in acceptedTokens {
            guard let tok = token(name), tokenType(tok) == "access_token" else { continue }
            let aud = tok.child("claims")?.child("aud")
            if let v = aud?.child("const"), v.kind == .scalar { seen.insert(v.value) }
            for v in items(aud?.child("enum")) { seen.insert(v.value) }
        }
        return seen.sorted(by: byteLess)
    }
}

extension Checker {
    /// The one specification given under a name, read, or nil.
    func givenOne(_ name: String) -> Design? {
        guard let list = given[name], list.count == 1, let root = list[0].root, root.kind == .mapping else { return nil }
        return Design(root)
    }

    /// Checks an operation's via against the modules given with this one.
    func checkChain(_ d: Design, _ o: Operation) {
        let module = specName(d.root)
        guard !module.isEmpty, let via = o.node.child("via") else { return }
        var named = Set<String>()
        for (i, item) in items(via).enumerated() {
            let key = clientKey(item.value)
            named.insert(key)
            guard let cd = givenOne(key), key != module else { continue }
            let callers = callersOf(cd, module, o)
            if callers.isEmpty {
                add(item, o.pointer("via", "\(i)"), .via, "\(o.id) names \(item.value), and no operation of the module \(key) calls \(o.method.uppercased()) \(o.path), so the chain has no hop before this one; call it from \(key), or take \(item.value) out of via")
                continue
            }
            for x in callers where cd.token(cd.acceptedToken(x.node)) == nil {
                add(item, o.pointer("via", "\(i)"), .via, "operation \(x.id) of \(key) calls \(o.id) and accepts no token, so it has no person's call to carry on; accept the transaction token, with a via of its own, or an access token, as the edge")
            }
        }
        for name in given.keys.sorted(by: byteLess) where name != module && !named.contains(name) {
            guard let od = givenOne(name) else { continue }
            let callers = callersOf(od, module, o)
            if let x = callers.first {
                add(via, o.pointer("via"), .via, "operation \(x.id) of \(name) calls \(o.id), and via does not name \(name), so the hop that calls it is not in the chain; add \(viaEntry(d, o, name)) to via, or stop the call")
            }
        }
    }
}

/// How via would name a client: through the dependency its other entries
/// go through, or by name.
func viaEntry(_ d: Design, _ o: Operation, _ client: String) -> String {
    for item in items(o.node.child("via")) {
        if let (dep, _) = crossRef(item.value) { return dep + "#/clients/" + client }
    }
    let dep = d.identityDependency
    return dep.isEmpty ? client : dep + "#/clients/" + client
}

/// The confused deputy's cases, derived once a service exists (ADR-108).
let serviceCases: Set<String> = [
    "refused for a service's own token",
    "refused with a req_wl that is not an edge",
    "refused with a txn differing from the one received",
    "refused with a scope no edge of its chain checked",
    "refused for a person's token requested by a service with no actor",
]
