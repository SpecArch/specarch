package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/approval"
	"github.com/SpecArch/specarch/internal/disk"
	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/gentests"
	"github.com/SpecArch/specarch/internal/metamodel"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/testpackage"
	"github.com/SpecArch/specarch/internal/validate"
)

// packageArg takes --package <environment> out of generate's arguments.
func packageArg(args []string) (rest []string, environment string, given bool, msg string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return append(rest, args[i:]...), environment, given, ""
		case a == "--package":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, "", false, "--package needs the key of an environment of the specification, the one the package's requests go to"
			}
			environment, given = args[i+1], true
			i++
		case strings.HasPrefix(a, "--package="):
			environment, given = strings.TrimPrefix(a, "--package="), true
		default:
			rest = append(rest, a)
		}
	}
	return rest, environment, given, ""
}

// runTestPackage writes the test package of one specification for one of
// its environments (ADR-233): every test, and every chosen derived case no
// test covers drafted as derive drafts it, as requests from outside. A
// test an open must or should question holds up is left out and listed;
// any other open question and the approval gate the package as they gate
// every target, and --unapproved makes it a draft.
func runTestPackage(target, environment, out string, check, unapproved, dry bool, paths []string, stdout, stderr io.Writer) int {
	refuse := func(format string, args ...any) int {
		fmt.Fprintf(stderr, "specarch generate: "+format+"\n", args...)
		return 2
	}
	switch {
	case target != "tests":
		return refuse("--package is a package of tests, and the target is %s; run specarch generate tests --package %s", target, environment)
	case out == "":
		return refuse("--package needs --out, the folder the package is written into; a package belongs to no stack, so no implementation file names its folder")
	case environment == "":
		return refuse("--package needs the key of an environment of the specification")
	}
	specs, status := loadSpecs(paths, "generate", stdout, stderr)
	if status != 0 {
		return status
	}
	if len(specs) != 1 {
		return refuse("--package makes the package of one specification, and %d were given; give the folder of the one whose tests the environment runs", len(specs))
	}
	l := specs[0]
	root := l.spec.Root
	envs := source.Child(root, "environments")
	if source.Child(envs, environment) == nil {
		names := sortedPairKeys(envs)
		have := "it has none; declare it under environments in the deployment stage"
		if len(names) > 0 {
			have = "its environments are " + strings.Join(names, ", ")
		}
		return refuse("%s is not an environment of %s; %s", environment, l.spec.Dir, have)
	}

	// The questions that hold up tests alone hold those tests back; any
	// other gates the package as it gates every target.
	held := map[string][]string{}
	var others []string
	for _, id := range generate.Holding(root, nil, "tests") {
		names, onlyTests := heldTests(root, id)
		if !onlyTests {
			others = append(others, id)
			continue
		}
		for _, n := range names {
			held[n] = append(held[n], id)
		}
	}
	var reasons []string
	if len(others) > 0 {
		verb := plural(len(others), "open question") + " of " + l.spec.Dir + " block"
		if len(others) == 1 {
			verb += "s"
		}
		if !unapproved {
			fmt.Fprintf(stderr, "specarch generate: %s what the package reads (%s); answer them before making it, as specarch gaps lists them, or pass --unapproved for a draft; a question that blocks only tests holds back those tests and not the package\n", verb, strings.Join(others, ", "))
			return 1
		}
		noun := "open question"
		if len(others) > 1 {
			noun = "open questions"
		}
		reasons = append(reasons, fmt.Sprintf("%s %s %s what the package reads", noun, strings.Join(others, ", "), map[bool]string{true: "blocks", false: "block"}[len(others) == 1]))
	}
	version := source.Str(source.Child(source.Child(root, "info"), "version"))
	if approved, text := approval.State(l.spec.Dir, version); !approved {
		if !unapproved {
			fmt.Fprintf(stderr, "specarch generate: %s is %s; read the documents and run specarch approve --by <stakeholder> %s, or pass --unapproved for a draft\n", l.spec.Dir, text, l.spec.Dir)
			return 1
		}
		reasons = append(reasons, "the specification is "+text)
	}
	draft := ""
	if len(reasons) > 0 {
		draft = strings.Join(strings.Fields("Draft: "+strings.Join(reasons, ", and ")+"; this file is not the approved output."), " ")
	}

	// The specification as a plug-in reads it, numbers as their text.
	plain, err := json.Marshal(metamodel.Normalized(l.spec.Value))
	if err != nil {
		return refuse("cannot read %s as plain values: %v", l.spec.Dir, err)
	}
	req, err := gentests.Decode([]byte(`{"specification":` + string(plain) + `}`))
	if err != nil {
		return refuse("cannot read %s as plain values: %v", l.spec.Dir, err)
	}
	req.Specarch = source.Str(source.Child(root, "specarch"))
	req.Digest = specDigest(l.spec.Dir)
	req.Root = filepath.ToSlash(l.spec.RootFile)
	req.Output = filepath.ToSlash(out)
	req.Draft = draft

	drafts, msg := packageDrafts(root, l.given)
	if msg != "" {
		return refuse("%s", msg)
	}
	var leftOut []testpackage.LeftOut
	for _, lo := range validate.LeftOutCases(root, l.given) {
		leftOut = append(leftOut, testpackage.LeftOut{Subject: lo.Subject, Case: lo.Case, Why: lo.Reason})
	}
	dialect, _ := apiDialectOf(l.spec)
	in := testpackage.Input{Request: req, Command: "generate tests --package " + environment, Environment: environment,
		Servers: packageServers(l.impls, environment), Held: held, Drafts: drafts, LeftOut: leftOut, Dialect: dialect}
	p := testpackage.Build(in)
	data, err := testpackage.Marshal(p, in)
	if err != nil {
		return refuse("cannot write the package: %v", err)
	}
	plan := []planned{{path: filepath.Join(out, testpackage.FileName), content: string(data)}}
	if check {
		status = checkPlan("generate", plan, stdout, stderr)
	} else {
		status = writePlan("generate", plan, disk.For(dry, stdout), stderr)
	}
	if status != 0 {
		return status
	}
	runnable, skipped, derived := 0, 0, 0
	for _, c := range p.Cases {
		if c.Skip != "" {
			skipped++
		} else {
			runnable++
		}
		if c.Derived {
			derived++
		}
	}
	for _, h := range p.Held {
		fmt.Fprintf(stderr, "specarch generate: held back %s, which %s %s; it joins the package once answered\n", h.Name, strings.Join(h.Questions, ", "), map[bool]string{true: "holds up", false: "hold up"}[len(h.Questions) == 1])
	}
	fmt.Fprintf(stderr, "specarch generate: test package for environment %s: %s, %d of them derived cases no test covers yet; %d to run, %d skipped with the reason in the package; %s held back; %s of rank other left out, listed under leftOut as the test plan lists them, until a test covers one\n",
		environment, plural(len(p.Cases), "case"), derived, runnable, skipped, plural(len(p.Held), "test"), plural(len(p.LeftOut), "derived case"))
	return 0
}

// heldTests names the tests a question blocks, and whether it blocks
// nothing but tests, each by a pointer into it.
func heldTests(root *yaml.Node, id string) ([]string, bool) {
	q := source.Child(source.Child(root, "questions"), id)
	var names []string
	for _, item := range source.Items(source.Child(q, "blocks")) {
		b, ok := spec.ParseBlock(item.Value)
		if !ok || len(b.Tokens) < 2 || b.Tokens[0] != "tests" {
			return nil, false
		}
		names = append(names, b.Tokens[1])
	}
	return names, len(names) > 0
}

// packageDrafts are the cases derive would draft, each with the element
// of its subject and its test read as plain values.
func packageDrafts(root *yaml.Node, given validate.Given) ([]testpackage.Draft, string) {
	var out []testpackage.Draft
	for _, dr := range validate.Drafts(root, given) {
		if dr.Collision != "" {
			continue
		}
		var doc yaml.Node
		if err := yaml.Unmarshal([]byte(dr.Text), &doc); err != nil || len(doc.Content) == 0 {
			return nil, fmt.Sprintf("the draft %s for %s does not read back: %v", dr.Name, dr.Subject, err)
		}
		node := doc.Content[0]
		plain, err := json.Marshal(source.ValueOf(node))
		if err != nil {
			return nil, fmt.Sprintf("the draft %s for %s does not read back: %v", dr.Name, dr.Subject, err)
		}
		r, err := gentests.Decode([]byte(`{"specification":` + string(plain) + `}`))
		if err != nil {
			return nil, fmt.Sprintf("the draft %s for %s does not read back: %v", dr.Name, dr.Subject, err)
		}
		element := generate.TestElement(root, node)
		if element == "" {
			element = "#" + source.Pointer("tests", dr.Name)
		}
		out = append(out, testpackage.Draft{Name: dr.Name, Element: element, Test: r.Specification, BlockedBy: dr.BlockedBy})
	}
	return out, ""
}

// packageServers are the servers that answer requests in the deployments
// of an environment: a server of kind service, or of no kind, that is no
// dependency and has an http or https address.
func packageServers(impls []generate.Implementation, environment string) []testpackage.Server {
	var out []testpackage.Server
	for _, i := range impls {
		for _, d := range source.Pairs(source.Child(i.Node, "deployments")) {
			if source.Str(source.Child(d.Value, "environment")) != environment {
				continue
			}
			for _, s := range source.Items(source.Child(d.Value, "servers")) {
				kind := source.Str(source.Child(s, "kind"))
				addr := source.Str(source.Child(s, "url"))
				u, err := url.Parse(addr)
				if kind != "" && kind != "service" || source.Child(s, "dependency") != nil || err != nil || u.Scheme != "http" && u.Scheme != "https" {
					continue
				}
				out = append(out, testpackage.Server{Deployment: d.Key.Value, Name: source.Str(source.Child(s, "name")), URL: addr})
			}
		}
	}
	return out
}

// runRun is the run command: it runs a test package against its
// environment and writes the results as JUnit XML. 0 when every case that
// ran passed, 1 when one failed or could not run to its end, 2 on a usage
// error or a package, callers file or server that cannot be read or
// chosen.
func runRun(args []string, stdout, stderr io.Writer) int {
	values := map[string]string{}
	options := []string{"results", "base-url", "server", "callers", "setup"}
	var rest []string
	dry := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if a == "--dry-run" {
			dry = true
			continue
		}
		if !strings.HasPrefix(a, "-") || len(a) == 1 {
			rest = append(rest, a)
			continue
		}
		name, value, inline := strings.Cut(strings.TrimPrefix(a, "--"), "=")
		if !contains(options, name) {
			fmt.Fprintf(stderr, "specarch run has no option %s; its options are --%s, each with a value, and --dry-run\n\n%s", a, strings.Join(options, ", --"), usage)
			return 2
		}
		if !inline {
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "specarch run: %s needs a value\n\n%s", a, usage)
				return 2
			}
			value = args[i+1]
			i++
		}
		values[name] = value
	}
	if len(rest) != 1 {
		fmt.Fprintf(stderr, "specarch run needs one test package: its folder, or its file %s\n\n%s", testpackage.FileName, usage)
		return 2
	}
	if values["results"] == "" {
		fmt.Fprintf(stderr, "specarch run needs --results, the file the JUnit XML results are written to\n\n%s", usage)
		return 2
	}
	path := rest[0]
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		path = filepath.Join(path, testpackage.FileName)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "specarch run: cannot read the test package %s: %s; make it with specarch generate tests --package <environment> --out <folder>\n", path, spec.PlainIOError(err))
		return 2
	}
	p, err := testpackage.Read(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch run: %s: %v\n", path, err)
		return 2
	}
	o := testpackage.Options{BaseURL: values["base-url"], Server: values["server"], Setup: values["setup"], Progress: stderr}
	if f := values["callers"]; f != "" {
		raw, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(stderr, "specarch run: cannot read the callers file %s: %s\n", f, spec.PlainIOError(err))
			return 2
		}
		if err := yaml.Unmarshal(raw, &o.Callers); err != nil {
			fmt.Fprintf(stderr, "specarch run: the callers file %s is not a mapping of role to the headers that sign it in, such as librarian: { Authorization: Bearer <token> }: %v\n", f, err)
			return 2
		}
	}
	if s := values["setup"]; s != "" {
		if _, err := os.Stat(s); err != nil {
			fmt.Fprintf(stderr, "specarch run: cannot find the setup program %s: %s; give the path of a program that puts a case's records in place\n", s, spec.PlainIOError(err))
			return 2
		}
	}
	base, err := testpackage.Address(p, o)
	if err != nil {
		fmt.Fprintf(stderr, "specarch run: %v\n", err)
		return 2
	}
	if p.Specification.Draft != "" {
		fmt.Fprintf(stderr, "specarch run: the package is a draft: %s\n", p.Specification.Draft)
	}
	if dry {
		for _, line := range testpackage.Plan(p, base, o) {
			fmt.Fprintln(stdout, line)
		}
		if err := disk.DryRun(stdout).WriteFile(values["results"], nil, 0o644); err != nil {
			fmt.Fprintf(stderr, "specarch run: %v\n", err)
			return 2
		}
		fmt.Fprintf(stderr, "specarch run: %s of %s %s against %s, environment %s; no request made and nothing written (--dry-run)\n", plural(len(p.Cases), "case"), p.Specification.Title, p.Specification.Version, base, p.Environment)
		return 0
	}
	fmt.Fprintf(stderr, "specarch run: %s of %s %s against %s, environment %s\n", plural(len(p.Cases), "case"), p.Specification.Title, p.Specification.Version, base, p.Environment)
	results := testpackage.Run(p, base, o)
	xmlData := testpackage.JUnit(p, base, results)
	if dir := filepath.Dir(values["results"]); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(stderr, "specarch run: cannot make the folder of %s: %s\n", values["results"], spec.PlainIOError(err))
			return 2
		}
	}
	if err := os.WriteFile(values["results"], xmlData, 0o644); err != nil {
		fmt.Fprintf(stderr, "specarch run: cannot write the results %s: %s\n", values["results"], spec.PlainIOError(err))
		return 2
	}
	counts := map[string]int{}
	for _, r := range results {
		counts[r.Outcome]++
		if r.Outcome == "failed" || r.Outcome == "error" {
			fmt.Fprintf(stdout, "%s: %s: %s\n", r.Outcome, r.Name, r.Message)
		}
	}
	var held []string
	for _, h := range p.Held {
		held = append(held, h.Name)
	}
	sort.Strings(held)
	if len(held) > 0 {
		fmt.Fprintf(stderr, "specarch run: not in the package, held by open questions: %s\n", strings.Join(held, ", "))
	}
	fmt.Fprintf(stderr, "specarch run: results written to %s: %d passed, %d failed, %d errored, %d skipped; record them with specarch log test-run --results <the file under records/> --environment %s\n",
		values["results"], counts["passed"], counts["failed"], counts["error"], counts["skipped"], p.Environment)
	if counts["failed"] > 0 || counts["error"] > 0 {
		return 1
	}
	return 0
}
