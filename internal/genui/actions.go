package genui

import (
	"strings"
)

// operationAction reads an action that runs an operation on a list's row
// or on the record a view shows: its path filled from the record, the
// rows it is offered on, its confirmation and the reason it sends, and
// where it leads once it succeeds. own are the parameters of a view's
// route, nil on a list. ok is false, with the reason at the action, when
// the action is left out.
func (g *gen) operationAction(at string, am, props map[string]any, key string, failed map[string]any, own []string, record string) (action, bool) {
	label, target := text(am["label"]), text(am["target"])
	op, path, method := g.operation(target)
	switch {
	case op == nil:
		g.problem("error", at, "the action %s runs %s, which is not an operation of the specification", label, target)
		return action{}, false
	case method == "get":
		g.problem("error", at, "the action %s runs %s, a GET, which changes nothing; open a page with kind navigate instead", label, target)
		return action{}, false
	case !g.plainOnly(at, "the action "+label, "runs", target, op):
		return action{}, false
	}
	parts := pathParam.Split(path, -1)
	js := jsTemplate(parts[0])
	for i, p := range routeParameters(path) {
		field := ""
		switch {
		case props[p] != nil:
			field = p
		case key != "":
			field = key
		default:
			g.problem("error", at, "%s takes {%s} in its path, which is neither a field of the record nor its primary key of one field", target, p)
			return action{}, false
		}
		js += "${encodeURIComponent(" + record + "." + field + ")}" + jsTemplate(parts[i+1])
	}
	a := action{label: label, operation: target, method: strings.ToUpper(method), path: js, confirm: text(am["confirm"]), refusals: refusals(op, failed)}
	if body := g.bodySchema(op); body != nil {
		name := text(am["reason"])
		required := list(body["required"])
		prop := obj0(obj0(body["properties"])[name])
		if name == "" || prop == nil || len(required) > 1 || len(required) == 1 && text(required[0]) != name {
			g.problem("warning", at, "the action %s runs %s, which takes a request body, and an action sends only the reason its confirmation asks for; name that property under reason. It is left out", label, target)
			return action{}, false
		}
		a.reason = &reason{name: name, label: orText(text(prop["title"]), "Reason"), minLength: text(prop["minLength"]), maxLength: text(prop["maxLength"])}
	} else if text(am["reason"]) != "" {
		g.problem("error", at+"/reason", "the action %s asks for a reason, and %s takes no request body to send it in", label, target)
		return action{}, false
	}
	if w := text(am["when"]); w != "" {
		rule, ok := g.rule(w, at+"/when", "the action's when", props)
		if !ok {
			return action{}, false
		}
		a.when = rule
	}
	then := obj0(am["then"])
	a.message = text(then["message"])
	if t := text(then["navigate"]); t != "" {
		with, ok := g.withJS(at+"/then", "the action "+label, t, obj0(then["with"]), props, own, record)
		if !ok {
			return action{}, false
		}
		a.navigate = &link{label: "the action " + label, page: t, with: with, at: at + "/then/navigate"}
	}
	return a, true
}

// listLink reads a list's action that opens a page: on each row when the
// page's route takes a parameter or the action names fields under with,
// otherwise once, above the table.
func (g *gen) listLink(at string, am, props map[string]any) (link, bool, bool) {
	label, target := text(am["label"]), text(am["target"])
	if _, ok := obj(obj0(g.spec["pages"])[target]); !ok {
		g.problem("error", at, "the action %s opens %s, which is not a page of the specification", label, target)
		return link{}, false, false
	}
	if text(am["confirm"]) != "" {
		g.problem("error", at+"/confirm", "the action %s opens a page, and specarch-gen-ui asks no confirmation before a page opens; leave confirm out", label)
		return link{}, false, false
	}
	route := text(obj0(obj0(g.spec["pages"])[target])["route"])
	row := len(obj0(am["with"])) > 0 || len(routeParameters(route)) > 0
	l := link{label: label, page: target, with: "() => ({})", at: at}
	if !row {
		if text(am["when"]) != "" {
			g.problem("error", at+"/when", "the action %s opens %s once for the whole list, so there is no row for its when to test; give it with, or leave when out", label, target)
			return link{}, false, false
		}
		return l, false, true
	}
	with, ok := g.withJS(at, "the action "+label, target, obj0(am["with"]), props, nil, "row")
	if !ok {
		return link{}, false, false
	}
	l.with = with
	if w := text(am["when"]); w != "" {
		rule, ok := g.rule(w, at+"/when", "the action's when", props)
		if !ok {
			return link{}, false, false
		}
		l.when = rule
	}
	return l, true, true
}

// bodySchema is the JSON schema of an operation's request body, an entity
// or a schema it refers to read in its place; nil when it takes none.
func (g *gen) bodySchema(op map[string]any) map[string]any {
	s := obj0(obj0(obj0(obj0(op["requestBody"])["content"])["application/json"])["schema"])
	if s == nil {
		return nil
	}
	return g.resolve(s)
}

// resolve reads a schema that refers to an entity or a schema in its place.
func (g *gen) resolve(s map[string]any) map[string]any {
	if ref, ok := strings.CutPrefix(text(s["$ref"]), "#/entities/"); ok {
		return obj0(obj0(g.spec["entities"])[ref])
	}
	if ref, ok := strings.CutPrefix(text(s["$ref"]), "#/schemas/"); ok {
		return obj0(obj0(g.spec["schemas"])[ref])
	}
	return s
}

// actionJS writes an action as the page's module holds it.
func actionJS(w func(string), a action, indent, record string) {
	w(indent + "{")
	in := indent + "  "
	w(in + "label: " + jsString(a.label) + ",")
	if a.link != nil {
		w(in + "page: " + jsString(a.link.page) + ",")
		w(in + "with: " + a.link.with + ",")
		if a.when != "" {
			w(in + "when: " + a.when + ",")
		}
		w(indent + "},")
		return
	}
	u := upperSnake(a.operation)
	w(in + "operation: " + jsString(a.operation) + ",")
	w(in + "method: " + jsString(a.method) + ",")
	w(in + "path: (" + record + ") => `" + a.path + "`,")
	if a.when != "" {
		w(in + "when: " + a.when + ",")
	}
	if a.confirm != "" {
		w(in + "confirm: " + jsString(a.confirm) + ",")
	}
	if r := a.reason; r != nil {
		parts := []string{"name: " + jsString(r.name), "label: " + jsString(r.label)}
		if r.minLength != "" {
			parts = append(parts, "minLength: "+r.minLength)
		}
		if r.maxLength != "" {
			parts = append(parts, "maxLength: "+r.maxLength)
		}
		w(in + "reason: { " + strings.Join(parts, ", ") + " },")
	}
	w(in + "message: " + jsString(a.message) + ",")
	if n := a.navigate; n != nil {
		w(in + "navigate: { page: " + jsString(n.page) + ", with: " + n.with + " },")
	}
	w(in + "succeeded: " + u + "_SUCCEEDED,")
	w(in + "failed: " + u + "_FAILED,")
	w(indent + "},")
}
