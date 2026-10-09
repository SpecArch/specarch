package genbpmn

import (
	"fmt"
	"slices"
	"strings"
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
// holds its pointer: a question at each entry it blocks. A problem outside
// the elements the file shows is in the problems file only.
func marksOf(wf *workflow, problems []Problem) map[string][]string {
	type element struct{ ptr, id string }
	elements := []element{{"/workflows", definitionsID}, {wf.ptr, processID}}
	for _, s := range wf.shapes {
		switch s.kind {
		case "startEvent", "userTask", "serviceTask":
			elements = append(elements, element{s.ptr, s.id})
		case "boundaryEvent":
			step := strings.TrimSuffix(s.ptr, "/deadline")
			for _, key := range []string{"deadline", "onDeadline", "escalateTo"} {
				elements = append(elements, element{step + "/" + key, s.id})
			}
		}
	}
	for _, r := range wf.roles {
		elements = append(elements, element{"/roles/" + escape(r), "role-" + r})
	}
	for _, op := range wf.operations {
		if p := wf.opPointers[op]; p != "" {
			elements = append(elements, element{p, "operation-" + op})
		}
	}
	out := map[string][]string{}
	for _, p := range problems {
		pointers := []string{p.Pointer}
		if p.Severity == "question" {
			pointers = nil
			for _, b := range p.Blocks {
				switch {
				case strings.HasPrefix(b, "#/"):
					pointers = append(pointers, b[1:])
				case strings.HasPrefix(b, "/"):
					pointers = append(pointers, b)
				default:
					pointers = append(pointers, "/"+escape(b))
				}
			}
		}
		line := comment(fmt.Sprintf("specarch-problem: %s: %s: %s [%s]", p.Severity, p.Rule, strings.Join(strings.Fields(p.Message), " "), p.ID))
		for _, ptr := range pointers {
			best := element{}
			for _, e := range elements {
				// The file as a whole holds a problem of the whole section,
				// and not one of another workflow.
				under := strings.HasPrefix(ptr, e.ptr+"/") && e.id != definitionsID
				if (ptr == e.ptr || under) && len(e.ptr) > len(best.ptr) {
					best = e
				}
			}
			if best.id == "" {
				continue
			}
			seen := false
			for _, m := range out[best.id] {
				seen = seen || m == line
			}
			if !seen {
				out[best.id] = append(out[best.id], line)
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
	return fmt.Sprintf("Workflow %s in the sequential subset of BPMN 2.0: its steps, the roles that approve, the deadlines and the operations by name. The subject, the permission each approval checks, the events and the requirements are in the specification only.", wf.name)
}

const draftText = "Draft: no approval record covers the specification's files as they are, so this file is not the approved output."

// bpmn writes the workflow as a BPMN 2.0 XML file.
func (wf *workflow) bpmn(header string, draft bool) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(comment(header) + "\n")
	b.WriteString(comment(wf.partial()) + "\n")
	if draft {
		b.WriteString(comment(draftText) + "\n")
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
	for _, r := range wf.roles {
		mark("role-"+r, "  ")
		fmt.Fprintf(&b, "  <resource id=\"role-%s\" name=\"%s\"/>\n", esc(r), esc(r))
	}
	mark(processID, "  ")
	fmt.Fprintf(&b, "  <process id=\"%s\" name=\"%s\" isExecutable=\"false\">\n", processID, esc(wf.name))
	if wf.description != "" {
		fmt.Fprintf(&b, "    <documentation>%s</documentation>\n", esc(strings.Join(strings.Fields(wf.description), " ")))
	}
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
			for _, r := range list(s.step["approvers"]) {
				fmt.Fprintf(&b, "      <potentialOwner>\n        <resourceRef>role-%s</resourceRef>\n      </potentialOwner>\n", esc(text(r)))
			}
			b.WriteString("    </userTask>\n")
		case "serviceTask":
			fmt.Fprintf(&b, "    <serviceTask id=\"%s\" name=\"%s\" operationRef=\"operation-%s\"/>\n", esc(s.id), esc(s.name), esc(s.sub))
		case "boundaryEvent":
			fmt.Fprintf(&b, "    <boundaryEvent id=\"%s\" attachedToRef=\"%s\" cancelActivity=\"true\">\n      <timerEventDefinition>\n        <timeDuration xsi:type=\"tFormalExpression\">%s</timeDuration>\n      </timerEventDefinition>\n    </boundaryEvent>\n",
				esc(s.id), esc(s.attached), esc(s.name))
		case "exclusiveGateway":
			fmt.Fprintf(&b, "    <exclusiveGateway id=\"%s\" name=\"%s\" default=\"%s\"/>\n", esc(s.id), esc(s.name), wf.defaults[s.id])
		case "endEvent":
			fmt.Fprintf(&b, "    <endEvent id=\"%s\" name=\"%s\"/>\n", esc(s.id), esc(s.name))
		}
	}
	for _, f := range wf.flows {
		if f.condition != "" {
			fmt.Fprintf(&b, "    <sequenceFlow id=\"%s\"%s sourceRef=\"%s\" targetRef=\"%s\">\n      <conditionExpression xsi:type=\"tFormalExpression\">%s</conditionExpression>\n    </sequenceFlow>\n",
				f.id, nameAttr(f.label), esc(f.source), esc(f.target), esc(f.condition))
			continue
		}
		fmt.Fprintf(&b, "    <sequenceFlow id=\"%s\"%s sourceRef=\"%s\" targetRef=\"%s\"/>\n", f.id, nameAttr(f.label), esc(f.source), esc(f.target))
	}
	b.WriteString("  </process>\n")
	fmt.Fprintf(&b, "  <bpmndi:BPMNDiagram id=\"diagram-%s\">\n    <bpmndi:BPMNPlane id=\"plane-%s\" bpmnElement=\"%s\">\n", esc(wf.name), esc(wf.name), processID)
	for _, s := range wf.shapes {
		marker := ""
		if s.kind == "exclusiveGateway" {
			marker = ` isMarkerVisible="true"`
		}
		fmt.Fprintf(&b, "      <bpmndi:BPMNShape id=\"shape-%s\" bpmnElement=\"%s\"%s>\n        <dc:Bounds x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\"/>\n      </bpmndi:BPMNShape>\n",
			esc(s.id), esc(s.id), marker, s.x, s.y, s.w, s.h)
	}
	for _, f := range wf.flows {
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
func (wf *workflow) svg(header string, draft bool) string {
	top := 0
	if draft {
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
    @media (prefers-color-scheme: dark) {
      .line { stroke: #d1d7e0; }
      .shape, .end { fill: #151b23; stroke: #d1d7e0; }
      .head { fill: #d1d7e0; }
      text { fill: #d1d7e0; }
      .small { fill: #9198a1; }
      .draft { fill: #d29922; }
    }
  </style>
  <defs>
    <marker id="head" viewBox="0 0 10 10" refX="10" refY="5" markerWidth="8" markerHeight="8" orient="auto-start-reverse">
      <path class="head" d="M0,0 L10,5 L0,10 z"/>
    </marker>
  </defs>
`)
	if draft {
		fmt.Fprintf(&b, "  %s\n  <text class=\"draft\" x=\"16\" y=\"20\">%s</text>\n", comment(draftText), esc("Draft: not from an approved specification"))
	}
	fmt.Fprintf(&b, "  <g transform=\"translate(0,%d)\">\n", top)
	for _, m := range wf.marks[definitionsID] {
		b.WriteString("    " + m + "\n")
	}
	for _, m := range wf.marks[processID] {
		b.WriteString("    " + m + "\n")
	}
	for _, f := range wf.flows {
		var pts []string
		for _, p := range f.points {
			pts = append(pts, fmt.Sprintf("%d,%d", p[0], p[1]))
		}
		fmt.Fprintf(&b, "    <polyline class=\"line\" points=\"%s\" marker-end=\"url(#head)\"/>\n", strings.Join(pts, " "))
		if f.label != "" {
			a, c := f.points[0], f.points[1]
			x, y := a[0]+6, a[1]-6
			if a[0] == c[0] { // a flow that goes down first is labelled beside it
				x, y = a[0]+6, a[1]+16
			}
			fmt.Fprintf(&b, "    <text class=\"small\" x=\"%d\" y=\"%d\">%s</text>\n", x, y, esc(f.label))
		}
	}
	for _, s := range wf.shapes {
		for _, m := range wf.shapeMarks(s) {
			b.WriteString("    " + m + "\n")
		}
		switch s.kind {
		case "startEvent", "endEvent":
			class := "shape"
			if s.kind == "endEvent" {
				class = "end"
			}
			fmt.Fprintf(&b, "    <circle class=\"%s\" cx=\"%d\" cy=\"%d\" r=\"%d\"/>\n", class, s.cx(), s.cy(), s.w/2)
			fmt.Fprintf(&b, "    <text x=\"%d\" y=\"%d\" text-anchor=\"middle\">%s</text>\n", s.cx(), s.bottom()+16, esc(s.name))
		case "userTask", "serviceTask":
			kind := "approval"
			if s.kind == "serviceTask" {
				kind = "operation"
			}
			fmt.Fprintf(&b, "    <rect class=\"shape\" x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" rx=\"10\"/>\n", s.x, s.y, s.w, s.h)
			fmt.Fprintf(&b, "    <text class=\"small\" x=\"%d\" y=\"%d\">%s</text>\n", s.x+8, s.y+16, kind)
			fmt.Fprintf(&b, "    <text class=\"name\" x=\"%d\" y=\"%d\" text-anchor=\"middle\">%s</text>\n", s.cx(), s.cy()+2, esc(s.name))
			if s.sub != "" {
				fmt.Fprintf(&b, "    <text class=\"small\" x=\"%d\" y=\"%d\" text-anchor=\"middle\">%s</text>\n", s.cx(), s.cy()+20, esc(s.sub))
			}
		case "boundaryEvent":
			fmt.Fprintf(&b, "    <circle class=\"shape\" cx=\"%d\" cy=\"%d\" r=\"%d\"/>\n", s.cx(), s.cy(), s.w/2)
			fmt.Fprintf(&b, "    <circle class=\"line\" cx=\"%d\" cy=\"%d\" r=\"%d\"/>\n", s.cx(), s.cy(), s.w/2-4)
			fmt.Fprintf(&b, "    <polyline class=\"line\" points=\"%d,%d %d,%d %d,%d\"/>\n", s.cx(), s.cy()-9, s.cx(), s.cy(), s.cx()+7, s.cy())
		case "exclusiveGateway":
			cx, cy, r := s.cx(), s.cy(), s.w/2
			fmt.Fprintf(&b, "    <polygon class=\"shape\" points=\"%d,%d %d,%d %d,%d %d,%d\"/>\n", cx, cy-r, cx+r, cy, cx, cy+r, cx-r, cy)
			fmt.Fprintf(&b, "    <path class=\"line\" d=\"M%d,%d L%d,%d M%d,%d L%d,%d\"/>\n", cx-8, cy-8, cx+8, cy+8, cx+8, cy-8, cx-8, cy+8)
		}
	}
	b.WriteString("  </g>\n</svg>\n")
	return b.String()
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
		for _, r := range list(s.step["approvers"]) {
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
