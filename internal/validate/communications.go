package validate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// The checks of the records that keep who said what and when: a
// communication, a session, a test session and a deploy (ADR-220).

// roles checks that every item of a list is a stakeholder.
func (rc *recordChecks) roles(r *record, key string) {
	for i, item := range source.Items(source.Child(r.root, key)) {
		if v := item.Value; v != "" && rc.d.stakeholders[v] == nil {
			r.c.add(item, source.Pointer(key, fmt.Sprint(i)), RuleRecordRef, "%s is not a stakeholder of the specification%s", v, suggest(v, rc.d.stakeholders))
		}
	}
}

// session checks that a communication's session is a session or a test
// session record.
func (rc *recordChecks) session(r *record) {
	n := source.Child(r.root, "session")
	v := source.Str(n)
	if n == nil || v == "" || rc.set.has("session", v) || rc.set.has("test-session", v) {
		return
	}
	r.c.add(n, "/session", RuleRecordRef, "%s is not a session or a test session: there is no records/sessions/%s.yaml or records/test-sessions/%s.yaml", v, v, v)
}

// checkLinks checks what a record links: questions open or answered,
// decisions, elements, and the change, defect and idea records.
func (rc *recordChecks) checkLinks(r *record) {
	links := source.Child(r.root, "links")
	for i, item := range source.Items(source.Child(links, "questions")) {
		if v := item.Value; v != "" && rc.d.questions[v] == nil && !rc.d.answered(v) {
			r.c.add(item, source.Pointer("links", "questions", fmt.Sprint(i)), RuleRecordRef, "%s is not a question of the specification: it is not open under questions and no decision answers it", v)
		}
	}
	for i, item := range source.Items(source.Child(links, "decisions")) {
		if v := item.Value; v != "" && rc.d.decisions[v] == nil {
			r.c.add(item, source.Pointer("links", "decisions", fmt.Sprint(i)), RuleRecordRef, "%s is not a decision of the specification%s", v, suggest(v, rc.d.decisions))
		}
	}
	for i, item := range source.Items(source.Child(links, "elements")) {
		rc.reference(r, item, source.Pointer("links", "elements", fmt.Sprint(i)))
	}
	for _, list := range []struct{ key, kind string }{{"changes", "change"}, {"defects", "defect"}, {"ideas", "idea"}} {
		for i, item := range source.Items(source.Child(links, list.key)) {
			rc.recordID(r, item, source.Pointer("links", list.key, fmt.Sprint(i)), list.kind)
		}
	}
}

// checkEvidence checks that each file a record names as evidence is under
// records/ and still has the SHA-256 the record holds.
func (rc *recordChecks) checkEvidence(r *record) {
	for i, item := range source.Items(source.Child(r.root, "evidence")) {
		pathNode := source.Child(item, "path")
		p := source.Str(pathNode)
		if p == "" {
			continue
		}
		ptr := source.Pointer("evidence", fmt.Sprint(i), "path")
		full := filepath.Join(rc.dir, filepath.FromSlash(p))
		if rel, err := filepath.Rel(rc.dir, full); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			r.c.add(pathNode, ptr, RuleEvidence, "%s is not under records/; evidence lives under records/, named by its path from there", p)
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			r.c.add(pathNode, ptr, RuleEvidence, "%s is not under records/: there is no records/%s; put the file there, or correct the path", p, p)
			continue
		}
		sum := sha256.Sum256(data)
		got := "sha256:" + hex.EncodeToString(sum[:])
		if want := source.Str(source.Child(item, "sha256")); want != "" && want != got {
			r.c.add(source.Child(item, "sha256"), source.Pointer("evidence", fmt.Sprint(i), "sha256"), RuleEvidence, "records/%s has changed since it was recorded: its SHA-256 is now %s, not %s; restore the file, or record it again as it is now", p, got, want)
		}
	}
}

// citedTypes are the types of a communication record an element may cite:
// what a stakeholder said, not what they asked for.
var citedTypes = map[string]bool{"answer": true, "information": true}

// checkCitedRecords checks every citation that names a communication
// record: the record exists, its type is one a citation may rest on, and a
// decision that answers questions and cites an answer cites one to one of
// them, said by the role that decided; information cited by a decision is
// held to neither, since it answers nothing (ADR-220).
func (rc *recordChecks) checkCitedRecords(c *checker) {
	walk(rc.d.root, nil, func(n *yaml.Node, path []string) {
		for i, item := range source.Items(source.Child(n, "cites")) {
			recNode := source.Child(item, "record")
			id := source.Str(recNode)
			if id == "" {
				continue
			}
			ptr := source.Pointer(append(path, "cites", fmt.Sprint(i), "record")...)
			rec := rc.set["communication"][id]
			if rec == nil {
				c.add(recNode, ptr, RuleRecordCited, "%s is not a communication record: there is no records/communications/%s.yaml; record what was said with specarch log, then cite it", id, id)
				continue
			}
			typ := rec.str("type")
			if !citedTypes[typ] {
				c.add(recNode, ptr, RuleRecordCited, "%s is %s, which settles nothing by itself: cite the record of the answer given, or the decision it led to; a citation rests on an answer or on information", id, typeArticle(typ))
				continue
			}
			if len(path) == 2 && path[0] == "decisions" && typ == "answer" {
				answers := itemValues(source.Child(n, "answers"))
				if len(answers) == 0 {
					continue
				}
				linked := map[string]bool{}
				for _, q := range itemValues(source.Child(source.Child(rec.root, "links"), "questions")) {
					linked[q] = true
				}
				found := false
				for _, q := range answers {
					found = found || linked[q]
				}
				if !found {
					c.add(recNode, ptr, RuleRecordCited, "%s answers %s, not %s, which %s answers; cite the record of the answer to %s, or link %s to it under questions", id, eitherOf(itemValues(source.Child(source.Child(rec.root, "links"), "questions"))), eitherOf(answers), path[1], eitherOf(answers), id)
					continue
				}
				if from, by := rec.str("from"), source.Str(source.Child(n, "decidedBy")); from != by {
					c.add(recNode, ptr, RuleRecordCited, "%s was said by %s, not %s, which %s names as decidedBy; a decision that answers a question is decided by the role that answered it: write decidedBy %s, or cite the record of the answer %s gave", id, from, by, path[1], from, by)
				}
			}
		}
	})
}

// typeArticle is a communication's type with its article, for a message.
func typeArticle(typ string) string {
	switch typ {
	case "answer", "objection", "information":
		return "an " + typ
	case "":
		return "of no type"
	}
	return "a " + typ
}

// eitherOf joins names with "or".
func eitherOf(items []string) string {
	switch len(items) {
	case 0:
		return "nothing"
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " or " + items[len(items)-1]
}

// checkUnheard warns, once the specification keeps who was heard, of a
// stakeholder that an accepted requirement needs, or that an approved
// specification's page is for, while no communication, session, interview
// or decision shows it was ever heard (ADR-220).
func (rc *recordChecks) checkUnheard(c *checker) {
	if len(rc.set["communication"])+len(rc.set["session"])+len(rc.set["test-session"]) == 0 {
		return
	}
	heard := map[string]bool{}
	for _, r := range rc.set["communication"] {
		heard[r.str("from")] = true
		heard[r.str("to")] = true
	}
	for _, kind := range []string{"session", "test-session"} {
		for _, r := range rc.set[kind] {
			for _, v := range itemValues(source.Child(r.root, "present")) {
				heard[v] = true
			}
		}
	}
	for _, src := range rc.d.sources {
		for _, v := range itemValues(source.Child(src, "participants")) {
			heard[v] = true
		}
	}
	for _, dec := range rc.d.decisions {
		heard[source.Str(source.Child(dec, "decidedBy"))] = true
	}
	relies := map[string][]string{} // role -> what relies on it, as named
	add := func(role, what string) {
		if role == "" || heard[role] || rc.d.stakeholders[role] == nil {
			return
		}
		for _, w := range relies[role] {
			if w == what {
				return
			}
		}
		relies[role] = append(relies[role], what)
	}
	for _, id := range sortedKeys(rc.d.requirements) {
		req := rc.d.requirements[id]
		if source.Str(source.Child(req, "status")) != "accepted" {
			continue
		}
		add(source.Str(source.Child(req, "ownedBy")), "requirement "+id)
		for _, needID := range itemValues(source.Child(req, "needs")) {
			for _, role := range itemValues(source.Child(rc.d.needs[needID], "stakeholders")) {
				add(role, "requirement "+id)
			}
		}
	}
	version := source.Str(source.Child(source.Child(rc.d.root, "info"), "version"))
	if rc.set.has("approval", version) {
		for _, name := range sortedKeys(rc.d.pages) {
			perm := source.Str(source.Child(rc.d.pages[name], "permission"))
			if perm == "" || perm == "public" || perm == permissionSignedIn {
				continue
			}
			for _, role := range sortedKeys(rc.d.roles) {
				if rc.d.roleHolds(rc.d.roles[role])[perm] {
					add(role, "page "+name)
				}
			}
		}
	}
	roles := make([]string, 0, len(relies))
	for role := range relies {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	for _, role := range roles {
		c.warn(rc.d.stakeholders[role], source.Pointer("stakeholders", role), RuleUnheard, "%s was never heard: no communication record names it, no session lists it present, no interview lists it and no decision is by it, while %s %s on it; hear it before acceptance, and record what it said with specarch log", role, joinAnd(relies[role]), relyVerb(len(relies[role])))
	}
}

func relyVerb(n int) string {
	if n == 1 {
		return "relies"
	}
	return "rely"
}
