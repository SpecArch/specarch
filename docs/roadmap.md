# Roadmap

Order of work after meta-model 0.1. Each step produces something the previous
one is checked by: the validator checks the meta-model, the generators check
the validator, and the real projects check all of it.

## 1. Validator CLI

Specified first, in SpecArch: `spec/specarch.specarch.yaml` is the design of
the `specarch` command and `spec/specarch.go.specarch-impl.yaml` the Go
implementation of it. The validator checks both files, which is the first
self-hosting check.

A single binary, `specarch validate <files>`, written in Go in this
repository. It checks definition and implementation files and does what
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
- the interface boundary: no stack-specific key in a definition file, no
  design in an implementation file, and an implementation file's
  `implements` and pointers resolve in its definition file.

It uses the same JSON Schema library the repository validates with today, so
the schema stays the single definition. It ships with a test suite of valid and
invalid files, and the repository's CI runs it on every example. The validator
is the first thing to be in version control and runnable by anyone, which the
lessons from an earlier in-house language made the first requirement.

## 2. Generators, in order of payoff

Each generator is one command, `specarch generate <target> <files>`, and
writes into a folder it owns. It reads the definition file for the design
and the implementation file for the target choices: the folder, the
downstream code generator and its settings, the type each field maps to. Generated files carry a header naming the spec
file and version they came from. A generator never edits a hand-written file;
Markdown documents are updated only between the `specarch:generate` markers.
Every emitter has a `--check` form that regenerates and fails on a
difference. The rules and the pattern each target follows are in
`docs/generators.md`.

1. Technical specification in Markdown, arc42 layout, with the generated
   Mermaid diagrams (entity, state, sequence, pages, permissions matrix).
   Highest payoff: it is what reviewers read, and it makes the YAML visible.
2. OpenAPI 3.1 document. Fields map one to one, since the keywords are
   OpenAPI's. Permissions become a security scheme plus a `x-specarch-permission`
   extension per operation. Server interfaces and types then come from a
   standard OpenAPI code generator per stack, not from SpecArch.
3. SQL migrations, new files only. The generator diffs the spec against
   the last generated snapshot and writes a forward migration. A deployed
   migration is never regenerated or edited, and a destructive step is its
   own file, so a live column changes by expand and contract across
   releases. Constraints become real database constraints; `check`
   expressions are translated, and a constraint the target cannot express
   fails generation rather than being dropped.
4. UI page definitions for the target component library: one generator
   per library, emitting the project's own component usage so generated
   screens look like the hand-built ones.
5. Other DSL formats on request, limited to what that DSL can execute:
   a concept the target cannot represent is reported, not silently omitted.
6. Test cases from worked examples, one test per example, in the target
   stack's test framework.
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
these first projects show which readers repeat; those become
`specarch extract <surface>` afterwards.

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

- `guards`: preconditions and postconditions on a data change, with exact
  expected counts; the input to generator 8;
- `separationOfDuties`: sets of permissions one holder must never have
  together; the validator checks that no role grants two of them;
- `flows`: a workflow binding (trigger, form schema, steps) as an object of
  its own, not an annotation on an endpoint.

Known candidates from the first meta-model:

- row-level permissions (a member sees only their own loans);
- cross-file references between bounded contexts;
- a fixed expression grammar;
- interfaces beyond HTTP, messaging and the command line, which 0.1 has:
  gRPC, file exchange, a Bluetooth or serial protocol, a menu-bar UI;
- value objects: data that is passed around but not stored and has no
  identity (a diagnostic, a request summary), now written as entities with a
  made-up key;
- sets and lists in the expression language, so a rule over many objects
  (every permission granted by some role) can be a formula rather than a
  count;
- background jobs and schedules;
- configuration and settings as a first-class concept.

## Not planned

A grammar or parser of SpecArch's own, a hosted service, or a promise of
complete generation. Algorithm bodies stay hand-written; generation stops at
what can be derived from the spec.
