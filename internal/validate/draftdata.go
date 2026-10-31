package validate

import (
	"crypto/sha1"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/source"
)

// draftCall is the structured data a draft test carries for the
// generators: the caller, the records that exist before the call, what
// the call carries and what it answers. Each value is YAML text in flow
// style, checked against its field as test_data checks it.
type draftCall struct {
	caller   string
	records  []draftRecord
	input    []keyText
	hasInput bool
	expect   []keyText
}

type draftRecord struct {
	entity string
	fields []keyText
}

type keyText struct{ key, text string }

func (c draftCall) empty() bool {
	return c.caller == "" && len(c.records) == 0 && !c.hasInput && len(c.expect) == 0
}

// lines writes the data as the fixture, input and expect keys of a
// test.yaml, in flow style, one key a line where it fits.
func (c draftCall) lines() []string {
	var out []string
	if c.caller != "" || len(c.records) > 0 {
		var parts []string
		if c.caller != "" {
			parts = append(parts, "caller: "+c.caller)
		}
		for _, r := range c.records {
			parts = append(parts, r.entity+": [{ "+joinKeyText(r.fields)+" }]")
		}
		if len(c.records) == 0 {
			out = append(out, "fixture: { "+strings.Join(parts, ", ")+" }")
		} else {
			out = append(out, "fixture:")
			for _, p := range parts {
				out = append(out, "  "+p)
			}
		}
	}
	if c.hasInput && len(c.input) > 0 {
		out = append(out, "input: { "+joinKeyText(c.input)+" }")
	}
	if len(c.expect) > 0 {
		out = append(out, "expect: { "+joinKeyText(c.expect)+" }")
	}
	return out
}

func joinKeyText(kts []keyText) string {
	parts := make([]string, len(kts))
	for i, kt := range kts {
		parts[i] = flowScalar(kt.key) + ": " + kt.text
	}
	return strings.Join(parts, ", ")
}

func (c draftCall) equal(o draftCall) bool {
	return strings.Join(c.lines(), "\n") == strings.Join(o.lines(), "\n")
}

// operationDraft is what the design fixes about a call of one operation:
// a valid value for each parameter and body field it requires, the
// records those values name, a role that holds the permission and one
// that does not.
type operationDraft struct {
	op       operation
	fields   map[string]*yaml.Node // every parameter and body field, by name
	input    []keyText             // a valid value of every required one, in the order the design lists them
	inputOK  bool                  // every required one has a value
	records  []draftRecord
	recordOf map[string]int // a field's name to the record it names
	holder   string         // a role that holds the permission, or "" for none needed
	holderOK bool           // a role holds it, or none is needed
	lacker   string         // a role that does not hold it
}

func (d *design) operationDraft(o operation) operationDraft {
	od := operationDraft{op: o, fields: map[string]*yaml.Node{}, recordOf: map[string]int{}, inputOK: true}
	type named struct {
		name     string
		field    *yaml.Node
		required bool
		path     bool
	}
	var all []named
	for _, p := range append(source.Items(source.Child(o.pathItem, "parameters")), source.Items(source.Child(o.node, "parameters"))...) {
		name := source.Str(source.Child(p, "name"))
		in := source.Str(source.Child(p, "in"))
		all = append(all, named{name, source.Child(p, "schema"), in == "path" || source.Str(source.Child(p, "required")) == "true", in == "path"})
	}
	body, required := d.requestFields(o.node)
	req := map[string]bool{}
	for _, r := range required {
		req[r] = true
	}
	var bodyNames []string
	for n := range body {
		bodyNames = append(bodyNames, n)
	}
	sort.Strings(bodyNames)
	for _, n := range bodyNames {
		all = append(all, named{n, body[n], req[n], false})
	}
	via := d.viaTargets(o)
	for _, f := range all {
		od.fields[f.name] = f.field
		if !f.required {
			continue
		}
		v, ok := d.sampleValue(f.name, f.field)
		if !ok {
			od.inputOK = false
			continue
		}
		od.input = append(od.input, keyText{f.name, v})
		target := via[f.name]
		if target == "" && f.path {
			target = d.entityNamedBy(f.name)
		}
		if pk := d.singleKey(target); pk != "" {
			od.recordOf[f.name] = len(od.records)
			od.records = append(od.records, draftRecord{target, []keyText{{pk, v}}})
		}
	}
	perm := source.Str(source.Child(o.node, "permission"))
	od.holder, od.holderOK = d.roleFor(perm, true)
	od.lacker, _ = d.roleFor(perm, false)
	return od
}

// viaTargets maps each body field that is the via of a relation of the
// entity the operation answers with to that relation's target.
func (d *design) viaTargets(o operation) map[string]string {
	out := map[string]string{}
	ent := d.responseEntity(o.node)
	for _, r := range source.Pairs(source.Child(d.entities[ent], "relations")) {
		kind := source.Str(source.Child(r.Value, "kind"))
		if kind == "many-to-one" || kind == "one-to-one" {
			out[source.Str(source.Child(r.Value, "via"))] = source.Str(source.Child(r.Value, "target"))
		}
	}
	return out
}

// entityNamedBy is the entity a path parameter names by its own name, the
// entity's name with a small first letter and then its one key field with
// a capital: memberId names Member when Member's key is id. "" when none
// does.
func (d *design) entityNamedBy(param string) string {
	for name := range d.entities {
		pk := d.singleKey(name)
		if pk != "" && param == strings.ToLower(name[:1])+name[1:]+strings.ToUpper(pk[:1])+pk[1:] {
			return name
		}
	}
	return ""
}

// singleKey is an entity's primary key when it is one field, or "".
func (d *design) singleKey(entity string) string {
	keys := source.Items(source.Child(d.entities[entity], "primaryKey"))
	if len(keys) != 1 {
		return ""
	}
	return keys[0].Value
}

// roleFor is the first role by name that holds the permission (holds
// true) or does not (holds false). For a public permission, or none, no
// role is needed, and ok is true with no role.
func (d *design) roleFor(perm string, holds bool) (string, bool) {
	if perm == "" || perm == permissionPublic {
		return "", holds
	}
	if perm == permissionNobody {
		return "", false
	}
	var names []string
	for name := range d.roles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		has := perm == permissionSignedIn || d.roleHolds(d.roles[name])[perm]
		if has == holds {
			return name, true
		}
	}
	return "", false
}

// sampleValue is a value of a field inside every limit it has, as YAML
// text: its own example, default or first enum value, or a value made from
// its type. ok is false when none can be made that test_data takes: an
// object, a list, a pattern.
func (d *design) sampleValue(name string, field *yaml.Node) (string, bool) {
	if field == nil {
		return "", false
	}
	var candidates []string
	for _, key := range []string{"example", "default"} {
		if n := source.Child(field, key); n != nil && n.Kind == yaml.ScalarNode {
			candidates = append(candidates, scalarText(n))
		}
	}
	if items := source.Items(source.Child(field, "examples")); len(items) > 0 && items[0].Kind == yaml.ScalarNode {
		candidates = append(candidates, scalarText(items[0]))
	}
	if made, ok := d.madeValue(name, field); ok {
		candidates = append(candidates, made)
	}
	for _, c := range candidates {
		if d.takes(c, field) {
			return c, true
		}
	}
	return "", false
}

// madeValue makes a value of a field's type, inside its limits.
func (d *design) madeValue(name string, field *yaml.Node) (string, bool) {
	t := d.fieldType(field)
	wireString := source.Str(source.Child(field, "type")) == "string"
	num := func(v string) string {
		if wireString {
			return strconv.Quote(v)
		}
		return v
	}
	switch t.Kind {
	case expr.Enum:
		list := source.Child(field, "enum")
		if ref := source.Str(source.Child(field, "$ref")); strings.HasPrefix(ref, "#/enums/") {
			list = source.Child(d.enums[strings.TrimPrefix(ref, "#/enums/")], "enum")
		}
		if items := source.Items(list); len(items) > 0 {
			return strconv.Quote(items[0].Value), true
		}
		return "", false
	case expr.Int, expr.Uint:
		return num(inRange(field, 1)), true
	case expr.Double:
		return inRange(field, 1), true
	case expr.Decimal:
		v := inRange(field, 1)
		if t.Scale > 0 {
			v += "." + strings.Repeat("0", t.Scale)
		}
		return strconv.Quote(v), true
	case expr.Bool:
		return "true", true
	case expr.Date:
		return `"2026-01-15"`, true
	case expr.Timestamp:
		return `"2026-01-15T09:00:00Z"`, true
	case expr.TimeOfDay:
		return `"09:00:00"`, true
	case expr.Duration:
		return `"PT1H"`, true
	case expr.String:
		if source.Child(field, "pattern") != nil {
			return "", false
		}
		switch source.Str(source.Child(field, "format")) {
		case "uuid":
			return strconv.Quote(nameUUID(name)), true
		case "email":
			return strconv.Quote(strings.ToLower(kebab(name)) + "@example.com"), true
		case "uri":
			return `"https://example.com/"`, true
		case "hostname":
			return `"example.com"`, true
		case "ipv4":
			return `"192.0.2.1"`, true
		case "ipv6":
			return `"2001:db8::1"`, true
		}
		return strconv.Quote(sizedText(name, field)), true
	}
	return "", false
}

// inRange is a whole number inside a field's minimum and maximum, the
// preferred one when it fits.
func inRange(field *yaml.Node, preferred int) string {
	v := preferred
	if n, ok := source.Int(source.Child(field, "minimum")); ok && int64(v) < n {
		v = int(n)
	}
	if n, ok := source.Int(source.Child(field, "exclusiveMinimum")); ok && int64(v) <= n {
		v = int(n) + 1
	}
	if n, ok := source.Int(source.Child(field, "maximum")); ok && int64(v) > n {
		v = int(n)
	}
	if n, ok := source.Int(source.Child(field, "exclusiveMaximum")); ok && int64(v) >= n {
		v = int(n) - 1
	}
	return strconv.Itoa(v)
}

// sizedText is a text for a string field, its name padded or cut to the
// field's lengths.
func sizedText(name string, field *yaml.Node) string {
	text := kebab(name)
	if n, ok := source.Int(source.Child(field, "minLength")); ok && int64(len(text)) < n {
		text += strings.Repeat("x", int(n)-len(text))
	}
	if n, ok := source.Int(source.Child(field, "maxLength")); ok && int64(len(text)) > n {
		text = text[:n]
	}
	return text
}

// nameUUID is a UUID made from a name, the same for the same name, so that
// a fixture's record and the input that names it agree.
func nameUUID(name string) string {
	h := sha1.Sum([]byte("specarch derive " + name))
	h[6] = h[6]&0x0f | 0x50
	h[8] = h[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

// takes reports whether a value, as YAML text, is one test_data takes for
// the field.
func (d *design) takes(text string, field *yaml.Node) bool {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.ScalarNode {
		return false
	}
	return d.fieldValue(doc.Content[0], field) == ""
}

// scalarText is a scalar node as YAML text in flow style.
func scalarText(n *yaml.Node) string {
	if n.ShortTag() == "!!str" {
		return strconv.Quote(n.Value)
	}
	return n.Value
}

// badStatus is the status an operation answers a request it cannot read
// with: its 400, or its 422 when it has that and no 400; "" when it has
// neither.
func badStatus(op *yaml.Node) string {
	for _, s := range []string{"400", "422"} {
		if source.Child(source.Child(op, "responses"), s) != nil {
			return s
		}
	}
	return ""
}

func successStatus(op *yaml.Node) string {
	for _, r := range source.Pairs(source.Child(op, "responses")) {
		if strings.HasPrefix(r.Key.Value, "2") {
			return r.Key.Value
		}
	}
	return ""
}

// operationCall is the data of one case of an operation, or an empty
// draftCall when the design does not fix it.
func (d *design) operationCall(od operationDraft, dc derivedCase) draftCall {
	op := od.op.node
	has := func(status string) bool { return source.Child(source.Child(op, "responses"), status) != nil }
	status := func(s string) []keyText { return []keyText{{"status", s}} }
	// valid is the call a caller who may makes with valid values, the
	// records they name existing.
	valid := func(expect, dropInput, dropRecord string, change keyText) draftCall {
		if !od.inputOK || !od.holderOK || expect == "" {
			return draftCall{}
		}
		c := draftCall{caller: od.holder, hasInput: true, expect: status(expect)}
		for i, r := range od.records {
			if k, ok := od.recordOf[dropRecord]; ok && k == i {
				continue
			}
			c.records = append(c.records, r)
		}
		changed := false
		for _, kt := range od.input {
			switch {
			case kt.key == change.key:
				c.input = append(c.input, change)
				changed = true
			case kt.key != dropInput:
				c.input = append(c.input, kt)
			}
		}
		if change.key != "" && !changed {
			c.input = append(c.input, change)
		}
		return c
	}
	name := dc.name
	switch {
	case name == "":
		if c := valid(successStatus(op), "", "", keyText{}); !c.empty() {
			return c
		}
		if s := successStatus(op); s != "" {
			return draftCall{expect: status(s)}
		}
	case strings.HasPrefix(name, "response "):
		return draftCall{expect: status(strings.TrimPrefix(name, "response "))}
	case strings.HasPrefix(name, "repeated with the same "):
		if s := successStatus(op); s != "" {
			return draftCall{expect: status(s)}
		}
	case name == "denied with expired session" && has("401"):
		return draftCall{expect: status("401")}
	case strings.HasPrefix(name, "denied without "):
		c := valid(map[bool]string{true: "403"}[has("403")], "", "", keyText{})
		if c.empty() {
			// The input cannot be made, or no status is declared: the role
			// and the status the design gives, as far as it gives them.
			c = draftCall{}
			if has("403") {
				c.expect = status("403")
			}
		}
		c.caller = od.lacker
		if c.caller == "" {
			// No role lacks the permission: the status alone.
			c = draftCall{expect: c.expect}
		}
		return c
	case name == "denied when not signed in" && has("401"):
		c := valid("401", "", "", keyText{})
		c.caller = ""
		if c.empty() || !c.hasInput {
			return draftCall{expect: status("401")}
		}
		return c
	case strings.HasPrefix(name, "not found ") && has("404"):
		field := strings.TrimPrefix(name, "not found ")
		if _, ok := od.recordOf[field]; ok {
			return valid("404", "", field, keyText{})
		}
		return draftCall{expect: status("404")}
	case strings.HasPrefix(name, "missing "):
		field := strings.TrimPrefix(name, "missing ")
		if od.fields[field] != nil {
			return valid(badStatus(op), field, "", keyText{})
		}
	default:
		if field, value, golden := d.limitValue(od, name, dc); field != "" {
			expect := badStatus(op)
			if golden {
				expect = successStatus(op)
			}
			return valid(expect, "", "", keyText{field, value})
		}
	}
	return draftCall{}
}

// limitValue is the field and the value of a case about one field's form
// or limits: not a valid format, below or above a limit, shorter or longer
// than a length, or golden at a limit. "" when the case is of no such
// kind, or its value is one test_data would refuse (a malformed date, a
// value not of an enum): such a value cannot be written in a test, which
// holds only values of the field's type.
func (d *design) limitValue(od operationDraft, name string, dc derivedCase) (field, value string, golden bool) {
	f := dc.fieldName
	schema := od.fields[f]
	if f == "" || schema == nil || !strings.HasPrefix(name, f+" ") {
		return "", "", false
	}
	rest := strings.TrimPrefix(name, f+" ")
	t := d.fieldType(schema)
	wireString := source.Str(source.Child(schema, "type")) == "string"
	number := func(key string, off int) string {
		n, ok := source.Int(source.Child(schema, key))
		if !ok {
			return ""
		}
		v := strconv.FormatInt(n+int64(off), 10)
		if t.Kind == expr.Decimal || wireString {
			return strconv.Quote(v)
		}
		return v
	}
	length := func(key string, off int) string {
		n, ok := source.Int(source.Child(schema, key))
		if !ok || n+int64(off) < 0 {
			return ""
		}
		return strconv.Quote(strings.Repeat("x", int(n)+off))
	}
	switch {
	case strings.HasPrefix(rest, "not a valid ") && t.Kind == expr.String:
		value = strconv.Quote("not a " + strings.TrimPrefix(rest, "not a valid "))
	case strings.HasPrefix(rest, "below minimum "):
		value = number("minimum", -1)
	case strings.HasPrefix(rest, "above maximum "):
		value = number("maximum", 1)
	case strings.HasPrefix(rest, "at minimum "):
		value, golden = number("minimum", 0), true
	case strings.HasPrefix(rest, "at maximum "):
		value, golden = number("maximum", 0), true
	case strings.HasPrefix(rest, "at exclusive minimum "):
		value = number("exclusiveMinimum", 0)
	case strings.HasPrefix(rest, "at exclusive maximum "):
		value = number("exclusiveMaximum", 0)
	case strings.HasPrefix(rest, "shorter than ") && t.Kind == expr.String:
		value = length("minLength", -1)
	case strings.HasPrefix(rest, "longer than ") && t.Kind == expr.String:
		value = length("maxLength", 1)
	case strings.HasPrefix(rest, "of ") && t.Kind == expr.String:
		n := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(rest, "of "), " characters"), " character")
		if c, err := strconv.Atoi(n); err == nil {
			value, golden = strconv.Quote(strings.Repeat("x", c)), true
		}
	}
	if value == "" || !d.takes(value, schema) {
		return "", "", false
	}
	return f, value, golden
}

// Fill is the structured data derive --fill adds to a test that has none:
// the lines of its fixture, input and expect, or, when it adds nothing,
// why.
type Fill struct {
	Name    string   // the test's name
	Subject string   // how a message names the subject
	Lines   []string // the lines to add, or none
	Reason  string   // why nothing is added, when the design fixes data but it cannot be added
}

// Fills lists, for every test with none of fixture, input and expect and
// not marked notApplicable, the data its cases draft: the data of the one
// case it covers, or of the success when a golden test covers none. A
// test of several cases is filled when the data of one of them holds what
// the others give, as not found memberId does response 404; otherwise its
// cases draft different data, and it is left as it is with that reason.
// A test whose cases the design fixes nothing about is not listed.
func Fills(root *yaml.Node, given Given) []Fill {
	d := newDesign(root).withIdentity(given)
	bySubject := map[string]*subject{}
	for _, s := range d.subjects() {
		bySubject[s.yamlKey] = s
	}
	var out []Fill
	for _, p := range source.Pairs(source.Child(root, "tests")) {
		t := p.Value
		if source.Child(t, "fixture") != nil || source.Child(t, "input") != nil || source.Child(t, "expect") != nil || source.Child(t, "notApplicable") != nil {
			continue
		}
		s := bySubject[testSubjectKey(t)]
		if s == nil {
			continue
		}
		var calls []draftCall
		covers := source.Items(source.Child(t, "covers"))
		if len(covers) == 0 && source.Str(source.Child(t, "scenario")) == "golden" {
			success := s.success
			success.name = ""
			calls = append(calls, d.draftData(s, success))
		}
		for _, item := range covers {
			for _, dc := range s.cases {
				if dc.name == item.Value {
					calls = append(calls, d.draftData(s, dc))
					break
				}
			}
		}
		var nonEmpty []draftCall
		for _, c := range calls {
			if !c.empty() {
				c.caller = d.namedCaller(s, c.caller, source.Str(source.Child(t, "given"))+" "+source.Str(source.Child(t, "when")))
				nonEmpty = append(nonEmpty, c)
			}
		}
		if len(nonEmpty) == 0 {
			continue
		}
		f := Fill{Name: p.Key.Value, Subject: s.label}
		if whole, ok := holdingAll(nonEmpty); ok {
			f.Lines = whole.lines()
		} else {
			f.Reason = "its cases draft different data; give the fixture, input and expect of the case it is about by hand, or split it into one test per case"
		}
		out = append(out, f)
	}
	return out
}

// holdingAll is the call among calls that holds what every other gives:
// the same expected outcome, and no caller, record or input that differs.
func holdingAll(calls []draftCall) (draftCall, bool) {
	for _, big := range calls {
		all := true
		for _, small := range calls {
			if !holds(big, small) {
				all = false
				break
			}
		}
		if all {
			return big, true
		}
	}
	return draftCall{}, false
}

func holds(big, small draftCall) bool {
	if big.equal(small) {
		return true
	}
	if joinKeyText(big.expect) != joinKeyText(small.expect) {
		return false
	}
	return (small.caller == "" || small.caller == big.caller) && len(small.records) == 0 && !small.hasInput
}

// namedCaller is the role a test's own words name in place of the one the
// draft chose, when it holds the operation's permission as that one does
// or does not: a test that says "a caller holding only the member role"
// signs in as member, not as the first role by name.
func (d *design) namedCaller(s *subject, chosen, words string) string {
	if chosen == "" || s.kind != "operation" {
		return chosen
	}
	o, ok := d.operations[strings.TrimPrefix(s.yamlKey, "operation: ")]
	if !ok {
		return chosen
	}
	perm := source.Str(source.Child(o.node, "permission"))
	holds := func(role string) bool {
		return perm == permissionSignedIn || d.roleHolds(d.roles[role])[perm]
	}
	want := holds(chosen)
	best, at := "", -1
	lower := strings.ToLower(words)
	var names []string
	for name := range d.roles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, form := range []string{name, strings.ReplaceAll(name, "-", " ")} {
			i := wordIndex(lower, strings.ToLower(form))
			if i >= 0 && holds(name) == want && (at < 0 || i < at) {
				best, at = name, i
			}
		}
	}
	if best == "" {
		return chosen
	}
	return best
}

// wordIndex is where a word or phrase first stands as a whole word in a
// text, or -1.
func wordIndex(text, word string) int {
	isLetter := func(b byte) bool { return b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '-' }
	for from := 0; ; {
		i := strings.Index(text[from:], word)
		if i < 0 {
			return -1
		}
		i += from
		end := i + len(word)
		if (i == 0 || !isLetter(text[i-1])) && (end == len(text) || !isLetter(text[end])) {
			return i
		}
		from = i + 1
	}
}
