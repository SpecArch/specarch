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

// deployment is chapter 7: the deployment stage of the specification
// (environments, configuration, release, rollback, migrations), the
// commissioning stage (checks and sign-off), the operation stage
// (monitors), then each implementation
// file: how one stack builds the design, where it runs, and the decisions
// that depend on it.
func deployment(d *doc, root *yaml.Node, impls []Implementation) {
	envs := pairs(root, "environments")
	cfg := pairs(root, "configuration")
	release := get(root, "release")
	rollback := get(root, "rollback")
	handover := get(root, "handover")
	backup := get(root, "backup")
	restore := get(root, "restore")
	migs := pairs(root, "migrations")
	checks := pairs(root, "checks")
	signoff := get(root, "signoff")
	if len(envs)+len(cfg)+len(migs)+len(checks)+len(impls)+len(pairs(root, "monitors")) == 0 && release == nil && rollback == nil && signoff == nil && handover == nil && backup == nil && restore == nil {
		return
	}
	d.heading(2, tr("7. Deployment and implementation"))
	if len(envs) > 0 {
		d.heading(3, tr("Environments"))
		environmentsTable(d, envs)
	}
	if len(cfg) > 0 {
		d.heading(3, tr("Configuration"))
		d.line(tr("| Setting | Type | Secret | Description |"))
		d.line("|---|---|---|---|")
		for _, c := range cfg {
			secret := ""
			if str(c.Value, "secret") == "true" {
				secret = tr("yes")
			}
			d.line("| %s | %s | %s | %s |", c.Key.Value, cell(typeText(get(c.Value, "schema"))), cell(secret), cell(str(c.Value, "description")))
		}
		d.blank()
		d.explainRows(rowsOf(cfg))
	}
	if release != nil {
		d.heading(3, tr("Release"))
		d.para(str(release, "description"))
		d.explain(release)
		stepsTable(d, get(release, "steps"))
	}
	if rollback != nil {
		d.heading(3, tr("Rollback"))
		d.para(str(rollback, "description"))
		d.explain(rollback)
		stepsTable(d, get(rollback, "steps"))
	}
	for _, p := range []struct {
		node  *yaml.Node
		title string
	}{{backup, "Backup"}, {restore, "Restore"}} {
		if p.node != nil {
			d.heading(3, p.title)
			d.para(str(p.node, "description"))
			d.explain(p.node)
			stepsTable(d, get(p.node, "steps"))
		}
	}
	if handover != nil {
		d.heading(3, tr("Handover"))
		handoverText(d, root, handover)
	}
	for _, m := range migs {
		d.heading(3, tr("Migration ")+m.Key.Value)
		d.para(str(m.Value, "description"))
		d.explain(m.Value)
		stepsTable(d, get(m.Value, "steps"))
		if rb := get(m.Value, "rollback"); rb != nil {
			d.para(tr("Reversed by:"))
			stepsTable(d, rb)
		}
	}
	if len(checks) > 0 {
		d.heading(3, tr("Commissioning checks"))
		d.para(tr("Run on the installed system before it is handed over. The results of each run are records kept outside the specification."))
		for _, c := range checks {
			d.heading(4, fmt.Sprintf(tr("%s (%s, in %s)"), c.Key.Value, str(c.Value, "kind"), str(c.Value, "environment")))
			d.para(str(c.Value, "description"))
			d.explain(c.Value)
			stepsTable(d, get(c.Value, "steps"))
		}
	}
	if signoff != nil {
		d.heading(3, tr("Sign-off"))
		d.para(tr("The system is accepted when:"))
		for _, c := range strs(signoff, "criteria") {
			d.line("- %s", c)
		}
		d.blank()
		var signers []string
		for _, s := range items(signoff, "signers") {
			signers = append(signers, str(s, "role"))
		}
		d.para(tr("Signed by: ") + strings.Join(signers, ", ") + ".")
		d.explain(signoff)
	}
	if mons := pairs(root, "monitors"); len(mons) > 0 {
		d.heading(3, tr("Monitors"))
		monitorsTable(d, mons)
	}
	for _, i := range impls {
		implementation(d, root, i.Node, i.Idioms)
	}
}

// monitorsTable lists the monitors of the operation stage under a heading
// the caller writes.
func monitorsTable(d *doc, mons []source.Pair) {
	d.para(tr("What is watched on the live system, and the objective each must meet."))
	d.line(tr("| Monitor | Environment | Measures | Objective | Verifies |"))
	d.line("|---|---|---|---|---|")
	for _, m := range mons {
		d.line("| %s | %s | %s | %s | %s |", m.Key.Value, str(m.Value, "environment"), cell(str(m.Value, "description")), cell(str(m.Value, "objective")), cell(strings.Join(strs(m.Value, "verifies"), ", ")))
	}
	d.blank()
	d.explainRows(rowsOf(mons))
}

// deploymentMonitors is the table of how one installation watches the
// monitors.
func deploymentMonitors(d *doc, dep *yaml.Node) {
	mons := pairs(dep, "monitors")
	if len(mons) == 0 {
		return
	}
	d.line(tr("| Monitor | Tool | How | Alert |"))
	d.line("|---|---|---|---|")
	for _, m := range mons {
		d.line("| %s | %s | %s | %s |", m.Key.Value, cell(str(m.Value, "tool")), cell(str(m.Value, "description")), cell(str(m.Value, "alert")))
	}
	d.blank()
}

func stepsTable(d *doc, steps *yaml.Node) {
	list := itemsOf(steps)
	if len(list) == 0 {
		return
	}
	d.line(tr("| Step | Action | Check |"))
	d.line("|---|---|---|")
	for i, s := range list {
		d.line("| %d. %s | %s | %s |", i+1, cell(str(s, "name")), cell(str(s, "action")), cell(str(s, "check")))
	}
	d.blank()
	d.explainRows(stepRows(steps))
}

// implementation is one implementation file's part of chapter 7.
func implementation(d *doc, root, impl *yaml.Node, idioms []IdiomUse) {
	info := get(impl, "info")
	d.heading(3, tr("Implementation: ")+str(info, "title"))
	d.para(fmt.Sprintf(tr("From the implementation file version %s."), str(info, "version")))
	d.para(str(info, "description"))

	target := get(impl, "stack")
	stack := []string{}
	if l := get(target, "language"); l != nil {
		stack = append(stack, fmt.Sprintf(tr("language %s %s"), str(l, "name"), str(l, "version")))
	}
	if t := get(target, "toolchain"); t != nil {
		stack = append(stack, fmt.Sprintf(tr("toolchain %s %s"), str(t, "name"), str(t, "version")))
	}
	if p := strs(target, "platforms"); len(p) > 0 {
		stack = append(stack, tr("platforms ")+strings.Join(p, ", "))
	}
	d.para(tr("Stack: ") + strings.Join(stack, "; ") + ".")

	if libs := pairs(impl, "libraries"); len(libs) > 0 {
		d.heading(4, tr("Libraries"))
		d.line(tr("| Library | Version | Licence | Purpose |"))
		d.line("|---|---|---|---|")
		for _, l := range libs {
			purpose := str(l.Value, "purpose")
			if scope := str(l.Value, "scope"); scope != "" && scope != "runtime" {
				purpose += " (" + scope + ")"
			}
			d.line("| %s | %s | %s | %s |", l.Key.Value, str(l.Value, "version"), str(l.Value, "licence"), cell(purpose))
		}
		d.blank()
		d.explainRows(rowsOf(libs))
	}
	if layout := pairs(impl, "layout"); len(layout) > 0 {
		d.heading(4, tr("Layout"))
		d.line(tr("| Path | Holds | Implements |"))
		d.line("|---|---|---|")
		for _, l := range layout {
			d.line("| %s | %s | %s |", l.Key.Value, cell(str(l.Value, "description")), cell(strings.Join(strs(l.Value, "implements"), ", ")))
		}
		d.blank()
	}
	if maps := pairs(impl, "mappings"); len(maps) > 0 {
		d.heading(4, tr("Mappings"))
		d.line(tr("| Design object | Implemented by | Notes |"))
		d.line("|---|---|---|")
		for _, m := range maps {
			notes := str(m.Value, "description")
			if by := str(m.Value, "ownedBy"); by != "" {
				// An element another stakeholder owns is described here and
				// generated nowhere (ADR-046).
				notes = strings.TrimSpace(tr("Owned by ") + by + tr(", not generated. ") + notes)
			}
			if ixs := items(m.Value, "indexes"); len(ixs) > 0 {
				// The plain indexes of the table (ADR-121).
				var named []string
				for _, ix := range ixs {
					one := strings.Join(strs(ix, "fields"), ", ")
					if n := str(ix, "name"); n != "" {
						one = n + " (" + one + ")"
					}
					named = append(named, one)
				}
				notes = strings.TrimSpace(notes + tr(" Indexed by ") + strings.Join(named, "; ") + ".")
			}
			d.line("| %s | %s | %s |", m.Key.Value, cell(str(m.Value, "target")), cell(notes))
		}
		d.blank()
	}
	if binds := pairs(impl, "bindings"); len(binds) > 0 {
		d.heading(4, tr("Bindings"))
		for _, b := range binds {
			d.para(fmt.Sprintf("%s: %s. %s", b.Key.Value, str(b.Value, "framework"), strings.TrimSpace(str(b.Value, "description"))))
		}
		d.explainRows(rowsOf(binds))
	}
	if gens := pairs(impl, "targets"); len(gens) > 0 {
		d.heading(4, tr("Targets"))
		d.line(tr("| Target | Output folder | Settings |"))
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
		d.explainRows(rowsOf(gens))
	}
	if tasks := pairs(impl, "tasks"); len(tasks) > 0 {
		d.heading(4, tr("Tasks"))
		d.line(tr("| Task | Command | In CI |"))
		d.line("|---|---|---|")
		for _, t := range tasks {
			ci := ""
			if str(t.Value, "ci") == "true" {
				ci = tr("yes")
			}
			d.line("| %s | `%s` | %s |", t.Key.Value, cell(str(t.Value, "run")), cell(ci))
		}
		d.blank()
	}
	if testing := get(impl, "testing"); testing != nil {
		d.heading(4, tr("Testing"))
		d.para(fmt.Sprintf(tr("Framework: %s. Run: `%s`."), str(testing, "framework"), str(testing, "run")))
		for _, k := range [][2]string{{"fixtures", "Fixtures"}, {"mocks", "Stand-ins"}, {"performance", "Performance"}} {
			if s := str(testing, k[0]); s != "" {
				d.para(k[1] + ": " + strings.TrimSpace(s))
			}
		}
		d.line(tr("| Suite | Level | Runs | Command |"))
		d.line("|---|---|---|---|")
		for _, s := range pairs(testing, "suites") {
			runs := tr("tests of this implementation only")
			if names := strs(s.Value, "designTests"); len(names) > 0 {
				runs = tr("design tests ") + strings.Join(names, ", ")
			}
			var of []string
			for _, subj := range items(s.Value, "designTestsOf") {
				for _, p := range pairsOf(subj) {
					of = append(of, p.Key.Value+" "+p.Value.Value)
				}
			}
			if len(of) > 0 {
				runs = tr("every design test of ") + strings.Join(of, ", ")
			}
			d.line("| %s | %s | %s | `%s` |", s.Key.Value, str(s.Value, "level"), cell(runs), cell(str(s.Value, "run")))
		}
		d.blank()
		d.explainRows(pairRows(testing, "suites"))
	}
	if len(idioms) > 0 {
		d.heading(4, tr("Idioms"))
		d.para(tr("How this implementation does each recurring concern: the idioms SpecArch ships apply unless the file excludes or overrides one, and an override replaces only the parts it names (docs/idioms.md)."))
		d.line(tr("| Idiom | Version | Applies as | Parts the project replaces |"))
		d.line("|---|---|---|---|")
		for _, u := range idioms {
			as := u.As
			switch u.As {
			case "overridden":
				as = tr("overridden, copied from ") + u.From
			case "project":
				as = tr("the project's own")
			}
			d.line("| %s | %s | %s | %s |", u.Name, u.Version, cell(as), cell(strings.Join(u.Parts, ", ")))
		}
		d.blank()
		for _, u := range idioms {
			if u.Why != "" {
				d.para(fmt.Sprintf(tr("**Insight on %s:** %s"), u.Name, oneParagraph(u.Why)))
			}
		}
	}
	if deps := pairs(impl, "deployments"); len(deps) > 0 {
		d.heading(4, tr("Deployments"))
		for _, dep := range deps {
			var urls []string
			for _, s := range items(dep.Value, "servers") {
				urls = append(urls, str(s, "url"))
			}
			env := ""
			if e := str(dep.Value, "environment"); e != "" {
				env = tr(" Environment: ") + e + "."
			}
			var settings []string
			for _, c := range pairs(dep.Value, "configuration") {
				settings = append(settings, c.Key.Value+" "+c.Value.Value)
			}
			line := fmt.Sprintf("%s: %s%s", dep.Key.Value, strings.TrimSpace(str(dep.Value, "description")), env)
			if len(urls) > 0 {
				line += tr(" Servers: ") + strings.Join(urls, ", ") + "."
			}
			if len(settings) > 0 {
				line += tr(" Settings: ") + strings.Join(settings, ", ") + "."
			}
			if b := get(dep.Value, "build"); b != nil {
				line += fmt.Sprintf(tr(" Built by the task %s"), str(b, "task"))
				if n := len(items(b, "baseImages")); n > 0 {
					line += fmt.Sprintf(tr(" from %s pinned by digest"), countText(n, tr("base image"), tr("base images")))
				}
				line += "."
			}
			if m := get(dep.Value, "method"); m != nil {
				line += tr(" Installed with ") + str(m, "tool")
				if f := str(m, "files"); f != "" {
					line += tr(" from ") + f
				}
				line += "."
			}
			d.para(line)
			deploymentPicture(d, root, dep.Key.Value, dep.Value)
			infrastructureTable(d, get(dep.Value, "infrastructure"))
			deploymentMonitors(d, dep.Value)
		}
		d.explainRows(rowsOf(deps))
	}
	decisions(d, "", impl)
}
