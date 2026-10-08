package validate

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// A subject is something tests are written about: an operation, a command,
// a page, or an entity's constraint or transition.
type subject struct {
	kind    string     // operation, command, page, constraint, transition, requirement, flow
	label   string     // how a message names it: "operation createLoan"
	node    *yaml.Node // where a missing scenario is reported
	path    string
	yamlKey string // the subject keys of a test, in flow style
	name    string // for the suggested test name
	raw     string // the subject's own name inside its case names, if any
	cases   []derivedCase
	// success is the golden path the design implies: what a caller
	// allowed to do it sends inside every limit, and the outcome.
	success derivedCase
	// critical is true when a requirement the subject satisfies names a
	// harm.
	critical bool
}

// A derived case is a scenario the rest of the file says a subject needs.
type derivedCase struct {
	name      string // as written in a test's covers: "missing memberId"
	scenario  string // golden or red
	given     string
	when      string
	then      string
	frequency string     // how often users make the mistake: frequent, occasional or rare
	field     *yaml.Node // the field or parameter schema the case is about, or nil
	fieldName string
	// critical marks one case critical on its own: a path through a
	// transition that satisfies a requirement with a harm, or a case nobody
	// exercises by hand (a failing dependency, two writers on one record).
	critical bool
	// reason says why the case is left out, when the general one does not
	// fit the subject.
	reason string
}

// The frequencies of the case kinds, from the table in docs/conventions.md.
const (
	frequent   = "frequent"
	occasional = "occasional"
	rare       = "rare"
)

// The ranks of a derived case. Critical and frequent cases are chosen: the
// validator warns when no test covers one. The others are left out and
// listed in the test plan.
const (
	rankCritical = "critical"
	rankFrequent = "frequent"
	rankOther    = "other"
)

func (s *subject) red(name, frequency, given, when, then string) {
	s.cases = append(s.cases, derivedCase{name: name, scenario: "red", given: given, when: when, then: then, frequency: frequency})
}

// fieldCase adds a case about one field, whose mistakes key may replace
// the frequency.
func (s *subject) fieldCase(scenario, name, frequency, fieldName string, field *yaml.Node, given, when, then string) {
	s.cases = append(s.cases, derivedCase{name: name, scenario: scenario, given: given, when: when, then: then,
		frequency: frequency, field: field, fieldName: fieldName})
}

// mistakes is the field's own frequency, or "".
func (dc derivedCase) mistakes() string {
	return source.Str(source.Child(dc.field, "mistakes"))
}

// rank is critical for a case of a critical subject or a case critical
// on its own, frequent for a case users get wrong often, other otherwise.
func (s *subject) rank(dc derivedCase) string {
	if s.critical || dc.critical {
		return rankCritical
	}
	freq := dc.frequency
	if m := dc.mistakes(); m != "" {
		freq = m
	}
	if freq == frequent {
		return rankFrequent
	}
	return rankOther
}

// leftOutReason says why a case of rank other is left out.
func (s *subject) leftOutReason(dc derivedCase) string {
	if dc.reason != "" {
		return dc.reason
	}
	if m := dc.mistakes(); m != "" {
		return fmt.Sprintf("%s is marked mistakes: %s, and %s satisfies no requirement with a harm", dc.fieldName, m, s.label)
	}
	return fmt.Sprintf("%s case, and %s satisfies no requirement with a harm", dc.frequency, s.label)
}

// chosen says whether a case of the subject is written by default: a case
// whose name the subject derives more than once is chosen when any of
// them is.
func (s *subject) chosen(name string) bool {
	for _, dc := range s.cases {
		if dc.name == name && s.rank(dc) != rankOther {
			return true
		}
	}
	return false
}

// subjects lists every subject of the file with its derived cases, in
// document order of kind: operations, commands, pages, constraints,
// transitions.
func (d *design) subjects() []*subject {
	var out []*subject
	for _, o := range d.opList {
		if o.id == "" {
			continue
		}
		out = append(out, d.withHarm(d.operationSubject(o), o.node))
	}
	for _, p := range source.Pairs(source.Child(d.root, "commands")) {
		out = append(out, d.withHarm(d.commandSubject(p), p.Value))
	}
	for _, p := range source.Pairs(source.Child(d.root, "jobs")) {
		out = append(out, d.withHarm(d.jobSubject(p), p.Value))
	}
	for _, p := range source.Pairs(source.Child(d.root, "workflows")) {
		out = append(out, d.withHarm(d.workflowSubject(p), p.Value))
	}
	for _, p := range source.Pairs(source.Child(d.root, "pages")) {
		out = append(out, d.withHarm(d.pageSubject(p), p.Value))
	}
	for _, p := range source.Pairs(source.Child(d.root, "flows")) {
		out = append(out, d.withHarm(d.screenFlowSubject(p), p.Value))
	}
	for _, e := range source.Pairs(source.Child(d.root, "entities")) {
		for _, c := range source.Pairs(source.Child(e.Value, "constraints")) {
			out = append(out, d.withHarm(constraintSubject(e.Key.Value, c), c.Value))
		}
		for i, t := range source.Items(source.Child(e.Value, "transitions")) {
			out = append(out, d.withHarm(transitionSubject(e.Key.Value, i, t), t))
		}
		if s := d.flowSubject(e); s != nil {
			out = append(out, s)
		}
	}
	for _, r := range source.Pairs(source.Child(d.root, "requirements")) {
		if s := requirementSubject(r); s != nil {
			out = append(out, s)
		}
	}
	return out
}

// requirementSubject is a requirement's acceptance tests: one golden case
// per acceptance criterion, with the criterion as the outcome. They are
// chosen when the requirement names a harm.
func requirementSubject(r source.Pair) *subject {
	id := r.Key.Value
	status := source.Str(source.Child(r.Value, "status"))
	criteria := source.Items(source.Child(r.Value, "acceptance"))
	if len(criteria) == 0 || status == "rejected" || status == "retired" {
		return nil
	}
	s := &subject{kind: "requirement", label: "requirement " + id, node: r.Key, path: source.Pointer("requirements", id),
		yamlKey: "requirement: " + id, name: strings.ToLower(id)}
	s.critical = len(source.Items(source.Child(r.Value, "harm"))) > 0
	for i, c := range criteria {
		s.cases = append(s.cases, derivedCase{name: fmt.Sprintf("acceptance %d", i+1), scenario: "golden", given: "...", when: "...",
			then: source.Str(c), frequency: occasional, reason: id + " names no harm"})
	}
	return s
}

// flowSubject is an entity's state machine: one golden case per path from
// an initial state (one no transition reaches) to a terminal state (one no
// transition leaves), taking the transitions in document order and never
// visiting a state twice. A path is chosen when one of its transitions
// satisfies a requirement with a harm.
func (d *design) flowSubject(e source.Pair) *subject {
	entity := e.Key.Value
	transitions := source.Items(source.Child(e.Value, "transitions"))
	if len(transitions) == 0 {
		return nil
	}
	reached, leaves := map[string]bool{}, map[string]bool{}
	var starts []string
	for _, t := range transitions {
		reached[source.Str(source.Child(t, "to"))] = true
		leaves[source.Str(source.Child(t, "from"))] = true
	}
	seen := map[string]bool{}
	for _, t := range transitions {
		if from := source.Str(source.Child(t, "from")); !reached[from] && !seen[from] {
			seen[from] = true
			starts = append(starts, from)
		}
	}
	s := &subject{kind: "flow", label: entity + " state machine", node: e.Key, path: source.Pointer("entities", entity),
		yamlKey: "entity: " + entity, name: kebab(entity)}
	var walk func(state string, states, triggers []string, critical bool)
	walk = func(state string, states, triggers []string, critical bool) {
		if !leaves[state] {
			if len(triggers) == 0 {
				return
			}
			s.cases = append(s.cases, derivedCase{name: strings.Join(states, " to "), scenario: "golden",
				given: "a " + entity + " that is " + states[0], when: strings.Join(triggers, ", then ") + " happen",
				then:      "the " + entity + " ends " + state + ", having been " + joinAnd(states[:len(states)-1]),
				frequency: occasional, critical: critical, reason: "no transition on the path satisfies a requirement with a harm"})
			return
		}
		for _, t := range transitions {
			if source.Str(source.Child(t, "from")) != state {
				continue
			}
			to := source.Str(source.Child(t, "to"))
			if slices.Contains(states, to) {
				continue
			}
			trigger := source.Str(source.Child(t, "trigger"))
			if trigger == "" {
				trigger = "the move to " + to
			}
			walk(to, append(slices.Clone(states), to), append(slices.Clone(triggers), trigger), critical || d.withHarm(&subject{}, t).critical)
		}
	}
	for _, start := range starts {
		walk(start, []string{start}, nil, false)
	}
	if len(s.cases) == 0 {
		return nil
	}
	return s
}

// withHarm marks the subject critical when a requirement its own
// satisfies names, the node holding that list, has a harm.
func (d *design) withHarm(s *subject, n *yaml.Node) *subject {
	for _, id := range source.Items(source.Child(n, "satisfies")) {
		if len(source.Items(source.Child(d.requirements[id.Value], "harm"))) > 0 {
			s.critical = true
		}
	}
	return s
}

// caller is who may do something that needs a permission.
func caller(perm string) string {
	if perm == "" || perm == "public" {
		return "any caller"
	}
	return "a caller with " + perm
}

func golden(given, when, then string) derivedCase {
	return derivedCase{name: "succeeds", scenario: "golden", given: given, when: when, then: then}
}

// denied adds the cases of a permission other than public: a caller
// without it, and, once the specification declares a session, a caller
// whose session has expired.
func (d *design) denied(s *subject, perm, what string) {
	if perm == "" || perm == "public" {
		return
	}
	s.red("denied without "+perm, frequent, "a caller without "+perm, what, "it is refused as not allowed")
	if source.Child(d.root, "session") != nil {
		s.red("denied with expired session", frequent, "a caller whose session has expired", what, "it is refused as not signed in")
	}
}

// disabled adds the case of an element served only when a setting is on:
// with the setting off, it is refused. A name that is not a boolean
// setting is reported by the setting rule instead.
func (d *design) disabled(s *subject, n *yaml.Node, what string) {
	setting := source.Str(source.Child(n, "enabledBy"))
	if source.Str(source.Child(source.Child(d.settings[setting], "schema"), "type")) == "boolean" {
		s.red("disabled by "+setting, frequent, "the setting "+setting+" is off", what, "it is refused as not available")
	}
}

// byNature adds a red case that is critical on its own.
func (s *subject) byNature(name, given, when, then string) {
	s.cases = append(s.cases, derivedCase{name: name, scenario: "red", given: given, when: when, then: then, frequency: rare, critical: true})
}

// guardCases adds the cases of a guard on an operation or a command: the
// precondition not holding, and a second writer on the same record.
func guardCases(s *subject, g *yaml.Node, what string) {
	if g == nil {
		return
	}
	ent := source.Str(source.Child(g, "entity"))
	if pre := source.Str(source.Child(g, "precondition")); pre != "" {
		s.red("guard precondition fails", occasional, "a "+ent+" for which "+pre+" does not hold", what, "it is refused and no "+ent+" changes")
	}
	s.byNature("concurrent write", "another caller changed the "+ent+" after this caller read it", what, "it is refused and the other caller's change stands")
}

// failureResponse is the response an operation gives when a dependency
// fails or does not answer in time, when the design declares one of the
// statuses given, or "...".
func failureResponse(op *yaml.Node, statuses ...string) string {
	for _, code := range statuses {
		if r := source.Child(source.Child(op, "responses"), code); r != nil {
			return "it answers " + code + ": " + source.Str(source.Child(r, "description"))
		}
	}
	return "..."
}

func (d *design) operationSubject(o operation) *subject {
	s := &subject{kind: "operation", label: "operation " + o.id, node: o.node, path: o.pointer(),
		yamlKey: "operation: " + o.id, name: kebab(o.id)}
	call := o.id + " is called"
	s.success = golden(caller(source.Str(source.Child(o.node, "permission"))), call, "it succeeds")
	if len(source.Pairs(source.Child(source.Child(o.node, "requestBody"), "content"))) > 0 || len(source.Items(source.Child(o.node, "parameters"))) > 0 || len(source.Items(source.Child(o.pathItem, "parameters"))) > 0 {
		s.success.when = call + " with values inside every limit"
	}
	for _, r := range source.Pairs(source.Child(o.node, "responses")) {
		if strings.HasPrefix(r.Key.Value, "2") {
			s.success.then = "it answers " + r.Key.Value + ": " + source.Str(source.Child(r.Value, "description"))
			break
		}
	}
	// The request body's fields: an inline object, or an entity's fields a
	// client may set.
	body, bodyRequired := d.requestFields(o.node)
	for _, f := range bodyRequired {
		s.fieldCase("red", "missing "+f, frequent, f, body[f], "...", o.id+" is called without "+f, "it is refused")
	}
	names := make([]string, 0, len(body))
	for n := range body {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		d.limitCases(s, n, body[n], call)
	}
	for _, p := range append(source.Items(source.Child(o.pathItem, "parameters")), source.Items(source.Child(o.node, "parameters"))...) {
		name := source.Str(source.Child(p, "name"))
		d.limitCases(s, name, source.Child(p, "schema"), call)
		if source.Str(source.Child(p, "in")) == "path" {
			s.fieldCase("red", "not found "+name, frequent, name, source.Child(p, "schema"), "no record has that "+name, o.id+" is called with that "+name, "it is refused as not found")
		}
	}
	// Body fields that point at another entity through the response
	// entity's relations.
	if ent := d.responseEntity(o.node); ent != "" {
		for _, r := range source.Pairs(source.Child(d.entities[ent], "relations")) {
			kind := source.Str(source.Child(r.Value, "kind"))
			via := source.Str(source.Child(r.Value, "via"))
			if (kind == "many-to-one" || kind == "one-to-one") && body[via] != nil {
				target := source.Str(source.Child(r.Value, "target"))
				s.fieldCase("red", "not found "+via, frequent, via, body[via], "no "+target+" has that "+via, o.id+" is called with that "+via, "it is refused as not found")
				if v := source.Child(d.entities[target], "validity"); v != nil {
					s.fieldCase("red", "expired "+via, occasional, via, body[via], "the "+target+" that "+via+" names is past its "+source.Str(source.Child(v, "until")), o.id+" is called with that "+via, "it is refused as expired")
					if from := source.Str(source.Child(v, "from")); from != "" {
						s.fieldCase("red", "not yet valid "+via, occasional, via, body[via], "the "+target+" that "+via+" names is before its "+from, o.id+" is called with that "+via, "it is refused as not yet valid")
					}
				}
			}
		}
		if source.Str(source.Child(d.entities[ent], "deletion")) == "soft" && o.method == "get" {
			d.softDeleteCases(s, o, ent, call)
		}
		if o.method == "post" && source.Child(source.Child(o.node, "responses"), "201") != nil {
			for _, c := range source.Pairs(source.Child(d.entities[ent], "constraints")) {
				if source.Str(source.Child(c.Value, "kind")) == "unique" {
					s.red("duplicate "+c.Key.Value, occasional, "a "+ent+" that "+c.Key.Value+" would clash with exists", call, "it is refused as a duplicate")
				}
			}
		}
	}
	d.denied(s, source.Str(source.Child(o.node, "permission")), call)
	d.disabled(s, o.node, call)
	seenChannel := map[string]bool{}
	for _, e := range source.Items(source.Child(o.node, "emits")) {
		ch, _, _ := strings.Cut(e.Value, "/")
		if !seenChannel[ch] {
			seenChannel[ch] = true
			s.byNature("dependency fails "+ch, ch+" cannot take the message", call, "...")
		}
	}
	for _, e := range source.Items(source.Child(o.node, "calls")) {
		dep := d.dependencies[e.Value]
		if dep == nil {
			continue
		}
		s.byNature("dependency fails "+e.Value, e.Value+" answers with an error", call, failureResponse(o.node, "502", "503"))
		s.byNature("dependency times out "+e.Value, e.Value+" does not answer within "+source.Str(source.Child(dep, "timeout")), call, failureResponse(o.node, "504", "503"))
	}
	if key := source.Str(source.Child(o.node, "idempotencyKey")); key != "" {
		schema := source.Child(headerParameter(o, key), "schema")
		given := o.id + " has answered a request that carried " + key
		s.fieldCase("golden", "repeated with the same "+key, frequent, key, schema, given, o.id+" is called again with the same "+key+" and the same request", "it answers as the first call did and nothing changes a second time")
		s.fieldCase("red", key+" reused for another request", occasional, key, schema, given, o.id+" is called with the same "+key+" and a different request", "it is refused")
	}
	guardCases(s, source.Child(o.node, "guard"), call)
	pageCases(s, o, call)
	limitsCases(s, o, call)
	for _, r := range source.Pairs(source.Child(o.node, "responses")) {
		if code := r.Key.Value; len(code) == 3 && (code[0] == '4' || code[0] == '5') {
			s.red("response "+code, occasional, "...", call, "it answers "+code+": "+source.Str(source.Child(r.Value, "description")))
		}
	}
	return s
}

// pageCases are the cases of a list: a page after the last, a page larger
// than the maximum, and a sort or filter outside the lists.
func pageCases(s *subject, o operation, call string) {
	l := source.Child(o.node, "listOf")
	if l == nil {
		return
	}
	s.cases = append(s.cases, derivedCase{name: "page beyond last", scenario: "golden", given: "fewer records than fill the pages asked for",
		when: call + " for a page after the last", then: "it answers an empty page with the true totals", frequency: occasional})
	if max := source.Str(source.Child(source.Child(l, "pageSize"), "maximum")); max != "" {
		n, _ := strconv.Atoi(max)
		s.red("page size above "+max, occasional, "...", call+" with a page size of "+strconv.Itoa(n+1), "it is refused")
	}
	if source.Child(l, "sortable") != nil {
		s.red("sort by a field not sortable", occasional, "...", call+" sorted by a field that is not sortable", "it is refused, not ignored")
	}
	if source.Child(l, "filterable") != nil {
		s.red("filter by a field not filterable", occasional, "...", call+" filtered by a field that is not filterable", "it is refused, not ignored")
	}
}

// limitsCases are the cases of an operation's limits: a body too large,
// and more requests than the rate allows.
func limitsCases(s *subject, o operation, call string) {
	l := source.Child(o.node, "limits")
	if n := source.Str(source.Child(l, "maxRequestBytes")); n != "" {
		s.red("request larger than "+n+" bytes", occasional, "...", call+" with a body larger than "+n+" bytes", failureOr(o.node, "413", "it is refused as too large"))
	}
	if rate := source.Child(l, "rate"); rate != nil {
		r, per := source.Str(source.Child(rate, "requests")), source.Str(source.Child(rate, "per"))
		s.red("rate exceeded", occasional, "a caller who has made "+r+" requests within "+per, call+" once more", failureOr(o.node, "429", "it is refused as too many requests"))
	}
}

// failureOr is the response of a status, or the plain refusal when the
// operation does not declare it.
func failureOr(op *yaml.Node, status, plain string) string {
	if then := failureResponse(op, status); then != "..." {
		return then
	}
	return plain
}

// softDeleteCases are the cases of a read of an entity with soft deletion:
// a list leaves a deleted record out, and a read by id answers as for a
// record that does not exist.
func (d *design) softDeleteCases(s *subject, o operation, ent, call string) {
	deleted := "a " + ent + " that is deleted"
	if d.responseIsList(o.node) {
		s.cases = append(s.cases, derivedCase{name: "deleted " + ent + " not listed", scenario: "golden", given: deleted, when: call,
			then: "the deleted " + ent + " is not in the answer", frequency: occasional})
		return
	}
	for _, p := range append(source.Items(source.Child(o.pathItem, "parameters")), source.Items(source.Child(o.node, "parameters"))...) {
		if source.Str(source.Child(p, "in")) == "path" {
			s.red("deleted "+ent+" read", occasional, deleted, o.id+" is called with its "+source.Str(source.Child(p, "name")), failureOr(o.node, "404", "it is refused as not found"))
			return
		}
	}
}

// responseIsList reports whether a successful response returns a list.
func (d *design) responseIsList(op *yaml.Node) bool {
	for _, r := range source.Pairs(source.Child(op, "responses")) {
		if !strings.HasPrefix(r.Key.Value, "2") {
			continue
		}
		for _, c := range source.Pairs(source.Child(r.Value, "content")) {
			if source.Child(source.Child(c.Value, "schema"), "items") != nil {
				return true
			}
		}
	}
	return false
}

// requestFields returns the request body's fields and its required ones.
// For a $ref to an entity or a schema, fields the system sets (readOnly)
// are left out.
func (d *design) requestFields(op *yaml.Node) (map[string]*yaml.Node, []string) {
	content := source.Pairs(source.Child(source.Child(op, "requestBody"), "content"))
	if len(content) == 0 {
		return nil, nil
	}
	schema := source.Child(content[0].Value, "schema")
	readOnlyLeftOut := false
	if ref := source.Str(source.Child(schema, "$ref")); strings.HasPrefix(ref, "#/entities/") {
		schema = d.entities[strings.TrimPrefix(ref, "#/entities/")]
		readOnlyLeftOut = true
	} else if strings.HasPrefix(ref, "#/schemas/") {
		schema = d.schemas[strings.TrimPrefix(ref, "#/schemas/")]
		readOnlyLeftOut = true
	}
	fields := map[string]*yaml.Node{}
	for _, p := range source.Pairs(source.Child(schema, "properties")) {
		if readOnlyLeftOut && source.Str(source.Child(p.Value, "readOnly")) == "true" {
			continue
		}
		fields[p.Key.Value] = p.Value
	}
	var required []string
	for _, r := range source.Items(source.Child(schema, "required")) {
		if fields[r.Value] != nil {
			required = append(required, r.Value)
		}
	}
	return fields, required
}

// responseEntity is the entity a successful response returns, alone or in
// a list, or "".
func (d *design) responseEntity(op *yaml.Node) string {
	for _, r := range source.Pairs(source.Child(op, "responses")) {
		if !strings.HasPrefix(r.Key.Value, "2") {
			continue
		}
		for _, c := range source.Pairs(source.Child(r.Value, "content")) {
			schema := source.Child(c.Value, "schema")
			if items := source.Child(schema, "items"); items != nil {
				schema = items
			}
			if ref := source.Str(source.Child(schema, "$ref")); strings.HasPrefix(ref, "#/entities/") {
				return strings.TrimPrefix(ref, "#/entities/")
			}
		}
	}
	return ""
}

// validatingFormats are the formats a value can break; the others (int32,
// password, binary) only say how a value is stored.
var validatingFormats = map[string]bool{
	"date": true, "date-time": true, "time": true, "duration": true, "email": true, "uuid": true,
	"uri": true, "hostname": true, "ipv4": true, "ipv6": true, "decimal": true,
}

// limitCases adds a field's boundary cases: just outside every limit is
// red, exactly on it is golden.
func (d *design) limitCases(s *subject, name string, field *yaml.Node, call string) {
	if field == nil || name == "" {
		return
	}
	with := call + " with " + name
	red := func(caseName, frequency, when string) {
		s.fieldCase("red", caseName, frequency, name, field, "...", when, "it is refused")
	}
	golden := func(caseName, when string) {
		s.fieldCase("golden", caseName, occasional, name, field, "...", when, "it succeeds")
	}
	num := func(key string) (string, bool) {
		n := source.Child(field, key)
		return source.Str(n), n != nil
	}
	if v, ok := num("minimum"); ok {
		red(name+" below minimum "+v, occasional, with+" just below "+v)
		golden(name+" at minimum "+v, with+" equal to "+v)
	}
	if v, ok := num("maximum"); ok {
		red(name+" above maximum "+v, occasional, with+" just above "+v)
		golden(name+" at maximum "+v, with+" equal to "+v)
	}
	if v, ok := num("exclusiveMinimum"); ok {
		red(name+" at exclusive minimum "+v, occasional, with+" equal to "+v)
	}
	if v, ok := num("exclusiveMaximum"); ok {
		red(name+" at exclusive maximum "+v, occasional, with+" equal to "+v)
	}
	minLen, hasMin := num("minLength")
	maxLen, hasMax := num("maxLength")
	if hasMin && minLen != "0" {
		red(name+" shorter than "+chars(minLen), occasional, with+" "+characters(minLen, -1))
		golden(name+" of "+chars(minLen), with+" "+characters(minLen, 0))
	}
	if hasMax {
		red(name+" longer than "+chars(maxLen), occasional, with+" "+characters(maxLen, 1))
		if !hasMin || minLen != maxLen {
			golden(name+" of "+chars(maxLen), with+" "+characters(maxLen, 0))
		}
	}
	if v, ok := num("minItems"); ok && v != "0" {
		red(name+" with fewer than "+v+" items", occasional, with+" holding fewer than "+v+" items")
	}
	if v, ok := num("maxItems"); ok {
		red(name+" with more than "+v+" items", occasional, with+" holding more than "+v+" items")
	}
	if _, ok := num("pattern"); ok {
		red(name+" not matching its pattern", frequent, with+" in the wrong form")
	}
	if _, _, isEnum := d.enumValues(field); isEnum {
		red(name+" not one of its values", frequent, with+" set to a value it does not allow")
	}
	if f, _ := num("format"); validatingFormats[f] {
		red(name+" not a valid "+f, frequent, with+" that is not a valid "+f)
	}
}

func characters(n string, off int) string {
	var v int
	fmt.Sscan(n, &v)
	v += off
	if v == 1 {
		return "of 1 character"
	}
	return fmt.Sprintf("of %d characters", v)
}

func (d *design) commandSubject(p source.Pair) *subject {
	name := p.Key.Value
	s := &subject{kind: "command", label: "command " + name, node: p.Key, path: source.Pointer("commands", name),
		yamlKey: "command: " + name, name: kebab(strings.ReplaceAll(name, " ", "-"))}
	run := name + " is run"
	s.success = golden("...", name+" is run with arguments it accepts", "it exits 0")
	if ok := source.Child(source.Child(p.Value, "exitCodes"), "0"); ok != nil {
		s.success.then = "it exits 0: " + source.Str(ok)
	}
	s.red("usage error", frequent, "...", name+" is run with arguments it does not accept", "it prints how to use it and exits with the usage status")
	for _, c := range source.Pairs(source.Child(p.Value, "exitCodes")) {
		if c.Key.Value != "0" {
			s.red("exit "+c.Key.Value, frequent, "...", run, "it exits "+c.Key.Value+": "+source.Str(c.Value))
		}
	}
	d.denied(s, source.Str(source.Child(p.Value, "permission")), run)
	d.disabled(s, p.Value, run)
	guardCases(s, source.Child(p.Value, "guard"), run)
	return s
}

func (d *design) pageSubject(p source.Pair) *subject {
	name := p.Key.Value
	s := &subject{kind: "page", label: "page " + name, node: p.Key, path: source.Pointer("pages", name),
		yamlKey: "page: " + name, name: name}
	open := "the page " + name + " is opened"
	s.success = golden(caller(source.Str(source.Child(p.Value, "permission"))), open, "it shows the page")
	if pathParam.MatchString(source.Str(source.Child(p.Value, "route"))) {
		s.success.when = open + " for a record that exists"
	}
	d.denied(s, source.Str(source.Child(p.Value, "permission")), open)
	d.disabled(s, p.Value, open)
	if source.Str(source.Child(p.Value, "kind")) == "task" {
		d.answerCases(s, p.Value)
		d.stateCases(s, p.Value, open)
		return s
	}
	for _, m := range pathParam.FindAllStringSubmatch(source.Str(source.Child(p.Value, "route")), -1) {
		s.red("not found "+m[1], frequent, "no record has that "+m[1], open+" for that "+m[1], "it says the record was not found")
	}
	d.stateCases(s, p.Value, open)
	d.pendingCase(s, p.Value)
	childRowCases(s, p.Value)
	d.elementCases(s, p.Value, open)
	return s
}

// pendingCase is the case of a form that submits to a workflow's trigger:
// it is answered 202 and shows the message that the request waits. A
// task page has it already, as its case for the 202 answer.
func (d *design) pendingCase(s *subject, pg *yaml.Node) {
	wf := d.workflowTriggers()[source.Str(source.Child(pg, "submit"))]
	if wf == "" || source.Str(source.Child(pg, "kind")) != "form" {
		return
	}
	then := "it is answered 202 and the request waits for approval in workflow " + wf
	ev := pendingEvent(pg)
	if nav := source.Str(source.Child(ev, "navigate")); nav != "" {
		then += ", leads to the page " + nav
	}
	if m := source.Str(source.Child(ev, "message")); m != "" {
		then += ", saying: " + m
	}
	s.cases = append(s.cases, derivedCase{name: "sent for approval", scenario: "golden", given: "...", when: "the form is submitted with every field valid", then: then, frequency: frequent})
}

// childRowCases are the cases of a form's child rows: one row past the
// maximum of each that has one.
func childRowCases(s *subject, pg *yaml.Node) {
	if source.Str(source.Child(pg, "kind")) != "form" {
		return
	}
	for _, row := range source.Items(source.Child(pg, "childRows")) {
		rel, max := source.Str(source.Child(row, "relation")), source.Str(source.Child(row, "maximum"))
		if max == "" {
			continue
		}
		s.red(rel+" with more than "+max+" rows", occasional, "the form holds "+max+" rows of "+rel, "a row of "+rel+" is added", "it is refused: the form holds at most "+max+" rows of "+rel)
	}
}

// answerCases are a task page's cases, one per success its submit
// operation answers: the first is the page's golden case, and each other
// is a golden case of its own. What the page does on each is its
// onSubmitted for that status, or nothing, and it stays.
func (d *design) answerCases(s *subject, pg *yaml.Node) {
	submitted := "the page " + s.name + " is submitted with every field valid"
	first := true
	for _, r := range source.Pairs(source.Child(d.operations[source.Str(source.Child(pg, "submit"))].node, "responses")) {
		status := r.Key.Value
		if !strings.HasPrefix(status, "2") {
			continue
		}
		then := "it is answered " + status + " and stays on the page"
		if ev := source.Child(source.Child(pg, "onSubmitted"), status); ev != nil {
			then = "it is answered " + status
			if nav := source.Str(source.Child(ev, "navigate")); nav != "" {
				then += " and leads to the page " + nav
			}
			if m := source.Str(source.Child(ev, "message")); m != "" {
				then += ", saying: " + m
			}
		}
		if first {
			s.success.when, s.success.then = submitted, then
			first = false
			continue
		}
		s.cases = append(s.cases, derivedCase{name: "answered " + status, scenario: "golden", given: "...", when: submitted, then: then, frequency: frequent})
	}
}

// stateCases are the cases of a page's states, when it declares them: its
// empty and filtered empty states, and each problem type it can meet,
// showing its own message or the default.
func (d *design) stateCases(s *subject, pg *yaml.Node, open string) {
	states := source.Child(pg, "states")
	task := source.Str(source.Child(pg, "kind")) == "task"
	if states == nil && !task {
		return
	}
	shows := func(st *yaml.Node) string { return "it shows: " + source.Str(source.Child(st, "message")) }
	if st := source.Child(states, "empty"); st != nil {
		s.cases = append(s.cases, derivedCase{name: "empty", scenario: "golden", given: "no records", when: open, then: shows(st), frequency: occasional})
	}
	if st := source.Child(states, "filteredEmpty"); st != nil {
		s.cases = append(s.cases, derivedCase{name: "filtered empty", scenario: "golden", given: "records, none matching the filters", when: open + " with those filters", then: shows(st), frequency: occasional})
	}
	failed := source.Child(states, "failed")
	labels := map[string]string{}
	for _, a := range source.Items(source.Child(pg, "actions")) {
		if source.Str(source.Child(a, "kind")) == "operation" && labels[source.Str(source.Child(a, "target"))] == "" {
			labels[source.Str(source.Child(a, "target"))] = source.Str(source.Child(a, "label"))
		}
	}
	problems, by := d.pageProblems(pg)
	for _, pr := range problems {
		st := source.Child(failed, pr)
		if st == nil {
			st = source.Child(failed, "default")
		}
		then := ""
		switch {
		case st != nil:
			then = shows(st)
		case task && states == nil:
			then = "it shows the problem " + pr // a task page has a case per answer, states or not
		default:
			continue // the state check reports it
		}
		when := open
		switch id := by[pr]; {
		case id == source.Str(source.Child(pg, "submit")) && task:
			when = "the page is submitted"
		case id == source.Str(source.Child(pg, "submit")):
			when = "the form is submitted"
		case id != source.Str(source.Child(pg, "source")):
			when = "the action " + labels[id] + " is taken"
		}
		given := source.Str(source.Child(source.Child(source.Child(d.root, "errors"), pr), "condition"))
		if given == "" {
			given = "..."
		}
		s.red("fails with "+pr, occasional, given, when, then)
	}
}

func constraintSubject(entity string, c source.Pair) *subject {
	name := c.Key.Value
	s := &subject{kind: "constraint", label: entity + " constraint " + name, node: c.Key,
		path:    source.Pointer("entities", entity, "constraints", name),
		yamlKey: "entity: " + entity + ", constraint: " + name, name: strings.ReplaceAll(name, "_", "-"), raw: name}
	msg := source.Str(source.Child(c.Value, "message"))
	switch source.Str(source.Child(c.Value, "kind")) {
	case "unique":
		where := source.Str(source.Child(c.Value, "where"))
		if where == "" {
			s.success = golden("no "+entity+" with the same values exists", "a "+entity+" is saved", "it is saved")
			s.red("duplicate "+name, occasional, "a "+entity+" exists", "another "+entity+" with the same values is saved", "it is refused: "+msg)
			break
		}
		// A partial unique constraint: the values clash only among the
		// records the condition holds for.
		s.success = golden("no "+entity+" with the same values for which "+where+" holds exists", "a "+entity+" for which "+where+" holds is saved", "it is saved")
		s.red("duplicate "+name, occasional, "a "+entity+" for which "+where+" holds exists", "another "+entity+" with the same values, for which "+where+" holds, is saved", "it is refused: "+msg)
		s.cases = append(s.cases, derivedCase{name: "duplicate outside the condition", scenario: "golden", given: "a " + entity + " for which " + where + " holds exists",
			when: "another " + entity + " with the same values, for which " + where + " does not hold, is saved", then: "it is saved", frequency: occasional})
	case "check":
		s.success = golden("...", "a "+entity+" keeping it is saved", "it is saved")
		rules := falsifiers(source.Str(source.Child(c.Value, "expression")))
		if len(rules) < 2 {
			s.red("violates "+name, occasional, "...", "a "+entity+" breaking it is saved", "it is refused: "+msg)
			break
		}
		// A decision table: one case per way the expression can be false.
		for _, rule := range rules {
			falsehood := strings.Join(rule, " is false and ") + " is false"
			s.red("violates "+name+": "+falsehood, occasional, "...", "a "+entity+" is saved with "+falsehood, "it is refused: "+msg)
		}
	}
	return s
}

func transitionSubject(entity string, i int, t *yaml.Node) *subject {
	from, to := source.Str(source.Child(t, "from")), source.Str(source.Child(t, "to"))
	s := &subject{kind: "transition", label: fmt.Sprintf("%s transition %s to %s", entity, from, to), node: t,
		path:    source.Pointer("entities", entity, "transitions", fmt.Sprint(i)),
		yamlKey: fmt.Sprintf("entity: %s, transition: { from: %s, to: %s }", entity, from, to),
		name:    kebab(entity) + "-" + from + "-to-" + to}
	trigger := source.Str(source.Child(t, "trigger"))
	if trigger == "" {
		trigger = "the move"
	}
	s.success = golden("the "+entity+" is "+from, trigger+" happens", "the "+entity+" is "+to)
	s.red("from wrong state", frequent, "the "+entity+" is not "+from, trigger+" happens", "it is refused and the state stays as it was")
	return s
}

// kebab turns createLoan into create-loan.
func kebab(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('-')
			}
			r = unicode.ToLower(r)
		}
		if r == ' ' || r == '_' || r == '.' {
			r = '-'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// chars says "1 character" or "40 characters".
func chars(n string) string {
	if n == "1" {
		return "1 character"
	}
	return n + " characters"
}

// falsifiers lists the ways a boolean expression can be false, each as
// the clauses that are false together: one per clause of an && made false
// alone, and for an || every disjunct false at once. The clauses are the
// expression's own text, so every build names them alike.
func falsifiers(expr string) [][]string {
	expr = strings.TrimSpace(expr)
	if parts := splitTop(expr, "||"); len(parts) > 1 {
		out := [][]string{nil}
		for _, p := range parts {
			var next [][]string
			for _, prefix := range out {
				for _, f := range falsifiers(p) {
					next = append(next, append(append([]string{}, prefix...), f...))
				}
			}
			out = next
		}
		return out
	}
	if parts := splitTop(expr, "&&"); len(parts) > 1 {
		var out [][]string
		for _, p := range parts {
			out = append(out, falsifiers(p)...)
		}
		return out
	}
	if inner, ok := unwrap(expr); ok {
		return falsifiers(inner)
	}
	return [][]string{{expr}}
}

// splitTop splits an expression at an operator that is outside every
// bracket and string.
func splitTop(expr, op string) []string {
	var parts []string
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(expr); i++ {
		ch := expr[i]
		switch {
		case quote != 0:
			if ch == '\\' {
				i++
			} else if ch == quote {
				quote = 0
			}
		case ch == '"' || ch == '\'':
			quote = ch
		case ch == '(' || ch == '[' || ch == '{':
			depth++
		case ch == ')' || ch == ']' || ch == '}':
			depth--
		case depth == 0 && strings.HasPrefix(expr[i:], op):
			parts = append(parts, strings.TrimSpace(expr[start:i]))
			i += len(op) - 1
			start = i + 1
		}
	}
	return append(parts, strings.TrimSpace(expr[start:]))
}

// unwrap removes the parentheses around a whole expression.
func unwrap(expr string) (string, bool) {
	if !strings.HasPrefix(expr, "(") || !strings.HasSuffix(expr, ")") {
		return "", false
	}
	depth := 0
	for i := 0; i < len(expr); i++ {
		switch expr[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && i != len(expr)-1 {
				return "", false
			}
		}
	}
	return strings.TrimSpace(expr[1 : len(expr)-1]), true
}
