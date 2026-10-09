package extract

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/wirename"
)

// swField is a Swift type written as a field, with what is asked about it.
type swField struct {
	node     *yaml.Node
	nullable bool
	model    string // the model type of the files read it refers to, through an array too
	array    bool
	held     string // why the meta-model cannot hold it, "" when it can
	optional bool   // a TypeScript union with undefined: a field that may be left out
	asks     []swAsk
}

// swAsk is a question on a field: on the key under it, or on the field
// itself when key is "".
type swAsk struct {
	priority, key, text, why string
}

// An enum's value, as the design schema takes it.
var snakeWord = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// The Swift types written as a field, as Foundation and the standard
// library name them. Each integer has the width Swift gives it; an Int
// or a UInt is as wide as the platform's word, so it is written 64 bits
// wide and asked.
var swiftScalars = map[string][]any{
	"String":  {"type", "string"},
	"Bool":    {"type", "boolean"},
	"Int8":    {"type", "integer", "format", "int32", "minimum", -128, "maximum", 127},
	"Int16":   {"type", "integer", "format", "int32", "minimum", -32768, "maximum", 32767},
	"Int32":   {"type", "integer", "format", "int32"},
	"UInt8":   {"type", "integer", "format", "int32", "minimum", 0, "maximum", 255},
	"UInt16":  {"type", "integer", "format", "int32", "minimum", 0, "maximum", 65535},
	"UInt32":  {"type", "integer", "format", "int64", "minimum", 0, "maximum", 4294967295},
	"Double":  {"type", "number", "format", "double"},
	"Float64": {"type", "number", "format", "double"},
	"UUID":    {"type", "string", "format", "uuid"},
	"URL":     {"type", "string", "format", "uri"},
}

func fieldOf(pairs []any) *yaml.Node {
	n := mapping()
	for i := 0; i+1 < len(pairs); i += 2 {
		v := pairs[i+1]
		if x, ok := v.(int); ok {
			v = int64(x)
		}
		set(n, pairs[i].(string), v)
	}
	return flow(n)
}

// field writes a Swift type as a field. wire is true for a type encoded
// as JSON by Codable, false for one stored by SwiftData or Core Data.
func (sw *swiftReader) field(text string, wire bool) swField {
	t := strings.TrimSpace(text)
	var f swField
	switch {
	case strings.HasSuffix(t, "?") || strings.HasSuffix(t, "!"):
		f = sw.field(t[:len(t)-1], wire)
		f.nullable = true
		return f
	case strings.HasPrefix(t, "Optional<") && strings.HasSuffix(t, ">"):
		f = sw.field(t[len("Optional<"):len(t)-1], wire)
		f.nullable = true
		return f
	case strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") && !strings.Contains(t, ":"),
		strings.HasPrefix(t, "Array<") && strings.HasSuffix(t, ">"):
		inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(t, "Array<"), "["), "]")
		if strings.HasPrefix(t, "Array<") {
			inner = t[len("Array<") : len(t)-1]
		}
		item := sw.field(inner, wire)
		if item.nullable || item.array {
			return swField{held: "a list of " + inner + ", whose items are optional or lists themselves, which a field's items do not hold"}
		}
		if item.held != "" {
			return item
		}
		f = swField{node: flow(mapping("type", "array", "items", item.node)), model: item.model, array: true}
		for _, a := range item.asks {
			if a.key != "" {
				a.key = "/items" + a.key
			} else {
				a.key = "/items"
			}
			f.asks = append(f.asks, a)
		}
		return f
	case strings.HasPrefix(t, "[") || strings.HasPrefix(t, "Dictionary<") || strings.HasPrefix(t, "Set<"):
		return swField{held: "the type " + t + ", a dictionary or a set, which a field does not hold"}
	}
	if pairs, ok := swiftScalars[t]; ok {
		return swField{node: fieldOf(pairs)}
	}
	switch t {
	case "Int", "UInt", "Int64", "UInt64":
		f = swField{node: fieldOf([]any{"type", "integer"})}
		if strings.HasPrefix(t, "U") {
			f.node = fieldOf([]any{"type", "integer", "minimum", 0})
		}
		wide := "64 bits wide"
		if t == "Int" || t == "UInt" {
			wide = "as wide as the platform's word, 64 bits on every 64-bit Apple platform and 32 on arm64_32"
		}
		f.asks = []swAsk{{"must", "/format", fmt.Sprintf("Swift's %s is %s, and a 64-bit integer sent as a JSON number loses digits above 2^53. Which format does it take (int32, int64 carried as a string, uint64), or which bounds keep it inside 2^53?", t, wide), "Every integer in a specification has a width, and one wider than 2^53 is carried as a string or bounded."}}
		return f
	case "Float", "Float32", "CGFloat":
		f = swField{node: fieldOf([]any{"type", "number"})}
		if t == "CGFloat" {
			f.node = fieldOf([]any{"type", "number", "format", "double"})
			f.asks = []swAsk{{"should", "/format", "CGFloat is a Double on every 64-bit Apple platform and a Float on a 32-bit one. Is double right for it?", "CGFloat's width follows the platform."}}
			return f
		}
		f.asks = []swAsk{{"must", "/format", "The source declares a 32-bit Float, and a number in the meta-model is a double. Is a double right here, or is the value a decimal with a precision and a scale?", "A Float has a width the meta-model's number formats do not name."}}
		return f
	case "Decimal":
		return swField{node: flow(mapping()), asks: []swAsk{{"must", "", "The source declares a Decimal, whose precision and scale it does not say, and Codable writes it on the wire as a JSON number. Which type, precision and scale is it?", "A decimal needs a precision and a scale, and the Swift type gives neither."}}}
	case "Date":
		f = swField{node: fieldOf([]any{"type", "string", "format", "date-time"})}
		if wire && !sw.isoDates() {
			f.asks = []swAsk{{"should", "", "Codable writes a Date as a number of seconds since 2001 unless the encoder is given another date strategy, and no .iso8601 strategy is set in the files read. Is it a date-time string on the wire?", "The wire form of a Date is the encoder's choice, which the code that encodes it sets."}}
		}
		return f
	case "Data":
		if wire {
			return swField{node: fieldOf([]any{"type", "string", "format", "byte"})}
		}
		return swField{node: fieldOf([]any{"type", "string", "format", "binary"})}
	}
	name := t
	if mt := sw.types[name]; mt != nil && mt.decl != nil {
		switch sw.modelKind(mt) {
		case "entity":
			return swField{node: flow(mapping("$ref", "#/entities/"+name)), model: name}
		case "schema":
			return swField{node: flow(mapping("$ref", "#/schemas/"+name)), model: name}
		case "enum":
			return swField{node: flow(mapping("$ref", "#/enums/"+name)), model: name}
		}
	}
	return swField{node: flow(mapping()), asks: []swAsk{{"must", "", fmt.Sprintf("The source declares the type %s, which is not declared in the files read as a model the reader knows, such as a type of a package. Which type is it?", t), "A type declared outside the files read, or not as a model, is not known by syntax."}}}
}

// isoDates says whether the files read set an ISO 8601 date strategy on a
// JSON encoder or decoder.
func (sw *swiftReader) isoDates() bool {
	for _, facts := range sw.byScope {
		for _, f := range facts {
			if f.Kind == "assignment" && (strings.HasSuffix(f.Target, ".dateDecodingStrategy") || strings.HasSuffix(f.Target, ".dateEncodingStrategy")) &&
				f.Value != nil && f.Value.Member != nil && *f.Value.Member == ".iso8601" {
				return true
			}
		}
	}
	return false
}

// snakeKeys says whether the files read convert keys to or from snake_case
// on a JSON encoder or decoder, and where.
func (sw *swiftReader) snakeKeys() string {
	var at []string
	for _, facts := range sw.byScope {
		for _, f := range facts {
			if f.Kind == "assignment" && f.Value != nil && f.Value.Member != nil &&
				(*f.Value.Member == ".convertFromSnakeCase" || *f.Value.Member == ".convertToSnakeCase") {
				at = append(at, sw.clause(f))
			}
		}
	}
	sort.Strings(at)
	if len(at) == 0 {
		return ""
	}
	return at[0]
}

// modelKind is what a type of the files read is written as: entity for a
// SwiftData model, schema for a Codable type, enum for a Codable enum of
// strings, or "" for none.
func (sw *swiftReader) modelKind(t *swType) string {
	if t.decl == nil || strings.Contains(t.name, ".") {
		return ""
	}
	switch {
	case t.hasAttribute("Model"):
		return "entity"
	case t.decl.Declaration == "enum":
		if t.conforms("String") && !t.conforms("CodingKey") && t.conforms("Codable", "Decodable", "Encodable", "Content") {
			return "enum"
		}
	case t.decl.Declaration == "struct" || t.decl.Declaration == "class":
		if t.conforms("Codable", "Decodable", "Encodable", "Content") {
			return "schema"
		}
	}
	return ""
}

// readModels writes each SwiftData model as an entity, each Codable type
// as a schema and each Codable enum of strings as an enum.
func (sw *swiftReader) readModels() {
	names := append([]string{}, sw.typeOrder...)
	sort.Strings(names)
	snakeAt := sw.snakeKeys()
	type wireNote struct {
		clause, schema, prop string
	}
	var camelOnWire []wireNote
	snakeSeen := snakeAt != ""
	stored, skipped := 0, 0
	for _, name := range names {
		t := sw.types[name]
		if t.decl == nil {
			continue
		}
		kind := sw.modelKind(t)
		if kind == "" {
			if strings.Contains(name, ".") && t.decl.Declaration != "extension" && (t.hasAttribute("Model") || t.conforms("Codable", "Decodable", "Encodable", "Content")) && !t.conforms("CodingKey") {
				sw.gap(sw.clause(t.decl), []string{"schemas"}, "", "the type %s is declared inside another type, and a model is named by one PascalCase word; left out", name)
			}
			continue
		}
		decl := sw.clause(t.decl)
		if kind == "enum" {
			sw.writeEnum(t)
			continue
		}
		section := "entities"
		if kind == "schema" {
			section = "schemas"
		}
		at := "#/" + section + "/" + name
		wire := kind == "schema"
		var keys map[string]string
		if ck := sw.types[name+".CodingKeys"]; ck != nil && wire {
			keys = map[string]string{}
			for _, c := range ck.cases {
				w := c.Name
				if s, ok := c.RawValue.stringLiteral(); ok {
					w = s
				}
				keys[c.Name] = w
			}
		}
		props := mapping()
		var required, unique []string
		relations := mapping()
		var relAsk []string
		for _, p := range t.props {
			pc := sw.clause(p)
			if p.Static || p.Computed {
				skipped++
				continue
			}
			if _, ok := p.attribute("Transient"); ok {
				sw.gap(pc, []string{at}, "", "%s.%s is marked @Transient, so SwiftData does not store it; left out", name, p.Name)
				continue
			}
			if !memberNameWord.MatchString(p.Name) {
				sw.gap(pc, []string{at}, "", "%s.%s: the property's name is not a camelCase name a field can take; left out", name, p.Name)
				continue
			}
			if keys != nil {
				w, ok := keys[p.Name]
				if !ok {
					sw.gap(pc, []string{at}, "", "%s.%s is not among the type's CodingKeys, so Codable does not encode it; left out", name, p.Name)
					continue
				}
				switch {
				case w == p.Name:
				case w == wirename.Snake(p.Name):
					snakeSeen = true
				default:
					sw.gap(pc, []string{at + "/properties/" + p.Name}, "", "%s.%s goes on the wire as %s, which info.wireNames cannot give; written by its Swift name", name, p.Name, w)
				}
			} else if wire && snakeAt == "" && wirename.Snake(p.Name) != p.Name {
				camelOnWire = append(camelOnWire, wireNote{pc, name, p.Name})
			}
			typeText := p.Type
			if typeText == "" {
				typeText = literalType(p.Value)
			}
			fat := at + "/properties/" + p.Name
			if typeText == "" {
				set(props, p.Name, flow(mapping()))
				sw.question("must", fmt.Sprintf("%s.%s at %s has no declared type, and its initial value does not say one. Which type is it?", name, p.Name, pc),
					[]string{fat}, "Syntax gives a type only where the source writes it.", sw.at(pc, "Declares "+p.Name+" with no type."))
				continue
			}
			f := sw.field(typeText, wire)
			if f.held != "" {
				sw.gap(pc, []string{at}, "", "%s.%s: %s; left out", name, p.Name, f.held)
				continue
			}
			if kind == "entity" && f.model != "" && sw.modelKind(sw.types[f.model]) != "enum" {
				target := sw.types[f.model]
				if sw.modelKind(target) != "entity" {
					sw.gap(pc, []string{at}, "", "%s.%s holds the schema %s, and an entity refers to an entity or an enum, never to a schema; left out", name, p.Name, f.model)
					continue
				}
				many := "one"
				if f.array {
					many = "many"
				}
				set(relations, p.Name, mapping("target", f.model, "origin", "stated", "cites", []*yaml.Node{sw.at(pc, fmt.Sprintf("%s.%s holds %s %s.", name, p.Name, many, f.model))}))
				relAsk = append(relAsk, at+"/relations/"+p.Name+"/kind", at+"/relations/"+p.Name+"/via")
				continue
			}
			node := f.node
			if f.nullable {
				node = nullable(node)
			} else {
				required = append(required, p.Name)
			}
			if a, ok := p.attribute("Attribute"); ok && strings.Contains(a.Arguments, ".unique") {
				unique = append(unique, p.Name)
			}
			set(props, p.Name, node)
			stored++
			for _, a := range f.asks {
				sw.question(a.priority, fmt.Sprintf("%s.%s at %s: %s", name, p.Name, pc, a.text), []string{fat + a.key}, a.why, sw.at(pc, fmt.Sprintf("Declares %s: %s.", p.Name, p.Type)))
			}
		}
		if len(props.Content) == 0 && len(relations.Content) == 0 {
			sw.gap(decl, []string{section}, "", "%s declares no stored property a field can hold; left out", name)
			continue
		}
		says := fmt.Sprintf("%s %s is a SwiftData model.", t.decl.Declaration, name)
		if kind == "schema" {
			says = fmt.Sprintf("%s %s conforms to %s.", t.decl.Declaration, name, strings.Join(t.inherits, ", "))
		}
		el := mapping("type", "object")
		if len(props.Content) > 0 {
			set(el, "properties", props)
		}
		if len(required) > 0 {
			set(el, "required", required)
		}
		if len(relations.Content) > 0 {
			set(el, "relations", relations)
		}
		set(el, "origin", "stated")
		set(el, "cites", []*yaml.Node{sw.at(decl, says)})
		if kind == "entity" {
			if sw.entities == nil {
				sw.entities = mapping()
			}
			set(sw.entities, name, el)
			sw.entityNames[name] = true
			key := "SwiftData identifies a record by its persistent identifier, which no property declares"
			if len(unique) > 0 {
				key += fmt.Sprintf(", and %s %s marked unique", joinAnd(unique), isOrAre(len(unique)))
			}
			sw.question("must", fmt.Sprintf("Which field or fields identify a record of %s? %s.", name, key),
				[]string{at + "/primaryKey"}, "A primary key is what a record is found by, and a SwiftData model declares none.", sw.at(decl, says))
			if len(relAsk) > 0 {
				sw.question("must", fmt.Sprintf("For each relation of %s, which kind is it (one-to-one, one-to-many, many-to-one or many-to-many), and which field holds the key or points back?", name),
					relAsk, "SwiftData keeps a relation by the related model, with no field that holds its key.", sw.at(decl, says))
			}
		} else {
			if sw.schemas == nil {
				sw.schemas = mapping()
			}
			set(sw.schemas, name, el)
		}
	}
	if snakeSeen {
		sw.wireSnake = true
		for _, w := range camelOnWire {
			sw.gap(w.clause, []string{"#/schemas/" + w.schema + "/properties/" + w.prop}, "", "%s.%s goes on the wire as %s, since its type has no CodingKeys, and info.wireNames snake_case, which another type's CodingKeys give, would write it %s", w.schema, w.prop, w.prop, wirename.Snake(w.prop))
		}
	}
	if count(sw.entities)+count(sw.schemas)+count(sw.enums) > 0 || stored > 0 {
		sw.res.say("models: counted %s and %s, and left out %s: every stored property of a SwiftData model or a Codable type of the files read, and every case of a Codable enum of strings", plural(stored, "stored property"), plural(count(sw.enums), "enum"), plural(skipped, "static or computed property"))
	}
}

// writeEnum writes a Codable enum of strings, its values its raw values.
func (sw *swiftReader) writeEnum(t *swType) {
	decl := sw.clause(t.decl)
	var values []string
	for _, c := range t.cases {
		v := c.Name
		if s, ok := c.RawValue.stringLiteral(); ok {
			v = s
		} else if c.RawValue != nil || c.Associated {
			sw.gap(sw.clause(c), []string{"#/enums/" + t.name}, "", "%s.%s has a raw value that is not a string literal, or associated values; left out", t.name, c.Name)
			continue
		}
		if !snakeWord.MatchString(v) {
			sw.gap(sw.clause(c), []string{"#/enums/" + t.name}, "", "%s.%s is %q on the wire, and an enum's values are snake_case words; left out", t.name, c.Name, v)
			continue
		}
		values = append(values, v)
	}
	if len(values) == 0 {
		sw.gap(decl, []string{"enums"}, "", "the enum %s has no value an enum can hold; left out", t.name)
		return
	}
	if sw.enums == nil {
		sw.enums = mapping()
	}
	set(sw.enums, t.name, mapping("type", "string", "enum", values, "origin", "stated",
		"cites", []*yaml.Node{sw.at(decl, fmt.Sprintf("enum %s: %s, with the cases %s.", t.name, strings.Join(t.inherits, ", "), strings.Join(values, ", ")))}))
}

// literalType is the type an initial value gives by its syntax alone.
func literalType(v *swValue) string {
	switch {
	case v == nil:
		return ""
	case v.String != nil || v.Interpolated != nil:
		return "String"
	case v.Boolean != nil:
		return "Bool"
	case v.Integer != nil:
		return "Int"
	case v.Float != nil:
		return "Double"
	}
	if name, c := v.construct(); name != "" && c == v.Call {
		return name
	}
	return ""
}

// nullable lets a field also be null, as JSON Schema writes it; a
// reference stays as it is, and is left out of required.
func nullable(n *yaml.Node) *yaml.Node {
	t := child(n, "type")
	if t == nil || t.Kind != yaml.ScalarNode {
		return n
	}
	t.Content = []*yaml.Node{str(t.Value), str("null")}
	t.Kind, t.Tag, t.Value, t.Style = yaml.SequenceNode, "", "", yaml.FlowStyle
	return n
}
