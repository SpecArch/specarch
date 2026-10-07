package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// Kind is the kind of a SpecArch file, read from its name.
type Kind int

const (
	KindNone Kind = iota
	KindDesign
	KindImplementation
)

const (
	DesignSuffix         = ".specarch-design.yaml"
	ImplementationSuffix = ".specarch-implementation.yaml"
)

// KindOf tells the kind of a file from its name.
func KindOf(name string) Kind {
	switch {
	case strings.HasSuffix(name, DesignSuffix):
		return KindDesign
	case strings.HasSuffix(name, ImplementationSuffix):
		return KindImplementation
	}
	return KindNone
}

// ReadFile reads a file the caller did not pass in: the design file an
// implementation file names.
type ReadFile func(path string) ([]byte, error)

// Check runs every check on one file and returns its diagnostics, sorted.
// The path is used as given in every diagnostic.
func Check(path string, data []byte, read ReadFile) []Diagnostic {
	c := &checker{file: path}
	c.run(data, read)
	ds := withoutEchoes(c.diags)
	Sort(ds)
	return ds
}

// withoutEchoes drops a diagnostic that only repeats a schema error: one at
// the same path, or inside a key the schema refused.
func withoutEchoes(ds []Diagnostic) []Diagnostic {
	schemaAt := map[string]bool{}
	var refused []string
	for _, d := range ds {
		if d.Rule == RuleSchema {
			schemaAt[d.Path] = true
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
	file  string
	root  *yaml.Node
	diags []Diagnostic
}

func (c *checker) add(n *yaml.Node, path string, rule Rule, format string, args ...any) {
	line := 1
	if n != nil && n.Line > 0 {
		line = n.Line
	}
	c.addLine(line, path, rule, format, args...)
}

func (c *checker) addLine(line int, path string, rule Rule, format string, args ...any) {
	if path == "" {
		path = "/"
	}
	c.diags = append(c.diags, Diagnostic{File: c.file, Line: line, Severity: Error, Path: path, Rule: rule, Message: fmt.Sprintf(format, args...)})
}

func (c *checker) warn(n *yaml.Node, path string, rule Rule, format string, args ...any) {
	c.add(n, path, rule, format, args...)
	c.diags[len(c.diags)-1].Severity = Warning
}

func (c *checker) run(data []byte, read ReadFile) {
	kind := KindOf(c.file)
	if kind == KindNone {
		c.addLine(1, "/", RuleFileKind, "the file name ends in neither %s nor %s; rename the file so its kind is clear", DesignSuffix, ImplementationSuffix)
		return
	}
	doc := source.Parse(data)
	for _, p := range doc.Problems {
		c.addLine(p.Line, p.Path, Rule(p.Rule), "%s", p.Message)
	}
	if doc.Root == nil {
		return
	}
	c.root = doc.Root
	if !c.kindMatchesRoot(kind) {
		return
	}
	c.checkSchema(kind, doc.Value)
	c.checkChangeLog()
	switch kind {
	case KindDesign:
		c.checkBoundaryDesign()
		d := newDesign(c.root)
		c.checkDesign(d)
	case KindImplementation:
		c.checkBoundaryImplementation()
		c.checkImplementation(read)
	}
}

// kindMatchesRoot refuses a file whose name and root key disagree, since
// every later check would only repeat that one mistake.
func (c *checker) kindMatchesRoot(kind Kind) bool {
	if c.root.Kind != yaml.MappingNode {
		return true // the schema reports it
	}
	hasDesign := source.Key(c.root, "specarch") != nil
	hasImpl := source.Key(c.root, "specarchImplementation") != nil
	switch {
	case kind == KindDesign && hasImpl && !hasDesign:
		c.add(source.Key(c.root, "specarchImplementation"), "/specarchImplementation", RuleFileKind,
			"the file is named as a design file but starts with specarchImplementation; rename it to end in %s, or start it with specarch", ImplementationSuffix)
		return false
	case kind == KindImplementation && hasDesign && !hasImpl:
		c.add(source.Key(c.root, "specarch"), "/specarch", RuleFileKind,
			"the file is named as an implementation file but starts with specarch; rename it to end in %s, or start it with specarchImplementation", DesignSuffix)
		return false
	}
	return true
}
