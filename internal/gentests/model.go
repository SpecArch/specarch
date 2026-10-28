package gentests

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/mark"
)

// Suite is what a specification's tests are, before they are written in a
// stack's language: every stack's plug-in reads the specification the same
// way and differs only in how it writes the result.
type Suite struct {
	Title      string
	Version    string
	Meta       string
	Root       string // the root file as seen from the output folder
	Tests      []Test
	Algorithms []Algorithm
	Mappings   []Mapping           // the calls whose mapping has worked examples (ADR-207)
	Draft      string              // the draft notice, or ""
	Marks      map[string][]string // the mark lines placed at each test and algorithm, by pointer, and at the file as a whole
}

// Test is one design test.
type Test struct {
	Name          string
	Subject       string
	Scenario      string
	Given         string
	When          string
	Then          string
	NotApplicable string // the reason, when the test does not apply
	Caller        string // the role signed in, or "" for none or public
	// CallerProperties are the caller's properties the fixture gives, by
	// name, for a harness that fills the caller (meta-model 0.3).
	CallerProperties []CallerProperty
	Inserts       []Stored
	Call          *Call // nil when the design gives no call to make
	Expect        Expect
}

// Stored is a record of an entity, through the entity's mapping.
// CallerProperty is one property of the caller a fixture gives, as the
// test wrote it.
type CallerProperty struct {
	Name  string
	Value any
}

type Stored struct {
	Mapping string
	Record  []Field
}

// Field is one value by its field or parameter name.
type Field struct {
	Name  string
	Value Value
}

// Value is one value of the design: its type in the expression language
// and its text, or null. An object, a value object or a record, has its
// parts by name and a list its items, each with the type its schema gives;
// Text is then the value as JSON.
type Value struct {
	Null  bool
	Kind  string
	Text  string
	Parts []Field
	Items []Value
}

// Call is what a test does: an operation called, a command run or a page
// opened.
type Call struct {
	Kind      string // request, run or open
	Mapping   string
	Method    string // request: the method in upper case
	Path      string // request: the path as the specification writes it
	Command   string // run: the command's name
	Page      string // open: the page's name
	Route     string // open: the page's route
	Input     []Field
	Arguments []Argument // run
	Options   []Field    // run
	Handshake string     // request: the handshake whose envelope the request travels in, "" for none (ADR-145)
	Stream    string     // request: upload or download when the operation streams a file, "" for none (ADR-146)
}

// Argument is a command's argument with its values.
type Argument struct {
	Name   string
	Values []Value
}

// Expect is a test's expected outcome; what it does not give stays empty.
type Expect struct {
	Status       string
	Body         []Field
	HasBody      bool
	Exit         string
	Output       []string
	HasOutput    bool
	State        []Stored
	Emits        []string
	EmitsNothing bool
}

// Checked reports whether the expectation checks what the call answers,
// as opposed to only what it left behind.
func (e Expect) Checked() bool {
	return e.Status != "" || e.HasBody || e.Exit != "" || e.HasOutput
}

// Algorithm is an algorithm with worked examples.
type Algorithm struct {
	Name     string
	Mapping  string
	Examples []Example
}

// Example is one worked example.
type Example struct {
	Label    string
	Input    []Field
	Expected Value
}

// Mapping is a call of an operation whose mapping has worked examples.
type Mapping struct {
	Operation string // the operationId
	Mapping   string // the operation's mapping, as a test's request takes it
	Call      string // the call as the derived cases name it
	Reference string // the calls entry's call, as the harness is given it
	Pointer   string // the operation's place, for the marks
	Examples  []MappingExample
}

// MappingExample is one worked example of a mapping: what is sent from
// our values, and what is kept from their answer; HasSent and HasKept say
// which it gives.
type MappingExample struct {
	Label   string
	Input   []Field
	Sent    []Field
	Answer  []Field
	Kept    []Field
	HasSent bool
	HasKept bool
}

// Sends and Keeps say whether any example of the mapping gives sent or
// kept.
func (m Mapping) Sends() bool {
	for _, ex := range m.Examples {
		if ex.HasSent {
			return true
		}
	}
	return false
}

func (m Mapping) Keeps() bool {
	for _, ex := range m.Examples {
		if ex.HasKept {
			return true
		}
	}
	return false
}

// Build reads a request into a Suite.
func Build(r *Request) Suite {
	g := &walker{spec: r.Specification, mappings: map[string]string{}}
	for _, impl := range r.Implementations {
		if m, ok := impl.Content["mappings"].(map[string]any); ok {
			for k, v := range m {
				if target := str(v, "target"); target != "" {
					g.mappings[k] = target
				}
			}
		}
	}
	info, _ := r.Specification["info"].(map[string]any)
	s := Suite{Title: oneLine(str(info, "title")), Version: str(info, "version"), Meta: r.Specarch, Root: relRoot(r.Root, r.Output)}
	tests, _ := r.Specification["tests"].(map[string]any)
	for _, name := range sortedKeys(tests) {
		t, _ := tests[name].(map[string]any)
		s.Tests = append(s.Tests, g.test(name, t))
	}
	algorithms, _ := r.Specification["algorithms"].(map[string]any)
	for _, name := range sortedKeys(algorithms) {
		a, _ := algorithms[name].(map[string]any)
		if alg, ok := g.algorithm(name, a); ok {
			s.Algorithms = append(s.Algorithms, alg)
		}
	}
	s.Mappings = g.callMappings()
	s.Draft = r.Draft
	var elements []string
	for _, t := range s.Tests {
		elements = append(elements, "/tests/"+mark.Escape(t.Name))
	}
	for _, a := range s.Algorithms {
		elements = append(elements, "/algorithms/"+mark.Escape(a.Name))
	}
	s.Marks = map[string][]string{}
	for ptr, ps := range mark.Place(r.Problems, elements, []string{"/tests", "/algorithms"}) {
		for _, p := range ps {
			s.Marks[ptr] = append(s.Marks[ptr], mark.Line(p))
		}
	}
	return s
}

// setting is a string the implementation files give the target under
// settings, or "".
func setting(r *Request, key string) string {
	for _, impl := range r.Implementations {
		if v, ok := impl.Settings[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// framework checks the testing framework every implementation file names
// against the ones a plug-in writes for: the framework to write for (the
// first named, or "" when none is), or a refusal naming the first file
// whose framework is not one of them.
func framework(r *Request, plugin string, supported ...string) (string, *Response) {
	chosen := ""
	for _, impl := range r.Implementations {
		testing, _ := impl.Content["testing"].(map[string]any)
		f := str(testing, "framework")
		if f == "" {
			continue
		}
		if !slices.Contains(supported, f) {
			refused := refusedFramework(impl.File, f, plugin, strings.Join(supported, " or "))
			return "", &refused
		}
		if chosen == "" {
			chosen = f
		}
	}
	return chosen, nil
}

type walker struct {
	spec     map[string]any
	mappings map[string]string
}

// mapping is the implementation's name for a design element, or the
// element's own name.
func (g *walker) mapping(pointer, own string) string {
	if m := g.mappings[pointer]; m != "" {
		return m
	}
	return own
}

func (g *walker) test(name string, t map[string]any) Test {
	out := Test{Name: name}
	if reason := str(t, "notApplicable"); reason != "" {
		out.NotApplicable = oneLine(reason)
		return out
	}
	out.Subject, out.Scenario = subjectOf(t), str(t, "scenario")
	out.Given, out.When, out.Then = oneLine(str(t, "given")), oneLine(str(t, "when")), oneLine(str(t, "then"))
	fixture, _ := t["fixture"].(map[string]any)
	// The caller is a role or public as a word, or an object with the
	// role and values for the caller's properties (meta-model 0.3).
	switch c := fixture["caller"].(type) {
	case string:
		if c != "public" {
			out.Caller = c
		}
	case map[string]any:
		if role := str(c, "role"); role != "public" {
			out.Caller = role
		}
		for _, name := range sortedKeys(c) {
			if name != "role" && !strings.HasPrefix(name, "x-") {
				out.CallerProperties = append(out.CallerProperties, CallerProperty{Name: name, Value: c[name]})
			}
		}
	}
	for _, ent := range sortedKeys(fixture) {
		if ent == "caller" || strings.HasPrefix(ent, "x-") {
			continue
		}
		recs, _ := fixture[ent].([]any)
		for _, rec := range recs {
			m, _ := rec.(map[string]any)
			out.Inserts = append(out.Inserts, Stored{g.mapping("#/entities/"+ent, ent), g.record(m, g.entityFields(ent))})
		}
	}
	input, hasInput := t["input"].(map[string]any)
	expect, hasExpect := t["expect"].(map[string]any)
	if hasInput || hasExpect {
		out.Call = g.call(t, input)
	}
	if out.Call == nil {
		return out
	}
	e := &out.Expect
	if v, ok := expect["status"]; ok {
		e.Status = text(v)
	}
	if body, ok := expect["body"].(map[string]any); ok {
		e.Body, e.HasBody = g.record(body, g.responseFields(t)), true
	}
	if v, ok := expect["exit"]; ok {
		e.Exit = text(v)
	}
	if lines, ok := expect["standardOutput"].([]any); ok {
		e.HasOutput = true
		for _, l := range lines {
			e.Output = append(e.Output, text(l))
		}
	}
	if state, ok := expect["state"].(map[string]any); ok {
		for _, ent := range sortedKeys(state) {
			recs, _ := state[ent].([]any)
			for _, rec := range recs {
				m, _ := rec.(map[string]any)
				e.State = append(e.State, Stored{g.mapping("#/entities/"+ent, ent), g.record(m, g.entityFields(ent))})
			}
		}
	}
	if emits, ok := expect["emits"].([]any); ok {
		for _, m := range emits {
			e.Emits = append(e.Emits, text(m))
		}
	}
	e.EmitsNothing = expect["emitsNothing"] == true
	return out
}

// call is the call a test makes, or nil when its subject is not an
// operation, a command or a page.
func (g *walker) call(t map[string]any, input map[string]any) *Call {
	switch {
	case str(t, "operation") != "":
		id := str(t, "operation")
		p, method, pointer, fields := g.operation(id)
		if p == "" {
			return nil
		}
		op, _ := mapAt(g.spec, "paths", p, method).(map[string]any)
		stream, _ := op["stream"].(map[string]any)
		return &Call{Kind: "request", Mapping: g.mapping(pointer, id), Method: strings.ToUpper(method), Path: p, Input: g.record(input, fields),
			Handshake: str(op, "handshake"), Stream: text(stream["direction"])}
	case str(t, "command") != "":
		name := str(t, "command")
		cmd, _ := mapAt(g.spec, "commands", name).(map[string]any)
		argFields := map[string]map[string]any{}
		if args, ok := cmd["arguments"].([]any); ok {
			for _, a := range args {
				m, _ := a.(map[string]any)
				s, _ := m["schema"].(map[string]any)
				argFields[str(m, "name")] = s
			}
		}
		c := &Call{Kind: "run", Mapping: g.mapping("#/commands/"+escape(name), name), Command: name}
		args, _ := input["arguments"].(map[string]any)
		for _, k := range sortedKeys(args) {
			vals, ok := args[k].([]any)
			if !ok {
				vals = []any{args[k]}
			}
			a := Argument{Name: k}
			for _, v := range vals {
				a.Values = append(a.Values, g.value(v, argFields[k]))
			}
			c.Arguments = append(c.Arguments, a)
		}
		optFields := map[string]map[string]any{}
		if opts, ok := cmd["options"].(map[string]any); ok {
			for k, o := range opts {
				m, _ := o.(map[string]any)
				s, _ := m["schema"].(map[string]any)
				optFields[k] = s
			}
		}
		opts, _ := input["options"].(map[string]any)
		c.Options = g.record(opts, optFields)
		return c
	case str(t, "page") != "":
		name := str(t, "page")
		page, _ := mapAt(g.spec, "pages", name).(map[string]any)
		return &Call{Kind: "open", Mapping: g.mapping("#/pages/"+name, name), Page: name, Route: str(page, "route"), Input: g.record(input, nil)}
	}
	return nil
}

// operation finds an operation by its operationId: its path, method,
// pointer, and the schemas of its parameters and body fields.
func (g *walker) operation(id string) (p, method, pointer string, fields map[string]map[string]any) {
	paths, _ := g.spec["paths"].(map[string]any)
	for _, pathKey := range sortedKeys(paths) {
		item, _ := paths[pathKey].(map[string]any)
		for _, m := range []string{"get", "post", "put", "patch", "delete"} {
			op, ok := item[m].(map[string]any)
			if !ok || str(op, "operationId") != id {
				continue
			}
			fields = map[string]map[string]any{}
			for _, list := range []any{item["parameters"], op["parameters"]} {
				params, _ := list.([]any)
				for _, prm := range params {
					pm, _ := prm.(map[string]any)
					s, _ := pm["schema"].(map[string]any)
					fields[str(pm, "name")] = s
				}
			}
			if rb, ok := op["requestBody"].(map[string]any); ok {
				content, _ := rb["content"].(map[string]any)
				for _, ct := range sortedKeys(content) {
					c, _ := content[ct].(map[string]any)
					schema, _ := c["schema"].(map[string]any)
					for k, v := range g.schemaFields(schema) {
						fields[k] = v
					}
					break
				}
			}
			return pathKey, m, "#/paths/" + escape(pathKey) + "/" + m, fields
		}
	}
	return "", "", "", nil
}

// responseFields are the fields of the entity a test's operation answers
// with, for its expected status or its first success.
func (g *walker) responseFields(t map[string]any) map[string]map[string]any {
	p, method, _, _ := g.operation(str(t, "operation"))
	op, _ := mapAt(g.spec, "paths", p, method).(map[string]any)
	responses, _ := op["responses"].(map[string]any)
	expect, _ := t["expect"].(map[string]any)
	status := text(expect["status"])
	for _, code := range sortedKeys(responses) {
		if status != "" && code != status || status == "" && !strings.HasPrefix(code, "2") {
			continue
		}
		r, _ := responses[code].(map[string]any)
		content, _ := r["content"].(map[string]any)
		for _, ct := range sortedKeys(content) {
			c, _ := content[ct].(map[string]any)
			schema, _ := c["schema"].(map[string]any)
			if items, ok := schema["items"].(map[string]any); ok {
				schema = items
			}
			return g.schemaFields(schema)
		}
	}
	return nil
}

// schemaFields are an object schema's fields, through a $ref to an entity
// or a schema of the specification.
func (g *walker) schemaFields(schema map[string]any) map[string]map[string]any {
	if ref := str(schema, "$ref"); strings.HasPrefix(ref, "#/entities/") {
		return g.entityFields(strings.TrimPrefix(ref, "#/entities/"))
	} else if strings.HasPrefix(ref, "#/schemas/") {
		s, _ := mapAt(g.spec, "schemas", strings.TrimPrefix(ref, "#/schemas/")).(map[string]any)
		return g.schemaFields(map[string]any{"properties": s["properties"]})
	}
	out := map[string]map[string]any{}
	props, _ := schema["properties"].(map[string]any)
	for k, v := range props {
		m, _ := v.(map[string]any)
		out[k] = m
	}
	return out
}

func (g *walker) entityFields(ent string) map[string]map[string]any {
	e, _ := mapAt(g.spec, "entities", ent).(map[string]any)
	return g.schemaFields(map[string]any{"properties": e["properties"]})
}

// algorithm reads an algorithm's worked examples; false when it has none.
func (g *walker) algorithm(name string, a map[string]any) (Algorithm, bool) {
	exs, _ := a["examples"].([]any)
	if len(exs) == 0 {
		return Algorithm{}, false
	}
	inputs := map[string]map[string]any{}
	if in, ok := a["inputs"].(map[string]any); ok {
		for k, v := range in {
			m, _ := v.(map[string]any)
			inputs[k] = m
		}
	}
	output, _ := a["output"].(map[string]any)
	out := Algorithm{Name: name, Mapping: g.mapping("#/algorithms/"+name, name)}
	for i, e := range exs {
		ex, _ := e.(map[string]any)
		label := str(ex, "name")
		if label == "" {
			label = fmt.Sprintf("example %d", i+1)
		}
		in, _ := ex["inputs"].(map[string]any)
		out.Examples = append(out.Examples, Example{Label: label, Input: g.record(in, inputs), Expected: g.value(ex["expected"], output)})
	}
	return out, true
}

// callMappings reads every operation's calls whose mapping has worked
// examples, in the order of the paths and their methods.
func (g *walker) callMappings() []Mapping {
	var out []Mapping
	paths, _ := g.spec["paths"].(map[string]any)
	for _, pathKey := range sortedKeys(paths) {
		item, _ := paths[pathKey].(map[string]any)
		for _, method := range []string{"get", "post", "put", "patch", "delete"} {
			op, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			id := str(op, "operationId")
			calls, _ := op["calls"].([]any)
			var refs []string
			for _, c := range calls {
				if s, ok := c.(string); ok {
					refs = append(refs, s)
				} else {
					refs = append(refs, str(c.(map[string]any), "call"))
				}
			}
			names := callNames(refs)
			_, _, pointer, ours := g.operation(id)
			guard, _ := op["guard"].(map[string]any)
			for k := range g.entityFields(str(guard, "entity")) {
				if ours[k] == nil {
					ours[k] = g.entityFields(str(guard, "entity"))[k]
				}
			}
			for i, c := range calls {
				entry, _ := c.(map[string]any)
				mapping, _ := entry["mapping"].(map[string]any)
				exs, _ := mapping["examples"].([]any)
				if len(exs) == 0 {
					continue
				}
				request, answer := g.dependencyFields(refs[i])
				m := Mapping{Operation: id, Mapping: g.mapping(pointer, id), Call: names[i], Reference: refs[i], Pointer: strings.TrimPrefix(pointer, "#")}
				for j, e := range exs {
					ex, _ := e.(map[string]any)
					label := str(ex, "name")
					if label == "" {
						label = fmt.Sprintf("example %d", j+1)
					}
					in, _ := ex["inputs"].(map[string]any)
					sent, hasSent := ex["sent"].(map[string]any)
					got, _ := ex["answer"].(map[string]any)
					kept, hasKept := ex["kept"].(map[string]any)
					m.Examples = append(m.Examples, MappingExample{Label: label, Input: g.record(in, ours), Sent: g.record(sent, request),
						Answer: g.record(got, answer), Kept: g.record(kept, g.entityFields(str(guard, "entity"))), HasSent: hasSent, HasKept: hasKept})
				}
				out = append(out, m)
			}
		}
	}
	return out
}

// dependencyFields are the fields of the request and of the first answer
// that succeeds of an operation a dependency declares, for a calls
// entry that names one as <dependency>#/operations/<name>.
func (g *walker) dependencyFields(ref string) (request, answer map[string]map[string]any) {
	dep, name, ok := strings.Cut(ref, "#/operations/")
	if !ok {
		return nil, nil
	}
	op, _ := mapAt(g.spec, "dependencies", dep, "operations", name).(map[string]any)
	req, _ := op["request"].(map[string]any)
	request = g.schemaFields(map[string]any{"properties": req["properties"]})
	responses, _ := op["responses"].(map[string]any)
	for _, status := range sortedKeys(responses) {
		if strings.HasPrefix(status, "2") {
			r, _ := responses[status].(map[string]any)
			answer = g.schemaFields(map[string]any{"properties": r["properties"]})
			break
		}
	}
	return request, answer
}

// callNames are calls entries by name as the derived cases name them: the
// dependency and the operation called, with the call's place when two
// still read the same.
func callNames(refs []string) []string {
	name := func(ref string) string {
		dep, ptr, ok := strings.Cut(ref, "#")
		switch {
		case !ok:
			return ref
		case strings.HasPrefix(ptr, "/operations/"):
			return dep + " " + strings.TrimPrefix(ptr, "/operations/")
		}
		tokens := strings.Split(strings.TrimPrefix(ptr, "/"), "/")
		if len(tokens) != 3 || tokens[0] != "paths" {
			return dep
		}
		path := strings.NewReplacer("~1", " ", "~0", " ", "{", "", "}", "").Replace(tokens[1])
		return strings.Join(append([]string{dep, tokens[2]}, strings.Fields(path)...), " ")
	}
	named := map[string]int{}
	for _, r := range refs {
		named[name(r)]++
	}
	var out []string
	for i, r := range refs {
		n := name(r)
		if named[n] > 1 {
			n += fmt.Sprintf(" call %d", i+1)
		}
		out = append(out, n)
	}
	return out
}

// record is values by name, sorted, each with the type its field gives.
func (g *walker) record(m map[string]any, fields map[string]map[string]any) []Field {
	var out []Field
	for _, k := range sortedKeys(m) {
		out = append(out, Field{k, g.value(m[k], fields[k])})
	}
	return out
}

func (g *walker) value(v any, field map[string]any) Value {
	if v == nil {
		return Value{Null: true, Kind: "null"}
	}
	out := Value{Kind: kind(field, v), Text: text(v)}
	switch x := v.(type) {
	case map[string]any:
		var parts map[string]map[string]any
		if field != nil {
			parts = g.schemaFields(field)
			// A map's members beyond its properties take the type of its
			// values.
			if values, ok := field["additionalProperties"].(map[string]any); ok {
				for k := range x {
					if parts[k] == nil {
						if parts == nil {
							parts = map[string]map[string]any{}
						}
						parts[k] = values
					}
				}
			}
		}
		out.Parts = g.record(x, parts)
	case []any:
		items, _ := field["items"].(map[string]any)
		for _, item := range x {
			out.Items = append(out.Items, g.value(item, items))
		}
	}
	return out
}

// kind is a field's type in the expression language, or, when the field is
// not known, the type of the value as written.
func kind(field map[string]any, v any) string {
	if field == nil {
		switch v.(type) {
		case json.Number:
			return "double"
		case bool:
			return "bool"
		case map[string]any:
			return "object"
		case []any:
			return "list"
		}
		return "string"
	}
	if strings.HasPrefix(str(field, "$ref"), "#/enums/") {
		return "enum"
	}
	if strings.HasPrefix(str(field, "$ref"), "#/entities/") || strings.HasPrefix(str(field, "$ref"), "#/schemas/") {
		return "object"
	}
	if _, ok := field["enum"]; ok {
		return "enum"
	}
	switch str(field, "format") {
	case "int32", "int64":
		return "int"
	case "uint64":
		return "uint"
	case "double":
		return "double"
	case "decimal":
		return "decimal"
	case "date":
		return "date"
	case "date-time":
		return "timestamp"
	case "duration":
		return "duration"
	case "time":
		return "time"
	case "byte", "binary":
		return "bytes"
	}
	switch str(field, "type") {
	case "integer":
		return "int"
	case "number":
		return "double"
	case "boolean":
		return "bool"
	case "array":
		return "list"
	case "object":
		return "object"
	}
	return "string"
}

func subjectOf(t map[string]any) string {
	for _, k := range []string{"operation", "command", "page", "job", "flow", "workflow", "requirement"} {
		if v := str(t, k); v != "" {
			return k + " " + v
		}
	}
	ent := str(t, "entity")
	if c := str(t, "constraint"); c != "" {
		return ent + " constraint " + c
	}
	if tr, ok := t["transition"].(map[string]any); ok {
		return fmt.Sprintf("%s transition %s to %s", ent, str(tr, "from"), str(tr, "to"))
	}
	return ent + " state machine"
}

func mapAt(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

func str(m any, key string) string {
	mm, _ := m.(map[string]any)
	s, _ := mm[key].(string)
	return s
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TransportNote says what the harness does for a request that travels in a
// handshake's envelope or streams a file, which the test leaves to it: the
// test names the call, the harness establishes the handshake first and
// seals the request, or streams the body. "" for a plain call.
func (c *Call) TransportNote() string {
	var parts []string
	if c.Handshake != "" {
		parts = append(parts, "travels in the envelope of handshake "+c.Handshake+": the harness establishes it first and seals the request")
	}
	switch c.Stream {
	case "upload":
		parts = append(parts, "streams the request body as a file: the harness streams the input's body")
	case "download":
		parts = append(parts, "streams the response body as a file: the harness reads the stream as the body")
	}
	if len(parts) == 0 {
		return ""
	}
	return "The request " + strings.Join(parts, "; and ") + "."
}
