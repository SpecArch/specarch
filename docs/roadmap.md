# Roadmap

Order of work after meta-model 0.1. Each step produces something the previous
one is checked by: the validator checks the meta-model, the generators check
the validator, and the real projects check all of it.

Each entry says where it stands in the code: built, with the release that
first carried it and the file or conformance case that shows it; partly
built, with what is missing; or not built. A release is a tag; 0.7.1 is the
latest.

## Releases

| Release | What it first carried |
|---|---|
| 0.1.0, 2026-10-07 | the validator in Go and in Swift (`cmd/specarch`, `swift/`) with the conformance suite `spec/tests/`; the documents techspec, requirements, test plan, traceability, deployment guide and commissioning procedure; the operation stage and its monitors |
| 0.2.0, 2026-10-08 | open questions, `specarch gaps`, the `questions` document, `specarch approve` and the gate on `generate` |
| 0.3.0, 2026-10-08 | `specarch derive`; `specarch diff`; change, defect, release and commissioning records with their rules; the `changes` and `releases` documents; `guard`, `dependencies`, `idempotencyKey`, `validity` and `session` |
| 0.4.0, 2026-10-08 | `specarch-gen-openapi`, with the dxlib dialect; `specarch-gen-sql`; `specarch-gen-go-dxlib`; `specarch-gen-tests-go`; `specarch idioms`; `views`, `jobs` and menus |
| 0.5.0, 2026-10-09 | `specarch extract` with the outline, database, router, documents, OpenAPI, permissions and pages readers; `specarch merge`; `specarch-gen-ui` for list pages in plain JavaScript; `specarch-gen-tests-swift` and `specarch-gen-tests-dart`; the page events, states and theme |
| 0.6.0, 2026-10-13 | meta-model 0.2 (`schema/specarch-design-0.3.schema.json`); the workflows reader; the Go reader on dxlib, `net/http` and the common routers; the Swift, JavaScript, Vue and Dart readers; `specarch-gen-ui-typescript` on Next.js and Carbon; `specarch-gen-bpmn`; the `problems` and `html` documents |
| 0.6.1 to 0.6.4, 2026-10-14 | the plans for the estate and for meta-model 0.3; the first estate points (a dump committed outside the code's repository, role names on the wire); a request body decoded into a struct in `extract go`; fixes to `merge` and the readers |
| 0.7.0, 2026-10-19 | the estate (`specarch-estate.yaml`, references between specifications, the module map); `specarch check`; `specarch facts`; `specarch-gen-sqlc`; indexes in `generate sql`; `implies`, `audience`, `middlewares`, the workflow table, documents in another language; the `handover` document |
| 0.7.1, 2026-10-20 | the techspec's parameters, request body and responses per operation; `extract go` reading a dxlib handler's decoded body and JSON answers where the registration declares none (ADR-140); every operation's handler followed into the code, recorded with its source and drawn as a flowchart (ADR-144); the designs of the diagram keywords (`docs/diagrams.md`) and of the gateway (`docs/gateway.md`), proposed; dxlib's encrypted API type as a handshake an operation names and large files as a stream it declares (ADR-145, ADR-146), read by `extract go`, served by the dxlib dialect and the go-dxlib target with a Go client, and shown in the techspec; this roadmap matched to the code |
| 0.7.2, open | `specarch-gen-ui-swiftui`, the screens for the iPhone in SwiftUI (ADR-147), with `generate` finding a plug-in by the target's framework before the file's language, and the pages read once for both UI generators that write a page as data (`internal/genscreens`) |

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
has the `validate` and `version` verbs.

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
repository's CI runs `specarch validate spec examples` on every push. The
validator is the first thing to be in version control and runnable by
anyone, which the lessons from an earlier in-house language made the first
requirement.

## 2. Targets, in order of payoff

Documents are `specarch document <target> <folders>`, built into the
program. Built: techspec, requirements, test plan, traceability, deployment
guide and commissioning procedure (0.1.0), questions (0.2.0), changes and
releases (0.3.0), problems and the HTML site (0.6.0), and the handover
(0.7.0); `document --audience external` leaves out the internal elements
(0.7.0). Not built: the manual and the operations guide, which wait for the
specification to hold the task-by-task and operational content they need.
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
use. The rules and the pattern each target follows are in
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
   - Partly built, 0.5.0, for the web in plain JavaScript:
     `specarch-gen-ui` writes list pages (ADR-040, `spec/tests/generate-ui`,
     `examples/library-lending/web/`). Not built: forms, views, navigation
     between pages, and the first real project's screen to check it against.
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
7. Not built: the user manual, the operations guide and other project
   documents. Pages and permissions give the manual its structure;
   endpoints, channels and deployment notes give the operations guide its
   checklist. The handover document (0.7.0) says what a customer who runs
   the system receives.
8. Partly built: guarded operational scripts for data changes on live
   systems: preconditions, dry run by default, postconditions with exact
   counts, idempotent. Built, 0.3.0: the `guard` on an operation or a
   command they are emitted from (`spec/tests/validate-guard`). Not built:
   the generator that writes the scripts.

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
`approve` and the gate on `generate` are built (0.2.0). Not built:
`specarch decide`, the dispatcher reading `problems.sarif` for the agent
queue, and the comparison of the old document with the refined one. A
system with both documents and code is written by the procedure in
`docs/from-sources.md`, and `gaps` shows which section of each source
produced which elements.

Candidates, in rough order of size: a menu-bar agent manager (macOS), a home
solar monitoring system (Go services on a small board plus an iPhone app), a
phone browser, a code editor with a remote-control protocol. The remote-control
protocol is the interesting one: it is shared by two applications, so its
specification must be importable by both.

## 5. Meta-model 0.2 and after

Meta-model 0.2 is built, 0.6.0 (`schema/specarch-design-0.3.schema.json`),
driven by the gaps found in step 4 and by the first request for generated
screens. `docs/meta-model-0.2.md` is the plan: what is taken, what waits
and why, and the steps that build it, each with what done looks like.
Built, in the order they were built:

- `separationOfDuties`: sets of permissions one holder must never have
  together; the validator checks that no role grants them
  (`spec/tests/validate-separation-of-duties`);
- task pages, a page that submits to an operation without loading a
  record (sign-in, a second factor; `spec/tests/validate-task-pages`);
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

Not built: meta-model 0.3, the caller as something an expression can name,
and what rests on it: the rows a role reaches (a member sees only their own
loans), machine clients and the gates on a route that are not a permission,
and token formats. `docs/meta-model-0.3.md` is the plan: what is taken,
what waits and why, and five steps, each with what done looks like. Not
planned until a project asks: interfaces beyond HTTP, messaging and the
command line; sets and lists in the expression language; seed data per
profile; more than one deliverable per specification.

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

Not built: caller context. The same estate's services call each other on a
person's behalf, and `docs/caller-context.md` designs the four inputs of an
authorisation decision beyond the person and the roles, decided on
2026-10-14 (ADR-106 to ADR-111): the application the person signed in
through as a condition on a grant (`kind`, `softwareVersion` and
`softwareStatement` on a client keyed by its `software_id`, `through` on a
role, checked at every hop), the calling service with grants of its own and
`callers` as an explicit subset, the delegation chain as one Transaction
Token across the trust domain with `via` on each operation naming its
immediate callers and a workload-identity proof on every service, and the
audience per hop as `aud` fixed per token kind, with the confused-deputy
red cases and a security event for every refusal. Clients, roles and the
trust domain live in an identity module. It amends 0.3's steps 3 and 4
before they are built and ends with five build items in order.

Designed in 0.7.1, not built: the diagram keywords. An editor that draws
every diagram from a specification could not draw the system context, the
components of an implementation, the deployment topology or the network,
and `docs/diagrams.md` decides the keywords those pictures need, proposed
for the owner's approval (ADR-133 to ADR-139): which package depends on
which, what a dependency is and how it is reached, what a server is and
what runs on it, zones, ports and allowed links with a check that every
declared call has a link, the model a program reads the specification
through, and lanes and ids in the BPMN output. Of its gaps, the
cross-system view is built (the estate's module map, ADR-113). It ends with
six build items in order.

Designed in 0.7.1, not built: the gateway. A gateway in front of an
estate's services, which verifies the token, reads its audience and
forwards every other request to the service the audience names, has no
element, no reader for its table of services, and a Go reader that asks at
every route registered through the module's own constants;
`docs/gateway.md` decides the three, proposed for the owner's approval
(ADR-141 to ADR-143): `gateways` in the design with the token, the claim,
the forwards and the refusals and no permission, the forwards fixing the
token's audience; a printed gateway table read by `extract gateway` and
compared by `check`; and the Go reader reading a package-level constant as
a literal and asking at every fallback handler and catch-all (BUG-028,
BUG-029, open). It ends with three build items in order, the reader fixes
first and the element after 0.3's tokens and the diagram keywords'
dependency and server kinds.

Partly built, 0.7.1: dxlib's encrypted API type and large files. An
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
(`document-techspec-envelope-stream`). Four parts are not built, in 5a.

## 5a. What the handshake and the streams still owe

The design declares the handshakes a client performs before it may call
an encrypted operation and the files an operation streams (ADR-145,
ADR-146); `extract go` reads both from a dxlib registration, the dxlib
dialect and the go-dxlib target serve them with a Go client, and the
documents show them. Four parts are not built, each with what done looks
like:

- **The handshake in the screens.** Not built. The two UI targets, plain
  JavaScript and TypeScript on Next.js, refuse a page that submits to,
  loads from or picks from an encrypted or streamed operation, as an error
  at the page. Done: a browser client of the handshake in the TypeScript
  screens' service module, X25519 and Ed25519 through the Web Crypto API
  where the browser has them (both are in current browsers) and
  AES-256-CBC through `subtle.encrypt`, the length-prefixed envelope
  dxlib's datablock packs, a handshake established on the first encrypted
  call and renewed when the service answers `REFRESH_PREKEY`, and the same
  for a streamed upload (`fetch` with a `ReadableStream` body) and
  download; the round trip the Go client passes, a generated screen
  against a generated service, is the acceptance test.
- **The extract leg of the round trip.** Not built. The round trip proved
  runs from a written specification to a generated service and client
  over a real dxlib API (`internal/gendxlib/roundtrip_test.go`); a tree
  `extract go` reads from a service's code carries must questions on
  every operation (whether the running system registers it, among
  others), which hold `generate` back until they are answered. Done: a
  fixture service read with `extract go`, a decisions file committed
  beside the extracted tree that answers its must questions, `generate`
  run on that tree, and the regenerated service and client passing the
  same round-trip test as the vault fixture, which shows that code read
  into a specification regenerates to code that behaves the same.
- **An upload's parameters.** Not built. dxlib reads the parameters of an
  upload stream from the request's `X-Var` header, a JSON object sent
  beside the streamed body, and the meta-model has no word for a
  parameter carried that way, so the dxlib dialect and the go-dxlib target
  refuse an upload that declares parameters, as an error at the operation,
  and the Go client takes none. Done: a meta-model word for where an
  upload's parameters travel (a header holding them as JSON, named by the
  stack), the dialect writing them as dxlib's parameters, the go-dxlib
  handler reading them from the header, the client sending it, and
  `extract go` writing them from the registration.
- **The V3 and V4 envelopes.** Not built. dxlib routes the body of an
  endpoint of type `EndPointTypeHTTPEndToEndEncryptionV3` or `V4`, a
  persistent-session envelope, to hooks the service supplies
  (`OnE2EEV3Unpack` and `OnE2EEV3Pack`, and the V4 pair) and fixes
  nothing of the exchange, the cipher or the session store, so `extract
  go` asks a must question at the operation's handshake and the generators
  serve neither type. Done: a handshake declared with the key agreement,
  signature and cipher the hooks use and the operation that establishes
  it, `extract go` reading the hooks the service registers to answer that,
  the dxlib dialect writing the type, the go-dxlib target writing the
  hooks and the client's side, and a round trip over each.

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
