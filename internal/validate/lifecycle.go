package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/metamodel"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// checkRequirementsStage checks the links inside the requirements stage:
// a requirement's needs exist, a need's stakeholders exist.
func (c *checker) checkRequirementsStage(d *design) {
	for name, need := range d.needs {
		for i, sh := range source.Items(source.Child(need, "stakeholders")) {
			if sh.Value != "" && d.stakeholders[sh.Value] == nil {
				c.add(sh, source.Pointer("needs", name, "stakeholders", fmt.Sprint(i)), RuleStakeholder,
					"%s is not a stakeholder of the specification%s", sh.Value, suggest(sh.Value, d.stakeholders))
			}
		}
	}
	for name, src := range d.sources {
		for i, sh := range source.Items(source.Child(src, "participants")) {
			if sh.Value != "" && d.stakeholders[sh.Value] == nil {
				c.add(sh, source.Pointer("sources", name, "participants", fmt.Sprint(i)), RuleStakeholder,
					"%s is not a stakeholder of the specification%s", sh.Value, suggest(sh.Value, d.stakeholders))
			}
		}
	}
	for name, req := range d.requirements {
		if by := source.Child(req, "ownedBy"); by != nil && by.Value != "" && d.stakeholders[by.Value] == nil {
			c.add(by, source.Pointer("requirements", name, "ownedBy"), RuleStakeholder,
				"%s is not a stakeholder of the specification%s", by.Value, suggest(by.Value, d.stakeholders))
		}
		for i, n := range source.Items(source.Child(req, "needs")) {
			if n.Value != "" && d.needs[n.Value] == nil {
				c.add(n, source.Pointer("requirements", name, "needs", fmt.Sprint(i)), RuleNeed,
					"%s is not a need of the specification%s", n.Value, suggest(n.Value, d.needs))
			}
		}
	}
	c.checkStakeholdersComplete(d)
}

// checkStakeholdersComplete warns while no role has confirmed that the
// stakeholder list is complete (ADR-231): the warning stands at the list
// until the root holds stakeholdersComplete, whose by is a stakeholder
// and whose source, when given, is a declared source. A must question
// that blocks the stakeholders holds the warning, since the gap is then
// already asked about. Writing order stays free: the rest of the
// specification is read as it is. The warning is anchored at the list
// itself, whose line is its first entry's in the file that holds it (the
// merged document's section key carries the same line), and at line 1 of
// the root file when there is no list; the pointer is /stakeholders
// either way. A specification with no stakeholders is told to list them
// first, since by must be one of them. A file on a meta-model before
// 1.0.0, whose schema has no stakeholdersComplete, is not warned: it
// could not follow the remedy, and its meta_model_version warning already
// tells it to upgrade.
func (c *checker) checkStakeholdersComplete(d *design) {
	if source.Child(d.root, "stakeholdersComplete") == nil {
		if d.stakeholdersUnconfirmed() {
			msg := stakeholdersUnconfirmed
			if len(d.stakeholders) == 0 {
				msg = stakeholdersNone
			}
			c.warn(source.Child(d.root, "stakeholders"), "/stakeholders", RuleStakeholdersUnconfirmed, "%s", msg)
		}
		return
	}
	complete := source.Child(d.root, "stakeholdersComplete")
	if by := source.Child(complete, "by"); by != nil && by.Value != "" && d.stakeholders[by.Value] == nil {
		c.add(by, "/stakeholdersComplete/by", RuleStakeholder, "%s is not a stakeholder of the specification%s", by.Value, suggest(by.Value, d.stakeholders))
	}
	if src := source.Child(complete, "source"); src != nil && src.Value != "" && d.sources[src.Value] == nil {
		if len(d.sources) == 0 {
			c.add(src, "/stakeholdersComplete/source", RuleSource, "%s is not a declared source: the specification declares none; add it under sources in %s", src.Value, spec.RootFile)
		} else {
			c.add(src, "/stakeholdersComplete/source", RuleSource, "%s is not a source declared under sources%s", src.Value, suggest(src.Value, d.sources))
		}
	}
}

// StakeholdersUnconfirmed reports whether a specification's stakeholder
// list is unconfirmed as the validator sees it (ADR-231). The documents,
// the site and approve show the banner and the warning on the same
// condition, so one gap gets one mark.
func StakeholdersUnconfirmed(root *yaml.Node) bool {
	return newDesign(root).stakeholdersUnconfirmed()
}

// stakeholdersUnconfirmed reports whether the root holds no
// stakeholdersComplete, no open must question blocks the stakeholders,
// and the file is on a meta-model whose schema has the key.
func (d *design) stakeholdersUnconfirmed() bool {
	return source.Child(d.root, "stakeholdersComplete") == nil &&
		!d.mustBlocked("stakeholders") &&
		!metamodel.Design.IsEarlier(d.metaModelVersion())
}

// stakeholdersUnconfirmedRemedy is the remedy both messages of
// stakeholders_unconfirmed end with, the same in both validators.
const stakeholdersUnconfirmedRemedy = "add stakeholdersComplete with by, the role that confirms it, and date, and name under source the declared source that records the confirmation, when there is one; the key ends this warning, the banner on every generated document and the warning of specarch approve"

// stakeholdersUnconfirmed is the message of stakeholders_unconfirmed at a
// list nobody has confirmed, and stakeholdersNone the one at a
// specification that lists no stakeholders yet, where by has nothing to
// name until the list is written.
const (
	stakeholdersUnconfirmed = "the stakeholder list is not confirmed complete, so a role with an interest in the system may still be missing from it; once it is complete, " + stakeholdersUnconfirmedRemedy
	stakeholdersNone        = "the specification lists no stakeholders, so no role has confirmed that the list is complete; list under stakeholders every role with an interest in the system, then " + stakeholdersUnconfirmedRemedy
)

// checkDeploymentStage checks the links inside the deployment,
// commissioning and operation stages, and that no secret carries a value.
func (c *checker) checkDeploymentStage(d *design) {
	for name, env := range d.environments {
		next := source.Child(env, "promotesTo")
		if v := source.Str(next); v != "" && d.environments[v] == nil {
			c.add(next, source.Pointer("environments", name, "promotesTo"), RuleEnvironment,
				"%s is not an environment of the specification%s", v, suggest(v, d.environments))
		}
		// The last environment is the one its users see; a trace there
		// shows them the inside of the system (ADR-222).
		if trace := source.Child(env, "trace"); next == nil && source.Str(trace) == "true" {
			c.warn(trace, source.Pointer("environments", name, "trace"), RuleEnvironmentTrace,
				"%s shows the trace and promotes to no environment, so it is the last one, where the people who use the system would see each screen's pointer into the specification; set trace on the environment they test in, or give %s a promotesTo", name, name)
		}
	}
	for name, chk := range d.checks {
		env := source.Child(chk, "environment")
		if v := source.Str(env); v != "" && d.environments[v] == nil {
			c.add(env, source.Pointer("checks", name, "environment"), RuleEnvironment,
				"%s is not an environment of the specification%s", v, suggest(v, d.environments))
		}
	}
	for name, mon := range d.monitors {
		env := source.Child(mon, "environment")
		if v := source.Str(env); v != "" && d.environments[v] == nil {
			c.add(env, source.Pointer("monitors", name, "environment"), RuleEnvironment,
				"%s is not an environment of the specification%s", v, suggest(v, d.environments))
		}
	}
	for name, setting := range d.settings {
		if source.Str(source.Child(setting, "secret")) != "true" {
			continue
		}
		if def := source.Child(source.Child(setting, "schema"), "default"); def != nil {
			c.add(def, source.Pointer("configuration", name, "schema", "default"), RuleSecretValue,
				"%s is a secret, and a secret's value is never written in a specification; remove the default and say in the description where the value comes from", name)
		}
	}
	c.checkHandover(d)
}

// checkHandover checks who runs each environment (ADR-129): one the
// customer runs needs the handover, release and rollback, and should have
// a commissioning check and a monitor in it and the backup and restore
// procedures; a handover needs an environment the customer runs, and goes
// to a stakeholder. An open must question that blocks the stage or the
// section holds the problem about what that section lacks.
func (c *checker) checkHandover(d *design) {
	handover := source.Child(d.root, "handover")
	var customerRun []string
	for _, name := range sortedKeys(d.environments) {
		if source.Str(source.Child(d.environments[name], "runBy")) == "customer" {
			customerRun = append(customerRun, name)
		}
	}
	for _, name := range customerRun {
		runBy := source.Child(d.environments[name], "runBy")
		ptr := source.Pointer("environments", name, "runBy")
		if handover == nil && !d.mustBlocked("handover") {
			c.add(runBy, ptr, RuleHandover, "%s is run by the customer, and the specification has no handover saying what they receive; add handover with description, to, %s and responsibilities", name, d.sourceDeliveryKey())
		}
		for _, proc := range []string{"release", "rollback"} {
			if source.Child(d.root, proc) == nil && !d.mustBlocked(proc) {
				c.add(runBy, ptr, RuleHandover, "%s is run by the customer from the %s steps, and the specification has none; add %s with its steps", name, proc, proc)
			}
		}
		if !d.mustBlocked("checks") && !d.anyIn(d.checks, name) {
			c.warn(runBy, ptr, RuleHandover, "no commissioning check runs in %s, so the customer has nothing to run after installing it; add a check with environment: %s", name, name)
		}
		if !d.mustBlocked("monitors") && !d.anyIn(d.monitors, name) {
			c.warn(runBy, ptr, RuleHandover, "no monitor watches %s, so the customer is not told what to watch; add a monitor with environment: %s", name, name)
		}
		var missing []string
		for _, proc := range []string{"backup", "restore"} {
			if source.Child(d.root, proc) == nil && !d.mustBlocked(proc) {
				missing = append(missing, proc)
			}
		}
		if len(missing) > 0 {
			c.warn(runBy, ptr, RuleHandover, "%s is run by the customer, and the specification has no %s procedure to hand them; add %s under operation", name, strings.Join(missing, " and "), strings.Join(missing, " and "))
		}
	}
	if handover != nil && len(customerRun) == 0 {
		c.add(handover, "/handover", RuleHandover, "no environment is run by the customer, so there is nobody to hand over to; add runBy: customer to the environment the customer runs, or remove handover")
	}
	if to := source.Child(handover, "to"); to != nil && to.Value != "" && d.stakeholders[to.Value] == nil {
		c.add(to, "/handover/to", RuleStakeholder, "%s is not a stakeholder of the specification%s", to.Value, suggest(to.Value, d.stakeholders))
	}
}

// sourceDeliveryKey is the key a handover gives the way the source reaches
// the customer under, in the file's version: source before 1.0.0.
func (d *design) sourceDeliveryKey() string {
	if metamodel.Design.IsEarlier(d.metaModelVersion()) {
		return metamodel.SourceDeliveryOldKey
	}
	return metamodel.SourceDeliveryKey
}

// anyIn reports whether any element of a section (checks or monitors) names
// the environment.
func (d *design) anyIn(section map[string]*yaml.Node, env string) bool {
	for _, n := range section {
		if source.Str(source.Child(n, "environment")) == env {
			return true
		}
	}
	return false
}

// mustBlocked reports whether an open must question blocks a section by
// name, or the stage it is in: the one licence for the section to be
// missing.
func (d *design) mustBlocked(section string) bool {
	stage := spec.Sections[section]
	for _, q := range d.questions {
		if source.Str(source.Child(q, "priority")) != "must" {
			continue
		}
		for _, item := range source.Items(source.Child(q, "blocks")) {
			b, ok := spec.ParseBlock(item.Value)
			if ok && !b.IsPointer() && (b.Section == section || b.Section == "" && b.Stage == stage) {
				return true
			}
		}
	}
	return false
}

// mustBlockedEnvironment reports whether an open must question holds up an
// environment: it blocks the environment itself, the environments section
// or the deployment stage (ADR-172).
func (d *design) mustBlockedEnvironment(env string) bool {
	if d.mustBlocked("environments") {
		return true
	}
	if env == "" {
		return false
	}
	for _, q := range d.questions {
		if source.Str(source.Child(q, "priority")) != "must" {
			continue
		}
		for _, item := range source.Items(source.Child(q, "blocks")) {
			b, ok := spec.ParseBlock(item.Value)
			if ok && len(b.Tokens) == 2 && b.Tokens[0] == "environments" && b.Tokens[1] == env {
				return true
			}
		}
	}
	return false
}

// checkTraceability reports what the stages a specification covers leave
// open: a need no requirement refines, a requirement without acceptance
// criteria, a requirement nothing satisfies once there is a design, and a
// requirement nothing verifies once there are tests, checks or monitors. A
// requirement another stakeholder owns (ownedBy) is fulfilled by that
// stakeholder's component, so nothing here satisfies or verifies it. All
// are warnings.
func (c *checker) checkTraceability(d *design) {
	satisfied := map[string]bool{}
	verified := map[string]bool{}
	walk(d.root, nil, func(n *yaml.Node, path []string) {
		for _, item := range source.Items(source.Child(n, "satisfies")) {
			satisfied[item.Value] = true
		}
		for _, item := range source.Items(source.Child(n, "verifies")) {
			verified[item.Value] = true
		}
	})
	if d.spec != nil {
		for _, impl := range d.spec.Implementations {
			doc := source.Parse(impl.Data)
			walk(doc.Root, nil, func(n *yaml.Node, path []string) {
				for _, item := range source.Items(source.Child(n, "satisfies")) {
					satisfied[item.Value] = true
				}
			})
		}
	}
	refined := map[string]bool{}
	for _, req := range d.requirements {
		for _, n := range source.Items(source.Child(req, "needs")) {
			refined[n.Value] = true
		}
	}
	hasDesign := d.covers("design")
	hasTests := len(source.Pairs(source.Child(d.root, "tests"))) > 0 || len(d.checks) > 0 || len(d.monitors) > 0
	for _, p := range source.Pairs(source.Child(d.root, "needs")) {
		if source.Str(source.Child(p.Value, "status")) == "rejected" {
			continue // a rejected need will not be met, so no requirement refines it
		}
		if !refined[p.Key.Value] {
			c.warn(p.Key, source.Pointer("needs", p.Key.Value), RuleNeedUnrefined,
				"no requirement refines need %s; add a requirement with needs: [%s], or set the need's status to rejected", p.Key.Value, p.Key.Value)
		}
	}
	for _, p := range source.Pairs(source.Child(d.root, "requirements")) {
		id := p.Key.Value
		ptr := source.Pointer("requirements", id)
		status := source.Str(source.Child(p.Value, "status"))
		if status == "rejected" || status == "retired" {
			continue
		}
		if source.Child(p.Value, "acceptance") == nil {
			c.warn(p.Key, ptr, RuleAcceptanceMissing, "requirement %s has no acceptance criteria, so no test can show it is met; add acceptance with one verifiable sentence per criterion", id)
		}
		if source.Str(source.Child(p.Value, "ownedBy")) != "" {
			continue
		}
		if hasDesign && !satisfied[id] {
			c.warn(p.Key, ptr, RuleRequirementUnsatisfied, "no design element satisfies requirement %s; add satisfies: [%s] to the entity, operation, command, page, algorithm or decision that meets it", id, id)
		}
		if hasTests && !verified[id] {
			c.warn(p.Key, ptr, RuleRequirementUnverified, "no test, commissioning check or monitor verifies requirement %s; add verifies: [%s] to the test that shows it is met", id, id)
		}
	}
}

// covers reports whether the specification has any section of a stage.
func (d *design) covers(stage string) bool {
	if d.spec != nil {
		return d.spec.Covers(stage)
	}
	for _, name := range spec.SectionsOf(stage) {
		if source.Child(d.root, name) != nil {
			return true
		}
	}
	return false
}
