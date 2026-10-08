package generate

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
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
			target, via := str(r.Value, "target"), str(r.Value, "via")
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
			fmt.Fprintf(&b, "    %s %s%s\n", mermaidID(strings.ReplaceAll(typeLabel(f.Value), "list of ", "list_")), f.Key.Value, keys)
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
		fmt.Fprintf(&b, "  participant D%d as %s\n", i+1, dep)
	}
	fmt.Fprintf(&b, "  C->>S: %s %s\n", strings.ToUpper(o.method), o.path)
	for i, dep := range calls {
		fmt.Fprintf(&b, "  S->>D%d: call, within %s\n", i+1, str(get(get(root, "dependencies"), dep), "timeout"))
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
		fmt.Fprintf(&b, "  S-->>C: %s %s\n", code, body)
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
// them.
func permissionsTable(root *yaml.Node) string {
	perms := pairs(root, "permissions")
	if len(perms) == 0 {
		return ""
	}
	roles := pairs(root, "roles")
	grants := map[string]map[string]bool{}
	for _, r := range roles {
		grants[r.Key.Value] = map[string]bool{}
		for _, p := range strs(r.Value, "permissions") {
			grants[r.Key.Value][p] = true
		}
	}
	hasPublic := get(get(root, "permissions"), "public") != nil
	var b strings.Builder
	b.WriteString("| Permission |")
	for _, r := range roles {
		b.WriteString(" " + r.Key.Value + " |")
	}
	if hasPublic {
		b.WriteString(" public |")
	}
	b.WriteString("\n|---|")
	b.WriteString(strings.Repeat("---|", len(roles)))
	if hasPublic {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for _, p := range perms {
		fmt.Fprintf(&b, "| %s |", p.Key.Value)
		for _, r := range roles {
			if grants[r.Key.Value][p.Key.Value] {
				b.WriteString(" yes |")
			} else {
				b.WriteString(" |")
			}
		}
		if hasPublic {
			if p.Key.Value == "public" {
				b.WriteString(" everyone |")
			} else {
				b.WriteString(" |")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// region renders what a marker names: "erDiagram", "stateDiagram Loan",
// "sequenceDiagram returnLoan", "flowchart pages" or "permissions".
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
	default:
		names := []string{"erDiagram", "stateDiagram <Entity>", "sequenceDiagram <operationId or command>", "flowchart pages", "permissions"}
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
