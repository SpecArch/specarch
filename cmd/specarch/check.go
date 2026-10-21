package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SpecArch/specarch/internal/extract"
	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/gensql"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/validate"
	"go.yaml.in/yaml/v3"
)

// runCheck compares a specification with the dumps of its code and fails
// on a difference the allowlist does not name (ADR-124).
func runCheck(args []string, stdout, stderr io.Writer) int {
	var allowlist, since string
	var paths []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			paths = append(paths, args[i+1:]...)
			i = len(args)
		case a == "--allowlist" || a == "--since":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "specarch check: %s needs a value\n\n%s", a, usage)
				return 2
			}
			if a == "--allowlist" {
				allowlist = args[i+1]
			} else {
				since = args[i+1]
			}
			i++
		case len(a) > 1 && a[0] == '-':
			fmt.Fprintf(stderr, "specarch check has no option %s; to name a path whose name starts with -, write -- before it\n\n%s", a, usage)
			return 2
		default:
			paths = append(paths, a)
		}
	}
	if len(paths) < 2 {
		fmt.Fprintf(stderr, "specarch check needs a specification and at least one dump\n\n%s", usage)
		return 2
	}
	if since != "" && allowlist == "" {
		fmt.Fprintf(stderr, "specarch check: --since compares the allowlist with the one at a revision, so it needs --allowlist\n\n%s", usage)
		return 2
	}
	all, status := loadSpecs(paths[:1], "check", stdout, stderr)
	if status != 0 {
		return status
	}
	if len(all) != 1 {
		fmt.Fprintf(stderr, "specarch check: %s holds %d specifications; give the folder of one\n", paths[0], len(all))
		return 2
	}
	l := all[0]
	var impls []*yaml.Node
	for _, i := range l.impls {
		impls = append(impls, i.Node)
	}
	var diffs []extract.Difference
	compared := map[string]bool{}
	for _, dump := range paths[1:] {
		format, err := extract.DumpFormat(dump)
		if err != nil {
			fmt.Fprintf(stderr, "specarch check: %s\n", readError(err))
			return 2
		}
		compared[format] = true
		var found []extract.Difference
		switch format {
		case extract.RouteTableDump:
			found, err = extract.CheckRoutes(dump, l.spec.Root, impls)
		case extract.PermissionTableDump:
			found, err = extract.CheckGrants(dump, l.spec.Root)
		case extract.GatewayTableDump:
			found, err = extract.CheckGateway(dump, l.spec.Root)
		case extract.CatalogueDump:
			tables, why := checkTables(l)
			if why != "" {
				fmt.Fprintf(stderr, "specarch check: the catalogue dump %s is compared with the tables specarch generate sql writes, and %s\n", dump, why)
				return 2
			}
			found, err = extract.CheckCatalogue(dump, tables)
		}
		if err != nil {
			fmt.Fprintf(stderr, "specarch check: %s\n", readError(err))
			return 2
		}
		diffs = append(diffs, found...)
	}
	var lines []allowLine
	if allowlist != "" {
		var err error
		if lines, err = readAllowlist(allowlist); err != nil {
			fmt.Fprintf(stderr, "specarch check: cannot read the allowlist %s: %s\n", allowlist, readError(err))
			return 2
		}
	}
	var before map[string]bool
	if since != "" {
		var why string
		if before, why = allowlistAt(allowlist, since); why != "" {
			fmt.Fprintf(stderr, "specarch check: %s\n", why)
			return 2
		}
	}
	allowed := map[string]bool{}
	for _, ln := range lines {
		allowed[ln.id] = true
	}
	var out []validate.Diagnostic
	failing, reported := map[string]bool{}, map[string]bool{}
	nAllowed, nReports, nFailing := 0, 0, 0
	for _, d := range diffs {
		if d.Report {
			reported[d.ID()] = true
		} else {
			failing[d.ID()] = true
		}
		switch {
		case d.Report:
			nReports++
		case allowed[d.ID()]:
			nAllowed++
			continue
		default:
			nFailing++
		}
		out = append(out, placeDifference(l, d))
	}
	nLines := 0
	seen := map[string]int{}
	for _, ln := range lines {
		d := validate.Diagnostic{File: allowlist, Line: ln.line, Column: 1, Severity: validate.Error, Path: "/", ID: ""}
		rule, _, _ := strings.Cut(ln.id, " ")
		format, known := extract.RuleFormats[rule]
		switch {
		case known && !compared[format] && seen[ln.id] == 0:
			// A line about a dump of a format this run does not compare
			// is another run's, so one allowlist serves a gate per dump.
			seen[ln.id] = ln.line
			continue
		case !known:
			d.Rule, d.Message = "allowlist_stale", fmt.Sprintf("%s is no rule of check's, so the line allows nothing; a line is the id check prints between brackets, a rule and what the difference is about", rule)
		case seen[ln.id] > 0:
			d.Rule, d.Message = "allowlist_stale", fmt.Sprintf("the allowlist names %s at line %d already; take this line out", ln.id, seen[ln.id])
		case !failing[ln.id] && reported[ln.id]:
			d.Rule, d.Message = "allowlist_stale", fmt.Sprintf("%s is a report, which never fails, so the line allows nothing; take it out", ln.id)
		case !failing[ln.id]:
			d.Rule, d.Message = "allowlist_stale", fmt.Sprintf("no difference that fails is %s, so the line allows nothing; take it out, since the list only shrinks", ln.id)
		case before != nil && !before[ln.id]:
			d.Rule, d.Message = "allowlist_added", fmt.Sprintf("the allowlist at %s does not name %s, and the list only shrinks; settle the difference instead of allowing it", since, ln.id)
		default:
			if seen[ln.id] == 0 {
				seen[ln.id] = ln.line
			}
			continue
		}
		if seen[ln.id] == 0 {
			seen[ln.id] = ln.line
		}
		d.ID = string(d.Rule) + " " + ln.id
		out = append(out, d)
		nLines++
	}
	validate.Sort(out)
	fmt.Fprintf(stdout, "specarch check: %s compared: %s, %d allowed by the allowlist, %s, %s that %s\n",
		countWords(len(paths)-1, "dump"), countWords(nFailing, "difference"), nAllowed, countWords(nReports, "report"), countWords(nLines, "allowlist line"), failOrFails(nLines))
	if since != "" && before == nil {
		fmt.Fprintf(stdout, "specarch check: %s holds no allowlist %s, so the gate is turned on and none of its lines is counted as added\n", since, allowlist)
	}
	for _, d := range out {
		printDiagnostic(stdout, stderr, d)
	}
	if nFailing > 0 || nLines > 0 {
		return 1
	}
	return 0
}

func countWords(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func failOrFails(n int) string {
	if n == 1 {
		return "fails"
	}
	return "fail"
}

// readError is the text of an error reading a dump or a file.
func readError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Path + ": " + pe.Err.Error()
	}
	return err.Error()
}

// checkTables are the tables specarch generate sql writes for PostgreSQL
// through the specification's implementation file, or why there are none.
func checkTables(l loaded) ([]gensql.Table, string) {
	var impls []generate.Implementation
	for _, i := range l.impls {
		if source.Child(source.Child(i.Node, "targets"), "sql") != nil {
			impls = append(impls, i)
		}
	}
	if len(impls) == 0 {
		impls = l.impls
	}
	if len(impls) == 0 {
		return nil, "the specification has no implementation file to write them through; add one"
	}
	for _, i := range impls {
		if d := source.Str(source.Child(source.Child(source.Child(i.Node, "targets"), "sql"), "dialect")); d != "" && d != "postgresql" {
			return nil, fmt.Sprintf("the implementation file %s writes them for %s, and a catalogue dump is PostgreSQL's", i.Path, d)
		}
	}
	// Nothing is written, so the request names an output folder no file
	// system holds, and no file of one is read into it.
	req, _ := newPluginRequest(l, "sql", filepath.Join(l.spec.Dir, "\x00no output"), impls)
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err.Error()
	}
	r, err := gensql.Decode(data)
	if err != nil {
		return nil, err.Error()
	}
	tables, _, diags := gensql.Schema(r)
	if len(diags) > 0 {
		var why []string
		for _, d := range diags {
			why = append(why, d.Path+": "+d.Message)
		}
		return nil, "they cannot be written: " + strings.Join(why, "; ")
	}
	return tables, ""
}

// placeDifference is a difference as a problem line: at the
// specification's entry, or at the dump's line.
func placeDifference(l loaded, d extract.Difference) validate.Diagnostic {
	v := validate.Diagnostic{File: d.File, Line: d.Line, Column: d.Column, Severity: validate.Error, Path: "/", Rule: validate.Rule(d.Rule), Message: d.Message, ID: d.ID()}
	if d.Report {
		v.Severity = validate.Warning
	}
	if d.DumpPointer != "" {
		v.Path = d.DumpPointer
	}
	if d.Pointer != "" {
		v.Path = d.Pointer
		v.File, v.Line, v.Column = l.spec.RootFile, 1, 1
		if start, value, _ := validate.Locate(l.spec.Root, validate.Tokens(d.Pointer)); start != nil {
			file := l.spec.Files[value]
			if file == "" {
				file = l.spec.Files[start]
			}
			if file != "" {
				v.File, v.Line, v.Column = file, start.Line, start.Column
			}
		}
	}
	return v
}

// allowLine is one line of the allowlist that names a difference.
type allowLine struct {
	id   string
	line int
}

func readAllowlist(path string) ([]allowLine, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return allowLines(data), nil
}

// allowLines are the ids of an allowlist's text, each with its line; an
// empty line and one that starts with # are skipped.
func allowLines(data []byte) []allowLine {
	var out []allowLine
	for i, ln := range strings.Split(string(data), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		out = append(out, allowLine{id: ln, line: i + 1})
	}
	return out
}

// allowlistAt is the set of ids the allowlist held at a revision, read
// from git; nil when the revision does not hold it, which is the gate
// being turned on.
func allowlistAt(path, revision string) (map[string]bool, string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err.Error()
	}
	dir := filepath.Dir(abs)
	git := func(args ...string) ([]byte, string, error) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		var errOut bytes.Buffer
		cmd.Stderr = &errOut
		out, err := cmd.Output()
		return out, strings.TrimSpace(errOut.String()), err
	}
	top, msg, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Sprintf("--since reads the allowlist %s from git, and its folder is in no repository: git rev-parse said: %s", path, msg)
	}
	if _, msg, err := git("rev-parse", "--verify", "--quiet", revision+"^{commit}"); err != nil {
		return nil, fmt.Sprintf("--since %s names no commit of the repository at %s%s", revision, strings.TrimSpace(string(top)), gitSaid(msg))
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(top)))
	if err != nil {
		return nil, err.Error()
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err.Error()
	}
	rel, err := filepath.Rel(root, real)
	if err != nil {
		return nil, err.Error()
	}
	data, _, err := git("show", revision+":"+filepath.ToSlash(rel))
	if err != nil {
		return nil, ""
	}
	held := map[string]bool{}
	for _, ln := range allowLines(data) {
		held[ln.id] = true
	}
	return held, ""
}

func gitSaid(msg string) string {
	if msg == "" {
		return ""
	}
	return ": git said: " + msg
}
