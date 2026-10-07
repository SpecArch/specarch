package generate

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

func pairsOf(n *yaml.Node) []source.Pair { return source.Pairs(n) }
func itemsOf(n *yaml.Node) []*yaml.Node  { return source.Items(n) }

// sortedLinks orders requirement links by source, then by number where the
// ID is one: LIB-2 before LIB-10.
func sortedLinks[V any](m map[string]V) []string {
	keys := sortedKeys(m)
	sort.SliceStable(keys, func(i, j int) bool {
		pi, ni, _ := strings.Cut(keys[i], "-")
		pj, nj, _ := strings.Cut(keys[j], "-")
		if pi != pj {
			return pi < pj
		}
		a, errA := strconv.Atoi(ni)
		b, errB := strconv.Atoi(nj)
		if errA == nil && errB == nil {
			return a < b
		}
		return ni < nj
	})
	return keys
}

// implementation is chapter 7, from the implementation file: how one stack
// builds the design, where it runs, and the decisions that depend on it.
func implementation(d *doc, impl *yaml.Node) {
	d.heading(2, "7. Deployment and implementation")
	info := get(impl, "info")
	d.para(fmt.Sprintf("From the implementation file %s, version %s.", str(info, "title"), str(info, "version")))
	d.para(str(info, "description"))

	target := get(impl, "target")
	stack := []string{}
	if l := get(target, "language"); l != nil {
		stack = append(stack, fmt.Sprintf("language %s %s", str(l, "name"), str(l, "version")))
	}
	if t := get(target, "toolchain"); t != nil {
		stack = append(stack, fmt.Sprintf("toolchain %s %s", str(t, "name"), str(t, "version")))
	}
	if p := strs(target, "platforms"); len(p) > 0 {
		stack = append(stack, "platforms "+strings.Join(p, ", "))
	}
	d.para("Stack: " + strings.Join(stack, "; ") + ".")

	if libs := pairs(impl, "libraries"); len(libs) > 0 {
		d.heading(3, "Libraries")
		d.line("| Library | Version | Licence | Purpose |")
		d.line("|---|---|---|---|")
		for _, l := range libs {
			purpose := str(l.Value, "purpose")
			if scope := str(l.Value, "scope"); scope != "" && scope != "runtime" {
				purpose += " (" + scope + ")"
			}
			d.line("| %s | %s | %s | %s |", l.Key.Value, str(l.Value, "version"), str(l.Value, "licence"), cell(purpose))
		}
		d.blank()
	}
	if layout := pairs(impl, "layout"); len(layout) > 0 {
		d.heading(3, "Layout")
		d.line("| Path | Holds | Implements |")
		d.line("|---|---|---|")
		for _, l := range layout {
			d.line("| %s | %s | %s |", l.Key.Value, cell(str(l.Value, "description")), cell(strings.Join(strs(l.Value, "implements"), ", ")))
		}
		d.blank()
	}
	if maps := pairs(impl, "mappings"); len(maps) > 0 {
		d.heading(3, "Mappings")
		d.line("| Design object | Implemented by | Notes |")
		d.line("|---|---|---|")
		for _, m := range maps {
			d.line("| %s | %s | %s |", m.Key.Value, cell(str(m.Value, "target")), cell(str(m.Value, "description")))
		}
		d.blank()
	}
	if binds := pairs(impl, "bindings"); len(binds) > 0 {
		d.heading(3, "Bindings")
		for _, b := range binds {
			d.para(fmt.Sprintf("%s: %s. %s", b.Key.Value, str(b.Value, "framework"), strings.TrimSpace(str(b.Value, "description"))))
		}
	}
	if gens := pairs(impl, "generators"); len(gens) > 0 {
		d.heading(3, "Generators")
		d.line("| Target | Output folder | Settings |")
		d.line("|---|---|---|")
		for _, g := range gens {
			var set []string
			for _, k := range []string{"tool", "dialect", "platform", "framework"} {
				if v := str(g.Value, k); v != "" {
					set = append(set, k+" "+v)
				}
			}
			d.line("| %s | %s | %s |", g.Key.Value, str(g.Value, "output"), cell(strings.Join(set, ", ")))
		}
		d.blank()
	}
	if tasks := pairs(impl, "tasks"); len(tasks) > 0 {
		d.heading(3, "Tasks")
		d.line("| Task | Command | In CI |")
		d.line("|---|---|---|")
		for _, t := range tasks {
			ci := ""
			if str(t.Value, "ci") == "true" {
				ci = "yes"
			}
			d.line("| %s | `%s` | %s |", t.Key.Value, cell(str(t.Value, "run")), cell(ci))
		}
		d.blank()
	}
	if testing := get(impl, "testing"); testing != nil {
		d.heading(3, "Testing")
		d.para(fmt.Sprintf("Framework: %s. Run: `%s`.", str(testing, "framework"), str(testing, "run")))
		for _, k := range [][2]string{{"fixtures", "Fixtures"}, {"mocks", "Stand-ins"}, {"performance", "Performance"}} {
			if s := str(testing, k[0]); s != "" {
				d.para(k[1] + ": " + strings.TrimSpace(s))
			}
		}
		d.line("| Suite | Runs | Command |")
		d.line("|---|---|---|")
		for _, s := range pairs(testing, "suites") {
			runs := "tests of this implementation only"
			if names := strs(s.Value, "designTests"); len(names) > 0 {
				runs = "design tests " + strings.Join(names, ", ")
			}
			var of []string
			for _, subj := range items(s.Value, "designTestsOf") {
				for _, p := range pairsOf(subj) {
					of = append(of, p.Key.Value+" "+p.Value.Value)
				}
			}
			if len(of) > 0 {
				runs = "every design test of " + strings.Join(of, ", ")
			}
			d.line("| %s | %s | `%s` |", s.Key.Value, cell(runs), cell(str(s.Value, "run")))
		}
		d.blank()
	}
	if deps := pairs(impl, "deployments"); len(deps) > 0 {
		d.heading(3, "Deployments")
		for _, dep := range deps {
			var urls []string
			for _, s := range items(dep.Value, "servers") {
				urls = append(urls, str(s, "url"))
			}
			d.para(fmt.Sprintf("%s: %s Servers: %s.", dep.Key.Value, strings.TrimSpace(str(dep.Value, "description")), strings.Join(urls, ", ")))
		}
	}
	decisions(d, "", impl)
}
