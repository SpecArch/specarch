# Idioms: one way to do each recurring thing

An idiom is a named, versioned statement of how one recurring implementation
concern is done: how an endpoint validates and refuses, how a list pages and
filters, how a row is audited and soft-deleted, how a secret reaches a
process, how a personal value is masked in a log. SpecArch ships a set of
them, versioned with SpecArch. An implementation file uses the shipped set
by default and may override an idiom, whole or part by part, in a file of
its own; every override is recorded, reasoned and reported. The validator,
the documents and the generators follow the idiom that results. This is the
owner's decision of 2026-10-08, and this document is the design of it: what
an idiom is, its file, where it lives, how an implementation file uses and
overrides one, how a project learns that a newer template exists, what the
validator checks, and the first set, written from dxlib.

The reason for idioms is the second principle of `docs/principles.md`: keep
the changing apart from the fixed. A project's design changes with the
project; how a Go service pages a list does not, and should be written once,
read by every generator and every agent, and changed on purpose. Without
idioms every implementation file restates the same conventions in prose, each
slightly differently, and a generator or an agent has nothing to follow but
that prose. dxlib is the proof that the conventions are stable: a dozen of
them have carried every service the owner built on it, unchanged.

## What an idiom is

An idiom belongs to the implementation side of the boundary in
`docs/conventions.md`. It never adds design: it cannot declare an entity, an
operation or a permission. It reads the design, through keywords it names,
and says how one stack carries what the design says. It has three parts:

1. **The contract**: statements that hold on every stack, each checkable by
   the validator or a generator where that is possible, and marked as
   guidance where it is not. A guidance statement is for the agent or
   person writing the code; a checkable one is enforced.
2. **The renderings**: per stack, the libraries, the settings, the derived
   names and the code shape that carry the contract. A stack is a language
   or a dialect: `go`, `swift`, `dart`, or `sql` with one rendering per
   dialect.
3. **The tests**: the design-test cases the idiom implies for every element
   it applies to, derived the way `docs/test-generation.md` derives cases
   from the design.

The rule from `docs/extraction.md` applies to every idiom: a keyword no tool
checks is a defect. A contract statement therefore says what checks it, and
an idiom whose every statement is guidance is still an idiom, but the
document says so, so a reader knows it is advice and not a gate.

## The file

An idiom is one YAML file, `<name>.specarch-idiom.yaml`, root key
`specarchIdiom: "0.1"`, with its own JSON Schema,
`schema/specarch-idiom-0.1.schema.json`, closed like the other two.

    specarchIdiom: "0.1"
    name: paginated-list
    version: 1.0.0
    concern: list-operations
    stacks: [go, swift, dart]
    description: How an operation that lists an entity pages, searches, filters and sorts.
    reads: [listOf]
    why: |
      Every list endpoint of every service answers the same four questions:
      which page, how large, in what order, matching what. One shape means one
      client component and one test set.
    contract:
      page:
        statement: The operation takes a page index from zero and a page size, and answers the rows, the total row count and the total page count.
        check: document
      page-size-limit:
        statement: The page size has a maximum, and a request above it is refused with the problem type page-size-too-large.
        check: test
      sortable:
        statement: A sort field that is not in the operation's sortable list is refused, never ignored.
        check: test
      filterable:
        statement: A filter over a field that is not in the filterable list is refused, never ignored.
        check: test
      search:
        statement: Free-text search matches any of the searchable fields, case-insensitively.
        check: guidance
    parts:
      parameters:
        description: The parameter names and where they go.
        stack:
          go:
            names: { search: search_text, filters: filter_key_values, sort: order_by, pageSize: row_per_page, pageIndex: page_index }
            code: |
              The parameters are body fields of a POST, as the runtime reads them.
          swift:
            names: { search: q, filters: filter, sort: sort, pageSize: pageSize, pageIndex: page }
      envelope:
        description: The answer's shape.
        stack:
          go:
            code: |
              { "list": { "rows": [...], "total_rows": 0, "total_page": 0 } }
      filter-operators:
        description: The operators a filter value may carry.
        stack:
          go:
            names: { equal: eq, in: in, greater: gt, greaterOrEqual: gte, less: lt, lessOrEqual: lte, null: is_null, notNull: not_null }
    tests:
      - { case: page beyond last, scenario: golden, then: an empty row list with the true totals }
      - { case: page size above maximum, scenario: red, then: the problem type page-size-too-large }
      - { case: sort by a field not sortable, scenario: red }
      - { case: filter by a field not filterable, scenario: red }

The keys:

| Key | Holds |
|---|---|
| `name` | kebab-case; the key an implementation file and an override use |
| `version` | the idiom's own version, semantic; it moves when a contract statement, a part or a test changes |
| `concern` | one of a closed list: `type-rendering`, `request-validation`, `error-response`, `list-operations`, `authorization`, `pii-logging`, `audit-fields`, `soft-delete`, `identifiers`, `transactions`, `retries`, `idempotency`, `configuration`, `secrets`, `background-jobs`, `migrations`, `encryption`, `health`, `rate-limit`, `other` |
| `stacks` | the stacks the idiom renders; `any` for one that is stack-neutral |
| `reads` | the design keywords it applies to; an idiom applies only to a specification that uses them, so an idiom for `listOf` is silent in a specification without one |
| `contract` | statements keyed by id, each with `statement`, and `check`: `schema` (a shape the implementation file or an override must carry), `name` (a derivation rule for names, as the conventions derive table names), `document` (something the generated document or code must contain, checked by the generator's `--check`), `test` (a derived test case), or `guidance` (no check) |
| `parts` | the renderings, keyed by part name, each with `description` and `stack`, under which each stack has `libraries` (name to version and licence, as the implementation file's), `settings`, `names`, `rows` (for a type rendering, below), `code`, `why` and `cites` |
| `tests` | the cases derived for every element the idiom applies to, in the shape of the derived cases of `docs/conventions.md` |
| `why`, `cites` | as everywhere else |
| `sources` | the sources the idiom's citations name, in the shape of a specification's `sources` |

In a project's file, one more key:

| Key | Holds |
|---|---|
| `overrides` | `{ idiom, version, parts: [names] }` or `{ idiom, version, whole: true }`, with `staysBehind: true` when the project keeps an older version on purpose, and `why` at the top of the file; `version` is the shipped version the file was copied from |

A project's own idiom, for a concern SpecArch ships nothing for, is the same
file without `overrides`, under a name no shipped idiom has.

## Where idioms live

The shipped set is in this repository under `idioms/<concern>/<name>.specarch-idiom.yaml`,
embedded into the `specarch` binary the way `schema/` is, so a project that
installs one SpecArch release has one idiom set. The set is versioned with
SpecArch: a release tag fixes both, and a project pins the tag, as it pins
the schema. There is no second version axis; a project does not pin one
idiom at an older version while running a newer SpecArch, it overrides it.

A project's overrides and its own idioms live beside its implementation
file, under `spec/implementation/<stack>/idioms/`, one file per idiom, named
after the idiom. The validator reads nothing else in that folder.

## How an implementation file uses them

By default, every shipped idiom whose `stacks` include the implementation
file's stack and whose `reads` keywords the specification uses applies. An
implementation file declares nothing to get them, which is the owner's rule
and the usual case. It declares only what deviates, under `idioms`:

    idioms:
      error-response:
        exclude: true
        why: The service answers in the error shape its existing clients parse; the problem catalogue is rendered into that shape by the project's own idiom legacy-error-response.
      type-rendering:
        override: idioms/type-rendering.specarch-idiom.yaml

`exclude` needs a `why`. `override` names the file in the project's idioms
folder, which carries `overrides` with the parts it replaces. A key under
`idioms` that names neither a shipped nor a project idiom is refused.

Because the defaults apply silently, two things make them visible, so that
no reader is surprised: `specarch idioms <folders>` lists, per
implementation file, every idiom that applies, its version, whether it is
shipped, overridden or the project's own, and which parts an override
replaces; and the techspec's implementation chapter carries the same as an
Idioms table, with each override's `why` as an Insight.

## Stacks and type rows

An implementation file's stacks are its language, from its file name
(`<name>.go.specarch-implementation.yaml` is `go`), and the `dialect` of each
of its targets: `postgresql`, `sqlserver`, `oracle` or `mariadb`; a target
named `sql` without a dialect is `postgresql`. An idiom applies to the file
when it renders one of those stacks, or `any`, and the specification uses
one of the keywords under its `reads`, as a section (`entities`) or as a key
anywhere inside one (`listOf` on an operation). An override may render only those stacks and
`any` (`idiom_stack`).

A part that renders types carries `rows` per stack. A row matches a field on
its JSON type (with `null` dropped from a type list; a `$ref` to an enum is
an enum string, to an entity an object), its `format`, `enum: true`,
`maxLengthAtMost`, `precisionAtMost`, and for an array `itemsType` and
`itemsFormat`. The rows are tried in order and the first that matches
renders the field. A row without a `format` matches only a format that no
row of the rendering names, so a decimal or a UUID never falls through to
the row for plain text. Words in braces in `render`, such as
`VARCHAR({maxLength})`, are filled from the field by a generator; they are a
function of the field, never left in output, and so not a placeholder. The
validator checks that every entity field has a row for every stack of the
file the idiom renders (`idiom_contract`), and reports a missing row at the
stack's key in the implementation file: `stack` for the language, the
target's `dialect` for a SQL dialect.

An idiom file declares the sources it cites under `sources`, in the shape of
a specification's, since it is read on its own.

## Lookup order

For one implementation file, one idiom and one part, the rendering used is
the first of:

1. the part in the project's override file, for the file's stack;
2. the part in the shipped idiom, for the file's stack;
3. the part in the shipped idiom under `any`, when the part has no rendering
   for the stack.

A part the override does not name comes from the template, so an override
is as small as the difference. `whole: true` replaces every part. An
override may add contract statements and may not remove or weaken a shipped
one: the contract is what makes the idiom the same across projects, and a
project that needs a different contract excludes the shipped idiom and
writes its own under another name, with the reason. A statement the override
repeats must be identical in `statement` and `check`; one it changes is
refused (`idiom_contract`).

## Noticing a newer template

Every override file records the shipped version it was copied from. On every
validation the validator compares that with the version it ships. When the
shipped one is newer it warns, `idiom_version_behind`, naming both versions
and the parts the override replaces, and `specarch idioms diff <name> <folders>`
prints, for each part the override replaces and each stack it renders, the
shipped rendering beside the override's. The warning repeats until
the project either moves the override to the new version, by copying the
changed parts it wants and updating `version`, or sets `staysBehind: true`
under `overrides` and says why, which silences the warning and leaves the
reason in the Idioms table. An idiom whose shipped `version` changed but whose
override replaced none of the changed parts gets the same warning, because
the contract or the tests may have moved.

## What the validator checks

| Rule | When |
|---|---|
| `idiom_unknown` | an `idioms` key or an `overrides.idiom` names no shipped or project idiom |
| `idiom_part_unknown` | an override names a part the idiom does not have |
| `idiom_override_reason` | an override or an exclusion without `why` |
| `idiom_stack` | an override renders a stack that is not the implementation file's |
| `idiom_version_behind` | the override's `version` is older than the shipped one (warning) |
| `idiom_contract` | a contract statement with a `schema`, `name` or `document` check fails for an element the idiom applies to: a field type with no row in the type-rendering table for the target's dialect, a `listOf` without a page-size maximum, an entity marked `audited` whose generated table lacks the audit columns |
| `test_case_missing` | the cases under `tests` join the derived cases of the subject, with the same ranking |

The generators follow the resolved idiom when they write, and their
`--check` form, the fourth gate of `docs/sync-gates.md`, keeps the written
code on it.

## The first set, from dxlib

All fifteen ship, under `idioms/<concern>/`. Each row names the idiom,
which part of its contract a tool checks, and the Go rendering taken from
dxlib. The stack-neutral contract is written first; the Go rendering second;
Swift and Dart renderings when the first project on each stack exists. The
dxlib files each is read from are in `docs/dxlib-lessons.md`. A `document`
check on a table's columns is made by `specarch-gen-sql`'s `--check`; the
identifiers' columns are not written by it, since a design's own primary
key is rendered as declared, and their row says so.

| Idiom | Contract, checked | Contract, guidance | Go rendering from dxlib |
|---|---|---|---|
| `type-rendering` | every field type has a row for every target of the implementation file (`schema`); the SQL emitted matches the rows, and a key or unique text column is at most 255 wide (`document`, checked by `specarch-gen-sql`) | | the four-dialect table of `docs/dxlib-lessons.md`; Go: `int32`, `int64`, `decimal.Decimal` from `shopspring/decimal` for a decimal and for money, `string`, `bool`, `time.Time` for an instant and a date, `[]byte`, a pointer for a nullable scalar, a presence flag for a field that may be left out, a map for an object |
| `request-validation` | every constraint of the design is in the emitted document (`document`); every red case of a constraint is derived (`test`) | a refusal names the parameter's path; a field that may be left out and one that may be null are told apart | the pre-processing of `api/api_endpoint_request.go`: required, type, bounds, format, enum, children, before the handler runs |
| `error-response` | every 4xx and 5xx response is `application/problem+json` with the catalogue's type (`document`) | a validation problem lists each failing parameter | dxlib's `{status, status_code, reason, reason_message}` rendered from the problem, under the stack `dxlib`, until the runtime answers with a problem document |
| `paginated-list` | the parameters and the envelope (`document`); page beyond last, size above maximum, sort and filter outside the lists (`test`) | search is case-insensitive over the searchable fields | stack-neutral, under `any`: `search`, `filter`, `sort`, `page`, `pageSize` and the envelope `items`, `totalItems`, `totalPages`; dxlib's own names (`search_text`, `filter_key_values`, `order_by`, `row_per_page`, `page_index`, the `list` envelope with `rows`, `total_rows`, `total_page`) are its rendering under the stack `dxlib`, which the dxlib dialect of the OpenAPI document uses |
| `authorization-check` | denied without the permission (`test`, already derived) | the check runs after authentication and before the handler; an unauthenticated caller gets 401, an authenticated one without the permission 403; fail closed | the session middleware of `dxlib_module/module/self` and the endpoint's `Privileges` |
| `pii-in-logs` | a `credential` field never appears in a response (`schema`, from `sensitivity`) | every personal field is masked by its rule in every log line, request dump and response dump; credential headers are masked whole | the mask rules of `utils/utils.go`: partial with front and back characters kept, email as two characters of each part, initials as the first letter of each word, location rounded to two decimals; `MaskForLog` on every dump |
| `audit-fields` | an audited entity's table carries the six columns (`document`, checked by `specarch-gen-sql`) | the library sets them; a caller's values are overwritten | `created_at`, `created_by_user_id`, `created_by_user_nameid`, `last_modified_at`, `last_modified_by_user_id`, `last_modified_by_user_nameid`, set in `tables/tables_table.go` |
| `soft-delete` | the column (`document`, checked by `specarch-gen-sql`); deleted rows are not listed and read as not found (`test`) | a hard delete is a separate, separately permitted operation | `is_deleted` boolean, false by default; the list filter adds `is_deleted = false` unless asked; `RequestSoftDelete` beside `RequestHardDelete` |
| `identifiers` | the column names (`document`, not yet written: a design's own primary key is rendered as declared) | an internal integer key that never leaves the service; a public opaque id of at most 255 characters that cannot be enumerated; an optional human name id, unique; an optional version tag | `id` as a 64-bit generated key; `uid` as hexadecimal microseconds plus a UUID, 255 wide; `nameid`; `utag` |
| `transactions` | | one transaction per mutating operation, the audit entry inside it, rolled back on any error | `DXDatabaseTx` through `TransactionBegin`, the `Tx` forms of insert, update and delete |
| `configuration-and-secrets` | no secret value in any file (`secret_value`, already a rule) | settings from a file and the environment; a secret read from a vault into locked memory and resolved only where it is used | `configuration` with `SensitiveDataKey`, `secure_memory`, `vault` |
| `background-jobs` | the derived cases `runs twice` and `dependency fails` (`test`) | a job runs once or repeats with a delay, stops on shutdown, retries a failed item a bounded number of times and then marks it dead | `task` with `once` or `always` and `after_delay_sec`; the drain loop of the notification module |
| `migrations` | new files only, destructive steps in their own file (`document`, checked by `specarch-gen-sql`) | the model is the source and the DDL is derived | `models.ModelDB` and `CreateDDL`, with the snapshot the ERP backend keeps |
| `encrypted-column` | an `atRest: encrypted` field renders through the idiom or fails (`document`, checked by `specarch-gen-sql`) | encryption in the engine with a session key from locked memory; a salted hash companion when the field must stay searchable | `EncryptionColumnDef` with `HashFieldName`, the per-dialect expressions of `databases/db/encryption_expression.go` |
| `health-endpoint` | | a public operation answers the service's name and version | `GET /ping` of `dxlib_module/module/oam` |

Three of these, `transactions`, `configuration-and-secrets` beyond its one
rule, and `health-endpoint`, are guidance only, or nearly; the operation a
health check answers cannot be told from its design alone. They are kept, because an
agent writing a service needs the sentence as much as a generator needs the
table, and the document marks them as what they are.

## Implementation items

In order; each changes the specification of `specarch` first, both validator
builds where it adds a rule, and the conformance cases.

Items 1 and 2 are built, and of item 3 the two verbs and the Idioms table;
not yet built are each idiom's contract in the techspec's chapter 8, and the
cases under an idiom's `tests` joining the derived cases
(`test_case_missing`).

1. The idiom schema, `schema/specarch-idiom-0.1.schema.json`, and the file
   format above; the shipped folder `idioms/` embedded into the binary; the
   `idioms` section and the `idioms/` folder of an implementation file in
   the implementation schema and the layout rule.
2. The validator: resolution by the lookup order, the rules of the table
   above, `idiom_contract` for the `schema` and `name` checks.
3. `specarch idioms <folders>` and `specarch idioms diff <name>`, the Idioms
   table in the techspec's implementation chapter, and each idiom's contract
   in chapter 8.
4. The first two shipped idioms, `type-rendering` and `paginated-list`, with
   their Go and SQL renderings, since `specarch-gen-sql` and
   `specarch-gen-openapi` read them. `type-rendering` is built;
   `paginated-list` is built too, with its stack-neutral wire names
and envelope, which `specarch-gen-openapi` reads.
5. Built: the rest of the first set, with its Go renderings. Swift and Dart
   renderings come with the first project on each stack.
