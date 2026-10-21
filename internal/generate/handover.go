package generate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// infrastructureKeys are the resources a deployment's infrastructure names,
// in the order a document lists them.
var infrastructureKeys = [][2]string{
	{"cpu", "Processor"}, {"memory", "Memory"}, {"storage", "Storage"},
	{"network", "Network"}, {"ports", "Ports"}, {"services", "Services"},
}

// customerRun lists the environments the customer runs, in document order.
func customerRun(envs []sourcePair) []string {
	var names []string
	for _, e := range envs {
		if str(e.Value, "runBy") == "customer" {
			names = append(names, e.Key.Value)
		}
	}
	return names
}

// environmentsTable lists the environments: with a column for who runs
// each once any environment says so, since every environment without runBy
// is run by the vendor (ADR-129).
func environmentsTable(d *doc, envs []sourcePair) {
	anyRunBy := false
	for _, e := range envs {
		if str(e.Value, "runBy") != "" {
			anyRunBy = true
		}
	}
	if anyRunBy {
		d.line("| Environment | Purpose | Run by | Promotes to |")
		d.line("|---|---|---|---|")
	} else {
		d.line("| Environment | Purpose | Promotes to |")
		d.line("|---|---|---|")
	}
	for _, e := range envs {
		if anyRunBy {
			by := str(e.Value, "runBy")
			if by == "" {
				by = "vendor"
			}
			d.line("| %s | %s | %s | %s |", e.Key.Value, cell(str(e.Value, "description")), by, cell(str(e.Value, "promotesTo")))
		} else {
			d.line("| %s | %s | %s |", e.Key.Value, cell(str(e.Value, "description")), cell(str(e.Value, "promotesTo")))
		}
	}
	d.blank()
	d.explainRows(rowsOf(envs))
}

// handoverText writes the handover under a heading the caller wrote: what
// it is, who receives it, how the source reaches them, and what stays whose
// responsibility.
func handoverText(d *doc, root, h *yaml.Node) {
	d.para(str(h, "description"))
	to := str(h, "to")
	who := to + "."
	if desc := str(get(get(root, "stakeholders"), to), "description"); desc != "" {
		who = to + ": " + strings.TrimSpace(desc)
	}
	d.para("Received by " + who)
	d.para("The source: " + strings.TrimSpace(str(h, "source")))
	resp := get(h, "responsibilities")
	for _, side := range [][2]string{{"customer", "The customer is responsible for:"}, {"vendor", "The vendor stays responsible for:"}} {
		if list := strs(resp, side[0]); len(list) > 0 {
			d.para(side[1])
			for _, s := range list {
				d.line("- %s", s)
			}
			d.blank()
		}
	}
	d.explain(h)
}

// taskCommand is the command of a task of the implementation file, by key,
// or the key alone when the file has no such task.
func taskCommand(impl *yaml.Node, key string) string {
	if run := str(get(get(impl, "tasks"), key), "run"); run != "" {
		return "`" + run + "`"
	}
	return key
}

// buildDetails writes how a deployment's artifact is built: the steps, the
// build command, the base images by digest, how a build is compared with
// the vendor's, and the software bill of materials, under headings of the
// given level when level is above 0.
func buildDetails(d *doc, impl, build *yaml.Node, level int) {
	if build == nil {
		return
	}
	d.para(str(build, "description"))
	d.para(fmt.Sprintf("Build command: %s (the task %s).", taskCommand(impl, str(build, "task")), str(build, "task")))
	if images := items(build, "baseImages"); len(images) > 0 {
		if level > 0 {
			d.heading(level, "Base images")
		}
		d.para("Every image the build starts from, pinned by its digest; a build from another digest is a different build.")
		d.line("| Image | Digest | Purpose |")
		d.line("|---|---|---|")
		for _, im := range images {
			d.line("| %s | %s | %s |", cell(str(im, "name")), cell(str(im, "digest")), cell(str(im, "purpose")))
		}
		d.blank()
	}
	if r := strings.TrimSpace(str(build, "reproducible")); r != "" {
		if level > 0 {
			d.heading(level, "Comparing a build with the vendor's")
		}
		d.para(r)
	}
	if sbom := str(build, "sbom"); sbom != "" {
		if level > 0 {
			d.heading(level, "Software bill of materials")
		}
		d.para(fmt.Sprintf("The software bill of materials of the build, and its scan for known vulnerabilities, are written by %s (the task %s); the customer runs it on their build and compares the result with the vendor's.", taskCommand(impl, sbom), sbom))
	}
}

// methodPara writes how a build is put on the hosts.
func methodPara(d *doc, method *yaml.Node) {
	if method == nil {
		return
	}
	line := "Installed with " + str(method, "tool")
	if f := str(method, "files"); f != "" {
		line += ", from " + f
	}
	line += "."
	if desc := strings.TrimSpace(str(method, "description")); desc != "" {
		line += " " + desc
	}
	d.para(line)
}

// infrastructureTable writes what the hosts of a deployment must provide.
func infrastructureTable(d *doc, infra *yaml.Node) {
	if infra == nil {
		return
	}
	d.line("| Resource | What the hosts must provide |")
	d.line("|---|---|")
	for _, k := range infrastructureKeys {
		if v := str(infra, k[0]); v != "" {
			d.line("| %s | %s |", k[1], cell(v))
		}
	}
	d.blank()
}

// hostNeeds reports whether a deployment says what its hosts must
// provide: an infrastructure, or links or listens, which the handover
// writes with or without an infrastructure (ADR-137).
func hostNeeds(dep *yaml.Node) bool {
	if get(dep, "infrastructure") != nil {
		return true
	}
	links, ports := linkSentences(dep)
	return links != "" || ports != ""
}

// customerDeployments are the deployments of an implementation file that
// install an environment the customer runs, in document order.
func customerDeployments(impl *yaml.Node, envs map[string]bool) []sourcePair {
	var out []sourcePair
	for _, dep := range pairs(impl, "deployments") {
		if envs[str(dep.Value, "environment")] {
			out = append(out, dep)
		}
	}
	return out
}

// Handover writes the handover document for the engineers of the customer
// who runs an environment (ADR-129): what they receive, what stays whose
// responsibility, how each implementation builds a release and the licences
// of its dependencies, what the hosts must provide, every setting, the
// migrations, how a release is installed and rolled back, backup and
// restore, what is watched, the checks to run after installing, the
// acceptance tests and the sign-off sheet, in plain words for a reader who
// knows nothing of SpecArch.
func Handover(root *yaml.Node, relRoot string, impls []Implementation, state *State) string {
	d := newDoc("handover", root, relRoot, impls, state)
	info := get(root, "info")
	h := get(root, "handover")
	envs := pairs(root, "environments")
	names := customerRun(envs)
	inEnv := map[string]bool{}
	for _, n := range names {
		inEnv[n] = true
	}
	cfg := pairs(root, "configuration")
	migs := pairs(root, "migrations")
	release := get(root, "release")
	rollback := get(root, "rollback")
	backup := get(root, "backup")
	restore := get(root, "restore")
	signoff := get(root, "signoff")
	var checks, mons, acceptance []sourcePair
	for _, c := range pairs(root, "checks") {
		if inEnv[str(c.Value, "environment")] {
			checks = append(checks, c)
		}
	}
	for _, m := range pairs(root, "monitors") {
		if inEnv[str(m.Value, "environment")] {
			mons = append(mons, m)
		}
	}
	for _, t := range pairs(root, "tests") {
		if str(t.Value, "level") == "acceptance" && str(t.Value, "notApplicable") == "" {
			acceptance = append(acceptance, t)
		}
	}
	type installation struct {
		impl Implementation
		deps []sourcePair
	}
	var installs []installation
	hasBuild, hasSBOM, hasLibraries, hasHosts := false, false, false, false
	for _, i := range impls {
		deps := customerDeployments(i.Node, inEnv)
		if len(deps) == 0 {
			continue
		}
		installs = append(installs, installation{i, deps})
		if len(pairs(i.Node, "libraries")) > 0 {
			hasLibraries = true
		}
		for _, dep := range deps {
			if b := get(dep.Value, "build"); b != nil {
				hasBuild = true
				if str(b, "sbom") != "" {
					hasSBOM = true
				}
			}
			if hostNeeds(dep.Value) {
				hasHosts = true
			}
		}
	}

	d.line("# %s: handover", str(info, "title"))
	d.blank()
	summary := fmt.Sprintf("Version %s of %s.", str(info, "version"), str(info, "title"))
	if len(names) > 0 {
		summary += fmt.Sprintf(" The customer builds, installs and runs %s from the source.", strings.Join(names, " and "))
	}
	summary += " This document says what the customer receives, what stays whose responsibility, how a release is built and installed, what the hosts must provide, and how the installed system is checked and accepted. It is written for the customer's engineers and needs nothing else to be read."
	d.para(summary)
	d.draftNotice("handover")
	d.para(str(h, "description"))
	d.explain(h)

	d.section("What the customer receives")
	to := str(h, "to")
	who := to + "."
	if desc := str(get(get(root, "stakeholders"), to), "description"); desc != "" {
		who = to + ": " + strings.TrimSpace(desc)
	}
	d.para("The handover goes to " + who + " It holds:")
	d.line("- The source of each release: %s", strings.TrimSpace(str(h, "source")))
	if hasBuild {
		d.line("- How a release is built, with the exact toolchain, the base images and how a build is compared with the vendor's (Building a release).")
	}
	if hasSBOM {
		d.line("- The software bill of materials of each release and its vulnerability scan (Building a release).")
	}
	if hasLibraries {
		d.line("- Every dependency with its version and licence (Building a release).")
	}
	if hasHosts {
		d.line("- What the hosts must provide (What the hosts must provide).")
	}
	if len(cfg) > 0 {
		d.line("- Every setting with its type, and where a secret's value comes from (Configuration).")
	}
	if len(migs) > 0 {
		d.line("- The data migrations, in the order they are applied (Data migrations).")
	}
	if release != nil || rollback != nil {
		d.line("- How a release is installed and rolled back (Installing a release, Rolling back).")
	}
	if backup != nil || restore != nil {
		d.line("- How the data is backed up and restored (Backing up, Restoring).")
	}
	if len(mons) > 0 {
		d.line("- What is watched on the live system (Monitoring).")
	}
	if len(checks) > 0 || len(acceptance) > 0 {
		d.line("- The checks to run after installing, and the acceptance tests (Checks after installing, Acceptance tests).")
	}
	if signoff != nil {
		d.line("- The sign-off sheet (Sign-off sheet).")
	}
	d.blank()

	resp := get(h, "responsibilities")
	if resp != nil {
		d.section("Responsibilities")
		for _, side := range [][2]string{{"customer", "The customer is responsible for:"}, {"vendor", "The vendor stays responsible for:"}} {
			if list := strs(resp, side[0]); len(list) > 0 {
				d.para(side[1])
				for _, s := range list {
					d.line("- %s", s)
				}
				d.blank()
			}
		}
	}

	for _, in := range installs {
		impl := in.impl.Node
		var build *yaml.Node
		for _, dep := range in.deps {
			if b := get(dep.Value, "build"); b != nil && build == nil {
				build = b
			}
		}
		libs := pairs(impl, "libraries")
		if build == nil && len(libs) == 0 {
			continue
		}
		title := str(get(impl, "info"), "title")
		d.section("Building a release: " + title)
		stack := get(impl, "stack")
		var parts []string
		if l := get(stack, "language"); l != nil {
			parts = append(parts, fmt.Sprintf("%s %s", str(l, "name"), str(l, "version")))
		}
		if t := get(stack, "toolchain"); t != nil {
			parts = append(parts, fmt.Sprintf("the toolchain %s %s", str(t, "name"), str(t, "version")))
		}
		if p := strs(stack, "platforms"); len(p) > 0 {
			parts = append(parts, "for "+strings.Join(p, ", "))
		}
		if len(parts) > 0 {
			d.para("Built with " + strings.Join(parts, ", ") + ". The versions are exact: a build with another version is a different build.")
		}
		if build != nil {
			d.heading(3, "Build")
			buildDetails(d, impl, build, 3)
		}
		if len(libs) > 0 {
			d.heading(3, "Dependencies and their licences")
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
			d.explainRows(rowsOf(libs))
		}
	}

	if hasHosts {
		d.section("What the hosts must provide")
		for _, in := range installs {
			for _, dep := range in.deps {
				if !hostNeeds(dep.Value) {
					continue
				}
				d.heading(3, fmt.Sprintf("%s: %s", str(get(in.impl.Node, "info"), "title"), dep.Key.Value))
				d.marksAt(dep.Key.Value, dep.Value)
				line := strings.TrimSpace(str(dep.Value, "description"))
				if e := str(dep.Value, "environment"); e != "" {
					line += " Environment: " + e + "."
				}
				d.para(line)
				// The links and listens come first, what the keys say, and
				// the infrastructure's sentences after them for the rest
				// (ADR-137).
				if links, ports := linkSentences(dep.Value); links != "" || ports != "" {
					d.para(links)
					d.para(ports)
					d.block(networkDiagram(root, dep.Key.Value, dep.Value))
				}
				infrastructureTable(d, get(dep.Value, "infrastructure"))
				if servers := items(dep.Value, "servers"); len(servers) > 0 {
					d.line("| Server | Description |")
					d.line("|---|---|")
					for _, s := range servers {
						d.line("| %s | %s |", cell(str(s, "url")), cell(str(s, "description")))
					}
					d.blank()
				}
			}
		}
	}

	if len(cfg) > 0 {
		d.section("Configuration")
		d.para("Every setting the system reads at run time. A secret's value is never written here or in any document; its description says where the value comes from.")
		d.line("| Setting | Type | Secret | Description |")
		d.line("|---|---|---|---|")
		for _, c := range cfg {
			secret := ""
			if str(c.Value, "secret") == "true" {
				secret = "yes"
			}
			d.line("| %s | %s | %s | %s |", c.Key.Value, cell(typeText(get(c.Value, "schema"))), cell(secret), cell(str(c.Value, "description")))
		}
		d.blank()
		d.explainRows(rowsOf(cfg))
		for _, in := range installs {
			for _, dep := range in.deps {
				values := pairs(dep.Value, "configuration")
				if len(values) == 0 {
					continue
				}
				d.para(fmt.Sprintf("Values the vendor gives for %s (%s):", dep.Key.Value, str(get(in.impl.Node, "info"), "title")))
				d.line("| Setting | Value |")
				d.line("|---|---|")
				for _, v := range values {
					d.line("| %s | %s |", v.Key.Value, cell(v.Value.Value))
				}
				d.blank()
			}
		}
	}

	if len(migs) > 0 {
		d.section("Data migrations")
		d.para("Applied in this order, each before the release that carries it is installed.")
		for _, m := range migs {
			d.heading(3, m.Key.Value)
			d.para(str(m.Value, "description"))
			d.explain(m.Value)
			stepsTable(d, get(m.Value, "steps"))
			if rb := get(m.Value, "rollback"); rb != nil {
				d.para("Reversed by:")
				stepsTable(d, rb)
			} else {
				d.para("This migration cannot be reversed.")
			}
		}
	}

	if release != nil {
		d.section("Installing a release")
		d.para(str(release, "description"))
		d.explain(release)
		stepsTable(d, get(release, "steps"))
		for _, in := range installs {
			for _, dep := range in.deps {
				if m := get(dep.Value, "method"); m != nil {
					d.para(fmt.Sprintf("How the vendor installs %s (%s):", dep.Key.Value, str(get(in.impl.Node, "info"), "title")))
					methodPara(d, m)
				}
			}
		}
	}
	for _, p := range []struct {
		node  *yaml.Node
		title string
	}{{rollback, "Rolling back"}, {backup, "Backing up"}, {restore, "Restoring"}} {
		if p.node != nil {
			d.section(p.title)
			d.para(str(p.node, "description"))
			d.explain(p.node)
			stepsTable(d, get(p.node, "steps"))
		}
	}

	if len(mons) > 0 {
		d.section("Monitoring")
		monitorsTable(d, mons)
		for _, in := range installs {
			for _, dep := range in.deps {
				if len(pairs(dep.Value, "monitors")) > 0 {
					d.para(fmt.Sprintf("How the vendor watches them in %s (%s):", dep.Key.Value, str(get(in.impl.Node, "info"), "title")))
					deploymentMonitors(d, dep.Value)
				}
			}
		}
	}

	if len(checks) > 0 {
		d.section("Checks after installing")
		d.para("Run on the installed system before it is accepted. Fill in the Result column as each step is run, and keep the filled-in run.")
		for _, c := range checks {
			d.heading(3, fmt.Sprintf("%s (%s, in %s)", c.Key.Value, str(c.Value, "kind"), str(c.Value, "environment")))
			d.para(str(c.Value, "description"))
			if v := strs(c.Value, "verifies"); len(v) > 0 {
				d.para("Verifies " + strings.Join(v, ", ") + ".")
			}
			d.explain(c.Value)
			steps := itemsOf(get(c.Value, "steps"))
			if len(steps) > 0 {
				d.line("| Step | Action | Expected | Result |")
				d.line("|---|---|---|---|")
				for i, s := range steps {
					d.line("| %d. %s | %s | %s | %s |", i+1, cell(str(s, "name")), cell(str(s, "action")), cell(str(s, "check")), cell(""))
				}
				d.blank()
				d.explainRows(stepRows(get(c.Value, "steps")))
			}
		}
	}

	if len(acceptance) > 0 {
		d.section("Acceptance tests")
		d.para("What the installed system must do for the people who use it, each as a scenario to run by hand or through the implementation's acceptance suite. A golden scenario succeeds; a red one is refused.")
		for _, t := range acceptance {
			d.heading(3, t.Key.Value)
			attrs := []string{"Scenario: " + str(t.Value, "scenario")}
			if v := strs(t.Value, "verifies"); len(v) > 0 {
				attrs = append(attrs, "verifies "+strings.Join(v, ", "))
			}
			d.para(strings.Join(attrs, "; ") + ".")
			d.line("- Given: %s", oneParagraph(str(t.Value, "given")))
			d.line("- When: %s", oneParagraph(str(t.Value, "when")))
			d.line("- Then: %s", oneParagraph(str(t.Value, "then")))
			d.blank()
			d.explain(t.Value)
		}
	}

	if signoff != nil {
		d.section("Sign-off sheet")
		d.line("| Run | Entry |")
		d.line("|---|---|")
		d.line("| Specification version | %s |", str(info, "version"))
		d.line("| Build | %s |", cell(""))
		d.line("| Environment | %s |", cell(""))
		d.line("| Date | %s |", cell(""))
		d.blank()
		d.para("The system is accepted when every criterion is met:")
		d.line("| Criterion | Met |")
		d.line("|---|---|")
		for _, c := range strs(signoff, "criteria") {
			d.line("| %s | %s |", cell(c), cell(""))
		}
		d.blank()
		d.line("| Role | Description | Name | Signature | Date |")
		d.line("|---|---|---|---|---|")
		for _, s := range items(signoff, "signers") {
			d.line("| %s | %s | %s | %s | %s |", str(s, "role"), cell(str(s, "description")), cell(""), cell(""), cell(""))
		}
		d.blank()
		d.explain(signoff)
	}
	d.sourcesIndex("Sources")
	return d.String()
}
