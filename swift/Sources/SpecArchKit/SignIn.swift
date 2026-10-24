import Foundation

// Sign-in through an identity provider (ADR-182): OpenID Connect's
// authorisation code flow with PKCE, the callback an operation of this
// system, the identity token it receives, the back-channel logout, and
// navigate: signIn, the one page event that leaves the application (see
// the Go build's signin.go).

/// The reserved word a page event navigates to for the provider's sign-in.
/// A page's name is kebab-case, so no page is named so.
let navigateSignIn = "signIn"

/// The claims an identity token declares (OpenID Connect Core, 3.1.3.7).
let identityTokenClaims = ["iss", "aud", "nonce"]

extension Design {
    var signIn: YNode? { root.child("signIn") }

    /// The operationId of the sign-in callback, or "".
    var signInCallback: String { str(signIn?.child("callback")) }

    /// The refusals of the sign-in callback, one per check that makes the
    /// flow safe, each critical and each a security event once
    /// securityEvents is declared.
    func signInCases(_ s: Subject, _ o: Operation, _ call: String) {
        guard signIn != nil, o.id == signInCallback else { return }
        let refused = "it is refused and no session starts"
        s.byNature("state does not match", "a callback whose state is not the one the sign-in request set in this browser, a forged or replayed redirect", call, refused + event("sign_in_state"))
        s.byNature("nonce replayed", "an identity token whose nonce is not the one this sign-in sent, or one a sign-in has used already", call, refused + event("sign_in_nonce"))
        s.byNature("issuer differs", "an identity token whose iss is not the deployment's issuerUrl, an answer of another provider", call, refused + event("sign_in_issuer"))
        s.byNature("identity token expired", "an identity token whose exp has passed", call, refused + event("sign_in_expired"))
        s.byNature("signature invalid", "an identity token whose signature does not verify with the provider's keys", call, refused + event("sign_in_signature"))
        s.byNature("code used twice", "a code the callback has exchanged already", call, refused + ", and the provider refuses the second exchange" + event("sign_in_code"))
    }

    /// An operationId close to name, as a suggestion.
    func suggestOperation(_ name: String) -> String {
        var ops: [String: YNode] = [:]
        for (id, o) in operations { ops[id] = o.node }
        return suggest(name, ops)
    }
}

private func dependencyKind(_ dep: YNode) -> String {
    let k = str(dep.child("kind"))
    return k.isEmpty ? "system" : k
}

extension Checker {
    /// Checks the sign-in object (see the Go build's checkSignIn).
    func checkSignIn(_ d: Design) {
        guard let si = d.signIn else { return }
        let provider = str(si.child("provider"))
        if let n = si.child("provider") {
            if let dep = d.dependencies[provider] {
                if str(dep.child("kind")) != "identityProvider" {
                    add(n, "/signIn/provider", .signIn, "\(provider) is a dependency of kind \(dependencyKind(dep)), and the provider a person signs in at is of kind identityProvider; set kind: identityProvider on it")
                }
            } else {
                add(n, "/signIn/provider", .signIn, "\(provider) is not a dependency of the specification\(suggest(provider, d.dependencies))")
            }
        }
        if let n = si.child("client") {
            if let cl = d.client(n.value) {
                if str(cl.child("kind")) != "application" {
                    add(n, "/signIn/client", .signIn, "\(n.value) is a service, and a person signs in through an application; name the client of kind application whose client_id the identity token's aud carries")
                }
            } else {
                add(n, "/signIn/client", .signIn, "\(n.value) is not a client of the specification\(suggest(n.value, d.clientNames))")
            }
        }
        if let n = si.child("callback") { checkSignInCallback(d, n, provider) }
        if let n = si.child("identityToken") { checkIdentityToken(d, n) }
        if let n = si.child("logout")?.child("backChannel") { checkBackChannel(d, n) }
    }

    func checkSignInCallback(_ d: Design, _ n: YNode, _ provider: String) {
        guard let o = d.operations[n.value] else {
            add(n, "/signIn/callback", .signIn, "\(n.value) is not an operation of the specification\(d.suggestOperation(n.value))")
            return
        }
        if o.method != "get" {
            add(n, "/signIn/callback", .signIn, "\(o.id) is a \(o.method.uppercased()), and the provider sends the person back with a redirect the browser follows as a GET (RFC 6749, section 4.1.2); make the callback a get")
        }
        var params: [String: YNode] = [:]
        for p in items(o.pathItem.child("parameters")) + items(o.node.child("parameters")) where str(p.child("in")) == "query" {
            params[str(p.child("name"))] = p
        }
        for name in ["code", "state"] {
            if params[name] == nil || str(params[name]?.child("required")) != "true" {
                add(n, "/signIn/callback", .signIn, "\(o.id) takes no required query parameter \(name), which the provider sends back on the redirect (RFC 6749, section 4.1.2); add it under parameters with in: query and required: true")
            }
        }
        if !provider.isEmpty && !calledDependencies(o.node).contains(provider) {
            add(n, "/signIn/callback", .signIn, "\(o.id) does not name \(provider) under calls, and the callback exchanges the code for the identity token there (RFC 6749, section 4.1.3); add calls: [\(provider)] to it")
        }
    }

    func checkIdentityToken(_ d: Design, _ n: YNode) {
        guard let tok = d.token(n.value) else {
            add(n, "/signIn/identityToken", .signIn, "\(n.value) is not a token of the specification\(suggest(n.value, d.tokenNames))")
            return
        }
        let at = pointer("tokens", n.value)
        if str(tok.child("format")) != "jwt" {
            add(n, "/signIn/identityToken", .signIn, "\(n.value) is not a JWT, and an identity token is one (OpenID Connect Core, section 2); set format: jwt on it")
        }
        let claims = tok.child("claims")
        let missing = identityTokenClaims.filter { claims?.child($0) == nil }
        if !missing.isEmpty {
            add(n, "/signIn/identityToken", .signIn, "\(n.value) declares no \(missing.joined(separator: " or ")), and the callback checks the identity token's iss, aud and nonce (OpenID Connect Core, section 3.1.3.7); declare them under its claims")
        }
        for claim in ["iss", "aud"] {
            let cl = claims?.child(claim)
            for key in ["const", "enum"] where cl?.child(key) != nil {
                let what = claim == "aud" ? "the client's client_id, the deployment's clientId" : "the provider's issuer, the deployment's issuerUrl"
                add(cl?.key(key), at + "/claims/" + claim + "/" + key, .signIn, "\(claim) of the identity token is \(what), which differs per environment, so the design does not fix it; remove \(key)")
            }
        }
    }

    func checkBackChannel(_ d: Design, _ n: YNode) {
        guard let o = d.operations[n.value] else {
            add(n, "/signIn/logout/backChannel", .signIn, "\(n.value) is not an operation of the specification\(d.suggestOperation(n.value))")
            return
        }
        if d.admitsPeople(o.node) {
            add(n, "/signIn/logout/backChannel", .signIn, "\(o.id) may be called by people, and the provider calls the back-channel logout itself (OpenID Connect Back-Channel Logout 1.0, section 2.5); list only the service client it calls through under callers")
            return
        }
        for item in items(o.node.child("callers")) {
            if let cl = d.client(item.value), str(cl.child("kind")) != "service" {
                add(n, "/signIn/logout/backChannel", .signIn, "\(o.id) may be called by \(item.value), an application, and the provider calls the back-channel logout itself; list only service clients under callers")
            }
        }
    }

    /// Checks a page event that navigates to signIn (see the Go build's
    /// checkSignInEvent).
    func checkSignInEvent(_ d: Design, _ ev: YNode, _ nav: YNode, _ base: [String], _ leaves: Bool) {
        let at = pointer(base + ["navigate"])
        guard d.signIn != nil else {
            add(nav, at, .signIn, "signIn leaves the application for the identity provider, and the specification declares no signIn; declare signIn with its provider, client, callback and identity token, or navigate to a page")
            return
        }
        guard leaves else {
            add(nav, at, .signIn, "signIn leaves the application, which a task page's answer or an action's then may do; lead this event to a page")
            return
        }
        for key in ["with", "message"] where ev.child(key) != nil {
            add(ev.key(key), pointer(base + [key]), .signIn, "the event navigates to signIn and leaves the application, so nothing of it is on screen to take \(key); remove \(key)")
        }
    }
}
