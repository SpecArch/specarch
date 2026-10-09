package extract

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The namespace of BPMN 2.0's semantic model, the one a BPMN 2.0 XML file
// declares on its definitions element.
const bpmnModel = "http://www.omg.org/spec/BPMN/20100524/MODEL"

// xmlElement is one element of an XML file, with the line it starts on.
type xmlElement struct {
	space, name string
	attrs       map[string]string
	children    []*xmlElement
	text        string
	line        int
}

func (e *xmlElement) attr(name string) string { return e.attrs[name] }

// all lists the children in the BPMN model's namespace with a local name.
func (e *xmlElement) all(name string) []*xmlElement {
	var out []*xmlElement
	for _, c := range e.children {
		if c.space == bpmnModel && c.name == name {
			out = append(out, c)
		}
	}
	return out
}

func (e *xmlElement) first(name string) *xmlElement {
	if all := e.all(name); len(all) > 0 {
		return all[0]
	}
	return nil
}

// parseXML reads an XML document into its elements, each with its line.
func parseXML(data []byte) (*xmlElement, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	lineAt := func(offset int64) int { return 1 + bytes.Count(data[:offset], []byte("\n")) }
	var root *xmlElement
	var stack []*xmlElement
	for {
		at := dec.InputOffset()
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			e := &xmlElement{space: t.Name.Space, name: t.Name.Local, attrs: map[string]string{}, line: lineAt(at)}
			for _, a := range t.Attr {
				if a.Name.Space == "" {
					e.attrs[a.Name.Local] = a.Value
				}
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, e)
			} else if root == nil {
				root = e
			}
			stack = append(stack, e)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(t)
			}
		}
	}
	if root == nil {
		return nil, fmt.Errorf("no element")
	}
	return root, nil
}

// The definitions a process refers to and the reader reads through the
// reference, so they write nothing of their own.
var bpmnReferenced = map[string]bool{"interface": true, "resource": true, "message": true, "itemDefinition": true}

// The elements of a process the sequential subset holds (ADR-054).
var bpmnHeld = map[string]bool{
	"startEvent": true, "endEvent": true, "userTask": true, "serviceTask": true,
	"exclusiveGateway": true, "sequenceFlow": true, "boundaryEvent": true, "documentation": true,
}

var stepWord = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)

// bpmnStep is one step of a process's chain, as read.
type bpmnStep struct {
	el   *xmlElement
	name string
}

// workflowReader reads one BPMN file.
type workflowReader struct {
	file       string // the file, from the repository's root
	key        string
	res        *Result
	operations map[string]string // operation id -> name
	resources  map[string]string // resource id -> name
	workflows  *yaml.Node
	roles      map[string][]*yaml.Node // role -> its citations
	questions  *yaml.Node
	nextID     int
	notHeld    []heldLine
}

// heldLine is one line about what the subset does not hold, at its line
// of the file, so the lines come in the file's order.
type heldLine struct {
	line int
	text string
}

// Workflows reads a BPMN 2.0 XML file into one workflow per process, in
// the sequential subset of ADR-054: a start event whose message names the
// trigger's operation, user tasks as approvals with their potential owners
// and a timer, service tasks as operation steps, and an exclusive gateway
// after an approval that ends the request when it is refused.
func Workflows(path, out, key string) (*Result, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, refuse("%s is a folder; extract workflows reads one BPMN 2.0 XML file", path)
	}
	r, err := Open([]string{path})
	if err != nil {
		return nil, err
	}
	file := r.Paths[0]
	data, err := os.ReadFile(filepath.Join(r.Repository.Root, filepath.FromSlash(file)))
	if err != nil {
		return nil, err
	}
	root, err := parseXML(data)
	if err != nil {
		return nil, refuse("%s is not XML: %v", file, err)
	}
	if root.space != bpmnModel || root.name != "definitions" {
		return nil, refuse("%s is not a BPMN 2.0 XML file: its root element is not definitions in the namespace %s", file, bpmnModel)
	}
	wr := &workflowReader{file: file, key: key, res: &Result{Tree: newTree()},
		operations: map[string]string{}, resources: map[string]string{}, workflows: mapping(),
		roles: map[string][]*yaml.Node{}, questions: mapping()}
	commitLine(wr.res, r)
	if line, text, ok := generatedMark(filepath.Join(r.Repository.Root, filepath.FromSlash(file))); ok {
		wr.res.say("generated: %s:%d says it is generated from another source: %s", file, line, text)
	}
	processes := wr.definitions(root)
	if len(processes) == 0 {
		return nil, refuse("%s holds no process; extract workflows writes one workflow per process", file)
	}
	counts := map[string]int{}
	for _, p := range processes {
		for _, c := range p.children {
			if c.space == bpmnModel {
				counts[c.name]++
			}
		}
	}
	wr.res.say("counted %s, %s, %s and %s: the BPMN elements process, userTask, serviceTask and boundaryEvent in %s, each once",
		plural(len(processes), "process"), plural(counts["userTask"], "user task"), plural(counts["serviceTask"], "service task"), plural(counts["boundaryEvent"], "boundary event"), file)
	for _, p := range processes {
		wr.process(p)
	}
	design := mapping()
	if len(wr.roles) > 0 {
		names := make([]string, 0, len(wr.roles))
		for name := range wr.roles {
			names = append(names, name)
		}
		sort.Strings(names)
		roles := mapping()
		for _, name := range names {
			set(roles, name, mapping("origin", "stated", "cites", wr.roles[name]))
			wr.question(fmt.Sprintf("Which permissions does role %s grant? The BPMN file names it as a potential owner and says nothing of what it may do.", name),
				[]string{"#/roles/" + name + "/permissions"}, "", "A potential owner in BPMN is a name; the grants are where the running permission check reads them.")
			wr.question(fmt.Sprintf("What is role %s, in a sentence a reviewer can check?", name),
				[]string{"#/roles/" + name + "/description"}, "", "The BPMN file names the role and does not describe it.")
		}
		set(design, "roles", roles)
	}
	sortPairs(wr.workflows)
	set(design, "workflows", wr.workflows)
	res := wr.res
	res.Tree.put("design/workflows.yaml", design)
	res.Tree.put("requirements/stakeholders.yaml", mapping("stakeholders", ownerStakeholder()))
	res.Tree.put("design/questions.yaml", mapping("questions", wr.questions))
	src, err := codeSource(r, out, []*yaml.Node{flow(mapping("clause", file, "title", "The BPMN 2.0 process definitions"))})
	if err != nil {
		return nil, err
	}
	res.say("wrote %s, %s and %s: one workflow per process, one role per potential owner, and one question per thing the file does not say or the subset does not hold",
		plural(len(wr.workflows.Content)/2, "workflow"), plural(len(wr.roles), "role"), plural(wr.nextID, "question"))
	sort.SliceStable(wr.notHeld, func(i, j int) bool { return wr.notHeld[i].line < wr.notHeld[j].line })
	for _, h := range wr.notHeld {
		res.Lines = append(res.Lines, h.text)
	}
	description := fmt.Sprintf("The workflows the BPMN 2.0 file %s defines at commit %s, in the sequential subset the meta-model holds: approvals, operation steps and deadlines. Every workflow cites the file; what it does not say, or says outside the subset, is a question.\n", file, r.Commit)
	res.Tree.put("specarch.yaml", rootFile("Workflows of "+file, description, []string{"requirements", "design"}, mapping(key, src)))
	return res, nil
}

func (wr *workflowReader) gap(e *xmlElement, format string, args ...any) {
	wr.notHeld = append(wr.notHeld, heldLine{e.line, fmt.Sprintf("not held: %s:%d: %s", wr.file, e.line, fmt.Sprintf(format, args...))})
}

// question asks a must question; names is the name the file gives for the
// one key the question blocks, or "".
func (wr *workflowReader) question(text string, blocks []string, names, why string) {
	wr.nextID++
	q := mapping(
		"question", text,
		"kind", "decision",
		"priority", "must",
		"blocks", blocks,
		"decidedBy", owner,
	)
	if names != "" {
		set(q, "names", names)
	}
	set(q, "why", why)
	set(wr.questions, fmt.Sprintf("Q-%d", wr.nextID), q)
}

func (wr *workflowReader) cite(e *xmlElement, says string) *yaml.Node {
	return citation(wr.key, wr.file, fmt.Sprintf("Line %d: %s", e.line, says))
}

// definitions reads the operations and resources the processes refer to,
// reports every other definition, and returns the processes.
func (wr *workflowReader) definitions(root *xmlElement) []*xmlElement {
	var processes []*xmlElement
	for _, c := range root.children {
		switch {
		case c.space != bpmnModel:
			wr.gap(c, "%s is not in BPMN's semantic model (its namespace is %s), such as diagram layout or a tool's extension; left out", c.name, c.space)
		case c.name == "process":
			processes = append(processes, c)
		case c.name == "interface":
			for _, op := range c.all("operation") {
				wr.operations[op.attr("id")] = op.attr("name")
			}
		case c.name == "resource":
			wr.resources[c.attr("id")] = c.attr("name")
		case bpmnReferenced[c.name]:
		default:
			wr.gap(c, "%s %s is outside the sequential subset of BPMN 2.0 the meta-model holds; left out", c.name, c.attr("id"))
		}
	}
	return processes
}

// process writes one process as a workflow.
func (wr *workflowReader) process(p *xmlElement) {
	label := p.attr("name")
	if label == "" {
		label = p.attr("id")
	}
	name := kebab(label)
	if !roleWord.MatchString(name) {
		name = "workflow-" + strings.Trim(notPageWord.ReplaceAllString(strings.ToLower(name), "-"), "-")
		wr.gap(p, "process %s: its name is not one kebab-case can say; the workflow is named %s", label, name)
	}
	base := name
	for n := 2; source0Has(wr.workflows, name); n++ {
		name = fmt.Sprintf("%s-%d", base, n)
	}
	if name != base {
		wr.gap(p, "process %s: workflow %s is taken by another process; it is named %s", label, base, name)
	}
	at := "#/workflows/" + name
	byID := map[string]*xmlElement{}
	outgoing := map[string][]*xmlElement{}
	var starts []*xmlElement
	var boundaries []*xmlElement
	for _, c := range p.children {
		if c.space != bpmnModel {
			wr.gap(c, "%s in process %s is not in BPMN's semantic model; left out", c.name, label)
			continue
		}
		if id := c.attr("id"); id != "" {
			byID[id] = c
		}
		switch {
		case c.name == "sequenceFlow":
			outgoing[c.attr("sourceRef")] = append(outgoing[c.attr("sourceRef")], c)
		case c.name == "startEvent":
			starts = append(starts, c)
		case c.name == "boundaryEvent":
			boundaries = append(boundaries, c)
		case c.name == "laneSet":
			wr.gap(c, "the lanes of process %s are not read: a potential owner on each user task names who approves; left out", label)
		}
	}
	w := mapping()
	if doc := p.first("documentation"); doc != nil && strings.TrimSpace(doc.text) != "" {
		set(w, "description", strings.Join(strings.Fields(doc.text), " "))
	} else {
		wr.question(fmt.Sprintf("What does workflow %s do, in a sentence a reviewer can check? Process %s has no documentation.", name, label),
			[]string{at + "/description"}, "", "The file gives the steps and not what the request is for.")
	}
	wr.question(fmt.Sprintf("Which entity holds the request of workflow %s while it waits?", name),
		[]string{at + "/subject"}, "", "BPMN has no element for the record the request is kept in; the subject is where a page reads where the request is.")

	// The trigger: the one start event, and the operation its message names.
	var start *xmlElement
	switch {
	case len(starts) == 1:
		start = starts[0]
		med := start.first("messageEventDefinition")
		opName := ""
		if med != nil {
			opName = wr.operations[med.attr("operationRef")]
		}
		switch {
		case opName != "" && stepWord.MatchString(opName):
			wr.question(fmt.Sprintf("Workflow %s is started by operation %s, which the BPMN file names on its start event; the file does not declare it, so it waits for the tree that does. Is it that operation?", name, opName),
				[]string{at + "/trigger"}, opName, "The file names the operation and not its method or path, so the workflow's trigger waits for the tree that declares it; specarch merge writes it once one does.")
		case opName != "":
			wr.gap(start, "start event %s names operation %s, which is not a name an operationId can take; asked for instead", start.attr("id"), opName)
			fallthrough
		default:
			wr.question(fmt.Sprintf("Which operation starts workflow %s? Its start event names no operation the file declares.", name),
				[]string{at + "/trigger"}, "", "A workflow starts from an operation that accepts the request and answers 202; the start event does not say which.")
		}
	default:
		where := p
		if len(starts) > 1 {
			where = starts[1]
		}
		wr.gap(where, "process %s has %d start events, and a workflow starts from one operation; its trigger and steps are asked for", label, len(starts))
		wr.question(fmt.Sprintf("Which operation starts workflow %s, and what are its steps? Process %s has %d start events.", name, label, len(starts)),
			[]string{at + "/trigger"}, "", "A workflow in the sequential subset has one start.")
	}

	// The chain of steps from the start event.
	var chain []bpmnStep
	seen := map[string]bool{}
	cur := start
	for cur != nil {
		flows := outgoing[cur.attr("id")]
		if len(flows) != 1 {
			if cur.name != "endEvent" {
				wr.chainGap(cur, at, label, name, fmt.Sprintf("%s %s has %d outgoing sequence flows, and a step of the sequential subset has one", cur.name, cur.attr("id"), len(flows)))
			}
			break
		}
		next := byID[flows[0].attr("targetRef")]
		if next == nil {
			wr.gap(flows[0], "sequence flow %s goes to %s, which process %s does not hold; the steps after it are left out", flows[0].attr("id"), flows[0].attr("targetRef"), label)
			break
		}
		if seen[next.attr("id")] {
			wr.chainGap(next, at, label, name, fmt.Sprintf("the sequence flows come back to %s %s, a loop", next.name, next.attr("id")))
			break
		}
		seen[next.attr("id")] = true
		switch next.name {
		case "userTask", "serviceTask":
			chain = append(chain, bpmnStep{el: next, name: wr.stepName(next, chain)})
			cur = next
		case "exclusiveGateway":
			onward := wr.refusal(next, outgoing, byID, chain)
			if onward == nil {
				wr.chainGap(next, at, label, name, fmt.Sprintf("exclusive gateway %s is not the refusal of an approval: one of its two flows ends the request right after a user task", next.attr("id")))
				cur = nil
				break
			}
			cur = onward
			seen[onward.attr("id")] = true
			if onward.name == "endEvent" {
				cur = nil
				break
			}
			if onward.name == "userTask" || onward.name == "serviceTask" {
				chain = append(chain, bpmnStep{el: onward, name: wr.stepName(onward, chain)})
				continue
			}
			wr.chainGap(onward, at, label, name, fmt.Sprintf("%s %s follows the gateway, and the subset holds a task or an end event there", onward.name, onward.attr("id")))
			cur = nil
		case "endEvent":
			cur = nil
		default:
			wr.chainGap(next, at, label, name, fmt.Sprintf("%s %s is not a step of the sequential subset", next.name, next.attr("id")))
			cur = nil
		}
	}
	for _, c := range p.children {
		switch {
		case c.space != bpmnModel || seen[c.attr("id")] || c == start:
		case c.name == "userTask" || c.name == "serviceTask" || c.name == "exclusiveGateway":
			wr.gap(c, "%s %s is not on the path from the start event the steps follow; left out", c.name, c.attr("id"))
		case !bpmnHeld[c.name] && c.name != "laneSet":
			wr.gap(c, "%s %s in process %s is outside the sequential subset of BPMN 2.0 the meta-model holds; left out", c.name, c.attr("id"), label)
			wr.question(fmt.Sprintf("Process %s has %s %s, which the sequential subset does not hold; how is workflow %s to be written?", label, c.name, c.attr("id"), name),
				[]string{at}, "", "Parallel paths, loops and other elements are left out of the subset until a real workflow asks for them (ADR-054), so the workflow as written may be missing a part of the process.")
		}
	}

	timers := map[string]*xmlElement{} // task id -> its timer
	for _, b := range boundaries {
		task := byID[b.attr("attachedToRef")]
		td := b.first("timerEventDefinition")
		switch {
		case task == nil || task.name != "userTask":
			wr.gap(b, "boundary event %s is not on a user task; the subset holds a timer on an approval only; left out", b.attr("id"))
		case td == nil:
			wr.gap(b, "boundary event %s on %s is not a timer; left out", b.attr("id"), task.attr("id"))
		case timers[task.attr("id")] != nil:
			wr.gap(b, "boundary event %s is a second timer on %s, and an approval has one deadline; left out", b.attr("id"), task.attr("id"))
		default:
			timers[task.attr("id")] = b
		}
	}

	var steps []*yaml.Node
	for i, s := range chain {
		ptr := fmt.Sprintf("%s/steps/%d", at, i)
		st := mapping("name", s.name)
		if s.el.name == "serviceTask" {
			set(st, "kind", "operation")
			opName := wr.operations[s.el.attr("operationRef")]
			if opName != "" && stepWord.MatchString(opName) {
				wr.question(fmt.Sprintf("Step %s of workflow %s calls operation %s, which the BPMN file names on its service task; the file does not declare it, so it waits for the tree that does. Is it that operation?", s.name, name, opName),
					[]string{ptr + "/operation"}, opName, "The file names the operation and not its method or path, so the step waits for the tree that declares it; specarch merge writes it once one does.")
			} else {
				wr.question(fmt.Sprintf("Which operation does step %s of workflow %s call? Its service task names no operation the file declares.", s.name, name),
					[]string{ptr + "/operation"}, "", "An operation step calls an operation of the specification; the service task does not say which.")
			}
			steps = append(steps, st)
			continue
		}
		set(st, "kind", "approval")
		if owners := wr.owners(s.el); len(owners) > 0 {
			set(st, "approvers", owners)
		} else {
			wr.question(fmt.Sprintf("Which roles may approve step %s of workflow %s? Its user task names no potential owner.", s.name, name),
				[]string{ptr + "/approvers"}, "", "An approval names the roles that may approve it.")
		}
		wr.question(fmt.Sprintf("Which permission does step %s of workflow %s check? It cannot be the permission of the operation that starts the workflow.", s.name, name),
			[]string{ptr + "/permission"}, "", "BPMN names who may approve and not the permission the approval checks.")
		timer := timers[s.el.attr("id")]
		deadline, onDeadline, escalateTo := "", "", ""
		if timer != nil {
			deadline, onDeadline, escalateTo = wr.deadline(timer, chain[i+1:], outgoing, byID)
		}
		if deadline != "" {
			set(st, "deadline", deadline)
		} else {
			wr.question(fmt.Sprintf("How long does step %s of workflow %s wait for an answer?", s.name, name),
				[]string{ptr + "/deadline"}, "", "An approval has a deadline, as an ISO 8601 duration in days, hours, minutes and seconds; the user task has no timer the subset holds.")
		}
		if onDeadline != "" {
			set(st, "onDeadline", onDeadline)
			if escalateTo != "" {
				set(st, "escalateTo", escalateTo)
			}
		} else {
			wr.question(fmt.Sprintf("What happens when step %s of workflow %s passes its deadline: is the request refused, or moved to a later approval?", s.name, name),
				[]string{ptr + "/onDeadline"}, "", "The timer's flow does not end the request or reach a later approval.")
		}
		steps = append(steps, st)
	}
	if len(steps) > 0 {
		set(w, "steps", steps)
	} else {
		wr.question(fmt.Sprintf("What are the steps of workflow %s? Process %s has no task the subset holds on the path from its start event.", name, label),
			[]string{at + "/steps"}, "", "A workflow has at least one step.")
	}
	says := fmt.Sprintf("Process %s defines the workflow", label)
	if len(chain) > 0 {
		var ids []string
		for _, s := range chain {
			ids = append(ids, s.el.attr("id"))
		}
		if len(ids) == 1 {
			says += ", with the task " + ids[0]
		} else {
			says += ", with the tasks " + joinAnd(ids) + " in that order"
		}
	}
	set(w, "origin", "stated")
	set(w, "cites", []*yaml.Node{wr.cite(p, says+".")})
	set(wr.workflows, name, w)
}

// chainGap reports where the chain leaves the subset, with a question on
// the workflow.
func (wr *workflowReader) chainGap(e *xmlElement, at, label, name, why string) {
	wr.gap(e, "%s; the steps after it are left out", why)
	wr.question(fmt.Sprintf("Process %s leaves the sequential subset at %s: %s. What are the steps of workflow %s from there?", label, e.attr("id"), why, name),
		[]string{at}, "", "Only the steps before the element are written, so the workflow may be missing a part of the process.")
}

// stepName is a task's name in camelCase, unique among the steps so far.
func (wr *workflowReader) stepName(e *xmlElement, chain []bpmnStep) string {
	label := e.attr("name")
	if label == "" {
		label = e.attr("id")
	}
	name := camel(strings.ReplaceAll(kebab(label), "-", "_"))
	if !stepWord.MatchString(name) {
		name = fmt.Sprintf("step%d", len(chain)+1)
		wr.gap(e, "%s %s: its name %q is not one camelCase can say; the step is named %s", e.name, e.attr("id"), label, name)
	}
	base := name
	for n := 2; ; n++ {
		taken := false
		for _, s := range chain {
			taken = taken || s.name == name
		}
		if !taken {
			break
		}
		name = fmt.Sprintf("%s%d", base, n)
	}
	if name != base {
		wr.gap(e, "%s %s: step %s is taken by an earlier task; it is named %s", e.name, e.attr("id"), base, name)
	}
	return name
}

// refusal reads an exclusive gateway right after a user task with two
// flows, one to an end event, as the refusal of that approval, and returns
// where the other goes; nil when it is not that.
func (wr *workflowReader) refusal(g *xmlElement, outgoing map[string][]*xmlElement, byID map[string]*xmlElement, chain []bpmnStep) *xmlElement {
	flows := outgoing[g.attr("id")]
	if len(chain) == 0 || chain[len(chain)-1].el.name != "userTask" || len(flows) != 2 {
		return nil
	}
	a, b := byID[flows[0].attr("targetRef")], byID[flows[1].attr("targetRef")]
	if a == nil || b == nil {
		return nil
	}
	switch {
	case b.name == "endEvent" && a.name != "endEvent":
		return a
	case a.name == "endEvent" && b.name != "endEvent":
		return b
	}
	return nil
}

// owners are the roles a user task's potential owners name.
func (wr *workflowReader) owners(task *xmlElement) []string {
	var out []string
	for _, po := range task.all("potentialOwner") {
		names := []string{}
		if ref := po.attr("resourceRef"); ref != "" {
			if n, ok := wr.resources[ref]; ok {
				names = append(names, n)
			} else {
				wr.gap(po, "potential owner of %s names resource %s, which the file does not declare; left out", task.attr("id"), ref)
			}
		}
		if rae := po.first("resourceAssignmentExpression"); rae != nil {
			if fe := rae.first("formalExpression"); fe != nil && strings.TrimSpace(fe.text) != "" {
				names = append(names, strings.TrimSpace(fe.text))
			}
		}
		for _, n := range names {
			role := kebab(n)
			switch {
			case !roleWord.MatchString(role):
				wr.gap(po, "potential owner %q of %s is not a name a role can take; left out", n, task.attr("id"))
				continue
			case role != n:
				wr.gap(po, "potential owner %q of %s is written as the role %s", n, task.attr("id"), role)
			}
			if !contains(out, role) {
				out = append(out, role)
				wr.roles[role] = append(wr.roles[role], wr.cite(po, fmt.Sprintf("Role %s is a potential owner of user task %s.", role, task.attr("id"))))
			}
		}
	}
	for _, c := range task.children {
		if c.space == bpmnModel && (c.name == "humanPerformer" || c.name == "performer") {
			wr.gap(c, "%s of %s names who does the task, and an approval names who may approve it; read the potential owners instead; left out", c.name, task.attr("id"))
		}
	}
	return out
}

var bpmnDuration = regexp.MustCompile(`^P(?:[0-9]+D|(?:[0-9]+D)?T(?:[0-9]+H(?:[0-9]+M)?(?:[0-9]+S)?|[0-9]+M(?:[0-9]+S)?|[0-9]+S))$`)

// deadline reads a timer on a user task: its duration, and refuse when its
// flow ends the request or escalate to the later approval it reaches.
func (wr *workflowReader) deadline(timer *xmlElement, later []bpmnStep, outgoing map[string][]*xmlElement, byID map[string]*xmlElement) (string, string, string) {
	if timer.attr("cancelActivity") == "false" {
		wr.gap(timer, "timer %s does not interrupt its task, and a deadline ends the wait; left out", timer.attr("id"))
		return "", "", ""
	}
	td := timer.first("timerEventDefinition")
	duration := ""
	switch d := td.first("timeDuration"); {
	case d == nil:
		wr.gap(timer, "timer %s gives a date or a cycle, and a deadline is a duration; asked for instead", timer.attr("id"))
	case !bpmnDuration.MatchString(strings.TrimSpace(d.text)) || zeroDuration(strings.TrimSpace(d.text)):
		wr.gap(d, "timer %s waits %s, and a deadline is an ISO 8601 duration in days, hours, minutes and seconds that is not zero; asked for instead", timer.attr("id"), strings.TrimSpace(d.text))
	default:
		duration = strings.TrimSpace(d.text)
	}
	flows := outgoing[timer.attr("id")]
	if len(flows) != 1 {
		wr.gap(timer, "timer %s has %d outgoing sequence flows, and a deadline goes one way; asked for instead", timer.attr("id"), len(flows))
		return duration, "", ""
	}
	target := byID[flows[0].attr("targetRef")]
	switch {
	case target != nil && target.name == "endEvent":
		return duration, "refuse", ""
	case target != nil && target.name == "userTask":
		for _, s := range later {
			if s.el == target {
				return duration, "escalate", s.name
			}
		}
	}
	wr.gap(timer, "timer %s goes to %s, neither an end event nor a later user task; asked for instead", timer.attr("id"), flows[0].attr("targetRef"))
	return duration, "", ""
}

func zeroDuration(d string) bool {
	return strings.Trim(d, "PTDHMS0") == ""
}

// source0Has says whether a mapping holds a key.
func source0Has(m *yaml.Node, key string) bool {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return true
		}
	}
	return false
}
