package validate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"go.yaml.in/yaml/v3"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/schema"
)

const (
	designSchemaID         = "https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-design-0.1.schema.json"
	implementationSchemaID = "https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-implementation-0.1.schema.json"
)

var (
	compileOnce  sync.Once
	designSchema *jsonschema.Schema
	implSchema   *jsonschema.Schema
	compileErr   error
	printer      = message.NewPrinter(language.English)
)

func compileSchemas() {
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	for id, raw := range map[string][]byte{designSchemaID: schema.Definition, implementationSchemaID: schema.Implementation} {
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
	implSchema, compileErr = c.Compile(implementationSchemaID)
}

// Plain descriptions of the naming patterns in the schemas, so a message can
// say "camelCase, such as dueOn" instead of quoting a regular expression.
var patternNames = map[string]string{
	"^[A-Z][A-Za-z0-9]*$":                                          "PascalCase, such as Loan",
	"^[a-z][A-Za-z0-9]*$":                                          "camelCase, such as dueOn",
	"^[a-z][a-z0-9]*(\\.[a-z][a-z0-9]*)*$":                         "dotted lower case, such as loans.create",
	"^[a-z][a-z0-9]*(-[a-z0-9]+)*$":                                "kebab-case, such as members-list",
	"^[a-z][a-z0-9]*(_[a-z0-9]+)*$":                                "snake_case, such as loan_due_after_loaned",
	"^[A-Z][A-Z0-9]{1,15}-[A-Za-z0-9._]+$":                         "an ID: an upper-case prefix, a dash and a number or name, such as LIB-5 or NEED-1",
	"^[A-Z][A-Z0-9]{0,15}-[A-Za-z0-9._]+$":                         "a question ID: an upper-case prefix, a dash and a number or name, such as Q-12 or OPEN-3",
	"^[A-Z][A-Z0-9]{1,15}$":                                        "an upper-case prefix of 2 to 16 letters or digits, such as LIB",
	"^ADR-[0-9]{3,}$":                                              "ADR- and three or more digits, such as ADR-001",
	"^[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?$":                 "a semantic version, such as 1.2.0",
	"^#/(entities|enums)/[A-Z][A-Za-z0-9]*$":                       "#/entities/Name or #/enums/Name",
	"^/":                                                           "a path starting with /",
	"^([1-5][0-9][0-9]|default)$":                                  "an HTTP status code such as 200, or default",
	"^[a-z]+/[a-z0-9.+-]+$":                                        "a media type, such as application/json",
	"^[a-z][a-z0-9]*(\\.[a-z][a-z0-9]*)*/[A-Z][A-Za-z0-9]*$":       "channel/Message, such as loan.lifecycle/LoanCreated",
	"^[a-z][a-z0-9]*(-[a-z0-9]+)*( [a-z][a-z0-9]*(-[a-z0-9]+)*)*$": "kebab-case words separated by single spaces, such as generate techspec",
	"^(0|[1-9][0-9]?|1[0-9][0-9]|2[0-4][0-9]|25[0-5])$":            "an exit status from 0 to 255",
	"(^|/)specarch\\.yaml$":                                        "a path ending in specarch.yaml, the specification's root file",
	"^[A-Za-z][A-Za-z0-9 ./'-]*$":                                  "a term: letters, digits, spaces, dots, slashes, apostrophes and dashes, such as late fee",
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
	if k == KindImplementation {
		sch = implSchema
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

// schemaLeaves reports the innermost causes, one per value and kind of
// problem, in plain words. parent is the location of the enclosing error.
func (c *checker) schemaLeaves(k Kind, ve *jsonschema.ValidationError, parent []string, seen map[string]bool) {
	if pn, ok := ve.ErrorKind.(*kind.PropertyNames); ok {
		c.schemaPropertyName(k, ve, c.propertyNameObject(pn.Property, ve.InstanceLocation, ve.SchemaURL), pn, seen)
		return
	}
	if len(ve.Causes) > 0 {
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
// schema library stores that error's location in a slice it goes on
// changing, so only the length of the location it reports can be trusted.
// The schema location says under which key the object sits
// (".../properties/responses/propertyNames"). Of the mappings at that depth
// that sit under that key and hold the property, the one whose path agrees
// most with the reported location is taken.
func (c *checker) propertyNameObject(property string, reported []string, schemaURL string) []string {
	under := ""
	if _, frag, ok := strings.Cut(schemaURL, "#"); ok {
		tokens := strings.Split(strings.TrimPrefix(frag, "/"), "/")
		if n := len(tokens); n >= 3 && tokens[n-1] == "propertyNames" && (tokens[n-3] == "properties" || tokens[n-3] == "$defs") {
			under = source.UnescapeToken(tokens[n-2])
		}
	}
	if found := c.findPropertyObject(property, reported, under); found != nil {
		return found
	}
	if found := c.findPropertyObject(property, reported, ""); found != nil {
		return found
	}
	return reported
}

func (c *checker) findPropertyObject(property string, reported []string, under string) []string {
	var best []string
	bestScore := -1
	walk(c.root, nil, func(n *yaml.Node, path []string) {
		if len(path) != len(reported) || source.Key(n, property) == nil {
			return
		}
		if under != "" && (len(path) == 0 || path[len(path)-1] != under) {
			return
		}
		score := 0
		for i := range path {
			if path[i] == reported[i] {
				score++
			}
		}
		if score > bestScore {
			best, bestScore = slices.Clone(path), score
		}
	})
	return best
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
