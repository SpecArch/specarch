package extract

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Systemd reads systemd unit files, each a .service or a .timer, into the
// processes they start (ADR-211): a .service is a process and a .timer the
// time a scheduled one is started. Only what a unit file states is
// written, each citing its line: the description, how it runs, the
// restart policy and its limit, the stop signal and its timeout, a reload
// that sends SIGHUP, the watchdog, and the settings Environment= names
// (never their values). What the unit leaves to systemd's defaults, which
// a host may change, is asked; what the meta-model holds in another way,
// or not at all, is a line and a could question.
func Systemd(paths []string, out, key string) (*Result, error) {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if info.IsDir() || (!strings.HasSuffix(p, ".service") && !strings.HasSuffix(p, ".timer")) {
			return nil, refuse("%s is not a unit file ending in .service or .timer; extract systemd reads service and timer units", p)
		}
	}
	r, err := Open(paths)
	if err != nil {
		return nil, err
	}
	sr := &systemdReader{key: key, res: &Result{Tree: newTree()}, processes: mapping(), settings: mapping()}
	commitLine(sr.res, r)
	units := map[string]*unitFile{}
	var services, timers []*unitFile
	for _, file := range r.Paths {
		data, err := os.ReadFile(filepath.Join(r.Repository.Root, filepath.FromSlash(file)))
		if err != nil {
			return nil, err
		}
		u := parseUnit(file, data)
		units[path.Base(file)] = u
		if strings.HasSuffix(file, ".timer") {
			timers = append(timers, u)
		} else {
			services = append(services, u)
		}
	}
	sr.res.say("counted %s and %s in %s", plural(len(services), "service unit"), plural(len(timers), "timer unit"), strings.Join(r.Paths, ", "))
	timerOf := map[string]*unitFile{}
	for _, t := range timers {
		target := strings.TrimSuffix(path.Base(t.file), ".timer") + ".service"
		if v := t.last("Timer", "Unit"); v != nil {
			target = v.value
		}
		if units[target] == nil {
			sr.notHeld(t.file, 1, "", "timer %s starts %s, which is not among the units read; read the two together", path.Base(t.file), target)
			continue
		}
		timerOf[target] = t
	}
	for _, u := range services {
		sr.service(u, timerOf[path.Base(u.file)])
	}
	var clauses []*yaml.Node
	for _, file := range r.Paths {
		clauses = append(clauses, flow(mapping("clause", file, "title", "The systemd unit "+path.Base(file))))
	}
	src, err := codeSource(r, out, clauses)
	if err != nil {
		return nil, err
	}
	sr.finish(src, r)
	return sr.res, nil
}

// unitFile is one unit file as systemd reads it: its assignments by section,
// each with its line.
type unitFile struct {
	file    string
	entries []unitEntry
}

type unitEntry struct {
	section, key, value string
	line                int
}

// parseUnit reads a unit file as systemd.syntax(7) writes it: sections in
// brackets, Key=Value assignments, comments starting with # or ;, and a
// line ending in a backslash continued on the next.
func parseUnit(file string, data []byte) *unitFile {
	u := &unitFile{file: file}
	section := ""
	sc := bufio.NewScanner(bytes.NewReader(data))
	n, start := 0, 0
	var pending strings.Builder
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if pending.Len() == 0 {
			start = n
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
				continue
			}
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				section = line[1 : len(line)-1]
				continue
			}
		}
		if strings.HasSuffix(line, "\\") {
			pending.WriteString(strings.TrimSuffix(line, "\\") + " ")
			continue
		}
		pending.WriteString(line)
		text := pending.String()
		pending.Reset()
		k, v, ok := strings.Cut(text, "=")
		if !ok {
			continue
		}
		u.entries = append(u.entries, unitEntry{section: section, key: strings.TrimSpace(k), value: strings.TrimSpace(v), line: start})
	}
	return u
}

// last is the assignment of a key that counts, the last one in its section.
func (u *unitFile) last(section, key string) *unitEntry {
	var found *unitEntry
	for i := range u.entries {
		if e := &u.entries[i]; e.section == section && e.key == key {
			found = e
		}
	}
	return found
}

type systemdReader struct {
	key                                  string
	res                                  *Result
	processes, settings                  *yaml.Node
	designQuestions, deploymentQuestions []questionAt
	held                                 []heldLine
	nextID                               int
	settingCount                         int
}

// questionAt is a question the reader asks, written into the stage of
// what it blocks.
type questionAt struct {
	text, priority, names, why string
	blocks                     []string
	cites                      []*yaml.Node
	deployment                 bool
}

// The keys the reader takes as the process says them; every other key is
// a line and a could question.
var systemdRead = map[string]bool{
	"Unit/Description": true, "Unit/StartLimitBurst": true, "Unit/StartLimitIntervalSec": true,
	"Service/Type": true, "Service/Restart": true, "Service/RestartSec": true, "Service/RestartPreventExitStatus": true,
	"Service/SuccessExitStatus": true, "Service/KillSignal": true, "Service/TimeoutStopSec": true, "Service/ExecReload": true,
	"Service/WatchdogSec": true, "Service/Environment": true, "Service/EnvironmentFile": true,
	"Service/StartLimitBurst": true, "Service/StartLimitIntervalSec": true,
}

// timerRead are the keys of a timer the reader takes.
var timerRead = map[string]bool{"Timer/OnCalendar": true, "Timer/OnUnitActiveSec": true, "Timer/Unit": true}

// systemdImplementation are the keys that say how a stack runs the
// process on one host: an implementation file's, not the design's.
var systemdImplementation = map[string]bool{
	"ExecStart": true, "ExecStartPre": true, "ExecStartPost": true, "ExecStop": true, "User": true, "Group": true,
	"WorkingDirectory": true, "Documentation": true, "After": true, "Before": true, "Wants": true, "Requires": true,
	"WantedBy": true, "RequiredBy": true,
}

var reloadHUP = regexp.MustCompile(`\bkill\s+(-HUP|-SIGHUP|-s\s+(SIG)?HUP|--signal[= ](SIG)?HUP)\s+\$MAINPID\b`)

// service reads one .service unit, and the .timer that starts it, into a
// process.
func (sr *systemdReader) service(u *unitFile, timer *unitFile) {
	name := unitName(strings.TrimSuffix(path.Base(u.file), ".service"))
	at := "#/processes/" + name
	p := mapping()
	var cites []*yaml.Node
	cite := func(f *unitFile, e *unitEntry, says string) {
		cites = append(cites, citation(sr.key, f.file, fmt.Sprintf("Line %d: %s=%s; %s", e.line, e.key, e.value, says)))
	}
	if e := u.last("Unit", "Description"); e != nil && e.value != "" {
		set(p, "description", e.value)
		cite(u, e, "the description")
	} else {
		sr.ask(questionAt{text: fmt.Sprintf("What does process %s do, in a sentence a reviewer can check? Its unit writes no Description=.", name),
			priority: "must", blocks: []string{at + "/description"}, why: "The unit file does not describe the process."})
	}
	typ := "simple"
	if e := u.last("Service", "Type"); e != nil {
		typ = e.value
	}
	runs := "always"
	switch {
	case timer != nil:
		runs = "scheduled"
	case typ == "oneshot":
		runs = "onDemand"
	}
	set(p, "runs", runs)
	switch runs {
	case "always":
		if e := u.last("Service", "Type"); e != nil {
			cite(u, e, "a service that is not oneshot runs until it is stopped, so it runs always")
		}
	case "onDemand":
		cite(u, u.last("Service", "Type"), "a oneshot service with no timer exits after its run and is started when asked, so it runs on demand")
	case "scheduled":
		sr.timer(timer, name, p, func(e *unitEntry, says string) { cite(timer, e, says) })
	}
	sr.environment(u, name, p, func(c *yaml.Node) { cites = append(cites, c) })
	sr.signals(u, name, p, cite)
	sr.restart(u, name, runs, p, cite)
	if e := u.last("Service", "WatchdogSec"); e != nil {
		if d, ok := sr.span(u, e, at); ok {
			set(p, "health", mapping("description", "it tells the service manager it is alive (sd_notify WATCHDOG=1) at least once each "+d+", and the service manager takes silence as a failure", "every", d))
			cite(u, e, "the watchdog")
		}
	}
	var codes []string
	var codeLines []*yaml.Node
	for _, k := range []string{"RestartPreventExitStatus", "SuccessExitStatus"} {
		e := u.last("Service", k)
		if e == nil {
			continue
		}
		for _, f := range strings.Fields(e.value) {
			if _, err := strconv.Atoi(f); err != nil {
				sr.notHeld(u.file, e.line, "", "%s= names the signal %s, and exitCodes holds exit statuses only", k, f)
				continue
			}
			codes = append(codes, f+" ("+k+"=)")
		}
		codeLines = append(codeLines, citation(sr.key, u.file, fmt.Sprintf("Line %d: %s=%s", e.line, k, e.value)))
	}
	if len(codes) > 0 {
		sr.ask(questionAt{text: fmt.Sprintf("What does each exit status of process %s mean, 0 among them, so they are written under exitCodes? Its unit names %s; a status RestartPreventExitStatus= names goes under restart.exceptExitCodes too.", name, strings.Join(codes, ", ")),
			priority: "must", blocks: []string{at + "/exitCodes"}, why: "The unit names the statuses and not what each means; exceptExitCodes names only statuses exitCodes declares.", cites: codeLines})
	}
	for _, e := range u.entries {
		if systemdRead[e.section+"/"+e.key] {
			continue
		}
		if systemdImplementation[e.key] {
			sr.notHeld(u.file, e.line, at, "[%s] %s= says how this host runs the process, which an implementation file's deployment holds, not the design", e.section, e.key)
			continue
		}
		sr.notHeld(u.file, e.line, at, "[%s] %s= has no key in a process", e.section, e.key)
	}
	set(p, "origin", "stated")
	set(p, "cites", cites)
	set(sr.processes, name, p)
}

// timer reads the .timer that starts a scheduled process.
func (sr *systemdReader) timer(t *unitFile, name string, p *yaml.Node, cite func(*unitEntry, string)) {
	at := "#/processes/" + name
	if e := t.last("Timer", "OnUnitActiveSec"); e != nil {
		if d, ok := sr.span(t, e, at); ok {
			set(p, "every", d)
			cite(e, "the time from one start to the next")
		}
	}
	if e := t.last("Timer", "OnCalendar"); e != nil {
		sr.ask(questionAt{text: fmt.Sprintf("When is process %s started, as a cron expression of five fields in UTC? Its timer says OnCalendar=%s, a systemd calendar expression read in the host's time zone, which is not cron.", name, e.value),
			priority: "must", blocks: []string{at + "/schedule"},
			why:   "A calendar expression and cron differ (seconds, ranges of days, the time zone), so a translation by the reader would be a guess.",
			cites: []*yaml.Node{citation(sr.key, t.file, fmt.Sprintf("Line %d: OnCalendar=%s", e.line, e.value))}})
	}
	if t.last("Timer", "OnCalendar") == nil && t.last("Timer", "OnUnitActiveSec") == nil {
		sr.ask(questionAt{text: fmt.Sprintf("When is process %s started? Its timer writes neither OnCalendar= nor OnUnitActiveSec=.", name),
			priority: "must", blocks: []string{at + "/schedule"}, why: "The timer does not say when it fires in a form the reader takes."})
	}
	for _, e := range t.entries {
		switch {
		case timerRead[e.section+"/"+e.key]:
		case e.section == "Unit" && e.key == "Description":
			sr.notHeld(t.file, e.line, at, "[Unit] Description= describes the timer, and the process takes its description from its service")
		case systemdImplementation[e.key]:
			sr.notHeld(t.file, e.line, at, "[%s] %s= says how this host runs the process, which an implementation file's deployment holds, not the design", e.section, e.key)
		default:
			sr.notHeld(t.file, e.line, at, "[%s] %s= has no key in a process", e.section, e.key)
		}
	}
}

// environment writes a setting for each variable Environment= names,
// never its value, and asks what an EnvironmentFile= holds.
func (sr *systemdReader) environment(u *unitFile, name string, p *yaml.Node, addCite func(*yaml.Node)) {
	var names []string
	for i := range u.entries {
		e := &u.entries[i]
		if e.section != "Service" {
			continue
		}
		switch e.key {
		case "Environment":
			var vars []string
			for _, a := range splitQuoted(e.value) {
				v, _, _ := strings.Cut(a, "=")
				s := unitName(v)
				if s == "" {
					continue
				}
				vars = append(vars, v)
				if source0Has(sr.settings, s) {
					names = appendNew(names, s)
					continue
				}
				names = append(names, s)
				sr.settingCount++
				set(sr.settings, s, mapping("origin", "stated", "cites", []*yaml.Node{citation(sr.key, u.file, fmt.Sprintf("Line %d: Environment= sets %s; its value is not copied, since it may be a secret", e.line, v))}))
				sr.ask(questionAt{text: fmt.Sprintf("What is setting %s (the variable %s): what it controls, whether it is a secret, and its type?", s, v),
					priority: "must", blocks: []string{"#/configuration/" + s + "/description", "#/configuration/" + s + "/secret", "#/configuration/" + s + "/schema"},
					why: "The unit sets the variable and says nothing of what it is; its value is left out, since it may be a secret.", deployment: true})
			}
			// The values are never copied: a variable may hold a secret.
			cites := citation(sr.key, u.file, fmt.Sprintf("Line %d: Environment= sets %s; the settings it reads, their values not copied", e.line, strings.Join(vars, ", ")))
			addCite(cites)
		case "EnvironmentFile":
			sr.ask(questionAt{text: fmt.Sprintf("Which settings does process %s read from %s? The unit names the file and not what it holds.", name, e.value),
				priority: "should", blocks: []string{"#/processes/" + name + "/configuration"},
				why:   "An environment file is read on the host, and its values may be secrets, so the reader does not read it.",
				cites: []*yaml.Node{citation(sr.key, u.file, fmt.Sprintf("Line %d: EnvironmentFile=%s", e.line, e.value))}})
		}
	}
	if len(names) > 0 {
		set(p, "configuration", names)
	}
}

// signals writes the stop signal with its timeout and SIGHUP when the
// reload sends it.
func (sr *systemdReader) signals(u *unitFile, name string, p *yaml.Node, cite func(*unitFile, *unitEntry, string)) {
	at := "#/processes/" + name
	sigs := mapping()
	stop := "SIGTERM"
	kill := u.last("Service", "KillSignal")
	if kill != nil {
		stop = kill.value
		if !strings.HasPrefix(stop, "SIG") {
			stop = "SIG" + stop
		}
	}
	timeout := u.last("Service", "TimeoutStopSec")
	if kill != nil || timeout != nil {
		if !processSignal(stop) {
			sr.notHeld(u.file, kill.line, at, "KillSignal=%s is not one of the signals a process honours (SIGTERM, SIGINT, SIGHUP, SIGQUIT, SIGUSR1, SIGUSR2)", kill.value)
		} else {
			s := mapping("description", "stops: the service manager sends it to stop the process, and kills the process when it has not ended within the time")
			if kill != nil {
				cite(u, kill, "the stop signal")
			}
			if timeout != nil {
				if d, ok := sr.span(u, timeout, at); ok {
					set(s, "within", d)
					cite(u, timeout, "the time the service manager waits before it kills the process")
				}
			}
			set(sigs, stop, s)
		}
	}
	if e := u.last("Service", "ExecReload"); e != nil {
		if reloadHUP.MatchString(e.value) && stop != "SIGHUP" {
			set(sigs, "SIGHUP", mapping("description", "reloads: the service manager sends it when the process is reloaded"))
			cite(u, e, "a reload sends SIGHUP")
		} else {
			sr.notHeld(u.file, e.line, at, "ExecReload= runs a command, not a signal the reader recognises; the command is not copied, since it may carry a secret")
		}
	}
	if len(sigs.Content) > 0 {
		set(p, "signals", sigs)
	}
}

func processSignal(s string) bool {
	switch s {
	case "SIGTERM", "SIGINT", "SIGHUP", "SIGQUIT", "SIGUSR1", "SIGUSR2":
		return true
	}
	return false
}

// restart writes when the service manager starts the process again, the
// delay, and the limit after which it gives up, and asks for the limit
// the unit leaves to systemd's defaults.
func (sr *systemdReader) restart(u *unitFile, name, runs string, p *yaml.Node, cite func(*unitFile, *unitEntry, string)) {
	at := "#/processes/" + name
	e := u.last("Service", "Restart")
	if e == nil {
		if runs == "always" {
			sr.ask(questionAt{text: fmt.Sprintf("Is process %s ever started again after it ends? Its unit writes no Restart=, so systemd's default, no, holds: it stays ended.", name),
				priority: "should", blocks: []string{at + "/restart"}, why: "A daemon that is not restarted stays down after a crash until a person starts it; the unit may have left it out by mistake."})
		}
		return
	}
	when := map[string]string{"no": "never", "always": "always", "on-failure": "onFailure"}[e.value]
	if when == "" {
		sr.notHeldAsked(u.file, e.line, at, at+"/restart", "Restart=%s is none of no, always and on-failure, which restart.when holds", e.value)
		sr.ask(questionAt{text: fmt.Sprintf("When is process %s started again: always, on failure or never? Its unit says Restart=%s.", name, e.value),
			priority: "must", blocks: []string{at + "/restart"}, why: "The unit's policy has no word in restart.when."})
		return
	}
	r := mapping("when", when)
	cite(u, e, "when it is started again")
	if when != "never" {
		if d := u.last("Service", "RestartSec"); d != nil {
			if v, ok := sr.span(u, d, at); ok {
				set(r, "delay", v)
				cite(u, d, "the delay before it is started again")
			}
		}
		burst, interval := u.last("Unit", "StartLimitBurst"), u.last("Unit", "StartLimitIntervalSec")
		if burst == nil {
			burst = u.last("Service", "StartLimitBurst")
		}
		if interval == nil {
			interval = u.last("Service", "StartLimitIntervalSec")
		}
		asked := false
		if burst != nil && interval != nil {
			times, err := strconv.Atoi(burst.value)
			within, ok := "", false
			if interval.value == "0" || interval.value == "infinity" {
				sr.notHeldAsked(u.file, interval.line, at, at+"/restart/limit", "StartLimitIntervalSec=%s turns the limit off, and a restarted process is given up after a limit", interval.value)
			} else {
				within, ok = sr.span(u, interval, at)
			}
			if err == nil && times > 0 && ok {
				set(r, "limit", mapping("times", times, "within", within))
				cite(u, burst, "the starts after which it is given up")
				cite(u, interval, "within this time")
			} else {
				asked = true
			}
		} else {
			asked = true
		}
		if asked {
			sr.ask(questionAt{text: fmt.Sprintf("After how many starts within what time does the service manager give process %s up? Its unit does not write both StartLimitBurst= and StartLimitIntervalSec= as a limit, so the host's defaults in system.conf decide (DefaultStartLimitBurst= and DefaultStartLimitIntervalSec=, 5 within 10 seconds unless the host changes them).", name),
				priority: "must", blocks: []string{at + "/restart/limit"}, why: "The defaults are the host's, not the unit's, and differ from one service manager to the next."})
		}
	}
	set(p, "restart", r)
}

// span reads a systemd time span (systemd.time(7)) as an ISO 8601
// duration in days, hours, minutes and seconds; a span in weeks, months,
// years or below a second, or of zero, is not held.
func (sr *systemdReader) span(u *unitFile, e *unitEntry, at string) (string, bool) {
	total, ok := timeSpan(e.value)
	if !ok || total == 0 {
		sr.notHeld(u.file, e.line, at, "%s=%s is not a whole number of seconds above zero in days, hours, minutes and seconds, which a duration holds", e.key, e.value)
		return "", false
	}
	return isoDuration(total), true
}

var spanPart = regexp.MustCompile(`([0-9]+)\s*([a-z]*)`)

// timeSpan is a systemd time span in seconds: a number alone is seconds.
func timeSpan(s string) (int, bool) {
	units := map[string]int{"": 1, "s": 1, "sec": 1, "second": 1, "seconds": 1, "m": 60, "min": 60, "minute": 60, "minutes": 60,
		"h": 3600, "hr": 3600, "hour": 3600, "hours": 3600, "d": 86400, "day": 86400, "days": 86400}
	lower := strings.ToLower(s)
	parts := spanPart.FindAllStringSubmatch(lower, -1)
	if len(parts) == 0 || strings.TrimSpace(spanPart.ReplaceAllString(lower, "")) != "" {
		return 0, false
	}
	total := 0
	for _, m := range parts {
		n, err := strconv.Atoi(m[1])
		f, ok := units[m[2]]
		if err != nil || !ok {
			return 0, false
		}
		total += n * f
	}
	return total, true
}

// isoDuration writes seconds as ISO 8601 does: P1DT2H, PT30S.
func isoDuration(total int) string {
	d, rest := total/86400, total%86400
	h, m, s := rest/3600, rest%3600/60, rest%60
	out := "P"
	if d > 0 {
		out += strconv.Itoa(d) + "D"
	}
	if rest > 0 {
		out += "T"
		if h > 0 {
			out += strconv.Itoa(h) + "H"
		}
		if m > 0 {
			out += strconv.Itoa(m) + "M"
		}
		if s > 0 {
			out += strconv.Itoa(s) + "S"
		}
	}
	return out
}

var nameWord = regexp.MustCompile(`[A-Za-z0-9]+`)

// unitName is a unit's or a variable's name in camelCase: release-watcher
// is releaseWatcher, REGISTRY_TOKEN is registryToken.
func unitName(s string) string {
	var b strings.Builder
	word := 0
	for _, w := range nameWord.FindAllString(s, -1) {
		w = strings.ToLower(w)
		if word > 0 {
			w = strings.ToUpper(w[:1]) + w[1:]
		}
		b.WriteString(w)
		word++
	}
	out := b.String()
	if out != "" && (out[0] < 'a' || out[0] > 'z') {
		out = "unit" + strings.ToUpper(out[:1]) + out[1:]
	}
	return out
}

// splitQuoted splits Environment='A=1' "B=two words" C=3 into its
// assignments.
func splitQuoted(s string) []string {
	var out []string
	var b strings.Builder
	quote := rune(0)
	for _, r := range s {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote == 0 && (r == '"' || r == '\''):
			quote = r
		case quote == 0 && r == ' ':
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

// ask keeps a question for the stage of what it blocks.
func (sr *systemdReader) ask(q questionAt) {
	if q.deployment {
		sr.deploymentQuestions = append(sr.deploymentQuestions, q)
	} else {
		sr.designQuestions = append(sr.designQuestions, q)
	}
}

// notHeld records what the meta-model does not hold, at the file's line,
// blocking the process it is about.
func (sr *systemdReader) notHeld(file string, line int, block, format string, args ...any) {
	sr.notHeldAsked(file, line, block, "", format, args...)
}

// notHeldAsked is notHeld where a must question the reader asks, blocking
// asked, already asks for it: the line names that question.
func (sr *systemdReader) notHeldAsked(file string, line int, block, asked, format string, args ...any) {
	if block == "" {
		block = "processes"
	}
	clause := fmt.Sprintf("%s:%d", file, line)
	sr.held = append(sr.held, heldLine{line, notHeld{text: clause + ": " + fmt.Sprintf(format, args...), clause: clause, blocks: []string{block}, asked: asked}})
}

// finish writes the tree: the processes, the settings, the questions in
// the stage of what each blocks, the lines for what was not held, and the
// root file.
func (sr *systemdReader) finish(src *yaml.Node, r *Read) {
	res := sr.res
	design, deployment := mapping(), mapping()
	set(design, "processes", sr.processes)
	stages := []string{"requirements", "design"}
	designQ, deploymentQ := mapping(), mapping()
	write := func(qs []questionAt, into *yaml.Node) {
		for _, q := range qs {
			sr.nextID++
			n := mapping("question", q.text, "kind", "decision", "priority", q.priority, "blocks", q.blocks, "decidedBy", owner)
			if q.names != "" {
				set(n, "names", q.names)
			}
			set(n, "why", q.why)
			if len(q.cites) > 0 {
				set(n, "cites", q.cites)
			}
			set(into, fmt.Sprintf("Q-%d", sr.nextID), n)
		}
	}
	write(sr.designQuestions, designQ)
	write(sr.deploymentQuestions, deploymentQ)
	asked := sr.nextID
	res.Tree.put("design/processes.yaml", design)
	if sr.settingCount > 0 {
		set(deployment, "configuration", sr.settings)
		res.Tree.put("deployment/configuration.yaml", deployment)
		res.Tree.put("deployment/questions.yaml", mapping("questions", deploymentQ))
		stages = append(stages, "deployment")
	}
	res.Tree.put("requirements/stakeholders.yaml", mapping("stakeholders", ownerStakeholder()))
	fileOf := func(h heldLine) string { return h.clause[:strings.LastIndex(h.clause, ":")] }
	sort.SliceStable(sr.held, func(i, j int) bool {
		if a, b := fileOf(sr.held[i]), fileOf(sr.held[j]); a != b {
			return a < b
		}
		return sr.held[i].line < sr.held[j].line
	})
	var held []notHeld
	for _, h := range sr.held {
		held = append(held, h.notHeld)
	}
	res.say("wrote %s, %s and %s: one process per service unit, its time from its timer, one setting per variable Environment= sets, and one question per thing the units leave to the host or do not say",
		plural(len(sr.processes.Content)/2, "process"), plural(sr.settingCount, "setting"), plural(asked+couldCount(oneQuestions(designQ), held), "question"))
	askNotHeld(res, oneQuestions(designQ), &sr.nextID, sr.key, held)
	res.Tree.put("design/questions.yaml", mapping("questions", designQ))
	description := fmt.Sprintf("The processes the systemd units %s define at commit %s: how each runs, is restarted and given up, is stopped and reloaded, and the settings it reads. Every process cites its unit; what a unit leaves to the host's defaults or does not say is a question.\n", strings.Join(r.Paths, ", "), r.Commit)
	res.Tree.put("specarch.yaml", rootFile("Processes of "+strings.Join(r.Paths, ", "), description, stages, mapping(sr.key, src)))
}
