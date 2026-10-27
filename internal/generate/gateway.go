package generate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// gatewaySection writes a gateway in the runtime view (ADR-141): the
// token it verifies, the claim it reads, its forwards, what it calls and
// runs first, its refusals, the tests that name it, and the sequence of
// one forwarded request.
func gatewaySection(d *doc, root *yaml.Node, name string, g *yaml.Node) {
	d.heading(3, "Gateway "+name)
	d.para(str(g, "description"))
	d.explain(g)
	tokName, claim := str(g, "token"), str(g, "claim")
	tok := get(get(root, "tokens"), tokName)
	var about []string
	if f := str(tok, "format"); f != "" {
		about = append(about, f)
	}
	if s := str(tok, "signature"); s != "" {
		about = append(about, "signed with "+s)
	}
	if iss := str(get(get(tok, "claims"), "iss"), "const"); iss != "" {
		about = append(about, "issued by "+iss)
	}
	verifies := "the " + tokName + " token"
	if len(about) > 0 {
		verifies += " (" + strings.Join(about, ", ") + ")"
	}
	d.para(fmt.Sprintf("Verifies %s before it reads anything else, reads its claim %s, and forwards every request no operation of this specification answers, as received, to the service the value names. It checks no permission: the service checks its own.", verifies, claim))
	d.line("| Value of %s | Forwarded to | Specification |", claim)
	d.line("|---|---|---|")
	deps := get(root, "dependencies")
	for _, row := range items(g, "forwards") {
		dep := str(row, "dependency")
		spec := str(get(deps, dep), "specification")
		if v := str(get(deps, dep), "version"); spec != "" && v != "" {
			spec += " " + v
		}
		d.line("| %s | %s | %s |", cell(str(row, "value")), cell(dep), cell(spec))
	}
	d.blank()
	var text []string
	if calls := calls(g); len(calls) > 0 {
		var parts []string
		for _, c := range calls {
			parts = append(parts, callText(root, c))
		}
		text = append(text, "Calls "+joinAnd(parts)+".")
	}
	if get(g, "middlewares") != nil {
		if m := strs(g, "middlewares"); len(m) > 0 {
			text = append(text, "Runs "+joinAnd(m)+" first, in that order.")
		} else {
			text = append(text, "Runs no middleware.")
		}
	}
	if len(text) > 0 {
		d.para(strings.Join(text, " "))
	}
	if refusals := items(g, "refusals"); len(refusals) > 0 {
		d.line("| Status | Problem type | When |")
		d.line("|---|---|---|")
		for _, r := range refusals {
			d.line("| %s | %s | %s |", str(r, "status"), cell(str(r, "problem")), cell(str(r, "condition")))
		}
		d.blank()
	}
	var tests []string
	for _, t := range pairs(root, "tests") {
		if str(t.Value, "gateway") != name {
			continue
		}
		line := t.Key.Value
		if cov := strs(t.Value, "covers"); len(cov) > 0 {
			line += " (" + strings.Join(cov, ", ") + ")"
		}
		tests = append(tests, line)
	}
	if len(tests) > 0 {
		d.para("Tested by " + joinAnd(tests) + ".")
	}
	d.block(gatewaySequence(root, name, g))
}

// gatewaySequence is one forwarded request: the client to the gateway,
// the gateway to the identity provider it calls for the keys, then to
// the service the claim names and back.
func gatewaySequence(root *yaml.Node, name string, g *yaml.Node) string {
	var b strings.Builder
	b.WriteString("sequenceDiagram\n")
	b.WriteString("  participant C as Client\n")
	fmt.Fprintf(&b, "  participant G as Gateway %s\n", mermaidText(name))
	deps := get(root, "dependencies")
	var keys []string
	for _, c := range calls(g) {
		if dep := calledDependency(c); str(get(deps, dep), "kind") == "identityProvider" {
			keys = append(keys, dep)
		}
	}
	for i, k := range keys {
		fmt.Fprintf(&b, "  participant I%d as %s\n", i+1, mermaidText(k))
	}
	claim := str(g, "claim")
	b.WriteString("  participant S as the service the " + mermaidText(claim) + " names\n")
	fmt.Fprintf(&b, "  C->>G: any request no operation answers, with a %s token\n", mermaidText(str(g, "token")))
	for i := range keys {
		fmt.Fprintf(&b, "  G->>I%d: the signing keys, within %s\n", i+1, str(get(deps, keys[i]), "timeout"))
		fmt.Fprintf(&b, "  I%d-->>G: the keys\n", i+1)
	}
	fmt.Fprintf(&b, "  G->>G: verify the token, read %s, find its forward\n", mermaidText(claim))
	b.WriteString("  G->>S: the request as received\n")
	b.WriteString("  S-->>G: the answer\n")
	b.WriteString("  G-->>C: the answer as received\n")
	return fence(b.String())
}

// gatewayDependencies are the dependencies a gateway forwards to, then
// those it calls besides, each once.
func gatewayDependencies(g *yaml.Node) []string {
	var out []string
	for _, row := range items(g, "forwards") {
		if dep := str(row, "dependency"); dep != "" && !contains(out, dep) {
			out = append(out, dep)
		}
	}
	for _, c := range calls(g) {
		if dep := calledDependency(c); !contains(out, dep) {
			out = append(out, dep)
		}
	}
	return out
}
