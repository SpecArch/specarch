# Roadmap

Order of work after meta-model 0.1. Each step produces something the previous
one is checked by: the validator checks the meta-model, the generators check
the validator, and the real projects check all of it.

Each entry says where it stands in the code: built, with the release that
first carried it and the file or conformance case that shows it; partly
built, with what is missing; or not built. A release is a tag; 1.4.0 is the
latest.

## Releases

| Release | What it first carried |
|---|---|
| 0.1.0, 2026-10-07 | the validator in Go and in Swift (`cmd/specarch`, `swift/`) with the conformance suite `spec/tests/`; the documents techspec, requirements, test plan, traceability, deployment guide and commissioning procedure; the operation stage and its monitors |
| 0.2.0, 2026-10-08 | open questions, `specarch gaps`, the `questions` document, `specarch approve` and the gate on `generate` |
| 0.3.0, 2026-10-08 | `specarch derive`; `specarch diff`; change, defect, release and commissioning records with their rules; the `changes` and `releases` documents; `guard`, `dependencies`, `idempotencyKey`, `validity` and `session` |
| 0.4.0, 2026-10-08 | `specarch-gen-openapi`, with the dxlib dialect; `specarch-gen-sql`; `specarch-gen-go-dxlib`; `specarch-gen-tests-go`; `specarch idioms`; `views`, `jobs` and menus |
| 0.5.0, 2026-10-09 | `specarch extract` with the outline, database, router, documents, OpenAPI, permissions and pages readers; `specarch merge`; `specarch-gen-ui` for list pages in plain JavaScript; `specarch-gen-tests-swift` and `specarch-gen-tests-dart`; the page events, states and theme |
| 0.6.0, 2026-10-13 | meta-model 0.2 (`schema/specarch-design-0.2.schema.json`); the workflows reader; the Go reader on dxlib, `net/http` and the common routers; the Swift, JavaScript, Vue and Dart readers; `specarch-gen-ui-typescript` on Next.js and Carbon; `specarch-gen-bpmn`; the `problems` and `html` documents |
| 0.6.1 to 0.6.4, 2026-10-14 | the plans for the estate and for meta-model 0.3; the first estate points (a dump committed outside the code's repository, role names on the wire); a request body decoded into a struct in `extract go`; fixes to `merge` and the readers |
| 0.7.0, 2026-10-19 | the estate (`specarch-estate.yaml`, references between specifications, the module map); `specarch check`; `specarch facts`; `specarch-gen-sqlc`; indexes in `generate sql`; `implies`, `audience`, `middlewares`, the workflow table, documents in another language; the `handover` document |
| 0.7.1, 2026-10-20 | the techspec's parameters, request body and responses per operation; `extract go` reading a dxlib handler's decoded body and JSON answers where the registration declares none (ADR-140); every operation's handler followed into the code, recorded with its source and drawn as a flowchart (ADR-144); the designs of the diagram keywords (`docs/diagrams.md`) and of the gateway (`docs/gateway.md`), proposed; dxlib's encrypted API type as a handshake an operation names and large files as a stream it declares (ADR-145, ADR-146), read by `extract go`, served by the dxlib dialect and the go-dxlib target with a Go client, and shown in the techspec; this roadmap matched to the code |
| 0.7.2, 2026-10-20 | `specarch-gen-ui-swiftui`, the screens for the iPhone in SwiftUI (ADR-147), with `generate` finding a plug-in by the target's framework before the file's language, and the pages read once for both UI generators that write a page as data (`internal/genscreens`); the diagram keywords (ADR-133 to ADR-139): `specarch model`, the context, component, deployment and network diagrams with their markers, the rules `channel`, `depends_on`, `server`, `link` and `link_missing`, and lanes and ids in the BPMN output; caller context (ADR-106 to ADR-111, ADR-159): the identity module and the references into it, the chain by induction through `calls`, one audience per module, the scope case, the identity module's matrix and the chains in the techspec, and the readers of the client, application, caller and audience checks and of a grant's `client` and `through`; the user manual and the operations guide, with `remedy` on a problem type and `response` on a monitor (ADR-160); the gateway (ADR-141 to ADR-143): `gateways` with the rule `gateway`, its documents, the estate's gateway module, and the gateway table read by `extract gateway` and compared by `check`; `specarch-gen-scripts`, the guarded operational scripts in psql for PostgreSQL (ADR-158); meta-model 0.3 (`schema/specarch-design-0.3.schema.json`), named by `specarch version` in both builds; `specarch-gen-ui`'s view, form and task pages and the navigation between pages in plain JavaScript (ADR-152, ADR-153); `specarch decide`, `problems.sarif` and the `comparison` document (ADR-155 to ADR-157) |
| 0.7.3, 2026-10-21 | every call of the plain JavaScript screens through one transport, under the names on the wire `wireNames` gives, with dxlib's pre-key handshake and the streams (ADR-164); a command-line tool's commands in the user manual, with `info.program` and a command's `remedies` (ADR-165); a job's `deadLetter` and its steps in the operations guide (ADR-166); `specarch-gen-scripts` for MariaDB beside PostgreSQL (ADR-167) and a guard's `writes` (ADR-168); the client, application, caller and audience checks and the event writers in the swift, javascript and dart readers (ADR-169); `through` on a rule of the permission table (ADR-170); the derived cases reading the identity module given in the same run (ADR-171); `link_missing` held by a must question on the environment a deployment installs (ADR-172); `specarch model` listing an operation's `via` and a role's `through`; `RUN_INSTRUCTION.md`, a specification from documents and code with the command alone; fixes to `generate ui`, `generate sql` on MariaDB, `derive`, `approve` and the handover and operations documents (BUG-035, BUG-036, BUG-038, BUG-041 to BUG-044, BUG-046, BUG-047, BUG-050 to BUG-052) |
| 0.7.4, 2026-10-22 | the swift, javascript and dart readers asking for the transaction token and `via` at a `Txn-Token` header read, as `extract go` does (ADR-173), which closes caller context; ADR-167 and ADR-168 accepted; both builds reading every YAML scalar as yaml.v3 does, integers in every base, floats with underscores and every tagged form held by one table of scalar forms (ADR-174 to ADR-176); a YAML syntax error worded the same by both builds, with the parser's own message on standard error, and the texts one build read and the other refused refused by both (ADR-177); fixes to `derive`, `validate`'s suggested test names, `extract go`'s question count, `extract javascript`'s mounted routers and the documents' marks at an implementation file's entry (BUG-030, BUG-031, BUG-033, BUG-045, BUG-048, BUG-049, BUG-054 to BUG-056) |
| 0.7.5, 2026-10-22 | every check that compares or counts an integer, the derived case names and the technical specification, manual and site reading a design's integer written in hexadecimal, octal, binary or with underscores as the schema check reads it, through one reader per build (ADR-178), held by `validate-integer-forms` and three document cases; fixes to `list_of`, `limits`, separation of duties and the derived case names (BUG-057, BUG-058) |
| 1.0.0, 2026-10-25 | meta-model 1.0.0, the first stable version (ADR-186, `docs/meta-model-1.0.0.md`): the design schema and its fragments at 1.0 with the version line in three parts, a role's `rows` renamed `rowPolicies` and the handover's `source` renamed `sourceDelivery` (the old keys deprecated and still read), the schema's conditionals requiring the key they read; the implementation file, record, idiom, estate and vocabulary formats and the model document at 1.0.0; 0.3, 0.2 and 0.1 read beside 1.0.0 with a warning; `specarch upgrade` with `--dry-run` in both builds, and extract and merge writing 1.0.0 trees; the tenant scope said once with `tenant` (ADR-179); the generated dxlib service applying the rows rules (ADR-180); `content` pages (ADR-181); sign-in through an OpenID Connect provider in the three UI generators (ADR-182); `custom` pages (ADR-183); generation across the estate with `--estate` (ADR-184); `storage: json` and the value object check on a field typed by another module's schema; a reference inside another module's element resolved through that module's dependencies (ADR-185); fixes to the flow step checks, the Next.js paging names and references across modules (BUG-060 to BUG-063) |
| 1.1.0, 2026-10-25 | the installed program saying what it can do and how to use it, from the definitions it was built with (ADR-187, SA-96): `specarch help <topic>` with the area guides and each command's reference, `specarch guide [--agent]`, `specarch capabilities [--json]` as an MCP-compatible tool listing with a JSON Schema 2020-12 input schema per command and per plug-in target, `--capabilities` in every plug-in, `--json` on every command with versioned output and error objects and stable error codes, `--dry-run` on every command that writes, and `AGENTS.md` kept equal to `specarch guide --agent` |
| 1.2.0, 2026-10-26 | a decision naming the decisions it rests on under `restsOn`, one optional key in the 1.0 design and implementation schemas (ADR-188, SA-97); both validators refusing a `restsOn` entry naming a missing decision and a chain that comes back to its own decision (`decision_cycle`, at every entry on the cycle), and warning at a current decision resting on a superseded one (`decision_superseded`, naming the successor); `specarch decision why` and `specarch decision rests` printing the chain each way, with `--json`; the technical specification and the site linking `restsOn` and `supersededBy`; the goal and the principle that a change carries its reason in `docs/principles.md` |
| 1.3.0, 2026-10-28 | `specarch try` in both builds, evaluating an algorithm's formula on its worked examples or on `--inputs`, with a proposed definition from `--file`, printing every operation and call with its value and failing a result whose type does not fit the output (ADR-195, SA-100); the idea record in `records/ideas/`, a proposal with its status, decision and what it `became`, checked in both validators (`idea_decision`, `idea_question`, `record_ref`) and named by `decision why` and `decision rests` (ADR-196, SA-101); `timeZone` at the root of the design, an IANA name, with `date()` of an instant read in it in both builds and `time_zone` at a wrong name or a `date()` without it (SA-99); the state machine and date-time designs, `docs/state-machines.md` (ADR-189 to ADR-194, SA-98), designed and not yet built; a role's home page, the reserved permission `signedIn`, a field's mode by permission and the navigation warnings (`role_home`, `navigation`), with the three ui generators opening each person's home (ADR-197, SA-102), and `navigate: callerHome` on a task page's answer (ADR-206); the workflow step kinds `decision`, `task` and `wait` and an operation step's `onFailure` and `compensatedBy`, checked in both validators, written and read in BPMN 2.0 (ADR-198, SA-103); an algorithm given as a decision table with a hit policy of `unique` or `first`, checked for overlap and gaps (`decision_table`, `table_overlap`, `table_gap`) and evaluated by `try` (ADR-199, SA-104); a dependency's authentication, rate limit and operations and a call's mapping, idempotency key, retries, error mapping and fire-and-forget mode, checked (`integration`) with the cases they make derived (ADR-200, SA-105); an idea's `inputMode: walkthrough` |
| 1.4.0, 2026-10-31 | a call's mapping holding worked examples, each its `inputs`, `sent`, `answer` and `kept`, checked in both validators under `example_input`, `example_expected`, `example_mismatch` and `example_error` (ADR-207, SA-106); `specarch try` taking an operationId and evaluating the mapping examples of its calls, `--json` under `tryOperation`; the Go, Swift and Dart test generators writing a test per mapping example through `MappingHarness`; `specarch decide` answering a question that blocks a decision with a record that rests on it, writing `restsOn` (ADR-208, BUG-065) |

## 1. Validator CLI

Built, 0.1.0. Specified first, in SpecArch: `spec/` is the specification of
the `specarch` command, every stage of it, and `spec/implementation/go/` the
Go implementation of it. The validator checks the tree, which is the first
self-hosting check.

The validator has two implementations of the one specification: Go, for
macOS, Linux, Windows and Android, and Swift, for macOS, each with its own
implementation file. They prove the split: one design, two languages, and
nothing in the design changed for the second. Both pass the same
conformance suite, the tests stage `spec/tests/`, which holds only inputs
and the expected exit status, output and files, with no code from either
language. The documents and generators are built in Go; the Swift build
has the `validate`, `model` (0.7.2, ADR-138) and `version` verbs.

A single binary, `specarch validate <files>`, written in Go in this
repository. It checks design and implementation files and does what
JSON Schema cannot:

- cross-references: relation targets, primary-key fields, page columns and
  fields, transition states, requirement prefixes, `emits` targets, `source`
  and `submit` operation IDs, `algorithm` names;
- the expression language of `check` constraints and formulas, parsed, not
  pattern-matched;
- worked examples: the formula is evaluated on each example's inputs and must
  produce the expected value, so a wrong example fails validation rather than
  becoming a wrong test;
- fail-closed access: every operation and page has a permission that exists,
  every permission is granted by at least one role or is `public`;
- the interface boundary: no stack-specific key in a specification, no
  design in an implementation file, and an implementation file's
  `implements` and pointers resolve in its specification;
- the tree: the root file's `stages` and the folders agree, every file
  holds only its stage's sections, and every name is defined once;
- the life cycle: every link between stages resolves, no secret carries a
  value, and the traceability gaps are reported.

The Go build reads the JSON Schemas in `schema/` with
`santhosh-tekuri/jsonschema`, so the schema stays the single definition. The
conformance suite holds valid and invalid trees for every rule, and the
repository's CI runs `specarch validate spec examples`, in both builds, on
every push to `main` and every pull request. The
validator is the first thing to be in version control and runnable by
anyone, which the lessons from an earlier in-house language made the first
requirement.

## 2. Targets, in order of payoff

Documents are `specarch document <target> <folders>`, built into the
program. Built: techspec, requirements, test plan, traceability, deployment
guide and commissioning procedure (0.1.0), questions (0.2.0), changes and
releases (0.3.0), problems and the HTML site (0.6.0), and the handover
(0.7.0); `document --audience external` leaves out the internal elements
(0.7.0); the user manual and the operations guide (0.7.2, ADR-160).
Code is `specarch generate <target> <folders>`, each
target a plug-in `specarch-gen-<target>` found on PATH (`docs/generators.md`
has the protocol). Every target writes into a folder it owns. It reads the
specification for the design and the implementation file for the target
choices: the folder, the downstream code generator and its settings, the
type each field maps to.
The defaults (PostgreSQL for SQL, plain JavaScript for the web UI, SwiftUI
for the iPhone UI) are recorded in the implementation schema, never in a
specification. Generated files carry a header naming the root file and
version they came from. A generator never edits a hand-written file;
Markdown documents are updated only between the `specarch:generate` markers.
Every emitter has a `--check` form that regenerates and fails on a
difference, and CI runs it on every document and code target the examples
use but the library lending example's `tests` target, which writes into
the service's own tree the example does not hold. The rules and the pattern each target follows are in
`docs/generators.md`. `specarch idioms` (0.4.0) lists the idioms each
implementation file uses and compares an override with the shipped idiom.

1. Built, 0.1.0: the technical specification in Markdown, arc42 layout,
   with the generated Mermaid diagrams (entity, state, sequence, pages,
   permissions matrix, and with an estate the module map). In 0.7.1 it
   shows each operation's parameters, request body and responses field by
   field (`spec/tests/document-techspec-requests`) and each handler's
   steps as a flowchart (`spec/tests/document-techspec-handler`). Highest
   payoff: it is what reviewers read, and it makes the YAML visible.
2. Built, 0.4.0, standard dialect: `specarch-gen-openapi` writes the
   OpenAPI 3.1 document (`spec/tests/generate-openapi`). Fields map one to
   one, since the keywords are OpenAPI's. Permissions become the security
   scheme the target names plus a `x-specarch-permission` extension per
   operation; refusals are RFC 9457 problem documents; lists page through
   the paginated-list idiom. Server interfaces and types for most stacks
   then come from a standard OpenAPI code generator per stack, not from
   SpecArch. Built, 0.4.0, for Go on dxlib: the dxlib dialect of the
   OpenAPI document (`spec/tests/generate-openapi-dxlib`) and
   `specarch-gen-go-dxlib`, which writes the tables, handlers, seeds and
   tasks of a dxlib service (`spec/tests/generate-go-dxlib`).
3. Built, 0.4.0: `specarch-gen-sql`, the first migration, the snapshot and
   the differ (`spec/tests/generate-sql`); indexes from a mapping's
   settings built in 0.7.0 (ADR-121, `spec/tests/generate-sql-index-changed`).
   SQL migrations, new files only, in PostgreSQL by default; SQL Server,
   Oracle and MariaDB render through the type-rendering idiom of
   `docs/idioms.md`, with the table in `docs/dxlib-lessons.md`. The
   generator diffs the spec against the last generated snapshot and writes
   a forward migration. A deployed migration is never regenerated or
   edited, and a destructive step is its own file, so a live column changes
   by expand and contract across releases. Constraints become real database
   constraints; `check` expressions are translated, and a constraint the
   target cannot express fails generation rather than being dropped. Built,
   0.7.0: `specarch-gen-sqlc` writes the sqlc queries the design derives
   against those migrations, in PostgreSQL (ADR-123,
   `spec/tests/generate-sqlc-queries`).
4. UI page definitions for the target component library: one generator
   per library, emitting the project's own component usage so generated
   screens look like the hand-built ones. The default for the iPhone is
   SwiftUI. The default for the web is plain JavaScript: native ES modules
   in the browser, no package install, no bundler and no build step, and any
   third-party library a pinned file (OSI licence, SBOM recorded) committed
   next to the generated code. Generated web pages and components talk
   through one small event bus instead of calling each other: a hand-readable
   module of a few lines with no library behind it; every event name declared
   once, in one file, as an UPPERCASE constant with its payload shape; every
   subscription registered where its component is created; no wildcard or
   computed event names; and an optional debug log of every event, so the
   flow can be followed. The specification's events stay neutral; the bus is
   how the plain JavaScript web implementation carries them. On Next.js,
   which has a router and React's props, a page's events are navigation in
   its `page.tsx` and no bus is written (`docs/ui-nextjs-carbon.md`). The
   design keywords a generator reads are built, 0.5.0 (`docs/ui-design.md`):
   page events and flows, states, sections, compact columns, the
   accessibility target and the theme.
   - Built for the web in plain JavaScript: `specarch-gen-ui` writes list
     pages (0.5.0, ADR-040, `spec/tests/generate-ui`), and, in 0.7.2, view
     pages and the navigation between pages, from a menu, a link, a
     selected row and an event (ADR-152,
     `spec/tests/generate-ui-view`), and form and task pages with their
     checks, conditions, pickers, values and child rows (ADR-153,
     `spec/tests/generate-ui-form`), every page of
     `examples/library-lending/web/`, checked against the member view and
     form written by hand first
     (`examples/library-lending/web/reference/`); and, in 0.7.3, the
     names on the wire by `wireNames`
     (`spec/tests/generate-ui-wire-names`) and every call through
     `transport.js`, in the dxlib dialect too, with dxlib's pre-key
     envelope and the streams (ADR-164, `spec/tests/generate-ui-dxlib`).
     The member view's and form's modules have moved on from the ones
     written by hand, since their calls name an operation's path with its
     {parameters} left in. Not built: the first real
     project's screen to check it against.
   - Built, 0.6.0, for the web on Next.js and Carbon, for a project whose
     screens are built there: `specarch-gen-ui-typescript` writes task pages
     such as sign-in (ADR-067), lists, forms, views, server routes, strings
     and the theme, with the components as an idiom a project overrides,
     and compares pages built by hand as data; all seven steps of
     `docs/ui-nextjs-carbon.md` (`spec/tests/generate-ui-typescript`,
     `spec/tests/generate-ui-typescript-hand-built`,
     `examples/library-lending/web-nextjs/`).
   - Built, 0.7.2, for the iPhone in SwiftUI: `specarch-gen-ui-swiftui`
     writes the same task pages, lists, forms, views, navigation (one
     `Route` enum the app's stack follows), strings and theme into a Swift
     package the project writes the `Package.swift` of, each page a Swift
     literal typed by its component's schema type, with the components as
     the idiom's `swiftui` rendering a project overrides, found by the
     target's framework before the file's language; the pages are read by
     the one analysis the TypeScript generator reads them through
     (ADR-147, `docs/ui-swiftui.md`, `spec/tests/generate-ui-swiftui`,
     `spec/tests/generate-ui-swiftui-refused`,
     `examples/library-lending/ios/`).
5. Other DSL formats on request, limited to what that DSL can execute:
   a concept the target cannot represent is reported, not silently omitted.
   Built, 0.6.0: `specarch-gen-bpmn` writes each workflow as a BPMN 2.0
   file with its diagram, and the diagram as SVG (`spec/tests/generate-bpmn`,
   `examples/library-lending/workflows/`).
6. Built: tests from the specification. The specification gives the
   business cases: every design test (golden and red, given, when and then)
   and every worked example. The implementation file gives the target: the
   test framework, the suites, fixtures and how each suite runs. One
   generated test per design test and per worked example, with the given,
   when and then written into the test as its steps. `docs/test-generation.md`
   designs the step before it: `specarch derive` (0.3.0,
   `spec/tests/derive-writes-drafts`) writes the derived cases as tests,
   choosing the red paths by the harm a requirement names and by how often
   users make the mistake, with the cases left out listed in the test plan;
   and structured `fixture`, `input` and `expect` on a test give the
   generator something to run. The targets: `specarch-gen-tests-go` (0.4.0,
   `cmd/specarch-gen-tests-go`; its case `spec/tests/generate-tests-go`
   came in 0.5.0), `specarch-gen-tests-swift` and
   `specarch-gen-tests-dart` (0.5.0, `spec/tests/generate-tests-swift`,
   `spec/tests/generate-tests-dart`).
7. Built, 0.7.2: the user manual and the operations guide (ADR-160).
   The manual is written from the flows, menus, pages, roles and problem
   types, with a problem type's `remedy` saying what the person does
   next; the operations guide from the environments and deployments,
   monitors with their `response`, jobs, channels, dependencies,
   endpoints, settings and the backup, restore and rollback steps. Both
   are written for any specification that names their folder, an empty
   section said in one sentence naming its key (`spec/tests/document-writes-manual`,
   `document-manual-none`, `document-writes-operations`,
   `document-operations-none`). A command-line tool's commands came in
   0.7.3 (ADR-165, `spec/tests/document-writes-manual-cli`): each with its
   usage line from `info.program`, its arguments, options, files and exit
   statuses with a command's `remedies`, and SpecArch's own manual is
   `docs/manual.md`. What a person does with an item a job keeps apart came
   in 0.7.3 (ADR-166, `spec/tests/document-writes-operations`): the job's
   `deadLetter` names the role that handles it, where it is kept and the
   steps to look at it, try it again or discard it. The handover document (0.7.0) says what a customer who
   runs the system receives. Not planned until a project asks: other
   project documents.
8. Built: guarded operational scripts for data changes on live systems:
   preconditions, dry run by default, postconditions with exact counts,
   idempotent. The `guard` on an operation or a command they are emitted
   from came in 0.3.0 (`spec/tests/validate-guard`); `specarch-gen-scripts`
   (0.7.2, ADR-158, `spec/tests/generate-scripts`) writes a psql script for
   PostgreSQL per guarded entry whose change a transition it triggers, a PUT
   by its key or a DELETE by its key says, and warns at the guard for every
   other change with no `writes`, left to be written by hand; the same scripts for the
   mariadb client on MariaDB (0.7.3, ADR-167,
   `spec/tests/generate-scripts-mariadb`). The guard's `writes` says what
   any other change writes, the fields it sets to a value the operator
   gives or a literal, or that it deletes the records, and the scripts are
   written from it (0.7.3, ADR-168, `spec/tests/validate-guard-writes`).
   Not built: SQL Server and Oracle.

"Any language or framework" means any stack that has a generator written for
it. Generators are added one stack at a time, driven by the real projects
below.

## 3. Sync gates in CI

Built. The checks that keep a spec and its code equal, in
`docs/sync-gates.md`: the spec validates, the built router's routes match
`paths` in both directions, a database built from every migration matches
`entities`, and regenerating everything leaves the working tree unchanged.
Each gate compares only what can be derived exactly and reports the rest.
`specarch check` (0.7.0, `spec/tests/check-fails-on-a-difference`) runs the
route and database gates and compares the grants on the dumps the readers
read, with an allowlist that only shrinks (ADR-124); the lending desk's CI
runs it. `generate --check` and `document --check` are the regeneration
gate.

## 4. The owner's finished applications as specifications

The first real test. Each finished personal application in the owner's
collection is written as a specification by extraction, following
`docs/extraction.md`: find where the running system reads each fact, extract
mechanically, regenerate what the generators can produce, compare it with the
code, and only then review by hand. The comparison is recorded, and so is
every concept the code has that the meta-model cannot express. Those gaps,
collected across the applications, are the input to the next meta-model.
Being personal projects, the resulting specifications can be published here
as examples; none is published yet. CI runs the readers on generic
examples instead: the lending desk, the notice board, the reading list and
the room booking in `examples/`, each extracted twice and compared.

`specarch extract <source>` is built one reader at a time, in the steps of
`docs/extraction.md`, Building extract. Built: outline, database, router,
documents, OpenAPI, permissions and pages (0.5.0); workflows, Go (dxlib,
`net/http` and the common routers), Swift, JavaScript and TypeScript with
Vue, and Dart (0.6.0); the workflow table (0.7.0). `specarch merge` (0.5.0)
joins their trees, and `specarch facts` (0.7.0) writes the code-facts dump
the Swift, JavaScript and Dart readers read. In 0.7.1 `extract go` follows
each operation's handler into the code and records its steps with the
source (ADR-144, `spec/tests/extract-go-handlers`). What the extraction
cannot settle becomes open questions, and the path from there to approved
code is `docs/refinement.md`: `gaps`, the decisions that answer questions,
`approve` and the gate on `generate` are built (0.2.0); `specarch decide`,
which answers a question with the decision record and takes the question
out (ADR-155, `spec/tests/decide-*`), SpecArch's side of the
dispatcher reading `problems.sarif`, the log carrying what the agent queue
needs of each question (ADR-156), and the document target
`comparison`, the old documents against the refined specification
(ADR-157, `document-writes-comparison`), are built (0.7.2). The dispatcher's own
reading of the log is the agent queue's item. A
system with both documents and code is written by the procedure in
`docs/from-sources.md`, and `gaps` shows which section of each source
produced which elements. How the questions are put to a
person, one at a time with a recommended answer and no fixed number, is
`specarch help asking`, and `specarch gaps --next` ranks the one to ask
next by what its answer unblocks, with `--json` for an agent (ADR-209,
SA-107, `spec/tests/gaps-next-*`): built on main, in no release yet.

Candidates, in rough order of size: a menu-bar agent manager (macOS), a home
solar monitoring system (Go services on a small board plus an iPhone app), a
phone browser, a code editor with a remote-control protocol. The remote-control
protocol is the interesting one: it is shared by two applications, so its
specification must be importable by both.

## 5. Meta-model 0.2 and after

Meta-model 0.2 is built, 0.6.0 (`schema/specarch-design-0.2.schema.json`),
driven by the gaps found in step 4 and by the first request for generated
screens. `docs/meta-model-0.2.md` is the plan: what is taken, what waits
and why, and the steps that build it, each with what done looks like.
Built, in the order they were built:

- `separationOfDuties`: sets of permissions one holder must never have
  together; the validator checks that no role grants them
  (`spec/tests/validate-separation-of-duties`);
- task pages, a page that submits to an operation without loading a
  record (sign-in, a second factor; `spec/tests/validate-task-pages`);
- content pages, ordered blocks of headings, paragraphs, images and
  links that load and send nothing, written by both web generators
  (ADR-181; `spec/tests/validate-page-content`);
- custom pages, a screen the project builds by hand (an editor, a live
  view) with its route, guard, menu entry, flows and derived tests in
  the design, the generated route importing the project's screen
  (ADR-183; `spec/tests/validate-page-custom`);
- `signIn`, a person's sign-in at an identity provider with OpenID
  Connect's authorisation code flow and PKCE: the callback's six derived
  refusals and `navigate: signIn`, written by the three UI generators
  (ADR-182; `spec/tests/validate-sign-in-oidc`);
- `workflows`: a request that finishes after people approve it (trigger,
  form, approval and operation steps, deadlines), as an object of its own,
  not an annotation on an endpoint; `flows` remains a person's navigation
  across pages (`spec/tests/validate-workflow`);
- the page elements of a back-office screen: lookup fields, row actions
  that depend on the row, a confirmation with a reason, a field's mode,
  checks across fields, child rows, and maker-checker on a page
  (`spec/tests/validate-child-rows`, `spec/tests/validate-maker-checker`);
- value objects, `schemas`: data passed around but not stored
  (`spec/tests/validate-value-objects`);
- a unique constraint with a condition, an element served only behind a
  setting, a requirement's target release, and a name on the wire
  (`spec/tests/validate-unique-where`, `spec/tests/validate-enabled-by`,
  `spec/tests/validate-requirement-release`,
  `spec/tests/validate-wire-names`);
- a schema per fragment file (`schema/specarch-fragment-*-0.2.schema.json`);
- missing test scenarios as errors, the one change that breaks a 0.1 file
  and so the one that moved the version
  (`spec/tests/validate-meta-model-0-2-missing-test`);
- the workflows reader of `specarch extract`
  (`spec/tests/extract-workflows-writes-tree`).

Built before 0.2 and kept in 0.1, since none breaks an existing file:
`guard` (0.3.0), the input to generator 8, whose postcondition is the
generated script's to check; a read model (`views`, 0.4.0); background jobs
and schedules (`jobs`, 0.4.0) and the other keywords `docs/dxlib-lessons.md`
found needed: the sensitivity of a field, encryption at rest, audited
entities and soft delete, list operations, a problem catalogue for error
responses, limits on an operation, and menus; and the concepts the red
paths of `docs/test-generation.md` needed, `dependencies` with a time limit
per call, `idempotencyKey`, `validity` on an entity and `session` (0.3.0).

Built, 0.7.2: meta-model 0.3, all five steps of `docs/meta-model-0.3.md`
(ADR-094 to ADR-097, ADR-154): the caller as something an expression can
name and the rows a role reaches (a member sees only their own loans),
written as row security policies by `generate sql`; machine clients with
a proof, `callers` as the explicit list, `nobody` and `through`; tokens
with their claims, checks and audience, the transaction token, `via` and
security events, with the schemes `generate openapi` writes; and in
`extract go` a question for the callers of a route a client check or a
signature check guards and for the claims where golang-jwt mints a token.
Not planned until a project asks: interfaces beyond HTTP, messaging and the
command line; sets and lists in the expression language; seed data per
profile; more than one deliverable per specification.

Built: meta-model 1.0.0, the first stable version (ADR-186,
`docs/meta-model-1.0.0.md`): what it freezes and what a 1.x may add, the
deprecation rule with `rowPolicies` and `sourceDelivery` as its first two
renames, every earlier version (0.3, 0.2 and 0.1) read beside it with a
warning, every other format at 1.0.0 with its 0.1 read beside it, the
schema's conditionals made to require the key they read, and `specarch
upgrade` in both builds. SpecArch's own specification and every example
are on 1.0.0. It takes the place of the breaking meta-model 0.4 that
`docs/decision-tracking-2026-10-25.md` planned: the reasons of a decision
stay warnings, never errors in a 1.x.

Built, 0.6.2 to 0.7.0: an estate of many modules, each specified on its own
by a team that may only read the code, as `docs/estate.md` designs it, all
fourteen of its items (ADR-098 to ADR-124): a dump committed outside the
code's repository, references between specifications and an umbrella that
lists the modules (`spec/tests/validate-estate`), documents in another
language, role names on the wire, authorisation by path pattern through
`implies`, a gate by caller kind, a printed workflow table, an element's
audience and a requirement held elsewhere, the sqlc queries and
`specarch check`. Every keyword in it is an addition to 0.2. Still owed in
it: the sqlc check against a real project's first query file.

Built, 0.7.2: caller context, all five items of
`docs/caller-context.md` (ADR-106 to ADR-111, and ADR-159 for the
readers). The same estate's services call each other on a person's
behalf, and the design decides the four inputs of an authorisation
decision beyond the person and the roles: the application the person
signed in through as a condition on a grant (`kind`, `softwareVersion`
and `softwareStatement` on a client keyed by its `software_id`, `through`
on a role, checked at every hop), the calling service with grants of its
own and `callers` as an explicit subset, the delegation chain as one
Transaction Token across the trust domain with `via` on each operation
naming its immediate callers and a workload-identity proof on every
service, and the audience per hop as `aud` fixed per token kind, with the
confused-deputy red cases and a security event for every refusal. Items
1 to 3 came with 0.3's steps 3 and 4. Item 4 keeps clients, roles and
the trust domain in an identity module the other modules name by
reference (`<dependency>#/permissions/<name>`, `#/clients/<name>`,
`#/roles/<name>`, `trustDomain`), counted as unresolved while that
module is not given; finds the chain by induction through `calls` when
the modules are given together, a client named in `via` being the module
of its name; refuses two modules accepting one audience; derives the
scope case; and draws the identity module's matrix and each chain in the
techspec (`spec/tests/validate-caller-*`, `gaps-caller-alone`,
`document-techspec-caller-estate`). Item 5 reads `applicationChecks`,
`callerChecks`, `audienceChecks`, `eventWriters` and `clientArgument` in
`extract go`, and a grant's `client` and `through` in `extract
permissions` (`extract-go-caller-checks`,
`extract-permissions-client-through`), and the same checks in the swift,
javascript and dart readers (ADR-169, `extract-swift-caller-checks`,
`extract-javascript-caller-checks`, `extract-dart-caller-checks`). The
derived cases read the identity module given in the same run: the
`through` cases of its roles and the event clause with its identifier
under the trust domain (ADR-171, `derive-caller-estate`,
`validate-caller-identity-cases`, `validate-caller-identity-case-waits`).
A client named in `callers` by reference to the identity module gets the
same cases as a client of the module's own (ADR-171).
`specarch model` lists an operation's `via` and a role's `through` among
its references (`model-lists-via-and-through`).
`through` on a rule of the permission table, a pattern row included,
came in 0.7.3 (ADR-170, `extract-permissions-pattern-through`). The
swift, javascript and dart readers ask for the transaction token and
`via` of an operation whose handler reads the `Txn-Token` header, as
`extract go` does, in 0.7.4 (ADR-173, `extract-swift-txn-token`,
`extract-javascript-txn-token`, `extract-dart-txn-token`). Every item of
caller context is built.

Built, 0.7.2: the diagram keywords. An editor that draws every diagram
from a specification can draw the system context, the components of an
implementation, the deployment topology and the network, by the keywords
`docs/diagrams.md` decides (ADR-133 to ADR-139), built in its six items in
order: `specarch model`, one canonical JSON document a program reads the
specification through, the same bytes from both builds; `kind` and
`protocol` on a dependency and `carriedBy` on a channel, with the context
diagram in the technical specification's chapter 3 and the rule
`channel`; `dependsOn` on a layout entry, filled by `extract go` from the
imports, with the component diagram in chapter 5 and the rule
`depends_on`; `name`, `kind`, `runs` and `dependency` on a server, with the
deployment diagram in chapter 7 and the deployment guide and the rule
`server`; `zone`, `listens` and `links`, with the network diagram, the
handover's links and ports, the rule `link` and the warning
`link_missing`, the check that every declared call and data access has a
link, held by a must question that blocks the environment the
deployment installs (ADR-172); and a lane set and an id per element in the BPMN output and its SVG.
Every diagram has its marker (`context`, `components <stack>`,
`deployment <deployment>`, `network <deployment>`), and a file without the
keys renders as before. Of its gaps, the cross-system view was built
before (the estate's module map, ADR-113). The library-lending example
uses every key.

Built, 0.7.2: the gateway (`docs/gateway.md`, ADR-141 to ADR-143). A
gateway in front of an estate's services, which verifies the token, reads
one claim and forwards every request no operation answers to the service
the claim's value names, is `gateways` in the design: the token, the
claim, the forwards, the calls, the middlewares and the refusals, and no
permission, checked as the rule `gateway` in both builds, the forwards by
`aud` fixing the token's audience (`token_audience`), a gateway's link to
each forwarded server warned of by `link_missing`, and in an estate run
each forward checked against the audiences the module it names accepts
(`spec/tests/validate-gateway`, `validate-gateway-wrong`,
`validate-gateway-estate-wrong`). A gateway is a subject of tests with its
derived cases, drawn in the context diagram, given a section in chapter 6
of the technical specification and in the test plan, and named, not
described, by `generate openapi` (`document-techspec-gateway`,
`document-testplan-gateway`, `generate-openapi-gateway`). Its table of
services is printed by the project around `tools/gateways/dump-gateway.sh`
and read by `extract gateway`, with the deployment's servers under
`--implementation-out`; `check` compares its forwards by value and by
the dependency each goes to, and `merge` leaves out the Go reader's
fallback question a tree with a gateway answers, one gateway to one
question over the whole merge, keeping a catch-all question, or a second
code tree's, no gateway is left for open
(`extract-gateway`, `extract-gateway-value-twice`, `check-gateway-forward-removed`,
`check-gateway-forward-moved`, `merge-gateway-fallback`,
`merge-gateway-catch-all-kept`, `merge-gateway-two-trees`,
`merge-gateway-fallback-across-trees`). `extract go` reads a package-level string
constant as the literal it names and a package-level list assigned once as
a literal list, and asks a must question blocking `gateways` at every
fallback handler and catch-all route (BUG-028 and BUG-029 fixed;
`extract-go-routes-constants`, `extract-go-routes-fallbacks`). The library
estate's third module, `library-gateway`, forwards to the lending desk and
the catalogue service, its table extracted and checked in CI. Left out by
design: a prefix or a method list on a gateway, routing by a header,
rewriting the request, and a code reader of the forwarding, which waits
for an http binding that names the verifier and the lookup.

Built, 0.7.1 and 0.7.2: dxlib's encrypted API type and large files. An
endpoint whose API type makes its request and response an encrypted
envelope after a Diffie-Hellman key exchange is an operation with a
`handshake`, and one that uploads or downloads a file as a stream rather
than a body is an operation with a `stream`; the design declares the
handshakes in a section of their own and the streams with their direction
and limits (ADR-145, ADR-146;
`validate-handshake`, `validate-stream`, both validators). `extract go`
reads both from a dxlib registration's endpoint type and asks where the
type is computed or one dxlib never finished
(`extract-go-dxlib-envelope-stream`); the dxlib dialect writes each
operation's type and the go-dxlib target the exchange, the envelope hooks,
the stream helpers and a Go client that performs the handshake and streams
a file, proved over a real dxlib service (`generate-openapi-dxlib-envelope-stream`,
`generate-go-dxlib-envelope-stream`, `internal/gendxlib/roundtrip_test.go`);
the technical specification shows the handshake in five steps, each
encrypted operation's envelope and a streamed file in place of a body table
(`document-techspec-envelope-stream`). Section 5a lists the four parts
built after it.

## 5a. The handshake and the streams, end to end

Built, 0.7.2 unless named, each with its proof:

- **The handshake in the screens** (ADR-161). The TypeScript screens call
  a service in the dxlib dialect as it serves it, told the dialect by
  `apiDialect` in the plug-in request, and `screens/transport.ts` performs
  dxlib's pre-key handshake in the browser through the Web Crypto API,
  seals and opens the envelope and renews the handshake when the service
  answers that it is used up. A task page streams the file it picks to an
  upload, and an action saves the file a download answers. The round trip
  drives the transport on Node against the generated vault
  (`generate-ui-typescript-dxlib`, `generate-ui-typescript-dxlib-refused`,
  `validate-task-page-upload`).
- **The handshake and the streams in the plain JavaScript screens**
  (ADR-164, 0.7.3). Every call of the plain JavaScript pages goes through
  `transport.js`, the TypeScript transport in plain JavaScript, in either
  dialect, so a list, a form, a view and their actions call an encrypted
  operation in dxlib's envelope, a task page uploads the file it picks
  and an action saves a download. The round trip drives `transport.js`
  against the generated vault beside `transport.ts`
  (`generate-ui-dxlib`, `generate-ui-handshake-refused`).
- **The extract leg of the round trip.** The vault written by hand on dxlib
  (`extract-go-dxlib-vault`) is read with `extract go`, its questions are
  answered by decision records committed beside the tree
  (`generate-go-dxlib-extracted-vault`), and the service and client
  regenerated from it pass the same round trip as the vault written as a
  specification (`TestExtractedRoundTrip`).
- **An upload's parameters** (ADR-162). `stream.parameters` holds the
  values that travel beside the file, as one JSON object in a header the
  stack names, X-Var for dxlib. Both validators check it
  (`validate-stream-parameters`); the dxlib dialect writes it as
  `x-dxlib-parameters`, the go-dxlib handler reads it and its client sends
  it, `extract go` writes it from an upload stream's registration, the
  TypeScript task page sends its fields beside the file (an error in the
  standard dialect, which names no header) and the technical specification
  lists it. Both vault round trips upload with a required parameter.
- **The V3 and V4 envelopes** (ADR-163). The implementation file serves a
  handshake in dxlib's persistent-session types with the openapi target's
  `settings.envelopes`; the dxlib dialect writes the type, go-dxlib writes
  the hooks, which carry the pre-key envelope's bytes, and the clients
  renew on `REFRESH_SESSION` as on `REFRESH_PREKEY`
  (`generate-openapi-dxlib-session`, `generate-go-dxlib-session`, and the
  round trip run in V2, V3 and V4). `extract go` reads a V3 or V4 endpoint
  as the handshake `session` when the module's hooks open and seal dxlib's
  datablock (`extract-go-dxlib-session`).

Left out by design: a download stream in a V3 or V4 envelope, which no
dxlib type carries; hooks that make their own bytes, another cipher or a
bootstrap through the hook, which `extract go` asks about rather than
reads; and parameters beside a body read whole as raw bytes, which stay a
note.

## 6. Changes, defects, releases and operation

Built. The life after commissioning, designed in `docs/maintenance.md`:
change requests, defects, releases and incidents as records beside the
specification, and an operation stage with monitors. Built in this order:

1. the operation stage and its monitors (0.1.0,
   `spec/tests/validate-monitor-valid`);
2. records and their rules, including commissioning records checked
   against the checks that exist (0.3.0, `spec/tests/validate-records-valid`,
   `spec/tests/validate-commissioning-record`);
3. release records and the version rules (0.3.0,
   `spec/tests/validate-release-bump`, `records/releases/`);
4. `specarch diff <old> <new>`, which compares two versions of a
   specification and checks a release's version step (0.3.0,
   `spec/tests/diff-version-step`);
5. the documentor targets `changes` and `releases` (0.3.0,
   `spec/tests/document-writes-changes`,
   `spec/tests/document-writes-releases`).

## Not planned

A grammar or parser of SpecArch's own, a hosted service, or a promise of
complete generation. Algorithm bodies stay hand-written; generation stops at
what can be derived from the spec.
