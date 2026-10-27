package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// runDecision prints, for "decision why", the chain of decisions a
// decision rests on, each after the ones it rests on and the decision
// last, and for "decision rests" the decisions that rest on it, nearest
// first; a summary line, then one decision per line (ADR-188). It works
// on a specification with errors too, after printing them (ADR-065), and
// exits 1 then, as it does for an ID that is no decision.
func runDecision(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "why" && args[0] != "rests") {
		fmt.Fprintf(stderr, "specarch decision needs why or rests after it: decision why <decision> <folder> prints the chain a decision rests on, decision rests <decision> <folder> the decisions that rest on it\n\n%s", usage)
		return 2
	}
	way := args[0]
	verb := "decision " + way
	var rest []string
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			fmt.Fprintf(stderr, "specarch %s has no option %s; it takes the decision's ID and one folder\n\n%s", verb, a, usage)
			return 2
		}
		rest = append(rest, a)
	}
	if len(rest) != 2 {
		fmt.Fprintf(stderr, "specarch %s needs the decision's ID and one folder, in that order\n\n%s", verb, usage)
		return 2
	}
	id, folder := rest[0], rest[1]
	specs, invalid, status := readSpecs([]string{folder}, verb, stdout, stderr)
	if status != 0 {
		return status
	}
	if len(specs) != 1 {
		fmt.Fprintf(stderr, "specarch %s: %s holds %d specifications; name the folder of the one the decision is in\n", verb, folder, len(specs))
		return 2
	}
	l := specs[0]
	root := l.spec.Root
	if root == nil {
		fmt.Fprintf(stderr, "specarch %s: nothing of %s could be read, so no decision can be found; fix the errors above first\n", verb, l.spec.RootFile)
		return 1
	}
	decisions := source.Child(root, "decisions")
	if source.Child(decisions, id) == nil {
		var ids []string
		for _, p := range source.Pairs(decisions) {
			ids = append(ids, p.Key.Value)
		}
		sort.Strings(ids)
		have := "it has no decision"
		if len(ids) > 0 {
			have = "its decisions are " + strings.Join(ids, ", ")
		}
		fmt.Fprintf(stderr, "specarch %s: %s has no decision %s; %s\n", verb, l.spec.Dir, id, have)
		return 1
	}
	var chain []string
	if way == "why" {
		chain = restsOnChain(decisions, id)
		switch n := len(chain) - 1; n {
		case 0:
			fmt.Fprintf(stdout, "%s rests on no decision\n", id)
		case 1:
			fmt.Fprintf(stdout, "%s rests on 1 decision, earliest first\n", id)
		default:
			fmt.Fprintf(stdout, "%s rests on %d decisions, earliest first\n", id, n)
		}
	} else {
		chain = restingChain(decisions, id)
		switch n := len(chain) - 1; n {
		case 0:
			fmt.Fprintf(stdout, "no decision rests on %s\n", id)
		case 1:
			fmt.Fprintf(stdout, "1 decision rests on %s, nearest first\n", id)
		default:
			fmt.Fprintf(stdout, "%d decisions rest on %s, nearest first\n", n, id)
		}
	}
	ideas := ideasByDecision(l.spec.Records)
	for _, d := range chain {
		line := decisionLine(d, source.Child(decisions, d))
		if from := ideas[d]; len(from) > 0 {
			line = strings.TrimSuffix(line, ")") + "; from idea " + strings.Join(from, ", ") + ")"
		}
		fmt.Fprintln(stdout, line)
	}
	if invalid {
		return 1
	}
	return 0
}

// ideasByDecision maps each decision to the ideas in records/ideas/ that
// became it, in ID order, IDEA-2 before IDEA-10. A decision never names the idea
// it came from, since the specification never points at its records, so
// the link is read from the idea's became.
func ideasByDecision(records []spec.Record) map[string][]string {
	out := map[string][]string{}
	for _, r := range records {
		root := source.Parse(r.Data).Root
		if source.Str(source.Child(root, "kind")) != "idea" {
			continue
		}
		id := source.Str(source.Child(root, "id"))
		if d := source.Str(source.Child(source.Child(root, "became"), "decision")); d != "" && id != "" {
			out[d] = append(out[d], id)
		}
	}
	for _, ids := range out {
		sort.SliceStable(ids, func(i, j int) bool { return ideaIDLess(ids[i], ids[j]) })
	}
	return out
}

// ideaIDLess orders IDs by prefix, then by number when both have one.
func ideaIDLess(a, b string) bool {
	pa, na, _ := strings.Cut(a, "-")
	pb, nb, _ := strings.Cut(b, "-")
	if pa != pb {
		return pa < pb
	}
	x, errA := strconv.Atoi(na)
	y, errB := strconv.Atoi(nb)
	if errA == nil && errB == nil {
		return x < y
	}
	return na < nb
}

// restsOnChain is the chain a decision rests on: every decision reached
// through restsOn, each after the ones it rests on, in the order the
// links are written, each once where it is first reached, and the
// decision itself last. A link to a decision that is not there, which
// validate reports, is passed over.
func restsOnChain(decisions *yaml.Node, id string) []string {
	var out []string
	seen := map[string]bool{}
	var walk func(string)
	walk = func(d string) {
		dec := source.Child(decisions, d)
		if seen[d] || dec == nil {
			return
		}
		seen[d] = true
		for _, link := range source.Items(source.Child(dec, "restsOn")) {
			walk(link.Value)
		}
		out = append(out, d)
	}
	walk(id)
	return out
}

// restingChain is the decision and the decisions that rest on it, the
// ones resting on it directly first, then the ones resting on those, each
// once where it is first reached, in the order the decisions are written.
func restingChain(decisions *yaml.Node, id string) []string {
	out := []string{id}
	seen := map[string]bool{id: true}
	for front := 0; front < len(out); front++ {
		for _, p := range source.Pairs(decisions) {
			if seen[p.Key.Value] {
				continue
			}
			for _, link := range source.Items(source.Child(p.Value, "restsOn")) {
				if link.Value == out[front] {
					seen[p.Key.Value] = true
					out = append(out, p.Key.Value)
					break
				}
			}
		}
	}
	return out
}

// decisionLine is one decision as the chain prints it: its ID, its title,
// its status with the decision that supersedes it, its date, and the
// decisions it rests on.
func decisionLine(id string, dec *yaml.Node) string {
	status := source.Str(source.Child(dec, "status"))
	if by := source.Str(source.Child(dec, "supersededBy")); by != "" {
		status += " by " + by
	}
	about := status + ", " + source.Str(source.Child(dec, "date"))
	var links []string
	for _, link := range source.Items(source.Child(dec, "restsOn")) {
		links = append(links, link.Value)
	}
	if len(links) > 0 {
		about += "; rests on " + strings.Join(links, ", ")
	}
	return fmt.Sprintf("%s: %s (%s)", id, oneLine(source.Str(source.Child(dec, "title"))), about)
}
