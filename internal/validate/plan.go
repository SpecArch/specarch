package validate

import (
	"fmt"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// WorkPackagePlan is what specarch plan reads of the work packages beside
// a specification (ADR-245, ADR-246): every work-package record, what
// keeps each from starting as the validator judges it, and the links of
// the reference graph the order rule reads.
type WorkPackagePlan struct {
	// Packages is every work-package record that parses, in the order the
	// records are read.
	Packages []WorkPackage
	// Satisfied is, per requirement ID, the specification's elements that
	// satisfy it, in document order.
	Satisfied map[string][]string
	// Needs is, per element, the other elements it names, without the
	// links that say what it meets.
	Needs map[string][]string
	// Holders is, per requirement ID, the packages not withdrawn that hold
	// it.
	Holders map[string][]string
}

// WorkPackage is one work-package record as plan reads it.
type WorkPackage struct {
	Path                                        string // the record's file
	ID, Title, Release, Status, AssignedTo, Why string
	Requirements, After                         []string
	// EstimatedHours and SpentHours are the effort as written, "" when
	// left out.
	EstimatedHours, SpentHours string
	// NotReady is what keeps the package from starting, whatever its
	// status says: each reason work_package_not_ready names, in its
	// order, and each element work_package_order warns of.
	NotReady []string
}

// PlanWorkPackages reads the work-package records beside a specification
// as the validator does; nil when the root file is not a specification.
func PlanWorkPackages(s *spec.Spec) *WorkPackagePlan {
	if s == nil || s.Root == nil {
		return nil
	}
	d := newDesign(s.Root)
	d.spec = s
	recs, set := readRecords(s)
	rc := &recordChecks{d: d, set: set, dir: s.RecordsDir}
	pl := rc.plan(recs)
	out := &WorkPackagePlan{Satisfied: pl.satisfied, Needs: pl.needs, Holders: pl.holders}
	for _, r := range recs {
		if r.kind != "work-package" || r.root == nil {
			continue
		}
		wp := WorkPackage{Path: r.path, ID: r.str("id"), Title: r.str("title"), Release: r.str("release"), Status: r.str("status"), AssignedTo: r.str("assignedTo"), Why: r.str("why")}
		for _, item := range source.Items(source.Child(r.root, "requirements")) {
			wp.Requirements = append(wp.Requirements, source.Str(item))
		}
		for _, item := range source.Items(source.Child(r.root, "after")) {
			wp.After = append(wp.After, source.Str(item))
		}
		effort := source.Child(r.root, "effort")
		wp.EstimatedHours = source.Str(source.Child(effort, "estimatedHours"))
		wp.SpentHours = source.Str(source.Child(effort, "spentHours"))
		if wp.Status != "withdrawn" {
			before := pl.before(wp.ID)
			var orders []misorder
			for _, req := range wp.Requirements {
				if req == "" || rc.d.requirements[req] == nil {
					continue // work_package_order skips it too: a requirement of a requirement set, or none
				}
				orders = append(orders, pl.misorders(req, before)...)
			}
			wp.NotReady = rc.notReady(r, pl, false)
			for _, m := range orders {
				wp.NotReady = append(wp.NotReady, fmt.Sprintf("requirement %s is met by %s, which needs %s, and only %s %s it, outside its after chain (work_package_order)", m.requirement, m.element, m.need, joinAnd(m.builtIn), pick(len(m.builtIn) == 1, "builds", "build")))
			}
		}
		out.Packages = append(out.Packages, wp)
	}
	return out
}
