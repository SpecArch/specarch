package generate

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/pdf"
	"github.com/SpecArch/specarch/internal/semver"
	"github.com/SpecArch/specarch/internal/spec"
)

// The pdf target (ADR-248): four books, each made from the Markdown the
// document targets write for the same specification, state, audience and
// language, with front matter of its own.

// book is one PDF the pdf target writes: its file, its title and the
// documents it is made of, in order.
type bookDef struct {
	file    string
	title   string
	targets []string
}

func bookDefs() []bookDef {
	return []bookDef{
		{"brd.pdf", tr("business requirements document"), []string{"brd"}},
		{"requirements.pdf", tr("requirements specification"), []string{"requirements", "traceability"}},
		{"technical.pdf", tr("technical specification"), []string{"techspec", "deployment", "operations"}},
		{"qa.pdf", tr("test document"), []string{"testplan", "traceability"}},
	}
}

// BookFiles are the files the pdf target writes, in order.
var BookFiles = []string{"brd.pdf", "requirements.pdf", "technical.pdf", "qa.pdf"}

// File is one file a target writes.
type File struct {
	Name string
	Data []byte
}

// Books writes the four books of the pdf target.
func Books(root *yaml.Node, relRoot string, impls []Implementation, state *State) ([]File, error) {
	if state == nil {
		state = &State{}
	}
	info := get(root, "info")
	ids := elementIDs(root)
	var out []File
	for _, def := range bookDefs() {
		var parts []pdf.Part
		for _, t := range def.targets {
			text, _ := Document(t, root, relRoot, impls, state)
			parts = append(parts, pdf.Part{Name: t, Markdown: text})
		}
		title := fmt.Sprintf("%s: %s", str(info, "title"), def.title)
		data, err := pdf.Write(pdf.Options{
			Title:       title,
			Lang:        activeLanguage(),
			FrontMatter: frontMatter(root, relRoot, state, title, def),
			Parts:       parts,
			IDs:         ids,
			Slug:        slug,
			IsMark:      isMark,
			Texts: pdf.Texts{
				Contents:      tr("Contents"),
				Problems:      tr("Problems and gaps"),
				ProblemsIntro: tr("%d problems and gaps are marked in this book, each at its entry; each number links to the mark, and the mark's label links back here."),
				NoProblems:    tr("No problem or gap is marked in this book."),
				OnPage:        tr("page %d"),
				PageOf:        tr("Page %d of %d"),
				Mermaid:       tr("Mermaid diagram, printed as its source: the PDF does not draw it, and the html document draws the entity diagram and the workflows."),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("%s: %v", def.file, err)
		}
		out = append(out, File{Name: def.file, Data: data})
	}
	return out, nil
}

// markLeads are the bold leads of the paragraphs that mark a problem or a
// gap, as the documents write them in the active language, cut before
// the first value.
func markLeads() []string {
	var out []string
	for _, f := range []string{
		"**Problem%s:** %s: %s: %s [%s]",
		"**Open question %s%s:** %s Decided by %s.",
		"**Open change request %s%s%s:** %s %s",
		"**Draft:** %s concerns this document (%s); see the open questions document, or run specarch gaps.",
		"**Gap:** %s",
		StakeholdersBanner,
	} {
		lead := strings.TrimPrefix(tr(f), "**")
		if i := strings.IndexAny(lead, "%*"); i >= 0 {
			lead = lead[:i]
		}
		out = append(out, strings.TrimRight(lead, " :"))
	}
	return out
}

// isMark says whether a paragraph whose bold lead is lead marks a problem
// or a gap. A Problem paragraph ends with its id in brackets, which tells
// it from the Problems notice that starts a document with the same word
// in some languages.
func isMark(lead, text string) bool {
	leads := markLeads()
	for i, l := range leads {
		if !strings.HasPrefix(lead, l) {
			continue
		}
		if i == 0 && !strings.HasSuffix(strings.TrimSpace(text), "]") {
			return false
		}
		return true
	}
	return false
}

// elementIDs are the keys of the specification's named sections that hold
// a digit or a hyphen, so that an ordinary word is never one; such a key
// in a book links to the heading or table row that describes it.
func elementIDs(root *yaml.Node) map[string]bool {
	out := map[string]bool{}
	for key := range spec.Sections {
		for _, p := range pairs(root, key) {
			k := p.Key.Value
			if strings.IndexFunc(k, func(r rune) bool { return unicode.IsDigit(r) || r == '-' }) >= 0 {
				out[k] = true
			}
		}
	}
	return out
}

// frontMatter is the Markdown of a book's front matter: what the book is,
// the specification's version, date and status, its owners, what it was
// made from, and the change history from the release records.
func frontMatter(root *yaml.Node, relRoot string, state *State, title string, def bookDef) string {
	d := &doc{}
	info := get(root, "info")
	version := str(info, "version")
	d.line("# %s", title)
	d.blank()
	date, dated := bookDate(version, state)
	status := state.Approval
	if status == "" {
		status = tr("not approved")
	}
	if ids := holding(questionsOf(root), bookReads(def)); len(ids) > 0 {
		status += "; " + fmt.Sprintf(tr("a draft: %s concern it (%s)"), countText(len(ids), tr("open question"), tr("open questions")), strings.Join(ids, ", "))
	}
	var titles []string
	for _, t := range def.targets {
		titles = append(titles, documentTitle(t))
	}
	d.line(tr("| Item | Value |"))
	d.line("|---|---|")
	d.line(tr("| Document | %s |"), cell(def.title))
	d.line(tr("| Specification | %s |"), cell(str(info, "title")))
	d.line(tr("| Version | %s |"), cell(version))
	d.line(tr("| Date | %s |"), cell(date))
	d.line(tr("| Status | %s |"), cell(status))
	if owners := strs(info, "owners"); len(owners) > 0 {
		d.line(tr("| Owners | %s |"), cell(strings.Join(owners, ", ")))
	}
	d.line(tr("| Made from | %s, meta-model %s |"), cell(relRoot), cell(str(root, "specarch")))
	d.line(tr("| Contains | %s |"), cell(strings.Join(titles, "; ")))
	d.blank()
	if !dated {
		d.para(fmt.Sprintf(tr("**Gap:** version %s is not dated: it is not approved, and no release record of it gives a date. Approve it with specarch approve, or record its release under records/releases/ with its date, and the book takes that date."), version))
	}
	d.para(tr("*Set in the Go fonts, copyright 2016 Bigelow & Holmes Inc., under the BSD 3-Clause licence of the Go project, embedded as subsets.*"))
	d.heading(2, tr("Change history"))
	releases := recordsOf(state, "release")
	sort.SliceStable(releases, func(i, j int) bool {
		a, _ := semver.Parse(str(releases[i], "version"))
		b, _ := semver.Parse(str(releases[j], "version"))
		return semver.Compare(a, b) > 0
	})
	if len(releases) == 0 {
		d.para(fmt.Sprintf(tr("No release is recorded beside the specification, so version %s is the only version this book knows. A release record under records/releases/ adds a row here."), version))
		return d.String()
	}
	d.para(tr("The releases recorded beside the specification, newest first, with the change requests and defects each includes."))
	d.line(tr("| Release | Specification version | Status | Date | Includes |"))
	d.line("|---|---|---|---|---|")
	for _, r := range releases {
		d.line("| %s | %s | %s | %s | %s |", cell(str(r, "version")), cell(str(r, "specificationVersion")), cell(str(r, "status")), cell(str(r, "date")), cell(strings.Join(strs(r, "includes"), ", ")))
	}
	d.blank()
	return d.String()
}

// bookDate is the date of a version: the approval's when it is approved,
// else the date of a release record of that version.
func bookDate(version string, state *State) (string, bool) {
	if state.Approved && state.ApprovedOn != "" {
		return state.ApprovedOn, true
	}
	for _, r := range recordsOf(state, "release") {
		v := str(r, "specificationVersion")
		if v == "" {
			v = str(r, "version")
		}
		if v == version && str(r, "date") != "" {
			return str(r, "date"), true
		}
	}
	return tr("not dated"), false
}

// bookReads are the sections the documents of a book read.
func bookReads(def bookDef) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range def.targets {
		for _, s := range documentReads[t] {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}

// documentTitle is what a document target is called in a book's front
// matter.
func documentTitle(target string) string {
	switch target {
	case "brd":
		return tr("business requirements document")
	case "requirements":
		return tr("requirements specification")
	case "traceability":
		return tr("traceability matrix")
	case "techspec":
		return tr("technical specification")
	case "deployment":
		return tr("deployment guide")
	case "operations":
		return tr("operations guide")
	case "testplan":
		return tr("test plan")
	}
	return target
}
