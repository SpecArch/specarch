package gentests

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/xref"
)

// Integration is what the integration tests of a specification are made
// from: the calls it makes into other modules of its estate, grouped by
// the module called, and the calls into modules the run does not give,
// which are named and not written (ADR-226).
type Integration struct {
	Caller        string // the calling module's info.name
	CallerVersion string
	Modules       []ModuleCalls
	Unwritten     []Unwritten
}

// Any says whether the specification calls another module at all, so a
// file whose specification calls none carries no integration harness.
func (in Integration) Any() bool { return len(in.Modules) > 0 || len(in.Unwritten) > 0 }

// ModuleCalls are the calls into one module, written into one file of
// their own so a change in that module's contract changes that file only.
type ModuleCalls struct {
	Module  string // the called module's info.name
	Version string // its info.version
	Calls   []ModuleCall
}

// ModuleCall is one operation of another module the specification calls,
// from every place that calls it, with the paths its contract implies.
type ModuleCall struct {
	Name       string // the operation's name in the test's: the module and the operationId
	Dependency string // the dependency's key in the calling specification
	Module     string
	Version    string
	Operation  string // the operationId in the called module
	Method     string // upper case
	Path       string // as the called module writes it
	Pointer    string // the operation's pointer in the called module, as module#/paths/...
	Timeout    string // the calling module's time limit for the dependency, "" for none
	Permission string // the operation's permission, "" for a public one
	Places     []string
	Input      []Field // the called module's golden input, or none
	InputFrom  string  // the golden design test the input is taken from, "" for none
	Paths      []CallPath
}

// CallPath is one path of a call: the golden one or a red one.
type CallPath struct {
	Name     string // golden, denied without <permission>, missing <field>, answers <status>, fails, times out
	Scenario string // golden or red
	Holding  bool   // the caller holds the operation's permission
	Omit     string // the required field left out, or ""
	Arrange  string // the status the harness arranges the called module to answer, or ""
	Break    string // fails or times out, or ""
	Expect   string // status, refused, failed or late
	Status   string // the status checked, for status
	Fields   []string
	List     bool // the success answer is a list of records, each with Fields
}

// Unwritten is a call into a module the run does not give.
type Unwritten struct {
	Place     string // the calling element's pointer
	Reference string // the reference as written
	Module    string // the module's info.name the dependency names
	Reason    string
}

// BuildIntegration reads the calls a specification makes into the other
// modules of its estate. A call is an operation of another module named by
// a calls entry of an operation, a job or a gateway, or by the source of a
// list page or a picker, through a dependency that names the module's
// specification; one operation is one call, from every place naming it.
func BuildIntegration(r *Request) Integration {
	info, _ := r.Specification["info"].(map[string]any)
	in := Integration{Caller: str(info, "name"), CallerVersion: str(info, "version")}
	deps, _ := r.Specification["dependencies"].(map[string]any)
	byModule := map[string]*ModuleCalls{}
	byCall := map[string]int{}
	for _, p := range callPlaces(r.Specification) {
		dep, ptr, ok := xref.Parse(p.ref)
		if !ok {
			continue
		}
		d, _ := deps[dep].(map[string]any)
		module := str(d, "specification")
		if module == "" || len(ptr) != 3 || ptr[0] != "paths" {
			continue
		}
		op, path, method, given, found := xref.Operation(r.Dependencies, p.ref)
		if _, ok := r.Dependencies[dep]; !ok {
			in.Unwritten = append(in.Unwritten, Unwritten{Place: p.place, Reference: p.ref, Module: module,
				Reason: fmt.Sprintf("the run does not give %s; give its estate with --estate", module)})
			continue
		}
		if !found {
			in.Unwritten = append(in.Unwritten, Unwritten{Place: p.place, Reference: p.ref, Module: module,
				Reason: fmt.Sprintf("%s %s gives no such operation", module, given.Version)})
			continue
		}
		key := given.Specification + "#" + path + "#" + method
		mc := byModule[given.Specification]
		if i, ok := byCall[key]; ok {
			mc.Calls[i].Places = append(mc.Calls[i].Places, p.place)
			continue
		}
		if mc == nil {
			mc = &ModuleCalls{Module: given.Specification, Version: given.Version}
			byModule[given.Specification] = mc
		}
		c := moduleCall(dep, str(d, "timeout"), given, op, path, method)
		c.Places = []string{p.place}
		mc.Calls = append(mc.Calls, c)
		byCall[key] = len(mc.Calls) - 1
	}
	var names []string
	for n := range byModule {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		mc := byModule[n]
		sort.SliceStable(mc.Calls, func(i, j int) bool { return mc.Calls[i].Name < mc.Calls[j].Name })
		in.Modules = append(in.Modules, *mc)
	}
	return in
}

type callPlace struct{ place, ref string }

// callPlaces are the references to other modules' operations a
// specification makes, with the pointer of each, in document order: the
// calls entries of each operation, job and gateway, and the source of each
// page and picker.
func callPlaces(s map[string]any) []callPlace {
	var out []callPlace
	calls := func(at string, el map[string]any) {
		list, _ := el["calls"].([]any)
		for i, c := range list {
			switch e := c.(type) {
			case string:
				out = append(out, callPlace{fmt.Sprintf("#%s/calls/%d", at, i), e})
			case map[string]any:
				if ref := str(e, "call"); ref != "" {
					out = append(out, callPlace{fmt.Sprintf("#%s/calls/%d/call", at, i), ref})
				}
			}
		}
	}
	paths, _ := s["paths"].(map[string]any)
	for _, p := range sortedKeys(paths) {
		item, _ := paths[p].(map[string]any)
		for _, m := range []string{"get", "post", "put", "patch", "delete"} {
			if op, ok := item[m].(map[string]any); ok {
				calls("/paths/"+escape(p)+"/"+m, op)
			}
		}
	}
	for _, section := range []string{"jobs", "gateways"} {
		els, _ := s[section].(map[string]any)
		for _, n := range sortedKeys(els) {
			el, _ := els[n].(map[string]any)
			calls("/"+section+"/"+escape(n), el)
		}
	}
	pages, _ := s["pages"].(map[string]any)
	for _, n := range sortedKeys(pages) {
		page, _ := pages[n].(map[string]any)
		if src := str(page, "source"); src != "" {
			out = append(out, callPlace{"#/pages/" + escape(n) + "/source", src})
		}
		pickers, _ := page["pickers"].(map[string]any)
		for _, f := range sortedKeys(pickers) {
			pk, _ := pickers[f].(map[string]any)
			if src := str(pk, "source"); src != "" {
				out = append(out, callPlace{"#/pages/" + escape(n) + "/pickers/" + escape(f) + "/source", src})
			}
		}
	}
	return out
}

// moduleCall is one call with the paths the called operation's contract
// implies, each checking the status the operation declares for it, as
// derive drafts it (docs/test-generation.md): the success status, 403 for
// a denied call, 400, or 422 when there is no 400, for a missing field;
// a refusal when it declares none.
func moduleCall(dep, timeout string, d xref.Dependency, op map[string]any, path, method string) ModuleCall {
	g := &walker{spec: d.Content, mappings: map[string]string{}}
	id := str(op, "operationId")
	c := ModuleCall{Name: d.Specification + " " + id, Dependency: dep, Module: d.Specification, Version: d.Version, Operation: id,
		Method: strings.ToUpper(method), Path: path, Pointer: d.Specification + "#/paths/" + escape(path) + "/" + method,
		Timeout: timeout, Permission: str(op, "permission")}
	if c.Permission == "public" {
		c.Permission = ""
	}
	_, _, _, fields := g.operation(id)
	tests, _ := d.Content["tests"].(map[string]any)
	for _, name := range sortedKeys(tests) {
		t, _ := tests[name].(map[string]any)
		input, ok := t["input"].(map[string]any)
		if ok && str(t, "operation") == id && str(t, "scenario") == "golden" {
			c.Input, c.InputFrom = g.record(input, fields), d.Specification+"#/tests/"+escape(name)
			break
		}
	}
	responses, _ := op["responses"].(map[string]any)
	declared := func(statuses ...string) string {
		for _, s := range statuses {
			if _, ok := responses[s]; ok {
				return s
			}
		}
		return ""
	}
	checked := func(p CallPath, status string) CallPath {
		if status == "" {
			p.Expect = "refused"
		} else {
			p.Expect, p.Status = "status", status
		}
		return p
	}
	golden := CallPath{Name: "golden", Scenario: "golden", Holding: true, Expect: "status"}
	for _, s := range sortedKeys(responses) {
		if strings.HasPrefix(s, "2") {
			golden.Status = s
			golden.Fields, golden.List = g.answerFields(responses[s])
			break
		}
	}
	if golden.Status == "" {
		golden.Expect = "answered"
	}
	c.Paths = append(c.Paths, golden)
	used := map[string]bool{}
	if c.Permission != "" {
		status := declared("403")
		used[status] = true
		c.Paths = append(c.Paths, checked(CallPath{Name: "denied without " + c.Permission, Scenario: "red"}, status))
	}
	if required := g.requiredFields(op, path); len(required) > 0 {
		status := declared("400", "422")
		used[status] = true
		for _, f := range required {
			c.Paths = append(c.Paths, checked(CallPath{Name: "missing " + f, Scenario: "red", Holding: true, Omit: f}, status))
		}
	}
	for _, s := range sortedKeys(responses) {
		if strings.HasPrefix(s, "2") || s == "default" || used[s] {
			continue
		}
		c.Paths = append(c.Paths, CallPath{Name: "answers " + s, Scenario: "red", Holding: true, Arrange: s, Expect: "status", Status: s})
	}
	c.Paths = append(c.Paths,
		CallPath{Name: "fails", Scenario: "red", Holding: true, Break: "fails", Expect: "failed"},
		CallPath{Name: "times out", Scenario: "red", Holding: true, Break: "times out", Expect: "late"})
	return c
}

// requiredFields are the fields a request of the operation may leave
// out and must not: its required query and header parameters and the
// required fields of its body, sorted. A path parameter is not one, since
// a request without it is a request for another path, and a field the
// server writes (readOnly) is not sent.
func (g *walker) requiredFields(op map[string]any, path string) []string {
	var out []string
	item, _ := mapAt(g.spec, "paths", path).(map[string]any)
	for _, list := range []any{item["parameters"], op["parameters"]} {
		params, _ := list.([]any)
		for _, prm := range params {
			pm, _ := prm.(map[string]any)
			if in := str(pm, "in"); (in == "query" || in == "header") && pm["required"] == true {
				out = append(out, str(pm, "name"))
			}
		}
	}
	rb, _ := op["requestBody"].(map[string]any)
	content, _ := rb["content"].(map[string]any)
	for _, ct := range sortedKeys(content) {
		cm, _ := content[ct].(map[string]any)
		schema, _ := cm["schema"].(map[string]any)
		props := g.schemaFields(schema)
		for _, f := range g.required(schema) {
			if p := props[f]; p == nil || p["readOnly"] != true {
				out = append(out, f)
			}
		}
		break
	}
	sort.Strings(out)
	return out
}

// required is the required list of a schema, through its $ref.
func (g *walker) required(schema map[string]any) []string {
	if ref := str(schema, "$ref"); strings.HasPrefix(ref, "#/entities/") || strings.HasPrefix(ref, "#/schemas/") {
		section, name, _ := strings.Cut(strings.TrimPrefix(ref, "#/"), "/")
		schema, _ = mapAt(g.spec, section, name).(map[string]any)
	}
	var out []string
	list, _ := schema["required"].([]any)
	for _, f := range list {
		if s, ok := f.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// answerFields are the fields a success answer must hold, and whether it
// is a list of records that each hold them: an object's required fields,
// or those of a list's items.
func (g *walker) answerFields(response any) ([]string, bool) {
	r, _ := response.(map[string]any)
	content, _ := r["content"].(map[string]any)
	for _, ct := range sortedKeys(content) {
		cm, _ := content[ct].(map[string]any)
		schema, _ := cm["schema"].(map[string]any)
		if str(schema, "type") == "array" {
			items, _ := schema["items"].(map[string]any)
			f := g.required(items)
			sort.Strings(f)
			return f, true
		}
		f := g.required(schema)
		sort.Strings(f)
		return f, false
	}
	return nil, false
}

// snake is a module's name as a file name takes it: catalogue-service as
// catalogue_service.
func snake(s string) string { return strings.ReplaceAll(s, "-", "_") }
