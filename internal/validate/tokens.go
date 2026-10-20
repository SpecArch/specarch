package validate

import (
	"net/url"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/source"
)

// Tokens (meta-model 0.3, ADR-097, ADR-108, ADR-109, ADR-111): the
// formats a service issues and accepts, each with its claims, its
// lifetime and its signature; a transaction token across a trust domain,
// carried in the Txn-Token header, with via on each operation naming its
// immediate callers; the audience fixed per token kind; and a security
// event for every refusal of a caller.

// registeredClaim is a claim of the JSON Web Token Claims registry that
// SpecArch knows: its type, as the registering document gives it, and
// the document.
type registeredClaim struct {
	typ string // string, integer, object or array
	doc string
}

var registeredClaims = map[string]registeredClaim{
	"iss": {"string", "RFC 7519"}, "sub": {"string", "RFC 7519"}, "aud": {"string", "RFC 7519"}, "exp": {"integer", "RFC 7519"},
	"nbf": {"integer", "RFC 7519"}, "iat": {"integer", "RFC 7519"}, "jti": {"string", "RFC 7519"},
	"act": {"object", "RFC 8693"}, "scope": {"string", "RFC 8693"}, "client_id": {"string", "RFC 8693"}, "may_act": {"object", "RFC 8693"},
	"cnf": {"object", "RFC 7800"},
	"txn": {"string", "RFC 8417"}, "events": {"object", "RFC 8417"}, "toe": {"integer", "RFC 8417"},
	"auth_time": {"integer", "OpenID Connect Core"}, "acr": {"string", "OpenID Connect Core"}, "amr": {"array", "OpenID Connect Core"}, "azp": {"string", "OpenID Connect Core"},
	"req_wl": {"string", "the Transaction Tokens draft"}, "rctx": {"object", "the Transaction Tokens draft"}, "tctx": {"object", "the Transaction Tokens draft"},
}

// The claims a transaction token declares (the draft, section 9.2).
var txnTokenClaims = []string{"txn", "sub", "scope", "req_wl"}

// The signature algorithms of RFC 7518 and RFC 8037.
var jwtAlgorithms = []string{"ES256", "ES384", "ES512", "EdDSA", "HS256", "HS384", "HS512", "PS256", "PS384", "PS512", "RS256", "RS384", "RS512"}

// The proof kinds that are a workload identity, which a hop in a chain
// presents.
var workloadProofs = map[string]bool{"mutualTls": true, "spiffe": true, "wimse": true}

// The severity of the security event each refusal derives (ADR-109).
var eventSeverity = map[string]string{"token_audience": "high", "via": "high", "deputy": "high", "callers": "medium", "through": "medium", "software_version": "medium", "token": "low"}

func (d *design) tokens() []source.Pair { return source.Pairs(source.Child(d.root, "tokens")) }

func (d *design) token(name string) *yaml.Node {
	return source.Child(source.Child(d.root, "tokens"), name)
}

func (d *design) tokenNames() map[string]*yaml.Node { return topMap(d.root, "tokens") }

// defaultToken is the token every operation accepts unless it names
// another, or "".
func (d *design) defaultToken() string {
	for _, t := range d.tokens() {
		if source.Str(source.Child(t.Value, "default")) == "true" {
			return t.Key.Value
		}
	}
	return ""
}

// acceptedToken is the token an operation accepts: the one it names; on
// an operation only clients call, the token the first listed client with
// a token proof presents, or "" when every one proves itself otherwise
// (a signed request, a certificate); else the default, or "".
func (d *design) acceptedToken(op *yaml.Node) string {
	if name := source.Str(source.Child(op, "token")); name != "" {
		return name
	}
	if !d.admitsPeople(op) {
		for _, item := range source.Items(source.Child(op, "callers")) {
			proof := source.Child(d.client(item.Value), "proof")
			if source.Str(source.Child(proof, "kind")) == "token" {
				if name := source.Str(source.Child(proof, "token")); name != "" {
					return name
				}
			}
		}
		return ""
	}
	return d.defaultToken()
}

func tokenType(tok *yaml.Node) string {
	if t := source.Str(source.Child(tok, "tokenType")); t != "" {
		return t
	}
	return "access_token"
}

// acceptedTokens are the tokens some operation accepts.
func (d *design) acceptedTokens() map[string]bool {
	out := map[string]bool{}
	for _, o := range d.opList {
		if name := d.acceptedToken(o.node); name != "" {
			out[name] = true
		}
	}
	return out
}

// claimType is a claim's type in the expression language: the registry's
// for a registered claim, the field's otherwise.
func (d *design) claimType(name string, claim *yaml.Node) expr.Type {
	if r, ok := registeredClaims[name]; ok && source.Child(claim, "type") == nil {
		switch r.typ {
		case "integer":
			return expr.Type{Kind: expr.Int, Bits: 64}
		case "object":
			return expr.Type{Kind: expr.Object}
		case "array":
			return expr.Type{Kind: expr.List}
		}
		return expr.Type{Kind: expr.String}
	}
	return d.fieldType(claim)
}

// event is the clause a derived refusal's then ends with once the
// specification, or the identity module given with it, declares where
// security events go: the event's rule and severity (ADR-109), and its
// identifier under the trust domain when one is known (ADR-171).
func (d *design) event(rule string) string {
	if source.Child(d.root, "securityEvents") == nil && (d.identity == nil || source.Child(d.identity.design.root, "securityEvents") == nil) {
		return ""
	}
	clause := ", and the security event " + rule + " (" + eventSeverity[rule] + ")"
	if domain := d.trustDomain(); domain != "" {
		clause += ", " + strings.TrimRight(domain, "/") + "/events/" + rule + ","
	}
	return clause + " is written"
}

// hasThrough reports whether any role narrows a grant by client.
func (d *design) hasThrough() bool {
	for _, r := range source.Pairs(source.Child(d.root, "roles")) {
		if source.Child(r.Value, "through") != nil {
			return true
		}
	}
	return false
}

// checkTokens checks every token, the operations' token and via, the
// audience of every accepted access token, and that a specification with
// tokens and clients says where its security events go.
func (c *checker) checkTokens(d *design) {
	tokens := d.tokens()
	accepted := d.acceptedTokens()
	forwarded := d.forwardedByAudience()
	defaults, own := 0, 0
	for _, t := range tokens {
		name := t.Key.Value
		at := source.Pointer("tokens", name)
		if _, ok := forwarded[name]; !ok {
			own++
		}
		format := source.Str(source.Child(t.Value, "format"))
		sig := source.Child(t.Value, "signature")
		switch {
		case format == "jwt" && sig == nil:
			c.add(t.Key, at, RuleToken, "%s is a JWT with no signature; name an algorithm of RFC 7518 under signature, one of %s", name, strings.Join(jwtAlgorithms, ", "))
		case format == "opaque" && sig != nil:
			c.add(sig, at+"/signature", RuleToken, "%s is opaque, a reference the service looks up, and has no signature; remove signature", name)
		case sig != nil && strings.EqualFold(source.Str(sig), "none"):
			c.add(sig, at+"/signature", RuleToken, "none is not accepted: RFC 8725 says a verifier must refuse the none algorithm, since a token with no signature is anyone's; name one of %s", strings.Join(jwtAlgorithms, ", "))
		case sig != nil && !contains(jwtAlgorithms, source.Str(sig)):
			c.add(sig, at+"/signature", RuleToken, "%s is not a signature algorithm RFC 7518 or RFC 8037 registers; use one of %s", source.Str(sig), strings.Join(jwtAlgorithms, ", "))
		}
		if source.Str(source.Child(t.Value, "default")) == "true" {
			defaults++
			if defaults > 1 {
				c.add(source.Child(t.Value, "default"), at+"/default", RuleToken, "%s is marked default while %s is already; one token is the default every operation accepts", name, d.defaultToken())
			}
		}
		claims := source.Child(t.Value, "claims")
		c.checkClaims(d, name, claims, at+"/claims")
		c.checkTokenChecks(d, name, t.Value, claims, at)
		kind := tokenType(t.Value)
		if kind == "txn_token" {
			if format != "jwt" {
				c.add(source.Child(t.Value, "format"), at+"/format", RuleToken, "%s is a transaction token, which is a JWT (the Transaction Tokens draft); set format: jwt", name)
			}
			for _, claim := range txnTokenClaims {
				if source.Child(claims, claim) == nil {
					c.add(t.Key, at, RuleToken, "%s is a transaction token and declares no %s; a transaction token declares the draft's claims %s", name, claim, strings.Join(txnTokenClaims, ", "))
				}
			}
			if aud := source.Child(claims, "aud"); aud != nil {
				c.add(source.Key(claims, "aud"), at+"/claims/aud", RuleToken, "aud is not written on a transaction token: its audience is the trust domain, one value for every hop (the draft, section 9.2); remove aud")
			}
			if accepted[name] && d.hasThrough() && source.Child(source.Child(source.Child(claims, "tctx"), "properties"), "application") == nil {
				c.add(t.Key, at, RuleToken, "%s is accepted where a role's through reaches, and declares no tctx.application, the application the person signed in through, immutable from the edge on; add tctx with properties.application", name)
			}
		} else if gw, ok := forwarded[name]; ok {
			c.checkForwardedAudience(name, gw, claims, at)
		} else if accepted[name] {
			c.checkTokenAudience(name, claims, at)
		}
	}
	if own > 0 && defaults == 0 {
		c.add(source.Key(d.root, "tokens"), "/tokens", RuleToken, "no token is marked default, so an operation naming no token accepts none; mark one default: true")
	}
	for _, o := range d.opList {
		if name := source.Str(source.Child(o.node, "token")); name != "" && d.token(name) == nil {
			c.add(source.Child(o.node, "token"), o.pointer("token"), RuleToken, "%s is not a token of the specification%s", name, suggest(name, d.tokenNames()))
		}
		c.checkVia(d, o)
		c.checkChain(d, o)
	}
	if len(tokens) > 0 && len(d.clients()) > 0 && source.Child(d.root, "securityEvents") == nil {
		c.add(source.Key(d.root, "tokens"), "/tokens", RuleSecurityEvents, "the specification declares tokens and clients, so every refusal of a caller is a security event, and declares no securityEvents; add securityEvents with store and alarm, two dependencies")
	}
	if se := source.Child(d.root, "securityEvents"); se != nil {
		for _, key := range []string{"store", "alarm"} {
			if dep := source.Str(source.Child(se, key)); dep != "" && d.dependencies[dep] == nil {
				c.add(source.Child(se, key), "/securityEvents/"+key, RuleSecurityEvents, "%s is not a dependency of the specification%s", dep, suggest(dep, d.dependencies))
			}
		}
	}
}

// checkClaims checks each claim: a registered claim is not retyped, an
// underscore name is a registered claim, and a claim of a caller
// property's name has its type.
func (c *checker) checkClaims(d *design, token string, claims *yaml.Node, at string) {
	callerProps := map[string]*yaml.Node{}
	for _, p := range source.Pairs(source.Child(source.Child(d.root, "caller"), "properties")) {
		callerProps[p.Key.Value] = p.Value
	}
	for _, cl := range source.Pairs(claims) {
		name := cl.Key.Value
		cat := at + "/" + name
		if r, ok := registeredClaims[name]; ok {
			if typ := source.Child(cl.Value, "type"); typ != nil && !typeIs(cl.Value, r.typ) {
				c.add(typ, cat+"/type", RuleClaim, "%s is a registered claim (%s) of type %s, and is written as %s; a registered claim carries the registry's type without being retyped; remove type", name, r.doc, r.typ, source.Str(typ))
			}
		} else if strings.Contains(name, "_") {
			c.add(cl.Key, cat, RuleClaim, "%s has an underscore and is not a claim of the JSON Web Token Claims registry that SpecArch knows; a claim of this specification's own is camelCase, as every property is", name)
		}
		if prop := callerProps[name]; prop != nil {
			ct, pt := d.claimType(name, cl.Value), d.fieldType(prop)
			if ct.Kind != pt.Kind {
				c.add(cl.Key, cat, RuleClaim, "the claim %s fills the caller's property %s, and their types differ (%s and %s); give them one type", name, name, ct, pt)
			}
		}
	}
}

// checkTokenChecks checks each check of a token: a bool expression over
// the token's claims.
func (c *checker) checkTokenChecks(d *design, token string, tok, claims *yaml.Node, at string) {
	env := expr.Env{}
	for _, cl := range source.Pairs(claims) {
		env[cl.Key.Value] = d.claimType(cl.Key.Value, cl.Value)
	}
	for _, ch := range source.Pairs(source.Child(tok, "checks")) {
		n := source.Child(ch.Value, "expression")
		if n == nil || !source.IsScalar(n) {
			continue
		}
		ptr := at + "/checks/" + ch.Key.Value + "/expression"
		tree, errs := expr.Parse(n.Value)
		if len(errs) > 0 {
			c.tokenErrors(n, ptr, errs)
			continue
		}
		t, errs := expr.Check(tree, env)
		c.tokenErrors(n, ptr, errs)
		if len(errs) == 0 && (t.Kind != expr.Bool || t.Nullable) {
			c.addFile(c.fileOf(n), exprLine(n, 1), ptr, RuleToken, "the check gives %s, but a check must give true or false; compare the claims with ==, <, > or similar", t)
		}
	}
}

// tokenErrors reports a check's expression errors: a name that is not a
// claim is the token rule, the rest the expression rules.
func (c *checker) tokenErrors(n *yaml.Node, ptr string, errs []expr.Error) {
	before := len(c.diags)
	c.exprErrors(n, ptr, "the check", errs)
	for i := before; i < len(c.diags); i++ {
		if c.diags[i].Rule == RuleExpressionName {
			c.diags[i].Rule = RuleToken
		}
	}
}

// checkAudience checks that an accepted access token fixes its audience
// with aud as const or enum, each value a URI when it has a colon.
func (c *checker) checkTokenAudience(token string, claims *yaml.Node, at string) {
	aud := source.Child(claims, "aud")
	cnst, enum := source.Child(aud, "const"), source.Child(aud, "enum")
	if aud == nil || (cnst == nil && enum == nil) {
		c.add(source.Key(claims, "aud"), at+"/claims/aud", RuleTokenAudience, "%s is accepted by an operation and fixes no audience; declare aud with const naming this service, or enum naming the few it stands for, so that a token minted for another service is refused (RFC 9068, section 4; RFC 8707)", token)
		return
	}
	var values []*yaml.Node
	if cnst != nil {
		values = append(values, cnst)
	}
	values = append(values, source.Items(enum)...)
	for _, v := range values {
		if s := source.Str(v); strings.Contains(s, ":") {
			if u, err := url.Parse(s); err != nil || u.Scheme == "" {
				c.add(v, at+"/claims/aud", RuleTokenAudience, "%s has a colon and is not a URI; an audience with a colon is a URI (RFC 7519, section 2), such as https://loans.example.org", s)
			}
		}
	}
}

// checkForwardedAudience refuses const or enum on the aud of a token a
// gateway forwards by aud: the forwards are its fixed audience, and the
// list written twice would be a disagreement to report instead of a fact
// (ADR-141).
func (c *checker) checkForwardedAudience(token, gateway string, claims *yaml.Node, at string) {
	aud := source.Child(claims, "aud")
	for _, key := range []string{"const", "enum"} {
		if n := source.Child(aud, key); n != nil {
			c.add(source.Key(aud, key), at+"/claims/aud/"+key, RuleTokenAudience, "%s is forwarded by aud by gateway %s, whose forwards are its fixed audience; remove %s, which says the list a second time", token, gateway, key)
		}
	}
}

// checkVia checks an operation's via: required where the operation accepts
// a transaction token or a token declaring act, refused elsewhere, and
// naming declared services with a workload-identity proof, once each.
func (c *checker) checkVia(d *design, o operation) {
	via := source.Child(o.node, "via")
	tokName := d.acceptedToken(o.node)
	tok := d.token(tokName)
	needs := tok != nil && (tokenType(tok) == "txn_token" || source.Child(source.Child(tok, "claims"), "act") != nil)
	at := o.pointer("via")
	switch {
	case via == nil && needs:
		c.add(source.Key(o.node, "operationId"), o.pointer(), RuleVia, "operation %s accepts %s, a transaction token or a token declaring act, and names no via; list its immediate callers, the services that call it on a person's behalf", o.id, tokName)
		return
	case via == nil:
		return
	case !needs:
		c.add(via, at, RuleVia, "operation %s accepts neither a transaction token nor a token declaring act, and via names the hops of a chain; remove via, or accept the chain's token", o.id)
	}
	items := source.Items(via)
	if len(items) == 0 {
		c.add(via, at, RuleVia, "via names no caller, which would make operation %s a route nobody reaches; list the services, or close it with permission: nobody", o.id)
	}
	seen := map[string]bool{}
	rv := newResolver(d, c.given)
	for i, item := range items {
		name := item.Value
		iat := at + "/" + itoa(i)
		if seen[name] {
			c.add(item, iat, RuleVia, "%s is listed twice; list it once", name)
			continue
		}
		seen[name] = true
		cl := d.client(name)
		if isCross(name) {
			// A client of the identity module: the reference check says
			// whether it is there, and it is checked once it is given.
			if _, cl = rv.referencedClient(name); cl == nil {
				continue
			}
		} else if cl == nil {
			c.add(item, iat, RuleVia, "%s is not a client of the specification%s", name, suggest(name, d.clientNames()))
			continue
		}
		if source.Str(source.Child(cl, "kind")) == "application" {
			c.add(item, iat, RuleVia, "%s is an application, and via names the services that carry a person's call; a person's own application calls the edge with an access token", name)
			continue
		}
		if kind := source.Str(source.Child(source.Child(cl, "proof"), "kind")); !workloadProofs[kind] {
			c.add(item, iat, RuleVia, "%s proves itself with %s, and a hop in a chain presents a workload identity: mutualTls, spiffe or wimse", name, orNone(kind))
		}
	}
}

func orNone(s string) string {
	if s == "" {
		return "nothing"
	}
	return s
}

// checkTokenProof checks a proof of kind token, spiffe or wimse.
func (c *checker) checkTokenProof(d *design, client string, proof *yaml.Node, at string) {
	kind := source.Str(source.Child(proof, "kind"))
	tok := source.Child(proof, "token")
	switch kind {
	case "token":
		if tok == nil {
			c.add(proof, at, RuleProof, "a token proof names the token format the client presents; add token, a name under tokens")
			return
		}
		t := d.token(source.Str(tok))
		switch {
		case t == nil:
			c.add(tok, at+"/token", RuleProof, "%s is not a token of the specification%s", source.Str(tok), suggest(source.Str(tok), d.tokenNames()))
		case tokenType(t) == "txn_token":
			c.add(tok, at+"/token", RuleProof, "%s is a transaction token, and a workload must not authenticate itself with one (the Transaction Tokens draft, section 13.12); name an access token", source.Str(tok))
		}
	case "spiffe", "wimse":
		if source.Child(proof, "algorithm") != nil || source.Child(proof, "covers") != nil || tok != nil {
			c.add(proof, at, RuleProof, "a %s proof is the workload's own identity and names no algorithm, covered components or token; remove them", kind)
		}
	default:
		if tok != nil {
			c.add(tok, at+"/token", RuleProof, "token goes with a proof of kind token; remove it, or set kind: token")
		}
	}
}

// tokenCases adds the cases of the token an operation accepts: expired,
// each check, the actor as the subject, a token for another audience, a
// caller not listed in via, and the confused deputy's token forms once a
// service exists (ADR-097, ADR-108, ADR-109).
func (d *design) tokenCases(s *subject, o operation, perm, call string) {
	name := d.acceptedToken(o.node)
	tok := d.token(name)
	if tok == nil {
		return
	}
	claims := source.Child(tok, "claims")
	if perm != "" && perm != permissionPublic && perm != permissionNobody {
		s.red("denied with expired "+name, frequent, "a caller whose "+name+" token has expired after "+source.Str(source.Child(tok, "lifetime")), call, "it is refused as not signed in"+d.event("token"))
	}
	for _, ch := range source.Pairs(source.Child(tok, "checks")) {
		s.red(ch.Key.Value+" fails on "+name, occasional, indefinite(name)+" "+name+" token for which "+source.Str(source.Child(ch.Value, "expression"))+" does not hold", call, "it is refused: "+source.Str(source.Child(ch.Value, "message"))+d.event("token"))
	}
	if source.Child(claims, "act") != nil {
		s.red("refused when the actor is the subject", rare, indefinite(name)+" "+name+" token whose act names its own sub", call, "it is refused as not allowed: a client may not act for itself (RFC 8693)"+d.event("deputy"))
	}
	txn := tokenType(tok) == "txn_token"
	if !txn {
		s.red("refused with a token for another audience", occasional, indefinite(name)+" "+name+" token minted for another service, whose aud does not name this one", call, "it is refused as not allowed"+d.event("token_audience"))
	}
	if source.Child(o.node, "via") != nil {
		s.red("refused with a caller not listed in via", rare, "the edge calling this operation directly with the transaction token, a caller via does not list", call, "it is refused as not allowed"+d.event("via"))
	}
	if d.hasService() && d.admitsPeople(o.node) {
		if txn {
			s.red("refused with a req_wl that is not an edge", rare, indefinite(name)+" "+name+" token whose req_wl names a later hop, which requested a transaction of its own", call, "it is refused as not allowed"+d.event("deputy"))
			s.red("refused with a txn differing from the one received", rare, indefinite(name)+" "+name+" token whose txn is not the one the hop received, minted anew instead of passed on unmodified", call, "it is refused as not allowed"+d.event("deputy"))
			s.red("refused with a scope no edge of its chain checked", rare, indefinite(name)+" "+name+" token whose scope is not the permission of an edge operation whose chain reaches this one, a transaction minted for another purpose", call, "it is refused as not allowed"+d.event("deputy"))
		} else {
			s.red("refused for a person's token requested by a service with no actor", rare, indefinite(name)+" "+name+" token whose client_id names a service and which declares no act, impersonation rather than delegation (RFC 8693, section 1.1)", call, "it is refused as not allowed"+d.event("deputy"))
		}
	}
}

// claimNames lists a token's claims, sorted.
func claimNames(tok *yaml.Node) []string {
	var out []string
	for _, cl := range source.Pairs(source.Child(tok, "claims")) {
		out = append(out, cl.Key.Value)
	}
	sort.Strings(out)
	return out
}
