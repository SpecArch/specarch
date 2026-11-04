package generate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The sign-in through an identity provider (ADR-182): what the techspec
// says of it, the checks the callback makes with the case each derives,
// and the flow as a sequence diagram.

// signInChecks are the checks the callback makes, in the order it makes
// them: the derived case, the security event's rule and severity, and the
// clause that asks for it.
var signInChecks = [][4]string{
	{tr("state does not match"), "sign_in_state", "medium", tr("RFC 6749, 10.12; RFC 9700, 4.7")},
	{tr("code used twice"), "sign_in_code", "high", tr("RFC 6749, 4.1.2; RFC 9700, 4.5")},
	{tr("signature invalid"), "sign_in_signature", "high", tr("OpenID Connect Core, 3.1.3.7, step 6")},
	{tr("issuer differs"), "sign_in_issuer", "high", tr("OpenID Connect Core, 3.1.3.7, step 2; RFC 9700, 4.4")},
	{tr("identity token expired"), "sign_in_expired", "low", tr("OpenID Connect Core, 3.1.3.7, step 9")},
	{tr("nonce replayed"), "sign_in_nonce", "high", tr("OpenID Connect Core, 3.1.3.7, step 11")},
}

// signInSection writes the sign-in flow of the runtime view, when the
// specification declares signIn.
func signInSection(d *doc, root *yaml.Node) {
	si := get(root, "signIn")
	if si == nil {
		return
	}
	d.heading(3, tr("Sign-in through ")+str(si, "provider"))
	d.para(str(si, "description"))
	d.explain(si)
	callback := str(si, "callback")
	path := callback
	if o := findOperation(root, callback); o != nil {
		path = fmt.Sprintf("%s (%s %s)", callback, strings.ToUpper(o.method), o.path)
	}
	d.para(fmt.Sprintf(tr("A person signs in through %s at %s, the identity provider, with OpenID Connect's authorisation code flow and PKCE's S256 method (RFC 6749, 4.1; RFC 7636). %s leaves the application for the provider's authorisation endpoint, which its discovery document names; the provider sends the person back to %s with the code and the state, and the callback exchanges the code at the provider for %s, the identity token. The provider's issuer and its keys, and the client_id of %s, are the deployment's: deployments.<environment>.signIn.issuerUrl and jwksUri, and clients.%s.clientId, in the implementation file."),
		str(si, "client"), str(si, "provider"), leavingPages(root), path, str(si, "identityToken"), str(si, "client"), str(si, "client")))
	d.para(tr("The callback refuses the sign-in, and starts no session, on each of these, each a derived case of ") + callback + ":")
	d.line(tr("| Check | Derived case | Security event | Severity | Asked for by |"))
	d.line("|---|---|---|---|---|")
	what := map[string]string{
		"state does not match":   tr("the state is the one the sign-in request set in this browser"),
		"code used twice":        tr("the code has not been exchanged before"),
		"signature invalid":      tr("the identity token's signature verifies with the provider's keys"),
		"issuer differs":         tr("iss is the deployment's issuerUrl"),
		"identity token expired": tr("exp has not passed"),
		"nonce replayed":         tr("nonce is the one this sign-in sent, used once"),
	}
	for _, c := range signInChecks {
		d.line("| %s | %s | %s | %s | %s |", what[c[0]], c[0], c[1], c[2], c[3])
	}
	d.blank()
	if back := str(get(si, "logout"), "backChannel"); back != "" {
		d.para(fmt.Sprintf(tr("When the person signs out at the provider, the provider calls %s on the back channel (OpenID Connect Back-Channel Logout 1.0), which only the service clients its callers list may call."), back))
	}
	d.block(signInSequence(root))
}

// leavingPages names the pages whose events leave for the sign-in.
func leavingPages(root *yaml.Node) string {
	var out []string
	for _, p := range pairs(root, "pages") {
		leaves := false
		for _, kv := range pairs(p.Value, "onSubmitted") {
			leaves = leaves || str(kv.Value, "navigate") == "signIn"
		}
		for _, a := range items(p.Value, "actions") {
			leaves = leaves || str(get(a, "then"), "navigate") == "signIn"
		}
		if leaves {
			out = append(out, tr("page ")+p.Key.Value)
		}
	}
	if len(out) == 0 {
		return tr("No page")
	}
	return capitalise(strings.Join(out, ", "))
}

// signInSequence draws the flow: the browser leaves for the provider, the
// provider sends the person back to the callback, and the callback
// exchanges the code and checks the identity token.
func signInSequence(root *yaml.Node) string {
	si := get(root, "signIn")
	provider := str(si, "provider")
	callback := str(si, "callback")
	route := callback
	if o := findOperation(root, callback); o != nil {
		route = o.path
	}
	var b strings.Builder
	b.WriteString(tr("sequenceDiagram\n"))
	b.WriteString(tr("  participant B as Browser\n"))
	fmt.Fprintf(&b, tr("  participant S as %s\n"), mermaidText(str(get(root, "info"), "title")))
	fmt.Fprintf(&b, tr("  participant I as %s\n"), mermaidText(provider))
	b.WriteString(tr("  B->>B: navigate signIn: a new state, nonce and PKCE verifier, kept in the __Host-sign-in cookie\n"))
	b.WriteString(tr("  B->>I: GET the discovery document at issuerUrl\n"))
	fmt.Fprintf(&b, tr("  B->>I: authorisation request: code, client_id of %s, redirect_uri %s, openid, state, nonce, S256 challenge\n"), mermaidText(str(si, "client")), mermaidText(route))
	fmt.Fprintf(&b, tr("  I-->>B: the person signs in, and is sent back to %s with the code and the state\n"), mermaidText(route))
	fmt.Fprintf(&b, tr("  B->>S: GET %s, the code and the state, with the cookie\n"), mermaidText(route))
	b.WriteString(tr("  S->>S: the state is the cookie's\n"))
	fmt.Fprintf(&b, tr("  S->>I: token request: the code and the PKCE verifier, within %s\n"), str(get(get(root, "dependencies"), provider), "timeout"))
	fmt.Fprintf(&b, tr("  I-->>S: %s, the identity token\n"), mermaidText(str(si, "identityToken")))
	b.WriteString(tr("  S->>S: the signature with the provider's keys, iss, aud, exp, the nonce once\n"))
	if o := findOperation(root, callback); o != nil {
		for _, r := range pairs(o.node, "responses") {
			if c := r.Key.Value; c[0] == '2' || c[0] == '3' {
				fmt.Fprintf(&b, tr("  S-->>B: %s %s\n"), c, mermaidText(str(r.Value, "description")))
				break
			}
		}
	}
	if back := str(get(si, "logout"), "backChannel"); back != "" {
		if o := findOperation(root, back); o != nil {
			fmt.Fprintf(&b, tr("  I->>S: %s %s, when the person signs out at the provider\n"), strings.ToUpper(o.method), mermaidText(o.path))
		}
	}
	return fence(b.String())
}
