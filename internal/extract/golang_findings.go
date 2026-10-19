package extract

import (
	"fmt"
	"go/ast"
	"regexp"
	"sort"
	"strings"
)

// The handler's code checked against what the operation declares (ADR-144).
// Each fault is a question on the operation, with its certainty: certain
// when the code as written cannot work or disagrees with the
// specification, suspected when a run or a person must confirm it. The
// code itself is never changed.

// goFault is one fault found in a handler's code, asked once for every
// operation whose handler reaches it: a shared function's fault is one
// question naming every operation, not one question per operation.
type goFault struct {
	certainty, clause, text, why, says string
	blocks                             []string
	handlers                           []*goHandler
}

// ask records a question on the operation about a fault in its handler.
// The text names the handler as %s, filled in when the question is written
// with every operation that reaches the fault.
func (h *goHandler) ask(certainty, clause, text, why, says string, blocks ...string) {
	g := h.g
	key := certainty + "|" + clause + "|" + text
	f := g.faultAt[key]
	if f == nil {
		f = &goFault{certainty: certainty, clause: clause, text: text, why: why, says: says}
		g.faultAt[key] = f
		g.faults = append(g.faults, f)
	}
	f.handlers = append(f.handlers, h)
	if len(blocks) == 0 {
		blocks = []string{h.at}
	}
	for _, b := range blocks {
		if !contains(f.blocks, b) {
			f.blocks = append(f.blocks, b)
		}
	}
}

// askFaults writes every fault as a question, in the order found.
func (g *goReader) askFaults() {
	for _, f := range g.faults {
		g.questionWith("should", f.certainty, f.clause+": "+strings.ReplaceAll(f.text, handlerPlaceholder, handlersText(f.handlers)), f.blocks, f.why, g.at(f.clause, f.says))
	}
}

// handlersText names the handler of one operation, or one handler serving
// several, or each handler of several operations; always one handler, so
// that the verb after it is singular.
func handlersText(hs []*goHandler) string {
	var ops []string
	oneTarget := true
	for _, h := range hs {
		ops = append(ops, h.label)
		oneTarget = oneTarget && h.target == hs[0].target
	}
	if oneTarget {
		return "the handler " + hs[0].target + " of " + joinAnd(ops)
	}
	return "each handler of " + joinAnd(ops)
}

// fault is a fault at a step.
func (h *goHandler) fault(certainty string, s *goStep, text, why, says string, blocks ...string) {
	h.ask(certainty, h.g.clause(s.file, s.syntax.Pos()), text, why, says, blocks...)
}

// handlerText names the handler in a question's text, filled in when the
// question is written.
func (h *goHandler) handlerText() string {
	return handlerPlaceholder
}

// handlerPlaceholder stands for the handler in a fault's text until the
// question is written.
const handlerPlaceholder = "\x00handler\x00"

// followedText says how far the handler was followed.
func (h *goHandler) followedText() string {
	if len(h.functions) == 1 {
		return "its own body, which calls no function of the module"
	}
	return fmt.Sprintf("its body and the %s it calls, followed to %d calls deep", plural(len(h.functions)-1, "function"), handlerDepth)
}

// findings runs every check on the handler.
func (h *goHandler) findings() {
	if len(h.functions) == 0 {
		return
	}
	h.stubs()
	if h.isStub() {
		return
	}
	h.unreadParameters()
	h.unwrittenResponses()
	h.undeclaredStatuses()
	h.pathsWithoutResponse()
	h.writesAfterResponse()
	h.uncheckedWrites()
	h.writesWithoutTransaction()
}

// each calls fn on every step of every function followed.
func (h *goHandler) each(fn func(f *goFunction, s *goStep, parents []*goStep)) {
	for _, f := range h.functions {
		walkSteps(f.steps, nil, func(s *goStep, parents []*goStep) { fn(f, s, parents) })
	}
}

// notFollowedWithRequest says whether a call that was not followed is given
// the request, so that what it reads or answers is not known.
func (h *goHandler) notFollowedWithRequest() *goStep {
	var found *goStep
	h.each(func(_ *goFunction, s *goStep, _ []*goStep) {
		if found == nil && s.kind == "call" && s.stopped != "" && s.givenRequest {
			found = s
		}
	})
	return found
}

// isStub says whether the handler's body does nothing but return.
func (h *goHandler) isStub() bool {
	for _, s := range h.functions[0].steps {
		if s.kind != "return" || s.returnsError {
			return false
		}
	}
	return true
}

var todoWord = regexp.MustCompile(`\b(TODO|FIXME|XXX|HACK)\b`)
var unbuiltWord = regexp.MustCompile(`(?i)not implemented|unimplemented|todo|placeholder|stub`)

// stubs asks about a handler that does nothing, a note that says the code
// is unfinished, a panic that says so, and a response made of fixed
// values.
func (h *goHandler) stubs() {
	g := h.g
	fn := h.functions[0]
	clause := fmt.Sprintf("%s:%d", fn.file.path, fn.first)
	if h.isStub() {
		body := "its body has no step"
		if len(fn.steps) > 0 {
			body += " but a return"
		}
		h.ask("suspected", clause, fmt.Sprintf("%s does nothing: %s. Is it a stub, with the operation not built yet?", h.handlerText(), body),
			"A handler that does nothing answers every request the same way, whatever the specification says the operation does.", "The body holds no step but a return.")
	}
	for _, f := range h.functions {
		for _, cg := range f.file.ast.Comments {
			line := g.fset.Position(cg.Pos()).Line
			if line < f.first || line > f.last {
				continue
			}
			text := strings.TrimSpace(cg.Text())
			if m := todoWord.FindStringIndex(text); m != nil {
				note := strings.Join(strings.Fields(text[m[0]:]), " ")
				if len(note) > 100 {
					note = note[:97] + "..."
				}
				h.ask("certain", fmt.Sprintf("%s:%d", f.file.path, line), fmt.Sprintf("%s carries the note %q in %s, so its code is not finished. What remains to be done, and is the operation built?", h.handlerText(), note, f.name),
					"A note that work remains says the code does not do what the specification says yet.", "Carries the note "+note+".")
			}
		}
	}
	h.each(func(f *goFunction, s *goStep, _ []*goStep) {
		switch {
		case s.kind == "statement" && strings.HasPrefix(s.text, "panics: ") && unbuiltWord.MatchString(s.text):
			h.fault("certain", s, fmt.Sprintf("%s %s in %s, so the operation is not built. When is it built?", h.handlerText(), s.text, f.name),
				"A handler that panics answers no request; the server answers 500 or drops the connection.", strings.ToUpper(s.text[:1])+s.text[1:]+".")
		case s.kind == "response" && s.fixedValue && !f.readsAnything():
			h.fault("suspected", s, fmt.Sprintf("%s %s with fixed values, and %s reads no input, no table and no other system. Is the answer mock data?", h.handlerText(), s.text, f.name),
				"A response of fixed values is the same for every request, which no operation of the specification is.", "Answers fixed values.")
		}
	})
}

// readsAnything says whether a function reads input, a table or another
// system, or calls a function that was followed.
func (f *goFunction) readsAnything() bool {
	reads := false
	walkSteps(f.steps, nil, func(s *goStep, _ []*goStep) {
		switch s.kind {
		case "input", "database", "external":
			reads = true
		case "call":
			if s.function != "" && s.reached != "defined" {
				reads = true
			}
		}
	})
	return reads
}

// unreadParameters asks about a parameter the registration declares and
// the handler never reads.
func (h *goHandler) unreadParameters() {
	ep := h.ep
	if ep == nil || ep.declared == nil {
		return
	}
	read := map[string]bool{}
	h.each(func(_ *goFunction, s *goStep, _ []*goStep) {
		if s.kind == "input" {
			read[s.parameter] = true
		}
	})
	var names []string
	for name := range ep.declared {
		if !read[name] && !contains(ep.params, name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return
	}
	fn := h.functions[0]
	clause := fmt.Sprintf("%s:%d", fn.file.path, fn.first)
	certainty, unless := "certain", ""
	if s := h.notFollowedWithRequest(); s != nil {
		certainty, unless = "suspected", fmt.Sprintf(", unless %s at %s reads them, which was not followed (%s)", s.text, h.g.clause(s.file, s.syntax.Pos()), s.stopped)
	}
	what, them := "the parameter "+names[0], "it"
	if len(names) > 1 {
		what, them = "the parameters "+joinAnd(names), "them"
	}
	h.ask(certainty, clause, fmt.Sprintf("the registration of %s at %s declares %s, and %s reads %s nowhere in %s%s. Are they needed, or does the handler miss reading them?", h.label, h.regClause, what, h.handlerText(), them, h.followedText(), unless),
		"A parameter a handler never reads is accepted and ignored, so a caller's value has no effect, which the specification does not say.", "Reads no parameter named "+joinAnd(names)+".")
}

// writtenStatuses is every status the handler's functions answer, by the
// step that answers it first.
func (h *goHandler) writtenStatuses() map[int]*goStep {
	out := map[int]*goStep{}
	h.each(func(_ *goFunction, s *goStep, _ []*goStep) {
		if s.kind == "response" && s.status != 0 && out[s.status] == nil {
			out[s.status] = s
		}
	})
	return out
}

// unwrittenResponses asks about a response the registration declares and
// neither the handler nor a middleware of its chain answers.
func (h *goHandler) unwrittenResponses() {
	ep := h.ep
	if ep == nil || ep.decl == nil || ep.decl.respWhy != "" || len(ep.decl.responses) == 0 {
		return
	}
	written := h.writtenStatuses()
	byMiddleware := map[int]string{}
	for _, name := range chainNames(ep) {
		if mw := h.g.middlewareOf[name]; mw != nil {
			for _, r := range mw.refusals {
				byMiddleware[r.status] = name
			}
		}
	}
	var codes []string
	for c := range ep.decl.responses {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	fn := h.functions[0]
	clause := fmt.Sprintf("%s:%d", fn.file.path, fn.first)
	for _, c := range codes {
		status := 0
		fmt.Sscan(c, &status)
		if status == 0 || written[status] != nil || byMiddleware[status] != "" {
			continue
		}
		if (status == 400 || status == 422) && len(ep.declared) > 0 {
			continue // dxlib refuses a parameter that fails its declared type itself
		}
		certainty, unless := "certain", ""
		if s := h.notFollowedWithRequest(); s != nil {
			certainty, unless = "suspected", fmt.Sprintf(", unless %s at %s answers it, which was not followed (%s)", s.text, h.g.clause(s.file, s.syntax.Pos()), s.stopped)
		}
		h.ask(certainty, clause, fmt.Sprintf("the registration of %s at %s declares the response %s, and %s answers it nowhere in %s, nor does a middleware of its chain%s. Does the operation answer %s, or is the declaration stale?", h.label, h.regClause, c, h.handlerText(), h.followedText(), unless, c),
			"A response declared and never answered is in the contract clients are built against, and the code cannot produce it.", "Answers no "+c+".", ep.at+"/responses/"+c)
	}
}

// undeclaredStatuses asks about a status the handler answers and the
// registration does not declare.
func (h *goHandler) undeclaredStatuses() {
	ep := h.ep
	if ep == nil || ep.decl == nil || ep.decl.respWhy != "" || len(ep.decl.responses) == 0 {
		return
	}
	written := h.writtenStatuses()
	var statuses []int
	for status := range written {
		if _, ok := ep.decl.responses[fmt.Sprint(status)]; !ok {
			statuses = append(statuses, status)
		}
	}
	sort.Ints(statuses)
	for _, status := range statuses {
		s := written[status]
		h.fault("certain", s, fmt.Sprintf("%s answers %d (%s), which the registration at %s does not declare, so the document dxlib emits does not list it. Should the registration declare it, or is the answer a mistake?", h.handlerText(), status, s.text, h.regClause),
			"A client built on the declared responses does not expect this status, and the two readings of the operation disagree on it.", fmt.Sprintf("Answers %d, which is not declared.", status), ep.at+"/responses")
	}
}

// flowState is what is known of a path through the handler: whether a
// response was written on it, and whether that is unknown because a call
// not followed was given the request.
type flowState struct {
	responded, unknown bool
}

// pathsWithoutResponse asks about a path through the handler that returns,
// or reaches the end, without a response written.
func (h *goHandler) pathsWithoutResponse() {
	fn := h.functions[0]
	asked := map[*goStep]bool{}
	ends := h.flow(fn.steps, []flowState{{}}, asked)
	for _, st := range ends {
		if !st.responded && !st.unknown {
			clause := fmt.Sprintf("%s:%d", fn.file.path, fn.last)
			h.ask("suspected", clause, fmt.Sprintf("%s reaches its end at line %d on a path that writes no response. What does a caller get on that path?", h.handlerText(), fn.last),
				"A path that answers nothing leaves the caller with whatever the framework writes by default, which the specification does not say.", "Reaches its end with no response written.")
			break
		}
	}
}

// flow follows the steps from the states given and returns the states at
// the exits that fall through, asking at each return on a path with no
// response.
func (h *goHandler) flow(steps []*goStep, states []flowState, asked map[*goStep]bool) []flowState {
	for _, s := range steps {
		// What the step itself does to every path through it.
		updated := make([]flowState, len(states))
		for i, st := range states {
			switch s.kind {
			case "response":
				st.responded = true
			case "call":
				// A function literal defined for a later call runs nothing
				// here: what it does counts at the call of the name it is
				// assigned to.
				if s.reached == "defined" {
					break
				}
				switch {
				case s.function != "" && h.functionResponds(s.function):
					st.responded = true
				case s.stopped != "" && s.givenRequest:
					st.unknown = true
				}
			}
			updated[i] = st
		}
		var next []flowState
		switch s.kind {
		case "return":
			for _, st := range updated {
				if !st.responded && !st.unknown && !(h.dxlib && s.returnsError) && !asked[s] {
					asked[s] = true
					h.fault("suspected", s, fmt.Sprintf("%s %s at line %d on a path that writes no response. What does a caller get on that path?", h.handlerText(), s.text, s.first),
						"A path that answers nothing leaves the caller with whatever the framework writes by default, which the specification does not say.", "Returns with no response written.")
					break
				}
			}
			return nil
		case "statement":
			if strings.HasPrefix(s.text, "panics") {
				return nil
			}
			next = updated
		case "branch":
			next = append(next, h.flow(s.then, updated, asked)...)
			if s.hasOtherwise {
				next = append(next, h.flow(s.otherwise, updated, asked)...)
			} else {
				next = append(next, updated...)
			}
		case "choice":
			// A case that falls through carries its states into the next
			// case rather than to the end of the switch.
			hasOtherwise := false
			var carried []flowState
			for _, c := range s.cases {
				hasOtherwise = hasOtherwise || c.when == "otherwise"
				ends := h.flow(c.steps, dedupeStates(append(append([]flowState{}, updated...), carried...)), asked)
				if fallsThrough(c.steps) {
					carried = ends
					continue
				}
				next = append(next, ends...)
				carried = nil
			}
			next = append(next, carried...)
			if !hasOtherwise {
				next = append(next, updated...)
			}
		case "loop":
			next = append(next, h.flow(s.body, updated, asked)...)
			next = append(next, updated...)
		default:
			next = updated
		}
		states = dedupeStates(next)
	}
	return states
}

// fallsThrough says whether a case's steps end in a fallthrough, which Go
// allows only as the last statement of a case.
func fallsThrough(steps []*goStep) bool {
	if len(steps) == 0 {
		return false
	}
	last := steps[len(steps)-1]
	return last.kind == "statement" && last.text == "falls through to the next case"
}

func dedupeStates(states []flowState) []flowState {
	var out []flowState
	seen := map[flowState]bool{}
	for _, st := range states {
		if !seen[st] {
			seen[st] = true
			out = append(out, st)
		}
	}
	return out
}

// functionResponds says whether a function followed writes a response
// anywhere, itself or through a function it calls.
func (h *goHandler) functionResponds(name string) bool {
	seen := map[string]bool{}
	var responds func(name string) bool
	responds = func(name string) bool {
		if seen[name] {
			return false
		}
		seen[name] = true
		for _, f := range h.functions {
			if f.name != name {
				continue
			}
			found := false
			walkSteps(f.steps, nil, func(s *goStep, _ []*goStep) {
				if s.kind == "response" || s.kind == "call" && s.function != "" && s.reached != "defined" && responds(s.function) {
					found = true
				}
			})
			return found
		}
		return false
	}
	return responds(name)
}

// writesAfterResponse asks about a table written after the response was
// written in the same block.
func (h *goHandler) writesAfterResponse() {
	for _, f := range h.functions {
		h.afterResponse(f, f.steps)
	}
}

func (h *goHandler) afterResponse(f *goFunction, steps []*goStep) {
	var responded *goStep
	for _, s := range steps {
		if responded != nil {
			walkSteps([]*goStep{s}, nil, func(w *goStep, _ []*goStep) {
				if w.kind == "database" && w.access == "write" {
					h.fault("certain", w, fmt.Sprintf("%s %s at line %d after it %s at line %d, in %s; a failure of the write can no longer change the answer. Is the order meant?", h.handlerText(), w.text, w.first, responded.text, responded.first, f.name),
						"A write after the response is sent fails silently for the caller, who was already told the request succeeded.", "Writes after the response.")
				}
			})
		}
		if s.kind == "response" {
			responded = s
		}
		h.afterResponse(f, s.then)
		h.afterResponse(f, s.otherwise)
		for _, c := range s.cases {
			h.afterResponse(f, c.steps)
		}
		h.afterResponse(f, s.body)
	}
}

// writes is every database write of the handler's functions, in order.
func (h *goHandler) writes() []*goStep {
	var out []*goStep
	h.each(func(_ *goFunction, s *goStep, _ []*goStep) {
		if s.kind == "database" && s.access == "write" {
			out = append(out, s)
		}
	})
	return out
}

// uncheckedWrites asks about a handler that writes a table while its
// operation runs no middleware and checks no permission, where other
// operations of the source do.
func (h *goHandler) uncheckedWrites() {
	writes := h.writes()
	if len(writes) == 0 {
		return
	}
	g := h.g
	others := map[string]int{}
	switch {
	case h.ep != nil:
		if !h.ep.chainKnown || len(h.ep.chain) > 0 || !h.ep.privKnown || len(h.ep.privileges) > 0 {
			return
		}
		for _, o := range g.endpoints {
			if o != h.ep {
				for _, n := range chainNames(o) {
					others[n]++
				}
				if len(o.privileges) > 0 {
					others["a privilege"]++
				}
			}
		}
	case h.route != nil:
		if len(h.route.middleware) > 0 || len(h.route.checks) > 0 {
			return
		}
		for _, o := range g.rt.routes {
			if o != h.route {
				for _, m := range o.middleware {
					others[m]++
				}
				for _, c := range o.checks {
					others[c.check]++
				}
			}
		}
	}
	var names []string
	for n := range others {
		if n != "a privilege" {
			names = append(names, n)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		if others[names[i]] != others[names[j]] {
			return others[names[i]] > others[names[j]]
		}
		return names[i] < names[j]
	})
	var parts []string
	if len(names) > 0 {
		parts = append(parts, "run "+names[0])
	}
	if others["a privilege"] > 0 {
		parts = append(parts, "check a privilege")
	}
	if len(parts) == 0 {
		return
	}
	w := writes[0]
	h.fault("suspected", w, fmt.Sprintf("%s %s and runs no middleware and checks no permission, while other operations %s. Should a caller be checked before it writes?", h.handlerText(), w.text, strings.Join(parts, " and ")),
		"A write open to every caller changes data nobody was asked to allow, which the other operations of the same service do not do.", "Writes with no check before it.")
}

// writesWithoutTransaction asks about several writes with no transaction
// around them.
func (h *goHandler) writesWithoutTransaction() {
	var plain []*goStep
	begun := false
	h.each(func(_ *goFunction, s *goStep, _ []*goStep) {
		if s.kind == "transaction" {
			begun = true
		}
		if s.kind == "database" && s.access == "write" && !s.transaction {
			plain = append(plain, s)
		}
	})
	if begun || len(plain) < 2 {
		return
	}
	a, b := plain[0], plain[1]
	h.fault("suspected", a, fmt.Sprintf("%s %s at line %d and %s at %s:%d with no transaction around them, so a failure of the second leaves the first in place. Must they succeed together?", h.handlerText(), a.text, a.first, b.text, b.file.path, b.first),
		"Two writes outside a transaction can leave the data half changed when the second fails.", "Writes twice with no transaction.")
}

// ignoredError asks about an error a call gives that the code drops.
func (sc *fnScope) ignoredError(certainty string, n ast.Node, callText string) {
	h := sc.h
	h.ask(certainty, h.g.clause(sc.file, n.Pos()), fmt.Sprintf("the result of %s is dropped in %s, so when the call fails %s goes on as if it had succeeded. Is the error meant to be ignored?", callText, sc.fn.name, h.handlerText()),
		"An error dropped is a failure nobody sees: the request is answered as a success, or with a value that is not there.", "Drops the result of "+callText+".")
}

// unreachable asks about statements that follow a return or a panic in
// the same block, and a constant condition.
func (sc *fnScope) unreachable(after ast.Stmt, first, last ast.Stmt) {
	h := sc.h
	g := h.g
	from, to := g.fset.Position(first.Pos()).Line, g.fset.Position(last.End()).Line
	where, they := fmt.Sprintf("line %d of %s in %s is", from, sc.file.path, sc.fn.name), "it follows"
	if to > from {
		where, they = fmt.Sprintf("lines %d-%d of %s in %s are", from, to, sc.file.path, sc.fn.name), "they follow"
	}
	h.ask("certain", g.clause(sc.file, first.Pos()), fmt.Sprintf("%s never run in %s: %s %s at line %d. Is the code meant to be there?", where, handlerPlaceholder, they, sc.text(after, after), g.fset.Position(after.Pos()).Line),
		"Code after a return in the same block cannot run, so what it does is not done.", "Follows a return in the same block.")
}

func (sc *fnScope) constantCondition(st *ast.IfStmt) {
	h := sc.h
	g := h.g
	cond := exprText(st.Cond)
	h.ask("certain", g.clause(sc.file, st.Pos()), fmt.Sprintf("the condition at line %d of %s in %s, reached by %s, is the constant %s, so one of its branches is never run. Is the branch meant to be switched off?", g.fset.Position(st.Pos()).Line, sc.file.path, sc.fn.name, handlerPlaceholder, cond),
		"A constant condition leaves one branch dead, so what it does is not done.", "Branches on the constant "+cond+".")
}
