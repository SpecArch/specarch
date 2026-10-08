package validate

import (
	"io/fs"
	"path"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/idioms"
	"github.com/SpecArch/specarch/internal/source"
)

// TestShippedIdiomsValid checks every shipped idiom against the idiom
// schema, and that each is named after its file and cites only the
// sources it declares.
func TestShippedIdiomsValid(t *testing.T) {
	paths, err := fs.Glob(idioms.Shipped, "*/*"+IdiomSuffix)
	if err != nil || len(paths) == 0 {
		t.Fatalf("no shipped idioms found (%v)", err)
	}
	for _, p := range paths {
		data, _ := idioms.Shipped.ReadFile(p)
		doc := source.Parse(data)
		if doc.Root == nil {
			t.Errorf("%s does not parse: %v", p, doc.Problems)
			continue
		}
		c := &checker{file: p, root: doc.Root}
		c.checkSchema(KindIdiom, doc.Value)
		for _, d := range c.diags {
			t.Errorf("%s", d)
		}
		i := Idiom{Path: p, Root: doc.Root}
		if path.Base(p) != i.Name()+IdiomSuffix {
			t.Errorf("%s holds the idiom %s; name the file after it", p, i.Name())
		}
		sources := source.Child(doc.Root, "sources")
		walk(doc.Root, nil, func(n *yaml.Node, at []string) {
			for _, cite := range source.Items(source.Child(n, "cites")) {
				if key := source.Str(source.Child(cite, "source")); source.Child(sources, key) == nil {
					t.Errorf("%s cites %s at %s, which its sources do not declare", p, key, source.Pointer(at...))
				}
			}
		})
	}
}
