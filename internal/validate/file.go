package validate

import (
	"fmt"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// Kind is the kind of a SpecArch file, read from its name.
type Kind int

const (
	KindNone Kind = iota
	KindDesign
	KindImplementation
	KindRecord
	KindIdiom
)

const (
	RootFile             = spec.RootFile
	ImplementationSuffix = spec.ImplementationSuffix
)

// KindOf tells the kind of a file from its name: the root file of a
// specification, an implementation file, or neither.
func KindOf(name string) Kind {
	switch {
	case name == RootFile || strings.HasSuffix(name, "/"+RootFile):
		return KindDesign
	case strings.HasSuffix(name, ImplementationSuffix):
		return KindImplementation
	}
	return KindNone
}

// Loader reads the specification rooted at a folder: the implementation
// file names it under implements.
type Loader func(dir string) *spec.Spec

// CheckSpec runs every check on a specification and on the implementation
// files inside it, and returns the diagnostics to report, sorted. What an
// open question covers is left out; CheckSpecCovered returns it too.
func CheckSpec(s *spec.Spec) []Diagnostic {
	kept, _ := CheckSpecCovered(s)
	return kept
}

// CheckSpecCovered runs every check and returns the diagnostics to report
// and, apart, the ones an open must question covers (see Covered), both
// sorted.
func CheckSpecCovered(s *spec.Spec) (kept, covered []Diagnostic) {
	all := checkSpecAll(s)
	p := SpecPlacer(s)
	if s.Root == nil {
		p.Place(all)
		Sort(all)
		return all, nil
	}
	kept, covered = Covered(all, s.Root)
	p.Place(kept)
	p.Place(covered)
	Sort(kept)
	Sort(covered)
	return kept, covered
}

// SpecPlacer places the diagnostics of a specification: ids name files
// from the specification's folder.
func SpecPlacer(s *spec.Spec) *Placer {
	return &Placer{Root: s.Root, Files: s.Files, RootFile: s.RootFile, Dir: s.Dir}
}

func checkSpecAll(s *spec.Spec) []Diagnostic {
	c := &checker{file: s.RootFile, files: s.Files}
	for _, p := range s.Problems {
		c.addFile(p.File, p.Line, p.Path, Rule(p.Rule), "%s", p.Message)
	}
	var out []Diagnostic
	var d *design
	if s.Root != nil {
		c.root = s.Root
		if c.rootIsSpec() {
			c.checkSchema(KindDesign, s.Value)
			c.checkChangeLog()
			c.checkBoundaryDesign()
			d = newDesign(c.root)
			d.spec = s
			c.checkDesign(d)
		}
	}
	out = append(out, withoutEchoes(c.diags)...)
	for _, impl := range s.Implementations {
		ic := &checker{file: impl.Path}
		ic.runImplementation(impl.Data, s)
		out = append(out, withoutEchoes(ic.diags)...)
	}
	if d != nil {
		out = append(out, checkRecords(s, d)...)
	}
	Sort(out)
	return out
}

// CheckImplementation checks one implementation file given on its own. The
// specification it implements is loaded through load.
func CheckImplementation(path string, data []byte, load Loader) []Diagnostic {
	c := &checker{file: path}
	c.runImplementation(data, nil, load)
	ds := withoutEchoes(c.diags)
	(&Placer{Dir: filepath.Dir(path)}).Place(ds)
	Sort(ds)
	return ds
}

// CheckNamed reports a file given by name that is neither a root file nor
// an implementation file.
func CheckNamed(path string) []Diagnostic {
	c := &checker{file: path}
	if root := spec.RootOf(path); root != "" {
		c.addLine(1, "/", RuleFileKind, "this file is part of the specification whose root file is %s; run specarch validate on that specification's folder", filepath.Join(root, RootFile))
	} else {
		c.addLine(1, "/", RuleFileKind, "the file is neither %s nor an implementation file (*%s); name a specification's folder or root file", RootFile, ImplementationSuffix)
	}
	(&Placer{Dir: filepath.Dir(path)}).Place(c.diags)
	return c.diags
}

// withoutEchoes drops a diagnostic that only repeats a schema error: one at
// the same path, or inside a key the schema refused. A missing key is
// reported at the object that lacks it, so it says nothing wrong about what
// the object does hold; it makes no echo of the other errors there.
func withoutEchoes(ds []Diagnostic) []Diagnostic {
	schemaAt := map[string]bool{}
	var refused []string
	for _, d := range ds {
		if d.Rule == RuleSchema {
			if !strings.HasSuffix(d.Message, " is missing; add it here") {
				schemaAt[d.Path] = true
			}
			if strings.Contains(d.Message, "is not a key this object can have") {
				refused = append(refused, d.Path+"/")
			}
		}
	}
	out := ds[:0]
	for _, d := range ds {
		if d.Rule != RuleSchema && d.Severity == Error {
			if schemaAt[d.Path] {
				continue
			}
			echo := false
			for _, r := range refused {
				if strings.HasPrefix(d.Path+"/", r) {
					echo = true
					break
				}
			}
			if echo {
				continue
			}
		}
		out = append(out, d)
	}
	return out
}

type checker struct {
	file  string                // the file diagnostics name when a node is not known
	files map[*yaml.Node]string // the file of each node of a merged specification
	root  *yaml.Node
	diags []Diagnostic
	text  Placer // the lines of the files read, for an expression's column
}

func (c *checker) fileOf(n *yaml.Node) string {
	if n != nil && c.files != nil {
		if f, ok := c.files[n]; ok {
			return f
		}
	}
	return c.file
}

func (c *checker) add(n *yaml.Node, path string, rule Rule, format string, args ...any) {
	line := 1
	if n != nil && n.Line > 0 {
		line = n.Line
	}
	c.addFile(c.fileOf(n), line, path, rule, format, args...)
}

func (c *checker) addLine(line int, path string, rule Rule, format string, args ...any) {
	c.addFile(c.file, line, path, rule, format, args...)
}

func (c *checker) addFile(file string, line int, path string, rule Rule, format string, args ...any) {
	if path == "" {
		path = "/"
	}
	if line < 1 {
		line = 1
	}
	c.diags = append(c.diags, Diagnostic{File: file, Line: line, Severity: Error, Path: path, Rule: rule, Message: fmt.Sprintf(format, args...)})
}

func (c *checker) warn(n *yaml.Node, path string, rule Rule, format string, args ...any) {
	c.add(n, path, rule, format, args...)
	c.diags[len(c.diags)-1].Severity = Warning
}

// rootIsSpec refuses a root file that starts with specarchImplementation,
// since every later check would only repeat that one mistake.
func (c *checker) rootIsSpec() bool {
	if c.root.Kind != yaml.MappingNode {
		return true // the schema reports it
	}
	hasDesign := source.Key(c.root, "specarch") != nil
	hasImpl := source.Key(c.root, "specarchImplementation") != nil
	if hasImpl && !hasDesign {
		c.add(source.Key(c.root, "specarchImplementation"), "/specarchImplementation", RuleFileKind,
			"%s is a specification's root file but starts with specarchImplementation; an implementation file is named <name>.<stack>%s", RootFile, ImplementationSuffix)
		return false
	}
	return true
}

// runImplementation checks an implementation file. With s given, the file
// is part of that specification; otherwise load reads the one it names.
func (c *checker) runImplementation(data []byte, s *spec.Spec, load ...Loader) {
	doc := source.Parse(data)
	for _, p := range doc.Problems {
		c.addLine(p.Line, p.Path, Rule(p.Rule), "%s", p.Message)
	}
	if doc.Root == nil {
		return
	}
	c.root = doc.Root
	if c.root.Kind == yaml.MappingNode && source.Key(c.root, "specarch") != nil && source.Key(c.root, "specarchImplementation") == nil {
		c.add(source.Key(c.root, "specarch"), "/specarch", RuleFileKind,
			"the file is named as an implementation file but starts with specarch; a specification's root file is named %s", RootFile)
		return
	}
	c.checkSchema(KindImplementation, doc.Value)
	c.checkChangeLog()
	c.checkBoundaryImplementation()
	var l Loader
	if len(load) > 0 {
		l = load[0]
	}
	c.checkImplementation(s, l)
}
