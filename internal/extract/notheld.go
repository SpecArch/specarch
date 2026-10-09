package extract

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// notHeld is one thing a reader read that the meta-model cannot hold as the
// source writes it. It becomes a could question in the tree, citing where
// the source says it, so that it is a problem like any other and reaches
// the problems file (ADR-065, docs/diagnostics.md step 5); and a line on
// standard output naming that question.
type notHeld struct {
	text   string   // what is not held, and what the reader did instead
	clause string   // where the source says it: path:line when the reader knows the line
	blocks []string // the entry it is about, or the section it would be in
	// asked is a pointer that a question the reader asks blocks, when that
	// question already asks for what is not held; the line then names that
	// question, and no could question is written. When no question blocks
	// the pointer, a could question is written as for any other.
	asked string
}

// notHeldWhy is the why of every could question a reader asks for what it
// could not hold.
const notHeldWhy = "The source says it, and the meta-model cannot hold it as the source writes it, so the reader left it out or wrote it as the question says. Nothing waits for the answer."

// questionsFor is the questions a reader writes a question into, by what
// the question blocks: one mapping for most readers, one per stage for a
// reader that writes more than one.
type questionsFor func(blocks []string) *yaml.Node

func oneQuestions(q *yaml.Node) questionsFor {
	return func([]string) *yaml.Node { return q }
}

// askNotHeld writes each thing not held as a could question after the
// questions the reader asked, numbered on from *next, and prints its line.
func askNotHeld(res *Result, into questionsFor, next *int, key string, items []notHeld) {
	for _, h := range items {
		questions := into(h.blocks)
		if id, priority := askedBy(questions, h.asked); id != "" {
			res.say("not held: %s; %s question %s", h.text, priority, id)
			continue
		}
		*next++
		id := fmt.Sprintf("Q-%d", *next)
		set(questions, id, mapping(
			"question", notHeldQuestion(h.text),
			"kind", "decision",
			"priority", "could",
			"blocks", h.blocks,
			"decidedBy", owner,
			"why", notHeldWhy,
			"cites", []*yaml.Node{citation(key, h.clause, notHeldSays(strings.TrimPrefix(h.text, h.clause+": ")))},
		))
		res.say("not held: %s; could question %s", h.text, id)
	}
}

// notHeldQuestion asks whether the specification needs what was not held.
func notHeldQuestion(text string) string {
	return "Not held: " + text + ". Does the specification need it, written another way or described by hand?"
}

// notHeldSays is what the cited place has, as the line on standard output
// says it.
func notHeldSays(text string) string {
	return "Not held: " + text + "."
}

// askedBy is the question the reader asked that blocks the pointer, and its
// priority. A could question is never one: it is what this file writes, and
// one thing not held must not stand in for another.
func askedBy(questions *yaml.Node, ptr string) (string, string) {
	if ptr == "" {
		return "", ""
	}
	for i := 0; i+1 < len(questions.Content); i += 2 {
		q := questions.Content[i+1]
		var priority string
		var blocks []string
		for j := 0; j+1 < len(q.Content); j += 2 {
			switch q.Content[j].Value {
			case "priority":
				priority = q.Content[j+1].Value
			case "blocks":
				for _, b := range q.Content[j+1].Content {
					blocks = append(blocks, b.Value)
				}
			}
		}
		for _, b := range blocks {
			if b == ptr && priority != "could" {
				return questions.Content[i].Value, priority
			}
		}
	}
	return "", ""
}

// couldCount is how many could questions askNotHeld will write.
func couldCount(into questionsFor, items []notHeld) int {
	n := 0
	for _, h := range items {
		if id, _ := askedBy(into(h.blocks), h.asked); id == "" {
			n++
		}
	}
	return n
}
