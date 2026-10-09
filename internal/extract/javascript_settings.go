package extract

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// jsSetting is one read of a setting.
type jsSetting struct {
	clause, says, kind string
	value              *yaml.Node
}

// readSettings reads the environment the code reads by a literal name:
// process.env.NAME and import.meta.env.NAME, with a literal default, and
// the type a wrapping call or a comparison gives it.
func (js *jsReader) readSettings() {
	settings := map[string][]jsSetting{}
	n := 0
	for _, f := range js.accesses {
		if len(f.Chain) < 2 || f.Chain[1] != "env" || (f.Chain[0] != "process" && f.Chain[0] != "import.meta") {
			continue
		}
		clause := js.clause(f)
		root := strings.Join(f.Chain[:2], ".")
		if len(f.Chain) == 2 {
			if f.WrappedBy == "" && f.Default == nil {
				continue
			}
			n++
			js.question("should", fmt.Sprintf("%s reads %s whole. Which settings does it read there?", clause, root),
				[]string{"configuration"}, "A setting is read by its name, and the environment read whole names none.", js.at(clause, "Reads "+root+"."))
			continue
		}
		n++
		key := f.Chain[2]
		if key == "[computed]" {
			js.question("should", fmt.Sprintf("%s reads %s by a name that is not a literal. Which setting does it read?", clause, root),
				[]string{"configuration"}, "A name computed at run time is not known by syntax.", js.at(clause, "Reads "+root+" by a computed name."))
			continue
		}
		s := jsSetting{clause: clause, says: fmt.Sprintf("Reads %s.%s.", root, key)}
		switch w := f.WrappedBy; {
		case w == "Number" || w == "parseFloat" || w == "Number.parseFloat":
			s.kind = "number"
		case w == "parseInt" || w == "Number.parseInt":
			s.kind = "int"
		case w == "Boolean":
			s.kind = "bool"
		}
		if c := f.Compared; c != nil {
			if v, ok := c.stringLiteral(); ok && (v == "true" || v == "false" || v == "1" || v == "0") {
				s.kind = "bool"
			}
		}
		if d := f.Default; d != nil {
			if lit := jsLiteralNode(d); lit != nil {
				s.value = lit
				if s.kind == "" {
					switch {
					case d.String != nil:
						s.kind = "string"
					case d.Number != nil:
						s.kind = "number"
					case d.Boolean != nil:
						s.kind = "bool"
					}
				}
				s.says = fmt.Sprintf("Reads %s.%s, %s when it is not set.", root, key, d.describe())
			}
		}
		// A wrapped default is the number the string gives: Number(x ?? '8080').
		if s.value != nil && s.value.Tag == "!!str" && (s.kind == "number" || s.kind == "int") {
			if parsed := yamlNumber(s.value.Value); parsed != s.value.Value {
				node := &yaml.Node{}
				_ = node.Encode(parsed)
				s.value = node
			} else {
				s.value = nil
			}
		}
		settings[key] = append(settings[key], s)
	}
	if n > 0 {
		js.res.say("settings: counted %s: every read of process.env or import.meta.env", plural(n, "read"))
	}
	js.writeSettings(settings)
}

func (js *jsReader) writeSettings(settings map[string][]jsSetting) {
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
	js.config = mapping()
	var describe, describeBlocks, secrets, secretBlocks []string
	for _, n := range names {
		ks := byName[n]
		reads := settings[ks[0]]
		first := reads[0].clause
		if len(ks) > 1 || !memberNameWord.MatchString(n) {
			if len(ks) > 1 {
				js.question("must", fmt.Sprintf("The settings %s are all named %s in camelCase, and a setting has one name. Which name does each take?", joinAnd(ks), n),
					[]string{"configuration"}, "Two settings the source tells apart would become one.", js.at(first, "Reads "+ks[0]+"."))
			} else {
				js.gap(first, []string{"configuration"}, "", "setting %s: no camelCase name can be made from it; left out", ks[0])
			}
			continue
		}
		key := ks[0]
		at := "#/configuration/" + n
		kinds := map[string]bool{}
		var values []*yaml.Node
		var cites []*yaml.Node
		for _, r := range reads {
			if r.kind != "" {
				kinds[r.kind] = true
			}
			if r.value != nil {
				values = append(values, r.value)
			}
			cites = append(cites, js.at(r.clause, r.says))
		}
		schema := mapping()
		switch {
		case len(kinds) > 1:
			var list []string
			for k := range kinds {
				list = append(list, k)
			}
			sort.Strings(list)
			js.question("must", fmt.Sprintf("The setting %s is read as %s. Which type is it?", key, joinAnd(list)),
				[]string{at + "/schema"}, "A setting has one type, and the reads disagree.", js.at(first, "Reads "+key+"."))
		case kinds["string"]:
			set(schema, "type", "string")
		case kinds["bool"]:
			set(schema, "type", "boolean")
		case kinds["number"]:
			set(schema, "type", "number")
			js.question("must", fmt.Sprintf("The setting %s is read as a number, and a JavaScript number has no width. Which width does it take: int32, int64 within 2^53, or a double?", key),
				[]string{at + "/schema/format"}, "Every number in a specification has a width, and the code states none.", js.at(first, "Reads "+key+" as a number."))
		case kinds["int"]:
			set(schema, "type", "integer")
			js.question("must", fmt.Sprintf("The setting %s is read as a whole number. How wide is it?", key),
				[]string{at + "/schema/format"}, "Every integer in a specification has a width, and the code states none.", js.at(first, "Reads "+key+" as a whole number."))
		default:
			js.question("must", fmt.Sprintf("Which type is the setting %s? The code reads it by name, and nothing read gives its value or a type.", key),
				[]string{at + "/schema"}, "A setting has a type, and nothing read gives it.", js.at(first, "Reads "+key+"."))
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
		set(s, "origin", "stated")
		set(s, "cites", cites)
		set(js.config, n, s)
	}
	if len(describe) > 0 {
		js.question("must", fmt.Sprintf("What is each setting for, where does its value come from, and is it a secret: %s?", strings.Join(describe, ", ")),
			describeBlocks, "The source names the settings it reads and not what they are for.")
	}
	if len(secrets) > 0 {
		js.question("must", fmt.Sprintf("Is each of these settings a secret: %s? Each name says it may hold a credential, and the source does not say.", strings.Join(secrets, ", ")),
			secretBlocks, "A secret's value is never written in a specification, and the name alone does not say which a setting is; a value import.meta.env gives a browser is readable by anyone who loads the page.")
	}
	if count(js.config) == 0 {
		js.config = nil
	}
}

// jsClient is one call to another system a client makes.
type jsClient struct {
	clause, method, url, dependency, says string
}

// readClients reads the calls fetch and axios make with a URL built from
// literal parts.
func (js *jsReader) readClients() {
	// The axios instances axios.create makes, by position, and the base
	// URL each is given.
	instances := map[string]*jsValue{}
	for _, f := range js.calls {
		if f.Callee == nil || f.Callee.Member == nil || f.Callee.Member.Name != "create" || f.AssignedTo == "" {
			continue
		}
		if _, ok := js.fromLibrary(&f.Callee.Member.Object, "axios"); !ok {
			continue
		}
		var base *jsValue
		if len(f.Arguments) > 0 {
			base = f.Arguments[0].prop("baseURL")
		}
		for k, vr := range js.variables {
			if vr.File == f.File && vr.Line == f.Line && vr.Name == f.AssignedTo {
				if base == nil {
					base = &jsValue{}
				}
				instances[k] = base
			}
		}
	}
	var clients []jsClient
	n := 0
	for _, f := range js.calls {
		if f.Callee == nil {
			continue
		}
		var method, how string
		var u *jsValue
		var base *jsValue
		switch {
		case f.Callee.Name != nil && *f.Callee.Name == "fetch" && f.Callee.Declaration == nil && f.Callee.Import == nil:
			if len(f.Arguments) == 0 {
				continue
			}
			u = &f.Arguments[0]
			method, how = "GET", "fetch"
			if len(f.Arguments) > 1 {
				if m := f.Arguments[1].prop("method"); m != nil {
					s, ok := js.constString(m)
					if !ok {
						n++
						js.question("should", fmt.Sprintf("%s calls fetch with the method %s, which is not a literal. Which method does it call?", js.clause(f), m.describe()),
							[]string{"dependencies"}, "A method computed at run time is not known by syntax.", js.at(js.clause(f), "Calls fetch."))
						continue
					}
					method = strings.ToUpper(s)
				}
			}
		case f.Callee.Name != nil:
			imported, ok := js.fromLibrary(f.Callee, "axios")
			if !ok || (imported != "default" && imported != "*") || len(f.Arguments) == 0 {
				continue
			}
			o := &f.Arguments[0]
			how = "axios"
			if o.Object != nil {
				u = o.prop("url")
				method = "GET"
				if m := o.prop("method"); m != nil {
					s, ok := js.constString(m)
					if !ok {
						method = ""
					}
					method = strings.ToUpper(s)
				}
				base = o.prop("baseURL")
			} else {
				u, method = o, "GET"
			}
		case f.Callee.Member != nil:
			name := f.Callee.Member.Name
			m, isMethod := jsRouteMethods[name]
			if !isMethod || name == "all" || name == "del" || len(f.Arguments) == 0 {
				if name != "request" {
					continue
				}
			}
			obj := &f.Callee.Member.Object
			if imported, ok := js.fromLibrary(obj, "axios"); ok && (imported == "default" || imported == "*") {
				how = "axios." + name
			} else if b, ok := instances[obj.Declaration.key()]; ok && obj.Declaration != nil {
				how, base = obj.describe()+"."+name, b
			} else {
				continue
			}
			if name == "request" {
				if len(f.Arguments) == 0 {
					continue
				}
				o := &f.Arguments[0]
				u = o.prop("url")
				m = "GET"
				if mv := o.prop("method"); mv != nil {
					s, _ := js.constString(mv)
					m = strings.ToUpper(s)
				}
			} else {
				u = &f.Arguments[0]
			}
			method = m
		default:
			continue
		}
		n++
		clause := js.clause(f)
		if method == "" {
			js.question("should", fmt.Sprintf("%s calls another system with %s by a method that is not a literal. Which method does it call?", clause, how),
				[]string{"dependencies"}, "A method computed at run time is not known by syntax.", js.at(clause, "Calls "+how+"."))
			continue
		}
		text, params, ok := js.urlTemplate(u)
		if ok && base != nil && base.String == nil && base.Template == nil && base.Name == nil && base.Member == nil {
			base = nil
		}
		if ok && base != nil {
			bt, bp, bok := js.urlTemplate(base)
			if !bok {
				js.question("should", fmt.Sprintf("%s calls %s %s with %s, whose base URL is %s, which is not built from literal parts. Which system does it call?", clause, method, text, how, base.describe()),
					[]string{"dependencies"}, "A dependency is named by the system it calls, and a base URL computed at run time names none by syntax.", js.at(clause, fmt.Sprintf("Calls %s %s.", method, text)))
				continue
			}
			text = strings.TrimSuffix(bt, "/") + "/" + strings.TrimPrefix(text, "/")
			params = append(bp, params...)
		}
		if u == nil || !ok {
			js.question("should", fmt.Sprintf("%s calls another system with %s at %s, built from parts the reader cannot place. Which system and operation does it call?", clause, how, u.describe()),
				[]string{"dependencies"}, "A URL is read when its host is a literal and each value fills one whole path segment.", js.at(clause, "Calls "+how+" at "+u.describe()+"."))
			continue
		}
		parsed, err := url.Parse(strings.NewReplacer("{", "", "}", "").Replace(text))
		if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
			js.question("should", fmt.Sprintf("%s calls %s %s, which names no host. Which system does it call, and is it an operation of this system?", clause, method, text),
				[]string{"dependencies"}, "A dependency is named by the system it calls, and the URL does not name one; a path of this system's own is joined to its operation by method and path only once a question can carry them.", js.at(clause, fmt.Sprintf("Calls %s %s.", method, text)))
			continue
		}
		dep := camel(strings.ToLower(hostWord.ReplaceAllString(parsed.Hostname(), "_")))
		says := fmt.Sprintf("Calls %s %s with %s.", method, text, how)
		clients = append(clients, jsClient{clause: clause, method: method, url: text, dependency: dep, says: says})
		if len(params) > 0 {
			var holes []string
			for _, p := range params {
				holes = append(holes, "{"+p+"}")
			}
			js.question("should", fmt.Sprintf("%s calls %s %s, filling %s from values of the code. What does each hold?", clause, method, text, joinAnd(holes)),
				[]string{"#/dependencies/" + dep}, "A part of a URL interpolated from a value is a parameter, and the code says only which variable fills it.", js.at(clause, says))
		}
	}
	if n > 0 {
		js.res.say("clients: counted %s: every call of fetch, of axios and its get, post, put, patch, delete, head, options and request, and of an instance axios.create makes", plural(n, "call"))
	}
	if len(clients) == 0 {
		return
	}
	byName := map[string][]jsClient{}
	var names []string
	for _, c := range clients {
		if byName[c.dependency] == nil {
			names = append(names, c.dependency)
		}
		byName[c.dependency] = append(byName[c.dependency], c)
	}
	sort.Strings(names)
	js.deps = mapping()
	for _, name := range names {
		var cites []*yaml.Node
		for _, c := range byName[name] {
			cites = append(cites, js.at(c.clause, c.says))
		}
		at := "#/dependencies/" + name
		set(js.deps, name, mapping("origin", "stated", "cites", cites))
		js.question("must", fmt.Sprintf("What is the system %s the source calls, and how long may one call to it take before the caller gives up?", name),
			[]string{at + "/description", at + "/timeout"}, "The source names the URL it calls, and not what the system is or a time limit, which every dependency has.")
	}
}

// urlTemplate writes a URL as a template: a literal, a const of the
// files read, or a template literal whose values are consts or fill one
// whole path segment each, as the parameter {name}.
func (js *jsReader) urlTemplate(v *jsValue) (string, []string, bool) {
	if v == nil {
		return "", nil, false
	}
	if s, ok := js.constString(v); ok {
		return s, nil, true
	}
	if v.Template == nil {
		return "", nil, false
	}
	var b strings.Builder
	var params []string
	for i, p := range v.Template {
		if p.Text != nil {
			b.WriteString(*p.Text)
			continue
		}
		if s, ok := js.constString(p.Expression); ok {
			b.WriteString(s)
			continue
		}
		name := ""
		switch {
		case p.Expression.Name != nil:
			name = *p.Expression.Name
		case p.Expression.Member != nil:
			name = p.Expression.Member.Name
		}
		before := b.String()
		next := ""
		if i+1 < len(v.Template) && v.Template[i+1].Text != nil {
			next = *v.Template[i+1].Text
		}
		if name == "" || !memberNameWord.MatchString(name) || !strings.HasSuffix(before, "/") || (strings.Contains(before, "://") && strings.Count(before, "/") < 3) ||
			(next != "" && !strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "?")) {
			return "", nil, false
		}
		params = append(params, name)
		b.WriteString("{" + name + "}")
	}
	return b.String(), params, true
}
