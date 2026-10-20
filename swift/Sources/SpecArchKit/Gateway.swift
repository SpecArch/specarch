import Foundation

// Gateways (ADR-141): an intermediary, after RFC 9110's gateway, that
// verifies a bearer token, reads one claim and forwards every request no
// operation answers, as received, to the dependency the claim's value
// names. It names no permission; when the claim is aud, its forwards are
// the token's fixed audience.

/// The dependency kinds a gateway cannot forward to: a store or a broker is
/// no HTTP origin (ADR-134).
let notForwardable: Set<String> = ["database", "messageBroker", "fileStore"]

/// The dependencies a gateway forwards to, in order, each once.
func forwardedDependencies(_ g: YNode?) -> [String] {
    var out: [String] = []
    for row in items(g?.child("forwards")) {
        let dep = str(row.child("dependency"))
        if !dep.isEmpty && !out.contains(dep) { out.append(dep) }
    }
    return out
}

extension Design {
    var gatewayList: [Pair] { pairs(root.child("gateways")) }

    /// Each token a gateway forwards by aud, with the first such gateway:
    /// the forwards fix its audience, so it is the services' token and not
    /// this system's.
    var forwardedByAudience: [String: String] {
        var out: [String: String] = [:]
        for g in gatewayList {
            let tok = str(g.value.child("token"))
            if tok.isEmpty || str(g.value.child("claim")) != "aud" || token(tok) == nil { continue }
            if out[tok] == nil { out[tok] = g.key.value }
        }
        return out
    }

    /// A gateway's tests: a request reaching each forwarded service, the
    /// token refused when missing, badly signed or expired, a claim naming
    /// no service or two, and each dependency failing or not answering in
    /// time.
    func gatewaySubject(_ p: Pair) -> Subject {
        let name = p.key.value
        let s = Subject(label: "gateway " + name, node: p.key, path: pointer("gateways", name), yamlKey: "gateway: " + name, name: kebab(name))
        let tokName = str(p.value.child("token"))
        let tok = token(tokName)
        let claim = str(p.value.child("claim"))
        let aToken = tokName.isEmpty ? "a token" : "a " + tokName + " token"
        let when = "a request no operation answers reaches the gateway " + name
        s.success = golden(aToken + " whose " + claim + " is a value of the gateway's forwards", when, "it reaches the service the value names as received")
        for row in items(p.value.child("forwards")) {
            let dep = str(row.child("dependency"))
            s.cases.append(DerivedCase(name: "forwarded to " + dep, scenario: "golden", given: aToken + " whose " + claim + " is " + str(row.child("value")),
                                       when: when, then: "it reaches " + dep + " as received", frequency: frequent))
        }
        s.red("refused with no token", frequent, "a request with no bearer token", when, "it is refused as not signed in and nothing is forwarded")
        s.red("refused with a bad signature", frequent, aToken + " not signed by the issuer's key", when, "it is refused as not signed in and nothing is forwarded")
        if let tok {
            s.red("denied with expired " + tokName, frequent, aToken + " that has expired after " + str(tok.child("lifetime")), when, "it is refused as not signed in and nothing is forwarded")
        }
        s.red("refused with an unknown " + claim, occasional, aToken + " whose " + claim + " is no value of the gateway's forwards", when, "it is refused and nothing is forwarded")
        if claim == "aud" || typeIs(tok?.child("claims")?.child(claim), "array") {
            s.red("refused with a " + claim + " naming two forwarded services", rare, aToken + " whose " + claim + " holds two values of the gateway's forwards", when, "it is refused and nothing is forwarded")
        }
        var deps = forwardedDependencies(p.value)
        for dep in calledDependencies(p.value) where !deps.contains(dep) { deps.append(dep) }
        for dep in deps {
            guard let n = dependencies[dep] else { continue }
            s.byNature("dependency fails " + dep, dep + " answers with an error", when, "...")
            s.byNature("dependency times out " + dep, dep + " does not answer within " + str(n.child("timeout")), when, "...")
        }
        return s
    }
}

extension Checker {
    /// Checks each gateway's token, claim, forwards, calls, middlewares and
    /// refusals, and that a token forwarded by aud is neither the default
    /// nor named by an operation (see the Go build's checkGateways).
    func checkGateways(_ d: Design) {
        var declared: [String: YNode] = [:]
        for p in pairs(d.root.child("middlewares")) { declared[p.key.value] = p.value }
        for g in d.gatewayList {
            let name = g.key.value
            let at = pointer("gateways", name)
            let tokNode = g.value.child("token")
            let tokName = str(tokNode)
            let tok = d.token(tokName)
            if tokNode != nil && tok == nil {
                add(tokNode, at + "/token", .gateway, "\(tokName) is not a token of the specification\(suggest(tokName, d.tokenNames))")
            }
            let claimNode = g.value.child("claim")
            let claim = str(claimNode)
            if claimNode != nil, let tok, tok.child("claims")?.child(claim) == nil {
                let names = pairs(tok.child("claims")).map { $0.key.value }.sorted(by: byteLess).joined(separator: ", ")
                add(claimNode, at + "/claim", .gateway, "\(tokName) does not declare the claim \(claim), so the gateway has no value to read; declare it under the token's claims, or read one it declares: \(names.isEmpty ? "nothing" : names)")
            }
            let forwards = g.value.child("forwards")
            if let forwards, items(forwards).isEmpty {
                add(forwards, at + "/forwards", .gateway, "gateway \(name) forwards to no service, so every request it takes is refused; add a row with the claim's value and the dependency it selects")
            }
            var seen: Set<String> = []
            var forwarded: Set<String> = []
            for (i, row) in items(forwards).enumerated() {
                let rat = at + "/forwards/\(i)"
                if let v = row.child("value") {
                    if seen.contains(v.value) {
                        add(v, rat + "/value", .gateway, "\(v.value) is written twice, and a request goes to one service; write each value once")
                    }
                    seen.insert(v.value)
                }
                guard let depNode = row.child("dependency") else { continue }
                forwarded.insert(depNode.value)
                guard let dep = d.dependencies[depNode.value] else {
                    add(depNode, rat + "/dependency", .gateway, "\(depNode.value) is not a dependency of the specification; declare it under dependencies with its timeout, kind and protocol\(suggest(depNode.value, d.dependencies))")
                    continue
                }
                let kind = str(dep.child("kind"))
                if notForwardable.contains(kind) {
                    add(depNode, rat + "/dependency", .gateway, "\(depNode.value) is a dependency of kind \(kind), which is no HTTP origin a request can be forwarded to; forward to a dependency of kind system or identityProvider")
                }
            }
            for (i, e) in items(g.value.child("calls")).enumerated() {
                let dep = callDependency(e.value)
                if d.dependencies[dep] == nil {
                    add(e, at + "/calls/\(i)", .dependency, "\(dep) is not a dependency of the specification; declare it under dependencies with its timeout\(suggest(dep, d.dependencies))")
                } else if forwarded.contains(dep) {
                    add(e, at + "/calls/\(i)", .gateway, "\(dep) is forwarded to, and a forward is a call already; remove it from calls")
                }
            }
            if let list = g.value.child("middlewares") {
                for (i, item) in items(list).enumerated() where declared[item.value] == nil {
                    add(item, at + "/middlewares/\(i)", .middleware, "\(item.value) is not a middleware of the specification\(suggest(item.value, declared))")
                }
            } else if !declared.isEmpty {
                warn(g.key, at, .middlewareMissing, "the specification declares its middlewares, and gateway \(name) names none; list the ones it runs, in order, under middlewares, or write middlewares: [] if it runs none")
            }
            checkRefusals(d, g.value.child("refusals"), at + "/refusals", "gateway " + name)
            guard claim == "aud", let tok else { continue }
            if str(tok.child("default")) == "true" {
                add(tokNode, at + "/token", .gateway, "\(tokName) is forwarded by aud, so it is the services' token, and it is marked default, which makes every operation of this specification accept it; remove default from \(tokName)")
            }
            for o in d.opList {
                if let n = o.node.child("token"), str(n) == tokName {
                    add(n, o.pointer("token"), .gateway, "\(tokName) is forwarded by aud by gateway \(name), so it is the services' token, and operation \(o.id) accepts it; name a token of this system's own")
                }
            }
        }
    }

    /// Checks a list of refusals against the catalogue of problem types, as
    /// a middleware's are (ADR-131).
    func checkRefusals(_ d: Design, _ refusals: YNode?, _ at: String, _ what: String) {
        let errors = d.root.child("errors")
        for (i, r) in items(refusals).enumerated() {
            let status = str(r.child("status"))
            let rat = at + "/\(i)"
            guard let p = r.child("problem") else {
                if !pairs(errors).isEmpty {
                    add(r, rat, .problem, "the specification declares its problem types under errors, so the \(status) refusal of \(what) names one under problem")
                }
                continue
            }
            guard let t = errors?.child(p.value) else {
                var valid: [String: YNode] = [:]
                for e in pairs(errors) { valid[e.key.value] = e.value }
                add(p, rat + "/problem", .problem, "\(p.value) is not a problem type under errors\(suggest(p.value, valid))")
                continue
            }
            let s = str(t.child("status"))
            if s != status {
                add(p, rat + "/problem", .problem, "\(p.value) is a \(s) problem, and this is a \(status) refusal; name a problem type of status \(status)")
            }
        }
    }

    /// Refuses const or enum on the aud of a token a gateway forwards by
    /// aud: the forwards are its fixed audience (ADR-141).
    func checkForwardedAudience(_ token: String, _ gateway: String, _ claims: YNode?, _ at: String) {
        let aud = claims?.child("aud")
        for key in ["const", "enum"] where aud?.child(key) != nil {
            add(aud?.key(key), at + "/claims/aud/" + key, .tokenAudience, "\(token) is forwarded by aud by gateway \(gateway), whose forwards are its fixed audience; remove \(key), which says the list a second time")
        }
    }
}
