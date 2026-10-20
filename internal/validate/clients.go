package validate

import (
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// Machine clients and the gates that are not a permission (meta-model
// 0.3, ADR-096, ADR-106, ADR-110): a client is a program that calls the
// system, an application where a person signs in or a service that calls
// on its own account with the roles it holds and a proof, or carries a
// person's call. An operation's or command's callers is the explicit
// subset of who may call it, people and clients by name; nobody is the
// reserved permission of a route closed on purpose; a role's through
// narrows a grant to the clients it holds through.

// The reserved words of callers and permissions.
const (
	callerPeople     = "people"
	permissionNobody = "nobody"
	permissionPublic = "public"
)

// The signature algorithms RFC 9421 registers.
var signatureAlgorithms = []string{"ecdsa-p256-sha256", "ecdsa-p384-sha384", "ed25519", "hmac-sha256", "rsa-pss-sha512", "rsa-v1_5-sha256"}

// The derived components of RFC 9421, section 2.2.
var derivedComponents = map[string]bool{"@method": true, "@target-uri": true, "@authority": true, "@scheme": true, "@request-target": true, "@path": true, "@query": true, "@query-param": true, "@status": true}

var (
	headerFieldName = regexp.MustCompile("^[a-z0-9!#$%&'*+.^_`|~-]+$")
	versionRange    = regexp.MustCompile(`^>=([0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?) <([0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?)$`)
)

// clients are the specification's clients, in document order.
func (d *design) clients() []source.Pair {
	return source.Pairs(source.Child(d.root, "clients"))
}

func (d *design) client(name string) *yaml.Node {
	return source.Child(source.Child(d.root, "clients"), name)
}

// clientNames are the clients' names, for a suggestion.
func (d *design) clientNames() map[string]*yaml.Node {
	return topMap(d.root, "clients")
}

// clientHolds is every permission a client holds through its roles.
func (d *design) clientHolds(client *yaml.Node) map[string]bool {
	out := map[string]bool{}
	for _, r := range source.Items(source.Child(client, "roles")) {
		for p := range d.roleHolds(d.roles[r.Value]) {
			out[p] = true
		}
	}
	return out
}

// hasService reports whether any client is a service.
func (d *design) hasService() bool {
	for _, c := range d.clients() {
		if source.Str(source.Child(c.Value, "kind")) == "service" {
			return true
		}
	}
	return false
}

// tokensDeclareClaim reports whether any declared token carries the claim.
func (d *design) tokensDeclareClaim(claim string) bool {
	for _, t := range source.Pairs(source.Child(d.root, "tokens")) {
		if source.Child(source.Child(t.Value, "claims"), claim) != nil {
			return true
		}
	}
	return false
}

// checkClients checks every client: its name is not a reserved word, its
// kind is application or service, its roles exist, a client with roles
// or a service has a proof, its proof is well formed, its version range
// parses and has a claim to be checked against, and a service with no
// roles that nothing names is reported as unused.
func (c *checker) checkClients(d *design) {
	for _, cl := range d.clients() {
		name := cl.Key.Value
		at := source.Pointer("clients", name)
		switch name {
		case callerPeople:
			c.add(cl.Key, at, RuleReservedName, "people is reserved for anyone signed in, in callers; give the client another name")
		case permissionNobody, permissionPublic:
			c.add(cl.Key, at, RuleReservedName, "%s is a reserved permission; give the client another name", name)
		}
		kind := source.Str(source.Child(cl.Value, "kind"))
		if kind != "" && kind != "application" && kind != "service" {
			c.add(source.Child(cl.Value, "kind"), at+"/kind", RuleClient, "%s is neither application nor service: an application is where a person signs in, a service a program that calls on its own account or carries a person's call", kind)
		}
		for i, r := range source.Items(source.Child(cl.Value, "roles")) {
			if d.roles[r.Value] == nil {
				c.add(r, source.Pointer("clients", name, "roles", itoa(i)), RuleClient, "%s is not a role of the specification%s", r.Value, suggest(r.Value, d.roles))
			}
		}
		proof := source.Child(cl.Value, "proof")
		roles := source.Child(cl.Value, "roles")
		switch {
		case roles != nil && proof == nil:
			c.add(cl.Key, at, RuleClient, "%s holds roles and has no proof, and a client that calls on its own account must prove itself on each request (RFC 6749, section 4.4); add proof, or take the roles away", name)
		case kind == "service" && proof == nil:
			c.add(cl.Key, at, RuleClient, "%s is a service with no proof; a service proves itself on each request with a signed request or its client certificate; add proof", name)
		}
		if proof != nil {
			c.checkProof(name, proof, at+"/proof")
		}
		if v := source.Child(cl.Value, "softwareVersion"); v != nil {
			if !versionRange.MatchString(source.Str(v)) {
				c.add(v, at+"/softwareVersion", RuleClient, "softwareVersion %s is not a range written >=<lower> <<upper>, each a Semantic Versioning 2.0.0 version, such as >=1.2.0 <2.0.0", source.Str(v))
			} else if !d.tokensDeclareClaim("softwareVersion") {
				c.add(v, at+"/softwareVersion", RuleSoftwareVersion, "softwareVersion on %s has nothing to be checked against: no token declares a softwareVersion claim, and the version is read at the edge from the claim the issuer fills from the registration; declare the claim on the token the edge accepts, or take softwareVersion away", name)
			}
		}
		if kind == "service" && roles == nil && !d.namedInVia(name) {
			c.warn(cl.Key, at, RuleClientUnused, "%s is a service with no roles that no operation names in via, so nothing reaches it; give it roles, name it in via, or remove it", name)
		}
	}
}

// namedInVia reports whether an operation names the client as an
// immediate caller.
func (d *design) namedInVia(name string) bool {
	for _, o := range d.opList {
		for _, v := range source.Items(source.Child(o.node, "via")) {
			if v.Value == name {
				return true
			}
		}
	}
	return false
}

// checkProof checks a signed request's algorithm and covered components,
// and that a client certificate names neither.
func (c *checker) checkProof(client string, proof *yaml.Node, at string) {
	kind := source.Str(source.Child(proof, "kind"))
	alg := source.Child(proof, "algorithm")
	covers := source.Child(proof, "covers")
	switch kind {
	case "signedRequest":
		if alg == nil {
			c.add(proof, at, RuleProof, "a signedRequest proof names the algorithm both sides agree on; add algorithm, one of %s", strings.Join(signatureAlgorithms, ", "))
		} else if !contains(signatureAlgorithms, source.Str(alg)) {
			c.add(alg, at+"/algorithm", RuleProof, "%s is not a signature algorithm RFC 9421 registers; use one of %s", source.Str(alg), strings.Join(signatureAlgorithms, ", "))
		}
		if len(source.Items(covers)) == 0 {
			c.add(proof, at, RuleProof, "a signed request covers at least one component of the request; list them under covers, such as [\"@method\", \"@path\", content-digest, date]")
		}
		for i, item := range source.Items(covers) {
			v := item.Value
			if !derivedComponents[v] && !headerFieldName.MatchString(v) {
				c.add(item, source.Pointer("clients", client, "proof", "covers", itoa(i)), RuleProof, "%s is not a component of a request that a signature covers: a derived component such as @method or @path, or a header field by its name in lower case", v)
			}
		}
	case "mutualTls":
		if alg != nil || covers != nil {
			c.add(proof, at, RuleProof, "a mutualTls proof is the client certificate of RFC 8705 and names no algorithm or covered components; remove them")
		}
	}
}

// checkThrough checks every role's through: each key is a permission the
// role grants, each list names declared clients and is not empty, and the
// specification carries the application a caller signed in through.
func (c *checker) checkThrough(d *design) {
	carries := d.tokensDeclareClaim("client_id") || d.tokensDeclareClaim("tctx")
	for _, r := range source.Pairs(source.Child(d.root, "roles")) {
		through := source.Child(r.Value, "through")
		if through == nil {
			continue
		}
		at := source.Pointer("roles", r.Key.Value, "through")
		if !carries {
			c.add(through, at, RuleThrough, "through needs the application the caller signed in through, which a session does not carry and no token declares: declare a token with the client_id claim, or a transaction token with tctx, or take through away")
		}
		holds := d.roleHolds(r.Value)
		for _, p := range source.Pairs(through) {
			perm := p.Key.Value
			pat := at + "/" + source.Pointer(perm)[1:]
			if !holds[perm] {
				c.add(p.Key, pat, RuleThrough, "%s is not a permission %s grants, directly or by implies, so through has no grant to narrow; add it to the role's permissions, or remove it here", perm, r.Key.Value)
			}
			items := source.Items(p.Value)
			if len(items) == 0 {
				c.add(p.Value, pat, RuleThrough, "through names no client for %s, which would close the grant; list the clients it holds through, or drop the permission from the role", perm)
			}
			for i, item := range items {
				if d.client(item.Value) == nil {
					c.add(item, pat+"/"+itoa(i), RuleThrough, "%s is not a client of the specification%s", item.Value, suggest(item.Value, d.clientNames()))
				}
			}
		}
	}
}

// checkCallers checks the callers of every operation and command: not
// empty, not on a closed route, each people or a declared client listed
// once, no service without roles, and every client holding a role that
// grants the permission.
func (c *checker) checkCallers(d *design) {
	for _, o := range d.opList {
		c.checkCallerList(d, o.node, o.pointer("callers"), "operation "+o.id)
	}
	for _, cmd := range source.Pairs(source.Child(d.root, "commands")) {
		c.checkCallerList(d, cmd.Value, source.Pointer("commands", cmd.Key.Value, "callers"), "command "+cmd.Key.Value)
	}
}

func (c *checker) checkCallerList(d *design, n *yaml.Node, at, who string) {
	callers := source.Child(n, "callers")
	if callers == nil {
		return
	}
	perm := source.Str(source.Child(n, "permission"))
	items := source.Items(callers)
	if len(items) == 0 {
		c.add(callers, at, RuleCallers, "callers names nobody, which would make %s a route nobody calls; list people or the clients, or close it with permission: nobody", who)
		return
	}
	if perm == permissionNobody {
		c.add(callers, at, RuleCallers, "%s is closed with permission nobody, so callers has nobody to list; remove callers", who)
	}
	seen := map[string]bool{}
	for i, item := range items {
		name := item.Value
		iat := at + "/" + itoa(i)
		if seen[name] {
			c.add(item, iat, RuleCallers, "%s is listed twice; list it once", name)
			continue
		}
		seen[name] = true
		if name == callerPeople {
			continue
		}
		cl := d.client(name)
		if cl == nil {
			c.add(item, iat, RuleCallers, "%s is neither people nor a client of the specification%s", name, suggest(name, d.clientNames()))
			continue
		}
		if source.Str(source.Child(cl, "kind")) == "service" && source.Child(cl, "roles") == nil {
			c.add(item, iat, RuleCallers, "%s is a service with no roles, which only carries a person's call and never calls on its own account; list people, whose calls it carries, or give it roles", name)
			continue
		}
		if perm != "" && perm != permissionPublic && perm != permissionNobody && !d.clientHolds(cl)[perm] {
			c.add(item, iat, RuleCallers, "%s holds no role that grants %s, so it could never call %s; add the permission to one of its roles, or take the client out of callers", name, perm, who)
		}
	}
}

// callerKinds says who an operation or command admits: people, and the
// clients listed.
func (d *design) admitsPeople(n *yaml.Node) bool {
	callers := source.Items(source.Child(n, "callers"))
	if len(callers) == 0 {
		return true
	}
	for _, item := range callers {
		if item.Value == callerPeople {
			return true
		}
	}
	return false
}

// callerCases adds the cases of who may call an operation or a command:
// a closed route refuses every caller; a route with callers refuses a
// person where people is absent and every caller not listed, and a
// signed request that does not verify from each client that signs; a
// route for people refuses a caller through a client its role's through
// does not list, a client at a version outside its admitted range, and,
// once a service exists, a service's own token in place of a person's
// (the confused deputy, ADR-108).
func (d *design) callerCases(s *subject, n *yaml.Node, perm, what string, atEdge bool) {
	if perm == permissionNobody {
		s.red("refused for every caller", frequent, "any caller, signed in or not, with any roles", what, "it is refused: the route is closed on purpose, with the problem the design declares")
		return
	}
	callers := source.Items(source.Child(n, "callers"))
	people := d.admitsPeople(n)
	if len(callers) > 0 {
		if !people {
			s.red("refused for a signed-in person", frequent, "a person signed in, whatever roles they hold", what, "it is refused: only the clients listed may call it")
		}
		s.red("refused for every caller not listed", occasional, "a client not listed in callers, whatever its roles grant", what, "it is refused as not allowed")
		for _, item := range callers {
			cl := d.client(item.Value)
			if cl == nil {
				continue
			}
			if proof := source.Child(cl, "proof"); source.Str(source.Child(proof, "kind")) == "signedRequest" {
				s.red("refused with a bad signature from "+item.Value, rare, item.Value+" calls with a signature that does not verify over "+joinAnd(itemValues(source.Child(proof, "covers"))), what, "it is refused as not signed, and nothing changes")
			}
			if atEdge {
				d.versionCase(s, item.Value, cl, what)
			}
		}
	}
	if !people {
		return
	}
	d.throughCases(s, perm, what)
	if !atEdge {
		return
	}
	for _, cl := range d.clients() {
		if source.Str(source.Child(cl.Value, "kind")) == "application" {
			d.versionCase(s, cl.Key.Value, cl.Value, what)
		}
	}
	if d.hasService() {
		s.red("refused for a service's own token", rare, "a service calling with its own proof or token and no person's token, so that the call carries the service and not the person", what, "it is refused as not allowed: the person, with their rows, roles and through, would be lost")
	}
}

// throughCases adds, for every role whose through narrows the permission,
// the case of a caller through a client it does not list.
func (d *design) throughCases(s *subject, perm, what string) {
	for _, r := range source.Pairs(source.Child(d.root, "roles")) {
		list := source.Child(source.Child(r.Value, "through"), perm)
		if list == nil {
			continue
		}
		s.red("refused through a client not listed for "+r.Key.Value, occasional, "a caller holding "+r.Key.Value+" who signed in through a client other than "+joinAnd(itemValues(list)), what, "it is refused as not allowed")
	}
}

// versionCase adds the case of a client with an admitted version range
// calling at a version outside it.
func (d *design) versionCase(s *subject, name string, cl *yaml.Node, what string) {
	if v := source.Str(source.Child(cl, "softwareVersion")); v != "" {
		s.red("refused with "+name+" at a version outside its admitted range", occasional, "a caller through "+name+" whose token carries a softwareVersion outside "+v, what, "it is refused with 403 and the problem the design declares")
	}
}
