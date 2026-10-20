package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// Gateways (ADR-141): an intermediary, after RFC 9110's gateway, that
// verifies a bearer token, reads one claim and forwards every request no
// operation answers, as received, to the dependency the claim's value
// names. It names no permission; when the claim is aud, its forwards are
// the token's fixed audience.

// The dependency kinds a gateway cannot forward to: a store or a broker
// is no HTTP origin (ADR-134).
var notForwardable = map[string]bool{"database": true, "messageBroker": true, "fileStore": true}

func (d *design) gateways() []source.Pair { return source.Pairs(source.Child(d.root, "gateways")) }

// forwardedByAudience maps each token a gateway forwards by aud to the
// first such gateway: the forwards fix its audience, so it is the
// services' token and not this system's.
func (d *design) forwardedByAudience() map[string]string {
	out := map[string]string{}
	for _, g := range d.gateways() {
		tok := source.Str(source.Child(g.Value, "token"))
		if tok == "" || source.Str(source.Child(g.Value, "claim")) != "aud" || d.token(tok) == nil {
			continue
		}
		if _, ok := out[tok]; !ok {
			out[tok] = g.Key.Value
		}
	}
	return out
}

// forwardedDependencies are the dependencies a gateway forwards to, in
// order, each once.
func forwardedDependencies(g *yaml.Node) []string {
	var out []string
	for _, row := range source.Items(source.Child(g, "forwards")) {
		if dep := source.Str(source.Child(row, "dependency")); dep != "" && !contains(out, dep) {
			out = append(out, dep)
		}
	}
	return out
}

// checkGateways checks each gateway's token, claim, forwards, calls,
// middlewares and refusals, and that a token forwarded by aud is neither
// the default nor named by an operation.
func (c *checker) checkGateways(d *design) {
	declared := map[string]*yaml.Node{}
	for _, p := range source.Pairs(source.Child(d.root, "middlewares")) {
		declared[p.Key.Value] = p.Value
	}
	for _, g := range d.gateways() {
		name := g.Key.Value
		at := source.Pointer("gateways", name)
		tokNode := source.Child(g.Value, "token")
		tokName := source.Str(tokNode)
		tok := d.token(tokName)
		if tokNode != nil && tok == nil {
			c.add(tokNode, at+"/token", RuleGateway, "%s is not a token of the specification%s", tokName, suggest(tokName, d.tokenNames()))
		}
		claimNode := source.Child(g.Value, "claim")
		claim := source.Str(claimNode)
		if claimNode != nil && tok != nil && source.Child(source.Child(tok, "claims"), claim) == nil {
			c.add(claimNode, at+"/claim", RuleGateway, "%s does not declare the claim %s, so the gateway has no value to read; declare it under the token's claims, or read one it declares: %s", tokName, claim, orNone(strings.Join(claimNames(tok), ", ")))
		}
		forwards := source.Child(g.Value, "forwards")
		if forwards != nil && len(source.Items(forwards)) == 0 {
			c.add(forwards, at+"/forwards", RuleGateway, "gateway %s forwards to no service, so every request it takes is refused; add a row with the claim's value and the dependency it selects", name)
		}
		seen := map[string]bool{}
		forwarded := map[string]bool{}
		for i, row := range source.Items(forwards) {
			rat := at + "/forwards/" + fmt.Sprint(i)
			if v := source.Child(row, "value"); v != nil {
				if seen[v.Value] {
					c.add(v, rat+"/value", RuleGateway, "%s is written twice, and a request goes to one service; write each value once", v.Value)
				}
				seen[v.Value] = true
			}
			depNode := source.Child(row, "dependency")
			if depNode == nil {
				continue
			}
			forwarded[depNode.Value] = true
			dep, ok := d.dependencies[depNode.Value]
			if !ok {
				c.add(depNode, rat+"/dependency", RuleGateway, "%s is not a dependency of the specification; declare it under dependencies with its timeout, kind and protocol%s", depNode.Value, suggest(depNode.Value, d.dependencies))
				continue
			}
			if kind := source.Str(source.Child(dep, "kind")); notForwardable[kind] {
				c.add(depNode, rat+"/dependency", RuleGateway, "%s is a dependency of kind %s, which is no HTTP origin a request can be forwarded to; forward to a dependency of kind system or identityProvider", depNode.Value, kind)
			}
		}
		for i, e := range source.Items(source.Child(g.Value, "calls")) {
			dep := callDependency(e.Value)
			switch {
			case d.dependencies[dep] == nil:
				c.add(e, at+"/calls/"+fmt.Sprint(i), RuleDependency, "%s is not a dependency of the specification; declare it under dependencies with its timeout%s", dep, suggest(dep, d.dependencies))
			case forwarded[dep]:
				c.add(e, at+"/calls/"+fmt.Sprint(i), RuleGateway, "%s is forwarded to, and a forward is a call already; remove it from calls", dep)
			}
		}
		if list := source.Child(g.Value, "middlewares"); list == nil {
			if len(declared) > 0 {
				c.warn(g.Key, at, RuleMiddlewareMissing, "the specification declares its middlewares, and gateway %s names none; list the ones it runs, in order, under middlewares, or write middlewares: [] if it runs none", name)
			}
		} else {
			for i, item := range source.Items(list) {
				if declared[item.Value] == nil {
					c.add(item, at+"/middlewares/"+fmt.Sprint(i), RuleMiddleware, "%s is not a middleware of the specification%s", item.Value, suggest(item.Value, declared))
				}
			}
		}
		c.checkRefusals(d, source.Child(g.Value, "refusals"), at+"/refusals", "gateway "+name)
		if claim != "aud" || tok == nil {
			continue
		}
		if def := source.Child(tok, "default"); source.Str(def) == "true" {
			c.add(tokNode, at+"/token", RuleGateway, "%s is forwarded by aud, so it is the services' token, and it is marked default, which makes every operation of this specification accept it; remove default from %s", tokName, tokName)
		}
		for _, o := range d.opList {
			if n := source.Child(o.node, "token"); source.Str(n) == tokName {
				c.add(n, o.pointer("token"), RuleGateway, "%s is forwarded by aud by gateway %s, so it is the services' token, and operation %s accepts it; name a token of this system's own", tokName, name, o.id)
			}
		}
	}
}

// checkRefusals checks a list of refusals against the catalogue of
// problem types, as a middleware's are (ADR-131).
func (c *checker) checkRefusals(d *design, refusals *yaml.Node, at, what string) {
	errors := source.Child(d.root, "errors")
	for i, r := range source.Items(refusals) {
		status := source.Str(source.Child(r, "status"))
		rat := at + "/" + fmt.Sprint(i)
		p := source.Child(r, "problem")
		if p == nil {
			if len(source.Pairs(errors)) > 0 {
				c.add(r, rat, RuleProblem, "the specification declares its problem types under errors, so the %s refusal of %s names one under problem", status, what)
			}
			continue
		}
		t := source.Child(errors, p.Value)
		if t == nil {
			c.add(p, rat+"/problem", RuleProblem, "%s is not a problem type under errors%s", p.Value, suggestPairs(p.Value, errors))
			continue
		}
		if s := source.Str(source.Child(t, "status")); s != status {
			c.add(p, rat+"/problem", RuleProblem, "%s is a %s problem, and this is a %s refusal; name a problem type of status %s", p.Value, s, status, status)
		}
	}
}

// gatewaySubject is a gateway's tests: a request reaching each forwarded
// service, the token refused when missing, badly signed or expired, a
// claim naming no service or two, and each dependency failing or not
// answering in time.
func (d *design) gatewaySubject(p source.Pair) *subject {
	name := p.Key.Value
	s := &subject{kind: "gateway", label: "gateway " + name, node: p.Key, path: source.Pointer("gateways", name),
		yamlKey: "gateway: " + name, name: kebab(name)}
	tokName := source.Str(source.Child(p.Value, "token"))
	tok := d.token(tokName)
	claim := source.Str(source.Child(p.Value, "claim"))
	aToken := "a token"
	if tokName != "" {
		aToken = "a " + tokName + " token"
	}
	when := "a request no operation answers reaches the gateway " + name
	s.success = golden(aToken+" whose "+claim+" is a value of the gateway's forwards", when, "it reaches the service the value names as received")
	for _, row := range source.Items(source.Child(p.Value, "forwards")) {
		dep := source.Str(source.Child(row, "dependency"))
		s.cases = append(s.cases, derivedCase{name: "forwarded to " + dep, scenario: "golden", given: aToken + " whose " + claim + " is " + source.Str(source.Child(row, "value")),
			when: when, then: "it reaches " + dep + " as received", frequency: frequent})
	}
	s.red("refused with no token", frequent, "a request with no bearer token", when, "it is refused as not signed in and nothing is forwarded")
	s.red("refused with a bad signature", frequent, aToken+" not signed by the issuer's key", when, "it is refused as not signed in and nothing is forwarded")
	if tok != nil {
		s.red("denied with expired "+tokName, frequent, aToken+" that has expired after "+source.Str(source.Child(tok, "lifetime")), when, "it is refused as not signed in and nothing is forwarded")
	}
	s.red("refused with an unknown "+claim, occasional, aToken+" whose "+claim+" is no value of the gateway's forwards", when, "it is refused and nothing is forwarded")
	if claimSchema := source.Child(source.Child(tok, "claims"), claim); claim == "aud" || typeIs(claimSchema, "array") {
		s.red("refused with a "+claim+" naming two forwarded services", rare, aToken+" whose "+claim+" holds two values of the gateway's forwards", when, "it is refused and nothing is forwarded")
	}
	deps := forwardedDependencies(p.Value)
	for _, dep := range calledDependencies(p.Value) {
		if !contains(deps, dep) {
			deps = append(deps, dep)
		}
	}
	for _, dep := range deps {
		n := d.dependencies[dep]
		if n == nil {
			continue
		}
		s.byNature("dependency fails "+dep, dep+" answers with an error", when, "...")
		s.byNature("dependency times out "+dep, dep+" does not answer within "+source.Str(source.Child(n, "timeout")), when, "...")
	}
	return s
}
