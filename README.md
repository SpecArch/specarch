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

SpecArch is built on three principles, set out in `docs/principles.md`; the
rest of SpecArch follows from them. It is a compiler for specifications: it
reads an incomplete or wrong definition rather than refusing it, and every
problem is reported at the place the author fixes it, with the entry it
touches marked in what SpecArch writes.
It follows the Low IQ Tax: every file, key and message should cost its
reader as little thinking as possible. And a change carries its reason at
the entry it changes, so a review reads decisions, not lines of
difference.

## The format

A specification is a folder. Its root file is `specarch.yaml`, which names
the system, its version, the stages it keeps and the sources it cites. Each
stage is a folder of its name beside the root, in life-cycle order:

| Folder | Holds |
|---|---|
| `requirements/` | stakeholders, needs, requirements with acceptance criteria, glossary, assumptions, constraints |
| `design/` | enums, entities, permissions, roles, session, endpoints, commands, channels, dependencies, jobs, workflows, problem types, pages, menus, algorithms, decisions |
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
commissioning runs and approvals). The same schema gives editors
validation and completion through `yaml-language-server`: put this on the
first line of a root file and most editors pick it up.

    # yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/v1.0.0/schema/specarch-design-0.3.schema.json

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
| `idioms/` | the idioms SpecArch ships, one folder per concern, embedded into the program: fifteen, each a stack-neutral contract and its Go rendering, such as `type-rendering` (what each field type becomes in Go and in PostgreSQL, SQL Server, Oracle and MariaDB), `paginated-list`, `request-validation` and `error-response`; `ui-components`, what draws each page and field on Next.js with Carbon; and `design-tokens`, which Carbon token takes each colour of the theme |
| `spec/` | SpecArch's own specification: every stage, the design of the `specarch` command, its Go and Swift implementation files, and in `spec/tests/` the conformance suite every implementation of `specarch` must pass |
| `docs/techspec.md` | SpecArch's technical specification, generated from `spec/` |
| `docs/requirements.md`, `testplan.md`, `traceability.md`, `deployment.md`, `commissioning.md`, `questions.md` | SpecArch's other documents, generated from `spec/` |
| `docs/problems.txt`, `problems.sarif` | every error, warning and open question of `spec/`, one compiler line each and as a SARIF 2.1.0 log |
| `docs/changes.md`, `releases.md` | the change register and the release notes, generated from the records in `records/` |
| `records/` | SpecArch's own records: its releases |
| `swift/` | the Swift build of `specarch` |
| `third_party/typescript-go/` | the TypeScript compiler's Go source that `specarch facts javascript` reads with, copied at a pinned release by `tools/typescript-go/pull.sh`, with its own LICENSE and NOTICE.txt |
| `readers/` | the Swift and Dart readers of `specarch facts`, with the lock files that pin them, embedded into the program |
| `history/` | what changed and why, one file per day |
| `AGENTS.md` | instructions for agents working in this repository, with the guide `specarch guide --agent` prints |
| `docs/help/` | the help topics built into the program, which `specarch help <topic>` prints |
| `BUG_OUTSTANDING.md`, `BUG_HISTORY.md` | the bugs, one BUG-NNN entry each: the open ones, and the fixed ones with the commit that fixed them; the repository has no GitHub issues (`CONTRIBUTING.md`, Bugs) |
| `docs/principles.md` | the three principles, a compiler for specifications, the Low IQ Tax and a change that carries its reason, and how SpecArch applies them |
| `docs/conventions.md` | the tree layout, YAML layout, Markdown sections, generated and hand-drawn diagrams |
| `docs/stages.md` | the seven life-cycle stages: what each holds, which standard says so, and why |
| `docs/maintenance.md` | after commissioning: change requests, defects, releases, incidents and operation, and the records that hold them; the release rules, the diff verb and their documents as designed |
| `docs/diagnostics.md` | the problems file, its SARIF form and the marks at each entry in every output, with the steps that build them |
| `docs/refinement.md` | from an old document to code: partial specifications, open questions, origin, approval and the gate on generation |
| `docs/test-generation.md` | tests from the specification: the golden paths, which red paths are written and why, structured test data, the derive verb and the tests target, as designed |
| `docs/dxlib-lessons.md` | what SpecArch takes from dxlib, the owner's Go library: one type rendered to many targets, the design keywords it proves are needed, a Go implementation on dxlib, and what is left behind |
| `docs/idioms.md` | idioms: how each recurring implementation concern is done the same way everywhere, shipped with SpecArch and overridable per project |
| `docs/ui-design.md` | what an analyst writes about a user interface: page states, events and screen flows, sections, compact screens, accessibility to WCAG 2.2 and a theme of design tokens, and the first UI generator |
| `docs/ui-nextjs-carbon.md` | the second web generator, TypeScript on Next.js and Carbon, with a project's own components as an idiom override: its design, the libraries checked, and the steps that build it |
| `docs/ui-swiftui.md` | the third UI generator, SwiftUI for the iPhone: the same pages as data in a Swift package, read by the one analysis both generators share |
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
| `examples/reading-list/` | an iPhone app on SwiftUI and SwiftData and the Vapor server it calls, with the code-facts dumps of their Swift source and `extract.sh`, which reads both and merges them |
| `examples/room-booking/` | one Express service written in TypeScript and in plain JavaScript, with the code-facts dumps of both and `extract.sh`, which reads both and lists the questions only one of them asks |

## Installing and running

`specarch` is one program with verbs named by direction: `validate`
checks, `gaps` lists the open questions and what they hold up, `document`
goes from the specification to a document, `decide` answers an open
question with a decision record and takes the question out, `approve`
records that the documents were read and the specification is approved, `generate` from
the approved specification to code, `extract` from one surface of
existing code or documents to a specification (its outline, database,
router, Markdown documents, OpenAPI, permissions, pages, workflows, Go,
Swift, JavaScript and Dart readers, as `docs/extraction.md` lists), `merge` joins the
trees extract writes into one, `check` compares a specification with the
dumps of its code and fails on a difference its allowlist does not name,
and `diff` compares two versions of a
specification and checks the release between them, and `derive` writes a
draft test for every derived case no test covers, `idioms` lists the
idioms each implementation file uses, `model` prints the specifications
as one JSON document for a program such as an editor (the merged
document, where every node came from, the references and the layout
problems), and `facts` writes the code-facts
dump extract reads of a committed folder of JavaScript, TypeScript and
Vue (its reader is inside the program, on the TypeScript compiler's own
Go source, so it needs no Node), Swift or Dart (their readers come with
the program and are built once in your cache with their toolchains). It has two builds from
the same design.
The Go build has every verb. With Go 1.27.2 or later:

    go install github.com/SpecArch/specarch/cmd/specarch@v1.1.0

or, from a clone of this repository:

    go install ./cmd/specarch

Code targets are plug-ins on PATH. This repository has twelve:
`specarch-gen-openapi`, which `specarch generate openapi` runs to write the
OpenAPI 3.1 document of a specification, `specarch-gen-sql`, which
`specarch generate sql` runs to write its migrations in PostgreSQL, SQL
Server, Oracle or MariaDB, `specarch-gen-sqlc`, which `specarch generate
sqlc` runs to write the sqlc queries the design derives against those
migrations, `specarch-gen-scripts`, which `specarch generate scripts` runs
to write a guarded script for each data change the design guards, for
psql on PostgreSQL or the mariadb client on MariaDB,
`specarch-gen-tests-go`, which `specarch
generate tests` runs to write its Go tests, `specarch-gen-tests-swift` and
`specarch-gen-tests-dart`, which `specarch generate tests` runs instead of
the Go one for an implementation file in Swift (Swift Testing) or in Dart
(`package:test` or `flutter_test`, so a Flutter app too),
`specarch-gen-go-dxlib`, which `specarch generate go-dxlib` runs to write
the tables, handlers, seeds and tasks of a Go service on dxlib, and
`specarch-gen-ui`, which `specarch generate ui` runs to write the list,
view, form, task and content pages for the web in plain JavaScript,
the route of each custom page around a screen the project writes,
with the redirect to an identity provider's sign-in,
`specarch-gen-ui-typescript`, which it runs instead for an implementation
file in TypeScript to write the screens on Next.js and Carbon, `specarch-gen-ui-swiftui`, which it runs
for a `ui` target of platform iphone to write the same screens in SwiftUI
into a Swift package, and `specarch-gen-bpmn`, which `specarch
generate bpmn` runs to write each workflow as a BPMN 2.0 file with its
diagram, and the diagram as SVG. Install them at the same tag as
`specarch`:

    go install github.com/SpecArch/specarch/cmd/specarch-gen-openapi@v1.1.0
    go install github.com/SpecArch/specarch/cmd/specarch-gen-sql@v1.1.0
    go install github.com/SpecArch/specarch/cmd/specarch-gen-sqlc@v1.1.0
    go install github.com/SpecArch/specarch/cmd/specarch-gen-tests-go@v1.1.0
    go install github.com/SpecArch/specarch/cmd/specarch-gen-tests-swift@v1.1.0
    go install github.com/SpecArch/specarch/cmd/specarch-gen-tests-dart@v1.1.0
    go install github.com/SpecArch/specarch/cmd/specarch-gen-go-dxlib@v1.1.0
    go install github.com/SpecArch/specarch/cmd/specarch-gen-ui@v1.1.0
    go install github.com/SpecArch/specarch/cmd/specarch-gen-ui-typescript@v1.1.0
    go install github.com/SpecArch/specarch/cmd/specarch-gen-bpmn@v1.1.0
    go install github.com/SpecArch/specarch/cmd/specarch-gen-ui-swiftui@v1.1.0
    go install github.com/SpecArch/specarch/cmd/specarch-gen-scripts@v1.1.0

The Swift build, for macOS, has the validate, model, upgrade and version
verbs and gives the same output. With Swift 6:

    swift build -c release --package-path swift
    swift/.build/release/specarch validate spec examples

Check specifications; a folder is searched for them, and a folder holding
`specarch.yaml` is read as one specification:

    specarch validate spec/ examples/

Every problem is one line, in the form compilers use: the file, the line,
the column, `error` or `warning`, the YAML path in the specification, the
rule, one sentence saying what is wrong and how to fix it, and the
problem's id in brackets, which stays the same while lines move.

    design/entities/order.yaml:12:17: error: /entities/Order/relations/customer/target: relation_target: Custmer is not an entity of the specification; did you mean Customer? [relation_target@design/entities/order.yaml#/entities/Order/relations/customer/target]

The exit status is 0 when every input is valid (warnings may be printed), 1
when any has an error, and 2 for a usage error, a path that cannot be
read, or a folder with no specification in it. `specarch version` prints the
program version and the versions of each format it reads.

A specification names the meta-model version it is written against on its
`specarch:` line, `"1.0.0"` today. One written against an earlier version,
0.3, 0.2 or 0.1, is still read by its own rules, with one warning at that
line; `specarch upgrade` moves it and every file beside it to 1.0.0,
editing only the version lines, the editor hints and the keys 1.0.0
renamed, and lists what it cannot do by itself:

    specarch upgrade --dry-run spec/   # print every change, write nothing
    specarch upgrade spec/

The compatibility promise: SpecArch specifications are written against a
meta-model version, the `specarch:` line of the root file. From 1.0.0 the
meta-model follows Semantic Versioning: a file valid on 1.0.0 stays valid
and means the same on every 1.x; a 1.x only adds. A keyword is deprecated
with a warning before a 2.0.0 removes it. Every earlier version, 0.3, 0.2
and 0.1, is read beside 1.0.0 with a warning; `specarch upgrade` moves a
file on one of them. The program's own version (`specarch version`) is a
separate number.

The validator checks the layout of the tree, applies the JSON Schema to the
merged specification, then checks what a schema cannot express: references
across the tree, fail-closed access, the boundary between design and
implementation, expressions and their types, worked examples evaluated to
the last digit, concrete integer types, the test scenarios each subject
needs, citations, environments and secrets, and traceability from needs
through requirements to design and tests. `docs/conventions.md` describes
every rule; the rule names are the `Rule` enum in `spec/design/enums/`.

## What the program says about itself

An installed `specarch` explains itself with no checkout or docs folder,
from text and command definitions built into it, so it describes exactly
the commands it was built with:

    specarch help                  # the usage and the topics
    specarch help generate         # an area's guide, or a command's reference
    specarch guide                 # the order of work
    specarch guide --agent         # the same for an agent, with when to ask the owner
    specarch capabilities --json   # everything, as JSON a program reads

The help topics are format, validate, gaps, generate, extract, upgrade,
plugins, conventions and examples, and every command by name.
`capabilities --json` is an MCP-compatible tool listing: one entry per
command, and per target a plug-in on PATH writes, each with a `name`, a
`description` and an `inputSchema` in JSON Schema draft 2020-12, beside
its outputs, exit codes and remedies; each plug-in answers
`--capabilities` with what it writes and what it does not. The entries are
made from `spec/design/commands`, and a test fails when a command or an
option has no definition there. `AGENTS.md` holds the agent guide, kept
equal to `guide --agent` by a test.

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

The other document targets work the same way: `requirements` (the
requirements specification), `testplan` (the test plan and test cases),
`traceability` (the matrix and its gaps), `deployment` (the deployment
guide), `commissioning` (the commissioning procedure and sign-off sheet),
`handover` (what a customer who builds and runs an environment from the
source receives), `questions` (the open questions and what they hold
up) and `comparison` (the old documents against the refined
specification: kept, clarified, added or corrected, and dropped), each as
`<target>.md`. Wherever the specification says why an element is so, or
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
question out and prints the entries to write from it; the questions
shrink, the documents are regenerated and read, and `specarch approve --by <stakeholder>` records
the approval with a digest of the files. `specarch generate` runs only
then. `docs/refinement.md` is the design.

Code targets (OpenAPI, SQL, UI, tests) are plug-ins: `specarch generate
<target>` runs `specarch-gen-<target>` from PATH, hands it the validated and
approved specification on its standard input, and writes the files it
answers with. `docs/generators.md` has the protocol.

## Status

Version 1.0.0 of the meta-model, October 2026, the first stable version;
a file written against 0.3, 0.2 or 0.1 is read beside it with a warning, a
0.1 file with its missing tests as warnings, and `specarch upgrade` moves
it. Release 1.1.0 is the latest. The schemas, the tree layout and the seven stages, SpecArch's own
specification, six examples, the validator in Go and in Swift, fifteen
documents, open questions with the gaps report, `decide` and the
comparison, the approval gate on generation, the extract readers with `merge`, `check`, `diff`, `derive`
and the twelve code plug-ins, the guarded scripts among them, exist; the rest is on the roadmap (`docs/roadmap.md`, which says where
each entry stands). The meta-model will change: the first real
projects written in SpecArch are expected to find concepts it cannot
express, and those gaps define the next version.

## Licence

Apache License 2.0. See `LICENSE` and `NOTICE`.
