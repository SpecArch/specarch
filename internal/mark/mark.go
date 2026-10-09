// Package mark places a specification's problems at the entries a
// generated file shows (docs/diagnostics.md, section 5): the rule every
// generator plug-in marks its output by, so each target puts a problem at
// the same entry and writes only its own comment form.
package mark

import (
	"fmt"
	"strings"
)

// Problem is one problem as a plug-in request carries it.
type Problem struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	// Pointer leads into the merged specification, or is "" for a problem
	// outside it.
	Pointer string `json:"pointer"`
	// Blocks are what a question blocks, as it names them: a section, or
	// a pointer that starts with "#/".
	Blocks []string `json:"blocks,omitempty"`
	// Target is true for a problem the target itself reported on its
	// first run: specarch runs it again with them, so it marks them too.
	Target bool `json:"target,omitempty"`
}

// File is the element that stands for a file as a whole: its marks go
// under the header.
const File = ""

// Line is a problem's mark from "specarch-problem:" on, on one line; each
// target writes it in its own comment form.
func Line(p Problem) string {
	return fmt.Sprintf("specarch-problem: %s: %s: %s [%s]", p.Severity, p.Rule, strings.Join(strings.Fields(p.Message), " "), p.ID)
}

// Place gives each element of a file the problems marked at it, in the
// order given, each once.
//
// elements are the pointers of the entries the file shows; an entry
// holds every pointer under it, and the deepest entry that holds a
// pointer is where it is marked. sections are the sections the file shows
// as a whole, marked under its header: a section holds only its own
// pointer, so a problem of an entry the file leaves out, such as one
// another stakeholder owns, is not marked in it.
//
// A warning or an error is marked at its pointer, a question at each
// pointer it blocks (a section by its name). A problem of the target
// itself that no entry holds, such as one about its settings, is marked
// at File, so none goes unmarked. Any other problem no entry holds is not
// marked in the file; the problems file lists it.
func Place(problems []Problem, elements, sections []string) map[string][]Problem {
	out := map[string][]Problem{}
	for _, p := range problems {
		pointers := []string{p.Pointer}
		if p.Severity == "question" {
			pointers = nil
			for _, b := range p.Blocks {
				switch {
				case strings.HasPrefix(b, "#/"):
					pointers = append(pointers, b[1:])
				case strings.HasPrefix(b, "/"):
					pointers = append(pointers, b)
				default:
					pointers = append(pointers, "/"+Escape(b))
				}
			}
		}
		for _, ptr := range pointers {
			at, found := "", false
			for _, s := range sections {
				if ptr == s {
					at, found = File, true
				}
			}
			best := -1
			for _, e := range elements {
				if (ptr == e || strings.HasPrefix(ptr, e+"/")) && len(e) > best {
					at, found, best = e, true, len(e)
				}
			}
			if !found && p.Target {
				at, found = File, true
			}
			if !found {
				continue
			}
			seen := false
			for _, m := range out[at] {
				seen = seen || m.ID == p.ID
			}
			if !seen {
				out[at] = append(out[at], p)
			}
		}
	}
	return out
}

// Escape writes a key as a JSON pointer token (RFC 6901).
func Escape(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}
