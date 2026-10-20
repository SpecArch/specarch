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

	"github.com/SpecArch/specarch/internal/source"
)

// The permission table a project's permission printer writes, in the
// format tools/permissions/dump-permissions.sh wraps: the version of the
// format, the path and the commit the check was built from, every grant
// the check reads, every check a setting switches off (ADR-050) and every
// check skipped for one kind of caller (ADR-114); and, for a check that
// reads a policy of path patterns, its rules (ADR-100).
type permissionTable struct {
	PermissionTable int         `json:"permissionTable"`
	Path            string      `json:"path"`
	Commit          string      `json:"commit"`
	Repository      string      `json:"repository"`
	Grants          []dumpGrant `json:"grants"`
	Gates           []dumpGate  `json:"gates"`
	Rules           []dumpRule  `json:"rules"`
}

type dumpGrant struct {
	Role       string `json:"role"`
	Permission string `json:"permission"`
	// Client is the row's subject when it is a client and not a role, as
	// a policy library's rows name a service directly; Through is the one
	// client the grant holds through, the application the person signed
	// in through (ADR-106, ADR-110).
	Client  string `json:"client"`
	Through string `json:"through"`
}

type dumpGate struct {
	Check       string `json:"check"`
	Setting     string `json:"setting"`
	BypassedFor string `json:"bypassedFor"`
}

// callerKinds are the kinds of caller a check may be skipped for, in
// meta-model 0.3's word: a client calls on its own account, not a
// person's.
var callerKinds = map[string]bool{"client": true}

var roleWord = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// Permissions reads a permission table into roles and the permissions
// each grants, and asks about every check the table says a setting
// switches off or one kind of caller skips. Given the route table too (routesPath), it writes the
// routes' operations as extract router does, each route that checks no
// permission taking the one the table's rules give it (ADR-100).
func Permissions(dumpPath, routesPath, out, key string) (*Result, error) {
	t, r, dumpName, err := openPermissionTable(dumpPath)
	if err != nil {
		return nil, err
	}
	res := &Result{Tree: newTree()}
	res.say("commit %s: the last change to %s, as the permission table %s names it", r.Commit, t.Path, dumpName)
	pr := &permissionReader{t: t, key: key, res: res, questions: &yaml.Node{Kind: yaml.MappingNode}, bypassBlocks: map[string]*yaml.Node{}}
	if err := pr.read(dumpName); err != nil {
		return nil, err
	}
	clauses := []*yaml.Node{flow(mapping("clause", t.Path, "title", "The grants the permission check built from here reads"))}
	var rd *routeReader
	var routesName string
	if routesPath != "" {
		rt, rr, name, err := openRouteTable(routesPath)
		routesName = name
		if err != nil {
			return nil, err
		}
		if rr.Repository.Root != r.Repository.Root {
			return nil, refuse("the route table %s reads the repository at %s, and the permission table %s the one at %s; give two tables of one repository", routesName, rr.Repository.Root, dumpName, r.Repository.Root)
		}
		edition, err := laterCommit(r.Repository.Root, r.Commit, rr.Commit)
		if err != nil {
			return nil, refuse("the permission table %s was made at commit %s and the route table %s at commit %s, and neither is an ancestor of the other, so no one commit holds both; make them again from one checkout", dumpName, r.Commit, routesName, rr.Commit)
		}
		r.Commit = edition
		res.say("commit %s: the last change to %s, as the route table %s names it; the tree is read at commit %s", rr.Commit, rt.Path, routesName, edition)
		rd = &routeReader{t: rt, key: key, res: res, questions: pr.questions, nextID: pr.nextID, ruled: pr.routePermission}
		if err := rd.read(routesName); err != nil {
			return nil, err
		}
		pr.nextID = rd.nextID
		pr.deadRules()
		pr.guarded(rd)
		clauses = append(clauses, flow(mapping("clause", rt.Path, "title", "The routes the router built from here registers")))
	}
	src, err := codeSource(r, out, clauses)
	if err != nil {
		return nil, err
	}
	set(src, "reading", "printed")
	var stages []string
	if pr.roles != nil {
		stages = []string{"design"}
		if pr.clients != nil {
			res.Tree.put("design/roles.yaml", mapping("permissions", pr.permissions, "roles", pr.roles, "clients", pr.clients))
		} else {
			res.Tree.put("design/roles.yaml", mapping("permissions", pr.permissions, "roles", pr.roles))
		}
	}
	if rd != nil && len(rd.routes) > 0 {
		stages = []string{"design"}
		design := mapping()
		// A permission a route names that a role grants too is written
		// once, with the roles.
		if rd.permissions != nil && pr.permissions != nil {
			for _, p := range source.Pairs(pr.permissions) {
				deleteKey(rd.permissions, p.Key.Value)
			}
		}
		if rd.permissions != nil && len(rd.permissions.Content) > 0 {
			set(design, "permissions", rd.permissions)
		}
		set(design, "paths", rd.paths)
		res.Tree.put("design/paths.yaml", design)
	}
	if len(pr.questions.Content) > 0 {
		stages = []string{"requirements", "design"}
		res.Tree.put("requirements/stakeholders.yaml", mapping("stakeholders", ownerStakeholder()))
		res.Tree.put("design/questions.yaml", mapping("questions", pr.questions))
	}
	description := fmt.Sprintf("The roles and the permissions each grants, as the permission check built from %s reads them, from the permission table %s, made at commit %s. Every role and permission cites the table; what the table does not say is a question, and so is what the meta-model cannot hold.\n", t.Path, dumpName, t.Commit)
	if rd != nil {
		description = fmt.Sprintf("The roles and the permissions each grants, as the permission check built from %s reads them, from the permission table %s, made at commit %s; and the routes the router built from %s registers, from the route table %s, made at commit %s, each route that checks no permission taking the one the table's rules give it. Every role, permission and operation cites its table; what the tables do not say is a question, and so is what the meta-model cannot hold.\n", t.Path, dumpName, t.Commit, rd.t.Path, routesName, rd.t.Commit)
	}
	root := rootFile("Permissions of "+t.Path, description, stages, mapping(key, src))
	if rule := pr.roleNames.rule(); rule != "" && pr.roles != nil {
		set(child(root, "info"), "roleNames", rule)
		res.say("role names: the table writes its roles in upper snake case, so each is read by its kebab-case name and the tree says info.roleNames %s", rule)
	}
	res.Tree.put("specarch.yaml", root)
	return res, nil
}

// openPermissionTable reads a permission table and checks it against the
// commit it names.
func openPermissionTable(dumpPath string) (*permissionTable, *Read, string, error) {
	data, err := os.ReadFile(dumpPath)
	if err != nil {
		return nil, nil, "", err
	}
	var t permissionTable
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return nil, nil, "", refuse("%s is not a permission table: %v", dumpPath, err)
	}
	if t.PermissionTable != 1 {
		return nil, nil, "", refuse("%s is not a permission table in the format tools/permissions/dump-permissions.sh writes: its permissionTable is %d, not 1", dumpPath, t.PermissionTable)
	}
	if t.Grants == nil || t.Gates == nil {
		return nil, nil, "", refuse("%s is not a permission table: it needs both grants and gates, each a list, empty when there are none", dumpPath)
	}
	r, dumpName, err := openDump(dumpPath, t.Repository, t.Path, t.Commit, "tools/permissions/dump-permissions.sh")
	if err != nil {
		return nil, nil, "", err
	}
	return &t, r, dumpName, nil
}

type permissionReader struct {
	t           *permissionTable
	key         string
	res         *Result
	roles       *yaml.Node
	permissions *yaml.Node
	clients     *yaml.Node // the clients the grants name, as subject or through
	questions   *yaml.Node
	nextID      int
	notHeld     []notHeld
	mappedFrom  map[string]string // a permission the rule of ADR-076 mapped -> the privilege named
	roleNames   roleNames         // how the table writes its roles (ADR-099)
	wireOf      map[string]string // a role's name -> the name the table writes
	rulePerms   []*rulePermission // the permissions the rules give, by name
	// bypassBlocks are the blocks of each bypassed gate's question, by
	// the check's name, given the guarded operations once the routes are
	// read.
	bypassBlocks map[string]*yaml.Node
	compared     []comparedGrant
}

// comparedGrant is one grant of the table as specarch check compares it.
type comparedGrant struct {
	role, permission string
	index            int // in the table's grants
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

// bypassQuestion is the must question on a check skipped for one kind of
// caller (ADR-114), the same whichever reader finds it, so that the merge
// joins them by the check's name.
func bypassQuestion(check, kind string) (question, why string) {
	return fmt.Sprintf("The check %s is skipped for every %s: a %s passes it whatever its roles grant. Is every %s meant to reach each operation the check guards, or which ones are, and should the check hold the others to their roles' grants?", check, kind, kind, kind),
		fmt.Sprintf("A check skipped for one kind of caller fails open for that kind without an error, so no role's grants hold for a %s.", kind)
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
	var roles []string
	for i, g := range pr.t.Grants {
		where := fmt.Sprintf("%s, grant %d", dumpName, i+1)
		switch {
		case g.Role != "" && g.Client != "":
			return refuse("%s names both the role %s and the client %s; a grant is one role or one client, and one permission", where, g.Role, g.Client)
		case g.Role == "" && g.Client == "" || g.Permission == "":
			return refuse("%s names no role or no permission; a grant is one role and one permission", where)
		case g.Client != "" && !roleWord.MatchString(g.Client):
			return refuse("%s: the client %q is not kebab-case; a client is named by its software_id in kebab-case", where, g.Client)
		case g.Through != "" && !roleWord.MatchString(g.Through):
			return refuse("%s: the client %q it holds through is not kebab-case; a client is named by its software_id in kebab-case", where, g.Through)
		}
		if seen[g] {
			if g.Client != "" {
				return refuse("%s: %s granting %s is listed twice", where, g.Client, g.Permission)
			}
			return refuse("%s: %s granting %s is listed twice", where, g.Role, g.Permission)
		}
		seen[g] = true
		if g.Role != "" {
			roles = append(roles, g.Role)
		}
	}
	for _, r := range pr.t.Rules {
		if r.Role != "" {
			roles = append(roles, r.Role)
		}
	}
	// Roles the table writes in upper snake case are read by their
	// kebab-case names under info.roleNames (ADR-099); a role the rule
	// cannot read keeps the name the table gives it, and is left out.
	pr.roleNames = readRoleNames(roles)
	pr.wireOf = map[string]string{}
	grants := make([]dumpGrant, len(pr.t.Grants))
	isRole := map[string]bool{}
	// A client that is a row's subject holds a role named after it, so
	// the matrix shows it beside the roles (ADR-110). Where a role of the
	// table already has that name, the client's role takes the name with
	// -client added, so neither holds the other's grants, and writeClients
	// asks about it.
	taken := map[string]bool{}
	for _, r := range roles {
		if n, ok := pr.roleNames.name(r); ok {
			taken[n] = true
		}
	}
	var clientNames []string
	for _, g := range pr.t.Grants {
		if g.Client != "" && !contains(clientNames, g.Client) {
			clientNames = append(clientNames, g.Client)
		}
	}
	sort.Strings(clientNames)
	clientRole := map[string]string{} // client -> the role it holds
	isClientRole := map[string]bool{}
	for _, c := range clientNames {
		n := c
		for taken[n] {
			n += "-client"
		}
		taken[n] = true
		clientRole[c] = n
		isClientRole[n] = true
	}
	for i, g := range pr.t.Grants {
		if g.Client != "" {
			g.Role = clientRole[g.Client]
			isRole[g.Role] = true
			pr.wireOf[g.Role] = g.Client
		} else if n, ok := pr.roleNames.name(g.Role); ok {
			g.Role = n
			isRole[n] = true
			pr.wireOf[n] = pr.t.Grants[i].Role
		}
		grants[i] = g
	}
	var names []string
	for _, g := range grants {
		if isRole[g.Role] && g.Permission != "public" {
			names = append(names, g.Permission)
		}
	}
	// dxlib_module's privilege names map to permissions by the rule of
	// ADR-076, as extract openapi and extract go map them.
	privileges := newPrivilegeNames(names)
	byRole := map[string][]string{}
	through := map[string]map[string][]string{} // role -> permission -> the clients a row holds it through
	plain := map[string]bool{}                  // "role permission" granted by a row with no through
	type asked struct{ role, question, why string }
	var problems []asked
	pr.mappedFrom = map[string]string{}
	for _, g := range grants {
		switch {
		case !isRole[g.Role]:
			pr.gap(g.Role, "grant of %s to %q: %s; left out", g.Permission, g.Role, pr.roleNames.why(g.Role))
		case g.Permission == "public":
			pr.gap(pr.wireOf[g.Role], "grant of public to %s: public is open to everyone and granted by no role; left out", pr.wireOf[g.Role])
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
			if g.Through != "" {
				if through[g.Role] == nil {
					through[g.Role] = map[string][]string{}
				}
				through[g.Role][p] = append(through[g.Role][p], g.Through)
			} else {
				plain[g.Role+" "+p] = true
			}
			if !contains(byRole[g.Role], p) {
				byRole[g.Role] = append(byRole[g.Role], p)
			}
		}
	}
	// What specarch check compares: every grant but a grant of public, by
	// the names the tree reads, or as the table writes them where it reads
	// none.
	for i, g := range grants {
		if g.Permission == "public" {
			continue
		}
		p := g.Permission
		if q, _ := privileges.grantProblem(g.Role, g.Permission); isRole[g.Role] && q == "" {
			p, _, _ = privileges.read(g.Permission)
		}
		pr.compared = append(pr.compared, comparedGrant{role: g.Role, permission: p, index: i})
	}
	unknown := map[string]bool{}
	for _, a := range problems {
		unknown[a.role] = true
	}
	for _, g := range grants {
		if isRole[g.Role] && byRole[g.Role] == nil {
			pr.gap(pr.wireOf[g.Role], "role %s: it grants nothing the meta-model holds, and a role that grants nothing is left out", pr.wireOf[g.Role])
			byRole[g.Role] = []string{}
		}
	}
	gates := map[dumpGate]bool{}
	for i, g := range pr.t.Gates {
		where := fmt.Sprintf("%s, gate %d", dumpName, i+1)
		switch {
		case g.Check == "":
			return refuse("%s names no check; a gate is the check and either the setting it needs or the kind of caller it skips", where)
		case g.Setting != "" && g.BypassedFor != "":
			return refuse("%s names both the setting %s and the kind of caller %s for %s; a gate is one or the other, so list each as a gate of its own", where, g.Setting, g.BypassedFor, g.Check)
		case g.Setting == "" && g.BypassedFor == "":
			return refuse("%s names neither a setting nor a kind of caller for %s; a gate is the check and either the setting it needs or the kind of caller it skips", where, g.Check)
		case g.BypassedFor != "" && !callerKinds[g.BypassedFor]:
			return refuse("%s: %s is skipped for %q, and the kind of caller a check is skipped for is client, a caller on its own account rather than a person's", where, g.Check, g.BypassedFor)
		}
		if gates[g] {
			if g.BypassedFor != "" {
				return refuse("%s: the check %s skipped for %s is listed twice", where, g.Check, g.BypassedFor)
			}
			return refuse("%s: the check %s needing %s is listed twice", where, g.Check, g.Setting)
		}
		gates[g] = true
	}
	if pr.t.Rules != nil {
		pr.res.say("counted %s, %s and %s: every entry of the permission table's lists, which hold every grant the check reads, every check the printer declares a setting switches off or a kind of caller skips and every rule of the policy the check reads", plural(len(pr.t.Grants), "grant"), plural(len(pr.t.Gates), "gate"), plural(len(pr.t.Rules), "rule"))
	} else {
		pr.res.say("counted %s and %s: every entry of the permission table's lists, which hold every grant the check reads and every check the printer declares a setting switches off or a kind of caller skips", plural(len(pr.t.Grants), "grant"), plural(len(pr.t.Gates), "gate"))
	}
	ruled, err := pr.readRules(dumpName, isRole)
	if err != nil {
		return err
	}
	for role, perms := range ruled {
		byRole[role] = append(byRole[role], perms...)
	}

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
			for _, g := range grants {
				if g.Role == name && !contains(granted, g.Permission) {
					granted = append(granted, g.Permission)
				}
			}
			sort.Strings(granted)
			role := mapping()
			if len(perms) > 0 {
				set(role, "permissions", perms)
			}
			if narrowed := pr.throughOf(name, through[name], plain); narrowed != nil {
				set(role, "through", narrowed)
			}
			set(role, "origin", "stated")
			wire := pr.wireOf[name]
			var says []string
			if len(granted) > 0 {
				says = append(says, fmt.Sprintf("%s grants %s.", wire, joinAnd(granted)))
			}
			if reach := pr.roleRows(wire); reach != "" && !isClientRole[name] {
				says = append(says, reach)
			}
			set(role, "cites", []*yaml.Node{citation(pr.key, pr.t.Path+" role "+wire, strings.Join(says, " "))})
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
			var wires []string
			for _, r := range roles {
				if !contains(wires, pr.wireOf[r]) {
					wires = append(wires, pr.wireOf[r])
				}
			}
			cites := []*yaml.Node{citation(pr.key, pr.t.Path+" role "+wires[0], fmt.Sprintf("%s %s %s.", joinAnd(wires), grantOrGrants(len(wires)), named))}
			if rp := pr.rulePermission(n); rp != nil {
				set(pr.permissions, n, pr.rulePermissionEntry(rp))
			} else if pr.mappedFrom[n] != "" {
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
		pr.renameQuestion()
		pr.writeClients(roleNames, clientRole, through)
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
		if sorted[i].Setting != sorted[j].Setting {
			return sorted[i].Setting < sorted[j].Setting
		}
		return sorted[i].BypassedFor < sorted[j].BypassedFor
	})
	for _, g := range sorted {
		if g.BypassedFor != "" {
			pr.res.say("gate: the check %s is skipped for every %s, and lets every %s through whatever its roles grant", g.Check, g.BypassedFor, g.BypassedFor)
			q, why := bypassQuestion(g.Check, g.BypassedFor)
			pr.question(q, []string{"roles"}, why, citation(pr.key, pr.t.Path+" gate "+g.Check, fmt.Sprintf("The permission table declares that %s is skipped for every %s.", g.Check, g.BypassedFor)))
			pr.bypassBlocks[g.Check] = child(pr.questions.Content[len(pr.questions.Content)-1], "blocks")
			continue
		}
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

// guarded gives each bypassed gate's question the operations the check
// guards, from the route table: each route that names a permission other
// than public, and, where the check reads a policy of rules, every route,
// since the policy decides each one.
func (pr *permissionReader) guarded(rd *routeReader) {
	if len(pr.bypassBlocks) == 0 {
		return
	}
	var at []string
	for _, w := range rd.written {
		if (w.printed != nil && *w.printed != "public") || (w.printed == nil && pr.t.Rules != nil) {
			at = append(at, w.at+"/permission")
		}
	}
	checks := make([]string, 0, len(pr.bypassBlocks))
	for c := range pr.bypassBlocks {
		checks = append(checks, c)
	}
	sort.Strings(checks)
	for _, c := range checks {
		if len(at) == 0 {
			pr.res.say("gate: the check %s guards no route of the route table, so its question blocks the roles", c)
			continue
		}
		pr.bypassBlocks[c].Content = value(at).Content
		pr.res.say("gate: the check %s guards %s of the route table, and its question blocks the permission of each", c, plural(len(at), "operation"))
	}
}

func grantOrGrants(n int) string {
	if n == 1 {
		return "grants"
	}
	return "grant"
}

// laterCommit is whichever of two commits has the other as an ancestor.
func laterCommit(repo, a, b string) (string, error) {
	if ok, err := isAncestor(repo, a, b); err != nil {
		return "", err
	} else if ok {
		return b, nil
	}
	if ok, err := isAncestor(repo, b, a); err != nil {
		return "", err
	} else if ok {
		return a, nil
	}
	return "", fmt.Errorf("neither %s nor %s is an ancestor of the other", a, b)
}

// throughOf is a role's through, keyed by permission, from the rows that
// hold a grant through one client; nil when none does. A permission one
// row grants through a client and another grants with none is a must
// question at the role, since the rows disagree on whether the grant is
// narrowed, and is left out of through (ADR-106).
func (pr *permissionReader) throughOf(role string, by map[string][]string, plain map[string]bool) *yaml.Node {
	if len(by) == 0 {
		return nil
	}
	var perms []string
	for p := range by {
		perms = append(perms, p)
	}
	sort.Strings(perms)
	out := mapping()
	for _, p := range perms {
		clients := append([]string(nil), by[p]...)
		sort.Strings(clients)
		if plain[role+" "+p] {
			pr.question(fmt.Sprintf("Role %s is granted %s by one row through %s and by another through any client. Is the grant narrowed to %s, or does it hold through every client?", pr.wireOf[role], p, joinAnd(clients), joinAnd(clients)),
				[]string{"#/roles/" + escapeToken(role) + "/through"},
				"Two rows of the table disagree on whether the grant is narrowed, and the check reads one of them in an order the table does not show; through is never guessed.",
				citation(pr.key, pr.t.Path+" role "+pr.wireOf[role], fmt.Sprintf("%s grants %s through %s, and grants it with no client too.", pr.wireOf[role], p, joinAnd(clients))))
			continue
		}
		set(out, p, flow(value(clients)))
	}
	if len(out.Content) == 0 {
		return nil
	}
	return out
}

// writeClients declares every client the grants name: a row's subject as
// a service holding the role named after it, with a must question for its
// proof and a could question to rename the role; and every client a grant
// holds through as an application, with a must question for what it is.
// One name used as both is a must question (ADR-106, ADR-110).
func (pr *permissionReader) writeClients(roleNames []string, clientRole map[string]string, through map[string]map[string][]string) {
	apps := map[string]bool{}
	var narrowed []string
	for _, r := range roleNames {
		if len(through[r]) > 0 && child(child(pr.roles, r), "through") != nil {
			narrowed = append(narrowed, "#/roles/"+escapeToken(r)+"/through")
		}
		for _, list := range through[r] {
			for _, c := range list {
				apps[c] = true
			}
		}
	}
	var names []string
	for c := range clientRole {
		names = append(names, c)
	}
	for c := range apps {
		if clientRole[c] == "" {
			names = append(names, c)
		}
	}
	if len(names) == 0 {
		return
	}
	sort.Strings(names)
	pr.clients = mapping()
	var services, appNames []string
	for _, c := range names {
		cl := mapping()
		switch {
		case clientRole[c] != "":
			set(cl, "kind", "service")
			set(cl, "roles", flow(value([]string{clientRole[c]})))
			services = append(services, c)
		default:
			set(cl, "kind", "application")
		}
		if apps[c] {
			appNames = append(appNames, c)
		}
		set(cl, "origin", "stated")
		says := fmt.Sprintf("The table grants %s permissions directly.", c)
		if clientRole[c] == "" {
			says = fmt.Sprintf("The table narrows a grant to %s.", c)
		}
		set(cl, "cites", []*yaml.Node{citation(pr.key, pr.t.Path+" client "+c, says)})
		set(pr.clients, c, cl)
		if r := clientRole[c]; r != "" && r != c {
			pr.question(fmt.Sprintf("The table grants the client %s permissions directly and names a role %s too. Is the client meant to hold the role %s, or only the grants the table gives the client, which the reader writes as the role %s?", c, c, c, r),
				[]string{"#/roles/" + escapeToken(r), "#/clients/" + escapeToken(c) + "/roles"},
				fmt.Sprintf("A client and a role of one name are two subjects in the table; read as one, the client would hold every grant of the role. The reader keeps them apart, and the role %s is named for this.", r))
		}
		if clientRole[c] != "" && apps[c] {
			pr.question(fmt.Sprintf("The table names %s as the subject of its own grants, a caller on its own account, and as the client a grant holds through, the application a person signs in through. Which is it?", c),
				[]string{"#/clients/" + escapeToken(c)},
				"One name is used as a caller and as a condition; a client is an application or a service, and the reader writes it as the service its own grants make it.")
		}
	}
	for _, c := range services {
		pr.question(fmt.Sprintf("The client %s calls on its own account with the grants the table gives it. How does it prove itself on each request: its proof?", c),
			[]string{"#/clients/" + escapeToken(c) + "/proof"},
			"A client that calls on its own account must prove itself on each request (RFC 6749, section 4.4), and the table names its grants, not its credential.")
	}
	if len(services) > 0 {
		var blocks, held []string
		for _, c := range services {
			blocks = append(blocks, "#/roles/"+escapeToken(clientRole[c]))
			held = append(held, clientRole[c])
		}
		pr.couldQuestion(fmt.Sprintf("The roles %s are named after the clients the table grants permissions directly. Should any be renamed to say what it allows?", joinAnd(held)),
			blocks, "A role is a set of grants a reviewer reads in the matrix; the table gives the client's grants, not a role's name. Nothing waits for the answer.")
	}
	var blocks []string
	for _, c := range names {
		blocks = append(blocks, "#/clients/"+escapeToken(c)+"/description")
	}
	pr.question(fmt.Sprintf("What is each client the table names: %s?", joinAnd(names)), blocks,
		"The table names a client by the grants it holds or narrows, not what it is, who runs it or who signs in through it.")
	if len(narrowed) > 0 {
		pr.question(fmt.Sprintf("The table narrows grants to the applications %s. Which token carries the application the person signed in through: the client_id of an access token, or tctx.application of a transaction token?", joinAnd(appNames)),
			append(narrowed, "tokens"),
			"through is checked against the application a token carries, which a session does not; the table names the condition, not the token (ADR-106, ADR-108).")
	}
}
