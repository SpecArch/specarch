// Package implies expands a set of permissions through the implications
// the design declares on them (permissions.<name>.implies, ADR-100): a
// holder of a permission holds every one it implies, transitively.
package implies

// Holds is every permission a holder of the given ones holds: those and
// every one they imply, transitively. impliedBy gives the permissions one
// permission lists under implies; a cycle is followed once.
func Holds(perms []string, impliedBy func(string) []string) map[string]bool {
	held := map[string]bool{}
	queue := append([]string(nil), perms...)
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if held[p] {
			continue
		}
		held[p] = true
		queue = append(queue, impliedBy(p)...)
	}
	return held
}

// Through is the first of the given permissions, in their order, whose
// implications reach perm, or "" when none does or perm is one of them.
func Through(perms []string, perm string, impliedBy func(string) []string) string {
	for _, p := range perms {
		if p == perm {
			return ""
		}
	}
	for _, p := range perms {
		if Holds([]string{p}, impliedBy)[perm] {
			return p
		}
	}
	return ""
}
