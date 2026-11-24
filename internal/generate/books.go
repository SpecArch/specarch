package generate

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/docx"
	"github.com/SpecArch/specarch/internal/mdbook"
	"github.com/SpecArch/specarch/internal/pdf"
	"github.com/SpecArch/specarch/internal/semver"
	"github.com/SpecArch/specarch/internal/spec"
)

// The books target (ADR-248) and the docx and pdf form of every Markdown
// document (ADR-249): each a book made from the Markdown the document
// targets write for the same specification, state, audience and language,
// with front matter of its own, written by internal/pdf or internal/docx
// from the one model internal/mdbook reads.

// bookDef is one book: the base name of its file, its title, and the
// documents it is made of, in order, with what each is called in the
// front matter.
type bookDef struct {
	name     string
	title    string
	targets  []string
	contains []string
}

func bookDefs() []bookDef {
	defs := []bookDef{
		{name: "business-requirements", title: tr("business requirements document"), targets: []string{"brd"}},
		{name: "requirements-specification", title: tr("requirements specification"), targets: []string{"requirements", "traceability"}},
		{name: "technical-specification", title: tr("technical specification"), targets: []string{"techspec", "deployment", "operations"}},
		{name: "test-document", title: tr("test document"), targets: []string{"testplan", "traceability"}},
	}
	for i := range defs {
		for _, t := range defs[i].targets {
			defs[i].contains = append(defs[i].contains, documentTitle(t))
		}
	}
	return defs
}

// BookNames are the base names of the files the books target writes, in
// order; each is written as <name>.pdf or <name>.docx.
var BookNames = []string{"business-requirements", "requirements-specification", "technical-specification", "test-document"}

// File is one file a target writes.
type File struct {
	Name string
	Data []byte
}

// Books writes the four books of the books target in format, pdf or docx.
func Books(format string, root *yaml.Node, relRoot string, impls []Implementation, state *State) ([]File, error) {
	if state == nil {
		state = &State{}
	}
	info := get(root, "info")
	var out []File
	for _, def := range bookDefs() {
		var parts []mdbook.Part
		for _, t := range def.targets {
			text, _ := Document(t, root, relRoot, impls, state)
			parts = append(parts, mdbook.Part{Name: t, Markdown: text})
		}
		title := fmt.Sprintf("%s: %s", str(info, "title"), def.title)
		data, err := writeBook(format, root, relRoot, state, title, def, parts)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %v", def.name, format, err)
		}
		out = append(out, File{Name: def.name + "." + format, Data: data})
	}
	return out, nil
}

// DocumentBook writes the Markdown document of target as a book of its own
// in format, pdf or docx: the front matter, the table of contents, the
// document and its problems and gaps, the same text the Markdown holds.
func DocumentBook(target, format string, root *yaml.Node, relRoot string, impls []Implementation, state *State) (File, error) {
	if state == nil {
		state = &State{}
	}
	text, _ := Document(target, root, relRoot, impls, state)
	title := firstHeading(text)
	if title == "" {
		title = target
	}
	what := strings.TrimPrefix(title, str(get(root, "info"), "title")+": ")
	def := bookDef{name: target, title: what, targets: []string{target}, contains: []string{what}}
	name := target + "." + format
	data, err := writeBook(format, root, relRoot, state, title, def, []mdbook.Part{{Name: target, Markdown: text}})
	if err != nil {
		return File{}, fmt.Errorf("%s: %v", name, err)
	}
	return File{Name: name, Data: data}, nil
}

// firstHeading is the text of the first heading of a Markdown document.
func firstHeading(text string) string {
	for _, b := range mdbook.Parse(text) {
		if b.Kind == mdbook.HeadingBlock {
			return mdbook.PlainText(b.Text)
		}
	}
	return ""
}

// writeBook writes one book in format with the writer of that format.
func writeBook(format string, root *yaml.Node, relRoot string, state *State, title string, def bookDef, parts []mdbook.Part) ([]byte, error) {
	header := fmt.Sprintf(tr("%s, version %s"), title, str(get(root, "info"), "version"))
	front := frontMatter(root, relRoot, state, title, def, format)
	ids := elementIDs(root)
	intro := tr("%d problems and gaps are marked in this book, each at its entry; each number links to the mark, and the mark's label links back here.")
	switch format {
	case "pdf":
		return pdf.Write(pdf.Options{
			Title: title, Header: header, Lang: activeLanguage(), FrontMatter: front, Parts: parts, IDs: ids, Slug: slug, IsMark: isMark,
			Texts: pdf.Texts{
				Contents:      tr("Contents"),
				FrontMatter:   tr("Front matter"),
				Problems:      tr("Problems and gaps"),
				ProblemsIntro: intro,
				NoProblems:    tr("No problem or gap is marked in this book."),
				OnPage:        tr("page %d"),
				PageOf:        tr("Page %d of %d"),
				Mermaid:       tr("Mermaid diagram, printed as its source, since it could not be drawn: %s."),
				Figures: pdf.Figures{
					Flowchart: tr("Flowchart of %s."),
					State:     tr("State diagram of %s."),
					Entity:    tr("Entity diagram of %s."),
					Sequence:  tr("Sequence diagram between %s."),
				},
			},
		})
	case "docx":
		return docx.Write(docx.Options{
			Title: title, Header: header, Lang: activeLanguage(), FrontMatter: front, Parts: parts, IDs: ids, Slug: slug, IsMark: isMark,
			Texts: docx.Texts{
				Contents:      tr("Contents"),
				FrontMatter:   tr("Front matter"),
				Problems:      tr("Problems and gaps"),
				ProblemsIntro: intro,
				NoProblems:    tr("No problem or gap is marked in this book."),
				PageOf:        tr("Page %d of %d"),
				Mermaid:       tr("Mermaid diagram, printed as its source: a Word document does not draw it; the pdf of this document draws it, and the html document draws the entity diagram and the workflows."),
			},
		})
	}
	return nil, fmt.Errorf("no writer for the format %s; the formats of a book are pdf and docx", format)
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
func frontMatter(root *yaml.Node, relRoot string, state *State, title string, def bookDef, format string) string {
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
	d.line(tr("| Contains | %s |"), cell(strings.Join(def.contains, "; ")))
	d.blank()
	if !dated {
		d.para(fmt.Sprintf(tr("**Gap:** version %s is not dated: it is not approved, and no release record of it gives a date. Approve it with specarch approve, or record its release under records/releases/ with its date, and the book takes that date."), version))
	}
	if format == "pdf" {
		d.para(tr("*Set in the Go fonts, copyright 2016 Bigelow & Holmes Inc., under the BSD 3-Clause licence of the Go project, embedded as subsets.*"))
	}
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
