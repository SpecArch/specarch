package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/vocabulary"
	"github.com/SpecArch/specarch/internal/xlsx"
)

// TablesOptions are what extract tables is told beside the paths.
type TablesOptions struct {
	Key          string                 // the source key of a single file, or "" for the file's name
	Entity       string                 // the one register every file belongs to, or ""
	Template     string                 // the register's template: a file holding the header row alone, or ""
	Instructions string                 // the register's instructions: a table of fields, or ""
	Vocabulary   *vocabulary.Vocabulary // the words named with --language or --vocabulary, or nil for English
}

// Tables reads one or more data files (CSV, TSV or Excel), each a register
// with a header row and a record per row, into a tree: one document source
// per file, and one entity per register whose fields are its columns, with
// their types inferred from every value of every file of the register,
// unless a template or the instructions state them (ADR-215). What the
// values do not settle is a question, never a guess.
func Tables(paths []string, out string, opts TablesOptions) (*Result, error) {
	v := opts.Vocabulary
	if v == nil {
		v = vocabulary.English()
	}
	all := append([]string(nil), paths...)
	if opts.Template != "" {
		all = append(all, opts.Template)
	}
	if opts.Instructions != "" {
		all = append(all, opts.Instructions)
	}
	for _, p := range all {
		if _, _, ok := tabFormat(p); !ok {
			return nil, refuse("%s is not a data file this build reads: it reads %s, by the file's extension; save the register in one of these formats and read that file", p, tabFormats)
		}
	}
	r, err := Open(all)
	if err != nil {
		return nil, err
	}
	res := &Result{Tree: newTree()}
	commitLine(res, r)
	if opts.Vocabulary != nil {
		res.say("vocabulary: the words of %s, from %s", v.Language, v.Path)
	}
	t := &tabReader{res: res, v: v, out: out, read: r, questions: &yaml.Node{Kind: yaml.MappingNode}, cited: map[string]map[string]string{}}
	if opts.Vocabulary != nil {
		t.language = opts.Vocabulary.Language
	}
	keys := map[string]string{}
	read := func(path, role string) (*tabFile, error) {
		abs, err := absolute(path)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(r.Repository.Root, abs)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		f, err := readTabFile(path, filepath.ToSlash(rel), data, v)
		if err != nil {
			return nil, err
		}
		f.Key = kebab(strings.TrimSuffix(f.Name, filepath.Ext(f.Name)))
		if role == "" && opts.Key != "" && len(paths) == 1 {
			f.Key = opts.Key
		}
		if !sourceKeyForm.MatchString(f.Key) {
			return nil, fmt.Errorf("the file name %s gives no source key; name one with --source-key", f.Name)
		}
		if other, ok := keys[f.Key]; ok {
			return nil, fmt.Errorf("%s and %s both give the source key %s; one source is written per file, so rename one of them", other, path, f.Key)
		}
		keys[f.Key] = path
		for _, line := range f.Says {
			res.say("%s", line)
		}
		return f, nil
	}
	for _, p := range paths {
		f, err := read(p, "")
		if err != nil {
			return nil, err
		}
		t.files = append(t.files, f)
	}
	var template, instructions *tabFile
	if opts.Template != "" {
		if template, err = read(opts.Template, "template"); err != nil {
			return nil, err
		}
	}
	if opts.Instructions != "" {
		if instructions, err = read(opts.Instructions, "instructions"); err != nil {
			return nil, err
		}
	}
	if err := t.group(opts.Entity, template, instructions); err != nil {
		return nil, err
	}
	for _, reg := range t.registers {
		t.readRegister(reg)
	}
	t.write(opts.Entity, template, instructions)
	return res, nil
}

var sourceKeyForm = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// tabPlace is a column in one table.
type tabPlace struct {
	table *tabTable
	col   int // index in the table's header
}

// tabColumn is one column of a register across its files.
type tabColumn struct {
	name   string // camelCase, the field's name
	header string // as first written
	in     []tabPlace
	// stated is the column's rule from the template and the instructions,
	// or nil when the column is inferred alone.
	stated *tabRule
	// values and blanks are every cell of the column, file by file.
	values, blanks []tabCellAt
}

// tabCellAt is a cell with its table.
type tabCellAt struct {
	table *tabTable
	cell  tabCell
}

func (c tabCellAt) ref() string { return c.table.Ref(c.cell.Row, c.cell.Col) }

// tabRule is what the template and the instructions say of a column.
type tabRule struct {
	inTemplate  bool
	inRules     bool
	typeKey     string // the vocabulary's key, or "" when the instructions give no type
	typeWord    string // the word as written
	required    bool
	hasRequired bool // the instructions have a Required column
	sensitivity string
	description string
	values      []string
	row         tabCellAt // the instructions' row, for citations
}

// tabRegister is one register: its tables, its columns and what states them.
type tabRegister struct {
	name         string
	tables       []*tabTable
	columns      []*tabColumn
	byName       map[string]*tabColumn
	template     *tabTable
	instructions *tabTable
	candidates   []string
	why          []string
}

type tabReader struct {
	res       *Result
	v         *vocabulary.Vocabulary
	out       string
	read      *Read
	files     []*tabFile
	registers []*tabRegister
	language  string   // the vocabulary's language, when one was named
	lines     []string // the entity lines, printed after the counts
	entities  *yaml.Node
	questions *yaml.Node
	nextID    int
	notHeld   []notHeld
	cited     map[string]map[string]string // source key -> clause -> title
	counts    struct{ tables, rows, columns int }
}

// group puts every data table in its register: all in the one --entity
// names, or each in the register of its name, the sheet's or the file's.
// A table of fields is the instructions, and a template's table gives the
// columns; both apply to the one register of the run.
func (t *tabReader) group(entity string, template, instructions *tabFile) error {
	byName := map[string]*tabRegister{}
	var rules, templates []*tabTable
	if template != nil {
		for _, tb := range template.Tables {
			if tb.Instructions {
				rules = append(rules, tb)
			} else {
				templates = append(templates, tb)
			}
		}
		if len(templates) == 0 {
			return refuse("the template %s holds no register sheet: a template holds the header row of the register, and may hold a table of fields headed %s beside it", template.Name, t.v.HeaderWord(vocabulary.Field))
		}
		if len(templates) > 1 {
			return refuse("the template %s holds %d register sheets, %s; a template holds one header row, so a run reads one register with it", template.Name, len(templates), tableNames(templates))
		}
		if entity == "" {
			entity = namePascal(templates[0].Name)
		}
	}
	for _, f := range t.files {
		for _, tb := range f.Tables {
			if tb.Instructions {
				rules = append(rules, tb)
				continue
			}
			name := entity
			if name == "" {
				name = namePascal(tb.Name)
			}
			if name == "" {
				return refuse("%s gives no entity name: a name starts with a letter; name the register with --entity", tb.Where())
			}
			reg := byName[name]
			if reg == nil {
				reg = &tabRegister{name: name, byName: map[string]*tabColumn{}}
				byName[name] = reg
				t.registers = append(t.registers, reg)
			}
			reg.tables = append(reg.tables, tb)
		}
	}
	if len(t.registers) == 0 {
		return refuse("no register was read: every table is a table of fields, headed %s; give the data files as paths and the table of fields with --instructions", t.v.HeaderWord(vocabulary.Field))
	}
	if instructions != nil {
		for _, tb := range instructions.Tables {
			if tb.Instructions {
				rules = append(rules, tb)
			} else {
				t.notHeld = append(t.notHeld, notHeld{text: fmt.Sprintf("the table at %s of the instructions %s is headed %s, not %s, so it is no table of fields; it is not read", tb.Range(), instructions.Name, tb.Header[0].Value, t.v.HeaderWord(vocabulary.Field)), clause: tb.Range(), blocks: []string{"entities"}})
			}
		}
		if len(rules) == 0 {
			return refuse("the instructions %s hold no table of fields: its first column is headed %s, and the others %s, %s, %s, %s or %s", instructions.Name, t.v.HeaderWord(vocabulary.Field),
				t.v.HeaderWord(vocabulary.Type), t.v.HeaderWord(vocabulary.Required), t.v.HeaderWord(vocabulary.Sensitivity), t.v.HeaderWord(vocabulary.Description), t.v.HeaderWord(vocabulary.Values))
		}
	}
	if len(rules) > 1 {
		return refuse("%d tables of fields were given, at %s; a run reads one register, with one table of fields for it", len(rules), tableRanges(rules))
	}
	if (len(templates) > 0 || len(rules) > 0) && len(t.registers) > 1 {
		var names []string
		for _, reg := range t.registers {
			names = append(names, reg.name)
		}
		return refuse("a template and a table of fields apply to one register, and this run reads %d: %s; read each register on its own, or name the one register with --entity", len(t.registers), strings.Join(names, ", "))
	}
	if len(templates) == 1 {
		t.registers[0].template = templates[0]
		if n := len(templates[0].Rows); n > 0 {
			t.notHeld = append(t.notHeld, notHeld{text: fmt.Sprintf("the template %s holds %s under its header row; a template holds the header row alone, so they are not read", template.Name, plural(n, "row")), clause: templates[0].Range(), blocks: []string{"#/entities/" + t.registers[0].name}})
		}
	}
	if len(rules) == 1 {
		t.registers[0].instructions = rules[0]
	}
	return nil
}

func tableNames(ts []*tabTable) string {
	var out []string
	for _, tb := range ts {
		out = append(out, tb.Name)
	}
	return strings.Join(out, ", ")
}

func tableRanges(ts []*tabTable) string {
	var out []string
	for _, tb := range ts {
		out = append(out, tb.Range()+" of "+tb.File.Name)
	}
	return strings.Join(out, " and ")
}

// readRegister joins the columns of the register's tables, applies the
// template and the instructions, infers each column's type and writes the
// entity with its questions.
func (t *tabReader) readRegister(reg *tabRegister) {
	at := "#/entities/" + reg.name
	t.counts.tables += len(reg.tables)
	// The stated columns first, in the template's order, then the
	// instructions' fields the template lacks.
	if reg.template != nil {
		for i, h := range reg.template.Header {
			name := nameCamel(h.Value)
			if name == "" || reg.byName[name] != nil {
				t.headerQuestion(reg, reg.template, i, name)
				continue
			}
			col := &tabColumn{name: name, header: strings.TrimSpace(h.Value), stated: &tabRule{inTemplate: true}}
			reg.columns = append(reg.columns, col)
			reg.byName[name] = col
		}
	}
	if reg.instructions != nil {
		t.readInstructions(reg)
	}
	// Then the data tables' columns, in the order met.
	for _, tb := range reg.tables {
		seen := map[string]bool{}
		for i, h := range tb.Header {
			name := nameCamel(h.Value)
			if name == "" || seen[name] {
				t.headerQuestion(reg, tb, i, name)
				continue
			}
			seen[name] = true
			col := reg.byName[name]
			if col == nil {
				col = &tabColumn{name: name, header: strings.TrimSpace(h.Value)}
				reg.columns = append(reg.columns, col)
				reg.byName[name] = col
			}
			col.in = append(col.in, tabPlace{tb, i})
			for _, row := range tb.Rows {
				c := tabCellAt{tb, row[i]}
				if c.cell.Blank() {
					col.blanks = append(col.blanks, c)
				} else {
					col.values = append(col.values, c)
				}
			}
		}
		t.counts.rows += len(tb.Rows)
		for r, extra := range tb.Extra {
			if len(extra) == 0 {
				continue
			}
			var cells []string
			for _, c := range extra {
				cells = append(cells, fmt.Sprintf("%q at %s", c.Value, tb.Ref(c.Row, c.Col)))
			}
			what := "what is it? It is left out."
			if len(extra) > 1 {
				what = "what are they? They are left out."
			}
			t.question(reg, fmt.Sprintf("Row %d of %s holds %s beyond the %d columns of the header (%s); %s", tb.Rows[r][0].Row, tb.Where(), plural(len(extra), "value"), len(tb.Header), strings.Join(cells, ", "), what),
				"must", []string{at}, "A value under no header names no field, and the reader does not guess which it belongs to.", []*yaml.Node{t.cite(tb, -1)})
		}
	}
	t.counts.columns += len(reg.columns)
	props := &yaml.Node{Kind: yaml.MappingNode}
	var required []string
	for _, col := range reg.columns {
		f, isRequired := t.readColumn(reg, col)
		set(props, col.name, f)
		if isRequired {
			required = append(required, col.name)
		}
	}
	e := mapping("type", "object", "properties", props)
	if len(required) > 0 {
		set(e, "required", required)
	}
	// The primary key: a must question listing the candidates, the
	// required columns whose values are all distinct.
	var candidates []string
	for _, col := range reg.columns {
		if col.stated != nil && col.stated.inRules && col.stated.hasRequired && !col.stated.required {
			continue
		}
		if len(col.blanks) == 0 && len(col.in) == len(reg.tables) && len(col.values) > 0 && distinctValues(col.values) == len(col.values) {
			candidates = append(candidates, col.name)
		}
	}
	reg.candidates = candidates
	says := "none: no required column holds a different value in every row"
	if len(candidates) > 0 {
		says = strings.Join(candidates, ", ")
	}
	var cites []*yaml.Node
	for _, tb := range reg.tables {
		cites = append(cites, t.cite(tb, -1))
	}
	t.question(reg, fmt.Sprintf("Which fields identify one %s record, its primary key? Candidates, required and distinct in every row of every file: %s.", reg.name, says), "must", []string{at + "/primaryKey"},
		"A register does not say which of its columns identify a record; the candidates are the required columns whose values are all distinct.", cites)
	origin := "inferred"
	var why string
	if reg.template != nil || reg.instructions != nil {
		origin = "stated"
		var from []string
		if reg.template != nil {
			from = append(from, "the columns of the template "+reg.template.File.Name)
			cites = append([]*yaml.Node{t.cite(reg.template, -1)}, cites...)
		}
		if reg.instructions != nil {
			from = append(from, "the rules of the instructions at "+reg.instructions.Range()+" of "+reg.instructions.File.Name)
			cites = append([]*yaml.Node{t.cite(reg.instructions, -1)}, cites...)
		}
		why = fmt.Sprintf("The fields are %s; what they do not give is inferred from the values of %s: %s.", strings.Join(from, " and "), t.filesOf(reg), strings.Join(reg.why, "; "))
		if len(reg.why) == 0 {
			why = fmt.Sprintf("The fields are %s, and the values of %s fit them.", strings.Join(from, " and "), t.filesOf(reg))
		}
	} else {
		why = fmt.Sprintf("Inferred from the values of %s: %s.", t.filesOf(reg), strings.Join(reg.why, "; "))
	}
	set(e, "origin", origin)
	set(e, "why", why)
	set(e, "cites", cites)
	if t.entities == nil {
		t.entities = &yaml.Node{Kind: yaml.MappingNode}
	}
	set(t.entities, reg.name, e)
	keys := "no candidate key"
	if len(candidates) > 0 {
		keys = "candidate keys " + strings.Join(candidates, ", ")
	}
	rows := 0
	for _, tb := range reg.tables {
		rows += len(tb.Rows)
	}
	t.lines = append(t.lines, fmt.Sprintf("entity %s: %s from %s in %s; %s", reg.name, plural(len(reg.columns), "field"), plural(rows, "row"), t.filesOf(reg), keys))
}

// filesOf names the register's files with their row counts.
func (t *tabReader) filesOf(reg *tabRegister) string {
	if len(reg.tables) == 1 {
		return reg.tables[0].Where()
	}
	var out []string
	for _, tb := range reg.tables {
		out = append(out, fmt.Sprintf("%s (%s)", tb.Where(), plural(len(tb.Rows), "row")))
	}
	return joinAnd(out)
}

// headerQuestion asks about a header that names no field, or one that
// gives a name another column of the table has; the column is left out.
func (t *tabReader) headerQuestion(reg *tabRegister, tb *tabTable, i int, name string) {
	h := tb.Header[i]
	letter := xlsx.ColumnName(h.Col)
	at := "#/entities/" + reg.name
	rows := len(tb.Rows)
	if tb == reg.template {
		rows = 0
	}
	if name == "" {
		text := fmt.Sprintf("Column %s of %s has no header", letter, tb.Where())
		if strings.TrimSpace(h.Value) != "" {
			text = fmt.Sprintf("Column %s of %s is headed %q, which gives no field name", letter, tb.Where(), h.Value)
		}
		left := fmt.Sprintf("Its %s are left out.", plural(rows, "value"))
		if rows == 0 {
			left = "The column is left out."
		}
		t.question(reg, fmt.Sprintf("%s; what is the field? %s", text, left), "must", []string{at + "/properties"},
			"A field is named by its header, in camelCase, and a header with no letter in it names none.", []*yaml.Node{t.cite(tb, i)})
	} else {
		other := i
		for j, o := range tb.Header[:i] {
			if nameCamel(o.Value) == name {
				other = j
				break
			}
		}
		said := fmt.Sprintf("Columns %s and %s of %s are headed %q and %q", xlsx.ColumnName(tb.Header[other].Col), letter, tb.Where(), tb.Header[other].Value, h.Value)
		t.question(reg, fmt.Sprintf("%s, which both give the field name %s; the second is left out. What is it?", said, name), "must", []string{at + "/properties/" + name},
			"Two columns of one name would be one field, so the reader keeps the first and asks about the second.", []*yaml.Node{t.cite(tb, i)})
	}
}

// readInstructions reads the table of fields as the documents reader reads
// one: each row names a field, with the type, whether it is required, the
// sensitivity, the description and the allowed values its columns give.
func (t *tabReader) readInstructions(reg *tabRegister) {
	tb := reg.instructions
	v := t.v
	keyOf := make([]string, len(tb.Header))
	for i, h := range tb.Header {
		keyOf[i] = vocabulary.Key(v.Headers, h.Value)
		if keyOf[i] == "" || keyOf[i] == vocabulary.RowNumber && i > 0 {
			t.notHeld = append(t.notHeld, notHeld{text: fmt.Sprintf("the column %s of the table of fields at %s of %s is headed %q, which is none of %s, %s, %s, %s, %s and %s; it is not read", xlsx.ColumnName(h.Col), tb.Range(), tb.File.Name, h.Value,
				v.HeaderWord(vocabulary.Field), v.HeaderWord(vocabulary.Type), v.HeaderWord(vocabulary.Required), v.HeaderWord(vocabulary.Sensitivity), v.HeaderWord(vocabulary.Description), v.HeaderWord(vocabulary.Values)), clause: tb.ColumnRange(h.Col), blocks: []string{"#/entities/" + reg.name}})
		}
	}
	hasRequired := false
	for _, k := range keyOf {
		hasRequired = hasRequired || k == vocabulary.Required
	}
	for _, row := range tb.Rows {
		col := map[string]tabCell{}
		for i, k := range keyOf {
			if k != "" {
				col[k] = row[i]
			}
		}
		field := col[vocabulary.Field]
		name := nameCamel(field.Value)
		if name == "" {
			if !field.Blank() {
				t.notHeld = append(t.notHeld, notHeld{text: fmt.Sprintf("the field %q at %s of %s gives no field name; its row is not read", field.Value, tb.Ref(field.Row, field.Col), tb.File.Name), clause: tb.ColumnRange(field.Col), blocks: []string{"#/entities/" + reg.name + "/properties"}})
			}
			continue
		}
		c := reg.byName[name]
		if c == nil {
			c = &tabColumn{name: name, header: strings.TrimSpace(field.Value), stated: &tabRule{}}
			reg.columns = append(reg.columns, c)
			reg.byName[name] = c
		}
		if c.stated == nil {
			c.stated = &tabRule{}
		}
		rule := c.stated
		if rule.inRules {
			t.notHeld = append(t.notHeld, notHeld{text: fmt.Sprintf("the field %s is named twice in the table of fields at %s of %s, at %s; the second row is not read", name, tb.Range(), tb.File.Name, tb.Ref(field.Row, field.Col)), clause: tb.ColumnRange(field.Col), blocks: []string{"#/entities/" + reg.name + "/properties/" + name}})
			continue
		}
		rule.inRules, rule.hasRequired, rule.row = true, hasRequired, tabCellAt{tb, field}
		if ty, ok := col[vocabulary.Type]; ok && !ty.Blank() {
			rule.typeWord = strings.TrimSpace(ty.Value)
			rule.typeKey = vocabulary.Key(v.Types, ty.Value)
			if rule.typeKey == "" {
				t.notHeld = append(t.notHeld, notHeld{text: fmt.Sprintf("field %s of %s, at %s of %s, of type %q: the vocabulary names no type by that word, so the type is inferred from the values", name, reg.name, tb.Ref(ty.Row, ty.Col), tb.File.Name, ty.Value), clause: tb.ColumnRange(ty.Col), blocks: []string{"#/entities/" + reg.name + "/properties/" + name + "/type"}})
			}
		}
		if req, ok := col[vocabulary.Required]; ok {
			rule.required = vocabulary.Has(v.Yes, req.Value)
		}
		if s, ok := col[vocabulary.Sensitivity]; ok && !s.Blank() {
			rule.sensitivity = vocabulary.Key(v.Sensitivities, s.Value)
			if rule.sensitivity == "" {
				t.notHeld = append(t.notHeld, notHeld{text: fmt.Sprintf("field %s of %s, at %s of %s, of sensitivity %q: the vocabulary names no sensitivity by that word; left out", name, reg.name, tb.Ref(s.Row, s.Col), tb.File.Name, s.Value), clause: tb.ColumnRange(s.Col), blocks: []string{"#/entities/" + reg.name + "/properties/" + name + "/sensitivity"}})
			}
		}
		if d, ok := col[vocabulary.Description]; ok && !d.Blank() {
			rule.description = strings.TrimSpace(d.Value)
		}
		if vals, ok := col[vocabulary.Values]; ok && !vals.Blank() {
			rule.values = splitValues(vals.Value)
		}
	}
}

// splitValues reads a list of allowed values written in one cell: the
// values separated by a bar, a semicolon, a comma or a line break, each
// trimmed, given once.
func splitValues(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, v := range strings.FieldsFunc(s, func(r rune) bool { return r == '|' || r == ';' || r == ',' || r == '\n' || r == '\r' }) {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// The forms a value is read in.
var (
	integerForm      = regexp.MustCompile(`^-?(0|[1-9][0-9]{0,18})$`)
	decimalForm      = regexp.MustCompile(`^-?[0-9]+\.[0-9]+$`)
	exponentForm     = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?[eE][+-]?[0-9]+$`)
	dateForm         = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	dateTimeForm     = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}[T ][0-9]{2}:[0-9]{2}(:[0-9]{2}(\.[0-9]+)?)?(Z|[+-][0-9]{2}:[0-9]{2})?$`)
	ambiguousForm    = regexp.MustCompile(`^[0-9]{1,2}[./-][0-9]{1,2}[./-][0-9]{2,4}$`)
	emailForm        = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	uuidForm         = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	leadingZeroForm  = regexp.MustCompile(`^-?0[0-9]+$`)
	valueKindsListed = []string{"integer", "decimal", "date", "date-time", "boolean", "email", "uuid", "text", "date of two readings", "error"}
)

// valueKind is what one value reads as: integer, decimal, date,
// date-time, boolean, email, uuid, text, a date of two readings (day-month
// or month-day), or an error cell of a workbook. The value is given as a
// decimal when a workbook stores it with an exponent.
func valueKind(c tabCell) (kind, value string) {
	s := strings.TrimSpace(c.Value)
	switch c.Kind {
	case xlsx.Boolean:
		return "boolean", strings.ToLower(s)
	case xlsx.Error:
		return "error", s
	case xlsx.Date:
		if strings.Contains(s, "T") {
			return "date-time", s
		}
		return "date", s
	case xlsx.Number:
		if exponentForm.MatchString(s) {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				s = strconv.FormatFloat(f, 'f', -1, 64)
			}
		}
		if integerForm.MatchString(s) {
			return "integer", s
		}
		if decimalForm.MatchString(s) {
			return "decimal", s
		}
		return "text", s
	}
	switch {
	case strings.EqualFold(s, "true") || strings.EqualFold(s, "false"):
		return "boolean", strings.ToLower(s)
	case leadingZeroForm.MatchString(s):
		return "text", s
	case integerForm.MatchString(s):
		return "integer", s
	case decimalForm.MatchString(s):
		return "decimal", s
	case dateForm.MatchString(s):
		if _, err := time.Parse("2006-01-02", s); err == nil {
			return "date", s
		}
		return "text", s
	case dateTimeForm.MatchString(s):
		return "date-time", s
	case ambiguousForm.MatchString(s):
		return "date of two readings", s
	case emailForm.MatchString(s):
		return "email", s
	case uuidForm.MatchString(s):
		return "uuid", s
	}
	return "text", s
}

// inference is what a column's values settle.
type inference struct {
	typ, format      string
	precision, scale int
	enum             []string
	words            string                 // the type in the words of why
	kinds            map[string][]tabCellAt // the cells by what each reads as
	order            []string               // the kinds, most frequent first
	problem          string                 // mixed, empty or ambiguous, or ""
}

// infer reads every value of a column as one type: whole numbers an
// integer, int32 when every value fits and else carried as text; numbers
// with a decimal point, alone or with whole numbers, a decimal of the
// widest value's precision and scale; and so on by valueKind. A text
// column of at most ten distinct values, fewer than half as many as its
// values, is an enum of them.
func infer(values []tabCellAt) inference {
	in := inference{kinds: map[string][]tabCellAt{}}
	counts := map[string]int{}
	for _, c := range values {
		k, _ := valueKind(c.cell)
		in.kinds[k] = append(in.kinds[k], c)
		counts[k]++
	}
	if counts["integer"] > 0 && counts["decimal"] > 0 {
		in.kinds["decimal"] = append(in.kinds["decimal"], in.kinds["integer"]...)
		counts["decimal"] += counts["integer"]
		delete(in.kinds, "integer")
		delete(counts, "integer")
	}
	for _, k := range valueKindsListed {
		if counts[k] > 0 {
			in.order = append(in.order, k)
		}
	}
	sort.SliceStable(in.order, func(i, j int) bool { return counts[in.order[i]] > counts[in.order[j]] })
	set := func(typ, format, words string) {
		in.typ, in.format, in.words = typ, format, words
	}
	switch {
	case len(values) == 0:
		set("string", "", "text, since the column holds no value")
		in.problem = "empty"
	case len(in.order) == 1 && in.order[0] == "integer":
		fits := true
		for _, c := range values {
			_, s := valueKind(c.cell)
			if n, err := strconv.ParseInt(s, 10, 64); err != nil || n < -1<<31 || n > 1<<31-1 {
				fits = false
			}
		}
		if fits {
			set("integer", "int32", "integer")
		} else {
			set("string", "int64", "integer beyond 32 bits, carried as text")
		}
	case len(in.order) == 1 && in.order[0] == "decimal":
		intDigits, scale := 0, 0
		for _, c := range values {
			_, s := valueKind(c.cell)
			s = strings.TrimPrefix(s, "-")
			whole, frac, _ := strings.Cut(s, ".")
			whole = strings.TrimLeft(whole, "0")
			if whole == "" {
				whole = "0"
			}
			intDigits, scale = max(intDigits, len(whole)), max(scale, len(frac))
		}
		in.precision, in.scale = intDigits+scale, scale
		set("string", "decimal", fmt.Sprintf("decimal of precision %d and scale %d", in.precision, in.scale))
	case len(in.order) == 1:
		switch in.order[0] {
		case "date":
			set("string", "date", "date")
		case "date-time":
			set("string", "date-time", "date-time")
		case "boolean":
			set("boolean", "", "boolean")
		case "email":
			set("string", "email", "email")
		case "uuid":
			set("string", "uuid", "uuid")
		case "date of two readings":
			set("string", "", "text, since its dates read as day-month or as month-day")
			in.problem = "ambiguous"
		case "text":
			set("string", "", "text")
			distinct := distinctList(values)
			if len(distinct) <= 10 && len(distinct)*2 < len(values) {
				in.enum = distinct
				in.words = "one of " + joinAnd(quoteAll(distinct))
			}
		default:
			set("string", "", "text, since the values are "+in.order[0]+" cells")
			in.problem = "mixed"
		}
	default:
		set("string", "", "text, since the values are mixed")
		in.problem = "mixed"
	}
	return in
}

// distinctList is the distinct values, sorted.
func distinctList(values []tabCellAt) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range values {
		v := strings.TrimSpace(c.cell.Value)
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func distinctValues(values []tabCellAt) int { return len(distinctList(values)) }

// readColumn writes one field and its questions, and says whether the
// column is required.
func (t *tabReader) readColumn(reg *tabRegister, col *tabColumn) (*yaml.Node, bool) {
	at := "#/entities/" + reg.name + "/properties/" + col.name
	in := infer(col.values)
	f := mapping("title", col.header)
	rule := col.stated
	words := in.words
	var cites []*yaml.Node
	for _, p := range col.in {
		cites = append(cites, t.cite(p.table, p.col))
	}
	citeRule := func() []*yaml.Node {
		out := append([]*yaml.Node(nil), cites...)
		if rule != nil && rule.inRules {
			out = append([]*yaml.Node{t.cite(reg.instructions, -1)}, out...)
		} else if rule != nil && rule.inTemplate {
			out = append([]*yaml.Node{t.cite(reg.template, -1)}, out...)
		}
		return out
	}
	// The type: the instructions' word, else the values'.
	stated := ""
	if rule != nil && rule.typeKey != "" {
		stated = rule.typeKey
		switch stated {
		case "string":
			set(f, "type", "string")
			words = "text"
		case "date":
			set(f, "type", "string")
			set(f, "format", "date")
			words = "date"
		case "dateTime":
			set(f, "type", "string")
			set(f, "format", "date-time")
			words = "date-time"
		case "boolean":
			set(f, "type", "boolean")
			words = "boolean"
		case "email":
			set(f, "type", "string")
			set(f, "format", "email")
			words = "email"
		case "integer":
			set(f, "type", "integer")
			set(f, "format", "int32")
			words = "integer"
		case "decimal":
			if in.format == "decimal" || in.typ == "integer" || in.format == "int64" {
				precision, scale := in.precision, in.scale
				if in.format != "decimal" {
					precision, scale = digitsOf(col.values), 0
				}
				set(f, "type", "string")
				set(f, "format", "decimal")
				set(f, "precision", precision)
				set(f, "scale", scale)
				words = fmt.Sprintf("decimal of precision %d and scale %d", precision, scale)
			} else {
				set(f, "type", "string")
				words = "text until decided"
				t.question(reg, fmt.Sprintf("The instructions give %s of %s the type %s; which precision and scale? The values give none, so the field is kept as a string until decided.", col.name, reg.name, rule.typeWord), "must", []string{at + "/type"},
					"A decimal is written with its precision and scale, which a type word alone does not give.", citeRule())
			}
		}
		// The values against the stated type.
		var wrong []tabCellAt
		for _, c := range col.values {
			k, s := valueKind(c.cell)
			ok := false
			switch stated {
			case "string":
				ok = true
			case "date":
				ok = k == "date"
			case "dateTime":
				ok = k == "date-time"
			case "boolean":
				ok = k == "boolean"
			case "email":
				ok = k == "email"
			case "integer":
				n, err := strconv.ParseInt(s, 10, 64)
				ok = k == "integer" && err == nil && n >= -1<<31 && n <= 1<<31-1
			case "decimal":
				ok = k == "integer" || k == "decimal"
			}
			if !ok {
				wrong = append(wrong, c)
			}
		}
		if len(wrong) > 0 {
			t.question(reg, fmt.Sprintf("The instructions give %s of %s the type %s, and %s %s read as it: %s. Is the type wrong, or the values?", col.name, reg.name, rule.typeWord, plural(len(wrong), "value"), doesNot(len(wrong)), t.cellsOf(reg, wrong, 5)), "must", []string{at + "/type"},
				"A value that does not read as the stated type is either a wrong value or a wrong rule, and the reader does not choose.", citeRule())
		}
	} else {
		set(f, "type", in.typ)
		if in.format != "" {
			set(f, "format", in.format)
		}
		if in.format == "decimal" {
			set(f, "precision", in.precision)
			set(f, "scale", in.scale)
		}
		if in.enum != nil {
			set(f, "enum", in.enum)
		}
		switch in.problem {
		case "mixed":
			var said []string
			for _, k := range in.order {
				said = append(said, fmt.Sprintf("%s as %s", plural(len(in.kinds[k]), "value"), kindPlural(k, len(in.kinds[k]))))
			}
			var minority []tabCellAt
			for _, k := range in.order[1:] {
				minority = append(minority, in.kinds[k]...)
			}
			t.question(reg, fmt.Sprintf("Column %s of %s reads %s (%s); which type is the field %s? It is kept as a string until decided.", col.header, reg.name, joinAnd(said), t.cellsOf(reg, minority, 5), col.name), "must", []string{at + "/type"},
				"The values do not read as one type, and the reader does not choose between them; a string holds every value, so the field is read.", cites)
		case "empty":
			t.question(reg, fmt.Sprintf("Column %s of %s holds no value in %s; which type is the field %s? It is kept as a string until decided.", col.header, reg.name, plural(len(col.blanks), "row"), col.name), "must", []string{at + "/type"},
				"A column with no value gives nothing to infer a type from.", cites)
		case "ambiguous":
			t.question(reg, fmt.Sprintf("Column %s of %s holds dates such as %s that read as day-month-year or as month-day-year; which is it? The field %s is kept as a string until decided.", col.header, reg.name, t.cellsOf(reg, col.values, 3), col.name), "must", []string{at + "/type"},
				"A date written with slashes or dots reads two ways, and the reader does not choose; ISO 8601 (2026-10-31) reads one way.", cites)
		}
		if in.enum != nil {
			t.question(reg, fmt.Sprintf("Are %s the only values of %s in %s? %s %s these %d in %s.", joinAnd(quoteAll(in.enum)), col.name, reg.name, capitalise(t.filesPhrase(reg)), holds(len(reg.tables)), len(in.enum), plural(len(col.values), "value")), "should", []string{at + "/enum"},
				"A column of few distinct values is read as an enum of them, which the values suggest and do not settle.", cites)
			if len(reg.tables) > 1 {
				t.enumDiffers(reg, col, in, at, cites)
			}
		}
	}
	// The allowed values the instructions give, and the values against them.
	if rule != nil && len(rule.values) > 0 {
		set(f, "enum", rule.values)
		allowed := map[string]bool{}
		for _, v := range rule.values {
			allowed[v] = true
		}
		var outside []tabCellAt
		for _, c := range col.values {
			if !allowed[strings.TrimSpace(c.cell.Value)] {
				outside = append(outside, c)
			}
		}
		if len(outside) > 0 {
			t.question(reg, fmt.Sprintf("The instructions allow %s for %s of %s, and %s %s outside them: %s. Is the list incomplete, or are the values wrong?", joinAnd(quoteAll(rule.values)), col.name, reg.name, plural(len(outside), "value"), isAre(len(outside)), t.cellsOf(reg, outside, 5)), "must", []string{at + "/enum"},
				"A value outside the allowed values is either a wrong value or an incomplete list, and the reader does not choose.", citeRule())
		}
		if stated == "" {
			words = "one of " + joinAnd(quoteAll(rule.values))
		}
	}
	if rule != nil && rule.description != "" {
		set(f, "description", rule.description)
	}
	if rule != nil && rule.sensitivity != "" {
		set(f, "sensitivity", rule.sensitivity)
	}
	// Presence: a column some files lack, or the template or the
	// instructions name and no file has.
	if len(reg.tables) > 0 && len(col.in) < len(reg.tables) {
		var have, lack []string
		has := map[*tabTable]bool{}
		for _, p := range col.in {
			has[p.table] = true
			have = append(have, fmt.Sprintf("%s (column %s)", p.table.Where(), xlsx.ColumnName(p.table.Header[p.col].Col)))
		}
		for _, tb := range reg.tables {
			if !has[tb] {
				lack = append(lack, tb.Where())
			}
		}
		switch {
		case len(col.in) == 0 && rule != nil:
			from := "The template names"
			if rule.inRules {
				from = "The instructions name"
			}
			t.question(reg, fmt.Sprintf("%s the column %s of %s, and %s %s it; is the file incomplete, or is the column optional?", from, col.header, reg.name, joinAnd(lack), lacks(len(lack))), "must", []string{at},
				"A column the template or the instructions name and the file lacks is either a missing column or a rule to take out, and the reader does not choose.", citeRule())
		case rule != nil:
			t.question(reg, fmt.Sprintf("Is %s a field of %s? %s %s the column, and %s %s it.", col.name, reg.name, joinAnd(have), hasHave(len(have)), joinAnd(lack), lacks(len(lack))), "must", []string{at},
				"The files disagree on the columns, and the reader keeps the column and asks rather than drop it.", citeRule())
		default:
			t.question(reg, fmt.Sprintf("Is %s a field of %s? %s %s the column, and %s %s it.", col.name, reg.name, joinAnd(have), hasHave(len(have)), joinAnd(lack), lacks(len(lack))), "must", []string{at},
				"The files disagree on the columns, and the reader keeps the column and asks rather than drop it.", cites)
		}
	} else if reg.template != nil && (rule == nil || !rule.inTemplate) && len(col.in) > 0 {
		var have []string
		for _, p := range col.in {
			have = append(have, fmt.Sprintf("%s (column %s)", p.table.Where(), xlsx.ColumnName(p.table.Header[p.col].Col)))
		}
		t.question(reg, fmt.Sprintf("Is %s a column of %s? %s %s it, and the template %s does not.", col.name, reg.name, joinAnd(have), hasHave(len(have)), reg.template.File.Name), "must", []string{at},
			"A column the template lacks is either a column added since or a mistake, and the reader keeps it and asks.", citeRule())
	}
	// Required: no blank in any file; the instructions' word when they
	// have a Required column.
	required := false
	switch {
	case rule != nil && rule.inRules && rule.hasRequired:
		required = rule.required
		if required && len(col.blanks) > 0 {
			t.question(reg, fmt.Sprintf("The instructions require %s of %s, and %s %s it blank at %s; are those records incomplete, or is the field optional?", col.name, reg.name, t.filesPhrase(reg), leave(len(reg.tables)), t.cellsOf(reg, col.blanks, 5)), "must", []string{"#/entities/" + reg.name + "/required"},
				"A blank in a required column is either an incomplete record or a rule to take out, and the reader does not choose.", citeRule())
		}
	case len(col.in) == len(reg.tables) && len(col.in) > 0 && len(col.blanks) == 0:
		required = true
	case len(col.in) == len(reg.tables) && len(reg.tables) > 1 && len(col.blanks) > 0:
		full, some := []string{}, []string{}
		blanksIn := map[*tabTable][]tabCellAt{}
		for _, b := range col.blanks {
			blanksIn[b.table] = append(blanksIn[b.table], b)
		}
		for _, p := range col.in {
			if bs := blanksIn[p.table]; len(bs) == 0 {
				full = append(full, fmt.Sprintf("%s has no blank in %s", p.table.Where(), plural(len(p.table.Rows), "row")))
			} else {
				some = append(some, fmt.Sprintf("%s has %s (%s)", p.table.Where(), plural(len(bs), "blank"), cellsIn(bs, 3)))
			}
		}
		if len(full) > 0 {
			t.question(reg, fmt.Sprintf("Is %s of %s required? %s, and %s.", col.name, reg.name, joinAnd(full), joinAnd(some)), "should", []string{"#/entities/" + reg.name + "/required"},
				"The files disagree on whether the column is filled, so the field is left out of required and asked about.", cites)
		}
	}
	// The sentence for why.
	var places []string
	same := true
	for _, p := range col.in {
		letter := xlsx.ColumnName(p.table.Header[p.col].Col)
		if len(places) > 0 && !strings.HasPrefix(places[0], letter+" ") && places[0] != letter {
			same = false
		}
		places = append(places, letter+" of "+p.table.Where())
	}
	where := ""
	switch {
	case len(col.in) == 0:
		where = "no column"
	case same:
		where = "column " + xlsx.ColumnName(col.in[0].table.Header[col.in[0].col].Col)
	default:
		where = "columns " + joinAnd(places)
	}
	blanks := "no blank"
	if len(col.blanks) > 0 {
		blanks = plural(len(col.blanks), "blank")
	}
	distinct := ""
	if len(col.values) > 1 && distinctValues(col.values) == len(col.values) {
		distinct = ", all distinct"
	}
	if stated != "" {
		words += " as the instructions say"
	}
	reg.why = append(reg.why, fmt.Sprintf("%s %s, %s, %s%s, %s", col.name, words, plural(len(col.values), "value"), blanks, distinct, where))
	if len(f.Content) <= 8 {
		f = flow(f)
	}
	return f, required
}

// enumDiffers asks when a value of an inferred enum is in some files only.
func (t *tabReader) enumDiffers(reg *tabRegister, col *tabColumn, in inference, at string, cites []*yaml.Node) {
	byValue := map[string]map[*tabTable]int{}
	for _, c := range col.values {
		v := strings.TrimSpace(c.cell.Value)
		if byValue[v] == nil {
			byValue[v] = map[*tabTable]int{}
		}
		byValue[v][c.table]++
	}
	for _, v := range in.enum {
		if len(byValue[v]) == len(col.in) {
			continue
		}
		var only []string
		for _, p := range col.in {
			if n := byValue[v][p.table]; n > 0 {
				only = append(only, fmt.Sprintf("%s (%d of %s)", p.table.Where(), n, plural(len(p.table.Rows), "row")))
			}
		}
		t.question(reg, fmt.Sprintf("Is %q a value of %s in %s? Only %s %s it.", v, col.name, reg.name, joinAnd(only), holds(len(only))), "should", []string{at + "/enum"},
			"A value only some files hold is either new in a period or a mistake, and the reader keeps it in the enum and asks.", cites)
	}
}

func digitsOf(values []tabCellAt) int {
	n := 0
	for _, c := range values {
		_, s := valueKind(c.cell)
		s = strings.TrimLeft(strings.TrimPrefix(s, "-"), "0")
		n = max(n, max(len(s), 1))
	}
	return n
}

func kindPlural(kind string, n int) string {
	switch kind {
	case "text":
		return "text"
	case "date of two readings":
		if n == 1 {
			return "a date of two readings"
		}
		return "dates of two readings"
	case "error":
		if n == 1 {
			return "an error cell"
		}
		return "error cells"
	}
	if n == 1 {
		if strings.HasPrefix(kind, "integer") || strings.HasPrefix(kind, "email") || strings.HasPrefix(kind, "uuid") {
			return "an " + kind
		}
		return "a " + kind
	}
	return kind + "s"
}

// cellsIn names cells with their values, at most n, then how many more,
// in a sentence that names their file.
func cellsIn(cells []tabCellAt, n int) string {
	var out []string
	for i, c := range cells {
		if i == n {
			out = append(out, fmt.Sprintf("and %d more", len(cells)-n))
			break
		}
		if c.cell.Blank() {
			out = append(out, c.ref())
		} else {
			out = append(out, fmt.Sprintf("%q at %s", strings.TrimSpace(c.cell.Value), c.ref()))
		}
	}
	return strings.Join(out, ", ")
}

// cellsOf names cells with their values, at most n, then how many more;
// the file is named when the register has several.
func (t *tabReader) cellsOf(reg *tabRegister, cells []tabCellAt, n int) string {
	var out []string
	for i, c := range cells {
		if i == n {
			out = append(out, fmt.Sprintf("and %d more", len(cells)-n))
			break
		}
		at := c.ref()
		if len(reg.tables) > 1 {
			at += " of " + c.table.File.Name
		}
		if c.cell.Blank() {
			out = append(out, at)
		} else {
			out = append(out, fmt.Sprintf("%q at %s", strings.TrimSpace(c.cell.Value), at))
		}
	}
	return strings.Join(out, ", ")
}

func (t *tabReader) filesPhrase(reg *tabRegister) string {
	if len(reg.tables) == 1 {
		return "the file " + reg.tables[0].Where()
	}
	return "the files"
}

func doesNot(n int) string {
	if n == 1 {
		return "does not"
	}
	return "do not"
}

func isAre(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

func hasHave(n int) string {
	if n == 1 {
		return "has"
	}
	return "have"
}

func lacks(n int) string {
	if n == 1 {
		return "lacks"
	}
	return "lack"
}

func leave(n int) string {
	if n == 1 {
		return "leaves"
	}
	return "leave"
}

func holds(n int) string {
	if n == 1 {
		return "holds"
	}
	return "hold"
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// cite is a citation of a table (col -1) or of one of its columns, listed
// among its source's clauses.
func (t *tabReader) cite(tb *tabTable, col int) *yaml.Node {
	key := tb.File.Key
	if t.cited[key] == nil {
		t.cited[key] = map[string]string{}
	}
	if col < 0 {
		t.cited[key][tb.Range()] = tb.Name
		var headers []string
		for _, h := range tb.Header {
			headers = append(headers, strings.TrimSpace(h.Value))
		}
		says := fmt.Sprintf("A register of %s with the columns %s.", plural(len(tb.Rows), "row"), strings.Join(headers, ", "))
		switch {
		case tb.Instructions:
			says = fmt.Sprintf("A table of the fields: %s.", strings.Join(instructionFields(tb), ", "))
		case len(tb.Rows) == 0:
			says = fmt.Sprintf("The header row: %s.", strings.Join(headers, ", "))
		}
		return citation(key, tb.Range(), says)
	}
	h := tb.Header[col]
	clause := tb.ColumnRange(h.Col)
	title := strings.TrimSpace(h.Value)
	if title == "" {
		title = "Column " + xlsx.ColumnName(h.Col) + ", with no header"
	}
	t.cited[key][clause] = title
	values, blanks := 0, 0
	for _, row := range tb.Rows {
		if row[col].Blank() {
			blanks++
		} else {
			values++
		}
	}
	return citation(key, clause, fmt.Sprintf("The column %s: %s and %s.", strings.TrimSpace(h.Value), plural(values, "value"), plural(blanks, "blank")))
}

func instructionFields(tb *tabTable) []string {
	var out []string
	for _, row := range tb.Rows {
		if len(row) > 0 && !row[0].Blank() {
			out = append(out, strings.TrimSpace(row[0].Value))
		}
	}
	return out
}

func (t *tabReader) question(reg *tabRegister, text, priority string, blocks []string, why string, cites []*yaml.Node) {
	t.nextID++
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
	set(t.questions, fmt.Sprintf("Q-%d", t.nextID), q)
}

// write puts the sources, the entities and the questions into the tree
// and prints the counts.
func (t *tabReader) write(entity string, template, instructions *tabFile) {
	t.res.say("counted %s, %s, %s and %s: every file given, every register sheet or file as a table, its first row the header and every other row a record, and every header that names a field as a column, once per register",
		plural(len(t.files), "file"), plural(t.counts.tables, "table"), plural(t.counts.rows, "row"), plural(t.counts.columns, "column"))
	if tb := t.registers[0].template; tb != nil {
		t.res.say("template %s: the header row at %s, %s", tb.File.Name, tb.Range(), plural(len(tb.Header), "column"))
	}
	if tb := t.registers[0].instructions; tb != nil {
		t.res.say("instructions %s: the table of fields at %s, %s", tb.File.Name, tb.Range(), plural(len(tb.Rows), "field"))
	}
	for _, line := range t.lines {
		t.res.say("%s", line)
	}
	var files []*tabFile
	files = append(files, t.files...)
	if template != nil {
		files = append(files, template)
	}
	if instructions != nil {
		files = append(files, instructions)
	}
	for _, f := range files {
		for _, a := range f.Aside {
			a.source = f.Key
			t.notHeld = append(t.notHeld, a)
		}
	}
	for i := range t.notHeld {
		h := &t.notHeld[i]
		if h.source == "" {
			h.source = t.sourceOf(*h)
		}
		if t.cited[h.source] == nil {
			t.cited[h.source] = map[string]string{}
		}
		if _, ok := t.cited[h.source][h.clause]; !ok {
			t.cited[h.source][h.clause] = "Set aside; the question that cites it says why"
		}
	}
	questionsOf := oneQuestions(t.questions)
	t.res.say("wrote %s and %s: one per entity's primary key, one per column whose values do not settle its type, whether it is required or its values, or that some files or the template lack, one per header that names no field, and one per thing the files hold that is not read",
		plural(len(t.registers), "entity"), plural(t.nextID+couldCount(questionsOf, t.notHeld), "question"))
	askNotHeld(t.res, questionsOf, &t.nextID, "", t.notHeld)
	design := mapping("entities", t.entities)
	if len(t.questions.Content) > 0 {
		set(design, "questions", t.questions)
	}
	t.res.Tree.put("design/entities.yaml", design)
	t.res.Tree.put("requirements/requirements.yaml", mapping("stakeholders", ownerStakeholder()))
	sources := &yaml.Node{Kind: yaml.MappingNode}
	var names []string
	for _, reg := range t.registers {
		names = append(names, reg.name)
	}
	for _, f := range files {
		url, err := relativeURL(t.out, filepath.Join(t.read.Repository.Root, filepath.FromSlash(f.Rel)))
		if err != nil {
			url = f.Rel
		}
		var clauses []*yaml.Node
		cited := t.cited[f.Key]
		var order []string
		for _, tb := range f.Tables {
			if _, ok := cited[tb.Range()]; ok {
				order = append(order, tb.Range())
			}
		}
		var rest []string
		for clause := range cited {
			listed := false
			for _, o := range order {
				listed = listed || o == clause
			}
			if !listed {
				rest = append(rest, clause)
			}
		}
		sort.Strings(rest)
		for _, clause := range append(order, rest...) {
			clauses = append(clauses, flow(mapping("clause", clause, "title", cited[clause])))
		}
		src := mapping("kind", "document", "title", f.Title, "edition", t.read.Commit, "url", url)
		if t.language != "" {
			set(src, "language", t.language)
		}
		set(src, "clauses", clauses)
		set(sources, f.Key, src)
	}
	description := fmt.Sprintf("The registers in %s, read at commit %s: each file is a document source cited as a spreadsheet program shows it (C17, or Register!C17 in a workbook), each register an entity whose fields are its columns, their types inferred from every value of every file unless a template or the instructions state them, and what the values do not settle a question.\n", t.filesList(files), t.read.Commit)
	t.res.Tree.put("specarch.yaml", rootFile(strings.Join(names, ", "), description, []string{"requirements", "design"}, sources))
}

func (t *tabReader) filesList(files []*tabFile) string {
	var out []string
	for _, f := range files {
		out = append(out, f.Rel)
	}
	return joinAnd(out)
}

// sourceOf is the key of the file a thing not read names, found by the
// table its clause is in.
func (t *tabReader) sourceOf(h notHeld) string {
	for _, reg := range t.registers {
		tables := append([]*tabTable(nil), reg.tables...)
		if reg.template != nil {
			tables = append(tables, reg.template)
		}
		if reg.instructions != nil {
			tables = append(tables, reg.instructions)
		}
		for _, tb := range tables {
			if h.clause == tb.Range() || strings.HasPrefix(h.clause, strings.TrimSuffix(tb.Range(), xlsx.Ref(tb.LastRow, tb.LastCol))) && strings.Contains(h.text, tb.File.Name) {
				return tb.File.Key
			}
		}
	}
	if len(t.files) > 0 {
		return t.files[0].Key
	}
	return ""
}
