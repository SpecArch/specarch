package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/vocabulary"
)

// The Markdown a document is read from: ATX headings, paragraphs whose
// first word is a section number, list items, tables and fenced code.
var (
	mdHeading       = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	headingNumber   = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)*)\.?\s+(.+)$`)
	paragraphNumber = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)+)\s+(.+)$`)
	listItem        = regexp.MustCompile(`^\s*(?:[-*+]|[0-9]+[.)])\s+(.*)$`)
	tableRule       = regexp.MustCompile(`^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?\s*$`)
	fence           = regexp.MustCompile("^\\s*(```|~~~)")
)

// docWords is a vocabulary made into the matchers the reader applies
// (docs/extraction.md, step 6): a commitment is a modal of obligation, a
// quantity, a time of day, a frequency or a time limit.
type docWords struct {
	v                                     *vocabulary.Vocabulary
	modal, quantity, clockTime, frequency *regexp.Regexp
	timeLimit, pronounStart, sensitive    *regexp.Regexp
}

func newDocWords(v *vocabulary.Vocabulary) *docWords {
	words := func(ws []string) *regexp.Regexp {
		return regexp.MustCompile(`(?i)` + vocabulary.Before + vocabulary.Words(ws) + vocabulary.After)
	}
	var seps []string
	for _, s := range v.TimeSeparators {
		seps = append(seps, regexp.QuoteMeta(s))
	}
	clock := `[^\s\S]`
	if len(seps) > 0 {
		clock = vocabulary.Before + `[0-9]{1,2}(?:` + strings.Join(seps, "|") + `)[0-9]{2}` + vocabulary.After
	}
	number := `([0-9]+(?:[.,][0-9]+)?`
	if len(v.Numbers) > 0 {
		number += `|` + vocabulary.Words(v.NumberWords())
	}
	return &docWords{
		v:            v,
		modal:        words(v.CommitmentWords()),
		quantity:     regexp.MustCompile(`(?i)` + vocabulary.Before + number + `)\s+\p{L}`),
		clockTime:    regexp.MustCompile(clock),
		frequency:    words(v.Frequencies),
		timeLimit:    words(v.TimeLimits),
		pronounStart: regexp.MustCompile(`(?i)^` + vocabulary.Words(v.Pronouns) + vocabulary.After),
		sensitive:    words(append(append([]string(nil), v.Sensitivities["personal"]...), v.Sensitivities["credential"]...)),
	}
}

// The types a field table may name, by the key a vocabulary lists their
// words under, and the field each becomes. A type with no word in the
// vocabulary is left for a question.
var tableTypes = map[string][]any{
	"string":   {"type", "string"},
	"date":     {"type", "string", "format", "date"},
	"dateTime": {"type", "string", "format", "date-time"},
	"boolean":  {"type", "boolean"},
	"email":    {"type", "string", "format", "email"},
}

// docClause is one clause of the document's outline: a heading, or a
// paragraph that starts with a section number.
type docClause struct {
	id, title string
	parent    *docClause
	children  int
	own       bool // holds text of its own
	cited     bool // listed only when a requirement or a question cites it
	used      bool // a requirement or a question cites it
}

type docSentence struct {
	clause *docClause
	text   string
}

type docTable struct {
	clause *docClause
	place  string
	header []string
	rows   [][]string
}

// DocumentOptions are what extract documents is told beside the path.
type DocumentOptions struct {
	Key          string                 // the source key, or "" for the file's name
	Vocabulary   *vocabulary.Vocabulary // the words named with --language or --vocabulary, or nil for English
	GivenOutside bool                   // the document was given to parties outside
}

// Documents reads one document (Markdown, PDF, Word, PowerPoint, Excel or
// an email message, docs/extraction.md) into a tree: its outline as the
// clauses of one document source, every commitment sentence as a
// requirement, and every table with a known header as an entity's fields,
// by the words of the vocabulary named, or English.
func Documents(path, out string, opts DocumentOptions) (*Result, error) {
	key := opts.Key
	format, ok := documentFormat(path)
	if !ok {
		return nil, refuse("%s is not a document this build reads: it reads %s, by the file's extension; save the document in one of these formats and read that file", path, documentFormatList())
	}
	r, err := Open([]string{path})
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if key == "" {
		key = kebab(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	}
	if !regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`).MatchString(key) {
		return nil, fmt.Errorf("the file name %s gives no source key; name one with --source-key", filepath.Base(path))
	}
	in, err := format.read(filepath.Base(path), data, 0)
	if err != nil {
		return nil, refuse("%s cannot be read as %s: %v", path, format.name, err)
	}
	res := &Result{Tree: newTree()}
	commitLine(res, r)
	v := opts.Vocabulary
	if v == nil {
		v = vocabulary.English()
	} else {
		res.say("vocabulary: the words of %s, from %s", v.Language, v.Path)
	}
	for _, line := range in.says {
		res.say("%s", line)
	}
	d := &docReader{key: key, res: res, questions: &yaml.Node{Kind: yaml.MappingNode}, words: newDocWords(v), counted: in.counted}
	d.read(in.blocks)
	if in.title != "" {
		d.title = in.title
	}
	if format.text {
		if line, text, ok := generatedMark(path); ok {
			res.say("generated: %s:%d says it is generated from another source: %s", r.Paths[0], line, text)
		}
	}
	d.write()

	url, err := relativeURL(out, filepath.Join(r.Repository.Root, filepath.FromSlash(r.Paths[0])))
	if err != nil {
		return nil, err
	}
	title := d.title
	if title == "" {
		title = filepath.Base(path)
	}
	var clauses []*yaml.Node
	for _, c := range d.clauses {
		if c.cited && !c.used {
			continue
		}
		if c.children == 0 || c.own {
			clauses = append(clauses, flow(mapping("clause", c.id, "title", c.title)))
		}
	}
	src := mapping("kind", "document", "title", title, "edition", r.Commit, "url", url)
	if opts.Vocabulary != nil {
		set(src, "language", opts.Vocabulary.Language)
	}
	if opts.GivenOutside {
		set(src, "givenOutside", true)
	}
	set(src, "clauses", clauses)
	var stages []string
	if d.requirements != nil || d.nextID > 0 {
		stages = append(stages, "requirements")
	}
	if d.entities != nil || d.designQuestions != nil {
		stages = append(stages, "design")
	}
	description := fmt.Sprintf("The document %s, read at commit %s: %s, every sentence that makes a commitment is a requirement citing its clause, and every table of fields is an entity's fields. What the document does not say is a question, and so is what the meta-model cannot hold.\n", r.Paths[0], r.Commit, in.clauses)
	res.Tree.put("specarch.yaml", rootFile(title, description, stages, mapping(key, src)))
	return res, nil
}

type docReader struct {
	key       string
	words     *docWords
	res       *Result
	title     string
	clauses   []*docClause
	sentences []docSentence
	tables    []docTable
	notHeld   []notHeld
	counts    struct{ headings, numbered, sentences, commitments, tables, known, codeBlocks int }

	requirements    *yaml.Node
	entities        *yaml.Node
	questions       *yaml.Node // about the requirements stage
	designQuestions *yaml.Node // about the design stage
	nextID          int
	counted         string // what the counts count, in the format's words
}

// gap records what the meta-model cannot hold, at the clause the document
// says it in, or at its place before the first clause.
func (d *docReader) gap(at *docClause, place string, block string, format string, args ...any) {
	clause := place
	if at != nil {
		clause = at.id
	}
	d.notHeld = append(d.notHeld, notHeld{text: fmt.Sprintf(format, args...), clause: clause, blocks: []string{block}})
}

// questionsFor is the questions of the stage a question's first block is
// in: the design stage for an entity, the requirements stage otherwise.
func (d *docReader) questionsFor(blocks []string) *yaml.Node {
	if strings.HasPrefix(blocks[0], "#/entities/") || blocks[0] == "entities" {
		if d.designQuestions == nil {
			d.designQuestions = &yaml.Node{Kind: yaml.MappingNode}
		}
		return d.designQuestions
	}
	return d.questions
}

// blockKind is what one block of a document is.
type blockKind int

const (
	blockHeading   blockKind = iota // a heading whose text may start with its number
	blockSection                    // a clause the format names itself: a page, a slide, a sheet, a cell, an attachment
	blockParagraph                  // a paragraph or a list item
	blockTable                      // a table, its header row first
	blockCode                       // a block of code
	blockNotHeld                    // something of the document the reader does not read
)

// docBlock is one block of a document as a reader of its format gives it.
// Every format's reader gives the same blocks, and one reader turns them
// into clauses, sentences and tables.
type docBlock struct {
	kind  blockKind
	level int    // heading and section: 1 for the top
	text  string // heading, paragraph and not held: the text
	id    string // section: the clause as citations name it
	title string // section: the clause's title
	rows  [][]string
	place string // where in the file: line 12, paragraph 4, page 2, slide 3, Sheet1!A1:C4
	// prefix goes before every clause the block names: an attachment's
	// name, so that its clauses stay apart from the message's.
	prefix string
	// plain: the paragraph's first word is never read as a section number
	// (a slide or a cell is its own clause).
	plain bool
	// cited: the section is listed only when a requirement or a question
	// cites it (a cell of a workbook).
	cited bool
}

// markdownBlocks reads Markdown lines into blocks: ATX headings,
// paragraphs, list items, tables and fenced code.
func markdownBlocks(lines []string) []docBlock {
	var out []docBlock
	var para []string
	flush := func() {
		if len(para) > 0 {
			out = append(out, docBlock{kind: blockParagraph, text: strings.Join(para, " ")})
			para = nil
		}
	}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case fence.MatchString(line):
			flush()
			open := fence.FindStringSubmatch(line)[1]
			start := i + 1
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), open); i++ {
			}
			out = append(out, docBlock{kind: blockCode, place: fmt.Sprintf("line %d", start)})
		case mdHeading.MatchString(line):
			flush()
			m := mdHeading.FindStringSubmatch(line)
			out = append(out, docBlock{kind: blockHeading, level: len(m[1]), text: strings.TrimSpace(m[2]), place: fmt.Sprintf("line %d", i+1)})
		case strings.HasPrefix(strings.TrimSpace(line), "|") && i+1 < len(lines) && tableRule.MatchString(lines[i+1]):
			flush()
			t := docBlock{kind: blockTable, place: fmt.Sprintf("line %d", i+1), rows: [][]string{cells(line)}}
			for i += 2; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|"); i++ {
				t.rows = append(t.rows, cells(lines[i]))
			}
			i--
			out = append(out, t)
		case strings.TrimSpace(line) == "":
			flush()
		case listItem.MatchString(line) && !paragraphNumber.MatchString(strings.TrimSpace(line)):
			flush()
			para = append(para, listItem.FindStringSubmatch(line)[1])
		default:
			para = append(para, strings.TrimSpace(line))
		}
	}
	flush()
	return out
}

// read turns the blocks into clauses, sentences and tables, in order.
func (d *docReader) read(blocks []docBlock) {
	var stack []*docClause // the open headings by level
	var levels []int
	var current *docClause
	open := func(level int, id, title string, b docBlock) {
		for len(levels) > 0 && levels[len(levels)-1] >= level {
			stack, levels = stack[:len(stack)-1], levels[:len(levels)-1]
		}
		var parent *docClause
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		current = d.clause(b.prefix+id, title, parent)
		current.cited = current.cited || b.cited
		stack, levels = append(stack, current), append(levels, level)
	}
	for _, b := range blocks {
		switch b.kind {
		case blockHeading:
			id, title := b.text, b.text
			if n := headingNumber.FindStringSubmatch(b.text); n != nil {
				id, title = n[1], n[2]
			}
			if b.level == 1 && d.title == "" && b.prefix == "" {
				d.title = b.text
			}
			open(b.level, id, title, b)
			d.counts.headings++
		case blockSection:
			open(b.level, b.id, b.title, b)
			if !b.cited {
				d.counts.headings++
			}
		case blockParagraph:
			text := b.text
			if m := paragraphNumber.FindStringSubmatch(text); m != nil && len(stack) > 0 && !b.plain {
				parent := stack[len(stack)-1]
				current = d.clause(b.prefix+m[1], parent.title, parent)
				d.counts.numbered++
				text = m[2]
			}
			if current == nil {
				continue
			}
			current.own = true
			for _, s := range sentences(text) {
				d.sentences = append(d.sentences, docSentence{clause: current, text: s})
			}
		case blockTable:
			t := docTable{clause: current, place: b.place, header: b.rows[0], rows: b.rows[1:]}
			if current != nil {
				current.own = true
			}
			d.tables = append(d.tables, t)
			d.counts.tables++
		case blockCode:
			d.counts.codeBlocks++
			d.gap(current, b.place, "requirements", "a code block at %s: code in a document is not read as a commitment", b.place)
		case blockNotHeld:
			d.gap(current, b.place, "requirements", "%s", b.text)
		}
	}
}

// clause adds a clause to the outline; a clause named twice is one.
func (d *docReader) clause(id, title string, parent *docClause) *docClause {
	for _, c := range d.clauses {
		if c.id == id {
			d.gap(c, "", "requirements", "the section %s is named twice; both are cited as one clause", id)
			return c
		}
	}
	c := &docClause{id: id, title: title, parent: parent}
	if parent != nil {
		parent.children++
	}
	d.clauses = append(d.clauses, c)
	return c
}

// sentences splits a paragraph at a full stop, a question or an
// exclamation mark followed by a space and a capital letter or a digit.
func sentences(text string) []string {
	var out []string
	rs := []rune(text)
	start := 0
	for i := 0; i < len(rs); i++ {
		if rs[i] != '.' && rs[i] != '!' && rs[i] != '?' {
			continue
		}
		j := i + 1
		for j < len(rs) && rs[j] == ' ' {
			j++
		}
		if j == len(rs) || (j > i+1 && (unicode.IsUpper(rs[j]) || unicode.IsDigit(rs[j]))) {
			if s := strings.TrimSpace(string(rs[start : i+1])); s != "" {
				out = append(out, s)
			}
			start = j
			i = j - 1
		}
	}
	if s := strings.TrimSpace(string(rs[start:])); s != "" {
		out = append(out, s)
	}
	return out
}

// cells splits a table row into its trimmed cells.
func cells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	var out []string
	for _, c := range strings.Split(line, "|") {
		out = append(out, strings.TrimSpace(c))
	}
	return out
}

// commitment says whether a sentence makes a commitment, and with what
// priority: should for a should word, must for anything else.
func (w *docWords) commitment(s string) (string, bool) {
	if m := w.modal.FindStringSubmatch(s); m != nil {
		return w.v.Priority(m[1]), true
	}
	if w.quantity.MatchString(s) || w.clockTime.MatchString(s) || w.frequency.MatchString(s) || w.timeLimit.MatchString(s) {
		return "must", true
	}
	return "", false
}

func (d *docReader) question(text, priority string, blocks []string, why string, cites []*yaml.Node) {
	d.nextID++
	q := mapping(
		"question", text,
		"kind", "decision",
		"priority", priority,
		"blocks", blocks,
		"decidedBy", owner,
		"why", why,
	)
	if len(cites) > 0 {
		set(q, "cites", cites)
	}
	set(d.questionsFor(blocks), fmt.Sprintf("Q-%d", d.nextID), q)
}

// prefix is the ID prefix of the requirements read: the source key in
// capitals, without its dashes, at most 16 characters.
func (d *docReader) prefix() string {
	p := strings.ToUpper(strings.ReplaceAll(d.key, "-", ""))
	if len(p) > 16 {
		p = p[:16]
	}
	if len(p) < 2 {
		p += "DOC"
	}
	return p
}

// write turns what was parsed into requirements, entities and questions,
// and prints the counts and what was not held.
func (d *docReader) write() {
	var ids []string
	for _, s := range d.sentences {
		d.counts.sentences++
		priority, ok := d.words.commitment(s.text)
		if !ok {
			continue
		}
		d.counts.commitments++
		cite := citation(d.key, s.clause.id, s.text)
		s.clause.used = true
		if d.words.pronounStart.MatchString(s.text) {
			d.question(fmt.Sprintf("Section %s says %q, and does not name what it commits; which element is it?", s.clause.id, s.text), priority,
				[]string{"requirements"}, "A sentence that names its subject only by a word such as it or they cannot be placed as one element without guessing.", []*yaml.Node{cite})
			continue
		}
		if d.requirements == nil {
			d.requirements = &yaml.Node{Kind: yaml.MappingNode}
		}
		id := fmt.Sprintf("%s-%d", d.prefix(), len(ids)+1)
		ids = append(ids, id)
		set(d.requirements, id, mapping(
			"statement", s.text,
			"priority", priority,
			"status", "accepted",
			"origin", "stated",
			"cites", []*yaml.Node{cite},
		))
	}
	if len(ids) > 0 {
		var blocks []string
		for _, id := range ids {
			blocks = append(blocks, "#/requirements/"+id+"/kind")
		}
		d.question(fmt.Sprintf("Which kind is each requirement read from the document: %s? Functional, quality, interface or constraint.", strings.Join(ids, ", ")), "must",
			blocks, "A document states what is committed, not which kind of requirement it is.", nil)
	}
	d.readTables()
	d.res.say("counted %s, %s, %s (%s) and %s (%s): %s",
		plural(d.counts.headings, "heading"), plural(d.counts.numbered, "numbered paragraph"), plural(d.counts.sentences, "sentence"),
		plural(d.counts.commitments, "commitment"), plural(d.counts.tables, "table"), plural(d.counts.known, "table")+" of fields", d.counted)
	entities := 0
	if d.entities != nil {
		entities = len(d.entities.Content) / 2
	}
	d.res.say("wrote %s, %s and %s: one per commitment that names its subject, one per table of fields, one per thing the document does not say, and one per thing the meta-model cannot hold",
		plural(len(ids), "requirement"), plural(entities, "entity"), plural(d.nextID+couldCount(d.questionsFor, d.notHeld), "question"))
	askNotHeld(d.res, d.questionsFor, &d.nextID, d.key, d.notHeld)
	if d.requirements != nil || d.nextID > 0 {
		req := mapping("stakeholders", ownerStakeholder())
		if d.requirements != nil {
			set(req, "requirements", d.requirements)
		}
		if len(d.questions.Content) > 0 {
			set(req, "questions", d.questions)
		}
		d.res.Tree.put("requirements/requirements.yaml", req)
	}
	if d.entities != nil || d.designQuestions != nil {
		design := mapping("entities", d.entities)
		if d.designQuestions != nil {
			set(design, "questions", d.designQuestions)
		}
		d.res.Tree.put("design/entities.yaml", design)
	}
}

// readTables writes every table of fields as an entity named after the
// heading of its section, in PascalCase as the database reader names a
// table, and every field in camelCase.
func (d *docReader) readTables() {
	v := d.words.v
	for _, t := range d.tables {
		t = withoutRowNumbers(v, t)
		header := make([]string, len(t.header))
		known := len(t.header) > 1 && vocabulary.Key(v.Headers, t.header[0]) == vocabulary.Field
		for i, h := range t.header {
			header[i] = vocabulary.Key(v.Headers, h)
			known = known && header[i] != "" && header[i] != vocabulary.RowNumber
		}
		if !known || t.clause == nil {
			d.gap(t.clause, t.place, "entities", "the table at %s, headed %s: only a table whose first column is %s, and whose others are %s, %s, %s or %s, is read", t.place, strings.Join(t.header, " | "),
				v.HeaderWord(vocabulary.Field), v.HeaderWord(vocabulary.Type), v.HeaderWord(vocabulary.Required), v.HeaderWord(vocabulary.Sensitivity), v.HeaderWord(vocabulary.Description))
			continue
		}
		d.counts.known++
		heading := t.clause
		name := namePascal(heading.title)
		if name == "" {
			d.gap(t.clause, t.place, "entities", "the table at %s: the heading %q gives no entity name", t.place, heading.title)
			continue
		}
		props := &yaml.Node{Kind: yaml.MappingNode}
		var required, unknown []string
		for _, row := range t.rows {
			col := map[string]string{}
			for i, h := range header {
				if i < len(row) {
					col[h] = row[i]
				}
			}
			field := nameCamel(col["field"])
			if field == "" {
				continue
			}
			f := &yaml.Node{Kind: yaml.MappingNode}
			if tt, ok := tableTypes[vocabulary.Key(v.Types, col["type"])]; ok {
				for i := 0; i < len(tt); i += 2 {
					set(f, tt[i].(string), tt[i+1])
				}
			} else {
				d.gap(t.clause, t.place, "#/entities/"+name, "field %s of %s, at %s, of type %q: the meta-model holds no type for it; left out", col["field"], name, t.place, col["type"])
				continue
			}
			if s := vocabulary.Key(v.Sensitivities, col["sensitivity"]); s != "" {
				set(f, "sensitivity", s)
			} else if sens, ok := d.sentenceSensitivity(col["field"]); ok {
				set(f, "sensitivity", sens)
			} else {
				unknown = append(unknown, field)
			}
			if desc := col["description"]; desc != "" {
				set(f, "description", desc)
			}
			set(props, field, flow(f))
			if vocabulary.Has(v.Yes, col["required"]) {
				required = append(required, field)
			}
		}
		says := fmt.Sprintf("A table of the fields of %s: %s.", heading.title, strings.Join(tableFields(t), ", "))
		e := mapping("type", "object", "properties", props)
		if len(required) > 0 {
			set(e, "required", required)
		}
		set(e, "origin", "stated")
		set(e, "cites", []*yaml.Node{citation(d.key, t.clause.id, says)})
		t.clause.used = true
		if d.entities == nil {
			d.entities = &yaml.Node{Kind: yaml.MappingNode}
		}
		set(d.entities, name, e)
		d.question(fmt.Sprintf("Which fields identify one %s, its primary key?", name), "must", []string{"#/entities/" + name + "/primaryKey"},
			"A table of fields does not say which of them identify a record.", nil)
		if len(unknown) > 0 {
			var blocks []string
			for _, f := range unknown {
				blocks = append(blocks, "#/entities/"+name+"/properties/"+f+"/sensitivity")
			}
			d.question(fmt.Sprintf("How sensitive is each of these fields of %s: %s? Public, internal, personal or credential.", name, strings.Join(unknown, ", ")), "should", blocks,
				"The document has no sensitivity column for them and no sentence that calls them personal or a credential.", nil)
		}
	}
}

// sentenceSensitivity finds the one sentence that names the field and
// calls it personal or a credential.
func (d *docReader) sentenceSensitivity(field string) (string, bool) {
	if field == "" {
		return "", false
	}
	found := ""
	for _, s := range d.sentences {
		m := d.words.sensitive.FindStringSubmatch(s.text)
		if m == nil || !strings.Contains(strings.ToLower(s.text), strings.ToLower(field)) {
			continue
		}
		v := vocabulary.Key(d.words.v.Sensitivities, m[1])
		if found != "" && found != v {
			return "", false
		}
		found = v
	}
	return found, found != ""
}

// withoutRowNumbers is a table without its leading column when the
// column's header is the vocabulary's row-number word: the column numbers
// the rows and says nothing about them.
func withoutRowNumbers(v *vocabulary.Vocabulary, t docTable) docTable {
	if len(t.header) < 2 || vocabulary.Key(v.Headers, t.header[0]) != vocabulary.RowNumber {
		return t
	}
	t.header = t.header[1:]
	var rows [][]string
	for _, r := range t.rows {
		if len(r) > 0 {
			r = r[1:]
		}
		rows = append(rows, r)
	}
	t.rows = rows
	return t
}

func tableFields(t docTable) []string {
	var out []string
	for _, row := range t.rows {
		if len(row) > 0 && row[0] != "" {
			out = append(out, row[0])
		}
	}
	return out
}

// words splits a name written in words, snake case or camel case.
func words(s string) []string {
	var out []string
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if unicode.IsUpper(r) && i > 0 && unicode.IsLower(rs[i-1]) && b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
			b.WriteRune(unicode.ToLower(r))
		default:
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

func namePascal(s string) string {
	var b strings.Builder
	for _, w := range words(s) {
		b.WriteString(strings.ToUpper(w[:1]) + w[1:])
	}
	out := b.String()
	if out != "" && !unicode.IsLetter(rune(out[0])) {
		return ""
	}
	return out
}

func nameCamel(s string) string {
	p := namePascal(s)
	if p == "" {
		return ""
	}
	return strings.ToLower(p[:1]) + p[1:]
}
