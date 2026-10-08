package gentests

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
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
	Inserts       []Stored
	Call          *Call // nil when the design gives no call to make
	Expect        Expect
}

// Stored is a record of an entity, through the entity's mapping.
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
// and its text, or null.
type Value struct {
	Null bool
	Kind string
	Text string
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
	if caller := str(fixture, "caller"); caller != "public" {
		out.Caller = caller
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
		return &Call{Kind: "request", Mapping: g.mapping(pointer, id), Method: strings.ToUpper(method), Path: p, Input: g.record(input, fields)}
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

// schemaFields are an object schema's fields, through a $ref to an entity.
func (g *walker) schemaFields(schema map[string]any) map[string]map[string]any {
	if ref := str(schema, "$ref"); strings.HasPrefix(ref, "#/entities/") {
		return g.entityFields(strings.TrimPrefix(ref, "#/entities/"))
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
	return Value{Kind: kind(field, v), Text: text(v)}
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
		}
		return "string"
	}
	if strings.HasPrefix(str(field, "$ref"), "#/enums/") {
		return "enum"
	}
	if strings.HasPrefix(str(field, "$ref"), "#/entities/") {
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
