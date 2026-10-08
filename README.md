# SpecArch

SpecArch is a specification language for whole systems through their whole
life cycle. One folder tree describes the stakeholders and requirements, the
data model, the API, the events, the screens, the permissions, the
algorithms, the tests, the deployment and the commissioning checks, and the
decisions behind them, each traced to the others. People read the files, and
so do AI assistants. Documents, diagrams and code are made from them.

SpecArch is personally owned and published under Apache-2.0. Anyone may use it
under that licence, including for commercial work. See `NOTICE` for the name
and for what you own in generated output.

SpecArch is built on one principle, the Low IQ Tax: every file, key and
message should cost its reader as little thinking as possible. It is set out
in `docs/principles.md`, and the rest of SpecArch follows from it.

## The format

A specification is a folder. Its root file is `specarch.yaml`, which names
the system, its version, the stages it keeps and the sources it cites. Each
stage is a folder of its name beside the root, in life-cycle order:

| Folder | Holds |
|---|---|
| `requirements/` | stakeholders, needs, requirements with acceptance criteria, glossary, assumptions, constraints |
| `design/` | enums, entities, permissions, roles, session, endpoints, commands, channels, dependencies, pages, algorithms, decisions |
| `implementation/<stack>/` | one implementation file per stack: how that stack builds the design |
| `tests/<name>/` | one folder per test: `test.yaml` with given, when and then, beside the scenario's own input and expected-output files |
| `deployment/` | environments, configuration, release and rollback steps, data migrations |
| `commissioning/` | the checks run on the installed system before handover, and the sign-off |

A file under a stage folder holds one or a few objects of one section, and
the folder names say what is inside, so a reader walks to a thing. A stage a
project has not reached yet has no folder; a tiny specification is the root
file alone with its sections inside. The validator reads the tree as one
document, so a reference is the plain name it always was, wherever the
target's file is. `docs/conventions.md` has the layout, `docs/stages.md`
what each stage holds and why.

The design side, everything but `implementation/`, is the SpecArch
Definition (SDF): neutral about language, framework and build tool. An
implementation file (SIF), `<name>.<stack>.specarch-implementation.yaml`,
names the specification and version it implements and holds the language,
toolchain, libraries with their licences, package layout, mappings, document
and code targets, build and test tasks, deployments and implementation
decisions. One specification can have several. The split follows one rule:
what a client of an interface must know is design; what only the builders or
operators of one implementation need is implementation.

Every element, at every stage, may say `why` it is the way it is, and
`cites` the standards, regulations, documents and interviews it rests on,
declared once under `sources` in the root file. Design elements name the
requirements they `satisfies`; tests and commissioning checks name the ones
they `verifies`; the validator reports every gap.

The YAML uses the keywords of existing standards wherever one exists:

- JSON Schema for fields: `type`, `properties`, `required`, `enum`, `format`,
  `minimum`, `maxLength`, `pattern`.
- OpenAPI for endpoints: `paths`, `parameters`, `requestBody`, `responses`.
  A standalone OpenAPI document is made from the specification, not kept
  beside it.
- AsyncAPI for events: `channels`, `messages`, `payload`.
- ISO/IEC/IEEE 29148 for requirements and their attributes, ISO/IEC/IEEE
  29119 for test levels, ISO/IEC/IEEE 12207 for the stages.

SpecArch adds keywords only where no standard has one: relations between
entities, permissions on operations, command-line commands, pages,
algorithms with worked examples, decision records, environments, release
steps and commissioning checks. `docs/conventions.md` lists every keyword
with its origin.

Markdown carries the explanation and rationale. Mermaid carries the diagrams.
Diagrams that show structure (entity relations, state machines, request
sequences) are generated from the YAML, so they cannot drift from it; only
explanatory pictures are drawn by hand.

SpecArch has no grammar and no parser of its own, apart from the small
expression language of checks and formulas. The language is defined by three
JSON Schema 2020-12 documents, the meta-model, in `schema/`: one for the
specification, one for implementation files and one for the records kept
beside a specification (change requests, defects, releases, incidents,
commissioning runs and approvals). The same schema gives editors
validation and completion through `yaml-language-server`: put this on the
first line of a root file and most editors pick it up.

    # yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/v0.3.0/schema/specarch-design-0.1.schema.json

## The rules SpecArch serves

1. Spec first. The specification is written and reviewed before code. Code
   is generated from it. Only algorithm bodies are written by hand, and only
   after the algorithm is specified.
2. One specification covers requirements, data, API, events, UI,
   permissions, algorithms, tests, deployment, commissioning and decisions.
3. Spec and code stay in sync, enforced in CI (`docs/sync-gates.md`). If
   code is ever written first, its spec is added in the same change and the
   merge waits until they match.
4. Algorithms are specified three ways: a formula, a worked numeric example
   and pseudocode. The worked examples become test cases.
5. Generated UI reuses the target project's component library, so it looks
   like the hand-built screens around it.
6. Existing code enters by extraction (`docs/extraction.md`). An as-built
   spec is produced from the code, checked by regenerating and comparing, and
   from then on changes go spec first.

Two design choices follow from lessons learned on an earlier in-house language.
Access control is fail-closed: an operation with no permission is a validation
error, not a public endpoint. And nothing in the language is "written but not
enforced": a constraint, a formula or a worked example that no tool checks is
a defect in the roadmap, not an accepted state.

## Repository layout

| Path | Holds |
|---|---|
| `schema/specarch-design-0.1.schema.json` | the meta-model of a specification, JSON Schema 2020-12 |
| `schema/specarch-implementation-0.1.schema.json` | the meta-model of implementation files |
| `schema/specarch-record-0.1.schema.json` | the records beside a specification |
| `schema/specarch-idiom-0.1.schema.json` | the meta-model of an idiom, shipped or a project's |
| `idioms/` | the idioms SpecArch ships, one folder per concern, embedded into the program: `type-rendering`, what each field type becomes in Go and in PostgreSQL, SQL Server, Oracle and MariaDB |
| `spec/` | SpecArch's own specification: every stage, the design of the `specarch` command, its Go and Swift implementation files, and in `spec/tests/` the conformance suite every implementation of `specarch` must pass |
| `docs/techspec.md` | SpecArch's technical specification, generated from `spec/` |
| `docs/requirements.md`, `testplan.md`, `traceability.md`, `deployment.md`, `commissioning.md`, `questions.md` | SpecArch's other documents, generated from `spec/` |
| `docs/changes.md`, `releases.md` | the change register and the release notes, generated from the records in `records/` |
| `records/` | SpecArch's own records: its releases |
| `swift/` | the Swift build of `specarch` |
| `history/` | what changed and why, one file per day |
| `docs/principles.md` | the Low IQ Tax principle and how SpecArch applies it |
| `docs/conventions.md` | the tree layout, YAML layout, Markdown sections, generated and hand-drawn diagrams |
| `docs/stages.md` | the seven life-cycle stages: what each holds, which standard says so, and why |
| `docs/maintenance.md` | after commissioning: change requests, defects, releases, incidents and operation, and the records that hold them; the release rules, the diff verb and their documents as designed |
| `docs/refinement.md` | from an old document to code: partial specifications, open questions, origin, approval and the gate on generation |
| `docs/test-generation.md` | tests from the specification: the golden paths, which red paths are written and why, structured test data, the derive verb and the tests target, as designed |
| `docs/dxlib-lessons.md` | what SpecArch takes from dxlib, the owner's Go library: one type rendered to many targets, the design keywords it proves are needed, a Go implementation on dxlib, and what is left behind |
| `docs/idioms.md` | idioms: how each recurring implementation concern is done the same way everywhere, shipped with SpecArch and overridable per project |
| `docs/authoring-layer-evaluation.md` | TypeSpec, CUE and Pkl as an optional authoring layer |
| `docs/generators.md` | the rules every document and code target follows, and the plug-in protocol |
| `docs/sync-gates.md` | the CI checks that keep a spec and its code equal |
| `docs/extraction.md` | how an existing system gets its as-built spec, and what goes wrong |
| `docs/from-sources.md` | the procedure an agent follows to write a specification from existing documents and code, with how to install and pin `specarch` |
| `docs/roadmap.md` | document targets, code targets, sync gates, first real projects, the next meta-model |
| `examples/library-lending/` | a small complete example: a specification with every stage in `spec/`, its document in `docs/` |
| `examples/lending-desk/` | a partial specification written from a desk manual and a small Go service that disagree, by `docs/from-sources.md` |

## Installing and running

`specarch` is one program with verbs named by direction: `validate`
checks, `gaps` lists the open questions and what they hold up, `document`
goes from the specification to a document, `approve` records that the
documents were read and the specification is approved, `generate` from
the approved specification to code, `extract` (designed, built later) from
existing code to a specification, and `diff` compares two versions of a
specification and checks the release between them, and `derive` writes a
draft test for every derived case no test covers, and `idioms` lists the
idioms each implementation file uses. It has two builds from
the same design.
The Go build has every verb. With Go 1.26 or later:

    go install github.com/SpecArch/specarch/cmd/specarch@v0.3.0

or, from a clone of this repository:

    go install ./cmd/specarch

Code targets are plug-ins on PATH. This repository has one,
`specarch-gen-tests-go`, which `specarch generate tests` runs to write the
Go tests of a specification; it is in no release yet, so install it from
a commit or a clone:

    go install github.com/SpecArch/specarch/cmd/specarch-gen-tests-go@<commit>

The Swift build, for macOS, has the validate and version verbs and gives
the same output. With Swift 6:

    swift build -c release --package-path swift
    swift/.build/release/specarch validate spec examples

Check specifications; a folder is searched for them, and a folder holding
`specarch.yaml` is read as one specification:

    specarch validate spec/ examples/

Every problem is one line: the file, the line, `error` or `warning`, the
YAML path in the specification, the rule, and one sentence saying what is
wrong and how to fix it.

    design/entities/order.yaml:12: error: /entities/Order/relations/customer/target: relation_target: Custmer is not an entity of the specification; did you mean Customer?

The exit status is 0 when every input is valid (warnings may be printed), 1
when any has an error, and 2 for a usage error, a path that cannot be
read, or a folder with no specification in it. `specarch version` prints the
program version and the meta-model versions it reads.

The validator checks the layout of the tree, applies the JSON Schema to the
merged specification, then checks what a schema cannot express: references
across the tree, fail-closed access, the boundary between design and
implementation, expressions and their types, worked examples evaluated to
the last digit, concrete integer types, the test scenarios each subject
needs, citations, environments and secrets, and traceability from needs
through requirements to design and tests. `docs/conventions.md` describes
every rule; the rule names are the `Rule` enum in `spec/design/enums/`.

## Making the documents

    specarch document techspec examples/library-lending/

writes `techspec.md`, an arc42 technical specification with Mermaid diagrams
of the entities, states, operations, commands, pages and permissions, the
deployment and commissioning stages, the requirements and the traceability
matrix, into the folder the implementation file names (or `--out`). It also
refreshes the diagrams between `specarch:generate` markers in the
hand-written `specarch.md` beside the root file. It makes nothing from a
specification with errors. With `--check` it writes nothing and exits 1 when
the committed output differs, which is how CI keeps the documents current.
`docs/techspec.md` is SpecArch's own, made from `spec/`.

The other document targets work the same way: `requirements` (the
requirements specification), `testplan` (the test plan and test cases),
`traceability` (the matrix and its gaps), `deployment` (the deployment
guide), `commissioning` (the commissioning procedure and sign-off sheet)
and `questions` (the open questions and what they hold up), each as
`<target>.md`. Wherever the specification says why an element is so, or
cites a standard for it, the document shows an Insight or a Note next to
the element; wherever it says how an element is known, an Origin line; and
wherever an open question blocks an element, the question.

## From an old document to code

A specification built from what exists, a prose document or running code,
says what its sources support and asks about the rest: every element
carries its `origin` (stated, with a citation; inferred, with a reason; or
decided), and what is not known is an open question that names who decides
and what it blocks. `specarch gaps` lists the questions by stage and says
which documents and code targets are ready, drafts or waiting. The owner
answers with decisions and material, the questions shrink, the documents
are regenerated and read, and `specarch approve --by <stakeholder>` records
the approval with a digest of the files. `specarch generate` runs only
then. `docs/refinement.md` is the design.

Code targets (OpenAPI, SQL, UI, tests) are plug-ins: `specarch generate
<target>` runs `specarch-gen-<target>` from PATH, hands it the validated and
approved specification on its standard input, and writes the files it
answers with. `docs/generators.md` has the protocol.

## Status

Version 0.1 of the meta-model, October 2026. The two schemas, the tree
layout and the seven stages, one example, SpecArch's own specification, the
validator in Go and in Swift, seven documents, open questions with the
gaps report, and the approval gate on generation exist; the manual, the
operations guide and the code targets are on the roadmap. The meta-model will change: the first real
projects written in SpecArch are expected to find concepts it cannot
express, and those gaps define the next version.

## Licence

Apache License 2.0. See `LICENSE` and `NOTICE`.
