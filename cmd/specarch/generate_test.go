package main

import (
	"testing"

	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// TestPluginRequestIdioms checks that each implementation file in a
// plug-in's request carries the idioms it uses, with the content of the
// idiom that applies, so a plug-in renders through them without reading
// the disk.
func TestPluginRequestIdioms(t *testing.T) {
	s := spec.Load("../../examples/library-lending/spec")
	l := loaded{spec: s}
	for _, impl := range s.Implementations {
		l.impls = append(l.impls, generate.Implementation{Node: source.Parse(impl.Data).Root, Path: impl.Path})
	}
	req := newPluginRequest(l, "openapi", "out", l.impls)
	if len(req.Implementations) != 1 {
		t.Fatalf("want one implementation file, got %d", len(req.Implementations))
	}
	got := map[string]pluginIdiom{}
	for _, i := range req.Implementations[0].Idioms {
		got[i.Name] = i
	}
	for _, name := range []string{"paginated-list", "type-rendering"} {
		i, ok := got[name]
		if !ok {
			t.Errorf("the request carries no %s", name)
			continue
		}
		if i.As != "shipped" || i.Content == nil {
			t.Errorf("%s is %s with content %v; want shipped, with the shipped idiom", name, i.As, i.Content != nil)
		}
	}
}
