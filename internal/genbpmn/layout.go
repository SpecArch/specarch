package genbpmn

import (
	"fmt"
	"strings"
)

// shape is one element of a process with its place in the diagram.
type shape struct {
	id   string
	kind string // the BPMN element: startEvent, userTask, serviceTask, boundaryEvent, exclusiveGateway or endEvent
	name string
	sub  string // the second line a task shows: its approvers, or the operation it calls
	ptr  string // the pointer of the element of the specification it shows
	step map[string]any
	// attached is the task a boundary event sits on.
	attached string
	// The shape's bounds, in the diagram's pixels.
	x, y, w, h int
}

func (s *shape) cx() int     { return s.x + s.w/2 }
func (s *shape) cy() int     { return s.y + s.h/2 }
func (s *shape) right() int  { return s.x + s.w }
func (s *shape) bottom() int { return s.y + s.h }

// flow is one sequence flow with its waypoints.
type flow struct {
	id, source, target string
	label              string // what the SVG writes beside it, or ""
	condition          string // the condition a gateway's flow carries, or ""
	points             [][2]int
}

// workflow is one workflow laid out as a process.
type workflow struct {
	name, description, title string
	ptr                      string
	trigger                  string
	shapes                   []*shape
	flows                    []*flow
	defaults                 map[string]string // gateway id -> its default flow
	operations               []string          // the operations the process names, in order of first use
	opPointers               map[string]string // operationId -> the operation's pointer
	roles                    []string          // the roles that approve, in order of first use
	marks                    map[string][]string
	width, height            int
}

// The diagram's measures, in pixels: the row the steps stand on, the
// shapes' sizes and the gap between them.
const (
	margin    = 40
	eventSize = 36
	taskH     = 80
	gateSize  = 50
	gap       = 60
	charW     = 7 // the width of a character at the SVG's font size, rounded up
)

// layout lays a workflow out: the steps left to right on one row, each
// approval followed by the gateway that ends the request when it is
// refused, a deadline's timer on the bottom edge of its task, and one end
// event for every refusal below the row. Every coordinate follows from the
// steps alone, so the same workflow is always drawn the same.
func layout(spec map[string]any, name string, w map[string]any) *workflow {
	wf := &workflow{name: name, description: text(w["description"]), title: text(obj(spec["info"])["title"]),
		ptr: "/workflows/" + escape(name), trigger: text(w["trigger"]), defaults: map[string]string{},
		opPointers: operationPointers(spec), marks: map[string][]string{}}
	steps := list(w["steps"])
	approvals := 0
	escalations := 0
	for _, s := range steps {
		if text(obj(s)["kind"]) == "approval" {
			approvals++
			if text(obj(s)["onDeadline"]) == "escalate" {
				escalations++
			}
		}
	}
	top := 40
	row := top + taskH/2 // the centre line of the row
	useOp := func(op string) {
		if op == "" {
			return
		}
		for _, o := range wf.operations {
			if o == op {
				return
			}
		}
		wf.operations = append(wf.operations, op)
	}
	useOp(wf.trigger)

	start := &shape{id: "start", kind: "startEvent", name: wf.trigger, ptr: wf.ptr + "/trigger", w: eventSize, h: eventSize}
	start.x = max(margin, len(start.name)*charW/2-eventSize/2+8)
	start.y = row - eventSize/2
	wf.shapes = append(wf.shapes, start)
	x := start.right() + gap
	prev := start
	var pending []*shape // gateways whose approved flow goes to the next shape
	byStep := map[string]*shape{}
	type timer struct {
		t    *shape
		step map[string]any
	}
	var timers []timer
	next := func(s *shape) {
		if prev != nil {
			wf.connect(prev, s, "", "")
		}
		for _, g := range pending {
			wf.connect(g, s, "approved", "approved")
		}
		pending = nil
		prev = s
	}
	for i, raw := range steps {
		st := obj(raw)
		stepName := text(st["name"])
		ptr := fmt.Sprintf("%s/steps/%d", wf.ptr, i)
		task := &shape{id: "step-" + stepName, name: stepName, ptr: ptr, step: st, h: taskH}
		if text(st["kind"]) == "approval" {
			task.kind = "userTask"
			var roles []string
			for _, r := range list(st["approvers"]) {
				role := text(r)
				roles = append(roles, role)
				known := false
				for _, k := range wf.roles {
					known = known || k == role
				}
				if !known {
					wf.roles = append(wf.roles, role)
				}
			}
			task.sub = strings.Join(roles, ", ")
		} else {
			task.kind = "serviceTask"
			task.sub = text(st["operation"])
			useOp(task.sub)
		}
		task.w = max(110, max(len(task.name), len(task.sub))*charW+20)
		task.x, task.y = x, row-taskH/2
		wf.shapes = append(wf.shapes, task)
		byStep[stepName] = task
		next(task)
		x = task.right() + gap
		if task.kind != "userTask" {
			continue
		}
		t := &shape{id: "deadline-" + stepName, kind: "boundaryEvent", name: text(st["deadline"]), ptr: ptr + "/deadline",
			attached: task.id, step: st, w: eventSize, h: eventSize}
		t.x, t.y = task.right()-eventSize-12, task.bottom()-eventSize/2
		wf.shapes = append(wf.shapes, t)
		timers = append(timers, timer{t, st})
		g := &shape{id: "decision-" + stepName, kind: "exclusiveGateway", name: "approved?", ptr: ptr, w: gateSize, h: gateSize}
		g.x, g.y = x, row-gateSize/2
		wf.shapes = append(wf.shapes, g)
		next(g)
		prev = nil
		pending = []*shape{g}
		x = g.right() + gap
	}
	done := &shape{id: "end-done", kind: "endEvent", name: "done", ptr: wf.ptr, w: eventSize, h: eventSize}
	done.x, done.y = x, row-eventSize/2
	wf.shapes = append(wf.shapes, done)
	next(done)

	lane := row + taskH/2 + eventSize/2 + 22 // the first escalation's line
	refusedY := lane + 16*escalations + 30
	wf.width = done.right() + margin + 20
	wf.height = row + taskH/2 + 60
	if approvals == 0 {
		return wf
	}
	refused := &shape{id: "end-refused", kind: "endEvent", name: "refused", ptr: wf.ptr, w: eventSize, h: eventSize}
	refused.x, refused.y = done.x, refusedY-eventSize/2
	wf.shapes = append(wf.shapes, refused)
	wf.height = refused.bottom() + 40
	// Each gateway's refusal goes down to the line of the refused end, and
	// is the gateway's default flow.
	for _, s := range wf.shapes {
		if s.kind == "exclusiveGateway" {
			f := wf.connect(s, refused, "refused", "")
			f.points = [][2]int{{s.cx(), s.bottom()}, {s.cx(), refusedY}, {refused.x, refusedY}}
			wf.defaults[s.id] = f.id
		}
	}
	k := 0
	for _, tm := range timers {
		t := tm.t
		if text(tm.step["onDeadline"]) == "escalate" {
			if target := byStep[text(tm.step["escalateTo"])]; target != nil {
				y := lane + 16*k
				k++
				f := wf.connect(t, target, "after "+t.name, "")
				f.points = [][2]int{{t.cx(), t.bottom()}, {t.cx(), y}, {target.x + 24, y}, {target.x + 24, target.bottom()}}
				continue
			}
		}
		f := wf.connect(t, refused, "after "+t.name, "")
		f.points = [][2]int{{t.cx(), t.bottom()}, {t.cx(), refusedY}, {refused.x, refusedY}}
	}
	return wf
}

// connect adds a flow on the row from one shape to the next.
func (wf *workflow) connect(from, to *shape, label, condition string) *flow {
	f := &flow{id: fmt.Sprintf("flow-%d", len(wf.flows)+1), source: from.id, target: to.id, label: label, condition: condition,
		points: [][2]int{{from.right(), from.cy()}, {to.x, to.cy()}}}
	wf.flows = append(wf.flows, f)
	return f
}

// operationPointers maps each operationId to its operation's pointer.
func operationPointers(spec map[string]any) map[string]string {
	out := map[string]string{}
	paths := obj(spec["paths"])
	for _, p := range sortedKeys(paths) {
		item := obj(paths[p])
		for _, m := range sortedKeys(item) {
			if id := text(obj(item[m])["operationId"]); id != "" {
				out[id] = "/paths/" + escape(p) + "/" + m
			}
		}
	}
	return out
}

// escape writes a key as a JSON pointer token (RFC 6901).
func escape(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}
