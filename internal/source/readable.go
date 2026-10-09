package source

import (
	"strconv"
	"strings"
)

// A file that does not parse keeps what can be read (ADR-080). YAML reads a
// document as one unit and refuses it whole at the first syntax error, so
// every entry of the file would be lost and every reference to one of them
// would be reported again. Instead the file is read entry by entry, by its
// indentation: an entry is parsed with the lines of the keys above it and
// every other line made a comment, so each value keeps its line and column.
// An entry that parses with the entries kept before it is kept. One that does not is read entry by entry in
// its turn when it is a key whose value is on the lines below, and is
// otherwise kept by its name with no value (held), its line cut after the
// key, so whatever refers to it still finds it; a top-level key is left out
// instead, since nothing refers to a section by name. Each entry that does
// not parse is a syntax error at its own line and pointer, and nothing else
// is reported at it, inside it, or of it as missing (Doc.Held).

const (
	tabNote     = "this line is indented with a tab, and YAML indents with spaces only; indent it with spaces"
	heldNote    = "; the other entries of the file are read, and this one is kept by its name only until it parses"
	droppedNote = "; the other entries of the file are read, and this one is left out until it parses"
)

type reader struct {
	lines    []string
	held     []string
	problems []Problem
}

// readable reads what can be read of a file that does not parse; nil when
// no entry can be told apart from the error.
func readable(data []byte) *Doc {
	r := &reader{lines: strings.Split(string(data), "\n")}
	ctx := map[int]string{}
	lo := 0
	// A directive or a document start before the first key is kept as it is.
	for lo < len(r.lines) {
		t := strings.TrimLeft(r.lines[lo], " ")
		if t != "" && !strings.HasPrefix(t, "#") && !strings.HasPrefix(t, "%") && !strings.HasPrefix(t, "---") {
			break
		}
		if t != "" && !strings.HasPrefix(t, "#") {
			ctx[lo] = r.lines[lo]
		}
		lo++
	}
	// A line indented with a tab is read by no YAML parser; it is left out.
	for i, l := range r.lines {
		if strings.HasPrefix(l, "\t") && strings.TrimSpace(l) != "" {
			r.problems = append(r.problems, Problem{Line: i + 1, Path: "/", Rule: "yaml_syntax", Message: tabNote + droppedNote})
			r.lines[i] = "#"
		}
	}
	out := r.block(ctx, lo, len(r.lines), "", 0)
	if len(r.problems) == 0 {
		return nil
	}
	d, _ := parseWhole([]byte(r.masked(ctx, out)))
	if d.Root == nil {
		return nil
	}
	d.Problems = append(r.problems, d.Problems...)
	d.Held = r.held
	return d
}

// masked is the file with the lines of every given map as they are given
// and every other line a comment.
func (r *reader) masked(parts ...map[int]string) string {
	lines := make([]string, len(r.lines))
	for i := range lines {
		lines[i] = "#"
		for _, p := range parts {
			if l, ok := p[i]; ok {
				lines[i] = l
			}
		}
	}
	return strings.Join(lines, "\n")
}

// parses reads the file made of the given lines, and returns its syntax
// error, or nil when it parses.
func (r *reader) parses(parts ...map[int]string) *Problem {
	d, syntax := parseWhole([]byte(r.masked(parts...)))
	if syntax {
		return &d.Problems[0]
	}
	return nil
}

// block reads the entries between lines lo and hi, under the keys of ctx,
// and returns the lines to read them by.
func (r *reader) block(ctx map[int]string, lo, hi int, ptr string, depth int) map[int]string {
	var starts []int
	indent, items := -1, false
	for i := lo; i < hi; i++ {
		t := strings.TrimLeft(r.lines[i], " ")
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		at := len(r.lines[i]) - len(t)
		item := t == "-" || strings.HasPrefix(t, "- ")
		if indent < 0 {
			indent, items = at, item
		}
		// A list written at the indentation of the key that holds it
		// belongs to that key.
		if at <= indent && (items || !item || at < indent) {
			starts = append(starts, i)
		}
	}
	out := map[int]string{}
	for n, s := range starts {
		e := hi
		if n+1 < len(starts) {
			e = starts[n+1]
		}
		chunk := map[int]string{}
		for i := s; i < e; i++ {
			chunk[i] = r.lines[i]
		}
		problem := r.parses(ctx, out, chunk)
		if problem == nil {
			for i, l := range chunk {
				out[i] = l
			}
			continue
		}
		if problem.Line < s+1 || problem.Line > e {
			// Read with the lines after it made comments, an entry left open
			// fails where the file ends; the error is the entry's.
			problem.Line = s + 1
		}
		line := r.lines[s]
		key, cut, item, ok := entryOf(line)
		at := ptr
		if ok {
			if item {
				key = strconv.Itoa(n)
			}
			at = ptr + "/" + EscapeToken(key)
		}
		if after := strings.TrimSpace(line[len(cut):]); ok && !item && e-s > 1 && (after == "" || strings.HasPrefix(after, "#")) {
			// A key whose value is on the lines below: read its entries.
			problems, held := len(r.problems), len(r.held)
			inner := map[int]string{s: cut}
			for k, v := range ctx {
				inner[k] = v
			}
			sub := r.block(inner, s+1, e, at, depth+1)
			sub[s] = cut
			if len(r.problems) > problems && r.parses(ctx, out, sub) == nil {
				for i, l := range sub {
					out[i] = l
				}
				continue
			}
			r.problems, r.held = r.problems[:problems], r.held[:held]
		}
		if ok && depth > 0 && r.parses(ctx, out, map[int]string{s: cut}) == nil {
			out[s] = cut
			r.held = append(r.held, at)
			r.problems = append(r.problems, Problem{Line: problem.Line, Path: at, Rule: problem.Rule, Message: problem.Message + heldNote})
			continue
		}
		if ok {
			r.held = append(r.held, at)
		}
		r.problems = append(r.problems, Problem{Line: problem.Line, Path: orRoot(at), Rule: problem.Rule, Message: problem.Message + droppedNote})
	}
	return out
}

// entryOf reads the start of an entry's first line: a list's item, whose
// line is cut after its dash, or a key, plain or quoted without escapes,
// cut after its colon. ok is false when the line starts no entry that can
// be told.
func entryOf(line string) (key, cut string, item, ok bool) {
	t := strings.TrimLeft(line, " ")
	lead := line[:len(line)-len(t)]
	if t == "-" || strings.HasPrefix(t, "- ") {
		return "", lead + "-", true, true
	}
	var rest string
	switch {
	case strings.HasPrefix(t, `"`):
		end := strings.Index(t[1:], `"`)
		if end < 0 || strings.Contains(t[1:1+end], `\`) {
			return "", "", false, false
		}
		key, rest = t[1:1+end], t[2+end:]
	case strings.HasPrefix(t, "'"):
		end := strings.Index(t[1:], "'")
		if end < 0 || strings.HasPrefix(t[2+end:], "'") {
			return "", "", false, false
		}
		key, rest = t[1:1+end], t[2+end:]
	case strings.ContainsAny(t[:1], "[]{},&*!|>%@`#?:-\t"):
		return "", "", false, false
	default:
		colon := -1
		for i := 0; i < len(t); i++ {
			if t[i] == ':' && (i+1 == len(t) || t[i+1] == ' ') {
				colon = i
				break
			}
			if t[i] == '#' && i > 0 && t[i-1] == ' ' {
				break
			}
		}
		if colon <= 0 {
			return "", "", false, false
		}
		key, rest = strings.TrimRight(t[:colon], " "), t[colon:]
	}
	if !strings.HasPrefix(rest, ":") || len(rest) > 1 && rest[1] != ' ' {
		return "", "", false, false
	}
	return key, line[:len(line)-len(rest)+1], false, true
}
