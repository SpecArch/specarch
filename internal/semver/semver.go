// Package semver reads and orders Semantic Versioning 2.0.0 versions and
// says how far one steps from another. The validator's release rules and
// the diff verb use it, so they measure a release the same way.
package semver

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a Semantic Versioning 2.0.0 version, without build metadata.
type Version struct {
	Major, Minor, Patch int
	Pre                 string
}

// Parse reads a version; ok is false when it is not one.
func Parse(v string) (Version, bool) {
	core, pre, _ := strings.Cut(v, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	var n [3]int
	for i, p := range parts {
		x, err := strconv.Atoi(p)
		if err != nil || x < 0 {
			return Version{}, false
		}
		n[i] = x
	}
	return Version{n[0], n[1], n[2], pre}, true
}

// Core is the version without its pre-release tag.
func (v Version) Core() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

// Compare orders two versions by SemVer's precedence (rule 11).
func Compare(a, b Version) int {
	for _, d := range [][2]int{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}} {
		if d[0] != d[1] {
			if d[0] < d[1] {
				return -1
			}
			return 1
		}
	}
	switch {
	case a.Pre == b.Pre:
		return 0
	case a.Pre == "":
		return 1
	case b.Pre == "":
		return -1
	}
	x, y := strings.Split(a.Pre, "."), strings.Split(b.Pre, ".")
	for i := 0; i < len(x) && i < len(y); i++ {
		if c := compareIdentifiers(x[i], y[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(x) < len(y):
		return -1
	case len(x) > len(y):
		return 1
	}
	return 0
}

// compareIdentifiers compares two dot-separated pre-release identifiers:
// numbers numerically, below any other identifier, and the rest in ASCII
// order.
func compareIdentifiers(a, b string) int {
	an, bn := isNumeric(a), isNumeric(b)
	switch {
	case an && bn:
		if len(a) != len(b) {
			if len(a) < len(b) {
				return -1
			}
			return 1
		}
	case an:
		return -1
	case bn:
		return 1
	}
	return strings.Compare(a, b)
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// The steps, by rank: 1 patch, 2 minor, 3 major; 0 is none.
const (
	None = iota
	Patch
	Minor
	Major
)

// StepNames names each step by its rank.
var StepNames = []string{"", "patch", "minor", "major"}

// Rank is the rank of an impact named patch, minor or major, or None.
func Rank(impact string) int {
	for i, n := range StepNames {
		if n == impact && i > None {
			return i
		}
	}
	return None
}

// Step is how far cur steps from prev.
func Step(prev, cur Version) int {
	switch {
	case cur.Major > prev.Major:
		return Major
	case cur.Major == prev.Major && cur.Minor > prev.Minor:
		return Minor
	case cur.Major == prev.Major && cur.Minor == prev.Minor && cur.Patch > prev.Patch:
		return Patch
	}
	return None
}

// Needed is the step an impact needs from prev: before 1.0.0 a major
// change needs the minor step and anything else the patch step.
func Needed(prev Version, impact int) int {
	if prev.Major == 0 && impact > None {
		return max(impact-1, Patch)
	}
	return impact
}

// Next is the version one step from prev.
func Next(prev Version, step int) string {
	switch step {
	case Major:
		return fmt.Sprintf("%d.0.0", prev.Major+1)
	case Minor:
		return fmt.Sprintf("%d.%d.0", prev.Major, prev.Minor+1)
	}
	return fmt.Sprintf("%d.%d.%d", prev.Major, prev.Minor, prev.Patch+1)
}
