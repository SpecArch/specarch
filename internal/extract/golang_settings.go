package extract

import (
	"encoding/json"
	"fmt"
	"go/ast"
	gotoken "go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The import paths of dxlib's packages that read settings.
const (
	dxlibOS            = "github.com/donnyhardyanto/dxlib/utils/os"
	dxlibConfiguration = "github.com/donnyhardyanto/dxlib/configuration"
)

// The calls that read one setting from the environment by its name, and
// the type of the value each gives: os's own, and dxlib's, which take a
// default after the name.
var envCalls = map[string]map[string]string{
	"os":    {"Getenv": "string", "LookupEnv": "string"},
	dxlibOS: {"GetEnvDefaultValue": "string", "GetEnvDefaultValueAsInt": "int", "GetEnvDefaultValueAsBool": "bool"},
}

// A setting's name that says it holds a credential, as dxlib's own masking
// of configuration reads it.
var secretWord = regexp.MustCompile(`(?i)(secret|token|password|passwd|key|credential)`)

// settingRead is one place the source reads a setting, or a configuration
// file gives one.
type settingRead struct {
	key      string // as the source names it
	kind     string // string, int, bool, number, or "" when not known
	value    *yaml.Node
	secret   bool // the source marks it sensitive
	clause   string
	says     string
	fromFile bool
}

// readSettings reads every setting the module reads from the environment
// by a literal name, and every configuration dxlib's NewConfiguration
// declares with a tracked JSON or YAML file read as data.
func (g *goReader) readSettings() {
	g.settings = map[string][]settingRead{}
	envs, configs := 0, 0
	for _, f := range g.files {
		ast.Inspect(f.ast, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok {
				if kind, ok := envCalls[f.imports[x.Name]][sel.Sel.Name]; ok {
					envs++
					g.envRead(f, call, sel.Sel.Name, kind)
					return true
				}
			}
			if (sel.Sel.Name == "NewConfiguration" || sel.Sel.Name == "NewIfNotExistConfiguration") && f.importName(dxlibConfiguration) != "" {
				configs++
				g.configuration(f, call, sel.Sel.Name)
			}
			return true
		})
	}
	if envs > 0 || configs > 0 {
		g.res.say("settings: counted %s of the environment and %s: every call of os or of dxlib's utils/os that reads the environment by a name, and every NewConfiguration of dxlib's configuration", plural(envs, "read"), plural(configs, "configuration"))
	}
}

// envRead reads one call that reads the environment.
func (g *goReader) envRead(f *goFile, call *ast.CallExpr, name, kind string) {
	clause := g.clause(f, call.Pos())
	key, ok := g.stringValue(f, firstArg(call))
	if !ok {
		g.question("should", fmt.Sprintf("%s reads a setting from the environment with %s, by the name %s, which is not a literal. Which setting does it read?", clause, name, exprText(firstArg(call))),
			[]string{"configuration"}, "A computed name is not known by syntax.", g.at(clause, "Calls "+name+" with a name that is not a literal."))
		return
	}
	f.cited = true
	r := settingRead{key: key, kind: kind, clause: clause, says: fmt.Sprintf("Reads %s from the environment with %s.", key, name)}
	if len(call.Args) > 1 {
		r.value = g.literalValue(f, call.Args[1], kind)
	}
	g.settings[key] = append(g.settings[key], r)
}

// literalValue is a literal of the kind given, or nil.
func (g *goReader) literalValue(f *goFile, e ast.Expr, kind string) *yaml.Node {
	switch kind {
	case "string":
		if s, ok := g.stringValue(f, e); ok {
			return str(s)
		}
	case "int":
		if b, ok := e.(*ast.BasicLit); ok && b.Kind == gotoken.INT {
			if n, err := strconv.ParseInt(b.Value, 0, 64); err == nil {
				return value(n)
			}
		}
	case "bool":
		if id, ok := e.(*ast.Ident); ok && (id.Name == "true" || id.Name == "false") {
			return value(id.Name == "true")
		}
	case "number":
		if b, ok := e.(*ast.BasicLit); ok && (b.Kind == gotoken.FLOAT || b.Kind == gotoken.INT) {
			return literal("!!float", b.Value)
		}
	}
	return nil
}

// configuration reads one NewConfiguration call: its file, read as data,
// gives settings named after the configuration and the key's path, and its
// literal defaults and sensitive keys add to them.
func (g *goReader) configuration(f *goFile, call *ast.CallExpr, name string) {
	clause := g.clause(f, call.Pos())
	f.cited = true
	if len(call.Args) != 7 {
		g.res.say("nothing read: %s calls %s with %d arguments, and dxlib's takes 7", clause, name, len(call.Args))
		return
	}
	nameID, ok1 := g.stringValue(f, call.Args[0])
	file, ok2 := g.stringValue(f, call.Args[1])
	format, ok3 := g.stringValue(f, call.Args[2])
	if !ok1 || !ok2 || !ok3 {
		g.question("should", fmt.Sprintf("%s declares a configuration whose name, file or format is not a literal. Which settings does it hold?", clause),
			[]string{"configuration"}, "A configuration's file is read as data only where the call names it with literals.", g.at(clause, "Calls "+name+" with values that are not literals."))
		return
	}
	sensitive := map[string]bool{}
	if keys, ok := g.stringList(f, call.Args[6]); ok {
		for _, k := range keys {
			sensitive[k] = true
		}
	} else {
		g.question("must", fmt.Sprintf("%s: the configuration %s names its sensitive keys with %s, which is not a literal list. Which of its settings are secrets?", clause, nameID, exprText(call.Args[6])),
			[]string{"configuration"}, "A secret's value is never written in a specification, and only the list says which keys dxlib masks.", g.at(clause, "Names the sensitive keys of "+nameID+" with something other than a literal list."))
	}
	values := map[string]any{}
	paths := g.r.Files
	var found []string
	for _, p := range paths {
		if p == file || strings.HasSuffix(p, "/"+strings.TrimPrefix(file, "./")) {
			found = append(found, p)
		}
	}
	switch {
	case len(found) == 1 && (format == "json" || format == "yaml"):
		data, err := os.ReadFile(filepath.Join(g.r.Repository.Root, filepath.FromSlash(found[0])))
		if err == nil {
			if format == "json" {
				err = json.Unmarshal(data, &values)
			} else {
				err = yaml.Unmarshal(data, &values)
			}
		}
		if err != nil {
			g.question("should", fmt.Sprintf("%s declares the configuration %s from %s, which does not read as %s. Which settings does it hold?", clause, nameID, found[0], strings.ToUpper(format)),
				[]string{"configuration"}, "A file that does not parse gives no setting.", g.at(clause, "Reads "+found[0]+"."))
			values = map[string]any{}
		} else {
			g.fileRead[found[0]] = true
		}
	case len(found) == 0:
		g.question("should", fmt.Sprintf("%s declares the configuration %s from the file %s, which is not a tracked file of the paths read. Which settings does it hold, and with which values?", clause, nameID, file),
			[]string{"configuration"}, "The file holds the values; one outside the commit read, such as one a deployment writes, is not read.", g.at(clause, "Reads "+file+"."))
	default:
		g.question("should", fmt.Sprintf("%s declares the configuration %s from the file %s, which names %s of the paths read. Which one does the service read?", clause, nameID, file, joinAnd(found)),
			[]string{"configuration"}, "The file is named relative to where the service runs, which syntax does not say.", g.at(clause, "Reads "+file+"."))
	}
	at := clause
	if len(found) == 1 {
		at = found[0]
	}
	leaves(values, "", func(key string, v any) {
		r := settingRead{key: nameID + "." + key, secret: sensitive[key], clause: at, fromFile: true,
			says: fmt.Sprintf("The configuration %s, read by %s, holds %s.", nameID, clause, key)}
		r.kind, r.value = jsonKind(v)
		g.settings[r.key] = append(g.settings[r.key], r)
	})
	if lit, ok := call.Args[5].(*ast.CompositeLit); ok {
		g.literalLeaves(f, lit, "", func(key string, kind string, v *yaml.Node) {
			r := settingRead{key: nameID + "." + key, kind: kind, value: v, secret: sensitive[key], clause: clause,
				says: fmt.Sprintf("The configuration %s defaults %s.", nameID, key)}
			g.settings[r.key] = append(g.settings[r.key], r)
		})
	}
	for k := range sensitive {
		if _, ok := g.settings[nameID+"."+k]; !ok {
			g.settings[nameID+"."+k] = append(g.settings[nameID+"."+k], settingRead{key: nameID + "." + k, secret: true, clause: clause,
				says: fmt.Sprintf("The configuration %s names %s sensitive.", nameID, k)})
		}
	}
}

// leaves calls back for every leaf of a value read from a file, by its
// dotted path, in the order of the keys.
func leaves(v any, prefix string, each func(string, any)) {
	m, ok := v.(map[string]any)
	if !ok {
		each(prefix, v)
		return
	}
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p := k
		if prefix != "" {
			p = prefix + "." + k
		}
		leaves(m[k], p, each)
	}
}

// literalLeaves calls back for every literal leaf of a utils.JSON literal.
func (g *goReader) literalLeaves(f *goFile, lit *ast.CompositeLit, prefix string, each func(string, string, *yaml.Node)) {
	for _, el := range lit.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		k, ok := g.stringValue(f, kv.Key)
		if !ok {
			continue
		}
		p := k
		if prefix != "" {
			p = prefix + "." + k
		}
		if inner, ok := kv.Value.(*ast.CompositeLit); ok {
			g.literalLeaves(f, inner, p, each)
			continue
		}
		for _, kind := range []string{"string", "int", "bool"} {
			if v := g.literalValue(f, kv.Value, kind); v != nil {
				each(p, kind, v)
				break
			}
		}
	}
}

// jsonKind is the kind of a value read from a file, and the value as a
// node when it is a scalar.
func jsonKind(v any) (string, *yaml.Node) {
	switch v := v.(type) {
	case string:
		return "string", str(v)
	case bool:
		return "bool", value(v)
	case int:
		return "int", value(v)
	case int64:
		return "int", value(v)
	case float64:
		if v == float64(int64(v)) {
			return "int", value(int64(v))
		}
		return "number", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: strconv.FormatFloat(v, 'g', -1, 64)}
	}
	return "", nil
}

// settingName is the camelCase name of a setting the source names in
// another form: its words, split at anything but a letter or a digit, in
// lower case and joined in camelCase, so DESK_API_KEY is deskApiKey.
func settingName(key string) string {
	words := regexp.MustCompile(`[^A-Za-z0-9]+`).Split(key, -1)
	var b strings.Builder
	for _, w := range words {
		if w == "" {
			continue
		}
		w = strings.ToLower(w)
		if b.Len() > 0 {
			w = strings.ToUpper(w[:1]) + w[1:]
		}
		b.WriteString(w)
	}
	return b.String()
}

// writeSettings writes each setting read under configuration, by its
// camelCase name, with the type and the default the source gives; what
// it is for, and whether it is a secret where the source does not say, is
// a question.
func (g *goReader) writeSettings() {
	if len(g.settings) == 0 {
		return
	}
	byName := map[string][]string{}
	var keys []string
	for k := range g.settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		n := settingName(k)
		byName[n] = append(byName[n], k)
	}
	g.config = &yaml.Node{Kind: yaml.MappingNode}
	var names, describe, secrets []string
	var describeBlocks, secretBlocks []string
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		ks := byName[n]
		reads := g.settings[ks[0]]
		first := reads[0].clause
		if len(ks) > 1 || !memberNameWord.MatchString(n) {
			if len(ks) > 1 {
				g.question("must", fmt.Sprintf("The settings %s are all named %s in camelCase, and a setting has one name. Which name does each take?", joinAnd(ks), n),
					[]string{"configuration"}, "Two settings the source tells apart would become one.", g.at(first, "Reads "+ks[0]+"."))
			} else {
				g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: setting %s: no camelCase name can be made from it; left out", first, ks[0]), clause: first, blocks: []string{"configuration"}})
			}
			continue
		}
		at := "#/configuration/" + n
		key := ks[0]
		kinds := map[string]bool{}
		// A configuration file's value wins over the code's default, as
		// dxlib merges the file over the defaults it is given.
		var values, fileValues []*yaml.Node
		secret := false
		var cites []*yaml.Node
		for _, r := range reads {
			if r.kind != "" {
				kinds[r.kind] = true
			}
			if r.value != nil && r.fromFile {
				fileValues = append(fileValues, r.value)
			} else if r.value != nil {
				values = append(values, r.value)
			}
			secret = secret || r.secret
			cites = append(cites, g.at(r.clause, r.says))
		}
		s := mapping()
		schema := mapping()
		switch {
		case len(kinds) > 1:
			var list []string
			for k := range kinds {
				list = append(list, k)
			}
			sort.Strings(list)
			g.question("must", fmt.Sprintf("The setting %s is read as %s. Which type is it?", key, joinAnd(list)),
				[]string{at + "/schema"}, "A setting has one type, and the reads disagree.", g.at(first, "Reads "+key+"."))
		case kinds["string"]:
			set(schema, "type", "string")
		case kinds["bool"]:
			set(schema, "type", "boolean")
		case kinds["number"]:
			set(schema, "type", "number")
			set(schema, "format", "double")
		case kinds["int"]:
			set(schema, "type", "integer")
			g.question("must", fmt.Sprintf("The setting %s is read as a whole number. How wide is it?", key),
				[]string{at + "/schema/format"}, "Go's int and a number in a file have no width the source states, and every integer in a specification has one.", g.at(first, "Reads "+key+" as a whole number."))
		default:
			g.question("must", fmt.Sprintf("Which type is the setting %s? The source names it sensitive and gives no value.", key),
				[]string{at + "/schema"}, "A setting has a type, and nothing read gives it.", g.at(first, "Names "+key+"."))
		}
		if len(fileValues) > 0 {
			values = fileValues
		}
		if !secret && !secretWord.MatchString(key) && len(values) > 0 {
			v := values[0]
			same := true
			for _, o := range values[1:] {
				same = same && o.Value == v.Value
			}
			if same {
				set(schema, "default", v)
			} else {
				g.question("should", fmt.Sprintf("The setting %s is given several defaults where it is read. Which is its default?", key),
					[]string{at + "/schema/default"}, "A setting has one default, and the reads disagree.", g.at(first, "Reads "+key+"."))
			}
		}
		set(s, "schema", flow(schema))
		describe = append(describe, key)
		describeBlocks = append(describeBlocks, at+"/description")
		switch {
		case secret:
			set(s, "secret", true)
		case secretWord.MatchString(key):
			secrets = append(secrets, key)
			secretBlocks = append(secretBlocks, at+"/secret")
		default:
			describeBlocks = append(describeBlocks, at+"/secret")
		}
		set(s, "origin", "stated")
		set(s, "cites", cites)
		set(g.config, n, s)
	}
	if len(describe) > 0 {
		g.question("must", fmt.Sprintf("What is each setting for, where does its value come from, and is it a secret: %s?", strings.Join(describe, ", ")),
			describeBlocks, "The source names the settings it reads and not what they are for; whether one is a secret is said only where the source marks it sensitive.")
	}
	if len(secrets) > 0 {
		g.question("must", fmt.Sprintf("Is each of these settings a secret: %s? Each name says it may hold a credential, and the source does not mark it sensitive.", strings.Join(secrets, ", ")),
			secretBlocks, "A secret's value is never written in a specification, and the name alone does not say which a setting is.")
	}
	if len(g.config.Content) == 0 {
		g.config = nil
	}
}

// readGates finds, in every middleware an endpoint's chain names that the
// reader finds by syntax, a check that lets every request through while a
// setting is empty or false: an if statement among the function's own
// statements that returns nil when a setting read from the environment is
// empty, or a boolean setting is false.
func (g *goReader) readGates() {
	seen := map[*ast.FuncDecl]bool{}
	for _, ep := range g.endpoints {
		for _, m := range ep.chainExprs {
			fd, file := g.funcOf(ep.file, m)
			if fd == nil || seen[fd] || fd.Body == nil {
				continue
			}
			seen[fd] = true
			g.gatesIn(file, fd)
		}
	}
}

// funcOf is the function a name in a call's arguments gives, found by
// syntax as a handler is.
func (g *goReader) funcOf(f *goFile, e ast.Expr) (*ast.FuncDecl, *goFile) {
	switch e := e.(type) {
	case *ast.Ident:
		if fd := g.funcs[path.Dir(f.path)][e.Name]; fd != nil {
			return fd, g.funcFile[fd]
		}
	case *ast.SelectorExpr:
		if x, ok := e.X.(*ast.Ident); ok && g.modPath != "" {
			ip := f.imports[x.Name]
			if ip == g.modPath || strings.HasPrefix(ip, g.modPath+"/") {
				dir := path.Join(g.modRoot, strings.TrimPrefix(strings.TrimPrefix(ip, g.modPath), "/"))
				if fd := g.funcs[dir][e.Sel.Name]; fd != nil {
					return fd, g.funcFile[fd]
				}
			}
		}
	}
	return nil, nil
}

// gatesIn finds the gates of one middleware: an if statement among its own
// statements that returns nil.
func (g *goReader) gatesIn(f *goFile, fd *ast.FuncDecl) {
	g.gatesInList(f, fd, fd.Body.List, func(body *ast.BlockStmt) bool {
		if len(body.List) != 1 {
			return false
		}
		ret, ok := body.List[0].(*ast.ReturnStmt)
		return ok && len(ret.Results) == 1 && exprText(ret.Results[0]) == "nil"
	})
}

// gatesInList finds, among one list of a check's statements, an if
// statement with no init whose body lets the request through, as through
// says, while a setting read from the environment by a literal name,
// directly or through a name the function assigns once, is empty, or a
// boolean one is false.
func (g *goReader) gatesInList(f *goFile, fd *ast.FuncDecl, list []ast.Stmt, through func(*ast.BlockStmt) bool) {
	locals := map[string]ast.Expr{} // a name -> the one value assigned to it
	counts := map[string]int{}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok && len(as.Rhs) == 1 {
			name := exprText(as.Lhs[0])
			counts[name]++
			locals[name] = as.Rhs[0]
		}
		return true
	})
	envOf := func(e ast.Expr) (string, string) {
		if id, ok := e.(*ast.Ident); ok && counts[id.Name] == 1 {
			e = locals[id.Name]
		}
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return "", ""
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return "", ""
		}
		x, ok := sel.X.(*ast.Ident)
		if !ok {
			return "", ""
		}
		kind, ok := envCalls[f.imports[x.Name]][sel.Sel.Name]
		if !ok {
			return "", ""
		}
		key, ok := g.stringValue(f, firstArg(call))
		if !ok {
			return "", ""
		}
		return key, kind
	}
	for _, st := range list {
		is, ok := st.(*ast.IfStmt)
		if !ok || is.Init != nil || !through(is.Body) {
			continue
		}
		setting, boolean := "", false
		switch c := is.Cond.(type) {
		case *ast.BinaryExpr:
			if c.Op == gotoken.EQL {
				if b, ok := c.Y.(*ast.BasicLit); ok && b.Value == `""` {
					if key, kind := envOf(c.X); kind == "string" {
						setting = key
					}
				}
				if call, ok := c.X.(*ast.CallExpr); ok && exprText(call.Fun) == "len" && len(call.Args) == 1 {
					if b, ok := c.Y.(*ast.BasicLit); ok && b.Value == "0" {
						if key, kind := envOf(call.Args[0]); kind == "string" {
							setting = key
						}
					}
				}
			}
		case *ast.UnaryExpr:
			if c.Op == gotoken.NOT {
				if key, kind := envOf(c.X); kind == "bool" {
					setting, boolean = key, true
				}
			}
		}
		if setting == "" {
			continue
		}
		clause := g.clause(f, is.Pos())
		f.cited = true
		g.gates = append(g.gates, goGate{check: fd.Name.Name, setting: setting, clause: clause, boolean: boolean})
	}
}

// goGate is a check found in source that lets every request through while
// a setting is empty.
type goGate struct {
	check, setting, clause string
	boolean                bool // the setting is a boolean that switches the check off while false
}

// writeGates asks about every gate found, as the permission table's
// reader asks about one it declares, so that the merge joins them by the
// check's name.
func (g *goReader) writeGates() {
	sort.SliceStable(g.gates, func(i, j int) bool {
		if g.gates[i].check != g.gates[j].check {
			return g.gates[i].check < g.gates[j].check
		}
		return g.gates[i].setting < g.gates[j].setting
	})
	for _, gt := range g.gates {
		q, why := gateQuestion(gt.check, gt.setting)
		says := fmt.Sprintf("%s lets every request through while %s is empty.", gt.check, gt.setting)
		if gt.boolean {
			q, why = boolGateQuestion(gt.check, gt.setting)
			says = fmt.Sprintf("%s lets every request through while %s is false.", gt.check, gt.setting)
			g.res.say("gate: %s: the check %s runs only when the setting %s is true, and lets every request through while it is false", gt.clause, gt.check, gt.setting)
		} else {
			g.res.say("gate: %s: the check %s runs only when the setting %s is present, and lets every request through while it is empty", gt.clause, gt.check, gt.setting)
		}
		g.question("must", q, []string{"roles"}, why, g.at(gt.clause, says))
	}
}
