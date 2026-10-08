// Package wirename writes a property's name as it goes on the wire, by the
// rule info.wireNames names (ADR-062). The validator, the readers and the
// generators all map a name here, so they agree on every name.
package wirename

import "strings"

// SnakeCase is the only rule info.wireNames takes.
const SnakeCase = "snake_case"

// Snake writes a camelCase name in snake_case: a capital starts a word
// after a lower-case letter or a digit, and so does the last capital of a
// run of capitals that a lower-case letter follows. loanedOn is loaned_on,
// userID and userId are both user_id, and HTTPServer is http_server.
func Snake(name string) string {
	var b strings.Builder
	rs := []rune(name)
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

// Of is a name on the wire under rule, the info.wireNames of a
// specification: the name as written when rule is empty.
func Of(rule, name string) string {
	if rule == SnakeCase {
		return Snake(name)
	}
	return name
}

// Camel is the camelCase name a snake_case wire name is read as, and
// whether Snake writes it back unchanged: line_2 is read as line2, which
// goes on the wire as line2, so it does not round-trip.
func Camel(wire string) (string, bool) {
	var b strings.Builder
	up := false
	for _, r := range wire {
		if r == '_' {
			up = true
			continue
		}
		if up && r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		up = false
		b.WriteRune(r)
	}
	name := b.String()
	return name, name != "" && Snake(name) == wire
}
