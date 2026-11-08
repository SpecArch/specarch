# SpecArch

**One specification as the single source of truth for a system's whole
life cycle.**

SpecArch is a specification language for whole systems through their whole
life cycle, and the `specarch` program that reads it, checks it and makes
documents, diagrams and code from it. One folder tree describes the
stakeholders and requirements, the data model, the API, the events, the
screens, the permissions, the algorithms, the tests, the deployment and the
commissioning checks, and the decisions behind them, each traced to the
others. People read the files, and so do AI assistants. Gaps, sources that
disagree and stakeholders nobody has heard are reported before code is
written. Its goal is to make each human decision in a specification cheap to
take, cheap to review and traceable to the decisions it rests on.

SpecArch is personally owned and published under Apache-2.0. Anyone may use it
under that licence, including for commercial work. See `NOTICE` for the name
and for what you own in generated output.

SpecArch is built on three principles, set out in `docs/principles.md`; the
rest of SpecArch follows from them. It is a compiler for specifications: it
reads an incomplete or wrong definition rather than refusing it, and every
problem is reported at the place the author fixes it, with the entry it
touches marked in what SpecArch writes.
It follows the Low IQ Tax: every file, key and message should cost its
reader as little thinking as possible. And a change carries its reason at
the entry it changes, so a review reads decisions, not lines of
difference.

## Why SpecArch

In most projects the requirements live in documents, the design in slides,
the code in repositories and the tests in spreadsheets. When something
changes they drift apart, and the difference shows late: at the user
acceptance test, as a bug, or as a delay.

SpecArch treats the specification of a system like source code:

- **A draft is read, never refused.** Start on the first day with what is
  known. SpecArch reports what is missing and where sources disagree as open
  questions, each with a priority (`must`, `should` or `could`) and the
  stakeholder role that decides it.
- **Built on existing standards.** JSON Schema for data, OpenAPI 3.1 for
  endpoints, AsyncAPI for events, BPMN 2.0 for workflows, and ISO/IEC/IEEE
  29148 for requirements, 29119 for testing and 12207 for the life cycle.
- **Fail-closed and traced.** Design elements name the requirements they
  satisfy, and tests the requirements they verify; every gap is reported.
  An operation with no permission is a validation error, not a public
  endpoint.

## SpecArch across the life cycle

### 1. Discovery and extraction: `specarch extract`, `specarch merge`

A specification is written from scratch, or read out of what exists:

- **Documents and data.** `extract documents` reads Markdown, PDF, Word,
  PowerPoint, Excel and email files; `extract tables` reads CSV, TSV and
  Excel registers into entities; `extract workflows` reads BPMN 2.0 files
  and workflow tables; `extract tests` reads Gherkin feature files and test
  sheets. What a stakeholder said in an interview, a meeting, a chat, an
  email or a call is kept with `specarch log` and cited by the decisions it
  supports.
- **Existing code, as an as-built specification.** Readers for Go, Swift,
  JavaScript and TypeScript (Vue included) and Dart, and for database
  schemas, routers, OpenAPI documents, permission tables, pages, gateway
  tables and systemd units. `specarch merge` joins what each reader wrote
  into one specification, and `specarch check` compares a specification with
  dumps of its code.

### 2. Validation, gaps and stakeholders: `specarch validate`, `specarch gaps`

Flaws in the requirements and the design show in the first week, not the
twelfth month:

- **Sources that disagree.** When the code and a document say different
  things, both are kept with their citations and an open question asks
  which holds. In `examples/lending-desk/` the desk manual says a loan lasts
  21 days and the Go service (`lending/model.go:8`) says 14.
- **Unheard stakeholders.** The `unheard` warning names every role an
  accepted requirement or a page needs that no communication, session,
  interview or decision shows was ever heard, so the risk of that role
  objecting first at acceptance shows early.
- **Open questions.** `specarch gaps` lists every question by priority with
  what it holds up and which documents and code targets are ready, drafts or
  waiting. A specification with an open `must` or `should` question cannot be
  approved.

### 3. Decisions and approval: `specarch decide`, `specarch approve`, `specarch decision`

- **Decision records.** A decision records why a technology or a design was
  chosen and which simpler options were set aside. `specarch decide` answers
  an open question with one; `specarch decision why` prints the chain of
  decisions it rests on, and `specarch decision rests` the decisions that
  rest on it.
- **Complexity that needs a reason.** The `technology_unjustified` warning
  marks a framework an implementation file brings in that no element of the
  design needs, no idiom approves and no decision justifies.
- **Sign-off by role.** `specarch approve --by <role>` records an approval
  only from a known stakeholder that the records show was heard, only with no
  errors and no open `must` or `should` question, and only when the documents
  are current.

### 4. Planning the work: `specarch plan`

A work package is a record under `records/work-packages/`: the accepted
requirements of one release that one team builds together, with the
packages it comes `after`, who it is `assignedTo`, its `status`
(proposed, planned, ready, in-progress, done or withdrawn) and its
`effort`. `examples/library-lending/` keeps three.

- **Propose.** `specarch plan --release 0.2.0 <folder>` writes a package for
  each group of accepted requirements of that release that no package holds
  yet, in the order of the reference graph, so what an element needs is
  built before it.
- **Take the next one.** `specarch plan --next <folder>` prints the first
  package that can start now, or what each one waits on; `--brief WP-2`
  prints one package's brief for whoever takes it.
- **Round-trip with a tracker.** `--export csv` or `--export json` prints
  every package for a tracker; `--import <file>` reads the same file back
  and writes the status, assignee and effort the tracker changed.
- **Checked by `specarch validate`.** `work_package_cycle` (an error) marks
  packages whose `after` entries come back to themselves. The rest are
  warnings: `work_package_order` (a package built before what it needs),
  `work_package_overlap` (a requirement held by two packages),
  `work_package_release` (a requirement of another release),
  `work_package_not_ready` (a package marked ready whose requirements are not
  accepted, have no acceptance criteria or wait on an open question),
  `work_package_unverified` (a package marked done whose tests have no passed
  result in the test-run records) and `work_package_unconfigured` (a setting
  the package needs that no deployment gives a value).

### 5. Code, data and deployment: `specarch generate`

`specarch generate <target>` writes the repetitive layers from the approved
specification, through sixteen plug-ins:

- **Backend and API.** A Go service on
  [dxlib](https://github.com/donnyhardyanto/dxlib) (`go-dxlib`) and the
  OpenAPI 3.1 document (`openapi`).
- **Screens.** The `ui` target writes the web in plain JavaScript, TypeScript
  on Next.js and Carbon for a TypeScript implementation file, the web with no
  build step on Vue and htm, or SwiftUI for the iPhone. With `--unapproved`
  it writes draft screens from a specification not approved yet, every file
  marked a draft, to show users early.
- **Databases.** Migrations (`sql`) and a development seed (`seed`) for
  PostgreSQL, SQL Server, Oracle and MariaDB; sqlc queries for PostgreSQL
  (`sqlc`); guarded scripts for data changes on PostgreSQL and MariaDB
  (`scripts`); a register template per entity (`register`).
- **Workflows.** Each workflow as a BPMN 2.0 file with its diagram (`bpmn`).
- **Deployment.** Compose files, Kubernetes manifests or a cloud-init file per
  host, the release, rollback and migration runbooks, and a script that builds
  a fresh Compose environment, tests it and tears it down (`deploy`).
- **Tests.** Go, Swift and Dart tests (`tests`), one per design test and per
  worked example of an algorithm.
- **Drift check.** `specarch generate --check` writes nothing and exits 1 when
  a generated file on disk differs; run it in CI.

The documents (the technical specification, requirements, test plan,
traceability matrix, commissioning procedure, release notes and the rest)
come from `specarch document`, below.

### 6. Tests and regression: `specarch derive`, `specarch run`, `specarch log test-run`

- **Derived tests.** `specarch derive` writes a draft test for every subject
  with no golden (success) test and for every derived case no test covers,
  red paths included: denied, not found, a missing field, a value past its
  limit.
- **A test loop with the customer.** `specarch generate tests --package
  <environment>` writes a test package for an environment, `specarch run`
  runs it and writes JUnit XML, and `specarch log test-run` records the
  results, each test case mapped back to its test and from there to the
  elements and requirements.

### 7. Change and comparison: `specarch diff`, `specarch compare`

- **What a change touches.** `specarch diff` lists what changed between two
  versions and checks the release between them. A change request records
  why the result was not what was wanted: `omitted`, `wrong`, `changed` or
  `unheard`.
- **Measured technology choices.** The design stays neutral about technology
  (the SpecArch Definition, SDF), and each stack is an implementation file
  (SIF). `specarch compare` puts several implementation files of one
  specification in one table, from their test results, scans and
  measurements, so competing choices are measured against the same tests.

## Seeing the specification

A specification is structured data, so what shows it is made from it and
does not drift from it:

- `specarch document html` writes the whole specification as one page to
  open from the disk: the overview, every element under its own anchor, the
  entity diagram, the workflows, the generated screens, the traceability
  matrix, the open questions and the problems, linked both ways with the
  entries they mark.
- `specarch document techspec` draws Mermaid diagrams from the YAML: entity
  relations, state machines, request sequences and flows. Each workflow is
  also written as BPMN 2.0.
- `specarch model` prints the specifications as one JSON document for an
  editor: the merged document, where every node came from, the references
  and the layout problems. An editor can build live views on it, such as
  readiness and open questions by role, the traceability matrix with the
  latest test results, and the elements a change to one element reaches.
- The JSON Schema of the format gives an editor validation and completion
  (`yaml-language-server`, below).
- `specarch generate ui --unapproved` gives clickable draft screens before the
  backend exists.

## Quick start

Install `specarch` and the plug-ins (below), then:

```sh
# Start a specification in a new folder
specarch init --title "Lending desk" spec/

# Check it, and list the open questions and what they hold up
specarch validate spec/
specarch gaps spec/

# Write a draft test for every case no test covers
specarch derive spec/

# Answer an open question with a decision record, citing where the answer came from
specarch decide Q-1 spec/ --by <role> --answer "<answer>" --source <source>

# Write draft screens from the specification before it is approved
specarch generate ui --unapproved spec/

# Write the documents the stakeholders read, then record their approval
specarch document techspec spec/
specarch approve --by <role> --session <session> spec/

# Generate from the approved specification
specarch generate openapi spec/
specarch generate sql spec/
specarch generate go-dxlib spec/
specarch generate deploy spec/
```

`specarch guide` prints the whole order of work, and `specarch help start`
a first entity and operation that validate. Generating needs an
implementation file that names the targets (`specarch help generate`), and
approval needs the documents the implementation file configures to be
current.

## The format

A specification is a folder. Its root file is `specarch.yaml`, which names
the system, its version, the stages it keeps and the sources it cites. Each
stage is a folder of its name beside the root, in life-cycle order:

| Folder | Holds |
|---|---|
| `requirements/` | stakeholders, the confirmation that their list is complete, needs, requirements with acceptance criteria, glossary, assumptions, constraints |
| `design/` | enums, entities, permissions, roles, session, endpoints, fixed routes, commands, channels, dependencies, jobs, processes, workflows, problem types, pages, menus, algorithms, decisions |
| `implementation/<stack>/` | one implementation file per stack: how that stack builds the design |
| `tests/<name>/` | one folder per test: `test.yaml` with given, when and then, beside the scenario's own input and expected-output files |
| `deployment/` | environments and who runs each, configuration, release and rollback steps, data migrations, and the handover to a customer who runs an environment |
| `commissioning/` | the checks run on the installed system before handover, and the sign-off |
| `operation/` | monitors of the live system, backup and restore |

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
commissioning runs, approvals, ideas, and who said what and when:
communications, sessions, test sessions and deploys). The same schema gives editors
validation and completion through `yaml-language-server`: put this on the
first line of a root file and most editors pick it up.

    # yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/v1.6.0/schema/specarch-design-1.0.schema.json

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
| `schema/specarch-design-1.0.schema.json` | the meta-model of a specification, JSON Schema 2020-12, version 1.0 |
| `schema/specarch-design-0.3.schema.json` | the meta-model of a specification on 0.3, 0.2 or 0.1, read beside 1.0 |
| `schema/specarch-implementation-1.0.schema.json` | the meta-model of implementation files; `-0.1` beside it for a file on 0.1 |
| `schema/specarch-record-1.0.schema.json` | the records beside a specification; `-0.1` beside it |
| `schema/specarch-idiom-1.0.schema.json` | the meta-model of an idiom, shipped or a project's; `-0.1` beside it |
| `schema/specarch-estate-1.0.schema.json` | an estate: the modules' specifications it lists and pins (`docs/conventions.md`, Estates); `-0.1` beside it |
| `schema/specarch-fragment-*-1.0.schema.json` | one schema per kind of file under a stage folder, written from the design schema by `tools/fragment-schemas` (`docs/conventions.md`, The tree); `-0.3` beside them |
| `idioms/` | the idioms SpecArch ships, one folder per concern, embedded into the program: seventeen, each a stack-neutral contract and its Go rendering, such as `type-rendering` (what each field type becomes in Go and in PostgreSQL, SQL Server, Oracle and MariaDB), `paginated-list`, `request-validation` and `error-response`; `ui-components`, what draws each page and field on Next.js with Carbon; and `design-tokens`, which Carbon token takes each colour of the theme |
| `spec/` | SpecArch's own specification: every stage, the design of the `specarch` command, its Go and Swift implementation files, and in `spec/tests/` the conformance suite every implementation of `specarch` must pass |
| `docs/techspec.md` | SpecArch's technical specification, generated from `spec/` |
| `docs/requirements.md`, `testplan.md`, `traceability.md`, `deployment.md`, `commissioning.md`, `questions.md` | SpecArch's other documents, generated from `spec/` |
| `docs/problems.txt`, `problems.sarif` | every error, warning and open question of `spec/`, one compiler line each and as a SARIF 2.1.0 log |
| `docs/changes.md`, `releases.md` | the change register and the release notes, generated from the records in `records/` |
| `docs/communications.md`, `timeline.md` | the communication history and the timeline of every event, generated from the records in `records/` where a specification keeps them (`examples/library-lending/docs/`) |
| `docs/progress.md` | the progress report: the size by stage, the open questions by priority and deciding role, the unheard roles, the stakeholder list's confirmation, the effort each change request records with the missing ones named, and a burn-up per release, generated from the specification and its records (`examples/library-lending/docs/`) |
| `records/` | SpecArch's own records: its releases |
| `swift/` | the Swift build of `specarch` |
| `third_party/typescript-go/` | the TypeScript compiler's Go source that `specarch facts javascript` reads with, copied at a pinned release by `tools/typescript-go/pull.sh`, with its own LICENSE and NOTICE.txt |
| `readers/` | the Swift and Dart readers of `specarch facts`, with the lock files that pin them, embedded into the program |
| `history/` | what changed and why, one file per day |
| `AGENTS.md` | instructions for agents working in this repository, with the guide `specarch guide --agent` prints |
| `skills/specarch/` | the agent skill `specarch skill` writes, in the Agent Skills format, kept equal to the program's by a test |
| `docs/help/` | the help topics built into the program, which `specarch help <topic>` prints |
| `docs/guide-by-situation.md` | the guide by situation: which feature to use in which real situation, each with when it happens, what to run, a worked example with the program's output, what to watch for and the deeper document; `specarch help situations` is its index, and a test keeps every feature of the program in it |
| `docs/id/guide-by-situation.md` | the same guide in Indonesian, under the same situation ids, kept equal by the same test |
| `BUG_OUTSTANDING.md`, `BUG_HISTORY.md` | the bugs, one BUG-NNN entry each: the open ones, and the fixed ones with the commit that fixed them; the repository has no GitHub issues (`CONTRIBUTING.md`, Bugs) |
| `docs/principles.md` | the three principles, a compiler for specifications, the Low IQ Tax and a change that carries its reason, and how SpecArch applies them |
| `docs/conventions.md` | the tree layout, YAML layout, Markdown sections, generated and hand-drawn diagrams |
| `docs/stages.md` | the seven life-cycle stages: what each holds, which standard says so, and why |
| `docs/maintenance.md` | after commissioning: change requests, defects, releases, incidents and operation, and the records that hold them; the release rules, the diff verb and their documents as designed |
| `docs/diagnostics.md` | the problems file, its SARIF form and the marks at each entry in every output, with the steps that build them |
| `docs/refinement.md` | from an old document to code: partial specifications, open questions, asking a person one question at a time, origin, approval and the gate on generation |
| `docs/test-generation.md` | tests from the specification: the golden paths, which red paths are written and why, structured test data, the derive verb and the tests target, as designed |
| `docs/dxlib-lessons.md` | what SpecArch takes from dxlib, the owner's Go library: one type rendered to many targets, the design keywords it proves are needed, a Go implementation on dxlib, and what is left behind |
| `docs/idioms.md` | idioms: how each recurring implementation concern is done the same way everywhere, shipped with SpecArch and overridable per project |
| `docs/ui-design.md` | what an analyst writes about a user interface: page states, events and screen flows, sections, compact screens, accessibility to WCAG 2.2 and a theme of design tokens, and the first UI generator |
| `docs/ui-nextjs-carbon.md` | the second web generator, TypeScript on Next.js and Carbon, with a project's own components as an idiom override: its design, the libraries checked, and the steps that build it |
| `docs/ui-swiftui.md` | the third UI generator, SwiftUI for the iPhone: the same pages as data in a Swift package, read by the one analysis both generators share |
| `docs/ui-vue-htm.md` | the fourth UI generator, the web with no build step on Vue's runtime-only build and htm: the menus, pages and actions as data files, one api.call, a vendor manifest with each file's integrity, and a table a project replaces through the idiom |
| `docs/authoring-layer-evaluation.md` | TypeSpec, CUE and Pkl as an optional authoring layer |
| `docs/generators.md` | the rules every document and code target follows, and the plug-in protocol |
| `docs/sync-gates.md` | the CI checks that keep a spec and its code equal |
| `docs/extraction.md` | how an existing system gets its as-built spec, and what goes wrong |
| `docs/reading-code.md` | what extract reads from source code per language and framework, and what stays with a printer |
| `docs/from-sources.md` | the procedure an agent follows to write a specification from existing documents and code, with how to install and pin `specarch` |
| `docs/roadmap.md` | document targets, code targets, sync gates, first real projects, the next meta-model |
| `docs/meta-model-0.2.md` | meta-model 0.2: separation of duties, workflows, task pages and page elements, value objects, missing tests as errors, and the steps that build them |
| `docs/meta-model-0.3.md` | meta-model 0.3: the caller as a record an expression can name, the rows a role reaches, machine clients and the gates that are not a permission, token formats, and the steps that build them |
| `examples/library-lending/` | a small complete example: a specification with every stage in `spec/`, its document in `docs/` |
| `examples/lending-desk/` | a partial specification written from a desk manual and a small Go service that disagree, by `docs/from-sources.md`, and `extract.sh`, which reads the same sources with every reader and merges them |
| `examples/estate/` | an estate of three modules, the lending desk above, a small catalogue service it depends on, and a gateway in front of both that forwards by the access token's audience: `specarch-estate.yaml` lists and pins them, and `docs/estate.md` is their module map |
| `examples/notice-board/` | a small Go service on dxlib, read by `extract openapi` from the OpenAPI document it prints and by `extract go` from its source, and `extract.sh`, which reads both and merges them |
| `examples/reading-list/` | an iPhone app on SwiftUI and SwiftData and the Vapor server it calls, with the code-facts dumps of their Swift source and `extract.sh`, which reads both and merges them |
| `examples/release-watcher/` | a daemon and a nightly program under systemd: their unit files, the specification of the two processes with every test they imply, and its technical specification and operations guide |
| `examples/room-booking/` | one Express service written in TypeScript and in plain JavaScript, with the code-facts dumps of both and `extract.sh`, which reads both and lists the questions only one of them asks |

## Installing and running

### Install

SpecArch has two builds from the same design.

**The Go build** has every command. With Go 1.27.3 or later:

    go install github.com/SpecArch/specarch/cmd/specarch@v1.7.0

or, from a clone of this repository:

    go install ./cmd/specarch

**The Swift build** (macOS) has `validate`, `model`, `upgrade`, `try` and `version`, with the same output. With
Swift 6:

    swift build -c release --package-path swift
    swift/.build/release/specarch validate spec examples

**Code generators** are separate programs on your `PATH`; `specarch generate <target>` runs the one for that
target. Install them at the same tag as `specarch`:

    for g in openapi sql seed sqlc scripts tests-go tests-swift tests-dart go-dxlib \
             ui ui-typescript ui-vue-htm ui-swiftui bpmn register deploy; do
      go install github.com/SpecArch/specarch/cmd/specarch-gen-$g@v1.7.0
    done

| Generator | Run by | Writes |
|---|---|---|
| `specarch-gen-openapi` | `generate openapi` | the OpenAPI 3.1 document of the specification |
| `specarch-gen-sql` | `generate sql` | database migrations for PostgreSQL, SQL Server, Oracle or MariaDB |
| `specarch-gen-seed` | `generate seed` | a development seed for those tables: made-up rows from the entities, the same on every run |
| `specarch-gen-sqlc` | `generate sqlc` | the sqlc queries the design derives, against those migrations |
| `specarch-gen-scripts` | `generate scripts` | a guarded script for each data change the design guards (psql or the mariadb client) |
| `specarch-gen-tests-go` | `generate tests` | Go tests, one per design test and per worked example of an algorithm |
| `specarch-gen-tests-swift` | `generate tests`, for a Swift implementation file | Swift Testing tests |
| `specarch-gen-tests-dart` | `generate tests`, for a Dart implementation file | `package:test` or `flutter_test` tests (Flutter apps too) |
| `specarch-gen-go-dxlib` | `generate go-dxlib` | the tables, handlers, seeds and tasks of a Go service on dxlib |
| `specarch-gen-ui` | `generate ui` | web pages in plain JavaScript: list, view, form, task and content pages, the routes of custom pages, the sign-in redirect |
| `specarch-gen-ui-typescript` | `generate ui`, for a TypeScript implementation file | the same screens on Next.js and Carbon |
| `specarch-gen-ui-vue-htm` | `generate ui`, framework `vue-htm` | the same screens with no build step (Vue runtime-only build and htm), menus, pages and actions as data files, a vendor manifest |
| `specarch-gen-ui-swiftui` | `generate ui`, platform `iphone` | the same screens in SwiftUI, as a Swift package |
| `specarch-gen-bpmn` | `generate bpmn` | each workflow as a BPMN 2.0 file with its diagram, and the diagram as SVG |
| `specarch-gen-register` | `generate register` | each entity's register template |
| `specarch-gen-deploy` | `generate deploy` | each deployment as Compose, Kubernetes manifests or cloud-init per host; release, rollback and migration runbooks; a fresh-environment script that builds, loads, tests and tears down a Compose deployment. Secrets are referenced, never written |

`generate tests --package <environment>` is not a generator of its own: the program writes the test package itself.

### Commands

Commands are named by direction: from the specification to something (`document`, `generate`), or from something
to the specification (`extract`, `decide`, `log`). `specarch help <command>` explains each one, with every option.
Every example below runs from the root of this repository, on its examples, in the order of the tables: the
`extract` example writes the trees `merge` joins, and `merge` the specification `diff` compares. A test runs them,
all but those that need the generators, a server, an agent's client or the repository's history, and fails when a
command of the usage has no row here.

**Write and check a specification**

| Command | Does | Example |
|---|---|---|
| `init` | writes the smallest specification that validates | `specarch init --title "Lending desk" --profile lean notes/` |
| `validate` | checks specifications and implementation files | `specarch validate examples/library-lending/spec` |
| `gaps` | lists the open questions and what each holds up; `--next` only the one to ask next | `specarch gaps --next examples/lending-desk/spec` |
| `upgrade` | moves a specification to the newest format | `specarch upgrade --dry-run examples/library-lending/spec` |

**Decide and approve**

| Command | Does | Example |
|---|---|---|
| `decide` | answers an open question with a decision record and takes the question out | `specarch decide Q-3 --by desk-staff --answer "One test per requirement" --source desk-manual examples/lending-desk/spec` |
| `decision why` | prints the chain of decisions a decision rests on, earliest first | `specarch decision why ADR-004 examples/library-lending/spec` |
| `decision rests` | prints the decisions that rest on a decision, nearest first | `specarch decision rests ADR-001 examples/library-lending/spec` |
| `log` | records what a stakeholder said, as a communication record; an option not given comes back as the question to ask | `specarch log --dry-run --direction in --from member --to librarian --at 2026-10-09T10:15:00Z --channel call --type request --words "May I keep the book a week longer?" --form summary --recorded-by librarian examples/library-lending/spec` |
| `approve` | records that the documents were read and the specification is approved, in the session `--session` names | `specarch approve --by library-board --session SES-1 examples/library-lending/spec` |

**Produce from the specification**

| Command | Does | Example |
|---|---|---|
| `document <target>` | writes a document for people to read (the targets below) | `specarch document techspec examples/library-lending/spec` |
| `generate <target>` | writes code or data from the approved specification (the generators above); `--unapproved` writes drafts before the approval, `--check` fails when a file on disk differs | `specarch generate openapi --out openapi examples/library-lending/spec` |
| `model` | prints the specifications as one JSON document for a program such as an editor: the merged document, where each node came from, the references, the layout problems | `specarch model examples/library-lending/spec` |
| `try` | evaluates an algorithm on its worked examples or given inputs, printing each step of the formula with its value; `--file` reads a proposed algorithm instead | `specarch try lateFee --example "one week late" examples/library-lending/spec` |
| `plan` | proposes and orders the work packages of a release; `--next`, `--brief <package>`, `--export csv\|json` and `--import <file>` take them from there (life cycle, step 4) | `specarch plan --brief WP-2 examples/library-lending/spec` |

`try` also takes inputs of its own: `specarch try lateFee --inputs "{daysLate: 7, dailyRate: '0.50', replacementCost:
'25.00'}" examples/library-lending/spec`.

The document targets, each written into the folder the implementation file names, or `--out`:

| Target | Writes |
|---|---|
| `intent` | `intent.md`: one page on the problem, the goals, what is out of scope and the quality goals |
| `techspec` | `techspec.md`: an arc42 technical specification with Mermaid diagrams of the entities, states, operations, commands, pages and permissions, the deployment and commissioning stages, the requirements and the traceability matrix |
| `requirements` | `requirements.md`: the requirements specification |
| `testplan` | `testplan.md`: the test plan and the test cases |
| `traceability` | `traceability.md`: the traceability matrix and its gaps |
| `deployment` | `deployment.md`: the deployment guide |
| `commissioning` | `commissioning.md`: the commissioning procedure and the sign-off sheet |
| `handover` | `handover.md`: what a customer who builds and runs an environment from the source receives |
| `questions` | `questions.md`: the open questions and what they hold up |
| `problems` | `problems.txt` and `problems.sarif`: every error, warning and open question, and a mark at each entry in the YAML |
| `comparison` | `comparison.md`: the old documents against the refined specification: kept, clarified, added or corrected, and dropped |
| `changes` | `changes.md`: the change and defect register, from the records |
| `releases` | `releases.md`: the release notes, from the records |
| `communications` | `communications.md`: who said what to whom and when, from the records |
| `timeline` | `timeline.md`: every event in one time order, from the records |
| `testruns` | `testruns.md`: the test runs and their failure trend, from the records |
| `progress` | `progress.md`: the progress report: size by stage, open questions, unheard roles, effort and a burn-up per release |
| `html` | `index.html`: the whole specification as one page to open from the disk |
| `manual` | `manual.md`: the user manual |
| `operations` | `operations.md`: the operations guide |

**Start from existing code or documents**

| Command | Does | Example |
|---|---|---|
| `extract <source>` | writes a specification from one surface of existing code or documents (`docs/extraction.md` lists the readers) | `specarch extract documents --out trees/manual examples/lending-desk/sources/manual.md` and `specarch extract openapi --out trees/openapi examples/lending-desk/sources/openapi/openapi.yaml` |
| `merge` | joins the trees `extract` writes into one specification | `specarch merge --out trees/merged trees/manual trees/openapi` |
| `facts <language>` | writes the code-facts dump `extract` reads, for a committed folder of JavaScript/TypeScript/Vue (`javascript`), Swift (`swift`) or Dart (`dart`) | `specarch facts javascript examples/room-booking/sources/ts ts.json` |
| `check` | compares a specification with the route tables, permission tables, catalogue dumps and gateway tables of its code; fails on a difference its allowlist does not name, on an allowlist line that names none, and on one added since the revision `--since` names | `specarch check --allowlist examples/lending-desk/check-allowlist.txt examples/lending-desk/spec examples/lending-desk/sources/routes/routes.json` |

`extract` reads: outline, database, router, documents (Markdown, PDF, Word, PowerPoint, Excel, email), tables (CSV,
TSV, Excel registers), openapi, permissions, pages, workflows (BPMN 2.0, or a workflow table with `--table`),
gateway, systemd and tests (Gherkin feature files and test sheets), plus Go (`go`), Swift (`swift`), JavaScript
(`javascript`) and Dart (`dart`) code. The JavaScript/TypeScript/Vue reader is built into the program (no Node
needed); the Swift and Dart readers ship with it and are built once in your cache with their own toolchains. Keep
the trees in a folder of their own, as `trees/` here: a `records/` folder beside a tree is read as its records.

**Test and compare**

| Command | Does | Example |
|---|---|---|
| `derive` | writes a draft test for every derived case no test covers; `--fill` also adds the fixture, input and expect the design fixes | `specarch derive --dry-run examples/library-lending/spec` |
| `generate tests` | writes a test package for an environment: every test and every derived case no test covers, as requests to its servers | `specarch generate tests --package staging --out package examples/library-lending/spec` |
| `run` | runs a test package against its environment and writes the results as JUnit XML | `specarch run --results results.xml --server api --dry-run package` |
| `log test-run` | records a test run's results from a JUnit XML file, each test case mapped to its test | `specarch log test-run --dry-run --results examples/library-lending/records/evidence/TR-2-junit.xml --environment staging --at 2026-10-15T09:30:00Z --by library-it examples/library-lending/spec` |
| `compare` | puts several implementation files of one specification in one table, from their test results, scans and measurements | `specarch compare examples/library-lending/spec examples/library-lending/spec/implementation/go/library-lending.go.specarch-implementation.yaml examples/library-lending/spec/implementation/typescript/library-lending.typescript.specarch-implementation.yaml` |
| `diff` | compares two versions of a specification and checks the release between them | `specarch diff examples/lending-desk/spec trees/merged` |
| `idioms` | lists the idioms each implementation file uses, shipped or overridden | `specarch idioms examples/library-lending/spec` |
| `idioms diff` | prints a shipped idiom's parts beside each override's | `specarch idioms diff ui-components examples/library-lending/spec` |

**Learn the program** (more in the next section)

| Command | Does | Example |
|---|---|---|
| `help` | explains an area or a command | `specarch help plan` |
| `guide` | prints the order of work, for a person or, with `--agent`, an agent | `specarch guide --agent` |
| `capabilities` | lists what the program and the plug-ins on `PATH` can do; `--json` as a tool listing | `specarch capabilities --json` |
| `mcp` | serves every command as a tool of the Model Context Protocol, over standard input and output | `specarch mcp` |
| `skill` | writes an agent skill in the Agent Skills format | `specarch skill --out .agents/skills` |
| `version` | prints the program version and the format versions it reads | `specarch version` |

### First steps

Start a specification. `init` writes only what the format requires (the meta-model version, the title and the
version) and nothing a person has not decided:

    specarch init --profile lean notes/

`specarch help start` shows the next step, a first entity and operation that validate. The **lean** profile is for
a small tool or an internal daemon: a test the design implies but nobody has written is a warning, not an error,
and one warning at `info.profile` says so. Every other rule, a permission on every operation among them, is the
same as on **full**, which `profile: full` or no profile at all chooses. `specarch derive` writes those missing
tests into the tests stage's folder once `stages` lists it. The idea of setting a project up in one command comes
from Spec Kit (github.com/github/spec-kit).

Check it. A folder is searched for specifications; a folder holding `specarch.yaml` is one specification:

    specarch validate spec/ examples/

Each problem is one line, in the form compilers use: file, line, column, severity, YAML path, rule, what is wrong and
how to fix it, and an id in brackets that stays the same while lines move:

    design/entities/order.yaml:12:17: error: /entities/Order/relations/customer/target: relation_target: Custmer is not an entity of the specification; did you mean Customer? [relation_target@design/entities/order.yaml#/entities/Order/relations/customer/target]

| Exit status | Meaning |
|---|---|
| 0 | every input is valid (warnings may be printed) |
| 1 | an input has an error |
| 2 | a usage error, a path that cannot be read, or a folder with no specification |

### What `validate` checks

The layout of the tree and the JSON Schema of the merged specification, then what a schema cannot express:
references across files, fail-closed access, the design/implementation boundary, expressions and their types,
worked examples evaluated to the last digit, concrete integer types, the test scenarios each subject needs,
citations, environments and secrets, and traceability from needs to requirements, design and tests.
`docs/conventions.md` describes every rule; the rule names are the `Rule` enum in `spec/design/enums/`.

### Versions

- **The format:** a specification names its meta-model version on its `specarch:` line (`"1.0.0"` today). From 1.0.0
  it follows Semantic Versioning: a file valid on 1.0.0 stays valid and means the same on every 1.x; a 1.x only
  adds, and a keyword is deprecated with a warning before a 2.0.0 removes it.
- **Older files:** 0.3, 0.2 and 0.1 are still read by their own rules, with one warning at the version line.
  `specarch upgrade` moves a file and those beside it to 1.0.0, editing only the version lines, editor hints and
  renamed keys, and lists what it cannot do by itself:

      specarch upgrade --dry-run spec/   # print every change, write nothing
      specarch upgrade spec/

- **The program:** `specarch version` prints its own version (a separate number) and the format versions it reads.

## What the program says about itself

An installed `specarch` explains itself with no checkout or docs folder,
from text and command definitions built into it, so it describes exactly
the commands it was built with:

    specarch help                  # the usage and the topics
    specarch help generate         # an area's guide, or a command's reference
    specarch guide                 # the order of work
    specarch guide --agent         # the same for an agent, with when to ask the owner
    specarch capabilities --json   # everything, as JSON a program reads
    specarch mcp                   # every command as a tool, for an agent's MCP client
    specarch skill --out .agents/skills   # an agent skill, in the Agent Skills format

The help topics are format, start, asking, situations, validate, gaps,
generate, extract, upgrade, plugins, conventions, agents and examples, and
every command by name (`specarch help` lists them all).
`capabilities --json` is an MCP-compatible tool listing: one entry per
command, and per target a plug-in on PATH writes, each with a `name`, a
`description` and an `inputSchema` in JSON Schema draft 2020-12, beside
its outputs, exit codes and remedies; each plug-in answers
`--capabilities` with what it writes and what it does not. The entries are
made from `spec/design/commands`, and a test fails when a command or an
option has no definition there. `AGENTS.md` holds the agent guide, kept
equal to `guide --agent` by a test.

For an agent to find SpecArch without being told, `specarch mcp` is a
Model Context Protocol server on standard input and output whose tools are
exactly the entries of `capabilities --json`, each call run with `--json`;
`specarch skill --out <folder>` writes an agent skill, the agent guide and
one reference per command, which `skills/specarch/` holds for this
release. `specarch help agents` shows how to configure each. Shipping into
the folders agents read follows Spec Kit, which installs its commands for
each agent it supports; SpecArch writes one skill in the Agent Skills
format and one server, both from the command definitions (ADR-214).

Every command takes `--json`: one JSON object on standard output and, on
exit status 2, one error object with a stable code on standard error.
Every command that writes takes `--dry-run`, which lists each file it
would write and writes nothing. No command asks anything.

## Making the documents

    specarch document techspec examples/library-lending/

writes `techspec.md`, an arc42 technical specification with Mermaid diagrams
of the entities, states, operations, commands, pages and permissions, the
deployment and commissioning stages, the requirements and the traceability
matrix, into the folder the implementation file names (or `--out`). It also
refreshes the diagrams between `specarch:generate` markers in the
hand-written `specarch.md` beside the root file. A specification with
errors gets its documents too, with each error marked at its element,
and the command exits 1. With `--check` it writes nothing and exits 1 when
the committed output differs, which is how CI keeps the documents current.
`docs/techspec.md` is SpecArch's own, made from `spec/`.

The other document targets work the same way; the table under Commands
(Installing and running) names each one and the file it writes. `--language id`
writes a document in Indonesian, and `--audience external` leaves out the
internal elements. Wherever the specification says why an element is so, or
cites a standard for it, the document shows an Insight or a Note next to
the element; wherever it says how an element is known, an Origin line; and
wherever an open question blocks an element, the question.

`specarch document html` writes `index.html`, the whole specification as
one page to open from the disk: the overview first, then every element with
its pointer as its id, so `index.html#/entities/Loan` opens at it, the
entity diagram, the workflows and the generated screens, and the problems
linked to the entries they mark. It needs no server and makes no request.
The examples keep theirs in `site/`.

`specarch document problems` writes `problems.txt` and `problems.sarif`,
every error, warning and open question as one compiler line each, and
marks each one in the YAML files of the specification, hand-written or
not: a `# specarch-problem:` comment line above the entry, with the
problem's severity, rule, message and id. The next run takes out every
such line and writes the marks of what it finds, so a fixed problem loses
its mark and nothing else in the file changes. `extract` and `merge` mark
the trees they write in the same way, and `derive` the drafts it writes.
A mark is not part of what is approved: writing or removing one never
voids an approval.

## From an old document to code

A specification built from what exists, a prose document or running code,
says what its sources support and asks about the rest: every element
carries its `origin` (stated, with a citation; inferred, with a reason; or
decided), and what is not known is an open question that names who decides
and what it blocks. `specarch gaps` lists the questions by stage and says
which documents and code targets are ready, drafts or waiting. The owner
answers with decisions and material; `specarch decide <question> --by
<stakeholder> --answer <text>` records each as a decision, takes the
question out and prints the entries to write from it; what a stakeholder
said in a meeting, a chat, an email or a call is kept with `specarch log`
as a communication record the decision cites; the questions
shrink, the documents are regenerated and read, and `specarch approve --by <stakeholder> --session <ID>` records
the approval with a digest of the files and the session in which the
documents were read, refusing a role no record shows was heard. `specarch generate` runs only
then. `docs/refinement.md` is the design.

Code targets (OpenAPI, SQL, UI, tests) are plug-ins: `specarch generate
<target>` runs `specarch-gen-<target>` from PATH, hands it the validated and
approved specification on its standard input, and writes the files it
answers with. `docs/generators.md` has the protocol.

## Status

Version 1.0.0 of the meta-model, October 2026, the first stable version;
a file written against 0.3, 0.2 or 0.1 is read beside it with a warning, a
0.1 file with its missing tests as warnings, and `specarch upgrade` moves
it. Release 1.7.0 is the latest. The schemas, the tree layout and the seven stages, SpecArch's own
specification, seven examples, the validator in Go and in Swift, twenty
documents, open questions with the gaps report, `decide` and the
comparison, the approval gate on generation, the extract readers with `merge`, `check`, `diff`, `derive`
and the sixteen code plug-ins, the guarded scripts among them, exist; the rest is on the roadmap (`docs/roadmap.md`, which says where
each entry stands). The meta-model will change: the first real
projects written in SpecArch are expected to find concepts it cannot
express, and those gaps define the next version.

## Licence

Apache License 2.0. See `LICENSE` and `NOTICE`.

## Compliance and Security

Every release's software bill of materials is committed in `sbom/<tag>/` at the root of this repository, in two
formats from the same syft run: CycloneDX JSON (`<name>-<version>.cdx.json`) and SPDX JSON
(`<name>-<version>.spdx.json`). Beside them are the report of each scanner that ran on the release's tree (grype
on the SBOM, osv-scanner on the source, govulncheck for each Go module, and the repository's own scanners when
it names any), `tools.json` with the scanners' versions and exit statuses, `allowlist-applied.json` with the
accepted findings that applied, and a README.md naming the tag, the commit scanned, the date and the result.
Where the repository's releases allow it, both SBOMs are also attached to the GitHub release.

To run the scan again for a tag: `sbom-record <tag>` from the repository's root. It scans the tag's tree,
writes `sbom/<tag>/` and commits it.

Accepted findings and licences, each with its reason, are in `.dependency-allowlist.json` (`vulnerabilities`,
`licences`, `licence_exceptions`); a scanner's own ignore file (`osv-scanner.toml`, `.grype.yaml`,
`.trivyignore`) holds its entries where one exists.
