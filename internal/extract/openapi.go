package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/wirename"
)

// The OpenAPI versions the reader takes, and the names the meta-model
// gives an entity, an enum value, a field and a media type.
var (
	openapiVersion = regexp.MustCompile(`^3\.[01]\.[0-9]+$`)
	typeNameWord   = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	memberNameWord = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
	enumValueWord  = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
	mediaTypeWord  = regexp.MustCompile(`^[a-z]+/[a-z0-9.+-]+$`)
	responseCode   = regexp.MustCompile(`^([1-5][0-9][0-9]|default)$`)
	templateParam  = regexp.MustCompile(`\{([^}/]+)\}`)
)

// The keywords of a schema the field subset holds as OpenAPI writes them,
// in the order they are written.
var fieldKeywords = []string{"title", "description", "enum", "const", "default", "examples", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum",
	"multipleOf", "minLength", "maxLength", "pattern", "minItems", "maxItems", "uniqueItems", "readOnly", "writeOnly", "deprecated"}

// The formats a field can take (docs/conventions.md, Types). float has no
// row: a number is a double or a decimal.
var fieldFormats = map[string]bool{"int32": true, "int64": true, "uint64": true, "double": true, "date": true, "date-time": true, "time": true, "duration": true,
	"email": true, "uuid": true, "uri": true, "hostname": true, "ipv4": true, "ipv6": true, "byte": true, "binary": true, "password": true}

var fieldTypes = map[string]bool{"string": true, "integer": true, "number": true, "boolean": true, "array": true, "object": true}

// OpenAPI reads one OpenAPI 3.0 or 3.1 document into a tree: its
// operations under their paths, its component schemas as entities,
// schemas and enums, and a question for what the document does not say
// (ADR-049).
func OpenAPI(path, out, key string) (*Result, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".yaml" && ext != ".yml" && ext != ".json" {
		return nil, refuse("%s is not an OpenAPI document: this build reads one written in YAML (.yaml, .yml) or JSON (.json)", path)
	}
	r, err := Open([]string{path})
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc *yaml.Node
	if ext == ".json" {
		doc, err = jsonNode(data)
	} else {
		var n yaml.Node
		err = yaml.Unmarshal(data, &n)
		if err == nil && len(n.Content) == 1 {
			doc = n.Content[0]
		}
	}
	if err != nil || doc == nil || doc.Kind != yaml.MappingNode {
		reason := "it holds no mapping"
		if err != nil {
			reason = err.Error()
		}
		return nil, refuse("%s is not an OpenAPI document: %s", path, reason)
	}
	version := scalar(child(doc, "openapi"))
	if !openapiVersion.MatchString(version) {
		if version == "" {
			version = "missing"
		}
		return nil, refuse("%s is not an OpenAPI 3.0 or 3.1 document: its openapi is %s", path, version)
	}
	if key == "" {
		key = kebab(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	}
	if !regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`).MatchString(key) {
		return nil, fmt.Errorf("the file name %s gives no source key; name one with --source-key", filepath.Base(path))
	}
	res := &Result{Tree: newTree()}
	commitLine(res, r)
	o := &openapiReader{doc: doc, v30: strings.HasPrefix(version, "3.0."), key: key, res: res, questions: &yaml.Node{Kind: yaml.MappingNode}}
	o.snake = snakeWire(doc)
	if o.snake {
		res.say("wire names: snake_case: every property name of the document that has two words joins them with an underscore; each is written in camelCase, and info.wireNames says how it goes on the wire")
	}
	if line, text, ok := generatedMark(path); ok {
		res.say("generated: %s:%d says it is generated from another source: %s", r.Paths[0], line, text)
	}
	o.read()

	url, err := relativeURL(out, filepath.Join(r.Repository.Root, filepath.FromSlash(r.Paths[0])))
	if err != nil {
		return nil, err
	}
	title := scalar(child(child(doc, "info"), "title"))
	if title == "" {
		title = filepath.Base(path)
	}
	src := mapping("kind", "document", "title", title, "edition", r.Commit, "url", url, "clauses", o.clauses)
	var stages []string
	design := mapping()
	if len(o.paths.Content) > 0 {
		set(design, "paths", o.paths)
	}
	if o.entities != nil {
		set(design, "entities", o.entities)
	}
	if o.schemas != nil {
		set(design, "schemas", o.schemas)
	}
	if o.enums != nil {
		set(design, "enums", o.enums)
	}
	if o.permits != nil {
		set(design, "permissions", o.permits)
	}
	if len(design.Content) > 0 {
		stages = []string{"design"}
		res.Tree.put("design/"+key+".yaml", design)
	}
	if len(o.questions.Content) > 0 {
		stages = []string{"requirements", "design"}
		res.Tree.put("requirements/stakeholders.yaml", mapping("stakeholders", ownerStakeholder()))
		res.Tree.put("design/questions.yaml", mapping("questions", o.questions))
	}
	description := fmt.Sprintf("The OpenAPI document %s, read at commit %s: every operation and component schema is a clause, every operation is written under its path citing it, every component schema of type object that an operation creates or a path with a parameter answers is an entity, and every other one a schema. What the document does not say is a question, and so is what the meta-model cannot hold.\n", r.Paths[0], r.Commit)
	root := rootFile(title, description, stages, mapping(key, src))
	if o.snake {
		set(child(root, "info"), "wireNames", wirename.SnakeCase)
	}
	res.Tree.put("specarch.yaml", root)
	return res, nil
}

// snakeWire reports whether a document names its properties in snake_case
// (ADR-062): one name at least joins words with an underscore, and none
// has a capital. A name of one word is both, and a document that mixes the
// two has camelCase names, which the meta-model takes as written.
func snakeWire(doc *yaml.Node) bool {
	underscore, capital := false, false
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		switch n.Kind {
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				k, v := n.Content[i].Value, n.Content[i+1]
				switch {
				case k == "example" || k == "examples" || k == "default" || k == "enum" || k == "const" || strings.HasPrefix(k, "x-"):
					continue
				case k == "properties" && v.Kind == yaml.MappingNode:
					for j := 0; j+1 < len(v.Content); j += 2 {
						name := v.Content[j].Value
						underscore = underscore || strings.Contains(name, "_")
						capital = capital || strings.ToLower(name) != name
					}
				}
				walk(v)
			}
		case yaml.SequenceNode, yaml.DocumentNode:
			for _, c := range n.Content {
				walk(c)
			}
		}
	}
	walk(doc)
	return underscore && !capital
}

type openapiReader struct {
	doc       *yaml.Node
	v30       bool
	snake     bool // the document names its properties in snake_case
	key       string
	res       *Result
	clauses   []*yaml.Node
	paths     *yaml.Node
	entities  *yaml.Node
	schemas   *yaml.Node
	enums     *yaml.Node
	questions *yaml.Node
	// The permissions dxlib's x-dxlib-privileges names (ADR-076), each
	// with the operations that check it, in the order read.
	dxlibOps  int // operations in dxlib's dialect
	checkedBy map[string][]string
	firstAt   map[string]string
	permits   *yaml.Node
	nextID    int
	notHeld   []notHeld
	// The component schemas written as an entity, a schema or an enum, by
	// name; a reference to any other is written in place.
	written map[string]string
	// The numbers with no width the meta-model holds, by the element
	// asked about, in the order read.
	widths []string
}

// gap records what the meta-model cannot hold at a pointer: into the tree
// (#/...), where the element it is about is written, or into the document
// (/...) for what no element holds. Its clause and the entry it blocks are
// found once everything is written (held).
func (o *openapiReader) gap(at string, format string, args ...any) {
	o.notHeld = append(o.notHeld, notHeld{text: fmt.Sprintf(format, args...), clause: at})
}

// held is what was not held, each citing the clause of the document it is
// in (the operation, the path or the component schema) and blocking the
// element written from that clause, or its section when none is.
func (o *openapiReader) held() []notHeld {
	var out []notHeld
	for _, h := range o.notHeld {
		at := h.clause
		if !strings.HasPrefix(at, "#/") {
			block := "paths"
			switch {
			case at == "/components/securitySchemes":
				block = "permissions"
			case strings.HasPrefix(at, "/components/schemas/"):
				block = "schemas"
			}
			h.clause, h.blocks = at, []string{block}
			out = append(out, h)
			continue
		}
		tokens := strings.Split(strings.TrimPrefix(at, "#/"), "/")
		section, name := tokens[0], tokens[1]
		element := "#/" + section + "/" + name
		var written *yaml.Node
		switch section {
		case "paths":
			h.clause = "/paths/" + name
			written = child(o.paths, strings.NewReplacer("~1", "/", "~0", "~").Replace(name))
			if len(tokens) > 2 && contains(pathMethods, tokens[2]) && child(written, tokens[2]) != nil {
				h.clause += "/" + tokens[2]
				element += "/" + tokens[2]
			}
		case "entities":
			h.clause = "/components/schemas/" + escapeToken(name)
			written = child(o.entities, name)
		case "schemas":
			h.clause = "/components/schemas/" + escapeToken(name)
			written = child(o.schemas, name)
		}
		h.blocks = []string{element}
		if written == nil {
			h.blocks = []string{section}
		}
		out = append(out, h)
	}
	return out
}

func (o *openapiReader) question(text string, blocks []string, why string) {
	o.nextID++
	set(o.questions, fmt.Sprintf("Q-%d", o.nextID), mapping(
		"question", text,
		"kind", "decision",
		"priority", "must",
		"blocks", blocks,
		"decidedBy", owner,
		"why", why,
	))
}

// read writes the component schemas first, so that a reference knows
// whether it names an entity, then the operations in the order of their
// paths and methods.
func (o *openapiReader) read() {
	o.written = map[string]string{}
	schemas := child(child(o.doc, "components"), "schemas")
	names := sortedKeys(schemas)
	resources := o.resources()
	for _, name := range names {
		s := o.resolve(child(schemas, name))
		switch {
		case !typeNameWord.MatchString(name):
		case schemaType(s) == "object" && child(s, "properties") != nil && resources[name]:
			o.written[name] = "entities"
		case schemaType(s) == "object" && child(s, "properties") != nil:
			o.written[name] = "schemas"
		case scalar(child(s, "type")) == "string" && isEnumOfWords(child(s, "enum")):
			o.written[name] = "enums"
		}
	}
	// An object none of whose properties can be held is no entity or
	// schema, and a reference to it is written in place; leaving one out
	// can leave another with none, so this runs until nothing changes.
	for changed := true; changed; {
		changed = false
		for _, name := range names {
			if o.written[name] != "entities" && o.written[name] != "schemas" {
				continue
			}
			trial := *o
			trial.notHeld, trial.widths = nil, nil
			if props, _ := trial.properties(o.resolve(child(schemas, name)), "", "#/"+o.written[name]+"/"+name); props == nil {
				delete(o.written, name)
				changed = true
			}
		}
	}
	o.paths = &yaml.Node{Kind: yaml.MappingNode}
	operations, pathCount := 0, 0
	for _, p := range sortedKeys(child(o.doc, "paths")) {
		item := o.resolve(child(child(o.doc, "paths"), p))
		if n := o.pathItem(p, item); n > 0 {
			operations += n
			pathCount++
		}
	}
	o.declarePermissions()
	if o.dxlibOps > 0 {
		o.res.say("dialect: dxlib: %s carry x-dxlib-endpoint-type, and each one's x-dxlib-privileges is read as its permission; %s declared, each named alone by an operation's privileges", plural(o.dxlibOps, "operation"), plural(len(o.checkedBy), "permission"))
	}
	entities, values, enums := 0, 0, 0
	for _, name := range names {
		pointer := "/components/schemas/" + escapeToken(name)
		o.clauses = append(o.clauses, flow(mapping("clause", pointer, "title", "schema "+name)))
		s := o.resolve(child(schemas, name))
		switch o.written[name] {
		case "entities":
			if o.entity(name, pointer, s) {
				entities++
			}
		case "schemas":
			if o.valueObject(name, pointer, s) {
				values++
			}
		case "enums":
			o.enum(name, pointer, s)
			enums++
		default:
			why := "it is neither an object with properties nor a string enum of snake_case values"
			if schemaType(s) == "object" && child(s, "properties") != nil {
				why = "none of its properties can be held"
			}
			if !typeNameWord.MatchString(name) {
				why = "its name is not PascalCase, which an entity's or an enum's is"
			}
			o.gap("/components/schemas/"+escapeToken(name), "schema %s: %s; written in place wherever it is referred to", name, why)
		}
	}
	o.askWidths()
	for _, k := range keys(o.doc) {
		switch k {
		case "openapi", "info", "paths", "components", "security":
		default:
			o.gap("/"+escapeToken(k), "the document's %s: the meta-model does not hold it; left out", k)
		}
	}
	for _, k := range keys(child(o.doc, "components")) {
		switch k {
		case "schemas", "parameters", "requestBodies", "responses":
			// Read where an operation refers to them.
		case "securitySchemes":
			if o.dxlibOps > 0 {
				o.gap("/components/securitySchemes", "components.securitySchemes: a security scheme is not a permission; named in each operation's permission question, or, for an operation in dxlib's dialect, in a line of its own, and left out")
			} else {
				o.gap("/components/securitySchemes", "components.securitySchemes: a security scheme is not a permission; named in each operation's permission question, and left out")
			}
		default:
			o.gap("/components/"+escapeToken(k), "components.%s: %s; left out", k, notHeldKey(k))
		}
	}
	countedPaths := len(sortedKeys(child(o.doc, "paths")))
	o.res.say("counted %s and %s: every key of paths and of components.schemas", plural(countedPaths, "path"), plural(len(names), "component schema"))
	o.res.say("wrote %s on %s, %s, %s, %s and %s: one operation per get, post, put, patch or delete, one entity per object schema an operation creates or a path with a parameter answers, one schema per other object schema, one enum per string enum, one question per thing the document does not say, and one per thing the meta-model cannot hold",
		plural(operations, "operation"), plural(pathCount, "path"), plural(entities, "entity"), plural(values, "schema"), plural(enums, "enum"), plural(o.nextID+couldCount(oneQuestions(o.questions), o.held()), "question"))
	askNotHeld(o.res, oneQuestions(o.questions), &o.nextID, o.key, o.held())
}

// pathItem writes the operations of one path and returns how many.
func (o *openapiReader) pathItem(p string, item *yaml.Node) int {
	var methods []string
	for _, k := range keys(item) {
		switch k {
		case "get", "post", "put", "patch", "delete":
			methods = append(methods, k)
		case "head", "options", "trace":
			o.clauses = append(o.clauses, flow(mapping("clause", "/paths/"+escapeToken(p)+"/"+k, "title", strings.ToUpper(k)+" "+p)))
			o.gap("#/paths/"+escapeToken(p), "operation %s %s: the meta-model holds the methods get, post, put, patch and delete; left out", strings.ToUpper(k), p)
		case "parameters", "summary", "description":
		default:
			o.gap("#/paths/"+escapeToken(p), "path %s: %s; left out", p, notHeldKey(k))
		}
	}
	sort.Slice(methods, func(i, j int) bool { return methodRank(methods[i]) < methodRank(methods[j]) })
	if len(methods) == 0 {
		return 0
	}
	params, reason := pathParameters(p)
	if reason != "" {
		for _, m := range methods {
			o.clauses = append(o.clauses, flow(mapping("clause", "/paths/"+escapeToken(p)+"/"+m, "title", strings.ToUpper(m)+" "+p)))
		}
		o.gap("#/paths/"+escapeToken(p), "path %s: %s; its operations are left out", p, reason)
		return 0
	}
	at := "#/paths/" + escapeToken(p)
	out := mapping()
	if d := scalar(child(item, "description")); d != "" {
		set(out, "description", d)
	} else if s := scalar(child(item, "summary")); s != "" {
		set(out, "description", s)
	}
	declared := map[string]bool{}
	var shared []*yaml.Node
	for _, pn := range items(child(item, "parameters")) {
		if param := o.parameter(o.resolve(pn), fmt.Sprintf("path %s", p), at+fmt.Sprintf("/parameters/%d", len(shared)), params); param != nil {
			shared = append(shared, param)
			if scalar(child(param, "in")) == "path" {
				declared[scalar(child(param, "name"))] = true
			}
		}
	}
	// A parameter of the template that no operation declares is written on
	// the path, its values asked for.
	var missing []string
	for _, name := range params {
		inAll := true
		for _, m := range methods {
			found := false
			for _, pn := range items(child(o.resolve(child(item, m)), "parameters")) {
				pn = o.resolve(pn)
				found = found || (scalar(child(pn, "in")) == "path" && scalar(child(pn, "name")) == name)
			}
			inAll = inAll && found
		}
		if !declared[name] && !inAll {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		var blocks []string
		for _, name := range missing {
			blocks = append(blocks, fmt.Sprintf("%s/parameters/%d/schema", at, len(shared)))
			shared = append(shared, flow(mapping("name", name, "in", "path", "required", true)))
		}
		o.question(fmt.Sprintf("What values does each path parameter of %s that the document does not declare take: %s?", p, strings.Join(missing, ", ")),
			blocks, "The path's template names the parameter, and the document declares no schema for it.")
	}
	if len(shared) > 0 {
		set(out, "parameters", shared)
	}
	for _, m := range methods {
		set(out, m, o.operation(p, m, o.resolve(child(item, m)), params))
	}
	set(o.paths, p, out)
	return len(methods)
}

// operation writes one operation in the meta-model's terms.
func (o *openapiReader) operation(p, method string, op *yaml.Node, params []string) *yaml.Node {
	pointer := "/paths/" + escapeToken(p) + "/" + method
	label := strings.ToUpper(method) + " " + p
	o.clauses = append(o.clauses, flow(mapping("clause", pointer, "title", label)))
	at := "#" + pointer
	out := mapping()
	id := scalar(child(op, "operationId"))
	if !memberNameWord.MatchString(id) {
		named := method
		for _, seg := range strings.Split(p, "/") {
			if m := routeSegment.FindStringSubmatch(seg); m != nil {
				seg = "by_" + m[1]
			}
			named += pascal(notWord.ReplaceAllString(seg, "_"))
		}
		if id == "" {
			o.gap(at, "operation %s: it has no operationId; it is named %s", label, named)
		} else {
			o.gap(at, "operation %s: its operationId %s is not camelCase, which an operationId is; it is named %s", label, id, named)
		}
		id = named
	}
	set(out, "operationId", id)
	summary := scalar(child(op, "summary"))
	set(out, "summary", nonEmpty(summary))
	set(out, "description", nonEmpty(scalar(child(op, "description"))))
	var unasked []string
	if summary == "" {
		unasked = append(unasked, at+"/summary")
	}
	var list []*yaml.Node
	for _, pn := range items(child(op, "parameters")) {
		if param := o.parameter(o.resolve(pn), "operation "+label, fmt.Sprintf("%s/parameters/%d", at, len(list)), params); param != nil {
			list = append(list, param)
		}
	}
	if len(list) > 0 {
		set(out, "parameters", list)
	}
	if rb := child(op, "requestBody"); rb != nil {
		set(out, "requestBody", o.requestBody(o.resolve(rb), label, at+"/requestBody"))
	}
	responses := mapping()
	for _, code := range keys(child(op, "responses")) {
		if !responseCode.MatchString(code) {
			o.gap(at, "operation %s, response %s: the meta-model holds a response code or default, not a range; left out", label, code)
			continue
		}
		at := at + "/responses/" + code
		set(responses, code, o.response(o.resolve(child(child(op, "responses"), code)), "operation "+label+", response "+code, at))
	}
	if len(responses.Content) > 0 {
		set(out, "responses", responses)
	} else {
		unasked = append(unasked, at+"/responses")
	}
	if child(op, "deprecated") != nil && scalar(child(op, "deprecated")) == "true" {
		set(out, "deprecated", true)
	}
	dxlib := child(op, "x-dxlib-endpoint-type") != nil
	for _, k := range keys(op) {
		switch {
		case k == "operationId", k == "summary", k == "description", k == "parameters", k == "requestBody", k == "responses", k == "deprecated", k == "security":
		case k == "x-dxlib-privileges" && dxlib:
			// Read below as the operation's permission.
		default:
			o.gap(at, "operation %s: %s; left out", label, notHeldKey(k))
		}
	}
	set(out, "origin", "stated")
	says := label + ": " + summary
	if summary == "" {
		says = label + ", with no summary."
	} else if !strings.HasSuffix(says, ".") {
		says += "."
	}
	set(out, "cites", []*yaml.Node{citation(o.key, pointer, says)})
	if len(unasked) > 0 {
		o.question(fmt.Sprintf("What does %s do, and what does it answer? The document gives no %s.", label, missingWhat(unasked)),
			unasked, "An operation needs a summary and its responses, and the document does not give them.")
	}
	if dxlib {
		o.dxlibOps++
		o.dxlibPermission(out, op, label, pointer, at)
		return out
	}
	o.question(fmt.Sprintf("Which permission does %s check? %s", label, o.securityText(op)),
		[]string{at + "/permission"}, "A security scheme says how a caller proves who it is, not what it may do, so the permission is asked and never written as public.")
	return out
}

// dxlibPermission reads the permission of an operation in dxlib's dialect
// (ADR-076): the one privilege x-dxlib-privileges names. dxlib lets a
// caller holding any one of an endpoint's privileges through, and one with
// none every caller its middleware admits, which the document does not
// name; so none, more than one, or a name that is no permission is asked.
func (o *openapiReader) dxlibPermission(out, op *yaml.Node, label, pointer, at string) {
	if child(op, "security") != nil || child(o.doc, "security") != nil {
		o.gap(at, "operation %s: %s A security scheme says how a caller proves who it is, and dxlib's privileges what it may do, so the security is not its permission; left out", label, o.securityText(op))
	}
	given := child(op, "x-dxlib-privileges")
	list := given != nil && given.Kind == yaml.SequenceNode
	var privileges []string
	seen := map[string]bool{}
	for _, item := range items(given) {
		if item.Kind != yaml.ScalarNode {
			list = false
			break
		}
		if !seen[item.Value] {
			seen[item.Value] = true
			privileges = append(privileges, item.Value)
		}
	}
	switch {
	case given != nil && !list:
		o.question(fmt.Sprintf("%s gives x-dxlib-privileges as %s, which is not a list of names as dxlib writes it. Which permission does it check?", label, inlineNode(given)),
			[]string{at + "/permission"}, "dxlib's dialect writes the privileges as a list, and a value in another form says nothing the reader can trust.")
	case len(privileges) == 1 && privileges[0] == "public":
		o.question(fmt.Sprintf("%s checks the privilege public, which dxlib grants only to a role that holds it, while public in a specification means open to everyone. Which permission should it check, or is it meant to be open?", label),
			[]string{at + "/permission"}, "Writing the privilege public as the permission public would open the operation to everyone, which the code does not do.")
	case len(privileges) == 1 && permissionWord.MatchString(privileges[0]):
		name := privileges[0]
		set(out, "permission", name)
		if o.checkedBy == nil {
			o.checkedBy, o.firstAt = map[string][]string{}, map[string]string{}
		}
		if _, seen := o.firstAt[name]; !seen {
			o.firstAt[name] = pointer
		}
		o.checkedBy[name] = append(o.checkedBy[name], label)
	case len(privileges) == 0:
		o.question(fmt.Sprintf("%s checks no privilege: its x-dxlib-privileges is empty or absent, so dxlib lets through every caller its middleware admits. Is it meant to be open to everyone (public), or which permission should it check?", label),
			[]string{at + "/permission"}, "dxlib checks no privilege on an endpoint that names none, and whether a caller must sign in is decided by middleware the document does not name; an open endpoint is never written as public without the owner's word.")
	case len(privileges) == 1:
		o.question(fmt.Sprintf("%s checks the privilege %s, which is not a permission name (lower-case words joined by dots). Which permission should it check?", label, privileges[0]),
			[]string{at + "/permission"}, "The meta-model names a permission in lower-case words joined by dots, and renaming the privilege would write a check the code does not make.")
	default:
		o.question(fmt.Sprintf("%s checks the privileges %s: dxlib lets a caller holding any one of them through, and an operation checks one permission. Which permission should it check?", label, joinAnd(privileges)),
			[]string{at + "/permission"}, "An operation names one permission; choosing one of dxlib's would write a check the code does not make.")
	}
}

// declarePermissions declares every permission dxlib's privileges named,
// citing the first operation that checks it, and asks what each allows
// and which role grants it, which the document does not say.
func (o *openapiReader) declarePermissions() {
	if len(o.checkedBy) == 0 {
		return
	}
	names := make([]string, 0, len(o.checkedBy))
	for n := range o.checkedBy {
		names = append(names, n)
	}
	sort.Strings(names)
	o.permits = &yaml.Node{Kind: yaml.MappingNode}
	var blocks, grants []string
	for _, n := range names {
		by := o.checkedBy[n]
		set(o.permits, n, mapping(
			"origin", "stated",
			"cites", []*yaml.Node{citation(o.key, o.firstAt[n], fmt.Sprintf("%s %s %s in x-dxlib-privileges.", joinAnd(by), checkOrChecks(len(by)), n))},
		))
		blocks = append(blocks, "#/permissions/"+escapeToken(n)+"/description")
		grants = append(grants, "#/permissions/"+escapeToken(n))
	}
	o.question(fmt.Sprintf("What does each permission allow: %s?", strings.Join(names, ", ")),
		blocks, "dxlib's document names the privilege an endpoint checks and not what it is for.")
	o.question(fmt.Sprintf("Which role grants each permission: %s?", strings.Join(names, ", ")),
		grants, "dxlib's document names the privilege an endpoint checks and not who holds it.")
}

// missingWhat names what an operation's question blocks.
func missingWhat(blocks []string) string {
	switch {
	case len(blocks) == 2:
		return "summary or responses"
	case strings.HasSuffix(blocks[0], "/summary"):
		return "summary"
	}
	return "responses"
}

// securityText says what the document gives as the operation's security.
func (o *openapiReader) securityText(op *yaml.Node) string {
	sec := child(op, "security")
	if sec == nil {
		sec = child(o.doc, "security")
	}
	if sec == nil {
		return "The document gives it no security."
	}
	var alts []string
	for _, req := range items(sec) {
		var parts []string
		for _, name := range keys(req) {
			scopes := scalarList(child(req, name))
			if len(scopes) > 0 {
				parts = append(parts, fmt.Sprintf("%s with the scopes %s", name, strings.Join(scopes, ", ")))
			} else {
				parts = append(parts, name)
			}
		}
		if len(parts) == 0 {
			alts = append(alts, "none")
		} else {
			alts = append(alts, strings.Join(parts, " and "))
		}
	}
	if len(alts) == 0 {
		return "The document says it needs no security."
	}
	return "The document gives it the security " + strings.Join(alts, ", or ") + "."
}

// parameter writes one parameter, or prints why it cannot.
func (o *openapiReader) parameter(pn *yaml.Node, where, at string, params []string) *yaml.Node {
	name, in := scalar(child(pn, "name")), scalar(child(pn, "in"))
	where = fmt.Sprintf("%s, parameter %s", where, name)
	switch {
	case pn == nil || name == "":
		o.gap(at, "%s: a parameter with no name; left out", where)
		return nil
	case in == "cookie":
		o.gap(at, "%s: the meta-model holds a parameter in the path, the query or a header, not a cookie; left out", where)
		return nil
	case in != "path" && in != "query" && in != "header":
		o.gap(at, "%s: in %q is not a place a parameter is sent; left out", where, in)
		return nil
	case child(pn, "schema") == nil:
		o.gap(at, "%s: it is given by content and not by a schema, and the meta-model holds a parameter's schema; left out", where)
		return nil
	case in == "path" && !contains(params, name):
		o.gap(at, "%s: the path's template has no {%s}; left out", where, name)
		return nil
	}
	out := mapping("name", name, "in", in)
	set(out, "description", nonEmpty(scalar(child(pn, "description"))))
	if in == "path" {
		set(out, "required", true)
	} else if scalar(child(pn, "required")) == "true" {
		set(out, "required", true)
	}
	schema := o.field(child(pn, "schema"), where, at+"/schema", 0)
	if schema == nil {
		return nil
	}
	set(out, "schema", schema)
	for _, k := range keys(pn) {
		switch k {
		case "name", "in", "description", "required", "schema":
		default:
			o.gap(at, "%s: %s; left out", where, notHeldKey(k))
		}
	}
	return out
}

func (o *openapiReader) requestBody(rb *yaml.Node, label, at string) *yaml.Node {
	content := o.content(child(rb, "content"), "operation "+label+", request body", at+"/content")
	if content == nil {
		o.gap(at, "operation %s, request body: no media type the meta-model holds; left out", label)
		return nil
	}
	out := mapping()
	set(out, "description", nonEmpty(scalar(child(rb, "description"))))
	if scalar(child(rb, "required")) == "true" {
		set(out, "required", true)
	}
	set(out, "content", content)
	return out
}

func (o *openapiReader) response(r *yaml.Node, where, at string) *yaml.Node {
	out := mapping()
	if d := scalar(child(r, "description")); d != "" {
		set(out, "description", d)
	} else {
		o.question(fmt.Sprintf("What does %s mean? The document gives it no description.", where), []string{at + "/description"},
			"A response needs a description, and the document does not give one.")
	}
	if c := child(r, "content"); c != nil {
		set(out, "content", o.content(c, where, at+"/content"))
	}
	for _, k := range keys(r) {
		switch k {
		case "description", "content":
		default:
			o.gap(at, "%s: %s; left out", where, notHeldKey(k))
		}
	}
	return out
}

func (o *openapiReader) content(c *yaml.Node, where, at string) *yaml.Node {
	out := mapping()
	for _, mt := range keys(c) {
		media := child(c, mt)
		switch {
		case !mediaTypeWord.MatchString(mt):
			o.gap(at, "%s, media type %s: the meta-model holds a media type of lower-case type and subtype, without wildcards or parameters; left out", where, mt)
			continue
		case child(media, "schema") == nil:
			o.gap(at, "%s, media type %s: it has no schema; left out", where, mt)
			continue
		}
		for _, k := range keys(media) {
			if k != "schema" {
				o.gap(at, "%s, media type %s: %s; left out", where, mt, notHeldKey(k))
			}
		}
		schema := o.field(child(media, "schema"), where+", media type "+mt, at+"/"+escapeToken(mt)+"/schema", 0)
		if schema == nil {
			continue
		}
		set(out, mt, mapping("schema", schema))
	}
	if len(out.Content) == 0 {
		return nil
	}
	return out
}

// entity writes a component schema of type object as an entity.
func (o *openapiReader) entity(name, pointer string, s *yaml.Node) bool {
	at := "#/entities/" + name
	props, required := o.properties(s, "schema "+name, at)
	if props == nil {
		o.gap(at, "schema %s: none of its properties can be held; left out", name)
		delete(o.written, name)
		return false
	}
	out := mapping("type", "object")
	set(out, "description", nonEmpty(scalar(child(s, "description"))))
	set(out, "properties", props)
	if len(required) > 0 {
		set(out, "required", required)
	}
	var fields []string
	for i := 0; i < len(props.Content); i += 2 {
		// The citation quotes the document, so it names each property as
		// the document does; a name read from snake_case goes back
		// unchanged.
		if o.snake {
			fields = append(fields, wirename.Snake(props.Content[i].Value))
		} else {
			fields = append(fields, props.Content[i].Value)
		}
	}
	set(out, "origin", "stated")
	set(out, "cites", []*yaml.Node{citation(o.key, pointer, fmt.Sprintf("The schema %s, with the properties %s.", name, joinAnd(fields)))})
	if o.entities == nil {
		o.entities = &yaml.Node{Kind: yaml.MappingNode}
	}
	set(o.entities, name, out)
	o.question(fmt.Sprintf("Which fields identify one %s, its primary key?", name), []string{at + "/primaryKey"},
		"A schema says what a record carries, not which of its fields identify it.")
	for _, k := range keys(s) {
		switch k {
		case "type", "properties", "required", "description", "title":
		default:
			o.gap(at, "schema %s: %s; left out", name, notHeldKey(k))
		}
	}
	return true
}

// valueObject writes a component schema of type object that no operation
// creates and no path with a parameter answers as a schema: data passed
// around, with no key to ask for.
func (o *openapiReader) valueObject(name, pointer string, s *yaml.Node) bool {
	at := "#/schemas/" + name
	props, required := o.properties(s, "schema "+name, at)
	if props == nil {
		o.gap(at, "schema %s: none of its properties can be held; left out", name)
		delete(o.written, name)
		return false
	}
	out := mapping("type", "object")
	set(out, "description", nonEmpty(scalar(child(s, "description"))))
	set(out, "properties", props)
	if len(required) > 0 {
		set(out, "required", required)
	}
	var fields []string
	for i := 0; i < len(props.Content); i += 2 {
		fields = append(fields, props.Content[i].Value)
	}
	set(out, "origin", "stated")
	set(out, "cites", []*yaml.Node{citation(o.key, pointer, fmt.Sprintf("The schema %s, with the properties %s.", name, joinAnd(fields)))})
	if o.schemas == nil {
		o.schemas = &yaml.Node{Kind: yaml.MappingNode}
	}
	set(o.schemas, name, out)
	for _, k := range keys(s) {
		switch k {
		case "type", "properties", "required", "description", "title":
		default:
			o.gap(at, "schema %s: %s; left out", name, notHeldKey(k))
		}
	}
	return true
}

// resources are the component schemas the document treats as records: one
// an operation's 201 answers, since 201 says a resource was created (RFC
// 9110, 15.3.2), or one a path with a parameter answers as it is, since
// the parameter names one record of it.
func (o *openapiReader) resources() map[string]bool {
	out := map[string]bool{}
	for _, p := range keys(child(o.doc, "paths")) {
		item := o.resolve(child(child(o.doc, "paths"), p))
		for _, method := range []string{"get", "post", "put", "patch", "delete"} {
			for _, code := range keys(child(child(item, method), "responses")) {
				if code != "201" && !(strings.Contains(p, "{") && strings.HasPrefix(code, "2")) {
					continue
				}
				r := o.resolve(child(child(child(item, method), "responses"), code))
				for _, mt := range keys(child(r, "content")) {
					ref := scalar(child(child(child(child(r, "content"), mt), "schema"), "$ref"))
					if name, ok := strings.CutPrefix(ref, "#/components/schemas/"); ok {
						out[name] = true
					}
				}
			}
		}
	}
	return out
}

func (o *openapiReader) enum(name, pointer string, s *yaml.Node) {
	values := scalarList(child(s, "enum"))
	out := mapping("type", "string", "enum", values)
	set(out, "description", nonEmpty(scalar(child(s, "description"))))
	set(out, "origin", "stated")
	set(out, "cites", []*yaml.Node{citation(o.key, pointer, fmt.Sprintf("The schema %s, one of %s.", name, joinAnd(values)))})
	if o.enums == nil {
		o.enums = &yaml.Node{Kind: yaml.MappingNode}
	}
	set(o.enums, name, out)
}

// properties writes an object schema's properties and the required list,
// each by its name in camelCase: as written, or read from snake_case when
// the document names its properties so. A name that cannot be written in
// camelCase and go back on the wire unchanged is left out.
func (o *openapiReader) properties(s *yaml.Node, where, at string) (*yaml.Node, []string) {
	var props *yaml.Node
	kept := map[string]string{}
	for _, wire := range keys(child(s, "properties")) {
		name, ok := o.memberName(wire, where, at)
		if !ok {
			continue
		}
		f := o.field(child(child(s, "properties"), wire), where+", property "+wire, at+"/properties/"+name, 0)
		if f == nil {
			continue
		}
		if props == nil {
			props = &yaml.Node{Kind: yaml.MappingNode}
		}
		set(props, name, f)
		kept[wire] = name
	}
	var required []string
	for _, r := range scalarList(child(s, "required")) {
		if name, ok := kept[r]; ok {
			required = append(required, name)
		}
	}
	return props, required
}

// memberName is the camelCase name of a property named wire on the wire,
// or false, with a line, when there is none that goes back unchanged.
func (o *openapiReader) memberName(wire, where, at string) (string, bool) {
	switch {
	case !o.snake && memberNameWord.MatchString(wire):
		return wire, true
	case !o.snake && enumValueWord.MatchString(wire):
		o.gap(at, "%s, property %s: its name is snake_case, and the document's other names are camelCase; a specification names its properties on the wire one way (info.wireNames); left out", where, wire)
	case !o.snake:
		o.gap(at, "%s, property %s: its name is neither camelCase nor snake_case; left out", where, wire)
	default:
		name, ok := wirename.Camel(wire)
		if ok && memberNameWord.MatchString(name) {
			return name, true
		}
		o.gap(at, "%s, property %s: read as %s, it goes on the wire as %s under info.wireNames snake_case, which is not its name; left out", where, wire, name, wirename.Snake(name))
	}
	return "", false
}

// field writes a schema in the meta-model's field subset (ADR-049).
func (o *openapiReader) field(s *yaml.Node, where, at string, depth int) *yaml.Node {
	if ref := scalar(child(s, "$ref")); ref != "" {
		// An entity may not hold a schema, so inside an entity a schema
		// is written in place.
		if name, ok := strings.CutPrefix(ref, "#/components/schemas/"); ok && o.written[name] != "" && !(o.written[name] == "schemas" && strings.HasPrefix(at, "#/entities/")) {
			return flow(mapping("$ref", "#/"+o.written[name]+"/"+name))
		}
		target := o.resolve(s)
		if target == s || depth > 8 {
			o.gap(at, "%s: the reference %s does not lead to a schema in the document that can be written in place; left out", where, ref)
			return nil
		}
		return o.field(target, where, at, depth+1)
	}
	out := mapping()
	typ := child(s, "type")
	if typ == nil && schemaType(s) != "" {
		// JSON Schema's properties constrain only an object, and its items
		// only an array, so either keyword says the type.
		typ = str(schemaType(s))
	}
	switch {
	case typ == nil:
	case typ.Kind == yaml.ScalarNode && fieldTypes[typ.Value]:
		if o.v30 && scalar(child(s, "nullable")) == "true" {
			set(out, "type", []string{typ.Value, "null"})
		} else {
			set(out, "type", typ.Value)
		}
	case typ.Kind == yaml.SequenceNode && len(typ.Content) == 2 && contains(scalarList(typ), "null"):
		set(out, "type", scalarList(typ))
	default:
		o.gap(at, "%s: the type %s is not one the meta-model holds; left out", where, inlineNode(typ))
	}
	base := scalar(typ)
	if typ != nil && typ.Kind == yaml.SequenceNode {
		for _, t := range scalarList(typ) {
			if t != "null" {
				base = t
			}
		}
	}
	format := scalar(child(s, "format"))
	switch {
	case (base == "integer" || base == "number") && !widthHeld(base, format, s):
		if format != "" {
			o.gap(at, "%s: the format %s is not a width the meta-model holds for a JSON %s; left out, and asked", where, format, base)
			o.notHeld[len(o.notHeld)-1].asked = at + "/format"
		}
		o.widths = append(o.widths, at+"/format")
	case format == "":
	case fieldFormats[format]:
		set(out, "format", format)
	default:
		o.gap(at, "%s: the format %s is not one the meta-model holds; left out", where, format)
	}
	for _, k := range fieldKeywords {
		v := child(s, k)
		if v == nil {
			continue
		}
		switch {
		case o.v30 && k == "exclusiveMinimum" || o.v30 && k == "exclusiveMaximum":
			// OpenAPI 3.0 writes the bound under minimum or maximum and
			// says here that it is exclusive.
			continue
		case k == "minimum" || k == "maximum":
			exclusive := "exclusiveMinimum"
			if k == "maximum" {
				exclusive = "exclusiveMaximum"
			}
			if o.v30 && scalar(child(s, exclusive)) == "true" {
				set(out, exclusive, copyNode(v))
				continue
			}
		}
		set(out, k, copyNode(v))
	}
	if o.v30 && child(s, "example") != nil {
		set(out, "examples", []*yaml.Node{copyNode(child(s, "example"))})
	}
	if items := child(s, "items"); items != nil {
		set(out, "items", o.field(items, where+", items", at+"/items", depth+1))
	}
	if child(s, "properties") != nil {
		props, required := o.properties(s, where, at)
		set(out, "properties", props)
		if len(required) > 0 {
			set(out, "required", required)
		}
	}
	for _, k := range keys(s) {
		switch k {
		case "type", "format", "items", "properties", "required", "nullable", "example":
		default:
			if !containsWord(fieldKeywords, k) {
				o.gap(at, "%s: %s; left out", where, notHeldKey(k))
			}
		}
	}
	if child(out, "type") == nil {
		o.gap(at, "%s: no type the meta-model holds is left; left out", where)
		return nil
	}
	if len(out.Content) <= 4 {
		flow(out)
	}
	return out
}

// schemaType is a schema's type, or the type its properties or items
// keyword implies when it names none.
func schemaType(s *yaml.Node) string {
	switch {
	case child(s, "type") != nil:
		return scalar(child(s, "type"))
	case child(s, "properties") != nil:
		return "object"
	case child(s, "items") != nil:
		return "array"
	}
	return ""
}

// widthHeld says whether a JSON integer or number has a width the
// meta-model holds: an int64 or uint64 only within 2^53, as the
// validator's unsafe_integer rule asks.
func widthHeld(base, format string, s *yaml.Node) bool {
	if base == "number" {
		return format == "double"
	}
	switch format {
	case "int32":
		return true
	case "int64", "uint64":
		safe := big.NewRat(9007199254740991, 1)
		high, ok := new(big.Rat).SetString(scalar(child(s, "maximum")))
		if !ok || high.Cmp(safe) > 0 {
			return false
		}
		if format == "uint64" {
			return true
		}
		low, ok := new(big.Rat).SetString(scalar(child(s, "minimum")))
		return ok && low.Cmp(new(big.Rat).Neg(safe)) >= 0
	}
	return false
}

// askWidths asks the width of every number that has none the meta-model
// holds, in one question.
func (o *openapiReader) askWidths() {
	if len(o.widths) == 0 {
		return
	}
	o.question("Which width does each of these numbers take: int32, int64 within 2^53 or carried as text, or a double or a decimal with its precision and scale?",
		o.widths, "The meta-model needs a number's width, and the document gives none it holds; choosing one would be a guess.")
}

// resolve follows a reference inside the document; anything else is
// returned as it is.
func (o *openapiReader) resolve(n *yaml.Node) *yaml.Node {
	for i := 0; i < 8; i++ {
		ref := scalar(child(n, "$ref"))
		if !strings.HasPrefix(ref, "#/") {
			return n
		}
		target := o.doc
		for _, tok := range strings.Split(ref[2:], "/") {
			tok = strings.NewReplacer("~1", "/", "~0", "~").Replace(tok)
			target = child(target, tok)
		}
		if target == nil {
			return n
		}
		n = target
	}
	return n
}

// --- Reading the document's nodes ---------------------------------------

func child(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func keys(n *yaml.Node) []string {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	var out []string
	for i := 0; i+1 < len(n.Content); i += 2 {
		out = append(out, n.Content[i].Value)
	}
	return out
}

func sortedKeys(n *yaml.Node) []string {
	k := keys(n)
	sort.Strings(k)
	return k
}

func items(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

func scalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

func scalarList(n *yaml.Node) []string {
	var out []string
	for _, i := range items(n) {
		out = append(out, scalar(i))
	}
	return out
}

func nonEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func isEnumOfWords(n *yaml.Node) bool {
	values := items(n)
	for _, v := range values {
		if v.Kind != yaml.ScalarNode || !enumValueWord.MatchString(v.Value) {
			return false
		}
	}
	return len(values) > 0
}

func containsWord(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// notHeldKey says why a key the reader does not write is left out.
func notHeldKey(k string) string {
	switch k {
	case "allOf", "oneOf", "anyOf", "not":
		return k + " combines schemas, which the field subset does not"
	case "additionalProperties":
		return "additionalProperties, which the field subset does not hold"
	case "discriminator":
		return "discriminator, which the field subset does not hold"
	case "$ref":
		return "a reference outside the document, which the reader does not follow"
	}
	if strings.HasPrefix(k, "x-") {
		return "the extension " + k + ", whose meaning the document's own tools define"
	}
	return k + ", which the meta-model does not hold here"
}

func inlineNode(n *yaml.Node) string {
	b, err := yaml.Marshal(flow(copyNode(n)))
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(b))
}

// jsonNode reads a JSON document into a YAML node, keeping the order of
// its keys and writing each number as it was written.
func jsonNode(data []byte) (*yaml.Node, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var read func() (*yaml.Node, error)
	read = func() (*yaml.Node, error) {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case json.Delim:
			if t == '{' {
				m := &yaml.Node{Kind: yaml.MappingNode}
				for dec.More() {
					k, err := dec.Token()
					if err != nil {
						return nil, err
					}
					v, err := read()
					if err != nil {
						return nil, err
					}
					m.Content = append(m.Content, str(k.(string)), v)
				}
				_, err := dec.Token()
				return m, err
			}
			s := &yaml.Node{Kind: yaml.SequenceNode}
			for dec.More() {
				v, err := read()
				if err != nil {
					return nil, err
				}
				s.Content = append(s.Content, v)
			}
			_, err := dec.Token()
			return s, err
		case string:
			return str(t), nil
		case json.Number:
			if strings.ContainsAny(t.String(), ".eE") {
				return literal("!!float", t.String()), nil
			}
			return literal("!!int", t.String()), nil
		case bool:
			return value(t), nil
		case nil:
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
		}
		return nil, fmt.Errorf("unexpected %v", tok)
	}
	n, err := read()
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("text after the document")
	}
	return n, nil
}
