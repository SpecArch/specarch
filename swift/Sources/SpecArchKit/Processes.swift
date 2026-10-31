import Foundation

// The long-running processes a service manager starts and watches
// (ADR-211): a daemon kept running, a loop started on a schedule, a
// program started on demand.

extension Checker {
    /// Checks that a process names jobs, settings and monitors that exist,
    /// that a job is run by one process, that when it runs is said in one
    /// place (a job's trigger, or a scheduled process's own time when it
    /// runs no job), that a restarted process has a limit, and that what it
    /// is never restarted on is one of its exit statuses.
    func checkProcesses(_ d: Design) {
        var runBy: [String: String] = [:]
        var jobsByName: [String: YNode] = [:]
        for j in pairs(d.root.child("jobs")) { jobsByName[j.key.value] = j.value }
        // A key an open must question asks for is not an error while it
        // waits for the answer, as a reader writes it (docs/principles.md).
        let held = blockedPointers(d.root)
        func waits(_ parent: String, _ key: String) -> Bool { held.under(parent) || (held.keys[parent] ?? []).contains(key) }
        for p in pairs(d.root.child("processes")) {
            let name = p.key.value
            func at(_ tokens: String...) -> String { pointer(["processes", name] + tokens) }
            let runs = str(p.value.child("runs"))
            let jobs = items(p.value.child("jobs"))
            let schedule = p.value.child("schedule"), every = p.value.child("every")
            if runs == "scheduled" && !jobs.isEmpty {
                for k in ["schedule", "every"] where p.value.child(k) != nil {
                    add(p.value.key(k), at(k), .process, "process \(name) runs jobs, so it is started at each job's schedule or every; when it runs is said once, so remove \(k)")
                }
            } else if runs == "scheduled" && schedule != nil && every != nil {
                add(p.value.key("every"), at("every"), .process, "process \(name) writes both schedule and every; when it is started is said once, so keep one")
            } else if runs == "scheduled" && schedule == nil && every == nil && !waits(at(), "schedule") && !waits(at(), "every") {
                add(p.value.key("runs"), at("runs"), .process, "process \(name) runs scheduled and runs no job, but says neither schedule nor every; write when it is started")
            } else if runs != "scheduled" && !runs.isEmpty {
                for k in ["schedule", "every"] where p.value.child(k) != nil {
                    add(p.value.key(k), at(k), .process, "process \(name) runs \(runs), so it is not started on a schedule; remove \(k), or write runs: scheduled")
                }
            }
            for (i, j) in jobs.enumerated() {
                guard let job = jobsByName[j.value] else {
                    add(j, at("jobs", "\(i)"), .process, "\(j.value) is not a job of the specification\(suggest(j.value, jobsByName))")
                    continue
                }
                if let other = runBy[j.value] {
                    add(j, at("jobs", "\(i)"), .process, "job \(j.value) is run by process \(other) already; a job is run by one process")
                    continue
                }
                runBy[j.value] = name
                if runs == "scheduled" && job.child("trigger")?.child("consumes") != nil {
                    add(j, at("jobs", "\(i)"), .process, "job \(j.value) consumes messages, and a consumer waits for them, but process \(name) is scheduled and exits after its run; run the job in a process that runs always")
                }
            }
            for (i, s) in items(p.value.child("configuration")).enumerated() where d.settings[s.value] == nil {
                add(s, at("configuration", "\(i)"), .process, "\(s.value) is not a setting of the specification; declare it under configuration\(suggest(s.value, d.settings))")
            }
            for (i, m) in items(p.value.child("monitors")).enumerated() where d.monitors[m.value] == nil {
                add(m, at("monitors", "\(i)"), .process, "\(m.value) is not a monitor of the specification; declare it under monitors\(suggest(m.value, d.monitors))")
            }
            let r = p.value.child("restart")
            let when = str(r?.child("when"))
            if r == nil || when.isEmpty {
            } else if when == "never" {
                for k in ["delay", "limit", "exceptExitCodes"] where r?.child(k) != nil {
                    add(r?.key(k), at("restart", k), .process, "process \(name) is never restarted, so \(k) says nothing; remove it, or write when: onFailure or always")
                }
            } else if r?.child("limit") == nil && waits(at("restart"), "limit") {
            } else if r?.child("limit") == nil {
                add(r?.key("when"), at("restart", "when"), .process, "process \(name) is restarted \(whenText(when)) with no limit; write limit with times and within, after which the service manager gives up, since a manager's own default differs from one manager to the next")
            } else if items(p.value.child("monitors")).isEmpty {
                warn(r?.key("limit"), at("restart", "limit"), .process, "process \(name) is given up after its limit, and no monitor watches it, so nobody is told it has stopped; name the monitor under monitors")
            }
            let codes = p.value.child("exitCodes")
            for (i, e) in items(r?.child("exceptExitCodes")).enumerated() where when != "never" && codes?.child(e.value) == nil {
                add(e, at("restart", "exceptExitCodes", "\(i)"), .process, "exit status \(e.value) is not one of the exit statuses of process \(name); declare it under exitCodes")
            }
        }
    }
}

/// When a process is restarted, in words.
private func whenText(_ when: String) -> String { when == "onFailure" ? "on failure" : when }

/// Which monitors alert: "the monitor a alerts", "the monitors a and b alert".
private func monitorText(_ names: [String]) -> String {
    if names.count == 1 { return "the monitor " + names[0] + " alerts" }
    return "the monitors " + names.dropLast().joined(separator: ", ") + " and " + names[names.count - 1] + " alert"
}

extension Design {
    /// A process's tests: it starts and does its work, each signal it
    /// honours, each setting it reads missing or invalid, each exit status,
    /// and, when it is restarted, a crash and the crash loop it is given up
    /// after. It has no permission, so no caller case.
    func processSubject(_ p: Pair) -> Subject {
        let name = p.key.value
        let v = p.value
        let s = Subject(label: "process " + name, node: p.key, path: pointer("processes", name), yamlKey: "process: " + name, name: kebab(name))
        var start = "the process " + name + " starts"
        if str(v.child("runs")) == "scheduled" {
            let sc = str(v.child("schedule")), e = str(v.child("every"))
            if !sc.isEmpty {
                start = "the process " + name + " is started on the schedule " + sc
            } else if !e.isEmpty {
                start = "the process " + name + " is started, every " + e
            } else if !items(v.child("jobs")).isEmpty {
                start = "the process " + name + " is started when a job it runs is due"
            }
        }
        let given = items(v.child("configuration")).isEmpty ? "..." : "its settings are valid"
        var then = "it does its work and exits 0"
        let ok = str(v.child("exitCodes")?.child("0"))
        if !ok.isEmpty { then += ": " + ok }
        if str(v.child("runs")) == "always" {
            then = "it keeps running until it is stopped"
            let h = str(v.child("health")?.child("description")).trimmingCharacters(in: .whitespacesAndNewlines)
            if !h.isEmpty { then = "it is healthy: " + h }
        }
        s.success = golden(given, start, then)
        let running = "the process " + name + " is running"
        for sg in pairs(v.child("signals")) {
            // Every stop and every deploy sends SIGTERM.
            let frequency = sg.key.value == "SIGTERM" ? frequent : occasional
            var does = str(sg.value.child("description")).trimmingCharacters(in: .whitespacesAndNewlines)
            let w = str(sg.value.child("within"))
            if !w.isEmpty { does += ", within " + w }
            s.cases.append(DerivedCase(name: "signal " + sg.key.value, scenario: "golden", given: running,
                                       when: "it receives " + sg.key.value, then: does, frequency: frequency))
        }
        for sn in items(v.child("configuration")) {
            guard let setting = settings[sn.value] else { continue }
            let secret = str(setting.child("secret")) == "true"
            s.red(secret ? "secret missing " + sn.value : "setting invalid " + sn.value, frequent,
                  secret ? "the secret " + sn.value + " is missing" : "the setting " + sn.value + " is missing or not valid",
                  start, "it exits with a status other than 0 before it does any work, naming " + sn.value)
        }
        let restart = v.child("restart")
        let when = str(restart?.child("when"))
        let except = Set(items(restart?.child("exceptExitCodes")).map { $0.value })
        for c in pairs(v.child("exitCodes")) where c.key.value != "0" {
            var then = "it exits " + c.key.value + ": " + str(c.value)
            if except.contains(c.key.value) && when != "never" { then += "; it is not started again" }
            s.red("exit " + c.key.value, frequent, "...", "the process " + name + " ends", then)
        }
        if when == "always" || when == "onFailure" {
            let ends = when == "always" ? "it ends" : "it ends with a status other than 0"
            var again = "the service manager starts it again"
            let delay = str(restart?.child("delay"))
            if !delay.isEmpty { again += " after " + delay }
            s.byNature("restarted after a crash", running, ends, again)
            if let limit = restart?.child("limit") {
                var then = "the service manager does not start it again"
                let mons = items(v.child("monitors")).map { $0.value }
                if !mons.isEmpty { then += ", and " + monitorText(mons) }
                s.byNature("crash loop given up", "the process " + name + " fails each time it starts",
                           "it has been started " + str(limit.child("times")) + " times within " + str(limit.child("within")), then)
            }
        }
        return s
    }
}
