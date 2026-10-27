package validate

import (
	"net/url"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// The identity module holds the estate's clients, roles and trust domain,
// and every other module names them by reference through a dependency of
// kind identityProvider: <dependency>#/clients/<name>,
// <dependency>#/roles/<name>, and a role there names the permission of the
// module that enforces it, <dependency>#/permissions/<name> (ADR-107).

// identityDependency is the dependency that names the identity module, of
// kind identityProvider and with a specification, the first by name; ""
// when there is none.
func (d *design) identityDependency() string {
	for _, name := range sortedKeys(d.dependencies) {
		dep := d.dependencies[name]
		if source.Str(source.Child(dep, "kind")) == "identityProvider" && source.Str(source.Child(dep, "specification")) != "" {
			return name
		}
	}
	return ""
}

// withIdentity gives the design the identity module given with it, so
// that its derived cases read the through of the identity module's roles
// and the security events and trust domain it holds (ADR-171).
func (d *design) withIdentity(given Given) *design {
	dep := d.identityDependency()
	if dep == "" || len(given) == 0 {
		return d
	}
	if o, problem := newResolver(d, given).otherOf(d.specificationOf(dep)); o != nil && problem == "" && o.name != SpecName(d.root) {
		d.identity = o
	}
	return d
}

// identityCaseWaits says whether a case a test covers is one the identity
// module derives while that module is not given: the through case of one
// of its roles, and the cases of one of its clients named in callers,
// which a run with the module checks, as a reference waits for the
// specification it names (ADR-107, ADR-171).
func (d *design) identityCaseWaits(name string) bool {
	dep := d.identityDependency()
	if dep == "" || d.identity != nil {
		return false
	}
	if strings.HasPrefix(name, "refused through a client not listed for ") && strings.HasSuffix(name, " of "+d.specificationOf(dep)) {
		return true
	}
	// The cases of a client of the identity module named in callers: its
	// signature, its version, and the deputy's cases once it is a service
	// (BUG-046).
	if ref, ok := strings.CutPrefix(name, "refused with a bad signature from "); ok {
		return strings.HasPrefix(ref, dep+"#")
	}
	if ref, ok := strings.CutSuffix(strings.TrimPrefix(name, "refused with "), " at a version outside its admitted range"); ok && strings.HasPrefix(name, "refused with ") {
		return strings.HasPrefix(ref, dep+"#")
	}
	return serviceCases[name] && !d.hasService() && d.namesIdentityCaller()
}

// serviceCases are the confused deputy's cases, derived once a service
// exists (ADR-108).
var serviceCases = map[string]bool{
	"refused for a service's own token":                                 true,
	"refused with a req_wl that is not an edge":                         true,
	"refused with a txn differing from the one received":                true,
	"refused with a scope no edge of its chain checked":                 true,
	"refused for a person's token requested by a service with no actor": true,
}

// identityClient is the client a reference names in the identity module
// given with the design, <dependency>#/clients/<name>; nil when the
// reference names another dependency or the module is not given (ADR-107,
// ADR-171).
func (d *design) identityClient(text string) *yaml.Node {
	dep, ptr, ok := crossRef(text)
	if !ok || d.identity == nil || dep != d.identityDependency() {
		return nil
	}
	tokens := pointerTokens(ptr)
	if len(tokens) != 2 || tokens[0] != "clients" {
		return nil
	}
	return d.identity.design.client(tokens[1])
}

// callerClient is the client an entry of callers names: one of the
// design's own, or one of the identity module's by reference.
func (d *design) callerClient(name string) *yaml.Node {
	if isCross(name) {
		return d.identityClient(name)
	}
	return d.client(name)
}

// namesIdentityCaller reports whether an operation names a client of the
// identity module in callers.
func (d *design) namesIdentityCaller() bool {
	dep := d.identityDependency()
	for _, o := range d.opList {
		for _, c := range source.Items(source.Child(o.node, "callers")) {
			if from, _, ok := crossRef(c.Value); ok && from == dep {
				return true
			}
		}
	}
	return false
}

// trustDomain is the trust domain of the design, or of the identity
// module given with it; "" when neither says one.
func (d *design) trustDomain() string {
	if t := source.Str(source.Child(d.root, "trustDomain")); t != "" || d.identity == nil {
		return t
	}
	return source.Str(source.Child(d.identity.design.root, "trustDomain"))
}

// specificationOf is the specification a dependency names, or "".
func (d *design) specificationOf(dep string) string {
	return source.Str(source.Child(d.dependencies[dep], "specification"))
}

// element is the element a reference names in a specification given with
// this one, with that specification; nil when the reference is not one,
// the specification is not given, or it holds no such element.
func (rv *resolver) element(text string) (*other, *yaml.Node) {
	dep, ptr, ok := crossRef(text)
	if !ok {
		return nil, nil
	}
	name := rv.d.specificationOf(dep)
	if name == "" {
		return nil, nil
	}
	o, problem := rv.otherOf(name)
	if o == nil || problem != "" {
		return nil, nil
	}
	tokens := pointerTokens(ptr)
	if len(tokens) != 2 {
		return nil, nil
	}
	return o, source.Child(source.Child(o.design.root, tokens[0]), tokens[1])
}

// grants reports whether one of the roles of the other specification
// grants the permission perm of the module named module, by a reference
// through one of its dependencies.
func (o *other) grants(roles []string, module, perm string) bool {
	for _, r := range roles {
		for p := range o.design.roleHolds(o.design.roles[r]) {
			dep, ptr, ok := crossRef(p)
			if ok && o.design.specificationOf(dep) == module && ptr == "/permissions/"+perm {
				return true
			}
		}
	}
	return false
}

// grantedHere is every permission a role of the specification grants,
// directly or by implies.
func (d *design) grantedHere() map[string]bool {
	out := map[string]bool{}
	for _, r := range d.roles {
		for p := range d.roleHolds(r) {
			out[p] = true
		}
	}
	return out
}

// identityGrant says whether a permission no role of the specification
// grants is granted by a role of the identity module: granted, or why it
// cannot be told (the module is not given), or neither, when there is no
// identity module or it grants nothing of the kind.
func (rv *resolver) identityGrant(perm string) (granted bool, waits string, identity string) {
	dep := rv.d.identityDependency()
	if dep == "" {
		return false, "", ""
	}
	name := rv.d.specificationOf(dep)
	o, problem := rv.otherOf(name)
	if problem != "" {
		return false, "", name
	}
	if o == nil {
		return false, name + " is not given, so the role that grants " + perm + " cannot be found", name
	}
	return o.grants(sortedKeys(o.design.roles), SpecName(rv.d.root), perm), "", name
}

// ungrantedWaiting are the permissions no role of the specification grants
// whose grant waits for the identity module, sorted: each is counted as an
// unresolved reference.
func ungrantedWaiting(d *design, rv *resolver) []source.Pair {
	var out []source.Pair
	granted := d.grantedHere()
	for _, p := range source.Pairs(source.Child(d.root, "permissions")) {
		if p.Key.Value == permissionPublic || p.Key.Value == permissionNobody || p.Key.Value == permissionSignedIn || granted[p.Key.Value] {
			continue
		}
		if _, waits, _ := rv.identityGrant(p.Key.Value); waits != "" {
			out = append(out, p)
		}
	}
	return out
}

// referencedClient is the client a cross reference in via or callers names
// when the identity module is given and holds it, with that module.
func (rv *resolver) referencedClient(text string) (*other, *yaml.Node) {
	_, ptr, ok := crossRef(text)
	if !ok || !strings.HasPrefix(ptr, "/clients/") {
		return nil, nil
	}
	return rv.element(text)
}

// namedInViaElsewhere reports whether a specification given with this one
// names the client in an operation's via, through a dependency on this
// specification.
func (c *checker) namedInViaElsewhere(d *design, client string) bool {
	module := SpecName(d.root)
	if module == "" {
		return false
	}
	for _, list := range c.given {
		for _, s := range list {
			if s.Root == nil || SpecName(s.Root) == module {
				continue
			}
			od := newDesign(s.Root)
			for _, o := range od.opList {
				for _, v := range source.Items(source.Child(o.node, "via")) {
					dep, ptr, ok := crossRef(v.Value)
					if ok && od.specificationOf(dep) == module && ptr == "/clients/"+client {
						return true
					}
				}
			}
		}
	}
	return false
}

// checkTrustDomain checks that the trust domain is a URI.
func (c *checker) checkTrustDomain(d *design) {
	n := source.Child(d.root, "trustDomain")
	if n == nil {
		return
	}
	if u, err := url.Parse(source.Str(n)); err != nil || u.Scheme == "" || u.Host == "" {
		c.add(n, "/trustDomain", RuleTokenAudience, "trustDomain %s is not a URI; it is the aud of every transaction token and the base of every security event's identifier, such as https://estate.example.org", source.Str(n))
	}
}

// The chain of a call is found by induction across the modules given
// together (ADR-111): a client named in via is the module whose info.name
// is the client's name, and that module's operations that call this one
// through a dependency on it are the hop before. Each must accept a
// transaction token, with a via of its own, or be the edge, accepting an
// access token; and every module that calls this operation must be named
// in its via.

// clientKey is the name of the client a via entry names: the last part of
// a reference, or the entry itself.
func clientKey(entry string) string {
	if _, ptr, ok := crossRef(entry); ok {
		return strings.TrimPrefix(ptr, "/clients/")
	}
	return entry
}

// callersOf are the operations of a design that call operation o of the
// module named module, through a dependency on it, in document order.
func callersOf(od *design, module string, o operation) []operation {
	var out []operation
	for _, x := range od.opList {
		for _, call := range callEntries(x.node) {
			e := call.ref
			dep, ptr, cross := crossRef(e.Value)
			if !cross {
				dep = e.Value
			}
			if od.specificationOf(dep) != module {
				continue
			}
			if tokens := pointerTokens(ptr); cross && (len(tokens) != 3 || tokens[0] != "paths" || tokens[1] != o.path || tokens[2] != o.method) {
				continue
			}
			out = append(out, x)
			break
		}
	}
	return out
}

// givenOne is the one specification given under a name, read, or nil.
func (c *checker) givenOne(name string) *design {
	list := c.given[name]
	if len(list) != 1 || list[0].Root == nil || list[0].Root.Kind != yaml.MappingNode {
		return nil
	}
	return newDesign(list[0].Root)
}

// checkChain checks an operation's via against the modules given with
// this one.
func (c *checker) checkChain(d *design, o operation) {
	module := SpecName(d.root)
	via := source.Child(o.node, "via")
	if module == "" || via == nil {
		return
	}
	named := map[string]bool{}
	for i, item := range source.Items(via) {
		key := clientKey(item.Value)
		named[key] = true
		cd := c.givenOne(key)
		if cd == nil || key == module {
			continue
		}
		callers := callersOf(cd, module, o)
		if len(callers) == 0 {
			c.add(item, o.pointer("via", itoa(i)), RuleVia, "%s names %s, and no operation of the module %s calls %s %s, so the chain has no hop before this one; call it from %s, or take %s out of via", o.id, item.Value, key, strings.ToUpper(o.method), o.path, key, item.Value)
			continue
		}
		for _, x := range callers {
			tok := cd.token(cd.acceptedToken(x.node))
			if tok == nil {
				c.add(item, o.pointer("via", itoa(i)), RuleVia, "operation %s of %s calls %s and accepts no token, so it has no person's call to carry on; accept the transaction token, with a via of its own, or an access token, as the edge", x.id, key, o.id)
			}
		}
	}
	others := make([]string, 0, len(c.given))
	for name := range c.given {
		others = append(others, name)
	}
	sort.Strings(others)
	for _, name := range others {
		if name == module || named[name] {
			continue
		}
		od := c.givenOne(name)
		if od == nil {
			continue
		}
		if callers := callersOf(od, module, o); len(callers) > 0 {
			c.add(via, o.pointer("via"), RuleVia, "operation %s of %s calls %s, and via does not name %s, so the hop that calls it is not in the chain; add %s to via, or stop the call", callers[0].id, name, o.id, name, viaEntry(d, o, name))
		}
	}
}

// viaEntry is how via would name a client: through the dependency its
// other entries go through, or by name.
func viaEntry(d *design, o operation, client string) string {
	for _, item := range source.Items(source.Child(o.node, "via")) {
		if dep, _, ok := crossRef(item.Value); ok {
			return dep + "#/clients/" + client
		}
	}
	if dep := d.identityDependency(); dep != "" {
		return dep + "#/clients/" + client
	}
	return client
}

// acceptedAudiences are the aud values of the access tokens an operation
// of the design accepts, each once, sorted.
func (d *design) acceptedAudiences() []string {
	seen := map[string]bool{}
	for name := range d.acceptedTokens() {
		tok := d.token(name)
		if tokenType(tok) != "access_token" {
			continue
		}
		aud := source.Child(source.Child(tok, "claims"), "aud")
		if v := source.Child(aud, "const"); source.IsScalar(v) {
			seen[v.Value] = true
		}
		for _, v := range source.Items(source.Child(aud, "enum")) {
			seen[v.Value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// A Hop is one operation of a delegation chain: the module and the
// operation, the token it accepts, and the proof kind its module presents
// to the next hop as the client the next hop's via names ("" for the
// last).
type Hop struct {
	Module, Operation, Token, Proof string
}

// A Chain is one way a call reaches an operation, from the edge on, as
// the induction through calls finds it across the modules given together.
type Chain struct {
	Operation string
	Hops      []Hop
}

// Chains are the chains that reach each operation of a specification
// with a via, found across the specifications given with it; nil when no
// module that a via names is given (ADR-111).
func Chains(s *spec.Spec, given Given) []Chain {
	if s.Root == nil || s.Root.Kind != yaml.MappingNode {
		return nil
	}
	c := &checker{given: given}
	d := newDesign(s.Root)
	module := SpecName(d.root)
	if module == "" {
		return nil
	}
	var out []Chain
	found := false
	for _, o := range d.opList {
		if source.Child(o.node, "via") == nil {
			continue
		}
		paths := c.back(d, module, o, map[string]bool{})
		for _, p := range paths {
			if len(p) > 1 {
				found = true
			}
			out = append(out, Chain{Operation: o.id, Hops: p})
		}
	}
	if !found {
		return nil
	}
	return out
}

// back is every path of hops that ends at operation o of module, the
// edge first; a caller module not given ends the path at o.
func (c *checker) back(d *design, module string, o operation, seen map[string]bool) [][]Hop {
	here := Hop{Module: module, Operation: o.id, Token: d.acceptedToken(o.node)}
	key := module + " " + o.id
	via := source.Items(source.Child(o.node, "via"))
	if seen[key] || len(via) == 0 {
		return [][]Hop{{here}}
	}
	seen[key] = true
	defer delete(seen, key)
	rv := newResolver(d, c.given)
	var out [][]Hop
	for _, item := range via {
		name := clientKey(item.Value)
		cd := c.givenOne(name)
		if cd == nil || name == module {
			continue
		}
		cl := d.client(item.Value)
		if isCross(item.Value) {
			_, cl = rv.referencedClient(item.Value)
		}
		proof := source.Str(source.Child(source.Child(cl, "proof"), "kind"))
		for _, x := range callersOf(cd, module, o) {
			for _, p := range c.back(cd, name, x, seen) {
				p[len(p)-1].Proof = proof
				out = append(out, append(p, here))
			}
		}
	}
	if len(out) == 0 {
		return [][]Hop{{here}}
	}
	return out
}

// A RoleColumn is one holder of a module's permissions in the identity
// module: a role, or a client with roles named with its kind, and the
// cell for each permission it reaches, yes or the clients a through
// narrows it to.
type RoleColumn struct {
	Name  string
	Cells map[string]string
}

// ReferencedGrants are the roles and clients with roles of the identity
// module that reach a permission of this specification, in the identity
// module's order; nil when the identity module is not given (ADR-107).
func ReferencedGrants(s *spec.Spec, given Given) []RoleColumn {
	if s.Root == nil || s.Root.Kind != yaml.MappingNode {
		return nil
	}
	d := newDesign(s.Root)
	module := SpecName(d.root)
	dep := d.identityDependency()
	if module == "" || dep == "" {
		return nil
	}
	o, problem := newResolver(d, given).otherOf(d.specificationOf(dep))
	if o == nil || problem != "" {
		return nil
	}
	cellsOf := func(role *yaml.Node) map[string]string {
		cells := map[string]string{}
		var holds []string
		for held := range o.design.roleHolds(role) {
			holds = append(holds, held)
		}
		sort.Strings(holds)
		for _, p := range source.Pairs(source.Child(d.root, "permissions")) {
			for _, held := range holds {
				hdep, ptr, ok := crossRef(held)
				if !ok || o.design.specificationOf(hdep) != module || ptr != "/permissions/"+p.Key.Value {
					continue
				}
				cells[p.Key.Value] = "yes"
				if list := itemValues(source.Child(source.Child(role, "through"), held)); len(list) > 0 {
					cells[p.Key.Value] = strings.Join(list, ", ") + " only"
				}
			}
		}
		return cells
	}
	var out []RoleColumn
	for _, r := range source.Pairs(source.Child(o.design.root, "roles")) {
		if cells := cellsOf(r.Value); len(cells) > 0 {
			out = append(out, RoleColumn{Name: o.name + " " + r.Key.Value, Cells: cells})
		}
	}
	for _, cl := range source.Pairs(source.Child(o.design.root, "clients")) {
		cells := map[string]string{}
		for _, r := range source.Items(source.Child(cl.Value, "roles")) {
			for p := range cellsOf(o.design.roles[r.Value]) {
				cells[p] = "yes"
			}
		}
		if len(cells) > 0 {
			out = append(out, RoleColumn{Name: o.name + " " + cl.Key.Value + " (" + source.Str(source.Child(cl.Value, "kind")) + ")", Cells: cells})
		}
	}
	return out
}
