package genseed

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/internal/gensql"
	"github.com/SpecArch/specarch/internal/mark"
)

// row draws one row of an entity: its foreign keys from the rows already
// made, an idiom's columns as the idiom means them, and every other column
// from its field. It answers false when a field cannot be given a value,
// which is reported at the field once.
func (g *gen) row(e string, t gensql.Table, parents []parent, i int64) (*row, bool) {
	r := &row{values: map[string]value{}, fields: map[string]expr.Value{}}
	at := "/entities/" + mark.Escape(e)
	entity := obj0(obj0(g.spec["entities"])[e])
	original := obj0(entity["properties"])
	required := map[string]bool{}
	for _, f := range list(entity["required"]) {
		required[text(f)] = true
	}
	// The foreign keys: a parent row, or NULL for an optional relation.
	fk := map[string]value{}
	for _, p := range parents {
		var pick *row
		switch {
		case p.self:
			if earlier := g.made[e]; len(earlier) > 0 && !g.rnd.chance(optionalOnceIn) {
				pick = &earlier[g.rnd.intn(int64(len(earlier)))]
			}
		case p.entity == "":
		default:
			rows := g.made[p.entity]
			switch {
			case len(rows) == 0:
			case p.oneToOne:
				// The row of the same rank; an optional relation holds
				// NULL beyond the other side's rows (a required one is
				// refused in fill).
				if i < int64(len(rows)) {
					pick = &rows[i]
				}
			case !p.required && g.rnd.chance(optionalOnceIn):
			default:
				pick = &rows[g.rnd.intn(int64(len(rows)))]
			}
		}
		for j, c := range p.fk.Columns {
			if pick == nil {
				fk[c] = null
			} else {
				fk[c] = pick.values[p.fk.References[j]]
			}
		}
	}
	// A value object kept in columns is present or absent as a whole, and
	// a required part of a present one is set: the plan says, for each
	// part's column, whether it gets a value.
	plan := map[string]bool{}
	schemas := obj0(g.spec["schemas"])
	for _, f := range sortedKeys(original) {
		fd := obj0(original[f])
		if ref, many := valueObjectOf(fd); ref != "" && !many && text(fd["storage"]) != "json" && schemas[ref] != nil {
			g.plan(snake(f), obj0(schemas[ref]), required[f] && !nullType(fd) || !g.rnd.chance(optionalOnceIn), plan)
		}
	}
	distinct := g.distinctColumns(t, fk)
	for _, c := range t.Columns {
		var v value
		switch {
		case hasKey(fk, c.Name):
			v = fk[c.Name]
		case c.Role == gensql.RoleCreated || c.Role == gensql.RoleModified:
			if c.Format == "date-time" {
				v = value{kind: kTimestamp, t: base.Add(time.Duration(i) * time.Hour)}
			} else {
				v = textValue("seed")
			}
		case c.Role == gensql.RoleDeleted:
			v = boolValue(false)
		case c.Role == gensql.RoleHash, text(c.Definition["atRest"]) == "encrypted":
			v = null
		case hasPlan(plan, c.Name) && !plan[c.Name]:
			v = null
		case !hasPlan(plan, c.Name) && c.Nullable && !c.Key && g.rnd.chance(optionalOnceIn):
			v = null
		default:
			var why string
			v, why = g.fieldValue(c.Definition, obj0(original[c.Field]), words(c.Name), i, distinct[c.Name])
			if why != "" {
				g.problem(at+"/properties/"+mark.Escape(c.Field), "%s.%s gets no value: %s", e, c.Field, why)
				g.out[e] = c.Field + " gets no value"
				return nil, false
			}
		}
		r.values[c.Name] = v
		// A list is given to the checks as its length and an object as
		// an object, as the evaluator reads a fixture's record, so a check
		// that reads one is evaluated rather than skipped.
		if c.Role == "" && c.Field != "" && !hasPlan(plan, c.Name) {
			r.fields[c.Field] = v.expression()
		}
	}
	return r, true
}

func hasKey(m map[string]value, k string) bool {
	_, ok := m[k]
	return ok
}

func hasPlan(m map[string]bool, k string) bool {
	_, ok := m[k]
	return ok
}

// plan decides, for the parts of a value object kept in columns under
// prefix, which columns get a value: a required part of a present value
// always, an optional one in three rows of four, and a nested value
// object as a whole by the same rule.
func (g *gen) plan(prefix string, schema map[string]any, present bool, plan map[string]bool) {
	required := map[string]bool{}
	for _, r := range list(schema["required"]) {
		required[text(r)] = true
	}
	schemas := obj0(g.spec["schemas"])
	props := obj0(schema["properties"])
	for _, p := range sortedKeys(props) {
		pd := obj0(props[p])
		col := prefix + "_" + snake(p)
		set := present && (required[p] && !nullType(pd) || !g.rnd.chance(optionalOnceIn))
		if inner, many := valueObjectOf(pd); inner != "" && !many && schemas[inner] != nil {
			g.plan(col, obj0(schemas[inner]), set, plan)
			continue
		}
		plan[col] = set
	}
}

// snake writes a camelCase or PascalCase name in snake case, as the
// migrations name a column.
func snake(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		if r >= 'A' && r <= 'Z' {
			if i > 0 && (rs[i-1] >= 'a' && rs[i-1] <= 'z' || rs[i-1] >= '0' && rs[i-1] <= '9' || i+1 < len(rs) && rs[i+1] >= 'a' && rs[i+1] <= 'z' && rs[i-1] >= 'A' && rs[i-1] <= 'Z') {
				b.WriteByte('_')
			}
			b.WriteRune(r + 'a' - 'A')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// distinctColumns are the columns whose values count up with the row
// rather than being drawn, so a key or a unique constraint never repeats:
// each key column that is not a foreign key, and the first such column of
// each unique constraint.
func (g *gen) distinctColumns(t gensql.Table, fk map[string]value) map[string]bool {
	out := map[string]bool{}
	for _, c := range t.Keys.PrimaryKey {
		if !hasKey(fk, c) {
			out[c] = true
		}
	}
	for _, u := range t.Keys.Uniques {
		for _, c := range u.Columns {
			if !hasKey(fk, c) {
				out[c] = true
				break
			}
		}
	}
	return out
}

// valueObjectOf names the schema a field holds, and whether a list of them.
func valueObjectOf(field map[string]any) (string, bool) {
	if ref, ok := strings.CutPrefix(text(field["$ref"]), "#/schemas/"); ok {
		return ref, false
	}
	if typeIs(field, "array") {
		if ref, ok := strings.CutPrefix(text(obj0(field["items"])["$ref"]), "#/schemas/"); ok {
			return ref, true
		}
	}
	return "", false
}

func typeIs(field map[string]any, t string) bool {
	if text(field["type"]) == t {
		return true
	}
	for _, item := range list(field["type"]) {
		if text(item) == t {
			return true
		}
	}
	return false
}

func nullType(field map[string]any) bool { return typeIs(field, "null") }

// typeOf is a field's JSON type with null dropped and its format; a $ref
// to an enum is enum, to a schema or an entity object.
func typeOf(def map[string]any) (typ, format string) {
	ref := text(def["$ref"])
	switch {
	case strings.HasPrefix(ref, "#/enums/"):
		return "enum", ""
	case ref != "":
		return "object", ""
	}
	typ = text(def["type"])
	for _, item := range list(def["type"]) {
		if s := text(item); s != "null" {
			typ = s
		}
	}
	if len(list(def["enum"])) > 0 {
		typ = "enum"
	}
	return typ, text(def["format"])
}

// enumValues are the values an enum field holds, by $ref or inline.
func (g *gen) enumValues(def map[string]any) []string {
	var out []string
	if ref, ok := strings.CutPrefix(text(def["$ref"]), "#/enums/"); ok {
		for _, v := range list(obj0(obj0(g.spec["enums"])[ref])["enum"]) {
			out = append(out, text(v))
		}
		return out
	}
	for _, v := range list(def["enum"]) {
		out = append(out, text(v))
	}
	return out
}

// words is a label for a column or a member, from its name: full_name is
// "Full name", dueOn "Due on".
func words(name string) string {
	var b strings.Builder
	for i, r := range name {
		switch {
		case r == '_':
			b.WriteByte(' ')
		case i > 0 && r >= 'A' && r <= 'Z':
			b.WriteByte(' ')
			b.WriteRune(r + 'a' - 'A')
		case i == 0 && r >= 'a' && r <= 'z':
			b.WriteRune(r - 'a' + 'A')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// fieldValue draws a value for a field. def is the field as the table
// read it, orig the design's field it comes from (the same, or the field
// a value object kept as JSON is written from), label the words the value
// names itself by, i the row and distinct whether the value counts up with
// the row. It answers why no value can be drawn, or "".
func (g *gen) fieldValue(def, orig map[string]any, label string, i int64, distinct bool) (value, string) {
	src := given(def, orig)
	if c, ok := src["const"]; ok {
		return g.fromExample(c, def, orig, label), ""
	}
	examples := list(src["examples"])
	if int64(len(examples)) > i {
		return g.fromExample(examples[i], def, orig, label), ""
	}
	if distinct && len(examples) > 0 {
		// The rows after the examples count from the first counted
		// value, skipping each value an example already holds, so a key
		// or unique never repeats one.
		taken := map[string]bool{}
		for _, x := range examples {
			taken[g.fromExample(x, def, orig, label).key()] = true
		}
		rank := i - int64(len(examples))
		for j := int64(0); j <= i; j++ {
			v, why := g.drawn(def, orig, label, j, true)
			if why != "" {
				return v, why
			}
			if taken[v.key()] {
				continue
			}
			if rank == 0 {
				return v, ""
			}
			rank--
		}
		return null, fmt.Sprintf("it is a key or unique and every value counted from its bounds repeats one of its %d examples; give it more examples or wider bounds", len(examples))
	}
	return g.drawn(def, orig, label, i, distinct)
}

// given is the definition a field's const and examples are read from: the
// field itself, or the design's field when the table reads a value object
// kept as JSON as one column, whose keywords it drops.
func given(def, orig map[string]any) map[string]any {
	if text(def["storage"]) == "json" && orig != nil {
		if _, ok := def["examples"]; !ok {
			if _, ok := def["const"]; !ok {
				return orig
			}
		}
	}
	return def
}

// drawn draws a value for a field from its type, format and bounds, with
// no example read; the arguments are fieldValue's.
func (g *gen) drawn(def, orig map[string]any, label string, i int64, distinct bool) (value, string) {
	typ, format := typeOf(def)
	switch typ {
	case "enum":
		values := g.enumValues(def)
		if len(values) == 0 {
			return null, "its enum has no values"
		}
		if distinct {
			if i >= int64(len(values)) {
				return null, fmt.Sprintf("it is a key or unique and its enum holds %d values, fewer than the rows; give the entity at most %d rows", len(values), len(values))
			}
			return value{kind: kText, text: values[i], enum: true}, ""
		}
		return value{kind: kText, text: values[g.rnd.intn(int64(len(values)))], enum: true}, ""
	case "boolean":
		if distinct {
			if i >= 2 {
				return null, "it is a key or unique and a boolean holds two values; give the entity at most 2 rows"
			}
			return boolValue(i == 1), ""
		}
		return boolValue(g.rnd.chance(2)), ""
	case "integer":
		return g.integer(def, format, i, distinct)
	case "number":
		return g.double(def, i, distinct)
	case "array":
		return g.array(def, orig, label, i)
	case "object":
		return g.object(def, orig, label, i)
	case "string":
		switch format {
		case "int64", "uint64":
			return g.integer(def, format, i, distinct)
		case "decimal":
			return g.decimal(def, i, distinct)
		case "date":
			if distinct {
				return dateValue(base.AddDate(0, 0, int(i))), ""
			}
			return dateValue(base.AddDate(0, 0, int(g.rnd.intn(365)))), ""
		case "date-time":
			if distinct {
				return value{kind: kTimestamp, t: base.Add(time.Duration(i) * time.Hour)}, ""
			}
			return value{kind: kTimestamp, t: base.Add(time.Duration(g.rnd.intn(365*24*3600)) * time.Second)}, ""
		case "time":
			if distinct {
				return value{kind: kTimeOfDay, num: (i * 60) % 86400}, ""
			}
			return value{kind: kTimeOfDay, num: g.rnd.intn(86400)}, ""
		case "duration":
			if distinct {
				return value{kind: kDuration, d: time.Duration(i+1) * time.Hour}, ""
			}
			return value{kind: kDuration, d: time.Duration(g.rnd.in(1, 72))*time.Hour + time.Duration(g.rnd.intn(4)*15)*time.Minute}, ""
		case "byte", "binary":
			return value{kind: kBytes, bytes: g.rnd.bytes(8)}, ""
		case "uuid":
			return textValue(g.uuid()), ""
		case "email":
			return textValue(fmt.Sprintf("%s%d@%s", localPart(label), i+1, exampleDomain(i))), ""
		case "uri":
			return textValue(fmt.Sprintf("https://%s/%s/%d", exampleDomain(i), strings.ReplaceAll(strings.ToLower(label), " ", "-"), i+1)), ""
		case "hostname":
			return textValue(fmt.Sprintf("%s-%d.%s", strings.ReplaceAll(strings.ToLower(label), " ", "-"), i+1, exampleDomain(i))), ""
		case "ipv4":
			// The blocks reserved for documentation (RFC 5737).
			nets := []string{"192.0.2", "198.51.100", "203.0.113"}
			if distinct {
				return textValue(fmt.Sprintf("%s.%d", nets[(i/254)%3], 1+i%254)), ""
			}
			return textValue(fmt.Sprintf("%s.%d", nets[g.rnd.intn(3)], g.rnd.in(1, 254))), ""
		case "ipv6":
			// The prefix reserved for documentation (RFC 3849).
			if distinct {
				return textValue(fmt.Sprintf("2001:db8::%x", i+1)), ""
			}
			return textValue(fmt.Sprintf("2001:db8::%x", g.rnd.in(1, 0xffff))), ""
		}
		if p := text(def["pattern"]); p != "" {
			return g.patterned(def, p)
		}
		return g.plainText(def, label, i)
	}
	return null, fmt.Sprintf("its type %s is not one a seed draws a value for", typ)
}

// localPart is the part of an email address before the @, from a label.
func localPart(label string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(label) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "address"
	}
	return b.String()
}

// exampleDomain is one of the domains reserved for documentation (RFC
// 2606), by the row.
func exampleDomain(i int64) string {
	return []string{"example.com", "example.org", "example.net"}[i%3]
}

// uuid is a version 4 UUID drawn from the generator.
func (g *gen) uuid() string {
	b := g.rnd.bytes(16)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// bound reads a numeric keyword as an exact number.
func bound(def map[string]any, key string) (*big.Rat, bool) {
	v, ok := def[key]
	if !ok {
		return nil, false
	}
	r, ok := new(big.Rat).SetString(text(v))
	return r, ok
}

// integerBounds are the smallest and largest values a field allows, from
// its minimum, maximum and exclusive bounds, with the type's own limits
// and a span of ten thousand where the design gives none.
func integerBounds(def map[string]any, format string) (lo, hi int64) {
	lo, hi = math.MinInt64, math.MaxInt64
	switch format {
	case "int32":
		lo, hi = math.MinInt32, math.MaxInt32
	case "uint64":
		lo = 0
	}
	given := false
	if m, ok := bound(def, "minimum"); ok {
		lo, given = ceil(m), true
	}
	if m, ok := bound(def, "exclusiveMinimum"); ok {
		lo, given = floor(m)+1, true
	}
	bounded := true
	if m, ok := bound(def, "maximum"); ok {
		hi = floor(m)
	} else if m, ok := bound(def, "exclusiveMaximum"); ok {
		hi = ceil(m) - 1
	} else {
		bounded = false
	}
	switch {
	case !given && !bounded:
		lo, hi = 0, 9999
	case !bounded:
		hi = lo + 9999
	case !given && hi >= 0:
		lo = 0 // a maximum alone: from zero up to it
	case !given:
		lo = hi - 9999
	}
	return lo, hi
}

func floor(r *big.Rat) int64 {
	q := new(big.Int).Div(r.Num(), r.Denom())
	return q.Int64()
}

func ceil(r *big.Rat) int64 {
	if r.IsInt() {
		return r.Num().Int64()
	}
	return floor(r) + 1
}

func (g *gen) integer(def map[string]any, format string, i int64, distinct bool) (value, string) {
	lo, hi := integerBounds(def, format)
	step := int64(1)
	if m, ok := bound(def, "multipleOf"); ok && m.IsInt() && m.Sign() > 0 {
		step = m.Num().Int64()
		if rem := lo % step; rem != 0 {
			if lo > 0 {
				lo += step - rem
			} else {
				lo -= rem
			}
		}
	}
	if lo > hi {
		return null, fmt.Sprintf("no integer lies between its minimum %d and maximum %d", lo, hi)
	}
	count := (hi-lo)/step + 1
	v := value{kind: kInt, uint: format == "uint64"}
	if distinct {
		if i >= count {
			return null, fmt.Sprintf("it is a key or unique and its bounds hold %d values, fewer than the rows", count)
		}
		v.num = lo + i*step
		return v, ""
	}
	v.num = lo + g.rnd.intn(count)*step
	return v, ""
}

func (g *gen) double(def map[string]any, i int64, distinct bool) (value, string) {
	lo, hi := 0.0, 9999.0
	if m, ok := bound(def, "minimum"); ok {
		lo, _ = m.Float64()
		hi = lo + 9999
	}
	if m, ok := bound(def, "exclusiveMinimum"); ok {
		lo, _ = m.Float64()
		lo += 0.01
		hi = lo + 9999
	}
	if m, ok := bound(def, "maximum"); ok {
		hi, _ = m.Float64()
	} else if m, ok := bound(def, "exclusiveMaximum"); ok {
		hi, _ = m.Float64()
		hi -= 0.01
	}
	if hi < lo && hi < 0 {
		lo = hi - 9999 // a maximum below zero alone
	}
	if lo > hi {
		return null, fmt.Sprintf("no number lies between its minimum %g and maximum %g", lo, hi)
	}
	if distinct {
		if v := lo + float64(i); v <= hi {
			return value{kind: kDouble, f: v}, ""
		}
		return null, "it is a key or unique and its bounds hold fewer values than the rows"
	}
	return value{kind: kDouble, f: math.Round((lo+g.rnd.float()*(hi-lo))*100) / 100}, ""
}

// decimal draws an exact decimal of the field's precision and scale, inside
// its bounds; without a maximum the whole part stays under ten thousand.
func (g *gen) decimal(def map[string]any, i int64, distinct bool) (value, string) {
	precision, _ := strconv.Atoi(text(def["precision"]))
	scale, _ := strconv.Atoi(text(def["scale"]))
	unit := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	whole := precision - scale
	limit := int64(9999)
	if whole < 4 {
		limit = int64(math.Pow10(whole)) - 1
	}
	lo, hi := int64(0), limit*unit.Int64()
	if m, ok := bound(def, "minimum"); ok {
		lo = ceil(new(big.Rat).Mul(m, new(big.Rat).SetInt(unit)))
		hi = lo + limit*unit.Int64()
	}
	if m, ok := bound(def, "exclusiveMinimum"); ok {
		lo = floor(new(big.Rat).Mul(m, new(big.Rat).SetInt(unit))) + 1
		hi = lo + limit*unit.Int64()
	}
	if m, ok := bound(def, "maximum"); ok {
		hi = floor(new(big.Rat).Mul(m, new(big.Rat).SetInt(unit)))
	} else if m, ok := bound(def, "exclusiveMaximum"); ok {
		hi = ceil(new(big.Rat).Mul(m, new(big.Rat).SetInt(unit))) - 1
	}
	if lo > hi {
		return null, "no decimal lies between its minimum and maximum"
	}
	var n int64
	if distinct {
		n = lo + i*unit.Int64()
		if n > hi {
			return null, "it is a key or unique and its bounds hold fewer whole values than the rows"
		}
	} else {
		n = g.rnd.in(lo, hi)
	}
	return value{kind: kDecimal, text: scaled(n, scale)}, ""
}

// scaled writes n / 10^scale with scale digits after the point.
func scaled(n int64, scale int) string {
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	s := strconv.FormatInt(n, 10)
	if scale == 0 {
		return sign + s
	}
	for len(s) <= scale {
		s = "0" + s
	}
	return sign + s[:len(s)-scale] + "." + s[len(s)-scale:]
}

// patterned draws text from the field's pattern, within its length.
func (g *gen) patterned(def map[string]any, pattern string) (value, string) {
	maxLength, _ := strconv.Atoi(text(def["maxLength"]))
	minLength, _ := strconv.Atoi(text(def["minLength"]))
	var why string
	for try := 0; try < 20; try++ {
		var s string
		s, why = patternValue(pattern, g.rnd)
		if why != "" {
			return null, why + "; give the field examples"
		}
		if n := len([]rune(s)); (maxLength == 0 || n <= maxLength) && n >= minLength {
			return textValue(s), ""
		}
	}
	return null, fmt.Sprintf("no value drawn from its pattern %s fit its length in 20 tries; give the field examples", pattern)
}

// plainText is "<label> <row>", repeated to the field's minLength and no
// longer than its maxLength; where the label does not fit, the row's
// number alone, padded to minLength with the label's letters and then
// zeros, so a short code column still holds a value of its length.
func (g *gen) plainText(def map[string]any, label string, i int64) (value, string) {
	maxLength, _ := strconv.Atoi(text(def["maxLength"]))
	minLength, _ := strconv.Atoi(text(def["minLength"]))
	n := strconv.FormatInt(i+1, 10)
	s := label + " " + n
	for len([]rune(s)) < minLength {
		s = label + " " + s
	}
	if maxLength > 0 && len([]rune(s)) > maxLength {
		if len(n) > maxLength {
			return null, fmt.Sprintf("its maxLength %d does not hold the row's number %s; give the field examples", maxLength, n)
		}
		if minLength > maxLength {
			return null, fmt.Sprintf("its minLength %d is above its maxLength %d, so no value fits", minLength, maxLength)
		}
		letters := []rune(strings.ReplaceAll(label, " ", ""))
		pad := minLength - len(n)
		if pad > len(letters) {
			pad = len(letters)
		}
		s = string(letters[:max(pad, 0)]) + strings.Repeat("0", max(minLength-len(n)-pad, 0)) + n
	}
	return textValue(s), ""
}

// array draws a list of the field's items, between minItems and maxItems,
// three at most where the design gives no maximum.
func (g *gen) array(def, orig map[string]any, label string, i int64) (value, string) {
	bounds := def
	if text(def["storage"]) == "json" && orig != nil {
		bounds = orig // the table reads the field as one JSON column
	}
	lo, _ := strconv.Atoi(text(bounds["minItems"]))
	hi, ok := strconv.Atoi(text(bounds["maxItems"]))
	if text(bounds["maxItems"]) == "" || ok != nil {
		hi = max(lo, 3)
	}
	if lo == 0 && hi > 0 {
		lo = 1
	}
	items := obj0(bounds["items"])
	origItems := obj0(orig["items"])
	if items == nil {
		items = origItems
	}
	v := value{kind: kList, items: []value{}, json: text(def["storage"]) == "json"}
	count := int(g.rnd.in(int64(lo), int64(hi)))
	for n := 0; n < count; n++ {
		item, why := g.fieldValue(items, origItems, label, int64(n), false)
		if why != "" {
			return null, "its items: " + why
		}
		v.items = append(v.items, item)
	}
	return v, ""
}

// object draws a value object, an entity's record or a map as JSON: the
// schema's parts, each required one given and an optional one in three
// rows of four; a map gets two members.
func (g *gen) object(def, orig map[string]any, label string, i int64) (value, string) {
	source := def
	if text(def["storage"]) == "json" && orig != nil {
		source = orig
		if typeIs(orig, "array") {
			return g.array(def, orig, label, i)
		}
	}
	v := value{kind: kJSON, members: map[string]value{}}
	schema := g.schemaOf(source)
	if schema == nil {
		return null, fmt.Sprintf("%s names no schema of the specification", text(source["$ref"]))
	}
	props := obj0(schema["properties"])
	if len(props) == 0 {
		switch extra := schema["additionalProperties"].(type) {
		case map[string]any:
			for n := int64(1); n <= 2; n++ {
				m, why := g.fieldValue(extra, extra, "Value", n-1, false)
				if why != "" {
					return null, "its members: " + why
				}
				v.members[fmt.Sprintf("key%d", n)] = m
			}
		case bool:
			if extra {
				v.members["key1"] = textValue("Value 1")
			}
		}
		return v, ""
	}
	required := map[string]bool{}
	for _, f := range list(schema["required"]) {
		required[text(f)] = true
	}
	for _, p := range sortedKeys(props) {
		pd := obj0(props[p])
		if !required[p] && g.rnd.chance(optionalOnceIn) {
			continue
		}
		m, why := g.fieldValue(pd, pd, words(p), i, false)
		if why != "" {
			return null, fmt.Sprintf("its part %s: %s", p, why)
		}
		v.members[p] = m
	}
	return v, ""
}

// schemaOf is the schema an object field draws its parts from: the one
// its $ref names, an entity's record, or the field's own properties; nil
// when the $ref names nothing.
func (g *gen) schemaOf(source map[string]any) map[string]any {
	ref := text(source["$ref"])
	switch {
	case strings.HasPrefix(ref, "#/schemas/"):
		return obj0(obj0(g.spec["schemas"])[strings.TrimPrefix(ref, "#/schemas/")])
	case strings.HasPrefix(ref, "#/entities/"):
		return obj0(obj0(g.spec["entities"])[strings.TrimPrefix(ref, "#/entities/")])
	}
	return source
}

// partOf is the definition of one member of an object example: the
// schema's part of that name, the additionalProperties schema of a map,
// or an empty definition.
func (g *gen) partOf(def, orig map[string]any, member string) map[string]any {
	source := def
	if text(def["storage"]) == "json" && orig != nil {
		source = orig
	}
	schema := g.schemaOf(source)
	if pd := obj0(obj0(schema["properties"])[member]); pd != nil {
		return pd
	}
	if extra := obj0(schema["additionalProperties"]); extra != nil {
		return extra
	}
	return map[string]any{}
}

// fromExample turns an example, a const or a default the design writes
// into a value of the field's type: each member of an object by the
// schema's part it is, each item of a list by the field's items.
func (g *gen) fromExample(x any, def, orig map[string]any, label string) value {
	typ, format := typeOf(def)
	switch e := x.(type) {
	case bool:
		return boolValue(e)
	case json.Number:
		switch {
		case typ == "integer" || format == "int64" || format == "uint64":
			n, _ := e.Int64()
			return value{kind: kInt, num: n, uint: format == "uint64"}
		case format == "decimal":
			return value{kind: kDecimal, text: e.String()}
		case typ == "number":
			f, _ := e.Float64()
			return value{kind: kDouble, f: f, text: e.String()}
		case typ == "" || typ == "object":
			// A member no part types stays a number, as it was given.
			if n, err := e.Int64(); err == nil {
				return value{kind: kInt, num: n}
			}
			f, _ := e.Float64()
			return value{kind: kDouble, f: f, text: e.String()}
		}
		return textValue(e.String())
	case string:
		switch {
		case typ == "enum":
			return value{kind: kText, text: e, enum: true}
		case format == "decimal":
			return value{kind: kDecimal, text: e}
		case format == "int64" || format == "uint64" || typ == "integer":
			n, _ := strconv.ParseInt(e, 10, 64)
			return value{kind: kInt, num: n, uint: format == "uint64"}
		case format == "date":
			if t, err := time.Parse("2006-01-02", e); err == nil {
				return dateValue(t)
			}
		case format == "date-time":
			if t, err := time.Parse(time.RFC3339Nano, e); err == nil {
				return value{kind: kTimestamp, t: t.UTC()}
			}
		case format == "time":
			if t, err := time.Parse("15:04:05", e); err == nil {
				return value{kind: kTimeOfDay, num: int64(t.Hour()*3600 + t.Minute()*60 + t.Second())}
			}
		case format == "duration":
			if d, ok := expr.ParseDuration(e); ok {
				return value{kind: kDuration, d: d}
			}
		case format == "byte" || format == "binary":
			return value{kind: kBytes, bytes: []byte(e)}
		}
		return textValue(e)
	case []any:
		v := value{kind: kList, items: []value{}, json: text(def["storage"]) == "json"}
		items := obj0(def["items"])
		if items == nil && text(def["storage"]) == "json" {
			items = obj0(orig["items"])
		}
		for _, it := range e {
			v.items = append(v.items, g.fromExample(it, items, obj0(orig["items"]), label))
		}
		return v
	case map[string]any:
		v := value{kind: kJSON, members: map[string]value{}}
		for _, k := range sortedKeys(e) {
			pd := g.partOf(def, orig, k)
			v.members[k] = g.fromExample(e[k], pd, pd, words(k))
		}
		return v
	}
	return null
}
