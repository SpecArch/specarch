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
	"github.com/SpecArch/specarch/internal/wirename"
)

// The version of the analyzer package whose dumps extract dart reads, as
// readers/dart/pubspec.lock pins it; a dump another version made is
// refused, since another parser may give other facts (ADR-092).
const dartAnalyzerVersion = "14.4.0"

type dartDump struct {
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
	Facts []*dartFact `json:"facts"`
}

// dartFact is one fact the Dart reader wrote. Which keys it has depends on
// its kind: import, class, enum, field, variable, constructor, method,
// call, compare or index.
type dartFact struct {
	Kind   string `json:"kind"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`

	URI    string `json:"uri"`
	Prefix string `json:"prefix"`

	Name        string           `json:"name"`
	Abstract    bool             `json:"abstract"`
	Extends     string           `json:"extends"`
	With        []string         `json:"with"`
	Implements  []string         `json:"implements"`
	Annotations []dartAnnotation `json:"annotations"`
	Values      []dartEnumValue  `json:"values"`

	Within       string      `json:"within"`
	Type         string      `json:"type"`
	Static       bool        `json:"static"`
	Final        bool        `json:"final"`
	Const        bool        `json:"const"`
	Getter       bool        `json:"getter"`
	Value        *dartValue  `json:"value"`
	Factory      bool        `json:"factory"`
	Redirect     string      `json:"redirect"`
	Parameters   []dartParam `json:"parameters"`
	Returns      string      `json:"returns"`
	ReturnValues []dartValue `json:"returnValues"`

	Target     *dartValue `json:"target"`
	Cascaded   bool       `json:"cascaded"`
	CascadeOf  string     `json:"cascadeOf"`
	Arguments  []dartArg  `json:"arguments"`
	AssignedTo string     `json:"assignedTo"`

	On      string     `json:"on"`
	Default bool       `json:"default"`
	Key     *dartValue `json:"key"`

	InLoop      bool   `json:"inLoop"`
	InCondition bool   `json:"inCondition"`
	Function    int    `json:"function"` // the line of the closure it is in, or 0
	Branch      string `json:"branch"`   // the case of a switch, or the value of an if, on a request's method it is in
}

type dartAnnotation struct {
	Name      string    `json:"name"`
	Arguments []dartArg `json:"arguments"`
}

type dartEnumValue struct {
	Name        string           `json:"name"`
	Line        int              `json:"line"`
	Column      int              `json:"column"`
	Annotations []dartAnnotation `json:"annotations"`
}

type dartParam struct {
	Name        string           `json:"name"`
	Type        string           `json:"type"`
	Named       bool             `json:"named"`
	Required    bool             `json:"required"`
	Field       bool             `json:"field"`
	Default     *dartValue       `json:"default"`
	Annotations []dartAnnotation `json:"annotations"`
}

type dartArg struct {
	Name  string    `json:"name"`
	Value dartValue `json:"value"`
}

type dartValue struct {
	String       *string     `json:"string"`
	Interpolated []dartPart  `json:"interpolated"`
	Integer      *string     `json:"integer"`
	Double       *string     `json:"double"`
	Boolean      *bool       `json:"boolean"`
	Null         *bool       `json:"null"`
	Name         *string     `json:"name"`
	Call         *dartCall   `json:"call"`
	List         []dartValue `json:"list"`
	Map          []dartEntry `json:"map"`
	Function     *dartFunc   `json:"function"`
	Assign       *dartAssign `json:"assign"`
	Text         *string     `json:"text"`
}

type dartPart struct {
	Text       *string `json:"text"`
	Expression *string `json:"expression"`
}

type dartCall struct {
	Line      int        `json:"line"`
	Target    *dartValue `json:"target"`
	Name      string     `json:"name"`
	Arguments []dartArg  `json:"arguments"`
	Const     bool       `json:"const"`
}

type dartEntry struct {
	Key   *dartValue `json:"key"`
	Value *dartValue `json:"value"`
	Text  *string    `json:"text"`
}

type dartFunc struct {
	Line       int         `json:"line"`
	Parameters []string    `json:"parameters"`
	Returns    []dartValue `json:"returns"`
}

type dartAssign struct {
	Target string    `json:"target"`
	Value  dartValue `json:"value"`
}

func (v *dartValue) describe() string {
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
				b.WriteString("${" + *p.Expression + "}")
			}
		}
		return "'" + b.String() + "'"
	case v.Integer != nil:
		return *v.Integer
	case v.Double != nil:
		return *v.Double
	case v.Boolean != nil:
		return fmt.Sprint(*v.Boolean)
	case v.Null != nil:
		return "null"
	case v.Name != nil:
		return *v.Name
	case v.Call != nil:
		if v.Call.Target != nil {
			return v.Call.Target.describe() + "." + v.Call.Name + "(...)"
		}
		return v.Call.Name + "(...)"
	case v.List != nil:
		return "a list"
	case v.Map != nil:
		return "a map"
	case v.Function != nil:
		return "a function"
	case v.Assign != nil:
		return v.Assign.Target + " = ..."
	case v.Text != nil:
		return *v.Text
	}
	return "an expression"
}

func (v *dartValue) stringLiteral() (string, bool) {
	if v != nil && v.String != nil {
		return *v.String, true
	}
	return "", false
}

func dartArgument(args []dartArg, name string) *dartValue {
	for i := range args {
		if args[i].Name == name {
			return &args[i].Value
		}
	}
	return nil
}

func dartPositional(args []dartArg, n int) *dartValue {
	i := 0
	for k := range args {
		if args[k].Name != "" {
			continue
		}
		if i == n {
			return &args[k].Value
		}
		i++
	}
	return nil
}

func (f *dartFact) annotation(names ...string) *dartAnnotation {
	for i := range f.Annotations {
		for _, n := range names {
			if f.Annotations[i].Name == n {
				return &f.Annotations[i]
			}
		}
	}
	return nil
}

type dartFile struct {
	path      string
	errors    int
	imports   []string
	generated bool
	cited     bool
}

type dartReader struct {
	r        *Read
	key      string
	res      *Result
	dump     *dartDump
	files    map[string]*dartFile
	order    []string
	dataRead map[string]string

	classes   map[string]*dartFact   // by name
	enums     map[string]*dartFact   // by name
	members   map[string][]*dartFact // the fields, constructors and methods of each class, by its name
	functions map[string]*dartFact   // top-level functions, by name
	variables []*dartFact
	calls     []*dartFact
	compares  []*dartFact
	indexes   []*dartFact

	questions *yaml.Node
	deployQs  *yaml.Node
	nextID    int
	notHeld   []notHeld

	pages, menus, schemas, enumsOut *yaml.Node
	paths, permissions, deps        *yaml.Node
	config                          *yaml.Node
	checks                          []permissionCheck
	checkFile                       string
	counts                          map[string]int
	widths                          []string
	wireSnake                       bool

	fieldAt    map[string]*dartFact // a freezed model's factory, by its class
	memberAt   map[string]*dartFact // a json_serializable model's field, by class.field
	enumWanted map[string]bool
	enumOrder  []*dartFact
}

// Dart reads a code-facts dump of Dart source (ADR-092): go_router's and
// Navigator's screens with their fields and titles, json_serializable
// and freezed models, retrofit, dio and http clients, shelf_router and
// dart_frog routes with the project's named check, and the settings
// fromEnvironment and Platform.environment give, each citing its file and
// line.
func Dart(dumpPath, out, key, implementation string) (*Result, error) {
	data, err := os.ReadFile(dumpPath)
	if err != nil {
		return nil, err
	}
	var d dartDump
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return nil, refuse("%s is not a code-facts dump: %v", dumpPath, err)
	}
	switch {
	case d.CodeFacts != 1:
		return nil, refuse("%s is not a code-facts dump in the format tools/code-facts/dump-dart.sh writes: its codeFacts is %d, not 1", dumpPath, d.CodeFacts)
	case d.Language != "dart":
		return nil, refuse("%s is a code-facts dump of %q, and extract dart reads one of Dart", dumpPath, d.Language)
	case d.Parser.Name != "analyzer" || d.Parser.Version != dartAnalyzerVersion:
		return nil, refuse("%s was made by %s %s, and this specarch reads dumps that the analyzer %s made; run tools/code-facts/dump-dart.sh of this release again and commit the dump", dumpPath, d.Parser.Name, d.Parser.Version, dartAnalyzerVersion)
	}
	r, dumpName, err := openDump(dumpPath, d.Path, d.Commit, "tools/code-facts/dump-dart.sh")
	if err != nil {
		return nil, err
	}
	dr := &dartReader{r: r, key: key, res: &Result{Tree: newTree()}, dump: &d, questions: &yaml.Node{Kind: yaml.MappingNode},
		files: map[string]*dartFile{}, dataRead: map[string]string{}, classes: map[string]*dartFact{}, enums: map[string]*dartFact{},
		members: map[string][]*dartFact{}, functions: map[string]*dartFact{}, counts: map[string]int{}}
	if err := dr.index(dumpName); err != nil {
		return nil, err
	}
	res := dr.res
	res.say("commit %s: the last change to %s, as the code-facts dump %s names it", r.Commit, d.Path, dumpName)
	if err := dr.loadChecks(implementation); err != nil {
		return nil, err
	}
	broken := 0
	for _, p := range dr.order {
		if dr.files[p].errors > 0 {
			broken++
		}
	}
	res.say("counted %s, %d of them with syntax the analyzer could not read, and %s: every tracked Dart file under %s, and every fact the dump holds", plural(len(dr.order), "Dart file"), broken, plural(len(d.Facts), "fact"), d.Path)
	for _, p := range dr.order {
		f := dr.files[p]
		if f.generated {
			res.say("generated: %s is the output of a code generator; its input, the file it is part of, is read instead", p)
		}
		if n := f.errors; n > 0 {
			res.say("not parsed: %s has %s the analyzer could not read; the rest of it is read", p, plural(n, "place"))
			dr.question("should", fmt.Sprintf("%s has %s the analyzer could not read as Dart. What does the file say there, and is it built?", p, plural(n, "place")),
				[]string{"design"}, "A place the parser cannot read is skipped, and what it would give is not read.", dr.at(p, "Has syntax the parser could not read."))
		}
	}
	dr.readModels()
	dr.readScreens()
	dr.readServer()
	dr.readClients()
	dr.readSettings()
	if len(dr.widths) > 0 {
		dr.question("must", "Which width does each of these numbers take: int32, int64 within 2^53 or carried as text, or a double or a decimal with its precision and scale?",
			dr.widths, "A Dart int is 64 bits on the native platforms and a double on the web, and the meta-model needs a width; choosing one would be a guess.")
	}
	for _, p := range dr.order {
		f := dr.files[p]
		if f.cited || f.generated {
			continue
		}
		var known []string
		for _, u := range f.imports {
			for _, lib := range []string{"go_router", "json_annotation", "freezed_annotation", "retrofit", "dio", "http", "shelf_router", "dart_frog", "flutter_form_builder"} {
				if strings.HasPrefix(u, "package:"+lib+"/") && !contains(known, lib) {
					known = append(known, lib)
				}
			}
		}
		sort.Strings(known)
		if len(known) > 0 {
			res.say("nothing read: %s imports %s, and the reader found nothing in it on a surface it reads", p, joinAnd(known))
		}
	}
	return dr.write(out)
}

func (dr *dartReader) under(f string) bool {
	return dr.dump.Path == "." || strings.HasPrefix(f, dr.dump.Path+"/")
}

func (dr *dartReader) fromFolder(f string) string {
	if dr.dump.Path == "." {
		return f
	}
	return strings.TrimPrefix(f, dr.dump.Path+"/")
}

// index checks the dump against the files git tracks and sorts its facts.
func (dr *dartReader) index(dumpName string) error {
	d := dr.dump
	tracked := map[string]bool{}
	for _, f := range dr.r.Files {
		tracked[f] = true
	}
	for _, f := range d.Files {
		if !tracked[f.File] || !dr.under(f.File) {
			return refuse("%s lists %s, which is not a tracked file under %s", dumpName, f.File, d.Path)
		}
		gen := strings.HasSuffix(f.File, ".g.dart") || strings.HasSuffix(f.File, ".freezed.dart")
		dr.files[f.File] = &dartFile{path: f.File, errors: f.SyntaxErrors, generated: gen}
		dr.order = append(dr.order, f.File)
	}
	for _, f := range dr.r.Files {
		if strings.HasSuffix(f, ".dart") && dr.under(f) && dr.files[f] == nil {
			return refuse("%s does not list %s, a tracked Dart file under %s; run tools/code-facts/dump-dart.sh again and commit the dump", dumpName, f, d.Path)
		}
	}
	sort.Strings(dr.order)
	for i, f := range d.Facts {
		file := dr.files[f.File]
		if file == nil {
			return refuse("%s, fact %d: %s is not a file the dump lists", dumpName, i+1, f.File)
		}
		if file.generated {
			continue
		}
		switch f.Kind {
		case "import":
			file.imports = append(file.imports, f.URI)
		case "class":
			if dr.classes[f.Name] == nil {
				dr.classes[f.Name] = f
			}
		case "enum":
			if dr.enums[f.Name] == nil {
				dr.enums[f.Name] = f
			}
		case "field", "constructor":
			dr.members[f.Within] = append(dr.members[f.Within], f)
		case "method":
			if f.Within == "" {
				if dr.functions[f.Name] == nil {
					dr.functions[f.Name] = f
				}
			} else {
				dr.members[f.Within] = append(dr.members[f.Within], f)
			}
		case "variable":
			dr.variables = append(dr.variables, f)
		case "call":
			dr.calls = append(dr.calls, f)
		case "compare":
			dr.compares = append(dr.compares, f)
		case "index":
			dr.indexes = append(dr.indexes, f)
		default:
			return refuse("%s, fact %d: the kind %q is not one the format has", dumpName, i+1, f.Kind)
		}
	}
	return nil
}

// imports says whether the file a fact is in imports a package.
func (dr *dartReader) imports(file, pkg string) bool {
	f := dr.files[file]
	if f == nil {
		return false
	}
	for _, u := range f.imports {
		if strings.HasPrefix(u, "package:"+pkg+"/") {
			return true
		}
	}
	return false
}

// loadChecks reads the project's permission checks the implementation
// file names. A Dart check is a top-level function, its package the path
// of its file from the folder read without .dart, or a package: URI an
// import names.
func (dr *dartReader) loadChecks(file string) error {
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
	dr.checkFile = file
	list := child(child(child(doc.Content[0], "bindings"), "http"), "permissionChecks")
	if list == nil {
		dr.res.say("checks: %s names no permission check under bindings.http.permissionChecks", file)
		return nil
	}
	var names []string
	for i, item := range list.Content {
		pkg, fn := scalar(child(item, "package")), scalar(child(item, "function"))
		var n int
		if _, err := fmt.Sscan(scalar(child(item, "permissionArgument")), &n); pkg == "" || fn == "" || err != nil || n < 1 {
			dr.question("must", fmt.Sprintf("The implementation file %s names a permission check (item %d of bindings.http.permissionChecks) without its package, its function and the argument, from 1, that carries the permission. Which check is it?", file, i+1),
				[]string{"paths"}, "A check named in part cannot be found in the source, and the operations it guards would be read as open.")
			continue
		}
		dr.checks = append(dr.checks, permissionCheck{pkg: pkg, function: fn, argument: n})
		names = append(names, pkg+"."+fn)
	}
	if len(names) > 0 {
		dr.res.say("checks: %s names the permission %s %s", file, map[bool]string{true: "check", false: "checks"}[len(names) == 1], joinAnd(names))
	}
	return nil
}

// checkOf is the check a call is: by its function's name, declared in the
// files read under its package's path, or imported by its package URI.
func (dr *dartReader) checkOf(name string, file string) *permissionCheck {
	for i := range dr.checks {
		c := &dr.checks[i]
		if c.function != name {
			continue
		}
		if fn := dr.functions[name]; fn != nil {
			p := strings.TrimSuffix(dr.fromFolder(fn.File), ".dart")
			if p == c.pkg {
				return c
			}
		}
		if f := dr.files[file]; f != nil {
			for _, u := range f.imports {
				if strings.TrimSuffix(u, ".dart") == strings.TrimSuffix(c.pkg, ".dart") {
					return c
				}
			}
		}
	}
	return nil
}

func (dr *dartReader) question(priority, text string, blocks []string, why string, cites ...*yaml.Node) string {
	into := dr.questionsFor(blocks)
	dr.nextID++
	id := fmt.Sprintf("Q-%d", dr.nextID)
	q := mapping("question", text, "kind", "decision", "priority", priority, "blocks", blocks, "decidedBy", owner, "why", why)
	if len(cites) > 0 {
		set(q, "cites", cites)
	}
	set(into, id, q)
	return id
}

func (dr *dartReader) questionsFor(blocks []string) *yaml.Node {
	if len(blocks) == 0 {
		return dr.questions
	}
	for _, b := range blocks {
		if b != "configuration" && !strings.HasPrefix(b, "#/configuration/") {
			return dr.questions
		}
	}
	if dr.deployQs == nil {
		dr.deployQs = &yaml.Node{Kind: yaml.MappingNode}
	}
	return dr.deployQs
}

func (dr *dartReader) at(clause, says string) *yaml.Node {
	dr.cite(clause)
	return citation(dr.key, clause, says)
}

func (dr *dartReader) cite(clause string) {
	if f := dr.files[strings.SplitN(clause, ":", 2)[0]]; f != nil {
		f.cited = true
	}
}

func (dr *dartReader) clause(f *dartFact) string { return fmt.Sprintf("%s:%d", f.File, f.Line) }

func (dr *dartReader) gap(clause string, blocks []string, asked, format string, args ...any) {
	dr.cite(clause)
	dr.notHeld = append(dr.notHeld, notHeld{text: clause + ": " + fmt.Sprintf(format, args...), clause: clause, blocks: blocks, asked: asked})
}

func (dr *dartReader) write(out string) (*Result, error) {
	res := dr.res
	var clauses []*yaml.Node
	for _, p := range dr.order {
		title := "Dart source, read by its syntax"
		if dr.files[p].generated {
			title = "Dart source a code generator wrote, not read"
		}
		clauses = append(clauses, flow(mapping("clause", p, "title", title)))
	}
	var data []string
	for p := range dr.dataRead {
		data = append(data, p)
	}
	sort.Strings(data)
	for _, p := range data {
		clauses = append(clauses, flow(mapping("clause", p, "title", dr.dataRead[p])))
	}
	if len(clauses) == 0 {
		clauses = append(clauses, flow(mapping("clause", dr.dump.Path, "title", "Holds no Dart source the reader reads")))
	}
	src, err := codeSource(dr.r, out, clauses)
	if err != nil {
		return nil, err
	}
	set(src, "reading", "parsed")
	has := map[string]bool{}
	models := mapping()
	if dr.enumsOut != nil {
		set(models, "enums", dr.enumsOut)
	}
	if dr.schemas != nil {
		set(models, "schemas", dr.schemas)
	}
	if len(models.Content) > 0 {
		has["design"] = true
		res.Tree.put("design/models.yaml", models)
	}
	screens := mapping()
	if dr.pages != nil {
		set(screens, "pages", dr.pages)
	}
	if dr.menus != nil {
		set(screens, "menus", dr.menus)
	}
	if len(screens.Content) > 0 {
		has["design"] = true
		res.Tree.put("design/pages.yaml", screens)
	}
	api := mapping()
	if dr.permissions != nil {
		set(api, "permissions", dr.permissions)
	}
	if dr.paths != nil {
		set(api, "paths", dr.paths)
	}
	if dr.deps != nil {
		set(api, "dependencies", dr.deps)
	}
	if len(api.Content) > 0 {
		has["design"] = true
		res.Tree.put("design/paths.yaml", api)
	}
	if dr.config != nil {
		has["deployment"] = true
		res.Tree.put("deployment/configuration.yaml", mapping("configuration", dr.config))
	}
	could := couldCount(dr.questionsFor, dr.notHeld)
	res.say("wrote %s, %s, %s, %s, %s, %s and %s: one per element the source declares through an idiom the reader knows, one question per thing the source does not say, and one per thing the meta-model cannot hold",
		plural(count(dr.pages), "page"), plural(count(dr.schemas), "schema"), plural(count(dr.enumsOut), "enum"),
		plural(dr.counts["operations"], "operation"), plural(count(dr.deps), "dependency"), plural(count(dr.config), "setting"),
		plural(dr.nextID+could, "question"))
	askNotHeld(res, dr.questionsFor, &dr.nextID, dr.key, dr.notHeld)
	if len(dr.questions.Content) > 0 {
		has["requirements"], has["design"] = true, true
		res.Tree.put("design/questions.yaml", mapping("questions", dr.questions))
	}
	if dr.deployQs != nil {
		has["requirements"], has["deployment"] = true, true
		res.Tree.put("deployment/questions.yaml", mapping("questions", dr.deployQs))
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
	description := fmt.Sprintf("The Dart source under %s, read at commit %s by the analyzer %s through the code-facts dump, by its syntax alone, resolving no package. Every element cites the line that declares it; what the source does not say as a literal, or through an idiom the reader knows, is a question, and so is what the meta-model cannot hold.\n", dr.dump.Path, dr.r.Commit, dartAnalyzerVersion)
	root := rootFile("Dart source of "+dr.dump.Path, description, stages, mapping(dr.key, src))
	if dr.wireSnake {
		set(child(root, "info"), "wireNames", wirename.SnakeCase)
	}
	res.Tree.put("specarch.yaml", root)
	return res, nil
}

// widgetPage is a page's name from its widget: NotesScreen is notes.
func widgetPage(name string) string {
	base := strings.TrimPrefix(name, "_")
	for _, suffix := range []string{"Screen", "Page", "View"} {
		if strings.HasSuffix(base, suffix) && len(base) > len(suffix) {
			base = strings.TrimSuffix(base, suffix)
			break
		}
	}
	return kebab(base)
}

// dartRoutePath writes a go_router or shelf_router path as a path
// template: :id and <id> are {id}; why says what it cannot hold.
func dartRoutePath(segs []string) (string, string) {
	var parts []string
	for _, s := range segs {
		switch {
		case strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">"):
			name := s[1 : len(s)-1]
			if strings.Contains(name, "|") {
				return "", "the parameter " + s + " has a pattern, which a path template cannot say"
			}
			parts = append(parts, "{"+name+"}")
		case strings.HasPrefix(s, ":"):
			if strings.ContainsAny(s[1:], "()[]?*+") {
				return "", "the parameter " + s + " has a pattern, which a path template cannot say"
			}
			parts = append(parts, "{"+s[1:]+"}")
		case strings.ContainsAny(s, "*<>()"):
			return "", "the segment " + s + " is a pattern, which a path template cannot say"
		default:
			parts = append(parts, s)
		}
	}
	p := "/" + strings.Join(parts, "/")
	if _, why := pathParameters(p); why != "" {
		return "", why
	}
	return p, ""
}

// dartFrogPath is the route a dart_frog file under routes/ serves.
func dartFrogPath(rel string) (string, bool) {
	rel = strings.TrimSuffix(rel, ".dart")
	var segs []string
	for _, s := range strings.Split(rel, "/") {
		switch {
		case s == "index":
		case strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"):
			segs = append(segs, ":"+s[1:len(s)-1])
		default:
			segs = append(segs, s)
		}
	}
	p, why := dartRoutePath(segs)
	return p, why == ""
}

var _ = path.Base
