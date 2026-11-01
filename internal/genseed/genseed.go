// Package genseed writes the development seed of a specification: the
// logic of the specarch-gen-seed plug-in. It fills the tables
// specarch-gen-sql writes with made-up rows drawn from the entities (their
// field types and bounds, enums, examples, keys, unique and check
// constraints and relations), in the order the foreign keys need, from a
// fixed seed, so the file is the same bytes on every run and generate
// --check holds. Nothing in a row is about a real person: text names the
// field and the row, addresses lie in the ranges reserved for
// documentation. The file is loaded by the project; the plug-in never
// touches a database.
package genseed

import (
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/gensql"
	"github.com/SpecArch/specarch/internal/mark"
)

// Request is what specarch writes on the plug-in's standard input: the
// request specarch-gen-sql takes, since the rows go into its tables.
type Request = gensql.Request

// Decode reads a request, keeping numbers as their text.
func Decode(data []byte) (*Request, error) { return gensql.Decode(data) }

// The plug-in's name, as its header and its messages give it.
const name = "specarch-gen-seed"

// FileName is the one file the plug-in writes.
const FileName = "seed.sql"

// defaultRows is how many rows an entity gets when the settings name none.
const defaultRows = 10

// attempts is how many rows are drawn for one before the entity's
// constraints are reported as unsatisfiable.
const attempts = 100

// optionalOnceIn is how often an optional value is left NULL: once in
// this many rows, so both states are seeded.
const optionalOnceIn = 4

// base is the instant the seeded dates count from.
var base = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

type gen struct {
	spec     map[string]any
	root     string
	impl     gensql.Implementation
	dialect  string
	model    *gensql.Model
	tables   map[string]gensql.Table // by entity
	byTable  map[string]string       // the entity of each table
	counts   map[string]int64        // the rows asked for, by entity
	seed     uint64
	rnd      *random
	zone     *time.Location
	diags    []gensql.Diagnostic
	placed   map[string][]mark.Problem
	made     map[string][]row  // the rows of each entity written so far
	out      map[string]string // why an entity gets no rows, by entity
	order    []string
	reported map[string]bool // the pointers reported, so a field is said once
	checks   map[string][]check
}

// row is one row: its values by column.
type row struct {
	values map[string]value
	fields map[string]expr.Value // by the design's field, for the checks
}

// check is one check constraint of an entity, parsed.
type check struct {
	name, message string
	tree          *expr.Node
}

// Generate writes the seed of a request.
func Generate(r *Request) gensql.Response {
	g := &gen{spec: r.Specification, root: r.Root, tables: map[string]gensql.Table{}, byTable: map[string]string{}, counts: map[string]int64{},
		made: map[string][]row{}, out: map[string]string{}, reported: map[string]bool{}, checks: map[string][]check{}, zone: time.UTC}
	if tz := text(g.spec["timeZone"]); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			g.zone = loc
		}
	}
	for _, i := range r.Implementations {
		if _, ok := obj(obj0(i.Content["targets"])[r.Target]); ok || len(r.Implementations) == 1 {
			g.impl = i
			break
		}
	}
	if !g.settle(r) {
		return g.response(nil)
	}
	g.place(r)
	g.readSettings()
	if len(g.diags) > 0 {
		return g.response(nil)
	}
	g.orderEntities()
	for _, e := range g.order {
		g.fill(e)
	}
	return g.response([]gensql.File{{Path: FileName, Content: g.write(r)}})
}

// settle reads the dialect and the tables: the seed target's dialect, the
// sql target's when it names none, PostgreSQL otherwise, and the two must
// agree, since the rows go into the tables the migrations build.
func (g *gen) settle(r *Request) bool {
	targets := obj0(g.impl.Content["targets"])
	seed := obj0(targets[r.Target])
	sql := text(obj0(targets["sql"])["dialect"])
	g.dialect = text(seed["dialect"])
	if g.dialect == "" {
		g.dialect = sql
	}
	if g.dialect == "" {
		g.dialect = "postgresql"
	}
	if sql != "" && g.dialect != sql {
		g.problem("/", "the seed target names the dialect %s and the sql target %s; the rows load into the tables the migrations build, so both are one dialect: name it on the sql target alone, or the same on both", g.dialect, sql)
		return false
	}
	// The tables come from gensql, which reads the dialect from the
	// target of the request, so the one settled here is set on it.
	content := maps.Clone(g.impl.Content)
	ts := maps.Clone(targets)
	st := maps.Clone(seed)
	if st == nil {
		st = map[string]any{}
	}
	st["dialect"] = g.dialect
	if ts == nil {
		ts = map[string]any{}
	}
	ts[r.Target] = st
	content["targets"] = ts
	sr := *r
	sr.Implementations = make([]gensql.Implementation, len(r.Implementations))
	for i, impl := range r.Implementations {
		sr.Implementations[i] = impl
		if impl.File == g.impl.File {
			sr.Implementations[i].Content = content
		}
	}
	model, diags := gensql.ModelOf(&sr)
	if len(diags) > 0 {
		for _, d := range diags {
			d.Message = strings.Replace(d.Message, "specarch-gen-sql", name, 1)
			g.diags = append(g.diags, d)
		}
		return false
	}
	g.model = model
	for _, t := range model.Tables {
		g.tables[t.Entity] = t
		g.byTable[t.Name] = t.Entity
	}
	return true
}

// readSettings reads rows and seed from the target's settings.
func (g *gen) readSettings() {
	g.seed = 1
	settings := g.impl.Settings
	if v, ok := settings["seed"]; ok {
		n, err := strconv.ParseUint(text(v), 10, 64)
		if err != nil {
			g.problem("/", "settings.seed of the seed target is %v, and the seed is a whole number from 0 to 18446744073709551615", v)
		}
		g.seed = n
	}
	g.rnd = newRandom(g.seed)
	for _, e := range sortedKeys(g.tables) {
		g.counts[e] = defaultRows
	}
	rows, ok := obj(settings["rows"])
	if !ok && settings["rows"] != nil {
		g.problem("/", "settings.rows of the seed target is not a map; write the entities and their counts under it, such as rows: { Book: 20 }")
		return
	}
	for _, e := range sortedKeys(rows) {
		if _, ok := g.tables[e]; !ok {
			if _, isEntity := obj0(g.spec["entities"])[e]; isEntity {
				g.problem("/", "settings.rows of the seed target names %s, which another stakeholder owns, so no row of it is seeded; take it out", e)
			} else {
				g.problem("/", "settings.rows of the seed target names %s, which is not an entity of the specification; name an entity", e)
			}
			continue
		}
		n, err := strconv.ParseInt(text(rows[e]), 10, 64)
		if err != nil || n < 0 {
			g.problem("/", "settings.rows.%s of the seed target is %v, and a count is a whole number of 0 or more", e, rows[e])
			continue
		}
		g.counts[e] = n
	}
}

// parent is one table another's rows point to.
type parent struct {
	entity   string
	fk       gensql.ForeignKey
	required bool
	oneToOne bool
	self     bool
}

// parents are the entities whose rows an entity's foreign keys point to,
// in the order of its foreign keys.
func (g *gen) parents(e string) []parent {
	t := g.tables[e]
	nullable := map[string]bool{}
	for _, c := range t.Columns {
		nullable[c.Name] = c.Nullable
	}
	relations := obj0(obj0(obj0(g.spec["entities"])[e])["relations"])
	var out []parent
	for _, fk := range t.Keys.ForeignKeys {
		p := parent{entity: g.byTable[fk.Table], fk: fk, required: true, oneToOne: text(obj0(relations[fk.Relation])["kind"]) == "one-to-one", self: fk.Table == t.Name}
		for _, c := range fk.Columns {
			if nullable[c] {
				p.required = false
			}
		}
		out = append(out, p)
	}
	return out
}

// orderEntities puts the entities in the order their rows can be inserted:
// an entity after the ones its required foreign keys point to, and after
// the ones its optional foreign keys point to where that is possible. A
// cycle of required foreign keys leaves its entities out, with an error.
func (g *gen) orderEntities() {
	remaining := map[string]bool{}
	for _, e := range sortedKeys(g.tables) {
		remaining[e] = true
	}
	placed := map[string]bool{}
	settled := func(p parent) bool { return p.self || p.entity == "" || !remaining[p.entity] || placed[p.entity] }
	for len(remaining) > 0 {
		var next string
		for _, e := range sortedKeys(remaining) {
			ready, all := true, true
			for _, p := range g.parents(e) {
				if settled(p) {
					continue
				}
				all = false
				if p.required {
					ready = false
				}
			}
			if ready && all {
				next = e
				break
			}
			if ready && next == "" {
				next = e
			}
		}
		if next == "" {
			cycle := sortedKeys(remaining)
			for _, e := range cycle {
				g.problem("/entities/"+e, "the required relations of %s form a cycle, so no table among them can be filled first; make one of the relations optional, or set rows.<entity>: 0 for one of them in the seed target's settings", strings.Join(cycle, ", "))
				g.out[e] = "its required relations form a cycle with " + strings.Join(cycle, ", ")
			}
			return
		}
		g.order = append(g.order, next)
		placed[next] = true
		delete(remaining, next)
	}
}

// fill makes the rows of one entity, or says why it gets none.
func (g *gen) fill(e string) {
	t := g.tables[e]
	at := "/entities/" + mark.Escape(e)
	n := g.counts[e]
	if n == 0 {
		g.out[e] = "rows." + e + " is 0"
		return
	}
	for _, c := range t.Columns {
		if text(c.Definition["atRest"]) == "encrypted" && !c.Nullable && c.Role == "" {
			g.problem(at+"/properties/"+mark.Escape(c.Field), "%s.%s is encrypted at rest, and the key that encrypts it and the salt of its hash are the service's, so no seed can write a value the service reads back; make such a record through the service, or set rows.%s: 0 in the seed target's settings and seed %s by hand", e, c.Field, e, e)
			g.out[e] = c.Field + " is encrypted at rest"
			return
		}
	}
	parents := g.parents(e)
	for _, p := range parents {
		switch {
		case p.self && p.required:
			g.problem(at+"/relations/"+p.fk.Relation, "%s.%s is a required relation to %s itself, so no first row can be written; make it optional, or set rows.%s: 0", e, p.fk.Relation, e, e)
			g.out[e] = "its relation " + p.fk.Relation + " to itself is required"
			return
		case p.self:
			continue
		case p.entity == "" && p.required:
			g.problem(at+"/relations/"+p.fk.Relation, "%s.%s points to %s, which another stakeholder owns, so no row of it is seeded for a row of %s to point to; make the relation optional, or set rows.%s: 0 and seed %s by hand", e, p.fk.Relation, p.fk.Table, e, e, e)
			g.out[e] = "its relation " + p.fk.Relation + " points to a table another stakeholder owns"
			return
		case p.entity == "":
			continue
		case p.required && len(g.made[p.entity]) == 0:
			why := g.out[p.entity]
			if why == "" {
				why = "it gets no rows"
			}
			g.problem(at, "%s needs a row of %s for each of its rows through %s, and %s has none (%s); set rows.%s: 0 as well, or give %s rows", e, p.entity, p.fk.Relation, p.entity, why, e, p.entity)
			g.out[e] = "it needs a row of " + p.entity + ", and " + p.entity + " has none"
			return
		case p.oneToOne && int64(len(g.made[p.entity])) < n:
			g.problem(at+"/relations/"+p.fk.Relation, "%s has %d rows, and its one-to-one relation %s needs a row of %s for each, and %s has %d; give %s at least %d rows, or %s at most %d", e, n, p.fk.Relation, p.entity, p.entity, len(g.made[p.entity]), p.entity, n, e, len(g.made[p.entity]))
			g.out[e] = "its one-to-one relation " + p.fk.Relation + " has too few " + p.entity + " rows"
			return
		}
	}
	g.parseChecks(e)
	seen := g.seenSets(t)
	for i := int64(0); i < n; i++ {
		var kept *row
		var why string
		for attempt := 0; attempt < attempts && kept == nil; attempt++ {
			r, ok := g.row(e, t, parents, i)
			if !ok {
				return // the field is reported
			}
			if !g.distinct(t, r, seen, false) {
				why = "two rows would hold the same key or unique values"
				continue
			}
			if why = g.satisfies(e, r); why != "" {
				continue
			}
			kept = r
		}
		if kept == nil {
			g.problem(at, "no row of %s satisfied its constraints in %d tries, the last one failing because %s; give the fields the constraint reads bounds or examples a row can satisfy, or set rows.%s: 0", e, attempts, why, e)
			g.out[e] = "no row satisfied its constraints"
			g.made[e] = nil
			return
		}
		g.distinct(t, kept, seen, true)
		g.made[e] = append(g.made[e], *kept)
	}
}

// seenSets is one set per key and unique constraint of a table, of the
// value tuples its rows hold.
func (g *gen) seenSets(t gensql.Table) [][]any {
	var sets [][]any
	if len(t.Keys.PrimaryKey) > 0 {
		sets = append(sets, []any{t.Keys.PrimaryKey, map[string]bool{}})
	}
	for _, u := range t.Keys.Uniques {
		sets = append(sets, []any{u.Columns, map[string]bool{}})
	}
	return sets
}

// distinct reports whether a row's key and unique tuples are new, and
// records them when record is set.
func (g *gen) distinct(t gensql.Table, r *row, sets [][]any, record bool) bool {
	for _, s := range sets {
		cols, seen := s[0].([]string), s[1].(map[string]bool)
		var parts []string
		allNull := true
		for _, c := range cols {
			v := r.values[c]
			if v.kind != kNull {
				allNull = false
			}
			parts = append(parts, v.key())
		}
		if allNull {
			continue // two NULLs are distinct in a unique constraint
		}
		k := strings.Join(parts, "\x00")
		if seen[k] && !record {
			return false
		}
		if record {
			seen[k] = true
		}
	}
	return true
}

// parseChecks parses an entity's check constraints once.
func (g *gen) parseChecks(e string) {
	cs := obj0(obj0(obj0(g.spec["entities"])[e])["constraints"])
	for _, cn := range sortedKeys(cs) {
		c := obj0(cs[cn])
		if text(c["kind"]) != "check" {
			continue
		}
		tree, errs := expr.Parse(text(c["expression"]))
		if len(errs) > 0 {
			continue // the validator reports it
		}
		g.checks[e] = append(g.checks[e], check{name: cn, message: strings.TrimSuffix(text(c["message"]), "."), tree: tree})
	}
}

// satisfies evaluates a row against the entity's check constraints, as the
// validator checks a fixture's record, and names the first one that fails.
func (g *gen) satisfies(e string, r *row) string {
	for _, c := range g.checks[e] {
		if !allGiven(c.tree, r.fields) {
			continue
		}
		got, err := expr.Eval(c.tree, r.fields, g.zone)
		if err != nil {
			return fmt.Sprintf("%s could not be evaluated on it (%v)", c.name, err)
		}
		if got.Kind == expr.Bool && !got.Bool {
			return fmt.Sprintf("it broke %s (%s)", c.name, c.message)
		}
	}
	return ""
}

func allGiven(n *expr.Node, vals map[string]expr.Value) bool {
	if n.Op == expr.OpName {
		if _, ok := vals[n.Text]; !ok {
			return false
		}
	}
	for _, a := range n.Args {
		if !allGiven(a, vals) {
			return false
		}
	}
	return true
}

// write is the text of the file.
func (g *gen) write(r *Request) string {
	var b strings.Builder
	info := obj0(g.spec["info"])
	fmt.Fprintf(&b, "-- Generated by %s from %s, version %s; meta-model %s; %s through type-rendering %s; seed %d. Do not edit this file: a change goes into the specification or the seed target's settings.\n",
		name, gensql.RelRoot(r.Root, r.Output), text(info["version"]), r.Specarch, g.dialect, g.model.Version, g.seed)
	var counts []string
	for _, e := range g.order {
		if n := len(g.made[e]); n > 0 {
			counts = append(counts, fmt.Sprintf("%d %s", n, e))
		}
	}
	summary := "no rows"
	if len(counts) > 0 {
		summary = strings.Join(counts, ", ")
	}
	fmt.Fprintf(&b, "-- Development data: %s, drawn from the entities with a fixed seed, so this file is the same on every run; no value in it is about a real person. Load it once into a database the migrations have just built; it truncates nothing.\n", summary)
	if r.Draft != "" {
		b.WriteString("-- " + r.Draft + "\n")
	}
	b.WriteString(g.marks(mark.File))
	for _, e := range g.order {
		t := g.tables[e]
		b.WriteString("\n")
		b.WriteString(g.marks("/entities/" + mark.Escape(e)))
		rows := g.made[e]
		if len(rows) == 0 {
			why := g.out[e]
			if why == "" {
				why = "it gets no rows"
			}
			fmt.Fprintf(&b, "-- %s: no rows; %s.\n", e, why)
			continue
		}
		fmt.Fprintf(&b, "-- %s: %d rows into %s.\n", e, len(rows), t.Name)
		var cols []string
		for _, c := range t.Columns {
			cols = append(cols, c.Name)
		}
		for _, row := range rows {
			var vals []string
			for _, c := range t.Columns {
				vals = append(vals, row.values[c.Name].literal(g.dialect, g.model.Literals))
			}
			fmt.Fprintf(&b, "INSERT INTO %s (%s) VALUES (%s);\n", t.Name, strings.Join(cols, ", "), strings.Join(vals, ", "))
		}
	}
	for _, e := range sortedKeys(g.tables) {
		if g.out[e] != "" && !contains(g.order, e) {
			fmt.Fprintf(&b, "\n%s-- %s: no rows; %s.\n", g.marks("/entities/"+mark.Escape(e)), e, g.out[e])
		}
	}
	return b.String()
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// place gives each problem its entry: a table, or the file.
func (g *gen) place(r *Request) {
	var elements []string
	for _, e := range sortedKeys(g.tables) {
		elements = append(elements, "/entities/"+mark.Escape(e))
	}
	g.placed = mark.Place(r.Problems, elements, []string{"/entities"})
}

// marks are the comment lines of the problems placed at an element.
func (g *gen) marks(ptr string) string {
	var b strings.Builder
	for _, p := range g.placed[ptr] {
		b.WriteString("-- " + mark.Line(p) + "\n")
	}
	return b.String()
}

func (g *gen) problem(path, format string, args ...any) {
	if g.reported[path+"\x00"+format] {
		return
	}
	g.reported[path+"\x00"+format] = true
	g.diags = append(g.diags, gensql.Diagnostic{File: g.root, Line: 1, Severity: "error", Path: path, Rule: "generator", Message: fmt.Sprintf(format, args...)})
}

func (g *gen) response(files []gensql.File) gensql.Response {
	if files == nil {
		files = []gensql.File{}
	}
	diags := g.diags
	if diags == nil {
		diags = []gensql.Diagnostic{}
	}
	return gensql.Response{Files: files, Diagnostics: diags}
}

func obj(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func obj0(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func text(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case nil:
		return ""
	}
	return fmt.Sprint(v)
}
