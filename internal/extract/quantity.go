package extract

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/source"
)

// A commitment that states a number of days, and the words before its
// verb, which name what it is about: "The loan period is 21 days."
var (
	daysQuantity = regexp.MustCompile(`(?i)\b([0-9]+|` + numberWords + `)\s+days?\b`)
	subjectVerb  = regexp.MustCompile(`(?i)^(.*?)\s+(is|are|shall|must|will|should)\b`)
	stopWords    = map[string]bool{"the": true, "a": true, "an": true, "of": true, "each": true, "every": true}
	numberValues = map[string]int{"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9,
		"ten": 10, "eleven": 11, "twelve": 12, "fifteen": 15, "twenty": 20, "thirty": 30, "forty": 40, "fifty": 50, "sixty": 60}
)

// daysCommitment is the subject's words and the number of days of a
// requirement statement, when it states exactly one number of days.
func daysCommitment(statement string) ([]string, int, bool) {
	ms := daysQuantity.FindAllStringSubmatch(statement, -1)
	sv := subjectVerb.FindStringSubmatch(statement)
	if len(ms) != 1 || sv == nil {
		return nil, 0, false
	}
	n, err := strconv.Atoi(ms[0][1])
	if err != nil {
		v, ok := numberValues[strings.ToLower(ms[0][1])]
		if !ok {
			return nil, 0, false
		}
		n = v
	}
	var subject []string
	for _, w := range words(sv[1]) {
		if !stopWords[w] {
			subject = append(subject, w)
		}
	}
	return subject, n, len(subject) > 0
}

// checkDays is the one number of whole days a check moves a date by, when
// it moves dates by exactly one.
func checkDays(expression string) (int, bool) {
	n, errs := expr.Parse(expression)
	if len(errs) > 0 || n == nil {
		return 0, false
	}
	found := map[int]bool{}
	var walk func(*expr.Node)
	walk = func(x *expr.Node) {
		if x == nil {
			return
		}
		if x.Op == expr.OpCall && x.Text == "duration" && len(x.Args) == 1 && x.Args[0].Op == expr.OpText {
			if d, ok := expr.ParseDuration(x.Args[0].Text); ok && d%(24*time.Hour) == 0 {
				found[int(d/(24*time.Hour))] = true
			}
		}
		for _, a := range x.Args {
			walk(a)
		}
	}
	walk(n)
	if len(found) != 1 {
		return 0, false
	}
	for d := range found {
		return d, true
	}
	return 0, false
}

// check is a check constraint of an entity the code side read.
type dayCheck struct {
	entity, name string
	node         *yaml.Node
	days         int
	cites        []*yaml.Node
}

// quantityQuestions compares a requirement of the documents side that
// states a number of days with the one check of the code side whose name
// holds every word of the requirement's subject and that moves a date by
// whole days (ADR-048). The check satisfies the requirement; a different
// number leaves the statement out with one must question citing both.
func (m *merger) quantityQuestions() int {
	var checks []dayCheck
	for _, ptr := range m.order {
		e := m.elements[ptr]
		if e.section != "entities" || e.parent != "" || !m.onSide(e, codeSide) {
			continue
		}
		n := m.elementNode(e)
		for _, c := range source.Pairs(source.Child(n, "constraints")) {
			if source.Str(source.Child(c.Value, "kind")) != "check" {
				continue
			}
			if d, ok := checkDays(source.Str(source.Child(c.Value, "expression"))); ok {
				checks = append(checks, dayCheck{entity: e.tokens[1], name: c.Key.Value, node: c.Value, days: d, cites: source.Items(source.Child(n, "cites"))})
			}
		}
	}
	asked := 0
	for _, ptr := range m.order {
		e := m.elements[ptr]
		if e.section != "requirements" || !m.onSide(e, documentsSide) {
			continue
		}
		req := m.elementNode(e)
		statement := source.Str(source.Child(req, "statement"))
		subject, days, ok := daysCommitment(statement)
		if !ok {
			continue
		}
		var match []dayCheck
		for _, c := range checks {
			if holdsAll(words(c.name), subject) {
				match = append(match, c)
			}
		}
		if len(match) != 1 {
			continue
		}
		c := match[0]
		id := e.tokens[1]
		setKey(c.node, "satisfies", value([]string{id}))
		if c.days == days {
			m.res.say("joined: %s and the check %s of %s both say %d days", id, c.name, c.entity, days)
			continue
		}
		reqPtr := "#" + source.Pointer("requirements", id, "statement")
		expression := source.Str(source.Child(c.node, "expression"))
		deleteKey(req, "statement")
		cites := &yaml.Node{Kind: yaml.SequenceNode}
		for _, x := range append(source.Items(source.Child(req, "cites")), c.cites...) {
			if !containsNode(cites, x) {
				cites.Content = append(cites.Content, copyNode(x))
			}
		}
		q := mapping(
			"question", fmt.Sprintf("Is it %d days, as %s says (%q), or %d days, as the check %s of %s has it (%s)?", days, id, statement, c.days, c.name, c.entity, expression),
			"kind", "decision",
			"priority", "must",
			"blocks", []string{reqPtr},
			"decidedBy", owner,
			"options", []string{fmt.Sprintf("%d days: the code is to change", days), fmt.Sprintf("%d days: the document is to change", c.days)},
			"why", fmt.Sprintf("The documents and the code disagree: the document says %d days, the code checks %d. The merge leaves the statement out rather than choose, and keeps the check the code runs, which satisfies the requirement whichever way it is answered.", days, c.days),
			"cites", cites,
		)
		m.addQuestion(q, questionFile(q))
		m.res.say("question Q-%d (must): %s: the document says %d days, the check %s of %s says %d", len(m.questions), reqPtr, days, c.name, c.entity, c.days)
		asked++
	}
	return asked
}

// onSide says whether a tree of that side has the element.
func (m *merger) onSide(e *element, side string) bool {
	for _, ti := range e.trees {
		if m.trees[ti].side == side {
			return true
		}
	}
	return false
}

func holdsAll(have, want []string) bool {
	set := map[string]bool{}
	for _, w := range have {
		set[w] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}
