import Foundation

// Machine clients and the gates that are not a permission (meta-model
// 0.3, ADR-096, ADR-106, ADR-110): a client is a program that calls the
// system, an application where a person signs in or a service that calls
// on its own account with the roles it holds and a proof, or carries a
// person's call. An operation's or command's callers is the explicit
// subset of who may call it, people and clients by name; nobody is the
// reserved permission of a route closed on purpose; a role's through
// narrows a grant to the clients it holds through.

/// The reserved words of callers and permissions.
let callerPeople = "people", permissionNobody = "nobody", permissionPublic = "public"

/// The signature algorithms RFC 9421 registers.
let signatureAlgorithms = ["ecdsa-p256-sha256", "ecdsa-p384-sha384", "ed25519", "hmac-sha256", "rsa-pss-sha512", "rsa-v1_5-sha256"]

/// The derived components of RFC 9421, section 2.2.
let derivedComponents: Set<String> = ["@method", "@target-uri", "@authority", "@scheme", "@request-target", "@path", "@query", "@query-param", "@status"]

private let headerFieldName = try! NSRegularExpression(pattern: "^[a-z0-9!#$%&'*+.^_`|~-]+$")
private let versionRange = try! NSRegularExpression(pattern: "^>=([0-9]+\\.[0-9]+\\.[0-9]+(?:-[0-9A-Za-z.-]+)?) <([0-9]+\\.[0-9]+\\.[0-9]+(?:-[0-9A-Za-z.-]+)?)$")

extension Design {
    /// The specification's clients, in document order.
    var clientList: [Pair] { pairs(root.child("clients")) }

    func client(_ name: String) -> YNode? { root.child("clients")?.child(name) }

    /// The clients' names, for a suggestion.
    var clientNames: [String: YNode] {
        var out: [String: YNode] = [:]
        for c in clientList { out[c.key.value] = c.value }
        return out
    }

    /// Every permission a client holds through its roles.
    func clientHolds(_ client: YNode) -> Set<String> {
        var out = Set<String>()
        for r in items(client.child("roles")) { out.formUnion(roleHolds(roles[r.value])) }
        return out
    }

    /// Whether any client is a service.
    var hasService: Bool { clientList.contains { str($0.value.child("kind")) == "service" } }

    /// Whether any declared token carries the claim.
    func tokensDeclareClaim(_ claim: String) -> Bool {
        pairs(root.child("tokens")).contains { $0.value.child("claims")?.child(claim) != nil }
    }

    /// Whether an operation names the client as an immediate caller.
    func namedInVia(_ name: String) -> Bool {
        opList.contains { items($0.node.child("via")).contains { $0.value == name } }
    }

    /// Who an operation or command admits: people, unless callers lists
    /// clients only.
    func admitsPeople(_ n: YNode) -> Bool {
        let callers = items(n.child("callers"))
        return callers.isEmpty || callers.contains { $0.value == callerPeople }
    }

    /// The cases of who may call an operation or a command (see the Go
    /// build's callerCases).
    func callerCases(_ s: Subject, _ n: YNode, _ perm: String, _ what: String, atEdge: Bool) {
        if perm == permissionNobody {
            s.red("refused for every caller", frequent, "any caller, signed in or not, with any roles", what, "it is refused: the route is closed on purpose, with the problem the design declares" + event("callers"))
            return
        }
        let callers = items(n.child("callers"))
        let people = admitsPeople(n)
        if !callers.isEmpty {
            if !people {
                s.red("refused for a signed-in person", frequent, "a person signed in, whatever roles they hold", what, "it is refused: only the clients listed may call it" + event("callers"))
            }
            s.red("refused for every caller not listed", occasional, "a client not listed in callers, whatever its roles grant", what, "it is refused as not allowed" + event("callers"))
            for item in callers {
                guard let cl = client(item.value) else { continue }
                if let proof = cl.child("proof"), str(proof.child("kind")) == "signedRequest" {
                    s.red("refused with a bad signature from " + item.value, rare, item.value + " calls with a signature that does not verify over " + joinAnd(items(proof.child("covers")).map { $0.value }), what, "it is refused as not signed, and nothing changes" + event("callers"))
                }
                if atEdge { versionCase(s, item.value, cl, what) }
            }
        }
        if !people { return }
        throughCases(s, perm, what)
        if !atEdge { return }
        for cl in clientList where str(cl.value.child("kind")) == "application" {
            versionCase(s, cl.key.value, cl.value, what)
        }
        if hasService {
            s.red("refused for a service's own token", rare, "a service calling with its own proof or token and no person's token, so that the call carries the service and not the person", what, "it is refused as not allowed: the person, with their rows, roles and through, would be lost" + event("deputy"))
        }
    }

    /// For every role whose through narrows the permission, the case of a
    /// caller through a client it does not list.
    func throughCases(_ s: Subject, _ perm: String, _ what: String) {
        for r in pairs(root.child("roles")) {
            guard let list = r.value.child("through")?.child(perm) else { continue }
            s.red("refused through a client not listed for " + r.key.value, occasional, "a caller holding " + r.key.value + " who signed in through a client other than " + joinAnd(items(list).map { $0.value }), what, "it is refused as not allowed" + event("through"))
        }
    }

    /// The case of a client with an admitted version range calling at a
    /// version outside it.
    func versionCase(_ s: Subject, _ name: String, _ cl: YNode, _ what: String) {
        let v = str(cl.child("softwareVersion"))
        if !v.isEmpty {
            s.red("refused with " + name + " at a version outside its admitted range", occasional, "a caller through " + name + " whose token carries a softwareVersion outside " + v, what, "it is refused with 403 and the problem the design declares" + versionEvent(cl))
        }
    }
}

extension Checker {
    /// Checks every client (see the Go build's checkClients).
    func checkClients(_ d: Design) {
        for cl in d.clientList {
            let name = cl.key.value
            let at = pointer("clients", name)
            switch name {
            case callerPeople:
                add(cl.key, at, .reservedName, "people is reserved for anyone signed in, in callers; give the client another name")
            case permissionNobody, permissionPublic:
                add(cl.key, at, .reservedName, "\(name) is a reserved permission; give the client another name")
            default:
                break
            }
            let kind = str(cl.value.child("kind"))
            if !kind.isEmpty && kind != "application" && kind != "service" {
                add(cl.value.child("kind"), at + "/kind", .client, "\(kind) is neither application nor service: an application is where a person signs in, a service a program that calls on its own account or carries a person's call")
            }
            for (i, r) in items(cl.value.child("roles")).enumerated() where d.roles[r.value] == nil {
                add(r, pointer("clients", name, "roles", "\(i)"), .client, "\(r.value) is not a role of the specification\(suggest(r.value, d.roles))")
            }
            let proof = cl.value.child("proof")
            let roles = cl.value.child("roles")
            if roles != nil && proof == nil {
                add(cl.key, at, .client, "\(name) holds roles and has no proof, and a client that calls on its own account must prove itself on each request (RFC 6749, section 4.4); add proof, or take the roles away")
            } else if kind == "service" && proof == nil {
                add(cl.key, at, .client, "\(name) is a service with no proof; a service proves itself on each request with a signed request or its client certificate; add proof")
            }
            if let proof {
                checkProof(name, proof, at + "/proof")
                checkTokenProof(d, name, proof, at + "/proof")
            }
            if let v = cl.value.child("softwareVersion") {
                if !matches(versionRange, str(v)) {
                    add(v, at + "/softwareVersion", .client, "softwareVersion \(str(v)) is not a range written >=<lower> <<upper>, each a Semantic Versioning 2.0.0 version, such as >=1.2.0 <2.0.0")
                } else if !d.tokensDeclareClaim("softwareVersion") {
                    add(v, at + "/softwareVersion", .softwareVersion, "softwareVersion on \(name) has nothing to be checked against: no token declares a softwareVersion claim, and the version is read at the edge from the claim the issuer fills from the registration; declare the claim on the token the edge accepts, or take softwareVersion away")
                }
            }
            if kind == "service" && roles == nil && !d.namedInVia(name) {
                warn(cl.key, at, .clientUnused, "\(name) is a service with no roles that no operation names in via, so nothing reaches it; give it roles, name it in via, or remove it")
            }
        }
    }

    /// Checks a signed request's algorithm and covered components, and that
    /// a client certificate names neither.
    func checkProof(_ client: String, _ proof: YNode, _ at: String) {
        let kind = str(proof.child("kind"))
        let alg = proof.child("algorithm")
        let covers = proof.child("covers")
        switch kind {
        case "signedRequest":
            if let alg {
                if !signatureAlgorithms.contains(str(alg)) {
                    add(alg, at + "/algorithm", .proof, "\(str(alg)) is not a signature algorithm RFC 9421 registers; use one of \(signatureAlgorithms.joined(separator: ", "))")
                }
            } else {
                add(proof, at, .proof, "a signedRequest proof names the algorithm both sides agree on; add algorithm, one of \(signatureAlgorithms.joined(separator: ", "))")
            }
            if items(covers).isEmpty {
                add(proof, at, .proof, "a signed request covers at least one component of the request; list them under covers, such as [\"@method\", \"@path\", content-digest, date]")
            }
            for (i, item) in items(covers).enumerated() {
                let v = item.value
                if !derivedComponents.contains(v) && !matches(headerFieldName, v) {
                    add(item, pointer("clients", client, "proof", "covers", "\(i)"), .proof, "\(v) is not a component of a request that a signature covers: a derived component such as @method or @path, or a header field by its name in lower case")
                }
            }
        case "mutualTls":
            if alg != nil || covers != nil {
                add(proof, at, .proof, "a mutualTls proof is the client certificate of RFC 8705 and names no algorithm or covered components; remove them")
            }
        default:
            break
        }
    }

    /// Checks every role's through (see the Go build's checkThrough).
    func checkThrough(_ d: Design) {
        let carries = d.tokensDeclareClaim("client_id") || d.tokensDeclareClaim("tctx")
        for r in pairs(d.root.child("roles")) {
            guard let through = r.value.child("through") else { continue }
            let at = pointer("roles", r.key.value, "through")
            if !carries {
                add(through, at, .through, "through needs the application the caller signed in through, which a session does not carry and no token declares: declare a token with the client_id claim, or a transaction token with tctx, or take through away")
            }
            let holds = d.roleHolds(r.value)
            for p in pairs(through) {
                let perm = p.key.value
                let pat = at + "/" + String(pointer(perm).dropFirst())
                if !holds.contains(perm) {
                    add(p.key, pat, .through, "\(perm) is not a permission \(r.key.value) grants, directly or by implies, so through has no grant to narrow; add it to the role's permissions, or remove it here")
                }
                let list = items(p.value)
                if list.isEmpty {
                    add(p.value, pat, .through, "through names no client for \(perm), which would close the grant; list the clients it holds through, or drop the permission from the role")
                }
                for (i, item) in list.enumerated() where d.client(item.value) == nil {
                    add(item, pat + "/\(i)", .through, "\(item.value) is not a client of the specification\(suggest(item.value, d.clientNames))")
                }
            }
        }
    }

    /// Checks the callers of every operation and command (see the Go
    /// build's checkCallers).
    func checkCallers(_ d: Design) {
        for o in d.opList { checkCallerList(d, o.node, o.pointer("callers"), "operation " + o.id) }
        for cmd in pairs(d.root.child("commands")) {
            checkCallerList(d, cmd.value, pointer("commands", cmd.key.value, "callers"), "command " + cmd.key.value)
        }
    }

    func checkCallerList(_ d: Design, _ n: YNode, _ at: String, _ who: String) {
        guard let callers = n.child("callers") else { return }
        let perm = str(n.child("permission"))
        let list = items(callers)
        if list.isEmpty {
            add(callers, at, .callers, "callers names nobody, which would make \(who) a route nobody calls; list people or the clients, or close it with permission: nobody")
            return
        }
        if perm == permissionNobody {
            add(callers, at, .callers, "\(who) is closed with permission nobody, so callers has nobody to list; remove callers")
        }
        var seen = Set<String>()
        for (i, item) in list.enumerated() {
            let name = item.value
            let iat = at + "/\(i)"
            if seen.contains(name) {
                add(item, iat, .callers, "\(name) is listed twice; list it once")
                continue
            }
            seen.insert(name)
            if name == callerPeople { continue }
            guard let cl = d.client(name) else {
                add(item, iat, .callers, "\(name) is neither people nor a client of the specification\(suggest(name, d.clientNames))")
                continue
            }
            if str(cl.child("kind")) == "service" && cl.child("roles") == nil {
                add(item, iat, .callers, "\(name) is a service with no roles, which only carries a person's call and never calls on its own account; list people, whose calls it carries, or give it roles")
                continue
            }
            if !perm.isEmpty && perm != permissionPublic && perm != permissionNobody && !d.clientHolds(cl).contains(perm) {
                add(item, iat, .callers, "\(name) holds no role that grants \(perm), so it could never call \(who); add the permission to one of its roles, or take the client out of callers")
            }
        }
    }
}
