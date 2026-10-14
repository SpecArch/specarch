package extract

import (
	"fmt"
	"regexp"

	"github.com/SpecArch/specarch/internal/wirename"
)

// upperSnakeWord is a name written in capitals, digits and underscores
// with a capital first: a role's name as a source writes it on the wire
// under info.roleNames upper_snake.
var upperSnakeWord = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// roleNames is how a source writes its roles' names (ADR-099). A source
// that names no role in kebab-case and at least one in upper snake case
// writes them on the wire as info.roleNames upper_snake says: each is read
// as its kebab-case name when the rule gives the name back unchanged. A
// source that names roles both ways is read as kebab-case, as before.
type roleNames struct {
	upper bool
	mixed bool // a source with roles in upper snake case beside kebab-case ones
}

// readRoleNames tells how the roles named are written.
func readRoleNames(names []string) roleNames {
	var rn roleNames
	kebabSeen := false
	for _, n := range names {
		switch {
		case roleWord.MatchString(n):
			kebabSeen = true
		case upperSnakeWord.MatchString(n):
			rn.upper = true
		}
	}
	if kebabSeen && rn.upper {
		return roleNames{mixed: true}
	}
	return rn
}

// rule is the info.roleNames the source's roles say, or "".
func (rn roleNames) rule() string {
	if rn.upper {
		return wirename.UpperSnake
	}
	return ""
}

// name is the role's name in the specification, and whether it is one: a
// kebab-case name as written, and under upper_snake the kebab-case name of
// an upper snake one that goes back on the wire unchanged.
func (rn roleNames) name(wire string) (string, bool) {
	if !rn.upper {
		return wire, roleWord.MatchString(wire)
	}
	if !upperSnakeWord.MatchString(wire) {
		return wire, false
	}
	n := kebab(wire)
	return n, roleWord.MatchString(n) && wirename.Role(wirename.UpperSnake, n) == wire
}

// why says why a role's name is left out: it is not kebab-case, and when
// the source names its roles in upper snake case, why the rule cannot map
// it.
func (rn roleNames) why(wire string) string {
	switch {
	case rn.mixed && upperSnakeWord.MatchString(wire):
		return "the role's name is in upper snake case while other roles of the source are kebab-case, and info.roleNames maps every role or none"
	case rn.upper && upperSnakeWord.MatchString(wire):
		n := kebab(wire)
		if !roleWord.MatchString(n) {
			return fmt.Sprintf("the role's name is read as %s under info.roleNames upper_snake, which is not kebab-case, which a role name is", n)
		}
		return fmt.Sprintf("the role's name is read as %s, which goes on the wire as %s under info.roleNames upper_snake, not as %s", n, wirename.Role(wirename.UpperSnake, n), wire)
	case rn.upper:
		return "the role's name is not in upper snake case, the form every other role of the source takes, and not kebab-case, which a role name is"
	}
	return "the role's name is not kebab-case, which a role name is"
}
