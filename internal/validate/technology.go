package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// bindingNeeds says, per interface kind of a binding, what in the design
// needs it: the sections whose elements the binding's framework serves.
// A design with none of them needs no binding of that kind (ADR-231).
var bindingNeeds = map[string]string{
	"http":      "no paths, routes or gateways",
	"messaging": "no channels and no dependency of kind messageBroker",
	"cli":       "no commands",
	"ui":        "no pages",
	"storage":   "no entities or views",
}

// bindingNeeded reports whether the design has an element a binding of
// the kind serves.
func bindingNeeded(d *design, kind string) bool {
	switch kind {
	case "http":
		return len(d.opList) > 0 || len(source.Pairs(source.Child(d.root, "routes"))) > 0 || len(source.Pairs(source.Child(d.root, "gateways"))) > 0
	case "messaging":
		if len(d.channels) > 0 {
			return true
		}
		for _, dep := range d.dependencies {
			if source.Str(source.Child(dep, "kind")) == "messageBroker" {
				return true
			}
		}
		return false
	case "cli":
		return len(d.commands) > 0
	case "ui":
		return len(d.pages) > 0
	case "storage":
		return len(d.entities) > 0 || len(d.views) > 0
	}
	return true
}

// checkTechnologies warns at a binding whose framework the design does
// not need: no element of its kind, no idiom of the file's stacks
// approving the framework, and no decision that justifies it against a
// simpler alternative (ADR-231). approved maps each approved name to the
// idiom that approves it; decisions are the design's and the file's. A
// binding on the standard library adopts nothing and is left alone, and
// a decidedIn naming no decision is the origin check's error already.
func (c *checker) checkTechnologies(d *design, decisions map[string]*yaml.Node, approved map[string]string) {
	for _, b := range source.Pairs(source.Child(c.root, "bindings")) {
		kind := b.Key.Value
		framework := source.Str(source.Child(b.Value, "framework"))
		lack, known := bindingNeeds[kind]
		if !known || framework == "" || framework == "standard library" || bindingNeeded(d, kind) {
			continue
		}
		if _, ok := approved[framework]; ok {
			continue
		}
		ptr := source.Pointer("bindings", kind)
		lead := fmt.Sprintf("nothing in the specification needs a %s binding: it has %s, and ", kind, lack)
		decidedIn := source.Child(b.Value, "decidedIn")
		if decidedIn == nil {
			c.warn(b.Key, ptr, RuleTechnologyUnjustified, "%sno decision justifies %s against a simpler alternative; add the design elements that need it, or set origin decided and decidedIn naming an accepted decision that satisfies a requirement and names under alternatives the simpler option it rejects, or approve it under technologies in an idiom of this file's stacks", lead, framework)
			continue
		}
		dec := decisions[decidedIn.Value]
		if dec == nil {
			continue
		}
		var missing []string
		if len(source.Items(source.Child(dec, "satisfies"))) == 0 {
			missing = append(missing, "no requirement under satisfies")
		}
		if len(source.Items(source.Child(dec, "alternatives"))) == 0 {
			missing = append(missing, "no simpler alternative under alternatives")
		}
		if len(missing) == 0 {
			continue
		}
		c.warn(decidedIn, ptr+"/decidedIn", RuleTechnologyUnjustified, "%s%s, which decides %s, names %s; add them, so the decision says what %s is for and which simpler option it rejects", lead, decidedIn.Value, framework, strings.Join(missing, " and "), framework)
	}
}

// approvedTechnologies reads the technologies the idioms approve for an
// implementation file: every name under technologies of an idiom that
// renders one of the file's stacks, whatever the design reads, mapped to
// the idiom's name (ADR-231).
func approvedTechnologies(apply map[string]Idiom, isStack map[string]bool) map[string]string {
	out := map[string]string{}
	for _, name := range sortedIdiomNames(apply) {
		i := apply[name]
		renders := false
		for _, st := range source.Items(source.Child(i.Root, "stacks")) {
			if isStack[st.Value] {
				renders = true
			}
		}
		if !renders {
			continue
		}
		for _, t := range source.Pairs(source.Child(i.Root, "technologies")) {
			for _, n := range source.Items(source.Child(t.Value, "names")) {
				if _, taken := out[n.Value]; !taken {
					out[n.Value] = name
				}
			}
		}
	}
	return out
}
