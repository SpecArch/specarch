package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The permission table a project's permission printer writes, in the
// format tools/permissions/dump-permissions.sh wraps: the version of the
// format, the path and the commit the check was built from, every grant
// the check reads, and every check a setting switches off (ADR-050).
type permissionTable struct {
	PermissionTable int         `json:"permissionTable"`
	Path            string      `json:"path"`
	Commit          string      `json:"commit"`
	Grants          []dumpGrant `json:"grants"`
	Gates           []dumpGate  `json:"gates"`
}

type dumpGrant struct {
	Role       string `json:"role"`
	Permission string `json:"permission"`
}

type dumpGate struct {
	Check   string `json:"check"`
	Setting string `json:"setting"`
}

var roleWord = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// Permissions reads a permission table into roles and the permissions
// each grants, and asks about every check the table says a setting
// switches off.
func Permissions(dumpPath, out, key string) (*Result, error) {
	data, err := os.ReadFile(dumpPath)
	if err != nil {
		return nil, err
	}
	var t permissionTable
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return nil, refuse("%s is not a permission table: %v", dumpPath, err)
	}
	if t.PermissionTable != 1 {
		return nil, refuse("%s is not a permission table in the format tools/permissions/dump-permissions.sh writes: its permissionTable is %d, not 1", dumpPath, t.PermissionTable)
	}
	if t.Grants == nil || t.Gates == nil {
		return nil, refuse("%s is not a permission table: it needs both grants and gates, each a list, empty when there are none", dumpPath)
	}
	r, dumpName, err := openDump(dumpPath, t.Path, t.Commit, "tools/permissions/dump-permissions.sh")
	if err != nil {
		return nil, err
	}
	res := &Result{Tree: newTree()}
	res.say("commit %s: the last change to %s, as the permission table %s names it", r.Commit, t.Path, dumpName)
	pr := &permissionReader{t: &t, key: key, res: res, questions: &yaml.Node{Kind: yaml.MappingNode}}
	if err := pr.read(dumpName); err != nil {
		return nil, err
	}
	src, err := codeSource(r, out, []*yaml.Node{flow(mapping("clause", t.Path, "title", "The grants the permission check built from here reads"))})
	if err != nil {
		return nil, err
	}
	set(src, "reading", "printed")
	var stages []string
	if pr.roles != nil {
		stages = []string{"design"}
		res.Tree.put("design/roles.yaml", mapping("permissions", pr.permissions, "roles", pr.roles))
	}
	if len(pr.questions.Content) > 0 {
		stages = []string{"requirements", "design"}
		res.Tree.put("requirements/stakeholders.yaml", mapping("stakeholders", ownerStakeholder()))
		res.Tree.put("design/questions.yaml", mapping("questions", pr.questions))
	}
	description := fmt.Sprintf("The roles and the permissions each grants, as the permission check built from %s reads them, from the permission table %s, made at commit %s. Every role and permission cites the table; what the table does not say is a question, and so is what the meta-model cannot hold.\n", t.Path, dumpName, r.Commit)
	res.Tree.put("specarch.yaml", rootFile("Permissions of "+t.Path, description, stages, mapping(key, src)))
	return res, nil
}

type permissionReader struct {
	t           *permissionTable
	key         string
	res         *Result
	roles       *yaml.Node
	permissions *yaml.Node
	questions   *yaml.Node
	nextID      int
	notHeld     []notHeld
	mappedFrom  map[string]string // a permission the rule of ADR-076 mapped -> the privilege named
}

// gap records what the meta-model cannot hold of a grant of the role, as
// a question on the roles.
func (pr *permissionReader) gap(role string, format string, args ...any) {
	pr.notHeld = append(pr.notHeld, notHeld{text: fmt.Sprintf(format, args...), clause: pr.t.Path + " role " + role, blocks: []string{"roles"}})
}

func (pr *permissionReader) question(text string, blocks []string, why string, cites ...*yaml.Node) {
	pr.nextID++
	q := mapping(
		"question", text,
		"kind", "decision",
		"priority", "must",
		"blocks", blocks,
		"decidedBy", owner,
		"why", why,
	)
	if len(cites) > 0 {
		set(q, "cites", cites)
	}
	set(pr.questions, fmt.Sprintf("Q-%d", pr.nextID), q)
}

// gateQuestion is the must question on a check that lets every request
// through while a setting is empty, the same whichever reader finds it, so
// that the merge joins them by the check's name.
func gateQuestion(check, setting string) (question, why string) {
	return fmt.Sprintf("The check %s runs only when the setting %s is present. Is an empty %s ever meant to let every request through, or should the check refuse every request until it is set?", check, setting, setting),
		"A check switched off by an empty setting fails open without an error, so no role's grants hold while it is off."
}

// boolGateQuestion is gateQuestion for a check a boolean setting switches
// off while it is false.
func boolGateQuestion(check, setting string) (question, why string) {
	return fmt.Sprintf("The check %s runs only when the setting %s is true. Is a false %s ever meant to let every request through, or should the check refuse every request whatever it says?", check, setting, setting),
		"A check switched off by a setting fails open without an error, so no role's grants hold while it is off."
}

// read checks every grant and gate, then writes the roles and the
// permissions in the order of their names, so the printer's order does
// not show.
func (pr *permissionReader) read(dumpName string) error {
	seen := map[dumpGrant]bool{}
	var names []string
	for i, g := range pr.t.Grants {
		where := fmt.Sprintf("%s, grant %d", dumpName, i+1)
		if g.Role == "" || g.Permission == "" {
			return refuse("%s names no role or no permission; a grant is one role and one permission", where)
		}
		if seen[g] {
			return refuse("%s: %s granting %s is listed twice", where, g.Role, g.Permission)
		}
		seen[g] = true
		if roleWord.MatchString(g.Role) && g.Permission != "public" {
			names = append(names, g.Permission)
		}
	}
	// dxlib_module's privilege names map to permissions by the rule of
	// ADR-076, as extract openapi and extract go map them.
	privileges := newPrivilegeNames(names)
	byRole := map[string][]string{}
	type asked struct{ role, question, why string }
	var problems []asked
	pr.mappedFrom = map[string]string{}
	for _, g := range pr.t.Grants {
		switch {
		case !roleWord.MatchString(g.Role):
			pr.gap(g.Role, "grant of %s to %q: the role's name is not kebab-case, which a role name is; left out", g.Permission, g.Role)
		case g.Permission == "public":
			pr.gap(g.Role, "grant of public to %s: public is open to everyone and granted by no role; left out", g.Role)
		default:
			if q, why := privileges.grantProblem(g.Role, g.Permission); q != "" {
				problems = append(problems, asked{g.Role, q, why})
				if byRole[g.Role] == nil {
					byRole[g.Role] = []string{}
				}
				continue
			}
			p, mapped, _ := privileges.read(g.Permission)
			if mapped {
				pr.mappedFrom[p] = g.Permission
			}
			byRole[g.Role] = append(byRole[g.Role], p)
		}
	}
	unknown := map[string]bool{}
	for _, a := range problems {
		unknown[a.role] = true
	}
	for _, g := range pr.t.Grants {
		if roleWord.MatchString(g.Role) && byRole[g.Role] == nil {
			pr.gap(g.Role, "role %s: it grants nothing the meta-model holds, and a role that grants nothing is left out", g.Role)
			byRole[g.Role] = []string{}
		}
	}
	gates := map[dumpGate]bool{}
	for i, g := range pr.t.Gates {
		where := fmt.Sprintf("%s, gate %d", dumpName, i+1)
		if g.Check == "" || g.Setting == "" {
			return refuse("%s names no check or no setting; a gate is the check and the setting it needs", where)
		}
		if gates[g] {
			return refuse("%s: the check %s needing %s is listed twice", where, g.Check, g.Setting)
		}
		gates[g] = true
	}
	pr.res.say("counted %s and %s: every entry of the permission table's lists, which hold every grant the check reads and every check the printer declares a setting switches off", plural(len(pr.t.Grants), "grant"), plural(len(pr.t.Gates), "gate"))

	var roleNames []string
	usedBy := map[string][]string{}
	for name, perms := range byRole {
		if len(perms) > 0 || unknown[name] {
			roleNames = append(roleNames, name)
		}
		for _, p := range perms {
			usedBy[p] = append(usedBy[p], name)
		}
	}
	sort.Strings(roleNames)
	if len(roleNames) > 0 {
		pr.roles = &yaml.Node{Kind: yaml.MappingNode}
		var blocks []string
		for _, name := range roleNames {
			perms := byRole[name]
			sort.Strings(perms)
			var granted []string
			for _, g := range pr.t.Grants {
				if g.Role == name {
					granted = append(granted, g.Permission)
				}
			}
			sort.Strings(granted)
			role := mapping()
			if len(perms) > 0 {
				set(role, "permissions", perms)
			}
			set(role, "origin", "stated")
			set(role, "cites", []*yaml.Node{citation(pr.key, pr.t.Path+" role "+name, fmt.Sprintf("%s grants %s.", name, joinAnd(granted)))})
			set(pr.roles, name, role)
			blocks = append(blocks, "#/roles/"+escapeToken(name)+"/description")
		}
		pr.question(
			fmt.Sprintf("What is each role for, and who holds it: %s?", strings.Join(roleNames, ", ")),
			blocks,
			"A permission table names a role and what it grants, not who holds it.",
		)
		var names []string
		for n := range usedBy {
			names = append(names, n)
		}
		sort.Strings(names)
		if len(names) > 0 {
			pr.permissions = &yaml.Node{Kind: yaml.MappingNode}
		}
		blocks = nil
		for _, n := range names {
			roles := usedBy[n]
			sort.Strings(roles)
			named := n
			if pr.mappedFrom[n] != "" {
				named = pr.mappedFrom[n]
			}
			cites := []*yaml.Node{citation(pr.key, pr.t.Path+" role "+roles[0], fmt.Sprintf("%s %s %s.", joinAnd(roles), grantOrGrants(len(roles)), named))}
			if pr.mappedFrom[n] != "" {
				set(pr.permissions, n, mapping("origin", "inferred", "why", privilegeWhy(named, n), "cites", cites))
			} else {
				set(pr.permissions, n, mapping("origin", "stated", "cites", cites))
			}
			blocks = append(blocks, "#/permissions/"+escapeToken(n)+"/description")
		}
		if len(names) > 0 {
			pr.question(
				fmt.Sprintf("What does each permission allow: %s?", strings.Join(names, ", ")),
				blocks,
				"A permission table names the permissions a role grants and not what they are for.",
			)
		}
	}
	sort.SliceStable(problems, func(i, j int) bool { return problems[i].role < problems[j].role })
	for _, a := range problems {
		pr.question(a.question, []string{"#/roles/" + escapeToken(a.role) + "/permissions"}, a.why)
	}
	sorted := append([]dumpGate(nil), pr.t.Gates...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Check != sorted[j].Check {
			return sorted[i].Check < sorted[j].Check
		}
		return sorted[i].Setting < sorted[j].Setting
	})
	for _, g := range sorted {
		pr.res.say("gate: the check %s runs only when the setting %s is present, and lets every request through while it is empty", g.Check, g.Setting)
		q, why := gateQuestion(g.Check, g.Setting)
		pr.question(q, []string{"roles"}, why, citation(pr.key, pr.t.Path+" gate "+g.Check, fmt.Sprintf("The permission table declares that %s runs only when %s is present.", g.Check, g.Setting)))
	}
	permissions := 0
	if pr.permissions != nil {
		permissions = len(pr.permissions.Content) / 2
	}
	pr.res.say("wrote %s, %s and %s: one role per name the table grants something the meta-model holds or asks about, one permission per name a role grants, one question per thing the permission table does not say and per gate, and one per thing the meta-model cannot hold", plural(len(roleNames), "role"), plural(permissions, "permission"), plural(pr.nextID+couldCount(oneQuestions(pr.questions), pr.notHeld), "question"))
	askNotHeld(pr.res, oneQuestions(pr.questions), &pr.nextID, pr.key, pr.notHeld)
	return nil
}

func grantOrGrants(n int) string {
	if n == 1 {
		return "grants"
	}
	return "grant"
}
