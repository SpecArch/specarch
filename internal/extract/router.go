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

// The route table a running router prints, in the format
// tools/routes/dump-routes.sh wraps: the version of the format, the path
// and the commit the router was built from, and every registered route.
type routeTable struct {
	RouteTable int         `json:"routeTable"`
	Path       string      `json:"path"`
	Commit     string      `json:"commit"`
	Routes     []dumpRoute `json:"routes"`
}

type dumpRoute struct {
	Method     string          `json:"method"`
	Path       string          `json:"path"`
	Permission json.RawMessage `json:"permission"`
	Handler    string          `json:"handler"`
}

// The methods an operation can be written under, as the design schema's
// path item lists them.
var heldMethods = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}

var (
	routeMethod    = regexp.MustCompile(`^[A-Z]+$`)
	routeSegment   = regexp.MustCompile(`^\{([A-Za-z_][A-Za-z0-9_]*)\}$`)
	plainSegment   = regexp.MustCompile(`^[A-Za-z0-9._~!$&'()+,;=:@-]+$`)
	permissionWord = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$`)
	handlerName    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)
	notWord        = regexp.MustCompile(`[^A-Za-z0-9]+`)
)

// route is one route the reader writes as an operation.
type route struct {
	dumpRoute
	key        string   // the path item's method key: get, post...
	params     []string // the path's parameters, in order
	permission string   // "" when the route checks none, or none the meta-model can name
	printed    *string  // the permission as the route table names it
	id         string   // the operationId
}

// Router reads a route table into operations, one per method and path
// pair, with their path parameters and the permission each checks.
func Router(dumpPath, out, key string) (*Result, error) {
	data, err := os.ReadFile(dumpPath)
	if err != nil {
		return nil, err
	}
	var t routeTable
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return nil, refuse("%s is not a route table: %v", dumpPath, err)
	}
	if t.RouteTable != 1 {
		return nil, refuse("%s is not a route table in the format tools/routes/dump-routes.sh writes: its routeTable is %d, not 1", dumpPath, t.RouteTable)
	}
	r, dumpName, err := openDump(dumpPath, t.Path, t.Commit, "tools/routes/dump-routes.sh")
	if err != nil {
		return nil, err
	}
	res := &Result{Tree: newTree()}
	res.say("commit %s: the last change to %s, as the route table %s names it", r.Commit, t.Path, dumpName)
	rd := &routeReader{t: &t, key: key, res: res, questions: &yaml.Node{Kind: yaml.MappingNode}}
	if err := rd.read(dumpName); err != nil {
		return nil, err
	}
	src, err := codeSource(r, out, []*yaml.Node{flow(mapping("clause", t.Path, "title", "The routes the router built from here registers"))})
	if err != nil {
		return nil, err
	}
	var stages []string
	if len(rd.routes) > 0 {
		stages = []string{"design"}
		design := mapping()
		if rd.permissions != nil {
			set(design, "permissions", rd.permissions)
		}
		set(design, "paths", rd.paths)
		res.Tree.put("design/paths.yaml", design)
	}
	if len(rd.questions.Content) > 0 {
		stages = []string{"requirements", "design"}
		res.Tree.put("requirements/stakeholders.yaml", mapping("stakeholders", ownerStakeholder()))
		res.Tree.put("design/questions.yaml", mapping("questions", rd.questions))
	}
	description := fmt.Sprintf("The routes the router built from %s registers, read from the route table %s, made at commit %s. Every operation cites its route; what the route table does not say is a question.\n", t.Path, dumpName, r.Commit)
	res.Tree.put("specarch.yaml", rootFile("Routes of "+t.Path, description, stages, mapping(key, src)))
	return res, nil
}

type routeReader struct {
	t           *routeTable
	key         string
	res         *Result
	routes      []*route
	paths       *yaml.Node
	permissions *yaml.Node
	questions   *yaml.Node
	nextID      int
	notHeld     []string
}

func (rd *routeReader) gap(format string, args ...any) {
	rd.notHeld = append(rd.notHeld, "not held: "+fmt.Sprintf(format, args...))
}

func (rd *routeReader) question(text string, blocks []string, why string) {
	rd.nextID++
	set(rd.questions, fmt.Sprintf("Q-%d", rd.nextID), mapping(
		"question", text,
		"kind", "decision",
		"priority", "must",
		"blocks", blocks,
		"decidedBy", owner,
		"why", why,
	))
}

func (rd *routeReader) clause(rt *route) string {
	return rd.t.Path + " " + rt.Method + " " + rt.Path
}

// read checks every route, then writes the held ones in the order of
// their paths and methods, so the router's own order does not show.
func (rd *routeReader) read(dumpName string) error {
	seen := map[string]bool{}
	byPath := map[string][]*route{}
	var pathOrder []string
	unchecked := 0
	for i, d := range rd.t.Routes {
		where := fmt.Sprintf("%s, route %d", dumpName, i+1)
		if !routeMethod.MatchString(d.Method) {
			return refuse("%s: the method %q is not an HTTP method in capitals", where, d.Method)
		}
		if !strings.HasPrefix(d.Path, "/") {
			return refuse("%s: the path %q does not start with /", where, d.Path)
		}
		if len(d.Permission) == 0 {
			return refuse("%s: %s %s names no permission; write null for a route that checks none", where, d.Method, d.Path)
		}
		if d.Handler == "" {
			return refuse("%s: %s %s names no handler", where, d.Method, d.Path)
		}
		pair := d.Method + " " + d.Path
		if seen[pair] {
			return refuse("%s: %s is listed twice; a router registers a method and path pair once", where, pair)
		}
		seen[pair] = true
		rt := &route{dumpRoute: d}
		var permission *string
		if err := json.Unmarshal(d.Permission, &permission); err != nil {
			return refuse("%s: the permission of %s is neither a name nor null", where, pair)
		}
		rt.printed = permission
		if permission != nil {
			rt.permission = *permission
		} else {
			unchecked++
		}
		if !heldMethods[d.Method] {
			rd.gap("route %s: the method %s has no operation in the meta-model, which holds GET, POST, PUT, PATCH and DELETE; left out", pair, d.Method)
			continue
		}
		params, reason := pathParameters(d.Path)
		if reason != "" {
			rd.gap("route %s: %s; left out", pair, reason)
			continue
		}
		rt.params = params
		rt.key = strings.ToLower(d.Method)
		if permission != nil && !permissionWord.MatchString(rt.permission) {
			rd.gap("route %s: its permission %q is not a permission name, which is lower-case words joined by dots; asked for instead", pair, rt.permission)
			rt.permission = ""
		}
		if byPath[d.Path] == nil {
			pathOrder = append(pathOrder, d.Path)
		}
		byPath[d.Path] = append(byPath[d.Path], rt)
		rd.routes = append(rd.routes, rt)
	}
	rd.res.say("counted %s (%d with no permission): every entry of the route table's list, which holds every route the router registered", plural(len(rd.t.Routes), "route"), unchecked)
	rd.operationIDs()
	sort.Strings(pathOrder)
	rd.paths = &yaml.Node{Kind: yaml.MappingNode}
	usedBy := map[string][]*route{}
	for _, p := range pathOrder {
		routes := byPath[p]
		sort.Slice(routes, func(i, j int) bool { return methodRank(routes[i].key) < methodRank(routes[j].key) })
		item := mapping()
		at := "#/paths/" + escapeToken(p)
		if params := routes[0].params; len(params) > 0 {
			var list []*yaml.Node
			var blocks []string
			for i, name := range params {
				list = append(list, flow(mapping("name", name, "in", "path", "required", true)))
				blocks = append(blocks, fmt.Sprintf("%s/parameters/%d/schema", at, i))
			}
			set(item, "parameters", list)
			rd.question(
				fmt.Sprintf("What values does each path parameter of %s take: %s?", p, strings.Join(params, ", ")),
				blocks,
				"A route table names a path's parameters and not the values they take.",
			)
		}
		for _, rt := range routes {
			op := mapping("operationId", rt.id)
			if rt.permission != "" {
				set(op, "permission", rt.permission)
				usedBy[rt.permission] = append(usedBy[rt.permission], rt)
			}
			checks := "checks no permission"
			if rt.printed != nil {
				checks = "checks " + *rt.printed
			}
			set(op, "origin", "stated")
			set(op, "cites", []*yaml.Node{citation(rd.key, rd.clause(rt), fmt.Sprintf("%s %s %s; its handler is %s.", rt.Method, rt.Path, checks, rt.Handler))})
			set(item, rt.key, op)
			opAt := at + "/" + rt.key
			rd.question(
				fmt.Sprintf("What does %s %s do, and what does it answer?", rt.Method, rt.Path),
				[]string{opAt + "/summary", opAt + "/responses"},
				"A route table names the handler and not what it does or the responses it gives.",
			)
			if rt.permission == "" {
				asked := fmt.Sprintf("%s %s checks no permission that the route table names.", rt.Method, rt.Path)
				if rt.printed != nil {
					asked = fmt.Sprintf("%s %s checks %s, which is not a permission name.", rt.Method, rt.Path, *rt.printed)
				}
				rd.question(
					asked+" Is it meant to be open to everyone (public), or which permission should it check?",
					[]string{opAt + "/permission"},
					"An operation open to everyone is how an open endpoint is usually found, so it is asked, never assumed.",
				)
			}
		}
		set(rd.paths, p, item)
	}
	if len(usedBy) > 0 {
		var names []string
		for n := range usedBy {
			names = append(names, n)
		}
		sort.Strings(names)
		rd.permissions = &yaml.Node{Kind: yaml.MappingNode}
		var blocks []string
		for _, n := range names {
			var says []string
			for _, rt := range usedBy[n] {
				says = append(says, rt.Method+" "+rt.Path)
			}
			first := usedBy[n][0]
			set(rd.permissions, n, mapping(
				"origin", "stated",
				"cites", []*yaml.Node{citation(rd.key, rd.clause(first), fmt.Sprintf("%s %s %s.", joinAnd(says), checkOrChecks(len(says)), n))},
			))
			blocks = append(blocks, "#/permissions/"+escapeToken(n)+"/description")
		}
		rd.question(
			fmt.Sprintf("What does each permission allow: %s?", strings.Join(names, ", ")),
			blocks,
			"A route table names the permission a route checks and not what it is for.",
		)
	}
	permissions := 0
	if rd.permissions != nil {
		permissions = len(rd.permissions.Content) / 2
	}
	rd.res.say("wrote %s on %s, %s and %s: one operation per method and path pair the meta-model holds, one permission per name a route checks, and one question per thing the route table does not say", plural(len(rd.routes), "operation"), plural(len(pathOrder), "path"), plural(permissions, "permission"), plural(rd.nextID, "question"))
	rd.res.Lines = append(rd.res.Lines, rd.notHeld...)
	return nil
}

// operationIDs names each operation after its handler, with the first
// letter in lower case; a handler that serves several routes, or a name
// that cannot be an operationId, gets the method and the path's words.
func (rd *routeReader) operationIDs() {
	count := map[string]int{}
	for _, rt := range rd.routes {
		if handlerName.MatchString(rt.Handler) {
			count[rt.Handler]++
		}
	}
	for _, rt := range rd.routes {
		if handlerName.MatchString(rt.Handler) && count[rt.Handler] == 1 {
			rt.id = strings.ToLower(rt.Handler[:1]) + rt.Handler[1:]
			continue
		}
		id := rt.key
		for _, seg := range strings.Split(rt.Path, "/") {
			if m := routeSegment.FindStringSubmatch(seg); m != nil {
				seg = "by_" + m[1]
			}
			id += pascal(notWord.ReplaceAllString(seg, "_"))
		}
		why := "serves more than one route"
		if !handlerName.MatchString(rt.Handler) {
			why = "is not a name an operationId can take"
		}
		rd.gap("route %s %s: its handler %s %s; the operation is named %s", rt.Method, rt.Path, rt.Handler, why, id)
		rt.id = id
	}
}

// pathParameters lists a route path's parameters, written {name} as the
// route table's format asks, or says why the meta-model cannot hold it.
func pathParameters(p string) ([]string, string) {
	var params []string
	seen := map[string]bool{}
	for i, seg := range strings.Split(p, "/") {
		if i == 0 || (seg == "" && i == len(strings.Split(p, "/"))-1) {
			continue
		}
		if m := routeSegment.FindStringSubmatch(seg); m != nil {
			if seen[m[1]] {
				return nil, fmt.Sprintf("the parameter %s appears twice", m[1])
			}
			seen[m[1]] = true
			params = append(params, m[1])
			continue
		}
		if strings.HasPrefix(seg, ":") {
			return nil, fmt.Sprintf("the segment %q looks like a parameter in the router's own syntax; the route table writes a parameter {name}", seg)
		}
		if !plainSegment.MatchString(seg) {
			return nil, fmt.Sprintf("the segment %q is neither a fixed word nor one whole parameter written {name}, and an OpenAPI path template holds only those", seg)
		}
	}
	return params, ""
}

func methodRank(key string) int {
	return strings.Index("get post put patch delete", key)
}

// escapeToken escapes a key for a JSON pointer (RFC 6901).
func escapeToken(s string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(s)
}

func joinAnd(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

func checkOrChecks(n int) string {
	if n == 1 {
		return "checks"
	}
	return "check"
}
