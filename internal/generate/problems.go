package generate

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// Mark is one error or warning of the specification as a document marks it
// (ADR-065): a Problem paragraph at the element its pointer leads to, or
// under the document's summary when the document shows no element for it.
type Mark struct {
	Severity     string // error or warning
	File         string // as the document names it, from its folder
	Line, Column int
	// Pointer leads into the merged specification; it is "" when the
	// problem is in a file outside it, such as an implementation file or a
	// record.
	Pointer           string
	Rule, Message, ID string
}

// String is the problem line: file:line:column: severity: pointer: rule:
// message [id].
func (m Mark) String() string {
	return fmt.Sprintf("%s:%d:%d: %s: %s: %s: %s [%s]", m.File, m.Line, m.Column, m.Severity, m.Pointer, m.Rule, m.Message, m.ID)
}

// paragraph is the mark's Problem paragraph: the problem's line from the
// severity on, without the pointer, which the element it stands at and the
// id already give.
func (m Mark) paragraph(label string) string {
	on := ""
	if label != "" {
		on = " on " + label
	}
	return fmt.Sprintf("**Problem%s:** %s: %s: %s [%s]", on, m.Severity, m.Rule, m.Message, m.ID)
}

// concerns reports whether a document reading the sections ("*" for all)
// is concerned by the mark: one in a section it reads, or one about a
// fragment as a whole.
func (m Mark) concerns(sections []string) bool {
	if len(sections) == 1 && sections[0] == "*" {
		return true
	}
	if m.Pointer == "" {
		return false
	}
	if m.Pointer == "/" {
		return true
	}
	first, _, _ := strings.Cut(strings.TrimPrefix(m.Pointer, "/"), "/")
	return slices.Contains(sections, source.UnescapeToken(first))
}

// placing is the state of a document written twice: the first time to
// learn which elements it shows, the second with each mark at the deepest
// of them that holds its pointer.
type placing struct {
	collecting bool
	shown      map[string]bool   // the pointers of the elements the document shows
	at         map[string][]Mark // element pointer -> the marks written there
}

// place writes a document twice when the specification has marks, so that
// each mark stands at the deepest element the document shows that holds
// it; write makes the document from the state.
func place(state *State, write func() string) string {
	if state == nil || len(state.Marks) == 0 {
		return write()
	}
	state.placing = &placing{collecting: true, shown: map[string]bool{}}
	write()
	p := state.placing
	p.collecting = false
	p.at = map[string][]Mark{}
	for _, m := range state.Marks {
		if at, ok := deepestShown(p.shown, m.Pointer); ok {
			p.at[at] = append(p.at[at], m)
		}
	}
	text := write()
	state.placing = nil
	return text
}

// deepestShown is the pointer of the deepest element shown that is the
// pointer or holds it. The root is never one: a mark there stands under
// the summary.
func deepestShown(shown map[string]bool, ptr string) (string, bool) {
	if ptr == "" || ptr == "/" {
		return "", false
	}
	for p := ptr; p != ""; {
		if shown[p] {
			return p, true
		}
		i := strings.LastIndex(p, "/")
		if i <= 0 {
			break
		}
		p = p[:i]
	}
	return "", false
}

// marksAt writes the Problem paragraphs of an element, or, on the first
// writing, notes that the document shows it.
func (d *doc) marksAt(label string, n *yaml.Node) {
	if d.placing == nil {
		return
	}
	ptr, ok := d.pointerOf[source.Deref(n)]
	if !ok || ptr == "" {
		return
	}
	if d.placing.collecting {
		d.placing.shown[ptr] = true
		return
	}
	for _, m := range d.placing.at[ptr] {
		d.para(m.paragraph(label))
	}
}

// problemsNotice writes, under a document's summary, that the
// specification is invalid while it has errors, and how many errors and
// warnings concern the document, then the Problem paragraph of each one
// the document shows no element for. A document with no error in the
// specification and no warning of its own gets none.
func (d *doc) problemsNotice(target string) {
	reads := documentReads[target]
	var here []Mark
	hereErrors := 0
	for _, m := range d.marks {
		if m.concerns(reads) || d.shows(m) {
			here = append(here, m)
			if m.Severity == "error" {
				hereErrors++
			}
		}
	}
	errors := d.errorCount()
	if errors == 0 && len(here) == 0 {
		return
	}
	var text []string
	if errors > 0 {
		text = append(text, fmt.Sprintf("The specification has %s, so it is invalid, and this document shows what could be read of it.", countText(errors, "error", "errors")))
	}
	switch {
	case len(here) == 0:
		text = append(text, "None of them is in what this document covers.")
	default:
		var parts []string
		if hereErrors > 0 {
			parts = append(parts, countText(hereErrors, "error", "errors"))
		}
		if warnings := len(here) - hereErrors; warnings > 0 {
			parts = append(parts, countText(warnings, "warning", "warnings"))
		}
		if len(here) == 1 {
			text = append(text, fmt.Sprintf("%s concerns this document; it is marked by a Problem paragraph at its element, or below when the document shows no element for it.", parts[0]))
		} else {
			text = append(text, fmt.Sprintf("%s concern this document; each is marked by a Problem paragraph at its element, or below when the document shows no element for it.", strings.Join(parts, " and ")))
		}
	}
	text = append(text, "The problems file lists every problem, and specarch validate prints them.")
	d.para("**Problems:** " + strings.Join(text, " "))
	for _, m := range here {
		if !d.shows(m) {
			d.para(m.paragraph(""))
		}
	}
}

// shows reports whether the document marks the problem at an element; it
// is false on the first writing, which only learns the elements.
func (d *doc) shows(m Mark) bool {
	if d.placing == nil || d.placing.collecting {
		return false
	}
	_, ok := deepestShown(d.placing.shown, m.Pointer)
	return ok
}

// errorsSection lists the errors of the specification as problem lines, for
// the open questions document and specarch gaps.
func (d *doc) errorsSection() {
	var lines []string
	for _, m := range d.marks {
		if m.Severity == "error" {
			lines = append(lines, m.String())
		}
	}
	if len(lines) == 0 {
		return
	}
	d.section("Errors")
	d.para(fmt.Sprintf("The specification has %s, so it is invalid: the questions below are those of what could be read, and nothing is ready until the errors are fixed. Each line gives the file, line and column, the pointer, the rule and how to fix it.", countText(len(lines), "error", "errors")))
	d.line("```text")
	for _, l := range lines {
		d.line("%s", l)
	}
	d.line("```")
	d.blank()
}

// errorCount is the number of errors the specification has.
func (d *doc) errorCount() int {
	return d.errorsConcerning([]string{"*"})
}

// errorsConcerning is the number of errors in what a document reading the
// sections covers.
func (d *doc) errorsConcerning(sections []string) int {
	n := 0
	for _, m := range d.marks {
		if m.Severity == "error" && m.concerns(sections) {
			n++
		}
	}
	return n
}
