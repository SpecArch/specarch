package genscripts

import (
	"fmt"
	"strings"

	"github.com/SpecArch/specarch/internal/gensql"
)

// The MariaDB form of a script (ADR-167): the same steps as the psql one,
// as one compound statement the mariadb client runs. Each value is a user
// variable the operator sets in the client's --init-command, since the
// client has no variables of its own and a SOURCE after a SET does not
// stop on an error in the file it reads. MariaDB's CAST cuts or rounds a
// value it cannot hold and warns rather than refusing, so the text of
// each variable is checked against its column's form before anything is
// read, and the session is made strict, so a value too long for its
// column is refused rather than cut.

// mariaNow is the instant of the change as the type-rendering idiom keeps
// a date-time on MariaDB: UTC text with nine fractional digits, whose byte
// order is time order. The server keeps microseconds, so the last three
// digits are zeros.
const mariaNow = "DATE_FORMAT(UTC_TIMESTAMP(6), '%Y-%m-%dT%H:%i:%s.%f000Z')"

// writeMariaDB is the text of one MariaDB script under its header.
func (g *gen) writeMariaDB(e entry, entity string, t gensql.Table, c change, path string) string {
	var b strings.Builder
	what := e.id
	if e.summary != "" {
		what += ": " + e.summary
	}
	fmt.Fprintf(&b, "-- %s\n", strings.TrimSuffix(what, "."))
	fmt.Fprintf(&b, "-- Changes exactly %s of table %s: %s.\n", records(c.count, entity), t.Name, g.describe(c))

	var vars []gensql.Column
	if c.keyed {
		vars = append(vars, c.key)
	}
	vars = append(vars, c.sets...)
	var audit []gensql.Column
	if c.kind != "delete" || c.soft.Name != "" {
		for _, col := range t.Columns {
			if col.Role == gensql.RoleModified {
				audit = append(audit, col)
				if !isTime(col) {
					vars = append(vars, col)
				}
			}
		}
	}
	var usage []string
	optional := false
	for _, v := range vars {
		if v.Nullable && !v.Key {
			usage = append(usage, fmt.Sprintf("[@%s = '...']", v.Name))
			optional = true
			continue
		}
		usage = append(usage, fmt.Sprintf("@%s = '...'", v.Name))
	}
	if len(usage) > 0 {
		fmt.Fprintf(&b, "-- Dry run: mariadb --init-command=\"SET %s\" <database> < %s\n", strings.Join(usage, ", "), path)
		b.WriteString("-- Apply: the same with @apply = true in the SET. Without it every check runs against the data and the change is rolled back.\n")
	} else {
		fmt.Fprintf(&b, "-- Dry run: mariadb <database> < %s\n", path)
		b.WriteString("-- Apply: the same with --init-command=\"SET @apply = true\". Without it every check runs against the data and the change is rolled back.\n")
	}
	if optional {
		b.WriteString("-- A variable in brackets may be left out: its column is set to NULL.\n")
	}
	b.WriteString("SET SESSION sql_mode = CONCAT_WS(',', NULLIF(REPLACE(@@SESSION.sql_mode, 'NO_BACKSLASH_ESCAPES', ''), ''), 'STRICT_ALL_TABLES');\n")
	b.WriteString("DELIMITER //\nBEGIN NOT ATOMIC\n  DECLARE found BIGINT;\n  DECLARE changed BIGINT;\n  DECLARE refusal TEXT;\n")
	b.WriteString("  DECLARE EXIT HANDLER FOR SQLEXCEPTION BEGIN ROLLBACK; RESIGNAL; END;\n")
	b.WriteString("  -- The variables: each is set, and its text is a value of its column.\n")
	fmt.Fprintf(&b, "  IF @apply IS NOT NULL AND NOT LOWER(@apply) IN ('true', 'false', '1', '0') THEN\n")
	fmt.Fprintf(&b, "    SET refusal = %s;\n    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = refusal;\n  END IF;\n",
		concat(fmt.Sprintf("%s: the variable @apply is %%, which is not true or false; nothing was read", path), "QUOTE(@apply)"))
	for _, v := range vars {
		if !(v.Nullable && !v.Key) {
			fmt.Fprintf(&b, "  IF @%s IS NULL THEN\n", v.Name)
			fmt.Fprintf(&b, "    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = %s;\n  END IF;\n",
				mariaQuote(fmt.Sprintf("%s: the variable @%s is not set; give it as SET @%s = '<%s>' (%s) in --init-command", path, v.Name, v.Name, columnOf(entity, v), v.Type)))
		}
		check, form := mariaForm(v)
		if check == "" {
			continue
		}
		fmt.Fprintf(&b, "  IF @%s IS NOT NULL AND NOT (%s) THEN\n", v.Name, check)
		fmt.Fprintf(&b, "    SET refusal = %s;\n    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = refusal;\n  END IF;\n",
			concat(fmt.Sprintf("%s: the variable @%s is %%, which is not %s for %s (%s); nothing was read", path, v.Name, form, columnOf(entity, v), v.Type), "QUOTE(@"+v.Name+")"))
	}
	b.WriteString("  START TRANSACTION;\n  script: BEGIN\n")

	where := strings.ReplaceAll(g.where(c), "\n    AND ", "\n        AND ")
	holds := g.holds(c)
	key := ""
	if c.keyed {
		key = "@" + c.key.Name
	}
	step := 0
	next := func() int { step++; return step }
	if c.keyed && holds != "" {
		fmt.Fprintf(&b, "    -- %d. Already applied: the record already holds what the change writes.\n", next())
		fmt.Fprintf(&b, "    IF EXISTS (SELECT 1 FROM %s WHERE %s AND %s) THEN\n", t.Name, g.keyEquals(c.key), holds)
		fmt.Fprintf(&b, "      SELECT %s AS message;\n", concat(fmt.Sprintf("%s: the %s with %s %% already holds %s; nothing to change", e.what, entity, c.key.Name, g.describeHolds(c)), key))
		b.WriteString("      ROLLBACK;\n      LEAVE script;\n    END IF;\n")
	}
	fmt.Fprintf(&b, "    -- %d. Precondition: exactly %s to change, locked until the end.\n", next(), records(c.count, entity))
	fmt.Fprintf(&b, "    SELECT COUNT(*) INTO found FROM %s\n      WHERE %s\n      FOR UPDATE;\n", t.Name, where)
	fmt.Fprintf(&b, "    IF found <> %d THEN\n", c.count)
	args := []string{"found"}
	if c.keyed {
		args = append(args, key)
	}
	fmt.Fprintf(&b, "      SET refusal = %s;\n      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = refusal;\n    END IF;\n",
		concat(fmt.Sprintf("%s: %% %s records hold %s, and the change expects exactly %d; nothing was changed", e.what, entity, g.condText(c), c.count), args...))
	fmt.Fprintf(&b, "    -- %d. The change, under the same condition.\n", next())
	switch {
	case c.kind == "delete" && c.soft.Name == "":
		fmt.Fprintf(&b, "    DELETE FROM %s\n      WHERE %s;\n", t.Name, where)
	default:
		var sets []string
		switch c.kind {
		case "transition":
			sets = append(sets, fmt.Sprintf("%s = %s", c.state.Name, mariaQuote(c.to)))
		case "put":
			for _, col := range c.sets {
				sets = append(sets, fmt.Sprintf("%s = %s", col.Name, mariaValue(col)))
			}
		case "delete":
			sets = append(sets, c.soft.Name+" = true")
		}
		for _, col := range audit {
			if isTime(col) {
				sets = append(sets, col.Name+" = "+mariaNow)
			} else {
				sets = append(sets, fmt.Sprintf("%s = %s", col.Name, mariaValue(col)))
			}
		}
		fmt.Fprintf(&b, "    UPDATE %s\n      SET %s\n      WHERE %s;\n", t.Name, strings.Join(sets, ",\n        "), where)
	}
	b.WriteString("    SET changed = ROW_COUNT();\n")
	post := "the exact count"
	if c.keyed {
		post += ", and the record holds what was written"
	}
	fmt.Fprintf(&b, "    -- %d. Postcondition: %s.\n", next(), post)
	fmt.Fprintf(&b, "    IF changed <> %d THEN\n", c.count)
	fmt.Fprintf(&b, "      SET refusal = %s;\n      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = refusal;\n    END IF;\n",
		concat(fmt.Sprintf("%s: the change wrote %% %s records, and exactly %d was expected; another writer may have changed them after they were counted; nothing was changed", e.what, entity, c.count), "changed"))
	if c.keyed {
		check, says := "NOT EXISTS (SELECT 1 FROM "+t.Name+" WHERE "+g.keyEquals(c.key)+" AND "+holds+")", "does not hold "+g.describeHolds(c)
		if holds == "" {
			check, says = "EXISTS (SELECT 1 FROM "+t.Name+" WHERE "+g.keyEquals(c.key)+")", "is still there"
		}
		fmt.Fprintf(&b, "    IF %s THEN\n", check)
		fmt.Fprintf(&b, "      SET refusal = %s;\n      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = refusal;\n    END IF;\n",
			concat(fmt.Sprintf("%s: after the change the %s with %s %% %s; nothing was changed", e.what, entity, c.key.Name, says), key))
	}
	fmt.Fprintf(&b, "    -- %d. Dry run unless apply is true.\n", next())
	b.WriteString("    IF LOWER(@apply) IN ('true', '1') THEN\n")
	fmt.Fprintf(&b, "      SELECT %s AS message;\n      COMMIT;\n", mariaQuote(fmt.Sprintf("%s: %s changed; committing", e.what, records(c.count, entity))))
	b.WriteString("    ELSE\n")
	fmt.Fprintf(&b, "      SELECT %s AS message;\n      ROLLBACK;\n", mariaQuote(fmt.Sprintf("%s: dry run: %s would change, and every check passed; rolling back. Run it again with @apply = true to make the change", e.what, records(c.count, entity))))
	b.WriteString("    END IF;\n  END script;\nEND//\nDELIMITER ;\n")
	return b.String()
}

// mariaValue is a variable's value as its column holds it. The variable's
// text has been checked against the column's form (mariaForm), so the
// cast is exact: a whole number and a decimal go through DECIMAL(65,s),
// which holds any number of up to 65 digits, and the column refuses one
// it cannot hold; text is cast without a length, so a key given too long
// matches no record and a value too long is refused by the strict
// session rather than cut.
func mariaValue(col gensql.Column) string {
	v := "@" + col.Name
	base, size := mariaType(col.Type)
	switch base {
	case "INT", "INTEGER", "BIGINT", "SMALLINT", "TINYINT", "MEDIUMINT":
		return "CAST(" + v + " AS DECIMAL(65,0))"
	case "DECIMAL", "NUMERIC":
		return fmt.Sprintf("CAST(%s AS DECIMAL(65,%d))", v, scale(size))
	case "DOUBLE", "FLOAT", "REAL":
		return "CAST(" + v + " AS DOUBLE)"
	case "BOOLEAN", "BOOL":
		return "(LOWER(" + v + ") IN ('true', '1'))"
	case "DATE":
		return "CAST(" + v + " AS DATE)"
	case "TIME":
		return "CAST(" + v + " AS TIME(6))"
	case "BLOB", "LONGBLOB", "MEDIUMBLOB", "TINYBLOB", "VARBINARY", "BINARY":
		return "UNHEX(" + v + ")"
	}
	return "CAST(" + v + " AS CHAR)"
}

// mariaSame is the condition that column col holds its variable's value.
// <=> treats two NULLs as equal, as IS NOT DISTINCT FROM does. A text
// column is compared byte for byte: under the server's default collation
// (case- and accent-insensitive, and padding trailing spaces) 'the hobbit'
// equals 'The Hobbit', so a change of case alone would be taken as already
// made. The value is converted to utf8mb4 first, which holds every
// character set a column may have, so the collation fits it.
func mariaSame(col gensql.Column) string {
	v := mariaValue(col)
	if v == "CAST(@"+col.Name+" AS CHAR)" {
		v = "CONVERT(@" + col.Name + " USING utf8mb4) COLLATE utf8mb4_nopad_bin"
	}
	return col.Name + " <=> " + v
}

// mariaForm is the condition a variable's text must meet to be a value of
// its column, and the form in words, or "" when any text is one.
func mariaForm(col gensql.Column) (string, string) {
	v := "@" + col.Name
	base, size := mariaType(col.Type)
	unsigned := strings.Contains(strings.ToUpper(col.Type), "UNSIGNED")
	switch {
	case col.Format == "uuid":
		return v + " REGEXP '^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}\\\\z'", "a uuid"
	case col.Format == "date-time":
		return v + " REGEXP '(?-i)^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}[.][0-9]{9}Z\\\\z'", "an instant in UTC with nine fractional digits, as 2006-01-02T15:04:05.000000000Z"
	}
	switch base {
	case "INT", "INTEGER", "BIGINT", "SMALLINT", "TINYINT", "MEDIUMINT":
		if unsigned {
			return v + " REGEXP '^[+]?[0-9]+\\\\z'", "a whole number of zero or more"
		}
		return v + " REGEXP '^[+-]?[0-9]+\\\\z'", "a whole number"
	case "DECIMAL", "NUMERIC":
		if s := scale(size); s > 0 {
			return fmt.Sprintf("%s REGEXP '^[+-]?[0-9]+([.][0-9]{1,%d})?\\\\z'", v, s), fmt.Sprintf("a decimal with at most %d digits after the point", s)
		}
		return v + " REGEXP '^[+-]?[0-9]+\\\\z'", "a whole number"
	case "DOUBLE", "FLOAT", "REAL":
		return v + " REGEXP '^[+-]?([0-9]+([.][0-9]*)?|[.][0-9]+)([eE][+-]?[0-9]+)?\\\\z'", "a number"
	case "BOOLEAN", "BOOL":
		return "LOWER(" + v + ") IN ('true', 'false', '1', '0')", "true or false"
	case "DATE":
		return v + " REGEXP '^[0-9]{4}-[0-9]{2}-[0-9]{2}\\\\z' AND CAST(" + v + " AS DATE) IS NOT NULL", "a date, as 2006-01-02"
	case "TIME":
		return v + " REGEXP '^[0-9]{2}:[0-9]{2}:[0-9]{2}([.][0-9]{1,6})?\\\\z' AND CAST(" + v + " AS TIME(6)) IS NOT NULL", "a time of day, as 15:04:05"
	case "BLOB", "LONGBLOB", "MEDIUMBLOB", "TINYBLOB", "VARBINARY", "BINARY":
		return v + " REGEXP '^([0-9A-Fa-f]{2})*\\\\z'", "hexadecimal"
	}
	return "", ""
}

// mariaType is a column type's name, upper case, and what its parentheses
// hold.
func mariaType(typ string) (string, string) {
	base, size, _ := strings.Cut(typ, "(")
	size, _, _ = strings.Cut(size, ")")
	fields := strings.Fields(strings.ToUpper(base))
	if len(fields) == 0 {
		return "", size
	}
	return fields[0], size
}

// scale is the scale of a DECIMAL(p,s), or 0.
func scale(size string) int {
	_, s, ok := strings.Cut(size, ",")
	if !ok {
		return 0
	}
	n := 0
	fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n
}

// mariaQuote is a MariaDB string literal: the backslash is an escape
// character there unless the session says otherwise, so it is doubled as
// the quote is.
func mariaQuote(s string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "''").Replace(s) + "'"
}

// concat is a message with values, written as the psql scripts write one
// for RAISE: each % is the place of the next value, %% a percent sign.
func concat(format string, values ...string) string {
	var parts []string
	var lit strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			lit.WriteByte(format[i])
			continue
		}
		if i+1 < len(format) && format[i+1] == '%' {
			lit.WriteByte('%')
			i++
			continue
		}
		if lit.Len() > 0 {
			parts = append(parts, mariaQuote(lit.String()))
			lit.Reset()
		}
		if len(values) > 0 {
			parts = append(parts, values[0])
			values = values[1:]
		}
	}
	if lit.Len() > 0 {
		parts = append(parts, mariaQuote(lit.String()))
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "CONCAT(" + strings.Join(parts, ", ") + ")"
}
