package extract

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// plistEntry is one key of a property list's top-level dictionary.
type plistEntry struct {
	key, kind, value string // kind: string, integer, real, true, false, or another element's name
	line             int
}

// readPlist reads a property list in XML: its top-level dictionary's keys,
// with their lines. ok is false when the file is not one.
func readPlist(data []byte) ([]plistEntry, bool) {
	if bytes.HasPrefix(data, []byte("bplist")) {
		return nil, false
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	line := func() int { l, _ := dec.InputPos(); return l }
	depth := 0
	var entries []plistEntry
	var cur *plistEntry
	sawDict := false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 && t.Name.Local == "dict" {
				sawDict = true
			}
			if depth != 3 {
				continue
			}
			if t.Name.Local == "key" {
				var k string
				if err := dec.DecodeElement(&k, &t); err != nil {
					return nil, false
				}
				depth--
				entries = append(entries, plistEntry{key: k, line: line()})
				cur = &entries[len(entries)-1]
				continue
			}
			if cur == nil {
				continue
			}
			cur.kind = t.Name.Local
			switch t.Name.Local {
			case "string", "integer", "real":
				var v string
				if err := dec.DecodeElement(&v, &t); err != nil {
					return nil, false
				}
				depth--
				cur.value = strings.TrimSpace(v)
			}
			cur = nil
		case xml.EndElement:
			depth--
		}
	}
	return entries, sawDict
}

// xcconfigEntry is one setting a build configuration file assigns.
type xcconfigEntry struct {
	key, value, clause string
}

var xcconfigLine = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)(\[[^\]]*\])*\s*=\s*(.*?)\s*;?\s*$`)

// readXcconfig reads the assignments of a build configuration file; a
// comment starts with // and an #include line names another file.
func readXcconfig(data []byte, file string) []xcconfigEntry {
	var out []xcconfigEntry
	sc := bufio.NewScanner(bytes.NewReader(data))
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		// A value may hold // inside $() as https:/$()/host; only a //
		// outside a value's $() starts a comment.
		if i := strings.Index(line, "//"); i >= 0 && !strings.Contains(line[:i], "$(") {
			line = line[:i]
		}
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if m := xcconfigLine.FindStringSubmatch(line); m != nil {
			out = append(out, xcconfigEntry{key: m[1], value: strings.ReplaceAll(m[3], "$()", ""), clause: fmt.Sprintf("%s:%d", file, n)})
		}
	}
	return out
}

// The keys of Info.plist that the system reads to describe the bundle,
// by their prefixes: Core Foundation's, Foundation's, UIKit's, Launch
// Services' and the like.
var appleKey = regexp.MustCompile(`^(CF|NS|UI|LS|WK|MK|GK|BG|ITS|AV|AppIdentifierPrefix|MinimumOSVersion|DT)`)

// A whole value that names one build setting: $(NAME) or ${NAME}.
var buildReference = regexp.MustCompile(`^\$[({]([A-Za-z_][A-Za-z0-9_]*)[)}]$`)

// swSetting is one read of a setting: in code, or in a file read as data.
type swSetting struct {
	clause, says string
	kind         string // string, bool, int, number, or "" when the read does not say
	value        *yaml.Node
	fromFile     bool
	code         bool
}

// readSettings reads the settings the code reads by a literal name from
// Info.plist and the environment, and Info.plist, .xcconfig and
// entitlements files read as data.
func (sw *swiftReader) readSettings() {
	settings := map[string][]swSetting{}
	reads := 0
	for _, scope := range sortedScopes(sw.byScope) {
		for _, f := range sw.byScope[scope] {
			var key *swValue
			what := ""
			switch {
			case f.Kind == "call" && f.Name == "object" && f.argument("forInfoDictionaryKey") != nil:
				key, what = f.argument("forInfoDictionaryKey"), "Info.plist"
			case f.Kind == "call" && f.Name == "get" && f.baseName() == "Environment" && len(f.Arguments) == 1:
				key, what = &f.Arguments[0].Value, "the environment"
			case f.Kind == "subscript" && strings.HasSuffix(f.baseName(), "infoDictionary"):
				key, what = f.Key, "Info.plist"
			case f.Kind == "subscript" && strings.HasSuffix(f.baseName(), "environment"):
				key, what = f.Key, "the environment"
			default:
				continue
			}
			reads++
			clause := sw.clause(f)
			name, ok := key.stringLiteral()
			if !ok {
				sw.question("should", fmt.Sprintf("%s reads a setting from %s by a key that is not a literal (%s). Which setting is it?", clause, what, key.describe()),
					[]string{"configuration"}, "A key computed at run time is not known by syntax.", sw.at(clause, "Reads a setting from "+what+"."))
				continue
			}
			settings[name] = append(settings[name], swSetting{clause: clause, says: fmt.Sprintf("Reads %s from %s.", name, what), code: true})
		}
	}
	builds := map[string][]xcconfigEntry{}
	var xcconfigs []string
	for _, f := range sw.r.Files {
		if strings.HasSuffix(f, ".xcconfig") && sw.under(f) {
			xcconfigs = append(xcconfigs, f)
			for _, e := range readXcconfig(sw.readData(f, "Build settings, read as data"), f) {
				builds[e.key] = append(builds[e.key], e)
			}
		}
	}
	used := map[string]bool{}
	plists := 0
	for _, f := range sw.r.Files {
		base := path.Base(f)
		if !sw.under(f) || !(base == "Info.plist" || strings.HasSuffix(base, "-Info.plist") || strings.HasSuffix(base, ".entitlements")) {
			continue
		}
		plists++
		entitlements := strings.HasSuffix(base, ".entitlements")
		title := "A property list the app is built with, read as data"
		if entitlements {
			title = "The app's entitlements, read as data"
		}
		entries, ok := readPlist(sw.readData(f, title))
		if !ok {
			sw.gap(f, []string{"configuration"}, "", "it is not a property list in XML, such as a binary one, so its keys are not read")
			continue
		}
		var apple []string
		for _, e := range entries {
			clause := fmt.Sprintf("%s:%d", f, e.line)
			if entitlements || appleKey.MatchString(e.key) {
				apple = append(apple, e.key)
				continue
			}
			s := swSetting{clause: clause, says: fmt.Sprintf("Info.plist sets %s.", e.key), fromFile: true}
			switch e.kind {
			case "string":
				s.kind = "string"
				if m := buildReference.FindStringSubmatch(e.value); m != nil {
					used[m[1]] = true
					s.says = fmt.Sprintf("Info.plist sets %s to the build setting %s.", e.key, m[1])
					var values []string
					for _, b := range builds[m[1]] {
						if !contains(values, b.value) {
							values = append(values, b.value)
						}
					}
					switch {
					case len(values) == 1 && !strings.Contains(values[0], "$"):
						s.value = str(values[0])
					case len(values) > 0:
						var each []string
						for _, b := range builds[m[1]] {
							each = append(each, fmt.Sprintf("%s at %s", b.value, b.clause))
						}
						sw.question("should", fmt.Sprintf("The setting %s takes the build setting %s, which the build configurations set as %s. Which value does each build of the app get?", e.key, m[1], joinAnd(each)),
							[]string{"#/configuration/" + settingName(e.key) + "/schema/default"}, "A setting has one default, and each build configuration may give another value.", sw.at(clause, s.says))
					}
				} else if strings.Contains(e.value, "$") {
					s.says = fmt.Sprintf("Info.plist sets %s to %s, which names build settings.", e.key, e.value)
				} else {
					s.value = str(e.value)
				}
			case "integer":
				s.kind, s.value = "int", literal("!!int", e.value)
			case "real":
				s.kind, s.value = "number", literal("!!float", e.value)
			case "true", "false":
				s.kind, s.value = "bool", literal("!!bool", e.kind)
			default:
				what := map[string]string{"array": "an array", "dict": "a dictionary", "date": "a date", "data": "data"}[e.kind]
				if what == "" {
					what = "a " + e.kind
				}
				sw.gap(clause, []string{"configuration"}, "", "the key %s holds %s, and a setting holds one value; left out", e.key, what)
				continue
			}
			settings[e.key] = append(settings[e.key], s)
		}
		if len(apple) > 0 {
			what := "the keys the system reads to describe the bundle"
			if entitlements {
				what = "the entitlements the system grants the app"
			}
			sw.gap(f, []string{"configuration"}, "", "%s, %s, are not settings the app reads; left out", what, strings.Join(apple, ", "))
		}
	}
	for _, f := range xcconfigs {
		var rest []string
		for k, list := range builds {
			for _, e := range list {
				if strings.HasPrefix(e.clause, f+":") && !used[k] && !contains(rest, k) {
					rest = append(rest, k)
				}
			}
		}
		sort.Strings(rest)
		if len(rest) > 0 {
			sw.gap(f, []string{"configuration"}, "", "the build settings Info.plist does not take, %s, configure the build and not the running app; left out", strings.Join(rest, ", "))
		}
	}
	if reads+plists+len(xcconfigs) > 0 {
		sw.res.say("settings: counted %s, %s and %s: every read of Info.plist or the environment by Bundle, ProcessInfo or Vapor's Environment, every Info.plist and entitlements file, and every .xcconfig file under %s", plural(reads, "read"), plural(plists, "property list"), plural(len(xcconfigs), "build configuration file"), sw.dump.Path)
	}
	sw.writeSettings(settings)
}

// writeSettings writes each setting under configuration by its camelCase
// name, as extract go does: its type and default from the file that holds
// it, and what it is for, and whether it is a secret, as questions.
func (sw *swiftReader) writeSettings(settings map[string][]swSetting) {
	if len(settings) == 0 {
		return
	}
	var keys []string
	for k := range settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	byName := map[string][]string{}
	var names []string
	for _, k := range keys {
		n := settingName(k)
		if byName[n] == nil {
			names = append(names, n)
		}
		byName[n] = append(byName[n], k)
	}
	sort.Strings(names)
	sw.config = mapping()
	var describe, describeBlocks, secrets, secretBlocks, unread, unreadBlocks []string
	for _, n := range names {
		ks := byName[n]
		reads := settings[ks[0]]
		first := reads[0].clause
		if len(ks) > 1 || !memberNameWord.MatchString(n) {
			if len(ks) > 1 {
				sw.question("must", fmt.Sprintf("The settings %s are all named %s in camelCase, and a setting has one name. Which name does each take?", joinAnd(ks), n),
					[]string{"configuration"}, "Two settings the source tells apart would become one.", sw.at(first, "Reads "+ks[0]+"."))
			} else {
				sw.gap(first, []string{"configuration"}, "", "setting %s: no camelCase name can be made from it; left out", ks[0])
			}
			continue
		}
		key := ks[0]
		at := "#/configuration/" + n
		kinds := map[string]bool{}
		var values []*yaml.Node
		inCode := false
		var cites []*yaml.Node
		for _, r := range reads {
			if r.kind != "" {
				kinds[r.kind] = true
			}
			if r.value != nil {
				values = append(values, r.value)
			}
			inCode = inCode || r.code
			cites = append(cites, sw.at(r.clause, r.says))
		}
		schema := mapping()
		switch {
		case len(kinds) > 1:
			var list []string
			for k := range kinds {
				list = append(list, k)
			}
			sort.Strings(list)
			sw.question("must", fmt.Sprintf("The setting %s is given as %s. Which type is it?", key, joinAnd(list)),
				[]string{at + "/schema"}, "A setting has one type, and the files disagree.", sw.at(first, "Names "+key+"."))
		case kinds["string"]:
			set(schema, "type", "string")
		case kinds["bool"]:
			set(schema, "type", "boolean")
		case kinds["number"]:
			set(schema, "type", "number")
			set(schema, "format", "double")
		case kinds["int"]:
			set(schema, "type", "integer")
			sw.question("must", fmt.Sprintf("The setting %s is a whole number. How wide is it?", key),
				[]string{at + "/schema/format"}, "A property list's integer has no width the source states, and every integer in a specification has one.", sw.at(first, "Gives "+key+" as a whole number."))
		default:
			sw.question("must", fmt.Sprintf("Which type is the setting %s? The code reads it by name, and no file read gives its value.", key),
				[]string{at + "/schema"}, "A setting has a type, and nothing read gives it.", sw.at(first, "Reads "+key+"."))
		}
		if !secretWord.MatchString(key) && len(values) > 0 {
			same := true
			for _, o := range values[1:] {
				same = same && o.Value == values[0].Value
			}
			if same {
				set(schema, "default", values[0])
			}
		}
		s := mapping("schema", flow(schema))
		describe = append(describe, key)
		describeBlocks = append(describeBlocks, at+"/description")
		if secretWord.MatchString(key) {
			secrets = append(secrets, key)
			secretBlocks = append(secretBlocks, at+"/secret")
		} else {
			describeBlocks = append(describeBlocks, at+"/secret")
		}
		if !inCode {
			unread = append(unread, key)
			unreadBlocks = append(unreadBlocks, at)
		}
		set(s, "origin", "stated")
		set(s, "cites", cites)
		set(sw.config, n, s)
	}
	if len(describe) > 0 {
		sw.question("must", fmt.Sprintf("What is each setting for, where does its value come from, and is it a secret: %s?", strings.Join(describe, ", ")),
			describeBlocks, "The source names the settings it reads and not what they are for.")
	}
	if len(secrets) > 0 {
		sw.question("must", fmt.Sprintf("Is each of these settings a secret: %s? Each name says it may hold a credential, and the source does not say.", strings.Join(secrets, ", ")),
			secretBlocks, "A secret's value is never written in a specification, and the name alone does not say which a setting is; an app's Info.plist is readable by anyone who has the app.")
	}
	if len(unread) > 0 {
		sw.question("should", fmt.Sprintf("Info.plist sets %s, and no code read reads %s by a literal name. Does the app read %s?", joinAnd(unread), itOrThem(len(unread)), itOrThem(len(unread))),
			unreadBlocks, "A setting a file holds and no code reads may be read by a library, read by a computed name, or unused.")
	}
	if count(sw.config) == 0 {
		sw.config = nil
	}
}

func itOrThem(n int) string {
	if n == 1 {
		return "it"
	}
	return "them"
}

// under says whether a tracked file is under the folder the dump was made
// from.
func (sw *swiftReader) under(f string) bool {
	return sw.dump.Path == "." || strings.HasPrefix(f, sw.dump.Path+"/")
}

// readData reads a tracked file as data and lists it as a clause.
func (sw *swiftReader) readData(f, title string) []byte {
	data, err := os.ReadFile(filepath.Join(sw.r.Repository.Root, filepath.FromSlash(f)))
	if err != nil {
		return nil
	}
	sw.dataRead[f] = title
	return data
}

func sortedScopes(m map[string][]*swFact) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// The Core Data attribute types written as a field.
var coreDataTypes = map[string][]any{
	"String":     {"type", "string"},
	"Boolean":    {"type", "boolean"},
	"Integer 16": {"type", "integer", "format", "int32", "minimum", -32768, "maximum", 32767},
	"Integer 32": {"type", "integer", "format", "int32"},
	"Integer 64": {"type", "integer", "format", "int64"},
	"Double":     {"type", "number", "format", "double"},
	"Date":       {"type", "string", "format", "date-time"},
	"Binary":     {"type", "string", "format", "binary"},
	"UUID":       {"type", "string", "format", "uuid"},
	"URI":        {"type", "string", "format", "uri"},
}

type cdAttr struct {
	XMLName   xml.Name
	Name      string `xml:"name,attr"`
	Type      string `xml:"attributeType,attr"`
	Optional  string `xml:"optional,attr"`
	Default   string `xml:"defaultValueString,attr"`
	Target    string `xml:"destinationEntity,attr"`
	ToMany    string `xml:"toMany,attr"`
	Inverse   string `xml:"inverseName,attr"`
	Transient string `xml:"transient,attr"`
	line      int
}

type cdEntity struct {
	name, clause string
	attrs        []cdAttr
	unique       []string
}

// readCoreData reads each Core Data model as data: its current version's
// entities, attributes and relationships.
func (sw *swiftReader) readCoreData() {
	var contents []string
	for _, f := range sw.r.Files {
		if sw.under(f) && strings.HasSuffix(f, ".xcdatamodel/contents") {
			contents = append(contents, f)
		}
	}
	if len(contents) == 0 {
		return
	}
	// A model with versions names its current one in .xccurrentversion.
	current := map[string]string{}
	for _, f := range sw.r.Files {
		if sw.under(f) && strings.HasSuffix(f, ".xcdatamodeld/.xccurrentversion") {
			entries, _ := readPlist(sw.readData(f, "The current version of a Core Data model, read as data"))
			for _, e := range entries {
				if e.key == "_XCCurrentVersionName" {
					current[path.Dir(f)] = e.value
				}
			}
		}
	}
	read := 0
	for _, f := range contents {
		version := path.Dir(f)
		bundle := path.Dir(version)
		if cur, ok := current[bundle]; ok && path.Base(version) != cur {
			sw.gap(f, []string{"entities"}, "", "a version of the Core Data model %s other than its current one, %s; left out", path.Base(bundle), cur)
			continue
		}
		read++
		entities := parseCoreData(sw.readData(f, "A Core Data model, read as data"), f)
		for _, e := range entities {
			sw.writeCoreDataEntity(e)
		}
	}
	sw.res.say("core data: counted %s: every current version of a Core Data model under %s", plural(read, "model"), sw.dump.Path)
}

func parseCoreData(data []byte, file string) []cdEntity {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var out []cdEntity
	var cur *cdEntity
	for {
		line, _ := dec.InputPos()
		tok, err := dec.Token()
		if err != nil {
			break
		}
		st, ok := tok.(xml.StartElement)
		if !ok {
			if en, ok := tok.(xml.EndElement); ok && en.Name.Local == "entity" {
				cur = nil
			}
			continue
		}
		_ = line
		at, _ := dec.InputPos()
		switch st.Name.Local {
		case "entity":
			name := ""
			for _, a := range st.Attr {
				if a.Name.Local == "name" {
					name = a.Value
				}
			}
			out = append(out, cdEntity{name: name, clause: fmt.Sprintf("%s:%d", file, at)})
			cur = &out[len(out)-1]
		case "attribute", "relationship":
			if cur == nil {
				continue
			}
			var a cdAttr
			if err := dec.DecodeElement(&a, &st); err != nil {
				continue
			}
			a.line = at
			cur.attrs = append(cur.attrs, a)
		case "constraint":
			if cur == nil {
				continue
			}
			for _, a := range st.Attr {
				if a.Name.Local == "value" {
					cur.unique = append(cur.unique, a.Value)
				}
			}
		}
	}
	return out
}

func (sw *swiftReader) writeCoreDataEntity(e cdEntity) {
	name := e.name
	if !typeNameWord.MatchString(name) || sw.entityNames[name] {
		sw.gap(e.clause, []string{"entities"}, "", "the Core Data entity %s: its name is not a PascalCase name, or another model has it; left out", name)
		return
	}
	file := strings.SplitN(e.clause, ":", 2)[0]
	at := "#/entities/" + name
	props, relations := mapping(), mapping()
	var required, relAsk []string
	for _, a := range e.attrs {
		clause := fmt.Sprintf("%s:%d", file, a.line)
		if !memberNameWord.MatchString(a.Name) {
			sw.gap(clause, []string{at}, "", "%s.%s: the name is not a camelCase name a field can take; left out", name, a.Name)
			continue
		}
		if a.Transient == "YES" {
			sw.gap(clause, []string{at}, "", "%s.%s is transient, so Core Data does not store it; left out", name, a.Name)
			continue
		}
		if a.XMLName.Local == "relationship" {
			many := "one"
			if a.ToMany == "YES" {
				many = "many"
			}
			says := fmt.Sprintf("%s.%s relates to %s %s", name, a.Name, many, a.Target)
			if a.Inverse != "" {
				says += ", whose inverse is " + a.Inverse
			}
			set(relations, a.Name, mapping("target", a.Target, "origin", "stated", "cites", []*yaml.Node{citation(sw.key, clause, says+".")}))
			relAsk = append(relAsk, at+"/relations/"+a.Name+"/kind", at+"/relations/"+a.Name+"/via")
			continue
		}
		fat := at + "/properties/" + a.Name
		var node *yaml.Node
		if pairs, ok := coreDataTypes[a.Type]; ok {
			node = fieldOf(pairs)
		} else {
			node = flow(mapping())
			what := "Which type, precision and scale is it?"
			if a.Type != "Decimal" {
				what = "Which type is it?"
			}
			if a.Type == "Float" {
				node = fieldOf([]any{"type", "number"})
				sw.question("must", fmt.Sprintf("%s.%s at %s is a Core Data Float, 32 bits wide, and a number in the meta-model is a double. Is a double right here?", name, a.Name, clause),
					[]string{fat + "/format"}, "A Float has a width the meta-model's number formats do not name.", citation(sw.key, clause, "attributeType "+a.Type+"."))
			} else {
				sw.question("must", fmt.Sprintf("%s.%s at %s has the Core Data type %s. %s", name, a.Name, clause, a.Type, what),
					[]string{fat}, "The attribute type does not say a type the meta-model holds.", citation(sw.key, clause, "attributeType "+a.Type+"."))
			}
		}
		if a.Optional == "YES" {
			node = nullable(node)
		} else {
			required = append(required, a.Name)
		}
		if a.Default != "" {
			format := scalar(child(node, "format"))
			if v := coreDataDefault(child(node, "type"), a.Default); v != nil && (format == "" || format == "int32" || format == "int64" || format == "double") {
				set(node, "default", v)
			} else {
				sw.gap(clause, []string{fat}, "", "%s.%s has the default %q, which is not a value its field's type takes as written; left out", name, a.Name, a.Default)
			}
		}
		set(props, a.Name, node)
	}
	if len(props.Content) == 0 && len(relations.Content) == 0 {
		sw.gap(e.clause, []string{"entities"}, "", "the Core Data entity %s has no attribute a field can hold; left out", name)
		return
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
	says := fmt.Sprintf("The Core Data model declares the entity %s.", name)
	set(el, "origin", "stated")
	set(el, "cites", []*yaml.Node{citation(sw.key, e.clause, says)})
	if sw.entities == nil {
		sw.entities = mapping()
	}
	set(sw.entities, name, el)
	sw.entityNames[name] = true
	key := "Core Data identifies a record by its object identifier, which no attribute declares"
	if len(e.unique) > 0 {
		key += fmt.Sprintf(", and the model makes %s unique", joinAnd(e.unique))
	}
	sw.question("must", fmt.Sprintf("Which field or fields identify a record of %s? %s.", name, key),
		[]string{at + "/primaryKey"}, "A primary key is what a record is found by, and a Core Data entity declares none.", citation(sw.key, e.clause, says))
	if len(relAsk) > 0 {
		sw.question("must", fmt.Sprintf("For each relation of %s, which kind is it (one-to-one, one-to-many, many-to-one or many-to-many), and which field holds the key or points back?", name),
			relAsk, "Core Data keeps a relationship by the related object, with no attribute that holds its key.", citation(sw.key, e.clause, says))
	}
}

// coreDataDefault is an attribute's default as a value of its field's
// type, or nil when the type takes none as Core Data writes it.
func coreDataDefault(t *yaml.Node, v string) *yaml.Node {
	if t == nil {
		return nil
	}
	kind := t.Value
	if t.Kind == yaml.SequenceNode && len(t.Content) > 0 {
		kind = t.Content[0].Value
	}
	switch kind {
	case "string":
		return str(v)
	case "integer":
		if _, err := fmt.Sscanf(v, "%d", new(int64)); err == nil && !strings.ContainsAny(v, ".eE") {
			return literal("!!int", v)
		}
	case "number":
		var f float64
		if _, err := fmt.Sscanf(v, "%g", &f); err == nil {
			return literal("!!float", v)
		}
	case "boolean":
		switch v {
		case "YES":
			return value(true)
		case "NO":
			return value(false)
		}
	}
	return nil
}
