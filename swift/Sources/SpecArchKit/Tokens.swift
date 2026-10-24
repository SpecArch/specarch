import Foundation

// Tokens (meta-model 0.3, ADR-097, ADR-108, ADR-109, ADR-111): the
// formats a service issues and accepts, a transaction token across a
// trust domain with via on each operation, the audience fixed per token
// kind, and a security event for every refusal of a caller. See the Go
// build's tokens.go, which this mirrors.

struct RegisteredClaim { let typ: String; let doc: String }

let registeredClaims: [String: RegisteredClaim] = [
    "iss": .init(typ: "string", doc: "RFC 7519"), "sub": .init(typ: "string", doc: "RFC 7519"), "aud": .init(typ: "string", doc: "RFC 7519"), "exp": .init(typ: "integer", doc: "RFC 7519"),
    "nbf": .init(typ: "integer", doc: "RFC 7519"), "iat": .init(typ: "integer", doc: "RFC 7519"), "jti": .init(typ: "string", doc: "RFC 7519"),
    "act": .init(typ: "object", doc: "RFC 8693"), "scope": .init(typ: "string", doc: "RFC 8693"), "client_id": .init(typ: "string", doc: "RFC 8693"), "may_act": .init(typ: "object", doc: "RFC 8693"),
    "cnf": .init(typ: "object", doc: "RFC 7800"),
    "txn": .init(typ: "string", doc: "RFC 8417"), "events": .init(typ: "object", doc: "RFC 8417"), "toe": .init(typ: "integer", doc: "RFC 8417"),
    "auth_time": .init(typ: "integer", doc: "OpenID Connect Core"), "acr": .init(typ: "string", doc: "OpenID Connect Core"), "amr": .init(typ: "array", doc: "OpenID Connect Core"), "azp": .init(typ: "string", doc: "OpenID Connect Core"),
    "req_wl": .init(typ: "string", doc: "the Transaction Tokens draft"), "rctx": .init(typ: "object", doc: "the Transaction Tokens draft"), "tctx": .init(typ: "object", doc: "the Transaction Tokens draft"),
]
let txnTokenClaims = ["txn", "sub", "scope", "req_wl"]
let jwtAlgorithms = ["ES256", "ES384", "ES512", "EdDSA", "HS256", "HS384", "HS512", "PS256", "PS384", "PS512", "RS256", "RS384", "RS512"]
let workloadProofs: Set<String> = ["mutualTls", "spiffe", "wimse"]
let eventSeverity = ["token_audience": "high", "via": "high", "deputy": "high", "callers": "medium", "through": "medium", "software_version": "medium", "token": "low",
                     "sign_in_state": "medium", "sign_in_nonce": "high", "sign_in_issuer": "high", "sign_in_expired": "low", "sign_in_signature": "high", "sign_in_code": "high"]

func tokenType(_ tok: YNode?) -> String {
    let t = str(tok?.child("tokenType"))
    return t.isEmpty ? "access_token" : t
}

extension Design {
    var tokenList: [Pair] { pairs(root.child("tokens")) }
    func token(_ name: String) -> YNode? { root.child("tokens")?.child(name) }
    var tokenNames: [String: YNode] {
        var out: [String: YNode] = [:]
        for t in tokenList { out[t.key.value] = t.value }
        return out
    }
    var defaultToken: String { tokenList.first { str($0.value.child("default")) == "true" }?.key.value ?? "" }
    /// The token an operation accepts: the one it names; on an operation
    /// only clients call, the token the first listed client with a token
    /// proof presents, or "" when none does; else the default, or "".
    func acceptedToken(_ op: YNode) -> String {
        let name = str(op.child("token"))
        if !name.isEmpty { return name }
        if !admitsPeople(op) {
            for item in items(op.child("callers")) {
                let proof = client(item.value)?.child("proof")
                if str(proof?.child("kind")) == "token", !str(proof?.child("token")).isEmpty {
                    return str(proof?.child("token"))
                }
            }
            return ""
        }
        return defaultToken
    }
    var acceptedTokens: Set<String> { Set(opList.map { acceptedToken($0.node) }.filter { !$0.isEmpty }) }

    func claimType(_ name: String, _ claim: YNode) -> ExprType {
        if let r = registeredClaims[name], claim.child("type") == nil {
            switch r.typ {
            case "integer": return ExprType(.int, bits: 64)
            case "object": return ExprType(.object)
            case "array": return ExprType(.list)
            default: return ExprType(.string)
            }
        }
        return fieldType(claim)
    }

    /// The clause a derived refusal's then ends with once the specification
    /// says where security events go (ADR-109).
    func event(_ rule: String) -> String {
        if root.child("securityEvents") == nil && identity?.design.root.child("securityEvents") == nil { return "" }
        var clause = ", and the security event " + rule + " (" + (eventSeverity[rule] ?? "") + ")"
        var domain = trustDomain
        if !domain.isEmpty {
            while domain.hasSuffix("/") { domain.removeLast() }
            clause += ", " + domain + "/events/" + rule + ","
        }
        return clause + " is written"
    }

    func versionEvent(_ cl: YNode) -> String {
        var e = event("software_version")
        if !e.isEmpty && str(cl.child("softwareStatement")) != "true" {
            e = e.replacingOccurrences(of: "(medium)", with: "(low)")
        }
        return e
    }

    var hasThrough: Bool { pairs(root.child("roles")).contains { $0.value.child("through") != nil } }

    /// The cases of the token an operation accepts (see the Go build's
    /// tokenCases).
    func tokenCases(_ s: Subject, _ o: Operation, _ perm: String, _ call: String) {
        let name = acceptedToken(o.node)
        guard let tok = token(name) else { return }
        if signIn != nil && o.id == signInCallback { return } // the sign-in callback is called before any token is issued
        let claims = tok.child("claims")
        if !perm.isEmpty && perm != permissionPublic && perm != permissionNobody {
            s.red("denied with expired " + name, frequent, "a caller whose " + name + " token has expired after " + str(tok.child("lifetime")), call, "it is refused as not signed in" + event("token"))
        }
        for ch in pairs(tok.child("checks")) {
            s.red(ch.key.value + " fails on " + name, occasional, indefinite(name) + " " + name + " token for which " + str(ch.value.child("expression")) + " does not hold", call, "it is refused: " + str(ch.value.child("message")) + event("token"))
        }
        if claims?.child("act") != nil {
            s.red("refused when the actor is the subject", rare, indefinite(name) + " " + name + " token whose act names its own sub", call, "it is refused as not allowed: a client may not act for itself (RFC 8693)" + event("deputy"))
        }
        let txn = tokenType(tok) == "txn_token"
        if !txn {
            s.red("refused with a token for another audience", occasional, indefinite(name) + " " + name + " token minted for another service, whose aud does not name this one", call, "it is refused as not allowed" + event("token_audience"))
        }
        if o.node.child("via") != nil {
            s.red("refused with a caller not listed in via", rare, "the edge calling this operation directly with the transaction token, a caller via does not list", call, "it is refused as not allowed" + event("via"))
        }
        if hasService && admitsPeople(o.node) {
            if txn {
                s.red("refused with a req_wl that is not an edge", rare, indefinite(name) + " " + name + " token whose req_wl names a later hop, which requested a transaction of its own", call, "it is refused as not allowed" + event("deputy"))
                s.red("refused with a txn differing from the one received", rare, indefinite(name) + " " + name + " token whose txn is not the one the hop received, minted anew instead of passed on unmodified", call, "it is refused as not allowed" + event("deputy"))
                s.red("refused with a scope no edge of its chain checked", rare, indefinite(name) + " " + name + " token whose scope is not the permission of an edge operation whose chain reaches this one, a transaction minted for another purpose", call, "it is refused as not allowed" + event("deputy"))
            } else {
                s.red("refused for a person's token requested by a service with no actor", rare, indefinite(name) + " " + name + " token whose client_id names a service and which declares no act, impersonation rather than delegation (RFC 8693, section 1.1)", call, "it is refused as not allowed" + event("deputy"))
            }
        }
    }
}

extension Checker {
    /// Checks every token, the operations' token and via, the audience of
    /// every accepted access token, and the security events declaration
    /// (see the Go build's checkTokens).
    func checkTokens(_ d: Design) {
        let tokens = d.tokenList
        let accepted = d.acceptedTokens
        let forwarded = d.forwardedByAudience
        var defaults = 0, own = 0
        for t in tokens {
            let name = t.key.value
            let at = pointer("tokens", name)
            if forwarded[name] == nil { own += 1 }
            let format = str(t.value.child("format"))
            let sig = t.value.child("signature")
            if format == "jwt" && sig == nil {
                add(t.key, at, .token, "\(name) is a JWT with no signature; name an algorithm of RFC 7518 under signature, one of \(jwtAlgorithms.joined(separator: ", "))")
            } else if format == "opaque", let sig {
                add(sig, at + "/signature", .token, "\(name) is opaque, a reference the service looks up, and has no signature; remove signature")
            } else if let sig, str(sig).lowercased() == "none" {
                add(sig, at + "/signature", .token, "none is not accepted: RFC 8725 says a verifier must refuse the none algorithm, since a token with no signature is anyone's; name one of \(jwtAlgorithms.joined(separator: ", "))")
            } else if let sig, !jwtAlgorithms.contains(str(sig)) {
                add(sig, at + "/signature", .token, "\(str(sig)) is not a signature algorithm RFC 7518 or RFC 8037 registers; use one of \(jwtAlgorithms.joined(separator: ", "))")
            }
            if str(t.value.child("default")) == "true" {
                defaults += 1
                if defaults > 1 {
                    add(t.value.child("default"), at + "/default", .token, "\(name) is marked default while \(d.defaultToken) is already; one token is the default every operation accepts")
                }
            }
            let claims = t.value.child("claims")
            checkClaims(d, name, claims, at + "/claims")
            checkTokenChecks(d, name, t.value, claims, at)
            if tokenType(t.value) == "txn_token" {
                if format != "jwt" {
                    add(t.value.child("format"), at + "/format", .token, "\(name) is a transaction token, which is a JWT (the Transaction Tokens draft); set format: jwt")
                }
                for claim in txnTokenClaims where claims?.child(claim) == nil {
                    add(t.key, at, .token, "\(name) is a transaction token and declares no \(claim); a transaction token declares the draft's claims \(txnTokenClaims.joined(separator: ", "))")
                }
                if claims?.child("aud") != nil {
                    add(claims?.key("aud"), at + "/claims/aud", .token, "aud is not written on a transaction token: its audience is the trust domain, one value for every hop (the draft, section 9.2); remove aud")
                }
                if accepted.contains(name) && d.hasThrough && claims?.child("tctx")?.child("properties")?.child("application") == nil {
                    add(t.key, at, .token, "\(name) is accepted where a role's through reaches, and declares no tctx.application, the application the person signed in through, immutable from the edge on; add tctx with properties.application")
                }
            } else if let gw = forwarded[name] {
                checkForwardedAudience(name, gw, claims, at)
            } else if accepted.contains(name) {
                checkTokenAudience(name, claims, at)
            }
        }
        if own > 0 && defaults == 0 {
            add(d.root.key("tokens"), "/tokens", .token, "no token is marked default, so an operation naming no token accepts none; mark one default: true")
        }
        for o in d.opList {
            let name = str(o.node.child("token"))
            if !name.isEmpty && d.token(name) == nil {
                add(o.node.child("token"), o.pointer("token"), .token, "\(name) is not a token of the specification\(suggest(name, d.tokenNames))")
            }
            checkVia(d, o)
            checkChain(d, o)
        }
        if !tokens.isEmpty && !d.clientList.isEmpty && d.root.child("securityEvents") == nil {
            add(d.root.key("tokens"), "/tokens", .securityEvents, "the specification declares tokens and clients, so every refusal of a caller is a security event, and declares no securityEvents; add securityEvents with store and alarm, two dependencies")
        }
        if let se = d.root.child("securityEvents") {
            for key in ["store", "alarm"] {
                let dep = str(se.child(key))
                if !dep.isEmpty && d.dependencies[dep] == nil {
                    add(se.child(key), "/securityEvents/" + key, .securityEvents, "\(dep) is not a dependency of the specification\(suggest(dep, d.dependencies))")
                }
            }
        }
    }

    func checkClaims(_ d: Design, _ token: String, _ claims: YNode?, _ at: String) {
        var callerProps: [String: YNode] = [:]
        for p in pairs(d.root.child("caller")?.child("properties")) { callerProps[p.key.value] = p.value }
        for cl in pairs(claims) {
            let name = cl.key.value
            let cat = at + "/" + name
            if let r = registeredClaims[name] {
                if let typ = cl.value.child("type"), !typeIs(cl.value, r.typ) {
                    add(typ, cat + "/type", .claim, "\(name) is a registered claim (\(r.doc)) of type \(r.typ), and is written as \(str(typ)); a registered claim carries the registry's type without being retyped; remove type")
                }
            } else if name.contains("_") {
                add(cl.key, cat, .claim, "\(name) has an underscore and is not a claim of the JSON Web Token Claims registry that SpecArch knows; a claim of this specification's own is camelCase, as every property is")
            }
            if let prop = callerProps[name] {
                let ct = d.claimType(name, cl.value), pt = d.fieldType(prop)
                if ct.kind != pt.kind {
                    add(cl.key, cat, .claim, "the claim \(name) fills the caller's property \(name), and their types differ (\(ct) and \(pt)); give them one type")
                }
            }
        }
    }

    func checkTokenChecks(_ d: Design, _ token: String, _ tok: YNode, _ claims: YNode?, _ at: String) {
        var env: ExprEnv = [:]
        for cl in pairs(claims) { env[cl.key.value] = d.claimType(cl.key.value, cl.value) }
        for ch in pairs(tok.child("checks")) {
            guard let n = ch.value.child("expression"), n.kind == .scalar else { continue }
            let ptr = at + "/checks/" + ch.key.value + "/expression"
            let (tree, perrs) = parseExpr(n.value)
            guard let tree, perrs.isEmpty else {
                tokenErrors(n, ptr, perrs)
                continue
            }
            let (t, errs) = checkExpr(tree, env)
            tokenErrors(n, ptr, errs)
            if errs.isEmpty && (t.kind != .bool || t.nullable) {
                addFile(fileOf(n), exprLine(n, 1), ptr, .token, "the check gives \(t), but a check must give true or false; compare the claims with ==, <, > or similar")
            }
        }
    }

    func tokenErrors(_ n: YNode, _ ptr: String, _ errs: [ExprError]) {
        let before = diags.count
        exprErrors(n, ptr, "the check", errs)
        for i in before..<diags.count where diags[i].rule == .expressionName { diags[i].rule = .token }
    }

    func checkTokenAudience(_ token: String, _ claims: YNode?, _ at: String) {
        let aud = claims?.child("aud")
        let cnst = aud?.child("const"), en = aud?.child("enum")
        if aud == nil || (cnst == nil && en == nil) {
            add(claims?.key("aud"), at + "/claims/aud", .tokenAudience, "\(token) is accepted by an operation and fixes no audience; declare aud with const naming this service, or enum naming the few it stands for, so that a token minted for another service is refused (RFC 9068, section 4; RFC 8707)")
            return
        }
        var values: [YNode] = []
        if let cnst { values.append(cnst) }
        values.append(contentsOf: items(en))
        for v in values {
            let s = str(v)
            if s.contains(":"), !(URL(string: s)?.scheme.map { !$0.isEmpty } ?? false) {
                add(v, at + "/claims/aud", .tokenAudience, "\(s) has a colon and is not a URI; an audience with a colon is a URI (RFC 7519, section 2), such as https://loans.example.org")
            }
        }
    }

    func checkVia(_ d: Design, _ o: Operation) {
        let via = o.node.child("via")
        let tokName = d.acceptedToken(o.node)
        let tok = d.token(tokName)
        let needs = tok != nil && (tokenType(tok) == "txn_token" || tok?.child("claims")?.child("act") != nil)
        let at = o.pointer("via")
        if via == nil && needs {
            add(o.node.key("operationId"), o.pointer(), .via, "operation \(o.id) accepts \(tokName), a transaction token or a token declaring act, and names no via; list its immediate callers, the services that call it on a person's behalf")
            return
        }
        guard let via else { return }
        if !needs {
            add(via, at, .via, "operation \(o.id) accepts neither a transaction token nor a token declaring act, and via names the hops of a chain; remove via, or accept the chain's token")
        }
        let list = items(via)
        if list.isEmpty {
            add(via, at, .via, "via names no caller, which would make operation \(o.id) a route nobody reaches; list the services, or close it with permission: nobody")
        }
        var seen = Set<String>()
        for (i, item) in list.enumerated() {
            let name = item.value
            let iat = at + "/\(i)"
            if seen.contains(name) {
                add(item, iat, .via, "\(name) is listed twice; list it once")
                continue
            }
            seen.insert(name)
            var found = d.client(name)
            if isCross(name) {
                // A client of the identity module: the reference check says
                // whether it is there, and it is checked once it is given.
                found = referencedClient(d, name)?.node
                if found == nil { continue }
            }
            guard let cl = found else {
                add(item, iat, .via, "\(name) is not a client of the specification\(suggest(name, d.clientNames))")
                continue
            }
            if str(cl.child("kind")) == "application" {
                add(item, iat, .via, "\(name) is an application, and via names the services that carry a person's call; a person's own application calls the edge with an access token")
                continue
            }
            let kind = str(cl.child("proof")?.child("kind"))
            if !workloadProofs.contains(kind) {
                add(item, iat, .via, "\(name) proves itself with \(kind.isEmpty ? "nothing" : kind), and a hop in a chain presents a workload identity: mutualTls, spiffe or wimse")
            }
        }
    }

    func checkTokenProof(_ d: Design, _ client: String, _ proof: YNode, _ at: String) {
        let kind = str(proof.child("kind"))
        let tok = proof.child("token")
        switch kind {
        case "token":
            guard let tok else {
                add(proof, at, .proof, "a token proof names the token format the client presents; add token, a name under tokens")
                return
            }
            if let t = d.token(str(tok)) {
                if tokenType(t) == "txn_token" {
                    add(tok, at + "/token", .proof, "\(str(tok)) is a transaction token, and a workload must not authenticate itself with one (the Transaction Tokens draft, section 13.12); name an access token")
                }
            } else {
                add(tok, at + "/token", .proof, "\(str(tok)) is not a token of the specification\(suggest(str(tok), d.tokenNames))")
            }
        case "spiffe", "wimse":
            if proof.child("algorithm") != nil || proof.child("covers") != nil || tok != nil {
                add(proof, at, .proof, "a \(kind) proof is the workload's own identity and names no algorithm, covered components or token; remove them")
            }
        default:
            if let tok {
                add(tok, at + "/token", .proof, "token goes with a proof of kind token; remove it, or set kind: token")
            }
        }
    }
}
