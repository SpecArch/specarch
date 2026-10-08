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
   how the web implementation carries them. The design keywords a generator
   reads are built (`docs/ui-design.md`): page events and flows, states,
   sections, compact columns, the accessibility target and the theme. The
   first generator waits for the owner's pick of stack; SwiftUI is
   recommended, since hand-built screens to reproduce exist there.
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
   idempotent. Waits on the `guard` concept in meta-model 0.2.

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

Extraction stays a manual method with scripts kept in each project until
these first projects show which readers repeat; those become the readers of
`specarch extract <source>`, the verb that is designed in `spec/` and built
then. What the extraction cannot settle becomes open questions, and the
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

Driven by the gaps found in step 4. Extraction of existing systems has
already asked for three concepts, and they go first. Each is a new object
that only the validator CLI or an emitter can enforce, which is why they wait
for 0.2 rather than being written into 0.1 unchecked:

- `guard`: built, as a precondition and an exact record count checked with
  a data change on an operation or a command; the input to generator 8;
- `separationOfDuties`: sets of permissions one holder must never have
  together; the validator checks that no role grants two of them;
- `flows`: a workflow binding (trigger, form schema, steps) as an object of
  its own, not an annotation on an endpoint.

- missing test scenarios as errors: in 0.1 the validator warns for every
  derived case no test covers; in 0.2 that becomes an error, once real
  specifications show the derivation is right.

Other known candidates:

- row-level permissions (a member sees only their own loans);
- a schema per fragment file, so an editor can validate one file of a tree
  on its own;
- missing golden tests of a designed but unbuilt command, so the validator
  stops warning about `extract` without a test that cannot run;
- a fixed expression grammar;
- interfaces beyond HTTP, messaging and the command line, which 0.1 has:
  gRPC, file exchange, a Bluetooth or serial protocol, a menu-bar UI;
- value objects: data that is passed around but not stored and has no
  identity (a diagnostic, a request summary), which 0.1 writes as entities
  with a made-up key;
- sets and lists in the expression language, so a rule over many objects
  (every permission granted by some role) can be a formula rather than a
  count;
- built: a read model (`views`), an entity's row with fields read through
  its relations and counts added, never written;
- built: background jobs and schedules (`jobs`), and the other keywords
  `docs/dxlib-lessons.md` found needed: the sensitivity of a field,
  encryption at rest, audited entities and soft delete, list operations,
  a problem catalogue for error responses, limits on an operation, and
  menus;
- configuration and settings as a first-class concept;
- built: the concepts the red paths of `docs/test-generation.md` needed,
  `dependencies` with a time limit per call, `idempotencyKey`, `validity`
  on an entity, `session`, and the guard above, each with its derived
  cases; a postcondition on a guard, which a test cannot set up from
  outside, is still open.

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
