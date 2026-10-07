# The life-cycle stages

What each stage of a specification holds, which standard says so, and why
SpecArch takes it from there. The stages are the technical processes of
ISO/IEC/IEEE 12207:2017 (clause 6.4), in the order a project meets them.
Nothing is mandatory except what a stage the specification keeps needs: a
project starts with requirements alone and adds a stage when it reaches it.

## Requirements

Sections: `stakeholders`, `needs`, `requirements`, `glossary`,
`assumptions`, `constraints`. Standard: ISO/IEC/IEEE 29148:2018.

- A stakeholder is a role, never a person: who they are and what they need
  from the system or fear about it (29148, 5.2.2).
- A need is what a stakeholder says before anyone has shaped it into a
  requirement (29148, 5.2.3 and 6.3). It names its stakeholders and its
  sources, usually an interview or a document. A need that no requirement
  refines is reported, because that is the stage's unfinished work; a need
  with status rejected is not, since it will not be met.
- A requirement is one verifiable sentence: who or what shall do what,
  under which condition (29148, 5.2.4 and 5.2.5), with the attributes the
  standard asks for (5.2.8): its kind (functional, quality, interface or
  constraint, a short form of the categories in 9.5), its priority (must,
  should or could, the MoSCoW ranking from DSDM, because three levels are
  what a release decision needs), its status (proposed, accepted, rejected,
  retired), its acceptance criteria, the way it is verified (inspection,
  analysis, demonstration or test, the four methods of MIL-STD-961E and the
  INCOSE handbook), and the needs it refines. A requirement without
  acceptance criteria is reported: nothing can show it is met.
- The glossary (29148, 9.2.3) defines the domain's terms once.
- Assumptions (29148, 9.5.19) are what is taken to be true without proof
  and would change the design if it turned out false. Constraints (29148,
  9.6.16; arc42 section 2) are the limits on the solution: technical,
  organisational or legal.

Requirement IDs follow the usual form, an upper-case prefix and a number,
because that is how every tracker and every reviewer refers to them. A
requirement kept in an external tracker is reached the same way: the
tracker is declared under `sources` with kind `requirement-set` and its
prefix.

## Design

Sections: `enums`, `entities`, `permissions`, `roles`, `paths`, `commands`,
`channels`, `pages`, `algorithms`, `decisions`. Standards: JSON Schema,
OpenAPI, AsyncAPI, the common decision record of Michael Nygard, and the
algorithm viewpoint of IEEE 1016, made testable with worked examples.

Every design element names the requirements it satisfies. The word is the
satisfy relation of requirements engineering and SysML, so that the two
columns of the traceability matrix, satisfied by and verified by, say
themselves. Once a design exists, a requirement nothing satisfies is
reported.

## Implementation

One file per stack under `implementation/<stack>/`: the language and
toolchain, the libraries with their licences, the layout, how each design
object maps onto the stack, the document and code targets with their
folders, the build and test tasks with the unit and integration suites, the
real deployments with hosts and the values of non-secret settings, and the
decisions that depend on the stack. Design never goes here, and the schema
has no keyword for it. See "Design and implementation" in
`docs/conventions.md`.

## Tests

One folder per test under `tests/`, holding `test.yaml` and the scenario's
own data files. Standard: ISO/IEC/IEEE 29119. A test is about one subject,
is golden (the path that succeeds) or red (a path that is refused), has a
level, system or acceptance (the levels of 29119-1; unit and integration
belong to the implementation's suites), and says what happens in plain
given, when and then sentences. A test names the requirements it verifies.
The validator derives the cases each subject needs and reports every one no
test covers. A test's data files are never read by the validator: they are
the test's own, in whatever form the generated test code wants, and the
folder is the test case specification of 29119-3 (8.3) kept beside its
data.

Golden and red stay a mark inside each test rather than two folders. A test
moved between folders could then disagree with its content, and tests group
by subject, not by outcome.

## Deployment

Sections: `environments`, `configuration`, `release`, `rollback`,
`migrations`. Standard: the transition process of ISO/IEC/IEEE 12207:2017
(6.4.10).

- An environment says what it is for and which environment a release goes
  to next, so the promotion path is written once. Hosts, URLs and ports are
  implementation; they go in the implementation file's deployments, each of
  which names its environment.
- A setting is named once, with its type and whether it is a secret. A
  secret's value is never written in a specification, as a default or in a
  deployment; the setting's description says where the value comes from.
  The validator refuses a value for a secret, because a specification is
  read and copied by more people than a secret store.
- Release and rollback are ordered steps, each with what is done and how
  the person running it knows it worked. The exact commands are
  implementation.
- A migration is a data change a release carries, with its steps and how it
  is reversed; one that cannot be reversed says so.

## Commissioning

Sections: `checks`, `signoff`. Standards: the validation process of
ISO/IEC/IEEE 12207:2017 (6.4.11), and the site acceptance test of IEC
62381:2024, which is the industrial name for running the installed system
in its real environment before it is accepted.

A check has a kind (smoke, end-to-end, performance, security or data), the
environment it runs in, its steps, and the requirements it verifies. The
sign-off states the acceptance criteria and the roles that sign.

The results of a commissioning run are records, not design. A result is a
fact about one run; a design file that changed on every run would be a
change log, which the Low IQ Tax principle forbids. So each run is one file
outside the specification, under `records/commissioning/` beside the
specification's folder, as `docs/conventions.md` describes. The
commissioning procedure that `specarch document commissioning` writes is
the form a run fills in; reading the filled-in records belongs to the
records design in `docs/maintenance.md`.

## Operation

Section: `monitors`. Standard: the operation process of ISO/IEC/IEEE
12207:2017 (6.4.12).

A monitor says what is measured on the live system, in which environment,
the objective it must meet as one verifiable sentence, and the requirements
it verifies. A monitor is a promise about the live system, the way a
requirement is, so it belongs in the specification; an incident that
breaks one is a record, as `docs/maintenance.md` designs. The tool that
measures, how, and where an alert goes are implementation: each deployment
in the implementation file names them per monitor, and the validator
refuses a deployment that watches a monitor the specification does not
declare (`monitor`). A monitor counts as verifying the requirements it
names, like a test or a check.

## Traceability

The links between the stages are plain lists of IDs: a requirement's
`needs`, an element's `satisfies`, a test's or check's `verifies`. The
validator checks every one resolves and reports, as warnings, a need no
requirement refines, a requirement without acceptance criteria, a
requirement nothing satisfies once there is a design, and a requirement
nothing verifies once there are tests, checks or monitors. The techspec's chapter 13
is the matrix. Warnings rather than errors, because a specification is
written in order and the gaps are its to-do list.
