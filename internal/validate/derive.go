package validate

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// A subject is something tests are written about: an operation, a command,
// a page, or an entity's constraint or transition.
type subject struct {
	kind    string     // operation, command, page, constraint, transition
	label   string     // how a message names it: "operation createLoan"
	node    *yaml.Node // where a missing scenario is reported
	path    string
	yamlKey string // the subject keys of a test, in flow style
	name    string // for the suggested test name
	raw     string // the subject's own name inside its case names, if any
	cases   []derivedCase
}

// A derived case is a scenario the rest of the file says a subject needs.
type derivedCase struct {
	name     string // as written in a test's covers: "missing memberId"
	scenario string // golden or red
	given    string
	when     string
	then     string
}

func (s *subject) red(name, given, when, then string) {
	s.cases = append(s.cases, derivedCase{name, "red", given, when, then})
}

func (s *subject) golden(name, given, when, then string) {
	s.cases = append(s.cases, derivedCase{name, "golden", given, when, then})
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
		out = append(out, d.operationSubject(o))
	}
	for _, p := range source.Pairs(source.Child(d.root, "commands")) {
		out = append(out, commandSubject(p))
	}
	for _, p := range source.Pairs(source.Child(d.root, "pages")) {
		out = append(out, pageSubject(p))
	}
	for _, e := range source.Pairs(source.Child(d.root, "entities")) {
		for _, c := range source.Pairs(source.Child(e.Value, "constraints")) {
			out = append(out, constraintSubject(e.Key.Value, c))
		}
		for i, t := range source.Items(source.Child(e.Value, "transitions")) {
			out = append(out, transitionSubject(e.Key.Value, i, t))
		}
	}
	return out
}

func denied(s *subject, perm, what string) {
	if perm == "" || perm == "public" {
		return
	}
	s.red("denied without "+perm, "a caller without "+perm, what, "it is refused as not allowed")
}

func (d *design) operationSubject(o operation) *subject {
	s := &subject{kind: "operation", label: "operation " + o.id, node: o.node, path: o.pointer(),
		yamlKey: "operation: " + o.id, name: kebab(o.id)}
	call := o.id + " is called"
	// The request body's fields: an inline object, or an entity's fields a
	// client may set.
	body, bodyRequired := d.requestFields(o.node)
	for _, f := range bodyRequired {
		s.red("missing "+f, "...", o.id+" is called without "+f, "it is refused")
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
			s.red("not found "+name, "no record has that "+name, o.id+" is called with that "+name, "it is refused as not found")
		}
	}
	// Body fields that point at another entity through the response
	// entity's relations.
	if ent := d.responseEntity(o.node); ent != "" {
		for _, r := range source.Pairs(source.Child(d.entities[ent], "relations")) {
			kind := source.Str(source.Child(r.Value, "kind"))
			via := source.Str(source.Child(r.Value, "via"))
			if (kind == "many-to-one" || kind == "one-to-one") && body[via] != nil {
				s.red("not found "+via, "no "+source.Str(source.Child(r.Value, "target"))+" has that "+via, o.id+" is called with that "+via, "it is refused as not found")
			}
		}
		if o.method == "post" && source.Child(source.Child(o.node, "responses"), "201") != nil {
			for _, c := range source.Pairs(source.Child(d.entities[ent], "constraints")) {
				if source.Str(source.Child(c.Value, "kind")) == "unique" {
					s.red("duplicate "+c.Key.Value, "a "+ent+" that "+c.Key.Value+" would clash with exists", call, "it is refused as a duplicate")
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
			s.red("dependency fails "+ch, ch+" cannot take the message", call, "...")
		}
	}
	for _, r := range source.Pairs(source.Child(o.node, "responses")) {
		if code := r.Key.Value; len(code) == 3 && (code[0] == '4' || code[0] == '5') {
			s.red("response "+code, "...", call, "it answers "+code+": "+source.Str(source.Child(r.Value, "description")))
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
	num := func(key string) (string, bool) {
		n := source.Child(field, key)
		return source.Str(n), n != nil
	}
	if v, ok := num("minimum"); ok {
		s.red(name+" below minimum "+v, "...", with+" just below "+v, "it is refused")
		s.golden(name+" at minimum "+v, "...", with+" equal to "+v, "it succeeds")
	}
	if v, ok := num("maximum"); ok {
		s.red(name+" above maximum "+v, "...", with+" just above "+v, "it is refused")
		s.golden(name+" at maximum "+v, "...", with+" equal to "+v, "it succeeds")
	}
	if v, ok := num("exclusiveMinimum"); ok {
		s.red(name+" at exclusive minimum "+v, "...", with+" equal to "+v, "it is refused")
	}
	if v, ok := num("exclusiveMaximum"); ok {
		s.red(name+" at exclusive maximum "+v, "...", with+" equal to "+v, "it is refused")
	}
	minLen, hasMin := num("minLength")
	maxLen, hasMax := num("maxLength")
	if hasMin && minLen != "0" {
		s.red(name+" shorter than "+chars(minLen), "...", with+" "+characters(minLen, -1), "it is refused")
		s.golden(name+" of "+chars(minLen), "...", with+" "+characters(minLen, 0), "it succeeds")
	}
	if hasMax {
		s.red(name+" longer than "+chars(maxLen), "...", with+" "+characters(maxLen, 1), "it is refused")
		if !hasMin || minLen != maxLen {
			s.golden(name+" of "+chars(maxLen), "...", with+" "+characters(maxLen, 0), "it succeeds")
		}
	}
	if v, ok := num("minItems"); ok && v != "0" {
		s.red(name+" with fewer than "+v+" items", "...", with+" holding fewer than "+v+" items", "it is refused")
	}
	if v, ok := num("maxItems"); ok {
		s.red(name+" with more than "+v+" items", "...", with+" holding more than "+v+" items", "it is refused")
	}
	if _, ok := num("pattern"); ok {
		s.red(name+" not matching its pattern", "...", with+" in the wrong form", "it is refused")
	}
	if _, _, isEnum := d.enumValues(field); isEnum {
		s.red(name+" not one of its values", "...", with+" set to a value it does not allow", "it is refused")
	}
	if f, _ := num("format"); validatingFormats[f] {
		s.red(name+" not a valid "+f, "...", with+" that is not a valid "+f, "it is refused")
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
	s.red("usage error", "...", name+" is run with arguments it does not accept", "it prints how to use it and exits with the usage status")
	for _, c := range source.Pairs(source.Child(p.Value, "exitCodes")) {
		if c.Key.Value != "0" {
			s.red("exit "+c.Key.Value, "...", run, "it exits "+c.Key.Value+": "+source.Str(c.Value))
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
	denied(s, source.Str(source.Child(p.Value, "permission")), open)
	for _, m := range pathParam.FindAllStringSubmatch(source.Str(source.Child(p.Value, "route")), -1) {
		s.red("not found "+m[1], "no record has that "+m[1], open+" for that "+m[1], "it says the record was not found")
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
		s.red("duplicate "+name, "a "+entity+" exists", "another "+entity+" with the same values is saved", "it is refused: "+msg)
	case "check":
		s.red("violates "+name, "...", "a "+entity+" breaking it is saved", "it is refused: "+msg)
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
	s.red("from wrong state", "the "+entity+" is not "+from, trigger+" happens", "it is refused and the state stays as it was")
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
