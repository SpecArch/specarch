package genui

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/SpecArch/specarch/internal/mark"
)

// The person's session (ADR-197). With settings.session the screens read
// who is signed in once per page: the menu shows only the entries whose
// page opens to the person, a field's hiddenUnless and readOnlyUnless
// follow the permissions they hold, and index.html opens the home of the
// first of their roles that has one. Without it they read nothing, and
// each of these is reported where the specification asks for it.

//go:embed runtime/session.js
var sessionRuntime string

// session is what the target's settings say of the person's session.
type session struct {
	// operation, method and path are the operation that answers who is
	// signed in.
	operation, method, path string
	// signedIn, permissions and roles are the properties of its answer, on
	// the wire; roles is "" when the settings name none.
	signedIn, permissions, roles string
}

// readSession reads settings.session, as the other ui generators do; nil,
// with an error, when it names no GET operation without parameters or
// properties its answer does not have, and nil when the settings name
// none.
func (g *gen) readSession() *session {
	s, ok := obj(g.settings["session"])
	if !ok {
		return nil
	}
	at := "/"
	opID := text(s["operation"])
	op, path, method := g.operation(opID)
	switch {
	case op == nil || method != "get":
		g.problem("error", at, "the ui target's settings.session names %q, which is not a GET operation of the specification; name the operation that answers who is signed in", opID)
		return nil
	case len(routeParameters(path)) > 0:
		g.problem("error", at, "the ui target's settings.session names %s, whose path takes a parameter; the screens read the session at one path", opID)
		return nil
	}
	props := obj0(obj0(obj0(obj0(obj0(obj0(obj0(op["responses"])["200"])["content"])["application/json"])["schema"]))["properties"])
	out := &session{operation: opID, method: strings.ToUpper(method), path: path}
	for _, k := range []struct{ key, typ string }{{"signedIn", "boolean"}, {"permissions", "array"}} {
		prop := text(s[k.key])
		if prop == "" {
			g.problem("error", at, "the ui target's settings.session names no %s, the property of %s's answer that holds it", k.key, opID)
			return nil
		}
		if text(obj0(props[prop])["type"]) != k.typ {
			g.problem("error", at, "the ui target's settings.session says %s holds %s, and the 200 answer of %s has no property %s of type %s", prop, k.key, opID, prop, k.typ)
			return nil
		}
		if k.key == "signedIn" {
			out.signedIn = g.wire(prop)
		} else {
			out.permissions = g.wire(prop)
		}
	}
	if prop := text(s["roles"]); prop != "" {
		if text(obj0(props[prop])["type"]) != "array" {
			g.problem("error", at, "the ui target's settings.session says %s holds roles, and the 200 answer of %s has no property %s of type array", prop, opID, prop)
			return nil
		}
		out.roles = g.wire(prop)
	}
	return out
}

// unless is the permission a field's hiddenUnless or readOnlyUnless names,
// for the screens to read against the session; without settings.session
// it is "", and a warning says the field is shown or changeable to
// everyone who opens the page.
func (g *gen) unless(at, field, key, perm string) string {
	if perm == "" || g.session != nil {
		return perm
	}
	what := "shown"
	if key == "readOnlyUnless" {
		what = "changeable"
	}
	g.problem("warning", at, "%s is %s unless the caller holds %s, and the ui target's settings.session names no operation that answers who is signed in, so it is %s to everyone who opens the page; name it under settings.session. The service still answers the field and takes it as its operation's permission allows", field, map[string]string{"hiddenUnless": "hidden", "readOnlyUnless": "read-only"}[key], perm, what)
	return ""
}

// home is a role's home page.
type home struct{ role, page string }

// homes are the roles' home pages the screens open, by role: those written
// or another stakeholder's (ADR-197). A home that is not is reported at its
// role, which then opens no home; without the session's roles none is
// opened, and a warning says so.
func (g *gen) homes() []home {
	roles := obj0(g.spec["roles"])
	var out []home
	for _, r := range sortedKeys(roles) {
		page := text(obj0(roles[r])["home"])
		if page == "" {
			continue
		}
		at := "/roles/" + mark.Escape(r) + "/home"
		switch {
		case g.session == nil:
			g.problem("warning", at, "role %s's home is %s, and the ui target's settings.session names no operation that answers who is signed in, so specarch-gen-ui cannot tell a person's roles and opens no home; name it under settings.session, with the property that holds the roles under roles", r, page)
			continue
		case !g.opens(page):
			if _, ok := obj(obj0(g.spec["pages"])[page]); ok && !g.failed() {
				g.problem("warning", at, "role %s's home is %s, which specarch-gen-ui does not write, so %s opens no home", r, page, r)
			}
			continue
		}
		out = append(out, home{role: r, page: page})
	}
	if len(out) > 0 && g.session.roles == "" {
		g.problem("warning", "/roles", "the roles name their home pages, and the ui target's settings.session names no roles, the property of the session's answer that holds them, so specarch-gen-ui opens no home; name it under settings.session.roles")
		return nil
	}
	return out
}

// sessionJS is session.js: the operation that answers who is signed in,
// the properties of its answer, the roles' homes and the runtime that
// reads them.
func (g *gen) sessionJS(homes []home) string {
	var b strings.Builder
	b.WriteString(`import { call } from "./service.js";

// The operation that answers who is signed in, and the properties of its
// answer that hold what the screens read.
`)
	roles := "null"
	if g.session.roles != "" {
		roles = jsString(g.session.roles)
	}
	fmt.Fprintf(&b, "const SESSION = {\n  source: { operation: %s, method: %s, path: %s },\n  signedIn: %s,\n  permissions: %s,\n  roles: %s,\n};\n",
		jsString(g.session.operation), jsString(g.session.method), jsString(g.session.path), jsString(g.session.signedIn), jsString(g.session.permissions), roles)
	b.WriteString("\n// The page each role's home is, by role.\nconst HOMES = {")
	if len(homes) == 0 {
		b.WriteString("};\n")
	} else {
		b.WriteString("\n")
		for _, h := range homes {
			fmt.Fprintf(&b, "  %s: %s,\n", jsString(h.role), jsString(h.page))
		}
		b.WriteString("};\n")
	}
	b.WriteString(sessionRuntime)
	return b.String()
}

// indexPage is index.html and index.js, the screens' start: once the
// session is read it opens the home of the first of the person's roles
// that has one, and leaves anyone else at the menu. Nil when no role has a
// home the screens open, or a page takes the name, which is reported.
func (g *gen) indexPage(homes []home) []File {
	if len(homes) == 0 {
		return nil
	}
	if _, taken := obj0(g.spec["pages"])["index"]; taken {
		g.problem("error", "/pages/index", "index is a page, and its index.html would be the file the screens start at, which opens each role's home; rename the page")
		return nil
	}
	title := text(obj0(g.spec["info"])["title"])
	out := g.headOf("index", title, text(g.settings["language"]), true)
	out = append(out, `<p id="status" role="status" aria-live="polite"></p>`, "</main>", "</body>", "</html>")
	js := `// The screens' start: once the session is read, it opens the home of the
// first of the person's roles that has one, and leaves anyone else at the
// menu.

import { href } from "./pages.js";
import { homeOf, session } from "./session.js";

session().then((person) => {
  const home = homeOf(person);
  if (home !== undefined) {
    location.replace(href(home, {}));
  } else if (person === null) {
    document.getElementById("status").textContent = ` + jsString(defaultFailed) + `;
  }
});
`
	return []File{{Path: "index.html", Content: strings.Join(out, "\n") + "\n"}, {Path: "index.js", Content: js}}
}
