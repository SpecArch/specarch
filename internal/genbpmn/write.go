package genbpmn

import (
	"fmt"
	"slices"
	"strings"

	"github.com/SpecArch/specarch/internal/mark"
)

// The ids the marks of the file as a whole and of the process are kept
// under. The process's id is fixed, as every other id is fixed or
// prefixed, so no workflow, step, role or operation name can make two
// elements share one.
const (
	definitionsID = "definitions"
	processID     = "process"
)

// marksOf places each problem at the deepest element the file shows that
// holds its pointer, by the rule of internal/mark: the whole workflows
// section, and an error no element holds, at the definitions.
func marksOf(wf *workflow, problems []mark.Problem) map[string][]string {
	byPointer := map[string]string{wf.ptr: processID}
	for _, s := range wf.shapes {
		switch {
		case s.kind == "startEvent", s.kind == "userTask", s.kind == "serviceTask", s.kind == "receiveTask", s.kind == "intermediateCatchEvent":
			byPointer[s.ptr] = s.id
		case s.kind == "exclusiveGateway" && text(s.step["kind"]) == "decision":
			byPointer[s.ptr] = s.id
		case s.kind == "boundaryEvent" && s.event == "error":
			byPointer[s.ptr] = s.id
		case s.kind == "boundaryEvent" && s.event == "timer":
			step := strings.TrimSuffix(s.ptr, "/deadline")
			for _, key := range []string{"deadline", "onDeadline", "escalateTo"} {
				byPointer[step+"/"+key] = s.id
			}
		}
	}
	for _, r := range wf.roles {
		byPointer["/roles/"+escape(r)] = "role-" + r
	}
	for _, op := range wf.operations {
		if p := wf.opPointers[op]; p != "" {
			byPointer[p] = "operation-" + op
		}
	}
	elements := make([]string, 0, len(byPointer))
	for p := range byPointer {
		elements = append(elements, p)
	}
	byPointer[mark.File] = definitionsID
	out := map[string][]string{}
	// Another workflow's problem is not this file's, even the target's own.
	var mine []mark.Problem
	for _, p := range problems {
		if !strings.HasPrefix(p.Pointer, "/workflows/") || p.Pointer == wf.ptr || strings.HasPrefix(p.Pointer, wf.ptr+"/") {
			mine = append(mine, p)
		}
	}
	placed := mark.Place(mine, elements, []string{"/workflows"})
	slices.Sort(elements)
	for _, p := range problems {
		line := comment(mark.Line(p))
		for _, ptr := range append([]string{mark.File}, elements...) {
			id := byPointer[ptr]
			if slices.ContainsFunc(placed[ptr], func(q mark.Problem) bool { return q.ID == p.ID }) && !slices.Contains(out[id], line) {
				out[id] = append(out[id], line)
			}
		}
	}
	return out
}

// comment is text as an XML comment, which may not hold "--" or end in
// "-": each "--" is written "- -".
func comment(s string) string {
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "- -")
	}
	return "<!-- " + s + " -->"
}

var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

func esc(s string) string { return xmlEscaper.Replace(s) }

// partial says what the file holds of the workflow and what it leaves to
// the specification, since BPMN has no element for the rest.
func (wf *workflow) partial() string {
	if wf.extended {
		return fmt.Sprintf("Workflow %s in a subset of BPMN 2.0 with no loops and no parallel paths: its steps, the roles that approve or do a task, the deadlines, the decisions with their conditions, the waits, the failures, the operations that undo a step, and the operations and messages by name. The subject, the permission each approval or task checks, the retry limits, the events and the requirements are in the specification only.", wf.name)
	}
	return fmt.Sprintf("Workflow %s in the sequential subset of BPMN 2.0: its steps, the roles that approve, the deadlines and the operations by name. The subject, the permission each approval checks, the events and the requirements are in the specification only.", wf.name)
}

// bpmn writes the workflow as a BPMN 2.0 XML file.
func (wf *workflow) bpmn(header, draft string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(comment(header) + "\n")
	b.WriteString(comment(wf.partial()) + "\n")
	if draft != "" {
		b.WriteString(comment(draft) + "\n")
	}
	mark := func(id, indent string) {
		for _, m := range wf.marks[id] {
			b.WriteString(indent + m + "\n")
		}
	}
	mark(definitionsID, "")
	fmt.Fprintf(&b, "<definitions xmlns=%q xmlns:bpmndi=%q xmlns:dc=%q xmlns:di=%q xmlns:xsi=%q id=%q targetNamespace=%q>\n",
		nsModel, nsBPMNDI, nsDC, nsDI, nsXSI, "definitions-"+wf.name, "urn:specarch:workflows:"+wf.name)
	if len(wf.operations) > 0 {
		fmt.Fprintf(&b, "  <interface id=\"interface\" name=%q>\n", esc(wf.title))
		for _, op := range wf.operations {
			mark("operation-"+op, "    ")
			fmt.Fprintf(&b, "    <operation id=\"operation-%s\" name=\"%s\">\n      <inMessageRef>message-%s</inMessageRef>\n    </operation>\n", esc(op), esc(op), esc(op))
		}
		b.WriteString("  </interface>\n")
		for _, op := range wf.operations {
			fmt.Fprintf(&b, "  <message id=\"message-%s\" name=\"%s request\"/>\n", esc(op), esc(op))
		}
	}
	for _, m := range wf.messages {
		fmt.Fprintf(&b, "  <message id=\"%s\" name=\"%s\"/>\n", esc(messageID(m)), esc(m))
	}
	for _, r := range wf.roles {
		mark("role-"+r, "  ")
		fmt.Fprintf(&b, "  <resource id=\"role-%s\" name=\"%s\"/>\n", esc(r), esc(r))
	}
	mark(processID, "  ")
	fmt.Fprintf(&b, "  <process id=\"%s\" name=\"%s\" isExecutable=\"false\">\n", processID, esc(wf.name))
	if wf.description != "" {
		fmt.Fprintf(&b, "    <documentation>%s</documentation>\n", esc(strings.Join(strings.Fields(wf.description), " ")))
	}
	b.WriteString("    <laneSet id=\"lanes\">\n")
	for i, l := range wf.lanes {
		fmt.Fprintf(&b, "      <lane id=\"%s\" name=\"%s\">\n", esc(l.id), esc(l.name))
		for _, s := range wf.shapes {
			if s.lane == i {
				fmt.Fprintf(&b, "        <flowNodeRef>%s</flowNodeRef>\n", esc(s.id))
			}
		}
		b.WriteString("      </lane>\n")
	}
	b.WriteString("    </laneSet>\n")
	for _, s := range wf.shapes {
		mark(s.id, "    ")
		switch s.kind {
		case "startEvent":
			fmt.Fprintf(&b, "    <startEvent id=\"start\" name=\"%s\">\n", esc(s.name))
			if wf.trigger != "" {
				fmt.Fprintf(&b, "      <messageEventDefinition messageRef=\"message-%s\">\n        <operationRef>operation-%s</operationRef>\n      </messageEventDefinition>\n", esc(wf.trigger), esc(wf.trigger))
			}
			b.WriteString("    </startEvent>\n")
		case "userTask":
			fmt.Fprintf(&b, "    <userTask id=\"%s\" name=\"%s\">\n", esc(s.id), esc(s.name))
			for _, r := range list(s.step[ownersKey(s.step)]) {
				fmt.Fprintf(&b, "      <potentialOwner>\n        <resourceRef>role-%s</resourceRef>\n      </potentialOwner>\n", esc(text(r)))
			}
			b.WriteString("    </userTask>\n")
		case "serviceTask":
			undo := ""
			if s.forCompensation {
				undo = ` isForCompensation="true"`
			}
			fmt.Fprintf(&b, "    <serviceTask id=\"%s\" name=\"%s\"%s operationRef=\"operation-%s\"/>\n", esc(s.id), esc(s.name), undo, esc(s.sub))
		case "receiveTask":
			fmt.Fprintf(&b, "    <receiveTask id=\"%s\" name=\"%s\" messageRef=\"%s\"/>\n", esc(s.id), esc(s.name), esc(messageID(s.sub)))
		case "intermediateCatchEvent":
			language := ""
			if s.event == "timeDate" {
				language = fmt.Sprintf(" language=%q", expressionLanguage)
			}
			fmt.Fprintf(&b, "    <intermediateCatchEvent id=\"%s\" name=\"%s\">\n      <timerEventDefinition>\n        <%s xsi:type=\"tFormalExpression\"%s>%s</%s>\n      </timerEventDefinition>\n    </intermediateCatchEvent>\n",
				esc(s.id), esc(s.name), s.event, language, esc(s.sub), s.event)
		case "boundaryEvent":
			if s.event == "error" {
				fmt.Fprintf(&b, "    <boundaryEvent id=\"%s\" attachedToRef=\"%s\" cancelActivity=\"true\">\n      <errorEventDefinition/>\n    </boundaryEvent>\n", esc(s.id), esc(s.attached))
				break
			}
			if s.event == "compensate" {
				fmt.Fprintf(&b, "    <boundaryEvent id=\"%s\" attachedToRef=\"%s\">\n      <compensateEventDefinition/>\n    </boundaryEvent>\n", esc(s.id), esc(s.attached))
				break
			}
			fmt.Fprintf(&b, "    <boundaryEvent id=\"%s\" attachedToRef=\"%s\" cancelActivity=\"true\">\n      <timerEventDefinition>\n        <timeDuration xsi:type=\"tFormalExpression\">%s</timeDuration>\n      </timerEventDefinition>\n    </boundaryEvent>\n",
				esc(s.id), esc(s.attached), esc(s.name))
		case "exclusiveGateway":
			def := ""
			if d := wf.defaults[s.id]; d != "" || text(s.step["kind"]) == "approval" {
				def = fmt.Sprintf(" default=\"%s\"", d)
			}
			fmt.Fprintf(&b, "    <exclusiveGateway id=\"%s\" name=\"%s\"%s/>\n", esc(s.id), esc(s.name), def)
		case "endEvent":
			if wf.compensated && s.id != "end-done" {
				// A request that ends refused or failed undoes the steps
				// done, each by its compensation handler.
				fmt.Fprintf(&b, "    <endEvent id=\"%s\" name=\"%s\">\n      <compensateEventDefinition/>\n    </endEvent>\n", esc(s.id), esc(s.name))
				break
			}
			fmt.Fprintf(&b, "    <endEvent id=\"%s\" name=\"%s\"/>\n", esc(s.id), esc(s.name))
		}
	}
	for _, f := range wf.flows {
		if f.condition != "" {
			language := ""
			if f.language != "" {
				language = fmt.Sprintf(" language=%q", f.language)
			}
			fmt.Fprintf(&b, "    <sequenceFlow id=\"%s\"%s sourceRef=\"%s\" targetRef=\"%s\">\n      <conditionExpression xsi:type=\"tFormalExpression\"%s>%s</conditionExpression>\n    </sequenceFlow>\n",
				f.id, nameAttr(f.label), esc(f.source), esc(f.target), language, esc(f.condition))
			continue
		}
		fmt.Fprintf(&b, "    <sequenceFlow id=\"%s\"%s sourceRef=\"%s\" targetRef=\"%s\"/>\n", f.id, nameAttr(f.label), esc(f.source), esc(f.target))
	}
	for _, a := range wf.associations {
		fmt.Fprintf(&b, "    <association id=\"%s\" sourceRef=\"%s\" targetRef=\"%s\" associationDirection=\"One\"/>\n", esc(a.id), esc(a.source), esc(a.target))
	}
	b.WriteString("  </process>\n")
	fmt.Fprintf(&b, "  <bpmndi:BPMNDiagram id=\"diagram-%s\">\n    <bpmndi:BPMNPlane id=\"plane-%s\" bpmnElement=\"%s\">\n", esc(wf.name), esc(wf.name), processID)
	for _, l := range wf.lanes {
		fmt.Fprintf(&b, "      <bpmndi:BPMNShape id=\"shape-%s\" bpmnElement=\"%s\" isHorizontal=\"true\">\n        <dc:Bounds x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\"/>\n      </bpmndi:BPMNShape>\n",
			esc(l.id), esc(l.id), laneX, l.y, wf.width-2*laneX, l.h)
	}
	for _, s := range wf.shapes {
		marker := ""
		if s.kind == "exclusiveGateway" {
			marker = ` isMarkerVisible="true"`
		}
		fmt.Fprintf(&b, "      <bpmndi:BPMNShape id=\"shape-%s\" bpmnElement=\"%s\"%s>\n        <dc:Bounds x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\"/>\n      </bpmndi:BPMNShape>\n",
			esc(s.id), esc(s.id), marker, s.x, s.y, s.w, s.h)
	}
	for _, f := range append(append([]*flow{}, wf.flows...), wf.associations...) {
		fmt.Fprintf(&b, "      <bpmndi:BPMNEdge id=\"edge-%s\" bpmnElement=\"%s\">\n", f.id, f.id)
		for _, p := range f.points {
			fmt.Fprintf(&b, "        <di:waypoint x=\"%d\" y=\"%d\"/>\n", p[0], p[1])
		}
		b.WriteString("      </bpmndi:BPMNEdge>\n")
	}
	b.WriteString("    </bpmndi:BPMNPlane>\n  </bpmndi:BPMNDiagram>\n</definitions>\n")
	return b.String()
}

// svg draws the same diagram as an SVG picture, in the colours of the
// reader's light or dark scheme.
func (wf *workflow) svg(header, draft string) string {
	top := 0
	if draft != "" {
		top = 24
	}
	var b strings.Builder
	b.WriteString(comment(header) + "\n")
	fmt.Fprintf(&b, "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 %d %d\" width=\"%d\" height=\"%d\" role=\"img\" aria-labelledby=\"title-%s\">\n",
		wf.width, wf.height+top, wf.width, wf.height+top, esc(wf.name))
	fmt.Fprintf(&b, "  <title id=\"title-%s\">Workflow %s</title>\n", esc(wf.name), esc(wf.name))
	b.WriteString(`  <style>
    .line { fill: none; stroke: #1f2328; stroke-width: 1.5; }
    .shape { fill: #ffffff; stroke: #1f2328; stroke-width: 2; }
    .end { fill: #ffffff; stroke: #1f2328; stroke-width: 4; }
    .head { fill: #1f2328; }
    text { fill: #1f2328; font-family: system-ui, sans-serif; font-size: 12px; }
    .name { font-weight: 600; }
    .small { font-size: 11px; fill: #57606a; }
    .draft { fill: #9a6700; font-weight: 600; }
    .lane { fill: none; stroke: #d1d9e0; stroke-width: 1; }
    @media (prefers-color-scheme: dark) {
      .line { stroke: #d1d7e0; }
      .shape, .end { fill: #151b23; stroke: #d1d7e0; }
      .head { fill: #d1d7e0; }
      text { fill: #d1d7e0; }
      .small { fill: #9198a1; }
      .draft { fill: #d29922; }
      .lane { stroke: #3d444d; }
    }
  </style>
  <defs>
    <marker id="head" viewBox="0 0 10 10" refX="10" refY="5" markerWidth="8" markerHeight="8" orient="auto-start-reverse">
      <path class="head" d="M0,0 L10,5 L0,10 z"/>
    </marker>
  </defs>
`)
	if draft != "" {
		fmt.Fprintf(&b, "  %s\n  <text class=\"draft\" x=\"16\" y=\"20\">%s</text>\n", comment(draft), esc("Draft: not the approved output"))
	}
	fmt.Fprintf(&b, "  <g transform=\"translate(0,%d)\">\n", top)
	for _, m := range wf.marks[definitionsID] {
		b.WriteString("    " + m + "\n")
	}
	for _, m := range wf.marks[processID] {
		b.WriteString("    " + m + "\n")
	}
	// Every element is a group with the BPMN element's id, so a click maps
	// to the element and, through data-step and data-role, to the entry of
	// the specification (ADR-139).
	b.WriteString("    <g id=\"lanes\">\n")
	for _, l := range wf.lanes {
		role := ""
		if l.role != "" {
			role = fmt.Sprintf(" data-role=\"%s\"", esc(l.role))
		}
		fmt.Fprintf(&b, "      <g id=\"%s\"%s>\n", esc(l.id), role)
		fmt.Fprintf(&b, "        <rect class=\"lane\" x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\"/>\n", laneX, l.y, wf.width-2*laneX, l.h)
		fmt.Fprintf(&b, "        <text class=\"small\" x=\"%d\" y=\"%d\">%s</text>\n", laneX+8, l.y+15, esc(l.name))
		b.WriteString("      </g>\n")
	}
	b.WriteString("    </g>\n")
	for _, f := range wf.flows {
		var pts []string
		for _, p := range f.points {
			pts = append(pts, fmt.Sprintf("%d,%d", p[0], p[1]))
		}
		fmt.Fprintf(&b, "    <g id=\"%s\">\n", f.id)
		fmt.Fprintf(&b, "      <polyline class=\"line\" points=\"%s\" marker-end=\"url(#head)\"/>\n", strings.Join(pts, " "))
		if f.label != "" {
			a, c := f.points[0], f.points[1]
			x, y := a[0]+6, a[1]-6
			if a[0] == c[0] { // a flow that goes down first is labelled beside it
				x, y = a[0]+6, a[1]+16
			}
			if f.corner { // a branch is labelled where it turns
				x, y = c[0]+6, c[1]-6
			}
			fmt.Fprintf(&b, "      <text class=\"small\" x=\"%d\" y=\"%d\">%s</text>\n", x, y, esc(f.label))
		}
		b.WriteString("    </g>\n")
	}
	for _, a := range wf.associations {
		var pts []string
		for _, p := range a.points {
			pts = append(pts, fmt.Sprintf("%d,%d", p[0], p[1]))
		}
		fmt.Fprintf(&b, "    <g id=\"%s\">\n      <polyline class=\"line\" stroke-dasharray=\"4 3\" points=\"%s\"/>\n    </g>\n", esc(a.id), strings.Join(pts, " "))
	}
	for _, s := range wf.shapes {
		for _, m := range wf.shapeMarks(s) {
			b.WriteString("    " + m + "\n")
		}
		step := ""
		if s.step != nil {
			step = fmt.Sprintf(" data-step=\"%s\"", esc(text(s.step["name"])))
		}
		fmt.Fprintf(&b, "    <g id=\"%s\"%s>\n", esc(s.id), step)
		switch s.kind {
		case "startEvent", "endEvent":
			class := "shape"
			if s.kind == "endEvent" {
				class = "end"
			}
			fmt.Fprintf(&b, "      <circle class=\"%s\" cx=\"%d\" cy=\"%d\" r=\"%d\"/>\n", class, s.cx(), s.cy(), s.w/2)
			fmt.Fprintf(&b, "      <text x=\"%d\" y=\"%d\" text-anchor=\"middle\">%s</text>\n", s.cx(), s.bottom()+16, esc(s.name))
		case "userTask", "serviceTask", "receiveTask":
			kind := "approval"
			switch {
			case s.forCompensation:
				kind = "undo"
			case s.kind == "serviceTask":
				kind = "operation"
			case s.kind == "receiveTask":
				kind = "wait"
			case text(s.step["kind"]) == "task":
				kind = "task"
			}
			fmt.Fprintf(&b, "      <rect class=\"shape\" x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" rx=\"10\"/>\n", s.x, s.y, s.w, s.h)
			fmt.Fprintf(&b, "      <text class=\"small\" x=\"%d\" y=\"%d\">%s</text>\n", s.x+8, s.y+16, kind)
			fmt.Fprintf(&b, "      <text class=\"name\" x=\"%d\" y=\"%d\" text-anchor=\"middle\">%s</text>\n", s.cx(), s.cy()+2, esc(s.name))
			if s.sub != "" {
				fmt.Fprintf(&b, "      <text class=\"small\" x=\"%d\" y=\"%d\" text-anchor=\"middle\">%s</text>\n", s.cx(), s.cy()+20, esc(s.sub))
			}
		case "boundaryEvent", "intermediateCatchEvent":
			fmt.Fprintf(&b, "      <circle class=\"shape\" cx=\"%d\" cy=\"%d\" r=\"%d\"/>\n", s.cx(), s.cy(), s.w/2)
			fmt.Fprintf(&b, "      <circle class=\"line\" cx=\"%d\" cy=\"%d\" r=\"%d\"/>\n", s.cx(), s.cy(), s.w/2-4)
			switch s.event {
			case "error":
				fmt.Fprintf(&b, "      <polyline class=\"line\" points=\"%d,%d %d,%d %d,%d %d,%d\"/>\n", s.cx()-7, s.cy()+7, s.cx()-2, s.cy()-6, s.cx()+2, s.cy()+2, s.cx()+7, s.cy()-7)
			case "compensate":
				fmt.Fprintf(&b, "      <path class=\"head\" d=\"M%d,%d L%d,%d L%d,%d z M%d,%d L%d,%d L%d,%d z\"/>\n",
					s.cx()-8, s.cy(), s.cx(), s.cy()-5, s.cx(), s.cy()+5, s.cx(), s.cy(), s.cx()+8, s.cy()-5, s.cx()+8, s.cy()+5)
			default:
				fmt.Fprintf(&b, "      <polyline class=\"line\" points=\"%d,%d %d,%d %d,%d\"/>\n", s.cx(), s.cy()-9, s.cx(), s.cy(), s.cx()+7, s.cy())
			}
			if s.kind == "intermediateCatchEvent" {
				fmt.Fprintf(&b, "      <text x=\"%d\" y=\"%d\" text-anchor=\"middle\">%s</text>\n", s.cx(), s.bottom()+16, esc(s.name))
				fmt.Fprintf(&b, "      <text class=\"small\" x=\"%d\" y=\"%d\" text-anchor=\"middle\">%s</text>\n", s.cx(), s.bottom()+30, esc(s.sub))
			}
		case "exclusiveGateway":
			cx, cy, r := s.cx(), s.cy(), s.w/2
			fmt.Fprintf(&b, "      <polygon class=\"shape\" points=\"%d,%d %d,%d %d,%d %d,%d\"/>\n", cx, cy-r, cx+r, cy, cx, cy+r, cx-r, cy)
			fmt.Fprintf(&b, "      <path class=\"line\" d=\"M%d,%d L%d,%d M%d,%d L%d,%d\"/>\n", cx-8, cy-8, cx+8, cy+8, cx+8, cy-8, cx-8, cy+8)
			if text(s.step["kind"]) == "decision" { // above on the left, clear of the branches
				fmt.Fprintf(&b, "      <text x=\"%d\" y=\"%d\" text-anchor=\"end\">%s</text>\n", cx-r/2, s.y-4, esc(s.name))
			}
		}
		b.WriteString("    </g>\n")
	}
	b.WriteString("  </g>\n</svg>\n")
	return b.String()
}

// messageID is the id of the message element a wait names: the channel
// message with its slash, which an XML id may not hold, written as a dot.
func messageID(m string) string {
	return "event-" + strings.ReplaceAll(m, "/", ".")
}

// ownersKey is the key of a user task's potential owners: a task's
// performers, an approval's approvers.
func ownersKey(step map[string]any) string {
	if text(step["kind"]) == "task" {
		return "performers"
	}
	return "approvers"
}

// nameAttr is a flow's name attribute, so a modeller shows its label too.
func nameAttr(label string) string {
	if label == "" {
		return ""
	}
	return fmt.Sprintf(" name=\"%s\"", esc(label))
}

// shapeMarks are the marks the SVG writes before a shape: its own, and
// those of what it shows of the interface and the resources, which the
// picture draws no shape for: the trigger's operation at the start event,
// a step's operation and approvers at its task.
func (wf *workflow) shapeMarks(s *shape) []string {
	ids := []string{s.id}
	switch s.kind {
	case "startEvent":
		ids = append(ids, "operation-"+wf.trigger)
	case "serviceTask":
		ids = append(ids, "operation-"+s.sub)
	case "userTask":
		for _, r := range list(s.step[ownersKey(s.step)]) {
			ids = append(ids, "role-"+text(r))
		}
	}
	var out []string
	for _, id := range ids {
		for _, m := range wf.marks[id] {
			if !slices.Contains(out, m) {
				out = append(out, m)
			}
		}
	}
	return out
}
