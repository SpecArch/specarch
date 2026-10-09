package extract

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/gensql"
	"github.com/SpecArch/specarch/internal/source"
)

// Value objects read back (ADR-077). The database reader sees columns: a
// field holding a schema in columns is address_street, address_city and
// address_postcode there. The merge writes each such field out as specarch
// generate sql does and compares that with the code side's fields, so it
// reads back exactly what generate sql writes and nothing else. A field one
// tree gives as a schema and another as an object is one kept as JSON.

// The keys a part and its column are compared on: what a column's type
// says. A key only the column gives is written into the part.
var columnKeys = []string{"type", "format", "maxLength", "minLength", "minimum", "maximum", "precision", "scale", "enum", "pattern"}

// voAsk is a question the merge asks about a value object, written after
// the questions on keys the trees disagree on.
type voAsk struct {
	q    *yaml.Node
	line string
}

// readBack is what answers the questions some trees asked on a pointer
// read back as a value object: the trees that asked, and what the line
// says answers them.
type readBack struct {
	askers map[int]bool
	says   string
}

// markRead records that the questions the askers asked on ptr, or below
// it, are answered by what says names.
func (m *merger) markRead(ptr string, askers []int, says string) {
	if m.readBack == nil {
		m.readBack = map[string]*readBack{}
	}
	r := &readBack{askers: map[int]bool{}, says: says}
	for _, ti := range askers {
		r.askers[ti] = true
	}
	m.readBack[ptr] = r
}

// readBackAnswers leaves out a tree's question whose every blocked key is
// at or below a pointer read back as a value object, when that tree asked
// it there: the field the trees give answers it.
func (m *merger) readBackAnswers(ti int, t *mergeTree, id string, blocks []*yaml.Node) bool {
	if len(blocks) == 0 || len(m.readBack) == 0 {
		return false
	}
	var read []string
	for ptr := range m.readBack {
		read = append(read, ptr)
	}
	sort.Strings(read)
	var ptrs, says []string
	for _, b := range blocks {
		var by *readBack
		for _, ptr := range read {
			if r := m.readBack[ptr]; r.askers[ti] && (b.Value == ptr || strings.HasPrefix(b.Value, ptr+"/")) {
				by = r
			}
		}
		if by == nil {
			return false
		}
		ptrs = append(ptrs, b.Value)
		if !contains(says, by.says) {
			says = append(says, by.says)
		}
	}
	m.res.say("read back: %s of %s, on %s, is left out; %s", id, t.s.Dir, strings.Join(ptrs, ", "), strings.Join(says, "; "))
	return true
}

// heldSchema names the schema a field holds and whether it is a list of
// them, as the design writes it.
func heldSchema(field *yaml.Node) (string, bool) {
	if ref, ok := strings.CutPrefix(source.Str(source.Child(field, "$ref")), "#/schemas/"); ok {
		return ref, false
	}
	if ref, ok := strings.CutPrefix(source.Str(source.Child(source.Child(field, "items"), "$ref")), "#/schemas/"); ok && typeOf(field) == "array" {
		return ref, true
	}
	return "", false
}

// typeOf is a field's type without null.
func typeOf(field *yaml.Node) string {
	t := source.Child(field, "type")
	if t == nil || t.Kind == yaml.ScalarNode {
		return source.Str(t)
	}
	for _, item := range source.Items(t) {
		if item.Value != "null" {
			return item.Value
		}
	}
	return ""
}

// anyObject is a field of type object with nothing more said: what the
// database reader writes for a JSON column.
func anyObject(field *yaml.Node) bool {
	if field == nil || field.Kind != yaml.MappingNode || typeOf(field) != "object" {
		return false
	}
	for _, k := range []string{"properties", "$ref", "items"} {
		if source.Child(field, k) != nil {
			return false
		}
	}
	return true
}

// jsonField makes a later tree's field fit the merged one when one gives a
// schema and the other an object: the field is that schema kept as JSON. It
// returns the field to merge, and changes the merged one in place.
func jsonField(acc, add *yaml.Node) *yaml.Node {
	switch {
	case anyObject(acc) && heldSchemaAny(add):
		deleteKey(acc, "type")
		for _, p := range source.Pairs(add) {
			if p.Key.Value == "$ref" || p.Key.Value == "type" || p.Key.Value == "items" {
				setKey(acc, p.Key.Value, copyNode(p.Value))
			}
		}
		if source.Child(add, "storage") == nil && source.Child(acc, "storage") == nil {
			setKey(acc, "storage", str("json"))
		}
	case heldSchemaAny(acc) && anyObject(add):
		add = copyNode(add)
		deleteKey(add, "type")
		if source.Child(acc, "storage") == nil {
			setKey(acc, "storage", str("json"))
		} else {
			setKey(add, "storage", str("json"))
		}
	}
	return add
}

func heldSchemaAny(field *yaml.Node) bool {
	name, _ := heldSchema(field)
	return name != ""
}

// decoded is a node as the generators read a specification.
func decoded(n *yaml.Node) map[string]any {
	var v map[string]any
	if n != nil && n.Decode(&v) == nil && v != nil {
		return v
	}
	return map[string]any{}
}

// mergedSchemas is every merged schema, as the generators read it.
func (m *merger) mergedSchemas() map[string]any {
	out := map[string]any{}
	for id, n := range m.entries {
		if name, ok := strings.CutPrefix(id, "schemas/"); ok {
			out[name] = decoded(n)
		}
	}
	return out
}

// readValueObjects reads back every entity field holding a schema in
// columns, and asks about columns that look like one value no tree names.
func (m *merger) readValueObjects() {
	schemas := m.mergedSchemas()
	if len(schemas) == 0 {
		return
	}
	for _, ptr := range append([]string{}, m.order...) {
		e := m.elements[ptr]
		if e == nil || e.section != "entities" || e.parent != "" {
			continue
		}
		name := e.tokens[1]
		node := m.entries["entities/"+name]
		m.jsonReadBack(name, node)
		for _, f := range source.Pairs(source.Child(node, "properties")) {
			if schema, list := heldSchema(f.Value); schema != "" && !list && schemas[schema] != nil {
				m.readColumns(name, node, f.Key.Value, f.Value, schema, schemas)
			}
		}
		m.unnamedValues(name, node, schemas)
	}
}

// jsonReadBack records, for each field one tree gives as an object and
// another as a schema, that the object's questions on it are answered.
func (m *merger) jsonReadBack(entity string, node *yaml.Node) {
	for _, f := range source.Pairs(source.Child(node, "properties")) {
		schema, _ := heldSchema(f.Value)
		if schema == "" {
			continue
		}
		steps := []mergeStep{{key: "entities"}, {key: entity}, {key: "properties"}, {key: f.Key.Value}}
		var askers []int
		var givers []string
		for ti, t := range m.trees {
			switch n := follow(t.s.Root, steps); {
			case anyObject(n):
				askers = append(askers, ti)
			case heldSchemaAny(n):
				givers = append(givers, t.title)
			}
		}
		if len(askers) > 0 && len(givers) > 0 {
			m.markRead("#"+source.Pointer("entities", entity, "properties", f.Key.Value), askers,
				fmt.Sprintf("%s.%s holds %s, kept as json, which %s %s", entity, f.Key.Value, schema, joinAnd(givers), giveOrGives(len(givers))))
		}
	}
}

// codeField is the code side's field of an entity by that name, or nil.
func (m *merger) codeField(entity, field string) *yaml.Node {
	e := m.elements[source.Pointer("entities", entity, "properties", field)]
	if e == nil || !m.onSide(e, codeSide) {
		return nil
	}
	return source.Child(source.Child(m.entries["entities/"+entity], "properties"), field)
}

// writtenOut is a field holding a schema written out in columns as
// generate sql writes it, alone in its entity.
func writtenOut(entity, field string, fd map[string]any, required bool, schemas map[string]any) gensql.WrittenOut {
	fd = copyMap(fd)
	delete(fd, "storage")
	e := map[string]any{"type": "object", "properties": map[string]any{field: fd}}
	if required {
		e["required"] = []any{field}
	}
	return gensql.WriteOut(entity, e, schemas)
}

func copyMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

// columnsOf is a written-out field's part columns in order.
func columnsOf(w gensql.WrittenOut) []string {
	var cols []string
	for col := range w.From {
		cols = append(cols, col)
	}
	sort.Strings(cols)
	return cols
}

// readColumns reads one field holding a schema back from the code side's
// columns, or asks why it cannot.
func (m *merger) readColumns(entity string, node *yaml.Node, field string, fn *yaml.Node, schema string, schemas map[string]any) {
	requiredList := source.Child(node, "required")
	isRequired := func(name string) bool {
		for _, r := range source.Items(requiredList) {
			if r.Value == name {
				return true
			}
		}
		return false
	}
	w := writtenOut(entity, field, decoded(fn), isRequired(field) && !nullableField(fn), schemas)
	cols := columnsOf(w)
	var have, missing []string
	for _, col := range cols {
		if m.codeField(entity, camel(col)) != nil {
			have = append(have, col)
		} else {
			missing = append(missing, w.From[col])
		}
	}
	if len(have) == 0 {
		return
	}
	at := source.Pointer("entities", entity, "properties", field)
	if source.Str(source.Child(fn, "storage")) == "json" {
		m.askColumns(entity, field, schema, have, []string{fmt.Sprintf("%s is kept as json, and the code has a column for its parts", field)})
		return
	}
	var diffs []string
	if len(missing) > 0 {
		diffs = append(diffs, fmt.Sprintf("the code has no column for %s", joinAnd(missing)))
	}
	add := map[string][]string{} // a column -> the keys only it gives
	for _, col := range have {
		cn := m.codeField(entity, camel(col))
		part := w.Fields[col]
		pn := &yaml.Node{}
		if err := pn.Encode(part); err != nil {
			continue
		}
		for _, k := range columnKeys {
			cv, pv := source.Child(cn, k), source.Child(pn, k)
			switch {
			case cv == nil:
			case source.Child(pn, "$ref") != nil:
				// A part that refers to an enum says its values there, and
				// the column's are not written beside the reference.
			case k == "type":
				if typeOf(cn) != typeOf(pn) {
					diffs = append(diffs, fmt.Sprintf("the column %s is of type %s, and %s of type %s", col, typeOf(cn), w.From[col], typeOf(pn)))
				}
			case pv == nil:
				add[col] = append(add[col], k)
			case !sameValue(cv, pv):
				diffs = append(diffs, fmt.Sprintf("the column %s has %s %s, and %s has %s", col, k, inline(cv), w.From[col], inline(pv)))
			}
		}
		notNull := isRequired(camel(col)) && !nullableField(cn)
		want := contains(w.Required, col)
		if notNull != want {
			diffs = append(diffs, fmt.Sprintf("the column %s is %s, and the field writes it %s", col, nullWord(notNull), nullWord(want)))
		}
	}
	// Every check naming a column must be a presence check of the field,
	// and every presence check of the field must be there.
	matched := map[string]bool{}
	var checks []string
	colField := map[string]bool{}
	for _, col := range have {
		colField[camel(col)] = true
	}
	for _, c := range source.Pairs(source.Child(node, "constraints")) {
		if source.Str(source.Child(c.Value, "kind")) != "check" {
			continue
		}
		text := source.Str(source.Child(c.Value, "expression"))
		if !namesAny(text, colField) {
			continue
		}
		form, ok := presenceForm(text, nil)
		found := ""
		for name, want := range w.Checks {
			if wf, _ := presenceForm(want, camel); ok && wf == form {
				found = name
			}
		}
		if found == "" {
			diffs = append(diffs, fmt.Sprintf("the check %s names a column of %s and is not the check that it is wholly absent or has its required parts", c.Key.Value, field))
			continue
		}
		matched[found] = true
		checks = append(checks, c.Key.Value)
	}
	var wanted []string
	for name := range w.Checks {
		wanted = append(wanted, name)
	}
	sort.Strings(wanted)
	for _, name := range wanted {
		if !matched[name] {
			diffs = append(diffs, fmt.Sprintf("the code has no check that %s is either wholly absent or has its required parts", valueOfCheck(w.Checks[name], w.From)))
		}
	}
	if len(diffs) > 0 {
		m.askColumns(entity, field, schema, have, diffs)
		return
	}
	// The columns are the field.
	props := source.Child(node, "properties")
	fe := m.elements[at]
	var givers []string
	for _, ti := range fe.trees {
		givers = append(givers, m.trees[ti].title)
	}
	says := fmt.Sprintf("the columns are read as %s.%s, which %s %s", entity, field, joinAnd(givers), giveOrGives(len(givers)))
	for _, c := range checks {
		var askers []int
		for _, col := range have {
			askers = append(askers, m.elements[source.Pointer("entities", entity, "properties", camel(col))].trees...)
		}
		m.markRead("#"+source.Pointer("entities", entity, "constraints", c), askers,
			fmt.Sprintf("the check %s is the one %s.%s writes, which %s %s", c, entity, field, joinAnd(givers), giveOrGives(len(givers))))
	}
	for _, col := range have {
		name := camel(col)
		m.writeIntoPart(schema, w.From[col], m.codeField(entity, name), add[col])
		ptr := source.Pointer("entities", entity, "properties", name)
		m.markRead("#"+ptr, m.elements[ptr].trees, says)
		for _, ti := range m.elements[ptr].trees {
			if !slicesContains(fe.trees, ti) {
				fe.trees = append(fe.trees, ti)
			}
		}
		delete(m.elements, ptr)
		m.order = without(m.order, ptr)
		deleteKey(props, name)
		dropItem(requiredList, name)
	}
	if requiredList != nil && len(requiredList.Content) == 0 {
		deleteKey(node, "required")
	}
	constraints := source.Child(node, "constraints")
	for _, c := range checks {
		deleteKey(constraints, c)
	}
	if constraints != nil && len(constraints.Content) == 0 {
		deleteKey(node, "constraints")
	}
	said := "the columns " + joinAnd(have)
	if len(checks) > 0 {
		said += " and the check " + joinAnd(checks)
	}
	m.res.say("value object: %s.%s, %s, read back from %s of the code", entity, field, schema, said)
}

// writeIntoPart writes into the merged schema's part the keys only its
// column gives. path is the part's path from the field, address.street.
func (m *merger) writeIntoPart(schema, path string, column *yaml.Node, keys []string) {
	if len(keys) == 0 {
		return
	}
	steps := strings.Split(path, ".")[1:]
	n := m.entries["schemas/"+schema]
	for i, s := range steps {
		part := source.Child(source.Child(n, "properties"), s)
		if i == len(steps)-1 {
			for _, k := range keys {
				setKey(part, k, copyNode(source.Child(column, k)))
			}
			return
		}
		inner, _ := heldSchema(part)
		n = m.entries["schemas/"+inner]
	}
}

// askColumns asks, at the field, whether the code's columns are it.
func (m *merger) askColumns(entity, field, schema string, cols []string, diffs []string) {
	at := "#" + source.Pointer("entities", entity, "properties", field)
	q := mapping(
		"question", fmt.Sprintf("Are the columns %s of %s the field %s, the schema %s kept in columns? %s.", joinAnd(cols), entity, field, schema, capital(strings.Join(diffs, "; "))),
		"kind", "decision",
		"priority", "must",
		"blocks", []string{at},
		"decidedBy", owner,
		"why", "specarch generate sql writes the field as these columns, and the code's differ from them. The merge reads columns as a field only where they are what the field writes, so it joins nothing here (ADR-077).",
	)
	m.citeEntity(q, entity, schema)
	m.voAsk = append(m.voAsk, voAsk{q: q, line: fmt.Sprintf("question Q-%%d (must): %s: the columns differ from the field", at)})
}

// unnamedValues asks about the code side's fields of an entity that are
// every part's column of a schema under one prefix no tree names.
func (m *merger) unnamedValues(entity string, node *yaml.Node, schemas map[string]any) {
	props := source.Child(node, "properties")
	var names []string
	for name := range schemas {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, schema := range names {
		// The ends of the column names, as a field named x gives them.
		sample := writtenOut(entity, "x", map[string]any{"$ref": "#/schemas/" + schema}, false, schemas)
		var ends []string
		for _, col := range columnsOf(sample) {
			ends = append(ends, strings.TrimPrefix(camel(col), "x"))
		}
		if len(ends) < 2 {
			continue
		}
		seen := map[string]bool{}
		for _, f := range source.Pairs(props) {
			for _, end := range ends {
				prefix, ok := strings.CutSuffix(f.Key.Value, end)
				if !ok || prefix == "" || seen[prefix] || !memberName.MatchString(prefix) || source.Child(props, prefix) != nil {
					continue
				}
				seen[prefix] = true
				w := writtenOut(entity, prefix, map[string]any{"$ref": "#/schemas/" + schema}, false, schemas)
				var fields []string
				all := true
				for _, col := range columnsOf(w) {
					if m.codeField(entity, camel(col)) == nil || m.onSide(m.elements[source.Pointer("entities", entity, "properties", camel(col))], documentsSide) {
						all = false
						break
					}
					fields = append(fields, camel(col))
				}
				if all {
					m.askUnnamed(entity, prefix, schema, fields)
				}
			}
		}
	}
}

func (m *merger) askUnnamed(entity, prefix, schema string, fields []string) {
	var blocks []string
	for _, f := range fields {
		blocks = append(blocks, "#"+source.Pointer("entities", entity, "properties", f))
	}
	q := mapping(
		"question", fmt.Sprintf("Are the fields %s of %s one value, a field %s holding the schema %s kept in columns, or fields of their own? They are the columns specarch generate sql writes for such a field.", joinAnd(fields), entity, prefix, schema),
		"kind", "decision",
		"priority", "should",
		"blocks", blocks,
		"decidedBy", owner,
		"why", fmt.Sprintf("Columns that share a prefix are often fields of their own, and the catalogue cannot tell; these are every part of %s, and no tree gives %s a field %s, so the merge asks rather than joins them (ADR-077).", schema, entity, prefix),
	)
	m.citeEntity(q, entity, schema)
	m.voAsk = append(m.voAsk, voAsk{q: q, line: fmt.Sprintf("question Q-%%d (should): #%s: the fields %s are the columns of a value %s", source.Pointer("entities", entity), joinAnd(fields), schema)})
}

// citeEntity cites, on a question, the entity and the schema as each tree
// that has them cites them.
func (m *merger) citeEntity(q *yaml.Node, entity, schema string) {
	cites := &yaml.Node{Kind: yaml.SequenceNode}
	for _, t := range m.trees {
		for _, steps := range [][]mergeStep{{{key: "entities"}, {key: entity}}, {{key: "schemas"}, {key: schema}}} {
			for _, c := range citesOn(t.s.Root, steps) {
				if !containsNode(cites, c) {
					cites.Content = append(cites.Content, copyNode(c))
				}
			}
		}
	}
	if len(cites.Content) > 0 {
		set(q, "cites", cites)
	}
}

// valueObjectQuestions writes the questions readValueObjects asked.
func (m *merger) valueObjectQuestions() int {
	for _, a := range m.voAsk {
		m.addQuestion(a.q, questionFile(a.q))
		m.res.say(a.line, len(m.questions))
	}
	return len(m.voAsk)
}

// nullableField says whether a field's type allows null.
func nullableField(field *yaml.Node) bool {
	for _, t := range source.Items(source.Child(field, "type")) {
		if t.Value == "null" {
			return true
		}
	}
	return false
}

func nullWord(notNull bool) string {
	if notNull {
		return "NOT NULL"
	}
	return "nullable"
}

func capital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// sameValue compares two scalars or lists by what they say.
func sameValue(a, b *yaml.Node) bool {
	return inline(a) == inline(b)
}

func without(list []string, s string) []string {
	out := list[:0:0]
	for _, item := range list {
		if item != s {
			out = append(out, item)
		}
	}
	return out
}

// dropItem removes a scalar from a sequence.
func dropItem(seq *yaml.Node, value string) {
	if seq == nil {
		return
	}
	var kept []*yaml.Node
	for _, item := range seq.Content {
		if item.Value != value {
			kept = append(kept, item)
		}
	}
	seq.Content = kept
}

// namesAny says whether an expression names one of the fields.
func namesAny(text string, fields map[string]bool) bool {
	n, errs := expr.Parse(text)
	if len(errs) > 0 || n == nil {
		return false
	}
	found := false
	var walk func(*expr.Node)
	walk = func(n *expr.Node) {
		if n.Op == expr.OpName && fields[n.Text] {
			found = true
		}
		for _, a := range n.Args {
			walk(a)
		}
	}
	walk(n)
	return found
}

// presenceForm writes an expression that is a presence check, ors of ands
// of name == null and name != null, in one order, each name renamed when
// rename is given; false when it is any other expression.
func presenceForm(text string, rename func(string) string) (string, bool) {
	n, errs := expr.Parse(text)
	if len(errs) > 0 || n == nil {
		return "", false
	}
	var ors []*expr.Node
	var flatten func(n *expr.Node, op string, into *[]*expr.Node)
	flatten = func(n *expr.Node, op string, into *[]*expr.Node) {
		if n.Op == op {
			for _, a := range n.Args {
				flatten(a, op, into)
			}
			return
		}
		*into = append(*into, n)
	}
	flatten(n, "||", &ors)
	var terms []string
	for _, or := range ors {
		var ands []*expr.Node
		flatten(or, "&&", &ands)
		var atoms []string
		for _, a := range ands {
			if (a.Op != "==" && a.Op != "!=") || len(a.Args) != 2 {
				return "", false
			}
			name, null := a.Args[0], a.Args[1]
			if name.Op == expr.OpNull {
				name, null = null, name
			}
			if name.Op != expr.OpName || null.Op != expr.OpNull {
				return "", false
			}
			text := name.Text
			if rename != nil {
				text = rename(text)
			}
			atoms = append(atoms, text+" "+a.Op+" null")
		}
		sort.Strings(atoms)
		terms = append(terms, strings.Join(atoms, " && "))
	}
	sort.Strings(terms)
	return strings.Join(terms, " || "), true
}

// valueOfCheck is the path of the value a presence check of generate sql
// is about: the parts its columns come from share it.
func valueOfCheck(text string, from map[string]string) string {
	var common []string
	first := true
	var cols []string
	presenceForm(text, func(col string) string {
		cols = append(cols, col)
		return col
	})
	for _, col := range cols {
		parts := strings.Split(from[col], ".")
		parts = parts[:len(parts)-1]
		if first {
			common, first = parts, false
			continue
		}
		n := 0
		for n < len(common) && n < len(parts) && common[n] == parts[n] {
			n++
		}
		common = common[:n]
	}
	return strings.Join(common, ".")
}
