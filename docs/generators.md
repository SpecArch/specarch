# Targets

What every SpecArch target must do, document or code, and the pattern each
one follows. `specarch document <target>` writes a document; `specarch
generate <target>` writes code or data, through a plug-in.
None of the patterns is new. Each was found running in production, in a tool
that already does one piece of the job well: sqlc for queries, protoc for
service contracts, the OpenAPI code generators that offer a strict server
mode, migration tools that only ever add a file. SpecArch copies those
patterns rather than inventing its own, and where a standard tool exists for
a step, the emitter stops at that tool's input and lets it do the rest.

## Rules for every emitter

1. One command, one folder. `specarch document <target> <folders>` and
   `specarch generate <target> <folders>` write into a folder the target
   owns and nothing outside it: the folder the implementation file names
   under `targets`, or `--out`. Every generated file starts with a header
   naming the root file, its `info.version` and the meta-model version it
   came from.
2. Generated output is never edited by hand. A change goes into the spec and
   the output is regenerated. With `--check` the output is made in memory
   and compared with the disk, and the run fails when it differs. In CI
   that is the third gate in `docs/sync-gates.md`.
3. Hand-written files are never touched. The one exception is a Markdown
   document, which is rewritten only between its `specarch:generate`
   markers.
4. What the target cannot express fails generation. A check constraint the
   database cannot evaluate, a decimal the language has no type for, a page
   kind the component library does not render: each is an error naming the
   object. There is no warning-only mode, because an omission that is only
   warned about is the same as a silent one by the next release.
5. One reviewed input becomes one typed output, and the input is the thing
   under review. This is the sqlc rule: the query is the spec, the function
   is derived, and nobody reviews the function. For SpecArch the YAML is the
   input and the review happens there.

## Plug-ins

A code target that `specarch` does not build in is produced by the
executable `specarch-gen-<target>` found on PATH, the pattern of
`protoc-gen-*`, `git-*` and `kubectl-*`, with protoc's protocol: the plug-in
never touches the disk. `specarch` validates the specification, refuses
while a must or should question blocks a section the target reads (the
sections the implementation file names under `targets.<target>.reads`, or
every section when it names none) and refuses without an approval record
of the files as they are unless `--unapproved` is given (`docs/refinement.md`),
then runs the plug-in once per specification, and writes on its standard
input one JSON object:

| Key | Holds |
|---|---|
| `specarch` | the meta-model version |
| `target` | the target name |
| `root` | the root file's path |
| `specification` | the merged, validated specification as plain values |
| `implementations` | one object per implementation file: `file`, `content` (the file as plain values) and `settings` (the target's settings from it, when any) |
| `output` | the folder the files are for |

The plug-in answers on its standard output with one JSON object: `files`,
each a `path` relative to the output folder and its `content`, and
`diagnostics`, each with the validator's fields (`file`, `line`,
`severity`, `path`, `rule`, `message`), and exits 0. `specarch` prints the
diagnostics, refuses a path outside the output folder, and writes or
compares the files itself, so `--check` and rule 1 hold for every plug-in
without each one implementing them. A plug-in that exits with another
status, or answers with anything else, fails the run with status 2; its
standard error is passed through. The request and answer structs are in
`cmd/specarch/generate.go`.

## Markdown technical specification

`specarch document techspec` writes `techspec.md` for a specification: the
arc42 chapters of `docs/conventions.md`, each only when the specification
has something for it, with the generated Mermaid diagrams and tables.
Chapter 2 comes from the constraints and assumptions, chapter 7 from the
deployment and commissioning stages and then from each implementation file
(stack, libraries, layout, mappings, bindings, targets, tasks, testing,
deployments and the implementation decisions), chapter 12 from the
glossary, and chapter 13 from the needs and requirements with the
traceability matrix of what satisfies and what verifies each requirement.

## The other documents

`specarch document requirements` writes `requirements.md`: purpose and
scope, the stakeholders, the needs with the requirements that refine them,
each requirement with its attributes and acceptance criteria, then the
constraints, assumptions and glossary. `testplan` writes `testplan.md`: the
count of golden and red tests, the levels, how each implementation's suites
run them, and every design test as a test case under its subject.
`traceability` writes `traceability.md`: needs to requirements,
requirements to what satisfies and verifies them, and the gaps.
`deployment` writes `deployment.md`: the environments and the path a
release takes, the settings, each installation of each implementation with
its servers and setting values, a secret only named, then release, rollback
and migrations. `commissioning` writes `commissioning.md`: the checks by
environment in the order a release reaches them, a Result column for every
step, and the sign-off sheet. `questions` writes `questions.md`: the open
questions by stage with what each blocks and who decides, and a table of
the outputs, each ready, a draft or waiting; `specarch gaps` prints the
same text and exits 1 while a must or should question is open. `manual`
and `operations` wait until the specification holds what they need.

In every document, an element's `why` is a paragraph that starts with
**Insight:** and each of its citations one that starts with **Note:**,
under the element's heading, or after the table when the element is a row,
labelled with the row's name. Before them, an element that says how it is
known gets one line, **Origin:**, and an element an open question blocks
gets one paragraph per question, **Open question Q-12 (must, decision):**.
A document whose sections a must or should question blocks starts with a
**Draft:** notice under its summary. A document that cites sources ends
with a table of them.

The techspec target also rewrites the generated diagrams and tables between the
markers of the hand-written `specarch.md` beside the root file and leaves
every other line alone. A document with no markers gets nothing; a marker for an
object that does not exist is an error, not an empty block, so a deleted
entity cannot leave a stale diagram behind.

## OpenAPI document and the server interface

The emitter writes an OpenAPI 3.1 document, field for field, since the
keywords are OpenAPI's own. Permissions become one security scheme plus an
`x-specarch-permission` extension on every operation.

Server code is not SpecArch's job. A standard OpenAPI code generator for the
stack turns the document into a typed server interface with request and
response types, and hand-written handlers implement that interface. The
compiler then refuses a handler whose signature does not match the spec. Use
the generator's strict mode where it has one: decoding the request and typing
the response move into generated code, so a handler cannot return a status
or a body the spec does not list.

The contract is fixed before either side is built. An operation may be
answered with 501 Not Implemented while its contract is final; the client is
built against the contract in the meantime, and the route-table gate sees the
stub as a served route. A list of stubs is a report, not a failure.

Where services talk to each other through an interface definition language
such as protobuf, the same rule applies: the interface file is the contract,
reviewed and fixed first, and the rest is derived. An emitter for such a
format is generator 5 on the roadmap, written when a real project needs it.

## SQL migrations

Migrations are new files only. The emitter keeps a snapshot of the spec as it
stood after the last generated migration, diffs the current spec against it,
writes one new numbered forward migration, and updates the snapshot in the
same run. The snapshot is committed with the migration. A migration that has
been applied anywhere is never regenerated or edited: regenerating it would
change history that a database has already recorded.

A destructive step (dropping a column or a table, narrowing a type) is
emitted only when asked for by a flag, and always as a file of its own. That
keeps the expand-contract pattern honest for a column that live code reads:
add the new column in one release, backfill it, switch the code, and remove
the old column in a later release, each in its own migration. The emitter
cannot tell a rename from a drop and an add, so a rename is written as those
two releases.

Constraints become real database constraints. A `check` expression is
translated into the target's dialect; one the target cannot express fails
generation (rule 4). Names are derived with the rules in
`docs/conventions.md`, so the database gate can derive them the same way.

## UI page definitions

Where the target project already renders lists, forms and views from a schema
object that its component library reads, the emitter writes that schema
object and nothing else. The screen then looks like every other screen in
the project, because the same library draws it. Only when no such layer
exists does the emitter write components, and then it uses the project's own
components in the project's own style.

An emitter that writes whole pages in a state-management style the project
does not use is dead on arrival; one such generator was found in a codebase,
unused, next to the screens people had written by hand instead. The first
emitter for a component library is therefore written against one hand-built
screen in a real project and has to reproduce it before it is accepted.

## Tests from the specification

The specification says what is tested; the implementation file says with
what. One test per worked example and one per design test, in the
framework the implementation file's `testing` names, each named after its
example or test.

A worked example's test calls the hand-written algorithm body with the
example's `inputs` and asserts the `expected` value, with every value in its
declared type: decimals as decimals, never as floating point. The validator
has already checked that the formula gives `expected` on those inputs, so a
generated test that fails points at the body, not at the spec.

A design test becomes a test with three marked steps, its `given`, `when`
and `then`, and a body the implementation fills in where the sentences
cannot be turned into code. Its subject and scenario go into the test's
name, so a failing red test says which refusal broke. A design test marked
`notApplicable` becomes no test; its reason is printed in the generated
file's header.

## Guarded operational scripts

A data change run against a live system (a new role and its permissions, a
backfill, a one-off correction) is safer emitted than typed. The emitted
script has preconditions that raise when the system is not in the expected
state, a dry run as the default, postconditions that check exact row counts,
and no effect on a second run.

The `guard` on an operation or a command (`docs/conventions.md`, Guards)
is what the script is emitted from: its `precondition` becomes the check
that raises, its `recordsChanged` the exact count the postcondition
asserts. The emitter is a later item.

## Other formats

Another DSL or configuration format is emitted on request, limited to what
that format can execute. A concept it cannot represent is reported by name
(rule 4). A format that can only hold part of the spec gets a partial
document that says so in its header.

## Adding a target

A new target is written against a real project, not against the example.
It is accepted when its output is what that project would have written by
hand, its `--check` form runs in that project's CI, and the matching gate in
`docs/sync-gates.md` is on. Code targets are plug-ins, added one stack at a
time, in the order the real projects on the roadmap need them; document
targets are built into `specarch`, one per stage of the life cycle.
