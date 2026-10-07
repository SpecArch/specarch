# Generators

What every SpecArch emitter must do, and the pattern each target follows.
None of the patterns is new. Each was found running in production, in a tool
that already does one piece of the job well: sqlc for queries, protoc for
service contracts, the OpenAPI code generators that offer a strict server
mode, migration tools that only ever add a file. SpecArch copies those
patterns rather than inventing its own, and where a standard tool exists for
a step, the emitter stops at that tool's input and lets it do the rest.

## Rules for every emitter

1. One command, one folder. `specarch generate <target> <files>` writes
   into a folder the target owns and nothing outside it. Every generated
   file starts with a header naming the spec file, its `info.version` and
   the meta-model version it came from.
2. Generated output is never edited by hand. A change goes into the spec and
   the output is regenerated. `specarch generate <target> --check` regenerates
   into a temporary location and fails when the committed output differs.
   In CI that is the third gate in `docs/sync-gates.md`.
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

## Markdown technical specification

`specarch generate techspec` writes `<name>.techspec.md` for each design
file: the arc42 chapters of `docs/conventions.md`, each only when the design
has something for it, with the generated Mermaid diagrams and tables. The
design file gives every chapter but one; chapter 7, deployment and
implementation, comes from the implementation file (stack, libraries,
layout, mappings, bindings, generators, tasks, testing, deployments and the
implementation decisions) and is left out when none is given. The output
folder is `--out`, or the implementation file's `generators.techspec.output`,
read relative to that file.

The emitter also rewrites the generated diagrams and tables between the
markers of the hand-written `<name>.specarch-design.md` and leaves every
other line alone. A document with no markers gets nothing; a marker for an
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

The design file says what is tested; the implementation file says with
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

This target needs the `guard` concept, which meta-model 0.1 does not have.
It is a named 0.2 item in `docs/roadmap.md`, and the emitter follows it.

## Other formats

Another DSL or configuration format is emitted on request, limited to what
that format can execute. A concept it cannot represent is reported by name
(rule 4). A format that can only hold part of the spec gets a partial
document that says so in its header.

## Adding a generator

A new emitter is written against a real project, not against the example.
It is accepted when its output is what that project would have written by
hand, its `--check` form runs in that project's CI, and the matching gate in
`docs/sync-gates.md` is on. Generators are added one stack at a time, in the
order the real projects on the roadmap need them.
