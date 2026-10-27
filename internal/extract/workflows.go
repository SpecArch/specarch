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

// The elements of a process the subset holds (ADR-054, ADR-198).
var bpmnHeld = map[string]bool{
	"startEvent": true, "endEvent": true, "userTask": true, "serviceTask": true,
	"exclusiveGateway": true, "sequenceFlow": true, "boundaryEvent": true, "documentation": true,
	"receiveTask": true, "intermediateCatchEvent": true, "association": true,
}

// expressionLanguage is the language generate bpmn names on the conditions
// and moments it writes: SpecArch's subset of CEL over the subject.
const expressionLanguage = "urn:specarch:expression"

// channelMessage is a message's name as a wait names it, channel/Message.
var channelMessage = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*/[A-Z][A-Za-z0-9]*$`)

var stepWord = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)

// bpmnStep is one step of a process's chain, as read.
type bpmnStep struct {
	el   *xmlElement
	name string
	// approval marks a user task the refusal gateway follows; a user
	// task without one is a task.
	approval bool
}

// workflowReader reads one BPMN file.
type workflowReader struct {
	file       string // the file, from the repository's root
	key        string
	res        *Result
	operations map[string]string // operation id -> name
	resources  map[string]string // resource id -> name
	messages   map[string]string // message id -> name
	workflows  *yaml.Node
	roles      map[string][]*yaml.Node // role -> its citations
	questions  *yaml.Node
	nextID     int
	notHeld    []heldLine
	at         string // the workflow being read, or "" outside a process
	// generated marks a file generate bpmn wrote, whose user task with no
	// gateway after it is a task by construction, not by a guess.
	generated bool
}

// heldLine is one thing the subset does not hold, at its line of the file,
// so the questions and lines come in the file's order.
type heldLine struct {
	line int
	notHeld
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
		operations: map[string]string{}, resources: map[string]string{}, messages: map[string]string{}, workflows: mapping(),
		roles: map[string][]*yaml.Node{}, questions: mapping()}
	commitLine(wr.res, r)
	if line, text, ok := generatedMark(filepath.Join(r.Repository.Root, filepath.FromSlash(file))); ok {
		wr.res.say("generated: %s:%d says it is generated from another source: %s", file, line, text)
		wr.generated = strings.Contains(text, "specarch-gen-bpmn")
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
	src, err := codeSource(r, out, []*yaml.Node{flow(mapping("clause", file, "title", "The BPMN 2.0 process definitions"))})
	if err != nil {
		return nil, err
	}
	description := fmt.Sprintf("The workflows the BPMN 2.0 file %s defines at commit %s, in the sequential subset the meta-model holds: approvals, operation steps and deadlines. Every workflow cites the file; what it does not say, or says outside the subset, is a question.\n", file, r.Commit)
	wr.finish(src, "Workflows of "+file, description, "one workflow per process, one role per potential owner, and one question per thing the file does not say or the subset does not hold",
		"The BPMN file names it as a potential owner and says nothing of what it may do.", "A potential owner in BPMN is a name; the grants are where the running permission check reads them.", "The BPMN file names the role and does not describe it.")
	return wr.res, nil
}

// finish writes the tree of the workflows read: the roles with their two
// questions each, the workflows, the questions, the line counting what was
// written, the lines for what was not held, and the root file.
func (wr *workflowReader) finish(src *yaml.Node, title, description, wroteWhat, roleSays, grantsWhy, describeWhy string) {
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
			wr.question(fmt.Sprintf("Which permissions does role %s grant? %s", name, roleSays),
				[]string{"#/roles/" + name + "/permissions"}, "", grantsWhy)
			wr.question(fmt.Sprintf("What is role %s, in a sentence a reviewer can check?", name),
				[]string{"#/roles/" + name + "/description"}, "", describeWhy)
		}
		set(design, "roles", roles)
	}
	sortPairs(wr.workflows)
	set(design, "workflows", wr.workflows)
	res := wr.res
	res.Tree.put("design/workflows.yaml", design)
	res.Tree.put("requirements/stakeholders.yaml", mapping("stakeholders", ownerStakeholder()))
	res.Tree.put("design/questions.yaml", mapping("questions", wr.questions))
	sort.SliceStable(wr.notHeld, func(i, j int) bool { return wr.notHeld[i].line < wr.notHeld[j].line })
	var held []notHeld
	for _, h := range wr.notHeld {
		held = append(held, h.notHeld)
	}
	res.say("wrote %s, %s and %s: %s",
		plural(len(wr.workflows.Content)/2, "workflow"), plural(len(wr.roles), "role"), plural(wr.nextID+couldCount(oneQuestions(wr.questions), held), "question"), wroteWhat)
	askNotHeld(res, oneQuestions(wr.questions), &wr.nextID, wr.key, held)
	res.Tree.put("specarch.yaml", rootFile(title, description, []string{"requirements", "design"}, mapping(wr.key, src)))
}

// gap records what the subset does not hold at an element of the file,
// blocking the workflow being read, or the workflows outside a process.
// asked is the pointer a question the reader asks for it blocks, or "".
func (wr *workflowReader) gap(e *xmlElement, asked string, format string, args ...any) {
	wr.gapAt(e.line, fmt.Sprintf("%s:%d", wr.file, e.line), asked, format, args...)
}

// gapAt records what is not held where the source says it, at a clause,
// ordered by line: the line of the file, or the order read.
func (wr *workflowReader) gapAt(line int, clause, asked string, format string, args ...any) {
	block := wr.at
	if block == "" {
		block = "workflows"
	}
	wr.notHeld = append(wr.notHeld, heldLine{line, notHeld{text: clause + ": " + fmt.Sprintf(format, args...), clause: clause, blocks: []string{block}, asked: asked}})
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
			wr.gap(c, "", "%s is not in BPMN's semantic model (its namespace is %s), such as diagram layout or a tool's extension; left out", c.name, c.space)
		case c.name == "process":
			processes = append(processes, c)
		case c.name == "interface":
			for _, op := range c.all("operation") {
				wr.operations[op.attr("id")] = op.attr("name")
			}
		case c.name == "resource":
			wr.resources[c.attr("id")] = c.attr("name")
		case c.name == "message":
			wr.messages[c.attr("id")] = c.attr("name")
		case bpmnReferenced[c.name]:
		default:
			wr.gap(c, "", "%s %s is outside the sequential subset of BPMN 2.0 the meta-model holds; left out", c.name, c.attr("id"))
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
		wr.gap(p, "", "process %s: its name is not one kebab-case can say; the workflow is named %s", label, base)
	}
	if name != base {
		wr.gap(p, "", "process %s: workflow %s is taken by another process; it is named %s", label, base, name)
	}
	byID := map[string]*xmlElement{}
	outgoing := map[string][]*xmlElement{}
	var starts []*xmlElement
	var boundaries []*xmlElement
	var laneSets []*xmlElement
	for _, c := range p.children {
		if c.space != bpmnModel {
			wr.gap(c, "", "%s in process %s is not in BPMN's semantic model; left out", c.name, label)
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
			laneSets = append(laneSets, c)
		}
	}
	// Lanes that put each user task in the lane of one of its potential
	// owners, as generate bpmn writes them (ADR-139), say nothing the
	// potential owners do not, and are read past.
	for _, ls := range laneSets {
		if !wr.lanesFollowOwners(ls, byID) {
			wr.gap(ls, "", "the lanes of process %s are not read: a potential owner on each user task names who approves; left out", label)
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
			opName = wr.operations[refOf(med, "operationRef")]
		}
		switch {
		case opName != "" && stepWord.MatchString(opName):
			wr.question(fmt.Sprintf("Workflow %s is started by operation %s, which the BPMN file names on its start event; the file does not declare it, so it waits for the tree that does. Is it that operation?", name, opName),
				[]string{at + "/trigger"}, opName, "The file names the operation and not its method or path, so the workflow's trigger waits for the tree that declares it; specarch merge writes it once one does.")
		case opName != "":
			wr.gap(start, at+"/trigger", "start event %s names operation %s, which is not a name an operationId can take; asked for instead", start.attr("id"), opName)
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
		wr.gap(where, at+"/trigger", "process %s has %d start events, and a workflow starts from one operation; its trigger and steps are asked for", label, len(starts))
		wr.question(fmt.Sprintf("Which operation starts workflow %s, and what are its steps? Process %s has %d start events.", name, label, len(starts)),
			[]string{at + "/trigger"}, "", "A workflow in the sequential subset has one start.")
	}

	// The chain of steps from the start event: each step's one flow leads
	// to the next, and a decision's branches to later steps, which are
	// walked in the order of the file once the path before them ends.
	var chain []bpmnStep
	seen := map[string]bool{}
	order := map[string]int{}
	for i, c := range p.children {
		order[c.attr("id")] = i
	}
	var pending []*xmlElement // the steps a decision's branches lead to, not walked yet
	var ended *xmlElement     // the end event the path before reached, or nil
	// enter reads one element reached on the path and returns the element
	// whose flow the path follows on, or nil where it stops.
	enter := func(next *xmlElement) *xmlElement {
		switch next.name {
		case "userTask", "serviceTask", "receiveTask", "intermediateCatchEvent":
			chain = append(chain, bpmnStep{el: next, name: wr.stepName(next, chain)})
			return next
		case "exclusiveGateway":
			if !specarchConditions(next, outgoing) {
				if onward := wr.refusal(next, outgoing, byID, chain); onward != nil {
					chain[len(chain)-1].approval = true
					seen[onward.attr("id")] = true
					switch onward.name {
					case "endEvent":
						ended = onward
						return nil
					case "userTask", "serviceTask", "receiveTask", "intermediateCatchEvent":
						chain = append(chain, bpmnStep{el: onward, name: wr.stepName(onward, chain)})
						return onward
					}
					wr.chainGap(onward, at, label, name, fmt.Sprintf("%s %s follows the gateway, and the subset holds a task or an end event there", onward.name, onward.attr("id")))
					return nil
				}
			}
			chain = append(chain, bpmnStep{el: next, name: wr.stepName(next, chain)})
			for _, f := range outgoing[next.attr("id")] {
				if t := byID[f.attr("targetRef")]; t != nil && t.name != "endEvent" {
					pending = append(pending, t)
				}
			}
			return nil
		case "endEvent":
			ended = next
			return nil
		}
		wr.chainGap(next, at, label, name, fmt.Sprintf("%s %s is not a step of the sequential subset", next.name, next.attr("id")))
		return nil
	}
	cur := start
	for cur != nil {
		for cur != nil {
			flows := outgoing[cur.attr("id")]
			if len(flows) != 1 {
				if cur.name != "endEvent" {
					wr.chainGap(cur, at, label, name, fmt.Sprintf("%s %s has %d outgoing sequence flows, and a step of the sequential subset has one", cur.name, cur.attr("id"), len(flows)))
				}
				pending = nil
				break
			}
			next := byID[flows[0].attr("targetRef")]
			if next == nil {
				wr.gap(flows[0], "", "sequence flow %s goes to %s, which process %s does not hold; the steps after it are left out", flows[0].attr("id"), flows[0].attr("targetRef"), label)
				pending = nil
				break
			}
			if seen[next.attr("id")] {
				wr.chainGap(next, at, label, name, fmt.Sprintf("the sequence flows come back to %s %s, a loop", next.name, next.attr("id")))
				pending = nil
				break
			}
			seen[next.attr("id")] = true
			cur = enter(next)
		}
		// The path stopped at a decision or an end: go on at the earliest
		// branch's step in the file not walked yet.
		sort.SliceStable(pending, func(i, j int) bool { return order[pending[i].attr("id")] < order[pending[j].attr("id")] })
		for len(pending) > 0 && seen[pending[0].attr("id")] {
			pending = pending[1:]
		}
		if len(pending) == 0 {
			break
		}
		next := pending[0]
		pending = pending[1:]
		if ended != nil {
			wr.chainGap(ended, at, label, name, fmt.Sprintf("the path ends at %s %s before %s %s, which a decision leads to, and only a decision's branch ends the request before its last step", ended.name, ended.attr("id"), next.name, next.attr("id")))
			break
		}
		seen[next.attr("id")] = true
		cur = enter(next)
		ended = nil
	}
	compensation := map[string]bool{} // the tasks that undo a step, outside the path
	for _, c := range p.children {
		if c.space == bpmnModel && c.name == "association" {
			if t := byID[c.attr("targetRef")]; t != nil && t.attr("isForCompensation") == "true" {
				compensation[t.attr("id")] = true
			}
		}
	}
	for _, c := range p.children {
		switch {
		case c.space != bpmnModel || seen[c.attr("id")] || c == start || compensation[c.attr("id")]:
		case c.name == "userTask" || c.name == "serviceTask" || c.name == "exclusiveGateway" || c.name == "receiveTask" || c.name == "intermediateCatchEvent":
			wr.gap(c, "", "%s %s is not on the path from the start event the steps follow; left out", c.name, c.attr("id"))
		case !bpmnHeld[c.name] && c.name != "laneSet":
			wr.gap(c, at, "%s %s in process %s is outside the sequential subset of BPMN 2.0 the meta-model holds; left out", c.name, c.attr("id"), label)
			wr.question(fmt.Sprintf("Process %s has %s %s, which the sequential subset does not hold; how is workflow %s to be written?", label, c.name, c.attr("id"), name),
				[]string{at}, "", "Parallel paths, loops and other elements are left out of the subset until a real workflow asks for them (ADR-054), so the workflow as written may be missing a part of the process.")
		}
	}

	timers := map[string]*xmlElement{}   // task id -> its timer
	failures := map[string]*xmlElement{} // service task id -> its error event
	undoes := map[string]*xmlElement{}   // service task id -> the task that undoes it
	for _, b := range boundaries {
		task := byID[b.attr("attachedToRef")]
		td := b.first("timerEventDefinition")
		switch {
		case task != nil && task.name == "serviceTask" && b.first("errorEventDefinition") != nil:
			if failures[task.attr("id")] != nil {
				wr.gap(b, "", "boundary event %s is a second error event on %s, and a step has one onFailure; left out", b.attr("id"), task.attr("id"))
				continue
			}
			failures[task.attr("id")] = b
		case task != nil && task.name == "serviceTask" && b.first("compensateEventDefinition") != nil:
			for _, c := range p.children {
				if c.space == bpmnModel && c.name == "association" && c.attr("sourceRef") == b.attr("id") {
					if h := byID[c.attr("targetRef")]; h != nil && h.name == "serviceTask" && h.attr("isForCompensation") == "true" {
						undoes[task.attr("id")] = h
					}
				}
			}
			if undoes[task.attr("id")] == nil {
				wr.gap(b, "", "compensation event %s on %s is tied to no service task marked isForCompensation; left out", b.attr("id"), task.attr("id"))
			}
		case task == nil || task.name != "userTask" && task.name != "receiveTask":
			wr.gap(b, "", "boundary event %s is not on a user task; the subset holds a timer on an approval only; left out", b.attr("id"))
		case td == nil:
			wr.gap(b, "", "boundary event %s on %s is not a timer; left out", b.attr("id"), task.attr("id"))
		case timers[task.attr("id")] != nil:
			wr.gap(b, "", "boundary event %s is a second timer on %s, and an approval has one deadline; left out", b.attr("id"), task.attr("id"))
		default:
			timers[task.attr("id")] = b
		}
	}
	index := map[*xmlElement]int{}
	for i, s := range chain {
		index[s.el] = i
	}
	// A user task an approval escalates to is an approval too, gateway or
	// not, since an approval escalates only to a later approval.
	for _, st := range chain {
		if t := timers[st.el.attr("id")]; st.approval && t != nil {
			for _, f := range outgoing[t.attr("id")] {
				if j, ok := index[byID[f.attr("targetRef")]]; ok && chain[j].el.name == "userTask" {
					chain[j].approval = true
				}
			}
		}
	}
	// target is where a flow from step i goes on: a later step by name, or
	// how an end event ends the request.
	target := func(i int, f *xmlElement) (to, end string, ok bool) {
		t := byID[f.attr("targetRef")]
		switch {
		case t == nil:
			return "", "", false
		case t.name == "endEvent":
			return "", endOf(t), true
		}
		if j, found := index[t]; found && j > i {
			return chain[j].name, "", true
		}
		return "", "", false
	}

	var steps []*yaml.Node
	for i, s := range chain {
		ptr := fmt.Sprintf("%s/steps/%d", at, i)
		st := mapping("name", s.name)
		switch {
		case s.el.name == "serviceTask":
			set(st, "kind", "operation")
			opName := wr.operations[s.el.attr("operationRef")]
			if opName != "" && stepWord.MatchString(opName) {
				wr.question(fmt.Sprintf("Step %s of workflow %s calls operation %s, which the BPMN file names on its service task; the file does not declare it, so it waits for the tree that does. Is it that operation?", s.name, name, opName),
					[]string{ptr + "/operation"}, opName, "The file names the operation and not its method or path, so the step waits for the tree that declares it; specarch merge writes it once one does.")
			} else {
				wr.question(fmt.Sprintf("Which operation does step %s of workflow %s call? Its service task names no operation the file declares.", s.name, name),
					[]string{ptr + "/operation"}, "", "An operation step calls an operation of the specification; the service task does not say which.")
			}
			if e := failures[s.el.attr("id")]; e != nil {
				flows := outgoing[e.attr("id")]
				to, end, ok := "", "", false
				if len(flows) == 1 {
					to, end, ok = target(i, flows[0])
				}
				switch {
				case ok && to != "":
					set(st, "onFailure", mapping("to", to))
				case ok && end == "failed":
					set(st, "onFailure", mapping("end", "failed"))
				default:
					wr.gap(e, ptr+"/onFailure", "error event %s on %s goes neither to a later step nor to the failed end; asked for instead", e.attr("id"), s.el.attr("id"))
					wr.question(fmt.Sprintf("What does workflow %s do when step %s fails: go on at a later step, or end failed?", name, s.name),
						[]string{ptr + "/onFailure"}, "", "The error event's flow does not reach a later step or an end named failed.")
				}
			}
			if h := undoes[s.el.attr("id")]; h != nil {
				opName := wr.operations[h.attr("operationRef")]
				if opName != "" && stepWord.MatchString(opName) {
					wr.question(fmt.Sprintf("Step %s of workflow %s is undone by operation %s, which the BPMN file names on its compensation task; the file does not declare it, so it waits for the tree that does. Is it that operation?", s.name, name, opName),
						[]string{ptr + "/compensatedBy"}, opName, "The file names the operation and not its method or path, so the step waits for the tree that declares it; specarch merge writes it once one does.")
				} else {
					wr.question(fmt.Sprintf("Which operation undoes step %s of workflow %s? Its compensation task names no operation the file declares.", s.name, name),
						[]string{ptr + "/compensatedBy"}, "", "A step is undone by an operation of the specification; the compensation task does not say which.")
				}
			}
			steps = append(steps, st)
			continue
		case s.el.name == "exclusiveGateway":
			set(st, "kind", "decision")
			var branches, otherwise []*yaml.Node
			for j, f := range outgoing[s.el.attr("id")] {
				br := mapping()
				bptr := fmt.Sprintf("%s/branches/%d", ptr, j)
				isDefault := s.el.attr("default") == f.attr("id")
				if isDefault {
					set(br, "otherwise", true)
				} else if ce := f.first("conditionExpression"); ce != nil && strings.TrimSpace(ce.text) != "" && ce.attr("language") == expressionLanguage {
					set(br, "when", strings.TrimSpace(ce.text))
				} else {
					says := "no condition"
					if ce != nil && strings.TrimSpace(ce.text) != "" {
						says = fmt.Sprintf("the condition %q in the language %q", strings.TrimSpace(ce.text), ce.attr("language"))
					}
					wr.gap(f, bptr+"/when", "sequence flow %s of decision %s carries %s, and a branch's condition is an expression over the subject; asked for instead", f.attr("id"), s.el.attr("id"), says)
					wr.question(fmt.Sprintf("When does decision %s of workflow %s take sequence flow %s? It carries %s.", s.name, name, f.attr("id"), says),
						[]string{bptr + "/when"}, "", "A branch's condition is read over the subject's fields in SpecArch's subset of CEL; another language, or none, cannot be checked against the subject.")
				}
				switch to, end, ok := target(i, f); {
				case ok && to != "":
					set(br, "to", to)
				case ok:
					set(br, "end", end)
				default:
					wr.gap(f, bptr, "sequence flow %s of decision %s goes to %s, which is neither a later step nor an end; asked for instead", f.attr("id"), s.el.attr("id"), f.attr("targetRef"))
					wr.question(fmt.Sprintf("Where does decision %s of workflow %s go on by sequence flow %s?", s.name, name, f.attr("id")),
						[]string{bptr}, "", "A branch goes on at a later step or ends the request; the flow does neither.")
				}
				if isDefault {
					otherwise = append(otherwise, br)
				} else {
					branches = append(branches, br)
				}
			}
			set(st, "branches", append(branches, otherwise...))
			steps = append(steps, st)
			continue
		case s.el.name == "receiveTask" || s.el.name == "intermediateCatchEvent":
			set(st, "kind", "wait")
			wr.wait(s, st, ptr, name)
			if timer := timers[s.el.attr("id")]; timer != nil {
				deadline, onDeadline, _ := wr.deadline(ptr, timer, nil, outgoing, byID)
				if deadline != "" {
					set(st, "deadline", deadline)
				}
				if onDeadline != "" {
					set(st, "onDeadline", onDeadline)
				}
			}
			steps = append(steps, st)
			continue
		case !s.approval:
			set(st, "kind", "task")
			if owners := wr.owners(s.el); len(owners) > 0 {
				set(st, "performers", owners)
			} else {
				wr.question(fmt.Sprintf("Which roles may do step %s of workflow %s? Its user task names no potential owner.", s.name, name),
					[]string{ptr + "/performers"}, "", "A task names the roles whose people may do it.")
			}
			wr.question(fmt.Sprintf("Which permission does step %s of workflow %s check?", s.name, name),
				[]string{ptr + "/permission"}, "", "BPMN names who may do a task and not the permission it checks.")
			if !wr.generated {
				wr.gap(s.el, "", "user task %s has no gateway after it that refuses the request, so step %s is read as a task; an approval is a user task followed by one", s.el.attr("id"), s.name)
			}
			if timer := timers[s.el.attr("id")]; timer != nil {
				deadline, onDeadline, escalateTo := wr.deadline(ptr, timer, chain[i+1:], outgoing, byID)
				if deadline != "" {
					set(st, "deadline", deadline)
				} else {
					wr.question(fmt.Sprintf("How long does step %s of workflow %s wait for the work to be done?", s.name, name),
						[]string{ptr + "/deadline"}, "", "The user task has a timer the subset does not hold; a task's deadline is an ISO 8601 duration in days, hours, minutes and seconds.")
				}
				if onDeadline != "" {
					set(st, "onDeadline", onDeadline)
					if escalateTo != "" {
						set(st, "escalateTo", escalateTo)
					}
				}
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
			deadline, onDeadline, escalateTo = wr.deadline(ptr, timer, chain[i+1:], outgoing, byID)
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
	wr.gap(e, at, "%s; the steps after it are left out", why)
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
		wr.gap(e, "", "%s %s: its name %q is not one camelCase can say; the step is named %s", e.name, e.attr("id"), label, name)
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
		wr.gap(e, "", "%s %s: step %s is taken by an earlier task; it is named %s", e.name, e.attr("id"), base, name)
	}
	return name
}

// specarchConditions says whether a gateway's flows carry a condition in
// the language generate bpmn names, which marks it a decision step rather
// than an approval's refusal.
func specarchConditions(g *xmlElement, outgoing map[string][]*xmlElement) bool {
	for _, f := range outgoing[g.attr("id")] {
		if ce := f.first("conditionExpression"); ce != nil && ce.attr("language") == expressionLanguage {
			return true
		}
	}
	return false
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
	case a.name == "endEvent" && b.name == "endEvent":
		// An approval that is the last step: the gateway's default flow is
		// the refusal, and the other ends the request approved.
		switch g.attr("default") {
		case flows[1].attr("id"):
			return a
		case flows[0].attr("id"):
			return b
		}
	}
	return nil
}

// refOf is a reference an element makes by name: the child element the
// BPMN 2.0 schema writes it as (resourceRef in a potential owner,
// operationRef in a message event definition), or the attribute some tools
// write instead.
func refOf(e *xmlElement, name string) string {
	if c := e.first(name); c != nil {
		return strings.TrimSpace(c.text)
	}
	return e.attr(name)
}

// owners are the roles a user task's potential owners name.
func (wr *workflowReader) owners(task *xmlElement) []string {
	var out []string
	for _, po := range task.all("potentialOwner") {
		names := []string{}
		if ref := refOf(po, "resourceRef"); ref != "" {
			if n, ok := wr.resources[ref]; ok {
				names = append(names, n)
			} else {
				wr.gap(po, "", "potential owner of %s names resource %s, which the file does not declare; left out", task.attr("id"), ref)
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
				wr.gap(po, "", "potential owner %q of %s is not a name a role can take; left out", n, task.attr("id"))
				continue
			case role != n:
				wr.gap(po, "", "potential owner %q of %s is written as the role %s", n, task.attr("id"), role)
			}
			if !contains(out, role) {
				out = append(out, role)
				wr.roles[role] = append(wr.roles[role], wr.cite(po, fmt.Sprintf("Role %s is a potential owner of user task %s.", role, task.attr("id"))))
			}
		}
	}
	for _, c := range task.children {
		if c.space == bpmnModel && (c.name == "humanPerformer" || c.name == "performer") {
			wr.gap(c, "", "%s of %s names who does the task, and an approval names who may approve it; read the potential owners instead; left out", c.name, task.attr("id"))
		}
	}
	return out
}

var bpmnDuration = regexp.MustCompile(`^P(?:[0-9]+D|(?:[0-9]+D)?T(?:[0-9]+H(?:[0-9]+M)?(?:[0-9]+S)?|[0-9]+M(?:[0-9]+S)?|[0-9]+S))$`)

// deadline reads a timer on a user task: its duration, and refuse when its
// flow ends the request or escalate to the later approval it reaches.
func (wr *workflowReader) deadline(ptr string, timer *xmlElement, later []bpmnStep, outgoing map[string][]*xmlElement, byID map[string]*xmlElement) (string, string, string) {
	if timer.attr("cancelActivity") == "false" {
		wr.gap(timer, "", "timer %s does not interrupt its task, and a deadline ends the wait; left out", timer.attr("id"))
		return "", "", ""
	}
	td := timer.first("timerEventDefinition")
	duration := ""
	switch d := td.first("timeDuration"); {
	case d == nil:
		wr.gap(timer, ptr+"/deadline", "timer %s gives a date or a cycle, and a deadline is a duration; asked for instead", timer.attr("id"))
	case !bpmnDuration.MatchString(strings.TrimSpace(d.text)) || zeroDuration(strings.TrimSpace(d.text)):
		wr.gap(d, ptr+"/deadline", "timer %s waits %s, and a deadline is an ISO 8601 duration in days, hours, minutes and seconds that is not zero; asked for instead", timer.attr("id"), strings.TrimSpace(d.text))
	default:
		duration = strings.TrimSpace(d.text)
	}
	flows := outgoing[timer.attr("id")]
	if len(flows) != 1 {
		wr.gap(timer, ptr+"/onDeadline", "timer %s has %d outgoing sequence flows, and a deadline goes one way; asked for instead", timer.attr("id"), len(flows))
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
	wr.gap(timer, ptr+"/onDeadline", "timer %s goes to %s, neither an end event nor a later user task; asked for instead", timer.attr("id"), flows[0].attr("targetRef"))
	return duration, "", ""
}

// endOf is how an end event ends the request, by its name or id as
// generate bpmn writes them: refused, failed, or approved.
func endOf(e *xmlElement) string {
	for _, w := range []string{strings.ToLower(e.attr("name")), strings.ToLower(e.attr("id"))} {
		switch {
		case strings.Contains(w, "refus"):
			return "refused"
		case strings.Contains(w, "fail"):
			return "failed"
		}
	}
	return "approved"
}

// wait reads what a wait step waits for: a receive task's or a message
// catch event's message, or a timer catch event's duration or moment.
func (wr *workflowReader) wait(s bpmnStep, st *yaml.Node, ptr, name string) {
	ref := s.el.attr("messageRef")
	if med := s.el.first("messageEventDefinition"); med != nil {
		ref = med.attr("messageRef")
	}
	td := s.el.first("timerEventDefinition")
	switch {
	case s.el.name == "receiveTask" || s.el.first("messageEventDefinition") != nil:
		if m := wr.messages[ref]; channelMessage.MatchString(m) {
			wr.question(fmt.Sprintf("Step %s of workflow %s waits for message %s, which the BPMN file names and does not declare; is it the message %s declared under channels?", s.name, name, m, m),
				[]string{ptr + "/message"}, "", "The file names the message and not its channel or payload; write message: "+m+" once its channel is declared.")
		} else {
			wr.gap(s.el, ptr+"/message", "%s %s waits for message %q, which is not a channel and one of its messages; asked for instead", s.el.name, s.el.attr("id"), m)
			wr.question(fmt.Sprintf("Which channel message does step %s of workflow %s wait for?", s.name, name),
				[]string{ptr + "/message"}, "", "A wait names a message declared under channels, as channel/Message; the file's message is not one.")
		}
	case td != nil && td.first("timeDuration") != nil && bpmnDuration.MatchString(strings.TrimSpace(td.first("timeDuration").text)) && !zeroDuration(strings.TrimSpace(td.first("timeDuration").text)):
		set(st, "after", strings.TrimSpace(td.first("timeDuration").text))
	case td != nil && td.first("timeDate") != nil && td.first("timeDate").attr("language") == expressionLanguage:
		set(st, "at", strings.TrimSpace(td.first("timeDate").text))
	default:
		wr.gap(s.el, ptr, "%s %s waits for something other than a message, a duration in days, hours, minutes and seconds, or a moment over the subject; asked for instead", s.el.name, s.el.attr("id"))
		wr.question(fmt.Sprintf("What does step %s of workflow %s wait for: a message, a duration, or a moment of the subject?", s.name, name),
			[]string{ptr}, "", "A wait waits for one of a channel message, a duration, or an expression over the subject giving a moment.")
	}
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

// lanesFollowOwners says whether a lane set has lanes and every lane that
// holds a user task is named after one of that task's potential owners,
// with no lane set nested in a lane and no reference to an element the
// process does not have.
func (wr *workflowReader) lanesFollowOwners(ls *xmlElement, byID map[string]*xmlElement) bool {
	if len(ls.all("lane")) == 0 {
		return false
	}
	for _, l := range ls.all("lane") {
		if l.first("childLaneSet") != nil {
			return false
		}
		for _, ref := range l.all("flowNodeRef") {
			node := byID[strings.TrimSpace(ref.text)]
			if node == nil {
				return false
			}
			if node.name != "userTask" {
				continue
			}
			owned := false
			for _, po := range node.all("potentialOwner") {
				owned = owned || l.attr("name") != "" && wr.resources[refOf(po, "resourceRef")] == l.attr("name")
			}
			if !owned {
				return false
			}
		}
	}
	return true
}
