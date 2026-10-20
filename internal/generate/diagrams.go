package generate

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/implies"
	"github.com/SpecArch/specarch/internal/source"
)

func fence(body string) string {
	return "```mermaid\n" + body + "```\n"
}

// entityDiagram is the erDiagram of every entity, its fields and relations.
func entityDiagram(root *yaml.Node) string {
	entities := pairs(root, "entities")
	if len(entities) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("erDiagram\n")
	// A relation declared from both sides is drawn once, from the side
	// that holds many.
	back := map[string]bool{}
	for _, e := range entities {
		for _, r := range pairs(e.Value, "relations") {
			if str(r.Value, "kind") == "one-to-many" {
				back[str(r.Value, "target")+"|"+e.Key.Value+"|"+str(r.Value, "via")] = true
			}
		}
	}
	for _, e := range entities {
		for _, r := range pairs(e.Value, "relations") {
			target, via := erName(str(r.Value, "target")), str(r.Value, "via")
			var line string
			switch str(r.Value, "kind") {
			case "one-to-many":
				line = fmt.Sprintf("  %s ||--o{ %s : %s\n", e.Key.Value, target, r.Key.Value)
			case "many-to-one":
				if back[e.Key.Value+"|"+target+"|"+via] {
					continue
				}
				line = fmt.Sprintf("  %s }o--|| %s : %s\n", e.Key.Value, target, r.Key.Value)
			case "one-to-one":
				line = fmt.Sprintf("  %s ||--|| %s : %s\n", e.Key.Value, target, r.Key.Value)
			case "many-to-many":
				line = fmt.Sprintf("  %s }o--o{ %s : %s\n", e.Key.Value, target, r.Key.Value)
			}
			b.WriteString(line)
		}
	}
	for _, e := range entities {
		pk := map[string]bool{}
		for _, k := range strs(e.Value, "primaryKey") {
			pk[k] = true
		}
		fk := map[string]bool{}
		for _, r := range pairs(e.Value, "relations") {
			if k := str(r.Value, "kind"); k == "many-to-one" || k == "one-to-one" {
				fk[str(r.Value, "via")] = true
			}
		}
		fmt.Fprintf(&b, "  %s {\n", e.Key.Value)
		for _, f := range pairs(e.Value, "properties") {
			keys := ""
			switch {
			case pk[f.Key.Value] && fk[f.Key.Value]:
				keys = " PK, FK"
			case pk[f.Key.Value]:
				keys = " PK"
			case fk[f.Key.Value]:
				keys = " FK"
			}
			fmt.Fprintf(&b, "    %s %s%s\n", mermaidID(strings.NewReplacer("list of ", "list_", "map of ", "map_").Replace(erName(typeLabel(f.Value)))), f.Key.Value, keys)
		}
		b.WriteString("  }\n")
	}
	return fence(b.String())
}

// stateDiagram draws one entity's transitions.
func stateDiagram(root *yaml.Node, entity string) (string, error) {
	e := get(get(root, "entities"), entity)
	if e == nil {
		return "", fmt.Errorf("%s is not an entity of the design file", entity)
	}
	transitions := items(e, "transitions")
	if len(transitions) == 0 {
		return "", fmt.Errorf("entity %s has no transitions to draw", entity)
	}
	var b strings.Builder
	b.WriteString("stateDiagram-v2\n")
	in, out := map[string]bool{}, map[string]bool{}
	var order []string
	seen := map[string]bool{}
	note := func(s string) {
		if !seen[s] {
			seen[s] = true
			order = append(order, s)
		}
	}
	for _, t := range transitions {
		from, to := str(t, "from"), str(t, "to")
		out[from], in[to] = true, true
		note(from)
		note(to)
	}
	for _, s := range order {
		if !in[s] {
			fmt.Fprintf(&b, "  [*] --> %s\n", s)
		}
	}
	for _, t := range transitions {
		line := fmt.Sprintf("  %s --> %s", str(t, "from"), str(t, "to"))
		if trig := str(t, "trigger"); trig != "" {
			line += " : " + trig
		}
		b.WriteString(line + "\n")
	}
	for _, s := range order {
		if !out[s] {
			fmt.Fprintf(&b, "  %s --> [*]\n", s)
		}
	}
	return fence(b.String()), nil
}

// sequenceDiagram draws one operation or command.
func sequenceDiagram(root *yaml.Node, name string) (string, error) {
	if o := findOperation(root, name); o != nil {
		return operationSequence(root, *o), nil
	}
	if c := get(get(root, "commands"), name); c != nil {
		return commandSequence(root, name, c), nil
	}
	return "", fmt.Errorf("%s is not an operationId or a command of the design file", name)
}

func operationSequence(root *yaml.Node, o operation) string {
	var b strings.Builder
	b.WriteString("sequenceDiagram\n")
	b.WriteString("  participant C as Client\n")
	fmt.Fprintf(&b, "  participant S as %s\n", mermaidText(str(get(root, "info"), "title")))
	channels := map[string]string{}
	var chOrder []string
	for _, e := range items(o.node, "emits") {
		ch, _, _ := strings.Cut(e.Value, "/")
		if _, ok := channels[ch]; !ok {
			channels[ch] = fmt.Sprintf("Q%d", len(channels)+1)
			chOrder = append(chOrder, ch)
		}
	}
	for _, ch := range chOrder {
		fmt.Fprintf(&b, "  participant %s as %s\n", channels[ch], ch)
	}
	calls := strs(o.node, "calls")
	for i, dep := range calls {
		fmt.Fprintf(&b, "  participant D%d as %s\n", i+1, callLabel(dep))
	}
	request := fmt.Sprintf("%s %s", strings.ToUpper(o.method), o.path)
	if streamDirection(o.node) == "upload" {
		request += ", the file streamed"
	}
	if h := str(o.node, "handshake"); h != "" {
		fmt.Fprintf(&b, "  Note over C,S: in the envelope of handshake %s\n", mermaidText(h))
		request += ", sealed"
	}
	fmt.Fprintf(&b, "  C->>S: %s\n", request)
	for i, dep := range calls {
		fmt.Fprintf(&b, "  S->>D%d: call, within %s\n", i+1, str(get(get(root, "dependencies"), calledDependency(dep)), "timeout"))
		fmt.Fprintf(&b, "  D%d-->>S: answer\n", i+1)
	}
	if alg := str(o.node, "algorithm"); alg != "" {
		var inputs []string
		for _, p := range pairs(get(get(root, "algorithms"), alg), "inputs") {
			inputs = append(inputs, p.Key.Value)
		}
		fmt.Fprintf(&b, "  S->>S: %s(%s)\n", alg, strings.Join(inputs, ", "))
	}
	for _, e := range items(o.node, "emits") {
		ch, msg, _ := strings.Cut(e.Value, "/")
		fmt.Fprintf(&b, "  S-->>%s: %s\n", channels[ch], msg)
	}
	if code, body := successResponse(o.node); code != "" {
		switch {
		case streamDirection(o.node) == "download":
			body = "the file, streamed"
		case str(o.node, "handshake") != "":
			body += ", sealed"
		}
		fmt.Fprintf(&b, "  S-->>C: %s %s\n", code, strings.TrimSpace(body))
	}
	return fence(b.String())
}

// successResponse is the first 2xx response and what it carries.
func successResponse(op *yaml.Node) (string, string) {
	for _, r := range pairs(op, "responses") {
		if !strings.HasPrefix(r.Key.Value, "2") {
			continue
		}
		for _, c := range pairs(r.Value, "content") {
			return r.Key.Value, typeLabel(get(c.Value, "schema"))
		}
		return r.Key.Value, ""
	}
	return "", ""
}

func commandSequence(root *yaml.Node, name string, c *yaml.Node) string {
	var b strings.Builder
	b.WriteString("sequenceDiagram\n")
	b.WriteString("  participant U as User\n")
	fmt.Fprintf(&b, "  participant P as %s\n", mermaidText(str(get(root, "info"), "title")))
	files := len(items(c, "reads"))+len(items(c, "writes")) > 0
	if files {
		b.WriteString("  participant F as Files\n")
	}
	args := []string{name}
	for _, a := range items(c, "arguments") {
		args = append(args, "<"+str(a, "name")+">")
	}
	fmt.Fprintf(&b, "  U->>P: %s\n", strings.Join(args, " "))
	for _, r := range items(c, "reads") {
		fmt.Fprintf(&b, "  P->>F: read %s\n", mermaidText(str(r, "path")))
	}
	if alg := str(c, "algorithm"); alg != "" {
		fmt.Fprintf(&b, "  P->>P: %s\n", alg)
	}
	for _, w := range items(c, "writes") {
		fmt.Fprintf(&b, "  P->>F: write %s\n", mermaidText(str(w, "path")))
	}
	var codes []string
	for _, e := range pairs(c, "exitCodes") {
		codes = append(codes, e.Key.Value)
	}
	fmt.Fprintf(&b, "  P-->>U: exit status %s\n", strings.Join(codes, ", "))
	return fence(b.String())
}

// pagesFlowchart draws the pages and where their actions lead.
func pagesFlowchart(root *yaml.Node) string {
	pages := pairs(root, "pages")
	if len(pages) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	for _, p := range pages {
		fmt.Fprintf(&b, "  %s[\"%s (%s)\"]\n", mermaidID(p.Key.Value), mermaidText(str(p.Value, "title")), str(p.Value, "kind"))
	}
	ops := map[string]bool{}
	op := func(target string) string {
		if !ops[target] {
			ops[target] = true
			fmt.Fprintf(&b, "  op_%s([\"%s\"])\n", mermaidID(target), target)
		}
		return "op_" + mermaidID(target)
	}
	for _, p := range pages {
		from := mermaidID(p.Key.Value)
		if nav := str(get(p.Value, "onSelect"), "navigate"); nav != "" {
			fmt.Fprintf(&b, "  %s -->|\"select\"| %s\n", from, mermaidID(nav))
		}
		if submit := str(p.Value, "submit"); submit != "" {
			o := op(submit)
			fmt.Fprintf(&b, "  %s -.->|\"submit\"| %s\n", from, o)
			if str(p.Value, "kind") == "task" {
				for _, kv := range pairs(p.Value, "onSubmitted") {
					if nav := str(kv.Value, "navigate"); nav != "" {
						fmt.Fprintf(&b, "  %s -->|\"%s\"| %s\n", o, kv.Key.Value, mermaidID(nav))
					}
				}
			} else if nav := str(get(p.Value, "onSubmitted"), "navigate"); nav != "" {
				fmt.Fprintf(&b, "  %s -->|\"submitted\"| %s\n", o, mermaidID(nav))
			}
		}
		for _, a := range items(p.Value, "actions") {
			target := str(a, "target")
			label := mermaidText(str(a, "label"))
			switch str(a, "kind") {
			case "navigate":
				fmt.Fprintf(&b, "  %s -->|\"%s\"| %s\n", from, label, mermaidID(target))
			case "operation":
				o := op(target)
				fmt.Fprintf(&b, "  %s -.->|\"%s\"| %s\n", from, label, o)
				if nav := str(get(a, "then"), "navigate"); nav != "" {
					fmt.Fprintf(&b, "  %s -->|\"%s done\"| %s\n", o, label, mermaidID(nav))
				}
			}
		}
	}
	return fence(b.String())
}

// permissionsTable is the matrix of permissions and the roles that grant
// them: yes where a role grants it, and through the permission that
// implies it where the role holds it only by an implication.
func permissionsTable(root *yaml.Node) string {
	perms := pairs(root, "permissions")
	if len(perms) == 0 {
		return ""
	}
	roles := pairs(root, "roles")
	grants := map[string]map[string]bool{}
	for _, r := range roles {
		grants[r.Key.Value] = roleHolds(root, r.Value)
	}
	// A client with roles is a column of its own, named with its kind,
	// so the callers are the rows' holders beside the roles (ADR-110).
	var clients []source.Pair
	for _, c := range pairs(root, "clients") {
		if len(strs(c.Value, "roles")) > 0 {
			clients = append(clients, c)
		}
	}
	hasPublic := get(get(root, "permissions"), "public") != nil
	var b strings.Builder
	b.WriteString("| Permission |")
	for _, r := range roles {
		b.WriteString(" " + r.Key.Value + " |")
	}
	for _, c := range clients {
		b.WriteString(" " + c.Key.Value + " (" + str(c.Value, "kind") + ") |")
	}
	if hasPublic {
		b.WriteString(" public |")
	}
	b.WriteString("\n|---|")
	b.WriteString(strings.Repeat("---|", len(roles)+len(clients)))
	if hasPublic {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for _, p := range perms {
		fmt.Fprintf(&b, "| %s |", p.Key.Value)
		for _, r := range roles {
			if list := strs(get(r.Value, "through"), p.Key.Value); len(list) > 0 && grants[r.Key.Value][p.Key.Value] {
				// A grant narrowed to the clients it holds through (ADR-106).
				b.WriteString(" " + strings.Join(list, ", ") + " only |")
			} else if through := implies.Through(strs(r.Value, "permissions"), p.Key.Value, impliedBy(root)); through != "" {
				b.WriteString(" through " + through + " |")
			} else if grants[r.Key.Value][p.Key.Value] {
				b.WriteString(" yes |")
			} else {
				b.WriteString(" |")
			}
		}
		for _, c := range clients {
			held := false
			for _, role := range strs(c.Value, "roles") {
				held = held || grants[role][p.Key.Value]
			}
			if held {
				b.WriteString(" yes |")
			} else {
				b.WriteString(" |")
			}
		}
		if hasPublic {
			switch p.Key.Value {
			case "public":
				b.WriteString(" everyone |")
			case "nobody":
				b.WriteString(" closed |")
			default:
				b.WriteString(" |")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// region renders what a marker names: "erDiagram", "stateDiagram Loan",
// "sequenceDiagram returnLoan", "flowchart pages", "permissions" or
// "context".
func region(root *yaml.Node, what string) (string, error) {
	kind, arg, _ := strings.Cut(strings.TrimSpace(what), " ")
	arg = strings.TrimSpace(arg)
	var out string
	var err error
	switch kind {
	case "erDiagram":
		out = entityDiagram(root)
		if out == "" {
			err = fmt.Errorf("the design file has no entities to draw")
		}
	case "stateDiagram":
		out, err = stateDiagram(root, arg)
	case "sequenceDiagram":
		out, err = sequenceDiagram(root, arg)
	case "flowchart":
		if arg != "pages" {
			return "", fmt.Errorf("flowchart takes pages, as flowchart pages")
		}
		out = pagesFlowchart(root)
		if out == "" {
			err = fmt.Errorf("the design file has no pages to draw")
		}
	case "permissions":
		out = permissionsTable(root)
		if out == "" {
			err = fmt.Errorf("the design file has no permissions")
		}
	case "context":
		out = contextDiagram(root)
		if out == "" {
			err = fmt.Errorf("the design file has no roles, dependencies or channels to draw")
		}
	default:
		names := []string{"context", "erDiagram", "stateDiagram <Entity>", "sequenceDiagram <operationId or command>", "flowchart pages", "permissions"}
		sort.Strings(names)
		return "", fmt.Errorf("%q is not something the generator draws; use one of %s", kind, strings.Join(names, ", "))
	}
	return out, err
}

// flowFlowchart draws a flow as its steps: each page, and the event on it
// that leads to the next.
func flowFlowchart(flow *yaml.Node) string {
	steps := items(flow, "steps")
	if len(steps) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	for i, st := range steps {
		fmt.Fprintf(&b, "  s%d[\"%s\"]\n", i, mermaidText(str(st, "page")))
	}
	for i := 0; i+1 < len(steps); i++ {
		event := str(steps[i], "event")
		if event == "action" {
			event = str(steps[i], "action")
		}
		fmt.Fprintf(&b, "  s%d -->|\"%s\"| s%d\n", i, mermaidText(event), i+1)
	}
	if last := steps[len(steps)-1]; str(last, "event") != "" {
		event := str(last, "event")
		if event == "action" {
			event = str(last, "action")
		}
		fmt.Fprintf(&b, "  s%d -->|\"%s\"| done((\"done\"))\n", len(steps)-1, mermaidText(event))
	}
	return fence(b.String())
}

// workflowFlowchart draws a workflow as its steps in order: each approval
// with its approvers and permission, the refusal that ends the request,
// the deadline on the edge it takes, and each operation the system calls.
func workflowFlowchart(d *doc, name string, w *yaml.Node) string {
	steps := items(w, "steps")
	if len(steps) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	fmt.Fprintf(&b, "  start([\"%s answers 202\"])\n", mermaidText(d.known(w, "trigger", "the trigger", "workflows", name)))
	index := map[string]int{}
	refused := false
	for i, st := range steps {
		index[str(st, "name")] = i
		at := []string{"workflows", name, "steps", fmt.Sprint(i)}
		if str(st, "kind") == "approval" {
			refused = true
			approvers := strings.Join(strs(st, "approvers"), ", ")
			if approvers == "" {
				approvers = d.known(st, "approvers", "the approvers", at...)
			}
			fmt.Fprintf(&b, "  s%d[\"%s: %s with %s\"]\n", i, mermaidText(str(st, "name")), mermaidText(approvers), mermaidText(d.known(st, "permission", "the permission", at...)))
		} else {
			fmt.Fprintf(&b, "  s%d[[\"%s\"]]\n", i, mermaidText(d.known(st, "operation", "the operation", at...)))
		}
	}
	b.WriteString("  done((\"approved\"))\n")
	if refused {
		b.WriteString("  refused((\"refused\"))\n")
	}
	b.WriteString("  start --> s0\n")
	for i, st := range steps {
		next := "done"
		if i+1 < len(steps) {
			next = fmt.Sprintf("s%d", i+1)
		}
		if str(st, "kind") != "approval" {
			fmt.Fprintf(&b, "  s%d --> %s\n", i, next)
			continue
		}
		fmt.Fprintf(&b, "  s%d -->|\"approved\"| %s\n", i, next)
		fmt.Fprintf(&b, "  s%d -->|\"refused\"| refused\n", i)
		late := "refused"
		if to, ok := index[str(st, "escalateTo")]; ok && str(st, "onDeadline") == "escalate" {
			late = fmt.Sprintf("s%d", to)
		}
		fmt.Fprintf(&b, "  s%d -->|\"%s passes\"| %s\n", i, mermaidText(d.known(st, "deadline", "the deadline", "workflows", name, "steps", fmt.Sprint(i))), late)
	}
	return fence(b.String())
}

// erName is a relation's target as the entity diagram names it: another
// specification's entity as the dependency and the entity's name, since a
// diagram's name holds no # or /.
func erName(target string) string {
	if dep, ptr, ok := strings.Cut(target, "#"); ok && dep != "" {
		return dep + "_" + ptr[strings.LastIndex(ptr, "/")+1:]
	}
	return target
}

// callLabel is a calls entry as a sequence diagram names it: the
// dependency, and for one of its operations the method and the path.
func callLabel(c string) string {
	dep, ptr, ok := strings.Cut(c, "#")
	if !ok || dep == "" {
		return c
	}
	tokens := strings.Split(strings.TrimPrefix(ptr, "/"), "/")
	if len(tokens) != 3 || tokens[0] != "paths" {
		return dep
	}
	return dep + " " + strings.ToUpper(tokens[2]) + " " + strings.NewReplacer("~1", "/", "~0", "~").Replace(tokens[1])
}

// handlerLogic writes what an operation's handler does, from the handler
// record of each implementation file that maps the operation (ADR-144):
// per function followed, a flowchart of its steps and its source as it was
// at the commit read, so a reader checks the chart against the code, and
// the calls that were not followed.
func handlerLogic(d *doc, o operation, impls []Implementation) {
	ptr := "#/paths/" + strings.NewReplacer("~", "~0", "/", "~1").Replace(o.path) + "/" + o.method
	var recorded []Implementation
	for _, i := range impls {
		if get(get(get(i.Node, "mappings"), ptr), "handler") != nil {
			recorded = append(recorded, i)
		}
	}
	for _, i := range recorded {
		m := get(get(i.Node, "mappings"), ptr)
		rec := get(m, "handler")
		title := "What the handler does"
		if len(recorded) > 1 {
			title += " in " + str(get(i.Node, "info"), "title")
		}
		d.heading(4, title)
		fns := items(rec, "functions")
		d.para(fmt.Sprintf("The handler is %s. Read from the code at commit %s by `specarch extract go`, which followed the calls of functions of the same code up to %s deep from the handler: %s. Each chart's node carries its step's id, and the source under the chart is the function as it was at that commit, so the chart can be checked against the code.",
			str(m, "target"), shortCommit(str(rec, "commit")), str(rec, "depth"), functionsText(fns)))
		var stopped []string
		for _, fn := range fns {
			d.para(fmt.Sprintf("**%s**, %s lines %s to %s.", str(fn, "name"), str(fn, "file"), str(fn, "firstLine"), str(fn, "lastLine")))
			d.block(handlerFlowchart(fn))
			d.block("```go\n" + str(fn, "source") + "\n```\n")
			walkStepNodes(items(fn, "steps"), func(st *yaml.Node) {
				if str(st, "kind") == "call" && str(st, "stopped") != "" {
					stopped = append(stopped, fmt.Sprintf("%s at %s:%s (%s)", str(st, "text"), str(st, "file"), str(st, "firstLine"), str(st, "stopped")))
				}
			})
		}
		if len(stopped) > 0 {
			d.para("Not followed: " + strings.Join(stopped, "; ") + ".")
		}
	}
}

// functionsText counts the functions of a handler record.
func functionsText(fns []*yaml.Node) string {
	switch len(fns) {
	case 1:
		return "the handler alone, which calls no function of the code"
	case 2:
		return "the handler and 1 function it reaches"
	}
	return fmt.Sprintf("the handler and %d functions it reaches", len(fns)-1)
}

// shortCommit is the first seven characters of a commit.
func shortCommit(c string) string {
	if len(c) > 7 {
		return c[:7]
	}
	return c
}

// walkStepNodes calls fn on every step and the steps inside it.
func walkStepNodes(steps []*yaml.Node, fn func(st *yaml.Node)) {
	for _, st := range steps {
		fn(st)
		walkStepNodes(items(st, "then"), fn)
		walkStepNodes(items(st, "otherwise"), fn)
		for _, c := range items(st, "cases") {
			walkStepNodes(items(c, "steps"), fn)
		}
		walkStepNodes(items(st, "body"), fn)
	}
}

// chartExit is a node an edge leaves, with the label the edge carries.
type chartExit struct {
	node, label string
}

// handlerFlowchart draws one function of a handler record: a node per
// step, shaped by its kind and named by the step's id, from a begin node to
// the returns and an end node for a path that falls off the end.
func handlerFlowchart(fn *yaml.Node) string {
	var b strings.Builder
	b.WriteString("flowchart TD\n")
	name := str(fn, "name")
	fmt.Fprintf(&b, "  begin([\"%s\"])\n", mermaidText(name))
	var edges []string
	exits := drawSteps(&b, &edges, items(fn, "steps"), []chartExit{{node: "begin"}}, nil)
	if len(exits) > 0 {
		fmt.Fprintf(&b, "  finish([\"end of %s\"])\n", mermaidText(name))
		for _, e := range exits {
			edges = append(edges, edge(e, "finish"))
		}
	}
	for _, e := range edges {
		b.WriteString("  " + e + "\n")
	}
	return fence(b.String())
}

// edge is one edge of the chart.
func edge(from chartExit, to string) string {
	if from.label == "" {
		return from.node + " --> " + to
	}
	return fmt.Sprintf("%s -->|\"%s\"| %s", from.node, mermaidText(from.label), to)
}

// chartScope is a loop or a choice the steps drawn are inside, for a step
// that leaves it or goes on with a loop's next round: the innermost is
// given, and each knows the one around it.
type chartScope struct {
	id, kind, label string      // kind is loop or choice
	breaks          []chartExit // the exits of the steps that leave it
	parent          *chartScope
}

// target is the scope a break or a continue goes to: the one its label
// names, or the innermost loop for a continue or a break that leaves a
// loop, or the innermost scope for a break that leaves a switch or a
// select; nil when none is drawn.
func (sc *chartScope) target(label string, loopsOnly bool) *chartScope {
	for s := sc; s != nil; s = s.parent {
		switch {
		case label != "":
			if s.label == label {
				return s
			}
		case !loopsOnly || s.kind == "loop":
			return s
		}
	}
	return nil
}

// drawSteps draws steps in order, each reached from the exits of the one
// before it, and returns the exits of the last.
func drawSteps(b *strings.Builder, edges *[]string, steps []*yaml.Node, from []chartExit, scope *chartScope) []chartExit {
	for _, st := range steps {
		id := str(st, "id")
		text := mermaidText(str(st, "text"))
		kind := str(st, "kind")
		shape := map[string]string{
			"input": "[/\"%s\"/]", "branch": "{\"%s\"}", "choice": "{{\"%s\"}}", "loop": "[/\"%s\"\\]",
			"database": "[(\"%s\")]", "transaction": "[(\"%s\")]", "call": "[[\"%s\"]]", "external": ">\"%s\"]",
			"response": "([\"%s\"])", "return": "((\"%s\"))", "statement": "[\"%s\"]",
		}[kind]
		if shape == "" {
			shape = "[\"%s\"]"
		}
		fmt.Fprintf(b, "  %s"+shape+"\n", id, text)
		for _, e := range from {
			*edges = append(*edges, edge(e, id))
		}
		switch kind {
		case "return":
			from = nil
		case "branch":
			var next []chartExit
			next = append(next, drawSteps(b, edges, items(st, "then"), []chartExit{{id, "yes"}}, scope)...)
			if get(st, "otherwise") != nil {
				next = append(next, drawSteps(b, edges, items(st, "otherwise"), []chartExit{{id, "no"}}, scope)...)
			} else {
				next = append(next, chartExit{id, "no"})
			}
			from = next
		case "choice":
			inner := &chartScope{id: id, kind: "choice", label: str(st, "label"), parent: scope}
			// A case that falls through leads into the next case's first
			// step, not past the switch.
			var next, carried []chartExit
			hasOtherwise := false
			for _, c := range items(st, "cases") {
				when := str(c, "when")
				hasOtherwise = hasOtherwise || when == "otherwise"
				steps := items(c, "steps")
				ends := drawSteps(b, edges, steps, append([]chartExit{{id, when}}, carried...), inner)
				if fallsThroughNodes(steps) {
					carried = ends
					continue
				}
				next = append(next, ends...)
				carried = nil
			}
			next = append(next, carried...)
			if !hasOtherwise {
				next = append(next, chartExit{id, "otherwise"})
			}
			from = append(next, inner.breaks...)
		case "loop":
			inner := &chartScope{id: id, kind: "loop", label: str(st, "label"), parent: scope}
			for _, e := range drawSteps(b, edges, items(st, "body"), []chartExit{{id, "each time"}}, inner) {
				*edges = append(*edges, edge(chartExit{e.node, joinLabel(e.label, "again")}, id))
			}
			from = append([]chartExit{{id, "done"}}, inner.breaks...)
		case "statement":
			text := str(st, "text")
			label := str(st, "label")
			switch {
			case strings.HasPrefix(text, "panics"):
				from = nil
			case strings.HasPrefix(text, "goes on with the next round of the loop"):
				if t := scope.target(label, true); t != nil {
					*edges = append(*edges, edge(chartExit{node: id}, t.id))
					from = nil
				} else {
					from = []chartExit{{node: id}}
				}
			case strings.HasPrefix(text, "leaves the "):
				if t := scope.target(label, strings.HasPrefix(text, "leaves the loop")); t != nil {
					t.breaks = append(t.breaks, chartExit{node: id})
					from = nil
				} else {
					from = []chartExit{{node: id}}
				}
			default:
				from = []chartExit{{node: id}}
			}
		default:
			from = []chartExit{{node: id}}
		}
	}
	return from
}

// fallsThroughNodes says whether a case's steps end in a fallthrough.
func fallsThroughNodes(steps []*yaml.Node) bool {
	if len(steps) == 0 {
		return false
	}
	last := steps[len(steps)-1]
	return str(last, "kind") == "statement" && str(last, "text") == "falls through to the next case"
}

// joinLabel joins an edge's label with a word, for an edge that is both a
// branch's answer and a loop's way back.
func joinLabel(label, word string) string {
	if label == "" {
		return word
	}
	return label + ", " + word
}

// dependencyKinds is the order the context diagram draws the
// dependencies in, grouped by kind (ADR-134).
var dependencyKinds = []string{"system", "identityProvider", "database", "messageBroker", "fileStore"}

// dependencyShape is the node of a dependency in the context diagram, a
// shape per kind.
func dependencyShape(id, label, kind string) string {
	switch kind {
	case "database":
		return fmt.Sprintf("%s[(\"%s\")]", id, label)
	case "messageBroker":
		return fmt.Sprintf("%s[[\"%s\"]]", id, label)
	case "fileStore":
		return fmt.Sprintf("%s[/\"%s\"/]", id, label)
	case "identityProvider":
		return fmt.Sprintf("%s{{\"%s\"}}", id, label)
	}
	return fmt.Sprintf("%s[\"%s\"]", id, label)
}

// fewNames is up to three names and then how many more there are.
func fewNames(names []string) string {
	if len(names) <= 3 {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(names[:3], ", "), len(names)-3)
}

// usesContextKeys says whether the design keys any dependency with a kind
// or a protocol, or any channel with carriedBy, so that the technical
// specification draws the context diagram only then and a specification
// without the keys renders as before.
func usesContextKeys(root *yaml.Node) bool {
	for _, dep := range pairs(root, "dependencies") {
		if get(dep.Value, "kind") != nil || get(dep.Value, "protocol") != nil {
			return true
		}
	}
	for _, ch := range pairs(root, "channels") {
		if get(ch.Value, "carriedBy") != nil {
			return true
		}
	}
	return false
}

// contextDiagram is the system context in C4's style (ADR-134, ADR-135):
// a person per role no job acts as and no service client holds, and one
// for anyone when an operation or a page is public; a system per client of
// kind service, with the roles it holds; this system in the middle; and
// the dependencies by kind, with their protocol, each edge labelled with
// the operations and jobs that call it and the channels it carries.
func contextDiagram(root *yaml.Node) string {
	deps := pairs(root, "dependencies")
	chans := pairs(root, "channels")
	roles := pairs(root, "roles")
	if len(deps)+len(chans)+len(roles) == 0 {
		return ""
	}
	jobs := pairs(root, "jobs")
	jobRoles := map[string]bool{}
	for _, j := range jobs {
		jobRoles[str(j.Value, "role")] = true
	}
	var services, applications []source.Pair
	serviceRoles := map[string]bool{}
	for _, c := range pairs(root, "clients") {
		if str(c.Value, "kind") == "service" {
			services = append(services, c)
			for _, r := range strs(c.Value, "roles") {
				serviceRoles[r] = true
			}
		} else {
			applications = append(applications, c)
		}
	}
	public := false
	for _, o := range operations(root) {
		public = public || str(o.node, "permission") == "public"
	}
	for _, p := range pairs(root, "pages") {
		public = public || str(p.Value, "permission") == "public"
	}
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	system := "system"
	fmt.Fprintf(&b, "  %s[\"%s\"]\n", system, mermaidText(str(get(root, "info"), "title")))
	// An application client is the way a person reaches the system, drawn
	// on the person's edge, not as a node.
	through := ""
	if len(applications) > 0 {
		var names []string
		for _, a := range applications {
			names = append(names, a.Key.Value)
		}
		through = fmt.Sprintf(" -->|\"%s\"|", mermaidText(fewNames(names)))
	} else {
		through = " -->"
	}
	for _, r := range roles {
		if jobRoles[r.Key.Value] || serviceRoles[r.Key.Value] {
			continue
		}
		id := "person_" + mermaidID(r.Key.Value)
		fmt.Fprintf(&b, "  %s([\"%s (person)\"])\n", id, mermaidText(r.Key.Value))
		fmt.Fprintf(&b, "  %s%s %s\n", id, through, system)
	}
	if public {
		fmt.Fprintf(&b, "  person_anyone([\"anyone (person)\"])\n")
		fmt.Fprintf(&b, "  person_anyone%s %s\n", through, system)
	}
	for _, c := range services {
		id := "client_" + mermaidID(c.Key.Value)
		fmt.Fprintf(&b, "  %s[\"%s (system, as %s)\"]\n", id, mermaidText(c.Key.Value), mermaidText(strings.Join(strs(c.Value, "roles"), ", ")))
		fmt.Fprintf(&b, "  %s --> %s\n", id, system)
	}
	// Who calls each dependency, who emits on each channel and which jobs
	// consume from it, in the order the design declares them.
	callers := map[string][]string{}
	emitters := map[string]bool{}
	consumers := map[string]bool{}
	call := func(name string, n *yaml.Node) {
		for _, c := range strs(n, "calls") {
			dep := calledDependency(c)
			if !contains(callers[dep], name) {
				callers[dep] = append(callers[dep], name)
			}
		}
		for _, e := range strs(n, "emits") {
			ch, _, _ := strings.Cut(e, "/")
			emitters[ch] = true
		}
	}
	for _, o := range operations(root) {
		call(o.id, o.node)
	}
	for _, j := range jobs {
		call(j.Key.Value, j.Value)
		if ch, _, ok := strings.Cut(str(get(j.Value, "trigger"), "consumes"), "/"); ok {
			consumers[ch] = true
		}
	}
	for _, w := range pairs(root, "workflows") {
		call(w.Key.Value, w.Value)
	}
	declared := map[string]bool{}
	for _, kind := range dependencyKinds {
		for _, dep := range deps {
			k := str(dep.Value, "kind")
			if k == "" {
				k = "system"
			}
			if k != kind {
				continue
			}
			declared[dep.Key.Value] = true
			label := mermaidText(dep.Key.Value)
			if p := str(dep.Value, "protocol"); p != "" {
				label += ", " + p
			}
			id := "dependency_" + mermaidID(dep.Key.Value)
			fmt.Fprintf(&b, "  %s\n", dependencyShape(id, label, kind))
			if names := callers[dep.Key.Value]; len(names) > 0 {
				fmt.Fprintf(&b, "  %s -->|\"%s\"| %s\n", system, mermaidText(fewNames(names)), id)
			}
		}
	}
	// A channel goes to each declared dependency that carries it, out when
	// the system emits on it and in when a job consumes from it; a channel
	// carried by none it can name is this system's own, at its boundary.
	for _, ch := range chans {
		var carriers []string
		for _, c := range strs(ch.Value, "carriedBy") {
			if declared[c] {
				carriers = append(carriers, c)
			}
		}
		name := mermaidText(ch.Key.Value)
		if len(carriers) == 0 {
			id := "channel_" + mermaidID(ch.Key.Value)
			fmt.Fprintf(&b, "  %s>\"%s (channel)\"]\n", id, name)
			if consumers[ch.Key.Value] {
				fmt.Fprintf(&b, "  %s -.-> %s\n", id, system)
			}
			if emitters[ch.Key.Value] || !consumers[ch.Key.Value] {
				fmt.Fprintf(&b, "  %s -.-> %s\n", system, id)
			}
			continue
		}
		for _, c := range carriers {
			id := "dependency_" + mermaidID(c)
			if emitters[ch.Key.Value] {
				fmt.Fprintf(&b, "  %s -.->|\"%s\"| %s\n", system, name, id)
			}
			if consumers[ch.Key.Value] {
				fmt.Fprintf(&b, "  %s -.->|\"%s\"| %s\n", id, name, system)
			}
		}
	}
	return fence(b.String())
}
