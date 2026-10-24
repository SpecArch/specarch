package validate

import (
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// Sign-in through an identity provider (ADR-182): OpenID Connect's
// authorisation code flow with PKCE, the callback an operation of this
// system, the identity token it receives, the back-channel logout, and
// navigate: signIn, the one page event that leaves the application.

// navigateSignIn is the reserved word a page event navigates to for the
// provider's sign-in. A page's name is kebab-case, so no page is named so.
const navigateSignIn = "signIn"

// The claims an identity token declares (OpenID Connect Core, 3.1.3.7).
var identityTokenClaims = []string{"iss", "aud", "nonce"}

func (d *design) signIn() *yaml.Node { return source.Child(d.root, "signIn") }

// signInCallback is the operationId of the sign-in callback, or "".
func (d *design) signInCallback() string {
	return source.Str(source.Child(d.signIn(), "callback"))
}

// checkSignIn checks the sign-in object: the provider is a dependency of
// kind identityProvider, the client an application, the callback a GET
// taking code and state that calls the provider, the identity token a JWT
// declaring iss, aud and nonce with neither iss nor aud fixed, and the
// back-channel logout an operation only service clients call.
func (c *checker) checkSignIn(d *design) {
	si := d.signIn()
	if si == nil {
		return
	}
	provider := source.Str(source.Child(si, "provider"))
	if n := source.Child(si, "provider"); n != nil {
		dep := d.dependencies[provider]
		switch {
		case dep == nil:
			c.add(n, "/signIn/provider", RuleSignIn, "%s is not a dependency of the specification%s", provider, suggest(provider, d.dependencies))
		case source.Str(source.Child(dep, "kind")) != "identityProvider":
			c.add(n, "/signIn/provider", RuleSignIn, "%s is a dependency of kind %s, and the provider a person signs in at is of kind identityProvider; set kind: identityProvider on it", provider, dependencyKind(dep))
		}
	}
	if n := source.Child(si, "client"); n != nil {
		cl := d.client(n.Value)
		switch {
		case cl == nil:
			c.add(n, "/signIn/client", RuleSignIn, "%s is not a client of the specification%s", n.Value, suggest(n.Value, d.clientNames()))
		case source.Str(source.Child(cl, "kind")) != "application":
			c.add(n, "/signIn/client", RuleSignIn, "%s is a service, and a person signs in through an application; name the client of kind application whose client_id the identity token's aud carries", n.Value)
		}
	}
	if n := source.Child(si, "callback"); n != nil {
		c.checkSignInCallback(d, n, provider)
	}
	if n := source.Child(si, "identityToken"); n != nil {
		c.checkIdentityToken(d, n)
	}
	if n := source.Child(source.Child(si, "logout"), "backChannel"); n != nil {
		c.checkBackChannel(d, n)
	}
}

func dependencyKind(dep *yaml.Node) string {
	if k := source.Str(source.Child(dep, "kind")); k != "" {
		return k
	}
	return "system"
}

// checkSignInCallback checks the callback: an operation of this system,
// a GET, taking code and state as required query parameters, and naming
// the provider under calls, since it exchanges the code there.
func (c *checker) checkSignInCallback(d *design, n *yaml.Node, provider string) {
	o, ok := d.operations[n.Value]
	if !ok {
		c.add(n, "/signIn/callback", RuleSignIn, "%s is not an operation of the specification%s", n.Value, suggestOperation(n.Value, d))
		return
	}
	if o.method != "get" {
		c.add(n, "/signIn/callback", RuleSignIn, "%s is a %s, and the provider sends the person back with a redirect the browser follows as a GET (RFC 6749, section 4.1.2); make the callback a get", o.id, strings.ToUpper(o.method))
	}
	params := map[string]*yaml.Node{}
	for _, p := range append(source.Items(source.Child(o.pathItem, "parameters")), source.Items(source.Child(o.node, "parameters"))...) {
		if source.Str(source.Child(p, "in")) == "query" {
			params[source.Str(source.Child(p, "name"))] = p
		}
	}
	for _, name := range []string{"code", "state"} {
		p := params[name]
		if p == nil || source.Str(source.Child(p, "required")) != "true" {
			c.add(n, "/signIn/callback", RuleSignIn, "%s takes no required query parameter %s, which the provider sends back on the redirect (RFC 6749, section 4.1.2); add it under parameters with in: query and required: true", o.id, name)
		}
	}
	if provider != "" && !contains(calledDependencies(o.node), provider) {
		c.add(n, "/signIn/callback", RuleSignIn, "%s does not name %s under calls, and the callback exchanges the code for the identity token there (RFC 6749, section 4.1.3); add calls: [%s] to it", o.id, provider, provider)
	}
}

// suggestOperation suggests an operationId close to name.
func suggestOperation(name string, d *design) string {
	ops := map[string]*yaml.Node{}
	for id, o := range d.operations {
		ops[id] = o.node
	}
	return suggest(name, ops)
}

// checkIdentityToken checks the identity token: a JWT declaring iss, aud
// and nonce, neither iss nor aud fixed in the design, since the issuer and
// the client_id are the deployment's.
func (c *checker) checkIdentityToken(d *design, n *yaml.Node) {
	tok := d.token(n.Value)
	if tok == nil {
		c.add(n, "/signIn/identityToken", RuleSignIn, "%s is not a token of the specification%s", n.Value, suggest(n.Value, d.tokenNames()))
		return
	}
	at := source.Pointer("tokens", n.Value)
	if source.Str(source.Child(tok, "format")) != "jwt" {
		c.add(n, "/signIn/identityToken", RuleSignIn, "%s is not a JWT, and an identity token is one (OpenID Connect Core, section 2); set format: jwt on it", n.Value)
	}
	claims := source.Child(tok, "claims")
	var missing []string
	for _, claim := range identityTokenClaims {
		if source.Child(claims, claim) == nil {
			missing = append(missing, claim)
		}
	}
	if len(missing) > 0 {
		c.add(n, "/signIn/identityToken", RuleSignIn, "%s declares no %s, and the callback checks the identity token's iss, aud and nonce (OpenID Connect Core, section 3.1.3.7); declare them under its claims", n.Value, strings.Join(missing, " or "))
	}
	for _, claim := range []string{"iss", "aud"} {
		cl := source.Child(claims, claim)
		for _, key := range []string{"const", "enum"} {
			if source.Child(cl, key) != nil {
				what := "the provider's issuer, the deployment's issuerUrl"
				if claim == "aud" {
					what = "the client's client_id, the deployment's clientId"
				}
				c.add(source.Key(cl, key), at+"/claims/"+claim+"/"+key, RuleSignIn, "%s of the identity token is %s, which differs per environment, so the design does not fix it; remove %s", claim, what, key)
			}
		}
	}
}

// checkBackChannel checks the back-channel logout: an operation that only
// declared service clients may call, since the provider calls it.
func (c *checker) checkBackChannel(d *design, n *yaml.Node) {
	o, ok := d.operations[n.Value]
	if !ok {
		c.add(n, "/signIn/logout/backChannel", RuleSignIn, "%s is not an operation of the specification%s", n.Value, suggestOperation(n.Value, d))
		return
	}
	if d.admitsPeople(o.node) {
		c.add(n, "/signIn/logout/backChannel", RuleSignIn, "%s may be called by people, and the provider calls the back-channel logout itself (OpenID Connect Back-Channel Logout 1.0, section 2.5); list only the service client it calls through under callers", o.id)
		return
	}
	for _, item := range source.Items(source.Child(o.node, "callers")) {
		if cl := d.client(item.Value); cl != nil && source.Str(source.Child(cl, "kind")) != "service" {
			c.add(n, "/signIn/logout/backChannel", RuleSignIn, "%s may be called by %s, an application, and the provider calls the back-channel logout itself; list only service clients under callers", o.id, item.Value)
		}
	}
}

// checkSignInEvent checks a page event that navigates to signIn: the
// specification declares signIn, the event is a task page's answer or an
// action's then (leaves), and it gives neither with nor message, since
// nothing of this application is on screen after it.
func (c *checker) checkSignInEvent(d *design, ev, nav *yaml.Node, base []string, leaves bool) {
	at := source.Pointer(append(base, "navigate")...)
	if d.signIn() == nil {
		c.add(nav, at, RuleSignIn, "signIn leaves the application for the identity provider, and the specification declares no signIn; declare signIn with its provider, client, callback and identity token, or navigate to a page")
		return
	}
	if !leaves {
		c.add(nav, at, RuleSignIn, "signIn leaves the application, which a task page's answer or an action's then may do; lead this event to a page")
		return
	}
	for _, key := range []string{"with", "message"} {
		if source.Child(ev, key) != nil {
			c.add(source.Key(ev, key), source.Pointer(append(base, key)...), RuleSignIn, "the event navigates to signIn and leaves the application, so nothing of it is on screen to take %s; remove %s", key, key)
		}
	}
}

// signInCases adds the refusals of the sign-in callback, one per check that
// makes the flow safe, each critical, since nobody exercises them by hand,
// and each a security event once securityEvents is declared (ADR-182).
func (d *design) signInCases(s *subject, o operation, call string) {
	if d.signIn() == nil || o.id != d.signInCallback() {
		return
	}
	refused := "it is refused and no session starts"
	s.byNature("state does not match", "a callback whose state is not the one the sign-in request set in this browser, a forged or replayed redirect", call, refused+d.event("sign_in_state"))
	s.byNature("nonce replayed", "an identity token whose nonce is not the one this sign-in sent, or one a sign-in has used already", call, refused+d.event("sign_in_nonce"))
	s.byNature("issuer differs", "an identity token whose iss is not the deployment's issuerUrl, an answer of another provider", call, refused+d.event("sign_in_issuer"))
	s.byNature("identity token expired", "an identity token whose exp has passed", call, refused+d.event("sign_in_expired"))
	s.byNature("signature invalid", "an identity token whose signature does not verify with the provider's keys", call, refused+d.event("sign_in_signature"))
	s.byNature("code used twice", "a code the callback has exchanged already", call, refused+", and the provider refuses the second exchange"+d.event("sign_in_code"))
}
