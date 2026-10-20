package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The gateway table a project's printer writes of the table of services
// its gateway loads, in the format tools/gateways/dump-gateway.sh wraps
// (ADR-142): the version of the format, the path and the commit the table
// was loaded from, the gateway's name, the claim it reads, the
// environment its addresses were resolved in, and one row per forward.
type gatewayTable struct {
	GatewayTable int            `json:"gatewayTable"`
	Path         string         `json:"path"`
	Commit       string         `json:"commit"`
	Repository   string         `json:"repository"`
	Gateway      string         `json:"gateway"`
	Claim        string         `json:"claim"`
	Environment  *string        `json:"environment"`
	Forwards     []tableForward `json:"forwards"`
}

type tableForward struct {
	Value   string  `json:"value"`
	Name    *string `json:"name"`
	Setting *string `json:"setting"`
	URL     *string `json:"url"`
}

// The keys of the table and of each row; every one is written, null where
// the format allows it, and no other is.
var (
	gatewayTableKeys = []string{"gatewayTable", "path", "commit", "gateway", "claim", "environment", "forwards"}
	tableForwardKeys = []string{"value", "name", "setting", "url"}
	claimWord        = regexp.MustCompile(`^[a-z][A-Za-z0-9_]*$`)
	environmentWord  = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
)

// gatewayReader writes the tree of one gateway table.
type gatewayReader struct {
	t         *gatewayTable
	key       string
	res       *Result
	design    *yaml.Node // the questions about the design stage
	deploy    *yaml.Node // the questions about the deployment stage
	nextID    int
	dumpName  string
	gateway   string // the gateway's name in the tree
	deps      []string
	depRows   map[string][]int // a dependency's rows, by index
	depURL    map[string]string
	settingOf map[string]string // a row's setting name in the tree, by the table's
}

// GatewayTable reads a gateway table into the gateway it names, a
// dependency per service it forwards to, a setting per address it reads,
// and, with an environment and implementationOut, the servers of that
// deployment (ADR-142).
func GatewayTable(dumpPath, out, key string, implementationOut bool) (*Result, error) {
	t, r, dumpName, err := openGatewayTable(dumpPath)
	if err != nil {
		return nil, err
	}
	gr := &gatewayReader{t: t, key: key, res: &Result{Tree: newTree()}, design: mapping(), deploy: mapping(),
		dumpName: dumpName, gateway: settingName(t.Gateway), depRows: map[string][]int{}, depURL: map[string]string{}, settingOf: map[string]string{}}
	gr.res.say("commit %s: the last change to %s, as the gateway table %s names it", r.Commit, t.Path, dumpName)
	gr.res.say("counted %s: every row of the gateway table, which holds every service the gateway %s loads", plural(len(t.Forwards), "forward"), t.Gateway)
	design := mapping()
	gw := gr.gatewayElement()
	set(design, "dependencies", gr.dependencies())
	set(design, "gateways", mapping(gr.gateway, gw))
	gr.res.Tree.put("design/gateway.yaml", design)
	stages := []string{"requirements", "design"}
	if config := gr.settings(); config != nil {
		stages = append(stages, "deployment")
		gr.res.Tree.put("deployment/configuration.yaml", mapping("configuration", config))
	}
	gr.res.Tree.put("requirements/stakeholders.yaml", mapping("stakeholders", ownerStakeholder()))
	gr.res.Tree.put("design/questions.yaml", mapping("questions", gr.design))
	if len(gr.deploy.Content) > 0 {
		gr.res.Tree.put("deployment/questions.yaml", mapping("questions", gr.deploy))
	}
	settings := len(gr.settingOf)
	gr.res.say("wrote 1 gateway, %s, %s and %s: the gateway with its claim and one forward per row, one dependency per service a row names, one setting per address a row reads, and one question per thing the table does not say",
		plural(len(gr.deps), "dependency"), plural(settings, "setting"), plural(gr.nextID, "question"))
	if implementationOut {
		gr.servers()
	}
	src, err := codeSource(r, out, []*yaml.Node{flow(mapping("clause", t.Path, "title", "The table of services the gateway loads from here, as its printer prints it"))})
	if err != nil {
		return nil, err
	}
	set(src, "reading", "printed")
	description := fmt.Sprintf("The gateway %s, read from the gateway table %s, made at commit %s: the claim it forwards by, the services each value of it selects and the settings their addresses are read from. Which token the gateway verifies, and what it answers itself, the table does not show; each is a question.\n", t.Gateway, dumpName, r.Commit)
	gr.res.Tree.put("specarch.yaml", rootFile("Gateway "+t.Gateway+" of "+t.Path, description, stages, mapping(key, src)))
	return gr.res, nil
}

// openGatewayTable reads a gateway table, checks that it and every row
// hold the keys of the format and no other, and checks it against the
// commit it names.
func openGatewayTable(dumpPath string) (*gatewayTable, *Read, string, error) {
	data, err := os.ReadFile(dumpPath)
	if err != nil {
		return nil, nil, "", err
	}
	var t gatewayTable
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return nil, nil, "", refuse("%s is not a gateway table: %v", dumpPath, err)
	}
	if t.GatewayTable != 1 {
		return nil, nil, "", refuse("%s is not a gateway table in the format tools/gateways/dump-gateway.sh writes: its gatewayTable is %d, not 1", dumpPath, t.GatewayTable)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, "", refuse("%s is not a gateway table: %v", dumpPath, err)
	}
	keys := gatewayTableKeys
	if _, ok := raw["repository"]; ok {
		keys = append(append([]string{}, keys...), "repository")
	}
	if err := hasKeys(dumpPath, "the table", raw, keys); err != nil {
		return nil, nil, "", err
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(raw["forwards"], &rows); err != nil {
		return nil, nil, "", refuse("%s: forwards is not a list of rows: %v", dumpPath, err)
	}
	for i, row := range rows {
		if err := hasKeys(dumpPath, fmt.Sprintf("row %d", i+1), row, tableForwardKeys); err != nil {
			return nil, nil, "", err
		}
	}
	r, dumpName, err := openDump(dumpPath, t.Repository, t.Path, t.Commit, "tools/gateways/dump-gateway.sh")
	if err != nil {
		return nil, nil, "", err
	}
	if !memberNameWord.MatchString(settingName(t.Gateway)) {
		return nil, nil, "", refuse("%s: the gateway is named %q, which gives no camelCase name; print the gateway's name in words", dumpName, t.Gateway)
	}
	if !claimWord.MatchString(t.Claim) {
		return nil, nil, "", refuse("%s: the claim %q is not a claim's name, a word of letters, digits and underscores starting with a small letter, as RFC 7519's are", dumpName, t.Claim)
	}
	seen := map[string]int{}
	for i, f := range t.Forwards {
		if f.Value == "" {
			return nil, nil, "", refuse("%s, row %d has no value", dumpName, i+1)
		}
		if j, ok := seen[f.Value]; ok {
			return nil, nil, "", refuse("%s, row %d: the value %s is also row %d's; a request goes to one service, so a gateway holds each value once", dumpName, i+1, f.Value, j+1)
		}
		seen[f.Value] = i
	}
	return &t, r, dumpName, nil
}

// at cites the table at a row, or at the table itself for row -1.
func (gr *gatewayReader) at(row int, says string) *yaml.Node {
	clause := gr.t.Path
	if row >= 0 {
		return citation(gr.key, clause, fmt.Sprintf("Row %d of the gateway table: %s", row+1, says))
	}
	return citation(gr.key, clause, says)
}

// question asks a question about the stage whose questions file holds it.
func (gr *gatewayReader) question(into *yaml.Node, priority, text string, blocks []string, why string, cite *yaml.Node) {
	gr.nextID++
	q := mapping(
		"question", text,
		"kind", "decision",
		"priority", priority,
		"blocks", blocks,
		"decidedBy", owner,
		"why", why,
	)
	if cite != nil {
		set(q, "cites", []*yaml.Node{cite})
	}
	set(into, fmt.Sprintf("Q-%d", gr.nextID), q)
}

// dependencyName is the dependency a row selects: the row's name in
// camelCase, else the host's first label of a URI value, else the value.
func dependencyName(f tableForward) string {
	if f.Name != nil && *f.Name != "" {
		return settingName(*f.Name)
	}
	if u, err := url.Parse(f.Value); err == nil && u.Scheme != "" && u.Hostname() != "" {
		host := u.Hostname()
		first, _, _ := strings.Cut(host, ".")
		return settingName(first)
	}
	return settingName(f.Value)
}

// gatewayElement writes the gateway with its claim and its forwards, and
// asks for its description, its token and its refusals.
func (gr *gatewayReader) gatewayElement() *yaml.Node {
	t := gr.t
	at := "#/gateways/" + gr.gateway
	cites := []*yaml.Node{gr.at(-1, fmt.Sprintf("The gateway %s forwards by the claim %s, to the services of %s.", t.Gateway, t.Claim, plural(len(t.Forwards), "row")))}
	var rows []*yaml.Node
	for i, f := range t.Forwards {
		dep := dependencyName(f)
		if !memberNameWord.MatchString(dep) {
			gr.question(gr.design, "must", fmt.Sprintf("Row %d of the gateway table forwards %s, and neither its name nor its value gives a dependency's name. Which dependency does it forward to?", i+1, f.Value),
				[]string{at + "/forwards"}, "A forward names the dependency it reaches, and the row gives no word to name it by.", gr.at(i, "Forwards "+f.Value+"."))
			continue
		}
		if len(gr.depRows[dep]) == 0 {
			gr.deps = append(gr.deps, dep)
		}
		gr.depRows[dep] = append(gr.depRows[dep], i)
		if f.URL != nil && gr.depURL[dep] == "" {
			gr.depURL[dep] = *f.URL
		}
		rows = append(rows, flow(mapping("value", f.Value, "dependency", dep)))
		cites = append(cites, gr.at(i, fmt.Sprintf("Forwards a request whose %s is %s to %s.", t.Claim, f.Value, dep)))
	}
	gw := mapping("claim", t.Claim)
	set(gw, "forwards", rows)
	set(gw, "origin", "stated")
	set(gw, "cites", cites)
	gr.question(gr.design, "must", fmt.Sprintf("What does gateway %s do, in a sentence a reviewer can check?", gr.gateway),
		[]string{at + "/description"}, "The gateway table is a lookup and says nothing of what the gateway is for.", nil)
	gr.question(gr.design, "must", fmt.Sprintf("Which token does gateway %s verify before it reads %s, with its format, its signature algorithm and its claims?", gr.gateway, t.Claim),
		[]string{at + "/token"}, "The gateway table shows a lookup by a claim and not the verifier that checks the token first; a claim read before the signature is checked is no gateway the meta-model holds.", nil)
	gr.question(gr.design, "should", fmt.Sprintf("What does gateway %s answer itself, with which status and problem type, when the token is missing or wrong, when the %s names no service, and when the service fails or does not answer in time?", gr.gateway, t.Claim),
		[]string{at + "/refusals"}, "The gateway table holds the services and not the gateway's own answers.", nil)
	var shared []string
	for _, dep := range gr.deps {
		if len(gr.depRows[dep]) > 1 {
			shared = append(shared, dep)
		}
	}
	if len(shared) > 0 {
		gr.question(gr.design, "must", fmt.Sprintf("The gateway table forwards several values to %s. Does one service answer each of those audiences, or is each another service?", joinAnd(shared)),
			[]string{at + "/forwards"}, "Two rows that give one dependency's name may be one service with two audiences or two services, and the table does not say which.", nil)
	}
	return gw
}

// dependencies writes a dependency per service the forwards name, and
// asks for what the table does not say of each.
func (gr *gatewayReader) dependencies() *yaml.Node {
	deps := mapping()
	for _, dep := range gr.deps {
		at := "#/dependencies/" + dep
		rows := gr.depRows[dep]
		f := gr.t.Forwards[rows[0]]
		d := mapping("kind", "system")
		u := gr.depURL[dep]
		if parsed, err := url.Parse(u); u != "" && err == nil && parsed.Scheme != "" {
			set(d, "protocol", strings.ToLower(parsed.Scheme))
		}
		var cites []*yaml.Node
		for _, i := range rows {
			says := fmt.Sprintf("Forwards %s to the service", gr.t.Forwards[i].Value)
			if n := gr.t.Forwards[i].Name; n != nil && *n != "" {
				says += " " + *n
			}
			cites = append(cites, gr.at(i, says+"."))
		}
		set(d, "origin", "stated")
		set(d, "cites", cites)
		set(deps, dep, d)
		gr.question(gr.design, "must", fmt.Sprintf("What is the service %s, and how long does the gateway wait for its answer?", dep),
			[]string{at + "/description", at + "/timeout"}, "The gateway table names the service and not what it is or how long a request to it may take.", gr.at(rows[0], "Forwards "+f.Value+"."))
		if u == "" {
			gr.question(gr.design, "should", fmt.Sprintf("How is the service %s reached: over https, or another protocol?", dep),
				[]string{at + "/protocol"}, "The row gives no address, so no scheme says how the service is reached.", gr.at(rows[0], "Forwards "+f.Value+" with no address."))
		}
		if f.Name == nil || *f.Name == "" {
			gr.question(gr.design, "could", fmt.Sprintf("The dependency %s is named after the value %s, since the gateway table gives the service no name. Is there a better name for it?", dep, f.Value),
				[]string{at}, "A name made from an address is a guess at what the service is called.", gr.at(rows[0], "Forwards "+f.Value+"."))
		}
	}
	return deps
}

// settings writes a setting per address a row reads, a string of format
// uri that is no secret, and asks to confirm that; nil when no row names
// one.
func (gr *gatewayReader) settings() *yaml.Node {
	config := mapping()
	type read struct {
		key  string
		rows []int
	}
	byName := map[string]*read{}
	var names []string
	for i, f := range gr.t.Forwards {
		if f.Setting == nil || *f.Setting == "" {
			continue
		}
		n := settingName(*f.Setting)
		if !memberNameWord.MatchString(n) {
			gr.question(gr.deploy, "must", fmt.Sprintf("Row %d of the gateway table reads the address of %s from the setting %s, which gives no camelCase name. Which setting is it?", i+1, f.Value, *f.Setting),
				[]string{"#/configuration"}, "A setting has a camelCase name, and this one gives none.", gr.at(i, "Reads the address from "+*f.Setting+"."))
			continue
		}
		if byName[n] == nil {
			byName[n] = &read{key: *f.Setting}
			names = append(names, n)
		}
		byName[n].rows = append(byName[n].rows, i)
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	var plain, secretLike []string
	var plainBlocks, secretBlocks []string
	for _, n := range names {
		r := byName[n]
		var deps []string
		var cites []*yaml.Node
		for _, i := range r.rows {
			f := gr.t.Forwards[i]
			dep := dependencyName(f)
			if !contains(deps, dep) {
				deps = append(deps, dep)
			}
			cites = append(cites, gr.at(i, fmt.Sprintf("Reads the address of the service %s forwards to from %s.", f.Value, r.key)))
		}
		gr.settingOf[r.key] = n
		set(config, n, mapping(
			"description", fmt.Sprintf("The address of %s, the service the gateway %s forwards to, read from %s.", joinAnd(deps), gr.t.Gateway, r.key),
			"secret", false,
			"schema", flow(mapping("type", "string", "format", "uri")),
			"origin", "stated",
			"cites", cites,
		))
		if secretWord.MatchString(r.key) {
			secretLike = append(secretLike, n)
			secretBlocks = append(secretBlocks, "#/configuration/"+n+"/secret")
		} else {
			plain = append(plain, n)
			plainBlocks = append(plainBlocks, "#/configuration/"+n+"/secret")
		}
	}
	if len(plain) > 0 {
		gr.question(gr.deploy, "should", fmt.Sprintf("Is each of these settings no secret, as an address is not: %s?", strings.Join(plain, ", ")),
			plainBlocks, "The gateway table names the setting an address is read from and not whether it is sensitive.", nil)
	}
	if len(secretLike) > 0 {
		gr.question(gr.deploy, "must", fmt.Sprintf("Is each of these settings a secret: %s? Each name says it may hold a credential, and the gateway table reads an address from it.", strings.Join(secretLike, ", ")),
			secretBlocks, "A secret's value is never written in a specification, and the name alone does not say which a setting is.", nil)
	}
	return config
}

// servers writes the implementation fragment of the deployment the table's
// addresses were resolved in: a server per service with an address, and
// the value of each setting there.
func (gr *gatewayReader) servers() {
	t := gr.t
	switch {
	case t.Environment == nil || *t.Environment == "":
		gr.res.say("not written: the servers of the forwarded services, since the gateway table names no environment the addresses were resolved in, and no deployment names where they hold")
		return
	case !environmentWord.MatchString(*t.Environment):
		gr.res.say("not written: the servers of the forwarded services, since the environment %s is not a deployment's kebab-case name", *t.Environment)
		return
	}
	var servers []*yaml.Node
	config := mapping()
	for _, dep := range gr.deps {
		u := gr.depURL[dep]
		if u == "" {
			continue
		}
		servers = append(servers, flow(mapping("name", kebab(dep), "url", u, "dependency", dep)))
	}
	var keys []string
	for k := range gr.settingOf {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, f := range t.Forwards {
			if f.Setting != nil && *f.Setting == k && f.URL != nil {
				set(config, gr.settingOf[k], *f.URL)
				break
			}
		}
	}
	if len(servers) == 0 {
		gr.res.say("not written: the servers of the forwarded services, since no row of the gateway table gives an address")
		return
	}
	d := mapping("description", fmt.Sprintf("Where the services the gateway %s forwards to are in %s, as the gateway table resolved their addresses.", t.Gateway, *t.Environment))
	set(d, "servers", servers)
	if len(config.Content) > 0 {
		set(d, "configuration", config)
	}
	gr.res.Implementation = mapping("deployments", mapping(*t.Environment, d))
	gr.res.ImplementationNote = fmt.Sprintf("The servers specarch extract gateway read from the gateway table %s, made at commit %s: the address of each service the gateway %s forwards to in %s, and the settings that hold them. They go under deployments in an implementation file of the specification.", gr.dumpName, t.Commit, t.Gateway, *t.Environment)
	gr.res.say("wrote the deployment %s: %s and %s", *t.Environment, plural(len(servers), "server"), plural(len(config.Content)/2, "setting value"))
}
