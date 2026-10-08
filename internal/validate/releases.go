package validate

import (
	"fmt"
	"sort"

	"github.com/SpecArch/specarch/internal/semver"
	"github.com/SpecArch/specarch/internal/source"
)

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
				if _, ok := semver.Parse(r.str("version")); ok {
					released = append(released, r)
				}
				rc.checkContents(r)
			}
		case "change", "defect":
			rc.checkNamedRelease(r)
		}
	}
	sort.SliceStable(released, func(i, j int) bool {
		a, _ := semver.Parse(released[i].str("version"))
		b, _ := semver.Parse(released[j].str("version"))
		return semver.Compare(a, b) < 0
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
	prev, _ := semver.Parse(prevRec.str("version"))
	now, _ := semver.Parse(cur.str("version"))
	need, why, whyImpact := 0, "", ""
	for _, item := range source.Items(source.Child(cur.root, "includes")) {
		impact := ""
		if x := rc.set["change"][item.Value]; x != nil {
			impact = x.str("impact")
		} else if rc.set["defect"][item.Value] != nil {
			impact = "patch"
		}
		if n := semver.Needed(prev, semver.Rank(impact)); n > need {
			need, why, whyImpact = n, item.Value, impact
		}
	}
	if need == 0 || semver.Step(prev, now) >= need {
		return
	}
	cur.c.add(source.Child(cur.root, "version"), "/version", RuleReleaseBump, "%s includes %s, whose impact is %s, so it needs a %s step from %s; release it as %s", cur.str("version"), why, whyImpact, semver.StepNames[need], prevRec.str("version"), semver.Next(prev, need))
}

// checkInfoVersion checks info.version against the newest released
// release and the planned ones.
func (rc *recordChecks) checkInfoVersion(newest *record, root *checker) {
	n := source.Child(source.Child(rc.d.root, "info"), "version")
	v, latest := source.Str(n), newest.str("version")
	if n == nil || v == "" || v == latest {
		return
	}
	pv, ok := semver.Parse(v)
	if !ok {
		return // the schema reports it
	}
	lv, _ := semver.Parse(latest)
	if pv.Pre != "" {
		if planned := rc.set["release"][pv.Core()]; planned != nil && planned.str("status") == "planned" && semver.Compare(pv, lv) > 0 {
			return
		}
		root.add(n, "/info/version", RuleReleaseVersion, "info.version is %s, a pre-release, but there is no planned release %s after %s; add records/releases/%s.yaml with status planned, or set info.version to %s", v, pv.Core(), latest, pv.Core(), latest)
		return
	}
	root.add(n, "/info/version", RuleReleaseVersion, "info.version is %s, but the newest released version is %s; set it to %s, or, while the next release is built, to that release's version with a pre-release tag and a planned release record", v, latest, latest)
}
