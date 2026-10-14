package facts

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/SpecArch/specarch/third_party/typescript-go/ast"
	"github.com/SpecArch/specarch/third_party/typescript-go/bundled"
	"github.com/SpecArch/specarch/third_party/typescript-go/checker"
	"github.com/SpecArch/specarch/third_party/typescript-go/collections"
	"github.com/SpecArch/specarch/third_party/typescript-go/compiler"
	"github.com/SpecArch/specarch/third_party/typescript-go/core"
	"github.com/SpecArch/specarch/third_party/typescript-go/module"
	"github.com/SpecArch/specarch/third_party/typescript-go/scanner"
	"github.com/SpecArch/specarch/third_party/typescript-go/tsoptions"
	"github.com/SpecArch/specarch/third_party/typescript-go/tspath"
	"github.com/SpecArch/specarch/third_party/typescript-go/vfs"
)

// The parser the JavaScript reader reads with, as its dumps name it.
const parserName = "TypeScript"

// ParserVersion is the version of the TypeScript compiler the reader is
// built on (third_party/typescript-go); specarch extract refuses a dump
// another version made.
func ParserVersion() string { return core.Version() }

// The modules whose calls and elements the reader writes wherever they
// are used: the libraries specarch extract javascript knows. A module is
// known when its name is one of these or starts with one and a slash.
var knownModules = []string{
	"express", "fastify", "zod", "yup", "joi", "@hapi/joi", "react-router", "react-router-dom",
	"react-hook-form", "@hookform/resolvers", "formik", "axios", "i18next", "react-i18next", "next-intl", "vue-router",
}

// The names of the calls the reader writes whatever their callee is:
// the methods of a router, a schema, a form, a client or a catalogue that
// are called on a value the reader cannot name by its module.
var callNames = setOf(
	// routes and their middleware
	"get", "post", "put", "patch", "delete", "del", "all", "options", "head", "use", "route", "register", "listen", "Router", "addHook",
	// validation
	"parse", "safeParse", "parseAsync", "safeParseAsync", "validate", "validateSync", "validateAsync", "cast",
	// forms
	"useForm", "useFormik", "getFieldProps", "useField",
	// clients
	"fetch", "request", "create",
	// message catalogues
	"t", "useTranslation", "useTranslations", "getTranslations", "init",
	// screens
	"createBrowserRouter", "createHashRouter", "createMemoryRouter", "useRoutes", "navigate", "useNavigate", "redirect",
	// modules
	"require",
	// Vue, vue-router, Nuxt and Nitro, whose names Nuxt makes global
	"definePageMeta", "defineNuxtRouteMiddleware", "navigateTo", "useFetch", "$fetch", "createRouter", "beforeEach", "beforeEnter",
	"defineEventHandler", "eventHandler", "readBody", "readValidatedBody", "getQuery", "getValidatedQuery", "getRouterParam", "getRouterParams",
)

// The elements of HTML the reader writes: a page's heading, a form and
// its controls, and a link.
var intrinsicElements = setOf("h1", "form", "input", "select", "textarea", "a")

// The members of a request that the reader writes reads of.
var requestMembers = setOf("body", "params", "query", "headers", "method")

// The calls of an array that run a function once per item: a function
// given to one runs in a loop.
var loopCalls = setOf("map", "forEach", "flatMap", "filter", "reduce", "some", "every")

var sourceExtensions = []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"}
var configNames = setOf("tsconfig.json", "jsconfig.json")
var jsFile = regexp.MustCompile(`\.[cm]?jsx?$`)
var lowerStart = regexp.MustCompile(`^[a-z]`)

// How deep a value is written; a value below this is written as its text.
const depth = 32
const textLimit = 160

func setOf(names ...string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}

// vueFile is a Vue single-file component as the reader reads it: its
// script blocks as one TypeScript or JavaScript file.
type vueFile struct {
	virtual string // the absolute path the script is read under
	text    string
	sfc     *sfcDescriptor
	ext     string
}

// jsReader reads one folder's JavaScript, TypeScript and Vue files.
type jsReader struct {
	root         string // the repository's root, absolute
	tracked      map[string]bool
	trackedDirs  map[string]bool
	vue          map[string]*vueFile // by the .vue file's path from the root
	virtualToVue map[string]string   // a virtual script's path -> the .vue file
	program      *compiler.Program
	checker      *checker.Checker
	resolver     *module.Resolver
	options      *core.CompilerOptions
	sf           *ast.SourceFile // the file being read
	facts        []*obj
}

// JavaScript reads the JavaScript, TypeScript and Vue files and the
// tsconfig.json and jsconfig.json files listed, paths from the repository's
// root, with the TypeScript compiler, and writes the code-facts dump of
// the folder path at commit (docs/reading-code.md, ADR-089, ADR-116). It
// says what the syntax says, with the declarations the compiler's checker
// resolves among the files read and the types that come with TypeScript,
// and nothing more. No file outside the list is read: no node_modules, no
// package's types.
func JavaScript(root, folder, commit string, listed []string) ([]byte, error) {
	r := &jsReader{root: root, tracked: map[string]bool{}, trackedDirs: map[string]bool{}, vue: map[string]*vueFile{}, virtualToVue: map[string]string{}}
	for _, f := range listed {
		r.tracked[f] = true
		for d := path.Dir(f); d != "." && d != "/" && !r.trackedDirs[d]; d = path.Dir(d) {
			r.trackedDirs[d] = true
		}
	}
	var sources, configs []string
	for _, f := range listed {
		if isSourceFile(f) {
			sources = append(sources, f)
		}
		if configNames[path.Base(f)] {
			configs = append(configs, f)
		}
	}
	sort.Slice(sources, func(i, j int) bool { return jsLess(sources[i], sources[j]) })
	sort.Slice(configs, func(i, j int) bool { return jsLess(configs[i], configs[j]) })

	for _, f := range sources {
		if !strings.HasSuffix(f, ".vue") {
			continue
		}
		data, err := os.ReadFile(r.abs(f))
		if err != nil {
			return nil, err
		}
		v := readVue(string(data))
		v.virtual = r.abs(f) + v.ext
		r.vue[f] = v
		r.virtualToVue[v.virtual] = f
	}

	fs := bundled.WrapFS(&jsFS{r: r})
	host := compiler.NewCompilerHost(root, fs, bundled.LibPath(), nil, nil)

	// The configuration files, as the compiler reads them; the one nearest
	// the folder's root gives the program its module paths.
	byDepth := append([]string(nil), configs...)
	sort.SliceStable(byDepth, func(i, j int) bool {
		a, b := strings.Count(byDepth[i], "/"), strings.Count(byDepth[j], "/")
		if a != b {
			return a < b
		}
		return jsLess(byDepth[i], byDepth[j])
	})
	configFacts := map[string]*obj{}
	var paths *configPaths
	for _, file := range byDepth {
		fact, p := r.readConfig(file, host)
		configFacts[file] = fact
		if paths == nil && p != nil {
			paths = p
		}
	}

	r.options = &core.CompilerOptions{
		AllowJs:                    core.TSTrue,
		CheckJs:                    core.TSFalse,
		NoEmit:                     core.TSTrue,
		Jsx:                        core.JsxEmitPreserve,
		Target:                     core.ScriptTargetESNext,
		Module:                     core.ModuleKindESNext,
		ModuleResolution:           core.ModuleResolutionKindBundler,
		AllowImportingTsExtensions: core.TSTrue,
		Strict:                     core.TSTrue,
		Types:                      []string{},
		SkipLibCheck:               core.TSTrue,
		NoResolve:                  core.TSFalse,
	}
	if paths != nil {
		r.options.Paths = paths.paths
		r.options.BaseUrl = paths.baseURL
		r.options.PathsBasePath = paths.pathsBasePath
	}
	var roots []string
	for _, f := range sources {
		if v := r.vue[f]; v != nil {
			roots = append(roots, v.virtual)
		} else {
			roots = append(roots, r.abs(f))
		}
	}
	cfg := tsoptions.NewParsedCommandLine(r.options, roots, tspath.ComparePathsOptions{UseCaseSensitiveFileNames: true, CurrentDirectory: root})
	r.program = compiler.NewProgram(compiler.ProgramOptions{Host: host, Config: cfg, SingleThreaded: core.TSTrue})
	ctx := context.Background()
	c, done := r.program.GetTypeChecker(ctx)
	defer done()
	r.checker = c
	r.resolver = module.NewResolver(host, r.options, "", "")

	type listedFile struct {
		file   string
		errors int
	}
	var fileList []listedFile
	all := append(append([]string(nil), sources...), configs...)
	sort.Slice(all, func(i, j int) bool { return jsLess(all[i], all[j]) })
	for _, file := range all {
		if configNames[path.Base(file)] {
			f := configFacts[file]
			errs := 0
			if f != nil && f.has("error") {
				errs = 1
			}
			fileList = append(fileList, listedFile{file, errs})
			if f != nil {
				r.facts = append(r.facts, f)
			}
			continue
		}
		v := r.vue[file]
		name := r.abs(file)
		if v != nil {
			name = v.virtual
		}
		r.sf = r.program.GetSourceFile(name)
		if r.sf == nil {
			fileList = append(fileList, listedFile{file, 1})
			continue
		}
		errs := len(r.program.GetSyntacticDiagnostics(ctx, r.sf))
		if v != nil {
			errs += v.sfc.errors
		}
		fileList = append(fileList, listedFile{file, errs})
		r.visit(r.sf.AsNode())
		if v != nil {
			r.templateFacts(file, v.sfc)
		}
	}

	index := make([]int, len(r.facts))
	for i := range index {
		index[i] = i
	}
	sort.SliceStable(index, func(i, j int) bool {
		a, b := r.facts[index[i]], r.facts[index[j]]
		fa, fb := a.get("file").(string), b.get("file").(string)
		if fa != fb {
			return jsLess(fa, fb)
		}
		if la, lb := a.get("line").(int), b.get("line").(int); la != lb {
			return la < lb
		}
		if ca, cb := a.get("column").(int), b.get("column").(int); ca != cb {
			return ca < cb
		}
		return index[i] < index[j]
	})

	var out strings.Builder
	out.WriteString("{\n")
	out.WriteString("    \"codeFacts\": 1,\n")
	out.WriteString("    \"language\": \"javascript\",\n")
	fmt.Fprintf(&out, "    \"parser\": {\"name\": %s, \"version\": %s},\n", encoded(parserName), encoded(ParserVersion()))
	if len(r.vue) > 0 {
		fmt.Fprintf(&out, "    \"templateParser\": {\"name\": %s, \"version\": %s},\n", encoded(TemplateParserName), encoded(TemplateParserVersion))
	}
	fmt.Fprintf(&out, "    \"path\": %s,\n", encoded(folder))
	fmt.Fprintf(&out, "    \"commit\": %s,\n", encoded(commit))
	out.WriteString("    \"files\": [\n")
	var lines []string
	for _, f := range fileList {
		lines = append(lines, fmt.Sprintf("        {\"file\": %s, \"syntaxErrors\": %d}", encoded(f.file), f.errors))
	}
	out.WriteString(strings.Join(lines, ",\n"))
	out.WriteString("\n    ],\n")
	out.WriteString("    \"facts\": [\n")
	lines = lines[:0]
	for _, i := range index {
		lines = append(lines, "        "+encoded(r.facts[i]))
	}
	out.WriteString(strings.Join(lines, ",\n"))
	out.WriteString("\n    ]\n}\n")
	return []byte(out.String()), nil
}

func isSourceFile(f string) bool {
	for _, e := range sourceExtensions {
		if strings.HasSuffix(f, e) {
			return true
		}
	}
	return strings.HasSuffix(f, ".vue")
}

func (r *jsReader) abs(f string) string {
	return tspath.NormalizePath(filepath.ToSlash(filepath.Join(r.root, filepath.FromSlash(f))))
}

// rel is a file's path from the root; a Vue file's script is its .vue file.
func (r *jsReader) rel(f string) string {
	if v, ok := r.virtualToVue[f]; ok {
		return v
	}
	rel, err := filepath.Rel(r.root, filepath.FromSlash(f))
	if err != nil {
		return f
	}
	return filepath.ToSlash(rel)
}

func (r *jsReader) isLib(f string) bool { return bundled.IsBundled(f) }

func (r *jsReader) allowed(f string) bool { return r.isLib(f) || r.tracked[r.rel(f)] }

// configPaths are the module paths a configuration file gives.
type configPaths struct {
	paths         *collections.OrderedMap[string, []string]
	baseURL       string
	pathsBasePath string
}

func (r *jsReader) readConfig(file string, host compiler.CompilerHost) (*obj, *configPaths) {
	abs := r.abs(file)
	fact := newObj("kind", "configuration", "file", file, "line", 1, "column", 1)
	text, ok := host.FS().ReadFile(abs)
	if !ok {
		fact.set("error", fmt.Sprintf("Cannot read file '%s'.", abs))
		return fact, nil
	}
	json, diags := tsoptions.ParseConfigFileTextToJson(abs, tspath.Path(abs), text)
	if len(diags) > 0 {
		fact.set("error", flatten(diags[0]))
		return fact, nil
	}
	parsed := tsoptions.ParseJsonConfigFileContent(json, host, path.Dir(abs), nil, abs, nil, nil, nil)
	o := parsed.CompilerOptions()
	fact.set("checkJs", o.CheckJs == core.TSTrue)
	fact.set("allowJs", o.AllowJs == core.TSTrue)
	fact.set("strict", o.Strict == core.TSTrue)
	if m, ok := json.(interface{ Get(string) (any, bool) }); ok {
		if ext, ok := m.Get("extends"); ok && ext != nil {
			var list []any
			switch e := ext.(type) {
			case []any:
				for _, x := range e {
					list = append(list, jsStringOf(x))
				}
			default:
				list = append(list, jsStringOf(e))
			}
			fact.set("extends", list)
			read := true
			for _, d := range parsed.Errors {
				if d.Code() == 6053 || d.Code() == 5083 {
					read = false
				}
			}
			fact.set("extendsRead", read)
		}
	}
	var p *configPaths
	if o.Paths != nil || o.BaseUrl != "" {
		p = &configPaths{paths: o.Paths, baseURL: o.BaseUrl, pathsBasePath: o.PathsBasePath}
	}
	return fact, p
}

func jsStringOf(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(x)
	}
}

// flatten is a diagnostic's message, its chain joined by spaces.
func flatten(d *ast.Diagnostic) string {
	return d.String()
}

// pos is where a node starts, its leading trivia skipped.
func (r *jsReader) start(n *ast.Node, sf *ast.SourceFile) int {
	return scanner.GetTokenPosOfNode(n, sf, false)
}

func lineColumn(sf *ast.SourceFile, pos int) (int, int) {
	l, c := scanner.GetECMALineAndUTF16CharacterOfPosition(sf, pos)
	return l + 1, int(c) + 1
}

func (r *jsReader) where(n *ast.Node) (int, int) {
	return lineColumn(r.sf, r.start(n, r.sf))
}

func (r *jsReader) at(n *ast.Node) *obj {
	file := ast.GetSourceFileOfNode(n)
	l, c := lineColumn(file, r.start(n, file))
	return newObj("file", r.rel(file.FileName()), "line", l, "column", c)
}

// nodeText is a node's source text.
func (r *jsReader) nodeText(n *ast.Node) string {
	file := ast.GetSourceFileOfNode(n)
	if file == nil {
		file = r.sf
	}
	return file.Text()[r.start(n, file):n.End()]
}

// text is a node's source text with each run of white space one space,
// cut to 160 characters.
func (r *jsReader) text(n *ast.Node) string {
	return cut(collapse(r.nodeText(n)))
}

func cut(s string) string {
	u := units(s)
	if len(u) > textLimit {
		return fromUnits(u[:textLimit-3]) + "..."
	}
	return s
}

// collapse makes each run of white space, as JavaScript's \s matches it,
// one space.
func collapse(s string) string {
	var b strings.Builder
	space := false
	for _, c := range s {
		if isJSSpace(c) {
			if !space {
				b.WriteByte(' ')
			}
			space = true
			continue
		}
		space = false
		b.WriteRune(c)
	}
	return b.String()
}

func isJSSpace(c rune) bool {
	switch c {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return c >= 0x2000 && c <= 0x200A
}

func (r *jsReader) emit(kind string, n *ast.Node, fields *obj) {
	l, c := r.where(n)
	f := newObj("kind", kind, "file", r.rel(r.sf.FileName()), "line", l, "column", c)
	f.merge(fields)
	r.facts = append(r.facts, f)
}

func reparsed(n *ast.Node) bool { return n != nil && n.Flags&ast.NodeFlagsReparsed != 0 }

func isFunctionLike(n *ast.Node) bool {
	return n != nil && (ast.IsFunctionDeclaration(n) || ast.IsFunctionExpression(n) || ast.IsArrowFunction(n) || ast.IsMethodDeclaration(n))
}

func nodes(l *ast.NodeList) []*ast.Node {
	if l == nil {
		return nil
	}
	return l.Nodes
}

// declarationNode is the node a declaration is cited by: a variable given
// a function is cited by the function, so that a name and the function
// fact meet.
func declarationNode(d *ast.Node) *ast.Node {
	if ast.IsVariableDeclaration(d) && d.Initializer() != nil {
		init := unwrap(d.Initializer())
		if ast.IsArrowFunction(init) || ast.IsFunctionExpression(init) {
			return init
		}
	}
	if ast.IsExportAssignment(d) {
		return unwrap(d.Expression())
	}
	return d
}

// importOf is the module and the name an import binds a name to, by its
// syntax.
func (r *jsReader) importOf(d *ast.Node) *obj {
	switch {
	case ast.IsImportSpecifier(d):
		spec := d.AsImportSpecifier()
		name := spec.PropertyName
		if name == nil {
			name = spec.Name()
		}
		return newObj("module", d.Parent.Parent.Parent.ModuleSpecifier().Text(), "imported", name.Text())
	case ast.IsImportClause(d):
		return newObj("module", d.Parent.ModuleSpecifier().Text(), "imported", "default")
	case ast.IsNamespaceImport(d):
		return newObj("module", d.Parent.Parent.ModuleSpecifier().Text(), "imported", "*")
	case ast.IsImportEqualsDeclaration(d):
		ref := d.AsImportEqualsDeclaration().ModuleReference
		if ast.IsExternalModuleReference(ref) && ast.IsStringLiteral(ref.Expression()) {
			return newObj("module", ref.Expression().Text(), "imported", "*")
		}
	}
	// CommonJS: const x = require('m'), and const { a } = require('m').
	if ast.IsVariableDeclaration(d) && d.Initializer() != nil {
		if m, ok := requiredModule(d.Initializer()); ok && ast.IsIdentifier(d.Name()) {
			return newObj("module", m, "imported", "*")
		}
	}
	if ast.IsBindingElement(d) && ast.IsObjectBindingPattern(d.Parent) && ast.IsVariableDeclaration(d.Parent.Parent) && d.Parent.Parent.Initializer() != nil {
		if m, ok := requiredModule(d.Parent.Parent.Initializer()); ok {
			name := d.PropertyName()
			if name == nil {
				name = d.Name()
			}
			return newObj("module", m, "imported", r.nodeText(name))
		}
	}
	return nil
}

func requiredModule(node *ast.Node) (string, bool) {
	n := unwrap(node)
	if ast.IsCallExpression(n) && ast.IsIdentifier(n.Expression()) && n.Expression().Text() == "require" {
		args := n.Arguments()
		if len(args) == 1 && ast.IsStringLiteralLike(args[0]) {
			return args[0].Text(), true
		}
	}
	return "", false
}

// resolveName is what a name resolves to: the import that binds it, and
// the declaration among the files read, or the library of TypeScript,
// that declares it.
func (r *jsReader) resolveName(id *ast.Node) *obj {
	out := newObj()
	sym := r.checker.GetSymbolAtLocation(id)
	if sym == nil && id.Parent != nil && ast.IsShorthandPropertyAssignment(id.Parent) {
		sym = r.checker.GetShorthandAssignmentValueSymbol(id.Parent)
	}
	if sym == nil {
		return out
	}
	var first *ast.Node
	if len(sym.Declarations) > 0 {
		first = sym.Declarations[0]
		if imp := r.importOf(first); imp != nil {
			out.set("import", imp)
		}
	}
	target := sym
	if sym.Flags&ast.SymbolFlagsAlias != 0 {
		target = r.aliased(sym)
	}
	if target != nil && len(target.Declarations) > 0 {
		d := target.Declarations[0]
		if target != sym {
			d = r.throughShorthand(d)
		}
		file := ast.GetSourceFileOfNode(d)
		if file != nil {
			if r.isLib(file.FileName()) {
				out.set("library", true)
			} else if r.tracked[r.rel(file.FileName())] {
				if !(out.has("import") && d == first && r.importOf(d) != nil) {
					out.set("declaration", r.at(declarationNode(d)))
				}
				if ast.IsParameterDeclaration(d) && isFunctionLike(d.Parent) {
					index := -1
					for i, p := range d.Parent.Parameters() {
						if p == d {
							index = i
						}
					}
					out.set("parameter", newObj("index", index, "function", r.at(d.Parent)))
				}
			}
		}
	}
	return out
}

// throughShorthand follows a declaration that is a shorthand property,
// such as allow in module.exports = { allow }, to the declaration of the
// value it holds. TypeScript 7 resolves a name a CommonJS module exports
// so to the property, where 6.0.3 resolved it to the value's declaration;
// the dump keeps 6.0.3's answer, the place the value is declared.
func (r *jsReader) throughShorthand(d *ast.Node) *ast.Node {
	for i := 0; i < 8 && d != nil && ast.IsShorthandPropertyAssignment(d); i++ {
		sym := r.checker.GetShorthandAssignmentValueSymbol(d)
		if sym == nil || len(sym.Declarations) == 0 || sym.Declarations[0] == d {
			break
		}
		d = sym.Declarations[0]
	}
	return d
}

func (r *jsReader) aliased(sym *ast.Symbol) (target *ast.Symbol) {
	defer func() {
		if recover() != nil {
			target = nil
		}
	}()
	return r.checker.GetAliasedSymbol(sym)
}

func unwrap(node *ast.Node) *ast.Node {
	n := node
	for {
		switch n.Kind {
		case ast.KindParenthesizedExpression, ast.KindAsExpression, ast.KindSatisfiesExpression, ast.KindNonNullExpression, ast.KindTypeAssertionExpression, ast.KindAwaitExpression:
			n = n.Expression()
		default:
			return n
		}
	}
}

func propertyName(name *ast.Node) (string, bool) {
	if ast.IsIdentifier(name) || ast.IsPrivateIdentifier(name) || ast.IsStringLiteralLike(name) || ast.IsNumericLiteral(name) {
		return name.Text(), true
	}
	return "", false
}

func withName(name string, more *obj) *obj {
	o := newObj("name", name)
	o.merge(more)
	return o
}

// value writes an expression as one of its keys: a literal, a template
// by its parts, a name with what it resolves to, a member, a call, an
// object, an array, a function by where it is, an element, or its text.
func (r *jsReader) value(node *ast.Node, d int) *obj {
	n := unwrap(node)
	if d > depth {
		return newObj("text", r.text(n))
	}
	switch n.Kind {
	case ast.KindStringLiteral, ast.KindNoSubstitutionTemplateLiteral:
		return newObj("string", n.Text())
	case ast.KindTemplateExpression:
		t := n.AsTemplateExpression()
		parts := []any{}
		if t.Head.Text() != "" {
			parts = append(parts, newObj("text", t.Head.Text()))
		}
		for _, s := range nodes(t.TemplateSpans) {
			span := s.AsTemplateSpan()
			parts = append(parts, newObj("expression", r.value(span.Expression, d+1)))
			if span.Literal.Text() != "" {
				parts = append(parts, newObj("text", span.Literal.Text()))
			}
		}
		return newObj("template", parts)
	case ast.KindNumericLiteral:
		return newObj("number", n.Text())
	case ast.KindTrueKeyword:
		return newObj("boolean", true)
	case ast.KindFalseKeyword:
		return newObj("boolean", false)
	case ast.KindNullKeyword:
		return newObj("null", true)
	case ast.KindRegularExpressionLiteral:
		return newObj("regex", n.Text())
	case ast.KindIdentifier:
		return withName(n.Text(), r.resolveName(n))
	case ast.KindMetaProperty:
		return newObj("name", r.text(n))
	case ast.KindPropertyAccessExpression:
		return newObj("member", newObj("object", r.value(n.Expression(), d+1), "name", n.Name().Text()))
	case ast.KindElementAccessExpression:
		arg := n.AsElementAccessExpression().ArgumentExpression
		if ast.IsStringLiteralLike(arg) || ast.IsNumericLiteral(arg) {
			return newObj("member", newObj("object", r.value(n.Expression(), d+1), "name", arg.Text(), "computed", true))
		}
		return newObj("text", r.text(n))
	case ast.KindCallExpression:
		return newObj("call", r.callOf(n, d))
	case ast.KindNewExpression:
		return newObj("new", r.callOf(n, d))
	case ast.KindObjectLiteralExpression:
		props := []any{}
		for _, p := range n.Properties() {
			props = append(props, r.property(p, d))
		}
		return newObj("object", props)
	case ast.KindArrayLiteralExpression:
		els := []any{}
		for _, e := range n.Elements() {
			if ast.IsSpreadElement(e) {
				els = append(els, newObj("spread", r.value(e.Expression(), d+1)))
			} else {
				els = append(els, r.value(e, d+1))
			}
		}
		return newObj("array", els)
	case ast.KindArrowFunction, ast.KindFunctionExpression:
		return newObj("function", r.at(n))
	case ast.KindJsxElement, ast.KindJsxSelfClosingElement:
		return newObj("jsx", r.element(n, d))
	case ast.KindPrefixUnaryExpression:
		u := n.AsPrefixUnaryExpression()
		if u.Operator == ast.KindMinusToken && ast.IsNumericLiteral(u.Operand) {
			return newObj("number", "-"+u.Operand.Text())
		}
		return newObj("text", r.text(n))
	case ast.KindUndefinedKeyword:
		return newObj("name", "undefined")
	}
	return newObj("text", r.text(n))
}

func (r *jsReader) property(p *ast.Node, d int) *obj {
	switch {
	case ast.IsPropertyAssignment(p):
		line, _ := r.where(p)
		if key, ok := propertyName(p.Name()); ok {
			return newObj("key", key, "line", line, "value", r.value(p.Initializer(), d+1))
		}
		return newObj("computed", r.text(p.Name()), "line", line, "value", r.value(p.Initializer(), d+1))
	case ast.IsShorthandPropertyAssignment(p):
		line, _ := r.where(p)
		return newObj("key", p.Name().Text(), "line", line, "value", withName(p.Name().Text(), r.resolveName(p.Name())))
	case ast.IsSpreadAssignment(p):
		return newObj("spread", r.value(p.Expression(), d+1))
	case ast.IsMethodDeclaration(p):
		if key, ok := propertyName(p.Name()); ok {
			return newObj("key", key, "value", newObj("function", r.at(p)))
		}
		return newObj("computed", r.text(p.Name()), "value", newObj("function", r.at(p)))
	}
	return newObj("computed", r.text(p))
}

func (r *jsReader) callOf(n *ast.Node, d int) *obj {
	args := []any{}
	for _, a := range n.Arguments() {
		if ast.IsSpreadElement(a) {
			args = append(args, newObj("spread", r.value(a.Expression(), d+1)))
		} else {
			args = append(args, r.value(a, d+1))
		}
	}
	c := newObj("callee", r.value(n.Expression(), d+1), "arguments", args)
	if ta := n.TypeArguments(); len(ta) > 0 {
		var types []any
		for _, t := range ta {
			types = append(types, r.typeOf(t))
		}
		c.set("typeArguments", types)
	}
	return c
}

func (r *jsReader) tagOf(tag *ast.Node) *obj {
	if ast.IsIdentifier(tag) && lowerStart.MatchString(tag.Text()) && !strings.Contains(tag.Text(), ".") {
		return newObj("intrinsic", tag.Text())
	}
	return r.value(tag, 0)
}

func opening(n *ast.Node) *ast.Node {
	if ast.IsJsxElement(n) {
		return n.AsJsxElement().OpeningElement
	}
	return n
}

// element writes a JSX element: its tag, its attributes, and the literal
// text it holds.
func (r *jsReader) element(n *ast.Node, d int) *obj {
	o := opening(n)
	attrs := []any{}
	for _, a := range o.Attributes().Properties() {
		if ast.IsJsxSpreadAttribute(a) {
			attrs = append(attrs, newObj("spread", r.value(a.Expression(), d+1)))
			continue
		}
		name := r.nodeText(a.Name())
		init := a.Initializer()
		switch {
		case init == nil:
			attrs = append(attrs, newObj("name", name, "value", newObj("boolean", true)))
		case ast.IsStringLiteral(init):
			attrs = append(attrs, newObj("name", name, "value", newObj("string", init.Text())))
		case ast.IsJsxExpression(init) && init.Expression() != nil:
			attrs = append(attrs, newObj("name", name, "value", r.value(init.Expression(), d+1)))
		case ast.IsJsxElement(init) || ast.IsJsxSelfClosingElement(init):
			attrs = append(attrs, newObj("name", name, "value", newObj("jsx", r.element(init, d+1))))
		}
	}
	e := newObj("tag", r.tagOf(o.TagName()), "attributes", attrs)
	if ast.IsJsxElement(n) {
		parts := []any{}
		for _, c := range n.Children().Nodes {
			if ast.IsJsxText(c) {
				if t := strings.TrimFunc(collapse(c.AsJsxText().Text), isJSSpace); t != "" {
					parts = append(parts, newObj("text", t))
				}
			} else if ast.IsJsxExpression(c) && c.Expression() != nil {
				parts = append(parts, newObj("expression", r.value(c.Expression(), d+1)))
			}
		}
		if len(parts) > 0 {
			e.set("children", parts)
		}
	}
	return e
}

// typeOf writes a type as its source states it, with the declaration
// among the files read that a name refers to.
func (r *jsReader) typeOf(node *ast.Node) any {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case ast.KindStringKeyword:
		return newObj("keyword", "string")
	case ast.KindNumberKeyword:
		return newObj("keyword", "number")
	case ast.KindBooleanKeyword:
		return newObj("keyword", "boolean")
	case ast.KindBigIntKeyword:
		return newObj("keyword", "bigint")
	case ast.KindAnyKeyword:
		return newObj("keyword", "any")
	case ast.KindUnknownKeyword:
		return newObj("keyword", "unknown")
	case ast.KindVoidKeyword:
		return newObj("keyword", "void")
	case ast.KindUndefinedKeyword:
		return newObj("keyword", "undefined")
	case ast.KindNeverKeyword:
		return newObj("keyword", "never")
	case ast.KindObjectKeyword:
		return newObj("keyword", "object")
	case ast.KindJSDocAllType:
		return newObj("keyword", "any")
	case ast.KindLiteralType:
		l := node.AsLiteralTypeNode().Literal
		switch {
		case l.Kind == ast.KindNullKeyword:
			return newObj("keyword", "null")
		case ast.IsStringLiteral(l):
			return newObj("literal", newObj("string", l.Text()))
		case ast.IsNumericLiteral(l):
			return newObj("literal", newObj("number", l.Text()))
		case l.Kind == ast.KindTrueKeyword:
			return newObj("literal", newObj("boolean", true))
		case l.Kind == ast.KindFalseKeyword:
			return newObj("literal", newObj("boolean", false))
		}
		return newObj("text", r.text(node))
	case ast.KindParenthesizedType, ast.KindJSDocTypeExpression, ast.KindJSDocNonNullableType:
		return r.typeOf(node.Type())
	case ast.KindJSDocNullableType:
		return newObj("union", []any{r.typeOf(node.Type()), newObj("keyword", "null")})
	case ast.KindJSDocOptionalType:
		return newObj("union", []any{r.typeOf(node.Type()), newObj("keyword", "undefined")})
	case ast.KindArrayType:
		return newObj("array", r.typeOf(node.AsArrayTypeNode().ElementType))
	case ast.KindUnionType:
		var types []any
		for _, t := range nodes(node.AsUnionTypeNode().Types) {
			types = append(types, r.typeOf(t))
		}
		return newObj("union", types)
	case ast.KindTypeLiteral:
		members := []any{}
		for _, m := range node.Members() {
			if ast.IsPropertySignatureDeclaration(m) && !reparsed(m) {
				members = append(members, r.member(m))
			}
		}
		return newObj("object", members)
	case ast.KindJSDocTypeLiteral:
		props := []any{}
		for _, t := range node.AsJSDocTypeLiteral().JSDocPropertyTags {
			props = append(props, r.jsDocProperty(t))
		}
		return newObj("object", props)
	case ast.KindTypeQuery:
		name := node.AsTypeQueryNode().ExprName
		t := newObj("typeof", r.nodeText(name))
		id := name
		if ast.IsQualifiedName(name) {
			id = name.AsQualifiedName().Right
		}
		t.merge(r.resolveName(id))
		t.del("parameter")
		return t
	case ast.KindTypeReference:
		ref := node.AsTypeReferenceNode()
		t := newObj("reference", r.nodeText(ref.TypeName))
		id := ref.TypeName
		for ast.IsQualifiedName(id) {
			id = id.AsQualifiedName().Left
		}
		res := r.resolveName(id)
		if ast.IsQualifiedName(ref.TypeName) {
			if res.has("import") {
				t.set("import", res.get("import"))
			}
		} else {
			t.merge(res)
			t.del("parameter")
		}
		if ta := node.TypeArguments(); len(ta) > 0 {
			var types []any
			for _, a := range ta {
				types = append(types, r.typeOf(a))
			}
			t.set("arguments", types)
		}
		return t
	case ast.KindImportType:
		it := node.AsImportTypeNode()
		reference := ""
		if it.Qualifier != nil {
			reference = r.nodeText(it.Qualifier)
		}
		var importType string
		if ast.IsLiteralTypeNode(it.Argument) {
			importType = it.Argument.AsLiteralTypeNode().Literal.Text()
		} else {
			importType = r.text(it.Argument)
		}
		t := newObj("reference", reference, "importType", importType)
		if ty := r.typeFromNode(node); ty != nil {
			var sym *ast.Symbol
			if a := ty.Alias(); a != nil {
				sym = a.Symbol()
			}
			if sym == nil {
				sym = ty.Symbol()
			}
			if sym != nil && len(sym.Declarations) > 0 {
				d := sym.Declarations[0]
				if file := ast.GetSourceFileOfNode(d); file != nil && r.tracked[r.rel(file.FileName())] {
					t.set("declaration", r.at(declarationNode(d)))
				}
			}
		}
		if ta := node.TypeArguments(); len(ta) > 0 {
			var types []any
			for _, a := range ta {
				types = append(types, r.typeOf(a))
			}
			t.set("arguments", types)
		}
		return t
	}
	return newObj("text", r.text(node))
}

func (r *jsReader) typeFromNode(n *ast.Node) (t *checker.Type) {
	defer func() {
		if recover() != nil {
			t = nil
		}
	}()
	return r.checker.GetTypeFromTypeNode(n)
}

func (r *jsReader) member(m *ast.Node) *obj {
	name, ok := propertyName(m.Name())
	if !ok {
		name = r.text(m.Name())
	}
	l, c := r.where(m)
	out := newObj("name", name, "line", l, "column", c)
	if t := m.Type(); t != nil && !reparsed(t) {
		out.set("type", r.typeOf(t))
	}
	if q := m.PostfixToken(); q != nil && q.Kind == ast.KindQuestionToken && !reparsed(q) {
		out.set("optional", true)
	}
	for _, mod := range modifiers(m) {
		if mod.Kind == ast.KindReadonlyKeyword {
			out.set("readonly", true)
		}
	}
	return out
}

func (r *jsReader) jsDocProperty(tag *ast.Node) *obj {
	p := tag.AsJSDocParameterOrPropertyTag()
	l, c := r.where(tag)
	out := newObj("name", r.nodeText(tag.Name()), "line", l, "column", c)
	if p.TypeExpression != nil {
		t := p.TypeExpression.Type()
		if t != nil && t.Kind == ast.KindJSDocOptionalType {
			out.set("type", r.typeOf(t.Type()))
			out.set("optional", true)
		} else {
			out.set("type", r.typeOf(p.TypeExpression))
		}
	}
	if p.IsBracketed {
		out.set("optional", true)
	}
	return out
}

// modifiers are a node's modifiers as its source writes them, not those
// the compiler adds from its JSDoc.
func modifiers(n *ast.Node) []*ast.Node {
	ml := n.Modifiers()
	if ml == nil {
		return nil
	}
	var out []*ast.Node
	for _, m := range ml.Nodes {
		if !reparsed(m) {
			out = append(out, m)
		}
	}
	return out
}

func (r *jsReader) decorators(n *ast.Node) []any {
	out := []any{}
	for _, m := range modifiers(n) {
		if !ast.IsDecorator(m) {
			continue
		}
		e := unwrap(m.Expression())
		if ast.IsCallExpression(e) {
			args := []any{}
			for _, a := range e.Arguments() {
				args = append(args, r.value(a, 0))
			}
			out = append(out, newObj("name", r.text(e.Expression()), "arguments", args))
		} else {
			out = append(out, newObj("name", r.text(e)))
		}
	}
	return out
}

func isExported(n *ast.Node) string {
	exported, def := false, false
	for _, m := range modifiers(n) {
		switch m.Kind {
		case ast.KindExportKeyword:
			exported = true
		case ast.KindDefaultKeyword:
			def = true
		}
	}
	if !exported {
		return ""
	}
	if def {
		return "default"
	}
	return "named"
}

// jsDocs are the JSDoc comments the parser attached to a node.
func (r *jsReader) jsDocs(n *ast.Node) []*ast.Node {
	if n == nil || n.Flags&ast.NodeFlagsHasJSDoc == 0 {
		return nil
	}
	var out []*ast.Node
	for _, d := range n.JSDoc(r.sf) {
		if !reparsed(d) {
			out = append(out, d)
		}
	}
	return out
}

func (r *jsReader) jsDocText(n *ast.Node) (string, bool) {
	docs := r.jsDocs(n)
	if len(docs) == 0 {
		return "", false
	}
	var parts []string
	for _, d := range docs {
		parts = append(parts, r.nodeText(d))
	}
	return strings.Join(parts, "\n"), true
}

// jsDocTags are the JSDoc tags that apply to a node: its own, and for a
// variable, a function given to one or a parameter, those of the
// statement or the declaration they are written on.
func (r *jsReader) jsDocTags(n *ast.Node) []*ast.Node {
	var hosts []*ast.Node
	for h := n; h != nil; h = h.Parent {
		hosts = append(hosts, h)
		if ast.IsVariableDeclaration(h) {
			if h.Parent != nil && h.Parent.Parent != nil && ast.IsVariableStatement(h.Parent.Parent) {
				hosts = append(hosts, h.Parent.Parent)
			}
			break
		}
		if !(ast.IsParameterDeclaration(h) || ast.IsArrowFunction(h) || ast.IsFunctionExpression(h) || ast.IsParenthesizedExpression(h)) {
			break
		}
		if ast.IsParameterDeclaration(h) || ast.IsParenthesizedExpression(h) {
			continue
		}
		if p := h.Parent; p == nil || !(ast.IsVariableDeclaration(p) || ast.IsPropertyAssignment(p) || ast.IsParenthesizedExpression(p)) {
			break
		}
	}
	var tags []*ast.Node
	for i := len(hosts) - 1; i >= 0; i-- {
		for _, d := range r.jsDocs(hosts[i]) {
			if t := d.AsJSDoc().Tags; t != nil {
				tags = append(tags, t.Nodes...)
			}
		}
	}
	return tags
}

// jsDocType is the type a JSDoc tag gives a declaration or a parameter:
// its @type, or a parameter's @param.
func (r *jsReader) jsDocType(n *ast.Node) *ast.Node {
	tags := r.jsDocTags(n)
	for _, t := range tags {
		if t.Kind == ast.KindJSDocTypeTag && t.TypeExpression() != nil && r.ownTag(t, n) {
			return t.TypeExpression().Type()
		}
	}
	if ast.IsParameterDeclaration(n) && ast.IsIdentifier(n.Name()) {
		for _, t := range r.jsDocTags(n.Parent) {
			if t.Kind != ast.KindJSDocParameterTag {
				continue
			}
			p := t.AsJSDocParameterOrPropertyTag()
			if ast.IsIdentifier(p.Name()) && p.Name().Text() == n.Name().Text() && p.TypeExpression != nil {
				return p.TypeExpression.Type()
			}
		}
	}
	return nil
}

// ownTag reports whether a @type tag found for n is n's own: a parameter
// takes only one written on itself.
func (r *jsReader) ownTag(tag, n *ast.Node) bool {
	if !ast.IsParameterDeclaration(n) {
		return true
	}
	for _, d := range r.jsDocs(n) {
		if t := d.AsJSDoc().Tags; t != nil {
			for _, x := range t.Nodes {
				if x == tag {
					return true
				}
			}
		}
	}
	return false
}

// context is the function a node is in, and whether it is in a loop or
// behind a condition there; a function given to an array's map or forEach
// runs in a loop.
func (r *jsReader) context(node *ast.Node) *obj {
	out := newObj()
	loop, condition := false, false
	prev := node
	for n := node.Parent; n != nil && !ast.IsSourceFile(n); prev, n = n, n.Parent {
		if isFunctionLike(n) {
			out.set("within", r.at(n))
			if call := n.Parent; call != nil && ast.IsCallExpression(call) && ast.IsPropertyAccessExpression(call.Expression()) && loopCalls[call.Expression().Name().Text()] {
				for _, a := range call.Arguments() {
					if a == n {
						loop = true
					}
				}
			}
			break
		}
		switch n.Kind {
		case ast.KindForStatement, ast.KindForInStatement, ast.KindForOfStatement, ast.KindWhileStatement, ast.KindDoStatement:
			loop = true
		case ast.KindIfStatement:
			s := n.AsIfStatement()
			if prev != s.Expression {
				condition = true
			}
			// if (req.method === 'POST') { ... }: the branch of one value.
			if !out.has("branch") && prev == s.ThenStatement && ast.IsBinaryExpression(s.Expression) {
				b := s.Expression.AsBinaryExpression()
				if op := b.OperatorToken.Kind; op == ast.KindEqualsEqualsEqualsToken || op == ast.KindEqualsEqualsToken {
					out.set("branch", newObj("on", r.text(b.Left), "value", r.value(b.Right, 0)))
				}
			}
		case ast.KindConditionalExpression:
			if prev != n.AsConditionalExpression().Condition {
				condition = true
			}
		case ast.KindSwitchStatement:
			condition = true
		case ast.KindCaseClause:
			condition = true
			// case 'GET': the branch of one value of the switch.
			if !out.has("branch") {
				out.set("branch", newObj("on", r.text(n.Parent.Parent.Expression()), "value", r.value(n.Expression(), 0)))
			}
		case ast.KindBinaryExpression:
			b := n.AsBinaryExpression()
			if op := b.OperatorToken.Kind; prev == b.Right && (op == ast.KindAmpersandAmpersandToken || op == ast.KindBarBarToken || op == ast.KindQuestionQuestionToken) {
				condition = true
			}
		}
	}
	if loop {
		out.set("inLoop", true)
	}
	if condition {
		out.set("inCondition", true)
	}
	return out
}

// calleeRoot is the root of a callee: the name a chain of members and
// calls starts at.
func calleeRoot(node *ast.Node) *ast.Node {
	n := unwrap(node)
	for {
		switch {
		case ast.IsPropertyAccessExpression(n), ast.IsElementAccessExpression(n), ast.IsCallExpression(n):
			n = unwrap(n.Expression())
		default:
			return n
		}
	}
}

func calleeName(node *ast.Node) (string, bool) {
	n := unwrap(node)
	if ast.IsIdentifier(n) {
		return n.Text(), true
	}
	if ast.IsPropertyAccessExpression(n) {
		return n.Name().Text(), true
	}
	return "", false
}

func knownModule(m string) bool {
	for _, k := range knownModules {
		if m == k || strings.HasPrefix(m, k+"/") {
			return true
		}
	}
	return false
}

// importedFrom is the module a name is imported from, or "".
func (r *jsReader) importedFrom(id *ast.Node) (string, bool) {
	if !ast.IsIdentifier(id) {
		return "", false
	}
	if imp, ok := r.resolveName(id).get("import").(*obj); ok {
		return imp.get("module").(string), true
	}
	return "", false
}

func (r *jsReader) wantedCall(n *ast.Node) bool {
	// Only the outermost call of a chain is written; the calls it is made
	// on are in its callee.
	if p := n.Parent; p != nil && ast.IsPropertyAccessExpression(p) && p.Expression() == n && p.Parent != nil && ast.IsCallExpression(p.Parent) && p.Parent.Expression() == p {
		return false
	}
	if name, ok := calleeName(n.Expression()); ok && callNames[name] {
		return true
	}
	if root := calleeRoot(n.Expression()); ast.IsIdentifier(root) {
		if _, ok := r.importedFrom(root); ok {
			return true
		}
	}
	return false
}

// assignedTo is the name a call's value is given to, when it is the
// initial value of a variable.
func assignedTo(node *ast.Node) (string, bool) {
	n := node
	for n.Parent != nil {
		switch n.Parent.Kind {
		case ast.KindParenthesizedExpression, ast.KindAwaitExpression, ast.KindAsExpression, ast.KindSatisfiesExpression, ast.KindNonNullExpression:
			n = n.Parent
			continue
		}
		break
	}
	if p := n.Parent; p != nil && ast.IsVariableDeclaration(p) && p.Initializer() == n && ast.IsIdentifier(p.Name()) {
		return p.Name().Text(), true
	}
	return "", false
}

// readAccess writes a read of the environment or of a request's members,
// a chain of names from its root.
func (r *jsReader) readAccess(node *ast.Node) *obj {
	var chain []any
	n := node
	for {
		if ast.IsPropertyAccessExpression(n) {
			chain = append([]any{n.Name().Text()}, chain...)
			n = n.Expression()
		} else if ast.IsElementAccessExpression(n) && ast.IsStringLiteralLike(n.AsElementAccessExpression().ArgumentExpression) {
			chain = append([]any{n.AsElementAccessExpression().ArgumentExpression.Text()}, chain...)
			n = n.Expression()
		} else if ast.IsElementAccessExpression(n) {
			chain = append([]any{nil}, chain...)
			n = n.Expression()
		} else {
			break
		}
	}
	var rootName string
	var parameter any
	switch {
	case ast.IsMetaProperty(n):
		rootName = "import.meta"
	case ast.IsIdentifier(n):
		rootName = n.Text()
		res := r.resolveName(n)
		parameter = res.get("parameter")
		if rootName == "process" && (res.has("declaration") || res.has("parameter")) {
			return nil
		}
	default:
		return nil
	}
	first, _ := firstOf(chain).(string)
	env := (rootName == "process" || rootName == "import.meta") && first == "env" && len(chain) > 0 && chain[0] != nil
	req := parameter != nil && len(chain) > 0 && chain[0] != nil && requestMembers[first]
	if !env && !req {
		return nil
	}
	full := []any{rootName}
	for _, c := range chain {
		if c == nil {
			full = append(full, "[computed]")
		} else {
			full = append(full, c)
		}
	}
	fact := newObj("chain", full)
	if parameter != nil {
		fact.set("parameter", parameter)
	}
	return fact
}

func firstOf(l []any) any {
	if len(l) == 0 {
		return nil
	}
	return l[0]
}

// directives is the prologue of a file or a function body: 'use server'.
func directives(statements []*ast.Node) []string {
	var out []string
	for _, st := range statements {
		if ast.IsExpressionStatement(st) && ast.IsStringLiteral(st.Expression()) {
			out = append(out, st.Expression().Text())
		} else {
			break
		}
	}
	return out
}

// returnValues is what a function returns: its expression body, or the
// expression of each return statement in it, not in a function inside it.
func returnValues(fn *ast.Node) []*ast.Node {
	body := fn.Body()
	if body == nil {
		return nil
	}
	if !ast.IsBlock(body) {
		return []*ast.Node{body}
	}
	var out []*ast.Node
	var walk func(n *ast.Node) bool
	walk = func(n *ast.Node) bool {
		if reparsed(n) {
			return false
		}
		if n != fn && isFunctionLike(n) {
			return false
		}
		if ast.IsReturnStatement(n) && n.Expression() != nil {
			out = append(out, n.Expression())
		}
		n.ForEachChild(walk)
		return false
	}
	body.ForEachChild(walk)
	return out
}

func (r *jsReader) accessDetails(node *ast.Node, fact *obj) {
	n := node
	for n.Parent != nil && (ast.IsParenthesizedExpression(n.Parent) || ast.IsAsExpression(n.Parent) || ast.IsNonNullExpression(n.Parent)) {
		n = n.Parent
	}
	p := n.Parent
	if p != nil && ast.IsBinaryExpression(p) && p.AsBinaryExpression().Left == n {
		b := p.AsBinaryExpression()
		switch b.OperatorToken.Kind {
		case ast.KindQuestionQuestionToken, ast.KindBarBarToken:
			fact.set("default", r.value(b.Right, 0))
		case ast.KindEqualsEqualsEqualsToken, ast.KindExclamationEqualsEqualsToken, ast.KindEqualsEqualsToken, ast.KindExclamationEqualsToken:
			fact.set("compared", r.value(b.Right, 0))
		}
	}
	if p != nil && ast.IsSwitchStatement(p) && p.Expression() == n {
		cases := []any{}
		hasDefault := false
		for _, c := range p.AsSwitchStatement().CaseBlock.AsCaseBlock().Clauses.Nodes {
			if c.Kind == ast.KindCaseClause {
				cases = append(cases, r.value(c.Expression(), 0))
			} else {
				hasDefault = true
			}
		}
		fact.set("cases", cases)
		if hasDefault {
			fact.set("hasDefault", true)
		}
	}
	// The call it is given to, through a default: Number(process.env.PORT ?? '8080').
	w, q := n, p
	if fact.has("default") && q != nil {
		w = q
		for w.Parent != nil && ast.IsParenthesizedExpression(w.Parent) {
			w = w.Parent
		}
		q = w.Parent
	}
	if q != nil && ast.IsCallExpression(q) {
		if args := q.Arguments(); len(args) > 0 && args[0] == w {
			fact.set("wrappedBy", r.text(q.Expression()))
		}
	}
}

func isAccessTop(node *ast.Node) bool {
	p := node.Parent
	if p == nil {
		return true
	}
	if (ast.IsPropertyAccessExpression(p) || ast.IsElementAccessExpression(p)) && p.Expression() == node {
		return false
	}
	return true
}

func functionName(n *ast.Node) (string, bool) {
	if (ast.IsFunctionDeclaration(n) || ast.IsFunctionExpression(n)) && n.Name() != nil {
		return n.Name().Text(), true
	}
	if ast.IsMethodDeclaration(n) {
		return propertyName(n.Name())
	}
	p := n.Parent
	if p != nil && ast.IsVariableDeclaration(p) && ast.IsIdentifier(p.Name()) {
		return p.Name().Text(), true
	}
	if p != nil && ast.IsPropertyAssignment(p) {
		return propertyName(p.Name())
	}
	if p != nil && ast.IsExportAssignment(p) {
		return "default", true
	}
	return "", false
}

func (r *jsReader) parameterOf(p *ast.Node) *obj {
	var out *obj
	if ast.IsIdentifier(p.Name()) {
		out = newObj("name", p.Name().Text())
	} else {
		out = newObj("pattern", r.text(p.Name()))
	}
	if t := p.Type(); t != nil && !reparsed(t) {
		out.set("type", r.typeOf(t))
		out.set("typeFrom", "annotation")
	} else if jt := r.jsDocType(p); jt != nil {
		out.set("type", r.typeOf(jt))
		out.set("typeFrom", "jsdoc")
	}
	if q := p.QuestionToken(); (q != nil && !reparsed(q)) || p.Initializer() != nil {
		out.set("optional", true)
	}
	return out
}

func (r *jsReader) parameters(fn *ast.Node) []any {
	out := []any{}
	for _, p := range fn.Parameters() {
		if !reparsed(p) {
			out = append(out, r.parameterOf(p))
		}
	}
	return out
}

func heritage(r *jsReader, n *ast.Node) []any {
	var out []any
	var clauses []*ast.Node
	switch {
	case ast.IsInterfaceDeclaration(n):
		clauses = nodes(n.AsInterfaceDeclaration().HeritageClauses)
	case ast.IsClassDeclaration(n):
		clauses = nodes(n.AsClassDeclaration().HeritageClauses)
	}
	for _, h := range clauses {
		if reparsed(h) {
			continue
		}
		for _, t := range nodes(h.AsHeritageClause().Types) {
			if !reparsed(t) {
				out = append(out, r.nodeText(t.Expression()))
			}
		}
	}
	return out
}

func (r *jsReader) visit(node *ast.Node) bool {
	if reparsed(node) {
		return false
	}
	if ast.IsImportDeclaration(node) && ast.IsStringLiteral(node.ModuleSpecifier()) {
		names := []any{}
		c := node.ImportClause()
		if c != nil {
			if c.Name() != nil {
				names = append(names, newObj("imported", "default", "local", c.Name().Text()))
			}
			if nb := c.AsImportClause().NamedBindings; nb != nil {
				if ast.IsNamespaceImport(nb) {
					names = append(names, newObj("imported", "*", "local", nb.Name().Text()))
				}
				if ast.IsNamedImports(nb) {
					for _, s := range nb.Elements() {
						imported := s.PropertyName()
						if imported == nil {
							imported = s.Name()
						}
						names = append(names, newObj("imported", imported.Text(), "local", s.Name().Text()))
					}
				}
			}
		}
		f := newObj("module", node.ModuleSpecifier().Text(), "names", names)
		if c != nil && c.IsTypeOnly() {
			f.set("typeOnly", true)
		}
		if resolved, ok := r.resolvedModule(node.ModuleSpecifier().Text()); ok {
			f.set("resolved", resolved)
		}
		r.emit("import", node, f)
		return false
	}
	switch {
	case ast.IsInterfaceDeclaration(node):
		members := []any{}
		for _, m := range node.Members() {
			if ast.IsPropertySignatureDeclaration(m) && !reparsed(m) {
				members = append(members, r.member(m))
			}
		}
		f := newObj("declaration", "interface", "name", node.Name().Text(), "members", members)
		if ex := isExported(node); ex != "" {
			f.set("exported", ex)
		}
		if ext := heritage(r, node); len(ext) > 0 {
			f.set("extends", ext)
		}
		r.emit("type", node, f)
	case ast.IsTypeAliasDeclaration(node):
		f := newObj("declaration", "alias", "name", node.Name().Text(), "type", r.typeOf(node.Type()))
		if ex := isExported(node); ex != "" {
			f.set("exported", ex)
		}
		r.emit("type", node, f)
	case ast.IsClassDeclaration(node) && node.Name() != nil:
		f := newObj("declaration", "class", "name", node.Name().Text(), "members", []any{})
		if ex := isExported(node); ex != "" {
			f.set("exported", ex)
		}
		if ext := heritage(r, node); len(ext) > 0 {
			f.set("extends", ext)
		}
		if decs := r.decorators(node); len(decs) > 0 {
			f.set("decorators", decs)
		}
		members := []any{}
		for _, m := range node.Members() {
			if !ast.IsPropertyDeclaration(m) || reparsed(m) {
				continue
			}
			mf := r.member(m)
			if md := r.decorators(m); len(md) > 0 {
				mf.set("decorators", md)
			}
			if t := m.Type(); t == nil || reparsed(t) {
				if jt := r.jsDocType(m); jt != nil {
					mf.set("type", r.typeOf(jt))
					mf.set("typeFrom", "jsdoc")
				}
			}
			for _, mod := range modifiers(m) {
				if mod.Kind == ast.KindStaticKeyword {
					mf.set("static", true)
				}
			}
			members = append(members, mf)
		}
		f.set("members", members)
		r.emit("type", node, f)
	case ast.IsEnumDeclaration(node):
		values := []any{}
		for _, m := range node.Members() {
			name, ok := propertyName(m.Name())
			if !ok {
				name = r.text(m.Name())
			}
			v := newObj("name", name)
			if m.Initializer() != nil {
				v.set("value", r.value(m.Initializer(), 0))
			}
			values = append(values, v)
		}
		f := newObj("declaration", "enum", "name", node.Name().Text(), "values", values)
		if ex := isExported(node); ex != "" {
			f.set("exported", ex)
		}
		r.emit("type", node, f)
	}
	for _, doc := range r.jsDocs(node) {
		tags := doc.AsJSDoc().Tags
		if tags == nil {
			continue
		}
		for _, tag := range tags.Nodes {
			if tag.Kind != ast.KindJSDocTypedefTag || tag.Name() == nil {
				continue
			}
			f := newObj("declaration", "typedef", "name", r.nodeText(tag.Name()), "comment", r.nodeText(doc))
			if te := tag.TypeExpression(); te != nil {
				f.set("type", r.typeOf(te))
			}
			r.emit("type", tag, f)
		}
	}
	if isFunctionLike(node) {
		f := newObj()
		name, named := functionName(node)
		if named {
			f.set("name", name)
		}
		f.set("parameters", r.parameters(node))
		if t := node.Type(); t != nil && !reparsed(t) {
			f.set("returns", r.typeOf(t))
		}
		holder := node
		if ast.IsVariableDeclaration(node.Parent) {
			holder = node.Parent.Parent.Parent
		}
		ex := ""
		if ast.IsExportAssignment(node.Parent) {
			ex = "default"
		} else {
			ex = isExported(holder)
		}
		if ex != "" {
			f.set("exported", ex)
		}
		if doc, ok := r.jsDocText(holder); ok && jsFile.MatchString(r.sf.FileName()) {
			f.set("jsdoc", doc)
		}
		if body := node.Body(); body != nil && ast.IsBlock(body) {
			if dirs := directives(body.Statements()); len(dirs) > 0 {
				var list []any
				for _, d := range dirs {
					list = append(list, d)
				}
				f.set("directives", list)
			}
		}
		// What generateStaticParams returns gives a route's parameters' values.
		if named && name == "generateStaticParams" {
			values := []any{}
			for _, v := range returnValues(node) {
				values = append(values, r.value(v, 0))
			}
			f.set("returnValues", values)
		}
		f.merge(r.context(node))
		r.emit("function", node, f)
	}
	if ast.IsVariableDeclaration(node) && node.Initializer() != nil {
		init := unwrap(node.Initializer())
		ctx := r.context(node)
		if !ctx.has("within") || ast.IsCallExpression(init) || ast.IsNewExpression(init) {
			f := newObj()
			switch name := node.Name(); {
			case ast.IsIdentifier(name):
				f.set("name", name.Text())
			case ast.IsObjectBindingPattern(name):
				names := []any{}
				for _, e := range name.Elements() {
					n := newObj("local", r.nodeText(e.Name()))
					if pn := e.PropertyName(); pn != nil {
						n.set("property", r.nodeText(pn))
					}
					if e.Initializer() != nil {
						n.set("default", r.value(e.Initializer(), 0))
					}
					if e.AsBindingElement().DotDotDotToken != nil {
						n.set("rest", true)
					}
					names = append(names, n)
				}
				f.set("names", names)
			default:
				f.set("pattern", r.text(name))
			}
			flags := node.Parent.Flags
			switch {
			case flags&ast.NodeFlagsConst != 0:
				f.set("declaration", "const")
			case flags&ast.NodeFlagsLet != 0:
				f.set("declaration", "let")
			default:
				f.set("declaration", "var")
			}
			f.set("value", r.value(node.Initializer(), 0))
			if t := node.Type(); t != nil && !reparsed(t) {
				f.set("type", r.typeOf(t))
			} else if jt := r.jsDocType(node); jt != nil {
				f.set("type", r.typeOf(jt))
				f.set("typeFrom", "jsdoc")
			}
			if stmt := node.Parent.Parent; ast.IsVariableStatement(stmt) {
				if ex := isExported(stmt); ex != "" {
					f.set("exported", ex)
				}
			}
			f.merge(ctx)
			r.emit("variable", node, f)
		}
		// Reads of the environment or of a request by destructuring.
		if ast.IsObjectBindingPattern(node.Name()) {
			if base := r.readAccess(init); base != nil {
				for _, e := range node.Name().Elements() {
					if e.AsBindingElement().DotDotDotToken != nil {
						continue
					}
					var key any
					if pn := e.PropertyName(); pn != nil {
						if k, ok := propertyName(pn); ok {
							key = k
						}
					} else {
						key = r.nodeText(e.Name())
					}
					if key == nil {
						key = "[computed]"
					}
					chain := append(append([]any(nil), base.get("chain").([]any)...), key)
					f := newObj("chain", chain)
					if base.has("parameter") {
						f.set("parameter", base.get("parameter"))
					}
					if e.Initializer() != nil {
						f.set("default", r.value(e.Initializer(), 0))
					}
					f.merge(r.context(node))
					r.emit("access", e, f)
				}
			}
		}
	}
	if ast.IsSourceFile(node) {
		stmts := node.Statements()
		for _, d := range directives(stmts) {
			r.emit("directive", stmts[0], newObj("name", d))
		}
	}
	// export { handler as GET }, from this module only.
	if ast.IsExportDeclaration(node) && node.ModuleSpecifier() == nil {
		if ec := node.AsExportDeclaration().ExportClause; ec != nil && ast.IsNamedExports(ec) {
			for _, e := range ec.Elements() {
				local := e.PropertyName()
				if local == nil {
					local = e.Name()
				}
				if ast.IsIdentifier(local) {
					r.emit("export", e, newObj("name", e.Name().Text(), "value", withName(local.Text(), r.resolveName(local))))
				}
			}
		}
	}
	if ast.IsExportAssignment(node) {
		r.emit("export", node, newObj("name", "default", "value", r.value(node.Expression(), 0)))
	}
	if ast.IsCallExpression(node) {
		callee := unwrap(node.Expression())
		args := node.Arguments()
		switch {
		case node.Expression().Kind == ast.KindImportKeyword:
			if len(args) > 0 && ast.IsStringLiteralLike(args[0]) {
				f := newObj("module", args[0].Text(), "names", []any{}, "dynamic", true)
				if resolved, ok := r.resolvedModule(args[0].Text()); ok {
					f.set("resolved", resolved)
				}
				f.merge(r.context(node))
				r.emit("import", node, f)
			} else {
				f := newObj("what", "import", "value", r.argValue(args))
				f.merge(r.context(node))
				r.emit("dynamic", node, f)
			}
		case ast.IsIdentifier(callee) && callee.Text() == "require" && !r.resolveName(callee).has("declaration"):
			if len(args) > 0 && ast.IsStringLiteralLike(args[0]) {
				f := newObj("module", args[0].Text(), "names", []any{}, "require", true)
				if resolved, ok := r.resolvedModule(args[0].Text()); ok {
					f.set("resolved", resolved)
				}
				r.emit("import", node, f)
			} else {
				f := newObj("what", "require", "value", r.argValue(args))
				f.merge(r.context(node))
				r.emit("dynamic", node, f)
			}
		case r.wantedCall(node):
			f := r.callOf(node, 0)
			if to, ok := assignedTo(node); ok {
				f.set("assignedTo", to)
			}
			f.merge(r.context(node))
			r.emit("call", node, f)
		}
	}
	if ast.IsBinaryExpression(node) && node.AsBinaryExpression().OperatorToken.Kind == ast.KindEqualsToken {
		l := node.AsBinaryExpression().Left
		if ast.IsElementAccessExpression(l) && !ast.IsStringLiteralLike(l.AsElementAccessExpression().ArgumentExpression) {
			if t := r.nodeText(l.Expression()); t == "module.exports" || t == "exports" {
				f := newObj("what", "export", "value", r.value(l.AsElementAccessExpression().ArgumentExpression, 0))
				f.merge(r.context(node))
				r.emit("dynamic", node, f)
			}
		}
	}
	if (ast.IsPropertyAccessExpression(node) || ast.IsElementAccessExpression(node)) && isAccessTop(node) {
		if f := r.readAccess(node); f != nil {
			r.accessDetails(node, f)
			f.merge(r.context(node))
			r.emit("access", node, f)
		}
	}
	if ast.IsJsxElement(node) || ast.IsJsxSelfClosingElement(node) {
		o := opening(node)
		wanted := false
		if tn := o.TagName(); ast.IsIdentifier(tn) && lowerStart.MatchString(tn.Text()) {
			wanted = intrinsicElements[tn.Text()]
		} else {
			m, ok := r.importedFrom(calleeRoot(tn))
			wanted = ok && knownModule(m)
		}
		if wanted {
			f := r.element(node, 0)
			for p := node.Parent; p != nil && !isFunctionLike(p); p = p.Parent {
				if (ast.IsJsxElement(p) || ast.IsJsxSelfClosingElement(p)) && p != node {
					po := opening(p)
					if !(ast.IsIdentifier(po.TagName()) && lowerStart.MatchString(po.TagName().Text())) {
						l, c := r.where(p)
						f.set("parent", newObj("line", l, "column", c))
						f.set("parentTag", r.tagOf(po.TagName()))
						break
					}
				}
			}
			f.merge(r.context(node))
			r.emit("jsx", node, f)
		}
	}
	node.ForEachChild(r.visit)
	return false
}

func (r *jsReader) argValue(args []*ast.Node) *obj {
	if len(args) > 0 {
		return r.value(args[0], 0)
	}
	return newObj("text", "")
}

// resolveSpec resolves an import: a relative import of a .vue file to its
// script, and any other as the compiler resolves it.
func (r *jsReader) resolveSpec(specifier, containing string) (string, bool) {
	if strings.HasSuffix(specifier, ".vue") && strings.HasPrefix(specifier, ".") {
		abs := tspath.NormalizePath(tspath.CombinePaths(tspath.GetDirectoryPath(containing), specifier))
		if v := r.vue[r.rel(abs)]; v != nil {
			return v.virtual, true
		}
		return "", false
	}
	m, _ := r.resolver.ResolveModuleName(specifier, containing, core.ResolutionModeNone, nil)
	if m == nil || m.ResolvedFileName == "" {
		return "", false
	}
	return m.ResolvedFileName, true
}

func (r *jsReader) resolvedModule(specifier string) (string, bool) {
	if f, ok := r.resolveSpec(specifier, r.sf.FileName()); ok && r.tracked[r.rel(f)] {
		return r.rel(f), true
	}
	return "", false
}

// jsFS is the file system the compiler reads: the files listed, the Vue
// files' scripts, and nothing else.
type jsFS struct{ r *jsReader }

func (f *jsFS) UseCaseSensitiveFileNames() bool { return true }

func (f *jsFS) virtual(p string) (*vueFile, bool) {
	if v, ok := f.r.virtualToVue[p]; ok {
		return f.r.vue[v], true
	}
	return nil, false
}

func (f *jsFS) FileExists(p string) bool {
	if _, ok := f.virtual(p); ok {
		return true
	}
	if !f.r.tracked[f.r.rel(p)] || strings.HasSuffix(f.r.rel(p), ".vue") {
		return false
	}
	info, err := os.Stat(filepath.FromSlash(p))
	return err == nil && !info.IsDir()
}

func (f *jsFS) ReadFile(p string) (string, bool) {
	if v, ok := f.virtual(p); ok {
		return v.text, true
	}
	if !f.r.tracked[f.r.rel(p)] {
		return "", false
	}
	data, err := os.ReadFile(filepath.FromSlash(p))
	if err != nil {
		return "", false
	}
	return string(data), true
}

func (f *jsFS) WriteFile(string, string) error { return os.ErrPermission }

func (f *jsFS) AppendFile(string, string) error { return os.ErrPermission }

func (f *jsFS) Remove(string) error { return os.ErrPermission }

func (f *jsFS) Chtimes(string, time.Time, time.Time) error { return os.ErrPermission }

func (f *jsFS) DirectoryExists(p string) bool {
	rel := f.r.rel(p)
	return rel == "." || rel == "" || f.r.trackedDirs[rel]
}

func (f *jsFS) GetAccessibleEntries(string) vfs.Entries { return vfs.Entries{} }

func (f *jsFS) Stat(p string) vfs.FileInfo {
	if !f.FileExists(p) {
		return nil
	}
	if _, ok := f.virtual(p); ok {
		return nil
	}
	info, err := os.Stat(filepath.FromSlash(p))
	if err != nil {
		return nil
	}
	return info
}

func (f *jsFS) WalkDir(string, vfs.WalkDirFunc) error { return nil }

func (f *jsFS) Realpath(p string) string { return p }
