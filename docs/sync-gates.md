# Sync gates

A specification that is not checked against the code drifts within months,
and a spec that has drifted is read by nobody. These checks run in CI on
every change and fail the build on any difference. Three comparisons cover
most of it.

## 0. The spec validates

Every specification and every implementation file passes
`specarch validate` with no error. The other gates assume a valid
spec.

## 1. Spec against the route table

A test builds the real router, the same object the service listens with,
lists its registered method and path pairs, and compares them with the
`paths` in the spec. Reading the built router is more reliable than parsing
source files: it sees the routes a framework adds by itself and the ones
mounted under a prefix.

The gate fails in both directions. A route the router serves that the spec
does not name is an endpoint with no permission, which fail-closed access
forbids. An operation in the spec that the router does not
serve is unimplemented and has no 501 stub. Path parameters are normalised
before comparison (`{id}` and `:id` are the same), and that is the only
normalisation.

## 2. Spec against the database

Apply every migration to a throwaway database, dump its catalogue (tables,
columns, types, nullability, primary and foreign keys, unique indexes, check
constraints), and compare it with the entities in the spec. Names on the
database side are derived with the rules in `docs/conventions.md`, the same
rules the migration emitter uses, so the two sides meet without a mapping
table.

Compare only what can be derived exactly. A column's type and nullability, a
key, a unique index and a check constraint's name are exact. The text of a
check expression is not, since the database normalises it, so that
comparison is a report. An index the spec does not know about is a report
too, until the meta-model has a word for it.

## 3. Generated output against the generator

Run every generator, then `git diff --exit-code`. Any change in the working
tree means the committed output was edited by hand or the spec moved without
a regeneration. This is the `--check` form of each emitter in
`docs/generators.md`, run for all of them at once.

The tests emitted from worked examples run with the rest of the test suite
and need no gate of their own.

## What a gate may compare

A gate that reports noise gets switched off, and then nothing is checked.
Each gate therefore compares only what an extractor can produce exactly, and
anything fuzzier is a report: printed in the build log, never a failure.
When a report keeps showing the same difference, the fix is to make that
comparison exact (a meta-model keyword, a naming rule) and promote it to the
gate, not to loosen the gate.

## Turning a gate on for an existing service

A service that predates its spec cannot pass gate 1 on day one. It is turned
on with an allowlist of the known differences, committed next to the test,
and the allowlist may only shrink: a change that adds a line to it fails.
New endpoints go spec first from the moment the gate is on; existing handlers
move onto the generated interface when the service is next touched, as
`docs/extraction.md` describes.
