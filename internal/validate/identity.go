package validate

import (
	"net/url"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
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
		if p.Key.Value == permissionPublic || p.Key.Value == permissionNobody || granted[p.Key.Value] {
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
