package generate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Operations writes the operations guide (ADR-160): for the people who
// run the live system day to day, the environments and their hosts, a
// checklist of what to look at, the monitors with what to do when one is
// missed, the jobs with what a person does with an item a job keeps
// apart, the channels, dependencies and endpoints, the settings, and the
// backup, restore and rollback steps. A section the specification
// has nothing for is one sentence naming the key that fills it.
func Operations(root *yaml.Node, relRoot string, impls []Implementation, state *State) string {
	d := newDoc("operations", root, relRoot, impls, state)
	info := get(root, "info")
	envs := pairs(root, "environments")
	mons := pairs(root, "monitors")
	jobs := pairs(root, "jobs")
	deps := pairs(root, "dependencies")
	ops := operations(root)
	d.line("# %s: operations guide", str(info, "title"))
	d.blank()
	d.para(fmt.Sprintf("Version %s of the specification: %s, %s, %s and %s. For the people who run the live system: where it runs, what to look at each day, what to do when a monitor's objective is missed, what runs on its own, what it depends on, and how its data is saved and brought back.",
		str(info, "version"), countText(len(mons), "monitor", "monitors"), countText(len(jobs), "job", "jobs"), countText(len(deps), "dependency", "dependencies"), countText(len(ops), "endpoint", "endpoints")))
	d.draftNotice("operations")

	d.section("Where it runs")
	if len(envs) == 0 {
		d.para("The specification has no environments; each place the system runs, and the one a release is promoted to next, is written under environments.")
	} else {
		environmentsTable(d, envs)
	}
	for _, i := range impls {
		for _, dep := range pairs(i.Node, "deployments") {
			servers := items(dep.Value, "servers")
			if len(servers) == 0 {
				continue
			}
			d.heading(3, fmt.Sprintf("%s: %s", str(get(i.Node, "info"), "title"), dep.Key.Value))
			line := strings.TrimSpace(str(dep.Value, "description"))
			if e := str(dep.Value, "environment"); e != "" {
				line += " Environment: " + e + "."
			}
			d.para(line)
			d.line("| Host | Address | Runs | Description |")
			d.line("|---|---|---|---|")
			for n, s := range servers {
				var runs []string
				runs = append(runs, strs(get(s, "runs"), "packages")...)
				for _, t := range strs(get(s, "runs"), "targets") {
					runs = append(runs, "the "+t+" target")
				}
				if dp := str(s, "dependency"); dp != "" {
					runs = append(runs, "the dependency "+dp)
				}
				name := str(s, "name")
				if name == "" {
					name = fmt.Sprintf("host %d", n+1)
				}
				d.line("| %s | %s | %s | %s |", cell(name), cell(str(s, "url")), cell(strings.Join(runs, ", ")), cell(str(s, "description")))
			}
			d.blank()
		}
	}

	d.section("Daily checklist")
	if len(mons)+len(jobs)+len(deps) == 0 {
		d.para("The specification has no monitors, jobs or dependencies, so there is nothing to look at each day; they are written under monitors, jobs and dependencies.")
	} else {
		d.para("What to look at on the live system each day. Tick the last column as each is seen to be as expected.")
		d.line("| Check | What to look at | Expected | Seen |")
		d.line("|---|---|---|---|")
		for _, m := range mons {
			d.line("| Monitor %s (%s) | %s | %s | %s |", m.Key.Value, str(m.Value, "environment"), cell(str(m.Value, "description")), cell(str(m.Value, "objective")), cell(""))
		}
		for _, j := range jobs {
			expected := "It ran, and failed on no item."
			if str(get(j.Value, "retries"), "then") == "deadLetter" {
				expected = "It ran, and no item is kept apart after its last try."
				if get(j.Value, "deadLetter") != nil {
					expected = "It ran, and no item is kept apart after its last try; the section Jobs says where a kept item is."
				}
			}
			d.line("| Job %s | %s | %s | %s |", j.Key.Value, cell("Its last run, "+triggerText(get(j.Value, "trigger"))+"."), cell(expected), cell(""))
		}
		for _, dp := range deps {
			look := "That it answers"
			if p := str(dp.Value, "protocol"); p != "" {
				look += " over " + p
			}
			expected := "It answers."
			if t := str(dp.Value, "timeout"); t != "" {
				expected = "It answers within " + t + "."
			}
			d.line("| Dependency %s | %s | %s | %s |", dp.Key.Value, cell(look+"."), cell(expected), cell(""))
		}
		d.blank()
	}

	d.section("Monitors")
	if len(mons) == 0 {
		d.para("The specification has no monitors; what is watched on the live system, and the objective it must meet, is written under monitors.")
	}
	for _, m := range mons {
		d.heading(3, m.Key.Value)
		d.para(fmt.Sprintf("%s In %s. Objective: %s", strings.TrimSpace(str(m.Value, "description")), str(m.Value, "environment"), str(m.Value, "objective")))
		if v := strs(m.Value, "verifies"); len(v) > 0 {
			d.para("Verifies " + strings.Join(v, ", ") + ".")
		}
		d.explain(m.Value)
		for _, i := range impls {
			for _, dep := range pairs(i.Node, "deployments") {
				w := get(get(dep.Value, "monitors"), m.Key.Value)
				if w == nil {
					continue
				}
				line := fmt.Sprintf("Watched in %s (%s) with %s", dep.Key.Value, str(get(i.Node, "info"), "title"), str(w, "tool"))
				if desc := strings.TrimSuffix(strings.TrimSpace(str(w, "description")), "."); desc != "" {
					line += ": " + desc
				}
				line += "."
				if a := strings.TrimSuffix(strings.TrimSpace(str(w, "alert")), "."); a != "" {
					line += " Alert: " + a + "."
				}
				d.para(line)
			}
		}
		if response := get(m.Value, "response"); response != nil {
			d.para("When the objective is missed:")
			stepsTable(d, response)
		} else {
			d.para("The specification does not say what to do when the objective is missed; the steps are written under the monitor's response.")
		}
	}

	d.section("Jobs")
	if len(jobs) == 0 {
		d.para("The specification has no jobs; what the system runs on its own, on a schedule or for each message, is written under jobs.")
	} else {
		d.para("What the system runs on its own. A job acts as its role and may do only what the role's permissions allow.")
		d.line("| Job | Starts | Acts as | After a failure | Description |")
		d.line("|---|---|---|---|---|")
		for _, j := range jobs {
			d.line("| %s | %s | %s | %s | %s |", j.Key.Value, cell(triggerText(get(j.Value, "trigger"))), cell(str(j.Value, "role")), cell(retriesText(j.Value)), cell(str(j.Value, "description")))
		}
		d.blank()
		d.explainRows(rowsOf(jobs))
		for _, j := range jobs {
			if str(get(j.Value, "retries"), "then") != "deadLetter" {
				continue
			}
			d.heading(3, j.Key.Value+": an item kept apart")
			dl := get(j.Value, "deadLetter")
			if dl == nil {
				d.para("The specification does not say what a person does with an item the job keeps apart; who handles it, where it is kept and the steps to try it again or discard it are written under the job's deadLetter.")
				continue
			}
			d.para(strings.TrimSpace(str(dl, "description")))
			d.para("Handled by the " + str(dl, "role") + " role: a person who holds it looks at each kept item, then tries it again or discards it.")
			stepsTable(d, get(dl, "steps"))
		}
	}

	d.section("Channels")
	if chans := pairs(root, "channels"); len(chans) == 0 {
		d.para("The specification has no channels; the messages the system publishes and consumes are written under channels.")
	} else {
		publishers, consumers := map[string][]string{}, map[string][]string{}
		for _, o := range ops {
			for _, e := range strs(o.node, "emits") {
				publishers[e] = append(publishers[e], "operation "+o.id)
			}
		}
		for _, j := range jobs {
			for _, e := range strs(j.Value, "emits") {
				publishers[e] = append(publishers[e], "job "+j.Key.Value)
			}
			if c := str(get(j.Value, "trigger"), "consumes"); c != "" {
				consumers[c] = append(consumers[c], "job "+j.Key.Value)
			}
		}
		for _, w := range pairs(root, "workflows") {
			for _, e := range strs(w.Value, "emits") {
				publishers[e] = append(publishers[e], "workflow "+w.Key.Value)
			}
		}
		d.para("Each message, what carries it and who publishes and consumes it. A channel no dependency carries is the system's own.")
		d.line("| Channel | Carried by | Message | Published by | Consumed by |")
		d.line("|---|---|---|---|---|")
		for _, c := range chans {
			carried := strings.Join(strs(c.Value, "carriedBy"), ", ")
			if carried == "" {
				carried = "the system itself"
			}
			for _, msg := range pairs(c.Value, "messages") {
				key := c.Key.Value + "/" + msg.Key.Value
				d.line("| %s | %s | %s | %s | %s |", c.Key.Value, cell(carried), msg.Key.Value, cell(strings.Join(publishers[key], ", ")), cell(strings.Join(consumers[key], ", ")))
			}
		}
		d.blank()
		d.explainRows(rowsOf(chans))
	}

	d.section("Dependencies")
	if len(deps) == 0 {
		d.para("The specification has no dependencies; each system it calls or that carries its channels is written under dependencies.")
	} else {
		callers := map[string][]string{}
		for _, o := range ops {
			for _, c := range strs(o.node, "calls") {
				callers[calledDependency(c)] = append(callers[calledDependency(c)], "operation "+o.id)
			}
		}
		for _, j := range jobs {
			for _, c := range strs(j.Value, "calls") {
				callers[calledDependency(c)] = append(callers[calledDependency(c)], "job "+j.Key.Value)
			}
		}
		for _, g := range pairs(root, "gateways") {
			for _, dp := range gatewayDependencies(g.Value) {
				callers[dp] = append(callers[dp], "gateway "+g.Key.Value)
			}
		}
		for _, c := range pairs(root, "channels") {
			for _, dp := range strs(c.Value, "carriedBy") {
				callers[dp] = append(callers[dp], "channel "+c.Key.Value)
			}
		}
		d.para("What fails or waits when a dependency is down: every operation, job, gateway and channel that uses it.")
		d.line("| Dependency | Kind | Protocol | Time limit | Used by | Description |")
		d.line("|---|---|---|---|---|---|")
		for _, dp := range deps {
			kind := str(dp.Value, "kind")
			if kind == "" {
				kind = "system"
			}
			d.line("| %s | %s | %s | %s | %s | %s |", dp.Key.Value, kind, cell(str(dp.Value, "protocol")), cell(str(dp.Value, "timeout")), cell(strings.Join(callers[dp.Key.Value], ", ")), cell(str(dp.Value, "description")))
		}
		d.blank()
		d.explainRows(rowsOf(deps))
	}

	d.section("Endpoints")
	if len(ops) == 0 {
		d.para("The specification has no endpoints; each operation the system serves is written under paths.")
	} else {
		d.para("Every endpoint the system serves, with the permission a caller needs; public is open to everyone.")
		d.line("| Method | Path | Operation | Permission | Summary |")
		d.line("|---|---|---|---|---|")
		var rows []row
		for _, o := range ops {
			d.line("| %s | %s | %s | %s | %s |", strings.ToUpper(o.method), cell(o.path), o.id, cell(str(o.node, "permission")), cell(str(o.node, "summary")))
			rows = append(rows, row{label: o.id, node: o.node})
		}
		d.blank()
		d.explainRows(rows)
	}

	d.section("Settings")
	if cfg := pairs(root, "configuration"); len(cfg) == 0 {
		d.para("The specification has no settings; each value the system reads at run time is written under configuration.")
	} else {
		d.para("Every setting the system reads at run time. A secret's value is never written here; its description says where the value comes from.")
		d.line("| Setting | Type | Secret | Description |")
		d.line("|---|---|---|---|")
		for _, c := range cfg {
			secret := ""
			if str(c.Value, "secret") == "true" {
				secret = "yes"
			}
			d.line("| %s | %s | %s | %s |", markInternal(c.Key.Value, c.Value), cell(typeText(get(c.Value, "schema"))), secret, cell(str(c.Value, "description")))
		}
		d.blank()
		d.explainRows(rowsOf(cfg))
	}

	for _, p := range []struct{ key, title, none string }{
		{"backup", "Backing up", "The specification has no backup; how the data of the live system is saved is written under backup."},
		{"restore", "Restoring", "The specification has no restore; how saved data is brought back is written under restore."},
		{"rollback", "Rolling back", "The specification has no rollback; how a release is taken back is written under rollback."},
	} {
		d.section(p.title)
		n := get(root, p.key)
		if n == nil {
			d.para(p.none)
			continue
		}
		d.para(str(n, "description"))
		d.explain(n)
		stepsTable(d, get(n, "steps"))
	}
	d.sourcesIndex("Sources")
	return d.String()
}

// triggerText says in words what starts a job.
func triggerText(t *yaml.Node) string {
	switch {
	case str(t, "schedule") != "":
		return "on the schedule `" + str(t, "schedule") + "` (UTC)"
	case str(t, "every") != "":
		return "every " + str(t, "every")
	case str(t, "consumes") != "":
		return "for each " + str(t, "consumes")
	}
	return "when started by hand"
}

// retriesText says how often a job tries a failed item again and what
// becomes of it after the last try, with the role that handles a kept item.
func retriesText(job *yaml.Node) string {
	r := get(job, "retries")
	if r == nil {
		return "not tried again"
	}
	times := "times"
	if str(r, "limit") == "1" {
		times = "time"
	}
	text := fmt.Sprintf("tried again up to %s %s, then ", str(r, "limit"), times)
	if str(r, "then") == "deadLetter" {
		if role := str(get(job, "deadLetter"), "role"); role != "" {
			return text + "kept apart for the " + role + " role"
		}
		return text + "kept apart for a person"
	}
	return text + "dropped"
}
