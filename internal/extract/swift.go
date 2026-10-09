package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/wirename"
)

// The version of SwiftSyntax whose dumps extract swift reads, as
// readers/swift/Package.resolved pins it; a dump another version made is
// refused, since another parser may give other facts (ADR-087).
const swiftSyntaxVersion = "604.0.0"

// codeFactsDump is a code-facts dump, as tools/code-facts/dump-swift.sh
// writes it: the version of the format, the language and the parser, the
// folder read and the commit that last changed it, the files read and
// every fact, sorted by file, line and column.
type codeFactsDump struct {
	CodeFacts int    `json:"codeFacts"`
	Language  string `json:"language"`
	Parser    struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"parser"`
	Path   string `json:"path"`
	Commit string `json:"commit"`
	Files  []struct {
		File         string `json:"file"`
		SyntaxErrors int    `json:"syntaxErrors"`
	} `json:"files"`
	Facts []*swFact `json:"facts"`
}

// swFact is one fact the Swift reader wrote. Which keys it has depends on
// its kind: import, type, function, property, case, call, subscript or
// assignment.
type swFact struct {
	Kind        string    `json:"kind"`
	File        string    `json:"file"`
	Line        int       `json:"line"`
	Column      int       `json:"column"`
	Name        string    `json:"name"`
	Declaration string    `json:"declaration"`
	Within      string    `json:"within"` // the types it is declared in, joined by dots
	Member      string    `json:"member"` // the function or property it is in
	Type        string    `json:"type"`
	Inherits    []string  `json:"inherits"`
	Attributes  []swAttr  `json:"attributes"`
	Static      bool      `json:"static"`
	Computed    bool      `json:"computed"`
	Associated  bool      `json:"associated"`
	Value       *swValue  `json:"value"`
	RawValue    *swValue  `json:"rawValue"`
	Parameters  []swParam `json:"parameters"`
	Base        *swValue  `json:"base"`
	Arguments   []swArg   `json:"arguments"`
	Closures    []swArg   `json:"closures"`
	AssignedTo  string    `json:"assignedTo"`
	InLoop      bool      `json:"inLoop"`
	InCondition bool      `json:"inCondition"`
	Key         *swValue  `json:"key"`
	Target      string    `json:"target"`
}

type swAttr struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type swParam struct {
	Label string `json:"label"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

type swArg struct {
	Label string  `json:"label"`
	Value swValue `json:"value"`
}

// swValue is an expression as the Swift reader writes it: one of its keys.
type swValue struct {
	String       *string     `json:"string"`
	Interpolated []swPart    `json:"interpolated"`
	Integer      *string     `json:"integer"`
	Float        *string     `json:"float"`
	Boolean      *bool       `json:"boolean"`
	Nil          *bool       `json:"nil"`
	TypeName     *string     `json:"type"`
	Member       *string     `json:"member"`
	Binding      *string     `json:"binding"`
	Name         *string     `json:"name"`
	KeyPath      *string     `json:"keyPath"`
	Text         *string     `json:"text"`
	Call         *swCall     `json:"call"`
	Closure      *swClosure  `json:"closure"`
	Array        []swValue   `json:"array"`
	Branches     [][]swValue `json:"branches"`
}

type swPart struct {
	Text       *string `json:"text"`
	Expression *string `json:"expression"`
}

type swCall struct {
	Name      string   `json:"name"`
	Base      *swValue `json:"base"`
	Arguments []swArg  `json:"arguments"`
	Closures  []swArg  `json:"closures"`
}

type swClosure struct {
	Parameters []string  `json:"parameters"`
	Statements []swValue `json:"statements"`
}

// describe is a value as a person reads it in a question.
func (v *swValue) describe() string {
	switch {
	case v == nil:
		return "nothing"
	case v.String != nil:
		return fmt.Sprintf("%q", *v.String)
	case v.Interpolated != nil:
		var b strings.Builder
		for _, p := range v.Interpolated {
			if p.Text != nil {
				b.WriteString(*p.Text)
			} else if p.Expression != nil {
				b.WriteString(`\(` + *p.Expression + `)`)
			}
		}
		return `"` + b.String() + `"`
	case v.Integer != nil:
		return *v.Integer
	case v.Float != nil:
		return *v.Float
	case v.Boolean != nil:
		return fmt.Sprint(*v.Boolean)
	case v.Nil != nil:
		return "nil"
	case v.TypeName != nil:
		return *v.TypeName + ".self"
	case v.Member != nil:
		return *v.Member
	case v.Binding != nil:
		return "$" + *v.Binding
	case v.Name != nil:
		return *v.Name
	case v.KeyPath != nil:
		return *v.KeyPath
	case v.Text != nil:
		return *v.Text
	case v.Call != nil:
		name := v.Call.Name
		if v.Call.Base != nil {
			name = v.Call.Base.describe() + "." + name
		}
		return name + "(...)"
	case v.Closure != nil:
		return "a closure"
	case v.Array != nil:
		return "an array"
	case v.Branches != nil:
		return "a branch"
	}
	return "an expression"
}

// stringLiteral is a value's text when it is a string literal.
func (v *swValue) stringLiteral() (string, bool) {
	if v != nil && v.String != nil {
		return *v.String, true
	}
	return "", false
}

// construct is the type a value makes when it is a call of a capitalised
// name, through the modifiers called on it: BookView().tabItem {...} makes
// BookView.
func (v *swValue) construct() (string, *swCall) {
	for v != nil && v.Call != nil {
		c := v.Call
		if c.Base == nil {
			if c.Name != "" && c.Name[0] >= 'A' && c.Name[0] <= 'Z' {
				return c.Name, c
			}
			return "", nil
		}
		v = c.Base
	}
	return "", nil
}

// modifiers is every call made on a value, outermost first.
func (v *swValue) modifiers() []*swCall {
	var list []*swCall
	for v != nil && v.Call != nil && v.Call.Base != nil {
		list = append(list, v.Call)
		v = v.Call.Base
	}
	return list
}

func (c *swCall) argument(label string) *swValue {
	for i := range c.Arguments {
		if c.Arguments[i].Label == label {
			return &c.Arguments[i].Value
		}
	}
	return nil
}

func (f *swFact) argument(label string) *swValue {
	for i := range f.Arguments {
		if f.Arguments[i].Label == label {
			return &f.Arguments[i].Value
		}
	}
	return nil
}

func (f *swFact) closure(label string) *swClosure {
	for i := range f.Closures {
		if f.Closures[i].Label == label {
			return f.Closures[i].Value.Closure
		}
	}
	return nil
}

func (f *swFact) baseName() string {
	if f.Base != nil && f.Base.Name != nil {
		return *f.Base.Name
	}
	return ""
}

func (f *swFact) attribute(name string) (swAttr, bool) {
	for _, a := range f.Attributes {
		if a.Name == name {
			return a, true
		}
	}
	return swAttr{}, false
}

// scope is the function or property a fact is in, by its type.
func (f *swFact) scope() string { return f.Within + "|" + f.Member }

// swType is a type the files read declare, with what its extensions add.
type swType struct {
	name     string // qualified by the types it is in, joined by dots
	decl     *swFact
	inherits []string
	props    []*swFact
	cases    []*swFact
	funcs    map[string]*swFact
}

func (t *swType) conforms(names ...string) bool {
	for _, i := range t.inherits {
		for _, n := range names {
			if i == n {
				return true
			}
		}
	}
	return false
}

func (t *swType) hasAttribute(name string) bool {
	_, ok := t.decl.attribute(name)
	return ok
}

// swFile is one Swift file read.
type swFile struct {
	path    string
	imports map[string]bool
	errors  int
	cited   bool
}

type swiftReader struct {
	r         *Read
	key       string
	res       *Result
	dump      *codeFactsDump
	files     map[string]*swFile
	fileOrder []string
	types     map[string]*swType
	typeOrder []string
	calls     []*swFact
	byScope   map[string][]*swFact // the calls, subscripts and assignments of each function or property
	dataRead  map[string]string    // configuration and model files read as data -> their clause title

	questions *yaml.Node
	deployQs  *yaml.Node
	nextID    int
	notHeld   []notHeld

	entities, schemas, enums *yaml.Node
	pages, menus             *yaml.Node
	paths, permissions, deps *yaml.Node
	config                   *yaml.Node
	entityNames              map[string]bool
	wireSnake                bool
	checks                   []permissionCheck
	checkFile                string
	counts                   map[string]int
}

// Swift reads a code-facts dump of Swift source (ADR-087): SwiftUI
// screens, their navigation and fields, SwiftData, Codable and Core Data
// models, URLSession clients, Vapor routes, and the settings Info.plist,
// .xcconfig files and the code give, each citing its file and line.
func Swift(dumpPath, out, key, implementation string) (*Result, error) {
	data, err := os.ReadFile(dumpPath)
	if err != nil {
		return nil, err
	}
	var d codeFactsDump
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return nil, refuse("%s is not a code-facts dump: %v", dumpPath, err)
	}
	switch {
	case d.CodeFacts != 1:
		return nil, refuse("%s is not a code-facts dump in the format tools/code-facts/dump-swift.sh writes: its codeFacts is %d, not 1", dumpPath, d.CodeFacts)
	case d.Language != "swift":
		return nil, refuse("%s is a code-facts dump of %q, and extract swift reads one of Swift", dumpPath, d.Language)
	case d.Parser.Name != "SwiftSyntax" || d.Parser.Version != swiftSyntaxVersion:
		return nil, refuse("%s was made by %s %s, and this specarch reads dumps that SwiftSyntax %s made; run tools/code-facts/dump-swift.sh of this release again and commit the dump", dumpPath, d.Parser.Name, d.Parser.Version, swiftSyntaxVersion)
	}
	r, dumpName, err := openDump(dumpPath, d.Path, d.Commit, "tools/code-facts/dump-swift.sh")
	if err != nil {
		return nil, err
	}
	sw := &swiftReader{r: r, key: key, res: &Result{Tree: newTree()}, dump: &d, questions: &yaml.Node{Kind: yaml.MappingNode},
		files: map[string]*swFile{}, types: map[string]*swType{}, byScope: map[string][]*swFact{}, dataRead: map[string]string{},
		entityNames: map[string]bool{}, counts: map[string]int{}}
	if err := sw.index(dumpName); err != nil {
		return nil, err
	}
	res := sw.res
	res.say("commit %s: the last change to %s, as the code-facts dump %s names it", r.Commit, d.Path, dumpName)
	if err := sw.loadChecks(implementation); err != nil {
		return nil, err
	}
	broken := 0
	for _, p := range sw.fileOrder {
		if sw.files[p].errors > 0 {
			broken++
		}
	}
	res.say("counted %s, %d of them with syntax SwiftSyntax could not read, and %s: every tracked Swift file under %s, and every fact the dump holds", plural(len(sw.fileOrder), "Swift file"), broken, plural(len(d.Facts), "fact"), d.Path)
	for _, p := range sw.fileOrder {
		if n := sw.files[p].errors; n > 0 {
			res.say("not parsed: %s has %s SwiftSyntax could not read as Swift; the rest of it is read", p, plural(n, "place"))
			sw.question("should", fmt.Sprintf("%s has %s SwiftSyntax could not read as Swift. What does the file say there, and is it built?", p, plural(n, "place")),
				[]string{"design"}, "A place the parser cannot read is skipped, and what it would give is not read.", sw.at(p, "Has syntax the parser could not read."))
		}
	}
	sw.readModels()
	sw.readCoreData()
	sw.readScreens()
	sw.readRoutes()
	sw.readClients()
	sw.readSettings()
	for _, p := range sw.fileOrder {
		f := sw.files[p]
		var known []string
		for _, lib := range []string{"SwiftUI", "SwiftData", "Vapor"} {
			if f.imports[lib] {
				known = append(known, lib)
			}
		}
		if len(known) > 0 && !f.cited {
			res.say("nothing read: %s imports %s, and the reader found nothing in it on a surface it reads", p, joinAnd(known))
		}
	}
	return sw.write(out)
}

// index checks the dump against the files git tracks and sorts its facts
// by what they are.
func (sw *swiftReader) index(dumpName string) error {
	d := sw.dump
	tracked := map[string]bool{}
	for _, f := range sw.r.Files {
		tracked[f] = true
	}
	under := func(p string) bool { return d.Path == "." || strings.HasPrefix(p, d.Path+"/") }
	for _, f := range d.Files {
		if !tracked[f.File] || !under(f.File) {
			return refuse("%s lists %s, which is not a tracked file under %s", dumpName, f.File, d.Path)
		}
		sw.files[f.File] = &swFile{path: f.File, imports: map[string]bool{}, errors: f.SyntaxErrors}
		sw.fileOrder = append(sw.fileOrder, f.File)
	}
	for _, f := range sw.r.Files {
		if strings.HasSuffix(f, ".swift") && under(f) && sw.files[f] == nil {
			return refuse("%s does not list %s, a tracked Swift file under %s; run tools/code-facts/dump-swift.sh again and commit the dump", dumpName, f, d.Path)
		}
	}
	sort.Strings(sw.fileOrder)
	for i, f := range d.Facts {
		file := sw.files[f.File]
		if file == nil {
			return refuse("%s, fact %d: %s is not a file the dump lists", dumpName, i+1, f.File)
		}
		switch f.Kind {
		case "import":
			file.imports[strings.Split(f.Name, ".")[0]] = true
		case "type":
			name := f.Name
			if f.Within != "" && f.Declaration != "extension" {
				name = f.Within + "." + f.Name
			}
			t := sw.types[name]
			if t == nil {
				t = &swType{name: name, funcs: map[string]*swFact{}}
				sw.types[name] = t
				sw.typeOrder = append(sw.typeOrder, name)
			}
			if f.Declaration != "extension" || t.decl == nil {
				if t.decl == nil || t.decl.Declaration == "extension" {
					t.decl = f
				}
			}
			t.inherits = append(t.inherits, f.Inherits...)
		case "property", "case", "function":
			t := sw.types[f.Within]
			if f.Within == "" {
				if f.Kind == "function" {
					t = sw.types[""]
					if t == nil {
						t = &swType{funcs: map[string]*swFact{}}
						sw.types[""] = t
					}
				} else {
					continue
				}
			}
			if t == nil {
				// An extension's members, or a type's, when the type is
				// extended under a name the reader keyed otherwise.
				t = &swType{name: f.Within, funcs: map[string]*swFact{}}
				sw.types[f.Within] = t
				sw.typeOrder = append(sw.typeOrder, f.Within)
			}
			switch f.Kind {
			case "property":
				t.props = append(t.props, f)
			case "case":
				t.cases = append(t.cases, f)
			default:
				if t.funcs[f.Name] == nil {
					t.funcs[f.Name] = f
				}
			}
		case "call", "subscript", "assignment":
			if f.Kind == "call" {
				sw.calls = append(sw.calls, f)
			}
			sw.byScope[f.scope()] = append(sw.byScope[f.scope()], f)
		default:
			return refuse("%s, fact %d: the kind %q is not one the format has", dumpName, i+1, f.Kind)
		}
	}
	return nil
}

// loadChecks reads the project's permission checks the implementation
// file names, as extract go does; a Swift check is a function or a type
// whose call carries the permission.
func (sw *swiftReader) loadChecks(file string) error {
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
	sw.checkFile = file
	list := child(child(child(doc.Content[0], "bindings"), "http"), "permissionChecks")
	if list == nil {
		sw.res.say("checks: %s names no permission check under bindings.http.permissionChecks", file)
		return nil
	}
	var names []string
	for i, item := range list.Content {
		pkg, fn := scalar(child(item, "package")), scalar(child(item, "function"))
		var n int
		if _, err := fmt.Sscan(scalar(child(item, "permissionArgument")), &n); pkg == "" || fn == "" || err != nil || n < 1 {
			sw.question("must", fmt.Sprintf("The implementation file %s names a permission check (item %d of bindings.http.permissionChecks) without its package, its function and the argument, from 1, that carries the permission. Which check is it?", file, i+1),
				[]string{"paths"}, "A check named in part cannot be found in the source, and the operations it guards would be read as open.")
			continue
		}
		sw.checks = append(sw.checks, permissionCheck{pkg: pkg, function: fn, argument: n})
		names = append(names, pkg+"."+fn)
	}
	if len(names) > 0 {
		word := "check"
		if len(names) > 1 {
			word = "checks"
		}
		sw.res.say("checks: %s names the permission %s %s", file, word, joinAnd(names))
	}
	return nil
}

func (sw *swiftReader) question(priority, text string, blocks []string, why string, cites ...*yaml.Node) string {
	into := sw.questionsFor(blocks)
	sw.nextID++
	id := fmt.Sprintf("Q-%d", sw.nextID)
	q := mapping("question", text, "kind", "decision", "priority", priority, "blocks", blocks, "decidedBy", owner, "why", why)
	if len(cites) > 0 {
		set(q, "cites", cites)
	}
	set(into, id, q)
	return id
}

// questionsFor is where a question on these blocks sits: one on the
// configuration in the deployment stage, any other in the design stage.
func (sw *swiftReader) questionsFor(blocks []string) *yaml.Node {
	if len(blocks) == 0 {
		return sw.questions
	}
	for _, b := range blocks {
		if b != "configuration" && !strings.HasPrefix(b, "#/configuration/") {
			return sw.questions
		}
	}
	if sw.deployQs == nil {
		sw.deployQs = &yaml.Node{Kind: yaml.MappingNode}
	}
	return sw.deployQs
}

func (sw *swiftReader) at(clause, says string) *yaml.Node {
	sw.cite(clause)
	return citation(sw.key, clause, says)
}

// cite marks the file a clause names as one something written cites.
func (sw *swiftReader) cite(clause string) {
	if f := sw.files[strings.SplitN(clause, ":", 2)[0]]; f != nil {
		f.cited = true
	}
}

func (sw *swiftReader) clause(f *swFact) string {
	return fmt.Sprintf("%s:%d", f.File, f.Line)
}

func (sw *swiftReader) gap(clause string, blocks []string, asked, format string, args ...any) {
	if f := sw.files[strings.SplitN(clause, ":", 2)[0]]; f != nil {
		f.cited = true
	}
	sw.notHeld = append(sw.notHeld, notHeld{text: clause + ": " + fmt.Sprintf(format, args...), clause: clause, blocks: blocks, asked: asked})
}

// imports says whether the file a fact is in imports a module.
func (sw *swiftReader) imports(f *swFact, module string) bool {
	file := sw.files[f.File]
	return file != nil && file.imports[module]
}

// write puts the tree together.
func (sw *swiftReader) write(out string) (*Result, error) {
	res := sw.res
	var clauses []*yaml.Node
	for _, p := range sw.fileOrder {
		clauses = append(clauses, flow(mapping("clause", p, "title", "Swift source, read by its syntax")))
	}
	var data []string
	for p := range sw.dataRead {
		data = append(data, p)
	}
	sort.Strings(data)
	for _, p := range data {
		clauses = append(clauses, flow(mapping("clause", p, "title", sw.dataRead[p])))
	}
	if len(clauses) == 0 {
		clauses = append(clauses, flow(mapping("clause", sw.dump.Path, "title", "Holds no Swift source the reader reads")))
	}
	src, err := codeSource(sw.r, out, clauses)
	if err != nil {
		return nil, err
	}
	set(src, "reading", "parsed")
	has := map[string]bool{}
	models := mapping()
	if sw.enums != nil {
		set(models, "enums", sw.enums)
	}
	if sw.entities != nil {
		set(models, "entities", sw.entities)
	}
	if sw.schemas != nil {
		set(models, "schemas", sw.schemas)
	}
	if len(models.Content) > 0 {
		has["design"] = true
		res.Tree.put("design/models.yaml", models)
	}
	screens := mapping()
	if sw.pages != nil {
		set(screens, "pages", sw.pages)
	}
	if sw.menus != nil {
		set(screens, "menus", sw.menus)
	}
	if len(screens.Content) > 0 {
		has["design"] = true
		res.Tree.put("design/pages.yaml", screens)
	}
	api := mapping()
	if sw.permissions != nil {
		set(api, "permissions", sw.permissions)
	}
	if sw.paths != nil {
		set(api, "paths", sw.paths)
	}
	if sw.deps != nil {
		set(api, "dependencies", sw.deps)
	}
	if len(api.Content) > 0 {
		has["design"] = true
		res.Tree.put("design/paths.yaml", api)
	}
	if sw.config != nil {
		has["deployment"] = true
		res.Tree.put("deployment/configuration.yaml", mapping("configuration", sw.config))
	}
	could := couldCount(sw.questionsFor, sw.notHeld)
	res.say("wrote %s, %s, %s, %s, %s, %s, %s and %s: one per element the source declares through an idiom the reader knows, one question per thing the source does not say, and one per thing the meta-model cannot hold",
		plural(count(sw.pages), "page"), plural(count(sw.entities), "entity"), plural(count(sw.schemas), "schema"),
		plural(count(sw.enums), "enum"), plural(sw.counts["operations"], "operation"), plural(count(sw.deps), "dependency"), plural(count(sw.config), "setting"),
		plural(sw.nextID+could, "question"))
	askNotHeld(res, sw.questionsFor, &sw.nextID, sw.key, sw.notHeld)
	if len(sw.questions.Content) > 0 {
		has["requirements"], has["design"] = true, true
		res.Tree.put("design/questions.yaml", mapping("questions", sw.questions))
	}
	if sw.deployQs != nil {
		has["requirements"], has["deployment"] = true, true
		res.Tree.put("deployment/questions.yaml", mapping("questions", sw.deployQs))
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
	description := fmt.Sprintf("The Swift source under %s, read at commit %s by SwiftSyntax %s through the code-facts dump, by its syntax alone, with the property lists, build settings and Core Data models there read as data. Every element cites the line that declares it; what the source does not say as a literal, or through an idiom the reader knows, is a question, and so is what the meta-model cannot hold.\n", sw.dump.Path, sw.r.Commit, swiftSyntaxVersion)
	root := rootFile("Swift source of "+sw.dump.Path, description, stages, mapping(sw.key, src))
	if sw.wireSnake {
		set(child(root, "info"), "wireNames", wirename.SnakeCase)
	}
	res.Tree.put("specarch.yaml", root)
	return res, nil
}

// count is how many keys a mapping holds.
func count(m *yaml.Node) int {
	if m == nil {
		return 0
	}
	return len(m.Content) / 2
}
