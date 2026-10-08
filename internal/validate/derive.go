package validate

import (
	"fmt"
	"slices"
	"sort"
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
	frequency string // how often users make the mistake: frequent, occasional or rare
	field     *yaml.Node // the field or parameter schema the case is about, or nil
	fieldName string
	// critical marks one case critical on its own: a path through a
	// transition that satisfies a requirement with a harm.
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

// rank is critical for a case of a critical subject or a failing
// dependency, frequent for a case users get wrong often, other otherwise.
func (s *subject) rank(dc derivedCase) string {
	if s.critical || dc.critical || strings.HasPrefix(dc.name, "dependency fails ") {
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
		out = append(out, d.withHarm(commandSubject(p), p.Value))
	}
	for _, p := range source.Pairs(source.Child(d.root, "pages")) {
		out = append(out, d.withHarm(pageSubject(p), p.Value))
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
				then: "the " + entity + " ends " + state + ", having been " + joinAnd(states[:len(states)-1]),
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

func denied(s *subject, perm, what string) {
	if perm == "" || perm == "public" {
		return
	}
	s.red("denied without "+perm, frequent, "a caller without "+perm, what, "it is refused as not allowed")
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
				s.fieldCase("red", "not found "+via, frequent, via, body[via], "no "+source.Str(source.Child(r.Value, "target"))+" has that "+via, o.id+" is called with that "+via, "it is refused as not found")
			}
		}
		if o.method == "post" && source.Child(source.Child(o.node, "responses"), "201") != nil {
			for _, c := range source.Pairs(source.Child(d.entities[ent], "constraints")) {
				if source.Str(source.Child(c.Value, "kind")) == "unique" {
					s.red("duplicate "+c.Key.Value, occasional, "a "+ent+" that "+c.Key.Value+" would clash with exists", call, "it is refused as a duplicate")
				}
			}
		}
	}
	denied(s, source.Str(source.Child(o.node, "permission")), call)
	seenChannel := map[string]bool{}
	for _, e := range source.Items(source.Child(o.node, "emits")) {
		ch, _, _ := strings.Cut(e.Value, "/")
		if !seenChannel[ch] {
			seenChannel[ch] = true
			s.red("dependency fails "+ch, rare, ch+" cannot take the message", call, "...")
		}
	}
	for _, r := range source.Pairs(source.Child(o.node, "responses")) {
		if code := r.Key.Value; len(code) == 3 && (code[0] == '4' || code[0] == '5') {
			s.red("response "+code, occasional, "...", call, "it answers "+code+": "+source.Str(source.Child(r.Value, "description")))
		}
	}
	return s
}

// requestFields returns the request body's fields and its required ones.
// For a $ref to an entity, fields the system sets (readOnly) are left out.
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

func commandSubject(p source.Pair) *subject {
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
	denied(s, source.Str(source.Child(p.Value, "permission")), run)
	return s
}

func pageSubject(p source.Pair) *subject {
	name := p.Key.Value
	s := &subject{kind: "page", label: "page " + name, node: p.Key, path: source.Pointer("pages", name),
		yamlKey: "page: " + name, name: name}
	open := "the page " + name + " is opened"
	s.success = golden(caller(source.Str(source.Child(p.Value, "permission"))), open, "it shows the page")
	if pathParam.MatchString(source.Str(source.Child(p.Value, "route"))) {
		s.success.when = open + " for a record that exists"
	}
	denied(s, source.Str(source.Child(p.Value, "permission")), open)
	for _, m := range pathParam.FindAllStringSubmatch(source.Str(source.Child(p.Value, "route")), -1) {
		s.red("not found "+m[1], frequent, "no record has that "+m[1], open+" for that "+m[1], "it says the record was not found")
	}
	return s
}

func constraintSubject(entity string, c source.Pair) *subject {
	name := c.Key.Value
	s := &subject{kind: "constraint", label: entity + " constraint " + name, node: c.Key,
		path:    source.Pointer("entities", entity, "constraints", name),
		yamlKey: "entity: " + entity + ", constraint: " + name, name: strings.ReplaceAll(name, "_", "-"), raw: name}
	msg := source.Str(source.Child(c.Value, "message"))
	switch source.Str(source.Child(c.Value, "kind")) {
	case "unique":
		s.success = golden("no "+entity+" with the same values exists", "a "+entity+" is saved", "it is saved")
		s.red("duplicate "+name, occasional, "a "+entity+" exists", "another "+entity+" with the same values is saved", "it is refused: "+msg)
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

