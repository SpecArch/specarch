package testpackage

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Options are what a run is given beside the package: where the server
// is, how each role signs in, and how the records a case needs are put in
// place. None of them is kept in the package.
type Options struct {
	// BaseURL is the server's address; when empty, Server names one of
	// the package's servers, or the package has exactly one.
	BaseURL string
	Server  string
	// Callers are the headers that sign a caller in, by role.
	Callers map[string]map[string]string
	// Setup is a program run before each case that needs records, with
	// the case's name as its argument and the records on its standard
	// input; "" leaves those cases skipped.
	Setup string
	// Client sends the requests; nil is a client with a 30-second limit.
	Client *http.Client
	// Progress, when given, gets one line per case as it ends.
	Progress io.Writer
}

// Result is one case's outcome, as JUnit XML records it.
type Result struct {
	Name    string
	Subject string
	Outcome string // passed, failed, error or skipped
	Message string
	Detail  string
	Seconds float64
}

// Address is the server a run sends its requests to, or why none can be
// chosen.
func Address(p *Package, o Options) (string, error) {
	if o.BaseURL != "" {
		return strings.TrimRight(o.BaseURL, "/"), nil
	}
	var names []string
	for _, s := range p.Servers {
		names = append(names, fmt.Sprintf("%s (%s, %s)", s.Name, s.Deployment, s.URL))
		if o.Server != "" && s.Name == o.Server {
			return strings.TrimRight(s.URL, "/"), nil
		}
	}
	switch {
	case o.Server != "" && len(names) == 0:
		return "", fmt.Errorf("--server %s names no server: the package names none for environment %s; give the address with --base-url", o.Server, p.Environment)
	case o.Server != "":
		return "", fmt.Errorf("--server %s is not a server of the package; its servers are %s", o.Server, strings.Join(names, ", "))
	case len(p.Servers) == 1:
		return strings.TrimRight(p.Servers[0].URL, "/"), nil
	case len(p.Servers) == 0:
		return "", fmt.Errorf("the package names no server for environment %s, since no deployment of an implementation file in that environment has a server with an http or https address; give the address with --base-url", p.Environment)
	}
	return "", fmt.Errorf("the package names %d servers for environment %s: %s; choose one with --server <name>, or give the address with --base-url", len(names), p.Environment, strings.Join(names, ", "))
}

// Run runs every case of the package against the server at base, in the
// package's order.
func Run(p *Package, base string, o Options) []Result {
	client := o.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	var out []Result
	for i, c := range p.Cases {
		start := time.Now()
		r := runCase(c, base, client, o)
		r.Seconds = time.Since(start).Seconds()
		if o.Progress != nil {
			line := fmt.Sprintf("%d of %d: %s: %s in %.3f s", i+1, len(p.Cases), c.Name, r.Outcome, r.Seconds)
			if r.Message != "" {
				line += ": " + r.Message
			}
			fmt.Fprintln(o.Progress, line)
		}
		out = append(out, r)
	}
	return out
}

// skipReason is why a run skips a case before any request: the package's
// own reason, or what the run was not given; "" when it can run.
func skipReason(c Case, o Options) string {
	switch {
	case c.Skip != "":
		return c.Skip
	case c.Request == nil:
		return "the case has no request to make"
	case c.Caller != "" && o.Callers[c.Caller] == nil:
		return fmt.Sprintf("the case signs in as role %s, and the run was given no headers for it; add %s to the file --callers names, with the headers that sign that role in", c.Caller, c.Caller)
	case c.Records != "" && o.Setup == "":
		return fmt.Sprintf("the case needs these records in the environment before its request: %s; give --setup <program>, which puts them in place, to run it", c.Records)
	}
	return ""
}

// Plan is what a run would do with each case, one line each, making no
// request and running no program: for --dry-run.
func Plan(p *Package, base string, o Options) []string {
	var out []string
	for _, c := range p.Cases {
		if why := skipReason(c, o); why != "" {
			out = append(out, fmt.Sprintf("would skip %s: %s", c.Name, why))
			continue
		}
		line := fmt.Sprintf("would request %s %s for %s", c.Request.Method, base+c.Request.Path, c.Name)
		if c.Caller != "" {
			line += ", signed in as " + c.Caller
		}
		if c.Records != "" {
			line += ", after " + o.Setup + " puts its records in place"
		}
		out = append(out, line)
	}
	return out
}

func runCase(c Case, base string, client *http.Client, o Options) Result {
	r := Result{Name: c.Name, Subject: c.Subject}
	if why := skipReason(c, o); why != "" {
		r.Outcome, r.Message = "skipped", why
		return r
	}
	headers := map[string]string{}
	for k, v := range o.Callers[c.Caller] {
		headers[k] = v
	}
	if c.Records != "" {
		if msg, detail := setup(o.Setup, c); msg != "" {
			r.Outcome, r.Message, r.Detail = "error", msg, detail
			return r
		}
	}
	for k, v := range c.Request.Headers {
		headers[k] = v
	}
	var body io.Reader
	if c.Request.Body != "" {
		body = strings.NewReader(c.Request.Body)
		headers["Content-Type"] = "application/json"
	}
	req, err := http.NewRequest(c.Request.Method, base+c.Request.Path, body)
	if err != nil {
		r.Outcome, r.Message = "error", fmt.Sprintf("the request %s %s cannot be made: %v", c.Request.Method, base+c.Request.Path, err)
		return r
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		r.Outcome, r.Message = "error", fmt.Sprintf("%s %s got no answer: %v", c.Request.Method, base+c.Request.Path, err)
		return r
	}
	defer resp.Body.Close()
	got, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		r.Outcome, r.Message = "error", fmt.Sprintf("%s %s answered %d and its body cannot be read: %v", c.Request.Method, base+c.Request.Path, resp.StatusCode, err)
		return r
	}
	r.Detail = fmt.Sprintf("%s %s answered %d: %s", c.Request.Method, base+c.Request.Path, resp.StatusCode, excerpt(got))
	if c.Expect != nil && c.Expect.Status != 0 && resp.StatusCode != c.Expect.Status {
		r.Outcome, r.Message = "failed", fmt.Sprintf("answered %d, and the test expects %d; the body: %s", resp.StatusCode, c.Expect.Status, excerpt(got))
		return r
	}
	if c.Expect != nil && c.Expect.Body != "" {
		if why := compareBody(c.Expect.Body, c.Expect.Numeric, got); why != "" {
			r.Outcome, r.Message = "failed", why
			return r
		}
	}
	r.Outcome = "passed"
	if len(c.Unchecked) > 0 {
		r.Detail += "\nNot checked from outside: " + strings.Join(c.Unchecked, "; ")
	}
	return r
}

// setup runs the setup program for a case: "" when it put the records in
// place, or what went wrong and the program's standard error.
func setup(program string, c Case) (string, string) {
	in, _ := json.Marshal(map[string]any{"case": c.Name, "caller": c.Caller, "records": json.RawMessage(c.Records)})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, c.Name)
	cmd.Stdin = bytes.NewReader(in)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return "", ""
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return fmt.Sprintf("the setup program %s exited with status %d for this case, so its records may not be in place: %s", program, exit.ExitCode(), excerpt(stderr.Bytes())), stderr.String()
	}
	return fmt.Sprintf("the setup program %s could not run: %v", program, err), stderr.String()
}

// compareBody checks that the answer's body holds each expected field with
// its value: "" when it does, or the first field that differs.
func compareBody(expected string, numeric []string, got []byte) string {
	var want map[string]any
	dec := json.NewDecoder(strings.NewReader(expected))
	dec.UseNumber()
	if err := dec.Decode(&want); err != nil {
		return fmt.Sprintf("the package's expected body does not read as JSON: %v; make the package again", err)
	}
	var have any
	dec = json.NewDecoder(bytes.NewReader(got))
	dec.UseNumber()
	if err := dec.Decode(&have); err != nil {
		return fmt.Sprintf("the answer's body is not JSON (%v), and the test expects fields in it: %s", err, excerpt(got))
	}
	obj, ok := have.(map[string]any)
	if !ok {
		return fmt.Sprintf("the answer's body is not a JSON object, and the test expects fields in it: %s", excerpt(got))
	}
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v, ok := obj[k]
		if !ok {
			return fmt.Sprintf("the answer's body has no field %s, and the test expects %s", k, compact(want[k]))
		}
		asNumber := false
		for _, n := range numeric {
			asNumber = asNumber || n == k
		}
		if !same(want[k], v, asNumber) {
			return fmt.Sprintf("the answer's body has %s %s, and the test expects %s", k, compact(v), compact(want[k]))
		}
	}
	return ""
}

// same compares two JSON values: numbers by value, objects by the fields
// expected, lists item by item; asNumber compares text as numbers too.
func same(want, have any, asNumber bool) bool {
	if a, ok := number(want, asNumber); ok {
		b, ok := number(have, asNumber)
		return ok && a.Cmp(b) == 0
	}
	switch w := want.(type) {
	case map[string]any:
		h, ok := have.(map[string]any)
		if !ok {
			return false
		}
		for k, v := range w {
			if hv, ok := h[k]; !ok || !same(v, hv, false) {
				return false
			}
		}
		return true
	case []any:
		h, ok := have.([]any)
		if !ok || len(h) != len(w) {
			return false
		}
		for i := range w {
			if !same(w[i], h[i], false) {
				return false
			}
		}
		return true
	}
	return want == have
}

func number(v any, text bool) (*big.Rat, bool) {
	var s string
	switch x := v.(type) {
	case json.Number:
		s = x.String()
	case string:
		if !text {
			return nil, false
		}
		s = x
	default:
		return nil, false
	}
	r, ok := new(big.Rat).SetString(s)
	return r, ok
}

func excerpt(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if s == "" {
		return "(empty)"
	}
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}

// JUnit writes the results as JUnit XML: one testsuite for the run, one
// testcase per case, named as the case is so specarch log test-run maps
// it back. It carries no time: the same outcomes give the same file, so
// its digest, which the test-run record keeps, names the outcomes, and
// the durations go to standard error as each case ends.
func JUnit(p *Package, base string, results []Result) []byte {
	type problem struct {
		Message string `xml:"message,attr"`
		Text    string `xml:",chardata"`
	}
	type testcase struct {
		Name      string   `xml:"name,attr"`
		Classname string   `xml:"classname,attr"`
		Failure   *problem `xml:"failure"`
		Error     *problem `xml:"error"`
		Skipped   *problem `xml:"skipped"`
		SystemOut string   `xml:"system-out,omitempty"`
	}
	type property struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value,attr"`
	}
	type testsuite struct {
		XMLName    xml.Name   `xml:"testsuite"`
		Name       string     `xml:"name,attr"`
		Tests      int        `xml:"tests,attr"`
		Failures   int        `xml:"failures,attr"`
		Errors     int        `xml:"errors,attr"`
		Skipped    int        `xml:"skipped,attr"`
		Properties []property `xml:"properties>property"`
		Cases      []testcase `xml:"testcase"`
	}
	s := testsuite{Name: fmt.Sprintf("%s %s, environment %s", p.Specification.Title, p.Specification.Version, p.Environment),
		Properties: []property{{"environment", p.Environment}, {"server", base}, {"specification", p.Specification.Version}, {"digest", p.Specification.Digest}}}
	for _, r := range results {
		tc := testcase{Name: r.Name, Classname: r.Subject, SystemOut: r.Detail}
		pr := &problem{Message: r.Message, Text: r.Detail}
		switch r.Outcome {
		case "failed":
			tc.Failure, s.Failures = pr, s.Failures+1
		case "error":
			tc.Error, s.Errors = pr, s.Errors+1
		case "skipped":
			tc.Skipped, s.Skipped = &problem{Message: r.Message}, s.Skipped+1
		}
		s.Cases = append(s.Cases, tc)
	}
	s.Tests = len(results)
	out, _ := xml.MarshalIndent(s, "", "  ")
	return append(append([]byte(xml.Header), out...), '\n')
}
