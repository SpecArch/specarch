package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/canonical"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// modelVersion is the version of the shape of the document model prints.
const modelVersion = "0.1"

// runModel prints the model of the specifications found under the paths
// given: one JSON document in RFC 8785's canonical form (ADR-138).
func runModel(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(stderr, "specarch model needs at least one folder or file\n\n%s", usage)
		return 2
	}
	for _, a := range args {
		if len(a) > 1 && a[0] == '-' {
			fmt.Fprintf(stderr, "specarch model has no option %s; to read a path whose name starts with -, write -- before it\n\n%s", a, usage)
			return 2
		}
	}
	inputs, ioError := collect(args, stderr)
	if ioError {
		return 2
	}
	var specs, impls, estates []any
	byFolder := map[string]*spec.Spec{}
	for _, in := range inputs {
		if in.root != "" {
			s := spec.Load(in.root)
			byFolder[filepath.Clean(in.root)] = s
			specs = append(specs, modelSpec(s))
		}
	}
	for _, in := range inputs {
		switch {
		case in.implementation != "":
			data, err := os.ReadFile(in.implementation)
			if err != nil {
				fmt.Fprintf(stderr, "specarch: cannot read %s: %s\n", in.implementation, spec.PlainIOError(err))
				return 2
			}
			impls = append(impls, modelImplementation(in.implementation, filepath.Dir(in.implementation), data, nil))
		case in.estate != nil:
			estates = append(estates, modelEstate(in.estate, byFolder))
		}
	}
	byFile := func(key string) func([]any) {
		return func(list []any) {
			sort.SliceStable(list, func(i, j int) bool {
				return list[i].(map[string]any)[key].(string) < list[j].(map[string]any)[key].(string)
			})
		}
	}
	byFile("root")(specs)
	byFile("file")(impls)
	byFile("file")(estates)
	out, err := canonical.Marshal(map[string]any{
		"specarchModel":   modelVersion,
		"program":         version,
		"specifications":  orEmpty(specs),
		"implementations": orEmpty(impls),
		"estates":         orEmpty(estates),
	})
	if err != nil {
		fmt.Fprintf(stderr, "specarch model: %v\n", err)
		return 2
	}
	stdout.Write(out)
	fmt.Fprintln(stdout)
	return 0
}

func orEmpty(list []any) []any {
	if list == nil {
		return []any{}
	}
	return list
}

func modelSpec(s *spec.Spec) map[string]any {
	info := source.Child(s.Root, "info")
	text := func(n *yaml.Node) any {
		if source.IsScalar(n) {
			return source.Str(n)
		}
		return nil
	}
	var impls []any
	for _, i := range s.Implementations {
		impls = append(impls, modelImplementation(i.Path, s.Dir, i.Data, s.Root))
	}
	return map[string]any{
		"root":            filepath.ToSlash(s.RootFile),
		"name":            text(source.Child(info, "name")),
		"version":         text(source.Child(info, "version")),
		"metaModel":       text(source.Child(s.Root, "specarch")),
		"document":        s.Value,
		"positions":       modelPositions(validate.Positions(s.Root, s.Files, s.RootFile)),
		"references":      modelReferences(validate.ModelReferences(s.Root)),
		"problems":        modelProblems(validate.LayoutProblems(s)),
		"implementations": orEmpty(impls),
	}
}

// modelImplementation is one implementation file: of a specification when
// design is its merged document, or on its own.
func modelImplementation(path, dir string, data []byte, design *yaml.Node) map[string]any {
	doc := source.Parse(data)
	name := filepath.Base(path)
	name = name[:len(name)-len(spec.ImplementationSuffix)]
	stack := name
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			stack = name[i+1:]
			break
		}
	}
	return map[string]any{
		"file":       filepath.ToSlash(path),
		"stack":      stack,
		"document":   doc.Value,
		"positions":  modelPositions(validate.Positions(doc.Root, nil, path)),
		"references": modelReferences(validate.ImplementationReferences(doc.Root, design)),
		"problems":   modelProblems(validate.ReadProblems(path, dir, doc)),
	}
}

func modelEstate(e *validate.Estate, byFolder map[string]*spec.Spec) map[string]any {
	var modules []any
	for _, m := range e.Modules {
		var resolved any
		if s := byFolder[filepath.Clean(m.Location)]; m.Location != "" && s != nil {
			resolved = filepath.ToSlash(s.RootFile)
		}
		modules = append(modules, map[string]any{
			"name":          m.Key,
			"location":      m.LocationText(),
			"version":       m.Version,
			"specification": resolved,
		})
	}
	return map[string]any{"file": filepath.ToSlash(e.Path), "modules": orEmpty(modules)}
}

func modelPositions(ps map[string]validate.Position) map[string]any {
	out := make(map[string]any, len(ps))
	for ptr, p := range ps {
		out[ptr] = map[string]any{"file": filepath.ToSlash(p.File), "line": p.Line, "column": p.Column}
	}
	return out
}

func modelReferences(rs []validate.ModelReference) []any {
	out := []any{}
	for _, r := range rs {
		out = append(out, map[string]any{"from": r.From, "to": r.To, "through": r.Through})
	}
	return out
}

func modelProblems(ds []validate.Diagnostic) []any {
	out := []any{}
	for _, d := range ds {
		out = append(out, map[string]any{
			"file": filepath.ToSlash(d.File), "line": d.Line, "column": d.Column, "severity": string(d.Severity),
			"path": d.Path, "rule": string(d.Rule), "message": d.Message, "id": d.ID,
		})
	}
	return out
}
