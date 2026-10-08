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
	description := fmt.Sprintf("The roles and the permissions each grants, as the permission check built from %s reads them, from the permission table %s, made at commit %s. Every role and permission cites the table; what the table does not say is a question.\n", t.Path, dumpName, r.Commit)
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
	notHeld     []string
}

func (pr *permissionReader) gap(format string, args ...any) {
	pr.notHeld = append(pr.notHeld, "not held: "+fmt.Sprintf(format, args...))
}

func (pr *permissionReader) question(text string, blocks []string, why string) {
	pr.nextID++
	set(pr.questions, fmt.Sprintf("Q-%d", pr.nextID), mapping(
		"question", text,
		"kind", "decision",
		"priority", "must",
		"blocks", blocks,
		"decidedBy", owner,
		"why", why,
	))
}

// read checks every grant and gate, then writes the roles and the
// permissions in the order of their names, so the printer's order does
// not show.
func (pr *permissionReader) read(dumpName string) error {
	seen := map[dumpGrant]bool{}
	byRole := map[string][]string{}
	for i, g := range pr.t.Grants {
		where := fmt.Sprintf("%s, grant %d", dumpName, i+1)
		if g.Role == "" || g.Permission == "" {
			return refuse("%s names no role or no permission; a grant is one role and one permission", where)
		}
		if seen[g] {
			return refuse("%s: %s granting %s is listed twice", where, g.Role, g.Permission)
		}
		seen[g] = true
		switch {
		case !roleWord.MatchString(g.Role):
			pr.gap("grant of %s to %q: the role's name is not kebab-case, which a role name is; left out", g.Permission, g.Role)
		case !permissionWord.MatchString(g.Permission):
			pr.gap("grant of %q to %s: the permission's name is not lower-case words joined by dots; left out", g.Permission, g.Role)
		case g.Permission == "public":
			pr.gap("grant of public to %s: public is open to everyone and granted by no role; left out", g.Role)
		default:
			byRole[g.Role] = append(byRole[g.Role], g.Permission)
		}
	}
	for _, g := range pr.t.Grants {
		if roleWord.MatchString(g.Role) && byRole[g.Role] == nil {
			pr.gap("role %s: it grants nothing the meta-model holds, and a role that grants nothing is left out", g.Role)
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
		if len(perms) > 0 {
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
			set(pr.roles, name, mapping(
				"permissions", perms,
				"origin", "stated",
				"cites", []*yaml.Node{citation(pr.key, pr.t.Path+" role "+name, fmt.Sprintf("%s grants %s.", name, joinAnd(perms)))},
			))
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
		pr.permissions = &yaml.Node{Kind: yaml.MappingNode}
		blocks = nil
		for _, n := range names {
			roles := usedBy[n]
			sort.Strings(roles)
			set(pr.permissions, n, mapping(
				"origin", "stated",
				"cites", []*yaml.Node{citation(pr.key, pr.t.Path+" role "+roles[0], fmt.Sprintf("%s %s %s.", joinAnd(roles), grantOrGrants(len(roles)), n))},
			))
			blocks = append(blocks, "#/permissions/"+escapeToken(n)+"/description")
		}
		pr.question(
			fmt.Sprintf("What does each permission allow: %s?", strings.Join(names, ", ")),
			blocks,
			"A permission table names the permissions a role grants and not what they are for.",
		)
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
		pr.question(
			fmt.Sprintf("The check %s runs only when the setting %s is present. Is an empty %s ever meant to let every request through, or should the check refuse every request until it is set?", g.Check, g.Setting, g.Setting),
			[]string{"roles"},
			"A check switched off by an empty setting fails open without an error, so no role's grants hold while it is off.",
		)
	}
	permissions := 0
	if pr.permissions != nil {
		permissions = len(pr.permissions.Content) / 2
	}
	pr.res.say("wrote %s, %s and %s: one role per name the table grants something the meta-model holds, one permission per name a role grants, and one question per thing the permission table does not say and per gate", plural(len(roleNames), "role"), plural(permissions, "permission"), plural(pr.nextID, "question"))
	pr.res.Lines = append(pr.res.Lines, pr.notHeld...)
	return nil
}

func grantOrGrants(n int) string {
	if n == 1 {
		return "grants"
	}
	return "grant"
}
