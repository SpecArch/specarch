package validate

import (
	"fmt"
	"strings"

	"github.com/SpecArch/specarch/internal/source"
)

// The long-running processes a service manager starts and watches
// (ADR-211): a daemon kept running, a loop started on a schedule, a
// program started on demand.

// checkProcesses checks that a process names jobs, settings and monitors
// that exist, that a job is run by one process, that when it runs is said
// in one place (a job's trigger, or a scheduled process's own time when it
// runs no job), that a restarted process has a limit, and that what it is
// never restarted on is one of its exit statuses.
func (c *checker) checkProcesses(d *design) {
	runBy := map[string]string{}
	for _, p := range source.Pairs(source.Child(d.root, "processes")) {
		name := p.Key.Value
		at := func(tokens ...string) string {
			return source.Pointer(append([]string{"processes", name}, tokens...)...)
		}
		runs := source.Str(source.Child(p.Value, "runs"))
		jobs := source.Items(source.Child(p.Value, "jobs"))
		schedule, every := source.Child(p.Value, "schedule"), source.Child(p.Value, "every")
		switch {
		case runs == "scheduled" && len(jobs) > 0:
			for _, k := range []string{"schedule", "every"} {
				if source.Child(p.Value, k) != nil {
					c.add(source.Key(p.Value, k), at(k), RuleProcess, "process %s runs jobs, so it is started at each job's schedule or every; when it runs is said once, so remove %s", name, k)
				}
			}
		case runs == "scheduled" && schedule != nil && every != nil:
			c.add(source.Key(p.Value, "every"), at("every"), RuleProcess, "process %s writes both schedule and every; when it is started is said once, so keep one", name)
		case runs == "scheduled" && schedule == nil && every == nil:
			c.add(source.Key(p.Value, "runs"), at("runs"), RuleProcess, "process %s runs scheduled and runs no job, but says neither schedule nor every; write when it is started", name)
		case runs != "scheduled" && runs != "":
			for _, k := range []string{"schedule", "every"} {
				if source.Child(p.Value, k) != nil {
					c.add(source.Key(p.Value, k), at(k), RuleProcess, "process %s runs %s, so it is not started on a schedule; remove %s, or write runs: scheduled", name, runs, k)
				}
			}
		}
		jobsByName := topMap(d.root, "jobs")
		for i, j := range jobs {
			job := jobsByName[j.Value]
			if job == nil {
				c.add(j, at("jobs", fmt.Sprint(i)), RuleProcess, "%s is not a job of the specification%s", j.Value, suggest(j.Value, jobsByName))
				continue
			}
			if other, ok := runBy[j.Value]; ok {
				c.add(j, at("jobs", fmt.Sprint(i)), RuleProcess, "job %s is run by process %s already; a job is run by one process", j.Value, other)
				continue
			}
			runBy[j.Value] = name
			if runs == "scheduled" && source.Child(source.Child(job, "trigger"), "consumes") != nil {
				c.add(j, at("jobs", fmt.Sprint(i)), RuleProcess, "job %s consumes messages, and a consumer waits for them, but process %s is scheduled and exits after its run; run the job in a process that runs always", j.Value, name)
			}
		}
		for i, s := range source.Items(source.Child(p.Value, "configuration")) {
			if d.settings[s.Value] == nil {
				c.add(s, at("configuration", fmt.Sprint(i)), RuleProcess, "%s is not a setting of the specification; declare it under configuration%s", s.Value, suggest(s.Value, d.settings))
			}
		}
		for i, m := range source.Items(source.Child(p.Value, "monitors")) {
			if d.monitors[m.Value] == nil {
				c.add(m, at("monitors", fmt.Sprint(i)), RuleProcess, "%s is not a monitor of the specification; declare it under monitors%s", m.Value, suggest(m.Value, d.monitors))
			}
		}
		r := source.Child(p.Value, "restart")
		when := source.Str(source.Child(r, "when"))
		switch {
		case r == nil || when == "":
		case when == "never":
			for _, k := range []string{"delay", "limit", "exceptExitCodes"} {
				if source.Child(r, k) != nil {
					c.add(source.Key(r, k), at("restart", k), RuleProcess, "process %s is never restarted, so %s says nothing; remove it, or write when: onFailure or always", name, k)
				}
			}
		case source.Child(r, "limit") == nil:
			c.add(source.Key(r, "when"), at("restart", "when"), RuleProcess, "process %s is restarted %s with no limit; write limit with times and within, after which the service manager gives up, since a manager's own default differs from one manager to the next", name, whenText(when))
		case len(source.Items(source.Child(p.Value, "monitors"))) == 0:
			c.warn(source.Key(r, "limit"), at("restart", "limit"), RuleProcess, "process %s is given up after its limit, and no monitor watches it, so nobody is told it has stopped; name the monitor under monitors", name)
		}
		codes := source.Child(p.Value, "exitCodes")
		for i, e := range source.Items(source.Child(r, "exceptExitCodes")) {
			if when != "never" && source.Child(codes, e.Value) == nil {
				c.add(e, at("restart", "exceptExitCodes", fmt.Sprint(i)), RuleProcess, "exit status %s is not one of the exit statuses of process %s; declare it under exitCodes", e.Value, name)
			}
		}
	}
}

// whenText says in words when a process is restarted.
func whenText(when string) string {
	if when == "onFailure" {
		return "on failure"
	}
	return when
}

// processSubject is a process's tests: it starts and does its work, each
// signal it honours, each setting it reads missing or invalid, each exit
// status, and, when it is restarted, a crash and the crash loop it is
// given up after. It has no permission, so no caller case.
func (d *design) processSubject(p source.Pair) *subject {
	name := p.Key.Value
	v := p.Value
	s := &subject{kind: "process", label: "process " + name, node: p.Key, path: source.Pointer("processes", name),
		yamlKey: "process: " + name, name: kebab(name)}
	start := "the process " + name + " starts"
	if source.Str(source.Child(v, "runs")) == "scheduled" {
		if sc := source.Str(source.Child(v, "schedule")); sc != "" {
			start = "the process " + name + " is started on the schedule " + sc
		} else if e := source.Str(source.Child(v, "every")); e != "" {
			start = "the process " + name + " is started, every " + e
		} else if len(source.Items(source.Child(v, "jobs"))) > 0 {
			start = "the process " + name + " is started when a job it runs is due"
		}
	}
	given := "..."
	if len(source.Items(source.Child(v, "configuration"))) > 0 {
		given = "its settings are valid"
	}
	then := "it does its work and exits 0"
	if ok := source.Str(source.Child(source.Child(v, "exitCodes"), "0")); ok != "" {
		then += ": " + ok
	}
	if source.Str(source.Child(v, "runs")) == "always" {
		then = "it keeps running until it is stopped"
		if h := strings.TrimSpace(source.Str(source.Child(source.Child(v, "health"), "description"))); h != "" {
			then = "it is healthy: " + h
		}
	}
	s.success = golden(given, start, then)
	running := "the process " + name + " is running"
	for _, sg := range source.Pairs(source.Child(v, "signals")) {
		frequency := occasional
		if sg.Key.Value == "SIGTERM" {
			frequency = frequent // every stop and every deploy sends it
		}
		does := strings.TrimSpace(source.Str(source.Child(sg.Value, "description")))
		if w := source.Str(source.Child(sg.Value, "within")); w != "" {
			does += ", within " + w
		}
		s.cases = append(s.cases, derivedCase{name: "signal " + sg.Key.Value, scenario: "golden", given: running,
			when: "it receives " + sg.Key.Value, then: does, frequency: frequency})
	}
	for _, sn := range source.Items(source.Child(v, "configuration")) {
		setting := d.settings[sn.Value]
		if setting == nil {
			continue
		}
		caseName, what := "setting invalid "+sn.Value, "the setting "+sn.Value+" is missing or not valid"
		if source.Str(source.Child(setting, "secret")) == "true" {
			caseName, what = "secret missing "+sn.Value, "the secret "+sn.Value+" is missing"
		}
		s.red(caseName, frequent, what, start, "it exits with a status other than 0 before it does any work, naming "+sn.Value)
	}
	restart := source.Child(v, "restart")
	when := source.Str(source.Child(restart, "when"))
	except := map[string]bool{}
	for _, e := range source.Items(source.Child(restart, "exceptExitCodes")) {
		except[e.Value] = true
	}
	for _, c := range source.Pairs(source.Child(v, "exitCodes")) {
		if c.Key.Value == "0" {
			continue
		}
		then := "it exits " + c.Key.Value + ": " + source.Str(c.Value)
		if except[c.Key.Value] && when != "never" {
			then += "; it is not started again"
		}
		s.red("exit "+c.Key.Value, frequent, "...", "the process "+name+" ends", then)
	}
	if when == "always" || when == "onFailure" {
		ends := "it ends with a status other than 0"
		if when == "always" {
			ends = "it ends"
		}
		again := "the service manager starts it again"
		if delay := source.Str(source.Child(restart, "delay")); delay != "" {
			again += " after " + delay
		}
		s.byNature("restarted after a crash", running, ends, again)
		if limit := source.Child(restart, "limit"); limit != nil {
			then := "the service manager does not start it again"
			if mons := source.Items(source.Child(v, "monitors")); len(mons) > 0 {
				var names []string
				for _, m := range mons {
					names = append(names, m.Value)
				}
				then += ", and " + monitorText(names)
			}
			s.byNature("crash loop given up", "the process "+name+" fails each time it starts",
				"it has been started "+source.Str(source.Child(limit, "times"))+" times within "+source.Str(source.Child(limit, "within")), then)
		}
	}
	return s
}

// monitorText says which monitors alert: "the monitor a alerts", "the
// monitors a and b alert".
func monitorText(names []string) string {
	if len(names) == 1 {
		return "the monitor " + names[0] + " alerts"
	}
	return "the monitors " + strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1] + " alert"
}
