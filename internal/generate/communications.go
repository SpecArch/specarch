package generate

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/semver"
)

// The documents written from the records of who said what and when
// (ADR-220): the communication history and the timeline.

// event is one thing that happened, placed in time: a record, or a dated
// entry of the specification (a decision).
type event struct {
	when     time.Time // for the order; the zero time when the record's instant does not read
	shown    string    // the instant or the date as the record writes it
	kind     string    // communication, session, test-session, deploy, release, commissioning, incident, defect, change, idea, decision, approval
	id       string    // the record's ID, or what stands for one
	label    string    // the event in a few words, starting with its ID
	what     string    // what was said, decided or found, in a line
	roles    []string  // the stakeholders it names
	refs     []string  // the requirements, pointers, decisions and questions it touches
	records  []string  // the other records it links
	versions []string  // the releases it belongs to
	node     *yaml.Node
}

// kindOrder places events of one instant: what was said before what was
// decided, and a session before what was said in it.
var kindOrder = map[string]int{"session": 0, "test-session": 0, "communication": 1, "idea": 2, "change": 3, "defect": 3, "incident": 3, "decision": 4, "approval": 5, "release": 6, "commissioning": 7, "deploy": 8, "test-run": 9}

// zoneOf is the specification's time zone, UTC when it declares none or
// the name is not in the database; a date-only event is the first instant
// of its day there (ADR-193).
func zoneOf(root *yaml.Node) *time.Location {
	if name := str(root, "timeZone"); name != "" {
		if loc, err := time.LoadLocation(name); err == nil {
			return loc
		}
	}
	return time.UTC
}

// placeInTime reads a record's instant, RFC 3339 with its offset, or a
// date, YYYY-MM-DD, read as the start of that day in the zone.
func placeInTime(s string, loc *time.Location) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.ParseInLocation("2006-01-02", s, loc); err == nil {
		return t
	}
	return time.Time{}
}

// quoteWords writes a words object: the text, quoted when it is verbatim.
func quoteWords(w *yaml.Node) string {
	text := oneLine(str(w, "text"))
	if str(w, "form") == "verbatim" {
		return "“" + text + "”"
	}
	return text
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// formWord names the form of the words as a document says it.
func formWord(w *yaml.Node) string {
	if str(w, "form") == "verbatim" {
		return "verbatim"
	}
	return tr("in summary")
}

// attendance is who was invited, present and absent at a session, by
// role; absent is invited minus present.
func attendance(r *yaml.Node) (invited, present, absent []string) {
	invited, present = strs(r, "invited"), strs(r, "present")
	there := map[string]bool{}
	for _, p := range present {
		there[p] = true
	}
	for _, i := range invited {
		if !there[i] {
			absent = append(absent, i)
		}
	}
	return invited, present, absent
}

// linkRefs is what a record's links name that is an element of the
// specification: questions, decisions and elements.
func linkRefs(r *yaml.Node) []string {
	links := get(r, "links")
	var out []string
	out = append(out, strs(links, "questions")...)
	out = append(out, strs(links, "decisions")...)
	out = append(out, strs(links, "elements")...)
	return out
}

// linkRecords is the other records a record's links name.
func linkRecords(r *yaml.Node) []string {
	links := get(r, "links")
	var out []string
	out = append(out, strs(links, "changes")...)
	out = append(out, strs(links, "defects")...)
	out = append(out, strs(links, "ideas")...)
	return out
}

// evidenceText says what shows a record: each file with its media type
// and hash, or that it was recorded without any.
func evidenceText(r *yaml.Node) string {
	ev := items(r, "evidence")
	if len(ev) == 0 {
		return fmt.Sprintf(tr("Recorded without evidence: the record holds what %s wrote down and nothing that shows it."), str(r, "recordedBy"))
	}
	var parts []string
	for _, e := range ev {
		parts = append(parts, fmt.Sprintf("`%s` (%s, %s)", str(e, "path"), str(e, "mediaType"), str(e, "sha256")))
	}
	return tr("Evidence: ") + strings.Join(parts, "; ") + "."
}

// touches names what a record links, grouped as the record writes them.
func touches(r *yaml.Node) string {
	links := get(r, "links")
	var parts []string
	for _, g := range []string{"questions", "decisions", "elements", "changes", "defects", "ideas"} {
		if refs := strs(links, g); len(refs) > 0 {
			parts = append(parts, g+" "+refList(refs))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return tr("Touches: ") + strings.Join(parts, "; ") + "."
}

// sessionLabel is a session's or test session's kind, title and setting.
func sessionLabel(r *yaml.Node) string {
	if str(r, "kind") == "test-session" {
		return fmt.Sprintf(tr("%s: test session “%s” (%s on %s, version %s)"), str(r, "id"), str(r, "title"), str(r, "level"), str(r, "environment"), str(r, "version"))
	}
	return fmt.Sprintf(tr("%s: session “%s” (%s)"), str(r, "id"), str(r, "title"), str(r, "channel"))
}

// communicationLabel is a communication's type, direction and parties.
func communicationLabel(r *yaml.Node) string {
	return fmt.Sprintf(tr("%s: %s from %s to %s (%s)"), str(r, "id"), str(r, "type"), str(r, "from"), str(r, "to"), str(r, "channel"))
}

// Communications writes the communication history from the records: every
// communication, session and test session in time order, then per
// stakeholder and per element (ADR-220). A document for everyone leaves
// the person label out.
func Communications(root *yaml.Node, relRoot string, state *State) string {
	d := newRecordDoc("communications", root, relRoot, state)
	info := get(root, "info")
	loc := zoneOf(root)
	everyone := state.Withheld != nil
	coms, sessions, tests := recordsOf(state, "communication"), recordsOf(state, "session"), recordsOf(state, "test-session")
	type entry struct {
		when time.Time
		r    *yaml.Node
	}
	var entries []entry
	for _, r := range coms {
		entries = append(entries, entry{placeInTime(str(r, "at"), loc), r})
	}
	for _, r := range append(append([]*yaml.Node{}, sessions...), tests...) {
		entries = append(entries, entry{placeInTime(str(r, "start"), loc), r})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if !entries[i].when.Equal(entries[j].when) {
			return entries[i].when.Before(entries[j].when)
		}
		a, b := entries[i].r, entries[j].r
		if kindOrder[str(a, "kind")] != kindOrder[str(b, "kind")] {
			return kindOrder[str(a, "kind")] < kindOrder[str(b, "kind")]
		}
		return idLess(str(a, "id"), str(b, "id"))
	})
	inSession := map[string][]string{}
	for _, r := range coms {
		if s := str(r, "session"); s != "" {
			inSession[s] = append(inSession[s], str(r, "id"))
		}
	}
	withoutEvidence := 0
	for _, r := range coms {
		if len(items(r, "evidence")) == 0 {
			withoutEvidence++
		}
	}

	d.line(tr("# %s: communication history"), str(info, "title"))
	d.blank()
	d.para(fmt.Sprintf(tr("Version %s of the specification: %s, %s and %s are recorded beside it, in `records/communications/`, `records/sessions/` and `records/test-sessions/`. They come first in time order, then the history of each stakeholder and of each question, decision and element they touch. Each record says who said what to whom, when, by which channel and whether the words are verbatim or a summary; %s. People are named by role."),
		str(info, "version"), countText(len(coms), tr("communication"), tr("communications")), countText(len(sessions), tr("session"), tr("sessions")), countText(len(tests), tr("test session"), tr("test sessions")), evidenceCount(len(coms), withoutEvidence)))
	if everyone {
		d.para(tr("This document is for everyone: the person labels the records carry are left out, and so is what touches an internal element."))
	}
	d.stakeholdersNotice()

	d.section(tr("In time order"))
	if len(entries) == 0 {
		d.para(tr("Nothing is recorded."))
	}
	for _, e := range entries {
		r := e.r
		switch str(r, "kind") {
		case "communication":
			d.heading(3, communicationLabel(r))
			d.para(communicationFacts(r))
			lines := []string{capitalise(formWord(get(r, "words"))) + ": " + quoteWords(get(r, "words"))}
			if reply := get(r, "reply"); reply != nil {
				lines = append(lines, fmt.Sprintf(tr("Reply, %s: %s"), formWord(reply), quoteWords(reply)))
			}
			if p := str(r, "person"); p != "" && !everyone {
				lines = append(lines, tr("Person: ")+p+".")
			}
			d.para(strings.Join(lines, " "))
			d.para(evidenceText(r))
			d.para(touchesFor(r, state))
		default:
			d.heading(3, sessionLabel(r))
			d.para(sessionFacts(r))
			if s := str(r, "summary"); s != "" {
				d.para(oneLine(s))
			}
			if ids := inSession[str(r, "id")]; len(ids) > 0 {
				d.para(tr("Said in it: ") + strings.Join(ids, ", ") + ".")
			}
			d.para(evidenceText(r))
			d.para(touchesFor(r, state))
		}
	}

	d.section(tr("By stakeholder"))
	for _, p := range pairs(root, "stakeholders") {
		role := p.Key.Value
		d.heading(3, role)
		var lines []string
		heard := false
		for _, e := range entries {
			r := e.r
			switch str(r, "kind") {
			case "communication":
				if str(r, "from") != role && str(r, "to") != role {
					continue
				}
				heard = true
				lines = append(lines, fmt.Sprintf("- %s, %s: %s", str(r, "at"), communicationLabel(r), quoteWords(get(r, "words"))))
			default:
				invited, present, _ := attendance(r)
				if !contains(invited, role) && !contains(present, role) {
					continue
				}
				was := "present"
				if !contains(present, role) {
					was = tr("invited, absent")
				} else {
					heard = true
				}
				lines = append(lines, fmt.Sprintf("- %s, %s: %s", str(r, "start"), sessionLabel(r), was))
			}
		}
		if len(lines) == 0 {
			d.para(tr("Never heard: no communication names it, and no session lists it."))
			continue
		}
		if !heard {
			d.para(tr("Never heard: invited and absent every time, and no communication names it."))
		}
		d.lines(lines)
	}

	d.section(tr("By question, decision and element"))
	byRef := map[string][]entry{}
	for _, e := range entries {
		for _, ref := range linkRefs(e.r) {
			if everyone && state.Withheld.CoversRef(ref) {
				continue
			}
			byRef[ref] = append(byRef[ref], e)
		}
	}
	refs := make([]string, 0, len(byRef))
	for ref := range byRef {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool { return refLess(refs[i], refs[j]) })
	if len(refs) == 0 {
		d.para(tr("No record touches a question, a decision or an element."))
	}
	for _, ref := range refs {
		d.heading(3, refList([]string{ref}))
		var lines []string
		for _, e := range byRef[ref] {
			r := e.r
			if str(r, "kind") == "communication" {
				lines = append(lines, fmt.Sprintf("- %s, %s: %s", str(r, "at"), communicationLabel(r), quoteWords(get(r, "words"))))
			} else {
				lines = append(lines, fmt.Sprintf("- %s, %s", str(r, "start"), sessionLabel(r)))
			}
		}
		d.lines(lines)
	}
	return d.String()
}

// evidenceCount says how many communications carry no evidence.
func evidenceCount(total, without int) string {
	switch {
	case total == 0:
		return tr("none is recorded yet")
	case without == 0:
		return tr("every communication carries evidence")
	case without == total:
		return tr("none carries evidence, so each reads as what its recorder wrote down")
	}
	if without == 1 {
		return tr("1 communication carries no evidence and reads as what its recorder wrote down")
	}
	return fmt.Sprintf(tr("%d communications carry no evidence and read as what their recorder wrote down"), without)
}

// communicationFacts is a communication's direction, parties, channel,
// instant, session, source and recorder in one paragraph.
func communicationFacts(r *yaml.Node) string {
	dir := tr("In, said by the stakeholder to the project")
	if str(r, "direction") == "out" {
		dir = tr("Out, said by the project to the stakeholder")
	}
	s := fmt.Sprintf(tr("%s: from %s to %s, by %s, at %s"), dir, str(r, "from"), str(r, "to"), str(r, "channel"), str(r, "at"))
	if ses := str(r, "session"); ses != "" {
		s += tr(", in ") + ses
	}
	if src := str(r, "source"); src != "" {
		s += tr(", part of the source ") + src
	}
	return s + tr(". Recorded by ") + str(r, "recordedBy") + "."
}

// sessionFacts is a session's or test session's time, attendance, result
// and recorder in one paragraph.
func sessionFacts(r *yaml.Node) string {
	s := tr("From ") + str(r, "start")
	if end := str(r, "end"); end != "" {
		s += tr(" to ") + end
	}
	if build := str(r, "build"); build != "" {
		s += tr(", build ") + build
	}
	invited, present, absent := attendance(r)
	s += fmt.Sprintf(tr(". Invited: %s. Present: %s. Absent: %s."), listOrNone(invited), listOrNone(present), listOrNone(absent))
	if res := str(r, "result"); res != "" {
		s += tr(" Result: ") + res + "."
	}
	return s + tr(" Recorded by ") + str(r, "recordedBy") + "."
}

func listOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}

// touchesFor is touches, with what a document for everyone leaves out
// taken out.
func touchesFor(r *yaml.Node, state *State) string {
	if state.Withheld == nil {
		return touches(r)
	}
	links := get(r, "links")
	var parts []string
	for _, g := range []string{"questions", "decisions", "elements", "changes", "defects", "ideas"} {
		var kept []string
		for _, ref := range strs(links, g) {
			if !state.Withheld.CoversRef(ref) {
				kept = append(kept, ref)
			}
		}
		if len(kept) > 0 {
			parts = append(parts, g+" "+refList(kept))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return tr("Touches: ") + strings.Join(parts, "; ") + "."
}

// refLess orders references: requirement IDs and record IDs by prefix and
// number, pointers after them by text.
func refLess(a, b string) bool {
	pa, pb := strings.HasPrefix(a, "#/"), strings.HasPrefix(b, "#/")
	if pa != pb {
		return !pa
	}
	if pa {
		return a < b
	}
	return idLess(a, b)
}

// lines writes a list, one line each, and a blank line after it.
func (d *doc) lines(lines []string) {
	for _, l := range lines {
		d.line("%s", l)
	}
	d.blank()
}

// Timeline writes every event of the records and the specification in one
// time order, then by stakeholder, by element and by release (ADR-220).
func Timeline(root *yaml.Node, relRoot string, state *State) string {
	d := newRecordDoc("timeline", root, relRoot, state)
	info := get(root, "info")
	loc := zoneOf(root)
	events := timelineEvents(root, state, loc)
	zone := tr("UTC, since the specification declares no time zone")
	if name := str(root, "timeZone"); name != "" {
		zone = name
	}

	d.line(tr("# %s: timeline"), str(info, "title"))
	d.blank()
	d.para(fmt.Sprintf(tr("Version %s of the specification: %s in one time order, from the records beside it and the specification's decisions: communications, sessions, test sessions, test runs, deploys, releases, commissioning runs, incidents, defects, change requests, ideas, decisions and approvals. An event with an instant is placed at it; one with a date alone is placed at the start of that day in the specification's time zone (%s) and shown by its date; one whose record gives no date yet, such as a planned release, comes last. Then the same events by stakeholder, by requirement, decision or element, and by release."),
		str(info, "version"), countText(len(events), tr("event"), tr("events")), zone))
	if state.Withheld != nil {
		d.para(tr("This document is for everyone: what touches an internal element is left out."))
	}
	d.stakeholdersNotice()

	d.section(tr("Every event"))
	if len(events) == 0 {
		d.para(tr("Nothing is recorded."))
	} else {
		d.line(tr("| When | Event | What | Links |"))
		d.line("|---|---|---|---|")
		for _, e := range events {
			d.line("| %s | %s | %s | %s |", cell(e.shown), cell(e.label), cell(e.what), cell(eventLinks(e, state)))
		}
		d.blank()
	}

	d.section(tr("By stakeholder"))
	for _, p := range pairs(root, "stakeholders") {
		role := p.Key.Value
		d.heading(3, role)
		var lines []string
		for _, e := range events {
			if contains(e.roles, role) {
				lines = append(lines, fmt.Sprintf("- %s, %s: %s", e.shown, e.label, e.what))
			}
		}
		if len(lines) == 0 {
			d.para(tr("No event names it."))
			continue
		}
		d.lines(lines)
	}

	d.section(tr("By requirement, decision and element"))
	byRef := map[string][]event{}
	for _, e := range events {
		for _, ref := range e.refs {
			if state.Withheld.CoversRef(ref) {
				continue
			}
			byRef[ref] = append(byRef[ref], e)
		}
	}
	refs := make([]string, 0, len(byRef))
	for ref := range byRef {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool { return refLess(refs[i], refs[j]) })
	if len(refs) == 0 {
		d.para(tr("No event touches a requirement, a decision or an element."))
	}
	for _, ref := range refs {
		d.heading(3, refList([]string{ref}))
		var lines []string
		for _, e := range byRef[ref] {
			lines = append(lines, fmt.Sprintf("- %s, %s: %s", e.shown, e.label, e.what))
		}
		d.lines(lines)
	}

	d.section(tr("By release"))
	byVersion := map[string][]event{}
	for _, e := range events {
		for _, v := range e.versions {
			byVersion[v] = append(byVersion[v], e)
		}
	}
	versions := make([]string, 0, len(byVersion))
	for v := range byVersion {
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool {
		a, okA := semver.Parse(versions[i])
		b, okB := semver.Parse(versions[j])
		if !okA || !okB {
			return versions[i] < versions[j]
		}
		return semver.Compare(a, b) < 0
	})
	if len(versions) == 0 {
		d.para(tr("No event names a release."))
	}
	for _, v := range versions {
		d.heading(3, v)
		var lines []string
		for _, e := range byVersion[v] {
			lines = append(lines, fmt.Sprintf("- %s, %s: %s", e.shown, e.label, e.what))
		}
		d.lines(lines)
	}
	return d.String()
}

// eventLinks is an event's links column: what it touches, the records it
// links and the releases it belongs to.
func eventLinks(e event, state *State) string {
	var parts []string
	var refs []string
	for _, ref := range e.refs {
		if !state.Withheld.CoversRef(ref) {
			refs = append(refs, ref)
		}
	}
	if len(refs) > 0 {
		parts = append(parts, refList(refs))
	}
	if len(e.records) > 0 {
		parts = append(parts, strings.Join(e.records, ", "))
	}
	if len(e.versions) > 0 {
		parts = append(parts, tr("release ")+strings.Join(e.versions, ", "))
	}
	return strings.Join(parts, "; ")
}

// timelineEvents reads every event, in time order.
func timelineEvents(root *yaml.Node, state *State, loc *time.Location) []event {
	var out []event
	add := func(e event) {
		e.refs = uniqueStrings(e.refs)
		e.records = uniqueStrings(e.records)
		e.versions = uniqueStrings(e.versions)
		e.roles = uniqueStrings(e.roles)
		out = append(out, e)
	}
	for _, r := range state.Records {
		id := str(r, "id")
		switch str(r, "kind") {
		case "communication":
			add(event{when: placeInTime(str(r, "at"), loc), shown: str(r, "at"), kind: "communication", id: id, label: communicationLabel(r), what: quoteWords(get(r, "words")),
				roles: []string{str(r, "from"), str(r, "to")}, refs: linkRefs(r), records: linkRecords(r), node: r})
		case "session", "test-session":
			invited, present, absent := attendance(r)
			what := oneLine(str(r, "summary"))
			if res := str(r, "result"); res != "" {
				what = strings.TrimSpace(tr("Result: ") + res + ". " + what)
			}
			what = strings.TrimSpace(what + fmt.Sprintf(tr(" Present: %s; absent: %s."), listOrNone(present), listOrNone(absent)))
			e := event{when: placeInTime(str(r, "start"), loc), shown: str(r, "start"), kind: str(r, "kind"), id: id, label: sessionLabel(r), what: what,
				roles: append(append([]string{}, invited...), present...), refs: linkRefs(r), records: linkRecords(r), node: r}
			if v := str(r, "version"); v != "" {
				e.versions = []string{v}
			}
			add(e)
		case "deploy":
			what := tr("Result: ") + str(r, "result") + "."
			if n := str(r, "note"); n != "" {
				what += " " + oneLine(n)
			}
			if b := str(r, "build"); b != "" {
				what += tr(" Build ") + b + "."
			}
			add(event{when: placeInTime(str(r, "at"), loc), shown: str(r, "at"), kind: "deploy", id: id, label: fmt.Sprintf(tr("%s: deploy of %s to %s by %s"), id, str(r, "version"), str(r, "environment"), str(r, "by")), what: what,
				roles: []string{str(r, "by")}, refs: linkRefs(r), records: linkRecords(r), versions: []string{str(r, "version")}, node: r})
		case "test-run":
			c := countRun(r)
			what := fmt.Sprintf(tr("%d passed, %d failed, %d errored, %d skipped, %d mapping to no test."), c.passed, c.failed, c.errored, c.skipped, c.unmapped)
			if b := str(r, "build"); b != "" {
				what += tr(" Build ") + b + "."
			}
			var refs, recs []string
			for _, res := range items(r, "results") {
				if o := str(res, "outcome"); o != "failed" && o != "error" {
					continue
				}
				refs = append(refs, strs(testOf(root, str(res, "test")), "verifies")...)
				if dft := str(res, "defect"); dft != "" {
					recs = append(recs, dft)
				}
			}
			e := event{when: placeInTime(str(r, "at"), loc), shown: str(r, "at"), kind: "test-run", id: id, label: fmt.Sprintf(tr("%s: test run in %s by %s"), id, str(r, "environment"), str(r, "by")), what: what,
				roles: []string{str(r, "by")}, refs: refs, records: recs, node: r}
			if v := str(r, "version"); v != "" {
				e.versions = []string{v}
			}
			add(e)
		case "release":
			v := str(r, "version")
			what := tr("Includes nothing recorded.")
			if inc := strs(r, "includes"); len(inc) > 0 {
				what = tr("Includes ") + strings.Join(inc, ", ") + "."
			}
			add(event{when: placeInTime(str(r, "date"), loc), shown: str(r, "date"), kind: "release", id: v, label: fmt.Sprintf(tr("release %s, %s"), v, str(r, "status")), what: what,
				records: strs(r, "includes"), versions: []string{v}, node: r})
		case "commissioning":
			v := str(r, "version")
			passed, failed := 0, 0
			for _, res := range pairs(r, "results") {
				if str(res.Value, "result") == "pass" {
					passed++
				} else {
					failed++
				}
			}
			what := fmt.Sprintf(tr("%s passed, %s failed."), countText(passed, tr("check"), tr("checks")), countText(failed, tr("check"), tr("checks")))
			roles := []string{str(r, "operator")}
			if so := get(r, "signoff"); so != nil {
				what += fmt.Sprintf(tr(" Signed off by %s on %s."), str(so, "by"), str(so, "date"))
				roles = append(roles, str(so, "by"))
			}
			add(event{when: placeInTime(str(r, "date"), loc), shown: str(r, "date"), kind: "commissioning", id: v + " " + str(r, "environment"), label: fmt.Sprintf(tr("commissioning run of %s on %s by %s"), v, str(r, "environment"), str(r, "operator")), what: what,
				roles: roles, versions: []string{v}, node: r})
		case "incident":
			add(event{when: placeInTime(str(r, "detected"), loc), shown: str(r, "detected"), kind: "incident", id: id, label: fmt.Sprintf(tr("%s: incident on %s, %s"), id, str(r, "environment"), str(r, "status")), what: oneLine(str(r, "summary")),
				roles: []string{str(r, "reportedBy")}, records: append(strs(r, "defects"), strs(r, "changes")...), node: r})
		case "defect":
			e := event{when: placeInTime(str(r, "reported"), loc), shown: str(r, "reported"), kind: "defect", id: id, label: fmt.Sprintf(tr("%s: defect reported by %s, %s"), id, str(r, "reportedBy"), str(r, "status")), what: oneLine(str(r, "title")),
				roles: []string{str(r, "reportedBy")}, refs: strs(r, "violates"), node: r}
			if rel := str(r, "release"); rel != "" {
				e.versions = []string{rel}
			}
			add(e)
		case "change":
			affects := get(r, "affects")
			refs := append(append(strs(affects, "adds"), strs(affects, "changes")...), strs(affects, "removes")...)
			e := event{when: placeInTime(str(r, "raised"), loc), shown: str(r, "raised"), kind: "change", id: id, label: fmt.Sprintf(tr("%s: change request raised by %s"), id, str(r, "raisedBy")), what: oneLine(str(r, "title")),
				roles: []string{str(r, "raisedBy")}, refs: refs, node: r}
			if rel := str(r, "release"); rel != "" {
				e.versions = []string{rel}
			}
			add(e)
			if dec := get(r, "decision"); dec != nil && str(dec, "date") != "" {
				add(event{when: placeInTime(str(dec, "date"), loc), shown: str(dec, "date"), kind: "change", id: id + tr(" decided"), label: fmt.Sprintf(tr("%s: change request %s by %s"), id, str(dec, "outcome"), str(dec, "by")), what: oneLine(str(dec, "why")),
					roles: []string{str(dec, "by")}, refs: refs, versions: e.versions, node: r})
			}
		case "idea":
			e := event{when: placeInTime(str(r, "raised"), loc), shown: str(r, "raised"), kind: "idea", id: id, label: fmt.Sprintf(tr("%s: idea raised%s, %s"), id, byRole(str(r, "raisedBy")), str(r, "status")), what: oneLine(str(r, "statement")),
				refs: []string{str(r, "about")}, node: r}
			if by := str(r, "raisedBy"); by != "" {
				e.roles = []string{by}
			}
			add(e)
			if dec := get(r, "decision"); dec != nil && str(dec, "date") != "" {
				add(event{when: placeInTime(str(dec, "date"), loc), shown: str(dec, "date"), kind: "idea", id: id + tr(" decided"), label: fmt.Sprintf(tr("%s: idea %s by %s"), id, str(dec, "outcome"), str(dec, "by")), what: oneLine(str(dec, "why")),
					roles: []string{str(dec, "by")}, refs: []string{str(r, "about")}, node: r})
			}
		case "approval":
			v := str(r, "version")
			add(event{when: placeInTime(str(r, "date"), loc), shown: str(r, "date"), kind: "approval", id: v, label: fmt.Sprintf(tr("approval of %s by %s"), v, str(r, "approvedBy")), what: tr("Documents: ") + listOrNone(strs(r, "documents")) + ".",
				roles: []string{str(r, "approvedBy")}, versions: []string{v}, node: r})
		}
	}
	for _, p := range pairs(root, "decisions") {
		dec := p.Value
		id := p.Key.Value
		refs := append([]string{id}, strs(dec, "answers")...)
		refs = append(refs, strs(dec, "satisfies")...)
		what := sentence(oneLine(str(dec, "title")))
		if by := str(dec, "decidedBy"); by != "" {
			what += tr(" Decided by ") + by + "."
		}
		var records []string
		for _, c := range items(dec, "cites") {
			if rec := str(c, "record"); rec != "" {
				records = append(records, rec)
			}
		}
		e := event{when: placeInTime(str(dec, "date"), loc), shown: str(dec, "date"), kind: "decision", id: id, label: fmt.Sprintf(tr("%s: decision, %s"), id, str(dec, "status")), what: what, refs: refs, records: records, node: dec}
		if by := str(dec, "decidedBy"); by != "" {
			e.roles = []string{by}
		}
		add(e)
	}
	for i := range out {
		if out[i].shown == "" {
			out[i].shown = tr("no date")
		}
	}
	// An event without a date or instant comes last, since it has not
	// happened yet or its record does not say when.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.when.IsZero() != b.when.IsZero() {
			return !a.when.IsZero()
		}
		if !a.when.Equal(b.when) {
			return a.when.Before(b.when)
		}
		if kindOrder[a.kind] != kindOrder[b.kind] {
			return kindOrder[a.kind] < kindOrder[b.kind]
		}
		return idLess(a.id, b.id)
	})
	return out
}

func byRole(role string) string {
	if role == "" {
		return ""
	}
	return tr(" by ") + role
}

func uniqueStrings(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// testOf is the test of the specification of that name, nil for none.
func testOf(root *yaml.Node, name string) *yaml.Node {
	if name == "" {
		return nil
	}
	return get(get(root, "tests"), name)
}
