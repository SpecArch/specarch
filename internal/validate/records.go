package validate

import (
	"fmt"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// recordFolders maps each kind of record to its folder under records/.
var recordFolders = map[string]string{
	"change": "changes", "defect": "defects", "release": "releases",
	"incident": "incidents", "commissioning": "commissioning", "approval": "approvals",
}

// record is one record file, read and checked against its schema.
type record struct {
	path   string
	folder string
	kind   string // a kind of recordFolders, or "" when it has none
	root   *yaml.Node
	c      *checker
}

// key is what a record is named by: its ID, its version, or the date and
// environment of a commissioning run; "" when the fields are missing.
func (r *record) key() string {
	switch r.kind {
	case "change", "defect", "incident":
		return r.str("id")
	case "release", "approval":
		return r.str("version")
	case "commissioning":
		date, env := r.str("date"), r.str("environment")
		if date == "" || env == "" {
			return ""
		}
		return date + "-" + env
	}
	return ""
}

func (r *record) str(key string) string { return source.Str(source.Child(r.root, key)) }

// recordSet indexes the records of a specification by kind and key.
type recordSet map[string]map[string]*record

func (rs recordSet) has(kind, key string) bool { return rs[kind][key] != nil }

// checkRecords checks the records beside a specification: each against the
// record schema, then against the specification and the other records.
func checkRecords(s *spec.Spec, d *design) []Diagnostic {
	var recs []*record
	set := recordSet{}
	for _, f := range s.Records {
		r := &record{path: f.Path, folder: f.Folder, c: &checker{file: f.Path}}
		recs = append(recs, r)
		doc := source.Parse(f.Data)
		for _, p := range doc.Problems {
			r.c.addLine(p.Line, p.Path, Rule(p.Rule), "%s", p.Message)
		}
		if doc.Root == nil {
			continue
		}
		r.root = doc.Root
		r.c.root = doc.Root
		r.c.checkSchema(KindRecord, doc.Value)
		if doc.Root.Kind != yaml.MappingNode {
			continue
		}
		if k := r.str("kind"); recordFolders[k] != "" {
			r.kind = k
			if key := r.key(); key != "" {
				if set[k] == nil {
					set[k] = map[string]*record{}
				}
				if set[k][key] == nil {
					set[k][key] = r
				}
			}
		}
	}
	rc := &recordChecks{d: d, set: set}
	for _, r := range recs {
		if r.kind != "" {
			rc.check(r)
		}
	}
	rootChecker := &checker{file: s.RootFile, files: s.Files}
	rc.checkReleases(recs, rootChecker)
	out := rootChecker.diags
	for _, r := range recs {
		out = append(out, withoutEchoes(r.c.diags)...)
	}
	return out
}

type recordChecks struct {
	d   *design
	set recordSet
}

func (rc *recordChecks) check(r *record) {
	rc.checkName(r)
	switch r.kind {
	case "change":
		rc.role(r, "raisedBy")
		rc.role(r, "decision", "by")
		rc.release(r, "release")
		rc.checkApplied(r)
		rc.checkDecision(r)
	case "defect":
		rc.role(r, "reportedBy")
		rc.environment(r, "environment")
		for i, item := range source.Items(source.Child(r.root, "violates")) {
			rc.reference(r, item, source.Pointer("violates", fmt.Sprint(i)))
		}
		rc.test(r)
		for i, item := range source.Items(source.Child(r.root, "incidents")) {
			rc.recordID(r, item, source.Pointer("incidents", fmt.Sprint(i)), "incident")
		}
		if n := source.Child(r.root, "change"); n != nil {
			rc.recordID(r, n, "/change", "change")
		}
		rc.release(r, "release")
		rc.checkDefectTest(r)
		rc.checkDuplicate(r)
	case "release":
		for i, item := range source.Items(source.Child(r.root, "includes")) {
			rc.recordID(r, item, source.Pointer("includes", fmt.Sprint(i)), "change", "defect")
		}
		for i, item := range source.Items(source.Child(r.root, "commissioning")) {
			if v := item.Value; v != "" && !rc.set.has("commissioning", v) {
				r.c.add(item, source.Pointer("commissioning", fmt.Sprint(i)), RuleRecordRef,
					"%s is not a commissioning run: there is no records/commissioning/%s.yaml", v, v)
			}
		}
	case "incident":
		rc.environment(r, "environment")
		if n := source.Child(r.root, "monitor"); n != nil {
			if v := source.Str(n); v != "" && rc.d.monitors[v] == nil {
				r.c.add(n, "/monitor", RuleRecordRef, "%s is not a monitor of the specification%s", v, suggest(v, rc.d.monitors))
			}
		}
		rc.role(r, "reportedBy")
		for i, item := range source.Items(source.Child(r.root, "defects")) {
			rc.recordID(r, item, source.Pointer("defects", fmt.Sprint(i)), "defect")
		}
		for i, item := range source.Items(source.Child(r.root, "changes")) {
			rc.recordID(r, item, source.Pointer("changes", fmt.Sprint(i)), "change")
		}
		rc.checkIncidentLink(r)
	case "commissioning":
		rc.environment(r, "environment")
		rc.role(r, "operator")
		rc.role(r, "signoff", "by")
		rc.checkCommissioning(r)
	case "approval":
		rc.role(r, "approvedBy")
	}
}

// checkName reports a record outside its kind's folder, or one whose file
// name is not what it is named by.
func (rc *recordChecks) checkName(r *record) {
	kindNode := source.Child(r.root, "kind")
	if want := recordFolders[r.kind]; r.folder != want {
		r.c.add(kindNode, "/kind", RuleRecordName, "a record of kind %s lives in records/%s/; move it there", r.kind, want)
	}
	key := r.key()
	name := strings.TrimSuffix(filepath.Base(r.path), ".yaml")
	if key == "" || name == key {
		return
	}
	switch r.kind {
	case "change", "defect", "incident":
		r.c.add(source.Child(r.root, "id"), "/id", RuleRecordName, "the file is named %s but the record's id is %s; name the file %s.yaml", name, key, key)
	case "release", "approval":
		r.c.add(source.Child(r.root, "version"), "/version", RuleRecordName, "the file is named %s but the record's version is %s; name the file %s.yaml", name, key, key)
	case "commissioning":
		r.c.add(source.Child(r.root, "date"), "/date", RuleRecordName, "the file is named %s but the run's date and environment are %s; name the file %s.yaml", name, key, key)
	}
}

// role checks that the value at a key is a stakeholder of the specification.
func (rc *recordChecks) role(r *record, keys ...string) {
	n := r.root
	for _, k := range keys {
		n = source.Child(n, k)
	}
	if v := source.Str(n); n != nil && v != "" && rc.d.stakeholders[v] == nil {
		r.c.add(n, source.Pointer(keys...), RuleRecordRef, "%s is not a stakeholder of the specification%s", v, suggest(v, rc.d.stakeholders))
	}
}

func (rc *recordChecks) environment(r *record, key string) {
	n := source.Child(r.root, key)
	if v := source.Str(n); n != nil && v != "" && rc.d.environments[v] == nil {
		r.c.add(n, source.Pointer(key), RuleRecordRef, "%s is not an environment of the specification%s", v, suggest(v, rc.d.environments))
	}
}

func (rc *recordChecks) release(r *record, key string) {
	n := source.Child(r.root, key)
	if v := source.Str(n); n != nil && v != "" && !rc.set.has("release", v) {
		r.c.add(n, source.Pointer(key), RuleRecordRef, "%s is not a release: there is no records/releases/%s.yaml", v, v)
	}
}

func (rc *recordChecks) test(r *record) {
	n := source.Child(r.root, "test")
	if v := source.Str(n); n != nil && v != "" && source.Child(source.Child(rc.d.root, "tests"), v) == nil {
		r.c.add(n, "/test", RuleRecordRef, "%s is not a test of the specification%s", v, suggest(v, topMap(rc.d.root, "tests")))
	}
}

// recordID checks that an ID names a record of one of the kinds, or falls
// in a declared change-set or defect-set.
func (rc *recordChecks) recordID(r *record, n *yaml.Node, path string, kinds ...string) {
	v := source.Str(n)
	if v == "" {
		return
	}
	prefix, _, _ := strings.Cut(v, "-")
	var where, sets []string
	for _, k := range kinds {
		if rc.set.has(k, v) {
			return
		}
		if k == "change" || k == "defect" {
			if rc.d.trackerPrefixes(k + "-set")[prefix] != nil {
				return
			}
			sets = append(sets, k+"-set")
		}
		where = append(where, "records/"+recordFolders[k]+"/"+v+".yaml")
	}
	what := map[string]string{"change": "a change request", "defect": "a defect", "incident": "an incident"}
	var names []string
	for _, k := range kinds {
		names = append(names, what[k])
	}
	msg := fmt.Sprintf("%s is not %s: there is no %s", v, strings.Join(names, " or "), strings.Join(where, " or "))
	if len(sets) > 0 {
		msg += fmt.Sprintf(" and no source of kind %s with prefix %s", strings.Join(sets, " or "), prefix)
	}
	r.c.add(n, path, RuleRecordRef, "%s", msg)
}

// trackerPrefixes lists the prefixes of the sources of one kind:
// requirement-set, change-set or defect-set.
func (d *design) trackerPrefixes(kind string) map[string]*yaml.Node {
	out := map[string]*yaml.Node{}
	for _, src := range d.sources {
		if source.Str(source.Child(src, "kind")) == kind {
			if prefix := source.Str(source.Child(src, "prefix")); prefix != "" {
				out[prefix] = src
			}
		}
	}
	return out
}

// resolution is what a reference into the specification finds.
type resolution int

const (
	refMissing  resolution = iota
	refFound               // a requirement, or an element a pointer reaches
	refRetired             // a requirement whose status is retired
	refExternal            // a requirement of a declared requirement set
)

func (d *design) resolve(ref string) resolution {
	if strings.HasPrefix(ref, "#/") {
		var tokens []string
		for _, t := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			tokens = append(tokens, source.UnescapeToken(t))
		}
		if _, ok := source.Resolve(d.root, tokens); ok {
			return refFound
		}
		return refMissing
	}
	if req := d.requirements[ref]; req != nil {
		if source.Str(source.Child(req, "status")) == "retired" {
			return refRetired
		}
		return refFound
	}
	prefix, _, _ := strings.Cut(ref, "-")
	if d.requirementSetPrefixes()[prefix] != nil {
		return refExternal
	}
	return refMissing
}

// reference checks a requirement ID or #/ pointer a record names.
func (rc *recordChecks) reference(r *record, n *yaml.Node, path string) {
	v := source.Str(n)
	if v == "" || rc.d.resolve(v) != refMissing {
		return
	}
	if strings.HasPrefix(v, "#/") {
		r.c.add(n, path, RuleRecordRef, "%s does not resolve in the specification; point at an element that exists", v)
		return
	}
	r.c.add(n, path, RuleRecordRef, "%s is not a requirement of the specification%s", v, suggest(v, rc.d.requirements))
}

var openChange = map[string]bool{"proposed": true, "analysed": true, "approved": true}

// checkApplied checks a change's affects against the specification as it
// is now: carried out once implemented, still to be done while open.
func (rc *recordChecks) checkApplied(r *record) {
	id, status := r.str("id"), r.str("status")
	done := status == "implemented" || status == "released"
	if !done && !openChange[status] {
		return
	}
	affects := source.Child(r.root, "affects")
	for _, list := range []string{"adds", "changes", "removes"} {
		for i, item := range source.Items(source.Child(affects, list)) {
			v := item.Value
			if v == "" {
				continue
			}
			res := rc.d.resolve(v)
			path := source.Pointer("affects", list, fmt.Sprint(i))
			present := res == refFound || res == refExternal
			switch {
			case done && list == "adds" && !present:
				r.c.add(item, path, RuleChangeApplied, "%s is %s but %s, which it adds, is not in the specification; add it, or set the change back to approved", id, status, v)
			case done && list == "changes" && !present:
				r.c.add(item, path, RuleChangeApplied, "%s is %s but %s, which it changes, is not in the specification; correct the reference", id, status, v)
			case done && list == "removes" && res == refFound:
				r.c.add(item, path, RuleChangeApplied, "%s is %s but %s, which it removes, is still in the specification; remove it, or give the requirement status retired", id, status, v)
			case !done && list != "adds" && res == refMissing:
				r.c.add(item, path, RuleChangeApplied, "%s, which %s %s, is not in the specification; a change can only change or remove what exists", v, id, list)
			case !done && list == "adds" && present:
				r.c.warn(item, path, RuleChangeApplied, "%s, which %s adds, is already in the specification; if the change is carried out, set its status to implemented, or name it under changes", v, id)
			}
		}
	}
}

// checkDecision checks that a decided change carries the decision.
func (rc *recordChecks) checkDecision(r *record) {
	id, status := r.str("id"), r.str("status")
	want := ""
	switch status {
	case "approved", "implemented", "released":
		want = "approved"
	case "rejected":
		want = "rejected"
	default:
		return
	}
	decision := source.Child(r.root, "decision")
	if decision == nil {
		r.c.add(source.Child(r.root, "status"), "/status", RuleChangeDecision, "%s is %s but has no decision; add decision with by, date, outcome %s and why", id, status, want)
		return
	}
	if outcome := source.Child(decision, "outcome"); source.Str(outcome) != "" && source.Str(outcome) != want {
		r.c.add(outcome, "/decision/outcome", RuleChangeDecision, "%s is %s but its decision's outcome is %s; a %s change needs outcome %s", id, status, source.Str(outcome), status, want)
	}
}

// checkDefectTest checks that a fixed defect names the test that shows the
// fix, and that the test is about what the defect breaks.
func (rc *recordChecks) checkDefectTest(r *record) {
	id, status := r.str("id"), r.str("status")
	if status != "fixed" && status != "released" {
		return
	}
	tn := source.Child(r.root, "test")
	name := source.Str(tn)
	if name == "" {
		r.c.add(source.Child(r.root, "status"), "/status", RuleDefectTest, "%s is %s but names no test; add test with the test that fails before the fix and passes after", id, status)
		return
	}
	t := source.Child(source.Child(rc.d.root, "tests"), name)
	if t == nil {
		return // reported as record_ref
	}
	verifies := map[string]bool{}
	for _, item := range source.Items(source.Child(t, "verifies")) {
		verifies[item.Value] = true
	}
	subject := rc.d.testElement(t)
	for _, item := range source.Items(source.Child(r.root, "violates")) {
		v := item.Value
		if verifies[v] {
			return
		}
		if subject != "" && strings.HasPrefix(v, "#/") && elementOf(v) == subject {
			return
		}
	}
	r.c.add(tn, "/test", RuleDefectTest, "test %s neither verifies a requirement %s violates nor is about an element it violates; add the requirement to the test's verifies, or name the test that shows the fix", name, id)
}

// testElement is the pointer of the element a test is about, such as
// /entities/Loan, or "".
func (d *design) testElement(t *yaml.Node) string {
	if v := source.Str(source.Child(t, "operation")); v != "" {
		if o, ok := d.operations[v]; ok {
			return source.Pointer("paths", o.path)
		}
		return ""
	}
	for _, k := range []string{"command", "page", "entity"} {
		if v := source.Str(source.Child(t, k)); v != "" {
			return source.Pointer(k+"s", v)
		}
	}
	return ""
}

// elementOf is the element a #/ pointer points into: its first two tokens.
func elementOf(ref string) string {
	tokens := strings.SplitN(strings.TrimPrefix(ref, "#/"), "/", 3)
	if len(tokens) < 2 {
		return ""
	}
	return "/" + tokens[0] + "/" + tokens[1]
}

// checkDuplicate checks that a duplicate names the defect it repeats.
func (rc *recordChecks) checkDuplicate(r *record) {
	id := r.str("id")
	if r.str("status") != "duplicate" {
		return
	}
	n := source.Child(r.root, "duplicateOf")
	of := source.Str(n)
	if of == "" {
		r.c.add(source.Child(r.root, "status"), "/status", RuleDefectDuplicate, "%s is a duplicate but names no defect; add duplicateOf with the defect it repeats", id)
		return
	}
	if other := rc.set["defect"][of]; other != nil {
		if other.str("status") == "duplicate" {
			r.c.add(n, "/duplicateOf", RuleDefectDuplicate, "%s is itself a duplicate; name the defect %s repeats instead", of, of)
		}
		return
	}
	prefix, _, _ := strings.Cut(of, "-")
	if rc.d.trackerPrefixes("defect-set")[prefix] != nil {
		return
	}
	r.c.add(n, "/duplicateOf", RuleDefectDuplicate, "%s is not a defect: there is no records/defects/%s.yaml and no source of kind defect-set with prefix %s", of, of, prefix)
}

// checkIncidentLink warns about a resolved incident that explains nothing.
func (rc *recordChecks) checkIncidentLink(r *record) {
	if r.str("status") != "resolved" {
		return
	}
	for _, k := range []string{"defects", "changes", "noChange"} {
		if source.Child(r.root, k) != nil {
			return
		}
	}
	r.c.warn(source.Child(r.root, "status"), "/status", RuleIncidentLink, "%s is resolved but leads to no defect and no change; name the defects or changes that follow from it, or say in noChange why there are none", r.str("id"))
}

// checkCommissioning checks a run's results name checks of the
// specification and its version is a release.
func (rc *recordChecks) checkCommissioning(r *record) {
	for _, p := range source.Pairs(source.Child(r.root, "results")) {
		if rc.d.checks[p.Key.Value] == nil {
			r.c.add(p.Key, source.Pointer("results", p.Key.Value), RuleCommissioningRecord, "%s is not a check of the specification%s", p.Key.Value, suggest(p.Key.Value, rc.d.checks))
		}
	}
	n := source.Child(r.root, "version")
	if v := source.Str(n); v != "" && !rc.set.has("release", v) {
		r.c.add(n, "/version", RuleCommissioningRecord, "%s is not a release: there is no records/releases/%s.yaml; a commissioning run accepts a release", v, v)
	}
}
