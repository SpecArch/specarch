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
