package extract

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// A schema's or an enum's name, as the design schema takes it.
var pascalWord = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

// jsStep is one call of a validation library's chain: z.string().min(1)
// is the steps string and min(1).
type jsStep struct {
	name string
	args []jsValue
}

// schemaChain flattens a value into the calls of a chain from its root
// name: the library the root is imported from, the root's imported name,
// and each call in order. ok is false when it is not such a chain.
func (js *jsReader) schemaChain(v *jsValue) (lib string, steps []jsStep, ok bool) {
	for v != nil && v.Call != nil {
		c := v.Call
		if c.Callee.Member != nil {
			steps = append([]jsStep{{name: c.Callee.Member.Name, args: c.Arguments}}, steps...)
			v = &c.Callee.Member.Object
			if v.Member != nil && v.Member.Name == "coerce" && v.Member.Object.Name != nil {
				// z.coerce.date(): a coercion before the check.
				steps = append(steps, jsStep{name: "coerce"})
				v = &v.Member.Object
			}
			if v.Name != nil {
				// z.string(): the root is the namespace.
				imported, known := js.fromLibrary(v, "zod", "yup", "joi", "@hapi/joi")
				if !known {
					return "", nil, false
				}
				m, _ := js.moduleOf(v)
				lib = jsLibrary(m)
				if imported != "*" && imported != "default" && imported != "z" && imported != "Joi" && imported != "yup" {
					return "", nil, false
				}
				return lib, steps, true
			}
			continue
		}
		if c.Callee.Name != nil {
			// string() imported from yup by name.
			imported, known := js.fromLibrary(&c.Callee, "zod", "yup", "joi", "@hapi/joi")
			if !known {
				return "", nil, false
			}
			m, _ := js.moduleOf(&c.Callee)
			steps = append([]jsStep{{name: imported, args: c.Arguments}}, steps...)
			return jsLibrary(m), steps, true
		}
		return "", nil, false
	}
	return "", nil, false
}

// validatorName is the schema or enum name a variable holding a
// validation schema gives: newBookingSchema is NewBooking.
func validatorName(variable string) string {
	n := variable
	for _, s := range []string{"Schema", "Validator", "Validation", "Shape"} {
		if strings.HasSuffix(n, s) && len(n) > len(s) {
			n = strings.TrimSuffix(n, s)
			break
		}
	}
	if n == "" {
		return ""
	}
	return strings.ToUpper(n[:1]) + n[1:]
}

// readValidators writes each object schema of zod, yup or joi that a
// module-level variable holds as a schema, and each enum of strings one
// holds as an enum, named after the variable.
func (js *jsReader) readValidators() {
	n := 0
	var held []*jsFact
	for _, vr := range js.dump.Facts {
		if vr.Kind != "variable" || vr.Within != nil || vr.Name == "" || vr.Value == nil {
			continue
		}
		lib, steps, ok := js.schemaChain(vr.Value)
		if !ok || len(steps) == 0 {
			continue
		}
		base := steps[0].name
		if base == "object" || base == "enum" || base == "nativeEnum" || (lib != "zod" && base == "string" && hasStep(steps, "oneOf", "valid")) {
			n++
			held = append(held, vr)
		}
	}
	if n == 0 {
		return
	}
	for _, vr := range held {
		name := validatorName(vr.Name)
		pos := (&jsPos{File: vr.File, Line: vr.Line, Column: vr.Column}).key()
		if !pascalWord.MatchString(name) {
			js.gap(js.clause(vr), []string{"schemas"}, "", "the validation schema %s: no PascalCase name can be made from it; left out", vr.Name)
			continue
		}
		if js.schemaFrom[name] {
			js.question("must", fmt.Sprintf("Two validation schemas of the files read are named %s; %s at %s is the second. Which name does each take?", name, vr.Name, js.clause(vr)),
				[]string{"#/schemas/" + name}, "A schema has one name, and two would become one.", js.at(js.clause(vr), "Declares "+vr.Name+"."))
			continue
		}
		js.validators[pos] = name
		js.schemaFrom[name] = true
	}
	for _, vr := range held {
		pos := (&jsPos{File: vr.File, Line: vr.Line, Column: vr.Column}).key()
		name := js.validators[pos]
		if name == "" {
			continue
		}
		lib, steps, _ := js.schemaChain(vr.Value)
		clause := js.clause(vr)
		at := "#/schemas/" + name
		if steps[0].name != "object" {
			f := js.validatorField(lib, steps, "#/enums/"+name)
			values := scalarList(child(f.node, "enum"))
			if len(values) == 0 {
				js.gap(clause, []string{"enums"}, "", "the %s enum %s holds no value an enum can hold; left out", lib, vr.Name)
				continue
			}
			if js.enums == nil {
				js.enums = mapping()
			}
			set(js.enums, name, mapping("type", "string", "enum", values, "origin", "stated",
				"cites", []*yaml.Node{js.at(clause, fmt.Sprintf("%s is a %s enum of %s.", vr.Name, lib, strings.Join(values, ", ")))}))
			continue
		}
		el := js.validatorObject(lib, steps, at, clause, vr.Name)
		if el == nil {
			continue
		}
		set(el, "origin", "stated")
		set(el, "cites", []*yaml.Node{js.at(clause, fmt.Sprintf("%s is a %s object schema.", vr.Name, lib))})
		if js.schemas == nil {
			js.schemas = mapping()
		}
		set(js.schemas, name, el)
		js.counts["validators"]++
	}
	js.res.say("validators: counted %s: every module-level variable that holds an object schema or an enum of zod, yup or joi", plural(n, "validation schema"))
}

func hasStep(steps []jsStep, names ...string) bool {
	for _, s := range steps {
		for _, n := range names {
			if s.name == n {
				return true
			}
		}
	}
	return false
}

// validatorObject writes an object schema of a validation library: its
// properties, and the ones required.
func (js *jsReader) validatorObject(lib string, steps []jsStep, at, clause, variable string) *yaml.Node {
	var shape *jsValue
	if len(steps[0].args) > 0 {
		shape = &steps[0].args[0]
	}
	for _, s := range steps[1:] {
		if (s.name == "shape" || s.name == "keys" || s.name == "extend") && len(s.args) > 0 {
			if shape == nil {
				shape = &s.args[0]
			} else {
				js.gap(clause, []string{at}, "", "the schema %s is extended with %s, which the reader does not join; read from its first shape", variable, s.name)
			}
		}
	}
	if shape == nil || shape.Object == nil {
		js.gap(clause, []string{"schemas"}, "", "the %s object schema %s has no literal shape; left out", lib, variable)
		return nil
	}
	props := mapping()
	var required []string
	for _, p := range shape.Object {
		if p.Key == nil {
			js.question("must", fmt.Sprintf("The schema %s at %s has a property %s, which is not a literal name. Which properties does it hold?", variable, clause, (&jsValue{Text: p.Computed}).describe()),
				[]string{at}, "A property named at run time, or spread from another value, is not known by syntax.", js.at(clause, "Declares "+variable+"."))
			continue
		}
		name := *p.Key
		fat := at + "/properties/" + name
		pclause := clause
		if p.Line > 0 {
			pclause = fmt.Sprintf("%s:%d", strings.SplitN(clause, ":", 2)[0], p.Line)
		}
		if !memberNameWord.MatchString(name) {
			js.gap(clause, []string{at}, "", "%s.%s: the property's name is not a camelCase name a field can take; left out", variable, name)
			continue
		}
		f, req := js.validatorValue(p.Value, fat)
		if f.held != "" {
			js.gap(pclause, []string{at}, "", "%s.%s: %s; left out", variable, name, f.held)
			continue
		}
		node := f.node
		if f.nullable {
			node = nullable(node)
		}
		set(props, name, node)
		if req && !f.nullable {
			required = append(required, name)
		}
		for _, a := range f.asks {
			js.question(a.priority, fmt.Sprintf("%s.%s at %s: %s", variable, name, pclause, a.text), []string{fat + a.key}, a.why, js.at(pclause, "Declares "+name+"."))
		}
	}
	el := mapping("type", "object")
	if len(props.Content) > 0 {
		set(el, "properties", props)
	}
	if len(required) > 0 {
		set(el, "required", required)
	}
	return el
}

// validatorValue writes one property's validation as a field, and says
// whether it is required.
func (js *jsReader) validatorValue(v *jsValue, at string) (swField, bool) {
	if v != nil && v.Name != nil {
		if vr := js.variableOf(v); vr != nil {
			pos := (&jsPos{File: vr.File, Line: vr.Line, Column: vr.Column}).key()
			if name := js.validators[pos]; name != "" {
				section := "schemas"
				if child(js.enums, name) != nil || (vr.Value != nil && !strings.Contains(vr.Value.describe(), "object")) {
					if _, steps, ok := js.schemaChain(vr.Value); ok && steps[0].name != "object" {
						section = "enums"
					}
				}
				return swField{node: flow(mapping("$ref", "#/"+section+"/"+name)), model: name}, true
			}
		}
	}
	lib, steps, ok := js.schemaChain(v)
	if !ok {
		return swField{node: flow(mapping()), asks: []swAsk{{"must", "", fmt.Sprintf("its validation is %s, which is not a schema of zod, yup or joi the reader reads. Which type is it?", v.describe()), "A property's type is read from a validation library's own calls, and this one is not."}}}, true
	}
	f := js.validatorField(lib, steps, at)
	required := lib == "zod"
	for _, s := range steps {
		switch s.name {
		case "optional", "nullish":
			required = false
		case "required", "exist", "defined":
			required = lib != "zod"
		case "default":
			required = false
		}
	}
	if lib == "zod" && hasStep(steps, "nullish") {
		f.nullable = true
	}
	return f, required
}

// validatorField writes a chain of a validation library as a field.
func (js *jsReader) validatorField(lib string, steps []jsStep, at string) swField {
	base := steps[0].name
	node := mapping()
	var f swField
	isNumber, isInteger := false, false
	switch base {
	case "string", "email", "uuid", "url", "iso":
		replaceKey(node, "type", "string")
	case "number":
		replaceKey(node, "type", "number")
		isNumber = true
	case "int":
		replaceKey(node, "type", "integer")
		isNumber, isInteger = true, true
	case "boolean", "bool":
		replaceKey(node, "type", "boolean")
	case "date":
		replaceKey(node, "type", "string")
		replaceKey(node, "format", "date-time")
		f.asks = append(f.asks, swAsk{"should", "", fmt.Sprintf("%s's date() takes a Date, which JSON carries as an ISO 8601 string by Date's toJSON. Is it a date-time string on the wire?", lib), "A JSON body holds no Date; how the value is sent is the sender's choice."})
	case "enum", "nativeEnum", "mixed":
		replaceKey(node, "type", "string")
	case "literal":
		replaceKey(node, "type", "string")
		if len(steps[0].args) == 1 {
			if s, ok := steps[0].args[0].stringLiteral(); ok {
				replaceKey(node, "enum", []string{s})
			}
		}
	case "array":
		var item *jsValue
		if len(steps[0].args) > 0 {
			item = &steps[0].args[0]
		}
		for _, s := range steps[1:] {
			if (s.name == "of" || s.name == "items") && len(s.args) > 0 {
				item = &s.args[0]
			}
		}
		if item == nil {
			return swField{node: flow(mapping("type", "array")), asks: []swAsk{{"must", "/items", "the list's items have no validation the reader reads. Which type is each item?", "A list's items have a type, and the schema states none."}}}
		}
		it, _ := js.validatorValue(item, at+"/items")
		if it.held != "" {
			return it
		}
		f = swField{node: flow(mapping("type", "array", "items", it.node)), model: it.model, array: true}
		for _, a := range it.asks {
			a.key = "/items" + a.key
			f.asks = append(f.asks, a)
		}
		return f
	case "object":
		return swField{held: "an object nested in a property, which a field holds only as a schema of its own"}
	case "union", "discriminatedUnion", "intersection", "alternatives", "record", "map", "tuple", "any", "unknown", "lazy", "when":
		return swField{held: fmt.Sprintf("%s's %s(), which a field does not hold", lib, base)}
	default:
		return swField{node: flow(mapping()), asks: []swAsk{{"must", "", fmt.Sprintf("its validation starts with %s's %s(), which the reader does not read. Which type is it?", lib, base), "A property's type is read from the validation calls the reader knows."}}}
	}
	if base == "enum" && len(steps[0].args) > 0 && steps[0].args[0].Array != nil {
		var values []string
		for _, a := range steps[0].args[0].Array {
			if s, ok := a.stringLiteral(); ok {
				values = append(values, s)
			}
		}
		replaceKey(node, "enum", values)
	}
	switch base {
	case "email":
		replaceKey(node, "format", "email")
	case "uuid":
		replaceKey(node, "format", "uuid")
	case "url":
		replaceKey(node, "format", "uri")
	case "iso":
		replaceKey(node, "format", "date-time")
	}
	for _, s := range steps[1:] {
		num := func() (string, bool) {
			if len(s.args) == 0 || s.args[0].Number == nil {
				return "", false
			}
			return *s.args[0].Number, true
		}
		switch s.name {
		case "email":
			replaceKey(node, "format", "email")
		case "uuid", "guid":
			replaceKey(node, "format", "uuid")
		case "url", "uri":
			replaceKey(node, "format", "uri")
		case "datetime", "iso":
			replaceKey(node, "format", "date-time")
			if scalar(child(node, "type")) == "string" && base == "date" {
				f.asks = nil
			}
		case "date":
			if base == "string" {
				replaceKey(node, "format", "date")
			}
		case "int", "integer":
			replaceKey(node, "type", "integer")
			isInteger = true
		case "positive":
			replaceKey(node, "exclusiveMinimum", int64(0))
		case "nonnegative":
			replaceKey(node, "minimum", int64(0))
		case "min", "max", "length", "gte", "lte", "greaterThan", "lessThan":
			n, ok := num()
			if !ok {
				f.asks = append(f.asks, swAsk{"should", "", fmt.Sprintf("its %s() is given %s, which is not a literal number. Which bound is it?", s.name, describeArgs(s.args)), "A bound computed at run time is not known by syntax."})
				continue
			}
			value := yamlNumber(n)
			text := scalar(child(node, "type")) == "string"
			switch s.name {
			case "min", "gte":
				if text {
					replaceKey(node, "minLength", value)
				} else {
					replaceKey(node, "minimum", value)
				}
			case "max", "lte":
				if text {
					replaceKey(node, "maxLength", value)
				} else {
					replaceKey(node, "maximum", value)
				}
			case "length":
				replaceKey(node, "minLength", value)
				replaceKey(node, "maxLength", value)
			case "greaterThan":
				replaceKey(node, "exclusiveMinimum", value)
			case "lessThan":
				replaceKey(node, "exclusiveMaximum", value)
			}
		case "oneOf", "valid":
			var values []string
			var list []jsValue
			if len(s.args) == 1 && s.args[0].Array != nil {
				list = s.args[0].Array
			} else {
				list = s.args
			}
			for _, a := range list {
				if str, ok := a.stringLiteral(); ok {
					values = append(values, str)
				} else if a.Null != nil {
					f.nullable = true
				}
			}
			replaceKey(node, "enum", values)
		case "nullable", "allow":
			if s.name == "nullable" || (len(s.args) > 0 && s.args[0].Null != nil) {
				f.nullable = true
			}
		case "default":
			if len(s.args) == 1 {
				if lit := jsLiteralNode(&s.args[0]); lit != nil {
					replaceKey(node, "default", lit)
				}
			}
		case "describe", "description":
			if len(s.args) == 1 {
				if str, ok := s.args[0].stringLiteral(); ok {
					replaceKey(node, "description", str)
				}
			}
		case "trim", "toLowerCase", "lowercase", "toUpperCase", "uppercase", "strict", "label", "meta", "optional", "required", "nullish", "exist", "defined", "of", "items", "shape", "keys", "strip", "passthrough":
		case "coerce":
			f.asks = append(f.asks, swAsk{"should", "", "zod's coerce turns whatever is sent into its type before the check, so the wire may carry another type. Which type does a sender send?", "A coercion accepts more than the type it checks, and the specification says what is sent."})
		case "refine", "superRefine", "transform", "test", "custom", "pipe", "preprocess", "regex", "matches", "pattern", "when":
			f.asks = append(f.asks, swAsk{"should", "", fmt.Sprintf("it is checked with %s(), a rule the reader does not write. What does the rule allow?", s.name), "A rule given as code or a pattern is not one the field subset says by syntax."})
		default:
			f.asks = append(f.asks, swAsk{"should", "", fmt.Sprintf("it is checked with %s's %s(), which the reader does not read. What does it allow?", lib, s.name), "The reader writes the rules of the validation calls it knows."})
		}
	}
	if isNumber && !widthHeld(scalar(child(node, "type")), scalar(child(node, "format")), node) {
		if isInteger && boundedInt32(node) {
			replaceKey(node, "format", "int32")
		} else {
			js.widths = append(js.widths, at+"/format")
		}
	}
	f.node = flow(node)
	return f
}

// boundedInt32 says whether an integer's literal bounds keep it inside
// a 32-bit integer.
func boundedInt32(n *yaml.Node) bool {
	low, err1 := strconv.ParseFloat(scalar(child(n, "minimum")), 64)
	high, err2 := strconv.ParseFloat(scalar(child(n, "maximum")), 64)
	return err1 == nil && err2 == nil && low >= -2147483648 && high <= 2147483647
}

// replaceKey sets a key of a mapping, replacing the value it has.
func replaceKey(m *yaml.Node, key string, v any) {
	if n := value(v); n != nil {
		setKey(m, key, n)
	}
}

func describeArgs(args []jsValue) string {
	var parts []string
	for i := range args {
		parts = append(parts, args[i].describe())
	}
	return strings.Join(parts, ", ")
}

func yamlNumber(s string) any {
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

// jsLiteralNode is a literal value as YAML, or nil.
func jsLiteralNode(v *jsValue) *yaml.Node {
	switch {
	case v.String != nil:
		return str(*v.String)
	case v.Number != nil:
		n := &yaml.Node{}
		_ = n.Encode(yamlNumber(*v.Number))
		return n
	case v.Boolean != nil:
		n := &yaml.Node{}
		_ = n.Encode(*v.Boolean)
		return n
	}
	return nil
}

// jsonSchemaField writes a JSON Schema object literal, as Fastify takes
// one, as a field.
func (js *jsReader) jsonSchemaField(v *jsValue, at string) swField {
	if v == nil || v.Object == nil {
		return swField{node: flow(mapping()), asks: []swAsk{{"must", "", fmt.Sprintf("its schema is %s, which is not a literal JSON Schema object. Which type is it?", v.describe()), "A schema computed at run time is not known by syntax."}}}
	}
	typ, _ := js.constString(v.prop("type"))
	node := mapping()
	var f swField
	switch typ {
	case "string", "boolean":
		replaceKey(node, "type", typ)
	case "integer", "number":
		replaceKey(node, "type", typ)
	case "array":
		it := js.jsonSchemaField(v.prop("items"), at+"/items")
		if it.held != "" {
			return it
		}
		f = swField{node: flow(mapping("type", "array", "items", it.node)), array: true}
		for _, a := range it.asks {
			a.key = "/items" + a.key
			f.asks = append(f.asks, a)
		}
		return f
	case "object":
		return swField{held: "an object nested in a property, which a field holds only as a schema of its own"}
	default:
		return swField{node: flow(mapping()), asks: []swAsk{{"must", "", "its JSON Schema gives no type the reader reads. Which type is it?", "A field has a type, and the schema states none."}}}
	}
	for _, k := range []string{"format", "pattern", "description"} {
		if s, ok := js.constString(v.prop(k)); ok {
			if k == "format" && !fieldFormats[s] {
				f.asks = append(f.asks, swAsk{"should", "/format", fmt.Sprintf("its format %s is not one the meta-model holds. Which format is it?", s), "The meta-model holds a fixed list of formats."})
				continue
			}
			replaceKey(node, k, s)
		}
	}
	for _, k := range []string{"minLength", "maxLength", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "minItems", "maxItems"} {
		if n := v.prop(k); n != nil && n.Number != nil {
			replaceKey(node, k, yamlNumber(*n.Number))
		}
	}
	if e := v.prop("enum"); e != nil && e.Array != nil {
		var values []string
		for _, a := range e.Array {
			if s, ok := a.stringLiteral(); ok {
				values = append(values, s)
			}
		}
		replaceKey(node, "enum", values)
	}
	if d := v.prop("default"); d != nil {
		if lit := jsLiteralNode(d); lit != nil {
			replaceKey(node, "default", lit)
		}
	}
	if n := v.prop("nullable"); n != nil && n.Boolean != nil && *n.Boolean {
		f.nullable = true
	}
	if typ == "integer" || typ == "number" {
		if !widthHeld(typ, scalar(child(node, "format")), node) {
			if typ == "integer" && boundedInt32(node) {
				replaceKey(node, "format", "int32")
			} else {
				js.widths = append(js.widths, at+"/format")
			}
		}
	}
	f.node = flow(node)
	return f
}

// jsonSchemaObject writes a JSON Schema object literal as a schema.
func (js *jsReader) jsonSchemaObject(v *jsValue, at, clause, what string) *yaml.Node {
	props := v.prop("properties")
	if typ, _ := js.constString(v.prop("type")); typ != "object" || props == nil || props.Object == nil {
		js.question("must", fmt.Sprintf("%s at %s is not a literal JSON Schema object with literal properties. What does it hold?", what, clause),
			[]string{at}, "A schema computed at run time, or of another type than object, is not one the reader writes as a schema.", js.at(clause, "Gives "+what+"."))
		return nil
	}
	var required []string
	if r := v.prop("required"); r != nil {
		for _, a := range r.Array {
			if s, ok := a.stringLiteral(); ok {
				required = append(required, s)
			}
		}
	}
	node := mapping()
	var req []string
	for _, p := range props.Object {
		if p.Key == nil || !memberNameWord.MatchString(*p.Key) {
			js.gap(clause, []string{at}, "", "%s: a property whose name is not a literal camelCase name; left out", what)
			continue
		}
		fat := at + "/properties/" + *p.Key
		pclause := clause
		if p.Line > 0 {
			pclause = fmt.Sprintf("%s:%d", strings.SplitN(clause, ":", 2)[0], p.Line)
		}
		f := js.jsonSchemaField(p.Value, fat)
		if f.held != "" {
			js.gap(pclause, []string{at}, "", "%s, property %s: %s; left out", what, *p.Key, f.held)
			continue
		}
		n := f.node
		if f.nullable {
			n = nullable(n)
		}
		set(node, *p.Key, n)
		if contains(required, *p.Key) {
			req = append(req, *p.Key)
		}
		for _, a := range f.asks {
			js.question(a.priority, fmt.Sprintf("%s, property %s at %s: %s", what, *p.Key, pclause, a.text), []string{fat + a.key}, a.why, js.at(pclause, "Gives "+what+"."))
		}
	}
	el := mapping("type", "object")
	if len(node.Content) > 0 {
		set(el, "properties", node)
	}
	if len(req) > 0 {
		set(el, "required", req)
	}
	return el
}

// typeDecl is the type fact a declared type refers to, or nil.
func (js *jsReader) typeDecl(t *jsType) *jsFact {
	if t == nil || t.Declaration == nil {
		return nil
	}
	return js.types[t.Declaration.key()]
}

// inferredValidator is the validation schema a type infers from, as
// z.infer<typeof newBookingSchema> or yup.InferType does, or "".
func (js *jsReader) inferredValidator(t *jsType) string {
	if t == nil || t.Reference == nil || len(t.Arguments) != 1 || t.Arguments[0].Typeof == nil {
		return ""
	}
	ref := *t.Reference
	if !strings.HasSuffix(ref, "infer") && !strings.HasSuffix(ref, "input") && !strings.HasSuffix(ref, "output") && !strings.HasSuffix(ref, "InferType") {
		return ""
	}
	return js.validators[t.Arguments[0].Declaration.key()]
}

// typeName is the schema or enum a declared type is written as: a type
// of the files read, queued to be written, or the validation schema it
// infers from. ok is false when it is neither.
func (js *jsReader) typeName(t *jsType) (string, string, bool) {
	if name := js.inferredValidator(t); name != "" {
		if child(js.enums, name) != nil {
			return name, "enums", true
		}
		return name, "schemas", true
	}
	d := js.typeDecl(t)
	if d == nil {
		return "", "", false
	}
	if d.Declaration == "alias" {
		if name := js.inferredValidator(d.Type); name != "" {
			return name, "schemas", true
		}
	}
	section := "schemas"
	if js.isEnumType(d) {
		section = "enums"
	}
	if !js.queued[d.Name] {
		js.queued[d.Name] = true
		js.schemaQueue = append(js.schemaQueue, d)
	}
	return d.Name, section, true
}

// isEnumType says whether a declared type is written as an enum: a union
// of string literals, or a TypeScript enum of strings.
func (js *jsReader) isEnumType(d *jsFact) bool {
	if d.Declaration == "enum" {
		for _, v := range d.Values {
			if v.Value == nil || v.Value.String == nil {
				return false
			}
		}
		return len(d.Values) > 0
	}
	if (d.Declaration == "alias" || d.Declaration == "typedef") && d.Type != nil {
		_, ok := stringUnion(d.Type)
		return ok
	}
	return false
}

// stringUnion is the values of a union of string literals.
func stringUnion(t *jsType) ([]string, bool) {
	if t == nil {
		return nil, false
	}
	if t.Literal != nil && t.Literal.String != nil {
		return []string{*t.Literal.String}, true
	}
	if len(t.Union) == 0 {
		return nil, false
	}
	var values []string
	for _, u := range t.Union {
		if u.Literal == nil || u.Literal.String == nil {
			return nil, false
		}
		values = append(values, *u.Literal.String)
	}
	return values, true
}

// typeField writes a declared type as a field.
func (js *jsReader) typeField(t *jsType, at string) swField {
	if t == nil {
		return swField{node: flow(mapping()), asks: []swAsk{{"must", "", "the source declares no type for it. Which type is it?", "A JavaScript value has no declared type, and a field needs one."}}}
	}
	if len(t.Union) > 0 {
		if values, ok := stringUnion(t); ok {
			return swField{node: flow(mapping("type", "string", "enum", values))}
		}
		var rest []*jsType
		nullable, optional := false, false
		for _, u := range t.Union {
			switch u.Keyword {
			case "null":
				nullable = true
			case "undefined":
				optional = true
			default:
				rest = append(rest, u)
			}
		}
		if len(rest) == 1 {
			f := js.typeField(rest[0], at)
			f.nullable = f.nullable || nullable
			f.optional = f.optional || optional
			return f
		}
		if values, ok := stringUnion(&jsType{Union: rest}); ok {
			return swField{node: flow(mapping("type", "string", "enum", values)), nullable: nullable}
		}
		return swField{held: "a union of several types, which a field does not hold"}
	}
	switch t.Keyword {
	case "string":
		return swField{node: fieldOf([]any{"type", "string"})}
	case "boolean":
		return swField{node: fieldOf([]any{"type", "boolean"})}
	case "number":
		js.widths = append(js.widths, at+"/format")
		return swField{node: fieldOf([]any{"type", "number"})}
	case "bigint":
		return swField{node: fieldOf([]any{"type", "integer"}), asks: []swAsk{{"must", "/format", "it is a bigint, which JSON.stringify refuses to write. How is it carried, and how wide is it?", "A bigint has no JSON form of its own."}}}
	case "":
	default:
		return swField{node: flow(mapping()), asks: []swAsk{{"must", "", fmt.Sprintf("the source declares it %s, which says no type. Which type is it?", t.Keyword), "A field has a type, and any, unknown and object say none."}}}
	}
	switch {
	case t.Literal != nil:
		switch {
		case t.Literal.String != nil:
			return swField{node: flow(mapping("type", "string", "enum", []string{*t.Literal.String}))}
		case t.Literal.Boolean != nil:
			return swField{node: fieldOf([]any{"type", "boolean"})}
		}
		return swField{node: fieldOf([]any{"type", "number"})}
	case t.Array != nil:
		item := js.typeField(t.Array, at+"/items")
		if item.held != "" {
			return item
		}
		if item.nullable || item.array {
			return swField{held: "a list whose items are nullable or lists themselves, which a field's items do not hold"}
		}
		f := swField{node: flow(mapping("type", "array", "items", item.node)), model: item.model, array: true}
		for _, a := range item.asks {
			a.key = "/items" + a.key
			f.asks = append(f.asks, a)
		}
		return f
	case t.Object != nil:
		return swField{held: "an object type written in place, which a field holds only as a schema of its own"}
	case t.Typeof != nil:
		return swField{node: flow(mapping()), asks: []swAsk{{"must", "", fmt.Sprintf("the source declares it as the type of %s. Which type is it?", *t.Typeof), "The type of a value is the compiler's inference, which the reader does not write."}}}
	case t.Reference != nil:
		ref := *t.Reference
		if ref == "Array" && len(t.Arguments) == 1 {
			return js.typeField(&jsType{Array: t.Arguments[0]}, at)
		}
		if ref == "Date" && t.Library {
			return swField{node: fieldOf([]any{"type", "string", "format", "date-time"})}
		}
		if name, section, ok := js.typeName(t); ok {
			return swField{node: flow(mapping("$ref", "#/"+section+"/"+name)), model: name}
		}
		if t.Library {
			return swField{held: fmt.Sprintf("the type %s, which a field does not hold", ref)}
		}
		where := "declared outside the files read, such as in a package"
		if t.Import != nil {
			where = "imported from " + t.Import.Module + ", which is not a file of the folder read"
		}
		return swField{node: flow(mapping()), asks: []swAsk{{"must", "", fmt.Sprintf("the source declares the type %s, %s. Which type is it?", ref, where), "Only tracked files are read, so a package's types are not known."}}}
	}
	return swField{node: flow(mapping()), asks: []swAsk{{"must", "", fmt.Sprintf("the source declares the type %s, which the reader does not read. Which type is it?", t.Text), "The reader writes the types a field can hold."}}}
}

// readTypes writes each declared type a route refers to as a schema or
// an enum, and each type those refer to.
func (js *jsReader) readTypes() {
	for i := 0; i < len(js.schemaQueue); i++ {
		d := js.schemaQueue[i]
		js.writeDeclared(d)
	}
}

// typeSays is how a citation names a declared type.
func typeSays(d *jsFact) string {
	switch d.Declaration {
	case "interface":
		return "interface " + d.Name
	case "alias":
		return "type " + d.Name
	case "typedef":
		return "the JSDoc @typedef " + d.Name
	case "enum":
		return "enum " + d.Name
	}
	return d.Declaration + " " + d.Name
}

// writeDeclared writes one declared type: an enum of strings as an enum,
// an object type as a schema.
func (js *jsReader) writeDeclared(d *jsFact) {
	clause := js.clause(d)
	if !pascalWord.MatchString(d.Name) {
		js.gap(clause, []string{"schemas"}, "", "the type %s: its name is not a PascalCase name a schema can take; left out", d.Name)
		return
	}
	if js.schemaFrom[d.Name] {
		js.question("must", fmt.Sprintf("The type %s at %s has the name of a validation schema the reader writes. Which name does each take?", d.Name, clause),
			[]string{"#/schemas/" + d.Name}, "A schema has one name, and two would become one.", js.at(clause, upperFirst(typeSays(d))+"."))
		return
	}
	js.schemaFrom[d.Name] = true
	if d.Declaration == "typedef" {
		if on, _ := js.checkJs(d.File); !on {
			// The caller asked already, at the use.
			return
		}
	}
	if js.isEnumType(d) {
		var values []string
		if d.Declaration == "enum" {
			for _, v := range d.Values {
				values = append(values, *v.Value.String)
			}
		} else {
			values, _ = stringUnion(d.Type)
		}
		var good []string
		for _, v := range values {
			if enumValueWord.MatchString(v) {
				good = append(good, v)
			} else {
				js.gap(clause, []string{"#/enums/" + d.Name}, "", "%s's value %q is not a snake_case word an enum's value takes; left out", d.Name, v)
			}
		}
		if len(good) == 0 {
			return
		}
		if js.enums == nil {
			js.enums = mapping()
		}
		set(js.enums, d.Name, mapping("type", "string", "enum", good, "origin", "stated",
			"cites", []*yaml.Node{js.at(clause, fmt.Sprintf("%s is the strings %s.", upperFirst(typeSays(d)), strings.Join(good, ", ")))}))
		return
	}
	members := d.Members
	if d.Declaration == "alias" || d.Declaration == "typedef" {
		if d.Type == nil || d.Type.Object == nil {
			js.question("must", fmt.Sprintf("%s at %s is not an object type written in place. Which fields does it hold?", typeSays(d), clause),
				[]string{"#/schemas/" + d.Name}, "A schema is read from an object type's own members.", js.at(clause, upperFirst(typeSays(d))+"."))
			return
		}
		members = d.Type.Object
	}
	if len(d.Extends) > 0 {
		js.gap(clause, []string{"#/schemas/" + d.Name}, "", "%s extends %s, whose members are not joined; read from its own members", typeSays(d), joinAnd(d.Extends))
	}
	at := "#/schemas/" + d.Name
	props := mapping()
	var required []string
	for _, m := range members {
		mc := fmt.Sprintf("%s:%d", d.File, m.Line)
		if m.Static {
			continue
		}
		if !memberNameWord.MatchString(m.Name) {
			js.gap(mc, []string{at}, "", "%s.%s: the member's name is not a camelCase name a field can take; left out", d.Name, m.Name)
			continue
		}
		fat := at + "/properties/" + m.Name
		f := js.typeField(m.Type, fat)
		if f.held != "" {
			js.gap(mc, []string{at}, "", "%s.%s: %s; left out", d.Name, m.Name, f.held)
			continue
		}
		optional := m.Optional || f.optional
		asks := f.asks
		node := f.node
		if f.nullable {
			node = nullable(node)
		}
		set(props, m.Name, node)
		if !optional && !f.nullable {
			required = append(required, m.Name)
		}
		for _, a := range asks {
			js.question(a.priority, fmt.Sprintf("%s.%s at %s: %s", d.Name, m.Name, mc, a.text), []string{fat + a.key}, a.why, js.at(mc, "Declares "+m.Name+"."))
		}
	}
	el := mapping("type", "object")
	if len(props.Content) > 0 {
		set(el, "properties", props)
	}
	if len(required) > 0 {
		set(el, "required", required)
	}
	set(el, "origin", "stated")
	set(el, "cites", []*yaml.Node{js.at(clause, upperFirst(typeSays(d))+".")})
	if js.schemas == nil {
		js.schemas = mapping()
	}
	set(js.schemas, d.Name, el)
}

// upperFirst writes a phrase's first letter in upper case, to start a
// sentence with it.
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
