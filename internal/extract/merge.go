package extract

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// The methods a path item holds, in the order the readers write them.
var pathMethods = []string{"get", "post", "put", "patch", "delete"}

// The two sides of a merge (docs/from-sources.md, section 3.2).
const (
	codeSide      = "code"
	documentsSide = "documents"
)

// mergeTree is one tree given to merge.
type mergeTree struct {
	s     *spec.Spec
	dir   string // with symbolic links resolved
	title string
	side  string
}

// element is one element of the merged specification: an entry of a
// section, an operation, or a field of an entity.
type element struct {
	tokens  []string // from the section down, as a pointer has them
	section string
	parent  string // the pointer of the entity a field belongs to
	trees   []int  // the trees that have it, in order
}

// mergeStep is one step of a path inside an element: a key of a mapping,
// or the item of a list of named objects with that name.
type mergeStep struct {
	key  string
	name bool
}

type merger struct {
	trees      []*mergeTree
	out        string
	res        *Result
	files      map[string]*yaml.Node  // the merged files by path, each a mapping of sections
	entries    map[string]*yaml.Node  // section/name -> the merged entry
	elements   map[string]*element    // pointer -> element
	order      []string               // the elements' pointers in the order they were met
	disputes   map[string][]mergeStep // path string -> the path of a key the trees disagree on
	disputeOrd []string               // the disputes in the order they were found
	sources    map[string]*yaml.Node  // the merged sources
	sourceKeys []string               // in the order they were met
	given      map[string]bool        // the source keys marked givenOutside
	questions  []*yaml.Node           // the merged questions' values, numbered in order
	qfiles     []string               // the file of each question
	stages     map[string]bool        // the stages written
}

// Merge merges the trees, each a specification that tracks origin and that
// validate accepts, into one written into out (docs/from-sources.md,
// section 3.2; ADR-045).
func Merge(specs []*spec.Spec, out string) (*Result, error) {
	m := &merger{out: out, res: &Result{Tree: newTree()}, files: map[string]*yaml.Node{}, entries: map[string]*yaml.Node{},
		elements: map[string]*element{}, disputes: map[string][]mergeStep{}, sources: map[string]*yaml.Node{},
		given: map[string]bool{}, stages: map[string]bool{}}
	for _, s := range specs {
		t, err := checkTree(s)
		if err != nil {
			return nil, err
		}
		m.trees = append(m.trees, t)
	}
	if err := m.mergeSources(); err != nil {
		return nil, err
	}
	for i, t := range m.trees {
		m.mergeTree(i, t)
	}
	m.removeDisputed()
	m.treeQuestions()
	asked := m.disputeQuestions()
	asked += m.sideQuestions()
	asked += m.quantityQuestions()
	m.placeholders()
	return m.write(asked)
}

// placeholders reports a documents-side source whose operations share no
// path with the code side's, such as an OpenAPI file a service template
// ships and nothing serves (ADR-049).
func (m *merger) placeholders() {
	served := map[string]bool{}
	for _, ptr := range m.order {
		e := m.elements[ptr]
		if e.section == "paths" && m.onSide(e, codeSide) {
			served[e.tokens[1]] = true
		}
	}
	if len(served) == 0 {
		return
	}
	for _, t := range m.trees {
		if t.side != documentsSide {
			continue
		}
		for _, src := range source.Pairs(source.Child(t.s.Root, "sources")) {
			var paths []string
			for _, p := range source.Pairs(source.Child(t.s.Root, "paths")) {
				for _, method := range pathMethods {
					for _, c := range source.Items(source.Child(source.Child(p.Value, method), "cites")) {
						if source.Str(source.Child(c, "source")) == src.Key.Value && !contains(paths, p.Key.Value) {
							paths = append(paths, p.Key.Value)
						}
					}
				}
			}
			if len(paths) == 0 {
				continue
			}
			shared := false
			for _, p := range paths {
				shared = shared || served[p]
			}
			if !shared {
				m.res.say("placeholder: the source %s of %s, %s, cites %s and the code serves none of them; a document no path of which is served is a placeholder until the owner says otherwise",
					src.Key.Value, t.s.Dir, source.Str(source.Child(src.Value, "title")), plural(len(paths), "path"))
			}
		}
	}
}

// checkTree refuses a tree merge does not take.
func checkTree(s *spec.Spec) (*mergeTree, error) {
	if s.Root == nil || s.Root.Kind != yaml.MappingNode {
		return nil, refuse("%s holds no specification to merge", s.Dir)
	}
	if source.Str(source.Child(source.Child(s.Root, "info"), "tracksOrigin")) != "true" {
		return nil, refuse("%s does not track origin (info.tracksOrigin); merge takes the trees the readers write, in which every element says where it was read", s.Dir)
	}
	switch {
	case source.Child(s.Root, "tests") != nil:
		return nil, refuse("%s holds tests, which no reader writes; merge the trees the readers wrote, then derive the tests", s.Dir)
	case len(s.Implementations) > 0:
		return nil, refuse("%s holds an implementation file, which no reader writes; merge the trees the readers wrote, then write the implementation file", s.Dir)
	case len(s.Records) > 0:
		return nil, refuse("%s has records beside it, which no reader writes; merge the trees the readers wrote", s.Dir)
	}
	dir, err := absolute(s.Dir)
	if err != nil {
		return nil, err
	}
	t := &mergeTree{s: s, dir: dir, title: source.Str(source.Child(source.Child(s.Root, "info"), "title")), side: codeSide}
	sources := source.Pairs(source.Child(s.Root, "sources"))
	if len(sources) == 0 {
		t.side = documentsSide
	}
	for _, p := range sources {
		if source.Str(source.Child(p.Value, "kind")) != "code" {
			t.side = documentsSide
		}
	}
	return t, nil
}

// --- Sources ---------------------------------------------------------------

// declared is one tree's declaration of a source.
type declared struct {
	tree int
	node *yaml.Node
	path string // the url as a folder on this machine, or "" when it is not one
}

// mergeSources joins the sources of every tree by key.
func (m *merger) mergeSources() error {
	byKey := map[string][]declared{}
	for i, t := range m.trees {
		for _, p := range source.Pairs(source.Child(t.s.Root, "sources")) {
			if byKey[p.Key.Value] == nil {
				m.sourceKeys = append(m.sourceKeys, p.Key.Value)
			}
			byKey[p.Key.Value] = append(byKey[p.Key.Value], declared{tree: i, node: p.Value, path: localURL(t.dir, source.Str(source.Child(p.Value, "url")))})
		}
	}
	for _, key := range m.sourceKeys {
		n, err := m.joinSource(key, byKey[key])
		if err != nil {
			return err
		}
		m.sources[key] = n
		m.given[key] = source.Str(source.Child(n, "givenOutside")) == "true"
	}
	return nil
}

// localURL is a source's url as a folder or file on this machine, when it
// is a relative path that exists there, and "" otherwise.
func localURL(treeDir, url string) string {
	if url == "" || strings.Contains(url, "://") {
		return ""
	}
	p := url
	if !filepath.IsAbs(p) {
		p = filepath.Join(treeDir, filepath.FromSlash(url))
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return ""
	}
	return resolved
}

// joinSource joins the declarations of one source key.
func (m *merger) joinSource(key string, ds []declared) (*yaml.Node, error) {
	first := ds[0]
	editions := map[string]bool{}
	for _, d := range ds {
		editions[source.Str(source.Child(d.node, "edition"))] = true
		for _, p := range source.Pairs(d.node) {
			switch k := p.Key.Value; k {
			case "clauses", "edition", "url":
			default:
				if !sameNode(p.Value, source.Child(first.node, k)) {
					return nil, m.differ(key, first, d, k)
				}
			}
		}
		for _, p := range source.Pairs(first.node) {
			if source.Child(d.node, p.Key.Value) == nil {
				return nil, m.differ(key, first, d, p.Key.Value)
			}
		}
		firstURL, url := source.Str(source.Child(first.node, "url")), source.Str(source.Child(d.node, "url"))
		if first.path != d.path || first.path == "" && firstURL != url {
			return nil, m.differ(key, first, d, "url")
		}
	}
	edition := source.Str(source.Child(first.node, "edition"))
	if len(editions) > 1 {
		if source.Str(source.Child(first.node, "kind")) != "code" || first.path == "" {
			return nil, m.differ(key, first, ds[1], "edition")
		}
		newest, err := m.newestEdition(key, ds)
		if err != nil {
			return nil, err
		}
		edition = newest
	}
	n := &yaml.Node{Kind: yaml.MappingNode}
	for _, p := range source.Pairs(first.node) {
		switch p.Key.Value {
		case "edition":
			set(n, "edition", edition)
		case "url":
			url := source.Str(p.Value)
			if first.path != "" {
				rel, err := relativeURL(m.out, first.path)
				if err != nil {
					return nil, err
				}
				url = rel
			}
			set(n, "url", url)
		case "clauses":
			set(n, "clauses", joinClauses(ds))
		default:
			set(n, p.Key.Value, copyNode(p.Value))
		}
	}
	return n, nil
}

func (m *merger) differ(key string, a, b declared, what string) error {
	return refuse("%s and %s declare the source %s differently (%s); give each source its own key, such as with extract --source-key, or read them again so that they agree", m.trees[a.tree].s.Dir, m.trees[b.tree].s.Dir, key, what)
}

// joinClauses is every tree's clauses of a source, each once, in the order
// of the clause; the first title given is kept.
func joinClauses(ds []declared) []*yaml.Node {
	seen := map[string]*yaml.Node{}
	var names []string
	for _, d := range ds {
		for _, c := range source.Items(source.Child(d.node, "clauses")) {
			name := source.Str(source.Child(c, "clause"))
			if seen[name] == nil {
				seen[name] = c
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	out := make([]*yaml.Node, 0, len(names))
	for _, name := range names {
		out = append(out, copyNode(seen[name]))
	}
	return out
}

// newestEdition is the edition of a code source read at several commits:
// the one every other is an ancestor of, once every clause path of each
// tree is found unchanged from that tree's commit up to it.
func (m *merger) newestEdition(key string, ds []declared) (string, error) {
	repo := ds[0].path
	shallow, err := git(repo, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return "", refuse("the source %s is read from %s, which is not a git repository here (%v)", key, repo, err)
	}
	if shallow == "true" {
		return "", refuse("the repository of the source %s is a shallow clone, whose history cannot say which commit is the newest; fetch its full history (git fetch --unshallow)", key)
	}
	var commits []string
	seen := map[string]bool{}
	for _, d := range ds {
		c := source.Str(source.Child(d.node, "edition"))
		if !fullHash.MatchString(c) {
			return "", refuse("%s declares the source %s at %q, which is not a full commit hash, so the commits the trees read cannot be compared", m.trees[d.tree].s.Dir, key, c)
		}
		if _, err := git(repo, "cat-file", "-e", c+"^{commit}"); err != nil {
			return "", refuse("%s read the source %s at commit %s, which is not in the repository's history", m.trees[d.tree].s.Dir, key, c)
		}
		if !seen[c] {
			seen[c] = true
			commits = append(commits, c)
		}
	}
	newest := ""
	for _, c := range commits {
		all := true
		for _, o := range commits {
			if a, err := isAncestor(repo, o, c); err != nil {
				return "", err
			} else if !a {
				all = false
				break
			}
		}
		if all {
			newest = c
			break
		}
	}
	if newest == "" {
		return "", refuse("the trees read the source %s at commits %s, none of which holds all the others, as on two branches; read them again from one commit", key, joinAnd(commits))
	}
	var older []string
	for _, d := range ds {
		c := source.Str(source.Child(d.node, "edition"))
		if c == newest {
			continue
		}
		for _, cl := range source.Items(source.Child(d.node, "clauses")) {
			path := source.Str(source.Child(cl, "clause"))
			last, err := git(repo, "log", "-1", "--format=%H", newest, "--", path)
			if err != nil {
				return "", err
			}
			if last == "" {
				return "", refuse("%s cites %s of the source %s, which is not a path in the repository at commit %s", m.trees[d.tree].s.Dir, path, key, newest)
			}
			if a, err := isAncestor(repo, last, c); err != nil {
				return "", err
			} else if !a {
				return "", refuse("%s read %s at commit %s, and commit %s changed it since, up to %s, the newest commit the trees read; extract it again", m.trees[d.tree].s.Dir, path, c, last, newest)
			}
		}
		older = append(older, fmt.Sprintf("%s (%s)", c, m.trees[d.tree].title))
	}
	m.res.say("source %s: one repository read at %d commits; the merged edition is %s, the newest, and every path read at %s is unchanged up to it", key, len(commits), newest, joinAnd(older))
	return newest, nil
}

// isAncestor says whether commit a is an ancestor of commit b, or b itself.
func isAncestor(repo, a, b string) (bool, error) {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", a, b)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "LC_ALL=C")
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git merge-base --is-ancestor %s %s: %v", a, b, err)
}

// --- Elements --------------------------------------------------------------

// sectionNames are the sections merged as named entries, in the order the
// files hold them.
func sectionNames() []string {
	var names []string
	for _, stage := range spec.Stages {
		names = append(names, spec.SectionsOf(stage)...)
	}
	return names
}

// single reports a section that is one object, not a mapping of named ones.
func single(section string) bool {
	switch section {
	case "release", "rollback", "signoff", "accessibility", "theme":
		return true
	}
	return false
}

// mergeTree adds one tree's entries to the merged specification.
func (m *merger) mergeTree(ti int, t *mergeTree) {
	for _, section := range sectionNames() {
		value := source.Child(t.s.Root, section)
		if value == nil {
			continue
		}
		if single(section) {
			m.mergeEntry(ti, t, section, "", source.Key(t.s.Root, section), value)
			continue
		}
		for _, p := range source.Pairs(value) {
			m.mergeEntry(ti, t, section, p.Key.Value, p.Key, p.Value)
		}
	}
}

// fileOf is the file a tree's node is written in, from the tree's root; a
// section the tree kept in its root file goes to a file of its stage.
func fileOf(t *mergeTree, n *yaml.Node, section string) string {
	rel, err := filepath.Rel(t.s.Dir, t.s.Files[n])
	if err != nil || rel == spec.RootFile || strings.HasPrefix(rel, "..") {
		return spec.Sections[section] + "/" + section + ".yaml"
	}
	return filepath.ToSlash(rel)
}

func (m *merger) mergeEntry(ti int, t *mergeTree, section, name string, key, value *yaml.Node) {
	id := section + "/" + name
	m.register(ti, t, section, name, value)
	acc := m.entries[id]
	if acc == nil {
		acc = copyNode(value)
		m.entries[id] = acc
		file := fileOf(t, key, section)
		f := m.files[file]
		if f == nil {
			f = &yaml.Node{Kind: yaml.MappingNode}
			m.files[file] = f
		}
		if single(section) {
			set(f, section, acc)
			return
		}
		sec := source.Child(f, section)
		if sec == nil {
			sec = &yaml.Node{Kind: yaml.MappingNode}
			set(f, section, sec)
		}
		set(sec, name, acc)
		return
	}
	tokens := []mergeStep{{key: section}}
	if !single(section) {
		tokens = append(tokens, mergeStep{key: name})
	}
	m.mergeNode(acc, value, tokens)
}

// register records which elements a tree has under one entry.
func (m *merger) register(ti int, t *mergeTree, section, name string, value *yaml.Node) {
	add := func(tokens []string, parent string) {
		ptr := source.Pointer(tokens...)
		e := m.elements[ptr]
		if e == nil {
			e = &element{tokens: tokens, section: section, parent: parent}
			m.elements[ptr] = e
			m.order = append(m.order, ptr)
		}
		e.trees = append(e.trees, ti)
	}
	switch {
	case single(section):
		add([]string{section}, "")
	case section == "paths":
		for _, method := range pathMethods {
			if source.Child(value, method) != nil {
				add([]string{section, name, method}, "")
			}
		}
	default:
		add([]string{section, name}, "")
		if section == "entities" {
			parent := source.Pointer(section, name)
			for _, f := range source.Pairs(source.Child(value, "properties")) {
				add([]string{section, name, "properties", f.Key.Value}, parent)
			}
		}
	}
}

func stepsKey(steps []mergeStep) string {
	var b strings.Builder
	for _, s := range steps {
		b.WriteByte('/')
		if s.name {
			b.WriteString("name=")
		}
		b.WriteString(source.EscapeToken(s.key))
	}
	return b.String()
}

// mergeNode merges a later tree's mapping into the merged one.
func (m *merger) mergeNode(acc, add *yaml.Node, at []mergeStep) {
	acc, add = source.Deref(acc), source.Deref(add)
	// An entity's required fields are joined against the fields it had
	// before this tree's were added.
	var before []string
	for _, p := range source.Pairs(source.Child(acc, "properties")) {
		before = append(before, p.Key.Value)
	}
	for _, p := range source.Pairs(add) {
		k := p.Key.Value
		here := append(append([]mergeStep{}, at...), mergeStep{key: k})
		if m.disputes[stepsKey(here)] != nil {
			continue
		}
		a := source.Child(acc, k)
		switch {
		case k == "cites":
			if a == nil {
				set(acc, k, copyNode(p.Value))
				continue
			}
			for _, c := range source.Items(p.Value) {
				if !containsNode(a, c) {
					a.Content = append(a.Content, copyNode(c))
				}
			}
		case k == "origin" || k == "why":
		case k == "required" && len(at) == 2 && at[0].key == "entities":
			m.mergeRequired(acc, add, at, before)
		case a == nil:
			set(acc, k, copyNode(p.Value))
		case a.Kind == yaml.MappingNode && p.Value.Kind == yaml.MappingNode:
			m.mergeNode(a, p.Value, here)
		case a.Kind == yaml.SequenceNode && p.Value.Kind == yaml.SequenceNode && namedItems(a) && namedItems(p.Value):
			for _, item := range source.Items(p.Value) {
				name := source.Str(source.Child(item, "name"))
				step := append(append([]mergeStep{}, here...), mergeStep{key: name, name: true})
				if mine := itemNamed(a, name); mine != nil {
					m.mergeNode(mine, item, step)
				} else {
					a.Content = append(a.Content, copyNode(item))
				}
			}
		case !sameNode(a, p.Value):
			m.disputes[stepsKey(here)] = here
			m.disputeOrd = append(m.disputeOrd, stepsKey(here))
		}
	}
	m.mergeOrigin(acc, add)
}

// mergeRequired joins the required fields of an entity field by field: a
// field only one tree has keeps what that tree says, and a field both have
// that one requires and the other does not is a disagreement, asked about
// on the field.
func (m *merger) mergeRequired(acc, add *yaml.Node, at []mergeStep, before []string) {
	mine, theirs := source.Child(acc, "required"), source.Child(add, "required")
	addProps := source.Child(add, "properties")
	listed := func(seq *yaml.Node, name string) bool {
		for _, item := range source.Items(seq) {
			if item.Value == name {
				return true
			}
		}
		return false
	}
	names := append([]string{}, before...)
	for _, p := range source.Pairs(addProps) {
		if !contains(names, p.Key.Value) {
			names = append(names, p.Key.Value)
		}
	}
	required := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, name := range names {
		inMine, inTheirs := contains(before, name), source.Child(addProps, name) != nil
		here := append(append([]mergeStep{}, at...), mergeStep{key: "properties"}, mergeStep{key: name}, mergeStep{key: "required"})
		switch {
		case m.disputes[stepsKey(here)] != nil:
		case inMine && inTheirs && listed(mine, name) != listed(theirs, name):
			m.disputes[stepsKey(here)] = here
			m.disputeOrd = append(m.disputeOrd, stepsKey(here))
		case inMine && listed(mine, name), !inMine && listed(theirs, name), inMine && inTheirs && listed(theirs, name):
			required.Content = append(required.Content, str(name))
		}
	}
	if len(required.Content) == 0 {
		deleteKey(acc, "required")
		return
	}
	setKey(acc, "required", required)
}

// mergeOrigin keeps the strongest origin: decided over stated over
// inferred, with the why of the tree that gave it.
func (m *merger) mergeOrigin(acc, add *yaml.Node) {
	rank := map[string]int{"inferred": 1, "stated": 2, "decided": 3}
	mine, theirs := source.Str(source.Child(acc, "origin")), source.Str(source.Child(add, "origin"))
	if rank[theirs] <= rank[mine] {
		if source.Child(acc, "why") == nil && theirs == mine && source.Child(add, "why") != nil {
			set(acc, "why", copyNode(source.Child(add, "why")))
		}
		return
	}
	setKey(acc, "origin", str(theirs))
	if w := source.Child(add, "why"); w != nil {
		setKey(acc, "why", copyNode(w))
	} else {
		deleteKey(acc, "why")
	}
}

// removeDisputed leaves out every key the trees disagree on.
func (m *merger) removeDisputed() {
	for _, k := range m.disputeOrd {
		steps := m.disputes[k]
		parent := m.walk(steps[:len(steps)-1])
		deleteKey(parent, steps[len(steps)-1].key)
	}
}

// walk follows steps through the merged entries.
func (m *merger) walk(steps []mergeStep) *yaml.Node {
	var n *yaml.Node
	rest := steps
	if single(steps[0].key) {
		n, rest = m.entries[steps[0].key+"/"], steps[1:]
	} else {
		n, rest = m.entries[steps[0].key+"/"+steps[1].key], steps[2:]
	}
	return follow(n, rest)
}

// follow follows steps from a node; nil when one is missing.
func follow(n *yaml.Node, steps []mergeStep) *yaml.Node {
	for _, s := range steps {
		if n == nil {
			return nil
		}
		if s.name {
			n = itemNamed(n, s.key)
		} else {
			n = source.Child(n, s.key)
		}
	}
	return n
}

// pointerOf is the JSON pointer of steps in the merged specification,
// with each named item at its index there.
func (m *merger) pointerOf(steps []mergeStep) string {
	var tokens []string
	var n *yaml.Node
	for i, s := range steps {
		switch {
		case i == 0:
			tokens = append(tokens, s.key)
			if single(s.key) {
				n = m.entries[s.key+"/"]
			}
		case i == 1 && !single(steps[0].key):
			tokens = append(tokens, s.key)
			n = m.entries[steps[0].key+"/"+s.key]
		case s.name:
			idx := 0
			for j, item := range source.Items(n) {
				if source.Str(source.Child(item, "name")) == s.key {
					idx = j
				}
			}
			tokens = append(tokens, strconv.Itoa(idx))
			n = itemNamed(n, s.key)
		default:
			tokens = append(tokens, s.key)
			n = source.Child(n, s.key)
		}
	}
	return "#" + source.Pointer(tokens...)
}

// treeSteps reads a pointer's tokens in one tree as steps, naming the item
// of a list of named objects rather than its index.
func treeSteps(root *yaml.Node, tokens []string) []mergeStep {
	var steps []mergeStep
	n := root
	for _, t := range tokens {
		if n != nil && n.Kind == yaml.SequenceNode && namedItems(n) {
			item := source.Child(n, t)
			if item != nil {
				name := source.Str(source.Child(item, "name"))
				steps = append(steps, mergeStep{key: name, name: true})
				n = item
				continue
			}
		}
		steps = append(steps, mergeStep{key: t})
		n = source.Child(n, t)
	}
	return steps
}

func namedItems(n *yaml.Node) bool {
	items := source.Items(n)
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if item.Kind != yaml.MappingNode || source.Str(source.Child(item, "name")) == "" {
			return false
		}
	}
	return true
}

func itemNamed(n *yaml.Node, name string) *yaml.Node {
	for _, item := range source.Items(n) {
		if source.Str(source.Child(item, "name")) == name {
			return item
		}
	}
	return nil
}

// --- Questions -------------------------------------------------------------

// treeQuestions takes every tree's questions, in the order the trees are
// given, with their pointers following the merged lists. A question whose
// every blocked key another tree gives is left out: that tree answers it
// (ADR-049).
func (m *merger) treeQuestions() {
	for ti, t := range m.trees {
		for _, p := range source.Pairs(source.Child(t.s.Root, spec.QuestionsSection)) {
			q := copyNode(p.Value)
			blocks := source.Items(source.Child(q, "blocks"))
			var givers []string
			answered := len(blocks) > 0
			for _, b := range blocks {
				parsed, ok := spec.ParseBlock(b.Value)
				if !ok || !parsed.IsPointer() {
					answered = false
					continue
				}
				steps := treeSteps(t.s.Root, parsed.Tokens)
				b.Value = m.pointerOf(steps)
				by := m.giversOf(ti, steps)
				answered = answered && len(by) > 0
				for _, g := range by {
					if !contains(givers, g) {
						givers = append(givers, g)
					}
				}
			}
			if answered {
				var ptrs []string
				for _, b := range blocks {
					ptrs = append(ptrs, b.Value)
				}
				m.res.say("answered: %s of %s, on %s, is left out; %s %s it", p.Key.Value, t.s.Dir, strings.Join(ptrs, ", "), joinAnd(givers), giveOrGives(len(givers)))
				continue
			}
			file := fileOf(t, p.Key, "")
			if file == "/.yaml" {
				file = questionFile(q)
			}
			m.addQuestion(q, file)
		}
	}
}

// giversOf names the trees other than the asking one that give the key at
// steps, when the asking tree does not and the merged tree holds it.
func (m *merger) giversOf(asking int, steps []mergeStep) []string {
	if len(steps) < 3 || follow(m.trees[asking].s.Root, steps) != nil || m.walk(steps) == nil {
		return nil
	}
	var out []string
	for ti, t := range m.trees {
		if ti == asking {
			continue
		}
		if follow(t.s.Root, steps) != nil {
			out = append(out, t.title)
		}
	}
	return out
}

func giveOrGives(n int) string {
	if n == 1 {
		return "gives"
	}
	return "give"
}

// questionFile is the questions file of the stage a question blocks.
func questionFile(q *yaml.Node) string {
	stage := "requirements"
	if items := source.Items(source.Child(q, "blocks")); len(items) > 0 {
		if b, ok := spec.ParseBlock(items[0].Value); ok {
			stage = b.Stage
		}
	}
	return stage + "/" + spec.QuestionsSection + ".yaml"
}

func (m *merger) addQuestion(q *yaml.Node, file string) {
	m.questions = append(m.questions, q)
	m.qfiles = append(m.qfiles, file)
}

// citesAt is the citations of the nearest element at or above steps in a
// tree.
func citesAt(root *yaml.Node, steps []mergeStep) []*yaml.Node {
	cites := citesOn(root, steps)
	if len(cites) == 0 && len(steps) > 1 && steps[0].key == "paths" {
		// A key of the path item itself: the operations under it cite it.
		item := source.Child(source.Child(root, "paths"), steps[1].key)
		for _, method := range pathMethods {
			cites = append(cites, source.Items(source.Child(source.Child(item, method), "cites"))...)
		}
	}
	return cites
}

func citesOn(root *yaml.Node, steps []mergeStep) []*yaml.Node {
	var cites []*yaml.Node
	n := root
	for _, s := range steps {
		if n == nil {
			break
		}
		if s.name {
			n = itemNamed(n, s.key)
		} else {
			n = source.Child(n, s.key)
		}
		if c := source.Child(n, "cites"); c != nil && n != nil && n.Kind == yaml.MappingNode {
			cites = source.Items(c)
		}
	}
	return cites
}

// disputeQuestions asks one must question per key the trees disagree on.
func (m *merger) disputeQuestions() int {
	for _, k := range m.disputeOrd {
		steps := m.disputes[k]
		ptr := m.pointerOf(steps)
		// A field one tree requires and another does not: the entity's
		// required list is what is blocked.
		field := ""
		if n := len(steps); n == 5 && steps[0].key == "entities" && steps[2].key == "properties" && steps[4].key == "required" {
			field = steps[3].key
			ptr = m.pointerOf(append(append([]mergeStep{}, steps[:2]...), mergeStep{key: "required"}))
		}
		var values []string
		var options []string
		cites := &yaml.Node{Kind: yaml.SequenceNode}
		for _, t := range m.trees {
			v := follow(t.s.Root, steps)
			text := ""
			switch {
			case field != "":
				if follow(t.s.Root, steps[:4]) == nil {
					continue
				}
				text = field + " not required"
				for _, item := range source.Items(follow(t.s.Root, append(append([]mergeStep{}, steps[:2]...), mergeStep{key: "required"}))) {
					if item.Value == field {
						text = field + " required"
					}
				}
			case v == nil:
				continue
			default:
				text = inline(v)
			}
			found := false
			for i, have := range values {
				if have == text {
					found = true
					options[i] = options[i] + "; " + t.title
				}
			}
			if !found {
				values = append(values, text)
				options = append(options, t.title)
			}
			for _, c := range citesAt(t.s.Root, steps) {
				if !containsNode(cites, c) {
					cites.Content = append(cites.Content, copyNode(c))
				}
			}
		}
		var said []string
		var opts []string
		for i, v := range values {
			said = append(said, fmt.Sprintf("%s says %s", options[i], v))
			opts = append(opts, fmt.Sprintf("%s, as %s has it", v, options[i]))
		}
		q := mapping(
			"question", fmt.Sprintf("Which is right for %s: %s?", ptr, strings.Join(values, " or ")),
			"kind", "decision",
			"priority", "must",
			"blocks", []string{ptr},
			"decidedBy", owner,
			"options", opts,
			"why", fmt.Sprintf("The sources disagree: %s. The merge leaves the key out rather than choose between them.", strings.Join(said, ", and ")),
		)
		if len(cites.Content) > 0 {
			set(q, "cites", cites)
		}
		m.addQuestion(q, questionFile(q))
		m.res.say("question Q-%d (must): %s: the trees disagree", len(m.questions), ptr)
	}
	return len(m.disputeOrd)
}

// inline writes a value on one line, as YAML's flow style does.
func inline(n *yaml.Node) string {
	c := copyNode(n)
	flowAll(c)
	text, err := encode(c)
	if err != nil {
		return n.Value
	}
	return strings.TrimSpace(string(text))
}

func flowAll(n *yaml.Node) {
	if n.Kind == yaml.MappingNode || n.Kind == yaml.SequenceNode {
		n.Style = yaml.FlowStyle
	} else if n.Style == yaml.LiteralStyle || n.Style == yaml.FoldedStyle {
		n.Style = yaml.DoubleQuotedStyle
	}
	for _, c := range n.Content {
		flowAll(c)
	}
}

// sideQuestions applies the rows of docs/from-sources.md, section 3.2, for
// an element only one side has, where both sides are merged.
func (m *merger) sideQuestions() int {
	speaks := map[string]map[string]bool{codeSide: {}, documentsSide: {}} // side -> section or entity pointer
	for _, ptr := range m.order {
		e := m.elements[ptr]
		for _, ti := range e.trees {
			side := m.trees[ti].side
			speaks[side][e.section] = true
			if e.section == "entities" && e.parent == "" {
				speaks[side][ptr] = true
			}
		}
	}
	if len(speaks[codeSide]) == 0 || len(speaks[documentsSide]) == 0 {
		return 0
	}
	asked := 0
	for _, ptr := range m.order {
		e := m.elements[ptr]
		if spec.Sections[e.section] != "design" || e.section == "decisions" {
			continue
		}
		sides := map[string]bool{}
		for _, ti := range e.trees {
			sides[m.trees[ti].side] = true
		}
		if len(sides) != 1 {
			continue
		}
		side := codeSide
		other := documentsSide
		if sides[documentsSide] {
			side, other = documentsSide, codeSide
		}
		about := e.section
		if e.parent != "" {
			about = e.parent
		}
		if !speaks[other][about] {
			continue
		}
		node := m.elementNode(e)
		priority := "should"
		if m.security(e, node) || m.givenOutside(node) {
			priority = "must"
		}
		label := m.label(e, node)
		if side == codeSide {
			m.undocumented(e, node, label, priority)
		} else {
			m.notBuilt(e, node, label, priority)
		}
		asked++
	}
	return asked
}

// elementNode is an element's merged node.
func (m *merger) elementNode(e *element) *yaml.Node {
	var steps []mergeStep
	for _, t := range e.tokens {
		steps = append(steps, mergeStep{key: t})
	}
	return m.walk(steps)
}

// security says whether an element concerns security: a role, a
// permission, an operation, or an entity or field that is personal or a
// credential.
func (m *merger) security(e *element, n *yaml.Node) bool {
	switch e.section {
	case "roles", "permissions", "paths":
		return true
	case "entities":
		if e.parent != "" {
			return sensitive(n)
		}
		for _, f := range source.Pairs(source.Child(n, "properties")) {
			if sensitive(f.Value) {
				return true
			}
		}
	}
	return false
}

func sensitive(field *yaml.Node) bool {
	switch source.Str(source.Child(field, "sensitivity")) {
	case "personal", "credential":
		return true
	}
	return false
}

// givenOutside says whether an element cites a source given to parties
// outside.
func (m *merger) givenOutside(n *yaml.Node) bool {
	for _, c := range source.Items(source.Child(n, "cites")) {
		if m.given[source.Str(source.Child(c, "source"))] {
			return true
		}
	}
	return false
}

// The word for one entry of each section, for a question's text.
var entryWords = map[string]string{
	"enums": "enum", "entities": "entity", "views": "view", "permissions": "permission", "roles": "role",
	"commands": "command", "channels": "channel", "dependencies": "dependency", "jobs": "job", "errors": "error",
	"pages": "page", "menus": "menu", "flows": "flow", "algorithms": "algorithm",
}

func (m *merger) label(e *element, n *yaml.Node) string {
	switch {
	case e.section == "paths":
		return fmt.Sprintf("operation %s (%s %s)", source.Str(source.Child(n, "operationId")), strings.ToUpper(e.tokens[2]), e.tokens[1])
	case e.parent != "":
		return fmt.Sprintf("field %s of entity %s", e.tokens[3], e.tokens[1])
	case single(e.section):
		return "the " + e.section
	case entryWords[e.section] != "":
		return entryWords[e.section] + " " + e.tokens[1]
	}
	return e.section + " " + e.tokens[1]
}

// clausesOf lists the clauses an element's citations name.
func clausesOf(n *yaml.Node) []string {
	var out []string
	for _, c := range source.Items(source.Child(n, "cites")) {
		cl := source.Str(source.Child(c, "clause"))
		if cl != "" && !contains(out, cl) {
			out = append(out, cl)
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// undocumented writes an element only the code has as inferred, and asks
// the owner to confirm it.
func (m *merger) undocumented(e *element, n *yaml.Node, label, priority string) {
	where := joinAnd(clausesOf(n))
	if where == "" {
		where = "a code tree"
	}
	setKey(n, "origin", str("inferred"))
	setKey(n, "why", str(fmt.Sprintf("Undocumented, from code. The code has it at %s, and no document merged here mentions it.", where)))
	ptr := "#" + source.Pointer(e.tokens...)
	q := mapping(
		"question", fmt.Sprintf("Undocumented, from code: is %s meant to be part of the system? The code has it at %s, and no document mentions it.", label, where),
		"kind", "decision",
		"priority", priority,
		"blocks", []string{ptr},
		"decidedBy", owner,
		"options", []string{"Yes: the documents are to describe it", "No: the code is to change"},
		"why", m.priorityWhy(e, n, priority, "Only the code has it"),
	)
	if c := source.Child(n, "cites"); c != nil {
		set(q, "cites", copyNode(c))
	}
	m.addQuestion(q, questionFile(q))
	m.res.say("question Q-%d (%s): %s: undocumented, from code", len(m.questions), priority, ptr)
}

// notBuilt keeps an element only the documents have as they state it, and
// asks in the implementation stage whether it is to be built.
func (m *merger) notBuilt(e *element, n *yaml.Node, label, priority string) {
	cites := copyNode(source.Child(n, "cites"))
	if cites == nil {
		cites = &yaml.Node{Kind: yaml.SequenceNode}
	}
	about := e.section
	if e.parent != "" {
		// A field: the entity's citations say where the documents have it
		// and where the code read it.
		about = "entity " + e.tokens[1]
		for _, ti := range m.elements[e.parent].trees {
			for _, c := range source.Items(source.Child(source.Child(source.Child(m.trees[ti].s.Root, "entities"), e.tokens[1]), "cites")) {
				if !containsNode(cites, c) {
					cites.Content = append(cites.Content, copyNode(c))
				}
			}
		}
	} else {
		for _, ptr := range m.order {
			other := m.elements[ptr]
			if other.section != e.section {
				continue
			}
			for _, ti := range other.trees {
				if m.trees[ti].side != codeSide {
					continue
				}
				for _, c := range codeClauses(m.trees[ti]) {
					if !containsNode(cites, c) {
						cites.Content = append(cites.Content, c)
					}
				}
			}
		}
	}
	q := mapping(
		"question", fmt.Sprintf("Not built yet. The documents have %s, and the code read for %s does not: is it to be built, or are the documents to change?", label, about),
		"kind", "decision",
		"priority", priority,
		"blocks", []string{"implementation"},
		"decidedBy", owner,
		"options", []string{"Built: the code is to add it", "Dropped: the documents are to change"},
		"why", m.priorityWhy(e, n, priority, "Only the documents have it"),
	)
	if len(cites.Content) > 0 {
		set(q, "cites", flowItems(cites))
	}
	m.addQuestion(q, "implementation/"+spec.QuestionsSection+".yaml")
	m.res.say("question Q-%d (%s): #%s: not built yet", len(m.questions), priority, source.Pointer(e.tokens...))
}

// codeClauses cites what a code tree read, as where the element would be.
func codeClauses(t *mergeTree) []*yaml.Node {
	var out []*yaml.Node
	for _, p := range source.Pairs(source.Child(t.s.Root, "sources")) {
		for _, c := range source.Items(source.Child(p.Value, "clauses")) {
			out = append(out, citation(p.Key.Value, source.Str(source.Child(c, "clause")), "Read for this tree; what the documents have is not here."))
		}
	}
	return out
}

func flowItems(n *yaml.Node) *yaml.Node {
	for _, c := range n.Content {
		c.Style = yaml.FlowStyle
	}
	return n
}

func (m *merger) priorityWhy(e *element, n *yaml.Node, priority, only string) string {
	if priority == "should" {
		return only + ", and it concerns neither security nor a source given outside, so the owner confirms it before it is built on."
	}
	if m.givenOutside(n) {
		return only + ", and it cites a source given to parties outside, whose promise is not the project's alone to change."
	}
	return only + ", and it concerns security, where an element nobody documented or built is how an open endpoint or an exposed field is found."
}

// --- Writing ---------------------------------------------------------------

func (m *merger) write(asked int) (*Result, error) {
	count, shared := 0, 0
	for _, ptr := range m.order {
		e := m.elements[ptr]
		if e.parent != "" {
			continue
		}
		count++
		if len(e.trees) > 1 {
			shared++
		}
	}
	var lines []string
	for _, t := range m.trees {
		n, q := 0, len(source.Pairs(source.Child(t.s.Root, spec.QuestionsSection)))
		for _, ptr := range m.order {
			e := m.elements[ptr]
			if e.parent == "" && slicesContains(e.trees, m.indexOf(t)) {
				n++
			}
		}
		lines = append(lines, fmt.Sprintf("tree %s: %s, on the %s side: %s and %s", t.s.Dir, t.title, t.side, plural(n, "element"), plural(q, "question")))
	}
	m.res.Lines = append(lines, m.res.Lines...)
	m.res.say("merged %s, each an entry of a section or an operation: %d in more than one tree, and %s the trees disagree on", plural(count, "element"), shared, plural(len(m.disputeOrd), "key"))
	m.res.say("wrote %s: %d from the trees, numbered again in the order the trees are given, and %d the merge asked", plural(len(m.questions), "question"), len(m.questions)-asked, asked)
	// The merge's own lines name questions by number; they come last.
	var qlines, rest []string
	for _, l := range m.res.Lines {
		if strings.HasPrefix(l, "question ") {
			qlines = append(qlines, l)
		} else {
			rest = append(rest, l)
		}
	}
	m.res.Lines = append(rest, qlines...)

	if asked > 0 && m.entries["stakeholders/"+owner] == nil {
		file := "requirements/stakeholders.yaml"
		f := m.files[file]
		if f == nil {
			f = &yaml.Node{Kind: yaml.MappingNode}
			m.files[file] = f
		}
		sec := source.Child(f, "stakeholders")
		if sec == nil {
			sec = &yaml.Node{Kind: yaml.MappingNode}
			f.Content = append([]*yaml.Node{str("stakeholders"), sec}, f.Content...)
		}
		sec.Content = append(sec.Content, ownerStakeholder().Content...)
	}
	for i, q := range m.questions {
		f := m.files[m.qfiles[i]]
		if f == nil {
			f = &yaml.Node{Kind: yaml.MappingNode}
			m.files[m.qfiles[i]] = f
		}
		sec := source.Child(f, spec.QuestionsSection)
		if sec == nil {
			sec = &yaml.Node{Kind: yaml.MappingNode}
			set(f, spec.QuestionsSection, sec)
		}
		set(sec, fmt.Sprintf("Q-%d", i+1), q)
	}
	for file, n := range m.files {
		m.stages[strings.SplitN(file, "/", 2)[0]] = true
		m.res.Tree.put(file, orderSections(n))
	}
	var stages []string
	for _, s := range spec.Stages {
		if m.stages[s] {
			stages = append(stages, s)
		}
	}
	var titles []string
	for _, t := range m.trees {
		titles = append(titles, t.title)
	}
	sources := &yaml.Node{Kind: yaml.MappingNode}
	for _, k := range m.sourceKeys {
		set(sources, k, m.sources[k])
	}
	root := rootFile(fmt.Sprintf("Merge of %d trees", len(m.trees)),
		fmt.Sprintf("The elements of these trees, merged in this order: %s. Every element cites where it was read; where the trees disagree, or the documents and the code do, a question asks which is right.\n", strings.Join(titles, "; ")),
		stages, sources)
	m.res.Tree.put(spec.RootFile, root)
	return m.res, nil
}

func (m *merger) indexOf(t *mergeTree) int {
	for i, x := range m.trees {
		if x == t {
			return i
		}
	}
	return -1
}

func slicesContains(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// orderSections writes a file's sections in life-cycle order, the
// questions last.
func orderSections(f *yaml.Node) *yaml.Node {
	out := &yaml.Node{Kind: yaml.MappingNode}
	for _, name := range append(sectionNames(), spec.QuestionsSection) {
		if v := source.Child(f, name); v != nil {
			set(out, name, v)
		}
	}
	return out
}

// --- Nodes -----------------------------------------------------------------

// copyNode copies a node deeply, without the position it was read at.
func copyNode(n *yaml.Node) *yaml.Node {
	n = source.Deref(n)
	if n == nil {
		return nil
	}
	c := &yaml.Node{Kind: n.Kind, Style: n.Style, Tag: n.Tag, Value: n.Value}
	if n.Kind == yaml.ScalarNode && n.Style == 0 && n.Tag != "" && n.Tag != "!!str" {
		c.Tag = n.ShortTag()
	}
	for _, x := range n.Content {
		c.Content = append(c.Content, copyNode(x))
	}
	return c
}

// sameNode compares two values, whatever their style or position.
func sameNode(a, b *yaml.Node) bool {
	a, b = source.Deref(a), source.Deref(b)
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case yaml.ScalarNode:
		return a.ShortTag() == b.ShortTag() && a.Value == b.Value
	case yaml.MappingNode:
		pa, pb := source.Pairs(a), source.Pairs(b)
		if len(pa) != len(pb) {
			return false
		}
		for _, p := range pa {
			if !sameNode(p.Value, source.Child(b, p.Key.Value)) {
				return false
			}
		}
		return true
	case yaml.SequenceNode:
		if len(a.Content) != len(b.Content) {
			return false
		}
		for i := range a.Content {
			if !sameNode(a.Content[i], b.Content[i]) {
				return false
			}
		}
		return true
	}
	return false
}

func containsNode(seq, n *yaml.Node) bool {
	for _, c := range source.Items(seq) {
		if sameNode(c, n) {
			return true
		}
	}
	return false
}

// setKey replaces a key's value in a mapping, or adds the key.
func setKey(m *yaml.Node, key string, v *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = v
			return
		}
	}
	m.Content = append(m.Content, str(key), v)
}

func deleteKey(m *yaml.Node, key string) {
	if m == nil {
		return
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}
