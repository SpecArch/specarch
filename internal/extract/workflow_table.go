package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The workflow table a project's printer writes of the workflows its engine
// loads, in the format tools/workflows/dump-workflows.sh wraps (ADR-118):
// the version of the format, the path and the commit the definitions were
// loaded from, and every workflow as the sequential subset of ADR-054 holds
// it.
type workflowTable struct {
	WorkflowTable int             `json:"workflowTable"`
	Path          string          `json:"path"`
	Commit        string          `json:"commit"`
	Repository    string          `json:"repository"`
	Workflows     []tableWorkflow `json:"workflows"`
}

type tableWorkflow struct {
	Name        string        `json:"name"`
	File        *string       `json:"file"`
	Description *string       `json:"description"`
	Trigger     *tableTrigger `json:"trigger"`
	Steps       []tableStep   `json:"steps"`
	Emits       []string      `json:"emits"`
}

type tableTrigger struct {
	Operation *string          `json:"operation"`
	Form      []tableFormField `json:"form"`
}

type tableFormField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type tableStep struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Approvers   []string `json:"approvers"`
	Deadline    *string  `json:"deadline"`
	OnDeadline  *string  `json:"onDeadline"`
	EscalateTo  *string  `json:"escalateTo"`
	Operation   *string  `json:"operation"`
	Description *string  `json:"description"`
}

// The keys of each object of the format; every one is written, null where
// the format allows it, and no other is.
var (
	tableWorkflowKeys = []string{"name", "file", "description", "trigger", "steps", "emits"}
	tableTriggerKeys  = []string{"operation", "form"}
	tableFieldKeys    = []string{"name", "type"}
	tableStepKeys     = map[string][]string{
		"approval":  {"name", "kind", "approvers", "deadline", "onDeadline", "escalateTo"},
		"operation": {"name", "kind", "operation"},
		"other":     {"name", "kind", "description"},
	}
	tableKeys = []string{"workflowTable", "path", "commit", "workflows"}
)

var emitsWord = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*/[A-Z][A-Za-z0-9]*$`)

// WorkflowTable reads a workflow table into one workflow per workflow it
// lists, with the questions the BPMN reader asks: the trigger's and each
// operation step's operation left out with a must question carrying the
// name under names, which specarch merge fills from the tree that declares
// it, and the subject and each approval's permission asked for.
func WorkflowTable(dumpPath, out, key string) (*Result, error) {
	t, r, dumpName, err := openWorkflowTable(dumpPath)
	if err != nil {
		return nil, err
	}
	wr := &workflowReader{file: t.Path, key: key, res: &Result{Tree: newTree()},
		workflows: mapping(), roles: map[string][]*yaml.Node{}, questions: mapping()}
	wr.res.say("commit %s: the last change to %s, as the workflow table %s names it", r.Commit, t.Path, dumpName)
	approvals, operations, others := 0, 0, 0
	for _, w := range t.Workflows {
		for _, s := range w.Steps {
			switch s.Kind {
			case "approval":
				approvals++
			case "operation":
				operations++
			default:
				others++
			}
		}
	}
	wr.res.say("counted %s, %s, %s and %s: every entry of the workflow table's list and of each one's steps, which hold every workflow the engine loads",
		plural(len(t.Workflows), "workflow"), plural(approvals, "approval step"), plural(operations, "operation step"), plural(others, "other step"))
	cited := map[string]string{} // clause -> its title
	order := 0
	for _, w := range t.Workflows {
		clause := t.Path
		if w.File != nil {
			f := *w.File
			_, statErr := os.Stat(filepath.Join(r.Repository.Root, filepath.FromSlash(f)))
			switch {
			case !under(f, t.Path):
				order++
				wr.gapAt(order, t.Path, "", "workflow %s: its file %s is not under %s, the folder the table was printed from; the workflow cites the folder", w.Name, f, t.Path)
			case statErr != nil:
				order++
				wr.gapAt(order, t.Path, "", "workflow %s: its file %s is not in the repository at commit %s; the workflow cites the folder", w.Name, f, r.Commit)
			default:
				clause = f
			}
		}
		if clause == t.Path {
			cited[clause] = "The workflows the engine loads from here, as the workflow table prints them"
		} else if cited[clause] == "" {
			cited[clause] = "A workflow definition the engine loads"
		}
		wr.tableWorkflow(w, clause, &order)
	}
	var clauses []string
	for c := range cited {
		clauses = append(clauses, c)
	}
	sort.Strings(clauses)
	var clauseNodes []*yaml.Node
	for _, c := range clauses {
		clauseNodes = append(clauseNodes, flow(mapping("clause", c, "title", cited[c])))
	}
	src, err := codeSource(r, out, clauseNodes)
	if err != nil {
		return nil, err
	}
	set(src, "reading", "printed")
	description := fmt.Sprintf("The workflows the engine loads from %s, read from the workflow table %s, made at commit %s, in the sequential subset the meta-model holds: approvals, operation steps and deadlines. Every workflow cites its definition; what the table does not say, or says outside the subset, is a question.\n", t.Path, dumpName, r.Commit)
	wr.finish(src, "Workflows of "+t.Path, description, "one workflow per workflow of the table, one role per approver, and one question per thing the table does not say or the subset does not hold",
		"The workflow table names it as an approver and says nothing of what it may do.", "An approver in the workflow table is a name; the grants are where the running permission check reads them.", "The workflow table names the role and does not describe it.")
	return wr.res, nil
}

// under says whether a path from the repository's root is the folder or
// inside it; "." is the whole repository.
func under(path, folder string) bool {
	return folder == "." || path == folder || strings.HasPrefix(path, folder+"/")
}

// openWorkflowTable reads a workflow table, checks that every object holds
// the keys of the format and no other, and checks it against the commit it
// names.
func openWorkflowTable(dumpPath string) (*workflowTable, *Read, string, error) {
	data, err := os.ReadFile(dumpPath)
	if err != nil {
		return nil, nil, "", err
	}
	var t workflowTable
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return nil, nil, "", refuse("%s is not a workflow table: %v", dumpPath, err)
	}
	if t.WorkflowTable != 1 {
		return nil, nil, "", refuse("%s is not a workflow table in the format tools/workflows/dump-workflows.sh writes: its workflowTable is %d, not 1", dumpPath, t.WorkflowTable)
	}
	if err := checkTableKeys(dumpPath, data); err != nil {
		return nil, nil, "", err
	}
	r, dumpName, err := openDump(dumpPath, t.Repository, t.Path, t.Commit, "tools/workflows/dump-workflows.sh")
	if err != nil {
		return nil, nil, "", err
	}
	seen := map[string]bool{}
	for i, w := range t.Workflows {
		where := fmt.Sprintf("%s, workflow %d", dumpName, i+1)
		if w.Name == "" {
			return nil, nil, "", refuse("%s has no name", where)
		}
		if seen[w.Name] {
			return nil, nil, "", refuse("%s: workflow %s is listed twice; an engine loads a workflow by its name once", where, w.Name)
		}
		seen[w.Name] = true
		for _, f := range w.Trigger.Form {
			if f.Name == "" || f.Type == "" {
				return nil, nil, "", refuse("%s: a field of the trigger's form has no name or no type", where)
			}
		}
		for j, s := range w.Steps {
			if s.Name == "" {
				return nil, nil, "", refuse("%s, step %d has no name", where, j+1)
			}
		}
	}
	return &t, r, dumpName, nil
}

// checkTableKeys refuses an object of the table that lacks a key of the
// format, or holds one of another kind of step; the decoder has refused
// every key the format does not have.
func checkTableKeys(dumpPath string, data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return refuse("%s is not a workflow table: %v", dumpPath, err)
	}
	if err := hasKeys(dumpPath, "", raw, tableKeys); err != nil {
		return err
	}
	var workflows []map[string]json.RawMessage
	if err := json.Unmarshal(raw["workflows"], &workflows); err != nil || workflows == nil {
		return refuse("%s: workflows is not a list of objects", dumpPath)
	}
	for i, w := range workflows {
		where := fmt.Sprintf("workflow %d", i+1)
		if err := hasKeys(dumpPath, where, w, tableWorkflowKeys); err != nil {
			return err
		}
		var trigger map[string]json.RawMessage
		if err := json.Unmarshal(w["trigger"], &trigger); err != nil || trigger == nil {
			return refuse("%s, %s: trigger is not an object; write its operation as null when the table cannot name it", dumpPath, where)
		}
		if err := hasKeys(dumpPath, where+", trigger", trigger, tableTriggerKeys); err != nil {
			return err
		}
		var form []map[string]json.RawMessage
		if err := json.Unmarshal(trigger["form"], &form); err != nil || form == nil {
			return refuse("%s, %s: the trigger's form is not a list of fields; write [] for a trigger with no form", dumpPath, where)
		}
		for j, f := range form {
			if err := hasKeys(dumpPath, fmt.Sprintf("%s, form field %d", where, j+1), f, tableFieldKeys); err != nil {
				return err
			}
		}
		for _, list := range []string{"steps", "emits"} {
			if string(w[list]) == "null" {
				return refuse("%s, %s: %s is null; write [] for none", dumpPath, where, list)
			}
		}
		var steps []map[string]json.RawMessage
		if err := json.Unmarshal(w["steps"], &steps); err != nil {
			return refuse("%s, %s: steps is not a list of objects", dumpPath, where)
		}
		for j, s := range steps {
			at := fmt.Sprintf("%s, step %d", where, j+1)
			var kind string
			if err := json.Unmarshal(s["kind"], &kind); err != nil {
				return refuse("%s, %s has no kind; a step is an approval, an operation or other", dumpPath, at)
			}
			keys, ok := tableStepKeys[kind]
			if !ok {
				return refuse("%s, %s: its kind %q is not approval, operation or other", dumpPath, at, kind)
			}
			if err := hasKeys(dumpPath, at, s, keys); err != nil {
				return err
			}
			for k := range s {
				if !contains(keys, k) {
					return refuse("%s, %s: a step of kind %s has no key %s", dumpPath, at, kind, k)
				}
			}
			if kind == "approval" && string(s["approvers"]) == "null" {
				return refuse("%s, %s: approvers is null; write [] for an approval that names none", dumpPath, at)
			}
		}
	}
	return nil
}

func hasKeys(dumpPath, where string, obj map[string]json.RawMessage, keys []string) error {
	for _, k := range keys {
		if _, ok := obj[k]; !ok {
			if where == "" {
				return refuse("%s has no key %s; a workflow table holds %s", dumpPath, k, joinAnd(keys))
			}
			return refuse("%s, %s has no key %s; it holds %s, null where the table cannot say", dumpPath, where, k, joinAnd(keys))
		}
	}
	return nil
}

// tableStepRead is one step of a workflow in the table, as read, with the
// name it is written by.
type tableStepRead struct {
	tableStep
	name string
}

// tableWorkflow writes one workflow of the table.
func (wr *workflowReader) tableWorkflow(w tableWorkflow, clause string, order *int) {
	gap := func(asked, format string, args ...any) {
		*order++
		wr.gapAt(*order, clause, asked, format, args...)
	}
	label := w.Name
	name := kebab(label)
	unsaid := !roleWord.MatchString(name)
	if unsaid {
		name = "workflow-" + strings.Trim(notPageWord.ReplaceAllString(strings.ToLower(name), "-"), "-")
	}
	base := name
	for n := 2; source0Has(wr.workflows, name); n++ {
		name = fmt.Sprintf("%s-%d", base, n)
	}
	at := "#/workflows/" + name
	wr.at = at
	defer func() { wr.at = "" }()
	if unsaid {
		gap("", "workflow %s: its name is not one kebab-case can say; the workflow is named %s", label, base)
	}
	if name != base {
		gap("", "workflow %s: workflow %s is taken by another workflow of the table; it is named %s", label, base, name)
	}
	out := mapping()
	if w.Description != nil && strings.TrimSpace(*w.Description) != "" {
		set(out, "description", strings.Join(strings.Fields(*w.Description), " "))
	} else {
		wr.question(fmt.Sprintf("What does workflow %s do, in a sentence a reviewer can check? The workflow table gives no description.", name),
			[]string{at + "/description"}, "", "The table gives the steps and not what the request is for.")
	}
	wr.question(fmt.Sprintf("Which entity holds the request of workflow %s while it waits?", name),
		[]string{at + "/subject"}, "", "The workflow table has no key for the record the request is kept in; the subject is where a page reads where the request is.")

	// The trigger: the operation the table names, and its form.
	op := ""
	if w.Trigger.Operation != nil {
		op = *w.Trigger.Operation
	}
	switch {
	case op != "" && stepWord.MatchString(op):
		wr.question(fmt.Sprintf("Workflow %s is started by operation %s, which the workflow table names as its trigger; the table does not declare it, so it waits for the tree that does. Is it that operation?", name, op),
			[]string{at + "/trigger"}, op, "The table names the operation and not its method or path, so the workflow's trigger waits for the tree that declares it; specarch merge writes it once one does.")
	default:
		if op != "" {
			gap(at+"/trigger", "workflow %s: its trigger names operation %s, which is not a name an operationId can take; asked for instead", label, op)
		}
		wr.question(fmt.Sprintf("Which operation starts workflow %s? The workflow table names no operation an operationId can name.", name),
			[]string{at + "/trigger"}, "", "A workflow starts from an operation that accepts the request and answers 202; the table does not say which.")
	}
	if len(w.Trigger.Form) > 0 {
		var fields []string
		for _, f := range w.Trigger.Form {
			fields = append(fields, fmt.Sprintf("%s (%s)", f.Name, f.Type))
		}
		gap("", "workflow %s: its trigger's form has the %s %s, and a workflow's form is the request body of its trigger's operation, which the tree that declares the operation gives; left out", label, pluralWord(len(fields), "field"), joinAnd(fields))
	}

	// The steps, in order, up to the first the subset does not hold. Their
	// names are settled first, since an escalation names a later step; what
	// is said of each comes in the order of the steps.
	var chain []tableStepRead
	var renamed [][]string
	var other *tableStep
	for _, s := range w.Steps {
		if s.Kind == "other" {
			other = &s
			break
		}
		name, notes := tableStepName(s, chain)
		chain = append(chain, tableStepRead{tableStep: s, name: name})
		renamed = append(renamed, notes)
	}
	var steps []*yaml.Node
	for i, s := range chain {
		ptr := fmt.Sprintf("%s/steps/%d", at, i)
		for _, note := range renamed[i] {
			gap("", "workflow %s: %s", label, note)
		}
		st := mapping("name", s.name)
		if s.Kind == "operation" {
			set(st, "kind", "operation")
			stepOp := ""
			if s.Operation != nil {
				stepOp = *s.Operation
			}
			if stepOp != "" && stepWord.MatchString(stepOp) {
				wr.question(fmt.Sprintf("Step %s of workflow %s calls operation %s, which the workflow table names; the table does not declare it, so it waits for the tree that does. Is it that operation?", s.name, name, stepOp),
					[]string{ptr + "/operation"}, stepOp, "The table names the operation and not its method or path, so the step waits for the tree that declares it; specarch merge writes it once one does.")
			} else {
				if stepOp != "" {
					gap(ptr+"/operation", "workflow %s: step %s names operation %s, which is not a name an operationId can take; asked for instead", label, s.Name, stepOp)
				}
				wr.question(fmt.Sprintf("Which operation does step %s of workflow %s call? The workflow table names no operation an operationId can name.", s.name, name),
					[]string{ptr + "/operation"}, "", "An operation step calls an operation of the specification; the table does not say which.")
			}
			steps = append(steps, st)
			continue
		}
		set(st, "kind", "approval")
		if approvers := wr.tableApprovers(s, clause, label, gap); len(approvers) > 0 {
			set(st, "approvers", approvers)
		} else {
			wr.question(fmt.Sprintf("Which roles may approve step %s of workflow %s? The workflow table names no approver.", s.name, name),
				[]string{ptr + "/approvers"}, "", "An approval names the roles that may approve it.")
		}
		wr.question(fmt.Sprintf("Which permission does step %s of workflow %s check? It cannot be the permission of the operation that starts the workflow.", s.name, name),
			[]string{ptr + "/permission"}, "", "The workflow table names who may approve and not the permission the approval checks.")
		deadline := ""
		if s.Deadline != nil {
			d := strings.TrimSpace(*s.Deadline)
			if bpmnDuration.MatchString(d) && !zeroDuration(d) {
				deadline = d
			} else {
				gap(ptr+"/deadline", "workflow %s: step %s waits %s, and a deadline is an ISO 8601 duration in days, hours, minutes and seconds that is not zero; asked for instead", label, s.Name, d)
			}
		}
		if deadline != "" {
			set(st, "deadline", deadline)
		} else {
			wr.question(fmt.Sprintf("How long does step %s of workflow %s wait for an answer?", s.name, name),
				[]string{ptr + "/deadline"}, "", "An approval has a deadline, as an ISO 8601 duration in days, hours, minutes and seconds; the table gives none it holds.")
		}
		onDeadline, escalateTo := "", ""
		switch {
		case s.OnDeadline == nil:
		case *s.OnDeadline == "refuse":
			onDeadline = "refuse"
		case *s.OnDeadline == "escalate":
			target := ""
			if s.EscalateTo != nil {
				for _, later := range chain[i+1:] {
					if later.Name == *s.EscalateTo && later.Kind == "approval" {
						target = later.name
						break
					}
				}
			}
			if target != "" {
				onDeadline, escalateTo = "escalate", target
			} else {
				to := "no step"
				if s.EscalateTo != nil {
					to = "step " + *s.EscalateTo
				}
				gap(ptr+"/onDeadline", "workflow %s: step %s escalates to %s, and an escalation moves the request to a later approval; asked for instead", label, s.Name, to)
			}
		default:
			gap(ptr+"/onDeadline", "workflow %s: step %s does %q on its deadline, which is neither refuse nor escalate; asked for instead", label, s.Name, *s.OnDeadline)
		}
		if s.EscalateTo != nil && (s.OnDeadline == nil || *s.OnDeadline != "escalate") {
			gap("", "workflow %s: step %s names %s to escalate to and does not escalate on its deadline; left out", label, s.Name, *s.EscalateTo)
		}
		if onDeadline != "" {
			set(st, "onDeadline", onDeadline)
			if escalateTo != "" {
				set(st, "escalateTo", escalateTo)
			}
		} else {
			wr.question(fmt.Sprintf("What happens when step %s of workflow %s passes its deadline: is the request refused, or moved to a later approval?", s.name, name),
				[]string{ptr + "/onDeadline"}, "", "The workflow table does not say that the deadline ends the request or reaches a later approval.")
		}
		steps = append(steps, st)
	}
	if other != nil {
		says := ""
		if other.Description != nil && strings.TrimSpace(*other.Description) != "" {
			says = " (" + strings.Join(strings.Fields(*other.Description), " ") + ")"
		}
		why := fmt.Sprintf("step %s%s is neither an approval nor an operation", other.Name, says)
		gap(at, "workflow %s: %s; the steps after it are left out", label, why)
		wr.question(fmt.Sprintf("Workflow %s of the workflow table leaves the sequential subset at step %s: %s. What are the steps of workflow %s from there?", label, other.Name, why, name),
			[]string{at}, "", "Only the steps before it are written, so the workflow may be missing a part of what the engine runs.")
	}
	if len(steps) > 0 {
		set(out, "steps", steps)
	} else {
		wr.question(fmt.Sprintf("What are the steps of workflow %s? The workflow table gives none the subset holds.", name),
			[]string{at + "/steps"}, "", "A workflow has at least one step.")
	}
	// How it ends: the messages it publishes, which name channels the
	// table does not declare, so they are left out with a line.
	var emits []string
	for _, e := range w.Emits {
		switch {
		case !emitsWord.MatchString(e):
			gap("", "workflow %s: it emits %s, which is not a channel and one of its messages written channel/Message; left out", label, e)
		case !contains(emits, e):
			emits = append(emits, e)
		}
	}
	if len(emits) > 0 {
		gap("", "workflow %s: it ends by publishing %s, and a workflow's emits name messages declared under channels, which the workflow table does not declare; left out", label, joinAnd(emits))
	}
	says := fmt.Sprintf("The workflow table prints workflow %s", label)
	if len(chain) > 0 {
		var names []string
		for _, s := range chain {
			names = append(names, s.Name)
		}
		if len(names) == 1 {
			says += ", with the step " + names[0]
		} else {
			says += ", with the steps " + joinAnd(names) + " in that order"
		}
	}
	set(out, "origin", "stated")
	set(out, "cites", []*yaml.Node{citation(wr.key, clause, says+".")})
	set(wr.workflows, name, out)
}

// tableStepName is a step's name in camelCase, unique among the steps so
// far, and what is said of a name the step does not keep.
func tableStepName(s tableStep, chain []tableStepRead) (string, []string) {
	var notes []string
	name := camel(strings.ReplaceAll(kebab(s.Name), "-", "_"))
	if !stepWord.MatchString(name) {
		name = fmt.Sprintf("step%d", len(chain)+1)
		notes = append(notes, fmt.Sprintf("step %q: its name is not one camelCase can say; the step is named %s", s.Name, name))
	}
	base := name
	for n := 2; ; n++ {
		taken := false
		for _, c := range chain {
			taken = taken || c.name == name
		}
		if !taken {
			break
		}
		name = fmt.Sprintf("%s%d", base, n)
	}
	if name != base {
		notes = append(notes, fmt.Sprintf("step %s: step %s is taken by an earlier step; it is named %s", s.Name, base, name))
	}
	return name, notes
}

// tableApprovers are the roles an approval step names, in kebab-case.
func (wr *workflowReader) tableApprovers(s tableStepRead, clause, label string, gap func(string, string, ...any)) []string {
	var out []string
	for _, n := range s.Approvers {
		role := kebab(n)
		switch {
		case !roleWord.MatchString(role):
			gap("", "workflow %s: approver %q of step %s is not a name a role can take; left out", label, n, s.Name)
			continue
		case role != n:
			gap("", "workflow %s: approver %q of step %s is written as the role %s", label, n, s.Name, role)
		}
		if !contains(out, role) {
			out = append(out, role)
			wr.roles[role] = append(wr.roles[role], citation(wr.key, clause, fmt.Sprintf("The workflow table names role %s as an approver of step %s of workflow %s.", role, s.Name, label)))
		}
	}
	return out
}

func pluralWord(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
