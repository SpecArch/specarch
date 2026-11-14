package validate

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// The design security rules (ADR-247): weaknesses a specification shows
// before any code is written. Each is a warning at the element to change,
// so the specification is still read whole, and each rule's name is
// stable, since a tool that traces a finding into the code cites it.

// SecurityRules are the design security rules, in the order of the
// design's Rule enum. The problems file's SARIF tags each with security.
var SecurityRules = []Rule{
	RuleObjectUnscoped,
	RuleInternalKeyExposed,
	RuleRefusalRevealsRecord,
	RuleQueryFromCaller,
	RuleDutiesUnseparated,
	RuleSecretUnprotected,
	RuleAuditNotAppendOnly,
}

// The values of a requirement's controls.
const (
	controlSeparationOfDuties = "separationOfDuties"
	controlAuditTrail         = "auditTrail"
)

// everyoneHolds says whether every caller holds a permission: public, and
// signedIn once signed in, which no role grants.
func everyoneHolds(perm string) bool {
	return perm == "public" || perm == permissionSignedIn
}

// checkSecurity runs the design security rules.
func (c *checker) checkSecurity(d *design) {
	keys := d.internalKeys()
	seen := map[string]bool{}
	for _, o := range d.opList {
		c.checkObjectScope(d, o)
		c.checkInternalKeys(d, o, keys, seen)
		c.checkCallerQuery(o, seen)
	}
	c.checkDutiesSeparated(d)
	c.checkSecrets(d)
	c.checkAuditTrail(d)
}

// param is one parameter of an operation, the path item's or its own,
// with its pointer.
type param struct {
	node *yaml.Node
	ptr  string
}

// params lists an operation's parameters: the path item's first, then
// its own.
func (o operation) params() []param {
	var out []param
	for i, p := range source.Items(source.Child(o.pathItem, "parameters")) {
		out = append(out, param{p, source.Pointer("paths", o.path, "parameters", fmt.Sprint(i))})
	}
	for i, p := range source.Items(source.Child(o.node, "parameters")) {
		out = append(out, param{p, o.pointer("parameters", fmt.Sprint(i))})
	}
	return out
}

// addressedEntity is the entity an operation reaches one record of by its
// key in the path: the one a path parameter <entity>Id is the key of, or,
// for a path parameter named id, the entity the operation reads or
// writes; "" for an operation with no such parameter.
func (d *design) addressedEntity(o operation) string {
	if e := d.keyEntity(o); e != "" {
		return e
	}
	for _, p := range o.params() {
		if source.Str(source.Child(p.node, "in")) == "path" && keyName(source.Str(source.Child(p.node, "name"))) == "id" {
			return d.rowsEntity(o)
		}
	}
	return ""
}

// checkObjectScope warns about an operation that reaches one record by
// its key with a permission every caller holds and no row policy on the
// entity (object_unscoped), and about a 403 on such an operation, which
// can only refuse that record (refusal_reveals_record). A declared 404 is
// not asked for: a key no record has, or one outside a row policy, is
// derived as refused as not found whether or not the operation lists it.
func (c *checker) checkObjectScope(d *design, o operation) {
	entity := d.addressedEntity(o)
	if entity == "" || d.entities[entity] == nil {
		return
	}
	permNode := source.Child(o.node, "permission")
	perm := source.Str(permNode)
	held := "every caller once signed in"
	if perm == "public" {
		held = "every caller, signed in or not"
	}
	// A public read is open to everyone on purpose; what it shows is
	// sensitivity_exposed's to judge.
	if everyoneHolds(perm) && !(perm == "public" && o.method == "get") && len(d.rowsOf(entity)) == 0 {
		c.warn(permNode, o.pointer("permission"), RuleObjectUnscoped,
			"%s reaches one %s by its key, and its permission %s is held by %s, with no row policy on %s, so any caller reaches any %s by changing the key; give the operation a permission a role grants, or give the roles a row policy on %s that compares a field of it with caller.<property>",
			o.id, entity, perm, held, entity, entity, entity)
	}
	responses := source.Child(o.node, "responses")
	if everyoneHolds(perm) {
		if r := source.Key(responses, "403"); r != nil {
			c.warn(r, o.pointer("responses", "403"), RuleRefusalRevealsRecord,
				"%s reaches one %s by its key, and its permission %s is held by %s, so a 403 can only refuse that record, which tells the caller that a %s with that key exists; answer 404, as for a key no record has",
				o.id, entity, perm, held, entity)
		}
	}
}

// isWholeNumber says whether a field holds a whole number: an integer, or
// a string of format int64 or uint64.
func isWholeNumber(f *yaml.Node) bool {
	f = source.Deref(f)
	if f == nil {
		return false
	}
	if carriesAs(f, "integer") {
		return true
	}
	format := source.Str(source.Child(f, "format"))
	return carriesAs(f, "string") && (format == "int64" || format == "uint64")
}

// identifierName says whether a name names an identifier: id, or a name
// that ends in Id, ID or _id.
func identifierName(name string) bool {
	return name == "id" || name == "ID" || strings.HasSuffix(name, "Id") || strings.HasSuffix(name, "ID") || strings.HasSuffix(name, "_id")
}

// internalKeys maps each entity field that is a whole-number primary key
// to its name, Entity.field.
func (d *design) internalKeys() map[*yaml.Node]string {
	out := map[*yaml.Node]string{}
	for name, e := range d.entities {
		fields := fieldsOf(e)
		for _, k := range source.Items(source.Child(e, "primaryKey")) {
			if f := fields[k.Value]; f != nil && isWholeNumber(f) {
				out[f] = name + "." + k.Value
			}
		}
	}
	return out
}

// checkInternalKeys warns about a whole-number key in an operation's URL,
// request body or response (internal_key_exposed): an entity's
// whole-number primary key, or a whole number named as an identifier.
func (c *checker) checkInternalKeys(d *design, o operation, keys map[*yaml.Node]string, seen map[string]bool) {
	for _, p := range o.params() {
		name := source.Str(source.Child(p.node, "name"))
		in := source.Str(source.Child(p.node, "in"))
		if (in != "path" && in != "query") || !identifierName(name) || !isWholeNumber(source.Child(p.node, "schema")) {
			continue
		}
		if at := string(RuleInternalKeyExposed) + p.ptr; !seen[at] {
			seen[at] = true
			c.warn(p.node, p.ptr, RuleInternalKeyExposed,
				"%s carries %s in the URL as a whole number, which a caller counts up or down to reach records nobody gave it; address the record by a public identifier instead, a field of format uuid with generatedDefault: identifier, and keep the number inside the system",
				o.id, name)
		}
	}
	carried := func(contents *yaml.Node) []string {
		set := map[string]bool{}
		for _, ct := range source.Pairs(contents) {
			for _, f := range d.responseFields(source.Child(ct.Value, "schema")) {
				last := f.name[strings.LastIndex(f.name, ".")+1:]
				if keys[f.node] != "" || identifierName(last) && isWholeNumber(f.node) {
					set[f.name] = true
				}
			}
		}
		var out []string
		for n := range set {
			out = append(out, n)
		}
		sort.Strings(out)
		return out
	}
	advice := "a whole number a caller counts up or down to reach records nobody gave it; carry a public identifier instead, a field of format uuid with generatedDefault: identifier, and keep the number inside the system"
	if names := carried(source.Child(source.Child(o.node, "requestBody"), "content")); len(names) > 0 {
		c.warn(source.Key(o.node, "requestBody"), o.pointer("requestBody"), RuleInternalKeyExposed,
			"the request body of %s carries %s, %s", o.id, joinAnd(names), advice)
	}
	for _, r := range source.Pairs(source.Child(o.node, "responses")) {
		if names := carried(source.Child(r.Value, "content")); len(names) > 0 {
			c.warn(r.Key, o.pointer("responses", r.Key.Value), RuleInternalKeyExposed,
				"the %s response of %s carries %s, %s", r.Key.Value, o.id, joinAnd(names), advice)
		}
	}
}

// The names of a parameter or body property that takes part of a query
// from the caller, compared with case, underscores and hyphens ignored:
// the field to sort by, a filter or the fields to return, and a query
// itself. Free text to search for is a value, bound as one, and is not
// among them.
var (
	sortNames   = map[string]bool{"sort": true, "sortby": true, "sortfield": true, "sortorder": true, "order": true, "orderby": true, "ordering": true}
	filterNames = map[string]bool{"filter": true, "filters": true, "where": true, "condition": true, "conditions": true, "criteria": true, "having": true, "groupby": true, "select": true, "fields": true, "columns": true}
	rawNames    = map[string]bool{"sql": true, "rawsql": true, "rawquery": true, "sqlquery": true, "statement": true}
)

// closedSchema says whether a schema allows only values the
// specification lists: an enum or a const, a $ref to a declared shape, a
// boolean or a number, an array of such, or an object with its properties
// and additionalProperties false.
func closedSchema(s *yaml.Node) bool {
	s = source.Deref(s)
	if s == nil {
		return false
	}
	if source.Child(s, "enum") != nil || source.Child(s, "const") != nil || source.Str(source.Child(s, "$ref")) != "" {
		return true
	}
	if items := source.Child(s, "items"); items != nil {
		return closedSchema(items)
	}
	if carriesAs(s, "boolean") || carriesAs(s, "integer") || carriesAs(s, "number") {
		return true
	}
	if carriesAs(s, "object") {
		return len(source.Pairs(source.Child(s, "properties"))) > 0 && source.Str(source.Child(s, "additionalProperties")) == "false"
	}
	return false
}

// checkCallerQuery warns about a parameter that takes the field to sort
// by, a filter, the fields to return or a query from the caller with no
// list of the values it allows, and about a request body property that
// takes a query (query_from_caller). A body property named order,
// condition or fields is as often a value of the record (a purchase
// order, the condition of a copy) as part of a query, so only a query's
// own names are read in a body.
func (c *checker) checkCallerQuery(o operation, seen map[string]bool) {
	check := func(n *yaml.Node, ptr, name string, schema *yaml.Node, where string, rawOnly bool) {
		k := keyName(name)
		var advice string
		switch {
		case rawOnly && !rawNames[k]:
			return
		case rawNames[k]:
			advice = "a query the caller writes runs with the service's rights over every table; name what a caller may ask for as operations of their own, each with its parameters"
		case sortNames[k]:
			advice = "the query is built from a field name the caller chose; list the fields under listOf.sortable, which gives the parameter its enum, or give it an enum"
		case filterNames[k]:
			advice = "the query is built from field names and conditions the caller chose; list the fields under listOf.filterable, or give it a closed schema, an object with its properties and additionalProperties: false"
		default:
			return
		}
		if closedSchema(schema) {
			return
		}
		if at := string(RuleQueryFromCaller) + ptr; !seen[at] {
			seen[at] = true
			c.warn(n, ptr, RuleQueryFromCaller, "%s takes %s %s from the caller with no list of the values it allows, so %s", o.id, where, name, advice)
		}
	}
	for _, p := range o.params() {
		check(p.node, p.ptr, source.Str(source.Child(p.node, "name")), source.Child(p.node, "schema"), "the parameter", false)
	}
	for _, ct := range source.Pairs(source.Child(source.Child(o.node, "requestBody"), "content")) {
		schema := source.Child(ct.Value, "schema")
		for _, p := range source.Pairs(source.Child(schema, "properties")) {
			check(p.Key, o.pointer("requestBody", "content", ct.Key.Value, "schema", "properties", p.Key.Value), p.Key.Value, p.Value, "the request body property", true)
		}
	}
}

// satisfiedBy says whether an element lists a requirement under
// satisfies.
func satisfiedBy(n *yaml.Node, id string) bool {
	for _, s := range source.Items(source.Child(n, "satisfies")) {
		if s.Value == id {
			return true
		}
	}
	return false
}

// control is a requirement's controls entry: the requirement's ID, the
// entry and its pointer.
type control struct {
	id   string
	node *yaml.Node
	ptr  string
}

// controlsOf lists the controls entries asking for a control, by
// requirement ID.
func (d *design) controlsOf(name string) []control {
	var out []control
	for _, id := range sortedKeys(d.requirements) {
		for i, ctl := range source.Items(source.Child(d.requirements[id], "controls")) {
			if ctl.Value == name {
				out = append(out, control{id, ctl, source.Pointer("requirements", id, "controls", fmt.Sprint(i))})
			}
		}
	}
	return out
}

// checkDutiesSeparated warns about a requirement asking for separation of
// duties that no set satisfies, and a permission that satisfies it and no
// set satisfying it names (duties_unseparated). A workflow's approval is
// left alone: its derived case already refuses a requester who approves
// their own request.
func (c *checker) checkDutiesSeparated(d *design) {
	sets := source.Pairs(source.Child(d.root, "separationOfDuties"))
	for _, ctl := range d.controlsOf(controlSeparationOfDuties) {
		id := ctl.id
		named := map[string]bool{}
		found := false
		for _, s := range sets {
			if satisfiedBy(s.Value, id) {
				found = true
				for _, p := range source.Items(source.Child(s.Value, "permissions")) {
					named[p.Value] = true
				}
			}
		}
		if !found {
			c.warn(ctl.node, ctl.ptr, RuleDutiesUnseparated,
				"%s asks for separation of duties, and no set under separationOfDuties satisfies it, so nothing keeps one person from holding the permissions it would separate; add a set naming them, with satisfies: [%s]", id, id)
			continue
		}
		for _, p := range source.Pairs(source.Child(d.root, "permissions")) {
			if satisfiedBy(p.Value, id) && !named[p.Key.Value] {
				c.warn(p.Key, source.Pointer("permissions", p.Key.Value), RuleDutiesUnseparated,
					"%s satisfies %s, which asks for separation of duties, and no set satisfying %s names it, so a role may grant it beside the permissions it should be kept from; name it in the set", p.Key.Value, id, id)
			}
		}
	}
}

// credentialNames are the endings of a field name that name a secret,
// compared with case, underscores and hyphens ignored.
var credentialNames = []string{"password", "passphrase", "secret", "apikey", "privatekey", "accesstoken", "refreshtoken"}

func credentialName(name string) bool {
	k := keyName(name)
	for _, end := range credentialNames {
		if strings.HasSuffix(k, end) {
			return true
		}
	}
	return false
}

// checkSecrets warns about an entity's credential field, or a field named
// as one with no sensitivity, kept with no atRest, and about a secret
// setting with no rotation (secret_unprotected).
func (c *checker) checkSecrets(d *design) {
	for _, e := range source.Pairs(source.Child(d.root, "entities")) {
		for _, f := range source.Pairs(source.Child(e.Value, "properties")) {
			sens := source.Str(source.Child(f.Value, "sensitivity"))
			if sens != "credential" && (sens != "" || !credentialName(f.Key.Value)) || source.Child(f.Value, "atRest") != nil {
				continue
			}
			mark := ""
			if sens == "" {
				mark = ", and mark it sensitivity: credential"
			}
			c.warn(f.Key, source.Pointer("entities", e.Key.Value, "properties", f.Key.Value), RuleSecretUnprotected,
				"%s.%s is a credential kept as it is, so whoever reads the table or a backup of it reads the value; add atRest: hashed for a value the system only compares with what a caller sends, such as a password, or atRest: encrypted for one it must send on, such as a key to another system%s",
				e.Key.Value, f.Key.Value, mark)
		}
	}
	for _, s := range source.Pairs(source.Child(d.root, "configuration")) {
		if source.Str(source.Child(s.Value, "secret")) == "true" && source.Child(s.Value, "rotation") == nil {
			c.warn(source.Key(s.Value, "secret"), source.Pointer("configuration", s.Key.Value, "secret"), RuleSecretUnprotected,
				"setting %s is a secret with no rotation, so a value that leaks stays good until someone notices; add rotation, the longest time one value is used before it is replaced, such as P90D", s.Key.Value)
		}
	}
}

// checkAuditTrail warns about a requirement asking for an audit trail that
// no entity or securityEvents satisfies, an entity that satisfies it and is
// not appendOnly, and an append-only entity that an operation changes or
// deletes, or that has soft deletion (audit_not_append_only).
func (c *checker) checkAuditTrail(d *design) {
	entities := source.Pairs(source.Child(d.root, "entities"))
	for _, ctl := range d.controlsOf(controlAuditTrail) {
		id := ctl.id
		kept := satisfiedBy(source.Child(d.root, "securityEvents"), id)
		for _, e := range entities {
			if !satisfiedBy(e.Value, id) {
				continue
			}
			kept = true
			if source.Str(source.Child(e.Value, "appendOnly")) != "true" {
				c.warn(e.Key, source.Pointer("entities", e.Key.Value), RuleAuditNotAppendOnly,
					"%s keeps the audit trail %s asks for, and nothing stops a record of it from being changed or deleted; add appendOnly: true", e.Key.Value, id)
			}
		}
		if !kept {
			c.warn(ctl.node, ctl.ptr, RuleAuditNotAppendOnly,
				"%s asks for an audit trail, and no entity or securityEvents satisfies it, so nothing in the design keeps one; name the entity that keeps it, with appendOnly: true and satisfies: [%s]", id, id)
		}
	}
	for _, e := range entities {
		name := e.Key.Value
		if source.Str(source.Child(e.Value, "appendOnly")) != "true" {
			continue
		}
		if source.Str(source.Child(e.Value, "deletion")) == "soft" {
			c.warn(source.Key(e.Value, "deletion"), source.Pointer("entities", name, "deletion"), RuleAuditNotAppendOnly,
				"%s is append-only, and soft deletion changes a record to mark it deleted; remove deletion", name)
		}
		for _, o := range d.opList {
			changes := o.method == "put" || o.method == "patch" || o.method == "delete"
			if changes && d.rowsEntity(o) == name || source.Str(source.Child(source.Child(o.node, "guard"), "entity")) == name {
				c.warn(source.Key(o.pathItem, o.method), o.pointer(), RuleAuditNotAppendOnly,
					"%s is append-only, and %s changes or deletes a record of it; record the change as a new %s, or leave appendOnly out", name, o.id, name)
			}
		}
	}
}
