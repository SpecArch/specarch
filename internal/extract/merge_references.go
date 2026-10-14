package extract

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// A reference and a shape written in place at one position (ADR-116). An
// OpenAPI document names a request body's schema, a response's or a
// property's by reference, and the code's handler decodes the body into a
// struct, which its reader writes in place. The two forms share no key, so
// joining them key by key would write both, which the design refuses. The
// merge keeps the position as it was first given while the trees are read,
// then, once every tree is in, reads the reference's target among the
// merged entries and compares the shapes: where they agree in all both
// say, the reference is kept, the one name for the shape; where a key both
// give differs, the key is left out as a disagreement; where they agree
// and only list different properties, the reference is kept and one should
// question says what each lists. A question a tree asked below the
// position follows the reference to the target.

// references is the merger's state for such positions.
type references struct {
	positions map[string][]mergeStep        // stepsKey -> the steps of a position with both forms
	order     []string                      // the positions in the order they were met
	resolved  map[string]*resolvedReference // merged pointer of the position -> how it was resolved
	ptrOrder  []string                      // the resolved pointers in order
	disputed  map[string]string             // stepsKey of a position left out -> how the shapes differ
}

// resolvedReference is one position after every tree is merged.
type resolvedReference struct {
	steps    []mergeStep // the position
	target   []mergeStep // the entry the reference names: its section and name
	ptr      string      // the position's pointer in the merged specification
	refTrees []int       // the trees that give the reference
	kept     bool        // the reference is kept; false when the key is left out
	drift    []string    // what each side lists and the other does not, when kept
}

// twoForms reports a position where one node refers to a named shape and
// the other writes a shape in place.
func twoForms(a, b *yaml.Node) bool {
	ra, rb := source.Child(a, "$ref") != nil, source.Child(b, "$ref") != nil
	if ra == rb {
		return false
	}
	in := b
	if rb {
		in = a
	}
	return typeOf(in) != "" || source.Child(in, "properties") != nil || source.Child(in, "items") != nil
}

// recordReference notes a position with both forms, to resolve once every
// tree is merged.
func (m *merger) recordReference(here []mergeStep) {
	if m.refs.positions == nil {
		m.refs.positions = map[string][]mergeStep{}
	}
	k := stepsKey(here)
	if m.refs.positions[k] == nil {
		m.refs.positions[k] = here
		m.refs.order = append(m.refs.order, k)
	}
}

// referencedEntry is the merged entry a reference names, with its steps,
// or nil for a reference to anything else.
func (m *merger) referencedEntry(ref *yaml.Node) ([]mergeStep, *yaml.Node) {
	value := source.Str(source.Child(ref, "$ref"))
	for _, section := range []string{"schemas", "entities", "enums"} {
		if name, ok := strings.CutPrefix(value, "#/"+section+"/"); ok && !strings.Contains(name, "/") {
			name = source.UnescapeToken(name)
			return []mergeStep{{key: section}, {key: name}}, m.entries[section+"/"+name]
		}
	}
	return nil, nil
}

// resolveReferences settles every position with both forms.
func (m *merger) resolveReferences() {
	for _, k := range m.refs.order {
		steps := m.refs.positions[k]
		acc := m.walk(steps)
		if acc == nil {
			continue
		}
		var refNode *yaml.Node
		var refTrees, inlineTrees []int
		var inlines []*yaml.Node
		for ti, t := range m.trees {
			n := follow(t.s.Root, steps)
			switch {
			case n == nil:
			case source.Child(n, "$ref") != nil:
				if refNode == nil {
					refNode = n
				}
				refTrees = append(refTrees, ti)
			default:
				inlineTrees = append(inlineTrees, ti)
				inlines = append(inlines, n)
			}
		}
		if refNode == nil || len(inlines) == 0 {
			continue
		}
		ptr := m.pointerOf(steps)
		target, entry := m.referencedEntry(refNode)
		r := &resolvedReference{steps: steps, target: target, ptr: ptr, refTrees: refTrees, kept: true}
		if m.refs.resolved == nil {
			m.refs.resolved = map[string]*resolvedReference{}
		}
		m.refs.resolved[ptr] = r
		m.refs.ptrOrder = append(m.refs.ptrOrder, ptr)
		refTitles, inlineTitles := m.titlesOf(refTrees), m.titlesOf(inlineTrees)
		named := source.Str(source.Child(refNode, "$ref"))
		if entry == nil {
			// Every tree passed validate, so a reference names an entry a
			// tree has; one to anything else is kept as it is.
			keepReference(acc, refNode)
			continue
		}
		var contradictions, drift []string
		for i, in := range inlines {
			c, d := m.shapeDiffs(entry, in, target[1].key, m.trees[inlineTrees[i]].title, map[string]bool{target[1].key: true})
			contradictions = appendNew(contradictions, c...)
			drift = appendNew(drift, d...)
		}
		if len(contradictions) > 0 {
			if m.refs.disputed == nil {
				m.refs.disputed = map[string]string{}
			}
			m.refs.disputed[k] = joinAnd(contradictions)
			m.disputes[k] = steps
			m.disputeOrd = append(m.disputeOrd, k)
			r.kept = false
			m.res.say("disagree: on %s, %s %s to %s and %s %s the shape in place, and they differ: %s; the key is left out", ptr, joinAnd(refTitles), referOrRefers(len(refTitles)), named, joinAnd(inlineTitles), writeOrWrites(len(inlineTitles)), joinAnd(contradictions))
			continue
		}
		keepReference(acc, refNode)
		r.drift = drift
		line := fmt.Sprintf("joined: on %s, %s %s to %s and %s %s the shape in place; the shapes agree, and the reference is kept", ptr, joinAnd(refTitles), referOrRefers(len(refTitles)), named, joinAnd(inlineTitles), writeOrWrites(len(inlineTitles)))
		if len(drift) > 0 {
			line += " with a question on what each lists"
		}
		m.res.say("%s", line)
	}
}

// keepReference writes the reference over the merged node when it holds the
// shape in place.
func keepReference(acc, ref *yaml.Node) {
	if source.Child(acc, "$ref") != nil {
		return
	}
	*acc = *copyNode(ref)
}

func (m *merger) titlesOf(trees []int) []string {
	var out []string
	for _, ti := range trees {
		out = append(out, m.trees[ti].title)
	}
	return out
}

func appendNew(list []string, items ...string) []string {
	for _, s := range items {
		if !contains(list, s) {
			list = append(list, s)
		}
	}
	return list
}

func referOrRefers(n int) string {
	if n == 1 {
		return "refers"
	}
	return "refer"
}

func writeOrWrites(n int) string {
	if n == 1 {
		return "writes"
	}
	return "write"
}

// shapeDiffs compares a named shape with one written in place by a tree:
// the keys both give that say what a value is (columnKeys), the required
// list and the enum as sets, the items, and each property both list, each
// a contradiction when it differs. A property only one side lists is
// drift, not a contradiction. A key only one side gives says nothing. where
// names the named shape, with the property path below it; seen holds the
// named shapes above, so one that holds itself ends.
func (m *merger) shapeDiffs(named, in *yaml.Node, where, title string, seen map[string]bool) (contradictions, drift []string) {
	if source.Child(named, "$ref") != nil || source.Child(in, "$ref") != nil {
		switch {
		case source.Child(named, "$ref") != nil && source.Child(in, "$ref") != nil:
			if a, b := source.Str(source.Child(named, "$ref")), source.Str(source.Child(in, "$ref")); a != b {
				contradictions = append(contradictions, fmt.Sprintf("%s refers to %s, and %s to %s", where, a, title, b))
			}
			return
		case source.Child(in, "$ref") != nil:
			named, in = in, named
			fallthrough
		default:
			target, entry := m.referencedEntry(named)
			if entry == nil || seen[target[1].key] {
				return
			}
			next := map[string]bool{target[1].key: true}
			for k := range seen {
				next[k] = true
			}
			return m.shapeDiffs(entry, in, target[1].key, title, next)
		}
	}
	for _, k := range columnKeys {
		av, bv := source.Child(named, k), source.Child(in, k)
		switch {
		case av == nil || bv == nil:
		case k == "type":
			if !sameNode(av, bv) {
				contradictions = append(contradictions, fmt.Sprintf("%s is of type %s, and %s of type %s", where, inline(av), title, inline(bv)))
			}
		case k == "enum":
			if !sameSet(av, bv) {
				contradictions = append(contradictions, fmt.Sprintf("%s has the values %s, and %s %s", where, inline(av), title, inline(bv)))
			}
		case !sameValue(av, bv):
			contradictions = append(contradictions, fmt.Sprintf("%s has %s %s, and %s %s", where, k, inline(av), title, inline(bv)))
		}
	}
	if av, bv := source.Child(named, "required"), source.Child(in, "required"); av != nil && bv != nil && !sameSet(av, bv) {
		contradictions = append(contradictions, fmt.Sprintf("%s requires %s, and %s %s", where, inline(av), title, inline(bv)))
	}
	if av, bv := source.Child(named, "items"), source.Child(in, "items"); av != nil && bv != nil && av.Kind == yaml.MappingNode && bv.Kind == yaml.MappingNode {
		c, d := m.shapeDiffs(av, bv, "the items of "+where, title, seen)
		contradictions = append(contradictions, c...)
		drift = append(drift, d...)
	}
	ap, bp := source.Child(named, "properties"), source.Child(in, "properties")
	if ap == nil || bp == nil {
		return
	}
	var onlyNamed, onlyIn []string
	for _, p := range source.Pairs(ap) {
		if q := source.Child(bp, p.Key.Value); q != nil {
			c, d := m.shapeDiffs(p.Value, q, where+"."+p.Key.Value, title, seen)
			contradictions = append(contradictions, c...)
			drift = append(drift, d...)
		} else {
			onlyNamed = append(onlyNamed, p.Key.Value)
		}
	}
	for _, p := range source.Pairs(bp) {
		if source.Child(ap, p.Key.Value) == nil {
			onlyIn = append(onlyIn, p.Key.Value)
		}
	}
	if len(onlyNamed) > 0 {
		drift = append(drift, fmt.Sprintf("%s lists %s, which %s does not write", where, propertyList(onlyNamed), title))
	}
	if len(onlyIn) > 0 {
		drift = append(drift, fmt.Sprintf("%s writes %s, which %s does not list", title, propertyList(onlyIn), where))
	}
	return
}

func propertyList(names []string) string {
	if len(names) == 1 {
		return "the property " + names[0]
	}
	return "the properties " + joinAnd(names)
}

// sameSet compares two lists of scalars as sets.
func sameSet(a, b *yaml.Node) bool {
	as, bs := scalarList(a), scalarList(b)
	sort.Strings(as)
	sort.Strings(bs)
	return strings.Join(as, "\x00") == strings.Join(bs, "\x00")
}

// movedSteps follows a tree's question from below a resolved position to
// where the key is now: under the named entry where the reference is
// kept, and the position itself where the key is left out.
func (m *merger) movedSteps(steps []mergeStep) []mergeStep {
	if len(m.refs.resolved) == 0 {
		return steps
	}
	ptr := m.pointerOf(steps)
	for _, at := range m.refs.ptrOrder {
		r := m.refs.resolved[at]
		if !strings.HasPrefix(ptr, r.ptr+"/") {
			continue
		}
		if !r.kept || r.target == nil {
			return r.steps
		}
		return append(append([]mergeStep{}, r.target...), steps[len(r.steps):]...)
	}
	return steps
}

// answeredByReference names the trees whose reference answers a question
// a tree asked at a position the reference is kept at: the named shape is
// the shape.
func (m *merger) answeredByReference(ti int, steps []mergeStep) []string {
	r := m.refs.resolved[m.pointerOf(steps)]
	if r == nil || !r.kept || slicesContains(r.refTrees, ti) {
		return nil
	}
	return m.titlesOf(r.refTrees)
}

// referenceQuestions asks one should question per position where the
// reference is kept and the sides list different properties.
func (m *merger) referenceQuestions() int {
	asked := 0
	for _, at := range m.refs.ptrOrder {
		r := m.refs.resolved[at]
		if !r.kept || len(r.drift) == 0 {
			continue
		}
		kind := map[string]string{"schemas": "schema", "entities": "entity", "enums": "enum"}[r.target[0].key]
		cites := &yaml.Node{Kind: yaml.SequenceNode}
		for _, t := range m.trees {
			if follow(t.s.Root, r.steps) == nil {
				continue
			}
			for _, c := range citesAt(t.s.Root, r.steps) {
				if !containsNode(cites, c) {
					cites.Content = append(cites.Content, copyNode(c))
				}
			}
		}
		q := mapping(
			"question", fmt.Sprintf("Is %s the %s %s, as %s %s it? The shapes agree where both speak, and %s.", r.ptr, kind, r.target[1].key, joinAnd(m.titlesOf(r.refTrees)), giveOrGives(len(r.refTrees)), joinAnd(r.drift)),
			"kind", "decision",
			"priority", "should",
			"blocks", []string{r.ptr},
			"decidedBy", owner,
			"why", "A reference is the one name for a shape, and is kept where the shape written in place agrees with it in all both say. A property only one side lists is no disagreement, and the merge neither writes it into the named shape nor leaves it out unsaid.",
		)
		if len(cites.Content) > 0 {
			set(q, "cites", cites)
		}
		m.addQuestion(q, questionFile(q))
		m.res.say("question Q-%d (should): %s: the shapes list different properties", len(m.questions), r.ptr)
		asked++
	}
	return asked
}
