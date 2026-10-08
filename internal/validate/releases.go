package validate

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/SpecArch/specarch/internal/source"
)

// version is a Semantic Versioning 2.0.0 version, without build metadata.
type version struct {
	major, minor, patch int
	pre                 string
}

func parseVersion(v string) (version, bool) {
	core, pre, _ := strings.Cut(v, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return version{}, false
	}
	var n [3]int
	for i, p := range parts {
		x, err := strconv.Atoi(p)
		if err != nil || x < 0 {
			return version{}, false
		}
		n[i] = x
	}
	return version{n[0], n[1], n[2], pre}, true
}

func (v version) core() string { return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch) }

// compareVersions orders two versions by SemVer's precedence (rule 11).
func compareVersions(a, b version) int {
	for _, d := range [][2]int{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if d[0] != d[1] {
			if d[0] < d[1] {
				return -1
			}
			return 1
		}
	}
	switch {
	case a.pre == b.pre:
		return 0
	case a.pre == "":
		return 1
	case b.pre == "":
		return -1
	}
	x, y := strings.Split(a.pre, "."), strings.Split(b.pre, ".")
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

// The version steps, smallest first.
var stepNames = []string{"", "patch", "minor", "major"}

func impactRank(impact string) int {
	for i, n := range stepNames {
		if n == impact && i > 0 {
			return i
		}
	}
	return 0
}

// stepOf is how far cur steps from prev: 3 major, 2 minor, 1 patch, 0 none.
func stepOf(prev, cur version) int {
	switch {
	case cur.major > prev.major:
		return 3
	case cur.major == prev.major && cur.minor > prev.minor:
		return 2
	case cur.major == prev.major && cur.minor == prev.minor && cur.patch > prev.patch:
		return 1
	}
	return 0
}

// neededStep is the step an impact needs from prev: before 1.0.0 a major
// change needs the minor step and anything else the patch step.
func neededStep(prev version, impact int) int {
	if prev.major == 0 && impact > 0 {
		return max(impact-1, 1)
	}
	return impact
}

func nextVersion(prev version, step int) string {
	switch step {
	case 3:
		return fmt.Sprintf("%d.0.0", prev.major+1)
	case 2:
		return fmt.Sprintf("%d.%d.0", prev.major, prev.minor+1)
	}
	return fmt.Sprintf("%d.%d.%d", prev.major, prev.minor, prev.patch+1)
}

// checkReleases checks the releases together: what each released one
// includes, how far it steps from the one before, and that info.version
// agrees with them. A diagnostic about info.version goes to root.
func (rc *recordChecks) checkReleases(recs []*record, root *checker) {
	var released []*record
	for _, r := range recs {
		switch r.kind {
		case "release":
			if n := source.Child(r.root, "specificationVersion"); n != nil {
				if v, want := source.Str(n), r.str("version"); v != "" && want != "" && v != want {
					r.c.add(n, "/specificationVersion", RuleReleaseVersion, "the specification's version at a release is the release's version; set it to %s", want)
				}
			}
			if r.str("status") == "released" {
				if _, ok := parseVersion(r.str("version")); ok {
					released = append(released, r)
				}
				rc.checkContents(r)
			}
		case "change", "defect":
			rc.checkNamedRelease(r)
		}
	}
	sort.SliceStable(released, func(i, j int) bool {
		a, _ := parseVersion(released[i].str("version"))
		b, _ := parseVersion(released[j].str("version"))
		return compareVersions(a, b) < 0
	})
	for i := 1; i < len(released); i++ {
		rc.checkBump(released[i-1], released[i])
	}
	if len(released) > 0 {
		rc.checkInfoVersion(released[len(released)-1], root)
	}
}

// checkContents checks what a released release includes.
func (rc *recordChecks) checkContents(r *record) {
	ver := r.str("version")
	for i, item := range source.Items(source.Child(r.root, "includes")) {
		id := item.Value
		path := source.Pointer("includes", fmt.Sprint(i))
		var x *record
		switch {
		case rc.set["change"][id] != nil:
			x = rc.set["change"][id]
			if st := x.str("status"); st != "implemented" && st != "released" {
				r.c.add(item, path, RuleReleaseContents, "%s is %s, but a released release includes only changes that are implemented or released; take it out of includes, or carry the change out first", id, st)
				continue
			}
		case rc.set["defect"][id] != nil:
			x = rc.set["defect"][id]
			if st := x.str("status"); st != "fixed" && st != "released" {
				r.c.add(item, path, RuleReleaseContents, "%s is %s, but a released release includes only defects that are fixed or released; take it out of includes, or fix the defect first", id, st)
				continue
			}
		default:
			continue // a tracker's ID, or reported as record_ref
		}
		if named := x.str("release"); x.str("status") == "released" && named != "" && named != ver {
			r.c.add(item, path, RuleReleaseContents, "%s was released in %s, which it names, so release %s cannot include it as well; a change or defect is released once", id, named, ver)
		}
	}
}

// checkNamedRelease checks that a released change or defect names a
// released release that includes it.
func (rc *recordChecks) checkNamedRelease(x *record) {
	if x.str("status") != "released" {
		return
	}
	n := source.Child(x.root, "release")
	ver, id := source.Str(n), x.str("id")
	rel := rc.set["release"][ver]
	if ver == "" || rel == nil {
		return // the schema or record_ref reports it
	}
	if st := rel.str("status"); st != "released" {
		x.c.add(n, "/release", RuleReleaseContents, "release %s is %s, so %s cannot be released in it yet; set %s's status back, or release %s", ver, st, id, id, ver)
		return
	}
	for _, item := range source.Items(source.Child(rel.root, "includes")) {
		if item.Value == id {
			return
		}
	}
	x.c.add(n, "/release", RuleReleaseContents, "release %s does not include %s; add %s to its includes, or name the release that does", ver, id, id)
}

// checkBump checks that cur steps from prev by at least what it includes
// needs.
func (rc *recordChecks) checkBump(prevRec, cur *record) {
	prev, _ := parseVersion(prevRec.str("version"))
	now, _ := parseVersion(cur.str("version"))
	need, why, whyImpact := 0, "", ""
	for _, item := range source.Items(source.Child(cur.root, "includes")) {
		impact := ""
		if x := rc.set["change"][item.Value]; x != nil {
			impact = x.str("impact")
		} else if rc.set["defect"][item.Value] != nil {
			impact = "patch"
		}
		if n := neededStep(prev, impactRank(impact)); n > need {
			need, why, whyImpact = n, item.Value, impact
		}
	}
	if need == 0 || stepOf(prev, now) >= need {
		return
	}
	cur.c.add(source.Child(cur.root, "version"), "/version", RuleReleaseBump, "%s includes %s, whose impact is %s, so it needs a %s step from %s; release it as %s", cur.str("version"), why, whyImpact, stepNames[need], prevRec.str("version"), nextVersion(prev, need))
}

// checkInfoVersion checks info.version against the newest released
// release and the planned ones.
func (rc *recordChecks) checkInfoVersion(newest *record, root *checker) {
	n := source.Child(source.Child(rc.d.root, "info"), "version")
	v, latest := source.Str(n), newest.str("version")
	if n == nil || v == "" || v == latest {
		return
	}
	pv, ok := parseVersion(v)
	if !ok {
		return // the schema reports it
	}
	lv, _ := parseVersion(latest)
	if pv.pre != "" {
		if planned := rc.set["release"][pv.core()]; planned != nil && planned.str("status") == "planned" && compareVersions(pv, lv) > 0 {
			return
		}
		root.add(n, "/info/version", RuleReleaseVersion, "info.version is %s, a pre-release, but there is no planned release %s after %s; add records/releases/%s.yaml with status planned, or set info.version to %s", v, pv.core(), latest, pv.core(), latest)
		return
	}
	root.add(n, "/info/version", RuleReleaseVersion, "info.version is %s, but the newest released version is %s; set it to %s, or, while the next release is built, to that release's version with a pre-release tag and a planned release record", v, latest, latest)
}
