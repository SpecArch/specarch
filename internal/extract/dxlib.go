package extract

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// The names dxlib_module gives its privileges: words in capitals and
// digits joined by underscores, in segments joined by dots, such as
// ORDER.CREATE, GLOBAL.SET_MAINTENANCE_MODE and EVERYTHING.
var privilegeWord = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*(\.[A-Z][A-Z0-9]*(_[A-Z0-9]+)*)*$`)

// privilegePermission maps a privilege name of dxlib_module's form to a
// permission name by the fixed rule of ADR-076: the name in lower case,
// with every underscore a dot. It is "" for a name of another form, and
// for one the rule would make into no permission name or into public.
func privilegePermission(name string) string {
	if !privilegeWord.MatchString(name) {
		return ""
	}
	p := strings.ReplaceAll(strings.ToLower(name), "_", ".")
	if !permissionWord.MatchString(p) || p == "public" {
		return ""
	}
	return p
}

// privilegeWhy is the why of a permission mapped from a privilege name.
func privilegeWhy(name, permission string) string {
	why := fmt.Sprintf("dxlib names the privilege %s, and by the rule of ADR-076 a privilege named in capitals, digits and underscores, in segments joined by dots, is the permission of that name in lower case with every underscore a dot: %s.", name, permission)
	if name == "EVERYTHING" {
		why += " dxlib_module lets a caller holding EVERYTHING through every check, so only a role granted EVERYTHING holds this permission."
	}
	return why
}

// privilegeNames tells how each privilege name of one surface maps to a
// permission: a lower-case dotted name is the permission as it is, a name
// of dxlib_module's form is mapped by the rule, and two names that give
// one permission collide, so that neither is written.
type privilegeNames struct {
	byPermission map[string][]string // permission -> the privilege names that give it, sorted
}

func newPrivilegeNames(names []string) *privilegeNames {
	pn := &privilegeNames{byPermission: map[string][]string{}}
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			continue
		}
		seen[n] = true
		if p := pn.permission(n); p != "" {
			pn.byPermission[p] = append(pn.byPermission[p], n)
		}
	}
	for p := range pn.byPermission {
		sort.Strings(pn.byPermission[p])
	}
	return pn
}

// permission is the permission a privilege name gives, before collisions.
func (pn *privilegeNames) permission(name string) string {
	if name != "public" && permissionWord.MatchString(name) {
		return name
	}
	return privilegePermission(name)
}

// read is how one privilege name is read: the permission and whether the
// rule mapped it, or the names it collides with when another name gives
// the same permission, or "" when it gives none.
func (pn *privilegeNames) read(name string) (permission string, mapped bool, collides []string) {
	p := pn.permission(name)
	if p == "" {
		return "", false, nil
	}
	if others := pn.byPermission[p]; len(others) > 1 {
		return p, false, others
	}
	return p, p != name, nil
}

func doOrDoes(n int) string {
	if n == 1 {
		return "does"
	}
	return "do"
}

func anotherPrivilege(n int) string {
	if n == 1 {
		return "another privilege"
	}
	return "other privileges"
}

// grantProblem is the must question on a granted privilege that gives no
// one permission, or "" when it gives one: EVERYTHING, which is never
// expanded (ADR-076), a name the rule does not map, and a name that gives
// the permission another granted name gives too.
func (pn *privilegeNames) grantProblem(role, name string) (question, why string) {
	p, _, collides := pn.read(name)
	switch {
	case name == "EVERYTHING":
		return fmt.Sprintf("%s is granted EVERYTHING, which dxlib_module lets through every check, so %s holds every permission, which no list of grants can say. Which permissions does %s hold, or is it meant to hold every one?", role, role, role),
			"A grant of EVERYTHING is not expanded: the permissions it stands for change whenever one is added, and a role's list names each one (ADR-076)."
	case p == "":
		return fmt.Sprintf("%s is granted the privilege %s, which is not a permission name (lower-case words joined by dots), nor a privilege in capitals that the rule of ADR-076 maps to one. Which permission does %s grant?", role, name, role),
			"The meta-model names a permission in lower-case words joined by dots, and renaming the privilege by a rule of the reader's own would write a grant the code does not make."
	case len(collides) > 0:
		var others []string
		for _, c := range collides {
			if c != name {
				others = append(others, c)
			}
		}
		return fmt.Sprintf("%s is granted the privilege %s, which gives the permission %s, and so %s %s, which dxlib checks as %s. Which permission does %s grant?", role, name, p, doOrDoes(len(others)), joinAnd(others), anotherPrivilege(len(others)), role),
			"Two privileges dxlib tells apart would become one permission, which would write a grant the code does not make."
	}
	return "", ""
}
