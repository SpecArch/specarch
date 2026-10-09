package extract

import (
	"fmt"
	"go/ast"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The import path every package of dxlib_module starts with.
const dxlibModule = "github.com/donnyhardyanto/dxlib_module/"

var (
	// The insert methods of a dxlib table.
	insertMethod = regexp.MustCompile(`^(Tx)?Insert(Auto)?(ReturningId)?(WithAudit)?$`)
	// dxlib_module's calls that grant a privilege to a role by its id:
	// RolePrivilegeMustInsert, RolePrivilegeTxInsert and the like.
	grantMethod = regexp.MustCompile(`^RolePrivilege(Tx|Sx|Wg|SWg)?(Must)?Insert$`)
)

// seedRole is one role a seed inserts.
type seedRole struct {
	name        string
	description string
	clause      string
}

// seedGrant is one privilege a seed grants to a role.
type seedGrant struct {
	role      string
	privilege string
	clause    string
}

// seedPrivilege is one privilege a seed inserts.
type seedPrivilege struct {
	name        string
	description string
	clause      string
}

// seeds is what dxlib_module's seed calls in the module give.
type seeds struct {
	roles      map[string]*seedRole
	privileges map[string]*seedPrivilege
	grants     []seedGrant
	calls      int
}

// readSeeds reads the role, privilege and grant inserts of every file that
// imports a package of dxlib_module: a role or a privilege inserted into
// dxlib_module's tables with a literal nameid, and a grant whose role id is
// a name the same function assigns from a role insert, and whose privilege
// is a literal.
func (g *goReader) readSeeds() {
	g.seeds = &seeds{roles: map[string]*seedRole{}, privileges: map[string]*seedPrivilege{}}
	for _, f := range g.files {
		uses := false
		for _, ip := range f.imports {
			uses = uses || strings.HasPrefix(ip, dxlibModule)
		}
		if !uses {
			continue
		}
		for _, d := range f.ast.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			g.seedFunc(f, fd)
		}
	}
	if g.seeds.calls > 0 || len(g.seeds.roles) > 0 {
		g.res.say("dxlib_module: counted %s, %s and %s seeded with literal names: every insert into dxlib_module's role and privilege tables, and every RolePrivilege...Insert call, in a file that imports a package of dxlib_module", plural(len(g.seeds.roles), "role"), plural(len(g.seeds.privileges), "privilege"), plural(len(g.seeds.grants), "grant"))
	}
}

// insertInto is the table an insert call writes into, Role or Privilege,
// and the literal values it inserts.
func (g *goReader) insertInto(f *goFile, call *ast.CallExpr) (table string, values map[string]string) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !insertMethod.MatchString(sel.Sel.Name) {
		return "", nil
	}
	recv, ok := sel.X.(*ast.SelectorExpr)
	if !ok || (recv.Sel.Name != "Role" && recv.Sel.Name != "Privilege") {
		return "", nil
	}
	values = map[string]string{}
	for _, a := range call.Args {
		lit, ok := a.(*ast.CompositeLit)
		if !ok {
			continue
		}
		for _, el := range lit.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			k, ok1 := g.stringValue(f, kv.Key)
			v, ok2 := g.stringValue(f, kv.Value)
			if ok1 && ok2 {
				values[k] = v
			}
		}
		return recv.Sel.Name, values
	}
	return recv.Sel.Name, values
}

// seedFunc reads the seed calls of one function: first the roles it
// inserts and the names their ids are assigned to, then its grants.
func (g *goReader) seedFunc(f *goFile, fd *ast.FuncDecl) {
	roleIDs := map[string][]string{} // a name -> the role nameids assigned to it, "" for one not read
	var stack []ast.Node
	type grantCall struct {
		call  *ast.CallExpr
		name  string
		stack []ast.Node
	}
	var grants []grantCall
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if as, ok := n.(*ast.AssignStmt); ok && len(as.Rhs) == 1 {
			if call, ok := as.Rhs[0].(*ast.CallExpr); ok {
				if table, values := g.insertInto(f, call); table == "Role" {
					roleIDs[exprText(as.Lhs[0])] = append(roleIDs[exprText(as.Lhs[0])], values["nameid"])
				}
			}
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if table, values := g.insertInto(f, call); table != "" {
				g.seedInsert(f, call, table, values, stack)
			} else if sel, ok := call.Fun.(*ast.SelectorExpr); ok && grantMethod.MatchString(sel.Sel.Name) {
				grants = append(grants, grantCall{call, sel.Sel.Name, append([]ast.Node(nil), stack...)})
			}
		}
		stack = append(stack, n)
		return true
	})
	for _, gc := range grants {
		g.seedGrant(f, gc.call, gc.name, gc.stack, roleIDs)
	}
}

// seedInsert reads one insert into the role or the privilege table.
func (g *goReader) seedInsert(f *goFile, call *ast.CallExpr, table string, values map[string]string, stack []ast.Node) {
	clause := g.clause(f, call.Pos())
	f.cited = true
	g.seeds.calls++
	what := strings.ToLower(table)
	if b := branch(stack); b != nil {
		g.question("must", fmt.Sprintf("%s inserts a %s %s at %s, so the reader cannot tell which %ss the seed inserts, or whether it inserts one. Which %ss does the running system hold?", clause, what, branchWord(b), g.clause(f, b.Pos()), what, what),
			[]string{"roles"}, "Syntax does not run the code; only the tables the check reads, printed as a permission table, say what a loop or a condition seeds.", g.at(clause, "Inserts a "+what+" "+branchWord(b)+"."))
		return
	}
	name := values["nameid"]
	if name == "" {
		g.question("must", fmt.Sprintf("%s inserts a %s whose nameid is not a literal. Which %s is it?", clause, what, what),
			[]string{"roles"}, "A "+what+" is named by its nameid, and a computed one is not known by syntax.", g.at(clause, "Inserts a "+what+" with no literal nameid."))
		return
	}
	switch table {
	case "Role":
		if _, seen := g.seeds.roles[name]; !seen {
			g.seeds.roles[name] = &seedRole{name: name, description: values["description"], clause: clause}
		}
	case "Privilege":
		if _, seen := g.seeds.privileges[name]; !seen {
			g.seeds.privileges[name] = &seedPrivilege{name: name, description: values["description"], clause: clause}
		}
	}
}

// seedGrant reads one grant: its role id traced to a role the function
// inserts, and its privilege a literal.
func (g *goReader) seedGrant(f *goFile, call *ast.CallExpr, name string, stack []ast.Node, roleIDs map[string][]string) {
	clause := g.clause(f, call.Pos())
	f.cited = true
	g.seeds.calls++
	if len(call.Args) < 2 {
		g.res.say("nothing read: %s calls %s with %d arguments, and dxlib_module's takes a role id and a privilege name last", clause, name, len(call.Args))
		return
	}
	if b := branch(stack); b != nil {
		g.question("must", fmt.Sprintf("%s grants a privilege with %s %s at %s, so the reader cannot tell which grants the seed makes, or whether it makes one. Which grants does the running system hold?", clause, name, branchWord(b), g.clause(f, b.Pos())),
			[]string{"roles"}, "Syntax does not run the code; only the tables the check reads, printed as a permission table, say what a loop or a condition seeds.", g.at(clause, "Calls "+name+" "+branchWord(b)+"."))
		return
	}
	roleArg, privArg := call.Args[len(call.Args)-2], call.Args[len(call.Args)-1]
	privilege, privOK := g.stringValue(f, privArg)
	role := ""
	if id, ok := roleArg.(*ast.Ident); ok {
		if names := roleIDs[id.Name]; len(names) == 1 {
			role = names[0]
		}
	}
	if role == "" || !privOK {
		var what []string
		if role == "" {
			what = append(what, "the role "+exprText(roleArg)+", which the reader does not trace to a role the same function inserts once with a literal nameid")
		}
		if !privOK {
			what = append(what, "the privilege "+exprText(privArg)+", which is not a literal")
		}
		g.question("must", fmt.Sprintf("%s grants with %s %s. Which role does it grant which privilege?", clause, name, strings.Join(what, ", and ")),
			[]string{"roles"}, "A grant is a role and a privilege; one the reader cannot read is a grant it would miss.", g.at(clause, "Calls "+name+" with "+strings.Join(what, ", and ")+"."))
		return
	}
	g.seeds.grants = append(g.seeds.grants, seedGrant{role: role, privilege: privilege, clause: clause})
}

// seedNames is every privilege name the seeds give.
func (g *goReader) seedNames() []string {
	var names []string
	for _, gr := range g.seeds.grants {
		if roleWord.MatchString(gr.role) {
			names = append(names, gr.privilege)
		}
	}
	for n := range g.seeds.privileges {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// writeSeeds writes the roles the seeds grant, and the permissions they
// declare, mapped as the endpoints' are; mapped tells which permission the
// rule of ADR-076 gave from which privilege.
func (g *goReader) writeSeeds(privileges *privilegeNames, declare func(permission, privilege, description, clause, says string)) {
	byRole := map[string][]string{}
	cites := map[string][]*yaml.Node{}
	unknown := map[string]bool{}
	type asked struct{ role, question, why, clause string }
	var problems []asked
	for _, gr := range g.seeds.grants {
		switch {
		case !roleWord.MatchString(gr.role):
			g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: grant of %s to %q: the role's name is not kebab-case, which a role name is; left out", gr.clause, gr.privilege, gr.role), clause: gr.clause, blocks: []string{"roles"}})
			continue
		case gr.privilege == "public":
			g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: grant of public to %s: public is open to everyone and granted by no role; left out", gr.clause, gr.role), clause: gr.clause, blocks: []string{"roles"}})
			continue
		}
		if byRole[gr.role] == nil {
			byRole[gr.role] = []string{}
		}
		cites[gr.role] = append(cites[gr.role], g.at(gr.clause, fmt.Sprintf("Grants %s to %s.", gr.privilege, gr.role)))
		if q, why := privileges.grantProblem(gr.role, gr.privilege); q != "" {
			problems = append(problems, asked{gr.role, gr.clause + ": " + q, why, gr.clause})
			unknown[gr.role] = true
			continue
		}
		p, _, _ := privileges.read(gr.privilege)
		if !contains(byRole[gr.role], p) {
			byRole[gr.role] = append(byRole[gr.role], p)
		}
		declare(p, gr.privilege, "", gr.clause, fmt.Sprintf("Grants %s to %s.", gr.privilege, gr.role))
	}
	for _, sp := range sortedPrivileges(g.seeds.privileges) {
		if sp.name == "public" || sp.name == "EVERYTHING" {
			continue
		}
		p, _, collides := privileges.read(sp.name)
		if p == "" || len(collides) > 0 {
			continue
		}
		declare(p, sp.name, sp.description, sp.clause, fmt.Sprintf("Inserts the privilege %s.", sp.name))
	}
	var names []string
	for name, perms := range byRole {
		if len(perms) > 0 || unknown[name] {
			names = append(names, name)
		}
	}
	var seededRoles []string
	for name := range g.seeds.roles {
		seededRoles = append(seededRoles, name)
	}
	sort.Strings(seededRoles)
	for _, name := range seededRoles {
		r := g.seeds.roles[name]
		if roleWord.MatchString(name) && byRole[name] == nil {
			g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: role %s: the seeds grant it nothing, and a role that grants nothing is left out", r.clause, name), clause: r.clause, blocks: []string{"roles"}})
		} else if !roleWord.MatchString(name) && !granted(g.seeds.grants, name) {
			g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: role %q: its name is not kebab-case, which a role name is; left out", r.clause, name), clause: r.clause, blocks: []string{"roles"}})
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return
	}
	g.roles = &yaml.Node{Kind: yaml.MappingNode}
	var undescribed []string
	var blocks []string
	for _, name := range names {
		perms := byRole[name]
		sort.Strings(perms)
		role := mapping()
		r := g.seeds.roles[name]
		if r != nil && strings.TrimSpace(r.description) != "" {
			set(role, "description", r.description)
		} else {
			undescribed = append(undescribed, name)
			blocks = append(blocks, "#/roles/"+escapeToken(name)+"/description")
		}
		if len(perms) > 0 {
			set(role, "permissions", perms)
		}
		set(role, "origin", "stated")
		var c []*yaml.Node
		if r != nil {
			c = append(c, g.at(r.clause, fmt.Sprintf("Inserts the role %s.", name)))
		}
		set(role, "cites", append(c, cites[name]...))
		set(g.roles, name, role)
	}
	if len(undescribed) > 0 {
		g.question("must", fmt.Sprintf("What is each role for, and who holds it: %s?", strings.Join(undescribed, ", ")),
			blocks, "A seed names a role and what it grants; the description is a literal of the role's insert, and none is read for these.")
	}
	for _, a := range problems {
		g.question("must", a.question, []string{"#/roles/" + escapeToken(a.role) + "/permissions"}, a.why, g.at(a.clause, "Grants a privilege to "+a.role+"."))
	}
	for _, name := range names {
		where := ""
		if r := g.seeds.roles[name]; r != nil {
			where = r.clause
		} else {
			for _, gr := range g.seeds.grants {
				if gr.role == name {
					where = gr.clause
					break
				}
			}
		}
		g.question("must", fmt.Sprintf("Does the running system grant role %s what the seeds grant it? It is seeded at %s, and no permission table of the running system was read with it.", name, where),
			[]string{"#/roles/" + escapeToken(name)}, "A seed that did not run, or whose rows something removed, is not a grant; the tables the check reads, printed as a permission table, say what is granted, and merging it answers this.", g.at(where, "Seeds role "+name+"."))
	}
}

func granted(grants []seedGrant, role string) bool {
	for _, gr := range grants {
		if gr.role == role {
			return true
		}
	}
	return false
}

func sortedPrivileges(m map[string]*seedPrivilege) []*seedPrivilege {
	var list []*seedPrivilege
	for _, p := range m {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })
	return list
}
