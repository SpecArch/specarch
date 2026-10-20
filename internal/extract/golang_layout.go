package extract

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// layoutKey is the layout entry a folder of the module belongs to: the key
// that is the folder, or else the longest key that holds it; "" when none
// does.
func layoutKey(keys []string, folder string) string {
	best := ""
	for _, k := range keys {
		if (folder == k || strings.HasPrefix(folder, k+"/")) && len(k) > len(best) {
			best = k
		}
	}
	return best
}

// layoutDependsOn reads, for each entry of the implementation file's
// layout, the entries of the same module its files import (ADR-133): an
// import is what the code says, so an entry whose files import none of the
// module's other packages gets an empty list, and an entry with no Go file
// read gets none. A test file's imports are left out, since a test is no
// part of the package's structure.
func (g *goReader) layoutDependsOn() *yaml.Node {
	if g.layout == nil || g.modPath == "" {
		return nil
	}
	var keys []string
	for _, p := range source.Pairs(g.layout) {
		keys = append(keys, p.Key.Value)
	}
	read := map[string]bool{}
	deps := map[string]map[string]bool{}
	for _, f := range g.files {
		if strings.HasSuffix(f.path, "_test.go") {
			continue
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(path.Dir(f.path), g.modRoot), "/")
		if g.modRoot == "." {
			rel = path.Dir(f.path)
		}
		from := layoutKey(keys, rel)
		if from == "" {
			continue
		}
		read[from] = true
		for _, ip := range f.imports {
			if ip != g.modPath && !strings.HasPrefix(ip, g.modPath+"/") {
				continue
			}
			to := layoutKey(keys, strings.TrimPrefix(strings.TrimPrefix(ip, g.modPath), "/"))
			if to == "" || to == from {
				continue
			}
			if deps[from] == nil {
				deps[from] = map[string]bool{}
			}
			deps[from][to] = true
		}
	}
	out := mapping()
	edges := 0
	for _, p := range source.Pairs(g.layout) {
		if !read[p.Key.Value] {
			continue
		}
		var list []string
		for to := range deps[p.Key.Value] {
			list = append(list, to)
		}
		sort.Strings(list)
		edges += len(list)
		entry := mapping("description", scalar(child(p.Value, "description")))
		if list == nil {
			list = []string{}
		}
		set(entry, "dependsOn", list)
		set(out, p.Key.Value, entry)
	}
	if len(out.Content) == 0 {
		return nil
	}
	g.res.say("layout: read the imports of %s of the implementation file's layout, %s within the module", plural(len(out.Content)/2, "entry"), plural(edges, "dependency"))
	return out
}

// writeLayout puts the layout entries with what each depends on into the
// implementation fragment, before the handlers' mappings.
func (g *goReader) writeLayout(writeImplementation bool) {
	if !writeImplementation {
		return
	}
	layout := g.layoutDependsOn()
	if layout == nil {
		return
	}
	frag := mapping("layout", layout)
	if g.res.Implementation != nil {
		frag.Content = append(frag.Content, g.res.Implementation.Content...)
	}
	g.res.Implementation = frag
	note := fmt.Sprintf("The layout entries specarch extract go read from the Go source under %s at commit %s, each with the entries of the same module its files import under dependsOn; they replace the dependsOn of the same keys under layout in the implementation file.", strings.Join(g.r.Paths, ", "), g.r.Commit)
	if g.res.ImplementationNote != "" {
		note = g.res.ImplementationNote + " " + note
	}
	g.res.ImplementationNote = note
}
