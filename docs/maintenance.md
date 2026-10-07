# After commissioning: changes, defects, releases and operation

A specification describes a system as it is now. After commissioning the
system keeps changing: a stakeholder asks for something new, someone finds
a defect, a release goes out, the live system misbehaves. This document is
the design of how SpecArch records that life, during development and in
production, without turning the specification into a change log. It is a
design: the validator does not read any of it yet, and the implementation
items are listed at the end.

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
record, in a folder per kind:

| Folder | One file per | Named by |
|---|---|---|
| `records/changes/` | change request | its ID, `CR-12.yaml` |
| `records/defects/` | defect | its ID, `DEF-7.yaml` |
| `records/releases/` | release | its version, `1.4.0.yaml` |
| `records/incidents/` | incident in production | its ID, `INC-3.yaml` |
| `records/commissioning/` | commissioning run | date and environment, `2026-10-07-production.yaml` |

A record file starts with `specarchRecord: "0.1"` and `kind`, and has its
own schema, `schema/specarch-record-0.1.schema.json`, beside the two that
exist. The file name repeats the ID or version, so a reader finds a record
by walking the folders, and the validator checks the two agree.

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
| `release` | the version the fix shipped in |
| `status` | below |

| From | To | Trigger |
|---|---|---|
| reported | confirmed, not-a-defect, duplicate | triage against the specification |
| confirmed | fixed | the fix is merged and `test` passes |
| fixed | released | a release record includes it |

`not-a-defect` is the answer when the system does what the specification
says and the reporter wanted something else; that is a change request, and
the defect names it. Triage against the specification is the point of
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
release, and a later verb, `specarch diff <old> <new>`, compares them: it
lists what was added, changed and removed, classifies each by the table
above, and checks that the release's version step is at least as large as
the largest change, and that every changed element is named in the
`affects` of a change request or the `violates` of a defect the release
includes. The validator, which sees one version at a time, cannot make
either check; the diff verb can, which is why it is its own item.

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
`monitor` that caught it (or the role that reported it), a `summary`, the
`impact`, `status` (open or resolved), and links to the defects or change
requests that follow from it. A resolved incident that leads to neither
says why in `noChange`.

Rollback is already the deployment stage's `rollback`; an incident record
names whether it was used.

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
| `record_name` | a record's file name equals its `id`, or its `version` for a release, and it sits in the folder of its kind |
| `record_ref` | every role is a stakeholder of the specification; every requirement ID, `#/` pointer, test, environment, setting, monitor and check a record names resolves, or resolves to a declared change-set or defect-set |
| `change_applied` | for a change that is implemented or released, every `adds` and `changes` reference resolves and every `removes` reference does not (a removed requirement may instead stay with status retired); for one still open, `changes` and `removes` resolve, and an `adds` that already resolves is a warning |
| `change_decision` | a change that is approved, implemented or released has a `decision` with outcome approved; a rejected one has outcome rejected |
| `defect_test` | a fixed or released defect names a `test` that exists and verifies a requirement the defect violates, or is about the element it violates |
| `defect_duplicate` | a duplicate names a defect that exists and is not itself a duplicate |
| `release_contents` | a released release includes only changes that are implemented or released and defects that are fixed or released; a change or defect with status released names a release that includes it, and the other way round |
| `release_bump` | a released version is greater than the previous released version by at least the largest `impact` of what it includes, a defect counting as patch |
| `release_version` | `info.version` is the newest released version, or the version of a planned release with a pre-release tag |
| `incident_link` | a resolved incident links to a defect or a change, or says `noChange` (warning) |
| `commissioning_record` | a commissioning record's results name checks that exist, and its version is a release's |

`monitors` adds no new rule: a monitor's `environment` uses the existing
`environment` rule, its `verifies` the existing `requirement` rule, and the
traceability warning counts a monitor as verifying the requirements it
names.

## What the documentor will make

Two targets join `DocumentTarget`, after the first set of documents:

- `changes`: the change and defect register, open items first, each with
  its status, what it affects and its decision.
- `releases`: release notes, newest first: for each version, what it
  includes, grouped as added, changed, removed and fixed, from the records.

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
