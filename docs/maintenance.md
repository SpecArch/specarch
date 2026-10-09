# After commissioning: changes, defects, releases and operation

A specification describes a system as it is now. After commissioning the
system keeps changing: a stakeholder asks for something new, someone finds
a defect, a release goes out, the live system misbehaves. This document is
the design of how SpecArch records that life, during development and in
production, without turning the specification into a change log. The
operation stage, the records with their rules, the release rules, the
diff verb and the two documents made from the records are built, and the
implementation items are listed at the end.

The processes come from ISO/IEC/IEEE 12207:2017: configuration management
(6.3.5), operation (6.4.12) and maintenance (6.4.13). Versions follow
Semantic Versioning 2.0.0. The names of the kinds of change come from
ISO/IEC/IEEE 14764:2022 and ITIL 4.

## The test that decides where a thing lives

One question decides whether something belongs in the specification or
beside it: does the file change on every event, or grow with every event?

- If it does, it is a record. A change request moves through statuses, a
  defect is found and fixed, a release happens once, an incident happens
  once. Written into the specification, each would leave a trail of what
  used to be, and a specification that grows with every event is a change
  log, which `docs/principles.md` forbids.
- If it describes what is true of the system now, it belongs in the
  specification. A monitor and its threshold describe the live system the
  way a requirement does, so they are specification.

This is the rule the stage design already applies to commissioning: a
run's results are records under `records/commissioning/`, because a design
file rewritten after every run would be a log.

So change requests, defects, releases and incidents are records. The
specification never points back at them: no element carries a list of the
changes that touched it. That list would be a change log under another
name, and the documentor can compute it from the records, which point
forward into the specification.

## Records

Records live in `records/` beside the specification's folder, one file per
record, in a folder per kind. A specification in `project/spec/` keeps its
records in `project/records/`; a `records/` folder inside the
specification's own folder is a layout error, because everything in that
folder is specification.

| Folder | One file per | Kind | Named by |
|---|---|---|---|
| `records/changes/` | change request | `change` | its ID, `CR-12.yaml` |
| `records/defects/` | defect | `defect` | its ID, `DEF-7.yaml` |
| `records/releases/` | release | `release` | its version, `1.4.0.yaml` |
| `records/incidents/` | incident in production | `incident` | its ID, `INC-3.yaml` |
| `records/commissioning/` | commissioning run | `commissioning` | its date and environment, `2026-10-07-production.yaml` |
| `records/approvals/` | approval for code generation | `approval` | its version, `1.4.0.yaml` |

A record file starts with `specarchRecord: "0.1"` and `kind`, and has its
own schema, `schema/specarch-record-0.1.schema.json`, beside the other two.
The file name repeats the ID or version, so a reader finds a record by
walking the folders, and the validator checks the two agree. An approval
is written by `specarch approve` (`docs/conventions.md`, Approval records)
and is checked like the others.

A record points into the specification in one of two ways. A requirement
is named by its ID, `LIB-5`, as `satisfies` names it. Everything else is a
`#/` pointer into the merged specification: `#/entities/Loan`,
`#/tests/borrow-limit`, `#/environments/production`,
`#/configuration/LATE_FEE`. A bare name is not allowed in a list that can
hold several kinds of element, because `production` could be an
environment, a test and a setting at once, and a reference that could mean
two things is an error under `docs/principles.md`. A field that holds one
kind of element (a defect's `test`, an incident's `monitor`) takes the bare
name, as `environment` does in the specification.

A record names people only by role: a stakeholder key of the
specification, such as `head-librarian`, never a person's name. It holds no
host secret, no customer data and no personal data. The schema enforces the
role (a role field must be a stakeholder key, checked as a reference), and
the same reasoning as the `secret_value` rule applies to the rest: records
are read and copied by more people than a ticket system.

A project that keeps change requests or defects in an external tracker does
not copy them into records. It declares the tracker under `sources`, the
way an external requirement set is declared today, with two more kinds:
`change-set` and `defect-set`, each with a `prefix`. A release then lists
`JIRA-120` and the validator resolves it to the tracker, as it resolves
`JIRA-12` in `satisfies` now. One tracker that holds several kinds is
declared once per kind; the prefixes may be the same.

## Change requests

A change request asks for the system to be different: a new capability, a
change to one, a removal, or an adaptation to a new environment. It can be
raised during development or in production.

| Field | Holds |
|---|---|
| `id`, `title` | `CR-12`, and one line |
| `raisedBy`, `raised` | the stakeholder role, and the date |
| `phase` | `development` or `production` |
| `type` | `additive`, `perfective`, `adaptive` or `preventive`: the maintenance types of ISO/IEC/IEEE 14764 other than corrective, which is a defect |
| `urgency` | `standard`, `normal` or `emergency`: the change types of ITIL 4 change enablement, which set how much assessment it gets before approval |
| `reason` | why it is asked for, in plain words |
| `affects` | what it changes, by stage: `adds`, `changes` and `removes`, each a list of references into the specification (requirement IDs, `#/` pointers such as `#/entities/Loan`, test names, environment and setting keys) |
| `impact` | `major`, `minor` or `patch`, by the version rule below |
| `decision` | `by` (a role), `date`, `outcome` (approved or rejected) and `why` |
| `release` | the version it shipped in, once released |
| `status` | below |

Statuses and transitions:

| From | To | Trigger | Who |
|---|---|---|---|
| proposed | analysed | the impact is written into `affects` and `impact` | the analyst role |
| analysed | approved, rejected | the decision | the role that owns the requirement it affects |
| approved | implemented | the specification and the code carry the change, in one commit or merge | the developer role |
| implemented | released | a release record includes it | the release manager role |
| proposed, analysed, approved | withdrawn | the requester takes it back | the requester |

Analysis is a status of its own because an approval without a written
impact is the usual way a small request turns out to break an interface.

The commit that implements a change names its ID in the message. That link
lives in git, where history belongs; the validator does not read it.

## Defects

A defect is the system not doing what its specification says. Raised
during development or in production, it is the corrective maintenance of
14764.

| Field | Holds |
|---|---|
| `id`, `title` | `DEF-7`, and one line |
| `reportedBy`, `reported` | the role, and the date |
| `phase`, `environment` | where it was found: `development` or `production`, and the environment key |
| `severity` | `critical`, `major`, `minor` or `cosmetic`: how badly it stops the system from meeting its requirements |
| `violates` | the requirement IDs, or `#/` pointers to the design elements, it breaks |
| `test` | the test in `tests/` that shows it is fixed: it fails before the fix and passes after |
| `incidents` | the incidents it caused, by ID |
| `duplicateOf` | for a duplicate, the defect it repeats |
| `change` | for one that is not a defect, the change request it became |
| `release` | the version the fix shipped in |
| `status` | below |

| From | To | Trigger |
|---|---|---|
| reported | confirmed, not-a-defect, duplicate | triage against the specification |
| confirmed | fixed | the fix is merged and `test` passes |
| fixed | released | a release record includes it |

`not-a-defect` is the answer when the system does what the specification
says and the reporter wanted something else; that is a change request, and
the defect names it under `change`. Triage against the specification is the point of
keeping one: a defect is measured against a requirement, not against what
someone expected.

A defect that only an incident in production found needs a test that
reproduces it before it can be fixed, so the same defect cannot come back
unseen. The test is an ordinary test of the specification, red or golden,
and it verifies the requirement the defect violates.

## Releases and versions

The system's version is the specification's `info.version`, a Semantic
Versioning 2.0.0 version. The specification is what clients, deployers and
testers rely on, so a version that describes it is the one that tells them
what changed.

What counts as the public interface of rules 6 to 8 of SemVer, for a
SpecArch specification: the paths and their operations, the commands with
their arguments and exit statuses, the channels and their messages, the
routes of the pages, and the settings a deployer gives values to, with the
fields of every entity and enum they carry.

| Impact | When |
|---|---|
| `major` | a part of the public interface is removed or changes so that a client written for the previous version breaks (rule 8) |
| `minor` | something is added to the public interface and nothing a client relies on changes (rule 7) |
| `patch` | nothing in the public interface changes: a defect fix, or a change inside the system (rule 6) |

Before 1.0.0 the step for a `major` change is the minor number: 0.3.0
after 0.2.x. SemVer rule 4 says that in major version zero anything may
change at any time, so the standard itself asks for no step there, and a
rule that asked for none would check nothing for a young system. The
minor number is the step Cargo and npm's caret ranges already treat as
breaking under 1.0.0, so a client that pins `^0.2` is protected the way
`^1.2` protects one after 1.0.0. A `minor` or `patch` change before 1.0.0
needs the patch step.

Versions are ordered by SemVer's precedence (rule 11): major, minor and
patch as numbers, and a version with a pre-release tag below the same
version without one.

A release record holds `version` (its file name), `status` (planned,
released or withdrawn), the `date`, the change requests and defects it
`includes`, the `specificationVersion` (`info.version` at release, equal to
`version`), the version of each implementation file
(`implementations: { go: 1.4.0 }`), the commissioning records of the run
that accepted it, and an optional `commit`.

While a release is being built, `info.version` carries its version with a
pre-release tag (rule 9), such as `1.5.0-dev`, and a release record with
status planned exists for `1.5.0`. On release the tag is dropped in the
same commit that sets the record to released.

Version N+1 relates to version N through two things. The release record of
N+1 says what it carries. Git holds both specifications, one tag per
release, and `specarch diff <old> <new>` compares them: it lists what was
added, changed and removed, classifies each by the table above, and checks
that the release's version step is at least as large as the largest
change, and that every changed element is named in the `affects` of a
change request or the `violates` of a defect the release includes. The
validator, which sees one version at a time, cannot make either check.

### The diff verb

`specarch diff <old> <new>` takes two specification folders. The old one
is normally the previous release's tag checked out on its own
(`git worktree add ../v1.4.0 v1.4.0`), the new one the working tree. Both
are validated first, and the errors of a specification that has them are
printed; it is compared as far as it can be read, and the command then
exits 1 whatever the checks find (ADR-065). The records read are the new one's: the
`records/` beside the new folder. The release is the record of the new
`info.version` with any pre-release tag dropped, so `1.5.0-dev` is checked
against `records/releases/1.5.0.yaml`, and the step is measured from the
old `info.version`.

What is compared is one element at a time: each named object of a section
(`#/entities/Loan`, `#/requirements/LIB-5`, `#/tests/borrow-limit`), each
operation (`#/paths/~1loans/post`), each source, and the sections that
are one object (`#/release`, `#/rollback`, `#/signoff`). `info`, `stages`
and `specarch` are not compared: the version always changes, and the
stages are layout.

The public interface of the table above is, element by element: every
operation, command, channel and setting; the `route` of every page; and
every entity and enum one of those reaches through `$ref`, directly or
through another entity. Every other element is inside the system, so any
change to it is `patch`.

A public element that is added is `minor` and one that is removed is
`major`. A public element that changes takes the largest impact of its
changes, each found by these rules, in this order:

| Change | Impact |
|---|---|
| a descriptive key: `description`, `summary`, `title`, `why`, `cites`, `examples`, `satisfies`, `verifies`, `origin`, `decidedIn`, or one starting `x-` | `patch` |
| a value added to an `enum` list, or a name removed from a `required` list | `minor` |
| a value removed from an `enum` list, or a name added to a `required` list | `major` |
| in a list whose items each have a `name` (parameters, arguments, options), an item added | `minor`, or `major` when it has `required: true` |
| in such a list, an item removed | `major` |
| a key added | `minor`, or `major` when the key is `required` and its value is not false |
| a key removed | `major` |
| any other value or list that differs | `major` |

The last row is cautious on purpose. A schema bound that moves, such as a
`maxLength` that grows, keeps old clients working when it bounds what they
send and breaks them when it bounds what they receive, and the diff
cannot tell which side of the interface a schema is on. A change it cannot
show to be safe is called `major`, with the reason printed, so a person
decides; calling it `minor` would let a breaking release through unseen.

Standard output is the change list, one line per element in pointer
order, `<impact> <added|changed|removed> <pointer>`, followed for a
changed public element by the change that decided its impact; then one
line per failed check, starting `error:`. The checks:

- `version`: the release's version is after the old one, and steps from
  it by at least the largest impact in the list, with the step before
  1.0.0 as above; the line names the version to release instead.
- `covered`: every element in the list is named by an entry of the
  `affects` of a change request, or of the `violates` of a defect, that
  the release includes. An entry names an element when one of the two
  pointers is a prefix of the other, and a requirement ID names
  `#/requirements/<ID>`. When the release includes an ID kept in a
  tracker, whose `affects` the diff cannot read, an element no record
  names is printed as a `warning:` line instead of an error.

The exit status is 0 when every check passes, 1 when one fails, there is
no release record for the new version or a specification has errors, and
2 on a usage error or a folder that cannot be read.

## Operation

Operation (12207, 6.4.12) is a stage of the specification only as far as
the flows above need it: what is watched on the live system, so that an
incident can say which promise it broke.

A new stage, `operation`, holds one section, `monitors`. A monitor says
what is measured, in which environment, the `objective` it must meet as
one verifiable sentence ("nine calls in ten answer within 300 ms over a
day"), and the requirements it `verifies`. It carries `why` and `cites`
like every element. The tool that measures, the query and where an alert
goes are implementation: the implementation file's deployments name, per
monitor, how that stack watches it.

An incident record holds `id`, `detected` (date), `environment`, the
`monitor` that caught it or, when no monitor did, the role that reported it
(`reportedBy`), a `summary`, the `impact`, `status` (open or resolved), and
the `defects` and `changes` that follow from it. A resolved incident that
leads to neither says why in `noChange`.

Rollback is already the deployment stage's `rollback`; an incident record
says whether it was used in `rollbackUsed`, true or false.

A commissioning record holds the `environment`, the `date`, the `version`
of the specification it ran against, the `build`, the `operator` as a
role, the `results` keyed by check name, each with `result` (pass, fail or
skipped) and a `note`, and the `signoff` with the role that signed and the
date.

## Stack-neutral and stack-specific

| In the specification or the records (stack-neutral) | In the implementation file (one stack) |
|---|---|
| change and defect statuses, `affects`, `impact`, `severity` | the tracker's URL template, when it is not a declared source |
| monitors, their objectives and environments | the monitoring tool, the query, alert routes |
| release versions and contents | the pipeline that builds and ships, image tags, the host |
| incident records | runbooks with commands |

## What the validator will check

Each rule reports against the record file and line it is about, as an
error unless it says warning.

| Rule | Checks |
|---|---|
| `record_name` | a record's file name equals its `id`, its `version` for a release or an approval, or its date and environment for a commissioning run, and it sits in the folder of its kind |
| `record_ref` | every role is a stakeholder of the specification; every requirement ID, `#/` pointer, test, environment and monitor a record names resolves; every change, defect, incident, release and commissioning run it names is a record, or a change or defect whose prefix is that of a declared change-set or defect-set. A requirement's `release` is a release record that is planned or released. A change's `affects` belong to `change_applied`, a defect's `duplicateOf` to `defect_duplicate` and a commissioning run's checks to `commissioning_record` |
| `change_applied` | for a change that is implemented or released, every `adds` and `changes` reference resolves and every `removes` reference does not (a removed requirement may instead stay with status retired); for one still open (proposed, analysed or approved), `changes` and `removes` resolve, and an `adds` that already resolves is a warning. A requirement of an external set is taken as resolving and never as removed, since the validator cannot read the set |
| `change_decision` | a change that is approved, implemented or released has a `decision` with outcome approved; a rejected one has outcome rejected |
| `defect_test` | a fixed or released defect names a `test` that verifies a requirement the defect violates, or whose subject (its operation, command, page or entity) is the element a violated `#/` pointer points into |
| `defect_duplicate` | a duplicate names a defect that exists and is not itself a duplicate |
| `release_contents` | a released release includes only changes that are implemented or released and defects that are fixed or released; a change or defect with status released names a released release that includes it, and one that a released release includes and whose status is released names that release |
| `release_bump` | a released version steps from the previous released version by at least the largest `impact` of what it includes, a defect counting as patch, with the step before 1.0.0 as above; withdrawn and planned releases are left out, and so is an ID of a tracker, whose impact the validator cannot read |
| `release_version` | once a release is released, `info.version` is the newest released version, or a later version with a pre-release tag whose release is planned; and a release's `specificationVersion` is its `version` |
| `incident_link` | a resolved incident links to a defect or a change, or says `noChange` (warning) |
| `commissioning_record` | a commissioning record's results name checks that exist, and its version is a release's |

`monitors` adds no new rule: a monitor's `environment` uses the existing
`environment` rule, its `verifies` the existing `requirement` rule, and the
traceability warning counts a monitor as verifying the requirements it
names.

## What the documentor will make

Two targets join `DocumentTarget`, after the first set of documents:

- `changes`: the change and defect register. A table of the change
  requests and one of the defects, open items first (a change that is
  proposed, analysed or approved, a defect that is reported or confirmed),
  then the done ones, then the ones closed without a change; within each
  group by ID. After each table, one paragraph per item: who raised it and
  when, its type and urgency or its severity, its reason, what it affects
  or violates, the test that shows a fix, and the decision with its why.
- `releases`: release notes, newest first by SemVer's precedence. For each
  version its status and date, then what it includes, grouped as added,
  changed, removed (from the `affects` of each change) and fixed (the
  defects), with an included ID kept in a tracker listed apart; then the
  specification and implementation versions, the commissioning runs that
  accepted it and the commit.

Both read the records beside the specification, name the records folder
in their generated-from header, and are never drafts: an open question is
about the specification, and these are made from what happened. For the
same reason they are not rows of the open questions document's Outputs
table, which says what can be made from the specification now.

The rule that documents carry no change-log text is about documents made
from the specification, which describe the system as it is. These two are
made from records, which are history by nature, so they read as history;
that is what they are for.

## What this changes in the stage design, and why

- A seventh stage, `operation`, after `commissioning`, with the section
  `monitors`. 12207 places operation after transition and validation, and
  a monitor is a promise about the live system, like a requirement, so it
  belongs in the specification. This touches the stage list and the
  section map in both validator builds, the design schema, `docs/stages.md`
  and `docs/conventions.md`.
- Two new source kinds, `change-set` and `defect-set`, beside
  `requirement-set`, so an external tracker is declared the one way it is
  declared today.
- The commissioning record becomes one kind of record file,
  with the shape `docs/conventions.md` already gives it, under the new
  record schema. No file in the repository holds one yet, so nothing moves.
- Nothing else in the stage design changes. In particular no element
  gains a back-reference to the changes or defects that touched it.

## Implementation items

In order; each changes the specification of `specarch` first, both
validator builds where it adds a rule, and the conformance cases.

1. The operation stage: `operation` and `monitors` in the schema and both
   loaders, the traceability warning counting monitors, the implementation
   file's per-monitor entry, `docs/stages.md` and `docs/conventions.md`,
   and monitors in SpecArch's own specification and library-lending.
2. Records: the record schema, reading `records/` beside a specification in
   both builds, and the rules `record_name`, `record_ref`,
   `change_applied`, `change_decision`, `defect_test`, `defect_duplicate`,
   `incident_link` and `commissioning_record`, with the source kinds
   `change-set` and `defect-set`, and example records in library-lending.
3. Releases: the rules `release_contents`, `release_bump` and
   `release_version` in both builds, and release records for SpecArch's
   own versions.
4. `specarch diff <old> <new>`: the change list, the classification by the
   version rule, and the two checks against the release records. Go only,
   like the other verbs.
5. The documentor targets `changes` and `releases`, after the first set
   of documents.
