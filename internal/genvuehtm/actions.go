package genvuehtm

import (
	"strings"

	"github.com/SpecArch/specarch/internal/genscreens"
)

// action is one operation the pages call, as config/actions.js writes it:
// how it travels, said once, so a page's data names the operation alone.
type action struct {
	operation, method, path, permission string
	// transport are the members a call carries beside its operation and
	// path: handshake, wire, stream and mediaType, as genscreens gives them.
	transport []member
}

// transportKeys are the members of a call in a page's schema that say how
// it travels, which move to config/actions.js.
var transportKeys = map[string]bool{"method": true, "path": true, "handshake": true, "wire": true, "stream": true, "mediaType": true}

// reduce is a schema with each call reduced to its operation's name: an
// object with an operation member loses the members that say how the call
// travels, which the action of that operation keeps, so a call is said
// once and api.call reads it by name. Every other value is kept as it is.
func (g *gen) reduce(v value) value {
	switch x := v.(type) {
	case genscreens.ObjectValue:
		var op string
		for _, m := range x.Members {
			if m.Key == "operation" {
				if l, ok := m.Val.(genscreens.Literal); ok && l.String {
					op = l.Text
				}
			}
		}
		members := make([]member, 0, len(x.Members))
		var carried []member
		for _, m := range x.Members {
			if op != "" && transportKeys[m.Key] {
				carried = append(carried, m)
				continue
			}
			members = append(members, member{Key: m.Key, Val: g.reduce(m.Val)})
		}
		if op != "" {
			g.record(op, carried)
		}
		return genscreens.ObjectValue{Members: members, Dictionary: x.Dictionary}
	case genscreens.ArrayValue:
		items := make([]value, len(x.Items))
		for i, e := range x.Items {
			items[i] = g.reduce(e)
		}
		return genscreens.ArrayValue{Items: items, Rule: x.Rule}
	}
	return v
}

// record keeps an operation the pages call with how it travels, from the
// members a call carried; the method and the path come from the
// specification, so a list's source, which carries no method, gets GET.
func (g *gen) record(id string, carried []member) {
	if _, seen := g.actions[id]; seen {
		return
	}
	a := &action{operation: id}
	op, path, method := g.Operation(id)
	a.path = path
	a.method = strings.ToUpper(method)
	a.permission = text(op["permission"])
	for _, m := range carried {
		switch m.Key {
		case "method":
			if l, ok := m.Val.(genscreens.Literal); ok && a.method == "" {
				a.method = l.Text
			}
		case "path":
			if l, ok := m.Val.(genscreens.Literal); ok {
				a.path = l.Text // the path as the screens call it, a prefix before another module's
			}
		default:
			a.transport = append(a.transport, m)
		}
	}
	g.actions[id] = a
}

// sessionAction records the operation that answers who is signed in, which
// the session reads through api.call too.
func (g *gen) sessionAction(sess *genscreens.Session) {
	id := text(obj0(g.Settings["session"])["operation"])
	if id == "" {
		return
	}
	op, path, _ := g.Operation(id)
	if op == nil {
		return
	}
	g.record(id, append([]member{{Key: "path", Val: str(path)}}, g.Transport(op, path)...))
}

// actionsFile is config/actions.js: every operation the pages call, by
// operationId, with its method, its path with its {parameters}, the
// permission it needs, and how it travels: the handshake whose envelope
// it needs, in the dxlib dialect each path parameter's name in the body,
// and the file it streams.
func (g *gen) actionsFile() string {
	var members []member
	for _, id := range sortedKeys(anyMapActions(g.actions)) {
		a := g.actions[id]
		fields := []member{{Key: "operation", Val: str(a.operation)}, {Key: "method", Val: str(a.method)}, {Key: "path", Val: str(a.path)}, {Key: "permission", Val: str(orPublic(a.permission))}}
		fields = append(fields, a.transport...)
		members = append(members, member{Key: id, Val: object(fields)})
	}
	var b strings.Builder
	b.WriteString("/**\n * Every operation the pages call, by its operationId: its method and path\n * in the standard dialect, the permission it needs, and how it travels.\n * api.call(action, payload) reads the action here; a page's data names the\n * operation alone.\n */\n")
	b.WriteString(statement("export const actions = ", genscreens.Dictionary(members), ";"))
	return b.String()
}

func anyMapActions(m map[string]*action) map[string]any {
	out := make(map[string]any, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

// orPublic is a permission, public when the operation names none.
func orPublic(permission string) string {
	if permission == "" {
		return "public"
	}
	return permission
}
