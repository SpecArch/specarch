# Roadmap

Order of work after meta-model 0.1. Each step produces something the previous
one is checked by: the validator checks the meta-model, the generators check
the validator, and the real projects check all of it.

## 1. Validator CLI

A single binary, `specarch validate <files>`, written in Go in this
repository. It does what JSON Schema cannot:

- cross-references: relation targets, primary-key fields, page columns and
  fields, transition states, requirement prefixes, `emits` targets, `source`
  and `submit` operation IDs, `algorithm` names;
- the expression language of `check` constraints and formulas, parsed, not
  pattern-matched;
- worked examples: the formula is evaluated on each example's inputs and must
  produce the expected value, so a wrong example fails validation rather than
  becoming a wrong test;
- fail-closed access: every operation and page has a permission that exists,
  every permission is granted by at least one role or is `public`.

It uses the same JSON Schema library the repository validates with today, so
the schema stays the single definition. It ships with a test suite of valid and
invalid files, and the repository's CI runs it on every example. The validator
is the first thing to be in version control and runnable by anyone, which the
lessons from an earlier in-house language made the first requirement.

## 2. Generators, in order of payoff

Each generator is one command, `specarch generate <target> <files>`, and
writes into a folder it owns. Generated files carry a header naming the spec
file and version they came from. A generator never edits a hand-written file;
Markdown documents are updated only between the `specarch:generate` markers.

1. Technical specification in Markdown, arc42 layout, with the generated
   Mermaid diagrams (entity, state, sequence, pages, permissions matrix).
   Highest payoff: it is what reviewers read, and it makes the YAML visible.
2. OpenAPI 3.1 document. Fields map one to one, since the keywords are
   OpenAPI's. Permissions become a security scheme plus a `x-specarch-permission`
   extension per operation. Server interfaces and types then come from a
   standard OpenAPI code generator per stack, not from SpecArch.
3. SQL migrations, new files only. The generator diffs the spec against
   the last generated snapshot and writes a forward migration. A deployed
   migration is never regenerated or edited. Constraints become real database
   constraints; `check` expressions are translated, and a constraint the
   target cannot express fails generation rather than being dropped.
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

"Any language or framework" means any stack that has a generator written for
it. Generators are added one stack at a time, driven by the real projects
below.

## 3. The owner's finished applications as specifications

The first real test. Each finished personal application in the MyOwnApps
collection is written as a specification by extraction: read the code, write
the as-built SDF, regenerate what the generators can produce, and compare it
with the code. The comparison is recorded, and so is every concept the code
has that the meta-model cannot express. Those gaps, collected across the
applications, are the input to meta-model 0.2. Being personal projects, the
resulting specifications can be published here as examples.

Candidates, in rough order of size: a menu-bar agent manager (macOS), a home
solar monitoring system (Go services on a small board plus an iPhone app), a
phone browser, a code editor with a remote-control protocol. The remote-control
protocol is the interesting one: it is shared by two applications, so its
specification must be importable by both.

## 4. Meta-model 0.2

Driven by the gaps found in step 3. Known candidates already:

- row-level permissions (a member sees only their own loans);
- cross-file references between bounded contexts;
- a fixed expression grammar;
- non-HTTP interfaces (a Bluetooth or serial protocol, a menu-bar UI);
- background jobs and schedules;
- configuration and settings as a first-class concept.

## Not planned

A grammar or parser of SpecArch's own, a hosted service, or a promise of
complete generation. Algorithm bodies stay hand-written; generation stops at
what can be derived from the spec.
