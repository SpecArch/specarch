package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"
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

// The words that make a sentence a commitment (docs/extraction.md, step 6):
// a modal of obligation, a quantity, a frequency or a time limit.
var (
	modalWord     = regexp.MustCompile(`(?i)\b(shall|must|will|should)\b`)
	numberWords   = `one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|fifteen|twenty|thirty|forty|fifty|sixty|hundred|thousand`
	quantity      = regexp.MustCompile(`(?i)\b([0-9]+(?:[.,][0-9]+)?|` + numberWords + `)\s+[a-z]`)
	clockTime     = regexp.MustCompile(`\b[0-9]{1,2}:[0-9]{2}\b`)
	frequencyWord = regexp.MustCompile(`(?i)\b(once|twice)\b`)
	timeLimit     = regexp.MustCompile(`(?i)\b(within|before|after|until|no later than)\b`)
	pronounStart  = regexp.MustCompile(`(?i)^(it|they|this|these|that|those|he|she)\b`)
	sensitiveWord = regexp.MustCompile(`(?i)\b(personal|credentials?)\b`)
)

// The header of a table the reader holds: the fields of one entity. The
// first column is Field; the others are any of these, in any order.
var fieldColumns = map[string]bool{"field": true, "type": true, "required": true, "sensitivity": true, "description": true}

// The types a field table may name, and the field each becomes. A type
// with no row here is left for a question.
var tableTypes = map[string][]any{
	"text":      {"type", "string"},
	"string":    {"type", "string"},
	"date":      {"type", "string", "format", "date"},
	"date-time": {"type", "string", "format", "date-time"},
	"timestamp": {"type", "string", "format", "date-time"},
	"boolean":   {"type", "boolean"},
	"yes/no":    {"type", "boolean"},
	"email":     {"type", "string", "format", "email"},
}

var sensitivities = map[string]bool{"public": true, "internal": true, "personal": true, "credential": true}

// docClause is one clause of the document's outline: a heading, or a
// paragraph that starts with a section number.
type docClause struct {
	id, title string
	parent    *docClause
	children  int
	own       bool // holds text of its own
}

type docSentence struct {
	clause *docClause
	text   string
}

type docTable struct {
	clause *docClause
	line   int
	header []string
	rows   [][]string
}

// Documents reads one Markdown document into a tree: its outline as the
// clauses of one document source, every commitment sentence as a
// requirement, and every table with a known header as an entity's fields.
func Documents(path, out, key string) (*Result, error) {
	if ext := strings.ToLower(filepath.Ext(path)); ext != ".md" && ext != ".markdown" {
		return nil, refuse("%s is not a Markdown file; this build reads Markdown documents, and PDF and slide decks come in a later step", path)
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
	res := &Result{Tree: newTree()}
	commitLine(res, r)
	d := &docReader{key: key, res: res, questions: &yaml.Node{Kind: yaml.MappingNode}}
	d.parse(strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"))
	if line, text, ok := generatedMark(path); ok {
		res.say("generated: %s:%d says it is generated from another source: %s", r.Paths[0], line, text)
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
		if c.children == 0 || c.own {
			clauses = append(clauses, flow(mapping("clause", c.id, "title", c.title)))
		}
	}
	src := mapping("kind", "document", "title", title, "edition", r.Commit, "url", url, "clauses", clauses)
	var stages []string
	if d.requirements != nil || d.nextID > 0 {
		stages = append(stages, "requirements")
	}
	if d.entities != nil || d.designQuestions != nil {
		stages = append(stages, "design")
	}
	description := fmt.Sprintf("The document %s, read at commit %s: every heading and numbered section is a clause, every sentence that makes a commitment is a requirement citing its clause, and every table of fields is an entity's fields. What the document does not say is a question, and so is what the meta-model cannot hold.\n", r.Paths[0], r.Commit)
	res.Tree.put("specarch.yaml", rootFile(title, description, stages, mapping(key, src)))
	return res, nil
}

type docReader struct {
	key       string
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
}

// gap records what the meta-model cannot hold, at the clause the document
// says it in, or at its line before the first heading.
func (d *docReader) gap(at *docClause, line int, block string, format string, args ...any) {
	clause := fmt.Sprintf("line %d", line)
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

// parse reads the lines into clauses, sentences and tables, in order.
func (d *docReader) parse(lines []string) {
	var stack []*docClause // the open headings by level
	var levels []int
	var current *docClause
	var para []string
	flush := func() {
		if len(para) == 0 {
			return
		}
		text := strings.Join(para, " ")
		para = nil
		if m := paragraphNumber.FindStringSubmatch(text); m != nil && len(stack) > 0 {
			parent := stack[len(stack)-1]
			title := parent.title
			current = d.clause(m[1], title, parent)
			d.counts.numbered++
			text = m[2]
		}
		if current == nil {
			return
		}
		current.own = true
		for _, s := range sentences(text) {
			d.sentences = append(d.sentences, docSentence{clause: current, text: s})
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
			d.counts.codeBlocks++
			d.gap(current, start, "requirements", "a code block at line %d: code in a document is not read as a commitment", start)
		case mdHeading.MatchString(line):
			flush()
			m := mdHeading.FindStringSubmatch(line)
			level, text := len(m[1]), strings.TrimSpace(m[2])
			for len(levels) > 0 && levels[len(levels)-1] >= level {
				stack, levels = stack[:len(stack)-1], levels[:len(levels)-1]
			}
			var parent *docClause
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			id, title := text, text
			if n := headingNumber.FindStringSubmatch(text); n != nil {
				id, title = n[1], n[2]
			}
			if level == 1 && d.title == "" {
				d.title = text
			}
			current = d.clause(id, title, parent)
			d.counts.headings++
			stack, levels = append(stack, current), append(levels, level)
		case strings.HasPrefix(strings.TrimSpace(line), "|") && i+1 < len(lines) && tableRule.MatchString(lines[i+1]):
			flush()
			t := docTable{clause: current, line: i + 1, header: cells(line)}
			for i += 2; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|"); i++ {
				t.rows = append(t.rows, cells(lines[i]))
			}
			i--
			if current != nil {
				current.own = true
			}
			d.tables = append(d.tables, t)
			d.counts.tables++
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
}

// clause adds a clause to the outline; a clause named twice is one.
func (d *docReader) clause(id, title string, parent *docClause) *docClause {
	for _, c := range d.clauses {
		if c.id == id {
			d.gap(c, 0, "requirements", "the section %s is named twice; both are cited as one clause", id)
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
// priority: should for should, must for anything else.
func commitment(s string) (string, bool) {
	if m := modalWord.FindStringSubmatch(s); m != nil {
		if strings.EqualFold(m[1], "should") {
			return "should", true
		}
		return "must", true
	}
	if quantity.MatchString(s) || clockTime.MatchString(s) || frequencyWord.MatchString(s) || timeLimit.MatchString(s) {
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
		priority, ok := commitment(s.text)
		if !ok {
			continue
		}
		d.counts.commitments++
		cite := citation(d.key, s.clause.id, s.text)
		if pronounStart.MatchString(s.text) {
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
	d.res.say("counted %s, %s, %s (%s) and %s (%s): every ATX heading, every paragraph that starts with a section number such as 3.1, every sentence of a paragraph or list item, and every table, outside fenced code",
		plural(d.counts.headings, "heading"), plural(d.counts.numbered, "numbered paragraph"), plural(d.counts.sentences, "sentence"),
		plural(d.counts.commitments, "commitment"), plural(d.counts.tables, "table"), plural(d.counts.known, "table")+" of fields")
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
	for _, t := range d.tables {
		header := make([]string, len(t.header))
		known := len(t.header) > 1 && strings.EqualFold(t.header[0], "field")
		for i, h := range t.header {
			header[i] = strings.ToLower(h)
			known = known && fieldColumns[header[i]]
		}
		if !known || t.clause == nil {
			d.gap(t.clause, t.line, "entities", "the table at line %d, headed %s: only a table whose first column is Field, and whose others are Type, Required, Sensitivity or Description, is read", t.line, strings.Join(t.header, " | "))
			continue
		}
		d.counts.known++
		heading := t.clause
		name := namePascal(heading.title)
		if name == "" {
			d.gap(t.clause, t.line, "entities", "the table at line %d: the heading %q gives no entity name", t.line, heading.title)
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
			if tt, ok := tableTypes[strings.ToLower(col["type"])]; ok {
				for i := 0; i < len(tt); i += 2 {
					set(f, tt[i].(string), tt[i+1])
				}
			} else {
				d.gap(t.clause, t.line, "#/entities/"+name, "field %s of %s, at line %d, of type %q: the meta-model holds no type for it; left out", col["field"], name, t.line, col["type"])
				continue
			}
			if s := strings.ToLower(col["sensitivity"]); sensitivities[s] {
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
			if r := strings.ToLower(col["required"]); r == "yes" || r == "true" || r == "required" {
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
		m := sensitiveWord.FindStringSubmatch(s.text)
		if m == nil || !strings.Contains(strings.ToLower(s.text), strings.ToLower(field)) {
			continue
		}
		v := "personal"
		if strings.HasPrefix(strings.ToLower(m[1]), "credential") {
			v = "credential"
		}
		if found != "" && found != v {
			return "", false
		}
		found = v
	}
	return found, found != ""
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
