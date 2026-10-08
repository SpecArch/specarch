package gensql

import (
	"fmt"
	"reflect"
	"strings"
)

// Views: each view of the specification becomes a SQL view, written after
// the tables and their foreign keys. A view lists its entity's columns by
// name, joins each relation a path follows with a LEFT JOIN, so a record
// with no related record is still listed, and counts a relation to many in
// a subquery that leaves out softly deleted records.

// viewName is a view's name in SQL: the mapping's when it names one
// (view loan_rows), otherwise the view's name in snake case.
func viewName(view string, mappings map[string]any) string {
	target := text(mappings["#/views/"+view])
	if m, ok := obj(mappings["#/views/"+view]); ok {
		target = text(m["target"])
	}
	if rest, ok := strings.CutPrefix(target, "view "); ok {
		if i := strings.IndexAny(rest, " ,"); i >= 0 {
			rest = rest[:i]
		}
		if rest != "" {
			return rest
		}
	}
	return snake(view)
}

// createViews are the statements that create every view, in name order.
func (g *gen) createViews() []string {
	views := obj0(g.spec["views"])
	var out []string
	for _, name := range sortedKeys(views) {
		if stmt := g.createView(name); stmt != "" {
			out = append(out, stmt)
		}
	}
	return out
}

// singleKey is the one column of an entity's primary key, or "" when the
// key is not one field.
func (g *gen) singleKey(entity string) string {
	pk := texts(list(obj0(obj0(g.spec["entities"])[entity])["primaryKey"]))
	if len(pk) != 1 {
		return ""
	}
	return snake(pk[0])
}

// createView is the statement that creates one view, or "" after a
// problem is reported.
func (g *gen) createView(name string) string {
	at := "/views/" + name
	v := obj0(obj0(g.spec["views"])[name])
	entities := obj0(g.spec["entities"])
	from := text(v["from"])
	if entities[from] == nil {
		g.problem(at+"/from", "%s is not an entity of the specification", from)
		return ""
	}
	var cols []string
	taken := map[string]bool{}
	for _, l := range g.table(from, obj0(entities[from])).columns {
		c := strings.Fields(l)[0]
		taken[c] = true
		cols = append(cols, "t0."+c)
	}
	aliases := map[string]string{"": "t0"} // a path's relations, as far as a hop, to the join's alias
	var joins []string
	props := obj0(v["properties"])
	for _, p := range sortedKeys(props) {
		prop := obj0(props[p])
		pat := at + "/properties/" + p
		if taken[snake(p)] {
			g.problem(pat, "%s is the column %s of %s's table already (an audit, deleted or hash column), and a view's columns must differ; give the added field another name", p, snake(p), from)
			return ""
		}
		if path := text(prop["path"]); path != "" {
			hops := strings.Split(path, ".")
			cur, prefix := from, ""
			for _, hop := range hops[:len(hops)-1] {
				rel := obj0(obj0(obj0(entities[cur])["relations"])[hop])
				target := text(rel["target"])
				if k := text(rel["kind"]); k != "many-to-one" && k != "one-to-one" || entities[target] == nil {
					g.problem(pat+"/path", "%s does not follow relations to one record", path)
					return ""
				}
				key := g.singleKey(target)
				if key == "" {
					g.problem(pat+"/path", "%s joins %s, whose primary key is not one field; a view joins on a key of one column", path, target)
					return ""
				}
				next := strings.TrimPrefix(prefix+"."+hop, ".")
				if aliases[next] == "" {
					aliases[next] = fmt.Sprintf("t%d", len(aliases))
					joins = append(joins, fmt.Sprintf("LEFT JOIN %s %s ON %s.%s = %s.%s", g.tableName(target), aliases[next], aliases[next], key, aliases[prefix], snake(text(rel["via"]))))
				}
				cur, prefix = target, next
			}
			last := hops[len(hops)-1]
			if obj0(obj0(obj0(entities[cur])["properties"])[last])["writeOnly"] == true {
				g.problem(pat+"/path", "%s ends in %s.%s, which is written and never read, and a view is only read; leave it out of the view", path, cur, last)
				return ""
			}
			cols = append(cols, fmt.Sprintf("%s.%s AS %s", aliases[prefix], snake(last), snake(p)))
			continue
		}
		count, ok := g.count(from, text(prop["count"]), pat)
		if !ok {
			return ""
		}
		cols = append(cols, count+" AS "+snake(p))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE VIEW %s AS\nSELECT\n", viewName(name, obj0(g.impl.Content["mappings"])))
	for i, c := range cols {
		b.WriteString("    " + c)
		if i < len(cols)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	// Oracle refuses AS before a table's alias; every engine takes it left out.
	fmt.Fprintf(&b, "FROM %s t0", g.tableName(from))
	for _, j := range joins {
		b.WriteString("\n    " + j)
	}
	if g.dialect == "sqlserver" {
		// SQL Server takes CREATE VIEW only as the first statement of a
		// batch, and a migration runs as one batch.
		return "EXEC('" + strings.ReplaceAll(b.String(), "'", "''") + "')"
	}
	return b.String()
}

// count is the subquery that counts a relation to many of an entity,
// leaving out softly deleted records.
func (g *gen) count(from, relation, at string) (string, bool) {
	entities := obj0(g.spec["entities"])
	rel := obj0(obj0(obj0(entities[from])["relations"])[relation])
	target, via := text(rel["target"]), text(rel["via"])
	key := g.singleKey(from)
	if key == "" {
		g.problem(at+"/count", "%s's primary key is not one field; a count matches its related records on a key of one column", from)
		return "", false
	}
	fn := "COUNT(*)"
	if g.dialect == "sqlserver" {
		fn = "COUNT_BIG(*)" // COUNT is a 32-bit integer on SQL Server
	}
	deleted := names(g.rendering("soft-delete", "column", "any"))["deleted"]
	notDeleted := func(entity, alias string) string {
		if text(obj0(entities[entity])["deletion"]) == "soft" {
			return fmt.Sprintf(" AND %s.%s = %s", alias, deleted, g.defaults["false"])
		}
		return ""
	}
	switch text(rel["kind"]) {
	case "one-to-many":
		if entities[target] == nil {
			break
		}
		return fmt.Sprintf("(SELECT %s FROM %s c WHERE c.%s = t0.%s%s)", fn, g.tableName(target), snake(via), key, notDeleted(target, "c")), true
	case "many-to-many":
		if entities[via] == nil || entities[target] == nil {
			break
		}
		if target == from {
			g.problem(at+"/count", "%s relates %s to itself through %s, so which side is counted cannot be told; count a one-to-many relation of %s to %s instead", relation, from, via, from, via)
			return "", false
		}
		var toFrom, toTarget []string
		jrels := obj0(obj0(entities[via])["relations"])
		for _, rn := range sortedKeys(jrels) {
			jr := obj0(jrels[rn])
			if k := text(jr["kind"]); k != "many-to-one" && k != "one-to-one" {
				continue
			}
			switch text(jr["target"]) {
			case from:
				toFrom = append(toFrom, text(jr["via"]))
			case target:
				toTarget = append(toTarget, text(jr["via"]))
			}
		}
		if len(toFrom) != 1 || len(toTarget) != 1 {
			g.problem(at+"/count", "%s, the join entity of %s, needs exactly one many-to-one relation to %s and one to %s, so the count knows which columns join", via, relation, from, target)
			return "", false
		}
		join := ""
		if text(obj0(entities[target])["deletion"]) == "soft" {
			tk := g.singleKey(target)
			if tk == "" {
				g.problem(at+"/count", "%s's primary key is not one field; a count joins it on a key of one column", target)
				return "", false
			}
			join = fmt.Sprintf(" INNER JOIN %s r ON r.%s = c.%s", g.tableName(target), tk, snake(toTarget[0]))
		}
		return fmt.Sprintf("(SELECT %s FROM %s c%s WHERE c.%s = t0.%s%s%s)", fn, g.tableName(via), join, snake(toFrom[0]), key, notDeleted(via, "c"), notDeleted(target, "r")), true
	}
	g.problem(at+"/count", "%s is not a relation of %s that leads to many records", relation, from)
	return "", false
}

// viewKeys are the keys of a view the schema is made from.
var viewKeys = map[string]any{"from": true, "properties": map[string]any{"path": true, "count": true}}

// readsOf are the entities a view reads, in a snapshot's or the
// specification's entities.
func readsOf(view, entities map[string]any) []string {
	from := text(view["from"])
	out := []string{from}
	for _, p := range obj0(view["properties"]) {
		prop := obj0(p)
		if path := text(prop["path"]); path != "" {
			cur := from
			hops := strings.Split(path, ".")
			for _, hop := range hops[:len(hops)-1] {
				cur = text(obj0(obj0(obj0(entities[cur])["relations"])[hop])["target"])
				out = append(out, cur)
			}
			continue
		}
		rel := obj0(obj0(obj0(entities[from])["relations"])[text(prop["count"])])
		out = append(out, text(rel["target"]))
		if text(rel["kind"]) == "many-to-many" {
			out = append(out, text(rel["via"]))
		}
	}
	return out
}

// staleViews are the views to drop (by their name in the snapshot) and to
// create again: a view that changed, came or went, or reads an entity
// that changed. A view holds no rows, so dropping and creating it loses
// nothing. Its column list is fixed when it is created, so a changed
// entity would not show through it until it is made again, and
// PostgreSQL refuses to change the type of a column a view reads.
func (g *gen) staleViews(prev, cur map[string]any) (drop []string, create []string) {
	pe, ce := obj0(prev["entities"]), obj0(cur["entities"])
	pm, cm := obj0(prev["mappings"]), obj0(cur["mappings"])
	pv, cv := obj0(prev["views"]), obj0(cur["views"])
	pn, cn := obj0(prev["enums"]), obj0(cur["enums"])
	changed := func(entity string) bool {
		if !reflect.DeepEqual(pe[entity], ce[entity]) || text(pm["#/entities/"+entity]) != text(cm["#/entities/"+entity]) {
			return true
		}
		// An enum's values set its column's width and check, so a view
		// over a field of the enum is made again when the enum changes.
		for _, f := range obj0(obj0(ce[entity])["properties"]) {
			for _, ref := range []string{text(obj0(f)["$ref"]), text(obj0(obj0(f)["items"])["$ref"])} {
				if e, ok := strings.CutPrefix(ref, "#/enums/"); ok && !reflect.DeepEqual(pn[e], cn[e]) {
					return true
				}
			}
		}
		return false
	}
	stale := func(name string, view, entities map[string]any) bool {
		if !reflect.DeepEqual(pv[name], cv[name]) || text(pm["#/views/"+name]) != text(cm["#/views/"+name]) {
			return true
		}
		for _, e := range readsOf(view, entities) {
			if changed(e) {
				return true
			}
		}
		return false
	}
	for _, name := range sortedKeys(pv) {
		if stale(name, obj0(pv[name]), pe) {
			drop = append(drop, "DROP VIEW "+viewName(name, pm))
		}
	}
	for _, name := range sortedKeys(cv) {
		if stale(name, obj0(cv[name]), ce) {
			create = append(create, name)
		}
	}
	return drop, create
}
