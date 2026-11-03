// Package gendeploy writes the files that put a specification's system
// into each deployment of an implementation file (ADR-227): a Compose
// file, Kubernetes manifests or a cloud-init file per host, as the deploy
// target's settings name the form of each deployment; the runbooks of the
// release, the rollback and each migration of the deployment stage; and,
// for a Compose deployment the settings name under fresh, the script that
// builds an environment from nothing, loads the migrations and the seed,
// runs the implementation's test suite there and tears it down. Every
// value written comes from the implementation file's deployment or the
// setting's default; a secret is referenced where the form reads it from
// the host, and never written.
package gendeploy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/SpecArch/specarch/internal/gensql"
	"github.com/SpecArch/specarch/internal/mark"
)

// Request is what specarch writes on the plug-in's standard input.
type Request struct {
	gensql.Request
	// Check is true when specarch only compares the output with the disk.
	Check bool `json:"check"`
}

// Decode reads a request, keeping numbers as their text.
func Decode(data []byte) (*Request, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var r Request
	if err := dec.Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// The plug-in's name, as its header and its messages give it.
const name = "specarch-gen-deploy"

// The forms a deployment is written in.
const (
	formCompose    = "compose"
	formKubernetes = "kubernetes"
	formCloudInit  = "cloud-init"
)

type gen struct {
	req      *Request
	spec     map[string]any
	diags    []gensql.Diagnostic
	reported map[string]bool
	files    []gensql.File
	written  map[string]string // a deployment's key: the implementation file it came from
	placed   map[string][]mark.Problem
	version  string
	sysName  string
	// settings are the specification's configuration, by key, in order.
	settings []setting
	// procedures and migrations are written once, at the output's root.
	runbooks bool
}

// setting is one entry of the specification's configuration.
type setting struct {
	key     string
	secret  bool
	env     string // UPPER_SNAKE of the key
	def     any    // the schema's default, or nil
	pointer string
}

// deployment is one deployment of an implementation file, read with the
// settings that apply to it.
type deployment struct {
	key     string
	file    string
	content map[string]any
	env     string // the environment it installs
	form    string
	fresh   bool
	servers []server
	links   []link
	values  []value // the non-secret settings' values, in configuration order
	// dialect is the sql target's dialect of the file, for the database.
	dialect string
	// missing are the non-secret settings with no value in the deployment
	// and no default, reported at the setting and named in the files.
	missing []setting
	// build and test are the commands the fresh script runs.
	build string
	test  string
	// migrations and seed are the folders of the sql and seed targets,
	// relative to the output folder, or "".
	migrations string
	seed       string
	impl       map[string]any
}

type server struct {
	name       string
	kind       string
	url        string
	dependency string
	image      string
	storage    string
	ports      []int
	protocols  map[int]string
	desc       string
}

type link struct {
	from, to string
	port     int
}

type value struct {
	s   setting
	val string
}

// Generate writes the deployment files of a request.
func Generate(r *Request) gensql.Response {
	g := &gen{req: r, spec: r.Specification, reported: map[string]bool{}, written: map[string]string{}}
	info := obj0(g.spec["info"])
	g.version, g.sysName = text(info["version"]), text(info["name"])
	g.settings = g.readSettings()
	g.placeProblems()
	var deployments []*deployment
	for _, impl := range r.Implementations {
		target, named := obj(obj0(impl.Content["targets"])[r.Target])
		if !named && len(r.Implementations) > 1 {
			continue
		}
		deployments = append(deployments, g.readDeployments(impl, target)...)
	}
	any := len(deployments) > 0
	if g.sysName == "" && any {
		g.problem("/info", "the deploy target names the Compose project, the Kubernetes namespace, the database and the host's folder after info.name, which the specification does not give; add info.name, in kebab case, and the files of each deployment are written")
		deployments = nil
	}
	for _, d := range deployments {
		switch d.form {
		case formCompose:
			g.compose(d)
			if d.fresh {
				g.fresh(d)
			}
		case formKubernetes:
			g.kubernetes(d)
		case formCloudInit:
			g.cloudInit(d)
		}
	}
	if any {
		g.runbookFiles()
	}
	return g.response()
}

// readSettings reads the specification's configuration in key order.
func (g *gen) readSettings() []setting {
	conf := obj0(g.spec["configuration"])
	keys := make([]string, 0, len(conf))
	for k := range conf {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []setting
	for _, k := range keys {
		s := obj0(conf[k])
		sch := obj0(s["schema"])
		out = append(out, setting{key: k, secret: s["secret"] == true, env: upperSnake(k), def: sch["default"], pointer: "/configuration/" + mark.Escape(k)})
	}
	return out
}

// placeProblems gives each element the output shows the problems marked
// at it: an environment, a setting, the release, the rollback and each
// migration.
func (g *gen) placeProblems() {
	var elements []string
	for k := range obj0(g.spec["environments"]) {
		elements = append(elements, "/environments/"+mark.Escape(k))
	}
	for _, s := range g.settings {
		elements = append(elements, s.pointer)
	}
	for k := range obj0(g.spec["migrations"]) {
		elements = append(elements, "/migrations/"+mark.Escape(k))
	}
	elements = append(elements, "/release", "/rollback")
	g.placed = mark.Place(g.req.Problems, elements, []string{"/environments", "/configuration", "/migrations", "/release", "/rollback"})
}

// readDeployments reads the deployments of one implementation file with
// the deploy target's settings: the form of each, the images and storage
// of its servers, and which are written fresh.
func (g *gen) readDeployments(impl gensql.Implementation, target map[string]any) []*deployment {
	settings := obj0(target["settings"])
	forms := obj0(settings["deployments"])
	images := obj0(settings["images"])
	storage := obj0(settings["storage"])
	deps := obj0(impl.Content["deployments"])
	keys := make([]string, 0, len(deps))
	for k := range deps {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for k := range forms {
		if _, ok := deps[k]; !ok {
			g.problem("/", "settings.deployments of the deploy target names %s, which is not a deployment of %s; name a deployment of the file", k, impl.File)
		}
	}
	fresh := map[string]bool{}
	for _, f := range list(settings["fresh"]) {
		k := text(f)
		if _, ok := deps[k]; !ok {
			g.problem("/", "settings.fresh of the deploy target names %s, which is not a deployment of %s; name a deployment of the file", k, impl.File)
			continue
		}
		fresh[k] = true
	}
	tasks := obj0(impl.Content["tasks"])
	var out []*deployment
	for _, k := range keys {
		content := obj0(deps[k])
		d := &deployment{key: k, file: impl.File, content: content, env: text(content["environment"]), impl: impl.Content}
		if prev, ok := g.written[k]; ok {
			g.problem("/", "the deployment %s is in %s and in %s, and the deploy target writes one folder per deployment; rename one of them", k, prev, impl.File)
			continue
		}
		g.written[k] = impl.File
		d.form = text(obj0(forms[k])["form"])
		switch d.form {
		case formCompose, formKubernetes, formCloudInit:
		case "":
			g.problem("/", "the deployment %s of %s names no form under settings.deployments.%s.form of the deploy target; write compose, kubernetes or cloud-init, and the files of the deployment are written", k, impl.File, k)
			continue
		default:
			g.problem("/", "settings.deployments.%s.form of the deploy target is %s, and %s writes compose, kubernetes and cloud-init; name one of them", k, d.form, name)
			continue
		}
		if fresh[k] {
			if d.form != formCompose {
				g.problem("/", "settings.fresh of the deploy target names %s, whose form is %s, and a fresh environment is written for a compose deployment alone, since a Compose project is built and torn down on one host with no provisioner; name a compose deployment, or leave %s out of fresh", k, d.form, k)
			} else {
				d.fresh = true
			}
		}
		d.dialect = "postgresql"
		if dl := text(obj0(obj0(impl.Content["targets"])["sql"])["dialect"]); dl != "" {
			d.dialect = dl
		}
		d.migrations = g.relOutput(impl.File, text(obj0(obj0(impl.Content["targets"])["sql"])["output"]))
		d.seed = g.relOutput(impl.File, text(obj0(obj0(impl.Content["targets"])["seed"])["output"]))
		d.test = text(obj0(impl.Content["testing"])["run"])
		buildTask := text(obj0(content["build"])["task"])
		if buildTask == "" {
			if _, ok := tasks["build"]; ok {
				buildTask = "build"
			}
		}
		d.build = text(obj0(tasks[buildTask])["run"])
		for _, s := range list(content["servers"]) {
			sv := obj0(s)
			srv := server{name: text(sv["name"]), kind: text(sv["kind"]), url: text(sv["url"]), dependency: text(sv["dependency"]), desc: text(sv["description"]), protocols: map[int]string{}}
			if srv.kind == "" {
				srv.kind = "service"
			}
			for _, l := range list(sv["listens"]) {
				lm := obj0(l)
				p := integer(lm["port"])
				srv.ports = append(srv.ports, p)
				srv.protocols[p] = text(lm["protocol"])
			}
			if len(srv.ports) == 0 {
				if u, err := url.Parse(srv.url); err == nil && u.Port() != "" {
					srv.ports = append(srv.ports, integer(json.Number(u.Port())))
				}
			}
			if srv.name == "" {
				g.problem("/", "a server of the deployment %s of %s has no name, and the deploy target writes each server under its name; give it one", k, impl.File)
				continue
			}
			if srv.dependency == "" {
				srv.image = text(images[srv.name])
				srv.storage = text(storage[srv.name])
			}
			d.servers = append(d.servers, srv)
		}
		for _, l := range list(content["links"]) {
			lm := obj0(l)
			d.links = append(d.links, link{from: text(lm["from"]), to: text(lm["to"]), port: integer(lm["port"])})
		}
		conf := obj0(content["configuration"])
		for _, s := range g.settings {
			if s.secret {
				continue
			}
			v, ok := conf[s.key]
			if !ok {
				v = s.def
			}
			if v == nil {
				g.problem(s.pointer, "the setting %s has no value in the deployment %s of %s and no default; write it under deployments.%s.configuration.%s, or give the setting a default", s.key, k, impl.File, k, s.key)
				d.missing = append(d.missing, s)
				continue
			}
			d.values = append(d.values, value{s: s, val: scalar(v)})
		}
		out = append(out, d)
	}
	return out
}

// relOutput is another target's output folder, named relative to the
// implementation file, written relative to this target's output folder;
// "" when the target has none.
func (g *gen) relOutput(implFile, output string) string {
	if output == "" {
		return ""
	}
	to := filepath.Join(filepath.Dir(filepath.FromSlash(implFile)), filepath.FromSlash(output))
	absTo, err1 := filepath.Abs(to)
	absOut, err2 := filepath.Abs(filepath.FromSlash(g.req.Output))
	if err1 != nil || err2 != nil {
		return filepath.ToSlash(to)
	}
	rel, err := filepath.Rel(absOut, absTo)
	if err != nil {
		return filepath.ToSlash(to)
	}
	return filepath.ToSlash(rel)
}

// header is the first comment of a file of a deployment.
func (g *gen) header(d *deployment, what string) string {
	h := fmt.Sprintf("# Generated by %s from %s, version %s; meta-model %s; digest %s; %s. Do not edit this file: a change goes into the specification, the implementation file's deployments or the deploy target's settings.\n",
		name, gensql.RelRoot(g.req.Root, g.req.Output), g.version, g.req.Specarch, g.req.Digest, what)
	if g.req.Draft != "" {
		h += "# " + g.req.Draft + "\n"
	}
	if d != nil {
		h += g.marks("/environments/" + mark.Escape(d.env))
	}
	h += g.marks(mark.File)
	return h
}

// marks are the problem lines of one element, each a comment.
func (g *gen) marks(element string) string {
	var b strings.Builder
	for _, p := range g.placed[element] {
		b.WriteString("# " + mark.Line(p) + "\n")
	}
	return b.String()
}

// secretReason says, where a form reads a secret, why it is not written.
func secretReason(s setting) string {
	return s.key + " is a secret of the specification, never written here"
}

// envFile is the non-secret settings' values as a file podman, Compose
// and systemd read: one NAME=value a line.
func (g *gen) envFile(d *deployment) string {
	var b strings.Builder
	b.WriteString(g.header(d, fmt.Sprintf("the values of the settings in the deployment %s of environment %s", d.key, d.env)))
	b.WriteString("# A secret is not here: the form that runs the system reads it from the host.\n")
	for _, v := range d.values {
		b.WriteString("# specarch: #" + v.s.pointer + "\n")
		b.WriteString(g.marks(v.s.pointer))
		b.WriteString(v.s.env + "=" + v.val + "\n")
	}
	for _, s := range d.missing {
		b.WriteString("# specarch: #" + s.pointer + "\n")
		b.WriteString(g.marks(s.pointer))
		b.WriteString("# " + s.env + ": no value in the deployment " + d.key + " and no default\n")
	}
	for _, s := range g.settings {
		if s.secret {
			b.WriteString("# specarch: #" + s.pointer + "\n")
			b.WriteString(g.marks(s.pointer))
			b.WriteString("# " + s.env + ": " + secretReason(s) + "\n")
		}
	}
	return b.String()
}

// secrets are the secret settings, in order.
func (g *gen) secrets() []setting {
	var out []setting
	for _, s := range g.settings {
		if s.secret {
			out = append(out, s)
		}
	}
	return out
}

// database says how a database server of a dialect is run as a container:
// the password variable the image reads, the data folder, the variable
// that names the database, and the client the fresh script loads a file
// with; ok is false for a dialect the plug-in does not run.
type database struct {
	passwordVar string
	dataPath    string
	databaseVar string
	ready       string // the command that answers once the server takes connections
	load        string // the command that loads standard input into the database
}

func (g *gen) database(d *deployment, s server) (database, bool) {
	db := dbName(g.sysName)
	switch d.dialect {
	case "postgresql":
		return database{passwordVar: "POSTGRES_PASSWORD", dataPath: "/var/lib/postgresql/data", databaseVar: "POSTGRES_DB",
			ready: "pg_isready -U postgres", load: "psql -v ON_ERROR_STOP=1 -q -U postgres -d " + db}, true
	case "mariadb":
		return database{passwordVar: "MARIADB_ROOT_PASSWORD", dataPath: "/var/lib/mysql", databaseVar: "MARIADB_DATABASE",
			ready: "mariadb-admin ping -uroot -p\"$MARIADB_ROOT_PASSWORD\" --silent", load: "mariadb -uroot -p\"$MARIADB_ROOT_PASSWORD\" " + db}, true
	}
	return database{}, false
}

// dbName is the database's name: the system's name as an identifier.
func dbName(system string) string {
	return strings.ReplaceAll(strings.ReplaceAll(system, "-", "_"), ".", "_")
}

// imageOf is a server's image from the settings, or "" with the problem
// reported at the target.
func (g *gen) imageOf(d *deployment, s server) string {
	if s.image == "" {
		g.problem("/", "the server %s of the deployment %s has no image under settings.images.%s of the deploy target; name the image the server runs, with its tag or digest, and the server is written", s.name, d.key, s.name)
	}
	return s.image
}

// serverPointer is the comment that names where a server comes from.
func (d *deployment) serverPointer(s server) string {
	return fmt.Sprintf("# %s: %s of the deployment %s, %s", s.name, s.kind, d.key, s.url)
}

// dependencyNote is the comment for a server that is a dependency, which
// runs nothing of this implementation.
func dependencyNote(s server) string {
	return fmt.Sprintf("# %s: the dependency %s, reached at %s; not run here\n", s.name, s.dependency, s.url)
}

// dependsOn are the servers a server links to, in the deployment's order.
func (d *deployment) dependsOn(from string) []string {
	var out []string
	for _, l := range d.links {
		if l.from == from && !contains(out, l.to) {
			for _, s := range d.servers {
				if s.name == l.to && s.dependency == "" {
					out = append(out, l.to)
				}
			}
		}
	}
	return out
}

// linksTo are the links into a server.
func (d *deployment) linksTo(to string) []link {
	var out []link
	for _, l := range d.links {
		if l.to == to {
			out = append(out, l)
		}
	}
	return out
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func (g *gen) file(p, content string) {
	g.files = append(g.files, gensql.File{Path: p, Content: content})
}

func (g *gen) problem(path, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if g.reported[path+"\x00"+msg] {
		return
	}
	g.reported[path+"\x00"+msg] = true
	g.diags = append(g.diags, gensql.Diagnostic{File: g.req.Root, Line: 1, Severity: "error", Path: path, Rule: "generator", Message: msg})
}

func (g *gen) response() gensql.Response {
	files := g.files
	if files == nil {
		files = []gensql.File{}
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	diags := g.diags
	if diags == nil {
		diags = []gensql.Diagnostic{}
	}
	return gensql.Response{Files: files, Diagnostics: diags}
}

// scalar writes a value as the text the files carry.
func scalar(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return fmt.Sprint(x)
	}
	return fmt.Sprint(v)
}

func integer(v any) int {
	switch x := v.(type) {
	case json.Number:
		i, _ := x.Int64()
		return int(i)
	case float64:
		return int(x)
	case int:
		return x
	}
	return 0
}

// yamlString writes a text as a double-quoted YAML scalar.
func yamlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// shQuote writes a text as a single-quoted shell word.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// upperSnake writes dailyRate as DAILY_RATE and notificationChannelUrl as
// NOTIFICATION_CHANNEL_URL: the environment variable of a setting.
func upperSnake(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case r == '-' || r == '.' || r == ' ':
			b.WriteByte('_')
		case unicode.IsUpper(r) && i > 0 && (unicode.IsLower(rs[i-1]) || unicode.IsDigit(rs[i-1]) || i+1 < len(rs) && unicode.IsLower(rs[i+1]) && unicode.IsUpper(rs[i-1])):
			b.WriteByte('_')
			b.WriteRune(r)
		default:
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	return b.String()
}

func obj(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func obj0(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func text(v any) string {
	s, _ := v.(string)
	return s
}
