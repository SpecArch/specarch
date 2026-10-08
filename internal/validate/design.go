package validate

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// design indexes the named objects of a specification.
type design struct {
	root         *yaml.Node
	spec         *spec.Spec // the specification on disk, when known
	entities     map[string]*yaml.Node
	views        map[string]*yaml.Node
	enums        map[string]*yaml.Node
	permissions  map[string]*yaml.Node
	roles        map[string]*yaml.Node
	commands     map[string]*yaml.Node
	channels     map[string]*yaml.Node
	dependencies map[string]*yaml.Node
	pages        map[string]*yaml.Node
	algorithms   map[string]*yaml.Node
	decisions    map[string]*yaml.Node
	sources      map[string]*yaml.Node
	stakeholders map[string]*yaml.Node
	needs        map[string]*yaml.Node
	requirements map[string]*yaml.Node
	environments map[string]*yaml.Node
	settings     map[string]*yaml.Node
	checks       map[string]*yaml.Node
	monitors     map[string]*yaml.Node
	questions    map[string]*yaml.Node
	operations   map[string]operation // by operationId, the first definition
	opList       []operation          // every operation in document order
}

type operation struct {
	id       string
	node     *yaml.Node
	path     string // the path template
	method   string
	pathItem *yaml.Node
}

var methods = []string{"get", "post", "put", "patch", "delete"}

func topMap(root *yaml.Node, key string) map[string]*yaml.Node {
	m := map[string]*yaml.Node{}
	for _, p := range source.Pairs(source.Child(root, key)) {
		m[p.Key.Value] = p.Value
	}
	return m
}

func newDesign(root *yaml.Node) *design {
	d := &design{
		root:         root,
		entities:     topMap(root, "entities"),
		views:        topMap(root, "views"),
		enums:        topMap(root, "enums"),
		permissions:  topMap(root, "permissions"),
		roles:        topMap(root, "roles"),
		commands:     topMap(root, "commands"),
		channels:     topMap(root, "channels"),
		dependencies: topMap(root, "dependencies"),
		pages:        topMap(root, "pages"),
		algorithms:   topMap(root, "algorithms"),
		decisions:    topMap(root, "decisions"),
		sources:      topMap(root, "sources"),
		stakeholders: topMap(root, "stakeholders"),
		needs:        topMap(root, "needs"),
		requirements: topMap(root, "requirements"),
		environments: topMap(root, "environments"),
		settings:     topMap(root, "configuration"),
		checks:       topMap(root, "checks"),
		monitors:     topMap(root, "monitors"),
		questions:    topMap(root, "questions"),
		operations:   map[string]operation{},
	}
	for _, p := range source.Pairs(source.Child(root, "paths")) {
		for _, m := range methods {
			op := source.Child(p.Value, m)
			if op == nil {
				continue
			}
			o := operation{id: source.Str(source.Child(op, "operationId")), node: op, path: p.Key.Value, method: m, pathItem: p.Value}
			d.opList = append(d.opList, o)
			if _, dup := d.operations[o.id]; !dup && o.id != "" {
				d.operations[o.id] = o
			}
		}
	}
	return d
}

func (o operation) pointer(tokens ...string) string {
	return source.Pointer(append([]string{"paths", o.path, o.method}, tokens...)...)
}

// message returns a channel's message node for "channel/Message", or nil.
func (d *design) message(ref string) *yaml.Node {
	ch, msg, ok := strings.Cut(ref, "/")
	if !ok {
		return nil
	}
	return source.Child(source.Child(d.channels[ch], "messages"), msg)
}

// fields returns an entity's property names, in order.
func fieldsOf(entity *yaml.Node) map[string]*yaml.Node {
	m := map[string]*yaml.Node{}
	for _, p := range source.Pairs(source.Child(entity, "properties")) {
		m[p.Key.Value] = p.Value
	}
	return m
}

// suggest returns "; did you mean X?" for a close name, or a list of the
// valid names when there are few, or "".
func suggest(name string, valid map[string]*yaml.Node) string {
	names := make([]string, 0, len(valid))
	for n := range valid {
		names = append(names, n)
	}
	sort.Strings(names)
	best, bestDist := "", 3
	for _, n := range names {
		if dist := distance(strings.ToLower(name), strings.ToLower(n)); dist < bestDist {
			best, bestDist = n, dist
		}
	}
	switch {
	case best != "":
		return "; did you mean " + best + "?"
	case len(names) == 0:
		return "; there are none in the specification"
	case len(names) <= 8:
		return "; use one of " + strings.Join(names, ", ")
	}
	return ""
}

func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}

func (c *checker) checkDesign(d *design) {
	c.checkRefs(d)
	c.checkIntegers()
	c.checkRequirementLinks(d.root, nil, d)
	c.checkCitations(d.root, nil, d)
	c.checkRequirementsStage(d)
	c.checkEnums(d)
	c.checkEntities(d)
	c.checkLookups(d)
	c.checkOperations(d)
	c.checkCommands(d)
	c.checkDependencies(d)
	c.checkJobs(d)
	c.checkMenus(d)
	c.checkViews(d)
	c.checkSession(d)
	c.checkPages(d)
	c.checkPageEvents(d)
	c.checkFlows(d)
	c.checkPageStates(d)
	c.checkCompactColumns(d)
	c.checkAccessibility(d)
	c.checkTheme(d)
	c.checkSections(d)
	c.checkChildRows(d)
	c.checkDecisions(d)
	c.checkAccess(d)
	c.checkSeparationOfDuties(d)
	c.checkExpressions(d)
	c.checkTests(d)
	c.checkTestData(d)
	c.checkDeploymentStage(d)
	c.checkTraceability(d)
	c.checkQuestions(d)
	c.checkOrigin(d.root, d.decisions, func(path []string) bool { return len(path) == 2 && path[0] == "decisions" })
	c.checkOriginTracked(d)
}

// checkRefs finds every $ref in the file and checks its target exists.
func (c *checker) checkRefs(d *design) {
	walk(d.root, nil, func(n *yaml.Node, path []string) {
		for _, p := range source.Pairs(n) {
			if p.Key.Value != "$ref" || !source.IsScalar(p.Value) {
				continue
			}
			ref := p.Value.Value
			ptr := source.Pointer(append(path, "$ref")...)
			switch {
			case strings.HasPrefix(ref, "#/entities/"):
				name := strings.TrimPrefix(ref, "#/entities/")
				if d.entities[name] == nil {
					c.add(p.Value, ptr, RuleRefType, "%s is not an entity of the specification%s", name, suggest(name, d.entities))
				}
			case strings.HasPrefix(ref, "#/views/"):
				name := strings.TrimPrefix(ref, "#/views/")
				if d.views[name] == nil {
					c.add(p.Value, ptr, RuleRefType, "%s is not a view of the specification%s", name, suggest(name, d.views))
				}
			case strings.HasPrefix(ref, "#/enums/"):
				name := strings.TrimPrefix(ref, "#/enums/")
				if d.enums[name] == nil {
					c.add(p.Value, ptr, RuleRefType, "%s is not an enum of the specification%s", name, suggest(name, d.enums))
				}
			}
		}
	})
}

// walk calls fn on every mapping in the tree, with its path.
func walk(n *yaml.Node, path []string, fn func(*yaml.Node, []string)) {
	n = source.Deref(n)
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.MappingNode:
		fn(n, path)
		for _, p := range source.Pairs(n) {
			walk(p.Value, append(append([]string{}, path...), p.Key.Value), fn)
		}
	case yaml.SequenceNode:
		for i, item := range source.Items(n) {
			walk(item, append(append([]string{}, path...), fmt.Sprint(i)), fn)
		}
	}
}

// requirementSetPrefixes lists the prefixes of the external requirement
// sets declared under sources.
func (d *design) requirementSetPrefixes() map[string]*yaml.Node {
	out := map[string]*yaml.Node{}
	for name, src := range d.sources {
		if source.Str(source.Child(src, "kind")) == "requirement-set" {
			if prefix := source.Str(source.Child(src, "prefix")); prefix != "" {
				out[prefix] = d.sources[name]
			}
		}
	}
	return out
}

// checkRequirementLinks checks every satisfies and verifies list names a
// requirement of the specification, or one of an external set declared
// under sources.
func (c *checker) checkRequirementLinks(root *yaml.Node, base []string, d *design) {
	prefixes := d.requirementSetPrefixes()
	walk(root, base, func(n *yaml.Node, path []string) {
		for _, key := range []string{"satisfies", "verifies"} {
			list := source.Child(n, key)
			for i, item := range source.Items(list) {
				link := source.Str(item)
				if d.requirements[link] != nil {
					continue
				}
				prefix, _, ok := strings.Cut(link, "-")
				if ok && prefixes[prefix] != nil {
					continue
				}
				ptr := source.Pointer(append(path, key, fmt.Sprint(i))...)
				switch {
				case len(d.requirements) == 0 && len(prefixes) == 0:
					c.add(item, ptr, RuleRequirement, "%s is not a requirement: the specification has none under requirements and no source of kind requirement-set; add the requirement, or declare the set under sources with prefix %s", link, prefix)
				case len(d.requirements) > 0:
					c.add(item, ptr, RuleRequirement, "%s is not a requirement of the specification%s", link, suggest(link, d.requirements))
				default:
					c.add(item, ptr, RuleRequirement, "%s is not in a declared requirement set; the sets under sources have the prefixes %s", link, strings.Join(sortedKeys(prefixes), ", "))
				}
			}
		}
	})
}

// checkCitations checks every citation names a declared source.
func (c *checker) checkCitations(root *yaml.Node, base []string, d *design) {
	walk(root, base, func(n *yaml.Node, path []string) {
		for i, item := range source.Items(source.Child(n, "cites")) {
			srcNode := source.Child(item, "source")
			name := source.Str(srcNode)
			if name == "" || d.sources[name] != nil {
				continue
			}
			ptr := source.Pointer(append(path, "cites", fmt.Sprint(i), "source")...)
			if len(d.sources) == 0 {
				c.add(srcNode, ptr, RuleSource, "%s is not a declared source: the specification declares none; add it under sources in %s", name, spec.RootFile)
				continue
			}
			c.add(srcNode, ptr, RuleSource, "%s is not a source declared under sources%s", name, suggest(name, d.sources))
		}
	})
}

func (c *checker) checkEnums(d *design) {
	for name, e := range d.enums {
		values := map[string]*yaml.Node{}
		for _, v := range source.Items(source.Child(e, "enum")) {
			values[v.Value] = v
		}
		for _, p := range source.Pairs(source.Child(e, "valueDescriptions")) {
			if values[p.Key.Value] == nil {
				c.add(p.Key, source.Pointer("enums", name, "valueDescriptions", p.Key.Value), RuleEnumValue,
					"%s is not a value of enum %s; describe only its values%s", p.Key.Value, name, suggest(p.Key.Value, values))
			}
		}
	}
}

func (c *checker) checkFieldList(list *yaml.Node, ptr []string, fields map[string]*yaml.Node, entity, what string) {
	for i, item := range source.Items(list) {
		name := source.Str(item)
		if name != "" && fields[name] == nil {
			c.add(item, source.Pointer(append(ptr, fmt.Sprint(i))...), RuleField,
				"%s is not a field of %s, so it cannot be %s%s", name, entity, what, suggest(name, fields))
		}
	}
}

func (c *checker) checkEntities(d *design) {
	for name, e := range d.entities {
		fields := fieldsOf(e)
		base := []string{"entities", name}
		c.checkFieldList(source.Child(e, "required"), append(base, "required"), fields, name, "required")
		c.checkFieldList(source.Child(e, "primaryKey"), append(base, "primaryKey"), fields, name, "part of the primary key")
		for _, p := range source.Pairs(source.Child(e, "constraints")) {
			if source.Str(source.Child(p.Value, "kind")) == "unique" {
				c.checkFieldList(source.Child(p.Value, "fields"), append(base, "constraints", p.Key.Value, "fields"), fields, name, "part of a unique constraint")
			}
		}
		for _, p := range source.Pairs(source.Child(e, "relations")) {
			c.checkRelation(d, name, fields, p)
		}
		c.checkStates(d, name, e, fields)
		c.checkValidity(name, e, fields)
		c.checkStored(name, e, fields)
	}
}

func (c *checker) checkRelation(d *design, entity string, fields map[string]*yaml.Node, p source.Pair) {
	base := []string{"entities", entity, "relations", p.Key.Value}
	targetNode := source.Child(p.Value, "target")
	target := source.Str(targetNode)
	if target != "" && d.entities[target] == nil {
		c.add(targetNode, source.Pointer(append(base, "target")...), RuleRelationTarget,
			"%s is not an entity of the specification%s", target, suggest(target, d.entities))
		return
	}
	viaNode := source.Child(p.Value, "via")
	via := source.Str(viaNode)
	if via == "" || target == "" {
		return
	}
	ptr := source.Pointer(append(base, "via")...)
	switch source.Str(source.Child(p.Value, "kind")) {
	case "many-to-one", "one-to-one":
		if fields[via] == nil {
			c.add(viaNode, ptr, RuleRelationVia, "%s is not a field of %s; for this kind of relation, via names the field on %s that holds the key%s", via, entity, entity, suggest(via, fields))
		}
	case "one-to-many":
		targetFields := fieldsOf(d.entities[target])
		if targetFields[via] == nil {
			c.add(viaNode, ptr, RuleRelationVia, "%s is not a field of %s; for one-to-many, via names the field on the target that points back%s", via, target, suggest(via, targetFields))
		}
	case "many-to-many":
		if d.entities[via] == nil {
			c.add(viaNode, ptr, RuleRelationVia, "%s is not an entity of the specification; for many-to-many, via names the join entity%s", via, suggest(via, d.entities))
		}
	}
}

// enumValues returns the values a field may take when it is an enum, by
// $ref or inline, and whether it is one.
func (d *design) enumValues(field *yaml.Node) (map[string]*yaml.Node, string, bool) {
	if ref := source.Str(source.Child(field, "$ref")); strings.HasPrefix(ref, "#/enums/") {
		name := strings.TrimPrefix(ref, "#/enums/")
		e := d.enums[name]
		if e == nil {
			return nil, name, false
		}
		values := map[string]*yaml.Node{}
		for _, v := range source.Items(source.Child(e, "enum")) {
			values[v.Value] = v
		}
		return values, name, true
	}
	if list := source.Child(field, "enum"); list != nil {
		values := map[string]*yaml.Node{}
		for _, v := range source.Items(list) {
			values[v.Value] = v
		}
		return values, "", true
	}
	return nil, "", false
}

func (c *checker) checkStates(d *design, name string, e *yaml.Node, fields map[string]*yaml.Node) {
	sfNode := source.Child(e, "stateField")
	sf := source.Str(sfNode)
	if sf == "" {
		return
	}
	base := []string{"entities", name}
	field := fields[sf]
	if field == nil {
		c.add(sfNode, source.Pointer(append(base, "stateField")...), RuleStateField, "%s is not a field of %s%s", sf, name, suggest(sf, fields))
		return
	}
	values, _, ok := d.enumValues(field)
	if !ok {
		if strings.HasPrefix(source.Str(source.Child(field, "$ref")), "#/enums/") {
			return // the dangling $ref is reported as ref_type
		}
		c.add(sfNode, source.Pointer(append(base, "stateField")...), RuleStateField, "%s is not an enum field, so its values cannot be states; give it a $ref to an enum", sf)
		return
	}
	for i, t := range source.Items(source.Child(e, "transitions")) {
		tp := append(base, "transitions", fmt.Sprint(i))
		for _, end := range []string{"from", "to"} {
			n := source.Child(t, end)
			v := source.Str(n)
			if v != "" && values[v] == nil {
				c.add(n, source.Pointer(append(tp, end)...), RuleStateValue, "%s is not a value of %s%s", v, sf, suggest(v, values))
			}
		}
		trig := source.Child(t, "trigger")
		if tr := source.Str(trig); tr != "" && !d.isTrigger(tr) {
			c.add(trig, source.Pointer(append(tp, "trigger")...), RuleTrigger,
				"%s is not an operationId, a command, a channel/Message or an algorithm of the specification; name the one that causes this move", tr)
		}
	}
}

func (d *design) isTrigger(t string) bool {
	if _, ok := d.operations[t]; ok {
		return true
	}
	return d.commands[t] != nil || d.algorithms[t] != nil || d.message(t) != nil
}

var pathParam = regexp.MustCompile(`\{([^{}]+)\}`)

func (c *checker) checkOperations(d *design) {
	seen := map[string]bool{}
	for _, o := range d.opList {
		if o.id != "" {
			if seen[o.id] {
				first := d.operations[o.id]
				c.add(source.Child(o.node, "operationId"), o.pointer("operationId"), RuleDuplicateOperation,
					"operationId %s is already used by %s %s; give this operation its own name", o.id, strings.ToUpper(first.method), first.path)
			}
			seen[o.id] = true
		}
		if alg := source.Child(o.node, "algorithm"); alg != nil && d.algorithms[alg.Value] == nil {
			c.add(alg, o.pointer("algorithm"), RuleAlgorithm, "%s is not an algorithm of the specification%s", alg.Value, suggest(alg.Value, d.algorithms))
		}
		for i, e := range source.Items(source.Child(o.node, "emits")) {
			if d.message(e.Value) == nil {
				c.add(e, o.pointer("emits", fmt.Sprint(i)), RuleEmits, "%s does not name a channel and one of its messages; write channel/Message for a message declared under channels", e.Value)
			}
		}
		c.checkPathParameters(o)
		c.checkCalls(d, o)
		c.checkIdempotencyKey(o)
		c.checkExposed(d, o)
		c.checkListOf(d, o)
		c.checkLimits(o)
		c.checkProblems(d, o)
		c.checkGuard(d, source.Child(o.node, "guard"), o.pointer("guard"))
	}
}

func (c *checker) checkPathParameters(o operation) {
	inTemplate := map[string]bool{}
	for _, m := range pathParam.FindAllStringSubmatch(o.path, -1) {
		inTemplate[m[1]] = true
	}
	declared := map[string]bool{}
	check := func(list *yaml.Node, base []string) {
		for i, p := range source.Items(list) {
			if source.Str(source.Child(p, "in")) != "path" {
				continue
			}
			name := source.Str(source.Child(p, "name"))
			declared[name] = true
			if !inTemplate[name] {
				c.add(source.Child(p, "name"), source.Pointer(append(base, fmt.Sprint(i), "name")...), RulePathParameter,
					"path parameter %s does not appear in %s; add {%s} to the path or remove the parameter", name, o.path, name)
			}
		}
	}
	check(source.Child(o.pathItem, "parameters"), []string{"paths", o.path, "parameters"})
	check(source.Child(o.node, "parameters"), []string{"paths", o.path, o.method, "parameters"})
	names := make([]string, 0, len(inTemplate))
	for n := range inTemplate {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if !declared[n] {
			c.add(o.node, o.pointer(), RulePathParameter, "{%s} in %s has no path parameter; declare it with in: path and required: true", n, o.path)
		}
	}
}

func (c *checker) checkCommands(d *design) {
	for name, cmd := range d.commands {
		if alg := source.Child(cmd, "algorithm"); alg != nil && d.algorithms[alg.Value] == nil {
			c.add(alg, source.Pointer("commands", name, "algorithm"), RuleAlgorithm, "%s is not an algorithm of the specification%s", alg.Value, suggest(alg.Value, d.algorithms))
		}
		args := source.Items(source.Child(cmd, "arguments"))
		for i, a := range args {
			if source.Str(source.Child(a, "repeatable")) == "true" && i != len(args)-1 {
				c.add(source.Child(a, "repeatable"), source.Pointer("commands", name, "arguments", fmt.Sprint(i), "repeatable"), RuleSchema,
					"only the last argument may be repeatable; move this argument to the end or make it a single value")
			}
		}
		c.checkGuard(d, source.Child(cmd, "guard"), source.Pointer("commands", name, "guard"))
	}
}

func (c *checker) checkPages(d *design) {
	opNames := map[string]*yaml.Node{}
	for id, o := range d.operations {
		opNames[id] = o.node
	}
	for name, pg := range d.pages {
		base := []string{"pages", name}
		entNode := source.Child(pg, "entity")
		ent := source.Str(entNode)
		if source.Str(source.Child(pg, "kind")) == "task" {
			c.checkTaskPage(d, name, pg)
		} else if ent != "" && d.entities[ent] == nil {
			c.add(entNode, source.Pointer(append(base, "entity")...), RuleRefType, "%s is not an entity of the specification%s", ent, suggest(ent, d.entities))
		} else if ent != "" {
			fields := fieldsOf(d.entities[ent])
			c.checkFieldList(source.Child(pg, "columns"), append(base, "columns"), fields, ent, "a column")
			c.checkFieldList(source.Child(pg, "fields"), append(base, "fields"), fields, ent, "shown on this page")
			for i, sec := range source.Items(source.Child(pg, "sections")) {
				c.checkFieldList(source.Child(sec, "fields"), append(base, "sections", fmt.Sprint(i), "fields"), fields, ent, "shown on this page")
			}
			c.checkFieldList(source.Child(pg, "filters"), append(base, "filters"), fields, ent, "a filter")
		}
		for _, key := range []string{"source", "submit"} {
			n := source.Child(pg, key)
			if id := source.Str(n); id != "" && d.operations[id].node == nil {
				c.add(n, source.Pointer(append(base, key)...), RuleOperation, "%s is not an operationId of the specification%s", id, suggest(id, opNames))
			}
		}
		for i, a := range source.Items(source.Child(pg, "actions")) {
			tn := source.Child(a, "target")
			t := source.Str(tn)
			ptr := source.Pointer(append(base, "actions", fmt.Sprint(i), "target")...)
			switch source.Str(source.Child(a, "kind")) {
			case "navigate":
				if t != "" && d.pages[t] == nil {
					c.add(tn, ptr, RulePage, "%s is not a page of the specification%s", t, suggest(t, d.pages))
				}
			case "operation":
				if t != "" && d.operations[t].node == nil {
					c.add(tn, ptr, RuleOperation, "%s is not an operationId of the specification%s", t, suggest(t, opNames))
				}
			}
		}
	}
}

func (c *checker) checkDecisions(d *design) {
	for id, dec := range d.decisions {
		if s := source.Child(dec, "supersededBy"); s != nil && d.decisions[s.Value] == nil {
			c.add(s, source.Pointer("decisions", id, "supersededBy"), RuleDecision, "%s is not a decision of the specification%s", s.Value, suggest(s.Value, d.decisions))
		}
	}
}

// checkAccess is the fail-closed rule: every permission used is declared,
// and every declared permission is granted by a role or is public.
func (c *checker) checkAccess(d *design) {
	use := func(n *yaml.Node, ptr string, who string) {
		p := source.Str(n)
		if p != "" && d.permissions[p] == nil {
			c.add(n, ptr, RulePermissionUndeclared, "%s needs permission %s, which is not declared; add it under permissions%s", who, p, strings.Replace(suggest(p, d.permissions), "; there are none in the specification", "", 1))
		}
	}
	for _, o := range d.opList {
		use(source.Child(o.node, "permission"), o.pointer("permission"), "operation "+o.id)
	}
	for name, cmd := range d.commands {
		use(source.Child(cmd, "permission"), source.Pointer("commands", name, "permission"), "command "+name)
	}
	for name, pg := range d.pages {
		use(source.Child(pg, "permission"), source.Pointer("pages", name, "permission"), "page "+name)
		for i, a := range source.Items(source.Child(pg, "actions")) {
			use(source.Child(a, "permission"), source.Pointer("pages", name, "actions", fmt.Sprint(i), "permission"), "an action of page "+name)
		}
	}
	granted := map[string]bool{}
	for name, r := range d.roles {
		for i, p := range source.Items(source.Child(r, "permissions")) {
			granted[p.Value] = true
			use(p, source.Pointer("roles", name, "permissions", fmt.Sprint(i)), "role "+name)
		}
	}
	for _, p := range source.Pairs(source.Child(d.root, "permissions")) {
		if p.Key.Value != "public" && !granted[p.Key.Value] {
			c.add(p.Key, source.Pointer("permissions", p.Key.Value), RulePermissionUngranted,
				"no role grants %s, so nobody can use what it guards; add it to a role, or use public if it is meant for everyone", p.Key.Value)
		}
	}
}

// stackSpecificKey in the design schema, the same pattern.
var stackKeyPattern = regexp.MustCompile(`^x-(oapi-codegen|ogen|openapi-generator|codegen|protoc|grpc|go|java|kotlin|python|typescript|javascript|rust|swift|dotnet|csharp|php|ruby|framework|router|middleware|cli-library|server|servers|host|port|deploy|deployment|environment)(-|$)`)

func (c *checker) checkBoundaryDesign() {
	walk(c.root, nil, func(n *yaml.Node, path []string) {
		for _, p := range source.Pairs(n) {
			if stackKeyPattern.MatchString(p.Key.Value) {
				c.add(p.Key, source.Pointer(append(path, p.Key.Value)...), RuleStackKey,
					"%s belongs to one implementation, not to the design; move it to the implementation file (%s), under mappings or generators", p.Key.Value, "*"+ImplementationSuffix)
			}
		}
	})
}
