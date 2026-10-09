package validate

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/idioms"
	"github.com/SpecArch/specarch/internal/semver"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/typerows"
)

// IdiomSuffix ends the name of every idiom file.
const IdiomSuffix = ".specarch-idiom.yaml"

// Idiom is one idiom file, parsed.
type Idiom struct {
	Path string // idioms/<concern>/<name>.specarch-idiom.yaml for a shipped one, the file for a project's
	Root *yaml.Node
}

// Name is the idiom's name.
func (i Idiom) Name() string { return source.Str(source.Child(i.Root, "name")) }

// Version is the idiom's version.
func (i Idiom) Version() string { return source.Str(source.Child(i.Root, "version")) }

var (
	shippedOnce sync.Once
	shipped     map[string]Idiom
)

// ShippedIdioms are the idioms this build ships, by name.
func ShippedIdioms() map[string]Idiom {
	shippedOnce.Do(func() {
		shipped = map[string]Idiom{}
		paths, _ := fs.Glob(idioms.Shipped, "*/*"+IdiomSuffix)
		for _, p := range paths {
			data, err := idioms.Shipped.ReadFile(p)
			if err != nil {
				continue
			}
			doc := source.Parse(data)
			if doc.Root == nil {
				continue
			}
			i := Idiom{Path: "idioms/" + p, Root: doc.Root}
			shipped[i.Name()] = i
		}
	})
	return shipped
}

// implementationStacks are the stacks of an implementation file: its
// language, named by its file name, the dialect of every target that has
// one (a target named sql has postgresql when it names none), and the
// framework of every ui target (plain-javascript on platform web and
// swiftui on platform iphone when it names none). at holds the node a
// problem with each stack is reported at.
func (c *checker) implementationStacks() (stacks []string, at map[string]*yaml.Node, ptr map[string]string) {
	at, ptr = map[string]*yaml.Node{}, map[string]string{}
	name := strings.TrimSuffix(filepath.Base(c.file), ImplementationSuffix)
	if i := strings.LastIndex(name, "."); i >= 0 {
		lang := name[i+1:]
		stacks = append(stacks, lang)
		at[lang], ptr[lang] = source.Key(c.root, "stack"), "/stack"
	}
	for _, t := range source.Pairs(source.Child(c.root, "targets")) {
		d := source.Child(t.Value, "dialect")
		dialect := source.Str(d)
		node := d
		if dialect == "" && t.Key.Value == "sql" {
			dialect, node = "postgresql", t.Key
		}
		if dialect == "" || at[dialect] != nil {
			continue
		}
		stacks = append(stacks, dialect)
		at[dialect] = node
		if d != nil {
			ptr[dialect] = source.Pointer("targets", t.Key.Value, "dialect")
		} else {
			ptr[dialect] = source.Pointer("targets", t.Key.Value)
		}
	}
	for _, t := range source.Pairs(source.Child(c.root, "targets")) {
		f := source.Child(t.Value, "framework")
		framework := source.Str(f)
		node, p := f, source.Pointer("targets", t.Key.Value, "framework")
		if framework == "" {
			framework = defaultFramework[source.Str(source.Child(t.Value, "platform"))]
			node, p = source.Child(t.Value, "platform"), source.Pointer("targets", t.Key.Value, "platform")
		}
		if framework == "" || at[framework] != nil {
			continue
		}
		stacks = append(stacks, framework)
		at[framework], ptr[framework] = node, p
	}
	return stacks, at, ptr
}

// defaultFramework is a ui target's framework when it names none, by its
// platform, as the implementation schema gives it.
var defaultFramework = map[string]string{"web": "plain-javascript", "iphone": "swiftui"}

// projectIdioms reads the idioms folder beside the implementation file:
// every *.specarch-idiom.yaml, checked against the idiom schema. Any other
// YAML file there is a layout problem.
func (c *checker) projectIdioms() map[string]Idiom {
	dir := filepath.Join(filepath.Dir(c.file), "idioms")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := map[string]Idiom{}
	for _, e := range entries {
		name := e.Name()
		p := filepath.Join(dir, name)
		if e.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		if !strings.HasSuffix(name, IdiomSuffix) {
			c.addFile(p, 1, "/", RuleLayout, "a file in idioms/ is an idiom named <name>%s; rename it, or move it out of the folder", IdiomSuffix)
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			c.addFile(p, 1, "/", RuleLayout, "cannot read it (%s)", spec.PlainIOError(err))
			continue
		}
		ic := &checker{file: p}
		doc := source.Parse(data)
		for _, pr := range doc.Problems {
			ic.addLine(pr.Line, pr.Path, Rule(pr.Rule), "%s", pr.Message)
		}
		if doc.Root != nil {
			ic.root = doc.Root
			ic.checkSchema(KindIdiom, doc.Value)
		}
		c.diags = append(c.diags, withoutEchoes(ic.diags)...)
		if doc.Root == nil || Errors(ic.diags) > 0 {
			continue
		}
		i := Idiom{Path: p, Root: doc.Root}
		if want := strings.TrimSuffix(name, IdiomSuffix); i.Name() != want {
			c.addFile(p, source.Key(doc.Root, "name").Line, "/name", RuleLayout, "the file is named %s but the idiom is %s; name the file after the idiom", want, i.Name())
			continue
		}
		out[i.Name()] = i
	}
	return out
}

// checkIdioms checks the idioms of an implementation file against the
// specification it implements: what the file excludes and overrides, the
// overrides themselves, and the contract of every idiom that applies.
func (c *checker) checkIdioms(s *spec.Spec) {
	stacks, at, ptr := c.implementationStacks()
	isStack := map[string]bool{"any": true}
	for _, st := range stacks {
		isStack[st] = true
	}
	shippedSet := ShippedIdioms()
	project := c.projectIdioms()
	excluded := map[string]bool{}
	override := map[string]Idiom{}

	for _, u := range source.Pairs(source.Child(c.root, "idioms")) {
		name := u.Key.Value
		base := source.Pointer("idioms", name)
		_, isShipped := shippedSet[name]
		_, isProject := project[name]
		if !isShipped && !isProject {
			c.add(u.Key, base, RuleIdiomUnknown, "%s is not an idiom SpecArch ships nor one in this file's idioms folder%s", name, suggestIdiom(name, shippedSet, project))
			continue
		}
		if source.Str(source.Child(u.Value, "exclude")) == "true" {
			excluded[name] = true
			if strings.TrimSpace(source.Str(source.Child(u.Value, "why"))) == "" {
				c.add(u.Key, base, RuleIdiomOverrideReason, "%s is excluded without why; say what the project does instead", name)
			}
			continue
		}
		file := source.Str(source.Child(u.Value, "override"))
		if file == "" {
			continue
		}
		o, ok := project[strings.TrimSuffix(path.Base(file), IdiomSuffix)]
		if !ok || source.Child(o.Root, "overrides") == nil {
			c.add(source.Child(u.Value, "override"), base+"/override", RuleIdiomUnknown, "%s is not an override in this file's idioms folder; add the file with overrides naming %s, or correct the path", file, name)
			continue
		}
		if idiom := source.Str(source.Child(source.Child(o.Root, "overrides"), "idiom")); idiom != name {
			c.add(source.Child(u.Value, "override"), base+"/override", RuleIdiomUnknown, "%s overrides %s, not %s; name it under idioms.%s, or correct its overrides", file, idiom, name, idiom)
			continue
		}
		override[name] = o
	}

	for _, name := range sortedIdiomNames(project) {
		o := project[name]
		ov := source.Child(o.Root, "overrides")
		if ov == nil {
			continue
		}
		oc := &checker{file: o.Path, root: o.Root}
		oc.checkOverride(o, ov, shippedSet, isStack)
		c.diags = append(c.diags, oc.diags...)
	}

	if s == nil || s.Root == nil {
		return
	}
	apply := map[string]Idiom{}
	for name, i := range shippedSet {
		if !excluded[name] {
			apply[name] = i
		}
	}
	for name, i := range project {
		if source.Child(i.Root, "overrides") == nil && !excluded[name] {
			apply[name] = i
		}
	}
	for _, name := range sortedIdiomNames(apply) {
		i := apply[name]
		if !idiomApplies(i, s.Root, isStack) {
			continue
		}
		o, hasOverride := override[name]
		for _, st := range stacks {
			if !rendersStack(i, st) {
				continue
			}
			for _, part := range source.Pairs(source.Child(i.Root, "parts")) {
				r := resolvedRendering(i, o, hasOverride, part.Key.Value, st)
				if r == nil || source.Child(r, "rows") == nil {
					continue
				}
				c.checkRows(i, part.Key.Value, st, r, s.Root, at[st], ptr[st])
			}
		}
	}
}

// checkOverride checks one override file against the shipped idiom it
// overrides. The receiver's file is the override's.
func (c *checker) checkOverride(o Idiom, ov *yaml.Node, shippedSet map[string]Idiom, isStack map[string]bool) {
	idiomNode := source.Child(ov, "idiom")
	shippedIdiom, ok := shippedSet[source.Str(idiomNode)]
	if !ok {
		c.add(idiomNode, "/overrides/idiom", RuleIdiomUnknown, "%s is not an idiom SpecArch ships, so there is nothing to override; drop overrides to make this the project's own idiom, or name a shipped one%s", source.Str(idiomNode), suggestIdiom(source.Str(idiomNode), shippedSet, nil))
		return
	}
	if strings.TrimSpace(source.Str(source.Child(o.Root, "why"))) == "" {
		c.add(source.Key(o.Root, "overrides"), "/overrides", RuleIdiomOverrideReason, "the override of %s has no why; say why the project renders it differently", shippedIdiom.Name())
	}
	shippedParts := source.Child(shippedIdiom.Root, "parts")
	listed := map[string]bool{}
	for i, p := range source.Items(source.Child(ov, "parts")) {
		listed[p.Value] = true
		if source.Child(shippedParts, p.Value) == nil {
			c.add(p, source.Pointer("overrides", "parts", itoa(i)), RuleIdiomPartUnknown, "%s has no part %s%s", shippedIdiom.Name(), p.Value, suggestPart(p.Value, shippedParts))
		}
	}
	whole := source.Str(source.Child(ov, "whole")) == "true"
	for _, p := range source.Pairs(source.Child(o.Root, "parts")) {
		if !whole && !listed[p.Key.Value] {
			c.add(p.Key, source.Pointer("parts", p.Key.Value), RuleIdiomPartUnknown, "the part %s is not listed under overrides.parts; list it, or remove it so the shipped one applies", p.Key.Value)
		}
		for _, st := range source.Pairs(source.Child(p.Value, "stack")) {
			if !isStack[st.Key.Value] {
				c.add(st.Key, source.Pointer("parts", p.Key.Value, "stack", st.Key.Value), RuleIdiomStack, "%s is not a stack of the implementation file; it renders %s", st.Key.Value, stackList(isStack))
			}
		}
	}
	shippedContract := source.Child(shippedIdiom.Root, "contract")
	for _, st := range source.Pairs(source.Child(o.Root, "contract")) {
		was := source.Child(shippedContract, st.Key.Value)
		if was == nil {
			continue
		}
		if source.Str(source.Child(st.Value, "statement")) != source.Str(source.Child(was, "statement")) || source.Str(source.Child(st.Value, "check")) != source.Str(source.Child(was, "check")) {
			c.add(st.Key, source.Pointer("contract", st.Key.Value), RuleIdiomContract, "%s is a contract statement of %s, which an override may not change; keep it as shipped, or exclude the idiom and write the project's own", st.Key.Value, shippedIdiom.Name())
		}
	}
	verNode := source.Child(ov, "version")
	from, ok1 := semver.Parse(source.Str(verNode))
	now, ok2 := semver.Parse(shippedIdiom.Version())
	if ok1 && ok2 && semver.Compare(from, now) < 0 && source.Str(source.Child(ov, "staysBehind")) != "true" {
		c.warn(verNode, "/overrides/version", RuleIdiomVersionBehind, "copied from %s %s, and SpecArch now ships %s; compare them with specarch idioms diff %s, then move to it, or set staysBehind: true and say why", shippedIdiom.Name(), source.Str(verNode), shippedIdiom.Version(), shippedIdiom.Name())
	}
}

// idiomApplies reports whether an idiom applies to a specification and an
// implementation file: it renders one of the file's stacks, or any, and
// the specification uses one of the keywords it reads, as a section or as a
// key anywhere inside one.
func idiomApplies(i Idiom, root *yaml.Node, isStack map[string]bool) bool {
	renders := false
	for _, st := range source.Items(source.Child(i.Root, "stacks")) {
		if isStack[st.Value] {
			renders = true
		}
	}
	if !renders {
		return false
	}
	for _, r := range source.Items(source.Child(i.Root, "reads")) {
		if usesKeyword(root, r.Value) {
			return true
		}
	}
	return false
}

// usesKeyword reports whether the design uses a keyword: a section of that
// name that is not empty, or the key anywhere inside a section.
func usesKeyword(root *yaml.Node, keyword string) bool {
	if n := source.Child(root, keyword); n != nil && len(source.Deref(n).Content) > 0 {
		return true
	}
	found := false
	walk(root, nil, func(n *yaml.Node, path []string) {
		if len(path) > 0 && source.Key(n, keyword) != nil {
			found = true
		}
	})
	return found
}

func rendersStack(i Idiom, stack string) bool {
	for _, st := range source.Items(source.Child(i.Root, "stacks")) {
		if st.Value == stack {
			return true
		}
	}
	return false
}

// resolvedRendering is the rendering of one part for one stack, by the
// lookup order: the override's part for the stack, the shipped part for
// the stack, the shipped part under any.
func resolvedRendering(i, o Idiom, hasOverride bool, part, stack string) *yaml.Node {
	if hasOverride {
		ov := source.Child(o.Root, "overrides")
		replaces := source.Str(source.Child(ov, "whole")) == "true"
		for _, p := range source.Items(source.Child(ov, "parts")) {
			replaces = replaces || p.Value == part
		}
		if replaces {
			if r := source.Child(source.Child(source.Child(source.Child(o.Root, "parts"), part), "stack"), stack); r != nil {
				return r
			}
		}
	}
	stacks := source.Child(source.Child(source.Child(i.Root, "parts"), part), "stack")
	if r := source.Child(stacks, stack); r != nil {
		return r
	}
	return source.Child(stacks, "any")
}

// checkRows reports every entity field that no row of a rendering matches.
func (c *checker) checkRows(i Idiom, part, stack string, r *yaml.Node, root *yaml.Node, at *yaml.Node, ptr string) {
	rows := typerows.RowsOf(asList(source.ValueOf(source.Child(r, "rows"))))
	for _, e := range source.Pairs(source.Child(root, "entities")) {
		for _, f := range source.Pairs(source.Child(e.Value, "properties")) {
			field, _ := source.ValueOf(f.Value).(map[string]any)
			shape := typerows.ShapeOf(field)
			if _, ok := typerows.Match(shape, rows); !ok {
				c.add(at, ptr, RuleIdiomContract, "%s %s has no %s row in its %s part for #/entities/%s/properties/%s (%s); add one in an override of the part, or exclude the idiom with the reason",
					i.Name(), i.Version(), stack, part, e.Key.Value, f.Key.Value, shape.Describe())
			}
		}
	}
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

func sortedIdiomNames(m map[string]Idiom) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func suggestIdiom(name string, sets ...map[string]Idiom) string {
	all := map[string]*yaml.Node{}
	for _, set := range sets {
		for k, v := range set {
			all[k] = v.Root
		}
	}
	return suggest(name, all)
}

func suggestPart(name string, parts *yaml.Node) string {
	all := map[string]*yaml.Node{}
	for _, p := range source.Pairs(parts) {
		all[p.Key.Value] = p.Value
	}
	return suggest(name, all)
}

func stackList(isStack map[string]bool) string {
	var out []string
	for k := range isStack {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

func itoa(n int) string { return strconv.Itoa(n) }

// IdiomUse is one idiom as an implementation file uses it.
type IdiomUse struct {
	Name, Version string
	As            string   // shipped, overridden, project or excluded
	Parts         []string // for an override: the parts it replaces, or every part
	From          string   // for an override: the shipped version it was copied from
	Override      string   // for an override: its file
	Why           string   // for an override or an exclusion: its reason
}

// IdiomUses lists the idioms an implementation file uses, in name order:
// every shipped idiom and every one of the project's own that applies,
// each as shipped, overridden or the project's, and every idiom it
// excludes. s is the specification it implements.
func IdiomUses(file string, root *yaml.Node, s *spec.Spec) []IdiomUse {
	c := &checker{file: file, root: root}
	stacks, _, _ := c.implementationStacks()
	isStack := map[string]bool{"any": true}
	for _, st := range stacks {
		isStack[st] = true
	}
	project := c.projectIdioms()
	all := map[string]Idiom{}
	for name, i := range ShippedIdioms() {
		all[name] = i
	}
	for name, i := range project {
		if source.Child(i.Root, "overrides") == nil {
			all[name] = i
		}
	}
	var out []IdiomUse
	for _, name := range sortedIdiomNames(all) {
		i := all[name]
		u := IdiomUse{Name: name, Version: i.Version(), As: "shipped"}
		if _, own := project[name]; own {
			u.As = "project"
		}
		use := source.Child(source.Child(root, "idioms"), name)
		if source.Str(source.Child(use, "exclude")) == "true" {
			u.As, u.Why = "excluded", strings.TrimSpace(source.Str(source.Child(use, "why")))
			out = append(out, u)
			continue
		}
		if s == nil || s.Root == nil || !idiomApplies(i, s.Root, isStack) {
			continue
		}
		if ref := source.Str(source.Child(use, "override")); ref != "" {
			if o, ok := project[strings.TrimSuffix(path.Base(ref), IdiomSuffix)]; ok {
				ov := source.Child(o.Root, "overrides")
				u.As, u.From, u.Override = "overridden", source.Str(source.Child(ov, "version")), o.Path
				u.Why = strings.TrimSpace(source.Str(source.Child(o.Root, "why")))
				if source.Str(source.Child(ov, "whole")) == "true" {
					for _, p := range source.Pairs(source.Child(i.Root, "parts")) {
						u.Parts = append(u.Parts, p.Key.Value)
					}
				} else {
					for _, p := range source.Items(source.Child(ov, "parts")) {
						u.Parts = append(u.Parts, p.Value)
					}
				}
			}
		}
		out = append(out, u)
	}
	return out
}
