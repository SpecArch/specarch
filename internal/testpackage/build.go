// Package testpackage writes and runs a test package: every test of a
// specification and every derived case no test covers yet, as requests
// made from outside against one environment, with the results written as
// JUnit XML (ADR-233). The package holds what the run needs and nothing of
// the specification's files, so it can be handed to whoever runs the
// environment; credentials and the records a case needs are given when it
// runs, never written into it.
package testpackage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/gentests"
	"github.com/SpecArch/specarch/internal/wirename"
)

// FileName is the package's file in its folder.
const FileName = "specarch-test-package.yaml"

// Format is the version of the package's format.
const Format = "1.0.0"

// Package is a test package, as its file holds it.
type Package struct {
	Format        string        `yaml:"specarchTestPackage"`
	Specification Specification `yaml:"specification"`
	Environment   string        `yaml:"environment"`
	Servers       []Server      `yaml:"servers"`
	Cases         []Case        `yaml:"cases"`
	Held          []Held        `yaml:"held,omitempty"`
	// LeftOut are the derived cases of rank other no test covers, which
	// the test plan lists as left out: named, not run, so none is dropped
	// without a word.
	LeftOut []LeftOut `yaml:"leftOut,omitempty"`
}

// LeftOut is a derived case left out, with why.
type LeftOut struct {
	Subject string `yaml:"subject"`
	Case    string `yaml:"case"`
	Why     string `yaml:"why"`
}

// Specification names the specification the package was made from.
type Specification struct {
	Title     string `yaml:"title"`
	Version   string `yaml:"version"`
	MetaModel string `yaml:"metaModel"`
	Digest    string `yaml:"digest"`
	Draft     string `yaml:"draft,omitempty"`
}

// Server is a server of a deployment in the package's environment that
// answers requests: the place a run sends them, unless it is given another.
type Server struct {
	Deployment string `yaml:"deployment"`
	Name       string `yaml:"name"`
	URL        string `yaml:"url"`
}

// Case is one path through the system: a test of the specification, or a
// derived case no test covers, drafted as specarch derive drafts it.
type Case struct {
	Name     string   `yaml:"name"`
	Element  string   `yaml:"element"`
	Derived  bool     `yaml:"derived,omitempty"`
	Subject  string   `yaml:"subject,omitempty"`
	Scenario string   `yaml:"scenario,omitempty"`
	Covers   []string `yaml:"covers,omitempty"`
	Given    string   `yaml:"given,omitempty"`
	When     string   `yaml:"when,omitempty"`
	Then     string   `yaml:"then,omitempty"`
	// Skip says why the case cannot be run from outside; a run reports it
	// as skipped with this reason.
	Skip   string `yaml:"skip,omitempty"`
	Caller string `yaml:"caller,omitempty"`
	// Records are the records the case needs to exist before its request,
	// as JSON: entity name to a list of records.
	Records   string   `yaml:"records,omitempty"`
	Request   *Request `yaml:"request,omitempty"`
	Expect    *Expect  `yaml:"expect,omitempty"`
	Unchecked []string `yaml:"unchecked,omitempty"`
}

// Request is the request a case makes, relative to the server's address.
type Request struct {
	Method  string            `yaml:"method"`
	Path    string            `yaml:"path"`
	Headers map[string]string `yaml:"headers,omitempty"`
	// Body is the request body as JSON, or empty for none.
	Body string `yaml:"body,omitempty"`
}

// Expect is what the answer must show.
type Expect struct {
	Status int `yaml:"status,omitempty"`
	// Body is the fields of the answer's body that matter, as a JSON
	// object; the answer may carry more.
	Body string `yaml:"body,omitempty"`
	// Numeric names the body fields compared as numbers when both sides
	// are text, a decimal or an integer carried as a string.
	Numeric []string `yaml:"numeric,omitempty"`
}

// Held is a test left out of the package because an open question holds
// it up; it joins the package once the question is answered.
type Held struct {
	Name      string   `yaml:"name"`
	Element   string   `yaml:"element"`
	Questions []string `yaml:"questions"`
}

// Draft is a derived case no test covers, as derive drafts it: the test's
// plain values, read as the specification's tests are.
type Draft struct {
	Name      string
	Element   string
	Test      map[string]any
	BlockedBy []string
}

// Input is what a package is made from.
type Input struct {
	// Request is the specification as a plug-in reads it, numbers kept as
	// their text.
	Request     *gentests.Request
	Command     string // the command line that made the package, for the header
	Environment string
	Servers     []Server
	// Held are the tests an open must or should question holds up, by
	// name, each with the questions.
	Held   map[string][]string
	Drafts []Draft
	// LeftOut are the derived cases of rank other no test covers.
	LeftOut []LeftOut
	// Dialect is dxlib when the API is served in the dxlib dialect, every
	// operation one POST in its envelope, which a package does not write.
	Dialect string
}

// Build makes the package of a specification.
func Build(in Input) Package {
	r := in.Request
	info, _ := r.Specification["info"].(map[string]any)
	p := Package{Format: Format, Environment: in.Environment, Servers: in.Servers,
		Specification: Specification{Title: oneLine(str(info, "title")), Version: str(info, "version"), MetaModel: r.Specarch, Digest: r.Digest, Draft: r.Draft}}
	if p.Servers == nil {
		p.Servers = []Server{}
	}
	tests, _ := r.Specification["tests"].(map[string]any)
	kept := map[string]any{}
	for name, t := range tests {
		if qs := in.Held[name]; len(qs) > 0 {
			p.Held = append(p.Held, Held{Name: name, Element: "#/tests/" + escape(name), Questions: qs})
			continue
		}
		kept[name] = t
	}
	sort.Slice(p.Held, func(i, j int) bool { return p.Held[i].Name < p.Held[j].Name })
	p.Cases = append(p.Cases, cases(r.Specification, kept, nil, in.Dialect)...)
	drafted := map[string]any{}
	elements := map[string]string{}
	var order []string
	for _, d := range in.Drafts {
		if _, taken := tests[d.Name]; taken {
			continue
		}
		if len(d.BlockedBy) > 0 {
			p.Held = append(p.Held, Held{Name: d.Name, Element: d.Element, Questions: d.BlockedBy})
			continue
		}
		if _, twice := drafted[d.Name]; twice {
			continue
		}
		drafted[d.Name] = d.Test
		elements[d.Name] = d.Element
		order = append(order, d.Name)
	}
	derived := cases(r.Specification, drafted, elements, in.Dialect)
	byName := map[string]Case{}
	for _, c := range derived {
		byName[c.Name] = c
	}
	for _, name := range order {
		p.Cases = append(p.Cases, byName[name])
	}
	if p.Cases == nil {
		p.Cases = []Case{}
	}
	p.LeftOut = in.LeftOut
	return p
}

// cases reads tests into cases, through the suite every test generator
// reads; elements, when given, marks them derived and names their element.
func cases(specification map[string]any, tests map[string]any, elements map[string]string, dialect string) []Case {
	if len(tests) == 0 {
		return nil
	}
	own := map[string]any{}
	for k, v := range specification {
		own[k] = v
	}
	own["tests"] = tests
	suite := gentests.Build(&gentests.Request{Specification: own})
	var out []Case
	for _, t := range suite.Tests {
		raw, _ := tests[t.Name].(map[string]any)
		c := Case{Name: t.Name, Element: "#/tests/" + escape(t.Name), Subject: t.Subject, Scenario: t.Scenario, Given: t.Given, When: t.When, Then: t.Then}
		if elements != nil {
			c.Element, c.Derived = elements[t.Name], true
		}
		for _, v := range list(raw["covers"]) {
			c.Covers = append(c.Covers, text(v))
		}
		c.Skip = skip(t, c.Derived)
		if c.Skip == "" {
			fill(&c, t, raw, own)
		}
		if c.Skip == "" && dialect == "dxlib" && c.Request != nil {
			c.Skip = "the API is served in the dxlib dialect, every operation one POST with its parameters and body in the dialect's envelope, which a test package does not write; run this test with the project's own suite"
			c.Caller, c.Records, c.Request, c.Expect, c.Unchecked = "", "", nil, nil, nil
		}
		// A request with no caller is refused as not signed in, which is
		// another case than a caller without the permission.
		for _, cv := range c.Covers {
			if c.Skip == "" && c.Caller == "" && strings.HasPrefix(cv, "denied without ") {
				c.Skip = fmt.Sprintf("the case covers %s and names no caller, and a request with no caller is refused as not signed in, which is another case; name a role without %s under fixture.caller, or declare one, and make the package again", cv, strings.TrimPrefix(cv, "denied without "))
				c.Caller, c.Records, c.Request, c.Expect, c.Unchecked = "", "", nil, nil, nil
			}
		}
		out = append(out, c)
	}
	return out
}

// skip is why a test, or a derived case drafted as derive drafts it,
// cannot be run from outside, or "".
func skip(t gentests.Test, derived bool) string {
	switch {
	case derived && t.Call == nil:
		return "no test covers this case, and derive drafts no request for it, since the design does not fix the call (a value derive cannot make, a record it cannot name, or no role to call with); write the test with its fixture, input and expect, and make the package again"
	case t.NotApplicable != "":
		return "the test does not apply: " + t.NotApplicable
	case t.Call == nil && t.NoCall != "" && t.NoCall != "the design gives no call to make":
		return t.NoCall
	case t.Call == nil:
		return "the test gives no input and no expect, so the specification gives no request to make; write them, or run specarch derive --fill, and make the package again"
	case t.Call.Kind == "run":
		return fmt.Sprintf("command %s runs on a host, not over HTTP; a test package makes requests only, so run this test with the command's own suite", t.Call.Command)
	case t.Call.Kind == "open":
		return fmt.Sprintf("page %s is opened in a browser, not requested; a test package makes requests only, so run this test with the screens' own suite", t.Call.Page)
	case t.Call.Handshake != "":
		return fmt.Sprintf("the request travels in the envelope of handshake %s, which only the project's harness can seal; run this test with the project's own suite", t.Call.Handshake)
	case t.Call.Stream != "":
		return fmt.Sprintf("the request streams a file (%s), which a test package does not send; run this test with the project's own suite", t.Call.Stream)
	case len(t.CallerProperties) > 0:
		var names []string
		for _, p := range t.CallerProperties {
			names = append(names, p.Name)
		}
		return fmt.Sprintf("the test signs in a %s whose %s it gives, and a run signs in one account per role; run this test with the project's own suite", t.Caller, strings.Join(names, " and "))
	case !t.Expect.Checked():
		return "the test expects only what is left behind (the state after or the messages emitted), which a request from outside cannot see"
	}
	return ""
}

// fill writes a runnable case's request and what its answer must show.
func fill(c *Case, t gentests.Test, raw, specification map[string]any) {
	c.Caller = t.Caller
	fixture, _ := raw["fixture"].(map[string]any)
	records := map[string]any{}
	for _, k := range sortedKeys(fixture) {
		if k != "caller" && !strings.HasPrefix(k, "x-") {
			records[k] = fixture[k]
		}
	}
	if len(records) > 0 {
		c.Records = compact(records)
	}
	input, _ := raw["input"].(map[string]any)
	req := &Request{Method: t.Call.Method, Path: t.Call.Path}
	op, _ := at(specification, "paths", t.Call.Path, strings.ToLower(t.Call.Method)).(map[string]any)
	params := map[string]string{}
	item, _ := at(specification, "paths", t.Call.Path).(map[string]any)
	for _, l := range []any{item["parameters"], op["parameters"]} {
		for _, prm := range list(l) {
			pm, _ := prm.(map[string]any)
			params[str(pm, "name")] = str(pm, "in")
		}
	}
	query := url.Values{}
	body := map[string]any{}
	var cookies []string
	for _, k := range sortedKeys(input) {
		v := input[k]
		switch params[k] {
		case "path":
			req.Path = strings.ReplaceAll(req.Path, "{"+k+"}", url.PathEscape(text(v)))
		case "query":
			for _, item := range values(v) {
				query.Add(k, text(item))
			}
		case "header":
			if req.Headers == nil {
				req.Headers = map[string]string{}
			}
			req.Headers[k] = text(v)
		case "cookie":
			cookies = append(cookies, k+"="+text(v))
		default:
			body[k] = v
		}
	}
	if len(cookies) > 0 {
		if req.Headers == nil {
			req.Headers = map[string]string{}
		}
		req.Headers["Cookie"] = strings.Join(cookies, "; ")
	}
	if q := query.Encode(); q != "" {
		req.Path += "?" + q
	}
	info, _ := specification["info"].(map[string]any)
	rule := str(info, "wireNames")
	if _, ok := op["requestBody"]; ok && len(body) > 0 {
		req.Body = compact(wired(rule, body))
	}
	c.Request = req
	expect, _ := raw["expect"].(map[string]any)
	e := &Expect{}
	if t.Expect.Status != "" {
		e.Status, _ = strconv.Atoi(t.Expect.Status)
	}
	if b, ok := expect["body"].(map[string]any); ok {
		e.Body = compact(wired(rule, b))
		for _, f := range t.Expect.Body {
			switch f.Value.Kind {
			case "int", "uint", "double", "decimal":
				e.Numeric = append(e.Numeric, wirename.Of(rule, f.Name))
			}
		}
	}
	c.Expect = e
	for _, s := range t.Expect.State {
		c.Unchecked = append(c.Unchecked, "the state after: a record of "+s.Mapping)
	}
	for _, m := range t.Expect.Emits {
		c.Unchecked = append(c.Unchecked, "the message emitted: "+m)
	}
	if t.Expect.EmitsNothing {
		c.Unchecked = append(c.Unchecked, "that no message is emitted")
	}
}

// wired is a body with every property named as it goes on the wire,
// at every depth, by the specification's info.wireNames (ADR-062).
func wired(rule string, v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[wirename.Of(rule, k)] = wired(rule, item)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = wired(rule, item)
		}
		return out
	}
	return v
}

// Marshal writes the package's file: the header, the draft notice when it
// is one, then the package.
func Marshal(p Package, in Input) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# Generated by specarch %s from %s, version %s; meta-model %s; digest %s. Do not edit this file: change the YAML and generate again.\n",
		in.Command, relRoot(in.Request.Root, in.Request.Output), p.Specification.Version, p.Specification.MetaModel, p.Specification.Digest)
	if p.Specification.Draft != "" {
		fmt.Fprintf(&b, "# %s\n", p.Specification.Draft)
	}
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(p); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Read reads a package's file.
func Read(data []byte) (*Package, error) {
	var p Package
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("it is not a test package's YAML: %v", err)
	}
	if p.Format == "" {
		return nil, fmt.Errorf("it has no specarchTestPackage key, which every test package starts with; make it with specarch generate tests --package <environment>")
	}
	if p.Format != Format {
		return nil, fmt.Errorf("it is a test package of format %s, and this build reads format %s; run it with the specarch that made it, or make it again with this one", p.Format, Format)
	}
	return &p, nil
}

func relRoot(root, output string) string {
	if root == "" {
		return "the specification"
	}
	return gentests.RelRoot(root, output)
}

// compact is a value as JSON on one line, numbers as written.
func compact(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func text(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		return strconv.FormatBool(x)
	}
	return compact(v)
}

func values(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return []any{v}
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func escape(token string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(token)
}

func at(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

func str(m any, key string) string {
	mm, _ := m.(map[string]any)
	s, _ := mm[key].(string)
	return s
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
