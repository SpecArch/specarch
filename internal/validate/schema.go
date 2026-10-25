package validate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"go.yaml.in/yaml/v3"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/SpecArch/specarch/internal/metamodel"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/schema"
)

const (
	designSchemaID         = "https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-design-1.0.schema.json"
	earlierDesignSchemaID  = "https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-design-0.3.schema.json"
	implementationSchemaID = "https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-implementation-0.1.schema.json"
	recordSchemaID         = "https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-record-0.1.schema.json"
	idiomSchemaID          = "https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json"
	estateSchemaID         = "https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-estate-0.1.schema.json"
)

var (
	compileOnce   sync.Once
	designSchema  *jsonschema.Schema
	earlierSchema *jsonschema.Schema
	implSchema    *jsonschema.Schema
	recordSchema  *jsonschema.Schema
	idiomSchema   *jsonschema.Schema
	estateSchema  *jsonschema.Schema
	compileErr    error
	printer       = message.NewPrinter(language.English)
)

func compileSchemas() {
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	for id, raw := range map[string][]byte{designSchemaID: schema.Definition, earlierDesignSchemaID: schema.DefinitionEarlier, implementationSchemaID: schema.Implementation, recordSchemaID: schema.Record, idiomSchemaID: schema.Idiom, estateSchemaID: schema.Estate} {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			compileErr = err
			return
		}
		if err := c.AddResource(id, doc); err != nil {
			compileErr = err
			return
		}
	}
	designSchema, compileErr = c.Compile(designSchemaID)
	if compileErr != nil {
		return
	}
	earlierSchema, compileErr = c.Compile(earlierDesignSchemaID)
	if compileErr != nil {
		return
	}
	implSchema, compileErr = c.Compile(implementationSchemaID)
	if compileErr != nil {
		return
	}
	recordSchema, compileErr = c.Compile(recordSchemaID)
	if compileErr != nil {
		return
	}
	idiomSchema, compileErr = c.Compile(idiomSchemaID)
	if compileErr != nil {
		return
	}
	estateSchema, compileErr = c.Compile(estateSchemaID)
}

// Plain descriptions of the naming patterns in the schemas, so a message can
// say "camelCase, such as dueOn" instead of quoting a regular expression.
var patternNames = map[string]string{
	"^[A-Z][A-Za-z0-9]*$":                                                      "PascalCase, such as Loan",
	"^[a-z][A-Za-z0-9]*$":                                                      "camelCase, such as dueOn",
	"^[a-z][a-z0-9]*(\\.[a-z][a-z0-9]*)*$":                                     "dotted lower case, such as loans.create",
	"^[a-z][a-z0-9]*(-[a-z0-9]+)*$":                                            "kebab-case, such as members-list",
	"^[a-z][a-z0-9]*(_[a-z0-9]+)*$":                                            "snake_case, such as loan_due_after_loaned",
	"^[A-Z][A-Z0-9]{1,15}-[A-Za-z0-9._]+$":                                     "an ID: an upper-case prefix, a dash and a number or name, such as LIB-5 or NEED-1",
	"^[A-Z][A-Z0-9]{0,15}-[A-Za-z0-9._]+$":                                     "a question ID: an upper-case prefix, a dash and a number or name, such as Q-12 or OPEN-3",
	"^[A-Z][A-Z0-9]{1,15}$":                                                    "an upper-case prefix of 2 to 16 letters or digits, such as LIB",
	"^ADR-[0-9]{3,}$":                                                          "ADR- and three or more digits, such as ADR-001",
	"^[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?$":                             "a semantic version, such as 1.2.0",
	"^[0-9]+\\.[0-9]+\\.[0-9]+$":                                               "a release's version, major.minor.patch with no pre-release, such as 1.2.0",
	"^([A-Z][A-Z0-9]{1,15}-[A-Za-z0-9._]+|#/.+)$":                              "a requirement ID such as LIB-5, or a #/ pointer such as #/entities/Loan",
	"^[0-9]{4}-[0-9]{2}-[0-9]{2}-[a-z][a-z0-9]*(-[a-z0-9]+)*$":                 "a date and an environment, such as 2026-10-07-production",
	"^sha256:[0-9a-f]{64}$":                                                    "sha256: and 64 lower-case hexadecimal digits",
	"^[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)+$":                               "relations and a field separated by dots, such as member.fullName",
	"^([a-z][A-Za-z0-9]*)?#/(entities|enums|views|schemas)/[A-Z][A-Za-z0-9]*$": "#/entities/Name, #/enums/Name, #/views/Name or #/schemas/Name, or one of another specification through a dependency, such as customerRegistry#/schemas/Address",
	"^([a-z][A-Za-z0-9]*#/entities/)?[A-Z][A-Za-z0-9]*$":                       "PascalCase, such as Loan, or an entity of another specification through a dependency, such as customerRegistry#/entities/Customer",
	"^[a-z][A-Za-z0-9]*(#/paths/~1[^/]*/(get|post|put|patch|delete))?$":        "camelCase, such as listMembers, or an operation of another specification through a dependency, such as customerRegistry#/paths/~1customers/get",
	"^/":                          "a path starting with /",
	"^([1-5][0-9][0-9]|default)$": "an HTTP status code such as 200, or default",
	"^[a-z]+/[a-z0-9.+-]+$":       "a media type, such as application/json",
	"^[a-z][a-z0-9]*(\\.[a-z][a-z0-9]*)*/[A-Z][A-Za-z0-9]*$":       "channel/Message, such as loan.lifecycle/LoanCreated",
	"^[a-z][a-z0-9]*(-[a-z0-9]+)*( [a-z][a-z0-9]*(-[a-z0-9]+)*)*$": "kebab-case words separated by single spaces, such as generate techspec",
	"^(0|[1-9][0-9]?|1[0-9][0-9]|2[0-4][0-9]|25[0-5])$":            "an exit status from 0 to 255",
	"(^|/)specarch\\.yaml$":                                        "a path ending in specarch.yaml, the specification's root file",
	"^[A-Za-z][A-Za-z0-9 ./'-]*$":                                  "a term: letters, digits, spaces, dots, slashes, apostrophes and dashes, such as late fee",
	"^[A-Za-z]{2,3}(-[A-Za-z0-9]{1,8})*$":                          "a BCP 47 language tag, such as en or pt-BR",
	"^https://[^\\s?#]+$":                                          "an https URL with no query or fragment, such as https://idp.example.org",
	"^https://[^\\s#]+$":                                           "an https URL with no fragment, such as https://idp.example.org/keys",
	"^([^/\\\\:?#%\\s.][^/\\\\:?#%\\s]*|\\.[^/\\\\:?#%\\s.][^/\\\\:?#%\\s]*)(/([^/\\\\:?#%\\s.][^/\\\\:?#%\\s]*|\\.[^/\\\\:?#%\\s.][^/\\\\:?#%\\s]*))*$": "a path relative to the site's root, such as images/studio.jpg, with no scheme, no leading / or \\, no . or .. folder and no ?, #, % or space",
}

func describePattern(p string) string {
	if s, ok := patternNames[p]; ok {
		return s
	}
	return "the pattern " + p
}

func (c *checker) checkSchema(k Kind, value any) {
	compileOnce.Do(compileSchemas)
	if compileErr != nil {
		c.addLine(1, "/", RuleSchema, "the built-in schema does not compile (%v); this is a bug in specarch", compileErr)
		return
	}
	sch := designSchema
	switch k {
	case KindDesign:
		sch, value = c.designSchemaFor(value)
	case KindImplementation:
		sch = implSchema
	case KindRecord:
		sch = recordSchema
	case KindIdiom:
		sch = idiomSchema
	case KindEstate:
		sch = estateSchema
	}
	err := sch.Validate(value)
	if err == nil {
		return
	}
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		c.addLine(1, "/", RuleSchema, "%v", err)
		return
	}
	seen := map[string]bool{}
	c.schemaLeaves(k, ve, nil, seen)
}

// designSchemaFor is the schema that checks a specification, by its
// version line: the earlier schema for 0.1, 0.2 and 0.3, the current one
// otherwise. A version line this program does not read is reported by
// checkMetaModelVersion, and the file is checked as 1.0.0 so that the
// schema does not report the line a second time.
func (c *checker) designSchemaFor(value any) (*jsonschema.Schema, any) {
	line := source.Child(c.root, "specarch")
	v := source.Str(line)
	if metamodel.Design.IsEarlier(v) {
		return earlierSchema, value
	}
	if m, ok := value.(map[string]any); ok && line != nil && line.Tag == "!!str" && metamodel.Design.Problem(v) != "" {
		cp := maps.Clone(m)
		cp["specarch"] = metamodel.Current
		return designSchema, cp
	}
	return designSchema, value
}

// schemaLeaves reports the innermost causes, one per value and kind of
// problem, in plain words. parent is the location of the enclosing error.
func (c *checker) schemaLeaves(k Kind, ve *jsonschema.ValidationError, parent []string, seen map[string]bool) {
	if pn, ok := ve.ErrorKind.(*kind.PropertyNames); ok {
		c.schemaPropertyName(k, ve, c.propertyNameObject(pn.Property, len(ve.InstanceLocation), ve.SchemaURL, parent, seen), pn, seen)
		return
	}
	// A list that holds no item contains accepts fails as a whole, not item
	// by item: the library keeps each item's failure as a cause, and those
	// would read as if every item were wrong.
	if _, ok := ve.ErrorKind.(*kind.Contains); !ok && len(ve.Causes) > 0 {
		for _, cause := range ve.Causes {
			c.schemaLeaves(k, cause, ve.InstanceLocation, seen)
		}
		return
	}
	path := source.Pointer(ve.InstanceLocation...)
	node, _ := source.Resolve(c.root, ve.InstanceLocation)
	switch e := ve.ErrorKind.(type) {
	case *kind.AdditionalProperties:
		for _, p := range e.Properties {
			if k == KindImplementation && designKeys[p] {
				continue // reported as design_key
			}
			key := ve.InstanceLocation
			kp := source.Pointer(append(slices.Clone(key), p)...)
			if seen[kp+"|extra"] {
				continue
			}
			seen[kp+"|extra"] = true
			c.add(source.Key(node, p), kp, RuleSchema, "%s is not a key this object can have; remove it or correct its spelling", p)
		}
		return
	case *kind.Required, *kind.DependentRequired:
		var missing []string
		if r, ok := e.(*kind.Required); ok {
			missing = r.Missing
		} else {
			missing = e.(*kind.DependentRequired).Missing
		}
		for _, m := range missing {
			id := path + "|required|" + m
			if seen[id] {
				continue
			}
			seen[id] = true
			c.add(node, path, RuleSchema, "%s is missing; add it here", m)
		}
		return
	}
	id := path + "|" + strings.Join(ve.ErrorKind.KeywordPath(), "/")
	if seen[path+"|any"] || seen[id] {
		return
	}
	seen[id] = true
	seen[path+"|any"] = true
	c.add(node, path, RuleSchema, "%s", plainSchemaMessage(ve.ErrorKind))
}

func (c *checker) schemaPropertyName(k Kind, ve *jsonschema.ValidationError, location []string, pn *kind.PropertyNames, seen map[string]bool) {
	if k == KindDesign && stackKeyPattern.MatchString(pn.Property) {
		return // reported as stack_key
	}
	obj, _ := source.Resolve(c.root, location)
	path := source.Pointer(append(slices.Clone(location), pn.Property)...)
	if seen[path+"|name"] {
		return
	}
	seen[path+"|name"] = true
	why := "it is not a name allowed here"
	for _, cause := range leaves(ve) {
		switch e := cause.ErrorKind.(type) {
		case *kind.Pattern:
			why = "it must be " + describePattern(e.Want)
		case *kind.Enum:
			why = "it must be one of " + joinValues(e.Want)
		}
	}
	c.add(source.Key(obj, pn.Property), path, RuleSchema, "the name %s is not valid: %s; rename it", pn.Property, why)
}

// propertyNameObject finds the object a propertyNames error is about. The
// schema library stores that error's location in the slice it goes on
// appending to while it walks the rest of the input, so when the errors are
// read, every propertyNames error of one walk holds the path of the last
// sibling walked at its depth; only the length of the location it reports
// can be trusted. Two other things are: the location of the enclosing
// error, parent, which the library copies, and which is the object itself
// or an ancestor of it (a $ref wraps the errors of the object it refers to
// in a Reference error at that object, so a rule on a definition, or on a
// key of one, is pinned to the object or to its parent); and the schema
// location, which ends in properties/<key>/propertyNames when the rule
// sits on a document key (".../properties/responses/propertyNames"), and
// in the definition's name when it sits on a definition reached through a
// $ref (".../$defs/statusEvents/propertyNames"), which is no key in the
// document and is not used. The candidates are the mappings at the error's
// depth under parent that hold the property, and, when the schema
// location names a key, sit under that key: when parent is the object,
// the object alone; when parent is its parent and the key is named, the
// one child under that key; when neither, the siblings under parent that
// hold the name, such as the media types of one content object. They are
// listed in the order of the merged specification (the root file's own
// keys, then the sections in life-cycle order, entities before schemas
// whatever the file says, and within a section the order written), and
// the k-th error of one property, depth, parent and key takes the k-th
// candidate; the count is keyed by exactly what the candidates were
// filtered by, so two errors never take one mapping. Where the candidates
// are siblings, that rests on one thing: every propertyNames rule the
// schema reaches at one depth under one parent gives the same verdict on
// a name, with the same pattern, so the candidates holding the name equal
// the errors, and the errors, which arrive in the library's map order, can
// be given to the candidates in any order. In the design schema every
// site but one is pinned to one candidate; the one is modes under theme,
// inline two keys below the root, whose enclosing error is the root and
// whose candidates are the depth-2 mappings named modes that hold the
// name, one in any specification written so far. seen holds the count.
func (c *checker) propertyNameObject(property string, depth int, schemaURL string, parent []string, seen map[string]bool) []string {
	under := ""
	if _, frag, ok := strings.Cut(schemaURL, "#"); ok {
		tokens := strings.Split(strings.TrimPrefix(frag, "/"), "/")
		if n := len(tokens); n >= 3 && tokens[n-1] == "propertyNames" && tokens[n-3] == "properties" {
			under = source.UnescapeToken(tokens[n-2])
		}
	}
	found := c.findPropertyObjects(property, depth, parent, under)
	key := property + "|" + strconv.Itoa(depth) + "|" + source.Pointer(parent...) + "|" + under + "|taken|"
	taken := 0
	for seen[key+strconv.Itoa(taken)] {
		taken++
	}
	seen[key+strconv.Itoa(taken)] = true
	if taken < len(found) {
		return found[taken]
	}
	if len(found) > 0 {
		return found[len(found)-1] // one error more than candidates: a repeat, dropped by the caller
	}
	return parent // nothing holds the name there: the nearest location known to be right
}

// findPropertyObjects lists, in document order, the mappings at the given
// depth under parent that hold the property, and, when under is given, sit
// under that key.
func (c *checker) findPropertyObjects(property string, depth int, parent []string, under string) [][]string {
	var out [][]string
	walk(c.root, nil, func(n *yaml.Node, path []string) {
		if len(path) != depth || len(path) < len(parent) || !slices.Equal(path[:len(parent)], parent) {
			return
		}
		if under != "" && (len(path) == 0 || path[len(path)-1] != under) {
			return
		}
		if source.Key(n, property) == nil {
			return
		}
		out = append(out, slices.Clone(path))
	})
	return out
}

func leaves(ve *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(ve.Causes) == 0 {
		return []*jsonschema.ValidationError{ve}
	}
	var out []*jsonschema.ValidationError
	for _, c := range ve.Causes {
		out = append(out, leaves(c)...)
	}
	return out
}

func plainSchemaMessage(k jsonschema.ErrorKind) string {
	switch e := k.(type) {
	case *kind.Type:
		// Got is the value's type in JSON Schema's instance data model,
		// never integer: 1 and 1.0 are both a number (ADR-072).
		return fmt.Sprintf("this is %s, but %s is expected here", article(e.Got), joinOr(e.Want))
	case *kind.Enum:
		return fmt.Sprintf("%s is not allowed here; use one of %s", display(e.Got), joinValues(e.Want))
	case *kind.Const:
		return fmt.Sprintf("this must be %s", display(e.Want))
	case *kind.Pattern:
		return fmt.Sprintf("%q does not have the right form; it must be %s", e.Got, describePattern(e.Want))
	case *kind.Format:
		return fmt.Sprintf("%s is not a valid %s; correct it", display(e.Got), formatName(e.Want))
	case *kind.MinItems:
		if e.Want == 1 {
			return "this list is empty; give at least one item, or leave the key out where that is allowed"
		}
		return fmt.Sprintf("this list has %d items and needs at least %d", e.Got, e.Want)
	case *kind.MinLength:
		return "this text is empty; write it, or leave the key out where that is allowed"
	case *kind.MinProperties:
		return "this object is empty; give at least one entry"
	case *kind.MaxProperties:
		return fmt.Sprintf("this object has %d entries and allows at most %d", e.Got, e.Want)
	case *kind.MaxItems:
		return fmt.Sprintf("this list has %d items and allows at most %d", e.Got, e.Want)
	case *kind.UniqueItems:
		return fmt.Sprintf("items %d and %d are the same; remove the repeat", e.Duplicates[0], e.Duplicates[1])
	case *kind.Minimum:
		return fmt.Sprintf("%s is below the minimum of %s", e.Got.RatString(), e.Want.RatString())
	case *kind.ExclusiveMinimum:
		return fmt.Sprintf("%s must be above %s", e.Got.RatString(), e.Want.RatString())
	case *kind.OneOf:
		if len(e.Subschemas) == 0 {
			return "this matches none of the allowed forms"
		}
		return "this matches more than one allowed form; write only one of them (for a field, either $ref or type, not both)"
	case *kind.FalseSchema:
		return "this key is not allowed here; remove it"
	case *kind.Not:
		return "this value is not allowed here"
	case *kind.Contains:
		return "this list does not hold the item it needs"
	}
	return k.LocalizedString(printer)
}

func article(t string) string {
	switch t {
	case "object", "array", "integer":
		if t == "integer" {
			return "an integer"
		}
		return "an " + t
	case "null":
		return "empty (null)"
	}
	return "a " + t
}

func joinOr(ts []string) string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = article(t)
	}
	return strings.Join(out, " or ")
}

func joinValues(vs []any) string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = display(v)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

func display(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return "null"
	case json.Number:
		return x.String()
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func formatName(f string) string {
	switch f {
	case "date":
		return "date (write it as \"YYYY-MM-DD\")"
	case "regex":
		return "regular expression"
	}
	return f
}
