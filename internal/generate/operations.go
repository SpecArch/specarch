package generate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// Operations writes the operations guide (ADR-160): for the people who
// run the live system day to day, the environments and their hosts, a
// checklist of what to look at, the monitors with what to do when one is
// missed, the jobs with what a person does with an item a job keeps
// apart, the processes the service manager runs (ADR-211), the channels, dependencies and endpoints, the settings, and the
// backup, restore and rollback steps. A section the specification
// has nothing for is one sentence naming the key that fills it.
func Operations(root *yaml.Node, relRoot string, impls []Implementation, state *State) string {
	d := newDoc("operations", root, relRoot, impls, state)
	info := get(root, "info")
	envs := pairs(root, "environments")
	mons := pairs(root, "monitors")
	jobs := pairs(root, "jobs")
	procs := pairs(root, "processes")
	deps := pairs(root, "dependencies")
	ops := operations(root)
	d.line(tr("# %s: operations guide"), str(info, "title"))
	d.blank()
	d.para(fmt.Sprintf(tr("Version %s of the specification: %s, %s, %s and %s. For the people who run the live system: where it runs, what to look at each day, what to do when a monitor's objective is missed, what runs on its own, what it depends on, and how its data is saved and brought back."),
		str(info, "version"), countText(len(mons), tr("monitor"), tr("monitors")), countText(len(jobs), tr("job"), tr("jobs")), countText(len(deps), tr("dependency"), tr("dependencies")), countText(len(ops), tr("endpoint"), tr("endpoints"))))
	d.draftNotice("operations")

	d.section(tr("Where it runs"))
	if len(envs) == 0 {
		d.para(tr("The specification has no environments; each place the system runs, and the one a release is promoted to next, is written under environments."))
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
				line += tr(" Environment: ") + e + "."
			}
			d.para(line)
			d.line(tr("| Host | Address | Runs | Description |"))
			d.line("|---|---|---|---|")
			for n, s := range servers {
				var runs []string
				runs = append(runs, strs(get(s, "runs"), "packages")...)
				for _, t := range strs(get(s, "runs"), "targets") {
					runs = append(runs, tr("the ")+t+tr(" target"))
				}
				if dp := str(s, "dependency"); dp != "" {
					runs = append(runs, tr("the dependency ")+dp)
				}
				name := str(s, "name")
				if name == "" {
					name = fmt.Sprintf(tr("host %d"), n+1)
				}
				d.line("| %s | %s | %s | %s |", cell(name), cell(str(s, "url")), cell(strings.Join(runs, ", ")), cell(str(s, "description")))
			}
			d.blank()
		}
	}

	d.section(tr("Daily checklist"))
	if len(mons)+len(jobs)+len(procs)+len(deps) == 0 {
		d.para(tr("The specification has no monitors, jobs, processes or dependencies, so there is nothing to look at each day; they are written under monitors, jobs, processes and dependencies."))
	} else {
		d.para(tr("What to look at on the live system each day. Tick the last column as each is seen to be as expected."))
		d.line(tr("| Check | What to look at | Expected | Seen |"))
		d.line("|---|---|---|---|")
		for _, m := range mons {
			d.line(tr("| Monitor %s (%s) | %s | %s | %s |"), m.Key.Value, str(m.Value, "environment"), cell(str(m.Value, "description")), cell(str(m.Value, "objective")), cell(""))
		}
		for _, j := range jobs {
			expected := tr("It ran, and failed on no item.")
			if str(get(j.Value, "retries"), "then") == "deadLetter" {
				expected = tr("It ran, and no item is kept apart after its last try.")
				if get(j.Value, "deadLetter") != nil {
					expected = tr("It ran, and no item is kept apart after its last try; the section Jobs says where a kept item is.")
				}
			}
			d.line(tr("| Job %s | %s | %s | %s |"), j.Key.Value, cell(tr("Its last run, ")+triggerText(get(j.Value, "trigger"))+"."), cell(expected), cell(""))
		}
		for _, p := range procs {
			look, expected := tr("Its last run."), tr("It ran, and exited 0.")
			if str(p.Value, "runs") == "always" {
				look, expected = tr("That it is running."), tr("It is running, and has not been started again since the last check.")
				if h := strings.TrimSpace(str(get(p.Value, "health"), "description")); h != "" {
					expected = tr("It is running and healthy: ") + strings.TrimSuffix(h, ".") + "."
				}
			}
			d.line(tr("| Process %s | %s | %s | %s |"), p.Key.Value, cell(look), cell(expected), cell(""))
		}
		for _, dp := range deps {
			look := tr("That it answers")
			if p := str(dp.Value, "protocol"); p != "" {
				look += tr(" over ") + p
			}
			expected := tr("It answers.")
			if t := str(dp.Value, "timeout"); t != "" {
				expected = tr("It answers within ") + t + "."
			}
			d.line(tr("| Dependency %s | %s | %s | %s |"), dp.Key.Value, cell(look+"."), cell(expected), cell(""))
		}
		d.blank()
	}

	d.section(tr("Monitors"))
	if len(mons) == 0 {
		d.para(tr("The specification has no monitors; what is watched on the live system, and the objective it must meet, is written under monitors."))
	}
	for _, m := range mons {
		d.heading(3, m.Key.Value)
		d.para(fmt.Sprintf(tr("%s In %s. Objective: %s"), strings.TrimSpace(str(m.Value, "description")), str(m.Value, "environment"), str(m.Value, "objective")))
		if v := strs(m.Value, "verifies"); len(v) > 0 {
			d.para(tr("Verifies ") + strings.Join(v, ", ") + ".")
		}
		d.explain(m.Value)
		for _, i := range impls {
			for _, dep := range pairs(i.Node, "deployments") {
				w := get(get(dep.Value, "monitors"), m.Key.Value)
				if w == nil {
					continue
				}
				line := fmt.Sprintf(tr("Watched in %s (%s) with %s"), dep.Key.Value, str(get(i.Node, "info"), "title"), str(w, "tool"))
				if desc := strings.TrimSuffix(strings.TrimSpace(str(w, "description")), "."); desc != "" {
					line += ": " + desc
				}
				line += "."
				if a := strings.TrimSuffix(strings.TrimSpace(str(w, "alert")), "."); a != "" {
					line += tr(" Alert: ") + a + "."
				}
				d.para(line)
			}
		}
		if response := get(m.Value, "response"); response != nil {
			d.para(tr("When the objective is missed:"))
			stepsTable(d, response)
		} else {
			d.para(tr("The specification does not say what to do when the objective is missed; the steps are written under the monitor's response."))
		}
	}

	d.section(tr("Jobs"))
	if len(jobs) == 0 {
		d.para(tr("The specification has no jobs; what the system runs on its own, on a schedule or for each message, is written under jobs."))
	} else {
		d.para(tr("What the system runs on its own. A job acts as its role and may do only what the role's permissions allow."))
		d.line(tr("| Job | Starts | Acts as | After a failure | Description |"))
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
			d.heading(3, j.Key.Value+tr(": an item kept apart"))
			dl := get(j.Value, "deadLetter")
			if dl == nil {
				d.para(tr("The specification does not say what a person does with an item the job keeps apart; who handles it, where it is kept and the steps to try it again or discard it are written under the job's deadLetter."))
				continue
			}
			d.para(strings.TrimSpace(str(dl, "description")))
			d.para(tr("Handled by the ") + str(dl, "role") + tr(" role: a person who holds it looks at each kept item, then tries it again or discards it."))
			stepsTable(d, get(dl, "steps"))
		}
	}

	d.section(tr("Processes"))
	if len(procs) == 0 {
		d.para(tr("The specification has no processes; a program a service manager starts and watches, kept running, started on a schedule or on demand, is written under processes."))
	} else {
		d.para(tr("The programs the service manager starts and watches. A process acts as no role: the jobs it runs act as theirs."))
		d.line(tr("| Process | Runs | Jobs | Restarted | Description |"))
		d.line("|---|---|---|---|---|")
		for _, p := range procs {
			d.line("| %s | %s | %s | %s | %s |", p.Key.Value, cell(runsText(p.Value)), cell(strings.Join(strs(p.Value, "jobs"), ", ")), cell(restartText(get(p.Value, "restart"))), cell(str(p.Value, "description")))
		}
		d.blank()
		d.explainRows(rowsOf(procs))
		for _, p := range procs {
			d.heading(3, p.Key.Value)
			if c := strs(p.Value, "configuration"); len(c) > 0 {
				d.para(tr("Reads the settings ") + strings.Join(c, ", ") + tr(" when it starts; the section Settings says what each is."))
			}
			if sigs := pairs(p.Value, "signals"); len(sigs) > 0 {
				d.line(tr("| Signal | What it does |"))
				d.line("|---|---|")
				for _, sg := range sigs {
					does := strings.TrimSpace(str(sg.Value, "description"))
					if w := str(sg.Value, "within"); w != "" {
						does += tr(", within ") + w
					}
					d.line("| %s | %s |", sg.Key.Value, cell(does))
				}
				d.blank()
			} else {
				d.para(tr("The specification does not say which signals it honours; each is written under the process's signals."))
			}
			if codes := pairs(p.Value, "exitCodes"); len(codes) > 0 {
				never := map[string]bool{}
				for _, e := range strs(get(p.Value, "restart"), "exceptExitCodes") {
					never[e] = true
				}
				d.line(tr("| Exit status | Meaning | Started again |"))
				d.line("|---|---|---|")
				for _, c := range codes {
					again := tr("as Restarted says")
					if never[c.Key.Value] {
						again = tr("never")
					}
					d.line("| %s | %s | %s |", c.Key.Value, cell(source.Str(c.Value)), again)
				}
				d.blank()
			}
			if h := get(p.Value, "health"); h != nil {
				line := tr("Healthy when ") + strings.TrimSuffix(strings.TrimSpace(str(h, "description")), ".") + "."
				if e := str(h, "every"); e != "" {
					line += tr(" Looked at every ") + e + "."
				}
				d.para(line)
			}
			if m := strs(p.Value, "monitors"); len(m) > 0 {
				which := tr("the monitor ") + m[0]
				if len(m) > 1 {
					which = tr("the monitors ") + strings.Join(m, ", ")
				}
				d.para(tr("Watched by ") + which + tr("; the section Monitors says what to do when one is missed."))
			} else if get(get(p.Value, "restart"), "limit") != nil {
				d.para(tr("The specification names no monitor that watches it, so nobody is told when the service manager gives it up; the monitor is named under the process's monitors."))
			}
		}
	}

	d.section(tr("Channels"))
	if chans := pairs(root, "channels"); len(chans) == 0 {
		d.para(tr("The specification has no channels; the messages the system publishes and consumes are written under channels."))
	} else {
		publishers, consumers := map[string][]string{}, map[string][]string{}
		for _, o := range ops {
			for _, e := range strs(o.node, "emits") {
				publishers[e] = append(publishers[e], tr("operation ")+o.id)
			}
		}
		for _, j := range jobs {
			for _, e := range strs(j.Value, "emits") {
				publishers[e] = append(publishers[e], tr("job ")+j.Key.Value)
			}
			if c := str(get(j.Value, "trigger"), "consumes"); c != "" {
				consumers[c] = append(consumers[c], tr("job ")+j.Key.Value)
			}
		}
		for _, w := range pairs(root, "workflows") {
			for _, e := range strs(w.Value, "emits") {
				publishers[e] = append(publishers[e], tr("workflow ")+w.Key.Value)
			}
		}
		d.para(tr("Each message, what carries it and who publishes and consumes it. A channel no dependency carries is the system's own."))
		d.line(tr("| Channel | Carried by | Message | Published by | Consumed by |"))
		d.line("|---|---|---|---|---|")
		for _, c := range chans {
			carried := strings.Join(strs(c.Value, "carriedBy"), ", ")
			if carried == "" {
				carried = tr("the system itself")
			}
			for _, msg := range pairs(c.Value, "messages") {
				key := c.Key.Value + "/" + msg.Key.Value
				d.line("| %s | %s | %s | %s | %s |", c.Key.Value, cell(carried), msg.Key.Value, cell(strings.Join(publishers[key], ", ")), cell(strings.Join(consumers[key], ", ")))
			}
		}
		d.blank()
		d.explainRows(rowsOf(chans))
	}

	d.section(tr("Dependencies"))
	if len(deps) == 0 {
		d.para(tr("The specification has no dependencies; each system it calls or that carries its channels is written under dependencies."))
	} else {
		callers := map[string][]string{}
		for _, o := range ops {
			for _, c := range calls(o.node) {
				callers[calledDependency(c)] = append(callers[calledDependency(c)], tr("operation ")+o.id)
			}
		}
		for _, j := range jobs {
			for _, c := range calls(j.Value) {
				callers[calledDependency(c)] = append(callers[calledDependency(c)], tr("job ")+j.Key.Value)
			}
		}
		for _, g := range pairs(root, "gateways") {
			for _, dp := range gatewayDependencies(g.Value) {
				callers[dp] = append(callers[dp], tr("gateway ")+g.Key.Value)
			}
		}
		for _, c := range pairs(root, "channels") {
			for _, dp := range strs(c.Value, "carriedBy") {
				callers[dp] = append(callers[dp], tr("channel ")+c.Key.Value)
			}
		}
		d.para(tr("What fails or waits when a dependency is down: every operation, job, gateway and channel that uses it."))
		d.line(tr("| Dependency | Kind | Protocol | Time limit | Used by | Description |"))
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

	d.section(tr("Endpoints"))
	if len(ops) == 0 {
		d.para(tr("The specification has no endpoints; each operation the system serves is written under paths."))
	} else {
		d.para(tr("Every endpoint the system serves, with the permission a caller needs; public is open to everyone."))
		d.line(tr("| Method | Path | Operation | Permission | Summary |"))
		d.line("|---|---|---|---|---|")
		var rows []row
		for _, o := range ops {
			d.line("| %s | %s | %s | %s | %s |", strings.ToUpper(o.method), cell(o.path), o.id, cell(str(o.node, "permission")), cell(str(o.node, "summary")))
			rows = append(rows, row{label: o.id, node: o.node})
		}
		d.blank()
		d.explainRows(rows)
	}

	d.section(tr("Settings"))
	if cfg := pairs(root, "configuration"); len(cfg) == 0 {
		d.para(tr("The specification has no settings; each value the system reads at run time is written under configuration."))
	} else {
		d.para(tr("Every setting the system reads at run time. A secret's value is never written here; its description says where the value comes from."))
		d.line(tr("| Setting | Type | Secret | Description |"))
		d.line("|---|---|---|---|")
		for _, c := range cfg {
			secret := ""
			if str(c.Value, "secret") == "true" {
				secret = tr("yes")
			}
			d.line("| %s | %s | %s | %s |", markInternal(c.Key.Value, c.Value), cell(typeText(get(c.Value, "schema"))), secret, cell(str(c.Value, "description")))
		}
		d.blank()
		d.explainRows(rowsOf(cfg))
	}

	for _, p := range []struct{ key, title, none string }{
		{"backup", tr("Backing up"), tr("The specification has no backup; how the data of the live system is saved is written under backup.")},
		{"restore", "Restoring", tr("The specification has no restore; how saved data is brought back is written under restore.")},
		{"rollback", tr("Rolling back"), tr("The specification has no rollback; how a release is taken back is written under rollback.")},
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
	d.sourcesIndex(tr("Sources"))
	return d.String()
}

// runsText says in words how a process runs.
func runsText(p *yaml.Node) string {
	switch str(p, "runs") {
	case "always":
		return tr("always, from the host's start")
	case "scheduled":
		switch {
		case str(p, "schedule") != "":
			return tr("on the schedule `") + str(p, "schedule") + tr("` (UTC)")
		case str(p, "every") != "":
			return tr("every ") + str(p, "every")
		}
		return tr("when a job it runs is due")
	case "onDemand":
		return tr("when started by a person or another program")
	}
	return ""
}

// restartText says when the service manager starts a process again and
// when it gives up.
func restartText(r *yaml.Node) string {
	switch str(r, "when") {
	case "":
		return tr("not said")
	case "never":
		return tr("never")
	}
	text := tr("whenever it ends")
	if str(r, "when") == "onFailure" {
		text = tr("when it fails")
	}
	if delay := str(r, "delay"); delay != "" {
		text += tr(", after ") + delay
	}
	if l := get(r, "limit"); l != nil {
		text += fmt.Sprintf(tr("; given up after %s starts within %s"), str(l, "times"), str(l, "within"))
	}
	return text
}

// triggerText says in words what starts a job.
func triggerText(t *yaml.Node) string {
	switch {
	case str(t, "schedule") != "":
		return tr("on the schedule `") + str(t, "schedule") + tr("` (UTC)")
	case str(t, "every") != "":
		return tr("every ") + str(t, "every")
	case str(t, "consumes") != "":
		return tr("for each ") + str(t, "consumes")
	}
	return tr("when started by hand")
}

// retriesText says how often a job tries a failed item again and what
// becomes of it after the last try, with the role that handles a kept item.
func retriesText(job *yaml.Node) string {
	r := get(job, "retries")
	if r == nil {
		return tr("not tried again")
	}
	times := tr("times")
	if str(r, "limit") == "1" {
		times = tr("time")
	}
	text := fmt.Sprintf(tr("tried again up to %s %s, then "), str(r, "limit"), times)
	if str(r, "then") == "deadLetter" {
		if role := str(get(job, "deadLetter"), "role"); role != "" {
			return text + fmt.Sprintf(tr("kept apart for the %s role"), role)
		}
		return text + tr("kept apart for a person")
	}
	return text + tr("dropped")
}
