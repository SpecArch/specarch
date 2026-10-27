package spec

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// Overlay reads the entry name of section from a design fragment file and
// puts it in the merged document in place of the specification's, key by
// key: each key the file gives takes the place of the specification's key
// of that name, and the others are kept (ADR-195). An entry only the file
// holds is added as the file gives it. Every node from the file remembers
// the file, so a problem of it is reported there; the specification's own
// files are not changed. It returns a reason when the file cannot be read
// (ioError) or does not hold the entry.
func (s *Spec) Overlay(path, section, name string) (reason string, ioError bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("cannot read %s (%s)", path, plainIOError(err)), true
	}
	doc := source.Parse(data)
	for _, p := range doc.Problems {
		s.problem(path, p.Line, p.Path, p.Rule, "%s", p.Message)
		s.detail(p.Detail)
	}
	entries := source.Child(doc.Root, section)
	given := source.Child(entries, name)
	if given == nil {
		var names []string
		for _, p := range source.Pairs(entries) {
			names = append(names, p.Key.Value)
		}
		sort.Strings(names)
		if len(names) == 0 {
			return fmt.Sprintf("%s holds no %s; write the %s entry %s under %s in it", path, section, section, name, section), false
		}
		return fmt.Sprintf("%s holds no %s entry %s; it holds %s", path, section, name, strings.Join(names, ", ")), false
	}
	s.record(given, path)
	if s.Root == nil || s.Root.Kind != yaml.MappingNode {
		return "", false
	}
	merged := source.Child(s.Root, section)
	if merged == nil {
		key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: section, Line: given.Line, Column: given.Column}
		merged = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Line: given.Line, Column: given.Column}
		s.Files[key], s.Files[merged] = path, path
		s.Root.Content = append(s.Root.Content, key, merged)
	}
	for i := 0; i+1 < len(merged.Content); i += 2 {
		if merged.Content[i].Value != name {
			continue
		}
		own := merged.Content[i+1]
		if own.Kind != yaml.MappingNode || given.Kind != yaml.MappingNode {
			merged.Content[i+1] = given
			s.Value = source.ValueOf(s.Root)
			return "", false
		}
		// The specification's keys in their order, each the file gives
		// taken from the file, then the keys only the file gives.
		out := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Line: given.Line, Column: given.Column}
		s.Files[out] = path
		fromFile := map[string]bool{}
		for _, p := range source.Pairs(own) {
			if k := source.Key(given, p.Key.Value); k != nil {
				out.Content = append(out.Content, k, source.Child(given, p.Key.Value))
				fromFile[p.Key.Value] = true
				continue
			}
			out.Content = append(out.Content, p.Key, p.Value)
		}
		for _, p := range source.Pairs(given) {
			if !fromFile[p.Key.Value] {
				out.Content = append(out.Content, p.Key, p.Value)
			}
		}
		merged.Content[i+1] = out
		s.Value = source.ValueOf(s.Root)
		return "", false
	}
	key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name, Line: given.Line, Column: given.Column}
	s.Files[key] = path
	merged.Content = append(merged.Content, key, given)
	s.Value = source.ValueOf(s.Root)
	return "", false
}
