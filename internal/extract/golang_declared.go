package extract

import (
	"fmt"
	"go/ast"
	gotoken "go/token"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/wirename"
)

// What an endpoint declares where it is registered (ADR-130): the
// parameters it takes, the content type its body arrives in and the
// responses it may answer, which dxlib's NewEndPoint receives as its
// seventh, sixth and tenth arguments. The reader recognises a parameter by
// its keys, not by the name of its type: a composite literal with NameId
// and Type is a parameter whatever package declares it, and one with
// StatusCode a response; so a project's own helpers that build such a list
// are read as long as they return a literal.

// dxType is how dxlib writes one of its parameter types in JSON Schema
// (dxlib's openAPITypeTable): a JSON type, a format, the bounds the type
// implies, and where the parameter's children go.
type dxType struct {
	json, format     string
	minimum          string // "" when the type implies none
	exclusiveMinimum string
	minLength        string
	items            *dxType // the items of an array whose items the type gives
	children         string  // "properties" or "items" where Children go, "" when dxlib reads none
	mapOfStrings     bool
}

var (
	dxString  = dxType{json: "string"}
	dxInt64   = dxType{json: "integer", format: "int64"}
	dxObjects = dxType{json: "object"}
)

// The parameter types by their value, as types.go declares them.
var dxTypes = map[string]dxType{
	"encrypted-blob": {json: "string", format: "byte"},
	"blob":           {json: "string", format: "binary"},

	"string": dxString, "protected-string": dxString, "protected-sql-string": dxString, "nullable-string": dxString,
	"protected-non-empty-string": {json: "string", minLength: "1"},
	"non-empty-string":           {json: "string", minLength: "1"},
	"email":                      {json: "string", format: "email"},
	"phonenumber":                dxString,
	"npwp":                       dxString,

	"int32":          {json: "integer", format: "int32"},
	"int32p":         {json: "integer", format: "int32", minimum: "1"},
	"int32zp":        {json: "integer", format: "int32", minimum: "0"},
	"nullable-int32": {json: "integer", format: "int32"},
	"int64":          dxInt64,
	"int64p":         {json: "integer", format: "int64", minimum: "1"},
	"int64zp":        {json: "integer", format: "int64", minimum: "0"},
	"nullable-int64": dxInt64,
	"id":             dxInt64,

	"float32":   {json: "number", format: "float"},
	"float32p":  {json: "number", format: "float", exclusiveMinimum: "0"},
	"float32zp": {json: "number", format: "float", minimum: "0"},
	"float64":   {json: "number", format: "double"},
	"float64p":  {json: "number", format: "double", exclusiveMinimum: "0"},
	"float64zp": {json: "number", format: "double", minimum: "0"},

	"money": dxString,
	"bool":  {json: "boolean"},

	"iso8601": {json: "string", format: "date-time"},
	"date":    {json: "string", format: "date"},
	"time":    {json: "string", format: "time"},

	"json":             {json: "object", children: "properties"},
	"json-passthrough": dxObjects,

	"array":               {json: "array"},
	"array-string":        {json: "array", items: &dxString},
	"array-int64":         {json: "array", items: &dxInt64},
	"array-json-template": {json: "array", children: "items"},

	"map-string-string": {json: "object", mapOfStrings: true},
}

// The parameter types by the name of their constant.
var dxTypeConstants = map[string]string{
	"APIParameterTypeEncryptedBlob": "encrypted-blob", "APIParameterTypeBlob": "blob",
	"APIParameterTypeString": "string", "APIParameterTypeProtectedString": "protected-string",
	"APIParameterTypeProtectedSQLString": "protected-sql-string", "APIParameterTypeProtectedNonEmptyString": "protected-non-empty-string",
	"APIParameterTypeNullableString": "nullable-string", "APIParameterTypeNonEmptyString": "non-empty-string",
	"APIParameterTypeEmail": "email", "APIParameterTypePhoneNumber": "phonenumber", "APIParameterTypeNPWP": "npwp",
	"APIParameterTypeInt32": "int32", "APIParameterTypeInt32P": "int32p", "APIParameterTypeInt32ZP": "int32zp",
	"APIParameterTypeNullableInt32": "nullable-int32", "APIParameterTypeInt64": "int64", "APIParameterTypeInt64P": "int64p",
	"APIParameterTypeInt64ZP": "int64zp", "APIParameterTypeNullableInt64": "nullable-int64", "APIParameterTypeID": "id",
	"APIParameterTypeFloat32": "float32", "APIParameterTypeFloat32P": "float32p", "APIParameterTypeFloat32ZP": "float32zp",
	"APIParameterTypeFloat64": "float64", "APIParameterTypeFloat64P": "float64p", "APIParameterTypeFloat64ZP": "float64zp",
	"APIParameterTypeMoney": "money", "APIParameterTypeBoolean": "bool",
	"APIParameterTypeISO8601": "iso8601", "APIParameterTypeDate": "date", "APIParameterTypeTime": "time",
	"APIParameterTypeJSON": "json", "APIParameterTypeJSONPassthrough": "json-passthrough",
	"APIParameterTypeArray": "array", "APIParameterTypeArrayString": "array-string", "APIParameterTypeArrayInt64": "array-int64",
	"APIParameterTypeArrayJSONTemplate": "array-json-template", "APIParameterTypeMapStringString": "map-string-string",
}

// The content types a request body arrives in, by the name of their
// constant and by their value (utils/http's RequestContentType, an iota
// from none).
var dxContentTypes = map[string]string{
	"RequestContentTypeNone":                          "",
	"RequestContentTypeApplicationOctetStream":        "application/octet-stream",
	"RequestContentTypeTextPlain":                     "text/plain",
	"RequestContentTypeApplicationJSON":               "application/json",
	"RequestContentTypeApplicationXWwwFormUrlEncoded": "application/x-www-form-urlencoded",
	"RequestContentTypeMultiPartFormData":             "multipart/form-data",
}

var dxContentTypeValues = []string{"", "application/octet-stream", "text/plain", "application/json", "application/x-www-form-urlencoded", "multipart/form-data"}

// The methods whose parameters dxlib reads from the body, and those whose
// it reads from the query; PreProcessRequest refuses every other method
// with 422, "Request method is not supported yet".
var (
	dxBodyMethods  = map[string]bool{"POST": true, "PUT": true}
	dxQueryMethods = map[string]bool{"GET": true, "DELETE": true}
)

// The keys of a parameter's declaration, as DXAPIEndPointParameter names
// them, that the reader reads; Owner and Parent are set by dxlib.
var dxParameterKeys = map[string]bool{
	"Owner": true, "Parent": true, "NameId": true, "Type": true, "Description": true, "IsMustExist": true, "IsNullable": true,
	"Children": true, "Enum": true, "Minimum": true, "ExclusiveMinimum": true, "Maximum": true, "ExclusiveMaximum": true,
	"MultipleOf": true, "MinLength": true, "MaxLength": true, "Pattern": true, "MinItems": true, "MaxItems": true,
	"UniqueItems": true, "Const": true, "Title": true, "Default": true, "ReadOnly": true, "WriteOnly": true,
}

// The bounds a declaration gives through a pointer, with the keyword each
// is in JSON Schema.
var dxBounds = []struct{ key, keyword string }{
	{"Minimum", "minimum"}, {"ExclusiveMinimum", "exclusiveMinimum"}, {"Maximum", "maximum"}, {"ExclusiveMaximum", "exclusiveMaximum"},
	{"MultipleOf", "multipleOf"}, {"MinLength", "minLength"}, {"MaxLength", "maxLength"}, {"MinItems", "minItems"}, {"MaxItems", "maxItems"},
}

// dxDecl is one parameter a registration declares: its name on the wire,
// whether it must be sent, and its schema as dxlib's emitter writes it, or
// why the reader could not read it.
type dxDecl struct {
	name     string
	clause   string
	required bool
	schema   *yaml.Node // in OpenAPI's form; nil when why is not ""
	why      string
	unread   []dxUnread // its children that were not read, below its schema
}

// dxUnread is a parameter inside a declaration that the reader could not
// read: where it sits below the schema it is in, as JSON Schema keywords
// and wire names, and why.
type dxUnread struct {
	path     []string
	name     string
	required bool
	clause   string
	why      string
}

// dxResponse is one response a registration declares.
type dxResponse struct {
	name        string
	clause      string
	description string
	headers     []string
	content     *yaml.Node // the object schema of its body, in OpenAPI's form; nil when it has none
	unread      []dxUnread
}

// dxDeclared is what one registration declares.
type dxDeclared struct {
	params      []*dxDecl
	paramsWhy   string // why the list is not read, "" when it is
	paramsText  string
	contentType string
	contentOK   bool
	contentText string
	responses   map[string]*dxResponse // by status code
	respWhy     string
	respText    string
	conflicts   []string // two responses on one status, as dxlib's emitter refuses them
}

// declare reads what the registration of ep declares.
func (g *goReader) declare(ep *goEndpoint, f *goFile, call *ast.CallExpr) {
	d := &dxDeclared{responses: map[string]*dxResponse{}}
	ep.decl = d
	d.paramsText = exprText(call.Args[6])
	if list, file, why := g.literalList(f, call.Args[6], 0); why != "" {
		d.paramsWhy = why
	} else {
		for _, el := range list {
			d.params = append(d.params, g.declaredParameter(file, el, 0))
		}
	}
	if d.paramsWhy == "" {
		ep.declared = map[string]bool{}
		for _, p := range d.params {
			if p.name == "" {
				ep.declared = nil
				break
			}
			ep.declared[p.name] = true
		}
	}
	d.contentText = exprText(call.Args[5])
	d.contentType, d.contentOK = g.contentType(f, call.Args[5])
	d.respText = exprText(call.Args[9])
	cl, file, why := g.literalComposite(f, call.Args[9], 0)
	switch {
	case why != "":
		d.respWhy = "it " + why
	case cl != nil:
		for _, el := range cl.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				d.respWhy = "an entry of it is not a name and a response"
				break
			}
			name, _ := g.stringValue(file, kv.Key)
			r, code, why := g.declaredResponse(file, kv.Value)
			if why != "" {
				d.respWhy = fmt.Sprintf("the response %s %s", nonEmptyText(name, exprText(kv.Key)), why)
				break
			}
			r.name = name
			if prior := d.responses[code]; prior != nil {
				d.conflicts = append(d.conflicts, fmt.Sprintf("%s and %s on %s", prior.name, name, code))
				continue
			}
			d.responses[code] = r
		}
	}
}

func nonEmptyText(s, otherwise string) string {
	if s == "" {
		return otherwise
	}
	return s
}

// literalList is the elements of a list: a literal, nil, or one a name
// assigned once holds or a function of the module returns as its one
// statement, followed as far as eight steps; why says what stopped it.
func (g *goReader) literalList(f *goFile, e ast.Expr, depth int) ([]ast.Expr, *goFile, string) {
	cl, file, why := g.literalComposite(f, e, depth)
	if cl == nil {
		return nil, file, why
	}
	return cl.Elts, file, ""
}

// literalComposite is the composite literal an expression gives, by the
// same steps as literalList; nil with no why for nil.
func (g *goReader) literalComposite(f *goFile, e ast.Expr, depth int) (*ast.CompositeLit, *goFile, string) {
	const shapes = "is not a literal, a name assigned one literal once, or a call to a function of the module that returns one"
	if depth > 8 {
		return nil, f, "is followed further than eight steps"
	}
	switch x := e.(type) {
	case *ast.Ident:
		if x.Name == "nil" {
			return nil, f, ""
		}
	case *ast.CompositeLit:
		return x, f, ""
	case *ast.ParenExpr:
		return g.literalComposite(f, x.X, depth+1)
	case *ast.UnaryExpr:
		if x.Op == gotoken.AND {
			return g.literalComposite(f, x.X, depth+1)
		}
		return nil, f, shapes
	case *ast.CallExpr:
		fd, file := g.funcOf(f, x.Fun)
		if fd == nil || fd.Body == nil {
			return nil, f, shapes
		}
		if len(fd.Body.List) != 1 {
			return nil, f, fmt.Sprintf("is a call to %s, which does more than return a literal", fd.Name.Name)
		}
		ret, ok := fd.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			return nil, f, fmt.Sprintf("is a call to %s, which does more than return a literal", fd.Name.Name)
		}
		if file != nil {
			file.cited = true
		}
		return g.literalComposite(file, ret.Results[0], depth+1)
	}
	if a, ok := g.valueOf(f, e); ok {
		return g.literalComposite(a.file, a.value, depth+1)
	}
	return nil, f, shapes
}

// contentType is the media type a content type argument names, "" for
// none.
func (g *goReader) contentType(f *goFile, e ast.Expr) (string, bool) {
	switch x := e.(type) {
	case *ast.SelectorExpr:
		mt, ok := dxContentTypes[x.Sel.Name]
		return mt, ok
	case *ast.Ident:
		mt, ok := dxContentTypes[x.Name]
		return mt, ok
	case *ast.BasicLit:
		if x.Kind == gotoken.INT {
			n, err := strconv.Atoi(x.Value)
			if err == nil && n >= 0 && n < len(dxContentTypeValues) {
				return dxContentTypeValues[n], true
			}
		}
	}
	return "", false
}

// declaredParameter reads one parameter's declaration: a composite literal
// with NameId and Type, or a pointer to one.
func (g *goReader) declaredParameter(f *goFile, e ast.Expr, depth int) *dxDecl {
	d := &dxDecl{clause: g.clause(f, e.Pos())}
	cl, file, why := g.literalComposite(f, e, 0)
	if cl == nil {
		d.why = "it " + nonEmptyText(why, "is nil")
		return d
	}
	f = file
	d.clause = g.clause(f, cl.Pos())
	fields := map[string]ast.Expr{}
	for _, el := range cl.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		k, isName := kvKey(kv)
		if !ok || !isName {
			d.why = "it does not name its fields"
			return d
		}
		if !dxParameterKeys[k] {
			d.why = "it has the field " + k + ", which a parameter's declaration does not"
			return d
		}
		fields[k] = kv.Value
	}
	name, ok := g.stringValue(f, fields["NameId"])
	if !ok || name == "" {
		d.why = "its NameId is not a literal"
		if fields["NameId"] == nil {
			d.why = "it gives no NameId"
		}
		return d
	}
	d.name = name
	d.required, ok = boolValue(fields["IsMustExist"])
	if !ok {
		d.why = "its IsMustExist is not true or false"
		return d
	}
	if fields["Type"] == nil {
		d.why = "it gives no Type"
		return d
	}
	tn, ok := g.parameterType(f, fields["Type"])
	if !ok {
		d.why = "its Type " + exprText(fields["Type"]) + " is not one of dxlib's parameter types"
		return d
	}
	t := dxTypes[tn]
	s := mapping()
	typ := any(t.json)
	nullable, ok := boolValue(fields["IsNullable"])
	if !ok {
		d.why = "its IsNullable is not true or false"
		return d
	}
	if nullable {
		typ = []string{t.json, "null"}
	}
	set(s, "type", typ)
	set(s, "format", nonEmpty(t.format))
	for _, k := range []string{"Title", "Description", "Pattern"} {
		if fields[k] == nil {
			continue
		}
		v, ok := g.stringValue(f, fields[k])
		if !ok {
			d.why = "its " + k + " is not a literal"
			return d
		}
		set(s, strings.ToLower(k[:1])+k[1:], nonEmpty(v))
	}
	bounds := map[string]*yaml.Node{}
	if t.minimum != "" {
		bounds["minimum"] = literal("!!int", t.minimum)
	}
	if t.exclusiveMinimum != "" {
		bounds["exclusiveMinimum"] = literal("!!int", t.exclusiveMinimum)
	}
	if t.minLength != "" {
		bounds["minLength"] = literal("!!int", t.minLength)
	}
	for _, b := range dxBounds {
		if fields[b.key] == nil {
			continue
		}
		v, ok := pointerLiteral(fields[b.key])
		if !ok || v.Tag == "!!str" {
			d.why = "its " + b.key + " is not a number the reader reads as a literal"
			return d
		}
		bounds[b.keyword] = v
	}
	for _, b := range dxBounds {
		if v := bounds[b.keyword]; v != nil {
			set(s, b.keyword, v)
		}
	}
	if fields["Enum"] != nil {
		values, ok := literalValues(fields["Enum"])
		if !ok {
			d.why = "its Enum is not a list of literals"
			return d
		}
		set(s, "enum", values)
	}
	for _, k := range []string{"Const", "Default"} {
		if fields[k] == nil {
			continue
		}
		v, ok := pointerLiteral(fields[k])
		if !ok {
			d.why = "its " + k + " is not a value the reader reads as a literal"
			return d
		}
		set(s, strings.ToLower(k), v)
	}
	for _, k := range []string{"UniqueItems", "ReadOnly", "WriteOnly"} {
		v, ok := boolValue(fields[k])
		if !ok {
			d.why = "its " + k + " is not true or false"
			return d
		}
		if v {
			set(s, strings.ToLower(k[:1])+k[1:], true)
		}
	}
	switch {
	case t.items != nil:
		set(s, "items", flow(mapping("type", t.items.json, "format", nonEmpty(t.items.format))))
	case t.mapOfStrings:
		set(s, "additionalProperties", flow(mapping("type", "string")))
	}
	if fields["Children"] != nil && t.children != "" {
		list, cf, why := g.literalList(f, fields["Children"], 0)
		if why != "" {
			d.why = "its Children " + why
			return d
		}
		obj := mapping("type", "object")
		prefix := []string(nil)
		if t.children == "items" {
			prefix = []string{"items"}
		}
		g.objectOf(cf, list, obj, prefix, &d.unread, depth+1)
		if t.children == "items" {
			set(s, "items", obj)
		} else {
			s.Content = append(s.Content, obj.Content[2:]...)
		}
	}
	d.schema = s
	return d
}

// objectOf writes the parameters of a list into an object schema in
// OpenAPI's form, under prefix below the schema the unread are recorded
// against.
func (g *goReader) objectOf(f *goFile, list []ast.Expr, obj *yaml.Node, prefix []string, unread *[]dxUnread, depth int) {
	var props *yaml.Node
	var required []string
	for _, el := range list {
		p := g.declaredParameter(f, el, depth)
		if p.why != "" {
			*unread = append(*unread, dxUnread{path: append(append([]string{}, prefix...), "properties", p.name), name: p.name, required: p.required, clause: p.clause, why: p.why})
			continue
		}
		recordUnread(p, append(append([]string{}, prefix...), "properties", p.name), unread)
		if props == nil {
			props = mapping()
		}
		set(props, p.name, p.schema)
		if p.required {
			required = append(required, p.name)
		}
	}
	if props != nil {
		set(obj, "properties", props)
	}
	if len(required) > 0 {
		set(obj, "required", required)
	}
}

// recordUnread moves the unread children of a parameter under the path
// the parameter has.
func recordUnread(p *dxDecl, at []string, unread *[]dxUnread) {
	for _, u := range p.unread {
		u.path = append(append([]string{}, at...), u.path...)
		*unread = append(*unread, u)
	}
}

// declaredResponse reads one response possibility: its status, its
// description, its headers and the template of its body.
func (g *goReader) declaredResponse(f *goFile, e ast.Expr) (*dxResponse, string, string) {
	cl, file, why := g.literalComposite(f, e, 0)
	if cl == nil {
		return nil, "", nonEmptyText(why, "is nil")
	}
	f = file
	r := &dxResponse{clause: g.clause(f, cl.Pos())}
	code := ""
	for _, el := range cl.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		k, isName := kvKey(kv)
		if !ok || !isName {
			return nil, "", "does not name its fields"
		}
		switch k {
		case "Owner":
		case "StatusCode":
			n, ok := g.statusValue(f, kv.Value)
			if !ok {
				if sel, isSel := kv.Value.(*ast.SelectorExpr); isSel {
					n, ok = httpStatusCodes[sel.Sel.Name]
				}
			}
			if !ok || n < 100 || n > 599 {
				return nil, "", "has a StatusCode " + exprText(kv.Value) + " that is not a literal status"
			}
			code = strconv.Itoa(n)
		case "Description":
			s, ok := g.stringValue(f, kv.Value)
			if !ok {
				return nil, "", "has a Description that is not a literal"
			}
			r.description = s
		case "Headers":
			hc, _, why := g.literalComposite(f, kv.Value, 0)
			if why != "" {
				return nil, "", "has Headers that " + why
			}
			if hc != nil {
				for _, h := range hc.Elts {
					if kv, ok := h.(*ast.KeyValueExpr); ok {
						name, _ := g.stringValue(f, kv.Key)
						r.headers = append(r.headers, nonEmptyText(name, exprText(kv.Key)))
					}
				}
			}
		case "DataTemplate":
			list, lf, why := g.literalList(f, kv.Value, 0)
			if why != "" {
				return nil, "", "has a DataTemplate that " + why
			}
			if len(list) > 0 {
				obj := mapping("type", "object")
				g.objectOf(lf, list, obj, nil, &r.unread, 1)
				r.content = obj
			}
		default:
			return nil, "", "has the field " + k + ", which a response possibility does not"
		}
	}
	if code == "" {
		return nil, "", "gives no StatusCode"
	}
	return r, code, ""
}

// The success and redirect statuses net/http names, beside the refusals.
var httpStatusCodes = map[string]int{
	"StatusContinue": 100, "StatusSwitchingProtocols": 101, "StatusOK": 200, "StatusCreated": 201, "StatusAccepted": 202,
	"StatusNonAuthoritativeInfo": 203, "StatusNoContent": 204, "StatusResetContent": 205, "StatusPartialContent": 206,
	"StatusMultipleChoices": 300, "StatusMovedPermanently": 301, "StatusFound": 302, "StatusSeeOther": 303,
	"StatusNotModified": 304, "StatusTemporaryRedirect": 307, "StatusPermanentRedirect": 308,
}

func kvKey(kv *ast.KeyValueExpr) (string, bool) {
	if kv == nil {
		return "", false
	}
	id, ok := kv.Key.(*ast.Ident)
	if !ok {
		return "", false
	}
	return id.Name, true
}

// parameterType is the value of a parameter type: a constant of dxlib's by
// its name, whatever package qualifies it, or a string literal.
func (g *goReader) parameterType(f *goFile, e ast.Expr) (string, bool) {
	switch x := e.(type) {
	case *ast.SelectorExpr:
		v, ok := dxTypeConstants[x.Sel.Name]
		return v, ok
	case *ast.Ident:
		v, ok := dxTypeConstants[x.Name]
		return v, ok
	case *ast.CallExpr:
		// A conversion such as types.APIParameterType("int64").
		if len(x.Args) == 1 {
			return g.parameterType(f, x.Args[0])
		}
	}
	if s, ok := g.stringValue(f, e); ok {
		_, known := dxTypes[s]
		return s, known
	}
	return "", false
}

// boolValue is true or false as written, false when nothing is.
func boolValue(e ast.Expr) (bool, bool) {
	if e == nil {
		return false, true
	}
	if id, ok := e.(*ast.Ident); ok && (id.Name == "true" || id.Name == "false") {
		return id.Name == "true", true
	}
	return false, false
}

// scalarLiteral is a basic literal, or a negative number, as a node.
func scalarLiteral(e ast.Expr) (*yaml.Node, bool) {
	switch x := e.(type) {
	case *ast.BasicLit:
		switch x.Kind {
		case gotoken.INT:
			return literal("!!int", x.Value), true
		case gotoken.FLOAT:
			return literal("!!float", x.Value), true
		case gotoken.STRING:
			s, err := strconv.Unquote(x.Value)
			return str(s), err == nil
		}
	case *ast.UnaryExpr:
		if x.Op == gotoken.SUB {
			if n, ok := scalarLiteral(x.X); ok && n.Tag != "!!str" {
				n.Value = "-" + n.Value
				return n, true
			}
		}
	case *ast.Ident:
		if x.Name == "true" || x.Name == "false" {
			return literal("!!bool", x.Name), true
		}
	}
	return nil, false
}

// pointerLiteral is the value a pointer field is given: a helper called
// with one literal, such as a generic Ptr(1.0), or a conversion of one.
func pointerLiteral(e ast.Expr) (*yaml.Node, bool) {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	if inner, ok := call.Args[0].(*ast.CallExpr); ok {
		return pointerLiteral(inner)
	}
	return scalarLiteral(call.Args[0])
}

// literalValues is a list of literals, as an enum gives them.
func literalValues(e ast.Expr) (*yaml.Node, bool) {
	cl, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	out := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, el := range cl.Elts {
		v, ok := scalarLiteral(el)
		if !ok {
			return nil, false
		}
		out.Content = append(out.Content, v)
	}
	return out, true
}

// declaredSnake says whether the names the registrations declare are
// snake_case, as snakeWire says it of a document: one name at least joins
// words with an underscore, and none has a capital.
func (g *goReader) declaredSnake() bool {
	underscore, capital := false, false
	note := func(n string) {
		underscore = underscore || strings.Contains(n, "_")
		capital = capital || strings.ToLower(n) != n
	}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if p := child(n, "properties"); p != nil {
			for _, k := range keys(p) {
				note(k)
				walk(child(p, k))
			}
		}
		if i := child(n, "items"); i != nil {
			walk(i)
		}
	}
	for _, ep := range g.endpoints {
		g.handlerWires(ep, note)
		if ep.decl == nil {
			continue
		}
		for _, p := range ep.decl.params {
			if p.name != "" {
				note(p.name)
			}
			walk(p.schema)
		}
		for _, r := range ep.decl.responses {
			walk(r.content)
			for _, u := range r.unread {
				note(u.name)
			}
		}
	}
	return underscore && !capital
}

// declaredWriter writes the declarations in the meta-model's field subset
// with the OpenAPI reader's own, so that the operation reads as extract
// openapi reads the document dxlib emits, and merging the two agrees.
type declaredWriter struct {
	g *goReader
	o *openapiReader
}

func (g *goReader) declaredWriter() *declaredWriter {
	o := &openapiReader{doc: mapping(), key: g.key, res: g.res, questions: g.questions, nextID: g.nextID,
		snake: g.declaredSnake(), written: map[string]string{}}
	if o.snake {
		g.wireSnake = true
	}
	return &declaredWriter{g: g, o: o}
}

// member is the name a wire name is written by, or false.
func (w *declaredWriter) member(wire string) (string, bool) {
	if !w.o.snake {
		return wire, memberNameWord.MatchString(wire)
	}
	name, ok := wirename.Camel(wire)
	return name, ok && memberNameWord.MatchString(name)
}

// request writes the parameters and the body of an operation from its
// declaration, and asks for what it does not say.
func (w *declaredWriter) request(ep *goEndpoint, op *yaml.Node, regAt, label string) {
	g, d := w.g, ep.decl
	if !dxBodyMethods[ep.method] && !dxQueryMethods[ep.method] {
		g.question("must", fmt.Sprintf("%s registers %s, and dxlib reads parameters for GET and DELETE from the query and for POST and PUT from the body, and refuses every other method with 422 (\"Request method is not supported yet\") before the handler runs. Does it answer anything but 422, and which parameters does it take, with their types?", regAt, label),
			[]string{ep.at}, "dxlib's PreProcessRequest reads parameters for four methods only, so the parameters declared here are not written where the running system would not read them.", g.at(regAt, "Registers "+label+" under "+ep.method+"."))
		return
	}
	if d.paramsWhy != "" {
		where := ep.at + "/parameters"
		if dxBodyMethods[ep.method] {
			where = ep.at + "/requestBody"
		}
		g.question("must", fmt.Sprintf("%s registers %s with the parameters %s, which %s, so the parameters it takes are not read. Which parameters does it take, with their types?", regAt, label, d.paramsText, d.paramsWhy),
			[]string{where}, "The parameters an endpoint declares are what dxlib reads and checks before the handler runs, and syntax reads only a declaration it can follow to a literal.", g.at(regAt, "Declares its parameters with "+d.paramsText+"."))
		return
	}
	if !dxBodyMethods[ep.method] {
		var list []*yaml.Node
		for _, p := range d.params {
			at := fmt.Sprintf("%s/parameters/%d", ep.at, len(list))
			if p.name == "" {
				g.question("must", fmt.Sprintf("%s: a parameter %s declares cannot be read: %s. Which parameter is it, with its type?", p.clause, label, p.why),
					[]string{ep.at + "/parameters"}, "A parameter is read only from a declaration that names it with a literal.", g.at(p.clause, "Declares a parameter the reader cannot read."))
				continue
			}
			if p.why != "" {
				pn := mapping("name", p.name, "in", "query")
				if p.required {
					set(pn, "required", true)
				}
				list = append(list, flow(pn))
				g.question("must", fmt.Sprintf("%s: the parameter %s of %s is declared, and %s. Which values does it take?", p.clause, p.name, label, p.why),
					[]string{at + "/schema"}, "A parameter's type is read from its declaration, and this one is not read.", g.at(p.clause, "Declares the parameter "+p.name+"."))
				continue
			}
			pn := mapping("name", p.name, "in", "query", "schema", p.schema)
			if p.required {
				set(pn, "required", true)
			}
			if desc := child(p.schema, "description"); desc != nil {
				set(pn, "description", desc.Value)
			}
			param := w.o.parameter(pn, "operation "+label, at, nil)
			if param == nil {
				continue
			}
			list = append(list, param)
			w.askUnread(p.unread, label, at+"/schema", child(param, "schema"))
		}
		if len(list) > 0 {
			set(op, "parameters", list)
		}
		return
	}
	switch {
	case !d.contentOK:
		g.question("must", fmt.Sprintf("%s registers %s with the content type %s, which is not one of dxlib's. Which media type does its body take?", regAt, label, d.contentText),
			[]string{ep.at + "/requestBody"}, "dxlib reads a body by the content type an endpoint declares, and only a constant of dxlib's names one.", g.at(regAt, "Declares the content type "+d.contentText+"."))
		return
	case d.contentType == "", d.contentType == "application/octet-stream", d.contentType == "text/plain":
		if d.contentType != "" {
			body := mapping("type", "string")
			if d.contentType == "application/octet-stream" {
				set(body, "format", "binary")
			}
			set(op, "requestBody", mapping("content", mapping(d.contentType, mapping("schema", flow(body)))))
		}
		if s := child(op, "stream"); ep.stream == "upload" && s != nil && len(d.params) > 0 {
			// dxlib reads an upload stream's parameters from X-Var, which
			// stream.parameters holds (ADR-162).
			at := ep.at + "/stream/parameters"
			obj, unread := w.parametersObject(ep, label, at)
			schema := w.o.field(obj, "operation "+label+", upload parameters", at, 0)
			if schema == nil {
				return
			}
			s.Style = 0
			set(s, "parameters", schema)
			w.askUnread(unread, label, at, schema)
			return
		}
		if len(d.params) > 0 {
			var names []string
			for _, p := range d.params {
				names = append(names, nonEmptyText(p.name, "one the reader cannot read"))
			}
			ct := "no content type"
			if d.contentType != "" {
				ct = "the content type " + d.contentType
			}
			g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: the endpoint %s declares %s and the %s %s, which dxlib reads from the X-Var header as JSON; the meta-model holds no parameter carried in a header as JSON; left out", regAt, label, ct, parameterOrParameters(len(names)), joinAnd(names)), clause: regAt, blocks: []string{ep.at}})
		}
		return
	}
	at := ep.at + "/requestBody"
	schemaAt := at + "/content/" + escapeToken(d.contentType) + "/schema"
	decode := (*handlerDecode)(nil)
	if ep.shapes != nil && d.contentType == "application/json" {
		decode = ep.shapes.decode
	}
	if decode != nil && ep.shapes.decodes > 1 {
		g.question("should", fmt.Sprintf("%s: the handler of %s decodes its body %d times; the reader writes the first, into %s. Do the others read the same shape?", decode.clause, label, ep.shapes.decodes, decode.text),
			[]string{at}, "A request body has one shape, and a second decoding may read another.", g.at(decode.clause, "Decodes the body into "+decode.text+"."))
	}
	decodeWhy := ""
	if decode != nil && len(d.params) == 0 {
		schema, why := w.handlerBodySchema(ep, label, schemaAt)
		if schema != nil {
			set(op, "requestBody", mapping("content", mapping(d.contentType, mapping("schema", schema))))
			return
		}
		decodeWhy = why
	}
	if decode != nil && decode.typ != nil && decode.from == "RequestBodyAsBytes" && len(d.params) > 0 {
		var declared []string
		for _, p := range d.params {
			if p.name != "" {
				declared = append(declared, p.name)
			}
		}
		if wires, ok := g.topWires(decode.file, decode.typ); ok && strings.Join(sortedCopy(wires), ",") != strings.Join(sortedCopy(declared), ",") {
			g.question("must", fmt.Sprintf("Two sources disagree on the body of %s: %s declares %s, and its handler decodes it at %s into %s, of type %s, with %s. Which fields does the body take?", label, regAt, fieldsText(sortedCopy(declared)), decode.clause, decode.text, typeText(decode.typ), fieldsText(sortedCopy(wires))),
				[]string{schemaAt}, "The registration declares the parameters dxlib reads and checks, and is written; the handler decodes the body itself, and the two name other fields.", g.at(decode.clause, "Decodes the body into "+decode.text+"."))
		}
	}
	obj, unread := w.parametersObject(ep, label, ep.at+"/requestBody")
	rb := w.o.requestBody(mapping("content", mapping(d.contentType, mapping("schema", obj))), label, at)
	if rb == nil {
		return
	}
	set(op, "requestBody", rb)
	schema := child(child(child(rb, "content"), d.contentType), "schema")
	w.askUnread(unread, label, schemaAt, schema)
	switch {
	case len(d.params) > 0, decode != nil && decodeWhy == "":
		// A struct the reader could not write is asked about where its
		// type is.
	case decodeWhy != "":
		g.question("must", fmt.Sprintf("%s declares no parameter where %s registers it, and %s, so its body is written as any object. Which fields does its body take, with their types?", label, regAt, decodeWhy),
			[]string{schemaAt}, "dxlib reads and checks only the parameters an endpoint declares, and the handler's own decoding gives no fields the reader can write: it is a value of no declared type, a map that names no field, a body dxlib refuses or one the decode never sees.", g.at(decode.clause, "Decodes the body into "+decode.text+"."))
	default:
		g.question("must", fmt.Sprintf("%s declares no parameter where %s registers it, and its handler reads none by name (%s), so its body is written as any object. Which fields does its body take, with their types?", label, regAt, nonEmptyText(ep.handlerAt, "its handler is not found")),
			[]string{schemaAt}, "dxlib reads and checks only the parameters an endpoint declares; a handler that reads its body in another way, such as decoding it itself, gives its fields only in code syntax does not follow.", g.at(regAt, "Declares no parameter."))
	}
}

// parametersObject is the object schema of the parameters a registration
// declares, one property each, with those it could not read to be asked
// about at their place; one it cannot even name is asked about at at.
func (w *declaredWriter) parametersObject(ep *goEndpoint, label, at string) (*yaml.Node, []dxUnread) {
	g, d := w.g, ep.decl
	obj := mapping("type", "object")
	props := mapping()
	var required []string
	var unread []dxUnread
	for _, p := range d.params {
		if p.name == "" {
			g.question("must", fmt.Sprintf("%s: a parameter %s declares cannot be read: %s. Which parameter is it, with its type?", p.clause, label, p.why),
				[]string{at}, "A parameter is read only from a declaration that names it with a literal.", g.at(p.clause, "Declares a parameter the reader cannot read."))
			continue
		}
		if p.why != "" {
			unread = append(unread, dxUnread{path: []string{"properties", p.name}, name: p.name, required: p.required, clause: p.clause, why: p.why})
			continue
		}
		recordUnread(p, []string{"properties", p.name}, &unread)
		set(props, p.name, p.schema)
		if p.required {
			required = append(required, p.name)
		}
	}
	if len(props.Content) > 0 {
		set(obj, "properties", props)
	}
	if len(required) > 0 {
		set(obj, "required", required)
	}
	return obj, unread
}

// askUnread writes each parameter inside a declaration that was not read
// into the schema at its place, as a field known by name only, and asks
// for it.
func (w *declaredWriter) askUnread(unread []dxUnread, label, at string, schema *yaml.Node) {
	g := w.g
	for _, u := range unread {
		ptr, node, ok := at, schema, schema != nil
		var parent *yaml.Node
		for i := 0; ok && i < len(u.path); i++ {
			switch u.path[i] {
			case "items":
				ptr += "/items"
				node = child(node, "items")
				ok = node != nil
			case "properties":
				i++
				name, held := w.member(u.path[i])
				if !held {
					ok = false
					break
				}
				ptr += "/properties/" + name
				parent = node
				props := child(node, "properties")
				if props == nil {
					props = mapping()
					set(node, "properties", props)
					node.Style = 0
				}
				next := child(props, name)
				if next == nil {
					next = mapping()
					set(props, name, next)
					if u.required {
						req := child(parent, "required")
						if req == nil {
							set(parent, "required", []string{name})
						} else {
							req.Content = append(req.Content, str(name))
						}
					}
				}
				node = next
			}
		}
		where := strings.Join(wirePath(u.path), ".")
		if !ok {
			g.question("must", fmt.Sprintf("%s: the parameter %s of %s is declared, and %s. Which values does it take?", u.clause, where, label, u.why),
				[]string{at}, "A parameter's type is read from its declaration, and this one is not read.", g.at(u.clause, "Declares the parameter "+where+"."))
			continue
		}
		g.question("must", fmt.Sprintf("%s: the parameter %s of %s is declared, and %s. Which values does it take?", u.clause, where, label, u.why),
			[]string{ptr}, "A parameter's type is read from its declaration, and this one is not read.", g.at(u.clause, "Declares the parameter "+where+"."))
	}
}

// wirePath is the wire names along a path of JSON Schema keywords.
func wirePath(path []string) []string {
	var out []string
	for i := 0; i < len(path); i++ {
		if path[i] == "properties" && i+1 < len(path) {
			i++
			out = append(out, path[i])
		}
	}
	return out
}

// responses writes the responses an endpoint declares into responses, by
// status code, and says which codes they are.
func (w *declaredWriter) responses(ep *goEndpoint, responses *yaml.Node, label, regAt string) map[string]bool {
	g, d := w.g, ep.decl
	declared := map[string]bool{}
	if d.respWhy != "" {
		g.question("must", fmt.Sprintf("%s registers %s with the response possibilities %s, and %s, so the responses it declares are not read. Which responses does it answer, and what does each carry?", regAt, label, d.respText, d.respWhy),
			[]string{ep.at + "/responses"}, "The responses an endpoint declares give its success and its body, and syntax reads only a declaration it can follow to a literal.", g.at(regAt, "Declares its responses with "+d.respText+"."))
		return declared
	}
	for _, c := range d.conflicts {
		g.question("must", fmt.Sprintf("%s registers %s with two responses on one status, %s; dxlib's emitter refuses that. Which one does it answer?", regAt, label, c),
			[]string{ep.at + "/responses"}, "A status code is answered with one response, and two named for it say different things.", g.at(regAt, "Declares two responses on one status."))
	}
	var codes []string
	for c := range d.responses {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	for _, c := range codes {
		r := d.responses[c]
		at := ep.at + "/responses/" + c
		where := "operation " + label + ", response " + c
		resp := mapping()
		set(resp, "description", nonEmpty(r.description))
		switch {
		case r.content != nil && ep.streamsOut(c):
			// A download stream answers the file's bytes, never the
			// template, so its widths are not asked (ADR-146).
			ep.templated[c] = true
		case r.content != nil:
			content := w.o.content(mapping("application/json", mapping("schema", r.content)), where, at+"/content")
			if content != nil {
				set(resp, "content", content)
				w.askUnread(r.unread, label, at+"/content/application~1json/schema", child(child(content, "application/json"), "schema"))
			}
		}
		if len(r.headers) > 0 {
			sort.Strings(r.headers)
			g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: the response %s of %s declares the %s %s; the meta-model holds no header on a response; left out", r.clause, c, label, pluralWord(len(r.headers), "header"), joinAnd(r.headers)), clause: r.clause, blocks: []string{at}})
		}
		set(responses, c, resp)
		declared[c] = true
	}
	return declared
}

// finish moves what the OpenAPI reader found into the Go reader: the
// widths it asks in one question, and what the meta-model cannot hold,
// each citing the registration it was declared at.
func (w *declaredWriter) finish(regAt map[string]string) {
	// The Go reader asks its own questions as it writes, so the OpenAPI
	// reader numbers on from them.
	w.o.nextID = w.g.nextID
	w.o.askWidths()
	w.g.nextID = w.o.nextID
	for _, h := range w.o.notHeld {
		op := operationOf(h.clause)
		clause := regAt[op]
		if clause == "" {
			clause = h.clause
		}
		w.g.notHeld = append(w.g.notHeld, notHeld{text: clause + ": " + h.text, clause: clause, blocks: []string{op}, asked: h.asked})
	}
}

// operationOf is the operation a pointer into the tree is in.
func operationOf(ptr string) string {
	tokens := strings.Split(ptr, "/")
	if len(tokens) >= 4 {
		return strings.Join(tokens[:4], "/")
	}
	return ptr
}
