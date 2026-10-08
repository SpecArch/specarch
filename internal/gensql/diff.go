package gensql

import (
	"fmt"
	"reflect"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The differ: what changed between the snapshot of the last migration and
// the schema now, written as the next migrations. What only adds (a table,
// a column, a constraint, a wider type, an enum value, a nullable column)
// goes into an expand file; what can lose data (a dropped table or column,
// a narrower type, a removed enum value, NOT NULL on a nullable column)
// into a contract file of its own, written only when the target's settings
// say destructive: true. What the differ cannot tell apart from a rename or
// a rewrite fails generation, so a person writes it.

// steps are the statements of the next migrations.
type steps struct {
	expand, contract []string
	destructive      []string // what the contract steps do, for the message when they are not allowed
}

func (s *steps) add(stmt string) { s.expand = append(s.expand, stmt) }
func (s *steps) remove(stmt, what string) {
	s.contract = append(s.contract, stmt)
	s.destructive = append(s.destructive, what)
}
func (s *steps) empty() bool                      { return len(s.expand)+len(s.contract) == 0 }
func (g *gen) cannot(at, format string, a ...any) { g.problem(at, format, a...) }

// diff compares the snapshot of the last migration with the schema now.
func (g *gen) diff(prevText string) *steps {
	var prev map[string]any
	if err := yaml.Unmarshal([]byte(prevText), &prev); err != nil {
		g.cannot("/", "%s cannot be read (%v); restore it from the folder's history", SnapshotName, err)
		return nil
	}
	var cur map[string]any
	if err := yaml.Unmarshal([]byte(g.snapshot()), &cur); err != nil {
		g.cannot("/", "the snapshot of the schema cannot be written (%v); this is a bug in specarch-gen-sql", err)
		return nil
	}
	if d := text(prev["dialect"]); d != g.dialect {
		g.cannot("/", "the folder holds %s migrations and the target is %s now; a new dialect starts in a new folder", d, g.dialect)
		return nil
	}
	s := &steps{}
	pe, ce := obj0(prev["entities"]), obj0(cur["entities"])
	pm, cm := obj0(prev["mappings"]), obj0(cur["mappings"])
	var fks []string
	for _, name := range sortedKeys(ce) {
		e := obj0(ce[name])
		if pe[name] == nil {
			t := g.table(name, obj0(obj0(g.spec["entities"])[name]))
			s.add(strings.TrimSuffix(t.create(g.dialect), ";\n"))
			fks = append(fks, t.foreignKeys...)
			continue
		}
		if text(pm["#/entities/"+name]) != text(cm["#/entities/"+name]) {
			g.cannot("/entities/"+name, "the table of %s changed in its mapping, which reads as a drop and a create; rename the table by hand in a migration of its own", name)
			continue
		}
		fks = append(fks, g.diffEntity(s, name, obj0(pe[name]), e, obj0(prev["enums"]), obj0(cur["enums"]))...)
	}
	for _, name := range sortedKeys(pe) {
		if ce[name] == nil {
			s.remove("DROP TABLE "+g.tableNameOf(name, pm), "drop the table of "+name)
		}
	}
	for _, fk := range fks {
		s.add(fk)
	}
	return s
}

// tableNameOf is an entity's table as the snapshot's mappings name it.
func (g *gen) tableNameOf(entity string, mappings map[string]any) string {
	if rest, ok := strings.CutPrefix(text(mappings["#/entities/"+entity]), "table "); ok {
		if i := strings.IndexAny(rest, " ,"); i >= 0 {
			rest = rest[:i]
		}
		if rest != "" {
			return rest
		}
	}
	return snake(entity)
}

// diffEntity compares one entity, and answers the foreign keys to add.
func (g *gen) diffEntity(s *steps, name string, prev, cur, prevEnums, curEnums map[string]any) []string {
	tn := g.tableName(name)
	at := "/entities/" + name
	if !reflect.DeepEqual(prev["primaryKey"], cur["primaryKey"]) {
		g.cannot(at+"/primaryKey", "the primary key of %s changed; moving a key under live rows is a migration a person writes", name)
		return nil
	}
	pp, cp := obj0(prev["properties"]), obj0(cur["properties"])
	preq, creq := requiredSet(prev), requiredSet(cur)
	specProps := obj0(obj0(obj0(g.spec["entities"])[name])["properties"])

	for _, f := range sortedKeys(cp) {
		if pp[f] != nil {
			g.diffField(s, tn, at, f, obj0(pp[f]), obj0(specProps[f]), preq[f], creq[f], prevEnums, curEnums)
			continue
		}
		field := obj0(specProps[f])
		fc, ok := g.fieldColumn(tn, at, f, field, creq[f], false)
		if !ok {
			continue
		}
		if !fc.nullable && field["default"] == nil {
			g.cannot(at+"/properties/"+f, "%s is a new required column of a table that may hold rows; add it nullable and backfill it, or give it a default", f)
			continue
		}
		for _, l := range fc.lines {
			s.add(g.addColumn(tn, l))
		}
		if fc.check != "" {
			s.add("ALTER TABLE " + tn + " ADD " + fc.check)
		}
	}
	for _, f := range sortedKeys(pp) {
		if cp[f] == nil {
			s.remove(g.dropColumn(tn, snake(f)), "drop the column "+snake(f)+" of "+tn)
			if text(obj0(pp[f])["lookup"]) == "hash" {
				s.remove(g.dropColumn(tn, snake(f)+names(g.rendering("encrypted-column", "column", "any"))["hashSuffix"]), "drop the hash column of "+snake(f))
			}
		}
	}

	switch {
	case prev["audited"] != true && cur["audited"] == true:
		g.cannot(at+"/audited", "%s became audited, and its audit columns are NOT NULL on a table that may hold rows; add them by hand with a backfill", name)
	case prev["audited"] == true && cur["audited"] != true:
		audit := names(g.rendering("audit-fields", "columns", "any"))
		for _, k := range []string{"createdAt", "createdBy", "createdByName", "lastModifiedAt", "lastModifiedBy", "lastModifiedByName"} {
			if c := audit[k]; c != "" {
				s.remove(g.dropColumn(tn, c), "drop the audit column "+c+" of "+tn)
			}
		}
	}
	deleted := names(g.rendering("soft-delete", "column", "any"))["deleted"]
	switch {
	case text(prev["deletion"]) != "soft" && text(cur["deletion"]) == "soft":
		typ, check, _ := g.render(map[string]any{"type": "boolean"}, deleted)
		s.add(g.addColumn(tn, columnLine(deleted, typ, g.defaults["false"], false)))
		if check != "" {
			s.add(fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT ck_%s_%s CHECK (%s)", tn, tn, deleted, check))
		}
	case text(prev["deletion"]) == "soft" && text(cur["deletion"]) != "soft":
		s.remove(g.dropColumn(tn, deleted), "drop the soft delete column of "+tn)
	}

	pc, cc := obj0(prev["constraints"]), obj0(cur["constraints"])
	specEntity := obj0(obj0(g.spec["entities"])[name])
	for _, c := range sortedKeys(pc) {
		if cc[c] == nil || !reflect.DeepEqual(pc[c], cc[c]) {
			s.add(g.dropConstraint(tn, c, text(obj0(pc[c])["kind"]), g.filteredUnique(obj0(pc[c]), pp, preq)))
		}
	}
	for _, c := range sortedKeys(cc) {
		if pc[c] == nil || !reflect.DeepEqual(pc[c], cc[c]) {
			one := map[string]any{"properties": specEntity["properties"], "required": specEntity["required"], "primaryKey": specEntity["primaryKey"],
				"constraints": map[string]any{c: obj0(specEntity["constraints"])[c]}}
			t := g.constraintsOnly(name, one)
			for _, con := range t.constraints {
				s.add("ALTER TABLE " + tn + " ADD " + con)
			}
			for _, ix := range t.indexes {
				s.add(ix)
			}
		}
	}

	var fks []string
	prel, crel := relationsByVia(prev), relationsByVia(cur)
	for _, via := range sortedKeys(prel) {
		if crel[via] == nil || !reflect.DeepEqual(prel[via], crel[via]) {
			s.add(g.dropForeignKey(tn, "fk_"+tn+"_"+snake(via)))
		}
	}
	for _, via := range sortedKeys(crel) {
		if prel[via] == nil || !reflect.DeepEqual(prel[via], crel[via]) {
			fks = append(fks, g.foreignKey(tn, obj0(crel[via])))
		}
	}
	return fks
}

// diffField compares one field that is in both.
func (g *gen) diffField(s *steps, tn, at, f string, prev, specField map[string]any, prevReq, curReq bool, prevEnums, curEnums map[string]any) {
	col := snake(f)
	cur := keep(specField, fieldKeys)
	same := func(keys ...string) bool {
		for _, k := range keys {
			if text(prev[k]) != text(cur[k]) {
				return false
			}
		}
		return true
	}
	if !same("$ref", "format", "atRest", "lookup") || fmt.Sprint(prev["type"]) != fmt.Sprint(plainValues(cur["type"])) || !reflect.DeepEqual(prev["items"], plainValues(cur["items"])) {
		g.cannot(at+"/properties/"+f, "the type of %s changed; a column that changes its type is a migration a person writes, in an expand and a contract step", f)
		return
	}
	if !same("default") {
		g.cannot(at+"/properties/"+f, "the default of %s changed, which this version does not write; change it by hand in a migration of its own", f)
		return
	}
	if text(cur["atRest"]) == "encrypted" {
		return // the ciphertext type does not follow the field's width
	}
	prevNullable, curNullable := !prevReq || nullType(prev), !curReq || nullType(specField)
	prevShape := map[string]any{}
	for k, v := range prev {
		prevShape[k] = v
	}
	if r := text(prev["$ref"]); strings.HasPrefix(r, "#/enums/") {
		prevShape["enum"] = obj0(prevEnums[strings.TrimPrefix(r, "#/enums/")])["enum"]
		delete(prevShape, "$ref")
		prevShape["type"] = "string"
	}
	oldTyp, oldCheck, ok1 := g.render(prevShape, col)
	newTyp, newCheck, ok2 := g.render(specField, col)
	if !ok1 || !ok2 {
		return // a missing row is reported by the table
	}
	oldRow, newRow := g.rowOf(prevShape), g.rowOf(specField)
	if oldTyp != newTyp {
		switch {
		case oldRow != newRow:
			g.cannot(at+"/properties/"+f, "%s moves from %s to %s on %s, a change of column type the engine does not make in place; write it by hand", f, oldTyp, newTyp, g.dialect)
			return
		case wider(prev, specField):
			s.add(g.alterType(tn, col, newTyp, curNullable))
		default:
			s.remove(g.alterType(tn, col, newTyp, curNullable), "narrow "+col+" of "+tn+" to "+newTyp)
		}
	}
	if oldCheck != newCheck {
		ck := "ck_" + tn + "_" + col
		if oldCheck != "" {
			s.add(g.dropConstraint(tn, ck, "check", false))
		}
		if newCheck != "" {
			stmt := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s)", tn, ck, newCheck)
			if enumShrank(prevShape, specField, g) {
				s.remove(stmt, "allow fewer values in "+col+" of "+tn)
			} else {
				s.add(stmt)
			}
		}
	}
	switch {
	case prevNullable && !curNullable:
		s.remove(g.setNullable(tn, col, newTyp, false), "make "+col+" of "+tn+" NOT NULL")
	case !prevNullable && curNullable:
		s.add(g.setNullable(tn, col, newTyp, true))
	}
}

// rowOf is the render pattern of the row that renders a field.
func (g *gen) rowOf(field map[string]any) string {
	t, _, _ := g.renderPattern(field)
	return t
}

// wider reports whether a field holds at least what it held: a longer
// maxLength, or more precision at the same scale.
func wider(prev, cur map[string]any) bool {
	n := func(m map[string]any, k string) int {
		var v int
		fmt.Sscan(text(m[k]), &v)
		return v
	}
	if text(prev["scale"]) != text(cur["scale"]) && cur["scale"] != nil {
		return false
	}
	return n(cur, "maxLength") >= n(prev, "maxLength") && n(cur, "precision") >= n(prev, "precision")
}

// enumShrank reports whether an enum lost a value.
func enumShrank(prev, cur map[string]any, g *gen) bool {
	have := map[string]bool{}
	for _, v := range g.enumValues(cur) {
		have[v] = true
	}
	for _, v := range texts(list(prev["enum"])) {
		if !have[v] {
			return true
		}
	}
	return false
}

func requiredSet(e map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, r := range list(e["required"]) {
		out[text(r)] = true
	}
	return out
}

func relationsByVia(e map[string]any) map[string]any {
	out := map[string]any{}
	for _, r := range obj0(e["relations"]) {
		rel := obj0(r)
		if k := text(rel["kind"]); (k == "many-to-one" || k == "one-to-one") && text(rel["via"]) != "" {
			out[text(rel["via"])] = rel
		}
	}
	return out
}

// filteredUnique reports whether a unique constraint was written as a
// filtered index (SQL Server, over a nullable column).
func (g *gen) filteredUnique(c, props map[string]any, req map[string]bool) bool {
	if g.dialect != "sqlserver" || text(c["kind"]) != "unique" {
		return false
	}
	for _, f := range texts(list(c["fields"])) {
		if !req[f] || nullType(obj0(props[f])) {
			return true
		}
	}
	return false
}

// The statements, per dialect.

func (g *gen) addColumn(t, line string) string {
	switch g.dialect {
	case "oracle":
		return "ALTER TABLE " + t + " ADD (" + line + ")"
	case "sqlserver":
		return "ALTER TABLE " + t + " ADD " + line
	}
	return "ALTER TABLE " + t + " ADD COLUMN " + line
}

func (g *gen) dropColumn(t, c string) string { return "ALTER TABLE " + t + " DROP COLUMN " + c }

func (g *gen) alterType(t, c, typ string, nullable bool) string {
	null := " NOT NULL"
	if nullable {
		null = " NULL"
	}
	switch g.dialect {
	case "postgresql":
		return "ALTER TABLE " + t + " ALTER COLUMN " + c + " TYPE " + typ
	case "sqlserver":
		return "ALTER TABLE " + t + " ALTER COLUMN " + c + " " + typ + null
	case "oracle":
		return "ALTER TABLE " + t + " MODIFY (" + c + " " + typ + ")"
	}
	return "ALTER TABLE " + t + " MODIFY " + c + " " + typ + null
}

func (g *gen) setNullable(t, c, typ string, nullable bool) string {
	switch g.dialect {
	case "postgresql":
		if nullable {
			return "ALTER TABLE " + t + " ALTER COLUMN " + c + " DROP NOT NULL"
		}
		return "ALTER TABLE " + t + " ALTER COLUMN " + c + " SET NOT NULL"
	case "oracle":
		if nullable {
			return "ALTER TABLE " + t + " MODIFY (" + c + " NULL)"
		}
		return "ALTER TABLE " + t + " MODIFY (" + c + " NOT NULL)"
	}
	return g.alterType(t, c, typ, nullable)
}

func (g *gen) dropConstraint(t, name, kind string, filtered bool) string {
	switch {
	case filtered:
		return "DROP INDEX " + name + " ON " + t
	case g.dialect == "mariadb" && kind == "unique":
		return "ALTER TABLE " + t + " DROP INDEX " + name
	}
	return "ALTER TABLE " + t + " DROP CONSTRAINT " + name
}

func (g *gen) dropForeignKey(t, name string) string {
	if g.dialect == "mariadb" {
		return "ALTER TABLE " + t + " DROP FOREIGN KEY " + name
	}
	return "ALTER TABLE " + t + " DROP CONSTRAINT " + name
}
