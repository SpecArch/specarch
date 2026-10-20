package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// checkQuestions checks the open questions: who decides is a stakeholder,
// every block names a stage, a section or an element of the specification,
// a question sits in the stage of what it blocks, and no accepted decision
// answers a question that is still open.
func (c *checker) checkQuestions(d *design) {
	for id, q := range d.questions {
		ptr := source.Pointer("questions", id)
		if by := source.Child(q, "decidedBy"); by != nil && by.Value != "" && d.stakeholders[by.Value] == nil {
			c.add(by, ptr+"/decidedBy", RuleStakeholder, "%s is not a stakeholder of the specification%s", by.Value, suggest(by.Value, d.stakeholders))
		}
		stages := map[string]bool{}
		var stageList []string
		blocks := source.Items(source.Child(q, "blocks"))
		for i, item := range blocks {
			b, ok := c.checkBlock(d, item, fmt.Sprintf("%s/blocks/%d", ptr, i))
			if !ok {
				continue
			}
			if names := source.Child(q, "names"); names != nil && len(blocks) == 1 {
				c.checkNames(d, id, names, b)
			}
			if !stages[b.Stage] {
				stages[b.Stage] = true
				stageList = append(stageList, b.Stage)
			}
		}
		if names := source.Child(q, "names"); names != nil && len(blocks) != 1 {
			c.add(names, ptr+"/names", RuleQuestionBlock, "question %s names %s for one key, and blocks %d entries; block only the key the name is for, such as #/workflows/<name>/trigger", id, names.Value, len(blocks))
		}
		key := source.Key(source.Child(d.root, "questions"), id)
		switch {
		case len(stageList) > 1:
			c.add(key, ptr, RuleQuestionStage, "question %s blocks the %s stages; a question is about one stage, so split it", id, joinAnd(stageList))
		case len(stageList) == 1 && d.spec != nil:
			want := stageList[0]
			at := d.spec.StageOfFile(c.fileOf(key))
			switch {
			case at == "" && d.spec.Listed(want):
				c.add(key, ptr, RuleQuestionStage, "question %s is about the %s stage, which has a folder; move it to %s/", id, want, want)
			case at != "" && at != want:
				c.add(key, ptr, RuleQuestionStage, "question %s sits under %s/ but is about the %s stage; move it to %s/", id, at, want, want)
			}
		}
	}
	for id, dec := range d.decisions {
		if source.Str(source.Child(dec, "status")) != "accepted" {
			continue
		}
		for i, a := range source.Items(source.Child(dec, "answers")) {
			if d.questions[a.Value] != nil {
				c.add(a, source.Pointer("decisions", id, "answers", fmt.Sprint(i)), RuleQuestionAnswered,
					"%s answers question %s, but %s is still open; remove the question now that it is answered, or set the decision's status to proposed", id, a.Value, a.Value)
			}
		}
	}
	// A decision opens questions that are open now or that a decision has
	// answered since, whatever its status; a question it answers itself is
	// not one it opened.
	answered := map[string]bool{}
	for _, dec := range d.decisions {
		for _, a := range source.Items(source.Child(dec, "answers")) {
			answered[a.Value] = true
		}
	}
	for id, dec := range d.decisions {
		own := map[string]bool{}
		for _, a := range source.Items(source.Child(dec, "answers")) {
			own[a.Value] = true
		}
		for i, o := range source.Items(source.Child(dec, "opens")) {
			ptr := source.Pointer("decisions", id, "opens", fmt.Sprint(i))
			switch {
			case own[o.Value]:
				c.add(o, ptr, RuleQuestionOpened, "%s both answers and opens question %s; a decision opens only what it leaves open, so name the open half as a question of its own", id, o.Value)
			case d.questions[o.Value] == nil && !answered[o.Value]:
				c.add(o, ptr, RuleQuestionOpened, "%s opens question %s, which is neither an open question of the specification nor answered by a decision; write the question, or correct the ID%s", id, o.Value, suggest(o.Value, d.questions))
			}
		}
	}
}

// checkBlock checks one entry of a question's blocks and returns it parsed.
func (c *checker) checkBlock(d *design, item *yaml.Node, ptr string) (spec.Block, bool) {
	text := item.Value
	b, ok := spec.ParseBlock(text)
	if !ok {
		switch {
		case strings.HasPrefix(text, "#/"):
			c.add(item, ptr, RuleQuestionBlock, "%s does not point into a section; a pointer is #/<section>/<name>, such as #/entities/Loan, with at most one key after them", text)
		default:
			c.add(item, ptr, RuleQuestionBlock, "%s is neither a stage, a section nor a #/ pointer to an element; name the stage (%s), a section, or #/<section>/<name>", text, strings.Join(spec.Stages, ", "))
		}
		return b, false
	}
	if !b.IsPointer() {
		return b, true
	}
	section := source.Child(d.root, b.Tokens[0])
	element := source.Child(section, b.Tokens[1])
	if element == nil {
		fix := suggest(b.Tokens[1], topMap(d.root, b.Tokens[0]))
		if !strings.HasPrefix(fix, "; did you mean") {
			fix = fmt.Sprintf("; write the element, down to an empty mapping when only its name is known, or block the section %s", b.Tokens[0])
		}
		c.add(item, ptr, RuleQuestionBlock, "%s does not point at anything in the specification%s", text, fix)
		return b, false
	}
	n := element
	for i, t := range b.Tokens[2:] {
		next := source.Child(n, t)
		if next == nil {
			if i == len(b.Tokens)-3 {
				return b, true // the one missing key the question is about
			}
			c.add(item, ptr, RuleQuestionBlock, "%s points below a key that does not exist; name the element, or one missing key of it", text)
			return b, false
		}
		n = next
	}
	return b, true
}

// checkNames checks that a question naming what the source gives blocks one
// key, and that the specification leaves that key out.
func (c *checker) checkNames(d *design, id string, names *yaml.Node, b spec.Block) {
	ptr := source.Pointer("questions", id, "names")
	if b.Key() == "" {
		c.add(names, ptr, RuleQuestionBlock, "question %s names %s for one key, and blocks %s, which is not a key; block the key the name is for, such as #/workflows/<name>/trigger", id, names.Value, "#"+source.Pointer(b.Tokens...))
		return
	}
	if _, given := source.Resolve(d.root, b.Tokens); given {
		c.add(names, ptr, RuleQuestionBlock, "question %s names %s for %s, which the specification gives already; leave the key out until the name resolves, or remove the question", id, names.Value, "#"+source.Pointer(b.Tokens...))
	}
}

func joinAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

// checkOrigin checks every element that says how it is known: stated
// cites something, inferred says why, decided names an accepted decision,
// and decidedIn goes only with decided. decisions are the decision records
// a decidedIn may name; isDecision tells whether a path is a decision
// record itself, which nothing decides.
func (c *checker) checkOrigin(root *yaml.Node, decisions map[string]*yaml.Node, isDecision func(path []string) bool) {
	walk(root, nil, func(n *yaml.Node, path []string) {
		origin := source.Child(n, "origin")
		decidedIn := source.Child(n, "decidedIn")
		if origin == nil && decidedIn == nil {
			return
		}
		ptr := source.Pointer(path...)
		kind := source.Str(origin)
		switch kind {
		case "stated":
			if len(source.Items(source.Child(n, "cites"))) == 0 {
				c.add(origin, ptr+"/origin", RuleOriginCitation, "origin is stated but nothing is cited; add cites with the source and where in it this is stated")
			}
		case "inferred":
			if strings.TrimSpace(source.Str(source.Child(n, "why"))) == "" {
				c.add(origin, ptr+"/origin", RuleOriginReason, "origin is inferred but why is missing; say from what evidence this was concluded")
			}
		case "decided":
			switch {
			case isDecision(path):
				c.add(origin, ptr+"/origin", RuleOriginDecision, "a decision is not decided by another decision; set supersededBy on the one it replaces, or state or infer it")
			case decidedIn == nil:
				c.add(origin, ptr+"/origin", RuleOriginDecision, "origin is decided but decidedIn is missing; name the decision that settled this")
			case decisions[decidedIn.Value] == nil:
				c.add(decidedIn, ptr+"/decidedIn", RuleOriginDecision, "%s is not a decision of the specification%s", decidedIn.Value, suggest(decidedIn.Value, decisions))
			case source.Str(source.Child(decisions[decidedIn.Value], "status")) != "accepted":
				c.add(decidedIn, ptr+"/decidedIn", RuleOriginDecision, "%s is %s, not accepted; an element rests only on an accepted decision", decidedIn.Value, source.Str(source.Child(decisions[decidedIn.Value], "status")))
			}
		}
		if decidedIn != nil && kind != "decided" && kind != "" {
			c.add(decidedIn, ptr+"/decidedIn", RuleOriginDecision, "decidedIn is set but origin is %s; set origin: decided, or remove decidedIn", kind)
		}
		if decidedIn != nil && kind == "" && origin == nil {
			c.add(decidedIn, ptr+"/decidedIn", RuleOriginDecision, "decidedIn is set but origin is missing; set origin: decided, or remove decidedIn")
		}
	})
}

// checkOriginTracked reports, when the root file says the specification
// tracks origin, every element of a section that carries none.
func (c *checker) checkOriginTracked(d *design) {
	if source.Str(source.Child(source.Child(d.root, "info"), "tracksOrigin")) != "true" {
		return
	}
	for _, p := range source.Pairs(d.root) {
		name := p.Key.Value
		if _, isSection := spec.Sections[name]; !isSection {
			continue
		}
		switch name {
		case "release", "rollback", "handover", "signoff", "backup", "restore":
			if source.Child(p.Value, "origin") == nil {
				c.warn(p.Key, source.Pointer(name), RuleOriginMissing, "%s carries no origin, and the specification tracks origin; add origin: stated, inferred or decided", name)
			}
		case "paths":
			for _, o := range d.opList {
				if source.Child(o.node, "origin") == nil {
					c.warn(source.Key(o.pathItem, o.method), o.pointer(), RuleOriginMissing, "operation %s carries no origin, and the specification tracks origin; add origin: stated, inferred or decided", o.id)
				}
			}
		default:
			for _, e := range source.Pairs(p.Value) {
				if source.Deref(e.Value).Kind == yaml.MappingNode && source.Child(e.Value, "origin") == nil {
					c.warn(e.Key, source.Pointer(name, e.Key.Value), RuleOriginMissing, "%s carries no origin, and the specification tracks origin; add origin: stated, inferred or decided", e.Key.Value)
				}
			}
		}
	}
}

// blocked is what the open must questions of a specification hold up by
// pointer: the elements, each the full pointer of a blocks entry that
// resolves, such as /entities/Loan or /entities/Loan/properties/dueOn; and
// the keys, a blocks entry whose last token is missing, by the pointer of
// the mapping it is missing from.
type blocked struct {
	elements []string
	keys     map[string][]string
}

// blockedPointers reads the pointers the must questions block. A pointer
// that resolves blocks the node it points to, whatever it holds: an
// entity, a field with content, a field known only by its name (an empty
// mapping) or one key with a value. A pointer of three tokens or more also
// blocks its last token as a key of the mapping before it, whether the key
// is there (an acceptance list left empty) or missing.
func blockedPointers(root *yaml.Node) blocked {
	b := blocked{keys: map[string][]string{}}
	for _, p := range source.Pairs(source.Child(root, "questions")) {
		if source.Str(source.Child(p.Value, "priority")) != "must" {
			continue
		}
		for _, item := range source.Items(source.Child(p.Value, "blocks")) {
			bl, ok := spec.ParseBlock(item.Value)
			if !ok || !bl.IsPointer() {
				continue
			}
			if _, ok := source.Resolve(root, bl.Tokens); ok {
				b.elements = append(b.elements, source.Pointer(bl.Tokens...))
			}
			if k := bl.Key(); k != "" {
				parent := source.Pointer(bl.Tokens[:len(bl.Tokens)-1]...)
				b.keys[parent] = append(b.keys[parent], k)
			}
		}
	}
	return b
}

// none reports whether no pointer is blocked.
func (b blocked) none() bool { return len(b.elements) == 0 && len(b.keys) == 0 }

// under reports whether the path is a blocked element or inside one.
func (b blocked) under(path string) bool {
	for _, e := range b.elements {
		if path == e || strings.HasPrefix(path, e+"/") {
			return true
		}
	}
	return false
}

// Covered splits the diagnostics into the ones to report and the ones an
// open must question covers: a required key missing at or under a pointer
// the question blocks, the warnings about that element, the problem a
// response must name once problem types are declared when the question
// blocks that problem, a permission no role grants when the question
// blocks that permission, and the one-namespace error at a schema or a
// view named like an entity, or a schema like a view, when the question
// blocks that schema or view. A pointer to an element covers everything missing
// at or under it, whether the element is written out in part, such as a
// field with its width and no type, or known only by name, an empty
// mapping. A pointer to a key that is not there covers that one key. A
// wrong value next to the gap stays an error. root is the merged
// specification.
func Covered(ds []Diagnostic, root *yaml.Node) (kept, covered []Diagnostic) {
	b := blockedPointers(root)
	if b.none() {
		return ds, nil
	}
	for _, d := range ds {
		missing := ""
		if d.Rule == RuleSchema && strings.HasSuffix(d.Message, " is missing; add it here") {
			missing = strings.TrimSuffix(d.Message, " is missing; add it here")
		}
		isCovered := false
		switch {
		case missing != "":
			isCovered = b.under(d.Path) || contains(b.keys[d.Path], missing)
		case d.Severity == Warning:
			isCovered = b.under(d.Path) || d.Rule == RuleAcceptanceMissing && contains(b.keys[d.Path], "acceptance") ||
				d.Rule == RuleMiddlewareMissing && contains(b.keys[d.Path], "middlewares")
		case d.Rule == RuleProblem && strings.HasSuffix(d.Message, " names one under problem"):
			// A response whose problem the source does not name, while
			// the specification declares problem types: the problem is
			// the key missing, and a question on it says it is not known.
			isCovered = b.under(d.Path) || contains(b.keys[d.Path], "problem")
		case (d.Rule == RuleCallers || d.Rule == RuleVia) && (strings.Contains(d.Message, " is neither people nor a client of the specification") || strings.Contains(d.Message, " is not a client of the specification") || strings.Contains(d.Message, " accepts neither a transaction token nor a token declaring act")):
			// A client the code names and the specification does not yet
			// declare, or a via whose token is not yet declared: the
			// question that asks for the declaration covers it.
			isCovered = b.under(d.Path)
		case d.Rule == RuleThrough && strings.HasPrefix(d.Message, "through needs the application the caller signed in through"):
			// A grant narrowed by a table that names no token: the question
			// on which token carries the application covers it.
			isCovered = b.under(d.Path)
		case d.Rule == RuleClient && strings.Contains(d.Message, " and has no proof"):
			// A client a table grants permissions to directly: the
			// question on its proof covers the proof missing.
			isCovered = b.under(d.Path) || contains(b.keys[d.Path], "proof")
		case d.Rule == RulePermissionUngranted:
			// The grant is what is missing, and it is written in a role,
			// not under the permission; a question on the permission
			// itself is the one place that can say it is not known.
			isCovered = b.under(d.Path)
		case (d.Rule == RuleValueObject || d.Rule == RuleView) && strings.Contains(d.Message, " are one namespace"):
			// A schema or a view named like an entity, or a schema like
			// a view: merge writes both, from trees that each pass on
			// their own, and asks whether they are one element; the
			// question on the schema or the view is the one place that
			// can say it is not known (ADR-132).
			isCovered = b.under(d.Path)
		}
		if isCovered {
			covered = append(covered, d)
		} else {
			kept = append(kept, d)
		}
	}
	return kept, covered
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Questions counts the open questions of a specification by priority:
// must, should and could, in that order.
func Questions(root *yaml.Node) (must, should, could int) {
	for _, p := range source.Pairs(source.Child(root, "questions")) {
		switch source.Str(source.Child(p.Value, "priority")) {
		case "must":
			must++
		case "should":
			should++
		case "could":
			could++
		}
	}
	return
}
