# Roadmap

Order of work after meta-model 0.1. Each step produces something the previous
one is checked by: the validator checks the meta-model, the generators check
the validator, and the real projects check all of it.

## 1. Validator CLI

Specified first, in SpecArch: `spec/` is the specification of the `specarch`
command, every stage of it, and `spec/implementation/go/` the Go
implementation of it. The validator checks the tree, which is the first
self-hosting check.

The validator has two implementations of the one specification: Go, for
macOS, Linux, Windows and Android, and Swift, for macOS, each with its own
implementation file. They prove the split: one design, two languages, and
nothing in the design changed for the second. Both pass the same
conformance suite, the tests stage `spec/tests/`, which holds only inputs
and the expected exit status, output and files, with no code from either
language. The documents and generators are built in Go.

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

It uses the same JSON Schema library the repository validates with today, so
the schema stays the single definition. It ships with a test suite of valid and
invalid files, and the repository's CI runs it on every example. The validator
is the first thing to be in version control and runnable by anyone, which the
lessons from an earlier in-house language made the first requirement.

## 2. Targets, in order of payoff

Documents are `specarch document <target> <folders>`, built into the
program, one per stage of the life cycle: techspec, requirements, test
plan, traceability, deployment guide and commissioning procedure (built),
then the manual and the operations guide, once the specification holds the
task-by-task and operational content they need. Code is `specarch generate <target> <folders>`, each
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
difference. The rules and the pattern each target follows are in
`docs/generators.md`.

1. Technical specification in Markdown, arc42 layout, with the generated
   Mermaid diagrams (entity, state, sequence, pages, permissions matrix).
   Highest payoff: it is what reviewers read, and it makes the YAML visible.
2. Built, standard dialect: `specarch-gen-openapi` writes the OpenAPI 3.1
   document. Fields map one to one, since the keywords are OpenAPI's.
   Permissions become the security scheme the target names plus a
   `x-specarch-permission` extension per operation; refusals are RFC 9457
   problem documents; lists page through the paginated-list idiom. Server
   interfaces and types then come from a standard OpenAPI code generator
   per stack, not from SpecArch. The dxlib dialect comes with go-dxlib.
3. Built: `specarch-gen-sql`, the first migration, the snapshot and the
   differ; indexes from a mapping's settings are next. SQL migrations, new files only, in PostgreSQL by
   default; the other dialects render through the type-rendering idiom of
   `docs/idioms.md`,
   with the table in `docs/dxlib-lessons.md`. The generator diffs the spec against
   the last generated snapshot and writes a forward migration. A deployed
   migration is never regenerated or edited, and a destructive step is its
   own file, so a live column changes by expand and contract across
   releases. Constraints become real database constraints; `check`
   expressions are translated, and a constraint the target cannot express
   fails generation rather than being dropped.
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
   its `page.tsx` and no bus is written (`docs/ui-nextjs-carbon.md`). The design keywords a generator
   reads are built (`docs/ui-design.md`): page events and flows, states,
   sections, compact columns, the accessibility target and the theme. Built
   for the web: `specarch-gen-ui` writes list pages in plain JavaScript
   (ADR-040). Next for it: forms, views, navigation between pages, and the
   first real project's screen to check it against. Planned for the web
   on Next.js and Carbon, for a project whose screens are built there:
   `specarch-gen-ui-typescript`, in the steps of `docs/ui-nextjs-carbon.md`,
   with the components as an idiom a project overrides. SwiftUI follows.
5. Other DSL formats on request, limited to what that DSL can execute:
   a concept the target cannot represent is reported, not silently omitted.
6. Tests from the specification. The specification gives the business cases:
   every design test (golden and red, given, when and then) and every worked
   example. The implementation file gives the target: the test framework,
   the suites, fixtures and how each suite runs. One generated test per
   design test and per worked example, with the given, when and then written
   into the test as its steps. `docs/test-generation.md` designs the step
   before it: `specarch derive` writes the derived cases as tests, choosing
   the red paths by the harm a requirement names and by how often users
   make the mistake, with the cases left out listed in the test plan; and
   structured `fixture`, `input` and `expect` on a test give the generator
   something to run.
7. User manual, operations guide and other project documents: pages and
   permissions give the manual its structure; endpoints, channels and
   deployment notes give the operations guide its checklist.
8. Guarded operational scripts for data changes on live systems:
   preconditions, dry run by default, postconditions with exact counts,
   idempotent. Emitted from the `guard` on an operation or a command,
   which is built.

"Any language or framework" means any stack that has a generator written for
it. Generators are added one stack at a time, driven by the real projects
below.

## 3. Sync gates in CI

The checks that keep a spec and its code equal, in `docs/sync-gates.md`:
the spec validates, the built router's routes match `paths` in both
directions, a database built from every migration matches `entities`, and
regenerating everything leaves the working tree unchanged. Each gate compares
only what can be derived exactly and reports the rest. The gates are written
as reusable test helpers, one per stack, alongside that stack's first
generator, and run in this repository's CI against the examples as soon as
the first generator exists.

## 4. The owner's finished applications as specifications

The first real test. Each finished personal application in the MyOwnApps
collection is written as a specification by extraction, following
`docs/extraction.md`: find where the running system reads each fact, extract
mechanically, regenerate what the generators can produce, compare it with the
code, and only then review by hand. The comparison is recorded, and so is
every concept the code has that the meta-model cannot express. Those gaps,
collected across the applications, are the input to meta-model 0.2. Being
personal projects, the resulting specifications can be published here as
examples.

Until its readers are built, extraction is a manual method with scripts
kept in each project. `specarch extract <source>` is built one reader at a
time for the first real project that needs it, in the steps of
`docs/extraction.md`, Building extract. What the extraction cannot settle becomes open questions, and the
path from there to approved code is `docs/refinement.md`: `gaps`, the
decisions that answer questions, `approve` and the gate on `generate` are
built; `specarch decide`, `gaps --json` for the agent queue, and the
comparison of the old document with the refined one are its next items.
A system with both documents and code is written by the procedure in
`docs/from-sources.md`, and `gaps` shows which section of each source
produced which elements.

Candidates, in rough order of size: a menu-bar agent manager (macOS), a home
solar monitoring system (Go services on a small board plus an iPhone app), a
phone browser, a code editor with a remote-control protocol. The remote-control
protocol is the interesting one: it is shared by two applications, so its
specification must be importable by both.

## 5. Meta-model 0.2

Driven by the gaps found in step 4 and by the first request for generated
screens. `docs/meta-model-0.2.md` is the plan: what is taken, what waits
and why, and the steps that build it, each with what done looks like.
Taken, in the order they are built:

- `separationOfDuties` (built): sets of permissions one holder must never
  have together; the validator checks that no role grants them;
- task pages, a page that submits to an operation without loading a
  record (sign-in, a second factor);
- `workflows`: a request that finishes after people approve it (trigger,
  form, approval and operation steps, deadlines), as an object of its own,
  not an annotation on an endpoint; `flows` remains a person's navigation
  across pages;
- the page elements of a back-office screen: lookup fields, row actions
  that depend on the row, a confirmation with a reason, a field's mode,
  checks across fields, child rows, and maker-checker on a page;
- value objects, `schemas`: data passed around but not stored;
- a unique constraint with a condition, an element served only behind a
  setting, a requirement's target release, and a name on the wire;
- a schema per fragment file;
- missing test scenarios as errors, the one change that breaks a 0.1 file
  and so the one that moves the version, built last;
- the workflows reader of `specarch extract`, after the workflow object.

Built before 0.2 and kept in 0.1, since none breaks an existing file:
`guard`, the input to generator 8, whose postcondition is the generated
script's to check; a read model (`views`); background jobs and schedules
(`jobs`) and the other keywords `docs/dxlib-lessons.md` found needed: the
sensitivity of a field, encryption at rest, audited entities and soft
delete, list operations, a problem catalogue for error responses, limits
on an operation, and menus; and the concepts the red paths of
`docs/test-generation.md` needed, `dependencies` with a time limit per
call, `idempotencyKey`, `validity` on an entity and `session`.

Waiting for 0.3, around the caller as something an expression can name:
row-level permissions (a member sees only their own loans), gates by
client identity or a signed request, and token formats. Not planned until
a project asks: interfaces beyond HTTP, messaging and the command line;
sets and lists in the expression language; seed data per profile; more
than one deliverable per specification.

## 6. Changes, defects, releases and operation

The life after commissioning, designed in `docs/maintenance.md`: change
requests, defects, releases and incidents as records beside the
specification, and an operation stage with monitors. Built in this order:

1. the operation stage and its monitors;
2. records and their rules, including commissioning records checked
   against the checks that exist;
3. release records and the version rules;
4. `specarch diff <old> <new>`, which compares two versions of a
   specification and checks a release's version step;
5. the documentor targets `changes` and `releases`.

## Not planned

A grammar or parser of SpecArch's own, a hosted service, or a promise of
complete generation. Algorithm bodies stay hand-written; generation stops at
what can be derived from the spec.
