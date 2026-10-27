package genbpmn

import (
	"fmt"
	"strings"
)

// shape is one element of a process with its place in the diagram.
type shape struct {
	id   string
	kind string // the BPMN element: startEvent, userTask, serviceTask, receiveTask, intermediateCatchEvent, boundaryEvent, exclusiveGateway or endEvent
	// event is what a boundary or catch event waits for: timer, error or
	// compensate; for a catch event, timeDuration or timeDate.
	event string
	// forCompensation marks the service task that undoes a step.
	forCompensation bool
	name            string
	sub             string // the second line a task shows: its approvers, or the operation it calls
	ptr             string // the pointer of the element of the specification it shows
	step            map[string]any
	// attached is the task a boundary event sits on.
	attached string
	// lane is the index of the lane the shape is in: 0 the system's, then
	// one per role in the order of its first approval.
	lane int
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
	language           string // the language of the condition, or "" for none named
	corner             bool   // the label is written at the flow's first bend, as a branch's is
	points             [][2]int
}

// lane is one lane of the process's lane set with its band in the
// diagram.
type lane struct {
	id, name, role string // role is "" for the system's lane
	y, h           int
	lines          int // the timer lines drawn along its bottom so far
}

// workflow is one workflow laid out as a process.
type workflow struct {
	name, description, title string
	ptr                      string
	trigger                  string
	shapes                   []*shape
	flows                    []*flow
	associations             []*flow           // from a compensation event to the task that undoes its step
	defaults                 map[string]string // gateway id -> its default flow
	operations               []string          // the operations the process names, in order of first use
	opPointers               map[string]string // operationId -> the operation's pointer
	roles                    []string          // the roles that approve or do a task, in order of first use
	messages                 []string          // the channel messages the waits name, in order of first use
	extended                 bool              // whether a step goes beyond approvals and operation steps (ADR-198)
	compensated              bool              // whether a step names the operation that undoes it
	lanes                    []*lane           // the system's lane, then one per role that approves first on a step
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
	laneX     = 10
	laneTop   = 22 // the room above a lane's shapes for its name
	lineGap   = 16 // between two timer lines along a lane's bottom
)

// expressionLanguage names the language of the conditions and moments a
// workflow's file carries: SpecArch's subset of CEL over the subject's
// fields, so a reader does not take them for XPath, BPMN's default.
const expressionLanguage = "urn:specarch:expression"

// layout lays a workflow out (ADR-085, ADR-139, ADR-198): a lane for the
// system holding the start event, the operation steps, the decisions, the
// waits, the gateways and the end events, and a lane below it per role
// that approves or does a task, in the order of the roles' first step,
// each holding the steps that role is listed first on with their
// deadlines. The steps go left to right, each approval followed by the
// gateway that ends the request when it is refused; a deadline's timer
// and a failure's error event sit on the bottom edge of their task, and
// their flows run along the bottom of the task's lane; a compensation
// event sits on the top edge, tied to the task that undoes the step in a
// row above; a branch that passes over steps runs along a line above the
// row. Every coordinate follows from the steps alone, so the same
// workflow is always drawn the same.
func layout(spec map[string]any, name string, w map[string]any) *workflow {
	wf := &workflow{name: name, description: text(w["description"]), title: text(obj(spec["info"])["title"]),
		ptr: "/workflows/" + escape(name), trigger: text(w["trigger"]), defaults: map[string]string{},
		opPointers: operationPointers(spec), marks: map[string][]string{}}
	steps := list(w["steps"])
	approvals := 0
	laneOf := map[string]int{} // a role -> its lane
	wf.lanes = []*lane{{id: "lane", name: wf.title}}
	// What the system's lane needs room for beyond the sequential subset:
	// the row of compensating tasks, the lines above the row a branch
	// passes over steps along, the refused and failed ends, and a line per
	// timer or error event on a step in it.
	refusedUsed, failedUsed, approvedSkips, skips, sysLines := false, false, 0, 0, 0
	names := map[string]int{}
	for i, s := range steps {
		names[text(obj(s)["name"])] = i
	}
	for i, s := range steps {
		st := obj(s)
		kind := text(st["kind"])
		if kind != "approval" && kind != "operation" || st["onFailure"] != nil || st["compensatedBy"] != nil {
			wf.extended = true
		}
		if st["compensatedBy"] != nil {
			wf.compensated = true
		}
		switch kind {
		case "approval", "task":
			if kind == "approval" {
				approvals++
				refusedUsed = true
			}
			if kind == "task" && st["deadline"] != nil && text(st["onDeadline"]) != "escalate" {
				refusedUsed = true
			}
			role := ""
			if first := list(st["approvers"]); kind == "approval" && len(first) > 0 {
				role = text(first[0])
			}
			if first := list(st["performers"]); kind == "task" && len(first) > 0 {
				role = text(first[0])
			}
			if _, ok := laneOf[role]; role != "" && !ok {
				laneOf[role] = len(wf.lanes)
				wf.lanes = append(wf.lanes, &lane{id: "lane-" + role, name: role, role: role})
			}
		case "decision":
			for _, b := range list(st["branches"]) {
				br := obj(b)
				switch end := text(br["end"]); {
				case end == "refused":
					refusedUsed = true
				case end == "failed":
					failedUsed = true
				case end == "approved" && i+1 < len(steps):
					approvedSkips++
				case end == "" && names[text(br["to"])] != i+1:
					skips++
				}
			}
		case "wait":
			if st["deadline"] != nil && st["message"] != nil {
				refusedUsed = true
				sysLines++
			}
		case "operation":
			if f := obj(st["onFailure"]); f != nil {
				sysLines++
				if text(f["to"]) == "" {
					failedUsed = true
				}
			}
		}
	}
	skips += approvedSkips
	compBand, skipBand := 0, 0
	if wf.compensated {
		compBand = taskH + 30
	}
	if skips > 0 {
		skipBand = lineGap*skips + 10
	}
	// The bands: the system's lane holds the row and, below it, the line
	// the refusals run along; a role's lane holds a row of tasks and the
	// timer lines under them, one per approval in it.
	sys := wf.lanes[0]
	sys.y = laneX
	row := sys.y + laneTop + compBand + skipBand + taskH/2 // the centre line of the system's row
	refusedY := row + taskH/2 + 40
	failedY := refusedY
	if refusedUsed {
		failedY = refusedY + eventSize + 30 // below the refused end and its name
	}
	sysLineY := failedY // the first line of the system's timers and failures
	if failedUsed {
		sysLineY = failedY + eventSize/2 + 28
	} else if refusedUsed {
		sysLineY = refusedY + eventSize/2 + 28
	}
	sys.h = row + taskH/2 + 30 - sys.y
	if refusedUsed || failedUsed {
		sys.h = sysLineY - sys.y
	}
	if sysLines > 0 {
		sys.h = sysLineY + lineGap*sysLines + 10 - sys.y
	}
	inLane := map[int]int{}
	for _, s := range steps {
		st := obj(s)
		if first := list(st["approvers"]); text(st["kind"]) == "approval" && len(first) > 0 {
			inLane[laneOf[text(first[0])]]++
		}
		if first := list(st["performers"]); text(st["kind"]) == "task" && len(first) > 0 && st["deadline"] != nil {
			inLane[laneOf[text(first[0])]]++
		}
	}
	centre := map[int]int{0: row}
	for i := 1; i < len(wf.lanes); i++ {
		l := wf.lanes[i]
		l.y = wf.lanes[i-1].y + wf.lanes[i-1].h
		centre[i] = l.y + laneTop + taskH/2
		l.h = laneTop + taskH + eventSize/2 + 14 + lineGap*inLane[i] + 10
	}
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
	useRole := func(role string) {
		for _, k := range wf.roles {
			if k == role {
				return
			}
		}
		wf.roles = append(wf.roles, role)
	}
	useOp(wf.trigger)

	start := &shape{id: "start", kind: "startEvent", name: wf.trigger, ptr: wf.ptr + "/trigger", w: eventSize, h: eventSize}
	start.x = max(margin, len(start.name)*charW/2-eventSize/2+8)
	start.y = row - eventSize/2
	wf.shapes = append(wf.shapes, start)
	x := start.right() + gap
	prev := start
	var pending []*shape // gateways whose approved flow goes to the next shape
	// jumps are the flows that wait for a later step's shape: a branch of
	// a decision, by the step it goes on at.
	type jump struct {
		from              *shape
		label, condition  string
		otherwise, around bool // around: the flow passes over steps, along a line above the row
	}
	jumps := map[string][]jump{}
	skipLine := 0 // the lines above the row taken so far
	byStep := map[string]*shape{}
	type timer struct {
		t    *shape
		step map[string]any
	}
	var timers []timer
	var failures []timer
	var toDone []jump // branches that end the request approved
	var toEnd []struct {
		from       *shape
		end, label string
		condition  string
		otherwise  bool
	}
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
	arrive := func(stepName string, s *shape) {
		for _, j := range jumps[stepName] {
			f := wf.connect(j.from, s, j.label, j.condition)
			f.language = expressionLanguage
			if j.condition == "" {
				f.language = ""
			}
			if j.around {
				y := row - taskH/2 - 10 - lineGap*skipLine
				skipLine++
				end := s.y
				f.points = [][2]int{{j.from.cx(), j.from.y}, {j.from.cx(), y}, {s.cx(), y}, {s.cx(), end}}
				f.corner = true
			}
			if j.otherwise {
				wf.defaults[j.from.id] = f.id
			}
		}
		delete(jumps, stepName)
	}
	for i, raw := range steps {
		st := obj(raw)
		stepName := text(st["name"])
		kind := text(st["kind"])
		ptr := fmt.Sprintf("%s/steps/%d", wf.ptr, i)
		task := &shape{id: "step-" + stepName, name: stepName, ptr: ptr, step: st, h: taskH}
		switch kind {
		case "approval", "task":
			task.kind = "userTask"
			key := "approvers"
			if kind == "task" {
				key = "performers"
			}
			var roles []string
			for _, r := range list(st[key]) {
				role := text(r)
				roles = append(roles, role)
				useRole(role)
			}
			task.sub = strings.Join(roles, ", ")
			if len(roles) > 0 {
				task.lane = laneOf[roles[0]]
			}
		case "decision":
			task.kind, task.w, task.h = "exclusiveGateway", gateSize, gateSize
		case "wait":
			switch {
			case st["message"] != nil:
				task.kind, task.sub = "receiveTask", text(st["message"])
				found := false
				for _, m := range wf.messages {
					found = found || m == task.sub
				}
				if !found {
					wf.messages = append(wf.messages, task.sub)
				}
			case st["at"] != nil:
				task.kind, task.event, task.sub, task.w, task.h = "intermediateCatchEvent", "timeDate", text(st["at"]), eventSize, eventSize
			default:
				task.kind, task.event, task.sub, task.w, task.h = "intermediateCatchEvent", "timeDuration", text(st["after"]), eventSize, eventSize
			}
		default:
			task.kind = "serviceTask"
			task.sub = text(st["operation"])
			useOp(task.sub)
		}
		if task.w == 0 {
			task.w = max(110, max(len(task.name), len(task.sub))*charW+20)
		}
		task.x, task.y = x, centre[task.lane]-task.h/2
		wf.shapes = append(wf.shapes, task)
		byStep[stepName] = task
		next(task)
		arrive(stepName, task)
		x = task.right() + gap
		switch {
		case kind == "decision":
			prev = nil
			x = task.right() + gap
			for _, b := range list(st["branches"]) {
				br := obj(b)
				label, cond, otherwise := text(br["when"]), text(br["when"]), br["otherwise"] != nil
				if otherwise {
					label = "otherwise"
				}
				if end := text(br["end"]); end != "" {
					if end == "approved" {
						toDone = append(toDone, jump{from: task, label: label, condition: cond, otherwise: otherwise, around: i+1 < len(steps)})
						continue
					}
					toEnd = append(toEnd, struct {
						from       *shape
						end, label string
						condition  string
						otherwise  bool
					}{task, end, label, cond, otherwise})
					continue
				}
				to := text(br["to"])
				jumps[to] = append(jumps[to], jump{from: task, label: label, condition: cond, otherwise: otherwise, around: names[to] != i+1})
			}
			continue
		case kind == "wait" && st["message"] != nil && st["deadline"] != nil, kind == "task" && st["deadline"] != nil, kind == "approval":
			t := &shape{id: "deadline-" + stepName, kind: "boundaryEvent", event: "timer", name: text(st["deadline"]), ptr: ptr + "/deadline",
				attached: task.id, step: st, lane: task.lane, w: eventSize, h: eventSize}
			t.x, t.y = task.right()-eventSize-12, task.bottom()-eventSize/2
			wf.shapes = append(wf.shapes, t)
			timers = append(timers, timer{t, st})
		case kind == "operation":
			if comp := text(st["compensatedBy"]); comp != "" {
				c := &shape{id: "compensation-" + stepName, kind: "boundaryEvent", event: "compensate", name: "undo", ptr: ptr + "/compensatedBy",
					attached: task.id, step: st, w: eventSize, h: eventSize}
				c.x, c.y = task.right()-eventSize-12, task.y-eventSize/2
				undo := &shape{id: "undo-" + stepName, kind: "serviceTask", forCompensation: true, name: "undo " + stepName, sub: comp,
					ptr: ptr + "/compensatedBy", step: st, h: taskH}
				undo.w = task.w
				undo.x, undo.y = task.x, sys.y+laneTop
				useOp(comp)
				wf.shapes = append(wf.shapes, c, undo)
				a := &flow{id: "association-" + stepName, source: c.id, target: undo.id,
					points: [][2]int{{c.cx(), c.y}, {c.cx(), undo.bottom()}}}
				wf.associations = append(wf.associations, a)
			}
			if st["onFailure"] != nil {
				e := &shape{id: "failure-" + stepName, kind: "boundaryEvent", event: "error", name: "fails", ptr: ptr + "/onFailure",
					attached: task.id, step: st, w: eventSize, h: eventSize}
				e.x, e.y = task.x+12, task.bottom()-eventSize/2
				wf.shapes = append(wf.shapes, e)
				failures = append(failures, timer{e, st})
			}
		}
		if kind != "approval" {
			continue
		}
		g := &shape{id: "decision-" + stepName, kind: "exclusiveGateway", name: "approved?", ptr: ptr, step: st, w: gateSize, h: gateSize}
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
	for _, j := range toDone {
		f := wf.connect(j.from, done, j.label, j.condition)
		if j.condition != "" {
			f.language = expressionLanguage
		}
		if j.around {
			y := row - taskH/2 - 10 - lineGap*skipLine
			skipLine++
			f.points = [][2]int{{j.from.cx(), j.from.y}, {j.from.cx(), y}, {done.cx(), y}, {done.cx(), done.y}}
			f.corner = true
		}
		if j.otherwise {
			wf.defaults[j.from.id] = f.id
		}
	}

	wf.width = done.right() + margin + 20
	last := wf.lanes[len(wf.lanes)-1]
	wf.height = last.y + last.h + 10
	if !refusedUsed && !failedUsed {
		return wf
	}
	ends := map[string]*shape{}
	if refusedUsed {
		refused := &shape{id: "end-refused", kind: "endEvent", name: "refused", ptr: wf.ptr, w: eventSize, h: eventSize}
		refused.x, refused.y = done.x, refusedY-eventSize/2
		wf.shapes = append(wf.shapes, refused)
		ends["refused"] = refused
	}
	if failedUsed {
		failed := &shape{id: "end-failed", kind: "endEvent", name: "failed", ptr: wf.ptr, w: eventSize, h: eventSize}
		failed.x, failed.y = done.x, failedY-eventSize/2
		wf.shapes = append(wf.shapes, failed)
		ends["failed"] = failed
	}
	// Each approval gateway's refusal goes down to the line of the refused
	// end, and is the gateway's default flow.
	for _, s := range wf.shapes {
		if s.kind == "exclusiveGateway" && s.step != nil && text(s.step["kind"]) == "approval" {
			refused := ends["refused"]
			f := wf.connect(s, refused, "refused", "")
			f.points = [][2]int{{s.cx(), s.bottom()}, {s.cx(), refusedY}, {refused.x, refusedY}}
			wf.defaults[s.id] = f.id
		}
	}
	// A decision's branch that ends the request goes down to the line of
	// its end.
	for _, e := range toEnd {
		end := ends[e.end]
		f := wf.connect(e.from, end, e.label, e.condition)
		if e.condition != "" {
			f.language = expressionLanguage
		}
		f.points = [][2]int{{e.from.cx(), e.from.bottom()}, {e.from.cx(), end.cy()}, {end.x, end.cy()}}
		f.corner = true
		if e.otherwise {
			wf.defaults[e.from.id] = f.id
		}
	}
	// A timer's flow goes down to a line along the bottom of its lane, one
	// line per timer, and along it to the task it escalates to, or up the
	// right of the refused end and into it.
	sysLine := 0
	lineOf := func(t *shape) int {
		if t.lane == 0 && text(t.step["kind"]) != "approval" {
			y := sysLineY + lineGap*sysLine
			sysLine++
			return y
		}
		l := wf.lanes[t.lane]
		y := centre[t.lane] + taskH/2 + eventSize/2 + 14 + lineGap*l.lines
		l.lines++
		return y
	}
	toward := func(t, target *shape, label string, y int) {
		end := target.bottom()
		if target.lane > t.lane {
			end = target.y
		}
		f := wf.connect(t, target, label, "")
		f.points = [][2]int{{t.cx(), t.bottom()}, {t.cx(), y}, {target.x + 24, y}, {target.x + 24, end}}
	}
	intoEnd := func(t, end *shape, label string, y int) {
		f := wf.connect(t, end, label, "")
		side := end.right() + 20 // up the right of the end, clear of its name
		f.points = [][2]int{{t.cx(), t.bottom()}, {t.cx(), y}, {side, y}, {side, end.cy()}, {end.right(), end.cy()}}
	}
	for _, tm := range timers {
		t := tm.t
		y := lineOf(t)
		if text(tm.step["onDeadline"]) == "escalate" {
			if target := byStep[text(tm.step["escalateTo"])]; target != nil {
				toward(t, target, "after "+t.name, y)
				continue
			}
		}
		intoEnd(t, ends["refused"], "after "+t.name, y)
	}
	for _, fl := range failures {
		e := fl.t
		y := lineOf(e)
		if target := byStep[text(obj(fl.step["onFailure"])["to"])]; target != nil {
			toward(e, target, "fails", y)
			continue
		}
		intoEnd(e, ends["failed"], "fails", y)
	}
	return wf
}

// connect adds a flow from one shape to the next: straight along a row,
// or across to the next shape's lane halfway between the two.
func (wf *workflow) connect(from, to *shape, label, condition string) *flow {
	f := &flow{id: fmt.Sprintf("flow-%d", len(wf.flows)+1), source: from.id, target: to.id, label: label, condition: condition,
		points: [][2]int{{from.right(), from.cy()}, {to.x, to.cy()}}}
	if from.cy() != to.cy() {
		mid := (from.right() + to.x) / 2
		f.points = [][2]int{{from.right(), from.cy()}, {mid, from.cy()}, {mid, to.cy()}, {to.x, to.cy()}}
	}
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
