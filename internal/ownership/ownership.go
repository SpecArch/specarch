// Package ownership reads the ownedBy marks of an implementation file's
// mappings, which every generator follows: an element another stakeholder
// owns, and everything under its pointer, is described and compared but
// never generated (ADR-046).
package ownership

import "strings"

// Owned maps the pointer of each owned element to the stakeholder that
// owns it.
type Owned map[string]string

// Of reads the marks from an implementation file's content, as plain values.
func Of(content map[string]any) Owned {
	out := Owned{}
	mappings, _ := content["mappings"].(map[string]any)
	for ptr, m := range mappings {
		mapping, _ := m.(map[string]any)
		if by, _ := mapping["ownedBy"].(string); by != "" {
			out[ptr] = by
		}
	}
	return out
}

// Covers reports whether the element at a pointer is owned: marked itself,
// or under a marked element.
func (o Owned) Covers(ptr string) bool {
	for p := range o {
		if ptr == p || strings.HasPrefix(ptr, p+"/") {
			return true
		}
	}
	return false
}

// Entity is the pointer of an entity, a view or an enum by its section and
// name.
func Entity(section, name string) string {
	return "#/" + section + "/" + Escape(name)
}

// Operation is the pointer of an operation by its path and method.
func Operation(path, method string) string {
	return "#/paths/" + Escape(path) + "/" + method
}

// Escape writes a key as a JSON pointer token (RFC 6901): ~ as ~0, / as ~1.
func Escape(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}
