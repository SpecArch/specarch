// Package spec reads a SpecArch specification from disk: the root file
// specarch.yaml and the stage folders beside it, merged into one document
// the validator and the generators work on. Every node remembers the file
// it came from, so a diagnostic names that file and its line.
package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// RootFile is the name of a specification's root file.
const RootFile = "specarch.yaml"

// ImplementationSuffix ends the name of every implementation file.
const ImplementationSuffix = ".specarch-implementation.yaml"

// TestFile is the file each test folder holds.
const TestFile = "test.yaml"

// RecordsFolder is the folder beside a specification's folder that holds
// its records: one file per change, defect, release, incident,
// commissioning run or approval.
const RecordsFolder = "records"

// Stages lists the life-cycle stages in order; each is a folder name.
var Stages = []string{"requirements", "design", "implementation", "tests", "deployment", "commissioning", "operation"}

// QuestionsSection is the one section every stage folder may hold: a
// question is written with what it is about, and questions arise at every
// stage. The loader merges them into one section.
const QuestionsSection = "questions"

// Sections maps every section to its stage.
var Sections = map[string]string{
	"stakeholders": "requirements", "needs": "requirements", "requirements": "requirements",
	"glossary": "requirements", "assumptions": "requirements", "constraints": "requirements",
	"enums": "design", "entities": "design", "views": "design", "permissions": "design", "roles": "design", "session": "design", "paths": "design",
	"commands": "design", "channels": "design", "dependencies": "design", "jobs": "design", "errors": "design", "pages": "design", "menus": "design", "flows": "design", "accessibility": "design", "theme": "design", "algorithms": "design", "decisions": "design",
	"tests":        "tests",
	"environments": "deployment", "configuration": "deployment", "release": "deployment",
	"rollback": "deployment", "migrations": "deployment",
	"checks": "commissioning", "signoff": "commissioning",
	"monitors": "operation",
}

// sectionOrder is the order sections appear in the merged document and in
// docs/conventions.md: life-cycle order, the questions last.
var sectionOrder = []string{
	"stakeholders", "needs", "requirements", "glossary", "assumptions", "constraints",
	"enums", "entities", "views", "permissions", "roles", "session", "paths", "commands", "channels", "dependencies", "jobs", "errors", "pages", "menus", "flows", "accessibility", "theme", "algorithms",
	"tests", "decisions",
	"environments", "configuration", "release", "rollback", "migrations",
	"checks", "signoff",
	"monitors",
	QuestionsSection,
}

// singleSections hold one object, not a map of named objects, so they
// cannot be split across files.
var singleSections = map[string]bool{"release": true, "rollback": true, "signoff": true, "accessibility": true, "theme": true}

// rootOnly are the keys that live only in the root file.
var rootOnly = map[string]bool{"specarch": true, "info": true, "stages": true, "sources": true}

// SectionsOf lists the sections of a stage, in order.
func SectionsOf(stage string) []string {
	var out []string
	for _, s := range sectionOrder {
		if Sections[s] == stage {
			out = append(out, s)
		}
	}
	return out
}

// Problem is something wrong with a file or the layout, found while reading.
type Problem struct {
	File    string
	Line    int
	Path    string
	Rule    string
	Message string
}

// Implementation is one implementation file of the specification.
type Implementation struct {
	Path  string
	Stack string
	Data  []byte
}

// Record is one record file beside the specification.
type Record struct {
	Path   string // the file
	Folder string // its folder under records/, with slashes; "" directly in records/
	Data   []byte
}

// Spec is one specification as read from disk.
type Spec struct {
	Dir             string                // the root folder, as given
	RootFile        string                // Dir/specarch.yaml
	Root            *yaml.Node            // the merged document; nil when the root file did not parse
	Value           any                   // the merged document as plain values, for the schema
	Files           map[*yaml.Node]string // the file each node came from
	Problems        []Problem
	Stages          []string // the stages listed in the root file
	Implementations []Implementation
	RecordsDir      string   // the records/ folder beside Dir
	Records         []Record // the record files under RecordsDir, in path order
}

// Load reads the specification rooted at dir.
func Load(dir string) *Spec {
	s := &Spec{Dir: dir, RootFile: filepath.Join(dir, RootFile), Files: map[*yaml.Node]string{}}
	data, err := os.ReadFile(s.RootFile)
	if err != nil {
		s.problem(s.RootFile, 1, "/", "layout", "cannot read %s (%s)", RootFile, plainIOError(err))
		return s
	}
	doc := source.Parse(data)
	for _, p := range doc.Problems {
		s.problem(s.RootFile, p.Line, p.Path, p.Rule, "%s", p.Message)
	}
	if doc.Root == nil {
		return s
	}
	if doc.Root.Kind != yaml.MappingNode {
		s.Root = doc.Root
		s.Value = doc.Value
		s.record(doc.Root, s.RootFile)
		return s
	}
	s.record(doc.Root, s.RootFile)
	merged := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Line: doc.Root.Line, Column: doc.Root.Column}
	s.Files[merged] = s.RootFile
	sections := map[string]*yaml.Node{}
	listed := map[string]bool{}
	for _, item := range source.Items(source.Child(doc.Root, "stages")) {
		listed[item.Value] = true
		s.Stages = append(s.Stages, item.Value)
	}
	// The root file's own keys: specarch, info, stages, sources, and the
	// sections of stages that have no folder.
	for _, p := range source.Pairs(doc.Root) {
		key := p.Key.Value
		if key == QuestionsSection {
			s.mergeSection(sections, key, p.Value, s.RootFile)
			continue
		}
		stage, isSection := Sections[key]
		if isSection && listed[stage] {
			s.problem(s.RootFile, p.Key.Line, source.Pointer(key), "layout",
				"%s belongs to the %s stage, which stages lists, so it lives under %s/ and not in %s; move it there", key, stage, stage, RootFile)
			continue
		}
		if isSection && !singleSections[key] {
			s.mergeSection(sections, key, p.Value, s.RootFile)
			continue
		}
		merged.Content = append(merged.Content, p.Key, p.Value)
		if isSection {
			sections[key] = p.Value
		}
	}
	// The stage folders.
	entries, err := os.ReadDir(dir)
	if err != nil {
		s.problem(s.RootFile, 1, "/", "layout", "cannot read the folder %s (%s)", dir, plainIOError(err))
	}
	present := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		present[e.Name()] = true
		if !listed[e.Name()] {
			switch stage := Sections[e.Name()]; {
			case isStage(e.Name()):
				s.problem(s.RootFile, 1, "/stages", "layout", "the folder %s/ exists but stages in %s does not list %s; add it to stages, or move its files into %s", e.Name(), RootFile, e.Name(), RootFile)
			case stage != "" && listed[stage]:
				s.problem(s.RootFile, 1, "/", "layout", "the folder %s/ is a section of the %s stage, not a stage; move it to %s/%s/", e.Name(), stage, stage, e.Name())
			case stage != "":
				s.problem(s.RootFile, 1, "/", "layout", "the folder %s/ is a section of the %s stage, not a stage; move it to %s/%s/ and list %s in stages", e.Name(), stage, stage, e.Name(), stage)
			case e.Name() == RecordsFolder:
				s.problem(s.RootFile, 1, "/", "layout", "the folder %s/ is not a stage; records live in %s/ beside the specification's folder, not inside it, so move it up one level", e.Name(), RecordsFolder)
			default:
				s.problem(s.RootFile, 1, "/", "layout", "the folder %s/ is not a stage; a specification's folders are %s", e.Name(), strings.Join(Stages, ", "))
			}
		}
	}
	// Implementation files beside the root file belong to the specification
	// when the implementation stage has no folder, as sections belong in
	// the root file when their stage has none.
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ImplementationSuffix) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if listed["implementation"] {
			s.problem(p, 1, "/", "layout", "stages lists implementation, so an implementation file lives under implementation/<stack>/; move it there")
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			s.problem(p, 1, "/", "layout", "cannot read it (%s)", plainIOError(err))
			continue
		}
		name := strings.TrimSuffix(e.Name(), ImplementationSuffix)
		s.Implementations = append(s.Implementations, Implementation{Path: p, Stack: name[strings.LastIndex(name, ".")+1:], Data: data})
	}
	for _, stage := range s.Stages {
		if !present[stage] {
			s.problem(s.RootFile, 1, "/stages", "layout", "stages lists %s but there is no %s/ folder beside %s; create it, or remove %s from stages", stage, stage, RootFile, stage)
			continue
		}
		folder := filepath.Join(dir, stage)
		switch stage {
		case "tests":
			s.loadTests(folder, sections)
		case "implementation":
			s.loadImplementations(folder, sections)
		default:
			s.loadStage(stage, folder, sections)
		}
	}
	// Sections in life-cycle order after the root file's own keys.
	for _, name := range sectionOrder {
		n := sections[name]
		if n == nil || source.Key(merged, name) != nil {
			continue
		}
		key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name, Line: n.Line, Column: n.Column}
		s.Files[key] = s.Files[n]
		merged.Content = append(merged.Content, key, n)
	}
	s.Root = merged
	s.Value = source.ValueOf(merged)
	s.loadRecords()
	return s
}

// loadRecords finds every record file under the records/ folder beside the
// specification's folder. A missing folder means there are no records.
func (s *Spec) loadRecords() {
	s.RecordsDir = filepath.Join(s.Dir, "..", RecordsFolder)
	if info, err := os.Stat(s.RecordsDir); err != nil || !info.IsDir() {
		return
	}
	err := filepath.WalkDir(s.RecordsDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != s.RecordsDir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".yml") {
			s.problem(p, 1, "/", "layout", "a SpecArch file ends in .yaml; rename it")
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			s.problem(p, 1, "/", "layout", "cannot read it (%s)", plainIOError(err))
			return nil
		}
		rel, _ := filepath.Rel(s.RecordsDir, filepath.Dir(p))
		folder := filepath.ToSlash(rel)
		if folder == "." {
			folder = ""
		}
		s.Records = append(s.Records, Record{Path: p, Folder: folder, Data: data})
		return nil
	})
	if err != nil {
		s.problem(s.RecordsDir, 1, "/", "layout", "cannot read the folder (%s)", plainIOError(err))
	}
}

func isStage(name string) bool {
	for _, s := range Stages {
		if s == name {
			return true
		}
	}
	return false
}

// mergeSection adds the entries of one file's section mapping to the
// merged section, reporting a name already defined by another file.
func (s *Spec) mergeSection(sections map[string]*yaml.Node, name string, value *yaml.Node, file string) {
	value = source.Deref(value)
	target := sections[name]
	if target == nil {
		target = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Line: value.Line, Column: value.Column}
		s.Files[target] = file
		sections[name] = target
	}
	if value.Kind != yaml.MappingNode {
		// Not a mapping: keep it so the schema reports the shape, unless a
		// mapping is already there.
		if len(target.Content) == 0 {
			sections[name] = value
		} else {
			s.problem(file, value.Line, source.Pointer(name), "layout", "%s must be a mapping of named objects here", name)
		}
		return
	}
	if sections[name].Kind != yaml.MappingNode {
		return
	}
	for _, p := range source.Pairs(value) {
		if first := source.Key(target, p.Key.Value); first != nil {
			s.problem(file, p.Key.Line, source.Pointer(name, p.Key.Value), "duplicate_key",
				"%s is already defined in %s on line %d; a name is defined once in the whole specification", p.Key.Value, s.Files[first], first.Line)
			continue
		}
		target.Content = append(target.Content, p.Key, p.Value)
	}
}

// loadStage reads every YAML file under a stage folder. A file holds only
// sections of its stage; under a sub-folder named after a section, only
// that section.
func (s *Spec) loadStage(stage, folder string, sections map[string]*yaml.Node) {
	allowed := SectionsOf(stage)
	err := filepath.WalkDir(folder, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != folder && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(folder, p)
		if strings.HasSuffix(d.Name(), ".yml") {
			s.problem(p, 1, "/", "layout", "a SpecArch file ends in .yaml; rename it")
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}
		only := ""
		for _, part := range strings.Split(filepath.ToSlash(filepath.Dir(rel)), "/") {
			if Sections[part] == stage || part == QuestionsSection {
				only = part
			}
		}
		data, err := os.ReadFile(p)
		if err != nil {
			s.problem(p, 1, "/", "layout", "cannot read it (%s)", plainIOError(err))
			return nil
		}
		doc := source.Parse(data)
		for _, pr := range doc.Problems {
			s.problem(p, pr.Line, pr.Path, pr.Rule, "%s", pr.Message)
		}
		if doc.Root == nil {
			return nil
		}
		s.record(doc.Root, p)
		if doc.Root.Kind != yaml.MappingNode {
			s.problem(p, doc.Root.Line, "/", "layout", "a file of the %s stage is a mapping of sections (%s)", stage, strings.Join(allowed, ", "))
			return nil
		}
		for _, pair := range source.Pairs(doc.Root) {
			key := pair.Key.Value
			switch {
			case only != "" && key != only:
				s.problem(p, pair.Key.Line, source.Pointer(key), "layout", "a file under %s/%s/ holds only %s; move %s to %s/%s/", stage, only, only, key, stage, sectionFolder(key, stage))
			case key == QuestionsSection:
				s.mergeSection(sections, key, pair.Value, p)
			case Sections[key] == stage && singleSections[key]:
				if first := sections[key]; first != nil {
					s.problem(p, pair.Key.Line, source.Pointer(key), "duplicate_key", "%s is already defined in %s on line %d; it is one object, written in one file", key, s.Files[first], first.Line)
					continue
				}
				sections[key] = pair.Value
			case Sections[key] == stage:
				s.mergeSection(sections, key, pair.Value, p)
			case Sections[key] != "":
				s.problem(p, pair.Key.Line, source.Pointer(key), "layout", "%s belongs to the %s stage, not to %s; move it under %s/", key, Sections[key], stage, Sections[key])
			case rootOnly[key]:
				s.problem(p, pair.Key.Line, source.Pointer(key), "layout", "%s is written only in %s; remove it here", key, RootFile)
			default:
				s.problem(p, pair.Key.Line, source.Pointer(key), "layout", "%s is not a section of the %s stage; the sections are %s", key, stage, strings.Join(allowed, ", "))
			}
		}
		return nil
	})
	if err != nil {
		s.problem(folder, 1, "/", "layout", "cannot read the folder (%s)", plainIOError(err))
	}
}

func sectionFolder(key, stage string) string {
	if Sections[key] == stage || key == QuestionsSection {
		return key
	}
	return "<section>"
}

// loadQuestionsFile reads a YAML file directly under tests/ or
// implementation/, whose entries are otherwise folders: it holds only
// questions. It reports whether the file was one.
func (s *Spec) loadQuestionsFile(p string, sections map[string]*yaml.Node) bool {
	data, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	doc := source.Parse(data)
	if doc.Root == nil || doc.Root.Kind != yaml.MappingNode {
		return false
	}
	for _, pair := range source.Pairs(doc.Root) {
		if pair.Key.Value != QuestionsSection {
			return false
		}
	}
	for _, pr := range doc.Problems {
		s.problem(p, pr.Line, pr.Path, pr.Rule, "%s", pr.Message)
	}
	s.record(doc.Root, p)
	for _, pair := range source.Pairs(doc.Root) {
		s.mergeSection(sections, QuestionsSection, pair.Value, p)
	}
	return true
}

// StageOfFile tells which stage folder a file of the specification sits
// under, or "" for the root file and the files beside it.
func (s *Spec) StageOfFile(file string) string {
	rel, err := filepath.Rel(s.Dir, file)
	if err != nil {
		return ""
	}
	first := strings.Split(filepath.ToSlash(rel), "/")[0]
	if isStage(first) {
		return first
	}
	return ""
}

// Listed reports whether the root file lists a stage, so that it has a
// folder.
func (s *Spec) Listed(stage string) bool {
	for _, st := range s.Stages {
		if st == stage {
			return true
		}
	}
	return false
}

// loadTests reads tests/<name>/test.yaml for every test folder.
func (s *Spec) loadTests(folder string, sections map[string]*yaml.Node) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		s.problem(folder, 1, "/", "layout", "cannot read the folder (%s)", plainIOError(err))
		return
	}
	tests := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Line: 1, Column: 1}
	s.Files[tests] = s.RootFile
	sections["tests"] = tests
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p := filepath.Join(folder, e.Name())
		if !e.IsDir() {
			if strings.HasSuffix(e.Name(), ".yaml") && s.loadQuestionsFile(p, sections) {
				continue
			}
			if strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml") {
				s.problem(p, 1, "/", "layout", "a file directly under tests/ holds only questions; each test is a folder tests/<name>/ holding %s", TestFile)
			}
			continue
		}
		tf := filepath.Join(p, TestFile)
		data, err := os.ReadFile(tf)
		if err != nil {
			s.problem(p, 1, "/", "layout", "the test folder tests/%s has no %s; add one with the test's subject, scenario, level, given, when and then", e.Name(), TestFile)
			continue
		}
		doc := source.Parse(data)
		for _, pr := range doc.Problems {
			s.problem(tf, pr.Line, pr.Path, pr.Rule, "%s", pr.Message)
		}
		if doc.Root == nil {
			continue
		}
		s.record(doc.Root, tf)
		key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: e.Name(), Line: doc.Root.Line, Column: 1}
		if key.Line == 0 {
			key.Line = 1
		}
		s.Files[key] = tf
		tests.Content = append(tests.Content, key, doc.Root)
	}
	if len(tests.Content) == 0 {
		delete(sections, "tests")
	}
}

// loadImplementations finds implementation/<stack>/<name>.<stack>.specarch-implementation.yaml.
func (s *Spec) loadImplementations(folder string, sections map[string]*yaml.Node) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		s.problem(folder, 1, "/", "layout", "cannot read the folder (%s)", plainIOError(err))
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p := filepath.Join(folder, e.Name())
		if !e.IsDir() {
			if strings.HasSuffix(e.Name(), ".yaml") && s.loadQuestionsFile(p, sections) {
				continue
			}
			if strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml") {
				s.problem(p, 1, "/", "layout", "a file directly under implementation/ holds only questions; an implementation file lives in implementation/<stack>/, named <name>.<stack>%s", ImplementationSuffix)
			}
			continue
		}
		stack := e.Name()
		files, err := os.ReadDir(p)
		if err != nil {
			s.problem(p, 1, "/", "layout", "cannot read the folder (%s)", plainIOError(err))
			continue
		}
		found := 0
		for _, f := range files {
			fp := filepath.Join(p, f.Name())
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".yaml") && !strings.HasSuffix(f.Name(), ".yml") {
				continue
			}
			if !strings.HasSuffix(f.Name(), ImplementationSuffix) {
				s.problem(fp, 1, "/", "layout", "a file under implementation/%s/ is an implementation file named <name>.%s%s; rename it, or move it to its stage", stack, stack, ImplementationSuffix)
				continue
			}
			found++
			name := strings.TrimSuffix(f.Name(), ImplementationSuffix)
			if i := strings.LastIndex(name, "."); i < 0 || name[i+1:] != stack {
				s.problem(fp, 1, "/", "layout", "the folder says the stack is %s but the file name says %s; make them agree", stack, name[strings.LastIndex(name, ".")+1:])
				continue
			}
			data, err := os.ReadFile(fp)
			if err != nil {
				s.problem(fp, 1, "/", "layout", "cannot read it (%s)", plainIOError(err))
				continue
			}
			s.Implementations = append(s.Implementations, Implementation{Path: fp, Stack: stack, Data: data})
		}
		if found == 0 {
			s.problem(p, 1, "/", "layout", "the folder implementation/%s/ holds no implementation file; add <name>.%s%s or remove the folder", stack, stack, ImplementationSuffix)
		}
	}
}

// record remembers the file of every node under n.
func (s *Spec) record(n *yaml.Node, file string) {
	if n == nil {
		return
	}
	s.Files[n] = file
	for _, c := range n.Content {
		s.record(c, file)
	}
}

func (s *Spec) problem(file string, line int, path, rule, format string, args ...any) {
	if line < 1 {
		line = 1
	}
	s.Problems = append(s.Problems, Problem{File: file, Line: line, Path: path, Rule: rule, Message: fmt.Sprintf(format, args...)})
}

// Covers reports whether the specification has any section of a stage.
func (s *Spec) Covers(stage string) bool {
	if s.Root == nil {
		return false
	}
	for _, name := range SectionsOf(stage) {
		if source.Child(s.Root, name) != nil {
			return true
		}
	}
	return false
}

// RootOf finds the specification a file belongs to: the nearest ancestor
// folder holding specarch.yaml, or "".
func RootOf(path string) string {
	dir := filepath.Dir(path)
	for {
		if _, err := os.Stat(filepath.Join(dir, RootFile)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// Find lists the specifications and the standalone implementation files
// under a folder: a folder holding specarch.yaml is a specification, and
// nothing under it is searched further.
func Find(dir string) (roots []string, implementations []string, err error) {
	err = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != dir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(p, RootFile)); err == nil {
				roots = append(roots, p)
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ImplementationSuffix) {
			implementations = append(implementations, p)
		}
		return nil
	})
	sort.Strings(roots)
	sort.Strings(implementations)
	return roots, implementations, err
}

// PlainIOError is the reason a file could not be read, in the words the C
// library uses, so both builds print the same.
func PlainIOError(err error) string { return plainIOError(err) }

func plainIOError(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 {
		return msg[i+2:]
	}
	return msg
}

// Block is one entry of a question's blocks: a stage, a section, or a
// pointer to an element of the specification or to one key of it.
type Block struct {
	Stage   string   // the stage the block is about
	Section string   // the section, for a section or a pointer
	Tokens  []string // for a pointer: section, name, and the keys below
}

// IsPointer reports whether the block names an element or a key of one.
func (b Block) IsPointer() bool { return len(b.Tokens) > 0 }

// Element is the pointer of the element the block is about, such as
// /entities/Loan, or "" for a stage or a section.
func (b Block) Element() string {
	if len(b.Tokens) < 2 {
		return ""
	}
	return source.Pointer(b.Tokens[:2]...)
}

// Key is the key of the element the block is about, when it names one.
func (b Block) Key() string {
	if len(b.Tokens) < 3 {
		return ""
	}
	return b.Tokens[len(b.Tokens)-1]
}

// Sections lists the sections the block holds up: every section of its
// stage, or the one section.
func (b Block) Sections() []string {
	if b.Section != "" {
		return []string{b.Section}
	}
	return SectionsOf(b.Stage)
}

// ParseBlock reads one entry of a question's blocks. ok is false when the
// text is neither a stage, a section, nor a pointer whose first token is a
// section; whether the pointer resolves is the validator's check.
func ParseBlock(s string) (b Block, ok bool) {
	if isStage(s) {
		return Block{Stage: s}, true
	}
	if stage, isSection := Sections[s]; isSection {
		return Block{Stage: stage, Section: s}, true
	}
	if !strings.HasPrefix(s, "#/") {
		return Block{}, false
	}
	for _, t := range strings.Split(strings.TrimPrefix(s, "#/"), "/") {
		b.Tokens = append(b.Tokens, source.UnescapeToken(t))
	}
	stage, isSection := Sections[b.Tokens[0]]
	if !isSection || len(b.Tokens) < 2 {
		return Block{}, false
	}
	b.Stage, b.Section = stage, b.Tokens[0]
	return b, true
}
