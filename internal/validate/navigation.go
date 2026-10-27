package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// The navigation a person goes through (ADR-197): the page each role is
// shown first, and the gaps a walkthrough of the screens asks about.

// opens reports whether a holder of the permissions held may open what
// needs perm: public and signedIn are held by every signed-in caller.
func opens(held map[string]bool, perm string) bool {
	return perm == permissionPublic || perm == permissionSignedIn || held[perm]
}

// navigateCallerHome is the reserved word a task page's answer navigates to
// for the caller's home: the home of the first of the session's roles that
// has one. A page's name is kebab-case, so no page is named so.
const navigateCallerHome = "callerHome"

// checkCallerHomeEvent checks a page event that navigates to callerHome:
// the event is a task page's answer, after which the session may hold
// other roles, it gives no with, since a home takes no route parameters,
// and some role names a home.
func (c *checker) checkCallerHomeEvent(d *design, ev, nav *yaml.Node, base []string, answer bool) {
	at := source.Pointer(append(base, "navigate")...)
	if !answer {
		c.add(nav, at, RuleFlow, "callerHome leads to the home of the first of the caller's roles that has one, which a task page's answer may do, as a sign-in does; lead this event to a page")
		return
	}
	if source.Child(ev, "with") != nil {
		c.add(source.Key(ev, "with"), source.Pointer(append(base, "with")...), RuleFlow, "the event navigates to callerHome, and a home takes no route parameters; remove with")
	}
	for _, r := range source.Pairs(source.Child(d.root, "roles")) {
		if source.Child(r.Value, "home") != nil {
			return
		}
	}
	c.add(nav, at, RuleFlow, "callerHome leads to the home of the first of the caller's roles that has one, and no role names a home; give a role its home under roles, or navigate to a page")
}

// checkRoleHomes checks each role's home: a page of the specification,
// opened with no route parameters, which the role may open.
func (c *checker) checkRoleHomes(d *design) {
	for _, r := range source.Pairs(source.Child(d.root, "roles")) {
		home := source.Child(r.Value, "home")
		if home == nil {
			continue
		}
		at := source.Pointer("roles", r.Key.Value, "home")
		pg := d.pages[home.Value]
		if pg == nil {
			c.add(home, at, RuleRoleHome, "%s is not a page of the specification%s", home.Value, suggest(home.Value, d.pages))
			continue
		}
		if params := routeParams(source.Str(source.Child(pg, "route"))); len(params) > 0 {
			c.add(home, at, RuleRoleHome, "%s takes the route parameters %s, and a home page is opened with none; name a page whose route takes none", home.Value, strings.Join(params, ", "))
		}
		if perm := source.Str(source.Child(pg, "permission")); perm != "" && !opens(d.roleHolds(r.Value), perm) {
			c.add(home, at, RuleRoleHome, "%s cannot open its home %s, which needs %s; grant it to the role, or name a page the role can open", r.Key.Value, home.Value, perm)
		}
	}
}

// menuEntry is a leaf of the menus: the entry and the page it opens.
type menuEntry struct {
	node *yaml.Node
	path []string
	page string
}

// menuEntries is every leaf of the menus, in the order written.
func (d *design) menuEntries() []menuEntry {
	var out []menuEntry
	var visit func(items *yaml.Node, path []string)
	visit = func(items *yaml.Node, path []string) {
		for _, m := range source.Pairs(items) {
			p := append(append([]string{}, path...), m.Key.Value)
			if page := source.Child(m.Value, "page"); page != nil {
				out = append(out, menuEntry{node: m.Key, path: p, page: page.Value})
			}
			visit(source.Child(m.Value, "items"), append(p, "items"))
		}
	}
	visit(source.Child(d.root, "menus"), []string{"menus"})
	return out
}

// pageTargets is every page a page's actions, events and links lead to.
func pageTargets(pg *yaml.Node) []string {
	var out []string
	add := func(n *yaml.Node) {
		if t := source.Str(source.Child(n, "navigate")); t != "" {
			out = append(out, t)
		}
	}
	for _, a := range source.Items(source.Child(pg, "actions")) {
		if source.Str(source.Child(a, "kind")) == "navigate" {
			out = append(out, source.Str(source.Child(a, "target")))
		}
		add(source.Child(a, "then"))
	}
	add(source.Child(pg, "onSelect"))
	if source.Str(source.Child(pg, "kind")) == "task" {
		for _, kv := range source.Pairs(source.Child(pg, "onSubmitted")) {
			add(kv.Value)
		}
	} else {
		add(source.Child(pg, "onSubmitted"))
	}
	for _, b := range source.Items(source.Child(pg, "blocks")) {
		if t := source.Str(source.Child(source.Child(b, "link"), "page")); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// personless is the roles no person signs in as: those a job acts as or a
// service client holds, which open no page.
func (d *design) personless() map[string]bool {
	out := map[string]bool{}
	for _, j := range source.Pairs(source.Child(d.root, "jobs")) {
		out[source.Str(source.Child(j.Value, "role"))] = true
	}
	for _, cl := range d.clients() {
		if source.Str(source.Child(cl.Value, "kind")) == "service" {
			for _, r := range source.Items(source.Child(cl.Value, "roles")) {
				out[r.Value] = true
			}
		}
	}
	return out
}

// checkNavigation reports, as warnings, the gaps a walkthrough of the
// screens asks about: a page nothing leads to, a role that can open no menu
// entry, a menu entry no role can open, an operation action whose problem
// types a page without states shows nothing for, and a form that says
// nothing of where it leads once sent. The ones about menus are checked
// once the specification has menus.
func (c *checker) checkNavigation(d *design) {
	pages := source.Pairs(source.Child(d.root, "pages"))
	entries := d.menuEntries()
	menus := source.Child(d.root, "menus") != nil
	led := map[string]bool{}
	for _, e := range entries {
		led[e.page] = true
	}
	for _, p := range pages {
		for _, t := range pageTargets(p.Value) {
			if t != p.Key.Value {
				led[t] = true
			}
		}
	}
	for _, r := range source.Pairs(source.Child(d.root, "roles")) {
		led[source.Str(source.Child(r.Value, "home"))] = true
	}
	for _, p := range pages {
		name, pg := p.Key.Value, p.Value
		at := source.Pointer("pages", name)
		perm := source.Str(source.Child(pg, "permission"))
		if menus && !led[name] && perm != permissionPublic {
			c.warn(p.Key, at, RuleNavigation, "nothing leads to %s: no menu entry, action, event or link opens it, and it is no role's home; add a way to it, make it a role's home, or remove it", name)
		}
		kind := source.Str(source.Child(pg, "kind"))
		if kind == "form" && source.Child(pg, "onSubmitted") == nil {
			c.warn(p.Key, at, RuleNavigation, "%s is a form with no onSubmitted, so nothing says where the person goes or what they are told once it is sent; add onSubmitted", name)
		}
		if source.Child(pg, "states") != nil {
			continue // checkPageStates reports a problem type without a message
		}
		for i, a := range source.Items(source.Child(pg, "actions")) {
			if source.Str(source.Child(a, "kind")) != "operation" {
				continue
			}
			o, ok := d.operations[source.Str(source.Child(a, "target"))]
			if !ok || o.node == nil {
				continue
			}
			var problems []string
			for _, r := range source.Pairs(source.Child(o.node, "responses")) {
				if pr := source.Str(source.Child(r.Value, "problem")); pr != "" && !contains(problems, pr) {
					problems = append(problems, pr)
				}
			}
			if len(problems) > 0 {
				c.warn(a, source.Pointer("pages", name, "actions", fmt.Sprint(i)), RuleNavigation, "the action %s runs %s, which can answer %s, and %s has no states to say what the person is shown then; add states.failed with a message for each, or a default", source.Str(source.Child(a, "label")), o.id, joinAnd(problems), name)
			}
		}
	}
	if !menus {
		return
	}
	personless := d.personless()
	for _, r := range source.Pairs(source.Child(d.root, "roles")) {
		if personless[r.Key.Value] {
			continue
		}
		held := d.roleHolds(r.Value)
		reaches := false
		for _, e := range entries {
			if pg := d.pages[e.page]; pg != nil && opens(held, source.Str(source.Child(pg, "permission"))) {
				reaches = true
				break
			}
		}
		if !reaches {
			c.warn(r.Key, source.Pointer("roles", r.Key.Value), RuleNavigation, "%s can open no menu entry, so a person holding it finds no page; grant it the permission of an entry's page, or add an entry it can open", r.Key.Value)
		}
	}
	if source.Child(d.root, "roles") == nil {
		return // the roles are another module's, which permission_ungranted reads
	}
	for _, e := range entries {
		pg := d.pages[e.page]
		if pg == nil {
			continue // checkMenus reports it
		}
		perm := source.Str(source.Child(pg, "permission"))
		reaches := false
		for _, r := range source.Pairs(source.Child(d.root, "roles")) {
			if opens(d.roleHolds(r.Value), perm) {
				reaches = true
				break
			}
		}
		switch {
		case perm == permissionNobody:
			c.warn(e.node, source.Pointer(e.path...), RuleNavigation, "%s is closed to every caller with nobody, so the entry is shown to nobody; take the entry out", e.page)
		case !reaches && perm != permissionPublic && perm != permissionSignedIn:
			c.warn(e.node, source.Pointer(e.path...), RuleNavigation, "no role can open %s, which needs %s, so the entry is shown to nobody; grant %s to a role, or take the entry out", e.page, perm, perm)
		}
	}
}
