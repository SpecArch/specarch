package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/spec"
)

// The version of the TypeScript compiler whose dumps extract javascript
// reads, as readers/javascript/package-lock.json pins it; a dump another
// version made is refused, since another compiler may give other facts
// (ADR-089).
const typeScriptVersion = "6.0.3"

// The version of @vue/compiler-sfc whose split of a .vue file the
// JavaScript reader's dumps hold, as its package-lock.json pins it
// (ADR-091).
const vueCompilerVersion = "3.5.43"

// jsDump is a code-facts dump of JavaScript and TypeScript, as
// tools/code-facts/dump-javascript.sh writes it.
type jsDump struct {
	CodeFacts int    `json:"codeFacts"`
	Language  string `json:"language"`
	Parser    struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"parser"`
	TemplateParser *struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"templateParser"`
	Path   string `json:"path"`
	Commit string `json:"commit"`
	Files  []struct {
		File         string `json:"file"`
		SyntaxErrors int    `json:"syntaxErrors"`
	} `json:"files"`
	Facts []*jsFact `json:"facts"`
}

// jsPos is where a declaration or a function is: a file, a line and a
// column.
type jsPos struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

func (p *jsPos) key() string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Column)
}

func (p *jsPos) clause() string { return fmt.Sprintf("%s:%d", p.File, p.Line) }

// jsFact is one fact the JavaScript reader wrote. Which keys it has
// depends on its kind: configuration, import, type, function, variable,
// call, access, jsx, export or dynamic.
type jsFact struct {
	Kind   string `json:"kind"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`

	// configuration
	CheckJs     bool   `json:"checkJs"`
	AllowJs     bool   `json:"allowJs"`
	Strict      bool   `json:"strict"`
	ExtendsRead *bool  `json:"extendsRead"`
	Error       string `json:"error"`

	// import
	Module   string   `json:"module"`
	Names    []jsName `json:"names"`
	TypeOnly bool     `json:"typeOnly"`
	Dynamic  bool     `json:"dynamic"`
	Require  bool     `json:"require"`
	Resolved string   `json:"resolved"`

	// type, function and variable
	Declaration  string        `json:"declaration"`
	Name         string        `json:"name"`
	Exported     string        `json:"exported"`
	Extends      []string      `json:"extends"`
	Members      []jsMember    `json:"members"`
	Decorators   []jsDecorator `json:"decorators"`
	Type         *jsType       `json:"type"`
	TypeFrom     string        `json:"typeFrom"`
	Values       []jsEnumValue `json:"values"`
	Comment      string        `json:"comment"`
	Parameters   []jsParam     `json:"parameters"`
	Returns      *jsType       `json:"returns"`
	Jsdoc        string        `json:"jsdoc"`
	Directives   []string      `json:"directives"`
	ReturnValues []jsValue     `json:"returnValues"`
	Pattern      string        `json:"pattern"`
	Value        *jsValue      `json:"value"`

	// call
	Callee        *jsValue  `json:"callee"`
	Arguments     []jsValue `json:"arguments"`
	TypeArguments []*jsType `json:"typeArguments"`
	AssignedTo    string    `json:"assignedTo"`

	// access
	Chain      []string    `json:"chain"`
	Parameter  *jsParamRef `json:"parameter"`
	Default    *jsValue    `json:"default"`
	Compared   *jsValue    `json:"compared"`
	WrappedBy  string      `json:"wrappedBy"`
	Cases      []jsValue   `json:"cases"`
	HasDefault bool        `json:"hasDefault"`

	// jsx
	Tag        *jsValue `json:"tag"`
	Attributes []jsAttr `json:"attributes"`
	Children   []jsPart `json:"children"`
	Parent     *jsPos   `json:"parent"`
	ParentTag  *jsValue `json:"parentTag"`

	// dynamic
	What string `json:"what"`

	// template: an element of a Vue template
	Element       string `json:"element"`
	ParentElement string `json:"parentElement"`

	// where it is
	Within      *jsPos    `json:"within"`
	InLoop      bool      `json:"inLoop"`
	InCondition bool      `json:"inCondition"`
	Branch      *jsBranch `json:"branch"`
}

// jsBranch is the branch of a switch's case, or of an if comparing a
// value with ===, that a fact is in: what is compared, and with which value.
type jsBranch struct {
	On    string  `json:"on"`
	Value jsValue `json:"value"`
}

type jsName struct {
	Imported string   `json:"imported"`
	Local    string   `json:"local"`
	Property string   `json:"property"`
	Default  *jsValue `json:"default"`
	Rest     bool     `json:"rest"`
}

type jsMember struct {
	Name       string        `json:"name"`
	Line       int           `json:"line"`
	Column     int           `json:"column"`
	Type       *jsType       `json:"type"`
	TypeFrom   string        `json:"typeFrom"`
	Optional   bool          `json:"optional"`
	Readonly   bool          `json:"readonly"`
	Static     bool          `json:"static"`
	Decorators []jsDecorator `json:"decorators"`
}

type jsDecorator struct {
	Name      string    `json:"name"`
	Arguments []jsValue `json:"arguments"`
}

type jsEnumValue struct {
	Name  string   `json:"name"`
	Value *jsValue `json:"value"`
}

type jsParam struct {
	Name     string  `json:"name"`
	Pattern  string  `json:"pattern"`
	Type     *jsType `json:"type"`
	TypeFrom string  `json:"typeFrom"`
	Optional bool    `json:"optional"`
}

type jsParamRef struct {
	Index    int    `json:"index"`
	Function *jsPos `json:"function"`
}

type jsImport struct {
	Module   string `json:"module"`
	Imported string `json:"imported"`
}

// jsValue is an expression as the JavaScript reader writes it: one of
// its keys, with what a name resolves to.
type jsValue struct {
	String      *string     `json:"string"`
	Template    []jsPart    `json:"template"`
	Number      *string     `json:"number"`
	Boolean     *bool       `json:"boolean"`
	Null        *bool       `json:"null"`
	Regex       *string     `json:"regex"`
	Name        *string     `json:"name"`
	Import      *jsImport   `json:"import"`
	Declaration *jsPos      `json:"declaration"`
	Library     bool        `json:"library"`
	Parameter   *jsParamRef `json:"parameter"`
	Member      *jsMemberOf `json:"member"`
	Call        *jsCall     `json:"call"`
	New         *jsCall     `json:"new"`
	Object      []jsProp    `json:"object"`
	Array       []jsValue   `json:"array"`
	Spread      *jsValue    `json:"spread"`
	Function    *jsPos      `json:"function"`
	Jsx         *jsElement  `json:"jsx"`
	Intrinsic   *string     `json:"intrinsic"`
	Text        *string     `json:"text"`
}

type jsMemberOf struct {
	Object   jsValue `json:"object"`
	Name     string  `json:"name"`
	Computed bool    `json:"computed"`
}

type jsPart struct {
	Text       *string  `json:"text"`
	Expression *jsValue `json:"expression"`
}

type jsProp struct {
	Key      *string  `json:"key"`
	Line     int      `json:"line"`
	Computed *string  `json:"computed"`
	Value    *jsValue `json:"value"`
	Spread   *jsValue `json:"spread"`
}

type jsCall struct {
	Callee        jsValue   `json:"callee"`
	Arguments     []jsValue `json:"arguments"`
	TypeArguments []*jsType `json:"typeArguments"`
}

type jsElement struct {
	Tag        jsValue  `json:"tag"`
	Attributes []jsAttr `json:"attributes"`
	Children   []jsPart `json:"children"`
}

type jsAttr struct {
	Directive string   `json:"directive"` // a Vue directive's name, such as model or bind
	Name      string   `json:"name"`
	Value     *jsValue `json:"value"`
	Spread    *jsValue `json:"spread"`
}

// jsType is a type as the source states it, in TypeScript's syntax or a
// JSDoc comment's.
type jsType struct {
	Keyword     string     `json:"keyword"`
	Literal     *jsLiteral `json:"literal"`
	Union       []*jsType  `json:"union"`
	Array       *jsType    `json:"array"`
	Object      []jsMember `json:"object"`
	Reference   *string    `json:"reference"`
	Import      *jsImport  `json:"import"`
	Declaration *jsPos     `json:"declaration"`
	Library     bool       `json:"library"`
	Arguments   []*jsType  `json:"arguments"`
	Typeof      *string    `json:"typeof"`
	ImportType  string     `json:"importType"`
	Text        string     `json:"text"`
}

type jsLiteral struct {
	String  *string `json:"string"`
	Number  *string `json:"number"`
	Boolean *bool   `json:"boolean"`
}

// describe is a value as a person reads it in a question.
func (v *jsValue) describe() string {
	switch {
	case v == nil:
		return "nothing"
	case v.String != nil:
		return fmt.Sprintf("%q", *v.String)
	case v.Template != nil:
		var b strings.Builder
		for _, p := range v.Template {
			if p.Text != nil {
				b.WriteString(*p.Text)
			} else {
				b.WriteString("${" + p.Expression.describe() + "}")
			}
		}
		return "`" + b.String() + "`"
	case v.Number != nil:
		return *v.Number
	case v.Boolean != nil:
		return fmt.Sprint(*v.Boolean)
	case v.Null != nil:
		return "null"
	case v.Regex != nil:
		return *v.Regex
	case v.Name != nil:
		return *v.Name
	case v.Member != nil:
		if v.Member.Computed {
			return fmt.Sprintf("%s[%q]", v.Member.Object.describe(), v.Member.Name)
		}
		return v.Member.Object.describe() + "." + v.Member.Name
	case v.Call != nil:
		return v.Call.Callee.describe() + "(...)"
	case v.New != nil:
		return "new " + v.New.Callee.describe() + "(...)"
	case v.Object != nil:
		return "an object"
	case v.Array != nil:
		return "an array"
	case v.Spread != nil:
		return "..." + v.Spread.describe()
	case v.Function != nil:
		return "a function"
	case v.Jsx != nil:
		return "<" + v.Jsx.Tag.describe() + ">"
	case v.Intrinsic != nil:
		return *v.Intrinsic
	case v.Text != nil:
		return *v.Text
	}
	return "an expression"
}

func (v *jsValue) stringLiteral() (string, bool) {
	if v != nil && v.String != nil {
		return *v.String, true
	}
	return "", false
}

// prop is the value an object literal gives a key, or nil.
func (v *jsValue) prop(key string) *jsValue {
	if v == nil {
		return nil
	}
	for _, p := range v.Object {
		if p.Key != nil && *p.Key == key {
			return p.Value
		}
	}
	return nil
}

// memberName is the name of the member a callee calls: get in
// router.get, or the name itself.
func (v *jsValue) memberName() string {
	switch {
	case v == nil:
		return ""
	case v.Member != nil:
		return v.Member.Name
	case v.Name != nil:
		return *v.Name
	}
	return ""
}

// jsFile is one file the dump lists.
type jsFile struct {
	path    string
	errors  int
	imports map[string]bool // the modules it imports
	config  bool
	cited   bool
}

type jsReader struct {
	r        *Read
	key      string
	res      *Result
	dump     *jsDump
	files    map[string]*jsFile
	order    []string
	dataRead map[string]string

	functions  map[string]*jsFact   // by position
	variables  map[string]*jsFact   // by position
	types      map[string]*jsFact   // by position
	exports    map[string]*jsFact   // the default export of each file
	named      []*jsFact            // every export fact: export default and export { a as b }
	directives map[string][]string  // the directives of each file's prologue, such as 'use server'
	calls      []*jsFact            // in the order of the files
	accesses   []*jsFact            // reads of the environment or a request
	elements   []*jsFact            // JSX elements
	templates  []*jsFact            // the elements of Vue templates
	pagePerms  map[string][]string  // the routes of the screens that check each permission a guard gives
	permAt     map[string]string    // where a guard checks each permission
	cat        *jsCatalogue         // the message catalogues, once read
	components []uiComponent        // the UI library\'s components the implementation file maps
	dynamics   []*jsFact            // computed requires, imports and exports
	configs    []*jsFact            // tsconfig.json and jsconfig.json
	byWithin   map[string][]*jsFact // the calls, reads, variables and elements of each function, "" for a module's own code

	questions *yaml.Node
	deployQs  *yaml.Node
	nextID    int
	counter   *int // the question numbers: nextID, or the pages reader's when it reads a dump
	notHeld   []notHeld

	entities, schemas, enums *yaml.Node
	pages                    *yaml.Node
	paths, permissions, deps *yaml.Node
	config                   *yaml.Node
	checks                   []permissionCheck
	checkFile                string
	counts                   map[string]int
	widths                   []string          // the numbers whose width is asked, by where they are
	schemaFrom               map[string]bool   // the names written as schemas or enums, so that one is written once
	schemaQueue              []*jsFact         // the declared types routes refer to, written as schemas
	queued                   map[string]bool   // the names of the declared types in schemaQueue
	validators               map[string]string // the variables holding a validation schema, by position, and the schema or enum each is written as
}

// JavaScript reads a code-facts dump of JavaScript and TypeScript source
// (ADR-089): Express and Fastify routes with the permission checks the
// implementation file names, the bodies a handler validates or declares,
// the schemas of zod, yup, joi and JSON Schema, React Router's screens
// with their fields, titles and links, fetch and axios clients, the
// environment the code reads, and message catalogues, each citing its
// file and line.
func JavaScript(dumpPath, out, key, implementation string) (*Result, error) {
	js, dumpName, err := openJS(dumpPath, key)
	if err != nil {
		return nil, err
	}
	r, d := js.r, js.dump
	res := js.res
	res.say("commit %s: the last change to %s, as the code-facts dump %s names it", r.Commit, d.Path, dumpName)
	if err := js.loadChecks(implementation); err != nil {
		return nil, err
	}
	sources, broken := 0, 0
	for _, p := range js.order {
		if !js.files[p].config {
			sources++
			if js.files[p].errors > 0 {
				broken++
			}
		}
	}
	res.say("counted %s, %d of them with syntax the TypeScript compiler could not read, %s and %s: every tracked JavaScript and TypeScript file under %s, its tsconfig.json and jsconfig.json files, and every fact the dump holds",
		plural(sources, "source file"), broken, plural(len(js.configs), "configuration file"), plural(len(d.Facts), "fact"), d.Path)
	for _, p := range js.order {
		f := js.files[p]
		if f.errors == 0 {
			continue
		}
		if f.config {
			res.say("not read: %s does not parse as the compiler's configuration", p)
			js.question("should", fmt.Sprintf("%s does not parse as a TypeScript configuration file. What does it set?", p),
				[]string{"design"}, "A configuration file says whether JSDoc types are checked, and one that does not parse says nothing.", js.at(p, "Does not parse."))
			continue
		}
		res.say("not parsed: %s has %s the TypeScript compiler could not read; the rest of it is read", p, plural(f.errors, "place"))
		js.question("should", fmt.Sprintf("%s has %s the TypeScript compiler could not read. What does the file say there, and is it built?", p, plural(f.errors, "place")),
			[]string{"design"}, "A place the parser cannot read is skipped, and what it would give is not read.", js.at(p, "Has syntax the parser could not read."))
	}
	for _, c := range js.configs {
		if c.ExtendsRead != nil && !*c.ExtendsRead {
			js.question("should", fmt.Sprintf("%s extends %s, which is not a file of the folder read, such as a package's configuration. Does it turn checkJs on?", c.File, joinAnd(c.Extends)),
				[]string{"design"}, "Only tracked files are read, so a configuration a package holds is not, and it may decide whether JSDoc types are checked.", js.at(c.File, "Extends "+joinAnd(c.Extends)+"."))
		}
	}
	js.readDynamics()
	js.readValidators()
	js.readRoutes()
	js.readTypes()
	js.readScreens()
	js.readVueRoutes(nil)
	js.readClients()
	js.readSettings()
	js.askWidths()
	for _, p := range js.order {
		f := js.files[p]
		if f.config || f.cited {
			continue
		}
		var known []string
		for m := range f.imports {
			if lib := jsLibrary(m); lib != "" && !contains(known, lib) {
				known = append(known, lib)
			}
		}
		sort.Strings(known)
		if len(known) > 0 {
			res.say("nothing read: %s imports %s, and the reader found nothing in it on a surface it reads", p, joinAnd(known))
		}
	}
	return js.write(out)
}

// openJS reads and indexes a code-facts dump of JavaScript and
// TypeScript, refusing one another compiler version made, one whose files
// are not the folder's and a stale one.
func openJS(dumpPath, key string) (*jsReader, string, error) {
	data, err := os.ReadFile(dumpPath)
	if err != nil {
		return nil, "", err
	}
	var d jsDump
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return nil, "", refuse("%s is not a code-facts dump: %v", dumpPath, err)
	}
	switch {
	case d.CodeFacts != 1:
		return nil, "", refuse("%s is not a code-facts dump in the format tools/code-facts/dump-javascript.sh writes: its codeFacts is %d, not 1", dumpPath, d.CodeFacts)
	case d.Language != "javascript":
		return nil, "", refuse("%s is a code-facts dump of %q, and extract reads one of JavaScript and TypeScript here", dumpPath, d.Language)
	case d.Parser.Name != "TypeScript" || d.Parser.Version != typeScriptVersion:
		return nil, "", refuse("%s was made by %s %s, and this specarch reads dumps that the TypeScript compiler %s made; run tools/code-facts/dump-javascript.sh of this release again and commit the dump", dumpPath, d.Parser.Name, d.Parser.Version, typeScriptVersion)
	case d.TemplateParser != nil && (d.TemplateParser.Name != "@vue/compiler-sfc" || d.TemplateParser.Version != vueCompilerVersion):
		return nil, "", refuse("%s split its .vue files with %s %s, and this specarch reads dumps that @vue/compiler-sfc %s made; run tools/code-facts/dump-javascript.sh of this release again and commit the dump", dumpPath, d.TemplateParser.Name, d.TemplateParser.Version, vueCompilerVersion)
	}
	for _, f := range d.Files {
		if strings.HasSuffix(f.File, ".vue") && d.TemplateParser == nil {
			return nil, "", refuse("%s lists the Vue file %s and names no template parser; run tools/code-facts/dump-javascript.sh of this release again and commit the dump", dumpPath, f.File)
		}
	}
	r, dumpName, err := openDump(dumpPath, d.Path, d.Commit, "tools/code-facts/dump-javascript.sh")
	if err != nil {
		return nil, "", err
	}
	js := &jsReader{r: r, key: key, res: &Result{Tree: newTree()}, dump: &d, questions: &yaml.Node{Kind: yaml.MappingNode},
		files: map[string]*jsFile{}, dataRead: map[string]string{}, functions: map[string]*jsFact{}, variables: map[string]*jsFact{},
		types: map[string]*jsFact{}, exports: map[string]*jsFact{}, directives: map[string][]string{}, byWithin: map[string][]*jsFact{}, counts: map[string]int{}, schemaFrom: map[string]bool{}, queued: map[string]bool{}, validators: map[string]string{}}
	js.counter = &js.nextID
	if err := js.index(dumpName); err != nil {
		return nil, "", err
	}
	return js, dumpName, nil
}

// jsLibrary is the library a module belongs to among those the reader
// knows, or "".
func jsLibrary(module string) string {
	for _, lib := range []string{"express", "fastify", "zod", "yup", "joi", "@hapi/joi", "react-router", "react-router-dom", "react-hook-form", "formik", "axios", "i18next", "react-i18next", "next-intl", "vue-router"} {
		if module == lib || strings.HasPrefix(module, lib+"/") {
			return lib
		}
	}
	return ""
}

// index checks the dump against the files git tracks and sorts its facts
// by what they are.
func (js *jsReader) index(dumpName string) error {
	d := js.dump
	tracked := map[string]bool{}
	for _, f := range js.r.Files {
		tracked[f] = true
	}
	for _, f := range d.Files {
		if !tracked[f.File] || !js.under(f.File) {
			return refuse("%s lists %s, which is not a tracked file under %s", dumpName, f.File, d.Path)
		}
		js.files[f.File] = &jsFile{path: f.File, errors: f.SyntaxErrors, imports: map[string]bool{}, config: jsConfigFile(f.File)}
		js.order = append(js.order, f.File)
	}
	for _, f := range js.r.Files {
		if (jsSourceFile(f) || jsConfigFile(f)) && js.under(f) && js.files[f] == nil {
			return refuse("%s does not list %s, a tracked file under %s that the reader reads; run tools/code-facts/dump-javascript.sh again and commit the dump", dumpName, f, d.Path)
		}
	}
	sort.Strings(js.order)
	for i, f := range d.Facts {
		file := js.files[f.File]
		if file == nil {
			return refuse("%s, fact %d: %s is not a file the dump lists", dumpName, i+1, f.File)
		}
		pos := (&jsPos{File: f.File, Line: f.Line, Column: f.Column}).key()
		within := f.Within.key()
		switch f.Kind {
		case "configuration":
			js.configs = append(js.configs, f)
		case "import":
			file.imports[f.Module] = true
		case "type":
			js.types[pos] = f
		case "function":
			js.functions[pos] = f
		case "variable":
			js.variables[pos] = f
			js.byWithin[within] = append(js.byWithin[within], f)
		case "call":
			js.calls = append(js.calls, f)
			js.byWithin[within] = append(js.byWithin[within], f)
		case "access":
			js.accesses = append(js.accesses, f)
			js.byWithin[within] = append(js.byWithin[within], f)
		case "jsx":
			js.elements = append(js.elements, f)
			js.byWithin[within] = append(js.byWithin[within], f)
		case "export":
			if f.Name == "default" {
				js.exports[f.File] = f
			}
			js.named = append(js.named, f)
		case "template":
			js.templates = append(js.templates, f)
		case "directive":
			js.directives[f.File] = append(js.directives[f.File], f.Name)
		case "dynamic":
			js.dynamics = append(js.dynamics, f)
		default:
			return refuse("%s, fact %d: the kind %q is not one the format has", dumpName, i+1, f.Kind)
		}
	}
	return nil
}

func jsSourceFile(f string) bool {
	for _, e := range []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs", ".vue"} {
		if strings.HasSuffix(f, e) {
			return true
		}
	}
	return false
}

func jsConfigFile(f string) bool {
	b := path.Base(f)
	return b == "tsconfig.json" || b == "jsconfig.json"
}

func (js *jsReader) under(f string) bool {
	return js.dump.Path == "." || strings.HasPrefix(f, js.dump.Path+"/")
}

// fromFolder is a file's path from the folder the dump was made from.
func (js *jsReader) fromFolder(f string) string {
	if js.dump.Path == "." {
		return f
	}
	return strings.TrimPrefix(f, js.dump.Path+"/")
}

// checkJs says whether the configuration nearest a file turns checkJs
// on, and which file that is.
func (js *jsReader) checkJs(file string) (bool, string) {
	best, depth := (*jsFact)(nil), -1
	for _, c := range js.configs {
		dir := path.Dir(c.File)
		if (dir == "." || strings.HasPrefix(file, dir+"/")) && strings.Count(dir, "/") > depth {
			best, depth = c, strings.Count(dir, "/")
		}
	}
	if best == nil {
		return false, ""
	}
	return best.CheckJs, best.File
}

// isJS says whether a file is JavaScript rather than TypeScript.
func isJS(file string) bool {
	for _, e := range []string{".js", ".jsx", ".mjs", ".cjs"} {
		if strings.HasSuffix(file, e) {
			return true
		}
	}
	return false
}

// variableOf is the variable a name was declared by, or nil.
func (js *jsReader) variableOf(v *jsValue) *jsFact {
	if v == nil || v.Name == nil || v.Declaration == nil {
		return nil
	}
	return js.variables[v.Declaration.key()]
}

// functionOf is the function a value is, or a name was declared by.
func (js *jsReader) functionOf(v *jsValue) *jsFact {
	switch {
	case v == nil:
		return nil
	case v.Function != nil:
		return js.functions[v.Function.key()]
	case v.Name != nil && v.Declaration != nil:
		return js.functions[v.Declaration.key()]
	}
	return nil
}

// moduleOf is the module and the name a value's root name is imported
// as, through a variable require gives.
func (js *jsReader) moduleOf(v *jsValue) (string, string) {
	if v == nil || v.Name == nil {
		return "", ""
	}
	if v.Import != nil {
		return v.Import.Module, v.Import.Imported
	}
	if vr := js.variableOf(v); vr != nil && vr.Value != nil && vr.Value.Call != nil {
		c := vr.Value.Call
		if c.Callee.Name != nil && *c.Callee.Name == "require" && c.Callee.Declaration == nil && len(c.Arguments) == 1 {
			if s, ok := c.Arguments[0].stringLiteral(); ok {
				return s, "*"
			}
		}
	}
	return "", ""
}

// fromLibrary says whether a value's root name is imported from one of
// the libraries named, and as which name.
func (js *jsReader) fromLibrary(v *jsValue, libs ...string) (string, bool) {
	m, imported := js.moduleOf(v)
	if m == "" {
		return "", false
	}
	for _, l := range libs {
		if jsLibrary(m) == l {
			return imported, true
		}
	}
	return "", false
}

// constString is the string a value gives: a literal, or a const of the
// files read whose value is one.
func (js *jsReader) constString(v *jsValue) (string, bool) {
	for i := 0; v != nil && i < 8; i++ {
		if s, ok := v.stringLiteral(); ok {
			return s, true
		}
		vr := js.variableOf(v)
		if vr == nil || vr.Declaration != "const" {
			return "", false
		}
		v = vr.Value
	}
	return "", false
}

// loadChecks reads the project's permission checks the implementation
// file names. A JavaScript check is a function a module exports, its
// package the module: a package's name as an import names it, or the
// path of the file from the folder read without its extension.
func (js *jsReader) loadChecks(file string) error {
	if file == "" {
		return nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) == 0 {
		return refuse("%s does not parse as YAML", file)
	}
	js.checkFile = file
	js.loadComponents(file, doc.Content[0])
	list := child(child(child(doc.Content[0], "bindings"), "http"), "permissionChecks")
	if list == nil {
		js.res.say("checks: %s names no permission check under bindings.http.permissionChecks", file)
		return nil
	}
	var names []string
	for i, item := range list.Content {
		pkg, fn := scalar(child(item, "package")), scalar(child(item, "function"))
		var n int
		if _, err := fmt.Sscan(scalar(child(item, "permissionArgument")), &n); pkg == "" || fn == "" || err != nil || n < 1 {
			js.question("must", fmt.Sprintf("The implementation file %s names a permission check (item %d of bindings.http.permissionChecks) without its package, its function and the argument, from 1, that carries the permission. Which check is it?", file, i+1),
				[]string{"paths"}, "A check named in part cannot be found in the source, and the operations it guards would be read as open.")
			continue
		}
		js.checks = append(js.checks, permissionCheck{pkg: pkg, function: fn, argument: n})
		names = append(names, pkg+"."+fn)
	}
	if len(names) > 0 {
		word := "check"
		if len(names) > 1 {
			word = "checks"
		}
		js.res.say("checks: %s names the permission %s %s", file, word, joinAnd(names))
	}
	return nil
}

// checkOf is the check the implementation file names that a callee
// calls, or nil: by its function's name, and by the module it is
// imported from or the file it is declared in.
func (js *jsReader) checkOf(callee *jsValue) *permissionCheck {
	if callee == nil || callee.Name == nil {
		return nil
	}
	for i := range js.checks {
		c := &js.checks[i]
		name := *callee.Name
		if callee.Import != nil {
			name = callee.Import.Imported
		}
		if name != c.function {
			continue
		}
		if callee.Import != nil && callee.Import.Module == c.pkg {
			return c
		}
		if callee.Declaration != nil {
			p := js.fromFolder(callee.Declaration.File)
			p = strings.TrimSuffix(p, path.Ext(p))
			if p == c.pkg || strings.TrimSuffix(p, "/index") == c.pkg {
				return c
			}
		}
	}
	return nil
}

func (js *jsReader) question(priority, text string, blocks []string, why string, cites ...*yaml.Node) string {
	into := js.questionsFor(blocks)
	*js.counter++
	id := fmt.Sprintf("Q-%d", *js.counter)
	q := mapping("question", text, "kind", "decision", "priority", priority, "blocks", blocks, "decidedBy", owner, "why", why)
	if len(cites) > 0 {
		set(q, "cites", cites)
	}
	set(into, id, q)
	return id
}

// questionsFor is where a question on these blocks sits: one on the
// configuration in the deployment stage, any other in the design stage.
func (js *jsReader) questionsFor(blocks []string) *yaml.Node {
	if len(blocks) == 0 {
		return js.questions
	}
	for _, b := range blocks {
		if b != "configuration" && !strings.HasPrefix(b, "#/configuration/") {
			return js.questions
		}
	}
	if js.deployQs == nil {
		js.deployQs = &yaml.Node{Kind: yaml.MappingNode}
	}
	return js.deployQs
}

func (js *jsReader) at(clause, says string) *yaml.Node {
	js.cite(clause)
	return citation(js.key, clause, says)
}

// cite marks the file a clause names as one something written cites.
func (js *jsReader) cite(clause string) {
	if f := js.files[strings.SplitN(clause, ":", 2)[0]]; f != nil {
		f.cited = true
	}
}

func (js *jsReader) clause(f *jsFact) string {
	return fmt.Sprintf("%s:%d", f.File, f.Line)
}

func (js *jsReader) gap(clause string, blocks []string, asked, format string, args ...any) {
	js.cite(clause)
	js.notHeld = append(js.notHeld, notHeld{text: clause + ": " + fmt.Sprintf(format, args...), clause: clause, blocks: blocks, asked: asked})
}

// readDynamics asks about each require, import or export whose module or
// name is computed.
func (js *jsReader) readDynamics() {
	for _, f := range js.dynamics {
		clause := js.clause(f)
		switch f.What {
		case "require", "import":
			how := "require"
			if f.What == "import" {
				how = "a dynamic import()"
			}
			js.question("should", fmt.Sprintf("%s loads a module with %s of %s, which is not a literal. Which module does it load, and what does that module register?", clause, how, f.Value.describe()),
				[]string{"design"}, "A module named at run time is not known by syntax, so what it declares is not read.", js.at(clause, "Loads a module by a computed name."))
		case "export":
			js.question("should", fmt.Sprintf("%s exports a value under %s, a name computed at run time. Which names does the module export there, and what are they?", clause, f.Value.describe()),
				[]string{"design"}, "An export named at run time is not known by syntax, so what imports it is not followed.", js.at(clause, "Exports under a computed name."))
		}
	}
}

// askWidths asks the width of every number whose width nothing read
// states, as extract openapi asks one with no format.
func (js *jsReader) askWidths() {
	if len(js.widths) == 0 {
		return
	}
	js.question("must", "Which width does each of these numbers take: int32, int64 within 2^53 or carried as text, or a double or a decimal with its precision and scale?",
		js.widths, "A JavaScript number has no width, and the meta-model needs one; choosing one would be a guess.")
}

// readData reads a tracked file as data and lists it as a clause.
func (js *jsReader) readData(f, title string) []byte {
	data, err := os.ReadFile(js.r.Repository.Root + "/" + f)
	if err != nil {
		return nil
	}
	js.dataRead[f] = title
	return data
}

// write puts the tree together.
func (js *jsReader) write(out string) (*Result, error) {
	res := js.res
	var clauses []*yaml.Node
	data := map[string]bool{}
	for p := range js.dataRead {
		data[p] = true
	}
	var all []string
	for _, p := range js.order {
		all = append(all, p)
	}
	for p := range data {
		if js.files[p] == nil {
			all = append(all, p)
		}
	}
	sort.Strings(all)
	for _, p := range all {
		title := "JavaScript source, read by its syntax"
		switch {
		case data[p]:
			title = js.dataRead[p]
		case jsConfigFile(p):
			title = "The compiler's configuration, read by the TypeScript compiler"
		case !isJS(p):
			title = "TypeScript source, read by its syntax and its declared types"
		}
		clauses = append(clauses, flow(mapping("clause", p, "title", title)))
	}
	if len(clauses) == 0 {
		clauses = append(clauses, flow(mapping("clause", js.dump.Path, "title", "Holds no JavaScript or TypeScript source the reader reads")))
	}
	src, err := codeSource(js.r, out, clauses)
	if err != nil {
		return nil, err
	}
	set(src, "reading", "parsed")
	has := map[string]bool{}
	models := mapping()
	if js.enums != nil {
		set(models, "enums", js.enums)
	}
	if js.entities != nil {
		set(models, "entities", js.entities)
	}
	if js.schemas != nil {
		set(models, "schemas", js.schemas)
	}
	if len(models.Content) > 0 {
		has["design"] = true
		res.Tree.put("design/models.yaml", models)
	}
	if js.pages != nil {
		has["design"] = true
		res.Tree.put("design/pages.yaml", mapping("pages", js.pages))
	}
	api := mapping()
	if js.permissions != nil {
		set(api, "permissions", js.permissions)
	}
	if js.paths != nil {
		set(api, "paths", js.paths)
	}
	if js.deps != nil {
		set(api, "dependencies", js.deps)
	}
	if len(api.Content) > 0 {
		has["design"] = true
		res.Tree.put("design/paths.yaml", api)
	}
	if js.config != nil {
		has["deployment"] = true
		res.Tree.put("deployment/configuration.yaml", mapping("configuration", js.config))
	}
	could := couldCount(js.questionsFor, js.notHeld)
	res.say("wrote %s, %s, %s, %s, %s, %s and %s: one per element the source declares through an idiom the reader knows, one question per thing the source does not say, and one per thing the meta-model cannot hold",
		plural(count(js.pages), "page"), plural(count(js.schemas), "schema"), plural(count(js.enums), "enum"),
		plural(js.counts["operations"], "operation"), plural(count(js.deps), "dependency"), plural(count(js.config), "setting"),
		plural(js.nextID+could, "question"))
	askNotHeld(res, js.questionsFor, &js.nextID, js.key, js.notHeld)
	if len(js.questions.Content) > 0 {
		has["requirements"], has["design"] = true, true
		res.Tree.put("design/questions.yaml", mapping("questions", js.questions))
	}
	if js.deployQs != nil {
		has["requirements"], has["deployment"] = true, true
		res.Tree.put("deployment/questions.yaml", mapping("questions", js.deployQs))
	}
	if has["requirements"] {
		res.Tree.put("requirements/stakeholders.yaml", mapping("stakeholders", ownerStakeholder()))
	}
	var stages []string
	for _, s := range spec.Stages {
		if has[s] {
			stages = append(stages, s)
		}
	}
	description := fmt.Sprintf("The JavaScript and TypeScript source under %s, read at commit %s by the TypeScript compiler %s through the code-facts dump, by its syntax and the types it declares, with the message catalogues there read as data. Every element cites the line that declares it; what the source does not say as a literal, or through an idiom the reader knows, is a question, and so is what the meta-model cannot hold.\n", js.dump.Path, js.r.Commit, typeScriptVersion)
	root := rootFile("JavaScript and TypeScript source of "+js.dump.Path, description, stages, mapping(js.key, src))
	res.Tree.put("specarch.yaml", root)
	return res, nil
}
