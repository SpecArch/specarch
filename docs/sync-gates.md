# Sync gates

A specification that is not checked against the code drifts within months,
and a spec that has drifted is read by nobody. These checks run in CI on
every change and fail the build on any difference. Three comparisons cover
most of it. `specarch check <specification> <dump>...` runs gates 1 and 2,
and compares the grants as well; `generate --check` and `document --check`
are gate 3 (ADR-124).

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
normalisation: the route table's printer writes every parameter `{name}`.

`specarch check` reads the route table with the router reader and matches
an operation by its method and path. An operation the router does not serve
is `operation_not_served` at the operation, a route the spec does not name
is `route_not_specified` at its line in the route table, and a permission
that differs is `operation_permission_differs`. With the permission table it
also matches each grant by its role and permission, each side widened
through `implies`, as `grant_not_in_code` and `grant_not_specified`.

## 2. Spec against the database

Apply every migration to a throwaway database, dump its catalogue (tables,
columns, types, nullability, primary and foreign keys, unique indexes, check
constraints), and compare it with the entities in the spec. Names on the
database side are derived with the rules in `docs/conventions.md`, the same
rules the migration emitter uses, so the two sides meet without a mapping
table.

Compare only what can be derived exactly. A column's type and nullability, a
key, a unique index and a check constraint's name are exact, and so are a
generated default the type-rendering idiom names and a plain index the
implementation file lists under the entity's mapping, its name and its
columns in order (ADR-121). The text of a check expression is not, since
the database normalises it, so that comparison is a report, and so is an
index that is not plain: one of an expression, with a condition or an
order, which no mapping holds.

`specarch check` reads the catalogue dump with the database reader and
compares it with the tables `specarch generate sql` writes for PostgreSQL
through the implementation file: a table by its name, a column by its table
and name, a type as PostgreSQL's `format_type` spells it (only the spelling
is normalised), the nullability, a generated default, the primary key, a
foreign key and a unique constraint by their columns, a check constraint by
its name and a listed index by its name and columns. A check's text that
differs and an index the mapping does not list are printed as warnings.

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

The allowlist of `specarch check` holds one id per line, the rule and what
the difference is about as check prints them between brackets (for example
`column_not_in_database members.phone_number`), with `#` comments. A line
that allows nothing fails, so a settled difference leaves the list with
the change that settles it; `--since <revision>` fails a line the allowlist
did not hold at that revision, which CI gives as the commit the change is
made on top of. `examples/lending-desk/check-allowlist.txt` is one, each line
with why it stays open.
New endpoints go spec first from the moment the gate is on; existing handlers
move onto the generated interface when the service is next touched, as
`docs/extraction.md` describes.
